-- Migration 012: Seed BizBudding / Total Sports Pro Configuration
-- Comprehensive seed data for account 12345 with normalized schema
-- Includes: account, publishers, bidders, ad slots, and slot-level bidder configs

-- ============================================================================
-- 1. Create Account
-- ============================================================================
INSERT INTO accounts (account_id, name, status) VALUES
('12345', 'BizBudding Network', 'active')
ON CONFLICT (account_id) DO UPDATE
SET name = EXCLUDED.name,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- 2. Create Publishers
-- ============================================================================
-- Production domain
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
    default_timeout_ms = EXCLUDED.default_timeout_ms,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP;

-- Dev domain
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
    default_timeout_ms = EXCLUDED.default_timeout_ms,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- 3. Create Bidders
-- ============================================================================
INSERT INTO bidders_new (code, name, status) VALUES
('rubicon', 'Magnite (Rubicon)', 'active'),
('kargo', 'Kargo', 'active'),
('sovrn', 'Sovrn', 'active'),
('oms', 'OMS (OpenMediation)', 'active'),
('aniview', 'Aniview', 'active'),
('pubmatic', 'PubMatic', 'active'),
('triplelift', 'TripleLift', 'active')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    status = EXCLUDED.status;

-- ============================================================================
-- 4. Create Media Type Profile (Standard Banner Sizes)
-- ============================================================================
INSERT INTO media_type_profiles (name, media_types, description) VALUES
('standard_banner_sizes',
 '{
   "banner": {
     "sizes": [
       [728, 90],
       [970, 250],
       [300, 250],
       [160, 600],
       [320, 50],
       [300, 600],
       [970, 90],
       [320, 100]
     ]
   }
 }'::jsonb,
 'Standard IAB banner sizes for desktop and mobile'
)
ON CONFLICT (name) DO UPDATE
SET media_types = EXCLUDED.media_types,
    description = EXCLUDED.description;

-- ============================================================================
-- 5. Create Ad Slots
-- ============================================================================
WITH pub_prod AS (
    SELECT p.id as publisher_id, p.domain
    FROM publishers_new p
    JOIN accounts a ON p.account_id = a.id
    WHERE a.account_id = '12345' AND p.domain = 'totalprosports.com'
),
pub_dev AS (
    SELECT p.id as publisher_id, p.domain
    FROM publishers_new p
    JOIN accounts a ON p.account_id = a.id
    WHERE a.account_id = '12345' AND p.domain = 'dev.totalprosports.com'
),
media_profile AS (
    SELECT id FROM media_type_profiles WHERE name = 'standard_banner_sizes'
)
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, div_pattern, is_adhesion, media_type_profile_id, status)
-- Production slots
SELECT p.publisher_id, p.domain || '/billboard', 'billboard', 'mai-ad-billboard%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/leaderboard', 'leaderboard', 'mai-ad-leaderboard%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/skyscraper', 'skyscraper', 'mai-ad-skyscraper%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/rectangle', 'rectangle', 'mai-ad-rectangle%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/mobile-banner', 'mobile-banner', 'mai-ad-mobile-banner%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/adhesion', 'adhesion', 'mai-ad-adhesion%', true, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/sidebar', 'sidebar', 'mai-ad-sidebar%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/footer', 'footer', 'mai-ad-footer%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/header', 'header', 'mai-ad-header%', false, m.id, 'active' FROM pub_prod p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/inline', 'inline', 'mai-ad-inline%', false, m.id, 'active' FROM pub_prod p, media_profile m
-- Dev slots (same patterns)
UNION ALL
SELECT p.publisher_id, p.domain || '/billboard', 'billboard', 'mai-ad-billboard%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/leaderboard', 'leaderboard', 'mai-ad-leaderboard%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/skyscraper', 'skyscraper', 'mai-ad-skyscraper%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/rectangle', 'rectangle', 'mai-ad-rectangle%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/mobile-banner', 'mobile-banner', 'mai-ad-mobile-banner%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/adhesion', 'adhesion', 'mai-ad-adhesion%', true, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/sidebar', 'sidebar', 'mai-ad-sidebar%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/footer', 'footer', 'mai-ad-footer%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/header', 'header', 'mai-ad-header%', false, m.id, 'active' FROM pub_dev p, media_profile m
UNION ALL
SELECT p.publisher_id, p.domain || '/inline', 'inline', 'mai-ad-inline%', false, m.id, 'active' FROM pub_dev p, media_profile m
ON CONFLICT (publisher_id, slot_pattern) DO UPDATE
SET slot_name = EXCLUDED.slot_name,
    div_pattern = EXCLUDED.div_pattern,
    is_adhesion = EXCLUDED.is_adhesion,
    media_type_profile_id = EXCLUDED.media_type_profile_id,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- 6. Create Slot Bidder Configs
