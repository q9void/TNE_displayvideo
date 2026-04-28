package agentic

import "errors"

// Sentinel errors returned by the agentic package. Callers should compare
// with errors.Is. None of these are auction-fatal — every site that reads
// them is required to log + drop, never propagate to the auction caller.
var (
	// ErrTmax indicates a per-agent or aggregate timeout was hit. Mutations
	// from late agents are dropped.
	ErrTmax = errors.New("agentic: tmax exceeded")

	// ErrCircuitOpen indicates the per-agent circuit breaker is open and the
	// call was skipped without dialing.
	ErrCircuitOpen = errors.New("agentic: circuit breaker open")

	// ErrUnsupportedIntent indicates the mutation carries an Intent not in
	// our whitelist (PRD R5.5.1).
	ErrUnsupportedIntent = errors.New("agentic: unsupported intent")

	// ErrUnsupportedOp indicates Op is OPERATION_UNSPECIFIED (PRD R5.5.3).
	ErrUnsupportedOp = errors.New("agentic: unsupported operation")

	// ErrPathInvalid indicates the mutation path does not match the
	// per-intent allowed selector (PRD R5.5.4).
	ErrPathInvalid = errors.New("agentic: mutation path invalid")

	// ErrPayloadTooLarge indicates the mutation payload exceeded the
	// configured size cap (PRD R5.5.6).
	ErrPayloadTooLarge = errors.New("agentic: payload exceeds size cap")

	// ErrShadeOutOfBounds indicates a BID_SHADE mutation tried to raise the
	// price or shade below the configured minimum fraction (PRD R5.5.10).
	ErrShadeOutOfBounds = errors.New("agentic: shade out of bounds")

	// ErrShadeDisabled indicates BID_SHADE was disabled by config (OQ3 default).
	ErrShadeDisabled = errors.New("agentic: shade disabled by config")

	// ErrFloorClamped is informational, not a hard rejection — the floor was
	// clamped to the publisher floor (PRD R5.5.9). Mutations carrying this
	// are still applied (with the clamped value).
	ErrFloorClamped = errors.New("agentic: floor clamped to publisher floor")

	// ErrConsentWithheld indicates agent fanout is suppressed because consent
	// was withheld for this auction (PRD §5.9 soft-filter).
	ErrConsentWithheld = errors.New("agentic: consent withheld")

	// ErrCOPPABlocked indicates COPPA traffic — agent fanout is hard-blocked
	// (PRD §5.9 hard-block).
	ErrCOPPABlocked = errors.New("agentic: COPPA blocks agent fanout")

	// ErrMutationCapExceeded indicates the per-response mutation cap was hit;
	// remaining mutations are dropped (PRD R5.5.5).
	ErrMutationCapExceeded = errors.New("agentic: mutation cap exceeded")

	// ErrWrongLifecycle indicates a mutation arrived at the wrong auction
	// stage (PRD R5.5.2).
	ErrWrongLifecycle = errors.New("agentic: wrong lifecycle for intent")
)
