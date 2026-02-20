package exchange

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// Test multiformat processor creation
func TestNewMultiformatProcessor(t *testing.T) {
	tests := []struct {
		name           string
		config         *MultiformatConfig
		expectEnabled  bool
		expectStrategy string
	}{
		{
			name:           "nil config uses defaults",
			config:         nil,
			expectEnabled:  true,
			expectStrategy: "server",
		},
		{
			name: "custom config",
			config: &MultiformatConfig{
				Enabled:         false,
				DefaultStrategy: "preferDeal",
			},
			expectEnabled:  false,
			expectStrategy: "preferDeal",
		},
		{
			name: "preferMediaType strategy",
			config: &MultiformatConfig{
				Enabled:         true,
				DefaultStrategy: "preferMediaType",
			},
			expectEnabled:  true,
			expectStrategy: "preferMediaType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultiformatProcessor(tt.config)
			if processor == nil {
				t.Fatal("expected non-nil processor")
			}
			if processor.config.Enabled != tt.expectEnabled {
				t.Errorf("expected Enabled=%v, got %v", tt.expectEnabled, processor.config.Enabled)
			}
			if processor.config.DefaultStrategy != tt.expectStrategy {
				t.Errorf("expected DefaultStrategy=%s, got %s", tt.expectStrategy, processor.config.DefaultStrategy)
			}
		})
	}
}

func TestDefaultMultiformatConfig(t *testing.T) {
	config := DefaultMultiformatConfig()
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if !config.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if config.DefaultStrategy != "server" {
		t.Errorf("expected DefaultStrategy=server, got %s", config.DefaultStrategy)
	}
}

// Test BidCandidate creation
func TestNewBidCandidate(t *testing.T) {
	tests := []struct {
		name          string
		bid           *openrtb.Bid
		mediaType     string
		bidderName    string
		expectCPM     float64
		expectHasDeal bool
	}{
		{
			name: "bid with deal",
			bid: &openrtb.Bid{
				ID:     "bid1",
				ImpID:  "imp1",
				Price:  2.5,
				DealID: "deal123",
			},
			mediaType:     "banner",
			bidderName:    "bidder1",
			expectCPM:     2.5,
			expectHasDeal: true,
		},
		{
			name: "bid without deal",
			bid: &openrtb.Bid{
				ID:     "bid2",
				ImpID:  "imp1",
				Price:  1.8,
				DealID: "",
			},
			mediaType:     "video",
			bidderName:    "bidder2",
			expectCPM:     1.8,
			expectHasDeal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := NewBidCandidate(tt.bid, tt.mediaType, tt.bidderName)
			if candidate == nil {
				t.Fatal("expected non-nil candidate")
			}
			if candidate.Bid != tt.bid {
				t.Error("bid reference mismatch")
			}
			if candidate.MediaType != tt.mediaType {
				t.Errorf("expected MediaType=%s, got %s", tt.mediaType, candidate.MediaType)
			}
			if candidate.BidderName != tt.bidderName {
				t.Errorf("expected BidderName=%s, got %s", tt.bidderName, candidate.BidderName)
			}
			if candidate.CPM != tt.expectCPM {
				t.Errorf("expected CPM=%f, got %f", tt.expectCPM, candidate.CPM)
			}
			if candidate.HasDeal != tt.expectHasDeal {
				t.Errorf("expected HasDeal=%v, got %v", tt.expectHasDeal, candidate.HasDeal)
			}
		})
	}
}

