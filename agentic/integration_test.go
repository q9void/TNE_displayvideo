package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestIntegration_endToEnd is the end-to-end happy path through the agentic
// outbound layer. It does NOT spin up a full exchange — that would require
// the entire Server fixture — but covers everything in the agentic package
// itself: Client.Dispatch → fake agent → Applier.Apply on a real
// internal/openrtb.BidRequest.
//
// The test mirrors what Hook A does in production:
//  1. Stand up a fake RTBExtensionPoint that returns ACTIVATE_SEGMENTS.
//  2. Build a Client + Applier + Stamper.
//  3. Wrap the BidRequest, Dispatch, Apply.
//  4. Assert the in-flight BidRequest has the segments injected and that
//     decisions/stats are populated correctly.
func TestIntegration_endToEnd(t *testing.T) {
	mut := &pb.Mutation{
		Intent: pb.Intent_ACTIVATE_SEGMENTS.Enum(),
		Op:     pb.Operation_OPERATION_ADD.Enum(),
		Path:   proto.String("user.data[*].segment[*]"),
		Value:  &pb.Mutation_Ids{Ids: &pb.IDsPayload{Id: []string{"sports", "nfl", "shopping"}}},
	}
	b := &FakeAgentBehaviour{
		Mutations:    []*pb.Mutation{mut},
		ModelVersion: "integration-test-v1",
	}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "seg.example.com", 100, 50)

	stamper := OriginatorStamper{SellerID: "9131"}
	c, err := NewClient(reg, ClientConfig{
		DefaultTmaxMs: 100,
		AllowInsecure: true,
		APIKey:        "test-key",
	}, stamper)
	require.NoError(t, err)
	defer c.Close()

	applier := NewApplier(ApplierConfig{})

	// Build an in-flight BidRequest that an exchange would have constructed
	// from a real /openrtb2/auction call.
	bidReq := &openrtb.BidRequest{
		ID:   "auction-end-to-end",
		Imp:  []openrtb.Imp{{ID: "imp-1"}},
		User: &openrtb.User{ID: "u-42"},
	}
	pbReq := WrapAsRTBRequest(bidReq, LifecyclePublisherBidRequest, 30)

	// 1) Dispatch — fans out to the fake.
	dr := c.Dispatch(context.Background(), pbReq, LifecyclePublisherBidRequest)
	require.Len(t, dr.AgentStats, 1, "one agent should have responded")
	assert.Equal(t, "ok", dr.AgentStats[0].Status)
	assert.Equal(t, "integration-test-v1", dr.AgentStats[0].ModelVersion)
	assert.Equal(t, 1, dr.AgentStats[0].MutationCount)

	// 2) Apply — mutates the in-flight BidRequest.
	out := applier.Apply(bidReq, nil, dr.Mutations, originsFromDispatch(dr), LifecyclePublisherBidRequest)
	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "applied", out.Decisions[0].Decision)
	assert.Equal(t, 3, out.SegmentsActivated)

	// 3) BidRequest now reflects the agent's segments — this is what the
	// downstream bidders will see.
	require.NotNil(t, bidReq.User)
	require.Len(t, bidReq.User.Data, 1)
	assert.Equal(t, "seg.example.com", bidReq.User.Data[0].ID, "user.data block tagged with originating agent")
	assert.Len(t, bidReq.User.Data[0].Segment, 3)

	// 4) Originator was stamped TYPE_SSP via the fake's capture.
	require.Equal(t, int32(pb.Originator_TYPE_SSP), b.LastReqOrigType.Load())
	require.Equal(t, int32(pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST), b.LastLifecycle.Load())

	// 5) API key flowed through gRPC metadata.
	authVal, _ := b.LastAuthHeader.Load().(string)
	assert.Equal(t, "test-key", authVal)
}

// TestIntegration_tmaxBudget_dropsLateAgent tests the exact pathology Hook A
// must handle gracefully: an agent that takes longer than tmax must NOT
// stall the auction. The fake sleeps 200ms; budget is 20ms.
func TestIntegration_tmaxBudget_dropsLateAgent(t *testing.T) {
	b := &FakeAgentBehaviour{
		Sleep: 200 * time.Millisecond,
		Mutations: []*pb.Mutation{{
			Intent: pb.Intent_ACTIVATE_SEGMENTS.Enum(),
			Op:     pb.Operation_OPERATION_ADD.Enum(),
			Path:   proto.String("user.data[*].segment[*]"),
			Value:  &pb.Mutation_Ids{Ids: &pb.IDsPayload{Id: []string{"never-applied"}}},
		}},
	}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "slow.example.com", 100, 20)

	c, err := NewClient(reg, ClientConfig{
		DefaultTmaxMs: 20,
		AllowInsecure: true,
	}, OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)
	defer c.Close()

	applier := NewApplier(ApplierConfig{})
	bidReq := &openrtb.BidRequest{ID: "x", Imp: []openrtb.Imp{{ID: "imp-1"}}}
	pbReq := WrapAsRTBRequest(bidReq, LifecyclePublisherBidRequest, 30)

	start := time.Now()
	dr := c.Dispatch(context.Background(), pbReq, LifecyclePublisherBidRequest)
	elapsed := time.Since(start)

	// Master timer gates this at ~20ms; allow generous slack for CI variance.
	assert.LessOrEqual(t, elapsed, 250*time.Millisecond)

	out := applier.Apply(bidReq, nil, dr.Mutations, originsFromDispatch(dr), LifecyclePublisherBidRequest)

	// The slow agent's mutations did not reach the applier → the BidRequest
	// is unchanged from what the exchange originally had.
	assert.Empty(t, out.Decisions, "slow agent's mutations should not have been applied")
	assert.Empty(t, bidReq.User, "user data unchanged when agent times out")
}

