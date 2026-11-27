package store

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// caseFoldedRoot returns a state root spelled "CaseRoot" and the same
// directory spelled "caseroot". The pair exists only on a case-insensitive
// volume (the macOS APFS default); elsewhere the test is skipped.
func caseFoldedRoot(t *testing.T) (root, folded string) {
	t.Helper()
	parent := t.TempDir()
	root = filepath.Join(parent, "CaseRoot")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	folded = filepath.Join(parent, "caseroot")
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	foldedInfo, err := os.Stat(folded)
	if err != nil || !os.SameFile(rootInfo, foldedInfo) {
		t.Skipf("temporary volume at %s is case-sensitive; a differently cased path names another entry there", parent)
	}
	return root, folded
}

// On a case-insensitive volume a differently cased path names the same
// directory entry. Ownership follows the entry the file system resolves, so
// the holder's own lock stays its own however the path is spelled, while
// another hard-link entry of the same file remains someone else's name.
func TestLockOwnershipFollowsTheDirectoryEntryNotItsSpelling(t *testing.T) {
	root, folded := caseFoldedRoot(t)
	lock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	alive := func(int) (bool, bool) { return true, true }

	owned := []string{
		filepath.Join(root, "scan.lock"),
		filepath.Join(folded, "scan.lock"),
		filepath.Join(folded, "SCAN.LOCK"),
		filepath.Join(root, "Scan.Lock"),
	}
	for _, path := range owned {
		reason, isOwned, err := ClassifyLockOwnership(path, alive)
		if err != nil || reason != LockReasonLive || !isOwned {
			t.Errorf("ClassifyLockOwnership(%s) = %s, owned=%v, %v; want lock_live, owned", path, reason, isOwned, err)
		}
		if !LockOwnedByCurrentProcess(path) {
			t.Errorf("LockOwnedByCurrentProcess(%s) = false for the held lock", path)
		}
	}

	// A second directory entry for the same file is not the acquired entry,
	// however it is spelled, and its existence does not disown the real one.
	alias := filepath.Join(root, "state.lock")
	if err := os.Link(filepath.Join(root, "scan.lock"), alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alias, filepath.Join(folded, "STATE.LOCK")} {
		reason, isOwned, err := ClassifyLockOwnership(path, alive)
		if err != nil || reason != LockReasonLive || isOwned {
			t.Errorf("hard-link alias %s = %s, owned=%v, %v; want lock_live, not owned", path, reason, isOwned, err)
		}
	}
	for _, path := range owned {
		if reason, isOwned, err := ClassifyLockOwnership(path, alive); err != nil || reason != LockReasonLive || !isOwned {
			t.Errorf("held lock at %s with another hard link = %s, owned=%v, %v; want lock_live, owned", path, reason, isOwned, err)
		}
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	for _, path := range owned {
		if LockOwnedByCurrentProcess(path) {
			t.Errorf("released lock still owned via %s", path)
		}
	}
}

// swapLockEntry keeps replacing path atomically, alternating between a regular
// file and a FIFO, until stop is closed. Both are hard links of fixed inodes,
// so a reader stuck opening the FIFO can always be released through fifo.
func swapLockEntry(t *testing.T, path, regular, fifo string, stop <-chan struct{}, swaps *atomic.Int64) {
	t.Helper()
	tmp := path + ".swap"
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}
		source := regular
		if i%2 == 1 {
			source = fifo
		}
		_ = os.Remove(tmp)
		if os.Link(source, tmp) == nil && os.Rename(tmp, path) == nil {
			swaps.Add(1)
		}
	}
}

// releaseFIFOReaders opens the FIFO's write end without blocking, which
// completes any open stuck waiting for a writer, then closes it again.
func releaseFIFOReaders(fifo string) {
	fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err == nil {
		time.Sleep(10 * time.Millisecond)
		_ = syscall.Close(fd)
	}
}

