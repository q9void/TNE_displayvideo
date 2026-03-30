package routing_test

import (
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestResolveEID_FromUserEIDs(t *testing.T) {
	user := &openrtb.User{
		EIDs: []openrtb.EID{
			{Source: "kargo.com", UIDs: []openrtb.UID{{ID: "kargo-buyer-123"}}},
		},
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "kargo-buyer-123" {
		t.Errorf("got %q, want %q", got, "kargo-buyer-123")
	}
}

func TestResolveEID_FallbackToExtEIDs(t *testing.T) {
	extEIDsJSON := `[{"source":"kargo.com","uids":[{"id":"kargo-ext-456"}]}]`
	user := &openrtb.User{
		Ext: json.RawMessage(`{"eids":` + extEIDsJSON + `}`),
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "kargo-ext-456" {
		t.Errorf("got %q, want %q", got, "kargo-ext-456")
	}
}

func TestResolveEID_MissingSource(t *testing.T) {
	user := &openrtb.User{
		EIDs: []openrtb.EID{
			{Source: "other.com", UIDs: []openrtb.UID{{ID: "other-111"}}},
		},
	}
	got := routing.ResolveEID(user, "kargo.com")
	if got != "" {
		t.Errorf("expected empty string for missing source, got %q", got)
	}
}

func TestResolveEID_NilUser(t *testing.T) {
	got := routing.ResolveEID(nil, "kargo.com")
	if got != "" {
		t.Errorf("expected empty string for nil user, got %q", got)
	}
}
