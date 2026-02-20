package fpd

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestDefaultDSAConfig tests the default DSA configuration
func TestDefaultDSAConfig(t *testing.T) {
	config := DefaultDSAConfig()

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	if !config.Enabled {
		t.Error("expected DSA to be enabled by default")
	}

	if config.DefaultDSARequired != openrtb.DSASupported {
		t.Errorf("DefaultDSARequired = %d, want %d", config.DefaultDSARequired, openrtb.DSASupported)
	}

	if config.DefaultPubRender != openrtb.PubRenderMaybe {
		t.Errorf("DefaultPubRender = %d, want %d", config.DefaultPubRender, openrtb.PubRenderMaybe)
	}

	if config.DefaultDataToPub != openrtb.DataToPubIfPresent {
		t.Errorf("DefaultDataToPub = %d, want %d", config.DefaultDataToPub, openrtb.DataToPubIfPresent)
	}

	if !config.EnforceRequired {
		t.Error("expected EnforceRequired to be true by default")
	}
}

// TestNewDSAProcessor tests DSA processor initialization
func TestNewDSAProcessor(t *testing.T) {
	tests := []struct {
		name   string
		config *DSAConfig
		verify func(t *testing.T, p *DSAProcessor)
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
			verify: func(t *testing.T, p *DSAProcessor) {
				if p == nil {
					t.Fatal("expected non-nil processor")
				}
				if p.config == nil {
					t.Fatal("expected non-nil config")
				}
				if !p.config.Enabled {
					t.Error("expected default config to be enabled")
				}
			},
		},
		{
			name: "custom config is used",
			config: &DSAConfig{
				Enabled:            false,
				DefaultDSARequired: openrtb.DSARequired,
				DefaultPubRender:   openrtb.PubRenderYes,
				DefaultDataToPub:   openrtb.DataToPubAlways,
				EnforceRequired:    false,
			},
			verify: func(t *testing.T, p *DSAProcessor) {
				if p == nil {
					t.Fatal("expected non-nil processor")
				}
				if p.config.Enabled {
					t.Error("expected config to be disabled")
				}
				if p.config.DefaultDSARequired != openrtb.DSARequired {
					t.Errorf("DefaultDSARequired = %d, want %d", p.config.DefaultDSARequired, openrtb.DSARequired)
				}
				if p.config.EnforceRequired {
					t.Error("expected EnforceRequired to be false")
				}
			},
		},
		{
			name: "enabled config with enforcement",
			config: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSARequiredPubRender,
				EnforceRequired:    true,
			},
			verify: func(t *testing.T, p *DSAProcessor) {
				if !p.config.Enabled {
					t.Error("expected config to be enabled")
				}
				if !p.config.EnforceRequired {
					t.Error("expected enforcement to be enabled")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDSAProcessor(tt.config)
			tt.verify(t, p)
		})
	}
}

