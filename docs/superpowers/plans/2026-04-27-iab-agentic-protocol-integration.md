# IAB Agentic Protocol Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Phase 1 of the IAB Tech Lab Agentic RTB Framework (ARTF v1.0) integration as a clustered `agentic/` umbrella: vendored protos + generated Go + outbound `RTBExtensionPoint` gRPC client + mutation applier + agents.json discovery + two lifecycle hooks in `Exchange.RunAuction`. Default-off behind `AGENTIC_ENABLED=false`. Zero behaviour change for existing publishers when disabled.

**Architecture:** All new code under `agentic/` at repo root (Go package + endpoints sub-pkg + protos + generated code + assets + prebid scaffold). Three integration files outside the umbrella are modified at known call sites: `internal/exchange/exchange.go` (Hook A pre-fanout, Hook B post-fanout), `cmd/server/server.go` (route registration + DI), `cmd/server/config.go` (Agentic sub-struct + env vars). gRPC + protobuf added as direct deps; generated code committed (no codegen at build time).

**Tech Stack:** Go 1.24, `google.golang.org/grpc` (new direct dep), `google.golang.org/protobuf` (promote from indirect to direct), zerolog (existing), Prometheus client_golang (existing), gojsonschema (existing), testify (existing). Module path `github.com/thenexusengine/tne_springwire`.

**Spec:** `docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md`

---

## File Map

```
agentic/                                                     NEW  umbrella for all new code
  README.md                                                  NEW  package overview + onboarding
  doc.go                                                     NEW  Go package doc

  client.go                                                  NEW  ExtensionPointClient: gRPC fan-out
  applier.go                                                 NEW  per-intent mutation handlers
  registry.go                                                NEW  AgentRegistry: load + validate agents.json
  originator.go                                              NEW  OriginatorStamper
  envelope.go                                                NEW  ext.aamp helpers
  consent.go                                                 NEW  DeriveAgentConsent
  decisions.go                                               NEW  ApplyDecision, AgentCallStat, DispatchResult
  errors.go                                                  NEW  sentinel errors
  lifecycle.go                                               NEW  Lifecycle enum + helpers

  client_test.go                                             NEW
  applier_test.go                                            NEW
  registry_test.go                                           NEW
  originator_test.go                                         NEW
  envelope_test.go                                           NEW
  consent_test.go                                            NEW
  fake_agent_test.go                                         NEW  in-process gRPC fake
  integration_test.go                                        NEW  full-auction integration

  proto/
    iabtechlab/                                              NEW  vendored, read-only
      README.md                                              NEW  provenance + commit SHA
      bidstream/mutation/v1/agenticrtbframework.proto        NEW  vendored
      bidstream/mutation/v1/agenticrtbframeworkservices.proto NEW vendored
      openrtb/v2.6/openrtb.proto                             NEW  vendored (transitive)
    tne/v1/                                                  NEW  reserved for our extensions
      README.md                                              NEW  placeholder

  gen/iabtechlab/                                            NEW  generated Go (committed)
    bidstream/mutation/v1/
      agenticrtbframework.pb.go                              NEW
      agenticrtbframework_grpc.pb.go                         NEW
    openrtb/v2.6/
      openrtb.pb.go                                          NEW

  endpoints/                                                 NEW  HTTP handlers sub-package
    agents_json.go                                           NEW  /.well-known/agents.json
    agents_admin.go                                          NEW  /admin/agents
    agents_json_test.go                                      NEW

  assets/                                                    NEW  static files
    agents.json                                              NEW  empty allow-list (Phase 1 default)
    agents.schema.json                                       NEW  JSON schema

  prebid-adapter/                                            NEW  JS scaffold (stretch, may slip)
    README.md                                                NEW
    package.json                                             NEW
    src/tneCatalystBidAdapter.js                             NEW
    src/tneCatalystAgenticEnvelope.js                        NEW
    src/agenticConsent.js                                    NEW
    src/agentDiscoveryRtdProvider.js                         NEW
    test/spec/tneCatalystBidAdapter_spec.js                  NEW
    test/spec/agenticEnvelope_spec.js                        NEW
    test/spec/agenticConsent_spec.js                         NEW
    examples/pbjs-config-basic.html                          NEW
    examples/pbjs-config-agentic.html                        NEW

  docs/                                                      NEW  onboarding docs
    README.md                                                NEW
    agent-vendor-onboarding.md                               NEW

internal/exchange/exchange.go                                MOD  Hook A line ~1596, Hook B line ~1797, +WithAgentic
cmd/server/config.go                                         MOD  AgenticConfig sub-struct + env parse + Validate
cmd/server/server.go                                         MOD  route registration + DI of Client/Applier/Stamper
go.mod                                                       MOD  add google.golang.org/grpc, promote protobuf
go.sum                                                       MOD  resolved deps
Makefile                                                     MOD  generate-protos target (documents the regen command)
```

---

## ═══════════════ PHASE 1 ═══════════════

Phase 1 ships the outbound, read-mostly slice. Inbound gRPC server, MCP transport, prebid adapter publication, and per-publisher overrides are out of scope (see PRD §10.2).

_Tasks 1–12 follow in subsequent chunks of this plan._

---

### Task 1: Vendor the ARTF protos

**Files:**
- Create: `agentic/proto/iabtechlab/README.md`
- Create: `agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframework.proto`
- Create: `agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframeworkservices.proto`
- Create: `agentic/proto/iabtechlab/openrtb/v2.6/openrtb.proto`
- Create: `agentic/proto/tne/v1/README.md`

Steps:

- [ ] **Step 1.1** Create directory tree:
  ```bash
  mkdir -p agentic/proto/iabtechlab/bidstream/mutation/v1
  mkdir -p agentic/proto/iabtechlab/openrtb/v2.6
  mkdir -p agentic/proto/tne/v1
  ```

- [ ] **Step 1.2** Resolve the upstream commit SHA on `IABTechLab/agentic-rtb-framework` `main` branch and record it.

- [ ] **Step 1.3** Vendor `proto/agenticrtbframework.proto` from the upstream repo to `agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframework.proto`. Verbatim copy, no edits.

- [ ] **Step 1.4** Vendor `agenticrtbframeworkservices.proto` (the gRPC service definition, contains `service RTBExtensionPoint { rpc GetMutations… }`) to `agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframeworkservices.proto`.

- [ ] **Step 1.5** Vendor the transitive OpenRTB v2.6 proto from `proto/com/iabtechlab/openrtb/v2.6/openrtb.proto` (referenced as `import "com/iabtechlab/openrtb/v2.6/openrtb.proto"`) to `agentic/proto/iabtechlab/openrtb/v2.6/openrtb.proto`.

- [ ] **Step 1.6** If any of the proto `import` paths or `option go_package` declarations don't match our `agentic/gen/...` layout, add a `go_package` option **only** at the bottom of each file (preferred non-invasive way is via a `--go_opt=Mfoo.proto=…` flag at codegen time — Step 2.6). Do NOT mutate the upstream proto bodies.

