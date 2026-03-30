# Bid Request Composer — Design Spec

**Date:** 2026-03-30
**Status:** Approved
**Scope:** Full OpenRTB 2.x bid request transformation config — visual editor + runtime engine

---

## Problem

Every SSP expects the OpenRTB bid request in a slightly different format. Today that logic is hardcoded in Go adapter files. There is no visibility into what each SSP receives, no way to audit or fix mappings without a code deployment, and no path to onboard a new SSP without writing a new adapter. First-party IDs, consent strings, EID routing, supply chain, and every other OpenRTB field are all implicit and invisible.

---

## Goal

A **Bid Request Composer**: a DB-backed configuration system + admin UI that makes every OpenRTB field mapping explicit, auditable, and editable — covering the full OpenRTB 2.x spec (BidRequest, Imp, Banner, Video, Audio, Native, Site, App, User, Device, Regs, Source, EIDs, SupplyChain, DOOH). Simple SSPs become fully config-driven. Complex SSPs (Rubicon) keep their Go adapter for nesting/auth but everything else is handled by the engine.

---

## Architecture

### Three concerns

| Concern | Description |
|---------|-------------|
| **Field presence** | Does this SSP receive `video`, `native`, `user.eids`, `regs.gpp`? |
| **Source mapping** | Where does each field value come from? |
| **Transform** | How is the value formatted for this SSP? |

### Source types (`source_type`)

| source_type | source_ref semantics | Description |
|-------------|---------------------|-------------|
| `standard` | Must be null | Pass-through from incoming OpenRTB — no override needed |
| `sdk_param` | SDK request param name (e.g. `pageUrl`) | From the Catalyst SDK ad tag request |
| `http_context` | HTTP header name (e.g. `User-Agent`) | From the live HTTP request |
| `account_param` | Column/key in `accounts` / `publishers_new` (e.g. `default_timeout_ms`) | From publisher DB record |
| `slot_param` | Key in `slot_bidder_configs.bidder_params` (e.g. `placementId`) | Falls back to `account_bidder_defaults` if not present at slot level |
| `eid` | EID source domain (e.g. `kargo.com`) | Walks `user.eids` (then falls back to `user.ext.eids`) to find `uids[0].id` for the matching source domain |
| `constant` | The literal value to use | Hardcoded string/int stored in `source_ref` |

### `__default__` rules
Rules with `bidder_code = '__default__'` define the baseline that applies to **all** bidders. When a bidder-specific rule exists for the same `field_path`, the bidder-specific rule wins. Merge order: bidder-specific > `__default__`. In the UI, `__default__` rows are shown as greyed-out baselines in every SSP's routing table; rows that override a default are visually highlighted.

### Transform types

| transform | Description |
|-----------|-------------|
| `none` | No transformation |
| `to_int` | Parse string → integer |
| `to_string` | Cast integer/float → string |
| `to_string_array` | Wrap single value in `["..."]` |
| `lowercase` | Lowercase string |
| `sha256` | SHA-256 hash |
| `base64_decode` | Decode base64 string |
| `url_encode` | URL-encode value |
| `url_decode` | URL-decode value |
| `json_stringify` | Serialize nested object to JSON string |
| `array_first` | Extract first element from array |
| `csv_to_array` | Split comma-separated string into JSON array |
| `wrap_ext_rp` | Rubicon-specific: nest inside `ext.rp` |

Complex conditional transforms (value depends on another field) require a Go adapter — document in adapter notes, do not store in rules table.

---

## Database Schema

### `bidder_field_rules`

