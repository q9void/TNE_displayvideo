-- Migration 021: Deal/curator dimensions in analytics + signal_receipts audit
-- Adds the columns and table needed to (a) attribute revenue to deal_id/curator_id,
-- and (b) prove signals were forwarded to each DSP for each curated deal.

-- ============================================================================
-- auction_events: roll-up curator participation per auction
-- ============================================================================
ALTER TABLE auction_events
    ADD COLUMN IF NOT EXISTS deal_count   INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS curator_ids  TEXT[] DEFAULT '{}'::TEXT[];

CREATE INDEX IF NOT EXISTS idx_auction_events_curator_ids ON auction_events USING GIN(curator_ids);

COMMENT ON COLUMN auction_events.deal_count IS 'Total imp.pmp.deals[] entries seen across all impressions in this auction';
COMMENT ON COLUMN auction_events.curator_ids IS 'Distinct curator IDs that contributed deals or signals to this auction';

-- ============================================================================
-- bidder_events: per-bidder deal/curator/seat attribution + signal sources
-- ============================================================================
ALTER TABLE bidder_events
    ADD COLUMN IF NOT EXISTS deal_id          TEXT,
    ADD COLUMN IF NOT EXISTS curator_id       TEXT,
    ADD COLUMN IF NOT EXISTS seat             TEXT,
    ADD COLUMN IF NOT EXISTS signal_sources   TEXT[] DEFAULT '{}'::TEXT[];

CREATE INDEX IF NOT EXISTS idx_bidder_events_deal_id ON bidder_events(deal_id) WHERE deal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bidder_events_curator_id ON bidder_events(curator_id) WHERE curator_id IS NOT NULL;

COMMENT ON COLUMN bidder_events.signal_sources IS 'Tags like "eid:liveramp.com", "seg:iab4:9001", "fpd:user.data" describing what was forwarded';

-- ============================================================================
-- win_events: revenue attribution by deal + curator
-- ============================================================================
ALTER TABLE win_events
    ADD COLUMN IF NOT EXISTS deal_id     TEXT,
    ADD COLUMN IF NOT EXISTS curator_id  TEXT,
    ADD COLUMN IF NOT EXISTS seat        TEXT;

CREATE INDEX IF NOT EXISTS idx_win_events_deal_id ON win_events(deal_id) WHERE deal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_win_events_curator_id ON win_events(curator_id) WHERE curator_id IS NOT NULL;

-- ============================================================================
-- signal_receipts: forensic record of signals sent to a DSP for a deal
-- One row per (auction, bidder, deal). Source of truth for the
-- /admin/curators/{id}/signal-receipts audit endpoint.
-- ============================================================================
CREATE TABLE IF NOT EXISTS signal_receipts (
    id                  BIGSERIAL PRIMARY KEY,
    auction_id          VARCHAR(255) NOT NULL,
    bidder_code         VARCHAR(100) NOT NULL,
    deal_id             TEXT NOT NULL,
    curator_id          TEXT,
    seat                TEXT,
    eids_sent           TEXT[] DEFAULT '{}'::TEXT[],     -- EID source domains forwarded (e.g. {"liveramp.com","audigent.com"})
    segments_sent       TEXT[] DEFAULT '{}'::TEXT[],     -- "iab<segtax>:<segment_id>" tags
    schain_nodes_sent   JSONB,                            -- Final source.ext.schain.nodes serialized to this bidder
    acknowledged_at     TIMESTAMP,                        -- Set when bid.ext.signal_receipt arrives back from adapter
    sent_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_signal_receipts_auction ON signal_receipts(auction_id);
CREATE INDEX IF NOT EXISTS idx_signal_receipts_curator ON signal_receipts(curator_id) WHERE curator_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_signal_receipts_deal ON signal_receipts(deal_id);
CREATE INDEX IF NOT EXISTS idx_signal_receipts_sent_at ON signal_receipts(sent_at);

COMMENT ON TABLE signal_receipts IS 'Forensic audit of signals (EIDs, segments, schain) forwarded to each DSP per curated deal';
COMMENT ON COLUMN signal_receipts.acknowledged_at IS 'Timestamp the DSP echoed back via bid.ext.signal_receipt (NULL = unacknowledged)';

-- ============================================================================
-- Migration complete
-- ============================================================================
SELECT 'Migration 021 complete: deal/curator columns + signal_receipts table' AS message;
