# Kargo Adapter Implementation Audit

**Date:** 2026-02-14
**Auditor:** Claude Code
**Scope:** Comparison of internal Kargo adapter vs. official Prebid Server implementation

---

## Executive Summary

Our Kargo adapter implementation includes several **enhancements** over the official Prebid Server version, particularly around GZIP compression and ID privacy. However, there are **critical differences** in bid type detection logic that may affect bid categorization, and we're missing the bidder params structure entirely.

**Overall Assessment:** Mostly compliant with functional enhancements, but with one significant behavioral difference in media type detection.

---

## Implementation Comparison

### 1. MakeRequests - Request Transformation

#### Official Prebid Server Implementation
```go
func (a *adapter) MakeRequests(request *openrtb2.BidRequest, requestInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    requestJSON, err := json.Marshal(request)
    if err != nil {
        return nil, []error{err}
    }

    requestData := &adapters.RequestData{
        Method: "POST",
        Uri:    a.URI,
        Body:   requestJSON,
        ImpIDs: openrtb_ext.GetImpIDs(request.Imp),
    }

    return []*adapters.RequestData{requestData}, nil
}
```

**Key characteristics:**
- Simple pass-through: marshals request as-is
- No request modification
- No compression
- No header customization
- Includes ImpIDs for tracking

#### Our Implementation
```go
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
    // Create a copy of the request to modify
    requestCopy := *request

    // Remove Catalyst internal IDs from Site
    if requestCopy.Site != nil {
        siteCopy := *requestCopy.Site
        siteCopy.ID = "" // Remove site.id

        if siteCopy.Publisher != nil {
            pubCopy := *siteCopy.Publisher
            pubCopy.ID = "" // Remove publisher.id
            siteCopy.Publisher = &pubCopy
        }
        requestCopy.Site = &siteCopy
    }

    // Remove Catalyst internal IDs from App (if present)
    if requestCopy.App != nil {
        appCopy := *requestCopy.App
        appCopy.ID = "" // Remove app.id

        if appCopy.Publisher != nil {
            pubCopy := *appCopy.Publisher
            pubCopy.ID = "" // Remove publisher.id
            appCopy.Publisher = &pubCopy
        }
        requestCopy.App = &appCopy
    }

    requestBody, err := json.Marshal(requestCopy)
    if err != nil {
        return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
    }

    // Compress request body with GZIP
    var compressedBody bytes.Buffer
    gzipWriter := gzip.NewWriter(&compressedBody)
    if _, err := gzipWriter.Write(requestBody); err != nil {
        return nil, []error{fmt.Errorf("failed to gzip request body: %w", err)}
    }
    if err := gzipWriter.Close(); err != nil {
        return nil, []error{fmt.Errorf("failed to close gzip writer: %w", err)}
    }

    headers := http.Header{}
    headers.Set("Content-Type", "application/json;charset=utf-8")
    headers.Set("Content-Encoding", "gzip")
    headers.Set("Accept", "application/json")
    headers.Set("Accept-Encoding", "gzip")

    return []*adapters.RequestData{{
        Method:  "POST",
        URI:     a.endpoint,
        Body:    compressedBody.Bytes(),
        Headers: headers,
    }}, nil
}
```

**Key enhancements:**
- **ID Privacy:** Removes internal Catalyst IDs (site.id, app.id, publisher.id)
- **GZIP Compression:** Compresses request body to reduce bandwidth
- **Custom Headers:** Sets proper encoding and content-type headers
- **Error Wrapping:** Better error messages with context

#### Verdict: ✅ ENHANCEMENT

**Analysis:**
- Our implementation is functionally superior for production use
- ID clearing prevents leaking internal identifiers to Kargo
- GZIP compression reduces network overhead (important for video/large requests)
- Header configuration is more explicit and correct
- **Missing:** We don't set the `ImpIDs` field that Prebid uses for tracking, but this may be handled at a higher level in our architecture

---

### 2. MakeBids - Response Parsing

