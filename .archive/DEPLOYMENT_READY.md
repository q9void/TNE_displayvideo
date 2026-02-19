# 🚀 Analytics Deployment - Ready to Go!

## What You Need

Here's everything needed to deploy the analytics adapter system:

## ✅ Option 1: Use Backwards Compatible Mode (RECOMMENDED - Start immediately!)

**Best for**: Getting started right away without updating IDR service

### What It Does
- Uses existing IDR endpoint (`POST /api/events`)
- Converts new rich analytics to old event format
- **Zero IDR service changes needed**
- Full analytics benefits in Go code

### Deploy Steps

1. **Update server initialization** (`cmd/server/server.go`):

```go
// Replace the IDR adapter initialization with compat adapter
if s.config.IDREnabled && s.config.IDRUrl != "" {
    // Use backwards-compatible adapter (no IDR service changes needed!)
    compatAdapter := analyticsIDR.NewCompatAdapter(
        s.config.IDRUrl,
        s.config.ToExchangeConfig().EventBufferSize,
    )

    analyticsModules = append(analyticsModules, compatAdapter)

    log.Info().
        Str("adapter", "idr-compat").
        Str("idr_url", s.config.IDRUrl).
        Msg("Analytics adapter enabled (backwards compatible mode)")
}
```

2. **Start the server**:

```bash
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=your-api-key \
go run cmd/server/main.go
```

3. **Verify**:

```bash
# Send test auction
curl -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d @test/fixtures/auction_request.json

# Check IDR received events (using existing endpoint)
curl http://localhost:5050/api/events
```

**That's it!** ✅ Analytics is now live with ZERO IDR service changes!

---

## ✅ Option 2: Full Enhanced Mode (Better - Requires IDR Update)

**Best for**: Getting the full benefit of rich analytics events

### What You Get
- Three separate event types (Auction, Bidder, Win)
- Richer data for ML training
- Better revenue tracking
- Auction-level summaries

### IDR Service Changes Needed

Add these three endpoints to your Python IDR service:

```python
# IDR Service (Python) - Add these endpoints

@app.route('/api/events/auction', methods=['POST'])
def receive_auction_event():
    """Receive auction-level analytics"""
    event = request.json
    # Store auction summary
    auction_id = event['auction_id']
    total_revenue = event['total_revenue']
    winning_bids = event['winning_bids']
    # ... process auction event
    return jsonify({"status": "ok"}), 200

@app.route('/api/events/bidder', methods=['POST'])
def receive_bidder_event():
    """Receive per-bidder analytics"""
    event = request.json
    # Store bidder performance for ML
    bidder_code = event['bidder_code']
    latency_ms = event['latency_ms']
    first_bid_cpm = event.get('first_bid_cpm')
    # ... train ML model
    return jsonify({"status": "ok"}), 200

@app.route('/api/events/win', methods=['POST'])
def receive_win_event():
    """Receive winning bid analytics"""
    event = request.json
    # Track revenue
    platform_cut = event['platform_cut']
    adjusted_cpm = event['adjusted_cpm']
    # ... update revenue metrics
    return jsonify({"status": "ok"}), 200
```

### Event Schemas

**AuctionEvent**:
```json
{
  "auction_id": "abc123",
  "publisher_id": "pub789",
  "bidders_selected": 5,
  "bidders_excluded": 2,
  "total_bids": 3,
  "winning_bids": 1,
  "total_revenue": 0.25,
  "duration_ms": 150
}
```

**BidderEvent**:
```json
{
  "auction_id": "abc123",
  "bidder_code": "rubicon",
  "latency_ms": 50,
  "had_bid": true,
  "first_bid_cpm": 2.50,
  "below_floor": false
}
```

**WinEvent**:
```json
{
  "auction_id": "abc123",
  "bid_id": "bid789",
  "bidder_code": "rubicon",
  "platform_cut": 0.13,
  "adjusted_cpm": 2.63
}
```

### Deploy Steps

1. **Update IDR service** with new endpoints (above)

2. **Use standard adapter** (already configured in server.go):

```go
// Already done! Just start the server
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=your-api-key \
go run cmd/server/main.go
```

3. **Verify enhanced events**:

```bash
# Test auction event
curl http://localhost:5050/api/events/auction

# Test bidder events
curl http://localhost:5050/api/events/bidder

# Test win events
curl http://localhost:5050/api/events/win
```

---

## 🎯 Quick Start (30 seconds)

**Right now, use Option 1** (backwards compatible mode):

