// Package smartadserver implements the Smart AdServer bidder adapter
package smartadserver

import (
	"encoding/json"
	"net/http"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const defaultEndpoint = "https://ssb-global.smartadserver.com/api/bid"

type Adapter struct{ endpoint string }

func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
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

	body, err := json.Marshal(&requestCopy)
	if err != nil {
		return nil, []error{err}
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	return []*adapters.RequestData{{Method: "POST", URI: a.endpoint, Body: body, Headers: headers}}, nil
}

func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode != http.StatusOK {
		return nil, nil
	}
	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		return nil, []error{err}
	}
	response := &adapters.BidderResponse{Currency: bidResp.Cur, ResponseID: bidResp.ID, Bids: make([]*adapters.TypedBid, 0)}
	for _, sb := range bidResp.SeatBid {
		for i := range sb.Bid {
			response.Bids = append(response.Bids, &adapters.TypedBid{Bid: &sb.Bid[i], BidType: adapters.BidTypeBanner})
		}
	}
	return response, nil
}

func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled: true, GVLVendorID: 45, Endpoint: defaultEndpoint,
		Maintainer: &adapters.MaintainerInfo{Email: "prebid@smartadserver.com"},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
			App:  &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
		},
	}
}

func init() {
	if err := adapters.RegisterAdapter("smartadserver", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "smartadserver").Msg("failed to register adapter")
	}
}
