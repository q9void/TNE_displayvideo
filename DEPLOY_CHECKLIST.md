# 🚀 Analytics Deployment - Complete Checklist

## Option B: Full Enhanced Mode

You chose the best option! Here's exactly what to do:

---

## ✅ Step-by-Step Checklist

### Phase 1: IDR Service Updates (30 mins)

- [ ] **1. Add three new endpoints to IDR service**
  - Location: Your Python IDR service (e.g., `app.py`)
  - Code: See `IDR_SERVICE_IMPLEMENTATION.md`
  - Endpoints:
    - `POST /api/events/auction`
    - `POST /api/events/bidder`
    - `POST /api/events/win`

- [ ] **2. Create database tables**
  ```bash
  # Run the SQL from IDR_SERVICE_IMPLEMENTATION.md
  psql -U your_user -d idr_db < schema.sql
  ```
  - Tables: `auction_events`, `bidder_events`, `win_events`

- [ ] **3. Test IDR endpoints**
  ```bash
  # Test auction endpoint
  curl -X POST http://localhost:5050/api/events/auction \
    -H "Content-Type: application/json" \
    -d '{"auction_id":"test-123","publisher_id":"pub-1","bidders_selected":5,"total_bids":3,"winning_bids":1,"duration_ms":150,"status":"success","total_revenue":0.25,"total_payout":2.25,"device":{"country":"US","type":"mobile"},"consent_ok":true,"validation_errors":0,"timestamp":"2024-01-15T10:30:00Z"}'

  # Expected: {"status": "ok", "auction_id": "test-123"}
  ```

- [ ] **4. Deploy updated IDR service**
  ```bash
  # Your deployment process
  # e.g., docker-compose restart idr
  # or: kubectl rollout restart deployment/idr
  ```

### Phase 2: Start PBS with Analytics (2 mins)

- [ ] **5. Verify configuration**
  ```bash
  # Check environment variables
  echo $IDR_URL      # Should be: http://your-idr-service:5050
  echo $IDR_API_KEY  # Should be set
  ```

- [ ] **6. Start PBS server**
  ```bash
  cd /Users/andrewstreets/tnevideo

  IDR_ENABLED=true \
  IDR_URL=http://localhost:5050 \
  IDR_API_KEY=your-api-key \
  go run cmd/server/main.go
  ```

- [ ] **7. Verify analytics initialization**
  Look for this in logs:
  ```
  INFO  Analytics adapter enabled adapter=idr idr_url=http://localhost:5050
  INFO  Analytics module initialized with multi-sink broadcasting adapter_count=1
  ```

### Phase 3: Verification (5 mins)

- [ ] **8. Send test auction**
  ```bash
  curl -X POST http://localhost:8080/openrtb2/auction \
    -H "Content-Type: application/json" \
    -H "X-Publisher-ID: test-publisher" \
    -d '{
      "id": "test-auction-456",
      "imp": [{
        "id": "imp-1",
        "banner": {"w": 300, "h": 250},
        "bidfloor": 1.0
      }],
      "site": {
        "domain": "example.com",
        "publisher": {"id": "pub-123"}
      },
      "device": {
        "ua": "Mozilla/5.0...",
        "ip": "192.168.1.1",
        "devicetype": 1,
        "geo": {"country": "US"}
      }
    }'
  ```

- [ ] **9. Verify events in IDR database**
  ```bash
  # Check auction events
  psql -U your_user -d idr_db -c "SELECT * FROM auction_events ORDER BY created_at DESC LIMIT 5;"

  # Check bidder events
  psql -U your_user -d idr_db -c "SELECT * FROM bidder_events ORDER BY created_at DESC LIMIT 10;"

  # Check win events
  psql -U your_user -d idr_db -c "SELECT * FROM win_events ORDER BY created_at DESC LIMIT 5;"
  ```

- [ ] **10. Check event counts**
  ```bash
  # Should see:
  # - 1 auction event per auction
  # - N bidder events (one per bidder)
  # - M win events (one per winning bid)

  psql -U your_user -d idr_db -c "
    SELECT
      'auction_events' as table_name, COUNT(*) as count
    FROM auction_events
    UNION ALL
    SELECT 'bidder_events', COUNT(*)
    FROM bidder_events
    UNION ALL
    SELECT 'win_events', COUNT(*)
    FROM win_events;
  "
  ```

### Phase 4: Monitoring (Ongoing)

- [ ] **11. Monitor performance**
  ```bash
  # Check auction latency (should not increase)
  curl http://localhost:8080/metrics | grep auction_duration

  # Check analytics errors
  grep -i "analytics.*error" server.log
  ```

