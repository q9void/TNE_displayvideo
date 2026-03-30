package storage_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// These are unit tests against the struct and method signatures only.
// Integration tests against a live DB are in the CI pipeline.

func TestBidderFieldRule_IsDefault(t *testing.T) {
	r := storage.BidderFieldRule{BidderCode: "__default__"}
	if !r.IsDefault() {
		t.Error("expected IsDefault() true for __default__")
	}
	r2 := storage.BidderFieldRule{BidderCode: "kargo"}
	if r2.IsDefault() {
		t.Error("expected IsDefault() false for kargo")
	}
}

func TestBidderFieldRule_Validate(t *testing.T) {
	cases := []struct {
		name    string
		rule    storage.BidderFieldRule
		wantErr bool
	}{
		{
			name:    "standard with no source_ref is valid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.eids", SourceType: "standard"},
			wantErr: false,
		},
		{
			name:    "slot_param with no source_ref is invalid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.buyeruid", SourceType: "slot_param"},
			wantErr: true,
		},
		{
			name:    "eid with source_ref is valid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "user.buyeruid", SourceType: "eid", SourceRef: strPtr("kargo.com")},
			wantErr: false,
		},
		{
			name:    "unknown source_type is invalid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", FieldPath: "x", SourceType: "magic"},
			wantErr: true,
		},
		{
			name:    "empty field_path is invalid",
			rule:    storage.BidderFieldRule{BidderCode: "kargo", SourceType: "standard"},
			wantErr: true,
		},
		{
			name:    "empty bidder_code is invalid",
			rule:    storage.BidderFieldRule{FieldPath: "x", SourceType: "standard"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
