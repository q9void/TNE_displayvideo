# Error Types Comparison: Prebid Server vs CATALYST

**Date:** 2026-02-14
**Reference:** https://github.com/prebid/prebid-server/tree/master/errortypes

---

## Quick Answer

Both systems have **typed error handling**, but with different philosophies:
- **Prebid Server:** Comprehensive error taxonomy (15 types) with severity levels and error codes
- **CATALYST:** Simpler adapter-focused errors (6 codes) with bidder context

---

## Prebid Server Error Types Structure

### Total: 9 files

```
errortypes/
├── errortypes.go          # 15 error type definitions
├── code.go                # Numeric error codes (999+)
├── code_test.go
├── severity.go            # Fatal vs Warning severity
├── severity_test.go
├── scope.go               # Error scope classification
├── scope_test.go
├── aggregate.go           # Error aggregation
└── aggregate_test.go
```

### Error Type Taxonomy (15 Types)

#### Fatal Severity Errors (12 types)

1. **Timeout** - Bidder failed to respond before PBS timeout
   ```go
   type Timeout struct{ Message string }
   Code: TimeoutErrorCode
   Severity: Fatal
   ```

2. **TmaxTimeout** - Insufficient tmax duration remaining
   ```go
   type TmaxTimeout struct{ Message string }
   Code: TmaxTimeoutErrorCode
   Severity: Fatal
   ```

3. **BadInput** - Malformed client request
   ```go
   type BadInput struct{ Message string }
   Code: BadInputErrorCode
   Severity: Fatal
   ```

4. **BlockedApp** - App ID matches blocked list
   ```go
   type BlockedApp struct{ Message string }
   Code: BlockedAppErrorCode
   Severity: Fatal
   ```

5. **AccountDisabled** - Account is disabled
   ```go
   type AccountDisabled struct{ Message string }
   Code: AccountDisabledErrorCode
   Severity: Fatal
   ```

6. **AcctRequired** - Missing required account ID
   ```go
   type AcctRequired struct{ Message string }
   Code: AcctRequiredErrorCode
   Severity: Fatal
   ```

7. **BadServerResponse** - SSP returned 500 or malformed response
   ```go
   type BadServerResponse struct{ Message string }
   Code: BadServerResponseErrorCode
   Severity: Fatal
   ```

8. **FailedToRequestBids** - Adapter generated no requests
   ```go
   type FailedToRequestBids struct{ Message string }
   Code: FailedToRequestBidsErrorCode
   Severity: Fatal
   ```

9. **MalformedAcct** - Account config unmarshaling failed
   ```go
   type MalformedAcct struct{ Message string }
   Code: MalformedAcctErrorCode
   Severity: Fatal
   ```

10. **FailedToUnmarshal** - JSON unmarshaling error
    ```go
    type FailedToUnmarshal struct{ Message string }
    Code: FailedToUnmarshalErrorCode
    Severity: Fatal
    ```

11. **FailedToMarshal** - JSON marshaling error
    ```go
    type FailedToMarshal struct{ Message string }
    Code: FailedToMarshalErrorCode
    Severity: Fatal
    ```

12. **InvalidImpFirstPartyData** - Invalid impression FPD
    ```go
    type InvalidImpFirstPartyData struct{ Message string }
    Code: InvalidImpFirstPartyDataErrorCode
    Severity: Fatal
    ```

#### Warning Severity Errors (3 types)

13. **Warning** - Generic non-fatal error
    ```go
    type Warning struct{
        Message     string
        WarningCode Code  // Configurable warning code
    }
    Severity: Warning
    ```

14. **BidderTemporarilyDisabled** - Bidder deprecated/disabled
    ```go
    type BidderTemporarilyDisabled struct{ Message string }
    Code: BidderTemporarilyDisabledErrorCode
    Severity: Warning
    ```

15. **BidderThrottled** - Bidder throttled due to high errors
    ```go
    type BidderThrottled struct{ Message string }
    Code: BidderTemporarilyThrottledErrorCode
    Severity: Warning
    ```

---

### Error Codes

