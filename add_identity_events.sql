-- Migration: add identity_events table and new columns to existing tables
-- Apply to production: psql -h catalyst-postgres -U catalyst_prod -d catalyst_production

-- Add ad_unit to auction_events (if not already present)
ALTER TABLE auction_events ADD COLUMN IF NOT EXISTS ad_unit TEXT;

-- Add ad_unit and sizes to bidder_events (if not already present)
ALTER TABLE bidder_events ADD COLUMN IF NOT EXISTS ad_unit TEXT;
ALTER TABLE bidder_events ADD COLUMN IF NOT EXISTS sizes TEXT;

-- Identity / EID events table (one row per auction, one column per ID source)
CREATE TABLE IF NOT EXISTS identity_events (
    id BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    total_eids INTEGER DEFAULT 0,

    fpid         TEXT,  -- thenexusengine.com first-party ID
    id5_uid      TEXT,  -- id5-sync.com
    rubicon_uid  TEXT,  -- rubiconproject.com
    kargo_uid    TEXT,  -- kargo.com
    pubmatic_uid TEXT,  -- pubmatic.com
    sovrn_uid    TEXT,  -- lijit.com (Sovrn)
    appnexus_uid TEXT,  -- adnxs.com (AppNexus/Xandr)
    buyer_uid    TEXT,  -- OpenRTB user.buyeruid

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_identity_events_auction_id ON identity_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_identity_events_created_at ON identity_events(created_at);

SELECT 'Migration complete' AS status;
