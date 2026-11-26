package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

const managedRestoreMarkerName = ".cjsi-restore-in-progress"

var markManagedRestoreInProgress = store.MarkStateRestoreInProgress
var beforeManagedRestoreMarkerClear func(targetRoot string)
var beforeManagedRestoreFileRecord func(path string)
var afterManagedRestoreLayoutProbe func(targetRoot string)
var afterManagedRestoreScanLock func(targetRoot string)
var acquireManagedRestoreScanLock = store.AcquireScanLock

const cjsiCoreSchemaVersion = 37

const portableSessionBaselineSchema = `
CREATE TABLE session_exclusions (kind TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(kind, value));
CREATE TABLE session_sources (source_path TEXT PRIMARY KEY, identity TEXT NOT NULL, cursor INTEGER NOT NULL, partial_line BLOB NOT NULL DEFAULT X'', size INTEGER NOT NULL, modified_at INTEGER NOT NULL, changed_at INTEGER NOT NULL DEFAULT 0, prefix_hash TEXT NOT NULL, priority INTEGER NOT NULL, parser_version INTEGER NOT NULL, scanned_at TEXT NOT NULL, parser_context TEXT NOT NULL DEFAULT '');
CREATE INDEX session_sources_identity ON session_sources(identity);
CREATE TABLE session_metadata (source_path TEXT NOT NULL REFERENCES session_sources(source_path) ON DELETE CASCADE, client TEXT NOT NULL, session_id TEXT NOT NULL, project TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', parser_version INTEGER NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL, PRIMARY KEY(source_path, client, session_id));
CREATE VIRTUAL TABLE portable_documents USING fts5(source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, event_at UNINDEXED, kind UNINDEXED, text);
`

