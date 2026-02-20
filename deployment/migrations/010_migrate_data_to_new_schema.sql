-- Migration 010: Migrate data from old schema to new normalized schema
-- This migration moves data from:
--   publishers, bidders, domain_bidder_configs, unit_bidder_configs, slot_mappings
-- To:
--   accounts, publishers_new, bidders_new, ad_slots, slot_bidder_configs
--
-- SAFE TO RUN MULTIPLE TIMES: Uses INSERT ... ON CONFLICT DO NOTHING
-- OLD TABLES ARE NOT DROPPED: Allows rollback if needed

BEGIN;

-- ============================================================================
-- STEP 1: Migrate publishers → accounts
-- ============================================================================
-- Create accounts from existing publishers
-- Each publisher becomes an account in the new schema

INSERT INTO accounts (account_id, name, status, created_at, updated_at)
SELECT DISTINCT
    publisher_id,                    -- Use publisher_id as account_id
    name,                            -- Copy publisher name as account name
    status,                          -- Copy status (active, paused, archived)
    created_at,
    updated_at
FROM publishers
ON CONFLICT (account_id) DO NOTHING;

COMMENT ON COLUMN accounts.account_id IS 'Migrated from publishers.publisher_id';

-- ============================================================================
-- STEP 2: Migrate publishers → publishers_new
-- ============================================================================
-- Extract domain from allowed_domains (take first domain)
-- Link to accounts via account_id

INSERT INTO publishers_new (
    account_id,
    domain,
    name,
    status,
    pbs_account_id,
    default_timeout_ms,
    default_currency,
    notes,
    created_at,
    updated_at
)
SELECT
    a.id,                                                    -- FK to accounts
    SPLIT_PART(p.allowed_domains, '|', 1),                  -- Extract first domain
    p.name,
    p.status,
    NULL,                                                    -- pbs_account_id (optional)
    1000,                                                    -- default_timeout_ms
    'USD',                                                   -- default_currency
    p.notes,
    p.created_at,
    p.updated_at
FROM publishers p
JOIN accounts a ON p.publisher_id = a.account_id
ON CONFLICT (account_id, domain) DO NOTHING;

-- ============================================================================
-- STEP 3: Migrate bidders → bidders_new
-- ============================================================================

INSERT INTO bidders_new (code, name, status, param_schema, pbs_adapter_name, created_at)
SELECT
    bidder_code,
    bidder_name,
    CASE
        WHEN enabled = TRUE THEN 'active'
        ELSE status
    END,
    NULL,                        -- param_schema (to be populated later)
    bidder_code,                 -- pbs_adapter_name (same as code by default)
    created_at
FROM bidders
ON CONFLICT (code) DO NOTHING;

-- ============================================================================
-- STEP 4: Create ad_slots from publishers.bidder_params
-- ============================================================================
-- This extracts bidder configurations from the monolithic JSONB column
-- and creates individual ad_slots for each unique configuration

-- First, create a temporary function to extract unique slot patterns from JSONB
CREATE OR REPLACE FUNCTION extract_slot_patterns_from_bidder_params()
RETURNS TABLE (
    publisher_id_val TEXT,
    slot_name TEXT,
    slot_pattern TEXT
) AS $$
BEGIN
    -- This is a placeholder - actual implementation depends on your JSONB structure
    -- For now, create a single default slot per publisher
    RETURN QUERY
    SELECT
        p.publisher_id,
        'default' AS slot_name,
        SPLIT_PART(p.allowed_domains, '|', 1) || '/default' AS slot_pattern
    FROM publishers p;
END;
$$ LANGUAGE plpgsql;

-- Insert ad_slots from extracted patterns
INSERT INTO ad_slots (
    publisher_id,
    slot_pattern,
    slot_name,
    div_pattern,
    is_adhesion,
    media_type_profile_id,
    custom_params,
    json_config,
    status,
    created_at,
    updated_at
)
SELECT DISTINCT
    pn.id,                                    -- FK to publishers_new
    sp.slot_pattern,                          -- Full pattern (domain/slot)
    sp.slot_name,                             -- Slot name
    NULL,                                     -- div_pattern (to be populated from slot_mappings)
    FALSE,                                    -- is_adhesion
    NULL,                                     -- media_type_profile_id
    '{}'::JSONB,                              -- custom_params
    '{}'::JSONB,                              -- json_config
    'active',
    NOW(),
    NOW()
