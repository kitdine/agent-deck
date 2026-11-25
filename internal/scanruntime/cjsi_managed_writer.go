package scanruntime

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

// A legacy Serve opens the root receipt journal BEFORE deleting a socket or
// publishing owner.json. It ignores new election locks. A managed journal
// directory at that same name therefore fences even an older executable.
// The v1 journal itself keeps exactly the same contents and retry semantics.
type managedWorkerElection struct {
	physical string
	root     store.CJSIRuntimeRoot
	path     string
	dir      *os.File
}

func electManagedWorker(root string, m store.CJSIRuntimeRoot) (*managedWorkerElection, *receiptJournal, error) {
	if err := m.Check(); err != nil {
		return nil, nil, err
	}
	path := filepath.Join(root, receiptJournalFilename)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err = os.Mkdir(path, 0700); err != nil {
			return nil, nil, err
		}
	case err != nil:
		return nil, nil, err
	case info.Mode().IsRegular():
		if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
			return nil, nil, errors.New("invalid private scan journal")
		}
		if _, err = openReceiptJournal(root); err != nil {
			return nil, nil, err
		}
		if err = moveManagedJournal(root, path); err != nil {
			return nil, nil, err
		}
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	dir := os.NewFile(uintptr(fd), path)
	e := &managedWorkerElection{scanPhysicalIdentity(root), m, path, dir}
	if err = e.check(); err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	}
	if errors.Is(err, unix.EWOULDBLOCK) {
		err = store.ErrStateBusy
	}
	if err != nil {
		return nil, nil, errors.Join(err, dir.Close())
	}
	// Sync on each entry so that an interrupted initial handoff is also
	// durable before scan.lock is given up.
	if err = syncDirectory(root); err != nil {
		return nil, nil, errors.Join(err, dir.Close())
	}
	j, err := openReceiptJournal(path)
	if err != nil {
		return nil, nil, errors.Join(err, dir.Close())
	}
	return e, j, nil
}

// Caller holds the startup scan.lock. A single exchange makes the previous
// journal's inode reachable inside the elected directory. At either crash
// boundary the authoritative name leads to all previously accepted receipts.
func moveManagedJournal(root, path string) (err error) {
	stage, err := os.MkdirTemp(root, ".cjsi-scan-journal-")
	if err != nil {
		return err
	}
	member := filepath.Join(stage, receiptJournalFilename)
	swapped := false
	defer func() {
		if swapped && err != nil {
			return
		}
		if !swapped {
			if e := os.Remove(member); e != nil && !errors.Is(e, os.ErrNotExist) {
				err = errors.Join(err, e)
			}
		}
		err = errors.Join(err, os.Remove(stage))
	}()
	if err = os.Link(path, member); err != nil {
		return err
	}
	if err = syncDirectory(stage); err != nil {
		return err
	}
	if err = unix.RenamexNp(stage, path, unix.RENAME_SWAP); err != nil {
		return err
	}
	swapped = true
	return syncDirectory(root)
}

func (e *managedWorkerElection) check() error {
	if err := e.root.Check(); err != nil {
		return err
	}
	h, err := e.dir.Stat()
	if err != nil {
		return err
	}
	entry, err := os.Lstat(e.path)
	if err != nil {
		return err
	}
	if h.Mode() != os.ModeDir|0700 || h.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) || !os.SameFile(entry, h) {
		return store.ErrLockLost
	}
	return nil
}

func removeStaleWorkerSocket(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	conn, err := dial(path)
	if err == nil {
		conn.Close()
		return store.ErrStateBusy
	}
	if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return removeSocket(path)
}

func removeWorkerEntry(path string, owned os.FileInfo) {
	if current, err := os.Lstat(path); err == nil && owned != nil && os.SameFile(current, owned) {
		_ = os.Remove(path)
	}
}

func (r *round) publishManaged(ctx context.Context, m store.CJSIRuntimeRoot, sources []ingest.Source) {
	// A worker round carries the journal election that admitted it. The
	// runner re-verifies it under its own scan/state locks before each write
	// transaction. Forced-local execution never held an election.
	var election store.CJSIRuntimeElection
	if r.election != nil {
		election = r.election.check
	}
	r.mu.RLock()
	scope := r.managedScope
	r.mu.RUnlock()
	out, err := dispatchCJSIRound(ctx, r.managedPlanner, scope, CJSIRoundRequest{Root: m, Home: r.home, Sources: sources, Election: election})
	if err != nil {
		r.setUsage(failedUsage(err, 0))
		r.setSession(failedSession(err, 0))
		return
	}
	r.setUsage(UsageResult{State: "completed", Changes: out.Usage})
	if scope == ScopeUsage {
		r.setSession(SessionResult{State: "pending"})
		return
	}
	r.setSession(SessionResult{State: "completed", Scan: session.ScanResult{Sources: out.Sources, Documents: out.Documents, Skipped: out.Skipped, Removed: out.Removed}})
}

// The path-based wire ID remains stable for receipt recovery. Bind a managed
// request separately to the physical directory and existing core at dispatch.
// A legacy request/root can lack the additional identity; it cannot authorize
// a managed worker running on a successor at the same path.
func scanPhysicalIdentity(root string) string {
	dir, err := os.Lstat(root)
	if err != nil || !dir.IsDir() {
		return ""
	}
	core, err := os.Lstat(filepath.Join(root, "agentdeck.sqlite3"))
	if err != nil || !core.Mode().IsRegular() {
		return ""
	}
	d := dir.Sys().(*syscall.Stat_t)
	c := core.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%x-%x/%x-%x", uint64(d.Dev), d.Ino, uint64(c.Dev), c.Ino)
}

// Journal renames use pathnames. Hold the short maintenance barrier while
// changing them so a cooperating root replacement cannot redirect the write.
// Never retain this guard for a scan, a client wait, or slot I/O.
func (s *server) guardManagedJournal() (func() error, error) {
	if s.election == nil {
		return nil, nil
	}
	lock, err := store.AcquireScanLock(s.ctx, s.stateRoot, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if err = s.election.check(); err != nil {
		return nil, errors.Join(err, lock.Release())
	}
	return lock.Release, nil
}
