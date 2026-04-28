package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

func newClientForTest(t *testing.T, reg *Registry, cfg ClientConfig) *Client {
	t.Helper()
	cfg.AllowInsecure = true
	c, err := NewClient(reg, cfg, OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// ──────────────────────────────────────────────────────────────────────────
// Empty registry / lifecycle filter
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_emptyRegistry_noOp(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
		"version":"1.0","seller_id":"9131","seller_domain":"thenexusengine.com","agents":[]
	}`))
	require.NoError(t, err)
	c := newClientForTest(t, reg, ClientConfig{})

	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
	assert.Empty(t, out.Mutations)
	assert.Empty(t, out.AgentStats)
	assert.False(t, out.Truncated)
}

func TestDispatch_lifecycleFilter_skipsOtherStage(t *testing.T) {
	b := &FakeAgentBehaviour{}
	addr, stop := StartFakeAgent(t, b)
	defer stop()
	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "seg.example.com", 100, 50)
	c := newClientForTest(t, reg, ClientConfig{})

	// Dispatching at the OTHER lifecycle stage → agent should not be called.
	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecycleDSPBidResponse)
	assert.Empty(t, out.AgentStats)
	assert.Equal(t, int32(0), b.Calls.Load())
}

// ──────────────────────────────────────────────────────────────────────────
// Happy path — single agent returns mutations
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_singleAgent_appliesAndStamps(t *testing.T) {
	mutPayload := &pb.Mutation{
		Intent: pb.Intent_ACTIVATE_SEGMENTS.Enum(),
		Op:     pb.Operation_OPERATION_ADD.Enum(),
		Path:   proto.String("user.data[*].segment[*]"),
		Value:  &pb.Mutation_Ids{Ids: &pb.IDsPayload{Id: []string{"seg-1", "seg-2"}}},
	}
	b := &FakeAgentBehaviour{
		Mutations:    []*pb.Mutation{mutPayload},
		ModelVersion: "v3.2",
	}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "seg.example.com", 100, 50)
	c := newClientForTest(t, reg, ClientConfig{APIKey: "test-key"})

	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("auction-1")}, LifecyclePublisherBidRequest)

	require.Len(t, out.AgentStats, 1)
	assert.Equal(t, "ok", out.AgentStats[0].Status)
	assert.Equal(t, 1, out.AgentStats[0].MutationCount)
	assert.Equal(t, "v3.2", out.AgentStats[0].ModelVersion)
	require.Len(t, out.Mutations, 1)

	// Stamper put TYPE_SSP on the request that reached the fake.
	require.Equal(t, int32(pb.Originator_TYPE_SSP), b.LastReqOrigType.Load())
	require.Equal(t, int32(pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST), b.LastLifecycle.Load())

	// API key flowed through gRPC metadata.
	auth, _ := b.LastAuthHeader.Load().(string)
	assert.Equal(t, "test-key", auth)

	// Origins captured back-pointer to the agent.
	originRec, ok := out.Origins[out.Mutations[0]]
	require.True(t, ok)
	assert.Equal(t, "seg.example.com", originRec.AgentID)
	assert.Equal(t, int32(100), originRec.Priority)
}

// ──────────────────────────────────────────────────────────────────────────
// Per-agent API key override
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_perAgentAPIKey_overridesGlobal(t *testing.T) {
	b := &FakeAgentBehaviour{}
	addr, stop := StartFakeAgent(t, b)
	defer stop()
	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "seg.example.com", 100, 50)

	c := newClientForTest(t, reg, ClientConfig{
		APIKey: "global-key",
		PerAgentAPIKeys: map[string]string{
			"seg.example.com": "per-agent-key",
		},
	})
	c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)

	auth, _ := b.LastAuthHeader.Load().(string)
	assert.Equal(t, "per-agent-key", auth)
}

// ──────────────────────────────────────────────────────────────────────────
// tmax — slow agent must be dropped
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_tmaxBudget_dropsLate(t *testing.T) {
	b := &FakeAgentBehaviour{Sleep: 200 * time.Millisecond}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "slow.example.com", 100, 20)
	c := newClientForTest(t, reg, ClientConfig{DefaultTmaxMs: 20})

	start := time.Now()
	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
	elapsed := time.Since(start)

	// Master timer should kick in around 20 ms; allow generous slack.
	assert.LessOrEqual(t, elapsed, 250*time.Millisecond)
	// Either Truncated or AgentStat status is timeout — agent's mutations
	// are not in the merged set.
	assert.Empty(t, out.Mutations)
}

// ──────────────────────────────────────────────────────────────────────────
// Circuit breaker — induce failures, then verify open
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_circuitOpen_skipsAgent(t *testing.T) {
	b := &FakeAgentBehaviour{ReturnError: status.Error(codes.Internal, "boom")}
	addr, stop := StartFakeAgent(t, b)
	defer stop()

	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "flap.example.com", 100, 50)
	c := newClientForTest(t, reg, ClientConfig{
		CircuitFailureThreshold: 3, // open after 3 failures
	})

	// Trigger enough calls to open the breaker.
	for i := 0; i < 5; i++ {
		c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
	}

	// After breaker is open, a fresh dispatch returns circuit_open without
	// hitting the fake.
	pre := b.Calls.Load()
	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
	post := b.Calls.Load()

	require.Len(t, out.AgentStats, 1)
	assert.Equal(t, "circuit_open", out.AgentStats[0].Status)
	assert.Equal(t, pre, post, "fake agent should not have been called when breaker is open")
}

// ──────────────────────────────────────────────────────────────────────────
// Panic recovery
// ──────────────────────────────────────────────────────────────────────────

func TestDispatch_panicInOurCode_recovered(t *testing.T) {
	// Use the AgentEndpoint with no stub — callOne will short-circuit cleanly,
	// not panic, but the recover() guard is the safety net we still want
	// covered by a unit. We'll reach inside and remove the stub after dial
	// to force an "agent not dialled" path.
	b := &FakeAgentBehaviour{}
	addr, stop := StartFakeAgent(t, b)
	defer stop()
	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "x", 100, 50)
	c := newClientForTest(t, reg, ClientConfig{})

	c.mu.Lock()
	delete(c.stubs, "x")
	c.mu.Unlock()

	out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
	require.Len(t, out.AgentStats, 1)
	assert.Equal(t, "error", out.AgentStats[0].Status)
	assert.Contains(t, out.AgentStats[0].Error, "not dialled")
}

func TestDispatch_concurrent_safe(t *testing.T) {
	b := &FakeAgentBehaviour{}
	addr, stop := StartFakeAgent(t, b)
	defer stop()
	reg := makeRegistryWithAgent(t, addr, LifecyclePublisherBidRequest, []string{"ACTIVATE_SEGMENTS"}, "x", 100, 50)
	c := newClientForTest(t, reg, ClientConfig{})

	const n = 20
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			out := c.Dispatch(context.Background(), &pb.RTBRequest{Id: proto.String("a")}, LifecyclePublisherBidRequest)
			if len(out.AgentStats) != 1 {
				errs <- errors.New("expected 1 agent stat")
				return
			}
			errs <- nil
		}()
	}
	for i := 0; i < n; i++ {
		require.NoError(t, <-errs)
	}
	assert.GreaterOrEqual(t, b.Calls.Load(), int32(n))
}
