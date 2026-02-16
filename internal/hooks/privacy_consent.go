package hooks

import (
	"context"
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// PrivacyConsentHook enforces GDPR, CCPA, and GPP privacy regulations
// Executes SECOND, BEFORE adapters see the request
// CRITICAL: Prevents PII leakage without consent
type PrivacyConsentHook struct{}

// NewPrivacyConsentHook creates a new privacy consent hook
func NewPrivacyConsentHook() *PrivacyConsentHook {
	return &PrivacyConsentHook{}
}

// ProcessRequest enforces privacy regulations and clears IDs without consent
func (h *PrivacyConsentHook) ProcessRequest(ctx context.Context, req *openrtb.BidRequest) error {
	// Check all privacy frameworks
	hasGDPRConsent := h.checkGDPRConsent(req)
	hasCCPAConsent := h.checkCCPAConsent(req)
	hasGPPConsent := h.checkGPPConsent(req)

	// Determine if we can use personal identifiers
	canUseIDs := hasGDPRConsent && hasCCPAConsent && hasGPPConsent

	if !canUseIDs {
		logger.Log.Warn().
			Bool("gdpr_consent", hasGDPRConsent).
			Bool("ccpa_consent", hasCCPAConsent).
			Bool("gpp_consent", hasGPPConsent).
			Str("request_id", req.ID).
			Msg("🔒 Privacy: No valid consent - stripping all user IDs")

		// Strip all user identifiers
		h.stripUserIDs(req)
	} else {
		logger.Log.Debug().
			Bool("gdpr_consent", hasGDPRConsent).
			Bool("ccpa_consent", hasCCPAConsent).
			Bool("gpp_consent", hasGPPConsent).
			Str("request_id", req.ID).
			Msg("✓ Privacy: Valid consent - preserving user IDs")
	}

	// ALWAYS clear CATALYST internal IDs (even with consent)
	// These are routing IDs, not SSP IDs. Adapters will set SSP-specific IDs.
	h.clearInternalIDs(req)

	// Task #53: Map regs to bidder-specific ext fields
	// Some bidders need privacy signals in ext.prebid or ext.gdpr
	h.mapRegsToBidderExt(req)

	return nil
}

// mapRegsToBidderExt maps regs fields to bidder-specific ext fields
// Task #53: Some bidders expect privacy signals in specific ext locations
func (h *PrivacyConsentHook) mapRegsToBidderExt(req *openrtb.BidRequest) {
	if req.Regs == nil {
		return
	}

	// Build ext.prebid with privacy signals
	var reqExt map[string]interface{}
	if req.Ext != nil && len(req.Ext) > 0 {
		if err := json.Unmarshal(req.Ext, &reqExt); err != nil {
			// Failed to parse - create new
			reqExt = make(map[string]interface{})
		}
	} else {
		reqExt = make(map[string]interface{})
	}

	// Get or create prebid extension
	var prebidExt map[string]interface{}
	if pbData, ok := reqExt["prebid"]; ok {
		if pbMap, ok := pbData.(map[string]interface{}); ok {
			prebidExt = pbMap
		} else {
			prebidExt = make(map[string]interface{})
		}
	} else {
		prebidExt = make(map[string]interface{})
	}

	// Map GDPR to ext.prebid.gdpr
	if req.Regs.GDPR != nil {
		prebidExt["gdpr"] = *req.Regs.GDPR
	}

	// Map US Privacy to ext.prebid.us_privacy
	if req.Regs.USPrivacy != "" {
		prebidExt["us_privacy"] = req.Regs.USPrivacy
	}

	// Map GPP to ext.prebid.gpp
	if req.Regs.GPP != "" {
		prebidExt["gpp"] = req.Regs.GPP
		if len(req.Regs.GPPSID) > 0 {
			prebidExt["gpp_sid"] = req.Regs.GPPSID
		}
	}

	// Map COPPA to ext.prebid.coppa
	if req.Regs.COPPA > 0 {
		prebidExt["coppa"] = req.Regs.COPPA
	}

	// Map user consent to ext.prebid.consent
	if req.User != nil && req.User.Consent != "" {
		prebidExt["consent"] = req.User.Consent
	}

	// Update request extension
	reqExt["prebid"] = prebidExt
	if extBytes, err := json.Marshal(reqExt); err == nil {
		req.Ext = extBytes
		logger.Log.Debug().
			Str("request_id", req.ID).
			Msg("✓ Mapped regs to bidder-specific ext fields")
	}
}

// stripUserIDs removes all personal identifiers from the request
func (h *PrivacyConsentHook) stripUserIDs(req *openrtb.BidRequest) {
	if req.User != nil {
		// Clear user.id and buyeruid
		req.User.ID = ""
		req.User.BuyerUID = ""

		// Strip user.ext.eids (Extended Identifiers)
		if len(req.User.Ext) > 0 {
			var userExt map[string]interface{}
			if err := json.Unmarshal(req.User.Ext, &userExt); err == nil {
				delete(userExt, "eids")
				if extBytes, err := json.Marshal(userExt); err == nil {
					req.User.Ext = extBytes
					logger.Log.Debug().Msg("Stripped user.ext.eids (no consent)")
				}
			}
		}
	}

	// Clear device IDs
	if req.Device != nil {
		req.Device.IFA = ""       // Identifier for Advertising (mobile)
		req.Device.DPIDMD5 = ""   // Platform device ID MD5
		req.Device.DPIDSHA1 = ""  // Platform device ID SHA1
	}
}

// clearInternalIDs removes CATALYST routing IDs from site/app/publisher
// These should NEVER be sent to SSPs (they're internal to CATALYST)
func (h *PrivacyConsentHook) clearInternalIDs(req *openrtb.BidRequest) {
	// Clear site IDs
	if req.Site != nil {
		req.Site.ID = "" // CATALYST accountId should NOT leak to SSPs

		if req.Site.Publisher != nil {
			req.Site.Publisher.ID = "" // Adapters set SSP-specific publisher IDs
		}
	}

	// Clear app IDs
	if req.App != nil {
		req.App.ID = "" // CATALYST accountId should NOT leak to SSPs

		if req.App.Publisher != nil {
			req.App.Publisher.ID = "" // Adapters set SSP-specific publisher IDs
		}
	}

	logger.Log.Debug().
		Str("request_id", req.ID).
		Msg("✓ Cleared CATALYST internal IDs (site.id, publisher.id)")
}

// checkGDPRConsent validates GDPR consent string
func (h *PrivacyConsentHook) checkGDPRConsent(req *openrtb.BidRequest) bool {
	// Check if GDPR applies
	if req.Regs == nil || req.Regs.Ext == nil {
		// No GDPR specified - default to allowing
		return true
	}

	var regsExt struct {
		GDPR *int8 `json:"gdpr"`
	}
	if err := json.Unmarshal(req.Regs.Ext, &regsExt); err != nil {
		// Parse error - default to allowing
		return true
	}

	// If GDPR doesn't apply (gdpr=0), allow all processing
	if regsExt.GDPR == nil || *regsExt.GDPR == 0 {
		return true
	}

	// GDPR applies (gdpr=1) - check for valid consent string
	if req.User == nil || req.User.Ext == nil {
		logger.Log.Warn().Msg("GDPR applies but no user.ext.consent found")
		return false
	}

	var userExt struct {
		Consent string `json:"consent"`
	}
	if err := json.Unmarshal(req.User.Ext, &userExt); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to parse user.ext for consent")
		return false
	}

	// Check if consent string exists
	if userExt.Consent == "" {
		logger.Log.Warn().Msg("GDPR applies but consent string is empty")
		return false
	}

	// TODO: Parse TCF consent string and check Purpose 1 (Store/access information)
	// For now, we accept any non-empty consent string
	// Production implementation should use IAB TCF library to parse and validate

	logger.Log.Debug().
		Str("consent_prefix", userExt.Consent[:min(10, len(userExt.Consent))]).
		Msg("GDPR consent string present")

	return true
}