#### Official Prebid Server Implementation
```go
func (a *adapter) MakeBids(request *openrtb2.BidRequest, requestData *adapters.RequestData, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
    if responseData.StatusCode == http.StatusNoContent {
        return nil, nil
    }

    if responseData.StatusCode != http.StatusOK {
        err := &errortypes.BadServerResponse{
            Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info.", responseData.StatusCode),
        }
        return nil, []error{err}
    }

    var response openrtb2.BidResponse
    if err := jsonutil.Unmarshal(responseData.Body, &response); err != nil {
        return nil, []error{err}
    }

    bidResponse := adapters.NewBidderResponseWithBidsCapacity(len(request.Imp))
    bidResponse.Currency = response.Cur
    for _, seatBid := range response.SeatBid {
        for i, bid := range seatBid.Bid {
            b := &adapters.TypedBid{
                Bid:     &seatBid.Bid[i],
                BidType: getMediaTypeForBid(bid.Ext),
            }
            bidResponse.Bids = append(bidResponse.Bids, b)
        }
    }
    return bidResponse, nil
}

func getMediaTypeForBid(ext json.RawMessage) openrtb_ext.BidType {
    var impExt kargoExt
    if err := jsonutil.Unmarshal(ext, &impExt); err == nil {
        switch impExt.MediaType {
        case string(openrtb_ext.BidTypeVideo):
            return openrtb_ext.BidTypeVideo
        case string(openrtb_ext.BidTypeNative):
            return openrtb_ext.BidTypeNative
        }
    }
    return openrtb_ext.BidTypeBanner
}
```

**Key characteristics:**
- Status code validation (204 = no content, 200 = success)
- **Media type detection from bid.ext.mediaType** (Kargo-specific extension)
- Falls back to banner if extension missing or parsing fails
- Uses typed error for bad responses

#### Our Implementation
```go
func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
    if responseData.StatusCode == http.StatusNoContent {
        return nil, nil
    }

    if responseData.StatusCode == http.StatusBadRequest {
        return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
    }

    if responseData.StatusCode != http.StatusOK {
        return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
    }

    // Decompress response if GZIP-encoded
    responseBody := responseData.Body
    if responseData.Headers.Get("Content-Encoding") == "gzip" {
        gzipReader, err := gzip.NewReader(bytes.NewReader(responseData.Body))
        if err != nil {
            return nil, []error{fmt.Errorf("failed to create gzip reader: %w", err)}
        }
        defer gzipReader.Close()

        var decompressed bytes.Buffer
        if _, err := decompressed.ReadFrom(gzipReader); err != nil {
            return nil, []error{fmt.Errorf("failed to decompress response: %w", err)}
        }
        responseBody = decompressed.Bytes()
    }

    var bidResp openrtb.BidResponse
    if err := json.Unmarshal(responseBody, &bidResp); err != nil {
        return nil, []error{fmt.Errorf("failed to parse response: %w", err)}
    }

    response := &adapters.BidderResponse{
        Currency:   bidResp.Cur,
        ResponseID: bidResp.ID,
        Bids:       make([]*adapters.TypedBid, 0),
    }

    // Build impression map for O(1) bid type detection
    impMap := adapters.BuildImpMap(request.Imp)

    for _, seatBid := range bidResp.SeatBid {
        for i := range seatBid.Bid {
            bid := &seatBid.Bid[i]
            bidType := adapters.GetBidTypeFromMap(bid, impMap)

            response.Bids = append(response.Bids, &adapters.TypedBid{
                Bid:     bid,
                BidType: bidType,
            })
        }
    }

    return response, nil
}
```

**Key enhancements:**
- **GZIP decompression:** Handles compressed responses
- **Better error handling:** Separate handling for 400 vs other errors
- **ResponseID tracking:** Captures response.id for debugging

**CRITICAL DIFFERENCE:**
- **Media type detection method is completely different**

#### Verdict: ⚠️ CRITICAL BEHAVIORAL DIFFERENCE

**Analysis:**

