package dv360

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestDV360_MakeRequests_HappyPath(t *testing.T) {
	t.Setenv("DV360_SEAT_ID", "seat-1")
	a := New("https://example.test/openrtb")

	req := &openrtb.BidRequest{
		ID: "auct-1",
		Imp: []openrtb.Imp{{
			ID: "imp-1",
			PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "DEAL-1"}}},
		}},
		Site: &openrtb.Site{Domain: "publisher.example"},
	}
	got, errs := a.MakeRequests(req, nil)
	if len(errs) != 0 {
		t.Fatalf("MakeRequests errors: %v", errs)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 request, got %d", len(got))
	}
	if got[0].Method != http.MethodPost {
		t.Errorf("expected POST, got %s", got[0].Method)
	}
	if got[0].Headers.Get("X-DV360-Seat-Id") != "seat-1" {
		t.Errorf("missing seat header: %v", got[0].Headers)
	}
	if got[0].Headers.Get("X-Openrtb-Version") != "2.6" {
		t.Errorf("missing openrtb version header")
	}

	var roundTrip openrtb.BidRequest
	if err := json.Unmarshal(got[0].Body, &roundTrip); err != nil {
		t.Fatalf("body not valid json: %v", err)
	}
	if roundTrip.ID != "auct-1" {
		t.Errorf("body lost id: %+v", roundTrip)
	}
}

func TestDV360_MakeBids_PreservesDealID(t *testing.T) {
	a := New("https://example.test/openrtb")
	body := `{"id":"r1","cur":"USD","seatbid":[{"bid":[{"id":"b1","impid":"i1","price":2.50,"dealid":"DEAL-1"}]}]}`
	rd := &adapters.ResponseData{StatusCode: 200, Body: []byte(body)}
	br, errs := a.MakeBids(nil, rd)
	if len(errs) != 0 {
		t.Fatalf("MakeBids errors: %v", errs)
	}
	if br == nil || len(br.Bids) != 1 {
		t.Fatalf("expected 1 bid, got %#v", br)
	}
	if br.Bids[0].Bid.DealID != "DEAL-1" {
		t.Errorf("deal_id lost in parse: %+v", br.Bids[0].Bid)
	}
}

func TestDV360_MakeBids_NoContentReturnsNil(t *testing.T) {
	a := New("https://example.test/openrtb")
	br, errs := a.MakeBids(nil, &adapters.ResponseData{StatusCode: http.StatusNoContent})
	if br != nil || len(errs) != 0 {
		t.Fatalf("expected nil/no-error for 204, got br=%v errs=%v", br, errs)
	}
}
