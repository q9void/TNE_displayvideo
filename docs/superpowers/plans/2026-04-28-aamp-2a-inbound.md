# Phase 2A — Inbound RTBExtensionPoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Phase 2A of the AAMP 2.0 integration — an inbound `RTBExtensionPoint` gRPC server on `:50051` that accepts mutations from authenticated buyer agents (curators), reusing the Phase 1 `Applier` in the inverse direction. Default-off behind `AGENTIC_INBOUND_ENABLED=false`. Zero behaviour change to existing auctions when disabled.

**Architecture:** New sub-package `agentic/inbound/` holds the server, authenticator, rate-limit, and per-RPC handlers. Reuses Phase 1's `agentic.Applier`, `agentic.Registry`, `agentic.OriginatorStamper`, `agentic.DeriveAgentConsent` as-is in the inverse direction (when a buyer agent submits mutations, we apply them to the in-flight bid request the same way Phase 1 applies outbound results). Bumps `agents.json` document version to `"2.0"` with a new `capabilities` block (additive — Phase 1 readers continue to work). Three integration files outside the umbrella are modified at known call sites: `cmd/server/server.go` (new `initAgenticInbound()`), `cmd/server/config.go` (Inbound fields on `AgenticConfig`), `agentic/registry.go` (parse the new capabilities block).

**Tech Stack:** Go 1.24, `google.golang.org/grpc` (existing direct dep), zerolog (existing), Prometheus client_golang (existing). Module path `github.com/thenexusengine/tne_springwire`.

**Spec:** `docs/superpowers/specs/2026-04-28-aamp-2-phase-2-curator-revenue-design.md` (§5, §11, §12.1 specifically).

---

## File Map

```
agentic/inbound/                                              NEW  inbound surface umbrella
  doc.go                                                      NEW  package overview
  config.go                                                   NEW  ServerConfig
  identity.go                                                 NEW  AgentIdentity
  auth.go                                                     NEW  Authenticator interface + DevAuthenticator (Phase 2A no-mTLS dev path)
  ratelimit.go                                                NEW  per-agent + per-publisher QPS limits
  server.go                                                   NEW  Server: lifecycle (Start/Stop), gRPC reg
  rtb_handler.go                                              NEW  GetMutations handler (reuses Applier)
  discovery_handler.go                                        NEW  DescribeCapabilities handler
  errors.go                                                   NEW  inbound-specific sentinel errors
  metrics.go                                                  NEW  4 new Prom metrics from PRD §5.5

  config_test.go                                              NEW
  auth_test.go                                                NEW
  ratelimit_test.go                                           NEW
  rtb_handler_test.go                                         NEW
  discovery_handler_test.go                                   NEW
  integration_test.go                                         NEW  in-process buyer-agent → inbound → applier round-trip

agentic/assets/
  agents.schema.json                                          MOD  bump to v2; add capabilities + media_kits + product_catalogs
  agents.json                                                 MOD  bump to "version": "2.0"; add capabilities block

agentic/registry.go                                           MOD  parse + expose capabilities; new Registry.Capabilities()
agentic/registry_test.go                                      MOD  v2 fixtures

cmd/server/config.go                                          MOD  AgenticConfig: InboundEnabled, InboundGRPCPort, MTLSCAPath, etc.
cmd/server/server.go                                          MOD  initAgenticInbound() after initExchange(); Server.agenticInbound field

go.mod                                                        UNCHANGED  grpc already direct dep from Phase 1
Makefile                                                      UNCHANGED  no new proto trees this sub-phase
```

---

## ═══════════════ PHASE 2A ═══════════════

Phase 2A scope: inbound surface only. No OpenDirect deal-create RPCs (return `Unimplemented` until Phase 2B); no AdCOM mapping; no IAB Registry self-registration; no curator SDK.

### Task 1: Bump agents.json schema + asset to v2

**Files:**
- Modify: `agentic/assets/agents.schema.json`
- Modify: `agentic/assets/agents.json`

Steps:

- [ ] **Step 1.1** Extend the JSON Schema to allow `version` enum to include `"2.0"` (keep `"1.0"` accepted for backwards compat).

