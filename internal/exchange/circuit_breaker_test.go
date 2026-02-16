package exchange

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/fpd"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
)

// ============================================================================
// CIRCUIT BREAKER TESTS
// ============================================================================

// TestExchange_InitBidderCircuitBreakers tests that circuit breakers are initialized for all enabled bidders
func TestExchange_InitBidderCircuitBreakers(t *testing.T) {
	registry := adapters.NewRegistry()

	// Register test adapters
	registry.Register("test1", &mockAdapter{}, adapters.BidderInfo{Enabled: true})
	registry.Register("test2", &mockAdapter{}, adapters.BidderInfo{Enabled: true})
	registry.Register("test3", &mockAdapter{}, adapters.BidderInfo{Enabled: false}) // Disabled

	ex := New(registry, DefaultConfig())

	// Verify circuit breakers created for enabled bidders only
	stats := ex.GetBidderCircuitBreakerStats()

	if _, exists := stats["test1"]; !exists {
		t.Error("Expected circuit breaker for enabled bidder test1")
	}
	if _, exists := stats["test2"]; !exists {
		t.Error("Expected circuit breaker for enabled bidder test2")
	}
	if _, exists := stats["test3"]; exists {
		t.Error("Expected NO circuit breaker for disabled bidder test3")
	}
}

