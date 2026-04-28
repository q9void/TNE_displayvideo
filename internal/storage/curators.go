// Package storage — curator catalog access for curated deals.
//
// The curator catalog (migration 020) lets the auction layer attribute
// imp.pmp.deals[] to a third-party curator, hydrate missing deal metadata,
// and route fanout to wseat-permitted bidders. This file is the read/write
// surface used by exchange, routing, and admin endpoints.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Curator is a third-party deal/data packager registered with TNE Catalyst.
type Curator struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	SChainASI  string         `json:"schain_asi"`
	SChainSID  string         `json:"schain_sid"`
	Notes      sql.NullString `json:"notes,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// CuratorDeal is one row from curator_deals — the catalog overlay applied to
// matching imp.pmp.deals[] entries during auction validation.
type CuratorDeal struct {
	DealID         string          `json:"deal_id"`
	CuratorID      string          `json:"curator_id"`
	BidFloor       sql.NullFloat64 `json:"bidfloor,omitempty"`
	BidFloorCur    string          `json:"bidfloorcur"`
	AT             sql.NullInt64   `json:"at,omitempty"`
	WSeat          []string        `json:"wseat"`
	WAdomain       []string        `json:"wadomain"`
	SegtaxAllowed  []int64         `json:"segtax_allowed"`
	CattaxAllowed  []int64         `json:"cattax_allowed"`
	Ext            json.RawMessage `json:"ext,omitempty"`
	Active         bool            `json:"active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CuratorSeat is one (curator, bidder_code, seat_id) binding.
type CuratorSeat struct {
	CuratorID  string `json:"curator_id"`
	BidderCode string `json:"bidder_code"`
	SeatID     string `json:"seat_id"`
}

// CuratorStore provides database operations for the curator catalog.
type CuratorStore struct {
	db *sql.DB
}

// NewCuratorStore returns a CuratorStore backed by the given database handle.
func NewCuratorStore(db *sql.DB) *CuratorStore {
	return &CuratorStore{db: db}
}

// Ping checks connectivity. Returns nil if no DB is configured (caller decides).
func (s *CuratorStore) Ping(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}

// ----------------------------------------------------------------------------
// Curators
// ----------------------------------------------------------------------------