// boundedLockRace runs probe repeatedly while the lock entry flips between a
// regular file and a FIFO. A probe that does not return within the bound has
// blocked on the FIFO; it is released before the test fails, so the test
// itself never hangs.
func boundedLockRace(t *testing.T, path, regular, fifo string, window time.Duration, probe func()) {
	t.Helper()
	stop := make(chan struct{})
	var swaps atomic.Int64
	var swapper sync.WaitGroup
	swapper.Add(1)
	go func() {
		defer swapper.Done()
		swapLockEntry(t, path, regular, fifo, stop, &swaps)
	}()
	defer func() {
		close(stop)
		swapper.Wait()
	}()

	const bound = 2 * time.Second
	deadline := time.Now().Add(window)
	attempts := 0
	for time.Now().Before(deadline) {
		attempts++
		done := make(chan struct{})
		go func() {
			defer close(done)
			probe()
		}()
		select {
		case <-done:
			continue
		case <-time.After(bound):
		}
		for {
			releaseFIFOReaders(fifo)
			select {
			case <-done:
			case <-time.After(50 * time.Millisecond):
				continue
			}
			break
		}
		t.Fatalf("attempt %d blocked for %s on a lock entry swapped to a FIFO (swaps=%d)", attempts, bound, swaps.Load())
	}
	if swaps.Load() == 0 {
		t.Fatal("the lock entry was never swapped; the race was not exercised")
	}
}

func fifoFixture(t *testing.T, dir string) string {
	t.Helper()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	return fifo
}

// Health reads classify the lock without holding it. If the entry becomes a
// FIFO between inspection and open, the read must still return promptly and
// report an unknown owner instead of waiting for a writer.
func TestLockClassificationDoesNotBlockWhenTheEntryBecomesAFIFO(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "scan.lock")
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("v1:1:00000000000000000000000000000001"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := fifoFixture(t, root)
	var sawFIFO atomic.Bool
	boundedLockRace(t, path, regular, fifo, 1500*time.Millisecond, func() {
		reason, owned, err := ClassifyLockOwnership(path, func(int) (bool, bool) { return true, true })
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if owned {
			t.Errorf("a swapped lock entry was reported as owned")
		}
		if reason == LockReasonOwnerUnknown {
			sawFIFO.Store(true)
		} else if reason != LockReasonLive {
			t.Errorf("reason = %s, want lock_live or lock_owner_unknown", reason)
		}
		_ = LockOwnedByCurrentProcess(path)
	})
	if !sawFIFO.Load() {
		t.Log("no probe observed the FIFO; the race window was not hit this run")
	}
}

// Release opens the lock entry to confirm its token. A FIFO swapped in after
// the identity check must not block the releasing process.
func TestLockReleaseDoesNotBlockWhenTheEntryBecomesAFIFO(t *testing.T) {
	root := t.TempDir()
	lock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := lock.path
	regular := filepath.Join(root, "regular")
	if err := os.Link(path, regular); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	fifo := fifoFixture(t, root)
	boundedLockRace(t, path, regular, fifo, 1500*time.Millisecond, func() {
		if err := lock.releaseOnce(); err != nil && !errors.Is(err, ErrLockLost) {
			t.Errorf("release = %v, want nil or ErrLockLost", err)
		}
		// A successful release removes the entry; restore it so later
		// attempts keep racing the swap instead of finding nothing.
		_ = os.Link(regular, path)
	})
}

// The lock is registered from the same directory resolution that created it.
// If the state directory is moved aside and replaced after the entry is
// created, the entry actually acquired stays the holder's, and a same-named
// alias placed in the replacement directory is another entry.
func TestLockRegistrationUsesTheDirectoryTheEntryWasCreatedIn(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(parent, "state-moved")
	lockCreatedHook = func(path string) {
		if filepath.Base(path) != "scan.lock" {
			return
		}
		if err := os.Rename(root, moved); err != nil {
			t.Error(err)
			return
		}
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Error(err)
			return
		}
		if err := os.Link(filepath.Join(moved, "scan.lock"), filepath.Join(root, "scan.lock")); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { lockCreatedHook = nil })
	lock, err := AcquireScanLock(context.Background(), root, 0)
	lockCreatedHook = nil
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = lock.Release()
		unregisterOwnedLock(lock.token)
	})
	alive := func(int) (bool, bool) { return true, true }

	acquired := filepath.Join(moved, "scan.lock")
	if reason, owned, err := ClassifyLockOwnership(acquired, alive); err != nil || reason != LockReasonLive || !owned {
		t.Errorf("acquired entry %s = %s, owned=%v, %v; want lock_live, owned", acquired, reason, owned, err)
	}
	alias := filepath.Join(root, "scan.lock")
	if reason, owned, err := ClassifyLockOwnership(alias, alive); err != nil || reason != LockReasonLive || owned {
		t.Errorf("alias in the replacement directory %s = %s, owned=%v, %v; want lock_live, not owned", alias, reason, owned, err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release of the acquired entry in its moved directory = %v", err)
	}
	if _, err := os.Lstat(acquired); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Release returned nil but the acquired entry still exists: Lstat = %v", err)
	}
	if token, err := os.ReadFile(alias); err != nil || string(token) != lock.token {
		t.Errorf("replacement hard link after Release = %q, %v; want original token intact", token, err)
	}
	if lockRegistered(lock.token) {
		t.Error("released lock stays registered")
	}
}

