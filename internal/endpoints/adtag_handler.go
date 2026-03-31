package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/bidcache"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// AdTagHandler handles direct ad tag requests
type AdTagHandler struct {
	exchange *exchange.Exchange
	bidCache *bidcache.BidCache
}

// NewAdTagHandler creates a new ad tag handler
func NewAdTagHandler(ex *exchange.Exchange, bc *bidcache.BidCache) *AdTagHandler {
	return &AdTagHandler{
		exchange: ex,
		bidCache: bc,
	}
}

// HandleJavaScriptAd handles JavaScript ad requests
func (h *AdTagHandler) HandleJavaScriptAd(w http.ResponseWriter, r *http.Request) {
	log := logger.Log

	// Parse parameters
	params := parseAdParams(r)
	if params == nil {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	// Build OpenRTB request
	bidRequest := h.buildBidRequest(r, params)

	// Run auction
	ctx, cancel := context.WithTimeout(r.Context(), 1000*time.Millisecond)
	defer cancel()

	auctionReq := &exchange.AuctionRequest{
		BidRequest: bidRequest,
		Timeout:    1000 * time.Millisecond,
	}

	auctionResp, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		log.Error().Err(err).Msg("Auction failed")
		h.writeNoAdResponse(w, params.DivID)
		return
	}

	// Extract winning bid
	winningBid := h.extractWinningBid(auctionResp)
	if winningBid == nil {
		h.writeNoAdResponse(w, params.DivID)
		return
	}

	// Render JavaScript response
	h.writeJavaScriptResponse(w, params, winningBid)
}

// HandleIframeAd handles iframe ad requests
func (h *AdTagHandler) HandleIframeAd(w http.ResponseWriter, r *http.Request) {
	log := logger.Log

	// Parse parameters
	params := parseAdParams(r)
	if params == nil {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	// Build OpenRTB request
	bidRequest := h.buildBidRequest(r, params)

	// Run auction
	ctx, cancel := context.WithTimeout(r.Context(), 1000*time.Millisecond)
	defer cancel()

	auctionReq := &exchange.AuctionRequest{
		BidRequest: bidRequest,
		Timeout:    1000 * time.Millisecond,
	}

	auctionResp, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		log.Error().Err(err).Msg("Auction failed")
		h.writeNoAdHTML(w, params.Width, params.Height)
		return
	}

	// Extract winning bid
	winningBid := h.extractWinningBid(auctionResp)
	if winningBid == nil {
		h.writeNoAdHTML(w, params.Width, params.Height)
		return
	}

	// Render HTML response
	h.writeHTMLResponse(w, params, winningBid)
}

// HandleGAMAd serves the winning ad markup to the GAM 3rd-party creative.
// Called by the creative snippet with ?bid=<hb_adid_catalyst>&creative=<hb_creative_catalyst>&w=...&h=...&pb=...
// The bid ID is used to look up the markup cached at auction time — no new auction is run.
func (h *AdTagHandler) HandleGAMAd(w http.ResponseWriter, r *http.Request) {
	log := logger.Log
	q := r.URL.Query()
	bidID := q.Get("bid")

	if bidID == "" {
		log.Warn().Msg("/ad/gam called without bid parameter")
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("/* no bid id */"))
		return
	}

	adMarkup, ok := h.bidCache.Get(bidID)
	if !ok {
		log.Warn().Str("bid_id", bidID).Msg("/ad/gam bid not found in cache (expired or unknown)")
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("/* bid expired */"))
		return
	}

	log.Info().
		Str("bid_id", bidID).
		Str("creative_id", q.Get("creative")).
		Str("pb", q.Get("pb")).
		Msg("/ad/gam serving cached markup")

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Write the ad markup directly into the GAM creative iframe via document.write.
	// The markup is trusted SSP HTML/JS — do not sanitize it.
	adMarkupJSON, _ := json.Marshal(adMarkup)
	// document.open() is required before document.write() when called from an
	// async script (which is how the GAM creative loads us via script.async=true)
	script := fmt.Sprintf("document.open();document.write(%s);document.close();", string(adMarkupJSON))
	w.Write([]byte(script))
}

// AdParams represents parsed ad request parameters
type AdParams struct {
	PublisherID string
	PlacementID string
	DivID       string
	Width       int
	Height      int
	PageURL     string
	Domain      string
	Keywords    []string
	CustomData  map[string]string
}

