//go:build cjsi_adapter_fixture

// Map this fixture into internal/store only for the physical session reader
// tests. Only installation needs package-private access; no reader is mocked.
package store

import (
	"context"
	"time"
)

func InstallTask5Fixture(ctx context.Context, root string) error {
	return migrateCJSIManagedRoot(ctx, root, time.Second, cjsiStoreLegacyManager{time.Second})
}
