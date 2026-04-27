package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/endpoints"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
)

// TestCatalystIntegration_EndToEnd tests the complete Catalyst integration flow
func TestCatalystIntegration_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup full exchange with real adapters
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout:       2500 * time.Millisecond,
		MaxBidders:           50,
		MaxConcurrentBidders: 10,
	}
	ex := exchange.New(registry, exchangeConfig)

	// Create handler
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil, nil)

	// Simulate a real MAI Publisher bid request
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "mai-publisher-12345",
		Timeout:   2800,
		Slots: []endpoints.MAISlot{
			{
				DivID:      "mai-ad-leaderboard",
				Sizes:      [][]int{{728, 90}, {970, 250}},
				AdUnitPath: "/123456/homepage/leaderboard",
				Position:   "atf",
			},
			{
				DivID:      "mai-ad-rectangle",
				Sizes:      [][]int{{300, 250}},
				AdUnitPath: "/123456/homepage/rectangle",
				Position:   "btf",
			},
		},
		Page: &endpoints.MAIPage{
			URL:        "https://example.com/article/12345",
			Domain:     "example.com",
			Keywords:   []string{"sports", "football"},
			Categories: []string{"IAB17", "IAB17-2"},
		},
		User: &endpoints.MAIUser{
			ConsentGiven: true,
			GDPRApplies:  func() *bool { b := true; return &b }(),
			USPConsent:   "1YNN",
		},
		Device: &endpoints.MAIDevice{
			Width:      1920,
			Height:     1080,
			DeviceType: "desktop",
			UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		},
	}

	body, err := json.Marshal(bidRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.RemoteAddr = "203.0.113.1:12345"
	w := httptest.NewRecorder()

	startTime := time.Now()

	// Execute request
	handler.HandleBidRequest(w, req)

	elapsed := time.Since(startTime)

	// Verify response
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var response endpoints.MAIBidResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Bids == nil {
		t.Fatal("Expected bids array, got nil")
	}

	if response.ResponseTime < 0 {
		t.Error("Expected non-negative response time")
	}

	// Verify SLA compliance
	if elapsed > 2500*time.Millisecond {
		t.Errorf("Response time %v exceeds 2500ms SLA", elapsed)
	}

	// Verify response metadata
	if response.ResponseTime > 2500 {
		t.Errorf("Response time %dms in response body exceeds 2500ms", response.ResponseTime)
	}

	// Log results
	t.Logf("Integration test completed successfully:")
	t.Logf("  Request ID: %s", bidRequest.AccountID)
	t.Logf("  Slots requested: %d", len(bidRequest.Slots))
	t.Logf("  Bids returned: %d", len(response.Bids))
	t.Logf("  Response time: %v", elapsed)
	t.Logf("  Server reported time: %dms", response.ResponseTime)

	// Verify bid format
	for i, bid := range response.Bids {
		t.Logf("  Bid %d: divId=%s, cpm=%.2f, size=%dx%d, adId=%s",
			i+1, bid.DivID, bid.CPM, bid.Width, bid.Height, bid.AdID)

		// Validate bid fields
		if bid.DivID == "" {
			t.Errorf("Bid %d: missing divId", i)
		}
		if bid.CPM < 0 {
			t.Errorf("Bid %d: invalid CPM %.2f", i, bid.CPM)
		}
		if bid.Width <= 0 || bid.Height <= 0 {
			t.Errorf("Bid %d: invalid dimensions %dx%d", i, bid.Width, bid.Height)
		}
		if bid.Currency == "" {
			t.Errorf("Bid %d: missing currency", i)
		}
	}
}