- [ ] **Step 1.2** Add an optional top-level `capabilities` object to the schema with subfields:
  ```json
  {
    "transports":               {"type": "array", "items": {"type": "string", "enum": ["grpc", "grpcs", "mcp", "http"]}},
    "intents_inbound":          {"type": "array", "items": {"type": "string", "enum": ["ACTIVATE_SEGMENTS","ACTIVATE_DEALS","SUPPRESS_DEALS","ADJUST_DEAL_FLOOR","ADJUST_DEAL_MARGIN","BID_SHADE","ADD_METRICS","ADD_CIDS"]}},
    "intents_outbound":         {"type": "array", "items": {"type": "string", "enum": [...]}},
    "lifecycles":               {"type": "array", "items": {"type": "string", "enum": ["PUBLISHER_BID_REQUEST","DSP_BID_RESPONSE"]}},
    "opendirect_version":       {"type": "string"},
    "adcom_version":            {"type": "string"},
    "content_taxonomy_version": {"type": "string"},
    "deal_types_accepted":      {"type": "array", "items": {"type": "string"}}
  }
  ```

- [ ] **Step 1.3** Add optional top-level `media_kits` and `product_catalogs` arrays per PRD §7.2 (objects with `id` + `url` shape). Reserved for Phase 2C; we just allow them in the schema now.

- [ ] **Step 1.4** Update `agentic/assets/agents.json` to:
  ```json
  {
    "$schema": "https://thenexusengine.com/schemas/agents.v2.json",
    "version": "2.0",
    "seller_id": "9131",
    "seller_domain": "thenexusengine.com",
    "contact": "agentic-ops@thenexusengine.com",
    "updated_at": "2026-04-28T00:00:00Z",
    "capabilities": {
      "transports":               ["grpc", "grpcs"],
      "intents_inbound":          ["ACTIVATE_SEGMENTS","ACTIVATE_DEALS","SUPPRESS_DEALS","ADJUST_DEAL_FLOOR","ADJUST_DEAL_MARGIN","BID_SHADE","ADD_METRICS"],
      "intents_outbound":         ["ACTIVATE_SEGMENTS","ADJUST_DEAL_FLOOR","BID_SHADE"],
      "lifecycles":               ["PUBLISHER_BID_REQUEST","DSP_BID_RESPONSE"],
      "opendirect_version":       "2.1",
      "adcom_version":            "1.0",
      "content_taxonomy_version": "3.0",
      "deal_types_accepted":      ["programmatic_pmp","programmatic_guaranteed"]
    },
    "agents": []
  }
  ```

**Verification:**
- `gojsonschema` validation against the new doc passes at boot.
- Phase 1 readers — those that only inspect `version`, `seller_id`, `agents` — continue to work because the v2 fields are additive.

---

### Task 2: Update agentic.Registry to parse capabilities

**Files:**
- Modify: `agentic/registry.go`
- Modify: `agentic/registry_test.go`

Steps:

- [ ] **Step 2.1** Add a `Capabilities` struct to `registry.go`:
  ```go
  type Capabilities struct {
      Transports             []string `json:"transports,omitempty"`
      IntentsInbound         []string `json:"intents_inbound,omitempty"`
      IntentsOutbound        []string `json:"intents_outbound,omitempty"`
      Lifecycles             []string `json:"lifecycles,omitempty"`
      OpenDirectVersion      string   `json:"opendirect_version,omitempty"`
      AdCOMVersion           string   `json:"adcom_version,omitempty"`
      ContentTaxonomyVersion string   `json:"content_taxonomy_version,omitempty"`
      DealTypesAccepted      []string `json:"deal_types_accepted,omitempty"`
  }
  ```

- [ ] **Step 2.2** Add `Capabilities Capabilities` field to the existing `registryDoc` struct so `json.Unmarshal` populates it.

- [ ] **Step 2.3** Expose a `Registry.Capabilities() Capabilities` accessor used by the Discovery handler.

- [ ] **Step 2.4** Loosen the `version` check: accept `"1.0"` OR `"2.0"`. Default-empty `Capabilities{}` for v1 docs.

