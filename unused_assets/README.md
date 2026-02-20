# Unused Assets

This directory contains code and configuration that has been removed from active use but preserved for reference.

## Removed Bidder Adapters

### OMS (Onemobile)
- **Removed:** 2026-02-12
- **Reason:** Not required for current SSP configuration
- **Location:** `adapters/oms/`
- **Notes:**
  - Previously configured at publisher level
  - User sync endpoint was disabled by default
  - No longer needed per client requirements

### Aniview
- **Removed:** 2026-02-12
- **Reason:** Not required for current SSP configuration
- **Location:** `adapters/aniview/`
- **Notes:**
  - Previously configured at publisher level
  - Client requirements specify different bidder set
  - Removed to simplify codebase

## Active Bidders

The following bidders remain active in the system:
- **Rubicon/Magnite** (per-publisher)
- **Kargo** (per-unit per-domain)
- **Sovrn** (per-unit per-domain)
- **Pubmatic** (per-publisher)
- **Triplelift** (per-publisher)
- **AppNexus** (per-publisher)

## Restoration

If these adapters need to be restored:
1. Move the adapter directory back to `/internal/adapters/`
2. Re-add the bidder params structs to `catalyst_bid_handler.go`
3. Add the bidder code to the active bidders list
4. Re-enable user sync if needed in `usersync/syncer.go`
5. Update documentation
