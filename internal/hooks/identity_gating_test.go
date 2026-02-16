package hooks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestIdentityGatingHook_NoUser(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:   "test-request-123",
		User: nil, // No user object
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIdentityGatingHook_NoEIDs_SkipsSetUserID(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "",
			Ext: json.RawMessage(`{}`), // No eids
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user.id should remain empty (no eids to extract from)
	if req.User.ID != "" {
		t.Errorf("expected user.id to remain empty, got: %s", req.User.ID)
	}
}

func TestIdentityGatingHook_SetsUserIDFromEIDs(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	// Create user.ext.eids with Rubicon UID
	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "",
			Ext: userExtBytes,
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user.id should be set from eids
	if req.User.ID != "rubicon-uid-123" {
		t.Errorf("expected user.id to be 'rubicon-uid-123', got: %s", req.User.ID)
	}
}

func TestIdentityGatingHook_BidderSourceMapping(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	tests := []struct {
		bidderName   string
		expectedSource string
		eidSource    string
		eidUID       string
	}{
		{
			bidderName:     "rubicon",
			expectedSource: "rubiconproject.com",
			eidSource:      "rubiconproject.com",
			eidUID:         "rubicon-uid-123",
		},
		{
			bidderName:     "pubmatic",
			expectedSource: "pubmatic.com",
			eidSource:      "pubmatic.com",
			eidUID:         "pubmatic-uid-456",
		},
		{
			bidderName:     "kargo",
			expectedSource: "kargo.com",
			eidSource:      "kargo.com",
			eidUID:         "kargo-uid-789",
		},
		{
			bidderName:     "sovrn",
			expectedSource: "lijit.com",
			eidSource:      "lijit.com",
			eidUID:         "lijit-uid-abc",
		},
		{
			bidderName:     "triplelift",
			expectedSource: "triplelift.com",
			eidSource:      "triplelift.com",
			eidUID:         "triplelift-uid-def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.bidderName, func(t *testing.T) {
			// Verify source mapping
			source := hook.getBidderEIDSource(tt.bidderName)
			if source != tt.expectedSource {
				t.Errorf("expected source '%s', got '%s'", tt.expectedSource, source)
			}

			// Create request with matching EID
			userExt := map[string]interface{}{
				"eids": []interface{}{
					map[string]interface{}{
						"source": tt.eidSource,
						"uids": []interface{}{
							map[string]interface{}{"id": tt.eidUID},
						},
					},
				},
			}
			userExtBytes, _ := json.Marshal(userExt)

			req := &openrtb.BidRequest{
				ID: "test-request-123",
				User: &openrtb.User{
					ID:  "",
					Ext: userExtBytes,
				},
			}

			err := hook.ProcessBidderRequest(ctx, req, tt.bidderName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// user.id should be set from matching EID
			if req.User.ID != tt.eidUID {
				t.Errorf("expected user.id to be '%s', got: %s", tt.eidUID, req.User.ID)
			}
		})
	}
}

func TestIdentityGatingHook_WrongSource_SkipsSetUserID(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	// Create user.ext.eids with Kargo UID
	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "kargo.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "kargo-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "",
			Ext: userExtBytes,
		},
	}

	// Call hook for Rubicon (which expects rubiconproject.com, not kargo.com)
	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user.id should remain empty (wrong source)
	if req.User.ID != "" {
		t.Errorf("expected user.id to remain empty (wrong source), got: %s", req.User.ID)
	}
}

func TestIdentityGatingHook_UnknownBidder_SkipsSetUserID(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "unknown-source.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "unknown-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "",
			Ext: userExtBytes,
		},
	}

	// Call hook for unknown bidder
	err := hook.ProcessBidderRequest(ctx, req, "unknown-bidder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user.id should remain empty (unknown bidder has no source mapping)
	if req.User.ID != "" {
		t.Errorf("expected user.id to remain empty (unknown bidder), got: %s", req.User.ID)
	}
}

func TestIdentityGatingHook_MultipleEIDs_PicksCorrectOne(t *testing.T) {
	hook := NewIdentityGatingHook()
	ctx := context.Background()

	// Create user.ext.eids with multiple sources
	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "kargo.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "kargo-uid-123"},
				},
			},
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-456"},
				},
			},
			map[string]interface{}{
				"source": "pubmatic.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "pubmatic-uid-789"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "",
			Ext: userExtBytes,
		},
	}

	// Call hook for Rubicon
	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pick Rubicon UID, not Kargo or PubMatic
	if req.User.ID != "rubicon-uid-456" {
		t.Errorf("expected user.id to be 'rubicon-uid-456', got: %s", req.User.ID)
	}
}
