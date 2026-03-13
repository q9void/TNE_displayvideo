-- Migration 016: Update Kargo placement IDs for Total Pro Sports and Inside Tailgating
-- Source: Bizbudding x Kargo Prebid Server spreadsheet (2026-03-10)
-- Kargo bidder_id = 2

BEGIN;

-- ============================================================
-- TOTAL PRO SPORTS (publisher_id = 12, NXS001 account)
-- ============================================================

-- billboard (slot 88)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_dFGhkhsSFh"}', updated_at = NOW()
  WHERE ad_slot_id = 88 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_riSzQQurYI"}', updated_at = NOW()
  WHERE ad_slot_id = 88 AND bidder_id = 2 AND device_type = 'mobile';

-- billboard-wide (slot 81)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_f7qx87SQbX"}', updated_at = NOW()
  WHERE ad_slot_id = 81 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_riSzQQurYI"}', updated_at = NOW()
  WHERE ad_slot_id = 81 AND bidder_id = 2 AND device_type = 'mobile';

-- leaderboard (slot 89)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_dFGhkhsSFh"}', updated_at = NOW()
  WHERE ad_slot_id = 89 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_rhUrUS9Y7m"}', updated_at = NOW()
  WHERE ad_slot_id = 89 AND bidder_id = 2 AND device_type = 'mobile';

-- leaderboard-wide (slot 82)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_dFGhkhsSFh"}', updated_at = NOW()
  WHERE ad_slot_id = 82 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_rhUrUS9Y7m"}', updated_at = NOW()
  WHERE ad_slot_id = 82 AND bidder_id = 2 AND device_type = 'mobile';

-- rectangle-medium (slot 83)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_wQOXd8ajvK"}', updated_at = NOW()
  WHERE ad_slot_id = 83 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_riSzQQurYI"}', updated_at = NOW()
  WHERE ad_slot_id = 83 AND bidder_id = 2 AND device_type = 'mobile';

-- skyscraper (slot 90) — desktop only per Kargo spec; remove stale mobile fallback
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_iF8i7ATFxI"}', updated_at = NOW()
  WHERE ad_slot_id = 90 AND bidder_id = 2 AND device_type = 'desktop';
DELETE FROM slot_bidder_configs
  WHERE ad_slot_id = 90 AND bidder_id = 2 AND device_type = 'mobile';

-- skyscraper-wide (slot 84) — desktop only
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_egkRE8lSCD"}', updated_at = NOW()
  WHERE ad_slot_id = 84 AND bidder_id = 2 AND device_type = 'desktop';

-- leaderboard-wide-adhesion (slot 87)
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_mKP7sOzRJa"}', updated_at = NOW()
  WHERE ad_slot_id = 87 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_i8NdKkO8iS"}', updated_at = NOW()
  WHERE ad_slot_id = 87 AND bidder_id = 2 AND device_type = 'mobile';

-- ============================================================
-- INSIDE TAILGATING (publisher_id = 13, NXS001 account)
-- ============================================================

-- billboard (slot 77) — update existing
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_rLk5hLWLln"}', updated_at = NOW()
  WHERE ad_slot_id = 77 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_e22tJHQCnZ"}', updated_at = NOW()
  WHERE ad_slot_id = 77 AND bidder_id = 2 AND device_type = 'mobile';

-- billboard-wide (slot 78) — update existing
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_xi1CFxT4Fm"}', updated_at = NOW()
  WHERE ad_slot_id = 78 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_e22tJHQCnZ"}', updated_at = NOW()
  WHERE ad_slot_id = 78 AND bidder_id = 2 AND device_type = 'mobile';

-- leaderboard (slot 79) — update existing
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_rLk5hLWLln"}', updated_at = NOW()
  WHERE ad_slot_id = 79 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_gVlFSkWwfy"}', updated_at = NOW()
  WHERE ad_slot_id = 79 AND bidder_id = 2 AND device_type = 'mobile';

-- leaderboard-wide (slot 80) — update existing
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_rLk5hLWLln"}', updated_at = NOW()
  WHERE ad_slot_id = 80 AND bidder_id = 2 AND device_type = 'desktop';
UPDATE slot_bidder_configs SET bidder_params = '{"placementId": "_gVlFSkWwfy"}', updated_at = NOW()
  WHERE ad_slot_id = 80 AND bidder_id = 2 AND device_type = 'mobile';

-- rectangle-medium (slot 133) — insert new Kargo configs
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  VALUES (133, 2, 'desktop', '{"placementId": "_pKUFDoowtb"}')
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  VALUES (133, 2, 'mobile', '{"placementId": "_e22tJHQCnZ"}')
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

-- Create missing ad slots for Inside Tailgating then add Kargo configs
INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, status)
  VALUES (13, 'insidetailgating.com/skyscraper', 'skyscraper', 'active')
  ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, status)
  VALUES (13, 'insidetailgating.com/skyscraper-wide', 'skyscraper-wide', 'active')
  ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
  VALUES (13, 'insidetailgating.com/leaderboard-wide-adhesion', 'leaderboard-wide-adhesion', true, 'active')
  ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- skyscraper — desktop only
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  SELECT id, 2, 'desktop', '{"placementId": "_v6rl94zvfO"}'
  FROM ad_slots WHERE publisher_id = 13 AND slot_pattern = 'insidetailgating.com/skyscraper'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

-- skyscraper-wide — desktop only
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  SELECT id, 2, 'desktop', '{"placementId": "_l5VkdgpQ1Z"}'
  FROM ad_slots WHERE publisher_id = 13 AND slot_pattern = 'insidetailgating.com/skyscraper-wide'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

-- leaderboard-wide-adhesion — both devices
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  SELECT id, 2, 'desktop', '{"placementId": "_z4RmIXAcFr"}'
  FROM ad_slots WHERE publisher_id = 13 AND slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
  SELECT id, 2, 'mobile', '{"placementId": "_sFY46MOpaF"}'
  FROM ad_slots WHERE publisher_id = 13 AND slot_pattern = 'insidetailgating.com/leaderboard-wide-adhesion'
  ON CONFLICT (ad_slot_id, bidder_id, device_type) DO UPDATE
    SET bidder_params = EXCLUDED.bidder_params, updated_at = NOW();

COMMIT;
