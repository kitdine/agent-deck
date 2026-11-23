package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// The usage-only reducer stages checkpoints without opening the session index,
// and a never-built root has no index at all. Its workset therefore belongs to
// the managed core migration, beside the layout journal.
func TestCJSICheckpointWorksetMigration(t *testing.T) {
	want := cjsiReferenceWorksetSchema(t)

	t.Run("schema 35 upgrade installs and rolls back atomically", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		db := c.openCore(t, "rw")
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", 35)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices WHERE notified_at='retained'", 1)
		cjsiFixtureExec(t, db, fmt.Sprintf(`CREATE TRIGGER abort_cjsi BEFORE UPDATE OF version ON schema_metadata WHEN NEW.version=%d BEGIN SELECT RAISE(ABORT,'capsule rollback'); END`, cjsiLayoutCoreSchemaVersion))
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err == nil {
			t.Fatal("managed migration committed despite abort trigger")
		}
		db = c.openCore(t, "rw")
		// Migration 36 committed before migration 37 rolled back.
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE notified_at='retained'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_alert_notices'", 0)
		if got := cjsiWorksetSchemaObjects(t, db); len(got) != 0 {
			t.Fatalf("rolled-back migration left workset objects: %v", got)
		}
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
		cjsiFixtureExec(t, db, "DROP TRIGGER abort_cjsi")
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db = c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE notified_at='retained'", 1)
		if got := cjsiWorksetSchemaObjects(t, db); !reflect.DeepEqual(got, want) {
			t.Fatalf("installed workset schema:\n%v\nwant approved DDL:\n%v", got, want)
		}
	})

	t.Run("a workset install failure commits nothing of managed migration", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		db := c.openCore(t, "rw")
		// The fragment's last statement collides after its tables were created.
		cjsiFixtureExec(t, db, "CREATE TABLE cjsi_checkpoint_call(conflict INTEGER)")
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err == nil {
			t.Fatal("managed migration committed although its workset failed")
		}
		db = c.openCore(t, "rw")
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
		conflict := []cjsiSchemaObject{{"table", "cjsi_checkpoint_call", "cjsi_checkpoint_call", "CREATE TABLE cjsi_checkpoint_call(conflict INTEGER)"}}
		if got := cjsiWorksetSchemaObjects(t, db); !reflect.DeepEqual(got, conflict) {
			t.Fatalf("failed migration left partial workset objects: %v", got)
		}
		for _, name := range []string{cjsiLegacyIndexName, cjsiPrivateIndexName} {
			if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, name)); err != nil {
				t.Fatal(err)
			}
		}
		cjsiFixtureExec(t, db, "DROP TABLE cjsi_checkpoint_call")
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db = c.openCore(t, "ro")
		defer db.Close()
		if got := cjsiWorksetSchemaObjects(t, db); !reflect.DeepEqual(got, want) {
			t.Fatalf("retried workset schema:\n%v\nwant approved DDL:\n%v", got, want)
		}
	})

	t.Run("cold bootstrap installs the same schema", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db := c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		if got := cjsiWorksetSchemaObjects(t, db); !reflect.DeepEqual(got, want) {
			t.Fatalf("cold workset schema:\n%v\nwant approved DDL:\n%v", got, want)
		}
	})

	t.Run("the real adapter reduces on the migrated core", func(t *testing.T) {
		ctx := t.Context()
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		core := openCJSIWorksetFixture(t, filepath.Join(c.root, "agentdeck.sqlite3"))
		// Same snapshot as the locator: layout is ready and never built.
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != (cjsiSessionLocation{route: cjsiRouteAbsent}) {
			t.Fatalf("migrated core location = %+v, %v", got, err)
		}
		input := cjsiWorksetIntegrationInput(t)
		file, err := os.Create(filepath.Join(t.TempDir(), "synthetic-source.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		if _, err := file.WriteString(input); err != nil {
			t.Fatal(err)
		}
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		identity, _, stable := ingest.FileGeneration(info)
		if !stable {
			t.Fatal("fixture lacks file identity")
		}
		b := cjsiCheckpointBinding{Round: 1, Attempt: 1, Identity: identity, CaptureEnd: int64(len(input)),
			Version: ingest.CodexCheckpointVersion{Generation: 7, Parser: 9, MaxLineBytes: 4096}}
		attempt, err := beginCJSICheckpointWorkset(ctx, core, b)
		if err != nil {
			t.Fatal(err)
		}
		replay, step := cjsiFinishIntegrationReplay(t, file, attempt, nil)
		if err := attempt.Seal(ctx, step); err != nil {
			t.Fatal(err)
		}
		cjsiCheckSQLiteReduction(t, file, b, replay, &cjsiProvisionalProofs{})
		if err := attempt.Retire(ctx); err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, core, "SELECT count(*) FROM cjsi_checkpoint_records", 0)
		assertCJSIMigrationNumber(t, core, "SELECT count(*) FROM cjsi_checkpoint_attempts WHERE state='aborted'", 1)
		if _, err := beginCJSICheckpointWorkset(ctx, core, b); err == nil {
			t.Fatal("retired attempt key was reusable on the migrated core")
		}
		if _, err := core.ExecContext(ctx, `INSERT INTO cjsi_checkpoint_records VALUES(9,9,0,1,zeroblob(32),'k','t','u','c','{}')`); err == nil {
			t.Fatal("core connection did not enforce the workset foreign key")
		}
	})
}

type cjsiSchemaObject struct{ kind, name, table, sql string }

func cjsiWorksetSchemaObjects(t *testing.T, db *sql.DB) []cjsiSchemaObject {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT type, name, tbl_name, coalesce(sql,'') FROM sqlite_master
		WHERE name LIKE 'cjsi_checkpoint%' OR tbl_name LIKE 'cjsi_checkpoint%' ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var objects []cjsiSchemaObject
	for rows.Next() {
		var object cjsiSchemaObject
		if err := rows.Scan(&object.kind, &object.name, &object.table, &object.sql); err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	return objects
}

// The approved fragment applied alone, as its adapter tests use it.
func cjsiReferenceWorksetSchema(t *testing.T) []cjsiSchemaObject {
	t.Helper()
	db := openCJSIWorksetFixture(t, filepath.Join(t.TempDir(), "reference.sqlite"))
	if _, err := db.ExecContext(t.Context(), cjsiCheckpointWorksetSchema); err != nil {
		t.Fatal(err)
	}
	objects := cjsiWorksetSchemaObjects(t, db)
	var tables, indexes int
	for _, object := range objects {
		switch {
		case object.kind == "table":
			tables++
		case object.kind == "index" && object.sql != "":
			indexes++
		}
	}
	if tables != 2 || indexes != 3 {
		t.Fatalf("approved workset fragment defines %v, want 2 tables and 3 declared indexes", objects)
	}
	return objects
}
