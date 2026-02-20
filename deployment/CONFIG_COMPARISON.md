# Config Structure: CATALYST vs Prebid Server

**Date:** 2026-02-14
**Reference:** https://github.com/prebid/prebid-server/tree/master/config

---

## Quick Answer

We're **missing several config files** from Prebid Server, but **most are not critical** for our current use case.

---

## What Prebid Server Has

### Essential Files (/config folder)

| File | Purpose | Do We Have? | Do We Need? |
|------|---------|-------------|-------------|
| **config.go** | Core configuration management | ✅ Yes (cmd/server/config.go) | ✅ Yes |
| **account.go** | Account-level settings | ❌ No | ⚠️ Maybe |
| **adapter.go** | Bidder adapter configurations | ❌ No | ❌ No |
| **stored_requests.go** | Stored request handling | ❌ No | ❌ No |
| **bidderinfo.go** | Bidder information/capabilities | ✅ Partial (in adapters) | ✅ Yes |
| **usersync.go** | User sync configuration | ✅ Partial (in code) | ✅ Yes |
| **events.go** | Event tracking config | ❌ No | ⚠️ Maybe |
| **compression.go** | Response compression | ❌ No | ✅ Have (constants.go) |
| **hooks.go** | Module hook configuration | ❌ No | ❌ No |
| **activity.go** | Activity-based access controls | ❌ No | ❌ No |
| **experiment.go** | A/B testing framework | ❌ No | ❌ No |
| **requestvalidation.go** | Request validation rules | ❌ No | ⚠️ Maybe |
| **interstitial.go** | Interstitial ad handling | ❌ No | ❌ No |

---

## What We Have

### CATALYST Config Structure

```
tnevideo/
├── cmd/server/
│   ├── config.go          # Main configuration (450 lines)
│   └── config_test.go     # Config tests
└── internal/config/
    └── constants.go       # Shared constants (105 lines)
```

### Our Config Files

#### 1. `cmd/server/config.go` (450 lines)

**What it includes:**
```go
type ServerConfig struct {
    // Server
    Port    string
    Timeout time.Duration

    // Database
    DatabaseConfig *DatabaseConfig

    // Redis
    RedisURL string

    // IDR
    IDREnabled bool
    IDRUrl     string
    IDRAPIKey  string

    // Currency
    CurrencyConversionEnabled bool
    DefaultCurrency           string

    // Privacy
    DisableGDPREnforcement bool

    // Cookie Sync
    HostURL string

    // CORS
    CORSOrigins []string
}
```

**Features:**
- ✅ Environment variable parsing
- ✅ Flag support
- ✅ Database configuration
- ✅ Redis configuration
- ✅ IDR integration
- ✅ Currency conversion
- ✅ Cookie sync settings
- ✅ CORS configuration
- ✅ Comprehensive validation
- ✅ Production security checks

#### 2. `internal/config/constants.go` (105 lines)

**What it includes:**
```go
// Server timeouts
ServerReadTimeout  = 5s
ServerWriteTimeout = 10s
ServerIdleTimeout  = 120s
ShutdownTimeout    = 30s

// CORS
CORSMaxAge = 86400

// Rate limiting
DefaultRPS = 1000
DefaultBurstSize = 100
DefaultPublisherRPS = 100

// Size limits
DefaultMaxBodySize = 1MB
DefaultMaxURLLength = 8KB

// Gzip compression
GzipMinLength = 256 bytes

// IDR client
IDRDefaultTimeout = 150ms
IDRMaxResponseSize = 1MB

// Exchange
DefaultAuctionTimeout = 1000ms
DefaultMaxBidders = 50

// Cookie
MaxCookieSize = 4KB

// Security
HSTSMaxAgeSeconds = 1 year
```

---

## Missing Config Files Analysis

### 🔴 **account.go** - Account-Level Settings

**What Prebid Has:**
```go
type Account struct {
    ID                string
    Disabled          bool
    CacheTTL          int
    EventsEnabled     bool
    GDPR              *AccountGDPR
    CCPA              *AccountCCPA
    AnalyticsConfig   *AccountAnalytics
    BidderControls    map[string]*AccountBidderControl
    PriceGranularity  *PriceGranularity
}
```

