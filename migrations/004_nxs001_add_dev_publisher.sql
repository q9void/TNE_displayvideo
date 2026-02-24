-- Add dev.totalprosports.com to NXS001 account (mirror of 12345)

INSERT INTO publishers_new (account_id, domain, name, status, notes)
SELECT
    (SELECT id FROM accounts WHERE account_id = 'NXS001'),
    p.domain, p.name, p.status, 'NXS001 backup mirror of 12345 account'
FROM publishers_new p
JOIN accounts a ON p.account_id = a.id
WHERE a.account_id = '12345' AND p.domain = 'dev.totalprosports.com'
ON CONFLICT (account_id, domain) DO NOTHING;

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
SELECT
    (SELECT id FROM publishers_new WHERE domain = 'dev.totalprosports.com' AND account_id = (SELECT id FROM accounts WHERE account_id = 'NXS001')),
    a.slot_pattern, a.slot_name, a.is_adhesion, a.status
FROM ad_slots a
JOIN publishers_new p ON a.publisher_id = p.id
JOIN accounts acc ON p.account_id = acc.id
WHERE p.domain = 'dev.totalprosports.com' AND acc.account_id = '12345'
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, bid_floor, bid_floor_cur, status)
SELECT nxs_slot.id, sbc.bidder_id, sbc.device_type, sbc.bidder_params, sbc.bid_floor, sbc.bid_floor_cur, sbc.status
FROM slot_bidder_configs sbc
JOIN ad_slots orig_slot ON sbc.ad_slot_id = orig_slot.id
JOIN publishers_new orig_pub ON orig_slot.publisher_id = orig_pub.id
JOIN accounts orig_acc ON orig_pub.account_id = orig_acc.id
JOIN publishers_new nxs_pub ON nxs_pub.domain = orig_pub.domain
JOIN accounts nxs_acc ON nxs_pub.account_id = nxs_acc.id
JOIN ad_slots nxs_slot ON nxs_slot.publisher_id = nxs_pub.id AND nxs_slot.slot_pattern = orig_slot.slot_pattern
WHERE orig_pub.domain = 'dev.totalprosports.com'
  AND orig_acc.account_id = '12345'
  AND nxs_acc.account_id = 'NXS001'
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;
