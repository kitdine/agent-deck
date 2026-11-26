package backup

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIR1PortableOrderRejectsForgedGroupKeys(t *testing.T) {
	const base = `
		PRAGMA foreign_keys=OFF;
		CREATE TABLE session_sources(source_path);
		CREATE TABLE session_metadata(source_path,client,session_id);
		CREATE VIRTUAL TABLE session_documents USING fts5(source_path UNINDEXED,client UNINDEXED,session_id UNINDEXED,text);
		INSERT INTO portable_session_state VALUES(1,4);`
	for _, tc := range []struct{ name, changes string }{
		{"orphan_member", `
			INSERT INTO session_metadata VALUES('registered','codex','known');
			INSERT INTO portable_session_groups VALUES('registered','codex','known',1);
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES(4,'foreign','codex','orphan','fixture');
			INSERT INTO portable_session_order VALUES(4,'foreign','codex','orphan',1);`},
		{"case_folded_member", `
			ALTER TABLE portable_session_order RENAME TO original_order;
			CREATE TABLE portable_session_order(doc_id,source_path COLLATE NOCASE,client,session_id,detail_ordinal);
			INSERT INTO session_metadata VALUES('Registered','codex','known');
			INSERT INTO portable_session_groups VALUES('Registered','codex','known',1);
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES(4,'registered','codex','known','fixture');
			INSERT INTO portable_session_order VALUES(4,'registered','codex','known',1);`},
		{"non_text_empty_group", `
			ALTER TABLE portable_session_groups RENAME TO original_groups;
			CREATE TABLE portable_session_groups(source_path,client,session_id,order_verified);
			INSERT INTO session_metadata VALUES('registered',42,'empty');
			INSERT INTO portable_session_groups VALUES('registered',42,'empty',1);`},
		{"non_text_member", `
			ALTER TABLE portable_session_order RENAME TO original_order;
			CREATE TABLE portable_session_order(doc_id,source_path,client,session_id,detail_ordinal);
			INSERT INTO session_metadata VALUES('registered','codex','known');
			INSERT INTO portable_session_groups VALUES('registered','codex','known',1);
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES(4,'registered','codex',x'6b6e6f776e','fixture');
			INSERT INTO portable_session_order VALUES(4,'registered','codex',x'6b6e6f776e',1);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			path := filepath.Join(t.TempDir(), sessionsName)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.ExecContext(ctx, portableSessionOrderSchema+base+tc.changes)
			closeErr := db.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("fixture: %v %v", err, closeErr)
			}
			err = validateSessionArchive(ctx, readR1Fixture(t, path), sessionSnapshotSchemaVersion, true)
			if !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("tampered order accepted: %v", err)
			}
			if tc.name == "non_text_member" && !errors.Is(err, errPortableOrderKeyType) {
				t.Fatalf("malformed member key did not reach the type check: %v", err)
			}
		})
	}
}

func TestCJSIR1RestoreSanitizesUnfilteredLegacyArchive(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	core, coreSchema := r1CoreFixture(t, root)
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	index.DB.SetMaxOpenConns(1)
	_, err = index.DB.ExecContext(ctx, `PRAGMA secure_delete=OFF;
		INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
		VALUES('fixture','id',100,CAST('R1RestorePartialSentinel' AS BLOB),100,0,'prefix',1,8,'','R1RestoreParserSentinel');
		INSERT INTO session_metadata VALUES('fixture','codex','keep','allowed','',8,'',''),('fixture','codex','drop','excluded','',8,'','');
		INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text) VALUES
			(1,'fixture','codex','keep','','user_prompt','safe-body'),
			(2,'fixture','codex','drop','','user_prompt','R1RestoreExcludedSentinel'),
			(3,'fixture','codex','keep','','user_prompt','R1RestoreDeletedSentinel');
		DELETE FROM session_documents WHERE rowid=3;
		INSERT INTO session_exclusions VALUES('project','excluded');
		UPDATE serving_generation SET revision=70 WHERE singleton=1;`)
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := index.SessionIndexEpoch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), sessionsName)
	if err := index.Backup(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	unfiltered := readR1Fixture(t, snapshot)
	sentinels := []string{"R1RestorePartialSentinel", "R1RestoreParserSentinel", "R1RestoreExcludedSentinel", "R1RestoreDeletedSentinel"}
	for _, sentinel := range sentinels {
		if !bytes.Contains(unfiltered, []byte(sentinel)) {
			t.Fatalf("dirty archive fixture did not carry %s", sentinel)
		}
	}
	archive := writeR1Archive(t, coreSchema, core, unfiltered)
	target := filepath.Join(t.TempDir(), "restored")
	if _, err := Restore(ctx, archive, target, "r1-fixture", syntheticMachineIdentity("target")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(target, sessionsName)
	native := readR1Fixture(t, path)
	for _, sentinel := range sentinels {
		if bytes.Contains(native, []byte(sentinel)) {
			t.Errorf("Restore retained %s in native SQLite bytes", sentinel)
		}
	}
	restored, err := store.OpenSessionsReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var valid bool
	if err := restored.DB.QueryRowContext(ctx, `SELECT
		(SELECT cursor=? AND parser_version=0 AND partial_line=X'' AND parser_context='' FROM session_sources WHERE source_path='fixture')
		AND (SELECT revision>70 FROM serving_generation WHERE singleton=1)
		AND (SELECT count(*)=1 FROM session_documents)
		AND EXISTS(SELECT 1 FROM session_documents WHERE rowid=1 AND text='safe-body')
		AND EXISTS(SELECT 1 FROM session_exclusions WHERE kind='project' AND value='excluded')`, 100-len(sentinels[0])).Scan(&valid); err != nil || !valid {
		t.Fatalf("restored rows invalid: %v", err)
	}
	if got, err := restored.SessionIndexEpoch(ctx); err != nil || got == epoch {
		t.Fatalf("restored epoch=%d, source=%d, error=%v", got, epoch, err)
	}
}

func TestCJSIR1RestoreRejectsReservedLegacyCoreCatalog(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	_, coreSchema := r1CoreFixture(t, root)
	db, err := sql.Open("sqlite", filepath.Join(root, coreName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE VIEW CjSi_unexpected AS SELECT 1"); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), coreName)
	if err := store.BackupSQLiteFile(ctx, filepath.Join(root, coreName), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	archive := writeR1Archive(t, coreSchema, readR1Fixture(t, snapshot), nil)
	target := filepath.Join(t.TempDir(), "never-created")
	if _, err := Restore(ctx, archive, target, "r1-fixture", syntheticMachineIdentity("target")); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("unexpected restore error: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("restore touched destination: %v", err)
	}
}

func TestCJSIR1SnapshotEscapesSQLiteURIDelimiters(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	for _, name := range []string{"question?.sqlite3", "fragment#.sqlite3"} {
		t.Run(name, func(t *testing.T) {
			legacy, err := store.OpenSessions(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(root, name)
			data := filepath.Join(t.TempDir(), sessionsName)
			if err := legacy.Backup(ctx, data); err != nil {
				t.Fatal(err)
			}
			if err := legacy.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(data, source); err != nil {
				t.Fatal(err)
			}
			clean := filepath.Join(root, "clean-"+name)
			if err := preparePortableSession(ctx, source, clean); err != nil {
				t.Fatal(err)
			}
			uri := url.URL{Scheme: "file", Path: clean, RawQuery: "mode=ro"}
			db, err := sql.Open("sqlite", uri.String())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var integrity string
			if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("clean snapshot integrity=%q err=%v", integrity, err)
			}
		})
	}
}

func readR1Fixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func r1CoreFixture(t *testing.T, root string) ([]byte, int) {
	t.Helper()
	ctx := t.Context()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	version, err := core.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), coreName)
	if err := core.Backup(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	return readR1Fixture(t, snapshot), version
}

func writeR1Archive(t *testing.T, coreSchema int, core, index []byte) string {
	t.Helper()
	contents := map[string][]byte{coreName: core, credentialsName: []byte("[]")}
	manifest := Manifest{SchemaVersion: ManifestSchemaVersion, CreatedAt: time.Unix(1, 0).UTC(),
		DatabaseSchemas: map[string]int{coreName: coreSchema}}
	if index != nil {
		contents[sessionsName] = index
		manifest.DatabaseSchemas[sessionsName] = sessionSnapshotSchemaVersion
	}
	manifest.Included = sortedKeys(contents)
	for _, name := range manifest.Included {
		data := contents[name]
		manifest.Entries = append(manifest.Entries, Entry{Name: name, Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	var err error
	contents[manifestName], err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "raw.adb")
	if err := writeEncrypted(path, "r1-fixture", contents, manifest.CreatedAt); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCJSIR2PortableOrderUsesBinaryGroupIdentity(t *testing.T) {
	const collatedTables = `
		CREATE TABLE session_sources(source_path);
		CREATE TABLE session_metadata(source_path COLLATE NOCASE,client COLLATE NOCASE,session_id COLLATE NOCASE);
		CREATE TABLE portable_session_groups(source_path COLLATE NOCASE,client COLLATE NOCASE,session_id COLLATE NOCASE,order_verified);
		CREATE TABLE portable_session_order(doc_id,source_path COLLATE NOCASE,client COLLATE NOCASE,session_id COLLATE NOCASE,detail_ordinal);
		CREATE TABLE portable_session_state(singleton,doc_id_high_water);
		INSERT INTO portable_session_state VALUES(1,4);`
	const twoGroups = `
		INSERT INTO session_metadata VALUES('A','codex','s'),('a','codex','s');
		INSERT INTO portable_session_groups VALUES('A','codex','s',1),('a','codex','s',1);
		CREATE VIRTUAL TABLE session_documents USING fts5(source_path UNINDEXED,client UNINDEXED,session_id UNINDEXED,text);
		INSERT INTO session_documents(rowid,source_path,client,session_id,text)
		VALUES (1,'A','codex','s','d1'),(2,'a','codex','s','d2');`
	const matchingFirstKey = `
		INSERT INTO session_metadata VALUES('Registered','codex','known');
		INSERT INTO portable_session_groups VALUES('Registered','codex','known',1);`
	const noDocuments = `CREATE VIRTUAL TABLE session_documents USING fts5(source_path UNINDEXED,client UNINDEXED,session_id UNINDEXED,text);`
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"case_distinct_groups_are_valid", twoGroups + `
			INSERT INTO portable_session_order VALUES(1,'A','codex','s',1),(2,'a','codex','s',1);`, true},
		{"folded_groups_hide_ordinal_gap", twoGroups + `
			INSERT INTO portable_session_order VALUES(1,'A','codex','s',1),(2,'a','codex','s',2);`, false},
		{"group_and_metadata_disagree", `
			INSERT INTO session_metadata VALUES('Registered','codex','known');
			INSERT INTO portable_session_groups VALUES('registered','codex','known',1);
			CREATE VIRTUAL TABLE session_documents USING fts5(source_path UNINDEXED,client UNINDEXED,session_id UNINDEXED,text);
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES(4,'registered','codex','known','fixture');
			INSERT INTO portable_session_order VALUES(4,'registered','codex','known',1);`, false},
		{"extra_group", matchingFirstKey + noDocuments + `
			INSERT INTO portable_session_groups VALUES('registered','codex','known',1);`, false},
		{"extra_metadata", matchingFirstKey + noDocuments + `
			INSERT INTO session_metadata VALUES('registered','codex','known');`, false},
		{"document_group_mismatch", matchingFirstKey + `
			CREATE TABLE session_documents(source_path COLLATE NOCASE,client COLLATE NOCASE,session_id COLLATE NOCASE,text);
			INSERT INTO session_documents(rowid,source_path,client,session_id,text) VALUES(4,'registered','codex','known','fixture');
			INSERT INTO portable_session_order VALUES(4,'Registered','codex','known',1);`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			name := filepath.Join(t.TempDir(), sessionsName)
			db, err := sql.Open("sqlite", name)
			if err != nil {
				t.Fatal(err)
			}
			_, seedErr := db.ExecContext(ctx, collatedTables+tc.data)
			if err := errors.Join(seedErr, db.Close()); err != nil {
				t.Fatal(err)
			}
			err = validateSessionArchive(ctx, readR1Fixture(t, name), sessionSnapshotSchemaVersion, true)
			if tc.valid && err != nil || !tc.valid && !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("archive valid=%t, validation error=%v", tc.valid, err)
			}
		})
	}
}