```sql
CREATE TABLE IF NOT EXISTS bidder_field_rules (
    id           SERIAL PRIMARY KEY,
    bidder_id    INTEGER REFERENCES bidders_new(id) ON DELETE CASCADE,
    -- NULL when bidder_code = '__default__' (baseline rules have no specific bidder row)
    bidder_code  TEXT NOT NULL,
    -- '__default__' for baseline rules; otherwise matches bidders_new.code
    field_path   TEXT NOT NULL,
    -- OpenRTB dot-path e.g. "user.buyeruid", "imp.ext.kargo.placementId"
    source_type  TEXT NOT NULL CHECK (source_type IN (
                   'standard','sdk_param','http_context',
                   'account_param','slot_param','eid','constant')),
    source_ref   TEXT,
    -- semantics depend on source_type; see source types table above
    -- NULL is valid only for source_type = 'standard'
    CONSTRAINT source_ref_required CHECK (
        source_type = 'standard' OR source_ref IS NOT NULL
    ),
    transform    TEXT NOT NULL DEFAULT 'none',
    required     BOOLEAN NOT NULL DEFAULT false,
    enabled      BOOLEAN NOT NULL DEFAULT true,
    notes        TEXT,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(bidder_code, field_path)
);

CREATE INDEX IF NOT EXISTS idx_bfr_bidder_id   ON bidder_field_rules(bidder_id);
CREATE INDEX IF NOT EXISTS idx_bfr_bidder_code ON bidder_field_rules(bidder_code);
CREATE INDEX IF NOT EXISTS idx_bfr_enabled     ON bidder_field_rules(enabled) WHERE enabled = true;
```

### Seed data

```sql
-- ── Default baseline rules (apply to all SSPs) ─────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes) VALUES
  ('__default__', 'device.ua',           'http_context',  'User-Agent',        false, 'Set from request header'),
  ('__default__', 'device.ip',           'http_context',  'X-Forwarded-For',   false, 'First IP in chain'),
  ('__default__', 'device.language',     'http_context',  'Accept-Language',   false, NULL),
  ('__default__', 'site.page',           'sdk_param',     'pageUrl',           false, NULL),
  ('__default__', 'site.domain',         'sdk_param',     'domain',            false, NULL),
  ('__default__', 'regs.gdpr',           'standard',      NULL,                false, NULL),
  ('__default__', 'regs.us_privacy',     'standard',      NULL,                false, NULL),
  ('__default__', 'regs.gpp',            'standard',      NULL,                false, NULL),
  ('__default__', 'regs.gpp_sid',        'standard',      NULL,                false, NULL),
  ('__default__', 'regs.coppa',          'standard',      NULL,                false, NULL),
  ('__default__', 'source.schain',       'standard',      NULL,                false, NULL),
  ('__default__', 'user.consent',        'standard',      NULL,                false, 'TCF string pass-through'),
  ('__default__', 'user.eids',           'standard',      NULL,                false, 'Full EID array pass-through'),
  ('__default__', 'tmax',                'account_param', 'default_timeout_ms',false, NULL);

-- ── Kargo ──────────────────────────────────────────────────────────────────
-- Seeded adapters for Phase 1 (config-driven in Phase 2): kargo, sovrn, pubmatic, triplelift, appnexus
-- Complex adapters (keep Go logic): rubicon, criteo, ix, openx
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.kargo.placementId', 'slot_param', 'placementId', true,  NULL),
  ('user.buyeruid',             'eid',        'kargo.com',   false, 'Extract from eids[source=kargo.com].uids[0].id; fallback to user.ext.eids')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'kargo';

-- ── Rubicon / Magnite ──────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, transform, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.transform, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.rubicon.accountId',   'slot_param',    'accountId',          'to_int',    true,  NULL),
  ('imp.ext.rubicon.siteId',      'slot_param',    'siteId',             'to_int',    true,  NULL),
  ('imp.ext.rubicon.zoneId',      'slot_param',    'zoneId',             'to_int',    true,  NULL),
  ('site.publisher.id',           'slot_param',    'accountId',          'to_string', true,  'Rubicon uses accountId as publisher ID'),
  ('user.buyeruid',               'eid',           'rubiconproject.com', 'none',      false, NULL)
) AS r(field_path, source_type, source_ref, transform, required, notes)
WHERE b.code = 'rubicon';

-- ── Pubmatic ───────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.pubmatic.publisherId', 'slot_param', 'publisherId', true,  NULL),
  ('imp.ext.pubmatic.adSlot',      'slot_param', 'adSlot',      true,  NULL),
  ('user.buyeruid',                'eid',        'pubmatic.com',false, NULL)
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'pubmatic';

-- ── Sovrn ──────────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, transform, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.transform, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.sovrn.tagid', 'slot_param', 'tagid', 'to_string', true, NULL)
) AS r(field_path, source_type, source_ref, transform, required, notes)
WHERE b.code = 'sovrn';

-- ── TrippleLift ────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.triplelift.inventoryCode', 'slot_param', 'inventoryCode', true, NULL)
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'triplelift';

-- ── AppNexus / Xandr ───────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.appnexus.placementId', 'slot_param', 'placementId', true,  NULL),
  ('imp.ext.appnexus.member',      'slot_param', 'member',      false, 'Alternative to placementId'),
  ('imp.ext.appnexus.invCode',     'slot_param', 'invCode',     false, 'Used with member')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'appnexus';
```

