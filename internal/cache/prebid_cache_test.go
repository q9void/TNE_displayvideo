package cache

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestDefaultCacheConfig verifies default configuration values
func TestDefaultCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()

	if config == nil {
		t.Fatal("DefaultCacheConfig returned nil")
	}

	// Verify default values
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Enabled", config.Enabled, false},
		{"Endpoint", config.Endpoint, "https://prebid-cache.example.com/cache"},
		{"Timeout", config.Timeout, 500 * time.Millisecond},
		{"DefaultTTL", config.DefaultTTL, 300},
		{"CacheBanner", config.CacheBanner, true},
		{"CacheVideo", config.CacheVideo, true},
		{"CacheNative", config.CacheNative, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.got)
			}
		})
	}
}

// TestNewPrebidCache verifies cache client creation
func TestNewPrebidCache(t *testing.T) {
	t.Run("with custom config", func(t *testing.T) {
		config := &CacheConfig{
			Enabled:  true,
			Endpoint: "https://custom-cache.com/cache",
			Timeout:  1 * time.Second,
		}

		cache := NewPrebidCache(config)

		if cache == nil {
			t.Fatal("NewPrebidCache returned nil")
		}
		if cache.config != config {
			t.Error("config not set correctly")
		}
		if cache.httpClient == nil {
			t.Error("httpClient not initialized")
		}
		if cache.httpClient.Timeout != config.Timeout {
			t.Errorf("expected timeout %v, got %v", config.Timeout, cache.httpClient.Timeout)
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		cache := NewPrebidCache(nil)

		if cache == nil {
			t.Fatal("NewPrebidCache returned nil")
		}
		if cache.config == nil {
			t.Fatal("config should be set to defaults")
		}
		if cache.config.Timeout != 500*time.Millisecond {
			t.Errorf("expected default timeout, got %v", cache.config.Timeout)
		}
	})
}

// TestGetCacheType verifies cache type determination based on media type
func TestGetCacheType(t *testing.T) {
	cache := NewPrebidCache(nil)

	tests := []struct {
		mediaType string
		expected  string
	}{
		{"video", "xml"},
		{"banner", "json"},
		{"native", "json"},
		{"audio", "json"},
		{"", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.mediaType, func(t *testing.T) {
			got := cache.getCacheType(tt.mediaType)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestGetCacheValue verifies cache value extraction from bids
func TestGetCacheValue(t *testing.T) {
	cache := NewPrebidCache(nil)

	t.Run("video with NURL", func(t *testing.T) {
		bid := &openrtb.Bid{
			ID:   "bid1",
			NURL: "https://example.com/vast.xml",
			AdM:  "<VAST>...</VAST>",
		}

		value := cache.getCacheValue(bid, "video")
		if value != bid.NURL {
			t.Errorf("expected NURL %q, got %v", bid.NURL, value)
		}
	})

	t.Run("video without NURL uses AdM", func(t *testing.T) {
		bid := &openrtb.Bid{
			ID:   "bid1",
			NURL: "",
			AdM:  "<VAST>...</VAST>",
		}

		value := cache.getCacheValue(bid, "video")
		if value != bid.AdM {
			t.Errorf("expected AdM %q, got %v", bid.AdM, value)
		}
	})

	t.Run("banner uses AdM", func(t *testing.T) {
		bid := &openrtb.Bid{
			ID:   "bid1",
			NURL: "https://example.com/nurl",
			AdM:  "<div>banner creative</div>",
		}

		value := cache.getCacheValue(bid, "banner")
		if value != bid.AdM {
			t.Errorf("expected AdM %q, got %v", bid.AdM, value)
		}
	})

	t.Run("native uses AdM", func(t *testing.T) {
		bid := &openrtb.Bid{
			ID:   "bid1",
			NURL: "https://example.com/nurl",
			AdM:  `{"native":{"ver":"1.2","assets":[]}}`,
		}

		value := cache.getCacheValue(bid, "native")
		if value != bid.AdM {
			t.Errorf("expected AdM %q, got %v", bid.AdM, value)
		}
	})
}

// TestShouldCache verifies media type caching logic
func TestShouldCache(t *testing.T) {
	tests := []struct {
		name         string
		config       *CacheConfig
		mediaType    string
		shouldCache  bool
	}{
		{
			name: "banner enabled",
			config: &CacheConfig{
				CacheBanner: true,
				CacheVideo:  false,
				CacheNative: false,
			},
			mediaType:   "banner",
			shouldCache: true,
		},
		{
			name: "banner disabled",
			config: &CacheConfig{
				CacheBanner: false,
				CacheVideo:  true,
				CacheNative: true,
			},
			mediaType:   "banner",
			shouldCache: false,
		},
		{
			name: "video enabled",
			config: &CacheConfig{
				CacheBanner: false,
				CacheVideo:  true,
				CacheNative: false,
			},
			mediaType:   "video",
			shouldCache: true,
		},
		{
			name: "video disabled",
			config: &CacheConfig{
				CacheBanner: true,
				CacheVideo:  false,
				CacheNative: true,
			},
			mediaType:   "video",
			shouldCache: false,
		},
		{
			name: "native enabled",
			config: &CacheConfig{
				CacheBanner: false,
				CacheVideo:  false,
				CacheNative: true,
			},
			mediaType:   "native",
			shouldCache: true,
		},
		{
			name: "native disabled",
			config: &CacheConfig{
				CacheBanner: true,
				CacheVideo:  true,
				CacheNative: false,
			},
			mediaType:   "native",
			shouldCache: false,
		},
		{
			name: "unknown media type",
			config: &CacheConfig{
				CacheBanner: true,
				CacheVideo:  true,
				CacheNative: true,
			},
			mediaType:   "audio",
			shouldCache: false,
		},
		{
			name: "empty media type",
			config: &CacheConfig{
				CacheBanner: true,
				CacheVideo:  true,
				CacheNative: true,
			},
			mediaType:   "",
			shouldCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewPrebidCache(tt.config)
			got := cache.shouldCache(tt.mediaType)
			if got != tt.shouldCache {
				t.Errorf("expected %v, got %v", tt.shouldCache, got)
			}
		})
	}
}

// TestGetCacheURL verifies cache URL generation
func TestGetCacheURL(t *testing.T) {
	config := &CacheConfig{
		Endpoint: "https://prebid-cache.example.com/cache",
	}
	cache := NewPrebidCache(config)

	tests := []struct {
		name     string
		uuid     string
		expected string
	}{
		{
			name:     "valid uuid",
			uuid:     "abc123-def456",
			expected: "https://prebid-cache.example.com/cache?uuid=abc123-def456",
		},
		{
			name:     "empty uuid",
			uuid:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.GetCacheURL(tt.uuid)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestCacheBid tests single bid caching with HTTP mocking
func TestCacheBid(t *testing.T) {
	t.Run("successful cache", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and headers
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
			}

			// Read and verify request body
			body, _ := io.ReadAll(r.Body)
			var req CacheRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("failed to unmarshal request: %v", err)
			}

			if len(req.Puts) != 1 {
				t.Errorf("expected 1 put, got %d", len(req.Puts))
			}
			if req.Puts[0].Type != "json" {
				t.Errorf("expected type json, got %s", req.Puts[0].Type)
			}
			if req.Puts[0].TTL != 300 {
				t.Errorf("expected TTL 300, got %d", req.Puts[0].TTL)
			}

			// Return successful response
			resp := CacheResponse{
				Responses: []CacheResponseItem{
					{UUID: "test-uuid-123"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			DefaultTTL:  300,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		uuid, err := cache.CacheBid(context.Background(), bid, "banner")
		if err != nil {
			t.Fatalf("CacheBid failed: %v", err)
		}
		if uuid != "test-uuid-123" {
			t.Errorf("expected uuid test-uuid-123, got %s", uuid)
		}
	})

	t.Run("cache disabled", func(t *testing.T) {
		config := &CacheConfig{
			Enabled: false,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		_, err := cache.CacheBid(context.Background(), bid, "banner")
		if err == nil {
			t.Error("expected error when cache disabled")
		}
		if err.Error() != "cache is disabled" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("media type not enabled for caching", func(t *testing.T) {
		config := &CacheConfig{
			Enabled:     true,
			CacheBanner: false,
			CacheVideo:  true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		uuid, err := cache.CacheBid(context.Background(), bid, "banner")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if uuid != "" {
			t.Errorf("expected empty uuid, got %s", uuid)
		}
	})

	t.Run("server returns non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		_, err := cache.CacheBid(context.Background(), bid, "banner")
		if err == nil {
			t.Error("expected error for non-200 status")
		}
		if err != nil && err.Error() != "cache returned status 500" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("server returns invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		_, err := cache.CacheBid(context.Background(), bid, "banner")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("server returns empty responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := CacheResponse{
				Responses: []CacheResponseItem{},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		_, err := cache.CacheBid(context.Background(), bid, "banner")
		if err == nil {
			t.Error("expected error for empty responses")
		}
		if err != nil && err.Error() != "cache returned no responses" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow server
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     10 * time.Millisecond, // Very short timeout
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:  "bid1",
			AdM: "<div>test creative</div>",
		}

		_, err := cache.CacheBid(context.Background(), bid, "banner")
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("video bid with NURL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify request body
			body, _ := io.ReadAll(r.Body)
			var req CacheRequest
			json.Unmarshal(body, &req)

			// Verify video uses xml type
			if req.Puts[0].Type != "xml" {
				t.Errorf("expected type xml for video, got %s", req.Puts[0].Type)
			}

			// Verify NURL is used as value
			if req.Puts[0].Value != "https://example.com/vast.xml" {
				t.Errorf("expected NURL as value, got %v", req.Puts[0].Value)
			}

			resp := CacheResponse{
				Responses: []CacheResponseItem{{UUID: "video-uuid"}},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:    true,
			Endpoint:   server.URL,
			Timeout:    1 * time.Second,
			CacheVideo: true,
		}
		cache := NewPrebidCache(config)

		bid := &openrtb.Bid{
			ID:   "bid1",
			NURL: "https://example.com/vast.xml",
			AdM:  "<VAST>...</VAST>",
		}

		uuid, err := cache.CacheBid(context.Background(), bid, "video")
		if err != nil {
			t.Fatalf("CacheBid failed: %v", err)
		}
		if uuid != "video-uuid" {
			t.Errorf("expected uuid video-uuid, got %s", uuid)
		}
	})
}

// TestCacheBids tests batch bid caching
func TestCacheBids(t *testing.T) {
	t.Run("successful batch cache", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify request
			body, _ := io.ReadAll(r.Body)
			var req CacheRequest
			json.Unmarshal(body, &req)

			if len(req.Puts) != 3 {
				t.Errorf("expected 3 puts, got %d", len(req.Puts))
			}

			// Return UUIDs for each bid
			resp := CacheResponse{
				Responses: []CacheResponseItem{
					{UUID: "uuid-1"},
					{UUID: "uuid-2"},
					{UUID: "uuid-3"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			DefaultTTL:  300,
			CacheBanner: true,
			CacheVideo:  true,
			CacheNative: true,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
			"bid2": {ID: "bid2", AdM: "<div>banner2</div>"},
			"bid3": {ID: "bid3", NURL: "https://example.com/vast.xml"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
			"bid2": "banner",
			"bid3": "video",
		}

		result, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err != nil {
			t.Fatalf("CacheBids failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("expected 3 results, got %d", len(result))
		}

		// Verify all bids have UUIDs
		for bidID := range bids {
			if _, exists := result[bidID]; !exists {
				t.Errorf("missing UUID for bid %s", bidID)
			}
		}
	})

	t.Run("cache disabled returns empty map", func(t *testing.T) {
		config := &CacheConfig{
			Enabled: false,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
		}

		result, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d items", len(result))
		}
	})

	t.Run("empty bids returns empty map", func(t *testing.T) {
		config := &CacheConfig{
			Enabled: true,
		}
		cache := NewPrebidCache(config)

		result, err := cache.CacheBids(context.Background(), map[string]*openrtb.Bid{}, map[string]string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d items", len(result))
		}
	})

	t.Run("filters by media type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify only video bids are cached
			body, _ := io.ReadAll(r.Body)
			var req CacheRequest
			json.Unmarshal(body, &req)

			if len(req.Puts) != 1 {
				t.Errorf("expected 1 put (only video), got %d", len(req.Puts))
			}

			resp := CacheResponse{
				Responses: []CacheResponseItem{{UUID: "video-uuid"}},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: false, // Disable banner caching
			CacheVideo:  true,
			CacheNative: false,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
			"bid2": {ID: "bid2", NURL: "https://example.com/vast.xml"},
			"bid3": {ID: "bid3", AdM: `{"native":{}}`},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
			"bid2": "video",
			"bid3": "native",
		}

		result, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err != nil {
			t.Fatalf("CacheBids failed: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("expected 1 result (only video), got %d", len(result))
		}
		if _, exists := result["bid2"]; !exists {
			t.Error("expected video bid to be cached")
		}
	})

	t.Run("all bids filtered returns empty map", func(t *testing.T) {
		config := &CacheConfig{
			Enabled:     true,
			CacheBanner: false,
			CacheVideo:  false,
			CacheNative: false,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
		}

		result, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d items", len(result))
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
		}

		_, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err == nil {
			t.Error("expected error for server error")
		}
		if err != nil && err.Error() != "cache returned status 502" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("invalid response JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>banner1</div>"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
		}

		_, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("preserves bid order in results", func(t *testing.T) {
		var receivedPuts []CachePut

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var req CacheRequest
			json.Unmarshal(body, &req)
			receivedPuts = req.Puts

			// Return UUIDs in same order
			resp := CacheResponse{
				Responses: []CacheResponseItem{
					{UUID: "uuid-1"},
					{UUID: "uuid-2"},
					{UUID: "uuid-3"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := &CacheConfig{
			Enabled:     true,
			Endpoint:    server.URL,
			Timeout:     1 * time.Second,
			CacheBanner: true,
		}
		cache := NewPrebidCache(config)

		bids := map[string]*openrtb.Bid{
			"bid1": {ID: "bid1", AdM: "<div>creative1</div>"},
			"bid2": {ID: "bid2", AdM: "<div>creative2</div>"},
			"bid3": {ID: "bid3", AdM: "<div>creative3</div>"},
		}
		mediaTypes := map[string]string{
			"bid1": "banner",
			"bid2": "banner",
			"bid3": "banner",
		}

		result, err := cache.CacheBids(context.Background(), bids, mediaTypes)
		if err != nil {
			t.Fatalf("CacheBids failed: %v", err)
		}

		// Verify we got results for all bids
		if len(result) != 3 {
			t.Errorf("expected 3 results, got %d", len(result))
		}

		// Verify the puts were sent
		if len(receivedPuts) != 3 {
			t.Errorf("expected 3 puts sent, got %d", len(receivedPuts))
		}
	})
}
