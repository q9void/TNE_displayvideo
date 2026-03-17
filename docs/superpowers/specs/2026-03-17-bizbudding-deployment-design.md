# Bizbudding Ad Server — Deployment Design

**Date:** 2026-03-17
**Status:** Approved
**Author:** Andrew Streets + Claude

---

## Problem

Bizbudding inventory has been failing SSP validation for weeks. Root cause: the auction engine runs at `ads.thenexusengine.com`, but SSPs crawl `bizbudding.com/ads.txt` to validate inventory ownership. When `ads.txt` references a TNE domain, the trust chain breaks — SSPs reject or discount the inventory.

**Fix:** Move the full stack to `ads.bizbudding.com`. The `ads.txt` at `bizbudding.com` then legitimately authorises a subdomain Bizbudding owns, establishing a valid trust chain for every SSP.

---

## Solution Overview

Fork the TNE Catalyst repository into a standalone `bizbudding-catalyst` repo. Configure and deploy it at `ads.bizbudding.com`. Build an embedded admin UI (net-new) so Bizbudding's team can manage SSP configs, placements, and sites without SSH access. Connect to Looker Studio for commercial reporting.

---

## Architecture

```
bizbudding.com/ads.txt
  → authorises ads.bizbudding.com
  → lists DIRECT entries for each active SSP

ads.bizbudding.com
  → Catalyst auction engine (Go binary, module: github.com/thenexusengine/tne_springwire — module path unchanged for v1)
  → Admin UI (net-new, embedded via embed.FS, served at /admin)
  → /sellers.json  (updated static file — Bizbudding as sole seller)
  → /ads.txt       (new endpoint, generated from ssp_configs table)

ads.bizbudding.com → PostgreSQL 14+ (analytics + config)
ads.bizbudding.com → Redis 7.x (caching, IVT detection)
ads.bizbudding.com → SSP adapters (Rubicon, Kargo, Sovrn, Pubmatic, TripleLift + others)

Looker Studio → PostgreSQL (revenue, fill rate, SSP performance)
```

**Note on module path:** `go.mod` references `github.com/thenexusengine/tne_springwire`. Renaming requires updating all imports across ~80 files. Deferred to a post-v1 cleanup task. The binary is renamed to `catalyst-bizbudding` via `Makefile` without touching the module path.

**Note on TimescaleDB:** The existing analytics schema is plain PostgreSQL (no TimescaleDB hypertables in the current schema). This deployment uses plain PostgreSQL 14+. TimescaleDB is not required. Looker Studio connects directly to PostgreSQL.

---

## Component Design

### 1. Repository

- New repo: `bizbudding-catalyst` (fork of `TNE_displayvideo`)
- Strip TNE credentials, TNE publisher seeds, and TNE-specific `.env` values
- Add `config/bizbudding.yaml` as the primary config file
- Add `.env.example` documenting all required environment variables
- Update `Makefile` — default build target produces `./catalyst-bizbudding`
- `README.md` rewritten for Bizbudding's ops team (setup, deploy, SSH instructions)
- Go version requirement: **Go 1.23+** (per `go.mod`)

### 2. Auction Engine

No changes to core engine logic. Changes are config and endpoint only:

**sellers.json** (`assets/sellers.json` — static file, served by existing `HandleSellersJSON`):
Replace with Bizbudding-only entry. Keep seller_id `NXS001` — SSPs may already have this ID registered; changing it risks breaking existing supply chain relationships. Update `contact_email`, `contact_address`, and `name` to Bizbudding's details. Full IAB-compliant structure required:

```json
{
  "contact_email": "[bizbudding-contact]@bizbudding.com",
  "contact_address": "[Bizbudding legal address]",
  "version": "1.0",
  "identifiers": [
    { "name": "GVLID", "value": "[bizbudding-gvlid-if-registered]" }
  ],
  "sellers": [
    {
      "seller_id": "NXS001",
      "name": "Bizbudding",
      "domain": "bizbudding.com",
      "seller_type": "PUBLISHER"
    }
  ]
}
```

**ads.txt** (`GET /ads.txt` — new endpoint, registered in `cmd/server/server.go`):
Auto-generated from the `ssp_configs` table (see DB schema below). The handler queries all active SSPs, builds the ads.txt lines, and returns plain text. Lines are cached in memory and invalidated when the `ssp_configs` table is updated via the admin UI. SSP cert IDs (TagIDs) to include:

| SSP | Cert ID |
|-----|---------|
| Rubicon (Magnite) | `0bfd66d529a55807` |
| Kargo | `(none published)` |
| Sovrn | `fafdf38b16bf6b2b` |
| Pubmatic | `5d62403b186f2ace` |
| TripleLift | `6c33edb13117fd86` |

Example output:
```
ads.bizbudding.com, [rubicon-account-id], DIRECT, 0bfd66d529a55807
ads.bizbudding.com, [sovrn-account-id], DIRECT, fafdf38b16bf6b2b
ads.bizbudding.com, [pubmatic-account-id], DIRECT, 5d62403b186f2ace
ads.bizbudding.com, [triplelift-account-id], DIRECT, 6c33edb13117fd86
ads.bizbudding.com, [kargo-account-id], DIRECT
```

