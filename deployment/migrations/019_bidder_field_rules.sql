-- 019_bidder_field_rules.sql
-- Stores per-bidder OpenRTB field routing rules.
-- Rules with bidder_code = '__default__' apply to all bidders as a baseline.
-- Bidder-specific rules override defaults on the same field_path.

CREATE TABLE IF NOT EXISTS bidder_field_rules (
    id           SERIAL PRIMARY KEY,
    bidder_id    INTEGER REFERENCES bidders_new(id) ON DELETE CASCADE,
    -- NULL when bidder_code = '__default__' (no bidders_new row for the pseudo-bidder)
    bidder_code  TEXT NOT NULL,
    field_path   TEXT NOT NULL,
    source_type  TEXT NOT NULL CHECK (source_type IN (
                   'standard','sdk_param','http_context',
                   'account_param','slot_param','eid','constant')),
    source_ref   TEXT,
    CONSTRAINT source_ref_required CHECK (
        source_type = 'standard' OR source_ref IS NOT NULL
    ),
    transform    TEXT NOT NULL DEFAULT 'none',
    required     BOOLEAN NOT NULL DEFAULT false,
    enabled      BOOLEAN NOT NULL DEFAULT true,
    notes        TEXT,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(bidder_code, field_path)
);

CREATE INDEX IF NOT EXISTS idx_bfr_bidder_id   ON bidder_field_rules(bidder_id);
CREATE INDEX IF NOT EXISTS idx_bfr_bidder_code ON bidder_field_rules(bidder_code);
CREATE INDEX IF NOT EXISTS idx_bfr_enabled     ON bidder_field_rules(enabled) WHERE enabled = true;

-- ── Default baseline rules ──────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_code, field_path, source_type, source_ref, required, notes) VALUES
  ('__default__', 'device.ua',        'http_context',  'User-Agent',         false, 'Set from request header'),
  ('__default__', 'device.ip',        'http_context',  'X-Forwarded-For',    false, 'First IP in chain'),
  ('__default__', 'device.language',  'http_context',  'Accept-Language',    false, NULL),
  ('__default__', 'site.page',        'sdk_param',     'pageUrl',            false, NULL),
  ('__default__', 'site.domain',      'sdk_param',     'domain',             false, NULL),
  ('__default__', 'regs.gdpr',        'standard',      NULL,                 false, NULL),
  ('__default__', 'regs.us_privacy',  'standard',      NULL,                 false, NULL),
  ('__default__', 'regs.gpp',         'standard',      NULL,                 false, NULL),
  ('__default__', 'regs.gpp_sid',     'standard',      NULL,                 false, NULL),
  ('__default__', 'regs.coppa',       'standard',      NULL,                 false, NULL),
  ('__default__', 'source.schain',    'standard',      NULL,                 false, NULL),
  ('__default__', 'user.consent',     'standard',      NULL,                 false, 'TCF string pass-through'),
  ('__default__', 'user.eids',        'standard',      NULL,                 false, 'Full EID array pass-through'),
  ('__default__', 'tmax',             'account_param', 'default_timeout_ms', false, NULL)
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── Kargo ───────────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.kargo.placementId', 'slot_param', 'placementId', true,  NULL),
  ('user.buyeruid',             'eid',        'kargo.com',   false, 'fallback: user.ext.eids if user.eids absent')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'kargo'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── Rubicon / Magnite ───────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, transform, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.transform, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.rubicon.accountId',   'slot_param',    'accountId',          'to_int',    true,  NULL),
  ('imp.ext.rubicon.siteId',      'slot_param',    'siteId',             'to_int',    true,  NULL),
  ('imp.ext.rubicon.zoneId',      'slot_param',    'zoneId',             'to_int',    true,  NULL),
  ('site.publisher.id',           'slot_param',    'accountId',          'to_string', true,  'Rubicon uses accountId as publisher ID'),
  ('user.buyeruid',               'eid',           'rubiconproject.com', 'none',      false, NULL)
) AS r(field_path, source_type, source_ref, transform, required, notes)
WHERE b.code = 'rubicon'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── Pubmatic ────────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.pubmatic.publisherId', 'slot_param', 'publisherId',  true,  NULL),
  ('imp.ext.pubmatic.adSlot',      'slot_param', 'adSlot',       true,  NULL),
  ('user.buyeruid',                'eid',        'pubmatic.com', false, NULL)
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'pubmatic'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── Sovrn ───────────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, transform, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.transform, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.sovrn.tagid', 'slot_param', 'tagid', 'to_string', true, NULL)
) AS r(field_path, source_type, source_ref, transform, required, notes)
WHERE b.code = 'sovrn'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── TrippleLift ─────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.triplelift.inventoryCode', 'slot_param', 'inventoryCode', true, NULL)
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'triplelift'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── AppNexus / Xandr ────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
CROSS JOIN (VALUES
  ('imp.ext.appnexus.placementId', 'slot_param', 'placementId', true,  NULL),
  ('imp.ext.appnexus.member',      'slot_param', 'member',      false, 'Alternative to placementId'),
  ('imp.ext.appnexus.invCode',     'slot_param', 'invCode',     false, 'Used with member')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'appnexus'
ON CONFLICT (bidder_code, field_path) DO NOTHING;
