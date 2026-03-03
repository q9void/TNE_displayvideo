package idr

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	if cb.State() != StateClosed {
		t.Errorf("expected initial state to be closed, got %s", cb.State())
	}

	stats := cb.Stats()
	if stats.TotalRequests != 0 {
		t.Errorf("expected 0 total requests, got %d", stats.TotalRequests)
	}
}

func TestCircuitBreakerSuccessfulRequests(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Execute successful requests
	for i := 0; i < 10; i++ {
		err := cb.Execute(func() error {
			return nil
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state to remain closed, got %s", cb.State())
	}

	stats := cb.Stats()
	if stats.TotalSuccesses != 10 {
		t.Errorf("expected 10 successes, got %d", stats.TotalSuccesses)
	}
}

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	testErr := errors.New("test error")

	// Cause failures
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after 3 failures, got %s", cb.State())
	}

	// Next request should be rejected
	err := cb.Execute(func() error {
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}

	stats := cb.Stats()
	if stats.TotalRejected != 1 {
		t.Errorf("expected 1 rejected, got %d", stats.TotalRejected)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state to be open, got %s", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Next request should transition to half-open and succeed
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error in half-open, got %v", err)
	}

	// Should now be closed
	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after success in half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open state
	cb.Execute(func() error {
		return errors.New("error")
	})

	// Should be back to open
	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after failure in half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state to be open, got %s", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after reset, got %s", cb.State())
	}

	// Should accept requests
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after reset, got %v", err)
	}
}

func TestCircuitBreakerForceOpen(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	cb.ForceOpen()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after ForceOpen, got %s", cb.State())
	}

	err := cb.Execute(func() error {
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 2,
		Timeout:          time.Second,
		MaxConcurrent:    0, // No limit
	})

	var wg sync.WaitGroup
	var successCount int64

	// Run 100 concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Execute(func() error {
				time.Sleep(time.Millisecond)
				return nil
			})
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 100 {
		t.Errorf("expected 100 successes, got %d", successCount)
	}
}

func TestCircuitBreakerMaxConcurrent(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 2,
		Timeout:          time.Second,
		MaxConcurrent:    2,
	})

	var wg sync.WaitGroup
	var rejectCount int64
	started := make(chan struct{})

	// Start 2 long-running requests
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Execute(func() error {
				started <- struct{}{}
				time.Sleep(100 * time.Millisecond)
				return nil
			})
		}()
	}

	// Wait for both to start
	<-started
	<-started

	// Try to start more - should be rejected
	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error {
			return nil
		})
		if err != nil {
			atomic.AddInt64(&rejectCount, 1)
		}
	}

	wg.Wait()

	if rejectCount != 5 {
		t.Errorf("expected 5 rejections due to max concurrent, got %d", rejectCount)
	}
}

func TestCircuitBreakerOnStateChange(t *testing.T) {
	var stateChanges []string
	var mu sync.Mutex

	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(from, to string) {
			mu.Lock()
			stateChanges = append(stateChanges, from+"->"+to)
			mu.Unlock()
		},
	})

	// Trigger open
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	// Wait for callback
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(stateChanges) == 0 || stateChanges[0] != "closed->open" {
		t.Errorf("expected closed->open transition, got %v", stateChanges)
	}
	mu.Unlock()
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// 5 successes
	for i := 0; i < 5; i++ {
		cb.Execute(func() error { return nil })
	}

	// 3 failures
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("error") })
	}

	stats := cb.Stats()

	if stats.TotalRequests != 8 {
		t.Errorf("expected 8 total requests, got %d", stats.TotalRequests)
	}
	if stats.TotalSuccesses != 5 {
		t.Errorf("expected 5 successes, got %d", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 3 {
		t.Errorf("expected 3 failures, got %d", stats.TotalFailures)
	}
	if stats.State != StateClosed {
		t.Errorf("expected closed state, got %s", stats.State)
	}
}

func TestIsOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          time.Second,
	})

	if cb.IsOpen() {
		t.Error("expected circuit to not be open initially")
	}

	cb.Execute(func() error { return errors.New("error") })

	if !cb.IsOpen() {
		t.Error("expected circuit to be open after failure")
	}
}

