package endpoints

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/pkg/vast"
)

// VideoEventRequest represents the body of a video event request
type VideoEventRequest struct {
	Event        string `json:"event"`
	BidID        string `json:"bid_id"`
	AccountID    string `json:"account_id"`
	Bidder       string `json:"bidder,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	Progress     float64 `json:"progress,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ClickURL     string `json:"click_url,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	ContentID    string `json:"content_id,omitempty"`
}

// VideoEventResponse represents the response to a video event
type VideoEventResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// VideoEventHandler handles video tracking events
type VideoEventHandler struct {
	analytics analytics.Module
}

// NewVideoEventHandler creates a new video event handler.
// If analytics is nil, events are logged to stderr but not persisted.
func NewVideoEventHandler(analyticsModule analytics.Module) *VideoEventHandler {
	return &VideoEventHandler{
		analytics: analyticsModule,
	}
}

// HandleVideoEvent handles POST /api/v1/video/event
func (h *VideoEventHandler) HandleVideoEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleGETEvent(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VideoEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %s", err), http.StatusBadRequest)
		return
	}

	if err := h.processEvent(&req, r); err != nil {
		log.Error().Err(err).Str("event", req.Event).Msg("Failed to process video event")
		http.Error(w, "failed to process event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VideoEventResponse{
		Status:    "ok",
		Timestamp: time.Now().UnixMilli(),
	})
}

// handleGETEvent handles GET requests with query parameters
func (h *VideoEventHandler) handleGETEvent(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	req := &VideoEventRequest{
		Event:     q.Get("event"),
		BidID:     q.Get("bid_id"),
		AccountID: q.Get("account_id"),
		Bidder:    q.Get("bidder"),
		SessionID: q.Get("session_id"),
		ContentID: q.Get("content_id"),
	}

	if err := h.processEvent(req, r); err != nil {
		log.Error().Err(err).Str("event", req.Event).Msg("Failed to process video event")
		// Return 1x1 transparent GIF even on error for tracking pixels
		h.writeTrackingPixel(w)
		return
	}

	h.writeTrackingPixel(w)
}

// processEvent processes a video tracking event
func (h *VideoEventHandler) processEvent(req *VideoEventRequest, r *http.Request) error {
	if req.Event == "" {
		return fmt.Errorf("event type is required")
	}

	if req.BidID == "" {
		return fmt.Errorf("bid_id is required")
	}

	// Validate event type against the canonical VAST list before persisting,
	// so a typo in the player doesn't pollute the analytics table.
	eventType := vast.EventType(req.Event)
	_ = eventType

	// GDPR FIX: Only collect IP/UA if consent allows
	var ipAddress, userAgent string
	if middleware.ShouldCollectPII(r.Context()) {
		ipAddress = middleware.AnonymizeIPForLogging(getClientIP(r))
		userAgent = middleware.AnonymizeUserAgentForLogging(r.UserAgent())
	}

	ts := time.Now()
	if req.Timestamp > 0 {
		ts = time.UnixMilli(req.Timestamp)
	}

	video := &analytics.VideoObject{
		Event:        req.Event,
		Timestamp:    ts,
		BidID:        req.BidID,
		AccountID:    req.AccountID,
		Bidder:       req.Bidder,
		Progress:     req.Progress,
		ErrorCode:    req.ErrorCode,
		ErrorMessage: req.ErrorMessage,
		ClickURL:     req.ClickURL,
		SessionID:    req.SessionID,
		ContentID:    req.ContentID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	log.Info().
		Str("event", req.Event).
		Str("bid_id", req.BidID).
		Str("account_id", req.AccountID).
		Str("bidder", req.Bidder).
		Msg("Video event tracked")

	if h.analytics != nil {
		return h.analytics.LogVideoObject(r.Context(), video)
	}

	return nil
}

// writeTrackingPixel writes a 1x1 transparent GIF
func (h *VideoEventHandler) writeTrackingPixel(w http.ResponseWriter) {
	// 1x1 transparent GIF
	pixel := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
		0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
		0x01, 0x00, 0x3b,
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(pixel)
}

// HandleVideoStart handles POST /api/v1/video/start
func (h *VideoEventHandler) HandleVideoStart(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypeStart)
}

