# PBS Operations Guide
## Quick Reference for Hook Validation & Troubleshooting

---

## Hook Processing Order (Request Flow)

```
1. REQUEST VALIDATION (Currency Normalization)
   ↓
2. PRIVACY ENFORCEMENT (GDPR/CCPA)
   ↓
3. IDENTITY GATING (EID Filtering)
   ↓
4. BIDDER CALLS (Parallel)
   ├─ Per-Bidder Clone
   └─ SCHAIN AUGMENTATION
   ↓
5. BID COLLECTION
   ├─ RESPONSE NORMALIZATION
   └─ Bid Validation
   ↓
6. AUCTION LOGIC
   ↓
7. MULTIFORMAT SELECTION
   ↓
8. RESPONSE ASSEMBLY
```

---

## Common Issues & Solutions

### Issue 1: Currency Normalization Errors

**Symptom:**
```
ERROR: invalid bid request: impression[0] has invalid currency code "EURO"
```

**Cause:** Currency code is not 3 letters (ISO 4217)

**Solution:**
- Valid codes: USD, EUR, GBP, JPY (3 letters)
- Invalid codes: EURO, DOLLAR, $ (not 3 letters)
- Fix publisher integration to send proper codes

**Verification:**
```bash
# Check if currency being normalized
grep "currency normalized" /var/log/tne-pbs/app.log | tail -20

# Should see entries like:
# level=info msg="currency normalized" original_currency=eur normalized_currency=EUR
```

---

### Issue 2: Privacy Rejection Spikes

**Symptom:**
```
WARN: privacy rejection: GDPR applies but consent missing
```

**Cause:** EU traffic without valid consent string

**Solution:**

1. **Check if GDPR actually applies:**
   ```bash
   # Verify geo detection
   curl -X POST http://localhost:8000/openrtb2/auction \
     -H "X-Forwarded-For: 89.160.20.112" \
     -d @test_request.json
   ```

2. **Check consent string format:**
   ```json
   {
     "regs": {"gdpr": 1},
     "user": {"consent": "CPc..."}  // Must be TCF v2 string
   }
   ```

3. **Temporarily disable for debugging (NEVER in production):**
   ```bash
   export PBS_ENFORCE_GDPR=false
   sudo systemctl restart tne-pbs
   ```

**Metrics to Monitor:**
- Expected GDPR rejection: 5-10% of EU traffic
- If > 20%: Check consent string integration
- If > 50%: Likely consent provider outage

---

### Issue 3: SChain Not Being Augmented

**Symptom:**
```
# No platform node in outgoing bidder requests
# Or: SChain is null/empty
```

**Cause:** SChain augmentation not being called or failing silently

**Solution:**

1. **Enable debug logging:**
   ```bash
   export LOG_LEVEL=debug
   sudo systemctl restart tne-pbs
   ```

2. **Check logs for augmentation:**
   ```bash
   grep "schain augmented" /var/log/tne-pbs/app.log

   # Should see:
   # level=debug msg="schain augmented" created=true nodes_added=1
   ```

3. **Verify platform node added:**
   ```bash
   # Check bidder request logs
   grep "thenexusengine.com" /var/log/tne-pbs/bidder_requests.log

   # Should see platform node:
   # "schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"thenexusengine.com","sid":"tne-platform","hp":1}]}
   ```

**Common Causes:**
- Source object is nil (should be created automatically)
- SChain already has platform node (duplicate check working)

---

### Issue 4: Multiformat Selection Not Working

**Symptom:**
```
# Multiformat impressions not selecting best format
# Or: Always selecting banner even when video has higher CPM
```

**Cause:** Multiformat disabled or strategy not configured

**Solution:**

1. **Check feature flag:**
   ```bash
   curl http://localhost:8000/health | jq '.features.multiformat'
   # Should return: {"enabled": true}
   ```

2. **Verify impression has multiformat strategy:**
   ```json
   {
     "imp": [{
       "banner": {...},
       "video": {...},
       "ext": {
         "prebid": {
           "multiformatRequestStrategy": "server"  // REQUIRED
         }
       }
     }]
   }
   ```

3. **Check analytics for selections:**
   ```bash
   curl http://localhost:8000/metrics | grep multiformat_selections_total
   ```

**Expected Behavior:**
- Impressions with 2+ formats trigger multiformat logic
- Server strategy: Selects highest CPM bid
- Missing strategy: Falls back to default (server)

---

### Issue 5: Response Validation Rejecting Valid Bids

**Symptom:**
```
ERROR: bid validation failed: invalid nurl format: URL must use HTTPS
```

**Cause:** Bidder returning HTTP nurl instead of HTTPS

**Solution:**

1. **Check bidder response:**
   ```bash
   grep "bidder response" /var/log/tne-pbs/app.log | grep nurl

   # Invalid:
   # "nurl": "http://example.com/win"

   # Valid:
   # "nurl": "https://example.com/win"
   ```

2. **Contact bidder to fix:**
   - NURL must be HTTPS (OpenRTB 2.5 requirement)
   - Or: Provide AdM instead of NURL

3. **Temporarily allow HTTP (NOT recommended):**
   ```go
   // In validateBid() - NOT for production
   if err := validateURL(bid.NURL, false); err != nil {  // false = allow HTTP
   ```

**Other Common Response Validation Failures:**
- Missing both `adm` and `nurl` (at least one required)
- Response ID mismatch (bidder echoing wrong request ID)
- Invalid currency not in request allowlist

---

## Feature Flags Reference

