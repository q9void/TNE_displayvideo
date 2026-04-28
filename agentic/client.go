package agentic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

// ClientConfig governs the outbound RTBExtensionPoint client.
type ClientConfig struct {
	// DefaultTmaxMs is the global per-call deadline ceiling. Per-agent
	// AgentEndpoint.TmaxMs may shrink this further but never exceed it.
	DefaultTmaxMs int

	// AuctionSafetyMs is subtracted from any auction-context deadline when
	// computing the effective per-call deadline. Default 50 ms.
	AuctionSafetyMs int

	// APIKey is the global x-aamp-key value sent in gRPC metadata for any
	// agent that declares auth=api_key_header and has no per-agent override.
	APIKey string

	// PerAgentAPIKeys overrides APIKey for specific agents (keyed by agent_id).
	PerAgentAPIKeys map[string]string

	// CircuitBreaker config. Threshold is failure count before the breaker
	// opens (count-based, not rate-based — Phase 1 simplification).
	CircuitFailureThreshold int           // default 5
	CircuitSuccessThreshold int           // default 2
	CircuitTimeout          time.Duration // default 30s

	// MaxRecvMsgBytes caps inbound gRPC message size. Default 4 MiB.
	MaxRecvMsgBytes int

	// AllowInsecure permits dialling plain "grpc://" / "host:port" endpoints
	// without TLS. Production should always set this false; it is true in
	// tests and dev. Defaults to false.
	AllowInsecure bool
}

// Client fans RTBRequests out to every agent eligible for the lifecycle,
// collects mutations within the configured tmax, and returns a merged
// DispatchResult. Safe for concurrent use across many auctions.
type Client struct {
	reg     *Registry
	cfg     ClientConfig
	stamper OriginatorStamper

	mu       sync.RWMutex
	conns    map[string]*grpc.ClientConn
	stubs    map[string]pb.RTBExtensionPointClient
	breakers map[string]*idr.CircuitBreaker
}

// NewClient constructs a Client and dials every agent in the registry
// eagerly. Returns the first dial error if any.
func NewClient(reg *Registry, cfg ClientConfig, stamper OriginatorStamper) (*Client, error) {
	if cfg.DefaultTmaxMs <= 0 {
		cfg.DefaultTmaxMs = 30
	}
	if cfg.AuctionSafetyMs <= 0 {
		cfg.AuctionSafetyMs = 50
	}
	if cfg.CircuitFailureThreshold <= 0 {
		cfg.CircuitFailureThreshold = 5
	}
	if cfg.CircuitSuccessThreshold <= 0 {
		cfg.CircuitSuccessThreshold = 2
	}
	if cfg.CircuitTimeout <= 0 {
		cfg.CircuitTimeout = 30 * time.Second
	}
	if cfg.MaxRecvMsgBytes <= 0 {
		cfg.MaxRecvMsgBytes = 4 * 1024 * 1024
	}

	c := &Client{
		reg:      reg,
		cfg:      cfg,
		stamper:  stamper,
		conns:    map[string]*grpc.ClientConn{},
		stubs:    map[string]pb.RTBExtensionPointClient{},
		breakers: map[string]*idr.CircuitBreaker{},
	}

	for _, agent := range reg.AllAgents() {
		if err := c.dialOne(agent); err != nil {
			c.Close()
			return nil, fmt.Errorf("agentic: dial %s: %w", agent.ID, err)
		}
	}
	return c, nil
}

