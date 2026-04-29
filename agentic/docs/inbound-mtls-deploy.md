# Deploying the AAMP 2.0 inbound surface with mTLS

This runbook covers turning on the production inbound path: mutual TLS
termination, per-buyer SPKI pinning, and IAB Tools Portal Registry
cross-check. Prereqs: Phase 2A is shipped (PR #34), Phase 2A.1 code is
deployed, and you have a CA we trust for buyer certs.

## Trust model in one paragraph

Every inbound caller MUST present a client cert that chains to the CA
bundle on disk. Once the handshake succeeds, the leaf cert's Subject
Public Key Info (SPKI) fingerprint is matched against a per-buyer table
on disk; mismatch ⇒ reject. Finally, the cert's CommonName (the
`agent_id`) is checked against the IAB Tools Portal Registry's
verified-seller-eligible list; not-listed ⇒ reject (with a configurable
cold-start grace window). All three checks must pass before the call
reaches a handler.

## Environment variables

Set these on the Catalyst process. All paths are read at boot; see
`agentic/inbound/tls.go` and `agentic/inbound/registry_client.go` for the
loaders.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `AGENTIC_INBOUND_ENABLED` | yes | `false` | Master switch for the inbound surface. |
| `AGENTIC_INBOUND_GRPC_PORT` | no | `50051` | Listener port. AAMP convention. |
| `AGENTIC_MTLS_CA_PATH` | yes (mTLS mode) | — | PEM bundle of CAs we trust to sign buyer certs. |
| `AGENTIC_MTLS_CERT_PATH` | yes (mTLS mode) | — | Our server cert (PEM). Presented to callers. |
| `AGENTIC_MTLS_KEY_PATH` | yes (mTLS mode) | — | Our server private key (PEM). |
| `AGENTIC_INBOUND_PINS_PATH` | yes (mTLS mode) | — | JSON file of per-buyer SPKI fingerprints. Schema below. |
| `AGENTIC_REGISTRY_URL` | no | `""` | IAB Tools Portal verified-seller endpoint. Empty ⇒ skip cross-check. |
| `AGENTIC_REGISTRY_REFRESH_SECONDS` | no | `900` | Refresh cadence for the registry snapshot. Min 30s. |
| `AGENTIC_REGISTRY_OPEN_WINDOW` | no | `true` | Accept callers before the first refresh succeeds. Set `false` for stricter posture once the registry is dependable. |
| `AGENTIC_INBOUND_ALLOW_DEV_NO_MTLS` | no | `false` | Dev/staging escape hatch — skips TLS termination and falls back to header-based auth. **Rejected when ENVIRONMENT=production.** |
| `AGENTIC_INBOUND_QPS_PER_AGENT` | no | `1000` | Per-buyer soft cap. |
| `AGENTIC_INBOUND_QPS_PER_PUBLISHER` | no | `200` | Per-publisher mutation soft cap. |

## Pins file format

The pins file lists every buyer that may dial us, paired with the
SHA-256 of their cert's SubjectPublicKeyInfo. Two slots per agent
support the 30-day rotation overlap (R5.2.3).

```json
{
  "pins": [
    {
      "agent_id": "dsp.example.com",
      "current": "5f2b8a...e1",
      "successor": ""
    },
    {
      "agent_id": "agency.example.com",
      "current": "9a1c0d...4b",
      "successor": "f3a872...77"
    }
  ]
}
```

- `agent_id` MUST match the cert's CommonName.
- `current` is the SPKI fingerprint of the cert in active use.
- `successor` is the SPKI fingerprint of the next cert. During a
  rotation window, both fingerprints are accepted; once the buyer cuts
  over, the successor moves into `current` and the slot empties.

### Computing a fingerprint

Given a PEM cert from the buyer:

```
openssl x509 -in buyer.pem -pubkey -noout \
  | openssl pkey -pubin -outform DER \
  | openssl dgst -sha256 \
  | awk '{print $2}'
```

The output is the lowercase hex SHA-256 we expect in the pins file.

## Bringing the surface up — first deploy

1. **Generate or obtain the server keypair.** This is the cert Catalyst
   presents to callers. CN should be the public hostname (e.g.
   `agentic.thenexusengine.com`).

2. **Stage CA bundle, cert, key, and pins file** onto the host. We
   recommend `/etc/tne/agentic/`:
   ```
   /etc/tne/agentic/ca.pem        # mode 0644
   /etc/tne/agentic/server.pem    # mode 0644
   /etc/tne/agentic/server.key    # mode 0600, owned by the catalyst user
   /etc/tne/agentic/pins.json     # mode 0644
   ```

