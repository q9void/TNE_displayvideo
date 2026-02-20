-- ============================================
-- Analytics Queries - Quick Reference
-- ============================================

-- 1. REVENUE DASHBOARD
-- Total revenue in last 24 hours by bidder
SELECT
    bidder_code,
    COUNT(*) as wins,
    ROUND(SUM(platform_cut)::numeric, 2) as total_revenue,
    ROUND(AVG(platform_cut)::numeric, 2) as avg_revenue_per_win,
    ROUND(AVG(adjusted_cpm)::numeric, 2) as avg_cpm
FROM win_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY bidder_code
ORDER BY total_revenue DESC;

-- 2. BIDDER PERFORMANCE
-- Win rate, bid rate, and latency by bidder
SELECT
    b.bidder_code,
    COUNT(DISTINCT b.auction_id) as auctions_participated,
    SUM(CASE WHEN b.had_bid THEN 1 ELSE 0 END) as total_bids,
    ROUND(100.0 * SUM(CASE WHEN b.had_bid THEN 1 ELSE 0 END) / COUNT(*), 2) as bid_rate_pct,
    COUNT(w.bid_id) as wins,
    ROUND(AVG(b.latency_ms)::numeric, 0) as avg_latency_ms,
    SUM(CASE WHEN b.timed_out THEN 1 ELSE 0 END) as timeouts
FROM bidder_events b
LEFT JOIN win_events w ON b.auction_id = w.auction_id AND b.bidder_code = w.bidder_code
WHERE b.created_at >= NOW() - INTERVAL '24 hours'
GROUP BY b.bidder_code
ORDER BY total_bids DESC;

-- 3. AUCTION SUMMARY
-- Hourly auction performance
SELECT
    DATE_TRUNC('hour', timestamp) as hour,
    COUNT(*) as total_auctions,
    ROUND(AVG(total_bids)::numeric, 1) as avg_bids_per_auction,
    ROUND(AVG(winning_bids)::numeric, 1) as avg_wins_per_auction,
    ROUND(AVG(duration_ms)::numeric, 0) as avg_duration_ms,
    ROUND(SUM(total_revenue)::numeric, 2) as hourly_revenue,
    ROUND(100.0 * SUM(CASE WHEN winning_bids > 0 THEN 1 ELSE 0 END) / COUNT(*), 2) as fill_rate_pct
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', timestamp)
ORDER BY hour DESC;

-- 4. PUBLISHER REVENUE
-- Revenue by publisher
SELECT
    a.publisher_id,
    COUNT(DISTINCT a.auction_id) as auctions,
    SUM(a.winning_bids) as total_wins,
    ROUND(SUM(a.total_revenue)::numeric, 2) as platform_revenue,
    ROUND(SUM(a.total_payout)::numeric, 2) as publisher_payout,
    ROUND(AVG(a.bid_multiplier)::numeric, 3) as avg_multiplier
FROM auction_events a
WHERE a.timestamp >= NOW() - INTERVAL '7 days'
GROUP BY a.publisher_id
ORDER BY platform_revenue DESC;

-- 5. GEOGRAPHIC PERFORMANCE
-- Performance by country
SELECT
    a.device_country as country,
    COUNT(*) as auctions,
    SUM(a.total_bids) as total_bids,
    ROUND(AVG(a.duration_ms)::numeric, 0) as avg_latency_ms,
    ROUND(SUM(a.total_revenue)::numeric, 2) as revenue
FROM auction_events a
WHERE a.timestamp >= NOW() - INTERVAL '7 days'
    AND a.device_country IS NOT NULL
GROUP BY a.device_country
ORDER BY auctions DESC
LIMIT 20;

-- 6. DEVICE TYPE PERFORMANCE
-- Mobile vs Desktop performance
SELECT
    a.device_type,
    COUNT(*) as auctions,
    ROUND(AVG(a.total_bids)::numeric, 1) as avg_bids,
    ROUND(100.0 * SUM(CASE WHEN a.winning_bids > 0 THEN 1 ELSE 0 END) / COUNT(*), 2) as fill_rate_pct,
    ROUND(SUM(a.total_revenue)::numeric, 2) as revenue
FROM auction_events a
WHERE a.timestamp >= NOW() - INTERVAL '7 days'
GROUP BY a.device_type
ORDER BY auctions DESC;

-- 7. REAL-TIME MONITORING (Last 5 minutes)
-- Recent auction activity
SELECT
    auction_id,
    publisher_id,
    timestamp,
    bidders_selected,
    total_bids,
    winning_bids,
    duration_ms,
    ROUND(total_revenue::numeric, 2) as revenue,
    status
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '5 minutes'
ORDER BY timestamp DESC
LIMIT 50;

-- 8. BIDDER COMPARISON
-- Side-by-side bidder comparison
SELECT
    bidder_code,
    COUNT(*) as requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as bids,
    ROUND(100.0 * SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) / COUNT(*), 2) as bid_rate,
    ROUND(AVG(latency_ms)::numeric, 0) as avg_latency,
    SUM(CASE WHEN timed_out THEN 1 ELSE 0 END) as timeouts,
    SUM(CASE WHEN had_error THEN 1 ELSE 0 END) as errors
FROM bidder_events
WHERE created_at >= NOW() - INTERVAL '1 hour'
GROUP BY bidder_code
ORDER BY requests DESC;

-- 9. FLOOR PRICE ANALYSIS
-- How bids compare to floor prices
SELECT
    CASE
        WHEN floor_price IS NULL THEN 'No Floor'
        WHEN floor_price < 1.0 THEN '< $1.00'
        WHEN floor_price < 2.0 THEN '$1.00 - $2.00'
        WHEN floor_price < 5.0 THEN '$2.00 - $5.00'
        ELSE '> $5.00'
    END as floor_range,
    COUNT(*) as bid_attempts,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as bids,
    SUM(CASE WHEN below_floor THEN 1 ELSE 0 END) as below_floor_count,
    ROUND(AVG(first_bid_cpm)::numeric, 2) as avg_bid_cpm
FROM bidder_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY floor_range
ORDER BY
    CASE floor_range
        WHEN 'No Floor' THEN 0
        WHEN '< $1.00' THEN 1
        WHEN '$1.00 - $2.00' THEN 2
        WHEN '$2.00 - $5.00' THEN 3
        ELSE 4
    END;

-- 10. DAILY REVENUE REPORT
-- Daily revenue trend
SELECT
    DATE(created_at) as date,
    bidder_code,
    COUNT(*) as wins,
    ROUND(SUM(platform_cut)::numeric, 2) as daily_revenue,
    ROUND(AVG(adjusted_cpm)::numeric, 2) as avg_cpm
FROM win_events
WHERE created_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at), bidder_code
ORDER BY date DESC, daily_revenue DESC;

-- ============================================
-- EXPORT QUERIES (For Dashboards)
-- ============================================

-- Export for Excel/CSV
COPY (
    SELECT
        DATE(timestamp) as date,
        publisher_id,
        COUNT(*) as auctions,
        SUM(total_bids) as bids,
        SUM(winning_bids) as wins,
        ROUND(SUM(total_revenue)::numeric, 2) as revenue
    FROM auction_events
    WHERE timestamp >= NOW() - INTERVAL '30 days'
    GROUP BY DATE(timestamp), publisher_id
    ORDER BY date DESC
) TO '/tmp/analytics_export.csv' WITH CSV HEADER;
