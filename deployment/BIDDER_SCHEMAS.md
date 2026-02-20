# Bidder Parameter Schemas Reference

**Last Updated:** 2026-02-14
**Migration:** `011_add_bidder_schemas.sql`
**Validation:** `internal/validation/bidder_params.go`

This document lists all JSON schemas for bidder adapter parameters stored in the `bidders_new.param_schema` column.

---

## 📋 Table of Contents

1. [Rubicon (Magnite)](#rubicon-magnite)
2. [PubMatic](#pubmatic)
3. [TripleLift](#triplelift)
4. [Kargo](#kargo)
5. [Sovrn](#sovrn)
6. [OMS (OpenMediation)](#oms-openmediatio)
7. [Aniview](#aniview)

---

## Rubicon (Magnite)

### Required Parameters
- `accountId` - Rubicon account ID (integer or numeric string)
- `siteId` - Site identifier (integer or numeric string)
- `zoneId` - Zone/placement identifier (integer or numeric string)

### Optional Parameters
- `inventory` - Key/value targeting for page context
- `bidonmultiformat` - Multi-format bidding flag (boolean)
- `visitor` - Key/value targeting for visitor
- `pchain` - Payment chain string
- `video` - Video-specific parameters object

### Example
```json
{
  "accountId": 26298,
  "siteId": 12345,
  "zoneId": 67890,
  "inventory": {
    "section": ["sports", "news"]
  },
  "bidonmultiformat": true
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Rubicon Adapter Params",
  "description": "A schema which validates params accepted by the Rubicon adapter",
  "type": "object",
  "properties": {
    "accountId": {
      "type": ["integer", "string"],
      "pattern": "^\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the publisher account"
    },
    "siteId": {
      "type": ["integer", "string"],
      "pattern": "^\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the site selling the impression"
    },
    "zoneId": {
      "type": ["integer", "string"],
      "pattern": "^\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the sub-section of the site"
    },
    "inventory": {
      "type": "object",
      "description": "Targeting key/value pairs related to the page",
      "additionalProperties": {"type": "array"}
    },
    "bidonmultiformat": {
      "type": "boolean",
      "description": "Process request in multi format way"
    },
    "visitor": {
      "type": "object",
      "description": "Targeting key/value pairs related to the visitor",
      "additionalProperties": {"type": "array"}
    },
    "pchain": {
      "type": "string",
      "description": "A payment ID chain string"
    },
    "video": {
      "type": "object",
      "properties": {
        "language": {"type": "string"},
        "playerHeight": {"type": ["integer", "string"]},
        "playerWidth": {"type": ["integer", "string"]},
        "size_id": {"type": "integer"},
        "skip": {"type": "integer"},
        "skipdelay": {"type": "integer"}
      }
    }
  },
  "required": ["accountId", "siteId", "zoneId"]
}
```

---

## PubMatic

### Required Parameters
- `publisherId` - PubMatic publisher identifier (string)

### Optional Parameters
- `adSlot` - Ad slot identifier (format: `adUnitName@WIDTHxHEIGHT`)
- `pmzoneid` - Comma-separated zone IDs for targeting
- `kadfloor` - Bid floor value (string)
- `dctr` - Deals custom targeting (pipe-separated key=value pairs)
- `acat` - Allowed categories array
- `wrapper` - OpenWrap configuration object
- `keywords` - Targeting keywords array

### Example
```json
{
  "publisherId": "166938",
  "adSlot": "billboard@728x90",
  "pmzoneid": "sports,news",
  "kadfloor": "0.50",
  "wrapper": {
    "profile": 12345,
    "version": 1
  }
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Pubmatic Adapter Params",
  "description": "A schema which validates params accepted by the Pubmatic adapter",
  "type": "object",
  "properties": {
    "publisherId": {
      "type": "string",
      "description": "An ID which identifies the publisher"
    },
    "adSlot": {
      "type": "string",
      "description": "An ID which identifies the ad slot"
    },
    "pmzoneid": {
      "type": "string",
      "description": "Comma separated zone id. Used in deal targeting & site section targeting. e.g drama,sport"
    },
    "kadfloor": {
      "type": "string",
      "description": "bid floor value set to imp.bidfloor"
    },
    "dctr": {
      "type": "string",
      "description": "Deals Custom Targeting, pipe separated key-value pairs e.g key1=V1,V2,V3|key2=v1|key3=v3,v5"
    },
    "acat": {
      "type": "array",
      "description": "List of allowed categories for a given auction to be sent in request.ext",
      "items": {"type": "string"}
    },
    "wrapper": {
      "type": "object",
      "description": "Specifies pubmatic openwrap configuration for a publisher",
      "properties": {
        "profile": {
          "type": "integer",
          "description": "An ID which identifies the openwrap profile of publisher"
        },
        "version": {
          "type": "integer",
          "description": "An ID which identifies version of the openwrap profile"
        }
      },
      "required": ["profile"]
    },
    "keywords": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "description": "A key with one or more values associated with it. These are used in buy-side segment targeting.",
        "properties": {
          "key": {"type": "string"},
          "value": {
            "type": "array",
            "minItems": 1,
            "items": {"type": "string"}
          }
        },
        "required": ["key", "value"]
      }
    }
  },
  "required": ["publisherId"]
}
```

---

## TripleLift

### Required Parameters
- `inventoryCode` - TripleLift inventory code (provided by partner manager)

### Optional Parameters
- `floor` - Bid floor price (number)

### Example
```json
{
  "inventoryCode": "total_pro_sports_billboard",
  "floor": 0.50
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Triplelift Adapter Params",
  "description": "A schema which validates params accepted by the Triplelift adapter",
  "type": "object",
  "properties": {
    "inventoryCode": {
      "type": "string",
      "description": "TripleLift inventory code for this ad unit (provided to you by your partner manager)"
    },
    "floor": {
      "type": "number",
      "description": "the bid floor"
    }
  },
  "required": ["inventoryCode"]
}
```

---

## Kargo

### Required Parameters
- `placementId` - Kargo placement identifier (string) **OR**
- `adSlotID` - Kargo placement identifier (string, deprecated - use `placementId`)

### Example
```json
{
  "placementId": "_kH12nDpoKl"
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Kargo Adapter Params",
  "description": "A schema which validates params accepted by the Kargo adapter",
  "type": "object",
  "properties": {
    "placementId": {
      "type": "string",
      "description": "An ID which identifies the adslot placement. Equivalent to the id of target inventory, ad unit code, or placement id"
    },
    "adSlotID": {
      "type": "string",
      "description": "[Deprecated: Use placementId] An ID which identifies the adslot placement. Equivalent to the id of target inventory, ad unit code, or placement id"
    }
  },
  "oneOf": [
    {"required": ["placementId"]},
    {"required": ["adSlotID"]}
  ]
}
```

---

## Sovrn

### Required Parameters
- `tagid` - Sovrn tag identifier (string) **OR**
- `tagId` - Sovrn tag identifier (string, deprecated - use `tagid`)

### Optional Parameters
- `bidfloor` - Bid floor price (number or string)
- `adunitcode` - Ad unit identifier (string)

### Example
```json
{
  "tagid": "123456",
  "bidfloor": 0.25,
  "adunitcode": "billboard_desktop"
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Sovrn Adapter Params",
  "description": "A schema which validates params accepted by the Sovrn adapter",
  "type": "object",
  "properties": {
    "tagid": {
      "type": "string",
      "description": "An ID which identifies the sovrn ad tag"
    },
    "tagId": {
      "type": "string",
      "description": "An ID which identifies the sovrn ad tag (DEPRECATED, use tagid instead)"
    },
    "bidfloor": {
      "anyOf": [
        {
          "type": "number",
          "description": "The minimum acceptable bid, in CPM, using US Dollars"
        },
        {
          "type": "string",
          "description": "The minimum acceptable bid, in CPM, using US Dollars (as a string)"
        }
      ]
    },
    "adunitcode": {
      "type": "string",
      "description": "The string which identifies Ad Unit"
    }
  },
  "oneOf": [
    {"required": ["tagid"]},
    {"required": ["tagId"]}
  ]
}
```

---

## OMS (OpenMediation)

### Required Parameters
- `publisherId` - OMS publisher identifier (string)
- `placementId` - OMS placement identifier (string)

### Example
```json
{
  "publisherId": "12345",
  "placementId": "billboard_desktop"
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "OMS Adapter Params",
  "type": "object",
  "properties": {
    "publisherId": {
      "type": "string",
      "description": "OMS publisher ID"
    },
    "placementId": {
      "type": "string",
      "description": "OMS placement ID"
    }
  },
  "required": ["publisherId", "placementId"]
}
```

---

## Aniview

### Required Parameters
- `publisherId` - Aniview publisher identifier (string)
- `channelId` - Aniview channel identifier (string)

### Example
```json
{
  "publisherId": "12345",
  "channelId": "sports_video"
}
```

### Full Schema
```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Aniview Adapter Params",
  "type": "object",
  "properties": {
    "publisherId": {
      "type": "string",
      "description": "Aniview publisher ID"
    },
    "channelId": {
      "type": "string",
      "description": "Aniview channel ID"
    }
  },
  "required": ["publisherId", "channelId"]
}
```

---

## Usage in Database

### Storing in New Schema

These schemas are stored in the `bidders_new.param_schema` column:

```sql
-- View all bidder schemas
SELECT
    code,
    name,
    CASE WHEN param_schema IS NOT NULL THEN 'Yes' ELSE 'No' END as has_schema,
    jsonb_pretty(param_schema->'required') as required_params
FROM bidders_new
ORDER BY code;
```

### Example Bidder Config

```sql
-- Insert a bidder config with validated params
INSERT INTO slot_bidder_configs (
    ad_slot_id,
    bidder_id,
    device_type,
    bidder_params
) VALUES (
    1,  -- ad_slot_id for billboard
    (SELECT id FROM bidders_new WHERE code = 'rubicon'),
    'desktop',
    '{
        "accountId": 26298,
        "siteId": 12345,
        "zoneId": 67890,
        "bidonmultiformat": true
    }'::jsonb
);
```

---

## Validation in Go

### Using the Validation Framework

```go
import "github.com/thenexusengine/tne_springwire/internal/validation"

// Create validator
validator := validation.NewBidderParamsValidator()

// Load schemas from database
rows, _ := db.Query("SELECT code, param_schema FROM bidders_new WHERE param_schema IS NOT NULL")
for rows.Next() {
    var code string
    var schema json.RawMessage
    rows.Scan(&code, &schema)
    validator.LoadSchemaFromDB(code, schema)
}

// Validate Rubicon params
params := map[string]interface{}{
    "accountId": 26298,
    "siteId": 12345,
    "zoneId": 67890,
}

valid, errors, err := validator.Validate("rubicon", params)
if !valid {
    fmt.Printf("Validation errors: %v\n", errors)
}
```

### Quick Validation Functions

```go
// Validate specific bidders without loading schemas
valid, errors := validation.ValidateRubiconParams(params)
valid, errors := validation.ValidatePubMaticParams(params)
valid, errors := validation.ValidateKargoParams(params)
valid, errors := validation.ValidateSovrnParams(params)
valid, errors := validation.ValidateTripleLiftParams(params)
```

---

## Database Storage Example

### Example: Total Pro Sports Billboard (Desktop)

```sql
-- Account
account_id: '12345'
name: 'BizBudding Network'

-- Publisher
domain: 'totalprosports.com'
name: 'Total Pro Sports'

-- Ad Slot
slot_pattern: 'totalprosports.com/billboard'
slot_name: 'billboard'

-- Bidder Configs (7 bidders)
1. Rubicon Desktop:
   {
     "accountId": 26298,
     "siteId": 12345,
     "zoneId": 67890
   }

2. PubMatic Desktop:
   {
     "publisherId": "166938",
     "adSlot": "billboard@728x90"
   }

3. Kargo Desktop:
   {
     "placementId": "_kH12nDpoKl"
   }

4. Sovrn Desktop:
   {
     "tagid": "123456"
   }

5. TripleLift Desktop:
   {
     "inventoryCode": "total_pro_sports_billboard"
   }

6. OMS Desktop:
   {
     "publisherId": "12345",
     "placementId": "billboard_desktop"
   }

7. Aniview Desktop:
   {
     "publisherId": "12345",
     "channelId": "sports_video"
   }
```

---

## Testing Schemas

### PostgreSQL Validation

```sql
-- Test Rubicon params against schema
SELECT jsonb_schema_is_valid(
    (SELECT param_schema FROM bidders_new WHERE code = 'rubicon'),
    '{"accountId": 26298, "siteId": 12345, "zoneId": 67890}'::jsonb
);
```

### Go Unit Tests

```go
func TestRubiconSchemaValidation(t *testing.T) {
    // Valid params
    valid := map[string]interface{}{
        "accountId": 26298,
        "siteId": 12345,
        "zoneId": 67890,
    }
    ok, errors := validation.ValidateRubiconParams(valid)
    assert.True(t, ok)
    assert.Empty(t, errors)

    // Invalid params (missing zoneId)
    invalid := map[string]interface{}{
        "accountId": 26298,
        "siteId": 12345,
    }
    ok, errors = validation.ValidateRubiconParams(invalid)
    assert.False(t, ok)
    assert.Contains(t, errors[0], "zoneId is required")
}
```

---

## Schema Updates

To add a new bidder schema:

1. **Create SQL migration:**
```sql
UPDATE bidders_new
SET param_schema = '{...}'::jsonb
WHERE code = 'new_bidder';
```

2. **Add validation function:**
```go
func ValidateNewBidderParams(params map[string]interface{}) (bool, []string) {
    // Validation logic
}
```

3. **Update switch statement:**
```go
case "new_bidder":
    return ValidateNewBidderParams(params)
```

---

## References

- **Migration:** `deployment/migrations/011_add_bidder_schemas.sql`
- **Validation:** `internal/validation/bidder_params.go`
- **Adapters:** `internal/adapters/{bidder}/`
- **Deployment:** `deployment/DEPLOYMENT_GUIDE.md`
