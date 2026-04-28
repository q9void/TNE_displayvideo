package adcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

// ClientConfig governs the outbound AdCP client. Fields mirror
// agentic.ClientConfig where the semantics overlap, so operators flipping
// between the two integrations have a consistent mental model.
type ClientConfig struct {
	// DefaultTmaxMs is the global per-call deadline ceiling. Per-agent
	// Agent.TmaxMs may shrink this further but never exceed it.
	DefaultTmaxMs int

	// AuctionSafetyMs is subtracted from any auction-context deadline
	// when computing the effective per-call deadline. Default 50 ms.
	AuctionSafetyMs int

	// APIKey is the global Authorization header value sent for any agent
	// that declares auth=api_key_header and has no per-agent override.
	APIKey string

	// PerAgentAPIKeys overrides APIKey for specific agents (keyed by
	// agent_id).
	PerAgentAPIKeys map[string]string

	// CircuitBreaker config. Threshold is failure count before the breaker
	// opens (count-based, not rate-based — Phase 1 simplification).
	CircuitFailureThreshold int           // default 5
	CircuitSuccessThreshold int           // default 2
	CircuitTimeout          time.Duration // default 30s

	// MaxSignalsPerResponse caps how many signals one get_signals response
	// may contribute to the merged DispatchResult. Excess signals are
	// dropped with ErrSignalsCapExceeded.
	MaxSignalsPerResponse int

	// AllowInsecure permits dialing plain http:// endpoints. Production
	// must always set this false; it is true in tests/dev. Defaults false.
	AllowInsecure bool

	// HTTPClient is the underlying transport. nil ⇒ a default
	// http.Client with sane timeouts is constructed in NewClient.
	HTTPClient *http.Client
}

// Client fans capability calls out to every agent eligible for the
// lifecycle, collects results within the configured tmax, and returns a
// merged DispatchResult. Safe for concurrent use across many auctions.
//
// Phase 1: the actual MCP/JSON-RPC framing is a stub — Call returns
// ErrNotImplemented for every capability. The wiring (registry,
// circuit breaker, tmax, consent) is fully exercised so operators can
// stage-flip ADCP_ENABLED=true and watch the control plane without
// integrating a real agent. Phase 2 replaces the stub with real RPC.
type Client struct {
	reg *Registry
	cfg ClientConfig
	hc  *http.Client

	mu       sync.RWMutex
	breakers map[string]*idr.CircuitBreaker
}

// NewClient constructs a Client and validates every agent in the
// registry. Returns the first validation error if any.
func NewClient(reg *Registry, cfg ClientConfig) (*Client, error) {
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
	if cfg.MaxSignalsPerResponse <= 0 {
		cfg.MaxSignalsPerResponse = 256
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{
			Timeout: time.Duration(cfg.DefaultTmaxMs+cfg.AuctionSafetyMs) * time.Millisecond * 4,
		}
	}

	c := &Client{
		reg:      reg,
		cfg:      cfg,
		hc:       hc,
		breakers: map[string]*idr.CircuitBreaker{},
	}

	for _, a := range reg.AllAgents() {
		if err := c.validateAndRegister(a); err != nil {
			return nil, fmt.Errorf("adcp: validate %s: %w", a.ID, err)
		}
	}
	return c, nil
}

func (c *Client) validateAndRegister(a Agent) error {
	tx, ok := a.PrimaryTransport()
	if !ok {
		return fmt.Errorf("agent %q has no endpoints", a.ID)
	}

	switch tx.Transport {
	case "mcp", "https":
		// canonical AdCP transports — accepted
	case "http":
		if !c.cfg.AllowInsecure {
			return fmt.Errorf("%w: agent %q transport=http", ErrInsecureTransport, a.ID)
		}
	default:
		return fmt.Errorf("%w: agent %q transport=%q", ErrUnsupportedTransport, a.ID, tx.Transport)
	}

	c.mu.Lock()
	c.breakers[a.ID] = idr.NewCircuitBreaker(&idr.CircuitBreakerConfig{
		FailureThreshold: c.cfg.CircuitFailureThreshold,
		SuccessThreshold: c.cfg.CircuitSuccessThreshold,
		Timeout:          c.cfg.CircuitTimeout,
		MaxConcurrent:    100,
	})
	c.mu.Unlock()
	return nil
}

