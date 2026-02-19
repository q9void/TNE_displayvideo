# Production Deployment Checklist
## 35 Critical Fixes + Hook Validation Implementation

**Deployment Date:** TBD
**Version:** v1.0.0 (Production-Ready)
**Deployment Lead:** TBD

---

## Pre-Deployment Checklist

### 1. Code Review & Testing ✓

- [x] All 26 implementation tasks completed
- [ ] All unit tests passing (`go test ./...`)
- [ ] Integration tests passing (`./test/hooks_validation_test.sh`)
- [ ] Load testing completed (1000 req/s sustained)
- [ ] Security audit passed (penetration testing)
- [ ] Code review approved by 2+ engineers

### 2. Environment Configuration

#### Required Environment Variables

```bash
# Core Configuration
PBS_PORT=8000
PBS_DEV_MODE=false
DEFAULT_TIMEOUT_MS=1000
DEFAULT_CURRENCY=USD

# Security (CRITICAL - Must be set)
ADMIN_API_KEY=<strong-random-key>           # Generate: openssl rand -base64 32
ADMIN_AUTH_REQUIRED=true                     # Fail-closed admin auth
PPROF_ENABLED=false                          # Never enable in production
TRUST_X_FORWARDED_FOR=false                  # Only true behind trusted proxy

# Privacy & Compliance
PBS_ENFORCE_GDPR=true                        # Enable GDPR enforcement
PBS_ENFORCE_CCPA=true                        # Enable CCPA enforcement
PRIVACY_MIDDLEWARE_STRICT=true               # Strict privacy mode

# CORS
CORS_ENABLED=true
CORS_ORIGINS=https://yourdomain.com,https://www.yourdomain.com
CORS_ALLOW_CREDENTIALS=true

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=tne_pbs
DB_USER=pbs_user
DB_PASSWORD=<secure-password>
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10

# Redis
REDIS_HOST=localhost:6379
REDIS_PASSWORD=<secure-password>
REDIS_DB=0

# Feature Flags
MULTIFORMAT_ENABLED=true                     # Enable multiformat support
SECOND_PRICE_ENABLED=true                    # Enable second-price auction
IDR_ENABLED=true                             # Enable Intelligent Demand Router
```

### 3. Database Migrations

```bash
# Run migrations
./scripts/migrate.sh up

# Verify schema
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "\dt"

# Verify test data (staging only)
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT COUNT(*) FROM publishers_new;"
```

### 4. Build & Package

```bash
# Build for production
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-linux \
  -ldflags="-s -w -X main.version=$(git describe --tags)" \
  ./cmd/server

# Verify binary
./server-linux --version

# Create Docker image (if using containers)
docker build -t tne-pbs:v1.0.0 .
docker tag tne-pbs:v1.0.0 tne-pbs:latest
```

---

## Deployment Phases (Staged Rollout)

### Phase 1: Staging Deployment (Day 1)

**Duration:** 24 hours
**Traffic:** Staging environment only
**Monitoring:** Full observability enabled

#### Actions:

1. **Deploy to staging**
   ```bash
   # Deploy binary
   scp server-linux staging:/opt/tne-pbs/

   # Update systemd service
   ssh staging "sudo systemctl restart tne-pbs"

   # Verify service started
   ssh staging "sudo systemctl status tne-pbs"
   ```

2. **Run smoke tests**
   ```bash
   export PBS_URL=https://staging.tne-pbs.com
   ./test/hooks_validation_test.sh
   ```

3. **Run load tests**
   ```bash
   # 100 req/s for 10 minutes
   ./scripts/load_test.sh --url https://staging.tne-pbs.com \
     --rate 100 --duration 600

   # Monitor: CPU, memory, latency, error rate
   ```

4. **Verify all hooks**
   - [ ] Currency normalization (check logs for "EUR" not "eur")
   - [ ] Privacy enforcement (GDPR/CCPA rejection logs)
   - [ ] SChain augmentation (verify platform node in debug logs)
   - [ ] Multiformat selection (check analytics for format distribution)
   - [ ] Response validation (no invalid response errors)

#### Success Criteria:
- [ ] All smoke tests pass
- [ ] P95 latency < 150ms (was 100ms + 50ms hook overhead acceptable)
- [ ] Error rate < 0.1%
- [ ] No memory leaks over 24h
- [ ] No crashes or panics

### Phase 2: Production Canary (Day 2)

**Duration:** 48 hours
**Traffic:** 10% production traffic
**Monitoring:** Real-time alerts enabled

#### Actions:

1. **Deploy to canary instances**
   ```bash
   # Deploy to 2 of 20 instances (10%)
   ansible-playbook -i inventory/production deploy.yml \
     --limit canary_hosts --tags deploy
   ```

2. **Enable gradual traffic shift**
   ```bash
   # Configure load balancer for 10% to canary
   ./scripts/traffic_shift.sh --canary 10
   ```

