# Bid Telemetry & SSP Audit Design

**Date:** 2026-03-13
**Status:** Approved for implementation
**Context:** All active SSPs (Kargo, Rubicon, Pubmatic) returning 204 No Content on every auction from insidetailgating.com. Two confirmed code bugs identified. Three unknowns require telemetry to resolve. One SSP-side hypothesis requires Bizbudding coordination.

---

## Problem Statement

Catalyst bid server is receiving 204 responses from all SSPs within 2–200ms. Privacy consent is working correctly. Request format is mostly correct. Two code-level bugs have been confirmed via comparison against Prebid Server Go reference implementation. Additional root causes are unknown and require telemetry to diagnose.

---

## Goals

1. Add structured telemetry at 5 checkpoints across the bid lifecycle (Workstream A)
2. Fix two confirmed code bugs found via Prebid reference comparison (Workstream B)
3. Produce a per-auction "report card" log entry that can be sent to SSP tech teams
4. Collapse the remaining unknowns into knowns after one deploy

---

## What We Know

### Confirmed Working
- Kargo endpoint: `https://kraken.prod.kargo.com/api/v1/openrtb` (HTTPS, correct)
- `imp.ext.bidder.placementId` rewrite (Kargo requires, we do it correctly)
- schain: `bizbudding.com/9039 → thenexusengine.com/9131`
- Privacy: CCPA `"1---"` passes, GDPR disabled for US traffic, IDs not stripped
- Rubicon Basic auth credentials sent correctly
- Pubmatic `regs.us_privacy → regs.ext.us_privacy` move (correct)
- `user.buyeruid` set for Kargo from kargo.com EID

### Confirmed Code Bugs

**Bug 1 — user.eids stubs (F-1)**

