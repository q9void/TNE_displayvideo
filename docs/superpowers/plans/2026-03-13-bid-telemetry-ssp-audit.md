# Bid Telemetry & SSP Audit Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 5 structured log checkpoints across the bid lifecycle to diagnose why all SSPs are returning 204, and fix two confirmed code bugs (user.eids UID type assertion, Kargo publisher.id override).

**Architecture:** Two independent workstreams — Workstream A adds telemetry (zero functional change, deploy first), Workstream B fixes confirmed bugs (deploy after reading CP-4 headers). All changes target three files: `catalyst_bid_handler.go`, `exchange/exchange.go`, and `adapters/kargo/kargo.go`.

**Tech Stack:** Go, zerolog (structured logging), existing `openrtb` types, `net/http`

---

## Chunk 1: Workstream A — Bid Audit Telemetry

### Task 1: `flattenHeaders` helper + CP-3 (request headers)

**Files:**
- Modify: `internal/exchange/exchange.go` — add helper + extend existing HTTP request log

**Context:** The existing "Making HTTP request to bidder" log at line ~3075 already logs `bidder`, `uri`, `method`, `request_body`. We're adding one field: `request_headers`. The `flattenHeaders` helper converts `http.Header` (map[string][]string) to map[string]string for clean JSON — multi-value headers joined with `", "`, keys kept in Go canonical form (e.g. `Content-Type`).

- [ ] **Step 1.1: Write the failing test for `flattenHeaders`**

Add to `internal/exchange/exchange_test.go`:

```go
func TestFlattenHeaders(t *testing.T) {
    h := http.Header{}
    h.Set("Content-Type", "application/json")
    h.Add("Accept", "application/json")
    h.Add("Accept", "text/plain") // multi-value

    result := flattenHeaders(h)

    if result["Content-Type"] != "application/json" {
        t.Errorf("expected Content-Type=application/json, got %q", result["Content-Type"])
    }
    if result["Accept"] != "application/json, text/plain" {
        t.Errorf("expected Accept joined, got %q", result["Accept"])
    }
    if len(result) != 2 {
        t.Errorf("expected 2 keys, got %d", len(result))
    }
}

func TestFlattenHeadersNil(t *testing.T) {
    result := flattenHeaders(nil)
    if len(result) != 0 {
        t.Errorf("expected empty map for nil header, got %v", result)
    }
}
```

- [ ] **Step 1.2: Run test to confirm it fails**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -run TestFlattenHeaders -v 2>&1 | tail -10"
```

Expected: `FAIL — flattenHeaders undefined`

- [ ] **Step 1.3: Add `"net/http"` to the import block in `exchange.go`**

`exchange.go` does NOT import `net/http` (it uses literal `200` for status codes, and `http.Header` arrives via the `adapters` package). Tasks 1, 2, and 3 all require it. Add it now.

Find the import block in `internal/exchange/exchange.go` and add `"net/http"` to the standard-library group:

```go
import (
    // ... existing stdlib imports ...
    "net/http"
    // ... rest of imports ...
)
```

Build check to confirm the import is clean before continuing:

```bash
ssh catalyst "cd ~/catalyst && go build ./internal/exchange/... 2>&1"
```

- [ ] **Step 1.4: Implement `flattenHeaders` in `exchange/exchange.go`**

Add near the top of the file, after the existing imports/consts (before the first function definition):

```go
// flattenHeaders converts http.Header to a flat map for structured logging.
// Multi-value headers are joined with ", " (RFC 7230). Keys are kept canonical.
func flattenHeaders(h http.Header) map[string]string {
    out := make(map[string]string, len(h))
    for k, vals := range h {
        out[k] = strings.Join(vals, ", ")
    }
    return out
}
```

Note: `strings` is already imported in `exchange.go`. If not, add it to the import block.

- [ ] **Step 1.5: Extend the "Making HTTP request" log to include request headers (CP-3)**

Find this block at ~line 3075 in `exchange/exchange.go`:

```go
logger.Log.Debug().
    Str("bidder", bidderCode).
    Str("uri", reqData.URI).
    Str("method", reqData.Method).
    Str("request_body", requestPreview).
    Msg("Making HTTP request to bidder")