- [ ] **Step 1.7** Write `agentic/proto/iabtechlab/README.md`:

  ```markdown
  # Vendored IAB Tech Lab protos

  These files are copied verbatim from the upstream IAB Tech Lab repos. Do not edit.

  ## Provenance

  - Upstream: https://github.com/IABTechLab/agentic-rtb-framework
  - Commit SHA: <FILL IN AT VENDOR TIME>
  - Pulled: 2026-04-27
  - Pulled by: claude/integrate-iab-agentic-protocol-6bvtJ branch

  ## Refresh procedure

  1. Resolve the latest commit SHA on `main`.
  2. Re-copy the four files listed below from upstream paths to vendored paths.
  3. Update SHA + date in this file.
  4. Re-run `make generate-protos` (Task 2).
  5. Run `go test ./agentic/...` and inspect for breakage in our applier whitelist.

  ## Vendored files

  | Vendored path | Upstream path |
  |---|---|
  | `bidstream/mutation/v1/agenticrtbframework.proto` | `proto/agenticrtbframework.proto` |
  | `bidstream/mutation/v1/agenticrtbframeworkservices.proto` | `agenticrtbframeworkservices.proto` |
  | `openrtb/v2.6/openrtb.proto` | `proto/com/iabtechlab/openrtb/v2.6/openrtb.proto` |
  ```

- [ ] **Step 1.8** Write `agentic/proto/tne/v1/README.md`:

  ```markdown
  # TNE proto extensions (reserved)

  Reserved for any TNE-specific protos that extend ARTF via the
  `extensions 500 to max` range on `RTBRequest.Ext`. Phase 1 has
  none — directory is a placeholder.
  ```

- [ ] **Step 1.9** Verify nothing else references the old `proto/` path by running:
  ```bash
  grep -rn "iabtechlab/agentic-rtb-framework" --include="*.go" --include="*.md" .
  ```
  Should match only the docs and the vendored README.

**Verification:** Files exist, no edits to upstream content. `git diff --stat agentic/proto/` shows only additions.

---

### Task 2: Add gRPC dep + generate Go code

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `Makefile`
- Create: `agentic/gen/iabtechlab/bidstream/mutation/v1/agenticrtbframework.pb.go`
- Create: `agentic/gen/iabtechlab/bidstream/mutation/v1/agenticrtbframework_grpc.pb.go`
- Create: `agentic/gen/iabtechlab/openrtb/v2.6/openrtb.pb.go`

Steps:

- [ ] **Step 2.1** Add `google.golang.org/grpc` to `go.mod` direct requires. Use a version from late 2025 / early 2026 compatible with Go 1.24 (e.g. `v1.66.0` or later — pick whatever `go get` resolves at the time):
  ```bash
  go get google.golang.org/grpc@latest
  ```

- [ ] **Step 2.2** Promote `google.golang.org/protobuf` from indirect to direct:
  ```bash
  go get google.golang.org/protobuf@latest
  ```

- [ ] **Step 2.3** Run `go mod tidy`. Verify `go.sum` updates and no other indirects break.

