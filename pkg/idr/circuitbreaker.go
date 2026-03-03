package idr

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Circuit breaker states
const (
	StateClosed   = "closed"    // Normal operation
	StateOpen     = "open"      // Failing, rejecting requests
	StateHalfOpen = "half-open" // Testing if service recovered
)

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           // Failures before opening circuit
	SuccessThreshold int           // Successes to close circuit from half-open
	Timeout          time.Duration // Time to wait before half-open
	MaxConcurrent    int           // Max concurrent requests (0 = unlimited)
	OnStateChange    func(from, to string)
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		MaxConcurrent:    100,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig

	mu              sync.RWMutex
	state           string
	failures        int
	successes       int
	lastFailureTime time.Time
	concurrent      int

	// Metrics
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64
	totalRejected  int64

	// Callback lifecycle management
	callbackWg sync.WaitGroup
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err)
	return err
}

// beforeRequest checks if the request should proceed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case StateClosed:
		// Check concurrent limit
		if cb.config.MaxConcurrent > 0 && cb.concurrent >= cb.config.MaxConcurrent {
			cb.totalRejected++
			return errors.New("max concurrent requests exceeded")
		}
		cb.concurrent++
		return nil

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.setState(StateHalfOpen)
			cb.concurrent++
			return nil
		}
		cb.totalRejected++
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests through
		if cb.concurrent < 1 { // Only allow one request at a time in half-open
			cb.concurrent++
			return nil
		}
		cb.totalRejected++
		return ErrCircuitOpen
	}

	return nil
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.concurrent--

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.totalFailures++
	cb.failures++
	cb.successes = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	cb.totalSuccesses++
	cb.successes++

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		if cb.successes >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
			cb.failures = 0
		}
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState string) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.successes = 0

	if cb.config.OnStateChange != nil {
		// Track callback goroutine for graceful shutdown
		cb.callbackWg.Add(1)
		go func(from, to string) {
			defer cb.callbackWg.Done()

			// Prevent callback from blocking forever - use 5 second timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Run callback in nested goroutine with done channel and panic recovery
			done := make(chan struct{})
			go func() {
				defer func() {
					// Recover from panics in callback to prevent crashes
					if r := recover(); r != nil {
						// In production, this would use a structured logger
						// log.Error("circuit breaker callback panicked", "panic", r, "from", from, "to", to)
					}
					close(done)
				}()
				cb.config.OnStateChange(from, to)
			}()

			// Wait for either completion or timeout
			select {
			case <-done:
				// Callback completed successfully
			case <-ctx.Done():
				// Callback timed out - nested goroutine will be orphaned but outer goroutine exits
				// WARNING: If callback blocks indefinitely, the nested goroutine cannot be killed.
				// This is a Go runtime limitation. Callbacks MUST be non-blocking or have internal timeouts.
				// In production, this would use a structured logger
				// log.Warn("circuit breaker callback timed out - goroutine may be orphaned", "from", from, "to", to, "timeout", "5s")
			}
		}(oldState, newState)
	}
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitBreakerStats{
		State:          cb.state,
		TotalRequests:  cb.totalRequests,
		TotalFailures:  cb.totalFailures,
		TotalSuccesses: cb.totalSuccesses,
		TotalRejected:  cb.totalRejected,
		Failures:       cb.failures,
		Concurrent:     cb.concurrent,
	}
}

// CircuitBreakerStats holds circuit breaker statistics
type CircuitBreakerStats struct {
	State          string `json:"state"`
	TotalRequests  int64  `json:"total_requests"`
	TotalFailures  int64  `json:"total_failures"`
	TotalSuccesses int64  `json:"total_successes"`
	TotalRejected  int64  `json:"total_rejected"`
	Failures       int    `json:"current_failures"`
	Concurrent     int    `json:"concurrent"`
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.setState(StateClosed)
	cb.failures = 0
	cb.successes = 0
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.setState(StateOpen)
	cb.lastFailureTime = time.Now()
}

// IsOpen returns true if the circuit breaker is currently blocking requests.
// If the circuit was open but the recovery timeout has elapsed, it transitions
// to half-open and returns false, allowing a probe request through.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.config.Timeout {
		cb.setState(StateHalfOpen)
	}
	return cb.state == StateOpen
}

// Close waits for any pending state change callbacks to complete.
// Call this during graceful shutdown to ensure all callbacks finish.
func (cb *CircuitBreaker) Close() {
	cb.callbackWg.Wait()
}

// RecordFailure records a failure without executing a function
// Useful for external failure tracking (e.g., timeouts, errors from async calls)
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.recordFailure()
}

// RecordSuccess records a success without executing a function
// Useful for external success tracking (e.g., successful async calls)
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.recordSuccess()
}