**Do we need it?** ⚠️ **Maybe**

**Why we might need it:**
- Per-account feature flags
- Per-account bidder controls
- Per-account analytics settings
- Per-account GDPR/CCPA settings

**Why we don't:**
- We have account-level bidder configs in database (`slot_bidder_configs`)
- Feature flags can be environment variables
- GDPR enforcement is global (DisableGDPREnforcement flag)

**Recommendation:**
- Add if we need per-account configuration
- Currently database handles account-specific settings

---

### 🟢 **adapter.go** - Bidder Adapter Configurations

**What Prebid Has:**
```go
type Adapter struct {
    Endpoint         string
    ExtraAdapterInfo string
    XAPI             *AdapterXAPI
    UserSyncURL      string
    PlatformID       string
    AppSecret        string
}
```

**Do we need it?** ❌ **No**

**Why not:**
- We configure adapters in code (each adapter's New() function)
- Endpoint URLs hardcoded in adapters
- Credentials from environment variables (RUBICON_XAPI_USER, etc.)
- More flexible than YAML config

**What we do instead:**
```go
// Each adapter configures itself
func New(endpoint string) *Adapter {
    if endpoint == "" {
        endpoint = defaultEndpoint  // Hardcoded
    }
    xapiUser := os.Getenv("RUBICON_XAPI_USER")  // From env
    xapiPass := os.Getenv("RUBICON_XAPI_PASS")
    return &Adapter{endpoint, xapiUser, xapiPass}
}
```

---

### 🔴 **stored_requests.go** - Stored Request Handling

**What Prebid Has:**
```go
type StoredRequests struct {
    Files            bool
    Postgres         *PostgresConfig
    HTTP             *HTTPConfig
    CacheEvents      *CacheEventsConfig
    InMemoryCache    *InMemoryCache
}
```

**Do we need it?** ❌ **No**

**Why not:**
- We don't support Prebid Mobile SDK (which uses stored requests)
- Our SDK sends full bid requests
- No need for request templates

**Note:** Prebid Server's stored requests are for:
- Mobile SDK optimization (reduce request size)
- Pre-storing bid request templates
- AMP page support

---

### 🟡 **bidderinfo.go** - Bidder Information/Capabilities

**What Prebid Has:**
```go
type BidderInfo struct {
    Endpoint         string
    Maintainer       *MaintainerInfo
    Capabilities     *CapabilitiesInfo
    Syncer           *SyncerInfo
    ModifyingVastXML bool
}
```

**Do we have it?** ✅ **Partial**

**Where we have it:**
```go
// internal/adapters/adapter.go
type BidderInfo struct {
    Enabled                 bool
    Maintainer              *MaintainerInfo
    Capabilities            *CapabilitiesInfo  // ✅ Have this
    ModifyingVastXmlAllowed bool
    GVLVendorID             int
    Syncer                  *SyncerInfo
    Endpoint                string
    DemandType              DemandType
}

// Each adapter implements Info()
func Info() adapters.BidderInfo { ... }
```

**Do we need separate config file?** ❌ **No**

**Why not:**
- Info() functions work well
- Compiled into binary (faster than YAML parsing)
- Type-safe (catches errors at compile time)

---

### 🟡 **usersync.go** - User Sync Configuration

**What Prebid Has:**
```go
type UserSync struct {
    ExternalURL    string
    RedirectURL    string
    CooperativeSync *CooperativeSync
    PriorityGroups  [][]string
}
```

**Do we have it?** ✅ **Partial**

**Where we have it:**
```go
// cmd/server/config.go
type ServerConfig struct {
    HostURL string  // Used for cookie sync redirect URLs
}

// Each adapter's Info() has:
Syncer: &adapters.SyncerInfo{
    URL: "https://...",
    Type: "redirect",
}
```

**Do we need separate config file?** ❌ **No**

**Why not:**
- Simple redirect-based syncs
- No cooperative sync needed
- No priority groups needed
- HostURL in main config is sufficient

---

### 🟡 **events.go** - Event Tracking Configuration

**What Prebid Has:**
```go
type Events struct {
    Enabled     bool
    VASTEnabled bool
    TimeoutMS   int
}
```

**Do we have it?** ✅ **Partial**

**Where we have it:**
```go
// exchange.Config has:
EventRecordEnabled: true
EventBufferSize:    100
```

**Do we need separate config file?** ⚠️ **Maybe**

**Recommendation:**
- Add if we need more event configuration
- Currently minimal event tracking
- Could add: event endpoint URL, timeout, batch size, etc.

---

### 🟢 **compression.go** - Response Compression Settings

**What Prebid Has:**
```go
type Compression struct {
    GZIP *CompressionGZIP
}

type CompressionGZIP struct {
    Enabled        bool
    ResponseFormat []string
}
```

**Do we have it?** ✅ **Yes**

**Where we have it:**
```go
// internal/config/constants.go
const GzipMinLength = 256  // Minimum size to compress
```

**Do we need separate config file?** ❌ **No**

**Why not:**
- GZIP compression is simple
- Single constant is sufficient
- Middleware handles compression automatically

---

### 🔴 **hooks.go** - Module Hook Configuration

**What Prebid Has:**
```go
type Hooks struct {
    Enabled bool
    Modules map[string]HookExecutionPlan
}
```

**Do we need it?** ❌ **No**

**Why not:**
- We don't have a plugin/module system
- We control all code directly
- No need for hook injection points

---

### 🔴 **activity.go** - Activity-Based Access Controls

**What Prebid Has:**
```go
type Activity struct {
    SyncUser      *ActivityControl
    FetchBids     *ActivityControl
    EnrichUFPD    *ActivityControl
    ReportAnalytics *ActivityControl
}
```

**Do we need it?** ❌ **No**

**Why not:**
- We don't have fine-grained activity controls
- Access control is at account level (database)
- Not a shared platform with permission system

---

### 🔴 **experiment.go** - A/B Testing Framework

**What Prebid Has:**
```go
type Experiments struct {
    AdsCert *AdsCert
}
```

**Do we need it?** ❌ **No**

**Why not:**
- No A/B testing framework currently
- Can add later if needed
- Would need separate experimentation infrastructure

---

### 🟡 **requestvalidation.go** - Request Validation Rules

**What Prebid Has:**
```go
type RequestValidation struct {
    BannerSizeValidation bool
    MaxNumberOfValues    int
    IpValidation         *IpValidation
}
```

**Do we need it?** ⚠️ **Maybe**

**Where we have it:**
- Validation in bid handler (catalyst_bid_handler.go)
- Adapter-level validation (each adapter validates)

**Recommendation:**
- Add if we need configurable validation rules
- Currently validation is hardcoded (which is fine)

---

### 🔴 **interstitial.go** - Interstitial Ad Handling

**What Prebid Has:**
```go
type Interstitial struct {
    MaxPercent int
    MinHeight  int
    MinWidth   int
}
```

**Do we need it?** ❌ **No**

**Why not:**
- We don't serve interstitial ads
- Banner/video/native only

---

## What We Should Add

### Priority 1: High Value ✅

**None currently needed**

All critical functionality is covered by our existing config structure.

### Priority 2: Nice to Have ⚠️

#### 1. Account-Level Configuration (account.go)

**Add if:**
- Need per-account feature flags
- Need per-account bidder controls
- Need per-account analytics settings

**Example implementation:**
```go
// internal/config/account.go
type AccountConfig struct {
    ID                    string
    Disabled              bool
    MaxBidders            int
    DefaultTimeout        time.Duration
    BidderBlacklist       []string
    EnabledMediaTypes     []string
    AnalyticsEnabled      bool
    GDPREnforcement       bool
}

func LoadAccountConfig(accountID string) (*AccountConfig, error) {
    // Load from database or cache
}
```

#### 2. Event Tracking Configuration (events.go)

**Add if:**
- Need configurable event endpoints
- Need event batching configuration
- Need VAST event tracking

**Example implementation:**
```go
// internal/config/events.go
type EventsConfig struct {
    Enabled          bool
    Endpoint         string
    BatchSize        int
    FlushInterval    time.Duration
    VASTEnabled      bool
    WinEventEnabled  bool
}
```

#### 3. Request Validation Configuration (requestvalidation.go)

**Add if:**
- Need configurable validation rules
- Need to toggle specific validations
- Need different validation per environment

**Example implementation:**
```go
// internal/config/validation.go
type ValidationConfig struct {
    StrictMode           bool
    MaxImpressions       int
    MaxBidders           int
    RequireDeviceInfo    bool
    RequireGeoInfo       bool
    AllowedMediaTypes    []string
}
```

---

## Configuration Philosophy Comparison

### Prebid Server Approach

**Philosophy:** Maximum flexibility, support every use case
- YAML config files for everything
- Runtime configuration changes
- Per-account, per-bidder, per-feature granularity
- Supports diverse deployments

**Pros:**
- ✅ Very flexible
- ✅ Easy to customize without code changes
- ✅ Supports multi-tenant scenarios

**Cons:**
- ❌ Complex configuration surface
- ❌ Runtime parsing overhead
- ❌ Type safety only at startup
- ❌ Configuration validation complexity

---

### CATALYST Approach

**Philosophy:** Simplicity, compile-time safety, environment-based
- Go structs for configuration
- Environment variables for deployment-specific settings
- Hardcoded constants for stable values
- Database for dynamic data (bidder params, etc.)

**Pros:**
- ✅ Type-safe (compile-time checking)
- ✅ Fast (no YAML parsing)
- ✅ Simple (less configuration surface)
- ✅ Clear defaults

**Cons:**
- ❌ Less flexible (need code changes for some configs)
- ❌ Can't change constants at runtime
- ❌ Environment variables can get messy

---

## Recommendation

### ✅ **Keep Current Structure**

**Why:**

1. **Sufficient for Current Needs**
   - We have everything we need
   - Config is simple and maintainable
   - Type-safe and fast

2. **Add Features When Needed**
   - Account-level config if we need per-account settings
   - Event config if we expand event tracking
   - Validation config if we need togglable rules

3. **Environment Variables Work Well**
   - Easy to deploy (Docker, systemd, etc.)
   - Secure (secrets stay in env, not files)
   - Standard practice

### ⚠️ **Potential Additions**

If we grow, consider adding:

1. **Account Configuration** (account.go)
   ```go
   // When: Need per-account settings
   // Effort: 1-2 days
   // Value: High for multi-tenant scenarios
   ```

2. **Enhanced Event Config** (events.go)
   ```go
   // When: Expand event tracking
   // Effort: 4-6 hours
   // Value: Medium
   ```

3. **Validation Config** (validation.go)
   ```go
   // When: Need environment-specific validation
   // Effort: 4-6 hours
   // Value: Low-Medium
   ```

---

## Summary

| Config File | Prebid Has | We Have | Priority |
|-------------|------------|---------|----------|
| **config.go** | ✅ | ✅ | ✅ Essential |
| **constants.go** | ❌ | ✅ | ✅ Essential |
| **account.go** | ✅ | ❌ | ⚠️ Nice to have |
| **adapter.go** | ✅ | ❌ (in code) | ✅ Have equivalent |
| **bidderinfo.go** | ✅ | ✅ (in adapters) | ✅ Have equivalent |
| **usersync.go** | ✅ | ✅ (in adapters) | ✅ Have equivalent |
| **events.go** | ✅ | ⚠️ Partial | ⚠️ Nice to have |
| **compression.go** | ✅ | ✅ | ✅ Have equivalent |
| **stored_requests.go** | ✅ | ❌ | ❌ Don't need |
| **hooks.go** | ✅ | ❌ | ❌ Don't need |
| **activity.go** | ✅ | ❌ | ❌ Don't need |
| **experiment.go** | ✅ | ❌ | ❌ Don't need |
| **requestvalidation.go** | ✅ | ❌ (in code) | ⚠️ Nice to have |
| **interstitial.go** | ✅ | ❌ | ❌ Don't need |

**Bottom Line:** We're not missing anything **crucial**. Our configuration is **simpler but sufficient**. Add account-level config and enhanced event tracking if we need them later. 🎯
