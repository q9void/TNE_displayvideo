// Build tag isolates this shim from the regular build. It only exists to
// expose unexported curated-deals plumbing to the e2e_curator_test in the
// _test package next door — same package's *_test.go files would have
// implicit access, but we want the e2e test to live in `package
// exchange_test` so it can import this package the same way external
// callers (cmd/server) do.
//go:build e2e
// +build e2e

package exchange

import (
	"context"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// HydrateForTest is a thin wrapper around Exchange.hydrateCuratedDealsFor for
// integration tests. Returns the per-auction CuratorContext exactly as the
// real auction loop would build it.
func HydrateForTest(e *Exchange, ctx context.Context, req *openrtb.BidRequest, publisherDBID int) *CuratorContext {
	return e.hydrateCuratedDealsFor(ctx, req, publisherDBID)
}