func (s Service) createManaged(ctx context.Context, destination, passphrase string, includeSessions bool) (Manifest, error) {
	scanLock, err := store.AcquireScanLock(ctx, s.StateRoot, 5*time.Second)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = scanLock.Release() }()
	stateLock, err := store.AcquireLock(ctx, s.StateRoot, 5*time.Second)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = stateLock.Release() }()

	managed, err := store.ResolveCJSIRuntimeRoot(ctx, s.StateRoot)
	if err != nil || managed == nil {
		if errors.Is(err, store.ErrStateBusy) || ctx.Err() != nil {
			return Manifest{}, err
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("%w: %w", ErrInvalidArchive, err)
		}
		return Manifest{}, fmt.Errorf("%w: managed root unavailable", ErrInvalidArchive)
	}

	staging, err := os.MkdirTemp("", "agentdeck-backup-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(staging)
	if err = os.Chmod(staging, platform.DirectoryMode); err != nil {
		return Manifest{}, err
	}

	coreSnapshot := filepath.Join(staging, coreName)
	cleanSessions := filepath.Join(staging, "sessions.compact.sqlite3")
	var credentials []byte
	var capturedStamp store.CJSIIndexStamp
	var hasSessionEntry bool

	err = managed.WithPublishedCore(ctx, func(tx *sql.Tx, stamp store.CJSIIndexStamp) error {
		if err := verifyManagedPublishedState(ctx, managed, tx, stamp); err != nil {
			return err
		}
		if err := managed.BackupPublishedCore(ctx, tx, coreSnapshot); err != nil {
			return fmt.Errorf("snapshot core database: %w", err)
		}
		if s.AfterCoreSnapshot != nil {
			s.AfterCoreSnapshot()
		}
		creds, err := s.credentialsFromTx(ctx, tx)
		if err != nil {
			return err
		}
		credentials = creds
		capturedStamp = stamp

		if includeSessions {
			indexErr := managed.WithIndexSnapshot(ctx, tx, func(snap store.CJSIIndexSnapshot) error {
				return materializePortableSession(ctx, snap, cleanSessions)
			})
			if errors.Is(indexErr, store.ErrCJSIIndexAbsent) {
				hasSessionEntry = false
			} else if indexErr != nil {
				return fmt.Errorf("snapshot session database: %w", indexErr)
			} else {
				hasSessionEntry = true
			}
		}
		return nil
	})
	if err != nil {
		return Manifest{}, err
	}

	compactSnapshot := filepath.Join(staging, "core.compact.sqlite3")
	if err = preparePortableManagedCore(ctx, coreSnapshot, compactSnapshot, capturedStamp); err != nil {
		return Manifest{}, fmt.Errorf("prepare core snapshot: %w", err)
	}

	entries := make(map[string][]byte)
	if entries[coreName], err = os.ReadFile(compactSnapshot); err != nil {
		return Manifest{}, err
	}
	entries[credentialsName] = credentials
	if hasSessionEntry {
		if entries[sessionsName], err = os.ReadFile(cleanSessions); err != nil {
			return Manifest{}, err
		}
		if err = validateSessionArchive(ctx, entries[sessionsName], sessionSnapshotSchemaVersion, true); err != nil {
			return Manifest{}, fmt.Errorf("snapshot session database: %w", err)
		}
	}

	manifest := Manifest{
		SchemaVersion:    ManifestSchemaVersion,
		AgentDeckVersion: s.Version,
		CreatedAt:        s.now(),
		SourcePlatform:   runtime.GOOS + "/" + runtime.GOARCH,
		DatabaseSchemas:  map[string]int{coreName: cjsiCoreSchemaVersion},
		Included:         sortedKeys(entries),
	}
	if _, included := entries[sessionsName]; included {
		manifest.DatabaseSchemas[sessionsName] = sessionSnapshotSchemaVersion
	}
	for _, name := range manifest.Included {
		digest := sha256.Sum256(entries[name])
		manifest.Entries = append(manifest.Entries, Entry{Name: name, Size: int64(len(entries[name])), SHA256: hex.EncodeToString(digest[:])})
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, err
	}
	entries[manifestName] = manifestBytes
	if err = writeEncrypted(destination, passphrase, entries, manifest.CreatedAt); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// verifyManagedPublishedState confirms the active core head and any selected
// serving slot are consistent before exporting. Unopened drafts and retired
// attempts are ignored so an in-progress publication falls back cleanly to the
// previous safe opened generation.
func verifyManagedPublishedState(ctx context.Context, managed *store.CJSIRuntimeRoot, tx *sql.Tx, stamp store.CJSIIndexStamp) error {
	var valid bool
	if err := tx.QueryRowContext(ctx, `SELECT
		(?=0 OR 1=(SELECT count(*) FROM main.cjsi_publication_attempts
			WHERE ticket=? AND core_epoch=? AND state='opened' AND diff_digest IS NOT NULL))
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection s
			LEFT JOIN main.cjsi_usage_groups g USING(ticket,attempt,thread)
			LEFT JOIN main.cjsi_publication_attempts a USING(ticket,attempt)
			WHERE g.ticket IS NULL OR a.state IS NOT 'opened' OR a.core_epoch IS NOT ? OR a.diff_digest IS NULL OR s.ticket>?)
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_takeovers t
			LEFT JOIN main.cjsi_usage_groups g ON g.ticket=t.opened_ticket AND g.attempt=t.attempt AND g.thread=t.thread
			LEFT JOIN main.cjsi_publication_attempts a ON a.ticket=t.opened_ticket AND a.attempt=t.attempt
			WHERE g.ticket IS NULL OR a.state IS NOT 'opened' OR a.core_epoch IS NOT ? OR a.diff_digest IS NULL OR t.opened_ticket>?)`,
		stamp.Ticket, stamp.Ticket, stamp.CoreEpoch, stamp.CoreEpoch, stamp.Ticket, stamp.CoreEpoch, stamp.Ticket).Scan(&valid); err != nil || !valid {
		return errors.Join(store.ErrCJSIIndexChanged, err)
	}

	// The attempt keeps the reservation revision. The head selects the
	// result through the opened ticket, attempt and seals.
	var selected, readable bool
	var built, until sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT h.slot_ticket<>0,
		coalesce(a.protection_revision=h.protection_revision AND NOT EXISTS(SELECT 1 FROM main.cjsi_scope_overlays WHERE masked=1),0),
		a.built_at,a.valid_until FROM main.cjsi_publication_head h LEFT JOIN main.cjsi_publication_attempts a
		ON a.ticket=h.slot_ticket AND a.attempt=h.slot_attempt AND a.slot_seal=h.slot_seal
		AND a.ticket=h.session_ticket AND a.attempt=h.session_attempt AND a.session_seal=h.session_seal
		AND a.core_epoch=h.core_epoch AND a.index_epoch=h.index_epoch
		AND a.state='opened' AND a.slot_size>0 AND a.slot_format=1 AND length(a.slot_identity)=16 AND length(a.slot_digest)=32
		AND a.protection_revision<=h.protection_revision AND a.coverage_revision<=h.coverage_revision
		WHERE h.singleton=1`).Scan(&selected, &readable, &built, &until)
	if err != nil {
		return err
	}
	if !selected {
		return nil
	}
	start, startErr := time.Parse(time.RFC3339Nano, built.String)
	end, endErr := time.Parse(time.RFC3339Nano, until.String)
	if startErr != nil || endErr != nil || !start.Before(end) {
		return store.ErrCJSIIndexChanged
	}
	if !readable {
		return nil
	}
	_, err = managed.ReadServingSlot(ctx, tx, start)
	return err
}

func (s Service) credentialsFromTx(ctx context.Context, tx *sql.Tx) ([]byte, error) {
	rows, err := tx.QueryContext(ctx, `SELECT pc.credential_ref, cs.algorithm, cs.key_version, cs.key_id, cs.nonce, cs.ciphertext
		FROM main.provider_credentials pc
		JOIN main.credential_secrets cs ON cs.credential_id = pc.id
		ORDER BY pc.credential_ref`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	credentials := make([]Credential, 0)
	for rows.Next() {
		var ref, algo, keyID string
		var keyVersion int
		var nonce, ciphertext []byte
		if err := rows.Scan(&ref, &algo, &keyVersion, &keyID, &nonce, &ciphertext); err != nil {
			return nil, err
		}
		value, openErr := s.Vault.Open(ctx, ref, credentialvault.Sealed{
			Algorithm:  algo,
			KeyVersion: keyVersion,
			KeyID:      keyID,
			Nonce:      nonce,
			Ciphertext: ciphertext,
		})
		if openErr != nil {
			return nil, fmt.Errorf("read credential reference: %w", openErr)
		}
		credentials = append(credentials, Credential{Reference: ref, Value: value})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].Reference < credentials[j].Reference })
	return json.Marshal(credentials)
}

func materializePortableSession(ctx context.Context, snap store.CJSIIndexSnapshot, destPath string) (err error) {
	uri := url.URL{Scheme: "file", Path: destPath, RawQuery: "mode=rwc&_pragma=foreign_keys(1)&_pragma=secure_delete(1)"}
	dst, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dst.Close(); err == nil {
			err = closeErr
		}
	}()
	dst.SetMaxOpenConns(1)

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, portableSessionBaselineSchema); err != nil {
		return err
	}

	// 1. Copy exclusions verbatim.
	exclRows, err := snap.Tx.QueryContext(ctx, `SELECT kind, value FROM main.session_exclusions`)
	if err != nil {
		return err
	}
	for exclRows.Next() {
		var kind, value string
		if err = exclRows.Scan(&kind, &value); err != nil {
			exclRows.Close()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO session_exclusions(kind, value) VALUES(?, ?)`, kind, value); err != nil {
			exclRows.Close()
			return err
		}
	}
	if err = errors.Join(exclRows.Err(), exclRows.Close()); err != nil {
		return err
	}

	// 2. Copy only selected sources with tails and parser_context scrubbed.
	stamp := snap.Selection()
	var compacted bool
	if err = snap.Tx.QueryRowContext(ctx, `SELECT compacted_ticket=? AND compacted_attempt=? FROM main.cjsi_index_state WHERE singleton=1`,
		stamp.SessionTicket, stamp.SessionAttempt).Scan(&compacted); err != nil {
		return err
	}
	useCandidate := !compacted && stamp.SessionTicket != 0
	srcRows, err := snap.Tx.QueryContext(ctx, `WITH copies AS (`+snap.CopiesSQL()+`),
		needed AS (SELECT DISTINCT source_path FROM copies),
		cand AS (
			SELECT source_path, identity, complete_end AS cursor, size, modified_at, changed_at, prefix_hash, priority, parser_version, scanned_at
			FROM main.cjsi_candidate_sources
			WHERE ? AND ticket=? AND attempt=? AND op='upsert'
		)
		SELECT n.source_path,
		       coalesce(c.identity, b.identity),
		       CASE WHEN c.source_path IS NOT NULL THEN c.cursor
		            WHEN length(CAST(b.partial_line AS BLOB))>0 THEN max(0, b.cursor - length(CAST(b.partial_line AS BLOB)))
		            ELSE b.cursor END,
		       coalesce(c.size, b.size),
		       coalesce(c.modified_at, b.modified_at),
		       coalesce(c.changed_at, b.changed_at),
		       coalesce(c.prefix_hash, b.prefix_hash),
		       coalesce(c.priority, b.priority),
		       CASE WHEN c.source_path IS NOT NULL THEN c.parser_version
		            WHEN length(CAST(b.partial_line AS BLOB))>0 OR b.parser_context<>'' THEN 0
		            ELSE b.parser_version END,
		       coalesce(c.scanned_at, b.scanned_at)
		FROM needed n
		LEFT JOIN cand c ON c.source_path = n.source_path
		LEFT JOIN main.session_sources b ON b.source_path = n.source_path
		WHERE c.source_path IS NOT NULL OR b.source_path IS NOT NULL`,
		useCandidate, stamp.SessionTicket, stamp.SessionAttempt)
	if err != nil {
		return err
	}
	for srcRows.Next() {
		var sourcePath, identity, prefixHash, scannedAt string
		var cursor, size, modAt, chgAt, priority, parserVersion int64
		if err = srcRows.Scan(&sourcePath, &identity, &cursor, &size, &modAt, &chgAt, &prefixHash, &priority, &parserVersion, &scannedAt); err != nil {
			srcRows.Close()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO session_sources
			(source_path, identity, cursor, partial_line, size, modified_at, changed_at, prefix_hash, priority, parser_version, scanned_at, parser_context)
			VALUES(?, ?, ?, X'', ?, ?, ?, ?, ?, ?, ?, '')`,
			sourcePath, identity, cursor, size, modAt, chgAt, prefixHash, priority, parserVersion, scannedAt); err != nil {
			srcRows.Close()
			return err
		}
	}
	if err = errors.Join(srcRows.Err(), srcRows.Close()); err != nil {
		return err
	}

	// 3. Copy selected metadata groups.
	metaRows, err := snap.Tx.QueryContext(ctx, `SELECT source_path, client, session_id, project, model, parser_version, first_at, last_at FROM (`+snap.CopiesSQL()+`)`)
	if err != nil {
		return err
	}
	for metaRows.Next() {
		var sourcePath, client, sessionID, project, model, firstAt, lastAt string
		var parserVersion int64
		if err = metaRows.Scan(&sourcePath, &client, &sessionID, &project, &model, &parserVersion, &firstAt, &lastAt); err != nil {
			metaRows.Close()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO session_metadata
			(source_path, client, session_id, project, model, parser_version, first_at, last_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			sourcePath, client, sessionID, project, model, parserVersion, firstAt, lastAt); err != nil {
			metaRows.Close()
			return err
		}
	}
	if err = errors.Join(metaRows.Err(), metaRows.Close()); err != nil {
		return err
	}

	// 4. Copy selected FTS documents with their exact doc_id as rowid.
	docRows, err := snap.Tx.QueryContext(ctx, `WITH m AS (`+snap.MembersSQL()+`)
		SELECT m.doc_id, m.source_path, m.client, m.session_id,
		       CASE WHEN m.doc_side='base' THEN bd.event_at ELSE cp.event_at END,
		       CASE WHEN m.doc_side='base' THEN bd.kind ELSE cp.kind END,
		       CASE WHEN m.doc_side='base' THEN bd.text ELSE cp.text END
		FROM m
		LEFT JOIN main.session_documents bd ON m.doc_side='base' AND bd.rowid=m.doc_id
		LEFT JOIN main.cjsi_candidate_payloads cp ON m.doc_side='candidate' AND cp.doc_id=m.doc_id
		ORDER BY m.doc_id`)
	if err != nil {
		return err
	}
	for docRows.Next() {
		var docID int64
		var sourcePath, client, sessionID string
		var eventAt, kind, text sql.NullString
		if err = docRows.Scan(&docID, &sourcePath, &client, &sessionID, &eventAt, &kind, &text); err != nil {
			docRows.Close()
			return err
		}
		if !eventAt.Valid || !kind.Valid || !text.Valid {
			docRows.Close()
			return store.ErrCJSIIndexChanged
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO portable_documents(rowid, source_path, client, session_id, event_at, kind, text)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			docID, sourcePath, client, sessionID, eventAt.String, kind.String, text.String); err != nil {
			docRows.Close()
			return err
		}
	}
	if err = errors.Join(docRows.Err(), docRows.Close()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `ALTER TABLE portable_documents RENAME TO session_documents`); err != nil {
		return err
	}

	// 5. Write and validate portable_session_* order and allocator high water.
	if err = writeSelectedSessionOrder(ctx, tx, snap); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	var integrity string
	if err = dst.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("%w: session snapshot integrity", ErrInvalidArchive)
	}
	return nil
}

func preparePortableManagedCore(ctx context.Context, snapshotPath, compactPath string, stamp store.CJSIIndexStamp) (err error) {
	uri := url.URL{Scheme: "file", Path: snapshotPath, RawQuery: "mode=rw"}
	database, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); err == nil {
			err = closeErr
		}
	}()
	conn, err := database.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); err == nil {
			err = closeErr
		}
	}()
	if _, err = conn.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	var version int
	if err = conn.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton=1`).Scan(&version); err != nil || version != cjsiCoreSchemaVersion {
		return fmt.Errorf("%w: unsupported portable core layout", ErrInvalidArchive)
	}
	var headTicket, headCoreEpoch int64
	if err = conn.QueryRowContext(ctx, `SELECT ticket, core_epoch FROM cjsi_publication_head WHERE singleton=1`).Scan(&headTicket, &headCoreEpoch); err != nil || headTicket != stamp.Ticket || headCoreEpoch != stamp.CoreEpoch {
		return store.ErrCJSIIndexChanged
	}

	for _, table := range originNonPortableTables {
		var present int
		if err = conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&present); err != nil {
			return err
		}
		if present == 0 {
			continue
		}
		if _, err = conn.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}

	for _, statement := range []string{
		"DELETE FROM quota_identities",
		"UPDATE quota_cycle SET seq=0,revalidate_seq=0",
		`UPDATE quota_envelopes_v2 SET cycle_seq=0,context_generation=0,invalidated_at='',invalidated_reason='',
            capture_gate='open_initial',confirmed_generation=-1,capture_open_boot='',capture_open_mono=0,
            failure_detail='',plan_unsafe=0`,
		"UPDATE quota_windows_v2 SET cycle_seq=0,context_generation=0",
		"DELETE FROM quota_alert_notices_v2 WHERE context_generation<>0",
		// Scrub CJSI transient worksets, dirty stamps, derived caches, and unopened attempts.
		"DELETE FROM cjsi_checkpoint_records",
		"DELETE FROM cjsi_checkpoint_attempts",
		"DELETE FROM cjsi_usage_interval_dirty_ranges",
		"DELETE FROM usage_derived_invalidations",
		"DELETE FROM usage_derived_partitions",
		"INSERT INTO usage_derived_partitions(client,model,day) SELECT DISTINCT client,model,substr(event_at,1,10) FROM usage_events",
		`DELETE FROM cjsi_usage_groups
			WHERE NOT EXISTS(SELECT 1 FROM cjsi_usage_selection s WHERE s.ticket=cjsi_usage_groups.ticket AND s.attempt=cjsi_usage_groups.attempt AND s.thread=cjsi_usage_groups.thread)
			  AND NOT EXISTS(SELECT 1 FROM cjsi_takeovers t WHERE t.opened_ticket=cjsi_usage_groups.ticket AND t.attempt=cjsi_usage_groups.attempt AND t.thread=cjsi_usage_groups.thread)`,
		`DELETE FROM cjsi_publication_attempts
			WHERE state<>'opened' OR (
				NOT EXISTS(SELECT 1 FROM cjsi_usage_selection s WHERE s.ticket=cjsi_publication_attempts.ticket AND s.attempt=cjsi_publication_attempts.attempt)
				AND NOT EXISTS(SELECT 1 FROM cjsi_takeovers t WHERE t.opened_ticket=cjsi_publication_attempts.ticket AND t.attempt=cjsi_publication_attempts.attempt))`,
		`UPDATE cjsi_publication_attempts
			SET index_epoch=0, session_seal=NULL, slot_seal=NULL, slot_digest=NULL,
			    slot_size=NULL, slot_format=NULL, slot_identity=NULL, built_at=NULL, valid_until=NULL`,
		`UPDATE cjsi_publication_head
			SET ticket=coalesce((SELECT max(ticket) FROM cjsi_publication_attempts),0),
			    session_ticket=0, session_attempt=0, index_epoch=0,
			    base_ticket=0, base_attempt=0, base_seal=NULL, base_revision=0,
			    session_revision=0, session_seal=NULL,
			    slot_ticket=0, slot_attempt=0, slot_seal=NULL,
			    protection_revision=0, coverage_revision=0
			WHERE singleton=1`,
		`UPDATE cjsi_scope_overlays SET coverage_revision=0, protection_revision=0`,
		`UPDATE cjsi_layout_journal
			SET phase=1, root_identity=X'00000000000000000000000000000001',
			    legacy_file=NULL, snapshot_file=NULL, fence_identity=NULL,
			    prepared_file=NULL, index_epoch=NULL, seal_version=NULL, seal_digest=NULL
			WHERE singleton=1`,
	} {
		if _, err = conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true

	var busy, logFrames, checkpointed int
	if err = conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return errors.New("core snapshot checkpoint did not complete")
	}
	if _, err = conn.ExecContext(ctx, "VACUUM INTO ?", compactPath); err != nil {
		return err
	}
	if err = validatePortableQuotaCore(ctx, compactPath); err != nil {
		return err
	}
	if err = store.ValidateCJSICoreArchive(ctx, compactPath); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	return nil
}