- [ ] **Step 2.4** Install codegen tooling locally (NOT a build dep — committed output is the artefact):
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```
  Devs need `protoc` (system protobuf compiler) on path. Document this in `agentic/README.md` (Task 11).

- [ ] **Step 2.5** Add a `Makefile` target (do not run automatically; documents the regen command):
  ```makefile
  .PHONY: generate-protos
  generate-protos:
  	protoc \
  	  --proto_path=agentic/proto/iabtechlab \
  	  --go_out=agentic/gen/iabtechlab \
  	  --go_opt=paths=source_relative \
  	  --go-grpc_out=agentic/gen/iabtechlab \
  	  --go-grpc_opt=paths=source_relative \
  	  --go_opt=Mbidstream/mutation/v1/agenticrtbframework.proto=github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1 \
  	  --go_opt=Mopenrtb/v2.6/openrtb.proto=github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/openrtb/v2.6 \
  	  --go_opt=Mcom/iabtechlab/openrtb/v2.6/openrtb.proto=github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/openrtb/v2.6 \
  	  $$(find agentic/proto/iabtechlab -name '*.proto')
  ```

- [ ] **Step 2.6** Run `make generate-protos` once locally. Confirm three `.pb.go` files plus one `_grpc.pb.go` file land under `agentic/gen/iabtechlab/...`.

- [ ] **Step 2.7** If the generated `_grpc.pb.go` was placed under the wrong package path because of import-path mismatches, re-run with adjusted `--go_opt=M` mappings until the layout matches the File Map. The most common fix is rewriting the import lines after generation — only do this if codegen flags can't be tuned. Prefer flag tuning.

- [ ] **Step 2.8** Run `go build ./agentic/gen/...` to confirm the generated code compiles.

- [ ] **Step 2.9** Commit the generated `*.pb.go` and `*_grpc.pb.go` files. Note in the commit message that codegen output is committed and the regen command lives in the Makefile.

**Verification:**
- `go build ./agentic/gen/...` clean.
- `agentic/gen/iabtechlab/bidstream/mutation/v1/agenticrtbframework_grpc.pb.go` declares `RTBExtensionPointClient` and `RTBExtensionPointServer` types.
- `agentic/gen/iabtechlab/bidstream/mutation/v1/agenticrtbframework.pb.go` declares `RTBRequest`, `RTBResponse`, `Mutation`, `Originator`, `Intent`, `Operation`, `Lifecycle` types.

---

### Task 3: Build agentic foundations — types, errors, lifecycle, decisions, registry

**Files:**
- Create: `agentic/doc.go`
- Create: `agentic/lifecycle.go`
- Create: `agentic/errors.go`
- Create: `agentic/decisions.go`
- Create: `agentic/registry.go`
- Create: `agentic/registry_test.go`
- Create: `agentic/assets/agents.json`
- Create: `agentic/assets/agents.schema.json`

Steps:

- [ ] **Step 3.1** Create `agentic/doc.go`:
  ```go
  // Package agentic implements the TNE Catalyst integration with the IAB
  // Tech Lab Agentic RTB Framework (ARTF v1.0). It provides:
  //
  //   - Outbound RTBExtensionPoint gRPC client (Client) for fanning out bid
  //     requests/responses to extension-point agents at two lifecycle hooks.
  //   - A Mutation Applier that whitelists the seven supported ARTF intents
  //     and rejects unsupported ones.
  //   - An AgentRegistry loaded from agents.json on disk.
  //   - An OriginatorStamper that emits Originator{TYPE_SSP, id=<seller_id>}
  //     and writes the ext.aamp envelope on outbound OpenRTB BidRequests.
  //
  // Phase 1 is outbound only. Inbound RTBExtensionPoint gRPC server, MCP
  // transport, and per-publisher agent overrides are Phase 2.
  //
  // See docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md
  // for the full design.
  package agentic
  ```

- [ ] **Step 3.2** Create `agentic/lifecycle.go`:
  ```go
  package agentic

  import pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"

  type Lifecycle int

  const (
      LifecycleUnspecified         Lifecycle = 0
      LifecyclePublisherBidRequest Lifecycle = 1
      LifecycleDSPBidResponse      Lifecycle = 2
  )

  func (l Lifecycle) String() string {
      switch l {
      case LifecyclePublisherBidRequest:
          return "PUBLISHER_BID_REQUEST"
      case LifecycleDSPBidResponse:
          return "DSP_BID_RESPONSE"
      default:
          return "UNSPECIFIED"
      }
  }

  func (l Lifecycle) Proto() pb.Lifecycle {
      switch l {
      case LifecyclePublisherBidRequest:
          return pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST
      case LifecycleDSPBidResponse:
          return pb.Lifecycle_LIFECYCLE_DSP_BID_RESPONSE
      default:
          return pb.Lifecycle_LIFECYCLE_UNSPECIFIED
      }
  }
  ```

- [ ] **Step 3.3** Create `agentic/errors.go` with sentinel errors per PRD §7.2:
  ```go
  package agentic

  import "errors"

  var (
      ErrTmax              = errors.New("agentic: tmax exceeded")
      ErrCircuitOpen       = errors.New("agentic: circuit breaker open")
      ErrUnsupportedIntent = errors.New("agentic: unsupported intent")
      ErrUnsupportedOp     = errors.New("agentic: unsupported operation")
      ErrPathInvalid       = errors.New("agentic: mutation path invalid")
      ErrPayloadTooLarge   = errors.New("agentic: payload exceeds size cap")
      ErrShadeOutOfBounds  = errors.New("agentic: shade out of bounds")
      ErrFloorClamped      = errors.New("agentic: floor clamped to publisher floor")
      ErrConsentWithheld   = errors.New("agentic: consent withheld")
      ErrCOPPABlocked      = errors.New("agentic: COPPA blocks agent fanout")
  )
  ```

- [ ] **Step 3.4** Create `agentic/decisions.go` with pure data types:
  ```go
  package agentic

  import "time"

  type ApplyDecision struct {
      AgentID      string
      Intent       string
      Op           string
      Path         string
      Decision     string // "applied" | "rejected" | "superseded"
      Reason       string
      SupersededBy string
      LatencyMs    int64
  }

  type AgentCallStat struct {
      AgentID       string
      Lifecycle     Lifecycle
      Status        string // "ok" | "timeout" | "circuit_open" | "error"
      LatencyMs     int64
      MutationCount int
      ModelVersion  string
      Error         string
  }

  type DispatchResult struct {
      Mutations  []*pbMutationRef // see client.go for the pb alias
      AgentStats []AgentCallStat
      Truncated  bool
      DispatchedAt time.Time
  }
  ```
  (Define `pbMutationRef` as a type alias in `client.go` to keep imports tidy.)

- [ ] **Step 3.5** Create `agentic/assets/agents.schema.json` — JSON Schema Draft-07 for the document shape per PRD §7.4. Must validate:
  - `version`, `seller_id`, `seller_domain`, `agents[]` required
  - `agents[].id`, `agents[].role`, `agents[].endpoints[]`, `agents[].lifecycles[]`, `agents[].intents[]` required
  - Enums for `role`, `transport`, `auth`, `lifecycle`, `intent`
  - `priority` integer 0–1000
  - `tmax_ms` integer 1–500
  - Use `additionalProperties: false` at the top level to catch typos

- [ ] **Step 3.6** Create `agentic/assets/agents.json` Phase 1 default (empty allow-list):
  ```json
  {
    "$schema": "https://thenexusengine.com/schemas/agents.v1.json",
    "version": "1.0",
    "seller_id": "9131",
    "seller_domain": "thenexusengine.com",
    "contact": "agentic-ops@thenexusengine.com",
    "updated_at": "2026-04-27T00:00:00Z",
    "agents": []
  }
  ```

- [ ] **Step 3.7** Create `agentic/registry.go`:
  - Struct `AgentEndpoint{ID, Role, Vendor, RegistryRef, URL, Transport, Auth, Lifecycles, Intents, Priority, TmaxMs, Essential, Requires, DataProcessing}`.
  - Struct `Registry{ Document json.RawMessage; Agents []AgentEndpoint }`.
  - `func LoadRegistry(docPath, schemaPath string) (*Registry, error)` — reads both files, validates with `gojsonschema`, parses agents into typed slice.
  - `func (r *Registry) AgentsForLifecycle(lc Lifecycle) []AgentEndpoint` — filters by `lifecycles` membership.
  - `func (r *Registry) AgentsForIntent(intent pb.Intent) []AgentEndpoint` — for tests/inspection.
  - `func (r *Registry) DocumentBytes() []byte` — returns the original JSON bytes for HTTP serving.
  - Sort `Agents` by `Priority` ascending at load time so iteration order is deterministic for last-writer-wins (PRD R5.5.7, R5.5.8).

- [ ] **Step 3.8** Create `agentic/registry_test.go`:
  - Test happy path with one segmentation agent in fixture `testdata/agents-one.json`.
  - Test schema validation failure (wrong type, missing required field).
  - Test empty allow-list case (`agents: []` returns zero-length slice).
  - Test `AgentsForLifecycle` filtering.
  - Test priority sorting.

**Verification:**
- `go test ./agentic -run TestRegistry` passes.
- `go vet ./agentic/...` clean.

---

### Task 4: OriginatorStamper + AAMP envelope helpers

**Files:**
- Create: `agentic/originator.go`
- Create: `agentic/envelope.go`
- Create: `agentic/consent.go`
- Create: `agentic/originator_test.go`
- Create: `agentic/envelope_test.go`
- Create: `agentic/consent_test.go`

Steps:

- [ ] **Step 4.1** Create `agentic/originator.go`:
  ```go
  package agentic

  import pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"

  type OriginatorStamper struct {
      SellerID string // e.g. "9131"
  }

  func (s OriginatorStamper) StampRTBRequest(req *pb.RTBRequest, lc Lifecycle) {
      if req == nil { return }
      req.Lifecycle = lc.Proto()
      req.Originator = &pb.Originator{
          Type: pb.Originator_TYPE_SSP,
          Id:   s.SellerID,
      }
  }
  ```

- [ ] **Step 4.2** Create `agentic/envelope.go` with helpers to read/write `BidRequest.ext.aamp` JSON. Use `encoding/json` against `openrtb.BidRequest.Ext` (which is `json.RawMessage` in the existing repo's OpenRTB types — verify by reading `internal/openrtb/request.go`).

  Functions:
  - `WriteOutboundEnvelope(req *openrtb.BidRequest, sellerID string, lc Lifecycle, consent bool, agentsCalled []string)` — writes the full envelope to `ext.aamp` per PRD §7.3 "to bidders" shape.
  - `WriteAgentEnvelope(req *openrtb.BidRequest, sellerID string, lc Lifecycle, consent bool)` — writes the minimal envelope (no `agentsCalled`) per PRD §7.3 "to extension-point agents" shape.
  - `ReadInboundEnvelope(req *openrtb.BidRequest) (*Envelope, error)` — reads page-side originator/intentHints/disclosedAgents from prebid.
  - `WriteBidExt(bid *openrtb.Bid, applied []AgentApplied, shadingDelta float64, segmentsActivated int)` — writes attribution to `bid.ext.aamp`.

  Cap envelope size at 8 KB hard, 4 KB soft per PRD R6.2.3 (server-side mirror).

- [ ] **Step 4.3** Create `agentic/consent.go`:
  ```go
  package agentic

  import "github.com/thenexusengine/tne_springwire/internal/openrtb"

  // DeriveAgentConsent returns true iff the request permits agent processing
  // by non-essential agents. Mirrors the prebid-side derivation exactly.
  func DeriveAgentConsent(req *openrtb.BidRequest) bool {
      if req == nil { return false }
      if req.Regs != nil && req.Regs.COPPA == 1 { return false }
      // GPP applicable section opt-out → false
      // TCF Purpose 7 withheld → false
      // Default permissive otherwise; keep tight to existing privacy middleware
      return true
  }
  ```
  Wire to whatever the existing `privacyMiddleware` already exposes (check `internal/middleware/privacy.go` for an existing helper before re-implementing). If a helper exists, prefer calling it.

- [ ] **Step 4.4** Tests:
  - `TestStampRTBRequest_setsSSPOriginator` — verify `pb.Originator_TYPE_SSP` and `SellerID`.
  - `TestStampRTBRequest_setsLifecycle` — check both PUBLISHER_BID_REQUEST and DSP_BID_RESPONSE.
  - `TestWriteOutboundEnvelope_includesAgentsCalled` — bidder-bound envelope contains list.
  - `TestWriteAgentEnvelope_excludesAgentsCalled` — agent-bound envelope must NOT contain it (R5.4 confidentiality).
  - `TestEnvelope_softCap_dropsPageContextFirst` — at 4–8 KB, `pageContext` dropped before `intentHints`.
  - `TestEnvelope_hardCap_dropsEntireBlock` — over 8 KB → no `ext.aamp`.
  - `TestDeriveAgentConsent_COPPABlocks` — `coppa=1` → false.
  - `TestDeriveAgentConsent_TCFP7Withheld` — Purpose 7 denied → false.

**Verification:** `go test ./agentic -run "TestStampRTBRequest|TestWriteOutboundEnvelope|TestWriteAgentEnvelope|TestEnvelope_|TestDeriveAgentConsent"` passes.

---

### Task 5: Mutation Applier — per-intent handlers

**Files:**
- Create: `agentic/applier.go`
- Create: `agentic/applier_test.go`
- Create: `agentic/testdata/mutations/` (fixture directory)

Steps:

- [ ] **Step 5.1** In `agentic/applier.go`, declare:
  ```go
  type ApplierConfig struct {
      MaxMutationsPerResponse int     // default 64
      MaxIDsPerPayload        int     // default 256
      ShadeMinFraction        float64 // default 0.5; bid.price * X is the floor
      DisableShadeIntent      bool    // env-gated kill switch for BID_SHADE
      PublisherFloorLookup    func(impID string) float64 // for floor clamping (R5.5.9)
  }

  type Applier struct { cfg ApplierConfig }

  func NewApplier(cfg ApplierConfig) *Applier { … }

  func (a *Applier) Apply(
      req *openrtb.BidRequest,
      rsp *openrtb.BidResponse,
      muts []*pb.Mutation,
      lc Lifecycle,
      agentMeta map[int]AgentEndpoint, // index in muts → agent that produced it
  ) []ApplyDecision
  ```

- [ ] **Step 5.2** Implement deterministic ordering per PRD R5.5.8:
  - Sort `muts` by `(intent, agent_priority, agent_id)` before any handler runs.
  - Capture original index → use to track conflicts for R5.5.7 last-writer-wins logging.

- [ ] **Step 5.3** Top-level dispatch table:
  ```go
  type intentHandler func(*openrtb.BidRequest, *openrtb.BidResponse, *pb.Mutation, AgentEndpoint, ApplierConfig) ApplyDecision

  var handlers = map[pb.Intent]struct {
      lifecycle Lifecycle
      fn        intentHandler
  }{
      pb.Intent_ACTIVATE_SEGMENTS: {LifecyclePublisherBidRequest, applyActivateSegments},
      pb.Intent_ACTIVATE_DEALS:    {LifecyclePublisherBidRequest, applyActivateDeals},
      pb.Intent_SUPPRESS_DEALS:    {LifecyclePublisherBidRequest, applySuppressDeals},
      pb.Intent_ADJUST_DEAL_FLOOR: {LifecyclePublisherBidRequest, applyAdjustDealFloor},
      pb.Intent_ADJUST_DEAL_MARGIN:{LifecycleDSPBidResponse,      applyAdjustDealMargin},
      pb.Intent_BID_SHADE:         {LifecycleDSPBidResponse,      applyBidShade},
      pb.Intent_ADD_METRICS:       {LifecyclePublisherBidRequest, applyAddMetrics},
  }
  ```

- [ ] **Step 5.4** Top-level rules (R5.5.1–R5.5.10) enforced before any handler:
  - Whitelist intent (R5.5.1) — unknown → reject `unsupported_intent`.
  - Lifecycle gate (R5.5.2) — wrong lifecycle → reject `wrong_lifecycle`.
  - Op gate (R5.5.3) — `OPERATION_UNSPECIFIED` → reject `op_unspecified`.
  - Mutation cap (R5.5.5) — index ≥ `MaxMutationsPerResponse` → reject `mutation_cap_exceeded`.
  - Payload size cap (R5.5.6) — `IDsPayload.id` length > `MaxIDsPerPayload` → reject `payload_size_exceeded`.

- [ ] **Step 5.5** `applyActivateSegments`:
  - Path must match `user.data[*].segment[*]` (R5.5.4).
  - Append IDs to `req.User.Data[]` as a new `Data{ID: agent_id, Segment: [{ID: id}]}` block.
  - Dedupe by segment ID across the request.

- [ ] **Step 5.6** `applyActivateDeals`:
  - Path must match `imp[*].pmp.deals[*]`.
  - For `op=ADD`: append the deal IDs to each imp's `pmp.deals` if not already present.
  - For `op=REPLACE`: replace the entire `pmp.deals` slice.

- [ ] **Step 5.7** `applySuppressDeals`:
  - Same path constraint.
  - Remove deal IDs from each imp's `pmp.deals`.

- [ ] **Step 5.8** `applyAdjustDealFloor`:
  - Payload `AdjustDealPayload.bidfloor`.
  - Find matching deal in `imp.pmp.deals`; replace `bidfloor`.
  - If new floor < `cfg.PublisherFloorLookup(imp.id)` → clamp to publisher floor; decision `applied` with reason `floor_clamped` (R5.5.9).

- [ ] **Step 5.9** `applyAdjustDealMargin`:
  - DSP_BID_RESPONSE only.
  - Stash margin info on a side-channel keyed by deal ID; the existing `applyBidMultiplier` (line 1132) hook reads it. (See Task 9 for wiring.)

- [ ] **Step 5.10** `applyBidShade`:
  - DSP_BID_RESPONSE only.
  - Honor `cfg.DisableShadeIntent` — if true, reject all `BID_SHADE` with reason `shade_disabled` (per OQ3 — code path always present, ENV-gated off in production until trust is established).
  - `new_price > original_price` → reject `shade_out_of_bounds` (R5.5.10 — SSPs cannot raise bids).
  - `new_price < original_price * cfg.ShadeMinFraction` → reject `shade_out_of_bounds`.
  - Otherwise: replace the matching `bid.price` and accumulate `shadingDelta` for envelope writeback.

- [ ] **Step 5.11** `applyAddMetrics`:
  - Append `MetricsPayload.metric[]` items to `imp[*].metric[]`.
  - Cap to 32 metrics per impression as a defensive bound.

- [ ] **Step 5.12** Conflict resolution (R5.5.7) — when two `REPLACE`s collide on the same `(intent, path)`:
  - Lower `agent_priority` first in sort order, so iteration applies higher priority last.
  - Record the lower-priority decision as `superseded` with `SupersededBy=<agent_id of winner>` (don't drop silently).

- [ ] **Step 5.13** Tests in `agentic/applier_test.go` — one test per intent, plus:
  - `TestApply_unsupportedIntent_rejects` — fake intent value 999 rejected.
  - `TestApply_wrongLifecycle_rejects` — `BID_SHADE` during PUBLISHER_BID_REQUEST rejected.
  - `TestApply_opUnspecified_rejects`
  - `TestApply_mutationCap_truncates`
  - `TestApply_payloadSizeCap_rejects`
  - `TestApply_floorClamping`
  - `TestApply_shadeOutOfBounds_rejectsRaise`
  - `TestApply_shadeOutOfBounds_rejectsTooLow`
  - `TestApply_shadeDisabledByConfig_rejects`
  - `TestApply_conflict_lastWriterWins_logsSuperseded`
  - `TestApply_deterministicOrder` — same input → same output across N runs.

**Verification:** `go test ./agentic -run TestApply` passes; coverage on each intent handler ≥ 90%.

---

### Task 6: ExtensionPointClient — gRPC fan-out with tmax + breaker

**Files:**
- Create: `agentic/client.go`
- Create: `agentic/fake_agent_test.go` (fixture used by client_test + integration_test)
- Create: `agentic/client_test.go`

Steps:

- [ ] **Step 6.1** In `agentic/client.go` declare:
  ```go
  type ClientConfig struct {
      DefaultTmaxMs       int
      AuctionSafetyMs     int    // subtract from remaining auction budget when computing per-call deadline
      APIKey              string // global x-aamp-key
      PerAgentAPIKeys     map[string]string
      CircuitFailureRate  float64
      CircuitMinCalls     int
      CircuitHalfOpenSec  int
      MaxRecvMsgBytes     int    // 4 MB default
  }

  type Client struct {
      reg      *Registry
      cfg      ClientConfig
      conns    map[string]*grpc.ClientConn   // keyed by agent_id
      stubs    map[string]pb.RTBExtensionPointClient
      breakers map[string]*idr.CircuitBreaker
      stamper  OriginatorStamper
      mu       sync.RWMutex
  }
  ```

- [ ] **Step 6.2** `NewClient(reg *Registry, cfg ClientConfig, breakerSet *idr.CircuitBreakerSet, stamper OriginatorStamper) (*Client, error)`:
  - For each `AgentEndpoint`, dial gRPC eagerly (`grpc.NewClient(url, ...)`) with:
    - `keepalive.ClientParameters{Time: 30s, Timeout: 10s, PermitWithoutStream: true}`
    - `MaxCallRecvMsgSize(cfg.MaxRecvMsgBytes)` (default 4 MB)
    - TLS credentials when scheme is `grpcs://`; refuse `grpc://` when `AGENTIC_ENV=production`.
  - Allocate one `*idr.CircuitBreaker` per agent_id with prefix `agentic:<agent_id>`.
  - Store in `c.conns` and `c.stubs`.

