package openrtb

import (
	"encoding/json"
	"testing"
)

// TestValidationError_Error tests the ValidationError error string
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "DSARequired error",
			err:      &ValidationError{Field: "dsa.dsarequired", Message: "must be 0-3"},
			expected: "dsa.dsarequired: must be 0-3",
		},
		{
			name:     "PubRender error",
			err:      &ValidationError{Field: "dsa.pubrender", Message: "must be 0-2"},
			expected: "dsa.pubrender: must be 0-2",
		},
		{
			name:     "Custom error",
			err:      &ValidationError{Field: "custom.field", Message: "custom message"},
			expected: "custom.field: custom message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestDSA_ValidateDSA tests DSA validation
func TestDSA_ValidateDSA(t *testing.T) {
	tests := []struct {
		name    string
		dsa     *DSA
		wantErr *ValidationError
	}{
		{
			name:    "nil DSA is valid",
			dsa:     nil,
			wantErr: nil,
		},
		{
			name: "valid DSA with all fields in range",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderYes,
				DataToPub:   DataToPubIfPresent,
				Transparency: []DSATransparency{
					{
						Domain:    "advertiser.com",
						DSAParams: []int{DSAParamPaidForBy},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid DSA with minimum values",
			dsa: &DSA{
				DSARequired: DSANotRequired,
				PubRender:   PubRenderNo,
				DataToPub:   DataToPubNo,
			},
			wantErr: nil,
		},
		{
			name: "valid DSA with maximum values",
			dsa: &DSA{
				DSARequired: DSARequiredPubRender,
				PubRender:   PubRenderMaybe,
				DataToPub:   DataToPubAlways,
			},
			wantErr: nil,
		},
		{
			name: "invalid DSARequired - negative",
			dsa: &DSA{
				DSARequired: -1,
				PubRender:   PubRenderNo,
				DataToPub:   DataToPubNo,
			},
			wantErr: ErrInvalidDSARequired,
		},
		{
			name: "invalid DSARequired - too high",
			dsa: &DSA{
				DSARequired: 4,
				PubRender:   PubRenderNo,
				DataToPub:   DataToPubNo,
			},
			wantErr: ErrInvalidDSARequired,
		},
		{
			name: "invalid PubRender - negative",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   -1,
				DataToPub:   DataToPubNo,
			},
			wantErr: ErrInvalidPubRender,
		},
		{
			name: "invalid PubRender - too high",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   3,
				DataToPub:   DataToPubNo,
			},
			wantErr: ErrInvalidPubRender,
		},
		{
			name: "invalid DataToPub - negative",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderNo,
				DataToPub:   -1,
			},
			wantErr: ErrInvalidDataToPub,
		},
		{
			name: "invalid DataToPub - too high",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderNo,
				DataToPub:   3,
			},
			wantErr: ErrInvalidDataToPub,
		},
		{
			name: "missing domain in transparency",
			dsa: &DSA{
				DSARequired: DSARequired,
				PubRender:   PubRenderNo,
				DataToPub:   DataToPubNo,
				Transparency: []DSATransparency{
					{
						Domain:    "",
						DSAParams: []int{DSAParamPaidForBy},
					},
				},
			},
			wantErr: ErrMissingDSADomain,
		},
		{
			name: "valid multiple transparency entries",
			dsa: &DSA{
				DSARequired: DSARequired,
				PubRender:   PubRenderYes,
				DataToPub:   DataToPubAlways,
				Transparency: []DSATransparency{
					{
						Domain:    "advertiser1.com",
						DSAParams: []int{DSAParamPaidForBy},
					},
					{
						Domain:    "advertiser2.com",
						DSAParams: []int{DSAParamOnBehalfOf},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "empty transparency array is valid",
			dsa: &DSA{
				DSARequired:  DSASupported,
				PubRender:    PubRenderNo,
				DataToPub:    DataToPubNo,
				Transparency: []DSATransparency{},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dsa.ValidateDSA()
			if (err == nil) != (tt.wantErr == nil) {
				t.Errorf("ValidateDSA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErr != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateDSA() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

// TestDSA_IsDSARequired tests the IsDSARequired method
func TestDSA_IsDSARequired(t *testing.T) {
	tests := []struct {
		name string
		dsa  *DSA
		want bool
	}{
		{
			name: "nil DSA",
			dsa:  nil,
			want: false,
		},
		{
			name: "DSANotRequired (0)",
			dsa: &DSA{
				DSARequired: DSANotRequired,
			},
			want: false,
		},
		{
			name: "DSASupported (1)",
			dsa: &DSA{
				DSARequired: DSASupported,
			},
			want: false,
		},
		{
			name: "DSARequired (2)",
			dsa: &DSA{
				DSARequired: DSARequired,
			},
			want: true,
		},
		{
			name: "DSARequiredPubRender (3)",
			dsa: &DSA{
				DSARequired: DSARequiredPubRender,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dsa.IsDSARequired()
			if got != tt.want {
				t.Errorf("IsDSARequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDSA_ShouldPublisherRender tests the ShouldPublisherRender method
func TestDSA_ShouldPublisherRender(t *testing.T) {
	tests := []struct {
		name string
		dsa  *DSA
		want bool
	}{
		{
			name: "nil DSA",
			dsa:  nil,
			want: false,
		},
		{
			name: "PubRenderNo",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderNo,
			},
			want: false,
		},
		{
			name: "PubRenderYes",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderYes,
			},
			want: true,
		},
		{
			name: "PubRenderMaybe",
			dsa: &DSA{
				DSARequired: DSASupported,
				PubRender:   PubRenderMaybe,
			},
			want: false,
		},
		{
			name: "DSARequiredPubRender with PubRenderNo",
			dsa: &DSA{
				DSARequired: DSARequiredPubRender,
				PubRender:   PubRenderNo,
			},
			want: true,
		},
		{
			name: "DSARequiredPubRender with PubRenderYes",
			dsa: &DSA{
				DSARequired: DSARequiredPubRender,
				PubRender:   PubRenderYes,
			},
			want: true,
		},
		{
			name: "DSARequired with PubRenderNo",
			dsa: &DSA{
				DSARequired: DSARequired,
				PubRender:   PubRenderNo,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dsa.ShouldPublisherRender()
			if got != tt.want {
				t.Errorf("ShouldPublisherRender() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDSA_ShouldSendDataToPub tests the ShouldSendDataToPub method
func TestDSA_ShouldSendDataToPub(t *testing.T) {
	tests := []struct {
		name            string
		dsa             *DSA
		hasTransparency bool
		want            bool
	}{
		{
			name:            "nil DSA",
			dsa:             nil,
			hasTransparency: true,
			want:            false,
		},
		{
			name: "DataToPubNo with transparency",
			dsa: &DSA{
				DataToPub: DataToPubNo,
			},
			hasTransparency: true,
			want:            false,
		},
		{
			name: "DataToPubNo without transparency",
			dsa: &DSA{
				DataToPub: DataToPubNo,
			},
			hasTransparency: false,
			want:            false,
		},
		{
			name: "DataToPubIfPresent with transparency",
			dsa: &DSA{
				DataToPub: DataToPubIfPresent,
			},
			hasTransparency: true,
			want:            true,
		},
		{
			name: "DataToPubIfPresent without transparency",
			dsa: &DSA{
				DataToPub: DataToPubIfPresent,
			},
			hasTransparency: false,
			want:            false,
		},
		{
			name: "DataToPubAlways with transparency",
			dsa: &DSA{
				DataToPub: DataToPubAlways,
			},
			hasTransparency: true,
			want:            true,
		},
		{
			name: "DataToPubAlways without transparency",
			dsa: &DSA{
				DataToPub: DataToPubAlways,
			},
			hasTransparency: false,
			want:            true,
		},
		{
			name: "invalid DataToPub value",
			dsa: &DSA{
				DataToPub: 99,
			},
			hasTransparency: true,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dsa.ShouldSendDataToPub(tt.hasTransparency)
			if got != tt.want {
				t.Errorf("ShouldSendDataToPub(%v) = %v, want %v", tt.hasTransparency, got, tt.want)
			}
		})
	}
}

// TestDSA_JSONMarshaling tests JSON marshaling and unmarshaling
func TestDSA_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name    string
		dsa     *DSA
		jsonStr string
	}{
		{
			name: "full DSA object",
			dsa: &DSA{
				DSARequired: DSARequired,
				PubRender:   PubRenderYes,
				DataToPub:   DataToPubIfPresent,
				Transparency: []DSATransparency{
					{
						Domain:    "advertiser.com",
						DSAParams: []int{DSAParamPaidForBy, DSAParamOnBehalfOf},
					},
				},
			},
			jsonStr: `{"dsarequired":2,"pubrender":1,"datatopub":1,"transparency":[{"domain":"advertiser.com","dsaparams":[1,2]}]}`,
		},
		{
			name: "minimal DSA object with non-zero value",
			dsa: &DSA{
				DSARequired: DSASupported,
			},
			jsonStr: `{"dsarequired":1}`,
		},
		{
			name: "DSA with multiple transparency entries",
			dsa: &DSA{
				DSARequired: DSARequiredPubRender,
				PubRender:   PubRenderMaybe,
				DataToPub:   DataToPubAlways,
				Transparency: []DSATransparency{
					{
						Domain:    "advertiser1.com",
						DSAParams: []int{DSAParamPaidForBy},
					},
					{
						Domain:    "advertiser2.com",
						DSAParams: []int{DSAParamOnBehalfOf},
					},
				},
			},
			jsonStr: `{"dsarequired":3,"pubrender":2,"datatopub":2,"transparency":[{"domain":"advertiser1.com","dsaparams":[1]},{"domain":"advertiser2.com","dsaparams":[2]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.dsa)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			if string(data) != tt.jsonStr {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.jsonStr)
			}

			// Test unmarshaling
			var decoded DSA
			if err := json.Unmarshal([]byte(tt.jsonStr), &decoded); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if decoded.DSARequired != tt.dsa.DSARequired {
				t.Errorf("DSARequired = %d, want %d", decoded.DSARequired, tt.dsa.DSARequired)
			}
			if decoded.PubRender != tt.dsa.PubRender {
				t.Errorf("PubRender = %d, want %d", decoded.PubRender, tt.dsa.PubRender)
			}
			if decoded.DataToPub != tt.dsa.DataToPub {
				t.Errorf("DataToPub = %d, want %d", decoded.DataToPub, tt.dsa.DataToPub)
			}
			if len(decoded.Transparency) != len(tt.dsa.Transparency) {
				t.Errorf("Transparency length = %d, want %d", len(decoded.Transparency), len(tt.dsa.Transparency))
			}
		})
	}
}

// TestDSA_OmitEmptyFields tests that empty fields are omitted in JSON
func TestDSA_OmitEmptyFields(t *testing.T) {
	dsa := &DSA{
		DSARequired: 0, // Explicitly set to 0
	}

	data, err := json.Marshal(dsa)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Since dsarequired is 0 (the zero value), it should be omitted due to omitempty
	expected := `{}`
	if string(data) != expected {
		t.Errorf("Marshal() = %s, want %s", string(data), expected)
	}
}

// TestDSAConstants tests that DSA constants have expected values
func TestDSAConstants(t *testing.T) {
	// DSARequired values
	if DSANotRequired != 0 {
		t.Errorf("DSANotRequired = %d, want 0", DSANotRequired)
	}
	if DSASupported != 1 {
		t.Errorf("DSASupported = %d, want 1", DSASupported)
	}
	if DSARequired != 2 {
		t.Errorf("DSARequired = %d, want 2", DSARequired)
	}
	if DSARequiredPubRender != 3 {
		t.Errorf("DSARequiredPubRender = %d, want 3", DSARequiredPubRender)
	}

	// PubRender values
	if PubRenderNo != 0 {
		t.Errorf("PubRenderNo = %d, want 0", PubRenderNo)
	}
	if PubRenderYes != 1 {
		t.Errorf("PubRenderYes = %d, want 1", PubRenderYes)
	}
	if PubRenderMaybe != 2 {
		t.Errorf("PubRenderMaybe = %d, want 2", PubRenderMaybe)
	}

	// DataToPub values
	if DataToPubNo != 0 {
		t.Errorf("DataToPubNo = %d, want 0", DataToPubNo)
	}
	if DataToPubIfPresent != 1 {
		t.Errorf("DataToPubIfPresent = %d, want 1", DataToPubIfPresent)
	}
	if DataToPubAlways != 2 {
		t.Errorf("DataToPubAlways = %d, want 2", DataToPubAlways)
	}

	// DSAParam values
	if DSAParamNotApplicable != 0 {
		t.Errorf("DSAParamNotApplicable = %d, want 0", DSAParamNotApplicable)
	}
	if DSAParamPaidForBy != 1 {
		t.Errorf("DSAParamPaidForBy = %d, want 1", DSAParamPaidForBy)
	}
	if DSAParamOnBehalfOf != 2 {
		t.Errorf("DSAParamOnBehalfOf = %d, want 2", DSAParamOnBehalfOf)
	}
	if DSAParamPaidForByAndOnBehalfOf != 3 {
		t.Errorf("DSAParamPaidForByAndOnBehalfOf = %d, want 3", DSAParamPaidForByAndOnBehalfOf)
	}
}

// TestDSATransparency_JSONMarshaling tests transparency object marshaling
func TestDSATransparency_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name  string
		trans DSATransparency
		json  string
	}{
		{
			name: "full transparency",
			trans: DSATransparency{
				Domain:    "example.com",
				DSAParams: []int{1, 2, 3},
			},
			json: `{"domain":"example.com","dsaparams":[1,2,3]}`,
		},
		{
			name: "domain only",
			trans: DSATransparency{
				Domain: "example.com",
			},
			json: `{"domain":"example.com"}`,
		},
		{
			name: "empty params array",
			trans: DSATransparency{
				Domain:    "example.com",
				DSAParams: []int{},
			},
			json: `{"domain":"example.com"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.trans)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			if string(data) != tt.json {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.json)
			}

			var decoded DSATransparency
			if err := json.Unmarshal([]byte(tt.json), &decoded); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if decoded.Domain != tt.trans.Domain {
				t.Errorf("Domain = %s, want %s", decoded.Domain, tt.trans.Domain)
			}
		})
	}
}

// TestDSAErrors tests the predefined error variables
func TestDSAErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expField string
		expMsg   string
	}{
		{
			name:     "ErrInvalidDSARequired",
			err:      ErrInvalidDSARequired,
			expField: "dsa.dsarequired",
			expMsg:   "must be 0-3",
		},
		{
			name:     "ErrInvalidPubRender",
			err:      ErrInvalidPubRender,
			expField: "dsa.pubrender",
			expMsg:   "must be 0-2",
		},
		{
			name:     "ErrInvalidDataToPub",
			err:      ErrInvalidDataToPub,
			expField: "dsa.datatopub",
			expMsg:   "must be 0-2",
		},
		{
			name:     "ErrMissingDSADomain",
			err:      ErrMissingDSADomain,
			expField: "dsa.transparency.domain",
			expMsg:   "required",
		},
		{
			name:     "ErrDSARequired",
			err:      ErrDSARequired,
			expField: "dsa",
			expMsg:   "DSA transparency required but not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Field != tt.expField {
				t.Errorf("Field = %s, want %s", tt.err.Field, tt.expField)
			}
			if tt.err.Message != tt.expMsg {
				t.Errorf("Message = %s, want %s", tt.err.Message, tt.expMsg)
			}
		})
	}
}
