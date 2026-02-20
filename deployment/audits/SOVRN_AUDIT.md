# Sovrn Adapter Implementation Audit

**Date:** 2026-02-14
**Comparison:** TNE Springwire vs. Official Prebid Server
**Files Compared:**
- TNE: `/Users/andrewstreets/tnevideo/internal/adapters/sovrn/sovrn.go`
- Prebid: `github.com/prebid/prebid-server/master/adapters/sovrn/sovrn.go`

---

## Executive Summary

Our Sovrn adapter implementation is **functionally correct** and closely matches the official Prebid Server implementation. The core bid request/response logic is properly implemented with only minor differences in error handling patterns and some additional features we've added (Site ID clearing).

**Overall Assessment:** ✅ Production-ready with minor recommendations

---

## Detailed Comparison

### 1. OpenRTB Request Transformation (MakeRequests)

#### ✅ **Correct Implementations**

Both implementations handle:
- **Header Construction**: Identical headers with Content-Type, User-Agent, X-Forwarded-For, Accept-Language, DNT
- **Cookie Handling**: Both set `ljt_reader` cookie from `User.BuyerUID`
- **TagID Assignment**: Both support `tagid` and `TagId` variants and assign to `imp.TagID`
- **Bid Floor Logic**: Both apply extension bidfloor when `imp.BidFloor == 0`
- **Video Validation**: Both validate MIMEs, MaxDuration, and Protocols for video impressions
- **Impression Filtering**: Both filter out invalid impressions and collect errors

**Code Comparison:**

```go
// TNE (lines 104-114)
headers := http.Header{}
headers.Set("Content-Type", "application/json")
if requestCopy.Device != nil {
    addHeaderIfNonEmpty(headers, "User-Agent", requestCopy.Device.UA)
    addHeaderIfNonEmpty(headers, "X-Forwarded-For", requestCopy.Device.IP)
    addHeaderIfNonEmpty(headers, "Accept-Language", requestCopy.Device.Language)
    if requestCopy.Device.DNT != nil {
        headers.Set("DNT", strconv.Itoa(*requestCopy.Device.DNT))
    }
}

// Prebid (lines 28-36)
headers := http.Header{}
headers.Add("Content-Type", "application/json")
if request.Device != nil {
    addHeaderIfNonEmpty(headers, "User-Agent", request.Device.UA)
    addHeaderIfNonEmpty(headers, "X-Forwarded-For", request.Device.IP)
    addHeaderIfNonEmpty(headers, "Accept-Language", request.Device.Language)
    if request.Device.DNT != nil {
        addHeaderIfNonEmpty(headers, "DNT", strconv.Itoa(int(*request.Device.DNT)))
    }
}
```

#### ⚠️ **Minor Differences**

**1. Site ID Clearing (TNE-specific feature)**

TNE implementation clears Site.ID and Publisher.ID before sending to Sovrn:

```go
// TNE (lines 41-51) - NOT in Prebid
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

**Assessment:** ✅ This is intentional ID leak prevention for Catalyst internal IDs. This is a valid enhancement and does not affect OpenRTB compliance.

**2. Header Method: `Set()` vs `Add()`**

```go
// TNE uses Set()
headers.Set("Content-Type", "application/json")
headers.Set("DNT", strconv.Itoa(*requestCopy.Device.DNT))

// Prebid uses Add()
headers.Add("Content-Type", "application/json")
addHeaderIfNonEmpty(headers, "DNT", strconv.Itoa(int(*request.Device.DNT)))
```

**Assessment:** ✅ No functional difference for single-value headers. Both work correctly.

**3. DNT Type Casting**

```go
// TNE
headers.Set("DNT", strconv.Itoa(*requestCopy.Device.DNT))

