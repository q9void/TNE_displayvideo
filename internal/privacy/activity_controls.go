// Package privacy provides activity controls for privacy-impacting operations
package privacy

import (
	"context"
)

// PrivacyRegulation represents different privacy regulations
type PrivacyRegulation string

const (
	RegulationGDPR   PrivacyRegulation = "GDPR"   // EU/EEA - TCF v2 consent
	RegulationCCPA   PrivacyRegulation = "CCPA"   // California - US Privacy String
	RegulationVCDPA  PrivacyRegulation = "VCDPA"  // Virginia
	RegulationCPA    PrivacyRegulation = "CPA"    // Colorado
	RegulationCTDPA  PrivacyRegulation = "CTDPA"  // Connecticut
	RegulationUCPA   PrivacyRegulation = "UCPA"   // Utah
	RegulationLGPD   PrivacyRegulation = "LGPD"   // Brazil
	RegulationPIPEDA PrivacyRegulation = "PIPEDA" // Canada
	RegulationPDPA   PrivacyRegulation = "PDPA"   // Singapore
	RegulationNone   PrivacyRegulation = "NONE"   // No regulation
)

// Activity represents a privacy-impacting activity that can be controlled
type Activity string

const (
	// ActivitySyncUser - User ID syncing (cookie sync)
	ActivitySyncUser Activity = "syncUser"

	// ActivityTransmitUfpd - Transmit user first-party data
	ActivityTransmitUfpd Activity = "transmitUfpd"

	// ActivityTransmitPreciseGeo - Transmit precise geolocation
	ActivityTransmitPreciseGeo Activity = "transmitPreciseGeo"

	// ActivityTransmitEids - Transmit extended IDs (EIDs)
	ActivityTransmitEids Activity = "transmitEids"

	// ActivityTransmitTids - Transmit transaction IDs
	ActivityTransmitTids Activity = "transmitTids"

	// ActivityEnrichUfpd - Enrich user first-party data
	ActivityEnrichUfpd Activity = "enrichUfpd"

	// ActivityFetchBids - Fetch bids from bidders
	ActivityFetchBids Activity = "fetchBids"

	// ActivityReportAnalytics - Report analytics data
	ActivityReportAnalytics Activity = "reportAnalytics"
)

// ActivityControl defines rules for a specific activity
type ActivityControl struct {
	// Activity name
	Activity Activity

	// Default behavior (true = allow, false = deny)
	DefaultAllow bool

	// Rules that override default
	Rules []ActivityRule
}

// ActivityRule defines a conditional rule for activity control
type ActivityRule struct {
	// Condition to evaluate
	Condition ActivityCondition

	// Action when condition matches (true = allow, false = deny)
	Allow bool

	// Priority (higher number = higher priority)
	Priority int
}

// ActivityCondition defines when a rule applies
type ActivityCondition struct {
	// GPP Section IDs that must be present (ANY match)
	GPPSectionIDs []int

	// Component type (e.g., "bidder", "analytics")
	ComponentType string

	// Component names (e.g., specific bidder codes)
	ComponentNames []string

	// Privacy regulation that must apply
	PrivacyRegulation PrivacyRegulation
}

// ActivityResult indicates whether an activity is allowed
type ActivityResult struct {
	// Allowed indicates if activity can proceed
	Allowed bool

	// Reason for the decision
	Reason string

	// Rule that made the decision (if any)
	Rule *ActivityRule
}

// ActivityControlsConfig configures activity controls
type ActivityControlsConfig struct {
	// Enabled turns on activity controls
	Enabled bool

	// Controls for each activity
	Controls map[Activity]*ActivityControl

	// GPP-aware enforcement
	GPPEnforcement bool
}

// DefaultActivityControlsConfig returns sensible defaults
func DefaultActivityControlsConfig() *ActivityControlsConfig {
	return &ActivityControlsConfig{
		Enabled:        true,
		GPPEnforcement: true,
		Controls: map[Activity]*ActivityControl{
			// Sync User - Allow by default but respect GPP opt-outs
			ActivitySyncUser: {
				Activity:     ActivitySyncUser,
				DefaultAllow: true,
				Rules: []ActivityRule{
					{
						Condition: ActivityCondition{
							PrivacyRegulation: RegulationGDPR,
						},
						Allow:    false, // Deny if GDPR applies without consent
						Priority: 10,
					},
				},
			},

			// Transmit UFPD - More restrictive
			ActivityTransmitUfpd: {
				Activity:     ActivityTransmitUfpd,
				DefaultAllow: false, // Deny by default, require explicit consent
				Rules: []ActivityRule{
					{
						Condition: ActivityCondition{
							GPPSectionIDs: []int{GPPSectionTCFv2EU},
						},
						Allow:    true, // Allow if TCF consent present
						Priority: 20,
					},
				},
			},

			// Transmit Precise Geo - Restrictive
			ActivityTransmitPreciseGeo: {
				Activity:     ActivityTransmitPreciseGeo,
				DefaultAllow: false,
				Rules:        []ActivityRule{},
			},

			// Transmit EIDs - Allow by default
			ActivityTransmitEids: {
				Activity:     ActivityTransmitEids,
				DefaultAllow: true,
				Rules:        []ActivityRule{},
			},

			// Transmit TIDs - Allow by default
			ActivityTransmitTids: {
				Activity:     ActivityTransmitTids,
				DefaultAllow: true,
				Rules:        []ActivityRule{},
			},

			// Enrich UFPD - Restrictive
			ActivityEnrichUfpd: {
				Activity:     ActivityEnrichUfpd,
				DefaultAllow: false,
				Rules:        []ActivityRule{},
			},

			// Fetch Bids - Allow by default
			ActivityFetchBids: {
				Activity:     ActivityFetchBids,
				DefaultAllow: true,
				Rules:        []ActivityRule{},
			},

			// Report Analytics - Allow by default
			ActivityReportAnalytics: {
				Activity:     ActivityReportAnalytics,
				DefaultAllow: true,
				Rules:        []ActivityRule{},
			},
		},
	}
}

