package bidcache

import (
	"sync"
	"time"
)

const defaultTTL = 10 * time.Minute

// BidCache stores winning bid ad markup keyed by bid ID.
// Entries expire after TTL — the GAM creative fires within seconds of the auction.
type BidCache struct {
	mu      sync.RWMutex
	entries map[string]*entry
}

type entry struct {
	adMarkup  string
	expiresAt time.Time
}

// New creates a BidCache with a background cleanup goroutine.
func New() *BidCache {
	c := &BidCache{
		entries: make(map[string]*entry),
	}
	go c.reap()
	return c
}

// Store saves ad markup for the given bid ID with a 10-minute TTL.
func (c *BidCache) Store(bidID, adMarkup string) {
	if bidID == "" || adMarkup == "" {
		return
	}
	c.mu.Lock()
	c.entries[bidID] = &entry{adMarkup: adMarkup, expiresAt: time.Now().Add(defaultTTL)}
	c.mu.Unlock()
}

// Get returns the ad markup for the given bid ID, or ("", false) if not found or expired.
func (c *BidCache) Get(bidID string) (string, bool) {
	c.mu.RLock()
	e, ok := c.entries[bidID]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.adMarkup, true
}

// reap removes expired entries every minute.
func (c *BidCache) reap() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for id, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, id)
			}
		}
		c.mu.Unlock()
	}
}
