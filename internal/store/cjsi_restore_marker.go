package store

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

const stateRestoreMarkerName = ".cjsi-restore-in-progress"

// ErrStateRestoreInProgress identifies a target reserved for recovery. It also
// covers unmarked artifacts that cannot distinguish a fresh root from Restore.
var ErrStateRestoreInProgress = &Error{Code: "state_restore_in_progress"}

// ErrStateCoreUninitialized reports an otherwise clean interrupted first open.
// A read-only caller cannot finish initialization.
var ErrStateCoreUninitialized = &Error{Code: "state_core_uninitialized"}

// MarkStateRestoreInProgress reserves a validated empty Restore target before
// the first database file is created. The caller owns scan.lock then state.lock.
// Existing SQLite admission is drained before durably creating the marker.
func MarkStateRestoreInProgress(ctx context.Context, root string) (err error) {
	root, err = platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	if err := checkStateRestoreMarker(root); err != nil {
		return err
	}
	scanEntry, err := readOwnedRestoreLock(root, "scan.lock")
	if err != nil {
		return err
	}
	stateEntry, err := readOwnedRestoreLock(root, "state.lock")
	if err != nil {
		return err
	}
	release, err := (cjsiStoreLegacyManager{wait: lockWait}).drainLegacy(ctx, root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, release()) }()
	marker, err := os.OpenFile(filepath.Join(root, stateRestoreMarkerName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, platform.FileMode)
	if errors.Is(err, os.ErrExist) {
		return ErrStateRestoreInProgress
	}
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(marker, restoreLockBinding(scanEntry.token, stateEntry.token))
	if err := errors.Join(writeErr, marker.Sync(), marker.Close()); err != nil {
		return err
	}
	return cjsiSyncLayoutPath(root, true)
}

// ClearStateRestoreInProgress releases the target after durable publication
// or complete rollback. The caller still owns scan.lock then state.lock.
func ClearStateRestoreInProgress(root string) error {
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	if err := cjsiRemoveLayoutFile(filepath.Join(root, stateRestoreMarkerName)); err != nil {
		return err
	}
	return cjsiSyncLayoutPath(root, true)
}

// CheckStateRestoreInProgress detects a durable marker without writing state.
func CheckStateRestoreInProgress(root string) error {
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	return checkStateRestoreMarker(root)
}

func checkStateRestoreMarker(root string) error {
	_, err := os.Lstat(filepath.Join(root, stateRestoreMarkerName))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	default:
		return ErrStateRestoreInProgress
	}
}

type restoreResolverAdmissionKey struct{}

type restoreResolverAdmission struct {
	root, marker cjsiFileIdentity
}

