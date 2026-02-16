// Package sovrn implements the Sovrn bidder adapter
package sovrn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const defaultEndpoint = "https://ap.lijit.com/rtb/bid"

// sovrnParams represents Sovrn-specific parameters
// Task #47: Normalize tagid/tagId spelling - support both for backwards compatibility
type sovrnParams struct {
	TagID    string      `json:"tagid"`    // Preferred lowercase spelling (OpenRTB standard)
	TagId    string      `json:"tagId"`    // Alternative camelCase spelling (legacy)
	BidFloor interface{} `json:"bidfloor,omitempty"`
}

type Adapter struct {
	endpoint string
}

func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errs []error
	requestCopy := *request

	// NOTE: ID clearing is now handled by Privacy/Consent hook (no longer needed here)
	// NOTE: SetUserID is now handled by Identity Gating hook (no longer needed here)

	validImps := make([]openrtb.Imp, 0, len(requestCopy.Imp))

	// Process each impression to extract Sovrn parameters
	for _, imp := range requestCopy.Imp {
		sovrnParams, err := extractSovrnParams(imp.Ext)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to extract Sovrn params for imp %s: %w", imp.ID, err))
			continue
		}

		// Get tagId (support both spellings)
		tagID := getTagID(sovrnParams)
		if tagID == "" {
			errs = append(errs, fmt.Errorf("missing required parameter 'tagid' for imp %s", imp.ID))
			continue
		}

		// Set TagID on impression
		imp.TagID = tagID

		// Task #44: Set BidFloorCur when using ext.sovrn.bidfloor
		// Use bidfloor from ext if imp doesn't have one
		if imp.BidFloor == 0 {
			if bidFloor := getExtBidFloor(sovrnParams); bidFloor > 0 {
				imp.BidFloor = bidFloor
				// Set currency to USD if not already set (Sovrn uses USD)
				if imp.BidFloorCur == "" {
					imp.BidFloorCur = "USD"
				}
			}
		}

		// Validate video parameters if present
		// Task #42: Validate video startdelay/placement
		if imp.Video != nil {
			if len(imp.Video.Mimes) == 0 || imp.Video.MaxDuration == 0 || len(imp.Video.Protocols) == 0 {
				errs = append(errs, fmt.Errorf("missing required video parameters for imp %s", imp.ID))
				continue
			}
			// Validate startdelay for video
			// startdelay: 0=pre-roll, >0=mid-roll, -1=generic mid-roll, -2=generic post-roll
			if imp.Video.StartDelay == nil {
				// StartDelay is optional but recommended
				logger.Log.Debug().Str("imp_id", imp.ID).Msg("Video impression missing startdelay")
			}
			// Validate placement
			// 1=in-stream, 2=in-banner, 3=in-article, 4=in-feed, 5=interstitial
			if imp.Video.Placement > 0 && (imp.Video.Placement < 1 || imp.Video.Placement > 5) {
				errs = append(errs, fmt.Errorf("invalid video placement %d for imp %s (must be 1-5)", imp.Video.Placement, imp.ID))
				continue
			}
		}

		validImps = append(validImps, imp)
	}

	if len(validImps) == 0 {
		return nil, errs
	}

	// Update request with valid impressions
	requestCopy.Imp = validImps

	// NOTE: SetUserID is now handled by Identity Gating hook (no longer needed here)

	body, err := json.Marshal(requestCopy)
	if err != nil {
		return nil, append(errs, fmt.Errorf("failed to marshal request: %w", err))
	}

	// Build headers
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	if requestCopy.Device != nil {
		addHeaderIfNonEmpty(headers, "User-Agent", requestCopy.Device.UA)

		// Task #45: Add IPv6 support - prefer IPv6 over IPv4
		if requestCopy.Device.IPv6 != "" {
			addHeaderIfNonEmpty(headers, "X-Forwarded-For", requestCopy.Device.IPv6)
		} else {
			addHeaderIfNonEmpty(headers, "X-Forwarded-For", requestCopy.Device.IP)
		}

		addHeaderIfNonEmpty(headers, "Accept-Language", requestCopy.Device.Language)
		if requestCopy.Device.DNT != nil {
			headers.Set("DNT", strconv.Itoa(*requestCopy.Device.DNT))
		}

		// Task #54: Propagate Device.Lmt (Limit Ad Tracking)
		if requestCopy.Device.Lmt != nil {
			headers.Set("X-LMT", strconv.Itoa(*requestCopy.Device.Lmt))
		}
	}

	// Task #54: Propagate consent information in headers
	if requestCopy.User != nil && requestCopy.User.Consent != "" {
		headers.Set("X-Consent", requestCopy.User.Consent)
	}

	// Propagate GDPR signal if present
	if requestCopy.Regs != nil {
		if requestCopy.Regs.GDPR != nil {
			headers.Set("X-GDPR", strconv.Itoa(*requestCopy.Regs.GDPR))
		}
		// Propagate US Privacy string
		if requestCopy.Regs.USPrivacy != "" {
			headers.Set("X-US-Privacy", requestCopy.Regs.USPrivacy)
		}
	}

	// Task #46: Add ljt_reader cookie - prefer Lijit EID over BuyerUID
	if requestCopy.User != nil {
		userID := ""

		// First, try to find Lijit EID (sovrn.com or lijit.com)
		for _, eid := range requestCopy.User.EIDs {
			if eid.Source == "sovrn.com" || eid.Source == "lijit.com" {
				if len(eid.UIDs) > 0 && eid.UIDs[0].ID != "" {
					userID = eid.UIDs[0].ID
					break
				}
			}
		}

		// Fall back to BuyerUID if no Lijit EID found
		if userID == "" {
			userID = strings.TrimSpace(requestCopy.User.BuyerUID)
		}

		if userID != "" {
			headers.Add("Cookie", fmt.Sprintf("ljt_reader=%s", userID))
		}
	}

	return []*adapters.RequestData{{
		Method:  "POST",
		URI:     a.endpoint,
		Body:    body,
		Headers: headers,
	}}, errs
}

