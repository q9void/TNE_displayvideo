package routing

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// HTTPContext holds values extracted from the live HTTP request.
type HTTPContext struct {
	UserAgent      string
	IP             string
	AcceptLanguage string
	Headers        http.Header
}

// ApplyRule resolves the value for a single BidderFieldRule.
// Returns (value, nil) on success; (nil, err) if required and not resolvable.
// Returns (nil, nil) for optional rules where the source is absent.
func ApplyRule(
	rule storage.BidderFieldRule,
	user *openrtb.User,
	slotParams map[string]interface{},
	httpCtx *HTTPContext,
	accountParams map[string]interface{},
) (interface{}, error) {
	var raw interface{}

	switch rule.SourceType {
	case "standard":
		// Pass-through: caller handles this by copying the field from the incoming request.
		return nil, nil

	case "constant":
		raw = *rule.SourceRef

	case "slot_param":
		key := *rule.SourceRef
		val, ok := slotParams[key]
		if !ok {
			if rule.Required {
				return nil, fmt.Errorf("required slot_param %q not found", key)
			}
			return nil, nil
		}
		raw = val

	case "account_param":
		key := *rule.SourceRef
		val, ok := accountParams[key]
		if !ok {
			if rule.Required {
				return nil, fmt.Errorf("required account_param %q not found", key)
			}
			return nil, nil
		}
		raw = val

	case "http_context":
		if httpCtx == nil {
			return nil, nil
		}
		header := *rule.SourceRef
		raw = httpCtx.Headers.Get(header)
		if raw == "" {
			return nil, nil
		}

	case "eid":
		if user == nil {
			return nil, nil
		}
		uid := ResolveEID(user, *rule.SourceRef)
		if uid == "" {
			return nil, nil
		}
		raw = uid

	default:
		return nil, fmt.Errorf("unknown source_type %q", rule.SourceType)
	}

	return ApplyTransform(raw, rule.Transform)
}

// ApplyTransform applies a named transform to a value.
func ApplyTransform(value interface{}, transform string) (interface{}, error) {
	switch transform {
	case "none", "":
		return value, nil

	case "to_int":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("to_int: cannot parse %q: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("to_int: unsupported type %T", value)
		}

	case "to_string":
		return fmt.Sprintf("%v", value), nil

	case "to_string_array":
		return []string{fmt.Sprintf("%v", value)}, nil

	case "lowercase":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("lowercase: expected string, got %T", value)
		}
		return strings.ToLower(s), nil

	case "url_encode":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("url_encode: expected string, got %T", value)
		}
		return strings.NewReplacer(" ", "+", "&", "%26", "=", "%3D").Replace(s), nil

	case "array_first":
		switch v := value.(type) {
		case []interface{}:
			if len(v) == 0 {
				return nil, nil
			}
			return v[0], nil
		case []string:
			if len(v) == 0 {
				return nil, nil
			}
			return v[0], nil
		default:
			return value, nil // already scalar
		}

	case "csv_to_array":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("csv_to_array: expected string, got %T", value)
		}
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out, nil

	case "sha256":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("sha256: expected string, got %T", value)
		}
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:]), nil

	case "base64_decode":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("base64_decode: expected string, got %T", value)
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64_decode: %w", err)
		}
		return string(b), nil

	case "wrap_ext_rp":
		// Rubicon-specific: wrap value in {"rp": value}
		return map[string]interface{}{"rp": value}, nil

	default:
		// Unknown transforms are treated as no-op to avoid breaking live requests.
		return value, nil
	}
}
