// Package httpcache provides a generic caching HTTP fetcher.
package httpcache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CacheConfig configures the caching behavior of a CachingFetcher.
type CacheConfig struct {
	// StaticExpiry defines a fixed cache duration (e.g., 1 hour)
	StaticExpiry time.Duration

	// ReturnStale controls whether stale data should be returned while refreshing
	// If true, returns stale data immediately and refreshes in background
	// If false, blocks until fresh data is fetched
	ReturnStale bool
}

// FetchFunc is a custom function type for fetching data from an HTTP endpoint.
type FetchFunc[T any] func(ctx context.Context, url string) (T, error)

// CacheResult indicates the status of cached data.
type CacheResult int

const (
	// CacheResultFresh indicates data was freshly fetched.
	CacheResultFresh CacheResult = iota
	// CacheResultCached indicates data was returned from cache.
	CacheResultCached
	// CacheResultStale indicates stale data was returned due to fetch error.
	CacheResultStale
)

// FetchResult contains the fetched data and metadata about the fetch.
type FetchResult[T any] struct {
	Data   T
	Result CacheResult
	Error  error
}

// CachingFetcher is a generic caching HTTP fetcher that handles HTTP requests with caching.
type CachingFetcher[T any] struct {
	url        string
	config     CacheConfig
	fetchFunc  FetchFunc[T] // custom fetch function, defaults to JSON fetching
	httpClient *http.Client

	mu          sync.RWMutex
	cachedData  *T
	cachedAt    time.Time
	expiresAt   time.Time
	lastError   error
	refreshing  bool
	refreshCond *sync.Cond
}

var defaultFetchClient = &http.Client{Timeout: 30 * time.Second}

// NewCachingFetcher creates a new caching fetcher for the specified URL and type.
// It uses JSON unmarshaling by default to decode the response body into type T.
func NewCachingFetcher[T any](url string, config CacheConfig) *CachingFetcher[T] {
	f := &CachingFetcher[T]{
		url:        url,
		config:     config,
		fetchFunc:  nil,
		httpClient: defaultFetchClient,
	}
	f.refreshCond = sync.NewCond(&f.mu)

	return f
}

// NewCachingFetcherWithFunc creates a new caching fetcher with a custom fetch function.
// If fetchFunc is nil, it defaults to JSON unmarshaling. This allows for custom
// parsing of the HTTP response (e.g., plain text lines).
func NewCachingFetcherWithFunc[T any](url string, config CacheConfig, fetchFunc FetchFunc[T]) *CachingFetcher[T] {
	f := &CachingFetcher[T]{
		url:        url,
		config:     config,
		fetchFunc:  fetchFunc,
		httpClient: defaultFetchClient,
	}
	f.refreshCond = sync.NewCond(&f.mu)

	return f
}

