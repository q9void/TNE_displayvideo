# InfoAwareBidder Analysis: Do We Need It?

**Date:** 2026-02-14
**Reference:** Prebid Server `adapters/bidder.go` - InfoAwareBidder wrapper

---

## Quick Answer

**No, we're NOT using InfoAwareBidder**, and we **probably don't need it** for our current use case.

---

## What Is InfoAwareBidder?

### Purpose
InfoAwareBidder is a **validation wrapper** that sits between the auction logic and individual bidder adapters. It enforces capabilities defined in YAML config files.

### What It Does

```go
type InfoAwareBidder struct {
    Bidder  // Wrapped adapter
    info parsedBidderInfo  // Capabilities from YAML
}
```

**Validation Steps:**

1. **Platform Validation**
   - Rejects Site requests if bidder doesn't support Site
   - Rejects App requests if bidder doesn't support App
   - Rejects DOOH requests if bidder doesn't support DOOH

2. **Media Type Pruning**
   - Removes Banner from imp if bidder doesn't support banner
   - Removes Video from imp if bidder doesn't support video
   - Removes Audio from imp if bidder doesn't support audio
   - Removes Native from imp if bidder doesn't support native

3. **Multi-Format Handling**
   - For bidders that don't support multi-format impressions
   - Reduces impression to single format based on `preferredMediaType`
   - Example: `imp` has Banner+Video → keep only Banner if preferred

4. **Impression Removal**
   - Removes impressions with no valid media types left
   - Returns error if all impressions removed

---

## Example Validation Flow

### Input Request
```json
{
  "imp": [
    {
      "id": "imp-1",
      "banner": {...},
      "video": {...}    // Multi-format impression
    },
    {
      "id": "imp-2",
      "audio": {...}    // Audio-only impression
    }
  ],
  "site": {...}
}
```

### Bidder Capabilities (from YAML)
```yaml
capabilities:
  site:
    mediaTypes: [banner, video]  # Supports banner and video only
  app:
    mediaTypes: []               # Doesn't support app
openrtb:
  multiformat-supported: false    # Doesn't support multi-format
```

### InfoAwareBidder Processing

1. **Check Platform**: ✅ Site request allowed
2. **Check Media Types**:
   - imp-1: Has banner ✅ and video ✅ → Keep both
   - imp-2: Has audio ❌ → Remove audio, imp has no media types → **Remove imp-2**
3. **Check Multi-Format**: Bidder doesn't support multi-format
   - imp-1: Has banner+video → **Reduce to banner** (preferred type)
4. **Final Request**:
   ```json
   {
     "imp": [
       {
         "id": "imp-1",
         "banner": {...}  // Video removed
       }
       // imp-2 removed entirely
     ]
   }
   ```

---

## Do We Have This?

### What We Have

**File:** `internal/adapters/adapter.go`

```go
// We have the structures
type BidderInfo struct {
    Enabled      bool
    Capabilities *CapabilitiesInfo
    // ...
}

type CapabilitiesInfo struct {
    App  *PlatformInfo
    Site *PlatformInfo
}

type PlatformInfo struct {
    MediaTypes []BidType
}
```

**Each adapter implements Info():**
```go
func Info() adapters.BidderInfo {
    return adapters.BidderInfo{
        Enabled: true,
        Capabilities: &adapters.CapabilitiesInfo{
            Site: &adapters.PlatformInfo{
                MediaTypes: []adapters.BidType{
                    adapters.BidTypeBanner,
                    adapters.BidTypeVideo,
                    adapters.BidTypeNative,
                },
            },
        },
    }
}
```

### What We DON'T Have

❌ **No InfoAwareBidder wrapper**
- No validation before calling adapter.MakeRequests()
- No platform checking (Site vs App)
- No media type pruning
- No multi-format handling
- No impression removal

**Current Flow:**
```
Bid Request → Exchange → Adapter.MakeRequests() → SSP
              (no validation layer)
```

**Prebid's Flow:**
```
Bid Request → Exchange → InfoAwareBidder.MakeRequests() → Validation → Adapter.MakeRequests() → SSP
                         (validates capabilities)
```

