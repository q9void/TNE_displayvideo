// Package exchange — curated deals catalog hydration.
//
// At the start of every auction we walk imp.pmp.deals[] and look each deal_id
// up in the curator catalog (storage.CuratorStore behind the CuratorCatalog
// interface). Catalog matches drive three downstream behaviors:
//
//   1. Hydration: missing wseat / wadomain / bidfloor / at on the inbound deal
//      are filled in from the catalog, so curators can register a deal once
//      and have downstream policy applied uniformly. Inbound values win on
//      conflict — publishers stay authoritative for what they accept.
//   2. Signal passthrough (chunk 0.5): when a bidder's seat is permitted on a
//      curated deal, EID and FPD strip rules are relaxed so curator-injected
//      audience data survives the per-bidder clone.
//   3. SChain attribution (chunk 0.6): each unique curator that contributed a
//      deal gets a node appended to source.ext.schain.nodes so DSPs can audit
//      where the signal came from.
//
// The catalog interface is small and side-effect free so tests can inject
// fakes without spinning up Postgres.
package exchange

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// CuratorCatalog is the read-only surface the auction layer needs from the
// curator store. Backed by *storage.CuratorStore in production, and by fakes
// in tests.
type CuratorCatalog interface {
	LookupDeal(ctx context.Context, dealID string) (*storage.CuratorDeal, error)
	LoadCurator(ctx context.Context, curatorID string) (*storage.Curator, error)
	SeatsForCurator(ctx context.Context, curatorID, bidderCode string) ([]string, error)
	PublisherAllowedForCurator(ctx context.Context, publisherID int, curatorID string) (bool, error)
}

// CuratorContext is the per-auction view assembled from the catalog. It is
// stashed on context.Context via WithCuratorContext for downstream stages
// (clone, schain, analytics) to read without changing function signatures.
type CuratorContext struct {
	// DealsByID holds catalog-matched deals only. Unmatched inbound deals
	// (those without a curator_deals row) are intentionally NOT present.
	DealsByID map[string]*storage.CuratorDeal

	// CuratorsByID is the deduped set of curators contributing to this
	// auction. Drives schain node augmentation and analytics roll-up.
	CuratorsByID map[string]*storage.Curator

	// OriginalUserEIDs is the inbound user.eids slice captured BEFORE the
	// global EID source allow-list filter mutates the request. Used to
	// re-attach curator-permitted EIDs on per-bidder clones when the bidder
	// is on a curated deal's wseat. nil ⇒ inbound request had no EIDs.
	OriginalUserEIDs []openrtb.EID

	// permittedCache memoizes IsBidderPermitted decisions per (bidder_code).
	// Catalog seat lookups are stable for the duration of an auction.
	permittedCache map[string]bool

	// receipts collects per-bidder per-deal signal receipts during fanout.
	// Bulk-flushed by the exchange after the auction completes via
	// analytics.Module.LogSignalReceipts.
	receiptsMu sync.Mutex
	receipts   []analytics.SignalReceipt
}

// AddSignalReceipt appends a receipt under the internal mutex.
// Safe to call from per-bidder goroutines during fanout.
func (c *CuratorContext) AddSignalReceipt(r analytics.SignalReceipt) {
	if c == nil {
		return
	}
	c.receiptsMu.Lock()
	defer c.receiptsMu.Unlock()
	c.receipts = append(c.receipts, r)
}

// DrainSignalReceipts returns the accumulated receipts and resets the buffer.
// Called once by RunAuction post-fanout to forward to analytics.
func (c *CuratorContext) DrainSignalReceipts() []analytics.SignalReceipt {
	if c == nil {
		return nil
	}
	c.receiptsMu.Lock()
	defer c.receiptsMu.Unlock()
	out := c.receipts
	c.receipts = nil
	return out
}

// HasDeal reports whether deal_id was hydrated from the catalog.
func (c *CuratorContext) HasDeal(dealID string) bool {
	if c == nil || c.DealsByID == nil {
		return false
	}
	_, ok := c.DealsByID[dealID]
	return ok
}

// CuratorIDs returns a stable slice of curator IDs that participated in this
// auction (for analytics auction_events.curator_ids and admin audits).
func (c *CuratorContext) CuratorIDs() []string {
	if c == nil || len(c.CuratorsByID) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.CuratorsByID))
	for id := range c.CuratorsByID {
		out = append(out, id)
	}
	return out
}

