# Publisher GPT & GAM Setup Guide

This guide covers what publishers need to do **once** in Google Ad Manager to enable Catalyst bids to win impressions.

The Catalyst SDK (v1.1.0+) handles all on-page coordination automatically — it disables GPT's initial load, waits for bids, sets targeting keys, and triggers the refresh. **Publishers do not need to change their existing GPT code.**

---

## What the SDK Does Automatically

- `googletag.pubads().disableInitialLoad()` — prevents GPT firing before bids are ready
- Sets `hb_pb_catalyst`, `hb_adid_catalyst`, `hb_size_catalyst`, `hb_bidder_catalyst` on each slot
- Calls `googletag.pubads().refresh()` once bids are ready (or on timeout)

## What Publishers Must Set Up in GAM

### Step 1: Create Custom Key-Values

In GAM: **Inventory → Custom Targeting → New Key**

Create these four keys (all type: **String**):

| Key name | Values |
|----------|--------|
| `hb_pb_catalyst` | Price tier values e.g. `0.50`, `1.00`, `1.50` … `20.00` |
| `hb_adid_catalyst` | Free-form (any) |
| `hb_size_catalyst` | e.g. `300x250`, `728x90`, `970x250` |
| `hb_bidder_catalyst` | e.g. `kargo`, `rubicon`, `appnexus` |

> GAM truncates key names exceeding 20 characters. All keys above are within the limit.

---

### Step 2: Create Price-Priority Line Items

Create one line item per CPM tier. Recommended price ladder:

- $0.50 steps from **$0.50 → $5.00** (10 line items)
- $1.00 steps from **$6.00 → $20.00** (15 line items)

**Settings for each line item:**

| Field | Value |
|-------|-------|
| Type | Price Priority |
| Priority | 12 |
| Rate | $X.XX CPM (matching the tier) |
| Targeting | `hb_pb_catalyst = X.XX` |

Example for the $2.00 tier:
```
Name:      Catalyst HB $2.00
Type:      Price Priority
Priority:  12
Rate:      $2.00 CPM
Targeting: hb_pb_catalyst = 2.00
```

---

### Step 3: Add Creative to Each Line Item

**Creative type:** 3rd Party Tag

**Creative code:**
```html
<script src="https://ads.thenexusengine.com/ad/render?adid=%%PATTERN:hb_adid_catalyst%%&w=%%WIDTH%%&h=%%HEIGHT%%"></script>
```

- `%%PATTERN:hb_adid_catalyst%%` — GAM macro that injects the winning bid's creative ID
- `%%WIDTH%%` / `%%HEIGHT%%` — standard GAM size macros

**Add this single creative to all price-tier line items** (GAM allows creative reuse).

---

## Minimal On-Page Snippet

Publishers only need to load the SDK and call `requestBids`. No GPT changes required:

```html
<!-- Load Catalyst SDK -->
<script async src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>
<script>
  window.catalyst = window.catalyst || { cmd: [] };
  catalyst.cmd.push(function() {
    catalyst.init({
      accountId: 'YOUR_PUBLISHER_ID',
      autoRefreshGPT: true  // Required — lets Catalyst manage GPT refresh timing
    });
    catalyst.requestBids({
      accountId: 'YOUR_PUBLISHER_ID',
      slots: [
        { divId: 'div-leaderboard',  sizes: [[728,90],[970,250]] },
        { divId: 'div-rectangle',    sizes: [[300,250]] }
      ]
    });
    // No callback needed — SDK sets targeting and triggers GPT refresh automatically
  });
</script>
```

The `divId` values must match the element IDs used in `googletag.defineSlot(...)`.

---

## Verification

Open browser DevTools console (with `catalyst._config.debug = true` if needed):

1. `[Catalyst] GPT initial load disabled` — confirms SDK is in control of refresh timing
2. `[Catalyst] Set slot targeting for div-X CPM: 2.08` — confirms bid targeting is set
3. `[Catalyst] GPT refresh triggered for 2 slot(s)` — confirms GPT is told to fetch ads

To inspect targeting keys directly:
```javascript
googletag.pubads().getSlots().forEach(function(slot) {
  console.log(slot.getSlotElementId(), slot.getTargeting('hb_pb_catalyst'));
});
```

Expected: `["2.08"]` (or whatever the winning CPM is).

---

## Troubleshooting

| Symptom | Likely cause |
|---------|-------------|
| `hb_pb_catalyst: []` | No bid was returned for that slot |
| Slot renders a house ad | GAM line items not set up, or price tier not covered |
| Slot is blank | `disableInitialLoad` fired but no line item matched — check key-value targeting in GAM |
| SDK not logging | Confirm `catalyst-sdk.js` is loading: check Network tab for `200 OK` |
