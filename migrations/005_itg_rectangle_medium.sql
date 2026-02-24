-- Add rectangle-medium slot for insidetailgating.com (wildcard params)
-- Rubicon zoneId 3491788 is the catch-all from the ITG spreadsheet wildcard row

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT p.id, 'insidetailgating.com/rectangle-medium', 'Rectangle Medium', false, 'active'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
WHERE p.domain = 'insidetailgating.com'
  AND a.account_id IN ('12345', 'NXS001')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, d.device_type,
  CASE b.code
    WHEN 'rubicon' THEN '{"accountId": 26298, "siteId": 556630, "zoneId": 3491788, "bidonmultiformat": false}'::jsonb
    WHEN 'pubmatic' THEN '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb
  END,
  'active'
FROM ad_slots a
CROSS JOIN bidders_new b
CROSS JOIN (VALUES ('desktop'), ('mobile')) AS d(device_type)
WHERE a.slot_pattern = 'insidetailgating.com/rectangle-medium'
  AND b.code IN ('rubicon', 'pubmatic')
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- Add leaderboard-2 slot for insidetailgating.com (wildcard params)
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT p.id, 'insidetailgating.com/leaderboard-2', 'Leaderboard 2', false, 'active'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
WHERE p.domain = 'insidetailgating.com'
  AND a.account_id IN ('12345', 'NXS001')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, d.device_type,
  CASE b.code
    WHEN 'rubicon' THEN '{"accountId": 26298, "siteId": 556630, "zoneId": 3491788, "bidonmultiformat": false}'::jsonb
    WHEN 'pubmatic' THEN '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb
  END,
  'active'
FROM ad_slots a
CROSS JOIN bidders_new b
CROSS JOIN (VALUES ('desktop'), ('mobile')) AS d(device_type)
WHERE a.slot_pattern = 'insidetailgating.com/leaderboard-2'
  AND b.code IN ('rubicon', 'pubmatic')
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- Add billboard-2 and leaderboard-3 slots (wildcard params)
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT p.id, s.pattern, s.name, false, 'active'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
CROSS JOIN (VALUES
  ('insidetailgating.com/billboard-2', 'Billboard 2'),
  ('insidetailgating.com/leaderboard-3', 'Leaderboard 3')
) AS s(pattern, name)
WHERE p.domain = 'insidetailgating.com'
  AND a.account_id IN ('12345', 'NXS001')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT a.id, b.id, d.device_type,
  CASE b.code
    WHEN 'rubicon' THEN '{"accountId": 26298, "siteId": 556630, "zoneId": 3491788, "bidonmultiformat": false}'::jsonb
    WHEN 'pubmatic' THEN '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb
  END,
  'active'
FROM ad_slots a
CROSS JOIN bidders_new b
CROSS JOIN (VALUES ('desktop'), ('mobile')) AS d(device_type)
WHERE a.slot_pattern IN ('insidetailgating.com/billboard-2', 'insidetailgating.com/leaderboard-3')
  AND b.code IN ('rubicon', 'pubmatic')
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;
