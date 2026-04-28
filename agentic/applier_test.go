package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	openrtbpb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/openrtb/v26"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// helper builders
func mut(intent pb.Intent, op pb.Operation, path string, value any) *pb.Mutation {
	m := &pb.Mutation{
		Intent: intent.Enum(),
		Op:     op.Enum(),
		Path:   proto.String(path),
	}
	switch v := value.(type) {
	case *pb.IDsPayload:
		m.Value = &pb.Mutation_Ids{Ids: v}
	case *pb.AdjustDealPayload:
		m.Value = &pb.Mutation_AdjustDeal{AdjustDeal: v}
	case *pb.AdjustBidPayload:
		m.Value = &pb.Mutation_AdjustBid{AdjustBid: v}
	case *pb.MetricsPayload:
		m.Value = &pb.Mutation_Metrics{Metrics: v}
	}
	return m
}

func origin(id string, prio int32) MutationOrigin { return MutationOrigin{AgentID: id, Priority: prio} }

// ──────────────────────────────────────────────────────────────────────────
// ACTIVATE_SEGMENTS
// ──────────────────────────────────────────────────────────────────────────

func TestApply_activateSegments_appendsAndDedupes(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{}
	muts := []*pb.Mutation{
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]",
			&pb.IDsPayload{Id: []string{"seg-1", "seg-2", "seg-2"}}),
	}
	out := a.Apply(req, nil, muts, []MutationOrigin{origin("seg.example.com", 100)},
		LifecyclePublisherBidRequest)

	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	require.Len(t, req.User.Data, 1)
	assert.Equal(t, "seg.example.com", req.User.Data[0].ID)
	assert.Len(t, req.User.Data[0].Segment, 2) // dedupe within payload
	assert.Equal(t, 2, out.SegmentsActivated)
}

func TestApply_activateSegments_invalidPath(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "regs.coppa",
			&pb.IDsPayload{Id: []string{"x"}}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "path_invalid", out.Decisions[0].Reason)
}

// ──────────────────────────────────────────────────────────────────────────
// ACTIVATE_DEALS / SUPPRESS_DEALS
// ──────────────────────────────────────────────────────────────────────────

func TestApply_activateDeals_addsToAllImps(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{Imp: []openrtb.Imp{{ID: "imp1"}, {ID: "imp2"}}}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ACTIVATE_DEALS, pb.Operation_OPERATION_ADD, "imp[*].pmp.deals",
			&pb.IDsPayload{Id: []string{"deal-1", "deal-2"}}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	assert.Len(t, req.Imp[0].PMP.Deals, 2)
	assert.Len(t, req.Imp[1].PMP.Deals, 2)
}

func TestApply_suppressDeals_removesByID(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{
		Imp: []openrtb.Imp{{ID: "imp1", PMP: &openrtb.PMP{Deals: []openrtb.Deal{
			{ID: "keep"}, {ID: "drop"}, {ID: "also-keep"},
		}}}},
	}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_SUPPRESS_DEALS, pb.Operation_OPERATION_REMOVE, "imp[*].pmp.deals",
			&pb.IDsPayload{Id: []string{"drop"}}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	require.Len(t, req.Imp[0].PMP.Deals, 2)
	assert.Equal(t, "keep", req.Imp[0].PMP.Deals[0].ID)
	assert.Equal(t, "also-keep", req.Imp[0].PMP.Deals[1].ID)
}

// ──────────────────────────────────────────────────────────────────────────
// ADJUST_DEAL_FLOOR
// ──────────────────────────────────────────────────────────────────────────

func TestApply_adjustDealFloor_clampsToPublisherFloor(t *testing.T) {
	a := NewApplier(ApplierConfig{
		PublisherFloorLookup: func(impID string) float64 { return 2.50 },
	})
	req := &openrtb.BidRequest{
		Imp: []openrtb.Imp{{ID: "imp1", PMP: &openrtb.PMP{Deals: []openrtb.Deal{
			{ID: "deal-X", BidFloor: 1.00},
		}}}},
	}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ADJUST_DEAL_FLOOR, pb.Operation_OPERATION_REPLACE, "imp[*].pmp.deals[deal-X]",
			&pb.AdjustDealPayload{Bidfloor: proto.Float64(0.50)}),
	}, []MutationOrigin{origin("floor.example.com", 0)}, LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	assert.Equal(t, "floor_clamped", out.Decisions[0].Reason)
	assert.Equal(t, 2.50, req.Imp[0].PMP.Deals[0].BidFloor)
}

