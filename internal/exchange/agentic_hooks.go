package exchange

import (
	"context"

	"github.com/thenexusengine/tne_springwire/agentic"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// agenticHookA fires the LIFECYCLE_PUBLISHER_BID_REQUEST extension point
// fan-out before bidder dispatch. Mutates req.BidRequest in place.
//
// No-op when agentic is disabled. All errors are logged + swallowed —
// agent failures never fail the auction.
func agenticHookA(ctx context.Context, e *Exchange, req *AuctionRequest) {
	if !e.agenticEnabled || req == nil || req.BidRequest == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error().
				Interface("panic", r).
				Str("auction_id", req.BidRequest.ID).
				Msg("agentic.hookA recovered from panic")
		}
	}()

	if !agentic.DeriveAgentConsent(ctx, req.BidRequest) {
		return
	}

	pbReq := agentic.WrapAsRTBRequest(req.BidRequest, agentic.LifecyclePublisherBidRequest, 30)
	dr := e.agenticClient.Dispatch(ctx, pbReq, agentic.LifecyclePublisherBidRequest)

	muts, origins := agenticZip(dr)
	out := e.agenticApplier.Apply(req.BidRequest, nil, muts, origins,
		agentic.LifecyclePublisherBidRequest)

	logAgenticDecisions(req.BidRequest.ID, agentic.LifecyclePublisherBidRequest, out, dr.AgentStats)

	// Stamp the outbound envelope BIDDERS will see (full envelope with
	// agentsCalled). Idempotent vs WrapAsRTBRequest's stamp.
	_ = agentic.WriteOutboundEnvelope(req.BidRequest,
		e.agenticStamper.SellerID,
		agentic.LifecyclePublisherBidRequest,
		true,
		dr.AgentIDs(),
		len(out.Decisions),
	)
}

// agenticHookB fires the LIFECYCLE_DSP_BID_RESPONSE extension point fan-out
// after bid collection but before winner selection. Mutates the bid prices
// inside validBids in place via the applier writing to a synthetic
// BidResponse, which we then read back into validBids.
func agenticHookB(ctx context.Context, e *Exchange, req *AuctionRequest, validBids []ValidatedBid) {
	if !e.agenticEnabled || req == nil || req.BidRequest == nil || len(validBids) == 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error().
				Interface("panic", r).
				Str("auction_id", req.BidRequest.ID).
				Msg("agentic.hookB recovered from panic")
		}
	}()

	if !agentic.DeriveAgentConsent(ctx, req.BidRequest) {
		return
	}

	// Build a synthetic BidResponse the applier can mutate. We project
	// validBids into a single SeatBid slice, mutate prices, then read back.
	synth := buildSyntheticBidResponse(validBids)
	pbReq := agentic.WrapAsRTBRequest(req.BidRequest, agentic.LifecycleDSPBidResponse, 30)
	dr := e.agenticClient.Dispatch(ctx, pbReq, agentic.LifecycleDSPBidResponse)

	muts, origins := agenticZip(dr)
	out := e.agenticApplier.Apply(req.BidRequest, synth, muts, origins,
		agentic.LifecycleDSPBidResponse)

	logAgenticDecisions(req.BidRequest.ID, agentic.LifecycleDSPBidResponse, out, dr.AgentStats)

	// Read mutated prices back into validBids by bid.id.
	if len(out.BidShadingDeltas) > 0 {
		newPrice := map[string]float64{}
		for _, sb := range synth.SeatBid {
			for _, b := range sb.Bid {
				newPrice[b.ID] = b.Price
			}
		}
		for i := range validBids {
			if validBids[i].Bid == nil || validBids[i].Bid.Bid == nil {
				continue
			}
			if p, ok := newPrice[validBids[i].Bid.Bid.ID]; ok {
				validBids[i].Bid.Bid.Price = p
			}
		}
	}
}

// agenticZip flattens a DispatchResult's mutations into parallel slices the
// Applier expects. The applier API takes (mutations, origins) by index.
func agenticZip(dr agentic.DispatchResult) (muts []*pbMutationProxy, origins []agentic.MutationOrigin) {
	if len(dr.Mutations) == 0 {
		return nil, nil
	}
	muts = make([]*pbMutationProxy, len(dr.Mutations))
	origins = make([]agentic.MutationOrigin, len(dr.Mutations))
	for i, m := range dr.Mutations {
		muts[i] = m
		if o, ok := dr.Origins[m]; ok {
			origins[i] = o
		}
	}
	return muts, origins
}

// pbMutationProxy is a one-line type alias to keep the agentic.gen import
// out of this file's signature surface. Defined as the same underlying type
// so callers can use it without conversion.
//
// We deliberately import the generated proto via the agentic package's
// reëxport (none yet — so we keep this as a `type alias = …` comment) and
// just use the agentic type directly. See agentic_hooks_proxy.go.
type pbMutationProxy = pbMutationT

// buildSyntheticBidResponse constructs an in-memory openrtb.BidResponse
// from a slice of ValidatedBid so the applier (which mutates BidResponse)
// can shade prices. The synthetic response is throwaway — the exchange
// reads back only the new prices keyed by bid.id.
func buildSyntheticBidResponse(validBids []ValidatedBid) *openrtb.BidResponse {
	if len(validBids) == 0 {
		return &openrtb.BidResponse{}
	}
	bids := make([]openrtb.Bid, 0, len(validBids))
	for _, vb := range validBids {
		if vb.Bid == nil || vb.Bid.Bid == nil {
			continue
		}
		bids = append(bids, *vb.Bid.Bid)
	}
	return &openrtb.BidResponse{
		SeatBid: []openrtb.SeatBid{{Bid: bids}},
	}
}

// logAgenticDecisions emits per-mutation + per-call structured log lines
// per PRD §5.8.
func logAgenticDecisions(auctionID string, lc agentic.Lifecycle, out agentic.ApplyOutput, stats []agentic.AgentCallStat) {
	for _, d := range out.Decisions {
		logger.Log.Info().
			Str("evt", "agentic.mutation").
			Str("auction_id", auctionID).
			Str("lifecycle", lc.String()).
			Str("agent_id", d.AgentID).
			Str("intent", d.Intent).
			Str("op", d.Op).
			Str("path", d.Path).
			Str("decision", d.Decision).
			Str("reason", d.Reason).
			Msg("agentic mutation")
	}
	for _, s := range stats {
		logger.Log.Info().
			Str("evt", "agentic.call").
			Str("auction_id", auctionID).
			Str("lifecycle", s.LifecycleName).
			Str("agent_id", s.AgentID).
			Str("status", s.Status).
			Int64("latency_ms", s.LatencyMs).
			Int("mutation_count", s.MutationCount).
			Str("model_version", s.ModelVersion).
			Str("error", s.Error).
			Msg("agentic call")
	}
}