**Publisher config:** Default publisher is Bizbudding (`domain: bizbudding.com`). The existing `slot_bidder_configs` and `publishers_new` tables in PostgreSQL are the source of truth for placement→SSP mappings. The existing `bizbudding-all-bidders-mapping.json` (currently wired to `totalprosports.com` with publisher ID `12345` — these are TotalProSports values, not Bizbudding values) cannot be used directly. **Pre-implementation blocker:** The correct Bizbudding slot names, SSP-specific account/site/zone IDs per slot, and publisher IDs must be collected from Bizbudding before the seed migration can be written. Seed script skeleton provided in `migrations/007b_seed_bizbudding_placements.sql` with `[PLACEHOLDER]` values to fill in.

**Default active bidders:** Rubicon, Kargo, Sovrn, Pubmatic, TripleLift.

### 3. Display + Video

Both formats are already implemented. Required wiring:

**Display:**
- Standard IAB sizes (300×250, 728×90, 160×600, 320×50) as defaults
- New `bizbudding.com` placement params authored into seed SQL (replacing the `totalprosports.com` entries in current seed files)

**Video:**
- Actual routes: `POST /video/vast` (VAST), `POST /video/pod` (VMAP/ad pod)
- Pre-roll and mid-roll supported
- Active video SSP adapters: SpotX, Beachfront (both confirmed present in `internal/adapters/`). Unruly adapter does **not** exist in the repo and is out of scope for v1.
- VAST tag builder in admin UI generates `/video/vast` URLs, not `/vast`

### 4. Admin UI (Net-New Build)

A new embedded admin UI served at `/admin`. This does not exist in the current codebase — the existing `/admin/*` routes are JSON APIs and raw templates, not a managed UI. This is a significant new build.

**Implementation approach:** Vanilla JS SPA embedded via `embed.FS` in the Go binary. No build step, no npm, no external CDN dependencies. All HTML/CSS/JS lives in `internal/admin/static/`. A new `internal/admin/handler.go` registers the routes and serves the embedded files.

**Authentication:** Extend the existing `AdminAuth` middleware (Bearer token via `ADMIN_API_KEY` env var). The admin UI sends `Authorization: Bearer <token>` on all API calls. No changes to the auth mechanism — reuse what's built.

**Config persistence:** All admin UI changes write to PostgreSQL (the same `publishers_new`, `slot_bidder_configs`, and new `ssp_configs` tables). The auction engine reads from PostgreSQL at request time. **No Redis writes from the admin UI.** The existing `PublisherAdminHandler` (which writes to Redis) is not used by the new admin UI — it is left in place for backward compat but the new UI bypasses it.

**Sidebar navigation sections:**

| Section | Purpose | Data source |
|---|---|---|
| Dashboard | Today's revenue, fill rate, impressions, active SSP count | `auction_events`, `bidder_events`, `win_events` tables |
| Sites | Add/remove domains | `publishers_new` table |
| Placements | List all placements with status badges. Add/edit via single-page form | `slot_bidder_configs` table |
| SSP Config | Global SSP on/off toggles, floor prices per SSP | `ssp_configs` table (new) |
| Video Tags | VAST tag builder generating `/video/vast` URLs, test player | client-side only |
| ads.txt | Live preview of `/ads.txt` output, one-click copy | calls `GET /ads.txt` |
| Settings | API keys, server config vars | `.env` documentation only — no write to env from UI |

**Placement editor (single-page form):**
- Placement name input (maps to `slot_bidder_configs.slot_name`)
- Site dropdown (from `publishers_new`)
- Format selector: Display / Video
- Size multi-select
- SSP rows — toggle on/off (writes to `slot_bidder_configs.status = 'active' | 'paused'`), expand inline to show param fields specific to each SSP (e.g. Rubicon: accountId, siteId, zoneId; Kargo: placementId)
- Save button — single `PUT /admin/api/placements/:id` call, live immediately, no restart