**Error Codes (999+):**
```go
const (
    UnknownErrorCode = 999

    TimeoutErrorCode = iota + 1
    BadInputErrorCode
    BlockedAppErrorCode
    BidderTemporarilyDisabledErrorCode
    AccountDisabledErrorCode
    AcctRequiredErrorCode
    BadServerResponseErrorCode
    FailedToRequestBidsErrorCode
    MalformedAcctErrorCode
    FailedToUnmarshalErrorCode
    BidderTemporarilyThrottledErrorCode
    InvalidImpFirstPartyDataErrorCode
)
```

**Warning Codes (10000+):**
```go
const (
    UnknownWarningCode = 10999

    InvalidPrivacyConsentWarningCode = iota + 10000
    AccountLevelDebugDisabledWarningCode
    BidderLevelDebugDisabledWarningCode
    DisabledCurrencyConversionWarningCode
    AlternateBidderCodeWarningCode
    MultiBidWarningCode
    BidAdjustmentWarningCode
    FloorBidRejectionWarningCode
    // ... more warning codes
)
```

---

### Severity Levels

```go
type Severity int

const (
    Fatal Severity = iota  // Request fails
    Warning                // Request succeeds with warnings
)

// All error types implement:
func (e *ErrorType) Severity() Severity {
    return Fatal // or Warning
}
```

---

### Error Aggregation

```go
// Aggregate combines multiple errors
type Aggregate struct {
    errors []error
}

func (a *Aggregate) Add(err error) {
    a.errors = append(a.errors, err)
}

func (a *Aggregate) Error() string {
    // Returns combined error message
}
```

---

## CATALYST Error Types Structure

### Total: 1 file + tests

```
internal/adapters/
├── helpers.go             # BidderError definition
└── helpers_test.go        # Error tests
```

### Error Type Definition

```go
// BidderErrorCode represents error categories
type BidderErrorCode string

const (
    ErrorCodeMarshal    BidderErrorCode = "MARSHAL_ERROR"
    ErrorCodeBadRequest BidderErrorCode = "BAD_REQUEST"
    ErrorCodeBadStatus  BidderErrorCode = "BAD_STATUS"
    ErrorCodeParse      BidderErrorCode = "PARSE_ERROR"
    ErrorCodeTimeout    BidderErrorCode = "TIMEOUT"
    ErrorCodeConnection BidderErrorCode = "CONNECTION_ERROR"
)

// BidderError represents a standardized adapter error
type BidderError struct {
    BidderCode string          // Which bidder
    Code       BidderErrorCode // Error category
    Message    string          // Human-readable message
    Cause      error           // Underlying error (chain)
}

func (e *BidderError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %s (%v)", e.Code, e.BidderCode, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s: %s", e.Code, e.BidderCode, e.Message)
}

func (e *BidderError) Unwrap() error {
    return e.Cause
}
```

### Error Constructors

```go
// Standardized error creation functions
func NewMarshalError(bidderCode string, cause error) *BidderError
func NewBadRequestError(bidderCode string, responseBody string) *BidderError
func NewBadStatusError(bidderCode string, statusCode int) *BidderError
func NewParseError(bidderCode string, cause error) *BidderError
```

---

## Side-by-Side Comparison

| Aspect | Prebid Server | CATALYST | Winner |
|--------|---------------|----------|--------|
| **Total Error Types** | 15 types | 1 type (6 codes) | Prebid |
| **Error Codes** | Numeric (999+) | String codes | Different |
| **Severity Levels** | Fatal + Warning | All fatal | Prebid |
| **Scope Classification** | Yes | No | Prebid |
| **Error Aggregation** | ✅ Built-in | ❌ Manual | Prebid |
| **Bidder Context** | ❌ Not included | ✅ BidderCode field | **CATALYST** |
| **Error Chaining** | ❌ No | ✅ Unwrap() support | **CATALYST** |
| **User/Input Errors** | 3 types | BadRequest | Prebid |
| **Timeout Errors** | 2 types (Timeout, TmaxTimeout) | 1 code (TIMEOUT) | Prebid |
| **Marshal/Unmarshal** | 2 types | 2 codes (Marshal, Parse) | Equal |
| **Response Errors** | BadServerResponse | BadStatus | Equal |
| **Account Errors** | 3 types | ❌ None | Prebid |
| **Privacy Errors** | InvalidPrivacy | ❌ None | Prebid |
| **Throttling** | BidderThrottled | ❌ None | Prebid |
| **Documentation** | ❌ Minimal | ✅ Good comments | **CATALYST** |
| **Test Coverage** | ✅ 4 test files | ✅ 1 test file | Prebid (more) |

