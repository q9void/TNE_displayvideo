// Package appnexus implements the AppNexus/Xandr bidder adapter
package appnexus

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	defaultEndpoint = "https://ib.adnxs.com/openrtb2/prebid"
)

// Adapter implements the AppNexus bidder
type Adapter struct {
	endpoint string
}

// New creates a new AppNexus adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

// MakeRequests builds HTTP requests for AppNexus
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errors []error

	// Create a copy to avoid modifying original
	requestCopy := *request

	// Remove Catalyst internal IDs from Site (prevent ID leakage)
	if requestCopy.Site != nil {
		siteCopy := *requestCopy.Site
		siteCopy.ID = ""
		if siteCopy.Publisher != nil {
			pubCopy := *siteCopy.Publisher
			pubCopy.ID = ""
			siteCopy.Publisher = &pubCopy
		}
		requestCopy.Site = &siteCopy
	}

	// Remove Catalyst internal IDs from App (if present)
	if requestCopy.App != nil {
		appCopy := *requestCopy.App
		appCopy.ID = ""
		if appCopy.Publisher != nil {
			pubCopy := *appCopy.Publisher
			pubCopy.ID = ""
			appCopy.Publisher = &pubCopy
		}
		requestCopy.App = &appCopy
	}

	// Add AppNexus-specific extensions
	requestBody, err := json.Marshal(requestCopy)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json;charset=utf-8")
	headers.Set("Accept", "application/json")

	return []*adapters.RequestData{
		{
			Method:  "POST",
			URI:     a.endpoint,
			Body:    requestBody,
			Headers: headers,
		},
	}, errors
}

// MakeBids parses AppNexus responses into bids
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
		ResponseID: bidResp.ID, // P1-1: Include ResponseID for validation
		Bids:       make([]*adapters.TypedBid, 0),
	}

	// P2-3: Build impression map once for O(1) lookups instead of O(n) per bid
	impMap := adapters.BuildImpMap(request.Imp)

	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]
			bidType := adapters.GetBidTypeFromMap(bid, impMap)

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}

	return response, nil
}

// Info returns bidder information
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled: true,
		Maintainer: &adapters.MaintainerInfo{
			Email: "prebid@xandr.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
					adapters.BidTypeNative,
				},
			},
			App: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
					adapters.BidTypeNative,
				},
			},
		},
		GVLVendorID: 32,
		Endpoint:    defaultEndpoint,
		DemandType:  adapters.DemandTypePlatform, // Platform demand (obfuscated as "thenexusengine")
	}
}

func init() {
	if err := adapters.RegisterAdapter("appnexus", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "appnexus").Msg("failed to register adapter")
	}
}
