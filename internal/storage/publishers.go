// Package storage provides database access for Catalyst
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" driver via init()
)

// PublisherVideoConfig holds per-publisher video inventory defaults (OpenRTB 2.5 Video object).
// These are used when the corresponding query param is absent from the ad tag URL.
type PublisherVideoConfig struct {
	Placement      int      `json:"placement"`
	Protocols      []int    `json:"protocols"`
	PlaybackMethod []int    `json:"playbackmethod"`
	API            []int    `json:"api"`
	Mimes          []string `json:"mimes"`
	MaxDur         int      `json:"maxdur"`
	MinDur         int      `json:"mindur"`
	MaxBitrate     int      `json:"maxbitrate"`
	MinBitrate     int      `json:"minbitrate"`
	Skip           int      `json:"skip"`
	SkipAfter      int      `json:"skipafter"`
}

// Publisher represents a publisher configuration from the database
type Publisher struct {
	ID             string                 `json:"id"`
	PublisherID    string                 `json:"publisher_id"`
	Name           string                 `json:"name"`
	AllowedDomains string                 `json:"allowed_domains"`
	BidderParams   map[string]interface{} `json:"bidder_params"`
	BidMultiplier  float64                `json:"bid_multiplier"` // Revenue share multiplier. Bid divided by this. 1.25 = 20% platform cut
	TMaxMs         int                    `json:"tmax_ms"`        // Per-publisher auction timeout in milliseconds
	VideoConfig    json.RawMessage        `json:"video_config"`   // Per-publisher video inventory defaults (JSONB)
	Status         string                 `json:"status"`
	Version        int                    `json:"version"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Notes          sql.NullString         `json:"notes,omitempty"`
	ContactEmail   sql.NullString         `json:"contact_email,omitempty"`
}

// GetAllowedDomains returns the allowed domains string (for middleware interface)
func (p *Publisher) GetAllowedDomains() string {
	return p.AllowedDomains
}

// GetBidMultiplier returns the bid multiplier (for exchange interface)
func (p *Publisher) GetBidMultiplier() float64 {
	return p.BidMultiplier
}

// GetPublisherID returns the publisher ID (for exchange interface)
func (p *Publisher) GetPublisherID() string {
	return p.PublisherID
}

// GetTMaxMs returns the per-publisher auction timeout in milliseconds (for exchange interface)
func (p *Publisher) GetTMaxMs() int {
	return p.TMaxMs
}

// GetVideoConfig parses and returns the publisher's video inventory config.
// Returns nil if no config is set.
func (p *Publisher) GetVideoConfig() *PublisherVideoConfig {
	if len(p.VideoConfig) == 0 {
		return nil
	}
	var cfg PublisherVideoConfig
	if err := json.Unmarshal(p.VideoConfig, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// PublisherStore provides database operations for publishers
type PublisherStore struct {
	db *sql.DB
}

// NewPublisherStore creates a new publisher store
func NewPublisherStore(db *sql.DB) *PublisherStore {
	return &PublisherStore{db: db}
}

// Ping checks if the database connection is alive
func (s *PublisherStore) Ping(ctx context.Context) error {
	if s.db == nil {
		return nil // No database configured, not an error
	}
	return s.db.PingContext(ctx)
}

// GetByPublisherID retrieves a publisher by their publisher_id
// Returns interface{} for middleware compatibility while maintaining concrete type internally
func (s *PublisherStore) GetByPublisherID(ctx context.Context, publisherID string) (interface{}, error) {
	return s.getByPublisherIDConcrete(ctx, publisherID)
}

// getByPublisherIDConcrete is the internal implementation returning concrete type
func (s *PublisherStore) getByPublisherIDConcrete(ctx context.Context, accountID string) (*Publisher, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT a.account_id, p.domain, p.name, p.status, p.default_timeout_ms,
		       p.bid_multiplier, p.video_config, p.notes, p.created_at, p.updated_at
		FROM publishers_new p
		JOIN accounts a ON p.account_id = a.id
		WHERE a.account_id = $1
		  AND p.status = 'active'
		  AND a.status = 'active'
		LIMIT 1
	`

	var p Publisher
	err := s.db.QueryRowContext(ctx, query, accountID).Scan(
		&p.PublisherID,
		&p.AllowedDomains,
		&p.Name,
		&p.Status,
		&p.TMaxMs,
		&p.BidMultiplier,
		&p.VideoConfig,
		&p.Notes,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Publisher not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query publisher: %w", err)
	}

	if p.BidMultiplier < 1.0 {
		p.BidMultiplier = 1.0 // Safety floor — never pay out more than the bid
	}

	return &p, nil
}

