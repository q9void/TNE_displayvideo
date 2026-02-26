-- Migration 013: Revenue share — add bid_multiplier to publishers_new
--
-- bid_multiplier controls TNE's platform cut.
-- Bid returned to publisher = SSP bid / bid_multiplier
--   1.0000 =  0% cut (pass-through)
--   1.2500 = 20% cut (publisher receives 80%)
--
-- Default is 1.25 (20% to TNE) for all new and existing publishers.
-- Bizbudding is currently 0% while commercial terms are being finalised.

ALTER TABLE publishers_new
    ADD COLUMN IF NOT EXISTS bid_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.2500;

-- Bizbudding Network: 0% rev-share while terms are being worked out
UPDATE publishers_new p
SET    bid_multiplier = 1.0000
FROM   accounts a
WHERE  p.account_id = a.id
  AND  a.account_id = '12345';
