package inbound

import (
	"fmt"
	"time"
)

// ServerConfig governs the inbound gRPC server's behaviour. All fields have
// sensible defaults filled in by defaults(); callers may pass a partial
// ServerConfig and the zero values are populated.
type ServerConfig struct {
	// Enabled is the master switch. When false, Server.Start returns
	// immediately without binding the listener.
	Enabled bool

	// GRPCPort is the listener port for the AAMP inbound services.
	// Default 50051 per AAMP convention.
	GRPCPort int

	// MTLSCAPath, MTLSServerCertPath, MTLSServerKeyPath configure the mTLS
	// listener. Required in production; optional in dev when AllowDevNoMTLS
	// is true. Phase 2A ships dev-only auth; full mTLS is Phase 2A.1.
	MTLSCAPath         string
	MTLSServerCertPath string
	MTLSServerKeyPath  string

	// AllowDevNoMTLS skips mTLS termination — caller authentication via a
	// static gRPC metadata header instead. Production deploys MUST set this
	// false; cmd/server/config.go::Validate enforces this.
	AllowDevNoMTLS bool

	// QPSPerAgent is the soft cap on inbound calls from a single agent_id.
	// Default 1000.
	QPSPerAgent int

	// QPSPerPublisher is the soft cap on inbound mutations affecting a
	// single publisher. Default 200.
	QPSPerPublisher int

	// MaxRecvMsgBytes caps inbound gRPC message size. Default 4 MiB matches
	// outbound default from Phase 1.
	MaxRecvMsgBytes int

	// IdempotencyWindow is how long a successful response is cached for
	// duplicate-RTBRequest.id requests (R5.1.11). Default 60s.
	IdempotencyWindow time.Duration

	// CircuitFailureThreshold is the count of consecutive failures before
	// the per-agent circuit breaker opens. Default 50 per minute.
	CircuitFailureThreshold int
	CircuitTimeout          time.Duration
}

// defaults fills zero-valued fields with Phase 2A defaults.
func (c ServerConfig) defaults() ServerConfig {
	if c.GRPCPort == 0 {
		c.GRPCPort = 50051
	}
	if c.QPSPerAgent <= 0 {
		c.QPSPerAgent = 1000
	}
	if c.QPSPerPublisher <= 0 {
		c.QPSPerPublisher = 200
	}
	if c.MaxRecvMsgBytes <= 0 {
		c.MaxRecvMsgBytes = 4 * 1024 * 1024
	}
	if c.IdempotencyWindow <= 0 {
		c.IdempotencyWindow = 60 * time.Second
	}
	if c.CircuitFailureThreshold <= 0 {
		c.CircuitFailureThreshold = 50
	}
	if c.CircuitTimeout <= 0 {
		c.CircuitTimeout = 30 * time.Second
	}
	return c
}

// Validate returns an error if cfg is internally inconsistent. The
// production-vs-dev mTLS check is the caller's responsibility (see
// cmd/server/config.go::Validate).
func (c ServerConfig) Validate() error {
	if !c.Enabled {
		return nil // disabled config is always valid
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("inbound: GRPCPort must be in [1, 65535], got %d", c.GRPCPort)
	}
	if !c.AllowDevNoMTLS {
		if c.MTLSCAPath == "" {
			return fmt.Errorf("inbound: MTLSCAPath required when AllowDevNoMTLS=false")
		}
		if c.MTLSServerCertPath == "" {
			return fmt.Errorf("inbound: MTLSServerCertPath required when AllowDevNoMTLS=false")
		}
		if c.MTLSServerKeyPath == "" {
			return fmt.Errorf("inbound: MTLSServerKeyPath required when AllowDevNoMTLS=false")
		}
	}
	return nil
}
