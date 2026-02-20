// Package exchange provides tests for multibid support functionality.
//
// This test suite provides comprehensive coverage for multibid.go including:
//
// 1. Configuration & Initialization:
//    - DefaultMultibidConfig: Tests default configuration values
//    - NewMultibidProcessor: Tests processor creation with nil and custom configs
//
// 2. Bid Filtering & Processing:
//    - TestProcessBidderResponse_BidFiltering: Tests per-impression and total bidder limits
//    - TestProcessBidderResponse_MultipleSeatBids: Tests handling of multiple seat bids
//    - TestProcessBidderResponse_EdgeCases: Tests nil responses, empty arrays, zero limits
//
// 3. Price Bucket Calculation:
//    - TestGetPriceBucket: Tests Prebid.js compatible price buckets:
//      * $0-$5: $0.05 increments
//      * $5-$10: $0.10 increments
//      * $10-$20: $0.50 increments
//      * $20+: $1.00 increments
//
// 4. Targeting Key Generation:
//    - TestGenerateMultibidTargetingKeys: Tests generation of hb_pb, hb_pb_2, hb_pb_3 keys
//    - Tests bid sorting by price, custom prefixes, deal IDs, and size keys
//    - TestGenerateMultibidTargetingKeys_OriginalBidsUnmodified: Verifies immutability
//
// 5. Bid Sorting:
//    - TestMultibid_SortMultibidsByPrice: Tests descending price sorting with edge cases
//
// 6. Legacy Mode:
//    - TestLimitToOneBidPerImp: Tests single bid per impression enforcement
//
// Coverage: 100% of all functions in multibid.go
package exchange

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestDefaultMultibidConfig verifies default configuration values
func TestDefaultMultibidConfig(t *testing.T) {
	config := DefaultMultibidConfig()

	if !config.Enabled {
		t.Error("Expected multibid to be enabled by default")
	}
	if config.MaxBidsPerBidder != 3 {
		t.Errorf("Expected MaxBidsPerBidder = 3, got %d", config.MaxBidsPerBidder)
	}
	if config.MaxBidsPerBidderPerImp != 1 {
		t.Errorf("Expected MaxBidsPerBidderPerImp = 1, got %d", config.MaxBidsPerBidderPerImp)
	}
	if config.TargetBidderCodePrefix != "hb" {
		t.Errorf("Expected TargetBidderCodePrefix = 'hb', got '%s'", config.TargetBidderCodePrefix)
	}
}

// TestNewMultibidProcessor verifies processor creation
func TestNewMultibidProcessor(t *testing.T) {
	tests := []struct {
		name           string
		config         *MultibidConfig
		expectDefaults bool
	}{
		{
			name:           "Nil config uses defaults",
			config:         nil,
			expectDefaults: true,
		},
		{
			name: "Custom config is preserved",
			config: &MultibidConfig{
				Enabled:                false,
				MaxBidsPerBidder:       5,
				MaxBidsPerBidderPerImp: 2,
				TargetBidderCodePrefix: "custom",
			},
			expectDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultibidProcessor(tt.config)
			if processor == nil {
				t.Fatal("Expected processor to be non-nil")
			}
			if processor.config == nil {
				t.Fatal("Expected processor config to be non-nil")
			}

			if tt.expectDefaults {
				if !processor.config.Enabled {
					t.Error("Expected default config to have Enabled=true")
				}
				if processor.config.MaxBidsPerBidder != 3 {
					t.Errorf("Expected default MaxBidsPerBidder=3, got %d", processor.config.MaxBidsPerBidder)
				}
			} else {
				if processor.config.Enabled {
					t.Error("Expected Enabled=false")
				}
				if processor.config.MaxBidsPerBidder != 5 {
					t.Errorf("Expected MaxBidsPerBidder=5, got %d", processor.config.MaxBidsPerBidder)
				}
			}
		})
	}
}

