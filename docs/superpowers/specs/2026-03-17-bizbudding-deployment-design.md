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

Fork the TNE Catalyst repository into a standalone `bizbudding-catalyst` repo. Configure and deploy it at `ads.bizbudding.com`. Add an embedded admin UI so Bizbudding's team can manage SSP configs, placements, and sites without SSH access. Connect to Looker Studio for commercial reporting.

---

## Architecture

```
bizbudding.com/ads.txt
  → authorises ads.bizbudding.com
  → lists DIRECT entries for each active SSP

ads.bizbudding.com
  → Catalyst auction engine (Go binary)
  → Admin UI (embedded, /admin)
  → /sellers.json  (Bizbudding listed as seller)
  → /ads.txt       (auto-generated from active SSP config)

ads.bizbudding.com → PostgreSQL/TimescaleDB (analytics + config)
ads.bizbudding.com → Redis (caching, IVT detection)
ads.bizbudding.com → SSP adapters (Rubicon, Kargo, Sovrn, Pubmatic, TripleLift + others)

Looker Studio → PostgreSQL (reporting)
```

---

## Component Design

### 1. Repository

- New repo: `bizbudding-catalyst` (fork of `TNE_displayvideo`)
- Remove TNE-specific branding, credentials, and publisher configs
- Add `config/bizbudding.yaml` as the primary config file
- Add `.env.example` documenting all required environment variables
- `README.md` rewritten for Bizbudding's ops team (setup, deploy, SSH instructions)

### 2. Auction Engine

No changes to core engine logic. Config-only changes:

- `sellers.json` endpoint at `GET /sellers.json` — serves Bizbudding as the single seller
- `ads.txt` endpoint at `GET /ads.txt` — auto-generated from SSPs that are active in the database; regenerated on config change
- Publisher ID set to `bizbudding.com` throughout default config
- Default active bidders: Rubicon, Kargo, Sovrn, Pubmatic, TripleLift (all existing adapters)

### 3. Display + Video

Both formats are already implemented. Required config wiring:

**Display:**
- Standard IAB sizes (300×250, 728×90, 160×600, 320×50) registered as default sizes
- Existing Bizbudding placement params from `config/bizbudding-all-bidders-mapping.json` migrated into the database via seed script

**Video:**
- VAST/VMAP endpoints active (`/vast`, `/vmap`)
- Pre-roll and mid-roll supported
- Active video SSP adapters: SpotX, Unruly, Beachfront (all present in repo)
- VAST tag builder exposed in admin UI for testing

### 4. Admin UI

Embedded in the Go binary using `embed.FS`. Served at `/admin` (behind basic auth or IP allowlist). Vanilla JS + minimal CSS — no build step, no external dependencies.

**Sidebar navigation sections:**

| Section | Purpose |
|---|---|
| Dashboard | Today's revenue, fill rate, impressions, active SSP count (live from analytics DB) |
| Sites | Add/remove domains (e.g. `bizbudding.com`, `forums.bizbudding.com`) |
| Placements | List all placements with status badges. Add/edit via single-page form |
| SSP Config | Global SSP on/off toggles, default floor prices per SSP |
| Video Tags | VAST tag builder, inline test player |
| ads.txt | Live preview of auto-generated ads.txt, one-click copy |
| Settings | API keys, server config |

**Placement editor (single-page form):**
- Placement name input
- Site dropdown
- Format selector (Display / Video)
- Size multi-select
- SSP rows — toggle on/off, expand to reveal inline param fields per SSP (e.g. Rubicon: accountId, siteId, zoneId; Kargo: placementId)
- Save writes immediately to PostgreSQL, no restart required

**Config persistence:** All admin changes write to PostgreSQL. The auction engine reads config from DB at request time — changes are live immediately with no server restart or SSH needed.

### 5. Deployment

Both options ship in the repo. Bizbudding chooses which to use.

**Native (Go binary):**
```bash
make build
./catalyst-bizbudding --config config/bizbudding.yaml
```
- Systemd unit file at `deployment/catalyst.service`
- Requires: Go 1.21+, PostgreSQL 14+ with TimescaleDB, Redis 7.x
- Deploy script: `scripts/deploy.sh` — rsync binary to server, reload systemd

**Docker Compose:**
```bash
docker compose up -d
```
- `docker-compose.yml` — three services: `catalyst`, `postgres` (TimescaleDB image), `redis`
- All config via environment variables in `.env`
- Volume mounts for data persistence

**SSL:** Let's Encrypt via certbot. Setup instructions in `docs/deployment.md`. nginx config provided for TLS termination + reverse proxy.

**SSH access:** `deployment/SSH_SETUP.md` — documents required server user, SSH key format, and deploy permissions.

### 6. Reporting (Looker Studio)

- TimescaleDB continues to store all bid, impression, and revenue events (existing analytics pipeline, no changes)
- Three SQL views pre-built in `migrations/`:
  - `v_daily_revenue` — revenue by day, broken down by SSP
  - `v_ssp_performance` — win rate, fill rate, avg CPM per SSP
  - `v_placement_fill_rate` — fill rate per placement per day
- `docs/looker-studio-setup.md` — step-by-step guide: enable external connections on PostgreSQL, connect Looker Studio, import the report template
- Looker Studio report template JSON in `reporting/bizbudding-report-template.json` (importable via Looker Studio's "Import from JSON" flow)

---

## ads.txt Trust Chain

When live, Bizbudding pastes the following into `bizbudding.com/ads.txt`:

```
# Auto-generated — copy from ads.bizbudding.com/ads.txt
ads.bizbudding.com, [rubicon-account-id], DIRECT, 0bfd66d529a55807
ads.bizbudding.com, [kargo-account-id], DIRECT
ads.bizbudding.com, [sovrn-account-id], DIRECT
ads.bizbudding.com, [pubmatic-account-id], DIRECT
ads.bizbudding.com, [triplelift-account-id], DIRECT
```

The admin UI's **ads.txt** section generates this automatically from active SSP config and provides a one-click copy.

`ads.bizbudding.com/sellers.json` lists Bizbudding as:
```json
{
  "seller_id": "bizbudding-001",
  "name": "Bizbudding",
  "domain": "bizbudding.com",
  "seller_type": "PUBLISHER"
}
```

---

## Decisions Made

| Decision | Choice | Reason |
|---|---|---|
| Architecture | Single Go binary with embedded admin | Simple to deploy, single process, works native or Docker |
| Admin nav | Sidebar | Familiar ops-tool pattern, easy to extend |
| Placement editor | Single-page form with SSP toggles | Fast for power users, SSP params expand inline |
| Reporting | Looker Studio | Free, shareable, no extra hosting, accessible to non-technical stakeholders |
| Deployment | Both native + Docker | Bizbudding's preference — they decide at install time |

---

## Out of Scope

- Multi-tenant support (other publishers) — this repo is Bizbudding-only
- Rebidding / auction floor optimisation — existing IDR handles this
- Custom DSP integrations — existing adapter pattern handles new SSPs via admin
- CI/CD pipeline — documented as a future step, not included in v1

---

## Success Criteria

- [ ] `ads.bizbudding.com` serves valid OpenRTB bid requests to all 5 default SSPs
- [ ] `bizbudding.com/ads.txt` + `ads.bizbudding.com/sellers.json` form a valid, crawlable trust chain
- [ ] Bizbudding's team can add a new placement + SSP params without SSH access
- [ ] Video pre-roll serving end-to-end via VAST
- [ ] Looker Studio dashboard showing revenue, fill rate, and SSP performance
- [ ] Both native and Docker deployment paths documented and tested