// Prebid
addHeaderIfNonEmpty(headers, "DNT", strconv.Itoa(int(*request.Device.DNT)))
```

**Assessment:** ✅ TNE assumes `Device.DNT` is already an `int`, Prebid explicitly casts. Both work if type definitions match OpenRTB spec (which defines DNT as int8).

**4. Extension Parsing Approach**

```go
// TNE (lines 183-206) - Manual map-based extraction
func extractSovrnParams(impExt json.RawMessage) (*sovrnParams, error) {
    var extMap map[string]interface{}
    if err := json.Unmarshal(impExt, &extMap); err != nil {
        return nil, fmt.Errorf("failed to unmarshal imp.ext: %w", err)
    }
    sovrnData, ok := extMap["sovrn"]
    if !ok {
        return nil, fmt.Errorf("no Sovrn parameters found in imp.ext")
    }
    // Marshal back to JSON and unmarshal into struct
    sovrnJSON, err := json.Marshal(sovrnData)
    // ...
}

// Prebid (lines 47-57) - Uses standard ExtImpBidder pattern
var bidderExt adapters.ExtImpBidder
if err := jsonutil.Unmarshal(imp.Ext, &bidderExt); err != nil {
    errs = append(errs, &errortypes.BadInput{Message: err.Error()})
    continue
}
var sovrnExt openrtb_ext.ExtImpSovrn
if err := jsonutil.Unmarshal(bidderExt.Bidder, &sovrnExt); err != nil {
    errs = append(errs, &errortypes.BadInput{Message: err.Error()})
    continue
}
```

**Assessment:** ⚠️ TNE's approach is more manual but functionally equivalent. Prebid uses standardized types from `openrtb_ext` package which provides better type safety.

**5. ImpIDs Tracking**

```go
// Prebid (line 114)
return []*adapters.RequestData{{
    Method:  "POST",
    Uri:     s.URI,
    Body:    reqJSON,
    Headers: headers,
    ImpIDs:  openrtb_ext.GetImpIDs(request.Imp), // ← Tracks impression IDs
}}, errs

// TNE (lines 124-129) - Does NOT set ImpIDs
return []*adapters.RequestData{{
    Method:  "POST",
    URI:     a.endpoint,
    Body:    body,
    Headers: headers,
}}, errs
```

**Assessment:** ⚠️ Missing ImpIDs tracking. This may be used for debugging or metrics in Prebid framework. Check if TNE framework requires this field.

---

### 2. Bid Response Parsing (MakeBids)

#### ✅ **Correct Implementations**

Both implementations:
- Return `nil` for 204 No Content
- Handle 400 as bad input error
- Handle non-200/204 as server errors
- URL-unescape the AdM (creative markup)
- Determine bid type from impression (Banner vs Video)
- Build typed bid response

**Code Comparison:**

```go
// TNE (lines 132-135)
if responseData.StatusCode == http.StatusNoContent {
    return nil, nil
}
if responseData.StatusCode == http.StatusBadRequest {
    return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
}

// Prebid (lines 122-129)
if bidderResponse.StatusCode == http.StatusNoContent {
    return nil, nil
}
if bidderResponse.StatusCode == http.StatusBadRequest {
    return nil, []error{&errortypes.BadInput{
        Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info", bidderResponse.StatusCode),
    }}
}
```

#### ⚠️ **Differences**

**1. Error Types**

```go
// TNE - Uses plain errors
return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
return nil, []error{fmt.Errorf("failed to parse response: %w", err)}

// Prebid - Uses typed errors from errortypes package
return nil, []error{&errortypes.BadInput{Message: "..."}}
return nil, []error{&errortypes.BadServerResponse{Message: "..."}}
```

**Assessment:** ⚠️ TNE uses generic errors, Prebid uses typed errors for better error categorization. Prebid's approach allows upstream code to handle different error types differently (e.g., retry server errors but not bad input).

**2. Bid Type Detection**

```go
// TNE (lines 156-170) - Uses utility function
impMap := adapters.BuildImpMap(request.Imp)
// ...
bidType := adapters.GetBidTypeFromMap(bid, impMap)

