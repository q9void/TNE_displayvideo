package endpoints

import (
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/internal/usersync"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// SetUIDHandler handles the /setuid endpoint for storing bidder user IDs
type SetUIDHandler struct {
	validBidders  map[string]bool
	idGraphStore  *storage.IDGraphStore
	userSyncStore *storage.UserSyncStore
}

// NewSetUIDHandler creates a new setuid handler
func NewSetUIDHandler(validBidders []string, idGraphStore *storage.IDGraphStore, userSyncStore *storage.UserSyncStore) *SetUIDHandler {
	bidderMap := make(map[string]bool)
	for _, b := range validBidders {
		bidderMap[strings.ToLower(b)] = true
	}
	return &SetUIDHandler{
		validBidders:  bidderMap,
		idGraphStore:  idGraphStore,
		userSyncStore: userSyncStore,
	}
}

// ServeHTTP handles the /setuid endpoint
// Expected query params:
//   - bidder: the bidder code
//   - uid: the user ID from the bidder
//   - gdpr: GDPR applies (0/1)
//   - gdpr_consent: TCF consent string
func (h *SetUIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	query := r.URL.Query()
	bidder := query.Get("bidder")
	uid := query.Get("uid")
	gdpr := query.Get("gdpr")
	gdprConsent := query.Get("gdpr_consent")

	// GDPR FIX: Validate GDPR consent before storing UIDs
	// If GDPR=1 but no valid consent, do not store the UID
	if gdpr == "1" {
		if gdprConsent == "" {
			logger.Log.Warn().Str("bidder", bidder).Msg("GDPR consent required but not provided for setuid")
			h.respondWithPixel(w)
			return
		}
		// Validate consent string format (minimum length for TCF v2)
		if len(gdprConsent) < 20 {
			logger.Log.Warn().Str("bidder", bidder).Msg("Invalid GDPR consent string for setuid")
			h.respondWithPixel(w)
			return
		}
	}

	// Validate bidder
	if bidder == "" {
		http.Error(w, "missing bidder parameter", http.StatusBadRequest)
		return
	}

	bidderLower := strings.ToLower(bidder)
	if !h.validBidders[bidderLower] {
		logger.Log.Warn().Str("bidder", bidder).Msg("Unknown bidder in setuid request")
		// Still process - bidder might be dynamically registered
	}

	// Parse existing cookie
	cookie := usersync.ParseCookie(r)

	// Check for opt-out
	if cookie.IsOptOut() {
		h.respondWithPixel(w)
		return
	}

	// Handle UID
	if uid == "" || uid == "$UID" || uid == "0" {
		// Bidder sent empty/invalid UID - delete any existing
		cookie.DeleteUID(bidderLower)
		logger.Log.Debug().Str("bidder", bidder).Msg("Deleted UID (empty value received)")
	} else {
		// Store the UID in cookie
		cookie.SetUID(bidderLower, uid)
		logger.Log.Debug().
			Str("bidder", bidder).
			Int("uid_length", len(uid)).
			Int("total_syncs", cookie.SyncCount()).
			Msg("Stored UID in cookie")

		// Store UID in database if FPID and userSyncStore are available
		if h.userSyncStore != nil && cookie.GetFPID() != "" {
			// Enhanced logging: Log actual UID value and all details for debugging
			logger.Log.Info().
				Str("fpid", cookie.GetFPID()).
				Str("bidder", bidderLower).
				Str("uid", uid).
				Int("uid_length", len(uid)).
				Msg("SetUID callback received - storing in database")

			if err := h.userSyncStore.UpdateUID(r.Context(), cookie.GetFPID(), bidderLower, uid); err != nil {
				// Change from Warn to Error for database storage failures
				logger.Log.Error().
					Err(err).
					Str("fpid", cookie.GetFPID()).
					Str("bidder", bidderLower).
					Str("uid", uid).
					Msg("FAILED to update UID in database")
			} else {
				logger.Log.Info().
					Str("fpid", cookie.GetFPID()).
					Str("bidder", bidderLower).
					Str("uid", uid).
					Msg("Successfully updated UID in database")
			}
		}

		// Record ID graph mapping (GDPR-compliant)
		// Only record if:
		// 1. ID graph store is available
		// 2. FPID exists in cookie
		// 3. Either GDPR doesn't apply OR valid consent is provided
		if h.idGraphStore != nil && cookie.GetFPID() != "" {
			// Check GDPR compliance
			hasConsent := gdpr != "1" || (gdprConsent != "" && len(gdprConsent) >= 20)

			if hasConsent {
				metadata := &storage.MappingMetadata{
					ConsentGiven: hasConsent,
					GDPRApplies:  gdpr == "1",
					IPAddress:    r.RemoteAddr,
					UserAgent:    r.UserAgent(),
				}

				if err := h.idGraphStore.RecordMapping(
					r.Context(),
					cookie.GetFPID(),
					bidderLower,
					uid,
					metadata,
				); err != nil {
					logger.Log.Warn().
						Err(err).
						Str("fpid", cookie.GetFPID()).
						Str("bidder", bidderLower).
						Msg("Failed to store ID graph mapping")
				} else {
					logger.Log.Debug().
						Str("fpid", cookie.GetFPID()).
						Str("bidder", bidderLower).
						Str("uid", uid).
						Bool("gdpr_applies", gdpr == "1").
						Msg("ID graph mapping recorded with valid consent")
				}
			} else {
				logger.Log.Warn().
					Str("fpid", cookie.GetFPID()).
					Str("bidder", bidderLower).
					Msg("ID graph mapping blocked - GDPR applies but no valid TCF consent")
			}
		}
	}

	// Set the updated cookie
	domain := h.getCookieDomain(r)
	logger.Log.Debug().
		Str("host", r.Host).
		Str("domain", domain).
		Str("bidder", bidder).
		Msg("Setting cookie domain")

	if httpCookie, err := cookie.ToHTTPCookie(domain); err == nil {
		http.SetCookie(w, httpCookie)
	} else {
		logger.Log.Error().Err(err).Msg("Failed to create cookie")
	}

	// Return tracking pixel
	h.respondWithPixel(w)
}

// getCookieDomain extracts the domain for cookies
func (h *SetUIDHandler) getCookieDomain(r *http.Request) string {
	return GetCookieDomain(r.Host)
}

// respondWithPixel returns a 1x1 transparent GIF
func (h *SetUIDHandler) respondWithPixel(w http.ResponseWriter) {
	// 1x1 transparent GIF
	pixel := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
		0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
		0x01, 0x00, 0x3B,
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Error writing response cannot be handled
	_, _ = w.Write(pixel) // Error writing response cannot be handled
}

// AddBidder adds a valid bidder code
func (h *SetUIDHandler) AddBidder(bidder string) {
	h.validBidders[strings.ToLower(bidder)] = true
}

// OptOutHandler handles opt-out requests
type OptOutHandler struct{}

// NewOptOutHandler creates a new opt-out handler
func NewOptOutHandler() *OptOutHandler {
	return &OptOutHandler{}
}

// ServeHTTP handles the /optout endpoint
func (h *OptOutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse existing cookie
	cookie := usersync.ParseCookie(r)

	// Set opt-out
	cookie.SetOptOut(true)

	// Set the updated cookie
	domain := GetCookieDomain(r.Host)
	if httpCookie, err := cookie.ToHTTPCookie(domain); err == nil {
		http.SetCookie(w, httpCookie)
	}

	// Return success page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Error writing response cannot be handled
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Opted Out</title></head>
<body>
<h1>You have been opted out</h1>
<p>You will no longer receive personalized ads through this service.</p>
<p>To opt back in, clear your cookies for this domain.</p>
</body>
</html>`))
}