---

## OpenRTB Field Coverage

All fields tracked in `bidder_field_rules`. Fields without SSP-specific overrides inherit `__default__` rules (or standard pass-through).

### BidRequest
`id`, `tmax`, `at`, `cur`, `test`, `allimps`, `wseat`, `bseat`, `acat`, `bcat`, `badv`, `bapp`, `cattax`

### Imp
`tagid`, `secure`, `instl`, `bidfloor`, `bidfloorcur`, `exp`, `rwdd`, `ssai`, `displaymanager`, `displaymanagerver`, `clickbrowser`

### Banner (within Imp)
`banner.w`, `banner.h`, `banner.format`, `banner.battr`, `banner.pos`, `banner.api`, `banner.mimes`, `banner.topframe`, `banner.expdir`, `banner.vcm`

### Video (within Imp)
`video.mimes`, `video.minduration`, `video.maxduration`, `video.protocols`, `video.w`, `video.h`, `video.startdelay`, `video.linearity`, `video.skip`, `video.skipmin`, `video.skipafter`, `video.playbackmethod`, `video.plcmt`, `video.placement`, `video.api`, `video.maxbitrate`, `video.minbitrate`, `video.delivery`, `video.companionad`, `video.companiontype`, `video.maxseq`, `video.poddur`, `video.mincpmpersec`, `video.battr`

### Audio (within Imp)
`audio.mimes`, `audio.minduration`, `audio.maxduration`, `audio.protocols`, `audio.startdelay`, `audio.poddur`, `audio.feed`, `audio.stitched`, `audio.nvol`, `audio.api`, `audio.battr`, `audio.companionad`, `audio.companiontype`

### Native (within Imp)
`native.request`, `native.ver`, `native.api`, `native.battr`

### PMP / Deals
`pmp.private_auction`, `pmp.deals`

### Site
`site.id`, `site.name`, `site.domain`, `site.page`, `site.ref`, `site.search`, `site.mobile`, `site.cat`, `site.sectioncat`, `site.pagecat`, `site.keywords`, `site.content`, `site.publisher.id`, `site.publisher.name`, `site.publisher.domain`, `site.inventorypartnerdomain`, `site.cattax`

### App
`app.id`, `app.name`, `app.bundle`, `app.domain`, `app.storeurl`, `app.ver`, `app.paid`, `app.keywords`, `app.publisher.id`, `app.cattax`, `app.inventorypartnerdomain`

### DOOH (OpenRTB 2.6+)
`dooh.id`, `dooh.name`, `dooh.venuetype`, `dooh.venuetypetax`, `dooh.publisher.id`

### User
`user.id`, `user.buyeruid`, `user.yob`, `user.gender`, `user.keywords`, `user.customdata`, `user.consent` (TCF string), `user.eids`, `user.data`, `user.geo`

### EID resolution
For each SSP, the `eid` source_type rule specifies the source domain (e.g. `kargo.com`). Resolution at runtime:
1. Walk `user.eids` looking for `eids[i].source == source_ref`
2. If not found, walk `user.ext.eids` (legacy location)
3. Extract `uids[0].id` from the matching entry
4. Write result to `user.buyeruid` (or whichever `field_path` the rule specifies)

