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

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bidder configs: %w", err)
	}

	return result, nil
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
