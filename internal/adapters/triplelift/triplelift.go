// Package triplelift implements the TripleLift bidder adapter
package triplelift

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const defaultEndpoint = "https://tlx.3lift.com/s2s/auction"

// Adapter implements the TripleLift bidder
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
	requestCopy := *request
	var errs []error
	var validImps []openrtb.Imp

	// NOTE: ID clearing is now handled by Privacy/Consent hook (no longer needed here)

	// Process each impression - extract TripleLift parameters
	for _, imp := range requestCopy.Imp {
		var tripleliftExt struct {
			InventoryCode string  `json:"inventoryCode"`
			Floor         float64 `json:"floor,omitempty"`
		}

		// Task #38: Check for multiformat preference
		// Count media types to detect multiformat impressions
		mediaTypeCount := 0
		if imp.Banner != nil {
			mediaTypeCount++
		}
		if imp.Native != nil {
			mediaTypeCount++
		}
		// TripleLift only supports banner and native

		// Extract TripleLift params from imp.ext.triplelift
		if imp.Ext != nil {
			var impExt struct {
				Triplelift json.RawMessage `json:"triplelift"`
			}
			if err := json.Unmarshal(imp.Ext, &impExt); err != nil {
				errs = append(errs, fmt.Errorf("failed to parse imp.ext for imp %s: %w", imp.ID, err))
				continue
			}
			if len(impExt.Triplelift) > 0 {
				if err := json.Unmarshal(impExt.Triplelift, &tripleliftExt); err != nil {
					errs = append(errs, fmt.Errorf("failed to parse triplelift params for imp %s: %w", imp.ID, err))
					continue
				}
			}
		}

		// Validate required parameter
		if tripleliftExt.InventoryCode == "" {
			errs = append(errs, fmt.Errorf("imp %s missing required inventoryCode", imp.ID))
			continue
		}

		// Validate impression has Banner or Native
		if imp.Banner == nil && imp.Native == nil {
			errs = append(errs, fmt.Errorf("imp %s must have Banner or Native", imp.ID))
			continue
		}

		// Task #41: Validate native.request for native impressions
		if imp.Native != nil && imp.Native.Request == "" {
			errs = append(errs, fmt.Errorf("imp %s has Native but missing native.request", imp.ID))
			continue
		}

		// Create impression copy and set TripleLift-specific fields
		impCopy := imp
		impCopy.TagID = tripleliftExt.InventoryCode

		// Set bid floor if provided
		// Task #40: Don't force BidFloorCur = USD - only set if not already set
		if tripleliftExt.Floor > 0 {
			impCopy.BidFloor = tripleliftExt.Floor
			// Only set currency if floor is provided but currency is missing
			// Don't overwrite existing currency as it breaks non-USD floors
		}

		validImps = append(validImps, impCopy)
	}

	// Return errors if no valid impressions
	if len(validImps) == 0 {
		if len(errs) > 0 {
			return nil, errs
		}
		return nil, []error{fmt.Errorf("no valid impressions")}
	}

	requestCopy.Imp = validImps

	// NOTE: SetUserID is now handled by Identity Gating hook (no longer needed here)

	requestBody, err := json.Marshal(requestCopy)
	if err != nil {
		return nil, append(errs, fmt.Errorf("failed to marshal request: %w", err))
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json;charset=utf-8")
	headers.Set("Accept", "application/json")

	return []*adapters.RequestData{
		{Method: "POST", URI: a.endpoint, Body: requestBody, Headers: headers},
	}, errs
}

func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if responseData.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
	}

	// Task #39: Accept application/json variants (charset, text/json)
	contentType := responseData.Headers.Get("Content-Type")
	if contentType != "" {
		lowerCT := strings.ToLower(contentType)
		// Accept: application/json, application/json;charset=utf-8, text/json
		if !strings.Contains(lowerCT, "application/json") && !strings.Contains(lowerCT, "text/json") {
			bodyPreview := string(responseData.Body)
			if len(bodyPreview) > 200 {
				bodyPreview = bodyPreview[:200] + "..."
			}
			return nil, []error{fmt.Errorf("invalid Content-Type: %s (expected application/json or text/json). Body preview: %s", contentType, bodyPreview)}
		}
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		// Enhanced error with response body preview for debugging
		bodyPreview := string(responseData.Body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, []error{fmt.Errorf("failed to parse JSON response: %w. Content-Type: %s, Body preview: %s", err, contentType, bodyPreview)}
	}

	response := &adapters.BidderResponse{Currency: bidResp.Cur, ResponseID: bidResp.ID, Bids: make([]*adapters.TypedBid, 0)}

	// Build impression map for O(1) bid type detection
	impMap := adapters.BuildImpMap(request.Imp)

	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]
			// Detect bid type from impression instead of hardcoding
			bidType := adapters.GetBidTypeFromMap(bid, impMap)

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}
	return response, nil
}

func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled:     true,
		GVLVendorID: 28,
		Endpoint:    defaultEndpoint,
		Maintainer:  &adapters.MaintainerInfo{Email: "prebid@triplelift.com"},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeNative}},
			App:  &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeNative}},
		},
	}
}

func init() {
	if err := adapters.RegisterAdapter("triplelift", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "triplelift").Msg("failed to register adapter")
	}
}