3. **Set the env vars** in the systemd unit / k8s manifest:
   ```
   AGENTIC_ENABLED=true
   AGENTIC_INBOUND_ENABLED=true
   AGENTIC_INBOUND_GRPC_PORT=50051
   AGENTIC_MTLS_CA_PATH=/etc/tne/agentic/ca.pem
   AGENTIC_MTLS_CERT_PATH=/etc/tne/agentic/server.pem
   AGENTIC_MTLS_KEY_PATH=/etc/tne/agentic/server.key
   AGENTIC_INBOUND_PINS_PATH=/etc/tne/agentic/pins.json
   AGENTIC_REGISTRY_URL=https://tools.iabtechlab.com/registry/v1/verified-seller-eligible
   AGENTIC_REGISTRY_REFRESH_SECONDS=900
   AGENTIC_REGISTRY_OPEN_WINDOW=true
   ```

4. **Open port 50051** at the load balancer / firewall. mTLS is
   terminated **inside** Catalyst, not at nginx — the gRPC server
   accepts pass-through TLS so it can read the leaf cert directly.

5. **Roll the binary.** On boot you should see:
   ```
   AAMP 2.0 inbound surface started grpc_port=50051 dev_no_mtls=false registry_url=...
   inbound: initial registry refresh succeeded
   ```

6. **Smoke-test from a test agent.** Use `grpcurl` with client certs:
   ```
   grpcurl -cacert ca.pem -cert client.pem -key client.key \
     -d '{"id":"smoke","originator":{"type":"TYPE_DSP","id":"dsp.example.com"}}' \
     agentic.thenexusengine.com:50051 \
     iabtechlab.bidstream.mutation.v1.RTBExtensionPoint/GetMutations
   ```
   Expected: empty `mutations` slice with `metadata.api_version: "1.0"`.

## 30-day SPKI rotation procedure

The buyer-side rotation cadence is 30 days (R5.2.3). For a given buyer:

1. Buyer generates their next keypair and signs a CSR.
2. Buyer sends us:
   - The new cert (PEM).
   - The new SPKI fingerprint, computed as above.
3. We **add** the new fingerprint as `successor` in `pins.json`,
   leaving `current` untouched. Both certs are now accepted.
4. Reload Catalyst (SIGHUP — Phase 2B; until then, restart) so the
   new pins file takes effect.
5. Buyer cuts over to the new cert at their leisure.
6. Once we observe traffic exclusively on the new fingerprint (check
   `agentic_inbound_call_duration_seconds{caller_agent_id="<id>"}` against
   the cert log), we promote: `current` = old `successor`, `successor` = "".
7. Reload again. Old cert is now rejected.

The rotation window MUST stay ≤ 30 days; longer windows weaken the
attack surface and aren't compliant with PRD §5.2.3.

## Onboarding a new buyer

1. Buyer registers with the IAB Tools Portal and receives an `agent_id`.
   They appear in the verified-seller-eligible list within 24 hours.
2. Buyer sends us their client cert (PEM) + SPKI fingerprint.
3. Append to `pins.json`:
   ```json
   {"agent_id":"newbuyer.example.com","current":"<fingerprint>","successor":""}
   ```
4. Reload Catalyst.
5. Confirm the buyer can call us via `grpcurl`. The
   `agentic_inbound_auth_failed_total{stage="..."}` counter should not
   tick for this caller.

## Cold-start posture

`AGENTIC_REGISTRY_OPEN_WINDOW=true` (the default) accepts authenticated
callers during the window between boot and the first successful registry
refresh. This is deliberate: an IAB outage at our boot time should not
black-hole all traffic. Calls served in the open window are flagged
`registry_verified=false` in the AgentIdentity for downstream
audit/handlers.

For a stricter posture (no calls accepted until the registry is
confirmed reachable), set `AGENTIC_REGISTRY_OPEN_WINDOW=false`. Recommended
once the registry has demonstrated >99.5% availability over a quarter.

## Failure modes to monitor

| Symptom | Likely cause | Action |
|---|---|---|
| All calls fail with `Unauthenticated` | CA bundle ≠ buyer issuer, OR pins file out of date | Diff buyer cert chain vs CA bundle; recompute SPKI. |
| Calls fail post-handshake with `Unauthenticated` from one buyer | Cert rotated without pin update | Add new fingerprint as `successor`; reload. |
| `inbound: registry refresh failed` warns repeatedly | IAB endpoint outage or our egress block | Verify with curl; if outage is theirs, no action — last snapshot is still serving. |
| First refresh fails at boot, all calls allowed | Open-window posture engaged | Expected. Investigate registry connectivity; calls are still being authenticated by mTLS+SPKI, just not registry-cross-checked. |
| `inbound: SPKI fingerprint mismatch` for a known buyer | Cert mid-rotation, pins file lags | Update pins file. |

## Disabling the surface

Set `AGENTIC_INBOUND_ENABLED=false` and roll. The outbound (Phase 1) path
is independent and unaffected.