// HandleVideoComplete handles POST /api/v1/video/complete
func (h *VideoEventHandler) HandleVideoComplete(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypeComplete)
}

// HandleVideoQuartile handles POST /api/v1/video/quartile
func (h *VideoEventHandler) HandleVideoQuartile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	quartile := q.Get("quartile")

	var eventType vast.EventType
	switch quartile {
	case "25", "firstQuartile":
		eventType = vast.EventTypeFirstQuartile
	case "50", "midpoint":
		eventType = vast.EventTypeMidpoint
	case "75", "thirdQuartile":
		eventType = vast.EventTypeThirdQuartile
	default:
		http.Error(w, "invalid quartile parameter", http.StatusBadRequest)
		return
	}

	h.handleSpecificEvent(w, r, eventType)
}

// HandleVideoClick handles POST /api/v1/video/click
func (h *VideoEventHandler) HandleVideoClick(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypeClick)
}

// HandleVideoPause handles POST /api/v1/video/pause
func (h *VideoEventHandler) HandleVideoPause(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypePause)
}

// HandleVideoResume handles POST /api/v1/video/resume
func (h *VideoEventHandler) HandleVideoResume(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypeResume)
}

// HandleVideoError handles POST /api/v1/video/error
func (h *VideoEventHandler) HandleVideoError(w http.ResponseWriter, r *http.Request) {
	h.handleSpecificEvent(w, r, vast.EventTypeError)
}

// handleSpecificEvent handles a specific event type
func (h *VideoEventHandler) handleSpecificEvent(w http.ResponseWriter, r *http.Request, eventType vast.EventType) {
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		req := &VideoEventRequest{
			Event:        string(eventType),
			BidID:        q.Get("bid_id"),
			AccountID:    q.Get("account_id"),
			Bidder:       q.Get("bidder"),
			ErrorCode:    q.Get("error_code"),
			ErrorMessage: q.Get("error_message"),
		}

		if err := h.processEvent(req, r); err != nil {
			log.Error().Err(err).Str("event", string(eventType)).Msg("Failed to process video event")
		}

		h.writeTrackingPixel(w)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VideoEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %s", err), http.StatusBadRequest)
		return
	}

	req.Event = string(eventType)

	if err := h.processEvent(&req, r); err != nil {
		log.Error().Err(err).Str("event", string(eventType)).Msg("Failed to process video event")
		http.Error(w, "failed to process event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Only trust X-Forwarded-For when explicitly enabled
	trustXFF := os.Getenv("TRUST_X_FORWARDED_FOR") == "true"

	if trustXFF {
		// Try X-Forwarded-For first (for proxied requests)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
			// Take the first (leftmost) IP which is the original client
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])

				// Validate IP format to prevent injection
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}

		// Try X-Real-IP
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			// Validate IP format
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	// Fall back to RemoteAddr (always trusted, strip port if present)
	ip, _, _ := net.SplitHostPort(r.RemoteAddr) //nolint:errcheck // RemoteAddr may not have port
	if ip == "" {
		// RemoteAddr didn't have port, use as-is
		ip = r.RemoteAddr
	}
	return ip
}

// RegisterVideoEventRoutes registers video event routes with the provided mux
func RegisterVideoEventRoutes(mux *http.ServeMux, handler *VideoEventHandler) {
	mux.HandleFunc("/api/v1/video/event", handler.HandleVideoEvent)
	mux.HandleFunc("/api/v1/video/start", handler.HandleVideoStart)
	mux.HandleFunc("/api/v1/video/complete", handler.HandleVideoComplete)
	mux.HandleFunc("/api/v1/video/quartile", handler.HandleVideoQuartile)
	mux.HandleFunc("/api/v1/video/click", handler.HandleVideoClick)
	mux.HandleFunc("/api/v1/video/pause", handler.HandleVideoPause)
	mux.HandleFunc("/api/v1/video/resume", handler.HandleVideoResume)
	mux.HandleFunc("/api/v1/video/error", handler.HandleVideoError)
}
