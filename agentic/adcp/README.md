# `agentic/adcp/` — Ad Context Protocol integration

Peer to `agentic/` (IAB ARTF). Speaks the [Ad Context Protocol](https://adcontextprotocol.org)
over MCP/HTTPS instead of ARTF's gRPC. Both packages plug into the same
auction lifecycle but model different things:

|                | `agentic/` (ARTF)                    | `agentic/adcp/` (AdCP)                              |
| -------------- | ------------------------------------ | --------------------------------------------------- |
| Protocol       | gRPC + protobuf (RTBExtensionPoint)  | MCP / JSON-RPC over HTTPS                           |
| Shape          | Mutation-oriented                    | Capability-oriented                                 |
| Examples       | `ACTIVATE_SEGMENTS`, `BID_SHADE`     | `get_signals`, `get_products`, `create_media_buy`   |
| Registry doc   | `agentic/assets/agents.json`         | `agentic/adcp/assets/adcp_agents.json`              |
| Feature flag   | `AGENTIC_ENABLED`                    | `ADCP_ENABLED`                                      |

## Status

**Phase 1 — scaffold, default-off.** Ships:

- Registry loader + JSON Schema for `adcp_agents.json`
- Lifecycle + capability enums (PRE_AUCTION_SIGNALS / _PRODUCTS, POST_AUCTION_REPORTING)
- `Client` skeleton with circuit breaker + tmax + per-agent budget,
  parallel fanout, panic recovery — same shape as `agentic.Client`
- `Envelope` helper for `ext.adcp` on outbound bid requests
- `ADCP_*` env vars wired through `cmd/server/config.go`
- Empty allow-list so enabling the flag is safe (no agents called yet)

**Phase 2 — out of scope this branch.** Real MCP/JSON-RPC framing in
`Client.invoke`; signals→ORTB user.data applier; sales-agent flow
(`get_products` → deals); per-publisher overrides; admin endpoints.

## Quick start

```bash
# Default: AdCP is OFF and the code path is unreachable.
go run ./cmd/server

# Enable with the empty allow-list (safe — no agents called yet, but the
# scaffold activates and ext.adcp envelopes are written).
ADCP_ENABLED=true go run ./cmd/server

# Add a real agent: edit agentic/adcp/assets/adcp_agents.json, then provide an API key.
ADCP_ENABLED=true ADCP_API_KEY=test-key go run ./cmd/server
```

## Environment variables

| Var | Default | Meaning |
|---|---|---|
| `ADCP_ENABLED` | `false` | Master kill switch. Off ⇒ code path unreachable. |
| `ADCP_AGENTS_PATH` | `agentic/adcp/assets/adcp_agents.json` | Registry document path. |
| `ADCP_SCHEMA_PATH` | `agentic/adcp/assets/adcp_agents.schema.json` | JSON Schema for validation. |
| `ADCP_TMAX_MS` | `30` | Global per-call deadline ceiling. |
| `ADCP_SAFETY_MS` | `50` | Subtracted from auction-context deadline. |
| `ADCP_SELLER_ID` | `9131` | Echoed in `ext.adcp.seller_id`. |
| `ADCP_API_KEY` | `""` | Global Authorization header value. |
| `ADCP_API_KEY_<AGENT_ID>` | `""` | Per-agent override. |
| `ADCP_CIRCUIT_FAILURE_THRESHOLD` | `5` | Breaker opens after N failures. |
| `ADCP_CIRCUIT_SUCCESS_THRESHOLD` | `2` | Closes after N successes from half-open. |
| `ADCP_CIRCUIT_TIMEOUT_SECONDS` | `30` | Half-open delay. |
| `ADCP_MAX_SIGNALS_PER_RESPONSE` | `256` | Cap. |
| `ADCP_ALLOW_INSECURE` | `false` | Permits plain `http://`; rejected in production. |

## Layout

```
agentic/adcp/
  README.md           this file
  doc.go              package overview
  client.go           HTTP/MCP fanout client (Phase 1: invoke=stub)
  registry.go         load + validate adcp_agents.json
  lifecycle.go        Lifecycle + Capability enums
  envelope.go         ext.adcp helpers
  decisions.go        CallStat / Signal / DispatchResult
  errors.go           sentinel errors
  *_test.go           unit tests
  assets/
    adcp_agents.json         empty allow-list (Phase 1 default)
    adcp_agents.schema.json  JSON Schema
```

## Onboarding a new agent

1. Add an entry to `agentic/adcp/assets/adcp_agents.json` matching the
   schema in `agentic/adcp/assets/adcp_agents.schema.json`.
2. Issue an API key, set `ADCP_API_KEY_<AGENT_ID>=<secret>` in the deploy
   env (where `<AGENT_ID>` is the env-var-safe form of the agent's `id`).
3. Stage flip: deploy with `ADCP_ENABLED=true`. Watch `adcp.call` log
   lines; in Phase 1 every status will be `not_implemented` until the
   real RPC layer lands.
4. When Phase 2 lands, the same registry entry begins exercising the
   real MCP transport — no further config changes.

## Why a peer package, not a transport plugin?

AdCP and ARTF look superficially similar (both call out to "agents") but
the data shapes are different in ways that would leak through any shared
abstraction:

- ARTF returns *mutations* the server applies. AdCP returns *resources*
  (signals, products, media-buy IDs) the server reads.
- ARTF is single-shot per lifecycle. AdCP `create_media_buy` is stateful
  across requests.
- ARTF is protobuf with a closed enum of intents. AdCP is JSON with an
  evolving capability vocabulary.

A shared `agentic.Dispatcher` interface would either erase the type
information that makes each protocol useful, or grow `interface{}`
escape hatches. Splitting at the package boundary keeps each
integration's grain visible.