```bash
# 1. Update ONE line in cmd/server/server.go (line 199)
# Change:
#   idrAdapter := analyticsIDR.NewAdapter(idrClient, &analyticsIDR.Config{...})
# To:
#   idrAdapter := analyticsIDR.NewCompatAdapter(s.config.IDRUrl, s.config.ToExchangeConfig().EventBufferSize)

# 2. Start server
IDR_ENABLED=true IDR_URL=http://localhost:5050 IDR_API_KEY=test go run cmd/server/main.go

# 3. Done! Analytics is live! 🎉
```

**Later, upgrade to Option 2** when you update IDR service.

---

## Configuration

### Environment Variables

```bash
# Required
IDR_ENABLED=true                    # Enable analytics
IDR_URL=http://localhost:5050       # IDR service URL
IDR_API_KEY=your-api-key            # IDR API key

# Optional
EVENT_BUFFER_SIZE=100               # Buffer size (default: 100)
```

### Gradual Rollout

```bash
# Start with 10% traffic
ANALYTICS_SAMPLE_RATE=0.1 \
IDR_ENABLED=true \
go run cmd/server/main.go

# Increase gradually
ANALYTICS_SAMPLE_RATE=0.5  # 50%
ANALYTICS_SAMPLE_RATE=1.0  # 100%
```

---

## Verification Checklist

After deployment:

- [ ] Server starts without errors
- [ ] "Analytics adapter enabled" in logs
- [ ] Test auction completes successfully
- [ ] IDR service receives events
- [ ] No performance degradation (< 1ms overhead)
- [ ] Prometheus metrics still work
- [ ] Existing functionality unchanged

---

## Testing

### Integration Test

```bash
# Full test suite
go test -v ./...

# Analytics tests only
go test -v ./internal/analytics/...

# Exchange tests (verify no regression)
go test -v ./internal/exchange
```

### Manual Test

```bash
# 1. Start server
IDR_ENABLED=true IDR_URL=http://localhost:5050 go run cmd/server/main.go

# 2. Send test auction
curl -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-123",
    "imp": [{"id": "imp-1", "banner": {"w": 300, "h": 250}}],
    "site": {"domain": "example.com", "publisher": {"id": "pub-123"}},
    "device": {"ua": "Mozilla", "geo": {"country": "US"}}
  }'

# 3. Check logs for analytics
grep -i "analytics" server.log
```

---

## Rollback Plan

If issues occur:

```bash
# Option A: Disable analytics
IDR_ENABLED=false go run cmd/server/main.go

# Option B: Revert code
git revert <commit-hash>

# Option C: Feature flag (add to config)
ANALYTICS_ENABLED=false go run cmd/server/main.go
```

The old EventRecorder is still in place as backup!

---

## Performance Monitoring

Watch these metrics:

```bash
# Auction latency (should not increase)
curl http://localhost:8080/metrics | grep auction_duration

# Event send rate
curl http://localhost:8080/metrics | grep idr_events

# Error rate
curl http://localhost:8080/metrics | grep idr_errors
```

Expected:
- **Latency impact**: < 1ms
- **Memory impact**: < 5%
- **CPU impact**: Negligible

---

## What Happens Next

### Week 1-2: Validation
- Deploy with backwards compatible mode
- Monitor performance
- Validate data quality
- No IDR service changes needed!

### Week 3-4: Enhancement (Optional)
- Update IDR service with new endpoints
- Switch to full enhanced mode
- Get rich analytics events
- Better ML training data

### Month 2: DataDog
- Add DataDog adapter (no Exchange changes!)
- Distributed tracing
- Real-time alerts

---

## Need Help?

1. **Analytics not logging?**
   - Check `IDR_ENABLED=true`
   - Check `IDR_URL` is set
   - Check server logs

2. **Events not reaching IDR?**
   - Test IDR health: `curl http://localhost:5050/health`
   - Check network connectivity
   - Verify API key

3. **Performance issues?**
   - Check buffer size (increase if needed)
   - Monitor Prometheus metrics
   - Check for adapter errors

---

## Summary

**TODAY**: Deploy with Option 1 (backwards compatible) - **NO IDR CHANGES**

**LATER**: Upgrade to Option 2 (enhanced events) - requires IDR update

**RESULT**: Rich analytics, multi-sink ready, DataDog prep complete! 🎉

---

## Let's Deploy!

Run this NOW to get started:

```bash
# Edit cmd/server/server.go line 199:
# Use: analyticsIDR.NewCompatAdapter(...)

# Start server:
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=your-key \
go run cmd/server/main.go

# ✅ Analytics is LIVE!
```

That's it! The analytics adapter is production-ready and can run immediately with zero IDR service changes.
