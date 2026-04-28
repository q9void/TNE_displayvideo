// Package endpoints exposes HTTP handlers for the agentic feature.
// Sub-package of agentic/ so the umbrella stays self-contained.
package endpoints

import (
	"net/http"

	"github.com/thenexusengine/tne_springwire/agentic"
)

// AgentsJSONHandler serves /.well-known/agents.json and /agents.json.
//
// When Enabled is false the handler returns 404 — we deliberately do NOT
// serve an empty document, so external scrapers do not register us as an
// agentic SSP just because the route exists (PRD §8.5).
type AgentsJSONHandler struct {
	reg     *agentic.Registry
	enabled bool
}

// NewAgentsJSONHandler builds a handler that serves the registry's raw
// document bytes verbatim.
func NewAgentsJSONHandler(reg *agentic.Registry, enabled bool) *AgentsJSONHandler {
	return &AgentsJSONHandler{reg: reg, enabled: enabled}
}

func (h *AgentsJSONHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.reg == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 1h cache — shorter than sellers.json since agent rosters change more often.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := w.Write(h.reg.DocumentBytes()); err != nil {
		// Client disconnect; nothing actionable.
		return
	}
}
