package inbound

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfig_defaults_fillsZeroValues(t *testing.T) {
	cfg := ServerConfig{Enabled: true, AllowDevNoMTLS: true}.defaults()
	assert.Equal(t, 50051, cfg.GRPCPort)
	assert.Equal(t, 1000, cfg.QPSPerAgent)
	assert.Equal(t, 200, cfg.QPSPerPublisher)
	assert.Equal(t, 4*1024*1024, cfg.MaxRecvMsgBytes)
	assert.Equal(t, 60*time.Second, cfg.IdempotencyWindow)
	assert.Equal(t, 50, cfg.CircuitFailureThreshold)
	assert.Equal(t, 30*time.Second, cfg.CircuitTimeout)
}

func TestServerConfig_defaults_preservesOverrides(t *testing.T) {
	cfg := ServerConfig{
		Enabled:                 true,
		AllowDevNoMTLS:          true,
		GRPCPort:                51052,
		QPSPerAgent:             500,
		IdempotencyWindow:       30 * time.Second,
		CircuitFailureThreshold: 10,
	}.defaults()
	assert.Equal(t, 51052, cfg.GRPCPort)
	assert.Equal(t, 500, cfg.QPSPerAgent)
	assert.Equal(t, 30*time.Second, cfg.IdempotencyWindow)
	assert.Equal(t, 10, cfg.CircuitFailureThreshold)
}

func TestServerConfig_Validate_disabledIsAlwaysValid(t *testing.T) {
	cfg := ServerConfig{Enabled: false}
	assert.NoError(t, cfg.Validate())
}

func TestServerConfig_Validate_enabledRequiresMTLSPaths(t *testing.T) {
	cfg := ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: false,
		GRPCPort:       50051,
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MTLSCAPath")
}

func TestServerConfig_Validate_enabledDevSkipsMTLS(t *testing.T) {
	cfg := ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: true,
		GRPCPort:       50051,
	}
	assert.NoError(t, cfg.Validate())
}

func TestServerConfig_Validate_rejectsBadPort(t *testing.T) {
	cfg := ServerConfig{
		Enabled:        true,
		AllowDevNoMTLS: true,
		GRPCPort:       0,
	}.defaults() // defaults will set 50051; explicitly test the underflow path
	cfg.GRPCPort = -1
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GRPCPort")
}
