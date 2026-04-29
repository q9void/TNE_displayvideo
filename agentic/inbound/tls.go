package inbound

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// LoadServerTLS builds a credentials.TransportCredentials from on-disk PEM
// material. The returned credentials require + verify a client certificate,
// matching the AAMP 2.0 deployment posture: every inbound caller MUST
// present a cert that chains to caPath.
//
// Paths come from validated server config, which is why gosec G304 is a
// false positive on the ReadFile calls.
func LoadServerTLS(caPath, certPath, keyPath string) (credentials.TransportCredentials, error) {
	if caPath == "" || certPath == "" || keyPath == "" {
		return nil, errors.New("inbound: LoadServerTLS requires non-empty caPath, certPath, keyPath")
	}

	caPEM, err := os.ReadFile(caPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("inbound: read CA bundle %q: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("inbound: CA bundle %q contains no usable PEM blocks", caPath)
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("inbound: load server keypair: %w", err)
	}

	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}), nil
}

// LoadCAPool reads a PEM bundle from caPath and returns it as an
// *x509.CertPool. Used to build the MTLSAuthenticator's CARoots — the
// pool is shared with the gRPC TLS terminator so any cert that passes the
// handshake is also trusted by the authenticator.
func LoadCAPool(caPath string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("inbound: read CA bundle %q: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("inbound: CA bundle %q contains no usable PEM blocks", caPath)
	}
	return pool, nil
}

// PinsFileEntry is one row in a pins JSON file:
//
//	{
//	  "pins": [
//	    {"agent_id":"dsp.example.com", "current":"<sha256-hex>", "successor":""},
//	    {"agent_id":"agency.example.com","current":"<sha256-hex>", "successor":"<next-sha256-hex>"}
//	  ]
//	}
//
// successor is optional — empty means "no rotation in flight". The
// loader rejects any entry missing agent_id or current.
type PinsFileEntry struct {
	AgentID   string `json:"agent_id"`
	Current   string `json:"current"`
	Successor string `json:"successor,omitempty"`
}

type pinsFileDoc struct {
	Pins []PinsFileEntry `json:"pins"`
}

// LoadPinsFile reads and validates a pins JSON file. Returns the parsed
// entries; the caller passes each one to MTLSAuthenticator.AddPinned.
//
// Empty file ("{}") is allowed — produces an empty slice. The MTLSAuth
// will then reject every caller until pins are added at runtime.
func LoadPinsFile(path string) ([]PinsFileEntry, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("inbound: read pins file %q: %w", path, err)
	}
	var doc pinsFileDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("inbound: parse pins file %q: %w", path, err)
	}
	for i, p := range doc.Pins {
		if p.AgentID == "" {
			return nil, fmt.Errorf("inbound: pins[%d].agent_id is empty", i)
		}
		if p.Current == "" {
			return nil, fmt.Errorf("inbound: pins[%d].current is empty (agent_id=%q)", i, p.AgentID)
		}
	}
	return doc.Pins, nil
}