// Close is a no-op today (HTTP is connectionless). Defined so the
// caller-side teardown looks identical to agentic.Client.Close.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.breakers = map[string]*idr.CircuitBreaker{}
	return nil
}

// Dispatch fans a single capability out to every agent declaring lc and
// merges their results into a DispatchResult. Never returns an error —
// agent failures are recorded on CallStat.Status. The auction must
// continue regardless of agent outcomes.
//
// Phase 1: every call resolves to ErrNotImplemented (status="not_implemented").
func (c *Client) Dispatch(ctx context.Context, cap Capability, lc Lifecycle) DispatchResult {
	out := DispatchResult{DispatchedAt: time.Now()}
	if c.reg == nil {
		return out
	}
	agents := c.reg.AgentsForLifecycle(lc)
	if len(agents) == 0 {
		return out
	}

	deadlineMs := c.computeDeadlineMs(ctx)

	type result struct {
		stat    CallStat
		signals []Signal
	}
	resCh := make(chan result, len(agents))
	var wg sync.WaitGroup

	for _, a := range agents {
		if !a.HasCapability(cap) {
			continue
		}
		wg.Add(1)
		go func(a Agent) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					resCh <- result{stat: CallStat{
						AgentID:       a.ID,
						Capability:    string(cap),
						Lifecycle:     lc,
						LifecycleName: lc.String(),
						Status:        "error",
						Error:         fmt.Sprintf("panic: %v", r),
					}}
				}
			}()
			stat, sigs := c.callOne(ctx, a, cap, lc, deadlineMs)
			resCh <- result{stat: stat, signals: sigs}
		}(a)
	}

	master := time.NewTimer(time.Duration(deadlineMs) * time.Millisecond)
	defer master.Stop()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	signalsBudget := c.cfg.MaxSignalsPerResponse
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
			out.CallStats = append(out.CallStats, r.stat)
			for _, sig := range r.signals {
				if signalsBudget <= 0 {
					out.Truncated = true
					break
				}
				out.Signals = append(out.Signals, sig)
				signalsBudget--
			}
		}
	}
	return out
}

func (c *Client) callOne(ctx context.Context, a Agent, cap Capability, lc Lifecycle, deadlineMs int) (CallStat, []Signal) {
	startMs := time.Now()
	stat := CallStat{
		AgentID:       a.ID,
		Capability:    string(cap),
		Lifecycle:     lc,
		LifecycleName: lc.String(),
	}

	c.mu.RLock()
	breaker, hasBr := c.breakers[a.ID]
	c.mu.RUnlock()
	if !hasBr {
		stat.Status = "error"
		stat.Error = "agent not registered"
		stat.LatencyMs = time.Since(startMs).Milliseconds()
		return stat, nil
	}

	agentBudget := deadlineMs
	if a.TmaxMs > 0 && int(a.TmaxMs) < agentBudget {
		agentBudget = int(a.TmaxMs)
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(agentBudget)*time.Millisecond)
	defer cancel()

	err := breaker.Execute(func() error {
		return c.invoke(callCtx, a, cap)
	})
	stat.LatencyMs = time.Since(startMs).Milliseconds()

	switch {
	case err == nil:
		stat.Status = "ok"
		return stat, nil
	case errors.Is(err, ErrNotImplemented):
		stat.Status = "not_implemented"
		stat.Error = err.Error()
		return stat, nil
	case errors.Is(err, idr.ErrCircuitOpen):
		stat.Status = "circuit_open"
		stat.Error = err.Error()
		return stat, nil
	case isContextTimeout(err, callCtx):
		stat.Status = "timeout"
		stat.Error = err.Error()
		return stat, nil
	default:
		stat.Status = "error"
		stat.Error = err.Error()
		return stat, nil
	}
}

// invoke is the protocol-specific RPC. Phase 1 returns ErrNotImplemented
// for every capability — Phase 2 replaces this with real MCP/JSON-RPC
// framing over c.hc.
func (c *Client) invoke(_ context.Context, _ Agent, _ Capability) error {
	return ErrNotImplemented
}

func (c *Client) keyFor(a Agent) string {
	if k, ok := c.cfg.PerAgentAPIKeys[a.ID]; ok && k != "" {
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
	// Best-effort string match for transport-level deadline errors that
	// don't wrap context.DeadlineExceeded directly.
	if err != nil && strings.Contains(err.Error(), "deadline exceeded") {
		return true
	}
	return false
}
