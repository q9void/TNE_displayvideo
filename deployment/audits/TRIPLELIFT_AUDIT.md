# TripleLift Adapter Audit Report

**Date:** 2026-02-14
**Auditor:** Claude Code
**Scope:** Comparison of internal TripleLift adapter vs. official Prebid Server implementation

---

## Executive Summary

Our TripleLift adapter implementation is **CRITICALLY INCOMPLETE**. While it correctly handles ID clearing and basic request/response flow, it **completely skips parameter extraction and impression processing**, which are essential for the adapter to function properly with TripleLift's requirements.

**Critical Issues:** 1
**Important Differences:** 2
**Minor Differences:** 1
**Correct Implementations:** 3

---

## Critical Issues

### ❌ CRITICAL: Missing Parameter Extraction and Impression Processing

**Issue:** Our implementation does not extract or use TripleLift-specific parameters from impression extensions.

**Official Prebid Implementation:**
```go
func processImp(imp *openrtb2.Imp) error {
    // get the triplelift extension
    var ext adapters.ExtImpBidder
    var tlext openrtb_ext.ExtImpTriplelift
    if err := jsonutil.Unmarshal(imp.Ext, &ext); err != nil {
        return err
    }
    if err := jsonutil.Unmarshal(ext.Bidder, &tlext); err != nil {
        return err
    }
    if imp.Banner == nil && imp.Video == nil {
        return fmt.Errorf("neither Banner nor Video object specified")
    }
    imp.TagID = tlext.InvCode
    // floor is optional
    if tlext.Floor == nil {
        return nil
    } else {
        imp.BidFloor = *tlext.Floor
    }
    return nil
}
```

**TripleLift Extension Structure:**
```go
type ExtImpTriplelift struct {
    InvCode string   `json:"inventoryCode"`  // REQUIRED
    Floor   *float64 `json:"floor"`          // Optional
}
```

**Our Implementation:**
```go
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    requestCopy := *request

    // Only removes IDs - does NOT process impressions
    if requestCopy.Site != nil {
        siteCopy := *requestCopy.Site
        siteCopy.ID = ""
        // ...
    }

    // Missing: No extraction of inventoryCode → imp.TagID
    // Missing: No extraction of floor → imp.BidFloor
    // Missing: No validation of Banner/Video presence
    // Missing: No impression filtering/validation
}
```

**Impact:**
- **inventoryCode** parameter is never extracted and set as `imp.TagID`
- **floor** parameter is never extracted and set as `imp.BidFloor`
- No validation that impressions have Banner or Video objects
- TripleLift will not receive the required TagID for each impression
- Floor prices are not respected
- Invalid impressions are not filtered out

**Validation exists but is not used:**
We have `ValidateTripleLiftParams()` in `/Users/andrewstreets/tnevideo/internal/validation/bidder_params.go` that validates `inventoryCode`, but this validation is never connected to the adapter's impression processing.

**What needs to be fixed:**
1. Create a TripleLift params struct:
```go
type tripleLiftParams struct {
    InventoryCode string   `json:"inventoryCode"`
    Floor         *float64 `json:"floor,omitempty"`
}
```

2. Add `processImp()` or `extractTripleLiftParams()` function similar to Sovrn adapter
3. In `MakeRequests()`, iterate through impressions and:
   - Extract TripleLift parameters from `imp.Ext.triplelift`
   - Set `imp.TagID = params.InventoryCode`
   - Set `imp.BidFloor = *params.Floor` if provided
   - Validate Banner or Video is present
   - Filter out invalid impressions
   - Collect errors for invalid impressions

**Reference Implementation:** See `/Users/andrewstreets/tnevideo/internal/adapters/sovrn/sovrn.go` lines 56-89 for the pattern used in our Sovrn adapter.

---

## Important Differences

### ⚠️ DIFFERENCE: Bid Type Detection Method