---

## Do We Need InfoAwareBidder?

### Arguments FOR Adding It

1. **Safety** ✅
   - Prevents sending invalid requests to SSPs
   - Catches configuration mistakes early
   - Reduces 400 Bad Request errors

2. **Consistency** ✅
   - Centralizes validation logic
   - Every adapter gets same validation
   - Less duplication in adapter code

3. **Multi-Format Support** ✅
   - Handles multi-format impressions correctly
   - Respects adapter limitations
   - Reduces bid response parsing complexity

4. **Clear Contracts** ✅
   - Info() declares what adapter supports
   - InfoAwareBidder enforces the contract
   - Easier to onboard new adapters

### Arguments AGAINST Adding It

1. **We Control Everything** ❌
   - We're not a shared platform with third-party adapters
   - We know what each adapter supports
   - We configure the bidder params directly

2. **Limited Use Cases** ❌
   - We only use Site traffic (no App, no DOOH)
   - Most adapters support same media types
   - Multi-format not widely used yet

3. **Adapters Already Validate** ❌
   - TripleLift checks for Banner or Native
   - Sovrn validates video params
   - Adapters return errors if media type unsupported

4. **Implementation Cost** ❌
   - Need to wrap all 26 adapters
   - Need to test validation for each
   - Need to handle edge cases

5. **Performance Overhead** ❌
   - Extra validation on every request
   - Impression copying and modification
   - May not be necessary for our volume

---

## Current Validation Patterns

### Example: TripleLift (We Already Validate)

```go
func (a *Adapter) MakeRequests(...) {
    // ... parameter extraction ...

    // Validate impression has Banner or Native
    if imp.Banner == nil && imp.Native == nil {
        errs = append(errs, fmt.Errorf("imp %s must have Banner or Native", imp.ID))
        continue  // Skip this impression
    }

    // ... rest of processing ...
}
```

### Example: Kargo (We Already Handle Media Types)

```go
func getMediaTypeForBid(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) adapters.BidType {
    // Check bid.ext.mediaType (video, native, banner)
    // Fallback to impression type
}
```

---

## Recommendation

### ✅ **Don't Add InfoAwareBidder (Yet)**

**Why:**

1. **Not Needed for Current Scale**
   - We're serving our own publishers (not a platform)
   - We control all bidder configurations
   - We only use Site traffic (not App/DOOH)

2. **Adapters Already Validate**
   - TripleLift checks media types
   - Sovrn validates video params
   - Each adapter handles its requirements

3. **Implementation Overhead**
   - Would need to wrap 26 adapters
   - Testing burden
   - Marginal benefit for our use case

4. **We Can Add Later If Needed**
   - If we start supporting App traffic
   - If we onboard third-party adapters
   - If multi-format becomes important

---

## When We WOULD Need It

### Trigger Conditions

1. **Supporting App Traffic** 🔔
   - If we start supporting in-app ads
   - Some adapters support Site only, some App only
   - Need platform validation

2. **Third-Party Adapter Integration** 🔔
   - If others build adapters for our platform
   - Can't trust they'll validate correctly
   - Need enforcement layer

3. **Multi-Format Impressions** 🔔
   - If publishers send Banner+Video impressions
   - Some adapters don't support multi-format
   - Need automatic reduction to single format

4. **DOOH Support** 🔔
   - Digital out-of-home advertising
   - Different adapter support matrix
   - Need platform validation

5. **Reducing SSP 400 Errors** 🔔
   - If we're getting frequent 400 Bad Request errors
   - InfoAwareBidder would catch before sending
   - Improves SSP relationships

---

## Alternative: Lightweight Validation

If we want some validation without full InfoAwareBidder:

### Option 1: Exchange-Level Validation

