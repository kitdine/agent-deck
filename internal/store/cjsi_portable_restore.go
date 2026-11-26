package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kitdine/agent-deck/internal/platform"
)

// BackupPublishedCore snapshots the physical core database bound to the
// caller's WithPublishedCore transaction and verifies the snapshot carries
// the same committed publication head. Callers must hold scan.lock then
// state.lock and validate the surrounding WithPublishedCore borrow.
func (m CJSIRuntimeRoot) BackupPublishedCore(ctx context.Context, tx *sql.Tx, destination string) error {
	if tx == nil || destination == "" {
		return ErrCJSIIndexChanged
	}
	if err := m.Check(); err != nil {
		return err
	}
	corePath := filepath.Join(m.path, "agentdeck.sqlite3")
	var actual string
	if err := tx.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&actual); err != nil || actual != corePath {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	head, err := cjsiReadIndexHead(ctx, tx)
	if err != nil {
		return err
	}
	if head.CoreEpoch != m.epoch {
		return ErrCJSIIndexChanged
	}
	if err := BackupSQLiteFile(ctx, corePath, destination); err != nil {
		return err
	}
	snap, err := sql.Open("sqlite", cjsiSQLiteURI(destination, "ro")+"&_pragma=query_only(1)")
	if err != nil {
		_ = os.Remove(destination)
		return err
	}
	snap.SetMaxOpenConns(1)
	snapHead, readErr := cjsiReadIndexHead(ctx, snap)
	closeErr := snap.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		_ = os.Remove(destination)
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	if snapHead != head {
		_ = os.Remove(destination)
		return ErrCJSIIndexChanged
	}
	return m.Check()
}

// ValidateCJSICoreArchive verifies that a decoded schema-37 core archive
// matches the canonical CJSI layout schema, passes integrity and foreign-key
// checks, and contains no unopened publication attempts or transient worksets.
func ValidateCJSICoreArchive(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errCJSILayoutInvalid
	}
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "ro")+"&_pragma=query_only(1)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version != cjsiLayoutCoreSchemaVersion {
		return ErrUnknownSchema
	}
	refs, err := validatedCJSISchemaReferences(ctx, CurrentSchemaVersion, migrations)
	if err != nil {
		return err
	}
	if err := verifyCJSICoreSchema(ctx, db, refs.layout37); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	hasFKViolation := rows.Next()
	if err := errors.Join(rows.Err(), rows.Close()); err != nil || hasFKViolation {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	var valid bool
	if err := db.QueryRowContext(ctx, `SELECT
		NOT EXISTS(SELECT 1 FROM main.cjsi_checkpoint_attempts)
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_checkpoint_records)
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_publication_attempts WHERE state<>'opened')
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection s
			LEFT JOIN main.cjsi_usage_groups g USING(ticket,attempt,thread)
			LEFT JOIN main.cjsi_publication_attempts a USING(ticket,attempt)
			WHERE g.ticket IS NULL OR a.state IS NOT 'opened' OR a.diff_digest IS NULL
			   OR s.ticket>(SELECT ticket FROM main.cjsi_publication_head WHERE singleton=1))
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_takeovers t
			LEFT JOIN main.cjsi_usage_groups g ON g.ticket=t.opened_ticket AND g.attempt=t.attempt AND g.thread=t.thread
			LEFT JOIN main.cjsi_publication_attempts a ON a.ticket=t.opened_ticket AND a.attempt=t.attempt
			WHERE g.ticket IS NULL OR a.state IS NOT 'opened' OR a.diff_digest IS NULL)
		AND ((SELECT ticket FROM main.cjsi_publication_head WHERE singleton=1)=0
			OR 1=(SELECT count(*) FROM main.cjsi_publication_attempts
				WHERE ticket=(SELECT ticket FROM main.cjsi_publication_head WHERE singleton=1)
				  AND state='opened' AND diff_digest IS NOT NULL))`).Scan(&valid); err != nil || !valid {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	return nil
}

// OpenOwnedStateRestoreMarker pins the marker bound to these exact held locks.
// A failed Mark can leave either a complete binding or an uncertain marker.
// Callers own the returned descriptor and must recheck the name before Clear.
func OpenOwnedStateRestoreMarker(root string, scanLock, stateLock *Lock) (*os.File, error) {
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return nil, err
	}
	if scanLock == nil || stateLock == nil || scanLock.path != filepath.Join(root, "scan.lock") || stateLock.path != filepath.Join(root, "state.lock") {
		return nil, ErrLockLost
	}
	scanLock.mu.Lock()
	defer scanLock.mu.Unlock()
	stateLock.mu.Lock()
	defer stateLock.mu.Unlock()
	scanEntry, err := verifyRestoreLock(scanLock, root, "scan.lock")
	if err != nil {
		return nil, err
	}
	stateEntry, err := verifyRestoreLock(stateLock, root, "state.lock")
	if err != nil {
		return nil, err
	}
	marker, err := cjsiOpenPrivateLayoutPath(filepath.Join(root, stateRestoreMarkerName), false)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(io.LimitReader(marker, 2*maxLockTokenBytes+3))
	if err != nil || string(contents) != restoreLockBinding(scanEntry.token, stateEntry.token) {
		return nil, errors.Join(ErrLockLost, err, marker.Close())
	}
	return marker, nil
}

