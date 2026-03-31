// Package triplelift implements the TripleLift bidder adapter
package triplelift

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const defaultEndpoint = "https://tlx.3lift.com/s2s/auction"

var defaultLoader *routing.Loader

func SetLoader(l *routing.Loader) { defaultLoader = l }

func extractSlotParams(imps []openrtb.Imp) map[string]interface{} {
	if len(imps) == 0 || imps[0].Ext == nil {
		return nil
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(imps[0].Ext, &outer); err != nil {
		return nil
	}
	raw, ok := outer["triplelift"]
	if !ok {
		raw, ok = outer["bidder"]
		if !ok {
			return nil
		}
	}
	var params map[string]interface{}
	json.Unmarshal(raw, &params) //nolint:errcheck
	return params
}

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

	if defaultLoader != nil {
		rules := defaultLoader.Get(context.Background(), "triplelift")
		composer := routing.NewComposer(rules)
		composed, _ := composer.Apply("triplelift", &requestCopy, extractSlotParams(requestCopy.Imp), nil, nil)
		requestCopy = *composed
	}

	// Process each impression — mirrors PBS TripLift adapter (processImp logic)
	for _, imp := range requestCopy.Imp {
		if imp.Ext == nil {
			errs = append(errs, fmt.Errorf("imp %s missing required imp.ext", imp.ID))
			continue
		}

		// Prefer imp.ext.bidder (standard PBS format injected by bid handler),
		// fall back to imp.ext.triplelift for legacy compatibility
		var impExt struct {
			Triplelift json.RawMessage `json:"triplelift"`
			Bidder     json.RawMessage `json:"bidder"`
		}
		if err := json.Unmarshal(imp.Ext, &impExt); err != nil {
			errs = append(errs, fmt.Errorf("failed to parse imp.ext for imp %s: %w", imp.ID, err))
			continue
		}
		bidderParams := impExt.Bidder
		if len(bidderParams) == 0 {
			bidderParams = impExt.Triplelift
		}
		if len(bidderParams) == 0 {
			errs = append(errs, fmt.Errorf("imp %s missing triplelift/bidder extension", imp.ID))
			continue
		}

		var tlExt struct {
			InventoryCode string  `json:"inventoryCode"`
			Floor         float64 `json:"floor,omitempty"`
		}
		if err := json.Unmarshal(bidderParams, &tlExt); err != nil {
			errs = append(errs, fmt.Errorf("failed to parse triplelift params for imp %s: %w", imp.ID, err))
			continue
		}
		if tlExt.InventoryCode == "" {
			errs = append(errs, fmt.Errorf("imp %s missing required inventoryCode", imp.ID))
			continue
		}

		// TripLift requires Banner or Video — matches PBS adapter behaviour
		if imp.Banner == nil && imp.Video == nil {
			errs = append(errs, fmt.Errorf("imp %s: neither Banner nor Video object specified", imp.ID))
			continue
		}

		impCopy := imp
		impCopy.TagID = tlExt.InventoryCode

		// Rewrite imp.ext to imp.ext.bidder.inventoryCode (what TripLift's endpoint expects)
		rewritten, err := json.Marshal(map[string]interface{}{
			"bidder": map[string]string{"inventoryCode": tlExt.InventoryCode},
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to rewrite imp.ext for imp %s: %w", imp.ID, err))
			continue
		}
		impCopy.Ext = rewritten

		if tlExt.Floor > 0 {
			impCopy.BidFloor = tlExt.Floor
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
		// TripleLift returns JavaScript/JSONP for "no bid" - treat as valid no-bid response
		if strings.Contains(lowerCT, "application/javascript") {
			// JavaScript response like: serveDefault("not_loaded","...") means no bid
			return nil, nil
		}
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
