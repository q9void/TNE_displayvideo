# IDR Service - Analytics Endpoints Implementation

## Overview

Add three new endpoints to your Python IDR service to receive enhanced analytics events.

## 🎯 Quick Implementation (Flask)

### 1. Add Endpoints to Your Flask App

Add this to your IDR service (e.g., `app.py` or `routes.py`):

```python
from flask import Flask, request, jsonify
from datetime import datetime
import logging

logger = logging.getLogger(__name__)

# ============================================
# ANALYTICS ENDPOINTS (NEW)
# ============================================

@app.route('/api/events/auction', methods=['POST'])
def receive_auction_event():
    """
    Receive auction-level analytics
    This is called ONCE per auction with summary data
    """
    try:
        event = request.json

        # Extract auction data
        auction_id = event['auction_id']
        publisher_id = event['publisher_id']
        bidders_selected = event['bidders_selected']
        bidders_excluded = event['bidders_excluded']
        total_bids = event['total_bids']
        winning_bids = event['winning_bids']
        duration_ms = event['duration_ms']
        status = event['status']
        total_revenue = event['total_revenue']
        total_payout = event['total_payout']

        logger.info(f"Auction {auction_id}: {bidders_selected} bidders, {total_bids} bids, {winning_bids} wins, revenue=${total_revenue:.2f}")

        # Store in database (see schema below)
        store_auction_event(event)

        # Update real-time metrics
        update_auction_metrics(event)

        return jsonify({"status": "ok", "auction_id": auction_id}), 200

    except Exception as e:
        logger.error(f"Failed to process auction event: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500


@app.route('/api/events/bidder', methods=['POST'])
def receive_bidder_event():
    """
    Receive per-bidder analytics
    This is called ONCE per bidder per auction (for ML training)
    """
    try:
        event = request.json

        # Extract bidder data
        auction_id = event['auction_id']
        bidder_code = event['bidder_code']
        latency_ms = event['latency_ms']
        had_bid = event['had_bid']
        first_bid_cpm = event.get('first_bid_cpm')
        floor_price = event.get('floor_price')
        below_floor = event['below_floor']
        timed_out = event['timed_out']

        logger.debug(f"Bidder {bidder_code} in auction {auction_id}: bid={had_bid}, latency={latency_ms}ms")

        # Store for ML training
        store_bidder_event(event)

        # Update bidder performance metrics
        update_bidder_metrics(bidder_code, event)

        # Train ML model asynchronously
        enqueue_ml_training(event)

        return jsonify({"status": "ok"}), 200

    except Exception as e:
        logger.error(f"Failed to process bidder event: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500


@app.route('/api/events/win', methods=['POST'])
def receive_win_event():
    """
    Receive winning bid analytics
    This is called ONCE per winning bid (for revenue tracking)
    """
    try:
        event = request.json

        # Extract win data
        auction_id = event['auction_id']
        bid_id = event['bid_id']
        bidder_code = event['bidder_code']
        original_cpm = event['original_cpm']
        adjusted_cpm = event['adjusted_cpm']
        platform_cut = event['platform_cut']
        demand_type = event['demand_type']

        logger.info(f"Win {bid_id} by {bidder_code}: ${adjusted_cpm:.2f} CPM, platform cut=${platform_cut:.2f}")

        # Store for revenue tracking
        store_win_event(event)

        # Update revenue metrics
        update_revenue_metrics(event)

        # Trigger revenue report update
        update_revenue_reports(bidder_code, platform_cut)

        return jsonify({"status": "ok", "bid_id": bid_id}), 200

    except Exception as e:
        logger.error(f"Failed to process win event: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500


# ============================================
# HELPER FUNCTIONS
# ============================================

def store_auction_event(event):
    """Store auction event in database"""
    # Example: PostgreSQL
    query = """
        INSERT INTO auction_events (
            auction_id, publisher_id, timestamp,
            bidders_selected, bidders_excluded, total_bids, winning_bids,
            duration_ms, status, total_revenue, total_payout,
            device_country, device_type, consent_ok
        ) VALUES (
            %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
        )
    """
    params = (
        event['auction_id'],
        event['publisher_id'],
        event['timestamp'],
        event['bidders_selected'],
        event['bidders_excluded'],
        event['total_bids'],
        event['winning_bids'],
        event['duration_ms'],
        event['status'],
        event['total_revenue'],
        event['total_payout'],
        event['device']['country'],
        event['device']['type'],
        event['consent_ok']
    )
    db.execute(query, params)


def store_bidder_event(event):
    """Store bidder event in database (for ML training)"""
    query = """
        INSERT INTO bidder_events (
            auction_id, bidder_code, latency_ms, had_bid, bid_count,
            first_bid_cpm, floor_price, below_floor, timed_out,
            country, device_type, media_type
        ) VALUES (
            %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
        )
    """
    params = (
        event['auction_id'],
        event['bidder_code'],
        event['latency_ms'],
        event['had_bid'],
        event['bid_count'],
        event.get('first_bid_cpm'),
        event.get('floor_price'),
        event['below_floor'],
        event['timed_out'],
        event['country'],
        event['device_type'],
        event['media_type']
    )
    db.execute(query, params)


def store_win_event(event):
    """Store win event in database (for revenue tracking)"""
    query = """
        INSERT INTO win_events (
            auction_id, bid_id, imp_id, bidder_code,
            original_cpm, adjusted_cpm, platform_cut, clear_price,
            demand_type, timestamp
        ) VALUES (
            %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW()
        )
    """
    params = (
        event['auction_id'],
        event['bid_id'],
        event['imp_id'],
        event['bidder_code'],
        event['original_cpm'],
        event['adjusted_cpm'],
        event['platform_cut'],
        event['clear_price'],
        event['demand_type']
    )
    db.execute(query, params)


def update_auction_metrics(event):
    """Update real-time auction metrics"""
    # Update in-memory metrics or Redis
    metrics = {
        'auctions_total': 1,
        'auctions_with_bids': 1 if event['total_bids'] > 0 else 0,
        'total_revenue': event['total_revenue'],
        'avg_duration_ms': event['duration_ms']
    }
    # redis.hincrby('auction_metrics', 'auctions_total', 1)
    # redis.hincrbyfloat('auction_metrics', 'total_revenue', event['total_revenue'])


def update_bidder_metrics(bidder_code, event):
    """Update per-bidder performance metrics"""
    # Track bidder performance for partner selection
    key = f"bidder:{bidder_code}"
    # redis.hincrby(key, 'requests', 1)
    # if event['had_bid']:
    #     redis.hincrby(key, 'bids', 1)
    # redis.hset(key, 'avg_latency_ms', event['latency_ms'])


def update_revenue_metrics(event):
    """Update revenue tracking metrics"""
    # Track revenue by bidder
    key = f"revenue:{event['bidder_code']}"
    # redis.hincrbyfloat(key, 'total', event['platform_cut'])
    # redis.hincrby(key, 'wins', 1)


def enqueue_ml_training(event):
    """Enqueue bidder event for ML model training"""
    # Add to queue for async ML training
    # training_queue.put(event)
    pass


def update_revenue_reports(bidder_code, platform_cut):
    """Update revenue reports"""
    # Update daily/monthly revenue reports
    pass


# ============================================
# LEGACY ENDPOINT (Keep for backwards compatibility)
# ============================================

@app.route('/api/events', methods=['POST'])
def receive_legacy_events():
    """
    Legacy endpoint for old event format
    Keep this for backwards compatibility during migration
    """
    try:
        data = request.json
        events = data.get('events', [])

        for event in events:
            # Process old format events
            process_legacy_event(event)

        return jsonify({"status": "ok", "processed": len(events)}), 200

    except Exception as e:
        logger.error(f"Failed to process legacy events: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500
```

