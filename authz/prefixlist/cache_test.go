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

// TestCachingFetcher_ParseCacheControl tests the parseCacheControl function with various header formats
func TestCachingFetcher_ParseCacheControl(t *testing.T) {
	tests := []struct {
		name          string
		cacheControl  string
		staticExpiry  time.Duration
		expectedDelta time.Duration // Expected duration from now
	}{
		{
			name:          "simple max-age",
			cacheControl:  "max-age=3600",
			staticExpiry:  0,
			expectedDelta: 3600 * time.Second,
		},
		{
			name:          "max-age with other directives",
			cacheControl:  "public, max-age=7200, must-revalidate",
			staticExpiry:  0,
			expectedDelta: 7200 * time.Second,
		},
		{
			name:          "max-age with spaces",
			cacheControl:  "max-age=1800, public",
			staticExpiry:  0,
			expectedDelta: 1800 * time.Second,
		},
		{
			name:          "max-age with quotes",
			cacheControl:  "max-age=\"600\"",
			staticExpiry:  0,
			expectedDelta: 600 * time.Second,
		},
		{
			name:          "no max-age falls back to static expiry",
			cacheControl:  "public, must-revalidate",
			staticExpiry:  5 * time.Minute,
			expectedDelta: 5 * time.Minute,
		},
		{
			name:          "no max-age and no static expiry uses default",
			cacheControl:  "public",
			staticExpiry:  0,
			expectedDelta: 1 * time.Hour,
		},
		{
			name:          "empty cache-control falls back to static",
			cacheControl:  "",
			staticExpiry:  10 * time.Minute,
			expectedDelta: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &CachingFetcher[testData]{
				config: CacheConfig{
					StaticExpiry: tt.staticExpiry,
				},
			}
			now := time.Now()
			result := fetcher.parseCacheControl(tt.cacheControl, now)

			// Check that the result is approximately the expected duration from now
			expectedExpiry := now.Add(tt.expectedDelta)
			assert.InDelta(t, expectedExpiry.Unix(), result.Unix(), 1.0)
		})
	}
}

// TestCachingFetcher_ParseExpires tests the parseExpires function with various header formats
func TestCachingFetcher_ParseExpires(t *testing.T) {
	tests := []struct {
		name         string
		expires      string
		staticExpiry time.Duration
		shouldParse  bool // Whether the header should parse successfully
	}{
		{
			name:        "RFC 1123 format",
			expires:     "Mon, 02 Jan 2006 15:04:05 GMT",
			shouldParse: true,
		},
		{
			name:        "future date",
			expires:     time.Now().Add(2 * time.Hour).Format(time.RFC1123),
			shouldParse: true,
		},
		{
			name:        "past date",
			expires:     time.Now().Add(-1 * time.Hour).Format(time.RFC1123),
			shouldParse: true,
		},
		{
			name:         "invalid format falls back to static",
			expires:      "invalid-date-format",
			staticExpiry: 15 * time.Minute,
			shouldParse:  false,
		},
		{
			name:         "empty expires falls back to static",
			expires:      "",
			staticExpiry: 20 * time.Minute,
			shouldParse:  false,
		},
		{
			name:        "invalid format with no static falls back to default",
			expires:     "not-a-date",
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &CachingFetcher[testData]{
				config: CacheConfig{
					StaticExpiry: tt.staticExpiry,
				},
			}
			now := time.Now()
			result := fetcher.parseExpires(tt.expires, now)

			if tt.shouldParse {
				// Parse the expected time from the header
				expectedTime, err := time.Parse(time.RFC1123, tt.expires)
				require.NoError(t, err)
				assert.InDelta(t, expectedTime.Unix(), result.Unix(), 1.0)
			} else {
				// Should fall back to static expiry or default
				var expectedExpiry time.Time
				if tt.staticExpiry > 0 {
					expectedExpiry = now.Add(tt.staticExpiry)
				} else {
					expectedExpiry = now.Add(1 * time.Hour)
				}
				assert.InDelta(t, expectedExpiry.Unix(), result.Unix(), 1.0)
			}
		})
	}
}

