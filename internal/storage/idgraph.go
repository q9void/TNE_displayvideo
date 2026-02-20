package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// IDGraphStore manages first-party ID to bidder UID mappings
type IDGraphStore struct {
	db *sql.DB
}

// MappingMetadata contains privacy and fraud detection metadata
type MappingMetadata struct {
	ConsentGiven bool
	GDPRApplies  bool
	IPAddress    string
	UserAgent    string
}

// NewIDGraphStore creates a new ID graph store
func NewIDGraphStore(db *sql.DB) *IDGraphStore {
	return &IDGraphStore{db: db}
}

// RecordMapping stores or updates a FPID→bidder UID mapping
func (s *IDGraphStore) RecordMapping(ctx context.Context, fpid, bidderCode, bidderUID string, metadata *MappingMetadata) error {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	// Hash IP and UA for privacy compliance
	ipHash := hashString(metadata.IPAddress)
	uaHash := hashString(metadata.UserAgent)

	query := `
		INSERT INTO id_graph_mappings
			(fpid, bidder_code, bidder_uid, consent_given, gdpr_applies, ip_hash, user_agent_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (fpid, bidder_code)
		DO UPDATE SET
			bidder_uid = EXCLUDED.bidder_uid,
			last_seen = NOW(),
			sync_count = id_graph_mappings.sync_count + 1,
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query, fpid, bidderCode, bidderUID,
		metadata.ConsentGiven, metadata.GDPRApplies, ipHash, uaHash)

	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("fpid", fpid).
			Str("bidder", bidderCode).
			Msg("Failed to record ID graph mapping")
		return err
	}

	logger.Log.Debug().
		Str("fpid", fpid).
		Str("bidder", bidderCode).
		Str("bidder_uid", bidderUID).
		Msg("Recorded ID graph mapping")

	return nil
}

// GetMappings retrieves all bidder UIDs for a given FPID
func (s *IDGraphStore) GetMappings(ctx context.Context, fpid string) (map[string]string, error) {
	ctx, cancel := withTimeout(ctx, DefaultDBTimeout)
	defer cancel()

	query := `
		SELECT bidder_code, bidder_uid
		FROM id_graph_mappings
		WHERE fpid = $1
		  AND last_seen > NOW() - INTERVAL '90 days'
	`

	rows, err := s.db.QueryContext(ctx, query, fpid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mappings := make(map[string]string)
	for rows.Next() {
		var bidderCode, bidderUID string
		if err := rows.Scan(&bidderCode, &bidderUID); err != nil {
			return nil, err
		}
		mappings[bidderCode] = bidderUID
	}

	return mappings, rows.Err()
}

// hashString creates SHA256 hash of input (for privacy-preserving storage)
func hashString(s string) string {
	if s == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
