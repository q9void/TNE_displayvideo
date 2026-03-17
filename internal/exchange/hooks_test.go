package exchange

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestCurrencyNormalization tests that currency codes are normalized to uppercase
func TestCurrencyNormalization(t *testing.T) {
	tests := []struct {
		name          string
		bidFloorCur   string
		expectError   bool
		expectedValue string
	}{
		{
			name:          "Lowercase EUR normalized to uppercase",
			bidFloorCur:   "eur",
			expectError:   false,
			expectedValue: "EUR",
		},
		{
			name:          "Mixed case UsD normalized to uppercase",
			bidFloorCur:   "UsD",
			expectError:   false,
			expectedValue: "USD",
		},
		{
			name:          "Already uppercase GBP unchanged",
			bidFloorCur:   "GBP",
			expectError:   false,
			expectedValue: "GBP",
		},
		{
			name:        "Invalid 4-letter code rejected",
			bidFloorCur: "EURO",
			expectError: true,
		},
		{
			name:        "Invalid 2-letter code rejected",
			bidFloorCur: "EU",
			expectError: true,
		},
		{
			name:          "Empty currency allowed (will be set to default)",
			bidFloorCur:   "",
			expectError:   false,
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create exchange with default config
			registry := adapters.NewRegistry()
			ex := New(registry, &Config{
				DefaultTimeout:  1000 * time.Millisecond,
				DefaultCurrency: "USD",
				CloneLimits:     DefaultCloneLimits(),
				IDREnabled:      false,
			})

			// Create auction request with test currency
			req := &AuctionRequest{
				BidRequest: &openrtb.BidRequest{
					ID: "test-auction-1",
					Imp: []openrtb.Imp{
						{
							ID: "imp1",
							Banner: &openrtb.Banner{
								W: 300,
								H: 250,
							},
							BidFloorCur: tt.bidFloorCur,
						},
					},
					Site: &openrtb.Site{
						Domain: "test.com",
					},
				},
				Timeout: 1000 * time.Millisecond,
			}

			// Run auction (which includes currency normalization)
			_, err := ex.RunAuction(context.Background(), req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for invalid currency %q, but got none", tt.bidFloorCur)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expectedValue != "" && req.BidRequest.Imp[0].BidFloorCur != tt.expectedValue {
					t.Errorf("Expected currency %q, got %q", tt.expectedValue, req.BidRequest.Imp[0].BidFloorCur)
				}
			}
		})
	}
}

