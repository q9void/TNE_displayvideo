# Analytics Module

## Overview

The analytics module provides a **standardized interface for auction analytics** with support for multiple sinks (IDR, DataDog, BigQuery, etc.). It replaces scattered event recording calls with a single, rich `AuctionObject` that contains complete auction transaction data.

## Architecture

```
┌──────────────────────────────────────────────────┐
│      Exchange (internal/exchange/exchange.go)    │
│   Single LogAuctionObject() call after auction   │
└─────────────────┬────────────────────────────────┘
                  │
                  ▼
┌──────────────────────────────────────────────────┐
│     Analytics Module (internal/analytics/)       │
│  Interface: LogAuctionObject(*AuctionObject)     │
└─────────────────┬────────────────────────────────┘
                  │
        ┌─────────┴─────────┬────────────────┐
        ▼                   ▼                ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ IDR Adapter  │   │DataDog Adapter│   │ Future       │
│              │   │ (planned)     │   │ Adapters     │
│ Enhanced     │   │               │   │ (BigQuery)   │
│ event format │   │ APM traces    │   │              │
└──────┬───────┘   └───────────────┘   └──────────────┘
       │
       ▼
┌──────────────┐
│ IDR Service  │
│ (ML Pipeline)│
└──────────────┘
```

## Key Features

### 1. Rich Auction Data Model

The `AuctionObject` provides complete auction context:

- **Bidder Selection**: Explicit tracking of excluded bidders with reasons
- **Validation Errors**: Detailed bid validation failure tracking
- **Floor Adjustments**: Platform bid multiplier and floor adjustments
- **Revenue Attribution**: Platform cuts and publisher payouts
- **Privacy Compliance**: GDPR/CCPA consent status

### 2. Multi-Sink Broadcasting

The `MultiModule` broadcasts analytics to multiple adapters simultaneously:

- Errors in one adapter don't affect others (fail-independently)
- Easy to add new adapters without touching core auction code
- Non-blocking execution

### 3. Enhanced IDR Events

The IDR adapter sends three event types to the IDR service:

1. **AuctionEvent**: Auction-level analytics with summary data
2. **BidderEvent**: Per-bidder analytics for ML model training
3. **WinEvent**: Winning bid analytics for revenue tracking

## Usage

### Server Initialization

Analytics is automatically initialized in `cmd/server/server.go` when IDR is enabled:

```go
// Analytics automatically initialized if IDR is enabled
if s.config.IDREnabled && s.config.IDRUrl != "" {
    // IDR adapter created and added to multi-module
    // Set in exchange config
}
```

### Configuration

Analytics is controlled via environment variables:

```bash
# Enable IDR (also enables analytics)
IDR_ENABLED=true
IDR_URL=http://localhost:5050
IDR_API_KEY=your-api-key
```

### Adding a New Analytics Adapter

To add a new adapter (e.g., DataDog):

1. **Create adapter implementation**:

```go
// internal/analytics/datadog/adapter.go
package datadog

import (
    "context"
    "github.com/thenexusengine/tne_springwire/internal/analytics"
)

type Adapter struct {
    tracer Tracer
}

func (a *Adapter) LogAuctionObject(ctx context.Context, auction *analytics.AuctionObject) error {
    span, _ := a.tracer.StartSpanFromContext(ctx, "auction")
    defer span.Finish()

    span.SetTag("auction.id", auction.AuctionID)
    span.SetTag("auction.bids", auction.TotalBids)
    span.SetTag("auction.revenue", auction.TotalRevenue)
    // ... more tags ...

    return nil
}

func (a *Adapter) LogVideoObject(ctx context.Context, video *analytics.VideoObject) error {
    // Implement video tracking
    return nil
}

func (a *Adapter) Shutdown() error {
    return nil
}
```

2. **Register in server initialization**:

