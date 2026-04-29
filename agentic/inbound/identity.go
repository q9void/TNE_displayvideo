package inbound

// AgentIdentity is the auth-resolved identity of a caller of the inbound
// AAMP services. Populated by Authenticator.Verify and propagated through
// the request context for use in handlers.
//
// Per PRD §5.3:
//   - AgentID is the caller's agent_id (DNS-name-shaped; matches a row in
//     our trusted-buyer table and an entry in the IAB Tools Portal Registry)
//   - AgentType is the AAMP Originator type — must be "DSP" or "PUBLISHER";
//     "SSP" is rejected (only we emit that)
//   - AuthorizedDeals is the set of deal IDs this caller may reference in
//     ACTIVATE_DEALS / SUPPRESS_DEALS / ADJUST_DEAL_FLOOR mutations.
//     Phase 2A ships with empty slices; Phase 2B populates from deal/Store.
//   - SPKIFingerprint is the caller cert's Subject Public Key Info hash;
//     captured per call for forensics. Empty in DevAuthenticator mode.
//   - RegistryVerified is true when the caller's agent_id was found in the
//     last successful IAB Tools Portal Registry refresh (R5.3.2).
//     DevAuthenticator sets this true unconditionally.
type AgentIdentity struct {
	AgentID          string
	AgentType        string
	AuthorizedDeals  []string
	SPKIFingerprint  string
	RegistryVerified bool
}

// IsAuthorizedForDeal returns true iff dealID is in the caller's
// AuthorizedDeals set. Used by the RTB handler before applying any
// deal-touching mutation (R5.1.10).
func (a AgentIdentity) IsAuthorizedForDeal(dealID string) bool {
	if a.AuthorizedDeals == nil {
		return false
	}
	for _, d := range a.AuthorizedDeals {
		if d == dealID {
			return true
		}
	}
	return false
}
