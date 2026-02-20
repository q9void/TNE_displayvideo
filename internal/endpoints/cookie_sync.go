package endpoints

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/internal/usersync"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// CookieSyncRequest is the request body for /cookie_sync
type CookieSyncRequest struct {
	// Bidders is the list of bidders to sync (empty = all configured bidders)
	Bidders []string `json:"bidders,omitempty"`
	// GDPR indicates if GDPR applies (0 = no, 1 = yes)
	GDPR int `json:"gdpr,omitempty"`
	// GDPRConsent is the TCF consent string
	GDPRConsent string `json:"gdpr_consent,omitempty"`
	// USPrivacy is the CCPA/US Privacy string
	USPrivacy string `json:"us_privacy,omitempty"`
	// Limit is the max number of syncs to return (default 8)
	Limit int `json:"limit,omitempty"`
	// CooperativeSync enables syncing for bidders not in the request
	CooperativeSync bool `json:"coopSync,omitempty"`
	// FilterSettings controls which sync types to use
	FilterSettings *FilterSettings `json:"filterSettings,omitempty"`
	// FPID is the client-provided first-party identifier
	FPID string `json:"fpid,omitempty"`
}

// FilterSettings controls sync type filtering
type FilterSettings struct {
	Iframe   *FilterConfig `json:"iframe,omitempty"`
	Redirect *FilterConfig `json:"image,omitempty"` // Called "image" in Prebid spec
}

// FilterConfig is a filter for a sync type
type FilterConfig struct {
	Bidders string   `json:"bidders,omitempty"` // "include" or "exclude"
	Filter  []string `json:"filter,omitempty"`  // List of bidder codes
}

// CookieSyncResponse is the response body for /cookie_sync
type CookieSyncResponse struct {
	Status       string             `json:"status"`
	BidderStatus []BidderSyncStatus `json:"bidder_status,omitempty"`
}

// BidderSyncStatus is the sync status for a single bidder
type BidderSyncStatus struct {
	Bidder   string             `json:"bidder"`
	NoCookie bool               `json:"no_cookie,omitempty"`
	UserSync *usersync.SyncInfo `json:"usersync,omitempty"`
	Error    string             `json:"error,omitempty"`
}

// CookieSyncHandler handles cookie sync requests
type CookieSyncHandler struct {
	syncers       map[string]*usersync.Syncer
	hostURL       string
	maxSyncs      int
	userSyncStore *storage.UserSyncStore // Database storage for user syncs
}

// CookieSyncConfig holds configuration for the cookie sync handler
type CookieSyncConfig struct {
	HostURL     string
	MaxSyncs    int
	SyncConfigs map[string]usersync.SyncerConfig
}

// DefaultCookieSyncConfig returns default configuration
func DefaultCookieSyncConfig(hostURL string) *CookieSyncConfig {
	return &CookieSyncConfig{
		HostURL:     hostURL,
		MaxSyncs:    8,
		SyncConfigs: usersync.DefaultSyncerConfigs(),
	}
}

// NewCookieSyncHandler creates a new cookie sync handler
func NewCookieSyncHandler(config *CookieSyncConfig, userSyncStore *storage.UserSyncStore) *CookieSyncHandler {
	syncers := make(map[string]*usersync.Syncer)

	for code, syncConfig := range config.SyncConfigs {
		syncers[code] = usersync.NewSyncer(syncConfig, config.HostURL)
	}

	return &CookieSyncHandler{
		syncers:       syncers,
		hostURL:       config.HostURL,
		maxSyncs:      config.MaxSyncs,
		userSyncStore: userSyncStore,
	}
}

