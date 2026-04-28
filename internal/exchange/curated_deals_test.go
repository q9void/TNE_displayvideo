package exchange

import (
	"context"
	"database/sql"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// fakeCatalog is an in-memory CuratorCatalog for unit tests.
type fakeCatalog struct {
	deals     map[string]*storage.CuratorDeal
	curators  map[string]*storage.Curator
	seats     map[string][]string // key: curator_id + "|" + bidder_code
	allowList map[string]bool     // key: curator_id + "|" + publisher_id
}

func (f *fakeCatalog) LookupDeal(_ context.Context, dealID string) (*storage.CuratorDeal, error) {
	return f.deals[dealID], nil
}
func (f *fakeCatalog) LoadCurator(_ context.Context, id string) (*storage.Curator, error) {
	return f.curators[id], nil
}
func (f *fakeCatalog) SeatsForCurator(_ context.Context, curatorID, bidderCode string) ([]string, error) {
	return f.seats[curatorID+"|"+bidderCode], nil
}
func (f *fakeCatalog) PublisherAllowedForCurator(_ context.Context, _ int, _ string) (bool, error) {
	return true, nil
}

func TestHydrateCuratedDeals_OverlaysMissingFields(t *testing.T) {
	cat := &fakeCatalog{
		deals: map[string]*storage.CuratorDeal{
			"CUR1-DEAL1": {
				DealID:      "CUR1-DEAL1",
				CuratorID:   "c1",
				BidFloor:    sql.NullFloat64{Float64: 2.5, Valid: true},
				BidFloorCur: "USD",
				AT:          sql.NullInt64{Int64: 2, Valid: true},
				WSeat:       []string{"seat-c1-rub"},
				Active:      true,
			},
		},
		curators: map[string]*storage.Curator{
			"c1": {ID: "c1", Name: "Curator One", SChainASI: "c1.example", SChainSID: "sid-1"},
		},
	}

	ex := &Exchange{curatorCatalog: cat}
	req := &openrtb.BidRequest{
		ID: "auction-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				PMP: &openrtb.PMP{
					Deals: []openrtb.Deal{{ID: "CUR1-DEAL1"}},
				},
			},
		},
	}

	cc := ex.hydrateCuratedDeals(context.Background(), req)

	if !cc.HasDeal("CUR1-DEAL1") {
		t.Fatalf("expected deal hydrated; got %#v", cc)
	}
	if _, ok := cc.CuratorsByID["c1"]; !ok {
		t.Fatalf("expected curator c1 recorded; got %v", cc.CuratorsByID)
	}
	d := req.Imp[0].PMP.Deals[0]
	if d.BidFloor != 2.5 || d.BidFloorCur != "USD" || d.AT != 2 {
		t.Errorf("overlay missed scalar fields: %+v", d)
	}
	if len(d.WSeat) != 1 || d.WSeat[0] != "seat-c1-rub" {
		t.Errorf("wseat not hydrated: %v", d.WSeat)
	}
}

func TestHydrateCuratedDeals_InboundWinsOnConflict(t *testing.T) {
	cat := &fakeCatalog{
		deals: map[string]*storage.CuratorDeal{
			"D1": {
				DealID:      "D1",
				CuratorID:   "c1",
				BidFloor:    sql.NullFloat64{Float64: 5.0, Valid: true},
				BidFloorCur: "USD",
				WSeat:       []string{"catalog-seat"},
				Active:      true,
			},
		},
		curators: map[string]*storage.Curator{"c1": {ID: "c1"}},
	}

	ex := &Exchange{curatorCatalog: cat}
	req := &openrtb.BidRequest{
		ID: "a",
		Imp: []openrtb.Imp{{
			ID: "i", PMP: &openrtb.PMP{
				Deals: []openrtb.Deal{{
					ID:       "D1",
					BidFloor: 1.25,
					WSeat:    []string{"inbound-seat"},
				}},
			},
		}},
	}

	ex.hydrateCuratedDeals(context.Background(), req)

	d := req.Imp[0].PMP.Deals[0]
	if d.BidFloor != 1.25 {
		t.Errorf("expected inbound bidfloor preserved (1.25), got %v", d.BidFloor)
	}
	if len(d.WSeat) != 1 || d.WSeat[0] != "inbound-seat" {
		t.Errorf("expected inbound wseat preserved, got %v", d.WSeat)
	}
}

func TestHydrateCuratedDeals_NoCatalogIsNoOp(t *testing.T) {
	ex := &Exchange{} // no curatorCatalog
	req := &openrtb.BidRequest{
		ID:  "a",
		Imp: []openrtb.Imp{{ID: "i", PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "D1"}}}}},
	}
	cc := ex.hydrateCuratedDeals(context.Background(), req)
	if cc == nil || len(cc.DealsByID) != 0 {
		t.Fatalf("expected empty context when no catalog wired, got %#v", cc)
	}
}

func TestHydrateCuratedDeals_UnknownDealPassesThrough(t *testing.T) {
	cat := &fakeCatalog{deals: map[string]*storage.CuratorDeal{}, curators: map[string]*storage.Curator{}}
	ex := &Exchange{curatorCatalog: cat}
	req := &openrtb.BidRequest{
		ID:  "a",
		Imp: []openrtb.Imp{{ID: "i", PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "UNKNOWN", BidFloor: 0.50}}}}},
	}
	cc := ex.hydrateCuratedDeals(context.Background(), req)
	if cc.HasDeal("UNKNOWN") {
		t.Fatalf("expected unknown deal NOT to be in catalog context")
	}
	// And the deal should still be present in the request — not dropped.
	if len(req.Imp[0].PMP.Deals) != 1 || req.Imp[0].PMP.Deals[0].BidFloor != 0.50 {
		t.Fatalf("expected unhydrated deal preserved as-is, got %+v", req.Imp[0].PMP.Deals)
	}
}

