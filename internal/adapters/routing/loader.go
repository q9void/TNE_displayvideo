package routing

import (
	"context"
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const cacheTTL = 30 * time.Second

// Loader caches bidder_field_rules from the DB.
// Rules are refreshed on TTL expiry or explicit Invalidate call.
type Loader struct {
	store     *storage.PublisherStore
	mu        sync.RWMutex
	cache     map[string][]storage.BidderFieldRule // bidder_code → merged rules
	fetchedAt time.Time
}

// NewLoader creates a Loader backed by the given store.
func NewLoader(store *storage.PublisherStore) *Loader {
	return &Loader{store: store, cache: make(map[string][]storage.BidderFieldRule)}
}

// Get returns the merged rules for bidderCode (bidder-specific + __default__).
// Refreshes the cache if stale.
func (l *Loader) Get(ctx context.Context, bidderCode string) []storage.BidderFieldRule {
	l.mu.RLock()
	stale := time.Since(l.fetchedAt) > cacheTTL
	l.mu.RUnlock()

	if stale {
		l.refresh(ctx)
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cache[bidderCode]
}

// Invalidate clears the cache so the next Get re-fetches from DB.
func (l *Loader) Invalidate(_ string) {
	l.mu.Lock()
	l.fetchedAt = time.Time{}
	l.mu.Unlock()
}

func (l *Loader) refresh(ctx context.Context) {
	all, err := l.store.GetAllBidderFieldRules(ctx)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("routing.Loader: failed to refresh rules")
		return
	}

	// Group by bidder_code
	byBidder := make(map[string][]storage.BidderFieldRule)
	defaults := make([]storage.BidderFieldRule, 0)
	for _, r := range all {
		if r.BidderCode == "__default__" {
			defaults = append(defaults, r)
		} else {
			byBidder[r.BidderCode] = append(byBidder[r.BidderCode], r)
		}
	}

	// Merge: bidder-specific wins over defaults for same field_path
	merged := make(map[string][]storage.BidderFieldRule)
	for code, bidderRules := range byBidder {
		seen := make(map[string]bool)
		var m []storage.BidderFieldRule
		for _, r := range bidderRules {
			seen[r.FieldPath] = true
			m = append(m, r)
		}
		for _, d := range defaults {
			if !seen[d.FieldPath] {
				m = append(m, d)
			}
		}
		merged[code] = m
	}
	merged["__default__"] = defaults

	l.mu.Lock()
	l.cache = merged
	l.fetchedAt = time.Now()
	l.mu.Unlock()
}
