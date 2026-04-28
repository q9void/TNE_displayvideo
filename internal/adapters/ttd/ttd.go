// Package ttd is a minimal OpenRTB-passthrough adapter for TheTradeDesk DSP.
//
// Curated-deals integration: see internal/adapters/dv360 for the full
// rationale. Curators register a curator_seats(curator, "ttd", <ttd_seat>)
// row, and the exchange routes deals carrying that seat in wseat to this
// adapter. TTD's bidder endpoint accepts standard OpenRTB 2.6.
//
// Phase 2 SCOPE: SKELETON only. A production TTD deployment requires:
//   * TTD seat onboarding + bid token (openrtb auth)
//   * Per-region bidder URLs (TTD operates regional PoPs)
//   * TTD-specific imp.ext shape (e.g. PartnerID, AdvertiserID)
//   * Audience/segment passthrough via TTD's Identity Alliance ID set
package ttd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// defaultEndpoint is a placeholder. Real TTD bidders use seat-specific URLs
// configured via TTD_ENDPOINT.
const defaultEndpoint = "https://insight.adsrvr.org/openrtb"

// Adapter implements the TheTradeDesk OpenRTB-passthrough bidder.
type Adapter struct {
	endpoint  string
	seatID    string // TTD partner/seat ID for curated-deal attribution
	bidToken  string // optional pre-shared auth token
}

// New constructs a TTD adapter. Reads TTD_ENDPOINT, TTD_SEAT_ID,
// TTD_BID_TOKEN from the environment when arguments are empty.
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = os.Getenv("TTD_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{
		endpoint: endpoint,
		seatID:   os.Getenv("TTD_SEAT_ID"),
		bidToken: os.Getenv("TTD_BID_TOKEN"),
	}
}

// MakeRequests serializes the BidRequest as-is and posts to the TTD endpoint.
func (a *Adapter) MakeRequests(req *openrtb.BidRequest, _ *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	if req == nil {
		return nil, []error{fmt.Errorf("ttd: nil request")}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, []error{fmt.Errorf("ttd: marshal request: %w", err)}
	}

	hdr := http.Header{}
	hdr.Set("Content-Type", "application/json;charset=UTF-8")
	hdr.Set("Accept", "application/json")
	hdr.Set("X-Openrtb-Version", "2.6")
	if a.seatID != "" {
		hdr.Set("X-TTD-Seat-Id", a.seatID)
	}
	if a.bidToken != "" {
		hdr.Set("Authorization", "Bearer "+a.bidToken)
	}

	return []*adapters.RequestData{{
		Method:  http.MethodPost,
		URI:     a.endpoint,
		Body:    body,
		Headers: hdr,
	}}, nil
}

// Info returns adapter capability metadata used by the registry. Default-on
// but, like DV360, only enters fanout when a curator routes a deal to it.
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled:     true,
		GVLVendorID: 21, // TheTradeDesk's published GVL ID (Phase 2; verify before prod)
		Endpoint:    defaultEndpoint,
		Maintainer:  &adapters.MaintainerInfo{Email: "engineering@thenexusengine.com"},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
			App:  &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
		},
		DemandType: adapters.DemandTypePlatform,
	}
}

func init() {
	if err := adapters.RegisterAdapter("ttd", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "ttd").Msg("failed to register adapter")
	}
}

// MakeBids parses a standard OpenRTB BidResponse from TTD.
func (a *Adapter) MakeBids(_ *openrtb.BidRequest, rd *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if rd == nil {
		return nil, nil
	}
	if rd.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if rd.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("ttd: unexpected status %d", rd.StatusCode)}
	}

	var resp openrtb.BidResponse
	if err := json.Unmarshal(bytes.TrimSpace(rd.Body), &resp); err != nil {
		return nil, []error{fmt.Errorf("ttd: parse response: %w", err)}
	}

	br := &adapters.BidderResponse{
		ResponseID: resp.ID,
		Currency:   resp.Cur,
		Bids:       make([]*adapters.TypedBid, 0, 4),
	}
	for i := range resp.SeatBid {
		for j := range resp.SeatBid[i].Bid {
			b := resp.SeatBid[i].Bid[j]
			br.Bids = append(br.Bids, &adapters.TypedBid{
				Bid:     &b,
				BidType: adapters.BidTypeBanner,
			})
		}
	}
	return br, nil
}
