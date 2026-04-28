package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestDeriveAgentConsent_nilRequest(t *testing.T) {
	assert.False(t, DeriveAgentConsent(context.Background(), nil))
}

func TestDeriveAgentConsent_COPPABlocks(t *testing.T) {
	req := &openrtb.BidRequest{Regs: &openrtb.Regs{COPPA: 1}}
	// Even on a fully-permissive context, COPPA hard-blocks.
	ctx := middleware.SetPrivacyContext(context.Background(), false, true, false, "")
	assert.False(t, DeriveAgentConsent(ctx, req))
}

func TestDeriveAgentConsent_COPPABlocks_evenWithCCPAAllow(t *testing.T) {
	req := &openrtb.BidRequest{Regs: &openrtb.Regs{COPPA: 1, USPrivacy: "1YNN"}}
	ctx := middleware.SetPrivacyContext(context.Background(), false, true, false, "")
	assert.False(t, DeriveAgentConsent(ctx, req))
}

func TestDeriveAgentConsent_GDPRMissingConsentBlocks(t *testing.T) {
	req := &openrtb.BidRequest{}
	// GDPR applies, consent NOT validated → soft-block.
	ctx := middleware.SetPrivacyContext(context.Background(), true, false, false, "")
	assert.False(t, DeriveAgentConsent(ctx, req))
}

func TestDeriveAgentConsent_GDPRConsented_Allows(t *testing.T) {
	req := &openrtb.BidRequest{}
	ctx := middleware.SetPrivacyContext(context.Background(), true, true, false, "CO_CONSENT_STR")
	assert.True(t, DeriveAgentConsent(ctx, req))
}

func TestDeriveAgentConsent_CCPAOptOut_Blocks(t *testing.T) {
	req := &openrtb.BidRequest{}
	// CCPA opt-out is a soft-block.
	ctx := middleware.SetPrivacyContext(context.Background(), false, true, true, "")
	assert.False(t, DeriveAgentConsent(ctx, req))
}

func TestDeriveAgentConsent_BareContext_PermissiveDefault(t *testing.T) {
	// Tests that bypass middleware end up here. Default permissive matches
	// existing bidder-path behavior.
	req := &openrtb.BidRequest{}
	assert.True(t, DeriveAgentConsent(context.Background(), req))
}
