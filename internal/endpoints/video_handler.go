package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thenexusengine/tne_springwire/internal/ctv"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/vast"
)

// VideoHandler handles video ad requests and returns VAST responses
type VideoHandler struct {
	exchange        *exchange.Exchange
	vastBuilder     *exchange.VASTResponseBuilder
	trackingBaseURL string
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(ex *exchange.Exchange, trackingBaseURL string) *VideoHandler {
	return &VideoHandler{
		exchange:        ex,
		vastBuilder:     exchange.NewVASTResponseBuilder(trackingBaseURL),
		trackingBaseURL: trackingBaseURL,
	}
}

// HandleVASTRequest handles GET /video/vast requests
// This endpoint accepts query parameters and returns a VAST XML response
func (h *VideoHandler) HandleVASTRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse video parameters from query string
	bidReq, err := h.parseVASTRequest(r)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid VAST request parameters")
		h.writeVASTError(w, "Invalid request parameters")
		return
	}

	// Detect CTV device for optimization
	if bidReq.Device != nil {
		deviceInfo := ctv.DetectDevice(bidReq.Device)
		if deviceInfo.IsCTV {
			h.applyCTVOptimizations(bidReq, deviceInfo)
		}
	}

	// Create auction request
	auctionReq := &exchange.AuctionRequest{
		BidRequest: bidReq,
		Timeout:    time.Duration(bidReq.TMax) * time.Millisecond,
	}

	// Run auction through exchange
	auctionResp, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		log.Error().Err(err).Msg("Video auction failed")
		h.writeVASTError(w, "Auction failed")
		return
	}

	// Build VAST response from auction results
	vastResp, err := h.vastBuilder.BuildVASTFromAuction(bidReq, auctionResp)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build VAST response")
		h.writeVASTError(w, "Failed to build response")
		return
	}

	// Marshal and write VAST XML
	data, err := vastResp.Marshal()
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal VAST")
		h.writeVASTError(w, "Failed to serialize response")
		return
	}

	// Set headers and write response
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// SECURITY NOTE: CORS wildcard (*) is intentional for VAST endpoints.
	// VAST/VPAID video players are typically embedded in iframes or third-party
	// contexts (e.g., video.js, JW Player, Brightcove) and require permissive CORS
	// to fetch ad responses. This is an IAB industry-standard practice for VAST
	// ad serving endpoints. The VAST response contains only ad markup, not
	// sensitive user data, so wildcard CORS does not create a security risk.
	// See: IAB VAST 4.2 spec section on "Cross-Origin Resource Sharing"
	h.setVASTCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	log.Info().
		Str("request_id", bidReq.ID).
		Bool("has_ads", !vastResp.IsEmpty()).
		Msg("VAST response sent")
}

// HandleOpenRTBVideo handles POST /video/openrtb requests
// This endpoint accepts OpenRTB JSON and returns VAST XML
func (h *VideoHandler) HandleOpenRTBVideo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse OpenRTB bid request from body
	var bidReq openrtb.BidRequest
	if err := json.NewDecoder(r.Body).Decode(&bidReq); err != nil {
		log.Warn().Err(err).Msg("Invalid OpenRTB request body")
		h.writeVASTError(w, "Invalid request body")
		return
	}

	// Validate that this is a video request
	hasVideo := false
	for _, imp := range bidReq.Imp {
		if imp.Video != nil {
			hasVideo = true
			break
		}
	}
	if !hasVideo {
		h.writeVASTError(w, "No video impressions in request")
		return
	}

	// Run auction
	auctionReq := &exchange.AuctionRequest{
		BidRequest: &bidReq,
		Timeout:    time.Duration(bidReq.TMax) * time.Millisecond,
	}

	auctionResp, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		log.Error().Err(err).Msg("Video auction failed")
		h.writeVASTError(w, "Auction failed")
		return
	}

	// Build VAST response
	vastResp, err := h.vastBuilder.BuildVASTFromAuction(&bidReq, auctionResp)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build VAST response")
		h.writeVASTError(w, "Failed to build response")
		return
	}

	// Marshal and write VAST XML
	data, err := vastResp.Marshal()
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal VAST")
		h.writeVASTError(w, "Failed to serialize response")
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// SECURITY NOTE: CORS wildcard intentional for VAST - see setVASTCORSHeaders
	h.setVASTCORSHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// setVASTCORSHeaders sets CORS headers for VAST responses.