func (c *Client) dialOne(agent AgentEndpoint) error {
	tx, ok := agent.PrimaryTransport()
	if !ok {
		return fmt.Errorf("agent %q has no endpoints", agent.ID)
	}

	var creds credentials.TransportCredentials
	switch tx.Transport {
	case "grpcs":
		creds = credentials.NewClientTLSFromCert(nil, "")
	case "grpc":
		if !c.cfg.AllowInsecure {
			return fmt.Errorf("agent %q uses plain grpc:// — refused (set AllowInsecure=true for dev/test)", agent.ID)
		}
		creds = insecure.NewCredentials()
	default:
		return fmt.Errorf("agent %q transport %q unsupported in Phase 1", agent.ID, tx.Transport)
	}

	conn, err := grpc.NewClient(tx.URL,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.cfg.MaxRecvMsgBytes),
		),
	)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conns[agent.ID] = conn
	c.stubs[agent.ID] = pb.NewRTBExtensionPointClient(conn)
	c.breakers[agent.ID] = idr.NewCircuitBreaker(&idr.CircuitBreakerConfig{
		FailureThreshold: c.cfg.CircuitFailureThreshold,
		SuccessThreshold: c.cfg.CircuitSuccessThreshold,
		Timeout:          c.cfg.CircuitTimeout,
		MaxConcurrent:    100,
	})
	c.mu.Unlock()
	return nil
}

// Close tears down every gRPC ClientConn. Safe to call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var firstErr error
	for id, conn := range c.conns {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close %s: %w", id, err)
		}
	}
	c.conns = map[string]*grpc.ClientConn{}
	c.stubs = map[string]pb.RTBExtensionPointClient{}
	c.breakers = map[string]*idr.CircuitBreaker{}
	return firstErr
}

// Dispatch fans out req to every agent declaring lc and merges their
// mutations into a single DispatchResult. Never returns an error — agent
// failures are logged on AgentCallStat.Status. The auction must continue
// regardless of agent outcomes.
func (c *Client) Dispatch(ctx context.Context, req *pb.RTBRequest, lc Lifecycle) DispatchResult {
	out := DispatchResult{
		Origins:      map[*pb.Mutation]MutationOrigin{},
		DispatchedAt: time.Now(),
	}
	if c.reg == nil {
		return out
	}
	agents := c.reg.AgentsForLifecycle(lc)
	if len(agents) == 0 {
		return out
	}

	// Stamp the request once before fanning out. The stamper sets
	// Originator{TYPE_SSP, id=…} and Lifecycle.
	c.stamper.StampRTBRequest(req, lc)

	deadlineMs := c.computeDeadlineMs(ctx)

	resCh := make(chan result, len(agents))
	var wg sync.WaitGroup

	for _, agent := range agents {
		wg.Add(1)
		go func(a AgentEndpoint) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					resCh <- result{
						agent: a,
						stat: AgentCallStat{
							AgentID:       a.ID,
							Lifecycle:     lc,
							LifecycleName: lc.String(),
							Status:        "error",
							Error:         fmt.Sprintf("panic: %v", r),
						},
					}
				}
			}()
			resCh <- c.callOne(ctx, a, lc, req, deadlineMs)
		}(agent)
	}

	// Hard wait — even if individual ctx aren't cancelled, we don't await
	// past the master deadline.
	master := time.NewTimer(time.Duration(deadlineMs) * time.Millisecond)
	defer master.Stop()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	collected := 0
loop:
	for {
		select {
		case <-master.C:
			out.Truncated = true
			break loop
		case r, ok := <-resCh:
			if !ok {
				break loop
			}
			out.AgentStats = append(out.AgentStats, r.stat)
			for _, m := range r.muts {
				out.Mutations = append(out.Mutations, m)
				out.Origins[m] = MutationOrigin{AgentID: r.agent.ID, Priority: r.agent.Priority}
			}
			collected++
			if collected == len(agents) {
				break loop
			}
		}
	}
	return out
}

