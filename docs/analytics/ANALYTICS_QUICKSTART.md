# Analytics Module - Quick Start Guide

## 🚀 Getting Started

The analytics adapter is **already integrated** and ready to use! Here's how to get started.

## Step 1: Verify Installation

Check that the analytics module was installed correctly:

```bash
# Verify files exist
ls -la internal/analytics/
ls -la internal/analytics/idr/

# Run tests
go test ./internal/analytics/...
```

Expected output:
```
PASS
ok  	github.com/thenexusengine/tne_springwire/internal/analytics	0.6s
ok  	github.com/thenexusengine/tne_springwire/internal/analytics/idr	0.7s
```

## Step 2: Start the Server with Analytics

Analytics is **automatically enabled** when IDR is enabled:

```bash
# Start server with analytics
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=your-api-key \
go run cmd/server/main.go
```

Look for these log messages:
```
INFO  Analytics adapter enabled adapter=idr idr_url=http://localhost:5050
INFO  Analytics module initialized with multi-sink broadcasting adapter_count=1
```

## Step 3: Send a Test Auction

```bash
# Send test auction request
curl -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -H "X-Publisher-ID: test-publisher" \
  -d '{
    "id": "test-auction-123",
    "imp": [{
      "id": "imp-1",
      "banner": {
        "w": 300,
        "h": 250
      },
      "bidfloor": 1.0
    }],
    "site": {
      "domain": "example.com",
      "publisher": {
        "id": "pub-123"
      }
    },
    "device": {
      "ua": "Mozilla/5.0...",
      "ip": "192.168.1.1",
      "devicetype": 1,
      "geo": {
        "country": "US"
      }
    }
  }'
```

## Step 4: Verify Analytics Events

Check that the IDR service received analytics events:

```bash
# Check IDR service health
curl http://localhost:5050/health

# Check IDR service logs
# (The IDR service should show received events)
```

Expected events sent to IDR:
1. **AuctionEvent** - Auction summary
2. **BidderEvent** - Per-bidder results (one per bidder)
3. **WinEvent** - Winning bids (one per winning bid)

## Step 5: View Analytics Data

Analytics data is now available through:

1. **IDR Service API** - Query auction events
2. **Prometheus Metrics** (existing) - Operational metrics at `/metrics`
3. **Structured Logs** (existing) - Debug information

Example analytics queries you can now run:

### Query: Why didn't a bidder participate?

```go
// Check excluded bidders in AuctionObject
if reason, excluded := auctionObj.ExcludedBidders["rubicon"]; excluded {
    fmt.Printf("Rubicon excluded: %s - %s\n", reason.Code, reason.Message)
}
```

### Query: Platform revenue by bidder

```go
// Sum platform cuts from winning bids
revenueByBidder := make(map[string]float64)
for _, win := range auctionObj.WinningBids {
    revenueByBidder[win.BidderCode] += win.PlatformCut
}
```

### Query: Bid validation failures

```go
// Count validation errors
for _, err := range auctionObj.ValidationErrors {
    fmt.Printf("Bid %s failed validation: %s\n", err.BidID, err.Reason)
}
```

## Configuration Options

### Environment Variables

```bash
# Required (analytics enabled when IDR is enabled)
IDR_ENABLED=true                    # Enable IDR and analytics
IDR_URL=http://localhost:5050       # IDR service URL
IDR_API_KEY=your-api-key            # IDR API key

# Optional
EVENT_BUFFER_SIZE=100               # Event buffer size (default: 100)
```

### Disable Analytics

To disable analytics (keep existing metrics/logging):

```bash
IDR_ENABLED=false \
go run cmd/server/main.go
```

## Troubleshooting

### Issue: "Analytics adapter not initialized"

**Solution**: Check that `IDR_ENABLED=true` and `IDR_URL` is set:

```bash
echo $IDR_ENABLED  # Should output: true
echo $IDR_URL      # Should output: http://localhost:5050
```

### Issue: "Failed to send auction event"

**Solution**: Check IDR service is running and accessible:

