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
)

// TestCatalystBidHandler_ValidRequest tests a valid bid request
func TestCatalystBidHandler_ValidRequest(t *testing.T) {
	// Setup exchange
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)

	// Create handler
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Create test request
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "test-account-123",
		Timeout:   2800,
		Slots: []endpoints.MAISlot{
			{
				DivID: "ad-slot-1",
				Sizes: [][]int{{728, 90}, {970, 250}},
				AdUnitPath: "/test/leaderboard",
				Position: "atf",
			},
		},
		Page: &endpoints.MAIPage{
			URL:    "https://example.com/article",
			Domain: "example.com",
			Keywords: []string{"sports", "football"},
		},
		Device: &endpoints.MAIDevice{
			Width:  1920,
			Height: 1080,
			DeviceType: "desktop",
		},
	}

	body, err := json.Marshal(bidRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleBidRequest(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response endpoints.MAIBidResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Bids == nil {
		t.Error("Expected bids array, got nil")
	}

	if response.ResponseTime < 0 {
		t.Error("Expected non-negative response time")
	}

	t.Logf("Response: %d bids in %dms", len(response.Bids), response.ResponseTime)
}

// TestCatalystBidHandler_InvalidRequest tests invalid request handling
func TestCatalystBidHandler_InvalidRequest(t *testing.T) {
	tests := []struct {
		name    string
		request interface{}
		wantErr bool
	}{
		{
			name:    "missing account ID",
			request: endpoints.MAIBidRequest{
				Slots: []endpoints.MAISlot{
					{DivID: "test", Sizes: [][]int{{300, 250}}},
				},
			},
			wantErr: true,
		},
		{
			name: "empty slots",
			request: endpoints.MAIBidRequest{
				AccountID: "test",
				Slots:     []endpoints.MAISlot{},
			},
			wantErr: true,
		},
		{
			name: "invalid slot - missing divId",
			request: endpoints.MAIBidRequest{
				AccountID: "test",
				Slots: []endpoints.MAISlot{
					{Sizes: [][]int{{300, 250}}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid slot - missing sizes",
			request: endpoints.MAIBidRequest{
				AccountID: "test",
				Slots: []endpoints.MAISlot{
					{DivID: "test", Sizes: [][]int{}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid size format",
			request: endpoints.MAIBidRequest{
				AccountID: "test",
				Slots: []endpoints.MAISlot{
					{DivID: "test", Sizes: [][]int{{300}}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			registry := adapters.DefaultRegistry
			exchangeConfig := &exchange.Config{
				DefaultTimeout: 2500 * time.Millisecond,
			}
			ex := exchange.New(registry, exchangeConfig)
			handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

			// Create request
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			handler.HandleBidRequest(w, req)

			// Check
			if tt.wantErr && w.Code == http.StatusOK {
				t.Error("Expected error status, got 200")
			}
		})
	}
}

// TestCatalystBidHandler_Timeout tests timeout handling
func TestCatalystBidHandler_Timeout(t *testing.T) {
	// Setup exchange with very short timeout
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 1 * time.Millisecond, // Very short for testing
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Create request
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "test-timeout",
		Timeout:   1, // 1ms timeout
		Slots: []endpoints.MAISlot{
			{
				DivID: "ad-slot-timeout",
				Sizes: [][]int{{300, 250}},
			},
		},
	}

	body, _ := json.Marshal(bidRequest)
	req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleBidRequest(w, req)

	// Should return empty bids on timeout, not error
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even on timeout, got %d", w.Code)
	}

	var response endpoints.MAIBidResponse
	json.NewDecoder(w.Body).Decode(&response)

	if len(response.Bids) > 0 {
		t.Error("Expected empty bids array on timeout")
	}
}

// TestCatalystBidHandler_CORS tests CORS headers
func TestCatalystBidHandler_CORS(t *testing.T) {
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Test OPTIONS preflight
	req := httptest.NewRequest("OPTIONS", "/v1/bid", nil)
	w := httptest.NewRecorder()

	handler.HandleBidRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS Allow-Origin header")
	}
}

// TestCatalystBidHandler_MultipleSlots tests multiple ad slots
func TestCatalystBidHandler_MultipleSlots(t *testing.T) {
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Create request with 3 slots
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "test-multi-slot",
		Slots: []endpoints.MAISlot{
			{DivID: "slot-1", Sizes: [][]int{{728, 90}}},
			{DivID: "slot-2", Sizes: [][]int{{300, 250}}},
			{DivID: "slot-3", Sizes: [][]int{{160, 600}}},
		},
	}

	body, _ := json.Marshal(bidRequest)
	req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleBidRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response endpoints.MAIBidResponse
	json.NewDecoder(w.Body).Decode(&response)

	// Should handle multiple slots without error
	if response.Bids == nil {
		t.Error("Expected bids array for multiple slots")
	}
}

// TestCatalystBidHandler_PrivacyConsent tests privacy consent handling
func TestCatalystBidHandler_PrivacyConsent(t *testing.T) {
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
		MaxBidders:     10,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Create request with privacy consent
	bidRequest := endpoints.MAIBidRequest{
		AccountID: "test-privacy",
		Slots: []endpoints.MAISlot{
			{DivID: "ad-slot", Sizes: [][]int{{300, 250}}},
		},
		User: &endpoints.MAIUser{
			ConsentGiven: true,
			GDPRApplies:  func() *bool { b := true; return &b }(),
			USPConsent:   "1YNN",
		},
	}

	body, _ := json.Marshal(bidRequest)
	req := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleBidRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestOpenRTBConversion tests MAI to OpenRTB conversion
func TestOpenRTBConversion(t *testing.T) {
	registry := adapters.DefaultRegistry
	exchangeConfig := &exchange.Config{
		DefaultTimeout: 2500 * time.Millisecond,
	}
	ex := exchange.New(registry, exchangeConfig)
	handler := endpoints.NewCatalystBidHandler(ex, nil, nil, nil, nil)

	// Create MAI request
	maiReq := &endpoints.MAIBidRequest{
		AccountID: "test-conversion",
		Timeout:   2800,
		Slots: []endpoints.MAISlot{
			{
				DivID:      "test-slot",
				Sizes:      [][]int{{728, 90}, {970, 250}},
				AdUnitPath: "/test/path",
			},
		},
		Page: &endpoints.MAIPage{
			URL:        "https://example.com",
			Domain:     "example.com",
			Keywords:   []string{"test"},
			Categories: []string{"IAB1"},
		},
	}

	// Convert to OpenRTB (this is a private method, so we test via public API)
	body, _ := json.Marshal(maiReq)
	httpReq := httptest.NewRequest("POST", "/v1/bid", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleBidRequest(w, httpReq)

	// If conversion worked, we should get 200
	if w.Code != http.StatusOK {
		t.Errorf("OpenRTB conversion failed with status %d", w.Code)
	}
}