```go
// cmd/server/server.go - in initExchange()

// Add DataDog adapter if enabled
if os.Getenv("DATADOG_ENABLED") == "true" {
    ddAdapter := datadog.NewAdapter(datadogAPIKey)
    analyticsModules = append(analyticsModules, ddAdapter)

    log.Info().Str("adapter", "datadog").Msg("Analytics adapter enabled")
}
```

That's it! The multi-module will automatically broadcast to your new adapter.

## AuctionObject Fields

### Request Context
- `AuctionID`: Unique auction identifier
- `RequestID`: Request ID from OpenRTB
- `PublisherID`: Publisher identifier
- `PublisherDomain`: Site domain or app bundle
- `Timestamp`: Auction start time

### Request Details
- `Impressions[]`: Impression details (media types, sizes, floors)
- `Device`: Device information (type, country, region)
- `User`: User information (privacy-safe)
- `TMax`: Timeout in milliseconds
- `Currency`: Bid currency

### Bidder Selection
- `SelectedBidders[]`: Bidders selected for auction
- `ExcludedBidders{}`: Map of excluded bidders with reasons
- `TotalBidders`: Total available bidders

### Bidding Results
- `BidderResults{}`: Per-bidder results with latency, bids, errors
- `WinningBids[]`: Winning bids with revenue details
- `TotalBids`: Total bids received

### Auction Outcome
- `AuctionDuration`: Total auction latency
- `Status`: "success", "no_bids", or "error"

### Revenue/Margin
- `BidMultiplier`: Platform bid multiplier applied
- `FloorAdjustments{}`: Adjusted floors per impression
- `TotalRevenue`: Platform revenue (sum of cuts)
- `TotalPayout`: Publisher payout

### Privacy
- `GDPR`: GDPR consent data
- `CCPA`: CCPA consent data
- `COPPA`: COPPA flag
- `ConsentOK`: Overall consent status

### Errors
- `ValidationErrors[]`: Bid validation failures
- `RequestErrors[]`: Request-level errors
- `BidderErrors{}`: Per-bidder errors

## Enhanced IDR Event Format

### AuctionEvent (New)

Auction-level summary sent once per auction:

```json
{
  "auction_id": "abc123",
  "request_id": "req456",
  "publisher_id": "pub789",
  "timestamp": "2024-01-15T10:30:00Z",
  "impression_count": 1,
  "bidders_selected": 5,
  "bidders_excluded": 2,
  "total_bids": 3,
  "winning_bids": 1,
  "duration_ms": 150,
  "status": "success",
  "bid_multiplier": 1.05,
  "total_revenue": 0.25,
  "total_payout": 2.25,
  "device": {
    "country": "US",
    "type": "mobile"
  },
  "consent_ok": true,
  "validation_errors": 0
}
```

### BidderEvent (Enhanced)

Per-bidder analytics for ML model:

```json
{
  "auction_id": "abc123",
  "bidder_code": "rubicon",
  "latency_ms": 50,
  "had_bid": true,
  "bid_count": 1,
  "first_bid_cpm": 2.50,
  "floor_price": 1.00,
  "below_floor": false,
  "timed_out": false,
  "had_error": false,
  "country": "US",
  "device_type": "mobile",
  "media_type": "banner"
}
```

### WinEvent (New)

Winning bid for revenue tracking:

```json
{
  "auction_id": "abc123",
  "bid_id": "bid789",
  "imp_id": "imp1",
  "bidder_code": "rubicon",
  "original_cpm": 2.50,
  "adjusted_cpm": 2.63,
  "platform_cut": 0.13,
  "clear_price": 2.63,
  "demand_type": "platform"
}
```

## Benefits

### 1. Single Source of Truth

**Before** (scattered across exchange.go):
```go
// Line 450
logger.Log.Debug().Str("bidder", bidder).Msg("Excluded by circuit breaker")

// Line 820
eventRecorder.RecordBidResponse(auctionID, bidder, latency, hadBid, ...)

// Line 1200
logger.Log.Debug().Float64("bid", price).Msg("Bid below floor")
```

**After** (single call):
```go
// Line 1500
analytics.LogAuctionObject(ctx, auctionObj) // Contains all data
```

