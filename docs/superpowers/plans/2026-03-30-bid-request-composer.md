# Bid Request Composer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a DB-backed OpenRTB field routing system with a visual admin editor (Phase 1) and a runtime `BidRequestComposer` that replaces hardcoded adapter logic (Phase 2).

**Architecture:** Phase 1 stores `bidder_field_rules` in Postgres and surfaces them as an editable routing table in the Catalyst admin Vue SPA — no runtime behaviour changes yet. Phase 2 adds `internal/adapters/routing/` (Loader → Composer → RuleApplier + EIDResolver) and migrates five simple adapters (Kargo, Sovrn, Pubmatic, TrippleLift, AppNexus) to use the engine; Rubicon keeps its Go adapter for `ext.rp` nesting and Basic auth but delegates standard fields to the engine.

**Tech Stack:** Go 1.23, PostgreSQL, Vue 3 (CDN, Options API), `database/sql` + `lib/pq`, stdlib `net/http`, module `github.com/thenexusengine/tne_springwire`

**Spec:** `docs/superpowers/specs/2026-03-30-bid-request-composer-design.md`

---

## File Map

```
deployment/migrations/
  019_bidder_field_rules.sql          NEW  DB table + seed data

internal/storage/
  bidder_field_rules.go               NEW  BidderFieldRule struct + CRUD methods

internal/endpoints/
  onboarding_admin.go                 MOD  3 new routes + __BIDDER_FIELD_RULES__ injection + Bid Request tab HTML
  onboarding_admin_preview.go         NEW  GET /bid-request-preview handler

internal/adapters/routing/
  loader.go                           NEW  Phase 2: DB-backed rule cache
  composer.go                         NEW  Phase 2: Composer.Apply()
  rule_applier.go                     NEW  Phase 2: per-source-type handlers
  eid_resolver.go                     NEW  Phase 2: EID → buyeruid lookup

internal/adapters/
  kargo/kargo.go                      MOD  Phase 2: remove EID + field logic → Composer
  sovrn/sovrn.go                      MOD  Phase 2: simplify to Composer pass-through
  pubmatic/pubmatic.go                MOD  Phase 2: simplify to Composer pass-through
  triplelift/triplelift.go            MOD  Phase 2: simplify to Composer pass-through
  appnexus/appnexus.go                MOD  Phase 2: simplify to Composer pass-through
  rubicon/rubicon.go                  MOD  Phase 2: remove standard-field logic, keep ext.rp + auth
```

---

## ═══════════════ PHASE 1 ═══════════════

---

### Task 1: DB Migration

**Files:**
- Create: `deployment/migrations/019_bidder_field_rules.sql`

- [ ] **Step 1: Create the migration file**

```sql
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
JOIN (VALUES
  ('imp.ext.kargo.placementId', 'slot_param', 'placementId', true,  NULL),
  ('user.buyeruid',             'eid',        'kargo.com',   false, 'fallback: user.ext.eids if user.eids absent')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'kargo'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── Rubicon / Magnite ───────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, transform, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.transform, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
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
JOIN (VALUES
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
JOIN (VALUES
  ('imp.ext.sovrn.tagid', 'slot_param', 'tagid', 'to_string', true, NULL)
) AS r(field_path, source_type, source_ref, transform, required, notes)
WHERE b.code = 'sovrn'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── TrippleLift ─────────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.triplelift.inventoryCode', 'slot_param', 'inventoryCode', true, NULL)
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'triplelift'
ON CONFLICT (bidder_code, field_path) DO NOTHING;

-- ── AppNexus / Xandr ────────────────────────────────────────────────────────
INSERT INTO bidder_field_rules (bidder_id, bidder_code, field_path, source_type, source_ref, required, notes)
SELECT b.id, b.code, r.field_path, r.source_type, r.source_ref, r.required, r.notes
FROM bidders_new b
JOIN (VALUES
  ('imp.ext.appnexus.placementId', 'slot_param', 'placementId', true,  NULL),
  ('imp.ext.appnexus.member',      'slot_param', 'member',      false, 'Alternative to placementId'),
  ('imp.ext.appnexus.invCode',     'slot_param', 'invCode',     false, 'Used with member')
) AS r(field_path, source_type, source_ref, required, notes)
WHERE b.code = 'appnexus'
ON CONFLICT (bidder_code, field_path) DO NOTHING;
```

- [ ] **Step 2: Apply migration to production**

```bash
cat deployment/migrations/019_bidder_field_rules.sql | ssh catalyst \
  "PGPASSWORD=ttlJRsJK7myCehgKyswnZP82v6L57xT5 docker run --rm -i \
   --network catalyst_catalyst-network postgres:15-alpine \
   psql -h catalyst-postgres -U catalyst_prod -d catalyst_production"
```

Expected: `CREATE TABLE`, `CREATE INDEX` (×3), then `INSERT 0 14`, `INSERT 0 2`, etc. for each bidder block. No errors.

- [ ] **Step 3: Verify table structure**

```bash
echo '\d bidder_field_rules' | ssh catalyst \
  "PGPASSWORD=ttlJRsJK7myCehgKyswnZP82v6L57xT5 docker run --rm -i \
   --network catalyst_catalyst-network postgres:15-alpine \
   psql -h catalyst-postgres -U catalyst_prod -d catalyst_production"
```

Expected: table with columns `id, bidder_id, bidder_code, field_path, source_type, source_ref, transform, required, enabled, notes, updated_at`.

- [ ] **Step 4: Commit**

```bash
git add deployment/migrations/019_bidder_field_rules.sql
git commit -m "feat(db): add bidder_field_rules table with seed data for all active SSPs"
```

---

### Task 2: Storage Layer

**Files:**
- Create: `internal/storage/bidder_field_rules.go`
- Create: `internal/storage/bidder_field_rules_test.go`

- [ ] **Step 1: Write failing tests first**

Create `internal/storage/bidder_field_rules_test.go`:

```go
package storage_test

import (
	"context"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// These are unit tests against the struct and method signatures only.
// Integration tests against a live DB are in the CI pipeline.

func TestBidderFieldRule_IsDefault(t *testing.T) {
	r := storage.BidderFieldRule{BidderCode: "__default__"}
	if !r.IsDefault() {
		t.Error("expected IsDefault() true for __default__")
	}
	r2 := storage.BidderFieldRule{BidderCode: "kargo"}
	if r2.IsDefault() {
		t.Error("expected IsDefault() false for kargo")
	}
}

func TestBidderFieldRule_Validate(t *testing.T) {
	cases := []struct {
		name    string
		rule    storage.BidderFieldRule
		wantErr bool
	}{
		{
			name:    "standard with no source_ref is valid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.eids", SourceType: "standard"},
			wantErr: false,
		},
		{
			name:    "slot_param with no source_ref is invalid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.buyeruid", SourceType: "slot_param"},
			wantErr: true,
		},
		{
			name:    "eid with source_ref is valid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.buyeruid", SourceType: "eid", SourceRef: strPtr("kargo.com")},
			wantErr: false,
		},
		{
			name:    "unknown source_type is invalid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "x", SourceType: "magic"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// Integration test stub — skipped unless DB is available.
func TestGetBidderFieldRules_Integration(t *testing.T) {
	t.Skip("requires live DB — run manually or in CI with DB service")
	_ = context.Background()
	// Usage pattern:
	// store := storage.NewPublisherStore(db)
	// rules, err := store.GetBidderFieldRules(ctx, "kargo")
	// if err != nil { t.Fatal(err) }
	// if len(rules) == 0 { t.Error("expected kargo rules") }
}
```