```go
// In exchange.go before calling adapter
func (e *Exchange) validateRequest(bidder Bidder, req *openrtb.BidRequest) error {
    info := bidder.Info()

    // Check platform
    if req.Site != nil && !info.Capabilities.Site.Enabled {
        return fmt.Errorf("bidder doesn't support site traffic")
    }
    if req.App != nil && !info.Capabilities.App.Enabled {
        return fmt.Errorf("bidder doesn't support app traffic")
    }

    return nil
}
```

### Option 2: Adapter Registration Validation

```go
// When registering adapter, validate Info() makes sense
func RegisterAdapter(code string, adapter Bidder, info BidderInfo) error {
    if info.Capabilities == nil {
        return fmt.Errorf("adapter %s must declare capabilities", code)
    }
    if info.Capabilities.Site == nil && info.Capabilities.App == nil {
        return fmt.Errorf("adapter %s must support at least Site or App", code)
    }
    // Register...
}
```

### Option 3: Config-Time Validation

```go
// When loading bidder configs from database, validate against capabilities
func ValidateBidderConfig(bidderCode string, params map[string]interface{}) error {
    info := adapters.GetInfo(bidderCode)

    // Check if bidder is enabled
    if !info.Enabled {
        return fmt.Errorf("bidder %s is disabled", bidderCode)
    }

    // Validate params against schema
    return ValidateParams(bidderCode, params)
}
```

---

## Implementation Effort

### If We Did Add InfoAwareBidder

**Effort:** 2-3 days

**Steps:**
1. Create InfoAwareBidder wrapper (2-3 hours)
2. Add multi-format preference config (2 hours)
3. Wrap all 26 adapters (4-6 hours)
4. Write tests for validation (4-6 hours)
5. Handle edge cases (4 hours)

**Testing needed:**
- Platform validation (Site/App/DOOH)
- Media type pruning
- Multi-format reduction
- Impression removal
- Error messaging

---

## Comparison Table

| Aspect | Prebid Server | CATALYST Current | CATALYST with InfoAware |
|--------|---------------|------------------|-------------------------|
| **Platform Validation** | ✅ Site/App/DOOH | ❌ None | ✅ Site/App/DOOH |
| **Media Type Pruning** | ✅ Automatic | ❌ Manual in adapters | ✅ Automatic |
| **Multi-Format Handling** | ✅ Automatic | ❌ Not supported | ✅ Automatic |
| **Impression Removal** | ✅ If no valid media | ❌ Adapter decides | ✅ If no valid media |
| **Validation Location** | Before adapter | In adapter | Before adapter |
| **Error Messages** | Standardized warnings | Adapter-specific | Standardized warnings |
| **Complexity** | Higher | Lower | Higher |
| **Performance** | Slight overhead | No overhead | Slight overhead |

---

## Decision Matrix

| Scenario | Need InfoAwareBidder? | Reason |
|----------|----------------------|--------|
| **Only Site traffic** | ❌ No | No platform validation needed |
| **Only banner/video** | ❌ No | Most adapters support both |
| **Single-format imps** | ❌ No | No multi-format handling needed |
| **Controlled adapters** | ❌ No | We validate in adapter code |
| **Third-party adapters** | ✅ Yes | Can't trust their validation |
| **App + Site traffic** | ✅ Yes | Need platform validation |
| **Multi-format imps** | ✅ Yes | Need format reduction |
| **High 400 error rate** | ✅ Yes | Pre-validate before SSP |

---

## Summary

| Question | Answer |
|----------|--------|
| **Do we use InfoAwareBidder?** | ❌ No |
| **Do we have the structures?** | ✅ Yes (BidderInfo, Capabilities) |
| **Do we need it now?** | ❌ No |
| **Should we add it?** | ❌ Not yet |
| **When would we need it?** | When supporting App traffic or third-party adapters |

**Current Status:** ✅ **Our adapter validation is sufficient**

**Recommendation:** ⏸️ **Wait until we have a clear need**
- Monitor for 400 errors from SSPs
- Re-evaluate if adding App support
- Re-evaluate if onboarding third-party adapters

**Bottom Line:** InfoAwareBidder is great for a shared platform with diverse adapters and traffic types, but **overkill for our current controlled environment**. Our in-adapter validation works fine. 🎯
