package inbound

import (
	"context"
	"strings"
	"sync"

	"google.golang.org/grpc/metadata"
)

// Authenticator resolves an inbound caller's AgentIdentity from the gRPC
// context, OR returns ErrAuthFailed (or a more specific sentinel) if the
// caller is not authorised. Implementations:
//
//   - DevAuthenticator — Phase 2A; identifies the caller via the static
//     gRPC metadata header "x-aamp-agent-id". Dev/staging only.
//   - MTLSAuthenticator — Phase 2A.1; verifies caller cert against a
//     CA bundle, pins SPKI fingerprint per buyer, cross-checks IAB
//     Tools Portal Registry. Production only.
//
// The interface is deliberately minimal: a single Verify call covers
// AuthN + AuthZ for inbound callers. Per-deal authorisation lives on
// the returned AgentIdentity (R5.1.10).
type Authenticator interface {
	// Verify pulls credentials from ctx and returns an AgentIdentity, or
	// an error mappable to ErrAuthFailed* sentinels.
	Verify(ctx context.Context) (*AgentIdentity, error)

	// RefreshRegistry refreshes the trusted-buyer cache from the IAB Tools
	// Portal Registry. DevAuthenticator no-ops (returns nil).
	RefreshRegistry(ctx context.Context) error
}

// AgentEntry is a row in the DevAuthenticator allow-list.
type AgentEntry struct {
	AgentID         string
	AgentType       string   // "DSP" | "PUBLISHER"
	AuthorisedDeals []string // optional in 2A; populated in 2B
}

// DevAuthenticator is the no-mTLS authenticator used in dev and staging.
// It identifies the caller via the gRPC metadata header
// "x-aamp-agent-id" and looks them up in an in-memory allow-list.
//
// Production deployments MUST use MTLSAuthenticator (Phase 2A.1).
// cmd/server/config.go::Validate enforces this when ENVIRONMENT=production.
type DevAuthenticator struct {
	mu        sync.RWMutex
	allowList map[string]AgentEntry // agent_id → entry
}

// NewDevAuthenticator constructs a DevAuthenticator with the given
// allow-list. Pass nil for an empty allow-list — every Verify call will
// then fail.
func NewDevAuthenticator(entries []AgentEntry) *DevAuthenticator {
	a := &DevAuthenticator{
		allowList: make(map[string]AgentEntry, len(entries)),
	}
	for _, e := range entries {
		a.allowList[e.AgentID] = e
	}
	return a
}

// Add inserts or replaces an allow-list entry. Useful in tests; ops should
// drive prod allow-lists from config rather than runtime calls.
func (a *DevAuthenticator) Add(e AgentEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowList[e.AgentID] = e
}

// Verify reads "x-aamp-agent-id" from the incoming context's metadata and
// returns the matching AgentIdentity. Header is case-insensitive (gRPC
// metadata keys are normalised to lowercase by grpc-go).
func (a *DevAuthenticator) Verify(ctx context.Context) (*AgentIdentity, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrAuthFailed
	}
	vals := md.Get(metadataKeyAgentID)
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return nil, ErrAuthFailed
	}
	id := strings.TrimSpace(vals[0])

	a.mu.RLock()
	entry, found := a.allowList[id]
	a.mu.RUnlock()
	if !found {
		return nil, ErrAuthFailed
	}

	agentType := entry.AgentType
	if agentType == "" {
		agentType = "DSP" // sensible default for curators
	}
	return &AgentIdentity{
		AgentID:          entry.AgentID,
		AgentType:        agentType,
		AuthorisedDeals:  entry.AuthorisedDeals,
		SPKIFingerprint:  "", // dev-only: no cert in this path
		RegistryVerified: true,
	}, nil
}

// RefreshRegistry is a no-op for the dev authenticator — there's no
// upstream registry to refresh against.
func (*DevAuthenticator) RefreshRegistry(context.Context) error { return nil }

// metadataKeyAgentID is the gRPC metadata header DevAuthenticator reads.
// AAMP convention: x-aamp-* prefix, mirroring x-aamp-key from Phase 1.
const metadataKeyAgentID = "x-aamp-agent-id"
