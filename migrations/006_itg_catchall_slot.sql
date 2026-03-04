-- Add wildcard catch-all slot for insidetailgating.com
-- From Demand-Manager-Web-domain_insidetailgating_desktop-Patterns.xlsx
-- The "*" row in that file maps to Rubicon zoneId 3491788 only (no kargo, no pubmatic).
-- This slot fires for any div ID that doesn't match a named slot pattern.

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT p.id, 'insidetailgating.com/*', 'Catch-all', false, 'active'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
WHERE p.domain = 'insidetailgating.com'
  AND a.account_id IN ('12345', 'NXS001')
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
SELECT s.id, b.id, d.device_type,
  '{"accountId": 26298, "siteId": 556630, "zoneId": 3491788, "bidonmultiformat": false}'::jsonb,
  'active'
FROM ad_slots s
JOIN publishers_new p ON s.publisher_id = p.id
JOIN accounts a ON p.account_id = a.id
CROSS JOIN bidders_new b
CROSS JOIN (VALUES ('desktop'), ('mobile')) AS d(device_type)
WHERE s.slot_pattern = 'insidetailgating.com/*'
  AND a.account_id IN ('12345', 'NXS001')
  AND b.code = 'rubicon'
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;
