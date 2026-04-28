// Package dv360 is a minimal OpenRTB-passthrough adapter for Google DV360
// (the buyer side of Authorized Buyers / AdX).
//
// Curated-deals integration: when an inbound auction carries a deal whose
// curator has a registered curator_seats(curator, "dv360", <seat_id>) row
// AND the deal's wseat includes that seat_id, the exchange routes the imp
// here. The adapter forwards the OpenRTB BidRequest as-is — DV360's RTB
// endpoint accepts standard OpenRTB 2.6 — and tags the outgoing request so
// the buyer can attribute the deal back to the curator.
//
// Phase 2 SCOPE: This is a SKELETON. A production DV360 deployment requires:
//   * Authorized-Buyer onboarding (account, seat IDs, encryption keys)
//   * Bidder-protocol message types (the Google Bidder Protocol is OpenRTB
//     with proprietary extensions in `BidRequest.ext.google`)
//   * Per-seat URL routing (DV360 uses geo-distributed PoPs)
//   * Real-time decisioning latency budgets (typical 100ms tmax)
// The skeleton here is intentionally generic so it works against any
// OpenRTB-compatible endpoint while leaving the DV360-specific bits clearly
// flagged for the eventual production build.
package dv360

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// defaultEndpoint is a placeholder. Real DV360 buyers use their per-seat URL
// from the Authorized Buyers UI; configure via DV360_ENDPOINT env.
const defaultEndpoint = "https://googleads.g.doubleclick.net/td/adx/openrtb"

// Adapter implements the DV360 OpenRTB-passthrough bidder.
type Adapter struct {
	endpoint string
	seatID   string // configured via DV360_SEAT_ID; included in outbound bid request
}

// New constructs a DV360 adapter. endpoint may be empty to fall back to env
// or default. seatID is the buyer's Authorized Buyer account/seat used for
// curated-deal eligibility validation.
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = os.Getenv("DV360_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{
		endpoint: endpoint,
		seatID:   os.Getenv("DV360_SEAT_ID"),
	}
}

// MakeRequests builds a single OpenRTB POST per bid request. DV360 accepts
// the request body verbatim — no per-imp transformation needed for the
// curated-deal path.
func (a *Adapter) MakeRequests(req *openrtb.BidRequest, _ *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	if req == nil {
		return nil, []error{fmt.Errorf("dv360: nil request")}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, []error{fmt.Errorf("dv360: marshal request: %w", err)}
	}

	hdr := http.Header{}
	hdr.Set("Content-Type", "application/json;charset=UTF-8")
	hdr.Set("Accept", "application/json")
	hdr.Set("X-Openrtb-Version", "2.6")
	if a.seatID != "" {
		hdr.Set("X-DV360-Seat-Id", a.seatID)
	}

	return []*adapters.RequestData{{
		Method:  http.MethodPost,
		URI:     a.endpoint,
		Body:    body,
		Headers: hdr,
	}}, nil
}

// MakeBids parses a standard OpenRTB BidResponse from DV360 and returns
// TypedBids. Bid.DealID is preserved when DV360 fills against a curated
// deal — that field flows directly through to the auction's win-event row.
func (a *Adapter) MakeBids(_ *openrtb.BidRequest, rd *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if rd == nil {
		return nil, nil
	}
	if rd.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if rd.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("dv360: unexpected status %d", rd.StatusCode)}
	}

	var resp openrtb.BidResponse
	if err := json.Unmarshal(bytes.TrimSpace(rd.Body), &resp); err != nil {
		return nil, []error{fmt.Errorf("dv360: parse response: %w", err)}
	}

	br := &adapters.BidderResponse{
		ResponseID: resp.ID,
		Currency:   resp.Cur,
		Bids:       make([]*adapters.TypedBid, 0, 4),
	}
	for i := range resp.SeatBid {
		for j := range resp.SeatBid[i].Bid {
			b := resp.SeatBid[i].Bid[j]
			bidType := bidTypeFor(&b)
			br.Bids = append(br.Bids, &adapters.TypedBid{
				Bid:     &b,
				BidType: bidType,
			})
		}
	}
	return br, nil
}

// bidTypeFor infers the bid media type from common DV360 response shapes.
// Falls back to banner.
func bidTypeFor(b *openrtb.Bid) adapters.BidType {
	if b == nil {
		return adapters.BidTypeBanner
	}
	if b.AdM != "" && (containsCI(b.AdM, "<vast") || containsCI(b.AdM, "<videoad")) {
		return adapters.BidTypeVideo
	}
	return adapters.BidTypeBanner
}

func containsCI(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	// Case-insensitive ASCII contains; small-string fast path.
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a := s[i+j]
			b := sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
