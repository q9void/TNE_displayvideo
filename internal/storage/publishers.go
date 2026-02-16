// Package storage provides database access for Catalyst
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
	"github.com/lib/pq" // PostgreSQL driver (use pq.Array for batch queries)
)

// Publisher represents a publisher configuration from the database
type Publisher struct {
	ID             string                 `json:"id"`
	PublisherID    string                 `json:"publisher_id"`
	Name           string                 `json:"name"`
	AllowedDomains string                 `json:"allowed_domains"`
	BidderParams   map[string]interface{} `json:"bidder_params"`
	BidMultiplier  float64                `json:"bid_multiplier"` // Revenue share multiplier (1.0000-10.0000). Bid divided by this. 1.05 = ~5% platform cut
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
func (s *PublisherStore) getByPublisherIDConcrete(ctx context.Context, publisherID string) (*Publisher, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT id, publisher_id, name, allowed_domains, bidder_params, bid_multiplier,
		       status, version, created_at, updated_at, notes, contact_email
		FROM publishers
		WHERE publisher_id = $1 AND status = 'active'
	`

	var p Publisher
	var bidderParamsJSON []byte

	err := s.db.QueryRowContext(ctx, query, publisherID).Scan(
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

	if err == sql.ErrNoRows {
		return nil, nil // Publisher not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query publisher: %w", err)
	}

	// Parse JSONB bidder_params
	if len(bidderParamsJSON) > 0 {
		if err := json.Unmarshal(bidderParamsJSON, &p.BidderParams); err != nil {
			return nil, fmt.Errorf("failed to parse bidder_params: %w", err)
		}
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

// GetDomainBidderConfig retrieves domain-level bidder configuration
func (s *PublisherStore) GetDomainBidderConfig(ctx context.Context, publisherID, domain, bidderCode string) (map[string]interface{}, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT params
		FROM domain_bidder_configs
		WHERE publisher_id = $1 AND domain = $2 AND bidder_code = $3
	`

	var paramsJSON []byte
	err := s.db.QueryRowContext(ctx, query, publisherID, domain, bidderCode).Scan(&paramsJSON)

	if err == sql.ErrNoRows {
		return nil, nil // No domain-level config
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query domain bidder config: %w", err)
	}

	if len(paramsJSON) == 0 {
		return nil, nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("failed to parse domain bidder params: %w", err)
	}

	return params, nil
}

// GetUnitBidderConfig retrieves unit-level bidder configuration
func (s *PublisherStore) GetUnitBidderConfig(ctx context.Context, publisherID, domain, adUnitPath, bidderCode string) (map[string]interface{}, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT params
		FROM unit_bidder_configs
		WHERE publisher_id = $1 AND domain = $2 AND ad_unit_path = $3 AND bidder_code = $4
	`

	var paramsJSON []byte
	err := s.db.QueryRowContext(ctx, query, publisherID, domain, adUnitPath, bidderCode).Scan(&paramsJSON)

	if err == sql.ErrNoRows {
		return nil, nil // No unit-level config
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query unit bidder config: %w", err)
	}

	if len(paramsJSON) == 0 {
		return nil, nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("failed to parse unit bidder params: %w", err)
	}

	return params, nil
}

// GetBidderConfigHierarchical retrieves bidder configuration with hierarchical fallback
// Lookup order: Unit-level → Domain-level → Publisher-level → nil
func (s *PublisherStore) GetBidderConfigHierarchical(ctx context.Context, publisherID, domain, adUnitPath, bidderCode string) (map[string]interface{}, error) {
	log := logger.Log.With().
		Str("publisher", publisherID).
		Str("domain", domain).
		Str("ad_unit", adUnitPath).
		Str("bidder", bidderCode).
		Logger()

	// Try unit-level config first (most specific)
	if adUnitPath != "" {
		log.Debug().Msg("🔍 Checking unit-level config...")
		params, err := s.GetUnitBidderConfig(ctx, publisherID, domain, adUnitPath, bidderCode)
		if err != nil {
			log.Error().Err(err).Msg("❌ Error checking unit-level config")
			return nil, fmt.Errorf("error checking unit-level config: %w", err)
		}
		if params != nil {
			log.Info().Interface("params", params).Msg("✓ Found unit-level config")
			return params, nil
		}
		log.Debug().Msg("⚠️  No unit-level config found")
	}

	// Try domain-level config second
	if domain != "" {
		log.Debug().Msg("🔍 Checking domain-level config...")
		params, err := s.GetDomainBidderConfig(ctx, publisherID, domain, bidderCode)
		if err != nil {
			log.Error().Err(err).Msg("❌ Error checking domain-level config")
			return nil, fmt.Errorf("error checking domain-level config: %w", err)
		}
		if params != nil {
			log.Info().Interface("params", params).Msg("✓ Found domain-level config")
			return params, nil
		}
		log.Debug().Msg("⚠️  No domain-level config found")
	}

	// Fall back to publisher-level config
	log.Debug().Msg("🔍 Checking publisher-level config...")
	params, err := s.GetBidderParams(ctx, publisherID, bidderCode)
	if err != nil {
		log.Error().Err(err).Msg("❌ Error checking publisher-level config")
		return nil, fmt.Errorf("error checking publisher-level config: %w", err)
	}

	if params != nil {
		log.Info().Interface("params", params).Msg("✓ Found publisher-level config")
		return params, nil
	}

	log.Warn().Msg("❌ No config found at any level (unit/domain/publisher)")
	return nil, nil
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

// GetAllBidderConfigsHierarchical retrieves all bidder configurations with hierarchical fallback in a single batch
// This is much more efficient than calling GetBidderConfigHierarchical for each bidder separately
// Returns map[bidderCode]params
func (s *PublisherStore) GetAllBidderConfigsHierarchical(ctx context.Context, publisherID, domain, adUnitPath string, bidders []string) (map[string]map[string]interface{}, error) {
	if len(bidders) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	result := make(map[string]map[string]interface{})
	remaining := make(map[string]bool)
	for _, b := range bidders {
		remaining[b] = true
	}

	// Try unit-level configs first (most specific)
	if adUnitPath != "" {
		query := `SELECT bidder_code, params FROM unit_bidder_configs 
		          WHERE publisher_id = $1 AND domain = $2 AND ad_unit_path = $3 AND bidder_code = ANY($4)`
		
		rows, err := s.db.QueryContext(ctx, query, publisherID, domain, adUnitPath, pq.Array(bidders))
		if err != nil {
			return nil, fmt.Errorf("error querying unit-level configs: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var bidderCode string
			var paramsJSON []byte
			if err := rows.Scan(&bidderCode, &paramsJSON); err != nil {
				return nil, fmt.Errorf("error scanning unit-level config: %w", err)
			}

			var params map[string]interface{}
			if err := json.Unmarshal(paramsJSON, &params); err != nil {
				return nil, fmt.Errorf("error unmarshaling unit-level params: %w", err)
			}

			result[bidderCode] = params
			delete(remaining, bidderCode)
		}
	}

	// Try domain-level configs for remaining bidders
	if domain != "" && len(remaining) > 0 {
		remainingBidders := make([]string, 0, len(remaining))
		for b := range remaining {
			remainingBidders = append(remainingBidders, b)
		}

		query := `SELECT bidder_code, params FROM domain_bidder_configs 
		          WHERE publisher_id = $1 AND domain = $2 AND bidder_code = ANY($3)`
		
		rows, err := s.db.QueryContext(ctx, query, publisherID, domain, pq.Array(remainingBidders))
		if err != nil {
			return nil, fmt.Errorf("error querying domain-level configs: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var bidderCode string
			var paramsJSON []byte
			if err := rows.Scan(&bidderCode, &paramsJSON); err != nil {
				return nil, fmt.Errorf("error scanning domain-level config: %w", err)
			}

			var params map[string]interface{}
			if err := json.Unmarshal(paramsJSON, &params); err != nil {
				return nil, fmt.Errorf("error unmarshaling domain-level params: %w", err)
			}

			result[bidderCode] = params
			delete(remaining, bidderCode)
		}
	}

	// Get publisher-level configs for any remaining bidders
	if len(remaining) > 0 {
		// Get the entire bidder_params JSONB and filter in Go
		query := `SELECT bidder_params FROM publishers WHERE publisher_id = $1 AND status = 'active'`

		var bidderParamsJSON []byte
		err := s.db.QueryRowContext(ctx, query, publisherID).Scan(&bidderParamsJSON)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("error querying publisher-level configs: %w", err)
		}

		if len(bidderParamsJSON) > 0 {
			var allBidderParams map[string]map[string]interface{}
			if err := json.Unmarshal(bidderParamsJSON, &allBidderParams); err != nil {
				return nil, fmt.Errorf("error unmarshaling publisher bidder_params: %w", err)
			}

			// Extract only the requested bidders
			for bidderCode := range remaining {
				if params, exists := allBidderParams[bidderCode]; exists {
					result[bidderCode] = params
					delete(remaining, bidderCode)
				}
			}
		}
	}

	return result, nil
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
	PBSAccountID    sql.NullString
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
			p.pbs_account_id, p.default_timeout_ms, p.default_currency,
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
		&pub.PBSAccountID, &pub.DefaultTimeout, &pub.DefaultCurrency,
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