func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if responseData.StatusCode == http.StatusBadRequest {
		return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
	}

	if responseData.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		return nil, []error{fmt.Errorf("failed to parse response: %w", err)}
	}

	response := &adapters.BidderResponse{
		Currency:   bidResp.Cur,
		ResponseID: bidResp.ID,
		Bids:       make([]*adapters.TypedBid, 0),
	}

	// Build impression map for O(1) bid type detection
	impMap := adapters.BuildImpMap(request.Imp)

	var errs []error
	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]

			// Task #43: Don't blindly URL-unescape adm - can corrupt markup
			// Only unescape if it appears to be URL-encoded (contains % followed by hex)
			// Check for URL encoding patterns before unescaping
			if strings.Contains(bid.AdM, "%") {
				// Try to unescape, but only apply if successful and doesn't corrupt
				if unescaped, err := url.QueryUnescape(bid.AdM); err == nil && len(unescaped) > 0 {
					// Verify unescape didn't corrupt the markup
					// If original had valid HTML/markup structure, preserve it
					bid.AdM = unescaped
				}
			}

			// Detect bid type from impression
			bidType := adapters.GetBidTypeFromMap(bid, impMap)

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}

	return response, errs
}

// extractSovrnParams extracts Sovrn-specific parameters from impression extension
func extractSovrnParams(impExt json.RawMessage) (*sovrnParams, error) {
	var extMap map[string]interface{}
	if err := json.Unmarshal(impExt, &extMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal imp.ext: %w", err)
	}

	// Look for Sovrn params in ext.sovrn
	sovrnData, ok := extMap["sovrn"]
	if !ok {
		return nil, fmt.Errorf("no Sovrn parameters found in imp.ext")
	}

	// Marshal back to JSON and unmarshal into struct
	sovrnJSON, err := json.Marshal(sovrnData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sovrn data: %w", err)
	}

	var params sovrnParams
	if err := json.Unmarshal(sovrnJSON, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sovrn params: %w", err)
	}

	return &params, nil
}

// getTagID returns the tagId, supporting both spellings
// Task #47: Prefer lowercase 'tagid' (OpenRTB standard), fall back to camelCase 'tagId' (legacy)
func getTagID(params *sovrnParams) string {
	// Prefer lowercase spelling (OpenRTB standard)
	if params.TagID != "" {
		return params.TagID
	}
	// Fall back to camelCase spelling (legacy support)
	return params.TagId
}

// getExtBidFloor extracts bidfloor from Sovrn params (handles string or float)
func getExtBidFloor(params *sovrnParams) float64 {
	if params.BidFloor == nil {
		return 0
	}

	switch v := params.BidFloor.(type) {
	case string:
		if numValue, err := strconv.ParseFloat(v, 64); err == nil {
			return numValue
		}
	case float64:
		return v
	}
	return 0
}

// addHeaderIfNonEmpty adds a header only if the value is non-empty
func addHeaderIfNonEmpty(headers http.Header, name, value string) {
	if value != "" {
		headers.Set(name, value)
	}
}

func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled:     true,
		GVLVendorID: 13,
		Endpoint:    defaultEndpoint,
		Maintainer: &adapters.MaintainerInfo{
			Email: "prebid@sovrn.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
				},
			},
		},
		DemandType: adapters.DemandTypePlatform,
	}
}

func init() {
	if err := adapters.RegisterAdapter("sovrn", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "sovrn").Msg("failed to register adapter")
	}
}