```bash
# Test IDR service health
curl http://localhost:5050/health

# Expected response:
# {"status":"healthy"}
```

### Issue: "Analytics module failed to log auction"

**Solution**: Check server logs for details:

```bash
# View recent logs
tail -f server.log | grep -i analytics

# Look for warning messages with error details
```

### Issue: Events not appearing in IDR

**Possible causes**:
1. IDR service not running → Start IDR service
2. Wrong URL → Check `IDR_URL` matches IDR service
3. Network issue → Test connectivity with `curl`
4. Buffer full → Check `EVENT_BUFFER_SIZE` (increase if needed)

## Next Steps

### 1. Monitor Analytics Performance

Watch key metrics:
```bash
# View Prometheus metrics
curl http://localhost:8080/metrics | grep auction

# Check analytics overhead
# (Should be < 1ms per auction)
```

### 2. Validate Data Quality

Check that analytics data makes sense:
- Are all bidders tracked?
- Are exclusion reasons accurate?
- Are winning bids correct?
- Are revenue calculations right?

### 3. Plan DataDog Integration (Month 2)

The system is ready for DataDog APM tracing:

```go
// Future: Add DataDog adapter
if os.Getenv("DATADOG_ENABLED") == "true" {
    ddAdapter := datadog.NewAdapter(apiKey)
    analyticsModules = append(analyticsModules, ddAdapter)
}
```

## Common Use Cases

### Use Case 1: Debug Why Bidder Was Excluded

```go
// In your analytics adapter or query tool
func analyzeBidderExclusion(auctionID, bidder string) {
    // Fetch auction object from IDR
    auction := getAuctionByID(auctionID)

    if reason, excluded := auction.ExcludedBidders[bidder]; excluded {
        fmt.Printf("Bidder: %s\n", bidder)
        fmt.Printf("Exclusion Code: %s\n", reason.Code)
        fmt.Printf("Reason: %s\n", reason.Message)
    } else {
        fmt.Printf("Bidder %s was selected for auction\n", bidder)
    }
}
```

### Use Case 2: Calculate Platform Revenue

```go
func calculateDailyRevenue(date string) float64 {
    // Query all auctions for the date
    auctions := getAuctionsByDate(date)

    totalRevenue := 0.0
    for _, auction := range auctions {
        totalRevenue += auction.TotalRevenue
    }

    return totalRevenue
}
```

### Use Case 3: Analyze Bidder Performance

```go
func analyzeBidderPerformance(bidder string) {
    // Query all auctions with this bidder
    results := getBidderResults(bidder)

    totalAuctions := len(results)
    totalBids := 0
    totalWins := 0
    avgLatency := 0.0

    for _, result := range results {
        totalBids += result.BidCount
        avgLatency += float64(result.LatencyMs)
        if result.HadWin {
            totalWins++
        }
    }

    fmt.Printf("Bidder: %s\n", bidder)
    fmt.Printf("Win Rate: %.2f%%\n", float64(totalWins)/float64(totalAuctions)*100)
    fmt.Printf("Avg Latency: %.0fms\n", avgLatency/float64(totalAuctions))
}
```

## Testing

### Run All Tests

```bash
# Analytics module tests
go test -v ./internal/analytics/...

# Exchange integration tests
go test -v ./internal/exchange

# Full test suite
go test -v ./...
```

### Manual Testing Checklist

- [ ] Server starts with analytics enabled
- [ ] Analytics initialization logged
- [ ] Test auction completes successfully
- [ ] IDR service receives events
- [ ] Prometheus metrics still work
- [ ] No performance degradation
- [ ] Graceful shutdown works

## Documentation

For detailed information, see:

- **Module Documentation**: `internal/analytics/README.md`
- **Implementation Summary**: `ANALYTICS_IMPLEMENTATION_SUMMARY.md`
- **Architecture Plan**: Review the original implementation plan

## Support

If you encounter issues:

1. Check this quick start guide
2. Review `internal/analytics/README.md`
3. Check unit tests for usage examples
4. Review server logs for error messages

---

**Ready to go!** 🎉

Your analytics adapter is now set up and ready to provide rich auction insights.
