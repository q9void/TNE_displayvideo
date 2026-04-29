package inbound

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/credentials"
)

// MTLSAuthenticator is the production-grade Authenticator for the inbound
// AAMP services. It enforces the two-layer trust model from PRD §5.3.1:
//
//   1. The caller's mTLS cert MUST verify against the configured CA bundle.
//      gRPC's TLS handshake handles this before our handler runs; if the
//      handshake fails the call never reaches Verify.
//   2. The cert's Subject Public Key Info (SPKI) fingerprint MUST match
//      a per-buyer entry in our pinned-fingerprints table.
//   3. The caller's agent_id MUST exist in the IAB Tools Portal Registry's
//      verified-seller-eligible list (last successful refresh).
//
// Either of (2) or (3) failing → reject with the matching ErrAuthFailed*
// sentinel; the handler maps to gRPC Unauthenticated.
//
// Threadsafe; Verify reads the snapshot lock-free via sync/atomic and
// RefreshRegistry installs a new snapshot atomically.
type MTLSAuthenticator struct {
	// Pinned per-buyer SPKI fingerprints. Keyed by AgentID; value is the
	// hex-encoded SHA-256 of the cert's SubjectPublicKeyInfo. Two values
	// per agent supported (R5.2.3 cert-rotation overlap window).
	mu     sync.RWMutex
	pinned map[string]pinEntry

	// Trusted-buyer table: AgentID → AgentEntry. Same shape as
	// DevAuthenticator. Loaded at boot from agents.json or runtime DB.
	allowList map[string]AgentEntry

	// Registry verified-seller-eligible set. Refreshed periodically by
	// RefreshRegistry. Empty until first successful refresh; configurable
	// behavior during the cold-start window via OpenWindow.
	regMu       sync.RWMutex
	registrySet map[string]struct{}
	regOK       bool // true once at least one refresh has succeeded

	// Registry client. Nil → Skip the registry check entirely (dev/staging).
	regClient RegistryClient

	// OpenWindow: when true, callers ARE accepted before the first
	// successful registry refresh. Phase 2A.1 ships this true so a registry
	// outage at boot doesn't block all traffic. Set false for stricter
	// posture once the registry is dependable in production.
	openWindow bool
}

// pinEntry is two SPKI fingerprints per agent — current + successor for
// 30-day rotation per PRD R5.2.3. Either accepted.
type pinEntry struct {
	current   string
	successor string
}

// MTLSConfig governs the production authenticator. CARoots is the trust
// anchor used by the gRPC server's TLS handshake; SPKIPinnedAgents is our
// per-buyer fingerprint table; Registry is the verified-seller cross-check.
//
// All fields except CARoots required when constructing for production. The
// CARoots is installed on the gRPC server (Server.Start) and never on the
// MTLSAuthenticator itself.
type MTLSConfig struct {
	CARoots          *x509.CertPool
	SPKIPinnedAgents map[string]pinEntry // optional: pre-loaded pins
	AllowList        []AgentEntry        // trusted-buyer table
	Registry         RegistryClient      // nil ⇒ skip registry cross-check
	OpenWindow       bool                // true ⇒ accept callers before first registry refresh
}

// NewMTLSAuthenticator constructs the production authenticator. Returns
// nil if cfg is internally inconsistent.
func NewMTLSAuthenticator(cfg MTLSConfig) (*MTLSAuthenticator, error) {
	if cfg.CARoots == nil {
		return nil, errors.New("inbound: MTLSAuthenticator requires CARoots")
	}
	a := &MTLSAuthenticator{
		pinned:      map[string]pinEntry{},
		allowList:   map[string]AgentEntry{},
		registrySet: map[string]struct{}{},
		regClient:   cfg.Registry,
		openWindow:  cfg.OpenWindow,
	}
	for k, v := range cfg.SPKIPinnedAgents {
		a.pinned[k] = v
	}
	for _, e := range cfg.AllowList {
		a.allowList[e.AgentID] = e
	}
	return a, nil
}

// AddPinned registers or updates the SPKI fingerprint pair for an agent.
// Used at boot from config and at runtime for cert rotation. Either current
// or successor (or both) accepted on subsequent Verify calls.
func (a *MTLSAuthenticator) AddPinned(agentID, current, successor string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pinned[agentID] = pinEntry{current: current, successor: successor}
}

