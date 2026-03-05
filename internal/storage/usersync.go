// Package storage provides database access for user sync data
package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// ConsentHash computes the SHA-256 hex digest of a TCF consent string.
// Returns empty string when consentString is empty (non-GDPR traffic).
func ConsentHash(consentString string) string {
	if consentString == "" {
		return ""
	}
	h := sha256.Sum256([]byte(consentString))
	return hex.EncodeToString(h[:])
}

// UserSync represents a user sync record from the database
type UserSync struct {
	ID          int        `json:"id"`
	FPID        string     `json:"fpid"`
	BidderCode  string     `json:"bidder_code"`
	BidderUID   *string    `json:"bidder_uid,omitempty"`
	SyncedAt    time.Time  `json:"synced_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ConsentHash *string    `json:"consent_hash,omitempty"` // SHA-256 of TCF consent string at sync time
}

// UserSyncStore provides database operations for user syncs
type UserSyncStore struct {
	db *sql.DB
}

// NewUserSyncStore creates a new user sync store
func NewUserSyncStore(db *sql.DB) *UserSyncStore {
	return &UserSyncStore{db: db}
}

// UpsertSync creates or updates a sync record for a bidder.
// consentHash is the SHA-256 hex of the TCF consent string at sync time
// (use ConsentHash() to compute it; pass "" for non-GDPR traffic).
// If bidderUID is nil, it creates a sync record without a UID (waiting for callback).
// If a UID already exists and a new UID is provided, it UPDATES to the new UID.
func (s *UserSyncStore) UpsertSync(ctx context.Context, fpid, bidderCode string, bidderUID *string, expiresAt *time.Time, consentHash string) error {
	if s.db == nil {
		return nil // Database disabled, skip
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	var ch *string
	if consentHash != "" {
		ch = &consentHash
	}

	query := `
		INSERT INTO user_syncs (fpid, bidder_code, bidder_uid, expires_at, consent_hash, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (fpid, bidder_code)
		DO UPDATE SET
			bidder_uid    = COALESCE(EXCLUDED.bidder_uid, user_syncs.bidder_uid),
			expires_at    = EXCLUDED.expires_at,
			consent_hash  = EXCLUDED.consent_hash,
			updated_at    = NOW()
	`

	_, err := s.db.ExecContext(ctx, query, fpid, bidderCode, bidderUID, expiresAt, ch)
	if err != nil {
		return fmt.Errorf("failed to upsert user sync: %w", err)
	}

	return nil
}

// UpdateUID updates the bidder UID for an existing sync record
func (s *UserSyncStore) UpdateUID(ctx context.Context, fpid, bidderCode, bidderUID string) error {
	if s.db == nil {
		return nil // Database disabled, skip
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		UPDATE user_syncs
		SET bidder_uid = $1, updated_at = NOW()
		WHERE fpid = $2 AND bidder_code = $3
	`

	result, err := s.db.ExecContext(ctx, query, bidderUID, fpid, bidderCode)
	if err != nil {
		return fmt.Errorf("failed to update user sync UID: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		// No existing record - create one (no consent context available here)
		return s.UpsertSync(ctx, fpid, bidderCode, &bidderUID, nil, "")
	}

	return nil
}

// GetSyncsForUser retrieves all synced UIDs for a given FPID
// Returns map of bidder_code -> bidder_uid
func (s *UserSyncStore) GetSyncsForUser(ctx context.Context, fpid string) (map[string]string, error) {
	if s.db == nil {
		return make(map[string]string), nil // Database disabled, return empty map
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT bidder_code, bidder_uid
		FROM user_syncs
		WHERE fpid = $1
		  AND bidder_uid IS NOT NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
	`

	rows, err := s.db.QueryContext(ctx, query, fpid)
	if err != nil {
		return nil, fmt.Errorf("failed to query user syncs: %w", err)
	}
	defer rows.Close()

	syncs := make(map[string]string)
	for rows.Next() {
		var bidderCode, bidderUID string
		if err := rows.Scan(&bidderCode, &bidderUID); err != nil {
			return nil, fmt.Errorf("failed to scan user sync row: %w", err)
		}
		syncs[bidderCode] = bidderUID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user sync rows: %w", err)
	}

	return syncs, nil
}

// GetSyncsForUserFiltered retrieves synced UIDs for a given FPID, filtered by consent hash.
// When currentConsentHash is non-empty (GDPR applies), only UIDs whose stored consent
// hash matches — or is NULL (pre-migration / non-GDPR syncs) — are returned.
// When currentConsentHash is empty (GDPR does not apply), all non-expired UIDs are returned.
func (s *UserSyncStore) GetSyncsForUserFiltered(ctx context.Context, fpid, currentConsentHash string) (map[string]string, error) {
	if s.db == nil {
		return make(map[string]string), nil
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	var (
		rows *sql.Rows
		err  error
	)

	if currentConsentHash == "" {
		// Non-GDPR traffic: use all valid UIDs regardless of stored consent hash.
		rows, err = s.db.QueryContext(ctx, `
			SELECT bidder_code, bidder_uid
			FROM user_syncs
			WHERE fpid = $1
			  AND bidder_uid IS NOT NULL
			  AND (expires_at IS NULL OR expires_at > NOW())
		`, fpid)
	} else {
		// GDPR traffic: only return UIDs synced with the same consent or without consent context.
		rows, err = s.db.QueryContext(ctx, `
			SELECT bidder_code, bidder_uid
			FROM user_syncs
			WHERE fpid = $1
			  AND bidder_uid IS NOT NULL
			  AND (expires_at IS NULL OR expires_at > NOW())
			  AND (consent_hash = $2 OR consent_hash IS NULL)
		`, fpid, currentConsentHash)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query user syncs (filtered): %w", err)
	}
	defer rows.Close()

	syncs := make(map[string]string)
	for rows.Next() {
		var bidderCode, bidderUID string
		if err := rows.Scan(&bidderCode, &bidderUID); err != nil {
			return nil, fmt.Errorf("failed to scan user sync row: %w", err)
		}
		syncs[bidderCode] = bidderUID
	}
	return syncs, rows.Err()
}

// MarkUsed updates the last_used_at timestamp for a sync record
func (s *UserSyncStore) MarkUsed(ctx context.Context, fpid, bidderCode string) error {
	if s.db == nil {
		return nil // Database disabled, skip
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		UPDATE user_syncs
		SET last_used_at = NOW()
		WHERE fpid = $1 AND bidder_code = $2
	`

	_, err := s.db.ExecContext(ctx, query, fpid, bidderCode)
	if err != nil {
		return fmt.Errorf("failed to mark user sync as used: %w", err)
	}

	return nil
}

// DeleteExpired removes expired sync records
func (s *UserSyncStore) DeleteExpired(ctx context.Context) (int64, error) {
	if s.db == nil {
		return 0, nil // Database disabled, skip
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		DELETE FROM user_syncs
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired user syncs: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

// DeleteStale removes sync records that haven't been used in the specified duration
func (s *UserSyncStore) DeleteStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	if s.db == nil {
		return 0, nil // Database disabled, skip
	}

	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		DELETE FROM user_syncs
		WHERE updated_at < $1
	`

	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete stale user syncs: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