// Test IsMultiformat detection
func TestIsMultiformat(t *testing.T) {
	processor := NewMultiformatProcessor(nil)

	tests := []struct {
		name            string
		imp             *openrtb.Imp
		expectMultiformat bool
	}{
		{
			name: "banner only - not multiformat",
			imp: &openrtb.Imp{
				ID:     "imp1",
				Banner: &openrtb.Banner{W: 300, H: 250},
			},
			expectMultiformat: false,
		},
		{
			name: "video only - not multiformat",
			imp: &openrtb.Imp{
				ID:    "imp2",
				Video: &openrtb.Video{W: 640, H: 480},
			},
			expectMultiformat: false,
		},
		{
			name: "native only - not multiformat",
			imp: &openrtb.Imp{
				ID:     "imp3",
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMultiformat: false,
		},
		{
			name: "banner and video - multiformat",
			imp: &openrtb.Imp{
				ID:     "imp4",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Video:  &openrtb.Video{W: 640, H: 480},
			},
			expectMultiformat: true,
		},
		{
			name: "banner and native - multiformat",
			imp: &openrtb.Imp{
				ID:     "imp5",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMultiformat: true,
		},
		{
			name: "video and native - multiformat",
			imp: &openrtb.Imp{
				ID:     "imp6",
				Video:  &openrtb.Video{W: 640, H: 480},
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMultiformat: true,
		},
		{
			name: "all three formats - multiformat",
			imp: &openrtb.Imp{
				ID:     "imp7",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Video:  &openrtb.Video{W: 640, H: 480},
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMultiformat: true,
		},
		{
			name: "no formats - not multiformat",
			imp: &openrtb.Imp{
				ID: "imp8",
			},
			expectMultiformat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.IsMultiformat(tt.imp)
			if result != tt.expectMultiformat {
				t.Errorf("expected IsMultiformat=%v, got %v", tt.expectMultiformat, result)
			}
		})
	}
}

// Test GetPreferredMediaType
func TestGetPreferredMediaType(t *testing.T) {
	processor := NewMultiformatProcessor(nil)

	tests := []struct {
		name          string
		imp           *openrtb.Imp
		expectMediaType string
	}{
		{
			name: "video has highest priority",
			imp: &openrtb.Imp{
				ID:     "imp1",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Video:  &openrtb.Video{W: 640, H: 480},
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMediaType: "video",
		},
		{
			name: "native second priority when no video",
			imp: &openrtb.Imp{
				ID:     "imp2",
				Banner: &openrtb.Banner{W: 300, H: 250},
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMediaType: "native",
		},
		{
			name: "banner third priority when only banner",
			imp: &openrtb.Imp{
				ID:     "imp3",
				Banner: &openrtb.Banner{W: 300, H: 250},
			},
			expectMediaType: "banner",
		},
		{
			name: "video only",
			imp: &openrtb.Imp{
				ID:    "imp4",
				Video: &openrtb.Video{W: 640, H: 480},
			},
			expectMediaType: "video",
		},
		{
			name: "native only",
			imp: &openrtb.Imp{
				ID:     "imp5",
				Native: &openrtb.Native{Ver: "1.2"},
			},
			expectMediaType: "native",
		},
		{
			name: "no formats - empty string",
			imp: &openrtb.Imp{
				ID: "imp6",
			},
			expectMediaType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.GetPreferredMediaType(tt.imp)
			if result != tt.expectMediaType {
				t.Errorf("expected MediaType=%s, got %s", tt.expectMediaType, result)
			}
		})
	}
}

// Test SelectBestBid with server strategy
func TestSelectBestBid_ServerStrategy(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "server",
	})

	tests := []struct {
		name                string
		bids                []*BidCandidate
		preferredMediaType  string
		expectBidID         string
		expectNil           bool
	}{
		{
			name:                "no bids returns nil",
			bids:                []*BidCandidate{},
			preferredMediaType:  "banner",
			expectNil:           true,
		},
		{
			name: "single bid returns that bid",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid1",
		},
		{
			name: "deal bid wins over higher CPM",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 3.0},
					MediaType: "banner",
					CPM:       3.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.0, DealID: "deal123"},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   true,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid2",
		},
		{
			name: "preferred format wins when CPM within 5%",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 1.95},
					MediaType: "video",
					CPM:       1.95,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
		},
		{
			name: "highest CPM wins when not preferred and > 5% difference",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 1.8},
					MediaType: "video",
					CPM:       1.8,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid1",
		},
		{
			name: "preferred format requires 5% threshold to win",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.2},
					MediaType: "video",
					CPM:       2.2,
					HasDeal:   false,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid2",
		},
		{
			name: "highest CPM wins with no preference",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 1.5},
					MediaType: "banner",
					CPM:       1.5,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "video",
					CPM:       2.5,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid3", Price: 2.0},
					MediaType: "native",
					CPM:       2.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "",
			expectBidID:        "bid2",
		},
		{
			name: "all same format - highest CPM wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 1.5},
					MediaType: "banner",
					CPM:       1.5,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "banner",
					CPM:       2.5,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid3", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid2",
		},
		{
			name: "multiple deals - highest CPM deal wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 1.5, DealID: "deal1"},
					MediaType: "banner",
					CPM:       1.5,
					HasDeal:   true,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5, DealID: "deal2"},
					MediaType: "video",
					CPM:       2.5,
					HasDeal:   true,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid3", Price: 3.0},
					MediaType: "native",
					CPM:       3.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "native",
			expectBidID:        "bid2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &openrtb.Imp{ID: "imp1"}
			result := processor.SelectBestBid(imp, tt.bids, tt.preferredMediaType)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil result, got bid %s", result.Bid.ID)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Bid.ID != tt.expectBidID {
				t.Errorf("expected bid %s, got %s", tt.expectBidID, result.Bid.ID)
			}
		})
	}
}

