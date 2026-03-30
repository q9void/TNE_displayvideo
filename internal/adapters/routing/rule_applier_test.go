package routing_test

import (
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

func strRef(s string) *string { return &s }

func TestApplyTransform_ToInt(t *testing.T) {
	got, err := routing.ApplyTransform("42", "to_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("expected int 42, got %v (%T)", got, got)
	}
}

func TestApplyTransform_ToString(t *testing.T) {
	got, err := routing.ApplyTransform(123, "to_string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "123" {
		t.Errorf("expected string \"123\", got %v (%T)", got, got)
	}
}

func TestApplyTransform_None(t *testing.T) {
	got, err := routing.ApplyTransform("hello", "none")
	if err != nil || got != "hello" {
		t.Errorf("none transform should be a pass-through, got %v err %v", got, err)
	}
}

func TestRuleApplier_SlotParam(t *testing.T) {
	rule := storage.BidderFieldRule{
		SourceType: "slot_param",
		SourceRef:  strRef("placementId"),
		Transform:  "none",
	}
	slotParams := map[string]interface{}{"placementId": "_o9n8eh8Lsw"}
	got, err := routing.ApplyRule(rule, nil, slotParams, nil, nil)
	if err != nil || got != "_o9n8eh8Lsw" {
		t.Errorf("expected placementId, got %v err %v", got, err)
	}
}

func TestRuleApplier_Constant(t *testing.T) {
	rule := storage.BidderFieldRule{
		SourceType: "constant",
		SourceRef:  strRef("1"),
		Transform:  "to_int",
	}
	got, err := routing.ApplyRule(rule, nil, nil, nil, nil)
	if err != nil || got != 1 {
		t.Errorf("expected constant int 1, got %v err %v", got, err)
	}
}