### Device
`device.ua`, `device.sua`, `device.ip`, `device.ipv6`, `device.devicetype`, `device.make`, `device.model`, `device.os`, `device.osv`, `device.hwv`, `device.w`, `device.h`, `device.language`, `device.carrier`, `device.mccmnc`, `device.connectiontype`, `device.ifa`, `device.dnt`, `device.lmt`, `device.geo`, `device.ppi`, `device.pxratio`

### Regs
`regs.coppa`, `regs.gdpr`, `regs.us_privacy`, `regs.gpp`, `regs.gpp_sid`

### Source
`source.fd`, `source.tid`, `source.pchain`, `source.schain` (full SupplyChain object with nodes)

---

## Phase 1 — Visibility + DB (no runtime change)

### Deliverables

1. **DB migration** `deployment/migrations/019_bidder_field_rules.sql` — table + seed data for all SSPs listed above

2. **Storage layer** `internal/storage/bidder_field_rules.go` (new file, following patterns in `publishers.go`):
   - `BidderFieldRule` struct
   - `GetBidderFieldRules(ctx, bidderCode string) ([]BidderFieldRule, error)` — returns bidder-specific rules merged over `__default__` rules (bidder-specific wins on conflict)
   - `UpsertBidderFieldRule(ctx, rule BidderFieldRule) error`
   - `DeleteBidderFieldRule(ctx, id int) error`

3. **API routes** in `internal/endpoints/onboarding_admin.go`:
   - `GET /bidder-field-rules?bidder_code=kargo` → merged rules list
   - `PUT /bidder-field-rules` → upsert a rule
   - `DELETE /bidder-field-rules/{id}` → delete a rule

4. **Admin UI** — new **"Bid Request"** section in sidebar:
   - SSP selector dropdown at top (all bidders from `bidders` Vue data)
   - OpenRTB field tree in collapsible sections: BidRequest / Imp / Banner / Video / Audio / Native / Site / App / User / Device / Regs / Source
   - Each row: `field_path` | `source_type` dropdown | `source_ref` input | `transform` dropdown | `required` toggle | `notes` | delete button
   - `__default__` rows shown as greyed baseline; overridden rows highlighted with an indigo badge
   - "Add rule" inline row at bottom of each section
   - Inject `__BIDDER_FIELD_RULES__` server-side (all rules) alongside existing injections

5. **Preview panel**: `GET /admin/bid-request-preview?bidder=kargo&slot_id=123`
   - Server assembles the full OpenRTB JSON using current rules (no live call)
   - Returns pretty-printed JSON
   - Shown in a syntax-highlighted code panel beside the routing table
   - Uses a fixed test fixture when `slot_id` is omitted

**Not in Phase 1:** Runtime engine does not use these rules yet. Adapters remain hardcoded.

---

## Phase 2 — Runtime Engine

### Deliverables

1. **New package** `internal/adapters/routing/`:

   - `loader.go` — loads `bidder_field_rules` from DB into a `sync.Map` keyed by `bidder_code`; TTL-based refresh (30s) + explicit invalidation on rule save
   - `composer.go` — `Composer.Apply(bidderCode, incomingRequest, slotParams, httpCtx) *openrtb.BidRequest`; applies all enabled rules (bidder-specific merged over `__default__`) to produce the outgoing request
   - `rule_applier.go` — handles each `source_type` / `transform` combination; each is a pure function (`applyEID`, `applySlotParam`, `applyHTTPContext`, etc.)
   - `eid_resolver.go` — walks `user.eids` then `user.ext.eids`, matches on `source == domain`, returns `uids[0].id`

2. **Migrate simple adapters** — replace bidder-specific field logic with `Composer.Apply()`:
   - `internal/adapters/kargo/kargo.go`
   - `internal/adapters/sovrn/sovrn.go`
   - `internal/adapters/pubmatic/pubmatic.go`
   - `internal/adapters/triplelift/triplelift.go`
   - `internal/adapters/appnexus/appnexus.go`

