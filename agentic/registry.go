package agentic

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/xeipuuv/gojsonschema"
)

// AgentEndpoint is the parsed, in-memory representation of a single agent
// declaration from agents.json. The original JSON stays available via
// Registry.DocumentBytes() for HTTP serving so we never re-marshal.
type AgentEndpoint struct {
	ID             string           `json:"id"`
	Name           string           `json:"name,omitempty"`
	Role           string           `json:"role"`
	Vendor         string           `json:"vendor,omitempty"`
	RegistryRef    string           `json:"registry_ref,omitempty"`
	Priority       int32            `json:"priority,omitempty"`
	TmaxMs         int32            `json:"tmax_ms,omitempty"`
	Essential      bool             `json:"essential,omitempty"`
	Endpoints      []AgentTransport `json:"endpoints"`
	Lifecycles     []string         `json:"lifecycles"`
	Intents        []string         `json:"intents"`
	Requires       AgentRequires    `json:"requires,omitempty"`
	DataProcessing AgentDataProcess `json:"data_processing,omitempty"`
}

// AgentTransport describes how to reach the agent. Phase 1 uses grpc / grpcs.
type AgentTransport struct {
	Transport string `json:"transport"`
	URL       string `json:"url"`
	Auth      string `json:"auth"`
}

// AgentRequires gates pre-call eligibility based on consent + signals.
type AgentRequires struct {
	TCFPurposes []int `json:"tcf_purposes,omitempty"`
	PreciseGeo  bool  `json:"precise_geo,omitempty"`
}

// AgentDataProcess is pure disclosure metadata served via /.well-known/agents.json.
// We do not act on it; it is informational for publishers and for the IAB
// Tools Portal once we register.
type AgentDataProcess struct {
	Categories    []string `json:"categories,omitempty"`
	RetentionDays int      `json:"retention_days,omitempty"`
}

// HasLifecycle returns true iff the agent declares it handles lc.
func (a AgentEndpoint) HasLifecycle(lc Lifecycle) bool {
	want := lc.String()
	for _, l := range a.Lifecycles {
		if l == want {
			return true
		}
	}
	return false
}

// HasIntent returns true iff the agent declares it can produce intent.
func (a AgentEndpoint) HasIntent(intent string) bool {
	for _, i := range a.Intents {
		if i == intent {
			return true
		}
	}
	return false
}

// PrimaryTransport returns the first endpoint declared. Phase 1 only dials one.
func (a AgentEndpoint) PrimaryTransport() (AgentTransport, bool) {
	if len(a.Endpoints) == 0 {
		return AgentTransport{}, false
	}
	return a.Endpoints[0], true
}

// Registry is the in-process, immutable view of agents.json. Constructed
// once at boot via LoadRegistry; safe for concurrent reads thereafter.
//
// Phase 2 will add hot-reload via SIGHUP, which re-introduces a sync.RWMutex
// on this struct. We deliberately do not pre-allocate the mutex now to keep
// lint clean.
type Registry struct {
	doc       []byte
	parsed    registryDoc
	agents    []AgentEndpoint // sorted by Priority asc (deterministic last-writer-wins)
	byLifecyc map[Lifecycle][]AgentEndpoint
}

type registryDoc struct {
	Schema       string          `json:"$schema,omitempty"`
	Version      string          `json:"version"`
	SellerID     string          `json:"seller_id"`
	SellerDomain string          `json:"seller_domain"`
	Contact      string          `json:"contact,omitempty"`
	UpdatedAt    string          `json:"updated_at,omitempty"`
	Agents       []AgentEndpoint `json:"agents"`
}

