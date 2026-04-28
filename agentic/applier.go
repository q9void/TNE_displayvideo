package agentic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// ApplierConfig governs the applier's behavior. All fields have sensible
// zero values per Phase 1 defaults; pass an explicit ApplierConfig to
// NewApplier when you want to override them.
type ApplierConfig struct {
	// MaxMutationsPerResponse caps the number of mutations honored per
	// Apply() call. Excess mutations are rejected with mutation_cap_exceeded.
	// Zero ⇒ default 64 per PRD §8.3.
	MaxMutationsPerResponse int

	// MaxIDsPerPayload caps the number of IDs in a single IDsPayload mutation.
	// Excess IDs cause the entire mutation to be rejected (defensive: agents
	// shouldn't be flooding us with thousands of segment IDs).
	// Zero ⇒ default 256.
	MaxIDsPerPayload int

	// ShadeMinFraction is the lowest fraction of original price a BID_SHADE
	// mutation may set. Default 0.5 ⇒ shading floor is 50% of the original.
	ShadeMinFraction float64

	// DisableShadeIntent kills BID_SHADE entirely when true. PRD OQ3 default
	// is true — the code path stays present but is gated until we trust a
	// shading agent in production.
	DisableShadeIntent bool

	// PublisherFloorLookup returns the publisher's configured floor for a
	// given imp ID. Used by ADJUST_DEAL_FLOOR to clamp agent-suggested floors
	// (R5.5.9). When nil, no clamping is performed (test mode).
	PublisherFloorLookup func(impID string) float64

	// MaxMetricsPerImp caps how many metrics one ADD_METRICS mutation may
	// append per impression. Default 32.
	MaxMetricsPerImp int

	// Now is injected for deterministic test latency tracking. Defaults to
	// time.Now when nil.
	Now func() time.Time

	// CuratorBindings maps agent_id → curator_id. When an agent listed here
	// emits ACTIVATE_DEALS or ACTIVATE_SEGMENTS, the applier validates the
	// payload against the bound curator's catalog before applying:
	//   - ACTIVATE_DEALS: deal_id must exist in curator_deals for the bound
	//     curator. Unknown deals are dropped from the payload (decision
	//     reason "deal_not_in_curator_catalog"); known deals proceed.
	//   - ACTIVATE_SEGMENTS: segment IDs proceed unconditionally (segtax
	//     allow-listing is enforced at the catalog layer).
	// nil ⇒ no curator binding; agents emit free-form mutations as before.
	CuratorBindings map[string]string

	// CuratorValidator is the lookup surface used when CuratorBindings is
	// set. nil ⇒ binding mode disables (cannot validate), ACTIVATE_DEALS
	// from a bound agent is rejected with reason "no_curator_validator".
	CuratorValidator CuratorValidator
}

// CuratorValidator is the minimal surface the applier needs to validate
// curator-bound mutations. Implemented by *storage.CuratorStore in production.
type CuratorValidator interface {
	DealIsCuratedBy(ctx context.Context, dealID, curatorID string) (bool, error)
}