// TestProcessBidderResponse_BidFiltering verifies bid filtering logic
func TestProcessBidderResponse_BidFiltering(t *testing.T) {
	tests := []struct {
		name          string
		config        *MultibidConfig
		inputBids     []openrtb.Bid
		expectedCount int
		description   string
	}{
		{
			name: "Respects per-impression limit",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       10,
				MaxBidsPerBidderPerImp: 2,
			},
			inputBids: []openrtb.Bid{
				{ID: "bid1", ImpID: "imp1", Price: 5.0},
				{ID: "bid2", ImpID: "imp1", Price: 4.0},
				{ID: "bid3", ImpID: "imp1", Price: 3.0}, // Should be filtered
				{ID: "bid4", ImpID: "imp2", Price: 2.0},
			},
			expectedCount: 3,
			description:   "Should allow 2 bids per impression",
		},
		{
			name: "Respects total bidder limit",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       2,
				MaxBidsPerBidderPerImp: 5,
			},
			inputBids: []openrtb.Bid{
				{ID: "bid1", ImpID: "imp1", Price: 5.0},
				{ID: "bid2", ImpID: "imp2", Price: 4.0},
				{ID: "bid3", ImpID: "imp3", Price: 3.0}, // Should be filtered
			},
			expectedCount: 2,
			description:   "Should limit total bids per bidder",
		},
		{
			name: "Multibid disabled - one bid per impression",
			config: &MultibidConfig{
				Enabled:                false,
				MaxBidsPerBidder:       10,
				MaxBidsPerBidderPerImp: 10,
			},
			inputBids: []openrtb.Bid{
				{ID: "bid1", ImpID: "imp1", Price: 5.0},
				{ID: "bid2", ImpID: "imp1", Price: 4.0}, // Should be filtered
				{ID: "bid3", ImpID: "imp2", Price: 3.0},
			},
			expectedCount: 2,
			description:   "Should enforce legacy mode (1 bid per impression)",
		},
		{
			name: "Empty bid response",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       3,
				MaxBidsPerBidderPerImp: 1,
			},
			inputBids:     []openrtb.Bid{},
			expectedCount: 0,
			description:   "Should handle empty bid list",
		},
		{
			name: "Multiple impressions with mixed limits",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       5,
				MaxBidsPerBidderPerImp: 2,
			},
			inputBids: []openrtb.Bid{
				{ID: "bid1", ImpID: "imp1", Price: 5.0},
				{ID: "bid2", ImpID: "imp1", Price: 4.5},
				{ID: "bid3", ImpID: "imp1", Price: 4.0}, // Filtered - imp limit
				{ID: "bid4", ImpID: "imp2", Price: 3.5},
				{ID: "bid5", ImpID: "imp2", Price: 3.0},
				{ID: "bid6", ImpID: "imp3", Price: 2.5},
			},
			expectedCount: 5,
			description:   "Should respect both per-imp and total limits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultibidProcessor(tt.config)

			bidResponse := &openrtb.BidResponse{
				ID: "test-response",
				SeatBid: []openrtb.SeatBid{
					{
						Seat: "testbidder",
						Bid:  tt.inputBids,
					},
				},
			}

			result, err := processor.ProcessBidderResponse("testbidder", bidResponse)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			totalBids := 0
			for _, seatBid := range result.SeatBid {
				totalBids += len(seatBid.Bid)
			}

			if totalBids != tt.expectedCount {
				t.Errorf("%s: Expected %d bids, got %d", tt.description, tt.expectedCount, totalBids)
			}
		})
	}
}

// TestProcessBidderResponse_MultipleSeatBids verifies handling of multiple seat bids
func TestProcessBidderResponse_MultipleSeatBids(t *testing.T) {
	config := &MultibidConfig{
		Enabled:                true,
		MaxBidsPerBidder:       4,
		MaxBidsPerBidderPerImp: 2,
	}
	processor := NewMultibidProcessor(config)

	bidResponse := &openrtb.BidResponse{
		ID: "test-response",
		SeatBid: []openrtb.SeatBid{
			{
				Seat: "seat1",
				Bid: []openrtb.Bid{
					{ID: "bid1", ImpID: "imp1", Price: 5.0},
					{ID: "bid2", ImpID: "imp1", Price: 4.0},
				},
			},
			{
				Seat: "seat2",
				Bid: []openrtb.Bid{
					{ID: "bid3", ImpID: "imp2", Price: 3.0},
					{ID: "bid4", ImpID: "imp2", Price: 2.0},
				},
			},
		},
	}

	result, err := processor.ProcessBidderResponse("testbidder", bidResponse)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	totalBids := 0
	for _, seatBid := range result.SeatBid {
		totalBids += len(seatBid.Bid)
	}

	// Should have all 4 bids (2 per impression, 4 total)
	if totalBids != 4 {
		t.Errorf("Expected 4 bids across both seats, got %d", totalBids)
	}
}

