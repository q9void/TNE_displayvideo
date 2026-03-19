-- Migration 017: Add Rubicon and PubMatic configs for insidetailgating.com/leaderboard-wide-adhesion
-- Context: slot was created in migration 016 with Kargo-only configs. Rubicon and PubMatic were
-- missing, causing MakeRequests errors (bidders_error:2) whenever this slot appeared in a
-- multi-impression auction alongside slots that did have Rubicon/PubMatic configs.
--
-- Rubicon zone: reusing leaderboard-wide zone (3491672) as placeholder until Magnite provides
-- a dedicated zone for this adhesion unit.
-- PubMatic adSlot: 7079276 (consistent with all ITG leaderboard slots)

BEGIN;

-- -------------------------------------------------------
-- Rubicon (code: 'rubicon')
-- -------------------------------------------------------
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
  SELECT s.id, b.id, 'desktop',
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3491672, "bidonmultiformat": false}'::jsonb,
    'active'
  FROM ad_slots s, bidders_new b
  WHERE s.publisher_id = 13
    AND s.slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
    AND b.code = 'rubicon'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params,
        status = EXCLUDED.status,
        updated_at = NOW();

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
  SELECT s.id, b.id, 'mobile',
    '{"accountId": 26298, "siteId": 556630, "zoneId": 3491672, "bidonmultiformat": false}'::jsonb,
    'active'
  FROM ad_slots s, bidders_new b
  WHERE s.publisher_id = 13
    AND s.slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
    AND b.code = 'rubicon'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params,
        status = EXCLUDED.status,
        updated_at = NOW();

-- -------------------------------------------------------
-- PubMatic (code: 'pubmatic')
-- -------------------------------------------------------
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
  SELECT s.id, b.id, 'desktop',
    '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb,
    'active'
  FROM ad_slots s, bidders_new b
  WHERE s.publisher_id = 13
    AND s.slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
    AND b.code = 'pubmatic'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params,
        status = EXCLUDED.status,
        updated_at = NOW();

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
  SELECT s.id, b.id, 'mobile',
    '{"publisherId": "166938", "adSlot": "7079276"}'::jsonb,
    'active'
  FROM ad_slots s, bidders_new b
  WHERE s.publisher_id = 13
    AND s.slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
    AND b.code = 'pubmatic'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params,
        status = EXCLUDED.status,
        updated_at = NOW();

COMMIT;