// Get fetches data from the URL with caching.
// It returns the data, cache result status (Fresh, Cached, or Stale), and any error encountered.
// If ReturnStale is enabled, it may return stale data immediately and start a background refresh.
//
// CacheResultStale is always returned with a nil error: the data is valid
// and safe to use even though it came from a failed refresh attempt rather
// than from fresh cache. This holds whether the stale data was returned
// immediately (ReturnStale) or as a fallback after a blocking refresh
// failed. A caller that follows the common `if err != nil { return }`
// pattern must not have that discard perfectly usable stale data; check the
// returned CacheResult, not just err, to distinguish "no data at all" from
// "stale but usable.".
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

	// Need to fetch now (blocking). If already refreshing, wait for it and
	// share its result instead of starting our own fetch.
	if data, result, err, waited := f.waitForConcurrentRefresh(); waited {
		f.mu.Unlock()

		return data, result, err
	}

	// Mark as refreshing
	f.refreshing = true
	f.mu.Unlock()

	// Perform the fetch
	data, headers, err := f.doFetch(ctx)

	f.mu.Lock()
	f.refreshing = false
	f.lastError = err

	if err != nil {
		// If fetch failed and we have stale data, return it. It is still
		// usable data, so -- as with every other CacheResultStale return in
		// this method -- the error is not surfaced here (see Get's doc
		// comment); it remains available via LastError() for introspection.
		if staleData != nil {
			result := *staleData
			f.mu.Unlock()
			f.refreshCond.Broadcast()

			return result, CacheResultStale, nil
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
	f.expiresAt = f.calculateExpiry(headers)

	f.mu.Unlock()
	f.refreshCond.Broadcast()

	return data, CacheResultFresh, nil
}

// waitForConcurrentRefresh waits while another goroutine is already
// refreshing (looping on the condition, rather than a single check, to
// protect against spurious wakeups per sync.Cond's documented contract)
// and, once that refresh completes, reports its shared result with
// waited=true. waited=false means no refresh was in progress, so the
// caller remains responsible for performing its own fetch. f.mu must be
// held on entry and remains held on return either way.
func (f *CachingFetcher[T]) waitForConcurrentRefresh() (T, CacheResult, error, bool) {
	waited := false
	for f.refreshing {
		waited = true
		f.refreshCond.Wait()
	}
	if !waited {
		var zero T

		return zero, CacheResultFresh, nil, false
	}

	if f.cachedData == nil {
		// The fetch we waited on failed and left no stale data to fall
		// back to. Every waiter shares that single failure rather than
		// each independently retrying against an already-struggling
		// origin.
		var zero T

		return zero, CacheResultFresh, f.lastError, true
	}

	result := CacheResultFresh
	if f.lastError != nil {
		// The fetch we waited on failed, but there is still usable
		// (stale) data -- CacheResultStale never carries a non-nil
		// error; see Get's doc comment.
		result = CacheResultStale
	}

	return *f.cachedData, result, nil, true
}

// backgroundRefresh performs a refresh in the background.
func (f *CachingFetcher[T]) backgroundRefresh(ctx context.Context) {
	data, headers, err := f.doFetch(ctx)

	f.mu.Lock()
	defer f.mu.Unlock()

	f.refreshing = false
	f.lastError = err

	if err == nil {
		f.cachedData = &data
		f.cachedAt = time.Now()
		f.expiresAt = f.calculateExpiry(headers)
	}

	f.refreshCond.Broadcast()
}

// doFetch performs the actual fetch, using custom function if provided.
// It returns the fetched data, the HTTP response headers (nil when fetchFunc is used),
// and any error.
func (f *CachingFetcher[T]) doFetch(ctx context.Context) (T, http.Header, error) {
	if f.fetchFunc != nil {
		data, err := f.fetchFunc(ctx, f.url)

		return data, nil, err
	}

	return f.fetchJSON(ctx)
}

// fetchJSON performs the actual HTTP request and JSON unmarshaling.
// It returns the parsed value, the response headers (for cache-expiry calculation),
// and any error.
func (f *CachingFetcher[T]) fetchJSON(ctx context.Context) (T, http.Header, error) {
	var result T

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return result, nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return result, nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	headers := resp.Header

	if resp.StatusCode != http.StatusOK {
		return result, headers, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, headers, fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, headers, fmt.Errorf("unmarshal json: %w", err)
	}

	return result, headers, nil
}

// calculateExpiry determines when the cached data expires based on HTTP cache headers.
func (f *CachingFetcher[T]) calculateExpiry(headers http.Header) time.Time {
	now := time.Now()

	if headers != nil {
		// Check Cache-Control header first (takes precedence)
		if cacheControl := headers.Get("Cache-Control"); cacheControl != "" {
			return f.parseCacheControl(cacheControl, now)
		}

		// Fall back to Expires header
		if expires := headers.Get("Expires"); expires != "" {
			return f.parseExpires(expires, now)
		}
	}

	// Use static expiry if configured
	if f.config.StaticExpiry > 0 {
		return now.Add(f.config.StaticExpiry)
	}

	// Default to 1 hour
	return now.Add(1 * time.Hour)
}

// cacheControlDirectives holds the caching-relevant directives extracted
// from a Cache-Control header.
type cacheControlDirectives struct {
	maxAge    time.Duration
	hasMaxAge bool
	noStore   bool
	noCache   bool
}

// applyCacheControlDirective parses a single (already-trimmed) Cache-Control
// directive, updating d with whichever of no-store, no-cache, or max-age it
// recognizes. must-revalidate is accepted but otherwise ignored: it is
// implicitly satisfied by this fetcher's existing expiry logic, since it
// never serves stale content past expiry without revalidating.
func applyCacheControlDirective(directive string, d *cacheControlDirectives) {
	switch directive {
	case "no-store":
		d.noStore = true
	case "no-cache":
		d.noCache = true
	case "must-revalidate":
	default:
		after, ok := strings.CutPrefix(directive, "max-age=")
		if !ok {
			return
		}

		maxAgeStr := strings.Trim(after, "\"") // handle quoted values
		if duration, err := time.ParseDuration(maxAgeStr + "s"); err == nil {
			d.maxAge = duration
			d.hasMaxAge = true
		}
	}
}

// parseCacheControl extracts max-age from Cache-Control header and handles caching directives.
func (f *CachingFetcher[T]) parseCacheControl(cacheControl string, now time.Time) time.Time {
	var directives cacheControlDirectives
	for directive := range strings.SplitSeq(cacheControl, ",") {
		applyCacheControlDirective(strings.TrimSpace(directive), &directives)
	}

	switch {
	case directives.noStore:
		// Don't cache: use immediate expiry.
		return now
	case directives.noCache:
		// Cache, but with a very short expiry (1 second) to effectively
		// force revalidation while still allowing brief caching to
		// prevent request storms.
		return now.Add(1 * time.Second)
	case directives.hasMaxAge:
		return now.Add(directives.maxAge)
	case f.config.StaticExpiry > 0:
		return now.Add(f.config.StaticExpiry)
	default:
		return now.Add(1 * time.Hour)
	}
}

// parseExpires parses the HTTP Expires header.
// According to RFC 7231, this may be in RFC 1123, RFC 850, or ANSI C's asctime format.
func (f *CachingFetcher[T]) parseExpires(expiresStr string, now time.Time) time.Time {
	// Try multiple standard HTTP date formats
	layouts := []string{
		time.RFC1123,
		time.RFC850,
		time.ANSIC,
	}

	for _, layout := range layouts {
		if expiresTime, err := time.Parse(layout, expiresStr); err == nil {
			return expiresTime
		}
	}
	// Fall back to static expiry
	if f.config.StaticExpiry > 0 {
		return now.Add(f.config.StaticExpiry)
	}

	return now.Add(1 * time.Hour)
}

// GetCachedData returns the currently cached data without performing a fetch.
// It returns nil if no data is currently cached.
func (f *CachingFetcher[T]) GetCachedData() *T {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.cachedData
}

// GetCacheInfo returns information about the current cache status.
// It returns the time the data was cached, the time it expires, and whether data is present.
func (f *CachingFetcher[T]) GetCacheInfo() (time.Time, time.Time, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.cachedAt, f.expiresAt, f.cachedData != nil
}

// LastError returns the error from the most recently completed fetch
// attempt (nil if the last attempt succeeded, or no fetch has completed
// yet). Use this to inspect why Get returned a CacheResultStale result --
// Get itself never pairs stale-but-usable data with a non-nil error.
func (f *CachingFetcher[T]) LastError() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.lastError
}
