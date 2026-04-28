# IAB Agentic Protocol Integration — Design Doc (PRD)

**Date:** 2026-04-27
**Status:** DRAFT — scaffold; sections to be filled in one at a time
**Branch:** `claude/integrate-iab-agentic-protocol-6bvtJ`
**Owner:** TBD
**Spec relations:** new — no prior spec
**Companion plan:** `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md` (TBD, after this PRD lands)

> **For agentic workers:** This is a PRD/design doc. The implementation plan with task-level checkboxes will live in the companion plan file once this is approved.

---

## 0. TL;DR

_Stub — fill last. One paragraph: what we're building, why, and the shape of the work._

---

## 1. Problem Statement & Strategic Framing

_Stub — why does an "agentic-first SSP" matter to TNE Catalyst commercially, and what does the IAB AAMP/ARTF release in Jan–Mar 2026 force us to decide right now?_

- Market context (LLM-driven buyer agents, AAMP launch, Agent Registry opened 2026-03-01)
- Competitive positioning (we ship agent-aware before peers)
- What "agentic SSP" means concretely for our P&L (segment activation, deal optimization, bid shading delegated to specialist agents)

---

## 2. Goals & Non-Goals

### 2.1 Goals
_Stub — bulleted, measurable. Phase-1 scope vs aspiration._

### 2.2 Non-Goals
_Stub — what we are explicitly not building this cycle._

---

## 3. Background — AAMP / ARTF v1.0 As of 2026-04

_Stub — short, factual primer on the spec we're implementing against. Cite IAB Tech Lab, ARTF GitHub, Deals API agentification status._

- AAMP umbrella; ARTF is the only frozen v1.0 wire spec today
- Single gRPC service: `RTBExtensionPoint.GetMutations(RTBRequest) → RTBResponse`
- Two lifecycles: `LIFECYCLE_PUBLISHER_BID_REQUEST`, `LIFECYCLE_DSP_BID_RESPONSE`
- Originator types: `PUBLISHER | SSP | EXCHANGE | DSP`
- Seven mutation intents: `ACTIVATE_SEGMENTS`, `ACTIVATE_DEALS`, `SUPPRESS_DEALS`, `ADJUST_DEAL_FLOOR`, `ADJUST_DEAL_MARGIN`, `BID_SHADE`, `ADD_METRICS` (+ `ADD_CIDS` reserved)
- Transports: gRPC (50051), MCP (50052), Web UI (8081)
- Status: "v1.0 for Public Comment"; field numbers may shift

---

## 4. Personas & User Stories

_Stub — one-line stories tied to acceptance criteria later._

- **Publisher integrator** ("As a publisher using prebid.js, I want…")
- **Buyer-agent operator** (LLM/ML buyer hitting our seller-agent endpoint)
- **Extension-point agent vendor** (segment/floor/shading specialist we call out to)
- **Internal ops / SSP analyst** (audit, dashboards)
- **Compliance / privacy lead** (consent + agent-data flow)

---

## 5. Functional Requirements — SSP Server

### 5.1 Outbound Extension-Point Client (Phase 1)
_Stub — gRPC client, fan-out, tmax budgeting, retries off, circuit breaker, originator stamping._

### 5.2 Inbound Seller-Agent Service (Phase 2)
_Stub — we expose `RTBExtensionPoint` ourselves; new gRPC port; same proto._

### 5.3 `/.well-known/agents.json`
_Stub — schema, caching, served alongside sellers.json._

### 5.4 OpenRTB `Originator` + AAMP Extensions
_Stub — `BidRequest.ext.aamp` shape; how we set `Originator{type=TYPE_SSP, id="9131"}` on outbound calls._

### 5.5 Mutation Application Engine
_Stub — per-intent handlers, ordering, conflict resolution, max mutations per request._

### 5.6 Per-Publisher / Global Agent Config
_Stub — extend `publishers_new` with `agentic_config` JSON column vs. global allow-list._

### 5.7 MCP Transport (Phase 2)
_Stub — expose `extend_rtb` tool so LLM clients can drive auctions in dev/test; gated behind feature flag in prod._

### 5.8 Audit & Telemetry
_Stub — ties to existing telemetry (see `2026-03-13-bid-telemetry-ssp-audit-design.md`); per-mutation log, agent SLA dashboards._

### 5.9 Privacy & Consent
_Stub — TCF + CCPA pass-through, agent-processing consent flag, COPPA hard-block._

---

## 6. Functional Requirements — Prebid.js Adapter

### 6.1 New `bidder.params.agentic`
_Stub — `enableAgents`, `agentTimeoutMs`, `intentHints[]`, `disclosedAgents[]`._

### 6.2 Page-Side Originator & Intent Passthrough
_Stub — adapter sets `ortb2.ext.aamp.originator = {type:"PUBLISHER", id}` from prebid `pubid`; reads `ortb2.user.ext.aamp.intent` if pub provides._

### 6.3 Response Decoration — Agent Attribution
_Stub — surface mutations applied (which agents touched the auction) on `bid.meta.aamp` for analytics adapters._

### 6.4 Consent Surfacing
_Stub — read GPP/TCF; emit `ext.aamp.agentConsent` flag; block agent processing when withheld._

### 6.5 Discovery Module — `agents.json` Reader
_Stub — optional prebid module (`agentDiscoveryRtdProvider`) that fetches our `agents.json` once per session and exposes registry to other modules._

### 6.6 Backwards Compatibility
_Stub — adapter still works if publisher omits `params.agentic`; agentic = additive._

---

## 7. Data Shapes