func TestRecordFailure_Direct(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Record initial stats
	initialStats := cb.Stats()
	if initialStats.TotalFailures != 0 {
		t.Errorf("expected 0 initial failures, got %d", initialStats.TotalFailures)
	}

	// Record failures directly (without Execute)
	cb.RecordFailure()
	cb.RecordFailure()

	// Check stats after 2 failures
	stats := cb.Stats()
	if stats.TotalFailures != 2 {
		t.Errorf("expected 2 failures, got %d", stats.TotalFailures)
	}
	if stats.Failures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", stats.Failures)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected state to remain closed after 2 failures (threshold is 3), got %s", cb.State())
	}

	// Record one more failure to open circuit
	cb.RecordFailure()

	// Circuit should now be open
	if cb.State() != StateOpen {
		t.Errorf("expected circuit to be open after 3 failures, got %s", cb.State())
	}

	finalStats := cb.Stats()
	if finalStats.TotalFailures != 3 {
		t.Errorf("expected 3 total failures, got %d", finalStats.TotalFailures)
	}
}

func TestRecordSuccess_Direct(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Record initial stats
	initialStats := cb.Stats()
	if initialStats.TotalSuccesses != 0 {
		t.Errorf("expected 0 initial successes, got %d", initialStats.TotalSuccesses)
	}

	// Record successes directly (without Execute)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()

	// Check stats
	stats := cb.Stats()
	if stats.TotalSuccesses != 3 {
		t.Errorf("expected 3 successes, got %d", stats.TotalSuccesses)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected state to remain closed, got %s", cb.State())
	}
}

func TestRecordSuccess_ResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Record some failures (not enough to open)
	cb.RecordFailure()
	cb.RecordFailure()

	stats := cb.Stats()
	if stats.Failures != 2 {
		t.Errorf("expected 2 failures, got %d", stats.Failures)
	}

	// Record a success - should reset failure count
	cb.RecordSuccess()

	stats = cb.Stats()
	if stats.Failures != 0 {
		t.Errorf("expected failure count to reset to 0 after success, got %d", stats.Failures)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected circuit to remain closed, got %s", cb.State())
	}
}

func TestRecordFailure_WithSuccessesInBetween(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	// Record failures and successes alternating
	cb.RecordFailure()
	cb.RecordSuccess() // Resets failure count
	cb.RecordFailure()
	cb.RecordSuccess() // Resets failure count
	cb.RecordFailure()

	// Should still be closed because successes reset failure count
	if cb.State() != StateClosed {
		t.Errorf("expected circuit to remain closed, got %s", cb.State())
	}

	stats := cb.Stats()
	if stats.TotalFailures != 3 {
		t.Errorf("expected 3 total failures, got %d", stats.TotalFailures)
	}
	if stats.TotalSuccesses != 2 {
		t.Errorf("expected 2 total successes, got %d", stats.TotalSuccesses)
	}
	if stats.Failures != 1 {
		t.Errorf("expected 1 current consecutive failure, got %d", stats.Failures)
	}
}

func TestRecordFailureSuccess_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	var wg sync.WaitGroup

	// Run 50 concurrent RecordSuccess and 50 concurrent RecordFailure
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
		go func() {
			defer wg.Done()
			cb.RecordFailure()
		}()
	}

	wg.Wait()

	// Verify counts are correct
	stats := cb.Stats()
	if stats.TotalSuccesses != 50 {
		t.Errorf("expected 50 successes, got %d", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 50 {
		t.Errorf("expected 50 failures, got %d", stats.TotalFailures)
	}

	// Circuit should still be closed (failures < threshold)
	if cb.State() != StateClosed {
		t.Errorf("expected circuit to remain closed, got %s", cb.State())
	}
}

// TestCircuitBreakerIsOpenTriggersHalfOpenTransition verifies that calling IsOpen()
// after the recovery timeout has elapsed transitions OPEN -> HALF-OPEN and allows
// a probe request through. This tests the exchange's usage pattern (IsOpen check +
// manual RecordFailure/RecordSuccess) rather than the Execute() path.
func TestCircuitBreakerIsOpenTriggersHalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Trip the circuit via direct RecordFailure (exchange pattern)
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatalf("expected state open after threshold failures, got %s", cb.State())
	}
	if !cb.IsOpen() {
		t.Fatal("IsOpen() should return true when circuit is open")
	}

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// IsOpen() should now trigger OPEN -> HALF-OPEN transition and return false
	if cb.IsOpen() {
		t.Fatal("IsOpen() should return false after recovery timeout (should transition to half-open)")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("expected state half-open after timeout, got %s", cb.State())
	}

	// A successful probe closes the circuit after SuccessThreshold successes
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("expected circuit closed after successes in half-open, got %s", cb.State())
	}
}
