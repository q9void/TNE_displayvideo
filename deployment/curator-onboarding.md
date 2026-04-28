# Curator Onboarding Runbook

How to add a curator to TNE Catalyst, register their DSP seats, define a deal,
allow-list publishers, and verify signals reach the bidstream end-to-end.

The flow below mirrors the live E2E test
(`internal/exchange/e2e_curator_test.go` with `-tags=e2e`) and was used to
generate the audit rows shown at the bottom.

---

## Prerequisites

- Postgres reachable via `DB_*` env vars; migrations `009`, `013`, `020`,
  `021` applied. The analytics tables also need `add_request_events.sql`,
  `add_render_events.sql`, `add_identity_events.sql`, plus `ad_unit` /
  `sizes` columns from `create_analytics_schema.sql`.
- Server boots with `Curated-deals catalog wired to exchange` and
  `Curator admin endpoints registered: /admin/curators` in the log.
- Admin basic-auth credentials (`ADMIN_USER` / `ADMIN_PASSWORD`).

```bash
# Sanity check
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD http://localhost:8000/admin/curators
# → {"count": 0, "curators": []}
```

---

## 1. Register the curator

```bash
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  -X POST -H 'Content-Type: application/json' \
  -d '{
    "id": "curator-acme",
    "name": "Acme Curator",
    "schain_asi": "acme.example",
    "schain_sid": "acme-001"
  }' \
  http://localhost:8000/admin/curators
```

`schain_asi` and `schain_sid` are appended to `source.ext.schain.nodes` on
every outbound request that carries this curator's deals — DSPs use them to
attribute the signal source.

## 2. Bind the curator to the DSP seats they own

A curator can hold seats at multiple bidders. Each row maps `(curator,
bidder_code, seat_id)`. The exchange's strict-PMP fanout filter joins this
table against `imp.pmp.deals[].wseat` to decide who sees the deal.

```bash
# Example: Acme has a Rubicon seat
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  -X POST -H 'Content-Type: application/json' \
  -d '{"bidder_code": "rubicon", "seat_id": "acme-rub-seat"}' \
  http://localhost:8000/admin/curators/curator-acme/seats

# Direct DSP via the bundled DV360 adapter
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  -X POST -H 'Content-Type: application/json' \
  -d '{"bidder_code": "dv360", "seat_id": "acme-adx-12345"}' \
  http://localhost:8000/admin/curators/curator-acme/seats
```

## 3. Register the deal catalog

```bash
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  -X POST -H 'Content-Type: application/json' \
  -d '{
    "deal_id":     "ACME-DEAL-1",
    "bidfloor":    2.50,
    "bidfloorcur": "USD",
    "at":          2,
    "wseat":       ["acme-rub-seat"],
    "segtax_allowed": [4],
    "active":      true
  }' \
  http://localhost:8000/admin/curators/curator-acme/deals
```

When an inbound request carries `imp.pmp.deals[].id == "ACME-DEAL-1"` with
the rest blank, the catalog overlay hydrates `wseat`, `bidfloor`, `at`, etc.
Inbound values WIN on conflict — publishers stay authoritative for what
they accept.

## 4. Allow-list publishers

```bash
# Resolve the publisher_id (publishers_new.id integer)
PUB_ID=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -tAc \
  "SELECT p.id FROM publishers_new p JOIN accounts a ON a.id=p.account_id
   WHERE a.account_id='12345' AND p.domain='e2e-publisher.example'")

curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  -X POST -H 'Content-Type: application/json' \
  -d "{\"publisher_id\": $PUB_ID}" \
  http://localhost:8000/admin/curators/curator-acme/publishers
```

Empty allow-list (no rows) means **allow all** — useful while onboarding,
strict once any row exists for that curator.

## 5. (Optional) Bind a curator to an agentic ARTF agent

If the curator runs an ARTF v1 agent that injects deals via gRPC, bind
agent → curator at boot so the applier validates every `ACTIVATE_DEALS`
payload against this curator's catalog. Unknown deal_ids are dropped from
the mutation; payloads with no valid deals are rejected with reason
`deal_not_in_curator_catalog`.

```bash
export AGENTIC_CURATOR_BINDINGS="acme-agent:curator-acme,beta-agent:curator-beta"
```

## 6. Verify signals flow end-to-end

Send an OpenRTB auction request that names the deal:

```bash
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{
    "id":"verify-1", "tmax":200,
    "site":{"id":"12345","domain":"e2e-publisher.example",
            "page":"https://e2e-publisher.example/x",
            "publisher":{"id":"12345"}},
    "device":{"ua":"Mozilla/5.0","ip":"203.0.113.42","geo":{"country":"US"}},
    "imp":[{
      "id":"imp-1","tagid":"e2e-publisher.example/billboard",
      "banner":{"w":728,"h":90,"format":[{"w":728,"h":90}]},
      "bidfloor":0.50,"bidfloorcur":"USD",
      "pmp":{"private_auction":1,"deals":[{"id":"ACME-DEAL-1"}]}
    }],
    "user":{
      "eids":[{"source":"audigent.com","uids":[{"id":"u-1"}]}],
      "data":[{"id":"acme","ext":{"segtax":4},"segment":[{"id":"9001"}]}]
    }
  }' http://localhost:8000/openrtb2/auction
```

### Inspect the audit (live row from the verification run that produced this doc)

```sql
SELECT auction_id, bidder_code, deal_id, curator_id, seat,
       eids_sent, segments_sent, schain_nodes_sent
  FROM signal_receipts WHERE auction_id='verify-1';
```

```
 auction_id | bidder_code |   deal_id   |  curator_id  |     seat      |   eids_sent    | segments_sent |  schain_nodes_sent
 live-e2e-2 | rubicon     | ACME-DEAL-1 | curator-acme | acme-rub-seat | {audigent.com} | {iab4:9001}   | [{ASI:acme.example,SID:acme-001},{ASI:thenexusengine.com,SID:NXS001}]
```

```sql
SELECT auction_id, bidder_code, deal_id, curator_id, seat, signal_sources
  FROM bidder_events WHERE deal_id IS NOT NULL;
```

```
 live-e2e-2 | rubicon | ACME-DEAL-1 | curator-acme | acme-rub-seat |
   {eid:audigent.com,seg:iab4:9001,schain:acme.example,schain:thenexusengine.com}
```

### Aggregate audit endpoint

```bash
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD \
  'http://localhost:8000/admin/curators/curator-acme/signal-receipts'
# → {"count":1,"rows":[{"deal_id":"ACME-DEAL-1","bidder_code":"rubicon",
#     "seat":"acme-rub-seat","receipt_count":N,
#     "eid_source_count":{"audigent.com":N},
#     "segment_count":{"iab4:9001":N}}]}
```

### Prometheus

The `/metrics` endpoint exposes per-curator counters:

```
pbs_curator_deals_hydrated_total{curator_id="curator-acme"}
pbs_curator_deals_dropped_total{curator_id="...",reason="publisher_not_allow_listed"}
pbs_curator_signal_receipts_total{curator_id="...",bidder="..."}
pbs_curator_signal_acks_total{bidder="..."}
pbs_curator_strict_pmp_filter_total
```

---

## How signals reach DSPs (the two patterns)

**Pattern (i) — upstream-injecting curator.** Curator is an SSP partner
upstream of TNE; their request arrives with `pmp.deals[].wseat` already
populated. Catalog hydration validates, schain augmentation appends the
curator node, signal-passthrough preserves their EIDs/segments for any
bidder whose seat appears in `wseat`. Existing adapters fan out as usual.

**Pattern (ii) — external curator with DSP-bidder map.** Curator registers
seats via `/admin/curators/{id}/seats` for `dv360` / `ttd` (or any SSP
adapter whose seat they hold). Inbound deal carries only `id`; catalog
hydrates `wseat` from `curator_seats`; the routing fanout filter
restricts the auction to those bidders only.

The DV360 and TTD adapters (`internal/adapters/dv360`, `internal/adapters/ttd`)
ship as OpenRTB-passthrough skeletons — production rollout requires
DSP-specific seat onboarding, encryption keys, and per-region URL routing
(see package docstrings).

## Removing a curator

```bash
# Soft-delete (sets status='archived'; preserves history for analytics)
curl -s -u $ADMIN_USER:$ADMIN_PASSWORD -X DELETE \
  http://localhost:8000/admin/curators/curator-acme

# Cascade is via the FK constraints only on hard delete; soft-deleted
# curators keep their deals/seats/allow-list rows for audit retention.
```
