-- Migration: Add unit-level bidder configuration support
-- Purpose: Enable per-ad-unit bidder configuration (e.g., sovrn, kargo)
-- This allows different bidder parameters for each ad unit on a domain

CREATE TABLE IF NOT EXISTS unit_bidder_configs (
    id SERIAL PRIMARY KEY,
    publisher_id TEXT NOT NULL REFERENCES publishers(publisher_id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    ad_unit_path TEXT NOT NULL,
    bidder_code TEXT NOT NULL,
    params JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(publisher_id, domain, ad_unit_path, bidder_code)
);

-- Index for fast lookups by publisher, domain, and ad unit
CREATE INDEX idx_unit_bidder_configs_lookup ON unit_bidder_configs(publisher_id, domain, ad_unit_path, bidder_code);

-- Index for finding all configs for a publisher
CREATE INDEX idx_unit_bidder_configs_publisher ON unit_bidder_configs(publisher_id);

-- Index for finding all configs for a domain
CREATE INDEX idx_unit_bidder_configs_domain ON unit_bidder_configs(publisher_id, domain);

-- Index for finding all configs for a specific bidder
CREATE INDEX idx_unit_bidder_configs_bidder ON unit_bidder_configs(bidder_code);

-- Add comment
COMMENT ON TABLE unit_bidder_configs IS 'Ad-unit-level bidder configuration that overrides domain and publisher-level defaults';
COMMENT ON COLUMN unit_bidder_configs.ad_unit_path IS 'Full ad unit path (e.g., "dev.totalprosports.com/billboard")';
COMMENT ON COLUMN unit_bidder_configs.params IS 'Bidder-specific configuration parameters in JSON format';
