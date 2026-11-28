package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestQuotaSampleMigrationStorageAndSessionRemoval(t *testing.T) {
	ctx := context.Background()
	core, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()

	var installed bool
	if err := core.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE type='table' AND name='quota_session_samples')`).Scan(&installed); err != nil {
		t.Fatal(err)
	}
	if !installed {
		tx, err := core.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := applyQuotaSessionSamples(ctx, tx); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	execSQL := func(statement string, args ...any) {
		t.Helper()
		if _, err := core.DB.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	countSamples := func(want int) {
		t.Helper()
		var count int
		if err := core.DB.QueryRowContext(ctx, `SELECT count(*) FROM quota_session_samples`).Scan(&count); err != nil || count != want {
			t.Fatalf("sample rows=%d, want %d: %v", count, want, err)
		}
	}
	const now = "2026-11-19T12:00:00.123Z"
	const insert = `INSERT INTO quota_session_samples VALUES(?,?,?, ?,?,?, '{}',?)`
	execSQL(`INSERT INTO usage_sessions(client,session_id,first_at,last_at)
		VALUES ('codex','same',?,?),('claude','same',?,?),('codex','other',?,?)`, now, now, now, now, now, now)
	execSQL(insert, "codex", "active", "a", "same", now, nil, nil)
	execSQL(insert, "codex", "latest", "b", "same", now, now, "{}")
	execSQL(insert, "claude", "latest", "c", "same", now, now, "{}")
	countSamples(3)
	for _, args := range [][]any{
		{"claude", "other", "d", "same", now, nil, nil},
		{"claude", "active", "", "same", now, nil, nil},
		{"claude", "active", "a", "same", now, nil, nil},
		{"claude", "active", "d", "same", now, now, "{}"},
		{"codex", "active", "d", "other", now, nil, nil},
	} {
		if _, err := core.DB.ExecContext(ctx, insert, args...); err == nil {
			t.Fatalf("invalid sample accepted: %v", args)
		}
	}
	execSQL(`UPDATE usage_sessions SET last_at=? WHERE client='codex' AND session_id='same'`, now)
	countSamples(3)

	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_sessions WHERE client='codex' AND session_id='same'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	countSamples(3)
	execSQL(`DELETE FROM usage_sessions WHERE client='codex' AND session_id='same'`)
	countSamples(1)

	// A clear in the usage data layer also clears the remaining sample.
	execSQL(`DELETE FROM usage_sessions`)
	countSamples(0)
}

func TestQuotaSampleNumberedUpgrade(t *testing.T) {
	const pricingSchema = PricingFactsSchemaVersion
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "core.sqlite")+"?"+writablePragmas)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(ctx, db, migrations[:pricingSchema]); err != nil {
		t.Fatal(err)
	}
	assertState := func(wantVersion int, wantSamples bool) {
		t.Helper()
		version, err := schemaVersion(ctx, db)
		if err != nil || version != wantVersion {
			t.Fatalf("version=%d, want %d: %v", version, wantVersion, err)
		}
		var pricing, samples bool
		err = db.QueryRowContext(ctx, `SELECT
			EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='price_rule_sets'),
			EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='quota_session_samples')`).Scan(&pricing, &samples)
		if err != nil || !pricing || samples != wantSamples {
			t.Fatalf("pricing=%t samples=%t, want true/%t: %v", pricing, samples, wantSamples, err)
		}
	}
	assertState(pricingSchema, false)
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER quota_samples_on_session_delete AFTER DELETE ON usage_sessions BEGIN SELECT 1; END`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); err == nil {
		t.Fatal("migration succeeded despite conflicting trigger")
	}
	assertState(pricingSchema, false)
	if _, err := db.ExecContext(ctx, `DROP TRIGGER quota_samples_on_session_delete`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	assertState(CurrentSchemaVersion, true)
}
