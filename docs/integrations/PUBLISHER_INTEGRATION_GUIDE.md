# TNE Catalyst — Publisher Integration Guide

Two ways to integrate. Pick the one that fits your stack.

| Method | What It Is | Best For |
|--------|-----------|----------|
| [Method 1: SDK + GAM](#method-1-catalyst-sdk--google-ad-manager) | Drop in our SDK, we run the auction, bids flow into GAM | Publishers who want a managed solution with minimal code |
| [Method 2: Prebid.js S2S](#method-2-prebidjs-server-to-server) | Point your existing Prebid.js at our server | Publishers already running Prebid.js who want server-side speed |

Both methods move the auction off the browser and onto our server. Your pages load faster, Core Web Vitals improve, and you get access to 21+ demand partners without managing individual SSP integrations.

---

## Method 1: Catalyst SDK + Google Ad Manager

**You provide:** Your GAM account, ad unit paths, and (optionally) your own SSP seat credentials.
**We provide:** The SDK, the server-side auction, and managed demand.

### How It Works

```
Page loads → SDK fires → Server runs parallel auction (21+ bidders, ~150ms)
→ Winning bid + targeting keys sent back → SDK sets GAM targeting
→ GAM makes final ad decision → Ad renders
```

### Step 1: Load the SDK

Add this to your page `<head>` or before your GAM tags:

```html
<script async src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>
```

The SDK is < 50KB gzipped and loads asynchronously — it won't block your page.

### Step 2: Initialize

```javascript
var catalyst = catalyst || {};
catalyst.cmd = catalyst.cmd || [];

catalyst.cmd.push(function() {
  catalyst.init({
    accountId: 'YOUR_PUBLISHER_ID',              // We provide this
    serverUrl: 'https://ads.thenexusengine.com',
    timeout: 2500,                                // Server auction timeout (ms)
    debug: false                                  // Set true during integration
  });
});
```

### Step 3: Define Your Ad Slots

Map your ad slots to match your GAM ad units:

```javascript
catalyst.cmd.push(function() {
  catalyst.requestBids({
    slots: [
      {
        divId: 'div-gpt-ad-leaderboard',
        sizes: [[728, 90], [970, 250]],
        adUnitPath: '/12345678/homepage/leaderboard',
        position: 'atf'
      },
      {
        divId: 'div-gpt-ad-sidebar',
        sizes: [[300, 250], [300, 600]],
        adUnitPath: '/12345678/homepage/sidebar',
        position: 'atf'
      },
      {
        divId: 'div-gpt-ad-incontent',
        sizes: [[300, 250], [728, 90]],
        adUnitPath: '/12345678/homepage/incontent',
        position: 'btf'
      }
    ],
    page: {
      url: window.location.href,
      domain: window.location.hostname,
      keywords: ['news', 'technology'],
      categories: ['IAB19']
    }
  }, function(bids) {
    // SDK handles GAM targeting automatically — see Step 4
  });
});
```

### Step 4: GAM Targeting (Automatic)

The SDK automatically sets these targeting keys on your GAM slots:

| GAM Key | Value | Example |
|---------|-------|---------|
| `hb_pb_catalyst` | Bid price (CPM) | `"2.50"` |
| `hb_size_catalyst` | Creative size | `"300x250"` |
| `hb_adid_catalyst` | Bid ID | `"abc123"` |
| `hb_bidder_catalyst` | Winning SSP | `"thenexusengine"` |
| `hb_source_catalyst` | Source type | `"s2s"` |
| `hb_format_catalyst` | Ad format | `"banner"` |
| `hb_deal_catalyst` | Deal ID (if PMP) | `"DEAL-12345"` |

These keys use the `_catalyst` suffix — they **never conflict** with your existing Prebid.js keys. Both can run side by side.

### Step 5: GAM Line Item Setup

Create line items in GAM to catch Catalyst bids:

**1. Create a new Order** named "TNE Catalyst Header Bidding"

**2. Create line items** at each price increment:

| Field | Value |
|-------|-------|
| Type | Price Priority |
| Priority | 12 |
| Rate | Match the targeting price (e.g. $2.50) |
| Targeting | `hb_pb_catalyst = 2.50` |

**3. Attach a 3rd-party creative** to each line item:

```html
<script>
  var w = window;
  for (var i = 0; i < 10; i++) {
    w = w.parent;
    if (w.catalyst) {
      w.catalyst.renderAd(document, '%%PATTERN:hb_adid_catalyst%%');
      break;
    }
  }
</script>
```

**4. Price granularity** — create line items to match your preferred granularity:

| Granularity | Increments | Line Items Needed |
|-------------|-----------|-------------------|
| Low | $0.50 | ~40 (up to $20) |
| Medium | $0.10 | ~200 (up to $20) |
| High | $0.01 | ~2000 (up to $20) |
| Auto | Mixed | ~100 (dense at low, sparse at high) |

> **Tip:** Start with Medium ($0.10 increments). You can always increase later.

### Step 6: Bring Your Own SSP Seats (Optional)

If you have direct relationships with SSPs, send us your seat credentials and we'll configure them on our server for your publisher ID. Your bids, your relationships — we just run the auction faster.

**What to send us:**

| SSP | What We Need |
|-----|-------------|
| Rubicon/Magnite | `accountId`, `siteId`, `zoneId` |
| AppNexus/Xandr | `placementId` or `member` + `invCode` |
| PubMatic | `publisherId`, `adSlot` |
| Sovrn | `tagId` |
| Kargo | `placementId` |
| TripleLift | `inventoryCode` |

Email these to **onboarding@thenexusengine.com** or configure them through the publisher admin dashboard. We support per-domain and per-ad-unit overrides — you can use your Rubicon seat on your main site and our managed seat on your subdomains.

**Or** pass them directly in the bid request (advanced — see [Bidder Params Guide](../guides/BIDDER-PARAMS-GUIDE.md)).

### Complete Page Example

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Site</title>

  <!-- Google Publisher Tag -->
  <script async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js"></script>

  <!-- TNE Catalyst SDK -->
  <script async src="https://ads.thenexusengine.com/assets/catalyst-sdk.js"></script>

  <script>
    // GPT setup
    var googletag = googletag || {};
    googletag.cmd = googletag.cmd || [];

    // Catalyst setup
    var catalyst = catalyst || {};
    catalyst.cmd = catalyst.cmd || [];

    // 1. Define GPT ad slots
    googletag.cmd.push(function() {
      googletag.defineSlot('/12345678/leaderboard', [[728, 90], [970, 250]], 'div-gpt-ad-leaderboard')
        .addService(googletag.pubads());

      googletag.defineSlot('/12345678/sidebar', [[300, 250], [300, 600]], 'div-gpt-ad-sidebar')
        .addService(googletag.pubads());

      googletag.pubads().disableInitialLoad();  // IMPORTANT: Don't load until bids are in
      googletag.pubads().enableSingleRequest();
      googletag.enableServices();
    });

    // 2. Initialize Catalyst and request bids
    catalyst.cmd.push(function() {
      catalyst.init({
        accountId: 'YOUR_PUBLISHER_ID',
        serverUrl: 'https://ads.thenexusengine.com',
        timeout: 2500,
        debug: false
      });

      catalyst.requestBids({
        slots: [
          {
            divId: 'div-gpt-ad-leaderboard',
            sizes: [[728, 90], [970, 250]],
            adUnitPath: '/12345678/leaderboard',
            position: 'atf'
          },
          {
            divId: 'div-gpt-ad-sidebar',
            sizes: [[300, 250], [300, 600]],
            adUnitPath: '/12345678/sidebar',
            position: 'atf'
          }
        ]
      }, function(bids) {
        // 3. Catalyst sets GAM targeting keys automatically

        // 4. Now tell GAM to fetch ads (with Catalyst targeting attached)
        googletag.cmd.push(function() {
          googletag.pubads().refresh();
        });
      });
    });
  </script>
</head>
<body>
  <div id="div-gpt-ad-leaderboard" style="min-height:90px;">
    <script>googletag.cmd.push(function() { googletag.display('div-gpt-ad-leaderboard'); });</script>
  </div>

  <div id="div-gpt-ad-sidebar" style="min-height:250px;">
    <script>googletag.cmd.push(function() { googletag.display('div-gpt-ad-sidebar'); });</script>
  </div>
</body>
</html>
```

### Verifying It Works

Open browser DevTools and check:

**Console (with `debug: true`):**
```
[Catalyst] SDK initialized with accountId: YOUR_PUBLISHER_ID
[Catalyst] Bid request started (requestId: abc123)
[Catalyst] Received 3 bids in 142ms
[Catalyst] Set slot targeting for div-gpt-ad-leaderboard: hb_pb_catalyst=3.20
[Catalyst] Set slot targeting for div-gpt-ad-sidebar: hb_pb_catalyst=1.85
```

**GAM targeting keys (paste in console):**
```javascript
googletag.pubads().getSlots().forEach(function(slot) {
  console.log(slot.getAdUnitPath());
  slot.getTargetingKeys().forEach(function(key) {
    if (key.includes('catalyst')) {
      console.log('  ' + key + ':', slot.getTargeting(key));
    }
  });
});
```

---

## Method 2: Prebid.js Server-to-Server

**You provide:** Your existing Prebid.js setup.
**We provide:** A Prebid Server-compatible endpoint that runs auctions server-side.

This is the right choice if you're already running Prebid.js and want to move some or all of your bidders server-side for faster auctions and lighter pages.

### How It Works

```
Prebid.js (client) → s2sConfig points to Catalyst → Catalyst runs server-side auction
→ OpenRTB responses flow back through Prebid.js → Normal GAM handoff
```

Prebid.js natively supports Server-to-Server (S2S) mode. You configure our endpoint as your Prebid Server, tell it which bidders to run server-side, and Prebid.js handles everything else — targeting, GAM integration, rendering — exactly as it does today.

### Step 1: Add S2S Config to Prebid.js

```javascript
pbjs.setConfig({
  s2sConfig: [{
    accountId: 'YOUR_PUBLISHER_ID',           // We provide this
    adapter: 'prebidServer',
    enabled: true,
    endpoint: {
      p1Consent: 'https://ads.thenexusengine.com/openrtb2/auction',
      noP1Consent: 'https://ads.thenexusengine.com/openrtb2/auction'
    },
    syncEndpoint: {
      p1Consent: 'https://ads.thenexusengine.com/cookie_sync',
      noP1Consent: 'https://ads.thenexusengine.com/cookie_sync'
    },
    bidders: ['rubicon', 'appnexus', 'pubmatic', 'sovrn', 'kargo', 'triplelift'],
    timeout: 1000,                             // Server-side timeout (ms)
    extPrebid: {
      cache: {
        bids: {}                               // Enable server-side bid caching
      }
    }
  }]
});
```

That's the core of it. Prebid.js will now route those bidders through our server instead of making individual client-side calls.

### Step 2: Define Ad Units

Ad units look exactly like standard Prebid.js ad units. No changes needed if you already have them.

**Using TNE managed demand (no bidder params needed):**

```javascript
var adUnits = [
  {
    code: 'div-gpt-ad-leaderboard',
    mediaTypes: {
      banner: {
        sizes: [[728, 90], [970, 250]]
      }
    },
    bids: [
      { bidder: 'rubicon' },
      { bidder: 'appnexus' },
      { bidder: 'pubmatic' }
    ]
  },
  {
    code: 'div-gpt-ad-sidebar',
    mediaTypes: {
      banner: {
        sizes: [[300, 250], [300, 600]]
      }
    },
    bids: [
      { bidder: 'rubicon' },
      { bidder: 'appnexus' },
      { bidder: 'pubmatic' }
    ]
  }
];
```

When you use TNE managed demand, our server already has the bidder credentials configured for your publisher ID. You just list which bidders you want — no `params` block needed.

**Using your own SSP seats (pass your bidder params):**

```javascript
var adUnits = [
  {
    code: 'div-gpt-ad-leaderboard',
    mediaTypes: {
      banner: {
        sizes: [[728, 90], [970, 250]]
      }
    },
    bids: [
      {
        bidder: 'rubicon',
        params: {
          accountId: 26298,
          siteId: 556630,
          zoneId: 3767186
        }
      },
      {
        bidder: 'appnexus',
        params: {
          placementId: 13232354
        }
      },
      {
        bidder: 'pubmatic',
        params: {
          publisherId: '156209',
          adSlot: 'leaderboard@728x90'
        }
      }
    ]
  }
];
```

**Hybrid — mix your seats with our managed demand:**

```javascript
bids: [
  {
    bidder: 'rubicon',
    params: {                        // Your direct Rubicon seat
      accountId: 26298,
      siteId: 556630,
      zoneId: 3767186
    }
  },
  { bidder: 'appnexus' },           // TNE managed — no params needed
  { bidder: 'kargo' },              // TNE managed
  { bidder: 'sovrn' }               // TNE managed
]
```

### Step 3: Cookie Sync

Prebid.js handles this automatically when you set the `syncEndpoint` in your s2sConfig. For additional control:

```javascript
pbjs.setConfig({
  userSync: {
    syncEnabled: true,
    syncsPerBidder: 5,
    syncDelay: 3000,
    filterSettings: {
      iframe: { bidders: '*', filter: 'include' },
      image: { bidders: '*', filter: 'include' }
    }
  }
});
```

### Step 4: Privacy / Consent

If you run a CMP (Consent Management Platform), Prebid.js will automatically forward consent signals to our server. No extra configuration needed.

**GDPR (TCF 2.0):**
```javascript
// Prebid.js picks this up from your CMP automatically via the __tcfapi
// Our server validates consent per-bidder using IAB Global Vendor List IDs
// In strict mode: invalid consent = no auction (400 response)
// In permissive mode: invalid consent = PII stripped, auction continues
```

**CCPA (US Privacy):**
```javascript
// Prebid.js reads the USP string from your __uspapi
// Our server enforces "Do Not Sell" signals
```

**COPPA:**
```javascript
pbjs.setConfig({
  coppa: true  // If your site has child-directed content
});
```

### Step 5: GAM Integration

This is standard Prebid.js → GAM. If you already have this working, nothing changes.

```javascript
pbjs.que.push(function() {
  pbjs.addAdUnits(adUnits);

  pbjs.requestBids({
    bidsBackHandler: function() {
      googletag.cmd.push(function() {
        pbjs.setTargetingForGPTAsync();
        googletag.pubads().refresh();
      });
    }
  });
});
```

### Complete Page Example

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Site — Prebid.js + Catalyst S2S</title>

  <!-- GPT -->
  <script async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js"></script>

  <!-- Prebid.js (your existing build) -->
  <script async src="/js/prebid.js"></script>

  <script>
    var googletag = googletag || {};
    googletag.cmd = googletag.cmd || [];
    var pbjs = pbjs || {};
    pbjs.que = pbjs.que || [];

    // ---- PREBID CONFIG ----
    pbjs.que.push(function() {

      // Point Prebid.js at Catalyst for server-side bidding
      pbjs.setConfig({
        s2sConfig: [{
          accountId: 'YOUR_PUBLISHER_ID',
          adapter: 'prebidServer',
          enabled: true,
          endpoint: {
            p1Consent: 'https://ads.thenexusengine.com/openrtb2/auction',
            noP1Consent: 'https://ads.thenexusengine.com/openrtb2/auction'
          },
          syncEndpoint: {
            p1Consent: 'https://ads.thenexusengine.com/cookie_sync',
            noP1Consent: 'https://ads.thenexusengine.com/cookie_sync'
          },
          bidders: ['rubicon', 'appnexus', 'pubmatic', 'sovrn', 'kargo', 'triplelift'],
          timeout: 1000
        }],
        userSync: {
          syncEnabled: true,
          syncsPerBidder: 5,
          syncDelay: 3000
        }
      });

      // Define ad units — your seats or managed demand
      var adUnits = [
        {
          code: 'div-gpt-ad-leaderboard',
          mediaTypes: { banner: { sizes: [[728, 90], [970, 250]] } },
          bids: [
            {
              bidder: 'rubicon',
              params: { accountId: 26298, siteId: 556630, zoneId: 3767186 }
            },
            { bidder: 'appnexus' },     // Managed by TNE
            { bidder: 'pubmatic' },      // Managed by TNE
            { bidder: 'sovrn' }          // Managed by TNE
          ]
        },
        {
          code: 'div-gpt-ad-sidebar',
          mediaTypes: { banner: { sizes: [[300, 250], [300, 600]] } },
          bids: [
            {
              bidder: 'rubicon',
              params: { accountId: 26298, siteId: 556630, zoneId: 4567890 }
            },
            { bidder: 'appnexus' },
            { bidder: 'kargo' },
            { bidder: 'triplelift' }
          ]
        }
      ];

      pbjs.addAdUnits(adUnits);

      // Request bids and hand off to GAM
      pbjs.requestBids({
        bidsBackHandler: function() {
          googletag.cmd.push(function() {
            pbjs.setTargetingForGPTAsync();
            googletag.pubads().refresh();
          });
        }
      });
    });

    // ---- GAM CONFIG ----
    googletag.cmd.push(function() {
      googletag.defineSlot('/12345678/leaderboard', [[728, 90], [970, 250]], 'div-gpt-ad-leaderboard')
        .addService(googletag.pubads());
      googletag.defineSlot('/12345678/sidebar', [[300, 250], [300, 600]], 'div-gpt-ad-sidebar')
        .addService(googletag.pubads());

      googletag.pubads().disableInitialLoad();
      googletag.pubads().enableSingleRequest();
      googletag.enableServices();
    });
  </script>
</head>
<body>
  <div id="div-gpt-ad-leaderboard">
    <script>googletag.cmd.push(function() { googletag.display('div-gpt-ad-leaderboard'); });</script>
  </div>

  <div id="div-gpt-ad-sidebar">
    <script>googletag.cmd.push(function() { googletag.display('div-gpt-ad-sidebar'); });</script>
  </div>
</body>
</html>
```

### Hybrid: Keep Some Bidders Client-Side

You don't have to move everything server-side. Run latency-sensitive bidders on the client and the rest through Catalyst:

```javascript
pbjs.setConfig({
  s2sConfig: [{
    accountId: 'YOUR_PUBLISHER_ID',
    adapter: 'prebidServer',
    enabled: true,
    endpoint: {
      p1Consent: 'https://ads.thenexusengine.com/openrtb2/auction',
      noP1Consent: 'https://ads.thenexusengine.com/openrtb2/auction'
    },
    syncEndpoint: {
      p1Consent: 'https://ads.thenexusengine.com/cookie_sync',
      noP1Consent: 'https://ads.thenexusengine.com/cookie_sync'
    },
    // Only these bidders go server-side
    bidders: ['rubicon', 'pubmatic', 'sovrn', 'kargo', 'triplelift'],
    timeout: 1000
  }]
});

var adUnits = [{
  code: 'div-gpt-ad-leaderboard',
  mediaTypes: { banner: { sizes: [[728, 90]] } },
  bids: [
    // These run SERVER-SIDE through Catalyst (listed in s2sConfig.bidders)
    { bidder: 'rubicon', params: { accountId: 26298, siteId: 556630, zoneId: 3767186 } },
    { bidder: 'pubmatic' },
    { bidder: 'sovrn' },

    // These run CLIENT-SIDE in the browser (NOT in s2sConfig.bidders)
    { bidder: 'amazon', params: { ... } },
    { bidder: 'ix', params: { siteId: '123456' } }
  ]
}];
```

Prebid.js runs both paths in parallel and merges the results before sending to GAM.

### Verifying It Works

**Prebid.js debug mode:**

Add `?pbjs_debug=true` to your page URL, then check the browser console:

```
Prebid.js: s2sConfig bidder rubicon          → server
Prebid.js: s2sConfig bidder appnexus         → server
Prebid.js: non-s2s bidder amazon             → client
Prebid.js: Bid received: rubicon 728x90 $3.20
Prebid.js: Bid received: appnexus 728x90 $2.85
Prebid.js: Auction completed in 245ms
```

**Network tab:** Look for a single POST to `ads.thenexusengine.com/openrtb2/auction` — that's your server-side auction replacing multiple individual bidder calls.

**Server-side debug:** Add `?debug=1` to the endpoint for verbose response data:

```javascript
endpoint: {
  p1Consent: 'https://ads.thenexusengine.com/openrtb2/auction?debug=1'
}
```

The response `ext` will include per-bidder response times and any errors:
```json
{
  "ext": {
    "responsetimemillis": { "rubicon": 45, "appnexus": 52, "pubmatic": 38 },
    "errors": {}
  }
}
```

---

## Supported Bidders

These bidders are available through Catalyst. Use them in either integration method.

| Bidder | Code | Required Params (if bringing your own seat) |
|--------|------|---------------------------------------------|
| Rubicon/Magnite | `rubicon` | `accountId` (int), `siteId` (int), `zoneId` (int) |
| AppNexus/Xandr | `appnexus` | `placementId` (int) |
| PubMatic | `pubmatic` | `publisherId` (string), `adSlot` (string) |
| Sovrn | `sovrn` | `tagId` (string) |
| Kargo | `kargo` | `placementId` (string) |
| TripleLift | `triplelift` | `inventoryCode` (string) |
| GumGum | `gumgum` | See bidder docs |
| Improve Digital | `improvedigital` | See bidder docs |
| Medianet | `medianet` | See bidder docs |
| SmartAdServer | `smartadserver` | See bidder docs |
| Conversant | `conversant` | See bidder docs |
| Beachfront | `beachfront` | See bidder docs |
| Adform | `adform` | See bidder docs |
| OneTag | `onetag` | See bidder docs |
| Criteo | `criteo` | See bidder docs |
| OpenX | `openx` | See bidder docs |
| Sharethrough | `sharethrough` | See bidder docs |
| Outbrain | `outbrain` | See bidder docs |
| Taboola | `taboola` | See bidder docs |
| Teads | `teads` | See bidder docs |
| Unruly | `unruly` | See bidder docs |

For full parameter reference, see [Bidder Params Guide](../guides/BIDDER-PARAMS-GUIDE.md).

---

## Which Method Should I Choose?

| | Method 1: SDK + GAM | Method 2: Prebid.js S2S |
|---|---|---|
| **Already using Prebid.js?** | Works alongside it | Plugs directly into it |
| **Starting from scratch?** | Fastest to integrate | Need Prebid.js setup first |
| **Want managed demand?** | Default — we handle SSP relationships | Supported — omit `params` |
| **Bringing your own SSP seats?** | Email us credentials or use dashboard | Pass `params` in ad units |
| **GAM setup required?** | Yes — line items with `_catalyst` keys | Yes — standard Prebid GAM setup |
| **Client-side JS weight** | ~50KB (Catalyst SDK) | 0KB extra (uses your existing Prebid.js) |
| **Video support** | Via `/video/vast` endpoint | Via Prebid.js video modules |

---

## Getting Started

1. **Contact us** — onboarding@thenexusengine.com — to get your Publisher ID
2. **Choose your method** — SDK + GAM or Prebid.js S2S
3. **Send us your SSP credentials** (optional) — or use our managed demand
4. **Integrate** using the examples above
5. **Test** with `debug: true` (Method 1) or `?pbjs_debug=true` (Method 2)
6. **Go live** — flip `debug` off and monitor via your GAM reports

---

## Support

- **Onboarding:** onboarding@thenexusengine.com
- **Technical:** support@thenexusengine.com
- **Dashboard:** https://ads.thenexusengine.com/admin/dashboard
- **API Reference:** [API docs](../api/API-REFERENCE.md)
- **Bidder Params:** [Bidder Params Guide](../guides/BIDDER-PARAMS-GUIDE.md)
- **GAM Keys Reference:** [GAM Targeting Keys](../GAM_TARGETING_KEYS.md)
