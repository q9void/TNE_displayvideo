-- Migration 020: Curator catalog tables
-- Adds first-class support for curated deals: curators register their deals,
-- the seats they hold against TNE adapters, and the publishers they may serve.
-- The auction layer hydrates inbound imp.pmp.deals[] against this catalog,
-- restricts fanout to wseat-permitted bidders, and preserves curator-injected
-- signals through the per-bidder request clone.

-- ============================================================================
-- curators: top-level identity for a third-party deal/data packager
-- ============================================================================
CREATE TABLE IF NOT EXISTS curators (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    status       VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    schain_asi   TEXT NOT NULL,
    schain_sid   TEXT NOT NULL,
    notes        TEXT,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_curators_status ON curators(status);

COMMENT ON TABLE curators IS 'Third-party deal/data packagers that supply curated deals to TNE Catalyst';
COMMENT ON COLUMN curators.schain_asi IS 'SChain authoritative seller ID (ASI) appended to source.ext.schain.nodes';
COMMENT ON COLUMN curators.schain_sid IS 'SChain seller-of-record ID (SID) for the curator node';

-- ============================================================================
-- curator_deals: deal_id -> curator + deal-level overrides
-- ============================================================================
CREATE TABLE IF NOT EXISTS curator_deals (
    deal_id         TEXT PRIMARY KEY,
    curator_id      TEXT NOT NULL REFERENCES curators(id) ON DELETE CASCADE,
    bidfloor        NUMERIC,
    bidfloorcur     VARCHAR(10) DEFAULT 'USD',
    at              INTEGER,                       -- OpenRTB auction type override (1=first-price, 2=second-price)
    wseat           TEXT[] DEFAULT '{}'::TEXT[],   -- Allowed buyer seats (passed through to imp.pmp.deals[].wseat)
    wadomain        TEXT[] DEFAULT '{}'::TEXT[],   -- Allowed advertiser domains
    segtax_allowed  INTEGER[] DEFAULT '{}'::INTEGER[],  -- IAB segment taxonomy IDs the curator may inject (e.g. {4})
    cattax_allowed  INTEGER[] DEFAULT '{}'::INTEGER[],  -- IAB content category taxonomy IDs the curator may inject
    ext             JSONB,                          -- Per-deal extensions (forwarded into imp.pmp.deals[].ext)
    active          BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_curator_deals_curator ON curator_deals(curator_id);
CREATE INDEX IF NOT EXISTS idx_curator_deals_active ON curator_deals(active) WHERE active = TRUE;

COMMENT ON TABLE curator_deals IS 'Deal catalog: maps deal_id to its curator and overlay metadata';
COMMENT ON COLUMN curator_deals.wseat IS 'Buyer seat allow-list - inbound request wins on conflict, catalog hydrates if absent';
COMMENT ON COLUMN curator_deals.segtax_allowed IS 'IAB Tech Lab segment taxonomy IDs (SDA) this curator is permitted to inject';

-- ============================================================================
-- curator_seats: maps a curator's logical seat to a TNE adapter+seat tuple
-- ============================================================================
CREATE TABLE IF NOT EXISTS curator_seats (
    curator_id   TEXT NOT NULL REFERENCES curators(id) ON DELETE CASCADE,
    bidder_code  TEXT NOT NULL,                    -- TNE adapter code (rubicon, pubmatic, dv360, ttd, ...)
    seat_id      TEXT NOT NULL,                    -- Buyer seat ID at that adapter
    notes        TEXT,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (curator_id, bidder_code, seat_id)
);

CREATE INDEX IF NOT EXISTS idx_curator_seats_bidder ON curator_seats(bidder_code);

COMMENT ON TABLE curator_seats IS 'Curator -> (adapter, seat_id) bindings; drives wseat hydration and fanout filtering';

-- ============================================================================
-- curator_publisher_allowlist: which publishers may receive a curator's deals
-- ============================================================================
CREATE TABLE IF NOT EXISTS curator_publisher_allowlist (
    curator_id    TEXT NOT NULL REFERENCES curators(id) ON DELETE CASCADE,
    publisher_id  INTEGER NOT NULL REFERENCES publishers_new(id) ON DELETE CASCADE,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (curator_id, publisher_id)
);

CREATE INDEX IF NOT EXISTS idx_curator_publisher_allowlist_publisher ON curator_publisher_allowlist(publisher_id);

COMMENT ON TABLE curator_publisher_allowlist IS 'Per-publisher curator allow-list (empty list for a publisher means no curators allowed)';

-- ============================================================================
-- Migration complete
-- ============================================================================
SELECT 'Migration 020 complete: curators, curator_deals, curator_seats, curator_publisher_allowlist' AS message;
