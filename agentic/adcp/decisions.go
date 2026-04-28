package adcp

import "time"

// CallStat is the per-agent-per-auction summary recorded after a Dispatch
// call. One line per stat is emitted to structured logs as
// `evt: adcp.call`.
type CallStat struct {
	AgentID       string    `json:"agent_id"`
	Capability    string    `json:"capability"`
	Lifecycle     Lifecycle `json:"-"`
	LifecycleName string    `json:"lifecycle"`
	Status        string    `json:"status"` // "ok" | "timeout" | "circuit_open" | "not_implemented" | "error"
	LatencyMs     int64     `json:"latency_ms"`
	ResultCount   int       `json:"result_count,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// Signal is a single AdCP signal/segment returned by a get_signals call.
// Phase 1 carries only the minimum fields needed to enrich the bid
// request; the full AdCP signal schema (taxonomy, audience size, pricing
// tier) lands in Phase 2.
type Signal struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	TaxonomyID string `json:"taxonomy_id,omitempty"`
	AgentID    string `json:"-"` // attribution; never sent on the wire
}

// DispatchResult is what the Client returns from a Dispatch call. The
// Signals slice is the merged, deterministically-ordered output across
// all agents that responded within tmax. CallStats is one entry per
// dialed agent (success or failure). Truncated indicates at least one
// agent missed tmax.
type DispatchResult struct {
	Signals      []Signal
	CallStats    []CallStat
	Truncated    bool
	DispatchedAt time.Time
}

// AgentIDs returns a copy of the IDs of agents that returned at least
// one result. Used to populate ext.adcp.agentsCalled on outbound bid
// requests.
func (d DispatchResult) AgentIDs() []string {
	if len(d.CallStats) == 0 {
		return nil
	}
	out := make([]string, 0, len(d.CallStats))
	for _, s := range d.CallStats {
		if s.Status == "ok" && s.ResultCount > 0 {
			out = append(out, s.AgentID)
		}
	}
	return out
}
