# IAB AAMP 2.0 Phase 2 — Inbound Seller Agent + Curator Revenue Path

**Date:** 2026-04-28
**Status:** DRAFT — full v1, awaiting review
**Branch:** `claude/integrate-iab-agentic-protocol-6bvtJ`
**Owner:** TBD
**Spec relations:** builds on `2026-04-27-iab-agentic-protocol-integration-design.md` (Phase 1 — outbound, shipped)
**Companion plan:** `docs/superpowers/plans/2026-04-28-aamp-2-phase-2-curator-revenue.md` (TBD, after PRD lands)

> **For agentic workers:** Phase 2 PRD. Phase 1 (outbound ARTF v1.0) is in production. This doc covers the inbound surface that turns Catalyst into a revenue-bearing Seller Agent under AAMP 2.0, with a curator-first GTM motion.

---

## 0. TL;DR

Phase 1 made TNE Catalyst the first SSP to speak ARTF v1.0 on the wire — outbound only, default-off, technically clean but commercially passive. **Phase 2 turns Catalyst into a revenue-bearing AAMP 2.0 Seller Agent** by exposing an inbound `RTBExtensionPoint` gRPC server on `:50051` (mTLS-only, SPKI-pinned, IAB-Registry-cross-checked), an OpenDirect 2.1 deal-creation lane that lets curator partners programmatically create deals against our inventory with per-deal take-rate accounting, and a Curator Integration Kit (Go + Node SDKs, sandbox endpoint, onboarding docs) that makes the second curator onboard in under an hour with zero engineering touch. Three sub-phases ship independently: **2A** is the inbound surface (2 weeks); **2B** is the deal-creation lane + Postgres migration + settlement events (3 weeks); **2C** is IAB Tools Portal self-registration + curator SDK + sandbox (2 weeks). Existing Phase 1 components — Applier, Registry, OriginatorStamper, Consent gate — all reuse as-is in the inverse direction; only the inbound surface and the deal lane are new. Default-off behind `AGENTIC_INBOUND_ENABLED` and `AGENTIC_OPENDIRECT_ENABLED`, fully reversible by env-var flip, Phase 2 closure metric is one curator partner live in staging with ≥ 1 USD test-mode GPM and a second curator onboarded entirely from documentation. Strategic posture: AAMP 2.0 just released, the market is moving, **PMG/Alli is the first proven counterparty** — curators are the right wedge because they bring demand we don't have to chase, and the take-rate on programmatic deals that don't exist as PMP slots today is pure incremental margin.

---

## 1. Problem Statement & Strategic Framing

Phase 1 (shipped 2026-04-28) made TNE Catalyst the first SSP to speak IAB ARTF v1.0 on the wire — outbound-only, default-off, technically clean but commercially passive. We can *receive* mutations from extension-point agents we choose to call out to. We cannot *receive revenue-bearing traffic* from buyer agents. That changes the day we expose an inbound `RTBExtensionPoint` server and accept programmatically-created deals.

**The IAB Tech Lab released AAMP 2.0 on 2026-04-26** — three days before we shipped Phase 1. AAMP 2.0 elevates the SDKs from "Direct Sold campaigns only" to **transaction-ready Buyer/Seller Agent SDKs** that handle programmatic deals, OpenDirect 2.1 negotiation, AdCOM-typed audiences, and IAB Tools Portal-mediated discovery. **PMG integrated AAMP into their Alli Operating System** as the first production reference; the market is moving from "wait for a buyer ask" to "match the spec curators are integrating against."

**The curator revenue motion (the commercial unlock).** We have existing curator partners — vendors who package audiences + content signals and bring buyer-side demand. Today they sell those packages bilaterally as static PMPs. With AAMP 2.0, a curator's buyer-agent can:

1. Discover Catalyst via the IAB Agent Registry
2. Programmatically create a deal on our inventory matching their audience criteria
3. Activate the deal at machine speed
4. Bid into it via the standard auction path

Revenue maths for us: a take-rate on **programmatic deals that don't exist as PMP slots today**. Curator brings demand + audience; we provide the agent endpoint, the auction, and the publisher inventory. The deal didn't exist before; the GPM (gross programmatic margin) is incremental.

**The strategic posture for this cycle.** Phase 2 is *curator-first* — we don't build for hypothetical DSP buyer-agents we have no relationship with; we build the surface our curator friends will actually integrate against, and let DSP traffic ride in over the same pipe later. Phase 2A ships the inbound `RTBExtensionPoint` server (existing mutation flow, just reversed); Phase 2B ships the OpenDirect 2.1 deal-creation lane (new, the revenue surface); Phase 2C ships the Agent Registry self-registration + curator SDK (so onboarding the next curator is documentation, not engineering).

**Main tradeoff to flag.** AAMP 2.0 is freshly released — there will be churn (likely a 2.1 / 2.2 within 90 days). We mitigate by (a) keeping deal-creation an *additive* lane alongside existing static deals, not a replacement; (b) versioning everything we publish in `agents.json`; (c) the `agentic/` umbrella means a v2.x bump is a single-package refresh, not a cross-cutting refactor.

---

## 2. Goals & Non-Goals

### 2.1 Goals (Phase 2 — this cycle)

- **G1.** Stand up an inbound `RTBExtensionPoint` gRPC server (`agentic/inbound/`) that accepts mutations from authenticated buyer agents and reuses the Phase 1 `Applier` in the inverse direction. Default-off behind `AGENTIC_INBOUND_ENABLED=false`.
- **G2.** Implement programmatic deal creation per the AAMP 2.0 OpenDirect 2.1 spec. New gRPC service `agentic/gen/iabtechlab/opendirect/v2/`. Persists to a new `deals` table joined to the existing `slot_bidder_configs` chain.
- **G3.** AdCOM content/audience taxonomy mapping. Translate AdCOM categories ↔ our internal taxonomy + segment IDs. Use IAB Content Taxonomy 3.0 as canonical at the boundary.
- **G4.** mTLS termination on inbound; per-buyer cert pinning; reject any caller not in our allow-list AND not present in the IAB Tools Portal Agent Registry's verified seller-eligible list.
- **G5.** Per-deal take-rate accounting. New `take_rate_pct` column on `deals`; revenue attributed to the curator on win; existing `applyBidMultiplier` extended to honour curator-specific margins.
- **G6.** Publisher opt-in surface — extend `publishers_new` with `curator_deals_enabled` (bool) and `curator_allowlist` (JSONB). Default off; ops flips per publisher.
- **G7.** Self-register Catalyst on the IAB Tools Portal Agent Registry as a Seller Agent declaring our capabilities, OpenDirect 2.1 support, lifecycles, and intent set.
- **G8.** Ship a Curator Integration Kit: Go + Node.js client SDK with mTLS helpers, reference "discover → create-deal → activate → bid" flow, sandbox endpoint URLs, and an onboarding doc at `agentic/docs/curator-onboarding.md`.
- **G9.** Audit + telemetry: every inbound call and every deal lifecycle transition emits a structured log line (`evt: agentic.inbound.call` and `evt: agentic.deal.lifecycle`).
- **G10.** First curator partner integrated end-to-end on staging; ≥ 1 USD in test-mode revenue captured against a synthetic auction.

### 2.2 Stretch goals (this cycle if time permits)

- **S1.** Curator-facing dashboard (read-only) at `/admin/curator-deals` showing live deal count, fill rate, GPM by curator.
- **S2.** Content taxonomy auto-mapping using existing IAB taxonomy lookup (no manual curator-side mapping required for the first deal).

### 2.3 Non-Goals

- **N1.** Buyer Agent SDK — we are a seller, not a buyer.
- **N2.** Agentic Audiences (cross-agent audience handoff per AAMP 2.0). Phase 3 — needs more than one curator live first.
- **N3.** AAMP 3.0 features. In flight per IAB; not frozen.
- **N4.** MCP transport. Phase 3 if at all; LLM-prompt-injection surface still unevaluated.
- **N5.** Publishing the prebid adapter to the prebid.js org repo. Independent decision.
- **N6.** Replacing static PMP deals. Curator deals are an additive lane.
- **N7.** Direct Sold campaign automation (AAMP 2.0 Buyer Agent feature; not relevant to our seller side).
- **N8.** Self-service curator onboarding UI. Phase 2C ships docs + SDK; UI follows revenue.
- **N9.** Multi-region deal-store replication. Single-master Postgres for Phase 2; geo-replication is a separate piece of work.

---

## 3. Background — AAMP 2.0 As of 2026-04

### 3.1 Release shape

**AAMP 2.0** released 2026-04-26 by IAB Tech Lab. Builds on the Phase-1 vocabulary (Originator, Mutation, Intent, Lifecycle from ARTF v1.0) and adds:

| Component | Status (Apr 2026) | Our scope |
|---|---|---|
| Buyer Agent SDK 2.0 | Frozen | Out (we're a seller) |
| Seller Agent SDK | Frozen | **In** (Phase 2A, 2B) |
| OpenDirect 2.1 agentification | Frozen | **In** (Phase 2B) |
| AdCOM 1.x integration | Frozen | **In** (Phase 2B mapping) |
| IAB Tools Portal Agent Registry | Live since 2026-03-01 | **In** (Phase 2C self-registration) |
| Agentic Audiences | Frozen | Out (Phase 3) |
| AAMP 3.0 transaction/media types | In flight | Out |

### 3.2 What "Seller Agent SDK" means concretely

The Seller Agent SDK natively wires three IAB standards into a single agent surface:
- **OpenDirect 2.1** — programmatic deal creation, activation, and lifecycle
- **AdCOM** — typed ad-product, audience, and content category objects on deals
- **IAB Content Taxonomy 3.0** — canonical category vocabulary

A buyer agent's "create deal" call therefore carries AdCOM-typed objects with IAB Content Taxonomy 3.0 IDs, not bespoke vendor strings. We translate at the boundary.

### 3.3 The IAB Tools Portal Agent Registry

A directory hosted by IAB Tech Lab where agents register their capabilities, certs, and trust signals. AAMP 2.0 buyer agents query the registry to discover seller agents. Live since 2026-03-01. We're not registered yet — Phase 2C.

### 3.4 What AAMP 2.0 does not specify

- **Take-rate / settlement** — left to bilateral commercial agreements. Our approach: per-deal `take_rate_pct` in the `deals` table.
- **Curator vs. buyer-agent identity disambiguation** — the spec treats a curator as just another `Originator{TYPE_DSP}` from our perspective. We add a TNE-internal `curator_id` field on the deal record for revenue attribution.
- **Publisher opt-in mechanics** — we own this UX; new columns on `publishers_new`.

### 3.5 Production reference points

- **PMG Alli Operating System** integrated AAMP standards (PRNewswire, late April 2026). First major buyer-side AAMP integration; gives us a counterparty to point at for "this is real, not vapor."

### 3.6 Why Phase 2 now and not later

- The spec is frozen. Waiting buys nothing technically and costs commercially.
- One major buyer-side platform is live. Two more rumoured for Q3.
- Curator partners we already know want to integrate but have nowhere to integrate *to*.

---

## 4. Personas & User Stories

### 4.1 Curator Partner (Phase 2 primary user)
> *"As a curator who packages audiences and brings buyer demand, I want to programmatically create deals on Catalyst inventory matching my audience criteria, activate them at machine speed, and route my buyer-agent traffic to bid into them — all without bilateral integration work for each deal."*

**Acceptance:** Curator's agent can discover Catalyst via the IAB Registry, exchange capability manifests, call `OpenDirect.CreateDeal` with an AdCOM payload, receive a `deal_id`, call `OpenDirect.ActivateDeal`, then send `RTBRequest` mutations referencing that deal — all over a single mTLS-authenticated gRPC connection, with structured errors on rejection.

### 4.2 Buyer Agent Operator (DSP-side, opportunistic)
> *"As a buyer running an LLM-driven DSP that already speaks AAMP 2.0, I want to discover Catalyst via the standard registry handshake and bid into deals (mine or curators') without bespoke integration."*

**Acceptance (Phase 2A):** Buyer-agent calls trip our inbound `RTBExtensionPoint`, get authenticated via mTLS + cert pinning, return appropriate mutations or 401 with a clear reason. Same path serves curators and direct DSPs — the curator distinction is internal.

### 4.3 Publisher
> *"As a publisher, I want explicit opt-in to curator deals, control over which curators can sell my inventory, and visibility into the take-rate split."*

**Acceptance:** `publishers_new.curator_deals_enabled` defaults to `false`. Ops flip on a per-publisher basis. `publishers_new.curator_allowlist` accepts an explicit list of curator IDs; deals from non-allowlisted curators are rejected with `curator_not_authorised`. Per-deal take-rate is visible in the publisher admin view.

### 4.4 Internal Ops / Revenue Analyst
> *"As an analyst, I want to see, per curator and per deal, how much demand they brought, what fill rate we hit, and what take-rate we captured."*

**Acceptance:** New `/admin/curator-deals` endpoint returns a JSON projection: `[{deal_id, curator_id, publisher_id, win_rate, gpm_usd, take_rate_pct, status, created_at, last_bid_at}]`. Per-curator rollup: Prometheus metric `agentic_deal_gpm_usd_total{curator_id, publisher_id}`.

### 4.5 Compliance / Privacy Lead
> *"As compliance, I need curator-injected audiences subject to the same TCF/GPP/COPPA gating as our own segments, and per-deal audit trail showing every audience source."*

**Acceptance:** When a curator deal carries an AdCOM audience, the existing `DeriveAgentConsent` gate runs unchanged. COPPA hard-blocks any deal with audience targeting. Every deal-creation event logs `evt: agentic.deal.lifecycle` with `audience_source`, `audience_categories`, `consent_decision`. mTLS cert chain captured per deal for forensics.

### 4.6 Product / Commercial Lead
> *"As product, I want a count of curator deals live, GPM week-over-week, and time-to-onboard a new curator under 1 hour after the first."*

**Acceptance:** Phase 2 closure metric (§14): one curator live on staging, ≥ 1 USD test-mode GPM, second curator onboarded entirely from `curator-onboarding.md` with no engineer touch.

---

## 5. Functional Requirements — Inbound Seller Agent gRPC server

A new sub-package `agentic/inbound/` implements the server-side mirror of Phase 1's outbound `Client`. Reuses `agentic.Applier`, `agentic.Registry`, `agentic.OriginatorStamper` as-is — only the surface is new.

### 5.1 RTBExtensionPoint server

```go
// agentic/inbound/server.go
type Server struct {
    applier  *agentic.Applier
    registry *agentic.Registry
    deals    *deal.Store
    auth     *Authenticator
    cfg      ServerConfig
}

func NewServer(cfg ServerConfig, applier *agentic.Applier, registry *agentic.Registry, deals *deal.Store, auth *Authenticator) *Server
func (s *Server) GetMutations(ctx context.Context, req *pb.RTBRequest) (*pb.RTBResponse, error)
func (s *Server) Serve(lis net.Listener) error
func (s *Server) Stop() error
```

Behavioural requirements:

- **R5.1.1 Listens on `:50051`** by default (configurable via `AGENTIC_INBOUND_GRPC_PORT`).
- **R5.1.2 Authentication-first.** Every call passes through `Authenticator.Verify(ctx) → AgentIdentity` *before* any business logic. Failed auth → gRPC `Unauthenticated` with a structured error code, no leak of registry contents.
- **R5.1.3 Reuses the Phase 1 Applier in inverse direction.** When a buyer agent submits mutations, we run them through `agentic.Applier.Apply` exactly as Phase 1 does for outbound results — same whitelist, same lifecycle gates, same path validation, same audit log lines.
- **R5.1.4 Originator stamping.** Inbound `RTBRequest.Originator` MUST be `TYPE_DSP` (or `TYPE_PUBLISHER` for legacy curator clients). `TYPE_SSP` is rejected — only we emit that, and only outbound.
- **R5.1.5 Deals-aware.** When the inbound mutation references a deal (`ACTIVATE_DEALS`, `SUPPRESS_DEALS`, `ADJUST_DEAL_FLOOR`), the deal ID must exist in our `deals` table AND the calling agent must be authorised on that deal — see R5.1.10.
- **R5.1.6 Defensive panic recovery.** Every `GetMutations` call is wrapped in `defer recover()` so a misbehaving caller cannot tear down the server; recovered panics emit `evt: agentic.inbound.panic` with the caller's `agent_id`.
- **R5.1.7 No mutation when disabled.** If `AGENTIC_INBOUND_ENABLED=false`, `Serve` returns immediately and the gRPC port is never bound.
- **R5.1.8 Streaming and unary parity.** Phase 2 ships unary only; bidirectional streaming reserved for Phase 3.
- **R5.1.9 Max message size 4 MiB inbound.** Match outbound. Larger payloads rejected `ResourceExhausted`.
- **R5.1.10 Per-deal authorisation.** The `Authenticator` resolves the calling agent's authorised deal IDs once per call; mutations referencing deals outside that set are rejected `not_authorised_for_deal`.
- **R5.1.11 Idempotency.** Each `RTBRequest.id` is treated as an idempotency key; a duplicate ID within a 60-second window returns the cached `RTBResponse` without re-running the applier.

### 5.2 mTLS termination

- **R5.2.1 mTLS only.** No plain gRPC accepted on the inbound port, regardless of `AGENTIC_ALLOW_INSECURE` (which is outbound-only).
- **R5.2.2 Per-buyer cert pinning.** The CA bundle at `AGENTIC_MTLS_CA_PATH` is the trust anchor; client cert SPKI fingerprint is pinned per buyer in the deal-store.
- **R5.2.3 Cert rotation.** Buyers can pre-register a successor SPKI fingerprint via the admin API; both fingerprints are accepted during a 30-day rotation window.
- **R5.2.4 Termination at the application** (gRPC server) **not at nginx**. Nginx forwards TLS unmodified so SPKI pinning works against the original cert. Document required nginx config in `agentic/docs/inbound-mtls-deploy.md`.
- **R5.2.5 ALPN `h2`.** Reject anything else.

### 5.3 Inbound auth + registry cross-check

`Authenticator` is the auth+authorisation primitive for inbound. Constructed from a static allow-list (DB-backed) plus a periodic refresh of the IAB Tools Portal Agent Registry.

```go
// agentic/inbound/auth.go
type AgentIdentity struct {
    AgentID         string
    AgentType       pb.Originator_Type   // TYPE_DSP | TYPE_PUBLISHER
    AuthorisedDeals []string
    SPKIFingerprint string
    RegistryVerified bool
}

type Authenticator interface {
    Verify(ctx context.Context) (*AgentIdentity, error)
    RefreshRegistry(ctx context.Context) error
}
```

Behaviour:

- **R5.3.1 Two-layer trust.** Caller cert MUST verify against our CA AND the SPKI fingerprint MUST match an entry in our local trusted-buyer table. Either alone is insufficient.
- **R5.3.2 Registry cross-check.** Caller's `agent_id` MUST also exist in the IAB Tools Portal Agent Registry's verified-seller-eligible list. Refreshed every `AGENTIC_REGISTRY_REFRESH_S` seconds (default 3600). On registry fetch failure, fall back to last-known-good cache; alert `agentic_registry_refresh_failed_total`.
- **R5.3.3 Authorised deal resolution.** On successful auth, the `Authenticator` queries `deal_store.AuthorisedDealsFor(agent_id)` and stamps the result on the `AgentIdentity` for downstream R5.1.10 enforcement.
- **R5.3.4 Audit on reject.** Every auth failure logs `evt: agentic.inbound.auth_failed` with `reason`, `caller_dn`, `caller_spki`, `failed_at_stage` (cert | spki | registry | dealset).

### 5.4 Rate limiting + breaker per buyer agent

- **R5.4.1 Per-agent QPS cap.** Per `agent_id`: 1000 QPS soft cap, 5000 hard cap. Excess returns `ResourceExhausted` with `Retry-After` metadata.
- **R5.4.2 Reuse the Phase 1 breaker.** A `idr.CircuitBreaker` keyed by `agent_id` opens after 50 failures in 60 seconds. When open, `GetMutations` returns `Unavailable` with `circuit_open` reason; agent is expected to back off.
- **R5.4.3 Per-publisher cap.** Aggregate inbound mutations affecting a publisher: 200 QPS soft cap. Prevents a single curator from saturating one publisher.

### 5.5 Telemetry

Extends Phase 1's structured log lines with `direction` and `caller_agent_id`:

```json
{
  "ts": "...",
  "evt": "agentic.inbound.call",
  "direction": "inbound",
  "caller_agent_id": "curator.example.com",
  "lifecycle": "PUBLISHER_BID_REQUEST",
  "auth_status": "ok",
  "mutation_count": 3,
  "deal_id": "deal-abc",
  "latency_ms": 12,
  "publisher_id": "9131"
}
```

Prometheus metrics (new):

| Metric | Type | Labels |
|---|---|---|
| `agentic_inbound_call_duration_seconds` | Histogram | `caller_agent_id`, `lifecycle`, `status` |
| `agentic_inbound_mutation_total` | Counter | `caller_agent_id`, `intent`, `decision` |
| `agentic_inbound_auth_failed_total` | Counter | `caller_agent_id`, `stage` |
| `agentic_registry_refresh_failed_total` | Counter | (none) |

---

## 6. Functional Requirements — Programmatic Deal Creation Lane

A new gRPC service `OpenDirect/v2` accepts curator deal-create / activate / lifecycle calls. Persists to a new `deals` table joined to the existing `slot_bidder_configs` chain. **This is the revenue surface.**

### 6.1 Agentic OpenDirect 2.1 endpoint

```protobuf
service OpenDirect {
  rpc CreateDeal   (CreateDealRequest)   returns (CreateDealResponse);
  rpc ActivateDeal (ActivateDealRequest) returns (ActivateDealResponse);
  rpc SuspendDeal  (SuspendDealRequest)  returns (SuspendDealResponse);
  rpc TerminateDeal(TerminateDealRequest) returns (TerminateDealResponse);
  rpc DescribeDeal (DescribeDealRequest)  returns (Deal);
}
```

Behavioural requirements:

- **R6.1.1 Same auth as RTBExtensionPoint.** mTLS + SPKI pinning + Registry cross-check (§5.3). Curators authenticate identically; this is just a different RPC family on the same server.
- **R6.1.2 Idempotent CreateDeal.** Curator supplies `external_deal_id`; we look up `(curator_id, external_deal_id)` first and return the existing record on collision instead of creating a duplicate.
- **R6.1.3 Synchronous validation.** `CreateDeal` returns within 200 ms in the happy case. Validation includes: AdCOM payload well-formed, audience categories resolve, target publishers exist + opted in, take-rate within the configured `[0, 50]%` band, schedule consistent (start ≤ end).
- **R6.1.4 Asynchronous activation budget.** `ActivateDeal` may take up to 5 seconds: it must (a) re-validate publisher opt-in (curator may have been delisted between create and activate), (b) propagate to in-memory deal cache, (c) emit a synthetic `slot_bidder_configs` row tying the deal to bidder routing.
- **R6.1.5 Conflict resolution with static deals.** When a static deal and a curator deal target the same `(publisher_id, ad_unit_id)`, the static deal wins by default. Override per publisher via `publishers_new.curator_priority_over_static`. Document the precedence in the deal response.
- **R6.1.6 Schedule enforcement.** Deals outside their `schedule.start_at..end_at` window are auto-suspended by a periodic scanner (every 60 seconds). Logged as `evt: agentic.deal.lifecycle, reason: schedule_expired`.
- **R6.1.7 Capacity caps.** Per curator: 1000 active deals max. Per publisher: 500 active curator deals max. Excess returns `FailedPrecondition` with `capacity_exceeded`.

### 6.2 Deal lifecycle

| State | Entry | Exit | Bidding effect |
|---|---|---|---|
| `DRAFT` | `CreateDeal` | `ActivateDeal` → `ACTIVE` | Not eligible for auctions |
| `ACTIVE` | `ActivateDeal` | `SuspendDeal` → `SUSPENDED`, schedule expiry → `EXPIRED`, `TerminateDeal` → `TERMINATED` | Eligible; curator buyer-agent can route bids |
| `SUSPENDED` | `SuspendDeal` | `ActivateDeal` → `ACTIVE`, `TerminateDeal` → `TERMINATED` | Not eligible; recoverable |
| `EXPIRED` | Schedule expiry | (terminal — recover via new deal) | Not eligible |
| `TERMINATED` | `TerminateDeal` | (terminal) | Not eligible; audit retained 90 days |

Every transition emits `evt: agentic.deal.lifecycle` with `from`, `to`, `reason`, `actor_agent_id`. Persisted to `deal_events` (append-only).

### 6.3 AdCOM content/audience taxonomy mapping

A new `agentic/adcom/` package translates AdCOM v1.x objects ↔ our internal types.

- **R6.3.1 IAB Content Taxonomy 3.0 canonical.** Inbound AdCOM `cattax=7` (IAB 3.0) is the reference. Other taxonomy versions are accepted but mapped to 3.0 IDs at the boundary; mapping table in `agentic/adcom/taxonomy_map.go`.
- **R6.3.2 Audience translation.** AdCOM `Audience` objects (typed segments + provider IDs) map to our internal `openrtb.Data{ID, Segment[]}` structure. Segment IDs are NOT modified; we only annotate `Data.ID = "curator:<curator_id>"` for audit.
- **R6.3.3 Ad-product validation.** Curator-declared ad-product MUST be one of our supported types (banner, native, video, audio). Unsupported types reject the deal at create time.
- **R6.3.4 Round-trip.** Outbound bid responses to the curator's buyer-agent carry the original AdCOM IDs intact, not our internal IDs. This means we keep curator-supplied IDs in the deal record.

### 6.4 Take-rate accounting

- **R6.4.1 Per-deal `take_rate_pct` column.** Decimal, range `[0, 0.50]`. Set at create time; immutable thereafter (use `TerminateDeal` + new `CreateDeal` to change).
- **R6.4.2 Revenue attribution on win.** When a bid wins a curator deal, the existing `applyBidMultiplier` (line ~1132 in exchange.go) reads the `take_rate_pct` from the winning deal and emits a settlement event:
  ```json
  {
    "evt": "agentic.deal.settlement",
    "deal_id": "deal-abc",
    "curator_id": "curator.example.com",
    "publisher_id": "9131",
    "winning_price": 1.50,
    "take_rate_pct": 0.15,
    "tne_revenue_usd": 0.225,
    "publisher_revenue_usd": 1.275,
    "auction_id": "auction-xyz"
  }
  ```
- **R6.4.3 Postgres column on `bids` ledger.** Existing `bids` event table gains a nullable `deal_id` FK so the analytics warehouse can roll up by deal/curator without a new table join.
- **R6.4.4 Daily reconciliation.** A nightly job aggregates settlement events into `curator_revenue_daily` (`curator_id`, `publisher_id`, `date`, `gpm_usd`, `bid_count`). Powers the `/admin/curator-deals` dashboard and any external invoicing.
- **R6.4.5 Currency.** All take-rate in USD initially. Multi-currency reserved for Phase 3.
- **R6.4.6 Negative-margin guard.** If `winning_price * take_rate_pct < 0.0001 USD`, the settlement event is suppressed (prevents flooding analytics with sub-microsecond noise on test traffic).

### 6.5 Publisher opt-in / opt-out

- **R6.5.1 Default off.** New publishers start with `curator_deals_enabled=false`. Existing publishers unaffected; ops flips per publisher.
- **R6.5.2 Per-curator allow-list.** `publishers_new.curator_allowlist` is a JSONB column with shape:
  ```json
  {"include": ["curator-1", "curator-2"], "exclude": [], "default_take_rate_max": 0.20}
  ```
  Empty `include` AND empty `exclude` → publisher accepts no curator deals (must explicitly include). `exclude` overrides `include` (defensive).
- **R6.5.3 Take-rate ceiling per publisher.** `default_take_rate_max` caps the take-rate any deal on this publisher can declare. Excess rejected at `CreateDeal` with `take_rate_exceeds_publisher_cap`.
- **R6.5.4 Live revocation.** When a publisher removes a curator from `curator_allowlist`, all that curator's `ACTIVE` deals on that publisher transition to `SUSPENDED` within 60 seconds (next periodic scan). Logged with reason `publisher_revoked`.
- **R6.5.5 Admin endpoints.** `PUT /admin/publishers/{id}/curator-allowlist` (adminAuth) replaces the JSONB document. `GET` returns it. Audit-logged.

---

## 7. Functional Requirements — Agent Registry + Trust

### 7.1 IAB Tools Portal self-registration

- **R7.1.1 One-time registration.** Submit Catalyst's seller agent record to the IAB Tools Portal Agent Registry via the standard form. Required fields: `seller_id` (9131), `seller_domain` (thenexusengine.com), `endpoints` (with mTLS cert chain), `capabilities`, supported `intents`, supported `lifecycles`, OpenDirect version (2.1), AdCOM version, IAB Content Taxonomy version (3.0).
- **R7.1.2 Cert chain published.** Our public TLS cert chain MUST be included so buyer-agents can pin against it without out-of-band exchange.
- **R7.1.3 Manual update procedure.** Capability changes (e.g. adding a new intent) trigger a registry update via a documented manual flow in `agentic/docs/registry-update-procedure.md`. We do not auto-push.
- **R7.1.4 Verified-seller status.** The Registry has a "verified" status flag granted after IAB review. Target ≥ "verified" within 30 days of submission.

### 7.2 Trust-signal publishing in `agents.json`

Extend the Phase 1 `/.well-known/agents.json` document with AAMP 2.0 capability fields:

```json
{
  "version": "2.0",
  "seller_id": "9131",
  "seller_domain": "thenexusengine.com",
  "registry_ref": "iab-tools-portal:reg-XXXXX",
  "capabilities": {
    "transports": ["grpc", "grpcs"],
    "intents_inbound":  ["ACTIVATE_SEGMENTS", "ACTIVATE_DEALS", "SUPPRESS_DEALS",
                         "ADJUST_DEAL_FLOOR", "ADJUST_DEAL_MARGIN", "BID_SHADE", "ADD_METRICS"],
    "intents_outbound": ["ACTIVATE_SEGMENTS", "ADJUST_DEAL_FLOOR", "BID_SHADE"],
    "lifecycles": ["PUBLISHER_BID_REQUEST", "DSP_BID_RESPONSE"],
    "opendirect_version": "2.1",
    "adcom_version": "1.0",
    "content_taxonomy_version": "3.0",
    "deal_types_accepted": ["programmatic_pmp", "programmatic_guaranteed"]
  },
  "media_kits": [
    {"id": "catalyst-display-2026q2", "url": "https://thenexusengine.com/media/display.json"}
  ],
  "product_catalogs": [
    {"id": "catalyst-products-2026q2", "url": "https://thenexusengine.com/media/products.json"}
  ],
  "agents": [ /* Phase 1 entries — extension-point agents we call out to */ ]
}
```

- **R7.2.1 Document version bump to `"2.0"`.** Phase 1 served `"1.0"`. Bump signals AAMP 2.0 capability set.
- **R7.2.2 Schema validated at boot.** Existing `agents.schema.json` extended; CI validates on every push.
- **R7.2.3 Backwards compat.** Existing Phase 1 agent entries continue to work unchanged — capabilities object is additive.

### 7.3 Buyer-agent discovery handshake

A new gRPC service `Discovery` exposed on the same `:50051` port:

```protobuf
service Discovery {
  rpc DescribeCapabilities (DescribeCapabilitiesRequest) returns (Capabilities);
  rpc ListMediaKits        (ListMediaKitsRequest)       returns (MediaKitList);
  rpc ListProductCatalogs  (ListProductCatalogsRequest) returns (ProductCatalogList);
}
```

- **R7.3.1 Auth-light.** `Discovery` calls require valid mTLS cert but do not require Registry cross-check or per-deal authorisation — they're discovery, not transaction.
- **R7.3.2 Identical content to `agents.json`.** The `DescribeCapabilities` response is the same data structure served at `/.well-known/agents.json`, just over gRPC. One source of truth (`agentic.Registry.Document()`).
- **R7.3.3 Cache-friendly.** Responses include an `etag` field; clients can cache and short-circuit refreshes.

---

## 8. Curator Integration Kit

The kit makes curator onboarding documentation-only after the first integration. Lives under `agentic/curator-sdk/`.

### 8.1 Curator SDK (Go + Node.js)

```
agentic/curator-sdk/
  go/
    catalyst/
      client.go            # NewClient(addr, certPath, keyPath, caBundle) → *Client
      discovery.go         # Client.Discover() → Capabilities
      deals.go             # Client.CreateDeal, ActivateDeal, etc.
      bid.go               # Client.SubmitMutation
      examples/
        create_and_bid.go  # ~80 LOC end-to-end smoke
  node/
    src/
      client.ts
      discovery.ts
      deals.ts
      bid.ts
      examples/
        createAndBid.ts
    package.json           # @thenexusengine/catalyst-curator (private:true initially)
  README.md
```

- **R8.1.1 mTLS helpers built in.** `NewClient` takes file paths; cert reload + rotation handled internally.
- **R8.1.2 Generated from same protos.** Go SDK uses our committed `agentic/gen/`; Node SDK regenerated via `protoc` against the same `agentic/proto/`. Documented in `agentic/curator-sdk/README.md` so curators can regenerate against vendor-specific proto patches if required.
- **R8.1.3 Reference implementation of "discover → create deal → activate → bid".** ~80 LOC in each language. Curator copy-pastes, swaps endpoint + cert paths, runs.
- **R8.1.4 Versioned independently.** SDK semver tracks AAMP-spec compatibility, not Catalyst internals. Bump major when AAMP wire spec breaks.

### 8.2 Sequence diagrams

ASCII sequence in `agentic/curator-sdk/SEQUENCE.md` covering five flows:

1. **First-time discovery** — curator → mTLS → `Discovery.DescribeCapabilities` → cache result
2. **Create + activate deal** — `OpenDirect.CreateDeal` → DRAFT → `OpenDirect.ActivateDeal` → ACTIVE
3. **Buyer-agent bidding** — `RTBExtensionPoint.GetMutations` with `ACTIVATE_DEALS` referencing the deal_id
4. **Lifecycle: suspend / re-activate** — for daypart-style scheduling
5. **Settlement readback** — curator polls `OpenDirect.DescribeDeal` to see win count + GPM (read-only; full revenue reconciliation is invoiced bilaterally per R6.4)

### 8.3 Sandbox + smoke kit

- **R8.3.1 Staging endpoint.** `grpcs://catalyst-sandbox.thenexusengine.com:50051` with self-signed CA bundle distributed to curators.
- **R8.3.2 Synthetic publisher.** Pre-seeded test publisher `pub-sandbox-1` with `curator_deals_enabled=true` and a wide-open `curator_allowlist`. Curator can hit deals on it without ops involvement.
- **R8.3.3 Test API keys.** Per-curator mTLS cert pre-issued by ops.
- **R8.3.4 Synthetic auction generator.** A small `tne-loadgen` binary curators can run to fire synthetic bids into their sandbox deal — they see settlement events without needing a real DSP. Tied to `agentic/curator-sdk/loadgen/`.

### 8.4 Onboarding doc

`agentic/docs/curator-onboarding.md` — concrete step-by-step:

1. Curator submits requested cert SPKI fingerprint + agent ID to `agentic-ops@thenexusengine.com`.
2. Ops adds entry to the trusted-buyer table + per-curator API key entry.
3. Curator clones SDK, points at sandbox endpoint, runs `examples/create_and_bid` to validate.
4. Ops + curator review the settlement event log together (smoke).
5. Curator submits target publisher list; ops flips `curator_deals_enabled` per publisher and adds to `curator_allowlist`.
6. Production endpoint swap: `grpcs://catalyst.thenexusengine.com:50051`.
7. Watch agent breaker + GPM metric for 24h.

Target onboarding time after the first curator: **≤ 1 hour from cert receipt to first prod bid** (Phase 2 closure metric).

---

## 9. Data Shapes

### 9.1 Vendored AAMP 2.0 protos

Refresh `agentic/proto/iabtechlab/` from `IABTechLab/AAMP@<commit>` at Phase 2 start. Add three new proto trees:

```
agentic/proto/iabtechlab/
  bidstream/mutation/v1/             # existing — Phase 1
  openrtb/v26/                       # existing — Phase 1
  opendirect/v2/                     # NEW — programmatic deal creation
    opendirect.proto
    opendirectservices.proto
  adcom/v1/                          # NEW — typed audience/content/product objects
    adcom.proto
  aamp/discovery/v1/                 # NEW — Discovery service
    discovery.proto
```

- **R9.1.1 Same vendoring policy as Phase 1.** Pinned upstream SHA, patches documented in `agentic/proto/iabtechlab/README.md`, generated Go committed under `agentic/gen/iabtechlab/`.
- **R9.1.2 Codegen Makefile target updated.** `make generate-protos` extends to cover the new trees.
- **R9.1.3 Forward-compat.** AAMP 2.0 `Capabilities` message reserves `extensions 1000+`; we use the reserved range only via our own `agentic/proto/tne/v2/` tree, never by editing upstream.

### 9.2 Deal model + Postgres migration

New migration `deployment/migrations/020_curator_deals.sql`:

```sql
CREATE TABLE deals (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_deal_id    TEXT NOT NULL,                     -- curator-supplied ID
    curator_id          TEXT NOT NULL,                      -- agent_id of the curator
    publisher_id        TEXT NOT NULL REFERENCES publishers_new(id),
    state               TEXT NOT NULL CHECK (state IN ('DRAFT','ACTIVE','SUSPENDED','EXPIRED','TERMINATED')),
    take_rate_pct       NUMERIC(5,4) NOT NULL CHECK (take_rate_pct BETWEEN 0 AND 0.50),
    floor_usd           NUMERIC(8,4) NOT NULL CHECK (floor_usd >= 0),
    audience            JSONB NOT NULL,                    -- AdCOM Audience object, verbatim
    ad_products         JSONB NOT NULL,                    -- AdCOM AdProduct[] objects, verbatim
    schedule_start_at   TIMESTAMPTZ NOT NULL,
    schedule_end_at     TIMESTAMPTZ NOT NULL,
    spki_fingerprint    TEXT NOT NULL,                     -- caller cert at create time
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (curator_id, external_deal_id)
);

CREATE INDEX deals_publisher_active ON deals (publisher_id, state) WHERE state = 'ACTIVE';
CREATE INDEX deals_curator         ON deals (curator_id);
CREATE INDEX deals_schedule_end    ON deals (schedule_end_at) WHERE state = 'ACTIVE';

CREATE TABLE deal_events (
    id                  BIGSERIAL PRIMARY KEY,
    deal_id             UUID NOT NULL REFERENCES deals(id) ON DELETE CASCADE,
    from_state          TEXT,
    to_state            TEXT NOT NULL,
    reason              TEXT NOT NULL,
    actor_agent_id      TEXT,
    payload             JSONB,                             -- structured details
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX deal_events_deal ON deal_events (deal_id, created_at DESC);

ALTER TABLE publishers_new
    ADD COLUMN curator_deals_enabled       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN curator_allowlist           JSONB,
    ADD COLUMN curator_priority_over_static BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE bids
    ADD COLUMN deal_id UUID REFERENCES deals(id);

CREATE TABLE curator_revenue_daily (
    curator_id   TEXT NOT NULL,
    publisher_id TEXT NOT NULL,
    date         DATE NOT NULL,
    gpm_usd      NUMERIC(12,4) NOT NULL DEFAULT 0,
    bid_count    BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (curator_id, publisher_id, date)
);
```

### 9.3 Capability manifest (extends agents.json)

Per §7.2 — version bumped to `"2.0"`, capabilities + media_kits + product_catalogs added. Schema validated at boot via `agents.schema.json`; CI fails the build if the doc doesn't match. Phase 1 agents.json stays valid against the v2 schema (additive change).

### 9.4 Go types (`agentic/inbound/`, `agentic/deal/`, `agentic/adcom/`)

```
agentic/
  inbound/
    server.go         # Server + GetMutations + Discovery handlers
    auth.go           # Authenticator + AgentIdentity + Registry refresh
    ratelimit.go      # per-agent QPS + breaker glue
    server_test.go
    auth_test.go
  deal/
    store.go          # Postgres-backed CRUD; AuthorisedDealsFor lookup
    state.go          # State machine: DRAFT → ACTIVE → SUSPENDED/EXPIRED/TERMINATED
    scheduler.go      # 60s scanner: schedule_expired, publisher_revoked
    settlement.go     # win-event hook → curator_revenue_daily
    store_test.go
    state_test.go
  adcom/
    types.go          # Go shapes mirroring AdCOM proto + JSON helpers
    taxonomy_map.go   # IAB taxonomy version → 3.0 mapping
    audience.go       # AdCOM.Audience ↔ openrtb.Data conversion
    types_test.go
  curator-sdk/        # see §8
```

---

## 10. API Surface

### 10.1 New gRPC endpoints

Listening on `:50051` (configurable). All require mTLS.

| Service | RPC | Purpose | Auth class |
|---|---|---|---|
| `RTBExtensionPoint` | `GetMutations` | Inbound mutation submission | mTLS + Registry + per-deal authz |
| `OpenDirect` | `CreateDeal` | Programmatic deal creation | mTLS + Registry |
| `OpenDirect` | `ActivateDeal` | Move DRAFT → ACTIVE | mTLS + Registry + deal owner |
| `OpenDirect` | `SuspendDeal` | Move ACTIVE → SUSPENDED | mTLS + Registry + deal owner |
| `OpenDirect` | `TerminateDeal` | Final state | mTLS + Registry + deal owner |
| `OpenDirect` | `DescribeDeal` | Read-only | mTLS + Registry |
| `Discovery` | `DescribeCapabilities` | Capability manifest | mTLS only |
| `Discovery` | `ListMediaKits` | Media kit URLs | mTLS only |
| `Discovery` | `ListProductCatalogs` | Product catalog URLs | mTLS only |

### 10.2 New HTTP routes

| Method | Path | Handler | Auth |
|---|---|---|---|
| GET | `/admin/curator-deals` | List active curator deals (JSON) | adminAuth |
| GET | `/admin/curator-deals/{id}` | Single deal projection | adminAuth |
| GET | `/admin/curator-deals/by-curator/{curator_id}` | Per-curator rollup | adminAuth |
| GET | `/admin/curator-revenue/{curator_id}` | Daily GPM rollup | adminAuth |
| PUT | `/admin/publishers/{id}/curator-allowlist` | Set per-publisher allow-list | adminAuth |
| GET | `/admin/publishers/{id}/curator-allowlist` | Read | adminAuth |

`/.well-known/agents.json` and `/agents.json` continue serving the same handler; document content extended per §7.2.

### 10.3 New env vars

| Var | Default | Meaning |
|---|---|---|
| `AGENTIC_INBOUND_ENABLED` | `false` | Master switch for the inbound `:50051` server. |
| `AGENTIC_INBOUND_GRPC_PORT` | `50051` | Listener port. |
| `AGENTIC_MTLS_CA_PATH` | `""` | Path to CA bundle for client cert verification. Required when `INBOUND_ENABLED=true`. |
| `AGENTIC_MTLS_CERT_PATH` | `""` | Our server cert. Required. |
| `AGENTIC_MTLS_KEY_PATH` | `""` | Our server key. Required. |
| `AGENTIC_OPENDIRECT_ENABLED` | `false` | OpenDirect deal-creation lane. Independent of `INBOUND_ENABLED` so we can ship Phase 2A without 2B. |
| `AGENTIC_REGISTRY_URL` | `https://toolsportal.iabtechlab.com/registry/v1` | Registry endpoint. |
| `AGENTIC_REGISTRY_REFRESH_S` | `3600` | Refresh interval. |
| `AGENTIC_DEAL_SCANNER_INTERVAL_S` | `60` | Schedule expiry scanner cadence. |
| `AGENTIC_INBOUND_QPS_PER_AGENT` | `1000` | Soft cap. |
| `AGENTIC_INBOUND_QPS_PER_PUBLISHER` | `200` | Soft cap. |

### 10.4 New DB columns + tables

Per §9.2 migration `020_curator_deals.sql`. Reversible via `020_curator_deals.down.sql` (drops the new tables + columns; existing static-deal traffic unaffected).

### 10.5 Admin UI hooks

Phase 2 stretch goal (S1) extends the existing onboarding admin SPA with a "Curator Deals" tab. Phase 2 ships JSON endpoints only; UI follows revenue.

### 10.6 Backwards-compatible HTTP behaviour

- When `AGENTIC_INBOUND_ENABLED=false`: gRPC server doesn't bind; curator-deals admin routes return 404; `/agents.json` serves the v2 document but with `capabilities.intents_inbound = []` so buyer-agents know not to attempt inbound calls.
- When `AGENTIC_OPENDIRECT_ENABLED=false`: `OpenDirect/*` RPCs return `Unimplemented`; `RTBExtensionPoint` still serves; deal-create admin routes 404.

---

## 11. Architecture

### 11.1 End-to-end flow

```
                Curator buyer-agent (e.g. PMG Alli, or our own friends)
                          │
                          │ mTLS gRPC over :50051
                          ▼
              ┌──────────────────────────────────┐
              │  nginx (TLS pass-through)        │
              └──────────────┬───────────────────┘
                             │
                             ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │  agentic/inbound/Server                                          │
   │                                                                  │
   │  ┌─────────────────────────────┐  ┌────────────────────────┐    │
   │  │ Discovery service            │  │ Authenticator           │    │
   │  │   DescribeCapabilities       │◄─┤  - mTLS verify          │    │
   │  │   ListMediaKits              │  │  - SPKI pin             │    │
   │  │   ListProductCatalogs        │  │  - Registry cross-check │    │
   │  └─────────────────────────────┘  │  - AuthorisedDealsFor   │    │
   │                                    └─────────────┬──────────┘    │
   │  ┌─────────────────────────────┐                 │               │
   │  │ OpenDirect/v2 service        │◄────────────────┤               │
   │  │   CreateDeal / ActivateDeal  │                 │               │
   │  │   SuspendDeal / Terminate    │  agentic/deal/  │               │
   │  │   DescribeDeal               │──► Store + State│               │
   │  └─────────────────────────────┘                 │               │
   │                                                   │               │
   │  ┌─────────────────────────────┐                 │               │
   │  │ RTBExtensionPoint service    │◄────────────────┘               │
   │  │   GetMutations (inbound)     │                                 │
   │  └────────────┬────────────────┘                                 │
   └────────────────┼────────────────────────────────────────────────┘
                   │
                   │ Reuse Phase 1 components ↓
                   ▼
        ┌──────────────────────┐    ┌─────────────────────┐
        │ agentic.Applier      │    │ agentic.Registry    │
        │ (whitelist intents)  │    │ (deal lookup)       │
        └──────────┬───────────┘    └─────────────────────┘
                   │
                   ▼
        ┌──────────────────────────────────────┐
        │ exchange.Exchange (existing)         │
        │   - Hook A & B from Phase 1 still    │
        │     fire on outbound                 │
        │   - applyBidMultiplier reads         │
        │     deal.take_rate_pct on win        │
        └──────────────────────────────────────┘

        Postgres:                       Background:
          deals                           deal/scheduler.go (60s scan)
          deal_events                       - schedule_expired
          curator_revenue_daily             - publisher_revoked
          publishers_new (extended)
          bids (extended deal_id col)
```

### 11.2 Hook points in existing code

Three integration files modified — minimal, well-bounded:

| File | Edit | Reason |
|---|---|---|
| `cmd/server/server.go` | New `initAgenticInbound()` after existing `initExchange()` | Construct `agentic/inbound/Server`, bind `:50051`, start. |
| `cmd/server/config.go` | Extend `AgenticConfig` with inbound fields (§10.3) | Same pattern as Phase 1; one struct per phase. |
| `internal/exchange/exchange.go` | Extend `applyBidMultiplier` (line ~1132) to read `deal.take_rate_pct` for curator deals | Settlement event emission per R6.4.2. |

Everything else lives under `agentic/` per the umbrella convention from Phase 1 (PRD §7.0). Net new sub-packages: `agentic/inbound/`, `agentic/deal/`, `agentic/adcom/`, `agentic/curator-sdk/`. Net new generated proto trees: `agentic/gen/iabtechlab/{opendirect,adcom,aamp/discovery}/`.

### 11.3 Reuse of Phase 1 components

| Phase 1 component | Phase 2 reuse | Notes |
|---|---|---|
| `agentic.Applier` | Inbound: applies caller's mutations to in-flight bid request | Same whitelist; same lifecycle gates. |
| `agentic.Registry` | Loaded with `version: "2.0"` doc; serves Discovery + agents.json | One source of truth. |
| `agentic.OriginatorStamper` | Reused on outbound responses to buyer-agents | Stamp `TYPE_SSP` on `RTBResponse.metadata`. |
| `agentic.DeriveAgentConsent` | Inbound consent gate before applying curator mutations | COPPA hard-blocks across both directions. |
| `agentic.Envelope` | Reads inbound `ext.aamp` from buyer-agents | Same shape, `Originator.type=DSP`. |
| `idr.CircuitBreaker` | Per-`agent_id` inbound breaker (§5.4.2) | Same primitive, opposite direction. |
| Phase 1 outbound `Client` | Unchanged | Continues to fan out to extension-point agents at Hook A/B. |

### 11.4 Failure-mode behaviour

| Failure | Effect on auction | Logged |
|---|---|---|
| Inbound auth fail | Connection rejected; no auction impact | `agentic.inbound.auth_failed` |
| Registry refresh fail | Falls back to last-known-good cache | `agentic_registry_refresh_failed_total` |
| Deal-store unavailable | `OpenDirect/*` returns `Unavailable`; existing static-deal traffic unaffected | `evt: agentic.deal.store_unavailable` |
| Curator buyer-agent panics our handler | Recovered via `defer recover()`; agent's call returns `Internal`; auction unaffected | `evt: agentic.inbound.panic` |
| Schedule-expiry scanner fails | Active deals continue; on next successful scan, expired ones transition | `evt: agentic.deal.scanner_failed` |
| Settlement event suppressed (R6.4.6) | Deal still wins; just no analytics row | (silent — by design) |

The auction continues regardless of any agentic failure. Curator deals are an *additive* lane; failure of the lane never affects existing PMP / open-auction traffic.

### 11.5 Concurrency model

- One `agentic/inbound/Server` per Catalyst instance, listens on its own goroutine.
- Per-call goroutine fan-in via gRPC server defaults; no custom worker pool.
- `deal/Store` Postgres reads are pooled via existing `database/sql` connection pool; writes single-row.
- The 60-second `deal/scheduler` runs on a dedicated goroutine with `time.Ticker`; exclusive advisory lock on Postgres so multiple Catalyst instances don't double-fire schedule-expiry events.

---

## 12. Phased Delivery within Phase 2

Phase 2 splits into three independently-shippable sub-phases. Each ends in a deployable state; we can pause between any of them without leaving half-finished work in master.

### 12.1 Phase 2A — Inbound `RTBExtensionPoint` (target: 2 weeks)

**Scope:**
- Refresh ARTF protos from upstream `IABTechLab/AAMP@<commit>`; vendor under `agentic/proto/iabtechlab/aamp/discovery/v1/` (no AAMP-2.0 changes to the existing ARTF tree).
- New sub-package `agentic/inbound/`: `Server`, `Authenticator`, `RateLimit` glue.
- mTLS termination on `:50051` with SPKI pinning; CA bundle from `AGENTIC_MTLS_CA_PATH`.
- Trusted-buyer table (in-memory at first, DB-backed in 2B). Manual ops onboarding.
- Reuses Phase 1 `agentic.Applier` for inbound mutations.
- New Discovery service (`DescribeCapabilities` only — `ListMediaKits` / `ListProductCatalogs` Phase 2C).
- `agents.json` document version bumped to `"2.0"` with new `capabilities` block (no media kits / product catalogs yet — Phase 2C).
- Per-agent QPS cap + breaker.
- Telemetry: `evt: agentic.inbound.{call,auth_failed,panic}`, plus 4 new Prom metrics from §5.5.
- Unit tests on `Server`, `Authenticator`, ratelimit. Integration test: in-process curator sends ACTIVATE_SEGMENTS → Applier writes user.data → assertion mirrors Phase 1's TestIntegration_endToEnd, just inverted.

**Out of scope for 2A:** OpenDirect deal-create RPCs (return `Unimplemented`), AdCOM mapping, IAB Registry self-registration, curator SDK.

**Default state shipped:** `AGENTIC_INBOUND_ENABLED=false`. Opt-in per environment.

**Closure criteria:**
- `go build ./...` clean
- `go test ./agentic/inbound/...` passes
- Staging: real curator's mTLS cert dialled in; ACTIVATE_SEGMENTS flows; no auction-failure regression

### 12.2 Phase 2B — Programmatic deal creation (target: 3 weeks)

**Scope:**
- Vendor AAMP 2.0 OpenDirect 2.1 + AdCOM 1.x protos under `agentic/proto/iabtechlab/{opendirect,adcom}/`.
- New sub-packages: `agentic/deal/` (Store, State, Scheduler, Settlement), `agentic/adcom/` (Types, TaxonomyMap, AudienceMap).
- DB migration `020_curator_deals.sql` (per §9.2) — `deals`, `deal_events`, `curator_revenue_daily` tables; `publishers_new` extensions; `bids.deal_id` column.
- `OpenDirect/v2` gRPC service: `CreateDeal`, `ActivateDeal`, `SuspendDeal`, `TerminateDeal`, `DescribeDeal`.
- Schedule-expiry scanner with Postgres advisory lock (multi-instance safe).
- `applyBidMultiplier` extended to read `deal.take_rate_pct` and emit `agentic.deal.settlement` events on win.
- Admin endpoints: `/admin/curator-deals*`, `/admin/curator-revenue*`, `/admin/publishers/{id}/curator-allowlist`.
- Telemetry: `evt: agentic.deal.{lifecycle,settlement,scanner_failed}`.
- Unit tests on state machine, store, taxonomy mapping, settlement math.
- Integration test: curator creates a deal, activates it, sends a bid via 2A path, wins, settlement event emitted with correct take-rate split.

**Out of scope for 2B:** IAB Registry self-registration, curator SDK packaging (sandbox endpoint URL doesn't exist yet), Agentic Audiences, multi-currency.

**Default state shipped:** `AGENTIC_OPENDIRECT_ENABLED=false`. Independent of `INBOUND_ENABLED`.

**Closure criteria:**
- Migration up + down clean on a copy of prod schema
- One curator partner has a deal live in staging at the end of this phase
- ≥ 1 USD test-mode GPM captured against the synthetic auction

### 12.3 Phase 2C — Registry + curator SDK (target: 2 weeks)

**Scope:**
- IAB Tools Portal Agent Registry submission with our seller-agent record (per §7.1).
- Periodic registry refresh in `Authenticator` (per §5.3.2).
- `Discovery.ListMediaKits` and `ListProductCatalogs` RPCs.
- `agents.json` extended with `media_kits` and `product_catalogs` arrays.
- `agentic/curator-sdk/go/` and `agentic/curator-sdk/node/` SDKs with reference flows.
- `agentic/curator-sdk/loadgen/` synthetic-auction CLI for curator smoke tests.
- Sandbox endpoint stood up at `grpcs://catalyst-sandbox.thenexusengine.com:50051` with self-signed CA bundle distributed to curators.
- `agentic/docs/curator-onboarding.md` written and validated against second-curator onboarding.

**Out of scope for 2C:** Self-service curator onboarding UI, Agentic Audiences, AAMP 3.0 features.

**Closure criteria:**
- Catalyst is `verified` in the IAB Tools Portal Agent Registry (or has submitted; verification timeline depends on IAB)
- Second curator partner onboarded entirely from `curator-onboarding.md` with **no engineering touch**
- Time from cert receipt to first prod bid ≤ 1 hour for the second curator

### 12.4 Rollout strategy

Per sub-phase, the rollout pattern is identical to Phase 1's (which worked):

1. **Day 1–N:** Implementation on a feature branch.
2. **Day N+1:** PR opens. CI must be green.
3. **Day N+2:** Code review.
4. **Day N+3:** Merge to master with feature flag default-off.
5. **Day N+4:** Deploy to staging with flag *on* and zero buyer-agents configured. Verify zero behaviour change to existing auctions.
6. **Day N+5:** Onboard the named curator partner to staging. Watch breaker + GPM for 24 h.
7. **Day N+6:** Prod deploy with flag *off*. Canary verify zero regression.
8. **Day N+7:** Prod flip with flag *on*, single curator allow-listed. Watch 24 h.
9. **Day N+8+:** Onboard additional curators using the documented flow.

### 12.5 Rollback strategy

Same env-var flip story as Phase 1. Each sub-phase is reversible:

| Sub-phase | Rollback action | Data implications |
|---|---|---|
| 2A | `AGENTIC_INBOUND_ENABLED=false` + restart | None — inbound state is in-memory |
| 2B | `AGENTIC_OPENDIRECT_ENABLED=false` + restart; ACTIVE deals freeze in place | DB tables retained (audit); no traffic flows |
| 2C | Withdraw registry entry; ship a `version: "1.0"` agents.json doc | None |

Forward-only DB migrations: `020_curator_deals.sql` is non-destructive (additive columns + new tables only). The `down.sql` exists but is not part of the rollback path — running it would lose audit data. Rolling back 2B in production = flag-flip, not migration-revert.

### 12.6 Branch strategy

Phase 2A, 2B, 2C each get their own branch + PR. The `claude/integrate-iab-agentic-protocol-6bvtJ` branch is reused per session policy where appropriate; new feature branches when the user explicitly authorises.

---

## 13. Risks & Open Questions

### 13.1 Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R1 | AAMP 2.0 wire spec churns within 90 days | High | Med | Curator deals = additive lane (not replacement); proto vendoring isolated to `agentic/`; version everything in `agents.json`. |
| R2 | A misbehaving curator floods us with deal-creates | Med | Med | Per-curator capacity cap (R6.1.7); per-agent QPS cap on inbound (R5.4.1); breaker per curator (R5.4.2). |
| R3 | Curator audience leaks PII via AdCOM payload | Med | High | Same TCF/GPP/COPPA gates as bidder path; segment IDs treated as opaque; never injected into `user.id` / `user.geo` (existing applier path validation). |
| R4 | Settlement events miscount → revenue dispute | Med | High | Idempotency on `RTBRequest.id` (R5.1.11); `bids.deal_id` ledger column for audit; nightly reconciliation job. |
| R5 | mTLS cert rotation fails silently → curator goes offline | Med | Med | 30-day overlap window for SPKI rotation (R5.2.3); breaker auto-detects + alerts. |
| R6 | IAB Tools Portal Registry becomes unavailable | Low | Med | Last-known-good cache (R5.3.2); existing curator allow-list is the primary trust signal anyway. |
| R7 | Static + curator deal collision causes wrong bidder routing | Med | High | Explicit precedence rule (static wins by default; per-publisher override; documented in deal response, R6.1.5). |
| R8 | Schedule-expiry scanner double-fires across instances | Med | Low | Postgres advisory lock (§11.5); idempotent state transitions. |
| R9 | Curator buyer-agent gets compromised → adversarial mutations | Low | High | Phase 1's intent whitelist + path validation already block out-of-band fields; per-deal authorisation (R5.1.10) limits blast radius to that curator's deals only. |
| R10 | OpenDirect 2.1 spec ambiguity around deal lifecycle states | High | Low | We codify our state machine explicitly in §6.2; if AAMP 2.1 disagrees, we adapt mappings without changing the underlying state machine. |
| R11 | Cert revocation latency: curator removed from registry but still has valid mTLS cert | Med | Med | Registry refresh interval (3600s default) is the cert revocation TTL; document this for ops; offer faster manual refresh via admin endpoint. |
| R12 | A buyer-agent hits us before being added to allow-list | High | Low | Reject `Unauthenticated` with structured error; include onboarding contact email in error; alert on first-call-from-unknown. |
| R13 | `take_rate_pct` immutability (R6.4.1) frustrates curators wanting to tune rate | Med | Low | Document the create-new-deal pattern; revisit in Phase 3 if friction is real. |
| R14 | Postgres `deals` table grows unbounded under heavy curator activity | Low | Med | TERMINATED deals retained 90 days then archived (Phase 3 cron); not a Phase 2 problem. |
| R15 | Non-AAMP-aware DSPs that hit our `:50051` see confusing errors | Low | Low | Discovery service responds to bare mTLS so a misconfigured DSP gets a useful capability reply; document in onboarding. |

### 13.2 Open Questions

- **OQ1.** Do we accept curator deals from agents NOT in the IAB Registry (i.e., bilateral pre-registry trust)?
  Recommendation: **No for prod, yes for staging.** Forces curators to register, helps IAB registry adoption, gives us standardised trust signals.
- **OQ2.** Take-rate ceiling at the platform level (separate from per-publisher cap)?
  Recommendation: **30%.** Beyond that, publishers complain; below that, curators won't bother. Make it a config (`AGENTIC_DEAL_PLATFORM_TAKE_RATE_MAX`).
- **OQ3.** Do we surface curator deal performance to publishers in their existing admin UI?
  Recommendation: **Phase 2 ships JSON only**, the existing admin SPA gets a "Curator Deals" tab in a Phase 2.5 cleanup.
- **OQ4.** First curator partner — who exactly?
  TBD by ops. Ideally one with an existing AAMP 2.0 buyer-agent integration (PMG via Alli is the obvious target if we can land it).
- **OQ5.** Currency handling — USD-only Phase 2, multi-currency later?
  Recommendation: **USD-only.** EUR / GBP curators rare in our current book; defer.
- **OQ6.** Do we expose Discovery without mTLS so a curator can introspect before onboarding?
  Recommendation: **No.** mTLS-only keeps trust posture simple; provide static `agents.json` over HTTPS for unauthenticated discovery instead (Phase 1 already does this).
- **OQ7.** mTLS cert rotation: 30-day overlap or 7-day?
  Recommendation: **30-day** for ops sanity; revisit if it causes operational pain.
- **OQ8.** Should curator deals be required to specify a publisher list at create time, or can they target "all opted-in publishers"?
  Recommendation: **Explicit publisher list required.** Forces curator to disclose targeting; matches how static PMPs work; simpler audit.

### 13.3 Decisions Locked This PRD

- **D1.** Phase 2 splits into 2A (inbound), 2B (deal-create), 2C (registry + SDK). Independently shippable.
- **D2.** mTLS-only on `:50051`; no plain gRPC inbound regardless of `AGENTIC_ALLOW_INSECURE`.
- **D3.** Take-rate immutable per deal; change = terminate + create new.
- **D4.** USD-only Phase 2.
- **D5.** Postgres advisory lock for multi-instance scheduler safety.
- **D6.** `agents.json` document version bumped to `"2.0"` in Phase 2A; Phase 1 readers continue to work.
- **D7.** Curator SDK private (in-repo); npm publish is a separate decision for Phase 3.
- **D8.** Settlement event suppressed below 0.0001 USD (R6.4.6).

---

## 14. Success Metrics

### 14.1 Phase 2 Acceptance KPIs (revenue-led)

| KPI | Target | Measured by |
|---|---|---|
| First curator live in staging by end of 2B | Yes / No | Manual sign-off; one named partner |
| Test-mode GPM captured against synthetic auctions | ≥ 1 USD | `curator_revenue_daily.gpm_usd` sum |
| Second curator onboarded with zero engineering touch | Yes / No | Onboarding ticket touchpoints |
| Time from cert receipt to first prod bid (curator 2+) | ≤ 1 hour | Stopwatch on first onboarding after the first |
| P95 inbound `GetMutations` latency | ≤ 50 ms (ex-database) | `agentic_inbound_call_duration_seconds{quantile="0.95"}` |
| Auction-failure rate caused by inbound code | 0 | Recovered panics + auction error rate diff |
| Settlement event accuracy vs ground truth | 100% on test set | Reconciliation job vs synthetic dataset |
| `agents.json` registry validation green in CI | 100% | Schema check on every PR |

### 14.2 Commercial KPIs (post-Phase 2, monitored over 90 days)

| KPI | Target | Notes |
|---|---|---|
| Active curator deals at end of 90 days | ≥ 10 | Across ≥ 2 curator partners |
| GPM captured from curator deals | TBD by commercial; first cut: ≥ $5k MRR by Day 90 | Net of publisher revenue split |
| Curator-deal CPM premium vs open auction | +15% to +40% | Curators bring matched audiences; pay more for them |
| Buyer-agent dial-ins (unique `caller_agent_id`) | ≥ 5 distinct in 90 days | Indicator of registry-driven discovery |
| Inbound auth-failure rate | ≤ 1% of attempts | High = misconfigured curators or attack traffic |
| Schedule-expiry events per million auctions | ≤ 100 | High = curators misusing daypart scheduling |

### 14.3 Operational Metrics

| KPI | Target |
|---|---|
| Time to disable inbound in prod (env flip + restart) | ≤ 60 s |
| Time to onboard a new curator (after the first) | ≤ 1 h documented; ≤ 2 h tolerated |
| Time to revoke a curator (allowlist remove + `SUSPEND` propagation) | ≤ 60 s (next scanner pass) |
| Daily reconciliation job runtime | ≤ 5 min for ≤ 1M settlement events |
| `agentic_registry_refresh_failed_total` rate | ≤ 1 in 100 attempts | Indicator of IAB infra reliability |

---

## 15. Compliance & Security

### 15.1 Privacy checklist (must pass before prod flip per sub-phase)

- [ ] **TCF v2 / GPP / CCPA pass-through unchanged.** Inbound mutations carrying audiences are gated by the same `DeriveAgentConsent` used outbound. No new minimisation logic.
- [ ] **COPPA hard-block.** Any deal carrying audience targeting on `regs.coppa=1` traffic is rejected at applier time, before any segment is written into `user.data`.
- [ ] **Curator-injected audiences treated as opaque.** Same path validation as Phase 1 (R5.5.4). Curators cannot inject `user.id`, `user.buyeruid`, `user.geo.lat/lon`, or any field outside the allowed selectors per intent.
- [ ] **Per-deal consent decision audited.** Every deal-creation event logs `audience_categories` + `consent_decision`. Compliance can reconstruct which audiences flowed through which deals.
- [ ] **Publisher opt-in is explicit.** `curator_deals_enabled=false` default; `curator_allowlist` requires explicit `include` entries.
- [ ] **GPP applicable-section honoured.** A curator deal targeting an audience requiring Purpose 7 is rejected when the auction's GPP withholds Purpose 7 — same gate as bidder path.

### 15.2 Security checklist

- [ ] **mTLS-only inbound.** Plain `grpc://` rejected on `:50051` regardless of any flag. Production-environment validation in `cmd/server/config.go::Validate` mirrors Phase 1's outbound-side check.
- [ ] **SPKI pinning** in addition to CA chain validation. Caller cert MUST verify against CA AND SPKI fingerprint MUST match an entry in our trusted-buyer table (R5.3.1).
- [ ] **Registry cross-check.** Caller `agent_id` MUST exist in the IAB Tools Portal Agent Registry's verified-seller-eligible list, refreshed every `AGENTIC_REGISTRY_REFRESH_S`. Last-known-good fallback on registry fetch failure.
- [ ] **Per-deal authorisation.** Mutations referencing deals outside the caller's authorised set (R5.1.10) rejected with structured error.
- [ ] **No leakage on auth failure.** Auth failures return `Unauthenticated` with a non-detailed message; details land in `agentic.inbound.auth_failed` log only. No registry-content leak; no deal-existence oracle.
- [ ] **Cert rotation runbook.** 30-day overlap window for SPKI rotation (R5.2.3); admin endpoint to register a successor SPKI; documented in `agentic/docs/curator-onboarding.md` and `agentic/docs/inbound-mtls-deploy.md`.
- [ ] **Per-call panic recovery.** Defensive `recover()` in every gRPC handler. A misbehaving caller cannot tear down the inbound server.
- [ ] **Settlement event integrity.** `bids.deal_id` FK ensures every settlement event ties to an audited deal; idempotency on `RTBRequest.id` prevents double-attribution.
- [ ] **Per-curator capacity caps** (R6.1.7) prevent a compromised curator agent from creating millions of phantom deals.
- [ ] **Audit log redaction.** mTLS cert chain captured per deal; SPKI fingerprints stored; no private keys, no API keys, ever in logs.

### 15.3 Operational security

- **Blast radius bounded by allow-list.** A compromised curator can only affect deals on publishers that explicitly allowlisted them — not the entire publisher set.
- **Settlement audit trail immutable.** `deal_events` is append-only; `bids.deal_id` is FK with no update path; settlement events are emitted to log + analytics, not to a mutable store.
- **Live revocation tested.** When a publisher removes a curator from the allowlist, all that curator's `ACTIVE` deals on that publisher transition to `SUSPENDED` within 60 s. CI test exercises the revocation path.
- **Non-AAMP traffic not exposed.** The inbound `:50051` server only handles AAMP RPCs. Existing OpenRTB JSON-over-HTTP traffic continues on `:8000` unchanged.
- **Penetration test required before first prod-curator flip.** Specifically: mTLS bypass attempts, deal-spoofing across curator boundaries, auth-failure timing-attack analysis.

### 15.4 Inbound fraud signals

Curators get the same IVT signals that bidders see today, but Phase 2 adds three new fraud-relevant inbound checks:

| Check | Trigger | Action |
|---|---|---|
| **Cert mismatch storm** | Same caller IP presents > 5 different SPKI fingerprints in 60 s | Drop connection; alert `agentic.inbound.spki_storm` |
| **Deal-create flood** | Same `agent_id` calls `CreateDeal` > 100/min | Breaker opens; new creates rejected `ResourceExhausted` |
| **Phantom-deal queries** | Same `agent_id` calls `DescribeDeal` for non-existent IDs > 10/min | Suggests probing; alert + breaker |

---

## 16. Out of Scope (This Cycle)

Explicit list, mapped to PRD references where relevant:

- **N1.** Buyer Agent SDK 2.0 (we're a seller, not a buyer; per PRD §3.1).
- **N2.** Agentic Audiences (cross-agent audience handoff per AAMP 2.0). Defer to Phase 3 — needs more than one curator live first to be useful.
- **N3.** AAMP 3.0 features (in flight per IAB; not frozen at time of writing).
- **N4.** MCP transport. Phase 3 if at all; LLM-prompt-injection surface still unevaluated.
- **N5.** Publishing the prebid.js adapter to the prebid.js org repo or any public npm registry. Independent decision.
- **N6.** Replacing static PMP deals. Curator deals are an *additive* lane; static deals continue unchanged.
- **N7.** Direct Sold campaign automation (AAMP 2.0 Buyer Agent feature; not relevant to our seller surface).
- **N8.** Self-service curator onboarding UI. Phase 2C ships docs + SDK; UI follows revenue.
- **N9.** Multi-region deal-store replication. Single-master Postgres for Phase 2; geo-replication is a separate piece of work.
- **N10.** Multi-currency settlement. USD-only Phase 2 (D4).
- **N11.** Take-rate mutability after deal create. D3 — terminate + create new instead.
- **N12.** Cross-tenant agent marketplace (publishers can't pick curators à la carte from a UI; ops still flips allow-list).
- **N13.** A2A protocol support (agent-to-agent direct, no SSP in the middle). Out of our seller-side surface entirely.
- **N14.** Real-time deal performance dashboards in the publisher admin SPA. JSON endpoints only Phase 2; UI follows revenue (S1 stretch).
- **N15.** Self-registration on IAB Tools Portal for Phase 1 — was deferred to Phase 2 (now in scope as Phase 2C / G7).

---

## 17. Appendix

### 17.1 References

**External**
- [IAB Tech Lab — AAMP 2.0 Release Brings Transaction-Ready Buyer and Seller Agent SDKs](https://iabtechlab.com/aamp-2-0-release-brings-transaction-ready-buyer-and-seller-agent-sdks/)
- [IABTechLab/AAMP repo on GitHub](https://github.com/IABTechLab/AAMP/blob/main/README.md)
- [PMG Integrates IAB Tech Lab AAMP Standards Into Alli Operating System](https://www.prnewswire.com/news-releases/pmg-integrates-iab-tech-lab-aamp-standards-into-alli-operating-system-302734906.html)
- [IAB Tech Lab — Agentic Standards hub](https://iabtechlab.com/standards/agentic-advertising-and-ai/)
- [IAB Canada — AAMP framework intro](https://iabcanada.com/iab-tech-lab-introducing-aamp-a-new-framework-for-agentic-advertising-standards/)
- [PPC Land — AAMP naming explainer](https://ppc.land/iab-tech-lab-names-its-agentic-ad-initiative-aamp-to-end-market-confusion/)
- IAB OpenDirect 2.1 spec (frozen with AAMP 2.0)
- IAB AdCOM 1.x spec
- IAB Content Taxonomy 3.0
- IAB Tools Portal Agent Registry (`https://toolsportal.iabtechlab.com/registry/v1`)

**Internal**
- Phase 1 PRD: `docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md`
- Phase 1 plan: `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md`
- Phase 1 codebase: `agentic/` (umbrella), `internal/exchange/agentic_hooks.go`, `cmd/server/server.go::initAgenticInbound` (TBD)
- Vendoring patches: `agentic/proto/iabtechlab/README.md`
- Phase 1 onboarding doc: `agentic/docs/agent-vendor-onboarding.md`

### 17.2 Vendored Files (Phase 2)

Source: `https://github.com/IABTechLab/AAMP`
Commit SHA: **TBD — pin at first vendor pull**
Date pulled: **2026-04-28** (or whenever Phase 2A starts)

| Vendored to | Source path |
|---|---|
| `agentic/proto/iabtechlab/aamp/discovery/v1/discovery.proto` | `proto/discovery/v1/discovery.proto` (Phase 2A) |
| `agentic/proto/iabtechlab/opendirect/v2/opendirect.proto` | `proto/opendirect/v2/opendirect.proto` (Phase 2B) |
| `agentic/proto/iabtechlab/opendirect/v2/opendirectservices.proto` | `proto/opendirect/v2/opendirectservices.proto` (Phase 2B) |
| `agentic/proto/iabtechlab/adcom/v1/adcom.proto` | `proto/adcom/v1/adcom.proto` (Phase 2B) |

Provenance recorded in `agentic/proto/iabtechlab/README.md` with SHA + date per Phase 1 convention.

### 17.3 Glossary (extends Phase 1 glossary)

- **AAMP 2.0** — Agentic Advertising Management Protocols 2.0; released 2026-04-26. Adds transaction-ready Buyer/Seller Agent SDKs, OpenDirect 2.1, AdCOM integration.
- **Curator** — A vendor that packages audiences + content signals and brings buyer-side demand. In AAMP 2.0 terms, a curator runs a Buyer Agent that creates programmatic deals on Seller Agent inventory.
- **OpenDirect 2.1** — IAB direct-deal negotiation protocol, agentified for AAMP 2.0. Defines `CreateDeal`, `ActivateDeal`, lifecycle.
- **AdCOM** — IAB's typed object schema for ad-products, audiences, content categories. Used as the payload format inside OpenDirect deal-create messages.
- **IAB Content Taxonomy 3.0** — Canonical IAB category vocabulary used by AdCOM.
- **Discovery Service** — gRPC service we expose at `:50051` returning capability manifests so buyer agents can introspect us.
- **Seller Agent** — In AAMP terms: an agent that exposes inventory + accepts deals. Catalyst, post Phase 2.
- **Buyer Agent** — An agent that discovers inventory + creates deals + bids. PMG Alli is the public reference.
- **IAB Tools Portal Agent Registry** — IAB-hosted directory where agents register their capabilities + certs. Live since 2026-03-01.
- **Take-rate** — Catalyst's margin on a curator deal. Per-deal, immutable, capped per publisher.
- **Settlement event** — A structured log line emitted on a curator-deal win, recording the take-rate split between Catalyst and the publisher.
- **SPKI fingerprint** — Subject Public Key Info hash; stable across cert renewals as long as the keypair is re-used. Used for cert pinning so buyers can rotate certs without out-of-band coordination.

### 17.4 Document History

| Date | Author | Change |
|---|---|---|
| 2026-04-28 | Claude (session 014d9kgdbq3ieoECL3qtcUB2) | Scaffold + full expansion |

---

_End of document._