//
// SECURITY RATIONALE: VAST endpoints intentionally use permissive CORS (Access-Control-Allow-Origin: *)
// because video players (video.js, JW Player, Brightcove, etc.) are typically embedded in third-party
// iframes and require cross-origin access to fetch ad responses. This is standard practice per IAB
// VAST specification. VAST XML contains only ad markup (media URLs, tracking pixels, etc.) and does
// not include sensitive user data, so wildcard CORS does not create a data exposure risk.
//
// This is distinct from the /openrtb2/auction endpoint which handles bid requests containing
// potentially sensitive user data and uses the configurable CORS middleware.
func (h *VideoHandler) setVASTCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
}

// parseVASTRequest parses video parameters from query string into OpenRTB bid request
func (h *VideoHandler) parseVASTRequest(r *http.Request) (*openrtb.BidRequest, error) {
	q := r.URL.Query()

	// Required parameters
	requestID := q.Get("id")
	if requestID == "" {
		requestID = generateRequestID()
	}

	// Video dimensions (default to 1920x1080)
	width := parseInt(q.Get("w"), 1920)
	height := parseInt(q.Get("h"), 1080)

	// Duration constraints
	minDuration := parseInt(q.Get("mindur"), 5)
	maxDuration := parseInt(q.Get("maxdur"), 30)

	// Skip parameters
	skip := parseInt(q.Get("skip"), 0)
	skipAfter := parseInt(q.Get("skipafter"), 0)

	// Placement type (1=in-stream, 3=in-article, 4=in-feed, 5=interstitial)
	placement := parseInt(q.Get("placement"), 1)

	// Protocols (comma-separated)
	protocols := parseIntArray(q.Get("protocols"), []int{2, 3, 5, 6})

	// MIME types (comma-separated)
	mimes := parseStringArray(q.Get("mimes"), []string{"video/mp4", "video/webm"})

	// Bitrate
	minBitrate := parseInt(q.Get("minbitrate"), 300)
	maxBitrate := parseInt(q.Get("maxbitrate"), 5000)

	// Floor price
	bidFloor := parseFloat(q.Get("bidfloor"), 0.0)

	// Build video object
	video := &openrtb.Video{
		Mimes:       mimes,
		MinDuration: minDuration,
		MaxDuration: maxDuration,
		Protocols:   protocols,
		W:           width,
		H:           height,
		Placement:   placement,
		Linearity:   1, // Linear/in-stream
		MinBitrate:  minBitrate,
		MaxBitrate:  maxBitrate,
		API:         []int{1, 2}, // VPAID 1.0 and 2.0
	}

	if skip == 1 {
		skipInt := skip
		video.Skip = &skipInt
		video.SkipAfter = skipAfter
	}

	// Build impression
	imp := openrtb.Imp{
		ID:          "1",
		Video:       video,
		BidFloor:    bidFloor,
		BidFloorCur: "USD",
	}

	// Build device from headers
	device := &openrtb.Device{
		UA: r.UserAgent(),
		IP: getClientIP(r),
		W:  width,
		H:  height,
	}

	// Build bid request
	bidReq := &openrtb.BidRequest{
		ID:   requestID,
		Imp:  []openrtb.Imp{imp},
		Device: device,
		TMax: 1000, // 1 second timeout
		Cur:  []string{"USD"},
		AT:   2, // Second-price auction
	}

	// Add site or app info if provided
	// Create Site if site_id OR domain is provided (OpenRTB allows ID to be optional)
	siteID := q.Get("site_id")
	domain := q.Get("domain")
	page := q.Get("page")

	if siteID != "" || domain != "" {
		bidReq.Site = &openrtb.Site{
			ID:     siteID,
			Domain: domain,
			Page:   page,
		}
	}

	return bidReq, nil
}

// writeVASTError writes a VAST error response
func (h *VideoHandler) writeVASTError(w http.ResponseWriter, message string) {
	// SECURITY: Escape message parameter to prevent URL injection (CVE-2026-XXXX)
	v := vast.CreateErrorVAST(fmt.Sprintf("%s/video/error?msg=%s", h.trackingBaseURL, url.QueryEscape(message)))
	data, _ := v.Marshal()

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// SECURITY NOTE: CORS wildcard intentional for VAST error responses - see setVASTCORSHeaders
	h.setVASTCORSHeaders(w)
	w.WriteHeader(http.StatusOK) // VAST always returns 200
	w.Write(data)
}