// LoadCurator fetches a single curator by ID. Returns (nil, nil) if not found.
func (s *CuratorStore) LoadCurator(ctx context.Context, id string) (*Curator, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT id, name, status, schain_asi, schain_sid, notes, created_at, updated_at
		FROM curators
		WHERE id = $1
	`
	var c Curator
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.Name, &c.Status, &c.SChainASI, &c.SChainSID,
		&c.Notes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load curator %s: %w", id, err)
	}
	return &c, nil
}

// ListCurators returns every curator (active and inactive) ordered by ID.
// Status filtering is left to the caller — admin lists need archived rows.
func (s *CuratorStore) ListCurators(ctx context.Context) ([]*Curator, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT id, name, status, schain_asi, schain_sid, notes, created_at, updated_at
		FROM curators
		ORDER BY id
	`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to list curators: %w", err)
	}
	defer rows.Close()

	out := make([]*Curator, 0, 32)
	for rows.Next() {
		var c Curator
		if err := rows.Scan(&c.ID, &c.Name, &c.Status, &c.SChainASI, &c.SChainSID,
			&c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan curator: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// UpsertCurator creates or updates a curator. Status defaults to 'active'.
func (s *CuratorStore) UpsertCurator(ctx context.Context, c *Curator) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	if c.Status == "" {
		c.Status = "active"
	}
	const q = `
		INSERT INTO curators (id, name, status, schain_asi, schain_sid, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			schain_asi = EXCLUDED.schain_asi,
			schain_sid = EXCLUDED.schain_sid,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`
	if err := s.db.QueryRowContext(ctx, q,
		c.ID, c.Name, c.Status, c.SChainASI, c.SChainSID, c.Notes,
	).Scan(&c.CreatedAt, &c.UpdatedAt); err != nil {
		return fmt.Errorf("failed to upsert curator %s: %w", c.ID, err)
	}
	return nil
}

// DeleteCurator soft-deletes a curator by setting status='archived'.
// Cascade on curator_deals/seats/allowlist is handled by the FK constraints
// only on hard delete; soft-delete preserves history for analytics joins.
func (s *CuratorStore) DeleteCurator(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	res, err := s.db.ExecContext(ctx,
		`UPDATE curators SET status='archived', updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("failed to archive curator %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("curator not found: %s", id)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Deals
// ----------------------------------------------------------------------------

// LookupDeal returns the catalog row for a given deal_id. (nil, nil) if absent.
// Inactive deals or deals owned by non-active curators are returned as nil so
// callers don't accidentally hydrate retired curated deals.
func (s *CuratorStore) LookupDeal(ctx context.Context, dealID string) (*CuratorDeal, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT d.deal_id, d.curator_id, d.bidfloor, d.bidfloorcur, d.at,
		       COALESCE(d.wseat, '{}'), COALESCE(d.wadomain, '{}'),
		       COALESCE(d.segtax_allowed, '{}'), COALESCE(d.cattax_allowed, '{}'),
		       COALESCE(d.ext, 'null'::jsonb),
		       d.active, d.created_at, d.updated_at
		FROM curator_deals d
		JOIN curators c ON c.id = d.curator_id
		WHERE d.deal_id = $1
		  AND d.active = TRUE
		  AND c.status = 'active'
	`
	var d CuratorDeal
	err := s.db.QueryRowContext(ctx, q, dealID).Scan(
		&d.DealID, &d.CuratorID, &d.BidFloor, &d.BidFloorCur, &d.AT,
		pq.Array(&d.WSeat), pq.Array(&d.WAdomain),
		pq.Array(&d.SegtaxAllowed), pq.Array(&d.CattaxAllowed),
		&d.Ext, &d.Active, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lookup deal %s: %w", dealID, err)
	}
	return &d, nil
}

// ListDeals returns every deal owned by a curator (active and inactive).
func (s *CuratorStore) ListDeals(ctx context.Context, curatorID string) ([]*CuratorDeal, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT deal_id, curator_id, bidfloor, bidfloorcur, at,
		       COALESCE(wseat, '{}'), COALESCE(wadomain, '{}'),
		       COALESCE(segtax_allowed, '{}'), COALESCE(cattax_allowed, '{}'),
		       COALESCE(ext, 'null'::jsonb),
		       active, created_at, updated_at
		FROM curator_deals
		WHERE curator_id = $1
		ORDER BY deal_id
	`
	rows, err := s.db.QueryContext(ctx, q, curatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to list deals for curator %s: %w", curatorID, err)
	}
	defer rows.Close()

	out := make([]*CuratorDeal, 0, 16)
	for rows.Next() {
		var d CuratorDeal
		if err := rows.Scan(&d.DealID, &d.CuratorID, &d.BidFloor, &d.BidFloorCur, &d.AT,
			pq.Array(&d.WSeat), pq.Array(&d.WAdomain),
			pq.Array(&d.SegtaxAllowed), pq.Array(&d.CattaxAllowed),
			&d.Ext, &d.Active, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan deal: %w", err)
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// UpsertDeal creates or updates a deal. The caller owns referential integrity
// (curator must exist). Empty BidFloorCur defaults to USD.
func (s *CuratorStore) UpsertDeal(ctx context.Context, d *CuratorDeal) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	if d.BidFloorCur == "" {
		d.BidFloorCur = "USD"
	}
	ext := d.Ext
	if len(ext) == 0 {
		ext = json.RawMessage("null")
	}
	const q = `
		INSERT INTO curator_deals (
			deal_id, curator_id, bidfloor, bidfloorcur, at,
			wseat, wadomain, segtax_allowed, cattax_allowed, ext, active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (deal_id) DO UPDATE SET
			curator_id = EXCLUDED.curator_id,
			bidfloor = EXCLUDED.bidfloor,
			bidfloorcur = EXCLUDED.bidfloorcur,
			at = EXCLUDED.at,
			wseat = EXCLUDED.wseat,
			wadomain = EXCLUDED.wadomain,
			segtax_allowed = EXCLUDED.segtax_allowed,
			cattax_allowed = EXCLUDED.cattax_allowed,
			ext = EXCLUDED.ext,
			active = EXCLUDED.active,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, q,
		d.DealID, d.CuratorID, d.BidFloor, d.BidFloorCur, d.AT,
		pq.Array(d.WSeat), pq.Array(d.WAdomain),
		pq.Array(d.SegtaxAllowed), pq.Array(d.CattaxAllowed),
		ext, d.Active,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
}

// DealIsCuratedBy reports whether dealID exists in curator_deals owned by
// curatorID and is active. Returns (false, nil) when the deal is unknown,
// inactive, or owned by a different curator. Used by the agentic applier
// to validate ACTIVATE_DEALS payloads from curator-bound agents.
func (s *CuratorStore) DealIsCuratedBy(ctx context.Context, dealID, curatorID string) (bool, error) {
	if s.db == nil || dealID == "" || curatorID == "" {
		return false, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM curator_deals
		 WHERE deal_id = $1 AND curator_id = $2 AND active = TRUE`,
		dealID, curatorID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("DealIsCuratedBy(%s,%s): %w", dealID, curatorID, err)
	}
	return n > 0, nil
}

// DeleteDeal removes a deal row. Hard delete is fine — the deal_id is a
// natural key; analytics tables retain a copy in win_events/bidder_events.
func (s *CuratorStore) DeleteDeal(ctx context.Context, dealID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `DELETE FROM curator_deals WHERE deal_id = $1`, dealID)
	if err != nil {
		return fmt.Errorf("failed to delete deal %s: %w", dealID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deal not found: %s", dealID)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Seats
// ----------------------------------------------------------------------------

// SeatsForCurator returns the seat IDs registered for (curator, bidder_code).
// Empty result means the curator has no buyer seat at that bidder — fanout
// filtering must therefore exclude it for that curator's deals.
func (s *CuratorStore) SeatsForCurator(ctx context.Context, curatorID, bidderCode string) ([]string, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT seat_id FROM curator_seats
		WHERE curator_id = $1 AND bidder_code = $2
		ORDER BY seat_id
	`
	rows, err := s.db.QueryContext(ctx, q, curatorID, bidderCode)
	if err != nil {
		return nil, fmt.Errorf("failed to load seats: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 4)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListSeats returns all (bidder_code, seat_id) pairs for a curator.
func (s *CuratorStore) ListSeats(ctx context.Context, curatorID string) ([]*CuratorSeat, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		`SELECT curator_id, bidder_code, seat_id FROM curator_seats
		 WHERE curator_id = $1 ORDER BY bidder_code, seat_id`, curatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to list seats: %w", err)
	}
	defer rows.Close()

	out := make([]*CuratorSeat, 0, 8)
	for rows.Next() {
		var cs CuratorSeat
		if err := rows.Scan(&cs.CuratorID, &cs.BidderCode, &cs.SeatID); err != nil {
			return nil, err
		}
		out = append(out, &cs)
	}
	return out, rows.Err()
}

// UpsertSeat inserts a seat binding (idempotent on conflict).
func (s *CuratorStore) UpsertSeat(ctx context.Context, cs *CuratorSeat) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO curator_seats (curator_id, bidder_code, seat_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (curator_id, bidder_code, seat_id) DO NOTHING`,
		cs.CuratorID, cs.BidderCode, cs.SeatID)
	if err != nil {
		return fmt.Errorf("failed to upsert seat: %w", err)
	}
	return nil
}

// DeleteSeat removes a single seat binding.
func (s *CuratorStore) DeleteSeat(ctx context.Context, curatorID, bidderCode, seatID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM curator_seats WHERE curator_id=$1 AND bidder_code=$2 AND seat_id=$3`,
		curatorID, bidderCode, seatID)
	if err != nil {
		return fmt.Errorf("failed to delete seat: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Publisher allow-list
// ----------------------------------------------------------------------------

// PublisherAllowedForCurator returns true if the (publisher_id, curator_id) pair
// is present in curator_publisher_allowlist. An EMPTY allow-list for a curator
// (no rows at all) is treated as "any publisher allowed" so a brand-new curator
// can be onboarded and tested before allow-listing is configured. Once any row
// exists for a curator, allow-listing is strictly enforced.
func (s *CuratorStore) PublisherAllowedForCurator(ctx context.Context, publisherID int, curatorID string) (bool, error) {
	if s.db == nil {
		return true, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	const q = `
		SELECT
			(SELECT COUNT(*) FROM curator_publisher_allowlist WHERE curator_id = $1) AS total,
			(SELECT COUNT(*) FROM curator_publisher_allowlist WHERE curator_id = $1 AND publisher_id = $2) AS match
	`
	var total, match int
	if err := s.db.QueryRowContext(ctx, q, curatorID, publisherID).Scan(&total, &match); err != nil {
		return false, fmt.Errorf("failed to check publisher allow-list: %w", err)
	}
	if total == 0 {
		return true, nil
	}
	return match > 0, nil
}

// ListAllowedPublishers returns publisher_id values allow-listed for a curator.
func (s *CuratorStore) ListAllowedPublishers(ctx context.Context, curatorID string) ([]int, error) {
	if s.db == nil {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		`SELECT publisher_id FROM curator_publisher_allowlist WHERE curator_id = $1 ORDER BY publisher_id`,
		curatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to list allow-listed publishers: %w", err)
	}
	defer rows.Close()

	out := make([]int, 0, 8)
	for rows.Next() {
		var pid int
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		out = append(out, pid)
	}
	return out, rows.Err()
}

// AllowPublisher inserts an allow-list row (idempotent on conflict).
func (s *CuratorStore) AllowPublisher(ctx context.Context, curatorID string, publisherID int) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO curator_publisher_allowlist (curator_id, publisher_id)
		 VALUES ($1, $2)
		 ON CONFLICT (curator_id, publisher_id) DO NOTHING`,
		curatorID, publisherID)
	if err != nil {
		return fmt.Errorf("failed to allow publisher: %w", err)
	}
	return nil
}

// DenyPublisher removes a single allow-list row.
func (s *CuratorStore) DenyPublisher(ctx context.Context, curatorID string, publisherID int) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM curator_publisher_allowlist WHERE curator_id=$1 AND publisher_id=$2`,
		curatorID, publisherID)
	if err != nil {
		return fmt.Errorf("failed to deny publisher: %w", err)
	}
	return nil
}
