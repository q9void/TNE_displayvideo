# IAB Agentic Protocol Integration — Design Doc (PRD)

**Date:** 2026-04-27
**Status:** DRAFT — full v1, awaiting review
**Branch:** `claude/integrate-iab-agentic-protocol-6bvtJ`
**Owner:** TBD
**Spec relations:** new — no prior spec
**Companion plan:** `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md`

> **For agentic workers:** This is the design doc / PRD. Task-level implementation checkboxes live in the companion plan file.

---

## 0. TL;DR

We're making TNE Catalyst the first SSP that **speaks IAB Tech Lab's Agentic RTB Framework (ARTF v1.0) on the wire** — Phase 1 ships outbound `RTBExtensionPoint.GetMutations` calls at two lifecycle hooks in `exchange.RunAuction`, an `Originator{type=TYPE_SSP, id=9131}` stamp on every outbound bid request, an applier that whitelists seven ARTF intents (segments, deals activate/suppress, deal-floor/margin, bid-shade, metrics), a `/.well-known/agents.json` discovery doc, full per-mutation audit + Prometheus metrics, and a tightly-budgeted gRPC client (default tmax 30 ms, parallel fan-out, circuit breaker, no retries) — all gated behind `AGENTIC_ENABLED=false` so prod ships dark and flips on per-environment. Phase 2 (out of cycle) adds the inbound seller-agent gRPC server, MCP transport, per-publisher overrides, and ships the prebid.js adapter (`tneCatalystBidAdapter@2.0.0`) that surfaces `ortb2.ext.aamp` from the page and decorates `bid.meta.aamp` for analytics. Phase 1 is two weeks, fully reversible by env-var flip, and gives commercial a real "agentic-first SSP" claim with no behaviour change for existing publishers.

---

## 1. Problem Statement & Strategic Framing

The IAB Tech Lab released the **Agentic Advertising Management Protocols (AAMP)** umbrella and an **Agentic Roadmap** on 2026-01-06, then unified all agentic work under AAMP in 2026-03 and opened the **Agent Registry** on the IAB Tools Portal on 2026-03-01. The first frozen wire spec, **Agentic RTB Framework (ARTF) v1.0**, is in public comment with a Go reference implementation. This is the first time programmatic has a standardised contract for AI agents to participate in real-time auctions.

For TNE Catalyst, the strategic question is no longer *"should we be agentic?"* but *"are we agentic before our peers, or after?"* Today our ranking and routing is hardcoded Go and an internal IDR ML service. As LLM-driven buyer agents come online over 2026, demand will increasingly arrive **as agents** (DSPs running buyer-side LLMs), and supply will increasingly **delegate decisions** (segment activation, deal eligibility, floor adjustment, bid shading) **to specialist seller-side agents**. An SSP that cannot speak ARTF cannot participate in either flow without bespoke integration per partner.

**Concrete P&L surface** ARTF's seven mutation intents map directly to revenue levers we already operate manually:

| ARTF Intent | Our equivalent today | Agentic upside |
|---|---|---|
| `ACTIVATE_SEGMENTS` / `ADD_CIDS` | Hardcoded EID promotion in `catalyst_bid_handler.go` | Per-impression segment enrichment from external specialists |
| `ACTIVATE_DEALS` / `SUPPRESS_DEALS` | Static `slot_bidder_configs` rows | Real-time deal eligibility based on context/audience |
| `ADJUST_DEAL_FLOOR` | Static `bidfloor` per imp | Dynamic floors negotiated per impression |
| `ADJUST_DEAL_MARGIN` | Fixed `BidMultiplier` per publisher | Margin tuned per impression × buyer |
| `BID_SHADE` | Not implemented | First-price shading via specialist agent |
| `ADD_METRICS` | Telemetry pipeline | Viewability/IVT signals injected pre-bid |

**Strategic posture for this cycle.** Phase 1 ships the wire compatibility (we *speak* ARTF and emit `Originator{type=TYPE_SSP}`) and the outbound extension-point client so we can *consume* third-party agents. Phase 2 exposes our own seller-agent endpoint so AI buyers can talk to us directly. Phase 3 rides whatever the IAB freezes next (Deals API agentification, agent profiles, MCP-based discovery). Done in this order, every phase is independently shippable and each one we hold delays a marketable claim.

---

## 2. Goals & Non-Goals

### 2.1 Goals (Phase 1 — this cycle)

- **G1.** Speak ARTF v1.0 on the wire: emit `Originator{type=TYPE_SSP, id=<seller_id>}` and a structured `BidRequest.ext.aamp` envelope on every outbound bid request and every extension-point call.
- **G2.** Implement an outbound `RTBExtensionPoint` gRPC client (`internal/agentic/client.go`) with strict tmax budgeting (default 30 ms), parallel fan-out to N configured agents, no retries, and circuit-breaker integration mirroring the existing bidder breaker.
- **G3.** Apply mutations safely. Honor seven intents (`ACTIVATE_SEGMENTS`, `ACTIVATE_DEALS`, `SUPPRESS_DEALS`, `ADJUST_DEAL_FLOOR`, `ADJUST_DEAL_MARGIN`, `BID_SHADE`, `ADD_METRICS`); reject anything else; cap mutations per request; log every applied/rejected decision.
- **G4.** Hook two lifecycle points in `internal/exchange/exchange.go`:
  - `LIFECYCLE_PUBLISHER_BID_REQUEST` — pre-fanout, before `callBiddersWithFPD` (line ~2302)
  - `LIFECYCLE_DSP_BID_RESPONSE` — post-fanout, before `runAuctionLogic` (line ~974)
- **G5.** Serve `/.well-known/agents.json` (and root `/agents.json`) listing the agents this SSP delegates to, mirroring the existing `sellers.json` handler.
- **G6.** Provide a config surface — global allow-list in `ServerConfig` (env-driven) plus a reserved `publishers_new.agentic_override` JSONB column for Phase 2 per-publisher control.
- **G7.** Default-off behind `AGENTIC_ENABLED=false`. When disabled the agentic code path is unreachable and cost is zero.
- **G8.** Audit every mutation: per-call structured log (`agent_id`, `intent`, `op`, `path`, `applied|rejected`, `reason`, `latency_ms`).
- **G9.** Tests: unit coverage on each intent applier; integration test that runs a full auction against an in-process fake agent server.

### 2.2 Stretch goals (this cycle if time permits)

- **S1.** Prebid.js adapter scaffold under `web/prebid-adapter/` — JS module + README + smoke tests, *not* published to the prebid.js org repo.
- **S2.** `agents.json` admin CRUD (read-only list view in the existing onboarding admin SPA).

### 2.3 Non-Goals

- **N1.** Inbound seller-agent gRPC server (Phase 2). We do not yet expose `RTBExtensionPoint` to the world.
- **N2.** MCP transport (Phase 2; production gating questions unresolved).
- **N3.** Deals API agentification (Phase 3; spec not frozen).
- **N4.** Replacing IDR ML routing. Agentic is additive — IDR continues to rank bidders.
- **N5.** Self-registering on the IAB Tools Portal Agent Registry. Manual decision; defer.
- **N6.** Cross-tenant agent marketplace, publisher-facing agent picker UI, billing for agent calls.
- **N7.** Shipping the prebid.js adapter to the official prebid.js repo — scaffold only, publishing is a separate decision.

---

## 3. Background — AAMP / ARTF v1.0 As of 2026-04

### 3.1 The umbrella

**AAMP (Agentic Advertising Management Protocols)** is the IAB Tech Lab's umbrella for agent-aware programmatic standards (announced 2026-01-06, unified under the AAMP name 2026-03). AAMP "agentifies" existing standards — OpenRTB, AdCOM, OpenDirect, VAST, Deals API — by layering machine-speed coordination via gRPC, Model Context Protocol (MCP), and Agent2Agent (A2A).

### 3.2 ARTF — the only frozen v1.0 wire spec

