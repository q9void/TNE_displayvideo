// Package inbound implements the AAMP 2.0 inbound surface for TNE Catalyst:
// a gRPC server on :50051 that accepts mutations from authenticated buyer
// agents (curators) and discovery queries from the wider AAMP-2.0 ecosystem.
//
// Phase 2A scope:
//   - Server: lifecycle (Start/Stop), gRPC service registration
//   - Authenticator: pluggable interface; DevAuthenticator for no-mTLS
//     development; MTLSAuthenticator deferred to Phase 2A.1
//   - RateLimiter: per-agent + per-publisher QPS caps with circuit breaker
//   - GetMutations handler (RTBExtensionPoint service): reuses the Phase 1
//     agentic.Applier in inverse direction
//   - DescribeCapabilities handler (Discovery service): serves the v2
//     agents.json capabilities block over gRPC for buyer-agent introspection
//
// Out of scope for Phase 2A:
//   - OpenDirect deal-creation RPCs (Phase 2B)
//   - IAB Tools Portal Registry self-registration (Phase 2C)
//   - Curator SDK (Phase 2C)
//   - Full mTLS authenticator (Phase 2A.1; ships DevAuthenticator only)
//
// See docs/superpowers/specs/2026-04-28-aamp-2-phase-2-curator-revenue-design.md
// for the full design and docs/superpowers/plans/2026-04-28-aamp-2a-inbound.md
// for the task-level plan.
package inbound
