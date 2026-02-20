# Adapter Audit Summary - Comparison with Official Prebid Server

**Date:** 2026-02-14
**Audited Adapters:** Kargo, TripleLift, PubMatic, Sovrn, Rubicon
**Reference:** https://github.com/prebid/prebid-server

---

## Executive Summary

We audited our 5 primary SSP adapters against the official Prebid Server implementations. Overall findings:

| Adapter | Grade | Status | Critical Issues |
|---------|-------|--------|-----------------|
| **Sovrn** | A | ✅ Production Ready | None |
| **PubMatic** | A- | ✅ Production Ready | Verify Site.ID clearing intentional |
| **Kargo** | B+ | ⚠️ Needs Fix | Media type detection incorrect |
| **Rubicon** | C+ | ⚠️ Needs Fix | Missing authentication |
| **TripleLift** | F | ❌ Non-Functional | Missing parameter extraction |

**Overall Assessment:** 2 adapters ready, 3 need fixes (1 critical, 2 high priority)

---

## Critical Issues (Fix Immediately)

### 1. TripleLift: Missing Parameter Extraction ❌

**Severity:** CRITICAL - Adapter is non-functional

**Problem:**
The adapter doesn't extract `inventoryCode` from impression extensions. TripleLift requires this as `imp.TagID` to identify the placement.

**Current Code:**
```go
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    requestCopy := *request

    // Remove Catalyst internal IDs from Site (prevent ID leakage)
    if requestCopy.Site != nil {
        siteCopy := *requestCopy.Site
        siteCopy.ID = ""
        // ...
    }

    // ❌ MISSING: No parameter extraction from imp.ext.triplelift

    requestBody, err := json.Marshal(requestCopy)
    // ...
}
```

**Required Fix:**
```go
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    requestCopy := *request
    var errs []error
    var validImps []openrtb.Imp

    // Clear internal IDs
    if requestCopy.Site != nil {
        siteCopy := *requestCopy.Site
        siteCopy.ID = ""
        if siteCopy.Publisher != nil {
            pubCopy := *siteCopy.Publisher
            pubCopy.ID = ""
            siteCopy.Publisher = &pubCopy
        }
        requestCopy.Site = &siteCopy
    }

    // Process each impression
    for _, imp := range requestCopy.Imp {
        var tripleliftExt struct {
            InventoryCode string  `json:"inventoryCode"`
            Floor         float64 `json:"floor,omitempty"`
        }

        // Extract TripleLift params from imp.ext.triplelift
        if imp.Ext != nil {
            var impExt struct {
                Triplelift json.RawMessage `json:"triplelift"`
            }
            if err := json.Unmarshal(imp.Ext, &impExt); err != nil {
                errs = append(errs, fmt.Errorf("failed to parse imp.ext for imp %s: %w", imp.ID, err))
                continue
            }
            if err := json.Unmarshal(impExt.Triplelift, &tripleliftExt); err != nil {
                errs = append(errs, fmt.Errorf("failed to parse triplelift params for imp %s: %w", imp.ID, err))
                continue
            }
        }

        // Validate required parameter
        if tripleliftExt.InventoryCode == "" {
            errs = append(errs, fmt.Errorf("imp %s missing required inventoryCode", imp.ID))
            continue
        }

        // Set TagID (required by TripleLift)
        impCopy := imp
        impCopy.TagID = tripleliftExt.InventoryCode

        // Set bid floor if provided
        if tripleliftExt.Floor > 0 {
            impCopy.BidFloor = tripleliftExt.Floor
        }

        validImps = append(validImps, impCopy)
    }

    if len(validImps) == 0 {
        return nil, errs
    }

    requestCopy.Imp = validImps

    requestBody, err := json.Marshal(requestCopy)
    if err != nil {
        return nil, append(errs, fmt.Errorf("failed to marshal request: %w", err))
    }

    headers := http.Header{}
    headers.Set("Content-Type", "application/json;charset=utf-8")
    headers.Set("Accept", "application/json")

    return []*adapters.RequestData{
        {Method: "POST", URI: a.endpoint, Body: requestBody, Headers: headers},
    }, errs
}
```

**File:** `internal/adapters/triplelift/triplelift.go`
**Lines:** 29-41 (replace entire MakeRequests function)