-- ============================================================================

-- Helper CTE to get all slots and bidders
WITH slots AS (
    SELECT
        s.id as slot_id,
        s.slot_pattern,
        s.slot_name
    FROM ad_slots s
    JOIN publishers_new p ON s.publisher_id = p.id
    JOIN accounts a ON p.account_id = a.id
    WHERE a.account_id = '12345'
),
bidders AS (
    SELECT id as bidder_id, code
    FROM bidders_new
    WHERE code IN ('rubicon', 'kargo', 'sovrn', 'oms', 'aniview', 'pubmatic', 'triplelift')
)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
-- RUBICON configurations
SELECT s.slot_id, b.bidder_id, 'desktop',
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'rubicon'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'rubicon'
-- KARGO configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    '{"placementId": "_o9n8eh8Lsw"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'kargo'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    '{"placementId": "_o9n8eh8Lsw"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'kargo'
-- SOVRN configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    '{"tagid": "1294952"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'sovrn'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    '{"tagid": "1294952"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'sovrn'
-- OMS configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    jsonb_build_object('publisherId', '12345', 'placementId', 'totalprosports-' || s.slot_name || '-desktop'),
    'active'
FROM slots s, bidders b WHERE b.code = 'oms'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    jsonb_build_object('publisherId', '12345', 'placementId', 'totalprosports-' || s.slot_name || '-mobile'),
    'active'
FROM slots s, bidders b WHERE b.code = 'oms'
-- ANIVIEW configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    jsonb_build_object('publisherId', '12345', 'placementId', 'totalprosports-' || s.slot_name || '-desktop'),
    'active'
FROM slots s, bidders b WHERE b.code = 'aniview'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    jsonb_build_object('publisherId', '12345', 'placementId', 'totalprosports-' || s.slot_name || '-mobile'),
    'active'
FROM slots s, bidders b WHERE b.code = 'aniview'
-- PUBMATIC configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'pubmatic'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb,
    'active'
FROM slots s, bidders b WHERE b.code = 'pubmatic'
-- TRIPLELIFT configurations
UNION ALL
SELECT s.slot_id, b.bidder_id, 'desktop',
    jsonb_build_object('inventoryCode', 'totalprosports_' || s.slot_name || '_prebid'),
    'active'
FROM slots s, bidders b WHERE b.code = 'triplelift'
UNION ALL
SELECT s.slot_id, b.bidder_id, 'mobile',
    jsonb_build_object('inventoryCode', 'totalprosports_' || s.slot_name || '_prebid_mobile'),
    'active'
FROM slots s, bidders b WHERE b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
SET bidder_params = EXCLUDED.bidder_params,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- Verification Queries
-- ============================================================================

-- Count configurations by domain and device
SELECT
    p.domain,
    sbc.device_type,
    COUNT(DISTINCT s.id) as slot_count,
    COUNT(DISTINCT b.code) as bidder_count,
    COUNT(*) as total_configs
FROM accounts a
JOIN publishers_new p ON p.account_id = a.id
JOIN ad_slots s ON s.publisher_id = p.id
JOIN slot_bidder_configs sbc ON sbc.ad_slot_id = s.id
JOIN bidders_new b ON sbc.bidder_id = b.id
WHERE a.account_id = '12345' AND sbc.status = 'active'
GROUP BY p.domain, sbc.device_type
ORDER BY p.domain, sbc.device_type;

-- Show sample configs for dev.totalprosports.com/billboard
SELECT
    s.slot_pattern,
    b.code as bidder,
    sbc.device_type,
    sbc.bidder_params
FROM ad_slots s
JOIN slot_bidder_configs sbc ON sbc.ad_slot_id = s.id
JOIN bidders_new b ON sbc.bidder_id = b.id
WHERE s.slot_pattern = 'dev.totalprosports.com/billboard'
  AND sbc.status = 'active'
ORDER BY b.code, sbc.device_type;