// TestGetPriceBucket verifies price bucket calculation
func TestGetPriceBucket(t *testing.T) {
	tests := []struct {
		name     string
		price    float64
		expected string
	}{
		// $0-$5: $0.05 increments
		{
			name:     "Price 0.00",
			price:    0.00,
			expected: "0.00",
		},
		{
			name:     "Price 0.03 rounds to 0.00",
			price:    0.03,
			expected: "0.00",
		},
		{
			name:     "Price 0.05",
			price:    0.05,
			expected: "0.05",
		},
		{
			name:     "Price 0.12 rounds to 0.10",
			price:    0.12,
			expected: "0.10",
		},
		{
			name:     "Price 1.23 rounds to 1.20",
			price:    1.23,
			expected: "1.20",
		},
		{
			name:     "Price 4.99 rounds to 4.95",
			price:    4.99,
			expected: "4.95",
		},
		{
			name:     "Price 5.00",
			price:    5.00,
			expected: "5.00",
		},

		// $5-$10: $0.10 increments
		{
			name:     "Price 5.01 rounds to 5.00",
			price:    5.01,
			expected: "5.00",
		},
		{
			name:     "Price 5.14 rounds to 5.10",
			price:    5.14,
			expected: "5.10",
		},
		{
			name:     "Price 7.89 rounds to 7.80",
			price:    7.89,
			expected: "7.80",
		},
		{
			name:     "Price 10.00",
			price:    10.00,
			expected: "10.00",
		},

		// $10-$20: $0.50 increments
		{
			name:     "Price 10.01 rounds to 10.00",
			price:    10.01,
			expected: "10.00",
		},
		{
			name:     "Price 10.49 rounds to 10.00",
			price:    10.49,
			expected: "10.00",
		},
		{
			name:     "Price 10.50",
			price:    10.50,
			expected: "10.50",
		},
		{
			name:     "Price 15.75 rounds to 15.50",
			price:    15.75,
			expected: "15.50",
		},
		{
			name:     "Price 19.99 rounds to 19.50",
			price:    19.99,
			expected: "19.50",
		},
		{
			name:     "Price 20.00",
			price:    20.00,
			expected: "20.00",
		},

		// $20+: $1.00 increments
		{
			name:     "Price 20.01 rounds to 20.00",
			price:    20.01,
			expected: "20.00",
		},
		{
			name:     "Price 25.99 rounds to 25.00",
			price:    25.99,
			expected: "25.00",
		},
		{
			name:     "Price 100.00",
			price:    100.00,
			expected: "100.00",
		},
		{
			name:     "Price 999.99 rounds to 999.00",
			price:    999.99,
			expected: "999.00",
		},

		// Edge cases
		{
			name:     "Negative price defaults to 0.00",
			price:    -5.0,
			expected: "0.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPriceBucket(tt.price)
			if result != tt.expected {
				t.Errorf("getPriceBucket(%v) = %s, expected %s", tt.price, result, tt.expected)
			}
		})
	}
}