// InstallRestoredCJSILayout installs a validated schema-37 core (and optional
// portable session index placed at targetRoot/sessions.sqlite3) into an empty
// target root. Callers must hold scan.lock then state.lock and have already
// validated the archive and empty target before calling. The supplied file
// information comes from the archive files' creation descriptors. Record
// receives each additional target file's identity before it can be used.
func InstallRestoredCJSILayout(ctx context.Context, targetRoot string, scanLock, stateLock *Lock, core, sessions os.FileInfo, record func(string, os.FileInfo) error, configureCore func(*Store) error, afterProbe func(string)) (err error) {
	targetRoot, err = platform.GuardStateRoot(targetRoot)
	if err != nil {
		return err
	}
	stat, err := privateServingPath(targetRoot, true)
	if err != nil {
		return err
	}
	corePath := filepath.Join(targetRoot, "agentdeck.sqlite3")
	if err := checkRestoredLayoutFile(corePath, core); err != nil {
		return err
	}
	if err := cjsiRequireAbsentSidecars(corePath); err != nil {
		return err
	}
	// Every writable SQLite handle belongs to a fresh scratch directory.
	// In particular, SQLite cannot unlink a foreign core sidecar on close.
	scratch, err := os.MkdirTemp(targetRoot, ".cjsi-restore-")
	if err != nil {
		return err
	}
	scratchInfo, err := os.Lstat(scratch)
	if err != nil {
		return err
	}
	type scratchEntry struct {
		path string
		info os.FileInfo
	}
	var scratchFiles []scratchEntry
	recordScratch := func(path string, info os.FileInfo) error {
		scratchFiles = append(scratchFiles, scratchEntry{path, info})
		return nil
	}
	defer func() {
		info, statErr := os.Lstat(scratch)
		if errors.Is(statErr, os.ErrNotExist) {
			return
		}
		if statErr != nil || !info.IsDir() || !os.SameFile(scratchInfo, info) {
			err = errors.Join(err, errCJSILayoutInvalid, statErr)
			return
		}
		for _, entry := range scratchFiles {
			// Successful publication already unlinked the scratch name.
			if removeErr := unlinkRestoredLayoutFile(entry.path, entry.info); !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
		if removeErr := syscall.Rmdir(scratch); !errors.Is(removeErr, os.ErrNotExist) &&
			!errors.Is(removeErr, syscall.ENOTEMPTY) && !errors.Is(removeErr, syscall.EEXIST) {
			err = errors.Join(err, removeErr)
		}
	}()
	stagedCore := filepath.Join(scratch, filepath.Base(corePath))
	if err := copyRestoredLayoutFile(corePath, stagedCore, core, recordScratch); err != nil {
		return err
	}

	db, err := openWritableSQL(legacyURI(stagedCore, "mode=rw&"+writablePragmas+"&_pragma=synchronous(FULL)"), "core")
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, db.Close())
		}
	}()
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return err
	}
	if journalMode != "wal" {
		return errCJSILayoutInvalid
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version != cjsiLayoutCoreSchemaVersion {
		return ErrUnknownSchema
	}
	refs, err := validatedCJSISchemaReferences(ctx, CurrentSchemaVersion, migrations)
	if err != nil {
		return err
	}
	if err := verifyCJSICoreSchema(ctx, db, refs.layout37); err != nil {
		return err
	}
	if configureCore != nil {
		if err := configureCore(&Store{DB: db, path: stagedCore}); err != nil {
			return err
		}
	}
	if err := checkRestoredLayoutFile(corePath, core); err != nil {
		return err
	}
	baseTicket, err := rebindRestoredCJSICore(ctx, db, cjsiIdentity(stat))
	if err != nil {
		return err
	}
	if err := installFreshRestoredLayout(ctx, targetRoot, scratch, db, baseTicket, sessions, record, recordScratch); err != nil {
		return err
	}

	if err := cjsiCheckpointLegacy(ctx, stagedCore); err != nil {
		return err
	}
	closed = true
	if err := db.Close(); err != nil {
		return err
	}
	if err := cjsiRequireAbsentSidecars(stagedCore); err != nil {
		return err
	}
	if err := cjsiSyncLayoutPath(stagedCore, false); err != nil {
		return err
	}
	var reservedSidecars [len(cjsiSidecarSuffixes)]os.FileInfo
	for index, suffix := range cjsiSidecarSuffixes {
		info, err := createRestoredLayoutFile(corePath+suffix, nil, record)
		if err != nil {
			return err
		}
		reservedSidecars[index] = info
	}
	if err := unlinkRestoredLayoutFile(corePath, core); err != nil {
		return err
	}
	if err := publishRestoredLayoutFile(stagedCore, corePath, record); err != nil {
		return err
	}
	if err := cjsiSyncLayoutPath(targetRoot, true); err != nil {
		return err
	}
	managed, err := ResolveCJSIRuntimeRootForRestore(ctx, targetRoot, scanLock, stateLock)
	if err != nil || managed == nil {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	if afterProbe != nil {
		afterProbe(targetRoot)
	}
	// The probe has closed its handles and the restore still holds both
	// locks. Remove only the sidecars reserved by this invocation.
	for index, info := range reservedSidecars {
		if info == nil {
			continue
		}
		path := corePath + cjsiSidecarSuffixes[index]
		if err := unlinkRestoredLayoutFile(path, info); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return cjsiSyncLayoutPath(targetRoot, true)
}

// A fresh restore has no previous install artifacts to recover. Refuse
// collisions and keep SQLite's disposable session files in a private scratch
// directory so they cannot consume a sidecar placed at a reserved target name.
func installFreshRestoredLayout(ctx context.Context, root, scratch string, db *sql.DB, baseTicket int64, sessions os.FileInfo, record, recordScratch func(string, os.FileInfo) error) error {
	legacy := filepath.Join(root, cjsiLegacyIndexName)
	snapshot := filepath.Join(root, cjsiSnapshotName)
	final := filepath.Join(root, cjsiPrivateIndexName)
	for _, name := range []string{cjsiSnapshotName, cjsiPreparedName, cjsiPrivateIndexName, cjsiFenceStage} {
		if err := cjsiRequireCleanIndexPath(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	if err := cjsiRequireAbsentSidecars(legacy); err != nil {
		return err
	}
	journal := cjsiSQLiteLayoutJournal{db: db}
	current, err := journal.load(ctx)
	if err != nil {
		return err
	}
	if current.phase != cjsiFenceInstalling {
		return errCJSILayoutInvalid
	}
	advance := func(next cjsiLayoutJournal) error {
		if err := cjsiCheckInstallRoot(root, current.root); err != nil {
			return err
		}
		if err := cjsiSyncLayoutPath(root, true); err != nil {
			return err
		}
		if err := journal.advance(ctx, current, next); err != nil {
			return err
		}
		current = next
		return nil
	}

	next := current
	next.phase = cjsiLegacyCaptured
	if sessions != nil {
		if err := checkRestoredLayoutFile(legacy, sessions); err != nil {
			return err
		}
		next.legacy, err = cjsiReadLayoutFile(ctx, legacy, true)
		if err != nil || next.legacy.identity != cjsiIdentity(sessions.Sys().(*syscall.Stat_t)) {
			return errors.Join(errCJSILayoutInvalid, err)
		}
		// The portable index has already been validated. Preserve its inode
		// as the captured snapshot without reopening it under the legacy name.
		if err := os.Link(legacy, snapshot); err != nil {
			return err
		}
		if err := record(snapshot, sessions); err != nil {
			return err
		}
		next.snapshot = next.legacy
	} else if err := cjsiRequireAbsent(legacy); err != nil {
		return err
	}
	if err := advance(next); err != nil {
		return err
	}

	if sessions != nil {
		if err := unlinkRestoredLayoutFile(legacy, sessions); err != nil {
			return err
		}
	}
	marker := cjsiFenceBytes()
	fence, err := createRestoredLayoutFile(legacy, marker[:], record)
	if err != nil {
		return err
	}
	next = current
	next.phase = cjsiFenceDurable
	next.fence, err = validateCJSIFence(root)
	if err != nil || next.fence != cjsiIdentity(fence.Sys().(*syscall.Stat_t)) {
		return errors.Join(errCJSIFenceInvalid, err)
	}
	if err := advance(next); err != nil {
		return err
	}

	if sessions != nil {
		if err := cjsiVerifyLayoutFile(ctx, snapshot, current.snapshot); err != nil {
			return err
		}
		// Copy the standalone archive into scratch without opening the
		// captured snapshot with SQLite in the target root.
		staging := filepath.Join(scratch, cjsiPreparedName)
		if err := copyRestoredLayoutFile(snapshot, staging, sessions, recordScratch); err != nil {
			return err
		}
		next = current
		next.phase = cjsiPrivatePrepared
		next.epoch, next.seal, err = prepareCJSIProductionIndexWithOptions(ctx, staging, true, true)
		if err != nil {
			return err
		}
		if err := cjsiSealPrivateFile(ctx, staging, next.epoch, next.seal, readCJSIProductionSeal); err != nil {
			return err
		}
		next.prepared, err = cjsiReadLayoutFile(ctx, staging, true)
		if err != nil {
			return err
		}
		if err := advance(next); err != nil {
			return err
		}
		if err := cjsiRequireAbsentSidecars(final); err != nil {
			return err
		}
		if err := publishRestoredLayoutFile(staging, final, record); err != nil {
			return err
		}
		if err := cjsiVerifyLayoutFile(ctx, final, current.prepared); err != nil {
			return err
		}
	}

	next = current
	next.phase = cjsiLayoutReady
	_, verifyErr := resolveCJSILayout(ctx, root, next.selection(), readCJSIProductionSeal)
	if err := errors.Join(verifyErr, closeCJSIInstallReadPool(final)); err != nil {
		return err
	}
	if fence, err := validateCJSIFence(root); err != nil || fence != current.fence {
		return errors.Join(errCJSIFenceInvalid, err)
	}
	if err := cjsiCheckInstallRoot(root, current.root); err != nil {
		return err
	}
	if err := cjsiSyncLayoutPath(root, true); err != nil {
		return err
	}
	if err := cjsiCommitRestoredHead(ctx, root, db, baseTicket, current, next); err != nil {
		return err
	}
	if sessions != nil {
		if err := unlinkRestoredLayoutFile(snapshot, sessions); err != nil {
			return err
		}
	}
	return cjsiSyncLayoutPath(root, true)
}

func copyRestoredLayoutFile(source, destination string, expected os.FileInfo, record func(string, os.FileInfo) error) (err error) {
	file, err := cjsiOpenPrivateLayoutPath(source, false)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(expected, info) {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	copyFile, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, copyFile.Close()) }()
	created, err := copyFile.Stat()
	if err != nil {
		return fmt.Errorf("ownership of restored file %q could not be proved; entry left in place: %w", filepath.Base(destination), err)
	}
	if err := record(destination, created); err != nil {
		return err
	}
	_, err = io.Copy(copyFile, file)
	return err
}

// Link publishes without replacing a colliding entry. The scratch copy
// provides the identity even if the destination is swapped after the link.
func publishRestoredLayoutFile(staged, destination string, record func(string, os.FileInfo) error) error {
	info, err := os.Lstat(staged)
	if err != nil {
		return err
	}
	if err := os.Link(staged, destination); err != nil {
		return err
	}
	if err := record(destination, info); err != nil {
		return err
	}
	return unlinkRestoredLayoutFile(staged, info)
}

func createRestoredLayoutFile(path string, data []byte, record func(string, os.FileInfo) error) (_ os.FileInfo, err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("ownership of restored file %q could not be proved; entry left in place: %w", filepath.Base(path), err)
	}
	if err := record(path, info); err != nil {
		return nil, err
	}
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	return info, file.Sync()
}

func checkRestoredLayoutFile(path string, expected os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(expected, current) {
		return errCJSILayoutInvalid
	}
	return nil
}

func unlinkRestoredLayoutFile(path string, expected os.FileInfo) error {
	if err := checkRestoredLayoutFile(path, expected); err != nil {
		return err
	}
	return syscall.Unlink(path)
}

func rebindRestoredCJSICore(ctx context.Context, db *sql.DB, root cjsiFileIdentity) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return 0, err
	}
	var newCoreEpoch int64
	if err := tx.QueryRowContext(ctx, `SELECT epoch FROM main.derived_snapshot_generation WHERE singleton=1`).Scan(&newCoreEpoch); err != nil || newCoreEpoch <= 0 {
		return 0, errors.Join(errCJSILayoutInvalid, err)
	}
	for _, stmt := range []string{
		`DELETE FROM main.cjsi_checkpoint_records`,
		`DELETE FROM main.cjsi_checkpoint_attempts`,
		`DELETE FROM main.cjsi_usage_interval_dirty_ranges`,
		`DELETE FROM main.usage_derived_invalidations`,
		`DELETE FROM main.usage_derived_partitions`,
		`INSERT INTO main.usage_derived_partitions(client,model,day) SELECT DISTINCT client,model,substr(event_at,1,10) FROM main.usage_events`,
		`DELETE FROM main.cjsi_usage_groups
			WHERE NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection s WHERE s.ticket=cjsi_usage_groups.ticket AND s.attempt=cjsi_usage_groups.attempt AND s.thread=cjsi_usage_groups.thread)
			  AND NOT EXISTS(SELECT 1 FROM main.cjsi_takeovers t WHERE t.opened_ticket=cjsi_usage_groups.ticket AND t.attempt=cjsi_usage_groups.attempt AND t.thread=cjsi_usage_groups.thread)`,
		`DELETE FROM main.cjsi_publication_attempts
			WHERE state<>'opened' OR (
				NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection s WHERE s.ticket=cjsi_publication_attempts.ticket AND s.attempt=cjsi_publication_attempts.attempt)
				AND NOT EXISTS(SELECT 1 FROM main.cjsi_takeovers t WHERE t.opened_ticket=cjsi_publication_attempts.ticket AND t.attempt=cjsi_publication_attempts.attempt))`,
		`UPDATE main.cjsi_scope_overlays SET coverage_revision=0, protection_revision=0`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_attempts
		SET core_epoch=?, index_epoch=0, session_seal=NULL, slot_seal=NULL, slot_digest=NULL,
		    slot_size=NULL, slot_format=NULL, slot_identity=NULL, built_at=NULL, valid_until=NULL`, newCoreEpoch); err != nil {
		return 0, err
	}
	var baseTicket int64
	if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(ticket),0) FROM main.cjsi_publication_attempts`).Scan(&baseTicket); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head
		SET core_epoch=?, ticket=?, session_ticket=0, session_attempt=0, index_epoch=0,
		    base_ticket=0, base_attempt=0, base_seal=NULL, base_revision=0, session_revision=0,
		    session_seal=NULL, slot_ticket=0, slot_attempt=0, slot_seal=NULL,
		    protection_revision=0, coverage_revision=0
		WHERE singleton=1`, newCoreEpoch, baseTicket)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return 0, errors.Join(errCJSILayoutInvalid, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM main.cjsi_layout_journal`); err != nil {
		return 0, err
	}
	if err := initializeCJSILayoutJournal(ctx, tx, root); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return baseTicket, nil
}