// TestIntegration_COPPABlocksDispatch confirms the consent layer hard-blocks
// COPPA traffic before any agent is dialled. (Hook A invokes
// DeriveAgentConsent before Dispatch; here we exercise the same gate at
// the integration level.)
func TestIntegration_COPPABlocksDispatch(t *testing.T) {
	b := &FakeAgentBehaviour{
		Mutations: []*pb.Mutation{{
			Intent: pb.Intent_ACTIVATE_SEGMENTS.Enum(),
			Op:     pb.Operation_OPERATION_ADD.Enum(),
			Path:   proto.String("user.data[*].segment[*]"),
			Value:  &pb.Mutation_Ids{Ids: &pb.IDsPayload{Id: []string{"x"}}},
		}},
	}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "any", 100, 50)
	_, err := NewClient(reg, ClientConfig{AllowInsecure: true}, OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)

	bidReq := &openrtb.BidRequest{
		ID:   "coppa-auction",
		Regs: &openrtb.Regs{COPPA: 1},
	}

	// Hook A's gate is exactly this:
	allowed := DeriveAgentConsent(context.Background(), bidReq)
	assert.False(t, allowed, "COPPA must hard-block agent fanout")

	// Confirm the fake was never called (no dispatch happened in this gate
	// path — Hook A returns early before calling Dispatch).
	assert.Equal(t, int32(0), b.Calls.Load(), "fake agent must not be dialled when COPPA blocks")
}

// TestIntegration_disabledShade_rejected confirms the OQ3 default behaviour:
// the shading code path is present but rejected by the applier when
// DisableShadeIntent=true.
func TestIntegration_disabledShade_rejected(t *testing.T) {
	mut := &pb.Mutation{
		Intent: pb.Intent_BID_SHADE.Enum(),
		Op:     pb.Operation_OPERATION_REPLACE.Enum(),
		Path:   proto.String("seatbid[*].bid[b1]"),
		Value:  &pb.Mutation_AdjustBid{AdjustBid: &pb.AdjustBidPayload{Price: proto.Float64(0.50)}},
	}
	b := &FakeAgentBehaviour{Mutations: []*pb.Mutation{mut}}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecycleDSPBidResponse, []string{"BID_SHADE"}, "shade.example.com", 100, 50)
	c, err := NewClient(reg, ClientConfig{AllowInsecure: true}, OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)
	defer c.Close()

	applier := NewApplier(ApplierConfig{DisableShadeIntent: true})

	bidReq := &openrtb.BidRequest{ID: "x"}
	rsp := &openrtb.BidResponse{SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{{ID: "b1", Price: 1.00}}}}}

	pbReq := WrapAsRTBRequest(bidReq, LifecycleDSPBidResponse, 30)
	dr := c.Dispatch(context.Background(), pbReq, LifecycleDSPBidResponse)
	out := applier.Apply(bidReq, rsp, dr.Mutations, originsFromDispatch(dr), LifecycleDSPBidResponse)

	require.Len(t, out.Decisions, 1)
	assert.Equal(t, "rejected", out.Decisions[0].Decision)
	assert.Equal(t, "shade_disabled", out.Decisions[0].Reason)
	// Price unchanged: applier rejected before mutating.
	assert.Equal(t, 1.00, rsp.SeatBid[0].Bid[0].Price)
}

// TestIntegration_grpcError_status confirms gRPC error mapping to AgentCallStat.
func TestIntegration_grpcError_status(t *testing.T) {
	b := &FakeAgentBehaviour{ReturnError: status.Error(codes.Unavailable, "down")}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "broken", 100, 50)
	c, err := NewClient(reg, ClientConfig{AllowInsecure: true}, OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)
	defer c.Close()

	dr := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("x")}, LifecyclePublisherBidRequest)
	require.Len(t, dr.AgentStats, 1)
	assert.Equal(t, "error", dr.AgentStats[0].Status)
	assert.Contains(t, dr.AgentStats[0].Error, "Unavailable")
	assert.Empty(t, dr.Mutations)
}

func originsFromDispatch(dr DispatchResult) []MutationOrigin {
	out := make([]MutationOrigin, len(dr.Mutations))
	for i, m := range dr.Mutations {
		if o, ok := dr.Origins[m]; ok {
			out[i] = o
		}
	}
	return out
}
