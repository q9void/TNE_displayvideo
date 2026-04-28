package adcp

import "errors"

// Sentinel errors returned by the adcp package. Callers should compare
// with errors.Is. None of these are auction-fatal — every site that reads
// them is required to log + drop, never propagate to the auction caller.
var (
	// ErrTmax indicates a per-agent or aggregate timeout was hit. Late
	// agent responses are dropped.
	ErrTmax = errors.New("adcp: tmax exceeded")

	// ErrCircuitOpen indicates the per-agent circuit breaker is open and
	// the call was skipped without dialing.
	ErrCircuitOpen = errors.New("adcp: circuit breaker open")

	// ErrNotImplemented indicates the requested capability has no
	// implementation in this build. Phase 1 returns this for every
	// outbound call so operators can stage-flip ADCP_ENABLED=true and
	// observe wiring without hitting a real agent.
	ErrNotImplemented = errors.New("adcp: capability not implemented in this build")

	// ErrUnsupportedCapability indicates the agent advertised a capability
	// the server does not know about. Logged and ignored.
	ErrUnsupportedCapability = errors.New("adcp: unsupported capability")

	// ErrUnsupportedTransport indicates the agent declared a transport the
	// server cannot dial (anything other than mcp/https in Phase 1).
	ErrUnsupportedTransport = errors.New("adcp: unsupported transport")

	// ErrInsecureTransport indicates the agent declared plain http:// and
	// ADCP_ALLOW_INSECURE is false. Always rejected in production.
	ErrInsecureTransport = errors.New("adcp: insecure transport refused")

	// ErrConsentWithheld indicates AdCP fanout is suppressed because
	// consent was withheld for this auction. Mirrors agentic.ErrConsentWithheld.
	ErrConsentWithheld = errors.New("adcp: consent withheld")

	// ErrCOPPABlocked indicates COPPA traffic — AdCP fanout hard-blocks.
	ErrCOPPABlocked = errors.New("adcp: COPPA blocks AdCP fanout")

	// ErrSignalsCapExceeded indicates the per-response signals cap was hit;
	// remaining signals are dropped.
	ErrSignalsCapExceeded = errors.New("adcp: signals cap exceeded")
)