3. **Monitor key metrics (first 4 hours)**
   - Request rate to canary instances
   - Error rate comparison (canary vs stable)
   - Latency P50/P95/P99 comparison
   - Privacy rejection rate (should increase if enforcement was broken)

#### Success Criteria:
- [ ] Canary error rate ≤ stable error rate
- [ ] Canary P95 latency < stable + 20ms
- [ ] No increase in client complaints
- [ ] Privacy metrics match expected rates:
  - GDPR rejection rate ~5-10% (EU traffic without consent)
  - CCPA opt-out rate ~2-3% (CA traffic)

#### Rollback Trigger:
- Error rate > 1%
- P95 latency > 200ms
- Memory leak detected
- Any crashes/panics

### Phase 3: Production Rollout (Day 3-4)

**Duration:** 2 days
**Traffic:** Gradual 10% → 50% → 100%
**Monitoring:** Continuous observation

#### Actions:

**Day 3 Morning (10% → 50%)**
```bash
# Increase canary traffic
./scripts/traffic_shift.sh --canary 50

# Monitor for 4 hours
# If stable, proceed to full rollout
```

**Day 3 Afternoon (50% → 100%)**
```bash
# Deploy to all instances
ansible-playbook -i inventory/production deploy.yml --tags deploy

# Full traffic cutover
./scripts/traffic_shift.sh --canary 0
```

**Day 4 - Stable Monitoring**
- Monitor for regression
- Verify all features working
- Collect baseline metrics

#### Success Criteria:
- [ ] No increase in error rate vs canary
- [ ] Latency remains stable
- [ ] Fill rate maintained or improved
- [ ] Revenue stable or increased

### Phase 4: Feature Enablement (Day 5-7)

**Duration:** 3 days
**Traffic:** 100% production
**Monitoring:** Feature-specific metrics

Features are deployed but **disabled by default**. Enable gradually:

#### Day 5: Core Security (Already Enabled)
- Admin auth fail-closed ✓ (ADMIN_AUTH_REQUIRED=true)
- pprof protection ✓ (PPROF_ENABLED=false)
- Privacy middleware coverage ✓ (Always on)

#### Day 6: Privacy Enforcement
```bash
# Enable GDPR enforcement
export PBS_ENFORCE_GDPR=true

# Enable CCPA enforcement
export PBS_ENFORCE_CCPA=true

# Restart service
sudo systemctl restart tne-pbs
```

**Monitor:**
- GDPR rejection rate (expect 5-10% EU traffic)
- CCPA opt-out rate (expect 2-3% CA traffic)
- Fill rate impact (may drop 2-5% due to enforcement)

#### Day 7: Multiformat & Advanced Features
```bash
# Enable multiformat (opt-in per publisher)
# Update publisher config in database
psql -c "UPDATE publishers_new SET
  multiformat_enabled = true
  WHERE account_id IN ('pilot-publisher-1', 'pilot-publisher-2');"

# Restart to pick up config changes
sudo systemctl restart tne-pbs
```

**Monitor:**
- Multiformat selection rate
- Video vs banner win rate
- Revenue per multiformat impression

---

## Rollback Procedures

### Emergency Rollback (< 5 minutes)

```bash
# Immediate traffic switch to previous version
./scripts/traffic_shift.sh --rollback

# Or: Stop new version, start old version
ssh production "sudo systemctl stop tne-pbs && \
  cp /opt/tne-pbs/server.old /opt/tne-pbs/server && \
  sudo systemctl start tne-pbs"
```

### Partial Rollback (Feature Flags)

```bash
# Disable specific features without full rollback
export PBS_ENFORCE_GDPR=false
export MULTIFORMAT_ENABLED=false
sudo systemctl restart tne-pbs
```

### Database Rollback

```bash
# Rollback migrations (if schema changes made)
./scripts/migrate.sh down 1
```

---

## Post-Deployment Verification

### Smoke Tests (Run after each phase)

```bash
# 1. Health check
curl https://api.tne-pbs.com/health
# Expected: {"status":"ok"}

# 2. Admin auth test
curl https://api.tne-pbs.com/admin/dashboard
# Expected: 401 Unauthorized

# 3. Basic auction
curl -X POST https://api.tne-pbs.com/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d @test/fixtures/basic_auction.json
# Expected: 200 OK with seatbid

# 4. Privacy enforcement
curl -X POST https://api.tne-pbs.com/openrtb2/auction \
  -H "X-Forwarded-For: 89.160.20.112" \
  -d @test/fixtures/gdpr_no_consent.json
# Expected: 400 Bad Request (GDPR error)

# 5. Currency normalization
curl -X POST https://api.tne-pbs.com/openrtb2/auction \
  -d @test/fixtures/lowercase_currency.json
# Expected: 200 OK (currency normalized)
```

### Metrics Validation

```bash
# Check Prometheus metrics
curl https://api.tne-pbs.com/metrics | grep -E "auction_requests|privacy_rejections|multiformat_selections"

# Expected metrics:
# auction_requests_total{endpoint="/openrtb2/auction"} > 0
# privacy_rejections_total{reason="gdpr_no_consent"} > 0
# multiformat_selections_total{strategy="server"} > 0
```

