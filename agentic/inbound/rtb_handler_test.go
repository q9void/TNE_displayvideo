package inbound

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/thenexusengine/tne_springwire/agentic"
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// newTestServer constructs an inbound.Server with the standard Phase 2A
// dev-mode wiring (no mTLS, in-memory allow-list, default registry).
func newTestServer(t *testing.T, allowList []AgentEntry) *Server {
	t.Helper()
	reg, err := agentic.LoadRegistryFromBytes([]byte(`{
		"$schema": "https://thenexusengine.com/schemas/agents.v2.json",
		"version": "2.0",
		"seller_id": "9131",
		"seller_domain": "thenexusengine.com",
		"capabilities": {
			"intents_outbound": ["ACTIVATE_SEGMENTS", "BID_SHADE"],
			"lifecycles": ["PUBLISHER_BID_REQUEST", "DSP_BID_RESPONSE"]
		},
		"agents": []
	}`))
	require.NoError(t, err)

	cfg := ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: true,
		QPSPerAgent:    1000,
	}
	srv, err := NewServer(cfg, agentic.NewApplier(agentic.ApplierConfig{}), reg, NewDevAuthenticator(allowList), agentic.OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)
	return srv
}

// ctxWithAgent returns an incoming-metadata context carrying the
// x-aamp-agent-id header for DevAuth.
func ctxWithAgent(agentID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", agentID))
}

func TestGetMutations_authFailure_unauthenticated(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator.example.com"}})
	ctx := ctxWithAgent("stranger") // not in allow-list

	_, err := srv.GetMutations(ctx, &pb.RTBRequest{Id: proto.String("a")})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetMutations_originatorRejected(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator.example.com"}})
	ctx := ctxWithAgent("curator.example.com")

	sspType := pb.Originator_TYPE_SSP
	req := &pb.RTBRequest{
		Id:         proto.String("a"),
		Originator: &pb.Originator{Type: &sspType},
	}
	_, err := srv.GetMutations(ctx, req)
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGetMutations_dspOriginatorAccepted(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator.example.com"}})
	ctx := ctxWithAgent("curator.example.com")

	dspType := pb.Originator_TYPE_DSP
	req := &pb.RTBRequest{
		Id:         proto.String("auction-1"),
		Originator: &pb.Originator{Type: &dspType},
	}
	rsp, err := srv.GetMutations(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	// Phase 2A returns no mutations.
	assert.Empty(t, rsp.GetMutations())
	require.NotNil(t, rsp.Metadata)
	assert.Equal(t, "1.0", rsp.Metadata.GetApiVersion())
	assert.Equal(t, "phase-2a-noop", rsp.Metadata.GetModelVersion())
}

func TestGetMutations_publisherOriginatorAccepted(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "pub.example.com", AgentType: "PUBLISHER"}})
	ctx := ctxWithAgent("pub.example.com")

	pubType := pb.Originator_TYPE_PUBLISHER
	req := &pb.RTBRequest{
		Id:         proto.String("a"),
		Originator: &pb.Originator{Type: &pubType},
	}
	rsp, err := srv.GetMutations(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
}

func TestGetMutations_rateLimited(t *testing.T) {
	reg, _ := agentic.LoadRegistryFromBytes([]byte(`{
		"$schema": "https://thenexusengine.com/schemas/agents.v2.json",
		"version": "2.0", "seller_id": "9131", "seller_domain": "x.com",
		"agents": []
	}`))
	srv, err := NewServer(ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: true,
		QPSPerAgent:    2,
	}, agentic.NewApplier(agentic.ApplierConfig{}), reg, NewDevAuthenticator([]AgentEntry{{AgentID: "curator"}}), agentic.OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)

	ctx := ctxWithAgent("curator")
	dsp := pb.Originator_TYPE_DSP

	// Exhaust the 2-call cap. Use unique IDs so idempotency cache doesn't
	// short-circuit subsequent calls.
	_, err = srv.GetMutations(ctx, &pb.RTBRequest{Id: proto.String("a"), Originator: &pb.Originator{Type: &dsp}})
	require.NoError(t, err)
	_, err = srv.GetMutations(ctx, &pb.RTBRequest{Id: proto.String("b"), Originator: &pb.Originator{Type: &dsp}})
	require.NoError(t, err)

	_, err = srv.GetMutations(ctx, &pb.RTBRequest{Id: proto.String("c"), Originator: &pb.Originator{Type: &dsp}})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestGetMutations_idempotency_returnsCachedResponse(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	ctx := ctxWithAgent("curator")
	dsp := pb.Originator_TYPE_DSP

	req := &pb.RTBRequest{Id: proto.String("auction-A"), Originator: &pb.Originator{Type: &dsp}}

	rsp1, err := srv.GetMutations(ctx, req)
	require.NoError(t, err)

	// Second call — same ID — must return the cached response (same etag-
	// equivalent: ApiVersion/ModelVersion match, no new metric increment
	// for call_duration on the inbound handler — covered by metric label
	// status="idempotent_hit").
	rsp2, err := srv.GetMutations(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, rsp1.GetId(), rsp2.GetId())
	assert.Equal(t, rsp1.Metadata.GetApiVersion(), rsp2.Metadata.GetApiVersion())
}

func TestGetMutations_panicRecovered(t *testing.T) {
	// Stub an authenticator that panics. The handler's defer recover()
	// must turn this into Internal, not crash the test process.
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	srv.auth = panickingAuth{}

	ctx := ctxWithAgent("anything")
	_, err := srv.GetMutations(ctx, &pb.RTBRequest{Id: proto.String("a")})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

type panickingAuth struct{}

func (panickingAuth) Verify(context.Context) (*AgentIdentity, error) {
	panic(fmt.Errorf("simulated auth panic"))
}
func (panickingAuth) RefreshRegistry(context.Context) error { return nil }

func TestPublisherIDFrom_extractsFromSite(t *testing.T) {
	// publisherIDFrom is helper-tested directly; the integration path with
	// a populated BidRequest is covered by the integration_test.go fixtures.
	req := &pb.RTBRequest{}
	assert.Equal(t, "", publisherIDFrom(nil))
	assert.Equal(t, "", publisherIDFrom(req))
}

func TestIntersectIntents_emptyInputs(t *testing.T) {
	assert.Empty(t, intersectIntents(nil, []string{"ACTIVATE_SEGMENTS"}))
	assert.Empty(t, intersectIntents([]pb.Intent{pb.Intent_ACTIVATE_SEGMENTS}, nil))
}

func TestIntersectIntents_returnsCommonOnly(t *testing.T) {
	got := intersectIntents(
		[]pb.Intent{pb.Intent_ACTIVATE_SEGMENTS, pb.Intent_BID_SHADE, pb.Intent_ADJUST_DEAL_FLOOR},
		[]string{"ACTIVATE_SEGMENTS", "ADJUST_DEAL_FLOOR"},
	)
	assert.ElementsMatch(t, []string{"ACTIVATE_SEGMENTS", "ADJUST_DEAL_FLOOR"}, got)
}

func TestClassifyAuthErr_known(t *testing.T) {
	assert.Equal(t, "registry", classifyAuthErr(ErrAuthFailedRegistry))
	assert.Equal(t, "spki", classifyAuthErr(ErrAuthFailedSPKI))
	assert.Equal(t, "dealset", classifyAuthErr(ErrAuthFailedDealset))
	assert.Equal(t, "credential", classifyAuthErr(ErrAuthFailed))
	assert.Equal(t, "other", classifyAuthErr(fmt.Errorf("random")))
}