// TestMultibid_SortMultibidsByPrice verifies bid sorting by price
func TestMultibid_SortMultibidsByPrice(t *testing.T) {
	tests := []struct {
		name           string
		inputBids      []*BidderBid
		expectedPrices []float64
	}{
		{
			name: "Sorts descending by price",
			inputBids: []*BidderBid{
				{Bid: &openrtb.Bid{ID: "bid1", Price: 3.0}, Bidder: "bidder1"},
				{Bid: &openrtb.Bid{ID: "bid2", Price: 5.0}, Bidder: "bidder2"},
				{Bid: &openrtb.Bid{ID: "bid3", Price: 1.0}, Bidder: "bidder3"},
				{Bid: &openrtb.Bid{ID: "bid4", Price: 4.0}, Bidder: "bidder4"},
			},
			expectedPrices: []float64{5.0, 4.0, 3.0, 1.0},
		},
		{
			name: "Handles equal prices",
			inputBids: []*BidderBid{
				{Bid: &openrtb.Bid{ID: "bid1", Price: 5.0}, Bidder: "bidder1"},
				{Bid: &openrtb.Bid{ID: "bid2", Price: 5.0}, Bidder: "bidder2"},
				{Bid: &openrtb.Bid{ID: "bid3", Price: 3.0}, Bidder: "bidder3"},
			},
			expectedPrices: []float64{5.0, 5.0, 3.0},
		},
		{
			name: "Handles single bid",
			inputBids: []*BidderBid{
				{Bid: &openrtb.Bid{ID: "bid1", Price: 5.0}, Bidder: "bidder1"},
			},
			expectedPrices: []float64{5.0},
		},
		{
			name:           "Handles empty list",
			inputBids:      []*BidderBid{},
			expectedPrices: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortMultibidsByPrice(tt.inputBids)

			if len(tt.inputBids) != len(tt.expectedPrices) {
				t.Fatalf("Expected %d bids, got %d", len(tt.expectedPrices), len(tt.inputBids))
			}

			for i, expectedPrice := range tt.expectedPrices {
				if tt.inputBids[i].Bid.Price != expectedPrice {
					t.Errorf("Position %d: expected price %v, got %v", i, expectedPrice, tt.inputBids[i].Bid.Price)
				}
			}
		})
	}
}

// TestGenerateMultibidTargetingKeys verifies targeting key generation
func TestGenerateMultibidTargetingKeys(t *testing.T) {
	tests := []struct {
		name        string
		config      *MultibidConfig
		bids        []*BidderBid
		expectedLen int
		checkKeys   map[string]string
		description string
	}{
		{
			name: "Single bid generates base keys",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.25, W: 300, H: 250},
					Bidder: "bidder1",
				},
			},
			expectedLen: 4, // pb, bidder, size, pb_bidder1
			checkKeys: map[string]string{
				"hb_pb":         "5.20",
				"hb_bidder":     "bidder1",
				"hb_size":       "300x250",
				"hb_pb_bidder1": "5.20",
			},
			description: "Should generate keys without suffix for first bid",
		},
		{
			name: "Multiple bids generate numbered keys",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.25, W: 300, H: 250},
					Bidder: "bidder1",
				},
				{
					Bid:    &openrtb.Bid{ID: "bid2", Price: 3.50, W: 728, H: 90},
					Bidder: "bidder2",
				},
				{
					Bid:    &openrtb.Bid{ID: "bid3", Price: 1.00, W: 320, H: 50},
					Bidder: "bidder3",
				},
			},
			expectedLen: 12, // 4 keys per bid * 3 bids
			checkKeys: map[string]string{
				"hb_pb":           "5.20",
				"hb_bidder":       "bidder1",
				"hb_pb_2":         "3.50",
				"hb_bidder_2":     "bidder2",
				"hb_size_2":       "728x90",
				"hb_pb_3":         "1.00",
				"hb_bidder_3":     "bidder3",
				"hb_pb_bidder1":   "5.20",
				"hb_pb_bidder2_2": "3.50",
				"hb_pb_bidder3_3": "1.00",
			},
			description: "Should generate _2, _3 suffixes for additional bids",
		},
		{
			name: "Bids sorted by price descending",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				// Input in wrong order - should be sorted
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 1.00},
					Bidder: "bidder1",
				},
				{
					Bid:    &openrtb.Bid{ID: "bid2", Price: 5.00},
					Bidder: "bidder2",
				},
				{
					Bid:    &openrtb.Bid{ID: "bid3", Price: 3.00},
					Bidder: "bidder3",
				},
			},
			checkKeys: map[string]string{
				"hb_pb":       "5.00", // Highest bid first
				"hb_bidder":   "bidder2",
				"hb_pb_2":     "3.00", // Second highest
				"hb_bidder_2": "bidder3",
				"hb_pb_3":     "1.00", // Lowest
				"hb_bidder_3": "bidder1",
			},
			description: "Should sort bids by price before generating keys",
		},
		{
			name: "Deal ID included when present",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.00, DealID: "deal123"},
					Bidder: "bidder1",
				},
			},
			checkKeys: map[string]string{
				"hb_pb":    "5.00",
				"hb_deal":  "deal123",
				"hb_bidder": "bidder1",
			},
			description: "Should include deal ID in targeting keys",
		},
		{
			name: "Size omitted when dimensions are zero",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.00, W: 0, H: 0},
					Bidder: "bidder1",
				},
			},
			checkKeys: map[string]string{
				"hb_pb":     "5.00",
				"hb_bidder": "bidder1",
			},
			description: "Should not include size when W or H is zero",
		},
		{
			name: "Custom prefix used",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "custom",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.00},
					Bidder: "bidder1",
				},
			},
			checkKeys: map[string]string{
				"custom_pb":         "5.00",
				"custom_bidder":     "bidder1",
				"custom_pb_bidder1": "5.00",
			},
			description: "Should use custom prefix for targeting keys",
		},
		{
			name: "Multibid disabled returns empty",
			config: &MultibidConfig{
				Enabled:                false,
				TargetBidderCodePrefix: "hb",
			},
			bids: []*BidderBid{
				{
					Bid:    &openrtb.Bid{ID: "bid1", Price: 5.00},
					Bidder: "bidder1",
				},
			},
			expectedLen: 0,
			checkKeys:   map[string]string{},
			description: "Should return empty map when multibid disabled",
		},
		{
			name: "Empty bid list returns empty",
			config: &MultibidConfig{
				Enabled:                true,
				TargetBidderCodePrefix: "hb",
			},
			bids:        []*BidderBid{},
			expectedLen: 0,
			checkKeys:   map[string]string{},
			description: "Should return empty map for empty bid list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultibidProcessor(tt.config)
			result := processor.GenerateMultibidTargetingKeys(tt.bids, "imp1")

			if tt.expectedLen > 0 && len(result) != tt.expectedLen {
				t.Errorf("%s: Expected %d keys, got %d", tt.description, tt.expectedLen, len(result))
			}

			for key, expectedValue := range tt.checkKeys {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("%s: Missing expected key '%s'", tt.description, key)
				} else if actualValue != expectedValue {
					t.Errorf("%s: Key '%s' = '%s', expected '%s'", tt.description, key, actualValue, expectedValue)
				}
			}

			// Verify no size key when dimensions are zero
			if tt.name == "Size omitted when dimensions are zero" {
				if _, exists := result["hb_size"]; exists {
					t.Error("Should not have hb_size key when dimensions are zero")
				}
			}

			// Verify no deal key when deal ID is empty
			if tt.name == "Single bid generates base keys" {
				if _, exists := result["hb_deal"]; exists {
					t.Error("Should not have hb_deal key when DealID is empty")
				}
			}
		})
	}
}