// ServeHTTP handles the /cookie_sync endpoint
func (h *CookieSyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only POST is allowed
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req CookieSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK - use defaults
		req = CookieSyncRequest{}
	}

	// Set defaults
	if req.Limit <= 0 || req.Limit > h.maxSyncs {
		req.Limit = h.maxSyncs
	}

	// Log cookie sync request
	logEvent := logger.Log.Info().
		Strs("bidders", req.Bidders).
		Int("gdpr", req.GDPR).
		Str("us_privacy", req.USPrivacy).
		Int("limit", req.Limit).
		Str("remote_addr", r.RemoteAddr)

	if req.GDPR == 1 && req.GDPRConsent != "" {
		// Only log first 20 chars of consent string to avoid log bloat
		consentPreview := req.GDPRConsent
		if len(consentPreview) > 20 {
			consentPreview = consentPreview[:20] + "..."
		}
		logEvent.Str("gdpr_consent_preview", consentPreview)
	}

	logEvent.Msg("Cookie sync request received")

	// GDPR FIX: Validate GDPR consent before processing cookie sync
	// If GDPR=1 but no valid consent, do not return sync URLs
	if req.GDPR == 1 {
		if req.GDPRConsent == "" {
			logger.Log.Warn().
				Str("remote_addr", r.RemoteAddr).
				Strs("bidders", req.Bidders).
				Msg("GDPR consent required but not provided for cookie sync - rejecting")
			h.respondJSON(w, CookieSyncResponse{
				Status:       "ok",
				BidderStatus: []BidderSyncStatus{},
			})
			return
		}
		// Validate consent string format (minimum length for TCF v2)
		if len(req.GDPRConsent) < 20 {
			logger.Log.Warn().
				Str("remote_addr", r.RemoteAddr).
				Strs("bidders", req.Bidders).
				Int("consent_length", len(req.GDPRConsent)).
				Msg("Invalid GDPR consent string for cookie sync - rejecting")
			h.respondJSON(w, CookieSyncResponse{
				Status:       "ok",
				BidderStatus: []BidderSyncStatus{},
			})
			return
		}
	}

	// Parse existing cookie to see what's already synced
	cookie := usersync.ParseCookie(r)

	// Store FPID from request if provided AND consent is valid
	// GDPR compliance: Only store FPID if we have valid consent or GDPR doesn't apply
	if req.FPID != "" {
		// GDPR check is already done above - if we reach here, consent is valid
		// But double-check to be explicit about GDPR compliance
		canStoreFPID := req.GDPR != 1 || (req.GDPRConsent != "" && len(req.GDPRConsent) >= 20)

		if canStoreFPID {
			cookie.SetFPID(req.FPID)
			logger.Log.Info().
				Str("fpid", req.FPID).
				Int("gdpr", req.GDPR).
				Bool("has_consent", req.GDPRConsent != "").
				Str("remote_addr", r.RemoteAddr).
				Msg("Stored FPID from cookie sync request")
		} else {
			logger.Log.Warn().
				Str("fpid", req.FPID).
				Int("gdpr", req.GDPR).
				Str("remote_addr", r.RemoteAddr).
				Msg("Rejected FPID - GDPR applies but no valid consent")
		}
	}

	// Check for opt-out
	if cookie.IsOptOut() {
		logger.Log.Info().
			Str("remote_addr", r.RemoteAddr).
			Msg("Cookie sync skipped - user opted out")
		h.respondJSON(w, CookieSyncResponse{Status: "ok"})
		return
	}

	// Determine which bidders to sync
	biddersToSync := h.getBiddersToSync(req, cookie)

	logger.Log.Debug().
		Strs("bidders_to_sync", biddersToSync).
		Int("count", len(biddersToSync)).
		Msg("Determined bidders to sync")

	// Build response
	response := CookieSyncResponse{
		Status:       "ok",
		BidderStatus: make([]BidderSyncStatus, 0, len(biddersToSync)),
	}

	// GDPR string for sync URLs
	gdprStr := "0"
	if req.GDPR == 1 {
		gdprStr = "1"
	}

	syncCount := 0
	for _, bidderCode := range biddersToSync {
		if syncCount >= req.Limit {
			break
		}

		syncer, ok := h.syncers[strings.ToLower(bidderCode)]
		if !ok {
			response.BidderStatus = append(response.BidderStatus, BidderSyncStatus{
				Bidder: bidderCode,
				Error:  "unsupported bidder",
			})
			continue
		}

		if !syncer.IsEnabled() {
			continue
		}

		// Check if already synced
		if cookie.HasUID(bidderCode) {
			logger.Log.Debug().
				Str("bidder", bidderCode).
				Msg("Skipping cookie sync - already synced")
			continue
		}

		// Determine sync type based on filterSettings
		syncType := h.getSyncTypeForBidder(bidderCode, req.FilterSettings)
		if syncType == usersync.SyncType("") {
			// Bidder filtered out by filterSettings
			continue
		}

		// Get sync URL
		syncInfo, err := syncer.GetSync(syncType, gdprStr, req.GDPRConsent, req.USPrivacy)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("bidder", bidderCode).
				Str("sync_type", string(syncType)).
				Msg("Failed to get sync URL for bidder")
			response.BidderStatus = append(response.BidderStatus, BidderSyncStatus{
				Bidder: bidderCode,
				Error:  err.Error(),
			})
			continue
		}

		logger.Log.Debug().
			Str("bidder", bidderCode).
			Str("sync_type", string(syncType)).
			Msg("Generated sync URL for bidder")

		response.BidderStatus = append(response.BidderStatus, BidderSyncStatus{
			Bidder:   bidderCode,
			NoCookie: true,
			UserSync: syncInfo,
		})
		syncCount++
	}

	// Log sync results
	syncedBidders := make([]string, 0, len(response.BidderStatus))
	for _, status := range response.BidderStatus {
		if status.UserSync != nil {
			syncedBidders = append(syncedBidders, status.Bidder)
		}
	}

	// Store sync records in database if FPID is available and database is enabled
	fpid := cookie.GetFPID()
	if fpid != "" && h.userSyncStore != nil && len(syncedBidders) > 0 {
		// Store each synced bidder in database (UID will be NULL until callback)
		ctx := r.Context()
		expiresAt := time.Now().Add(90 * 24 * time.Hour) // 90 days typical cookie lifetime

		for _, bidderCode := range syncedBidders {
			if err := h.userSyncStore.UpsertSync(ctx, fpid, bidderCode, nil, &expiresAt); err != nil {
				logger.Log.Warn().
					Err(err).
					Str("fpid", fpid).
					Str("bidder", bidderCode).
					Msg("Failed to store user sync record in database")
			}
		}

		logger.Log.Debug().
			Str("fpid", fpid).
			Strs("bidders", syncedBidders).
			Msg("Stored user sync records in database")
	}

	logger.Log.Info().
		Str("fpid", fpid).
		Strs("synced_bidders", syncedBidders).
		Int("synced_count", len(syncedBidders)).
		Int("requested_count", len(biddersToSync)).
		Str("remote_addr", r.RemoteAddr).
		Msg("Cookie sync completed")

	// Set cookie
	if httpCookie, err := cookie.ToHTTPCookie(h.getCookieDomain(r)); err == nil {
		http.SetCookie(w, httpCookie)
	}

	h.respondJSON(w, response)
}

