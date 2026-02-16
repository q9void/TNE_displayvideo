package hooks

import (
	"context"
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// MultiformatHook handles multiformat impression selection
// Executes AFTER all bidder responses, BEFORE auction
// Picks the best media type per impression based on strategy
type MultiformatHook struct{}

// NewMultiformatHook creates a new multiformat hook
func NewMultiformatHook() *MultiformatHook {
	return &MultiformatHook{}
}

// ProcessAuction applies multiformat selection strategy
func (h *MultiformatHook) ProcessAuction(ctx context.Context, req *openrtb.BidRequest, responses []*BidderResponse) error {
	// Build impression map for O(1) lookup
	impMap := make(map[string]*openrtb.Imp)
	for i := range req.Imp {
		impMap[req.Imp[i].ID] = &req.Imp[i]
	}

	// For each impression, check if multiformat strategy applies
	for _, imp := range req.Imp {
		// Count how many media types this impression supports
		mediaTypeCount := 0
		if imp.Banner != nil {
			mediaTypeCount++
		}
		if imp.Video != nil {
			mediaTypeCount++
		}
		if imp.Native != nil {
			mediaTypeCount++
		}
		if imp.Audio != nil {
			mediaTypeCount++
		}

		// Only apply multiformat logic if impression supports multiple types
		if mediaTypeCount <= 1 {
			continue
		}

		// Extract multiformat config from imp.ext.prebid
		strategy, preferredMediaType := h.getMultiformatConfig(imp)

		if strategy != "" || preferredMediaType != "" {
			logger.Log.Debug().
				Str("imp_id", imp.ID).
				Str("strategy", strategy).
				Str("preferred", preferredMediaType).
				Int("media_type_count", mediaTypeCount).
				Msg("Applying multiformat selection")

			// Apply strategy to filter bids for this impression
			h.applyMultiformatStrategy(imp.ID, strategy, preferredMediaType, responses, impMap)
		}
	}

	return nil
}

// getMultiformatConfig extracts multiformat config from imp.ext.prebid
func (h *MultiformatHook) getMultiformatConfig(imp openrtb.Imp) (string, string) {
	if imp.Ext == nil {
		return "", ""
	}

	var impExt struct {
		Prebid struct {
			MultiformatRequestStrategy string `json:"multiformat_request_strategy"`
			PreferredMediaType         string `json:"preferred_media_type"`
		} `json:"prebid"`
	}

	if err := json.Unmarshal(imp.Ext, &impExt); err != nil {
		return "", ""
	}

	return impExt.Prebid.MultiformatRequestStrategy, impExt.Prebid.PreferredMediaType
}

// applyMultiformatStrategy filters bids based on strategy
func (h *MultiformatHook) applyMultiformatStrategy(impID, strategy, preferredMediaType string, responses []*BidderResponse, impMap map[string]*openrtb.Imp) {
	imp := impMap[impID]
	if imp == nil {
		return
	}

	// Strategy options:
	// - "server_price_priority" (default): Pick highest CPM regardless of type
	// - "preferred_media_type_first": Prefer specified media type if available
	// - "media_type_priority": Use media type priority order

	// For now, implement preferred_media_type_first
	if preferredMediaType == "" && strategy == "" {
		// No strategy specified, use default (price priority)
		return
	}

	// Map preferred media type string to BidType
	var preferredType adapters.BidType
	switch preferredMediaType {
	case "banner":
		preferredType = adapters.BidTypeBanner
	case "video":
		preferredType = adapters.BidTypeVideo
	case "native":
		preferredType = adapters.BidTypeNative
	case "audio":
		preferredType = adapters.BidTypeAudio
	default:
		// Unknown preference, skip filtering
		return
	}

	// Check if we have bids of the preferred type
	hasPreferredType := false
	for _, bidderResp := range responses {
		if bidderResp.Response == nil {
			continue
		}

		for _, seatBid := range bidderResp.Response.SeatBid {
			for _, bid := range seatBid.Bid {
				if bid.ImpID != impID {
					continue
				}

				// Detect bid type
				bidType := adapters.GetBidTypeFromMap(&bid, impMap)
				if bidType == preferredType {
					hasPreferredType = true
					break
				}
			}
			if hasPreferredType {
				break
			}
		}
		if hasPreferredType {
			break
		}
	}

	// If we have bids of preferred type, filter out other types
	if hasPreferredType {
		filteredCount := 0
		for _, bidderResp := range responses {
			if bidderResp.Response == nil {
				continue
			}

			for seatIdx := range bidderResp.Response.SeatBid {
				seatBid := &bidderResp.Response.SeatBid[seatIdx]
				filteredBids := make([]openrtb.Bid, 0, len(seatBid.Bid))

				for _, bid := range seatBid.Bid {
					if bid.ImpID != impID {
						// Keep bids for other impressions
						filteredBids = append(filteredBids, bid)
						continue
					}

					// Check if this bid matches preferred type
					bidType := adapters.GetBidTypeFromMap(&bid, impMap)
					if bidType == preferredType {
						filteredBids = append(filteredBids, bid)
					} else {
						filteredCount++
					}
				}

				seatBid.Bid = filteredBids
			}
		}

		logger.Log.Debug().
			Str("imp_id", impID).
			Str("preferred_type", preferredMediaType).
			Int("filtered_bids", filteredCount).
			Msg("✓ Multiformat: filtered bids to preferred type")
	}
}
