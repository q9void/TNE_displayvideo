/**
 * Builds the page-side ortb2.ext.aamp envelope for the TNE Catalyst
 * adapter. Mirrors PRD §6.2 + §7.3.
 *
 * Soft cap 4 KB drops pageContext first, then intentHints. Hard cap 8 KB
 * drops the entire envelope (returns null → caller should not write).
 */

import { deriveConsent } from './agenticConsent.js';

export const ENV_SOFT_CAP = 4 * 1024;
export const ENV_HARD_CAP = 8 * 1024;

export function buildEnvelope(bidRequest, params) {
  const agentic = (params && params.agentic) || {};
  if (agentic.enabled === false) {
    return { disabled: true };
  }

  const originatorID =
    (params && params.publisherId) ||
    (typeof window !== 'undefined' && window.location && window.location.hostname) ||
    '';

  const env = {
    version: '1.0',
    originator: { type: 'PUBLISHER', id: String(originatorID) },
    agentConsent: true
  };

  if (Array.isArray(agentic.intentHints) && agentic.intentHints.length > 0) {
    env.intentHints = agentic.intentHints.slice();
  }
  if (Array.isArray(agentic.disclosedAgents) && agentic.disclosedAgents.length > 0) {
    env.disclosedAgents = agentic.disclosedAgents.slice();
  }
  if (agentic.pageContext && typeof agentic.pageContext === 'object') {
    try {
      // Drop non-serialisable values.
      env.pageContext = JSON.parse(JSON.stringify(agentic.pageContext));
    } catch (_) {
      // skip
    }
  }

  const consent = deriveConsent(bidRequest);
  env.agentConsent = consent.agentConsent;
  // PRD R6.4.2: never send pageContext when consent withheld.
  if (!consent.agentConsent) {
    delete env.pageContext;
  }

  return capEnvelope(env);
}

export function capEnvelope(env) {
  let body = JSON.stringify(env);
  if (body.length > ENV_HARD_CAP) {
    return null;
  }
  if (body.length > ENV_SOFT_CAP) {
    delete env.pageContext;
    body = JSON.stringify(env);
  }
  if (body.length > ENV_SOFT_CAP) {
    delete env.intentHints;
    body = JSON.stringify(env);
  }
  if (body.length > ENV_HARD_CAP) {
    return null;
  }
  return env;
}
