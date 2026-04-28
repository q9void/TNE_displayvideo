package adcp

import "testing"

func TestBuild_EmptySellerIDProducesZeroEnvelope(t *testing.T) {
	got := Build("", DispatchResult{})
	if got.SellerID != "" || len(got.AgentsCalled) != 0 || got.SignalsActivated != 0 || got.Disabled {
		t.Errorf("Build(empty) = %+v; want zero Envelope", got)
	}
}

func TestBuild_PopulatesAgentsAndSignalCount(t *testing.T) {
	d := DispatchResult{
		CallStats: []CallStat{
			{AgentID: "ok-agent", Status: "ok", ResultCount: 3},
			{AgentID: "stub-agent", Status: "not_implemented"},
		},
		Signals: []Signal{
			{ID: "s1"}, {ID: "s2"}, {ID: "s3"},
		},
	}
	got := Build("9131", d)
	if got.SellerID != "9131" {
		t.Errorf("SellerID = %q, want 9131", got.SellerID)
	}
	if got.SignalsActivated != 3 {
		t.Errorf("SignalsActivated = %d, want 3", got.SignalsActivated)
	}
	if len(got.AgentsCalled) != 1 || got.AgentsCalled[0] != "ok-agent" {
		t.Errorf("AgentsCalled = %v; want [ok-agent] (only ok+ResultCount>0 entries)", got.AgentsCalled)
	}
}

func TestLifecycle_StringRoundTrip(t *testing.T) {
	cases := []Lifecycle{
		LifecyclePreAuctionSignals,
		LifecyclePreAuctionProducts,
		LifecyclePostAuctionReporting,
	}
	for _, lc := range cases {
		if got := ParseLifecycle(lc.String()); got != lc {
			t.Errorf("ParseLifecycle(%q) = %v, want %v", lc.String(), got, lc)
		}
	}
}

func TestCapability_IsKnown(t *testing.T) {
	if !CapabilityGetSignals.IsKnown() {
		t.Error("CapabilityGetSignals.IsKnown() = false")
	}
	if Capability("not_a_real_capability").IsKnown() {
		t.Error("unknown capability reported as known")
	}
}