---

## What Prebid Server Has That We Don't

### 1. Severity Levels (Fatal vs Warning)

**What they have:**
```go
type Severity int
const (
    Fatal   // Request fails completely
    Warning // Request succeeds with warnings
)
```

**Why they have it:**
- Some errors are recoverable (warnings)
- Auction continues with partial results
- Client sees warnings in response

**Do we need it?** ⚠️ **Maybe**

**What we have:**
- All errors are fatal (adapter fails)
- No warning concept
- No partial success

**When we would need it:**
- If we want to return bids with warnings
- If we implement partial auction success
- If we add non-critical validation warnings

**Implementation effort:** 4-6 hours

---

### 2. Error Scope Classification

**What they have:**
```go
// scope.go
type Scope int
const (
    General Scope = iota  // General error
    Prebid                // Prebid-specific error
    Debug                 // Debug-only error
)
```

**Why they have it:**
- Filter errors by scope
- Hide debug errors in production
- Categorize error sources

**Do we need it?** ❌ **No**

**Why not:**
- Our errors are already scoped (BidderCode field)
- No need for additional categorization
- Simpler is better

---

### 3. Error Aggregation

**What they have:**
```go
// aggregate.go
type Aggregate struct {
    errors []error
}

func (a *Aggregate) Add(err error)
func (a *Aggregate) Error() string
```

**Why they have it:**
- Combine errors from multiple bidders
- Return all errors in single response
- Better debugging

**Do we need it?** ✅ **Yes**

**What we have:**
- Return `[]error` from adapters
- Exchange collects errors manually
- No aggregation helper

**When we would need it:**
- When collecting errors from 10+ bidders
- When returning error summary to client
- When logging all errors together

**Implementation effort:** 2-3 hours

**Recommended:** ✅ Add error aggregation helper

---

### 4. Account-Level Errors

**What they have:**
```go
AccountDisabled     // Account is disabled
AcctRequired        // Missing account ID
MalformedAcct       // Invalid account config
BlockedApp          // App ID blocked
```

**What we have:**
- Generic errors (no account-specific types)
- Account validation in bid handler
- No dedicated error types

**Do we need it?** ⚠️ **Maybe**

**When we would need it:**
- If we add account enable/disable feature
- If we implement app blocking
- If we validate account config at runtime

**Implementation effort:** 3-4 hours

---

### 5. Timeout Differentiation

**What they have:**
```go
Timeout       // Bidder timeout
TmaxTimeout   // Insufficient tmax remaining
```

**What we have:**
```go
ErrorCodeTimeout  // Generic timeout
```

**Do we need it?** ❌ **No**

**Why not:**
- We don't use tmax (OpenRTB timeout field)
- Single timeout concept sufficient
- Less complexity

---

### 6. Privacy Errors

**What they have:**
```go
InvalidImpFirstPartyData      // Invalid FPD
InvalidPrivacyConsentWarning  // GDPR/CCPA issues
```

**What we have:**
- Privacy validation in middleware
- No dedicated error types
- Generic validation errors

**Do we need it?** ❌ **No**

**Why not:**
- Privacy middleware handles validation
- Errors returned before auction
- No need for bidder-level privacy errors

---

### 7. Numeric Error Codes

**What they have:**
```go
const (
    TimeoutErrorCode = 1
    BadInputErrorCode = 2
    // ... numeric codes
)
```

**What we have:**
```go
const (
    ErrorCodeTimeout = "TIMEOUT"
    ErrorCodeBadRequest = "BAD_REQUEST"
    // ... string codes
)
```

**Do we need it?** ❌ **No**

**Why not:**
- String codes are more readable
- No need for numeric mapping
- Self-documenting in logs