// TestCachingFetcher_CalculateExpiry tests the calculateExpiry function with various header combinations
func TestCachingFetcher_CalculateExpiry(t *testing.T) {
	tests := []struct {
		name          string
		headers       http.Header
		staticExpiry  time.Duration
		expectedDelta time.Duration // Expected duration from now
	}{
		{
			name: "Cache-Control takes precedence over Expires",
			headers: http.Header{
				"Cache-Control": []string{"max-age=3600"},
				"Expires":       []string{time.Now().Add(2 * time.Hour).Format(time.RFC1123)},
			},
			staticExpiry:  10 * time.Minute,
			expectedDelta: 3600 * time.Second, // Cache-Control wins
		},
		{
			name: "Expires used when no Cache-Control",
			headers: http.Header{
				"Expires": []string{time.Now().Add(30 * time.Minute).Format(time.RFC1123)},
			},
			staticExpiry:  10 * time.Minute,
			expectedDelta: 30 * time.Minute,
		},
		{
			name:          "StaticExpiry used when no cache headers",
			headers:       http.Header{},
			staticExpiry:  45 * time.Minute,
			expectedDelta: 45 * time.Minute,
		},
		{
			name:          "Default 1 hour when no headers or static expiry",
			headers:       http.Header{},
			staticExpiry:  0,
			expectedDelta: 1 * time.Hour,
		},
		{
			name: "Cache-Control without max-age falls back to static",
			headers: http.Header{
				"Cache-Control": []string{"public, must-revalidate"},
				"Expires":       []string{time.Now().Add(15 * time.Minute).Format(time.RFC1123)},
			},
			staticExpiry:  10 * time.Minute,
			expectedDelta: 10 * time.Minute, // Falls back to static because Cache-Control is present without max-age; per HTTP semantics Expires is only used when Cache-Control is absent
		},
		{
			name: "Invalid Expires falls back to static",
			headers: http.Header{
				"Expires": []string{"invalid-date"},
			},
			staticExpiry:  25 * time.Minute,
			expectedDelta: 25 * time.Minute,
		},
		{
			name: "Cache-Control with multiple directives",
			headers: http.Header{
				"Cache-Control": []string{"public, max-age=7200, must-revalidate"},
			},
			staticExpiry:  10 * time.Minute,
			expectedDelta: 7200 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &CachingFetcher[testData]{
				config: CacheConfig{
					StaticExpiry: tt.staticExpiry,
				},
			}
			result := fetcher.calculateExpiry(tt.headers)

			expectedExpiry := time.Now().Add(tt.expectedDelta)
			assert.InDelta(t, expectedExpiry.Unix(), result.Unix(), 2.0) // Within 2 seconds
		})
	}
}

// TestCachingFetcher_HTTPCacheHeaders tests end-to-end behavior with HTTP cache headers
func TestCachingFetcher_HTTPCacheHeaders(t *testing.T) {
	t.Run("respects Cache-Control max-age", func(t *testing.T) {
		callCount := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			w.Header().Set("Cache-Control", "max-age=1")
			data := testData{Message: "hello", Count: int(callCount.Load())}
			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
			StaticExpiry: 1 * time.Hour, // Static expiry should be ignored
		})

		ctx := context.Background()

		// First call
		data1, result1, err1 := fetcher.Get(ctx)
		require.NoError(t, err1)
		assert.Equal(t, CacheResultFresh, result1)
		assert.Equal(t, 1, data1.Count)

		// Second call should use cache
		data2, result2, err2 := fetcher.Get(ctx)
		require.NoError(t, err2)
		assert.Equal(t, CacheResultCached, result2)
		assert.Equal(t, 1, data2.Count)
		assert.Equal(t, int32(1), callCount.Load())

		// Wait for cache to expire (1 second)
		time.Sleep(1200 * time.Millisecond)

		// Third call should fetch again
		data3, result3, err3 := fetcher.Get(ctx)
		require.NoError(t, err3)
		assert.Equal(t, CacheResultFresh, result3)
		assert.Equal(t, 2, data3.Count)
		assert.Equal(t, int32(2), callCount.Load())
	})

	t.Run("respects Expires header", func(t *testing.T) {
		callCount := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			expiresTime := time.Now().Add(1 * time.Second)
			w.Header().Set("Expires", expiresTime.Format(time.RFC1123))
			data := testData{Message: "hello", Count: int(callCount.Load())}
			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
			StaticExpiry: 1 * time.Hour,
		})

		ctx := context.Background()

		// First call
		data1, _, err1 := fetcher.Get(ctx)
		require.NoError(t, err1)
		assert.Equal(t, 1, data1.Count)

		// Second call should use cache
		data2, result2, err2 := fetcher.Get(ctx)
		require.NoError(t, err2)
		assert.Equal(t, CacheResultCached, result2)
		assert.Equal(t, 1, data2.Count)

		// Wait for expiry
		time.Sleep(1200 * time.Millisecond)

		// Third call should fetch again
		data3, result3, err3 := fetcher.Get(ctx)
		require.NoError(t, err3)
		assert.Equal(t, CacheResultFresh, result3)
		assert.Equal(t, 2, data3.Count)
	})