- [ ] **Step 6.3** `(c *Client) Dispatch(ctx context.Context, req *pb.RTBRequest, lc Lifecycle) DispatchResult`:
  - Snapshot `agents := c.reg.AgentsForLifecycle(lc)`.
  - If empty → return zero `DispatchResult` with no allocation (R5.1.6).
  - Compute per-call deadline: `min(agent.TmaxMs, cfg.DefaultTmaxMs, remainingFromCtx − cfg.AuctionSafetyMs)`.
  - Spawn one goroutine per eligible agent. Channel-collect mutations + AgentCallStat. Hard wait on a master timer = max per-call deadline; late results dropped.
  - For each agent goroutine:
    - Check breaker — if open, append `AgentCallStat{Status:"circuit_open"}` and return.
    - Stamp request via `c.stamper.StampRTBRequest(req, lc)` once before goroutines spawn (req is read-only inside goroutines).
    - Wrap with `metadata.AppendToOutgoingContext(ctx, "x-aamp-key", c.keyFor(agent))`.
    - Call `c.stubs[agent.ID].GetMutations(ctx, req)`. Record latency; on error mark breaker failure; on success mark breaker success.
  - Return `DispatchResult{Mutations, AgentStats, Truncated}`.

- [ ] **Step 6.4** `(c *Client) Close() error` — closes all `grpc.ClientConn` and returns the joined error.

