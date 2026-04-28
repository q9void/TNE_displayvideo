package adcp

// EnvelopeKey is the JSON object key the AdCP envelope is written under.
// Symmetric with agentic/envelope.go's "aamp" key — a bid request that
// transits both integrations carries ext.aamp and ext.adcp side by side.
const EnvelopeKey = "adcp"

// Envelope is the ext.adcp object written to outbound bid requests.
// Structure tracks AdCP's "context" object on the bid stream side; we
// keep it small in Phase 1 and grow it as more capabilities ship.
type Envelope struct {
	// SellerID echoes our seller_id from adcp_agents.json. Bidders use it
	// to correlate the AdCP context with our schain identity.
	SellerID string `json:"seller_id,omitempty"`

	// AgentsCalled lists the agent IDs that actually produced results for
	// this auction (status="ok" && ResultCount>0). Empty when no agent
	// returned anything — *not* the full registry.
	AgentsCalled []string `json:"agents_called,omitempty"`

	// SignalsActivated counts how many signals were merged into the bid
	// request from get_signals responses. Phase 1 always 0 because the
	// client is a stub.
	SignalsActivated int `json:"signals_activated,omitempty"`

	// Disabled is true iff ext.adcp.disabled was set on the inbound
	// publisher request — the page-side opt-out from the prebid adapter.
	// Bidders downstream MUST treat any other AdCP fields as advisory
	// when this is true.
	Disabled bool `json:"disabled,omitempty"`
}

// Build assembles an Envelope from a DispatchResult plus the seller_id.
// Returns the zero Envelope when sellerID is empty — there is nothing
// useful to write without identity.
func Build(sellerID string, d DispatchResult) Envelope {
	if sellerID == "" {
		return Envelope{}
	}
	return Envelope{
		SellerID:         sellerID,
		AgentsCalled:     d.AgentIDs(),
		SignalsActivated: len(d.Signals),
	}
}