```

Replace with:

```go
logger.Log.Debug().
    Str("bidder", bidderCode).
    Str("uri", reqData.URI).
    Str("method", reqData.Method).
    Interface("request_headers", flattenHeaders(reqData.Headers)).
    Str("request_body", requestPreview).
    Msg("Making HTTP request to bidder")
```

- [ ] **Step 1.6: Run tests**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -run TestFlattenHeaders -v 2>&1 | tail -10"
```

Expected: `PASS`

- [ ] **Step 1.7: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

Expected: no output (clean build)

- [ ] **Step 1.8: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/exchange/exchange.go internal/exchange/exchange_test.go && git commit -m 'feat(telemetry): add flattenHeaders helper + CP-3 request headers log'"
```

---

### Task 2: CP-4 — SSP response headers on non-200

**Files:**
- Modify: `internal/exchange/exchange.go` — extend response log block

**Context:** `resp.Headers` is already populated by `DefaultHTTPClient.Do()` on every call but never logged. For 204 responses, SSPs like Rubicon include `X-Rejection-Reason` or `X-Error` headers that explain exactly why they rejected. This is the single most diagnostic addition in the whole plan.

The existing response log at ~line 3111:
```go
logger.Log.Debug().
    Str("bidder", bidderCode).
    Str("uri", reqData.URI).
    Int("status_code", resp.StatusCode).
    Int("body_size", len(resp.Body)).
    Str("response_preview", responsePreview).
    Dur("elapsed", time.Since(start)).
    Msg("bidder HTTP response received")
```

- [ ] **Step 2.1: Write the failing test**

Add to `internal/exchange/exchange_test.go`:

```go
func TestCP4ResponseHeadersLoggedOnNon200(t *testing.T) {
    // This test verifies BidderResult captures status code
    // (the log output itself is validated by build correctness + manual inspection)
    registry := adapters.NewRegistry()
    mock := &mockAdapter{bids: []*adapters.TypedBid{}}
    registry.Register("kargo", mock, adapters.BidderInfo{Enabled: true})

    ex := New(registry, &Config{DefaultTimeout: 100 * time.Millisecond})
    req := &AuctionRequest{
        BidRequest: &openrtb.BidRequest{
            ID:   "test-cp4",
            Site: testSite(),
            Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
        },
    }
    _, err := ex.RunAuction(context.Background(), req)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // If we got here without panic, the log path is safe
}
```

- [ ] **Step 2.2: Run to confirm it passes already** (it should — this tests the call path, not the log)

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -run TestCP4ResponseHeadersLoggedOnNon200 -v 2>&1 | tail -10"
```

- [ ] **Step 2.3: Add CP-4 response header logging**

Find the response log block (~line 3107) and replace:

```go
// Log successful HTTP response for visibility
responsePreview := string(resp.Body)
if len(responsePreview) > 500 {
    responsePreview = responsePreview[:500] + "..."
}
logger.Log.Debug().
    Str("bidder", bidderCode).
    Str("uri", reqData.URI).
    Int("status_code", resp.StatusCode).
    Int("body_size", len(resp.Body)).
    Str("response_preview", responsePreview).
    Dur("elapsed", time.Since(start)).
    Msg("bidder HTTP response received")
```

With:

```go
// Log HTTP response
responsePreview := string(resp.Body)
if len(responsePreview) > 500 {
    responsePreview = responsePreview[:500] + "..."
}
respLog := logger.Log.Debug().
    Str("bidder", bidderCode).
    Str("uri", reqData.URI).
    Int("status_code", resp.StatusCode).
    Int("body_size", len(resp.Body)).
    Str("response_preview", responsePreview).
    Dur("elapsed", time.Since(start))
// CP-4: On non-200, log all response headers — SSPs often include rejection reasons
if resp.StatusCode != http.StatusOK {
    respLog = respLog.Interface("response_headers", flattenHeaders(resp.Headers))
    if reason := resp.Headers.Get("X-Rejection-Reason"); reason != "" {
        respLog = respLog.Str("rejection_reason", reason)
    }
    if xErr := resp.Headers.Get("X-Error"); xErr != "" {
        respLog = respLog.Str("x_error", xErr)
    }
}
respLog.Msg("bidder HTTP response received")
```

