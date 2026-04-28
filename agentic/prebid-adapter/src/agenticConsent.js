/**
 * Page-side derivation of agent-processing consent for the IAB AAMP/ARTF
 * envelope. Mirrors agentic/consent.go on the server.
 *
 * Hard-block (returns false): COPPA. Soft-blocks: TCF Purpose 7 withheld,
 * GPP applicable section asserts opt-out. Defaults to true on bare contexts
 * — matching the bidder-path default for consistency with the server.
 */

export function deriveConsent(bidRequest) {
  if (!bidRequest) return defaultConsent();
  const out = {
    agentConsent: true,
    coppa: false,
    tcfPurposeFlags: {},
    gppSid: []
  };

  // COPPA hard-block.
  const coppa = bidRequest.ortb2 && bidRequest.ortb2.regs && bidRequest.ortb2.regs.coppa;
  if (coppa === 1) {
    out.coppa = true;
    out.agentConsent = false;
  }

  // TCF v2 — collect purpose flags; agentConsent withheld if Purpose 7 false.
  const tcf = bidRequest.gdprConsent;
  if (tcf && tcf.vendorData && tcf.vendorData.purpose && tcf.vendorData.purpose.consents) {
    out.tcfPurposeFlags = { ...tcf.vendorData.purpose.consents };
    if (tcf.gdprApplies && out.tcfPurposeFlags[7] === false) {
      out.agentConsent = false;
    }
  }

  // GPP — capture applicable sections.
  const gpp = bidRequest.gppConsent;
  if (gpp && Array.isArray(gpp.applicableSections)) {
    out.gppSid = [...gpp.applicableSections];
  }

  return out;
}

function defaultConsent() {
  return { agentConsent: true, coppa: false, tcfPurposeFlags: {}, gppSid: [] };
}