// ActivityController manages activity control enforcement
type ActivityController struct {
	config *ActivityControlsConfig
}

// NewActivityController creates a new activity controller
func NewActivityController(config *ActivityControlsConfig) *ActivityController {
	if config == nil {
		config = DefaultActivityControlsConfig()
	}
	return &ActivityController{
		config: config,
	}
}

// EvaluateActivity checks if an activity is allowed
func (ac *ActivityController) EvaluateActivity(
	ctx context.Context,
	activity Activity,
	gppString *GPPString,
	componentType string,
	componentName string,
	regulation PrivacyRegulation,
) ActivityResult {
	if !ac.config.Enabled {
		return ActivityResult{
			Allowed: true,
			Reason:  "activity controls disabled",
		}
	}

	control, exists := ac.config.Controls[activity]
	if !exists {
		// No control defined - use safe default (deny)
		return ActivityResult{
			Allowed: false,
			Reason:  "no activity control defined",
		}
	}

	// Evaluate rules in priority order (highest first)
	var matchedRule *ActivityRule
	for i := range control.Rules {
		rule := &control.Rules[i]
		if ac.ruleMatches(rule, gppString, componentType, componentName, regulation) {
			if matchedRule == nil || rule.Priority > matchedRule.Priority {
				matchedRule = rule
			}
		}
	}

	// If a rule matched, use its decision
	if matchedRule != nil {
		return ActivityResult{
			Allowed: matchedRule.Allow,
			Reason:  "matched activity rule",
			Rule:    matchedRule,
		}
	}

	// No rule matched - use default
	return ActivityResult{
		Allowed: control.DefaultAllow,
		Reason:  "default activity policy",
	}
}

// ruleMatches checks if a rule's conditions are met
func (ac *ActivityController) ruleMatches(
	rule *ActivityRule,
	gppString *GPPString,
	componentType string,
	componentName string,
	regulation PrivacyRegulation,
) bool {
	cond := &rule.Condition

	// Check GPP section IDs
	if len(cond.GPPSectionIDs) > 0 && gppString != nil {
		matched := false
		for _, sectionID := range cond.GPPSectionIDs {
			if gppString.HasSection(sectionID) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check component type
	if cond.ComponentType != "" && cond.ComponentType != componentType {
		return false
	}

	// Check component names
	if len(cond.ComponentNames) > 0 {
		matched := false
		for _, name := range cond.ComponentNames {
			if name == componentName {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check privacy regulation
	if cond.PrivacyRegulation != "" && cond.PrivacyRegulation != regulation {
		return false
	}

	return true
}

// CanSyncUser checks if user syncing is allowed
func (ac *ActivityController) CanSyncUser(
	ctx context.Context,
	gppString *GPPString,
	bidderName string,
	regulation PrivacyRegulation,
) bool {
	result := ac.EvaluateActivity(ctx, ActivitySyncUser, gppString, "bidder", bidderName, regulation)
	return result.Allowed
}

// CanTransmitUfpd checks if user first-party data can be transmitted
func (ac *ActivityController) CanTransmitUfpd(
	ctx context.Context,
	gppString *GPPString,
	bidderName string,
	regulation PrivacyRegulation,
) bool {
	result := ac.EvaluateActivity(ctx, ActivityTransmitUfpd, gppString, "bidder", bidderName, regulation)
	return result.Allowed
}

// CanTransmitPreciseGeo checks if precise geolocation can be transmitted
func (ac *ActivityController) CanTransmitPreciseGeo(
	ctx context.Context,
	gppString *GPPString,
	bidderName string,
	regulation PrivacyRegulation,
) bool {
	result := ac.EvaluateActivity(ctx, ActivityTransmitPreciseGeo, gppString, "bidder", bidderName, regulation)
	return result.Allowed
}

// CanTransmitEids checks if extended IDs can be transmitted
func (ac *ActivityController) CanTransmitEids(
	ctx context.Context,
	gppString *GPPString,
	bidderName string,
	regulation PrivacyRegulation,
) bool {
	result := ac.EvaluateActivity(ctx, ActivityTransmitEids, gppString, "bidder", bidderName, regulation)
	return result.Allowed
}
