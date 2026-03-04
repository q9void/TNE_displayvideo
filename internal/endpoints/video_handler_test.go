package endpoints

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// mockVideoBidder implements Bidder interface for video handler testing
type mockVideoBidder struct {
	bids []*adapters.TypedBid
	err  error
}

func (m *mockVideoBidder) MakeRequests(request *openrtb.BidRequest, reqInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	if m.err != nil {
		return nil, []error{m.err}
	}
	return []*adapters.RequestData{{Method: "POST", URI: "http://test.com", Body: []byte("{}")}}, nil
}

func (m *mockVideoBidder) MakeBids(request *openrtb.BidRequest, response *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if m.err != nil {
		return nil, []error{m.err}
	}
	return &adapters.BidderResponse{Bids: m.bids, Currency: "USD"}, nil
}

// Helper function to create test exchange with video adapter
func newTestVideoExchange() *exchange.Exchange {
	registry := adapters.NewRegistry()
	// Register a mock bidder for testing
	mockBidder := &mockVideoBidder{
		bids: []*adapters.TypedBid{
			{
				Bid: &openrtb.Bid{
					ID:    "bid-1",
					ImpID: "1",
					Price: 2.50,
					AdM:   "http://example.com/video.mp4",
					AdID:  "ad-123",
					NURL:  "http://example.com/win",
				},
				BidType: adapters.BidTypeVideo,
			},
		},
	}

	bidderInfo := adapters.BidderInfo{
		Enabled: true,
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{adapters.BidTypeVideo},
			},
		},
	}

	registry.Register("testbidder", mockBidder, bidderInfo)

	return exchange.New(registry, &exchange.Config{
		DefaultTimeout: 100 * time.Millisecond,
	})
}

// Helper function to create empty test exchange
func newEmptyTestVideoExchange() *exchange.Exchange {
	registry := adapters.NewRegistry()
	return exchange.New(registry, &exchange.Config{
		DefaultTimeout: 100 * time.Millisecond,
	})
}

func TestNewVideoHandler(t *testing.T) {
	registry := adapters.NewRegistry()
	ex := exchange.New(registry, &exchange.Config{
		DefaultTimeout: 100 * time.Millisecond,
	})
	trackingURL := "https://track.example.com"

	handler := NewVideoHandler(ex, trackingURL, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.exchange != ex {
		t.Error("expected exchange to be set")
	}
	if handler.trackingBaseURL != trackingURL {
		t.Errorf("expected trackingBaseURL to be %s, got %s", trackingURL, handler.trackingBaseURL)
	}
	if handler.vastBuilder == nil {
		t.Error("expected vastBuilder to be initialized")
	}
}

func TestHandleVASTRequest_MethodNotAllowed(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/video/vast?id=test-1", nil)
			w := httptest.NewRecorder()

			handler.HandleVASTRequest(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
			}
		})
	}
}

func TestHandleVASTRequest_ValidRequest(t *testing.T) {
	ex := newTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	queryParams := url.Values{
		"id":          {"test-request-1"},
		"w":           {"1920"},
		"h":           {"1080"},
		"mindur":      {"5"},
		"maxdur":      {"30"},
		"placement":   {"1"},
		"protocols":   {"2,3,5,6"},
		"mimes":       {"video/mp4,video/webm"},
		"minbitrate":  {"300"},
		"maxbitrate":  {"5000"},
		"bidfloor":    {"1.50"},
		"site_id":     {"site-123"},
		"domain":      {"example.com"},
		"page":        {"https://example.com/page"},
	}

	req := httptest.NewRequest(http.MethodGet, "/video/vast?"+queryParams.Encode(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/xml") {
		t.Errorf("expected Content-Type to contain application/xml, got %s", contentType)
	}

	// Check CORS headers
	corsOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if corsOrigin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be *, got %s", corsOrigin)
	}

	// Verify VAST response is valid XML
	body := w.Body.String()
	if !strings.Contains(body, "<VAST") {
		t.Error("expected response to contain VAST XML")
	}
}

func TestHandleVASTRequest_MinimalParameters(t *testing.T) {
	ex := newTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	// Request with minimal parameters - should use defaults
	req := httptest.NewRequest(http.MethodGet, "/video/vast", nil)
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify defaults were applied
	body := w.Body.String()
	if !strings.Contains(body, "<VAST") {
		t.Error("expected response to contain VAST XML")
	}
}