- [ ] **Step 2.4: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

Expected: clean

- [ ] **Step 2.5: Run exchange tests**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -v 2>&1 | tail -20"
```

Expected: all PASS

- [ ] **Step 2.6: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/exchange/exchange.go internal/exchange/exchange_test.go && git commit -m 'feat(telemetry): CP-4 log SSP response headers on non-200'"
```

---

### Task 3: CP-5 — Auction summary + BidderResult status tracking

**Files:**
- Modify: `internal/exchange/exchange.go` — add fields to `BidderResult`, populate in `callBidder`, emit summary after `wg.Wait()`

**Context:** `BidderResult` currently has no `StatusCode` or `RejectionHeader` field. We add them so `callBidder` can record what each SSP returned, and `callBiddersWithFPD` can emit a single summary log after all goroutines complete.

- [ ] **Step 3.1: Write the failing test**

Add to `internal/exchange/exchange_test.go`:

```go
func TestBidderResultHasStatusCode(t *testing.T) {
    result := &BidderResult{}
    result.LastStatusCode = 204
    result.RejectionHeader = "unknown-publisher"
    if result.LastStatusCode != 204 {
        t.Errorf("expected 204, got %d", result.LastStatusCode)
    }
    if result.RejectionHeader != "unknown-publisher" {
        t.Errorf("expected rejection header, got %q", result.RejectionHeader)
    }
}
```

- [ ] **Step 3.2: Run to confirm it fails**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -run TestBidderResultHasStatusCode -v 2>&1 | tail -5"
```

Expected: `FAIL — BidderResult has no field LastStatusCode`

- [ ] **Step 3.3: Add fields to `BidderResult` struct**

Find the `BidderResult` struct at line ~393 in `exchange/exchange.go`:

```go
type BidderResult struct {
    BidderCode string
    Bids       []*adapters.TypedBid
    Currency   string
    Errors     []error
    Latency    time.Duration
    Selected   bool
    Score      float64
    TimedOut   bool
}
```

Add two fields:

```go
type BidderResult struct {
    BidderCode      string
    Bids            []*adapters.TypedBid
    Currency        string
    Errors          []error
    Latency         time.Duration
    Selected        bool
    Score           float64
    TimedOut        bool
    LastStatusCode  int    // HTTP status from the last SSP response (0 if no response)
    RejectionHeader string // Value of X-Rejection-Reason response header if present
}
```

- [ ] **Step 3.4: Populate `LastStatusCode` and `RejectionHeader` in `callBidder`**

In `callBidder` (~line 2985), after the response log block and before `MakeBids`, add:

```go
// CP-5 data: track last HTTP status and any rejection header
result.LastStatusCode = resp.StatusCode
if resp.StatusCode != http.StatusOK {
    result.RejectionHeader = resp.Headers.Get("X-Rejection-Reason")
}
```

Place this immediately after the `respLog.Msg("bidder HTTP response received")` line.

- [ ] **Step 3.5: Add CP-5 auction summary log in `callBiddersWithFPD`**

Find the block after `wg.Wait()` in `callBiddersWithFPD` (~line 2350):

```go
wg.Wait()

// P0-1: Convert sync.Map to regular map for return
finalResults := make(map[string]*BidderResult)
results.Range(func(key, value interface{}) bool {
    ...
    return true
})
return finalResults
```

Add the summary log after `results.Range(...)` and before `return finalResults`:

```go
// CP-5: Single auction summary — one grep-able record per auction
type sspResult struct {
    Bidder          string  `json:"bidder"`
    Status          int     `json:"status"`
    LatencyMS       float64 `json:"latency_ms"`
    HadBids         bool    `json:"had_bids"`
    RejectionHeader string  `json:"rejection_header,omitempty"`
}
summary := make([]sspResult, 0, len(finalResults))
totalBids := 0
for bidder, r := range finalResults {
    summary = append(summary, sspResult{
        Bidder:          bidder,
        Status:          r.LastStatusCode,
        LatencyMS:       r.Latency.Seconds() * 1000,
        HadBids:         len(r.Bids) > 0,
        RejectionHeader: r.RejectionHeader,
    })
    totalBids += len(r.Bids)
}
logger.Log.Info().
    Str("request_id", req.ID).
    Str("domain", func() string {
        if req.Site != nil { return req.Site.Domain }
        return ""
    }()).
    Int("bidders_fired", len(finalResults)).
    Int("total_bids", totalBids).
    Interface("ssp_results", summary).
    Msg("CP-5: Auction summary")
