# Publisher Onboarding Guide

This guide covers everything needed to integrate a new publisher into the TNE exchange: account provisioning, authentication, consent signal passing, and first-party ID resolution.

**Reference migration (full working example):** `deployment/migrations/012_seed_bizbudding_totalprosports.sql`

---

## 1. Prerequisites — What TNE Ops Must Do First

Before a publisher can send traffic, the following provisioning steps must be completed by TNE operations staff.

### Database Records

**1. Create an account row**
```sql
INSERT INTO accounts (account_id, name) VALUES ('acct-001', 'Publisher Name');
```

**2. Create a `publishers_new` row per domain, linked to the account**
```sql
INSERT INTO publishers_new (publisher_id, account_id, domain, name)
VALUES ('pub-123', 'acct-001', 'example.com', 'Example Site');
```

**3. Create `ad_slots` rows with `slot_pattern`**

The `slot_pattern` is a path glob matched against `site.page` in the bid request:
```sql
INSERT INTO ad_slots (slot_id, publisher_id, slot_pattern, name)
VALUES
  ('slot-001', 'pub-123', 'example.com/billboard',    'Billboard'),
  ('slot-002', 'pub-123', 'example.com/leaderboard',  'Leaderboard');
```
Wildcard patterns (e.g. `example.com/billboard-*`) are supported.

**4. Create `slot_bidder_configs` rows** (one per slot × bidder × device_type)
```sql
INSERT INTO slot_bidder_configs (slot_id, bidder_code, device_type, bidder_params)
VALUES
  ('slot-001', 'pubmatic',    'desktop', '{"publisherId":"12345","adSlot":"billboard@728x90"}'),
  ('slot-001', 'appnexus',    'desktop', '{"placementId":98765}'),
  ('slot-001', 'pubmatic',    'mobile',  '{"publisherId":"12345","adSlot":"billboard-mobile@320x50"}');
```
See `config/bizbudding-all-bidders-mapping.json` for the full set of bidder param schemas.

### Redis Registration

Register the publisher in the `tne_catalyst:publishers` hash. The value is a pipe-separated list of allowed domains (wildcards supported):
```
HSET tne_catalyst:publishers pub-123 "example.com|*.example.com"
```

### Share Credentials with Publisher

Provide the publisher contact with:
- `publisher_id` — used in every bid request as `site.publisher.id`
- `ADMIN_API_KEY` — needed only if the publisher manages their own registration via the admin API

---

## 2. Client-Side Integration (Publisher Action Required)

After provisioning, send the publisher the integration guide:

**[docs/integrations/PUBLISHER_GPT_SETUP.md](integrations/PUBLISHER_GPT_SETUP.md)**

Summary of what publishers must do:
1. Load `catalyst-sdk.js` and call `catalyst.requestBids()` — no GPT code changes needed
2. In GAM: create custom key-values (`hb_pb_catalyst`, `hb_adid_catalyst`, `hb_size_catalyst`, `hb_bidder_catalyst`)
3. In GAM: create price-priority line items targeting `hb_pb_catalyst` at each CPM tier ($0.50 increments)
4. Add the Catalyst render creative (`/ad/render?adid=%%PATTERN:hb_adid_catalyst%%`) to all line items

Without the GAM line items, bids will flow and targeting will be set but no impression will win.

---

## 3. Publisher Authentication

**Middleware:** `internal/middleware/publisher_auth.go`

Every auction request must include `site.publisher.id`. Validation chain:

1. Check Redis hash `tne_catalyst:publishers` (fastest path)
2. Fall back to PostgreSQL `publishers_new` table
3. Cache result in-memory for 30 seconds
4. Final fallback: accept if `PUBLISHER_ALLOW_UNREGISTERED=true`

**Optional domain validation** (`PUBLISHER_VALIDATE_DOMAIN=true`): checks `site.domain` against the publisher's registered `allowed_domains`.

**Rate limit:** 100 RPS per publisher (token bucket). Excess requests return `429 Too Many Requests`.

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PUBLISHER_AUTH_ENABLED` | `true` | Set to `false` only for local dev/testing |
| `PUBLISHER_ALLOW_UNREGISTERED` | `false` | Allow requests from unknown publisher IDs |
| `PUBLISHER_VALIDATE_DOMAIN` | `false` | Enforce `site.domain` matches registered domains |

### Admin CRUD API

Requires `Authorization: Bearer <ADMIN_API_KEY>` on all requests.

```
POST   /admin/publishers
       Body: {"id":"pub-123","allowed_domains":"example.com|*.example.com"}