// List retrieves all active publishers
func (s *PublisherStore) List(ctx context.Context) ([]*Publisher, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT id, publisher_id, name, allowed_domains, bidder_params, bid_multiplier,
		       status, version, created_at, updated_at, notes, contact_email
		FROM publishers
		WHERE status = 'active'
		ORDER BY publisher_id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query publishers: %w", err)
	}
	defer rows.Close()

	publishers := make([]*Publisher, 0, 100)
	for rows.Next() {
		var p Publisher
		var bidderParamsJSON []byte

		err := rows.Scan(
			&p.ID,
			&p.PublisherID,
			&p.Name,
			&p.AllowedDomains,
			&bidderParamsJSON,
			&p.BidMultiplier,
			&p.Status,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Notes,
			&p.ContactEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan publisher row: %w", err)
		}

		// Parse JSONB bidder_params
		if len(bidderParamsJSON) > 0 {
			if err := json.Unmarshal(bidderParamsJSON, &p.BidderParams); err != nil {
				return nil, fmt.Errorf("failed to parse bidder_params: %w", err)
			}
		}

		publishers = append(publishers, &p)
	}

	return publishers, rows.Err()
}

// Create adds a new publisher
func (s *PublisherStore) Create(ctx context.Context, p *Publisher) error {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	// Default to 1.0 (no adjustment) if not set
	if p.BidMultiplier == 0 {
		p.BidMultiplier = 1.0
	}

	// Default status to 'active' if not set to prevent DB constraint violation
	status := p.Status
	if status == "" {
		status = "active"
	}

	query := `
		INSERT INTO publishers (
			publisher_id, name, allowed_domains, bidder_params, bid_multiplier, status, notes, contact_email
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, version, created_at, updated_at
	`

	bidderParamsJSON, err := json.Marshal(p.BidderParams)
	if err != nil {
		return fmt.Errorf("failed to marshal bidder_params: %w", err)
	}

	err = s.db.QueryRowContext(ctx, query,
		p.PublisherID,
		p.Name,
		p.AllowedDomains,
		bidderParamsJSON,
		p.BidMultiplier,
		status,
		p.Notes,
		p.ContactEmail,
	).Scan(&p.ID, &p.Version, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create publisher: %w", err)
	}

	return nil
}