// parseAdParams parses ad parameters from request
func parseAdParams(r *http.Request) *AdParams {
	query := r.URL.Query()

	width, _ := strconv.Atoi(query.Get("w"))
	height, _ := strconv.Atoi(query.Get("h"))

	if width <= 0 || height <= 0 {
		return nil
	}

	params := &AdParams{
		PublisherID: query.Get("pub"),
		PlacementID: query.Get("placement"),
		DivID:       query.Get("div"),
		Width:       width,
		Height:      height,
		PageURL:     query.Get("url"),
		Domain:      query.Get("domain"),
		CustomData:  make(map[string]string),
	}

	// Parse keywords
	if kw := query.Get("kw"); kw != "" {
		params.Keywords = strings.Split(kw, ",")
	}

	// Parse custom data (any other parameters)
	for key, values := range query {
		if len(values) > 0 && !isReservedParam(key) {
			params.CustomData[key] = values[0]
		}
	}

	return params
}

// isReservedParam checks if parameter name is reserved
func isReservedParam(name string) bool {
	reserved := []string{"pub", "placement", "div", "w", "h", "url", "domain", "kw"}
	for _, r := range reserved {
		if name == r {
			return true
		}
	}
	return false
}

// buildBidRequest builds OpenRTB bid request from ad parameters
func (h *AdTagHandler) buildBidRequest(r *http.Request, params *AdParams) *openrtb.BidRequest {
	// Generate request ID
	requestID := fmt.Sprintf("adtag-%d", time.Now().UnixNano())

	// Determine domain
	domain := params.Domain
	if domain == "" && params.PageURL != "" {
		// Extract domain from URL
		domain = extractDomain(params.PageURL)
	}

	// Build impression
	imp := openrtb.Imp{
		ID: "1",
		Banner: &openrtb.Banner{
			W: params.Width,
			H: params.Height,
			Format: []openrtb.Format{
				{W: params.Width, H: params.Height},
			},
		},
		TagID: params.PlacementID,
	}

	// Build site
	site := &openrtb.Site{
		ID:     params.PublisherID,
		Domain: domain,
	}

	if params.PageURL != "" {
		site.Page = params.PageURL
	}

	// Build device from request
	device := &openrtb.Device{
		UA: r.Header.Get("User-Agent"),
		IP: getClientIP(r),
	}

	// Build request
	bidRequest := &openrtb.BidRequest{
		ID:   requestID,
		Imp:  []openrtb.Imp{imp},
		Site: site,
		Device: device,
		Cur:  []string{"USD"},
		TMax: 1000,
	}

	return bidRequest
}

// extractWinningBid extracts the winning bid from auction response
func (h *AdTagHandler) extractWinningBid(resp *exchange.AuctionResponse) *openrtb.Bid {
	if resp.BidResponse == nil || len(resp.BidResponse.SeatBid) == 0 {
		return nil
	}

	// Get first bid from first seat
	for _, seatBid := range resp.BidResponse.SeatBid {
		if len(seatBid.Bid) > 0 {
			return &seatBid.Bid[0]
		}
	}

	return nil
}

// writeJavaScriptResponse writes JavaScript ad response
func (h *AdTagHandler) writeJavaScriptResponse(w http.ResponseWriter, params *AdParams, bid *openrtb.Bid) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Render ad creative
	creative := sanitizeHTML(bid.AdM)

	// Sanitize all user-controlled parameters to prevent XSS
	safeDivID := sanitizeForJS(params.DivID)
	safeBidID := sanitizeForJS(bid.ID)
	safePlacementID := sanitizeForJS(params.PlacementID)

	script := fmt.Sprintf(`(function() {
  var container = document.getElementById('%s');
  if (container) {
    container.innerHTML = %s;
    // Fire impression tracking
    if (typeof tne !== 'undefined' && tne.trackImpression) {
      tne.trackImpression('%s', '%s');
    }
  }
})();`, safeDivID, toJSONString(creative), safeBidID, safePlacementID)

	w.Write([]byte(script))
}

// writeHTMLResponse writes HTML ad response for iframe
func (h *AdTagHandler) writeHTMLResponse(w http.ResponseWriter, params *AdParams, bid *openrtb.Bid) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	// Allow iframe embedding for ad serving
	w.Header().Del("X-Frame-Options")

	creative := sanitizeHTML(bid.AdM)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { margin: 0; padding: 0; overflow: hidden; }
