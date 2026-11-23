package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
)

const (
	// Fixed persistent core schema versions. Quota identity (PR #79) owns 36;
	// the fenced CJSI layout owns 37 and must never drift with future ceilings.
	cjsiBaseCoreSchemaVersion   = 36
	cjsiLayoutCoreSchemaVersion = 37
	cjsiPrivateLayoutVersion    = 1
)

// Hook for the managed core migration's apply callback. The migration runner
// updates schema_metadata.version to cjsiLayoutCoreSchemaVersion in this same
// transaction. In cold bootstrap the callback runs right after base migrations.
func initializeCJSILayoutCore(ctx context.Context, tx *sql.Tx, root string) error {
	if tx == nil || !filepath.IsAbs(root) {
		return errCJSILayoutInvalid
	}
	var prior any
	if err := tx.QueryRowContext(ctx, "SELECT version FROM main.schema_metadata WHERE singleton=1").Scan(&prior); err != nil {
		return err
	}
	if prior != int64(cjsiBaseCoreSchemaVersion) {
		return ErrUnknownSchema
	}
	stat, err := privateServingPath(root, true)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, cjsiLayoutJournalSchema); err != nil {
		return err
	}
	return initializeCJSILayoutJournal(ctx, tx, cjsiIdentity(stat))
}

// The caller must hold maintenance then state locks and have drained the
// managed legacy writers, readers, and idle connections before entering.
// It supplies the core connection for this guarded physical root after the
// schema-ahead transaction commits. Locks cannot revoke an old open file
// descriptor. Readiness here records installation, with ticket selection
// and publication left to the serving layer.
func resumeCJSIProductionLayout(ctx context.Context, root string, core *sql.DB) error {
	return resumeCJSILayoutWithReady(ctx, root, core, nil)
}

func resumeCJSILayoutWithReady(ctx context.Context, root string, core *sql.DB, commitReady func(context.Context, cjsiLayoutJournal, cjsiLayoutJournal) error) error {
	if core == nil || !filepath.IsAbs(root) {
		return errCJSILayoutInvalid
	}
	version, err := schemaVersion(ctx, core)
	if err != nil {
		return err
	}
	if version != cjsiLayoutCoreSchemaVersion {
		return ErrUnknownSchema
	}

	journal := cjsiSQLiteLayoutJournal{db: core}
	privatePath := filepath.Join(root, cjsiPrivateIndexName)
	install := cjsiLayoutInstall{
		load: journal.load,
		advance: func(ctx context.Context, prior, next cjsiLayoutJournal) error {
			if next.phase == cjsiLayoutReady {
				// The resolver has returned its verification lease.
				if err := closeCJSIInstallReadPool(privatePath); err != nil {
					return err
				}
				if commitReady != nil {
					return commitReady(ctx, prior, next)
				}
			}
			return journal.advance(ctx, prior, next)
		},
		prepare:  prepareCJSIProductionIndex,
		readSeal: readCJSIProductionSeal,
	}
	err = resumeCJSILayoutInstall(ctx, root, install)
	// Also close a pool opened while rechecking ready or on failed
	// verification. Under the held locks no serving opener may race this.
	if err := errors.Join(err, closeCJSIInstallReadPool(privatePath)); err != nil {
		return err
	}
	ready, err := journal.load(ctx)
	if err != nil {
		return err
	}
	return retireCJSILegacySnapshot(ctx, root, ready)
}

// Scope verification-handle retirement to this private path and leave
// existing pools for other roots and domains alone.
func closeCJSIInstallReadPool(path string) error {
	p := &processReadPools
	p.mu.Lock()
	var closed []*sql.DB
	for key, entry := range p.entries {
		if key.path == path {
			if entry.users != 0 || entry.ready != nil {
				p.mu.Unlock()
				return ErrStateBusy
			}
		}
	}
	for key, entry := range p.entries {
		if key.path == path {
			delete(p.entries, key)
			entry.retired = true
			closed = append(closed, entry.db)
		}
	}
	p.mu.Unlock()
	var err error
	for _, db := range closed {
		err = errors.Join(err, db.Close())
	}
	return err
}