// Test SelectBestBid with preferDeal strategy
func TestSelectBestBid_PreferDealStrategy(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "preferDeal",
	})

	tests := []struct {
		name               string
		bids               []*BidCandidate
		preferredMediaType string
		expectBidID        string
	}{
		{
			name: "deal wins regardless of format preference",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 3.0},
					MediaType: "video",
					CPM:       3.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 1.5, DealID: "deal123"},
					MediaType: "banner",
					CPM:       1.5,
					HasDeal:   true,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
		},
		{
			name: "highest CPM wins when no deals",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "video",
					CPM:       2.5,
					HasDeal:   false,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid2",
		},
		{
			name: "format preference ignored with preferDeal strategy",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 1.95},
					MediaType: "banner",
					CPM:       1.95,
					HasDeal:   false,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid1", // Higher CPM wins, format ignored
		},
		{
			name: "multiple deals - highest CPM deal wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 1.5, DealID: "deal1"},
					MediaType: "banner",
					CPM:       1.5,
					HasDeal:   true,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.0, DealID: "deal2"},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   true,
				},
			},
			preferredMediaType: "banner",
			expectBidID:        "bid2",
		},
		{
			name: "both deals with equal CPM - first wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0, DealID: "deal1"},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   true,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.0, DealID: "deal2"},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   true,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &openrtb.Imp{ID: "imp1"}
			result := processor.SelectBestBid(imp, tt.bids, tt.preferredMediaType)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Bid.ID != tt.expectBidID {
				t.Errorf("expected bid %s, got %s", tt.expectBidID, result.Bid.ID)
			}
		})
	}
}

// Test SelectBestBid with preferMediaType strategy
func TestSelectBestBid_PreferMediaTypeStrategy(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "preferMediaType",
	})

	tests := []struct {
		name               string
		bids               []*BidCandidate
		preferredMediaType string
		expectBidID        string
	}{
		{
			name: "preferred format wins over deal",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 3.0, DealID: "deal123"},
					MediaType: "banner",
					CPM:       3.0,
					HasDeal:   true,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.0},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
		},
		{
			name: "highest CPM preferred format wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "video",
					CPM:       2.5,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid3", Price: 3.0},
					MediaType: "banner",
					CPM:       3.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
		},
		{
			name: "falls back to highest CPM when no preferred format match",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "native",
					CPM:       2.5,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
		},
		{
			name: "no preference falls back to server strategy",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.5},
					MediaType: "video",
					CPM:       2.5,
					HasDeal:   false,
				},
			},
			preferredMediaType: "",
			expectBidID:        "bid2",
		},
		{
			name: "lower CPM preferred format still wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 1.0},
					MediaType: "native",
					CPM:       1.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 3.0},
					MediaType: "banner",
					CPM:       3.0,
					HasDeal:   false,
				},
			},
			preferredMediaType: "native",
			expectBidID:        "bid1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &openrtb.Imp{ID: "imp1"}
			result := processor.SelectBestBid(imp, tt.bids, tt.preferredMediaType)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Bid.ID != tt.expectBidID {
				t.Errorf("expected bid %s, got %s", tt.expectBidID, result.Bid.ID)
			}
		})
	}
}

// Test SelectBestBid when disabled
func TestSelectBestBid_Disabled(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         false,
		DefaultStrategy: "server",
	})

	bids := []*BidCandidate{
		{
			Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
			MediaType: "banner",
			CPM:       2.0,
		},
	}

	imp := &openrtb.Imp{ID: "imp1"}
	result := processor.SelectBestBid(imp, bids, "banner")
	if result != nil {
		t.Error("expected nil result when processor disabled")
	}
}

