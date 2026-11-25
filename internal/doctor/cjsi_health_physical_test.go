//go:build cjsi_adapter_fixture

package doctor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestCJSIR1PhysicalDatabaseHealth(t *testing.T) {
	t.Run("opened_expired_slot", func(t *testing.T) {
		s, core := r1ManagedDoctor(t, true)
		path, data := r1OpenedSlot(t, s.StateRoot, core)
		quick := r1DoctorReport(t, s, false)
		assertR1Check(t, quick, "schema", "", "ok")
		assertR1Check(t, quick, "sessions", "", "ok")
		for _, check := range quick.Checks {
			if check.Name == "database_integrity" || check.Name == "session_sources" {
				t.Fatalf("quick check performed full-only inspection: %+v", check)
			}
		}
		full := r1DoctorReport(t, s, true)
		assertR1Check(t, full, "schema", "", "ok")
		assertR1Check(t, full, "database_integrity", "", "ok")
		assertR1Check(t, full, "session_sources", "", "ok")
		assertR1FenceRejectsSQLite(t, s.StateRoot)

		if err := os.WriteFile(path, bytes.Repeat([]byte{'X'}, len(data)), 0600); err != nil {
			t.Fatal(err)
		}
		assertR1Check(t, r1DoctorReport(t, s, false), "schema", "schema_incompatible", "error")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		assertR1Check(t, r1DoctorReport(t, s, true), "schema", "schema_incompatible", "error")
	})
	t.Run("integrity_check_fails_only_in_full_mode", func(t *testing.T) {
		s, core := r1ManagedDoctor(t, false)
		r1DamageUnusedIndexPage(t, s.StateRoot, core)
		var integrity string
		probe := r1OpenDatabase(t, filepath.Join(s.StateRoot, "agentdeck.sqlite3"), "ro")
		if err := probe.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity == "ok" {
			t.Fatalf("fixture integrity=%q err=%v", integrity, err)
		}
		quick := r1DoctorReport(t, s, false)
		assertR1Check(t, quick, "schema", "", "ok")
		assertR1Check(t, quick, "sessions", "session_index_missing", "warning")
		if check := findCheck(quick, "database_integrity", "integrity_failed"); check != nil {
			t.Fatalf("quick mode ran integrity check: %+v", check)
		}
		full := r1DoctorReport(t, s, true)
		assertR1Check(t, full, "schema", "", "ok")
		assertR1Check(t, full, "database_integrity", "integrity_failed", "error")
	})
	t.Run("damaged_fence", func(t *testing.T) {
		s, _ := r1ManagedDoctor(t, false)
		path := filepath.Join(s.StateRoot, "sessions.sqlite3")
		if err := os.WriteFile(path, []byte("synthetic damaged fence"), 0600); err != nil {
			t.Fatal(err)
		}
		assertR1Check(t, r1DoctorReport(t, s, false), "schema", "schema_incompatible", "error")
	})
	t.Run("installation_pending", func(t *testing.T) {
		s, core := r1ManagedDoctor(t, false)
		r1Exec(t, core, `UPDATE cjsi_layout_journal SET phase=1,fence_identity=NULL`)
		assertR1Check(t, r1DoctorReport(t, s, true), "schema", "schema_incompatible", "error")
	})
}