func TestApply_adjustDealFloor_dealNotFound(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{Imp: []openrtb.Imp{{ID: "imp1", PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "other"}}}}}}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ADJUST_DEAL_FLOOR, pb.Operation_OPERATION_REPLACE, "imp[*].pmp.deals[missing]",
			&pb.AdjustDealPayload{Bidfloor: proto.Float64(3.00)}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "deal_not_found", out.Decisions[0].Reason)
}

// ──────────────────────────────────────────────────────────────────────────
// BID_SHADE
// ──────────────────────────────────────────────────────────────────────────

func TestApply_bidShade_disabledByConfig(t *testing.T) {
	a := NewApplier(ApplierConfig{DisableShadeIntent: true})
	req := &openrtb.BidRequest{}
	rsp := &openrtb.BidResponse{SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{{ID: "b1", Price: 1.00}}}}}
	out := a.Apply(req, rsp, []*pb.Mutation{
		mut(pb.Intent_BID_SHADE, pb.Operation_OPERATION_REPLACE, "seatbid[*].bid[b1]",
			&pb.AdjustBidPayload{Price: proto.Float64(0.80)}),
	}, []MutationOrigin{origin("a", 0)}, LifecycleDSPBidResponse)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "shade_disabled", out.Decisions[0].Reason)
	assert.Equal(t, 1.00, rsp.SeatBid[0].Bid[0].Price)
}

func TestApply_bidShade_rejectsRaise(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{}
	rsp := &openrtb.BidResponse{SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{{ID: "b1", Price: 1.00}}}}}
	out := a.Apply(req, rsp, []*pb.Mutation{
		mut(pb.Intent_BID_SHADE, pb.Operation_OPERATION_REPLACE, "seatbid[*].bid[b1]",
			&pb.AdjustBidPayload{Price: proto.Float64(1.50)}),
	}, []MutationOrigin{origin("a", 0)}, LifecycleDSPBidResponse)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "shade_out_of_bounds", out.Decisions[0].Reason)
	assert.Equal(t, 1.00, rsp.SeatBid[0].Bid[0].Price, "price unchanged after rejection")
}

func TestApply_bidShade_rejectsBelowFloorFraction(t *testing.T) {
	a := NewApplier(ApplierConfig{ShadeMinFraction: 0.5})
	rsp := &openrtb.BidResponse{SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{{ID: "b1", Price: 1.00}}}}}
	out := a.Apply(&openrtb.BidRequest{}, rsp, []*pb.Mutation{
		mut(pb.Intent_BID_SHADE, pb.Operation_OPERATION_REPLACE, "seatbid[*].bid[b1]",
			&pb.AdjustBidPayload{Price: proto.Float64(0.20)}),
	}, []MutationOrigin{origin("a", 0)}, LifecycleDSPBidResponse)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "shade_out_of_bounds", out.Decisions[0].Reason)
}

func TestApply_bidShade_appliesWithinBounds(t *testing.T) {
	a := NewApplier(ApplierConfig{ShadeMinFraction: 0.5})
	rsp := &openrtb.BidResponse{SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{{ID: "b1", Price: 1.00}}}}}
	out := a.Apply(&openrtb.BidRequest{}, rsp, []*pb.Mutation{
		mut(pb.Intent_BID_SHADE, pb.Operation_OPERATION_REPLACE, "seatbid[*].bid[b1]",
			&pb.AdjustBidPayload{Price: proto.Float64(0.80)}),
	}, []MutationOrigin{origin("a", 0)}, LifecycleDSPBidResponse)
	require.Equal(t, "applied", out.Decisions[0].Decision)
	assert.InDelta(t, 0.80, rsp.SeatBid[0].Bid[0].Price, 1e-9)
	assert.InDelta(t, -0.20, out.BidShadingDeltas["b1"], 1e-9)
}

