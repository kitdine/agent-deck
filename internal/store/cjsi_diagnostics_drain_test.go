package store_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSISessionCheckHealthParticipatesInDrain(t *testing.T) {
	ctx := t.Context()

	t.Run("rejected during managed maintenance without touching root", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := store.CloseLegacyReadPools(root); err != nil {
			t.Fatal(err)
		}

		// Sanity control before maintenance: CheckHealth succeeds.
		health, err := session.CheckHealth(ctx, root, false)
		if err != nil || !health.Present || !health.FTSAvailable || health.Integrity != "ok" {
			t.Fatalf("control CheckHealth = %+v, %v", health, err)
		}

		type dirEntry struct {
			name string
			size int64
			mode os.FileMode
		}
		snapshot := func() []dirEntry {
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			out := make([]dirEntry, 0, len(entries))
			for _, e := range entries {
				info, err := e.Info()
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, dirEntry{e.Name(), info.Size(), info.Mode()})
			}
			return out
		}

		// Inside real managed core maintenance (scan lock -> state lock -> drain EX),
		// both quick and full CheckHealth must fail with ErrStateBusy before opening SQLite.
		err = store.WithCJSIManagedCoreForTest(ctx, root, time.Second, func(physical string, _ *sql.DB) error {
			for _, full := range []bool{false, true} {
				before := snapshot()
				got, checkErr := session.CheckHealth(ctx, physical, full)
				if !errors.Is(checkErr, store.ErrStateBusy) || got != (session.Health{}) {
					t.Fatalf("CheckHealth(full=%v) during maintenance = %+v, %v; want ErrStateBusy", full, got, checkErr)
				}
				if after := snapshot(); !slices.Equal(before, after) {
					t.Fatalf("refused CheckHealth(full=%v) mutated drained root:\nbefore=%+v\nafter =%+v", full, before, after)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// After maintenance finishes, CheckHealth works again.
		health, err = session.CheckHealth(ctx, root, true)
		if err != nil || !health.Present || !health.FTSAvailable || health.Integrity != "ok" {
			t.Fatalf("post-maintenance CheckHealth = %+v, %v", health, err)
		}
	})

	t.Run("drain waits until existing diagnostic physical connection closes", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := store.CloseLegacyReadPools(root); err != nil {
			t.Fatal(err)
		}

		diag, err := store.OpenSessionDiagnosticsReadOnly(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := diag.DB.Conn(ctx)
		if err != nil {
			_ = diag.Close()
			t.Fatal(err)
		}
		defer raw.Close()

		// While the Store is open, drain must fail with ErrStateBusy.
		if release, err := store.DrainLegacyForTest(ctx, root, 30*time.Millisecond); release != nil || !errors.Is(err, store.ErrStateBusy) {
			if release != nil {
				_ = release()
			}
			t.Fatalf("drain succeeded with open diagnostic store: release=%v err=%v", release != nil, err)
		}

		// Even after diag.Close(), the borrowed physical SQLite connection still pins the root.
		if err := diag.Close(); err != nil {
			t.Fatal(err)
		}
		if release, err := store.DrainLegacyForTest(ctx, root, 30*time.Millisecond); release != nil || !errors.Is(err, store.ErrStateBusy) {
			if release != nil {
				_ = release()
			}
			t.Fatalf("drain succeeded while diagnostic physical conn was still open: release=%v err=%v", release != nil, err)
		}

		// Closing the physical connection allows drain to complete.
		if err := raw.Close(); err != nil {
			t.Fatal(err)
		}
		release, err := store.DrainLegacyForTest(ctx, root, time.Second)
		if err != nil {
			t.Fatalf("drain failed after closing diagnostic physical conn: %v", err)
		}
		if err := release(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("preserves missing-index and partial-schema diagnostic semantics", func(t *testing.T) {
		missingRoot := filepath.Join(t.TempDir(), "absent")
		h, err := session.CheckHealth(ctx, missingRoot, false)
		if err != nil || h.Present || h.FTSAvailable || h.Integrity != "not_requested" {
			t.Fatalf("missing root CheckHealth = %+v, %v", h, err)
		}
		if _, err := os.Stat(missingRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing root was created: %v", err)
		}

		partialRoot := t.TempDir()
		raw, err := sql.Open("sqlite", filepath.Join(partialRoot, "sessions.sqlite3"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.ExecContext(ctx, "CREATE TABLE session_sources (source_path TEXT PRIMARY KEY)"); err != nil {
			_ = raw.Close()
			t.Fatal(err)
		}
		if err := raw.Close(); err != nil {
			t.Fatal(err)
		}
		h, err = session.CheckHealth(ctx, partialRoot, false)
		if err != nil || !h.Present || h.FTSAvailable || h.Integrity != "not_requested" {
			t.Fatalf("partial schema CheckHealth = %+v, %v", h, err)
		}
		// Partial diagnostic check must not leave any connection pinning the root.
		release, err := store.DrainLegacyForTest(ctx, partialRoot, time.Second)
		if err != nil {
			t.Fatalf("drain blocked after CheckHealth returned: %v", err)
		}
		if err := release(); err != nil {
			t.Fatal(err)
		}
	})
}