// On a case-sensitive volume a case-only rename makes another name. Once the
// name the lock was created under no longer resolves to the file, the renamed
// entry is not provably the acquired one and keeps reporting contention.
// The case-insensitive APFS default cannot express this and skips.
func TestLockOwnershipRequiresTheCreatedNameToStillResolve(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "PROBE")); err == nil {
		t.Skipf("temporary volume at %s is case-insensitive; a case-only rename keeps the created name resolving", root)
	}
	lock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	renamed := filepath.Join(root, "Scan.lock")
	if err := os.Link(lock.path, renamed); err != nil {
		t.Fatal(err)
	}
	if !LockOwnedByCurrentProcess(lock.path) || LockOwnedByCurrentProcess(renamed) {
		t.Fatal("distinct case-only hard links must be owned only under the acquired name")
	}
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(lock.path, renamed); err != nil {
		t.Fatal(err)
	}
	if _, owned, _ := ClassifyLockOwnership(renamed, func(int) (bool, bool) { return true, true }); owned {
		t.Errorf("case-renamed entry %s reported owned although %s no longer resolves", renamed, lock.path)
	}
	if LockOwnedByCurrentProcess(renamed) {
		t.Errorf("LockOwnedByCurrentProcess(%s) = true for a renamed entry", renamed)
	}
}

// shortLockRoot returns a fresh directory whose paths fit a Unix socket
// address (104 bytes on Darwin), which a per-test TMPDIR may exceed.
func shortLockRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if len(filepath.Join(root, "scan.lock")) < 100 {
		return root
	}
	root, err := os.MkdirTemp("/private/tmp", "adlk")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func lockRegistered(token string) bool {
	ownedLocks.mu.Lock()
	defer ownedLocks.mu.Unlock()
	_, ok := ownedLocks.records[token]
	return ok
}

// An entry replaced by a Unix socket cannot be opened on Darwin at all
// (EOPNOTSUPP from open). It is still not the lock this process wrote, so the
// release reports the lock as lost and forgets it.
func TestLockReleaseReportsASocketEntryAsLost(t *testing.T) {
	root := shortLockRoot(t)
	lock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unregisterOwnedLock(lock.token) })
	if err := os.Remove(lock.path); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", lock.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	if reason, owned, err := ClassifyLockOwnership(lock.path, func(int) (bool, bool) { return true, true }); err != nil || reason != LockReasonOwnerUnknown || owned {
		t.Errorf("socket entry classified %s, owned=%v, %v; want lock_owner_unknown, not owned", reason, owned, err)
	}
	if err := lock.Release(); !errors.Is(err, ErrLockLost) {
		t.Errorf("Release over a socket entry = %v, want ErrLockLost", err)
	}
	if lockRegistered(lock.token) {
		t.Error("lost lock stays registered as owned")
	}
	if info, err := os.Lstat(lock.path); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Errorf("release touched the socket entry: %v, %v", info, err)
	}
}

// A permission error on the lock's own regular file is not proof the lock is
// gone: the release reports it and stays retryable, keeping the registration.
func TestLockReleaseKeepsAnUnreadableRegularEntryRetryable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads files regardless of mode")
	}
	root := t.TempDir()
	lock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(lock.path, 0o600)
		_ = lock.Release()
		unregisterOwnedLock(lock.token)
	})
	if err := os.Chmod(lock.path, 0); err != nil {
		t.Fatal(err)
	}
	err = lock.Release()
	if err == nil || errors.Is(err, ErrLockLost) {
		t.Fatalf("Release of an unreadable lock = %v, want a retryable error", err)
	}
	if !lockRegistered(lock.token) {
		t.Fatal("a retryable release dropped the registration")
	}
	if err := os.Chmod(lock.path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("retried Release = %v", err)
	}
	if lockRegistered(lock.token) {
		t.Fatal("released lock stays registered")
	}
	if _, err := os.Stat(lock.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retried release left the entry: %v", err)
	}
}
