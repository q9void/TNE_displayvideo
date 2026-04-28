package agentic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schemaRel = "assets/agents.schema.json"

func TestLoadRegistry_emptyAllowList(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("assets", "agents.json"), schemaRel)
	require.NoError(t, err)
	assert.Equal(t, "9131", reg.SellerID())
	assert.Equal(t, 0, reg.AgentCount())
	assert.Nil(t, reg.AgentsForLifecycle(LifecyclePublisherBidRequest))
}

func TestLoadRegistry_oneAgent(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("testdata", "agents-one.json"), schemaRel)
	require.NoError(t, err)
	assert.Equal(t, 1, reg.AgentCount())

	got := reg.AgentsForLifecycle(LifecyclePublisherBidRequest)
	require.Len(t, got, 1)
	assert.Equal(t, "seg.example.com", got[0].ID)
	assert.Equal(t, "segmentation", got[0].Role)
	assert.Equal(t, int32(100), got[0].Priority)
	assert.True(t, got[0].HasIntent("ACTIVATE_SEGMENTS"))
	assert.False(t, got[0].HasIntent("BID_SHADE"))

	tx, ok := got[0].PrimaryTransport()
	require.True(t, ok)
	assert.Equal(t, "grpc", tx.Transport)
	assert.Equal(t, "127.0.0.1:50051", tx.URL)
	assert.Equal(t, "api_key_header", tx.Auth)
}

func TestLoadRegistry_filtersByLifecycle(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("testdata", "agents-two.json"), schemaRel)
	require.NoError(t, err)
	assert.Equal(t, 2, reg.AgentCount())

	preBid := reg.AgentsForLifecycle(LifecyclePublisherBidRequest)
	require.Len(t, preBid, 1)
	assert.Equal(t, "seg.example.com", preBid[0].ID)

	postBid := reg.AgentsForLifecycle(LifecycleDSPBidResponse)
	require.Len(t, postBid, 1)
	assert.Equal(t, "shade.example.com", postBid[0].ID)
}

func TestLoadRegistry_sortsByPriorityAsc(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("testdata", "agents-two.json"), schemaRel)
	require.NoError(t, err)
	all := reg.AllAgents()
	require.Len(t, all, 2)
	// Lower priority first; seg=100, shade=200.
	assert.Equal(t, "seg.example.com", all[0].ID)
	assert.Equal(t, "shade.example.com", all[1].ID)
}

func TestLoadRegistry_AgentByID(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("testdata", "agents-one.json"), schemaRel)
	require.NoError(t, err)

	got, ok := reg.AgentByID("seg.example.com")
	require.True(t, ok)
	assert.Equal(t, "Example Segmentation Agent", got.Name)

	_, ok = reg.AgentByID("does.not.exist")
	assert.False(t, ok)
}

func TestLoadRegistry_schemaViolation(t *testing.T) {
	bad := []byte(`{"version": "1.0", "agents": [{"id": "x"}]}`)
	_, err := LoadRegistry_inMemory_test(t, bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation")
}

func TestLoadRegistry_invalidJSON(t *testing.T) {
	bad := []byte(`{ not valid json `)
	_, err := LoadRegistry_inMemory_test(t, bad)
	require.Error(t, err)
}

func TestLoadRegistry_DocumentBytes_unchanged(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join("testdata", "agents-one.json"), schemaRel)
	require.NoError(t, err)
	body := reg.DocumentBytes()
	require.NotEmpty(t, body)
	// The serving handler must return verbatim content. A simple stability
	// check: the bytes contain the agent ID we put on disk.
	assert.Contains(t, string(body), `"seg.example.com"`)
}

// LoadRegistry_inMemory_test is a test helper that writes a doc to a temp
// file and runs LoadRegistry against it, exercising the same schema path.
func LoadRegistry_inMemory_test(t *testing.T, body []byte) (*Registry, error) {
	t.Helper()
	dir := t.TempDir()
	docPath := filepath.Join(dir, "agents.json")
	require.NoError(t, writeFile(docPath, body))
	return LoadRegistry(docPath, schemaRel)
}

func writeFile(path string, body []byte) error {
	return os.WriteFile(path, body, 0o644)
}