func checkStateRestoreMarkerAt(ctx context.Context, admission legacyAdmission) error {
	var stat unix.Stat_t
	err := unix.Fstatat(admission.fd, stateRestoreMarkerName, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	if proof, ok := ctx.Value(restoreResolverAdmissionKey{}).(restoreResolverAdmission); ok &&
		proof.root == (cjsiFileIdentity{admission.root.dev, admission.root.ino}) &&
		proof.marker == (cjsiFileIdentity{uint64(stat.Dev), uint64(stat.Ino)}) {
		return nil
	}
	return ErrStateRestoreInProgress
}

func readOwnedRestoreLock(root, name string) (lockEntry, error) {
	entry, state, err := readLockFile(filepath.Join(root, name))
	if err != nil || state != lockFileOK || !entry.owned {
		return lockEntry{}, errors.Join(ErrLockLost, err)
	}
	return entry, nil
}

func restoreLockBinding(scanToken, stateToken string) string {
	return scanToken + "\n" + stateToken + "\n"
}

// ResolveCJSIRuntimeRootForRestore verifies a published layout before clearing
// the marker. The same live scan/state handles must still own the marker.
// The ordinary resolver's checks run with a root- and inode-bound admission
// limited to this call. The returned handle carries no admission exemption.
func ResolveCJSIRuntimeRootForRestore(ctx context.Context, root string, scanLock, stateLock *Lock) (_ *CJSIRuntimeRoot, err error) {
	root, err = platform.GuardStateRoot(root)
	if err != nil {
		return nil, err
	}
	if scanLock == nil || stateLock == nil || scanLock.path != filepath.Join(root, "scan.lock") || stateLock.path != filepath.Join(root, "state.lock") {
		return nil, ErrLockLost
	}
	scanLock.mu.Lock()
	defer scanLock.mu.Unlock()
	stateLock.mu.Lock()
	defer stateLock.mu.Unlock()
	scanEntry, err := verifyRestoreLock(scanLock, root, "scan.lock")
	if err != nil {
		return nil, err
	}
	stateEntry, err := verifyRestoreLock(stateLock, root, "state.lock")
	if err != nil {
		return nil, err
	}
	markerPath := filepath.Join(root, stateRestoreMarkerName)
	marker, err := cjsiOpenPrivateLayoutPath(markerPath, false)
	if err != nil {
		return nil, errors.Join(ErrLockLost, err)
	}
	defer func() { err = errors.Join(err, marker.Close()) }()
	contents, err := io.ReadAll(io.LimitReader(marker, 2*maxLockTokenBytes+3))
	if err != nil || string(contents) != restoreLockBinding(scanEntry.token, stateEntry.token) {
		return nil, errors.Join(ErrLockLost, err)
	}
	info, err := marker.Stat()
	if err != nil {
		return nil, err
	}
	proof := restoreResolverAdmission{
		root: cjsiIdentity(scanEntry.dir.Sys().(*syscall.Stat_t)), marker: cjsiIdentity(info.Sys().(*syscall.Stat_t)),
	}
	return ResolveCJSIRuntimeRoot(context.WithValue(ctx, restoreResolverAdmissionKey{}, proof), root)
}

func verifyRestoreLock(lock *Lock, root, name string) (lockEntry, error) {
	if lock.released || lock.dir == nil || lock.info == nil || lock.token == "" {
		return lockEntry{}, ErrLockLost
	}
	entry, err := readOwnedRestoreLock(root, name)
	if err != nil {
		return lockEntry{}, err
	}
	dir, err := lock.dir.Stat()
	if err != nil || entry.token != lock.token || !os.SameFile(entry.info, lock.info) || !os.SameFile(entry.dir, dir) {
		return lockEntry{}, errors.Join(ErrLockLost, err)
	}
	return entry, nil
}

// A missing core may be a new root only while its physical layout provides
// no evidence of a managed installation or an interrupted Restore.
func checkCJSICorePath(root string) error {
	_, err := inspectCJSICorePath(root)
	return err
}

func inspectCJSICorePath(root string) (cold bool, err error) {
	if err := checkStateRestoreMarker(root); err != nil {
		return false, err
	}
	path := filepath.Join(root, "agentdeck.sqlite3")
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() != 0 {
			return false, nil
		}
		// The main and any present sidecars must be owned and empty.
		// Writers will restore private permissions before initialization.
		// This covers both a killed legacy opener and an interrupted O_EXCL.
		for _, suffix := range [...]string{"", "-wal", "-shm", "-journal"} {
			file, err := ownedLegacySQLiteInfo(path + suffix)
			if suffix != "" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, err
			}
			if file.Size() != 0 {
				return false, ErrStateRestoreInProgress
			}
		}
		cold = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	} else if err := cjsiRequireAbsentSidecars(path); err != nil {
		return false, errors.Join(ErrStateRestoreInProgress, err)
	}
	if err := cjsiCheckLegacyRuntimeFiles(root); err != nil {
		return false, errors.Join(ErrStateRestoreInProgress, err)
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".cjsi-restore-") {
			return false, ErrStateRestoreInProgress
		}
	}
	return cold, nil
}