- [ ] **Step 2.5** Add tests:
  - `TestLoadRegistry_v2DocPopulatesCapabilities`
  - `TestLoadRegistry_v1DocStillWorks` (regression)
  - `TestLoadRegistry_v2EmptyCapabilities` (when capabilities omitted)

**Verification:**
- `go test ./agentic -run TestLoadRegistry` passes with new + existing tests.
- `Registry.AgentCount()` semantics unchanged.

---

### Task 3: Build agentic/inbound/ skeleton

**Files:**
- Create: `agentic/inbound/doc.go`
- Create: `agentic/inbound/config.go`
- Create: `agentic/inbound/identity.go`
- Create: `agentic/inbound/auth.go`
- Create: `agentic/inbound/ratelimit.go`
- Create: `agentic/inbound/server.go`
- Create: `agentic/inbound/errors.go`
- Create: `agentic/inbound/metrics.go`

Steps:

- [ ] **Step 3.1** `doc.go` — short package docstring per PRD §11.1: "Inbound surface for AAMP 2.0 buyer/curator agents. Phase 2A ships the RTBExtensionPoint + Discovery handlers; OpenDirect is Phase 2B."

- [ ] **Step 3.2** `config.go`:
  ```go
  type ServerConfig struct {
      Enabled              bool
      GRPCPort             int           // default 50051
      MTLSCAPath           string        // path to client-cert CA bundle (Phase 2A.1)
      MTLSServerCertPath   string
      MTLSServerKeyPath    string
      AllowDevNoMTLS       bool          // dev only; Validate() rejects in prod
      QPSPerAgent          int           // default 1000
      QPSPerPublisher      int           // default 200
      MaxRecvMsgBytes      int           // default 4 MiB
      IdempotencyWindow    time.Duration // default 60s
  }

  func (c ServerConfig) defaults() ServerConfig { /* fill zero values */ }
  ```

- [ ] **Step 3.3** `identity.go` — `AgentIdentity` per PRD §5.3:
  ```go
  type AgentIdentity struct {
      AgentID          string
      AgentType        string   // "DSP" or "PUBLISHER"
      AuthorisedDeals  []string
      SPKIFingerprint  string
      RegistryVerified bool
  }
  ```

- [ ] **Step 3.4** `auth.go` — `Authenticator` interface + `DevAuthenticator` (no-mTLS implementation for Phase 2A.0; reads agent_id from a static gRPC metadata header `x-aamp-agent-id`). `MTLSAuthenticator` is sketched but stubbed; full mTLS impl is Phase 2A.1 (a follow-up sub-PR before merge).
  ```go
  type Authenticator interface {
      Verify(ctx context.Context) (*AgentIdentity, error)
      RefreshRegistry(ctx context.Context) error
  }

  // DevAuthenticator: dev-only allow-list keyed by header.
  // Production must use MTLSAuthenticator (Phase 2A.1).
  type DevAuthenticator struct {
      AllowList map[string]AgentIdentity // agent_id → identity
  }
  ```
  Document clearly that `DevAuthenticator` is dev-only; `Validate()` in `cmd/server/config.go` rejects it in production.

- [ ] **Step 3.5** `ratelimit.go` — token-bucket-style limiter using existing patterns. Two limiters: per-`agent_id` (default 1000 QPS) and per-`publisher_id` (default 200 QPS). Reuse `idr.CircuitBreaker` keyed by agent_id for failure-rate-driven shedding.

- [ ] **Step 3.6** `errors.go` — sentinels for the inbound path:
  ```go
  ErrAuthFailed              = errors.New("inbound: authentication failed")
  ErrAuthFailedRegistry      = errors.New("inbound: agent not in registry")
  ErrAuthFailedSPKI          = errors.New("inbound: SPKI fingerprint mismatch")
  ErrAuthFailedDealset       = errors.New("inbound: deal not in caller's authorised set")
  ErrRateLimitedPerAgent     = errors.New("inbound: per-agent QPS exceeded")
  ErrRateLimitedPerPublisher = errors.New("inbound: per-publisher QPS exceeded")
  ErrCircuitOpen             = errors.New("inbound: circuit breaker open")
  ErrOriginatorRejected      = errors.New("inbound: only TYPE_DSP/TYPE_PUBLISHER accepted")
  ```

