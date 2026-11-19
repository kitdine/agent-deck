package quota

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	corestore "github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

type deliveryFixture struct {
	store *Store
	root  string
	id    string
}

// Open a real core through an alias, retaining its managed drain pin and
// refresh lock. Reconciliation independently resolves the core's path.
func newDeliveryFixture(t *testing.T) (context.Context, deliveryFixture, *corestore.Lock) {
	t.Helper()
	ctx := t.Context()
	root := canonicalQuotaFixtureRoot(t, t.TempDir())
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	core, err := corestore.Open(ctx, alias)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	lock, err := corestore.AcquireQuotaRefreshLock(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lock.Release() })
	var st unix.Stat_t
	if err := unix.Stat(root, &st); err != nil {
		t.Fatal(err)
	}
	return ctx, deliveryFixture{NewWritableStore(core), root, fmt.Sprintf("%d:%d", uint64(st.Dev), st.Ino)}, lock
}

func TestQuotaDeliveryIndependentDomains(t *testing.T) {
	ctx, r, lock := newDeliveryFixture(t) // enters through a directory alias
	c := startIdentityCycle(t, ctx, r.store, lock, time.Now())
	dir, err := unix.Open(r.root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(dir)
	if err := unix.Flock(dir, unix.LOCK_SH|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	g, err := r.store.ReconcileQuotaDelivery(ctx, c)
	if err != nil {
		t.Fatal("root shared lock interfered with reconciliation", err)
	}
	if err := g.locked(ctx, func(heldDeliveryGuard) error {
		other, err := unix.Openat(dir, deliveryName, deliveryOpenFlags, 0)
		if err != nil {
			return err
		}
		defer unix.Close(other)
		if err := unix.Flock(other, unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
			t.Fatal("second guard acquired", err)
		}
		if err := unix.Flock(dir, unix.LOCK_UN); err != nil {
			return err
		}

		// A managed core can hold its own shared drain pin. Simulate drain
		// after retiring the core while the independent delivery file is held.
		if err := r.store.core.Close(); err != nil {
			return err
		}
		if err := corestore.CloseReadPools(); err != nil {
			return err
		}
		return unix.Flock(dir, unix.LOCK_EX|unix.LOCK_NB)
	}); err != nil {
		t.Fatal("delivery guard interfered with directory exclusivity after drain", err)
	}

	// Like the App, this guard checks the binding without a SQLite cycle.
	appGuard := QuotaDeliveryGuard{root: g.root, id: g.id, inode: g.inode}
	if err := appGuard.locked(ctx, func(heldDeliveryGuard) error { return nil }); err != nil {
		t.Fatal("root exclusive lock interfered with delivery", err)
	}
}

func TestQuotaDeliveryInterruptedPublication(t *testing.T) {
	// Actual sync boundaries in the first reconciliation. An injected error
	// immediately after fsync leaves the same namespace as an interruption.
	for idx, label := range []string{"pending-inode", "pending-directory", "binding-directory", "linked-directory", "unlinked-directory", "epoch-inode", "epoch-directory"} {
		t.Run(label, func(t *testing.T) {
			ctx, r, l := newDeliveryFixture(t)
			c := startIdentityCycle(t, ctx, r.store, l, time.Now())
			interrupted := QuotaDeliveryGuard{cycle: c, root: r.root, id: r.id}
			syncs := 0
			interrupted.syncFD = func(fd int) error {
				if e := unix.Fsync(fd); e != nil {
					return e
				}
				syncs++
				if syncs == idx+1 {
					return unix.EIO
				}
				return nil
			}
			e := interrupted.acquire(ctx, time.Now().Add(time.Second), true, func(h heldDeliveryGuard) error { return interrupted.publish(h, 0) })
			if !errors.Is(e, ErrQuotaDeliveryUnavailable) || syncs != idx+1 {
				t.Fatalf("syncs=%d err=%v", syncs, e)
			}
			dir, e := unix.Open(r.root, unix.O_RDONLY|unix.O_DIRECTORY, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer unix.Close(dir)
			p, hasP, e := deliveryEntry(dir, deliveryPending)
			if e != nil {
				t.Fatal(e)
			}
			o, hasO, e := deliveryEntry(dir, deliveryName)
			if e != nil {
				t.Fatal(e)
			}
			b, hasB, e := deliveryBinding(dir)
			if e != nil {
				t.Fatal(e)
			}
			if hasP != (idx <= 3) || hasO != (idx >= 3) || hasB != (idx >= 2) {
				t.Fatalf("wrong crash image pending=%v official=%v binding=%v", hasP, hasO, hasB)
			}
			original := deliveryID(&o)
			if hasP {
				original = deliveryID(&p)
			}
			if hasB && original != b {
				t.Fatal("binding changed before recovery")
			}
			if hasP && hasO && (deliveryID(&o) != original || p.Nlink != 2 || o.Nlink != 2) {
				t.Fatal("not two names for one inode")
			}
			g, e := r.store.ReconcileQuotaDelivery(ctx, c)
			if e != nil {
				t.Fatal(e)
			}
			final, ok, e := deliveryEntry(dir, deliveryName)
			if e != nil || !ok || !privateDeliveryInode(&final, 1) || deliveryID(&final) != original || g.inode != original {
				t.Fatal("rebound on resume", e)
			}
			if e = requireDeliveryAbsent(dir, deliveryPending); e != nil {
				t.Fatal(e)
			}
			checkEpochImage(t, filepath.Join(r.root, quotaEpochName), 0)
		})
	}
}

func TestQuotaDeliverySetupDeadline(t *testing.T) {
	for _, cancelIt := range []bool{false, true} {
		ctx, r, l := newDeliveryFixture(t)
		c := startIdentityCycle(t, ctx, r.store, l, time.Now())
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		until := time.Now().Add(70 * time.Millisecond)
		g := QuotaDeliveryGuard{cycle: c, root: r.root, id: r.id}
		calls := 0
		g.syncFD = func(fd int) error {
			calls++
			if cancelIt {
				cancel()
			} else {
				time.Sleep(time.Until(until) + time.Millisecond)
			}
			return unix.Fsync(fd)
		}
		e := g.acquire(ctx, until, true, func(heldDeliveryGuard) error { t.Fatal("late lock authorized"); return nil })
		want := ErrQuotaDeliveryBusy
		if cancelIt {
			want = context.Canceled
		}
		if !errors.Is(e, want) || calls != 1 {
			t.Fatal(e, calls)
		}
		dir, e := unix.Open(r.root, unix.O_RDONLY|unix.O_DIRECTORY, 0)
		if e != nil {
			t.Fatal(e)
		}
		_, bound, e := deliveryBinding(dir)
		if e != nil || bound {
			t.Fatal("late binding", e)
		}
		if e = requireDeliveryAbsent(dir, deliveryName); e != nil {
			t.Fatal(e)
		}
		unix.Close(dir)
	}
}

func TestQuotaDeliveryRefusesRebind(t *testing.T) {
	for _, kind := range []string{"missing", "replaced", "wrong-mode", "symlink", "fifo", "directory", "extra-link", "binding-missing", "binding-malformed", "binding-different", "pending-different", "bound-stage-missing", "bound-stage-replaced", "two-names-different", "prior-epoch", "unbound-official", "unsafe-unbound-stage"} {
		t.Run(kind, func(t *testing.T) {
			ctx, r, l := newDeliveryFixture(t)
			c := startIdentityCycle(t, ctx, r.store, l, time.Now())
			g, e := r.store.ReconcileQuotaDelivery(ctx, c)
			if e != nil {
				t.Fatal(e)
			}
			dir, e := unix.Open(r.root, unix.O_RDONLY|unix.O_DIRECTORY, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer unix.Close(dir)
			old, e := unix.Openat(dir, deliveryName, deliveryOpenFlags, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer unix.Close(old)
			if e = unix.Flock(old, unix.LOCK_EX|unix.LOCK_NB); e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(r.root, deliveryName)
			stage := filepath.Join(r.root, deliveryPending)
			epoch := filepath.Join(r.root, quotaEpochName)
			must := func(e error) {
				t.Helper()
				if e != nil {
					t.Fatal(e)
				}
			}
			switch kind {
			case "missing":
				must(os.Remove(file))
			case "replaced":
				must(os.Remove(file))
				must(os.WriteFile(file, nil, 0600))
			case "wrong-mode":
				must(os.Chmod(file, 0644))
			case "symlink":
				must(os.Rename(file, file+".old"))
				must(os.Symlink(file+".old", file))
			case "fifo":
				must(os.Remove(file))
				must(unix.Mkfifo(file, 0600))
			case "directory":
				must(os.Remove(file))
				must(os.Mkdir(file, 0700))
			case "extra-link":
				must(os.Link(file, file+".extra"))
			case "binding-missing":
				must(unix.Fremovexattr(dir, deliveryAttr))
			case "binding-malformed":
				must(unix.Fsetxattr(dir, deliveryAttr, []byte{1}, unix.XATTR_REPLACE))
			case "binding-different":
				id := g.inode
				binary.BigEndian.PutUint64(id[8:], binary.BigEndian.Uint64(id[8:])+1)
				must(unix.Fsetxattr(dir, deliveryAttr, id[:], unix.XATTR_REPLACE))
			case "pending-different":
				must(os.WriteFile(stage, nil, 0600))
			case "bound-stage-missing":
				must(os.Remove(file))
				must(os.Remove(epoch))
			case "bound-stage-replaced":
				must(os.Remove(file))
				must(os.Remove(epoch))
				must(os.WriteFile(stage, nil, 0600))
			case "two-names-different":
				must(os.Remove(epoch))
				must(os.WriteFile(stage, nil, 0600))
			case "prior-epoch":
				must(os.Remove(file))
				must(unix.Fremovexattr(dir, deliveryAttr))
			case "unbound-official":
				must(os.Remove(epoch))
				must(unix.Fremovexattr(dir, deliveryAttr))
			case "unsafe-unbound-stage":
				must(os.Remove(file))
				must(os.Remove(epoch))
				must(unix.Fremovexattr(dir, deliveryAttr))
				must(os.Symlink("absent", stage))
			}
			attr := func() string {
				var b [128]byte
				n, e := unix.Fgetxattr(dir, deliveryAttr, b[:])
				if e != nil {
					return e.Error()
				}
				return string(b[:n])
			}
			before := attr()
			if newGuard, e := r.store.ReconcileQuotaDelivery(ctx, c); newGuard != nil || !errors.Is(e, ErrQuotaDeliveryUnavailable) {
				t.Fatalf("refreshed invalid guard: %v", e)
			}
			if after := attr(); after != before {
				t.Fatal("silently changed binding")
			}
			if kind == "missing" || kind == "replaced" {
				out, e := g.ApplyClaudeIdentity(ctx, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{})
				if !errors.Is(e, ErrQuotaDeliveryUnavailable) || out.Outcome != WriteUnresolved {
					t.Fatal(out, e)
				}
				state, e := r.store.CommittedClaudeContext(ctx, c)
				if e != nil || state.Generation != 0 {
					t.Fatal(state, e)
				}
			}
		})
	}
}

func TestQuotaDeliveryOwnerValidation(t *testing.T) {
	var st unix.Stat_t
	st.Mode = unix.S_IFREG | 0600
	st.Nlink = 1
	st.Uid = uint32(os.Geteuid())
	if !privateDeliveryInode(&st, 1) {
		t.Fatal("valid inode refused")
	}
	st.Uid ^= 1
	if privateDeliveryInode(&st, 1) {
		t.Fatal("foreign owner accepted")
	}
}

// The notifier XCTest launches this test binary only in its private fixture
// directory. No CLI, service, account, or notification center participates.
func TestQuotaDeliverySwiftPeer(t *testing.T) {
	if os.Args[len(os.Args)-1] != "sai-independent-guard-peer" {
		t.Skip("Swift subprocess fixture")
	}
	root, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	marker, e := os.ReadFile(filepath.Join(root, "test-fixture"))
	if e != nil || string(marker) != "synthetic delivery guard\n" {
		t.Fatal("missing fixture")
	}
	ctx := t.Context()
	core, e := corestore.Open(ctx, root)
	if e != nil {
		t.Fatal(e)
	}
	defer core.Close()
	l, e := corestore.AcquireQuotaRefreshLock(ctx, root, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	s := NewWritableStore(core)
	c := startIdentityCycle(t, ctx, s, l, time.Now())
	g, e := s.ReconcileQuotaDelivery(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	in := bufio.NewScanner(os.Stdin)
	e = g.locked(ctx, func(_ heldDeliveryGuard) error {
		fmt.Println("held " + g.StateRootID())
		if !in.Scan() || in.Text() != "release" {
			return errors.New("missing release")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("released")
	for _, wantBusy := range []bool{true, false} {
		if !in.Scan() || in.Text() != "apply" {
			t.Fatal("missing apply")
		}
		result, e := g.ApplyClaudeIdentity(ctx, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{})
		if wantBusy {
			if !errors.Is(e, ErrQuotaDeliveryBusy) || result.Outcome != WriteUnresolved {
				t.Fatal(result, e)
			}
			fmt.Println("busy")
		} else {
			if e != nil || result.Outcome != WriteCommitted || result.Generation != 1 {
				t.Fatal(result, e)
			}
			fmt.Println("committed")
		}
	}
}

func TestQuotaDeliverySetupAndFlockShareDeadline(t *testing.T) {
	ctx, r, l := newDeliveryFixture(t)
	c := startIdentityCycle(t, ctx, r.store, l, time.Now())
	fd, e := unix.Open(filepath.Join(r.root, deliveryPending), unix.O_RDONLY|unix.O_CREAT|unix.O_EXCL, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer unix.Close(fd)
	if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	g := QuotaDeliveryGuard{cycle: c, root: r.root, id: r.id}
	first := true
	g.syncFD = func(fd int) error {
		if first {
			first = false
			time.Sleep(250 * time.Millisecond)
		}
		return unix.Fsync(fd)
	}
	began := time.Now()
	e = g.acquire(ctx, began.Add(450*time.Millisecond), true, func(heldDeliveryGuard) error { t.Fatal("admitted late"); return nil })
	elapsed := time.Since(began)
	if !errors.Is(e, ErrQuotaDeliveryBusy) || elapsed < 450*time.Millisecond || elapsed > 630*time.Millisecond {
		t.Fatal(elapsed, e)
	}
	if _, e = os.Stat(filepath.Join(r.root, deliveryName)); e != nil {
		t.Fatal("never reached flock", e)
	}
	t.Logf("250 ms of setup consumed the same 450 ms budget; returned at %v", elapsed)
}

func canonicalQuotaFixtureRoot(t *testing.T, path string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve fixture root: %v", err)
	}
	return root
}

func TestQuotaDeliveryProcessDeathDuringSetup(t *testing.T) {
	const fixture = "quota-file-crash-fixture"
	open := func(t *testing.T, root string) (*Store, RefreshCycle) {
		t.Helper()
		core, e := corestore.Open(t.Context(), root)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { core.Close() })
		lock, e := corestore.AcquireQuotaRefreshLock(t.Context(), root, time.Second)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { lock.Release() })
		s := NewWritableStore(core)
		return s, startIdentityCycle(t, t.Context(), s, lock, time.Now())
	}
	if len(os.Args) >= 3 && os.Args[len(os.Args)-2] == fixture {
		root, e := os.Getwd()
		if e != nil {
			t.Fatal(e)
		}
		if b, e := os.ReadFile("test-fixture"); e != nil || string(b) != fixture {
			t.Fatal("wrong fixture")
		}
		at, e := strconv.Atoi(os.Args[len(os.Args)-1])
		if e != nil || at < 1 || at > 7 {
			t.Fatal("bad boundary")
		}
		root = canonicalQuotaFixtureRoot(t, root)
		s, c := open(t, root)
		g := QuotaDeliveryGuard{root: root, cycle: c}
		count := 0
		g.syncFD = func(fd int) error {
			count++
			// Exit with descriptors and leases held, before acknowledging this sync.
			if count == at {
				os.Exit(73)
			}
			return unix.Fsync(fd)
		}
		e = g.acquire(t.Context(), time.Now().Add(time.Second), true, func(h heldDeliveryGuard) error {
			state, e := s.CommittedClaudeContext(t.Context(), c)
			if e != nil {
				return e
			}
			return g.publish(h, state.Generation)
		})
		t.Fatalf("child survived (syncs=%d err=%v)", count, e)
	}
	for idx, label := range []string{"created", "before-binding", "after-binding", "two-names", "after-unlink", "epoch-staged", "epoch-renamed"} {
		t.Run(label, func(t *testing.T) {
			root := canonicalQuotaFixtureRoot(t, t.TempDir())
			if e := os.WriteFile(filepath.Join(root, "test-fixture"), []byte(fixture), 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQuotaDeliveryProcessDeathDuringSetup$", "--", fixture, strconv.Itoa(idx+1))
			cmd.Dir = root
			b, e := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(e, &exit) || exit.ExitCode() != 73 {
				t.Fatalf("child %v: %s", e, b)
			}
			file := deliveryPending
			if idx >= 4 {
				file = deliveryName
			}
			before, e := os.Lstat(filepath.Join(root, file))
			if e != nil {
				t.Fatal(e)
			}
			s, c := open(t, root)
			if idx == 2 {
				// The child exited after XATTR_CREATE but before fsync(root).
				// Neither a failed re-sync nor expiry during a successful re-sync
				// may publish the final name on a later cycle.
				dir, err := unix.Open(root, unix.O_DIRECTORY|unix.O_RDONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer unix.Close(dir)
				original, ok, err := deliveryBinding(dir)
				if err != nil || !ok {
					t.Fatal("missing committed binding", err)
				}
				for _, expire := range []bool{false, true} {
					calls := 0
					deadline := time.Now().Add(time.Second)
					if expire {
						deadline = time.Now().Add(100 * time.Millisecond)
					}
					interrupted := QuotaDeliveryGuard{root: root, cycle: c, syncFD: func(fd int) error {
						calls++
						var st, physical unix.Stat_t
						if unix.Fstat(fd, &st) != nil || unix.Fstat(dir, &physical) != nil || st.Mode&unix.S_IFMT != unix.S_IFDIR || deliveryID(&st) != deliveryID(&physical) {
							t.Fatal("resumption did not sync the original root")
						}
						if e := requireDeliveryAbsent(dir, deliveryName); e != nil {
							t.Fatal("published before resumed sync")
						}
						if !expire {
							return unix.EIO
						}
						time.Sleep(time.Until(deadline) + time.Millisecond)
						return unix.Fsync(fd)
					}}
					err = interrupted.acquire(ctx, deadline, true, func(heldDeliveryGuard) error { t.Fatal("late publication"); return nil })
					want := ErrQuotaDeliveryUnavailable
					if expire {
						want = ErrQuotaDeliveryBusy
					}
					if !errors.Is(err, want) || calls != 1 {
						t.Fatalf("resume calls=%d err=%v", calls, err)
					}
					if err = requireDeliveryAbsent(dir, deliveryName); err != nil {
						t.Fatal("continued publication after failed/late sync", err)
					}
					got, ok, e := deliveryBinding(dir)
					if e != nil || !ok || got != original {
						t.Fatal("changed binding", e)
					}
					stage, ok, e := deliveryEntry(dir, deliveryPending)
					if e != nil || !ok || deliveryID(&stage) != original || stage.Nlink != 1 {
						t.Fatal("lost pending inode", e)
					}
				}
			}
			if _, e = s.ReconcileQuotaDelivery(ctx, c); e != nil {
				t.Fatal(e)
			}
			after, e := os.Lstat(filepath.Join(root, deliveryName))
			if e != nil || !os.SameFile(before, after) {
				t.Fatal("recovery switched inode", e)
			}
			if _, e = os.Lstat(filepath.Join(root, deliveryPending)); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("pending survived", e)
			}
			checkEpochImage(t, filepath.Join(root, quotaEpochName), 0)
		})
	}
}
