package backup

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

const (
	activeSampleMarker     = "fixture-private-active-session"
	latestSampleMarker     = "fixture-private-finish-digest"
	historicalSampleMarker = "fixture-private-deleted-previous-endpoint"
)

func TestPortableBackupAndRestoreScrubQuotaSamples(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "source")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	installSampleTable(t, core.DB)
	seedSampleHistory(t, core.DB)

	sourcePath := filepath.Join(root, coreName)
	mainFile, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Contains(mainFile, []byte(historicalSampleMarker)) {
		t.Fatalf("fixture lacks deleted sample bytes in main database: %v", err)
	}
	wal, err := os.ReadFile(sourcePath + "-wal")
	if err != nil || !bytes.Contains(wal, []byte(latestSampleMarker)) {
		t.Fatalf("fixture lacks the latest private sample in the source WAL: %v", err)
	}

	service := Service{Core: core, StateRoot: root, Vault: testBackupVault(t, "source-machine")}
	archive := filepath.Join(t.TempDir(), "portable.adb")
	if _, err := service.Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	if count := rowCount(t, core.DB, quotaSampleTable); count != 2 {
		t.Fatalf("backup changed source samples: %d rows", count)
	}
	_, entries, err := readEncrypted(archive, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range entries {
		assertNoSampleMarkers(t, data)
	}
	if entries[coreName+"-wal"] != nil {
		t.Fatal("backup carried a WAL sidecar")
	}
	if count := rowCount(t, openCoreBytes(t, entries[coreName]).DB, quotaSampleTable); count != 0 {
		t.Fatalf("archive has %d sample rows", count)
	}

	external := tamperCore(t, archive, func(db *sql.DB) { seedSampleHistory(t, db) })
	_, externalEntries, err := readEncrypted(external, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{activeSampleMarker, latestSampleMarker, historicalSampleMarker} {
		if !bytes.Contains(externalEntries[coreName], []byte(marker)) {
			t.Fatalf("external fixture missing %q", marker)
		}
	}

	for name, path := range map[string]string{"portable": archive, "external": external} {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "target")
			if _, err := Restore(ctx, path, target, "passphrase", syntheticMachineIdentity("target-machine")); err != nil {
				t.Fatal(err)
			}
			restored, err := store.OpenReadOnly(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			if count := rowCount(t, restored.DB, quotaSampleTable); count != 0 {
				t.Fatalf("restore kept %d sample rows", count)
			}
			for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
				data, err := os.ReadFile(filepath.Join(target, coreName) + suffix)
				if suffix != "" && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				assertNoSampleMarkers(t, data)
			}
		})
	}
}

func TestPortableBackupAndRestoreFailOnSampleCleanupError(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "source")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	installSampleTable(t, core.DB)
	service := Service{Core: core, StateRoot: root, Vault: testBackupVault(t, "source-machine")}
	archive := filepath.Join(t.TempDir(), "portable.adb")
	if _, err := service.Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	seedFailure := func(db *sql.DB) {
		seedSampleHistory(t, db)
		execAll(t, db, `CREATE TRIGGER abort_scrub BEFORE DELETE ON quota_session_samples
			BEGIN SELECT RAISE(ABORT, 'synthetic sample scrub failure'); END`)
	}
	seedFailure(core.DB)
	blocked := filepath.Join(t.TempDir(), "blocked.adb")
	if _, err := service.Create(ctx, blocked, "passphrase", false); err == nil || !strings.Contains(err.Error(), "synthetic sample scrub failure") {
		t.Fatalf("backup result = %v", err)
	}
	if _, err := os.Stat(blocked); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("failed backup created an archive: %v", err)
	}
	if count := rowCount(t, core.DB, quotaSampleTable); count != 2 {
		t.Fatalf("failed backup changed source: %d rows", count)
	}

	external := tamperCore(t, archive, seedFailure)
	target := filepath.Join(t.TempDir(), "target")
	if _, err := Restore(ctx, external, target, "passphrase", syntheticMachineIdentity("target-machine")); err == nil ||
		!strings.Contains(err.Error(), "synthetic sample scrub failure") {
		t.Fatalf("restore result = %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("failed restore retained files: %v", err)
	}
}

func installSampleTable(t *testing.T, db *sql.DB) {
	t.Helper()
	var installed bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='quota_session_samples')`).Scan(&installed); err != nil {
		t.Fatal(err)
	}
	if !installed {
		schema, err := os.ReadFile("../store/quota_session_samples.sql")
		if err != nil {
			t.Fatal(err)
		}
		execAll(t, db, string(schema))
	}
}

func seedSampleHistory(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`PRAGMA secure_delete=OFF`)
	exec(`PRAGMA wal_autocheckpoint=0`)
	exec(`INSERT INTO quota_session_samples VALUES('claude','active','previous','previous','2026-11-19T12:00:00Z',NULL,?,NULL)`,
		`{"version":1,"account_digest":"`+strings.Repeat(historicalSampleMarker, 2000)+`"}`)
	exec(`DELETE FROM quota_session_samples WHERE generation='previous'`)
	exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	exec(`INSERT INTO quota_session_samples VALUES ('codex','active','active',?,'2026-11-19T12:00:00Z',NULL,'{"version":1}',NULL),
		('codex','latest','latest','other','2026-11-19T11:00:00Z','2026-11-19T12:00:00Z','{"version":1}',?)`,
		activeSampleMarker, `{"version":1,"account_digest":"`+latestSampleMarker+`"}`)
}

func assertNoSampleMarkers(t *testing.T, data []byte) {
	t.Helper()
	for _, marker := range []string{activeSampleMarker, latestSampleMarker, historicalSampleMarker} {
		if bytes.Contains(data, []byte(marker)) {
			t.Errorf("portable core retains %q", marker)
		}
	}
}
