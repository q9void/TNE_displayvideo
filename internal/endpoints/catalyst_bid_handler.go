package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/device"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/geo"
	"github.com/thenexusengine/tne_springwire/internal/hooks"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/internal/usersync"
	"github.com/thenexusengine/tne_springwire/internal/validation"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// BidderMapping represents the full bidder parameter mapping configuration
type BidderMapping struct {
	Publisher struct {
		PublisherID    string   `json:"publisherId"`
		Domain         string   `json:"domain"`
		DefaultBidders []string `json:"defaultBidders"`
	} `json:"publisher"`
	AdUnits map[string]AdUnitConfig `json:"adUnits"`
}

// AdUnitConfig contains bidder-specific parameters for an ad unit
type AdUnitConfig struct {
	Rubicon    *RubiconParams    `json:"rubicon,omitempty"`
	Kargo      *KargoParams      `json:"kargo,omitempty"`
	Sovrn      *SovrnParams      `json:"sovrn,omitempty"`
	Pubmatic   *PubmaticParams   `json:"pubmatic,omitempty"`
	Triplelift *TripleliftParams `json:"triplelift,omitempty"`
}

// RubiconParams are Rubicon/Magnite adapter parameters
type RubiconParams struct {
	AccountID        int  `json:"accountId"`
	SiteID           int  `json:"siteId"`
	ZoneID           int  `json:"zoneId"`
	BidOnMultiFormat bool `json:"bidonmultiformat"`
}

// KargoParams are Kargo adapter parameters
type KargoParams struct {
	PlacementID string `json:"placementId"`
}

// SovrnParams are Sovrn adapter parameters
type SovrnParams struct {
	TagID string `json:"tagid"` // Must be string per Prebid Server spec
}

// PubmaticParams are Pubmatic adapter parameters
type PubmaticParams struct {
	PublisherID string `json:"publisherId"` // Must be string per Prebid Server spec
	AdSlot      string `json:"adSlot"`      // Must be string per Prebid Server spec
}

// TripleliftParams are Triplelift adapter parameters
type TripleliftParams struct {
	InventoryCode string `json:"inventoryCode"`
}

// CatalystBidHandler handles MAI Publisher-compatible bid requests
type CatalystBidHandler struct {
	exchange       *exchange.Exchange
	mapping        *BidderMapping          // Legacy: static mapping file (fallback)
	publisherStore *storage.PublisherStore // Dynamic hierarchical config from database
	userSyncStore  *storage.UserSyncStore  // User sync storage for persistent UIDs
	syncAwaiter    *usersync.SyncAwaiter
}

// LoadBidderMapping loads bidder parameter mapping from JSON file
func LoadBidderMapping(path string) (*BidderMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read mapping file: %w", err)
	}

	var mapping BidderMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to parse mapping JSON: %w", err)
	}

	logger.Log.Info().
		Int("ad_units", len(mapping.AdUnits)).
		Str("publisher", mapping.Publisher.PublisherID).
		Msg("Loaded bidder mapping")

	return &mapping, nil
}

// NewCatalystBidHandler creates a new Catalyst bid handler
func NewCatalystBidHandler(ex *exchange.Exchange, mapping *BidderMapping, publisherStore *storage.PublisherStore, userSyncStore *storage.UserSyncStore, syncAwaiter *usersync.SyncAwaiter) *CatalystBidHandler {
	return &CatalystBidHandler{
		exchange:       ex,
		mapping:        mapping,
		publisherStore: publisherStore,
		userSyncStore:  userSyncStore,
		syncAwaiter:    syncAwaiter,
	}
}

// MAIBidRequest represents the MAI Publisher bid request format
type MAIBidRequest struct {
	AccountID string       `json:"accountId"`
	Timeout   int          `json:"timeout"` // Client-side timeout in ms
	Slots     []MAISlot    `json:"slots"`
	Page      *MAIPage     `json:"page,omitempty"`
	User      *MAIUser     `json:"user,omitempty"`
	Device    *MAIDevice   `json:"device,omitempty"`
}

// MAISlot represents an ad slot
type MAISlot struct {
	DivID                  string      `json:"divId"`
	Sizes                  [][]int     `json:"sizes"`                            // Banner sizes
	AdUnitPath             string      `json:"adUnitPath,omitempty"`
	Position               string      `json:"position,omitempty"`
	Video                  bool        `json:"video,omitempty"`                  // Enable video format
	VideoWidth             int         `json:"videoWidth,omitempty"`             // Video player width
	VideoHeight            int         `json:"videoHeight,omitempty"`            // Video player height
	VideoMimes             []string    `json:"videoMimes,omitempty"`             // Supported MIME types
	Native                 bool        `json:"native,omitempty"`                 // Enable native format
	MultiformatStrategy    string      `json:"multiformatStrategy,omitempty"`    // Strategy for multiformat
	PreferredMediaType     string      `json:"preferredMediaType,omitempty"`     // Preferred media type for multiformat
}

// MAIPage represents page context
type MAIPage struct {
	URL        string   `json:"url,omitempty"`
	Domain     string   `json:"domain,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

// MAIUser represents user/privacy info
type MAIUser struct {
	FPID           string                   `json:"fpid,omitempty"`           // First-party identifier
	ConsentGiven   bool                     `json:"consentGiven,omitempty"`
	ConsentString  string                   `json:"consentString,omitempty"`  // TCFv2 consent string
	GDPRApplies    *bool                    `json:"gdprApplies,omitempty"`
	USPConsent     string                   `json:"uspConsent,omitempty"`
	GPPString      string                   `json:"gppString,omitempty"`      // IAB GPP consent string from __gpp()
	GPPSIDs        []int                    `json:"gppSids,omitempty"`        // Applicable GPP section IDs
	UserIds        map[string]string        `json:"userIds,omitempty"`        // Bidder-specific user IDs from cookie sync
	Eids           []map[string]interface{} `json:"eids,omitempty"`           // Prebid ID module EIDs (id5, pubCommonId, etc.)
	Data           []map[string]interface{} `json:"data,omitempty"`           // ORTB2 user data segments
	Ext            map[string]interface{}   `json:"ext,omitempty"`            // Additional user extensions
}

// MAIDevice represents device info
type MAIDevice struct {
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	DeviceType string   `json:"deviceType,omitempty"`
	UserAgent  string   `json:"userAgent,omitempty"`
	Geo        *MAIGeo  `json:"geo,omitempty"` // Client-side geolocation (optional)
}

// MAIGeo represents client-side geolocation data
type MAIGeo struct {
	Lat      float64 `json:"lat,omitempty"`      // Latitude from GPS/browser
	Lon      float64 `json:"lon,omitempty"`      // Longitude from GPS/browser
	Accuracy int     `json:"accuracy,omitempty"` // Accuracy in meters
}

// MAIBidResponse represents the MAI Publisher bid response format
type MAIBidResponse struct {
	Bids         []MAIBid `json:"bids"`
	ResponseTime int      `json:"responseTime"` // In milliseconds
}

// MAIBid represents a single bid
type MAIBid struct {
	DivID      string            `json:"divId"`
	CPM        float64           `json:"cpm"`
	Currency   string            `json:"currency"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	AdID       string            `json:"adId"`
	CreativeID string            `json:"creativeId"`
	DealID     string            `json:"dealId,omitempty"`
	Meta       *MAIBidMeta       `json:"meta,omitempty"`
	Targeting  map[string]string `json:"targeting,omitempty"` // Pre-built GAM targeting key-values
}

// MAIBidMeta represents bid metadata
type MAIBidMeta struct {
	AdvertiserDomains []string `json:"advertiserDomains,omitempty"`
	NetworkID         string   `json:"networkId,omitempty"`
	NetworkName       string   `json:"networkName,omitempty"`
}