// Prebid (lines 148-157) - Inline logic with getImpIdx helper
impIdx, impIdErr := getImpIdx(bid.ImpID, request)
if impIdErr != nil {
    errs = append(errs, impIdErr)
    continue
} else if request.Imp[impIdx].Video != nil {
    bidType = openrtb_ext.BidTypeVideo
}
```

**Assessment:** ✅ Both approaches are correct. TNE's utility function is cleaner and reusable. Prebid's inline approach is more explicit.

**3. AdM Unescaping Error Handling**

```go
// TNE (lines 164-167) - Silently ignores unescape errors
if adm, err := url.QueryUnescape(bid.AdM); err == nil {
    bid.AdM = adm
}

// Prebid (lines 148-165) - Also silently ignores errors
adm, err := url.QueryUnescape(bid.AdM)
if err == nil {
    bid.AdM = adm
    // ... continues processing
}
```

**Assessment:** ✅ Both handle the same way. If unescaping fails, original AdM is used.

**4. Response Initialization**

```go
// TNE (lines 150-154)
response := &adapters.BidderResponse{
    Currency:   bidResp.Cur,
    ResponseID: bidResp.ID,
    Bids:       make([]*adapters.TypedBid, 0),
}

// Prebid (line 142)
response := adapters.NewBidderResponseWithBidsCapacity(5)
```

**Assessment:** ⚠️ Prebid uses a factory function with pre-allocated capacity. TNE manually constructs response. **TNE is missing Currency and ResponseID assignment in the initialization**, but wait - looking again at TNE code, it does set them. Prebid's factory may set defaults internally.

Actually, reviewing TNE code again:
- TNE **does** set `Currency` and `ResponseID` (lines 151-152)
- Prebid uses factory which may or may not set these

**Re-assessment:** ✅ TNE explicitly sets important fields. This is clearer than relying on factory behavior.

---

### 3. Required Parameters

#### ✅ **Identical Handling**

Both implementations:
- Support `tagid` (lowercase) and `TagId` (camelCase) variants
- Return error if neither is provided
- Use helper function `getTagID()`/`getTagId()` to retrieve value

```go
// TNE (lines 209-215)
func getTagID(params *sovrnParams) string {
    if params.TagID != "" {
        return params.TagID
    }
    return params.TagId
}

// Prebid (lines 186-191)
func getTagId(sovrnExt openrtb_ext.ExtImpSovrn) string {
    if len(sovrnExt.Tagid) > 0 {
        return sovrnExt.Tagid
    } else {
        return sovrnExt.TagId
    }
}
```

**Assessment:** ✅ Functionally identical. Prebid uses `len() > 0` check, TNE uses `!= ""` - both valid.

---

### 4. Endpoint URL Construction

#### ✅ **Identical**

```go
// TNE (lines 17, 30-34)
const defaultEndpoint = "https://ap.lijit.com/rtb/bid"
func New(endpoint string) *Adapter {
    if endpoint == "" {
        endpoint = defaultEndpoint
    }
    return &Adapter{endpoint: endpoint}
}

// Prebid (Builder function, lines 193-198)
func Builder(bidderName openrtb_ext.BidderName, config config.Adapter, server config.Server) (adapters.Bidder, error) {
    bidder := &SovrnAdapter{
        URI: config.Endpoint,
    }
    return bidder, nil
}
```

**Assessment:** ✅ Both use configurable endpoint with same default URL.

---

### 5. Headers

#### ✅ **Identical**

Headers set:
1. `Content-Type: application/json`
2. `User-Agent` (from Device.UA)
3. `X-Forwarded-For` (from Device.IP)
4. `Accept-Language` (from Device.Language)
5. `DNT` (from Device.DNT)
6. `Cookie: ljt_reader={BuyerUID}` (from User.BuyerUID)

**Assessment:** ✅ Perfect match.

---

### 6. ID Clearing (Site.ID, Publisher.ID)

#### ⚠️ **TNE-Specific Enhancement**

TNE clears these IDs, Prebid does not.

```go
// TNE ONLY (lines 41-51)
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