**Official Prebid Implementation:**
```go
type TripleliftInnerExt struct {
    Format int `json:"format"`
}

type TripleliftRespExt struct {
    Triplelift TripleliftInnerExt `json:"triplelift_pb"`
}

func getBidType(ext TripleliftRespExt) openrtb_ext.BidType {
    t := ext.Triplelift.Format
    if t == 11 || t == 12 || t == 17 {
        return openrtb_ext.BidTypeVideo
    }
    return openrtb_ext.BidTypeBanner
}

// In MakeBids:
var bidExt TripleliftRespExt
if err := jsonutil.Unmarshal(bid.Ext, &bidExt); err != nil {
    errs = append(errs, err)
} else {
    bidType := getBidType(bidExt)
    // ...
}
```

**Our Implementation:**
```go
// Build impression map for O(1) bid type detection
impMap := adapters.BuildImpMap(request.Imp)

for _, seatBid := range bidResp.SeatBid {
    for i := range seatBid.Bid {
        bid := &seatBid.Bid[i]
        // Detect bid type from impression instead of hardcoding
        bidType := adapters.GetBidTypeFromMap(bid, impMap)
        // ...
    }
}
```

**Analysis:**
- **Prebid approach:** Reads format code from bid response extension (`bid.ext.triplelift_pb.format`)
- **Our approach:** Infers bid type from original impression (Video object → video, else banner)
- **Prebid format codes:**
  - 11, 12, 17 = Video
  - All others = Banner

**Impact:**
- **Our method is simpler** and avoids parsing bid extensions
- **Prebid method is more accurate** because it uses TripleLift's explicit format signal
- **Potential issue:** If TripleLift returns a different media type than requested, we might misclassify it
- **TripleLift might use format codes for other purposes** (e.g., different banner formats)

**Recommendation:**
Switch to Prebid's format-based detection for accuracy. TripleLift explicitly signals the format in their response for a reason. However, this is **lower priority** than fixing the critical parameter extraction issue.

---

### ⚠️ DIFFERENCE: Error Handling for 400 Bad Request

**Official Prebid Implementation:**
```go
if response.StatusCode == http.StatusBadRequest {
    return nil, []error{&errortypes.BadInput{
        Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info", response.StatusCode),
    }}
}
```

**Our Implementation:**
```go
if responseData.StatusCode != http.StatusOK {
    return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
}
```

**Analysis:**
- Prebid uses specific `errortypes.BadInput` for 400 errors (client error)
- We use generic error for all non-200/204 status codes
- Prebid's approach allows upstream code to handle bad input differently from server errors

**Impact:**
- Minor: Our approach is simpler and still returns an error
- Prebid's typed errors allow better error categorization and debugging
- We may want to use `adapters.NewBadRequestError()` from our helpers

**Recommendation:**
Low priority. Our current approach works, but using typed errors would be more consistent with Prebid Server patterns.

---

## Minor Differences

### ℹ️ DIFFERENCE: Impression Validation Timing

**Official Prebid Implementation:**
```go
func (a *TripleliftAdapter) MakeRequests(request *openrtb2.BidRequest, extra *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    errs := make([]error, 0, len(request.Imp)+1)
    reqs := make([]*adapters.RequestData, 0, 1)
    tlRequest := *request
    var validImps []openrtb2.Imp

    // Pre-process the imps
    for _, imp := range tlRequest.Imp {
        if err := processImp(&imp); err == nil {
            validImps = append(validImps, imp)
        } else {
            errs = append(errs, err)
        }
    }

    if len(validImps) == 0 {
        err := fmt.Errorf("No valid impressions for triplelift")
        errs = append(errs, err)
        return nil, errs
    }

    tlRequest.Imp = validImps
    // ... marshal and return
}
```

**Our Implementation:**
```go
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    requestCopy := *request

    // No impression processing - sends all impressions as-is

    requestBody, err := json.Marshal(requestCopy)
    if err != nil {
        return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
    }
    // ...
}
```

**Analysis:**
- Prebid filters impressions and only sends valid ones
- Prebid collects errors for invalid impressions but continues
- Prebid returns error only if ALL impressions are invalid
- We send all impressions without filtering