---

## 📊 Database Schema

### PostgreSQL Schema

```sql
-- Auction-level events
CREATE TABLE auction_events (
    id BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    request_id VARCHAR(255),
    publisher_id VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP NOT NULL,

    -- Bidder selection
    bidders_selected INTEGER NOT NULL,
    bidders_excluded INTEGER NOT NULL,
    total_bidders INTEGER NOT NULL,

    -- Auction results
    total_bids INTEGER NOT NULL,
    winning_bids INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,

    -- Revenue
    bid_multiplier DECIMAL(10,4) DEFAULT 1.0,
    total_revenue DECIMAL(10,4) DEFAULT 0,
    total_payout DECIMAL(10,4) DEFAULT 0,

    -- Device/Context
    device_country VARCHAR(2),
    device_type VARCHAR(50),
    impression_count INTEGER DEFAULT 1,

    -- Privacy
    consent_ok BOOLEAN DEFAULT TRUE,
    validation_errors INTEGER DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW(),

    INDEX idx_auction_id (auction_id),
    INDEX idx_publisher_id (publisher_id),
    INDEX idx_timestamp (timestamp)
);

-- Per-bidder events (for ML training)
CREATE TABLE bidder_events (
    id BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    bidder_code VARCHAR(100) NOT NULL,

    -- Performance
    latency_ms INTEGER NOT NULL,
    had_bid BOOLEAN NOT NULL,
    bid_count INTEGER DEFAULT 0,
    first_bid_cpm DECIMAL(10,4),
    floor_price DECIMAL(10,4),
    below_floor BOOLEAN DEFAULT FALSE,

    -- Status
    timed_out BOOLEAN DEFAULT FALSE,
    had_error BOOLEAN DEFAULT FALSE,
    no_bid_reason VARCHAR(255),

    -- Context (for ML features)
    country VARCHAR(2),
    device_type VARCHAR(50),
    media_type VARCHAR(50),

    created_at TIMESTAMP DEFAULT NOW(),

    INDEX idx_auction_id (auction_id),
    INDEX idx_bidder_code (bidder_code),
    INDEX idx_created_at (created_at)
);

-- Win events (for revenue tracking)
CREATE TABLE win_events (
    id BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    bid_id VARCHAR(255) NOT NULL,
    imp_id VARCHAR(255) NOT NULL,
    bidder_code VARCHAR(100) NOT NULL,

    -- Pricing
    original_cpm DECIMAL(10,4) NOT NULL,
    adjusted_cpm DECIMAL(10,4) NOT NULL,
    platform_cut DECIMAL(10,4) NOT NULL,
    clear_price DECIMAL(10,4) NOT NULL,

    -- Metadata
    demand_type VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),

    INDEX idx_auction_id (auction_id),
    INDEX idx_bidder_code (bidder_code),
    INDEX idx_created_at (created_at)
);

-- Revenue aggregation view
CREATE VIEW revenue_by_bidder AS
SELECT
    bidder_code,
    COUNT(*) as wins,
    SUM(platform_cut) as total_revenue,
    AVG(platform_cut) as avg_revenue,
    DATE(created_at) as date
FROM win_events
GROUP BY bidder_code, DATE(created_at);

-- Bidder performance view
CREATE VIEW bidder_performance AS
SELECT
    bidder_code,
    COUNT(*) as total_requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as total_bids,
    AVG(latency_ms) as avg_latency,
    SUM(CASE WHEN timed_out THEN 1 ELSE 0 END) as timeouts,
    DATE(created_at) as date
FROM bidder_events
GROUP BY bidder_code, DATE(created_at);
```