Our implementation uses **impression-based type detection** (checking if imp.Video/Native/Banner exists), while Prebid uses **bid extension-based detection** (checking bid.ext.mediaType).

**Prebid's approach:**
```go
// Looks at bid.ext for Kargo's mediaType signal
type kargoExt struct {
    MediaType string `json:"mediaType"`
}
```

**Our approach:**
```go
// Looks at original request impression to determine type
func GetBidTypeFromMap(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) BidType {
    imp, ok := impMap[bid.ImpID]
    if !ok {
        return BidTypeBanner
    }

    if imp.Video != nil {
        return BidTypeVideo
    }
    if imp.Native != nil {
        return BidTypeNative
    }
    return BidTypeBanner
}
```

**Why this matters:**

1. **Kargo may return a different media type than requested** - For example, if a publisher requests video but Kargo can only serve banner, they'll indicate this in `bid.ext.mediaType = "banner"`

2. **Our method assumes the bid type matches the request** - We look at the original impression type, not what Kargo actually returned

3. **Multi-format impressions** - If an impression supports both banner and video, Kargo uses the extension to indicate which format they're serving

**Recommendation:**
We should adopt Prebid's approach or create a hybrid that checks both. Kargo specifically includes the `mediaType` extension to signal what they're actually serving.

**Potential issues:**
- Bid categorization may be incorrect if Kargo serves a different format than requested
- Multi-format auctions may not work correctly
- Analytics/reporting may show wrong impression types

---

### 3. Required Parameters

#### Official Prebid Server Schema
```json
{
    "title": "Kargo Adapter Params",
    "type": "object",
    "properties": {
      "placementId": {
        "type": "string",
        "description": "An ID which identifies the adslot placement"
      },
      "adSlotID": {
        "type": "string",
        "description": "[Deprecated: Use `placementId`] An ID which identifies the adslot placement"
      }
    },
    "oneOf": [
      { "required": ["placementId"] },
      { "required": ["adSlotID"] }
    ]
}
```

**Valid examples:**
- `{"placementId": "11523"}`
- `{"adSlotID": "11523"}` (deprecated but supported)

**Invalid:**
- `{"placementId": "11523", "adSlotID": "12345"}` - Cannot have both
- `{}` - Must have one

#### Our Implementation

**MISSING:** We don't have a `params.go` file defining the bidder parameters schema.

#### Verdict: ❌ MISSING IMPLEMENTATION

**Analysis:**
- We have no formal parameter validation
- No struct definition for Kargo-specific params
- The adapter works without parameters because we pass the entire request through
- However, for proper Prebid compatibility, we should define params even if unused

**Impact:**
- Low impact since Kargo doesn't require preprocessing based on params
- Parameters are likely passed in impression extensions
- Would be good for documentation and future compatibility

---

### 4. Endpoint URL Construction

#### Official Prebid Server
```go
URI: config.Endpoint  // Set via configuration
```

#### Our Implementation
```go
const (
    defaultEndpoint = "https://krk.kargo.com/api/v1/openrtb"
)

func New(endpoint string) *Adapter {
    if endpoint == "" {
        endpoint = defaultEndpoint
    }
    return &Adapter{endpoint: endpoint}
}
```

#### Verdict: ✅ CORRECT

**Analysis:**
- Both use configurable endpoints
- Our default matches Kargo's production endpoint
- Endpoint is correctly used without modification
- No special URL construction needed (unlike some adapters that append paths)

---

### 5. Headers & GZIP Compression

#### Official Prebid Server
- **Request:** No custom headers set (uses framework defaults)
- **Response:** No decompression handling

#### Our Implementation
- **Request:** Explicit headers with GZIP compression
  ```go
  headers.Set("Content-Type", "application/json;charset=utf-8")
  headers.Set("Content-Encoding", "gzip")
  headers.Set("Accept", "application/json")
  headers.Set("Accept-Encoding", "gzip")
  ```
