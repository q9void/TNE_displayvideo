# `agentic/` — IAB AAMP / ARTF v1.0 integration

This umbrella directory houses TNE Catalyst's integration with the IAB Tech
Lab Agentic RTB Framework (ARTF v1.0). All net-new code for the feature
lives here. Three integration files outside the umbrella are modified at
known call sites: `internal/exchange/exchange.go`, `cmd/server/server.go`,
`cmd/server/config.go` (see PRD §7.0 for the rationale).

## Status

**Phase 1 — outbound, default-off.** Ships:
- ARTF protos vendored at pinned upstream SHA
- Outbound `RTBExtensionPoint` gRPC client with tmax + circuit breaker
- Mutation applier whitelisting the seven supported ARTF intents
- Two lifecycle hooks in `Exchange.RunAuction` (PUBLISHER_BID_REQUEST + DSP_BID_RESPONSE)
- `/.well-known/agents.json` + `/agents.json` discovery
- `/admin/agents` + `/admin/agents/{id}` (read-only, behind admin auth)
- Per-mutation structured audit log + per-call summary
- 68 unit/integration tests passing under `-race`
- Prebid.js adapter scaffold (in-repo only; not published)

**Phase 2 — out of scope this branch.** Inbound seller-agent gRPC server,
MCP transport, per-publisher overrides, prebid adapter publication. See
PRD §10.2.

## Quick start

```bash
# Default: agentic is OFF and the code path is unreachable.
go run ./cmd/server

# Enable outbound agentic with the empty allow-list (safe — no agents
# called yet, but the path activates and ext.aamp envelopes are written).
AGENTIC_ENABLED=true go run ./cmd/server

# Add a real agent: edit agentic/assets/agents.json, then provide an API key:
AGENTIC_ENABLED=true \
  AGENTIC_API_KEY=test-key \
  go run ./cmd/server

curl -s http://localhost:8000/.well-known/agents.json | jq .seller_id
# expect: "9131"
```

## Environment variables

| Var | Default | Meaning |
|---|---|---|
| `AGENTIC_ENABLED` | `false` | Master kill switch. Off ⇒ code path unreachable. |
| `AGENTIC_AGENTS_PATH` | `agentic/assets/agents.json` | Registry document path. |
| `AGENTIC_SCHEMA_PATH` | `agentic/assets/agents.schema.json` | JSON Schema for validation. |
| `AGENTIC_TMAX_MS` | `30` | Global per-call deadline ceiling. |
| `AGENTIC_SAFETY_MS` | `50` | Subtracted from auction-context deadline. |
| `AGENTIC_SELLER_ID` | `9131` | Emitted as `Originator.id`. |
| `AGENTIC_API_KEY` | `""` | Global `x-aamp-key` gRPC metadata header. |
| `AGENTIC_API_KEY_<AGENT_ID>` | `""` | Per-agent override of API key. |
| `AGENTIC_CIRCUIT_FAILURE_THRESHOLD` | `5` | Breaker opens after N failures. |
| `AGENTIC_CIRCUIT_SUCCESS_THRESHOLD` | `2` | Closes after N successes from half-open. |
| `AGENTIC_CIRCUIT_TIMEOUT_SECONDS` | `30` | Half-open delay. |
| `AGENTIC_MAX_MUTATIONS_PER_RESPONSE` | `64` | Cap. |
| `AGENTIC_MAX_IDS_PER_PAYLOAD` | `256` | Cap. |
| `AGENTIC_DISABLE_SHADE` | `true` | OQ3 default — `BID_SHADE` rejected until trusted. |
| `AGENTIC_ALLOW_INSECURE` | `false` | Permits plain `grpc://`; rejected in production. |
| `AGENTIC_GRPC_PORT` | `0` | Reserved (Phase 2). |
| `AGENTIC_MCP_ENABLED` | `false` | Reserved (Phase 2). |

## Layout

```
agentic/
  README.md                  this file
  doc.go                     Go package overview

  client.go                  ExtensionPointClient: gRPC fan-out
  applier.go                 per-intent mutation handlers
  registry.go                AgentRegistry: load + validate agents.json
  originator.go              OriginatorStamper
  envelope.go                ext.aamp helpers (read/write/cap)
  consent.go                 DeriveAgentConsent
  decisions.go               ApplyDecision / AgentCallStat / DispatchResult
  errors.go                  sentinel errors
  lifecycle.go               Lifecycle enum
  *_test.go                  68 unit + integration tests

  proto/iabtechlab/          vendored protos (read-only)
    bidstream/mutation/v1/   ARTF service + messages
    openrtb/v26/             transitive OpenRTB dep
  proto/tne/v1/              reserved for our extensions

  gen/iabtechlab/            generated Go code (committed)

  endpoints/                 HTTP handlers sub-package
    agents_json.go           /.well-known/agents.json
    agents_admin.go          /admin/agents

  assets/                    static files
    agents.json              empty allow-list (Phase 1 default)
    agents.schema.json       JSON Schema

  prebid-adapter/            JS scaffold (stretch; not published)
  docs/                      onboarding docs
```

## Onboarding a new agent

See `docs/agent-vendor-onboarding.md` for the step-by-step. Summary:

1. Add an entry to `agentic/assets/agents.json` matching the schema in
   `agentic/assets/agents.schema.json`.
2. Issue an API key, set `AGENTIC_API_KEY_<AGENT_ID>=<secret>` in the
   deploy env (where `<AGENT_ID>` is the env-var-safe form of the agent's
   `id` field).
3. Smoke against the in-process fake-agent suite: `go test ./agentic/... -run TestIntegration`.
4. Stage flip: deploy with `AGENTIC_ENABLED=true`. Verify
   `/.well-known/agents.json` returns 200 and the agent appears.
5. Watch `agentic.call` and `agentic.mutation` log lines for ≥ 24 h before
   prod flip.

## Regenerating the protos

The generated Go code under `agentic/gen/` is committed to the repo (no
codegen at build time; CI does not run `make generate-protos`). When the
upstream IAB protos change:

```bash
make install-proto-tools   # one-time per machine
make generate-protos
go test ./agentic/...
```

The applier whitelist is the canary — if an upstream proto change renames
or renumbers a field used in `applier.go`, the test suite breaks
deterministically. Reconcile before merging.

## Related docs

- PRD: `docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md`
- Plan: `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md`
- Vendoring patches: `agentic/proto/iabtechlab/README.md`
- Prebid adapter: `agentic/prebid-adapter/README.md`