func validateManagedCoreSnapshot(ctx context.Context, data []byte, expected int) (map[string]int64, portableCoreSchema, error) {
	if expected != cjsiCoreSchemaVersion {
		return nil, portableCoreSchema{}, fmt.Errorf("%w: database schema", ErrInvalidArchive)
	}
	root, err := os.MkdirTemp("", "agentdeck-restore-validate-")
	if err != nil {
		return nil, portableCoreSchema{}, err
	}
	defer os.RemoveAll(root)
	if err = os.Chmod(root, platform.DirectoryMode); err != nil {
		return nil, portableCoreSchema{}, err
	}
	corePath := filepath.Join(root, coreName)
	if err = os.WriteFile(corePath, data, platform.FileMode); err != nil {
		return nil, portableCoreSchema{}, err
	}
	if err = store.ValidateCJSICoreArchive(ctx, corePath); err != nil {
		return nil, portableCoreSchema{}, fmt.Errorf("%w: core database", ErrInvalidArchive)
	}
	uri := url.URL{Scheme: "file", Path: corePath, RawQuery: "mode=ro&_pragma=query_only(1)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, portableCoreSchema{}, fmt.Errorf("%w: core database", ErrInvalidArchive)
	}
	defer db.Close()
	if err = validateOriginStateDB(ctx, db); err != nil {
		return nil, portableCoreSchema{}, err
	}
	rows, err := db.QueryContext(ctx, `SELECT pc.credential_ref, pc.id
		FROM main.provider_credentials pc
		JOIN main.credential_secrets cs ON cs.credential_id = pc.id`)
	if err != nil {
		return nil, portableCoreSchema{}, fmt.Errorf("%w: provider records", ErrInvalidArchive)
	}
	defer rows.Close()
	referenceSet := make(map[string]int64)
	for rows.Next() {
		var ref string
		var id int64
		if err = rows.Scan(&ref, &id); err != nil {
			return nil, portableCoreSchema{}, fmt.Errorf("%w: provider records", ErrInvalidArchive)
		}
		referenceSet[ref] = id
	}
	if err = rows.Err(); err != nil {
		return nil, portableCoreSchema{}, fmt.Errorf("%w: provider records", ErrInvalidArchive)
	}
	return referenceSet, portableCoreSchema{compact: true, managed: true}, nil
}

