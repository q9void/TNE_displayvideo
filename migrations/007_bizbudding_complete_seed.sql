-- ==============================================================
-- 007_bizbudding_complete_seed.sql
-- Full SSP placement configs for all Bizbudding network domains
-- Generated from Demand Manager xlsx exports 2026-03-17
-- Domains: totalprosports.com, lamag.com, insidetailgating.com
-- SSPs: rubicon, kargo, sovrn, pubmatic, triplelift, axonix
-- Phase 2: oms, axonix (adapters to be built)
-- Removed: aniview (not used)
-- ==============================================================

BEGIN;

-- 1. Ensure all required bidders exist
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('rubicon', 'Rubicon (Magnite)', 'active', 'rubicon') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('kargo', 'Kargo', 'active', 'kargo') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('sovrn', 'Sovrn', 'active', 'sovrn') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('oms', 'OMS', 'active', 'oms') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('pubmatic', 'PubMatic', 'active', 'pubmatic') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('triplelift', 'TripleLift', 'active', 'triplelift') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();
INSERT INTO bidders_new (code, name, status, pbs_adapter_name) VALUES ('axonix', 'Axonix', 'active', 'axonix') ON CONFLICT (code) DO UPDATE SET status='active', updated_at=NOW();

-- ================================================================
-- totalprosports.com
-- ================================================================

INSERT INTO publishers_new (account_id, domain, name, status) VALUES (
    (SELECT id FROM accounts WHERE account_id = '12345'),
    'totalprosports.com', 'TotalProSports', 'active'
) ON CONFLICT (account_id, domain) DO NOTHING;

-- Ad slots for totalprosports.com
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/billboard', 'Billboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/billboard-wide', 'Billboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/leaderboard', 'Leaderboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/leaderboard-wide', 'Leaderboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/rectangle-medium', 'Rectangle Medium', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/skyscraper', 'Skyscraper', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/skyscraper-wide', 'Skyscraper Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/rectangle-medium-adhesion', 'Rectangle Medium Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/skyscraper-wide-adhesion', 'Skyscraper Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'totalprosports.com/leaderboard-wide-adhesion', 'Leaderboard Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Bidder configs: totalprosports.com (desktop)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294952"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_skuLuovqg9"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277815"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277816"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277814"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767182, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_dZdSc3mrpg"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277818"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3952223, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_trvKK0NhUx"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294953"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_xPHrEbaqUL"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277817"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948853, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_dZdSc3mrpg"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294576"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948855, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_xPHrEbaqUL"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294577"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948851, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_alxQWkaGyi"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294575"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

-- Bidder configs: totalprosports.com (mobile)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294952"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277815"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/billboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_rBnq4aqhaV"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277816"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_rBnq4aqhaV"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277814"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767182, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277818"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3952223, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294953"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277817"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948853, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294576"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/rectangle-medium-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948855, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294577"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/skyscraper-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948851, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_zT1mb43RiX"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294575"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='oms'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='totalprosports.com/leaderboard-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='totalprosports.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

-- ================================================================
-- lamag.com
-- ================================================================

INSERT INTO publishers_new (account_id, domain, name, status) VALUES (
    (SELECT id FROM accounts WHERE account_id = '12345'),
    'lamag.com', 'LA Mag', 'active'
) ON CONFLICT (account_id, domain) DO NOTHING;

-- Ad slots for lamag.com
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/billboard', 'Billboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/billboard-wide', 'Billboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/leaderboard', 'Leaderboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/leaderboard-wide', 'Leaderboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/rectangle-medium', 'Rectangle Medium', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/skyscraper', 'Skyscraper', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/skyscraper-wide', 'Skyscraper Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/leaderboard-wide-adhesion', 'Leaderboard Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/rectangle-medium-adhesion', 'Rectangle Medium Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'lamag.com/skyscraper-wide-adhesion', 'Skyscraper Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Bidder configs: lamag.com (desktop)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885116, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288535"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885118, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288536"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_pDwjUExxJF"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885112, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288533"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885114, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288534"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885124, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288532"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_hRjeN3QUzD"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885120, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288537"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_oTWEOFH5Md"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885122, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288538"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_ulDVHh4kmG"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950621, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294709"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_spQBbtOwS9"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950619, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294710"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_hRjeN3QUzD"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950617, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294711"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_ulDVHh4kmG"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

-- Bidder configs: lamag.com (mobile)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885116, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288535"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885118, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288536"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/billboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885112, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288533"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_gaNAXtp16e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885114, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288534"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_gaNAXtp16e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885124, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288532"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885120, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288537"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885122, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288538"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950621, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294709"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_wgh3FQJLYQ"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/leaderboard-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950619, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294710"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/rectangle-medium-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950617, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294711"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='sovrn'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='triplelift'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='lamag.com/skyscraper-wide-adhesion' AND b.code='axonix'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='lamag.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

-- ================================================================
-- insidetailgating.com
-- ================================================================

INSERT INTO publishers_new (account_id, domain, name, status) VALUES (
    (SELECT id FROM accounts WHERE account_id = '12345'),
    'insidetailgating.com', 'Inside Tailgating', 'active'
) ON CONFLICT (account_id, domain) DO NOTHING;

-- Ad slots for insidetailgating.com
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'insidetailgating.com/billboard', 'Billboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'insidetailgating.com/billboard-wide', 'Billboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'insidetailgating.com/leaderboard', 'Leaderboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status) VALUES (
    (SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345')),
    'insidetailgating.com/leaderboard-wide', 'Leaderboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Bidder configs: insidetailgating.com (desktop)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491678, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_b7qknQwZG0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_b7qknQwZG0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_b7qknQwZG0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o95O1osGcc"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

-- Bidder configs: insidetailgating.com (mobile)
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491678, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_hVQrbgYrof"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_hVQrbgYrof"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/billboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_hVQrbgYrof"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3491672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='rubicon'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='pubmatic'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_sYvY8bedug"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.slot_pattern='insidetailgating.com/leaderboard-wide' AND b.code='kargo'
  AND a.publisher_id=(SELECT id FROM publishers_new WHERE domain='insidetailgating.com' AND account_id=(SELECT id FROM accounts WHERE account_id='12345'))
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE SET bidder_params=EXCLUDED.bidder_params, updated_at=NOW();

COMMIT;