// HandleBidRequest handles POST /v1/bid requests
func (h *CatalystBidHandler) HandleBidRequest(w http.ResponseWriter, r *http.Request) {
	log := logger.Log
	startTime := time.Now()

	// CORS is handled by middleware - removed hardcoded wildcard

	// Only accept POST
	if r.Method != "POST" {
		log.Error().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("Method not allowed - expected POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse MAI bid request
	var maiBidReq MAIBidRequest

	// Close body when done
	defer r.Body.Close()

	// Limit request body size to prevent DoS attacks (1MB limit)
	const maxRequestBodySize = 1024 * 1024 // 1MB
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
	if err != nil {
		log.Error().
			Err(err).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.Header.Get("User-Agent")).
			Msg("Failed to read MAI bid request body")
		h.writeErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// DEBUG: Full request dump if enabled
	debugDumpRequests := os.Getenv("DEBUG_DUMP_REQUESTS") == "true"
	if debugDumpRequests {
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.Header.Get("User-Agent")).
			Str("content_type", r.Header.Get("Content-Type")).
			Str("request_body", string(bodyBytes)).
			Msg("🔍 DEBUG_DUMP_REQUESTS: Full incoming request")
	} else {
		// Log preview for normal debug mode
		requestPreview := string(bodyBytes)
		if len(requestPreview) > 2000 {
			requestPreview = requestPreview[:2000] + "..."
		}
		log.Debug().Str("request_body_preview", requestPreview).Msg("Received MAI bid request")
	}

	if err := json.Unmarshal(bodyBytes, &maiBidReq); err != nil {
		log.Error().
			Err(err).
			Str("request_body", string(bodyBytes)).
			Msg("Failed to parse MAI bid request - invalid JSON")
		h.writeErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validateMAIBidRequest(&maiBidReq); err != nil {
		log.Error().
			Err(err).
			Str("account_id", maiBidReq.AccountID).
			Int("slots_count", len(maiBidReq.Slots)).
			Interface("request", maiBidReq).
			Msg("❌ Invalid MAI bid request - validation failed")
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log all SDK request fields used to build the bid request
	{
		slotSummary := make([]map[string]interface{}, 0, len(maiBidReq.Slots))
		for _, s := range maiBidReq.Slots {
			slotSummary = append(slotSummary, map[string]interface{}{
				"div_id":       s.DivID,
				"ad_unit_path": s.AdUnitPath,
				"sizes":        s.Sizes,
				"video":        s.Video,
				"native":       s.Native,
			})
		}
		pageURL, pageDomain := "", ""
		if maiBidReq.Page != nil {
			pageURL = maiBidReq.Page.URL
			pageDomain = maiBidReq.Page.Domain
		}
		fpid, consentGiven, eidCount, hasSyncedIDs := "", false, 0, false
		if maiBidReq.User != nil {
			fpid = maiBidReq.User.FPID
			consentGiven = maiBidReq.User.ConsentGiven
			hasSyncedIDs = len(maiBidReq.User.UserIds) > 0

			// EIDs may be in user.eids (top-level) or user.ext.eids (Prebid legacy location)
			eidCount = len(maiBidReq.User.Eids)
			if eidCount == 0 {
				if extEids, ok := maiBidReq.User.Ext["eids"]; ok {
					if eidsSlice, ok := extEids.([]interface{}); ok {
						eidCount = len(eidsSlice)
					}
				}
			}
			// Extract FPID from user.ext.eids if not set directly
			if fpid == "" && maiBidReq.User.Ext != nil {
				if extEids, ok := maiBidReq.User.Ext["eids"]; ok {
					if eidsSlice, ok := extEids.([]interface{}); ok {
						for _, e := range eidsSlice {
							if eid, ok := e.(map[string]interface{}); ok {
								if eid["source"] == "thenexusengine.com" {
									if uids, ok := eid["uids"].([]interface{}); ok && len(uids) > 0 {
										if uid, ok := uids[0].(map[string]interface{}); ok {
											fpid, _ = uid["id"].(string)
										}
									}
									break
								}
							}
						}
					}
				}
			}
		}
		deviceType, ua := "", r.Header.Get("User-Agent")
		if maiBidReq.Device != nil {
			deviceType = maiBidReq.Device.DeviceType
			if maiBidReq.Device.UserAgent != "" {
				ua = maiBidReq.Device.UserAgent
			}
		}
		log.Info().
			Str("account_id", maiBidReq.AccountID).
			Int("slot_count", len(maiBidReq.Slots)).
			Interface("slots", slotSummary).
			Str("page_url", pageURL).
			Str("page_domain", pageDomain).
			Str("fpid", fpid).
			Bool("consent_given", consentGiven).
			Int("eid_count", eidCount).
			Bool("has_synced_ids", hasSyncedIDs).
			Str("device_type", deviceType).
			Str("user_agent", ua).
			Str("remote_addr", r.RemoteAddr).
			Msg("SDK bid request received")
	}

	// Convert to OpenRTB
	ortbReq, impToSlot, err := h.convertToOpenRTB(r, &maiBidReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("account_id", maiBidReq.AccountID).
			Int("slots_count", len(maiBidReq.Slots)).
			Msg("❌ Failed to convert to OpenRTB")
		h.writeErrorResponse(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// CP-1: OpenRTB scaffold audit — state before any hook runs
	eidsTopLevel := 0
	eidsInExt := 0
	if ortbReq.User != nil {
		eidsTopLevel = len(ortbReq.User.EIDs)
		eidsInExt = countExtEIDs(ortbReq.User.Ext)
	}
	publisherID := ""
	schainNodes := 0
	if ortbReq.Site != nil && ortbReq.Site.Publisher != nil {
		publisherID = ortbReq.Site.Publisher.ID
	}
	if ortbReq.Source != nil && ortbReq.Source.SChain != nil {
		schainNodes = len(ortbReq.Source.SChain.Nodes)
	}
	usPrivacy := ""
	if ortbReq.Regs != nil {
		usPrivacy = ortbReq.Regs.USPrivacy
	}
	log.Debug().
		Str("request_id", ortbReq.ID).
		Int("imp_count", len(ortbReq.Imp)).
		Int("eids_top_level", eidsTopLevel).
		Int("eids_in_ext", eidsInExt).
		Str("publisher_id", publisherID).
		Int("schain_nodes", schainNodes).
		Str("us_privacy", usPrivacy).
		Msg("CP-1: OpenRTB scaffold built")

	// If every slot was skipped (no bidder config found), return empty bids immediately
	// without touching the exchange — avoids spurious circuit breaker trips.
	if len(ortbReq.Imp) == 0 {
		log.Warn().
			Str("account_id", maiBidReq.AccountID).
			Int("slots", len(maiBidReq.Slots)).
			Msg("All slots have no bidder config — returning empty bids without calling exchange")
		h.writeMAIResponse(w, &MAIBidResponse{
			Bids:         []MAIBid{},
			ResponseTime: int(time.Since(startTime).Milliseconds()),
		})
		return
	}

	// Execute request-level hooks (BEFORE auction)
	// Hook execution order is critical:
	// 1. Request Validation - validates and normalizes request
	// 2. Privacy/Consent - enforces GDPR/CCPA/GPP, strips IDs without consent
	hookExecutor := hooks.NewHookExecutor()
	hookExecutor.RegisterRequestHook(hooks.NewRequestValidationHook())
	hookExecutor.RegisterRequestHook(hooks.NewPrivacyConsentHook())

	if err := hookExecutor.ExecuteRequestHooks(r.Context(), ortbReq); err != nil {
		log.Error().
			Err(err).
			Str("account_id", maiBidReq.AccountID).
			Str("request_id", ortbReq.ID).
			Msg("❌ Request hook failed")
		h.writeErrorResponse(w, fmt.Sprintf("Request validation failed: %v", err), http.StatusBadRequest)
		return
	}

	log.Debug().
		Str("request_id", ortbReq.ID).
		Str("account_id", maiBidReq.AccountID).
		Msg("✓ Request hooks executed successfully")

	// Run auction with per-publisher timeout (falls back to 1500 ms)
	tmaxMs := publisherTMaxMs(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(tmaxMs)*time.Millisecond)
	defer cancel()

	auctionReq := &exchange.AuctionRequest{
		BidRequest: ortbReq,
		Timeout:    time.Duration(tmaxMs) * time.Millisecond,
	}

	auctionResp, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("account_id", maiBidReq.AccountID).
			Int("slots", len(maiBidReq.Slots)).
			Int("timeout_ms", int(time.Since(startTime).Milliseconds())).
			Str("error_type", fmt.Sprintf("%T", err)).
			Msg("❌ Auction failed - returning empty bids")
		// Return empty bids on error (MAI Publisher requirement)
		h.writeMAIResponse(w, &MAIBidResponse{
			Bids:         []MAIBid{},
			ResponseTime: int(time.Since(startTime).Milliseconds()),
		})
		return
	}

	// Convert OpenRTB response to MAI format
	maiResp := h.convertToMAIResponse(auctionResp, impToSlot)
	maiResp.ResponseTime = int(time.Since(startTime).Milliseconds())

	// DEBUG: Full response dump if enabled
	debugDumpResponses := os.Getenv("DEBUG_DUMP_RESPONSES") == "true"
	if debugDumpResponses {
		if respJSON, err := json.Marshal(maiResp); err == nil {
			log.Debug().
				Str("response_body", string(respJSON)).
				Int("bids", len(maiResp.Bids)).
				Int("response_time_ms", maiResp.ResponseTime).
				Msg("🔍 DEBUG_DUMP_RESPONSES: Full outgoing response")
		}
	} else {
		// Normal debug logging
		log.Debug().
			Int("bids", len(maiResp.Bids)).
			Int("response_time_ms", maiResp.ResponseTime).
			Msg("Catalyst response ready")
	}

	// Write response
	h.writeMAIResponse(w, maiResp)

	log.Info().
		Str("account_id", maiBidReq.AccountID).
		Int("slots", len(maiBidReq.Slots)).
		Int("bids", len(maiResp.Bids)).
		Int("response_time_ms", maiResp.ResponseTime).
		Msg("✓ Catalyst bid request completed")
}

// validateMAIBidRequest validates the MAI bid request
func (h *CatalystBidHandler) validateMAIBidRequest(req *MAIBidRequest) error {
	if req.AccountID == "" {
		return fmt.Errorf("accountId is required")
	}
	if len(req.Slots) == 0 {
		return fmt.Errorf("at least one slot is required")
	}
	for i, slot := range req.Slots {
		if slot.DivID == "" {
			return fmt.Errorf("slot[%d].divId is required", i)
		}
		if len(slot.Sizes) == 0 {
			return fmt.Errorf("slot[%d].sizes is required", i)
		}
		for j, size := range slot.Sizes {
			if len(size) != 2 || size[0] <= 0 || size[1] <= 0 {
				return fmt.Errorf("slot[%d].sizes[%d] must be [width, height] with positive values", i, j)
			}
		}
	}
	return nil
}

// normalizeSlotPattern generates multiple slot pattern variations for flexible matching
// Returns patterns in priority order: exact match → without prefix → without suffix → base
func normalizeSlotPattern(domain, divID, adUnitPath string) []string {
	patterns := []string{}

	// 1. Try exact match with adUnitPath first (if provided)
	if adUnitPath != "" {
		patterns = append(patterns, fmt.Sprintf("%s%s", domain, adUnitPath))
	}

	// 2. Try exact domain/divID
	if divID != "" {
		patterns = append(patterns, fmt.Sprintf("%s/%s", domain, divID))
	}

	// 3. Try without "mai-ad-" prefix (common pattern)
	if divID != "" && strings.HasPrefix(divID, "mai-ad-") {
		simpleDivID := strings.TrimPrefix(divID, "mai-ad-")
		patterns = append(patterns, fmt.Sprintf("%s/%s", domain, simpleDivID))

		// 4. Progressively remove compound suffixes
		// Examples: "leaderboard-wide-adhesion" → "leaderboard-wide" → "leaderboard"
		//           "rectangle-medium" → "rectangle"
		suffixes := []string{"-wide", "-narrow", "-tablet", "-adhesion", "-medium", "-large", "-small"}

		current := simpleDivID
		for {
			removed := false
			for _, suffix := range suffixes {
				if strings.HasSuffix(current, suffix) {
					current = strings.TrimSuffix(current, suffix)
					patterns = append(patterns, fmt.Sprintf("%s/%s", domain, current))
					removed = true
					break // Try removing another suffix from the new current
				}
			}
			if !removed {
				break // No more suffixes to remove
			}
		}
	}

	// 5. Domain-level wildcard catch-all (lowest priority)
	// Matches slots configured with pattern "domain/*" in the DB.
	patterns = append(patterns, fmt.Sprintf("%s/*", domain))

	return patterns
}

// convertToOpenRTB converts MAI bid request to OpenRTB format
func (h *CatalystBidHandler) convertToOpenRTB(r *http.Request, maiBid *MAIBidRequest) (*openrtb.BidRequest, map[string]string, error) {
	// Generate request ID
	requestID := fmt.Sprintf("catalyst-%d", time.Now().UnixNano())

	// Build impressions and track mapping (impID -> divID)
	imps := make([]openrtb.Imp, 0, len(maiBid.Slots))
	impToSlot := make(map[string]string) // Maps impression ID to slot divID

	for i, slot := range maiBid.Slots {
		impID := fmt.Sprintf("%d", i+1)
		impToSlot[impID] = slot.DivID

		// Convert sizes to format array
		formats := make([]openrtb.Format, len(slot.Sizes))
		for j, size := range slot.Sizes {
			formats[j] = openrtb.Format{
				W: size[0],
				H: size[1],
			}
		}

		// We'll set TagID after resolving adUnitPath below
		secureFlag := 1 // Assume HTTPS (OpenRTB 2.6 requirement)
		imp := openrtb.Imp{
			ID:     impID,
			Secure: &secureFlag,
		}

		// Add banner if sizes are provided (default format)
		// Set w/h from the first (largest) format entry — required by Kargo and
		// standard PBS exchange behaviour; format array carries all acceptable sizes.
		if len(slot.Sizes) > 0 {
			imp.Banner = &openrtb.Banner{
				Format: formats,
				W:      formats[0].W,
				H:      formats[0].H,
			}
		}

		// Add video if enabled
		if slot.Video {
			videoWidth := slot.VideoWidth
			videoHeight := slot.VideoHeight
			if videoWidth == 0 || videoHeight == 0 {
				// Default to first banner size if available
				if len(slot.Sizes) > 0 {
					videoWidth = slot.Sizes[0][0]
					videoHeight = slot.Sizes[0][1]
				} else {
					videoWidth = 640
					videoHeight = 480
				}
			}

			mimes := slot.VideoMimes
			if len(mimes) == 0 {
				mimes = []string{"video/mp4", "video/webm"}
			}

			imp.Video = &openrtb.Video{
				W:         videoWidth,
				H:         videoHeight,
				Mimes:     mimes,
				Protocols: []int{2, 3, 5, 6, 7, 8}, // VAST 2.0, 3.0, 4.0 + Wrappers (no VPAID)
				Linearity: 1,                        // Linear only
			}
		}

		// Add native if enabled
		if slot.Native {
			imp.Native = &openrtb.Native{
				Request: "", // Native request would be built based on requirements
				Ver:     "1.2",
			}
		}

		// Set multiformat strategy if multiple formats present
		hasMultipleFormats := 0
		if imp.Banner != nil {
			hasMultipleFormats++
		}
		if imp.Video != nil {
			hasMultipleFormats++
		}
		if imp.Native != nil {
			hasMultipleFormats++
		}

		if hasMultipleFormats > 1 {
			// Add multiformat strategy and preferred media type to imp.ext.prebid
			impExtPrebid := map[string]interface{}{}

			if slot.MultiformatStrategy != "" {
				impExtPrebid["multiformatRequestStrategy"] = slot.MultiformatStrategy
			}

			if slot.PreferredMediaType != "" {
				impExtPrebid["preferredMediaType"] = slot.PreferredMediaType
			}

			if len(impExtPrebid) > 0 {
				if len(imp.Ext) > 0 {
					// Merge with existing ext
					var extMap map[string]interface{}
					if err := json.Unmarshal(imp.Ext, &extMap); err == nil {
						extMap["prebid"] = impExtPrebid
						if extBytes, err := json.Marshal(extMap); err == nil {
							imp.Ext = extBytes
						}
					}
				} else {
					// Create new ext
					extMap := map[string]interface{}{
						"prebid": impExtPrebid,
					}
					if extBytes, err := json.Marshal(extMap); err == nil {
						imp.Ext = extBytes
					}
				}
			}
		}

	// Look up bidder parameters using hierarchical config (DB first, then mapping file fallback)
	// Always try to lookup bidder config, with appropriate fallbacks
	impExt := make(map[string]interface{})

	if maiBid.AccountID == "" {
		logger.Log.Warn().Msg("Missing accountId - cannot lookup bidder config")
	} else {
		// Extract domain from page context
		domain := ""
		if maiBid.Page != nil && maiBid.Page.Domain != "" {
			domain = maiBid.Page.Domain
		}

		// If adUnitPath is missing, try to resolve it from divId mapping
		adUnitPath := slot.AdUnitPath
		if adUnitPath == "" && h.publisherStore != nil {
			resolvedPath, err := h.publisherStore.GetAdUnitPathFromDivID(r.Context(), maiBid.AccountID, domain, slot.DivID)
			if err == nil && resolvedPath != "" {
				adUnitPath = resolvedPath
				logger.Log.Info().
					Str("publisher", maiBid.AccountID).
					Str("domain", domain).
					Str("div_id", slot.DivID).
					Str("resolved_path", adUnitPath).
					Msg("✓ Resolved adUnitPath from divId mapping")
			} else {
				logger.Log.Warn().
					Str("publisher", maiBid.AccountID).
					Str("domain", domain).
					Str("div_id", slot.DivID).
					Msg("⚠️  Missing adUnitPath and no divId mapping found - falling back to publisher-level config only")
			}
		}

		// Set TagID with resolved adUnitPath
		imp.TagID = adUnitPath

		// List of bidders to look up
		bidders := []string{"rubicon", "kargo", "sovrn", "pubmatic", "triplelift"}

		// Detect device type from User-Agent for device-specific bidder configs
		deviceType := "desktop" // Default to desktop
		if maiBid.Device != nil && maiBid.Device.UserAgent != "" {
			deviceType = detectDeviceType(maiBid.Device.UserAgent)
		}

		// Try multiple slot pattern variations for flexible matching
		// This handles cases like "mai-ad-billboard-wide" → "billboard"
		var allConfigs map[string]map[string]interface{}
		var err error
		var matchedPattern string

		if h.publisherStore != nil && domain != "" && slot.DivID != "" {
			patterns := normalizeSlotPattern(domain, slot.DivID, adUnitPath)

			logger.Log.Debug().
				Str("div_id", slot.DivID).
				Strs("patterns_to_try", patterns).
				Msg("Trying multiple slot pattern variations")

			// Try each pattern until we find configs
			for _, pattern := range patterns {
				allConfigs, err = h.publisherStore.GetSlotBidderConfigs(
					r.Context(),
					maiBid.AccountID, // CATALYST internal account ID (e.g., '12345')
					domain,           // Publisher domain (e.g., 'totalprosports.com')
					pattern,          // Ad slot pattern (trying variations)
					deviceType,       // Device type ('desktop' or 'mobile')
				)

				if err != nil {
					logger.Log.Debug().
						Err(err).
						Str("pattern", pattern).
						Msg("Pattern lookup failed, trying next")
					continue
				}

				if len(allConfigs) > 0 {
					matchedPattern = pattern
					logger.Log.Info().
						Str("slot_pattern", matchedPattern).
						Str("div_id", slot.DivID).
						Str("device_type", deviceType).
						Strs("bidder_names", getConfiguredBidders(allConfigs)).
						Int("bidders_configured", len(allConfigs)).
						Str("account_id", maiBid.AccountID).
						Str("domain", domain).
						Msg("✓ Matched slot pattern and loaded bidder configs")
					break
				}
			}

			// If no patterns matched, log error
			if matchedPattern == "" {
				logger.Log.Warn().
					Str("account_id", maiBid.AccountID).
					Str("domain", domain).
					Str("div_id", slot.DivID).
					Strs("patterns_tried", patterns).
					Msg("⚠️  No bidder configs found for any slot pattern variation")
				allConfigs = make(map[string]map[string]interface{})
			}
		} else {
			logger.Log.Warn().
				Str("account_id", maiBid.AccountID).
				Str("domain", domain).
				Str("div_id", slot.DivID).
				Msg("⚠️  Missing div_id - cannot query slot bidder configs")
			allConfigs = make(map[string]map[string]interface{})
		}

		// Now populate impExt with configs
		for _, bidderCode := range bidders {
			params := allConfigs[bidderCode]

			// If no DB config found, fall back to mapping file (only if we have adUnitPath)
			if params == nil && h.mapping != nil && adUnitPath != "" {
				if adUnitConfig, ok := h.mapping.AdUnits[adUnitPath]; ok {
					params = h.extractBidderParamsFromMapping(bidderCode, &adUnitConfig)
				}
			}

			// Add params to impExt if found
			if params != nil && len(params) > 0 {
				if valid, errs := validation.ValidateBidderParams(bidderCode, params); !valid {
					logger.Log.Warn().
						Str("bidder", bidderCode).
						Str("publisher", maiBid.AccountID).
						Str("domain", domain).
						Str("ad_unit", adUnitPath).
						Strs("missing_fields", errs).
						Msg("⚠️  Skipping bidder — required params missing or invalid")
					continue
				}
				impExt[bidderCode] = params
			} else {
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Str("publisher", maiBid.AccountID).
					Str("domain", domain).
					Str("ad_unit", adUnitPath).
					Msg("No configuration found for bidder")
			}
		}

		// Marshal and attach to impression
		if len(impExt) > 0 {
			extJSON, err := json.Marshal(impExt)
			if err == nil {
				imp.Ext = extJSON
				logger.Log.Info().
					Str("publisher", maiBid.AccountID).
					Str("domain", domain).
					Str("ad_unit", adUnitPath).
					Int("bidders_configured", len(impExt)).
					Interface("bidder_names", getBidderNames(impExt)).
					Interface("full_config", impExt).
					Msg("✓ Injected bidder parameters using hierarchical config")
			} else {
				logger.Log.Error().
					Err(err).
					Str("publisher", maiBid.AccountID).
					Str("domain", domain).
					Str("ad_unit", adUnitPath).
					Interface("impExt", impExt).
					Msg("❌ Failed to marshal bidder parameters")
			}
		} else {
			logger.Log.Warn().
				Str("publisher", maiBid.AccountID).
				Str("domain", domain).
				Str("ad_unit", adUnitPath).
				Str("div_id", slot.DivID).
				Msg("No bidder config for slot — skipping imp to avoid exchange errors")
			continue // do not append an imp with no bidder params; avoids spurious circuit breaker trips
		}
	}

		imps = append(imps, imp)
	}

	// Build site
	// NOTE: We intentionally leave site.id EMPTY to prevent leaking CATALYST's
	// internal accountId ('12345') to SSPs. Each adapter sets SSP-specific IDs.
	site := &openrtb.Site{}

	if maiBid.Page != nil {
		site.Domain = maiBid.Page.Domain
		if maiBid.Page.URL != "" {
			site.Page = maiBid.Page.URL
		}
		if maiBid.Page.Domain == "" && maiBid.Page.URL != "" {
			// Extract domain from URL if not provided
			site.Domain = extractDomain(maiBid.Page.URL)
		}
		if len(maiBid.Page.Keywords) > 0 {
			site.Keywords = strings.Join(maiBid.Page.Keywords, ",")
		}
		if len(maiBid.Page.Categories) > 0 {
			iabCats := make([]string, 0, len(maiBid.Page.Categories))
			for _, cat := range maiBid.Page.Categories {
				if strings.HasPrefix(cat, "IAB") {
					iabCats = append(iabCats, cat)
				}
			}
			if len(iabCats) > 0 {
				site.Cat = iabCats
			}
		}
	}

	// NOTE: We intentionally leave publisher.id EMPTY to prevent leaking CATALYST's
	// internal accountId. Adapters set SSP-specific publisher IDs from bidder configs.
	site.Publisher = &openrtb.Publisher{}

	// Get publisher name from database for brand safety
	if h.publisherStore != nil {
		if pub, err := h.publisherStore.GetByPublisherID(r.Context(), maiBid.AccountID); err == nil && pub != nil {
			if publisher, ok := pub.(*storage.Publisher); ok && publisher.Name != "" {
				site.Publisher.Name = publisher.Name
			}
		}
	}

	// Set publisher domain from site domain
	if site.Domain != "" {
		site.Publisher.Domain = site.Domain
	}

	// Build device
	deviceObj := &openrtb.Device{
		IP: getClientIP(r),
	}

	if maiBid.Device != nil {
		if maiBid.Device.UserAgent != "" {
			deviceObj.UA = maiBid.Device.UserAgent
		} else {
			deviceObj.UA = r.Header.Get("User-Agent")
		}
		if maiBid.Device.Width > 0 && maiBid.Device.Height > 0 {
			deviceObj.W = maiBid.Device.Width
			deviceObj.H = maiBid.Device.Height
		}
		// Map device type to OpenRTB device type
		switch strings.ToLower(maiBid.Device.DeviceType) {
		case "mobile", "phone":
			deviceObj.DeviceType = 4 // Phone (OpenRTB 2.6: 4=Phone, 5=Tablet, 2=PC, 3=CTV)
		case "tablet":
			deviceObj.DeviceType = 5 // Tablet
		case "desktop", "pc":
			deviceObj.DeviceType = 2 // Personal Computer
		case "tv", "ctv", "connected_tv":
			deviceObj.DeviceType = 3 // Connected TV
		}
	} else {
		deviceObj.UA = r.Header.Get("User-Agent")
	}

	// Parse User-Agent to extract device details (OpenRTB 2.6 enhancement)
	if deviceObj.UA != "" {
		if deviceInfo := device.ParseUserAgent(deviceObj.UA); deviceInfo != nil {
			// Set device make, model, os, osv from parsed UA
			deviceObj.Make = deviceInfo.Make
			deviceObj.Model = deviceInfo.Model
			deviceObj.OS = deviceInfo.OS
			deviceObj.OSV = deviceInfo.OSV

			// Override device type from UA parser if not already set from client
			if deviceObj.DeviceType == 0 {
				deviceObj.DeviceType = deviceInfo.DeviceType
			}

			logger.Log.Debug().
				Str("make", deviceObj.Make).
				Str("model", deviceObj.Model).
				Str("os", deviceObj.OS).
				Str("osv", deviceObj.OSV).
				Int("device_type", deviceObj.DeviceType).
				Msg("Parsed device details from User-Agent")
		}
	}

	// Add geolocation data (OpenRTB 2.6 critical enhancement - 15-30% CPM lift)
	// Priority: Client-side geo (GPS/browser) > IP-based geo
	if maiBid.Device != nil && maiBid.Device.Geo != nil && maiBid.Device.Geo.Lat != 0 && maiBid.Device.Geo.Lon != 0 {
		// Client-side geolocation available (most accurate).
		// Do NOT enrich with IP-based country/region — for VPN/proxy users the IP geo
		// would contradict the GPS coordinates (e.g. GPS=New York, IP=China), producing
		// an incoherent geo object that breaks GDPR enforcement and geo-fenced campaigns.
		deviceObj.Geo = &openrtb.Geo{
			Lat:  maiBid.Device.Geo.Lat,
			Lon:  maiBid.Device.Geo.Lon,
			Type: 1, // GPS/Location Services
		}

		logger.Log.Info().
			Float64("lat", deviceObj.Geo.Lat).
			Float64("lon", deviceObj.Geo.Lon).
			Int("accuracy", maiBid.Device.Geo.Accuracy).
			Msg("Using client-side geolocation (GPS/browser)")
	} else if deviceObj.IP != "" {
		// Fallback to IP-based geolocation
		geoService, err := geo.GetDefaultService()
		if err == nil && geoService != nil {
			// Perform IP geolocation lookup
			if geoInfo := geoService.LookupSafe(deviceObj.IP); geoInfo != nil {
				deviceObj.Geo = &openrtb.Geo{
					Country: geoInfo.Country,
					Region:  geoInfo.Region,
					City:    geoInfo.City,
					Metro:   geoInfo.Metro,
					ZIP:     geoInfo.Zip,
					Lat:     geoInfo.Lat,
					Lon:     geoInfo.Lon,
					Type:    2, // IP-based geolocation
				}

				logger.Log.Info().
					Str("ip", deviceObj.IP).
					Str("country", geoInfo.Country).
					Str("region", geoInfo.Region).
					Str("city", geoInfo.City).
					Float64("lat", geoInfo.Lat).
					Float64("lon", geoInfo.Lon).
					Msg("Added IP-based geolocation to device")
			} else {
				logger.Log.Debug().
					Str("ip", deviceObj.IP).
					Msg("IP geolocation lookup returned no results")
			}
		} else {
			logger.Log.Debug().
				Msg("GeoIP service not available - skipping geolocation (set GEOIP2_DB_PATH env var)")
		}
	}

	// Build user object with IDs from request body AND cookies
	var user *openrtb.User
	var regs *openrtb.Regs

	// Collect user IDs from all sources (priority: request body > database > cookie)
	userIDs := make(map[string]string)

	// Extract FPID from request or cookie for database lookups
	fpid := ""
	if maiBid.User != nil && maiBid.User.FPID != "" {
		fpid = maiBid.User.FPID
	} else if maiBid.User != nil && maiBid.User.Ext != nil {
		// Try to extract FPID from user.ext.eids (SDK sends it here)
		if eids, ok := maiBid.User.Ext["eids"].([]interface{}); ok {
			for _, eidRaw := range eids {
				if eid, ok := eidRaw.(map[string]interface{}); ok {
					if source, ok := eid["source"].(string); ok && source == "thenexusengine.com" {
						if uids, ok := eid["uids"].([]interface{}); ok && len(uids) > 0 {
							if uid, ok := uids[0].(map[string]interface{}); ok {
								if id, ok := uid["id"].(string); ok {
									fpid = id
									logger.Log.Debug().
										Str("fpid", fpid).
										Msg("Extracted FPID from user.ext.eids")
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback to cookie if still not found
	if fpid == "" {
		cookieSync := usersync.ParseCookie(r)
		fpid = cookieSync.GetFPID()
		if fpid != "" {
			logger.Log.Debug().
				Str("fpid", fpid).
				Msg("Extracted FPID from cookie")
		}
	}

	// 1. From database + fresh sync awaiter (concurrent race, prefer fresh)
	// Compute consent hash once for use in both goroutines: UIDs synced with a different
	// consent string (e.g. after user revoked or updated consent) are excluded.
	var currentConsentHash string
	if maiBid.User != nil {
		currentConsentHash = storage.ConsentHash(maiBid.User.ConsentString)
	}

	if fpid != "" && h.userSyncStore != nil {
		type syncResult struct {
			syncs map[string]string
			fresh bool
		}
		resultCh := make(chan syncResult, 2)

		// Path A: cached syncs from DB (fast, ~2ms)
		go func() {
			syncs, err := h.userSyncStore.GetSyncsForUserFiltered(r.Context(), fpid, currentConsentHash)
			if err != nil {
				logger.Log.Warn().Err(err).Str("fpid", fpid).Msg("Failed to load user syncs from database")
				syncs = nil
			}
			resultCh <- syncResult{syncs: syncs, fresh: false}
		}()

		// Path B: fresh sync from in-flight setuid callback (preferred)
		go func() {
			if h.syncAwaiter != nil {
				if signaled := h.syncAwaiter.Wait(r.Context(), fpid, 50*time.Millisecond); signaled {
					syncs, _ := h.userSyncStore.GetSyncsForUserFiltered(r.Context(), fpid, currentConsentHash)
					resultCh <- syncResult{syncs: syncs, fresh: true}
					return
				}
			}
			resultCh <- syncResult{fresh: true} // timeout or no awaiter
		}()

		// Collect both results (always exactly 2 sends on resultCh)
		var cached, fresh map[string]string
		for i := 0; i < 2; i++ {
			res := <-resultCh
			if res.fresh {
				fresh = res.syncs
			} else {
				cached = res.syncs
			}
		}

		// Merge: cached first, fresh overrides (preferred)
		for bidder, uid := range cached {
			userIDs[bidder] = uid
		}
		for bidder, uid := range fresh {
			userIDs[bidder] = uid // fresh wins
		}

		// Telemetry
		switch {
		case len(fresh) > 0:
			logger.Log.Info().
				Str("fpid", fpid).
				Int("fresh_syncs", len(fresh)).
				Int("cached_syncs", len(cached)).
				Strs("bidders", getBidderKeys(fresh)).
				Msg("sync_awaiter_hit")
		case len(cached) > 0:
			logger.Log.Info().
				Str("fpid", fpid).
				Int("syncs_loaded", len(cached)).
				Strs("bidders", getBidderKeys(cached)).
				Msg("Loaded user syncs from database")
		default:
			logger.Log.Info().
				Str("fpid", fpid).
				Msg("sync_awaiter_timeout")
		}
	} else {
		logger.Log.Debug().
			Str("fpid", fpid).
			Bool("userSyncStore_available", h.userSyncStore != nil).
			Msg("Skipping database UID lookup")
	}

	// 2. From request body (SDK sends these) - overrides database
	if maiBid.User != nil && len(maiBid.User.UserIds) > 0 {
		for bidder, uid := range maiBid.User.UserIds {
			userIDs[bidder] = uid
		}
	}

	// 3. From HTTP cookie as final fallback (least reliable due to cookie restrictions)
	cookieSync := usersync.ParseCookie(r)
	bidders := []string{"rubicon", "kargo", "sovrn", "pubmatic", "triplelift", "appnexus"}
	for _, bidder := range bidders {
		if uid := cookieSync.GetUID(bidder); uid != "" {
			if _, exists := userIDs[bidder]; !exists {
				userIDs[bidder] = uid
			}
		}
	}

	// Sync coverage telemetry — measure UID availability at bid time
	logger.Log.Info().
		Str("fpid", fpid).
		Int("uid_count", len(userIDs)).
		Bool("has_syncs", len(userIDs) > 0).
		Strs("synced_bidders", getBidderKeys(userIDs)).
		Msg("sync_coverage")

	// Log final user ID collection status before building OpenRTB request
	logger.Log.Info().
		Str("fpid", fpid).
		Int("total_user_ids", len(userIDs)).
		Strs("bidders_with_uids", getBidderKeys(userIDs)).
		Msg("User IDs available for auction")

	// Create user object if we have IDs or consent data
	if len(userIDs) > 0 || maiBid.User != nil {
		user = &openrtb.User{}

		// Set FPID as OpenRTB user.id (first-party identifier)
		if maiBid.User != nil && maiBid.User.FPID != "" {
			user.ID = maiBid.User.FPID
			logger.Log.Debug().
				Str("fpid", maiBid.User.FPID).
				Msg("Including FPID in bid request as user.id")
		}

		// Build user.ext.eids from all sources, deduplicating by source domain.
		// Priority order:
		//   1. Prebid ID module EIDs (id5, pubCommonId, …) sent by SDK in user.eids
		//   2. SDK-provided user.ext.eids (legacy path, same EIDs may arrive here)
		//   3. Server-side bidder UIDs from database / cookie sync
		hasPrebidEids := maiBid.User != nil && len(maiBid.User.Eids) > 0
		hasExtEids := maiBid.User != nil && maiBid.User.Ext != nil
		if hasExtEids {
			if _, ok := maiBid.User.Ext["eids"]; !ok {
				hasExtEids = false
			}
		}
		if len(userIDs) > 0 || hasPrebidEids || hasExtEids {
			seenSources := make(map[string]struct{})
			eids := make([]map[string]interface{}, 0)

			// 1. Prebid ID module EIDs (id5-sync.com, pubcid.org, etc.)
			if hasPrebidEids {
				for _, eid := range maiBid.User.Eids {
					if src, ok := eid["source"].(string); ok && src != "" {
						seenSources[src] = struct{}{}
					}
					eids = append(eids, eid)
				}
			}

			// 2. SDK's user.ext.eids (may overlap with above — skip duplicates)
			userExt := make(map[string]interface{})
			if maiBid.User != nil && maiBid.User.Ext != nil {
				for k, v := range maiBid.User.Ext {
					userExt[k] = v
				}
			}
			if existingEids, ok := userExt["eids"].([]interface{}); ok {
				for _, raw := range existingEids {
					if eid, ok := raw.(map[string]interface{}); ok {
						src, _ := eid["source"].(string)
						if _, seen := seenSources[src]; seen {
							continue
						}
						if src != "" {
							seenSources[src] = struct{}{}
						}
						eids = append(eids, eid)
					}
				}
			}

			// 3. Server-side bidder UIDs
			sourceDomains := map[string]string{
				"rubicon":    "rubiconproject.com",
				"kargo":      "kargo.com",
				"sovrn":      "lijit.com",
				"pubmatic":   "pubmatic.com",
				"triplelift": "3lift.com",
				"appnexus":   "adnxs.com",
			}
			for bidder, uid := range userIDs {
				source := sourceDomains[bidder]
				if source == "" {
					source = bidder + ".com"
				}
				if _, seen := seenSources[source]; seen {
					continue
				}
				seenSources[source] = struct{}{}
				eids = append(eids, map[string]interface{}{
					"source": source,
					"uids": []map[string]interface{}{
						{"id": uid, "atype": 1},
					},
				})
			}

			userExt["eids"] = eids
			extJSON, _ := json.Marshal(userExt)
			user.Ext = extJSON

			// Also populate user.EIDs (typed struct) so the EID filter in exchange.go
			// can enforce the allowed-source list. Without this, EIDs only exist in
			// user.ext.eids (raw JSON) and bypass ProcessRequestEIDs entirely.
			typedEIDs := make([]openrtb.EID, 0, len(eids))
			for _, eid := range eids {
				src, _ := eid["source"].(string)
				typed := openrtb.EID{Source: src}

				// uids may be []map[string]interface{} (built in-process) or
				// []interface{} (JSON-decoded from Prebid ID module). Handle both.
				switch rawUIDs := eid["uids"].(type) {
				case []map[string]interface{}:
					for _, u := range rawUIDs {
						uid := openrtb.UID{}
						if id, ok := u["id"].(string); ok {
							uid.ID = id
						}
						// atype may be int (in-process literal) or float64 (JSON-decoded).
						switch v := u["atype"].(type) {
						case int:
							uid.AType = v
						case float64:
							uid.AType = int(v)
						}
						typed.UIDs = append(typed.UIDs, uid)
					}
				case []interface{}:
					for _, raw := range rawUIDs {
						if u, ok := raw.(map[string]interface{}); ok {
							uid := openrtb.UID{}
							if id, ok := u["id"].(string); ok {
								uid.ID = id
							}
							// JSON numbers decode as float64
							if atype, ok := u["atype"].(float64); ok {
								uid.AType = int(atype)
							}
							typed.UIDs = append(typed.UIDs, uid)
						}
					}
				}
				typedEIDs = append(typedEIDs, typed)
			}
			user.EIDs = typedEIDs

			logger.Log.Info().
				Str("fpid", user.ID).
				Int("prebid_eids", len(maiBid.User.Eids)).
				Int("server_uids", len(userIDs)).
				Int("total_eids", len(eids)).
				Msg("Populated user EIDs for auction")
		} else if maiBid.User != nil && maiBid.User.Ext != nil {
			// No server-side UIDs or Prebid EIDs, but always preserve SDK's user.ext
			// (contains the FPID eid used by identity_gating hook).
			userExt := make(map[string]interface{})
			for k, v := range maiBid.User.Ext {
				userExt[k] = v
			}
			extJSON, _ := json.Marshal(userExt)
			user.Ext = extJSON
		}

		// Set TCF consent string (OpenRTB 2.5+ compliance)
		// Only populate user.Consent when we have an actual TCF v2 string.
		// "1" / "0" are not valid consent strings and confuse SSPs on non-GDPR traffic.
		if maiBid.User != nil {
			if maiBid.User.ConsentString != "" {
				extMap := make(map[string]interface{})
				if user.Ext != nil {
					_ = json.Unmarshal(user.Ext, &extMap)
				}
				extMap["consent"] = maiBid.User.ConsentString
				consentExtJSON, _ := json.Marshal(extMap)
				user.Ext = consentExtJSON
				previewLen := min(20, len(maiBid.User.ConsentString))
				logger.Log.Debug().
					Str("consent_string_preview", maiBid.User.ConsentString[:previewLen]).
					Msg("Using TCFv2 consent string in user.ext.consent")
			}

			// Format user.data segments for first-party data targeting (OpenRTB 2.6)
			if len(maiBid.User.Data) > 0 {
				user.Data = make([]openrtb.Data, 0, len(maiBid.User.Data))
				for _, segment := range maiBid.User.Data {
					data := openrtb.Data{}

					// Extract name if present
					if name, ok := segment["name"].(string); ok {
						data.Name = name
					}

					// Extract ID if present
					if id, ok := segment["id"].(string); ok {
						data.ID = id
					}

					// Extract segments if present
					if segs, ok := segment["segment"].([]interface{}); ok {
						data.Segment = make([]openrtb.Segment, 0, len(segs))
						for _, seg := range segs {
							if segMap, ok := seg.(map[string]interface{}); ok {
								segment := openrtb.Segment{}
								if id, ok := segMap["id"].(string); ok {
									segment.ID = id
								}
								if name, ok := segMap["name"].(string); ok {
									segment.Name = name
								}
								if value, ok := segMap["value"].(string); ok {
									segment.Value = value
								}
								data.Segment = append(data.Segment, segment)
							}
						}
					}

					user.Data = append(user.Data, data)
				}

				logger.Log.Info().
					Int("data_segments", len(user.Data)).
					Msg("Formatted user.data segments for OpenRTB")
			}
		}
	}

	// Handle regulations (GDPR / CCPA / GPP)
	//
	// US traffic:  GDPR never applies. Use GPP (SID 7 = US National minimum).
	//              Pass through the GPP string from the client __gpp() API if present.
	// Non-US:      Trust client gdprApplies when sent; fall back to IP geo otherwise.
	//
	// US detection uses IP geo country, which is always available regardless of
	// whether the client also sent GPS coordinates.
	regs = &openrtb.Regs{}

	// Resolve IP country — deviceObj.Geo.Country is set by IP geo lookup but empty
	// when GPS coordinates were used instead. In that case do a second lookup just
	// for compliance purposes (fast in-memory MaxMind call).
	ipCountry := ""
	if deviceObj.Geo != nil {
		ipCountry = deviceObj.Geo.Country
	}
	if ipCountry == "" && deviceObj.IP != "" {
		if geoService, err := geo.GetDefaultService(); err == nil && geoService != nil {
			if geoInfo := geoService.LookupSafe(deviceObj.IP); geoInfo != nil {
				ipCountry = geoInfo.Country
			}
		}
	}
	isUS := ipCountry == "US"

	if isUS {
		// US traffic: GDPR does not apply
		gdpr := 0
		regs.GDPR = &gdpr

		// GPP — use string from client __gpp() if provided, otherwise signal US National
		if maiBid.User != nil && maiBid.User.GPPString != "" {
			regs.GPP = maiBid.User.GPPString
			regs.GPPSID = maiBid.User.GPPSIDs
		} else {
			// SID 7 = US National Privacy (MSPA) — minimum signal for US traffic
			regs.GPPSID = []int{7}
		}

		logger.Log.Debug().
			Str("ip_country", ipCountry).
			Msg("US traffic: GDPR disabled, GPP applied")
	} else if maiBid.User != nil {
		// Non-US: trust client gdprApplies; fall back to IP geo if absent
		var gdprApplies bool
		if maiBid.User.GDPRApplies != nil {
			gdprApplies = *maiBid.User.GDPRApplies
		} else {
			gdprApplies = middleware.DetectRegulationFromGeo(deviceObj.Geo) == middleware.RegulationGDPR
		}
		if gdprApplies {
			gdpr := 1
			regs.GDPR = &gdpr
		}

		// Pass through GPP string from client if provided for non-US regions
		if maiBid.User.GPPString != "" {
			regs.GPP = maiBid.User.GPPString
			regs.GPPSID = maiBid.User.GPPSIDs
		}
	}

	// USP string: set at top-level regs.us_privacy (OpenRTB 2.6).
	// Sovrn reads Regs.USPrivacy for its X-US-Privacy header.
	// Pubmatic adapter automatically moves it to regs.ext.us_privacy.
	// All other adapters get it in the marshaled regs.us_privacy field.
	if maiBid.User != nil && maiBid.User.USPConsent != "" {
		regs.USPrivacy = maiBid.User.USPConsent
	}

	// Source object - schain is built per-bidder by SChainAugmentationHook
	// using the bidder-specific seller ID assigned by each SSP
	source := &openrtb.Source{}

	// Build OpenRTB request
	ortbReq := &openrtb.BidRequest{
		ID:     requestID,
		AT:     1, // First-price auction
		Imp:    imps,
		Site:   site,
		Device: deviceObj,
		User:   user,
		Regs:   regs,
		Source: source, // Supply chain transparency
		Cur:  []string{"USD"},
		TMax: publisherTMaxMs(r.Context()),
	}

	logger.Log.Debug().
		Str("account_id", maiBid.AccountID).
		Msg("Added supply chain (schain) to bid request")

	return ortbReq, impToSlot, nil
}

// convertToMAIResponse converts OpenRTB response to MAI format
func (h *CatalystBidHandler) convertToMAIResponse(auctionResp *exchange.AuctionResponse, impToSlot map[string]string) *MAIBidResponse {
	maiResp := &MAIBidResponse{
		Bids: []MAIBid{},
	}

	if auctionResp == nil || auctionResp.BidResponse == nil {
		return maiResp
	}

	// Extract all bids from all seats
	for _, seatBid := range auctionResp.BidResponse.SeatBid {
		for _, bid := range seatBid.Bid {
			// Map impression ID back to divID
			divID, ok := impToSlot[bid.ImpID]
			if !ok {
				continue // Skip if we can't map back to slot
			}

			maiBid := MAIBid{
				DivID:      divID,
				CPM:        bid.Price,
				Currency:   auctionResp.BidResponse.Cur,
				Width:      bid.W,
				Height:     bid.H,
				AdID:       bid.ID,
				CreativeID: bid.CRID,
				DealID:     bid.DealID,
			}

			// Set default currency if not specified
			if maiBid.Currency == "" {
				maiBid.Currency = "USD"
			}

			// Build metadata
			if len(bid.ADomain) > 0 || bid.CID != "" {
				maiBid.Meta = &MAIBidMeta{
					AdvertiserDomains: bid.ADomain,
					NetworkID:         bid.CID,
					NetworkName:       seatBid.Seat,
				}
			}

			// Extract pre-built GAM targeting keys from bid extension
			// These are set by exchange.buildBidExtension and include _catalyst keys
			if len(bid.Ext) > 0 {
				var bidExt openrtb.BidExt
				if err := json.Unmarshal(bid.Ext, &bidExt); err == nil && bidExt.Prebid != nil && len(bidExt.Prebid.Targeting) > 0 {
					maiBid.Targeting = bidExt.Prebid.Targeting
				}
			}

			maiResp.Bids = append(maiResp.Bids, maiBid)
		}
	}

	return maiResp
}

// writeMAIResponse writes MAI-formatted JSON response
func (h *CatalystBidHandler) writeMAIResponse(w http.ResponseWriter, resp *MAIBidResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to encode MAI response")
	}
}

// writeErrorResponse writes error response
func (h *CatalystBidHandler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// extractBidderParamsFromMapping extracts bidder-specific params from legacy mapping file
// Note: This maintains backward compatibility with integer values in the mapping file
func (h *CatalystBidHandler) extractBidderParamsFromMapping(bidderCode string, adUnitConfig *AdUnitConfig) map[string]interface{} {
	switch bidderCode {
	case "rubicon":
		if adUnitConfig.Rubicon != nil {
			return map[string]interface{}{
				"accountId":        adUnitConfig.Rubicon.AccountID,        // int (correct)
				"siteId":           adUnitConfig.Rubicon.SiteID,           // int (correct)
				"zoneId":           adUnitConfig.Rubicon.ZoneID,           // int (correct)
				"bidonmultiformat": adUnitConfig.Rubicon.BidOnMultiFormat, // bool (correct)
			}
		}
	case "kargo":
		if adUnitConfig.Kargo != nil {
			return map[string]interface{}{
				"placementId": adUnitConfig.Kargo.PlacementID, // string (correct)
			}
		}
	case "sovrn":
		if adUnitConfig.Sovrn != nil {
			// Note: tagid must be string per Prebid Server spec
			return map[string]interface{}{
				"tagid": adUnitConfig.Sovrn.TagID, // string (will be converted from mapping)
			}
		}
	case "pubmatic":
		if adUnitConfig.Pubmatic != nil {
			// Note: publisherId and adSlot must be strings per Prebid Server spec
			return map[string]interface{}{
				"publisherId": adUnitConfig.Pubmatic.PublisherID, // string (will be converted from mapping)
				"adSlot":      adUnitConfig.Pubmatic.AdSlot,      // string (will be converted from mapping)
			}
		}
	case "triplelift":
		if adUnitConfig.Triplelift != nil {
			return map[string]interface{}{
				"inventoryCode": adUnitConfig.Triplelift.InventoryCode, // string (correct)
			}
		}
	}
	return nil
}

// getBidderKeys extracts keys from user IDs map for logging
func getBidderKeys(userIDs map[string]string) []string {
	keys := make([]string, 0, len(userIDs))
	for k := range userIDs {
		keys = append(keys, k)
	}
	return keys
}

// getBidderNames extracts keys from bidder config map for logging
func getBidderNames(impExt map[string]interface{}) []string {
	keys := make([]string, 0, len(impExt))
	for k := range impExt {
		keys = append(keys, k)
	}
	return keys
}


// getConfiguredBidders extracts bidder names from config map for logging
func getConfiguredBidders(configs map[string]map[string]interface{}) []string {
	keys := make([]string, 0, len(configs))
	for k := range configs {
		keys = append(keys, k)
	}
	return keys
}

// detectDeviceType determines device type from User-Agent string
// publisherTMaxMs returns the auction timeout for the publisher in context.
// Falls back to 1500 ms if no publisher is configured or their timeout is unset.
func publisherTMaxMs(ctx context.Context) int {
	const defaultTMaxMs = 1500
	type tmaxGetter interface {
		GetTMaxMs() int
	}
	if pub := middleware.PublisherFromContext(ctx); pub != nil {
		if g, ok := pub.(tmaxGetter); ok {
			if t := g.GetTMaxMs(); t > 0 {
				return t
			}
		}
	}
	return defaultTMaxMs
}

// Returns "mobile" for mobile devices, "desktop" otherwise
func detectDeviceType(userAgent string) string {
	if userAgent == "" {
		return "desktop" // Default to desktop if no UA
	}

	// Normalize to lowercase for case-insensitive matching
	ua := strings.ToLower(userAgent)

	// Mobile device indicators
	mobileIndicators := []string{
		"mobile", "android", "iphone", "ipad", "ipod",
		"blackberry", "windows phone", "webos", "opera mini",
		"opera mobi", "kindle", "silk", "palm", "symbian",
	}

	for _, indicator := range mobileIndicators {
		if strings.Contains(ua, indicator) {
			return "mobile"
		}
	}

	return "desktop"
}

// countExtEIDs returns the number of EID entries in user.ext.eids JSON.
// Used by CP-1 telemetry to compare top-level vs ext EID counts.
func countExtEIDs(ext json.RawMessage) int {
	if len(ext) == 0 {
		return 0
	}
	var parsed struct {
		EIDs []json.RawMessage `json:"eids"`
	}
	if err := json.Unmarshal(ext, &parsed); err != nil {
		return 0
	}
	return len(parsed.EIDs)
}