type ownedRestoreEntry struct {
	path      string
	info      os.FileInfo
	pin       *os.File
	hasDigest bool
	digest    [32]byte
}

// Pin the inode until rollback finishes so a removed file's identity cannot
// be recycled for a different entry while it is still in the ownership list.
func pinOwnedRestoreFile(path string, expected os.FileInfo, withDigest bool) (ownedRestoreEntry, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return ownedRestoreEntry{}, err
	}
	pin := os.NewFile(uintptr(fd), path)
	info, err := pin.Stat()
	if err != nil {
		pin.Close()
		return ownedRestoreEntry{}, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(expected, info) {
		pin.Close()
		return ownedRestoreEntry{}, errors.New("restored file identity changed")
	}
	entry := ownedRestoreEntry{path: path, info: info, pin: pin, hasDigest: withDigest}
	if withDigest {
		entry.digest, err = digestRestoreKey(pin)
		if err != nil {
			pin.Close()
			return ownedRestoreEntry{}, err
		}
	}
	return entry, nil
}

func digestRestoreKey(file *os.File) ([32]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return [32]byte{}, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [32]byte{}, err
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func removeOwnedRestoreFile(entry ownedRestoreEntry) error {
	matches := func() (bool, error) {
		info, err := os.Lstat(entry.path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return info.Mode().IsRegular() && os.SameFile(entry.info, info), nil
	}
	if same, err := matches(); !same || err != nil {
		return err
	}
	if entry.hasDigest {
		digest, err := digestRestoreKey(entry.pin)
		if err != nil {
			return err
		}
		if digest != entry.digest {
			return nil
		}
		if same, err := matches(); !same || err != nil {
			return err
		}
	}
	if err := syscall.Unlink(entry.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func restoreOwnershipError(path string, err error) error {
	return fmt.Errorf("ownership of restored file %q could not be proved; entry left in place: %w", filepath.Base(path), err)
}

func writeOwnedRestoreFile(path string, data []byte, record func(string, os.FileInfo) error) (info os.FileInfo, err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if info, err = file.Stat(); err != nil {
		return nil, restoreOwnershipError(path, err)
	}
	if err = record(path, info); err != nil {
		return nil, err
	}
	if _, err = file.Write(data); err != nil {
		return nil, err
	}
	return info, file.Sync()
}

func restoreManaged(ctx context.Context, manifest Manifest, entries map[string][]byte, credentials []Credential, targetRoot string, machineIdentity credentialvault.MachineIdentity) (_ Manifest, err error) {
	sessionData, hasSessions := entries[sessionsName]
	if hasSessions {
		if err = validateSessionArchive(ctx, sessionData, manifest.DatabaseSchemas[sessionsName], true); err != nil {
			return Manifest{}, err
		}
	}

	targetRoot, err = platform.GuardStateRoot(targetRoot)
	if err != nil {
		return Manifest{}, err
	}
	if err = store.CheckStateRestoreInProgress(targetRoot); err != nil {
		return Manifest{}, err
	}
	createdRoot, originalMode, err := reserveEmptyTarget(targetRoot)
	if err != nil {
		return Manifest{}, err
	}

	var scanLock, lock *store.Lock
	rootInfo, err := os.Lstat(targetRoot)
	if err != nil {
		return Manifest{}, err
	}
	ownedFiles := make([]ownedRestoreEntry, 0, 8)
	recordOwned := func(path string, info os.FileInfo, digest *[sha256.Size]byte) error {
		if beforeManagedRestoreFileRecord != nil {
			beforeManagedRestoreFileRecord(path)
		}
		owned, err := pinOwnedRestoreFile(path, info, digest != nil)
		if err != nil {
			return restoreOwnershipError(path, err)
		}
		if digest != nil && owned.digest != *digest {
			owned.pin.Close()
			return restoreOwnershipError(path, errors.New("restored key contents changed"))
		}
		ownedFiles = append(ownedFiles, owned)
		return nil
	}
	recordOwnedRestoreFile := func(path string, info os.FileInfo) error {
		return recordOwned(path, info, nil)
	}
	sameRoot := func() bool {
		current, statErr := os.Lstat(targetRoot)
		return statErr == nil && current.IsDir() && os.SameFile(rootInfo, current)
	}
	var marker ownedRestoreEntry
	clearMarker := func() error {
		if beforeManagedRestoreMarkerClear != nil {
			beforeManagedRestoreMarkerClear(targetRoot)
		}
		if !sameRoot() ||
			!store.LockOwnedByCurrentProcess(filepath.Join(targetRoot, "scan.lock")) ||
			!store.LockOwnedByCurrentProcess(filepath.Join(targetRoot, "state.lock")) {
			return store.ErrStateRestoreInProgress
		}
		current, err := os.Lstat(marker.path)
		if err != nil {
			return restoreOwnershipError(marker.path, err)
		}
		if !current.Mode().IsRegular() || !os.SameFile(marker.info, current) {
			return restoreOwnershipError(marker.path, errors.New("restore marker identity changed"))
		}
		return store.ClearStateRestoreInProgress(targetRoot)
	}

	published := false
	defer func() {
		failed := err != nil && !published
		var rollbackErrs []error
		if failed && sameRoot() {
			for index := len(ownedFiles) - 1; index >= 0; index-- {
				if removeErr := removeOwnedRestoreFile(ownedFiles[index]); removeErr != nil {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback file %q: %w", ownedFiles[index].path, removeErr))
				}
			}
		}
		if marker.pin != nil {
			if failed && sameRoot() && len(rollbackErrs) == 0 {
				if clearErr := validateReservedTargetManaged(targetRoot, true); clearErr == nil {
					if clearErr = clearMarker(); clearErr != nil {
						rollbackErrs = append(rollbackErrs, fmt.Errorf("clear restore marker: %w", clearErr))
					}
				} else if !errors.Is(clearErr, ErrTargetNotEmpty) {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("verify restore rollback: %w", clearErr))
				}
			}
			if closeErr := marker.pin.Close(); closeErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("close restore marker: %w", closeErr))
			}
		}
		for _, owned := range ownedFiles {
			if closeErr := owned.pin.Close(); closeErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("close restore file %q: %w", owned.path, closeErr))
			}
		}
		if lock != nil {
			if releaseErr := lock.Release(); releaseErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("release restore state lock: %w", releaseErr))
			}
		}
		if scanLock != nil {
			if releaseErr := scanLock.Release(); releaseErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("release restore scan lock: %w", releaseErr))
			}
		}
		if failed && !createdRoot && originalMode.Perm() != platform.DirectoryMode && sameRoot() {
			if chmodErr := os.Chmod(targetRoot, originalMode.Perm()); chmodErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback state root permissions: %w", chmodErr))
			}
		}
		if failed && createdRoot && sameRoot() {
			if removeErr := syscall.Rmdir(targetRoot); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) &&
				!errors.Is(removeErr, syscall.ENOTEMPTY) && !errors.Is(removeErr, syscall.EEXIST) {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback state root: %w", removeErr))
			}
		}
		if len(rollbackErrs) > 0 {
			err = errors.Join(append([]error{err}, rollbackErrs...)...)
		}
	}()

	if !createdRoot && originalMode.Perm() != platform.DirectoryMode {
		if err = os.Chmod(targetRoot, platform.DirectoryMode); err != nil {
			return Manifest{}, err
		}
	}
	scanLock, err = acquireManagedRestoreScanLock(ctx, targetRoot, 5*time.Second)
	if err != nil {
		return Manifest{}, err
	}
	if afterManagedRestoreScanLock != nil {
		afterManagedRestoreScanLock(targetRoot)
	}
	lock, err = store.AcquireLock(ctx, targetRoot, 5*time.Second)
	if err != nil {
		return Manifest{}, err
	}
	if err = validateReservedTargetManaged(targetRoot, false); err != nil {
		return Manifest{}, err
	}
	markerPath := filepath.Join(targetRoot, managedRestoreMarkerName)
	if markErr := markManagedRestoreInProgress(ctx, targetRoot); markErr != nil {
		// Mark can fail after writing its full binding. Enter ordinary
		// rollback only with a pinned marker owned by both live locks.
		if sameRoot() {
			pin, proofErr := store.OpenOwnedStateRestoreMarker(targetRoot, scanLock, lock)
			if proofErr == nil {
				if info, statErr := pin.Stat(); statErr == nil {
					marker = ownedRestoreEntry{path: markerPath, info: info, pin: pin}
				} else {
					_ = pin.Close()
				}
			}
		}
		return Manifest{}, markErr
	}
	markerInfo, statErr := os.Lstat(markerPath)
	if statErr != nil {
		return Manifest{}, restoreOwnershipError(markerPath, statErr)
	}
	marker, err = pinOwnedRestoreFile(markerPath, markerInfo, false)
	if err != nil {
		return Manifest{}, restoreOwnershipError(markerPath, err)
	}

	corePath := filepath.Join(targetRoot, coreName)
	core, err := writeOwnedRestoreFile(corePath, entries[coreName], recordOwnedRestoreFile)
	if err != nil {
		return Manifest{}, err
	}
	var sessions os.FileInfo
	if hasSessions {
		sessions, err = writeOwnedRestoreFile(filepath.Join(targetRoot, sessionsName), sessionData, recordOwnedRestoreFile)
		if err != nil {
			return Manifest{}, err
		}
	}
	if err = validateManagedRestoreEntries(targetRoot, hasSessions); err != nil {
		return Manifest{}, err
	}

	err = store.InstallRestoredCJSILayout(ctx, targetRoot, scanLock, lock, core, sessions, recordOwnedRestoreFile, func(database *store.Store) error {
		vault := credentialvault.New(targetRoot, machineIdentity)
		if len(credentials) > 0 {
			var recorded bool
			keyPath := vault.KeyPath()
			initVault := vault.WithNewKeyObserver(func(info os.FileInfo, digest [sha256.Size]byte) error {
				err := recordOwned(keyPath, info, &digest)
				recorded = err == nil
				return err
			})
			keyCreated, keyErr := initializeRestoreCredentialKey(ctx, initVault)
			if keyCreated && !recorded {
				return errors.Join(keyErr, restoreOwnershipError(keyPath, errors.New("creation identity was not recorded")))
			}
			if keyErr != nil {
				if errors.Is(keyErr, fs.ErrExist) {
					return fmt.Errorf("%w: credential key already exists", ErrTargetNotEmpty)
				}
				return keyErr
			}
		}
		targetOwners := make(map[string]int64)
		targetProviders, listErr := database.ListProviders(ctx)
		if listErr != nil {
			return listErr
		}
		for _, provider := range targetProviders {
			for _, credential := range provider.Credentials {
				targetOwners[credential.CredentialRef] = credential.ID
			}
		}
		sealed := make([]store.CredentialSecret, 0, len(credentials))
		for _, credential := range credentials {
			credentialID, ok := targetOwners[credential.Reference]
			if !ok {
				return fmt.Errorf("%w: credential ownership", ErrInvalidArchive)
			}
			payload, sealErr := vault.SealExisting(ctx, credential.Reference, credential.Value)
			if sealErr != nil {
				return sealErr
			}
			sealed = append(sealed, store.CredentialSecret{
				CredentialID: credentialID,
				Algorithm:    payload.Algorithm,
				KeyVersion:   payload.KeyVersion,
				KeyID:        payload.KeyID,
				Nonce:        payload.Nonce,
				Ciphertext:   payload.Ciphertext,
			})
		}
		if err := database.ReplaceCredentialSecrets(ctx, sealed); err != nil {
			return err
		}
		if err := resetRestoredOriginState(ctx, database); err != nil {
			return err
		}
		if err := database.MintDerivedSnapshotEpoch(ctx); err != nil {
			return err
		}
		if err := clearRestoredQuotaState(ctx, database.DB, true); err != nil {
			return err
		}
		quotaSettings, loadErr := quota.LoadSettings(ctx, database)
		if loadErr != nil {
			return loadErr
		}
		if quotaSettings.StatusLineConsent {
			quotaSettings.StatusLineConsent = false
			if err := quota.SaveSettings(ctx, database, quotaSettings); err != nil {
				return err
			}
		}
		return nil
	}, afterManagedRestoreLayoutProbe)
	if err != nil {
		return Manifest{}, err
	}
	// A Clear failure can occur after its unlink has released admission.
	// Keep the verified publication intact even if that sync fails.
	published = true
	if err = clearMarker(); err != nil {
		return Manifest{}, fmt.Errorf("clear restore marker after publication: %w", err)
	}
	return manifest, nil
}

func validateReservedTargetManaged(path string, allowMarker bool) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "state.lock" && name != "scan.lock" && !(allowMarker && name == managedRestoreMarkerName) {
			return ErrTargetNotEmpty
		}
	}
	return nil
}

func validateManagedRestoreEntries(path string, hasSessions bool) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "state.lock" || name == "scan.lock" || name == managedRestoreMarkerName || name == coreName || (hasSessions && name == sessionsName) {
			continue
		}
		return ErrTargetNotEmpty
	}
	return nil
}
