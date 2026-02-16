// Package exchange provides multiformat request handling
package exchange

import (
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// MultiformatConfig configures multiformat behavior
type MultiformatConfig struct {
	// Enabled turns on multiformat support
	Enabled bool

	// DefaultStrategy when not specified in request
	// Options: "server" (server picks best), "preferDeal" (prefer deal over format)
	DefaultStrategy string
}

// DefaultMultiformatConfig returns sensible defaults
func DefaultMultiformatConfig() *MultiformatConfig {
	return &MultiformatConfig{
		Enabled:         true,
		DefaultStrategy: "server", // Server picks best format
	}
}

// MultiformatProcessor handles multiformat impression selection
type MultiformatProcessor struct {
	config *MultiformatConfig
}

// NewMultiformatProcessor creates a new multiformat processor
func NewMultiformatProcessor(config *MultiformatConfig) *MultiformatProcessor {
	if config == nil {
		config = DefaultMultiformatConfig()
	}
	return &MultiformatProcessor{
		config: config,
	}
}

// SelectBestBid selects the best bid for a multiformat impression
// Returns nil if multiformat processing is disabled (caller should keep all bids)
func (mfp *MultiformatProcessor) SelectBestBid(
	imp *openrtb.Imp,
	bids []*BidCandidate,
	preferredMediaType string,
) *BidCandidate {
	if len(bids) == 0 {
		return nil
	}

	// If multiformat processing is disabled, return nil to signal caller
	// should keep all bids unchanged
	if !mfp.config.Enabled {
		return nil
	}

	if len(bids) == 1 {
		return bids[0]
	}

	// Determine selection strategy
	strategy := mfp.getStrategy(imp, preferredMediaType)

	switch strategy {
	case "preferDeal":
		return mfp.selectPreferDeal(bids, preferredMediaType)
	case "preferMediaType":
		return mfp.selectPreferMediaType(bids, preferredMediaType)
	default: // "server" or unknown
		return mfp.selectServerBest(bids, preferredMediaType)
	}
}

// getStrategy determines which selection strategy to use
func (mfp *MultiformatProcessor) getStrategy(
	imp *openrtb.Imp,
	preferredMediaType string,
) string {
	// Check if strategy specified in imp.ext.prebid.multiformatRequestStrategy
	if imp.Ext != nil {
		var prebidExt struct {
			Prebid struct {
				MultiformatRequestStrategy string `json:"multiformatRequestStrategy"`
			} `json:"prebid"`
		}

		if err := json.Unmarshal(imp.Ext, &prebidExt); err == nil {
			if prebidExt.Prebid.MultiformatRequestStrategy != "" {
				return prebidExt.Prebid.MultiformatRequestStrategy
			}
		}
	}

	return mfp.config.DefaultStrategy
}

// selectServerBest selects best bid using server logic
// Priority: Deal ID > Preferred Format > Highest CPM
func (mfp *MultiformatProcessor) selectServerBest(
	bids []*BidCandidate,
	preferredMediaType string,
) *BidCandidate {
	var best *BidCandidate

	for _, bid := range bids {
		if best == nil {
			best = bid
			continue
		}

		// Priority 1: Deal bids win
		if bid.HasDeal && !best.HasDeal {
			best = bid
			continue
		}
		if !bid.HasDeal && best.HasDeal {
			continue
		}

		// Priority 2: Preferred media type wins (if CPM within 5%)
		if preferredMediaType != "" {
			bidMatchesPreferred := bid.MediaType == preferredMediaType
			bestMatchesPreferred := best.MediaType == preferredMediaType

			if bidMatchesPreferred && !bestMatchesPreferred {
				// Bid matches preferred, best doesn't - take bid if CPM close
				if bid.CPM >= best.CPM*0.95 {
					best = bid
					continue
				}
			}
			if !bidMatchesPreferred && bestMatchesPreferred {
				// Best matches preferred, bid doesn't - keep best unless bid much higher
				if bid.CPM < best.CPM*1.05 {
					continue
				}
			}
		}

		// Priority 3: Highest CPM wins
		if bid.CPM > best.CPM {
			best = bid
		}
	}

	return best
}

// selectPreferDeal selects bid preferring deals over format match
// Priority: Deal ID > Highest CPM (ignore format preference)
func (mfp *MultiformatProcessor) selectPreferDeal(
	bids []*BidCandidate,
	preferredMediaType string,
) *BidCandidate {
	var best *BidCandidate

	for _, bid := range bids {
		if best == nil {
			best = bid
			continue
		}

		// Deals always win
		if bid.HasDeal && !best.HasDeal {
			best = bid
			continue
		}
		if !bid.HasDeal && best.HasDeal {
			continue
		}

		// Highest CPM wins (format preference ignored)
		if bid.CPM > best.CPM {
			best = bid
		}
	}

	return best
}

// selectPreferMediaType selects bid strictly preferring media type
// Priority: Preferred Format > Deal ID > Highest CPM
// When preferMediaType strategy is used, format preference is paramount
func (mfp *MultiformatProcessor) selectPreferMediaType(
	bids []*BidCandidate,
	preferredMediaType string,
) *BidCandidate {
	if preferredMediaType == "" {
		return mfp.selectServerBest(bids, "")
	}

	var bestPreferredDeal *BidCandidate // Preferred format with deal
	var bestPreferred *BidCandidate     // Preferred format without deal
	var bestOtherDeal *BidCandidate     // Other format with deal
	var bestOther *BidCandidate         // Other format without deal

	for _, bid := range bids {
		matchesPreferred := bid.MediaType == preferredMediaType

		if matchesPreferred {
			if bid.HasDeal {
				if bestPreferredDeal == nil || bid.CPM > bestPreferredDeal.CPM {
					bestPreferredDeal = bid
				}
			} else {
				if bestPreferred == nil || bid.CPM > bestPreferred.CPM {
					bestPreferred = bid
				}
			}
		} else {
			if bid.HasDeal {
				if bestOtherDeal == nil || bid.CPM > bestOtherDeal.CPM {
					bestOtherDeal = bid
				}
			} else {
				if bestOther == nil || bid.CPM > bestOther.CPM {
					bestOther = bid
				}
			}
		}
	}

	// Priority: Preferred+Deal > Preferred > Other+Deal > Other
	// This ensures preferred format wins, but deals get secondary priority
	if bestPreferredDeal != nil {
		return bestPreferredDeal
	}
	if bestPreferred != nil {
		return bestPreferred
	}
	if bestOtherDeal != nil {
		return bestOtherDeal
	}
	return bestOther
}

// GetPreferredMediaType extracts preferred media type from impression
func (mfp *MultiformatProcessor) GetPreferredMediaType(imp *openrtb.Imp) string {
	// Check imp.ext.prebid.preferredMediaType
	if imp.Ext != nil {
		var prebidExt struct {
			Prebid struct {
				PreferredMediaType string `json:"preferredMediaType"`
			} `json:"prebid"`
		}

		if err := json.Unmarshal(imp.Ext, &prebidExt); err == nil {
			if prebidExt.Prebid.PreferredMediaType != "" {
				return prebidExt.Prebid.PreferredMediaType
			}
		}
	}

	// Fallback to heuristic based on what's present
	// Priority order: Video > Audio > Native > Banner
	if imp.Video != nil {
		return "video"
	}
	if imp.Audio != nil {
		return "audio"
	}
	if imp.Native != nil {
		return "native"
	}
	if imp.Banner != nil {
		return "banner"
	}

	return ""
}

// IsMultiformat checks if impression accepts multiple formats
func (mfp *MultiformatProcessor) IsMultiformat(imp *openrtb.Imp) bool {
	formatCount := 0

	if imp.Banner != nil {
		formatCount++
	}
	if imp.Video != nil {
		formatCount++
	}
	if imp.Native != nil {
		formatCount++
	}
	if imp.Audio != nil {
		formatCount++
	}

	return formatCount > 1
}

// BidCandidate represents a bid with metadata for selection
type BidCandidate struct {
	Bid        *openrtb.Bid
	MediaType  string
	CPM        float64
	HasDeal    bool
	BidderName string
}

// NewBidCandidate creates a bid candidate from a bid
func NewBidCandidate(bid *openrtb.Bid, mediaType string, bidderName string) *BidCandidate {
	return &BidCandidate{
		Bid:        bid,
		MediaType:  mediaType,
		CPM:        bid.Price,
		HasDeal:    bid.DealID != "",
		BidderName: bidderName,
	}
}