func TestCuratorContext_IsBidderPermitted(t *testing.T) {
	cat := &fakeCatalog{
		seats: map[string][]string{
			"c1|rubicon":  {"seat-c1-rub"},
			"c1|pubmatic": {"seat-c1-pub"},
		},
	}
	cc := &CuratorContext{
		DealsByID: map[string]*storage.CuratorDeal{
			"D1": {DealID: "D1", CuratorID: "c1", WSeat: []string{"seat-c1-rub"}},
		},
	}
	ctx := context.Background()

	if !cc.IsBidderPermitted(ctx, cat, "rubicon") {
		t.Errorf("expected rubicon permitted (seat-c1-rub matches deal wseat)")
	}
	if cc.IsBidderPermitted(ctx, cat, "pubmatic") {
		t.Errorf("expected pubmatic NOT permitted (seat-c1-pub not in deal wseat)")
	}
	if cc.IsBidderPermitted(ctx, cat, "unknown") {
		t.Errorf("expected unknown bidder NOT permitted")
	}
	// Memoization: changing the catalog should not flip the cached answer.
	cat.seats["c1|rubicon"] = nil
	if !cc.IsBidderPermitted(ctx, cat, "rubicon") {
		t.Errorf("expected memoized true to persist")
	}
}

func TestCuratorContext_PermittedFalseWhenNoDeals(t *testing.T) {
	cat := &fakeCatalog{}
	cc := &CuratorContext{}
	if cc.IsBidderPermitted(context.Background(), cat, "rubicon") {
		t.Errorf("no deals → never permitted")
	}
}

func TestHydrateCuratedDeals_SnapshotsOriginalEIDs(t *testing.T) {
	ex := &Exchange{} // no catalog → still snapshots EIDs
	req := &openrtb.BidRequest{
		ID: "a",
		User: &openrtb.User{
			EIDs: []openrtb.EID{
				{Source: "audigent.com"},
				{Source: "liveramp.com"},
			},
		},
	}
	cc := ex.hydrateCuratedDeals(context.Background(), req)
	if len(cc.OriginalUserEIDs) != 2 {
		t.Fatalf("expected 2 EIDs snapshotted, got %d", len(cc.OriginalUserEIDs))
	}
	// Mutating req.User.EIDs after snapshot must not corrupt the snapshot.
	req.User.EIDs = nil
	if len(cc.OriginalUserEIDs) != 2 {
		t.Errorf("snapshot was not deep-copied: %v", cc.OriginalUserEIDs)
	}
}

func TestPrependCuratorSChainNodes_StableOrderAndCap(t *testing.T) {
	req := &openrtb.BidRequest{
		ID: "a",
		Source: &openrtb.Source{
			SChain: &openrtb.SupplyChain{
				Complete: 1, Ver: "1.0",
				Nodes: []openrtb.SupplyChainNode{
					{ASI: "publisher.example", SID: "p-1"},
				},
			},
		},
	}
	curators := []*storage.Curator{
		{ID: "c2", SChainASI: "c2.example", SChainSID: "sid-2"},
		{ID: "c1", SChainASI: "c1.example", SChainSID: "sid-1"},
	}
	prependCuratorSChainNodes(req, curators, 20)

	got := req.Source.SChain.Nodes
	if len(got) != 3 {
		t.Fatalf("expected 3 nodes, got %d (%+v)", len(got), got)
	}
	// Sorted by curator ID, then existing nodes preserved.
	if got[0].ASI != "c1.example" || got[1].ASI != "c2.example" || got[2].ASI != "publisher.example" {
		t.Fatalf("unexpected order: %+v", got)
	}
	if got[0].RID != "a" {
		t.Errorf("expected RID set to auction id, got %q", got[0].RID)
	}
}

func TestPrependCuratorSChainNodes_RespectsCap(t *testing.T) {
	req := &openrtb.BidRequest{ID: "a"}
	curators := []*storage.Curator{
		{ID: "c1", SChainASI: "c1"},
		{ID: "c2", SChainASI: "c2"},
		{ID: "c3", SChainASI: "c3"},
	}
	prependCuratorSChainNodes(req, curators, 2)
	if got := len(req.Source.SChain.Nodes); got != 2 {
		t.Fatalf("expected truncation to 2, got %d", got)
	}
}

func TestPrependCuratorSChainNodes_SkipsCuratorsMissingASI(t *testing.T) {
	req := &openrtb.BidRequest{ID: "a"}
	curators := []*storage.Curator{
		{ID: "c1"}, // no SChainASI
		{ID: "c2", SChainASI: "c2.example", SChainSID: "sid-2"},
	}
	prependCuratorSChainNodes(req, curators, 10)
	if got := len(req.Source.SChain.Nodes); got != 1 || req.Source.SChain.Nodes[0].ASI != "c2.example" {
		t.Fatalf("expected only c2 appended, got %+v", req.Source.SChain.Nodes)
	}
}

func TestCuratorContext_ContextRoundTrip(t *testing.T) {
	cc := &CuratorContext{
		DealsByID:    map[string]*storage.CuratorDeal{"D": {DealID: "D"}},
		CuratorsByID: map[string]*storage.Curator{"c": {ID: "c"}},
	}
	ctx := WithCuratorContext(context.Background(), cc)
	got := CuratorContextFromCtx(ctx)
	if got != cc {
		t.Fatalf("expected same pointer round-tripped, got %p vs %p", got, cc)
	}
	if CuratorContextFromCtx(context.Background()) != nil {
		t.Errorf("expected nil from empty context")
	}
}
