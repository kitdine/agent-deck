package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// CJSISlotReceipt describes a verified private file. Store seal admission and
// publication still require their own live attempt, deadline and revision checks.
type CJSISlotReceipt struct {
	Identity [16]byte
	Digest   [sha256.Size]byte
	Size     int64
}

// WriteCJSIServingSlot borrows the caller's pinned directory and checks its
// identity against the root admitted for this attempt. Call after releasing
// blocking root locks and SQLite snapshots, with a deadline for result acceptance.
// EWOULDBLOCK means a write already occupies this physical root.
func WriteCJSIServingSlot(ctx context.Context, root *os.File, admitted os.FileInfo, identity [16]byte, contents []byte) (CJSISlotReceipt, error) {
	return writeCJSIServingSlot(ctx, root, admitted, identity, contents, func(dir int, id [16]byte, payload []byte) (cjsiDurableSlot, error) {
		return persistCJSIServingSlotAt(dir, id, payload, unix.Fsync)
	})
}

type cjsiSlotWriter func(dir int, identity [16]byte, payload []byte) (cjsiDurableSlot, error)

func writeCJSIServingSlot(ctx context.Context, root *os.File, admitted os.FileInfo, identity [16]byte, contents []byte, persist cjsiSlotWriter) (CJSISlotReceipt, error) {
	if err := cjsiSlotWaitError(ctx); err != nil {
		return CJSISlotReceipt{}, err
	}
	if err := validateCJSISlotInput(identity, contents); err != nil {
		return CJSISlotReceipt{}, err
	}
	dir, err := acquireCJSISlotDirectory(root, admitted)
	if err != nil {
		return CJSISlotReceipt{}, err
	}
	lock, err := acquireCJSIWorkerLock(int(dir.Fd()), ".cjsi-serving-worker.lock")
	if err != nil {
		dir.Close()
		return CJSISlotReceipt{}, err
	}
	payload := bytes.Clone(contents)
	if err := cjsiSlotWaitError(ctx); err != nil {
		lock.Close()
		dir.Close()
		return CJSISlotReceipt{}, err
	}

	type result struct {
		slot cjsiDurableSlot
		err  error
	}
	done := make(chan result, 1)
	go func() {
		slot, err := persist(int(dir.Fd()), identity, payload)
		// Closing our independent descriptor releases occupancy even when
		// the waiter has already gone. Release before advertising completion.
		if closeErr := errors.Join(lock.Close(), dir.Close()); err == nil && closeErr != nil {
			slot, err = cjsiDurableSlot{}, closeErr
		}
		done <- result{slot, err}
	}()
	select {
	case <-ctx.Done():
		return CJSISlotReceipt{}, ctx.Err()
	case r := <-done:
		// Both cases can be ready, and the deadline timer may run late.
		if err := cjsiSlotWaitError(ctx); err != nil {
			return CJSISlotReceipt{}, err
		}
		if r.err != nil {
			return CJSISlotReceipt{}, r.err
		}
		return CJSISlotReceipt{r.slot.identity, r.slot.digest, r.slot.size}, nil
	}
}

func cjsiSlotWaitError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return errCJSISlotAdmission
	}
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func acquireCJSISlotDirectory(root *os.File, admitted os.FileInfo) (*os.File, error) {
	conn, err := root.SyscallConn()
	if err != nil {
		return nil, err
	}
	// An open of "." has its own flock lifetime. dup would share the
	// caller's open file description, and a pathname could select a successor.
	var fd int
	var openErr error
	err = conn.Control(func(borrowed uintptr) {
		fd, openErr = unix.Openat(int(borrowed), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	})
	if err != nil {
		return nil, err
	}
	if openErr != nil {
		return nil, openErr
	}
	dir := os.NewFile(uintptr(fd), "cjsi slot directory")
	info, err := dir.Stat()
	if err == nil && (!os.SameFile(info, admitted) || info.Mode() != os.ModeDir|0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid())) {
		err = errCJSISlotAdmission
	}

	if err != nil {
		dir.Close()
		return nil, err
	}
	return dir, nil
}

// Occupancy belongs to the physical root, on a separate inode from the shared
// root-admission flock. The entry is persistent, even after the worker exits.
func acquireCJSIWorkerLock(dir int, name string) (*os.File, error) {
	fd, err := unix.Openat(dir, name, unix.O_RDWR|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CREAT|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	var held, named unix.Stat_t
	if err = unix.Fstat(fd, &held); err == nil && !privateCJSISlotFile(&held, 0) {
		err = errCJSISlotAdmission
	}
	if err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	}
	if err == nil {
		err = unix.Fstatat(dir, name, &named, unix.AT_SYMLINK_NOFOLLOW)
	}
	if err != nil || !privateCJSISlotFile(&named, 0) || named.Dev != held.Dev || named.Ino != held.Ino {
		f.Close()
		return nil, errors.Join(err, errCJSISlotAdmission)
	}
	return f, nil
}
