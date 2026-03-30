-- Migration: add render_events table (one row per SDK render/viewability beacon)
-- Apply to production: psql -h catalyst-postgres -U catalyst_prod -d catalyst_production

CREATE TABLE IF NOT EXISTS render_events (
    id         BIGSERIAL PRIMARY KEY,
    auction_id VARCHAR(255) NOT NULL,
    div_id     VARCHAR(255),
    bidder     VARCHAR(100),
    cpm        DECIMAL(10,5),
    event      VARCHAR(50),  -- 'rendered' or 'viewable'
    timestamp  TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_render_events_auction_id ON render_events(auction_id);
CREATE INDEX IF NOT EXISTS idx_render_events_created_at ON render_events(created_at);
CREATE INDEX IF NOT EXISTS idx_render_events_event      ON render_events(event);

SELECT 'Migration complete' AS status;
