# Video VAST Integration - Setup Guide

## Prerequisites

- [ ] Publisher account and credentials
- [ ] Video player on your site/app
- [ ] HTTPS website/app
- [ ] Basic understanding of VAST tags

## Step 1: Get Credentials

### 1.1 Request Access

Contact: video-sales@tne-catalyst.com

Provide:
- Domain or app name
- Expected video impressions/month
- Video inventory type (pre-roll, mid-roll, etc.)
- Platforms (web, mobile, CTV)

### 1.2 Receive Credentials

You'll receive:
```
Publisher ID: pub-video-123456
API Key: tne_live_video_abc123 (optional, for POST endpoint)
Test Publisher ID: pub-video-test
```

## Step 2: Choose Integration Method

### Option A: Query Parameters (Easiest)

Best for: Simple integrations, quick testing

```javascript
const vastUrl =
  'https://ads.thenexusengine.com/video/vast?' +
  'pub_id=pub-video-123456&' +
  'w=1920&h=1080&' +
  'mindur=5&maxdur=30&' +
  'mimes=video/mp4&' +
  'bidfloor=3.0';
```

### Option B: POST OpenRTB (Advanced)

Best for: Advanced targeting, complex scenarios

```javascript
const response = await fetch(
  'https://ads.thenexusengine.com/video/openrtb',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': 'tne_live_video_abc123'
    },
    body: JSON.stringify({
      id: generateRequestId(),
      imp: [{
        id: '1',
        video: {
          w: 1920,
          h: 1080,
          minduration: 5,
          maxduration: 30,
          mimes: ['video/mp4', 'video/webm'],
          protocols: [2, 3, 5, 6],
          placement: 1,
          linearity: 1,
          skip: 1,
          skipafter: 5
        },
        bidfloor: 3.0,
        bidfloorcur: 'USD'
      }],
      site: {
        id: 'pub-video-123456',
        domain: 'yoursite.com',
        page: window.location.href
      },
      device: {
        ua: navigator.userAgent,
        ip: '', // Server will detect
        devicetype: detectDeviceType(),
        w: screen.width,
        h: screen.height
      },
      user: {
        id: getUserId() // Your user ID
      }
    })
  }
);

const vastXml = await response.text();
```

## Step 3: Video Player Integration

### Google IMA SDK (Recommended)

```html
<!DOCTYPE html>
<html>
<head>
  <script src="//imasdk.googleapis.com/js/sdkloader/ima3.js"></script>
  <style>
    #video-container { width: 640px; height: 360px; }
    #video { width: 100%; height: 100%; }
    #ad-container { position: absolute; top: 0; left: 0; }
  </style>
</head>
<body>
  <div id="video-container">
    <video id="video" controls>
      <source src="content.mp4" type="video/mp4">
    </video>
    <div id="ad-container"></div>
  </div>

  <script>
    // IMA SDK Setup
    const videoElement = document.getElementById('video');
    const adContainer = document.getElementById('ad-container');

    const adDisplayContainer = new google.ima.AdDisplayContainer(
      adContainer,
      videoElement
    );

    const adsLoader = new google.ima.AdsLoader(adDisplayContainer);
    const adsManager;

    // Build VAST URL
    const vastUrl = buildVastUrl({
      pubId: 'pub-video-123456',
      width: 640,
      height: 360,
      minDuration: 5,
      maxDuration: 30,
      mimes: 'video/mp4'
    });

    function buildVastUrl(params) {
      const base = 'https://ads.thenexusengine.com/video/vast';
      const query = new URLSearchParams({
        pub_id: params.pubId,
        w: params.width,
        h: params.height,
        mindur: params.minDuration,
        maxdur: params.maxDuration,
        mimes: params.mimes
      });
      return `${base}?${query}`;
    }

    // Request ads
    function requestAds() {
      adDisplayContainer.initialize();

      const adsRequest = new google.ima.AdsRequest();
      adsRequest.adTagUrl = vastUrl;
      adsRequest.linearAdSlotWidth = 640;
      adsRequest.linearAdSlotHeight = 360;

      adsLoader.requestAds(adsRequest);
    }

    // Event listeners
    adsLoader.addEventListener(
      google.ima.AdsManagerLoadedEvent.Type.ADS_MANAGER_LOADED,
      onAdsManagerLoaded
    );

    adsLoader.addEventListener(
      google.ima.AdErrorEvent.Type.AD_ERROR,
      onAdError
    );

    function onAdsManagerLoaded(event) {
      adsManager = event.getAdsManager(videoElement);
      adsManager.init(640, 360, google.ima.ViewMode.NORMAL);
      adsManager.start();
    }

    function onAdError(event) {
      console.error('Ad error:', event.getError());
      videoElement.play(); // Play content on error
    }

    // Start when user clicks play
    videoElement.addEventListener('click', requestAds);
  </script>
</body>
</html>
```

