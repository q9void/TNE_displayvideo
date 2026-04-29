package inbound

import (
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

// RateLimiter implements per-agent and per-publisher QPS soft caps with a
// per-agent circuit breaker. Phase 2A uses a simple sliding-second token
// bucket: each Allow call records "now" against a per-key ring buffer,
// rejecting if the current second's count exceeds the cap.
//
// Concurrency: safe for concurrent use across the gRPC server's per-call
// goroutines.
type RateLimiter struct {
	cfg ServerConfig

	mu             sync.Mutex
	agentBuckets   map[string]*secondBucket
	publisherBkts  map[string]*secondBucket
	agentBreakers  map[string]*idr.CircuitBreaker
}

// NewRateLimiter constructs a RateLimiter using cfg's QPSPerAgent +
// QPSPerPublisher caps and circuit-breaker thresholds.
func NewRateLimiter(cfg ServerConfig) *RateLimiter {
	return &RateLimiter{
		cfg:           cfg.defaults(),
		agentBuckets:  map[string]*secondBucket{},
		publisherBkts: map[string]*secondBucket{},
		agentBreakers: map[string]*idr.CircuitBreaker{},
	}
}

// AllowAgent returns nil if the agent_id may make another call this
// second, or ErrRateLimitedPerAgent. Also checks the breaker — if open,
// returns ErrCircuitOpen.
func (r *RateLimiter) AllowAgent(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	br := r.breakerForLocked(agentID)
	if br.State() == idr.StateOpen {
		return ErrCircuitOpen
	}

	bucket, ok := r.agentBuckets[agentID]
	if !ok {
		bucket = &secondBucket{}
		r.agentBuckets[agentID] = bucket
	}
	if !bucket.allow(r.cfg.QPSPerAgent) {
		return ErrRateLimitedPerAgent
	}
	return nil
}

// AllowPublisher returns nil if mutations affecting publisherID may proceed
// this second, or ErrRateLimitedPerPublisher.
func (r *RateLimiter) AllowPublisher(publisherID string) error {
	if publisherID == "" {
		return nil // no publisher context → no per-publisher check
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.publisherBkts[publisherID]
	if !ok {
		bucket = &secondBucket{}
		r.publisherBkts[publisherID] = bucket
	}
	if !bucket.allow(r.cfg.QPSPerPublisher) {
		return ErrRateLimitedPerPublisher
	}
	return nil
}

// RecordSuccess feeds a success into the per-agent breaker so it can
// half-open. Called by handlers after a successful GetMutations.
func (r *RateLimiter) RecordSuccess(agentID string) {
	r.mu.Lock()
	br := r.breakerForLocked(agentID)
	r.mu.Unlock()
	// Drive the breaker via a no-op success Execute. The breaker's
	// Execute() is the only way it observes outcomes.
	_ = br.Execute(func() error { return nil })
}

// RecordFailure feeds a failure into the per-agent breaker.
func (r *RateLimiter) RecordFailure(agentID string, err error) {
	r.mu.Lock()
	br := r.breakerForLocked(agentID)
	r.mu.Unlock()
	_ = br.Execute(func() error { return err })
}

func (r *RateLimiter) breakerForLocked(agentID string) *idr.CircuitBreaker {
	br, ok := r.agentBreakers[agentID]
	if !ok {
		br = idr.NewCircuitBreaker(&idr.CircuitBreakerConfig{
			FailureThreshold: r.cfg.CircuitFailureThreshold,
			SuccessThreshold: 2,
			Timeout:          r.cfg.CircuitTimeout,
			MaxConcurrent:    100,
		})
		r.agentBreakers[agentID] = br
	}
	return br
}

// secondBucket is a single-key, sliding-second counter. Not concurrency-
// safe in isolation; RateLimiter holds its mutex while consulting it.
type secondBucket struct {
	currentSecond int64
	count         int
}

func (b *secondBucket) allow(cap int) bool {
	now := time.Now().Unix()
	if now != b.currentSecond {
		b.currentSecond = now
		b.count = 0
	}
	if b.count >= cap {
		return false
	}
	b.count++
	return true
}