**Impact:**
- This is part of the critical missing parameter extraction issue
- Without filtering, we send malformed impressions to TripleLift
- TripleLift may reject entire request or return errors

---

## Correct Implementations

### ✅ CORRECT: ID Clearing (Site and Publisher)

**Our Implementation:**
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

**Official Prebid:** Does not clear these IDs (sends them as-is)

**Analysis:**
- Our ID clearing is **intentional security feature** to prevent leaking Catalyst internal IDs
- This is consistent with our other adapters (Sovrn, Kargo, etc.)
- Prebid doesn't need this because they don't have internal tracking IDs
- **This is correct behavior for our use case**

---

### ✅ CORRECT: Endpoint Configuration

**Our Implementation:**
```go
const defaultEndpoint = "https://tlx.3lift.com/s2s/auction"

func New(endpoint string) *Adapter {
    if endpoint == "" {
        endpoint = defaultEndpoint
    }
    return &Adapter{endpoint: endpoint}
}
```

**Official Prebid:**
```go
func Builder(bidderName openrtb_ext.BidderName, config config.Adapter, server config.Server) (adapters.Bidder, error) {
    bidder := &TripleliftAdapter{
        endpoint: config.Endpoint,
    }
    return bidder, nil
}
```

**Analysis:**
- Same default endpoint: `https://tlx.3lift.com/s2s/auction`
- Both allow endpoint override
- Our pattern (constructor with default fallback) is simpler and equally effective
- **Correct**

---

### ✅ CORRECT: HTTP Headers

**Our Implementation:**
```go
headers := http.Header{}
headers.Set("Content-Type", "application/json;charset=utf-8")
headers.Set("Accept", "application/json")
```

**Official Prebid:**
```go
headers := http.Header{}
headers.Add("Content-Type", "application/json;charset=utf-8")
headers.Add("Accept", "application/json")
```

**Analysis:**
- Identical headers
- Minor difference: `Set` vs `Add` (both work for single-value headers)
- **Correct**

---

### ✅ CORRECT: Status Code Handling

**Our Implementation:**
```go
if responseData.StatusCode == http.StatusNoContent {
    return nil, nil
}
if responseData.StatusCode != http.StatusOK {
    return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
}
```

**Official Prebid:**
```go
if response.StatusCode == http.StatusNoContent {
    return nil, nil
}

if response.StatusCode == http.StatusBadRequest {
    return nil, []error{&errortypes.BadInput{...}}
}

if response.StatusCode != http.StatusOK {
    return nil, []error{fmt.Errorf("Unexpected status code: %d. Run with request.debug = 1 for more info", response.StatusCode)}
}
```

**Analysis:**
- We handle 204 No Content correctly (return nil for no bids)
- We handle non-200 status codes correctly (return error)
- Prebid adds special handling for 400 (see earlier section)
- **Functionally correct**, though could use typed errors

---

## Additional Observations

### Content-Type Validation (Our Enhancement)

**Our Implementation:**
```go
// Validate Content-Type before parsing JSON
contentType := responseData.Headers.Get("Content-Type")
if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
    bodyPreview := string(responseData.Body)
    if len(bodyPreview) > 200 {
        bodyPreview = bodyPreview[:200] + "..."
    }
    return nil, []error{fmt.Errorf("invalid Content-Type: %s (expected application/json). Body preview: %s", contentType, bodyPreview)}
}
```

**Official Prebid:** No Content-Type validation

**Analysis:**
- This is a **defensive enhancement** we added
- Provides better debugging information for unexpected responses
- Not critical but helpful for troubleshooting
- **Good defensive programming practice**

---

### Bid Response Parsing

**Our Implementation:**
```go
var bidResp openrtb.BidResponse
if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
    // Enhanced error with response body preview for debugging
    bodyPreview := string(responseData.Body)
    if len(bodyPreview) > 500 {
        bodyPreview = bodyPreview[:500] + "..."
    }
    return nil, []error{fmt.Errorf("failed to parse JSON response: %w. Content-Type: %s, Body preview: %s", err, contentType, bodyPreview)}
}
```

