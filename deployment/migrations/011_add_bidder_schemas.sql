-- Migration 011: Add JSON Schemas for bidder parameter validation
-- This migration populates the param_schema column in bidders_new table
-- Schemas define required/optional parameters for each SSP adapter

BEGIN;

-- ============================================================================
-- Rubicon (Magnite) Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Rubicon Adapter Params",
  "description": "A schema which validates params accepted by the Rubicon adapter",
  "type": "object",
  "properties": {
    "accountId": {
      "type": ["integer", "string"],
      "pattern": "^\\\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the publisher account"
    },
    "siteId": {
      "type": ["integer", "string"],
      "pattern": "^\\\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the site selling the impression"
    },
    "zoneId": {
      "type": ["integer", "string"],
      "pattern": "^\\\\d+$",
      "minimum": 1,
      "description": "An ID which identifies the sub-section of the site where the impression is located"
    },
    "inventory": {
      "type": "object",
      "description": "An object defining arbitrary targeting key/value pairs related to the page",
      "additionalProperties": {
        "type": "array"
      }
    },
    "bidonmultiformat": {
      "type": "boolean",
      "description": "It determines if the request should be processed in multi format way."
    },
    "visitor": {
      "type": "object",
      "description": "An object defining arbitrary targeting key/value pairs related to the visitor",
      "additionalProperties": {
        "type": "array"
      }
    },
    "pchain": {
      "type": "string",
      "description": "A payment ID chain string"
    },
    "video": {
      "type": "object",
      "description": "An object defining additional Rubicon video parameters",
      "properties": {
        "language": {
          "type": "string",
          "description": "Language of the ad - should match content video"
        },
        "playerHeight": {
          "type": ["integer", "string"],
          "description": "Height in pixels of the video player"
        },
        "playerWidth": {
          "type": ["integer", "string"],
          "description": "Width in pixels of the video player"
        },
        "size_id": {
          "type": "integer",
          "description": "Rubicon size_id, used to describe type of video ad (preroll, postroll, etc)"
        },
        "skip": {
          "type": "integer",
          "description": "Can this ad be skipped (0 = no, 1 = yes)"
        },
        "skipdelay": {
          "type": "integer",
          "description": "number of seconds until the ad can be skipped"
        }
      }
    }
  },
  "required": ["accountId", "siteId", "zoneId"]
}'::jsonb
WHERE code = 'rubicon';

-- ============================================================================
-- Kargo Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
}'::jsonb
WHERE code = 'kargo';

-- ============================================================================
-- PubMatic Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
      "items": {
        "type": "string"
      }
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
          "key": {
            "type": "string"
          },
          "value": {
            "type": "array",
            "minItems": 1,
            "items": {
              "type": "string"
            }
          }
        },
        "required": ["key", "value"]
      }
    }
  },
  "required": ["publisherId"]
}'::jsonb
WHERE code = 'pubmatic';

-- ============================================================================
-- Sovrn Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
}'::jsonb
WHERE code = 'sovrn';

-- ============================================================================
-- TripleLift Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
}'::jsonb
WHERE code = 'triplelift';

-- ============================================================================
-- OMS (OpenMediation) Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
}'::jsonb
WHERE code = 'oms';

-- ============================================================================
-- Aniview Adapter Schema
-- ============================================================================

UPDATE bidders_new
SET param_schema = '{
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
}'::jsonb
WHERE code = 'aniview';

COMMIT;

-- ============================================================================
-- Verification
-- ============================================================================

-- Check which bidders have schemas defined:
-- SELECT code, name,
--        CASE WHEN param_schema IS NOT NULL THEN 'Yes' ELSE 'No' END as has_schema
-- FROM bidders_new
-- ORDER BY code;

-- View a specific schema:
-- SELECT code, jsonb_pretty(param_schema)
-- FROM bidders_new
-- WHERE code = 'rubicon';
