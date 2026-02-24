BEGIN;

-- 1. Add axonix bidder
INSERT INTO bidders_new (code, name, status, pbs_adapter_name)
VALUES ('axonix', 'Axonix', 'active', 'axonix')
ON CONFLICT (code) DO NOTHING;

-- 2. Fix pbs_adapter_name
UPDATE bidders_new SET pbs_adapter_name = 'rubicon' WHERE code = 'rubicon' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'kargo' WHERE code = 'kargo' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'sovrn' WHERE code = 'sovrn' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'oms' WHERE code = 'oms' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'aniview' WHERE code = 'aniview' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'pubmatic' WHERE code = 'pubmatic' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');
UPDATE bidders_new SET pbs_adapter_name = 'triplelift' WHERE code = 'triplelift' AND (pbs_adapter_name IS NULL OR pbs_adapter_name = '');

-- 3. Insert new ad_slots for totalprosports.com (publisher_id=1)
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/billboard-wide', 'Billboard Wide', false, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/leaderboard-wide', 'Leaderboard Wide', false, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/rectangle-medium', 'Rectangle Medium', false, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/skyscraper-wide', 'Skyscraper Wide', false, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/rectangle-medium-adhesion', 'Rectangle Medium Adhesion', true, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/skyscraper-wide-adhesion', 'Skyscraper Wide Adhesion', true, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (1, 'totalprosports.com/leaderboard-wide-adhesion', 'Leaderboard Wide Adhesion', true, 'active')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- 4. Upsert slot_bidder_configs for all totalprosports.com slots from xlsx
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767186, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294952"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294952"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775672, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_skuLuovqg9"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277815"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277815"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/billboard-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_rBnq4aqhaV"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277816"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277816"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775674, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_o9n8eh8Lsw"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_rBnq4aqhaV"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277814"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277814"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767182, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3767182, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_dZdSc3mrpg"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277818"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277818"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3952223, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3952223, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_trvKK0NhUx"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294953"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294953"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3775676, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_xPHrEbaqUL"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1277817"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1277817"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948853, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948853, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_dZdSc3mrpg"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_w8SFrOL82e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294576"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294576"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/rectangle-medium-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948855, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948855, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_xPHrEbaqUL"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294577"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294577"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/skyscraper-wide-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948851, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3948851, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_alxQWkaGyi"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_zT1mb43RiX"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294575"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294575"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079290"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "6806a79f20173d1cde0a4895"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "21146"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = 1 AND a.slot_pattern = 'totalprosports.com/leaderboard-wide-adhesion' AND b.code = 'oms'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

COMMIT;


-- ============================================================
-- NXS001 SECONDARY ACCOUNT (backup duplicate of 12345 mapping)
-- ============================================================

-- Create NXS001 account
INSERT INTO accounts (account_id, name, status)
VALUES ('NXS001', 'BizBudding Network (NXS001)', 'active')
ON CONFLICT (account_id) DO NOTHING;

-- Duplicate publishers under NXS001 for real domains only
INSERT INTO publishers_new (account_id, domain, name, status, notes)
SELECT
    (SELECT id FROM accounts WHERE account_id = 'NXS001'),
    p.domain,
    p.name,
    'active',
    'NXS001 backup mirror of 12345 account'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
WHERE a.account_id = '12345'
  AND p.domain NOT LIKE 'dev.%'
ON CONFLICT (account_id, domain) DO NOTHING;

-- Duplicate ad_slots for NXS001 publishers
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT
    pn.id,
    a.slot_pattern,
    a.slot_name,
    a.is_adhesion,
    a.status
FROM ad_slots a
JOIN publishers_new p ON a.publisher_id = p.id
JOIN accounts acc ON p.account_id = acc.id
JOIN publishers_new pn ON pn.domain = p.domain
JOIN accounts acc2 ON pn.account_id = acc2.id
WHERE acc.account_id = '12345'
  AND p.domain NOT LIKE 'dev.%'
  AND acc2.account_id = 'NXS001'
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Duplicate slot_bidder_configs for NXS001 ad_slots
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, bid_floor, bid_floor_cur, status)
SELECT
    nxs_slot.id,
    sbc.bidder_id,
    sbc.device_type,
    sbc.bidder_params,
    sbc.bid_floor,
    sbc.bid_floor_cur,
    sbc.status
FROM slot_bidder_configs sbc
JOIN ad_slots orig_slot ON sbc.ad_slot_id = orig_slot.id
JOIN publishers_new orig_pub ON orig_slot.publisher_id = orig_pub.id
JOIN accounts orig_acc ON orig_pub.account_id = orig_acc.id
JOIN publishers_new nxs_pub ON nxs_pub.domain = orig_pub.domain
JOIN accounts nxs_acc ON nxs_pub.account_id = nxs_acc.id
JOIN ad_slots nxs_slot ON nxs_slot.publisher_id = nxs_pub.id AND nxs_slot.slot_pattern = orig_slot.slot_pattern
WHERE orig_acc.account_id = '12345'
  AND orig_pub.domain NOT LIKE 'dev.%'
  AND nxs_acc.account_id = 'NXS001'
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

COMMIT;
