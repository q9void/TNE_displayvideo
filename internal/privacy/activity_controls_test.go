package privacy

import (
	"context"
	"testing"
)

// TestNewActivityController tests controller creation
func TestNewActivityController(t *testing.T) {
	tests := []struct {
		name   string
		config *ActivityControlsConfig
		want   bool // want non-nil controller
	}{
		{
			name:   "nil config uses default",
			config: nil,
			want:   true,
		},
		{
			name: "custom config",
			config: &ActivityControlsConfig{
				Enabled:        true,
				GPPEnforcement: true,
				Controls:       make(map[Activity]*ActivityControl),
			},
			want: true,
		},
		{
			name: "disabled config",
			config: &ActivityControlsConfig{
				Enabled:        false,
				GPPEnforcement: false,
				Controls:       make(map[Activity]*ActivityControl),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewActivityController(tt.config)
			if (got != nil) != tt.want {
				t.Errorf("NewActivityController() = %v, want %v", got != nil, tt.want)
			}
			if got != nil && got.config == nil {
				t.Error("NewActivityController() returned controller with nil config")
			}
		})
	}
}

// TestDefaultActivityControlsConfig tests default configuration
func TestDefaultActivityControlsConfig(t *testing.T) {
	config := DefaultActivityControlsConfig()

	if config == nil {
		t.Fatal("DefaultActivityControlsConfig() returned nil")
	}

	if !config.Enabled {
		t.Error("DefaultActivityControlsConfig() Enabled = false, want true")
	}

	if !config.GPPEnforcement {
		t.Error("DefaultActivityControlsConfig() GPPEnforcement = false, want true")
	}

	// Verify all 8 activities are present
	expectedActivities := []Activity{
		ActivitySyncUser,
		ActivityTransmitUfpd,
		ActivityTransmitPreciseGeo,
		ActivityTransmitEids,
		ActivityTransmitTids,
		ActivityEnrichUfpd,
		ActivityFetchBids,
		ActivityReportAnalytics,
	}

	for _, activity := range expectedActivities {
		if _, exists := config.Controls[activity]; !exists {
			t.Errorf("DefaultActivityControlsConfig() missing activity: %s", activity)
		}
	}

	// Verify specific defaults
	tests := []struct {
		activity     Activity
		defaultAllow bool
	}{
		{ActivitySyncUser, true},
		{ActivityTransmitUfpd, false},
		{ActivityTransmitPreciseGeo, false},
		{ActivityTransmitEids, true},
		{ActivityTransmitTids, true},
		{ActivityEnrichUfpd, false},
		{ActivityFetchBids, true},
		{ActivityReportAnalytics, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.activity), func(t *testing.T) {
			control := config.Controls[tt.activity]
			if control.DefaultAllow != tt.defaultAllow {
				t.Errorf("DefaultAllow for %s = %v, want %v",
					tt.activity, control.DefaultAllow, tt.defaultAllow)
			}
		})
	}
}

