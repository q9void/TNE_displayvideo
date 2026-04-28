package endpoints

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/agentic"
)

// AgentsAdminHandler serves the /admin/agents and /admin/agents/{id} routes.
// The caller is responsible for wrapping with adminAuth middleware before
// registering on the mux.
type AgentsAdminHandler struct {
	reg *agentic.Registry
}

// NewAgentsAdminHandler builds the read-only admin handler.
func NewAgentsAdminHandler(reg *agentic.Registry) *AgentsAdminHandler {
	return &AgentsAdminHandler{reg: reg}
}

// adminAgentsList is the JSON shape returned at /admin/agents.
type adminAgentsList struct {
	Agents []agentic.AgentEndpoint `json:"agents"`
}

// adminAgentsGet is the JSON shape returned at /admin/agents/{id}.
type adminAgentsGet struct {
	Agent agentic.AgentEndpoint `json:"agent"`
}

func (h *AgentsAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.reg == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Two cases: "/admin/agents" → list, "/admin/agents/<id>" → single.
	id := strings.TrimPrefix(r.URL.Path, "/admin/agents")
	id = strings.TrimPrefix(id, "/")
	if id == "" {
		_ = json.NewEncoder(w).Encode(adminAgentsList{Agents: h.reg.AllAgents()})
		return
	}
	agent, ok := h.reg.AgentByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = json.NewEncoder(w).Encode(adminAgentsGet{Agent: agent})
}