func TestHandleVASTRequest_AuctionError(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	req := httptest.NewRequest(http.MethodGet, "/video/vast?id=test-1", nil)
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	// VAST always returns 200, even for errors
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<Error>") {
		t.Error("expected VAST error response")
	}
}

func TestHandleVASTRequest_EmptyAuction(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	req := httptest.NewRequest(http.MethodGet, "/video/vast?id=test-1", nil)
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<VAST") {
		t.Error("expected response to contain VAST XML")
	}
}

func TestHandleVASTRequest_SkippableAd(t *testing.T) {
	ex := newTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	queryParams := url.Values{
		"id":        {"test-skip"},
		"skip":      {"1"},
		"skipafter": {"5"},
	}

	req := httptest.NewRequest(http.MethodGet, "/video/vast?"+queryParams.Encode(), nil)
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<VAST") {
		t.Error("expected response to contain VAST XML")
	}
}

func TestHandleVASTRequest_CTVOptimization(t *testing.T) {
	ex := newTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	// Request with Roku user agent
	req := httptest.NewRequest(http.MethodGet, "/video/vast?id=test-ctv", nil)
	req.Header.Set("User-Agent", "Roku/DVP-9.10 (519.10E04111A)")
	w := httptest.NewRecorder()

	handler.HandleVASTRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleOpenRTBVideo_MethodNotAllowed(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/video/openrtb", nil)
			w := httptest.NewRecorder()

			handler.HandleOpenRTBVideo(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
			}
		})
	}
}

func TestHandleOpenRTBVideo_ValidRequest(t *testing.T) {
	ex := newTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	bidReq := &openrtb.BidRequest{
		ID: "test-openrtb-1",
		Imp: []openrtb.Imp{
			{
				ID: "1",
				Video: &openrtb.Video{
					Mimes:       []string{"video/mp4"},
					MinDuration: 5,
					MaxDuration: 30,
					W:           1920,
					H:           1080,
					Protocols:   []int{2, 3, 5, 6},
				},
				BidFloor:    1.00,
				BidFloorCur: "USD",
			},
		},
		Site: &openrtb.Site{
			ID:     "site-1",
			Domain: "example.com",
		},
		TMax: 1000,
	}

	body, err := json.Marshal(bidReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/video/openrtb", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleOpenRTBVideo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/xml") {
		t.Errorf("expected Content-Type to contain application/xml, got %s", contentType)
	}

	// Check CORS headers
	corsOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if corsOrigin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be *, got %s", corsOrigin)
	}

	// Verify VAST response
	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "<VAST") {
		t.Error("expected response to contain VAST XML")
	}
}

func TestHandleOpenRTBVideo_InvalidJSON(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	req := httptest.NewRequest(http.MethodPost, "/video/openrtb", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleOpenRTBVideo(w, req)

	// VAST always returns 200, even for errors
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<Error>") {
		t.Error("expected VAST error response")
	}
}

func TestHandleOpenRTBVideo_NoVideoImpressions(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	// Request with banner impression (no video)
	bidReq := &openrtb.BidRequest{
		ID: "test-no-video",
		Imp: []openrtb.Imp{
			{
				ID: "1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
	}

	body, err := json.Marshal(bidReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/video/openrtb", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleOpenRTBVideo(w, req)

	// VAST always returns 200
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "<Error>") {
		t.Error("expected VAST error response for non-video request")
	}
}

func TestHandleOpenRTBVideo_AuctionError(t *testing.T) {
	ex := newEmptyTestVideoExchange()
	handler := NewVideoHandler(ex, "https://track.example.com", nil)

	bidReq := &openrtb.BidRequest{
		ID: "test-error",
		Imp: []openrtb.Imp{
			{
				ID: "1",
				Video: &openrtb.Video{
					Mimes:       []string{"video/mp4"},
					MinDuration: 5,
					MaxDuration: 30,
				},
			},
		},
	}

	body, err := json.Marshal(bidReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/video/openrtb", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleOpenRTBVideo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "<Error>") {
		t.Error("expected VAST error response")
	}
}

func TestSetVASTCORSHeaders(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	w := httptest.NewRecorder()
	handler.setVASTCORSHeaders(w)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", origin)
	}

	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods != "GET, POST, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods: GET, POST, OPTIONS, got %s", methods)
	}

	if headers := w.Header().Get("Access-Control-Allow-Headers"); headers != "Content-Type, Accept" {
		t.Errorf("expected Access-Control-Allow-Headers: Content-Type, Accept, got %s", headers)
	}
}