func retireCJSILegacySnapshot(ctx context.Context, root string, ready cjsiLayoutJournal) error {
	if !ready.valid() || ready.phase != cjsiLayoutReady {
		return errCJSILayoutInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cjsiCheckInstallRoot(root, ready.root); err != nil {
		return err
	}
	if fence, err := validateCJSIFence(root); err != nil || fence != ready.fence {
		return errCJSIFenceInvalid
	}
	path := filepath.Join(root, cjsiSnapshotName)
	if !ready.snapshot.present() {
		return cjsiRequireCleanIndexPath(path)
	}
	if err := cjsiRequireAbsentSidecars(path); err != nil {
		return err
	}
	if err := cjsiVerifyLayoutFile(ctx, path, ready.snapshot); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Sync even when a prior attempt unlinked the snapshot but could not
	// complete the directory sync. The ready journal keeps the evidence.
	return cjsiSyncLayoutPath(root, true)
}

// Only the scratch copy owned by recovery is mutable here. Every return
// closes its SQLite handles, including errors prior to seal commit.
func prepareCJSIProductionIndex(ctx context.Context, path string) (epoch int64, seal cjsiIndexSeal, err error) {
	if !filepath.IsAbs(path) || filepath.Base(path) != cjsiPreparedName {
		return 0, cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	stat, err := privateServingPath(path, false)
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if stat.Nlink != 1 {
		return 0, cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	if err := cjsiRequireAbsentSidecars(path); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	db, err := openWritableSQL(cjsiSQLiteURI(path, "rw")+"&_pragma=foreign_keys(1)&_pragma=temp_store(2)", "session")
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	var sealed bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.sqlite_master
		WHERE type='table' AND name='cjsi_layout_initial_seal')`).Scan(&sealed); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if sealed {
		return readPreparedCJSIProductionIndex(ctx, db)
	}

	rebuilt, err := migrateSessionSchema(ctx, db)
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if _, err := db.ExecContext(ctx, cjsiSessionBaselineSchema); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := ensureSessionServingStorage(ctx, db); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	defer tx.Rollback()
	if rebuilt {
		epoch, err = MintSessionIndexEpoch(ctx, tx)
	} else if err = ensureSessionIndexGeneration(ctx, tx); err == nil {
		err = tx.QueryRowContext(ctx, `SELECT epoch FROM main.session_index_generation WHERE singleton=1`).Scan(&epoch)
	}
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	seal = cjsiProductionSeal(epoch)
	if _, err := tx.ExecContext(ctx, cjsiLayoutSealSchema); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := initializeCJSIIndexSeal(ctx, tx, epoch, seal); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := initializeCJSIPrivateIndex(ctx, tx, seal); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := tx.Commit(); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	return epoch, seal, nil
}

func readPreparedCJSIProductionIndex(ctx context.Context, db *sql.DB) (int64, cjsiIndexSeal, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	defer tx.Rollback()
	var epoch int64
	if err := tx.QueryRowContext(ctx, `SELECT epoch FROM main.session_index_generation WHERE singleton=1`).Scan(&epoch); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	seal, err := readCJSIProductionSeal(ctx, tx)
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := tx.Commit(); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	return epoch, seal, nil
}

func readCJSIProductionSeal(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
	seal, err := readCJSIIndexSeal(ctx, tx)
	if err != nil {
		return cjsiIndexSeal{}, err
	}
	if seal.version != cjsiPrivateLayoutVersion {
		return cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	var epoch int64
	if err := tx.QueryRowContext(ctx, `SELECT epoch FROM main.session_index_generation WHERE singleton=1`).Scan(&epoch); err != nil {
		return cjsiIndexSeal{}, err
	}
	if seal != cjsiProductionSeal(epoch) {
		return cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	return seal, nil
}

func cjsiProductionSeal(epoch int64) cjsiIndexSeal {
	// Domain-separated installation marker. The journal separately binds
	// the verified file contents and identity before installation.
	material := binary.BigEndian.AppendUint64([]byte("AgentDeck/CJSI/initial-layout-seal"), cjsiPrivateLayoutVersion)
	material = binary.BigEndian.AppendUint64(material, uint64(epoch))
	return cjsiIndexSeal{version: cjsiPrivateLayoutVersion, digest: sha256.Sum256(material)}
}

// The baseline schema matches the legacy opener. The combined tree can
// share its definition once the public opener is switched to v1.
const cjsiSessionBaselineSchema = `
CREATE TABLE IF NOT EXISTS session_exclusions (kind TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(kind, value));
CREATE TABLE IF NOT EXISTS session_sources (source_path TEXT PRIMARY KEY, identity TEXT NOT NULL, cursor INTEGER NOT NULL, partial_line BLOB NOT NULL DEFAULT X'', size INTEGER NOT NULL, modified_at INTEGER NOT NULL, changed_at INTEGER NOT NULL DEFAULT 0, prefix_hash TEXT NOT NULL, priority INTEGER NOT NULL, parser_version INTEGER NOT NULL, scanned_at TEXT NOT NULL, parser_context TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS session_sources_identity ON session_sources(identity);
CREATE TABLE IF NOT EXISTS session_metadata (source_path TEXT NOT NULL REFERENCES session_sources(source_path) ON DELETE CASCADE, client TEXT NOT NULL, session_id TEXT NOT NULL, project TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', parser_version INTEGER NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL, PRIMARY KEY(source_path, client, session_id));
CREATE VIRTUAL TABLE IF NOT EXISTS session_documents USING fts5(source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, event_at UNINDEXED, kind UNINDEXED, text);
`
