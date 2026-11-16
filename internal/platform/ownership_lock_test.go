package platform

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnershipLocksAndTimeouts(t *testing.T) {
	for _, exclusive := range []bool{false, true} {
		t.Run(fmt.Sprint(exclusive), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			first, err := AcquireOwnershipLock(context.Background(), home, exclusive)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Release()
			if !exclusive {
				second, err := acquireOwnershipLock(context.Background(), home, false, 100*time.Millisecond)
				if err != nil {
					t.Fatalf("shared locks conflict: %v", err)
				}
				if err := second.Release(); err != nil {
					t.Fatal(err)
				}
			}
			start := time.Now()
			_, err = AcquireOwnershipLock(context.Background(), home, !exclusive)
			var busy *OwnershipLockError
			want := "owner_running"
			if exclusive {
				want = "ownership_busy"
			}
			if !errors.As(err, &busy) || busy.Reason != want {
				t.Fatalf("timeout error=%v want=%s", err, want)
			}
			elapsed := time.Since(start)
			if elapsed < ownershipLockWait || elapsed > ownershipLockWait+2*time.Second {
				t.Fatalf("timeout elapsed=%s", elapsed)
			}
			if err := first.Release(); err != nil {
				t.Fatal(err)
			}
			after, err := acquireOwnershipLock(context.Background(), home, !exclusive, 100*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			defer after.Release()
		})
	}
}

func TestOwnershipLockProcessExit(t *testing.T) {
	if home := os.Getenv("AGENTDECK_LOCK_CHILD_ACCOUNT"); home != "" {
		lock, err := AcquireOwnershipLock(context.Background(), home, true)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("locked")
		_, _ = io.Copy(io.Discard, os.Stdin)
		// No Release: process teardown must release the kernel lock. Keep the
		// descriptor reachable until exit so a finalizer cannot satisfy the test.
		_ = lock.WithExclusive(home, func() error { os.Exit(0); return nil })
		return
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := exec.Command(os.Args[0], "-test.run=^TestOwnershipLockProcessExit$")
	cmd.Env = append(os.Environ(), "AGENTDECK_LOCK_CHILD_ACCOUNT="+home)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("child ready=%q err=%v", line, err)
	}
	if lock, err := acquireOwnershipLock(context.Background(), home, false, 50*time.Millisecond); err == nil {
		_ = lock.Release()
		t.Fatal("child did not hold exclusive lock")
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireOwnershipLock(context.Background(), home, true, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("lock survived process exit: %v", err)
	}
	defer lock.Release()
}

func TestOwnershipLockRejectsSymlinkAndCancelledContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, "Library/Application Support/AgentDeck")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	backing := filepath.Join(home, "backing")
	if err := os.WriteFile(backing, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ownership.lock")
	if err := os.Symlink(backing, path); err != nil {
		t.Fatal(err)
	}
	if lock, err := AcquireOwnershipLock(context.Background(), home, true); err == nil {
		_ = lock.Release()
		t.Fatal("followed symlink lock")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AcquireOwnershipLock(ctx, home, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
