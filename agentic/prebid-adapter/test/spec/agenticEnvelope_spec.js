import { expect } from 'chai';
import { buildEnvelope, capEnvelope, ENV_SOFT_CAP, ENV_HARD_CAP } from '../../src/tneCatalystAgenticEnvelope.js';

describe('tneCatalystAgenticEnvelope', () => {
  it('writes an envelope with PUBLISHER originator', () => {
    const env = buildEnvelope({}, { publisherId: 'pub-123' });
    expect(env.version).to.equal('1.0');
    expect(env.originator).to.deep.equal({ type: 'PUBLISHER', id: 'pub-123' });
    expect(env.agentConsent).to.equal(true);
  });

  it('respects params.agentic.enabled = false', () => {
    const env = buildEnvelope({}, { publisherId: 'p', agentic: { enabled: false } });
    expect(env).to.deep.equal({ disabled: true });
  });

  it('forwards intentHints + disclosedAgents when provided', () => {
    const env = buildEnvelope({}, {
      publisherId: 'p',
      agentic: {
        intentHints: ['ACTIVATE_SEGMENTS'],
        disclosedAgents: ['seg.example.com']
      }
    });
    expect(env.intentHints).to.deep.equal(['ACTIVATE_SEGMENTS']);
    expect(env.disclosedAgents).to.deep.equal(['seg.example.com']);
  });

  it('drops pageContext when COPPA blocks agentConsent', () => {
    const env = buildEnvelope({
      ortb2: { regs: { coppa: 1 } }
    }, {
      publisherId: 'p',
      agentic: { pageContext: { topic: 'kids' } }
    });
    expect(env.agentConsent).to.equal(false);
    expect(env.pageContext).to.equal(undefined);
  });

  it('drops pageContext under soft cap', () => {
    const env = {
      version: '1.0',
      originator: { type: 'PUBLISHER', id: 'p' },
      agentConsent: true,
      pageContext: { huge: 'x'.repeat(ENV_SOFT_CAP) }
    };
    const out = capEnvelope(env);
    expect(out).to.not.equal(null);
    expect(out.pageContext).to.equal(undefined);
  });

  it('drops entire envelope above hard cap', () => {
    const env = {
      version: '1.0',
      originator: { type: 'PUBLISHER', id: 'p' },
      agentConsent: true,
      // The huge field cannot be dropped (not pageContext / intentHints) so
      // the cap logic must drop the whole envelope.
      bogus: 'y'.repeat(ENV_HARD_CAP + 100)
    };
    const out = capEnvelope(env);
    expect(out).to.equal(null);
  });
});