func TestParseVASTRequest_Defaults(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/video/vast", nil)
	bidReq, err := handler.parseVASTRequest(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bidReq == nil {
		t.Fatal("expected non-nil bid request")
	}

	// Check defaults
	if len(bidReq.Imp) != 1 {
		t.Errorf("expected 1 impression, got %d", len(bidReq.Imp))
	}

	imp := bidReq.Imp[0]
	if imp.Video == nil {
		t.Fatal("expected video impression")
	}

	if imp.Video.W != 1920 {
		t.Errorf("expected default width 1920, got %d", imp.Video.W)
	}

	if imp.Video.H != 1080 {
		t.Errorf("expected default height 1080, got %d", imp.Video.H)
	}

	if imp.Video.MinDuration != 5 {
		t.Errorf("expected default min duration 5, got %d", imp.Video.MinDuration)
	}

	if imp.Video.MaxDuration != 30 {
		t.Errorf("expected default max duration 30, got %d", imp.Video.MaxDuration)
	}

	if imp.Video.Placement != 1 {
		t.Errorf("expected default placement 1, got %d", imp.Video.Placement)
	}
}

func TestParseVASTRequest_CustomValues(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	queryParams := url.Values{
		"id":          {"custom-id"},
		"w":           {"640"},
		"h":           {"480"},
		"mindur":      {"10"},
		"maxdur":      {"60"},
		"placement":   {"3"},
		"protocols":   {"2,5"},
		"mimes":       {"video/mp4"},
		"minbitrate":  {"500"},
		"maxbitrate":  {"2500"},
		"bidfloor":    {"2.00"},
		"site_id":     {"site-abc"},
		"domain":      {"test.com"},
		"page":        {"https://test.com/video"},
	}

	req := httptest.NewRequest(http.MethodGet, "/video/vast?"+queryParams.Encode(), nil)
	bidReq, err := handler.parseVASTRequest(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bidReq.ID != "custom-id" {
		t.Errorf("expected ID custom-id, got %s", bidReq.ID)
	}

	imp := bidReq.Imp[0]
	if imp.Video.W != 640 {
		t.Errorf("expected width 640, got %d", imp.Video.W)
	}

	if imp.Video.H != 480 {
		t.Errorf("expected height 480, got %d", imp.Video.H)
	}

	if imp.Video.MinDuration != 10 {
		t.Errorf("expected min duration 10, got %d", imp.Video.MinDuration)
	}

	if imp.Video.MaxDuration != 60 {
		t.Errorf("expected max duration 60, got %d", imp.Video.MaxDuration)
	}

	if imp.Video.Placement != 3 {
		t.Errorf("expected placement 3, got %d", imp.Video.Placement)
	}

	if imp.BidFloor != 2.00 {
		t.Errorf("expected bid floor 2.00, got %f", imp.BidFloor)
	}

	if bidReq.Site == nil {
		t.Fatal("expected site to be set")
	}

	if bidReq.Site.ID != "site-abc" {
		t.Errorf("expected site ID site-abc, got %s", bidReq.Site.ID)
	}

	if bidReq.Site.Domain != "test.com" {
		t.Errorf("expected domain test.com, got %s", bidReq.Site.Domain)
	}
}

func TestParseVASTRequest_OnlyDomain(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	queryParams := url.Values{
		"domain": {"example.com"},
	}

	req := httptest.NewRequest(http.MethodGet, "/video/vast?"+queryParams.Encode(), nil)
	bidReq, err := handler.parseVASTRequest(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bidReq.Site == nil {
		t.Fatal("expected site to be set when domain is provided")
	}

	if bidReq.Site.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", bidReq.Site.Domain)
	}
}

func TestWriteVASTError_URLInjectionPrevention(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	// Test with malicious input that should be escaped
	maliciousInputs := []string{
		"test<script>alert('xss')</script>",
		"test&foo=bar",
		"test?newparam=value",
		"test#fragment",
		"test%00null",
	}

	for _, input := range maliciousInputs {
		t.Run(input, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.writeVASTError(w, input)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			body := w.Body.String()

			// Verify XML structure
			if !strings.Contains(body, "<Error>") {
				t.Error("expected Error element in VAST XML")
			}

			// Verify URL encoding was applied (should not contain raw special chars)
			if strings.Contains(body, "<script>") {
				t.Error("response contains unescaped script tag")
			}

			// Verify it's valid VAST XML (just check it contains expected elements)
			if !strings.Contains(body, "<?xml") {
				t.Error("response should contain XML declaration")
			}
		})
	}
}

func TestWriteVASTError_Headers(t *testing.T) {
	handler := &VideoHandler{
		trackingBaseURL: "https://track.example.com",
	}

	w := httptest.NewRecorder()
	handler.writeVASTError(w, "test error")

	if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "application/xml") {
		t.Errorf("expected Content-Type to contain application/xml, got %s", contentType)
	}

	if cors := w.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", cors)
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetClientIP(t *testing.T) {
	t.Setenv("TRUST_X_FORWARDED_FOR", "true")
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:          "X-Forwarded-For single IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:          "X-Forwarded-For multiple IPs",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 198.51.100.1, 192.0.2.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.1",
			expectedIP: "203.0.113.1",
		},
		{
			name:          "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			xRealIP:       "198.51.100.1",
			expectedIP:    "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestParseHelperFunctions(t *testing.T) {
	t.Run("parseInt", func(t *testing.T) {
		tests := []struct {
			input    string
			defVal   int
			expected int
		}{
			{"", 10, 10},
			{"42", 10, 42},
			{"invalid", 10, 10},
			{"0", 10, 0},
			{"-5", 10, -5},
		}

		for _, tt := range tests {
			result := parseInt(tt.input, tt.defVal)
			if result != tt.expected {
				t.Errorf("parseInt(%q, %d) = %d, want %d", tt.input, tt.defVal, result, tt.expected)
			}
		}
	})

	t.Run("parseFloat", func(t *testing.T) {
		tests := []struct {
			input    string
			defVal   float64
			expected float64
		}{
			{"", 1.0, 1.0},
			{"2.5", 1.0, 2.5},
			{"invalid", 1.0, 1.0},
			{"0.0", 1.0, 0.0},
		}

		for _, tt := range tests {
			result := parseFloat(tt.input, tt.defVal)
			if result != tt.expected {
				t.Errorf("parseFloat(%q, %f) = %f, want %f", tt.input, tt.defVal, result, tt.expected)
			}
		}
	})

	t.Run("parseIntArray", func(t *testing.T) {
		tests := []struct {
			input    string
			defVal   []int
			expected []int
		}{
			{"", []int{1, 2}, []int{1, 2}},
			{"3,4,5", []int{1, 2}, []int{3, 4, 5}},
			{"10", []int{1, 2}, []int{10}},
			{"invalid", []int{1, 2}, []int{1, 2}},
			{"1,invalid,3", []int{}, []int{1, 3}},
		}

		for _, tt := range tests {
			result := parseIntArray(tt.input, tt.defVal)
			if len(result) != len(tt.expected) {
				t.Errorf("parseIntArray(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				continue
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("parseIntArray(%q)[%d] = %d, want %d", tt.input, i, result[i], tt.expected[i])
				}
			}
		}
	})

	t.Run("parseStringArray", func(t *testing.T) {
		tests := []struct {
			input    string
			defVal   []string
			expected []string
		}{
			{"", []string{"a", "b"}, []string{"a", "b"}},
			{"x,y,z", []string{"a"}, []string{"x", "y", "z"}},
			{"single", []string{}, []string{"single"}},
			{"a,,b", []string{}, []string{"a", "b"}}, // empty strings filtered
		}

		for _, tt := range tests {
			result := parseStringArray(tt.input, tt.defVal)
			if len(result) != len(tt.expected) {
				t.Errorf("parseStringArray(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				continue
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("parseStringArray(%q)[%d] = %s, want %s", tt.input, i, result[i], tt.expected[i])
				}
			}
		}
	})
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == id2 {
		t.Error("expected unique request IDs")
	}

	if !strings.HasPrefix(id1, "video-") {
		t.Errorf("expected ID to start with 'video-', got %s", id1)
	}
}