- [ ] **Step 6.5** Defensive panic recovery in each agent goroutine — recover and emit `AgentCallStat{Status:"error", Error:"panic: …"}` so a misbehaving stub never tears down the auction.

- [ ] **Step 6.6** `agentic/fake_agent_test.go` — exported helper `StartFakeAgent(t *testing.T, behaviour FakeBehaviour) (addr string, stop func())`:
  - Spins up a `grpc.NewServer()` on `127.0.0.1:0`.
  - Registers a stub `RTBExtensionPointServer` whose `GetMutations` is configurable via the `FakeBehaviour` struct: `{ReturnMutations []*pb.Mutation, SleepMs int, ReturnError error}`.
  - Returns the listener addr and a `stop()` closure.

- [ ] **Step 6.7** `agentic/client_test.go`:
  - `TestDispatch_emptyRegistry_noOp` — zero allocations check.
  - `TestDispatch_singleAgent_appliesAndStamps`.
  - `TestDispatch_tmaxBudget_dropsLate` — agent sleeps longer than tmax → `Truncated=true`, mutations not present.
  - `TestDispatch_circuitOpen_skipsAgent` — induce 20 failures, breaker opens, next dispatch records `circuit_open`.
  - `TestDispatch_panicInStub_recovered`.
  - `TestDispatch_perAgentAPIKey_overridesGlobal` — verify metadata header on the wire (use a real fake server; intercept ctx).
  - `TestDispatch_concurrent_safe` — N goroutines × M dispatches under `-race`.

**Verification:** `go test ./agentic -run TestDispatch -race` passes.

---

### Task 7: HTTP endpoints — agents.json + admin

**Files:**
- Create: `agentic/endpoints/agents_json.go`
- Create: `agentic/endpoints/agents_admin.go`
- Create: `agentic/endpoints/agents_json_test.go`

Steps:

- [ ] **Step 7.1** `agentic/endpoints/agents_json.go` — handler that mirrors the sellers.json pattern at `internal/endpoints/sellers_json.go`:
  ```go
  package endpoints

  import (
      "net/http"
      "github.com/thenexusengine/tne_springwire/agentic"
  )

  type AgentsJSONHandler struct {
      reg     *agentic.Registry
      enabled bool
  }

  func NewAgentsJSONHandler(reg *agentic.Registry, enabled bool) *AgentsJSONHandler {
      return &AgentsJSONHandler{reg: reg, enabled: enabled}
  }

  func (h *AgentsJSONHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
      if !h.enabled || h.reg == nil {
          http.NotFound(w, r) // PRD §8.5: 404 when disabled
          return
      }
      w.Header().Set("Content-Type", "application/json; charset=utf-8")
      w.Header().Set("Access-Control-Allow-Origin", "*")
      w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
      w.Header().Set("Cache-Control", "public, max-age=3600")
      if r.Method == http.MethodOptions { w.WriteHeader(http.StatusOK); return }
      if r.Method != http.MethodGet { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
      _, _ = w.Write(h.reg.DocumentBytes())
  }
  ```

- [ ] **Step 7.2** `agentic/endpoints/agents_admin.go` — read-only admin handler at `/admin/agents` and `/admin/agents/{id}`:
  - Wrap with the existing `adminAuth` middleware in `cmd/server/server.go` when registering.
  - JSON list view: `{"agents": [...]}` projection of `Registry.Agents` minus the API-key field (defensive — though we never store keys in the registry, this codifies the redaction policy).
  - Single-agent view: `{"agent": {...}}` for `/admin/agents/{id}`.
  - 404 on unknown agent_id.