---

## 🧪 Testing

### 1. Test Auction Endpoint

```bash
curl -X POST http://localhost:5050/api/events/auction \
  -H "Content-Type: application/json" \
  -d '{
    "auction_id": "test-auction-123",
    "request_id": "req-456",
    "publisher_id": "pub-789",
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
  }'

# Expected: {"status": "ok", "auction_id": "test-auction-123"}
```

### 2. Test Bidder Endpoint

```bash
curl -X POST http://localhost:5050/api/events/bidder \
  -H "Content-Type: application/json" \
  -d '{
    "auction_id": "test-auction-123",
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
  }'

# Expected: {"status": "ok"}
```

### 3. Test Win Endpoint

```bash
curl -X POST http://localhost:5050/api/events/win \
  -H "Content-Type: application/json" \
  -d '{
    "auction_id": "test-auction-123",
    "bid_id": "bid-789",
    "imp_id": "imp-1",
    "bidder_code": "rubicon",
    "original_cpm": 2.50,
    "adjusted_cpm": 2.63,
    "platform_cut": 0.13,
    "clear_price": 2.63,
    "demand_type": "platform"
  }'

# Expected: {"status": "ok", "bid_id": "bid-789"}
```

---

## 📈 Analytics Queries

### Revenue by Bidder (Last 7 Days)

