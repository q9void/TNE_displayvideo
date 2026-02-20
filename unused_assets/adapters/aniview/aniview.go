// Package aniview implements the Aniview bidder adapter
package aniview

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	defaultEndpoint = "https://gov.aniview.com/api/adserver/vast3/"
)

// Adapter implements the Aniview bidder
type Adapter struct {
	endpoint string
}

// ExtImpAniview defines the Aniview-specific parameters
type ExtImpAniview struct {
	PublisherID string `json:"publisherId,omitempty"`
	ChannelID   string `json:"channelId,omitempty"`
	// Support alternate parameter names from Prebid.js
	AVPublisherID string `json:"AV_PUBLISHERID,omitempty"`
	AVChannelID   string `json:"AV_CHANNELID,omitempty"`
}

// New creates a new Aniview adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

// MakeRequests builds HTTP requests for Aniview
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errors []error
	var validImps []openrtb.Imp

	// Validate impressions and extract Aniview-specific parameters
	for _, imp := range request.Imp {
		var extImp ExtImpAniview
		if err := json.Unmarshal(imp.Ext, &extImp); err != nil {
			errors = append(errors, fmt.Errorf("failed to parse ext for imp %s: %w", imp.ID, err))
			continue
		}

		// Support both BizBudding (publisherId/channelId) and Prebid.js (AV_PUBLISHERID/AV_CHANNELID) parameter names
		publisherID := extImp.PublisherID
		if publisherID == "" {
			publisherID = extImp.AVPublisherID
		}
		channelID := extImp.ChannelID
		if channelID == "" {
			channelID = extImp.AVChannelID
		}

		// Validate required parameters
		if publisherID == "" {
			errors = append(errors, fmt.Errorf("publisherId is required for imp %s", imp.ID))
			continue
		}
		if channelID == "" {
			errors = append(errors, fmt.Errorf("channelId is required for imp %s", imp.ID))
			continue
		}

		validImps = append(validImps, imp)
	}

	// Return early if no valid impressions
	if len(validImps) == 0 {
		return nil, errors
	}

	// Create a single request with all valid impressions
	reqCopy := *request
	reqCopy.Imp = validImps

	requestBody, err := json.Marshal(reqCopy)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to marshal request: %w", err))
		return nil, errors
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

// MakeBids parses Aniview responses into bids
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
		Enabled:    true,
		GVLVendorID: 780,
		Endpoint:   defaultEndpoint,
		Maintainer: &adapters.MaintainerInfo{
			Email: "info@aniview.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
				},
			},
			App: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
				},
			},
		},
		DemandType: adapters.DemandTypePublisher, // Publisher's own demand partner
	}
}

func init() {
	if err := adapters.RegisterAdapter("aniview", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "aniview").Msg("failed to register adapter")
	}
}