- [ ] **Step 7.3** Tests in `agents_json_test.go`:
  - `TestAgentsJSON_disabledReturns404` — `enabled=false` → 404.
  - `TestAgentsJSON_enabledServesDoc` — body equals `Registry.DocumentBytes()`.
  - `TestAgentsJSON_setsCorsAndCacheHeaders`.
  - `TestAgentsJSON_handlesOptions`.
  - `TestAgentsJSON_rejectsPost`.
  - `TestAgentsAdmin_listProjectsAgentsWithoutSecrets`.

**Verification:** `go test ./agentic/endpoints/...` passes.

---

### Task 8: Wire ServerConfig.Agentic + env vars

**Files:**
- Modify: `cmd/server/config.go`

Steps:

- [ ] **Step 8.1** Add the `AgenticConfig` sub-struct (mirroring PRD §8.3):
  ```go
  type AgenticConfig struct {
      Enabled                 bool
      AgentsPath              string
      SchemaPath              string
      TmaxMs                  int
      AuctionSafetyMs         int
      SellerID                string
      APIKey                  string
      PerAgentAPIKeys         map[string]string
      CircuitFailureRate      float64
      CircuitMinCalls         int
      CircuitHalfOpenSeconds  int
      MaxMutationsPerResponse int
      MaxIDsPerPayload        int
      DisableShadeIntent      bool

      // Phase 2 — reserved
      GRPCPort   int
      MCPEnabled bool
  }
  ```

- [ ] **Step 8.2** Add `Agentic *AgenticConfig` field to `ServerConfig`. Pointer so we can detect "not configured" cleanly.

- [ ] **Step 8.3** In `ParseConfig()`, after the existing parse block, add:
  ```go
  if getEnvBoolOrDefault("AGENTIC_ENABLED", false) {
      cfg.Agentic = &AgenticConfig{
          Enabled:                 true,
          AgentsPath:              getEnvOrDefault("AGENTIC_AGENTS_PATH",  "agentic/assets/agents.json"),
          SchemaPath:              getEnvOrDefault("AGENTIC_SCHEMA_PATH",  "agentic/assets/agents.schema.json"),
          TmaxMs:                  getEnvIntOrDefault("AGENTIC_TMAX_MS",   30),
          AuctionSafetyMs:         getEnvIntOrDefault("AGENTIC_SAFETY_MS", 50),
          SellerID:                getEnvOrDefault("AGENTIC_SELLER_ID",    "9131"),
          APIKey:                  os.Getenv("AGENTIC_API_KEY"),
          PerAgentAPIKeys:         parseAgenticPerAgentKeys(),
          CircuitFailureRate:      getEnvFloatOrDefault("AGENTIC_CIRCUIT_FAILURE_RATE", 0.5),
          CircuitMinCalls:         getEnvIntOrDefault("AGENTIC_CIRCUIT_MIN_CALLS", 20),
          CircuitHalfOpenSeconds:  getEnvIntOrDefault("AGENTIC_CIRCUIT_HALFOPEN_S", 30),
          MaxMutationsPerResponse: getEnvIntOrDefault("AGENTIC_MAX_MUTATIONS_PER_RESPONSE", 64),
          MaxIDsPerPayload:        getEnvIntOrDefault("AGENTIC_MAX_IDS_PER_PAYLOAD", 256),
          DisableShadeIntent:      getEnvBoolOrDefault("AGENTIC_DISABLE_SHADE", true), // OQ3 default
          GRPCPort:                getEnvIntOrDefault("AGENTIC_GRPC_PORT", 0),
          MCPEnabled:              getEnvBoolOrDefault("AGENTIC_MCP_ENABLED", false),
      }
  }
  ```

- [ ] **Step 8.4** Add helpers `getEnvFloatOrDefault` and `parseAgenticPerAgentKeys` (the latter scans env for `AGENTIC_API_KEY_<AGENT_ID>` prefix and builds the map).

- [ ] **Step 8.5** Extend `ServerConfig.Validate()`:
  - When `c.Agentic != nil && c.Agentic.Enabled`:
    - `AgentsPath` non-empty AND file exists.
    - `SchemaPath` file exists.
    - `TmaxMs` ∈ [5, 500].
    - `SellerID` non-empty.
    - `APIKey` non-empty when **any** agent in the registry declares `auth=api_key_header` (defer this check to boot once Registry is loaded — keep config-level validation focused on shape).

- [ ] **Step 8.6** Unit tests in `cmd/server/config_test.go`:
  - `TestParseConfig_AgenticDisabledByDefault` — env unset → `cfg.Agentic == nil`.
  - `TestParseConfig_AgenticEnabled` — env set → struct populated with defaults.
  - `TestValidateAgentic_TmaxBounds`.
  - `TestParseAgenticPerAgentKeys`.

**Verification:** `go test ./cmd/server/... -run "Agentic"` passes.

---

### Task 9: Hook Exchange.RunAuction (Hook A + Hook B)

**Files:**
- Modify: `internal/exchange/exchange.go`
- Modify: `cmd/server/server.go` (DI wiring — see Step 9.7)

Steps:

- [ ] **Step 9.1** Read `internal/exchange/exchange.go` lines 251–310 (`New(...)`, `SetMetrics`, `Close`) and 1351–1820 (`RunAuction` body). Confirm Hook A site (just before line 1596 `callBiddersWithFPD`) and Hook B site (just before line 1797 `runAuctionLogic`).

- [ ] **Step 9.2** Add new fields on `Exchange`:
  ```go
  type Exchange struct {
      // … existing fields …
      agenticClient  *agentic.Client
      agenticApplier *agentic.Applier
      agenticStamper agentic.OriginatorStamper
      agenticEnabled bool
  }
  ```