### Video.js Integration

```html
<!DOCTYPE html>
<html>
<head>
  <link href="//vjs.zencdn.net/7.20.3/video-js.css" rel="stylesheet">
  <script src="//vjs.zencdn.net/7.20.3/video.min.js"></script>
  <script src="//path/to/videojs-ima.min.js"></script>
</head>
<body>
  <video id="video" class="video-js" controls>
    <source src="content.mp4" type="video/mp4">
  </video>

  <script>
    const player = videojs('video');

    player.ima({
      adTagUrl: 'https://ads.thenexusengine.com/video/vast?' +
        'pub_id=pub-video-123456&w=640&h=360&' +
        'mindur=5&maxdur=30&mimes=video/mp4'
    });
  </script>
</body>
</html>
```

### JW Player Integration

```html
<div id="player"></div>
<script src="//cdn.jwplayer.com/libraries/PLAYER_KEY.js"></script>
<script>
  jwplayer('player').setup({
    file: 'content.mp4',
    advertising: {
      client: 'vast',
      tag: 'https://ads.thenexusengine.com/video/vast?' +
        'pub_id=pub-video-123456&w=1280&h=720&' +
        'mindur=5&maxdur=30&mimes=video/mp4',
      vpaidmode: 'insecure'
    }
  });
</script>
```

## Step 4: Configure Video Parameters

### 4.1 Video Dimensions

Match your player size:
```javascript
// Desktop
w=1920&h=1080  // Full HD
w=1280&h=720   // HD

// Mobile
w=640&h=360    // Mobile landscape
w=360&h=640    // Mobile portrait

// CTV
w=3840&h=2160  // 4K
w=1920&h=1080  // Full HD
```

### 4.2 Duration Constraints

```javascript
// Pre-roll (short ads)
mindur=5&maxdur=15

// Mid-roll (standard)
mindur=15&maxdur=30

// Long-form content
mindur=30&maxdur=60
```

### 4.3 MIME Types

```javascript
// Standard web
mimes=video/mp4,video/webm

// Mobile optimized
mimes=video/mp4

// CTV optimized
mimes=video/mp4,application/x-mpegURL  // Includes HLS
```

### 4.4 Protocols (OpenRTB POST only)

```javascript
protocols: [
  2,  // VAST 2.0
  3,  // VAST 3.0
  5,  // VAST 2.0 wrapper
  6,  // VAST 3.0 wrapper
  7,  // VAST 4.0
  8   // VAST 4.0 wrapper
]
```

### 4.5 Placement Types

```javascript
placement: 1  // In-stream (pre/mid/post-roll)
placement: 2  // In-banner
placement: 3  // In-article
placement: 4  // In-feed
placement: 5  // Interstitial/slider
```

## Step 4.6: Google Ad Manager Direct Integration

For publishers serving video via Google Ad Manager, Catalyst can be trafficked
as a **Third Party VAST Redirect** creative. GAM substitutes its macros
server-side at ad call time, so every enrichment (page, referrer, ad unit,
content KVs, privacy signals, IFA) lands on the Catalyst endpoint populated.

### Ready-to-paste VAST tag

This template lives in the repo so The Nexus Engine team can bake in
`{GVL_ID}` (our own IAB TCF vendor ID) before emailing the tag to the
publisher &mdash; the publisher never has to look it up.

**Publisher fills in:** `{PUBLISHER_ID}`, `{MIN}`/`{MAX}` duration, and
`{FLOOR}` (or drop the `bidfloor=` pair to disable).

**The Nexus Engine fills in before delivery:** `{GVL_ID}` in the
`%%GDPR_CONSENT_{GVL_ID}%%` macro. The macro needs a concrete numeric
vendor ID for GAM to emit the signed TCF string &mdash; GAM returns empty
if the GVL is unregistered.

