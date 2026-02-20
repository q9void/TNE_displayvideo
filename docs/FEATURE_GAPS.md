# Prebid Server Feature Gaps

Implementation roadmap for missing and partial Prebid Server features.

**Last Updated:** 2026-02-12

## Executive Summary

This document outlines the features that are either:
- ❌ **Not Implemented** - Feature is completely missing
- ⚠️ **Partially Implemented** - Feature exists but is incomplete

The gaps are prioritized based on:
- **Business Impact** - Revenue, compliance, or competitive advantage
- **Adoption Rate** - How common the feature is in the industry
- **Implementation Effort** - Development complexity and time

## Gap Analysis

### Total Gaps: 12 Features
- **High Priority**: 4 features (33%)
- **Medium Priority**: 5 features (42%)
- **Low Priority**: 3 features (25%)

---

## 🔴 HIGH PRIORITY GAPS

### 1. GPP (Global Privacy Platform) Framework

**Status:** ⚠️ Partial (GDPR/CCPA exist separately)

**Current State:**
- GDPR enforcement implemented (TCF v2, all Purpose IDs)
- CCPA enforcement implemented (US Privacy string)
- COPPA, LGPD, PIPEDA, PDPA also implemented
- **Missing:** Unified GPP string parsing and enforcement

**Why It Matters:**
- IAB's modern privacy framework (replaces separate GDPR/CCPA)
- Industry is migrating to GPP as the standard
- Better multi-jurisdiction privacy handling
- Required for modern SSPs/DSPs

**What to Implement:**
- GPP string parsing (IAB GPP specification)
- GPP consent enforcement framework
- Integration with existing GDPR/CCPA logic
- GPP-aware activity controls
- Configuration for GPP sections (US, EU, etc.)

**Files to Modify:**
- `internal/middleware/privacy.go` - Add GPP parsing
- `internal/privacy/gpp.go` - New GPP-specific logic
- `cmd/server/config.go` - Add GPP configuration

**Effort:** High (3-4 weeks)
**Business Impact:** High (compliance, future-proofing)

**Resources:**
- IAB GPP Specification: https://github.com/InteractiveAdvertisingBureau/Global-Privacy-Platform
- GPP String Encoder/Decoder libraries

---

### 2. Multibid Support

**Status:** ❌ Not Implemented

**Current State:**
- Bidders can only return one bid per impression
- Single creative per bidder limitation

**Why It Matters:**
- Common feature in modern Prebid Server deployments
- Allows bidders to submit multiple creatives at different price points
- Increases competition and potential revenue
- Better fill rates with fallback creatives

**What to Implement:**
- Bid response handling for multiple bids from same bidder
- Targeting key generation for multiple bids (`hb_pb_2`, `hb_bidder_2`, etc.)
- Configuration for max bids per bidder (default: 1-3)
- Bid deduplication logic
- Analytics updates for multibid scenarios

**Files to Modify:**
- `internal/exchange/exchange.go` - Bid response handling
- `internal/exchange/targeting.go` - Multi-bid targeting keys
- `internal/openrtb/models.go` - Multibid extensions
- `cmd/server/config.go` - Multibid configuration

**Effort:** Medium (2-3 weeks)
**Business Impact:** High (revenue optimization)

**Resources:**
- Prebid Multibid Documentation: https://docs.prebid.org/prebid-server/features/pbs-multibid.html

---

### 3. Activity Controls Framework

**Status:** ⚠️ Partial (privacy enforcement exists, but not full framework)

**Current State:**
- Privacy enforcement implemented
- Purpose-based consent checking
- **Missing:** Full IAB Activity Controls framework with GPP integration

**Why It Matters:**
- Granular control over privacy-impacting activities
- GPP-aware activity rules
- Publisher control over data usage
- Better compliance with evolving privacy regulations

**What to Implement:**
- Activity control rules engine
- GPP-aware activity enforcement
- Per-activity consent checking (sync, enrichment, transmit, etc.)
- Publisher-configurable activity rules
- Default activity controls

