package prefixlist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CacheConfig configures the caching behavior of a CachingFetcher
type CacheConfig struct {
	// StaticExpiry defines a fixed cache duration (e.g., 1 hour)
	StaticExpiry time.Duration
	
	// ReturnStale controls whether stale data should be returned while refreshing
	// If true, returns stale data immediately and refreshes in background
	// If false, blocks until fresh data is fetched
	ReturnStale bool
	
	// ExpiryHeader specifies the HTTP header to check for expiry (e.g., "Expires", "Cache-Control")
	// If set, this takes precedence over StaticExpiry
	ExpiryHeader string
}

// FetchFunc is a custom function type for fetching data from an HTTP endpoint
type FetchFunc[T any] func(ctx context.Context, url string) (T, error)

// CacheResult indicates the status of cached data
type CacheResult int

const (
	// CacheResultFresh indicates data was freshly fetched
	CacheResultFresh CacheResult = iota
	// CacheResultCached indicates data was returned from cache
	CacheResultCached
	// CacheResultStale indicates stale data was returned due to fetch error
	CacheResultStale
)

// FetchResult contains the fetched data and metadata about the fetch
type FetchResult[T any] struct {
	Data   T
	Result CacheResult
	Error  error
}

// CachingFetcher is a generic caching HTTP fetcher that handles HTTP requests with caching
type CachingFetcher[T any] struct {
	url       string
	config    CacheConfig
	fetchFunc FetchFunc[T] // custom fetch function, defaults to JSON fetching
	
	mu           sync.RWMutex
	cachedData   *T
	cachedAt     time.Time
	expiresAt    time.Time
	lastError    error
	refreshing   bool
	refreshCond  *sync.Cond
}

// NewCachingFetcher creates a new caching fetcher for the specified URL and type
// Uses JSON unmarshaling by default
func NewCachingFetcher[T any](url string, config CacheConfig) *CachingFetcher[T] {
	f := &CachingFetcher[T]{
		url:       url,
		config:    config,
		fetchFunc: nil,
	}
	f.refreshCond = sync.NewCond(&f.mu)
	return f
}

// NewCachingFetcherWithFunc creates a new caching fetcher with a custom fetch function
// If fetchFunc is nil, defaults to JSON unmarshaling
func NewCachingFetcherWithFunc[T any](url string, config CacheConfig, fetchFunc FetchFunc[T]) *CachingFetcher[T] {
	f := &CachingFetcher[T]{
		url:       url,
		config:    config,
		fetchFunc: fetchFunc,
	}
	f.refreshCond = sync.NewCond(&f.mu)
	return f
}

// Get fetches data from the URL with caching
// Returns the data, cache result status, and any error encountered
func (f *CachingFetcher[T]) Get(ctx context.Context) (T, CacheResult, error) {
	f.mu.Lock()
	
	// Check if we have valid cached data
	if f.cachedData != nil && time.Now().Before(f.expiresAt) {
		data := *f.cachedData
		f.mu.Unlock()
		return data, CacheResultCached, nil
	}
	
	// Data is expired or doesn't exist
	staleData := f.cachedData
	
	// If return stale is enabled and we have stale data
	if f.config.ReturnStale && staleData != nil {
		// Return stale data immediately
		data := *staleData
		
		// Start background refresh if not already refreshing
		if !f.refreshing {
			f.refreshing = true
			go f.backgroundRefresh(ctx)
		}
		
		f.mu.Unlock()
		return data, CacheResultStale, nil
	}
	
	// Need to fetch now (blocking)
	// If already refreshing, wait for it
	if f.refreshing {
		f.refreshCond.Wait()
		// After wait, check if we now have data
		if f.cachedData != nil {
			data := *f.cachedData
			err := f.lastError
			result := CacheResultFresh
			if err != nil {
				result = CacheResultStale
			}
			f.mu.Unlock()
			return data, result, err
		}
	}
	
	// Mark as refreshing
	f.refreshing = true
	f.mu.Unlock()
	
	// Perform the fetch
	data, err := f.doFetch(ctx)
	
	f.mu.Lock()
	f.refreshing = false
	f.lastError = err
	
	if err != nil {
		// If fetch failed and we have stale data, return it
		if staleData != nil {
			result := *staleData
			f.mu.Unlock()
			f.refreshCond.Broadcast()
			return result, CacheResultStale, err
		}
		// No stale data, return zero value
		var zero T
		f.mu.Unlock()
		f.refreshCond.Broadcast()
		return zero, CacheResultFresh, err
	}
	
	// Success - cache the data
	f.cachedData = &data
	f.cachedAt = time.Now()
	f.expiresAt = f.calculateExpiry(nil) // TODO: pass response headers
	
	f.mu.Unlock()
	f.refreshCond.Broadcast()
	return data, CacheResultFresh, nil
}

// backgroundRefresh performs a refresh in the background
func (f *CachingFetcher[T]) backgroundRefresh(ctx context.Context) {
	data, err := f.doFetch(ctx)
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.refreshing = false
	f.lastError = err
	
	if err == nil {
		f.cachedData = &data
		f.cachedAt = time.Now()
		f.expiresAt = f.calculateExpiry(nil) // TODO: pass response headers
	}
	
	f.refreshCond.Broadcast()
}

// doFetch performs the actual fetch, using custom function if provided
func (f *CachingFetcher[T]) doFetch(ctx context.Context) (T, error) {
	if f.fetchFunc != nil {
		return f.fetchFunc(ctx, f.url)
	}
	return f.fetchJSON(ctx)
}

// fetchJSON performs the actual HTTP request and JSON unmarshaling
func (f *CachingFetcher[T]) fetchJSON(ctx context.Context) (T, error) {
	var result T
	
	req, err := http.NewRequestWithContext(ctx, "GET", f.url, nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("read response: %w", err)
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("unmarshal json: %w", err)
	}
	
	return result, nil
}

// calculateExpiry determines when the cached data expires
func (f *CachingFetcher[T]) calculateExpiry(headers http.Header) time.Time {
	// TODO: Implement header-based expiry parsing
	// For now, use static expiry
	if f.config.StaticExpiry > 0 {
		return time.Now().Add(f.config.StaticExpiry)
	}
	
	// Default to 1 hour
	return time.Now().Add(1 * time.Hour)
}

// GetCachedData returns the currently cached data without fetching
// Returns nil if no data is cached
func (f *CachingFetcher[T]) GetCachedData() *T {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cachedData
}

// GetCacheInfo returns information about the cache status
func (f *CachingFetcher[T]) GetCacheInfo() (cachedAt, expiresAt time.Time, hasData bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cachedAt, f.expiresAt, f.cachedData != nil
}
