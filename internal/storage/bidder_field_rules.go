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
	ID         int       `json:"id"`
	BidderID   *int      `json:"bidder_id"` // nil for __default__
	BidderCode string    `json:"bidder_code"`
	FieldPath  string    `json:"field_path"`
	SourceType string    `json:"source_type"`
	SourceRef  *string   `json:"source_ref"`
	Transform  string    `json:"transform"`
	Required   bool      `json:"required"`
	Enabled    bool      `json:"enabled"`
	Notes      *string   `json:"notes"`
	UpdatedAt  time.Time `json:"updated_at"`
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
		return nil, fmt.Errorf("LookupBidderIDByCode: %w", err)
	}
	return &id, nil
}
