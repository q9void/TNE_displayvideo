package inbound

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	"github.com/thenexusengine/tne_springwire/agentic"
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// Server is the AAMP 2.0 inbound surface — gRPC services that authenticated
// buyer agents (curators, DSPs) can dial to submit mutations and discover
// our capabilities.
//
// Phase 2A registers two services on a single :50051 listener:
//   - RTBExtensionPoint.GetMutations (rtb_handler.go)
//   - Discovery.DescribeCapabilities (discovery_handler.go)
//
// OpenDirect/v2 (deal creation) is Phase 2B; this Server returns
// Unimplemented for those calls until then.
type Server struct {
	cfg      ServerConfig
	applier  *agentic.Applier
	registry *agentic.Registry
	auth     Authenticator
	rate     *RateLimiter
	stamper  agentic.OriginatorStamper
	metrics  *Metrics

	// Idempotency cache: map[RTBRequest.id]→cachedEntry. Cleared by a
	// goroutine kicked off in Start; entries past IdempotencyWindow are
	// dropped.
	idemMu    sync.Mutex
	idemCache map[string]*idempotentEntry

	// gRPC lifecycle.
	grpcServer *grpc.Server
	listener   net.Listener
	started    atomic.Bool
	stopped    atomic.Bool

	// Required to satisfy the generated gRPC server interface; we register
	// the RTBExtensionPoint service explicitly in Start.
	pb.UnimplementedRTBExtensionPointServer
}

// idempotentEntry caches a successful response for replay within the
// IdempotencyWindow. Phase 2A caches only successful responses.
type idempotentEntry struct {
	response  *pb.RTBResponse
	timestamp time.Time
}

// NewServer constructs an inbound Server. cfg.defaults() is applied so
// callers can pass a partial ServerConfig.
//
// applier and registry MUST be non-nil; auth MUST be non-nil. stamper
// SellerID is used to fill RTBResponse.metadata.
func NewServer(
	cfg ServerConfig,
	applier *agentic.Applier,
	registry *agentic.Registry,
	auth Authenticator,
	stamper agentic.OriginatorStamper,
) (*Server, error) {
	cfg = cfg.defaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("inbound: invalid config: %w", err)
	}
	if applier == nil {
		return nil, fmt.Errorf("inbound: applier required")
	}
	if registry == nil {
		return nil, fmt.Errorf("inbound: registry required")
	}
	if auth == nil {
		return nil, fmt.Errorf("inbound: authenticator required")
	}
	return &Server{
		cfg:       cfg,
		applier:   applier,
		registry:  registry,
		auth:      auth,
		rate:      NewRateLimiter(cfg),
		stamper:   stamper,
		metrics:   mustRegisterMetrics(),
		idemCache: map[string]*idempotentEntry{},
	}, nil
}

// Start binds the listener on cfg.GRPCPort and serves both registered
// services. Blocks until Stop is called or the listener errors.
//
// When cfg.Enabled is false, Start returns nil immediately — no listener
// bound, no goroutines spawned. Idempotent: subsequent Start calls return
// ErrServerNotReady.
func (s *Server) Start() error {
	if !s.cfg.Enabled {
		return nil
	}
	if !s.started.CompareAndSwap(false, true) {
		return ErrServerNotReady
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("inbound: listen on :%d: %w", s.cfg.GRPCPort, err)
	}
	s.listener = lis

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgBytes),
	}
	// Phase 2A.1: terminate mTLS in-process when AllowDevNoMTLS=false. The
	// CA bundle drives client-cert verification at the handshake; the
	// MTLSAuthenticator then layers SPKI pinning + registry cross-check on
	// top via the verified leaf cert in the gRPC peer info.
	if !s.cfg.AllowDevNoMTLS {
		creds, err := LoadServerTLS(s.cfg.MTLSCAPath, s.cfg.MTLSServerCertPath, s.cfg.MTLSServerKeyPath)
		if err != nil {
			return fmt.Errorf("inbound: configure TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}
	s.grpcServer = grpc.NewServer(opts...)

	// RTBExtensionPoint — handler in rtb_handler.go
	pb.RegisterRTBExtensionPointServer(s.grpcServer, s)

	// Discovery — handler in discovery_handler.go (registered in
	// registerDiscovery so the proto-import dependency is local to that file)
	s.registerDiscovery()

	// Idempotency cache sweeper.
	go s.sweepIdempotencyCache()

	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully shuts the server down. Safe to call multiple times. Calls
// after the first are no-ops.
func (s *Server) Stop() {
	if !s.stopped.CompareAndSwap(false, true) {
		return
	}
	if s.grpcServer != nil {
		// GracefulStop blocks until in-flight calls finish; cap with a
		// timer so a misbehaving caller can't block shutdown indefinitely.
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			s.grpcServer.Stop()
		}
	}
}

// sweepIdempotencyCache prunes entries past IdempotencyWindow. Runs every
// minute; cheap walk of the map.
func (s *Server) sweepIdempotencyCache() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for range tick.C {
		if s.stopped.Load() {
			return
		}
		now := time.Now()
		s.idemMu.Lock()
		for id, e := range s.idemCache {
			if now.Sub(e.timestamp) > s.cfg.IdempotencyWindow {
				delete(s.idemCache, id)
			}
		}
		s.idemMu.Unlock()
	}
}

// idempotencyLookup returns a cached response if present + not expired.
func (s *Server) idempotencyLookup(id string) *pb.RTBResponse {
	if id == "" {
		return nil
	}
	s.idemMu.Lock()
	defer s.idemMu.Unlock()
	e, ok := s.idemCache[id]
	if !ok {
		return nil
	}
	if time.Since(e.timestamp) > s.cfg.IdempotencyWindow {
		delete(s.idemCache, id)
		return nil
	}
	return e.response
}

// idempotencyStore caches a response keyed by the call's RTBRequest.id.
func (s *Server) idempotencyStore(id string, rsp *pb.RTBResponse) {
	if id == "" || rsp == nil {
		return
	}
	s.idemMu.Lock()
	defer s.idemMu.Unlock()
	s.idemCache[id] = &idempotentEntry{response: rsp, timestamp: time.Now()}
}

// registerDiscovery is implemented in discovery_handler.go so the
// generated discovery proto import stays local to that file. Stub here
// keeps the lifecycle code clean.
func (s *Server) registerDiscovery() {
	// implemented in discovery_handler.go via init()/explicit registration
	registerDiscoveryHandler(s)
}