// TestSChainAugmentation tests that supply chain is properly augmented
func TestSChainAugmentation(t *testing.T) {
	tests := []struct {
		name           string
		inputSChain    *openrtb.SupplyChain
		inputSourceExt json.RawMessage
		expectCreated  bool
		expectPlatform bool
	}{
		{
			name:           "Missing SChain creates new one",
			inputSChain:    nil,
			expectCreated:  true,
			expectPlatform: true,
		},
		{
			name: "Existing SChain gets platform node appended",
			inputSChain: &openrtb.SupplyChain{
				Ver:      "1.0",
				Complete: 1,
				Nodes: []openrtb.SupplyChainNode{
					{
						ASI: "upstream.com",
						SID: "12345",
						HP:  1,
					},
				},
			},
			expectCreated:  false,
			expectPlatform: true,
		},
		{
			name:        "SChain in source.ext moved to source.schain",
			inputSChain: nil,
			inputSourceExt: json.RawMessage(`{
				"schain": {
					"ver": "1.0",
					"complete": 1,
					"nodes": [{
						"asi": "legacy.com",
						"sid": "67890",
						"hp": 1
					}]
				}
			}`),
			expectCreated:  false,
			expectPlatform: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &Exchange{
				config: &Config{
					DefaultCurrency: "USD",
				},
			}

			req := &openrtb.BidRequest{
				ID: "test-req-1",
				Source: &openrtb.Source{
					SChain: tt.inputSChain,
					Ext:    tt.inputSourceExt,
				},
			}

			// Call augmentSChain
			ex.augmentSChain(req, "testbidder", "")

			// Verify SChain exists
			if req.Source == nil || req.Source.SChain == nil {
				t.Fatal("Expected SChain to be created, but it's nil")
			}

			// Verify SChain has nodes
			if len(req.Source.SChain.Nodes) == 0 {
				t.Error("Expected SChain to have nodes, but it's empty")
			}

			// Verify platform node exists
			if tt.expectPlatform {
				foundPlatform := false
				for _, node := range req.Source.SChain.Nodes {
					if node.ASI == "thenexusengine.com" {
						foundPlatform = true
						if node.SID != "NXS001" {
							t.Errorf("Expected platform SID 'NXS001', got %q", node.SID)
						}
						if node.HP != 1 {
							t.Errorf("Expected platform HP=1, got %d", node.HP)
						}
						if node.RID != req.ID {
							t.Errorf("Expected platform RID=%q, got %q", req.ID, node.RID)
						}
						break
					}
				}
				if !foundPlatform {
					t.Error("Expected platform node with ASI='thenexusengine.com', but not found")
				}
			}

			// If SChain was in ext, verify it was moved
			if tt.inputSourceExt != nil {
				var sourceExt struct {
					SChain *openrtb.SupplyChain `json:"schain"`
				}
				if err := json.Unmarshal(tt.inputSourceExt, &sourceExt); err == nil && sourceExt.SChain != nil {
					// Verify the original nodes are preserved
					foundLegacy := false
					for _, node := range req.Source.SChain.Nodes {
						if node.ASI == "legacy.com" {
							foundLegacy = true
							break
						}
					}
					if !foundLegacy {
						t.Error("Expected legacy node to be preserved when moved from ext")
					}
				}
			}
		})
	}
}

// TestSChainNoDuplicatePlatformNode verifies platform node is only added once
func TestSChainNoDuplicatePlatformNode(t *testing.T) {
	ex := &Exchange{
		config: &Config{
			DefaultCurrency: "USD",
		},
	}

	req := &openrtb.BidRequest{
		ID: "test-req-1",
		Source: &openrtb.Source{
			SChain: &openrtb.SupplyChain{
				Ver:      "1.0",
				Complete: 1,
				Nodes: []openrtb.SupplyChainNode{
					{
						ASI: "thenexusengine.com", // Platform node already exists
						SID: "NXS001",
						HP:  1,
						RID: "existing-req-id",
					},
				},
			},
		},
	}

	initialNodeCount := len(req.Source.SChain.Nodes)

	// Call augmentSChain multiple times
	ex.augmentSChain(req, "bidder1", "")
	ex.augmentSChain(req, "bidder2", "")

	// Verify node count didn't increase (no duplicates)
	if len(req.Source.SChain.Nodes) != initialNodeCount {
		t.Errorf("Expected %d nodes (no duplicates), got %d", initialNodeCount, len(req.Source.SChain.Nodes))
	}

	// Verify only one platform node exists
	platformNodeCount := 0
	for _, node := range req.Source.SChain.Nodes {
		if node.ASI == "thenexusengine.com" {
			platformNodeCount++
		}
	}

	if platformNodeCount != 1 {
		t.Errorf("Expected exactly 1 platform node, found %d", platformNodeCount)
	}
}