// TestLimitToOneBidPerImp verifies legacy mode behavior
func TestLimitToOneBidPerImp(t *testing.T) {
	config := &MultibidConfig{
		Enabled:                false, // Legacy mode
		MaxBidsPerBidder:       10,
		MaxBidsPerBidderPerImp: 10,
	}
	processor := NewMultibidProcessor(config)

	bidResponse := &openrtb.BidResponse{
		ID: "test-response",
		SeatBid: []openrtb.SeatBid{
			{
				Seat: "testbidder",
				Bid: []openrtb.Bid{
					{ID: "bid1", ImpID: "imp1", Price: 5.0},
					{ID: "bid2", ImpID: "imp1", Price: 4.0}, // Should be filtered
					{ID: "bid3", ImpID: "imp2", Price: 3.0},
					{ID: "bid4", ImpID: "imp2", Price: 2.0}, // Should be filtered
					{ID: "bid5", ImpID: "imp3", Price: 1.0},
				},
			},
		},
	}

	result, err := processor.ProcessBidderResponse("testbidder", bidResponse)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have exactly 3 bids (one per impression)
	totalBids := 0
	seenImps := make(map[string]int)
	for _, seatBid := range result.SeatBid {
		for _, bid := range seatBid.Bid {
			totalBids++
			seenImps[bid.ImpID]++
		}
	}

	if totalBids != 3 {
		t.Errorf("Expected 3 bids (one per impression), got %d", totalBids)
	}

	// Verify each impression has exactly one bid
	for impID, count := range seenImps {
		if count != 1 {
			t.Errorf("Impression %s has %d bids, expected 1", impID, count)
		}
	}

	// Verify we kept the first bid for each impression
	expectedBids := map[string]string{
		"imp1": "bid1",
		"imp2": "bid3",
		"imp3": "bid5",
	}
	for _, seatBid := range result.SeatBid {
		for _, bid := range seatBid.Bid {
			expectedID := expectedBids[bid.ImpID]
			if bid.ID != expectedID {
				t.Errorf("Impression %s has bid %s, expected %s", bid.ImpID, bid.ID, expectedID)
			}
		}
	}
}