// IsBidderPermitted reports whether bidderCode has a registered curator seat
// that intersects any hydrated deal's wseat in this auction. Permitted bidders
// receive curator-injected signals (EIDs, segments) that would otherwise be
// stripped by the global allow-list. Result is memoized for the auction.
//
// The catalog argument MUST be non-nil — call sites that pass nil will get a
// false answer and fall through to the default (signals stripped).
func (c *CuratorContext) IsBidderPermitted(ctx context.Context, catalog CuratorCatalog, bidderCode string) bool {
	if c == nil || catalog == nil || bidderCode == "" || len(c.DealsByID) == 0 {
		return false
	}
	if c.permittedCache == nil {
		c.permittedCache = make(map[string]bool, 8)
	}
	if v, ok := c.permittedCache[bidderCode]; ok {
		return v
	}
	for _, deal := range c.DealsByID {
		seats, err := catalog.SeatsForCurator(ctx, deal.CuratorID, bidderCode)
		if err != nil || len(seats) == 0 {
			continue
		}
		seatSet := make(map[string]struct{}, len(seats))
		for _, s := range seats {
			seatSet[s] = struct{}{}
		}
		for _, w := range deal.WSeat {
			if _, ok := seatSet[w]; ok {
				c.permittedCache[bidderCode] = true
				return true
			}
		}
	}
	c.permittedCache[bidderCode] = false
	return false
}

type curatorCtxKey struct{}

// WithCuratorContext returns a context carrying the given curator context.
// Reads via CuratorContextFromCtx. nil-safe in both directions.
func WithCuratorContext(ctx context.Context, cc *CuratorContext) context.Context {
	if cc == nil {
		return ctx
	}
	return context.WithValue(ctx, curatorCtxKey{}, cc)
}

// CuratorContextFromCtx extracts the curator context. Returns nil when no
// catalog hydration ran (no catalog configured, no PMP deals, etc.).
func CuratorContextFromCtx(ctx context.Context) *CuratorContext {
	if ctx == nil {
		return nil
	}
	cc, _ := ctx.Value(curatorCtxKey{}).(*CuratorContext)
	return cc
}

// hydrateCuratedDeals walks imp.pmp.deals[] in the bid request, looks each
// deal up in the catalog, and overlays missing metadata from the curator's
// registered values. The bid request is mutated in place; the returned
// *CuratorContext records which deals/curators were matched.
//
// Behavior on edge cases:
//   - No catalog configured (nil receiver field): returns a zero-value
//     context, request unchanged. Curated-deals support is opt-in.
//   - Deal not in catalog: passes through as-is (we do NOT drop it). This
//     supports upstream-SSP curators whose deals haven't been mirrored to
//     our catalog yet. Phase 1.2 tightens this to enforce publisher
//     allow-lists.
//   - Catalog lookup error: warning logged, the deal is treated as unmatched.
// hydrateCuratedDealsFor wraps hydrateCuratedDeals with a publisher allow-list
// check. When publisherDBID > 0 and the curator catalog reports the publisher
// is NOT allow-listed for a given curator, that curator's deals are dropped
// from the hydrated set (the inbound deal entry remains in pmp.deals so
// downstream code can still see it; it just won't get curator overlay,
// signal passthrough, or schain attribution).
func (e *Exchange) hydrateCuratedDealsFor(
	ctx context.Context,
	req *openrtb.BidRequest,
	publisherDBID int,
) *CuratorContext {
	cc := e.hydrateCuratedDeals(ctx, req)
	if e.curatorCatalog == nil || publisherDBID <= 0 || len(cc.DealsByID) == 0 {
		return cc
	}

	// Drop curator deals whose curator isn't allow-listed for this publisher.
	for dealID, deal := range cc.DealsByID {
		ok, err := e.curatorCatalog.PublisherAllowedForCurator(ctx, publisherDBID, deal.CuratorID)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("deal_id", dealID).
				Str("curator_id", deal.CuratorID).
				Int("publisher_db_id", publisherDBID).
				Msg("publisher allow-list check failed; keeping deal hydrated (fail-open)")
			continue
		}
		if !ok {
			logger.Log.Info().
				Str("deal_id", dealID).
				Str("curator_id", deal.CuratorID).
				Int("publisher_db_id", publisherDBID).
				Msg("publisher not allow-listed for curator; deal will not receive curator overlay")
			delete(cc.DealsByID, dealID)
		}
	}
	// If all of a curator's deals were dropped, remove the curator from the
	// schain attribution set too — they didn't actually contribute to this
	// auction's permitted set.
	stillReferenced := make(map[string]struct{}, len(cc.DealsByID))
	for _, deal := range cc.DealsByID {
		stillReferenced[deal.CuratorID] = struct{}{}
	}
	for curatorID := range cc.CuratorsByID {
		if _, ok := stillReferenced[curatorID]; !ok {
			delete(cc.CuratorsByID, curatorID)
		}
	}
	return cc
}