`catalyst_bid_handler.go:1170–1187` creates typed `user.EIDs` with only `Source` populated (no `UIDs`) to pass through the EID filter. Because `user.EIDs != nil`, EID promotion from `user.ext.eids` never fires. SSPs receive `user.eids` with source-only stubs while all real UIDs are buried in `user.ext.eids` (a Prebid legacy field most SSPs don't read).

Prebid Server Go handles this via `moveEIDFrom25To26()` in `openrtb_ext/convert_up.go`, which copies `user.ext.eids → user.EIDs` only when `user.EIDs == nil`. Our stub approach prevents this promotion entirely.

**Bug 2 — site.publisher.id = "NXS001" sent to Kargo (F-2)**

We hardcode `site.publisher.id = "NXS001"` in the Kargo adapter. "NXS001" is our internal seller ID — not a publisher ID registered in Kargo's system. Prebid's reference Kargo adapter does not touch `site.publisher.id` at all. Kargo validates this field against registered publishers; an unrecognised value likely causes instant 204 rejection.

### Unknowns (Telemetry Will Answer)
- **F-3:** What `site.publisher.id` does Kargo actually expect? Options: `""`, `"9039"` (Bizbudding seller ID), or a Kargo-internal publisher ID.
- **F-4:** Is `insidetailgating.com` registered under Bizbudding's Rubicon account (26298)? Rubicon's 7ms 204 suggests domain-level rejection before demand evaluation.
- **F-5:** Does Pubmatic's adSlot `"7079276"` pass their format validation? Prebid's Pubmatic adapter enforces `"tagId@WxH"` or `"tagId"` format.

### SSP-Side Hypothesis (Requires Bizbudding)
- **F-6:** Bizbudding confirmed active on their own domains. insidetailgating.com may not be registered as an authorised publisher domain in their Kargo, Rubicon, or Pubmatic accounts. ads.txt had wrong domain (`thenexusengine.io` instead of `thenexusengine.com`) until 2026-03-13 — SSP crawlers cache for 24–48h.

---

## Workstream A — Bid Audit Logging

Five new structured log checkpoints. All use existing `zerolog` patterns. No new dependencies.

### Scope per checkpoint

| ID | Fires | Location | Key fields |
|---|---|---|---|
| CP-1 | 1× per auction | `catalyst_bid_handler.go` after `convertToOpenRTB()` | `eids_top_level`, `eids_in_ext`, `publisher_id`, `schain_nodes`, `us_privacy` |
| CP-2 | 1× per auction | `exchange/exchange.go` after hooks, before bidder loop | Per-EID: `source`, `uid_count`, `location`. `location` values: `"user.eids"` (top-level, correct), `"user.ext.eids"` (legacy, SSPs won't see it), `"both"`. Emit WARN when `location == "user.ext.eids"`. |
| CP-3 | N× per auction | `exchange/exchange.go` existing HTTP request log | Add `request_headers` field (extend existing log line). Keys canonical (Go `http.Header` default). Multi-value headers joined with `, `. |
| CP-4 | N× per auction | `exchange/exchange.go` after HTTP call, when status ≠ 200 | `response_headers` (full, same format as CP-3). `rejection_reason` = value of `X-Rejection-Reason` response header (empty string if absent). `x_error` = value of `X-Error` response header (empty string if absent). |
| CP-5 | 1× per auction | `exchange/exchange.go` after all bidder goroutines complete | `ssp_results: [{bidder, status, latency_ms, had_bids, rejection_header}]` |

### Shared helper

One new function in `exchange/exchange.go`:

```go
func flattenHeaders(h http.Header) map[string]string
```

Converts `http.Header` (map[string][]string) to flat map for clean JSON logging. Used by CP-3 and CP-4. ~8 lines. Keys are kept in Go canonical form (e.g. `Content-Type`, not `content-type`). Multiple values for the same key are joined with `", "` (RFC 7230 list format).

### Log format example (CP-5)

```json
{
  "level": "info",
  "service": "pbs",
  "request_id": "catalyst-abc123",
  "domain": "insidetailgating.com",
  "bidders_fired": 3,
  "total_bids": 0,
  "ssp_results": [
    {"bidder": "kargo",   "status": 204, "latency_ms": 9,   "had_bids": false, "rejection_header": "unknown-publisher"},
    {"bidder": "rubicon", "status": 204, "latency_ms": 7,   "had_bids": false, "rejection_header": ""},
    {"bidder": "pubmatic","status": 204, "latency_ms": 193, "had_bids": false, "rejection_header": ""}
  ],
  "message": "CP-5: Auction summary"
}
```

---

## Workstream B — Code Bug Fixes

### Fix 1 — Promote user.ext.eids → user.eids with full UIDs

**File:** `catalyst_bid_handler.go`
**Function:** `convertToOpenRTB()` — the section building `user.EIDs` (lines ~1095–1190)

**Change:** Instead of creating source-only stub typed EIDs, build fully-populated `openrtb.EID` structs (with `UIDs` array) from the same EID data being written to `user.ext.eids`. The `user.EIDs` field should mirror `user.ext.eids` with complete data, not stubs.

Specifically: when iterating `eids` to build `user.ext.eids` JSON, simultaneously build `[]openrtb.EID` with proper `UIDs` populated. Assign to `user.EIDs`. The EID filter in `exchange.go` can then work against properly-populated typed EIDs.

**Why user.ext.eids still needs to exist:** Some older SSP adapters (Sovrn, legacy Pubmatic path) may still read from `user.ext.eids`. Keep it — just ensure `user.EIDs` has matching full data.

### Fix 2 — Remove site.publisher.id override in Kargo adapter

**File:** `internal/adapters/kargo/kargo.go`
**Function:** `MakeRequests()`

**Change:** Remove the `site.publisher.id = "NXS001"` override. Pass whatever `site.publisher.id` comes in from the upstream request.

**Fallback:** If Bizbudding cannot confirm the correct value within a reasonable timeframe, omit the field entirely (empty string / zero value). This matches Prebid's reference Kargo adapter behaviour exactly and is safe — Kargo's own sample requests show the field populated with the buyer's own publisher ID, which they manage on their side.

**Note on F-3 resolution:** CP-4 telemetry will confirm whether `site.publisher.id` is the rejection cause (via response headers). It cannot supply the correct value. The correct value must come from Bizbudding's Kargo dashboard or Kargo tech support directly.

---

## Sequence

1. Deploy Workstream A (telemetry) first — no functional changes, zero risk
2. Read CP-4 headers from first auction after deploy
3. If CP-4 reveals `X-Rejection-Reason: unknown-publisher` on Kargo → confirms Fix 2 direction
4. Deploy Workstream B Fix 1 (EID stubs) — highest confidence, directly addresses F-1
5. Deploy Workstream B Fix 2 (publisher.id) — after confirming correct value with Bizbudding
6. Coordinate with Bizbudding on F-6 (domain registration) in parallel

---

## Files Touched

| File | Change |
|---|---|
| `internal/endpoints/catalyst_bid_handler.go` | CP-1 log + Fix 1 (EID promotion) |
| `internal/exchange/exchange.go` | CP-2, CP-3, CP-4, CP-5 logs + `flattenHeaders` helper |
| `internal/adapters/kargo/kargo.go` | Fix 2 (remove publisher.id override) |

---

## Out of Scope

- AppNexus adapter (test bidder, not production)
- Sovrn, TripleLift, other secondary adapters
- GAM integration (Stage 7/8) — no bids to evaluate yet
- Prometheus metrics changes (existing metrics sufficient for now)