// LoadRegistry reads agents.json and agents.schema.json from disk, validates
// the document against the schema, parses agents into typed slices, and
// returns an immutable Registry.
//
// On schema validation failure, the document is rejected wholesale — partial
// load is not supported. On invalid JSON, ditto. The Phase 1 default
// agents.json with `agents: []` validates and produces an empty registry.
func LoadRegistry(docPath, schemaPath string) (*Registry, error) {
	// docPath and schemaPath come from server config (env-var-driven, validated
	// at boot in cmd/server/config.go::Validate). Both are read-only loads of
	// JSON documents, never executed. gosec G304 is a known false positive
	// when the variable source is trusted; suppress with a doc-anchored
	// annotation rather than refactoring.
	doc, err := os.ReadFile(docPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("agentic: read agents.json %q: %w", docPath, err)
	}
	schemaBytes, err := os.ReadFile(schemaPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("agentic: read schema %q: %w", schemaPath, err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	docLoader := gojsonschema.NewBytesLoader(doc)
	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("agentic: schema validation %q: %w", docPath, err)
	}
	if !result.Valid() {
		var msg string
		for _, e := range result.Errors() {
			msg += "\n  - " + e.String()
		}
		return nil, fmt.Errorf("agentic: agents.json failed schema validation:%s", msg)
	}

	return loadRegistryFromBytes(doc)
}

// LoadRegistryFromBytes is exposed for tests that want to build a Registry
// in-memory from a byte slice (skipping the schema validation we already
// exercise in TestRegistry_schema).
func LoadRegistryFromBytes(doc []byte) (*Registry, error) {
	return loadRegistryFromBytes(doc)
}

func loadRegistryFromBytes(doc []byte) (*Registry, error) {
	var parsed registryDoc
	if err := json.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("agentic: parse agents.json: %w", err)
	}

	agents := make([]AgentEndpoint, len(parsed.Agents))
	copy(agents, parsed.Agents)

	// Stable sort by Priority asc; tiebreak by ID for determinism (R5.5.7/R5.5.8).
	sort.SliceStable(agents, func(i, j int) bool {
		if agents[i].Priority != agents[j].Priority {
			return agents[i].Priority < agents[j].Priority
		}
		return agents[i].ID < agents[j].ID
	})

	byLC := map[Lifecycle][]AgentEndpoint{
		LifecyclePublisherBidRequest: nil,
		LifecycleDSPBidResponse:      nil,
	}
	for _, a := range agents {
		for _, lc := range []Lifecycle{LifecyclePublisherBidRequest, LifecycleDSPBidResponse} {
			if a.HasLifecycle(lc) {
				byLC[lc] = append(byLC[lc], a)
			}
		}
	}

	return &Registry{
		doc:       doc,
		parsed:    parsed,
		agents:    agents,
		byLifecyc: byLC,
	}, nil
}

// DocumentBytes returns the raw agents.json bytes for serving at
// /.well-known/agents.json. Callers must not mutate the returned slice.
func (r *Registry) DocumentBytes() []byte {
	return r.doc
}

// SellerID returns the configured seller_id field. Used to cross-check
// AGENTIC_SELLER_ID env at boot.
func (r *Registry) SellerID() string {
	return r.parsed.SellerID
}

// AgentCount returns the number of agents in the registry.
func (r *Registry) AgentCount() int {
	return len(r.agents)
}

// AgentsForLifecycle returns the slice of agents that declare lc, in
// priority-ascending order. The returned slice is a shared internal slice;
// callers must not mutate it. Returns nil for an empty match.
func (r *Registry) AgentsForLifecycle(lc Lifecycle) []AgentEndpoint {
	return r.byLifecyc[lc]
}

// AllAgents returns the full agent slice in priority order (test helper).
func (r *Registry) AllAgents() []AgentEndpoint {
	return r.agents
}

// AgentByID returns the agent with the given id, or false if absent.
// Used by the admin endpoint to project a single record.
func (r *Registry) AgentByID(id string) (AgentEndpoint, bool) {
	for _, a := range r.agents {
		if a.ID == id {
			return a, true
		}
	}
	return AgentEndpoint{}, false
}