**Activities to Support:**
- `syncUser` - User syncing
- `transmitUfpd` - User first-party data transmission
- `transmitPreciseGeo` - Precise geolocation transmission
- `transmitEids` - Extended ID transmission
- `transmitTids` - Transaction ID transmission
- `enrichUfpd` - User data enrichment

**Files to Modify:**
- `internal/middleware/privacy.go` - Activity controls integration
- `internal/privacy/activities.go` - New activity framework
- `internal/fpd/processor.go` - FPD activity checks
- `internal/usersync/` - Sync activity checks
- `cmd/server/config.go` - Activity control configuration

**Effort:** High (3-4 weeks)
**Business Impact:** High (compliance, publisher control)

**Resources:**
- IAB Activity Controls: https://docs.prebid.org/prebid-server/features/pbs-activitycontrols.html

---

### 4. DSA (Digital Services Act) Compliance

**Status:** ❌ Not Implemented

**Current State:**
- No DSA passthrough or initialization
- Missing EU transparency requirements

**Why It Matters:**
- EU regulatory requirement (effective 2024+)
- Required for EU publishers and advertisers
- Transparency in automated decision-making
- Competitive requirement for EU market

**What to Implement:**
- DSA object passthrough in OpenRTB requests
- DSA initialization from publisher config
- DSA fields in bid responses
- DSA transparency requirements (who paid, on whose behalf)
- Configuration for DSA defaults

**DSA Fields:**
- `dsarequired` - DSA transparency required
- `pubrender` - Publisher will render DSA transparency
- `datatopub` - DSA data required levels (0-3)
- `transparency` - Advertiser transparency data

**Files to Modify:**
- `internal/openrtb/models.go` - Add DSA objects
- `internal/exchange/exchange.go` - DSA passthrough
- `internal/fpd/processor.go` - DSA initialization
- `cmd/server/config.go` - DSA configuration

**Effort:** Medium (2-3 weeks)
**Business Impact:** High (EU compliance)

**Resources:**
- Prebid DSA Documentation: https://docs.prebid.org/prebid-server/features/pbs-dsa.html
- EU DSA Regulation

---

## 🟡 MEDIUM PRIORITY GAPS

### 5. Prebid Cache Integration

**Status:** ⚠️ Limited (VAST caching only)

**Current State:**
- VAST response caching implemented
- No dedicated Prebid cache service integration
- No bid caching for banner/native

**Why It Matters:**
- Faster ad rendering (pre-cached creatives)
- Reduced latency for client-side rendering
- Better user experience
- Standard Prebid Server feature

**What to Implement:**
- Prebid cache service client
- Bid caching for all media types (banner, native, video)
- Cache key generation and storage
- Cache URL injection into targeting keys
- Configuration for cache endpoints
- TTL management

**Files to Modify:**
- `internal/cache/prebid_cache.go` - New cache client
- `internal/exchange/exchange.go` - Cache integration
- `internal/exchange/targeting.go` - Cache key targeting
- `cmd/server/config.go` - Cache configuration

**Effort:** Medium (2-3 weeks)
**Business Impact:** Medium (performance improvement)

**Resources:**
- Prebid Cache Documentation: https://docs.prebid.org/prebid-server/features/pbs-caching.html

---

### 6. Complete Multiformat Request Handling

**Status:** ⚠️ Partial (native priority logic exists)

**Current State:**
- Native priority logic implemented
- Multi-format auction handling
- **Missing:** Full preferred mediatype selection

**Why It Matters:**
- Publisher control over ad format preferences
- Better fill rates with format fallbacks
- Standard Prebid feature

**What to Implement:**
- Preferred mediatype selection (`ext.prebid.multiformatRequestStrategy`)
- Format-specific bid validation
- Format priority enforcement
- Fallback logic when preferred format not available

**Files to Modify:**
- `internal/exchange/exchange.go` - Multiformat logic
- `internal/openrtb/models.go` - Multiformat extensions
- `internal/fpd/models.go` - Multiformat config

**Effort:** Low (1 week)
**Business Impact:** Medium (fill rate optimization)