func (e *Exchange) hydrateCuratedDeals(ctx context.Context, req *openrtb.BidRequest) *CuratorContext {
	cc := &CuratorContext{
		DealsByID:    make(map[string]*storage.CuratorDeal),
		CuratorsByID: make(map[string]*storage.Curator),
	}
	// Snapshot inbound EIDs BEFORE the global allow-list filter runs. Even
	// when no catalog is configured we capture this — keeps the field
	// authoritative if curated-deals is enabled later in the same process.
	if req != nil && req.User != nil && len(req.User.EIDs) > 0 {
		cc.OriginalUserEIDs = append(make([]openrtb.EID, 0, len(req.User.EIDs)), req.User.EIDs...)
	}
	if e.curatorCatalog == nil || req == nil {
		return cc
	}

	for i := range req.Imp {
		if req.Imp[i].PMP == nil || len(req.Imp[i].PMP.Deals) == 0 {
			continue
		}
		for j := range req.Imp[i].PMP.Deals {
			d := &req.Imp[i].PMP.Deals[j]
			if d.ID == "" {
				continue
			}
			catDeal, err := e.curatorCatalog.LookupDeal(ctx, d.ID)
			if err != nil {
				logger.Log.Warn().
					Err(err).
					Str("deal_id", d.ID).
					Str("auction_id", req.ID).
					Msg("curator catalog lookup failed; passing deal through unhydrated")
				continue
			}
			if catDeal == nil {
				continue
			}
			cc.DealsByID[d.ID] = catDeal
			overlayDeal(d, catDeal)

			// Deduplicate curator load across multiple deals from the same curator.
			if _, seen := cc.CuratorsByID[catDeal.CuratorID]; !seen {
				cur, err := e.curatorCatalog.LoadCurator(ctx, catDeal.CuratorID)
				if err != nil {
					logger.Log.Warn().
						Err(err).
						Str("curator_id", catDeal.CuratorID).
						Msg("curator load failed; deal still hydrated but schain node will be skipped")
					continue
				}
				if cur != nil {
					cc.CuratorsByID[catDeal.CuratorID] = cur
				}
			}
		}
	}

	if len(cc.DealsByID) > 0 {
		logger.Log.Debug().
			Str("auction_id", req.ID).
			Int("hydrated_deals", len(cc.DealsByID)).
			Int("curators", len(cc.CuratorsByID)).
			Msg("curated deals hydrated")
	}
	return cc
}

// collectSignalReceiptAcks scans bidder results for `bid.ext.signal_receipt`
// (any truthy presence) and returns one ack per (bidder, deal_id) pair.
// Used to mark previously-recorded signal receipts as acknowledged in the
// audit table so curators can prove the DSP actually got the payload.
//
// Looks at bid.Ext only — no spec change required from adapters. Adapters
// that already pass through bid.ext verbatim (the default) will support
// acks transparently when their DSPs include the field.
func collectSignalReceiptAcks(auctionID string, results map[string]*BidderResult) []analytics.SignalReceiptAck {
	if len(results) == 0 {
		return nil
	}
	now := time.Now()
	var acks []analytics.SignalReceiptAck
	for bidderCode, br := range results {
		if br == nil {
			continue
		}
		seenDeals := make(map[string]struct{}, len(br.Bids))
		for _, tb := range br.Bids {
			if tb == nil || tb.Bid == nil || tb.Bid.DealID == "" || len(tb.Bid.Ext) == 0 {
				continue
			}
			if !hasSignalReceiptAck(tb.Bid.Ext) {
				continue
			}
			if _, dup := seenDeals[tb.Bid.DealID]; dup {
				continue
			}
			seenDeals[tb.Bid.DealID] = struct{}{}
			acks = append(acks, analytics.SignalReceiptAck{
				AuctionID:  auctionID,
				BidderCode: bidderCode,
				DealID:     tb.Bid.DealID,
				AckedAt:    now,
			})
		}
	}
	return acks
}