// TestCatalystIntegration_HighLoad tests behavior under high load
func TestCatalystIntegration_HighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	// Setup exchange
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout:       2500 * time.Millisecond,
		MaxBidders:           50,
		MaxConcurrentBidders: 10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil, nil)

	// Create bid request
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "load-test-account",
		Timeout:   2800,
		Slots: []endpoints.MAISlot{
			{DivID: "ad-slot-1", Sizes: [][]int{{728, 90}}},
		},
	}

	numRequests := 100
	results := make(chan time.Duration, numRequests)
	errors := make(chan error, numRequests)

	startTime := time.Now()

	// Send concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			reqStart := time.Now()

			body, _ := json.Marshal(bidRequest)
			req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleBidRequest(w, req)

			elapsed := time.Since(reqStart)
			results <- elapsed

			if w.Code != http.StatusOK {
				errors <- nil // Count as error
			}
		}()
	}

	// Collect results
	var totalTime time.Duration
	var maxTime time.Duration
	minTime := time.Hour
	errorCount := 0
	timeoutCount := 0

	for i := 0; i < numRequests; i++ {
		select {
		case elapsed := <-results:
			totalTime += elapsed
			if elapsed > maxTime {
				maxTime = elapsed
			}
			if elapsed < minTime {
				minTime = elapsed
			}
			if elapsed > 2500*time.Millisecond {
				timeoutCount++
			}
		case <-errors:
			errorCount++
		}
	}

	totalElapsed := time.Since(startTime)
	avgTime := totalTime / time.Duration(numRequests)

	// Log results
	t.Logf("Load test results (%d requests):", numRequests)
	t.Logf("  Total time: %v", totalElapsed)
	t.Logf("  Avg response: %v", avgTime)
	t.Logf("  Min response: %v", minTime)
	t.Logf("  Max response: %v", maxTime)
	t.Logf("  Errors: %d (%.1f%%)", errorCount, float64(errorCount)/float64(numRequests)*100)
	t.Logf("  Timeouts: %d (%.1f%%)", timeoutCount, float64(timeoutCount)/float64(numRequests)*100)

	// Verify SLA compliance
	if errorCount > numRequests/100 { // Allow 1% error rate
		t.Errorf("Error rate %.1f%% exceeds 1%% SLA target", float64(errorCount)/float64(numRequests)*100)
	}

	if timeoutCount > numRequests/20 { // Allow 5% timeout rate
		t.Errorf("Timeout rate %.1f%% exceeds 5%% SLA target", float64(timeoutCount)/float64(numRequests)*100)
	}

	if avgTime > 2500*time.Millisecond {
		t.Errorf("Average response time %v exceeds 2500ms SLA", avgTime)
	}
}

// TestCatalystIntegration_SDKCompatibility tests SDK and server compatibility
func TestCatalystIntegration_SDKCompatibility(t *testing.T) {
	// Setup server
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil, nil)

	// Wrap with CORS middleware (mirrors cmd/server/server.go). Configure with an
	// explicit allowed origin since the production middleware no longer falls back
	// to a wildcard (P1-3 hardening in internal/middleware/cors.go).
	const sdkOrigin = "https://example.com"
	cors := middleware.NewCORS(&middleware.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{sdkOrigin},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Origin"},
	})
	server := httptest.NewServer(cors.Middleware(http.HandlerFunc(handler.HandleBidRequest)))
	defer server.Close()

	// Simulate SDK request (what the JavaScript would send)
	sdkRequest := map[string]interface{}{
		"accountId": "sdk-test-account",
		"timeout":   2800,
		"slots": []map[string]interface{}{
			{
				"divId": "test-slot",
				"sizes": [][]int{{300, 250}},
			},
		},
		"page": map[string]interface{}{
			"url":    "https://example.com/test",
			"domain": "example.com",
		},
		"device": map[string]interface{}{
			"width":      1920,
			"height":     1080,
			"deviceType": "desktop",
			"userAgent":  "Mozilla/5.0",
		},
	}

	body, _ := json.Marshal(sdkRequest)

	// Make request with Origin header so the CORS middleware can echo it back
	req, err := http.NewRequest("POST", server.URL+"/v1/bid", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Build request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", sdkOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify CORS headers (middleware echoes the request's Origin when allowed)
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != sdkOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, sdkOrigin)
	}

	// Parse response
	var bidResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&bidResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify SDK-expected fields
	if _, ok := bidResponse["bids"]; !ok {
		t.Error("Response missing 'bids' field expected by SDK")
	}

	if _, ok := bidResponse["responseTime"]; !ok {
		t.Error("Response missing 'responseTime' field expected by SDK")
	}

	t.Logf("SDK compatibility test passed")
}

// BenchmarkCatalystBidRequest benchmarks bid request performance
func BenchmarkCatalystBidRequest(b *testing.B) {
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil, nil)

	bidRequest := endpoints.MAIBidRequest{
		AccountID: "benchmark-account",
		Slots: []endpoints.MAISlot{
			{DivID: "bench-slot", Sizes: [][]int{{728, 90}}},
		},
	}

	body, _ := json.Marshal(bidRequest)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.HandleBidRequest(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Request failed with status %d", w.Code)
		}
	}
}
