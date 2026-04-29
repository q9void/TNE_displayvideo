package inbound

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/thenexusengine/tne_springwire/agentic"
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	dpb "github.com/thenexusengine/tne_springwire/agentic/gen/tne/v1"
)

// startTestServer spins up a real grpc.Server bound to 127.0.0.1:0 with
// both inbound services registered. Mirrors what cmd/server's
// initAgenticInbound() will do in production. Returns the resolved address
// and a stop closure the caller MUST defer.
func startTestServer(t *testing.T, allowList []AgentEntry) (string, func()) {
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

	srv, err := NewServer(ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: true,
		QPSPerAgent:    1000,
	}, agentic.NewApplier(agentic.ApplierConfig{}), reg, NewDevAuthenticator(allowList), agentic.OriginatorStamper{SellerID: "9131"})
	require.NoError(t, err)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv.grpcServer = grpc.NewServer(grpc.MaxRecvMsgSize(srv.cfg.MaxRecvMsgBytes))
	pb.RegisterRTBExtensionPointServer(srv.grpcServer, srv)
	dpb.RegisterDiscoveryServer(srv.grpcServer, &discoveryHandler{server: srv})

	go func() { _ = srv.grpcServer.Serve(lis) }()
	srv.started.Store(true)
	srv.listener = lis

	stop := func() {
		done := make(chan struct{})
		go func() { srv.grpcServer.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			srv.grpcServer.Stop()
		}
	}
	return lis.Addr().String(), stop
}

// dialTestClient builds a plain grpc.ClientConn pointing at the test
// server. AAMP wire format is plain gRPC; Phase 2A skips TLS in dev mode.
func dialTestClient(t *testing.T, addr, agentID string) (*grpc.ClientConn, context.Context) {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-aamp-agent-id", agentID)
	return conn, ctx
}

// TestIntegration_endToEnd_GetMutations is the inverse of Phase 1's
// TestIntegration_endToEnd: a curator-side gRPC client dials our inbound
// server, sends an RTBRequest, and receives a Phase 2A no-op response
// with the correct envelope.
func TestIntegration_endToEnd_GetMutations(t *testing.T) {
	addr, stop := startTestServer(t, []AgentEntry{
		{AgentID: "curator.example.com", AgentType: "DSP"},
	})
	defer stop()

	conn, ctx := dialTestClient(t, addr, "curator.example.com")
	client := pb.NewRTBExtensionPointClient(conn)

	dsp := pb.Originator_TYPE_DSP
	lc := pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST
	req := &pb.RTBRequest{
		Id:         proto.String("auction-integration"),
		Originator: &pb.Originator{Type: &dsp, Id: proto.String("curator.example.com")},
		Lifecycle:  &lc,
		ApplicableIntents: []pb.Intent{pb.Intent_ACTIVATE_SEGMENTS},
	}

	rsp, err := client.GetMutations(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	assert.Equal(t, "auction-integration", rsp.GetId())
	assert.Empty(t, rsp.GetMutations(), "Phase 2A is a no-op responder")
	require.NotNil(t, rsp.Metadata)
	assert.Equal(t, "1.0", rsp.Metadata.GetApiVersion())
	assert.Equal(t, "phase-2a-noop", rsp.Metadata.GetModelVersion())
}

// TestIntegration_endToEnd_DescribeCapabilities mirrors the Phase 1
// integration test for outbound — proves the wire works end-to-end and
// the registry payload round-trips correctly.
func TestIntegration_endToEnd_DescribeCapabilities(t *testing.T) {
	addr, stop := startTestServer(t, []AgentEntry{
		{AgentID: "curator.example.com", AgentType: "DSP"},
	})
	defer stop()

	conn, ctx := dialTestClient(t, addr, "curator.example.com")
	client := dpb.NewDiscoveryClient(conn)

	rsp, err := client.DescribeCapabilities(ctx, &dpb.DescribeCapabilitiesRequest{})
	require.NoError(t, err)
	assert.Equal(t, "2.0", rsp.GetVersion())
	assert.Equal(t, "9131", rsp.GetSellerId())
	assert.Equal(t, "thenexusengine.com", rsp.GetSellerDomain())
	assert.NotEmpty(t, rsp.GetEtag())

	// Capabilities JSON should round-trip without modification.
	var caps map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(rsp.GetCapabilitiesJson()), &caps))
	intents := caps["intents_outbound"].([]interface{})
	assert.Contains(t, intents, "ACTIVATE_SEGMENTS")
}

// TestIntegration_unauthenticated_caller is the negative-path mirror.
// A real gRPC client missing the x-aamp-agent-id header gets
// Unauthenticated end-to-end without leaking which check failed.
func TestIntegration_unauthenticated_caller(t *testing.T) {
	addr, stop := startTestServer(t, []AgentEntry{
		{AgentID: "curator.example.com"},
	})
	defer stop()

	conn, _ := dialTestClient(t, addr, "stranger") // not in allow-list
	client := pb.NewRTBExtensionPointClient(conn)

	_, err := client.GetMutations(metadata.AppendToOutgoingContext(
		context.Background(), "x-aamp-agent-id", "stranger"),
		&pb.RTBRequest{Id: proto.String("a")})
	require.Error(t, err)
	// Assertion on gRPC status code is in handler unit tests; here we just
	// confirm the wire round-trip surfaces an error.
}