// Update modifies an existing publisher using optimistic locking
func (s *PublisherStore) Update(ctx context.Context, p *Publisher) error {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	// Begin transaction for optimistic locking
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check current version
	var currentVersion int
	err = tx.QueryRowContext(ctx, "SELECT version FROM publishers WHERE publisher_id = $1", p.PublisherID).Scan(&currentVersion)
	if err == sql.ErrNoRows {
		return fmt.Errorf("publisher not found: %s", p.PublisherID)
	}
	if err != nil {
		return fmt.Errorf("failed to check version: %w", err)
	}

	// Verify version matches (optimistic lock check)
	if currentVersion != p.Version {
		return fmt.Errorf("concurrent modification detected: publisher %s was updated by another process", p.PublisherID)
	}

	query := `
		UPDATE publishers
		SET name = $1, allowed_domains = $2, bidder_params = $3,
		    bid_multiplier = $4, status = $5, notes = $6, contact_email = $7
		WHERE publisher_id = $8 AND version = $9
	`

	bidderParamsJSON, err := json.Marshal(p.BidderParams)
	if err != nil {
		return fmt.Errorf("failed to marshal bidder_params: %w", err)
	}

	result, err := tx.ExecContext(ctx, query,
		p.Name,
		p.AllowedDomains,
		bidderParamsJSON,
		p.BidMultiplier,
		p.Status,
		p.Notes,
		p.ContactEmail,
		p.PublisherID,
		p.Version,
	)

	if err != nil {
		return fmt.Errorf("failed to update publisher: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("concurrent modification detected: publisher %s version mismatch", p.PublisherID)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update version in the struct for caller
	p.Version = currentVersion + 1

	return nil
}

// Delete soft-deletes a publisher by setting status to 'archived'
func (s *PublisherStore) Delete(ctx context.Context, publisherID string) error {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		UPDATE publishers
		SET status = 'archived'
		WHERE publisher_id = $1
	`

	result, err := s.db.ExecContext(ctx, query, publisherID)
	if err != nil {
		return fmt.Errorf("failed to delete publisher: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("publisher not found: %s", publisherID)
	}

	return nil
}

// GetBidderParams retrieves bidder parameters for a specific bidder (publisher-level only)
func (s *PublisherStore) GetBidderParams(ctx context.Context, publisherID, bidderCode string) (map[string]interface{}, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT bidder_params->$2 as params
		FROM publishers
		WHERE publisher_id = $1 AND status = 'active'
	`

	var paramsJSON []byte
	err := s.db.QueryRowContext(ctx, query, publisherID, bidderCode).Scan(&paramsJSON)

	if err == sql.ErrNoRows {
		return nil, nil // No params for this bidder
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query bidder params: %w", err)
	}

	if len(paramsJSON) == 0 {
		return nil, nil
	}

	// First unmarshal to interface{} to handle any JSON type
	var rawValue interface{}
	if err := json.Unmarshal(paramsJSON, &rawValue); err != nil {
		return nil, fmt.Errorf("failed to parse bidder params: %w", err)
	}

	// If it's already a map, return it
	if params, ok := rawValue.(map[string]interface{}); ok {
		return params, nil
	}

	// If it's a scalar or array, wrap it in a map with "value" key
	// This maintains backwards compatibility while supporting non-object params
	return map[string]interface{}{
		"value": rawValue,
	}, nil
}


// NewDBConnection creates a new database connection
// The caller should pass a context with appropriate timeout for connection establishment
func NewDBConnection(ctx context.Context, host, port, user, password, dbname, sslmode string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for high-concurrency auction workload
	db.SetMaxOpenConns(100) // Increased for parallel bidder lookups
	db.SetMaxIdleConns(25)  // Keep more idle connections ready
	db.SetConnMaxLifetime(10 * time.Minute)

	// Test connection using provided context
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}


// GetAdUnitPathFromDivID looks up the ad unit path from a div ID mapping
// This allows the server to resolve ad unit paths when the SDK only sends div IDs
func (s *PublisherStore) GetAdUnitPathFromDivID(ctx context.Context, publisherID, domain, divID string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not available")
	}

	var adUnitPath string
	query := `
		SELECT ad_unit_path
		FROM slot_mappings
		WHERE publisher_id = $1
		  AND domain = $2
		  AND div_id = $3
		LIMIT 1
	`

	err := s.db.QueryRowContext(ctx, query, publisherID, domain, divID).Scan(&adUnitPath)
	if err == sql.ErrNoRows {
		// Not found - return empty string (not an error, just no mapping exists)
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("error querying slot mapping: %w", err)
	}

	return adUnitPath, nil
}

// PublisherNew represents a publisher in the normalized schema
type PublisherNew struct {
	ID              int
	AccountID       int
	Domain          string
	Name            string
	Status          string
	DefaultTimeout  int
	DefaultCurrency string
	Notes           sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GetSlotBidderConfigs retrieves bidder configurations for a specific ad slot
func (s *PublisherStore) GetSlotBidderConfigs(ctx context.Context, accountID, domain, slotPattern, deviceType string) (map[string]map[string]interface{}, error) {
	query := `
		SELECT
			b.code AS bidder_code,
			sbc.bidder_params
		FROM slot_bidder_configs sbc
		JOIN ad_slots s ON sbc.ad_slot_id = s.id
		JOIN publishers_new p ON s.publisher_id = p.id
		JOIN accounts a ON p.account_id = a.id
		JOIN bidders_new b ON sbc.bidder_id = b.id
		WHERE a.account_id = $1
		  AND p.domain = $2
		  AND s.slot_pattern = $3
		  AND (sbc.device_type = $4 OR sbc.device_type = 'all')
		  AND sbc.status = 'active'
		  AND s.status = 'active'
		  AND p.status = 'active'
		  AND a.status = 'active'
		  AND b.status = 'active'
	`

	rows, err := s.db.QueryContext(ctx, query, accountID, domain, slotPattern, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to query slot bidder configs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]interface{})

	for rows.Next() {
		var bidderCode string
		var paramsJSON []byte

		if err := rows.Scan(&bidderCode, &paramsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var params map[string]interface{}
		if err := json.Unmarshal(paramsJSON, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bidder params: %w", err)
		}

		result[bidderCode] = params
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bidder configs: %w", err)
	}

	// Merge account-level defaults (tier 1) under slot params (tier 2 wins on conflict).
	// Failures here are non-fatal — we still return the slot-only params.
	if defaults, err := s.getAccountBidderDefaultsMap(ctx, accountID); err == nil {
		for code, def := range defaults {
			if slot, ok := result[code]; ok {
				result[code] = mergeBidderParams(def, slot)
			}
		}
	}

	return result, nil
}

// SlotBidderConfigRow is a flat view of slot_bidder_configs with joined account/domain/bidder info.
type SlotBidderConfigRow struct {
	ID           int             `json:"id"`
	AccountID    string          `json:"account_id"`
	Domain       string          `json:"domain"`
	AdSlotID     int             `json:"ad_slot_id"`
	SlotPattern  string          `json:"slot_pattern"`
	BidderID     int             `json:"bidder_id"`
	BidderCode   string          `json:"bidder_code"`
	DeviceType   string          `json:"device_type"`
	BidderParams json.RawMessage `json:"bidder_params"`
	Status       string          `json:"status"`
}

// GetAllSlotBidderConfigs returns all slot_bidder_configs rows with joined account/domain/bidder info.
func (s *PublisherStore) GetAllSlotBidderConfigs(ctx context.Context) ([]SlotBidderConfigRow, error) {
	query := `
		SELECT sbc.id, a.account_id, p.domain, sbc.ad_slot_id, s.slot_pattern,
		       sbc.bidder_id, b.code AS bidder_code, sbc.device_type, sbc.bidder_params, sbc.status
		FROM slot_bidder_configs sbc
		JOIN ad_slots s       ON sbc.ad_slot_id = s.id
		JOIN publishers_new p ON s.publisher_id = p.id
		JOIN accounts a       ON p.account_id   = a.id
		JOIN bidders_new b    ON sbc.bidder_id   = b.id
		ORDER BY a.account_id, p.domain, s.slot_pattern, b.code, sbc.device_type
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all slot bidder configs: %w", err)
	}
	defer rows.Close()

	var result []SlotBidderConfigRow
	for rows.Next() {
		var row SlotBidderConfigRow
		if err := rows.Scan(&row.ID, &row.AccountID, &row.Domain, &row.AdSlotID, &row.SlotPattern,
			&row.BidderID, &row.BidderCode, &row.DeviceType, &row.BidderParams, &row.Status); err != nil {
			return nil, fmt.Errorf("failed to scan slot bidder config row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating slot bidder configs: %w", err)
	}
	return result, nil
}

// UpdateSlotBidderParams updates the bidder_params JSON for a specific slot_bidder_configs row.
func (s *PublisherStore) UpdateSlotBidderParams(ctx context.Context, id int, params json.RawMessage) error {

	_, err := s.db.ExecContext(ctx,
		`UPDATE slot_bidder_configs SET bidder_params = $2, updated_at = NOW() WHERE id = $1`,
		id, params,
	)
	if err != nil {
		return fmt.Errorf("failed to update slot bidder params: %w", err)
	}
	return nil
}

// GetByAccountID retrieves publisher information by account ID
func (s *PublisherStore) GetByAccountID(ctx context.Context, accountID string) (*PublisherNew, error) {
	query := `
		SELECT
			p.id, p.account_id, p.domain, p.name, p.status,
			p.default_timeout_ms, p.default_currency,
			p.notes, p.created_at, p.updated_at
		FROM publishers_new p
		JOIN accounts a ON p.account_id = a.id
		WHERE a.account_id = $1
		  AND p.status = 'active'
		  AND a.status = 'active'
		LIMIT 1
	`

	var pub PublisherNew
	err := s.db.QueryRowContext(ctx, query, accountID).Scan(
		&pub.ID, &pub.AccountID, &pub.Domain, &pub.Name, &pub.Status,
		&pub.DefaultTimeout, &pub.DefaultCurrency,
		&pub.Notes, &pub.CreatedAt, &pub.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query publisher: %w", err)
	}

	return &pub, nil
}

// ============================================================================
// Onboarding admin — structs and CRUD helpers
// ============================================================================

// AccountRow is a flat view of an account row for admin display.
type AccountRow struct {
	ID         int            `json:"id"`
	AccountID  string         `json:"account_id"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Publishers []PublisherRow `json:"publishers"`
}

// PublisherRow is a flat view of a publishers_new row for admin display.
type PublisherRow struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// AdSlotRow is a flat view of an ad_slots row with account/domain context.
type AdSlotRow struct {
	ID          int    `json:"id"`
	AccountID   string `json:"account_id"`
	PublisherID int    `json:"publisher_id"`
	Domain      string `json:"domain"`
	SlotPattern string `json:"slot_pattern"`
	SlotName    string `json:"slot_name"`
	IsAdhesion  bool   `json:"is_adhesion"`
	Status      string `json:"status"`
}

// BidderRow is a flat view of a bidders_new row for admin display.
type BidderRow struct {
	ID          int             `json:"id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	ParamSchema json.RawMessage `json:"param_schema"`
}

// GetAllAccountsWithPublishers returns all accounts with their nested publishers.
func (s *PublisherStore) GetAllAccountsWithPublishers(ctx context.Context) ([]AccountRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.account_id, COALESCE(a.name,''), a.status,
		       COALESCE(p.id,0), COALESCE(p.domain,''), COALESCE(p.name,''), COALESCE(p.status,'')
		FROM accounts a
		LEFT JOIN publishers_new p ON p.account_id = a.id
		ORDER BY a.account_id, p.domain
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var result []AccountRow
	accountIdx := map[int]int{}
	for rows.Next() {
		var aID int
		var aAccID, aName, aStatus string
		var pID int
		var pDomain, pName, pStatus string
		if err := rows.Scan(&aID, &aAccID, &aName, &aStatus, &pID, &pDomain, &pName, &pStatus); err != nil {
			return nil, fmt.Errorf("failed to scan account row: %w", err)
		}
		idx, exists := accountIdx[aID]
		if !exists {
			result = append(result, AccountRow{ID: aID, AccountID: aAccID, Name: aName, Status: aStatus})
			idx = len(result) - 1
			accountIdx[aID] = idx
		}
		if pID > 0 {
			result[idx].Publishers = append(result[idx].Publishers, PublisherRow{
				ID: pID, Domain: pDomain, Name: pName, Status: pStatus,
			})
		}
	}
	return result, rows.Err()
}

// GetAllAdSlots returns all ad slots with publisher/account context.
func (s *PublisherStore) GetAllAdSlots(ctx context.Context) ([]AdSlotRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, a.account_id, p.id, p.domain, s.slot_pattern,
		       COALESCE(s.slot_name,''), s.is_adhesion, s.status
		FROM ad_slots s
		JOIN publishers_new p ON s.publisher_id = p.id
		JOIN accounts a       ON p.account_id = a.id
		ORDER BY a.account_id, p.domain, s.slot_pattern
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query ad slots: %w", err)
	}
	defer rows.Close()

	var result []AdSlotRow
	for rows.Next() {
		var r AdSlotRow
		if err := rows.Scan(&r.ID, &r.AccountID, &r.PublisherID, &r.Domain,
			&r.SlotPattern, &r.SlotName, &r.IsAdhesion, &r.Status); err != nil {
			return nil, fmt.Errorf("failed to scan ad slot row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetAllBidders returns all active bidders with their param schemas.
func (s *PublisherStore) GetAllBidders(ctx context.Context) ([]BidderRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, COALESCE(name,''), COALESCE(param_schema,'null'::jsonb)
		FROM bidders_new
		WHERE status = 'active'
		ORDER BY code
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query bidders: %w", err)
	}
	defer rows.Close()

	var result []BidderRow
	for rows.Next() {
		var r BidderRow
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.ParamSchema); err != nil {
			return nil, fmt.Errorf("failed to scan bidder row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// CreateAccountIfNotExists creates or upserts an account row and returns its DB id.
func (s *PublisherStore) CreateAccountIfNotExists(ctx context.Context, accountID, name string) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO accounts (account_id, name)
		VALUES ($1, $2)
		ON CONFLICT (account_id) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()
		RETURNING id
	`, accountID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert account: %w", err)
	}
	return id, nil
}

// CreatePublisher inserts a publishers_new row. Returns (id, nil) on success
// or (0, nil) if the row already exists (idempotent).
func (s *PublisherStore) CreatePublisher(ctx context.Context, accountDBID int, domain, name string) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO publishers_new (account_id, domain, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, domain) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()
		RETURNING id
	`, accountDBID, domain, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert publisher: %w", err)
	}
	return id, nil
}

// CreateAdSlot inserts an ad_slots row. Returns the new row's id.
func (s *PublisherStore) CreateAdSlot(ctx context.Context, publisherDBID int, slotPattern, slotName string) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (publisher_id, slot_pattern) DO UPDATE SET slot_name = EXCLUDED.slot_name, updated_at = NOW()
		RETURNING id
	`, publisherDBID, slotPattern, slotName).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert ad slot: %w", err)
	}
	return id, nil
}