// ──────────────────────────────────────────────────────────────────────────
// Top-level rejects
// ──────────────────────────────────────────────────────────────────────────

func TestApply_unsupportedIntent(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{}
	// Use an unknown intent value (cast through enum).
	weird := pb.Intent(9999)
	out := a.Apply(req, nil, []*pb.Mutation{
		{Intent: weird.Enum(), Op: pb.Operation_OPERATION_ADD.Enum(), Path: proto.String("")},
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "unsupported_intent", out.Decisions[0].Reason)
}

func TestApply_wrongLifecycle_rejectsBidShadeAtPreFanout(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	out := a.Apply(&openrtb.BidRequest{}, nil, []*pb.Mutation{
		mut(pb.Intent_BID_SHADE, pb.Operation_OPERATION_REPLACE, "seatbid[*].bid[b1]",
			&pb.AdjustBidPayload{Price: proto.Float64(0.80)}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "wrong_lifecycle", out.Decisions[0].Reason)
}

func TestApply_opUnspecified(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	out := a.Apply(&openrtb.BidRequest{}, nil, []*pb.Mutation{
		{Intent: pb.Intent_ACTIVATE_SEGMENTS.Enum(), Op: pb.Operation_OPERATION_UNSPECIFIED.Enum(), Path: proto.String("")},
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "op_unspecified", out.Decisions[0].Reason)
}

func TestApply_mutationCap(t *testing.T) {
	a := NewApplier(ApplierConfig{MaxMutationsPerResponse: 2})
	muts := []*pb.Mutation{
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]", &pb.IDsPayload{Id: []string{"a"}}),
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]", &pb.IDsPayload{Id: []string{"b"}}),
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]", &pb.IDsPayload{Id: []string{"c"}}),
	}
	origins := []MutationOrigin{origin("a", 1), origin("b", 2), origin("c", 3)}
	out := a.Apply(&openrtb.BidRequest{}, nil, muts, origins, LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 3)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	assert.Equal(t, "applied", out.Decisions[1].Decision)
	assert.Equal(t, "rejected", out.Decisions[2].Decision)
	assert.Equal(t, "mutation_cap_exceeded", out.Decisions[2].Reason)
}

func TestApply_payloadSizeCap(t *testing.T) {
	a := NewApplier(ApplierConfig{MaxIDsPerPayload: 3})
	out := a.Apply(&openrtb.BidRequest{}, nil, []*pb.Mutation{
		mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]",
			&pb.IDsPayload{Id: []string{"a", "b", "c", "d"}}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "payload_size_exceeded", out.Decisions[0].Reason)
}

// ──────────────────────────────────────────────────────────────────────────
// Determinism + conflict resolution
// ──────────────────────────────────────────────────────────────────────────

func TestApply_deterministicOrder(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	build := func() ([]*pb.Mutation, []MutationOrigin) {
		return []*pb.Mutation{
				mut(pb.Intent_ACTIVATE_SEGMENTS, pb.Operation_OPERATION_ADD, "user.data[*].segment[*]", &pb.IDsPayload{Id: []string{"x"}}),
				mut(pb.Intent_ACTIVATE_DEALS, pb.Operation_OPERATION_ADD, "imp[*].pmp.deals", &pb.IDsPayload{Id: []string{"d-1"}}),
			}, []MutationOrigin{
				origin("z", 50),
				origin("a", 100),
			}
	}
	var prev []ApplyDecision
	for i := 0; i < 5; i++ {
		req := &openrtb.BidRequest{Imp: []openrtb.Imp{{ID: "imp1"}}}
		muts, origins := build()
		out := a.Apply(req, nil, muts, origins, LifecyclePublisherBidRequest)
		if prev != nil {
			require.Len(t, out.Decisions, len(prev))
			for j := range prev {
				assert.Equal(t, prev[j].Intent, out.Decisions[j].Intent)
				assert.Equal(t, prev[j].AgentID, out.Decisions[j].AgentID)
				assert.Equal(t, prev[j].Decision, out.Decisions[j].Decision)
			}
		}
		prev = out.Decisions
	}
}

func TestApply_conflict_lastWriterWins_logsSuperseded(t *testing.T) {
	// Two REPLACE mutations on the same path; higher priority wins.
	a := NewApplier(ApplierConfig{})
	req := &openrtb.BidRequest{Imp: []openrtb.Imp{{ID: "imp1", PMP: &openrtb.PMP{Deals: []openrtb.Deal{{ID: "deal-X"}}}}}}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ADJUST_DEAL_FLOOR, pb.Operation_OPERATION_REPLACE, "imp[*].pmp.deals[deal-X]",
			&pb.AdjustDealPayload{Bidfloor: proto.Float64(1.00)}),
		mut(pb.Intent_ADJUST_DEAL_FLOOR, pb.Operation_OPERATION_REPLACE, "imp[*].pmp.deals[deal-X]",
			&pb.AdjustDealPayload{Bidfloor: proto.Float64(2.50)}),
	}, []MutationOrigin{
		origin("low-prio.example.com", 100),
		origin("high-prio.example.com", 500),
	}, LifecyclePublisherBidRequest)

	require.Len(t, out.Decisions, 2)
	// Lower-priority sorts first → applied first → then superseded by higher.
	assert.Equal(t, "low-prio.example.com", out.Decisions[0].AgentID)
	assert.Equal(t, "superseded", out.Decisions[0].Decision)
	assert.Equal(t, "high-prio.example.com", out.Decisions[0].SupersededBy)
	assert.Equal(t, "high-prio.example.com", out.Decisions[1].AgentID)
	assert.Equal(t, "applied", out.Decisions[1].Decision)
	// Final value reflects winner.
	assert.Equal(t, 2.50, req.Imp[0].PMP.Deals[0].BidFloor)
}