func TestCJSIR1ManagedDatabaseLockRefusals(t *testing.T) {
	t.Run("sqlite_exclusive", func(t *testing.T) {
		s, core := r1ManagedDoctor(t, false)
		if err := store.CloseReadPools(); err != nil {
			t.Fatal(err)
		}
		r1Exec(t, core, "PRAGMA journal_mode=DELETE; BEGIN EXCLUSIVE")
		defer r1Exec(t, core, "ROLLBACK")
		// Exercise the managed route directly. The initial legacy probe in
		// Service.Check has its own timeout and can be the first reader refused.
		report, err := s.checkCJSIDatabase(t.Context(), false, newReport(false))
		if err != nil {
			t.Fatal(err)
		}
		assertR1Check(t, report, "database", "database_unreadable", "error")
		if reportCode(report, "schema_incompatible") {
			t.Fatal("exclusive SQLite lock reported as a schema defect")
		}
	})
	t.Run("root_admission", func(t *testing.T) {
		for _, late := range []bool{false, true} {
			name := "before_initial_open"
			if late {
				name = "after_layout_probe"
			}
			t.Run(name, func(t *testing.T) {
				s, _ := r1ManagedDoctor(t, false)
				fd, err := unix.Open(s.StateRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer unix.Close(fd)
				locked := false
				lock := func() {
					if err := store.CloseReadPools(); err != nil {
						t.Fatal(err)
					}
					if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
						t.Fatal(err)
					}
					locked = true
				}
				if late {
					s.cjsiLayoutProbe = func(ctx context.Context, root string) (bool, error) {
						managed, err := hasCJSILayout(ctx, root)
						if err != nil || !managed {
							t.Fatalf("fixture layout probe: managed=%v err=%v", managed, err)
						}
						lock()
						return managed, nil
					}
				} else {
					lock()
				}
				assertR2UnreadableDatabase(t, r1DoctorReport(t, s, false))
				if !locked {
					t.Fatal("the requested lock window was not exercised")
				}
			})
		}
	})
}

func TestCJSIR2ServiceCheckReportsProbeAndPrivateIndexBusy(t *testing.T) {
	t.Run("layout_probe", func(t *testing.T) {
		s, core := r1ManagedDoctor(t, false)
		probed := false
		s.cjsiLayoutProbe = func(ctx context.Context, root string) (bool, error) {
			probed = true
			// The legacy opener has already returned SchemaAhead. Acquire
			// SQLite's lock before the separate managed-layout probe runs.
			r2LockSQLite(t, core)
			defer r1Exec(t, core, "ROLLBACK")
			managed, err := hasCJSILayout(ctx, root)
			var sqliteErr *sqlite.Error
			if managed || !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_BUSY {
				t.Fatalf("layout contention fixture got managed=%t err=%v", managed, err)
			}
			return managed, err
		}
		assertR2UnreadableDatabase(t, r1DoctorReport(t, s, false))
		if !probed {
			t.Fatal("layout probe did not run")
		}
	})
	t.Run("selected_private_index", func(t *testing.T) {
		s, _ := r1ManagedDoctor(t, true)
		index := r1OpenDatabase(t, filepath.Join(s.StateRoot, "session-index-v1.sqlite3"), "rw")
		r2LockSQLite(t, index)
		defer r1Exec(t, index, "ROLLBACK")
		assertR2UnreadableDatabase(t, r1DoctorReport(t, s, true))
	})
}

func r2LockSQLite(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := store.CloseReadPools(); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := db.QueryRowContext(t.Context(), "PRAGMA journal_mode=DELETE").Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("rollback journal mode=%q err=%v", mode, err)
	}
	r1Exec(t, db, "BEGIN EXCLUSIVE")
}

func assertR2UnreadableDatabase(t *testing.T, report Report) {
	t.Helper()
	assertR1Check(t, report, "database", "database_unreadable", "error")
	if !report.Partial || report.Errors != 1 || reportCode(report, "schema_incompatible") ||
		reportCode(report, "session_index_unhealthy") || reportCode(report, "state_busy") {
		t.Fatalf("unexpected diagnoses for read contention: %+v", report)
	}
}

func r1ManagedDoctor(t *testing.T, built bool) (Service, *sql.DB) {
	t.Helper()
	ctx := t.Context()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if built {
		index, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := index.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatal(err)
	}
	core := r1OpenDatabase(t, filepath.Join(root, "agentdeck.sqlite3"), "rw")
	t.Cleanup(func() {
		if err := store.CloseReadPools(); err != nil {
			t.Error(err)
		}
	})
	home := t.TempDir()
	return Service{StateRoot: root, Home: home, Workdir: home, RegistrationProbe: deterministicRegistration}, core
}