### 7.1 Vendored Protos
_Stub — `proto/iabtechlab/bidstream/mutation/v1/agenticrtbframework.proto` + OpenRTB v2.6 protos; pinned to ARTF commit SHA._

### 7.2 Go Types (`internal/agentic/`)
_Stub — `ExtensionPointClient`, `MutationApplier`, `AgentRegistry`, `OriginatorStamper`._

### 7.3 OpenRTB `ext.aamp` JSON Shape
_Stub — concrete JSON example for request and response._

### 7.4 `agents.json` Schema
_Stub — fields: `version`, `seller_id`, `agents[]{id, role, transports[], intents[], registry_ref}`._

---

## 8. API Surface

### 8.1 New HTTP Routes
_Stub — `/.well-known/agents.json`, `/admin/agents` CRUD._

### 8.2 New gRPC Endpoints (Phase 2)
_Stub — `RTBExtensionPoint` on `:50051`._

### 8.3 New Env Vars / Config
_Stub — `AGENTIC_ENABLED`, `AGENTIC_AGENTS_PATH`, `AGENTIC_TMAX_MS`, `AGENTIC_GRPC_PORT`, `AGENTIC_MCP_ENABLED`._

### 8.4 Admin UI Hooks
_Stub — new "Agents" tab in onboarding admin SPA; per-publisher agent toggle._

---

## 9. Architecture

### 9.1 Diagram
_Stub — ASCII diagram: prebid.js → catalyst → [pre-bid agent fanout] → bidders → [post-bid agent fanout] → mutation applier → response._

### 9.2 Hook Points in Existing Code
_Stub — exact line ranges in `internal/exchange/exchange.go` where the two lifecycle calls are inserted._

---

## 10. Phased Delivery Plan

### 10.1 Phase 1 — Outbound, Read-Mostly (target: 2 weeks)
_Stub — extension-point client + originator stamping + agents.json + telemetry. No public gRPC server. No prebid changes shipped to publishers; only `ext.aamp` on the wire._

### 10.2 Phase 2 — Inbound Seller-Agent + Prebid.js (target: 4 weeks)
_Stub — expose our own RTBExtensionPoint; ship updated prebid.js adapter; MCP behind flag._

### 10.3 Phase 3 — Registry & Deals API Agentification (target: when IAB freezes spec)
_Stub — register on Tools Portal; agentic Deals API endpoints._

---

## 11. Risks & Open Questions

_Stub — table: risk, likelihood, impact, mitigation._

- ARTF v1.0 is "for public comment" — field numbers may renumber
- Agent SLA: a slow agent stretches our auction tmax
- Consent model for agent processing not yet codified by IAB
- MCP transport in production = LLM-prompt-injection surface
- Open question: per-publisher vs global agent allow-list (asked of user 2026-04-27)
- Open question: do we self-register on the IAB Tools Portal in Phase 1 or wait?

---

## 12. Success Metrics

_Stub — KPIs and how we measure._

- Latency overhead of pre-bid agent fanout: P95 ≤ +30ms
- Mutation apply rate (agent calls succeeded ÷ agent calls dispatched): ≥ 95%
- Revenue lift on auctions with agent path enabled vs control: target +X%
- Number of distinct buyer agents reaching our seller-agent endpoint (Phase 2): ≥ 3 within 60 days
- Agent SLA breaches per million auctions: ≤ 100

---

## 13. Compliance & Security

_Stub — checklist._

- TCF v2 / GPP pass-through unchanged
- Agent-processing consent: `ext.aamp.agentConsent` required for non-essential agents
- mTLS or signed JWT on outbound extension-point gRPC
- COPPA: agent fanout disabled when `coppa=1`
- Audit log: every mutation captured with `originator`, `agent_id`, `intent`, `op`, `path`, `applied|rejected`, latency

---

## 14. Out of Scope (This Cycle)

_Stub — list._

- Full Deals API agentification (waiting on IAB)
- Buyer-agent SDK we publish for DSPs (consumer of our endpoint, not producer)
- Cross-tenant agent marketplace
- Replacing existing IDR ML routing — agentic is additive, not a replacement

---

## 15. Appendix

### 15.1 References
- [IAB Tech Lab — AAMP overview](https://iabtechlab.com/standards/aamp-agentic-advertising-management-protocols/)
- [IAB Tech Lab — Agentic Standards hub](https://iabtechlab.com/standards/agentic-advertising-and-ai/)
- [IABTechLab/agentic-rtb-framework (GitHub)](https://github.com/IABTechLab/agentic-rtb-framework)
- [PRNewswire — ARTF v1.0 Public Comment](https://www.prnewswire.com/news-releases/iab-tech-lab-announces-agentic-rtb-framework-artf-v1-0-for-public-comment-302613712.html)
- [PRNewswire — IAB Tech Lab Agentic Roadmap (Jan 2026)](https://www.prnewswire.com/news-releases/iab-tech-lab-unveils-agentic-roadmap-for-digital-advertising-302654047.html)
- Internal: `docs/superpowers/specs/2026-03-13-bid-telemetry-ssp-audit-design.md`
- Internal: `docs/superpowers/specs/2026-03-30-bid-request-composer-design.md`
- Internal: `docs/integrations/web-prebid/README.md`

### 15.2 Vendored Files (Phase 1)
_Stub — exact file list from ARTF repo + commit SHA._

### 15.3 Glossary
_Stub — AAMP, ARTF, MCP, A2A, Originator, Mutation, Intent, Lifecycle._

---

_End of scaffold. Sections marked **_Stub_** to be expanded one at a time in subsequent commits._
