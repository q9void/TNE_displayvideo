package routing_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

func TestComposer_SetsSlotParam(t *testing.T) {
	rules := []storage.BidderFieldRule{
		{
			BidderCode: "kargo",
			FieldPath:  "imp.ext.kargo.placementId",
			SourceType: "slot_param",
			SourceRef:  strRef("placementId"),
			Transform:  "none",
			Required:   true,
			Enabled:    true,
		},
	}
	c := routing.NewComposer(rules)
	req := &openrtb.BidRequest{
		Imp: []openrtb.Imp{{ID: "1"}},
	}
	slotParams := map[string]interface{}{"placementId": "_abc123"}
	result, errs := c.Apply("kargo", req, slotParams, nil, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result == nil || len(result.Imp) == 0 {
		t.Fatal("expected imp in result")
	}
	if result.Imp[0].Ext == nil {
		t.Fatal("expected imp[0].Ext to be set")
	}
}

func TestComposer_EIDtoBuyerUID(t *testing.T) {
	rules := []storage.BidderFieldRule{
		{
			BidderCode: "kargo",
			FieldPath:  "user.buyeruid",
			SourceType: "eid",
			SourceRef:  strRef("kargo.com"),
			Transform:  "none",
			Enabled:    true,
		},
	}
	c := routing.NewComposer(rules)
	req := &openrtb.BidRequest{
		User: &openrtb.User{
			EIDs: []openrtb.EID{
				{Source: "kargo.com", UIDs: []openrtb.UID{{ID: "k-buyer-xyz"}}},
			},
		},
	}
	result, errs := c.Apply("kargo", req, nil, nil, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result.User == nil || result.User.BuyerUID != "k-buyer-xyz" {
		t.Errorf("expected BuyerUID = k-buyer-xyz, got %v", result.User)
	}
}