- [ ] **Step 9.3** Add a builder/setter on `Exchange` (don't break the `New(registry, config)` signature — too widely used):
  ```go
  func (e *Exchange) WithAgentic(c *agentic.Client, a *agentic.Applier, s agentic.OriginatorStamper) *Exchange {
      e.agenticClient = c
      e.agenticApplier = a
      e.agenticStamper = s
      e.agenticEnabled = c != nil && a != nil
      return e
  }
  ```

- [ ] **Step 9.4** **Hook A** — insert before line 1596 (`results := e.callBiddersWithFPD(...)`):
  ```go
  if e.agenticEnabled {
      consent := agentic.DeriveAgentConsent(req.BidRequest)
      if consent {
          rtbReq := agentic.WrapAsRTBRequest(req.BidRequest, agentic.LifecyclePublisherBidRequest)
          dr := e.agenticClient.Dispatch(ctx, rtbReq, agentic.LifecyclePublisherBidRequest)
          decisions := e.agenticApplier.Apply(req.BidRequest, nil, dr.Mutations, agentic.LifecyclePublisherBidRequest, dr.AgentIndex())
          logAgenticDecisions(ctx, req.BidRequest.ID, agentic.LifecyclePublisherBidRequest, decisions, dr.AgentStats)
          agentic.WriteOutboundEnvelope(req.BidRequest, e.agenticStamper.SellerID, agentic.LifecyclePublisherBidRequest, true, dr.AgentIDs())
      }
  }
  ```
  - `WrapAsRTBRequest` is a helper in `agentic/envelope.go` (Task 4) that constructs a `pb.RTBRequest` from the OpenRTB BidRequest — implement it as part of Task 9 if it wasn't already.
  - `logAgenticDecisions` is a private helper in this file — emits the per-mutation log lines per PRD §5.8.

- [ ] **Step 9.5** **Hook B** — insert before line 1797 (`auctionedBids := e.runAuctionLogic(validBids, impFloors)`):
  ```go
  if e.agenticEnabled {
      consent := agentic.DeriveAgentConsent(req.BidRequest)
      if consent {
          pseudoRsp := buildPseudoBidResponse(validBids)
          rtbReq := agentic.WrapAsRTBRequestWithResponse(req.BidRequest, pseudoRsp, agentic.LifecycleDSPBidResponse)
          dr := e.agenticClient.Dispatch(ctx, rtbReq, agentic.LifecycleDSPBidResponse)
          decisions := e.agenticApplier.Apply(req.BidRequest, pseudoRsp, dr.Mutations, agentic.LifecycleDSPBidResponse, dr.AgentIndex())
          logAgenticDecisions(ctx, req.BidRequest.ID, agentic.LifecycleDSPBidResponse, decisions, dr.AgentStats)
          applyShadeAndMarginToValidBids(validBids, decisions, pseudoRsp)
      }
  }
  ```
  - `buildPseudoBidResponse` constructs an OpenRTB BidResponse from the in-flight `validBids` so the applier can mutate prices. New private helper at the bottom of this file.
  - `applyShadeAndMarginToValidBids` reads the applier's decisions back and patches the `[]ValidatedBid` in place. Margin decisions stash on a side-channel map read by the existing `applyBidMultiplier` (line 1132).

- [ ] **Step 9.6** Defensive recovery — wrap each hook block in a deferred `recover()` that logs `agentic.applier_panic` and continues. Auction must never fail because of agentic (PRD §9.3).

- [ ] **Step 9.7** In `cmd/server/server.go`, after the existing `exchange.New(...)` call:
  ```go
  if cfg.Agentic != nil && cfg.Agentic.Enabled {
      reg, err := agentic.LoadRegistry(cfg.Agentic.AgentsPath, cfg.Agentic.SchemaPath)
      if err != nil {
          log.Fatal().Err(err).Msg("failed to load agents registry")
      }
      stamper := agentic.OriginatorStamper{SellerID: cfg.Agentic.SellerID}
      client, err := agentic.NewClient(reg, agentic.ClientConfig{
          DefaultTmaxMs:      cfg.Agentic.TmaxMs,
          AuctionSafetyMs:    cfg.Agentic.AuctionSafetyMs,
          APIKey:             cfg.Agentic.APIKey,
          PerAgentAPIKeys:    cfg.Agentic.PerAgentAPIKeys,
          CircuitFailureRate: cfg.Agentic.CircuitFailureRate,
          CircuitMinCalls:    cfg.Agentic.CircuitMinCalls,
          CircuitHalfOpenSec: cfg.Agentic.CircuitHalfOpenSeconds,
          MaxRecvMsgBytes:    4 * 1024 * 1024,
      }, breakerSet, stamper)
      if err != nil {
          log.Fatal().Err(err).Msg("failed to construct agentic client")
      }
      applier := agentic.NewApplier(agentic.ApplierConfig{
          MaxMutationsPerResponse: cfg.Agentic.MaxMutationsPerResponse,
          MaxIDsPerPayload:        cfg.Agentic.MaxIDsPerPayload,
          DisableShadeIntent:      cfg.Agentic.DisableShadeIntent,
          ShadeMinFraction:        0.5,
          PublisherFloorLookup:    s.publisher.FloorLookup, // wire to publisher store
      })
      s.exchange.WithAgentic(client, applier, stamper)
      s.agentsRegistry = reg
  }
  ```

- [ ] **Step 9.8** Register the new HTTP routes adjacent to sellers.json (around line 403–406):
  ```go
  agentsHandler := agenticEndpoints.NewAgentsJSONHandler(s.agentsRegistry, cfg.Agentic != nil && cfg.Agentic.Enabled)
  mux.Handle("/agents.json", agentsHandler)
  mux.Handle("/.well-known/agents.json", agentsHandler)
  log.Info().Msg("Agents.json endpoints registered: /agents.json, /.well-known/agents.json")

  if cfg.Agentic != nil && cfg.Agentic.Enabled {
      adminAgents := agenticEndpoints.NewAgentsAdminHandler(s.agentsRegistry)
      mux.Handle("/admin/agents", adminAuth(adminAgents))
      mux.Handle("/admin/agents/", adminAuth(adminAgents))
  }
  ```

- [ ] **Step 9.9** Add an `Exchange.Close` cleanup branch that closes the agentic client (so we don't leak gRPC conns on shutdown).

- [ ] **Step 9.10** In `cmd/server/server.go` graceful-shutdown path, call `s.exchange.Close()` (likely already does — just verify the gRPC conns close).

**Verification:**
- `go build ./...` clean.
- `go vet ./...` clean.
- Existing exchange tests still pass: `go test ./internal/exchange/... -run RunAuction`.

---

### Task 10: Integration test — full auction with fake agent

**Files:**
- Create: `agentic/integration_test.go`
- Create: `agentic/testdata/agents-integration.json`

Steps:

- [ ] **Step 10.1** `agentic/testdata/agents-integration.json` — fixture with one segmentation agent at `localhost:0` (port resolved at test runtime via env-substitution after `StartFakeAgent`).

- [ ] **Step 10.2** `agentic/integration_test.go`:
  - Build a real `Exchange` from `internal/exchange` with a stub bidder registry (one always-bidding bidder).
  - Spin up a fake agent (`StartFakeAgent`) returning a single `ACTIVATE_SEGMENTS` mutation with one segment ID.
  - Construct `Client`, `Applier`, `OriginatorStamper`, `Registry` (writing the resolved port into a temp `agents.json`).
  - Wire via `Exchange.WithAgentic(...)`.
  - Submit a synthetic `AuctionRequest` and assert:
    - The bidder received a `BidRequest` with the segment activated in `user.data[*].segment[*]`.
    - `BidRequest.ext.aamp` envelope present, `originator.type=SSP`, `originator.id=9131`, `agentsCalled` includes the fake agent's ID.
    - The `AuctionResponse` carries `bid.ext.aamp.agentsApplied` listing the fake agent.

- [ ] **Step 10.3** Add tmax-violation variant: fake agent sleeps 200 ms with `AGENTIC_TMAX_MS=20`. Assert auction completes, `dispatch_truncated_total` increments, `bid.ext.aamp.agentsApplied` does NOT include the slow agent.

- [ ] **Step 10.4** Add COPPA hard-block variant: set `regs.coppa=1`. Assert no agent fanout (fake agent records zero calls).

- [ ] **Step 10.5** Add disabled-shade variant: configure `DisableShadeIntent=true` and a fake agent that returns a `BID_SHADE` mutation. Assert mutation rejected with reason `shade_disabled`.

- [ ] **Step 10.6** Wire the test under `-race`. Run for `-count=10` to catch flakes. Document any flake mitigation notes in the test file header.

**Verification:**
- `go test ./agentic -run TestIntegration -race -count=10` passes.

---

### Task 11: Stretch — prebid-adapter scaffold

This task is the cycle stretch goal (PRD §10.1.s1). Drop if Phase 1 server work runs long. None of Tasks 1–10 depend on it.

**Files:**
- Create: `agentic/prebid-adapter/README.md`
- Create: `agentic/prebid-adapter/package.json`
- Create: `agentic/prebid-adapter/src/tneCatalystBidAdapter.js`
- Create: `agentic/prebid-adapter/src/tneCatalystAgenticEnvelope.js`
- Create: `agentic/prebid-adapter/src/agenticConsent.js`
- Create: `agentic/prebid-adapter/src/agentDiscoveryRtdProvider.js`
- Create: `agentic/prebid-adapter/test/spec/tneCatalystBidAdapter_spec.js`
- Create: `agentic/prebid-adapter/test/spec/agenticEnvelope_spec.js`
- Create: `agentic/prebid-adapter/test/spec/agenticConsent_spec.js`
- Create: `agentic/prebid-adapter/examples/pbjs-config-basic.html`
- Create: `agentic/prebid-adapter/examples/pbjs-config-agentic.html`

Steps:

- [ ] **Step 11.1** `package.json` with `"private": true`, no `publishConfig`. Dev deps: `mocha`, `chai`, `sinon`, `karma`, `karma-chrome-launcher`, `karma-mocha`. Pin to whatever Prebid uses upstream.

- [ ] **Step 11.2** `src/tneCatalystBidAdapter.js` — a minimal Prebid bidder adapter (`spec` object with `code: 'tneCatalyst'`, `isBidRequestValid`, `buildRequests`, `interpretResponse`, `getUserSyncs`). On `buildRequests`, call `tneCatalystAgenticEnvelope.write(ortb2, params)` to attach `ortb2.ext.aamp`. On `interpretResponse`, copy `bid.ext.aamp` to `bid.meta.aamp` and emit `agentMutationApplied` events per PRD R6.3.3.

- [ ] **Step 11.3** `src/tneCatalystAgenticEnvelope.js` — pure function building the page-side envelope per PRD §6.2 with the 4 KB/8 KB caps from R6.2.3.

- [ ] **Step 11.4** `src/agenticConsent.js` — `deriveConsent(bidRequest)` per PRD §6.4. No external deps, must be pure for unit testing.

- [ ] **Step 11.5** `src/agentDiscoveryRtdProvider.js` — minimal RTD module skeleton per PRD §6.5. Stretch within stretch — implement only if there's time.

- [ ] **Step 11.6** Unit tests for envelope construction (every input matrix from PRD R6.8.3) and consent derivation. **Do not** ship a full karma config Phase 1 — `mocha test/spec/agenticEnvelope_spec.js` running under Node is enough to claim "tests pass" for the scaffold.

- [ ] **Step 11.7** `examples/pbjs-config-basic.html` and `examples/pbjs-config-agentic.html` — runnable HTML demos against a local Catalyst with `AGENTIC_ENABLED=true`. Reference but do not bundle Prebid (publisher uses their own).

- [ ] **Step 11.8** `README.md`:
  - Status: "scaffold, not published"
  - Module dependencies: `core@>=8.0`, `currency`
  - Wire-up snippet for `pbjs.bidderSettings`
  - Pointer to PRD §6 for full requirement list

**Verification (stretch):**
- `cd agentic/prebid-adapter && npm install && npm test` passes.
- `examples/pbjs-config-agentic.html` loads in a browser without console errors when pointed at `localhost:8000`.

---

### Task 12: Build, test, document, push

**Files:**
- Create: `agentic/README.md`
- Create: `agentic/docs/README.md`
- Create: `agentic/docs/agent-vendor-onboarding.md`
- Modify: `README.md` (top-level — add a one-paragraph reference to `agentic/`)
- Modify: `Makefile` (already touched in Task 2)

Steps:

- [ ] **Step 12.1** `agentic/README.md`:
  - One-paragraph overview ("Phase 1 outbound ARTF v1.0 integration").
  - Feature flag instructions (`AGENTIC_ENABLED=true`).
  - Pointer to PRD + plan.
  - Onboarding-an-agent checklist (5 steps, pointing to `docs/agent-vendor-onboarding.md`).
  - Regen-protos quick-reference.

- [ ] **Step 12.2** `agentic/docs/agent-vendor-onboarding.md` — concrete walk-through:
  1. Vendor registers their endpoint URL + role.
  2. We add an entry to `agentic/assets/agents.json`.
  3. Issue an API key, set `AGENTIC_API_KEY_<AGENT_ID>` in the deploy env.
  4. Stage flip → smoke against fake-agent suite.
  5. Prod flip with breaker thresholds set conservatively.

- [ ] **Step 12.3** Append to top-level `README.md` (one paragraph, ~5 lines): "Catalyst integrates the IAB Tech Lab Agentic RTB Framework (ARTF v1.0) under `agentic/`. Default-off; enable with `AGENTIC_ENABLED=true`. See `agentic/README.md` and `docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md`."

- [ ] **Step 12.4** Run the full local check:
  ```bash
  go vet ./...
  go build ./...
  go test ./agentic/... -race -count=1
  go test ./internal/exchange/... -count=1
  go test ./cmd/server/... -count=1
  ```

- [ ] **Step 12.5** If any test fails: do NOT mark this plan task complete. Diagnose, patch, re-run. Repeat until clean.

- [ ] **Step 12.6** Final smoke (manual, optional Phase 1):
  ```bash
  AGENTIC_ENABLED=true \
  AGENTIC_AGENTS_PATH=agentic/assets/agents.json \
  AGENTIC_SCHEMA_PATH=agentic/assets/agents.schema.json \
  go run ./cmd/server &
  curl -s http://localhost:8000/.well-known/agents.json | jq .seller_id
  # expect: "9131"
  ```

- [ ] **Step 12.7** Commit on `claude/integrate-iab-agentic-protocol-6bvtJ` with logical groupings (one commit per task block where reasonable; squash-on-merge is fine):
  - `feat(agentic): vendor ARTF protos`
  - `feat(agentic): add grpc + protobuf deps, generate Go code`
  - `feat(agentic): build core package (registry, applier, client, stamper)`
  - `feat(agentic): add agents.json + admin endpoints`
  - `feat(agentic): wire ServerConfig.Agentic + env vars`
  - `feat(agentic): hook RunAuction at PUBLISHER_BID_REQUEST + DSP_BID_RESPONSE`
  - `test(agentic): integration test against in-process fake agent`
  - `feat(agentic): scaffold prebid-adapter (stretch)` — optional
  - `docs(agentic): README + agent-vendor onboarding`

- [ ] **Step 12.8** Push the branch (no PR, per D8):
  ```bash
  git push -u origin claude/integrate-iab-agentic-protocol-6bvtJ
  ```

**Verification:**
- `git status` clean.
- All commits pushed.
- Branch readable end-to-end: someone reading from PRD → plan → commits should be able to reproduce the build.

---

## Phase 1 Done When

- [ ] `go build ./...` clean
- [ ] `go test ./...` clean (full repo, not just `agentic/`)
- [ ] `AGENTIC_ENABLED=false` → zero behaviour change observed in load test (P95 latency identical to control branch)
- [ ] `AGENTIC_ENABLED=true` + zero agents → P95 latency ≤ +5 ms vs control
- [ ] `AGENTIC_ENABLED=true` + one fake agent → mutations appear in `bid.ext.aamp.agentsApplied`, audit log emits per-mutation lines, Prom metrics tick
- [ ] `/.well-known/agents.json` returns 200 + valid JSON when enabled, 404 when disabled
- [ ] PRD acceptance KPIs from §12.1 hit on staging
- [ ] Branch pushed; no PR

---

## Phase 2 (Out of Cycle)

See PRD §10.2. Do NOT start without explicit approval — Phase 1 must run in production for ≥ 7 days at zero auction-failure rate first.





