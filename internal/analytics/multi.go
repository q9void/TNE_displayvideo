package analytics

import (
	"context"
	"fmt"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// MultiModule broadcasts analytics to multiple adapters
// Errors from one adapter don't affect others (fail-independently pattern)
type MultiModule struct {
	modules []Module
}

// NewMultiModule creates a new multi-module broadcaster
func NewMultiModule(modules ...Module) *MultiModule {
	return &MultiModule{modules: modules}
}

// LogAuctionObject broadcasts auction data to all modules
// Non-blocking: errors from one adapter don't affect others
func (m *MultiModule) LogAuctionObject(ctx context.Context, auction *AuctionObject) error {
	for _, module := range m.modules {
		if err := module.LogAuctionObject(ctx, auction); err != nil {
			logger.Log.Warn().
				Err(err).
				Str("module", fmt.Sprintf("%T", module)).
				Str("auction_id", auction.AuctionID).
				Msg("Analytics module failed to log auction")
		}
	}
	return nil
}

// LogVideoObject broadcasts video data to all modules
func (m *MultiModule) LogVideoObject(ctx context.Context, video *VideoObject) error {
	for _, module := range m.modules {
		if err := module.LogVideoObject(ctx, video); err != nil {
			logger.Log.Warn().
				Err(err).
				Str("module", fmt.Sprintf("%T", module)).
				Str("auction_id", video.AuctionID).
				Msg("Analytics module failed to log video")
		}
	}
	return nil
}

// Shutdown gracefully shuts down all modules
// Collects all errors and returns them
func (m *MultiModule) Shutdown() error {
	var errs []error
	for _, module := range m.modules {
		if err := module.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