---

### 7. Privacy Sandbox Support

**Status:** ❌ Not Implemented

**Current State:**
- No Chrome Topics support
- No Protected Audience (FLEDGE) support
- No testing labels support

**Why It Matters:**
- Chrome's privacy-preserving advertising future
- Prepare for cookie deprecation
- Competitive advantage for early adopters
- Industry is moving toward Privacy Sandbox

**What to Implement:**
- Topics API support (`browsingTopics` in request)
- Protected Audience (FLEDGE) auction config
- Testing labels passthrough
- Interest group bidding logic
- Configuration for Privacy Sandbox features

**Files to Modify:**
- `internal/openrtb/models.go` - Privacy Sandbox objects
- `internal/exchange/exchange.go` - FLEDGE auction logic
- `internal/fpd/processor.go` - Topics processing
- `cmd/server/config.go` - Privacy Sandbox config

**Effort:** High (4-6 weeks)
**Business Impact:** Medium (future-proofing)

**Resources:**
- Chrome Privacy Sandbox: https://privacysandbox.com/
- Prebid Privacy Sandbox: https://docs.prebid.org/prebid-server/features/pbs-privacy-sandbox.html

---

### 8. Stored Responses

**Status:** ❌ Not Implemented

**Current State:**
- Stored requests implemented
- **Missing:** Stored auction/bid responses

**Why It Matters:**
- Testing and debugging
- QA workflows
- Load testing with realistic data
- Bid replay scenarios

**What to Implement:**
- Stored response retrieval from database
- Response injection into auction flow
- Configuration for stored response IDs
- Admin API for managing stored responses
- TTL and versioning

**Files to Modify:**
- `internal/storage/stored_responses.go` - New storage layer
- `internal/exchange/exchange.go` - Response injection
- `internal/endpoints/auction.go` - Stored response handling
- `cmd/server/config.go` - Stored response config

**Effort:** Medium (2 weeks)
**Business Impact:** Low (QA/debugging tool)

---

### 9. Enhanced Currency Conversion

**Status:** ✅→⚠️ (Complete but could be enhanced)

**Current State:**
- 32+ currencies supported
- Auto-updates every 30 minutes
- **Potential Enhancement:** Historical rate tracking, fallback rates

**Why It Matters:**
- Better rate accuracy with fallbacks
- Historical rate analysis
- Rate volatility protection

**What to Implement:**
- Historical rate storage
- Rate fallback logic (if CDN update fails)
- Rate staleness detection
- Manual rate override capability

**Files to Modify:**
- `pkg/currency/converter.go` - Rate history
- `internal/exchange/currency.go` - Fallback logic

**Effort:** Low (1 week)
**Business Impact:** Low (marginal improvement)

---

## 🟢 LOW PRIORITY GAPS

### 10. Ads.Cert 2.0

**Status:** ❌ Not Implemented

**Current State:**
- No authenticated connections
- No supply chain verification via Ads.cert

**Why It Matters:**
- Fraud prevention
- Supply chain authentication
- Industry best practice for high-value inventory

**What to Implement:**
- Ads.cert signature generation
- Certificate management
- Signature validation
- Configuration for Ads.cert keys

**Files to Modify:**
- `internal/adscert/` - New package
- `internal/exchange/exchange.go` - Signature injection
- `cmd/server/config.go` - Ads.cert config

**Effort:** High (3-4 weeks)
**Business Impact:** Low (niche use case)

**Resources:**
- IAB Ads.cert Specification: https://iabtechlab.com/ads-cert/

---

### 11. ortb2-blocking Module

**Status:** ❌ Not Implemented

**Current State:**
- No advanced OpenRTB blocking rules
- Basic bid validation only

**Why It Matters:**
- Brand safety controls
- Content category blocking
- Advertiser domain filtering
- Advanced compliance rules

**What to Implement:**
- ortb2 blocking rules engine
- Category blocking (bcat)
- Advertiser domain blocking (badv)
- Attribute blocking (battr)
- Custom blocking rules via config

