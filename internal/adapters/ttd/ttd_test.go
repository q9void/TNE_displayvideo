package ttd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestTTD_MakeRequests_HeadersAndAuth(t *testing.T) {
	t.Setenv("TTD_SEAT_ID", "ttd-seat-99")
	t.Setenv("TTD_BID_TOKEN", "tok123")
	a := New("https://example.test/ttd")

	req := &openrtb.BidRequest{ID: "a1", Imp: []openrtb.Imp{{ID: "i1"}}}
	got, errs := a.MakeRequests(req, nil)
	if len(errs) != 0 {
		t.Fatalf("MakeRequests errors: %v", errs)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 request, got %d", len(got))
	}
	if got[0].Headers.Get("X-TTD-Seat-Id") != "ttd-seat-99" {
		t.Errorf("missing TTD seat header: %v", got[0].Headers)
	}
	if got[0].Headers.Get("Authorization") != "Bearer tok123" {
		t.Errorf("missing bearer token: %v", got[0].Headers)
	}
	var roundTrip openrtb.BidRequest
	if err := json.Unmarshal(got[0].Body, &roundTrip); err != nil {
		t.Fatalf("body not valid json: %v", err)
	}
}

func TestTTD_MakeBids_ParsesSeatBids(t *testing.T) {
	a := New("https://example.test/ttd")
	body := `{"id":"r1","cur":"USD","seatbid":[{"bid":[{"id":"b1","impid":"i1","price":1.20,"dealid":"D1"}]}]}`
	br, errs := a.MakeBids(nil, &adapters.ResponseData{StatusCode: 200, Body: []byte(body)})
	if len(errs) != 0 {
		t.Fatalf("MakeBids errors: %v", errs)
	}
	if br == nil || len(br.Bids) != 1 || br.Bids[0].Bid.DealID != "D1" {
		t.Fatalf("unexpected response: %#v", br)
	}
}

func TestTTD_NoContentReturnsNil(t *testing.T) {
	a := New("")
	br, errs := a.MakeBids(nil, &adapters.ResponseData{StatusCode: http.StatusNoContent})
	if br != nil || len(errs) != 0 {
		t.Fatalf("expected nil/no-error for 204, got br=%v errs=%v", br, errs)
	}
}
