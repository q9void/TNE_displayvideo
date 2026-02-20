-- Seed data for bizbudding / Total Sports Pro configuration
-- Uses the EXISTING domain_bidder_configs schema (migration 005)

-- Insert bidder configs for totalprosports.com
INSERT INTO domain_bidder_configs (publisher_id, domain, bidder_code, params)
VALUES
('totalsportspro', 'totalprosports.com', 'rubicon', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb),
('totalsportspro', 'totalprosports.com', 'kargo', '{"placementId": "_o9n8eh8Lsw"}'::jsonb),
('totalsportspro', 'totalprosports.com', 'sovrn', '{"tagid": "1294952"}'::jsonb),
('totalsportspro', 'totalprosports.com', 'pubmatic', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb),
('totalsportspro', 'totalprosports.com', 'triplelift', '{"inventoryCode": "totalprosports_billboard_prebid"}'::jsonb)
ON CONFLICT (publisher_id, domain, bidder_code) DO UPDATE
SET params = EXCLUDED.params;

-- Also add for dev subdomain
INSERT INTO domain_bidder_configs (publisher_id, domain, bidder_code, params)
VALUES
('totalsportspro', 'dev.totalprosports.com', 'rubicon', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb),
('totalsportspro', 'dev.totalprosports.com', 'kargo', '{"placementId": "_o9n8eh8Lsw"}'::jsonb),
('totalsportspro', 'dev.totalprosports.com', 'sovrn', '{"tagid": "1294952"}'::jsonb),
('totalsportspro', 'dev.totalprosports.com', 'pubmatic', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb),
('totalsportspro', 'dev.totalprosports.com', 'triplelift', '{"inventoryCode": "totalprosports_billboard_prebid"}'::jsonb)
ON CONFLICT (publisher_id, domain, bidder_code) DO UPDATE
SET params = EXCLUDED.params;

-- Verification
SELECT
    publisher_id,
    domain,
    COUNT(*) as bidder_count,
    array_agg(bidder_code ORDER BY bidder_code) as bidders
FROM domain_bidder_configs
WHERE publisher_id = 'totalsportspro'
GROUP BY publisher_id, domain
ORDER BY domain;