// CreateSlotBidderConfig inserts a slot_bidder_configs row (idempotent on conflict).
func (s *PublisherStore) CreateSlotBidderConfig(ctx context.Context, adSlotID, bidderDBID int, deviceType string, params json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ad_slot_id, bidder_id, device_type) DO NOTHING
	`, adSlotID, bidderDBID, deviceType, params)
	if err != nil {
		return fmt.Errorf("failed to insert slot bidder config: %w", err)
	}
	return nil
}

// UpdatePublisher updates a publisher's domain, name, and status by ID.
func (s *PublisherStore) UpdatePublisher(ctx context.Context, id int, domain, name, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE publishers_new SET domain=$2, name=$3, status=$4, updated_at=NOW() WHERE id=$1
	`, id, domain, name, status)
	if err != nil {
		return fmt.Errorf("failed to update publisher %d: %w", id, err)
	}
	return nil
}

// UpdateAdSlot updates an ad slot's pattern, name, and status by ID.
func (s *PublisherStore) UpdateAdSlot(ctx context.Context, id int, slotPattern, slotName, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ad_slots SET slot_pattern=$2, slot_name=$3, status=$4, updated_at=NOW() WHERE id=$1
	`, id, slotPattern, slotName, status)
	if err != nil {
		return fmt.Errorf("failed to update ad slot %d: %w", id, err)
	}
	return nil
}

// UpdateSlotBidderConfigFull replaces all editable fields of a slot_bidder_configs row.
func (s *PublisherStore) UpdateSlotBidderConfigFull(ctx context.Context, id, adSlotID, bidderDBID int, deviceType string, params json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE slot_bidder_configs SET ad_slot_id=$2, bidder_id=$3, device_type=$4, bidder_params=$5 WHERE id=$1
	`, id, adSlotID, bidderDBID, deviceType, params)
	if err != nil {
		return fmt.Errorf("failed to update slot bidder config %d: %w", id, err)
	}
	return nil
}

