package inbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RegistryClient resolves the IAB Tools Portal Agent Registry's current
// "verified-seller-eligible" set. A buyer agent must appear in this set to
// be permitted to call our inbound surface.
//
// We keep the surface narrow on purpose:
//
//   - One method, returning the full set as a slice of agent_ids. The
//     caller (MTLSAuthenticator) materializes this into a map for O(1)
//     lookups and atomically swaps the snapshot.
//   - No streaming, no pagination, no diffing. The IAB registry is small
//     (hundreds of buyers, not millions) and we only refresh on a timer
//     (default 15m); a full snapshot is the simplest correct contract.
//
// Implementations:
//
//   - HTTPRegistryClient — production. Pulls JSON from a configurable URL
//     (the IAB Tools Portal endpoint, or a vendor-mirrored copy).
//   - StaticRegistryClient — tests / dev. Returns a fixed slice.
type RegistryClient interface {
	// VerifiedSellerEligible returns the current set of agent_ids that
	// have IAB-attested verified-seller eligibility. Order is not
	// significant; duplicates are tolerated by the caller.
	VerifiedSellerEligible(ctx context.Context) ([]string, error)
}

// HTTPRegistryClient is the production RegistryClient. It fetches a JSON
// document from URL and extracts the agent_id list.
//
// Response shape (Phase 2A.1, pending IAB Tools Portal API freeze):
//
//	{
//	  "version": "2026-04-01",
//	  "verified_seller_eligible": [
//	    "dsp.example.com",
//	    "agency.example.com"
//	  ]
//	}
//
// Unknown fields are ignored so the IAB can extend the schema without
// breaking us.
type HTTPRegistryClient struct {
	URL        string
	HTTPClient *http.Client // nil ⇒ default with 5s timeout

	// MaxBytes caps the response read. Defaults to 8 MiB, plenty for a
	// flat list of buyer DNS names. Defends against a misbehaving upstream
	// streaming a multi-GB response.
	MaxBytes int64
}

const (
	defaultRegistryTimeout  = 5 * time.Second
	defaultRegistryMaxBytes = 8 << 20 // 8 MiB
)

// registryDocument is the wire shape of the IAB Tools Portal endpoint.
// Internal — callers see only the agent_id slice via the interface.
type registryDocument struct {
	Version                string   `json:"version,omitempty"`
	VerifiedSellerEligible []string `json:"verified_seller_eligible"`
}

// VerifiedSellerEligible implements RegistryClient.
func (c *HTTPRegistryClient) VerifiedSellerEligible(ctx context.Context) ([]string, error) {
	if c == nil || c.URL == "" {
		return nil, errors.New("inbound: HTTPRegistryClient.URL is empty")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultRegistryTimeout}
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultRegistryMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("inbound: registry request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tne-springwire/agentic-inbound")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inbound: registry fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inbound: registry HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("inbound: registry read: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("inbound: registry body exceeds %d bytes", maxBytes)
	}

	var doc registryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("inbound: registry parse: %w", err)
	}
	return doc.VerifiedSellerEligible, nil
}

// StaticRegistryClient returns a fixed list. Used in tests and as the
// effective implementation when the IAB endpoint is unreachable but ops
// want to ship a known-good snapshot pinned in config.
type StaticRegistryClient struct {
	IDs []string
}

// VerifiedSellerEligible implements RegistryClient.
func (c *StaticRegistryClient) VerifiedSellerEligible(context.Context) ([]string, error) {
	out := make([]string, len(c.IDs))
	copy(out, c.IDs)
	return out, nil
}

// Compile-time guards.
var (
	_ RegistryClient = (*HTTPRegistryClient)(nil)
	_ RegistryClient = (*StaticRegistryClient)(nil)
)
