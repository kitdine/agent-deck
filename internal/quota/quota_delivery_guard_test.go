package quota

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestQuotaDeliveryReconciliation(t *testing.T) {
	ctx, s, lock, root := newIdentityStore(t)
	cycle := startIdentityCycle(t, ctx, s, lock, time.Now())
	path := filepath.Join(root, quotaEpochName)
	for _, bad := range [][]byte{nil, {}, []byte("invalid"), []byte("AGENTDECK-QDG-1\n\x00\x00\x00\x00\x00\x00\x00\x09"), bytes.Repeat([]byte{255}, 24)} {
		if bad != nil {
			if err := os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
		}
		g, err := s.ReconcileQuotaDelivery(ctx, cycle)
		if err != nil {
			t.Fatal(err)
		}
		checkEpochImage(t, path, 0)
		var st unix.Stat_t
		if unix.Stat(root, &st) != nil || g.StateRootID() != fmt.Sprintf("%d:%d", uint64(st.Dev), st.Ino) {
			t.Fatal("wrong physical identity")
		}
	}
	target := filepath.Join(t.TempDir(), "untouched")
	if err := os.WriteFile(target, []byte("private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileQuotaDelivery(ctx, cycle); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "private fixture" {
		t.Fatal("followed epoch symlink")
	}
	checkEpochImage(t, path, 0)

	t.Run("replace image with unsafe mode", func(t *testing.T) {
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReconcileQuotaDelivery(ctx, cycle); err != nil {
			t.Fatal(err)
		}
		checkEpochImage(t, path, 0)
	})

	t.Run("replace shared inode", func(t *testing.T) {
		link := filepath.Join(root, "linked-epoch")
		if err := os.Link(path, link); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReconcileQuotaDelivery(ctx, cycle); err != nil {
			t.Fatal(err)
		}
		checkEpochImage(t, path, 0)
		checkEpochImage(t, link, 0)
		current, err := os.Stat(path)
		previous, linkErr := os.Stat(link)
		if err != nil || linkErr != nil || os.SameFile(current, previous) {
			t.Fatalf("epoch still uses a shared inode: %v, %v", err, linkErr)
		}
	})

	t.Run("replace pipe", func(t *testing.T) {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := unix.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReconcileQuotaDelivery(ctx, cycle); err != nil {
			t.Fatal(err)
		}
		checkEpochImage(t, path, 0)
	})

	t.Run("leave an occupied directory unavailable", func(t *testing.T) {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if guard, err := s.ReconcileQuotaDelivery(ctx, cycle); guard != nil || err != ErrQuotaDeliveryUnavailable {
			t.Fatalf("epoch directory was accepted: guard=%v, error=%v", guard, err)
		}
		if info, err := os.Lstat(path); err != nil || !info.IsDir() {
			t.Fatalf("reconciliation changed the directory: %v", err)
		}
	})
}

func checkEpochImage(t *testing.T, path string, low byte) {
	t.Helper()
	want := append([]byte("AGENTDECK-QDG-1\n\x00\x00\x00\x00\x00\x00\x00"), low)
	got, err := os.ReadFile(path)
	info, statErr := os.Lstat(path)
	if err != nil || !bytes.Equal(got, want) || statErr != nil || info.Mode() != 0600 {
		t.Fatal("epoch is not the expected private 24-byte image")
	}
}

func TestQuotaDeliveryCommitRecovery(t *testing.T) {
	ctx, s, lock, root := newIdentityStore(t)
	c := startIdentityCycle(t, ctx, s, lock, time.Now())
	path := filepath.Join(root, quotaEpochName)
	g, err := s.ReconcileQuotaDelivery(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	apply := func() (ClaudeIdentityResult, error) {
		return g.ApplyClaudeIdentity(ctx, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{})
	}
	for _, failDir := range []bool{false, true} {
		g.syncFD = func(fd int) error {
			var st unix.Stat_t
			if err := unix.Fstat(fd, &st); err != nil {
				return err
			}
			if (st.Mode&unix.S_IFMT == unix.S_IFDIR) == failDir {
				return errors.New("private fault sentinel")
			}
			return unix.Fsync(fd)
		}
		res, err := apply()
		if res.Outcome != WriteUnresolved || err != ErrQuotaDeliveryUnavailable {
			t.Fatalf("outcome=%v err=%v", res.Outcome, err)
		}
		var attempts int
		if err := s.db.QueryRow(`SELECT count(*) FROM quota_identities`).Scan(&attempts); err != nil || attempts != 0 {
			t.Fatal("attempt survived mirror failure", err)
		}
		if failDir {
			checkEpochImage(t, path, 1)
		} else {
			checkEpochImage(t, path, 0)
		}
		g, err = s.ReconcileQuotaDelivery(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		checkEpochImage(t, path, 0)
	}
	// A deferred constraint fires at COMMIT, after the real sync callback.
	_, err = s.db.Exec(`CREATE TABLE qdg_parent(k INTEGER PRIMARY KEY);
	CREATE TABLE qdg_child(k INTEGER REFERENCES qdg_parent(k) DEFERRABLE INITIALLY DEFERRED);
	CREATE TRIGGER qdg_commit_fault AFTER INSERT ON quota_identities BEGIN INSERT INTO qdg_child VALUES(1); END;`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := apply()
	if err == nil || r.Outcome != WriteUnresolved {
		t.Fatal("expected commit failure", r.Outcome, err)
	}
	checkEpochImage(t, path, 1)
	g, err = s.ReconcileQuotaDelivery(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	checkEpochImage(t, path, 0)
	if _, err = s.db.Exec(`DROP TRIGGER qdg_commit_fault`); err != nil {
		t.Fatal(err)
	}
	r, err = apply()
	requireQuotaOutcome(t, r.Outcome, err, WriteCommitted)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReconcileQuotaDelivery(ctx, c); err != nil {
		t.Fatal(err)
	}
	checkEpochImage(t, path, 1)
}

func TestQuotaDeliveryBusyDatabase(t *testing.T) {
	ctx, s, lock, root := newIdentityStore(t)
	c := startIdentityCycle(t, ctx, s, lock, time.Now())
	g, err := s.ReconcileQuotaDelivery(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, "ROLLBACK")
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := g.ApplyClaudeIdentity(bounded, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{})
	if err != ErrQuotaDeliveryBusy || result.Outcome != WriteUnresolved {
		t.Fatalf("blocked write outcome=%v err=%v", result.Outcome, err)
	}
	checkEpochImage(t, filepath.Join(root, quotaEpochName), 0)
	if err := g.locked(ctx, func(heldDeliveryGuard) error { return nil }); err != nil {
		t.Fatal("failed write leaked delivery lock", err)
	}
}

func TestQuotaDeliveryDirectoryReplacement(t *testing.T) {
	ctx, s, l, root := newIdentityStore(t)
	g, err := s.ReconcileQuotaDelivery(ctx, startIdentityCycle(t, ctx, s, l, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	err = g.locked(context.Background(), func(lock heldDeliveryGuard) error {
		fd := lock.dir
		g.id = lock.rootID
		if err := os.Rename(root, root+".old"); err != nil {
			t.Fatal(err)
		}
		defer os.Rename(root+".old", root)
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(root)
		if _, err := g.checkDirectory(fd); err != ErrQuotaDeliveryUnavailable {
			t.Fatal("pinned descriptor accepted replacement path")
		}
		return g.locked(context.Background(), func(heldDeliveryGuard) error { t.Fatal("replacement accepted"); return nil })
	})
	if err != ErrQuotaDeliveryUnavailable {
		t.Fatal(err)
	}
}

// Child mode is enabled only by the parent test's private cwd and explicit
// argv. No installed helper, notification service or user state is involved.
func TestQuotaDeliveryFixture(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "qdg-fixture" {
		t.Skip("child fixture")
	}
	mode := os.Args[len(os.Args)-1]
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile("fixture-marker"); err != nil || string(b) != "synthetic quota delivery\n" {
		t.Fatal("missing fixture")
	}
	if mode == "exec-stop" {
		if err := unix.Kill(os.Getpid(), unix.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		return
	}
	g := QuotaDeliveryGuard{root: root}
	err = g.locked(context.Background(), func(lock heldDeliveryGuard) error {
		fd := lock.dir
		for _, descriptor := range []int{lock.dir, lock.file} {
			flags, err := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0)
			if err != nil || flags&unix.FD_CLOEXEC == 0 {
				return errors.New("missing CLOEXEC")
			}
		}
		if mode == "exec" {
			args := append([]string(nil), os.Args...)
			args[len(args)-1] = "exec-stop"
			return unix.Exec(args[0], args, os.Environ())
		}
		gen, ok := readQuotaEpoch(fd)
		if !ok {
			return errors.New("invalid fixture epoch")
		}
		if gen != 0 {
			return nil
		}
		if err := unix.Kill(os.Getpid(), unix.SIGSTOP); err != nil {
			return err
		}
		var ts unix.Timespec
		if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
			return err
		}
		return os.WriteFile("submit.fifo", []byte(strconv.FormatInt(ts.Nano(), 10)+"\n"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuotaDeliveryStoppedHolder(t *testing.T) {
	for _, mode := range []string{"continue", "kill", "exec"} {
		t.Run(mode, func(t *testing.T) {
			ctx, s, lock, root := newIdentityStore(t)
			c := startIdentityCycle(t, ctx, s, lock, time.Now())
			g, err := s.ReconcileQuotaDelivery(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "fixture-marker"), []byte("synthetic quota delivery\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fifo := filepath.Join(root, "submit.fifo")
			if err := unix.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			fd, err := unix.Open(fifo, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(fd)
			spawn := func(mode string) *exec.Cmd {
				deadline, cancel := context.WithTimeout(ctx, 15*time.Second)
				cmd := exec.CommandContext(deadline, os.Args[0], "-test.run=^TestQuotaDeliveryFixture$", "--", "qdg-fixture", mode)
				cmd.Dir = root
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					cancel()
					t.Fatal(err)
				}
				t.Cleanup(func() {
					cancel()
					if cmd.ProcessState == nil {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
					}
				})
				return cmd
			}
			cmd := spawn(mode)
			var status unix.WaitStatus
			// The BSD x/sys Stopped helper explicitly excludes SIGSTOP.
			if _, err := unix.Wait4(cmd.Process.Pid, &status, unix.WUNTRACED, nil); err != nil || int(status) != int(unix.SIGSTOP)<<8|0x7f {
				t.Fatal("child did not stop under guard", status, err)
			}
			apply := func(ctx context.Context) (ClaudeIdentityResult, error) {
				return g.ApplyClaudeIdentity(ctx, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{})
			}
			if mode != "exec" {
				for i := 0; i < 2; i++ {
					if i == 1 {
						path := filepath.Join(root, quotaEpochName)
						image, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						if err = os.Remove(path); err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(path, image, 0600); err != nil {
							t.Fatal(err)
						}
					}
					r, err := apply(ctx)
					if err != ErrQuotaDeliveryBusy || r.Outcome != WriteUnresolved {
						t.Fatalf("contended outcome=%v err=%v", r.Outcome, err)
					}
				}
				state, err := s.CommittedClaudeContext(ctx, c)
				if err != nil || state.Generation != 0 {
					t.Fatal("contended transaction changed generation", err)
				}
				var attempts int
				if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM quota_identities`).Scan(&attempts); err != nil || attempts != 0 {
					t.Fatalf("contended transaction recorded %d identity attempts: %v", attempts, err)
				}
				checkEpochImage(t, filepath.Join(root, quotaEpochName), 0)
				if mode == "kill" {
					if err := cmd.Process.Kill(); err != nil {
						t.Fatal(err)
					}
				} else if err := cmd.Process.Signal(unix.SIGCONT); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Wait(); (err != nil) != (mode == "kill") {
					t.Fatal(err)
				}
			}
			r, err := apply(ctx)
			requireQuotaOutcome(t, r.Outcome, err, WriteCommitted)
			var committed unix.Timespec
			if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &committed); err != nil {
				t.Fatal(err)
			}
			if mode == "exec" {
				if err := cmd.Process.Signal(unix.SIGCONT); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Wait(); err != nil {
					t.Fatal(err)
				}
			}
			var buf [80]byte
			if mode == "continue" {
				n, err := unix.Read(fd, buf[:])
				if err != nil {
					t.Fatal(err)
				}
				ts, err := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
				if err != nil || ts >= committed.Nano() {
					t.Fatal("submission did not precede commit")
				}
			}
			if err := spawn("continue").Wait(); err != nil {
				t.Fatal(err)
			}
			if _, err := unix.Read(fd, buf[:]); err != unix.EAGAIN {
				t.Fatal("unexpected old-generation submission", err)
			}
		})
	}
}

// An exec-confirmed child stays alive after the original owner returns from
// locked (close only). The control clears CLOEXEC and must block a fresh
// process, so process-runner descriptor sanitization cannot yield a false pass.
func TestQuotaDeliveryCloseOnExecAcrossProcesses(t *testing.T) {
	const marker = "quota-exec-probe"
	if os.Args[len(os.Args)-1] == marker {
		root, e := os.Getwd()
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile("fixture-marker")
		if e != nil || string(b) != "synthetic quota delivery\n" {
			t.Fatal("missing fixture marker")
		}
		g := QuotaDeliveryGuard{root: canonicalQuotaFixtureRoot(t, root)}
		e = g.acquire(t.Context(), time.Now().Add(150*time.Millisecond), false, func(heldDeliveryGuard) error { return nil })
		if errors.Is(e, ErrQuotaDeliveryBusy) {
			os.Exit(71)
		}
		if e != nil {
			t.Fatal(e)
		}
		return
	}
	for _, inherit := range []bool{false, true} {
		t.Run(map[bool]string{false: "cloexec", true: "inheritance-control"}[inherit], func(t *testing.T) {
			ctx, s, l, root := newIdentityStore(t)
			g, e := s.ReconcileQuotaDelivery(ctx, startIdentityCycle(t, ctx, s, l, time.Now()))
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(root, "fixture-marker"), []byte("synthetic quota delivery\n"), 0600); e != nil {
				t.Fatal(e)
			}
			deadline, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			child := exec.CommandContext(deadline, os.Args[0], "-test.run=^TestQuotaDeliveryFixture$", "--", "qdg-fixture", "exec-stop")
			child.Dir = root
			defer func() {
				if child.Process != nil {
					child.Process.Kill()
					child.Wait()
				}
			}()
			e = g.locked(ctx, func(h heldDeliveryGuard) error {
				flags, e := unix.FcntlInt(uintptr(h.file), unix.F_GETFD, 0)
				if e != nil {
					return e
				}
				if flags&unix.FD_CLOEXEC == 0 {
					t.Fatal("missing close-on-exec")
				}
				if inherit {
					if _, e = unix.FcntlInt(uintptr(h.file), unix.F_SETFD, flags&^unix.FD_CLOEXEC); e != nil {
						return e
					}
				}
				if e = child.Start(); e != nil {
					return e
				}
				var st unix.WaitStatus
				if _, e = unix.Wait4(child.Process.Pid, &st, unix.WUNTRACED, nil); e != nil {
					return e
				}
				if int(st) != int(unix.SIGSTOP)<<8|0x7f {
					t.Fatalf("exec child did not reach its own stop: %v", st)
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			contender := exec.CommandContext(deadline, os.Args[0], "-test.run=^TestQuotaDeliveryCloseOnExecAcrossProcesses$", "--", marker)
			contender.Dir = root
			b, e := contender.CombinedOutput()
			var exit *exec.ExitError
			if (!inherit && e != nil) || (inherit && (!errors.As(e, &exit) || exit.ExitCode() != 71)) {
				t.Fatalf("contender: %v %s", e, b)
			}
			if e = unix.Kill(child.Process.Pid, 0); e != nil {
				t.Fatal("exec child exited before contention was tested", e)
			}
		})
	}
}
