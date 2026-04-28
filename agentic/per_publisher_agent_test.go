package agentic

import (
	"context"
	"errors"
	"testing"
)

type stubAllowList struct {
	allowed map[string]bool // key: "publisherID|curatorID"
	err     error
}

func (s *stubAllowList) PublisherAllowedForCurator(_ context.Context, pub int, cur string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed[itoaInt(pub)+"|"+cur], nil
}

func itoaInt(v int) string {
	if v == 0 {
		return "0"
	}
	d := make([]byte, 0, 4)
	for v > 0 {
		d = append([]byte{byte('0' + v%10)}, d...)
		v /= 10
	}
	return string(d)
}

func TestAgentFilterForPublisher_BoundAgentRespectsAllowList(t *testing.T) {
	bindings := map[string]string{
		"curator-agent": "c1",
	}
	allow := &stubAllowList{allowed: map[string]bool{"42|c1": true}}
	filter := AgentFilterForPublisher(context.Background(), 42, bindings, allow)
	if filter == nil {
		t.Fatal("expected filter, got nil")
	}
	if !filter("curator-agent") {
		t.Errorf("publisher 42 IS allow-listed for c1; agent should pass")
	}
	// Unbound platform-wide agent should ALWAYS pass.
	if !filter("platform-agent") {
		t.Errorf("unbound agent must pass through unconditionally")
	}
}

func TestAgentFilterForPublisher_BoundAgentBlockedWhenNotAllowed(t *testing.T) {
	bindings := map[string]string{"curator-agent": "c1"}
	allow := &stubAllowList{allowed: map[string]bool{}} // 99 not allow-listed
	filter := AgentFilterForPublisher(context.Background(), 99, bindings, allow)
	if filter("curator-agent") {
		t.Errorf("publisher 99 NOT allow-listed for c1; agent should be blocked")
	}
}

func TestAgentFilterForPublisher_FailOpenOnDBError(t *testing.T) {
	bindings := map[string]string{"a": "c"}
	allow := &stubAllowList{err: errors.New("db unavailable")}
	filter := AgentFilterForPublisher(context.Background(), 7, bindings, allow)
	if !filter("a") {
		t.Errorf("expected fail-open on db error; got block")
	}
}

func TestAgentFilterForPublisher_ZeroPublisherIDDisables(t *testing.T) {
	if got := AgentFilterForPublisher(context.Background(), 0, map[string]string{"a": "c"}, &stubAllowList{}); got != nil {
		t.Fatalf("expected nil filter when publisher_id=0")
	}
}

func TestRegistry_AgentsForLifecycleFiltered_SkipsBlocked(t *testing.T) {
	doc := []byte(`{
		"version":"1.0","seller_id":"X","seller_domain":"x.com",
		"agents":[
			{"id":"a","role":"segmentation","intents":["ACTIVATE_DEALS"],
			 "lifecycles":["PUBLISHER_BID_REQUEST"],
			 "endpoints":[{"transport":"GRPCS","url":"https://x"}],
			 "auth":{"method":"NONE"}},
			{"id":"b","role":"floor","intents":["ACTIVATE_DEALS"],
			 "lifecycles":["PUBLISHER_BID_REQUEST"],
			 "endpoints":[{"transport":"GRPCS","url":"https://y"}],
			 "auth":{"method":"NONE"}}
		]
	}`)
	r, err := LoadRegistryFromBytes(doc)
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	got := r.AgentsForLifecycleFiltered(LifecyclePublisherBidRequest, func(id string) bool {
		return id != "b"
	})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("expected only [a], got %v", got)
	}
}
