package hooks

import (
	"context"
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// SChainAugmentationHook completes the supply chain by appending platform node
// Executes per-bidder, AFTER identity gating, BEFORE adapter
// Ensures transparent supply chain for DSPs (anti-fraud, brand safety)
type SChainAugmentationHook struct {
	platformASI string // "thenexusengine.com"
	platformSID string // Account ID (from bid request context)
}

// NewSChainAugmentationHook creates a new schain augmentation hook
func NewSChainAugmentationHook(platformASI, platformSID string) *SChainAugmentationHook {
	return &SChainAugmentationHook{
		platformASI: platformASI,
		platformSID: platformSID,
	}
}

// ProcessBidderRequest augments the supply chain with platform node
func (h *SChainAugmentationHook) ProcessBidderRequest(ctx context.Context, req *openrtb.BidRequest, bidderName string) error {
	// Ensure source object exists
	if req.Source == nil {
		req.Source = &openrtb.Source{}
	}

	// Get existing schain or create new one
	schain := h.getExistingSChain(req)

	// If no schain exists, create one
	if schain.Ver == "" {
		schain.Ver = "1.0"
		schain.Complete = 1 // We control the entire chain
		schain.Nodes = []openrtb.SupplyChainNode{}
	}

	// Create platform node
	platformNode := openrtb.SupplyChainNode{
		ASI:    h.platformASI,           // Advertising System Identifier
		SID:    h.platformSID,           // Seller ID (account ID)
		HP:     1,                       // Hash Priority: 1 = direct seller
		RID:    req.ID,                  // Request ID for tracking
		Name:   "TheNexusEngine",        // Platform name
		Domain: h.platformASI,           // Platform domain
	}

	// Check if platform node already exists (prevent duplicates)
	platformNodeExists := false
	for _, node := range schain.Nodes {
		if node.ASI == h.platformASI && node.SID == h.platformSID {
			platformNodeExists = true
			break
		}
	}

	// Append platform node if not present
	if !platformNodeExists {
		schain.Nodes = append(schain.Nodes, platformNode)
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("asi", h.platformASI).
			Str("sid", h.platformSID).
			Str("request_id", req.ID).
			Int("total_nodes", len(schain.Nodes)).
			Msg("✓ Appended platform node to schain")
	} else {
		logger.Log.Debug().
			Str("bidder", bidderName).
			Str("request_id", req.ID).
			Msg("Platform node already exists in schain")
	}

	// Set schain in source.SChain (typed field)
	h.setSChain(req, &schain)

	return nil
}

// getExistingSChain extracts existing schain, preferring typed source.SChain (OpenRTB 2.6)
// and falling back to source.ext.schain for backward compatibility.
func (h *SChainAugmentationHook) getExistingSChain(req *openrtb.BidRequest) openrtb.SupplyChain {
	// Prefer typed field (OpenRTB 2.6)
	if req.Source != nil && req.Source.SChain != nil {
		return *req.Source.SChain
	}
	// Legacy: source.ext.schain
	var schain openrtb.SupplyChain
	if req.Source == nil || req.Source.Ext == nil {
		return schain
	}
	var sourceExt struct {
		SChain *openrtb.SupplyChain `json:"schain"`
	}
	if err := json.Unmarshal(req.Source.Ext, &sourceExt); err == nil && sourceExt.SChain != nil {
		return *sourceExt.SChain
	}
	return schain
}

// setSChain writes schain to source.SChain (typed, canonical OpenRTB 2.6 location)
// and removes any legacy schain key from source.ext to prevent duplication.
func (h *SChainAugmentationHook) setSChain(req *openrtb.BidRequest, schain *openrtb.SupplyChain) {
	// Write to typed field (canonical OpenRTB 2.6 location)
	req.Source.SChain = schain

	// Remove legacy schain from source.ext to prevent duplication
	if req.Source.Ext != nil {
		var sourceExt map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &sourceExt); err == nil {
			if _, hasSchain := sourceExt["schain"]; hasSchain {
				delete(sourceExt, "schain")
				if len(sourceExt) == 0 {
					req.Source.Ext = nil
				} else if extBytes, err := json.Marshal(sourceExt); err == nil {
					req.Source.Ext = extBytes
				} else {
					logger.Log.Error().
						Err(err).
						Msg("Failed to marshal source.ext after removing legacy schain")
				}
			}
		}
	}
}