---

### 2. Rubicon: Missing Authentication ❌

**Severity:** CRITICAL - Requests likely rejected

**Problem:**
The adapter doesn't set XAPI credentials (username/password) required by Rubicon.

**Required Fix:**
```go
type Adapter struct {
    endpoint string
    xapiUser string
    xapiPass string
}

func New(endpoint string) *Adapter {
    return &Adapter{
        endpoint: endpoint,
        xapiUser: os.Getenv("RUBICON_XAPI_USER"), // or from config
        xapiPass: os.Getenv("RUBICON_XAPI_PASS"),
    }
}

func (a *Adapter) MakeRequests(...) {
    // ... build request ...

    headers := http.Header{}
    headers.Set("Content-Type", "application/json;charset=utf-8")
    headers.Set("Accept", "application/json")

    // Add basic auth
    if a.xapiUser != "" && a.xapiPass != "" {
        auth := base64.StdEncoding.EncodeToString([]byte(a.xapiUser + ":" + a.xapiPass))
        headers.Set("Authorization", "Basic " + auth)
    }

    // ...
}
```

**Files:**
- `internal/adapters/rubicon/rubicon.go` (add auth)
- `internal/config/config.go` (add XAPI credentials to config)

---

### 3. Kargo: Incorrect Media Type Detection ⚠️

**Severity:** HIGH - Affects bid categorization

**Problem:**
We determine bid type from original impression format, but Kargo's response includes authoritative `bid.ext.mediaType` field. For multi-format impressions, this causes incorrect categorization.

**Current Code:**
```go
func (a *Adapter) MakeBids(...) {
    // ...
    for _, seatBid := range bidResp.SeatBid {
        for i := range seatBid.Bid {
            bid := &seatBid.Bid[i]

            // ❌ Checks original impression, not Kargo's response
            bidType := adapters.GetBidTypeFromMap(bid, impMap)

            typedBids = append(typedBids, &adapters.TypedBid{
                Bid:     bid,
                BidType: bidType,
            })
        }
    }
}
```

**Required Fix:**
```go
func (a *Adapter) MakeBids(...) {
    // ...
    for _, seatBid := range bidResp.SeatBid {
        for i := range seatBid.Bid {
            bid := &seatBid.Bid[i]

            // ✅ Check Kargo's extension first (authoritative)
            bidType := getMediaTypeForBid(bid, impMap)

            typedBids = append(typedBids, &adapters.TypedBid{
                Bid:     bid,
                BidType: bidType,
            })
        }
    }
}

func getMediaTypeForBid(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) openrtb_ext.BidType {
    // Check Kargo's extension first
    if bid.Ext != nil {
        var kargoExt struct {
            MediaType string `json:"mediaType"`
        }
        if err := json.Unmarshal(bid.Ext, &kargoExt); err == nil {
            switch kargoExt.MediaType {
            case "video":
                return openrtb_ext.BidTypeVideo
            case "native":
                return openrtb_ext.BidTypeNative
            case "banner":
                return openrtb_ext.BidTypeBanner
            }
        }
    }

    // Fallback to impression-based detection
    return adapters.GetBidTypeFromMap(bid, impMap)
}
```

**File:** `internal/adapters/kargo/kargo.go`
**Lines:** Add helper function, update MakeBids around line 185-195

---

## Verification Needed

### PubMatic: Site.ID and App.ID Clearing ⚠️

**Issue:**
We clear `Site.ID` and `App.ID`, but the official Prebid adapter preserves them.

**Our Code (lines 119-141):**
```go
siteCopy.ID = ""  // We clear this
appCopy.ID = ""   // We clear this
```

**Official Prebid:**
```go
// Preserves Site.ID and App.ID
// Only sets Publisher.ID from publisherId param
```

**Question:** Is this intentional for privacy/security?

**Recommendation:**
- If intentional (prevent Catalyst ID leakage): Document why and keep
- If not intentional: Align with official behavior and preserve Site.ID/App.ID

**File:** `internal/adapters/pubmatic/pubmatic.go`
**Lines:** 119-141

---

## TNE-Specific Enhancements (Keep These!)

### 1. GZIP Compression ✅

**Adapters:** Kargo, PubMatic

