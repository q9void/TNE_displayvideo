package agentic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestWriteOutboundEnvelope_includesAgentsCalled(t *testing.T) {
	req := &openrtb.BidRequest{ID: "auction-1"}
	err := WriteOutboundEnvelope(req, "9131", LifecyclePublisherBidRequest, true,
		[]string{"seg.example.com", "floor.example.com"}, 5)
	require.NoError(t, err)
	require.NotEmpty(t, req.Ext)

	// Decode and inspect.
	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(req.Ext, &top))
	require.Contains(t, top, "aamp")

	var env Envelope
	require.NoError(t, json.Unmarshal(top["aamp"], &env))

	assert.Equal(t, "1.0", env.Version)
	require.NotNil(t, env.Originator)
	assert.Equal(t, "SSP", env.Originator.Type)
	assert.Equal(t, "9131", env.Originator.ID)
	assert.Equal(t, "PUBLISHER_BID_REQUEST", env.Lifecycle)
	assert.True(t, env.AgentConsent)
	assert.ElementsMatch(t, []string{"seg.example.com", "floor.example.com"}, env.AgentsCalled)
	assert.Equal(t, 5, env.MutationsApplied)
}

func TestWriteAgentEnvelope_excludesAgentsCalled(t *testing.T) {
	req := &openrtb.BidRequest{ID: "auction-1"}
	err := WriteAgentEnvelope(req, "9131", LifecycleDSPBidResponse, true)
	require.NoError(t, err)

	env, err := ReadInboundEnvelope(req)
	require.NoError(t, err)
	require.NotNil(t, env)
	// Confidentiality: agents must NOT see who else has been dialed.
	assert.Empty(t, env.AgentsCalled, "agent envelope must not carry agentsCalled")
	assert.Equal(t, "DSP_BID_RESPONSE", env.Lifecycle)
}

func TestWriteOutboundEnvelope_preservesExistingExtKeys(t *testing.T) {
	req := &openrtb.BidRequest{
		ID:  "auction-1",
		Ext: json.RawMessage(`{"prebid": {"a": 1}, "schain": {"complete": 1}}`),
	}
	err := WriteOutboundEnvelope(req, "9131", LifecyclePublisherBidRequest, true, nil, 0)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(req.Ext, &top))
	assert.Contains(t, top, "prebid")
	assert.Contains(t, top, "schain")
	assert.Contains(t, top, "aamp")
}

func TestWriteOutboundEnvelope_nilRequest(t *testing.T) {
	// must not panic
	require.NoError(t, WriteOutboundEnvelope(nil, "9131", LifecyclePublisherBidRequest, true, nil, 0))
}

func TestEnvelope_hardCap_dropsEntireBlock(t *testing.T) {
	// Build an oversized agents-called list to bust the hard cap.
	huge := make([]string, 0, 1000)
	long := strings.Repeat("a", 50)
	for i := 0; i < 200; i++ {
		huge = append(huge, long)
	}
	req := &openrtb.BidRequest{ID: "x"}
	err := WriteOutboundEnvelope(req, "9131", LifecyclePublisherBidRequest, true, huge, 0)
	require.NoError(t, err)

	// ext should be nil OR not contain aamp.
	if req.Ext != nil {
		var top map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(req.Ext, &top))
		assert.NotContains(t, top, "aamp", "hard cap should drop aamp entirely")
	}
}

func TestReadInboundEnvelope_emptyExt(t *testing.T) {
	req := &openrtb.BidRequest{ID: "x"}
	env, err := ReadInboundEnvelope(req)
	require.NoError(t, err)
	assert.Nil(t, env)
}

func TestReadInboundEnvelope_pageSideShape(t *testing.T) {
	// What the prebid adapter writes (PRD §6.2 + §7.3).
	req := &openrtb.BidRequest{
		ID: "x",
		Ext: json.RawMessage(`{
			"aamp": {
				"version": "1.0",
				"originator": {"type": "PUBLISHER", "id": "insidetailgating.com"},
				"intentHints": ["ACTIVATE_SEGMENTS"],
				"disclosedAgents": ["seg.example.com"],
				"agentConsent": true
			}
		}`),
	}
	env, err := ReadInboundEnvelope(req)
	require.NoError(t, err)
	require.NotNil(t, env)
	assert.Equal(t, "PUBLISHER", env.Originator.Type)
	assert.Equal(t, "insidetailgating.com", env.Originator.ID)
	assert.Equal(t, []string{"ACTIVATE_SEGMENTS"}, env.IntentHints)
	assert.Equal(t, []string{"seg.example.com"}, env.DisclosedAgents)
}

func TestWriteBidExt_setsAttribution(t *testing.T) {
	bid := &openrtb.Bid{ID: "b1", Price: 1.50}
	delta := -0.12
	err := WriteBidExt(bid, []AgentApplied{
		{AgentID: "seg.example.com", Intents: []string{"ACTIVATE_SEGMENTS"}, MutationCount: 3},
	}, &delta, 12)
	require.NoError(t, err)
	require.NotEmpty(t, bid.Ext)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(bid.Ext, &top))
	require.Contains(t, top, "aamp")

	var payload BidExtAAMP
	require.NoError(t, json.Unmarshal(top["aamp"], &payload))
	require.Len(t, payload.AgentsApplied, 1)
	assert.Equal(t, "seg.example.com", payload.AgentsApplied[0].AgentID)
	assert.Equal(t, 12, payload.SegmentsActivated)
	require.NotNil(t, payload.BidShadingDelta)
	assert.InDelta(t, -0.12, *payload.BidShadingDelta, 1e-9)
}

func TestWriteBidExt_noOpWhenAllZero(t *testing.T) {
	bid := &openrtb.Bid{ID: "b1"}
	require.NoError(t, WriteBidExt(bid, nil, nil, 0))
	assert.Empty(t, bid.Ext)
}