func TestCachingFetcher_CacheControl_NoStore(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call should fetch again (no-store means don't cache)
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultFresh, result2)
	assert.Equal(t, 2, data2.Count)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestCachingFetcher_CacheControl_NoCache(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Cache-Control", "no-cache")
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call immediately should use cache (no-cache allows brief caching)
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultCached, result2)
	assert.Equal(t, 1, data2.Count)
	assert.Equal(t, int32(1), callCount.Load())

	// Wait for the short expiry (1 second + buffer)
	time.Sleep(1100 * time.Millisecond)

	// Third call should fetch again (short expiry expired)
	data3, result3, err3 := fetcher.Get(ctx)
	require.NoError(t, err3)
	assert.Equal(t, CacheResultFresh, result3)
	assert.Equal(t, 2, data3.Count)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestCachingFetcher_CacheControl_MustRevalidate(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Cache-Control", "max-age=1, must-revalidate")
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		ReturnStale: false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)

	// Second call immediately should use cache
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultCached, result2)
	assert.Equal(t, 1, data2.Count)

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Third call should fetch again (must-revalidate means don't use stale)
	data3, result3, err3 := fetcher.Get(ctx)
	require.NoError(t, err3)
	assert.Equal(t, CacheResultFresh, result3)
	assert.Equal(t, 2, data3.Count)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestCachingFetcher_CacheControl_MaxAgeWithNoCache(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// no-cache should take precedence over max-age
		w.Header().Set("Cache-Control", "max-age=3600, no-cache")
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)

	// Wait just over 1 second (no-cache expiry)
	time.Sleep(1100 * time.Millisecond)

	// Should fetch again because no-cache uses 1-second expiry
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultFresh, result2)
	assert.Equal(t, 2, data2.Count)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestCachingFetcher_CacheControl_MaxAgeWithNoStore(t *testing.T) {
	callCount := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// no-store should take precedence over max-age
		w.Header().Set("Cache-Control", "max-age=3600, no-store")
		data := testData{Message: "hello", Count: int(callCount.Load())}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	fetcher := NewCachingFetcher[testData](server.URL, CacheConfig{
		StaticExpiry: 1 * time.Hour,
		ReturnStale:  false,
	})

	ctx := context.Background()

	// First call should fetch
	data1, result1, err1 := fetcher.Get(ctx)
	require.NoError(t, err1)
	assert.Equal(t, CacheResultFresh, result1)
	assert.Equal(t, 1, data1.Count)

	// Second call should fetch again (no-store means immediate expiry)
	data2, result2, err2 := fetcher.Get(ctx)
	require.NoError(t, err2)
	assert.Equal(t, CacheResultFresh, result2)
	assert.Equal(t, 2, data2.Count)
	assert.Equal(t, int32(2), callCount.Load())
}