**What we do:**
```go
var requestBody bytes.Buffer
gzipWriter := gzip.NewWriter(&requestBody)
gzipWriter.Write(requestJSON)
gzipWriter.Close()

headers.Set("Content-Encoding", "gzip")
headers.Set("Accept-Encoding", "gzip")
```

**Benefits:**
- 70-90% bandwidth reduction
- Faster request/response times
- Lower network costs

**Status:** ✅ Keep - This is an optimization over official implementation

---

### 2. ID Clearing for Privacy ✅

**All Adapters**

**What we do:**
```go
// Remove Catalyst internal IDs from Site (prevent ID leakage)
if requestCopy.Site != nil {
    siteCopy := *requestCopy.Site
    siteCopy.ID = ""
    if siteCopy.Publisher != nil {
        pubCopy := *siteCopy.Publisher
        pubCopy.ID = ""
        siteCopy.Publisher = &pubCopy
    }
    requestCopy.Site = &siteCopy
}
```

**Benefits:**
- Prevents leaking Catalyst's internal account IDs to SSPs
- Security best practice
- Clean separation between routing IDs and SSP IDs

**Status:** ✅ Keep - This is correct and intentional

---

## Adapter-Specific Findings

### Sovrn ✅ (Grade: A)

**Status:** Production Ready

**What we do correctly:**
- Complete OpenRTB request transformation
- Proper tagid/TagId parameter handling (supports both variants)
- Accurate bid floor handling (string and float64)
- Video parameter validation
- Bid response parsing with URL-unescaping
- Correct bid type detection
- Cookie handling (ljt_reader)

**Minor differences:**
- Error handling style (generic vs typed)
- Missing ImpIDs tracking (may be framework-level)

**Recommendation:** ✅ No changes needed

**Audit Report:** `/Users/andrewstreets/tnevideo/deployment/audits/SOVRN_AUDIT.md`

---

### PubMatic ✅ (Grade: A-)

**Status:** Production Ready (pending verification)

**What we do correctly:**
- Impression validation (Banner, Video, Native; rejects Audio)
- Publisher ID extraction and setting
- Wrapper extension merging
- Ad slot parsing with dimensions
- Bid floor (kadfloor) handling
- First-party data processing
- Extension building (key_val, pmZoneId, gpid, ae, skadn)
- Bid.Cat limiting
- Video duration and deal priority extraction
- Native ad transformation
- In-banner video flag handling

**Enhancements over official:**
- GZIP compression implementation

**Needs verification:**
- Site.ID/App.ID clearing (may be intentional)
- Marketplace Seat field support

**Recommendation:** ⚠️ Verify Site.ID clearing is intentional, otherwise production ready

**Audit Report:** `/Users/andrewstreets/tnevideo/deployment/audits/PUBMATIC_AUDIT.md`

---

### Kargo ⚠️ (Grade: B+)

**Status:** Needs Fix (High Priority)

**What we do correctly:**
- GZIP compression (enhancement)
- ID clearing (security)
- Error handling with context
- Response decompression
- Explicit headers

**Critical issue:**
- ❌ Media type detection uses impression format instead of `bid.ext.mediaType`
- Affects multi-format impressions and bid categorization

**Missing components:**
- Bidder parameters structure (params.go)
- JSON schema validation
- ImpIDs tracking

**Recommendation:** 🔧 Fix media type detection before production use

**Audit Report:** `/Users/andrewstreets/tnevideo/deployment/audits/KARGO_AUDIT.md`

---

### Rubicon ⚠️ (Grade: C+)

**Status:** Needs Multiple Fixes

**What we do correctly:**
- Core request flow (one request per impression)
- Parameter extraction (accountId, siteId, zoneId)
- Extension structures (rp.zone_id, rp.site_id, rp.account_id)
- Publisher ID setting (critical for Rubicon)
- ID clearing (security)
- Response parsing and error handling

**Critical issues:**
- ❌ Missing XAPI basic authentication

**Missing advanced features:**
- OpenRTB 2.6 version conversion
- Bid-on-multiformat support
- First-party data / targeting parameters
- Video extensions (skip, skipdelay, size_id)
- GDPR/USPrivacy handling
- Supply chain (SChain) support
- App request handling
- User extensions (EIDs, consent)
- Native ad support