func (c ApplierConfig) defaults() ApplierConfig {
	if c.MaxMutationsPerResponse <= 0 {
		c.MaxMutationsPerResponse = 64
	}
	if c.MaxIDsPerPayload <= 0 {
		c.MaxIDsPerPayload = 256
	}
	if c.ShadeMinFraction <= 0 {
		c.ShadeMinFraction = 0.5
	}
	if c.MaxMetricsPerImp <= 0 {
		c.MaxMetricsPerImp = 32
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Applier executes the mutation whitelist on an in-flight auction.
type Applier struct {
	cfg ApplierConfig
}

// NewApplier constructs an Applier with the given config. The cfg is copied
// and defaults are filled in; the caller may discard cfg after.
func NewApplier(cfg ApplierConfig) *Applier {
	return &Applier{cfg: cfg.defaults()}
}

// MarginAdjustment is the side-channel record produced by ADJUST_DEAL_MARGIN.
// The exchange's existing applyBidMultiplier (line ~1132) consumes this
// before final price calculation.
type MarginAdjustment struct {
	DealID          string
	Value           float64
	CalculationType pb.Margin_CalculationType
}

// ApplyOutput collects everything the exchange needs to read after Apply
// returns: the per-mutation decisions for audit logging, plus side-channel
// records for downstream stages (margin/shade/segment counters).
type ApplyOutput struct {
	Decisions          []ApplyDecision
	MarginAdjustments  []MarginAdjustment        // consumed by exchange before applyBidMultiplier
	BidShadingDeltas   map[string]float64        // bid.id → original_price - final_price
	SegmentsActivated  int                       // total segments added across all mutations
	AgentMutationCount map[string]map[string]int // agent_id → intent → count, for bid.ext.aamp
}

// Apply mutates req or rsp in place per the incoming mutations. origins must
// be a parallel slice (origins[i] describes the agent that produced muts[i]).
// Returns an ApplyOutput summarizing every decision.
//
// The mutation slice is sorted in place by (intent, priority, agent_id) for
// deterministic iteration. Conflicts on the same (intent, path) tuple use
// last-writer-wins ordered by priority — the loser is recorded as
// "superseded" rather than dropped silently.
func (a *Applier) Apply(
	req *openrtb.BidRequest,
	rsp *openrtb.BidResponse,
	muts []*pb.Mutation,
	origins []MutationOrigin,
	lc Lifecycle,
) ApplyOutput {
	out := ApplyOutput{
		BidShadingDeltas:   map[string]float64{},
		AgentMutationCount: map[string]map[string]int{},
	}
	if len(muts) == 0 {
		return out
	}
	if len(muts) != len(origins) {
		// Caller bug — surface loudly but don't panic the auction. Pad with
		// an unknown origin so attribution stays consistent.
		pad := make([]MutationOrigin, len(muts))
		copy(pad, origins)
		for i := range pad {
			if pad[i].AgentID == "" {
				pad[i].AgentID = "unknown"
			}
		}
		origins = pad
	}

	// Sort with original index preserved so we can re-zip mutations and
	// origins after the sort.
	type pair struct {
		m *pb.Mutation
		o MutationOrigin
		i int
	}
	pairs := make([]pair, len(muts))
	for i := range muts {
		pairs[i] = pair{muts[i], origins[i], i}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		ai, aj := pairs[i], pairs[j]
		ii, ij := intentRank(ai.m), intentRank(aj.m)
		if ii != ij {
			return ii < ij
		}
		if ai.o.Priority != aj.o.Priority {
			return ai.o.Priority < aj.o.Priority
		}
		return ai.o.AgentID < aj.o.AgentID
	})

	// Track conflicts: same (intent, path) → record loser as superseded.
	type pathKey struct {
		intent string
		path   string
	}
	winners := map[pathKey]string{} // intent+path → agent_id of last applied REPLACE

	for idx, p := range pairs {
		dec := ApplyDecision{
			AgentID: p.o.AgentID,
			Intent:  intentName(p.m),
			Op:      opName(p.m),
			Path:    p.m.GetPath(),
		}

		// R5.5.5 mutation cap.
		if idx >= a.cfg.MaxMutationsPerResponse {
			dec.Decision = "rejected"
			dec.Reason = "mutation_cap_exceeded"
			out.Decisions = append(out.Decisions, dec)
			continue
		}

		// R5.5.3 op gate.
		if p.m.GetOp() == pb.Operation_OPERATION_UNSPECIFIED {
			dec.Decision = "rejected"
			dec.Reason = "op_unspecified"
			out.Decisions = append(out.Decisions, dec)
			continue
		}

		// R5.5.1 / R5.5.2 intent + lifecycle whitelist.
		intent := p.m.GetIntent()
		hh, ok := handlers[intent]
		if !ok {
			dec.Decision = "rejected"
			dec.Reason = "unsupported_intent"
			out.Decisions = append(out.Decisions, dec)
			continue
		}
		if hh.lifecycle != lc {
			dec.Decision = "rejected"
			dec.Reason = "wrong_lifecycle"
			out.Decisions = append(out.Decisions, dec)
			continue
		}

		// R5.5.6 payload size — applies to IDsPayload before the handler runs.
		if ids := p.m.GetIds(); ids != nil {
			if len(ids.GetId()) > a.cfg.MaxIDsPerPayload {
				dec.Decision = "rejected"
				dec.Reason = "payload_size_exceeded"
				out.Decisions = append(out.Decisions, dec)
				continue
			}
		}

		// Last-writer-wins for REPLACE on same (intent, path).
		if p.m.GetOp() == pb.Operation_OPERATION_REPLACE {
			key := pathKey{dec.Intent, dec.Path}
			if prev, ok := winners[key]; ok {
				// Mark the previous applied decision as superseded.
				for i := len(out.Decisions) - 1; i >= 0; i-- {
					d := &out.Decisions[i]
					if d.Decision == "applied" && d.Intent == dec.Intent && d.Path == dec.Path && d.AgentID == prev {
						d.Decision = "superseded"
						d.SupersededBy = p.o.AgentID
						break
					}
				}
			}
			winners[key] = p.o.AgentID
		}

		dec = hh.fn(req, rsp, p.m, p.o, a.cfg, &out, dec)
		out.Decisions = append(out.Decisions, dec)
	}

	return out
}

// intentRank gives a stable ordering across the seven supported intents.
// Lower rank applies first.
func intentRank(m *pb.Mutation) int {
	switch m.GetIntent() {
	case pb.Intent_ACTIVATE_SEGMENTS:
		return 1
	case pb.Intent_ADD_CIDS:
		return 2
	case pb.Intent_ACTIVATE_DEALS:
		return 3
	case pb.Intent_SUPPRESS_DEALS:
		return 4
	case pb.Intent_ADJUST_DEAL_FLOOR:
		return 5
	case pb.Intent_ADD_METRICS:
		return 6
	case pb.Intent_ADJUST_DEAL_MARGIN:
		return 7
	case pb.Intent_BID_SHADE:
		return 8
	default:
		return 99
	}
}

func intentName(m *pb.Mutation) string {
	if m == nil {
		return "UNSPECIFIED"
	}
	return m.GetIntent().String()
}

func opName(m *pb.Mutation) string {
	if m == nil {
		return "UNSPECIFIED"
	}
	switch m.GetOp() {
	case pb.Operation_OPERATION_ADD:
		return "ADD"
	case pb.Operation_OPERATION_REMOVE:
		return "REMOVE"
	case pb.Operation_OPERATION_REPLACE:
		return "REPLACE"
	default:
		return "UNSPECIFIED"
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Handler dispatch table
// ──────────────────────────────────────────────────────────────────────────

type intentHandler func(
	req *openrtb.BidRequest,
	rsp *openrtb.BidResponse,
	m *pb.Mutation,
	origin MutationOrigin,
	cfg ApplierConfig,
	out *ApplyOutput,
	dec ApplyDecision,
) ApplyDecision

var handlers = map[pb.Intent]struct {
	lifecycle Lifecycle
	fn        intentHandler
}{
	pb.Intent_ACTIVATE_SEGMENTS:  {LifecyclePublisherBidRequest, applyActivateSegments},
	pb.Intent_ADD_CIDS:           {LifecyclePublisherBidRequest, applyActivateSegments}, // reserved; reuse segments shape
	pb.Intent_ACTIVATE_DEALS:     {LifecyclePublisherBidRequest, applyActivateDeals},
	pb.Intent_SUPPRESS_DEALS:     {LifecyclePublisherBidRequest, applySuppressDeals},
	pb.Intent_ADJUST_DEAL_FLOOR:  {LifecyclePublisherBidRequest, applyAdjustDealFloor},
	pb.Intent_ADD_METRICS:        {LifecyclePublisherBidRequest, applyAddMetrics},
	pb.Intent_ADJUST_DEAL_MARGIN: {LifecycleDSPBidResponse, applyAdjustDealMargin},
	pb.Intent_BID_SHADE:          {LifecycleDSPBidResponse, applyBidShade},
}

// ──────────────────────────────────────────────────────────────────────────
// Per-intent handlers
// ──────────────────────────────────────────────────────────────────────────

func applyActivateSegments(
	req *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, _ ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	// R5.5.4 path validation. We accept the canonical path or empty.
	if !pathOK(dec.Path, "user.data", "user.data[*].segment", "user.data[*].segment[*]", "") {
		dec.Decision = "rejected"
		dec.Reason = "path_invalid"
		return dec
	}
	ids := m.GetIds()
	if ids == nil || len(ids.GetId()) == 0 {
		dec.Decision = "rejected"
		dec.Reason = "empty_payload"
		return dec
	}
	if req.User == nil {
		req.User = &openrtb.User{}
	}
	// Build a set of existing segment IDs across all User.Data blocks for
	// dedupe — agents shouldn't be able to flood us with duplicate IDs.
	have := map[string]struct{}{}
	for _, d := range req.User.Data {
		for _, s := range d.Segment {
			have[s.ID] = struct{}{}
		}
	}
	added := make([]openrtb.Segment, 0, len(ids.GetId()))
	for _, id := range ids.GetId() {
		if _, dup := have[id]; dup {
			continue
		}
		have[id] = struct{}{}
		added = append(added, openrtb.Segment{ID: id})
	}
	if len(added) == 0 {
		dec.Decision = "applied"
		dec.Reason = "all_duplicates"
		return dec
	}
	req.User.Data = append(req.User.Data, openrtb.Data{
		ID:      origin.AgentID,
		Segment: added,
	})
	out.SegmentsActivated += len(added)
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applyActivateDeals(
	req *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, cfg ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	if !pathOK(dec.Path, "imp", "imp[*].pmp.deals", "imp[*].pmp.deals[*]", "") {
		dec.Decision = "rejected"
		dec.Reason = "path_invalid"
		return dec
	}
	ids := m.GetIds()
	if ids == nil || len(ids.GetId()) == 0 {
		dec.Decision = "rejected"
		dec.Reason = "empty_payload"
		return dec
	}

	// Curator-bound agents: every deal_id MUST belong to the bound curator's
	// catalog. Unknown deal_ids are filtered out before mutation. This is the
	// applier-side equivalent of an INJECT_CURATOR_SIGNALS intent — we use
	// the existing ACTIVATE_DEALS shape with an upstream validation hook
	// rather than introducing a new proto enum value.
	curatorID, bound := cfg.CuratorBindings[origin.AgentID]
	allowedIDs := ids.GetId()
	if bound {
		if cfg.CuratorValidator == nil {
			dec.Decision = "rejected"
			dec.Reason = "no_curator_validator"
			return dec
		}
		filtered := make([]string, 0, len(allowedIDs))
		dropped := 0
		for _, id := range allowedIDs {
			ok, err := cfg.CuratorValidator.DealIsCuratedBy(context.Background(), id, curatorID)
			if err != nil || !ok {
				dropped++
				continue
			}
			filtered = append(filtered, id)
		}
		if len(filtered) == 0 {
			dec.Decision = "rejected"
			dec.Reason = "deal_not_in_curator_catalog"
			return dec
		}
		allowedIDs = filtered
		if dropped > 0 {
			dec.Reason = fmt.Sprintf("dropped_%d_deals_not_in_curator_catalog", dropped)
		}
	}

	for i := range req.Imp {
		if req.Imp[i].PMP == nil {
			req.Imp[i].PMP = &openrtb.PMP{}
		}
		existing := map[string]struct{}{}
		for _, d := range req.Imp[i].PMP.Deals {
			existing[d.ID] = struct{}{}
		}
		for _, id := range allowedIDs {
			if _, ok := existing[id]; ok {
				continue
			}
			req.Imp[i].PMP.Deals = append(req.Imp[i].PMP.Deals, openrtb.Deal{ID: id})
			existing[id] = struct{}{}
		}
	}
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applySuppressDeals(
	req *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, _ ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	if !pathOK(dec.Path, "imp", "imp[*].pmp.deals", "imp[*].pmp.deals[*]", "") {
		dec.Decision = "rejected"
		dec.Reason = "path_invalid"
		return dec
	}
	ids := m.GetIds()
	if ids == nil || len(ids.GetId()) == 0 {
		dec.Decision = "rejected"
		dec.Reason = "empty_payload"
		return dec
	}
	suppress := map[string]struct{}{}
	for _, id := range ids.GetId() {
		suppress[id] = struct{}{}
	}
	for i := range req.Imp {
		if req.Imp[i].PMP == nil {
			continue
		}
		filtered := req.Imp[i].PMP.Deals[:0]
		for _, d := range req.Imp[i].PMP.Deals {
			if _, drop := suppress[d.ID]; drop {
				continue
			}
			filtered = append(filtered, d)
		}
		req.Imp[i].PMP.Deals = filtered
	}
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applyAdjustDealFloor(
	req *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, cfg ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	// path is expected to identify a deal; we accept "imp[*].pmp.deals[*]"
	// or a more specific selector. The deal is matched by Path's last
	// non-bracket token treated as an id, or the AdjustDeal payload itself.
	ad := m.GetAdjustDeal()
	if ad == nil {
		dec.Decision = "rejected"
		dec.Reason = "missing_adjust_deal_payload"
		return dec
	}
	dealID := extractDealIDFromPath(dec.Path)
	matched := 0
	for i := range req.Imp {
		if req.Imp[i].PMP == nil {
			continue
		}
		for j := range req.Imp[i].PMP.Deals {
			if dealID != "" && req.Imp[i].PMP.Deals[j].ID != dealID {
				continue
			}
			newFloor := ad.GetBidfloor()
			// R5.5.9 clamp to publisher floor.
			if cfg.PublisherFloorLookup != nil {
				pfloor := cfg.PublisherFloorLookup(req.Imp[i].ID)
				if newFloor < pfloor {
					newFloor = pfloor
					dec.Reason = "floor_clamped"
				}
			}
			req.Imp[i].PMP.Deals[j].BidFloor = newFloor
			matched++
		}
	}
	if matched == 0 {
		dec.Decision = "rejected"
		dec.Reason = "deal_not_found"
		return dec
	}
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applyAddMetrics(
	req *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, cfg ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	if !pathOK(dec.Path, "imp", "imp[*].metric", "imp[*].metric[*]", "") {
		dec.Decision = "rejected"
		dec.Reason = "path_invalid"
		return dec
	}
	mp := m.GetMetrics()
	if mp == nil || len(mp.GetMetric()) == 0 {
		dec.Decision = "rejected"
		dec.Reason = "empty_payload"
		return dec
	}
	for i := range req.Imp {
		room := cfg.MaxMetricsPerImp - len(req.Imp[i].Metric)
		if room <= 0 {
			continue
		}
		for _, pm := range mp.GetMetric() {
			if room == 0 {
				break
			}
			req.Imp[i].Metric = append(req.Imp[i].Metric, openrtb.Metric{
				Type:   pm.GetType(),
				Value:  pm.GetValue(),
				Vendor: pm.GetVendor(),
			})
			room--
		}
	}
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applyAdjustDealMargin(
	_ *openrtb.BidRequest, _ *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, _ ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	ad := m.GetAdjustDeal()
	if ad == nil || ad.GetMargin() == nil {
		dec.Decision = "rejected"
		dec.Reason = "missing_margin_payload"
		return dec
	}
	dealID := extractDealIDFromPath(dec.Path)
	out.MarginAdjustments = append(out.MarginAdjustments, MarginAdjustment{
		DealID:          dealID,
		Value:           ad.GetMargin().GetValue(),
		CalculationType: ad.GetMargin().GetCalculationType(),
	})
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

func applyBidShade(
	_ *openrtb.BidRequest, rsp *openrtb.BidResponse,
	m *pb.Mutation, origin MutationOrigin, cfg ApplierConfig,
	out *ApplyOutput, dec ApplyDecision,
) ApplyDecision {
	if cfg.DisableShadeIntent {
		dec.Decision = "rejected"
		dec.Reason = "shade_disabled"
		return dec
	}
	ab := m.GetAdjustBid()
	if ab == nil {
		dec.Decision = "rejected"
		dec.Reason = "missing_adjust_bid_payload"
		return dec
	}
	bidID := extractBidIDFromPath(dec.Path)
	if rsp == nil {
		dec.Decision = "rejected"
		dec.Reason = "no_bid_response"
		return dec
	}
	matched := false
outer:
	for si := range rsp.SeatBid {
		for bi := range rsp.SeatBid[si].Bid {
			if bidID != "" && rsp.SeatBid[si].Bid[bi].ID != bidID {
				continue
			}
			orig := rsp.SeatBid[si].Bid[bi].Price
			newPrice := ab.GetPrice()
			// R5.5.10 — cannot raise.
			if newPrice > orig {
				dec.Decision = "rejected"
				dec.Reason = "shade_out_of_bounds"
				return dec
			}
			// R5.5.10 — cannot drop below floor fraction.
			if newPrice < orig*cfg.ShadeMinFraction {
				dec.Decision = "rejected"
				dec.Reason = "shade_out_of_bounds"
				return dec
			}
			rsp.SeatBid[si].Bid[bi].Price = newPrice
			out.BidShadingDeltas[rsp.SeatBid[si].Bid[bi].ID] = newPrice - orig
			matched = true
			break outer
		}
	}
	if !matched {
		dec.Decision = "rejected"
		dec.Reason = "bid_not_found"
		return dec
	}
	bumpAgentCount(out, origin.AgentID, dec.Intent)
	dec.Decision = "applied"
	return dec
}

// ──────────────────────────────────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────────────────────────────────

// pathOK returns true if path matches one of the allowed selectors
// case-insensitively. Empty path is always accepted (PRD R5.5.4 defensive).
func pathOK(path string, allowed ...string) bool {
	if path == "" {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(path))
	for _, a := range allowed {
		if low == strings.ToLower(a) {
			return true
		}
	}
	return false
}

// extractDealIDFromPath best-effort extracts a deal id from a path like
// "imp[*].pmp.deals[deal-123]" or "imp[0].pmp.deals[abc]". Empty path → "".
func extractDealIDFromPath(path string) string {
	return extractBracketed(path, "deals")
}

// extractBidIDFromPath best-effort extracts a bid id from a path like
// "seatbid[*].bid[bid-1]". Empty path → "" (handler treats as match-all).
func extractBidIDFromPath(path string) string {
	return extractBracketed(path, "bid")
}

func extractBracketed(path, key string) string {
	idx := strings.Index(path, key+"[")
	if idx < 0 {
		return ""
	}
	rest := path[idx+len(key)+1:]
	end := strings.Index(rest, "]")
	if end < 0 {
		return ""
	}
	tok := rest[:end]
	// skip wildcard / numeric index forms.
	if tok == "*" {
		return ""
	}
	// numeric index (e.g. "0") is also not a usable id selector — return "".
	if _, err := fmt.Sscanf(tok, "%d", new(int)); err == nil {
		return ""
	}
	return tok
}

func bumpAgentCount(out *ApplyOutput, agentID, intent string) {
	if _, ok := out.AgentMutationCount[agentID]; !ok {
		out.AgentMutationCount[agentID] = map[string]int{}
	}
	out.AgentMutationCount[agentID][intent]++
}