</style>
</head>
<body>
%s
<script>
// Fire impression tracking
var img = new Image();
img.src = '/ad/track?bid=%s&placement=%s&event=impression';
</script>
</body>
</html>`, creative, bid.ID, params.PlacementID)

	w.Write([]byte(html))
}

// writeGAMResponse writes GAM-compatible JavaScript response
func (h *AdTagHandler) writeGAMResponse(w http.ResponseWriter, params *AdParams, bid *openrtb.Bid) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	creative := sanitizeHTML(bid.AdM)

	script := fmt.Sprintf(`(function() {
  var container = document.getElementById('%s');
  if (!container) {
    console.warn('TNE: Container not found:', '%s');
    return;
  }

  // Render ad
  container.innerHTML = %s;

  // Fire impression tracking
  var trackingPixel = new Image();
  trackingPixel.src = '/ad/track?bid=%s&placement=%s&event=impression&ts=' + Date.now();

  // Setup click tracking
  var links = container.querySelectorAll('a');
  links.forEach(function(link) {
    link.addEventListener('click', function() {
      var clickPixel = new Image();
      clickPixel.src = '/ad/track?bid=%s&placement=%s&event=click&ts=' + Date.now();
    });
  });

  console.log('TNE: Ad rendered successfully');
})();`, params.DivID, params.DivID, toJSONString(creative), bid.ID, params.PlacementID, bid.ID, params.PlacementID)

	w.Write([]byte(script))
}

// writeNoAdResponse writes no-ad JavaScript response
func (h *AdTagHandler) writeNoAdResponse(w http.ResponseWriter, divID string) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Sanitize divID to prevent XSS
	safeDivID := sanitizeForJS(divID)

	script := fmt.Sprintf(`(function() {
  var container = document.getElementById('%s');
  if (container) {
    container.style.display = 'none';
  }
})();`, safeDivID)

	w.Write([]byte(script))
}

// writeNoAdHTML writes no-ad HTML response
func (h *AdTagHandler) writeNoAdHTML(w http.ResponseWriter, width, height int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	// Allow iframe embedding for ad serving
	w.Header().Del("X-Frame-Options")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body { margin: 0; padding: 0; width: %dpx; height: %dpx; }
</style>
</head>
<body></body>
</html>`, width, height)

	w.Write([]byte(html))
}

// HandleAdTracking handles ad tracking pixel requests
func (h *AdTagHandler) HandleAdTracking(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	bidID := query.Get("bid")
	placementID := query.Get("placement")
	event := query.Get("event")

	// Log tracking event
	logger.Log.Info().
		Str("bid_id", bidID).
		Str("placement_id", placementID).
		Str("event", event).
		Str("ip", getClientIP(r)).
		Str("user_agent", r.Header.Get("User-Agent")).
		Msg("Ad tracking event")

	// Return 1x1 transparent GIF
	gif := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
		0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x21,
		0xF9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2C, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
		0x01, 0x00, 0x3B,
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Length", strconv.Itoa(len(gif)))
	w.Write(gif)
}

// sanitizeHTML sanitizes HTML to prevent XSS
func sanitizeHTML(htmlStr string) string {
	// Basic sanitization - in production, use a proper HTML sanitizer
	htmlStr = strings.ReplaceAll(htmlStr, "<script>", "&lt;script&gt;")
	htmlStr = strings.ReplaceAll(htmlStr, "</script>", "&lt;/script&gt;")
	return htmlStr
}

// sanitizeForJS sanitizes a string for safe embedding in JavaScript context
// Prevents XSS attacks via user-controlled parameters in ad tags
func sanitizeForJS(s string) string {
	// HTML escape first
	s = html.EscapeString(s)
	// Escape quotes and special chars for JavaScript string context
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}

// toJSONString converts string to JSON-safe string literal
func toJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// extractDomain extracts domain from URL
func extractDomain(urlStr string) string {
	// Simple domain extraction
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")
	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		domain := strings.Split(parts[0], ":")[0]
		// Strip subdomain (e.g., "dev.totalprosports.com" -> "totalprosports.com")
		return stripSubdomain(domain)
	}
	return ""
}

// stripSubdomain removes subdomains, keeping only the root domain
// Examples: "dev.totalprosports.com" -> "totalprosports.com"
//           "www.example.com" -> "example.com"
//           "totalprosports.com" -> "totalprosports.com"
func stripSubdomain(domain string) string {
	parts := strings.Split(domain, ".")

	// If it's already a root domain (e.g., "com" or "example.com"), return as-is
	if len(parts) <= 2 {
		return domain
	}

	// Return the last two parts (root domain + TLD)
	// e.g., ["dev", "totalprosports", "com"] -> "totalprosports.com"
	return strings.Join(parts[len(parts)-2:], ".")
}
