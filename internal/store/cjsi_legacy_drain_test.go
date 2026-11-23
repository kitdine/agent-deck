package store

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func legacyTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fixture?#")
	for _, open := range []func(context.Context, string) (*Store, error){Open, OpenSessions} {
		s, err := open(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func mustDrainLegacy(t *testing.T, root string) func() error {
	t.Helper()
	release, err := (cjsiStoreLegacyManager{time.Second}).drainLegacy(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	return release
}

func requireLegacyBusy(t *testing.T, root string) {
	t.Helper()
	release, err := (cjsiStoreLegacyManager{30 * time.Millisecond}).drainLegacy(t.Context(), root)
	if release != nil || !errors.Is(err, ErrStateBusy) {
		t.Fatalf("expected incomplete handoff: lease=%t err=%v", release != nil, err)
	}
}

func TestCJSILegacyDrainAdmission(t *testing.T) {
	ctx := t.Context()
	root, other := legacyTestRoot(t), legacyTestRoot(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	// Keep an idle cached reader in each root and check their actual DB handles.
	probe := func(root string) *sql.DB {
		t.Helper()
		s, err := OpenReadOnly(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		return s.DB
	}
	idle, untouched := probe(root), probe(other)
	release := mustDrainLegacy(t, alias)
	if idle.Ping() == nil {
		t.Fatal("idle connection survived handoff")
	}
	if probe(other) != untouched {
		t.Fatal("other root's cache was retired")
	}
	type rootEntry struct {
		name    string
		size    int64
		mode    os.FileMode
		modTime time.Time
	}
	snapshot := func() []rootEntry {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		state := make([]rootEntry, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			state = append(state, rootEntry{entry.Name(), info.Size(), info.Mode(), info.ModTime()})
		}
		return state
	}
	for _, entry := range []struct {
		name string
		open func(context.Context, string) (*Store, error)
	}{
		{"Open", Open},
		{"OpenExisting", OpenExisting},
		{"OpenWithLockHeld", OpenWithLockHeld},
		{"OpenSessions", OpenSessions},
		{"OpenReadOnly", OpenReadOnly},
		{"OpenSessionsReadOnly", OpenSessionsReadOnly},
		{"OpenSessionDiagnosticsReadOnly", OpenSessionDiagnosticsReadOnly},
	} {
		before := snapshot()
		s, err := entry.open(ctx, root)
		if s != nil {
			_ = s.Close()
		}
		if !errors.Is(err, ErrStateBusy) || s != nil {
			t.Fatalf("%s admission during drain: store=%t err=%v", entry.name, s != nil, err)
		}
		if after := snapshot(); !slices.Equal(before, after) {
			t.Fatalf("%s changed excluded root: before=%+v after=%+v", entry.name, before, after)
		}
	}
	if _, err := OpenServingGenerationReader(ctx, alias); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("serving: %v", err)
	}
	core := filepath.Join(root, "agentdeck.sqlite3")
	side := filepath.Join(other, "snapshot.sqlite3")
	if err := BackupSQLiteFile(ctx, core, side); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("backup source: %v", err)
	}
	if err := BackupSQLiteFile(ctx, filepath.Join(other, "agentdeck.sqlite3"), filepath.Join(root, "target.sqlite3")); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("backup destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "target.sqlite3")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created excluded target: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	s, err := OpenExisting(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.SchemaVersion(ctx); err != nil || n != CurrentSchemaVersion {
		t.Fatalf("ordinary open: version=%d err=%v", n, err)
	}
	if err := s.Backup(ctx, side); err != nil {
		t.Fatal(err)
	}
}

func TestCJSILegacyDrainRetainedConnections(t *testing.T) {
	for name, open := range map[string]func(context.Context, string) (*Store, error){
		"writer":              Open,
		"session_writer":      OpenSessions,
		"reader":              OpenReadOnly,
		"session_reader":      OpenSessionsReadOnly,
		"session_diagnostics": OpenSessionDiagnosticsReadOnly,
	} {
		t.Run(name, func(t *testing.T) {
			root := legacyTestRoot(t)
			s, err := open(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := s.DB.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			requireLegacyBusy(t, root)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			// The logical Store and the retired pool are now closed, but the
			// database/sql borrower still owns a usable physical connection.
			requireLegacyBusy(t, root)
			if err := raw.PingContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := raw.Close(); err != nil {
				t.Fatal(err)
			}
			mustDrainLegacy(t, root)
		})
	}
}

func TestCJSILegacyDrainInitializer(t *testing.T) {
	root := legacyTestRoot(t)
	path := filepath.Join(root, "sessions.sqlite3")
	entered, proceed := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, release, err := acquireLegacyRead(t.Context(), path, "paused", func(id legacyRootID) (*sql.DB, error) {
			close(entered)
			<-proceed
			return openLegacySQL(path, legacyURI(path, "mode=ro"), "", id)
		})
		if err == nil {
			err = release()
		}
		result <- err
	}()
	<-entered
	requireLegacyBusy(t, root)
	// The first refusal has already detached the initializing entry. A new
	// attempt still cannot succeed just because the cache no longer lists it.
	requireLegacyBusy(t, root)
	close(proceed)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	mustDrainLegacy(t, root)
}

func TestCJSILegacyDrainCancellation(t *testing.T) {
	root := legacyTestRoot(t)
	s, err := OpenSessions(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		release, err := (cjsiStoreLegacyManager{time.Minute}).drainLegacy(ctx, root)
		if release != nil {
			release()
			err = errors.New("unexpected lease")
		}
		done <- err
	}()
	// Observe actual admission refusal while the original writer still owns its lease.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r, err := OpenSessionsReadOnly(t.Context(), root)
		if errors.Is(err, ErrStateBusy) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		r.Close()
		if time.Now().After(deadline) {
			t.Fatal("never observed admission closed")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	r, err := OpenSessionsReadOnly(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	mustDrainLegacy(t, root)
}

func TestCJSILegacyDrainMigration(t *testing.T) {
	root := legacyTestRoot(t)
	s, err := OpenSessions(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateCJSIManagedRoot(t.Context(), root, time.Second, cjsiStoreLegacyManager{25 * time.Millisecond}); !errors.Is(err, ErrStateBusy) {
		t.Fatal(err)
	}
	r, err := OpenReadOnly(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := r.SchemaVersion(t.Context()); err != nil || v != CurrentSchemaVersion {
		t.Fatalf("version=%d err=%v", v, err)
	}
	r.Close()
	s.Close()
	if err := migrateCJSIManagedRoot(t.Context(), root, time.Second, cjsiStoreLegacyManager{time.Second}); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReadOnly(t.Context(), root); !errors.Is(err, ErrSchemaAhead) {
		t.Fatalf("ordinary core after installation: %v", err)
	}
	if _, err := OpenSessions(t.Context(), root); err == nil {
		t.Fatal("legacy fence opened")
	}
}

// All helpers communicate over inherited pipes and receive only a temporary
// root created by their parent. Exiting with a live Store exercises kernel
// teardown. An idle reader remains until its process cooperates or exits.
func TestCJSILegacyDrainProcess(t *testing.T) {
	if mode := os.Getenv("CJSI_DRAIN_CHILD"); mode != "" {
		root := os.Getenv("CJSI_DRAIN_ROOT")
		if mode == "manager" {
			_, err := (cjsiStoreLegacyManager{time.Second}).drainLegacy(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			open := OpenSessionsReadOnly
			if mode == "writer" {
				open = OpenSessions
			}
			s, err := open(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "idle" {
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			defer s.Close()
		}
		fmt.Println("holding")
		b := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, b); err != nil {
			t.Fatal(err)
		}
		os.Exit(73)
	}
	for _, mode := range []string{"writer", "reader", "idle", "manager"} {
		t.Run(mode, func(t *testing.T) {
			root := legacyTestRoot(t)
			if err := CloseReadPools(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCJSILegacyDrainProcess$")
			cmd.Env = append(os.Environ(), "CJSI_DRAIN_CHILD="+mode, "CJSI_DRAIN_ROOT="+root)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			in, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				in.Close()
				if cmd.ProcessState == nil {
					cancel()
					_ = cmd.Wait()
				}
			}()
			msg, err := bufio.NewReader(out).ReadString('\n')
			if err != nil || msg != "holding\n" {
				t.Fatalf("child: %q %v", msg, err)
			}
			if mode == "manager" {
				if s, err := OpenSessions(ctx, root); s != nil || !errors.Is(err, ErrStateBusy) {
					t.Fatalf("cross-process admission: %v", err)
				}
			} else {
				requireLegacyBusy(t, root)
			}
			if _, err := in.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); cmd.ProcessState.ExitCode() != 73 || err == nil {
				t.Fatalf("child exit: %v", err)
			}
			mustDrainLegacy(t, root)
		})
	}
}

func TestCJSILegacyDrainLiveHandoff(t *testing.T) {
	if root := os.Getenv("CJSI_LIVE_HANDOFF_ROOT"); root != "" {
		other := os.Getenv("CJSI_LIVE_HANDOFF_OTHER")
		local, err := OpenReadOnly(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		independent, err := OpenReadOnly(t.Context(), other)
		if err != nil {
			t.Fatal(err)
		}
		dbOther := independent.DB
		if err := independent.Close(); err != nil {
			t.Fatal(err)
		}
		step := func() {
			t.Helper()
			b := make([]byte, 1)
			if _, err := io.ReadFull(os.Stdin, b); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Println("holding")
		step()
		if s, err := OpenSessions(t.Context(), root); s != nil || !errors.Is(err, ErrStateBusy) {
			t.Fatalf("pending drain admitted writer: %v", err)
		}
		if err := CloseLegacyReadPools(root); err != nil {
			t.Fatal(err)
		}
		if err := local.DB.Ping(); err != nil {
			t.Fatal("retirement invalidated active reader", err)
		}
		if err := local.Close(); err != nil {
			t.Fatal(err)
		}
		again, err := OpenReadOnly(t.Context(), other)
		if err != nil {
			t.Fatal(err)
		}
		if again.DB != dbOther {
			t.Fatal("remote shutdown evicted unrelated root")
		}
		defer again.Close()
		fmt.Println("released")
		step()
		if s, err := OpenReadOnly(t.Context(), root); s != nil || !errors.Is(err, ErrStateBusy) {
			t.Fatalf("exclusive drain admitted reader: %v", err)
		}
		return
	}
	root, other := legacyTestRoot(t), legacyTestRoot(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCJSILegacyDrainLiveHandoff$")
	cmd.Env = append(os.Environ(), "CJSI_LIVE_HANDOFF_ROOT="+root, "CJSI_LIVE_HANDOFF_OTHER="+other)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		in.Close()
		if cmd.ProcessState == nil {
			cancel()
			_ = cmd.Wait()
		}
	}()
	reader := bufio.NewReader(out)
	expect := func(line string) {
		t.Helper()
		got, err := reader.ReadString('\n')
		if err != nil || got != line+"\n" {
			t.Fatalf("want %q: got %q %v", line, got, err)
		}
	}
	step := func() {
		t.Helper()
		if _, err := in.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
	}
	expect("holding")
	type handoff struct {
		release func() error
		err     error
	}
	done := make(chan handoff, 1)
	go func() {
		release, err := (cjsiStoreLegacyManager{4 * time.Second}).drainLegacy(ctx, root)
		done <- handoff{release, err}
	}()
	for {
		a, err := admitLegacy(ctx, root)
		if errors.Is(err, ErrStateBusy) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		a.close()
		select {
		case result := <-done:
			t.Fatalf("handoff before remote release: %v", result.err)
		case <-time.After(time.Millisecond):
		}
	}
	step()
	expect("released")
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	defer result.release()
	step()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

// This synthetic raw opener models the exact boundary left by callers that
// bypass Store. Only exercise the lock; never install a fence over this reader.
func TestCJSILegacyDrainNonparticipant(t *testing.T) {
	root := legacyTestRoot(t)
	path := filepath.Join(root, "sessions.sqlite3")
	db, err := sql.Open("sqlite", legacyURI(path, "mode=ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	mustDrainLegacy(t, root)
	var n int
	if err := conn.QueryRowContext(t.Context(), "SELECT count(*) FROM session_sources").Scan(&n); err != nil {
		t.Fatal(err)
	}
}
