package hooks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestSChainAugmentationHook_CreatesNewSChain(t *testing.T) {
	hook := NewSChainAugmentationHook("thenexusengine.com", "12345")
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		// No source object
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create source object
	if req.Source == nil {
		t.Fatal("expected source object to be created")
	}

	// Should write schain to typed field
	if req.Source.SChain == nil {
		t.Fatal("expected source.SChain to be created")
	}

	// source.ext should not contain schain key
	if req.Source.Ext != nil {
		var extMap map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &extMap); err == nil {
			if _, hasSchain := extMap["schain"]; hasSchain {
				t.Error("expected schain key to be absent from source.ext")
			}
		}
	}

	// Verify schain structure
	schain := req.Source.SChain
	if schain.Ver != "1.0" {
		t.Errorf("expected schain version 1.0, got: %s", schain.Ver)
	}
	if schain.Complete != 1 {
		t.Errorf("expected schain complete=1, got: %d", schain.Complete)
	}
	if len(schain.Nodes) != 1 {
		t.Fatalf("expected 1 node, got: %d", len(schain.Nodes))
	}

	// Verify platform node
	node := schain.Nodes[0]
	if node.ASI != "thenexusengine.com" {
		t.Errorf("expected ASI 'thenexusengine.com', got: %s", node.ASI)
	}
	if node.SID != "12345" {
		t.Errorf("expected SID '12345', got: %s", node.SID)
	}
	if node.HP != 1 {
		t.Errorf("expected HP=1 (direct seller), got: %d", node.HP)
	}
	if node.RID != "test-request-123" {
		t.Errorf("expected RID to match request.id, got: %s", node.RID)
	}
}

func TestSChainAugmentationHook_AppendsToExistingSChain(t *testing.T) {
	hook := NewSChainAugmentationHook("thenexusengine.com", "12345")
	ctx := context.Background()

	// Create existing schain in typed field
	existingSChain := openrtb.SupplyChain{
		Ver:      "1.0",
		Complete: 0, // Not complete yet
		Nodes: []openrtb.SupplyChainNode{
			{
				ASI:    "publisher.com",
				SID:    "pub-123",
				HP:     1,
				RID:    "upstream-req-id",
				Domain: "publisher.com",
			},
		},
	}

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Source: &openrtb.Source{
			SChain: &existingSChain,
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be in typed field
	if req.Source.SChain == nil {
		t.Fatal("expected source.SChain to be set")
	}

	// source.ext should not contain schain key
	if req.Source.Ext != nil {
		var extMap map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &extMap); err == nil {
			if _, hasSchain := extMap["schain"]; hasSchain {
				t.Error("expected schain key to be absent from source.ext")
			}
		}
	}

	// Should have 2 nodes now (original + platform)
	if len(req.Source.SChain.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got: %d", len(req.Source.SChain.Nodes))
	}

	// First node should be unchanged
	if req.Source.SChain.Nodes[0].ASI != "publisher.com" {
		t.Errorf("expected first node ASI 'publisher.com', got: %s", req.Source.SChain.Nodes[0].ASI)
	}

	// Second node should be platform node
	platformNode := req.Source.SChain.Nodes[1]
	if platformNode.ASI != "thenexusengine.com" {
		t.Errorf("expected platform node ASI 'thenexusengine.com', got: %s", platformNode.ASI)
	}
	if platformNode.SID != "12345" {
		t.Errorf("expected platform node SID '12345', got: %s", platformNode.SID)
	}
}

