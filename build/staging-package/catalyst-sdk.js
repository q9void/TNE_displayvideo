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
    serverUrl: window.location.protocol + '//' + window.location.host,
    timeout: 2800, // Client-side timeout (slightly higher than server 2500ms)
    debug: false,
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
  catalyst._syncedBidders = [];

  /**
   * Initialize Catalyst SDK
   * @param {Object} config - Configuration object
   * @param {string} config.accountId - MAI Publisher account ID
   * @param {string} [config.serverUrl] - Optional custom server URL
   * @param {number} [config.timeout] - Optional timeout in ms (default: 2800)
   * @param {boolean} [config.debug] - Enable debug logging
   */
  catalyst.init = function(config) {
    // Use provided accountId or fallback to '12345' for testing
    if (!config) {
      config = {};
    }
    catalyst._config.accountId = config.accountId || '12345';
    catalyst.log('Using accountId:', catalyst._config.accountId);

    if (config.serverUrl) {
      catalyst._config.serverUrl = config.serverUrl;
    }

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

    // Trigger user sync after initialization (with delay)
    if (catalyst._config.userSync.enabled) {
      var syncDelay = catalyst._config.userSync.syncDelay || 1000;
      setTimeout(function() {
        catalyst._performUserSync();
      }, syncDelay);
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

    catalyst.log('Requesting bids for', config.slots.length, 'slots with timeout', timeoutMs + 'ms');

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
        deviceType: catalyst._detectDeviceType(),
        userAgent: navigator.userAgent
      }
    };

    // Process slots
    for (var i = 0; i < config.slots.length; i++) {
      var slot = config.slots[i];

      if (!slot.divId || !slot.sizes || slot.sizes.length === 0) {
        catalyst.log('Warning: Invalid slot configuration, skipping:', slot);
        continue;
      }

      // Normalize sizes to [[w, h], ...] format
      var normalizedSizes = catalyst._normalizeSizes(slot.sizes);
      if (!normalizedSizes || normalizedSizes.length === 0) {
        catalyst.log('Warning: Could not normalize sizes for slot:', slot.divId);
        continue;
      }

      bidRequest.slots.push({
        divId: slot.divId,
        sizes: normalizedSizes,
        adUnitPath: slot.adUnitPath || '',
        position: slot.position || '',
        enabled_bidders: slot.enabled_bidders || ['catalyst']
      });
    }

    // Add page context if provided
    if (config.page) {
      if (config.page.keywords) {
        bidRequest.page.keywords = config.page.keywords;
      }
      if (config.page.categories) {
        bidRequest.page.categories = config.page.categories;
      }
    }

    // Add privacy/consent info if available
    if (window.__tcfapi || window.__uspapi || window.__cmp) {
      catalyst._addPrivacyConsent(bidRequest);
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

  /**
   * Make POST request to bid endpoint
   * @param {Object} bidRequest - Bid request payload
   * @param {Function} callback - Callback function(error, response)
   * @private
   */
  catalyst._makeBidRequest = function(bidRequest, callback) {
    var url = catalyst._config.serverUrl + '/v1/bid';

    var xhr = new XMLHttpRequest();
    xhr.open('POST', url, true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.timeout = catalyst._config.timeout;

    xhr.onload = function() {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          var response = JSON.parse(xhr.responseText);
          callback(null, response);
        } catch (e) {
          catalyst.log('Error parsing response:', e);
          callback(e, null);
        }
      } else {
        callback(new Error('HTTP ' + xhr.status), null);
      }
    };

    xhr.onerror = function() {
      callback(new Error('Network error'), null);
    };

    xhr.ontimeout = function() {
      callback(new Error('Request timeout'), null);
    };

    try {
      xhr.send(JSON.stringify(bidRequest));
    } catch (e) {
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
   * Add privacy consent to bid request
   * @param {Object} bidRequest - Bid request object
   * @private
   */
  catalyst._addPrivacyConsent = function(bidRequest) {
    bidRequest.user = bidRequest.user || {};

    // Try to get GDPR consent via TCF API
    if (window.__tcfapi) {
      try {
        window.__tcfapi('getTCData', 2, function(tcData, success) {
          if (success && tcData) {
            bidRequest.user.gdprApplies = tcData.gdprApplies || false;
            bidRequest.user.consentGiven = tcData.eventStatus === 'tcloaded' || tcData.eventStatus === 'useractioncomplete';
          }
        });
      } catch (e) {
        catalyst.log('Error getting GDPR consent:', e);
      }
    }

    // Try to get US Privacy string
    if (window.__uspapi) {
      try {
        window.__uspapi('getUSPData', 1, function(uspData, success) {
          if (success && uspData && uspData.uspString) {
            bidRequest.user.uspConsent = uspData.uspString;
          }
        });
      } catch (e) {
        catalyst.log('Error getting USP consent:', e);
      }
    }
  };

  /**
   * Perform user sync with configured bidders
   * @private
   */
  catalyst._performUserSync = function() {
    if (!catalyst._config.userSync.enabled) {
      catalyst.log('User sync disabled');
      return;
    }

    if (catalyst._userSyncComplete) {
      catalyst.log('User sync already performed');
      return;
    }

    // Check privacy consent before syncing
    if (!catalyst._hasPrivacyConsent()) {
      catalyst.log('User sync blocked by privacy settings');
      return;
    }

    catalyst.log('Starting user sync for bidders:', catalyst._config.userSync.bidders);

    // Build cookie sync request
    var syncRequest = {
      bidders: catalyst._config.userSync.bidders,
      gdpr: 0,
      gdpr_consent: '',
      us_privacy: '',
      limit: catalyst._config.userSync.maxSyncs
    };

    // Add privacy parameters if available
    catalyst._addPrivacyToSyncRequest(syncRequest);

    var url = catalyst._config.serverUrl + '/cookie_sync';
    var xhr = new XMLHttpRequest();
    xhr.open('POST', url, true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.timeout = 5000;

    xhr.onload = function() {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          var response = JSON.parse(xhr.responseText);
          catalyst._fireSyncPixels(response);
        } catch (e) {
          catalyst.log('Error parsing sync response:', e);
        }
      } else {
        catalyst.log('User sync request failed:', xhr.status);
      }
    };

    xhr.onerror = function() {
      catalyst.log('User sync network error');
    };

    xhr.ontimeout = function() {
      catalyst.log('User sync timeout');
    };

    try {
      xhr.send(JSON.stringify(syncRequest));
    } catch (e) {
      catalyst.log('Error sending sync request:', e);
    }

    catalyst._userSyncComplete = true;
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
   */
  catalyst._hasPrivacyConsent = function() {
    // If no consent framework present, allow sync
    if (!window.__tcfapi && !window.__uspapi) {
      return true;
    }

    // Check GDPR consent (synchronous check only)
    if (window.__tcfapi) {
      var hasConsent = true;
      try {
        window.__tcfapi('getTCData', 2, function(tcData, success) {
          if (success && tcData) {
            // Require consent if GDPR applies
            if (tcData.gdprApplies) {
              hasConsent = tcData.eventStatus === 'tcloaded' ||
                          tcData.eventStatus === 'useractioncomplete';
            }
          }
        });
      } catch (e) {
        catalyst.log('Error checking GDPR consent:', e);
      }
      return hasConsent;
    }

    // Check US Privacy consent
    if (window.__uspapi) {
      var optedOut = false;
      try {
        window.__uspapi('getUSPData', 1, function(uspData, success) {
          if (success && uspData && uspData.uspString) {
            // Check if user opted out (third character = 'Y')
            optedOut = uspData.uspString.charAt(2) === 'Y';
          }
        });
      } catch (e) {
        catalyst.log('Error checking USP consent:', e);
      }
      return !optedOut;
    }

    return true;
  };

  /**
   * Add privacy parameters to sync request
   * @param {Object} syncRequest - Sync request object to modify
   * @private
   */
  catalyst._addPrivacyToSyncRequest = function(syncRequest) {
    // Try to get GDPR consent
    if (window.__tcfapi) {
      try {
        window.__tcfapi('getTCData', 2, function(tcData, success) {
          if (success && tcData) {
            syncRequest.gdpr = tcData.gdprApplies ? 1 : 0;
            syncRequest.gdpr_consent = tcData.tcString || '';
          }
        });
      } catch (e) {
        catalyst.log('Error getting GDPR consent for sync:', e);
      }
    }

    // Try to get US Privacy string
    if (window.__uspapi) {
      try {
        window.__uspapi('getUSPData', 1, function(uspData, success) {
          if (success && uspData && uspData.uspString) {
            syncRequest.us_privacy = uspData.uspString;
          }
        });
      } catch (e) {
        catalyst.log('Error getting USP consent for sync:', e);
      }
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
      return [];
    }

    var normalized = [];

    // Handle array input
    if (Array.isArray(sizes)) {
      for (var i = 0; i < sizes.length; i++) {
        var size = sizes[i];

        // Already in [width, height] format
        if (Array.isArray(size) && size.length === 2 &&
            typeof size[0] === 'number' && typeof size[1] === 'number') {
          normalized.push(size);
        }
        // String format: "300x250"
        else if (typeof size === 'string') {
          var parsed = catalyst._parseSizeString(size);
          if (parsed) {
            normalized.push(parsed);
          }
        }
        // Single [width, height] - not nested
        else if (i === 0 && typeof size === 'number' && typeof sizes[1] === 'number') {
          // Input is [300, 250] not [[300, 250]]
          normalized.push([sizes[0], sizes[1]]);
          break; // Done processing
        }
      }
    }
    // Handle string input: "300x250"
    else if (typeof sizes === 'string') {
      var parsed = catalyst._parseSizeString(sizes);
      if (parsed) {
        normalized.push(parsed);
      }
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
    if (catalyst._config.debug && console && console.log) {
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
  catalyst.cmd.push = function(cmd) {
    if (typeof cmd === 'function') {
      try {
        cmd();
      } catch (e) {
        catalyst.log('Error executing command:', e);
      }
    }
    return Array.prototype.push.call(this, cmd);
  };

  // Process existing commands
  catalyst._processCommandQueue();

  catalyst.log('Catalyst SDK v1.0.0 loaded');

})(window);