**Files to Modify:**
- `internal/blocking/ortb2.go` - New blocking module
- `internal/exchange/exchange.go` - Blocking integration
- `cmd/server/config.go` - Blocking configuration

**Effort:** Medium (2-3 weeks)
**Business Impact:** Low (available via bidder params already)

---

### 12. Request/Response Correction Modules

**Status:** N/A (Java-only feature)

**Why Not Applicable:**
- Prebid Server Java-specific feature
- Go implementation uses different architecture
- Not relevant for Go codebase

---

## Implementation Roadmap

### Phase 1: Privacy & Compliance (8-10 weeks)
1. **GPP Framework** (3-4 weeks)
2. **Activity Controls** (3-4 weeks)
3. **DSA Compliance** (2-3 weeks)

**Rationale:** Privacy compliance is critical for EU and US markets

---

### Phase 2: Revenue Optimization (3-5 weeks)
1. **Multibid Support** (2-3 weeks)
2. **Prebid Cache Integration** (2-3 weeks)
3. **Multiformat Enhancement** (1 week)

**Rationale:** Directly impacts revenue and fill rates

---

### Phase 3: Future-Proofing (4-6 weeks)
1. **Privacy Sandbox** (4-6 weeks)

**Rationale:** Prepare for cookie deprecation

---

### Phase 4: Nice-to-Have (4-6 weeks)
1. **Stored Responses** (2 weeks)
2. **ortb2-blocking** (2-3 weeks)
3. **Ads.Cert 2.0** (3-4 weeks) - If needed

**Rationale:** QA tools and niche features

---

## Effort Summary

| Priority | Features | Total Effort | Business Impact |
|----------|----------|--------------|-----------------|
| High | 4 | 10-14 weeks | Critical |
| Medium | 5 | 9-14 weeks | Significant |
| Low | 3 | 7-10 weeks | Marginal |
| **Total** | **12** | **26-38 weeks** | - |

## Resource Requirements

### Development Team
- **2-3 Backend Engineers** (Go expertise)
- **1 QA Engineer** (testing infrastructure)
- **1 DevOps Engineer** (deployment, configuration)

### Specialized Knowledge
- IAB specifications (GPP, DSA, Ads.cert)
- OpenRTB protocol expertise
- Privacy regulation knowledge (GDPR, CCPA, DSA)
- Prebid Server architecture

### Infrastructure
- Test Prebid cache instance
- GPP consent management platform (for testing)
- Privacy Sandbox test environment

---

## Success Metrics

### Coverage Target
- **Current:** 65% feature coverage
- **After Phase 1:** 75% coverage
- **After Phase 2:** 85% coverage
- **After Phase 3:** 90% coverage

### Business Metrics
- Revenue lift from Multibid (expected: 5-10%)
- Fill rate improvement from Multiformat (expected: 2-5%)
- Compliance score (100% for GPP/DSA)
- Latency impact from caching (expected: -20-30ms)

---

## Dependencies & Blockers

### External Dependencies
- **GPP Specification** - IAB finalization (currently stable)
- **Privacy Sandbox** - Chrome rollout timeline
- **DSA Requirements** - EU regulatory clarity

### Internal Dependencies
- Existing privacy middleware (for GPP/Activity Controls)
- OpenRTB model extensions (for new features)
- Configuration management (for new flags)
- Testing infrastructure (for QA)

### Potential Blockers
- Breaking changes to existing privacy logic
- Performance impact from new features (needs benchmarking)
- Third-party service dependencies (Prebid cache, GPP vendors)

---

## Quick Links

- **Full Audit**: [PREBID_FEATURE_AUDIT.md](PREBID_FEATURE_AUDIT.md)
- **Status Summary**: [FEATURE_IMPLEMENTATION_STATUS.md](FEATURE_IMPLEMENTATION_STATUS.md)
- **Prebid Docs**: https://docs.prebid.org/prebid-server/features/pbs-feature-idx.html

---

*Gap Analysis Date: 2026-02-12*
*Next Review: Q2 2026*
