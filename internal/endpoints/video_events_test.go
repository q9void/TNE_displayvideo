package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/pkg/vast"
)

// mockVideoAnalytics implements analytics.Module — collects VideoObjects so
// tests can assert on what reaches the analytics pipeline.
type mockVideoAnalytics struct {
	events      []*analytics.VideoObject
	shouldError bool
}

func (m *mockVideoAnalytics) LogAuctionObject(_ context.Context, _ *analytics.AuctionObject) error {
	return nil
}

func (m *mockVideoAnalytics) LogVideoObject(_ context.Context, video *analytics.VideoObject) error {
	if m.shouldError {
		return errors.New("tracking failed")
	}
	m.events = append(m.events, video)
	return nil
}

func (m *mockVideoAnalytics) Shutdown() error { return nil }

func TestNewVideoEventHandler(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.analytics != analytics {
		t.Error("expected analytics to be set")
	}
}

func TestHandleVideoEvent_POST_ValidRequest(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		Event:     "start",
		BidID:     "bid-123",
		AccountID: "account-456",
		Bidder:    "testbidder",
		SessionID: "session-789",
		ContentID: "content-abc",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify response
	var resp VideoEventResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", resp.Status)
	}

	// Verify event was tracked
	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.BidID != "bid-123" {
		t.Errorf("expected BidID bid-123, got %s", event.BidID)
	}
	if event.AccountID != "account-456" {
		t.Errorf("expected AccountID account-456, got %s", event.AccountID)
	}
	if event.Bidder != "testbidder" {
		t.Errorf("expected Bidder testbidder, got %s", event.Bidder)
	}
}

func TestHandleVideoEvent_POST_InvalidJSON(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleVideoEvent_POST_MissingRequiredFields(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	tests := []struct {
		name string
		req  VideoEventRequest
	}{
		{
			name: "missing event",
			req: VideoEventRequest{
				BidID:     "bid-123",
				AccountID: "account-456",
			},
		},
		{
			name: "missing bid_id",
			req: VideoEventRequest{
				Event:     "start",
				AccountID: "account-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleVideoEvent(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 500, got %d", w.Code)
			}
		})
	}
}

func TestHandleVideoEvent_POST_TrackingError(t *testing.T) {
	analytics := &mockVideoAnalytics{shouldError: true}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		Event: "start",
		BidID: "bid-123",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleVideoEvent_POST_WithTimestamp(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	timestamp := int64(1640000000000) // Specific timestamp
	eventReq := VideoEventRequest{
		Event:     "start",
		BidID:     "bid-123",
		Timestamp: timestamp,
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Timestamp.UnixMilli() != timestamp {
		t.Errorf("expected timestamp %d, got %d", timestamp, event.Timestamp.UnixMilli())
	}
}

func TestHandleVideoEvent_GET_ValidRequest(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	queryParams := url.Values{
		"event":      {"impression"},
		"bid_id":     {"bid-123"},
		"account_id": {"account-456"},
		"bidder":     {"testbidder"},
		"session_id": {"session-789"},
		"content_id": {"content-abc"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/video/event?"+queryParams.Encode(), nil)
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify response is tracking pixel (GIF)
	contentType := w.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type image/gif, got %s", contentType)
	}

	// Verify event was tracked
	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.BidID != "bid-123" {
		t.Errorf("expected BidID bid-123, got %s", event.BidID)
	}
}

func TestHandleVideoEvent_GET_ErrorReturnsPixel(t *testing.T) {
	analytics := &mockVideoAnalytics{shouldError: true}
	handler := NewVideoEventHandler(analytics)

	queryParams := url.Values{
		"event":  {"start"},
		"bid_id": {"bid-123"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/video/event?"+queryParams.Encode(), nil)
	w := httptest.NewRecorder()

	handler.HandleVideoEvent(w, req)

	// Should still return 200 with tracking pixel
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type image/gif, got %s", contentType)
	}
}

func TestHandleVideoEvent_MethodNotAllowed(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/video/event?event=start&bid_id=bid-123", nil)
			w := httptest.NewRecorder()

			handler.HandleVideoEvent(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
			}
		})
	}
}

func TestHandleVideoEvent_ConsentValidation(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	tests := []struct {
		name           string
		withConsent    bool
		expectedIP     string
		expectedUA     string
	}{
		{
			name:           "with consent",
			withConsent:    true,
			expectedIP:     "192.168.1.0", // Anonymized
			expectedUA:     "Mozilla/5.0",
		},
		{
			name:           "without consent",
			withConsent:    false,
			expectedIP:     "",
			expectedUA:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analytics.events = nil // Reset events

			eventReq := VideoEventRequest{
				Event: "start",
				BidID: "bid-123",
			}

			body, err := json.Marshal(eventReq)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			req.RemoteAddr = "192.168.1.100:12345"

			// Set consent in context
			ctx := req.Context()
			if tt.withConsent {
				// GDPR applies with valid consent
				ctx = middleware.SetPrivacyContext(ctx, true, true, false, "")
			} else {
				// GDPR applies but NO consent
				ctx = middleware.SetPrivacyContext(ctx, true, false, false, "")
			}
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.HandleVideoEvent(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			if len(analytics.events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(analytics.events))
			}

			event := analytics.events[0]

			// When consent is not provided, IP and UA should be empty
			if !tt.withConsent {
				if event.IPAddress != "" {
					t.Errorf("expected empty IP without consent, got %s", event.IPAddress)
				}
				if event.UserAgent != "" {
					t.Errorf("expected empty UA without consent, got %s", event.UserAgent)
				}
			} else {
				// With consent, should have anonymized values
				if event.IPAddress == "" {
					t.Error("expected IP with consent")
				}
				if event.UserAgent == "" {
					t.Error("expected UA with consent")
				}
			}
		})
	}
}

