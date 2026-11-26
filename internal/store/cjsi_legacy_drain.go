// Package-private admission for cooperating Store clients. The maintenance
// caller owns scan.lock and state.lock before entering drainLegacy.
package store

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Ordinary reads take a shared flock on the physical root directory. They
// create no files. A draining manager also locks this temporary subdirectory
// to close admission while outstanding directory leases finish. A crash drops
// both kernel locks; an abandoned empty subdirectory is safe to reuse.
const legacyDrainIntent = ".cjsi-legacy-drain"

type legacyRootID struct {
	dev uint64
	ino uint64
}

type legacyAdmission struct {
	fd   int
	root legacyRootID
}

func openLegacyRoot(root string) (legacyAdmission, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return legacyAdmission{}, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return legacyAdmission{}, err
	}
	return legacyAdmission{fd, legacyRootID{uint64(st.Dev), uint64(st.Ino)}}, nil
}

func legacyFlock(fd, mode int) error {
	err := unix.Flock(fd, mode|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrStateBusy
	}
	return err
}

func openLegacyIntent(fd int) (int, error) {
	return unix.Openat(fd, legacyDrainIntent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}

func admitLegacy(ctx context.Context, root string) (legacyAdmission, error) {
	if err := ctx.Err(); err != nil {
		return legacyAdmission{}, err
	}
	a, err := openLegacyRoot(root)
	if err != nil {
		return legacyAdmission{}, err
	}
	if err = legacyFlock(a.fd, unix.LOCK_SH); err == nil {
		var intent int
		intent, err = openLegacyIntent(a.fd)
		switch {
		case err == nil:
			err = errors.Join(legacyFlock(intent, unix.LOCK_SH), unix.Close(intent))
		case errors.Is(err, unix.ENOENT):
			err = nil
		}
	}
	if err == nil {
		err = checkStateRestoreMarkerAt(ctx, a)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return legacyAdmission{}, errors.Join(err, a.close())
	}
	return a, nil
}

func (a legacyAdmission) close() error { return unix.Close(a.fd) }

// Refuse file aliases whose SQLite inode would belong to another managed
// directory. Root-directory aliases themselves share the same kernel lock.
func admitLegacyFile(ctx context.Context, path string) (legacyAdmission, error) {
	a, err := admitLegacy(ctx, filepath.Dir(path))
	if err != nil {
		return legacyAdmission{}, err
	}
	var st unix.Stat_t
	err = unix.Fstatat(a.fd, filepath.Base(path), &st, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil && (st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1) {
		err = errors.New("invalid legacy SQLite file")
	}
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return legacyAdmission{}, errors.Join(err, a.close())
	}
	return a, nil
}

// wait bounds handoff only. Zero permits one nonblocking attempt. Earlier
// context cancellation/deadline wins; expiry of this budget means state_busy.
type cjsiStoreLegacyManager struct{ wait time.Duration }

var _ cjsiLegacyManager = cjsiStoreLegacyManager{}

func (m cjsiStoreLegacyManager) drainLegacy(ctx context.Context, root string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(m.wait)
	a, err := openLegacyRoot(root)
	if err != nil {
		return nil, err
	}
	err = unix.Mkdirat(a.fd, legacyDrainIntent, 0700)
	created := err == nil
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return nil, errors.Join(err, a.close())
	}
	intent, err := openLegacyIntent(a.fd)
	if err == nil {
		err = waitLegacyExclusive(ctx, intent, deadline)
		if err != nil {
			err = errors.Join(err, unix.Close(intent))
		}
	}
	if err != nil {
		if created {
			err = errors.Join(err, unix.Unlinkat(a.fd, legacyDrainIntent, unix.AT_REMOVEDIR))
		}
		return nil, errors.Join(err, a.close())
	}
	// The outer maintenance locks serialize issuers, including unlink/reuse
	// of the marker. All admission locks are advisory and process-death safe.
	release := sync.OnceValue(func() error {
		unlinkErr := unix.Unlinkat(a.fd, legacyDrainIntent, unix.AT_REMOVEDIR)
		return errors.Join(unlinkErr, unix.Close(intent), a.close())
	})
	// Retire only this root's cached entries. Active leases and initializers
	// finish on their ordinary release path. Physical SQLite connections have
	// independent directory pins, even after database/sql reports DB.Close.
	retired := processReadPools.retireLegacy(a.root)
	if len(retired) != 0 {
		done := make(chan error, 1)
		go func() { done <- closeReadDBs(retired) }()
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		select {
		case err = <-done:
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer.C:
			err = ErrStateBusy
		}
	}
	if err == nil {
		err = waitLegacyExclusive(ctx, a.fd, deadline)
	}
	if err != nil {
		return nil, errors.Join(err, ctx.Err(), release())
	}
	return release, nil
}

func waitLegacyExclusive(ctx context.Context, fd int, deadline time.Time) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := legacyFlock(fd, unix.LOCK_EX)
		if err == nil {
			return ctx.Err()
		}
		remaining := time.Until(deadline)
		if !errors.Is(err, ErrStateBusy) || remaining <= 0 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min(remaining, 10*time.Millisecond)):
		}
	}
}

// Ensure SQLite receives a filename URI even if a root contains '?' or '#'.
func legacyURI(path, query string) string {
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = query
	return u.String()
}
