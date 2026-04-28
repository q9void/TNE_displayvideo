// Build tag isolates this test from the unit suite; runs against a real
// Postgres seeded by the curator E2E setup. Skipped automatically when
// CATALYST_E2E_DSN is unset.
//go:build e2e
// +build e2e

package exchange_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/analytics"
	pgan "github.com/thenexusengine/tne_springwire/internal/analytics/postgres"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// captureAnalytics is an in-memory analytics.Module that records the auction
// summary + signal receipts so the test can assert on them.
type captureAnalytics struct {
	auction  *analytics.AuctionObject
	receipts []analytics.SignalReceipt
}

func (c *captureAnalytics) LogAuctionObject(_ context.Context, a *analytics.AuctionObject) error {
	c.auction = a
	return nil
}
func (c *captureAnalytics) LogVideoObject(_ context.Context, _ *analytics.VideoObject) error {
	return nil
}
func (c *captureAnalytics) LogSignalReceipts(_ context.Context, r []analytics.SignalReceipt) error {
	c.receipts = append(c.receipts, r...)
	return nil
}
func (c *captureAnalytics) AckSignalReceipts(_ context.Context, _ []analytics.SignalReceiptAck) error {
	return nil
}
func (c *captureAnalytics) Shutdown() error { return nil }

func TestE2E_CuratedDealHydration_AgainstRealDB(t *testing.T) {
	dsn := os.Getenv("CATALYST_E2E_DSN")
	if dsn == "" {
		t.Skip("CATALYST_E2E_DSN unset; skipping E2E test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	curator := storage.NewCuratorStore(db)

	// Sanity check the seed actually landed.
	d, err := curator.LookupDeal(context.Background(), "ACME-DEAL-1")
	if err != nil || d == nil {
		t.Fatalf("seed missing: deal=%v err=%v", d, err)
	}
	if d.CuratorID != "curator-acme" {
		t.Fatalf("unexpected curator: %s", d.CuratorID)
	}
	if len(d.WSeat) != 1 || d.WSeat[0] != "acme-rub-seat" {
		t.Fatalf("wseat not hydrated from DB: %v", d.WSeat)
	}

	// Resolve publisher_db_id for the allow-list check.
	var pubID int
	err = db.QueryRow(`
		SELECT p.id FROM publishers_new p
		JOIN accounts a ON a.id = p.account_id
		WHERE a.account_id='12345' AND p.domain='e2e-publisher.example'`).Scan(&pubID)
	if err != nil {
		t.Fatalf("resolve publisher_id: %v", err)
	}

	// Build a minimal exchange with the catalog wired.
	cfg := &exchange.Config{
		DefaultTimeout: 100 * time.Millisecond,
		CloneLimits:    exchange.DefaultCloneLimits(),
	}
	ex := exchange.New(adapters.NewRegistry(), cfg).WithCuratorCatalog(curator)

	// Inbound: id-only PMP deal that must be hydrated from the catalog.
	req := &openrtb.BidRequest{
		ID: "auct-e2e-1",
		Imp: []openrtb.Imp{{
			ID:  "imp-1",
			PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "ACME-DEAL-1"}}},
		}},
		User: &openrtb.User{
			EIDs: []openrtb.EID{
				{Source: "audigent.com", UIDs: []openrtb.UID{{ID: "user-from-acme"}}},
			},
		},
	}

	// Drive hydration via the public path (publisher-aware overload).
	cc := exchange.HydrateForTest(ex, context.Background(), req, pubID)

	// --- Assertions: criterion (a) signals attribution
	if !cc.HasDeal("ACME-DEAL-1") {
		t.Fatalf("deal not hydrated: %#v", cc)
	}
	if _, ok := cc.CuratorsByID["curator-acme"]; !ok {
		t.Fatalf("curator not recorded in context: %v", cc.CuratorsByID)
	}
	d0 := req.Imp[0].PMP.Deals[0]
	if d0.BidFloor != 2.50 || d0.BidFloorCur != "USD" || d0.AT != 2 {
		t.Errorf("catalog overlay missed: %+v", d0)
	}
	if len(d0.WSeat) != 1 || d0.WSeat[0] != "acme-rub-seat" {
		t.Errorf("wseat overlay missed: %+v", d0.WSeat)
	}
	if len(cc.OriginalUserEIDs) != 1 || cc.OriginalUserEIDs[0].Source != "audigent.com" {
		t.Errorf("EID snapshot missing: %+v", cc.OriginalUserEIDs)
	}

	// Re-run with a non-allow-listed publisher_id; deal must NOT hydrate.
	denied := exchange.HydrateForTest(ex, context.Background(), &openrtb.BidRequest{
		ID:  "auct-e2e-2",
		Imp: []openrtb.Imp{{ID: "imp-1", PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "ACME-DEAL-1"}}}}},
	}, 999999)
	if denied.HasDeal("ACME-DEAL-1") {
		t.Fatalf("publisher 999999 should NOT see hydrated deal (not allow-listed)")
	}

	t.Logf("E2E OK: deal=%s curator=%s wseat=%v eids=%d",
		"ACME-DEAL-1", d.CuratorID, d0.WSeat, len(cc.OriginalUserEIDs))

	// --- Persistence: prove the postgres adapter writes signal_receipts.
	pgAdapter := pgan.NewAdapter(db)
	receipt := analytics.SignalReceipt{
		AuctionID:    "auct-e2e-1",
		BidderCode:   "rubicon",
		DealID:       "ACME-DEAL-1",
		CuratorID:    "curator-acme",
		Seat:         "acme-rub-seat",
		EIDsSent:     []string{"audigent.com"},
		SegmentsSent: []string{"iab4:9001"},
		SChainNodesSent: []analytics.SChainNodeSent{
			{ASI: "acme.example", SID: "acme-001", HP: 1, RID: "auct-e2e-1"},
		},
		SentAt: time.Now(),
	}
	if err := pgAdapter.LogSignalReceipts(context.Background(), []analytics.SignalReceipt{receipt}); err != nil {
		t.Fatalf("LogSignalReceipts: %v", err)
	}
	// LogSignalReceipts inserts asynchronously in a goroutine — wait briefly
	// then poll the row.
	deadline := time.Now().Add(2 * time.Second)
	var got int
	for time.Now().Before(deadline) {
		if err := db.QueryRow(`SELECT count(*) FROM signal_receipts WHERE deal_id='ACME-DEAL-1'`).Scan(&got); err == nil && got > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got == 0 {
		t.Fatalf("signal_receipts row never landed")
	}

	// Inspect the persisted row to prove signals are visible end-to-end.
	var rec struct {
		Auction, Bidder, Deal, Curator, Seat string
		EIDs, Segs                           []string
		SChain                               []byte
	}
	row := db.QueryRow(`
		SELECT auction_id, bidder_code, deal_id, curator_id, seat,
		       eids_sent, segments_sent, schain_nodes_sent
		  FROM signal_receipts
		 WHERE deal_id='ACME-DEAL-1' ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&rec.Auction, &rec.Bidder, &rec.Deal, &rec.Curator, &rec.Seat,
		pq.Array(&rec.EIDs), pq.Array(&rec.Segs), &rec.SChain); err != nil {
		t.Fatalf("scan signal_receipts: %v", err)
	}
	if rec.Curator != "curator-acme" || rec.Seat != "acme-rub-seat" {
		t.Errorf("audit row mismatched: %s", jsonString(rec))
	}
	if len(rec.EIDs) != 1 || rec.EIDs[0] != "audigent.com" {
		t.Errorf("expected EID source recorded: %v", rec.EIDs)
	}
	t.Logf("PERSISTED signal_receipt: auction=%s bidder=%s curator=%s seat=%s eids=%v segs=%v",
		rec.Auction, rec.Bidder, rec.Curator, rec.Seat, rec.EIDs, rec.Segs)

	// Drive the admin signal-receipts aggregator query to prove the
	// /admin/curators/{id}/signal-receipts endpoint sees the row.
	type aggRow struct {
		DealID       string
		BidderCode   string
		ReceiptCount int
	}
	rows, err := db.Query(`
		SELECT deal_id, bidder_code, COUNT(*)
		  FROM signal_receipts
		 WHERE curator_id=$1
		 GROUP BY deal_id, bidder_code`, "curator-acme")
	if err != nil {
		t.Fatalf("admin agg query: %v", err)
	}
	defer rows.Close()
	var aggs []aggRow
	for rows.Next() {
		var a aggRow
		_ = rows.Scan(&a.DealID, &a.BidderCode, &a.ReceiptCount)
		aggs = append(aggs, a)
	}
	if len(aggs) == 0 {
		t.Fatalf("admin aggregator returned 0 rows")
	}
	t.Logf("ADMIN AUDIT: %+v", aggs)
}


// jsonString is a tiny pretty-print helper used in failure messages.
func jsonString(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
