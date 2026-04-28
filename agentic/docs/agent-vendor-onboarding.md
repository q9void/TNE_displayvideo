# Onboarding a new agent vendor

This walks through the operational steps to add a new IAB ARTF extension
point agent to a running TNE Catalyst deployment. Prereqs: the vendor has
a gRPC `RTBExtensionPoint.GetMutations` endpoint reachable from our SSP.

## 1. Negotiate the contract

Confirm with the vendor:

- **Endpoint URL** (e.g. `grpcs://seg.example.com:50051`). Production must
  be `grpcs://`. `grpc://` is dev/test only.
- **Auth scheme.** Phase 1 supports `api_key_header` (we send `x-aamp-key:
  <secret>` in gRPC metadata) and `none`. mTLS lands in Phase 2.
- **Lifecycles** the agent serves: `PUBLISHER_BID_REQUEST` and/or
  `DSP_BID_RESPONSE`.
- **Intents** the agent emits: subset of `ACTIVATE_SEGMENTS`,
  `ACTIVATE_DEALS`, `SUPPRESS_DEALS`, `ADJUST_DEAL_FLOOR`,
  `ADJUST_DEAL_MARGIN`, `BID_SHADE`, `ADD_METRICS`, `ADD_CIDS`. Anything
  else is rejected by the applier as `unsupported_intent`.
- **tmax budget** (per-call deadline the vendor commits to). The applier
  caps this at the global `AGENTIC_TMAX_MS` regardless.
- **Priority.** 0–1000. Higher priority wins on REPLACE conflicts. Default
  100. Don't go above 500 unless we've explicitly committed to last-writer
  precedence.
- **Data processing disclosure.** Categories of data they touch and
  retention. Goes into `agents.json` for publisher transparency.

## 2. Add to `agents.json`

Edit `agentic/assets/agents.json` and add an entry under `agents`:

```json
{
  "id": "seg.example.com",
  "name": "Example Segmentation Agent",
  "role": "segmentation",
  "vendor": "Example Inc",
  "registry_ref": "iab-tools-portal:reg-12345",
  "priority": 100,
  "tmax_ms": 25,
  "essential": false,
  "endpoints": [
    {
      "transport": "grpcs",
      "url": "grpcs://seg.example.com:50051",
      "auth": "api_key_header"
    }
  ],
  "lifecycles": ["PUBLISHER_BID_REQUEST"],
  "intents": ["ACTIVATE_SEGMENTS"],
  "data_processing": {
    "categories": ["interest_segments", "contextual"],
    "retention_days": 0
  }
}
```

The schema at `agentic/assets/agents.schema.json` is enforced at boot. If
any required field is missing or any enum value is wrong, the server
fails to start with a clear error message.

## 3. Inject the API key

The deploy env must contain a per-agent secret. The env var format is
`AGENTIC_API_KEY_<AGENT_ID>` where `<AGENT_ID>` is the env-var-safe form
of the agent's `id` field — replace `.` and `-` with `_`, leave alphanumerics
alone, do **not** uppercase (the lookup is case-sensitive against the env-var
suffix as written).

For agent id `seg.example.com`, set:

```
AGENTIC_API_KEY_seg_example_com=secret-key-from-vendor
```

Or set the global fallback (used for any agent that has no per-agent key):

```
AGENTIC_API_KEY=global-key
```

Per-agent keys take precedence.

## 4. Local smoke

```bash
AGENTIC_ENABLED=true \
  AGENTIC_AGENTS_PATH=agentic/assets/agents.json \
  AGENTIC_SCHEMA_PATH=agentic/assets/agents.schema.json \
  AGENTIC_API_KEY=test \
  go run ./cmd/server
```

Verify:
- Server boots with `IAB ARTF agentic integration enabled` in the log
- `curl http://localhost:8000/.well-known/agents.json` returns 200 and lists
  the new agent
- A test auction triggers an `agentic.call` log line with `agent_id` set to
  the new agent's id and `status: ok` (or `circuit_open` / `error` /
  `timeout` — debug accordingly)

## 5. Staging

Same env, in staging. Hit the staging /openrtb2/auction endpoint with a
synthetic bid request. Watch:

- `agentic_call_duration_seconds` Prometheus histogram — P95 should be
  under the agent's declared `tmax_ms`
- `agentic_mutation_total{decision="applied"}` should increment
- No `agentic.applier_panic` in logs

Run for ≥ 24 h before prod.

## 6. Production

Flip `AGENTIC_ENABLED=true` in production env. Phase 1 is reversible by
env-var flip — to roll back, set `AGENTIC_ENABLED=false` and restart the
server.

## 7. Watch the breaker

The first 30 minutes after onboarding a new agent are the highest-risk
window. Watch:

- `agentic_circuit_breaker_state{agent_id="<id>"}` — if it flips to 1 or 2,
  the vendor is failing → reach out, don't disable on your side first
- `agentic.call` log lines with `status: timeout` or `status: error`
- Auction P95 latency for any regression

## Decommissioning

Remove the entry from `agents.json` and restart the server. The breaker
state is in-memory only, so removal is clean.