// ──────────────────────────────────────────────────────────────────────────
// ADJUST_DEAL_MARGIN / ADD_METRICS quick checks
// ──────────────────────────────────────────────────────────────────────────

func TestApply_adjustDealMargin_recordsSideChannel(t *testing.T) {
	a := NewApplier(ApplierConfig{})
	rsp := &openrtb.BidResponse{}
	out := a.Apply(&openrtb.BidRequest{}, rsp, []*pb.Mutation{
		mut(pb.Intent_ADJUST_DEAL_MARGIN, pb.Operation_OPERATION_REPLACE, "imp[*].pmp.deals[deal-X]",
			&pb.AdjustDealPayload{Margin: &pb.Margin{Value: proto.Float64(15.0), CalculationType: pb.Margin_PERCENT.Enum()}}),
	}, []MutationOrigin{origin("a", 0)}, LifecycleDSPBidResponse)
	require.Equal(t, "applied", out.Decisions[0].Decision)
	require.Len(t, out.MarginAdjustments, 1)
	assert.Equal(t, "deal-X", out.MarginAdjustments[0].DealID)
	assert.InDelta(t, 15.0, out.MarginAdjustments[0].Value, 1e-9)
	assert.Equal(t, pb.Margin_PERCENT, out.MarginAdjustments[0].CalculationType)
}

func TestApply_addMetrics_appendsToImps(t *testing.T) {
	a := NewApplier(ApplierConfig{MaxMetricsPerImp: 5})
	req := &openrtb.BidRequest{Imp: []openrtb.Imp{{ID: "imp1"}}}
	out := a.Apply(req, nil, []*pb.Mutation{
		mut(pb.Intent_ADD_METRICS, pb.Operation_OPERATION_ADD, "imp[*].metric",
			&pb.MetricsPayload{Metric: []*openrtbpb.BidRequest_Imp_Metric{
				{Type: proto.String("viewability"), Value: proto.Float64(0.85), Vendor: proto.String("ias")},
			}}),
	}, []MutationOrigin{origin("a", 0)}, LifecyclePublisherBidRequest)
	require.Equal(t, "applied", out.Decisions[0].Decision)
	require.Len(t, req.Imp[0].Metric, 1)
	assert.Equal(t, "viewability", req.Imp[0].Metric[0].Type)
	assert.Equal(t, 0.85, req.Imp[0].Metric[0].Value)
}