PUT    /admin/publishers/:id
       Body: {"allowed_domains":"example.com|*.example.com|cdn.example.com"}

GET    /admin/publishers
GET    /admin/publishers/:id
```

---

## 3. Consent Signals — What to Pass

**Middleware:** `internal/middleware/privacy.go`
**Hook:** `internal/hooks/privacy_consent.go`

The exchange evaluates `regs` fields on every request. Include the appropriate signal based on the user's jurisdiction.

### GDPR (EU / EEA / UK users)

```json
{
  "regs": { "gdpr": 1 },
  "user": { "consent": "<TCF v2 base64-encoded string>" }
}
```

Required purposes in the consent string: **1** (storage/access), **2** (basic ads), **7** (measurement).

- Missing or refused consent → `400 Bad Request` with body `"Privacy compliance violation"` (when `PBS_ENFORCE_GDPR=true`)
- IP anonymization applied automatically: IPv4 last octet masked, IPv6 last 80 bits zeroed

### CCPA / US State Privacy

```json
{
  "regs": { "us_privacy": "1YNN" }
}
```

Position 2 = `Y` signals opt-out. When `PBS_ENFORCE_CCPA=true`:
- User IDs are stripped from all downstream bid requests
- `user.ext.eids` is cleared before forwarding to bidders

### COPPA (Child-Directed Content)

```json
{
  "regs": { "coppa": 1 }
}
```

Blocks the request entirely — no PII, no tracking, no bidding.

### No Regulation Applicable

Omit all `regs` fields. The request proceeds normally.

### Enforcement Summary

| Signal | Missing/Refused | Opt-Out | Enforced by default |
|---|---|---|---|
| GDPR | 400 + privacy error | — | Yes (`PBS_ENFORCE_GDPR=true`) |
| CCPA | Proceeds normally | UIDs stripped | Yes (`PBS_ENFORCE_CCPA=true`) |
| COPPA | — | Request blocked | Always |

### Note on `site.publisher.id`

The `PrivacyConsentHook` clears `site.publisher.id` before the bid request reaches any SSP adapter. The internal publisher ID must never leak to demand partners.

### TCF Disclosure Endpoint

Required for CMP integration (TCF v2 compliance deadline: 28 Feb 2026):

```
GET /.well-known/tcf-disclosure.json
```

Declares all 26+ bidder cookie identifiers for inclusion in the publisher's CMP consent layer.

---

## 4. First-Party IDs — What to Pass & How Sync Works

### Step 1 — Include FPID and SDK eids in Every Bid Request

```json
{
  "user": {
    "id": "<publisher_first_party_uuid>",
    "ext": {
      "eids": [
        { "source": "liveramp.com",  "uids": [{ "id": "<ramp_id>"    }] },
        { "source": "id5-sync.com",  "uids": [{ "id": "<id5_id>"     }] },
        { "source": "uidapi.com",    "uids": [{ "id": "<uid2_token>" }] }
      ]
    }
  }
}
```

`user.id` is the publisher's own first-party UUID for this user (FPID). SDK-sourced `eids` are always preserved and forwarded to all bidders (fix: commit `b7800175`).

### Step 2 — Initiate Cookie Sync (Once Per New User)

```
POST /cookie_sync
Content-Type: application/json

