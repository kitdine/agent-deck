package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openCoreAtVersion(t *testing.T, root string, version int) *Store {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "agentdeck.sqlite3")
	if err := preparePrivateSQLiteFiles(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := bootstrapAndMigrate(context.Background(), db, migrations[:version]); err != nil {
		t.Fatal(err)
	}
	return &Store{DB: db, path: path}
}

func TestQuotaIdentityUpgradePreservesLegacyRows(t *testing.T) {
	ctx := context.Background()
	for _, version := range []int{27, 28, 29, 30, 35} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			root := t.TempDir()
			legacy := openCoreAtVersion(t, root, version)
			if _, err := legacy.DB.Exec(`INSERT INTO quota_envelopes(client,plan,failure) VALUES
				('codex','prolite',''),('claude','','parse_failed');
				INSERT INTO quota_windows(client,account_id,window_key,source,observed_at,used_percent,resets_at)
				VALUES('codex','legacy-digest','weekly','codex','2026-11-01T00:00:00Z',37,'2026-11-02T00:00:00Z')`); err != nil {
				t.Fatal(err)
			}
			core, err := Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			defer core.Close()
			var plan, digest, failure, failureAt string
			var rejected int
			err = core.DB.QueryRow(`SELECT e.plan,e.plan_unsafe,w.account_id,c.failure,c.failure_observed_at
				FROM quota_envelopes_v2 e JOIN quota_windows_v2 w ON e.client=w.client
				JOIN quota_envelopes_v2 c ON c.client='claude' WHERE e.client='codex'`).
				Scan(&plan, &rejected, &digest, &failure, &failureAt)
			if err != nil || plan != "prolite" || rejected != 0 || digest != "legacy-digest" || failure != "parse_failed" || failureAt != "" {
				t.Fatalf("legacy values: %q %d %q %q %q: %v", plan, rejected, digest, failure, failureAt, err)
			}
		})
	}
}

func TestQuotaIdentityMigrationRejectsPreparedLegacyWriter(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	legacy := openCoreAtVersion(t, root, 35)
	oldSQL := "INSERT INTO quota_envelopes(client,plan) VALUES('codex',?) ON CONFLICT(client) DO UPDATE SET plan=excluded.plan"
	conn, err := legacy.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, oldSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec("forbidden@example.test"); err != nil {
		t.Fatal(err)
	}
	core, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	if _, err := stmt.Exec("late-plan"); err == nil || !strings.Contains(err.Error(), "no such table") {
		t.Fatalf("pre-migration prepared writer = %v", err)
	}
	var plan, reason string
	var unsafe bool
	if err := core.DB.QueryRow("SELECT plan,plan_reason,plan_unsafe FROM quota_envelopes_v2 WHERE client='codex'").
		Scan(&plan, &reason, &unsafe); err != nil || plan != "" || reason != "not_reported" || !unsafe {
		t.Fatalf("migrated safety classification %q/%q/%v: %v", plan, reason, unsafe, err)
	}
	version, err := schemaVersion(ctx, legacy.DB)
	if err != nil || version != CurrentSchemaVersion {
		t.Fatalf("reopen schema version = %d: %v", version, err)
	}
	if err := checkSchemaAhead(version, 35); !errors.Is(err, ErrSchemaAhead) {
		t.Fatalf("old supported migration list reopening = %v", err)
	}
}

func TestQuotaIdentityRevisionAndRevalidationTriggers(t *testing.T) {
	core, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	if _, err := core.DB.Exec(`UPDATE quota_cycle SET seq=2; INSERT INTO settings VALUES('quota.probe','true');
		UPDATE quota_cycle SET seq=4; UPDATE settings SET value='true' WHERE key='quota.probe';
		UPDATE settings SET value='false' WHERE key='quota.probe';
		UPDATE quota_cycle SET seq=6; UPDATE settings SET value='false' WHERE key='quota.probe';
		INSERT INTO quota_identities(client) VALUES('claude');
		INSERT INTO quota_envelopes_v2(client) VALUES('claude')`); err != nil {
		t.Fatal(err)
	}
	var revision, floor, serving int
	if err := core.DB.QueryRow(`SELECT revision,revalidate_seq,(SELECT revision FROM serving_generation)
		FROM quota_projection_revision JOIN quota_cycle USING(singleton)`).Scan(&revision, &floor, &serving); err != nil ||
		revision != 6 || floor != 4 || serving < 2 {
		t.Fatalf("revision=%d floor=%d serving=%d err=%v", revision, floor, serving, err)
	}
}
