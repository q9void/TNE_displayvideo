package hooks

import (
	"context"
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// IdentityGatingHook controls EID→user.id mapping per bidder
// Executes per-bidder, AFTER privacy hook, BEFORE adapter
// Only sets user.id if consent permits AND bidder-specific EID exists
type IdentityGatingHook struct{}

// NewIdentityGatingHook creates a new identity gating hook
func NewIdentityGatingHook() *IdentityGatingHook {
	return &IdentityGatingHook{}
}

// ProcessBidderRequest sets user.id from eids if consent permits
func (h *IdentityGatingHook) ProcessBidderRequest(ctx context.Context, req *openrtb.BidRequest, bidderName string) error {
	// Skip if no user object
	if req.User == nil {
		return nil
	}

	// Check if consent allows ID usage (Privacy hook may have stripped eids)
	if !h.hasConsentForIDs(req) {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("request_id", req.ID).
			Msg("No consent for IDs - skipping SetUserID")
		return nil
	}

	// Map bidder name to EID source
	source := h.getBidderEIDSource(bidderName)
	if source == "" {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Msg("No EID source mapping for bidder - skipping SetUserID")
		return nil
	}

	// Extract UID from user.ext.eids and set user.id
	// This uses the helper from internal/adapters/userids.go
	originalUserID := req.User.ID
	req.User = adapters.SetUserID(req.User, source)

	// Log if UID was found and set
	if req.User.ID != "" && req.User.ID != originalUserID {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("source", source).
			Str("user_id", req.User.ID).
			Str("request_id", req.ID).
			Msg("✓ Set bidder user ID from eids")
	} else if req.User.ID == "" {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("source", source).
			Str("request_id", req.ID).
			Msg("No UID found for bidder source in eids")
	}

	return nil
}

// hasConsentForIDs checks if user.ext.eids still exists (Privacy hook preserves it if consent OK)
func (h *IdentityGatingHook) hasConsentForIDs(req *openrtb.BidRequest) bool {
	if req.User == nil || len(req.User.Ext) == 0 {
		return false
	}

	var userExt struct {
		EIDs []interface{} `json:"eids"`
	}
	if err := json.Unmarshal(req.User.Ext, &userExt); err != nil {
		return false
	}

	// If eids array exists and is not empty, we have consent
	return len(userExt.EIDs) > 0
}

// getBidderEIDSource maps bidder codes to their EID source domains
// This is the authoritative mapping of which UID source each bidder expects
func (h *IdentityGatingHook) getBidderEIDSource(bidderName string) string {
	// Map bidder codes to EID sources (source domains in user.ext.eids)
	// See: https://github.com/prebid/Prebid.js/blob/master/modules/userId/eids.js
	sources := map[string]string{
		"rubicon":    "rubiconproject.com",
		"pubmatic":   "pubmatic.com",
		"kargo":      "kargo.com",
		"sovrn":      "lijit.com",
		"triplelift": "triplelift.com",
		"oms":        "openmediation.com",
		"aniview":    "aniview.com",
		"appnexus":   "adnxs.com",
		"ix":         "ixiaa.com", // Index Exchange
		"openx":      "openx.net",
		"criteo":     "criteo.com",
		"ttd":        "uidapi.com", // The Trade Desk Unified ID 2.0
		"liveramp":   "idl-env",    // LiveRamp IdentityLink
		"id5":        "id5-sync.com",
		"uid2":       "uidapi.com", // Unified ID 2.0
		"sharedid":   "sharedid.org",
		"pubcommon":  "pubcid.org",
	}

	return sources[bidderName]
}
