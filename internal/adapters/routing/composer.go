package routing

import (
	"encoding/json"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// Composer applies bidder_field_rules to a copy of the incoming BidRequest,
// producing the outgoing request for a specific SSP.
type Composer struct {
	rules []storage.BidderFieldRule
}

// NewComposer creates a Composer for the given pre-loaded rules.
func NewComposer(rules []storage.BidderFieldRule) *Composer {
	return &Composer{rules: rules}
}

// Apply returns a modified copy of req with all rules applied.
func (c *Composer) Apply(
	bidderCode string,
	req *openrtb.BidRequest,
	slotParams map[string]interface{},
	httpCtx *HTTPContext,
	accountParams map[string]interface{},
) (*openrtb.BidRequest, []error) {
	// Shallow-copy the request; deep-copy Imp to avoid mutating caller.
	out := *req
	if req.Imp != nil {
		out.Imp = make([]openrtb.Imp, len(req.Imp))
		copy(out.Imp, req.Imp)
	}

	// Build a mutable ext map for imp[0]
	impExt := make(map[string]interface{})
	if len(out.Imp) > 0 && out.Imp[0].Ext != nil {
		_ = json.Unmarshal(out.Imp[0].Ext, &impExt)
	}

	var errs []error

	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}
		val, err := ApplyRule(rule, req.User, slotParams, httpCtx, accountParams)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if val == nil {
			continue
		}

		if err := setField(&out, &impExt, rule.FieldPath, val); err != nil {
			logger.Log.Warn().Err(err).Str("field", rule.FieldPath).Msg("composer: setField failed")
		}
	}

	// Re-marshal impExt back to imp[0].Ext
	if len(out.Imp) > 0 && len(impExt) > 0 {
		b, _ := json.Marshal(impExt)
		out.Imp[0].Ext = b
	}

	return &out, errs
}

// setField writes val to the dot-path within the request or impExt map.
// Standard pass-through fields (source_type="standard") return nil from ApplyRule
// and never reach this function — they pass through unchanged.
func setField(req *openrtb.BidRequest, impExt *map[string]interface{}, path string, val interface{}) error {
	// imp.ext.* → write into impExt map
	if strings.HasPrefix(path, "imp.ext.") {
		key := strings.TrimPrefix(path, "imp.ext.")
		setNestedMap(*impExt, strings.Split(key, "."), val)
		return nil
	}

	switch path {
	case "user.buyeruid":
		if req.User == nil {
			req.User = &openrtb.User{}
		} else {
			u := *req.User
			req.User = &u
		}
		if s, ok := val.(string); ok {
			req.User.BuyerUID = s
		}
	case "device.ua":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.UA = s
		}
	case "device.ip":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.IP = s
		}
	case "device.language":
		if req.Device == nil {
			req.Device = &openrtb.Device{}
		} else {
			d := *req.Device
			req.Device = &d
		}
		if s, ok := val.(string); ok {
			req.Device.Language = s
		}
	case "site.page":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Page = s
		}
	case "site.domain":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Domain = s
		}
	case "site.publisher.id":
		if req.Site == nil {
			req.Site = &openrtb.Site{}
		} else {
			s := *req.Site
			req.Site = &s
		}
		if s, ok := val.(string); ok {
			req.Site.Publisher = &openrtb.Publisher{ID: s}
		}
	case "tmax":
		if i, ok := val.(int); ok {
			req.TMax = i
		}
	default:
		logger.Log.Debug().Str("path", path).Msg("composer: unhandled non-standard field path")
	}
	return nil
}

// setNestedMap sets a value at a nested key path in a map[string]interface{}.
func setNestedMap(m map[string]interface{}, keys []string, val interface{}) {
	if len(keys) == 1 {
		m[keys[0]] = val
		return
	}
	sub, ok := m[keys[0]].(map[string]interface{})
	if !ok {
		sub = make(map[string]interface{})
		m[keys[0]] = sub
	}
	setNestedMap(sub, keys[1:], val)
}