- [ ] **Step 3.7** `metrics.go` — register the four new Prom metrics from PRD §5.5:
  - `agentic_inbound_call_duration_seconds` (Histogram)
  - `agentic_inbound_mutation_total` (Counter)
  - `agentic_inbound_auth_failed_total` (Counter)
  - `agentic_registry_refresh_failed_total` (Counter)

- [ ] **Step 3.8** `server.go` — `Server` type with lifecycle:
  ```go
  type Server struct {
      cfg      ServerConfig
      applier  *agentic.Applier
      registry *agentic.Registry
      auth     Authenticator
      rate     *RateLimiter
      stamper  agentic.OriginatorStamper

      grpc *grpc.Server
      lis  net.Listener
  }

  func NewServer(cfg ServerConfig, ...) (*Server, error)
  func (s *Server) Start() error  // binds + serves on s.cfg.GRPCPort
  func (s *Server) Stop()         // graceful stop with 5s timeout
  ```
  When `cfg.Enabled=false`, `Start()` returns immediately without binding.

**Verification:** `go build ./agentic/inbound/...` clean. No tests yet — Task 6.

---

### Task 4: Implement RTBExtensionPoint inbound handler

**Files:**
- Create: `agentic/inbound/rtb_handler.go`

Steps:

- [ ] **Step 4.1** Implement `Server.GetMutations(ctx, req)` per PRD §5.1:
  ```go
  func (s *Server) GetMutations(ctx context.Context, req *pb.RTBRequest) (*pb.RTBResponse, error) {
      // 1. Authenticate (R5.1.2)
      identity, err := s.auth.Verify(ctx)
      if err != nil { return nil, status.Error(codes.Unauthenticated, "auth failed") }

      // 2. Originator type gate (R5.1.4) — accept TYPE_DSP / TYPE_PUBLISHER only
      // 3. Rate limit per-agent + per-publisher (R5.4.1, R5.4.3)
      // 4. Idempotency cache lookup (R5.1.11)
      // 5. Reuse Phase 1 Applier in inverse direction
      // 6. Return RTBResponse{Mutations: [], Metadata: {ApiVersion, ModelVersion}}
  }
  ```

- [ ] **Step 4.2** Idempotency cache: simple `sync.Map` keyed by `RTBRequest.id`, TTL via timestamp comparison on each lookup. Cleanup goroutine sweeps every minute.

- [ ] **Step 4.3** Defensive panic recovery (R5.1.6):
  ```go
  defer func() {
      if r := recover(); r != nil {
          logger.Log.Error().Interface("panic", r).
              Str("agent_id", identity.AgentID).
              Msg("inbound.handler.panic")
          // emit agentic.inbound.panic metric
      }
  }()
  ```

- [ ] **Step 4.4** Telemetry: emit `evt: agentic.inbound.call` with `direction=inbound`, `caller_agent_id`, `lifecycle`, `auth_status`, `mutation_count`, `latency_ms` per PRD §5.5 example.

- [ ] **Step 4.5** Mutation application: in Phase 2A we receive mutations FROM the buyer-agent and apply them TO the request the buyer-agent is targeting. Phase 2A only handles the lifecycle stage indicated by the caller's `RTBRequest.lifecycle`. Reuse `agentic.Applier.Apply` directly — same whitelist, same gates.

- [ ] **Step 4.6** Per-deal authorisation gate (R5.1.10): when a mutation references a deal (`ACTIVATE_DEALS`, `SUPPRESS_DEALS`, `ADJUST_DEAL_FLOOR`), check `identity.AuthorisedDeals` membership before applying. Phase 2A: empty `AuthorisedDeals` → reject all deal-touching mutations with `not_authorised_for_deal`. Phase 2B will populate this via the `deal/Store`.

- [ ] **Step 4.7** Originator stamping on response (R5.1.4 inverse): set `RTBResponse.Metadata.ModelVersion` to a build-stamp string for buyer-agent observability.

