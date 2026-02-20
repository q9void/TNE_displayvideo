-- Migration: Add domain-level bidder configuration support
-- Purpose: Enable per-domain bidder configuration (e.g., oms, aniview)
-- This allows different bidder parameters for different domains under the same publisher

CREATE TABLE IF NOT EXISTS domain_bidder_configs (
    id SERIAL PRIMARY KEY,
    publisher_id TEXT NOT NULL REFERENCES publishers(publisher_id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    bidder_code TEXT NOT NULL,
    params JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(publisher_id, domain, bidder_code)
);

-- Index for fast lookups by publisher and domain
CREATE INDEX idx_domain_bidder_configs_lookup ON domain_bidder_configs(publisher_id, domain, bidder_code);

-- Index for finding all configs for a publisher
CREATE INDEX idx_domain_bidder_configs_publisher ON domain_bidder_configs(publisher_id);

-- Index for finding all configs for a specific bidder
CREATE INDEX idx_domain_bidder_configs_bidder ON domain_bidder_configs(bidder_code);

-- Add comment
COMMENT ON TABLE domain_bidder_configs IS 'Domain-level bidder configuration that overrides publisher-level defaults';
COMMENT ON COLUMN domain_bidder_configs.params IS 'Bidder-specific configuration parameters in JSON format';
