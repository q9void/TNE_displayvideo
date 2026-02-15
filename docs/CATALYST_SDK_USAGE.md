# Catalyst SDK Usage Guide

## Quick Start

### 1. Load the SDK

```html
<script src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>
```

### 2. Initialize

```javascript
catalyst.init({
  accountId: 'your-account-id',           // Required: Your MAI Publisher account ID
  serverUrl: 'https://ads.thenexusengine.com',  // Required: Catalyst server URL
  timeout: 2800,                          // Optional: Timeout in ms (default: 2800)
  debug: true                             // Optional: Enable debug logging
});
```

### 3. Request Bids

```javascript
catalyst.requestBids({
  slots: [
    {
      divId: 'ad-slot-1',
      sizes: [[728, 90], [970, 250]],
      adUnitPath: '/homepage/leaderboard',
      position: 'atf'  // above-the-fold
    }
  ]
}, function(bids) {
  console.log('Received bids:', bids);

  bids.forEach(function(bid) {
    // Render ad
    document.getElementById(bid.divId).innerHTML = bid.ad;
  });
});
```

## Common Issues

### Issue: HTTP 404 Error

**Symptom:**
```
[Catalyst] Bid request failed: Error: HTTP 404
```

**Cause:** Missing `serverUrl` in initialization

**Solution:** Always specify `serverUrl`:
```javascript
catalyst.init({
  accountId: 'your-account-id',
  serverUrl: 'https://ads.thenexusengine.com',  // ← Must include this!
  debug: true
});
```

### Issue: SDK Not Loading

**Symptom:** `catalyst is not defined`

**Cause:** Script tag using relative path from wrong domain

**Solution:** Use absolute URL:
```html
<!-- ✅ Correct -->
<script src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>

<!-- ❌ Wrong (only works on ads.thenexusengine.com) -->
<script src="/assets/catalyst-sdk.js"></script>
```

### Issue: CORS Errors

**Symptom:** Cross-origin request blocked

**Cause:** Testing from `file://` or unauthorized domain

**Solutions:**
1. Test from a web server (not local files)
2. Use the test page: `tests/catalyst_sdk_test.html`
3. Server already has CORS enabled, so this should rarely happen

## Configuration Options

### Required

| Option | Type | Description |
|--------|------|-------------|
| `accountId` | string | Your MAI Publisher account ID |
| `serverUrl` | string | Catalyst server URL (usually `https://ads.thenexusengine.com`) |

### Optional

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `timeout` | number | 2800 | Bid request timeout in milliseconds |
| `debug` | boolean | false | Enable debug logging to console |
| `userSync.enabled` | boolean | true | Enable user ID syncing |
| `userSync.bidders` | array | `['kargo', 'rubicon', 'pubmatic', 'sovrn', 'triplelift']` | Bidders to sync |
| `userSync.syncDelay` | number | 1000 | Delay before syncing (ms) |
| `userSync.maxSyncs` | number | 5 | Max syncs per page load |

## Complete Example

```html
<!DOCTYPE html>
<html>
<head>
    <title>Catalyst SDK Example</title>
</head>
<body>
    <!-- Ad slot -->
    <div id="leaderboard-ad" style="width: 728px; height: 90px;"></div>

    <!-- Load SDK -->
    <script src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>

    <script>
        // Initialize
        catalyst.init({
            accountId: 'my-account-123',
            serverUrl: 'https://ads.thenexusengine.com',
            timeout: 2800,
            debug: true
        });

        // Request bids when page loads
        window.addEventListener('DOMContentLoaded', function() {
            catalyst.requestBids({
                slots: [
                    {
                        divId: 'leaderboard-ad',
                        sizes: [[728, 90]],
                        adUnitPath: '/homepage/leaderboard',
                        position: 'atf'
                    }
                ],
                page: {
                    url: window.location.href,
                    domain: window.location.hostname,
                    keywords: ['news', 'technology'],
                    categories: ['IAB19']  // Technology & Computing
                }
            }, function(bids) {
                if (bids.length === 0) {
                    console.log('No bids received');
                    return;
                }

                // Use highest CPM bid
                var topBid = bids.reduce(function(max, bid) {
                    return bid.cpm > max.cpm ? bid : max;
                }, bids[0]);

                console.log('Top bid CPM:', topBid.cpm);

                // Render ad
                document.getElementById(topBid.divId).innerHTML = topBid.ad;
            });
        });
    </script>
</body>
</html>
```

## Testing

### Local Testing

1. **Open test page:**
   - Download `tests/catalyst_sdk_test.html`
   - Open directly in browser (works as `file://`)
   - All tests should pass

2. **Check console logs:**
   ```
   [Catalyst] SDK initialized with accountId: test-account-123
   [Catalyst] Bid request started (requestId: abc123)
   [Catalyst] Received 5 bids in 245ms
   ```

3. **Run built-in tests:**
   - Test 1: Basic bid request
   - Test 2: Multiple slots
   - Test 3: Timeout handling
   - Test 4: Coordination callback
   - Test 5: Performance test

### Production Testing

```javascript
// Check SDK version
console.log('SDK version:', catalyst.version());

// Enable debug mode
catalyst.init({
    accountId: 'your-account',
    serverUrl: 'https://ads.thenexusengine.com',
    debug: true  // ← Logs all requests/responses
});
```

## API Reference

### Methods

#### `Catalyst.init(config)`
Initialize the SDK. Must be called before requesting bids.

#### `Catalyst.requestBids(params, callback)`
Request bids for ad slots.

**Parameters:**
- `params.slots[]` - Array of ad slot configurations
- `params.page` - Page-level targeting data (optional)
- `callback(bids)` - Called when bids are ready

#### `catalyst.version()`
Returns SDK version string.

### Slot Configuration

```javascript
{
  divId: 'ad-container-id',      // Required: DOM element ID
  sizes: [[728, 90], [970, 250]], // Required: Ad sizes [width, height]
  adUnitPath: '/site/page/slot',  // Optional: GAM ad unit path
  position: 'atf',                // Optional: 'atf' or 'btf'
  targeting: {                    // Optional: Slot-level targeting
    category: 'news',
    keywords: ['politics', 'economy']
  }
}
```

### Bid Response

```javascript
{
  divId: 'ad-container-id',  // Slot ID
  cpm: 2.50,                 // Bid price (CPM)
  currency: 'USD',           // Currency code
  width: 728,                // Creative width
  height: 90,                // Creative height
  ad: '<div>...</div>',      // Ad markup (HTML)
  bidder: 'rubicon',         // Winning bidder
  dealId: 'ABC123'           // Deal ID (if applicable)
}
```

## Support

- **Documentation:** `/docs/CATALYST_SDK_USAGE.md`
- **Test Page:** `/tests/catalyst_sdk_test.html`
- **API Endpoint:** `https://ads.thenexusengine.com/v1/bid`
- **SDK URL:** `https://ads.thenexusengine.com/assets/catalyst-sdk.js`

## Troubleshooting Checklist

- [ ] SDK script loaded from `https://ads.thenexusengine.com/assets/catalyst-sdk.js`
- [ ] `Catalyst.init()` called with `accountId` and `serverUrl`
- [ ] `serverUrl` is set to `https://ads.thenexusengine.com`
- [ ] Ad slot `divId` matches actual DOM element ID
- [ ] Page is served over HTTPS (or HTTP for localhost)
- [ ] Browser console shows no CORS errors
- [ ] Debug mode enabled to see request/response logs