```
https://ads.thenexusengine.com/video/vast?pub_id={PUBLISHER_ID}&w=%%WIDTH%%&h=%%HEIGHT%%&mindur={MIN}&maxdur={MAX}&mimes=video/mp4,application/x-mpegURL&protocols=2,3,5,6,7,8&placement=1&linearity=1&bidfloor={FLOOR}&placement_id=%%ADUNIT%%&page_url=%%PAGE_URL%%&domain=%%SITE_DOMAIN%%&ref=%%REFERRER_URL%%&description_url=%%DESCRIPTION_URL%%&cb=%%CACHEBUSTER%%&sport=%%PATTERN:sport%%&competition=%%PATTERN:competition%%&lang=%%PATTERN:language%%&device_type=%%PATTERN:device%%&geo=%%PATTERN:geo%%&content_type=%%PATTERN:content_type%%&gdpr=%%GDPR%%&gdpr_consent=%%GDPR_CONSENT_{GVL_ID}%%&addtl_consent=%%ADDTL_CONSENT%%&us_privacy=%%US_PRIVACY%%&gpp=%%GPP_STRING%%&gpp_sid=%%GPP_SID%%&coppa=%%TFCD%%&ifa=%%ADVERTISING_IDENTIFIER_PLAIN%%&ifa_type=%%ADVERTISING_IDENTIFIER_TYPE%%&lmt=%%LIMITADTRACKING%%&ua=%%USER_AGENT_ESC%%&session_id=%%CLICK_ID%%&schain=%%SCHAIN%%
```

A formatted version with copy-button and full macro reference lives at
[`examples/gam-vast-tag.html`](../../../examples/gam-vast-tag.html); a
plaintext copy is at [`examples/gam-vast-tag.txt`](../../../examples/gam-vast-tag.txt).

### Macro -> OpenRTB 2.5 mapping

| Enrichment | GAM macro | Catalyst param | OpenRTB field |
|---|---|---|---|
| Page URL | `%%PAGE_URL%%` | `page_url` | `site.page` |
| Domain | `%%SITE_DOMAIN%%` | `domain` | `site.domain` |
| Referrer | `%%REFERRER_URL%%` | `ref` | `site.ref` |
| Description URL | `%%DESCRIPTION_URL%%` | `description_url` | `site.content.url` |
| Cache buster | `%%CACHEBUSTER%%` | `cb` | *(not carried; forces fresh fetch)* |
| Ad unit / placement | `%%ADUNIT%%` | `placement_id` | `imp[0].tagid` |
| Player width / height | `%%WIDTH%%` / `%%HEIGHT%%` | `w` / `h` | `imp[0].video.w` / `.h` |
| User agent | `%%USER_AGENT_ESC%%` | `ua` | `device.ua` |
| KV `sport` | `%%PATTERN:sport%%` | `sport` | `site.content.genre` + `imp.ext.context.sport` |
| KV `competition` | `%%PATTERN:competition%%` | `competition` | `site.content.series` + `imp.ext.context.competition` |
| KV `language` | `%%PATTERN:language%%` | `lang` | `site.content.language` (BCP-47) |
| KV `device` | `%%PATTERN:device%%` | `device_type` | `device.devicetype` |
| KV `geo` | `%%PATTERN:geo%%` | `geo` | `device.geo.country` (ISO 3166-1 alpha-3) |
| KV `content_type` | `%%PATTERN:content_type%%` | `content_type` | `site.content.cattax` + `site.content.cat[]` |
| GDPR applies | `%%GDPR%%` | `gdpr` | `regs.ext.gdpr` |
| TCF v2 consent | `%%GDPR_CONSENT_{GVL_ID}%%` | `gdpr_consent` | `user.ext.consent` |
| Google AC string | `%%ADDTL_CONSENT%%` | `addtl_consent` | `user.ext.ConsentedProvidersSettings.consented_providers` |
| US Privacy (CCPA) | `%%US_PRIVACY%%` | `us_privacy` | `regs.ext.us_privacy` |
| GPP string | `%%GPP_STRING%%` | `gpp` | `regs.gpp` |
| GPP section IDs | `%%GPP_SID%%` | `gpp_sid` | `regs.gpp_sid[]` |
| COPPA | `%%TFCD%%` | `coppa` | `regs.coppa` |
| Limit Ad Tracking | `%%LIMITADTRACKING%%` | `lmt` | `device.lmt` |
| IFA (IDFA/AAID/RIDA/TIFA) | `%%ADVERTISING_IDENTIFIER_PLAIN%%` | `ifa` | `device.ifa` |
| IFA type | `%%ADVERTISING_IDENTIFIER_TYPE%%` | `ifa_type` | `device.ext.ifa_type` |
| Supply chain | `%%SCHAIN%%` | `schain` | `source.ext.schain` |