| Flag | Default | Purpose | Safe to Toggle |
|------|---------|---------|----------------|
| `ADMIN_AUTH_REQUIRED` | `true` | Require admin API key | ❌ Never disable in prod |
| `PPROF_ENABLED` | `false` | Enable pprof debug endpoints | ❌ Never enable in prod |
| `PBS_ENFORCE_GDPR` | `true` | Enforce GDPR requirements | ⚠️ Only for debugging |
| `PBS_ENFORCE_CCPA` | `true` | Enforce CCPA requirements | ⚠️ Only for debugging |
| `TRUST_X_FORWARDED_FOR` | `false` | Trust X-Forwarded-For header | ⚠️ Only behind trusted proxy |
| `MULTIFORMAT_ENABLED` | `true` | Enable multiformat selection | ✅ Safe to toggle |
| `SECOND_PRICE_ENABLED` | `true` | Use second-price auction | ✅ Safe to toggle |

---

## Health Check Endpoints

```bash
# Basic health
curl http://localhost:8000/health
# Response: {"status":"ok","version":"v1.0.0"}

# Detailed health (requires admin auth)
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8000/admin/health
# Response: {...detailed stats...}

# Readiness (for load balancers)
curl http://localhost:8000/health/ready
# 200 = ready, 503 = not ready

# Metrics (Prometheus format)
curl http://localhost:8000/metrics
```

---

## Debugging Commands

### View Recent Auctions
```bash
# Last 100 auctions
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8000/admin/auctions?limit=100

# Filter by publisher
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8000/admin/auctions?publisher_id=pub123
```

### Check Privacy Enforcement Stats
```bash
# GDPR rejection count (last hour)
curl http://localhost:8000/metrics | \
  grep 'pbs_privacy_rejections_total{regulation="gdpr"}'

# CCPA opt-out rate
curl http://localhost:8000/metrics | \
  grep 'pbs_ccpa_opt_out_total'
```

### View SChain Completion
```bash
# Requests with complete schain
curl http://localhost:8000/metrics | \
  grep 'pbs_schain_complete_total'

# Requests where schain was created
curl http://localhost:8000/metrics | \
  grep 'pbs_schain_created_total'
```

### Check Multiformat Activity
```bash
# Multiformat impressions
curl http://localhost:8000/metrics | \
  grep 'pbs_multiformat_impressions_total'

# Format selections (banner vs video)
curl http://localhost:8000/metrics | \
  grep 'pbs_multiformat_selected_format_total'
```

---

## Log Analysis

### Find Currency Normalization Events
```bash
# Show all normalized currencies
journalctl -u tne-pbs | grep "currency normalized" | \
  jq '{original: .original_currency, normalized: .normalized_currency}' | \
  sort | uniq -c
```

### Find Privacy Rejections by Country
```bash
# GDPR rejections by country
journalctl -u tne-pbs | grep "privacy rejection" | \
  grep "gdpr" | jq '.country' | sort | uniq -c
```

### Find SChain Augmentation Issues
```bash
# Show SChain creation events
journalctl -u tne-pbs | grep "schain augmented" | \
  jq '{created: .created, nodes_added: .nodes_added, bidder: .bidder}'
```

### Find Response Validation Failures
```bash
# Group by failure reason
journalctl -u tne-pbs | grep "response validation failed" | \
  jq '.reason' | sort | uniq -c | sort -rn
```

---

## Performance Tuning

### If Latency Too High (> 150ms P95)

1. **Check hook overhead:**
   ```bash
   # Compare with hooks disabled
   export MULTIFORMAT_ENABLED=false
   export PBS_ENFORCE_GDPR=false
   # Measure latency difference
   ```

2. **Optimize database queries:**
   ```sql
   -- Check slow queries
   SELECT * FROM pg_stat_statements
   ORDER BY total_time DESC LIMIT 10;
   ```

3. **Increase concurrent bidder limit:**
   ```bash
   export MAX_CONCURRENT_BIDDERS=20  # Default: 10
   ```

### If Memory Usage Too High

1. **Check clone limits:**
   ```bash
   # Reduce max impressions per request
   export MAX_IMPRESSIONS_PER_REQUEST=50  # Default: 100
   ```

2. **Check for leaks:**
   ```bash
   # Enable pprof (staging only!)
   export PPROF_ENABLED=true
   curl http://localhost:8000/debug/pprof/heap > heap.prof
   go tool pprof heap.prof
   ```

---

## Emergency Procedures

### Privacy Enforcement Bypass (Regulatory Emergency Only)

```bash
# ONLY if legal approval received to bypass enforcement temporarily
export PBS_ENFORCE_GDPR=false
export PBS_ENFORCE_CCPA=false
sudo systemctl restart tne-pbs

# MUST re-enable within 24 hours
# Document incident and approval
```

### Rollback to Previous Version

```bash
# Stop current version
sudo systemctl stop tne-pbs

# Restore previous binary
sudo cp /opt/tne-pbs/server.backup /opt/tne-pbs/server

# Start previous version
sudo systemctl start tne-pbs

# Verify
curl http://localhost:8000/health
```

### Disable Specific Bidder

```bash
# Via admin API
curl -X POST -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8000/admin/bidders/disable \
  -d '{"bidder_code": "problematic-bidder"}'

# Or via database
psql -c "UPDATE bidder_configs SET enabled = false
         WHERE bidder_code = 'problematic-bidder';"
```

---

## Support Contacts

**On-Call Rotation:** Check PagerDuty schedule

**Escalation:**
1. On-Call Engineer (immediate response)
2. Engineering Manager (within 1 hour)
3. CTO (critical incidents only)

**Slack Channels:**
- `#pbs-ops` - Operational issues
- `#pbs-alerts` - Automated alerts
- `#pbs-deploys` - Deployment notifications

**Documentation:**
- Runbook: `/docs/runbook.md`
- Architecture: `/docs/architecture.md`
- API Docs: `https://docs.tne-pbs.com`

---

**Last Updated:** 2026-02-15
**Version:** v1.0.0
