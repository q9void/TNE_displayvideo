BEGIN;

-- ============================================================
-- lamag.com publisher + slots + bidder configs
-- ============================================================

INSERT INTO publishers_new (account_id, domain, name, status)
VALUES ((SELECT id FROM accounts WHERE account_id = '12345'), 'lamag.com', 'LA Mag', 'active')
ON CONFLICT (account_id, domain) DO NOTHING;

-- Ad slots
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/billboard', 'Billboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/billboard-wide', 'Billboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/leaderboard', 'Leaderboard', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/leaderboard-wide', 'Leaderboard Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/rectangle-medium', 'Rectangle Medium', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/skyscraper', 'Skyscraper', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/skyscraper-wide', 'Skyscraper Wide', false, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/leaderboard-wide-adhesion', 'Leaderboard Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/rectangle-medium-adhesion', 'Rectangle Medium Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
VALUES (
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345')),
    'lamag.com/skyscraper-wide-adhesion', 'Skyscraper Wide Adhesion', true, 'active'
) ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Slot bidder configs
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885116, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885116, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288535"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288535"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885118, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885118, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288536"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288536"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_pDwjUExxJF"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/billboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885112, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885112, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288533"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288533"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_gaNAXtp16e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885114, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885114, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288534"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288534"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_w52KSHRNjm"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_gaNAXtp16e"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885124, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885124, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288532"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288532"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_hRjeN3QUzD"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885120, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885120, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288537"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288537"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_HDX_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_oTWEOFH5Md"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885122, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3885122, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1288538"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1288538"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_ulDVHh4kmG"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950621, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950621, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294709"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294709"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_Native_Adhesion_Prebidc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_spQBbtOwS9"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_wgh3FQJLYQ"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/leaderboard-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950619, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950619, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294710"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294710"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_hRjeN3QUzD"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"placementId": "_pqqleFeYkE"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/rectangle-medium-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950617, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"accountId": 26298, "siteId": 556630, "zoneId": 3950617, "bidonmultiformat": false}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"tagid": "1294711"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"tagid": "1294711"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'sovrn'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"AV_PUBLISHERID": "66aa757144c99c7ca504e937", "AV_CHANNELID": "68cfb39a02fdf0f37e053f28"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'aniview'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"publisherId": "166938", "adSlot": "7079278"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'pubmatic'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'triplelift'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'mobile', '{"supplyId": "4311ace9-d8cd-437e-bf21-0bae2d463eb0"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'axonix'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, 'desktop', '{"placementId": "_ulDVHh4kmG"}'::jsonb, 'active'
FROM ad_slots a, bidders_new b
WHERE a.publisher_id = (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = '12345'))
  AND a.slot_pattern = 'lamag.com/skyscraper-wide-adhesion' AND b.code = 'kargo'
ON CONFLICT (ad_slot_id, bidder_id, device_type)
DO UPDATE SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

-- Duplicate lamag.com under NXS001 account
INSERT INTO publishers_new (account_id, domain, name, status, notes)
VALUES ((SELECT id FROM accounts WHERE account_id = 'NXS001'), 'lamag.com', 'LA Mag', 'active', 'NXS001 backup mirror of 12345 account')
ON CONFLICT (account_id, domain) DO NOTHING;

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT
    (SELECT id FROM publishers_new WHERE domain = 'lamag.com' AND account_id = (SELECT id FROM accounts WHERE account_id = 'NXS001')),
    a.slot_pattern, a.slot_name, a.is_adhesion, a.status
FROM ad_slots a
JOIN publishers_new p ON a.publisher_id = p.id
WHERE p.domain = 'lamag.com' AND p.account_id = (SELECT id FROM accounts WHERE account_id = '12345')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, bid_floor, bid_floor_cur, status)
SELECT nxs_slot.id, sbc.bidder_id, sbc.device_type, sbc.bidder_params, sbc.bid_floor, sbc.bid_floor_cur, sbc.status
FROM slot_bidder_configs sbc
JOIN ad_slots orig_slot ON sbc.ad_slot_id = orig_slot.id
JOIN publishers_new orig_pub ON orig_slot.publisher_id = orig_pub.id
JOIN publishers_new nxs_pub ON nxs_pub.domain = orig_pub.domain
JOIN ad_slots nxs_slot ON nxs_slot.publisher_id = nxs_pub.id AND nxs_slot.slot_pattern = orig_slot.slot_pattern
WHERE orig_pub.domain = 'lamag.com'
  AND orig_pub.account_id = (SELECT id FROM accounts WHERE account_id = '12345')
  AND nxs_pub.account_id = (SELECT id FROM accounts WHERE account_id = 'NXS001')
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

COMMIT;
