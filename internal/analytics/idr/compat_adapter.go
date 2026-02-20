package idr

import (
	"context"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

// CompatAdapter provides backwards compatibility with the old event format
// Use this if your IDR service doesn't support the new endpoints yet
// It converts the new rich AuctionObject format to the old BidEvent format
type CompatAdapter struct {
	recorder *idr.EventRecorder
}

// NewCompatAdapter creates a backwards-compatible adapter
// This sends events in the old format to POST /api/events
func NewCompatAdapter(baseURL string, bufferSize int) *CompatAdapter {
	return &CompatAdapter{
		recorder: idr.NewEventRecorder(baseURL, bufferSize),
	}
}

// LogAuctionObject converts new analytics format to old event format
// Maintains compatibility while you update the IDR service
func (a *CompatAdapter) LogAuctionObject(ctx context.Context, auction *analytics.AuctionObject) error {
	// Convert to old BidEvent format for each bidder
	for bidderCode, result := range auction.BidderResults {
		var bidCPM *float64
		if len(result.Bids) > 0 {
			cpm := result.Bids[0].OriginalPrice
			bidCPM = &cpm
		}

		var floor *float64
		if len(auction.Impressions) > 0 && auction.Impressions[0].Floor > 0 {
			f := auction.Impressions[0].Floor
			floor = &f
		}

		mediaType := ""
		adSize := ""
		if len(auction.Impressions) > 0 {
			if len(auction.Impressions[0].MediaTypes) > 0 {
				mediaType = auction.Impressions[0].MediaTypes[0]
			}
			if len(auction.Impressions[0].Sizes) > 0 {
				adSize = auction.Impressions[0].Sizes[0]
			}
		}

		errorMsg := ""
		if len(result.Errors) > 0 {
			errorMsg = result.Errors[0]
		}

		country := ""
		deviceType := ""
		if auction.Device != nil {
			country = auction.Device.Country
			deviceType = auction.Device.Type
		}

		// Record using old format (POST /api/events)
		a.recorder.RecordBidResponse(
			auction.AuctionID,
			bidderCode,
			float64(result.Latency.Milliseconds()),
			len(result.Bids) > 0,
			bidCPM,
			floor,
			country,
			deviceType,
			mediaType,
			adSize,
			auction.PublisherID,
			result.TimedOut,
			len(result.Errors) > 0,
			errorMsg,
		)
	}

	// Record win events using old format
	for _, win := range auction.WinningBids {
		country := ""
		deviceType := ""
		if auction.Device != nil {
			country = auction.Device.Country
			deviceType = auction.Device.Type
		}

		a.recorder.RecordWin(
			auction.AuctionID,
			win.BidderCode,
			win.AdjustedPrice,
			country,
			deviceType,
			extractMediaType(auction.Impressions),
			extractAdSize(auction.Impressions),
			auction.PublisherID,
		)
	}

	return nil
}

func (a *CompatAdapter) LogVideoObject(ctx context.Context, video *analytics.VideoObject) error {
	// Video events not implemented in old format
	return nil
}

func (a *CompatAdapter) Shutdown() error {
	return a.recorder.Close()
}

// extractAdSize is a local helper for compat adapter
func extractAdSize(imps []analytics.Impression) string {
	if len(imps) == 0 || len(imps[0].Sizes) == 0 {
		return ""
	}
	return imps[0].Sizes[0]
}
