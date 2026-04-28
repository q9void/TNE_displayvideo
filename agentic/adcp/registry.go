package adcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/xeipuuv/gojsonschema"
)

// Agent is the parsed, in-memory representation of a single AdCP agent
// declaration from adcp_agents.json. The original JSON stays available
// via Registry.DocumentBytes() for HTTP serving so we never re-marshal.
type Agent struct {
	ID             string         `json:"id"`
	Name           string         `json:"name,omitempty"`
	Role           string         `json:"role"`
	Vendor         string         `json:"vendor,omitempty"`
	RegistryRef    string         `json:"registry_ref,omitempty"`
	Priority       int32          `json:"priority,omitempty"`
	TmaxMs         int32          `json:"tmax_ms,omitempty"`
	Essential      bool           `json:"essential,omitempty"`
	Endpoints      []Transport    `json:"endpoints"`
	Lifecycles     []string       `json:"lifecycles"`
	Capabilities   []string       `json:"capabilities"`
	Requires       Requires       `json:"requires,omitempty"`
	DataProcessing DataProcessing `json:"data_processing,omitempty"`
}

// Transport describes how to reach an AdCP agent. Phase 1 dials over
// HTTPS; the "mcp" transport is the AdCP-canonical form (MCP framing
// over HTTPS) and "http" is a raw JSON-RPC fallback.
type Transport struct {
	Transport string `json:"transport"`
	URL       string `json:"url"`
	Auth      string `json:"auth"`
}

// Requires gates pre-call eligibility based on consent + signals.
type Requires struct {
	TCFPurposes []int `json:"tcf_purposes,omitempty"`
	PreciseGeo  bool  `json:"precise_geo,omitempty"`
}

// DataProcessing is pure disclosure metadata served via /.well-known/adcp_agents.json
// in a future revision. We do not act on it; it is informational for
// publishers.
type DataProcessing struct {
	Categories    []string `json:"categories,omitempty"`
	RetentionDays int      `json:"retention_days,omitempty"`
}

// HasLifecycle returns true iff the agent declares it handles lc.
func (a Agent) HasLifecycle(lc Lifecycle) bool {
	want := lc.String()
	for _, l := range a.Lifecycles {
		if l == want {
			return true
		}
	}
	return false
}

// HasCapability returns true iff the agent declares it can serve cap.
func (a Agent) HasCapability(cap Capability) bool {
	for _, c := range a.Capabilities {
		if c == string(cap) {
			return true
		}
	}
	return false
}

// PrimaryTransport returns the first endpoint declared. Phase 1 only
// dials one.
func (a Agent) PrimaryTransport() (Transport, bool) {
	if len(a.Endpoints) == 0 {
		return Transport{}, false
	}
	return a.Endpoints[0], true
}

// Registry is the in-process, immutable view of adcp_agents.json.
// Constructed once at boot via LoadRegistry; safe for concurrent reads
// thereafter.
type Registry struct {
	doc       []byte
	parsed    registryDoc
	agents    []Agent
	byLifecyc map[Lifecycle][]Agent
}

type registryDoc struct {
	Schema       string  `json:"$schema,omitempty"`
	Version      string  `json:"version"`
	SellerID     string  `json:"seller_id"`
	SellerDomain string  `json:"seller_domain"`
	Contact      string  `json:"contact,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
	Agents       []Agent `json:"agents"`
}

// LoadRegistry reads adcp_agents.json and adcp_agents.schema.json from
// disk, validates the document against the schema, parses agents into
// typed slices, and returns an immutable Registry.
//
// On schema validation failure, the document is rejected wholesale —
// partial load is not supported. The Phase 1 default with `agents: []`
// validates and produces an empty registry.
func LoadRegistry(docPath, schemaPath string) (*Registry, error) {
	// docPath and schemaPath come from server config (env-var-driven,
	// validated at boot). Both are read-only loads of JSON documents,
	// never executed. gosec G304 is a known false positive when the
	// variable source is trusted.
	doc, err := os.ReadFile(docPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("adcp: read adcp_agents.json %q: %w", docPath, err)
	}
	schemaBytes, err := os.ReadFile(schemaPath) // #nosec G304 -- path comes from validated server config
	if err != nil {
		return nil, fmt.Errorf("adcp: read schema %q: %w", schemaPath, err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	docLoader := gojsonschema.NewBytesLoader(doc)
	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("adcp: schema validation %q: %w", docPath, err)
	}
	if !result.Valid() {
		var msg string
		for _, e := range result.Errors() {
			msg += "\n  - " + e.String()
		}
		return nil, fmt.Errorf("adcp: adcp_agents.json failed schema validation:%s", msg)
	}

	return loadRegistryFromBytes(doc)
}

// LoadRegistryFromBytes is exposed for tests that want to build a
// Registry in-memory from a byte slice (skipping schema validation).
func LoadRegistryFromBytes(doc []byte) (*Registry, error) {
	return loadRegistryFromBytes(doc)
}

func loadRegistryFromBytes(doc []byte) (*Registry, error) {
	var parsed registryDoc
	if err := json.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("adcp: parse adcp_agents.json: %w", err)
	}

	agents := make([]Agent, len(parsed.Agents))
	copy(agents, parsed.Agents)

	// Stable sort by Priority asc; tiebreak by ID for determinism.
	sort.SliceStable(agents, func(i, j int) bool {
		if agents[i].Priority != agents[j].Priority {
			return agents[i].Priority < agents[j].Priority
		}
		return agents[i].ID < agents[j].ID
	})

	byLC := map[Lifecycle][]Agent{
		LifecyclePreAuctionSignals:    nil,
		LifecyclePreAuctionProducts:   nil,
		LifecyclePostAuctionReporting: nil,
	}
	for _, a := range agents {
		for lc := range byLC {
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

// DocumentBytes returns the raw adcp_agents.json bytes. Callers must not
// mutate the returned slice.
func (r *Registry) DocumentBytes() []byte {
	return r.doc
}

// SellerID returns the configured seller_id field.
func (r *Registry) SellerID() string {
	return r.parsed.SellerID
}

// AgentCount returns the number of agents in the registry.
func (r *Registry) AgentCount() int {
	return len(r.agents)
}

// AgentsForLifecycle returns the slice of agents that declare lc, in
// priority-ascending order. The returned slice is shared internal state;
// callers must not mutate it. Returns nil for an empty match.
func (r *Registry) AgentsForLifecycle(lc Lifecycle) []Agent {
	return r.byLifecyc[lc]
}

// AllAgents returns the full agent slice in priority order.
func (r *Registry) AllAgents() []Agent {
	return r.agents
}

// AgentByID returns the agent with the given id, or false if absent.
func (r *Registry) AgentByID(id string) (Agent, bool) {
	for _, a := range r.agents {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}
