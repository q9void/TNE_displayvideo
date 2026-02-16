# Bidder Mapping Reference

This document explains the bidder parameter mapping from BizBudding Excel to Catalyst configuration.

---

## Excel Source

**File:** `docs/integrations/tps-onboarding.xlsx`
**Sheet:** `PLACEMENTS`

### Row Structure

- **Rows 3-13:** Desktop placements
- **Rows 19-29:** Mobile placements
- **Column A:** Slot Pattern (ad unit identifier)

### Column Mapping

| Excel Column | Parameter | Bidder | Type |
|--------------|-----------|--------|------|
| F | `accountId` | Rubicon/Magnite | int |
| G | `siteId` | Rubicon/Magnite | int |
| H | `zoneId` | Rubicon/Magnite | int |
| I | `bidonmultiformat` | Rubicon/Magnite | bool |
| J | `placementId` | Kargo | string |
| K | `tagid` | Sovrn | **string** |
| L | `publisherId` | OMS (Onemobile) | int |
| N | `publisherId` | Aniview | string |
| O | `channelId` | Aniview | string |
| P | `publisherId` | Pubmatic | **string** |
| Q | `adSlot` | Pubmatic | **string** |
| R | `inventoryCode` | Triplelift | string |

---

## JSON Configuration

**File:** `config/bizbudding-all-bidders-mapping.json`

### Structure

```json
{
  "publisher": {
    "publisherId": "icisic-media",
    "domain": "totalprosports.com",
    "defaultBidders": ["rubicon", "kargo", "sovrn", "pubmatic", "triplelift"]
  },
  "adUnits": {
    "<slot-pattern>": {
      "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false },
      "kargo": { "placementId": "_o9n8eh8Lsw" },
      "sovrn": { "tagid": "1277816" },
      "oms": { "publisherId": 21146 },
      "aniview": { "publisherId": "...", "channelId": "..." },
      "pubmatic": { "publisherId": "166938", "adSlot": "7079290" },
      "triplelift": { "inventoryCode": "BizBudding_RON_HDX_pbc2s" }
    }
  }
}
```

---

## Complete Ad Unit Mapping

### Desktop Placements

#### 1. totalprosports.com/billboard
**Size:** 970x250
```json
{
  "rubicon": {
    "accountId": 26298,
    "siteId": 556630,
    "zoneId": 3767186,
    "bidonmultiformat": false
  },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294952 },
  "oms": { "publisherId": 21146 },
  "aniview": {
    "publisherId": "66aa757144c99c7ca504e937",
    "channelId": "6806a79f20173d1cde0a4895"
  },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

#### 2. totalprosports.com/billboard-wide
**Size:** 970x250
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3775672, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294952 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

#### 3. totalprosports.com/leaderboard
**Size:** 728x90, 970x90
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3767184, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1277816 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_HDX_pbc2s" }
}
```

#### 4. totalprosports.com/leaderboard-wide
**Size:** 970x90
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3775674, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294952 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

#### 5. totalprosports.com/leaderboard-wide-adhesion
**Size:** 970x90 (sticky)
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3775676, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294952 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

#### 6. totalprosports.com/skyscraper
**Size:** 160x600
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3767188, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1277820 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_HDX_pbc2s" }
}
```

#### 7. totalprosports.com/skyscraper-wide
**Size:** 300x600
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3775668, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294950 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

#### 8. totalprosports.com/skyscraper-wide-adhesion
**Size:** 300x600 (sticky)
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3775670, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1294950 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s" }
}
```

### Mobile Placements