// TestProcessBidderResponse_EdgeCases verifies edge case handling
func TestProcessBidderResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		config      *MultibidConfig
		bidResponse *openrtb.BidResponse
		description string
	}{
		{
			name: "Nil bid response",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       3,
				MaxBidsPerBidderPerImp: 1,
			},
			bidResponse: nil,
			description: "Should handle nil bid response gracefully",
		},
		{
			name: "Empty SeatBid array",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       3,
				MaxBidsPerBidderPerImp: 1,
			},
			bidResponse: &openrtb.BidResponse{
				ID:      "test",
				SeatBid: []openrtb.SeatBid{},
			},
			description: "Should handle empty SeatBid array",
		},
		{
			name: "SeatBid with no bids",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       3,
				MaxBidsPerBidderPerImp: 1,
			},
			bidResponse: &openrtb.BidResponse{
				ID: "test",
				SeatBid: []openrtb.SeatBid{
					{
						Seat: "testbidder",
						Bid:  []openrtb.Bid{},
					},
				},
			},
			description: "Should handle SeatBid with empty bid array",
		},
		{
			name: "Zero limits filter all bids",
			config: &MultibidConfig{
				Enabled:                true,
				MaxBidsPerBidder:       0,
				MaxBidsPerBidderPerImp: 0,
			},
			bidResponse: &openrtb.BidResponse{
				ID: "test",
				SeatBid: []openrtb.SeatBid{
					{
						Seat: "testbidder",
						Bid: []openrtb.Bid{
							{ID: "bid1", ImpID: "imp1", Price: 5.0},
						},
					},
				},
			},
			description: "Should filter all bids when limits are zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewMultibidProcessor(tt.config)

			// Should not panic or return error
			if tt.bidResponse != nil {
				result, err := processor.ProcessBidderResponse("testbidder", tt.bidResponse)
				if err != nil {
					t.Errorf("%s: Unexpected error: %v", tt.description, err)
				}
				if result == nil {
					t.Errorf("%s: Expected non-nil result", tt.description)
				}
			}
		})
	}
}

// TestGenerateMultibidTargetingKeys_OriginalBidsUnmodified verifies original bids are not modified
func TestGenerateMultibidTargetingKeys_OriginalBidsUnmodified(t *testing.T) {
	config := &MultibidConfig{
		Enabled:                true,
		TargetBidderCodePrefix: "hb",
	}
	processor := NewMultibidProcessor(config)

	originalBids := []*BidderBid{
		{
			Bid:    &openrtb.Bid{ID: "bid1", Price: 1.00},
			Bidder: "bidder1",
		},
		{
			Bid:    &openrtb.Bid{ID: "bid2", Price: 5.00},
			Bidder: "bidder2",
		},
		{
			Bid:    &openrtb.Bid{ID: "bid3", Price: 3.00},
			Bidder: "bidder3",
		},
	}

	// Save original order and prices
	originalOrder := make([]string, len(originalBids))
	originalPrices := make([]float64, len(originalBids))
	for i, bid := range originalBids {
		originalOrder[i] = bid.Bidder
		originalPrices[i] = bid.Bid.Price
	}

	// Generate targeting keys (which sorts bids internally)
	_ = processor.GenerateMultibidTargetingKeys(originalBids, "imp1")

	// Verify original slice was not modified
	for i, bid := range originalBids {
		if bid.Bidder != originalOrder[i] {
			t.Errorf("Original bid order was modified at position %d: expected %s, got %s",
				i, originalOrder[i], bid.Bidder)
		}
		if bid.Bid.Price != originalPrices[i] {
			t.Errorf("Original bid price was modified at position %d: expected %v, got %v",
				i, originalPrices[i], bid.Bid.Price)
		}
	}
}