func (c *Client) callOne(ctx context.Context, agent AgentEndpoint, lc Lifecycle, req *pb.RTBRequest, deadlineMs int) result {
	startMs := time.Now()
	stat := AgentCallStat{
		AgentID:       agent.ID,
		Lifecycle:     lc,
		LifecycleName: lc.String(),
	}

	c.mu.RLock()
	stub, hasStub := c.stubs[agent.ID]
	breaker, hasBr := c.breakers[agent.ID]
	c.mu.RUnlock()

	if !hasStub || !hasBr {
		stat.Status = "error"
		stat.Error = "agent not dialled"
		stat.LatencyMs = time.Since(startMs).Milliseconds()
		return result{agent: agent, stat: stat}
	}

	// Per-agent budget — bounded by configured min(global, agent declared).
	agentBudget := deadlineMs
	if agent.TmaxMs > 0 && int(agent.TmaxMs) < agentBudget {
		agentBudget = int(agent.TmaxMs)
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(agentBudget)*time.Millisecond)
	defer cancel()

	// API key in metadata.
	if key := c.keyFor(agent); key != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-aamp-key", key)
	}

	// Per-call clone so concurrent goroutines don't race on the shared req.
	cloned := proto.Clone(req).(*pb.RTBRequest)

	var rsp *pb.RTBResponse
	err := breaker.Execute(func() error {
		var e error
		rsp, e = stub.GetMutations(callCtx, cloned)
		return e
	})
	stat.LatencyMs = time.Since(startMs).Milliseconds()

	switch {
	case err == nil:
		stat.Status = "ok"
		if rsp != nil {
			stat.MutationCount = len(rsp.GetMutations())
			if md := rsp.GetMetadata(); md != nil {
				stat.ModelVersion = md.GetModelVersion()
			}
		}
		return result{agent: agent, stat: stat, muts: rsp.GetMutations()}
	case errors.Is(err, idr.ErrCircuitOpen):
		stat.Status = "circuit_open"
		stat.Error = err.Error()
		return result{agent: agent, stat: stat}
	case isContextTimeout(err, callCtx):
		stat.Status = "timeout"
		stat.Error = err.Error()
		return result{agent: agent, stat: stat}
	default:
		stat.Status = "error"
		stat.Error = err.Error()
		return result{agent: agent, stat: stat}
	}
}

type result struct {
	agent AgentEndpoint
	stat  AgentCallStat
	muts  []*pb.Mutation
}

func (c *Client) keyFor(agent AgentEndpoint) string {
	if k, ok := c.cfg.PerAgentAPIKeys[agent.ID]; ok && k != "" {
		return k
	}
	return c.cfg.APIKey
}

// computeDeadlineMs returns the per-call ceiling in milliseconds. It is
// the smaller of (DefaultTmaxMs, ctx.Deadline()-AuctionSafetyMs).
func (c *Client) computeDeadlineMs(ctx context.Context) int {
	d := c.cfg.DefaultTmaxMs
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl).Milliseconds() - int64(c.cfg.AuctionSafetyMs)
		if remaining < int64(d) {
			d = int(remaining)
		}
	}
	if d < 1 {
		d = 1
	}
	return d
}

func isContextTimeout(err error, ctx context.Context) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return true
	}
	if s, ok := status.FromError(err); ok {
		return s.Code().String() == "DeadlineExceeded"
	}
	return false
}

// WrapAsRTBRequest builds a minimal pb.RTBRequest from an internal openrtb
// BidRequest. Phase 1 carries id, lifecycle, originator, tmax, and a
// minimal pb BidRequest containing only the fields most agents need (id).
// A fuller conversion is documented as a Phase 2 follow-up.
//
// This intentionally does NOT round-trip the entire request through proto —
// that would require a 200+ LOC field-by-field copy and is out of Phase 1
// scope. Real agent vendors will negotiate the conversion gap during
// integration.
func WrapAsRTBRequest(req *openrtb.BidRequest, lc Lifecycle, tmaxMs int32) *pb.RTBRequest {
	if req == nil {
		return nil
	}
	out := &pb.RTBRequest{
		Id:   proto.String(req.ID),
		Tmax: proto.Int32(tmaxMs),
	}
	// Lifecycle and Originator are filled in by Stamper.StampRTBRequest at
	// Dispatch time — leaving them blank here is intentional.
	_ = lc
	return out
}
