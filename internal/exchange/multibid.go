// Package exchange provides multibid support for returning multiple bids per bidder
package exchange

import (
	"fmt"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// MultibidConfig configures multibid behavior
type MultibidConfig struct {
	// Enabled turns on multibid support
	Enabled bool

	// MaxBidsPerBidder limits bids from a single bidder (default: 1, max recommended: 9)
	MaxBidsPerBidder int

	// MaxBidsPerBidderPerImp limits bids from a single bidder for a single impression
	MaxBidsPerBidderPerImp int

	// TargetBidderCodePrefix for multibid targeting keys (default: "hb")
	TargetBidderCodePrefix string
}

// DefaultMultibidConfig returns sensible defaults
func DefaultMultibidConfig() *MultibidConfig {
	return &MultibidConfig{
		Enabled:                true,
		MaxBidsPerBidder:       3,  // Allow up to 3 bids per bidder total
		MaxBidsPerBidderPerImp: 1,  // But only 1 per impression by default
		TargetBidderCodePrefix: "hb",
	}
}

// MultibidProcessor handles multibid bid responses
type MultibidProcessor struct {
	config *MultibidConfig
}

// NewMultibidProcessor creates a new multibid processor
func NewMultibidProcessor(config *MultibidConfig) *MultibidProcessor {
	if config == nil {
		config = DefaultMultibidConfig()
	}
	return &MultibidProcessor{
		config: config,
	}
}

// ProcessBidderResponse handles multiple bids from a single bidder
func (mp *MultibidProcessor) ProcessBidderResponse(
	bidderName string,
	bidResponse *openrtb.BidResponse,
) (*openrtb.BidResponse, error) {
	if !mp.config.Enabled {
		// Multibid disabled - take only first bid per impression
		return mp.limitToOneBidPerImp(bidResponse), nil
	}

	// Count bids per impression
	impBidCount := make(map[string]int)
	totalBids := 0

	for seatIdx := range bidResponse.SeatBid {
		seatBid := &bidResponse.SeatBid[seatIdx]

		// Filter bids based on limits
		filteredBids := make([]openrtb.Bid, 0, len(seatBid.Bid))

		for bidIdx := range seatBid.Bid {
			bid := &seatBid.Bid[bidIdx]

			// Check per-impression limit
			currentCount := impBidCount[bid.ImpID]
			if currentCount >= mp.config.MaxBidsPerBidderPerImp {
				continue // Skip this bid - impression limit reached
			}

			// Check total bidder limit
			if totalBids >= mp.config.MaxBidsPerBidder {
				continue // Skip this bid - bidder limit reached
			}

			filteredBids = append(filteredBids, *bid)
			impBidCount[bid.ImpID]++
			totalBids++
		}

		seatBid.Bid = filteredBids
	}

	return bidResponse, nil
}

// limitToOneBidPerImp ensures only one bid per impression (legacy behavior)
func (mp *MultibidProcessor) limitToOneBidPerImp(bidResponse *openrtb.BidResponse) *openrtb.BidResponse {
	seenImps := make(map[string]bool)

	for seatIdx := range bidResponse.SeatBid {
		seatBid := &bidResponse.SeatBid[seatIdx]
		filteredBids := make([]openrtb.Bid, 0, len(seatBid.Bid))

		for bidIdx := range seatBid.Bid {
			bid := &seatBid.Bid[bidIdx]

			if !seenImps[bid.ImpID] {
				filteredBids = append(filteredBids, *bid)
				seenImps[bid.ImpID] = true
			}
		}

		seatBid.Bid = filteredBids
	}

	return bidResponse
}

// GenerateMultibidTargetingKeys generates targeting keys for multiple bids
func (mp *MultibidProcessor) GenerateMultibidTargetingKeys(
	bids []*BidderBid,
	impID string,
) map[string]string {
	targeting := make(map[string]string)

	if !mp.config.Enabled || len(bids) == 0 {
		return targeting
	}

	// Sort bids by price (highest first)
	sortedBids := make([]*BidderBid, len(bids))
	copy(sortedBids, bids)
	sortMultibidsByPrice(sortedBids)

	// Generate keys for each bid
	for i, bid := range sortedBids {
		suffix := ""
		if i > 0 {
			suffix = fmt.Sprintf("_%d", i+1)
		}

		// Price bucket
		targeting[fmt.Sprintf("%s_pb%s", mp.config.TargetBidderCodePrefix, suffix)] =
			getPriceBucket(bid.Bid.Price)

		// Bidder name
		targeting[fmt.Sprintf("%s_bidder%s", mp.config.TargetBidderCodePrefix, suffix)] =
			bid.Bidder

		// Size
		if bid.Bid.W > 0 && bid.Bid.H > 0 {
			targeting[fmt.Sprintf("%s_size%s", mp.config.TargetBidderCodePrefix, suffix)] =
				fmt.Sprintf("%dx%d", bid.Bid.W, bid.Bid.H)
		}

		// Deal ID (if present)
		if bid.Bid.DealID != "" {
			targeting[fmt.Sprintf("%s_deal%s", mp.config.TargetBidderCodePrefix, suffix)] =
				bid.Bid.DealID
		}

		// Bidder-specific keys
		targeting[fmt.Sprintf("%s_pb_%s%s", mp.config.TargetBidderCodePrefix, bid.Bidder, suffix)] =
			getPriceBucket(bid.Bid.Price)
	}

	return targeting
}

// BidderBid represents a bid with its bidder information
type BidderBid struct {
	Bid    *openrtb.Bid
	Bidder string
}

// sortMultibidsByPrice sorts bids by price descending (highest first)
func sortMultibidsByPrice(bids []*BidderBid) {
	// Simple bubble sort - good enough for small arrays (typically <10 bids)
	for i := 0; i < len(bids); i++ {
		for j := i + 1; j < len(bids); j++ {
			if bids[j].Bid.Price > bids[i].Bid.Price {
				bids[i], bids[j] = bids[j], bids[i]
			}
		}
	}
}

// getPriceBucket converts a price to a price bucket string
func getPriceBucket(price float64) string {
	// Standard price buckets (Prebid.js compatible)
	// $0-$5: $0.05 increments
	// $5-$10: $0.10 increments
	// $10-$20: $0.50 increments
	// $20+: $1.00 increments

	if price < 0 {
		return "0.00"
	}

	var bucket float64
	switch {
	case price <= 5.0:
		bucket = float64(int(price*20)) / 20.0 // $0.05 increments
	case price <= 10.0:
		bucket = float64(int(price*10)) / 10.0 // $0.10 increments
	case price <= 20.0:
		bucket = float64(int(price*2)) / 2.0 // $0.50 increments
	default:
		bucket = float64(int(price)) // $1.00 increments
	}

	return fmt.Sprintf("%.2f", bucket)
}

// MultibidExtension represents Prebid multibid extension
type MultibidExtension struct {
	// Bidders is a list of bidder codes allowed to return multiple bids
	Bidders []string `json:"bidders,omitempty"`

	// MaxBids is the maximum number of bids per bidder
	MaxBids int `json:"maxbids,omitempty"`

	// TargetBidderCodePrefix for targeting keys
	TargetBidderCodePrefix string `json:"targetbiddercodeprefix,omitempty"`
}
