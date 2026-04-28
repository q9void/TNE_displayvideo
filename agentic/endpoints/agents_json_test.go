package endpoints_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenexusengine/tne_springwire/agentic"
	"github.com/thenexusengine/tne_springwire/agentic/endpoints"
)

func loadTestRegistry(t *testing.T, body string) *agentic.Registry {
	t.Helper()
	reg, err := agentic.LoadRegistryFromBytes([]byte(body))
	require.NoError(t, err)
	return reg
}

const oneAgentDoc = `{
	"$schema": "https://thenexusengine.com/schemas/agents.v1.json",
	"version": "1.0",
	"seller_id": "9131",
	"seller_domain": "thenexusengine.com",
	"agents": [
		{
			"id": "seg.example.com",
			"name": "Example Segmentation Agent",
			"role": "segmentation",
			"endpoints": [{"transport": "grpc", "url": "127.0.0.1:50051", "auth": "api_key_header"}],
			"lifecycles": ["PUBLISHER_BID_REQUEST"],
			"intents": ["ACTIVATE_SEGMENTS"]
		}
	]
}`

// ──────────────────────────────────────────────────────────────────────────
// /agents.json (public)
// ──────────────────────────────────────────────────────────────────────────

func TestAgentsJSON_disabledReturns404(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsJSONHandler(reg, false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/agents.json", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAgentsJSON_enabledServesDoc(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsJSONHandler(reg, true)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/agents.json", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
	// Body is the verbatim doc.
	assert.Equal(t, string(reg.DocumentBytes()), rec.Body.String())
}

func TestAgentsJSON_handlesOptions(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsJSONHandler(reg, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/.well-known/agents.json", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAgentsJSON_rejectsPost(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsJSONHandler(reg, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/.well-known/agents.json", strings.NewReader("{}")))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// ──────────────────────────────────────────────────────────────────────────
// /admin/agents
// ──────────────────────────────────────────────────────────────────────────

func TestAgentsAdmin_listReturnsAll(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsAdminHandler(reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/agents", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Agents []agentic.AgentEndpoint `json:"agents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Agents, 1)
	assert.Equal(t, "seg.example.com", body.Agents[0].ID)
}

func TestAgentsAdmin_getByID(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsAdminHandler(reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/agents/seg.example.com", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Agent agentic.AgentEndpoint `json:"agent"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "seg.example.com", body.Agent.ID)
}

func TestAgentsAdmin_unknownIDReturns404(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsAdminHandler(reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/agents/does-not-exist", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAgentsAdmin_rejectsPost(t *testing.T) {
	reg := loadTestRegistry(t, oneAgentDoc)
	h := endpoints.NewAgentsAdminHandler(reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/admin/agents", strings.NewReader("{}")))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