- **Response:** GZIP decompression support
  ```go
  if responseData.Headers.Get("Content-Encoding") == "gzip" {
      // Decompress...
  }
  ```

#### Verdict: ✅ ENHANCEMENT

**Analysis:**
- Our GZIP support is a significant bandwidth optimization
- Properly signals compression capabilities to Kargo
- Correctly handles compressed responses
- This is particularly valuable for video ad markup which can be large

**Trade-off:**
- Adds CPU overhead for compression/decompression
- But saves significant bandwidth (typically 70-90% size reduction for JSON)
- Worth it for production environments with high traffic

---

### 6. ID Clearing (site.id, publisher.id)

#### Official Prebid Server
- No ID clearing
- Passes site.id and publisher.id as-is

#### Our Implementation
- Explicitly clears:
  - `site.id`
  - `site.publisher.id`
  - `app.id`
  - `app.publisher.id`

#### Verdict: ✅ PRIVACY ENHANCEMENT

**Analysis:**
- Prevents leaking internal Catalyst identifiers to Kargo
- Good privacy practice
- Aligns with data minimization principles
- Kargo doesn't need these IDs (they use their own placement IDs)

**Benefit:**
- Protects proprietary publisher/site ID scheme
- Reduces data sharing with third parties
- Maintains auction functionality (other fields like domain, bundle remain)

---

### 7. Error Handling

#### Official Prebid Server
```go
// Simple error return
return nil, []error{err}

// Typed error for bad server responses
err := &errortypes.BadServerResponse{
    Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info.", responseData.StatusCode),
}
```

#### Our Implementation
```go
// Wrapped errors with context
return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
return nil, []error{fmt.Errorf("failed to gzip request body: %w", err)}
return nil, []error{fmt.Errorf("failed to decompress response: %w", err)}

// Separate handling for different status codes
if responseData.StatusCode == http.StatusBadRequest {
    return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
}
```

#### Verdict: ✅ IMPROVEMENT

**Analysis:**
- Our error messages are more descriptive
- Error wrapping preserves stack traces
- Separate 400 handling includes response body for debugging
- Better developer experience for troubleshooting

**Missing:**
- We don't use typed errors like Prebid's `errortypes.BadServerResponse`
- This might affect error classification in higher-level retry logic

---

## Summary Tables

### Features Comparison

| Feature | Official Prebid | Our Implementation | Status |
|---------|----------------|-------------------|--------|
| **Request Transformation** | Pass-through | ID clearing + GZIP | ✅ Enhanced |
| **GZIP Compression** | No | Yes (request + response) | ✅ Enhanced |
| **ID Privacy** | No | Yes (clears internal IDs) | ✅ Enhanced |
| **Headers** | Framework defaults | Explicit headers | ✅ Enhanced |
| **Media Type Detection** | From bid.ext.mediaType | From request impression | ⚠️ Different |
| **Bidder Params** | Defined (placementId) | Not defined | ❌ Missing |
| **Error Handling** | Typed errors | Wrapped errors | ✅ Good |
| **Response Decompression** | No | Yes | ✅ Enhanced |
| **Status Code Handling** | 204, 200 | 204, 400, 200 | ✅ Enhanced |

---

## Critical Issues

### 🔴 CRITICAL: Media Type Detection Logic Difference

**Issue:** Our implementation determines bid type from the original request impression, while Kargo signals the actual served type via `bid.ext.mediaType`.

**Impact:**
- Bids may be miscategorized
- Multi-format requests won't work correctly
- Analytics will show incorrect media types

**Example scenario:**
```javascript
// Request has both banner and video
imp: {
    id: "1",
    banner: { w: 300, h: 250 },
    video: { w: 640, h: 480 }
}

// Kargo decides to serve banner
bid.ext.mediaType = "banner"

// Our code will check: imp.Video != nil → returns BidTypeVideo ❌
// Prebid code will check: bid.ext.mediaType → returns BidTypeBanner ✅
```