3. **Partial Rubicon migration** — engine handles standard fields + EID/buyeruid; Go adapter retains `ext.rp` nesting, Basic auth header injection, PBS identity target, and size_id logic

4. **Cache invalidation** — when a rule is saved via admin UI (`PUT /bidder-field-rules`), call `loader.Invalidate(bidderCode)` so the next request picks up the change with no restart

5. **Telemetry** — structured log event per auction: which rules applied, which fields were absent, which source_types resolved successfully

---

## Files Modified / Created

| Path | Phase | Change |
|------|-------|--------|
| `deployment/migrations/019_bidder_field_rules.sql` | 1 | New table + seed data |
| `internal/storage/bidder_field_rules.go` | 1 | New file: storage methods |
| `internal/endpoints/onboarding_admin.go` | 1 | "Bid Request" tab + API routes + `__BIDDER_FIELD_RULES__` injection |
| `internal/adapters/routing/loader.go` | 2 | New file: DB-backed rule cache |
| `internal/adapters/routing/composer.go` | 2 | New file: rule application engine |
| `internal/adapters/routing/rule_applier.go` | 2 | New file: per-source-type handlers |
| `internal/adapters/routing/eid_resolver.go` | 2 | New file: EID → buyeruid resolution |
| `internal/adapters/kargo/kargo.go` | 2 | Simplify to engine pass-through |
| `internal/adapters/sovrn/sovrn.go` | 2 | Simplify to engine pass-through |
| `internal/adapters/pubmatic/pubmatic.go` | 2 | Simplify to engine pass-through |
| `internal/adapters/triplelift/triplelift.go` | 2 | Simplify to engine pass-through |
| `internal/adapters/appnexus/appnexus.go` | 2 | Simplify to engine pass-through |
| `internal/adapters/rubicon/rubicon.go` | 2 | Remove standard-field logic, keep ext.rp + auth |

---

## Verification

### Phase 1

**Test fixture setup:**
```sql
-- Ensure a Kargo slot config exists with:
-- slot_bidder_configs.bidder_params = {"placementId": "test-kargo-123"}
-- And an EID in the test request: user.ext.eids = [{"source":"kargo.com","uids":[{"id":"test-buyer-456"}]}]
```

1. `go build ./...` — clean compile
2. Admin UI → "Bid Request" tab → select Kargo → routing table shows:
   - `imp.ext.kargo.placementId` ← `slot_param / placementId` (required ✓)
   - `user.buyeruid` ← `eid / kargo.com`
   - Greyed `__default__` rows for `device.ua`, `site.page`, `regs.gdpr`, etc.
3. Edit the Kargo `user.buyeruid` rule: change source_ref from `kargo.com` to `kargo-test.com` → save → reload → change persists
4. Preview for Kargo + test slot → JSON panel shows:
   - `imp[0].ext.bidder.placementId = "test-kargo-123"`
   - `user.buyeruid = "test-buyer-456"` (if EID present in synthetic request)
5. Add a new rule for Kargo (`site.publisher.id` ← `account_param / account_id`) → save → appears in table
6. Delete the new rule → disappears from table

### Phase 2

1. Fire real auction for a Kargo slot → `imp.ext.kargo.placementId` still correct, `user.buyeruid` populated from EID
2. Fire for Rubicon → `ext.rp` nesting still present; `user.buyeruid` now driven by DB EID rule (not hardcoded `rubiconproject.com` string in Go)
3. Modify Kargo EID source domain in admin UI to a dummy value → next auction attempt uses new domain (no restart needed)
4. Add a brand-new SSP (e.g. `newSSP`) entirely via UI: insert rows for `imp.ext.newSSP.placementId` ← `slot_param` + a slot config → trigger a test auction → confirm imp.ext populated correctly
5. Structured log event visible: `{"bidder":"kargo","rules_applied":["imp.ext.kargo.placementId","user.buyeruid"],"eids_resolved":["kargo.com"]}`
