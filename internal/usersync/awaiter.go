package usersync

import (
	"context"
	"sync"
	"time"
)

// SyncAwaiter lets a bid request wait briefly for in-flight setuid callbacks
// to land in the same server process. Works in-process only; no cross-instance
// coordination. Entries are cleaned up automatically after Signal or timeout.
type SyncAwaiter struct {
	mu      sync.Mutex
	pending map[string][]chan struct{}
}

func NewSyncAwaiter() *SyncAwaiter {
	return &SyncAwaiter{pending: make(map[string][]chan struct{})}
}

// Wait registers a channel for fpid and blocks until Signal is called or
// timeout/ctx expires. Returns true if a sync was signaled, false on timeout.
func (a *SyncAwaiter) Wait(ctx context.Context, fpid string, timeout time.Duration) bool {
	ch := make(chan struct{}, 1)
	a.mu.Lock()
	a.pending[fpid] = append(a.pending[fpid], ch)
	a.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	defer a.remove(fpid, ch)

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Signal wakes all goroutines waiting on fpid (e.g. after a setuid DB write).
func (a *SyncAwaiter) Signal(fpid string) {
	a.mu.Lock()
	chans := a.pending[fpid]
	delete(a.pending, fpid)
	a.mu.Unlock()
	for _, ch := range chans {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (a *SyncAwaiter) remove(fpid string, target chan struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	chans := a.pending[fpid]
	for i, ch := range chans {
		if ch == target {
			a.pending[fpid] = append(chans[:i], chans[i+1:]...)
			break
		}
	}
	if len(a.pending[fpid]) == 0 {
		delete(a.pending, fpid)
	}
}
