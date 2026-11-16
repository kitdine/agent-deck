//go:build unix

package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// sqliteDeadManSwitch is the byte of a -shm file every WAL connection holds a
// shared POSIX lock on (SQLite's UNIX_SHM_DMS). A process that finds it
// unlocked treats itself as the first connection and reinitializes the file.
const sqliteDeadManSwitch = 128

const lockProbeEnv = "AGENTDECK_TEST_LOCK_PROBE"

// TestHelperProbeSQLiteLock runs in a child process: POSIX locks never
// conflict within one process, so only another process can see them.
func TestHelperProbeSQLiteLock(t *testing.T) {
	path := os.Getenv(lockProbeEnv)
	if path == "" {
		t.Skip("helper process only")
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	probe := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: sqliteDeadManSwitch, Len: 1}
	if err := syscall.FcntlFlock(file.Fd(), syscall.F_GETLK, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Type == syscall.F_UNLCK {
		os.Stdout.WriteString("LOCK=free\n")
	} else {
		os.Stdout.WriteString("LOCK=held\n")
	}
}

func sharedMemoryLock(t *testing.T, shm string) string {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestHelperProbeSQLiteLock$", "-test.v")
	command.Env = append(os.Environ(), lockProbeEnv+"="+shm)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	switch {
	case strings.Contains(string(out), "LOCK=held"):
		return "held"
	case strings.Contains(string(out), "LOCK=free"):
		return "free"
	}
	t.Fatalf("probe printed no result:\n%s", out)
	return ""
}

// Opening a database a second time in one process must not release the locks
// the first connection holds. Losing them lets another process reinitialize
// the WAL shared memory under the first connection, which then dies with
// SIGBUS on its next WAL access; the scan worker crashed this way while a
// desktop helper opened the same state.
func TestReopeningADatabaseInOneProcessKeepsTheFirstConnectionsLocks(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	for name, reopen := range map[string]func() (*Store, error){
		"core":                  func() (*Store, error) { return Open(ctx, state) },
		"core at existing root": func() (*Store, error) { return OpenExisting(ctx, state) },
		"sessions":              func() (*Store, error) { return OpenSessions(ctx, state) },
	} {
		t.Run(name, func(t *testing.T) {
			first, err := reopen()
			if name == "core at existing root" && err != nil {
				// The root must exist first.
				created, createErr := Open(ctx, state)
				if createErr != nil {
					t.Fatal(createErr)
				}
				created.Close()
				first, err = reopen()
			}
			if err != nil {
				t.Fatal(err)
			}
			defer first.Close()
			conn, err := first.DB.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			var mode string
			if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
				t.Fatalf("journal_mode = %q %v, want wal", mode, err)
			}
			var tables int
			if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master").Scan(&tables); err != nil {
				t.Fatal(err)
			}
			database := "agentdeck.sqlite3"
			if name == "sessions" {
				database = "sessions.sqlite3"
			}
			shm := filepath.Join(state, database+"-shm")
			if got := sharedMemoryLock(t, shm); got != "held" {
				t.Fatalf("precondition: the open connection's shared-memory lock is %s", got)
			}
			second, err := reopen()
			if err != nil {
				t.Fatal(err)
			}
			if err := second.Close(); err != nil {
				t.Fatal(err)
			}
			if got := sharedMemoryLock(t, shm); got != "held" {
				t.Fatalf("reopening the database released the first connection's shared-memory lock (%s)", got)
			}
		})
	}
}
