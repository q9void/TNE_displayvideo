-- Account-level bidder defaults: stores SSP params that are shared across all
-- slots for a given account+bidder combination (e.g. Rubicon accountId/siteId).
-- At auction time these are merged with slot_bidder_configs (slot params win on conflict).

CREATE TABLE IF NOT EXISTS account_bidder_defaults (
    id          SERIAL PRIMARY KEY,
    account_id  INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    bidder_id   INTEGER NOT NULL REFERENCES bidders_new(id) ON DELETE CASCADE,
    base_params JSONB NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, bidder_id)
);

CREATE INDEX IF NOT EXISTS idx_account_bidder_defaults_account ON account_bidder_defaults(account_id);
