package agentic

import (
	"context"

	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// DeriveAgentConsent returns true iff the request permits agent processing
// by non-essential agents at the current lifecycle stage.
//
// Hard-block rules (PRD §5.9 — never call any agent, including essentials):
//   - Regs.COPPA == 1
//
// Soft-filter rules (PRD §5.9 — only "essential" agents called):
//   - Existing privacy middleware says PII collection is not allowed
//     (GDPR applies + consent not validated, OR CCPA opt-out)
//
// We deliberately reuse middleware.ShouldCollectPII rather than re-deriving
// consent semantics so the agent path stays in lock-step with the bidder
// path. Anything we'd block from a bidder we'd block from a non-essential
// agent.
//
// The caller is responsible for the soft/hard distinction at the agent
// level — DeriveAgentConsent returns the union of both decisions, and the
// Client applies essentials-only behavior in the soft case based on the
// AgentEndpoint.Essential flag.
func DeriveAgentConsent(ctx context.Context, req *openrtb.BidRequest) bool {
	if req == nil {
		return false
	}
	// Hard block: COPPA. No essentials override.
	if req.Regs != nil && req.Regs.COPPA == 1 {
		return false
	}
	// Soft check via existing privacy middleware. When the middleware
	// hasn't run (e.g. in tests using a bare context), ShouldCollectPII
	// defaults true, which is the right behavior for tests that bypass
	// the middleware path entirely.
	return middleware.ShouldCollectPII(ctx)
}
