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
type sovrnParams struct {
	TagID    string      `json:"tagid"`
	TagId    string      `json:"tagId"` // Alternative spelling
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

		// Use bidfloor from ext if imp doesn't have one
		if imp.BidFloor == 0 {
			if bidFloor := getExtBidFloor(sovrnParams); bidFloor > 0 {
				imp.BidFloor = bidFloor
			}
		}

		// Validate video parameters if present
		if imp.Video != nil {
			if len(imp.Video.Mimes) == 0 || imp.Video.MaxDuration == 0 || len(imp.Video.Protocols) == 0 {
				errs = append(errs, fmt.Errorf("missing required video parameters for imp %s", imp.ID))
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
		addHeaderIfNonEmpty(headers, "X-Forwarded-For", requestCopy.Device.IP)
		addHeaderIfNonEmpty(headers, "Accept-Language", requestCopy.Device.Language)
		if requestCopy.Device.DNT != nil {
			headers.Set("DNT", strconv.Itoa(*requestCopy.Device.DNT))
		}
	}

	// Add ljt_reader cookie if we have a BuyerUID
	if requestCopy.User != nil {
		userID := strings.TrimSpace(requestCopy.User.BuyerUID)
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

			// URL-unescape the AdM (creative markup)
			if adm, err := url.QueryUnescape(bid.AdM); err == nil {
				bid.AdM = adm
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
func getTagID(params *sovrnParams) string {
	if params.TagID != "" {
		return params.TagID
	}
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