// Test edge case: CPM exactly at 5% threshold
func TestSelectBestBid_ExactThreshold(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "server",
	})

	tests := []struct {
		name               string
		bids               []*BidCandidate
		preferredMediaType string
		expectBidID        string
		description        string
	}{
		{
			name: "preferred format at exact 95% threshold wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "banner",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 1.9}, // Exactly 95%
					MediaType: "video",
					CPM:       1.9,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
			description:        "1.9 is exactly 95% of 2.0, should win",
		},
		{
			name: "non-preferred at exact 105% threshold wins",
			bids: []*BidCandidate{
				{
					Bid:       &openrtb.Bid{ID: "bid1", Price: 2.0},
					MediaType: "video",
					CPM:       2.0,
					HasDeal:   false,
				},
				{
					Bid:       &openrtb.Bid{ID: "bid2", Price: 2.1}, // Exactly 105%
					MediaType: "banner",
					CPM:       2.1,
					HasDeal:   false,
				},
			},
			preferredMediaType: "video",
			expectBidID:        "bid2",
			description:        "2.1 is exactly 105% of 2.0, should win",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &openrtb.Imp{ID: "imp1"}
			result := processor.SelectBestBid(imp, tt.bids, tt.preferredMediaType)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Bid.ID != tt.expectBidID {
				t.Errorf("%s: expected bid %s, got %s", tt.description, tt.expectBidID, result.Bid.ID)
			}
		})
	}
}

// Test getStrategy method
func TestGetStrategy(t *testing.T) {
	tests := []struct {
		name            string
		defaultStrategy string
		expectStrategy  string
	}{
		{
			name:            "server strategy",
			defaultStrategy: "server",
			expectStrategy:  "server",
		},
		{
			name:            "preferDeal strategy",
			defaultStrategy: "preferDeal",
			expectStrategy:  "preferDeal",
		},
		{
			name:            "preferMediaType strategy",
			defaultStrategy: "preferMediaType",
			expectStrategy:  "preferMediaType",
		},
		{
			name:            "unknown strategy defaults to server",
			defaultStrategy: "unknown",
			expectStrategy:  "unknown", // getStrategy returns as-is, SelectBestBid handles default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultiformatProcessor(&MultiformatConfig{
				Enabled:         true,
				DefaultStrategy: tt.defaultStrategy,
			})
			imp := &openrtb.Imp{ID: "imp1"}
			result := processor.getStrategy(imp, "")
			if result != tt.expectStrategy {
				t.Errorf("expected strategy %s, got %s", tt.expectStrategy, result)
			}
		})
	}
}

// Test complex scenario with multiple bids
func TestSelectBestBid_ComplexScenario(t *testing.T) {
	processor := NewMultiformatProcessor(&MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "server",
	})

	bids := []*BidCandidate{
		{
			Bid:        &openrtb.Bid{ID: "bid1", Price: 1.8},
			MediaType:  "banner",
			CPM:        1.8,
			HasDeal:    false,
			BidderName: "bidder1",
		},
		{
			Bid:        &openrtb.Bid{ID: "bid2", Price: 2.0},
			MediaType:  "video",
			CPM:        2.0,
			HasDeal:    false,
			BidderName: "bidder2",
		},
		{
			Bid:        &openrtb.Bid{ID: "bid3", Price: 1.95},
			MediaType:  "native",
			CPM:        1.95,
			HasDeal:    false,
			BidderName: "bidder3",
		},
		{
			Bid:        &openrtb.Bid{ID: "bid4", Price: 1.5, DealID: "deal123"},
			MediaType:  "banner",
			CPM:        1.5,
			HasDeal:    true,
			BidderName: "bidder4",
		},
		{
			Bid:        &openrtb.Bid{ID: "bid5", Price: 1.92},
			MediaType:  "native",
			CPM:        1.92,
			HasDeal:    false,
			BidderName: "bidder5",
		},
	}

	imp := &openrtb.Imp{ID: "imp1"}

	// With server strategy, deal should win
	result := processor.SelectBestBid(imp, bids, "video")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Bid.ID != "bid4" {
		t.Errorf("expected deal bid4 to win, got %s", result.Bid.ID)
	}
}