func r1OpenedSlot(t *testing.T, root string, core *sql.DB) (string, []byte) {
	t.Helper()
	data := []byte("synthetic-slot-body-sentinel")
	digest := sha256.Sum256(data)
	sessionSeal := sha256.Sum256([]byte("selected session seal"))
	slotSeal := sha256.Sum256([]byte("selected slot seal"))
	identity := [16]byte{1, 2, 3}
	private := r1OpenDatabase(t, filepath.Join(root, "session-index-v1.sqlite3"), "rw")
	// Model a durable opened postimage. The reader under test still validates
	// a real installed root, core head, private stamp and physical slot.
	r1Exec(t, private, `UPDATE cjsi_index_state SET compacted_ticket=1,compacted_attempt=1,compacted_seal=?`, sessionSeal[:])
	r1Exec(t, core, `INSERT INTO cjsi_publication_attempts
		(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,
		  price_revision,protection_revision,coverage_revision,input_digest,diff_digest,session_seal,slot_seal,slot_digest,
		  slot_size,slot_format,slot_identity,built_at,valid_until,accept_until,state)
		SELECT 1,1,0,core_epoch,index_epoch,session_revision,0,0,0,0,protection_revision,coverage_revision,
		  randomblob(32),randomblob(32),?,?,?,?,1,?,'2000-01-01T00:00:00Z','2000-01-02T00:00:00Z',
		  '2000-01-02T00:00:00Z','opened' FROM cjsi_publication_head`, sessionSeal[:], slotSeal[:], digest[:], len(data), identity[:])
	r1Exec(t, core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_seal=?,
		slot_ticket=1,slot_attempt=1,slot_seal=?`, sessionSeal[:], slotSeal[:])
	path := filepath.Join(root, fmt.Sprintf(".cjsi-serving-slot-%x", identity))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path, data
}

// Damage only the root page of an unrelated index. Ordinary core
// queries remain readable, but SQLite's full page walk must report it.
func r1DamageUnusedIndexPage(t *testing.T, root string, core *sql.DB) {
	t.Helper()
	ctx := t.Context()
	r1Exec(t, core, `CREATE TABLE integrity_fixture(value INTEGER);
		INSERT INTO integrity_fixture VALUES(1);
		CREATE INDEX integrity_fixture_idx ON integrity_fixture(value)`)
	var indexPage, pageSize int64
	if err := core.QueryRowContext(ctx, "SELECT rootpage FROM sqlite_schema WHERE name='integrity_fixture_idx'").Scan(&indexPage); err != nil {
		t.Fatal(err)
	}
	if err := core.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if indexPage <= 1 || pageSize < 512 {
		t.Fatalf("invalid test page %d (page_size=%d)", indexPage, pageSize)
	}
	var busy, frames, checkpointed int
	if err := core.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed); err != nil || busy != 0 {
		t.Fatalf("checkpoint busy=%d err=%v", busy, err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseReadPools(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(root, "agentdeck.sqlite3"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteAt([]byte{0}, (indexPage-1)*pageSize)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("damage index page: %v %v", writeErr, closeErr)
	}
}

func assertR1FenceRejectsSQLite(t *testing.T, root string) {
	t.Helper()
	db := r1OpenDatabase(t, filepath.Join(root, "sessions.sqlite3"), "ro")
	var count int
	err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM sqlite_schema").Scan(&count)
	sqliteErr, ok := err.(*sqlite.Error)
	if !ok || sqliteErr.Code() != sqlite3.SQLITE_NOTADB {
		t.Fatalf("legacy SQLite reader got %v, want NOTADB", err)
	}
}

func r1OpenDatabase(t *testing.T, path, mode string) *sql.DB {
	t.Helper()
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func r1Exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func r1DoctorReport(t *testing.T, s Service, full bool) Report {
	t.Helper()
	report, err := s.Check(t.Context(), full)
	if err != nil {
		t.Fatal(err)
	}
	text, err := json.Marshal(report)
	if err != nil || bytes.Contains(text, []byte("sentinel")) {
		t.Fatalf("invalid report shape: %s, err=%v", text, err)
	}
	return report
}

func assertR1Check(t *testing.T, report Report, name, code, status string) {
	t.Helper()
	check := findCheck(report, name, code)
	if check == nil || check.Status != status {
		t.Fatalf("wanted %s/%s/%s in %+v", name, code, status, report.Checks)
	}
}

func reportCode(report Report, code string) bool {
	for _, check := range report.Checks {
		if check.Code == code {
			return true
		}
	}
	return false
}
