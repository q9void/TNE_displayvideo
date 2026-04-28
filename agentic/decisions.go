package agentic

import (
	"time"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// ApplyDecision is the audit record for a single mutation that the Applier
// considered (whether applied, rejected, or superseded). One line per
// decision is emitted to structured logs per PRD §5.8.
type ApplyDecision struct {
	AgentID      string `json:"agent_id"`
	Intent       string `json:"intent"`
	Op           string `json:"op"`
	Path         string `json:"path"`
	Decision     string `json:"decision"` // "applied" | "rejected" | "superseded"
	Reason       string `json:"reason,omitempty"`
	SupersededBy string `json:"superseded_by,omitempty"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
}

// AgentCallStat is the per-agent-per-auction summary recorded after a
// Dispatch call. One line per stat is emitted to structured logs as
// `evt: agentic.call`.
type AgentCallStat struct {
	AgentID       string    `json:"agent_id"`
	Lifecycle     Lifecycle `json:"-"`
	LifecycleName string    `json:"lifecycle"`
	Status        string    `json:"status"` // "ok" | "timeout" | "circuit_open" | "error"
	LatencyMs     int64     `json:"latency_ms"`
	MutationCount int       `json:"mutation_count"`
	ModelVersion  string    `json:"model_version,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// MutationOrigin links each returned Mutation back to the agent that
// produced it. The Client builds this map in Dispatch and passes it to
// the Applier so audit logs can attribute decisions correctly.
type MutationOrigin struct {
	AgentID  string
	Priority int32
}

// DispatchResult is what the Client returns from a Dispatch call. The
// Mutations slice is the merged, deterministically-ordered output across
// all agents that responded within tmax. AgentStats is one entry per
// dialed agent (success or failure). Truncated indicates at least one
// agent missed tmax.
type DispatchResult struct {
	Mutations    []*pb.Mutation
	Origins      map[*pb.Mutation]MutationOrigin
	AgentStats   []AgentCallStat
	Truncated    bool
	DispatchedAt time.Time
}

// AgentIDs returns a copy of the IDs of agents that returned at least one
// mutation. Used to populate ext.aamp.agentsCalled on outbound bid requests.
func (d DispatchResult) AgentIDs() []string {
	if len(d.AgentStats) == 0 {
		return nil
	}
	out := make([]string, 0, len(d.AgentStats))
	for _, s := range d.AgentStats {
		if s.Status == "ok" && s.MutationCount > 0 {
			out = append(out, s.AgentID)
		}
	}
	return out
}