The **Agentic RTB Framework (ARTF) v1.0** is the only piece of AAMP with a frozen proto and a public reference implementation ([IABTechLab/agentic-rtb-framework](https://github.com/IABTechLab/agentic-rtb-framework)). Everything else in AAMP is roadmap.

**Service definition** (single RPC):

```protobuf
service RTBExtensionPoint {
  rpc GetMutations (RTBRequest) returns (RTBResponse);
}
```

**Lifecycles** (the spec defines two, with extensibility for more):

| Enum | Where in the auction |
|---|---|
| `LIFECYCLE_PUBLISHER_BID_REQUEST` | Before bidder fan-out — agents enrich/suppress/adjust the bid request |
| `LIFECYCLE_DSP_BID_RESPONSE` | After bid responses collected, before winner selection — agents adjust prices/margins |

**Originator types:** `TYPE_PUBLISHER | TYPE_SSP | TYPE_EXCHANGE | TYPE_DSP`. We will emit `TYPE_SSP` with `id=9131` (our existing schain seller ID for `thenexusengine.com`).

**Operations:** `OPERATION_ADD | OPERATION_REMOVE | OPERATION_REPLACE`.

**Intents (and the payload type each carries):**

| Intent | Payload | Effect |
|---|---|---|
| `ACTIVATE_SEGMENTS` | `IDsPayload` | Add external segment IDs to `user.data[]` / `user.ext.segtax` |
| `ACTIVATE_DEALS` | `IDsPayload` | Whitelist deal IDs on `imp.pmp.deals` |
| `SUPPRESS_DEALS` | `IDsPayload` | Remove deal IDs from `imp.pmp.deals` |
| `ADJUST_DEAL_FLOOR` | `AdjustDealPayload{bidfloor}` | Replace `imp.bidfloor` for matched deal |
| `ADJUST_DEAL_MARGIN` | `AdjustDealPayload{margin{value, calculation_type}}` | Adjust SSP take rate (CPM or PERCENT) |
| `BID_SHADE` | `AdjustBidPayload{price}` | Replace `bid.price` post-response |
| `ADD_METRICS` | `MetricsPayload{metric[]}` | Append `Metric` objects to `imp.metric[]` |
| `ADD_CIDS` | `IDsPayload` | Reserved — content IDs |

**Transports the reference impl exposes:** native gRPC on `:50051`, MCP on `:50052`, browser test UI on `:8081`.

**Status:** v1.0 **for public comment**. Field numbers in the proto reserve 1000–1999 for experimental intents and 500+ for `Ext`, but renumbering of stable fields is unlikely. Proto edition 2023.

### 3.3 What ARTF does *not* yet specify

- No spec for **agent discovery** (no equivalent to `sellers.json` for agents). Our `/.well-known/agents.json` is therefore a **TNE convention** Phase 1, with the schema ready to migrate to whatever IAB freezes.
- No spec for **agent authentication / auth** between SSP and extension-point agent. We use API key in gRPC metadata header (`x-aamp-key`) Phase 1.
- No spec for **agent-processing consent**. We will gate on a TNE-defined `ext.aamp.agentConsent` boolean derived from existing TCF/GPP signals + COPPA flag.
- No spec for **billing or rate limiting** of agent calls.

### 3.4 Why ARTF first (not Deals API or MCP)

- ARTF has a frozen proto and reference impl — implementable today.
- Deals API agentification is on the 2026 roadmap but not frozen — Phase 3.
- MCP is a transport, not a protocol — useful in Phase 2 for LLM clients reaching us in dev/test, not a production-first surface.

---

## 4. Personas & User Stories

### 4.1 Publisher Integrator (existing — header bidding)
> *"As a publisher already using prebid.js with the TNE bidder, I want my header bidding stack to keep working with no changes when TNE flips on agentic, and I want to optionally pass agent-context signals from my page if I have them."*

**Acceptance:** Existing prebid.js integrations continue without code changes. New `params.agentic` is optional and additive.

### 4.2 Buyer-Agent Operator (Phase 2)
> *"As a buyer running an LLM-driven DSP agent, I want to discover what TNE Catalyst exposes, call its `RTBExtensionPoint.GetMutations`, and receive standard mutations I can reason about."*

**Acceptance (Phase 2):** Buyer can fetch our `/.well-known/agents.json`, dial gRPC, send `RTBRequest` with `Originator{type=TYPE_DSP}`, and receive a valid `RTBResponse`. (Out of Phase 1 scope.)

### 4.3 Extension-Point Agent Vendor (Phase 1 primary user)
> *"As a third-party segment / floor / shading agent, I expose `RTBExtensionPoint.GetMutations` on a gRPC endpoint. I want TNE Catalyst to dial me on every auction within a tight tmax, send me an OpenRTB bid request with the right `Originator`, and apply the mutations I return."*

**Acceptance:** When `AGENTIC_ENABLED=true` and the vendor's URL is in the allow-list, every auction dials them at the configured lifecycle hook within `AGENTIC_TMAX_MS`. Mutations they return that match the seven supported intents are applied before bidder fan-out (or before winner selection); unsupported intents are logged and dropped.

### 4.4 Internal Ops / SSP Analyst
> *"As an analyst, I want to know which agents touched which auctions, what they changed, and whether revenue went up or down."*

**Acceptance:** Per-mutation log line emitted with `agent_id`, `intent`, `op`, `path`, `applied|rejected`, `reason`, `latency_ms`. Existing telemetry pipeline (`docs/superpowers/specs/2026-03-13-bid-telemetry-ssp-audit-design.md`) ingests these for the per-auction report card.

### 4.5 Compliance / Privacy Lead
> *"As compliance, I need to know that user data does not leak to agents that shouldn't see it, that consent is honored, and that COPPA traffic is hard-walled from agent processing."*

**Acceptance:** When `coppa=1`, agent fanout is skipped (no extension-point call dispatched). When TCF/GPP withholds the relevant purpose, only allow-listed "essential" agents are called. Every outbound `RTBRequest` carries a redacted `User` matching existing privacy middleware behaviour.

### 4.6 Product / Commercial Lead
> *"As product, I want to claim 'agentic-first SSP' in our deck on day one of Phase 1 deploy, and I want a real demo agent we call out to."*

**Acceptance:** `AGENTIC_ENABLED=true` works in staging against an in-process fake agent. README + integration doc updated. One named external agent partner has dialled (Phase 2 stretch).

---

## 5. Functional Requirements — SSP Server

### 5.1 Outbound Extension-Point Client (Phase 1)

A new package `internal/agentic/` contains an `ExtensionPointClient` that dials configured agents via ARTF gRPC and returns a flat `[]Mutation` per call.

**Public interface:**

```go
// internal/agentic/client.go
package agentic

type Client interface {
    // Dispatch fans out to all agents eligible for this lifecycle stage,
    // collects mutations within tmax, and returns the merged result.
    // Never returns an error from agent failures — those are logged and dropped.
    Dispatch(ctx context.Context, req *RTBRequest, lifecycle Lifecycle) DispatchResult
}

type DispatchResult struct {
    Mutations  []*pb.Mutation
    AgentStats []AgentCallStat   // per-agent: id, latency, status, mutation_count
    Truncated  bool              // true if any agent missed tmax
}
```

**Behavioural requirements:**

- **R5.1.1 Parallel fan-out.** All eligible agents dialled concurrently in goroutines. No serialisation.
- **R5.1.2 tmax budget.** Hard cap per call: `min(AgentConfig.TmaxMs, AGENTIC_TMAX_MS, remaining auction budget − 50ms safety)`. Default 30 ms. Late responses dropped, not awaited.
- **R5.1.3 No retries.** A single attempt per agent per lifecycle. Failed calls log, drop, and fall through.
- **R5.1.4 Circuit breaker per agent.** Reuse `idr.CircuitBreaker` (already used for bidders — see `internal/exchange/exchange.go:337`). Trip at 50% failure rate over 20 calls; half-open after 30 s. When tripped, agent is skipped entirely for that window.
- **R5.1.5 No mutation when disabled.** If `AGENTIC_ENABLED=false`, `Dispatch` is never called; returns immediately if invoked defensively.
- **R5.1.6 No allocation on hot path when no agents configured.** Empty `agents` list → return zero-value `DispatchResult` without spawning goroutines.
- **R5.1.7 Pooled gRPC connections.** One persistent `grpc.ClientConn` per agent endpoint, kept warm; `keepalive.ClientParameters{Time: 30s, Timeout: 10s, PermitWithoutStream: true}`.

### 5.2 Inbound Seller-Agent Service (Phase 2 — out of cycle)

Out of Phase 1 scope. Section retained for forward-compat:
- New gRPC server listening on `:50051` exposing `RTBExtensionPoint.GetMutations`.
- TLS terminated by ingress; mTLS for verified buyer agents.
- Reuses the same `MutationApplier` and `Originator` types as outbound, in inverse direction.
- Documented **non-goal** for this cycle (N1).

### 5.3 `/.well-known/agents.json`

A new HTTP handler `internal/endpoints/agents_json.go` serves a static JSON document declaring the agents this SSP delegates to. Mirrors the `sellers.json` handler at `internal/endpoints/sellers_json.go`:

- Routes registered in `cmd/server/server.go` adjacent to the existing sellers.json registrations (currently lines ~403–406):
  - `GET /agents.json`
  - `GET /.well-known/agents.json`
- `Content-Type: application/json; charset=utf-8`
- `Cache-Control: public, max-age=3600` (1h — shorter than sellers.json since agent rosters change more often)
- CORS: `Access-Control-Allow-Origin: *`
- Document loaded from disk path `assets/agents.json` (override via `AGENTIC_AGENTS_PATH`).

**Document is the source of truth for the in-process allow-list.** On boot, `cmd/server/server.go` reads `assets/agents.json`, validates against the schema (§7.4), and constructs the `agentic.AgentRegistry`. The same file is then served unchanged.

### 5.4 OpenRTB `Originator` + AAMP Extensions

**Outbound stamping.** Every `RTBRequest` we send to an extension-point agent carries:

```go
Originator{
    Type: TYPE_SSP,
    Id:   "9131",   // schain seller_id for thenexusengine.com
}
```

`Originator.id` is read once at boot from `AGENTIC_SELLER_ID` (default `"9131"`).

**OpenRTB envelope.** When the auction's `BidRequest.ext` is forwarded out of Catalyst (to bidders **or** to extension-point agents), we attach an `aamp` block:

```json
{
  "ext": {
    "aamp": {
      "version": "1.0",
      "originator": {"type": "SSP", "id": "9131"},
      "lifecycle": "PUBLISHER_BID_REQUEST",
      "agentConsent": true,
      "agentsCalled": ["seg.example.com", "floor.example.com"]
    }
  }
}
```

Notes:
- `agentsCalled` is only populated on the way out to **bidders**, not on the way out to agents themselves (agents must not see who else has been called this auction — confidentiality).
- `agentConsent` is derived per §5.9.
- Bidders that don't understand `ext.aamp` ignore it (additive).

### 5.5 Mutation Application Engine

`internal/agentic/applier.go` exposes a single function:

```go
// Apply mutates req in-place (PUBLISHER_BID_REQUEST lifecycle)
// or rsp in-place (DSP_BID_RESPONSE lifecycle).
// Returns a slice of decision logs for telemetry.
func (a *Applier) Apply(
    req *openrtb.BidRequest,
    rsp *openrtb.BidResponse,
    mutations []*pb.Mutation,
    lifecycle Lifecycle,
) []ApplyDecision
```

**Per-intent handlers** (one Go function each, table-dispatched):

| Intent | Handler | Lifecycle | Effect |
|---|---|---|---|
| `ACTIVATE_SEGMENTS` | `applyActivateSegments` | PUBLISHER_BID_REQUEST | Append to `req.user.data[].segment[]`, dedupe by ID |
| `ACTIVATE_DEALS` | `applyActivateDeals` | PUBLISHER_BID_REQUEST | Add to `imp[*].pmp.deals` if not present |
| `SUPPRESS_DEALS` | `applySuppressDeals` | PUBLISHER_BID_REQUEST | Remove matching deal IDs from `imp[*].pmp.deals` |
| `ADJUST_DEAL_FLOOR` | `applyAdjustDealFloor` | PUBLISHER_BID_REQUEST | Replace `imp[*].pmp.deals[].bidfloor` for matched deal |
| `ADJUST_DEAL_MARGIN` | `applyAdjustDealMargin` | DSP_BID_RESPONSE | Adjust SSP take rate before winner select; write to internal `ValidatedBid.Margin`, applied by existing `applyBidMultiplier` (line ~1132) |
| `BID_SHADE` | `applyBidShade` | DSP_BID_RESPONSE | Replace `bid.price` in collected bids |
| `ADD_METRICS` | `applyAddMetrics` | PUBLISHER_BID_REQUEST | Append to `imp[*].metric[]` |

**Hard rules:**

- **R5.5.1 Whitelist only.** Unknown intents → log + reject. No "best effort" interpretation.
- **R5.5.2 Lifecycle gating.** Each intent only applies in the lifecycle(s) listed above. A `BID_SHADE` returned during PUBLISHER_BID_REQUEST is dropped with reason `wrong_lifecycle`.
- **R5.5.3 Op gating.** Only `OPERATION_ADD | OPERATION_REMOVE | OPERATION_REPLACE` honored. `OPERATION_UNSPECIFIED` → reject with reason `op_unspecified`.
- **R5.5.4 Path validation.** `Mutation.path` must match a known JSONPath-ish selector for the intent (e.g., `imp[*].pmp.deals[*]` for deal mutations). Mismatch → reject with reason `path_invalid`.
- **R5.5.5 Bounded mutations.** Per `RTBResponse` cap: 64 mutations. Over → drop the rest, log `mutation_cap_exceeded`.
- **R5.5.6 Bounded payload size.** Per mutation `ids` payload: max 256 IDs. Exceed → reject mutation, log `payload_size_exceeded`.
- **R5.5.7 Conflict resolution.** When two agents return conflicting `REPLACE` mutations on the same path, **last-writer-wins** ordered by `agent_priority` then by `agent_id` lexicographic. The lower-priority mutation is recorded as `superseded_by=<id>` rather than dropped silently.
- **R5.5.8 Deterministic ordering.** All mutations from all agents are merged into a single sorted slice before application: `(intent, agent_priority, agent_id)`. Output order must be reproducible for tests.
- **R5.5.9 No floor lowering below publisher floor.** `ADJUST_DEAL_FLOOR` with a value below the publisher's configured `bidfloor` is clamped to the publisher floor and logged `floor_clamped`.
- **R5.5.10 Bid shading bounds.** `BID_SHADE` cannot raise a bid (we are an SSP; raising would be fraud-adjacent) and cannot reduce below `bid.price * 0.5` per call. Out-of-bounds → reject with reason `shade_out_of_bounds`.

### 5.6 Per-Publisher / Global Agent Config

**Phase 1 = global allow-list.** The agent roster lives in `assets/agents.json` and is loaded once at boot. No per-publisher override at runtime.

**Reserved for Phase 2.** A nullable `agentic_override` JSONB column on `publishers_new` (added in a Phase 2 migration, not Phase 1) will allow a publisher to opt specific agents in/out:

```sql
-- Phase 2 only — DO NOT ship in Phase 1
ALTER TABLE publishers_new ADD COLUMN agentic_override JSONB DEFAULT NULL;
-- Shape: {"include": ["agent-id-1"], "exclude": ["agent-id-2"], "tmax_ms": 25}
```

In Phase 1, the `Publisher` struct (`internal/storage/publishers.go`) is **untouched**. This keeps the migration story simple.

### 5.7 MCP Transport (Phase 2 — non-goal)

Out of cycle. The reference impl exposes MCP on `:50052` so LLM clients (Claude Desktop, Claude Code) can call `extend_rtb` directly. We will not ship this in Phase 1 because:
- LLM-prompt-injection surface is not yet evaluated.
- No production buyer uses this transport in Q2 2026.
- Adding MCP requires a second listener, a tool registration manifest, and a separate auth surface.

Documented as N2.

### 5.8 Audit & Telemetry

**Per-mutation structured log line** (zerolog, level `info`):

```json
{
  "ts": "2026-04-27T03:14:15Z",
  "evt": "agentic.mutation",
  "auction_id": "abc-123",
  "lifecycle": "PUBLISHER_BID_REQUEST",
  "agent_id": "seg.example.com",
  "agent_priority": 100,
  "intent": "ACTIVATE_SEGMENTS",
  "op": "ADD",
  "path": "user.data[*].segment[*]",
  "decision": "applied",
  "reason": "",
  "latency_ms": 12,
  "model_version": "v3.2",
  "originator_type": "SSP",
  "originator_id": "9131"
}
```

**Per-call summary** (one line per agent per auction): `evt: "agentic.call"` with `agent_id`, `lifecycle`, `mutation_count`, `latency_ms`, `status: ok|timeout|circuit_open|error`.

**Per-auction rollup** integrates with the existing checkpoint logger from `2026-03-13-bid-telemetry-ssp-audit-design.md`. New checkpoint: `CP-AGENTIC-1` after pre-bid agent dispatch, `CP-AGENTIC-2` after post-bid agent dispatch.

**Prometheus metrics:**

| Metric | Type | Labels |
|---|---|---|
| `agentic_call_duration_seconds` | Histogram | `agent_id`, `lifecycle`, `status` |
| `agentic_mutation_total` | Counter | `agent_id`, `intent`, `decision` |
| `agentic_circuit_breaker_state` | Gauge | `agent_id` (0=closed, 1=half-open, 2=open) |
| `agentic_dispatch_truncated_total` | Counter | `lifecycle` |

### 5.9 Privacy & Consent

**Hard-block conditions** (no agent fanout dispatched at all):

- `regs.coppa == 1` — COPPA traffic, hard wall.
- `regs.ext.gpc == 1` AND no agent has `essential: true` flag in `agents.json`.
- TCF v2 `consent_string` denies Purpose 7 (measurement) AND no agent declares only Purpose 1 (storage) needs.

**Soft-filter** (only "essential" agents called):

- TCF Special Feature 1 (geolocation) absent → agents needing precise geo are filtered out via `agents.json` field `requires.precise_geo: true`.

**Consent envelope.** `BidRequest.ext.aamp.agentConsent` is computed from existing privacy middleware output:

```go
func DeriveAgentConsent(req *openrtb.BidRequest) bool {
    if req.Regs != nil && req.Regs.COPPA == 1 {
        return false
    }
    // Honor existing GDPR / CCPA / GPP middleware decisions
    if !privacy.AllowsThirdPartyProcessing(req) {
        return false
    }
    return true
}
```

**Data minimisation outbound.** The same redaction the existing `privacyMiddleware` applies before bidder fanout is reused before agent fanout. We do not invent new minimisation rules — agents see exactly what bidders see, no more.

**No agent-supplied PII back into the auction.** `ACTIVATE_SEGMENTS` payloads are treated as opaque IDs; we do not let an agent inject `user.id`, `user.buyeruid`, or `user.geo.lat/lon` (covered by R5.5.4 path validation).

---

## 6. Functional Requirements — Prebid.js Adapter

The TNE prebid.js adapter (`tneCatalystBidAdapter.js`) must surface AAMP-aware signals **from the publisher's page** to our SSP, and surface attribution **back to publisher analytics**, without breaking existing integrations. ARTF is server-to-server, so the adapter is **not** an ARTF participant — it is the originator-stamp source and the attribution sink.

### 6.1 New `bidder.params.agentic` block

A new optional object on bidder params:

```javascript
bids: [{
  bidder: 'tneCatalyst',
  params: {
    publisherId: 'pub-123456',
    placement: 'homepage-banner',
    agentic: {                              // all fields optional
      enabled: true,                        // default true; false = opt out per ad unit
      tmaxMs: 30,                           // suggested agent budget (server caps to AGENTIC_TMAX_MS)
      intentHints: ['ACTIVATE_SEGMENTS'],   // hints to server which intents this slot benefits from
      disclosedAgents: ['seg.example.com'], // page-side allow-list; server intersects with global
      pageContext: {                        // free-form publisher signals
        articleTopic: 'sports.nfl',
        userIntent: 'shopping',
        sessionStage: 'returning'
      }
    }
  }
}]
```

**R6.1.1** Adapter must validate the shape: drop unknown keys, coerce types, never throw.
**R6.1.2** When `agentic` is omitted, behaviour is identical to today (no `ext.aamp` written client-side; server still stamps its own `Originator{SSP}`).
**R6.1.3** When `agentic.enabled === false`, adapter writes `ortb2.ext.aamp.disabled = true` so the server hard-skips the agent path for this auction.

### 6.2 Page-Side Originator & Intent Passthrough

The adapter writes a structured envelope to `ortb2.ext.aamp` in the bid request:

```javascript
ortb2.ext.aamp = {
  version: '1.0',
  originator: {
    type: 'PUBLISHER',
    id: pubdomain || params.publisherId
  },
  pageContext: params.agentic.pageContext || undefined,
  intentHints: params.agentic.intentHints || undefined,
  disclosedAgents: params.agentic.disclosedAgents || undefined,
  consent: deriveConsent(bidRequest)   // see 6.4
};
```

**R6.2.1** `originator.id` defaults to `window.location.hostname` if `params.publisherId` is absent.
**R6.2.2** All fields under `pageContext` MUST pass JSON-serialisable check; non-serialisable values → drop with `console.warn`.
**R6.2.3** Total `ext.aamp` payload soft-capped at 4 KB. Over → drop `pageContext` first, then `intentHints`. Hard cap 8 KB — drop entire envelope.

### 6.3 Response Decoration — Agent Attribution

The server returns mutation attribution on `seatbid[].bid[].ext.aamp.agentsApplied`:

```javascript
bid.ext.aamp = {
  agentsApplied: [
    { agent_id: 'seg.example.com', intents: ['ACTIVATE_SEGMENTS'], mutation_count: 3 },
    { agent_id: 'shade.example.com', intents: ['BID_SHADE'], mutation_count: 1 }
  ],
  bidShadingDelta: -0.12,    // present iff BID_SHADE applied; original_price - final_price
  segmentsActivated: 12
};
```

**R6.3.1** Adapter copies this onto Prebid's standard `bid.meta.aamp` so analytics adapters (`gaAnalyticsAdapter`, `pubmaticAnalyticsAdapter`, etc.) can read it without bidder coupling.
**R6.3.2** When the response has no `ext.aamp`, `bid.meta.aamp` is absent — analytics adapters must defensive-check.
**R6.3.3** A new prebid event `agentMutationApplied` is fired per `agentsApplied` entry, payload `{auctionId, bidderCode, agent_id, intents, mutation_count}`.

### 6.4 Consent Surfacing

```javascript
function deriveConsent(bidRequest) {
  const tcf = bidRequest.gdprConsent;
  const gpp = bidRequest.gppConsent;
  const usp = bidRequest.uspConsent;
  return {
    agentConsent: computeAgentConsent(tcf, gpp, usp),
    coppa: bidRequest.ortb2?.regs?.coppa === 1,
    tcfPurposeFlags: tcfPurposes(tcf),  // {1: true, 7: true, ...}
    gppSid: gpp?.applicableSections     // [7, 8, 9, ...]
  };
}
```

**R6.4.1** `agentConsent` must be `false` when COPPA is set, when TCF withholds Purpose 7 (measurement), or when GPP applicable section asserts opt-out.
**R6.4.2** Adapter never sends `pageContext` when `agentConsent === false`; `originator` and `intentHints` still flow (no PII).
**R6.4.3** Logic encapsulated in `src/utils/agenticConsent.js` so it can be unit-tested without a full prebid stack.

### 6.5 Discovery Module — `agentDiscoveryRtdProvider`

A new optional **Real-Time Data (RTD) module** fetches our `/.well-known/agents.json` once per page-load and surfaces the roster to other modules (publishers can render an agent-disclosure banner, build allow-lists, etc.).

```javascript
pbjs.setConfig({
  realTimeData: {
    dataProviders: [{
      name: 'agentDiscovery',
      params: {
        endpoint: 'https://ads.thenexusengine.com/.well-known/agents.json',
        cacheMinutes: 60
      }
    }]
  }
});
```

**R6.5.1** Optional. Default behaviour: not loaded.
**R6.5.2** Cached in `localStorage` keyed by endpoint URL + a TTL.
**R6.5.3** Exposes `pbjs.getAgentRegistry()` returning the parsed JSON or `null`.
**R6.5.4** Stretch goal — not required to ship the prebid integration.

### 6.6 Backwards Compatibility

**R6.6.1** Existing `bidder.params` shape unchanged; `agentic` is purely additive.
**R6.6.2** Server returns valid responses with no `ext.aamp` for old adapter versions; adapter handles missing `bid.meta.aamp` gracefully.
**R6.6.3** Adapter version bumped to `2.0.0` (breaking on server-response shape decoration; non-breaking on request shape). Semver justification documented in adapter README.
**R6.6.4** Adapter declares Prebid module dependencies: `core@>=8.0` (for `ortb2` config) and `currency` (existing).

### 6.7 Where the code lives

**Phase 1 scaffold:** `web/prebid-adapter/` directory in this repo containing:

```
web/prebid-adapter/
  README.md
  package.json                       # private:true, no publish
  src/
    tneCatalystBidAdapter.js         # main adapter
    tneCatalystAgenticEnvelope.js    # builds ext.aamp envelope
    agenticConsent.js                # consent derivation
    agentDiscoveryRtdProvider.js     # optional RTD module
  test/
    spec/
      tneCatalystBidAdapter_spec.js
      agenticEnvelope_spec.js
      agenticConsent_spec.js
  examples/
    pbjs-config-basic.html
    pbjs-config-agentic.html
```

Not published to the prebid.js org repo this cycle. Contribution upstream is a separate decision (out of cycle, see N7).

### 6.8 Testing the adapter

**R6.8.1** Unit tests via Karma + Mocha (Prebid's standard) for envelope construction and consent derivation.
**R6.8.2** End-to-end smoke test: serve `examples/pbjs-config-agentic.html` against a dockerised Catalyst running `AGENTIC_ENABLED=true` with the in-process fake agent. Expect `bid.meta.aamp` populated.
**R6.8.3** Snapshot tests for the `ext.aamp` JSON across the matrix `{consent: yes/no} × {coppa: yes/no} × {agentic.enabled: yes/no}`.

---

## 7. Data Shapes

### 7.1 Vendored Protos

We vendor the IAB protos verbatim under `proto/iabtechlab/`:

```
proto/iabtechlab/
  README.md                                            (provenance + commit SHA)
  bidstream/mutation/v1/
    agenticrtbframework.proto                          (vendored from IABTechLab/agentic-rtb-framework)
    agenticrtbframeworkservices.proto                  (gRPC service definition)
  openrtb/v2.6/
    openrtb.proto                                      (transitive dep of ARTF)
```

**Pinning policy:**
- `proto/iabtechlab/README.md` records the upstream repo URL, commit SHA, and date pulled.
- Refresh procedure documented but manual — a script `scripts/refresh-agentic-protos.sh` is **out of scope** Phase 1.
- We do not modify the upstream protos. If we need to extend, we use the reserved `Ext` extension range (`extensions 500 to max`) in our own proto file under `proto/tne/agentic/v1/`.

**Go code generation:**
- Generated code lands in `pkg/pb/iabtechlab/...` (matching the existing `pkg/pb/` convention referenced in the upstream repo's layout).
- Generated files `*.pb.go` and `*_grpc.pb.go` are committed to the repo (no `go generate` at build time). This matches the existing repo policy (no codegen in CI).
- A `Makefile` target `make generate-protos` documents the regeneration command. Devs run it locally; CI does not.

### 7.2 Go Types (`internal/agentic/`)

```
internal/agentic/
  client.go            # ExtensionPointClient: gRPC fan-out + tmax + circuit breaker
  applier.go           # Applier: per-intent mutation handlers
  registry.go          # AgentRegistry: load+validate agents.json
  originator.go        # OriginatorStamper: sets ext.aamp on outbound BidRequests
  envelope.go          # AAMPEnvelope: helpers to read/write ext.aamp JSON on OpenRTB
  consent.go           # DeriveAgentConsent (mirrors prebid-side derivation)
  decisions.go         # ApplyDecision, AgentCallStat, DispatchResult — pure data types
  errors.go            # sentinel errors: ErrTmax, ErrCircuitOpen, ErrUnsupportedIntent…
  client_test.go
  applier_test.go
  registry_test.go
  originator_test.go
  consent_test.go
  fake_agent_test.go   # in-process gRPC fake server for tests
```

**Public API surface** (everything else is package-private):

```go
type Client struct { /* ... */ }
func NewClient(reg *Registry, cfg ClientConfig, breakers *idr.CircuitBreakerSet) *Client
func (c *Client) Dispatch(ctx context.Context, req *pb.RTBRequest, lc Lifecycle) DispatchResult
func (c *Client) Close() error

type Applier struct { /* ... */ }
func NewApplier(cfg ApplierConfig) *Applier
func (a *Applier) Apply(req *openrtb.BidRequest, rsp *openrtb.BidResponse,
                        muts []*pb.Mutation, lc Lifecycle) []ApplyDecision

type Registry struct { /* ... */ }
func LoadRegistry(path string) (*Registry, error)
func (r *Registry) AgentsForLifecycle(lc Lifecycle) []AgentEndpoint
func (r *Registry) Document() json.RawMessage  // for /.well-known/agents.json

type OriginatorStamper struct { SellerID string }
func (s OriginatorStamper) StampRequest(req *pb.RTBRequest, lc Lifecycle, consent bool)
func (s OriginatorStamper) StampOpenRTB(req *openrtb.BidRequest, lc Lifecycle, consent bool, agentsCalled []string)

type Lifecycle int
const (
    LifecyclePublisherBidRequest Lifecycle = 1
    LifecycleDSPBidResponse      Lifecycle = 2
)
```

### 7.3 OpenRTB `ext.aamp` JSON Shape

**On the bid request to bidders (after agent processing):**

```json
{
  "ext": {
    "aamp": {
      "version": "1.0",
      "originator": { "type": "SSP", "id": "9131" },
      "lifecycle": "PUBLISHER_BID_REQUEST",
      "agentConsent": true,
      "agentsCalled": ["seg.example.com", "floor.example.com"],
      "mutationsApplied": 7,
      "publisherEnvelope": {
        "originator": { "type": "PUBLISHER", "id": "insidetailgating.com" },
        "intentHints": ["ACTIVATE_SEGMENTS"]
      }
    }
  }
}
```

**On the bid request to extension-point agents** (the gRPC RTBRequest carries OpenRTB BidRequest in `bid_request` field; we set `ext.aamp` minimally — agents must not see `agentsCalled`):

```json
{
  "ext": {
    "aamp": {
      "version": "1.0",
      "originator": { "type": "SSP", "id": "9131" },
      "lifecycle": "PUBLISHER_BID_REQUEST",
      "agentConsent": true
    }
  }
}
```

**On the bid response back to publisher:**

```json
{
  "seatbid": [{
    "bid": [{
      "id": "...",
      "price": 1.38,
      "ext": {
        "aamp": {
          "agentsApplied": [
            {"agent_id": "seg.example.com", "intents": ["ACTIVATE_SEGMENTS"], "mutation_count": 3},
            {"agent_id": "shade.example.com", "intents": ["BID_SHADE"], "mutation_count": 1}
          ],
          "bidShadingDelta": -0.12,
          "segmentsActivated": 12
        }
      }
    }]
  }]
}
```

**Type tags.** All `ext.aamp` objects carry `version: "1.0"` so we can introduce schema changes without breaking older readers.

### 7.4 `agents.json` Schema

**Path:** `assets/agents.json` on disk; served at `/.well-known/agents.json` and `/agents.json`.

**Schema** (validated at boot via `gojsonschema` — already in `go.mod`):

```json
{
  "$schema": "https://thenexusengine.com/schemas/agents.v1.json",
  "version": "1.0",
  "seller_id": "9131",
  "seller_domain": "thenexusengine.com",
  "contact": "agentic-ops@thenexusengine.com",
  "updated_at": "2026-04-27T00:00:00Z",
  "agents": [
    {
      "id": "seg.example.com",
      "name": "Example Segmentation Agent",
      "role": "segmentation",
      "vendor": "Example Inc",
      "registry_ref": "iab-tools-portal:reg-12345",
      "endpoints": [
        {
          "transport": "grpc",
          "url": "grpcs://seg.example.com:50051",
          "auth": "api_key_header"
        }
      ],
      "lifecycles": ["PUBLISHER_BID_REQUEST"],
      "intents": ["ACTIVATE_SEGMENTS", "ADD_CIDS"],
      "priority": 100,
      "tmax_ms": 25,
      "essential": false,
      "requires": {
        "tcf_purposes": [1, 7],
        "precise_geo": false
      },
      "data_processing": {
        "categories": ["interest_segments", "contextual"],
        "retention_days": 0
      }
    }
  ]
}
```

**Field semantics:**

| Field | Required | Meaning |
|---|---|---|
| `version` | yes | Schema version; we own. Currently `"1.0"`. |
| `seller_id` | yes | Our `Originator.id`. Match schain. |
| `agents[].id` | yes | Stable identifier (DNS-name-shaped recommended). Used as the audit log key. |
| `agents[].role` | yes | One of `segmentation | floor | shading | margin | metrics | composite`. |
| `agents[].registry_ref` | no | IAB Tools Portal registration ID once we register. |
| `agents[].endpoints[].transport` | yes | `grpc` (Phase 1). `mcp`, `http` reserved. |
| `agents[].endpoints[].auth` | yes | `api_key_header | mtls | none`. |
| `agents[].lifecycles` | yes | Subset of `PUBLISHER_BID_REQUEST | DSP_BID_RESPONSE`. |
| `agents[].intents` | yes | Subset of the seven supported intents. |
| `agents[].priority` | no | 0–1000, higher = applied later (last-writer-wins per R5.5.7). Default 100. |
| `agents[].tmax_ms` | no | Per-agent budget; clamped to global `AGENTIC_TMAX_MS`. |
| `agents[].essential` | no | If `true`, agent runs even when consent withholds optional purposes. |
| `agents[].requires` | no | Pre-call gating signals. |
| `agents[].data_processing` | no | Disclosure for the publisher / IAB registry. |

**JSON Schema** stored alongside the document at `assets/agents.schema.json` and loaded at boot.

---

## 8. API Surface

### 8.1 New HTTP Routes

Registered in `cmd/server/server.go` adjacent to existing well-known routes (around line 403):

| Method | Path | Handler | Auth |
|---|---|---|---|
| GET | `/agents.json` | `endpoints.HandleAgentsJSON` | none (public) |
| GET | `/.well-known/agents.json` | `endpoints.HandleAgentsJSON` | none (public) |
| GET | `/admin/agents` | `endpoints.HandleAgentsAdminList` | adminAuth |
| GET | `/admin/agents/{id}` | `endpoints.HandleAgentsAdminGet` | adminAuth |

Phase 1 ships **read-only** admin endpoints. CRUD comes Phase 2.

### 8.2 New gRPC Endpoints (Phase 2 — non-goal this cycle)

Documented for forward-compat:
- `:50051` plain gRPC `RTBExtensionPoint/GetMutations`
- `:50052` MCP transport (gated)

### 8.3 New Env Vars / Config

Added to `cmd/server/config.go`:

| Env var | Default | Meaning |
|---|---|---|
| `AGENTIC_ENABLED` | `false` | Master kill switch. When false, agentic code path is skipped entirely. |
| `AGENTIC_AGENTS_PATH` | `assets/agents.json` | Path to agent registry document. |
| `AGENTIC_SCHEMA_PATH` | `assets/agents.schema.json` | Path to JSON schema. |
| `AGENTIC_TMAX_MS` | `30` | Global hard cap on agent fanout (per lifecycle). |
| `AGENTIC_SELLER_ID` | `9131` | Value emitted as `Originator.id`. |
| `AGENTIC_API_KEY` | `""` | Outbound `x-aamp-key` gRPC metadata header value. Per-agent override via env `AGENTIC_API_KEY_<AGENT_ID>`. |
| `AGENTIC_CIRCUIT_FAILURE_RATE` | `0.5` | Circuit-breaker trip threshold. |
| `AGENTIC_CIRCUIT_MIN_CALLS` | `20` | Min sample size before breaker can trip. |
| `AGENTIC_CIRCUIT_HALFOPEN_S` | `30` | Half-open delay seconds. |
| `AGENTIC_MAX_MUTATIONS_PER_RESPONSE` | `64` | Hard cap. |
| `AGENTIC_MAX_IDS_PER_PAYLOAD` | `256` | Hard cap. |
| `AGENTIC_GRPC_PORT` | `0` | Reserved (Phase 2). `0` = disabled. |
| `AGENTIC_MCP_ENABLED` | `false` | Reserved (Phase 2). |

`ServerConfig` gains a sub-struct:

```go
// cmd/server/config.go
type AgenticConfig struct {
    Enabled                bool
    AgentsPath             string
    SchemaPath             string
    TmaxMs                 int
    SellerID               string
    APIKey                 string
    PerAgentAPIKeys        map[string]string  // agent_id → key
    CircuitFailureRate     float64
    CircuitMinCalls        int
    CircuitHalfOpenSeconds int
    MaxMutationsPerResponse int
    MaxIDsPerPayload       int

    // Phase 2 — reserved but unused
    GRPCPort   int
    MCPEnabled bool
}
```

`Validate()` enforces:
- `Enabled=true` AND `AgentsPath` not empty AND file exists.
- `TmaxMs` between 5 and 500.
- `SellerID` non-empty.
- `APIKey` non-empty when at least one agent declares `auth=api_key_header`.

### 8.4 Admin UI Hooks

`/admin/agents` returns JSON list view (Phase 1). The existing onboarding admin SPA (`internal/endpoints/onboarding_admin.go`) gains a stub "Agents" tab in Phase 2 — for Phase 1 we expose the JSON only, no UI.

### 8.5 Backwards-Compatible HTTP Behaviour

When `AGENTIC_ENABLED=false`:
- `/agents.json` and `/.well-known/agents.json` return **404 Not Found** (rather than an empty doc) so external scrapers do not register us as agentic.
- All admin routes return 404.
- `ext.aamp` is **never** written to outbound bid requests.
- Bidders see no behaviour change.

---

## 9. Architecture

### 9.1 End-to-end Flow

```
                      Publisher page (prebid.js + tneCatalystBidAdapter)
                                         │
                            ortb2.ext.aamp = { originator: PUBLISHER, … }
                                         │
                                         ▼
            ┌───────────────────────────────────────────────────────────────┐
            │  Catalyst SSP (cmd/server)                                     │
            │                                                                │
            │  ┌─────────────────────────────────────────────────────────┐   │
            │  │ /openrtb2/auction → privacyMiddleware → auction handler │   │
            │  └────────────────────────────┬────────────────────────────┘   │
            │                                │                                │
            │                                ▼                                │
            │  ┌──────────────────────────────────────────────────────────┐  │
            │  │ exchange.RunAuction (internal/exchange/exchange.go:1351) │  │
            │  │                                                          │  │
            │  │   ┌── input validation, FPD, schain, currency, etc. ──┐ │  │
            │  │                                                       │ │  │
            │  │   ╔══════════════════════════════════════════════╗   │ │  │
            │  │   ║ HOOK A — agentic.Client.Dispatch(            ║◄──┘ │  │
            │  │   ║    LIFECYCLE_PUBLISHER_BID_REQUEST)          ║     │  │
            │  │   ║   → applier.Apply(req, …)                    ║     │  │
            │  │   ║   inserted at exchange.go:~1595              ║     │  │
            │  │   ╚══════════════════════════════════════════════╝     │  │
            │  │                              │                          │  │
            │  │                              ▼                          │  │
            │  │       callBiddersWithFPD (line 1596) → adapters         │  │
            │  │                              │                          │  │
            │  │                              ▼                          │  │
            │  │   ╔══════════════════════════════════════════════╗     │  │
            │  │   ║ HOOK B — agentic.Client.Dispatch(            ║     │  │
            │  │   ║    LIFECYCLE_DSP_BID_RESPONSE)               ║     │  │
            │  │   ║   → applier.Apply(rsp, …) [BID_SHADE,        ║     │  │
            │  │   ║      ADJUST_DEAL_MARGIN]                     ║     │  │
            │  │   ║   inserted at exchange.go:~1796              ║     │  │
            │  │   ╚══════════════════════════════════════════════╝     │  │
            │  │                              │                          │  │
            │  │                              ▼                          │  │
            │  │       runAuctionLogic (line 1797) → applyBidMultiplier  │  │
            │  │                              │                          │  │
            │  │                              ▼                          │  │
            │  │       buildAuctionObject → response with                │  │
            │  │       bid.ext.aamp.agentsApplied[]                      │  │
            │  └──────────────────────────────────────────────────────────┘  │
            │                                                                │
            └─────────┬──────────────────────────┬───────────────────────────┘
                      │                          │
                      │ HOOK A: gRPC fan-out     │ HOOK B: gRPC fan-out
                      │ (parallel, tmax-budgeted) (parallel, tmax-budgeted)
                      ▼                          ▼
            ┌─────────────────┐        ┌─────────────────┐
            │ Segmentation    │        │ Bid-shading     │
            │ Agent           │        │ Agent           │
            │ (RTBExtension-  │        │ (RTBExtension-  │
            │  Point gRPC)    │        │  Point gRPC)    │
            └─────────────────┘        └─────────────────┘

                   Each agent: returns []Mutation per ARTF v1.0
                   We apply: whitelisted intents only, audited per mutation
```

### 9.2 Hook Points in Existing Code

Two hooks added to `internal/exchange/exchange.go`. Line numbers are pre-edit; exact diff lands in the implementation plan doc.

**Hook A — pre-fanout, `LIFECYCLE_PUBLISHER_BID_REQUEST`:**

Inserted immediately before `callBiddersWithFPD(ctx, req.BidRequest, selectedBidders, timeout, bidderFPD)` at line **1596**. Pseudo-diff:

```go
// before line 1596
if e.agenticClient != nil {
    rtbReq := agentic.WrapAsRTBRequest(req.BidRequest, agentic.LifecyclePublisherBidRequest, e.agenticStamper)
    res := e.agenticClient.Dispatch(ctx, rtbReq, agentic.LifecyclePublisherBidRequest)
    decisions := e.agenticApplier.Apply(req.BidRequest, nil, res.Mutations, agentic.LifecyclePublisherBidRequest)
    e.agenticTelemetry.Record(req.BidRequest.ID, decisions, res.AgentStats)
    e.agenticStamper.StampOpenRTB(req.BidRequest, agentic.LifecyclePublisherBidRequest, /*consent=*/agentic.DeriveAgentConsent(req.BidRequest), agentNames(res))
}

results := e.callBiddersWithFPD(ctx, req.BidRequest, selectedBidders, timeout, bidderFPD)
```

**Hook B — post-fanout, `LIFECYCLE_DSP_BID_RESPONSE`:**

Inserted between `validBids` collection and `runAuctionLogic(validBids, impFloors)` at line **1797**. Pseudo-diff:

```go
// before line 1797
if e.agenticClient != nil {
    pseudoRsp := buildPseudoBidResponse(validBids)  // ARTF needs a BidResponse to mutate
    rtbReq := agentic.WrapAsRTBRequestWithResponse(req.BidRequest, pseudoRsp, agentic.LifecycleDSPBidResponse, e.agenticStamper)
    res := e.agenticClient.Dispatch(ctx, rtbReq, agentic.LifecycleDSPBidResponse)
    decisions := e.agenticApplier.Apply(req.BidRequest, pseudoRsp, res.Mutations, agentic.LifecycleDSPBidResponse)
    e.agenticTelemetry.Record(req.BidRequest.ID, decisions, res.AgentStats)
    applyShadingDecisionsToValidBids(validBids, decisions)  // mutate prices in-place
}

auctionedBids := e.runAuctionLogic(validBids, impFloors)
```

**Constructor wiring.** `exchange.New` (line 251) gains optional `WithAgentic(client *agentic.Client, applier *agentic.Applier, stamper agentic.OriginatorStamper)` builder method. When unset, hooks are no-ops (R5.1.5). `cmd/server/server.go` constructs and injects when `AGENTIC_ENABLED=true`.

### 9.3 Failure Mode Behaviour

| Failure | Effect on auction | Logged |
|---|---|---|
| All agents timeout (Hook A) | Auction proceeds with original `BidRequest`, no enrichment | `agentic.dispatch_truncated_total` += 1 |
| All agents timeout (Hook B) | Auction proceeds with original prices, no shading | `agentic.dispatch_truncated_total` += 1 |
| Single agent timeout | That agent's mutations dropped, others applied | per-call `status: timeout` |
| Agent returns malformed proto | Drop entire response, log `decode_failed` | per-call `status: error` |
| Circuit breaker open for agent | Agent skipped without dialling | per-call `status: circuit_open` |
| `agentic.Apply` panics | Recovered; auction proceeds with no mutations applied; alert metric incremented | sentinel `agentic.applier_panic` |

The auction **never fails** because of an agent failure. Agents are strictly best-effort.

### 9.4 Concurrency Model

- One `*agentic.Client` per `Exchange` instance, safe for concurrent use.
- One persistent `grpc.ClientConn` per agent endpoint, established at boot, not per-call.
- Per-call `context.WithTimeout(ctx, agentTmax)` derived from auction context; cancellation propagates so a slow agent does not hold the auction.
- `Apply` runs synchronously on the caller goroutine (no fan-in latency); mutations are applied serially in deterministic order (R5.5.8).

### 9.5 Where Existing Subsystems Plug In

| Existing subsystem | How it interacts |
|---|---|
| `idr.CircuitBreaker` (exchange.go:337) | Reused per agent endpoint. New breaker prefix: `agentic:<agent_id>`. |
| `metrics.Recorder` (exchange.go:53) | Extended interface gains `RecordAgenticCall`, `RecordAgenticMutation`. |
| `privacyMiddleware` (server.go:323) | Unchanged. Runs before auction handler — agentic code reads its output. |
| `bidder_field_rules` (per `2026-03-30-bid-request-composer-design.md`) | Independent. Composer runs after agentic Hook A so agent-injected segments flow through to bidders. |
| Telemetry checkpoints (per `2026-03-13-bid-telemetry-ssp-audit-design.md`) | New checkpoints `CP-AGENTIC-1` and `CP-AGENTIC-2` slot into the existing report card. |
| `assets/sellers.json` handler | `assets/agents.json` handler is a parallel implementation; they share no code but live side-by-side. |

---

## 10. Phased Delivery Plan

### 10.1 Phase 1 — Outbound, Read-Mostly (this cycle, ~2 weeks)

**Scope:**
- ARTF protos vendored under `proto/iabtechlab/`
- `internal/agentic/` package: `Client`, `Applier`, `Registry`, `OriginatorStamper`, `Envelope`, `Consent`
- Hook A and Hook B wired into `exchange.RunAuction`
- `/.well-known/agents.json` + `/agents.json` HTTP handlers
- `assets/agents.json` + `assets/agents.schema.json` shipped
- `ServerConfig.Agentic` + env vars per §8.3
- Read-only `/admin/agents` + `/admin/agents/{id}`
- Per-mutation structured logging + new Prometheus metrics
- Unit tests on every intent applier + circuit breaker integration test
- Integration test: full auction against in-process fake agent gRPC server
- README section + new `docs/integrations/agentic/` directory documenting how to register an agent

**Out of scope:**
- Inbound gRPC server (Phase 2)
- MCP transport (Phase 2)
- Per-publisher overrides (Phase 2)
- Live integration with a named external agent (Phase 2 stretch)
- Prebid.js adapter delivered to publishers (scaffold only — see §10.1.s1)

**Default state shipped:** `AGENTIC_ENABLED=false`. Production opts in per environment.

**Success criteria for Phase 1 closure:**
- `go build ./...` clean
- `go test ./...` clean, including new integration test
- Staging deploy with `AGENTIC_ENABLED=true` + fake agent shows mutations applied in audit log
- P95 auction latency overhead ≤ +30 ms with one fake agent
- Zero regressions in existing test suite

#### 10.1.s1 Stretch — Prebid Adapter Scaffold (Phase 1)

`web/prebid-adapter/` directory created per §6.7. Adapter compiles, unit tests pass on `tneCatalystAgenticEnvelope` and `agenticConsent`. Not yet shipped to publishers. **Stretch:** drop if Phase 1 server work runs long.

### 10.2 Phase 2 — Inbound Seller-Agent + Prebid Adapter Public Beta (~4 weeks after Phase 1)

**Scope:**
- Inbound `RTBExtensionPoint` gRPC server on `:50051`
- mTLS termination via existing nginx; per-buyer cert pinning
- Per-publisher `agentic_override` JSONB column on `publishers_new` + admin UI tab
- Prebid adapter `tneCatalystBidAdapter@2.0.0` published privately (via npm GitHub Packages or similar)
- `agentDiscoveryRtdProvider` published as separate prebid module
- One named external agent partner integrated end-to-end (segmentation vendor)
- MCP transport on `:50052` behind feature flag, dev/test only

**Trigger to start Phase 2:** Phase 1 in production with `AGENTIC_ENABLED=true` for ≥ 7 days at zero auction-failure rate, OR a named buyer-agent partner asks for the inbound endpoint.

### 10.3 Phase 3 — Registry & Deals API Agentification (when IAB freezes)

**Scope:**
- Self-register on the IAB Tools Portal Agent Registry
- Implement Deals API agentic extensions once spec is frozen
- Cross-platform agent attribution rollups in our analytics warehouse

**Trigger:** IAB Tech Lab publishes Deals API agentic extension v1.0 (no committed date as of 2026-04).

### 10.4 Rollout Strategy (Phase 1)

1. **Day 1–7:** Implementation on `claude/integrate-iab-agentic-protocol-6bvtJ`. PR opens at end of Day 7.
2. **Day 8–10:** Code review + test run-through with fake agent.
3. **Day 11–12:** Deploy to staging with `AGENTIC_ENABLED=true` + fake agent. Run smoke + load tests.
4. **Day 13:** Deploy to prod with `AGENTIC_ENABLED=false`. Verify zero behaviour change (canary).
5. **Day 14:** Flip `AGENTIC_ENABLED=true` in prod with **zero agents configured** (the path runs but does nothing). Watch latency for 24 h.
6. **Day 15+:** Onboard first agent (TBD).

### 10.5 Rollback Plan

Phase 1 is reversible by env var: setting `AGENTIC_ENABLED=false` and SIGHUP'ing the server reverts the auction path immediately. No data migrations to roll back. No bidder-side dependencies to coordinate.

---

## 11. Risks & Open Questions

### 11.1 Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|---|---|---|
| R1 | ARTF v1.0 field numbers renumber before final | Med | Med | Vendor protos at pinned SHA; isolate proto-touching code in `internal/agentic/`; refresh-on-spec-change is one-package change |
| R2 | A slow agent stretches auction tmax | High | High | Hard tmax cap (R5.1.2); circuit breaker (R5.1.4); dispatch never blocks bidder fanout; cancellation propagates |
| R3 | Agent returns adversarial mutations (e.g. floors to $0) | Med | High | Whitelist intents (R5.5.1); path validation (R5.5.4); bounds clamping (R5.5.9, R5.5.10); per-mutation audit |
| R4 | Consent model for agent processing not codified | High | Med | Derive from existing TCF/GPP middleware; document our mapping; ready to swap when IAB freezes |
| R5 | Inbound MCP transport = LLM-prompt-injection surface | Med | High | Phase 2 only; gated by feature flag; never enabled in prod without security review |
| R6 | Vendor lock-in to one agent vendor's mutation patterns | Low | Med | Standard ARTF intents only; reject vendor-specific extensions Phase 1 |
| R7 | Latency P95 regression in prod | Med | Med | Default-off shipping; canary deploy with `AGENTIC_ENABLED=true` + zero agents; per-agent breaker |
| R8 | Spec churn in `agents.json` since we own the schema | Low | Low | Versioned (`version: "1.0"` in document); migration path documented |
| R9 | Bidders mis-handle `ext.aamp` (some bidders strip unknown ext keys, some 400) | Med | Med | Bidder-specific allow/deny via existing `bidder_field_rules`; QA on each adapter before flipping `AGENTIC_ENABLED=true` in prod |
| R10 | gRPC + protobuf added to a previously-stdlib codebase increases build complexity | Low | Low | Generated code committed; no codegen at build time; both deps already transitive |

### 11.2 Open Questions

- **OQ1.** Self-register on IAB Tools Portal Agent Registry in Phase 1 or Phase 2? — **Defer to Phase 2** (recommended). Registry implies an SLA we are not yet ready to publicly commit to.
- **OQ2.** First named external agent partner? — TBD; ops to identify a segmentation vendor.
- **OQ3.** Should `BID_SHADE` be enabled in prod from Day 1, or held off until we have a live shading agent we trust? — **Hold off**; ship the code path but document `BID_SHADE`-disabled-by-default per-environment env var.
- **OQ4.** Do we publish the prebid adapter to npm (private registry) Phase 1 or Phase 2? — **Phase 2.** Phase 1 ships the scaffold in-repo only.
- **OQ5.** mTLS on outbound calls Phase 1 or API key only? — **API key only** per user decision 2026-04-27. mTLS in Phase 2.
- **OQ6.** Does the existing `slot_bidder_configs` chain need to know about agent-injected deals? — Probably yes for reporting; punt to Phase 2 once we see real usage patterns.

### 11.3 Decisions Locked This PRD

- **D1.** Phase 1 only on this branch.
- **D2.** Global allow-list in `ServerConfig` for Phase 1; `publishers_new.agentic_override` reserved unused.
- **D3.** `Originator.id = "9131"`.
- **D4.** `AGENTIC_ENABLED=false` default.
- **D5.** API key in gRPC metadata for outbound auth.
- **D6.** Add `google.golang.org/grpc` + promote `google.golang.org/protobuf` to direct dep.
- **D7.** Prebid adapter scaffold in `web/prebid-adapter/`; not published.
- **D8.** Push branch when done; no PR opened.

---

## 12. Success Metrics

### 12.1 Phase 1 Acceptance KPIs

| KPI | Target | Measured by |
|---|---|---|
| P95 auction latency overhead with `AGENTIC_ENABLED=true` + 1 agent | ≤ +30 ms vs control | Prom histogram diff: auction_duration_seconds with/without agentic label |
| P99 auction latency overhead | ≤ +60 ms | Same |
| Mutation apply rate (applied / dispatched mutations) | ≥ 95% | `agentic_mutation_total{decision="applied"}` / total |
| Agent call success rate (ok / total dialled) | ≥ 99% against fake agent; ≥ 95% against real partner | `agentic_call_duration_seconds_count{status="ok"}` / total |
| `dispatch_truncated_total` per million auctions | ≤ 100 | Counter rate |
| Auction-failure rate due to agentic code | 0 | Recovered panics + auction error rate diff |
| Prom + log signal coverage on every mutation | 100% of applied + rejected | Manual audit on first 10 k mutations |

### 12.2 Commercial Metrics (post-Phase 1, monitored over 30 d)

| KPI | Target | Notes |
|---|---|---|
| Revenue lift on auctions with agent path | +1.5% to +5% | Holdout: 10% control with `AGENTIC_ENABLED=false`; needs a real segmentation agent live |
| Floor compliance | ≥ 99.99% | Floor mutations clamped to publisher floor never violated |
| Bid shading delta P95 | -10% to -30% of original price | Once `BID_SHADE` enabled |
| Buyer agent dial-ins (Phase 2) | ≥ 3 distinct in 60 d | Unique `originator.id` from inbound |

### 12.3 Operational Metrics (Phase 1)

| KPI | Target |
|---|---|
| Time from `AGENTIC_ENABLED=true` flip to first applied mutation in prod | ≤ 5 min |
| Time to disable agentic in prod (env flip + SIGHUP) | ≤ 60 s |
| Onboarding a new agent (config + deploy) | ≤ 1 h |

---

## 13. Compliance & Security

### 13.1 Privacy Checklist (must pass before prod flip)

- [ ] TCF v2 consent string passed unchanged to agents that declare matching purposes
- [ ] GPP applicable section honored; non-essential agents skipped on opt-out
- [ ] CCPA `us_privacy` string passed unchanged
- [ ] COPPA: hard-block all agent fanout when `regs.coppa=1` (verified test)
- [ ] No PII added by agents; `ACTIVATE_SEGMENTS` payloads treated as opaque IDs
- [ ] Existing `privacyMiddleware` redaction applied before agent fanout (no new minimisation logic)
- [ ] User opt-out (`/optout` endpoint) suppresses agent fanout (test added)
- [ ] `ext.aamp.agentConsent=false` traffic does not dispatch non-essential agents (test added)

### 13.2 Security Checklist

- [ ] All agent endpoints in `agents.json` resolve via TLS (`grpcs://`); `grpc://` rejected at boot when `AGENTIC_ENV=production`
- [ ] `AGENTIC_API_KEY` and per-agent overrides pulled from env, never logged
- [ ] No agent allowed to mutate `user.id`, `user.buyeruid`, `user.geo.lat/lon`, or any field outside the per-intent allowlist (R5.5.4)
- [ ] Mutation cap (R5.5.5) and payload-size cap (R5.5.6) enforced before any work done with agent data
- [ ] Agent gRPC clients use `keepalive` to avoid connection-flood; `MaxRecvMsgSize` set to 4 MB
- [ ] Audit log redacts API keys; only includes `agent_id`
- [ ] Phase 2 inbound mTLS handshake verified end-to-end (out of cycle)

### 13.3 Operational Security

- Agentic feature kill switch (`AGENTIC_ENABLED=false`) verified to disable code path in load test before prod flip.
- Per-agent breaker tested under simulated failure injection (5xx, timeouts, malformed proto).
- `agents.json` schema validation failure at boot causes startup failure in production env, log + continue with empty registry in dev.

---

## 14. Out of Scope (This Cycle)

- **N1.** Inbound `RTBExtensionPoint` gRPC server. Phase 2.
- **N2.** MCP transport (`extend_rtb` tool exposure). Phase 2.
- **N3.** Deals API agentification. Phase 3 (waiting on IAB).
- **N4.** Replacing IDR ML routing.
- **N5.** Self-registration on IAB Tools Portal.
- **N6.** Cross-tenant agent marketplace, publisher-facing agent picker UI, billing/metering.
- **N7.** Publishing the prebid.js adapter to the official prebid.js repo (or any public npm registry).
- **N8.** Per-publisher agent override (DB column reserved, not used).
- **N9.** Live external agent integration (defer to Phase 2 trigger).
- **N10.** A2A protocol support.
- **N11.** Buyer-agent SDK we publish for DSPs.
- **N12.** Auction-time agent billing / cost attribution.

---

## 15. Appendix

### 15.1 References

**External**
- [IAB Tech Lab — AAMP overview](https://iabtechlab.com/standards/aamp-agentic-advertising-management-protocols/)
- [IAB Tech Lab — Agentic Standards hub](https://iabtechlab.com/standards/agentic-advertising-and-ai/)
- [IABTechLab/agentic-rtb-framework (GitHub)](https://github.com/IABTechLab/agentic-rtb-framework)
- [PRNewswire — ARTF v1.0 Public Comment](https://www.prnewswire.com/news-releases/iab-tech-lab-announces-agentic-rtb-framework-artf-v1-0-for-public-comment-302613712.html)
- [PRNewswire — IAB Tech Lab Agentic Roadmap (Jan 2026)](https://www.prnewswire.com/news-releases/iab-tech-lab-unveils-agentic-roadmap-for-digital-advertising-302654047.html)
- [AdExchanger — first framework for agentic ad-buying standards](https://www.adexchanger.com/platforms/the-iab-tech-lab-releases-its-first-framework-for-agentic-ad-buying-standards/)
- [MarTech — Agentic RTB Framework explainer](https://martech.org/iab-tech-lab-unveils-agentic-rtb-framework-to-boost-real-time-ad-trading-efficiency/)
- [Prebid.js — Adapter Authoring Guide](https://docs.prebid.org/dev-docs/bidder-adaptor.html)
- [Prebid.js — RTD Provider Guide](https://docs.prebid.org/dev-docs/add-rtd-submodule.html)

**Internal**
- `docs/superpowers/specs/2026-03-13-bid-telemetry-ssp-audit-design.md` — telemetry checkpoints we extend
- `docs/superpowers/specs/2026-03-30-bid-request-composer-design.md` — composer runs after Hook A
- `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md` — companion task plan (created alongside this PRD)
- `docs/integrations/web-prebid/README.md` — existing prebid integration story
- `internal/exchange/exchange.go:1351` — `RunAuction` host of Hook A and Hook B
- `internal/endpoints/sellers_json.go` — pattern for `/.well-known/agents.json`
- `cmd/server/server.go:403` — sellers.json registration; agents.json registers adjacent

### 15.2 Vendored Files (Phase 1)

Source: `https://github.com/IABTechLab/agentic-rtb-framework`
Commit SHA: **TBD — pin at first vendor pull**
Date pulled: **2026-04-27**

| Vendored to | Source path |
|---|---|
| `proto/iabtechlab/bidstream/mutation/v1/agenticrtbframework.proto` | `proto/agenticrtbframework.proto` |
| `proto/iabtechlab/bidstream/mutation/v1/agenticrtbframeworkservices.proto` | `agenticrtbframeworkservices.proto` |
| `proto/iabtechlab/openrtb/v2.6/openrtb.proto` | `proto/com/iabtechlab/openrtb/v2.6/openrtb.proto` (transitive) |

Provenance recorded in `proto/iabtechlab/README.md` with SHA + date.

### 15.3 Glossary

- **AAMP** — Agentic Advertising Management Protocols. IAB Tech Lab umbrella for agent-aware programmatic standards (announced 2026-01-06, unified 2026-03).
- **ARTF** — Agentic RTB Framework. The first frozen wire spec under AAMP. v1.0 in public comment as of 2026-04.
- **MCP** — Model Context Protocol. Anthropic-originated transport for LLM-tool interaction; ARTF supports MCP as an alternate transport on `:50052`.
- **A2A** — Agent2Agent. Protocol for inter-agent coordination; referenced in IAB roadmap, not yet a wire spec.
- **Originator** — ARTF message describing who created the enclosed BidRequest/BidResponse. One of `PUBLISHER | SSP | EXCHANGE | DSP`.
- **Mutation** — A single change request from an agent: `(intent, op, path, value)`.
- **Intent** — The semantic purpose of a mutation: `ACTIVATE_SEGMENTS`, `BID_SHADE`, etc. Seven supported in v1.0.
- **Lifecycle** — Stage of the auction at which a mutation applies: `PUBLISHER_BID_REQUEST` (pre-fanout) or `DSP_BID_RESPONSE` (post-fanout).
- **Operation** — `ADD | REMOVE | REPLACE`. Borrows JSON-Patch semantics.
- **Extension Point** — An agent endpoint exposing `RTBExtensionPoint.GetMutations`.
- **Seller Agent** — An agent endpoint operated by the SSP. Phase 2 only.
- **Buyer Agent** — An agent endpoint operated by the DSP. Out of cycle.
- **Agent Registry** — IAB Tools Portal directory of registered agents. Opened 2026-03-01.

### 15.4 Document History

| Date | Author | Change |
|---|---|---|
| 2026-04-27 | Claude (overnight session 014d9kgdbq3ieoECL3qtcUB2) | Initial scaffold + full expansion |


---

_End of document._