**Statistics:**
- Our implementation: 384 lines
- Official implementation: ~1,150 lines
- Feature coverage: ~30%

**Recommendation:** 🔧 Add authentication immediately, prioritize targeting/video/multiformat for revenue impact

**Audit Report:** `/Users/andrewstreets/tnevideo/deployment/audits/RUBICON_AUDIT.md`

---

### TripleLift ❌ (Grade: F)

**Status:** Non-Functional

**Critical issue:**
- ❌ Missing parameter extraction entirely
- Adapter doesn't extract `inventoryCode` or `floor` from `imp.ext.triplelift`
- TripleLift won't receive required TagID

**What we do correctly:**
- ID clearing (security)
- Endpoint configuration
- HTTP headers
- Status code handling

**Recommendation:** 🚨 Fix immediately - adapter is non-functional without parameter extraction

**Audit Report:** `/Users/andrewstreets/tnevideo/deployment/audits/TRIPLELIFT_AUDIT.md`

---

## Implementation Priority

### Phase 1: Critical Fixes (1-2 days)

1. **TripleLift:** Add parameter extraction
   - Extract inventoryCode → imp.TagID
   - Extract floor → imp.BidFloor
   - Add validation
   - **Impact:** Makes adapter functional
   - **Effort:** 2-4 hours

2. **Rubicon:** Add authentication
   - Add XAPI username/password to config
   - Set Authorization header
   - **Impact:** Prevents request rejection
   - **Effort:** 1-2 hours

3. **Kargo:** Fix media type detection
   - Check bid.ext.mediaType first
   - Fallback to impression-based
   - **Impact:** Correct bid categorization
   - **Effort:** 1-2 hours

### Phase 2: Verification (1 day)

4. **PubMatic:** Verify Site.ID clearing
   - Determine if intentional for privacy
   - Document decision or align with official
   - **Impact:** Clarity on behavior
   - **Effort:** 2 hours

### Phase 3: Enhanced Features (1-2 weeks)

5. **Rubicon:** Add advanced features
   - Priority 1: Targeting, video extensions, multiformat
   - Priority 2: GDPR/USPrivacy, SChain
   - Priority 3: App support, user extensions
   - **Impact:** Revenue optimization
   - **Effort:** 1-2 weeks

---

## Testing Recommendations

### Unit Tests to Add

1. **TripleLift:**
   - Test parameter extraction from imp.ext.triplelift
   - Test missing inventoryCode error handling
   - Test floor value setting

2. **Kargo:**
   - Test media type detection from bid.ext.mediaType
   - Test multi-format impressions
   - Test fallback to impression-based detection

3. **Rubicon:**
   - Test authentication header presence
   - Test multiformat impression splitting (when implemented)
   - Test targeting parameter propagation (when implemented)

### Integration Tests

1. **All Adapters:**
   - Verify no Catalyst internal IDs in outbound requests
   - Verify GZIP compression working (Kargo, PubMatic)
   - Verify response parsing with real SSP responses

---

## Detailed Audit Reports

Full line-by-line comparisons with code examples and recommendations:

1. `/Users/andrewstreets/tnevideo/deployment/audits/KARGO_AUDIT.md`
2. `/Users/andrewstreets/tnevideo/deployment/audits/TRIPLELIFT_AUDIT.md`
3. `/Users/andrewstreets/tnevideo/deployment/audits/PUBMATIC_AUDIT.md`
4. `/Users/andrewstreets/tnevideo/deployment/audits/SOVRN_AUDIT.md`
5. `/Users/andrewstreets/tnevideo/deployment/audits/RUBICON_AUDIT.md`

---

## Conclusion

**Overall Status:** 2/5 adapters production-ready, 3/5 need fixes

**Critical Fixes Required:** 3 adapters
- TripleLift: Non-functional (missing params)
- Rubicon: Likely failing (missing auth)
- Kargo: Incorrect bid categorization

**Production-Ready:** 2 adapters
- Sovrn: ✅ Fully compliant
- PubMatic: ✅ Compliant (pending Site.ID verification)

**Estimated Effort to Fix All:** 1-2 days for critical issues, 1-2 weeks for full feature parity

**Recommendation:** Fix Phase 1 critical issues immediately before production deployment.