// TestExchange_CircuitBreakerOpensAfterFailures tests that circuit opens after threshold failures
func TestExchange_CircuitBreakerOpensAfterFailures(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create a failing adapter
	failingAdapter := &mockAdapter{
		bidsErr: fmt.Errorf("bidder timeout"),
	}

	registry.Register("failing_bidder", failingAdapter, adapters.BidderInfo{
		Enabled: true,
	})

	config := DefaultConfig()
	config.MaxConcurrentBidders = 10

	ex := New(registry, config)

	// Get the circuit breaker
	breaker := ex.getBidderCircuitBreaker("failing_bidder")
	if breaker == nil {
		t.Fatal("Expected circuit breaker to exist")
	}

	// Verify initial state is closed
	if breaker.State() != idr.StateClosed {
		t.Errorf("Expected initial state to be closed, got %s", breaker.State())
	}

	// Create a test request
	bidReq := &openrtb.BidRequest{
		ID:  "test-circuit",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Call bidder 5 times (failure threshold)
	for i := 0; i < 5; i++ {
		ex.callBiddersWithFPD(
			context.Background(),
			bidReq,
			[]string{"failing_bidder"},
			100*time.Millisecond,
			fpd.BidderFPD{},
		)
	}

	// Circuit should now be open
	if breaker.State() != idr.StateOpen {
		t.Errorf("Expected circuit to be open after 5 failures, got state: %s", breaker.State())
	}
}

// TestExchange_CircuitBreakerSkipsBidderWhenOpen tests that open circuit skips bidder calls
func TestExchange_CircuitBreakerSkipsBidderWhenOpen(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create adapter - we'll check if it's called
	testAdapter := &mockAdapter{}

	registry.Register("test_bidder", testAdapter, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, DefaultConfig())

	// Manually force circuit breaker open
	breaker := ex.getBidderCircuitBreaker("test_bidder")
	if breaker == nil {
		t.Fatal("Expected circuit breaker to exist")
	}
	breaker.ForceOpen()

	// Create test request
	bidReq := &openrtb.BidRequest{
		ID:  "test-skip",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Call bidder - should be skipped
	results := ex.callBiddersWithFPD(
		context.Background(),
		bidReq,
		[]string{"test_bidder"},
		100*time.Millisecond,
		fpd.BidderFPD{},
	)

	// Verify result indicates circuit breaker
	result, exists := results["test_bidder"]
	if !exists {
		t.Fatal("Expected result for test_bidder")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error in result when circuit breaker is open")
	}

	// Circuit breaker failures should NOT be marked as timeouts
	if result.TimedOut {
		t.Error("Expected TimedOut=false when circuit breaker is open, got TimedOut=true")
	}
}

// TestExchange_CircuitBreakerRecordsSuccess tests that successful bids are recorded
func TestExchange_CircuitBreakerRecordsSuccess(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create successful adapter
	successAdapter := &mockAdapter{
		bids: []*adapters.TypedBid{
			{
				Bid: &openrtb.Bid{
					ID:    "bid1",
					ImpID: "imp1",
					Price: 1.50,
				},
				BidType: adapters.BidTypeBanner,
			},
		},
	}

	registry.Register("success_bidder", successAdapter, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, DefaultConfig())

	breaker := ex.getBidderCircuitBreaker("success_bidder")
	if breaker == nil {
		t.Fatal("Expected circuit breaker to exist")
	}

	// Record initial stats
	initialStats := breaker.Stats()

	// Create test request
	bidReq := &openrtb.BidRequest{
		ID:  "test-success",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Call bidder
	ex.callBiddersWithFPD(
		context.Background(),
		bidReq,
		[]string{"success_bidder"},
		100*time.Millisecond,
		fpd.BidderFPD{},
	)

	// Verify success was recorded
	finalStats := breaker.Stats()

	if finalStats.TotalSuccesses <= initialStats.TotalSuccesses {
		t.Error("Expected TotalSuccesses to increase after successful bid")
	}

	if finalStats.State != idr.StateClosed {
		t.Errorf("Expected circuit to remain closed after success, got %s", finalStats.State)
	}
}

// TestExchange_CircuitBreakerRecordsFailure tests that errors and timeouts are recorded
func TestExchange_CircuitBreakerRecordsFailure(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create failing adapter
	failingAdapter := &mockAdapter{
		bidsErr: fmt.Errorf("connection timeout"),
	}

	registry.Register("failing_bidder", failingAdapter, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, DefaultConfig())

	breaker := ex.getBidderCircuitBreaker("failing_bidder")
	if breaker == nil {
		t.Fatal("Expected circuit breaker to exist")
	}

	initialStats := breaker.Stats()

	// Create test request
	bidReq := &openrtb.BidRequest{
		ID:  "test-failure",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Call bidder
	ex.callBiddersWithFPD(
		context.Background(),
		bidReq,
		[]string{"failing_bidder"},
		100*time.Millisecond,
		fpd.BidderFPD{},
	)

	// Verify failure was recorded
	finalStats := breaker.Stats()

	if finalStats.TotalFailures <= initialStats.TotalFailures {
		t.Error("Expected TotalFailures to increase after failed bid")
	}

	if finalStats.Failures != 1 {
		t.Errorf("Expected current Failures to be 1, got %d", finalStats.Failures)
	}
}

// TestExchange_GetBidderCircuitBreakerStats tests the stats export function
func TestExchange_GetBidderCircuitBreakerStats(t *testing.T) {
	registry := adapters.NewRegistry()

	registry.Register("bidder1", &mockAdapter{}, adapters.BidderInfo{Enabled: true})
	registry.Register("bidder2", &mockAdapter{}, adapters.BidderInfo{Enabled: true})

	ex := New(registry, DefaultConfig())

	stats := ex.GetBidderCircuitBreakerStats()

	if len(stats) != 2 {
		t.Errorf("Expected stats for 2 bidders, got %d", len(stats))
	}

	for bidder, stat := range stats {
		if stat.State != idr.StateClosed {
			t.Errorf("Expected bidder %s to have closed circuit initially, got %s", bidder, stat.State)
		}
		if stat.TotalRequests != 0 {
			t.Errorf("Expected bidder %s to have 0 total requests initially, got %d", bidder, stat.TotalRequests)
		}
	}
}

// TestExchange_CircuitBreakerConcurrentAccess tests thread-safety of circuit breaker operations
func TestExchange_CircuitBreakerConcurrentAccess(t *testing.T) {
	registry := adapters.NewRegistry()

	registry.Register("concurrent_bidder", &mockAdapter{
		bids: []*adapters.TypedBid{
			{
				Bid: &openrtb.Bid{
					ID:    "bid1",
					ImpID: "imp1",
					Price: 1.0,
				},
				BidType: adapters.BidTypeBanner,
			},
		},
	}, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, DefaultConfig())

	bidReq := &openrtb.BidRequest{
		ID:  "test-concurrent",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Run 50 concurrent auctions
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ex.callBiddersWithFPD(
				context.Background(),
				bidReq,
				[]string{"concurrent_bidder"},
				100*time.Millisecond,
				fpd.BidderFPD{},
			)
		}()
	}

	wg.Wait()

	// Verify stats are consistent (no race conditions)
	breaker := ex.getBidderCircuitBreaker("concurrent_bidder")
	if breaker == nil {
		t.Fatal("Expected circuit breaker to exist")
	}

	stats := breaker.Stats()
	if stats.TotalSuccesses != 50 {
		t.Errorf("Expected 50 successful requests, got %d", stats.TotalSuccesses)
	}

	if stats.State != idr.StateClosed {
		t.Errorf("Expected circuit to remain closed, got %s", stats.State)
	}
}

// TestExchange_CircuitBreakerNoNilPanic tests that nil circuit breaker doesn't panic
func TestExchange_CircuitBreakerNoNilPanic(t *testing.T) {
	registry := adapters.NewRegistry()

	registry.Register("test_bidder", &mockAdapter{}, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, DefaultConfig())

	// Manually remove circuit breaker to test nil handling
	ex.bidderBreakersMu.Lock()
	delete(ex.bidderBreakers, "test_bidder")
	ex.bidderBreakersMu.Unlock()

	bidReq := &openrtb.BidRequest{
		ID:  "test-nil-check",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
	}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("callBiddersWithFPD panicked with nil circuit breaker: %v", r)
		}
	}()

	ex.callBiddersWithFPD(
		context.Background(),
		bidReq,
		[]string{"test_bidder"},
		100*time.Millisecond,
		fpd.BidderFPD{},
	)
}
