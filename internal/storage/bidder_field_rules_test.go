package storage_test

import (
	"context"
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

// Integration test stub — skipped unless DB is available.
func TestGetBidderFieldRules_Integration(t *testing.T) {
	t.Skip("requires live DB — run manually or in CI with DB service")
	_ = context.Background()
}