- [ ] **Step 2: Run tests — confirm they fail (struct not defined yet)**

```bash
cd /Users/andrewstreets/tnevideo
go test ./internal/storage/... 2>&1 | head -20
```

Expected: `undefined: storage.BidderFieldRule` or similar.

- [ ] **Step 3: Create the storage file**

Create `internal/storage/bidder_field_rules.go`:

```go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// validSourceTypes is the set of allowed source_type values.
var validSourceTypes = map[string]bool{
	"standard": true, "sdk_param": true, "http_context": true,
	"account_param": true, "slot_param": true, "eid": true, "constant": true,
}

// BidderFieldRule represents one row in bidder_field_rules.
type BidderFieldRule struct {
	ID          int        `json:"id"`
	BidderID    *int       `json:"bidder_id"` // nil for __default__
	BidderCode  string     `json:"bidder_code"`
	FieldPath   string     `json:"field_path"`
	SourceType  string     `json:"source_type"`
	SourceRef   *string    `json:"source_ref"`
	Transform   string     `json:"transform"`
	Required    bool       `json:"required"`
	Enabled     bool       `json:"enabled"`
	Notes       *string    `json:"notes"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsDefault returns true if this is a baseline rule (bidder_code = '__default__').
func (r *BidderFieldRule) IsDefault() bool {
	return r.BidderCode == "__default__"
}

// Validate checks that the rule is self-consistent.
func (r *BidderFieldRule) Validate() error {
	if !validSourceTypes[r.SourceType] {
		return fmt.Errorf("unknown source_type %q", r.SourceType)
	}
	if r.SourceType != "standard" && (r.SourceRef == nil || *r.SourceRef == "") {
		return fmt.Errorf("source_ref required for source_type %q", r.SourceType)
	}
	if r.FieldPath == "" {
		return fmt.Errorf("field_path must not be empty")
	}
	if r.BidderCode == "" {
		return fmt.Errorf("bidder_code must not be empty")
	}
	return nil
}

// GetBidderFieldRules returns rules for the given bidder merged over __default__ rules.
// Bidder-specific rules override defaults on the same field_path.
// Pass bidderCode = "__default__" to get only baseline rules.
func (s *PublisherStore) GetBidderFieldRules(ctx context.Context, bidderCode string) ([]BidderFieldRule, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT id, bidder_id, bidder_code, field_path, source_type, source_ref,
		       transform, required, enabled, notes, updated_at
		FROM bidder_field_rules
		WHERE (bidder_code = '__default__' OR bidder_code = $1)
		  AND enabled = true
		ORDER BY bidder_code DESC, field_path ASC
	`
	// ORDER BY bidder_code DESC puts bidder-specific rows before __default__ so we can
	// deduplicate: first row seen for each field_path wins.

	rows, err := s.db.QueryContext(ctx, query, bidderCode)
	if err != nil {
		return nil, fmt.Errorf("GetBidderFieldRules: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var result []BidderFieldRule
	for rows.Next() {
		var r BidderFieldRule
		if err := rows.Scan(
			&r.ID, &r.BidderID, &r.BidderCode, &r.FieldPath,
			&r.SourceType, &r.SourceRef, &r.Transform,
			&r.Required, &r.Enabled, &r.Notes, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetBidderFieldRules scan: %w", err)
		}
		if !seen[r.FieldPath] {
			seen[r.FieldPath] = true
			result = append(result, r)
		}
	}
	return result, rows.Err()
}

// GetAllBidderFieldRules returns all rules (all bidders) — used by the admin UI.
func (s *PublisherStore) GetAllBidderFieldRules(ctx context.Context) ([]BidderFieldRule, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT id, bidder_id, bidder_code, field_path, source_type, source_ref,
		       transform, required, enabled, notes, updated_at
		FROM bidder_field_rules
		ORDER BY bidder_code ASC, field_path ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("GetAllBidderFieldRules: %w", err)
	}
	defer rows.Close()

	var result []BidderFieldRule
	for rows.Next() {
		var r BidderFieldRule
		if err := rows.Scan(
			&r.ID, &r.BidderID, &r.BidderCode, &r.FieldPath,
			&r.SourceType, &r.SourceRef, &r.Transform,
			&r.Required, &r.Enabled, &r.Notes, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetAllBidderFieldRules scan: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpsertBidderFieldRule inserts or updates a rule. Returns the saved rule with its ID.
func (s *PublisherStore) UpsertBidderFieldRule(ctx context.Context, rule BidderFieldRule) (BidderFieldRule, error) {
	if err := rule.Validate(); err != nil {
		return BidderFieldRule{}, fmt.Errorf("invalid rule: %w", err)
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		INSERT INTO bidder_field_rules
		  (bidder_id, bidder_code, field_path, source_type, source_ref, transform, required, enabled, notes, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (bidder_code, field_path) DO UPDATE SET
		  source_type = EXCLUDED.source_type,
		  source_ref  = EXCLUDED.source_ref,
		  transform   = EXCLUDED.transform,
		  required    = EXCLUDED.required,
		  enabled     = EXCLUDED.enabled,
		  notes       = EXCLUDED.notes,
		  updated_at  = NOW()
		RETURNING id, updated_at
	`
	err := s.db.QueryRowContext(ctx, query,
		rule.BidderID, rule.BidderCode, rule.FieldPath,
		rule.SourceType, rule.SourceRef, rule.Transform,
		rule.Required, rule.Enabled, rule.Notes,
	).Scan(&rule.ID, &rule.UpdatedAt)
	if err != nil {
		return BidderFieldRule{}, fmt.Errorf("UpsertBidderFieldRule: %w", err)
	}
	return rule, nil
}

// DeleteBidderFieldRule hard-deletes a rule by ID.
// Seed data rows (from the migration) can be re-created by re-running the migration.
func (s *PublisherStore) DeleteBidderFieldRule(ctx context.Context, id int) error {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM bidder_field_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("DeleteBidderFieldRule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule id %d not found", id)
	}
	return nil
}

