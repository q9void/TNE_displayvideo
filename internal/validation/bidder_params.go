package validation

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// BidderParamsValidator validates bidder parameters against JSON schemas
type BidderParamsValidator struct {
	schemas map[string]*gojsonschema.Schema
}

// NewBidderParamsValidator creates a new validator
func NewBidderParamsValidator() *BidderParamsValidator {
	return &BidderParamsValidator{
		schemas: make(map[string]*gojsonschema.Schema),
	}
}

// LoadSchema loads a JSON schema for a specific bidder
func (v *BidderParamsValidator) LoadSchema(bidderCode string, schemaJSON []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("failed to load schema for %s: %w", bidderCode, err)
	}

	v.schemas[bidderCode] = schema
	return nil
}

// LoadSchemaFromDB loads a schema from the database param_schema JSONB column
func (v *BidderParamsValidator) LoadSchemaFromDB(bidderCode string, paramSchemaJSONB json.RawMessage) error {
	if paramSchemaJSONB == nil || len(paramSchemaJSONB) == 0 {
		return fmt.Errorf("no schema defined for bidder %s", bidderCode)
	}
	return v.LoadSchema(bidderCode, paramSchemaJSONB)
}

// Validate validates bidder parameters against the loaded schema
func (v *BidderParamsValidator) Validate(bidderCode string, params map[string]interface{}) (bool, []string, error) {
	schema, exists := v.schemas[bidderCode]
	if !exists {
		return false, nil, fmt.Errorf("no schema loaded for bidder %s", bidderCode)
	}

	// Convert params to JSON for validation
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(paramsJSON)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return false, nil, fmt.Errorf("validation error: %w", err)
	}

	if result.Valid() {
		return true, nil, nil
	}

	// Collect validation errors
	var errors []string
	for _, err := range result.Errors() {
		errors = append(errors, err.String())
	}

	return false, errors, nil
}

// ValidateRubiconParams validates Rubicon-specific parameters
func ValidateRubiconParams(params map[string]interface{}) (bool, []string) {
	var errors []string

	// Check required fields
	if _, ok := params["accountId"]; !ok {
		errors = append(errors, "accountId is required")
	}
	if _, ok := params["siteId"]; !ok {
		errors = append(errors, "siteId is required")
	}
	if _, ok := params["zoneId"]; !ok {
		errors = append(errors, "zoneId is required")
	}

	// Validate accountId is numeric
	if accountID, ok := params["accountId"]; ok {
		switch v := accountID.(type) {
		case int, int64, float64:
			// Valid
		case string:
			// Should be numeric string
			if v == "" {
				errors = append(errors, "accountId cannot be empty")
			}
		default:
			errors = append(errors, "accountId must be integer or numeric string")
		}
	}

	// Validate siteId is numeric
	if siteID, ok := params["siteId"]; ok {
		switch v := siteID.(type) {
		case int, int64, float64:
			// Valid
		case string:
			if v == "" {
				errors = append(errors, "siteId cannot be empty")
			}
		default:
			errors = append(errors, "siteId must be integer or numeric string")
		}
	}

	// Validate zoneId is numeric
	if zoneID, ok := params["zoneId"]; ok {
		switch v := zoneID.(type) {
		case int, int64, float64:
			// Valid
		case string:
			if v == "" {
				errors = append(errors, "zoneId cannot be empty")
			}
		default:
			errors = append(errors, "zoneId must be integer or numeric string")
		}
	}

	return len(errors) == 0, errors
}

// ValidateKargoParams validates Kargo-specific parameters
func ValidateKargoParams(params map[string]interface{}) (bool, []string) {
	var errors []string

	placementID, ok := params["placementId"]
	if !ok {
		errors = append(errors, "placementId is required")
		return false, errors
	}

	if str, ok := placementID.(string); !ok || str == "" {
		errors = append(errors, "placementId must be a non-empty string")
	}

	return len(errors) == 0, errors
}

// ValidatePubMaticParams validates PubMatic-specific parameters
func ValidatePubMaticParams(params map[string]interface{}) (bool, []string) {
	var errors []string

	// Required: publisherId
	if _, ok := params["publisherId"]; !ok {
		errors = append(errors, "publisherId is required")
	} else if str, ok := params["publisherId"].(string); !ok || str == "" {
		errors = append(errors, "publisherId must be a non-empty string")
	}

	// Required: adSlot
	if _, ok := params["adSlot"]; !ok {
		errors = append(errors, "adSlot is required")
	} else if str, ok := params["adSlot"].(string); !ok || str == "" {
		errors = append(errors, "adSlot must be a non-empty string")
	}

	return len(errors) == 0, errors
}

// ValidateSovrnParams validates Sovrn-specific parameters
func ValidateSovrnParams(params map[string]interface{}) (bool, []string) {
	var errors []string

	tagID, ok := params["tagid"]
	if !ok {
		errors = append(errors, "tagid is required")
		return false, errors
	}

	if str, ok := tagID.(string); !ok || str == "" {
		errors = append(errors, "tagid must be a non-empty string")
	}

	return len(errors) == 0, errors
}

// ValidateTripleLiftParams validates TripleLift-specific parameters
func ValidateTripleLiftParams(params map[string]interface{}) (bool, []string) {
	var errors []string

	inventoryCode, ok := params["inventoryCode"]
	if !ok {
		errors = append(errors, "inventoryCode is required")
		return false, errors
	}

	if str, ok := inventoryCode.(string); !ok || str == "" {
		errors = append(errors, "inventoryCode must be a non-empty string")
	}

	return len(errors) == 0, errors
}

// ValidateBidderParams validates parameters for any bidder using the appropriate validator
func ValidateBidderParams(bidderCode string, params map[string]interface{}) (bool, []string) {
	switch bidderCode {
	case "rubicon":
		return ValidateRubiconParams(params)
	case "kargo":
		return ValidateKargoParams(params)
	case "pubmatic":
		return ValidatePubMaticParams(params)
	case "sovrn":
		return ValidateSovrnParams(params)
	case "triplelift":
		return ValidateTripleLiftParams(params)
	default:
		// No specific validator - accept all params
		return true, nil
	}
}