```sql
SELECT
    bidder_code,
    COUNT(*) as wins,
    SUM(platform_cut) as total_revenue,
    AVG(platform_cut) as avg_revenue_per_win
FROM win_events
WHERE created_at >= NOW() - INTERVAL '7 days'
GROUP BY bidder_code
ORDER BY total_revenue DESC;
```

### Bidder Performance

```sql
SELECT
    bidder_code,
    COUNT(*) as requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as bids,
    ROUND(100.0 * SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) / COUNT(*), 2) as bid_rate,
    ROUND(AVG(latency_ms), 0) as avg_latency_ms
FROM bidder_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY bidder_code
ORDER BY requests DESC;
```

### Auction Summary

```sql
SELECT
    COUNT(*) as total_auctions,
    AVG(total_bids) as avg_bids_per_auction,
    AVG(winning_bids) as avg_wins_per_auction,
    AVG(duration_ms) as avg_duration_ms,
    SUM(total_revenue) as total_revenue
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '24 hours';
```

---

## 🚀 Deployment Steps

### 1. Add Code to IDR Service

```bash
# Add the endpoints to your Flask app
# Update app.py or routes.py with the code above
```

### 2. Create Database Tables

```bash
# Run the SQL schema
psql -U your_user -d idr_db < schema.sql
```

### 3. Test Locally

```bash
# Start IDR service
python app.py

# Test endpoints
./test_endpoints.sh
```

### 4. Deploy IDR Service

```bash
# Deploy updated IDR service
# (Your deployment process)
```

### 5. Start PBS with Analytics

```bash
# Start PBS with analytics enabled
cd /path/to/tnevideo
IDR_ENABLED=true \
IDR_URL=http://localhost:5050 \
IDR_API_KEY=your-api-key \
go run cmd/server/main.go
```

### 6. Verify Events Flow

```bash
# Send test auction
curl -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d @test/fixtures/auction_request.json

# Check IDR received events
psql -U your_user -d idr_db -c "SELECT COUNT(*) FROM auction_events;"
psql -U your_user -d idr_db -c "SELECT COUNT(*) FROM bidder_events;"
psql -U your_user -d idr_db -c "SELECT COUNT(*) FROM win_events;"
```

---

## 🎯 What You Get

✅ **Auction-level summaries** - Track overall performance
✅ **Per-bidder analytics** - Train ML models better
✅ **Revenue tracking** - Know your margins
✅ **Rich data** - Country, device type, media type, etc.
✅ **Performance metrics** - Latency, timeouts, errors
✅ **Better insights** - Query by any dimension

---

## 🔧 Optional Enhancements

### Add Redis Caching

```python
import redis

redis_client = redis.Redis(host='localhost', port=6379, db=0)

def update_auction_metrics(event):
    # Real-time metrics in Redis
    redis_client.hincrby('metrics:auctions', 'total', 1)
    redis_client.hincrbyfloat('metrics:revenue', 'total', event['total_revenue'])
```

### Add Async Processing

```python
from celery import Celery

celery = Celery('idr', broker='redis://localhost:6379/0')

@celery.task
def process_bidder_event_async(event):
    """Process bidder event asynchronously"""
    store_bidder_event(event)
    train_ml_model(event)

# In endpoint:
process_bidder_event_async.delay(event)
```

### Add Monitoring

```python
from prometheus_client import Counter, Histogram

auction_events = Counter('idr_auction_events_total', 'Total auction events')
bidder_events = Counter('idr_bidder_events_total', 'Total bidder events')
event_latency = Histogram('idr_event_processing_seconds', 'Event processing time')

@event_latency.time()
def store_auction_event(event):
    # ... store event ...
    auction_events.inc()
```

---

## ✅ Ready to Deploy!

Your IDR service is now ready to receive enhanced analytics. The PBS server will automatically send rich event data to these endpoints.

**Next**: Start the PBS server and watch the events flow! 🎉
