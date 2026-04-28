// Package adcp implements the TNE Catalyst integration with the Ad Context
// Protocol (https://adcontextprotocol.org), Scope3's open agentic protocol
// for advertising. AdCP is a peer to the IAB ARTF integration in the
// sibling package agentic/: both surface "agentic" capabilities to the
// auction, but they speak different protocols and model different
// capabilities.
//
//   - ARTF (agentic/) is gRPC + protobuf, single-shot, mutation-oriented:
//     agents return discrete ORTB mutations the applier whitelists.
//   - AdCP (agentic/adcp/) is MCP/HTTP + JSON, capability-oriented:
//     agents expose verbs like get_signals, get_products, activate_signal,
//     and create_media_buy that the server queries pre- or post-auction.
//
// Phase 1 — scaffold, default-off. This package ships:
//
//   - AgentRegistry loaded from adcp_agents.json (separate document from
//     ARTF agents.json so the two integrations can be onboarded
//     independently).
//   - HTTP/JSON client skeleton with per-call tmax + circuit breaker,
//     mirroring agentic.Client. Actual MCP framing lands in a follow-up;
//     Phase 1 returns ErrNotImplemented from Call so the wiring can be
//     stage-flipped without integrating a real agent.
//   - Envelope helper that emits ext.adcp on outbound bid requests when
//     the feature is enabled, symmetric with agentic/envelope.go.
//   - Feature flag plumbed through ADCP_* env vars.
//
// Phase 2 (out of scope this branch):
//
//   - MCP/JSON-RPC transport layer (real RPC framing over HTTP/SSE).
//   - Signals applier that maps AdCP get_signals responses onto
//     openrtb.User.Data segments and ext.adcp.signalsActivated.
//   - Sales-agent (get_products / create_media_buy) flow for direct deals.
//   - Per-publisher overrides.
//
// See agentic/adcp/README.md for onboarding and ADCP_* env var docs.
package adcp