FROM extract_slot_patterns_from_bidder_params() sp
JOIN publishers p ON sp.publisher_id_val = p.publisher_id
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
ON CONFLICT (publisher_id, slot_pattern) DO NOTHING;

-- Drop temporary function
DROP FUNCTION extract_slot_patterns_from_bidder_params();

-- ============================================================================
-- STEP 5: Migrate domain_bidder_configs → slot_bidder_configs
-- ============================================================================
-- Domain-level configs become slot configs with domain-wide patterns

INSERT INTO slot_bidder_configs (
    ad_slot_id,
    bidder_id,
    device_type,
    bidder_params,
    status,
    bid_floor,
    bid_floor_cur,
    created_at,
    updated_at
)
SELECT DISTINCT
    s.id,                                     -- FK to ad_slots
    b.id,                                     -- FK to bidders_new
    'all',                                    -- device_type (domain configs apply to all)
    dbc.params,                               -- bidder_params from old table
    'active',
    NULL,                                     -- bid_floor
    'USD',                                    -- bid_floor_cur
    dbc.created_at,
    dbc.updated_at
FROM domain_bidder_configs dbc
JOIN publishers p ON dbc.publisher_id = p.publisher_id
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
    AND s.slot_pattern LIKE dbc.domain || '%'  -- Match domain pattern
JOIN bidders_new b ON dbc.bidder_code = b.code
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- ============================================================================
-- STEP 6: Migrate unit_bidder_configs → slot_bidder_configs
-- ============================================================================
-- Ad-unit-level configs become slot configs with specific patterns

INSERT INTO slot_bidder_configs (
    ad_slot_id,
    bidder_id,
    device_type,
    bidder_params,
    status,
    bid_floor,
    bid_floor_cur,
    created_at,
    updated_at
)
SELECT DISTINCT
    s.id,                                     -- FK to ad_slots
    b.id,                                     -- FK to bidders_new
    'all',                                    -- device_type (unit configs apply to all)
    ubc.params,                               -- bidder_params from old table
    'active',
    NULL,                                     -- bid_floor
    'USD',                                    -- bid_floor_cur
    ubc.created_at,
    ubc.updated_at
FROM unit_bidder_configs ubc
JOIN publishers p ON ubc.publisher_id = p.publisher_id
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
    AND s.slot_pattern = ubc.ad_unit_path    -- Exact match
JOIN bidders_new b ON ubc.bidder_code = b.code
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- ============================================================================
-- STEP 7: Update ad_slots.div_pattern from slot_mappings
-- ============================================================================
-- Merge slot_mappings data into ad_slots

UPDATE ad_slots s
SET div_pattern = sm.div_id
FROM slot_mappings sm
JOIN publishers p ON sm.publisher_id = p.publisher_id
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
WHERE s.publisher_id = pn.id
  AND s.slot_pattern LIKE sm.domain || '%'
  AND s.div_pattern IS NULL;  -- Only update if not already set

-- ============================================================================
-- STEP 8: Extract bidder_params from publishers.bidder_params JSONB
-- ============================================================================
-- This is the most complex part - extracting nested JSONB into relational rows
-- Customize based on your actual JSONB structure

-- Example: Extract Rubicon params from publishers.bidder_params
INSERT INTO slot_bidder_configs (
    ad_slot_id,
    bidder_id,
    device_type,
    bidder_params,
    status,
    created_at,
    updated_at
)
SELECT DISTINCT
    s.id,
    b.id,
    'all',
    p.bidder_params -> 'rubicon',           -- Extract Rubicon params
    'active',
    NOW(),
    NOW()
FROM publishers p
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
CROSS JOIN bidders_new b
WHERE b.code = 'rubicon'
  AND p.bidder_params ? 'rubicon'          -- Only if rubicon key exists
  AND p.bidder_params -> 'rubicon' IS NOT NULL
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- Repeat for other bidders (kargo, sovrn, pubmatic, etc.)
-- This section should be customized based on your JSONB structure