**Advantage of strings:** `[TIMEOUT] rubicon: request timeout` vs `[1] rubicon: request timeout`

---

## What We Have That Prebid Server Doesn't

### ✅ Bidder Context in Errors

**What we have:**
```go
type BidderError struct {
    BidderCode string  // Which bidder failed
    Code       BidderErrorCode
    Message    string
    Cause      error
}

// Error message:
// "[TIMEOUT] rubicon: request timeout (context deadline exceeded)"
```

**What they have:**
```go
type Timeout struct {
    Message string  // No bidder context
}

// Error message:
// "bidder timeout: request timeout"
// (Bidder name not in error)
```

**Value:** ✅ **High** - Much better debugging

**Why it's better:**
- Immediately know which bidder failed
- Logs are self-explanatory
- No need to check context separately

---

### ✅ Error Chaining (Unwrap Support)

**What we have:**
```go
func (e *BidderError) Unwrap() error {
    return e.Cause
}

// Usage:
err := NewParseError("rubicon", jsonErr)
errors.Is(err, jsonErr)      // true
errors.Unwrap(err)           // returns jsonErr
```

**What they have:**
- No error chaining
- No Unwrap() support
- Can't trace root cause

**Value:** ✅ **High** - Better error tracing

**Why it's better:**
- Standard Go 1.13+ error wrapping
- Can use errors.Is() and errors.As()
- Root cause preservation

---

### ✅ Response Body in Errors

**What we have:**
```go
func NewBadRequestError(bidderCode string, responseBody string) *BidderError {
    return &BidderError{
        Code:    ErrorCodeBadRequest,
        Message: fmt.Sprintf("bad request: %s", responseBody),
    }
}

// Error message includes response:
// "[BAD_REQUEST] rubicon: bad request: {\"error\":\"invalid accountId\"}"
```

**What they have:**
```go
type BadInput struct {
    Message string  // Generic message only
}
```

**Value:** ✅ **Medium** - Easier debugging

---

### ✅ Standardized Error Constructors

**What we have:**
```go
NewMarshalError(bidderCode, cause)
NewBadRequestError(bidderCode, responseBody)
NewBadStatusError(bidderCode, statusCode)
NewParseError(bidderCode, cause)
```

**What they have:**
- Create errors manually
- No helper functions
- More verbose

**Value:** ✅ **Medium** - Consistent error creation

---

## Comparison Summary

### Philosophy Differences

**Prebid Server:**
- **Comprehensive taxonomy** - 15 error types
- **Severity-based** - Fatal vs Warning
- **Scope-based** - General, Prebid, Debug
- **Numeric codes** - 999, 1000, 10000+
- **Account-centric** - Account errors
- **Warning support** - Partial success

**CATALYST:**
- **Simple taxonomy** - 6 error codes
- **All fatal** - No warnings
- **Bidder-centric** - Bidder context in every error
- **String codes** - Self-documenting
- **Error chaining** - Go 1.13+ Unwrap support
- **Response context** - Includes response bodies

---

### Error Message Comparison

**Prebid Server:**
```
"bidder timeout: request exceeded deadline"
```
- ❌ No bidder name
- ❌ No error code
- ❌ No cause chain

**CATALYST:**
```
"[TIMEOUT] rubicon: request timeout (context deadline exceeded)"
```
- ✅ Error code prefix
- ✅ Bidder name
- ✅ Cause in parentheses

**Winner:** ✅ **CATALYST** (more informative)

---

## Recommendation

### ✅ **Keep Our Current Error Structure + Add Enhancements**

**Why:**

1. **Simpler and More Focused**
   - 6 error codes vs 15 error types
   - All errors are bidder-related
   - No complex taxonomy needed

2. **Better Context**
   - Bidder name in every error
   - Error chaining (Unwrap)
   - Response body inclusion

3. **More Debuggable**
   - Self-documenting string codes
   - Cause chain preservation
   - Informative error messages

4. **Go Idiomatic**
   - Standard error wrapping (Go 1.13+)
   - errors.Is() and errors.As() support
   - No custom interfaces

---

### ⚠️ **Optional: Add Missing Features**

#### 1. Error Aggregation Helper (Recommended)

