package privacy

import (
	"testing"
)

func TestParseGPPString(t *testing.T) {
	tests := []struct {
		name    string
		gpp     string
		wantErr bool
	}{
		{
			name:    "empty string",
			gpp:     "",
			wantErr: true,
		},
		{
			name:    "valid GPP with TCF v2",
			gpp:     "DBABMA~CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA~1YNN",
			wantErr: false,
		},
		{
			name:    "simple header only",
			gpp:     "DBABMA",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGPPString(tt.gpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGPPString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("ParseGPPString() returned nil without error")
			}
		})
	}
}

func TestGPPString_HasSection(t *testing.T) {
	gpp := &GPPString{
		SectionData: map[int]interface{}{
			GPPSectionTCFv2EU:    &TCFv2Data{ConsentString: "test"},
			GPPSectionUSNational: &USNationalData{Version: 1},
		},
	}

	tests := []struct {
		name      string
		sectionID int
		want      bool
	}{
		{"TCF v2 present", GPPSectionTCFv2EU, true},
		{"US National present", GPPSectionUSNational, true},
		{"US CA not present", GPPSectionUSCA, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gpp.HasSection(tt.sectionID); got != tt.want {
				t.Errorf("HasSection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGPPString_IsOptedOutOfSale(t *testing.T) {
	tests := []struct {
		name string
		gpp  *GPPString
		want bool
	}{
		{
			name: "US National opted out",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSNational: &USNationalData{
						SaleOptOut: 1,
					},
				},
			},
			want: true,
		},
		{
			name: "US National not opted out",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSNational: &USNationalData{
						SaleOptOut: 2,
					},
				},
			},
			want: false,
		},
		{
			name: "US Privacy opted out (1YYN)",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSPrivacy: "1YYN",
				},
			},
			want: true,
		},
		{
			name: "US Privacy not opted out (1YNN)",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSPrivacy: "1YNN",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gpp.IsOptedOutOfSale(); got != tt.want {
				t.Errorf("IsOptedOutOfSale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGPPString_RequiresConsent(t *testing.T) {
	tests := []struct {
		name string
		gpp  *GPPString
		want bool
	}{
		{
			name: "has TCF v2",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionTCFv2EU: &TCFv2Data{},
				},
			},
			want: true,
		},
		{
			name: "has US National",
			gpp: &GPPString{
				SectionData: map[int]interface{}{
					GPPSectionUSNational: &USNationalData{},
				},
			},
			want: true,
		},
		{
			name: "no sections",
			gpp: &GPPString{
				SectionData: map[int]interface{}{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gpp.RequiresConsent(); got != tt.want {
				t.Errorf("RequiresConsent() = %v, want %v", got, tt.want)
			}
		})
	}
}