- [ ] **Step 4.8** Bounded payload: enforce `MaxRecvMsgBytes` via `grpc.MaxCallRecvMsgSize` server option (configured in `Server.Start`).

**Verification:** `go build ./agentic/inbound/...` clean. Handler smoke-testable via Task 6 integration test.

---

### Task 5: Implement Discovery service (DescribeCapabilities)

**Files:**
- Create: `agentic/inbound/discovery_handler.go`
- Decision: vendor a tiny Discovery proto OR define an in-house proto under `agentic/proto/tne/v1/discovery.proto` for Phase 2A.

Steps:

- [ ] **Step 5.1** **Decision recorded.** AAMP 2.0 Discovery proto is in flight upstream (per PRD §3) but not yet at a stable commit we can vendor cleanly. Phase 2A defines a minimal in-house proto under `agentic/proto/tne/v1/discovery.proto` with one RPC `DescribeCapabilities` returning the same JSON shape we serve at `/.well-known/agents.json`. When upstream freezes, we vendor and replace with a one-line type alias in `discovery_handler.go`. Document this in `agentic/proto/tne/v1/README.md`.

- [ ] **Step 5.2** Add the proto file:
  ```protobuf
  edition = "2023";

  package com.thenexusengine.agentic.discovery.v1;

  option go_package = "github.com/thenexusengine/tne_springwire/agentic/gen/tne/discovery/v1;discoveryv1";

  service Discovery {
    rpc DescribeCapabilities (DescribeCapabilitiesRequest) returns (CapabilitiesResponse);
  }

  message DescribeCapabilitiesRequest {}

  message CapabilitiesResponse {
    string version              = 1;
    string seller_id            = 2;
    string seller_domain        = 3;
    string capabilities_json    = 4;  // entire capabilities block as JSON; future-proof
    string etag                 = 5;
  }
  ```
  Phase 2A ships only `DescribeCapabilities`; `ListMediaKits` + `ListProductCatalogs` are Phase 2C.

- [ ] **Step 5.3** Update `Makefile` `generate-protos` target to include the new tne/v1 path. Run `make generate-protos`. Commit generated Go.

- [ ] **Step 5.4** Implement `Server.DescribeCapabilities`:
  ```go
  func (s *Server) DescribeCapabilities(ctx context.Context, req *dpb.DescribeCapabilitiesRequest) (*dpb.CapabilitiesResponse, error) {
      // Auth-light per R7.3.1: valid mTLS cert (or DevAuth header) but no Registry cross-check, no per-deal authz
      _, err := s.auth.Verify(ctx)
      if err != nil { return nil, status.Error(codes.Unauthenticated, "auth failed") }

      caps := s.registry.Capabilities()
      capsJSON, _ := json.Marshal(caps)
      etag := computeETag(capsJSON)
      return &dpb.CapabilitiesResponse{
          Version:           proto.String("2.0"),
          SellerId:          proto.String(s.registry.SellerID()),
          SellerDomain:      proto.String(s.registry.SellerDomain()),
          CapabilitiesJson:  proto.String(string(capsJSON)),
          Etag:              proto.String(etag),
      }, nil
  }
  ```

- [ ] **Step 5.5** Add `Registry.SellerDomain() string` accessor if missing.

- [ ] **Step 5.6** Register the Discovery service alongside RTBExtensionPoint on the same `:50051` listener in `Server.Start`.

**Verification:** `go build ./agentic/inbound/...` and `./agentic/gen/...` clean.

---

### Task 6: Tests — unit + integration

**Files:**
- Create: `agentic/inbound/config_test.go`
- Create: `agentic/inbound/auth_test.go`
- Create: `agentic/inbound/ratelimit_test.go`
- Create: `agentic/inbound/rtb_handler_test.go`
- Create: `agentic/inbound/discovery_handler_test.go`
- Create: `agentic/inbound/integration_test.go`

Steps:

- [ ] **Step 6.1** `config_test.go`:
  - `TestServerConfig_defaults` — zero values get filled.
  - `TestServerConfig_Validate_prodRejectsAllowDevNoMTLS` — production env + `AllowDevNoMTLS=true` fails.

