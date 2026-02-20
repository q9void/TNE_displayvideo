-- Migration 009: Create New Account-Based Schema
-- This migration creates a normalized schema with accounts, publishers, ad slots, and bidder configurations
-- Replaces simple publishers.bidder_params JSONB with proper relational structure

-- ============================================================================
-- Accounts Table (Top-level entity)
-- ============================================================================
CREATE TABLE IF NOT EXISTS accounts (
    id              SERIAL PRIMARY KEY,
    account_id      VARCHAR(255) NOT NULL UNIQUE,  -- External account ID from SDK (e.g., '12345')
    name            TEXT,                          -- Account display name
    status          VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_accounts_account_id ON accounts(account_id);

COMMENT ON TABLE accounts IS 'Top-level account entity - represents a CATALYST customer account';
COMMENT ON COLUMN accounts.account_id IS 'External account identifier from SDK (CATALYST internal ID)';

-- ============================================================================
-- Publishers Table (Belongs to account)
-- ============================================================================
CREATE TABLE IF NOT EXISTS publishers_new (
    id              SERIAL PRIMARY KEY,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    domain          TEXT NOT NULL,                 -- Publisher domain (e.g., 'totalprosports.com')
    name            TEXT,                          -- Publisher display name
    status          VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    pbs_account_id  TEXT,                          -- Optional Prebid Server account override
    default_timeout_ms INTEGER DEFAULT 1000,       -- Default bidder timeout
    default_currency TEXT DEFAULT 'USD',           -- Default currency code
    notes           TEXT,                          -- Admin notes
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, domain)
);

CREATE INDEX idx_publishers_new_account ON publishers_new(account_id);
CREATE INDEX idx_publishers_new_domain ON publishers_new(domain);

COMMENT ON TABLE publishers_new IS 'Publisher websites belonging to accounts';
COMMENT ON COLUMN publishers_new.domain IS 'Publisher domain - used for matching incoming requests';
COMMENT ON COLUMN publishers_new.pbs_account_id IS 'Optional Prebid Server account ID override';

-- ============================================================================
-- Media Type Profiles (Reusable size/format definitions)
-- ============================================================================
CREATE TABLE IF NOT EXISTS media_type_profiles (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,          -- Profile name (e.g., 'billboard_desktop')
    media_types     JSONB NOT NULL,                -- {"banner":{"sizes":[[728,90]...]}, "native":{...}}
    description     TEXT,                          -- Profile description
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE media_type_profiles IS 'Reusable media type and size definitions for ad slots';
COMMENT ON COLUMN media_type_profiles.media_types IS 'OpenRTB media types configuration (banner, video, native)';

-- ============================================================================
-- Bidders Table (Adapter registry)
-- ============================================================================
CREATE TABLE IF NOT EXISTS bidders_new (
    id              SERIAL PRIMARY KEY,
    code            TEXT NOT NULL UNIQUE,          -- Bidder code (e.g., 'rubicon', 'kargo')
    name            TEXT,                          -- Bidder display name
    status          VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'disabled')),
    param_schema    JSONB,                         -- JSON Schema of expected params
    pbs_adapter_name TEXT,                         -- Prebid Server adapter name (if different from code)
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bidders_new_code ON bidders_new(code);

COMMENT ON TABLE bidders_new IS 'Registry of available SSP bidder adapters';
COMMENT ON COLUMN bidders_new.code IS 'Bidder code used in OpenRTB requests (e.g., rubicon, kargo)';
COMMENT ON COLUMN bidders_new.param_schema IS 'JSON Schema defining expected bidder parameters';

-- ============================================================================
-- Ad Slots (Logical ad placements)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ad_slots (
    id              SERIAL PRIMARY KEY,
    publisher_id    INTEGER NOT NULL REFERENCES publishers_new(id) ON DELETE CASCADE,
    slot_pattern    TEXT NOT NULL,                 -- Slot identifier pattern (e.g., 'totalprosports.com/billboard')
    slot_name       TEXT NOT NULL,                 -- Human-readable slot name (e.g., 'billboard')
    div_pattern     TEXT,                          -- DIV ID pattern for matching
    is_adhesion     BOOLEAN DEFAULT FALSE,         -- Is this an adhesion/sticky ad?
    media_type_profile_id INTEGER REFERENCES media_type_profiles(id),
    custom_params   JSONB,                         -- Custom slot parameters
    json_config     JSONB,                         -- Additional JSON configuration
    status          VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(publisher_id, slot_pattern)
);

CREATE INDEX idx_ad_slots_publisher ON ad_slots(publisher_id);
CREATE INDEX idx_ad_slots_pattern ON ad_slots(slot_pattern);
CREATE INDEX idx_ad_slots_name ON ad_slots(slot_name);

COMMENT ON TABLE ad_slots IS 'Logical ad placement units within publishers';
COMMENT ON COLUMN ad_slots.slot_pattern IS 'Unique slot identifier (domain/slot_name)';
COMMENT ON COLUMN ad_slots.is_adhesion IS 'Whether this is a sticky/adhesion ad unit';

-- ============================================================================
-- Slot Bidder Configs (Core junction table)
-- ============================================================================
CREATE TABLE IF NOT EXISTS slot_bidder_configs (
    id              SERIAL PRIMARY KEY,
    ad_slot_id      INTEGER NOT NULL REFERENCES ad_slots(id) ON DELETE CASCADE,
    bidder_id       INTEGER NOT NULL REFERENCES bidders_new(id) ON DELETE CASCADE,
    device_type     VARCHAR(50) NOT NULL CHECK (device_type IN ('desktop', 'mobile', 'all')),
    bidder_params   JSONB NOT NULL,                -- SSP-specific parameters (goes to imp.ext.prebid.bidder.{code})
    status          VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    bid_floor       NUMERIC,                       -- Optional bid floor
    bid_floor_cur   VARCHAR(10) DEFAULT 'USD',     -- Bid floor currency
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(ad_slot_id, bidder_id, device_type)
);

CREATE INDEX idx_sbc_slot ON slot_bidder_configs(ad_slot_id);
CREATE INDEX idx_sbc_bidder ON slot_bidder_configs(bidder_id);
CREATE INDEX idx_sbc_device ON slot_bidder_configs(device_type);
CREATE INDEX idx_sbc_slot_device ON slot_bidder_configs(ad_slot_id, device_type);
CREATE INDEX idx_sbc_status ON slot_bidder_configs(status);

COMMENT ON TABLE slot_bidder_configs IS 'Bidder configurations per ad slot and device type';
COMMENT ON COLUMN slot_bidder_configs.bidder_params IS 'SSP-specific parameters sent in OpenRTB request';
COMMENT ON COLUMN slot_bidder_configs.device_type IS 'Device targeting: desktop, mobile, or all';

-- ============================================================================
-- Migration Complete
-- ============================================================================

-- To seed initial data, run: deployment/seed_data_v2.sql
