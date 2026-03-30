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
