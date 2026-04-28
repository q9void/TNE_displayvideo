package adcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewClient_AppliesDefaults(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com","agents":[]
	}`))
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	c, err := NewClient(reg, ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.cfg.DefaultTmaxMs != 30 {
		t.Errorf("DefaultTmaxMs = %d, want 30 (default)", c.cfg.DefaultTmaxMs)
	}
	if c.cfg.AuctionSafetyMs != 50 {
		t.Errorf("AuctionSafetyMs = %d, want 50 (default)", c.cfg.AuctionSafetyMs)
	}
	if c.cfg.MaxSignalsPerResponse != 256 {
		t.Errorf("MaxSignalsPerResponse = %d, want 256 (default)", c.cfg.MaxSignalsPerResponse)
	}
	if c.cfg.CircuitTimeout != 30*time.Second {
		t.Errorf("CircuitTimeout = %v, want 30s (default)", c.cfg.CircuitTimeout)
	}
}

func TestNewClient_RejectsInsecureByDefault(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com",
	  "agents":[
	    {"id":"x","role":"signals","endpoints":[{"transport":"http","url":"http://x","auth":"none"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]}
	  ]
	}`))
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	_, err = NewClient(reg, ClientConfig{})
	if err == nil {
		t.Fatal("NewClient with http:// transport returned nil error; want ErrInsecureTransport")
	}
	if !errors.Is(err, ErrInsecureTransport) {
		t.Errorf("err = %v; want errors.Is(err, ErrInsecureTransport)", err)
	}
}

func TestNewClient_AcceptsInsecureWhenAllowed(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com",
	  "agents":[
	    {"id":"x","role":"signals","endpoints":[{"transport":"http","url":"http://x","auth":"none"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]}
	  ]
	}`))
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	if _, err := NewClient(reg, ClientConfig{AllowInsecure: true}); err != nil {
		t.Fatalf("NewClient with AllowInsecure=true: %v", err)
	}
}

func TestDispatch_NoAgents(t *testing.T) {
	reg, _ := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com","agents":[]
	}`))
	c, _ := NewClient(reg, ClientConfig{})
	got := c.Dispatch(context.Background(), CapabilityGetSignals, LifecyclePreAuctionSignals)
	if len(got.CallStats) != 0 || len(got.Signals) != 0 {
		t.Errorf("empty registry returned non-empty result: %+v", got)
	}
}

func TestDispatch_NotImplementedStubReportsStatus(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com",
	  "agents":[
	    {"id":"sig.example.com","role":"signals","endpoints":[{"transport":"mcp","url":"https://sig","auth":"api_key_header"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_signals"]}
	  ]
	}`))
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	c, err := NewClient(reg, ClientConfig{DefaultTmaxMs: 100})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got := c.Dispatch(context.Background(), CapabilityGetSignals, LifecyclePreAuctionSignals)
	if len(got.CallStats) != 1 {
		t.Fatalf("len(CallStats) = %d, want 1; got=%+v", len(got.CallStats), got)
	}
	stat := got.CallStats[0]
	if stat.Status != "not_implemented" {
		t.Errorf("Status = %q, want %q (Phase 1 stub)", stat.Status, "not_implemented")
	}
	if stat.AgentID != "sig.example.com" {
		t.Errorf("AgentID = %q, want sig.example.com", stat.AgentID)
	}
	if stat.Capability != string(CapabilityGetSignals) {
		t.Errorf("Capability = %q, want get_signals", stat.Capability)
	}
}

func TestDispatch_SkipsAgentsLackingCapability(t *testing.T) {
	reg, err := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com",
	  "agents":[
	    {"id":"sales.example.com","role":"sales","endpoints":[{"transport":"mcp","url":"https://s","auth":"api_key_header"}],"lifecycles":["PRE_AUCTION_SIGNALS"],"capabilities":["get_products"]}
	  ]
	}`))
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	c, _ := NewClient(reg, ClientConfig{DefaultTmaxMs: 100})

	got := c.Dispatch(context.Background(), CapabilityGetSignals, LifecyclePreAuctionSignals)
	if len(got.CallStats) != 0 {
		t.Errorf("CallStats=%d; agent lacks get_signals capability so it should be skipped", len(got.CallStats))
	}
}

func TestKeyFor_PerAgentOverridesGlobal(t *testing.T) {
	reg, _ := LoadRegistryFromBytes([]byte(`{
	  "version":"1.0","seller_id":"9131","seller_domain":"example.com","agents":[]
	}`))
	c, _ := NewClient(reg, ClientConfig{
		APIKey:          "global",
		PerAgentAPIKeys: map[string]string{"foo": "specific"},
	})
	if got := c.keyFor(Agent{ID: "foo"}); got != "specific" {
		t.Errorf("keyFor(foo) = %q, want %q", got, "specific")
	}
	if got := c.keyFor(Agent{ID: "bar"}); got != "global" {
		t.Errorf("keyFor(bar) = %q, want %q", got, "global")
	}
}