- [ ] **Step 6.2** `auth_test.go`:
  - `TestDevAuthenticator_acceptsAllowListed` — header maps to identity.
  - `TestDevAuthenticator_rejectsUnknown` — missing header → `ErrAuthFailed`.
  - `TestDevAuthenticator_metadataParsing` — case-insensitive metadata key lookup.

- [ ] **Step 6.3** `ratelimit_test.go`:
  - `TestRateLimit_perAgent_blocksAboveQPS` — 1001th call within 1s rejected.
  - `TestRateLimit_perPublisher_independent` — different publishers don't share a bucket.
  - `TestRateLimit_circuitBreakerIntegration` — N consecutive failures opens breaker; subsequent calls rejected with `ErrCircuitOpen`.

- [ ] **Step 6.4** `rtb_handler_test.go` (table-driven):
  - happy path: caller sends ACTIVATE_SEGMENTS, Applier writes `user.data`, response returned with `mutation_count` matching applier decisions
  - auth failure → `Unauthenticated`
  - rate limit → `ResourceExhausted`
  - originator type rejection: `TYPE_SSP` rejected (only DSP/PUBLISHER allowed)
  - per-deal authorisation: `ACTIVATE_DEALS` with empty `AuthorisedDeals` → mutation rejected with `not_authorised_for_deal`
  - idempotency: same `RTBRequest.id` within window returns cached response, applier called once
  - panic recovery: stub a handler that panics; recover() emits metric; gRPC returns `Internal`

- [ ] **Step 6.5** `discovery_handler_test.go`:
  - `TestDescribeCapabilities_returnsRegistryDoc` — payload matches `Registry.Capabilities()`.
  - `TestDescribeCapabilities_etagStable` — same input → same etag.
  - `TestDescribeCapabilities_authLight` — DevAuth ID accepted; no per-deal authorisation needed.

- [ ] **Step 6.6** `integration_test.go` — end-to-end mirror of Phase 1's `TestIntegration_endToEnd`, just inverted:
  - Stand up an `inbound.Server` on `127.0.0.1:0`.
  - Construct a real Phase 1 `Applier`, `Registry`, `OriginatorStamper`.
  - Wire them via `NewServer`.
  - Build a curator-side gRPC client manually (no SDK yet).
  - Send `RTBRequest` with `Originator{TYPE_DSP}` and one `ACTIVATE_SEGMENTS` mutation.
  - Assert: response has `mutation_count: 1`, the inflight `BidRequest` (provided as a test argument) reflects the segment in `User.Data`, and structured log line `agentic.inbound.call` was emitted with `caller_agent_id`.

**Verification:** `go test ./agentic/inbound/... -race -count=1` passes — target ≥ 15 tests.

---

### Task 7: Wire inbound into cmd/server

**Files:**
- Modify: `cmd/server/config.go`
- Modify: `cmd/server/server.go`

Steps:

- [ ] **Step 7.1** Extend `AgenticConfig` (the existing struct from Phase 1) with inbound fields:
  ```go
  type AgenticConfig struct {
      // ... existing Phase 1 fields ...

      // Phase 2A — inbound surface
      InboundEnabled       bool
      InboundGRPCPort      int
      MTLSCAPath           string
      MTLSServerCertPath   string
      MTLSServerKeyPath    string
      AllowDevNoMTLS       bool
      InboundQPSPerAgent   int
      InboundQPSPerPub     int
  }
  ```

