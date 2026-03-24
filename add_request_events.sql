-- Migration: add request_events table (one row per bid request, SDK-level view)
-- Apply to production: psql -h catalyst-postgres -U catalyst_prod -d catalyst_production

-- Add page_url to auction_events for consistency
ALTER TABLE auction_events ADD COLUMN IF NOT EXISTS page_url TEXT;

-- SDK / request-level events (one row per bid request)
CREATE TABLE IF NOT EXISTS request_events (
    id BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    publisher_id VARCHAR(255),

    -- Page context
    page_url TEXT,
    page_domain VARCHAR(255),
    first_ad_unit TEXT,
    slot_count INTEGER DEFAULT 1,

    -- Device
    device_type VARCHAR(50),
    device_country VARCHAR(2),

    -- Identity
    fpid TEXT,
    eid_count INTEGER DEFAULT 0,
    consent_ok BOOLEAN DEFAULT TRUE,

    -- Timing
    tmax_ms INTEGER,
    auction_ms INTEGER,

    -- Outcome
    total_bids INTEGER DEFAULT 0,
    bids_returned INTEGER DEFAULT 0,
    timed_out_bidders TEXT,  -- comma-joined list, NULL if none
    outcome VARCHAR(50),     -- bids_returned / no_bids / timeout / error

    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_events_auction_id ON request_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_request_events_publisher_id ON request_events(publisher_id);
CREATE INDEX IF NOT EXISTS idx_request_events_created_at ON request_events(created_at);
CREATE INDEX IF NOT EXISTS idx_request_events_outcome ON request_events(outcome);

SELECT 'Migration complete' AS status;