### First-party identifier strategy (future state)

The GET tag above carries publisher enrichments, but deterministic IDs
(UID 2.0, ID5, LiveRamp RampID, SharedID, hashed PPID) should be sent as
structured `user.ext.eids[]` objects. Switch to `POST /video/openrtb` once
you're ready to pass them:

```json
{
  "user": {
    "ext": {
      "eids": [
        { "source": "uidapi.com",   "uids": [{ "id": "<UID2 token>", "atype": 3 }] },
        { "source": "id5-sync.com", "uids": [{ "id": "<ID5 ID>",     "atype": 1, "ext": { "linkType": 2 } }] },
        { "source": "yoursite.com", "uids": [{ "id": "<hashed PPID>","atype": 1, "ext": { "stype": "ppuid" } }] }
      ]
    }
  }
}
```

| Source | `eids[].source` | Notes |
|---|---|---|
| UID 2.0 | `uidapi.com` | Token rotates ~30 days; refresh server-side |
| ID5 | `id5-sync.com` | Populate `ext.linkType` (0/1/2) |
| LiveRamp RampID | `liveramp.com` | ATS envelope, `ext.rtiPartner=idl` |
| SharedID | `pubcid.org` | First-party cookie; region-safe |
| Publisher PPID | `{your-domain}` | Hashed + salted internal user ID; `ext.stype=ppuid` |

### Trafficking checklist (GAM UI)

1. Delivery -> Orders -> create/select order.
2. New line item, type **Price priority** (or **Sponsorship** for guaranteed).
3. Creative type **Video**, format **VAST redirect**.
4. Paste the URL above into **VAST Tag URL**.
5. Leave **"Convert to XML"** off - Catalyst already returns VAST XML.
6. Save, approve, traffic. The Nexus Engine team validates enrichment
   coverage after the first ~1,000 bid requests land.

## Step 5: Privacy Compliance

### 5.1 GDPR (EU Traffic)

Include consent string:

```javascript
// Query parameter method
vastUrl += '&gdpr=1&gdpr_consent=' + encodeURIComponent(consentString);

// POST method
{
  user: {
    ext: { consent: consentString }
  },
  regs: {
    ext: { gdpr: 1 }
  }
}
```

### 5.2 CCPA (US Traffic)

```javascript
// Query parameter method
vastUrl += '&us_privacy=' + encodeURIComponent(privacyString);

// POST method
{
  regs: {
    ext: { us_privacy: '1YNN' }
  }
}
```

### 5.3 COPPA (Children)

```javascript
// Query parameter method
vastUrl += '&coppa=1';

// POST method
{
  regs: { coppa: 1 }
}
```

## Step 6: CTV/OTT Optimization

### Roku Integration

```brightscript
' BrightScript for Roku
function loadVideoAd() as void
    vastUrl = "https://ads.thenexusengine.com/video/vast?" +
              "pub_id=pub-video-123456&" +
              "w=1920&h=1080&" +
              "mindur=15&maxdur=30&" +
              "mimes=video/mp4"

    adPod = CreateObject("roSGNode", "ContentNode")
    adPod.url = vastUrl

    m.video.content = adPod
    m.video.control = "play"
end function
```

### Fire TV (Android)

```java
// Android Java
String vastUrl = "https://ads.thenexusengine.com/video/vast?" +
    "pub_id=pub-video-123456&" +
    "w=1920&h=1080&" +
    "mindur=15&maxdur=30&" +
    "mimes=video/mp4";

AdsLoader.Builder adsLoaderBuilder = new AdsLoader.Builder(context, vastUrl);
adsLoader = adsLoaderBuilder.build();
```

### Apple TV (tvOS)