// TestDSAProcessor_InitializeDSA tests DSA initialization in requests
func TestDSAProcessor_InitializeDSA(t *testing.T) {
	tests := []struct {
		name   string
		config *DSAConfig
		req    *openrtb.BidRequest
		verify func(t *testing.T, req *openrtb.BidRequest)
	}{
		{
			name: "disabled processor does nothing",
			config: &DSAConfig{
				Enabled: false,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			verify: func(t *testing.T, req *openrtb.BidRequest) {
				if req.Regs != nil {
					t.Error("expected Regs to remain nil when disabled")
				}
			},
		},
		{
			name: "initializes Regs when nil",
			config: &DSAConfig{
				Enabled: true,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			verify: func(t *testing.T, req *openrtb.BidRequest) {
				if req.Regs == nil {
					t.Error("expected Regs to be initialized")
				}
			},
		},
		{
			name: "preserves existing Regs",
			config: &DSAConfig{
				Enabled: true,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
				Regs: &openrtb.Regs{
					COPPA: 1,
				},
			},
			verify: func(t *testing.T, req *openrtb.BidRequest) {
				if req.Regs == nil {
					t.Fatal("expected Regs to exist")
				}
				if req.Regs.COPPA != 1 {
					t.Error("expected existing Regs fields to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDSAProcessor(tt.config)
			p.InitializeDSA(tt.req)
			tt.verify(t, tt.req)
		})
	}
}

// TestDSAProcessor_ValidateBidResponseDSA tests bid response validation
func TestDSAProcessor_ValidateBidResponseDSA(t *testing.T) {
	tests := []struct {
		name    string
		config  *DSAConfig
		req     *openrtb.BidRequest
		resp    *openrtb.BidResponse
		wantErr bool
	}{
		{
			name: "disabled processor passes all",
			config: &DSAConfig{
				Enabled: false,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			resp: &openrtb.BidResponse{
				ID: "test-1",
				SeatBid: []openrtb.SeatBid{
					{
						Bid: []openrtb.Bid{
							{ID: "bid-1", ImpID: "imp-1", Price: 1.0},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "enforcement disabled passes all",
			config: &DSAConfig{
				Enabled:         true,
				EnforceRequired: false,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			resp: &openrtb.BidResponse{
				ID: "test-1",
				SeatBid: []openrtb.SeatBid{
					{
						Bid: []openrtb.Bid{
							{ID: "bid-1", ImpID: "imp-1", Price: 1.0},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DSA not required passes without DSA",
			config: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSANotRequired,
				EnforceRequired:    true,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			resp: &openrtb.BidResponse{
				ID: "test-1",
				SeatBid: []openrtb.SeatBid{
					{
						Bid: []openrtb.Bid{
							{ID: "bid-1", ImpID: "imp-1", Price: 1.0},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DSA supported passes without DSA",
			config: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSASupported,
				EnforceRequired:    true,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			resp: &openrtb.BidResponse{
				ID: "test-1",
				SeatBid: []openrtb.SeatBid{
					{
						Bid: []openrtb.Bid{
							{ID: "bid-1", ImpID: "imp-1", Price: 1.0},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty seatbid passes",
			config: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSARequired,
				EnforceRequired:    true,
			},
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			resp: &openrtb.BidResponse{
				ID:      "test-1",
				SeatBid: []openrtb.SeatBid{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDSAProcessor(tt.config)
			err := p.ValidateBidResponseDSA(tt.req, tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBidResponseDSA() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDSAProcessor_ProcessDSATransparency tests transparency processing for targeting
func TestDSAProcessor_ProcessDSATransparency(t *testing.T) {
	tests := []struct {
		name     string
		config   *DSAConfig
		bid      *openrtb.Bid
		dsa      *openrtb.DSA
		expected map[string]string
	}{
		{
			name: "disabled processor returns empty",
			config: &DSAConfig{
				Enabled: false,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DSARequired: openrtb.DSARequired,
				Transparency: []openrtb.DSATransparency{
					{Domain: "example.com", DSAParams: []int{1}},
				},
			},
			expected: map[string]string{},
		},
		{
			name: "nil DSA returns empty",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa:      nil,
			expected: map[string]string{},
		},
		{
			name: "DataToPubNo returns empty",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub: openrtb.DataToPubNo,
				Transparency: []openrtb.DSATransparency{
					{Domain: "example.com", DSAParams: []int{1}},
				},
			},
			expected: map[string]string{},
		},
		{
			name: "DataToPubIfPresent without transparency returns empty",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub:    openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{},
			},
			expected: map[string]string{},
		},
		{
			name: "basic transparency info",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1}},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "advertiser.com",
				"hb_dsa_params": "1",
			},
		},
		{
			name: "multiple DSA params",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub: openrtb.DataToPubAlways,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1, 2, 3}},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "advertiser.com",
				"hb_dsa_params": "1,2,3",
			},
		},
		{
			name: "transparency without params",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com"},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "advertiser.com",
			},
		},
		{
			name: "publisher should render",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				PubRender: openrtb.PubRenderYes,
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1}},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "advertiser.com",
				"hb_dsa_params": "1",
				"hb_dsa_render": "1",
			},
		},
		{
			name: "DSARequiredPubRender implies render",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DSARequired: openrtb.DSARequiredPubRender,
				PubRender:   openrtb.PubRenderNo,
				DataToPub:   openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{2}},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "advertiser.com",
				"hb_dsa_params": "2",
				"hb_dsa_render": "1",
			},
		},
		{
			name: "multiple transparency entries uses first",
			config: &DSAConfig{
				Enabled: true,
			},
			bid: &openrtb.Bid{
				ID: "bid-1",
			},
			dsa: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "first.com", DSAParams: []int{1}},
					{Domain: "second.com", DSAParams: []int{2}},
				},
			},
			expected: map[string]string{
				"hb_dsa_domain": "first.com",
				"hb_dsa_params": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDSAProcessor(tt.config)
			result := p.ProcessDSATransparency(tt.bid, tt.dsa)

			if len(result) != len(tt.expected) {
				t.Errorf("got %d targeting keys, want %d", len(result), len(tt.expected))
			}

			for key, expectedVal := range tt.expected {
				if result[key] != expectedVal {
					t.Errorf("targeting[%s] = %s, want %s", key, result[key], expectedVal)
				}
			}

			for key := range result {
				if _, ok := tt.expected[key]; !ok {
					t.Errorf("unexpected targeting key: %s", key)
				}
			}
		})
	}
}

// TestIsDSAApplicable tests EU/EEA country detection
func TestIsDSAApplicable(t *testing.T) {
	tests := []struct {
		name string
		req  *openrtb.BidRequest
		want bool
	}{
		{
			name: "no geo information",
			req: &openrtb.BidRequest{
				ID: "test-1",
			},
			want: false,
		},
		{
			name: "device geo from EU country (Germany)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: "DEU",
					},
				},
			},
			want: true,
		},
		{
			name: "device geo from non-EU country (USA)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: "USA",
					},
				},
			},
			want: false,
		},
		{
			name: "user geo from EU country (France)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "FRA",
					},
				},
			},
			want: true,
		},
		{
			name: "user geo from EEA country (Iceland)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "ISL",
					},
				},
			},
			want: true,
		},
		{
			name: "user geo from EEA country (Norway)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "NOR",
					},
				},
			},
			want: true,
		},
		{
			name: "user geo from EEA country (Liechtenstein)",
			req: &openrtb.BidRequest{
				ID: "test-1",
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "LIE",
					},
				},
			},
			want: true,
		},
		{
			name: "device geo takes precedence",
			req: &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: "DEU",
					},
				},
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "USA",
					},
				},
			},
			want: true,
		},
		{
			name: "fallback to user geo",
			req: &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: "USA",
					},
				},
				User: &openrtb.User{
					Geo: &openrtb.Geo{
						Country: "ITA",
					},
				},
			},
			want: true,
		},
		{
			name: "empty country code",
			req: &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: "",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDSAApplicable(tt.req)
			if got != tt.want {
				t.Errorf("IsDSAApplicable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDSACountries tests all EU and EEA countries are included
func TestDSACountries(t *testing.T) {
	// EU Member States (27)
	euCountries := []string{
		"AUT", "BEL", "BGR", "HRV", "CYP", "CZE", "DNK", "EST", "FIN", "FRA",
		"DEU", "GRC", "HUN", "IRL", "ITA", "LVA", "LTU", "LUX", "MLT", "NLD",
		"POL", "PRT", "ROU", "SVK", "SVN", "ESP", "SWE",
	}

	// EEA (non-EU) countries (3)
	eeaCountries := []string{
		"ISL", "LIE", "NOR",
	}

	// Verify all EU countries are present
	for _, country := range euCountries {
		if !DSACountries[country] {
			t.Errorf("EU country %s not found in DSACountries", country)
		}
	}

	// Verify all EEA countries are present
	for _, country := range eeaCountries {
		if !DSACountries[country] {
			t.Errorf("EEA country %s not found in DSACountries", country)
		}
	}

	// Verify total count (27 EU + 3 EEA = 30)
	if len(DSACountries) != 30 {
		t.Errorf("DSACountries has %d entries, expected 30 (27 EU + 3 EEA)", len(DSACountries))
	}

	// Verify non-EU/EEA countries are not present
	nonDSACountries := []string{
		"USA", "CAN", "GBR", "CHE", "AUS", "JPN", "CHN", "RUS", "BRA", "IND",
	}
	for _, country := range nonDSACountries {
		if DSACountries[country] {
			t.Errorf("Non-DSA country %s should not be in DSACountries", country)
		}
	}
}

// TestDSACountriesComprehensive tests each EU/EEA country individually
func TestDSACountriesComprehensive(t *testing.T) {
	tests := []struct {
		country string
		inDSA   bool
		region  string
	}{
		// EU Countries
		{"AUT", true, "EU - Austria"},
		{"BEL", true, "EU - Belgium"},
		{"BGR", true, "EU - Bulgaria"},
		{"HRV", true, "EU - Croatia"},
		{"CYP", true, "EU - Cyprus"},
		{"CZE", true, "EU - Czech Republic"},
		{"DNK", true, "EU - Denmark"},
		{"EST", true, "EU - Estonia"},
		{"FIN", true, "EU - Finland"},
		{"FRA", true, "EU - France"},
		{"DEU", true, "EU - Germany"},
		{"GRC", true, "EU - Greece"},
		{"HUN", true, "EU - Hungary"},
		{"IRL", true, "EU - Ireland"},
		{"ITA", true, "EU - Italy"},
		{"LVA", true, "EU - Latvia"},
		{"LTU", true, "EU - Lithuania"},
		{"LUX", true, "EU - Luxembourg"},
		{"MLT", true, "EU - Malta"},
		{"NLD", true, "EU - Netherlands"},
		{"POL", true, "EU - Poland"},
		{"PRT", true, "EU - Portugal"},
		{"ROU", true, "EU - Romania"},
		{"SVK", true, "EU - Slovakia"},
		{"SVN", true, "EU - Slovenia"},
		{"ESP", true, "EU - Spain"},
		{"SWE", true, "EU - Sweden"},

		// EEA (non-EU)
		{"ISL", true, "EEA - Iceland"},
		{"LIE", true, "EEA - Liechtenstein"},
		{"NOR", true, "EEA - Norway"},

		// Non-DSA countries
		{"USA", false, "USA"},
		{"GBR", false, "UK (post-Brexit)"},
		{"CHE", false, "Switzerland"},
		{"CAN", false, "Canada"},
		{"AUS", false, "Australia"},
		{"JPN", false, "Japan"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := DSACountries[tt.country]
			if got != tt.inDSA {
				t.Errorf("DSACountries[%s] = %v, want %v", tt.country, got, tt.inDSA)
			}
		})
	}
}

// TestDSAProcessor_IntegrationScenarios tests realistic DSA processing scenarios
func TestDSAProcessor_IntegrationScenarios(t *testing.T) {
	tests := []struct {
		name            string
		country         string
		dsaConfig       *DSAConfig
		bidDSA          *openrtb.DSA
		expectTargeting bool
		expectRender    bool
	}{
		{
			name:    "EU user with required DSA",
			country: "DEU",
			dsaConfig: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSARequired,
				EnforceRequired:    true,
			},
			bidDSA: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1}},
				},
			},
			expectTargeting: true,
			expectRender:    false,
		},
		{
			name:    "EU user with publisher rendering",
			country: "FRA",
			dsaConfig: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSARequiredPubRender,
				EnforceRequired:    true,
			},
			bidDSA: &openrtb.DSA{
				DSARequired: openrtb.DSARequiredPubRender,
				DataToPub:   openrtb.DataToPubAlways,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1, 2}},
				},
			},
			expectTargeting: true,
			expectRender:    true,
		},
		{
			name:    "non-EU user",
			country: "USA",
			dsaConfig: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSANotRequired,
				EnforceRequired:    false,
			},
			bidDSA: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{1}},
				},
			},
			expectTargeting: true,
			expectRender:    false,
		},
		{
			name:    "EEA country (Norway)",
			country: "NOR",
			dsaConfig: &DSAConfig{
				Enabled:            true,
				DefaultDSARequired: openrtb.DSARequired,
				EnforceRequired:    true,
			},
			bidDSA: &openrtb.DSA{
				DataToPub: openrtb.DataToPubIfPresent,
				Transparency: []openrtb.DSATransparency{
					{Domain: "advertiser.com", DSAParams: []int{3}},
				},
			},
			expectTargeting: true,
			expectRender:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if country is DSA-applicable
			req := &openrtb.BidRequest{
				ID: "test-1",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{
						Country: tt.country,
					},
				},
			}

			isDSA := IsDSAApplicable(req)
			if tt.country == "USA" && isDSA {
				t.Error("USA should not be DSA-applicable")
			}
			if (tt.country == "DEU" || tt.country == "FRA" || tt.country == "NOR") && !isDSA {
				t.Errorf("%s should be DSA-applicable", tt.country)
			}

			// Process transparency
			p := NewDSAProcessor(tt.dsaConfig)
			bid := &openrtb.Bid{ID: "bid-1", ImpID: "imp-1", Price: 1.0}
			targeting := p.ProcessDSATransparency(bid, tt.bidDSA)

			if tt.expectTargeting {
				if len(targeting) == 0 {
					t.Error("expected targeting keys but got none")
				}
				if _, ok := targeting["hb_dsa_domain"]; !ok {
					t.Error("expected hb_dsa_domain in targeting")
				}
			}

			if tt.expectRender {
				if targeting["hb_dsa_render"] != "1" {
					t.Error("expected hb_dsa_render=1 in targeting")
				}
			} else {
				if targeting["hb_dsa_render"] == "1" {
					t.Error("did not expect hb_dsa_render in targeting")
				}
			}
		})
	}
}