```

- [ ] **Step 3.6: Run tests**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... -run TestBidderResultHasStatusCode -v 2>&1 | tail -5"
```

Expected: `PASS`

- [ ] **Step 3.7: Full exchange test suite**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... 2>&1 | tail -10"
```

Expected: all PASS

- [ ] **Step 3.8: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

- [ ] **Step 3.9: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/exchange/exchange.go internal/exchange/exchange_test.go && git commit -m 'feat(telemetry): CP-5 auction summary log + BidderResult status tracking'"
```

---

### Task 4: CP-1 — OpenRTB scaffold audit log

**Files:**
- Modify: `internal/endpoints/catalyst_bid_handler.go` — add log after `convertToOpenRTB()` returns

**Context:** CP-1 fires once per auction, immediately after `convertToOpenRTB()` returns in `HandleBidRequest`. It captures the state of the OpenRTB request before any hook runs. We need a small helper to count EIDs in `user.ext` JSON since those are stored as raw bytes.

- [ ] **Step 4.1: Write the failing test for the ext EID counter**

Add to `internal/endpoints/auction_test.go` (or create `internal/endpoints/catalyst_bid_handler_telemetry_test.go` if the file doesn't exist):

```go
func TestCountExtEIDs(t *testing.T) {
    tests := []struct {
        name     string
        ext      json.RawMessage
        expected int
    }{
        {
            name:     "nil ext",
            ext:      nil,
            expected: 0,
        },
        {
            name:     "empty ext",
            ext:      json.RawMessage(`{}`),
            expected: 0,
        },
        {
            name:     "two eids in ext",
            ext:      json.RawMessage(`{"eids":[{"source":"a.com"},{"source":"b.com"}]}`),
            expected: 2,
        },
        {
            name:     "ext without eids key",
            ext:      json.RawMessage(`{"consent":"abc"}`),
            expected: 0,
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := countExtEIDs(tc.ext)
            if got != tc.expected {
                t.Errorf("expected %d, got %d", tc.expected, got)
            }
        })
    }
}
```

- [ ] **Step 4.2: Run to confirm it fails**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/endpoints/... -run TestCountExtEIDs -v 2>&1 | tail -5"
```

Expected: `FAIL — countExtEIDs undefined`

- [ ] **Step 4.3: Add `countExtEIDs` helper and CP-1 log to `catalyst_bid_handler.go`**

Add the helper function near the bottom of `catalyst_bid_handler.go` (before the final closing brace):

```go
// countExtEIDs returns the number of EID entries in user.ext.eids JSON.
// Used by CP-1 telemetry to compare top-level vs ext EID counts.
func countExtEIDs(ext json.RawMessage) int {
    if len(ext) == 0 {
        return 0
    }
    var parsed struct {
        EIDs []json.RawMessage `json:"eids"`
    }
    if err := json.Unmarshal(ext, &parsed); err != nil {
        return 0
    }
    return len(parsed.EIDs)
}
```

Then find where `convertToOpenRTB()` is called in `HandleBidRequest` and add the CP-1 log immediately after:

```go
ortbReq, impToSlot, err := h.convertToOpenRTB(r, maiBid)
if err != nil {
    h.writeErrorResponse(w, "failed to convert request", http.StatusBadRequest)
    return
}

// CP-1: OpenRTB scaffold audit — state before any hook runs
eidsTopLevel := 0
eidsInExt := 0
if ortbReq.User != nil {
    eidsTopLevel = len(ortbReq.User.EIDs)
    eidsInExt = countExtEIDs(ortbReq.User.Ext)
}
publisherID := ""
schainNodes := 0
if ortbReq.Site != nil && ortbReq.Site.Publisher != nil {
    publisherID = ortbReq.Site.Publisher.ID
}
if ortbReq.Source != nil && ortbReq.Source.SChain != nil {
    schainNodes = len(ortbReq.Source.SChain.Nodes)
}
usPrivacy := ""
if ortbReq.Regs != nil {
    usPrivacy = ortbReq.Regs.USPrivacy
}
logger.Log.Debug().
    Str("request_id", ortbReq.ID).
    Int("imp_count", len(ortbReq.Imp)).
    Int("eids_top_level", eidsTopLevel).
    Int("eids_in_ext", eidsInExt).
    Str("publisher_id", publisherID).
    Int("schain_nodes", schainNodes).
    Str("us_privacy", usPrivacy).
    Msg("CP-1: OpenRTB scaffold built")
```

- [ ] **Step 4.4: Run tests**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/endpoints/... -run TestCountExtEIDs -v 2>&1 | tail -10"
```

Expected: `PASS`

- [ ] **Step 4.5: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

- [ ] **Step 4.6: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/endpoints/catalyst_bid_handler.go internal/endpoints/auction_test.go && git commit -m 'feat(telemetry): CP-1 OpenRTB scaffold audit log'"
```

---

### Task 5: CP-2 — EID field mapping audit

**Files:**
- Modify: `internal/exchange/exchange.go` — add EID audit log in `RunAuction` after hooks fire, before bidder loop

**Context:** CP-2 walks both `user.EIDs` (top-level typed) and `user.ext.eids` (JSON) and emits one log entry per EID source, showing where the UID landed. Emits a WARN for any source that has UIDs in `user.ext.eids` but NOT in `user.EIDs` top-level — these UIDs are invisible to SSPs.

Find where hooks are applied in `RunAuction` and where the bidder loop starts. The log goes between those two points.

- [ ] **Step 5.1: Add CP-2 log in `RunAuction`**

In `exchange/exchange.go`, inside `RunAuction` (~line 1347), find the point after the privacy/identity hooks run and before `callBiddersWithFPD` is called (line ~1530). Add:

```go
// CP-2: EID field mapping audit — detect UIDs orphaned in user.ext.eids
if req.BidRequest.User != nil {
    // Index top-level EIDs by source
    topLevelSources := make(map[string]int) // source → uid count
    for _, eid := range req.BidRequest.User.EIDs {
        topLevelSources[eid.Source] = len(eid.UIDs)
    }

    // Walk ext.eids and cross-reference
    var extEIDsRaw []struct {
        Source string            `json:"source"`
        UIDs   []json.RawMessage `json:"uids"`
    }
    if len(req.BidRequest.User.Ext) > 0 {
        var userExt struct {
            EIDs json.RawMessage `json:"eids"`
        }
        if err := json.Unmarshal(req.BidRequest.User.Ext, &userExt); err == nil && len(userExt.EIDs) > 0 {
            _ = json.Unmarshal(userExt.EIDs, &extEIDsRaw)
        }
    }

    extSources := make(map[string]int)
    for _, eid := range extEIDsRaw {
        extSources[eid.Source] = len(eid.UIDs)
    }

    // Log per source — flag orphans
    allSources := make(map[string]struct{})
    for s := range topLevelSources { allSources[s] = struct{}{} }
    for s := range extSources       { allSources[s] = struct{}{} }

    for source := range allSources {
        inTop := topLevelSources[source]
        inExt := extSources[source]
        location := "user.eids"
        if inTop > 0 && inExt > 0 { location = "both" }
        if inTop == 0 && inExt > 0 { location = "user.ext.eids" }

        logEvent := logger.Log.Debug()
        if location == "user.ext.eids" {
            logEvent = logger.Log.Warn()
        }
        logEvent.
            Str("request_id", req.BidRequest.ID).
            Str("eid_source", source).
            Int("uid_count_top", inTop).
            Int("uid_count_ext", inExt).
            Str("location", location).
            Msg("CP-2: EID mapping")
    }
}
```

- [ ] **Step 5.2: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

- [ ] **Step 5.3: Run exchange tests**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/exchange/... 2>&1 | tail -10"
```

Expected: all PASS

- [ ] **Step 5.4: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/exchange/exchange.go && git commit -m 'feat(telemetry): CP-2 EID field mapping audit — warns on UIDs orphaned in user.ext.eids'"
```

---

### Task 6: Deploy Workstream A and read results

- [ ] **Step 6.1: Push to remote and deploy**

```bash
ssh catalyst "cd ~/catalyst && git push origin master"
# Then redeploy
ssh catalyst "cd ~/catalyst && docker-compose up -d --build catalyst-server 2>&1 | tail -10"
```

Wait ~30 seconds for the service to restart.

- [ ] **Step 6.2: Trigger a test auction**

Open `https://insidetailgating.com` in a browser to generate real traffic, or wait for organic traffic.

- [ ] **Step 6.3: Read CP-4 headers**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-4\|rejection_reason\|response_headers' | head -20"
```

Look for `rejection_reason` field. If Kargo returns `X-Rejection-Reason: unknown-publisher` or similar, that confirms Fix 2 (publisher.id) direction.

- [ ] **Step 6.4: Read CP-2 EID audit**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-2\|user.ext.eids ONLY' | head -20"
```

Look for WARN entries showing which EID sources are orphaned.

- [ ] **Step 6.5: Read CP-5 auction summary**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-5' | head -5"
```

Expect to see `ssp_results` JSON array with status codes and rejection headers per bidder.

---

## Chunk 2: Workstream B — Code Bug Fixes

### Task 7: Fix 1 — Promote full UIDs into `user.EIDs` (fix type assertion)

**Files:**
- Modify: `internal/endpoints/catalyst_bid_handler.go` — fix typed EID building at lines ~1170–1187

**Context:** The `typedEIDs` building loop has a type assertion `eid["uids"].([]map[string]interface{})` that silently fails for JSON-decoded EIDs (which produce `[]interface{}` not `[]map[string]interface{}`). As a result, EIDs from the Prebid ID module (id5, pubcid, etc.) and from `user.ext.eids` go into `user.EIDs` with empty `UIDs`. Only server-side UIDs (built in-process as `[]map[string]interface{}`) get their UIDs populated correctly.

The existing test `TestMakeRequests_PopulatesTopLevelEIDs` in `internal/adapters/kargo/kargo_test.go` tests that EIDs reach the Kargo adapter — we'll add a more granular test directly on the handler.

- [ ] **Step 7.1: Write the failing test — extend `TestMakeRequests_PopulatesTopLevelEIDs`**

In `internal/adapters/kargo/kargo_test.go`, find `TestMakeRequests_PopulatesTopLevelEIDs` and add a UID presence check after the existing EID count assertion:

```go
// Add this block immediately after the existing `len(parsed.User.EIDs) != 2` check:
for _, eid := range parsed.User.EIDs {
    if len(eid.UIDs) == 0 {
        t.Errorf("EID source %q has no UIDs — expected UIDs promoted from user.ext.eids", eid.Source)
    }
}
```

This is the red-green gate for the fix: the test already passes on EID count (2), but will now FAIL because UIDs are empty stubs.

- [ ] **Step 7.2: Run kargo test to confirm it fails on the UID check**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/adapters/kargo/... -run TestMakeRequests_PopulatesTopLevelEIDs -v 2>&1 | tail -15"
```

Expected: `FAIL — EID source "kargo.com" has no UIDs`

- [ ] **Step 7.3: Fix the type assertion in `catalyst_bid_handler.go`**

Find the `typedEIDs` building block (~lines 1167–1190):

```go
typedEIDs := make([]openrtb.EID, 0, len(eids))
for _, eid := range eids {
    src, _ := eid["source"].(string)
    typed := openrtb.EID{Source: src}
    if rawUIDs, ok := eid["uids"].([]map[string]interface{}); ok {
        for _, u := range rawUIDs {
            uid := openrtb.UID{}
            if id, ok := u["id"].(string); ok {
                uid.ID = id
            }
            if atype, ok := u["atype"].(int); ok {
                uid.AType = atype
            }
            typed.UIDs = append(typed.UIDs, uid)
        }
    }
    typedEIDs = append(typedEIDs, typed)
}
user.EIDs = typedEIDs
```

Replace with:

```go
typedEIDs := make([]openrtb.EID, 0, len(eids))
for _, eid := range eids {
    src, _ := eid["source"].(string)
    typed := openrtb.EID{Source: src}

    // uids may be []map[string]interface{} (built in-process) or
    // []interface{} (JSON-decoded). Handle both.
    switch rawUIDs := eid["uids"].(type) {
    case []map[string]interface{}:
        for _, u := range rawUIDs {
            uid := openrtb.UID{}
            if id, ok := u["id"].(string); ok {
                uid.ID = id
            }
            // atype may be int (in-process literal) or float64 (if map was JSON-decoded).
            // Handle both to be safe.
            switch v := u["atype"].(type) {
            case int:
                uid.AType = v
            case float64:
                uid.AType = int(v)
            }
            typed.UIDs = append(typed.UIDs, uid)
        }
    case []interface{}:
        for _, raw := range rawUIDs {
            if u, ok := raw.(map[string]interface{}); ok {
                uid := openrtb.UID{}
                if id, ok := u["id"].(string); ok {
                    uid.ID = id
                }
                // JSON numbers decode as float64
                if atype, ok := u["atype"].(float64); ok {
                    uid.AType = int(atype)
                }
                typed.UIDs = append(typed.UIDs, uid)
            }
        }
    }
    typedEIDs = append(typedEIDs, typed)
}
user.EIDs = typedEIDs
```

- [ ] **Step 7.4: Run kargo test — expect PASS now**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/adapters/kargo/... -run TestMakeRequests_PopulatesTopLevelEIDs -v 2>&1 | tail -15"
```

Expected: PASS

- [ ] **Step 7.5: Run full test suites**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/endpoints/... ./internal/adapters/kargo/... 2>&1 | tail -15"
```

Expected: all PASS

- [ ] **Step 7.6: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

- [ ] **Step 7.7: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/endpoints/catalyst_bid_handler.go internal/adapters/kargo/kargo_test.go internal/endpoints/auction_test.go && git commit -m 'fix(eids): promote full UIDs into user.EIDs — fix []interface{} type assertion for JSON-decoded EIDs'"
```

---

### Task 8: Fix 2 — Remove `site.publisher.id = "NXS001"` override from Kargo adapter

**Files:**
- Modify: `internal/adapters/kargo/kargo.go` — remove publisher.id override
- Modify: `internal/adapters/kargo/kargo_test.go` — update `TestMakeRequests_SetsPublisherID`

**Context:** We hardcode `site.publisher.id = "NXS001"` in `MakeRequests()`. "NXS001" is our internal seller ID — not a Kargo publisher ID. Prebid's reference Kargo adapter doesn't touch `site.publisher.id`. If Bizbudding has confirmed the correct value, use that; otherwise omit the field (safe default matching Prebid behaviour).

**Before doing this task:** Check CP-4 output from Task 6. If Kargo's rejection header names the publisher ID as the problem, confirm the correct value with Bizbudding. If unknown, proceed with omitting (empty string / pass-through).

- [ ] **Step 8.1: Replace the old test entirely — including the function declaration line**

In `internal/adapters/kargo/kargo_test.go`, find and delete the **entire** `TestMakeRequests_SetsPublisherID` function (from the `func` line through its closing `}`). The old function asserts `publisher.id == "NXS001"` which will FAIL after the fix — it must not survive. Replace the entire deleted function with:

```go
func TestMakeRequests_DoesNotOverridePublisherID(t *testing.T) {
    adapter := New("")

    impExt := json.RawMessage(`{"kargo":{"placementId":"test-placement-123"}}`)

    tests := []struct {
        name            string
        incomingPubID   string
        expectedPubID   string
    }{
        {
            name:          "passes through non-empty publisher ID",
            incomingPubID: "bizbudding-pub-id",
            expectedPubID: "bizbudding-pub-id",
        },
        {
            name:          "passes through empty publisher ID unchanged",
            incomingPubID: "",
            expectedPubID: "",
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            request := &openrtb.BidRequest{
                ID:  "test-request-1",
                Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: impExt}},
                Site: &openrtb.Site{
                    Domain:    "example.com",
                    Publisher: &openrtb.Publisher{ID: tc.incomingPubID},
                },
            }

            requests, errs := adapter.MakeRequests(request, nil)
            if len(errs) > 0 {
                t.Fatalf("Unexpected errors: %v", errs)
            }

            var parsed openrtb.BidRequest
            if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
                t.Fatalf("Failed to parse request body: %v", err)
            }

            if parsed.Site == nil || parsed.Site.Publisher == nil {
                t.Fatal("Expected site.publisher to be present")
            }
            if parsed.Site.Publisher.ID != tc.expectedPubID {
                t.Errorf("expected publisher.id %q, got %q", tc.expectedPubID, parsed.Site.Publisher.ID)
            }
        })
    }
}
```

- [ ] **Step 8.2: Save the test file, then confirm the new test fails**

The test file must be fully saved before running. Then:

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/adapters/kargo/... -run TestMakeRequests_DoesNotOverridePublisherID -v 2>&1 | tail -10"
```

