//go:build cjsi_acceptance_upgrade

// TestCJSISourceIssueAcceptance maps this file into internal/store with
// `go build -overlay` and builds the upgrader beside it. It adds no migration
// step of its own: the complete mode is the production MigrateState entry,
// and the interrupt mode is the same committed managed migration through
// migrateCJSIManagedRootWithReady, with the observation pause injected only
// through its beforeReady hook (after the installed fence and index are
// verified, immediately before the ready journal and initial head commit).
package store

import (
	"context"

	"github.com/kitdine/agent-deck/internal/platform"
)

// CJSIAcceptanceUpgrade runs the committed managed migration on one root. A
// nil pause is exactly MigrateState. Otherwise pause runs in beforeReady and
// the migration continues only if pause returns.
func CJSIAcceptanceUpgrade(ctx context.Context, root string, pause func()) error {
	if pause == nil {
		return MigrateState(ctx, root)
	}
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	return migrateCJSIManagedRootWithReady(ctx, root, lockWait, cjsiStoreLegacyManager{wait: lockWait}, func(context.Context) error {
		pause()
		return nil
	})
}