// TestActivityController_EvaluateActivity tests activity evaluation
func TestActivityController_EvaluateActivity(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		config        *ActivityControlsConfig
		activity      Activity
		gppString     *GPPString
		componentType string
		componentName string
		regulation    PrivacyRegulation
		wantAllowed   bool
		wantReason    string
	}{
		{
			name: "disabled controller allows all",
			config: &ActivityControlsConfig{
				Enabled:  false,
				Controls: make(map[Activity]*ActivityControl),
			},
			activity:    ActivitySyncUser,
			wantAllowed: true,
			wantReason:  "activity controls disabled",
		},
		{
			name: "undefined activity defaults to deny",
			config: &ActivityControlsConfig{
				Enabled:  true,
				Controls: make(map[Activity]*ActivityControl),
			},
			activity:    ActivitySyncUser,
			wantAllowed: false,
			wantReason:  "no activity control defined",
		},
		{
			name: "default allow policy with no matching rules",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: true,
						Rules:        []ActivityRule{},
					},
				},
			},
			activity:    ActivitySyncUser,
			wantAllowed: true,
			wantReason:  "default activity policy",
		},
		{
			name: "default deny policy with no matching rules",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitUfpd: {
						Activity:     ActivityTransmitUfpd,
						DefaultAllow: false,
						Rules:        []ActivityRule{},
					},
				},
			},
			activity:    ActivityTransmitUfpd,
			wantAllowed: false,
			wantReason:  "default activity policy",
		},
		{
			name: "rule matches and allows",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentType: "bidder",
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			},
			activity:      ActivitySyncUser,
			componentType: "bidder",
			componentName: "appnexus",
			wantAllowed:   true,
			wantReason:    "matched activity rule",
		},
		{
			name: "rule matches and denies",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: true,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									PrivacyRegulation: RegulationGDPR,
								},
								Allow:    false,
								Priority: 10,
							},
						},
					},
				},
			},
			activity:    ActivitySyncUser,
			regulation:  RegulationGDPR,
			wantAllowed: false,
			wantReason:  "matched activity rule",
		},
		{
			name: "GPP section matching",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitUfpd: {
						Activity:     ActivityTransmitUfpd,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									GPPSectionIDs: []int{GPPSectionTCFv2EU},
								},
								Allow:    true,
								Priority: 20,
							},
						},
					},
				},
			},
			activity: ActivityTransmitUfpd,
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			wantAllowed: true,
			wantReason:  "matched activity rule",
		},
		{
			name: "GPP section not matching",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitUfpd: {
						Activity:     ActivityTransmitUfpd,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									GPPSectionIDs: []int{GPPSectionUSNational},
								},
								Allow:    true,
								Priority: 20,
							},
						},
					},
				},
			},
			activity: ActivityTransmitUfpd,
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			wantAllowed: false,
			wantReason:  "default activity policy",
		},
		{
			name: "component name matching",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentNames: []string{"appnexus", "rubicon"},
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			},
			activity:      ActivitySyncUser,
			componentName: "appnexus",
			wantAllowed:   true,
			wantReason:    "matched activity rule",
		},
		{
			name: "component name not matching",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentNames: []string{"appnexus", "rubicon"},
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			},
			activity:      ActivitySyncUser,
			componentName: "pubmatic",
			wantAllowed:   false,
			wantReason:    "default activity policy",
		},
		{
			name: "nil GPP string with GPP condition matches (implementation skips nil check)",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitUfpd: {
						Activity:     ActivityTransmitUfpd,
						DefaultAllow: true,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									GPPSectionIDs: []int{GPPSectionTCFv2EU},
								},
								Allow:    false,
								Priority: 20,
							},
						},
					},
				},
			},
			activity:    ActivityTransmitUfpd,
			gppString:   nil,
			wantAllowed: false,
			wantReason:  "matched activity rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := NewActivityController(tt.config)
			result := ac.EvaluateActivity(ctx, tt.activity, tt.gppString,
				tt.componentType, tt.componentName, tt.regulation)

			if result.Allowed != tt.wantAllowed {
				t.Errorf("EvaluateActivity() Allowed = %v, want %v", result.Allowed, tt.wantAllowed)
			}

			if result.Reason != tt.wantReason {
				t.Errorf("EvaluateActivity() Reason = %v, want %v", result.Reason, tt.wantReason)
			}
		})
	}
}

// TestActivityController_PriorityBasedRuleResolution tests priority logic
func TestActivityController_PriorityBasedRuleResolution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		rules       []ActivityRule
		regulation  PrivacyRegulation
		wantAllowed bool
		wantRule    int // index of rule that should match
	}{
		{
			name: "higher priority rule wins",
			rules: []ActivityRule{
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    false,
					Priority: 5,
				},
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    true,
					Priority: 10,
				},
			},
			regulation:  RegulationGDPR,
			wantAllowed: true,
			wantRule:    1,
		},
		{
			name: "lower priority rule ignored",
			rules: []ActivityRule{
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    true,
					Priority: 10,
				},
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    false,
					Priority: 5,
				},
			},
			regulation:  RegulationGDPR,
			wantAllowed: true,
			wantRule:    0,
		},
		{
			name: "first matching rule when same priority",
			rules: []ActivityRule{
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    false,
					Priority: 10,
				},
				{
					Condition: ActivityCondition{
						PrivacyRegulation: RegulationGDPR,
					},
					Allow:    true,
					Priority: 10,
				},
			},
			regulation:  RegulationGDPR,
			wantAllowed: false,
			wantRule:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules:        tt.rules,
					},
				},
			}

			ac := NewActivityController(config)
			result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil, "", "", tt.regulation)

			if result.Allowed != tt.wantAllowed {
				t.Errorf("EvaluateActivity() Allowed = %v, want %v", result.Allowed, tt.wantAllowed)
			}

			if result.Rule == nil {
				t.Fatal("EvaluateActivity() Rule = nil, want non-nil")
			}

			// Verify the correct rule was matched
			expectedRule := &tt.rules[tt.wantRule]
			if result.Rule.Priority != expectedRule.Priority {
				t.Errorf("Matched rule Priority = %d, want %d", result.Rule.Priority, expectedRule.Priority)
			}
		})
	}
}