// checkCCPAConsent validates US Privacy (CCPA) string
func (h *PrivacyConsentHook) checkCCPAConsent(req *openrtb.BidRequest) bool {
	// Check for US Privacy string
	if req.Regs == nil || req.Regs.Ext == nil {
		// No CCPA specified - default to allowing
		return true
	}

	var regsExt struct {
		USPrivacy string `json:"us_privacy"`
	}
	if err := json.Unmarshal(req.Regs.Ext, &regsExt); err != nil {
		// Parse error - default to allowing
		return true
	}

	// No US Privacy string - default to allowing
	if regsExt.USPrivacy == "" {
		return true
	}

	// Parse CCPA string (format: "1YNN")
	// Position 0: Version (1)
	// Position 1: Notice given (Y/N)
	// Position 2: Opt-out (Y=opted out, N=no opt-out)
	// Position 3: LSPA (Limited Service Provider Agreement)
	if len(regsExt.USPrivacy) < 3 {
		logger.Log.Warn().
			Str("us_privacy", regsExt.USPrivacy).
			Msg("Invalid US Privacy string - too short")
		return false
	}

	// Check if user opted out (position 2 == 'Y')
	if regsExt.USPrivacy[2] == 'Y' {
		logger.Log.Warn().
			Str("us_privacy", regsExt.USPrivacy).
			Msg("CCPA opt-out detected - blocking ID usage")
		return false
	}

	logger.Log.Debug().
		Str("us_privacy", regsExt.USPrivacy).
		Msg("CCPA consent OK")

	return true
}

// checkGPPConsent validates Global Privacy Platform (GPP) consent
func (h *PrivacyConsentHook) checkGPPConsent(req *openrtb.BidRequest) bool {
	// Check for GPP (Global Privacy Platform)
	if req.Regs == nil || req.Regs.Ext == nil {
		// No GPP specified - default to allowing
		return true
	}

	var regsExt struct {
		GPP    string `json:"gpp"`
		GPPSID []int  `json:"gpp_sid"`
	}
	if err := json.Unmarshal(req.Regs.Ext, &regsExt); err != nil {
		// Parse error - default to allowing
		return true
	}

	// No GPP string - default to allowing
	if regsExt.GPP == "" {
		return true
	}

	// TODO: Parse GPP string and check activity controls
	// GPP is complex - each section ID (gpp_sid) has different rules:
	// - Section 2: TCFv2 (GDPR)
	// - Section 6: USNat (National US Privacy)
	// - Section 7-12: State-specific (California, Virginia, etc.)
	//
	// For now, we accept any GPP string as valid
	// Production implementation should use IAB GPP library

	logger.Log.Debug().
		Str("gpp_prefix", regsExt.GPP[:min(20, len(regsExt.GPP))]).
		Ints("gpp_sid", regsExt.GPPSID).
		Msg("GPP consent string present")

	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
