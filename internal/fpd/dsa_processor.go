// Package fpd provides DSA initialization and passthrough logic
package fpd

import (
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// DSAConfig configures DSA behavior
type DSAConfig struct {
	// Enabled turns on DSA processing
	Enabled bool

	// DefaultDSARequired sets default DSA requirement level (0-3)
	DefaultDSARequired int

	// DefaultPubRender sets default publisher render setting (0-2)
	DefaultPubRender int

	// DefaultDataToPub sets default data to publisher setting (0-2)
	DefaultDataToPub int

	// EnforceRequired rejects bids without DSA when required
	EnforceRequired bool
}

// DefaultDSAConfig returns sensible defaults
func DefaultDSAConfig() *DSAConfig {
	return &DSAConfig{
		Enabled:            true,
		DefaultDSARequired: openrtb.DSASupported,       // Support DSA but don't require
		DefaultPubRender:   openrtb.PubRenderMaybe,     // Publisher may render
		DefaultDataToPub:   openrtb.DataToPubIfPresent, // Send if present
		EnforceRequired:    true,
	}
}

// DSAProcessor handles DSA initialization and validation
type DSAProcessor struct {
	config *DSAConfig
}

// NewDSAProcessor creates a new DSA processor
func NewDSAProcessor(config *DSAConfig) *DSAProcessor {
	if config == nil {
		config = DefaultDSAConfig()
	}
	return &DSAProcessor{
		config: config,
	}
}

// InitializeDSA initializes DSA object in request if not present
func (dp *DSAProcessor) InitializeDSA(req *openrtb.BidRequest) {
	if !dp.config.Enabled {
		return
	}

	// Check if DSA already set in request
	if req.Regs != nil && req.Regs.Ext != nil {
		// DSA would be in regs.ext.dsa
		// For now, just ensure Regs exists
	}

	// Initialize Regs if needed
	if req.Regs == nil {
		req.Regs = &openrtb.Regs{}
	}

	// Initialize DSA with defaults if not present
	// Note: This would require extending the Regs model with DSA field
	// For now, this is a placeholder for the initialization logic
}

// ValidateBidResponseDSA validates DSA in bid response
func (dp *DSAProcessor) ValidateBidResponseDSA(
	req *openrtb.BidRequest,
	bidResp *openrtb.BidResponse,
) error {
	if !dp.config.Enabled || !dp.config.EnforceRequired {
		return nil
	}

	// Check if DSA is required in request
	dsaRequired := dp.getDSARequired(req)
	if dsaRequired < openrtb.DSARequired {
		return nil // Not required
	}

	// Validate each seat bid
	for _, seatBid := range bidResp.SeatBid {
		for _, bid := range seatBid.Bid {
			// Check if bid has DSA transparency
			// This would require extending Bid model with DSA field
			// For now, this is a placeholder for validation logic
			_ = bid // Use bid to avoid unused variable error

			// In real implementation:
			// if bid.Ext.DSA == nil || len(bid.Ext.DSA.Transparency) == 0 {
			//     return openrtb.ErrDSARequired
			// }
		}
	}

	return nil
}

// getDSARequired extracts DSA requirement level from request
func (dp *DSAProcessor) getDSARequired(req *openrtb.BidRequest) int {
	// This would extract from req.Regs.Ext.DSA.DSARequired
	// For now, return default
	return dp.config.DefaultDSARequired
}

// ProcessDSATransparency processes DSA transparency for ad rendering
func (dp *DSAProcessor) ProcessDSATransparency(
	bid *openrtb.Bid,
	dsa *openrtb.DSA,
) map[string]string {
	targeting := make(map[string]string)

	if !dp.config.Enabled || dsa == nil {
		return targeting
	}

	// Check if we should send DSA data to publisher
	hasTransparency := len(dsa.Transparency) > 0
	if !dsa.ShouldSendDataToPub(hasTransparency) {
		return targeting
	}

	// Add DSA transparency to targeting
	if hasTransparency {
		// Primary transparency info (first entry)
		trans := dsa.Transparency[0]
		targeting["hb_dsa_domain"] = trans.Domain

		if len(trans.DSAParams) > 0 {
			// Encode DSA params as comma-separated string
			paramsStr := ""
			for i, param := range trans.DSAParams {
				if i > 0 {
					paramsStr += ","
				}
				paramsStr += string(rune(param + '0'))
			}
			targeting["hb_dsa_params"] = paramsStr
		}
	}

	// Add DSA render flag
	if dsa.ShouldPublisherRender() {
		targeting["hb_dsa_render"] = "1"
	}

	return targeting
}

// DSACountry represents countries where DSA applies
var DSACountries = map[string]bool{
	// EU Member States
	"AUT": true, "BEL": true, "BGR": true, "HRV": true, "CYP": true,
	"CZE": true, "DNK": true, "EST": true, "FIN": true, "FRA": true,
	"DEU": true, "GRC": true, "HUN": true, "IRL": true, "ITA": true,
	"LVA": true, "LTU": true, "LUX": true, "MLT": true, "NLD": true,
	"POL": true, "PRT": true, "ROU": true, "SVK": true, "SVN": true,
	"ESP": true, "SWE": true,

	// EEA (non-EU)
	"ISL": true, "LIE": true, "NOR": true,
}

// IsDSAApplicable checks if DSA applies based on user geo
func IsDSAApplicable(req *openrtb.BidRequest) bool {
	// Check device geo
	if req.Device != nil && req.Device.Geo != nil {
		if DSACountries[req.Device.Geo.Country] {
			return true
		}
	}

	// Check user geo
	if req.User != nil && req.User.Geo != nil {
		if DSACountries[req.User.Geo.Country] {
			return true
		}
	}

	return false
}