// applyCTVOptimizations applies CTV device-specific optimizations
func (h *VideoHandler) applyCTVOptimizations(bidReq *openrtb.BidRequest, deviceInfo *ctv.DeviceInfo) {
	caps := ctv.GetCapabilities(deviceInfo.Type)

	for i := range bidReq.Imp {
		if bidReq.Imp[i].Video != nil {
			// Limit bitrate based on device capabilities
			if bidReq.Imp[i].Video.MaxBitrate > caps.MaxBitrate {
				bidReq.Imp[i].Video.MaxBitrate = caps.MaxBitrate
			}

			// Filter VPAID if not supported
			if !caps.SupportsVPAID {
				filtered := make([]int, 0)
				for _, api := range bidReq.Imp[i].Video.API {
					if api != 1 && api != 2 { // Remove VPAID 1.0 and 2.0
						filtered = append(filtered, api)
					}
				}
				bidReq.Imp[i].Video.API = filtered
			}
		}
	}
}

// Helper functions

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseFloat(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseIntArray(s string, defaultVal []int) []int {
	if s == "" {
		return defaultVal
	}
	var result []int
	for _, part := range parseStringArray(s, nil) {
		if val, err := strconv.Atoi(part); err == nil {
			result = append(result, val)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}

func parseStringArray(s string, defaultVal []string) []string {
	if s == "" {
		return defaultVal
	}
	// Split by comma
	parts := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func generateRequestID() string {
	return fmt.Sprintf("video-%d", time.Now().UnixNano())
}

// HandleVASTWrapper handles GET /video/wrapper requests.
// Returns a VAST 4.0 Wrapper pointing to the /video/vast auction endpoint,
// with TNE impression and quartile tracking pixels injected at the wrapper level.
func (h *VideoHandler) HandleVASTWrapper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	requestID := q.Get("id")
	if requestID == "" {
		requestID = generateRequestID()
	}

	auctionURL := h.buildAuctionURL(q, requestID)
	vastDoc := h.buildWrapperVAST(requestID, auctionURL)

	data, err := vastDoc.Marshal()
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal VAST wrapper")
		h.writeVASTError(w, "Failed to serialize response")
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// SECURITY NOTE: CORS wildcard intentional for VAST - see setVASTCORSHeaders
	h.setVASTCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	log.Info().
		Str("request_id", requestID).
		Str("auction_url", auctionURL).
		Msg("VAST wrapper response sent")
}

// buildAuctionURL constructs the downstream /video/vast URL by forwarding
// relevant query parameters from the wrapper request.
func (h *VideoHandler) buildAuctionURL(q url.Values, requestID string) string {
	params := url.Values{}
	for _, key := range []string{
		"w", "h", "mindur", "maxdur", "skip", "skipafter",
		"protocols", "mimes", "minbitrate", "maxbitrate",
		"bidfloor", "site_id", "domain", "page", "placement",
	} {
		if v := q.Get(key); v != "" {
			params.Set(key, v)
		}
	}
	params.Set("id", requestID)
	return h.trackingBaseURL + "/video/vast?" + params.Encode()
}

// buildWrapperVAST constructs a VAST 4.0 Wrapper document with TNE tracking pixels.
func (h *VideoHandler) buildWrapperVAST(requestID, auctionURL string) *vast.VAST {
	base := h.trackingBaseURL + "/video/event"
	trackingURL := func(event string) string {
		return fmt.Sprintf("%s?event=%s&bid_id=%s", base, event, url.QueryEscape(requestID))
	}

	return &vast.VAST{
		Version: "4.0",
		Ads: []vast.Ad{
			{
				ID: requestID,
				Wrapper: &vast.Wrapper{
					FollowAdditionalWraps: true,
					FallbackOnNoAd:        true,
					AdSystem:              vast.AdSystem{Value: "TNEVideo"},
					VASTAdTagURI:          auctionURL,
					Impressions: []vast.Impression{
						{ID: "tne-imp", Value: trackingURL("impression")},
					},
					Creatives: vast.Creatives{
						Creative: []vast.Creative{
							{
								ID: requestID + "-tracking",
								Linear: &vast.Linear{
									TrackingEvents: vast.TrackingEvents{
										Tracking: []vast.Tracking{
											{Event: vast.EventStart, Value: trackingURL(vast.EventStart)},
											{Event: vast.EventFirstQuartile, Value: trackingURL(vast.EventFirstQuartile)},
											{Event: vast.EventMidpoint, Value: trackingURL(vast.EventMidpoint)},
											{Event: vast.EventThirdQuartile, Value: trackingURL(vast.EventThirdQuartile)},
											{Event: vast.EventComplete, Value: trackingURL(vast.EventComplete)},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