{
  "fpid": "<uuid>",
  "gdpr": 1,
  "gdpr_consent": "<tcf_string>",
  "bidders": ["pubmatic", "openx", "appnexus"]
}
```

Returns a list of redirect/iframe sync URLs per bidder. The publisher page must load these pixels (e.g. in a hidden `<img>` or `<iframe>`).

### Step 3 — Bidder Callback (Automatic)

After the user's browser loads a sync pixel, the bidder redirects back to:

```
GET /setuid?bidder=<code>&uid=<bidder_uid>&gdpr=1&gdpr_consent=<tcf>
```

The exchange stores the mapping in:
- `user_syncs` table: FPID → bidder UID
- `id_graph_mappings` table: cross-ID graph entry

Source: `internal/endpoints/setuid.go`, `internal/storage/usersync.go`, `internal/storage/idgraph.go`

### Step 4 — On Subsequent Requests

The exchange automatically merges DB-stored bidder UIDs into `user.ext.eids` before forwarding the request to each SSP. Publishers do not need to do anything extra after the initial sync.

### Supported Sync Partners

| Partner | Bidder Code | Notes |
|---|---|---|
| AppNexus / Xandr | `appnexus` | Active |
| PubMatic | `pubmatic` | Active |
| OpenX | `openx` | Active |
| TripleLift | `triplelift` | Active |
| Index Exchange | `ix` | Active |
| Criteo | `criteo` | Active |
| Sharethrough | `sharethrough` | Active |
| Sovrn | `sovrn` | Active |
| Kargo | `kargo` | Active |
| 33Across | `33across` | Active |
| GumGum | `gumgum` | Active |
| Media.net | `medianet` | Active |
| Rubicon / Magnite | `rubicon` | Not yet active — needs custom `p=` key; email `header-bidding@rubiconproject.com` |

### IDR Integration (TNE-Internal)

The Intelligent Demand Router selects optimal bidders per request using ML scoring. Publishers do not interact with it directly.

Required env vars for IDR:
```
IDR_URL=<internal service URL>
IDR_API_KEY=<api key>
IDR_ENABLED=true
```

---

## 5. Required Environment Variables Summary

| Variable | Default | Publisher Impact |
|---|---|---|
| `PBS_HOST_URL` | `https://ads.thenexusengine.com` | Base URL for tracking and sync callback URLs |
| `PUBLISHER_AUTH_ENABLED` | `true` | Publisher must be registered before going live |
| `PBS_ENFORCE_GDPR` | `true` | GDPR consent string required for EU/EEA/UK users |
| `PBS_ENFORCE_CCPA` | `true` | CCPA opt-out signal respected; UIDs stripped |
| `ADMIN_API_KEY` | (set by ops) | Required to call `/admin/publishers` endpoints |
| `IDR_ENABLED` | `true` | Enables ML-based bidder selection optimization |
| `IDR_URL` | — | IDR service endpoint (required when `IDR_ENABLED=true`) |
| `IDR_API_KEY` | — | IDR service authentication key |

---

## 6. Verification Checklist

Run these checks for every new publisher before marking them live.

- [ ] `GET /admin/publishers/<id>` returns 200 with correct `allowed_domains`
- [ ] Test auction: `POST /v1/bid` with `site.publisher.id` set → returns bids (not 403)
- [ ] Auth rejection: `POST /v1/bid` without `site.publisher.id` → returns 401/403
- [ ] GDPR enforcement: omit `user.consent` with `regs.gdpr=1` → returns 400 privacy error
- [ ] CCPA opt-out: `regs.us_privacy="1YNN"` → request succeeds but UIDs stripped from response
- [ ] Cookie sync: `POST /cookie_sync` with FPID and valid bidder list → returns sync URLs
- [ ] UID callback: load a sync pixel URL and confirm `user_syncs` row created
- [ ] TCF disclosure: `GET /.well-known/tcf-disclosure.json` reachable from publisher's CMP
- [ ] Rate limit: confirm 100 RPS limit applies (excess returns 429)

---

## Critical Files Reference

| File | Purpose |
|---|---|
| `internal/middleware/publisher_auth.go` | Auth validation, domain matching, rate limiting |
| `internal/middleware/privacy.go` | Consent parsing, geo detection, IP anonymization |
| `internal/hooks/privacy_consent.go` | ID stripping, regs field mapping, publisher ID clearing |
| `internal/endpoints/cookie_sync.go` | Sync flow initiation, FPID handling |
| `internal/endpoints/setuid.go` | UID callback storage |
| `internal/endpoints/publisher_admin.go` | Publisher CRUD API |
| `internal/storage/publishers.go` | Publisher data model and store |
| `internal/storage/usersync.go` | Bidder UID store |
| `internal/storage/idgraph.go` | Cross-ID graph mappings |
| `deployment/migrations/009_create_new_schema.sql` | Canonical DB schema |
| `deployment/migrations/012_seed_bizbudding_totalprosports.sql` | Full provisioning example |
| `config/bizbudding-all-bidders-mapping.json` | Bidder params reference for all SSPs |
