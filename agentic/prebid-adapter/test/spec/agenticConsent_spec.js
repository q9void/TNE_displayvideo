import { expect } from 'chai';
import { deriveConsent } from '../../src/agenticConsent.js';

describe('agenticConsent', () => {
  it('returns true on bare bidRequest', () => {
    expect(deriveConsent({}).agentConsent).to.equal(true);
  });

  it('hard-blocks COPPA', () => {
    const c = deriveConsent({ ortb2: { regs: { coppa: 1 } } });
    expect(c.agentConsent).to.equal(false);
    expect(c.coppa).to.equal(true);
  });

  it('soft-blocks when TCF Purpose 7 withheld', () => {
    const c = deriveConsent({
      gdprConsent: {
        gdprApplies: true,
        vendorData: { purpose: { consents: { 1: true, 7: false } } }
      }
    });
    expect(c.agentConsent).to.equal(false);
    expect(c.tcfPurposeFlags[1]).to.equal(true);
  });

  it('captures GPP applicableSections', () => {
    const c = deriveConsent({ gppConsent: { applicableSections: [7, 8] } });
    expect(c.gppSid).to.deep.equal([7, 8]);
  });
});
