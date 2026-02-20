package idr

import (
	"context"
	"fmt"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// Adapter implements analytics.Module for IDR service
// Wraps existing event recorder with enhanced event format
type Adapter struct {
	client *idr.Client
	config *Config
}

// Config holds IDR adapter configuration
type Config struct {
	BufferSize  int
	VerboseMode bool // log individual bid details
}

// NewAdapter creates a new IDR analytics adapter
func NewAdapter(client *idr.Client, cfg *Config) *Adapter {
	if cfg == nil {
		cfg = &Config{
			BufferSize:  100,
			VerboseMode: false,
		}
	}
	return &Adapter{
		client: client,
		config: cfg,
	}
}

// LogAuctionObject sends enhanced auction analytics to IDR service
// Converts analytics.AuctionObject to IDR-specific event format
func (a *Adapter) LogAuctionObject(ctx context.Context, auction *analytics.AuctionObject) error {
	// Send enhanced auction-level event
	auctionEvent := &AuctionEvent{
		AuctionID:        auction.AuctionID,
		RequestID:        auction.RequestID,
		PublisherID:      auction.PublisherID,
		Timestamp:        auction.Timestamp,
		ImpressionCount:  len(auction.Impressions),
		BiddersSelected:  len(auction.SelectedBidders),
		BiddersExcluded:  len(auction.ExcludedBidders),
		TotalBids:        auction.TotalBids,
		WinningBids:      len(auction.WinningBids),
		DurationMs:       auction.AuctionDuration.Milliseconds(),
		Status:           auction.Status,
		BidMultiplier:    auction.BidMultiplier,
		TotalRevenue:     auction.TotalRevenue,
		TotalPayout:      auction.TotalPayout,
		Device:           convertDevice(auction.Device),
		ConsentOK:        auction.ConsentOK,
		ValidationErrors: len(auction.ValidationErrors),
	}

	if err := a.client.SendAuctionEvent(ctx, auctionEvent); err != nil {
		return fmt.Errorf("failed to send auction event: %w", err)
	}

	// Send per-bidder events (for ML model)
	for bidder, result := range auction.BidderResults {
		bidderEvent := &BidderEvent{
			AuctionID:   auction.AuctionID,
			BidderCode:  bidder,
			LatencyMs:   result.Latency.Milliseconds(),
			HadBid:      len(result.Bids) > 0,
			BidCount:    len(result.Bids),
			TimedOut:    result.TimedOut,
			HadError:    len(result.Errors) > 0,
			NoBidReason: result.NoBidReason,
			FirstBidCPM: extractFirstBidCPM(result.Bids),
			FloorPrice:  extractFloorForBidder(auction, result),
			BelowFloor:  checkBelowFloor(result.Bids),
			Country:     auction.Device.Country,
			DeviceType:  auction.Device.Type,
			MediaType:   extractMediaType(auction.Impressions),
		}

		if err := a.client.SendBidderEvent(ctx, bidderEvent); err != nil {
			logger.Log.Warn().
				Err(err).
				Str("auction_id", auction.AuctionID).
				Str("bidder", bidder).
				Msg("Failed to send bidder event")
		}
	}

	// Send win events (for revenue tracking)
	for _, win := range auction.WinningBids {
		winEvent := &WinEvent{
			AuctionID:   auction.AuctionID,
			BidID:       win.BidID,
			ImpID:       win.ImpID,
			BidderCode:  win.BidderCode,
			OriginalCPM: win.OriginalPrice,
			AdjustedCPM: win.AdjustedPrice,
			PlatformCut: win.PlatformCut,
			ClearPrice:  win.ClearPrice,
			DemandType:  win.DemandType,
		}

		if err := a.client.SendWinEvent(ctx, winEvent); err != nil {
			logger.Log.Warn().
				Err(err).
				Str("auction_id", auction.AuctionID).
				Str("bid_id", win.BidID).
				Msg("Failed to send win event")
		}
	}

	return nil
}

// LogVideoObject sends video analytics to IDR service
func (a *Adapter) LogVideoObject(ctx context.Context, video *analytics.VideoObject) error {
	// Video event handling - can be implemented later if needed
	return nil
}

// Shutdown flushes any buffered events
func (a *Adapter) Shutdown() error {
	// Flush any buffered events via client
	return a.client.Flush()
}

// Helper functions to extract data from auction object

func convertDevice(device *analytics.DeviceInfo) Device {
	if device == nil {
		return Device{}
	}
	return Device{
		Country: device.Country,
		Type:    device.Type,
	}
}

func extractFirstBidCPM(bids []analytics.BidDetails) *float64 {
	if len(bids) == 0 {
		return nil
	}
	cpm := bids[0].OriginalPrice
	return &cpm
}

func extractFloorForBidder(auction *analytics.AuctionObject, result *analytics.BidderResult) *float64 {
	// Extract floor from first impression if available
	if len(auction.Impressions) > 0 && auction.Impressions[0].Floor > 0 {
		floor := auction.Impressions[0].Floor
		return &floor
	}
	return nil
}

func checkBelowFloor(bids []analytics.BidDetails) bool {
	for _, bid := range bids {
		if bid.BelowFloor {
			return true
		}
	}
	return false
}

func extractMediaType(impressions []analytics.Impression) string {
	if len(impressions) == 0 || len(impressions[0].MediaTypes) == 0 {
		return "banner"
	}
	return impressions[0].MediaTypes[0]
}
