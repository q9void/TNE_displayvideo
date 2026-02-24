package endpoints

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thenexusengine/tne_springwire/pkg/vast"
)

// PodHandler handles VMAP ad pod requests for CTV and instream video.
type PodHandler struct {
	trackingBaseURL string
}

// NewPodHandler creates a new pod handler.
func NewPodHandler(trackingBaseURL string) *PodHandler {
	return &PodHandler{trackingBaseURL: trackingBaseURL}
}

// HandleVMAP handles GET /video/pod requests.
//
// Returns a VMAP 1.0 document with one AdBreak per requested break position.
// Each break's AdTagURI points to /video/vast so auctions are resolved
// just-in-time when the SSAI system approaches each cue point.
//
// Query parameters:
//
//	breaks      Comma-separated break positions: "start", "end", "HH:MM:SS", "n%"
//	            Default: "start" (pre-roll only)
//	maxads      Max ads per break; >1 sets allowMultipleAds on the AdSource. Default: 1
//	pub         Publisher ID (forwarded to /video/vast as site.publisher.id / app.publisher.id)
//
//	Video params forwarded to each break's /video/vast call:
//	  w, h, mindur, maxdur, skip, skipafter, protocols, mimes,
//	  minbitrate, maxbitrate, bidfloor, placement
//
//	Site params (desktop instream):
//	  site_id, domain, page
//
//	App params (CTV):
//	  app_bundle   App bundle ID (e.g. "com.beinsports.tv")
//	  app_name     Human-readable app name
//
//	Content params (contextual targeting):
//	  content_title, content_series, content_season,
//	  content_episode, content_genre, livestream
func (h *PodHandler) HandleVMAP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	requestID := q.Get("id")
	if requestID == "" {
		requestID = generateRequestID()
	}

	// Parse break positions (default: pre-roll only)
	breaksParam := q.Get("breaks")
	if breaksParam == "" {
		breaksParam = "start"
	}
	breakPositions := parseBreakPositions(breaksParam)

	maxAds := parseInt(q.Get("maxads"), 1)
	allowMultiple := maxAds > 1

	// Build VMAP — one AdBreak per position, each pointing to /video/vast
	vmapDoc := vast.NewVMAP()
	trackingBase := h.trackingBaseURL + "/video/event"

	for i, position := range breakPositions {
		breakID := breakIDFromPosition(position, i)
		adTagURI := h.buildBreakAdTagURI(q, requestID, breakID)
		vmapDoc.AddAdTagBreak(position, breakID, adTagURI, allowMultiple, trackingBase)
	}

	data, err := vmapDoc.Marshal()
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("Failed to marshal VMAP")
		http.Error(w, "Failed to build VMAP response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// CORS wildcard intentional — same rationale as VAST endpoints (see setVASTCORSHeaders)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	log.Info().
		Str("request_id", requestID).
		Int("breaks", len(breakPositions)).
		Bool("allow_multiple_ads", allowMultiple).
		Msg("VMAP response sent")
}

// buildBreakAdTagURI constructs the /video/vast URL for a specific break,
// forwarding all relevant video, site, app, and content targeting params.
func (h *PodHandler) buildBreakAdTagURI(q url.Values, requestID, breakID string) string {
	params := url.Values{}

	for _, key := range []string{
		// Video
		"w", "h", "mindur", "maxdur", "skip", "skipafter",
		"protocols", "mimes", "minbitrate", "maxbitrate",
		"bidfloor", "placement",
		// Publisher
		"pub",
		// Site (desktop instream)
		"site_id", "domain", "page",
		// App (CTV)
		"app_bundle", "app_name",
		// Content (contextual)
		"content_title", "content_series", "content_season",
		"content_episode", "content_genre", "livestream",
	} {
		if v := q.Get(key); v != "" {
			params.Set(key, v)
		}
	}

	// Each break gets a unique request ID so auction logs are traceable per break
	params.Set("id", requestID+"-"+breakID)

	return h.trackingBaseURL + "/video/vast?" + params.Encode()
}

// parseBreakPositions splits a comma-separated break string and trims whitespace.
func parseBreakPositions(s string) []string {
	var positions []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			positions = append(positions, p)
		}
	}
	if len(positions) == 0 {
		return []string{"start"}
	}
	return positions
}

// breakIDFromPosition maps a time offset to a human-readable break ID.
// Midroll breaks are numbered by their index in the overall break list.
func breakIDFromPosition(position string, index int) string {
	switch position {
	case "start":
		return "preroll"
	case "end":
		return "postroll"
	default:
		return fmt.Sprintf("midroll-%d", index)
	}
}