// getSyncTypeForBidder determines the sync type for a bidder based on filterSettings
// Returns empty string if the bidder should be filtered out
func (h *CookieSyncHandler) getSyncTypeForBidder(bidderCode string, filterSettings *FilterSettings) usersync.SyncType {
	if filterSettings == nil {
		// No filter settings - default to redirect
		return usersync.SyncTypeRedirect
	}

	// Try iframe first (preferred for better sync rates)
	if filterSettings.Iframe != nil {
		if h.shouldIncludeBidder(bidderCode, filterSettings.Iframe) {
			return usersync.SyncTypeIframe
		}
	}

	// Try redirect as fallback
	if filterSettings.Redirect != nil {
		if h.shouldIncludeBidder(bidderCode, filterSettings.Redirect) {
			return usersync.SyncTypeRedirect
		}
	}

	// If filterSettings is provided but bidder doesn't match any filter, default to redirect
	// This matches Prebid.js behavior where filterSettings is advisory, not restrictive
	return usersync.SyncTypeRedirect
}

// shouldIncludeBidder checks if a bidder passes the filter configuration
func (h *CookieSyncHandler) shouldIncludeBidder(bidderCode string, config *FilterConfig) bool {
	if config == nil || len(config.Filter) == 0 {
		return true // No filter = include all
	}

	bidderInList := h.containsBidder(config.Filter, bidderCode)

	if config.Bidders == "include" {
		return bidderInList // Include only if in list
	} else if config.Bidders == "exclude" {
		return !bidderInList // Exclude if in list
	}

	// Unknown mode - default to include
	return true
}

// containsBidder checks if a bidder code is in a list (case-insensitive)
func (h *CookieSyncHandler) containsBidder(list []string, bidder string) bool {
	bidderLower := strings.ToLower(bidder)
	for _, b := range list {
		if strings.ToLower(b) == bidderLower {
			return true
		}
	}
	return false
}

// getBiddersToSync determines which bidders need syncing
func (h *CookieSyncHandler) getBiddersToSync(req CookieSyncRequest, cookie *usersync.Cookie) []string {
	var bidders []string

	if len(req.Bidders) > 0 {
		// Use requested bidders
		bidders = req.Bidders
	} else if req.CooperativeSync {
		// Sync all configured bidders
		for code := range h.syncers {
			bidders = append(bidders, code)
		}
	} else {
		// No bidders specified and no coop sync - return common bidders
		bidders = []string{"appnexus", "rubicon", "pubmatic", "openx", "triplelift"}
	}

	// Filter out bidders that already have UIDs (optimization to avoid redundant syncs)
	if cookie != nil {
		needsSync := make([]string, 0, len(bidders))
		for _, bidder := range bidders {
			if !cookie.HasUID(bidder) {
				needsSync = append(needsSync, bidder)
			}
		}
		return needsSync
	}

	return bidders
}

// getCookieDomain extracts the domain for cookies
func (h *CookieSyncHandler) getCookieDomain(r *http.Request) string {
	return GetCookieDomain(r.Host)
}

// respondJSON writes a JSON response
func (h *CookieSyncHandler) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to encode cookie sync response")
	}
}

// AddSyncer adds a syncer for a bidder
func (h *CookieSyncHandler) AddSyncer(config usersync.SyncerConfig) {
	h.syncers[strings.ToLower(config.BidderCode)] = usersync.NewSyncer(config, h.hostURL)
}

// ListBidders returns all configured bidder codes
func (h *CookieSyncHandler) ListBidders() []string {
	bidders := make([]string, 0, len(h.syncers))
	for code := range h.syncers {
		bidders = append(bidders, code)
	}
	return bidders
}
