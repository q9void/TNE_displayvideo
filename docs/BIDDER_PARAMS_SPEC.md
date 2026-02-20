# Bidder Parameter Specifications

This document defines the correct parameter structure for each SSP bidder based on official Prebid Server documentation.

## Summary of Fixes

### ✅ Type Corrections Applied
1. **Sovrn `tagid`**: Changed from `int` to `string` per [Prebid Server spec](https://docs.prebid.org/dev-docs/bidders/sovrn.html)
2. **Pubmatic `publisherId`**: Changed from `int` to `string` per [Prebid Server spec](https://docs.prebid.org/dev-docs/bidders/pubmatic.html)
3. **Pubmatic `adSlot`**: Changed from `int` to `string` per [Prebid Server spec](https://docs.prebid.org/dev-docs/bidders/pubmatic.html)
4. **Mapping file updated**: All integer values converted to strings for Sovrn and Pubmatic

### ✅ Ad Unit Code
- **`imp.tagid`**: Correctly set to `slot.AdUnitPath` (e.g., "dev.totalprosports.com/billboard")
- This is passed in the OpenRTB impression object for all bidders

---

## Bidder-Specific Parameters

### 1. Rubicon/Magnite ✅

**Location in request:** `imp.ext.rubicon`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `accountId` | integer | ✅ Required | Publisher account ID |
| `siteId` | integer | ✅ Required | Unique ID for site |
| `zoneId` | integer | ✅ Required | Unique ID for ad placement |
| `bidonmultiformat` | boolean | Optional | Enable multi-format bidding |

**Example:**
```json
{
  "accountId": 26298,
  "siteId": 556630,
  "zoneId": 3767186,
  "bidonmultiformat": false
}
```

**Source:** [Rubicon Prebid Server Docs](https://docs.prebid.org/dev-docs/bidders/rubicon.html)

---

### 2. Kargo ✅

**Location in request:** `imp.ext.kargo`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `placementId` | string | ✅ Required | Kargo placement identifier |

**Example:**
```json
{
  "placementId": "_o9n8eh8Lsw"
}
```

**Source:** [Kargo Prebid Server Docs](https://docs.prebid.org/dev-docs/bidders/kargo.html)

---

### 3. Sovrn ✅ (Fixed)

**Location in request:** `imp.ext.sovrn`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tagid` | **string** | ✅ Required | Sovrn Ad Tag ID |

**⚠️ Important:** `tagid` must be a **string**, not an integer!

**Example:**
```json
{
  "tagid": "1294952"
}
```

**Source:** [Sovrn Prebid Server Docs](https://docs.prebid.org/dev-docs/bidders/sovrn.html)

---

### 4. Pubmatic ✅ (Fixed)

**Location in request:** `imp.ext.pubmatic`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `publisherId` | **string** | ✅ Required | PubMatic publisher identifier |
| `adSlot` | **string** | Optional | Ad Tag Name or ID |

**⚠️ Important:** Both `publisherId` and `adSlot` must be **strings**, not integers!

**Example:**
```json
{
  "publisherId": "166938",
  "adSlot": "7079290"
}
```

**Source:** [Pubmatic Prebid Server Docs](https://docs.prebid.org/dev-docs/bidders/pubmatic.html)

---

### 5. Triplelift ✅

**Location in request:** `imp.ext.triplelift`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `inventoryCode` | string | ✅ Required | TripleLift inventory code |
| `floor` | float | Optional | Bid floor |

**Example:**
```json
{
  "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"
}
```

**Source:** [Triplelift Prebid Server Docs](https://docs.prebid.org/dev-docs/bidders/triplelift.html)

---

## OpenRTB Impression Structure

### Complete Example

```json
{
  "id": "1",
  "banner": {
    "w": 300,
    "h": 250,
    "format": [
      {"w": 300, "h": 250},
      {"w": 728, "h": 90}
    ]
  },
  "tagid": "dev.totalprosports.com/billboard",  // ✅ Ad unit code
  "ext": {
    "rubicon": {
      "accountId": 26298,
      "siteId": 556630,
      "zoneId": 3767186,
      "bidonmultiformat": false
    },
    "kargo": {
      "placementId": "_o9n8eh8Lsw"
    },
    "sovrn": {
      "tagid": "1294952"  // ✅ String, not int
    },
    "pubmatic": {
      "publisherId": "166938",  // ✅ String, not int
      "adSlot": "7079290"       // ✅ String, not int
    },
    "triplelift": {
      "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"
    }
  }
}
```

---

## Configuration Hierarchy

Based on client requirements, bidders use different configuration levels:

| Bidder | Config Level | Database Table | Lookup Priority |
|--------|--------------|----------------|-----------------|
| **Pubmatic** | Per-publisher | `publishers.bidder_params` | Publisher only |
| **Triplelift** | Per-publisher | `publishers.bidder_params` | Publisher only |
| **Kargo** | Per-unit per-domain | `unit_bidder_configs` | Unit → Domain → Publisher |
| **Sovrn** | Per-unit per-domain | `unit_bidder_configs` | Unit → Domain → Publisher |
| **Rubicon** | Per-publisher | `publishers.bidder_params` | Publisher only |

---

## Database Configuration Examples

### Per-Publisher (Pubmatic, Triplelift, Rubicon)

```sql
-- Store in publishers.bidder_params JSONB column
UPDATE publishers
SET bidder_params = jsonb_set(
  bidder_params,
  '{pubmatic}',
  '{"publisherId": "166938", "adSlot": "7079290"}'
)
WHERE publisher_id = 'icisic-media';
```

### Per-Unit (Kargo, Sovrn)

```sql
-- Store in unit_bidder_configs table
INSERT INTO unit_bidder_configs (publisher_id, domain, ad_unit_path, bidder_code, params)
VALUES (
  'icisic-media',
  'dev.totalprosports.com',
  'dev.totalprosports.com/billboard',
  'kargo',
  '{"placementId": "_o9n8eh8Lsw"}'
);

INSERT INTO unit_bidder_configs (publisher_id, domain, ad_unit_path, bidder_code, params)
VALUES (
  'icisic-media',
  'dev.totalprosports.com',
  'dev.totalprosports.com/billboard',
  'sovrn',
  '{"tagid": "1294952"}'  -- ⚠️ Must be string in JSON
);
```

---

## Testing Checklist

When adding new bidder configurations:

- [ ] Verify parameter types match Prebid Server spec
- [ ] Ensure strings use quotes in JSON (e.g., `"tagid": "123"`, not `"tagid": 123`)
- [ ] Confirm `imp.tagid` is set to the ad unit path
- [ ] Test with hierarchical config lookup (unit → domain → publisher)
- [ ] Validate JSON structure before inserting into database
- [ ] Check adapter logs for parameter validation errors

---

## References

- [Prebid Server Bidder List](https://docs.prebid.org/dev-docs/pbs-bidders.html)
- [Kargo Bidder Params](https://docs.prebid.org/dev-docs/bidders/kargo.html)
- [Sovrn Bidder Params](https://docs.prebid.org/dev-docs/bidders/sovrn.html)
- [Pubmatic Bidder Params](https://docs.prebid.org/dev-docs/bidders/pubmatic.html)
- [Triplelift Bidder Params](https://docs.prebid.org/dev-docs/bidders/triplelift.html)
- [Rubicon Bidder Params](https://docs.prebid.org/dev-docs/bidders/rubicon.html)
