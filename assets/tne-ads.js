/**
 * TNE Catalyst Ad SDK
 * Client-side JavaScript SDK for direct ad tag integration
 * @version 1.0.0
 */

(function(window) {
  'use strict';

  // TNE namespace
  var tne = window.tne || {};
  window.tne = tne;

  // Command queue
  tne.cmd = tne.cmd || [];
  tne._commands = tne._commands || [];

  // Configuration
  tne.config = {
    serverUrl: 'https://ads.thenexusengine.com',
    timeout: 2000,
    debug: false
  };

  // Active ad slots
  tne._slots = {};

  /**
   * Display an ad
   * @param {Object} options - Ad display options
   */
  tne.display = function(options) {
    if (!options || !options.divId) {
      tne.log('Error: divId is required');
      return;
    }

    var slot = {
      divId: options.divId,
      publisherId: options.publisherId || '',
      placementId: options.placementId || '',
      size: options.size || [300, 250],
      serverUrl: options.serverUrl || tne.config.serverUrl,
      pageUrl: options.pageUrl || window.location.href,
      domain: options.domain || window.location.hostname,
      keywords: options.keywords || [],
      customData: options.customData || {},
      refreshRate: options.refreshRate || 0
    };

    tne._slots[slot.divId] = slot;

    // Request ad
    tne.requestAd(slot);

    // Setup auto-refresh if configured
    // FIXED #6: Store interval ID so it can be cleared in destroySlot()
    if (slot.refreshRate > 0) {
      slot.refreshInterval = setInterval(function() {
        tne.refreshAd(slot.divId);
      }, slot.refreshRate * 1000);
    }
  };

  /**
   * Request an ad for a slot
   * @param {Object} slot - Ad slot configuration
   */
  tne.requestAd = function(slot) {
    var container = document.getElementById(slot.divId);
    if (!container) {
      tne.log('Error: Container not found:', slot.divId);
      return;
    }

    // Show loading state
    container.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100%;color:#999;">Loading...</div>';

    // Build request URL
    var url = slot.serverUrl + '/ad/js?' +
      'pub=' + encodeURIComponent(slot.publisherId) +
      '&placement=' + encodeURIComponent(slot.placementId) +
      '&div=' + encodeURIComponent(slot.divId) +
      '&w=' + slot.size[0] +
      '&h=' + slot.size[1] +
      '&url=' + encodeURIComponent(slot.pageUrl) +
      '&domain=' + encodeURIComponent(slot.domain);

    // Add keywords
    if (slot.keywords.length > 0) {
      url += '&kw=' + encodeURIComponent(slot.keywords.join(','));
    }

    // Add custom data
    for (var key in slot.customData) {
      if (slot.customData.hasOwnProperty(key)) {
        url += '&' + encodeURIComponent(key) + '=' + encodeURIComponent(slot.customData[key]);
      }
    }

    // Load ad script
    var script = document.createElement('script');
    script.src = url;
    script.async = true;
    script.onerror = function() {
      tne.log('Error: Failed to load ad for', slot.divId);
      container.innerHTML = '';
      container.style.display = 'none';
    };

    document.body.appendChild(script);
  };

  /**
   * Refresh an ad slot
   * @param {string} divId - Container div ID
   */
  tne.refreshAd = function(divId) {
    var slot = tne._slots[divId];
    if (!slot) {
      tne.log('Error: Slot not found:', divId);
      return;
    }

    tne.log('Refreshing ad:', divId);
    tne.requestAd(slot);
  };

  /**
   * Track impression
   * @param {string} bidId - Bid ID
   * @param {string} placementId - Placement ID
   */
  tne.trackImpression = function(bidId, placementId) {
    var url = tne.config.serverUrl + '/ad/track?' +
      'bid=' + encodeURIComponent(bidId) +
      '&placement=' + encodeURIComponent(placementId) +
      '&event=impression' +
      '&ts=' + Date.now();

    var img = new Image();
    img.src = url;

    tne.log('Tracked impression:', bidId, placementId);
  };

  /**
   * Track click
   * @param {string} bidId - Bid ID
   * @param {string} placementId - Placement ID
   */
  tne.trackClick = function(bidId, placementId) {
    var url = tne.config.serverUrl + '/ad/track?' +
      'bid=' + encodeURIComponent(bidId) +
      '&placement=' + encodeURIComponent(placementId) +
      '&event=click' +
      '&ts=' + Date.now();

    var img = new Image();
    img.src = url;

    tne.log('Tracked click:', bidId, placementId);
  };

  /**
   * Log message (if debug enabled)
   * @param {...*} args - Arguments to log
   */
  tne.log = function() {
    if (tne.config.debug && console && console.log) {
      console.log.apply(console, ['[TNE]'].concat(Array.prototype.slice.call(arguments)));
    }
  };

  /**
   * Set configuration
   * @param {Object} config - Configuration object
   */
  tne.setConfig = function(config) {
    for (var key in config) {
      if (config.hasOwnProperty(key)) {
        tne.config[key] = config[key];
      }
    }
  };

  /**
   * Get slot information
   * @param {string} divId - Container div ID
   * @returns {Object} Slot configuration
   */
  tne.getSlot = function(divId) {
    return tne._slots[divId] || null;
  };

  /**
   * Destroy ad slot
   * @param {string} divId - Container div ID
   */
  tne.destroySlot = function(divId) {
    // FIXED #6: Clear refresh interval to prevent memory leak
    var slot = tne._slots[divId];
    if (slot && slot.refreshInterval) {
      clearInterval(slot.refreshInterval);
      slot.refreshInterval = null;
    }

    var container = document.getElementById(divId);
    if (container) {
      container.innerHTML = '';
    }
    delete tne._slots[divId];
  };

  // Process command queue
  tne._processCommandQueue = function() {
    while (tne.cmd.length > 0) {
      var cmd = tne.cmd.shift();
      if (typeof cmd === 'function') {
        try {
          cmd();
        } catch (e) {
          tne.log('Error executing command:', e);
        }
      }
    }
  };

  // Override push to execute commands immediately
  tne.cmd.push = function(cmd) {
    if (typeof cmd === 'function') {
      try {
        cmd();
      } catch (e) {
        tne.log('Error executing command:', e);
      }
    }
    return Array.prototype.push.call(this, cmd);
  };

  // Process existing commands
  tne._processCommandQueue();

  // Ready callback
  if (typeof tne.onReady === 'function') {
    tne.onReady();
  }

  tne.log('TNE Catalyst Ad SDK loaded');

})(window);
