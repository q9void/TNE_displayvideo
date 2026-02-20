-- Analytics Database Schema
-- Run this to create the tables for storing analytics events

-- Auction-level events
CREATE TABLE IF NOT EXISTS auction_events (
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

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auction_events_auction_id ON auction_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_auction_events_publisher_id ON auction_events(publisher_id);
CREATE INDEX IF NOT EXISTS idx_auction_events_timestamp ON auction_events(timestamp);

-- Per-bidder events (for ML training)
CREATE TABLE IF NOT EXISTS bidder_events (
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

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bidder_events_auction_id ON bidder_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_bidder_events_bidder_code ON bidder_events(bidder_code);
CREATE INDEX IF NOT EXISTS idx_bidder_events_created_at ON bidder_events(created_at);

-- Win events (for revenue tracking)
CREATE TABLE IF NOT EXISTS win_events (
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
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_win_events_auction_id ON win_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_win_events_bidder_code ON win_events(bidder_code);
CREATE INDEX IF NOT EXISTS idx_win_events_created_at ON win_events(created_at);

-- Revenue aggregation view
CREATE OR REPLACE VIEW revenue_by_bidder AS
SELECT
    bidder_code,
    COUNT(*) as wins,
    SUM(platform_cut) as total_revenue,
    AVG(platform_cut) as avg_revenue,
    DATE(created_at) as date
FROM win_events
GROUP BY bidder_code, DATE(created_at);

-- Bidder performance view
CREATE OR REPLACE VIEW bidder_performance AS
SELECT
    bidder_code,
    COUNT(*) as total_requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as total_bids,
    AVG(latency_ms) as avg_latency,
    SUM(CASE WHEN timed_out THEN 1 ELSE 0 END) as timeouts,
    DATE(created_at) as date
FROM bidder_events
GROUP BY bidder_code, DATE(created_at);

-- Insert some sample data for testing
INSERT INTO auction_events (
    auction_id, request_id, publisher_id, timestamp,
    bidders_selected, bidders_excluded, total_bidders,
    total_bids, winning_bids, duration_ms, status,
    bid_multiplier, total_revenue, total_payout,
    device_country, device_type, impression_count,
    consent_ok, validation_errors
) VALUES
    ('demo-auction-1', 'req-001', 'pub-demo', NOW() - INTERVAL '1 hour',
     5, 2, 7, 3, 1, 150, 'success',
     1.05, 0.25, 2.25, 'US', 'mobile', 1, true, 0),
    ('demo-auction-2', 'req-002', 'pub-demo', NOW() - INTERVAL '30 minutes',
     4, 1, 5, 2, 1, 120, 'success',
     1.05, 0.18, 1.82, 'UK', 'desktop', 1, true, 0),
    ('demo-auction-3', 'req-003', 'pub-test', NOW() - INTERVAL '15 minutes',
     6, 0, 6, 4, 2, 180, 'success',
     1.10, 0.45, 4.05, 'US', 'mobile', 2, true, 0);

INSERT INTO bidder_events (
    auction_id, bidder_code, latency_ms, had_bid, bid_count,
    first_bid_cpm, floor_price, below_floor, timed_out, had_error,
    country, device_type, media_type
) VALUES
    ('demo-auction-1', 'rubicon', 45, true, 1, 2.50, 1.00, false, false, false, 'US', 'mobile', 'banner'),
    ('demo-auction-1', 'appnexus', 60, true, 1, 2.20, 1.00, false, false, false, 'US', 'mobile', 'banner'),
    ('demo-auction-1', 'pubmatic', 55, false, 0, NULL, 1.00, false, false, false, 'US', 'mobile', 'banner'),
    ('demo-auction-2', 'rubicon', 50, true, 1, 1.80, 1.00, false, false, false, 'UK', 'desktop', 'banner'),
    ('demo-auction-2', 'kargo', 70, true, 1, 1.95, 1.00, false, false, false, 'UK', 'desktop', 'banner');

INSERT INTO win_events (
    auction_id, bid_id, imp_id, bidder_code,
    original_cpm, adjusted_cpm, platform_cut, clear_price, demand_type
) VALUES
    ('demo-auction-1', 'bid-001', 'imp-1', 'rubicon', 2.50, 2.63, 0.25, 2.63, 'platform'),
    ('demo-auction-2', 'bid-002', 'imp-1', 'kargo', 1.95, 2.05, 0.18, 2.05, 'platform'),
    ('demo-auction-3', 'bid-003', 'imp-1', 'rubicon', 2.10, 2.31, 0.23, 2.31, 'platform'),
    ('demo-auction-3', 'bid-004', 'imp-2', 'appnexus', 2.00, 2.20, 0.22, 2.20, 'platform');

-- Print success message
SELECT 'Analytics database schema created successfully!' as message;
SELECT 'Tables: auction_events, bidder_events, win_events' as tables;
SELECT 'Sample data: ' || COUNT(*) || ' demo auctions inserted' as sample_data
FROM auction_events;