**Admin API routes (consumed by the embedded JS frontend):**

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/admin/api/sites` | List all publishers |
| POST | `/admin/api/sites` | Create site |
| GET | `/admin/api/placements` | List all slot_bidder_configs |
| POST | `/admin/api/placements` | Create placement |
| PUT | `/admin/api/placements/:id` | Update placement (params + status) |
| DELETE | `/admin/api/placements/:id` | Remove placement |
| GET | `/admin/api/ssp-configs` | List all ssp_configs |
| PUT | `/admin/api/ssp-configs/:name` | Toggle SSP active/floor price |
| GET | `/admin/api/dashboard` | Revenue/fill/impression summary from analytics tables |

All routes automatically protected by the existing `AdminAuth` middleware (Bearer token, `ADMIN_API_KEY` env var) since they share the `/admin/` prefix.

### 5. Database Schema Additions

New migration required: `007_create_ssp_configs.sql` (existing migrations run 002–006). New table `ssp_configs`:

```sql
CREATE TABLE ssp_configs (
  id          SERIAL PRIMARY KEY,
  ssp_name    TEXT NOT NULL UNIQUE,        -- e.g. 'rubicon', 'kargo'
  active      BOOLEAN NOT NULL DEFAULT true,
  account_id  TEXT,                        -- SSP-specific primary ID (for ads.txt)
  cert_id     TEXT,                        -- IAB cert ID for ads.txt line
  floor_cpm   NUMERIC(10,4) DEFAULT 0,
  params      JSONB,                       -- SSP-specific extra params
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  updated_at  TIMESTAMPTZ DEFAULT NOW()
);
```

Seed this table with the 5 default SSPs and their cert IDs at deploy time.

Existing tables used: `publishers_new`, `slot_bidder_configs` (config); `auction_events`, `bidder_events`, `win_events` (analytics).

### 6. Deployment

Both options ship in the repo. Bizbudding chooses.

**Native (Go binary):**
```bash
make build           # produces ./catalyst-bizbudding
./catalyst-bizbudding --config config/bizbudding.yaml
```
- Requires: Go 1.23+, PostgreSQL 14+, Redis 7.x
- Systemd unit file at `deployment/catalyst.service` (new file)
- Deploy script: `scripts/deploy.sh` — rsync binary to server, reload systemd

**Docker Compose** (full new `docker-compose.yml` — the existing file covers only `catalyst` + `nginx`):
```yaml
services:
  catalyst:   # Go binary
  postgres:   # postgres:16 image (plain PostgreSQL, no TimescaleDB)
  redis:      # redis:7-alpine
```
All config via `.env`. Volume mounts for data persistence.

**SSL:** Let's Encrypt via certbot. nginx TLS termination config included. New file: `deployment/nginx-bizbudding.conf`.

**SSH deploy docs:** New file `deployment/SSH_SETUP.md` — server user, SSH key format, deploy permissions.

**New documentation files to create:**
- `deployment/SSH_SETUP.md`
- `deployment/nginx-bizbudding.conf`
- `deployment/catalyst.service`
- `docs/deployment.md` (replaces/supersedes existing deployment docs for this repo)
- `docs/looker-studio-setup.md`

### 7. Reporting (Looker Studio)

- Existing PostgreSQL analytics tables stay as-is
- Three SQL views added in a new migration (`008_reporting_views.sql`):
  - `v_daily_revenue` — revenue by day, by SSP (from `win_events` joined to `bidder_events`)
  - `v_ssp_performance` — win rate, fill rate, avg CPM per SSP (from `auction_events`, `bidder_events`, `win_events`)
  - `v_placement_fill_rate` — fill rate per placement per day (from `auction_events`, `win_events`)
- `docs/looker-studio-setup.md` (new file) — step-by-step: enable `pg_hba.conf` external access, connect Looker Studio, import report template
- `reporting/bizbudding-report-template.json` — Looker Studio report template (importable via "Import from JSON")

---

## ads.txt Trust Chain

When live, Bizbudding pastes into `bizbudding.com/ads.txt` the output of `GET ads.bizbudding.com/ads.txt`. The admin UI's **ads.txt** section shows a live preview with a one-click copy button.

`ads.bizbudding.com/sellers.json` lists Bizbudding as the sole publisher with seller_id `NXS001`.

---

## Decisions Made

| Decision | Choice | Reason |
|---|---|---|
| Architecture | Single Go binary with embedded admin | Simple to deploy, single process, works native or Docker |
| Admin nav | Sidebar | Familiar ops-tool pattern |
| Placement editor | Single-page form with SSP toggles | Fast, SSP params expand inline |
| Reporting | Looker Studio + plain PostgreSQL | Free, shareable, no TimescaleDB required |
| Deployment | Both native + Docker | Bizbudding decides at install time |
| Go module path | Unchanged for v1 | Rename requires touching ~80 files; deferred |
| seller_id | Keep NXS001 | SSPs may have this registered; changing risks breaking supply chain |
| Admin auth | Bearer token (ADMIN_API_KEY env var) | Reuse existing AdminAuth middleware |

---

## Out of Scope

- Multi-tenant support (other publishers)
- Auction floor / IDR ML optimisation
- Custom DSP integrations (use existing adapter pattern)
- CI/CD pipeline (documented as future step)
- Go module path rename (deferred to post-v1)
- TimescaleDB migration (plain PostgreSQL sufficient)

---

## Success Criteria

- [ ] `ads.bizbudding.com` serves valid OpenRTB bid requests to all 5 default SSPs
- [ ] `bizbudding.com/ads.txt` + `ads.bizbudding.com/sellers.json` form a valid, crawlable trust chain verified by SSP crawlers
- [ ] Bizbudding's team can add a new placement + SSP params without SSH access
- [ ] Video pre-roll serving end-to-end via VAST (`/video/vast`)
- [ ] Looker Studio dashboard showing revenue, fill rate, and SSP performance connected to live PostgreSQL
- [ ] Both native binary and Docker Compose deployment paths documented and tested
- [ ] `ads.txt` endpoint auto-reflects SSP changes made via admin UI within one request cycle