**Assessment:** ✅ This is intentional to prevent internal Catalyst ID leakage. Similar pattern was implemented in Kargo adapter. This is a valid enhancement.

**Why Prebid doesn't do this:** Prebid Server is open-source and doesn't have "internal IDs" to hide. Publishers using Prebid directly may want to pass their site/publisher IDs to SSPs.

---

### 7. Bid Floor Handling

#### ✅ **Identical**

Both implementations:
1. Check if `imp.BidFloor == 0`
2. Extract floor from extension using `getExtBidFloor()`
3. Support both string and float64 formats
4. Use extension floor if impression floor is zero

```go
// TNE (lines 74-78)
if imp.BidFloor == 0 {
    if bidFloor := getExtBidFloor(sovrnParams); bidFloor > 0 {
        imp.BidFloor = bidFloor
    }
}

// Prebid (lines 68-71)
extBidFloor := getExtBidFloor(sovrnExt)
if imp.BidFloor == 0 && extBidFloor > 0 {
    imp.BidFloor = extBidFloor
}
```

**Assessment:** ✅ Functionally identical, different code style.

---

### 8. Error Handling

#### ⚠️ **Different Approaches**

**TNE:**
- Uses standard Go errors with `fmt.Errorf()`
- Wraps errors with `%w` verb for error chains
- Generic error messages

**Prebid:**
- Uses typed errors from `errortypes` package:
  - `&errortypes.BadInput{}` - for client errors (400s, validation failures)
  - `&errortypes.BadServerResponse{}` - for server errors (non-200/204/400)
- Provides debug guidance in error messages ("Run with request.debug = 1")

**Example:**

```go
// TNE
errs = append(errs, fmt.Errorf("missing required parameter 'tagid' for imp %s", imp.ID))

// Prebid
errs = append(errs, &errortypes.BadInput{
    Message: "Missing required parameter 'tagid'",
})
```

**Assessment:** ⚠️ Prebid's typed errors allow better error handling upstream. However, TNE's approach includes impression IDs in error messages which is helpful for debugging multi-impression requests.

**Recommendation:** Consider adopting typed errors if TNE framework supports them, but keep impression IDs in error messages.

---

## Additional Observations

### TNE Implementation Strengths

1. **Better Error Context**: Includes impression IDs in error messages
   ```go
   fmt.Errorf("failed to extract Sovrn params for imp %s: %w", imp.ID, err)
   ```

2. **Explicit ID Clearing**: Prevents internal ID leakage with documented comments

3. **Code Documentation**: Uses descriptive struct field names and comments

### Prebid Implementation Strengths

1. **Typed Errors**: Better error categorization for upstream handling

2. **Standard Patterns**: Uses `openrtb_ext` types and `jsonutil` for consistency

3. **ImpIDs Tracking**: Returns impression IDs in RequestData for debugging/metrics

4. **Debug Guidance**: Error messages guide users to enable debug mode

---

## Potential Issues

### ❌ **Critical Issues**

**None identified.** Both implementations are functionally correct.

### ⚠️ **Medium Priority**

1. **Missing ImpIDs in RequestData** (TNE)
   - Prebid sets `ImpIDs: openrtb_ext.GetImpIDs(request.Imp)`
   - TNE doesn't set this field
   - **Impact:** May affect debugging or metrics if TNE framework uses this field
   - **Recommendation:** Check if TNE's adapters.RequestData struct has ImpIDs field and if it's used

2. **Error Type Differences**
   - Prebid uses typed errors for categorization
   - TNE uses generic errors
   - **Impact:** Upstream code cannot distinguish error types without parsing messages
   - **Recommendation:** Adopt typed errors if available in TNE framework

### ⚠️ **Low Priority**

1. **Extension Parsing Approach**
   - TNE manually parses extensions through maps
   - Prebid uses standardized `ExtImpBidder` pattern
   - **Impact:** Slightly less type-safe but functionally equivalent
   - **Recommendation:** Consider adopting standard extension types if available