func TestHandleVideoStart(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		BidID:     "bid-123",
		AccountID: "account-456",
		Bidder:    "testbidder",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoStart(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypeStart) {
		t.Errorf("expected event type start, got %s", event.Event)
	}
}

func TestHandleVideoStart_GET(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	queryParams := url.Values{
		"bid_id":     {"bid-123"},
		"account_id": {"account-456"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/video/start?"+queryParams.Encode(), nil)
	w := httptest.NewRecorder()

	handler.HandleVideoStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type image/gif, got %s", contentType)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}
}

func TestHandleVideoComplete(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		BidID: "bid-123",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoComplete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypeComplete) {
		t.Errorf("expected event type complete, got %s", event.Event)
	}
}

func TestHandleVideoQuartile(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	tests := []struct {
		name          string
		quartile      string
		expectedType  vast.EventType
		expectError   bool
	}{
		{
			name:         "first quartile - 25",
			quartile:     "25",
			expectedType: vast.EventTypeFirstQuartile,
		},
		{
			name:         "first quartile - name",
			quartile:     "firstQuartile",
			expectedType: vast.EventTypeFirstQuartile,
		},
		{
			name:         "midpoint - 50",
			quartile:     "50",
			expectedType: vast.EventTypeMidpoint,
		},
		{
			name:         "midpoint - name",
			quartile:     "midpoint",
			expectedType: vast.EventTypeMidpoint,
		},
		{
			name:         "third quartile - 75",
			quartile:     "75",
			expectedType: vast.EventTypeThirdQuartile,
		},
		{
			name:         "third quartile - name",
			quartile:     "thirdQuartile",
			expectedType: vast.EventTypeThirdQuartile,
		},
		{
			name:        "invalid quartile",
			quartile:    "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analytics.events = nil // Reset

			eventReq := VideoEventRequest{
				BidID: "bid-123",
			}

			body, err := json.Marshal(eventReq)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/video/quartile?quartile="+tt.quartile, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleVideoQuartile(w, req)

			if tt.expectError {
				if w.Code != http.StatusBadRequest {
					t.Errorf("expected status 400, got %d", w.Code)
				}
				return
			}

			if w.Code != http.StatusNoContent {
				t.Errorf("expected status 204, got %d", w.Code)
			}

			if len(analytics.events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(analytics.events))
			}

			event := analytics.events[0]
			if event.Event != string(tt.expectedType) {
				t.Errorf("expected event type %s, got %s", tt.expectedType, event.Event)
			}
		})
	}
}

func TestHandleVideoClick(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		BidID: "bid-123",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/click", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoClick(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypeClick) {
		t.Errorf("expected event type click, got %s", event.Event)
	}
}

func TestHandleVideoPause(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		BidID: "bid-123",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoPause(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypePause) {
		t.Errorf("expected event type pause, got %s", event.Event)
	}
}

