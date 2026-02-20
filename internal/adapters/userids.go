package adapters

import (
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// ExtractUIDFromEids extracts a bidder's user ID from user.ext.eids
// by matching the source domain (e.g., "rubiconproject.com" for Rubicon)
//
// user.ext.eids format (OpenRTB Extended Identifiers):
// {
//   "eids": [
//     {
//       "source": "rubiconproject.com",
//       "uids": [{"id": "abc123", "atype": 1}]
//     },
//     {
//       "source": "pubmatic.com",
//       "uids": [{"id": "xyz789", "atype": 1}]
//     }
//   ]
// }
func ExtractUIDFromEids(user *openrtb.User, source string) string {
	if user == nil || len(user.Ext) == 0 {
		return ""
	}

	// Parse user.ext
	var userExt map[string]interface{}
	if err := json.Unmarshal(user.Ext, &userExt); err != nil {
		return ""
	}

	// Get eids array
	eidsRaw, ok := userExt["eids"]
	if !ok {
		return ""
	}

	eids, ok := eidsRaw.([]interface{})
	if !ok {
		return ""
	}

	// Find matching source
	for _, eidRaw := range eids {
		eid, ok := eidRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if source matches
		eidSource, ok := eid["source"].(string)
		if !ok || eidSource != source {
			continue
		}

		// Extract UID from uids array
		uidsRaw, ok := eid["uids"]
		if !ok {
			continue
		}

		uids, ok := uidsRaw.([]interface{})
		if !ok || len(uids) == 0 {
			continue
		}

		firstUID, ok := uids[0].(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := firstUID["id"].(string)
		if !ok || id == "" {
			continue
		}

		return id
	}

	return ""
}

// SetUserID sets user.id on the request, extracting from eids if needed
// Falls back to existing user.id if no UID found in eids
func SetUserID(user *openrtb.User, source string) *openrtb.User {
	if user == nil {
		return user
	}

	// Try to extract UID from eids
	uid := ExtractUIDFromEids(user, source)
	if uid != "" {
		// Make a copy to avoid modifying original
		userCopy := *user
		userCopy.ID = uid
		return &userCopy
	}

	// Return original if no UID found
	return user
}
