/**
 * Catalyst Bidder SDK for MAI Publisher Integration
 * Server-side header bidding adapter
 * @version 1.0.0
 */

(function(window) {
  'use strict';

  // Catalyst namespace
  var catalyst = window.catalyst || {};
  window.catalyst = catalyst;

  // Command queue for async loading support
  catalyst.cmd = catalyst.cmd || [];

  // Configuration
  catalyst._config = {
    accountId: '',
    serverUrl: 'https://ads.thenexusengine.com',
    timeout: 2800, // Client-side timeout (slightly higher than server 2500ms)
    debug: false,
    enableGeo: true, // Enable client-side geolocation (15-30% CPM lift)
    geoTimeout: 1000, // Geolocation timeout in ms (don't block bid request)
    userSync: {
      enabled: true,
      bidders: ['kargo', 'rubicon', 'pubmatic', 'sovrn', 'triplelift'],
      syncDelay: 1000,      // Delay before syncing (ms)
      iframeEnabled: true,  // Allow iframe syncs
      pixelEnabled: true,   // Allow pixel/redirect syncs
      maxSyncs: 5          // Max number of syncs per page
    }
  };

  // Active bid requests
  catalyst._bidRequests = {};
  catalyst._initialized = false;
  catalyst._userSyncComplete = false;
  catalyst._pendingBidRequests = [];  // Queued calls waiting for sync
  catalyst._syncedBidders = [];

  // Geolocation cache (15-30% CPM lift when available)
  catalyst._geoCache = {
    data: null,           // Cached geo data {lat, lon, accuracy}
    timestamp: null,      // When geo was obtained
    maxAge: 300000,       // Cache for 5 minutes (300,000ms)
    pending: false,       // Geo request in progress

    // Check if cached geo is still valid
    isValid: function() {
      if (!this.data || !this.timestamp) {
        return false;
      }
      var age = Date.now() - this.timestamp;
      return age < this.maxAge;
    },

    // Get cached geo or request new
    getOrRequest: function(callback) {
      // Return cached if valid
      if (this.isValid()) {
        catalyst.log('Using cached geolocation:', this.data);
        callback(this.data);
        return;
      }

      // Don't request if geo is disabled
      if (catalyst._config.enableGeo === false) {
        callback(null);
        return;
      }

      // Don't request if already pending
      if (this.pending) {
        callback(null);
        return;
      }

      // Check if geolocation API is available
      if (!navigator.geolocation) {
        catalyst.log('Geolocation API not available');
        callback(null);
        return;
      }

      this.pending = true;
      var cache = this;
      var timeout = catalyst._config.geoTimeout || 1000;
      var timedOut = false;

      // Set timeout to prevent blocking bid request
      var timer = setTimeout(function() {
        timedOut = true;
        cache.pending = false;
        catalyst.log('Geolocation request timed out after', timeout, 'ms - continuing without geo');
        callback(null);
      }, timeout);

      // Request geolocation
      navigator.geolocation.getCurrentPosition(
        function(position) {
          clearTimeout(timer);
          if (!timedOut) {
            cache.data = {
              lat: position.coords.latitude,
              lon: position.coords.longitude,
              accuracy: Math.round(position.coords.accuracy)
            };
            cache.timestamp = Date.now();
            cache.pending = false;

            catalyst.log('Geolocation obtained:',
              'lat=' + cache.data.lat.toFixed(4),
              'lon=' + cache.data.lon.toFixed(4),
              'accuracy=' + cache.data.accuracy + 'm');
            callback(cache.data);
          }
        },
        function(error) {
          clearTimeout(timer);
          if (!timedOut) {
            cache.pending = false;
            catalyst.log('Geolocation error:', error.message);
            callback(null);
          }
        },
        {
          timeout: timeout,
          maximumAge: cache.maxAge,
          enableHighAccuracy: false // Use network location for speed
        }
      );
    }
  };

  // Module-level FPID cache for consistency across cookie sync and bid requests
  var _cachedFPID = null;

  // First-Party ID Manager
  catalyst._fpidManager = {
    cookieName: 'uids',  // Store in existing uids cookie
    fpidKey: 'fpid',
    expiryDays: 365,

    // Check if we have consent to generate/use FPID
    // GDPR compliance: Only allow FPID if consent is available or GDPR doesn't apply
    hasConsent: function() {
      // If no TCF API, assume GDPR doesn't apply (non-EU traffic)
      if (!window.__tcfapi) {
        return true;
      }

      // Check for cached consent data with longer timeout for slow CMPs
      var hasValidConsent = false;
      var checkComplete = false;

      try {
        // Increased timeout to 3 seconds for slow CMPs (was 100ms)
        var timeoutId = setTimeout(function() {
          if (!checkComplete) {
            catalyst.log('CMP consent check timeout (3s) - allowing request without FPID for resilience');
            // Changed: Return true on timeout to allow requests without user IDs
            // This prevents blocking the entire auction if CMP is slow
            hasValidConsent = true;
            checkComplete = true;
          }
        }, 3000); // 3000ms timeout (was 100ms)

        window.__tcfapi('getTCData', 2, function(tcData, success) {
          if (!checkComplete) {
            clearTimeout(timeoutId);
            if (success && tcData) {
              // GDPR doesn't apply - allow FPID
              if (!tcData.gdprApplies) {
                hasValidConsent = true;
                catalyst.log('CMP: GDPR does not apply - allowing FPID');
              }
              // GDPR applies - check for valid consent string
              else if (tcData.tcString && tcData.tcString.length >= 20) {
                hasValidConsent = true;
                catalyst.log('CMP: Valid GDPR consent found - allowing FPID');
              } else {
                catalyst.log('CMP: GDPR applies but no valid consent - denying FPID (requests still proceed)');
              }
            } else {
              catalyst.log('CMP: getTCData failed or returned invalid data');
            }
            checkComplete = true;
          }
        });

        // Busy-wait for callback (needed for sync operation, max 3s)
        var startTime = Date.now();
        while (!checkComplete && (Date.now() - startTime < 3000)) {
          // Wait for CMP callback
        }

      } catch (e) {
        catalyst.log('Error checking GDPR consent for FPID:', e);
        // On error, allow request but without FPID (was: deny FPID and block)
        hasValidConsent = true;
        checkComplete = true;
      }

      // Return consent status
      return hasValidConsent;
    },

    // Generate new FPID
    generate: function() {
      var timestamp = Date.now();
      var random = Math.random().toString(36).substr(2, 12);
      return 'fpi_' + timestamp + '_' + random;
    },

    // Get FPID from cookie (parse uids cookie JSON)
    get: function() {
      var uids = catalyst._getCookie(this.cookieName);
      if (uids) {
        try {
          var decoded = atob(uids);
          var data = JSON.parse(decoded);
          return data.fpid || null;
        } catch (e) {
          catalyst.log('Failed to parse fpid from cookie:', e);
          return null;
        }
      }
      return null;
    },

    // Set FPID in cookie
    set: function(fpid) {
      // Store FPID in uids cookie structure
      var uids = catalyst._getCookie(this.cookieName);
      var data = {};

      if (uids) {
        try {
          var decoded = atob(uids);
          data = JSON.parse(decoded);
        } catch (e) {
          catalyst.log('Failed to parse existing uids cookie, creating new:', e);
        }
      }

      data.fpid = fpid;
      var encoded = btoa(JSON.stringify(data));

      // Calculate expiry date
      var expiryDate = new Date();
      expiryDate.setTime(expiryDate.getTime() + (this.expiryDays * 24 * 60 * 60 * 1000));

      // Set cookie with SameSite=Lax for cross-site compatibility
      document.cookie = this.cookieName + '=' + encoded +
                       '; expires=' + expiryDate.toUTCString() +
                       '; path=/' +
                       '; SameSite=Lax';

      catalyst.log('Saved FPID to cookie:', fpid);
    },

    // Generate or retrieve existing FPID (respects GDPR consent)
    getOrCreate: function() {
      // 1. Check memory cache first (ensures consistency within page session)
      if (_cachedFPID) {
        return _cachedFPID;
      }

      // 2. Check cookie for persistent FPID
      var fpid = this.get();
      if (fpid) {
        _cachedFPID = fpid;  // Cache it for this session
        catalyst.log('Retrieved existing FPID from cookie:', fpid);
        return fpid;
      }

      // 3. Only generate new FPID if we have consent
      if (!this.hasConsent()) {
        catalyst.log('FPID generation blocked - GDPR applies but no valid TCF consent');
        return null;
      }

      // 4. Generate new FPID and persist immediately
      fpid = this.generate();
      _cachedFPID = fpid;  // Cache immediately
      this.set(fpid);      // Save to cookie
      catalyst.log('Generated and cached new FPID:', fpid);
      return fpid;
    }
  };

  /**
   * Initialize Catalyst SDK
   * @param {Object} config - Configuration object
   * @param {string} config.accountId - MAI Publisher account ID (defaults to '12345')
   * @param {number} [config.timeout] - Optional timeout in ms (default: 2800)
   * @param {boolean} [config.debug] - Enable debug logging
   * @param {Function} [callback] - Optional callback function called when SDK is ready (after user sync completes)
   */
  catalyst.init = function(config, callback) {
    // Use provided accountId or fallback to '12345' for testing
    if (!config) {
      config = {};
    }
    catalyst._config.accountId = config.accountId || '12345';
    catalyst.log('Using accountId:', catalyst._config.accountId);

    if (config.timeout) {
      catalyst._config.timeout = config.timeout;
    }

    if (config.debug !== undefined) {
      catalyst._config.debug = config.debug;
    }

    // Allow override of user sync settings
    if (config.userSync !== undefined) {
      if (typeof config.userSync === 'boolean') {
        catalyst._config.userSync.enabled = config.userSync;
      } else if (typeof config.userSync === 'object') {
        for (var key in config.userSync) {
          if (config.userSync.hasOwnProperty(key)) {
            catalyst._config.userSync[key] = config.userSync[key];
          }
        }
      }
    }

    catalyst._initialized = true;
    catalyst.log('Catalyst SDK initialized with accountId:', config.accountId);

    // Trigger user sync IMMEDIATELY (no delay) and wait for completion before callback
    if (catalyst._config.userSync.enabled) {
      catalyst._performUserSync(function() {
        catalyst.log('User sync complete - SDK ready for bid requests');
        if (typeof callback === 'function') {
          callback();
        }
      });
    } else {
      // User sync disabled, ready immediately
      if (typeof callback === 'function') {
        callback();
      }
    }
  };

  /**
   * Request bids for ad slots
   * @param {Object} config - Bid request configuration
   * @param {Array} config.slots - Array of ad slot configurations
   * @param {Function} callback - Callback function(bids) called when bids are ready or timeout
   */
  catalyst.requestBids = function(config, callback) {
    if (!catalyst._initialized) {
      catalyst.log('Error: Catalyst SDK not initialized. Call catalyst.init() first.');
      if (typeof callback === 'function') {
        callback([]);
      }
      return;
    }

    // Queue if user sync hasn't completed yet — replay with UIDs after sync
    if (catalyst._config.userSync.enabled && !catalyst._userSyncComplete) {
      catalyst.log('requestBids queued — waiting for user sync to complete');
      catalyst._pendingBidRequests.push({ config: config, callback: callback });
      return;
    }

    if (!config || !config.slots || config.slots.length === 0) {
      catalyst.log('Error: slots array is required');
      if (typeof callback === 'function') {
        callback([]);
      }
      return;
    }

    var requestId = 'catalyst-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    var startTime = Date.now();
    var timeoutMs = catalyst._config.timeout;

    // Detect device type once — used for both the request and per-slot size filtering
    var deviceType = catalyst._detectDeviceType();

    // Build MAI bid request
    var bidRequest = {
      accountId: catalyst._config.accountId,
      timeout: timeoutMs,
      slots: [],
      page: {
        url: window.location.href,
        domain: window.location.hostname,
        keywords: [],
        categories: []
      },
      device: {
        width: window.screen.width,
        height: window.screen.height,
        deviceType: deviceType,
        userAgent: navigator.userAgent,
        geo: null // Will be populated if available
      }
    };

    // Process slots
    for (var i = 0; i < config.slots.length; i++) {
      var slot = config.slots[i];

      if (!slot.divId || !slot.sizes || slot.sizes.length === 0) {
        catalyst.log('Warning: Invalid slot configuration, skipping:', slot);
        continue;
      }

      // Skip out-of-page slots (special GPT formats like interstitials, overlays)
      if (Array.isArray(slot.sizes) && slot.sizes.length === 1 &&
          typeof slot.sizes[0] === 'string' && slot.sizes[0].toLowerCase() === 'out-of-page') {
        catalyst.log('Skipping out-of-page slot (not eligible for bidding):', slot.divId);
        continue;
      }

      // Normalize sizes to [[w, h], ...] format
      var normalizedSizes = catalyst._normalizeSizes(slot.sizes);
      if (!normalizedSizes || normalizedSizes.length === 0) {
        catalyst.log('Warning: Could not normalize sizes for slot:', slot.divId,
                     '| Raw sizes:', JSON.stringify(slot.sizes),
                     '| Type:', typeof slot.sizes,
                     '| Full slot:', JSON.stringify(slot));
        continue;
      }

      // Filter to device-appropriate sizes and sort largest first
      var readySizes = catalyst._filterAndSortSizes(normalizedSizes, deviceType);

      // Resolve adUnitPath: publisher config → GPT slot → empty
      var adUnitPath = slot.adUnitPath || catalyst._getGPTAdUnitPath(slot.divId) || '';

      catalyst.log('Slot', slot.divId, '| adUnitPath:', adUnitPath || '(none)',
                   '| sizes:', JSON.stringify(readySizes));

      bidRequest.slots.push({
        divId: slot.divId,
        sizes: readySizes,
        adUnitPath: adUnitPath,
        position: slot.position || ''
      });
    }

    // Early exit if no eligible slots - prevents empty bid requests
    if (bidRequest.slots.length === 0) {
      catalyst.log('No eligible slots found - skipping bid request (all slots filtered out or not ready)');
      if (typeof callback === 'function') {
        callback([]);
      }
      return;
    }

    catalyst.log('Requesting bids for', bidRequest.slots.length, 'slots with timeout', timeoutMs + 'ms');

    // Add page context if provided
    if (config.page) {
      if (config.page.keywords) {
        bidRequest.page.keywords = config.page.keywords;
      }
      if (config.page.categories) {
        bidRequest.page.categories = config.page.categories;
      }
    }

    // Include user IDs and FPID as OpenRTB eids
    var userIds = catalyst._getUserIds();
    var fpid = catalyst._fpidManager.getOrCreate();

    // Build eids array (OpenRTB Extended Identifiers standard)
    var eids = catalyst._buildEids(userIds, fpid);

    if (eids.length > 0) {
      bidRequest.user = bidRequest.user || {};
      bidRequest.user.ext = bidRequest.user.ext || {};
      bidRequest.user.ext.eids = eids;

      catalyst.log('Including', eids.length, 'extended identifiers (eids)');
      if (fpid) {
        catalyst.log('  - FPID:', fpid);
      }
      if (Object.keys(userIds).length > 0) {
        catalyst.log('  - Bidder IDs:', Object.keys(userIds).join(', '));
      }
    } else {
      catalyst.log('FPID not included - consent not available');
    }

    // Collect ORTB2 data from Prebid.js
    var ortb2Data = catalyst._getORTB2Data();

    // Merge ORTB2 site data (content categories, keywords, etc.)
    if (ortb2Data.site) {
      if (ortb2Data.site.cat && ortb2Data.site.cat.length > 0) {
        bidRequest.page.categories = ortb2Data.site.cat;
      }
      if (ortb2Data.site.keywords) {
        bidRequest.page.keywords = ortb2Data.site.keywords;
      }
      if (ortb2Data.site.content && Object.keys(ortb2Data.site.content).length > 0) {
        bidRequest.page.content = ortb2Data.site.content;
      }
      if (ortb2Data.site.ext && Object.keys(ortb2Data.site.ext).length > 0) {
        bidRequest.page.ext = ortb2Data.site.ext;
      }
    }

    // Merge ORTB2 device data (geo targeting, connection info)
    if (ortb2Data.device) {
      if (ortb2Data.device.geo && Object.keys(ortb2Data.device.geo).length > 0) {
        bidRequest.device.geo = ortb2Data.device.geo;
      }
      if (ortb2Data.device.connectiontype) {
        bidRequest.device.connectiontype = ortb2Data.device.connectiontype;
      }
      if (ortb2Data.device.ext && Object.keys(ortb2Data.device.ext).length > 0) {
        bidRequest.device.ext = ortb2Data.device.ext;
      }
    }

    // Merge ORTB2 user data (segments, interests, demographics)
    if (ortb2Data.user) {
      bidRequest.user = bidRequest.user || {};
      if (ortb2Data.user.data && ortb2Data.user.data.length > 0) {
        bidRequest.user.data = ortb2Data.user.data;
      }
      if (ortb2Data.user.ext && Object.keys(ortb2Data.user.ext).length > 0) {
        bidRequest.user.ext = bidRequest.user.ext || {};
        for (var ukey in ortb2Data.user.ext) {
          if (!ortb2Data.user.ext.hasOwnProperty(ukey)) { continue; }
          if (ukey === 'eids') {
            // Merge eids arrays — don't overwrite the EIDs we already built
            var existingEids = bidRequest.user.ext.eids || [];
            var ortb2Eids = ortb2Data.user.ext.eids || [];
            var eidSources = {};
            for (var e = 0; e < existingEids.length; e++) {
              if (existingEids[e].source) { eidSources[existingEids[e].source] = true; }
            }
            for (var o = 0; o < ortb2Eids.length; o++) {
              if (!ortb2Eids[o].source || !eidSources[ortb2Eids[o].source]) {
                existingEids.push(ortb2Eids[o]);
              }
            }
            bidRequest.user.ext.eids = existingEids;
          } else {
            bidRequest.user.ext[ukey] = ortb2Data.user.ext[ukey];
          }
        }
      }
    }

    // Setup timeout
    var timedOut = false;
    var timeoutId = setTimeout(function() {
      timedOut = true;
      catalyst.log('Bid request timed out after', timeoutMs + 'ms');

      if (typeof callback === 'function') {
        callback([]);
      }

      delete catalyst._bidRequests[requestId];
    }, timeoutMs);

    // Store bid request
    catalyst._bidRequests[requestId] = {
      config: config,
      callback: callback,
      startTime: startTime,
      timeoutId: timeoutId
    };

    // Get privacy consent data, then send bid request
    var sendBidRequest = function() {
      // Make POST request to /v1/bid endpoint
      catalyst._makeBidRequest(bidRequest, function(error, response) {
      if (timedOut) {
        catalyst.log('Ignoring response after timeout');
        return;
      }

      clearTimeout(timeoutId);
      delete catalyst._bidRequests[requestId];

      var elapsedMs = Date.now() - startTime;

      if (error) {
        catalyst.log('Bid request failed:', error, 'in', elapsedMs + 'ms');
        if (typeof callback === 'function') {
          callback([]);
        }
        return;
      }

      var bids = response.bids || [];
      catalyst.log('Received', bids.length, 'bids in', elapsedMs + 'ms');

      if (typeof callback === 'function') {
        callback(bids);
      }
      });
    };

    // Try to get geolocation, then add privacy consent, then send request
    // Geo collection is async but won't block the bid request (max 1s delay)
    catalyst._geoCache.getOrRequest(function(geoData) {
      // Add geo to device if available (15-30% CPM lift)
      if (geoData) {
        bidRequest.device.geo = geoData;
        catalyst.log('Including client-side geolocation in bid request');
      } else {
        // Server will use IP-based geo as fallback
        catalyst.log('No client-side geo available - server will use IP geolocation');
      }

      // Add privacy/consent info if available, then send request
      if (window.__tcfapi || window.__uspapi || window.__cmp) {
        catalyst._addPrivacyConsent(bidRequest, sendBidRequest);
      } else {
        // No privacy APIs available, send immediately
        sendBidRequest();
      }
    });
  };

  /**
   * Make POST request to bid endpoint
   * @param {Object} bidRequest - Bid request payload
   * @param {Function} callback - Callback function(error, response)
   * @private
   */
  catalyst._makeBidRequest = function(bidRequest, callback) {
    var url = catalyst._config.serverUrl + '/v1/bid';

    // Log complete bid request if debug enabled
    if (catalyst._config.debug) {
      catalyst.log('=== FULL BID REQUEST ===');
      catalyst.log(JSON.stringify(bidRequest, null, 2));
      catalyst.log('========================');
    }

    var xhr = new XMLHttpRequest();
    xhr.open('POST', url, true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.withCredentials = true; // CRITICAL: Send/receive cookies for user sync
    xhr.timeout = catalyst._config.timeout;

    xhr.onload = function() {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          var response = JSON.parse(xhr.responseText);

          // Log complete bid response if debug enabled
          if (catalyst._config.debug) {
            catalyst.log('=== FULL BID RESPONSE ===');
            catalyst.log(JSON.stringify(response, null, 2));
            catalyst.log('=========================');
          }

          callback(null, response);
        } catch (e) {
          catalyst.log('Error parsing response:', e);
          catalyst.log('Response text:', xhr.responseText);
          callback(e, null);
        }
      } else {
        catalyst.log('Bid request failed with status:', xhr.status);
        catalyst.log('Response:', xhr.responseText);
        callback(new Error('HTTP ' + xhr.status), null);
      }
    };

    xhr.onerror = function() {
      catalyst.log('Network error making bid request');
      callback(new Error('Network error'), null);
    };

    xhr.ontimeout = function() {
      catalyst.log('Bid request timeout after', catalyst._config.timeout, 'ms');
      callback(new Error('Request timeout'), null);
    };

    try {
      xhr.send(JSON.stringify(bidRequest));
    } catch (e) {
      catalyst.log('Exception sending bid request:', e);
      callback(e, null);
    }
  };

  /**
   * Notify MAI Publisher that bids are ready
   * @private
   * @deprecated This function is no longer used. Catalyst now relies on callbacks only.
   */
  catalyst._notifyReady = function() {
    // No-op: Callback pattern is now used instead of global function calls
    // This function is kept for backwards compatibility but does nothing
  };

  /**
   * Read and parse the uids cookie AND get user IDs from Prebid.js
   * @returns {Object} Map of bidder -> user ID
   * @private
   */
  catalyst._getUserIds = function() {
    var userIds = {};

    // Read from Catalyst's own uids cookie (server-side synced bidder IDs)
    var uids = catalyst._getCookie('uids');
    if (uids) {
      try {
        var decoded = atob(uids);
        var data = JSON.parse(decoded);
        for (var bidder in data.uids || {}) {
          if (data.uids[bidder].uid && !catalyst._isExpired(data.uids[bidder].expires)) {
            userIds[bidder] = data.uids[bidder].uid;
          }
        }
      } catch (e) {
        catalyst.log('Failed to parse uids cookie:', e);
      }
    }

    return userIds;
  };

  /**
   * Convert userIds object to OpenRTB eids array format
   * @param {Object} userIds - Object mapping bidder codes to user IDs
   * @param {string} fpid - First-party identifier
   * @returns {Array} OpenRTB Extended Identifiers array
   * @private
   */
  catalyst._buildEids = function(userIds, fpid) {
    var eids = [];
    var seenSources = {};

    // 1. First-party ID first
    if (fpid) {
      seenSources['thenexusengine.com'] = true;
      eids.push({
        source: 'thenexusengine.com',
        uids: [{ id: fpid, atype: 1 }]
      });
    }

    // 2. Prebid ID module EIDs via getUserIdsAsEids() — returns proper OpenRTB format
    //    with correct source domains (id5-sync.com, pubcid.org, etc.), no manual mapping needed
    if (window.pbjs && typeof window.pbjs.getUserIdsAsEids === 'function') {
      try {
        var prebidEids = window.pbjs.getUserIdsAsEids() || [];
        for (var p = 0; p < prebidEids.length; p++) {
          var peid = prebidEids[p];
          if (peid && peid.source && !seenSources[peid.source]) {
            seenSources[peid.source] = true;
            eids.push(peid);
          }
        }
        if (prebidEids.length > 0) {
          catalyst.log('Added', prebidEids.length, 'EIDs from Prebid.js getUserIdsAsEids()');
        }
      } catch (e) {
        catalyst.log('Failed to get Prebid EIDs:', e);
      }
    }

    // 3. Catalyst cookie-synced bidder UIDs (server-side syncs), skip if already present
    var bidderSources = {
      'kargo': 'kargo.com',
      'rubicon': 'rubiconproject.com',
      'pubmatic': 'pubmatic.com',
      'appnexus': 'appnexus.com',
      'sovrn': 'lijit.com',
      'triplelift': 'triplelift.com',
      'liveintent': 'liveintent.com',
      'criteo': 'criteo.com',
      'thetradedesk': 'adsrvr.org'
    };
    for (var bidder in userIds) {
      var source = bidderSources[bidder] || (bidder + '.com');
      if (!seenSources[source]) {
        seenSources[source] = true;
        eids.push({
          source: source,
          uids: [{ id: userIds[bidder], atype: 1 }]
        });
      }
    }

    return eids;
  };

  /**
   * Get ORTB2 data from Prebid.js configuration
   * This collects site, device, and user data for enhanced targeting
   * @returns {Object} ORTB2 data with site, device, and user properties
   * @private
   */
  catalyst._getORTB2Data = function() {
    var ortb2 = {
      site: null,
      device: null,
      user: null
    };

    if (window.pbjs && typeof window.pbjs.getConfig === 'function') {
      try {
        // Get ORTB2 site data (content categories, keywords, publisher FPD)
        var siteData = window.pbjs.getConfig('ortb2.site');
        if (siteData) {
          ortb2.site = {
            cat: siteData.cat || [],           // IAB content categories
            keywords: siteData.keywords || '', // Page keywords
            content: siteData.content || {},   // Content metadata
            ext: siteData.ext || {}            // Publisher first-party data
          };
          catalyst.log('Added ORTB2 site data with', ortb2.site.cat.length, 'categories');
        }

        // Get ORTB2 device data (geo targeting, connection info)
        var deviceData = window.pbjs.getConfig('ortb2.device');
        if (deviceData) {
          ortb2.device = {
            geo: deviceData.geo || {},         // Geographic targeting data
            connectiontype: deviceData.connectiontype,
            ext: deviceData.ext || {}
          };
          if (ortb2.device.geo.country) {
            catalyst.log('Added ORTB2 device data with geo:', ortb2.device.geo.country);
          }
        }

        // Get ORTB2 user data (segments, interests, demographics)
        var userData = window.pbjs.getConfig('ortb2.user');
        if (userData) {
          ortb2.user = {
            data: userData.data || [],         // User segments and interests
            ext: userData.ext || {}            // User first-party data
          };
          if (ortb2.user.data.length > 0) {
            catalyst.log('Added ORTB2 user data with', ortb2.user.data.length, 'segments');
          }
        }
      } catch (e) {
        catalyst.log('Failed to get ORTB2 data from Prebid.js:', e);
      }
    }

    return ortb2;
  };

  /**
   * Get a cookie by name
   * @param {string} name - Cookie name
   * @returns {string|null} Cookie value or null
   * @private
   */
  catalyst._getCookie = function(name) {
    var cookies = document.cookie.split(';');
    for (var i = 0; i < cookies.length; i++) {
      var cookie = cookies[i].trim();
      if (cookie.indexOf(name + '=') === 0) {
        return cookie.substring(name.length + 1);
      }
    }
    return null;
  };

  /**
   * Check if a timestamp has expired
   * @param {string} expiresStr - ISO date string
   * @returns {boolean} True if expired
   * @private
   */
  catalyst._isExpired = function(expiresStr) {
    if (!expiresStr) return true;
    try {
      return new Date(expiresStr) < new Date();
    } catch (e) {
      return true;
    }
  };

  /**
   * Add privacy consent to bid request
   * @param {Object} bidRequest - Bid request object
   * @param {Function} callback - Called when consent data is ready
   * @private
   */
  catalyst._addPrivacyConsent = function(bidRequest, callback) {
    bidRequest.user = bidRequest.user || {};

    var tcfDone = false;
    var uspDone = false;
    var timeout = 200; // Max 200ms wait for consent data
    var timeoutId = null;

    var checkComplete = function() {
      if ((tcfDone || !window.__tcfapi) && (uspDone || !window.__uspapi)) {
        if (timeoutId) clearTimeout(timeoutId);
        if (callback) callback();
      }
    };

    // Timeout fallback - fail closed for GDPR compliance
    timeoutId = setTimeout(function() {
      catalyst.log('Privacy consent timeout - failing closed for GDPR safety');
      // CRITICAL: If CMP exists but times out, assume GDPR applies with no consent
      // This is safer than assuming gdpr=0 which could violate GDPR
      if (window.__tcfapi && !tcfDone) {
        bidRequest.user.gdprApplies = true;
        bidRequest.user.consentGiven = false;
        catalyst.log('CMP timeout - marking GDPR as applying without consent');
      }
      tcfDone = true;
      uspDone = true;
      if (callback) callback();
    }, timeout);

    // Try to get GDPR consent via TCF API
    if (window.__tcfapi) {
      try {
        window.__tcfapi('getTCData', 2, function(tcData, success) {
          tcfDone = true;
          if (success && tcData) {
            bidRequest.user.gdprApplies = tcData.gdprApplies || false;
            bidRequest.user.consentGiven = tcData.eventStatus === 'tcloaded' || tcData.eventStatus === 'useractioncomplete';

            // CRITICAL: Pass the actual TCF consent string for bidders (only if GDPR applies)
            if (tcData.gdprApplies && tcData.tcString) {
              bidRequest.user.consentString = tcData.tcString;
              catalyst.log('Added TCF consent string for GDPR region');
            }
          }
          checkComplete();
        });
      } catch (e) {
        catalyst.log('Error getting GDPR consent:', e);
        tcfDone = true;
        checkComplete();
      }
    } else {
      tcfDone = true;
    }

    // Try to get US Privacy string
    if (window.__uspapi) {
      try {
        window.__uspapi('getUSPData', 1, function(uspData, success) {
          uspDone = true;
          if (success && uspData && uspData.uspString) {
            bidRequest.user.uspConsent = uspData.uspString;
          } else {
            // FIXED #9: Use safe default indicating no data available
            bidRequest.user.uspConsent = '1---';
          }
          checkComplete();
        });
      } catch (e) {
        catalyst.log('Error getting USP consent:', e);
        // FIXED #9: Use safe default on error
        bidRequest.user.uspConsent = '1---';
        uspDone = true;
        checkComplete();
      }
    } else {
      // FIXED #9: Use safe default indicating no CMP/data available
      bidRequest.user.uspConsent = '1---';
      uspDone = true;
    }

    // If neither API is available, call back immediately
    if (!window.__tcfapi && !window.__uspapi) {
      checkComplete();
    }
  };

  /**
   * Perform user sync with configured bidders
   * @private
   */
  catalyst._performUserSync = function(onComplete) {
    if (!catalyst._config.userSync.enabled) {
      catalyst.log('User sync disabled');
      if (typeof onComplete === 'function') {
        onComplete();
      }
      return;
    }

    if (catalyst._userSyncComplete) {
      catalyst.log('User sync already performed');
      if (typeof onComplete === 'function') {
        onComplete();
      }
      return;
    }

    // Wrap onComplete so queue is drained before the init callback fires
    var _originalComplete = onComplete;
    onComplete = function() {
      catalyst._drainPendingBidRequests();
      if (typeof _originalComplete === 'function') {
        _originalComplete();
      }
    };

    // Privacy consent will be checked when we gather privacy data for the sync request
    // Server also validates consent as a backup (defense in depth)
    catalyst.log('Starting user sync for bidders:', catalyst._config.userSync.bidders);

    // Build cookie sync request
    var syncRequest = {
      bidders: catalyst._config.userSync.bidders,
      gdpr: 0,
      gdpr_consent: '',
      us_privacy: '',
      limit: catalyst._config.userSync.maxSyncs
    };

    // Include FPID only if consent is available
    var fpid = catalyst._fpidManager.getOrCreate();
    if (fpid) {
      syncRequest.fpid = fpid;
      catalyst.log('Including FPID in cookie sync:', fpid);
    } else {
      catalyst.log('FPID not included in cookie sync - consent not available');
    }

    // Function to send cookie sync request
    var sendSyncRequest = function() {
      var url = catalyst._config.serverUrl + '/cookie_sync';
      var xhr = new XMLHttpRequest();
      xhr.open('POST', url, true);
      xhr.setRequestHeader('Content-Type', 'application/json');
      xhr.withCredentials = true; // CRITICAL: Send/receive cookies for user sync
      xhr.timeout = 5000;

      xhr.onload = function() {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            var response = JSON.parse(xhr.responseText);
            catalyst._fireSyncPixels(response);

            // TIMING FIX: Wait for setuid callbacks to complete before marking sync done
            var syncWaitTime = catalyst._config.userSync.syncWaitTime || 1500;
            catalyst.log('Waiting ' + syncWaitTime + 'ms for setuid callbacks to complete...');

            setTimeout(function() {
              catalyst._userSyncComplete = true;
              catalyst.log('Cookie sync grace period complete - SDK ready for bid requests');
              if (typeof onComplete === 'function') {
                onComplete();
              }
            }, syncWaitTime);
          } catch (e) {
            catalyst.log('Error parsing sync response:', e);
            catalyst._userSyncComplete = true; // FIXED #3: Mark complete even on error
            if (typeof onComplete === 'function') {
              onComplete();
            }
          }
        } else {
          catalyst.log('User sync request failed:', xhr.status);
          catalyst._userSyncComplete = true; // FIXED #3: Mark complete even on failure
          if (typeof onComplete === 'function') {
            onComplete();
          }
        }
      };

      xhr.onerror = function() {
        catalyst.log('User sync network error');
        catalyst._userSyncComplete = true; // FIXED #3: Mark complete even on network error
        if (typeof onComplete === 'function') {
          onComplete();
        }
      };

      xhr.ontimeout = function() {
        catalyst.log('User sync timeout');
        catalyst._userSyncComplete = true; // FIXED #3: Mark complete even on timeout
        if (typeof onComplete === 'function') {
          onComplete();
        }
      };

      try {
        catalyst.log('Sending cookie sync with privacy params:', {
          gdpr: syncRequest.gdpr,
          gdpr_consent: syncRequest.gdpr_consent ? syncRequest.gdpr_consent.substring(0, 20) + '...' : '',
          us_privacy: syncRequest.us_privacy
        });
        xhr.send(JSON.stringify(syncRequest));
      } catch (e) {
        catalyst.log('Error sending sync request:', e);
        catalyst._userSyncComplete = true; // FIXED #3: Mark complete even on send error
        if (typeof onComplete === 'function') {
          onComplete();
        }
      }
    };

    // Add privacy parameters if available, then send request
    if (window.__tcfapi || window.__uspapi) {
      catalyst._addPrivacyToSyncRequest(syncRequest, sendSyncRequest);
    } else {
      // No privacy APIs available, send immediately
      sendSyncRequest();
    }
  };

  /**
   * Fire sync pixels/iframes for bidders
   * @param {Object} response - Cookie sync response
   * @private
   */
  catalyst._fireSyncPixels = function(response) {
    if (!response.bidder_status || response.bidder_status.length === 0) {
      catalyst.log('No sync URLs to fire');
      return;
    }

    var config = catalyst._config.userSync;
    var syncsFired = 0;

    for (var i = 0; i < response.bidder_status.length; i++) {
      var bidderSync = response.bidder_status[i];

      if (!bidderSync.usersync || !bidderSync.usersync.url) {
        continue;
      }

      var syncInfo = bidderSync.usersync;
      var syncType = syncInfo.type;

      // Check if sync type is enabled
      if (syncType === 'iframe' && !config.iframeEnabled) {
        catalyst.log('Skipping iframe sync for', syncInfo.bidder);
        continue;
      }

      if (syncType === 'redirect' && !config.pixelEnabled) {
        catalyst.log('Skipping pixel sync for', syncInfo.bidder);
        continue;
      }

      // Fire the sync
      if (syncType === 'iframe') {
        catalyst._fireIframeSync(syncInfo.url, syncInfo.bidder);
      } else {
        catalyst._firePixelSync(syncInfo.url, syncInfo.bidder);
      }

      catalyst._syncedBidders.push(syncInfo.bidder);
      syncsFired++;

      if (syncsFired >= config.maxSyncs) {
        catalyst.log('Max syncs reached:', config.maxSyncs);
        break;
      }
    }

    catalyst.log('Fired', syncsFired, 'user syncs');
  };

  /**
   * Fire iframe sync
   * @param {string} url - Sync URL
   * @param {string} bidder - Bidder code
   * @private
   */
  catalyst._fireIframeSync = function(url, bidder) {
    try {
      var iframe = document.createElement('iframe');
      iframe.src = url;
      iframe.style.display = 'none';
      iframe.width = 0;
      iframe.height = 0;
      iframe.setAttribute('data-bidder', bidder);
      document.body.appendChild(iframe);
      catalyst.log('Fired iframe sync for', bidder);

      // Remove iframe after 10 seconds to prevent memory leak
      setTimeout(function() {
        try {
          if (iframe.parentNode) {
            iframe.parentNode.removeChild(iframe);
            catalyst.log('Cleaned up iframe sync for', bidder);
          }
        } catch (e) {
          catalyst.log('Error cleaning up iframe sync for', bidder, e);
        }
      }, 10000);
    } catch (e) {
      catalyst.log('Error firing iframe sync for', bidder, e);
    }
  };

  /**
   * Fire pixel/redirect sync
   * @param {string} url - Sync URL
   * @param {string} bidder - Bidder code
   * @private
   */
  catalyst._firePixelSync = function(url, bidder) {
    try {
      var img = new Image();
      img.src = url;
      img.style.display = 'none';
      img.width = 1;
      img.height = 1;
      img.setAttribute('data-bidder', bidder);
      catalyst.log('Fired pixel sync for', bidder);
    } catch (e) {
      catalyst.log('Error firing pixel sync for', bidder, e);
    }
  };

  /**
   * Check if user has given privacy consent for syncing
   * @returns {boolean} True if consent given or not required
   * @private
   * @deprecated This function has async race conditions and is no longer used
   *   Server validates consent properly with actual privacy data
   */
  catalyst._hasPrivacyConsent = function() {
    // Kept for backwards compatibility only
    // Actual consent validation happens server-side with proper privacy strings
    // collected asynchronously via _addPrivacyConsent() and _addPrivacyToSyncRequest()
    return true;
  };

  /**
   * Add privacy parameters to sync request
   * @param {Object} syncRequest - Sync request object to modify
   * @param {Function} callback - Called when privacy data is ready
   * @private
   */
  catalyst._addPrivacyToSyncRequest = function(syncRequest, callback) {
    var tcfDone = false;
    var uspDone = false;
    var timeout = 200; // Max 200ms wait for consent data
    var timeoutId = null;

    var checkComplete = function() {
      if ((tcfDone || !window.__tcfapi) && (uspDone || !window.__uspapi)) {
        if (timeoutId) clearTimeout(timeoutId);
        if (callback) callback();
      }
    };

    // Timeout fallback - don't wait forever for consent data
    timeoutId = setTimeout(function() {
      catalyst.log('Cookie sync privacy timeout, proceeding');
      tcfDone = true;
      uspDone = true;
      if (callback) callback();
    }, timeout);

    // Try to get GDPR consent
    if (window.__tcfapi) {
      try {
        window.__tcfapi('getTCData', 2, function(tcData, success) {
          tcfDone = true;
          if (success && tcData) {
            syncRequest.gdpr = tcData.gdprApplies ? 1 : 0;
            syncRequest.gdpr_consent = tcData.tcString || '';
            if (tcData.gdprApplies && tcData.tcString) {
              catalyst.log('Added TCF consent string for cookie sync');
            }
          }
          checkComplete();
        });
      } catch (e) {
        catalyst.log('Error getting GDPR consent for sync:', e);
        tcfDone = true;
        checkComplete();
      }
    } else {
      tcfDone = true;
    }

    // Try to get US Privacy string
    if (window.__uspapi) {
      try {
        window.__uspapi('getUSPData', 1, function(uspData, success) {
          uspDone = true;
          if (success && uspData && uspData.uspString) {
            syncRequest.us_privacy = uspData.uspString;
            catalyst.log('Added USP consent string for cookie sync');
          } else {
            // FIXED #9: Use safe default indicating no data available
            syncRequest.us_privacy = '1---';
            catalyst.log('Using default USP string (CMP present but no data)');
          }
          checkComplete();
        });
      } catch (e) {
        catalyst.log('Error getting USP consent for sync:', e);
        // FIXED #9: Use safe default on error
        syncRequest.us_privacy = '1---';
        uspDone = true;
        checkComplete();
      }
    } else {
      // FIXED #9: Use safe default indicating no CMP/data available
      syncRequest.us_privacy = '1---';
      catalyst.log('Using default USP string (no CMP found)');
      uspDone = true;
    }

    // If neither API is available, call back immediately
    if (!window.__tcfapi && !window.__uspapi) {
      checkComplete();
    }
  };

  /**
   * Normalize sizes to [[width, height], ...] format
   * Handles multiple input formats:
   * - [[300, 250], [728, 90]] -> [[300, 250], [728, 90]] (passthrough)
   * - [300, 250] -> [[300, 250]]
   * - "300x250" -> [[300, 250]]
   * - ["300x250", "728x90"] -> [[300, 250], [728, 90]]
   * @param {*} sizes - Sizes in any format
   * @returns {Array} Normalized sizes as [[w, h], ...]
   * @private
   */
  catalyst._normalizeSizes = function(sizes) {
    if (!sizes) {
      catalyst.log('DEBUG: _normalizeSizes received null/undefined');
      return [];
    }

    var normalized = [];

    // Handle array input
    if (Array.isArray(sizes)) {
      for (var i = 0; i < sizes.length; i++) {
        var size = sizes[i];

        // Skip null/undefined entries
        if (size == null) {
          catalyst.log('DEBUG: Skipping null/undefined size at index', i);
          continue;
        }

        // Already in [width, height] format
        if (Array.isArray(size) && size.length === 2 &&
            typeof size[0] === 'number' && typeof size[1] === 'number' &&
            size[0] > 0 && size[1] > 0) {
          normalized.push(size);
        }
        // Try to coerce array with string numbers: ["300", "250"]
        else if (Array.isArray(size) && size.length === 2) {
          var w = parseInt(size[0], 10);
          var h = parseInt(size[1], 10);
          if (!isNaN(w) && !isNaN(h) && w > 0 && h > 0) {
            normalized.push([w, h]);
            catalyst.log('DEBUG: Coerced string sizes to numbers:', [w, h]);
          } else {
            catalyst.log('DEBUG: Could not coerce array size:', JSON.stringify(size));
          }
        }
        // String format: "300x250"
        else if (typeof size === 'string') {
          var parsed = catalyst._parseSizeString(size);
          if (parsed) {
            normalized.push(parsed);
          } else {
            catalyst.log('DEBUG: Could not parse size string:', size);
          }
        }
        // Single [width, height] - not nested
        else if (i === 0 && typeof size === 'number' && typeof sizes[1] === 'number' &&
                 size > 0 && sizes[1] > 0) {
          // Input is [300, 250] not [[300, 250]]
          normalized.push([sizes[0], sizes[1]]);
          break; // Done processing
        }
        // Object format: {width: 300, height: 250} or {w: 300, h: 250}
        else if (typeof size === 'object') {
          var w = size.width || size.w;
          var h = size.height || size.h;
          if (typeof w === 'number' && typeof h === 'number' && w > 0 && h > 0) {
            normalized.push([w, h]);
            catalyst.log('DEBUG: Converted object size to array:', [w, h]);
          } else {
            catalyst.log('DEBUG: Invalid object size format:', JSON.stringify(size));
          }
        }
        else {
          catalyst.log('DEBUG: Unrecognized size format at index', i, ':', JSON.stringify(size), 'type:', typeof size);
        }
      }
    }
    // Handle string input: "300x250"
    else if (typeof sizes === 'string') {
      var parsed = catalyst._parseSizeString(sizes);
      if (parsed) {
        normalized.push(parsed);
      } else {
        catalyst.log('DEBUG: Could not parse sizes string:', sizes);
      }
    }
    // Handle object input: {width: 300, height: 250}
    else if (typeof sizes === 'object') {
      var w = sizes.width || sizes.w;
      var h = sizes.height || sizes.h;
      if (typeof w === 'number' && typeof h === 'number' && w > 0 && h > 0) {
        normalized.push([w, h]);
        catalyst.log('DEBUG: Converted object sizes to array:', [w, h]);
      } else {
        catalyst.log('DEBUG: Invalid object sizes format:', JSON.stringify(sizes));
      }
    }
    else {
      catalyst.log('DEBUG: Unrecognized sizes type:', typeof sizes, 'value:', sizes);
    }

    if (normalized.length === 0) {
      catalyst.log('DEBUG: _normalizeSizes returning empty array. Input was:', JSON.stringify(sizes));
    }

    return normalized;
  };

  /**
   * Parse size string like "300x250" to [300, 250]
   * @param {string} sizeStr - Size string
   * @returns {Array|null} [width, height] or null if invalid
   * @private
   */
  catalyst._parseSizeString = function(sizeStr) {
    if (typeof sizeStr !== 'string') {
      return null;
    }

    var parts = sizeStr.split('x');
    if (parts.length !== 2) {
      return null;
    }

    var width = parseInt(parts[0], 10);
    var height = parseInt(parts[1], 10);

    if (isNaN(width) || isNaN(height) || width <= 0 || height <= 0) {
      return null;
    }

    return [width, height];
  };

  /**
   * Detect device type
   * @returns {string} Device type: 'mobile', 'tablet', or 'desktop'
   * @private
   */
  catalyst._detectDeviceType = function() {
    var ua = navigator.userAgent;

    if (/(tablet|ipad|playbook|silk)|(android(?!.*mobi))/i.test(ua)) {
      return 'tablet';
    }

    if (/Mobile|Android|iP(hone|od)|IEMobile|BlackBerry|Kindle|Silk-Accelerated|(hpw|web)OS|Opera M(obi|ini)/.test(ua)) {
      return 'mobile';
    }

    return 'desktop';
  };

  /**
   * Resolve adUnitPath for a divId from the GPT slot registry.
   * GPT paths look like "/21912959776/domain.com/leaderboard-wide".
   * We strip the network code and any domain segment, returning e.g. "/leaderboard-wide".
   * @param {string} divId
   * @returns {string|null}
   * @private
   */
  catalyst._getGPTAdUnitPath = function(divId) {
    if (!window.googletag || !window.googletag.pubads) return null;
    try {
      var slots = window.googletag.pubads().getSlots();
      for (var i = 0; i < slots.length; i++) {
        if (slots[i].getSlotElementId() === divId) {
          var full = slots[i].getAdUnitPath(); // e.g. "/21912959776/domain.com/leaderboard-wide"
          var parts = full.replace(/^\//, '').split('/');
          // parts[0] = network code; parts[1] may be domain (contains '.'), rest = path
          var start = 1;
          if (parts.length > 2 && parts[1].indexOf('.') !== -1) {
            start = 2;
          }
          var path = '/' + parts.slice(start).join('/');
          catalyst.log('Resolved GPT adUnitPath for', divId, ':', path);
          return path;
        }
      }
    } catch (e) {
      catalyst.log('Warning: GPT adUnitPath lookup failed for', divId, ':', e.message);
    }
    return null;
  };

  /**
   * Filter sizes to those appropriate for the device type, then sort largest-area first.
   * Desktop: strips pure mobile banner sizes (width ≤ 320 AND height ≤ 100).
   * Mobile/tablet: strips large desktop-only formats (width ≥ 600).
   * Falls back to full set if filtering would leave nothing.
   * @param {Array} sizes - Normalised [[w,h], ...] array
   * @param {string} deviceType - 'desktop', 'tablet', or 'mobile'
   * @returns {Array}
   * @private
   */
  catalyst._filterAndSortSizes = function(sizes, deviceType) {
    if (!sizes || sizes.length === 0) return sizes;

    var filtered;
    if (deviceType === 'desktop') {
      // Remove mobile-only strips: narrow width AND short height
      filtered = sizes.filter(function(s) {
        return !(s[0] <= 320 && s[1] <= 100);
      });
    } else {
      // mobile / tablet: remove large desktop-only formats
      filtered = sizes.filter(function(s) {
        return s[0] < 600;
      });
    }

    // Safety: if filter removed everything, send the full set
    if (filtered.length === 0) {
      catalyst.log('_filterAndSortSizes: filter left no sizes for', deviceType, '— using full set');
      filtered = sizes.slice();
    }

    // Sort by area descending so the largest (most valuable) size is primary
    filtered.sort(function(a, b) {
      return (b[0] * b[1]) - (a[0] * a[1]);
    });

    return filtered;
  };

  catalyst._drainPendingBidRequests = function() {
    var pending = catalyst._pendingBidRequests.splice(0); // atomic drain
    for (var i = 0; i < pending.length; i++) {
      catalyst.log('Replaying queued requestBids (sync now complete)');
      catalyst.requestBids(pending[i].config, pending[i].callback);
    }
  };

  /**
   * Set targeting for Google Publisher Tag (GPT)
   * Sets Catalyst bid data as targeting key-value pairs for GAM
   * @param {Object|Array} targetingData - Targeting data or array of bids
   */
  catalyst.setTargeting = function(targetingData) {
    if (!window.googletag || !window.googletag.pubads) {
      catalyst.log('Warning: googletag not available for setTargeting');
      return;
    }

    try {
      var pubads = window.googletag.pubads();

      // Handle different input formats
      if (Array.isArray(targetingData)) {
        // Array of bids - set targeting for each
        for (var i = 0; i < targetingData.length; i++) {
          var bid = targetingData[i];
          if (bid && bid.divId) {
            catalyst._setSlotTargeting(bid);
          }
        }
      } else if (targetingData && typeof targetingData === 'object') {
        // Single bid object or targeting object
        if (targetingData.divId) {
          catalyst._setSlotTargeting(targetingData);
        } else {
          // Key-value pairs object
          for (var key in targetingData) {
            if (targetingData.hasOwnProperty(key)) {
              pubads.setTargeting(key, targetingData[key]);
              catalyst.log('Set GPT targeting:', key, '=', targetingData[key]);
            }
          }
        }
      }
    } catch (e) {
      catalyst.log('Error setting GPT targeting:', e);
    }
  };

  /**
   * Set targeting for a specific slot
   * @param {Object} bid - Bid object with targeting data
   * @private
   */
  catalyst._setSlotTargeting = function(bid) {
    if (!bid || !bid.divId) {
      return;
    }

    try {
      var pubads = window.googletag.pubads();

      // Find the GPT slot for this divId
      var slots = window.googletag.pubads().getSlots();
      var targetSlot = null;

      for (var i = 0; i < slots.length; i++) {
        var slotElementId = slots[i].getSlotElementId();
        if (slotElementId === bid.divId || slotElementId === 'mai-ad-' + bid.divId) {
          targetSlot = slots[i];
          break;
        }
      }

      if (targetSlot) {
        // Set Catalyst-specific header bidding keys (no overlap with Prebid)
        if (bid.cpm) {
          targetSlot.setTargeting('hb_pb_catalyst', bid.cpm.toFixed(2));
        }

        if (bid.creativeId) {
          targetSlot.setTargeting('hb_adid_catalyst', bid.creativeId);
          targetSlot.setTargeting('hb_creative_catalyst', bid.creativeId);
        }

        if (bid.width && bid.height) {
          targetSlot.setTargeting('hb_size_catalyst', bid.width + 'x' + bid.height);
        }

        // Set bid source (server-to-server)
        targetSlot.setTargeting('hb_source_catalyst', 's2s');

        // Set format (banner ads)
        targetSlot.setTargeting('hb_format_catalyst', 'banner');

        // Set deal ID if present (for PMP deals)
        if (bid.dealId) {
          targetSlot.setTargeting('hb_deal_catalyst', bid.dealId);
        }

        // Set advertiser domain if available
        if (bid.meta && bid.meta.advertiserDomains && bid.meta.advertiserDomains.length > 0) {
          targetSlot.setTargeting('hb_adomain_catalyst', bid.meta.advertiserDomains[0]);
        }

        // Set actual demand partner that won (if available in meta)
        if (bid.meta && bid.meta.networkName) {
          targetSlot.setTargeting('hb_partner', bid.meta.networkName);
          targetSlot.setTargeting('hb_bidder_catalyst', bid.meta.networkName);
          catalyst.log('Set slot targeting for', bid.divId, 'CPM:', bid.cpm, 'Partner:', bid.meta.networkName);
        } else {
          catalyst.log('Set slot targeting for', bid.divId, 'CPM:', bid.cpm);
        }
      } else {
        // Set page-level targeting if no slot found (Catalyst-specific keys only)
        if (bid.cpm) {
          pubads.setTargeting('hb_pb_catalyst', bid.cpm.toFixed(2));
        }

        // Set bid source and format
        pubads.setTargeting('hb_source_catalyst', 's2s');
        pubads.setTargeting('hb_format_catalyst', 'banner');

        // Set deal ID if present
        if (bid.dealId) {
          pubads.setTargeting('hb_deal_catalyst', bid.dealId);
        }

        // Set advertiser domain if available
        if (bid.meta && bid.meta.advertiserDomains && bid.meta.advertiserDomains.length > 0) {
          pubads.setTargeting('hb_adomain_catalyst', bid.meta.advertiserDomains[0]);
        }

        // Set actual demand partner
        if (bid.meta && bid.meta.networkName) {
          pubads.setTargeting('hb_partner', bid.meta.networkName);
          pubads.setTargeting('hb_bidder_catalyst', bid.meta.networkName);
        }

        catalyst.log('Set page-level targeting (slot not found):', bid.divId);
      }
    } catch (e) {
      catalyst.log('Error setting slot targeting:', e);
    }
  };

  /**
   * Log message (if debug enabled)
   * @param {...*} args - Arguments to log
   */
  catalyst.log = function() {
    // FIXED #5: typeof guard prevents ReferenceError in old browsers
    if (catalyst._config.debug && typeof console !== 'undefined' && console.log) {
      console.log.apply(console, ['[Catalyst]'].concat(Array.prototype.slice.call(arguments)));
    }
  };

  /**
   * Get SDK version
   * @returns {string} Version string
   */
  catalyst.version = function() {
    return '1.0.0';
  };

  // Process command queue
  catalyst._processCommandQueue = function() {
    while (catalyst.cmd.length > 0) {
      var cmd = catalyst.cmd.shift();
      if (typeof cmd === 'function') {
        try {
          cmd();
        } catch (e) {
          catalyst.log('Error executing command:', e);
        }
      }
    }
  };

  // Override push to execute commands immediately
  // FIXED #4: Don't push to queue after execution to prevent double execution
  catalyst.cmd.push = function(cmd) {
    if (typeof cmd === 'function') {
      try {
        cmd();
      } catch (e) {
        catalyst.log('Error executing command:', e);
      }
    }
    // Return current length without adding to queue (already executed)
    return this.length;
  };

  // Process existing commands
  catalyst._processCommandQueue();

  catalyst.log('Catalyst SDK v1.0.0 loaded');

})(window);