### 2. Easy Extensibility

Adding DataDog tracing requires zero changes to exchange code:

```go
// Just add adapter to server initialization
ddAdapter := datadog.NewAdapter(apiKey)
analyticsModules = append(analyticsModules, ddAdapter)
```

### 3. Rich Analytics Queries

Examples of queries now possible:

- **"Why didn't Rubicon bid?"** → Check `excludedBidders["rubicon"]`
- **"Which DSPs give best margins?"** → Query `WinningBid.PlatformCut` by bidder
- **"Are floors too aggressive?"** → Compare `FloorAdjustments` vs. bid prices
- **"Which bidders violate OpenRTB most?"** → Count `ValidationErrors` by bidder

### 4. Privacy Compliance

All sensitive data is filtered before analytics:
- IP addresses hashed/anonymized
- User IDs are buyer UIDs only
- GDPR/CCPA consent tracked explicitly

## Testing

### Unit Tests

```bash
# Test analytics module
go test -v ./internal/analytics/...

# Test IDR adapter
go test -v ./internal/analytics/idr/...
```

### Integration Tests

```bash
# Test exchange integration
go test -v ./internal/exchange -run TestExchangeRunAuction
```

### Manual Testing

```bash
# Start server with analytics enabled
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=test-key \
go run cmd/server/main.go

# Send test auction request
curl -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d @test/fixtures/auction_request.json
```

## Performance

The analytics module adds minimal overhead:

- **Latency**: < 1ms per auction (object construction)
- **Memory**: Reuses existing data structures (no deep copies)
- **CPU**: Single method call vs. multiple scattered calls
- **Network**: Non-blocking sends to adapters

## Migration Path

The implementation supports gradual rollout:

1. **Phase 1**: Deploy with analytics disabled (current state)
2. **Phase 2**: Enable on 10% of traffic with feature flag
3. **Phase 3**: Monitor metrics and validate data quality
4. **Phase 4**: Full rollout to 100%
5. **Phase 5**: Remove old `eventRecorder` direct calls

## Future Enhancements

### Planned Adapters

1. **DataDog APM** (Month 2)
   - Distributed tracing
   - Custom metrics
   - Real-time alerting

2. **BigQuery Export** (Month 3)
   - Historical analytics
   - Publisher reports
   - Long-term trends

3. **Custom Dashboard** (Month 3)
   - Real-time auction view
   - Publisher-specific analytics
   - Revenue tracking

### Planned Features

1. **Enhanced Revenue Tracking**
   - Actual platform cuts calculated
   - Publisher payouts tracked
   - Margin analysis

2. **Better Validation Tracking**
   - Detailed validation errors
   - Per-field validation failures
   - Automatic remediation hints

3. **Consent Tracking**
   - Per-bidder consent checks
   - GDPR/CCPA compliance scoring
   - Privacy violation detection

## Troubleshooting

### Analytics not logging

Check that IDR is enabled:
```bash
echo $IDR_ENABLED  # Should be "true"
echo $IDR_URL      # Should be set
```

Check logs for analytics errors:
```bash
grep "Analytics" server.log
```

### Events not reaching IDR service

Check IDR service health:
```bash
curl http://localhost:5050/health
```

Check network connectivity:
```bash
curl -X POST http://localhost:5050/api/events/auction \
  -H "Content-Type: application/json" \
  -d '{"auction_id":"test"}'
```

### High memory usage

Analytics reuses existing data structures, but if you see high memory:

1. Check buffer sizes: `EVENT_BUFFER_SIZE` (default: 100)
2. Monitor analytics module metrics
3. Check for memory leaks in custom adapters

## Contributing

When adding new analytics fields:

1. Update `analytics.AuctionObject` in `core.go`
2. Update IDR adapter in `idr/adapter.go` to extract field
3. Update IDR event structs in `idr/events.go`
4. Add tests for new fields
5. Update this README

## License

Internal use only - The Nexus Engine