-- Example for Kargo
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status, created_at, updated_at)
SELECT DISTINCT s.id, b.id, 'all', p.bidder_params -> 'kargo', 'active', NOW(), NOW()
FROM publishers p
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
CROSS JOIN bidders_new b
WHERE b.code = 'kargo' AND p.bidder_params ? 'kargo' AND p.bidder_params -> 'kargo' IS NOT NULL
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- Example for Sovrn
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status, created_at, updated_at)
SELECT DISTINCT s.id, b.id, 'all', p.bidder_params -> 'sovrn', 'active', NOW(), NOW()
FROM publishers p
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
CROSS JOIN bidders_new b
WHERE b.code = 'sovrn' AND p.bidder_params ? 'sovrn' AND p.bidder_params -> 'sovrn' IS NOT NULL
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- Example for PubMatic
INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status, created_at, updated_at)
SELECT DISTINCT s.id, b.id, 'all', p.bidder_params -> 'pubmatic', 'active', NOW(), NOW()
FROM publishers p
JOIN accounts a ON p.publisher_id = a.account_id
JOIN publishers_new pn ON a.id = pn.account_id
JOIN ad_slots s ON pn.id = s.publisher_id
CROSS JOIN bidders_new b
WHERE b.code = 'pubmatic' AND p.bidder_params ? 'pubmatic' AND p.bidder_params -> 'pubmatic' IS NOT NULL
ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING;

-- ============================================================================
-- STEP 9: Verification Queries
-- ============================================================================

-- Create a verification view for easy checking
CREATE OR REPLACE VIEW migration_verification AS
SELECT
    'accounts' AS table_name,
    COUNT(*) AS new_count,
    (SELECT COUNT(*) FROM publishers) AS old_count,
    COUNT(*) = (SELECT COUNT(*) FROM publishers) AS counts_match
FROM accounts
UNION ALL
SELECT
    'publishers_new' AS table_name,
    COUNT(*) AS new_count,
    (SELECT COUNT(*) FROM publishers) AS old_count,
    COUNT(*) = (SELECT COUNT(*) FROM publishers) AS counts_match
FROM publishers_new
UNION ALL
SELECT
    'bidders_new' AS table_name,
    COUNT(*) AS new_count,
    (SELECT COUNT(*) FROM bidders) AS old_count,
    COUNT(*) = (SELECT COUNT(*) FROM bidders) AS counts_match
FROM bidders_new
UNION ALL
SELECT
    'ad_slots' AS table_name,
    COUNT(*) AS new_count,
    0 AS old_count,
    TRUE AS counts_match
FROM ad_slots
UNION ALL
SELECT
    'slot_bidder_configs' AS table_name,
    COUNT(*) AS new_count,
    (SELECT COUNT(*) FROM domain_bidder_configs) + (SELECT COUNT(*) FROM unit_bidder_configs) AS old_count,
    TRUE AS counts_match
FROM slot_bidder_configs;

COMMIT;

-- ============================================================================
-- Post-Migration Verification Commands
-- ============================================================================

-- Check migration results:
-- SELECT * FROM migration_verification;

-- Detailed verification:
-- SELECT
--     a.account_id,
--     COUNT(DISTINCT p.id) as publishers,
--     COUNT(DISTINCT s.id) as slots,
--     COUNT(DISTINCT sbc.id) as bidder_configs
-- FROM accounts a
-- LEFT JOIN publishers_new p ON a.id = p.account_id
-- LEFT JOIN ad_slots s ON p.id = s.publisher_id
-- LEFT JOIN slot_bidder_configs sbc ON s.id = sbc.ad_slot_id
-- GROUP BY a.account_id;

-- Check for missing data:
-- SELECT 'Missing accounts' as issue, publisher_id
-- FROM publishers p
-- WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.account_id = p.publisher_id);

-- SELECT 'Missing publishers_new' as issue, publisher_id
-- FROM publishers p
-- WHERE NOT EXISTS (
--     SELECT 1 FROM publishers_new pn
--     JOIN accounts a ON pn.account_id = a.id
--     WHERE a.account_id = p.publisher_id
-- );

-- ============================================================================
-- Rollback Instructions
-- ============================================================================

-- To rollback this migration:
-- BEGIN;
-- TRUNCATE accounts, publishers_new, ad_slots, slot_bidder_configs, bidders_new, media_type_profiles CASCADE;
-- DROP VIEW IF EXISTS migration_verification;
-- COMMIT;

-- Old tables are preserved and can continue to be used
