package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/platform"
)

// CJSIRuntimeRoot is the physical managed root admitted for a scan. It
// retains identities only, never a SQLite connection or a writer lock.
type CJSIRuntimeRoot struct {
	path              string
	root, core, fence cjsiFileIdentity
	epoch             int64
}

// ResolveCJSIRuntimeRoot is called under scan.lock, before any legacy opener.
// A nil result is the legacy route. Installing, inconsistent and ahead layouts
// fail closed. A first-use legacy root may still have no core.
func ResolveCJSIRuntimeRoot(ctx context.Context, path string) (managed *CJSIRuntimeRoot, err error) {
	root, err := platform.GuardStateRoot(path)
	if err != nil {
		return nil, err
	}
	st, err := privateServingPath(root, true)
	if err != nil {
		return nil, err
	}
	id := cjsiIdentity(st)
	corePath := filepath.Join(root, "agentdeck.sqlite3")
	core, err := privateServingPath(corePath, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, cjsiCheckLegacyRuntimeFiles(root)
	}
	if err != nil {
		return nil, err
	}
	if core.Nlink != 1 {
		return nil, errCJSILayoutInvalid
	}
	db, err := openLegacySQL(corePath, legacyURI(corePath, "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(0)"), "core", legacyRootID{id.device, id.inode})
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// A valid, still-empty private SQLite file is a supported legacy
	// first-open input. Merely probing it must not initialize the schema.
	var empty bool
	if err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM main.sqlite_schema LIMIT 1)`).Scan(&empty); err != nil {
		return nil, err
	}
	if empty {
		return nil, cjsiCheckLegacyRuntimeFiles(root)
	}
	// Routing never opens the index. Combined readers and writers verify its
	// contents only after selecting a session-dependent operation.
	var v int
	err = tx.QueryRowContext(ctx, "SELECT version FROM main.schema_metadata WHERE singleton=1").Scan(&v)
	if err != nil {
		return nil, err
	}
	var managedObjects bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE name IN ('cjsi_layout_journal','cjsi_publication_head'))").Scan(&managedObjects); err != nil {
		return nil, err
	}
	if v < cjsiLayoutCoreSchemaVersion && !managedObjects {
		return nil, cjsiCheckLegacyRuntimeFiles(root)
	}
	if v > cjsiLayoutCoreSchemaVersion {
		return nil, &SchemaAhead{Stored: v, Supported: cjsiLayoutCoreSchemaVersion}
	}
	if v != cjsiLayoutCoreSchemaVersion || !managedObjects {
		return nil, errCJSILayoutInvalid
	}
	j, err := cjsiReadLayoutJournal(ctx, tx)
	if err != nil {
		return nil, err
	}
	h, err := cjsiReadIndexHead(ctx, tx)
	if err != nil {
		return nil, err
	}
	if j.phase != cjsiLayoutReady || j.root != id || j.epoch != h.IndexEpoch {
		return nil, errCJSILayoutInvalid
	}
	m := &CJSIRuntimeRoot{root, id, cjsiIdentity(core), j.fence, h.CoreEpoch}
	if err := m.Check(); err != nil {
		return nil, err
	}
	return m, nil
}

func cjsiCheckLegacyRuntimeFiles(root string) error {
	if _, err := validateCJSIFence(root); err == nil {
		return errCJSILayoutInvalid
	}
	if _, err := os.Lstat(filepath.Join(root, cjsiPrivateIndexName)); !errors.Is(err, os.ErrNotExist) {
		return errCJSILayoutInvalid
	}
	return nil
}

// Check binds a long-lived daemon to its original physical installation.
func (m CJSIRuntimeRoot) Check() error {
	if err := cjsiCheckInstallRoot(m.path, m.root); err != nil {
		return err
	}
	f, err := privateServingPath(filepath.Join(m.path, "agentdeck.sqlite3"), false)
	if err != nil {
		return err
	}
	if f.Nlink != 1 || cjsiIdentity(f) != m.core {
		return errCJSILayoutInvalid
	}
	return cjsiCheckRecordedFence(m.path, m.fence)
}

// CJSIRuntimeResult carries the precomputed domain counts only after the
// verified publication commits. The owner binds them to the same delta.
type CJSIRuntimeResult struct {
	Usage                                map[string]int
	Sources, Documents, Skipped, Removed int
}

// CJSIRuntimeElection is rechecked inside every managed writer admission.
type CJSIRuntimeElection func() error
