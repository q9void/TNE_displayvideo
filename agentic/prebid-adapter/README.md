# tne-catalyst-prebid-adapter

Prebid.js bidder adapter for TNE Catalyst with IAB AAMP/ARTF agentic support.

**Status:** scaffold only — Phase 1 ships this in-repo, not published to the
official prebid.js org repo or any private npm registry. Publishing is a
separate decision (Phase 2; PRD §10.2).

## Module dependencies

- `core` ≥ 8.0 (for `ortb2` config support)
- `currency` (existing)

## Bidder params

```javascript
bids: [{
  bidder: 'tneCatalyst',
  params: {
    publisherId: 'pub-123456',
    placement: 'homepage-banner',
    bidfloor: 1.50,
    agentic: {                              // all fields optional
      enabled: true,                        // default true
      tmaxMs: 30,                           // suggested agent budget
      intentHints: ['ACTIVATE_SEGMENTS'],   // hints to server
      disclosedAgents: ['seg.example.com'], // page-side allow-list
      pageContext: { articleTopic: 'sports.nfl' }
    }
  }
}]
```

`agentic` is purely additive. Existing integrations that omit it work
unchanged. When `agentic.enabled === false`, the adapter writes
`ortb2.ext.aamp.disabled = true` so the SSP hard-skips the agent path
for that auction.

## Response decoration

Server returns mutation attribution at `seatbid[].bid[].ext.aamp`:

```javascript
bid.ext.aamp = {
  agentsApplied: [
    { agent_id: 'seg.example.com', intents: ['ACTIVATE_SEGMENTS'], mutation_count: 3 }
  ],
  bidShadingDelta: -0.12,    // present iff BID_SHADE applied
  segmentsActivated: 12
}
```

The adapter copies this onto Prebid's standard `bid.meta.aamp` so analytics
adapters can read it without bidder coupling. Defensive-check — old server
responses carry no `ext.aamp`.

## Consent

`src/agenticConsent.js` mirrors the server-side consent derivation in
`agentic/consent.go`. COPPA hard-blocks. TCF Purpose 7 withheld and GPP
opt-out soft-block. When consent is withheld, `pageContext` is dropped
from the envelope but `originator` and `intentHints` still flow (no PII).

## Layout

```
src/
  tneCatalystBidAdapter.js          main adapter (spec object)
  tneCatalystAgenticEnvelope.js     ext.aamp builder + size caps
  agenticConsent.js                 consent derivation
test/
  spec/
    agenticEnvelope_spec.js
    agenticConsent_spec.js
examples/                           pbjs configs (TBD)
```

## Tests

```bash
cd agentic/prebid-adapter
npm install
npm test
```

Tests run under Mocha + Chai under Node — no Karma harness in Phase 1.

## Versioning

This scaffold is `2.0.0`:
- **Breaking** on response shape: `bid.meta.aamp` is a new field analytics
  adapters must defensive-check. Old adapters and old servers continue to
  work; the field is simply absent.
- **Non-breaking** on request shape: `params.agentic` is purely additive.

## Related

- Server-side PRD: `docs/superpowers/specs/2026-04-27-iab-agentic-protocol-integration-design.md`
- Implementation plan: `docs/superpowers/plans/2026-04-27-iab-agentic-protocol-integration.md`
- Server-side consent helper: `agentic/consent.go`
- Server-side envelope helpers: `agentic/envelope.go`