**Official Prebid:**
```go
var bidResp openrtb2.BidResponse
if err := jsonutil.Unmarshal(response.Body, &bidResp); err != nil {
    return nil, []error{err}
}
```

**Analysis:**
- We provide **enhanced error messages** with body previews for debugging
- Prebid uses minimal error reporting
- Our approach is better for debugging production issues
- **Enhancement over official implementation**

---

## Summary of Required Changes

### Priority 1 (CRITICAL - Adapter Non-Functional Without This)

1. **Add parameter extraction** in `MakeRequests()`:
   - Create `tripleLiftParams` struct with `inventoryCode` and `floor`
   - Add function to extract params from `imp.Ext.triplelift`
   - Set `imp.TagID = inventoryCode`
   - Set `imp.BidFloor = floor` if provided
   - Validate Banner or Video is present
   - Filter invalid impressions and collect errors

   **Example pattern:** Follow Sovrn adapter (`/Users/andrewstreets/tnevideo/internal/adapters/sovrn/sovrn.go`)

### Priority 2 (Important - Affects Accuracy)

2. **Switch to format-based bid type detection**:
   - Parse `bid.ext.triplelift_pb.format` in response
   - Use format codes 11, 12, 17 for video; others for banner
   - Keep fallback to impression-based detection if extension missing

### Priority 3 (Nice to Have)

3. **Add typed error for 400 Bad Request**:
   - Use `adapters.NewBadRequestError()` for consistent error handling
   - Allows better upstream error categorization

---

## Testing Recommendations

1. **Test with inventoryCode parameter:**
   ```json
   {
     "imp": [{
       "id": "imp1",
       "banner": {"w": 300, "h": 250},
       "ext": {
         "triplelift": {
           "inventoryCode": "test_inv_code_123"
         }
       }
     }]
   }
   ```
   Verify `imp.TagID` is set to `"test_inv_code_123"`

2. **Test with floor parameter:**
   ```json
   {
     "ext": {
       "triplelift": {
         "inventoryCode": "test_inv_code_123",
         "floor": 1.50
       }
     }
   }
   ```
   Verify `imp.BidFloor` is set to `1.50`

3. **Test missing inventoryCode:**
   Should return error and not send request

4. **Test invalid impression (no Banner/Video):**
   Should filter out impression and return error

5. **Test format-based bid type (after implementing):**
   Mock response with `bid.ext.triplelift_pb.format = 11` → should classify as video

---

## Compliance Status

| Requirement | Status | Notes |
|-------------|--------|-------|
| Parameter extraction | ❌ MISSING | Critical issue - adapter non-functional |
| inventoryCode → TagID | ❌ MISSING | Required for TripleLift to process |
| floor → BidFloor | ❌ MISSING | Optional but should be supported |
| Impression validation | ❌ MISSING | Should validate Banner/Video |
| ID clearing | ✅ CORRECT | Security enhancement |
| Endpoint URL | ✅ CORRECT | Matches official |
| Headers | ✅ CORRECT | Matches official |
| Status code handling | ✅ CORRECT | Handles 204, 200, errors |
| Bid type detection | ⚠️ DIFFERENT | Works but less accurate than format-based |
| Error handling | ⚠️ DIFFERENT | Works but could use typed errors |

---

## Conclusion

The TripleLift adapter requires **immediate fixes** to the request processing logic. The current implementation will not work correctly with TripleLift because it fails to extract and set required parameters (`inventoryCode` → `TagID`).

**Positive aspects:**
- ID clearing implementation is correct and intentional
- Enhanced error reporting with body previews is helpful
- Basic request/response flow is correct
- Endpoint and headers are correct

**Critical gap:**
The missing parameter extraction makes this adapter **non-functional** for actual TripleLift integration. This must be fixed before the adapter can be used in production.

**Recommended action:**
Follow the pattern from our Sovrn adapter to add proper impression processing and parameter extraction. This is a well-understood pattern we've successfully implemented elsewhere in our codebase.