// TestResponseValidation tests bid response normalization
func TestResponseValidation(t *testing.T) {
	registry := adapters.NewRegistry()
	ex := New(registry, &Config{
		DefaultCurrency: "USD",
		MinBidPrice:     0.01,
		IDREnabled:      false,
	})

	tests := []struct {
		name        string
		bid         *openrtb.Bid
		bidderCode  string
		impID       string
		expectError bool
		errorReason string
	}{
		{
			name: "Valid bid with AdM",
			bid: &openrtb.Bid{
				ID:    "bid1",
				ImpID: "imp1",
				Price: 1.50,
				AdM:   "<div>Ad Creative</div>",
			},
			bidderCode:  "testbidder",
			impID:       "imp1",
			expectError: false,
		},
		{
			name: "Valid bid with NURL",
			bid: &openrtb.Bid{
				ID:    "bid2",
				ImpID: "imp1",
				Price: 2.00,
				NURL:  "https://example.com/win?price=${AUCTION_PRICE}",
			},
			bidderCode:  "testbidder",
			impID:       "imp1",
			expectError: false,
		},
		{
			name: "Invalid bid - missing both AdM and NURL",
			bid: &openrtb.Bid{
				ID:    "bid3",
				ImpID: "imp1",
				Price: 1.00,
			},
			bidderCode:  "testbidder",
			impID:       "imp1",
			expectError: true,
			errorReason: "must have either adm or nurl",
		},
		{
			name: "Invalid bid - invalid NURL (malformed URL)",
			bid: &openrtb.Bid{
				ID:    "bid4",
				ImpID: "imp1",
				Price: 1.00,
				NURL:  "not-a-valid-url",
			},
			bidderCode:  "testbidder",
			impID:       "imp1",
			expectError: true,
			errorReason: "invalid nurl format",
		},
		{
			name: "Invalid bid - negative price",
			bid: &openrtb.Bid{
				ID:    "bid5",
				ImpID: "imp1",
				Price: -1.00,
				AdM:   "<div>Ad</div>",
			},
			bidderCode:  "testbidder",
			impID:       "imp1",
			expectError: true,
			errorReason: "negative price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &openrtb.BidRequest{
				ID: "test-req-1",
				Imp: []openrtb.Imp{
					{
						ID: tt.impID,
						Banner: &openrtb.Banner{
							W: 300,
							H: 250,
						},
					},
				},
			}

			impMap := adapters.BuildImpMap(req.Imp)
			impFloors := make(map[string]float64)

			validErr := ex.validateBid(tt.bid, tt.bidderCode, req, impMap, impFloors)

			if tt.expectError {
				if validErr == nil {
					t.Errorf("Expected validation error, but got none")
				} else if tt.errorReason != "" && !containsSubstring(validErr.Error(), tt.errorReason) {
					t.Errorf("Expected error containing %q, got %q", tt.errorReason, validErr.Error())
				}
			} else {
				if validErr != nil {
					t.Errorf("Unexpected validation error: %v", validErr)
				}
			}
		})
	}
}

// TestMultiformatBidSelection tests multiformat bid selection logic
func TestMultiformatBidSelection(t *testing.T) {
	mfConfig := &MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "server",
	}

	mfp := NewMultiformatProcessor(mfConfig)

	// Create multiformat impression (banner + video)
	imp := &openrtb.Imp{
		ID: "imp1",
		Banner: &openrtb.Banner{
			W: 300,
			H: 250,
		},
		Video: &openrtb.Video{
			W:     640,
			H:     480,
			Mimes: []string{"video/mp4"},
		},
	}

	// Verify impression is detected as multiformat
	if !mfp.IsMultiformat(imp) {
		t.Error("Expected impression to be multiformat, but IsMultiformat returned false")
	}

	// Create candidate bids (one banner, one video)
	candidates := []*BidCandidate{
		{
			Bid: &openrtb.Bid{
				ID:    "bid-banner",
				ImpID: "imp1",
				Price: 1.50,
				W:     300,
				H:     250,
			},
			MediaType:  "banner",
			BidderName: "bidder1",
			CPM:        1.50,
		},
		{
			Bid: &openrtb.Bid{
				ID:       "bid-video",
				ImpID:    "imp1",
				Price:    2.00, // Higher CPM
				Protocol: 2,
			},
			MediaType:  "video",
			BidderName: "bidder2",
			CPM:        2.00,
		},
	}

	// Select best bid (should prefer higher CPM video bid)
	selected := mfp.SelectBestBid(imp, candidates, "")

	if selected == nil {
		t.Fatal("Expected bid to be selected, but got nil")
	}

	if selected.Bid.ID != "bid-video" {
		t.Errorf("Expected video bid to be selected (higher CPM), got %s", selected.Bid.ID)
	}

	if selected.MediaType != "video" {
		t.Errorf("Expected selected bid to be video, got %s", selected.MediaType)
	}
}

// Note: containsSubstring helper is defined in exchange_test.go