// hasSignalReceiptAck looks for any truthy "signal_receipt" key in a bid's
// ext blob. Accepts bool true, string "1"/"true"/"yes", or any non-null
// object — the precise shape is up to the adapter/DSP. Absent or false is
// treated as no-ack.
func hasSignalReceiptAck(ext []byte) bool {
	if len(ext) == 0 {
		return false
	}
	type ackProbe struct {
		SignalReceipt json.RawMessage `json:"signal_receipt"`
	}
	var probe ackProbe
	if err := json.Unmarshal(ext, &probe); err != nil {
		return false
	}
	if len(probe.SignalReceipt) == 0 {
		return false
	}
	s := string(probe.SignalReceipt)
	if s == "false" || s == "null" || s == `""` || s == "0" {
		return false
	}
	return true
}

// filterBiddersForCuratedDeals restricts the selected-bidder slice when the
// auction contains a private-marketplace deal (any imp.pmp.private_auction=1).
//
// In strict private-marketplace mode every accepted bidder must either:
//   - Have a curator_seats entry whose seat_id appears in the deal's wseat
//     for the deal's curator, OR
//   - Be on the deal's wseat directly when no curator catalog mapping exists
//     (graceful fallback for upstream-SSP curators not yet onboarded here).
//
// Bidders not on any deal's permitted seat list are dropped. Returns the
// input unchanged when there are no PMP deals or no impression marks
// private_auction=1 — open auctions are unaffected.
//
// Catalog argument MAY be nil (curated-deals support disabled); in that case
// we fall back to wseat-only matching.
func filterBiddersForCuratedDeals(
	ctx context.Context,
	imps []openrtb.Imp,
	bidders []string,
	cc *CuratorContext,
	catalog CuratorCatalog,
) []string {
	// Determine whether ANY imp is a strict PMP. If none, leave the bidder
	// list alone — curators can still ride open auctions.
	strict := false
	for _, imp := range imps {
		if imp.PMP != nil && imp.PMP.PrivateAuction == 1 && len(imp.PMP.Deals) > 0 {
			strict = true
			break
		}
	}
	if !strict {
		return bidders
	}

	// Build the union of permitted bidders from all PMP deals.
	permitted := make(map[string]struct{}, len(bidders))
	for _, imp := range imps {
		if imp.PMP == nil || imp.PMP.PrivateAuction != 1 {
			continue
		}
		for _, d := range imp.PMP.Deals {
			// Direct wseat match → permit any bidder whose adapter code
			// appears verbatim. Used by upstream curators who pass
			// adapter codes (e.g. "rubicon") in wseat.
			for _, w := range d.WSeat {
				permitted[w] = struct{}{}
			}
			// Catalog-mediated match: deal hydrated → join with curator_seats.
			if cc == nil || catalog == nil {
				continue
			}
			catDeal, ok := cc.DealsByID[d.ID]
			if !ok {
				continue
			}
			for _, b := range bidders {
				seats, err := catalog.SeatsForCurator(ctx, catDeal.CuratorID, b)
				if err != nil || len(seats) == 0 {
					continue
				}
				seatSet := make(map[string]struct{}, len(seats))
				for _, s := range seats {
					seatSet[s] = struct{}{}
				}
				for _, w := range catDeal.WSeat {
					if _, ok := seatSet[w]; ok {
						permitted[b] = struct{}{}
						break
					}
				}
			}
		}
	}

	out := make([]string, 0, len(bidders))
	for _, b := range bidders {
		if _, ok := permitted[b]; ok {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		// Defensive: if no bidder was permitted, log loud and return the
		// input unchanged. Better to attempt the auction than to silently
		// no-bid.
		logger.Log.Warn().
			Int("bidders_in", len(bidders)).
			Msg("curator fanout filter: PMP strict mode but no permitted bidders; falling back to all bidders")
		return bidders
	}
	return out
}

// recordSignalReceipts builds one analytics.SignalReceipt per hydrated deal
// where THIS bidder is permitted, capturing the EIDs/segments/schain that
// were actually serialized to that bidder. Appends to the curator context's
// internal buffer. Safe to call from per-bidder goroutines.
func recordSignalReceipts(
	cc *CuratorContext,
	clone *openrtb.BidRequest,
	catalog CuratorCatalog,
	bidderCode string,
	auctionID string,
) {
	if cc == nil || clone == nil || len(cc.DealsByID) == 0 {
		return
	}

	// Collect EID source domains forwarded to this bidder.
	var eidsSent []string
	if clone.User != nil && len(clone.User.EIDs) > 0 {
		eidsSent = make([]string, 0, len(clone.User.EIDs))
		for _, e := range clone.User.EIDs {
			if e.Source != "" {
				eidsSent = append(eidsSent, e.Source)
			}
		}
	}

	// Collect SDA-style segments: "iab<segtax>:<segment_id>".
	var segmentsSent []string
	if clone.User != nil && len(clone.User.Data) > 0 {
		for _, dat := range clone.User.Data {
			segtax := segtaxFromDataExt(dat.Ext)
			for _, seg := range dat.Segment {
				if seg.ID == "" {
					continue
				}
				if segtax > 0 {
					segmentsSent = append(segmentsSent, formatSegment(segtax, seg.ID))
				} else {
					segmentsSent = append(segmentsSent, "iab:"+seg.ID)
				}
			}
		}
	}

	// Snapshot the schain nodes serialized to this bidder.
	var schainNodes []analytics.SChainNodeSent
	if clone.Source != nil && clone.Source.SChain != nil {
		schainNodes = make([]analytics.SChainNodeSent, 0, len(clone.Source.SChain.Nodes))
		for _, n := range clone.Source.SChain.Nodes {
			schainNodes = append(schainNodes, analytics.SChainNodeSent{
				ASI: n.ASI, SID: n.SID, HP: n.HP, RID: n.RID,
			})
		}
	}

	// One receipt per (bidder, deal) where the bidder is permitted.
	now := time.Now()
	for dealID, deal := range cc.DealsByID {
		if !bidderPermittedOnDeal(cc, catalog, deal, bidderCode) {
			continue
		}
		// Resolve the seat the bidder holds at the curator (first one wins).
		seat := ""
		if catalog != nil {
			if seats, err := catalog.SeatsForCurator(context.Background(), deal.CuratorID, bidderCode); err == nil && len(seats) > 0 {
				seat = seats[0]
			}
		}
		cc.AddSignalReceipt(analytics.SignalReceipt{
			AuctionID:       auctionID,
			BidderCode:      bidderCode,
			DealID:          dealID,
			CuratorID:       deal.CuratorID,
			Seat:            seat,
			EIDsSent:        append([]string(nil), eidsSent...),
			SegmentsSent:    append([]string(nil), segmentsSent...),
			SChainNodesSent: append([]analytics.SChainNodeSent(nil), schainNodes...),
			SentAt:          now,
		})
	}
}

// bidderPermittedOnDeal is the cheap per-deal version of IsBidderPermitted —
// used in receipt building so we don't double-cache decisions across all deals
// when a bidder is permitted on one but not another curator's deal.
func bidderPermittedOnDeal(_ *CuratorContext, catalog CuratorCatalog, deal *storage.CuratorDeal, bidderCode string) bool {
	if catalog == nil || deal == nil || bidderCode == "" {
		return false
	}
	seats, err := catalog.SeatsForCurator(context.Background(), deal.CuratorID, bidderCode)
	if err != nil || len(seats) == 0 {
		return false
	}
	seatSet := make(map[string]struct{}, len(seats))
	for _, s := range seats {
		seatSet[s] = struct{}{}
	}
	for _, w := range deal.WSeat {
		if _, ok := seatSet[w]; ok {
			return true
		}
	}
	return false
}

// segtaxFromDataExt parses {"segtax": <int>} from a Data.ext blob. Returns 0
// if absent or malformed (caller falls back to a generic "iab:" prefix).
func segtaxFromDataExt(ext []byte) int {
	if len(ext) == 0 {
		return 0
	}
	type segtaxOnly struct {
		Segtax int `json:"segtax"`
	}
	var s segtaxOnly
	if err := json.Unmarshal(ext, &s); err != nil {
		return 0
	}
	return s.Segtax
}

// formatSegment produces "iab<segtax>:<segment_id>" without importing fmt.
func formatSegment(segtax int, segID string) string {
	// Hot path on auction; avoid fmt allocation. itoa via stdlib strconv is
	// pulled in transitively elsewhere — but to keep this file's import
	// surface minimal, just inline a tiny loop.
	digits := make([]byte, 0, 6)
	if segtax == 0 {
		digits = append(digits, '0')
	} else {
		for segtax > 0 {
			digits = append([]byte{byte('0' + segtax%10)}, digits...)
			segtax /= 10
		}
	}
	out := make([]byte, 0, 4+len(digits)+len(segID))
	out = append(out, 'i', 'a', 'b')
	out = append(out, digits...)
	out = append(out, ':')
	out = append(out, segID...)
	return string(out)
}

// prependCuratorSChainNodes adds one node per curator participating in this
// auction to the front of source.ext.schain.nodes. Curators are upstream
// signal/deal originators relative to TNE Catalyst; placing them first in the
// chain lets DSPs identify the data source when they receive a deal-tagged
// bid request. Stable order: sorted by curator ID for deterministic chains.
//
// Bound by exchange.CloneLimits.MaxSChainNodes — once the cap is hit, further
// curator nodes are dropped (warned in logs). Returns silently when there
// are no curators or the source is missing.
func prependCuratorSChainNodes(req *openrtb.BidRequest, curators []*storage.Curator, maxNodes int) {
	if req == nil || len(curators) == 0 {
		return
	}
	if req.Source == nil {
		req.Source = &openrtb.Source{}
	}
	if req.Source.SChain == nil {
		req.Source.SChain = &openrtb.SupplyChain{Complete: 1, Ver: "1.0"}
	}

	// Stable order — sort curators by ID so the chain is deterministic
	// across bidders and tests can pin expectations.
	sorted := make([]*storage.Curator, 0, len(curators))
	for _, c := range curators {
		if c != nil && c.SChainASI != "" {
			sorted = append(sorted, c)
		}
	}
	sortCuratorsByID(sorted)

	added := make([]openrtb.SupplyChainNode, 0, len(sorted))
	for _, c := range sorted {
		added = append(added, openrtb.SupplyChainNode{
			ASI: c.SChainASI,
			SID: c.SChainSID,
			HP:  1,
			RID: req.ID,
		})
	}

	// Prepend, then truncate to MaxSChainNodes. We trust the spec cap to
	// be larger than the number of curators in any realistic auction;
	// otherwise platform/partner nodes get dropped, which is the same
	// safety the existing augmentSChain enforces.
	combined := append(added, req.Source.SChain.Nodes...)
	if maxNodes > 0 && len(combined) > maxNodes {
		combined = combined[:maxNodes]
	}
	req.Source.SChain.Nodes = combined
}

// sortCuratorsByID is a small helper that avoids importing sort at the top of
// the file when the rest of the file doesn't need it.
func sortCuratorsByID(in []*storage.Curator) {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j-1].ID > in[j].ID; j-- {
			in[j-1], in[j] = in[j], in[j-1]
		}
	}
}

// overlayDeal fills in missing fields on the inbound deal from the catalog
// row. Inbound values win on conflict — the publisher's request remains the
// authoritative description of what they will accept. Catalog only hydrates
// gaps.
func overlayDeal(d *openrtb.Deal, cat *storage.CuratorDeal) {
	if d.BidFloor == 0 && cat.BidFloor.Valid {
		d.BidFloor = cat.BidFloor.Float64
	}
	if d.BidFloorCur == "" && cat.BidFloorCur != "" {
		d.BidFloorCur = cat.BidFloorCur
	}
	if d.AT == 0 && cat.AT.Valid {
		d.AT = int(cat.AT.Int64)
	}
	if len(d.WSeat) == 0 && len(cat.WSeat) > 0 {
		d.WSeat = append(make([]string, 0, len(cat.WSeat)), cat.WSeat...)
	}
	if len(d.WADomain) == 0 && len(cat.WAdomain) > 0 {
		d.WADomain = append(make([]string, 0, len(cat.WAdomain)), cat.WAdomain...)
	}
}
