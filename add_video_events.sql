-- Migration: add video_events table (one row per VAST tracking pixel: impression / quartile / complete / error / etc.)
-- Apply to production: psql -h catalyst-postgres -U catalyst_prod -d catalyst_production

CREATE TABLE IF NOT EXISTS video_events (
    id            BIGSERIAL PRIMARY KEY,
    bid_id        VARCHAR(255) NOT NULL,
    account_id    VARCHAR(255),
    bidder        VARCHAR(100),
    event         VARCHAR(50) NOT NULL,  -- 'impression','start','firstQuartile','midpoint','thirdQuartile','complete','skip','pause','resume','click','error'
    progress      DECIMAL(6,4),
    error_code    VARCHAR(50),
    error_message TEXT,
    click_url     TEXT,
    session_id    VARCHAR(255),
    content_id    VARCHAR(255),
    ip_address    VARCHAR(64),  -- already anonymised by middleware before reaching here
    user_agent    TEXT,
    timestamp     TIMESTAMP NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_video_events_bid_id     ON video_events(bid_id);
CREATE INDEX IF NOT EXISTS idx_video_events_account_id ON video_events(account_id);
CREATE INDEX IF NOT EXISTS idx_video_events_event      ON video_events(event);
CREATE INDEX IF NOT EXISTS idx_video_events_created_at ON video_events(created_at);

SELECT 'Migration complete' AS status;
