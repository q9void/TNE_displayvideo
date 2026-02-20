package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// RequestValidationHook validates and normalizes OpenRTB requests
// Executes FIRST, before any other processing
type RequestValidationHook struct{}

// NewRequestValidationHook creates a new request validation hook
func NewRequestValidationHook() *RequestValidationHook {
	return &RequestValidationHook{}
}

// ProcessRequest validates and normalizes the bid request
func (h *RequestValidationHook) ProcessRequest(ctx context.Context, req *openrtb.BidRequest) error {
	// 1. Validate site XOR app (exactly one must be present)
	if req.Site != nil && req.App != nil {
		return fmt.Errorf("request cannot have both site and app")
	}
	if req.Site == nil && req.App == nil {
		return fmt.Errorf("request must have either site or app")
	}

	// 2. Validate impressions present
	if len(req.Imp) == 0 {
		return fmt.Errorf("request must have at least one impression")
	}

	// 3. Validate unique imp.id
	impIDs := make(map[string]bool)
	for _, imp := range req.Imp {
		if imp.ID == "" {
			return fmt.Errorf("impression missing required id")
		}
		if impIDs[imp.ID] {
			return fmt.Errorf("duplicate impression id: %s", imp.ID)
		}
		impIDs[imp.ID] = true
	}

	// 4. Normalize currency codes to uppercase (OpenRTB spec requires ISO-4217)
	if req.Cur != nil {
		for i, cur := range req.Cur {
			req.Cur[i] = strings.ToUpper(cur)
		}
		logger.Log.Debug().
			Strs("currencies", req.Cur).
			Msg("Normalized request currency codes")
	}

	// 5. Normalize bid floor currency per impression
	for i := range req.Imp {
		if req.Imp[i].BidFloorCur != "" {
			req.Imp[i].BidFloorCur = strings.ToUpper(req.Imp[i].BidFloorCur)
		}
	}

	// 6. Enforce tmax bounds (PBS best practices: 100ms min, 5000ms max)
	if req.TMax > 0 {
		if req.TMax < 100 {
			originalTMax := req.TMax
			req.TMax = 100
			logger.Log.Debug().
				Int("original_tmax", originalTMax).
				Int("enforced_tmax", 100).
				Msg("Enforced minimum tmax")
		} else if req.TMax > 5000 {
			originalTMax := req.TMax
			req.TMax = 5000
			logger.Log.Debug().
				Int("original_tmax", originalTMax).
				Int("enforced_tmax", 5000).
				Msg("Enforced maximum tmax")
		}
	}

	// 7. Validate and normalize source.schain
	// If schain exists in source.ext, ensure it's properly formatted
	if req.Source != nil && req.Source.Ext != nil {
		var sourceExt map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &sourceExt); err == nil {
			// Check if schain exists in ext
			if schainRaw, ok := sourceExt["schain"]; ok {
				// Validate schain structure
				schainBytes, err := json.Marshal(schainRaw)
				if err != nil {
					logger.Log.Warn().
						Err(err).
						Msg("Invalid schain in source.ext - removing")
					delete(sourceExt, "schain")
				} else {
					var schain openrtb.SupplyChain
					if err := json.Unmarshal(schainBytes, &schain); err != nil {
						logger.Log.Warn().
							Err(err).
							Msg("Malformed schain structure - removing")
						delete(sourceExt, "schain")
					} else {
						// Validate schain version
						if schain.Ver != "1.0" {
							logger.Log.Warn().
								Str("version", schain.Ver).
								Msg("Unknown schain version - expected 1.0")
						}
					}
				}

				// Re-marshal source.ext if we modified it
				if extBytes, err := json.Marshal(sourceExt); err == nil {
					req.Source.Ext = extBytes
				}
			}
		}
	}

	// 8. Validate each impression has at least one media type
	for _, imp := range req.Imp {
		hasMediaType := imp.Banner != nil || imp.Video != nil || imp.Native != nil || imp.Audio != nil
		if !hasMediaType {
			return fmt.Errorf("impression %s must have at least one media type (banner, video, native, audio)", imp.ID)
		}
	}

	logger.Log.Debug().
		Str("request_id", req.ID).
		Int("imp_count", len(req.Imp)).
		Bool("has_site", req.Site != nil).
		Bool("has_app", req.App != nil).
		Msg("✓ Request validation passed")

	return nil
}
