package storage

import (
	"context"
	"time"
)

// DefaultDBTimeout is the default timeout for database operations
// Reduced from 5s to 500ms to fail fast and expose slow queries
const DefaultDBTimeout = 500 * time.Millisecond

// withTimeout wraps a context with a default timeout if it doesn't already have a deadline
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	// Check if context already has a deadline
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		// Return context as-is with a no-op cancel function
		return ctx, func() {}
	}

	// Add default timeout
	return context.WithTimeout(ctx, timeout)
}
