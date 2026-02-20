-- Migration: Add slot mapping table for divId → adUnitPath lookup
-- Purpose: Map HTML div IDs to GAM ad unit paths for server-side resolution
-- This allows the server to determine the ad unit path when SDK only sends divId

CREATE TABLE IF NOT EXISTS slot_mappings (
    id SERIAL PRIMARY KEY,
    publisher_id TEXT NOT NULL REFERENCES publishers(publisher_id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    div_id TEXT NOT NULL,
    ad_unit_path TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(publisher_id, domain, div_id)
);

-- Index for fast lookups by publisher, domain, and div_id
CREATE INDEX idx_slot_mappings_lookup ON slot_mappings(publisher_id, domain, div_id);

-- Index for finding all mappings for a publisher
CREATE INDEX idx_slot_mappings_publisher ON slot_mappings(publisher_id);

-- Add comment
COMMENT ON TABLE slot_mappings IS 'Maps HTML div IDs to GAM ad unit paths for server-side resolution';
COMMENT ON COLUMN slot_mappings.div_id IS 'HTML element ID (e.g., mai-ad-billboard-wide)';
COMMENT ON COLUMN slot_mappings.ad_unit_path IS 'Full GAM ad unit path (e.g., /21775744923/totalprosports/billboard)';

-- Insert example mapping for publisher 12345
INSERT INTO slot_mappings (publisher_id, domain, div_id, ad_unit_path) VALUES
    ('12345', 'dev.totalprosports.com', 'mai-ad-billboard-wide', '/21775744923/totalprosports/billboard'),
    ('12345', 'dev.totalprosports.com', 'mai-ad-leaderboard', '/21775744923/totalprosports/leaderboard'),
    ('12345', 'dev.totalprosports.com', 'mai-ad-rectangle', '/21775744923/totalprosports/rectangle')
ON CONFLICT (publisher_id, domain, div_id) DO NOTHING;