#### 9. totalprosports.com/rectangle-medium
**Size:** 300x250
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3767180, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1277818 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_HDX_pbc2s" }
}
```

#### 10. totalprosports.com/rectangle-medium-adhesion
**Size:** 300x250 (sticky)
```json
{
  "rubicon": { "accountId": 26298, "siteId": 556630, "zoneId": 3767182, "bidonmultiformat": false },
  "kargo": { "placementId": "_o9n8eh8Lsw" },
  "sovrn": { "tagid": 1277818 },
  "oms": { "publisherId": 21146 },
  "aniview": { "publisherId": "66aa757144c99c7ca504e937", "channelId": "6806a79f20173d1cde0a4895" },
  "pubmatic": { "publisherId": 166938, "adSlot": 7079290 },
  "triplelift": { "inventoryCode": "BizBudding_RON_HDX_pbc2s" }
}
```

---

## OpenRTB Integration

### How Parameters Are Used

When a bid request comes in with `adUnitPath: "totalprosports.com/leaderboard"`, the system:

1. **Looks up the ad unit** in `config/bizbudding-all-bidders-mapping.json`
2. **Extracts all bidder parameters** for that ad unit
3. **Injects them into OpenRTB** `imp.ext` field

### Example OpenRTB Request

```json
{
  "id": "catalyst-123456789",
  "imp": [{
    "id": "1",
    "banner": {
      "w": 728,
      "h": 90,
      "format": [
        {"w": 728, "h": 90},
        {"w": 970, "h": 90}
      ]
    },
    "tagid": "totalprosports.com/leaderboard",
    "ext": {
      "rubicon": {
        "accountId": 26298,
        "siteId": 556630,
        "zoneId": 3767184,
        "bidonmultiformat": false
      },
      "kargo": {
        "placementId": "_o9n8eh8Lsw"
      },
      "sovrn": {
        "tagid": 1277816
      },
      "onetag": {
        "publisherId": 21146
      },
      "aniview": {
        "publisherId": "66aa757144c99c7ca504e937",
        "channelId": "6806a79f20173d1cde0a4895"
      },
      "pubmatic": {
        "publisherId": 166938,
        "adSlot": 7079290
      },
      "triplelift": {
        "inventoryCode": "BizBudding_RON_HDX_pbc2s"
      }
    }
  }],
  "site": {
    "id": "icisic-media",
    "domain": "totalprosports.com"
  }
}
```

---

## Bidder Adapter Mapping

| Our Config | OpenRTB Field | Adapter |
|------------|---------------|---------|
| `rubicon` | `imp.ext.rubicon` | `internal/adapters/rubicon` |
| `kargo` | `imp.ext.kargo` | `internal/adapters/kargo` |
| `sovrn` | `imp.ext.sovrn` | `internal/adapters/sovrn` |
| `oms` | `imp.ext.onetag` | `internal/adapters/onetag` |
| `aniview` | `imp.ext.aniview` | `internal/adapters/aniview` |
| `pubmatic` | `imp.ext.pubmatic` | `internal/adapters/pubmatic` |
| `triplelift` | `imp.ext.triplelift` | `internal/adapters/triplelift` |

**Note:** OMS uses the "onetag" adapter name in OpenRTB.

---

## Updating the Mapping

### If Excel Changes

1. Update the Excel file: `docs/integrations/tps-onboarding.xlsx`
2. Re-run conversion script:
   ```bash
   python3 scripts/convert-excel-to-json.py
   ```
3. Review changes:
   ```bash
   git diff config/bizbudding-all-bidders-mapping.json
   ```
4. Rebuild and redeploy:
   ```bash
   go build -o build/catalyst-server ./cmd/server
   ./scripts/deploy-catalyst.sh
   ```

### Manual Edits

You can manually edit `config/bizbudding-all-bidders-mapping.json`:

```bash
# Edit the file
vim config/bizbudding-all-bidders-mapping.json

# Validate JSON syntax
cat config/bizbudding-all-bidders-mapping.json | jq . > /dev/null

# Redeploy
./scripts/deploy-catalyst.sh
```

---

## Common Constants

All ad units share these common values:

- **Rubicon accountId:** 26298
- **Rubicon siteId:** 556630
- **OMS publisherId:** 21146
- **Aniview publisherId:** 66aa757144c99c7ca504e937
- **Aniview channelId:** 6806a79f20173d1cde0a4895
- **Pubmatic publisherId:** 166938
- **Pubmatic adSlot:** 7079290
- **Kargo placementId:** _o9n8eh8Lsw

**Only varies:**
- Rubicon zoneId (unique per ad unit)
- Sovrn tagid (unique per ad unit)
- Triplelift inventoryCode (unique per ad unit)

---

## Validation

### Check Mapping File

```bash
# Total ad units
cat config/bizbudding-all-bidders-mapping.json | jq '.adUnits | length'
# Expected: 10

# List all ad units
cat config/bizbudding-all-bidders-mapping.json | jq -r '.adUnits | keys[]'

# Check specific ad unit
cat config/bizbudding-all-bidders-mapping.json | jq '.adUnits["totalprosports.com/leaderboard"]'

# Count bidders per ad unit
cat config/bizbudding-all-bidders-mapping.json | jq '.adUnits["totalprosports.com/leaderboard"] | length'
# Expected: 7
```

### Verify Parameters

```bash
# Check Rubicon parameters
cat config/bizbudding-all-bidders-mapping.json | jq '.adUnits | to_entries[] | {ad_unit: .key, zoneId: .value.rubicon.zoneId}'

# Check all bidders present
cat config/bizbudding-all-bidders-mapping.json | jq '.adUnits | to_entries[] | {ad_unit: .key, bidders: (.value | keys)}'
```

---

## References

- **Source Excel:** `docs/integrations/tps-onboarding.xlsx`
- **Generated Config:** `config/bizbudding-all-bidders-mapping.json`
- **Conversion Script:** `scripts/convert-excel-to-json.py`
- **Handler Code:** `internal/endpoints/catalyst_bid_handler.go:272-342`
