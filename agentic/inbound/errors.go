package inbound

import "errors"

// Sentinel errors for the inbound path. Compare with errors.Is.
// Handlers map these to gRPC status codes; see rtb_handler.go.
var (
	// ErrAuthFailed is the generic auth-failure marker. Specific causes
	// below; this one is returned to callers (gRPC Unauthenticated) so we
	// don't leak which check failed.
	ErrAuthFailed = errors.New("inbound: authentication failed")

	// ErrAuthFailedRegistry indicates the caller's agent_id is not in the
	// last-known-good IAB Tools Portal Registry refresh.
	ErrAuthFailedRegistry = errors.New("inbound: agent not in registry")

	// ErrAuthFailedSPKI indicates the caller's cert SPKI fingerprint did
	// not match the per-buyer pinned value.
	ErrAuthFailedSPKI = errors.New("inbound: SPKI fingerprint mismatch")

	// ErrAuthFailedDealset indicates a mutation referenced a deal not in
	// the caller's authorised deal set (R5.1.10).
	ErrAuthFailedDealset = errors.New("inbound: deal not in caller's authorised set")

	// ErrRateLimitedPerAgent indicates the caller exceeded the per-agent
	// QPS soft cap.
	ErrRateLimitedPerAgent = errors.New("inbound: per-agent QPS exceeded")

	// ErrRateLimitedPerPublisher indicates aggregate inbound mutations
	// affecting a single publisher exceeded the per-publisher cap.
	ErrRateLimitedPerPublisher = errors.New("inbound: per-publisher QPS exceeded")

	// ErrCircuitOpen indicates the per-agent circuit breaker is open and
	// the call was rejected without business logic.
	ErrCircuitOpen = errors.New("inbound: circuit breaker open")

	// ErrOriginatorRejected indicates the caller's RTBRequest.Originator
	// was not TYPE_DSP or TYPE_PUBLISHER. Only we emit TYPE_SSP, and only
	// outbound (R5.1.4).
	ErrOriginatorRejected = errors.New("inbound: only TYPE_DSP/TYPE_PUBLISHER accepted")

	// ErrServerNotReady indicates a call arrived before Server.Start
	// finished, or after Server.Stop was invoked.
	ErrServerNotReady = errors.New("inbound: server not ready")
)
