#!/bin/bash
# Quick Analytics Viewer
# Run: ./view_analytics.sh

echo "==========================================="
echo "  TNE Analytics Dashboard"
echo "==========================================="
echo ""

echo "📊 REVENUE BY BIDDER (All Time)"
echo "-------------------------------------------"
psql -U andrewstreets -d idr_db -c "
SELECT
    bidder_code,
    COUNT(*) as wins,
    ROUND(SUM(platform_cut)::numeric, 2) as revenue,
    ROUND(AVG(adjusted_cpm)::numeric, 2) as avg_cpm
FROM win_events
GROUP BY bidder_code
ORDER BY revenue DESC;
" -q

echo ""
echo "🎯 BIDDER PERFORMANCE"
echo "-------------------------------------------"
psql -U andrewstreets -d idr_db -c "
SELECT
    bidder_code,
    COUNT(*) as requests,
    SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as bids,
    ROUND(100.0 * SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) / COUNT(*), 1) || '%' as bid_rate,
    ROUND(AVG(latency_ms)::numeric, 0) || 'ms' as avg_latency
FROM bidder_events
GROUP BY bidder_code
ORDER BY requests DESC;
" -q

echo ""
echo "📈 RECENT AUCTIONS"
echo "-------------------------------------------"
psql -U andrewstreets -d idr_db -c "
SELECT
    auction_id,
    publisher_id,
    bidders_selected as bidders,
    total_bids as bids,
    winning_bids as wins,
    duration_ms || 'ms' as latency,
    ROUND(total_revenue::numeric, 2) as revenue,
    device_country as country
FROM auction_events
ORDER BY created_at DESC
LIMIT 10;
" -q

echo ""
echo "💰 TOTAL STATS"
echo "-------------------------------------------"
psql -U andrewstreets -d idr_db -c "
SELECT
    COUNT(DISTINCT auction_id) as total_auctions,
    SUM(total_bids) as total_bids,
    SUM(winning_bids) as total_wins,
    ROUND(AVG(duration_ms)::numeric, 0) || 'ms' as avg_latency,
    ROUND(SUM(total_revenue)::numeric, 2) as total_revenue
FROM auction_events;
" -q

echo ""
echo "==========================================="
echo "  Run './view_analytics.sh' anytime!"
echo "==========================================="
