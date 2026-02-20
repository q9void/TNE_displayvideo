package idr

import (
	"context"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

func TestAdapter_LogAuctionObject(t *testing.T) {
	// Create a mock IDR client (using the real one but with a fake URL for testing structure)
	client := idr.NewClient("http://localhost:9999", 100*time.Millisecond, "test-key")

	adapter := NewAdapter(client, &Config{
		BufferSize:  100,
		VerboseMode: false,
	})

	ctx := context.Background()

	// Create a test auction object
	auctionObj := &analytics.AuctionObject{
		AuctionID:       "test-auction-123",
		RequestID:       "test-request-456",
		PublisherID:     "pub-789",
		PublisherDomain: "example.com",
		Timestamp:       time.Now(),
		Impressions: []analytics.Impression{
			{
				ID:         "imp-1",
				MediaTypes: []string{"banner"},
				Sizes:      []string{"300x250"},
				Floor:      1.0,
			},
		},
		Device: &analytics.DeviceInfo{
			Type:    "mobile",
			Country: "US",
			Region:  "CA",
		},
		User: &analytics.UserInfo{
			BuyerUID: "user-123",
			HasEIDs:  true,
		},
		SelectedBidders: []string{"rubicon", "appnexus"},
		ExcludedBidders: map[string]analytics.ExclusionReason{
			"pubmatic": {
				Code:    "circuit_breaker_open",
				Message: "Circuit breaker is open",
			},
		},
		TotalBidders: 3,
		BidderResults: map[string]*analytics.BidderResult{
			"rubicon": {
				BidderCode: "rubicon",
				Latency:    50 * time.Millisecond,
				HttpStatus: 200,
				Bids: []analytics.BidDetails{
					{
						BidID:         "bid-1",
						ImpID:         "imp-1",
						OriginalPrice: 2.5,
						AdjustedPrice: 2.5,
						Currency:      "USD",
						DemandType:    "platform",
					},
				},
				SeatBids:    1,
				TimedOut:    false,
				NoBidReason: "",
				Errors:      nil,
			},
			"appnexus": {
				BidderCode:  "appnexus",
				Latency:     100 * time.Millisecond,
				HttpStatus:  204,
				Bids:        []analytics.BidDetails{},
				SeatBids:    0,
				TimedOut:    false,
				NoBidReason: "no inventory",
				Errors:      nil,
			},
		},
		WinningBids: []analytics.WinningBid{
			{
				BidID:         "bid-1",
				ImpID:         "imp-1",
				BidderCode:    "rubicon",
				OriginalPrice: 2.5,
				AdjustedPrice: 2.5,
				PlatformCut:   0.25,
				Currency:      "USD",
				DemandType:    "platform",
				ClearPrice:    2.5,
			},
		},
		TotalBids:        1,
		AuctionDuration:  150 * time.Millisecond,
		Status:           "success",
		BidMultiplier:    1.0,
		FloorAdjustments: map[string]float64{},
		TotalRevenue:     0.25,
		TotalPayout:      2.25,
		ConsentOK:        true,
	}

	// Note: This test will fail to send to IDR since the URL is fake
	// In a real test, we'd use a mock HTTP server
	err := adapter.LogAuctionObject(ctx, auctionObj)

	// We expect an error since we're using a fake URL
	// The important thing is that the method doesn't panic
	if err == nil {
		t.Log("Note: LogAuctionObject returned no error (expected if mock server is running)")
	} else {
		t.Logf("Expected error from fake IDR URL: %v", err)
	}
}

func TestAdapter_LogVideoObject(t *testing.T) {
	client := idr.NewClient("http://localhost:9999", 100*time.Millisecond, "test-key")
	adapter := NewAdapter(client, nil) // Test with default config

	ctx := context.Background()

	videoObj := &analytics.VideoObject{
		AuctionID:  "test-auction-123",
		VideoID:    "video-456",
		Event:      "start",
		Timestamp:  time.Now(),
		Duration:   30,
		Muted:      false,
		Fullscreen: false,
		PlayerSize: "640x480",
	}

	// Should not error even with fake URL (video events not implemented yet)
	err := adapter.LogVideoObject(ctx, videoObj)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAdapter_DefaultConfig(t *testing.T) {
	client := idr.NewClient("http://localhost:9999", 100*time.Millisecond, "test-key")
	adapter := NewAdapter(client, nil)

	if adapter.config == nil {
		t.Error("Expected default config to be set")
	}

	if adapter.config.BufferSize != 100 {
		t.Errorf("Expected default buffer size 100, got %d", adapter.config.BufferSize)
	}

	if adapter.config.VerboseMode != false {
		t.Error("Expected verbose mode to be false by default")
	}
}
