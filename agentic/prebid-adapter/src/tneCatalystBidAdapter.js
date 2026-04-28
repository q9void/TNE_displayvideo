/**
 * TNE Catalyst Prebid.js bidder adapter with IAB AAMP/ARTF agentic support.
 *
 * Status: scaffold. Not published to the official prebid.js org repo.
 * See agentic/prebid-adapter/README.md.
 *
 * Adapter version 2.0.0 — breaking on response-shape decoration (bid.meta.aamp
 * is a new field analytics adapters must defensive-check), non-breaking on
 * request shape (params.agentic is purely additive).
 */

import { buildEnvelope } from './tneCatalystAgenticEnvelope.js';

const BIDDER_CODE = 'tneCatalyst';
const ENDPOINT_URL = 'https://ads.thenexusengine.com/openrtb2/auction';

export const spec = {
  code: BIDDER_CODE,
  supportedMediaTypes: ['banner', 'native', 'video'],

  isBidRequestValid(bid) {
    return Boolean(bid && bid.params && bid.params.publisherId);
  },

  buildRequests(validBidRequests, bidderRequest) {
    if (!validBidRequests || validBidRequests.length === 0) {
      return [];
    }

    // Build a minimal OpenRTB 2.x BidRequest. Real adapters delegate this
    // to prebid's converter library; the scaffold keeps it inline so the
    // ext.aamp wiring is obvious to reviewers.
    const ortb2 = (bidderRequest && bidderRequest.ortb2) || {};
    const ext = ortb2.ext || {};

    // Use the first bid's params as the canonical agentic params source.
    // Production adapters should aggregate per-imp params — out of scope
    // for the scaffold.
    const params = (validBidRequests[0] && validBidRequests[0].params) || {};
    const aamp = buildEnvelope(bidderRequest, params);
    if (aamp) {
      ext.aamp = aamp;
    }

    const payload = {
      id: bidderRequest && bidderRequest.bidderRequestId,
      imp: validBidRequests.map((b) => ({
        id: b.bidId,
        tagid: b.params.placement,
        bidfloor: b.params.bidfloor,
        ext: { bidder: b.params }
      })),
      ext
    };

    return [{
      method: 'POST',
      url: ENDPOINT_URL,
      data: JSON.stringify(payload),
      options: { contentType: 'application/json', withCredentials: true }
    }];
  },

  interpretResponse(serverResponse, request) {
    const body = serverResponse && serverResponse.body;
    if (!body || !Array.isArray(body.seatbid)) {
      return [];
    }
    const bids = [];
    body.seatbid.forEach((sb) => {
      (sb.bid || []).forEach((b) => {
        const bid = {
          requestId: b.impid,
          cpm: b.price,
          width: b.w,
          height: b.h,
          creativeId: b.crid,
          dealId: b.dealid,
          currency: body.cur || 'USD',
          netRevenue: true,
          ttl: 300,
          ad: b.adm,
          meta: {}
        };
        // PRD R6.3.1: surface server-side agent attribution onto bid.meta.aamp.
        if (b.ext && b.ext.aamp) {
          bid.meta.aamp = b.ext.aamp;
        }
        bids.push(bid);
      });
    });
    return bids;
  },

  // Cookie sync — defer to existing /cookie_sync endpoint.
  getUserSyncs(syncOptions) {
    if (syncOptions && syncOptions.iframeEnabled) {
      return [{ type: 'iframe', url: 'https://ads.thenexusengine.com/cookie_sync' }];
    }
    return [];
  }
};

// In real Prebid integration this would be:
//   import { registerBidder } from '../src/adapters/bidderFactory.js';
//   registerBidder(spec);
// We export `spec` so unit tests can exercise it without the full Prebid runtime.