// UpdateBidderName updates the display name of a bidder by ID.
func (s *PublisherStore) UpdateBidderName(ctx context.Context, id int, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bidders_new SET name=$2 WHERE id=$1`, id, name)
	if err != nil {
		return fmt.Errorf("failed to update bidder %d: %w", id, err)
	}
	return nil
}

// ─── Account Bidder Defaults ──────────────────────────────────────────────────

// AccountBidderDefault holds account-level SSP params shared across all slots.
type AccountBidderDefault struct {
	AccountDBID int             `json:"account_db_id"`
	AccountID   string          `json:"account_id"`
	BidderID    int             `json:"bidder_id"`
	BidderCode  string          `json:"bidder_code"`
	BaseParams  json.RawMessage `json:"base_params"`
}

// GetAllAccountBidderDefaults returns all rows from account_bidder_defaults with joined account/bidder info.
func (s *PublisherStore) GetAllAccountBidderDefaults(ctx context.Context) ([]AccountBidderDefault, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT abd.account_id, a.account_id, abd.bidder_id, b.code, abd.base_params
		FROM account_bidder_defaults abd
		JOIN accounts a     ON abd.account_id = a.id
		JOIN bidders_new b  ON abd.bidder_id  = b.id
		ORDER BY a.account_id, b.code
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query account bidder defaults: %w", err)
	}
	defer rows.Close()
	var result []AccountBidderDefault
	for rows.Next() {
		var r AccountBidderDefault
		if err := rows.Scan(&r.AccountDBID, &r.AccountID, &r.BidderID, &r.BidderCode, &r.BaseParams); err != nil {
			return nil, fmt.Errorf("failed to scan account bidder default: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpsertAccountBidderDefault creates or replaces the base params for a given account+bidder pair.
func (s *PublisherStore) UpsertAccountBidderDefault(ctx context.Context, accountDBID, bidderDBID int, params json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO account_bidder_defaults (account_id, bidder_id, base_params, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (account_id, bidder_id) DO UPDATE
		    SET base_params = EXCLUDED.base_params, updated_at = NOW()
	`, accountDBID, bidderDBID, params)
	if err != nil {
		return fmt.Errorf("failed to upsert account bidder default: %w", err)
	}
	return nil
}