---

## Monitoring & Alerts

### Key Metrics to Watch

#### Performance Metrics
- **Request Rate:** req/s by endpoint
- **Latency:** P50, P95, P99 by endpoint
- **Error Rate:** % by endpoint and error type
- **CPU/Memory:** % utilization per instance

#### Business Metrics
- **Fill Rate:** % impressions with bids
- **Revenue:** $ per 1000 impressions (RPM)
- **Bid Rate:** Bids per impression
- **Win Rate:** % bids that win

#### Hook-Specific Metrics
- **Currency Normalization Rate:** % requests with lowercase currency
- **Privacy Rejection Rate:** % requests blocked by GDPR/CCPA
- **SChain Completion Rate:** % requests with complete supply chain
- **Multiformat Selection Rate:** % multiformat impressions
- **Response Validation Failures:** Invalid response count by reason

### Alert Thresholds

```yaml
# Critical Alerts (PagerDuty)
- name: high_error_rate
  condition: error_rate > 1%
  duration: 5m

- name: high_latency
  condition: p95_latency > 200ms
  duration: 5m

- name: service_down
  condition: health_check_failures > 3
  duration: 1m

# Warning Alerts (Slack)
- name: elevated_error_rate
  condition: error_rate > 0.5%
  duration: 10m

- name: privacy_rejection_spike
  condition: privacy_rejection_rate > 20%
  duration: 15m

- name: fill_rate_drop
  condition: fill_rate < baseline * 0.9
  duration: 30m
```

---

## Communication Plan

### Stakeholders

| Group | Notification Timing | Method |
|-------|-------------------|--------|
| Engineering Team | 48h before, during rollout | Slack #engineering |
| Product Team | 24h before, after completion | Email + Slack |
| Customer Success | Before canary, after Phase 3 | Email + Meeting |
| Publishers (if affected) | After successful Phase 3 | Email announcement |

### Status Updates

**During Deployment:**
- Post updates every 2 hours in #deployments
- Immediate notification if rollback triggered

**Post-Deployment:**
- 24h report: Metrics comparison vs baseline
- 7-day report: Feature adoption, revenue impact

---

## Success Metrics (7-Day Post-Deployment)

### Technical Success
- [ ] Error rate < 0.1% (same as pre-deployment)
- [ ] P95 latency < 150ms (within 50ms of baseline)
- [ ] No production incidents
- [ ] Uptime > 99.9%

### Business Success
- [ ] Fill rate maintained within 5% of baseline
- [ ] Revenue per 1000 impressions (RPM) maintained or improved
- [ ] Privacy compliance: 100% enforcement (no violations)

### Feature Adoption
- [ ] Currency normalization: 100% of requests with currency codes
- [ ] SChain completion: >95% of requests have complete chain
- [ ] Multiformat: >50% of eligible publishers opted in
- [ ] Response validation: <0.1% invalid responses

---

## Rollback Decision Matrix

| Metric | Threshold | Action |
|--------|-----------|--------|
| Error rate > 2% | Immediate | Emergency rollback |
| P95 latency > 300ms | 10 minutes | Emergency rollback |
| Service crash | Immediate | Emergency rollback |
| Error rate 1-2% | 30 minutes | Feature rollback, investigate |
| Fill rate drop >10% | 1 hour | Feature rollback (privacy enforcement) |
| Privacy bypass detected | Immediate | Investigation, potential rollback |

---

## Post-Deployment Tasks

- [ ] Update documentation with new features
- [ ] Archive deployment logs and metrics
- [ ] Conduct post-mortem (if issues occurred)
- [ ] Update runbook with learnings
- [ ] Schedule tech debt cleanup
- [ ] Plan next iteration features

---

## Emergency Contacts

| Role | Name | Phone | Slack |
|------|------|-------|-------|
| Deployment Lead | TBD | TBD | @deployment-lead |
| On-Call Engineer | TBD | TBD | @oncall |
| Engineering Manager | TBD | TBD | @eng-manager |
| DevOps Lead | TBD | TBD | @devops-lead |

---

**Approval Signatures:**

- [ ] Engineering Lead: _________________ Date: _______
- [ ] Product Lead: _________________ Date: _______
- [ ] Security Lead: _________________ Date: _______
- [ ] DevOps Lead: _________________ Date: _______

---

**Deployment Notes:**
_Use this section to track actual deployment progress and any deviations from plan_

```
[Date/Time] - [Phase] - [Status] - [Notes]

Example:
2026-02-15 09:00 - Phase 1 - Started - Deployed to staging
2026-02-15 10:30 - Phase 1 - Success - All smoke tests passed
2026-02-16 14:00 - Phase 2 - Started - Canary at 10%
2026-02-16 18:00 - Phase 2 - Success - No errors, latency stable
```
