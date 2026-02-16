package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// ResponseNormalizationHook validates and normalizes bidder responses
// Executes per-bidder, AFTER MakeBids, BEFORE auction
// Ensures all bids are PBS-compliant and safe to process
type ResponseNormalizationHook struct{}

// NewResponseNormalizationHook creates a new response normalization hook
func NewResponseNormalizationHook() *ResponseNormalizationHook {
	return &ResponseNormalizationHook{}
}

// ProcessBidderResponse validates and normalizes a bidder's response
func (h *ResponseNormalizationHook) ProcessBidderResponse(ctx context.Context, req *openrtb.BidRequest, resp *openrtb.BidResponse, bidderName string) error {
	if resp == nil {
		// Empty response is OK (no bid)
		return nil
	}

	// 1. Validate BidResponse.id matches BidRequest.id
	if resp.ID != req.ID {
		return fmt.Errorf("bid response id '%s' does not match request id '%s'", resp.ID, req.ID)
	}

	// 2. Normalize and validate currency
	if resp.Cur != "" {
		resp.Cur = strings.ToUpper(resp.Cur)

		// Check if currency is in request allowlist
		if req.Cur != nil && len(req.Cur) > 0 {
			validCurrency := false
			for _, allowedCur := range req.Cur {
				if strings.ToUpper(allowedCur) == resp.Cur {
					validCurrency = true
					break
				}
			}
			if !validCurrency {
				logger.Log.Warn().
					Str("bidder", bidderName).
					Str("response_currency", resp.Cur).
					Strs("allowed_currencies", req.Cur).
					Msg("Bid response currency not in request allowlist - using default")
				// Don't reject - just use first allowed currency
				if len(req.Cur) > 0 {
					resp.Cur = req.Cur[0]
				}
			}
		}
	} else {
		// Default to USD if not specified
		resp.Cur = "USD"
		logger.Log.Debug().
			Str("bidder", bidderName).
			Msg("Bid response missing currency - defaulting to USD")
	}

	// 3. Validate each seat bid
	totalBids := 0
	validBids := 0
	invalidBids := 0

	for seatIdx := range resp.SeatBid {
		seatBid := &resp.SeatBid[seatIdx]

		for bidIdx := range seatBid.Bid {
			bid := &seatBid.Bid[bidIdx]
			totalBids++

			// Validate bid
			if err := h.validateBid(bid, req, bidderName); err != nil {
				logger.Log.Warn().
					Str("bidder", bidderName).
					Str("bid_id", bid.ID).
					Str("imp_id", bid.ImpID).
					Err(err).
					Msg("Invalid bid - will be filtered")
				invalidBids++
				// Mark bid for removal (we'll filter later)
				bid.Price = -1
			} else {
				validBids++
			}
		}
	}

	// 4. Filter out invalid bids (price == -1)
	for seatIdx := range resp.SeatBid {
		seatBid := &resp.SeatBid[seatIdx]
		filteredBids := make([]openrtb.Bid, 0, len(seatBid.Bid))

		for _, bid := range seatBid.Bid {
			if bid.Price >= 0 {
				filteredBids = append(filteredBids, bid)
			}
		}

		seatBid.Bid = filteredBids
	}

	// 5. Remove empty seat bids
	filteredSeatBids := make([]openrtb.SeatBid, 0, len(resp.SeatBid))
	for _, seatBid := range resp.SeatBid {
		if len(seatBid.Bid) > 0 {
			filteredSeatBids = append(filteredSeatBids, seatBid)
		}
	}
	resp.SeatBid = filteredSeatBids

	logger.Log.Debug().
		Str("bidder", bidderName).
		Str("response_id", resp.ID).
		Str("currency", resp.Cur).
		Int("total_bids", totalBids).
		Int("valid_bids", validBids).
		Int("invalid_bids", invalidBids).
		Int("seat_bids", len(resp.SeatBid)).
		Msg("✓ Response normalization complete")

	return nil
}

// validateBid validates a single bid
func (h *ResponseNormalizationHook) validateBid(bid *openrtb.Bid, req *openrtb.BidRequest, bidderName string) error {
	// 1. Validate bid.id exists
	if bid.ID == "" {
		return fmt.Errorf("bid missing required id")
	}

	// 2. Validate bid.impid exists and matches an impression
	if bid.ImpID == "" {
		return fmt.Errorf("bid missing required impid")
	}

	// Check if impid matches a request impression
	impExists := false
	for _, imp := range req.Imp {
		if imp.ID == bid.ImpID {
			impExists = true
			break
		}
	}
	if !impExists {
		return fmt.Errorf("bid impid '%s' does not match any request impression", bid.ImpID)
	}

	// 3. Validate price is positive
	if bid.Price <= 0 {
		return fmt.Errorf("bid price must be positive, got %.4f", bid.Price)
	}

	// 4. Validate nurl or adm present (at least one required)
	if bid.NURL == "" && bid.AdM == "" {
		return fmt.Errorf("bid must have either nurl or adm")
	}

	// 5. Validate dimensions for banner bids
	// Note: For video/native, dimensions may come from creative metadata
	if bid.W == 0 || bid.H == 0 {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("bid_id", bid.ID).
			Msg("Bid missing width/height - may be video/native")
	}

	// 6. Validate creative ID exists (important for reporting)
	if bid.CRID == "" {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("bid_id", bid.ID).
			Msg("Bid missing creative ID (crid) - using bid.id")
		bid.CRID = bid.ID
	}

	return nil
}
