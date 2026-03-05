-- ============================================================
-- Migration 015: Add consent_hash to user_syncs
-- ============================================================
-- Track the TCF consent string hash at time of sync.
-- At bid time, UIDs whose stored consent hash differs from the
-- current request's consent hash are excluded (user may have
-- revoked or updated consent since syncing).
-- ============================================================

ALTER TABLE user_syncs
    ADD COLUMN IF NOT EXISTS consent_hash VARCHAR(64);

COMMENT ON COLUMN user_syncs.consent_hash IS 'SHA-256 hex of the TCF consent string present when the sync was stored. NULL = synced without GDPR consent context (non-EU traffic or pre-migration records).';

CREATE INDEX IF NOT EXISTS idx_user_syncs_consent_hash ON user_syncs(consent_hash);