func TestSChainAugmentationHook_AppendsToExistingSChainInExt(t *testing.T) {
	hook := NewSChainAugmentationHook("thenexusengine.com", "12345")
	ctx := context.Background()

	// Legacy: existing schain in source.ext (backward compat path)
	existingSChain := openrtb.SupplyChain{
		Ver:      "1.0",
		Complete: 0,
		Nodes: []openrtb.SupplyChainNode{
			{
				ASI:    "publisher.com",
				SID:    "pub-123",
				HP:     1,
				RID:    "upstream-req-id",
				Domain: "publisher.com",
			},
		},
	}

	sourceExt := map[string]interface{}{
		"schain": existingSChain,
	}
	sourceExtBytes, _ := json.Marshal(sourceExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Source: &openrtb.Source{
			Ext: sourceExtBytes,
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be in typed field
	if req.Source.SChain == nil {
		t.Fatal("expected source.SChain to be set")
	}

	// schain key should be removed from source.ext
	if req.Source.Ext != nil {
		var extMap map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &extMap); err == nil {
			if _, hasSchain := extMap["schain"]; hasSchain {
				t.Error("expected schain key to be removed from source.ext")
			}
		}
	}

	// Should have 2 nodes
	if len(req.Source.SChain.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got: %d", len(req.Source.SChain.Nodes))
	}
}

func TestSChainAugmentationHook_SkipsDuplicatePlatformNode(t *testing.T) {
	hook := NewSChainAugmentationHook("thenexusengine.com", "12345")
	ctx := context.Background()

	// Create schain with platform node already present in typed field
	existingSChain := openrtb.SupplyChain{
		Ver:      "1.0",
		Complete: 1,
		Nodes: []openrtb.SupplyChainNode{
			{
				ASI:    "thenexusengine.com",
				SID:    "12345",
				HP:     1,
				RID:    "old-req-id",
				Domain: "thenexusengine.com",
			},
		},
	}

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Source: &openrtb.Source{
			SChain: &existingSChain,
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be in typed field
	if req.Source.SChain == nil {
		t.Fatal("expected source.SChain to be set")
	}

	// source.ext should not contain schain key
	if req.Source.Ext != nil {
		var extMap map[string]interface{}
		if err := json.Unmarshal(req.Source.Ext, &extMap); err == nil {
			if _, hasSchain := extMap["schain"]; hasSchain {
				t.Error("expected schain key to be absent from source.ext")
			}
		}
	}

	// Should still have only 1 node (not duplicated)
	if len(req.Source.SChain.Nodes) != 1 {
		t.Errorf("expected 1 node (no duplicate), got: %d", len(req.Source.SChain.Nodes))
	}
}

func TestSChainAugmentationHook_PreservesOtherSourceExtFields(t *testing.T) {
	hook := NewSChainAugmentationHook("thenexusengine.com", "12345")
	ctx := context.Background()

	// Create source.ext with other fields (no schain)
	sourceExt := map[string]interface{}{
		"custom_field":  "custom_value",
		"another_field": 123,
	}
	sourceExtBytes, _ := json.Marshal(sourceExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Source: &openrtb.Source{
			Ext: sourceExtBytes,
		},
	}

	err := hook.ProcessBidderRequest(ctx, req, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// schain should be in typed field
	if req.Source.SChain == nil {
		t.Error("expected source.SChain to be set")
	}

	// source.ext should still have other fields (not cleared)
	if req.Source.Ext == nil {
		t.Fatal("expected source.ext to be preserved")
	}
	var resultExt map[string]interface{}
	if err := json.Unmarshal(req.Source.Ext, &resultExt); err != nil {
		t.Fatalf("failed to unmarshal source.ext: %v", err)
	}

	// Should preserve other fields
	if resultExt["custom_field"] != "custom_value" {
		t.Error("expected custom_field to be preserved")
	}
	if resultExt["another_field"] == nil {
		t.Error("expected another_field to be preserved")
	}

	// source.ext should NOT contain schain
	if _, hasSchain := resultExt["schain"]; hasSchain {
		t.Error("expected schain key to be absent from source.ext")
	}
}

func TestSChainAugmentationHook_DifferentAccountIDs(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
	}{
		{
			name:      "numeric account ID",
			accountID: "12345",
		},
		{
			name:      "string account ID",
			accountID: "account-abc-123",
		},
		{
			name:      "UUID account ID",
			accountID: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := NewSChainAugmentationHook("thenexusengine.com", tt.accountID)
			ctx := context.Background()

			req := &openrtb.BidRequest{
				ID: "test-request-123",
			}

			err := hook.ProcessBidderRequest(ctx, req, "rubicon")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify SID in typed field
			if req.Source.SChain == nil {
				t.Fatal("expected source.SChain to be set")
			}
			if req.Source.SChain.Nodes[0].SID != tt.accountID {
				t.Errorf("expected SID '%s', got: %s", tt.accountID, req.Source.SChain.Nodes[0].SID)
			}
		})
	}
}
