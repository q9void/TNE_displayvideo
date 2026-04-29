package inbound

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dpb "github.com/thenexusengine/tne_springwire/agentic/gen/tne/v1"
)

func TestDescribeCapabilities_returnsRegistryDoc(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	h := &discoveryHandler{server: srv}

	rsp, err := h.DescribeCapabilities(ctxWithAgent("curator"), &dpb.DescribeCapabilitiesRequest{})
	require.NoError(t, err)
	assert.Equal(t, "2.0", rsp.GetVersion())
	assert.Equal(t, "9131", rsp.GetSellerId())
	assert.Equal(t, "thenexusengine.com", rsp.GetSellerDomain())
	assert.NotEmpty(t, rsp.GetEtag())

	var caps map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(rsp.GetCapabilitiesJson()), &caps))
	intents, ok := caps["intents_outbound"].([]interface{})
	require.True(t, ok, "capabilities.intents_outbound must be a JSON array")
	assert.Contains(t, intents, "ACTIVATE_SEGMENTS")
}

func TestDescribeCapabilities_authFailure(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	h := &discoveryHandler{server: srv}

	_, err := h.DescribeCapabilities(ctxWithAgent("stranger"), &dpb.DescribeCapabilitiesRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestDescribeCapabilities_etagStable(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	h := &discoveryHandler{server: srv}

	rsp1, err := h.DescribeCapabilities(ctxWithAgent("curator"), &dpb.DescribeCapabilitiesRequest{})
	require.NoError(t, err)
	rsp2, err := h.DescribeCapabilities(ctxWithAgent("curator"), &dpb.DescribeCapabilitiesRequest{})
	require.NoError(t, err)

	assert.Equal(t, rsp1.GetEtag(), rsp2.GetEtag(), "etag must be deterministic for the same registry content")
}

func TestComputeETag_lengthAndStability(t *testing.T) {
	a := computeETag([]byte(`{"foo":"bar"}`))
	b := computeETag([]byte(`{"foo":"bar"}`))
	c := computeETag([]byte(`{"foo":"baz"}`))

	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	assert.Len(t, a, 16, "16 hex chars = 8 bytes of SHA-256 prefix")
}

// Minimal smoke that the auth-light contract (DescribeCapabilities does
// NOT require Registry cross-check or per-deal authz beyond what
// DevAuthenticator does) holds. With DevAuthenticator + an empty deal set,
// Discovery still answers.
func TestDescribeCapabilities_emptyDealSetStillAnswers(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator", AuthorisedDeals: nil}})
	h := &discoveryHandler{server: srv}

	rsp, err := h.DescribeCapabilities(ctxWithAgent("curator"), &dpb.DescribeCapabilitiesRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, rsp.GetCapabilitiesJson())
}

// background-context test confirms missing metadata fails the auth gate.
func TestDescribeCapabilities_noMetadataFails(t *testing.T) {
	srv := newTestServer(t, []AgentEntry{{AgentID: "curator"}})
	h := &discoveryHandler{server: srv}

	_, err := h.DescribeCapabilities(context.Background(), &dpb.DescribeCapabilitiesRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
