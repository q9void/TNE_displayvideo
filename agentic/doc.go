// Package agentic implements the TNE Catalyst integration with the IAB Tech
// Lab Agentic RTB Framework (ARTF v1.0). Phase 1 ships the outbound slice:
//
//   - Client: an outbound RTBExtensionPoint gRPC client that fans bid
//     requests/responses out to extension-point agents at two lifecycle hooks
//     (LIFECYCLE_PUBLISHER_BID_REQUEST and LIFECYCLE_DSP_BID_RESPONSE), with
//     parallel dispatch, per-call tmax budgeting, and a circuit breaker per
//     agent endpoint.
//   - Applier: a mutation applier that whitelists the seven supported ARTF
//     intents (ACTIVATE_SEGMENTS, ACTIVATE_DEALS, SUPPRESS_DEALS,
//     ADJUST_DEAL_FLOOR, ADJUST_DEAL_MARGIN, BID_SHADE, ADD_METRICS) and
//     rejects unsupported ones, with deterministic ordering, conflict
//     resolution, and bound enforcement.
//   - Registry: an AgentRegistry loaded from agents.json on disk and served
//     verbatim at /.well-known/agents.json.
//   - OriginatorStamper: emits Originator{TYPE_SSP, id=<seller_id>} on every
//     outbound RTBRequest and writes the ext.aamp envelope on bid requests
//     forwarded to bidders.
//   - DeriveAgentConsent: privacy gating that hard-blocks COPPA traffic and
//     soft-filters by TCF/GPP signals.
//
// Phase 2 (out of scope for this branch) adds the inbound RTBExtensionPoint
// gRPC server, MCP transport, prebid.js adapter publication, and per-publisher
// agent overrides.
//
// See docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md
// for the full design and docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md
// for the task-level plan.
package agentic