2. **Response Body in 400 Errors**
   - TNE includes response body: `fmt.Errorf("bad request: %s", string(responseData.Body))`
   - Prebid uses generic message
   - **Assessment:** TNE's approach is actually better for debugging!

---

## Optimizations from Prebid We Could Adopt

### 💡 **Consider Adopting**

1. **Typed Errors**
   ```go
   // Instead of:
   fmt.Errorf("unexpected status: %d", responseData.StatusCode)

   // Use:
   &errortypes.BadServerResponse{
       Message: fmt.Sprintf("unexpected status: %d", responseData.StatusCode),
   }
   ```

2. **ImpIDs Tracking**
   ```go
   return []*adapters.RequestData{{
       Method:  "POST",
       URI:     a.endpoint,
       Body:    body,
       Headers: headers,
       ImpIDs:  getImpIDs(requestCopy.Imp), // Add this if supported
   }}, errs
   ```

3. **Debug Guidance in Errors**
   ```go
   fmt.Errorf("unexpected status: %d. Run with request.debug = 1 for more info", status)
   ```

4. **Pre-allocated Response Capacity**
   ```go
   // Instead of:
   Bids: make([]*adapters.TypedBid, 0),

   // Use:
   Bids: make([]*adapters.TypedBid, 0, 5), // Pre-allocate for ~5 bids
   ```

### 💡 **Keep TNE's Approach**

1. **ID Clearing** - Important for Catalyst privacy
2. **Impression IDs in Errors** - Better debugging
3. **Response Body in 400 Errors** - More informative
4. **Explicit Field Setting** - Clearer than factory functions

---

## Test Recommendations

Based on comparison, verify these test cases:

1. **TagID Variants**
   - ✅ Test both `tagid` and `TagId` spellings work
   - ✅ Test error when neither is provided

2. **Bid Floor Handling**
   - ✅ Extension floor used when imp.BidFloor = 0
   - ✅ Imp floor takes priority when set
   - ✅ String and float64 floor formats work

3. **Video Validation**
   - ✅ Missing MIMEs/MaxDuration/Protocols rejected
   - ✅ Valid video params accepted

4. **ID Clearing**
   - ✅ Site.ID and Publisher.ID cleared before sending
   - ✅ Other site fields preserved

5. **Headers**
   - ✅ All device headers set correctly
   - ✅ DNT header format correct
   - ✅ ljt_reader cookie set

6. **Response Handling**
   - ✅ 204 returns nil
   - ✅ 400 returns error
   - ✅ 200 with valid response parses correctly
   - ✅ AdM URL-unescaping works
   - ✅ Bid type detection (banner vs video)

---

## Conclusion

Our Sovrn adapter implementation is **functionally correct and production-ready**. The core bidding logic matches Prebid Server's official implementation with only minor stylistic differences and one intentional enhancement (ID clearing).

### Key Findings

**✅ What We Do Correctly:**
- Complete OpenRTB request transformation
- Proper header construction with all required fields
- Correct parameter extraction (tagid variants)
- Accurate bid floor handling
- Valid video parameter validation
- Proper bid response parsing
- URL-unescaping of creative markup
- Correct bid type detection

**⚠️ Differences That Might Matter:**
- Missing ImpIDs tracking in RequestData (check if needed)
- Generic errors vs typed errors (affects error handling)
- Manual extension parsing vs standardized types (stylistic)

**❌ Critical Issues:**
- None

**💡 Optional Optimizations:**
- Add typed errors if framework supports them
- Add ImpIDs tracking if framework uses it
- Include debug guidance in error messages
- Pre-allocate response slice capacity

### Final Verdict

**Status:** ✅ **APPROVED FOR PRODUCTION**

The adapter correctly implements the Sovrn bidding protocol and matches official Prebid Server behavior. The ID clearing enhancement is intentional and appropriate for Catalyst's privacy requirements. No critical issues identified.
