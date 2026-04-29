package inbound

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_perAgent_allowsBelowCap(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerAgent: 5})
	for i := 0; i < 5; i++ {
		require.NoError(t, rl.AllowAgent("a"))
	}
}

func TestRateLimit_perAgent_blocksAboveCap(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerAgent: 3})
	require.NoError(t, rl.AllowAgent("a"))
	require.NoError(t, rl.AllowAgent("a"))
	require.NoError(t, rl.AllowAgent("a"))

	err := rl.AllowAgent("a")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimitedPerAgent))
}

func TestRateLimit_perAgent_independentBuckets(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerAgent: 1})
	require.NoError(t, rl.AllowAgent("a"))
	require.NoError(t, rl.AllowAgent("b"), "different agent should have its own bucket")

	require.Error(t, rl.AllowAgent("a"))
	require.Error(t, rl.AllowAgent("b"))
}

func TestRateLimit_perPublisher_emptyIDIsAlwaysAllowed(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerPublisher: 1})
	require.NoError(t, rl.AllowPublisher(""))
	require.NoError(t, rl.AllowPublisher(""))
	require.NoError(t, rl.AllowPublisher(""))
}

func TestRateLimit_perPublisher_blocksAboveCap(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerPublisher: 2})
	require.NoError(t, rl.AllowPublisher("pub-1"))
	require.NoError(t, rl.AllowPublisher("pub-1"))

	err := rl.AllowPublisher("pub-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimitedPerPublisher))
}

func TestRateLimit_perPublisher_independentBuckets(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{QPSPerPublisher: 1})
	require.NoError(t, rl.AllowPublisher("pub-1"))
	require.NoError(t, rl.AllowPublisher("pub-2"))
}

func TestRateLimit_circuitBreaker_opensAfterFailures(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{
		QPSPerAgent:             1000,
		CircuitFailureThreshold: 3,
	})

	// Drive 3 failures to trip the breaker.
	for i := 0; i < 3; i++ {
		rl.RecordFailure("flap.example.com", fmt.Errorf("boom"))
	}

	// Subsequent AllowAgent returns ErrCircuitOpen.
	err := rl.AllowAgent("flap.example.com")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCircuitOpen))
}

func TestRateLimit_circuitBreaker_independentPerAgent(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{
		QPSPerAgent:             1000,
		CircuitFailureThreshold: 2,
	})

	// Trip breaker on agent-A.
	for i := 0; i < 2; i++ {
		rl.RecordFailure("a", fmt.Errorf("boom"))
	}

	// agent-A breaker open.
	require.Error(t, rl.AllowAgent("a"))

	// agent-B unaffected.
	require.NoError(t, rl.AllowAgent("b"))
}

func TestRateLimit_recordSuccess_doesNotPanic(t *testing.T) {
	rl := NewRateLimiter(ServerConfig{})
	rl.RecordSuccess("a")
	rl.RecordSuccess("a")
}
