package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestCJSIProductionStateMigration(t *testing.T) {
	ctx := t.Context()
	t.Run("ordinary locked calls do not install", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		scan, err := AcquireScanLock(ctx, c.root, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer scan.Release()
		state, err := AcquireLock(ctx, c.root, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer state.Release()
		// The deadline also covers SQLite's 35→36 migration under -race.
		bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		core, err := OpenWithLockHeld(bounded, c.root)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, core.DB, "SELECT count(*) FROM sqlite_schema WHERE name='cjsi_layout_journal'", 0)
	})

	t.Run("cold open does not create a private index", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "state")
		if err := EnsureStateMigrated(ctx, root); err != nil {
			t.Fatal(err)
		}
		core, err := Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", CurrentSchemaVersion)
		assertCJSIMigrationNumber(t, core.DB, "SELECT phase FROM cjsi_layout_journal", int(cjsiLayoutReady))
		for _, open := range []func(context.Context, string) (*Store, error){OpenSessions, OpenSessionsReadOnly, OpenSessionDiagnosticsReadOnly} {
			s, err := open(ctx, root)
			if s != nil || !errors.Is(err, os.ErrNotExist) || !errors.Is(err, ErrCJSIIndexAbsent) {
				t.Fatalf("absent index opener = %v, %v", s, err)
			}
		}
		if _, err := os.Stat(filepath.Join(root, cjsiPrivateIndexName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("absent index was created: %v", err)
		}
		if err := EnsureStateMigrated(ctx, root); err != nil {
			t.Fatalf("ready fast path tried to drain its own open core: %v", err)
		}
		// A competing process may have seen the old version before the first
		// installer committed. Simulate its arrival at the locked recheck now
		// that the next writer already holds a live core connection.
		const wait = 100 * time.Millisecond
		if err := migrateCJSIManagedRootWhenNeeded(ctx, root, wait, cjsiStoreLegacyManager{wait}, cjsiStateInstalled, nil); err != nil {
			t.Fatalf("locked readiness recheck did not bypass a second drain: %v", err)
		}
	})

	t.Run("concurrent entry preserves one fence and selected index", func(t *testing.T) {
		// Under -race the first SQLite migration can take longer than the
		// production wait bound. Exercise the handoff rather than its timeout.
		previousWait := lockWait
		lockWait = 30 * time.Second
		defer func() { lockWait = previousWait }()
		c := newCJSIMigrationCapsule(t)
		c.seed36(t)
		seedCJSILegacySessions(t, c.root)
		start := make(chan struct{})
		type result struct {
			fence cjsiFileIdentity
			err   error
		}
		done := make(chan result, 2)
		keepWriter := make(chan struct{})
		var writers sync.WaitGroup
		defer func() {
			close(keepWriter)
			writers.Wait()
		}()
		for range 2 {
			writers.Add(1)
			go func() {
				defer writers.Done()
				<-start
				if err := EnsureStateMigrated(ctx, c.root); err != nil {
					done <- result{err: err}
					return
				}
				core, err := Open(ctx, c.root)
				if err != nil {
					done <- result{err: err}
					return
				}
				fence, err := validateCJSIFence(c.root)
				done <- result{fence, err}
				<-keepWriter
				_ = core.Close()
			}()
		}
		close(start)
		first, second := <-done, <-done
		if first.err != nil || second.err != nil || first.fence != second.fence || first.fence.inode == 0 {
			t.Fatalf("concurrent installations: fence=%+v error=%v, fence=%+v error=%v", first.fence, first.err, second.fence, second.err)
		}
		assertCJSIInstalledFence(t, c.root)
	})

	t.Run("killed installer is resumed by the next writable entry", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		seedCJSILegacySessions(t, c.root)
		if err := CloseLegacyReadPools(c.root); err != nil {
			t.Fatal(err)
		}
		child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCJSIStateMigrationCrashHelper$")
		child.Env = append(os.Environ(), "AGENTDECK_CJSI_STATE_CRASH="+c.root)
		output, err := child.CombinedOutput()
		var killed *exec.ExitError
		if !errors.As(err, &killed) || killed.ProcessState.String() != "signal: killed" {
			t.Fatalf("installer did not end at the crash seam: %v\n%s", err, output)
		}
		core := c.openCore(t, "ro")
		assertCJSIMigrationNumber(t, core, "SELECT phase FROM cjsi_layout_journal", int(cjsiFenceInstalling))
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		for _, open := range []func(context.Context, string) (*Store, error){Open, OpenReadOnly, OpenSessions, OpenSessionsReadOnly} {
			s, err := open(ctx, c.root)
			if s != nil || !errors.Is(err, errCJSILayoutPending) {
				t.Fatalf("pending layout opener = %v, %v", s, err)
			}
		}
		if err := EnsureStateMigrated(ctx, c.root); err != nil {
			t.Fatal(err)
		}
		assertCJSIInstalledFence(t, c.root)
	})

	t.Run("ready hook abort leaves a resumable installation", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed36(t)
		seedCJSILegacySessions(t, c.root)
		stop := errors.New("pause before ready commit")
		calls := 0
		err := migrateCJSIManagedRootWithReady(ctx, c.root, time.Second, cjsiStoreLegacyManager{wait: time.Second}, func(ctx context.Context) error {
			calls++
			core := c.openCore(t, "ro")
			defer core.Close()
			j, err := cjsiReadLayoutJournal(ctx, core)
			if err != nil || j.phase == cjsiLayoutReady {
				t.Fatalf("ready hook saw journal %+v: %v", j, err)
			}
			if _, err := validateCJSIFence(c.root); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(c.root, cjsiPrivateIndexName)); err != nil {
				t.Fatal(err)
			}
			assertCJSIMigrationNumber(t, core, "SELECT index_epoch FROM cjsi_publication_head", 0)
			return stop
		})
		if calls != 1 || !errors.Is(err, stop) {
			t.Fatalf("ready hook called %d times: %v", calls, err)
		}
		if err := EnsureStateMigrated(ctx, c.root); err != nil {
			t.Fatal(err)
		}
		assertCJSIInstalledFence(t, c.root)
	})

	t.Run("future schema exposes only an independent legacy reader", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed36(t)
		seedCJSILegacySessions(t, c.root)
		core := c.openCore(t, "rw")
		cjsiFixtureExec(t, core, "UPDATE schema_metadata SET version=?", CurrentSchemaVersion+1)
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		for _, open := range []func(context.Context, string) (*Store, error){Open, OpenReadOnly, OpenSessions} {
			s, err := open(ctx, c.root)
			if s != nil {
				_ = s.Close()
			}
			assertSchemaAhead(t, err, CurrentSchemaVersion+1)
		}
		assertSchemaAhead(t, EnsureStateMigrated(ctx, c.root), CurrentSchemaVersion+1)
		legacy, err := OpenSessionsReadOnly(ctx, c.root)
		if err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, legacy.DB, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
		if err := legacy.Close(); err != nil {
			t.Fatal(err)
		}
		core = c.openCore(t, "rw")
		cjsiFixtureExec(t, core, "CREATE TABLE cjsi_layout_journal (fixture INTEGER)")
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = OpenSessionsReadOnly(ctx, c.root)
		assertSchemaAhead(t, err, CurrentSchemaVersion+1)
	})

	t.Run("legacy read fallback requires no managed files", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		seedCJSILegacySessions(t, c.root)
		if err := os.WriteFile(filepath.Join(c.root, "agentdeck.sqlite3"), []byte("broken"), 0600); err != nil {
			t.Fatal(err)
		}
		legacy, err := OpenSessionsReadOnly(ctx, c.root)
		if err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, legacy.DB, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
		if err := legacy.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(c.root, legacyDrainIntent), 0700); err != nil {
			t.Fatal(err)
		}
		if s, err := OpenSessionsReadOnly(ctx, c.root); s != nil || err == nil {
			t.Fatalf("unreadable core with migration evidence reached the legacy index: %v, %v", s, err)
		}
	})
}