**Add:** Error aggregation like Prebid Server

```go
// internal/adapters/errors.go
type ErrorList struct {
    errors []error
}

func (e *ErrorList) Add(err error) {
    e.errors = append(e.errors, err)
}

func (e *ErrorList) Errors() []error {
    return e.errors
}

func (e *ErrorList) Error() string {
    if len(e.errors) == 0 {
        return ""
    }
    if len(e.errors) == 1 {
        return e.errors[0].Error()
    }
    return fmt.Sprintf("%d errors occurred: %s", len(e.errors), e.errors[0].Error())
}
```

**Effort:** 2-3 hours

**Value:** ✅ High - Better error collection

---

#### 2. Warning Support (Optional)

**Add:** Non-fatal warnings like Prebid Server

```go
// internal/adapters/helpers.go
type BidderWarning struct {
    BidderCode string
    Message    string
}

func (w *BidderWarning) Error() string {
    return fmt.Sprintf("[WARNING] %s: %s", w.BidderCode, w.Message)
}

func (w *BidderWarning) IsWarning() bool {
    return true
}
```

**Effort:** 4-6 hours

**Value:** ⚠️ Medium - Only if we need partial success

---

#### 3. Account Errors (Optional)

**Add:** Account-specific errors if needed

```go
const (
    ErrorCodeAccountDisabled BidderErrorCode = "ACCOUNT_DISABLED"
    ErrorCodeAccountInvalid  BidderErrorCode = "ACCOUNT_INVALID"
)

func NewAccountDisabledError(accountID string) *BidderError {
    return &BidderError{
        Code:    ErrorCodeAccountDisabled,
        Message: fmt.Sprintf("account disabled: %s", accountID),
    }
}
```

**Effort:** 2-3 hours

**Value:** ⏸️ Low - Add if we implement account enable/disable

---

## Summary Table

| Feature | Prebid Server | CATALYST | Winner |
|---------|---------------|----------|--------|
| **Error Types** | 15 types | 1 type (6 codes) | Prebid (more granular) |
| **Bidder Context** | ❌ No | ✅ BidderCode field | **CATALYST** |
| **Error Chaining** | ❌ No | ✅ Unwrap() | **CATALYST** |
| **Severity Levels** | ✅ Fatal + Warning | ❌ All fatal | Prebid |
| **Error Codes** | Numeric | String | **CATALYST** (readable) |
| **Error Aggregation** | ✅ Built-in | ❌ Manual | Prebid |
| **Account Errors** | ✅ 3 types | ❌ None | Prebid |
| **Timeout Types** | ✅ 2 types | ❌ 1 code | Prebid |
| **Privacy Errors** | ✅ Yes | ❌ No | Prebid |
| **Response Context** | ❌ No | ✅ Includes body | **CATALYST** |
| **Constructors** | ❌ Manual | ✅ Helpers | **CATALYST** |
| **Error Messages** | Generic | Detailed | **CATALYST** |

---

## Conclusion

**Architecture Verdict:**

| Aspect | Winner | Reason |
|--------|--------|--------|
| **Taxonomy** | Prebid | More error types (15 vs 6) |
| **Context** | **CATALYST** | Bidder name + error chaining |
| **Simplicity** | **CATALYST** | 6 codes vs 15 types |
| **Debuggability** | **CATALYST** | Better error messages |
| **Features** | Prebid | Severity, scope, aggregation |

**Bottom Line:**
Prebid Server has a **more comprehensive error taxonomy** (15 types, severity levels, scope classification), but our **simpler approach with better context** (bidder name, error chaining, response bodies) is **more practical and debuggable** for our use case.

**Key Takeaway:**
We're not missing critical error types. Our 6 error codes cover the essential cases. The main feature worth adding is **error aggregation** (2-3 hours) for better error collection. Everything else is optional.

**Recommended Actions:**
1. ✅ Add error aggregation helper (2-3 hours) - **High value**
2. ⏸️ Add warning support (4-6 hours) - Only if we need partial success
3. ⏸️ Add account errors (2-3 hours) - Only if we add account management features

Our error handling is **simpler, more debuggable, and sufficient** for our current needs. 🎯