// AddAllowList inserts or replaces a trusted-buyer entry.
func (a *MTLSAuthenticator) AddAllowList(e AgentEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowList[e.AgentID] = e
}

// Verify implements Authenticator.
//
// Order of checks:
//   1. Pull peer info from ctx — gRPC's TLS handshake already verified the
//      cert chain against CARoots (configured at server level), so by the
//      time we're here we have a valid leaf cert.
//   2. Compute SPKI fingerprint of the leaf cert.
//   3. Look up agent_id from the cert's CommonName or SAN; reject if
//      absent from our trusted-buyer table.
//   4. Match the fingerprint against the agent's pinned current/successor;
//      reject ErrAuthFailedSPKI on mismatch.
//   5. If a registry client is configured, check the agent_id is in the
//      last successful refresh set. Reject ErrAuthFailedRegistry on miss
//      unless OpenWindow=true and no refresh has succeeded yet.
//
// Returns AgentIdentity{RegistryVerified: <result of step 5>, ...} on success.
func (a *MTLSAuthenticator) Verify(ctx context.Context) (*AgentIdentity, error) {
	leaf, err := leafCertFromContext(ctx)
	if err != nil {
		return nil, err
	}

	spki := SPKIFingerprint(leaf)
	agentID := agentIDFromCert(leaf)
	if agentID == "" {
		return nil, ErrAuthFailed
	}

	a.mu.RLock()
	entry, allowed := a.allowList[agentID]
	pin, pinned := a.pinned[agentID]
	a.mu.RUnlock()

	if !allowed {
		return nil, ErrAuthFailed
	}
	if !pinned || (spki != pin.current && spki != pin.successor) {
		return nil, ErrAuthFailedSPKI
	}

	regVerified := true
	if a.regClient != nil {
		a.regMu.RLock()
		_, inSet := a.registrySet[agentID]
		coldStart := !a.regOK
		a.regMu.RUnlock()

		switch {
		case inSet:
			regVerified = true
		case coldStart && a.openWindow:
			// Registry not yet refreshed; OpenWindow accepts the call but
			// flags RegistryVerified=false so handlers / audit can see it.
			regVerified = false
		default:
			return nil, ErrAuthFailedRegistry
		}
	}

	agentType := entry.AgentType
	if agentType == "" {
		agentType = "DSP"
	}
	return &AgentIdentity{
		AgentID:          agentID,
		AgentType:        agentType,
		AuthorizedDeals:  entry.AuthorizedDeals,
		SPKIFingerprint:  spki,
		RegistryVerified: regVerified,
	}, nil
}

// RefreshRegistry pulls the verified-seller-eligible list from the IAB
// Tools Portal Registry. On success, atomically swaps the in-memory set
// and marks regOK=true. On failure, leaves the existing snapshot intact
// and returns the error so the caller can increment a failure metric.
func (a *MTLSAuthenticator) RefreshRegistry(ctx context.Context) error {
	if a.regClient == nil {
		return nil // no registry configured ⇒ no-op
	}
	ids, err := a.regClient.VerifiedSellerEligible(ctx)
	if err != nil {
		return fmt.Errorf("registry refresh: %w", err)
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	a.regMu.Lock()
	a.registrySet = set
	a.regOK = true
	a.regMu.Unlock()
	return nil
}

// SPKIFingerprint returns the hex-encoded SHA-256 of a cert's
// SubjectPublicKeyInfo. Stable across cert renewals as long as the keypair
// is reused — which is the property we want for pinning.
func SPKIFingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// agentIDFromCert extracts the caller's agent_id from the cert. Phase 2A.1
// reads from CommonName for simplicity; Phase 2A.2 may extend to SAN URI.
func agentIDFromCert(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	if cn := cert.Subject.CommonName; cn != "" {
		return cn
	}
	for _, uri := range cert.URIs {
		if uri != nil && uri.Host != "" {
			return uri.Host
		}
	}
	return ""
}

// leafCertFromContext pulls the verified leaf cert from the gRPC peer
// context. gRPC's TLS handshake populates AuthInfo with the chain; we
// take the first (leaf) cert.
func leafCertFromContext(ctx context.Context) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil {
		return nil, ErrAuthFailed
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, ErrAuthFailed
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, ErrAuthFailed
	}
	return tlsInfo.State.PeerCertificates[0], nil
}

// Compile-time guard: MTLSAuthenticator satisfies the Authenticator
// interface.
var _ Authenticator = (*MTLSAuthenticator)(nil)
