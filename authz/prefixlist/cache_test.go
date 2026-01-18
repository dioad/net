package prefixlist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testData struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func TestCachingFetcher_Basic(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 100 * time.Millisecond,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, "hello", data1.Message)
	assert.Equal(t, 1, data1.Count)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call should use cache
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultCached, result2)
	assert.Equal(t, "hello", data2.Message)
	assert.Equal(t, 1, data2.Count)             // Same as before
	assert.Equal(t, int32(1), callCount.Load()) // No new call

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Third call should fetch again
	data3, result3, err3 := fetcher.Get(ctx)
	require.NoError(t, err3)
	assert.Equal(t, CacheResultFresh, result3)
	assert.Equal(t, "hello", data3.Message)
	assert.Equal(t, 2, data3.Count) // Updated
	assert.Equal(t, int32(2), callCount.Load())
}

func TestCachingFetcher_ReturnStale(t *testing.T) {
	callCount := atomic.Int32{}
	shouldFail := atomic.Bool{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if shouldFail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 100 * time.Millisecond,
		ReturnStale:  true,
	})

	ctx := context.Background()

	// First call should fetch successfully
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Make server fail
	shouldFail.Store(true)

	// Should return stale data immediately
	data2, result2, err2 := fetcher.Get(ctx)
	assert.Equal(t, CacheResultStale, result2)
	assert.Equal(t, 1, data2.Count) // Stale data
	assert.NoError(t, err2)         // No error returned with stale data

	// Wait for background refresh to complete
	time.Sleep(200 * time.Millisecond)

	// Should still return stale data (refresh failed)
	data3, result3, _ := fetcher.Get(ctx)
	assert.Equal(t, CacheResultStale, result3)
	assert.Equal(t, 1, data3.Count) // Still stale
}

func TestCachingFetcher_NoReturnStale_BlocksOnExpiry(t *testing.T) {
	callCount := atomic.Int32{}
	shouldDelay := atomic.Bool{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if shouldDelay.Load() {
			time.Sleep(100 * time.Millisecond)
		}
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 50 * time.Millisecond,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call
	data1, _, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, 1, data1.Count)

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Enable delay
	shouldDelay.Store(true)

	// Should block until fetch completes
	start := time.Now()
	data2, result2, err2 := fetcher.Get(ctx)
	duration := time.Since(start)

	require.NoError(t, err2)
	assert.Equal(t, CacheResultFresh, result2)
	assert.Equal(t, 2, data2.Count)
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
}

func TestCachingFetcher_Error_NoStaleData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// Should return error and zero value
	data, result, err := fetcher.Get(ctx)
	assert.Error(t, err)
	assert.Equal(t, CacheResultFresh, result)
	assert.Equal(t, "", data.Message)
	assert.Equal(t, 0, data.Count)
}

func TestCachingFetcher_ConcurrentAccess(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow response
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 200 * time.Millisecond,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// Launch multiple concurrent requests
	const goroutines = 10
	results := make(chan testData, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			data, _, err := fetcher.Get(ctx)
			require.NoError(t, err)
			results <- data
		}()
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		<-results
	}

	// Should have only called the server once (concurrent requests wait for the same fetch)
	assert.Equal(t, int32(1), callCount.Load())
}

func TestCachingFetcher_GetCachedData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := testData{Message: "hello", Count: 42}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
	})

	// Should be nil initially
	assert.Nil(t, fetcher.GetCachedData())

	// Fetch data
	ctx := context.Background()
	_, _, err := fetcher.Get(ctx)
	require.NoError(t, err)

	// Should now have cached data
	cached := fetcher.GetCachedData()
	require.NotNil(t, cached)
	assert.Equal(t, "hello", cached.Message)
	assert.Equal(t, 42, cached.Count)
}

func TestCachingFetcher_GetCacheInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := testData{Message: "hello", Count: 42}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	expiry := 500 * time.Millisecond
	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: expiry,
	})

	// Initially no data
	_, _, hasData := fetcher.GetCacheInfo()
	assert.False(t, hasData)

	// Fetch data
	ctx := context.Background()
	beforeFetch := time.Now()
	_, _, err := fetcher.Get(ctx)
	require.NoError(t, err)
	afterFetch := time.Now()

	// Check cache info
	cachedAt, expiresAt, hasData := fetcher.GetCacheInfo()
	assert.True(t, hasData)
	assert.True(t, cachedAt.After(beforeFetch) || cachedAt.Equal(beforeFetch))
	assert.True(t, cachedAt.Before(afterFetch) || cachedAt.Equal(afterFetch))
	assert.True(t, expiresAt.After(cachedAt))

	// Expiry should be approximately StaticExpiry from now
	expectedExpiry := time.Now().Add(expiry)
	assert.InDelta(t, expectedExpiry.Unix(), expiresAt.Unix(), 2) // Within 2 seconds
}