func cjsiCommitRestoredHead(ctx context.Context, root string, db *sql.DB, baseTicket int64, prior, next cjsiLayoutJournal) error {
	if next.phase != cjsiLayoutReady || !cjsiValidLayoutAdvance(prior, next) {
		return errCJSILayoutInvalid
	}
	var rev int64
	var digest any
	if next.prepared.present() {
		path := filepath.Join(root, cjsiPrivateIndexName)
		st, err := privateServingPath(path, false)
		if err != nil || cjsiIdentity(st) != next.prepared.identity {
			return errCJSILayoutInvalid
		}
		index, err := sql.Open("sqlite", cjsiSQLiteURI(path, "ro"))
		if err != nil {
			return err
		}
		rev, err = cjsiInitialIndexRevision(ctx, index, next.seal)
		if err = errors.Join(err, index.Close()); err != nil {
			return err
		}
		digest = next.seal.digest[:]
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET index_epoch=?,base_revision=?,session_revision=?,
		base_seal=?,session_seal=? WHERE singleton=1 AND ticket=? AND index_epoch=0 AND session_ticket=0 AND slot_ticket=0
		AND protection_revision=0 AND coverage_revision=0
		AND core_epoch=(SELECT epoch FROM main.derived_snapshot_generation WHERE singleton=1)`,
		next.epoch, rev, rev, digest, digest, baseTicket)
	if err != nil {
		return err
	}
	if n, err := r.RowsAffected(); err != nil || n != 1 {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	if err := cjsiAdvanceLayoutRecord(ctx, tx, prior, next); err != nil {
		return err
	}
	return tx.Commit()
}