- [ ] **12. Query analytics data**
  ```sql
  -- Revenue by bidder (last 24 hours)
  SELECT
    bidder_code,
    COUNT(*) as wins,
    SUM(platform_cut) as revenue
  FROM win_events
  WHERE created_at >= NOW() - INTERVAL '24 hours'
  GROUP BY bidder_code
  ORDER BY revenue DESC;

  -- Bidder performance
  SELECT
    bidder_code,
    COUNT(*) as requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as bids,
    AVG(latency_ms) as avg_latency
  FROM bidder_events
  WHERE created_at >= NOW() - INTERVAL '24 hours'
  GROUP BY bidder_code;
  ```

- [ ] **13. Set up alerts** (Optional)
  - High analytics error rate
  - Events not arriving in IDR
  - Latency increase > 10ms

---

## 🎯 Success Criteria

After deployment, you should see:

✅ **Server starts** with "Analytics adapter enabled" message
✅ **Events flowing** to all three IDR endpoints
✅ **Database tables** populated with auction data
✅ **No errors** in server logs
✅ **Performance** - latency increase < 1ms
✅ **Data quality** - all auctions tracked correctly

---

## 📊 What You Can Query Now

### Revenue Analysis
```sql
-- Daily revenue by bidder
SELECT
  DATE(created_at) as date,
  bidder_code,
  SUM(platform_cut) as daily_revenue,
  COUNT(*) as wins
FROM win_events
GROUP BY DATE(created_at), bidder_code
ORDER BY date DESC, daily_revenue DESC;
```

### Bidder Performance
```sql
-- Bidder win rate and latency
SELECT
  b.bidder_code,
  COUNT(DISTINCT b.auction_id) as auctions,
  SUM(CASE WHEN b.had_bid THEN 1 ELSE 0 END) as bids,
  COUNT(w.bid_id) as wins,
  AVG(b.latency_ms) as avg_latency
FROM bidder_events b
LEFT JOIN win_events w ON b.auction_id = w.auction_id AND b.bidder_code = w.bidder_code
WHERE b.created_at >= NOW() - INTERVAL '7 days'
GROUP BY b.bidder_code
ORDER BY wins DESC;
```

### Auction Summary
```sql
-- Hourly auction stats
SELECT
  DATE_TRUNC('hour', timestamp) as hour,
  COUNT(*) as auctions,
  AVG(total_bids) as avg_bids,
  AVG(duration_ms) as avg_duration_ms,
  SUM(total_revenue) as revenue
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', timestamp)
ORDER BY hour DESC;
```

---

## 🔧 Troubleshooting

### Issue: "Failed to send auction event"

**Check:**
```bash
# 1. IDR service is running
curl http://localhost:5050/health

# 2. Endpoints exist
curl -X POST http://localhost:5050/api/events/auction \
  -H "Content-Type: application/json" \
  -d '{"auction_id":"test"}'

# 3. PBS can reach IDR
ping your-idr-host
```

**Fix:**
- Verify IDR_URL is correct
- Check network connectivity
- Verify IDR service logs

### Issue: "Events not in database"

**Check:**
```bash
# 1. Database connection
psql -U your_user -d idr_db -c "SELECT 1;"

# 2. Tables exist
psql -U your_user -d idr_db -c "\dt"

# 3. IDR service logs
tail -f /var/log/idr/app.log
```

**Fix:**
- Create missing tables (run schema.sql)
- Check database permissions
- Verify IDR service can write to DB

### Issue: "High latency"

**Check:**
```bash
# Prometheus metrics
curl http://localhost:8080/metrics | grep duration

# PBS logs
grep "auction completed" server.log | tail -20
```

**Fix:**
- Analytics sends are async (shouldn't affect latency)
- Check IDR service response time
- Increase EVENT_BUFFER_SIZE if needed

---

## 📈 Next Steps

### Week 1: Validation
- [x] Deploy analytics
- [ ] Monitor for 1 week
- [ ] Validate data quality
- [ ] Check ML model still trains

### Week 2-3: Enhancement
- [ ] Build revenue dashboard
- [ ] Set up daily reports
- [ ] Add alerting rules
- [ ] Optimize queries

### Month 2: DataDog
- [ ] Add DataDog adapter
- [ ] Distributed tracing
- [ ] Real-time alerts
- [ ] APM dashboard

---

## 🎉 You're Done!

Once all checkboxes are complete, your analytics system is fully operational with:

✅ Rich auction transaction data
✅ ML training pipeline enhanced
✅ Revenue tracking enabled
✅ Multi-sink architecture ready
✅ DataDog prep complete

**Enjoy your new analytics superpowers!** 🚀

---

## Quick Reference

| What | Where |
|------|-------|
| IDR Implementation | `IDR_SERVICE_IMPLEMENTATION.md` |
| Full Documentation | `internal/analytics/README.md` |
| Quick Start | `ANALYTICS_QUICKSTART.md` |
| Implementation Details | `ANALYTICS_IMPLEMENTATION_SUMMARY.md` |
| This Checklist | `DEPLOY_CHECKLIST.md` |

**Questions?** Check the docs above or review the code in `internal/analytics/`