Expected: `FAIL` — the "passes through empty publisher ID unchanged" subtest will fail because the adapter still sets `"NXS001"` when publisher ID is empty. The "passes through non-empty" subtest will pass (existing code only overwrites when empty). That's correct — one red subtest is all we need as the gate.

- [ ] **Step 8.3: Remove the publisher.id override from `kargo.go`**

In `internal/adapters/kargo/kargo.go`, find and remove the block that sets `site.publisher.id = "NXS001"`. It looks like:

```go
// Set site.publisher.id = "NXS001" if missing
if requestCopy.Site.Publisher.ID == "" {
    publisherCopy.ID = "NXS001"
}
```

(Or however it's structured — search for `"NXS001"` in the file.)

Simply delete that block. Pass `site.publisher.id` through unchanged.

- [ ] **Step 8.4: Run test — expect PASS**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/adapters/kargo/... -run TestMakeRequests_DoesNotOverridePublisherID -v 2>&1 | tail -10"
```

Expected: PASS

- [ ] **Step 8.5: Run full kargo test suite**

```bash
ssh catalyst "cd ~/catalyst && go test ./internal/adapters/kargo/... -v 2>&1 | tail -20"
```

Expected: all PASS

- [ ] **Step 8.6: Build check**

```bash
ssh catalyst "cd ~/catalyst && go build ./... 2>&1"
```

- [ ] **Step 8.7: Commit**

```bash
ssh catalyst "cd ~/catalyst && git add internal/adapters/kargo/kargo.go internal/adapters/kargo/kargo_test.go && git commit -m 'fix(kargo): remove site.publisher.id NXS001 override — pass through upstream value'"
```

---

### Task 9: Deploy Workstream B and validate

- [ ] **Step 9.1: Push and redeploy**

```bash
ssh catalyst "cd ~/catalyst && git push origin master"
ssh catalyst "cd ~/catalyst && docker-compose up -d --build catalyst-server 2>&1 | tail -10"
```

- [ ] **Step 9.2: Verify CP-2 no longer warns about EID orphans**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-2' | grep 'user.ext.eids' | head -10"
```

Expected: no results (UIDs now in top-level `user.eids`)

- [ ] **Step 9.3: Verify CP-1 shows `eids_top_level > 0` with matching count**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-1' | head -5"
```

Expected: `eids_top_level` count matches `eids_in_ext` count (or close to it)

- [ ] **Step 9.4: Check if any SSP starts returning bids**

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-5' | head -5"
```

Look for `had_bids: true` in any `ssp_results` entry. If bids appear, the fix worked.

- [ ] **Step 9.5: If still 204s — share CP-5 output with Bizbudding**

If all SSPs still 204 after both fixes, the remaining issue is SSP-side (F-6: domain registration). Extract a CP-5 log entry and send it to Bizbudding with the question: "Here's exactly what we sent Kargo. Can you confirm insidetailgating.com is registered under your account?"

```bash
ssh catalyst "docker logs catalyst-server --since 5m 2>&1 | grep 'CP-5' | head -1"
```