- [ ] **Step 7.2** In `ParseConfig()` (the part that builds `AgenticConfig` from env vars per Phase 1's pattern), add the inbound parsing:
  ```go
  if getEnvBoolOrDefault("AGENTIC_INBOUND_ENABLED", false) {
      cfg.Agentic.InboundEnabled       = true
      cfg.Agentic.InboundGRPCPort      = getEnvIntOrDefault("AGENTIC_INBOUND_GRPC_PORT", 50051)
      cfg.Agentic.MTLSCAPath           = os.Getenv("AGENTIC_MTLS_CA_PATH")
      cfg.Agentic.MTLSServerCertPath   = os.Getenv("AGENTIC_MTLS_CERT_PATH")
      cfg.Agentic.MTLSServerKeyPath    = os.Getenv("AGENTIC_MTLS_KEY_PATH")
      cfg.Agentic.AllowDevNoMTLS       = getEnvBoolOrDefault("AGENTIC_INBOUND_ALLOW_DEV_NO_MTLS", false)
      cfg.Agentic.InboundQPSPerAgent   = getEnvIntOrDefault("AGENTIC_INBOUND_QPS_PER_AGENT", 1000)
      cfg.Agentic.InboundQPSPerPub     = getEnvIntOrDefault("AGENTIC_INBOUND_QPS_PER_PUBLISHER", 200)
  }
  ```

- [ ] **Step 7.3** Extend `ServerConfig.Validate()`:
  - `AGENTIC_INBOUND_ENABLED=true` requires either (mTLS cert/key/CA paths all set) OR (`AllowDevNoMTLS=true`).
  - Production env (`isProduction()` true) + `AllowDevNoMTLS=true` → return error.
  - `InboundGRPCPort` in `[1, 65535]`.

- [ ] **Step 7.4** In `cmd/server/server.go`, add a new field on `Server`:
  ```go
  type Server struct {
      // ... existing ...
      agenticInbound *inbound.Server
  }
  ```

- [ ] **Step 7.5** Add `initAgenticInbound()` called after `initExchange()`:
  ```go
  func (s *Server) initAgenticInbound() error {
      if s.config.Agentic == nil || !s.config.Agentic.InboundEnabled {
          return nil
      }
      var auth inbound.Authenticator
      if s.config.Agentic.AllowDevNoMTLS {
          auth = inbound.NewDevAuthenticator() // pre-loaded with allowlist from env or DB
      } else {
          // Phase 2A.1 stretch — full mTLS authenticator. For Phase 2A we ship dev-only auth.
          return fmt.Errorf("inbound: production mTLS authenticator not yet implemented (Phase 2A.1)")
      }
      srv, err := inbound.NewServer(inbound.ServerConfig{
          Enabled:            true,
          GRPCPort:           s.config.Agentic.InboundGRPCPort,
          QPSPerAgent:        s.config.Agentic.InboundQPSPerAgent,
          QPSPerPublisher:    s.config.Agentic.InboundQPSPerPub,
          AllowDevNoMTLS:     s.config.Agentic.AllowDevNoMTLS,
          MaxRecvMsgBytes:    4 * 1024 * 1024,
          IdempotencyWindow:  60 * time.Second,
      }, s.exchange.Applier(), s.exchange.AgenticRegistry(), auth, agentic.OriginatorStamper{SellerID: s.config.Agentic.SellerID})
      if err != nil { return err }
      go func() {
          if err := srv.Start(); err != nil {
              logger.Log.Error().Err(err).Msg("agentic inbound server stopped")
          }
      }()
      s.agenticInbound = srv
      return nil
  }
  ```

- [ ] **Step 7.6** Expose `Exchange.Applier()` and `Exchange.AgenticRegistry()` accessors (small additions to `internal/exchange/exchange.go`) so the inbound init can fetch them. They were private in Phase 1; we make them readable now.

- [ ] **Step 7.7** Hook up graceful shutdown — `Server.Shutdown()` calls `s.agenticInbound.Stop()` if non-nil.

- [ ] **Step 7.8** Document in `agentic/README.md` under a new "Phase 2A — Inbound" section: env vars, dev-no-mTLS guidance, plan to upgrade to full mTLS in Phase 2A.1.

**Verification:**
- `go build ./...` clean across the whole repo.
- Existing test suites unchanged: `go test ./internal/exchange/... ./agentic/...` still passes.
- With `AGENTIC_INBOUND_ENABLED=false` (default), zero behaviour change.

---

### Task 8: Build, test, document, push

**Files:**
- (no new files — verification + commit only)

Steps:

- [ ] **Step 8.1** Local toolchain check (use the v1.64.8 golangci-lint installed in the Phase 1 session):
  ```bash
  go vet ./...
  go build ./...
  go test ./agentic/... -race -count=1
  go test ./agentic/inbound/... -race -count=1
  go test ./internal/exchange/... -short -count=1
  go test ./cmd/server/... -short -count=1
  golangci-lint run --timeout=5m --new-from-rev=origin/master ./agentic/... ./agentic/inbound/... ./internal/exchange/... ./cmd/server/...
  ```

- [ ] **Step 8.2** If any of the above fails: do NOT mark this plan task complete. Diagnose, patch, re-run.

- [ ] **Step 8.3** Manual smoke (optional Phase 2A):
  ```bash
  AGENTIC_ENABLED=true \
    AGENTIC_INBOUND_ENABLED=true \
    AGENTIC_INBOUND_ALLOW_DEV_NO_MTLS=true \
    AGENTIC_AGENTS_PATH=agentic/assets/agents.json \
    AGENTIC_SCHEMA_PATH=agentic/assets/agents.schema.json \
    go run ./cmd/server &
  # Use a tiny grpcurl call against localhost:50051 to hit DescribeCapabilities
  grpcurl -plaintext -H "x-aamp-agent-id: test-curator" \
    localhost:50051 com.thenexusengine.agentic.discovery.v1.Discovery/DescribeCapabilities
  ```

- [ ] **Step 8.4** Commit on `claude/integrate-iab-agentic-protocol-6bvtJ` with logical groupings (one commit per task block where reasonable):
  - `feat(agentic): bump agents.json schema + asset to v2 with capabilities`
  - `feat(agentic): add agentic/inbound/ skeleton (config, identity, auth, ratelimit, server)`
  - `feat(agentic): RTBExtensionPoint inbound handler reusing Phase 1 Applier`
  - `feat(agentic): Discovery service — DescribeCapabilities RPC`
  - `test(agentic): inbound package unit + integration tests`
  - `feat(agentic): wire AgenticConfig.Inbound + initAgenticInbound() into cmd/server`

- [ ] **Step 8.5** Push the branch and open the PR (no auto-merge):
  ```bash
  git push -u origin claude/integrate-iab-agentic-protocol-6bvtJ
  ```

- [ ] **Step 8.6** PR description references the Phase 2A subsection of the PRD (`docs/superpowers/specs/2026-04-28-aamp-2-phase-2-curator-revenue-design.md` §12.1) and the closure criteria below.

**Verification:**
- `git status` clean; commits pushed.
- PR open against `master`, CI green.

---

## Phase 2A Done When

- [ ] `go build ./...` clean
- [ ] `go test ./...` clean (full repo)
- [ ] `go test ./agentic/inbound/... -race` passes ≥ 15/15 tests
- [ ] `golangci-lint run --new-from-rev=origin/master ./agentic/... ./cmd/server/...` returns zero PR-introduced findings
- [ ] `AGENTIC_INBOUND_ENABLED=false` (default) — zero behaviour change observed in load test (P95 latency identical to pre-PR control)
- [ ] `AGENTIC_INBOUND_ENABLED=true` + `AGENTIC_INBOUND_ALLOW_DEV_NO_MTLS=true` in dev — `grpcurl` against `:50051` returns capabilities document; `RTBExtensionPoint.GetMutations` accepts a curator-shaped `RTBRequest` and applies whitelisted mutations to a test BidRequest
- [ ] Existing Phase 1 outbound traffic unaffected (verified by running `agentic/integration_test.go` end-to-end)
- [ ] PR open on master, CI green
- [ ] PRD §12.1 closure criteria documented as met OR explicitly deferred to Phase 2A.1

---

## Phase 2A.1 (deferred follow-up)

Two pieces deliberately deferred to a sub-PR after Phase 2A merges:

- **Full mTLS Authenticator.** `MTLSAuthenticator` with SPKI pinning, CA bundle verification, IAB Tools Portal Registry cross-check (PRD §5.3). Phase 2A ships `DevAuthenticator` only.
- **Cert rotation runbook.** `agentic/docs/inbound-mtls-deploy.md` covering nginx TLS pass-through config + 30-day SPKI rotation window per PRD §5.2.3.

These two together unblock the first real curator integration (Phase 2A → staging-curator handoff). The split keeps Phase 2A reviewable in a single PR rather than a 4000-line one.




