package hooks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestMultiformatHook_SingleMediaType_NoFiltering(t *testing.T) {
	hook := NewMultiformatHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{
				ID:     "imp-1",
				Banner: &openrtb.Banner{W: 300, H: 250}, // Only banner
			},
		},
	}

	// No responses yet (tested in isolation)
	responses := []*BidderResponse{}

	err := hook.ProcessAuction(ctx, req, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultiformatHook_NoMultiformatConfig_NoFiltering(t *testing.T) {
	hook := NewMultiformatHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{
				ID:     "imp-1",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Video:  &openrtb.Video{}, // Multiple media types but no config
			},
		},
	}

	responses := []*BidderResponse{}

	err := hook.ProcessAuction(ctx, req, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultiformatHook_WithPreferredMediaType(t *testing.T) {
	hook := NewMultiformatHook()
	ctx := context.Background()

	// Create imp.ext with preferred_media_type
	impExt := map[string]interface{}{
		"prebid": map[string]interface{}{
			"preferred_media_type": "banner",
		},
	}
	impExtBytes, _ := json.Marshal(impExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{
				ID:     "imp-1",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Video:  &openrtb.Video{},
				Ext:    impExtBytes,
			},
		},
	}

	// Create bidder responses with different media types
	bannerBid := openrtb.Bid{
		ID:    "bid-banner",
		ImpID: "imp-1",
		Price: 1.50,
		AdM:   "<banner>",
		W:     300,
		H:     250,
	}

	videoBid := openrtb.Bid{
		ID:    "bid-video",
		ImpID: "imp-1",
		Price: 2.00, // Higher price but not preferred type
		AdM:   "<VAST>",
	}

	responses := []*BidderResponse{
		{
			BidderName: "rubicon",
			Response: &openrtb.BidResponse{
				ID:  "test-request-123",
				Cur: "USD",
				SeatBid: []openrtb.SeatBid{
					{
						Bid: []openrtb.Bid{bannerBid, videoBid},
					},
				},
			},
		},
	}

	err := hook.ProcessAuction(ctx, req, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Note: Actual filtering logic depends on GetBidTypeFromMap which needs full context
	// This test verifies the hook executes without error
}

func TestMultiformatHook_ExtractsMultiformatConfig(t *testing.T) {
	hook := NewMultiformatHook()

	tests := []struct {
		name                   string
		impExt                 map[string]interface{}
		expectedStrategy       string
		expectedPreferredType  string
	}{
		{
			name: "preferred_media_type only",
			impExt: map[string]interface{}{
				"prebid": map[string]interface{}{
					"preferred_media_type": "video",
				},
			},
			expectedStrategy:      "",
			expectedPreferredType: "video",
		},
		{
			name: "multiformat_request_strategy only",
			impExt: map[string]interface{}{
				"prebid": map[string]interface{}{
					"multiformat_request_strategy": "server_price_priority",
				},
			},
			expectedStrategy:      "server_price_priority",
			expectedPreferredType: "",
		},
		{
			name: "both strategy and preferred type",
			impExt: map[string]interface{}{
				"prebid": map[string]interface{}{
					"multiformat_request_strategy": "preferred_media_type_first",
					"preferred_media_type":         "banner",
				},
			},
			expectedStrategy:      "preferred_media_type_first",
			expectedPreferredType: "banner",
		},
		{
			name:                  "no config",
			impExt:                map[string]interface{}{},
			expectedStrategy:      "",
			expectedPreferredType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impExtBytes, _ := json.Marshal(tt.impExt)
			imp := openrtb.Imp{
				ID:  "imp-1",
				Ext: impExtBytes,
			}

			strategy, preferredType := hook.getMultiformatConfig(imp)

			if strategy != tt.expectedStrategy {
				t.Errorf("expected strategy '%s', got '%s'", tt.expectedStrategy, strategy)
			}
			if preferredType != tt.expectedPreferredType {
				t.Errorf("expected preferred type '%s', got '%s'", tt.expectedPreferredType, preferredType)
			}
		})
	}
}

func TestMultiformatHook_NilResponses(t *testing.T) {
	hook := NewMultiformatHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
		},
	}

	err := hook.ProcessAuction(ctx, req, nil)
	if err != nil {
		t.Fatalf("unexpected error for nil responses: %v", err)
	}
}