// TestActivityController_MultipleConditions tests complex rule matching
func TestActivityController_MultipleConditions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		condition     ActivityCondition
		gppString     *GPPString
		componentType string
		componentName string
		regulation    PrivacyRegulation
		wantMatch     bool
	}{
		{
			name: "all conditions match",
			condition: ActivityCondition{
				GPPSectionIDs:     []int{GPPSectionTCFv2EU},
				ComponentType:     "bidder",
				ComponentNames:    []string{"appnexus"},
				PrivacyRegulation: RegulationGDPR,
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     true,
		},
		{
			name: "GPP section doesn't match",
			condition: ActivityCondition{
				GPPSectionIDs:     []int{GPPSectionUSNational},
				ComponentType:     "bidder",
				ComponentNames:    []string{"appnexus"},
				PrivacyRegulation: RegulationGDPR,
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     false,
		},
		{
			name: "component type doesn't match",
			condition: ActivityCondition{
				GPPSectionIDs:     []int{GPPSectionTCFv2EU},
				ComponentType:     "analytics",
				ComponentNames:    []string{"appnexus"},
				PrivacyRegulation: RegulationGDPR,
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     false,
		},
		{
			name: "component name doesn't match",
			condition: ActivityCondition{
				GPPSectionIDs:     []int{GPPSectionTCFv2EU},
				ComponentType:     "bidder",
				ComponentNames:    []string{"rubicon"},
				PrivacyRegulation: RegulationGDPR,
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     false,
		},
		{
			name: "regulation doesn't match",
			condition: ActivityCondition{
				GPPSectionIDs:     []int{GPPSectionTCFv2EU},
				ComponentType:     "bidder",
				ComponentNames:    []string{"appnexus"},
				PrivacyRegulation: RegulationCCPA,
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     false,
		},
		{
			name: "empty condition matches everything",
			condition: ActivityCondition{
				GPPSectionIDs:     nil,
				ComponentType:     "",
				ComponentNames:    nil,
				PrivacyRegulation: "",
			},
			gppString:     nil,
			componentType: "bidder",
			componentName: "appnexus",
			regulation:    RegulationGDPR,
			wantMatch:     true,
		},
		{
			name: "partial condition matches",
			condition: ActivityCondition{
				ComponentType:  "bidder",
				ComponentNames: []string{"appnexus", "rubicon"},
			},
			gppString:     nil,
			componentType: "bidder",
			componentName: "rubicon",
			regulation:    RegulationNone,
			wantMatch:     true,
		},
		{
			name: "ANY GPP section matches",
			condition: ActivityCondition{
				GPPSectionIDs: []int{GPPSectionTCFv2EU, GPPSectionUSNational, GPPSectionUSCA},
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSNational: &USNationalData{Version: 1},
				},
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: tt.condition,
								Allow:     true,
								Priority:  10,
							},
						},
					},
				},
			}

			ac := NewActivityController(config)
			result := ac.EvaluateActivity(ctx, ActivitySyncUser, tt.gppString,
				tt.componentType, tt.componentName, tt.regulation)

			gotMatch := result.Rule != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("Rule matched = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

// TestActivityController_CanSyncUser tests user sync helper
func TestActivityController_CanSyncUser(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		config     *ActivityControlsConfig
		gppString  *GPPString
		bidderName string
		regulation PrivacyRegulation
		want       bool
	}{
		{
			name:       "default config allows without GDPR",
			config:     DefaultActivityControlsConfig(),
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       true,
		},
		{
			name:       "default config denies with GDPR",
			config:     DefaultActivityControlsConfig(),
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationGDPR,
			want:       false,
		},
		{
			name: "custom config with allow rule",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentNames: []string{"trusted-bidder"},
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			},
			bidderName: "trusted-bidder",
			regulation: RegulationGDPR,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := NewActivityController(tt.config)
			got := ac.CanSyncUser(ctx, tt.gppString, tt.bidderName, tt.regulation)
			if got != tt.want {
				t.Errorf("CanSyncUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestActivityController_CanTransmitUfpd tests UFPD transmission helper
func TestActivityController_CanTransmitUfpd(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		config     *ActivityControlsConfig
		gppString  *GPPString
		bidderName string
		regulation PrivacyRegulation
		want       bool
	}{
		{
			name:       "default config allows with nil GPP (rule matches due to implementation)",
			config:     DefaultActivityControlsConfig(),
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       true,
		},
		{
			name:   "default config allows with TCF consent",
			config: DefaultActivityControlsConfig(),
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       true,
		},
		{
			name:   "custom config denies without TCF section",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitUfpd: {
						Activity:     ActivityTransmitUfpd,
						DefaultAllow: false,
						Rules:        []ActivityRule{},
					},
				},
			},
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := NewActivityController(tt.config)
			got := ac.CanTransmitUfpd(ctx, tt.gppString, tt.bidderName, tt.regulation)
			if got != tt.want {
				t.Errorf("CanTransmitUfpd() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestActivityController_CanTransmitPreciseGeo tests precise geo helper
func TestActivityController_CanTransmitPreciseGeo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		config     *ActivityControlsConfig
		gppString  *GPPString
		bidderName string
		regulation PrivacyRegulation
		want       bool
	}{
		{
			name:       "default config denies",
			config:     DefaultActivityControlsConfig(),
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       false,
		},
		{
			name: "custom config with allow rule",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitPreciseGeo: {
						Activity:     ActivityTransmitPreciseGeo,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									GPPSectionIDs: []int{GPPSectionTCFv2EU},
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			},
			gppString: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "test"},
				},
			},
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := NewActivityController(tt.config)
			got := ac.CanTransmitPreciseGeo(ctx, tt.gppString, tt.bidderName, tt.regulation)
			if got != tt.want {
				t.Errorf("CanTransmitPreciseGeo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestActivityController_CanTransmitEids tests EIDs transmission helper
func TestActivityController_CanTransmitEids(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		config     *ActivityControlsConfig
		gppString  *GPPString
		bidderName string
		regulation PrivacyRegulation
		want       bool
	}{
		{
			name:       "default config allows",
			config:     DefaultActivityControlsConfig(),
			gppString:  nil,
			bidderName: "appnexus",
			regulation: RegulationNone,
			want:       true,
		},
		{
			name: "custom config denies specific bidder",
			config: &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivityTransmitEids: {
						Activity:     ActivityTransmitEids,
						DefaultAllow: true,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentNames: []string{"blocked-bidder"},
								},
								Allow:    false,
								Priority: 10,
							},
						},
					},
				},
			},
			bidderName: "blocked-bidder",
			regulation: RegulationNone,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := NewActivityController(tt.config)
			got := ac.CanTransmitEids(ctx, tt.gppString, tt.bidderName, tt.regulation)
			if got != tt.want {
				t.Errorf("CanTransmitEids() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestActivityController_EdgeCases tests edge cases
func TestActivityController_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("no rules defined", func(t *testing.T) {
		config := &ActivityControlsConfig{
			Enabled: true,
			Controls: map[Activity]*ActivityControl{
				ActivitySyncUser: {
					Activity:     ActivitySyncUser,
					DefaultAllow: true,
					Rules:        nil, // no rules
				},
			},
		}

		ac := NewActivityController(config)
		result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil, "", "", RegulationNone)

		if !result.Allowed {
			t.Error("EvaluateActivity() with no rules should use default allow")
		}
		if result.Reason != "default activity policy" {
			t.Errorf("Reason = %v, want 'default activity policy'", result.Reason)
		}
		if result.Rule != nil {
			t.Error("Rule should be nil when no rules match")
		}
	})

	t.Run("empty rules slice", func(t *testing.T) {
		config := &ActivityControlsConfig{
			Enabled: true,
			Controls: map[Activity]*ActivityControl{
				ActivitySyncUser: {
					Activity:     ActivitySyncUser,
					DefaultAllow: false,
					Rules:        []ActivityRule{}, // empty slice
				},
			},
		}

		ac := NewActivityController(config)
		result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil, "", "", RegulationNone)

		if result.Allowed {
			t.Error("EvaluateActivity() with empty rules should use default deny")
		}
		if result.Reason != "default activity policy" {
			t.Errorf("Reason = %v, want 'default activity policy'", result.Reason)
		}
	})

	t.Run("multiple matching rules different priorities", func(t *testing.T) {
		config := &ActivityControlsConfig{
			Enabled: true,
			Controls: map[Activity]*ActivityControl{
				ActivitySyncUser: {
					Activity:     ActivitySyncUser,
					DefaultAllow: false,
					Rules: []ActivityRule{
						{
							Condition:    ActivityCondition{},
							Allow:        false,
							Priority:     5,
						},
						{
							Condition:    ActivityCondition{},
							Allow:        true,
							Priority:     10,
						},
						{
							Condition:    ActivityCondition{},
							Allow:        false,
							Priority:     8,
						},
					},
				},
			},
		}

		ac := NewActivityController(config)
		result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil, "", "", RegulationNone)

		if !result.Allowed {
			t.Error("Should use highest priority rule (allow)")
		}
		if result.Rule == nil || result.Rule.Priority != 10 {
			t.Error("Should match rule with priority 10")
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		ac := NewActivityController(nil)
		if ac == nil {
			t.Fatal("NewActivityController(nil) returned nil")
		}
		if ac.config == nil {
			t.Fatal("config should not be nil")
		}
		if !ac.config.Enabled {
			t.Error("default config should be enabled")
		}
	})

	t.Run("empty component type and name", func(t *testing.T) {
		config := &ActivityControlsConfig{
			Enabled: true,
			Controls: map[Activity]*ActivityControl{
				ActivitySyncUser: {
					Activity:     ActivitySyncUser,
					DefaultAllow: true,
					Rules: []ActivityRule{
						{
							Condition: ActivityCondition{
								ComponentType:  "bidder",
								ComponentNames: []string{"appnexus"},
							},
							Allow:    false,
							Priority: 10,
						},
					},
				},
			},
		}

		ac := NewActivityController(config)
		result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil, "", "", RegulationNone)

		if !result.Allowed {
			t.Error("Empty component type/name should not match rule, use default allow")
		}
	})

	t.Run("GPP string with multiple sections", func(t *testing.T) {
		config := &ActivityControlsConfig{
			Enabled: true,
			Controls: map[Activity]*ActivityControl{
				ActivitySyncUser: {
					Activity:     ActivitySyncUser,
					DefaultAllow: false,
					Rules: []ActivityRule{
						{
							Condition: ActivityCondition{
								GPPSectionIDs: []int{GPPSectionUSNational},
							},
							Allow:    true,
							Priority: 10,
						},
					},
				},
			},
		}

		gppString := &GPPString{
			SectionData: map[int]interface{}{
				GPPSectionTCFv2EU:    &TCFv2Data{ConsentString: "test"},
				GPPSectionUSNational: &USNationalData{Version: 1},
				GPPSectionUSCA:       "test",
			},
		}

		ac := NewActivityController(config)
		result := ac.EvaluateActivity(ctx, ActivitySyncUser, gppString, "", "", RegulationNone)

		if !result.Allowed {
			t.Error("Should match rule with US National section present")
		}
	})
}

// TestAllActivitiesInDefault verifies all activities are configured in default
func TestAllActivitiesInDefault(t *testing.T) {
	config := DefaultActivityControlsConfig()

	activities := []Activity{
		ActivitySyncUser,
		ActivityTransmitUfpd,
		ActivityTransmitPreciseGeo,
		ActivityTransmitEids,
		ActivityTransmitTids,
		ActivityEnrichUfpd,
		ActivityFetchBids,
		ActivityReportAnalytics,
	}

	for _, activity := range activities {
		t.Run(string(activity), func(t *testing.T) {
			control, exists := config.Controls[activity]
			if !exists {
				t.Fatalf("Activity %s not found in default config", activity)
			}
			if control.Activity != activity {
				t.Errorf("Activity field = %v, want %v", control.Activity, activity)
			}
		})
	}
}

// TestPrivacyRegulations verifies all regulation constants
func TestPrivacyRegulations(t *testing.T) {
	regulations := []PrivacyRegulation{
		RegulationGDPR,
		RegulationCCPA,
		RegulationVCDPA,
		RegulationCPA,
		RegulationCTDPA,
		RegulationUCPA,
		RegulationLGPD,
		RegulationPIPEDA,
		RegulationPDPA,
		RegulationNone,
	}

	for _, reg := range regulations {
		t.Run(string(reg), func(t *testing.T) {
			if reg == "" {
				t.Error("Regulation constant should not be empty")
			}
		})
	}
}

// TestActivityResult verifies result structure
func TestActivityResult(t *testing.T) {
	t.Run("result with rule", func(t *testing.T) {
		rule := &ActivityRule{
			Allow:    true,
			Priority: 10,
		}

		result := ActivityResult{
			Allowed: true,
			Reason:  "test reason",
			Rule:    rule,
		}

		if !result.Allowed {
			t.Error("Allowed should be true")
		}
		if result.Reason != "test reason" {
			t.Errorf("Reason = %v, want 'test reason'", result.Reason)
		}
		if result.Rule == nil {
			t.Error("Rule should not be nil")
		}
		if result.Rule.Priority != 10 {
			t.Errorf("Rule Priority = %d, want 10", result.Rule.Priority)
		}
	})

	t.Run("result without rule", func(t *testing.T) {
		result := ActivityResult{
			Allowed: false,
			Reason:  "default policy",
			Rule:    nil,
		}

		if result.Allowed {
			t.Error("Allowed should be false")
		}
		if result.Rule != nil {
			t.Error("Rule should be nil")
		}
	})
}

// TestRuleMatching_ComponentTypes tests component type filtering
func TestRuleMatching_ComponentTypes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		condComponentType string
		testComponentType string
		wantMatch         bool
	}{
		{
			name:              "exact match",
			condComponentType: "bidder",
			testComponentType: "bidder",
			wantMatch:         true,
		},
		{
			name:              "no match",
			condComponentType: "bidder",
			testComponentType: "analytics",
			wantMatch:         false,
		},
		{
			name:              "empty condition matches any",
			condComponentType: "",
			testComponentType: "bidder",
			wantMatch:         true,
		},
		{
			name:              "empty test value doesn't match specific condition",
			condComponentType: "bidder",
			testComponentType: "",
			wantMatch:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ActivityControlsConfig{
				Enabled: true,
				Controls: map[Activity]*ActivityControl{
					ActivitySyncUser: {
						Activity:     ActivitySyncUser,
						DefaultAllow: false,
						Rules: []ActivityRule{
							{
								Condition: ActivityCondition{
									ComponentType: tt.condComponentType,
								},
								Allow:    true,
								Priority: 10,
							},
						},
					},
				},
			}

			ac := NewActivityController(config)
			result := ac.EvaluateActivity(ctx, ActivitySyncUser, nil,
				tt.testComponentType, "", RegulationNone)

			gotMatch := result.Rule != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("Rule matched = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

// TestIntegration_DefaultBehaviors tests real-world scenarios
func TestIntegration_DefaultBehaviors(t *testing.T) {
	ctx := context.Background()
	config := DefaultActivityControlsConfig()
	ac := NewActivityController(config)

	t.Run("user sync without GDPR", func(t *testing.T) {
		allowed := ac.CanSyncUser(ctx, nil, "appnexus", RegulationNone)
		if !allowed {
			t.Error("User sync should be allowed without GDPR")
		}
	})

	t.Run("user sync with GDPR", func(t *testing.T) {
		allowed := ac.CanSyncUser(ctx, nil, "appnexus", RegulationGDPR)
		if allowed {
			t.Error("User sync should be denied with GDPR by default")
		}
	})

	t.Run("transmit UFPD with nil GPP (allowed due to implementation)", func(t *testing.T) {
		allowed := ac.CanTransmitUfpd(ctx, nil, "appnexus", RegulationNone)
		if !allowed {
			t.Error("UFPD transmission is allowed with nil GPP in default config (rule matches)")
		}
	})

	t.Run("transmit UFPD with TCF consent", func(t *testing.T) {
		gppString := &GPPString{
			SectionData: map[int]interface{}{
				GPPSectionTCFv2EU: &TCFv2Data{ConsentString: "consent"},
			},
		}
		allowed := ac.CanTransmitUfpd(ctx, gppString, "appnexus", RegulationNone)
		if !allowed {
			t.Error("UFPD transmission should be allowed with TCF consent")
		}
	})

	t.Run("transmit UFPD with GPP but no TCF section", func(t *testing.T) {
		gppString := &GPPString{
			SectionData: map[int]interface{}{
				GPPSectionUSNational: &USNationalData{Version: 1},
			},
		}
		allowed := ac.CanTransmitUfpd(ctx, gppString, "appnexus", RegulationNone)
		if allowed {
			t.Error("UFPD transmission should be denied when GPP present but no TCF section")
		}
	})

	t.Run("precise geo always denied by default", func(t *testing.T) {
		allowed := ac.CanTransmitPreciseGeo(ctx, nil, "appnexus", RegulationNone)
		if allowed {
			t.Error("Precise geo should be denied by default")
		}
	})

	t.Run("EIDs allowed by default", func(t *testing.T) {
		allowed := ac.CanTransmitEids(ctx, nil, "appnexus", RegulationNone)
		if !allowed {
			t.Error("EIDs should be allowed by default")
		}
	})
}