```swift
// Swift for tvOS
let vastUrl = "https://ads.thenexusengine.com/video/vast?" +
    "pub_id=pub-video-123456&" +
    "w=1920&h=1080&" +
    "mindur=15&maxdur=30&" +
    "mimes=video/mp4,application/x-mpegURL"

let adsLoader = IMAAdsLoader(settings: nil)
let request = IMAAdsRequest(adTagUrl: vastUrl)
adsLoader?.requestAds(with: request)
```

## Step 7: Testing

### 7.1 Test VAST Response

```bash
# Get VAST XML
curl "https://test.tne-catalyst.com/video/vast?\
pub_id=pub-video-test&\
w=1920&h=1080&\
mindur=5&maxdur=30&\
mimes=video/mp4" > test-vast.xml

# Validate XML
xmllint --noout test-vast.xml && echo "✅ Valid XML"

# Check for required elements
grep -q "<VAST" test-vast.xml && echo "✅ VAST root element"
grep -q "<MediaFile" test-vast.xml && echo "✅ Media file found"
grep -q "<Impression" test-vast.xml && echo "✅ Tracking found"
```

### 7.2 Test with VAST Inspector

1. Go to: https://googleads.github.io/googleads-ima-html5/vsi/
2. Paste your VAST URL
3. Click "Test Ad"
4. Verify:
   - [ ] Ad loads successfully
   - [ ] Correct dimensions
   - [ ] Tracking fires
   - [ ] No errors

### 7.3 Test on Real Device

- [ ] Desktop Chrome
- [ ] Desktop Safari
- [ ] Mobile iOS Safari
- [ ] Mobile Android Chrome
- [ ] Roku (if applicable)
- [ ] Fire TV (if applicable)

## Step 8: Monitoring

### 8.1 Track Events

Events are automatically tracked. View in dashboard:
- Impressions
- Start rate
- Completion rate
- Click-through rate
- Quartile tracking

### 8.2 Monitor Performance

```javascript
// Add timing measurement
const startTime = performance.now();
fetch(vastUrl)
  .then(response => {
    const loadTime = performance.now() - startTime;
    console.log(`VAST load time: ${loadTime}ms`);
  });
```

## Step 9: Go Live

### 9.1 Pre-Launch Checklist

- [ ] Tested on all target platforms
- [ ] Privacy compliance implemented
- [ ] Floor prices configured
- [ ] Ad player error handling works
- [ ] Tracking verified in dashboard
- [ ] No console errors

### 9.2 Switch to Production

```javascript
// Change from test to production
const VAST_ENDPOINT =
  process.env.NODE_ENV === 'production'
    ? 'https://ads.thenexusengine.com/video/vast'
    : 'https://test.tne-catalyst.com/video/vast';

const PUBLISHER_ID =
  process.env.NODE_ENV === 'production'
    ? 'pub-video-123456'
    : 'pub-video-test';
```

### 9.3 Gradual Rollout

1. Start with 10% of traffic
2. Monitor for 24 hours
3. Increase to 50%
4. Monitor for 24 hours
5. Increase to 100%

## Troubleshooting

### No Ads Showing

1. Check VAST URL is correct
2. Verify publisher ID
3. Check ad player console for errors
4. Test VAST URL in browser
5. Verify floor price isn't too high

### Ads Load But Don't Play

1. Check MIME type support
2. Verify VPAID compatibility (disable for CTV)
3. Check ad duration matches constraints
4. Verify player initialization

### Tracking Not Working

1. Check browser console for blocked requests
2. Verify no ad blockers active
3. Test tracking URLs manually
4. Check CORS headers

## Advanced Features

### Companion Ads

Request companion banner ads:

```javascript
// POST method only
{
  imp: [{
    video: { /* video params */ },
    companionad: [{
      w: 300,
      h: 250
    }]
  }]
}
```

### Skippable Ads

Configure skip offset:

```javascript
// POST method
{
  video: {
    skip: 1,        // Enable skip
    skipafter: 5    // Skip after 5 seconds
  }
}
```

## Support

- **Technical Questions**: video-support@tne-catalyst.com
- **Account Issues**: account-manager@tne-catalyst.com
- **Documentation**: [VIDEO_E2E_COMPLETE.md](../../video/VIDEO_E2E_COMPLETE.md)

---

**Next:** Review [Best Practices](./BEST-PRACTICES.md) for optimization
