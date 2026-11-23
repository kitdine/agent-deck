package store

import (
	"context"
	"database/sql"
	"time"
)

// WithCJSIManagedCoreForTest exposes the real managed maintenance sequence
// (scan lock -> state lock -> physical-root drain -> managed core open) for
// external store_test cross-package regressions without widening production APIs.
func WithCJSIManagedCoreForTest(ctx context.Context, root string, wait time.Duration, run func(string, *sql.DB) error) error {
	return withCJSIManagedCore(ctx, root, wait, cjsiStoreLegacyManager{wait: wait}, run)
}

// DrainLegacyForTest exposes cjsiStoreLegacyManager.drainLegacy for external
// store_test cross-package regressions.
func DrainLegacyForTest(ctx context.Context, root string, wait time.Duration) (func() error, error) {
	return (cjsiStoreLegacyManager{wait: wait}).drainLegacy(ctx, root)
}
