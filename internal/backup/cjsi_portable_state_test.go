package backup

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIPortableState(t *testing.T) {
	t.Run("cleans_legacy_index_and_keeps_safe_rows", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		core, err := store.Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		index, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer index.Close()

		if _, err := index.DB.ExecContext(ctx, `INSERT INTO session_sources
			(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			VALUES ('fixture','fixture',31,CAST('PortableTailSentinel' AS BLOB),31,0,'prefix',1,8,'','PortableContextSentinel');
			INSERT INTO session_metadata VALUES ('fixture','codex','keep','keep','',8,'',''),('fixture','codex','drop','drop','',8,'','');
			INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text) VALUES
				(1,'fixture','codex','keep','','user_prompt','SafePortableBody'),
				(3,'fixture','codex','drop','','user_prompt','PortableRevokedSentinel'),
				(4,'fixture','codex','keep','','user_prompt','PortableDeletedSentinel'),
				(5,'unregistered','codex','gone','','user_prompt','PortableDirectRevocationSentinel');
			DELETE FROM session_documents WHERE rowid=4;
			INSERT INTO session_exclusions VALUES ('project','drop'),('path','unregistered')`); err != nil {
			t.Fatal(err)
		}
		epoch, err := index.SessionIndexEpoch(ctx)
		if err != nil {
			t.Fatal(err)
		}
		service := Service{Core: core, StateRoot: root, Vault: testBackupVault(t, "source")}
		archive := filepath.Join(t.TempDir(), "safe.adb")
		if _, err := service.Create(ctx, archive, "test", true); err != nil {
			t.Fatal(err)
		}
		_, entries, err := readEncrypted(archive, "test")
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range []string{"PortableTailSentinel", "PortableContextSentinel", "PortableRevokedSentinel", "PortableDeletedSentinel", "PortableDirectRevocationSentinel"} {
			if bytes.Contains(entries[sessionsName], []byte(sentinel)) {
				t.Fatalf("archive retained %s", sentinel)
			}
		}
		target := filepath.Join(t.TempDir(), "restored")
		if _, err := Restore(ctx, archive, target, "test", syntheticMachineIdentity("target")); err != nil {
			t.Fatal(err)
		}
		restored, err := store.OpenSessionsReadOnly(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		defer restored.Close()
		newEpoch, err := restored.SessionIndexEpoch(ctx)
		if err != nil || newEpoch == epoch {
			t.Fatalf("restore epoch = %d, original = %d, err = %v", newEpoch, epoch, err)
		}
		for _, query := range []string{
			`SELECT count(*) FROM session_documents WHERE rowid=1 AND text='SafePortableBody'`,
			`SELECT count(*) FROM session_documents`,
			`SELECT count(*) FROM session_exclusions WHERE kind='project' AND value='drop'`,
			`SELECT count(*) FROM session_sources WHERE cursor=11 AND length(partial_line)=0 AND parser_context='' AND parser_version=0`,
		} {
			var count int
			if err := restored.DB.QueryRowContext(ctx, query).Scan(&count); err != nil || count != 1 {
				t.Fatalf("%s = %d, err = %v", query, count, err)
			}
		}
		var present int
		if err := index.DB.QueryRowContext(ctx, "SELECT count(*) FROM session_documents").Scan(&present); err != nil || present != 3 {
			t.Fatalf("live index changed: count=%d, err=%v", present, err)
		}
	})
	t.Run("refuses_unbound_core_even_without_sessions", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		core, err := store.Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		if _, err := core.DB.ExecContext(ctx, "CREATE TABLE cjsi_publication_head(ticket INTEGER)"); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "no.adb")
		_, err = (Service{Core: core, StateRoot: root, Vault: testBackupVault(t, "source")}).Create(ctx, destination, "test", false)
		if !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("backup error = %v", err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("destination was published: %v", err)
		}
	})
	t.Run("rejects_private_index_archive_entry", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		index, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := index.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, sessionsName)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "CREATE TABLE CJSI_candidate_payloads(text TEXT)"); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		snapshot := filepath.Join(t.TempDir(), sessionsName)
		if err := store.BackupSQLiteFile(ctx, path, snapshot); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateSessionSnapshot(ctx, data, sessionSnapshotSchemaVersion); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("snapshot validation = %v", err)
		}
	})
	t.Run("validates_portable_order_and_high_water", func(t *testing.T) {
		ctx := context.Background()
		path := filepath.Join(t.TempDir(), sessionsName)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		run := func(query string) {
			t.Helper()
			if _, err := db.ExecContext(ctx, query); err != nil {
				t.Fatal(err)
			}
		}
		run(portableSessionOrderSchema + `
			CREATE TABLE session_sources(source_path);
			CREATE TABLE session_metadata(source_path,client,session_id);
			CREATE VIRTUAL TABLE session_documents USING fts5(source_path,client,session_id,text);
			INSERT INTO session_metadata VALUES('fixture','codex','thread'),('fixture','claude','empty');
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES
				(1,'fixture','codex','thread','A'),(2,'fixture','codex','thread','B'),
				(3,'fixture','codex','thread','C'),(4,'fixture','codex','thread','X');
			INSERT INTO portable_session_state VALUES(1,12);
			INSERT INTO portable_session_groups VALUES('fixture','codex','thread',1),('fixture','claude','empty',0);
			INSERT INTO portable_session_order VALUES(1,'fixture','codex','thread',1),(4,'fixture','codex','thread',2),
				(2,'fixture','codex','thread',3),(3,'fixture','codex','thread',4)`)
		if err := validatePortableSessionOrder(ctx, db); err != nil {
			t.Fatal(err)
		}
		var text string
		err = db.QueryRowContext(ctx, `SELECT group_concat(text,'') FROM
			(SELECT d.text FROM portable_session_order o JOIN session_documents d ON d.rowid=o.doc_id
				ORDER BY detail_ordinal)`).Scan(&text)
		if err != nil || text != "AXBC" {
			t.Fatalf("order=%q, err=%v", text, err)
		}
		for _, edit := range []struct{ breakSQL, repairSQL string }{
			{"UPDATE portable_session_order SET detail_ordinal=5 WHERE doc_id=4", "UPDATE portable_session_order SET detail_ordinal=2 WHERE doc_id=4"},
			{"UPDATE portable_session_state SET doc_id_high_water=3", "UPDATE portable_session_state SET doc_id_high_water=12"},
			{"UPDATE session_documents SET client='claude' WHERE rowid=4", "UPDATE session_documents SET client='codex' WHERE rowid=4"},
			{"DELETE FROM portable_session_groups WHERE session_id='empty'", "INSERT INTO portable_session_groups VALUES('fixture','claude','empty',0)"},
		} {
			run(edit.breakSQL)
			if err := validatePortableSessionOrder(ctx, db); !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("invalid order accepted after %q: %v", edit.breakSQL, err)
			}
			run(edit.repairSQL)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateSessionArchive(ctx, data, sessionSnapshotSchemaVersion, true); err != nil {
			t.Fatal(err)
		}
		if err := validateSessionSnapshot(ctx, data, sessionSnapshotSchemaVersion); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("legacy validation accepted managed order: %v", err)
		}
		run("DROP TABLE portable_session_order")
		data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateSessionArchive(ctx, data, sessionSnapshotSchemaVersion, true); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("managed validation accepted missing order: %v", err)
		}

	})
}
