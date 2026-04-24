package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thenexusengine/tne_springwire/internal/ctv"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/vast"
)

// VideoHandler handles video ad requests and returns VAST responses
type VideoHandler struct {
	exchange        *exchange.Exchange
	vastBuilder     *exchange.VASTResponseBuilder
	trackingBaseURL string
	publishers      *storage.PublisherStore
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(ex *exchange.Exchange, trackingBaseURL string, publishers *storage.PublisherStore) *VideoHandler {
	return &VideoHandler{
		exchange:        ex,
		vastBuilder:     exchange.NewVASTResponseBuilder(trackingBaseURL),
		trackingBaseURL: trackingBaseURL,
		publishers:      publishers,
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

// parseVASTRequest parses video parameters from query string into OpenRTB bid request.
// If a pub= param is present, publisher video config is loaded from the DB and used
// as defaults — URL params always win over DB config, DB config wins over code defaults.
func (h *VideoHandler) parseVASTRequest(r *http.Request) (*openrtb.BidRequest, error) {
	q := r.URL.Query()

	// Request ID: prefer IAB VAST 4 [TRANSACTIONID] from the player, then
	// explicit id=, else auto-generate. cleanMacro strips "-1" per IAB spec
	// and empty values, so an unsupported macro falls through cleanly.
	requestID := cleanMacro(q.Get("transaction_id"))
	if requestID == "" {
		requestID = q.Get("id")
	}
	if requestID == "" {
		requestID = generateRequestID()
	}

	// Publisher ID: accept GAM's pub_id first, fall back to legacy pub.
	pubID := cleanMacro(q.Get("pub_id"))
	if pubID == "" {
		pubID = q.Get("pub")
	}

	// Load publisher video config if we have a publisher ID
	var vidCfg *storage.PublisherVideoConfig
	if pubID != "" && h.publishers != nil {
		if pub, err := h.publishers.GetByPublisherID(r.Context(), pubID); err == nil && pub != nil {
			if p, ok := pub.(*storage.Publisher); ok {
				vidCfg = p.GetVideoConfig()
			}
		}
	}

	// Helper: return url param value if present, else publisher config value, else fallback
	intParam := func(key string, cfgVal, fallback int) int {
		if v := q.Get(key); v != "" {
			return parseInt(v, fallback)
		}
		if cfgVal != 0 {
			return cfgVal
		}
		return fallback
	}
	intArrayParam := func(key string, cfgVal, fallback []int) []int {
		if v := q.Get(key); v != "" {
			return parseIntArray(v, fallback)
		}
		if len(cfgVal) > 0 {
			return cfgVal
		}
		return fallback
	}
	stringArrayParam := func(key string, cfgVal, fallback []string) []string {
		if v := q.Get(key); v != "" {
			return parseStringArray(v, fallback)
		}
		if len(cfgVal) > 0 {
			return cfgVal
		}
		return fallback
	}

	// Seed config values (nil-safe)
	var cfgPlacement, cfgMaxDur, cfgMinDur, cfgMinBitrate, cfgMaxBitrate, cfgSkip, cfgSkipAfter int
	var cfgProtocols, cfgPlayback, cfgAPI []int
	var cfgMimes []string
	if vidCfg != nil {
		cfgPlacement = vidCfg.Placement
		cfgMaxDur = vidCfg.MaxDur
		cfgMinDur = vidCfg.MinDur
		cfgMinBitrate = vidCfg.MinBitrate
		cfgMaxBitrate = vidCfg.MaxBitrate
		cfgSkip = vidCfg.Skip
		cfgSkipAfter = vidCfg.SkipAfter
		cfgProtocols = vidCfg.Protocols
		cfgPlayback = vidCfg.PlaybackMethod
		cfgAPI = vidCfg.API
		cfgMimes = vidCfg.Mimes
	}

	// Video dimensions (default to 1920x1080)
	width := parseInt(q.Get("w"), 1920)
	height := parseInt(q.Get("h"), 1080)

	// Duration constraints
	minDuration := intParam("mindur", cfgMinDur, 5)
	maxDuration := intParam("maxdur", cfgMaxDur, 30)

	// Skip parameters
	skip := intParam("skip", cfgSkip, 0)
	skipAfter := intParam("skipafter", cfgSkipAfter, 0)

	// Placement type (1=in-stream, 3=in-article, 4=in-feed, 5=interstitial)
	placement := intParam("placement", cfgPlacement, 1)

	// Protocols (comma-separated)
	protocols := intArrayParam("protocols", cfgProtocols, []int{2, 3, 5, 6})

	// MIME types (comma-separated)
	mimes := stringArrayParam("mimes", cfgMimes, []string{"video/mp4"})

	// Playback method — 1=autoplay sound on, 2=autoplay sound off, 3=click to play
	playbackMethod := intArrayParam("playbackmethod", cfgPlayback, []int{2})

	// API frameworks — 6=VPAID 2.0 JS, 7=OMID-1
	apiFrameworks := intArrayParam("api", cfgAPI, []int{6, 7})

	// Bitrate
	minBitrate := intParam("minbitrate", cfgMinBitrate, 300)
	maxBitrate := intParam("maxbitrate", cfgMaxBitrate, 5000)

	// Floor price
	bidFloor := parseFloat(q.Get("bidfloor"), 0.0)

	// Build video object
	video := &openrtb.Video{
		Mimes:          mimes,
		MinDuration:    minDuration,
		MaxDuration:    maxDuration,
		Protocols:      protocols,
		W:              width,
		H:              height,
		Placement:      placement,
		Linearity:      1, // Linear/in-stream
		MinBitrate:     minBitrate,
		MaxBitrate:     maxBitrate,
		PlaybackMethod: playbackMethod,
		API:            apiFrameworks,
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

	// Build content object if any content params are present (used by both Site and App)
	var content *openrtb.Content
	contentTitle := q.Get("content_title")
	contentSeries := q.Get("content_series")
	contentSeason := q.Get("content_season")
	contentGenre := q.Get("content_genre")
	contentEpisode := parseInt(q.Get("content_episode"), 0)
	livestream := parseInt(q.Get("livestream"), 0)
	if contentTitle != "" || contentSeries != "" || contentGenre != "" {
		content = &openrtb.Content{
			Title:      contentTitle,
			Series:     contentSeries,
			Season:     contentSeason,
			Genre:      contentGenre,
			Episode:    contentEpisode,
			LiveStream: livestream,
		}
	}

	// App object takes precedence for CTV requests.
	// app_bundle may also come from IAB [APPBUNDLE]; cleanMacro drops "-1".
	appBundle := cleanMacro(q.Get("app_bundle"))
	appName := q.Get("app_name")
	if appBundle != "" || appName != "" {
		app := &openrtb.App{
			Bundle:  appBundle,
			Name:    appName,
			Content: content,
		}
		if pubID != "" {
			app.Publisher = &openrtb.Publisher{ID: pubID}
		}
		bidReq.App = app
		return bidReq, nil
	}

	// Site object for web / desktop in-stream. Accept both the legacy
	// page= param and GAM's page_url (%%PAGE_URL%%); ref from %%REFERRER_URL%%.
	siteID := q.Get("site_id")
	domain := q.Get("domain")
	page := q.Get("page")
	if page == "" {
		page = q.Get("page_url")
	}
	ref := q.Get("ref")
	if siteID != "" || domain != "" || page != "" {
		site := &openrtb.Site{
			ID:      siteID,
			Domain:  domain,
			Page:    page,
			Ref:     ref,
			Content: content,
		}
		if pubID != "" {
			site.Publisher = &openrtb.Publisher{ID: pubID}
		}
		bidReq.Site = site
	}

	// Apply GAM + IAB VAST 4 enrichments (placement_id, KVs, privacy,
	// identifiers, player/content context). Mutates bidReq in place.
	h.applyVASTEnrichments(q, bidReq)

	return bidReq, nil
}

// applyVASTEnrichments layers GAM macro + IAB VAST 4 player signals onto the
// bid request built by parseVASTRequest. Maps each query param to its
// OpenRTB 2.6 home and enforces the two-layer resolution rules documented
// in docs/integrations/video-vast/SETUP.md:
//
//   - imp.video.w/h         → player_size (IAB) wins over GAM w/h
//   - imp.video.placement   → placement_type (IAB) wins over placement= default
//   - device.geo.*          → GAM geo country + IAB lat_long precision, merged
//
// Values of "-1" (IAB "macro unsupported" sentinel) and empty strings are
// treated as absent so the handler never overwrites a real default with a
// placeholder.
func (h *VideoHandler) applyVASTEnrichments(q url.Values, bidReq *openrtb.BidRequest) {
	if len(bidReq.Imp) == 0 {
		return
	}
	imp := &bidReq.Imp[0]

	// --- Placement / ad unit ---
	if tagID := cleanMacro(q.Get("placement_id")); tagID != "" {
		imp.TagID = tagID
	}

	// --- Player size override (IAB [PLAYERSIZE] = "WxH") ---
	if w, hgt, ok := parsePlayerSize(q.Get("player_size")); ok && imp.Video != nil {
		imp.Video.W = w
		imp.Video.H = hgt
		if bidReq.Device != nil {
			bidReq.Device.W = w
			bidReq.Device.H = hgt
		}
	}

	// --- Placement type override (IAB [PLACEMENTTYPE] = IAB enum int) ---
	if pt := cleanMacro(q.Get("placement_type")); pt != "" {
		if n, err := strconv.Atoi(pt); err == nil && n > 0 && imp.Video != nil {
			imp.Video.Placement = n
			imp.Video.Plcmt = n // OpenRTB 2.6 field
		}
	}

	// --- OMID partner / verification vendors ---
	if omid := cleanMacro(q.Get("omid_partner")); omid != "" && imp.Video != nil {
		imp.Video.Ext = setExtKey(imp.Video.Ext, []string{"omidpartner"}, omid)
	}
	if vv := cleanMacro(q.Get("verification_vendors")); vv != "" {
		imp.Ext = setExtKey(imp.Ext, []string{"verification"}, splitCSV(vv))
	}

	// --- Inventory state (autoplay/muted bitmask from the player) ---
	if inv := cleanMacro(q.Get("inventory_state")); inv != "" {
		imp.Ext = setExtKey(imp.Ext, []string{"inventorystate"}, inv)
	}

	// --- Content enrichments (populate site.content or app.content) ---
	contentID := cleanMacro(q.Get("content_id"))
	contentURL := cleanMacro(q.Get("content_url"))
	sport := cleanMacro(q.Get("sport"))
	competition := cleanMacro(q.Get("competition"))
	lang := cleanMacro(q.Get("lang"))
	contentType := cleanMacro(q.Get("content_type"))

	if contentID != "" || contentURL != "" || sport != "" || competition != "" || lang != "" || contentType != "" {
		content := resolveContent(bidReq)
		if contentID != "" {
			content.ID = contentID
		}
		if contentURL != "" {
			content.URL = contentURL
		}
		if sport != "" && content.Genre == "" {
			content.Genre = sport
		}
		if competition != "" && content.Series == "" {
			content.Series = competition
		}
		if lang != "" {
			content.Language = lang
			content.LangB = lang
		}
		if contentType != "" {
			content.Cat = appendUnique(content.Cat, contentType)
		}
	}

	// --- KVs → imp.ext.context (forward-compat channel; every SSP reads this) ---
	for _, kv := range []struct{ param, key string }{
		{"sport", "sport"},
		{"competition", "competition"},
		{"content_type", "content_type"},
		{"lang", "language"},
	} {
		if v := cleanMacro(q.Get(kv.param)); v != "" {
			imp.Ext = setExtKey(imp.Ext, []string{"context", kv.key}, v)
		}
	}

	// --- Device enrichments ---
	if bidReq.Device == nil {
		bidReq.Device = &openrtb.Device{}
	}
	dev := bidReq.Device

	if dt := cleanMacro(q.Get("device_type")); dt != "" {
		if n := parseDeviceType(dt); n > 0 {
			dev.DeviceType = n
		}
	}
	if ifa := cleanMacro(q.Get("ifa")); ifa != "" {
		dev.IFA = ifa
	}
	if ifaType := cleanMacro(q.Get("ifa_type")); ifaType != "" {
		dev.Ext = setExtKey(dev.Ext, []string{"ifa_type"}, ifaType)
	}
	if lmt := cleanMacro(q.Get("lmt")); lmt != "" {
		if n, err := strconv.Atoi(lmt); err == nil {
			dev.Lmt = &n
		}
	}

	// Geo: GAM country + IAB lat/lng merged.
	geoCountry := cleanMacro(q.Get("geo"))
	lat, lon, hasLatLon := parseLatLong(q.Get("lat_long"))
	if geoCountry != "" || hasLatLon {
		if dev.Geo == nil {
			dev.Geo = &openrtb.Geo{}
		}
		if geoCountry != "" {
			dev.Geo.Country = geoCountry
		}
		if hasLatLon {
			dev.Geo.Lat = lat
			dev.Geo.Lon = lon
		}
	}

	// --- Regs (GDPR flag, COPPA, US Privacy, GPP) ---
	regs := bidReq.Regs
	ensureRegs := func() *openrtb.Regs {
		if regs == nil {
			regs = &openrtb.Regs{}
			bidReq.Regs = regs
		}
		return regs
	}
	if gdpr := cleanMacro(q.Get("gdpr")); gdpr != "" {
		if n, err := strconv.Atoi(gdpr); err == nil {
			ensureRegs().GDPR = &n
		}
	}
	if coppa := cleanMacro(q.Get("coppa")); coppa != "" {
		if n, err := strconv.Atoi(coppa); err == nil {
			ensureRegs().COPPA = n
		}
	}
	if usp := cleanMacro(q.Get("us_privacy")); usp != "" {
		ensureRegs().USPrivacy = usp
	}
	if gpp := cleanMacro(q.Get("gpp")); gpp != "" {
		ensureRegs().GPP = gpp
	}
	if gppSID := cleanMacro(q.Get("gpp_sid")); gppSID != "" {
		ids := make([]int, 0, 4)
		for _, p := range splitCSV(gppSID) {
			if n, err := strconv.Atoi(p); err == nil {
				ids = append(ids, n)
			}
		}
		if len(ids) > 0 {
			ensureRegs().GPPSID = ids
		}
	}

	// --- User consent (TCF v2 string + Google Additional Consent) ---
	ensureUser := func() *openrtb.User {
		if bidReq.User == nil {
			bidReq.User = &openrtb.User{}
		}
		return bidReq.User
	}
	if consent := cleanMacro(q.Get("gdpr_consent")); consent != "" {
		ensureUser().Consent = consent
	}
	if addtl := cleanMacro(q.Get("addtl_consent")); addtl != "" {
		u := ensureUser()
		u.Ext = setExtKey(u.Ext, []string{"ConsentedProvidersSettings", "consented_providers"}, addtl)
	}
}

// resolveContent returns the Content object attached to whichever inventory
// parent this request uses (Site or App), creating it if necessary.
func resolveContent(bidReq *openrtb.BidRequest) *openrtb.Content {
	if bidReq.App != nil {
		if bidReq.App.Content == nil {
			bidReq.App.Content = &openrtb.Content{}
		}
		return bidReq.App.Content
	}
	if bidReq.Site == nil {
		bidReq.Site = &openrtb.Site{}
	}
	if bidReq.Site.Content == nil {
		bidReq.Site.Content = &openrtb.Content{}
	}
	return bidReq.Site.Content
}

// cleanMacro normalises an IAB VAST 4 macro value. Per IAB spec, players
// emit "-1" for unsupported macros; treat that and empty strings as absent
// so we never overwrite real defaults with sentinel placeholders.
func cleanMacro(v string) string {
	if v == "" || v == "-1" {
		return ""
	}
	return v
}

// parsePlayerSize parses the IAB [PLAYERSIZE] macro output, format "WxH"
// (e.g. "1920x1080"). Returns (0, 0, false) on any parse failure.
func parsePlayerSize(v string) (int, int, bool) {
	v = cleanMacro(v)
	if v == "" {
		return 0, 0, false
	}
	parts := splitOn(v, 'x')
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(parts[0])
	hgt, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || hgt <= 0 {
		return 0, 0, false
	}
	return w, hgt, true
}

// parseLatLong parses the IAB [LATLONG] macro output, format "lat,lon".
// Returns (0, 0, false) on any parse failure or consent-gated empty value.
func parseLatLong(v string) (float64, float64, bool) {
	v = cleanMacro(v)
	if v == "" {
		return 0, 0, false
	}
	parts := splitCSV(v)
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(parts[0], 64)
	lon, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

// parseDeviceType accepts either a numeric OpenRTB device-type enum or a
// GAM-style string KV and returns the OpenRTB constant (0 = unknown).
func parseDeviceType(v string) int {
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	switch strings.ToLower(v) {
	case "mobile", "smartphone":
		return openrtb.DeviceTypeMobile
	case "pc", "desktop", "computer":
		return openrtb.DeviceTypePC
	case "ctv", "tv", "connectedtv", "connected_tv", "smarttv", "smart_tv":
		return openrtb.DeviceTypeCTV
	case "phone":
		return openrtb.DeviceTypePhone
	case "tablet":
		return openrtb.DeviceTypeTablet
	case "connected", "connecteddevice":
		return openrtb.DeviceTypeConnected
	case "stb", "settopbox", "set_top_box":
		return openrtb.DeviceTypeSetTopBox
	}
	return 0
}

// setExtKey merges a value into an OpenRTB ext blob at the given path,
// creating intermediate objects as needed. Returns the re-serialised JSON;
// on marshal failure returns the original bytes unchanged.
func setExtKey(raw json.RawMessage, path []string, value interface{}) json.RawMessage {
	if len(path) == 0 {
		return raw
	}
	m := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	cur := m
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[key] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return b
}

// splitCSV splits a comma-separated string and trims each element.
func splitCSV(v string) []string {
	return splitOn(v, ',')
}

// splitOn splits on a single separator rune and trims whitespace. Empty
// segments are dropped so "a,,b" returns ["a", "b"].
func splitOn(v string, sep rune) []string {
	out := make([]string, 0, 4)
	cur := make([]rune, 0, len(v))
	flush := func() {
		s := strings.TrimSpace(string(cur))
		if s != "" {
			out = append(out, s)
		}
		cur = cur[:0]
	}
	for _, r := range v {
		if r == sep {
			flush()
			continue
		}
		cur = append(cur, r)
	}
	flush()
	return out
}

// appendUnique appends v to list iff not already present (case-sensitive).
func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
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
					VASTAdTagURI:          vast.CDATAElement{Value: auctionURL},
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