// getAccountBidderDefaultsMap returns a map[bidderCode]map[string]interface{} for the given accountID string.
// Used internally to merge into slot configs at auction time.
func (s *PublisherStore) getAccountBidderDefaultsMap(ctx context.Context, accountID string) (map[string]map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.code, abd.base_params
		FROM account_bidder_defaults abd
		JOIN accounts a    ON abd.account_id = a.id
		JOIN bidders_new b ON abd.bidder_id  = b.id
		WHERE a.account_id = $1
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query account bidder defaults for %s: %w", accountID, err)
	}
	defer rows.Close()
	result := map[string]map[string]interface{}{}
	for rows.Next() {
		var code string
		var raw json.RawMessage
		if err := rows.Scan(&code, &raw); err != nil {
			return nil, err
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err == nil {
			result[code] = m
		}
	}
	return result, rows.Err()
}

// mergeBidderParams merges base (account-level) params with slot-level overrides.
// Slot-level values win on conflict. Both inputs may be nil.
func mergeBidderParams(base, slot map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(slot))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range slot {
		out[k] = v
	}
	return out
}

// AdSlotExportRow holds the data needed to generate an ad tag for a slot.
type AdSlotExportRow struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	SlotPattern string `json:"slot_pattern"`
	SlotName    string `json:"slot_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// GetAdSlotsForExport returns all ad slots (optionally filtered by account) with
// the first banner size from their media type profile (defaults to 300×250).
func (s *PublisherStore) GetAdSlotsForExport(ctx context.Context, accountID string) ([]AdSlotExportRow, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT
			a.account_id,
			a.name,
			sl.slot_pattern,
			sl.slot_name,
			COALESCE(
				NULLIF((mtp.media_types->'banner'->'sizes'->0->>0)::text, '')::integer,
				300
			) AS width,
			COALESCE(
				NULLIF((mtp.media_types->'banner'->'sizes'->0->>1)::text, '')::integer,
				250
			) AS height
		FROM ad_slots sl
		JOIN publishers_new p ON p.id = sl.publisher_id
		JOIN accounts a ON a.id = p.account_id
		LEFT JOIN media_type_profiles mtp ON mtp.id = sl.media_type_profile_id
		WHERE ($1 = '' OR a.account_id = $1)
		  AND sl.status = 'active'
		ORDER BY a.account_id, sl.slot_pattern
	`

	rows, err := s.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ad slots for export: %w", err)
	}
	defer rows.Close()

	var result []AdSlotExportRow
	for rows.Next() {
		var r AdSlotExportRow
		if err := rows.Scan(&r.AccountID, &r.AccountName, &r.SlotPattern, &r.SlotName, &r.Width, &r.Height); err != nil {
			return nil, fmt.Errorf("failed to scan ad slot export row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
