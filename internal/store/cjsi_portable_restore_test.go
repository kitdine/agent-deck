package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCJSIPortableRestoreStoreHelpers(t *testing.T) {
	t.Run("initialize_without_portable_order_preserves_legacy_behavior", func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		staging := filepath.Join(dir, cjsiPreparedName)

		db, err := openWritableSQL(cjsiSQLiteURI(staging, "rwc"), "session")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, cjsiSessionBaselineSchema+`
			INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			VALUES('src.jsonl','id1',10,X'',10,1,1,'h1',1,6,'2026-01-01T00:00:00Z','');
			INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			VALUES('src.jsonl','codex','s1','proj','m1',6,'2026-01-01T00:00:00Z','2026-01-01T00:00:01Z');
			INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			VALUES(1,'src.jsonl','codex','s1','2026-01-01T00:00:00Z','user_prompt','A'),
			      (2,'src.jsonl','codex','s1','2026-01-01T00:00:01Z','assistant_message','B'),
			      (5,'src.jsonl','codex','s1','2026-01-01T00:00:02Z','user_prompt','C');`); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := ensureSessionIndexGeneration(ctx, db); err != nil {
			db.Close()
			t.Fatal(err)
		}
		var origEpoch int64
		if err := db.QueryRowContext(ctx, `SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&origEpoch); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		// Legacy migration path uses forceMintEpoch=false and no portable_session_* tables.
		epoch, _, err := prepareCJSIProductionIndex(ctx, staging)
		if err != nil {
			t.Fatal(err)
		}
		if epoch != origEpoch {
			t.Fatalf("legacy prepareCJSIProductionIndex re-minted epoch: got %d want %d", epoch, origEpoch)
		}

		checkDB, err := sql.Open("sqlite", cjsiSQLiteURI(staging, "ro"))
		if err != nil {
			t.Fatal(err)
		}
		defer checkDB.Close()

		var highWater int64
		if err := checkDB.QueryRowContext(ctx, `SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1`).Scan(&highWater); err != nil {
			t.Fatal(err)
		}
		if highWater != 5 {
			t.Fatalf("doc_id_high_water = %d, want 5", highWater)
		}
		var verified int
		if err := checkDB.QueryRowContext(ctx, `SELECT order_verified FROM cjsi_base_groups WHERE source_path='src.jsonl' AND client='codex' AND session_id='s1'`).Scan(&verified); err != nil {
			t.Fatal(err)
		}
		if verified != 0 {
			t.Fatalf("order_verified = %d, want 0", verified)
		}
		var seq string
		if err := checkDB.QueryRowContext(ctx, `SELECT group_concat(doc_id, ',') FROM (SELECT doc_id FROM cjsi_base_order ORDER BY detail_ordinal)`).Scan(&seq); err != nil {
			t.Fatal(err)
		}
		if seq != "1,2,5" {
			t.Fatalf("legacy rowid order = %q, want 1,2,5", seq)
		}
	})

	t.Run("initialize_with_portable_order_imports_order_high_water_and_mints_epoch", func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		staging := filepath.Join(dir, cjsiPreparedName)

		db, err := openWritableSQL(cjsiSQLiteURI(staging, "rwc"), "session")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, cjsiSessionBaselineSchema+`
			CREATE TABLE portable_session_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), doc_id_high_water INTEGER NOT NULL CHECK(doc_id_high_water>=0)) STRICT;
			CREATE TABLE portable_session_groups (source_path TEXT NOT NULL, client TEXT NOT NULL, session_id TEXT NOT NULL, order_verified INTEGER NOT NULL CHECK(order_verified IN (0,1)), PRIMARY KEY(source_path,client,session_id)) STRICT, WITHOUT ROWID;
			CREATE TABLE portable_session_order (doc_id INTEGER PRIMARY KEY CHECK(doc_id>0), source_path TEXT NOT NULL, client TEXT NOT NULL, session_id TEXT NOT NULL, detail_ordinal INTEGER NOT NULL CHECK(detail_ordinal>0), UNIQUE(source_path,client,session_id,detail_ordinal), FOREIGN KEY(source_path,client,session_id) REFERENCES portable_session_groups) STRICT;
			INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			VALUES('src.jsonl','id1',10,X'',10,1,1,'h1',1,6,'2026-01-01T00:00:00Z','');
			INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			VALUES('src.jsonl','codex','s1','proj','m1',6,'2026-01-01T00:00:00Z','2026-01-01T00:00:01Z');
			INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			VALUES(1,'src.jsonl','codex','s1','2026-01-01T00:00:00Z','user_prompt','A'),
			      (2,'src.jsonl','codex','s1','2026-01-01T00:00:01Z','assistant_message','B'),
			      (3,'src.jsonl','codex','s1','2026-01-01T00:00:02Z','user_prompt','C'),
			      (4,'src.jsonl','codex','s1','2026-01-01T00:00:03Z','assistant_message','X');
			INSERT INTO portable_session_state VALUES(1, 12);
			INSERT INTO portable_session_groups VALUES('src.jsonl','codex','s1',1);
			INSERT INTO portable_session_order VALUES
				(1,'src.jsonl','codex','s1',1),
				(4,'src.jsonl','codex','s1',2),
				(2,'src.jsonl','codex','s1',3),
				(3,'src.jsonl','codex','s1',4);`); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := ensureSessionIndexGeneration(ctx, db); err != nil {
			db.Close()
			t.Fatal(err)
		}
		var origEpoch int64
		if err := db.QueryRowContext(ctx, `SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&origEpoch); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		// Restore path forces epoch mint and imports portable_session_*.
		newEpoch, _, err := prepareCJSIProductionIndexWithOptions(ctx, staging, true, true)
		if err != nil {
			t.Fatal(err)
		}
		if newEpoch == origEpoch || newEpoch <= 0 {
			t.Fatalf("restore prepareCJSIProductionIndexWithOptions did not mint new epoch: old=%d new=%d", origEpoch, newEpoch)
		}

		checkDB, err := sql.Open("sqlite", cjsiSQLiteURI(staging, "ro"))
		if err != nil {
			t.Fatal(err)
		}
		defer checkDB.Close()

		var highWater int64
		if err := checkDB.QueryRowContext(ctx, `SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1`).Scan(&highWater); err != nil {
			t.Fatal(err)
		}
		if highWater != 12 {
			t.Fatalf("doc_id_high_water = %d, want 12", highWater)
		}
		var verified int
		if err := checkDB.QueryRowContext(ctx, `SELECT order_verified FROM cjsi_base_groups WHERE source_path='src.jsonl' AND client='codex' AND session_id='s1'`).Scan(&verified); err != nil {
			t.Fatal(err)
		}
		if verified != 1 {
			t.Fatalf("order_verified = %d, want 1", verified)
		}
		var seq string
		if err := checkDB.QueryRowContext(ctx, `SELECT group_concat(doc_id, ',') FROM (SELECT doc_id FROM cjsi_base_order ORDER BY detail_ordinal)`).Scan(&seq); err != nil {
			t.Fatal(err)
		}
		if seq != "1,4,2,3" {
			t.Fatalf("portable order = %q, want 1,4,2,3", seq)
		}
		var leftover int
		if err := checkDB.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE lower(name) GLOB 'portable_session_*'`).Scan(&leftover); err != nil || leftover != 0 {
			t.Fatalf("leftover portable_session_* tables = %d, err = %v", leftover, err)
		}
	})

	t.Run("backup_published_core_rejects_invalid_inputs", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		core, err := Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, root, time.Second, cjsiStoreLegacyManager{time.Second}); err != nil {
			t.Fatal(err)
		}
		managed, err := ResolveCJSIRuntimeRoot(ctx, root)
		if err != nil || managed == nil {
			t.Fatalf("ResolveCJSIRuntimeRoot = %v, %v", managed, err)
		}
		dst := filepath.Join(t.TempDir(), "snap.sqlite3")
		if err := managed.BackupPublishedCore(ctx, nil, dst); !errors.Is(err, ErrCJSIIndexChanged) {
			t.Fatalf("nil tx error = %v, want ErrCJSIIndexChanged", err)
		}
		if err := managed.WithPublishedCore(ctx, func(tx *sql.Tx, _ CJSIIndexStamp) error {
			return managed.BackupPublishedCore(ctx, tx, dst)
		}); err != nil {
			t.Fatalf("BackupPublishedCore failed: %v", err)
		}
		if err := ValidateCJSICoreArchive(ctx, dst); err != nil {
			t.Fatalf("ValidateCJSICoreArchive on fresh snapshot failed: %v", err)
		}
	})

	t.Run("legacy_migration_ignores_unvalidated_portable_tables", func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		staging := filepath.Join(dir, cjsiPreparedName)

		db, err := openWritableSQL(cjsiSQLiteURI(staging, "rwc"), "session")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, cjsiSessionBaselineSchema+`
			INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			VALUES('src.jsonl','id1',10,X'',10,1,1,'h1',1,6,'2026-01-01T00:00:00Z','');
			INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			VALUES('src.jsonl','codex','s1','proj','m1',6,'2026-01-01T00:00:00Z','2026-01-01T00:00:01Z');
			INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			VALUES(1,'src.jsonl','codex','s1','2026-01-01T00:00:00Z','user_prompt','A'),
			      (2,'src.jsonl','codex','s1','2026-01-01T00:00:01Z','assistant_message','B'),
			      (5,'src.jsonl','codex','s1','2026-01-01T00:00:02Z','user_prompt','C');
			CREATE TABLE portable_session_state(x);
			CREATE TABLE portable_session_groups(x);
			CREATE TABLE portable_session_order(x);`); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := ensureSessionIndexGeneration(ctx, db); err != nil {
			db.Close()
			t.Fatal(err)
		}
		var origEpoch int64
		if err := db.QueryRowContext(ctx, `SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&origEpoch); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		// Legacy migration path must ignore arbitrary portable_session_* tables and match 6d255ed.
		epoch, _, err := prepareCJSIProductionIndex(ctx, staging)
		if err != nil {
			t.Fatalf("legacy migration failed on residual portable_session_* tables: %v", err)
		}
		if epoch != origEpoch {
			t.Fatalf("legacy migration re-minted epoch: got %d want %d", epoch, origEpoch)
		}

		checkDB, err := sql.Open("sqlite", cjsiSQLiteURI(staging, "ro"))
		if err != nil {
			t.Fatal(err)
		}
		defer checkDB.Close()

		var highWater int64
		if err := checkDB.QueryRowContext(ctx, `SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1`).Scan(&highWater); err != nil {
			t.Fatal(err)
		}
		if highWater != 5 {
			t.Fatalf("doc_id_high_water = %d, want 5", highWater)
		}
		var verified int
		if err := checkDB.QueryRowContext(ctx, `SELECT order_verified FROM cjsi_base_groups WHERE source_path='src.jsonl' AND client='codex' AND session_id='s1'`).Scan(&verified); err != nil {
			t.Fatal(err)
		}
		if verified != 0 {
			t.Fatalf("order_verified = %d, want 0", verified)
		}
		var seq string
		if err := checkDB.QueryRowContext(ctx, `SELECT group_concat(doc_id, ',') FROM (SELECT doc_id FROM cjsi_base_order ORDER BY detail_ordinal)`).Scan(&seq); err != nil {
			t.Fatal(err)
		}
		if seq != "1,2,5" {
			t.Fatalf("legacy rowid order = %q, want 1,2,5", seq)
		}
	})

	t.Run("restore_rejects_unvalidated_portable_tables", func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		staging := filepath.Join(dir, cjsiPreparedName)

		db, err := openWritableSQL(cjsiSQLiteURI(staging, "rwc"), "session")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, cjsiSessionBaselineSchema+`
			CREATE TABLE portable_session_state(x);
			CREATE TABLE portable_session_groups(x);
			CREATE TABLE portable_session_order(x);`); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if _, _, err := prepareCJSIProductionIndexWithOptions(ctx, staging, true, true); err == nil {
			t.Fatal("restore path accepted malformed portable_session_* tables")
		}
	})
}
