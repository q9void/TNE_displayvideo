package adcp

import (
	"path/filepath"
	"testing"
)

func TestRegistry_LoadEmptyAllowList(t *testing.T) {
	docPath := filepath.Join("assets", "adcp_agents.json")
	schemaPath := filepath.Join("assets", "adcp_agents.schema.json")

	reg, err := LoadRegistry(docPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if reg.AgentCount() != 0 {
		t.Errorf("AgentCount = %d, want 0 (Phase 1 ships empty allow-list)", reg.AgentCount())
	}
	if reg.SellerID() == "" {
		t.Error("SellerID is empty; expected populated value from assets/adcp_agents.json")
	}
	if len(reg.AgentsForLifecycle(LifecyclePreAuctionSignals)) != 0 {
		t.Error("expected no agents for LifecyclePreAuctionSignals on empty registry")
	}
}

func TestRegistry_FromBytes_SortsByPriority(t *testing.T) {
	doc := []byte(`{
	  "version": "1.0",
	  "seller_id": "9131",
	  "seller_domain": "example.com",
	  "agents": [
	    {"id":"b","role":"signals","priority":10,"endpoints":[{"transport":"mcp","url":"https://b","auth":"api_key_header"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]},
	    {"id":"a","role":"signals","priority":10,"endpoints":[{"transport":"mcp","url":"https://a","auth":"api_key_header"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]},
	    {"id":"c","role":"signals","priority":1, "endpoints":[{"transport":"mcp","url":"https://c","auth":"api_key_header"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]}
	  ]
	}`)
	reg, err := LoadRegistryFromBytes(doc)
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	all := reg.AllAgents()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	// Priority 1 first, then ties broken by id ascending.
	wantOrder := []string{"c", "a", "b"}
	for i, want := range wantOrder {
		if all[i].ID != want {
			t.Errorf("agents[%d].ID = %q, want %q", i, all[i].ID, want)
		}
	}
}

func TestAgent_HasLifecycleAndCapability(t *testing.T) {
	a := Agent{
		Lifecycles:   []string{"PRE_AUCTION_SIGNALS"},
		Capabilities: []string{"get_signals"},
	}
	if !a.HasLifecycle(LifecyclePreAuctionSignals) {
		t.Error("HasLifecycle(PRE_AUCTION_SIGNALS) = false")
	}
	if a.HasLifecycle(LifecyclePostAuctionReporting) {
		t.Error("HasLifecycle(POST_AUCTION_REPORTING) = true; want false")
	}
	if !a.HasCapability(CapabilityGetSignals) {
		t.Error("HasCapability(get_signals) = false")
	}
	if a.HasCapability(CapabilityCreateMediaBuy) {
		t.Error("HasCapability(create_media_buy) = true; want false")
	}
}
