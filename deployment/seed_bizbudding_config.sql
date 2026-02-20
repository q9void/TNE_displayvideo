-- Seed data for bizbudding / Total Sports Pro configuration
-- Based on config/bizbudding-all-bidders-mapping.json

-- 1. Create account (maps SDK accountId '12345' to publisher 'icisic-media')
INSERT INTO accounts (account_id, name, status) VALUES
('12345', 'ICISIC Media / Total Sports Pro', 'active')
ON CONFLICT (account_id) DO NOTHING;

-- 2. Create publisher under this account
INSERT INTO publishers_new (account_id, domain, name, default_timeout_ms, status)
SELECT
    a.id,
    'totalprosports.com',
    'Total Sports Pro',
    2800,
    'active'
FROM accounts a
WHERE a.account_id = '12345'
ON CONFLICT (account_id, domain) DO UPDATE
SET name = EXCLUDED.name,
    default_timeout_ms = EXCLUDED.default_timeout_ms;

-- Also add dev subdomain
INSERT INTO publishers_new (account_id, domain, name, default_timeout_ms, status)
SELECT
    a.id,
    'dev.totalprosports.com',
    'Total Sports Pro (Dev)',
    2800,
    'active'
FROM accounts a
WHERE a.account_id = '12345'
ON CONFLICT (account_id, domain) DO UPDATE
SET name = EXCLUDED.name,
    default_timeout_ms = EXCLUDED.default_timeout_ms;

-- 3. Create domain-level bidder configs (fallback for all slots)
WITH pub AS (
    SELECT p.id as publisher_id
    FROM publishers_new p
    JOIN accounts a ON p.account_id = a.id
    WHERE a.account_id = '12345' AND p.domain = 'totalprosports.com'
)
INSERT INTO domain_bidder_configs (publisher_id, bidder_code, enabled, params)
SELECT
    pub.publisher_id,
    'rubicon',
    true,
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'kargo',
    true,
    '{"placementId": "_o9n8eh8Lsw"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'sovrn',
    true,
    '{"tagid": "1294952"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'pubmatic',
    true,
    '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'triplelift',
    true,
    '{"inventoryCode": "totalprosports_billboard_prebid"}'::jsonb
FROM pub
ON CONFLICT (publisher_id, bidder_code) DO UPDATE
SET params = EXCLUDED.params,
    enabled = EXCLUDED.enabled;

-- 4. Also add configs for dev subdomain
WITH pub AS (
    SELECT p.id as publisher_id
    FROM publishers_new p
    JOIN accounts a ON p.account_id = a.id
    WHERE a.account_id = '12345' AND p.domain = 'dev.totalprosports.com'
)
INSERT INTO domain_bidder_configs (publisher_id, bidder_code, enabled, params)
SELECT
    pub.publisher_id,
    'rubicon',
    true,
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'kargo',
    true,
    '{"placementId": "_o9n8eh8Lsw"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'sovrn',
    true,
    '{"tagid": "1294952"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'pubmatic',
    true,
    '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb
FROM pub
UNION ALL
SELECT
    pub.publisher_id,
    'triplelift',
    true,
    '{"inventoryCode": "totalprosports_billboard_prebid"}'::jsonb
FROM pub
ON CONFLICT (publisher_id, bidder_code) DO UPDATE
SET params = EXCLUDED.params,
    enabled = EXCLUDED.enabled;

-- Verification
SELECT
    a.account_id,
    p.domain,
    COUNT(DISTINCT dbc.bidder_code) as bidder_count
FROM accounts a
JOIN publishers_new p ON p.account_id = a.id
LEFT JOIN domain_bidder_configs dbc ON dbc.publisher_id = p.id AND dbc.enabled = true
WHERE a.account_id = '12345'
GROUP BY a.account_id, p.domain
ORDER BY p.domain;