// LookupBidderIDByCode is a helper used by the upsert handler to resolve bidder_id from code.
func (s *PublisherStore) LookupBidderIDByCode(ctx context.Context, code string) (*int, error) {
	if code == "__default__" {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	var id int
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM bidders_new WHERE code = $1`, code,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil // unknown bidder — allow rule creation anyway
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/storage/... -run TestBidderFieldRule -v
```

Expected:
```
--- PASS: TestBidderFieldRule_IsDefault (0.00s)
--- PASS: TestBidderFieldRule_Validate (0.00s)
PASS
```

- [ ] **Step 5: Compile check**

```bash
go build ./...
```

Expected: no output (clean compile).

- [ ] **Step 6: Commit**

```bash
git add internal/storage/bidder_field_rules.go internal/storage/bidder_field_rules_test.go
git commit -m "feat(storage): add BidderFieldRule CRUD — GetBidderFieldRules, UpsertBidderFieldRule, DeleteBidderFieldRule"
```

---

### Task 3: API Routes + Server-Side Injection

**Files:**
- Modify: `internal/endpoints/onboarding_admin.go`

The admin handler uses a `switch` on `sub` (URL path relative to basePath). Add three new cases and extend `serveUI` to inject `__BIDDER_FIELD_RULES__`.

- [ ] **Step 1: Add the switch cases to `ServeHTTP`**

Find the block ending in:
```go
	case strings.HasPrefix(sub, "/account-defaults/"):
		if r.Method == http.MethodPut {
			h.upsertAccountDefault(w, r, strings.TrimPrefix(sub, "/account-defaults/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
```

Replace with:
```go
	case strings.HasPrefix(sub, "/account-defaults/"):
		if r.Method == http.MethodPut {
			h.upsertAccountDefault(w, r, strings.TrimPrefix(sub, "/account-defaults/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/bidder-field-rules":
		switch r.Method {
		case http.MethodGet:
			h.getBidderFieldRules(w, r)
		case http.MethodPut:
			h.upsertBidderFieldRule(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/bidder-field-rules/"):
		if r.Method == http.MethodDelete {
			h.deleteBidderFieldRule(w, r, strings.TrimPrefix(sub, "/bidder-field-rules/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/bid-request-preview":
		if r.Method == http.MethodGet {
			h.bidRequestPreview(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
```

- [ ] **Step 2: Extend `serveUI` to load and inject bidder field rules**

In `serveUI`, after the `accountDefaults` block, add:

```go
	fieldRules, err := h.store.GetAllBidderFieldRules(ctx)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("onboarding admin: failed to load bidder field rules")
		fieldRules = []storage.BidderFieldRule{}
	}
	if fieldRules == nil {
		fieldRules = []storage.BidderFieldRule{}
	}
	fieldRulesJSON, _ := json.Marshal(fieldRules)
```

Then add this line to the `strings.ReplaceAll` block:
```go
	page = strings.ReplaceAll(page, `"__BIDDER_FIELD_RULES__"`, string(fieldRulesJSON))
```

- [ ] **Step 3: Add the three handler methods**

Append to `onboarding_admin.go` (before the large `onboardingHTML` const):

```go
// ─── Bidder Field Rules ───────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) getBidderFieldRules(w http.ResponseWriter, r *http.Request) {
	bidderCode := r.URL.Query().Get("bidder_code")
	ctx := r.Context()
	var (
		rules []storage.BidderFieldRule
		err   error
	)
	if bidderCode != "" {
		rules, err = h.store.GetBidderFieldRules(ctx, bidderCode)
	} else {
		rules, err = h.store.GetAllBidderFieldRules(ctx)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []storage.BidderFieldRule{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules) //nolint:errcheck
}

func (h *OnboardingAdminHandler) upsertBidderFieldRule(w http.ResponseWriter, r *http.Request) {
	var rule storage.BidderFieldRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	// Resolve bidder_id from code so callers don't need to know the DB id.
	bidderID, err := h.store.LookupBidderIDByCode(ctx, rule.BidderCode)
	if err != nil {
		logger.Log.Warn().Err(err).Str("code", rule.BidderCode).Msg("upsertBidderFieldRule: bidder lookup failed")
	}
	rule.BidderID = bidderID

	saved, err := h.store.UpsertBidderFieldRule(ctx, rule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(saved) //nolint:errcheck
}

func (h *OnboardingAdminHandler) deleteBidderFieldRule(w http.ResponseWriter, r *http.Request, idStr string) {
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid rule id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteBidderFieldRule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Add the preview stub (flesh out in Task 4)**

```go
func (h *OnboardingAdminHandler) bidRequestPreview(w http.ResponseWriter, r *http.Request) {
	// Phase 1 stub: return the rules for the requested bidder as pretty JSON.
	// Phase 2 will assemble a real OpenRTB request.
	bidderCode := r.URL.Query().Get("bidder")
	if bidderCode == "" {
		http.Error(w, "bidder param required", http.StatusBadRequest)
		return
	}
	rules, err := h.store.GetBidderFieldRules(r.Context(), bidderCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]interface{}{
		"bidder": bidderCode,
		"note":   "Phase 1 stub — shows active rules. Phase 2 will return assembled OpenRTB JSON.",
		"rules":  rules,
	}) //nolint:errcheck
}
```

- [ ] **Step 5: Build check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/endpoints/onboarding_admin.go
git commit -m "feat(api): add /bidder-field-rules CRUD routes + /bid-request-preview stub + __BIDDER_FIELD_RULES__ injection"
```

---

### Task 4: Admin UI — "Bid Request" Tab

**Files:**
- Modify: `internal/endpoints/onboarding_admin.go` (the `onboardingHTML` const)

This is the largest single task. Work section by section.

- [ ] **Step 1: Add `bidderFieldRules` to Vue `data()`**

Find:
```js
      exportCopied:  null,
```

Add after it:
```js
      bidderFieldRules: "__BIDDER_FIELD_RULES__",
      bfrBidder:     '__default__',
      bfrNewRow:     null,
      bfrSaving:     false,
      bfrErr:        '',
```

- [ ] **Step 2: Add "Bid Request" to the nav array**

Find:
```js
        { id:'tags',     label:'Export Tags',      addLabel:null,          addModal:null         },
```

Add after it:
```js
        { id:'bidrules', label:'Bid Request',       addLabel:null,          addModal:null         },
```

- [ ] **Step 3: Add computed properties for the Bid Request tab**

In the `computed:` block, add:

```js
    bfrForBidder: function() {
      var self = this;
      var defaults = {};
      this.bidderFieldRules.filter(function(r){ return r.bidder_code === '__default__'; })
        .forEach(function(r){ defaults[r.field_path] = r; });
      var bidderRules = {};
      this.bidderFieldRules.filter(function(r){ return r.bidder_code === self.bfrBidder && r.bidder_code !== '__default__'; })
        .forEach(function(r){ bidderRules[r.field_path] = r; });
      // Merge: bidder-specific wins
      var merged = Object.assign({}, defaults, bidderRules);
      // Return array sorted by field_path, with override flag
      return Object.values(merged).sort(function(a,b){ return a.field_path.localeCompare(b.field_path); })
        .map(function(r){
          return Object.assign({}, r, { _isOverride: !!bidderRules[r.field_path] && r.bidder_code !== '__default__' });
        });
    },
    bfrSections: function() {
      var sections = [
        'BidRequest', 'Imp', 'Banner', 'Video', 'Audio', 'Native',
        'Site', 'App', 'User', 'Device', 'Regs', 'Source'
      ];
      var self = this;
      return sections.map(function(s) {
        var prefix = s === 'BidRequest' ? '' : s.toLowerCase() + '.';
        var rules = self.bfrForBidder.filter(function(r){
          if (s === 'BidRequest') return !r.field_path.includes('.');
          return r.field_path.startsWith(prefix);
        });
        return { label: s, rules: rules };
      }).filter(function(s){ return s.rules.length > 0; });
    },
    bfrAllBidderCodes: function() {
      var codes = ['__default__'];
      this.bidders.forEach(function(b){ if (b.code) codes.push(b.code); });
      return codes;
    },
```

- [ ] **Step 4: Add Vue methods for the Bid Request tab**

In the `methods:` block, add:

```js
    bfrSave: async function(rule) {
      this.bfrSaving = true; this.bfrErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/bidder-field-rules', 'PUT', rule);
        if (!res.ok) throw new Error(await res.text());
        var saved = await res.json();
        // Replace in local array
        var idx = this.bidderFieldRules.findIndex(function(r){ return r.id === saved.id; });
        if (idx >= 0) { this.bidderFieldRules.splice(idx, 1, saved); }
        else { this.bidderFieldRules.push(saved); }
        this.bfrNewRow = null;
        this.showToast('Rule saved', false);
      } catch(e) {
        this.bfrErr = e.message;
        this.showToast('Save failed', true);
      } finally { this.bfrSaving = false; }
    },
    bfrDelete: async function(rule) {
      if (!confirm('Delete rule for ' + rule.field_path + '?')) return;
      try {
        var res = await this.apiFetch(this.apiBase+'/bidder-field-rules/'+rule.id, 'DELETE');
        if (!res.ok) throw new Error(await res.text());
        this.bidderFieldRules = this.bidderFieldRules.filter(function(r){ return r.id !== rule.id; });
        this.showToast('Rule deleted', false);
      } catch(e) { this.showToast('Delete failed: '+e.message, true); }
    },
    bfrAddRow: function() {
      this.bfrNewRow = {
        bidder_code: this.bfrBidder, field_path: '', source_type: 'standard',
        source_ref: null, transform: 'none', required: false, enabled: true, notes: null
      };
    },
    bfrPreview: async function() {
      if (this.bfrBidder === '__default__') return;
      var url = this.apiBase+'/bid-request-preview?bidder='+encodeURIComponent(this.bfrBidder);
      window.open(url, '_blank');
    },
```

- [ ] **Step 5: Add the Bid Request tab HTML panel**

Find:
```html
      <!-- ── Tags Tab ─────────────────────────────────────────────────────── -->
```

Add before it:

```html
      <!-- ── Bid Request Tab ──────────────────────────────────────────────── -->
      <div v-show="section === 'bidrules'">
        <div class="card p-5 mb-4">
          <div class="flex flex-wrap items-end gap-3">
            <div class="field mb-0">
              <label>SSP / Bidder</label>
              <select v-model="bfrBidder" class="field-input w-48">
                <option v-for="c in bfrAllBidderCodes" :key="c" :value="c">{{ c }}</option>
              </select>
            </div>
            <button @click="bfrAddRow" class="btn-primary">+ Add Rule</button>
            <button v-if="bfrBidder !== '__default__'" @click="bfrPreview" class="btn-secondary">Preview JSON ↗</button>
            <span v-if="bfrErr" class="text-red-400 text-xs ml-auto">{{ bfrErr }}</span>
          </div>
        </div>

        <!-- New-row form -->
        <div v-if="bfrNewRow" class="card p-4 mb-4 border border-indigo-500/40">
          <div class="text-xs font-semibold text-indigo-400 mb-3">New Rule — {{ bfrBidder }}</div>
          <div class="grid grid-cols-2 gap-3 mb-3 md:grid-cols-4">
            <div class="field mb-0 col-span-2"><label>Field Path *</label>
              <input v-model="bfrNewRow.field_path" class="field-input font-mono" placeholder="user.buyeruid"></div>
            <div class="field mb-0"><label>Source Type *</label>
              <select v-model="bfrNewRow.source_type" class="field-input">
                <option>standard</option><option>sdk_param</option><option>http_context</option>
                <option>account_param</option><option>slot_param</option><option>eid</option><option>constant</option>
              </select></div>
            <div class="field mb-0"><label>Source Ref</label>
              <input v-model="bfrNewRow.source_ref" class="field-input font-mono" placeholder="kargo.com"></div>
            <div class="field mb-0"><label>Transform</label>
              <select v-model="bfrNewRow.transform" class="field-input">
                <option>none</option><option>to_int</option><option>to_string</option>
                <option>to_string_array</option><option>lowercase</option><option>sha256</option>
                <option>url_encode</option><option>json_stringify</option><option>array_first</option>
                <option>csv_to_array</option><option>wrap_ext_rp</option>
              </select></div>
            <div class="field mb-0 col-span-2"><label>Notes</label>
              <input v-model="bfrNewRow.notes" class="field-input" placeholder="optional"></div>
            <div class="field mb-0 flex items-center gap-2 pt-5">
              <input type="checkbox" v-model="bfrNewRow.required" id="bfr-req">
              <label for="bfr-req" class="text-xs cursor-pointer">Required</label>
            </div>
          </div>
          <div class="flex gap-2">
            <button @click="bfrSave(bfrNewRow)" :disabled="bfrSaving" class="btn-success text-xs">
              {{ bfrSaving ? 'Saving…' : 'Save Rule' }}
            </button>
            <button @click="bfrNewRow=null" class="btn-secondary text-xs">Cancel</button>
          </div>
        </div>

        <!-- Rules by section -->
        <div v-if="bfrSections.length === 0" class="text-gray-500 text-sm text-center py-12">
          No rules found. Select a bidder or add a rule.
        </div>
        <div v-for="sec in bfrSections" :key="sec.label" class="card mb-3 overflow-hidden">
          <div class="px-4 py-2 bg-white/5 border-b border-white/10 text-xs font-semibold text-gray-300">
            {{ sec.label }}
          </div>
          <table class="w-full text-xs">
            <thead class="border-b border-white/10">
              <tr class="text-left text-gray-400 font-medium">
                <th class="px-3 py-2">Field Path</th>
                <th class="px-3 py-2">Source</th>
                <th class="px-3 py-2">Ref / Value</th>
                <th class="px-3 py-2">Transform</th>
                <th class="px-3 py-2">Req</th>
                <th class="px-3 py-2">Notes</th>
                <th class="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="rule in sec.rules" :key="rule.id"
                  :class="['border-t border-white/5', rule._isOverride ? 'bg-indigo-900/20' : 'hover:bg-white/5']">
                <td class="px-3 py-2 font-mono text-gray-200 whitespace-nowrap">
                  <span v-if="rule._isOverride" class="mr-1 px-1 py-0.5 rounded text-indigo-300 bg-indigo-600/30 text-xs">override</span>
                  {{ rule.field_path }}
                </td>
                <td class="px-3 py-2 text-yellow-400 whitespace-nowrap">{{ rule.source_type }}</td>
                <td class="px-3 py-2 font-mono text-gray-400">{{ rule.source_ref || '—' }}</td>
                <td class="px-3 py-2 text-gray-400">{{ rule.transform !== 'none' ? rule.transform : '—' }}</td>
                <td class="px-3 py-2">
                  <span v-if="rule.required" class="text-red-400 font-bold">✓</span>
                  <span v-else class="text-gray-600">—</span>
                </td>
                <td class="px-3 py-2 text-gray-500 max-w-xs truncate" :title="rule.notes">{{ rule.notes || '' }}</td>
                <td class="px-3 py-2 text-right whitespace-nowrap">
                  <button v-if="rule.bidder_code !== '__default__'" @click="bfrDelete(rule)"
                    class="text-xs px-2 py-1 rounded bg-red-900/40 hover:bg-red-700/60 text-red-300 transition-colors">
                    Delete
                  </button>
                  <span v-else class="text-gray-600 text-xs">default</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

```

- [ ] **Step 6: Build and verify**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 7: Deploy and smoke-test Phase 1**

```bash
docker build -t catalyst-server:latest . && \
docker save catalyst-server:latest | ssh catalyst \
  "docker load && cd ~/catalyst && docker-compose up -d --no-deps catalyst"
```

Then visit `https://thenexusengine.com/catalyst/admin` → "Bid Request" tab → select `kargo` → confirm routing table shows `imp.ext.kargo.placementId ← slot_param` and `user.buyeruid ← eid / kargo.com` rows, with `__default__` rows greyed below.

- [ ] **Step 8: Commit**

```bash
git add internal/endpoints/onboarding_admin.go
git commit -m "feat(ui): add Bid Request tab — per-SSP OpenRTB field routing table with add/delete and preview button"
```

---

## ═══════════════ PHASE 2 ═══════════════

---

### Task 5: EID Resolver

**Files:**
- Create: `internal/adapters/routing/eid_resolver.go`
- Create: `internal/adapters/routing/eid_resolver_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/adapters/routing/eid_resolver_test.go`:

```go
package routing_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestResolveEID_FromUserEIDs(t *testing.T) {
	user := &openrtb.User{
		EIDs: []openrtb.EID{
			{Source: "kargo.com", UIDs: []openrtb.UID{{ID: "kargo-buyer-123"}}},
		},
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "kargo-buyer-123" {
		t.Errorf("got %q, want %q", got, "kargo-buyer-123")
	}
}

func TestResolveEID_FallbackToExtEIDs(t *testing.T) {
	extEIDsJSON := `[{"source":"kargo.com","uids":[{"id":"kargo-ext-456"}]}]`
	user := &openrtb.User{
		Ext: []byte(`{"eids":` + extEIDsJSON + `}`),
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "kargo-ext-456" {
		t.Errorf("got %q, want %q", got, "kargo-ext-456")
	}
}

func TestResolveEID_MissingSource(t *testing.T) {
	user := &openrtb.User{
		EIDs: []openrtb.EID{
			{Source: "other.com", UIDs: []openrtb.UID{{ID: "other-111"}}},
		},
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "" {
		t.Errorf("expected empty string for missing source, got %q", got)
	}
}

func TestResolveEID_NilUser(t *testing.T) {
	got := routing.ResolveEID(nil, "kargo.com")
	if got != "" {
		t.Errorf("expected empty string for nil user, got %q", got)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

```bash
go test ./internal/adapters/routing/... 2>&1 | head -10
```

Expected: `cannot find package` or `undefined: routing.ResolveEID`.

- [ ] **Step 3: Create the resolver**

Create `internal/adapters/routing/eid_resolver.go`:

```go
// Package routing provides the BidRequestComposer — a DB-driven engine
// that applies bidder_field_rules to build per-SSP OpenRTB requests.
package routing

import (
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// ResolveEID looks up the buyer UID for the given source domain.
// It first checks user.eids, then falls back to user.ext.eids (legacy).
// Returns "" if the user is nil or no matching EID is found.
func ResolveEID(user *openrtb.User, sourceDomain string) string {
	if user == nil {
		return ""
	}
	// 1. Walk user.eids
	for _, eid := range user.EIDs {
		if eid.Source == sourceDomain && len(eid.UIDs) > 0 {
			return eid.UIDs[0].ID
		}
	}
	// 2. Fall back to user.ext.eids (legacy location)
	if len(user.Ext) == 0 {
		return ""
	}
	var ext struct {
		EIDs []openrtb.EID `json:"eids"`
	}
	if err := json.Unmarshal(user.Ext, &ext); err != nil {
		return ""
	}
	for _, eid := range ext.EIDs {
		if eid.Source == sourceDomain && len(eid.UIDs) > 0 {
			return eid.UIDs[0].ID
		}
	}
	return ""
}
```

> **Note:** Adjust the `openrtb.EID` / `openrtb.UID` type paths to match what's in `internal/openrtb/`. Check `internal/adapters/userids.go` for the existing EID struct names.

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/adapters/routing/... -run TestResolveEID -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/routing/
git commit -m "feat(routing): add EIDResolver with user.eids → user.ext.eids fallback"
```

---

### Task 6: Rule Applier

**Files:**
- Create: `internal/adapters/routing/rule_applier.go`
- Create: `internal/adapters/routing/rule_applier_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/adapters/routing/rule_applier_test.go`:

```go
package routing_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

func strRef(s string) *string { return &s }

func TestApplyTransform_ToInt(t *testing.T) {
	got, err := routing.ApplyTransform("42", "to_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("expected int 42, got %v (%T)", got, got)
	}
}

func TestApplyTransform_ToString(t *testing.T) {
	got, err := routing.ApplyTransform(123, "to_string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "123" {
		t.Errorf("expected string \"123\", got %v (%T)", got, got)
	}
}

func TestApplyTransform_None(t *testing.T) {
	got, err := routing.ApplyTransform("hello", "none")
	if err != nil || got != "hello" {
		t.Errorf("none transform should be a pass-through, got %v err %v", got, err)
	}
}

func TestRuleApplier_SlotParam(t *testing.T) {
	rule := storage.BidderFieldRule{
		SourceType: "slot_param",
		SourceRef:  strRef("placementId"),
		Transform:  "none",
	}
	slotParams := map[string]interface{}{"placementId": "_o9n8eh8Lsw"}
	got, err := routing.ApplyRule(rule, nil, slotParams, nil, nil)
	if err != nil || got != "_o9n8eh8Lsw" {
		t.Errorf("expected placementId, got %v err %v", got, err)
	}
}

func TestRuleApplier_Constant(t *testing.T) {
	rule := storage.BidderFieldRule{
		SourceType: "constant",
		SourceRef:  strRef("1"),
		Transform:  "to_int",
	}
	got, err := routing.ApplyRule(rule, nil, nil, nil, nil)
	if err != nil || got != 1 {
		t.Errorf("expected constant int 1, got %v err %v", got, err)
	}
}
```

- [ ] **Step 2: Create the rule applier**

Create `internal/adapters/routing/rule_applier.go`:

```go
package routing

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// HTTPContext holds values extracted from the live HTTP request.
type HTTPContext struct {
	UserAgent      string
	IP             string
	AcceptLanguage string
	Headers        http.Header
}

// ApplyRule resolves the value for a single BidderFieldRule.
// Returns (value, nil) on success; (nil, err) if required and not resolvable.
// Returns (nil, nil) for optional rules where the source is absent.
func ApplyRule(
	rule storage.BidderFieldRule,
	user *openrtb.User,
	slotParams map[string]interface{},
	httpCtx *HTTPContext,
	accountParams map[string]interface{},
) (interface{}, error) {
	var raw interface{}

	switch rule.SourceType {
	case "standard":
		// Pass-through: caller handles this by copying the field from the incoming request.
		return nil, nil

	case "constant":
		raw = *rule.SourceRef

	case "slot_param":
		key := *rule.SourceRef
		val, ok := slotParams[key]
		if !ok {
			if rule.Required {
				return nil, fmt.Errorf("required slot_param %q not found", key)
			}
			return nil, nil
		}
		raw = val

	case "account_param":
		key := *rule.SourceRef
		val, ok := accountParams[key]
		if !ok {
			if rule.Required {
				return nil, fmt.Errorf("required account_param %q not found", key)
			}
			return nil, nil
		}
		raw = val

	case "http_context":
		if httpCtx == nil {
			return nil, nil
		}
		header := *rule.SourceRef
		raw = httpCtx.Headers.Get(header)
		if raw == "" {
			return nil, nil
		}

	case "eid":
		if user == nil {
			return nil, nil
		}
		uid := ResolveEID(user, *rule.SourceRef)
		if uid == "" {
			return nil, nil
		}
		raw = uid

	default:
		return nil, fmt.Errorf("unknown source_type %q", rule.SourceType)
	}

	return ApplyTransform(raw, rule.Transform)
}

// ApplyTransform applies a named transform to a value.
func ApplyTransform(value interface{}, transform string) (interface{}, error) {
	switch transform {
	case "none", "":
		return value, nil

	case "to_int":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("to_int: cannot parse %q: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("to_int: unsupported type %T", value)
		}

	case "to_string":
		return fmt.Sprintf("%v", value), nil

	case "to_string_array":
		return []string{fmt.Sprintf("%v", value)}, nil

	case "lowercase":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("lowercase: expected string, got %T", value)
		}
		return strings.ToLower(s), nil

	case "url_encode":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("url_encode: expected string, got %T", value)
		}
		// Basic URL encoding without net/url to avoid import bloat
		return strings.NewReplacer(" ", "+", "&", "%26", "=", "%3D").Replace(s), nil

	case "array_first":
		switch v := value.(type) {
		case []interface{}:
			if len(v) == 0 {
				return nil, nil
			}
			return v[0], nil
		case []string:
			if len(v) == 0 {
				return nil, nil
			}
			return v[0], nil
		default:
			return value, nil // already scalar
		}

	case "csv_to_array":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("csv_to_array: expected string, got %T", value)
		}
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out, nil

	case "sha256":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("sha256: expected string, got %T", value)
		}
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:]), nil

	case "base64_decode":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("base64_decode: expected string, got %T", value)
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64_decode: %w", err)
		}
		return string(b), nil

	case "wrap_ext_rp":
		// Rubicon-specific: wrap value in {"rp": value}
		return map[string]interface{}{"rp": value}, nil

	default:
		// Unknown transforms are treated as no-op to avoid breaking live requests.
		return value, nil
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/adapters/routing/... -v
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/adapters/routing/rule_applier.go internal/adapters/routing/rule_applier_test.go
git commit -m "feat(routing): add RuleApplier with ApplyRule + ApplyTransform (to_int, to_string, csv_to_array, etc.)"
```

---

### Task 7: Loader + Composer

**Files:**
- Create: `internal/adapters/routing/loader.go`
- Create: `internal/adapters/routing/composer.go`
- Create: `internal/adapters/routing/composer_test.go`

- [ ] **Step 1: Write composer tests**

Create `internal/adapters/routing/composer_test.go`:

```go
package routing_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

func TestComposer_SetsSlotParam(t *testing.T) {
	rules := []storage.BidderFieldRule{
		{
			BidderCode: "kargo",
			FieldPath:  "imp.ext.kargo.placementId",
			SourceType: "slot_param",
			SourceRef:  strRef("placementId"),
			Transform:  "none",
			Required:   true,
		},
	}
	c := routing.NewComposer(rules)
	req := &openrtb.BidRequest{
		Imp: []openrtb.Imp{{ID: "1"}},
	}
	slotParams := map[string]interface{}{"placementId": "_abc123"}
	result, errs := c.Apply("kargo", req, slotParams, nil, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result == nil || len(result.Imp) == 0 {
		t.Fatal("expected imp in result")
	}
	// imp.ext should contain kargo.placementId
	if result.Imp[0].Ext == nil {
		t.Fatal("expected imp[0].Ext to be set")
	}
}

func TestComposer_EIDtoBuyerUID(t *testing.T) {
	rules := []storage.BidderFieldRule{
		{
			BidderCode: "kargo",
			FieldPath:  "user.buyeruid",
			SourceType: "eid",
			SourceRef:  strRef("kargo.com"),
			Transform:  "none",
		},
	}
	c := routing.NewComposer(rules)
	req := &openrtb.BidRequest{
		User: &openrtb.User{
			EIDs: []openrtb.EID{
				{Source: "kargo.com", UIDs: []openrtb.UID{{ID: "k-buyer-xyz"}}},
			},
		},
	}
	result, errs := c.Apply("kargo", req, nil, nil, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result.User == nil || result.User.BuyerUID != "k-buyer-xyz" {
		t.Errorf("expected BuyerUID = k-buyer-xyz, got %v", result.User)
	}
}
```

- [ ] **Step 2: Create the loader**

Create `internal/adapters/routing/loader.go`:

```go
package routing

import (
	"context"
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const cacheTTL = 30 * time.Second

// Loader caches bidder_field_rules from the DB.
// Rules are refreshed on TTL expiry or explicit Invalidate call.
type Loader struct {
	store     *storage.PublisherStore
	mu        sync.RWMutex
	cache     map[string][]storage.BidderFieldRule // bidder_code → merged rules
	fetchedAt time.Time
}

// NewLoader creates a Loader backed by the given store.
func NewLoader(store *storage.PublisherStore) *Loader {
	return &Loader{store: store, cache: make(map[string][]storage.BidderFieldRule)}
}

// Get returns the merged rules for bidderCode (bidder-specific + __default__).
// Refreshes the cache if stale.
func (l *Loader) Get(ctx context.Context, bidderCode string) []storage.BidderFieldRule {
	l.mu.RLock()
	stale := time.Since(l.fetchedAt) > cacheTTL
	l.mu.RUnlock()

	if stale {
		l.refresh(ctx)
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cache[bidderCode]
}

// Invalidate clears the cache so the next Get re-fetches from DB.
func (l *Loader) Invalidate(_ string) {
	l.mu.Lock()
	l.fetchedAt = time.Time{}
	l.mu.Unlock()
}

func (l *Loader) refresh(ctx context.Context) {
	all, err := l.store.GetAllBidderFieldRules(ctx)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("routing.Loader: failed to refresh rules")
		return
	}

	// Group by bidder_code
	byBidder := make(map[string][]storage.BidderFieldRule)
	defaults := make([]storage.BidderFieldRule, 0)
	for _, r := range all {
		if r.BidderCode == "__default__" {
			defaults = append(defaults, r)
		} else {
			byBidder[r.BidderCode] = append(byBidder[r.BidderCode], r)
		}
	}

	// Merge: bidder-specific wins over defaults for same field_path
	merged := make(map[string][]storage.BidderFieldRule)
	for code, bidderRules := range byBidder {
		seen := make(map[string]bool)
		var m []storage.BidderFieldRule
		for _, r := range bidderRules {
			seen[r.FieldPath] = true
			m = append(m, r)
		}
		for _, d := range defaults {
			if !seen[d.FieldPath] {
				m = append(m, d)
			}
		}
		merged[code] = m
	}
	merged["__default__"] = defaults

	l.mu.Lock()
	l.cache = merged
	l.fetchedAt = time.Now()
	l.mu.Unlock()
}
```

- [ ] **Step 3: Create the composer**

Create `internal/adapters/routing/composer.go`:

```go
package routing

import (
	"encoding/json"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// Composer applies bidder_field_rules to a copy of the incoming BidRequest,
// producing the outgoing request for a specific SSP.
type Composer struct {
	rules []storage.BidderFieldRule
}

// NewComposer creates a Composer for the given pre-loaded rules.
func NewComposer(rules []storage.BidderFieldRule) *Composer {
	return &Composer{rules: rules}
}

// Apply returns a modified copy of req with all rules applied.
// slotParams: merged slot_bidder_configs + account_bidder_defaults for this slot.
// httpCtx: live HTTP request context (UA, IP, etc.).
// accountParams: publisher DB record fields.
func (c *Composer) Apply(
	bidderCode string,
	req *openrtb.BidRequest,
	slotParams map[string]interface{},
	httpCtx *HTTPContext,
	accountParams map[string]interface{},
) (*openrtb.BidRequest, []error) {
	// Shallow-copy the request; deep-copy Imp to avoid mutating caller.
	out := *req
	if req.Imp != nil {
		out.Imp = make([]openrtb.Imp, len(req.Imp))
		copy(out.Imp, req.Imp)
	}

	// Build a mutable ext map for imp[0] (most adapters work per-imp; we process imp[0] here
	// and rely on the adapter loop for multi-imp requests).
	impExt := make(map[string]interface{})
	if len(out.Imp) > 0 && out.Imp[0].Ext != nil {
		_ = json.Unmarshal(out.Imp[0].Ext, &impExt)
	}

	var errs []error

	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}
		val, err := ApplyRule(rule, req.User, slotParams, httpCtx, accountParams)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if val == nil {
			continue // optional, not resolved
		}

		// Write to the correct location
		if err := setField(&out, &impExt, rule.FieldPath, val); err != nil {
			logger.Log.Warn().Err(err).Str("field", rule.FieldPath).Msg("composer: setField failed")
		}
	}

	// Re-marshal impExt back to imp[0].Ext
	if len(out.Imp) > 0 && len(impExt) > 0 {
		b, _ := json.Marshal(impExt)
		out.Imp[0].Ext = b
	}

	return &out, errs
}

// setField writes val to the dot-path within the request or impExt map.
//
// Scope note: setField only handles fields that come from NON-STANDARD sources
// (sdk_param, http_context, account_param, eid, constant). Rules with
// source_type = "standard" return nil from ApplyRule and never reach setField —
// standard fields are already present in the incoming request and just pass through.
//
// To add support for more fields, add a case following the existing pattern.
func setField(req *openrtb.BidRequest, impExt *map[string]interface{}, path string, val interface{}) error {
	// imp.ext.* → write into impExt map using the remainder as nested key path
	if strings.HasPrefix(path, "imp.ext.") {
		key := strings.TrimPrefix(path, "imp.ext.")
		setNestedMap(*impExt, strings.Split(key, "."), val)
		return nil
	}

	// Deep-copy user/device/site before mutating to avoid sharing with caller.
	switch path {
	case "user.buyeruid":
		if req.User == nil {
			req.User = &openrtb.User{}
		} else {
			u := *req.User
			req.User = &u
		}
		if s, ok := val.(string); ok {
			req.User.BuyerUID = s
		}
	case "device.ua":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.UA = s
		}
	case "device.ip":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.IP = s
		}
	case "device.language":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.Language = s
		}
	case "site.page":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Page = s
		}
	case "site.domain":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Domain = s
		}
	case "site.publisher.id":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Publisher = &openrtb.Publisher{ID: s}
		}
	case "tmax":
		if i, ok := val.(int); ok {
			req.TMax = int64(i)
		}
	// Add more cases here following the same copy-on-write pattern.
	// Standard pass-through fields (regs.gdpr, source.schain, user.eids, etc.)
	// never reach this function — they have source_type="standard" and return
	// nil from ApplyRule, so the Composer preserves the original values unchanged.
	default:
		logger.Log.Debug().Str("path", path).Msg("composer: unhandled non-standard field path — add a case to setField to support it")
	}
	return nil
}

// setNestedMap sets a value at a nested key path in a map[string]interface{}.
func setNestedMap(m map[string]interface{}, keys []string, val interface{}) {
	if len(keys) == 1 {
		m[keys[0]] = val
		return
	}
	sub, ok := m[keys[0]].(map[string]interface{})
	if !ok {
		sub = make(map[string]interface{})
		m[keys[0]] = sub
	}
	setNestedMap(sub, keys[1:], val)
}
```

- [ ] **Step 4: Run all routing tests**

```bash
go test ./internal/adapters/routing/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Build check**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/routing/
git commit -m "feat(routing): add Loader (TTL cache) + Composer (rule engine) — applies bidder_field_rules to build per-SSP OpenRTB requests"
```

---

### Task 8: Migrate Kargo Adapter

**Files:**
- Modify: `internal/adapters/kargo/kargo.go`

The Kargo adapter currently:
1. Validates `imp.ext.kargo.placementId`
2. Rewrites `imp.ext` to `{"bidder":{"placementId":"..."}}`
3. Resolves `user.buyeruid` from EIDs

After this task, steps 1-3 are handled by the Composer. The adapter only does the HTTP request/response.

- [ ] **Step 1: Check existing kargo tests**

```bash
ls internal/adapters/kargo/
go test ./internal/adapters/kargo/... -v 2>&1 | tail -20
```

Note the existing test names so you can confirm they still pass after the refactor.

- [ ] **Step 2: Wire Composer into the Kargo adapter**

The `Adapter` struct needs access to the `Loader`. Add it:

```go
type Adapter struct {
	endpoint string
	loader   *routing.Loader  // nil = Composer disabled (Phase 1 compat)
}

func New(endpoint string, loader *routing.Loader) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint, loader: loader}
}
```

In `MakeRequests`, after copying the request, add at the top:
```go
	if a.loader != nil {
		rules := a.loader.Get(r.Context(), "kargo")
		composer := routing.NewComposer(rules)
		composed, _ := composer.Apply("kargo", &requestCopy, extractSlotParams(requestCopy.Imp), nil, nil)
		requestCopy = *composed
	}
```

Also add this helper at the bottom of `kargo.go` (copy the same function to each adapter, changing the bidder key):

```go
// extractSlotParams reads imp[0].ext.{bidderKey} (or imp[0].ext.bidder after PBS translation)
// into a flat map suitable for the Composer's slotParams argument.
func extractSlotParams(imps []openrtb.Imp, bidderKey string) map[string]interface{} {
	if len(imps) == 0 || imps[0].Ext == nil {
		return nil
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(imps[0].Ext, &outer); err != nil {
		return nil
	}
	// PBS translates imp.ext.{bidder} → imp.ext.bidder; check both.
	raw, ok := outer[bidderKey]
	if !ok {
		raw, ok = outer["bidder"]
		if !ok {
			return nil
		}
	}
	var params map[string]interface{}
	json.Unmarshal(raw, &params) //nolint:errcheck
	return params
}
```

Call it as: `extractSlotParams(requestCopy.Imp, "kargo")` (use adapter's own bidder key for each adapter).

- [ ] **Step 3: Update registry call to pass loader**

In `internal/adapters/kargo/kargo.go`'s `init()` (or wherever `registry.Register` is called), pass `nil` for now — the loader gets wired when the exchange initialises it in Task 10.

- [ ] **Step 4: Run kargo tests**

```bash
go test ./internal/adapters/kargo/... -v
```

Expected: all existing tests still PASS (Composer with nil loader is a no-op).

- [ ] **Step 5: Build check**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/kargo/
git commit -m "feat(kargo): wire BidRequestComposer — EID resolution + field routing now DB-driven when loader provided"
```

---

### Task 9: Migrate Sovrn, Pubmatic, TrippleLift, AppNexus

Apply the same pattern as Task 8 to each remaining simple adapter. They all follow the same structure: `Adapter` gets a `loader *routing.Loader` field, `MakeRequests` calls `Composer.Apply` if loader is non-nil.

- [ ] **Sovrn** — `internal/adapters/sovrn/sovrn.go`
- [ ] **Pubmatic** — `internal/adapters/pubmatic/pubmatic.go`
- [ ] **TrippleLift** — `internal/adapters/triplelift/triplelift.go`
- [ ] **AppNexus** — `internal/adapters/appnexus/appnexus.go`

For each:
1. Add `loader *routing.Loader` to struct and constructor
2. Add Composer call in `MakeRequests`
3. Run adapter-specific tests: `go test ./internal/adapters/{name}/... -v`
4. Commit per-adapter: `feat({name}): wire BidRequestComposer`

---

### Task 10: Wire Loader into the Exchange / Server

**Files:**
- Modify: `cmd/server/server.go`
- Modify: `internal/adapters/registry.go` (if needed)

- [ ] **Step 1: Create the loader at server startup**

In `cmd/server/server.go`, after `s.publisher = storage.NewPublisherStore(dbConn)`, add:

```go
	routingLoader := routing.NewLoader(s.publisher)
```

- [ ] **Step 2: Pass loader to each adapter via package-level setter**

Adapters register themselves in `init()` and the registry holds the single instance. The cleanest wiring is a package-level `var defaultLoader *routing.Loader` in each adapter package, set after server startup.

Add to each adapter package (e.g. `kargo/kargo.go`):
```go
// defaultLoader is set by the server after startup via SetLoader.
// nil = Composer disabled (Phase 1 behaviour preserved).
var defaultLoader *routing.Loader

// SetLoader injects the routing Loader into the registered Kargo adapter instance.
// Call once from cmd/server/server.go after creating the Loader.
func SetLoader(l *routing.Loader) { defaultLoader = l }
```

In `MakeRequests`, replace `a.loader` with the package-level `defaultLoader`:
```go
	if defaultLoader != nil {
		rules := defaultLoader.Get(r.Context(), "kargo")
		composer := routing.NewComposer(rules)
		composed, _ := composer.Apply("kargo", &requestCopy, extractSlotParams(requestCopy.Imp, "kargo"), nil, nil)
		requestCopy = *composed
	}
```

Then in `cmd/server/server.go`, after `routingLoader := routing.NewLoader(s.publisher)`:
```go
	kargo.SetLoader(routingLoader)
	sovrn.SetLoader(routingLoader)
	pubmatic.SetLoader(routingLoader)
	triplelift.SetLoader(routingLoader)
	appnexus.SetLoader(routingLoader)
	rubicon.SetLoader(routingLoader)
```

- [ ] **Step 3: Wire Loader invalidation on rule save**

In `onboarding_admin.go`, `upsertBidderFieldRule`, after saving:

```go
	if h.routingLoader != nil {
		h.routingLoader.Invalidate(rule.BidderCode)
	}
```

Add `routingLoader *routing.Loader` to `OnboardingAdminHandler` and pass it from `server.go` via a new constructor param or setter.

- [ ] **Step 4: Integration smoke test**

```bash
go build ./...
docker build -t catalyst-server:latest . && \
docker save catalyst-server:latest | ssh catalyst \
  "docker load && cd ~/catalyst && docker-compose up -d --no-deps catalyst"
```

Fire a test auction for a Kargo slot:
```bash
curl -s "https://ads.thenexusengine.com/openrtb2/auction" \
  -H "Content-Type: application/json" \
  -d '{"id":"test","imp":[{"id":"1","banner":{"w":728,"h":90},"ext":{"kargo":{"placementId":"_o9n8eh8Lsw"}}}],"site":{"page":"https://test.com","domain":"test.com"},"tmax":2000}' \
  | python3 -m json.tool | head -40
```

Expected: valid OpenRTB response with at least one `seatbid`.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/server.go internal/adapters/routing/ internal/endpoints/onboarding_admin.go
git commit -m "feat(exchange): wire BidRequestComposer into server — routing loader initialised at startup, invalidated on rule save"
```

---

### Task 11: Partial Rubicon Migration

**Files:**
- Modify: `internal/adapters/rubicon/rubicon.go`

Rubicon keeps: `ext.rp` nesting, Basic auth header, PBS identity target, `size_id` logic.
Rubicon delegates to Composer: standard fields (`device.ua`, `site.page`, etc.), EID → `user.buyeruid`.

- [ ] **Step 1: Wire Composer for non-rp fields only**

Add `loader` to Rubicon adapter. In `MakeRequests`, before the existing `ext.rp` logic, call:

```go
	if a.loader != nil {
		rules := a.loader.Get(r.Context(), "rubicon")
		// Filter: only apply rules that don't conflict with rubicon's Go logic
		safeRules := filterNonRPRules(rules)
		composer := routing.NewComposer(safeRules)
		composed, _ := composer.Apply("rubicon", &requestCopy, slotParams, nil, nil)
		requestCopy = *composed
	}
```

`filterNonRPRules` excludes rules with `field_path` starting with `imp.ext.rubicon.` (those are handled by the existing `ext.rp` nesting code).

- [ ] **Step 2: Run rubicon tests**

```bash
go test ./internal/adapters/rubicon/... -v
```

Expected: PASS.

- [ ] **Step 3: Build + deploy + integration test**

```bash
go build ./... && \
docker build -t catalyst-server:latest . && \
docker save catalyst-server:latest | ssh catalyst \
  "docker load && cd ~/catalyst && docker-compose up -d --no-deps catalyst"
```

Fire a Rubicon auction and confirm `imp.ext.rp` still present in outgoing request (check server logs).

- [ ] **Step 4: Commit**

```bash
git add internal/adapters/rubicon/
git commit -m "feat(rubicon): delegate standard-field routing to Composer; retain ext.rp nesting + auth in Go"
```

---

## Verification Checklist

### Phase 1
- [ ] Admin → "Bid Request" tab → select `kargo` → routing table shows `imp.ext.kargo.placementId`, `user.buyeruid`, plus greyed `__default__` rows
- [ ] Add a new rule (`site.ref ← sdk_param / referer`) → save → appears in table → page reload persists
- [ ] Delete the new rule → disappears
- [ ] Preview button for `kargo` → new tab shows active rules JSON
- [ ] `__default__` rows visible in every SSP's view, shown as greyed baseline

### Phase 2
- [ ] Real Kargo auction fires → `imp.ext.kargo.placementId` correct → `user.buyeruid` populated from EID
- [ ] Real Rubicon auction fires → `ext.rp` nesting present → `user.buyeruid` from `rubiconproject.com` EID
- [ ] Change Kargo EID domain in UI → next auction uses new domain (no restart)
- [ ] All `go test ./...` pass
- [ ] `go build ./...` clean