func TestCJSIStateMigrationCrashHelper(t *testing.T) {
	root := os.Getenv("AGENTDECK_CJSI_STATE_CRASH")
	if root == "" {
		return
	}
	ctx := t.Context()
	err := withCJSIManagedCore(ctx, root, time.Second, cjsiStoreLegacyManager{wait: time.Second}, func(root string, core *sql.DB) error {
		if err := migrateCJSICore(ctx, root, core); err != nil {
			return err
		}
		return killCJSIFixtureProcess(ctx)
	})
	t.Fatalf("crash helper returned: %v", err)
}

func killCJSIFixtureProcess(ctx context.Context) error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	if err := process.Kill(); err != nil {
		return err
	}
	// The syscall can return before the self-signal is delivered. Keep the
	// child alive so it cannot exit via a test assertion first.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("fixture remained alive after SIGKILL")
	}
}

func assertCJSIInstalledFence(t *testing.T, root string) {
	t.Helper()
	core, err := OpenReadOnly(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", CurrentSchemaVersion)
	assertCJSIMigrationNumber(t, core.DB, "SELECT phase FROM cjsi_layout_journal", int(cjsiLayoutReady))
	index := openCJSIFixtureDB(t, filepath.Join(root, cjsiPrivateIndexName), "ro")
	defer index.Close()
	assertCJSIMigrationNumber(t, index, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
	oldPath := filepath.Join(root, cjsiLegacyIndexName)
	for _, mode := range []string{"ro", "rw"} {
		old := openCJSIFixtureDB(t, oldPath, mode)
		query := "SELECT count(*) FROM sqlite_schema"
		if mode == "rw" {
			query = "PRAGMA journal_mode=WAL"
		}
		var result string
		err := old.QueryRowContext(t.Context(), query).Scan(&result)
		var sqliteErr *sqlite.Error
		if closeErr := old.Close(); !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_NOTADB || closeErr != nil {
			t.Fatalf("old %s opened the fence: %q, %v, close %v", mode, result, err, closeErr)
		}
	}
	for _, open := range []func(context.Context, string) (*Store, error){OpenSessions, OpenSessionsReadOnly} {
		s, err := open(t.Context(), root)
		if s != nil || !errors.Is(err, ErrCJSIIndexChanged) {
			t.Fatalf("managed data escaped through a raw opener: %v, %v", s, err)
		}
	}
}