**Recommended fix:**
```go
func getMediaTypeForBid(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) BidType {
    // First check Kargo's extension (authoritative)
    if bid.Ext != nil {
        var kargoExt struct {
            MediaType string `json:"mediaType"`
        }
        if err := json.Unmarshal(bid.Ext, &kargoExt); err == nil {
            switch kargoExt.MediaType {
            case "video":
                return BidTypeVideo
            case "native":
                return BidTypeNative
            case "banner":
                return BidTypeBanner
            }
        }
    }

    // Fallback to impression-based detection
    return adapters.GetBidTypeFromMap(bid, impMap)
}
```

---

## Missing Components

### ❌ Bidder Parameters Definition

**What's missing:**
- `params.go` with parameter structs
- JSON schema for validation
- Tests for parameter validation

**Reference from Prebid:**
```go
// We should have something like:
package kargo

type ImpExtKargo struct {
    PlacementID string `json:"placementId,omitempty"`
    AdSlotID    string `json:"adSlotID,omitempty"`  // Deprecated
}
```

**Impact:** Low - we're not using these for request preprocessing, but should add for completeness.

---

## Optimizations We Could Adopt

### 💡 From Prebid Server

1. **Pre-allocated bid slice capacity**
   ```go
   // Prebid does this:
   bidResponse := adapters.NewBidderResponseWithBidsCapacity(len(request.Imp))

   // We do:
   Bids: make([]*adapters.TypedBid, 0)  // No capacity hint

   // Should do:
   Bids: make([]*adapters.TypedBid, 0, len(request.Imp))
   ```
   **Benefit:** Reduces memory allocations when appending bids

2. **ImpIDs tracking field**
   ```go
   // Prebid includes:
   ImpIDs: openrtb_ext.GetImpIDs(request.Imp)

   // We don't set this field
   ```
   **Benefit:** Helps with request tracking and debugging. Check if our framework uses this.

3. **Typed errors**
   ```go
   // Prebid uses:
   &errortypes.BadServerResponse{Message: "..."}

   // We use:
   fmt.Errorf("unexpected status: %d", responseData.StatusCode)
   ```
   **Benefit:** Better error classification, may affect retry logic

---

## Recommendations

### Priority 1: Fix Media Type Detection
Implement Kargo's extension-based media type detection before impression-based fallback.

### Priority 2: Add Bidder Parameters
Create `params.go` with proper parameter structs for documentation and future compatibility.

### Priority 3: Consider Optimizations
- Add bid slice capacity hint
- Check if ImpIDs field is used in our framework
- Evaluate if typed errors would benefit our error handling flow

### Priority 4: Documentation
Document our enhancements (GZIP, ID clearing) and why they differ from Prebid reference.

---

## Testing Recommendations

### Add test cases for:

1. **Multi-format impressions**
   ```go
   // Test impression with both banner and video
   // Verify bid type is determined by Kargo's extension
   ```

2. **GZIP response handling**
   ```go
   // Test compressed response parsing
   // Test non-compressed response parsing
   ```

3. **ID clearing verification**
   ```go
   // Verify site.id, publisher.id are cleared in outgoing request
   ```

4. **Bid extension parsing**
   ```go
   // Test bid.ext.mediaType = "video" → BidTypeVideo
   // Test bid.ext.mediaType = "native" → BidTypeNative
   // Test missing extension → fallback to impression type
   ```

---

## Conclusion

Our Kargo adapter implementation is **functionally enhanced** with GZIP compression, ID privacy, and better error handling. However, we have a **critical difference in media type detection** that needs to be addressed to ensure correct bid categorization, especially for multi-format impressions.

**Action Items:**
1. ✅ Keep GZIP compression (valuable optimization)
2. ✅ Keep ID clearing (good privacy practice)
3. 🔴 **Fix media type detection to use bid.ext.mediaType first**
4. 📝 Add bidder params definition
5. 📝 Add tests for multi-format scenarios
6. ⚡ Apply minor optimizations (capacity hints, ImpIDs)

**Overall Grade:** B+ (Would be A with media type detection fix)