func TestHandleVideoResume(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	eventReq := VideoEventRequest{
		BidID: "bid-123",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/resume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoResume(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypeResume) {
		t.Errorf("expected event type resume, got %s", event.Event)
	}
}

func TestHandleVideoError(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	queryParams := url.Values{
		"bid_id":        {"bid-123"},
		"error_code":    {"400"},
		"error_message": {"Video load failed"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/video/error?"+queryParams.Encode(), nil)
	w := httptest.NewRecorder()

	handler.HandleVideoError(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]
	if event.Event != string(vast.EventTypeError) {
		t.Errorf("expected event type error, got %s", event.Event)
	}
	if event.ErrorCode != "400" {
		t.Errorf("expected error code 400, got %s", event.ErrorCode)
	}
	if event.ErrorMessage != "Video load failed" {
		t.Errorf("expected error message 'Video load failed', got %s", event.ErrorMessage)
	}
}

func TestWriteTrackingPixel(t *testing.T) {
	handler := &VideoEventHandler{}

	w := httptest.NewRecorder()
	handler.writeTrackingPixel(w)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type image/gif, got %s", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("expected Cache-Control no-cache, got %s", cacheControl)
	}

	pragma := w.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %s", pragma)
	}

	expires := w.Header().Get("Expires")
	if expires != "0" {
		t.Errorf("expected Expires 0, got %s", expires)
	}

	// Verify it's a valid 1x1 transparent GIF
	body := w.Body.Bytes()
	if len(body) != 43 {
		t.Errorf("expected 43 bytes, got %d", len(body))
	}

	// Check GIF magic bytes
	if body[0] != 0x47 || body[1] != 0x49 || body[2] != 0x46 {
		t.Error("response is not a valid GIF")
	}
}

func TestProcessEvent_NoAnalytics(t *testing.T) {
	// Handler with nil analytics should not error
	handler := &VideoEventHandler{
		analytics: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/video/event?event=start&bid_id=bid-123", nil)

	eventReq := &VideoEventRequest{
		Event: "start",
		BidID: "bid-123",
	}

	err := handler.processEvent(eventReq, req)
	if err != nil {
		t.Errorf("expected no error with nil analytics, got %v", err)
	}
}

func TestRegisterVideoEventRoutes(t *testing.T) {
	mux := http.NewServeMux()
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	RegisterVideoEventRoutes(mux, handler)

	// Test that routes are registered
	routes := []string{
		"/api/v1/video/event",
		"/api/v1/video/start",
		"/api/v1/video/complete",
		"/api/v1/video/quartile",
		"/api/v1/video/click",
		"/api/v1/video/pause",
		"/api/v1/video/resume",
		"/api/v1/video/error",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route+"?event=start&bid_id=test", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Should not return 404
			if w.Code == http.StatusNotFound {
				t.Errorf("route %s not registered", route)
			}
		})
	}
}

func TestVideoEvent_AllFields(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	// Test with all fields populated
	eventReq := VideoEventRequest{
		Event:        "start",
		BidID:        "bid-123",
		AccountID:    "account-456",
		Bidder:       "testbidder",
		Timestamp:    1640000000000,
		Progress:     0.25,
		ErrorCode:    "",
		ErrorMessage: "",
		ClickURL:     "https://example.com/click",
		SessionID:    "session-789",
		ContentID:    "content-abc",
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.RemoteAddr = "192.168.1.100:12345"

	// Add consent to context
	ctx := middleware.SetPrivacyContext(req.Context(), false, true, false, "")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.HandleVideoEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(analytics.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(analytics.events))
	}

	event := analytics.events[0]

	// Verify all fields
	if event.Event != "start" {
		t.Errorf("expected Event start, got %s", event.Event)
	}
	if event.BidID != "bid-123" {
		t.Errorf("expected BidID bid-123, got %s", event.BidID)
	}
	if event.AccountID != "account-456" {
		t.Errorf("expected AccountID account-456, got %s", event.AccountID)
	}
	if event.Bidder != "testbidder" {
		t.Errorf("expected Bidder testbidder, got %s", event.Bidder)
	}
	if event.Progress != 0.25 {
		t.Errorf("expected Progress 0.25, got %f", event.Progress)
	}
	if event.ClickURL != "https://example.com/click" {
		t.Errorf("expected ClickURL, got %s", event.ClickURL)
	}
	if event.SessionID != "session-789" {
		t.Errorf("expected SessionID session-789, got %s", event.SessionID)
	}
	if event.ContentID != "content-abc" {
		t.Errorf("expected ContentID content-abc, got %s", event.ContentID)
	}
	if event.IPAddress == "" {
		t.Error("expected IPAddress to be set with consent")
	}
	if event.UserAgent == "" {
		t.Error("expected UserAgent to be set with consent")
	}
}

func TestGetClientIP_VideoEvents(t *testing.T) {
	t.Setenv("TRUST_X_FORWARDED_FOR", "true")
	// Test getClientIP function used in video_events.go
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "RemoteAddr with port",
			remoteAddr: "203.0.113.1:54321",
			expectedIP: "203.0.113.1",
		},
		{
			name:          "X-Forwarded-For takes precedence",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 198.51.100.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "X-Real-IP fallback",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.1",
			expectedIP: "203.0.113.1",
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

func TestHandleSpecificEvent_MethodNotAllowed(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/video/start", nil)
	w := httptest.NewRecorder()

	handler.HandleVideoStart(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleSpecificEvent_POST_InvalidJSON(t *testing.T) {
	analytics := &mockVideoAnalytics{}
	handler := NewVideoEventHandler(analytics)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/video/start", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleVideoStart(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
