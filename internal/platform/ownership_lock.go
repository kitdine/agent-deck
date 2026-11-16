package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const ownershipLockWait = 5 * time.Second

type OwnershipLockError struct{ Reason string }

func (e *OwnershipLockError) Error() string { return e.Reason }

// OwnershipLock holds an OS advisory lock until Release or process exit. It is
// intentionally separate from state.lock and lives outside the state directory.
type OwnershipLock struct {
	mu        sync.Mutex
	file      *os.File
	account   string
	exclusive bool
}

func AcquireOwnershipLock(ctx context.Context, account string, exclusive bool) (*OwnershipLock, error) {
	return acquireOwnershipLock(ctx, account, exclusive, ownershipLockWait)
}

func acquireOwnershipLock(ctx context.Context, account string, exclusive bool, wait time.Duration) (*OwnershipLock, error) {
	if !filepath.IsAbs(account) {
		return nil, errors.New("real_root_unresolved")
	}
	dir := filepath.Join(account, "Library", "Application Support", "AgentDeck")
	if err := os.MkdirAll(dir, DirectoryMode); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, DirectoryMode); err != nil {
		return nil, err
	}
	fd, err := unix.Open(filepath.Join(dir, "ownership.lock"), unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, uint32(FileMode))
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "ownership.lock")
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("invalid ownership lock")
	}
	if err := file.Chmod(FileMode); err != nil {
		return nil, err
	}
	op, reason := unix.LOCK_SH, "ownership_busy"
	if exclusive {
		op, reason = unix.LOCK_EX, "owner_running"
	}
	deadline := time.Now().Add(wait)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := unix.Flock(fd, op|unix.LOCK_NB)
		if err == nil {
			failed = false
			return &OwnershipLock{file: file, account: account, exclusive: exclusive}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) && !errors.Is(err, unix.EINTR) {
			return nil, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, &OwnershipLockError{Reason: reason}
		}
		timer := time.NewTimer(min(10*time.Millisecond, remaining))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// WithExclusive keeps release and the protected publication in one lifetime.
// Receipt writers must use the same account and the same continuously held lock.
func (l *OwnershipLock) WithExclusive(account string, action func() error) error {
	if l == nil {
		return errors.New("exclusive ownership lock required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil || !l.exclusive || l.account != account {
		return errors.New("exclusive ownership lock required")
	}
	return action()
}

func (l *OwnershipLock) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
