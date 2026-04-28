package agentic

import (
	"encoding/json"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// Envelope is the structured shape we read/write into BidRequest.ext.aamp.
// JSON tags match the wire shape documented in PRD §7.3.
type Envelope struct {
	Version           string             `json:"version"`
	Originator        *EnvelopeOrig      `json:"originator,omitempty"`
	Lifecycle         string             `json:"lifecycle,omitempty"`
	AgentConsent      bool               `json:"agentConsent"`
	AgentsCalled      []string           `json:"agentsCalled,omitempty"`
	MutationsApplied  int                `json:"mutationsApplied,omitempty"`
	PublisherEnvelope *PublisherEnvelope `json:"publisherEnvelope,omitempty"`

	// Fields below originate from the page-side prebid adapter and flow IN.
	IntentHints     []string         `json:"intentHints,omitempty"`
	DisclosedAgents []string         `json:"disclosedAgents,omitempty"`
	PageContext     *json.RawMessage `json:"pageContext,omitempty"`
	Disabled        bool             `json:"disabled,omitempty"`
}

// EnvelopeOrig is the {type, id} pair carried in ext.aamp.originator. It
// mirrors the proto Originator but stays JSON-shaped for OpenRTB ext.
type EnvelopeOrig struct {
	Type string `json:"type"` // "PUBLISHER" | "SSP" | "EXCHANGE" | "DSP"
	ID   string `json:"id"`
}

// PublisherEnvelope is the page-side originator block we forward to bidders
// so DSPs can see the original Originator chain (PRD §7.3).
type PublisherEnvelope struct {
	Originator  *EnvelopeOrig `json:"originator,omitempty"`
	IntentHints []string      `json:"intentHints,omitempty"`
}

// Envelope size caps mirror the prebid-side caps so the same payload that
// the adapter accepts is the same the server will accept inbound and the
// same the server will write outbound.
const (
	envelopeSoftCapBytes = 4 * 1024 // pageContext dropped above this
	envelopeHardCapBytes = 8 * 1024 // entire ext.aamp dropped above this
)

// extWrap is the marshalling helper for round-tripping an existing ext blob
// while only modifying the `aamp` key. Existing keys are preserved verbatim.
type extWrap struct {
	AAMP json.RawMessage `json:"aamp,omitempty"`
	// All other fields are preserved as-is via the rawOther map.
	rawOther map[string]json.RawMessage
}

// readExt unmarshals an existing BidRequest.Ext into a key-keyed map plus
// the AAMP slot. Empty / nil ext returns an empty wrap.
func readExt(raw json.RawMessage) (*extWrap, error) {
	w := &extWrap{rawOther: map[string]json.RawMessage{}}
	if len(raw) == 0 {
		return w, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for k, v := range m {
		if k == "aamp" {
			w.AAMP = v
			continue
		}
		w.rawOther[k] = v
	}
	return w, nil
}

// writeExt re-marshals the wrap back into a json.RawMessage with `aamp`
// either present or absent.
func writeExt(w *extWrap) (json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for k, v := range w.rawOther {
		out[k] = v
	}
	if len(w.AAMP) > 0 {
		out["aamp"] = w.AAMP
	}
	if len(out) == 0 {
		return nil, nil
	}
	return json.Marshal(out)
}

// WriteOutboundEnvelope writes the full ext.aamp envelope onto a BidRequest
// destined for bidders. AgentsCalled is populated; this envelope is NOT
// sent to extension-point agents (use WriteAgentEnvelope for that).
//
// Soft cap (4 KB): drops PageContext first, then IntentHints. Hard cap
// (8 KB): drops the entire aamp block, returning the unmodified ext (PRD
// R6.2.3 mirror). Returns nil error on size-cap drops — they are not
// errors, they are documented behaviour.
func WriteOutboundEnvelope(req *openrtb.BidRequest, sellerID string, lc Lifecycle, consent bool, agentsCalled []string, mutationsApplied int) error {
	if req == nil {
		return nil
	}
	env := Envelope{
		Version:          "1.0",
		Originator:       &EnvelopeOrig{Type: "SSP", ID: sellerID},
		Lifecycle:        lc.String(),
		AgentConsent:     consent,
		AgentsCalled:     agentsCalled,
		MutationsApplied: mutationsApplied,
	}
	return setEnvelopeWithSizeCaps(req, env)
}

// WriteAgentEnvelope writes the minimal ext.aamp envelope onto a BidRequest
// destined for an extension-point agent. AgentsCalled is omitted on
// purpose (PRD §5.4 confidentiality — agents must not see who else has been
// dialled this auction).
func WriteAgentEnvelope(req *openrtb.BidRequest, sellerID string, lc Lifecycle, consent bool) error {
	if req == nil {
		return nil
	}
	env := Envelope{
		Version:      "1.0",
		Originator:   &EnvelopeOrig{Type: "SSP", ID: sellerID},
		Lifecycle:    lc.String(),
		AgentConsent: consent,
	}
	return setEnvelopeWithSizeCaps(req, env)
}

func setEnvelopeWithSizeCaps(req *openrtb.BidRequest, env Envelope) error {
	wrap, err := readExt(req.Ext)
	if err != nil {
		// Don't fail the auction on a malformed existing ext — leave it alone
		// and skip writing aamp.
		return err
	}

	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Hard cap — drop the block entirely.
	if len(body) > envelopeHardCapBytes {
		// Re-marshal without aamp.
		wrap.AAMP = nil
		out, err := writeExt(wrap)
		if err != nil {
			return err
		}
		req.Ext = out
		return nil
	}
	// Soft cap — drop pageContext first, then intentHints. (Outbound envelopes
	// don't carry pageContext, so this is mostly a no-op for the server-side
	// path; the cap still applies symmetrically with the prebid adapter.)
	if len(body) > envelopeSoftCapBytes {
		env.PageContext = nil
		body, err = json.Marshal(env)
		if err != nil {
			return err
		}
		if len(body) > envelopeSoftCapBytes {
			env.IntentHints = nil
			body, err = json.Marshal(env)
			if err != nil {
				return err
			}
		}
	}
	wrap.AAMP = body
	out, err := writeExt(wrap)
	if err != nil {
		return err
	}
	req.Ext = out
	return nil
}

// ReadInboundEnvelope reads any existing ext.aamp set by the prebid adapter
// (or a previous middleware) and returns it. Returns nil for absent/empty.
func ReadInboundEnvelope(req *openrtb.BidRequest) (*Envelope, error) {
	if req == nil || len(req.Ext) == 0 {
		return nil, nil
	}
	wrap, err := readExt(req.Ext)
	if err != nil {
		return nil, err
	}
	if len(wrap.AAMP) == 0 {
		return nil, nil
	}
	var env Envelope
	if err := json.Unmarshal(wrap.AAMP, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// AgentApplied is the per-agent attribution record we write back onto each
// bid's ext.aamp.agentsApplied[] (PRD §7.3 response shape).
type AgentApplied struct {
	AgentID       string   `json:"agent_id"`
	Intents       []string `json:"intents"`
	MutationCount int      `json:"mutation_count"`
}

// BidExtAAMP is the per-bid attribution we set on bid.ext.aamp.
type BidExtAAMP struct {
	AgentsApplied     []AgentApplied `json:"agentsApplied,omitempty"`
	BidShadingDelta   *float64       `json:"bidShadingDelta,omitempty"`
	SegmentsActivated int            `json:"segmentsActivated,omitempty"`
}

// WriteBidExt writes attribution onto a single openrtb.Bid. Mirrors the
// ext-merge logic of setEnvelopeWithSizeCaps but skips the size caps —
// per-bid blocks are tiny by construction.
func WriteBidExt(bid *openrtb.Bid, applied []AgentApplied, shadingDelta *float64, segmentsActivated int) error {
	if bid == nil || (len(applied) == 0 && shadingDelta == nil && segmentsActivated == 0) {
		return nil
	}
	wrap, err := readExt(bid.Ext)
	if err != nil {
		return err
	}
	payload := BidExtAAMP{
		AgentsApplied:     applied,
		BidShadingDelta:   shadingDelta,
		SegmentsActivated: segmentsActivated,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	wrap.AAMP = body
	out, err := writeExt(wrap)
	if err != nil {
		return err
	}
	bid.Ext = out
	return nil
}
