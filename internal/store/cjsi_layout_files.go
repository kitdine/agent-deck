package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"net/url"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type cjsiLayoutFile struct {
	identity cjsiFileIdentity
	size     int64
	digest   [sha256.Size]byte
}

func (f cjsiLayoutFile) present() bool { return f.identity.inode != 0 }

func (f cjsiLayoutFile) valid() bool {
	return f == (cjsiLayoutFile{}) || f.present() && f.size >= 0 && f.digest != ([sha256.Size]byte{})
}

type cjsiLegacyFiles struct {
	main cjsiLayoutFile
	wal  cjsiLayoutFile
}

func (f cjsiLegacyFiles) valid() bool {
	return f.main.valid() && f.wal.valid() &&
		(f.main.present() && f.main.size > 0 || !f.main.present() && f.wal.size == 0)
}

var cjsiSidecarSuffixes = [...]string{"-wal", "-shm", "-journal"}

func cjsiCheckInstallRoot(root string, expected cjsiFileIdentity) error {
	stat, err := privateServingPath(root, true)
	if err != nil || cjsiIdentity(stat) != expected {
		return errCJSILayoutInvalid
	}
	return nil
}

func cjsiOpenPrivateLayoutPath(path string, directory bool) (*os.File, error) {
	stat, err := privateServingPath(path, directory)
	if err != nil {
		return nil, err
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	opened := info.Sys().(*syscall.Stat_t)
	if cjsiIdentity(stat) != cjsiIdentity(opened) || stat.Mode != opened.Mode ||
		stat.Uid != opened.Uid || (!directory && opened.Nlink != 1) {
		file.Close()
		return nil, errCJSILayoutInvalid
	}
	return file, nil
}

func cjsiSyncLayoutPath(path string, directory bool) error {
	file, err := cjsiOpenPrivateLayoutPath(path, directory)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

func cjsiReadLayoutFile(ctx context.Context, path string, sync bool) (image cjsiLayoutFile, err error) {
	file, err := cjsiOpenPrivateLayoutPath(path, false)
	if err != nil {
		return cjsiLayoutFile{}, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	before, err := file.Stat()
	if err != nil {
		return cjsiLayoutFile{}, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, probeContextReader{ctx, file})
	if err != nil {
		return cjsiLayoutFile{}, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return cjsiLayoutFile{}, err
	}
	if n != before.Size() || n != after.Size() || !os.SameFile(before, after) ||
		before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return cjsiLayoutFile{}, errCJSILayoutInvalid
	}
	image = cjsiLayoutFile{identity: cjsiIdentity(before.Sys().(*syscall.Stat_t)), size: n}
	copy(image.digest[:], hash.Sum(nil))
	if sync {
		return image, file.Sync()
	}
	return image, nil
}

func cjsiReadLegacyFiles(ctx context.Context, path string) (cjsiLegacyFiles, error) {
	var files cjsiLegacyFiles
	for _, item := range []struct {
		suffix string
		image  *cjsiLayoutFile
	}{{"", &files.main}, {"-wal", &files.wal}} {
		got, err := cjsiReadLayoutFile(ctx, path+item.suffix, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return cjsiLegacyFiles{}, err
		}
		if item.suffix != "-wal" || got.size > 0 {
			*item.image = got
		}
	}
	if !files.valid() {
		return cjsiLegacyFiles{}, errCJSILayoutInvalid
	}
	return files, cjsiCheckLegacySidecars(path, files.main.present())
}

func cjsiCheckLegacySidecars(path string, hasMain bool) error {
	for _, suffix := range cjsiSidecarSuffixes {
		stat, err := privateServingPath(path+suffix, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		// A nonempty rollback journal can represent recovery still owed
		// to the legacy database. Never let it replay against the marker.
		if stat.Nlink != 1 || (suffix == "-journal" || !hasMain) && stat.Size != 0 {
			return errCJSILayoutInvalid
		}
	}
	return nil
}

func cjsiCheckEmptyLayoutSidecar(path string) error {
	stat, err := privateServingPath(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || stat.Size != 0 || stat.Nlink != 1 {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	return nil
}

func cjsiVerifyLayoutFile(ctx context.Context, path string, want cjsiLayoutFile) error {
	got, err := cjsiReadLayoutFile(ctx, path, false)
	if err != nil {
		return err
	}
	if got != want {
		return errCJSILayoutInvalid
	}
	return cjsiRequireAbsentSidecars(path)
}

func cjsiRemoveLayoutFile(path string) error {
	if _, err := privateServingPath(path, false); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Remove(path)
}

func cjsiRemoveLayoutSidecars(path string, emptyOnly bool) error {
	for _, suffix := range cjsiSidecarSuffixes {
		stat, err := privateServingPath(path+suffix, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if stat.Nlink != 1 || emptyOnly && stat.Size != 0 {
			return errCJSILayoutInvalid
		}
		if err := os.Remove(path + suffix); err != nil {
			return err
		}
	}
	return nil
}

func cjsiRemoveIndexFiles(path string) error {
	if err := cjsiRemoveLayoutSidecars(path, false); err != nil {
		return err
	}
	return cjsiRemoveLayoutFile(path)
}

func cjsiRequireAbsentSidecars(path string) error {
	for _, suffix := range cjsiSidecarSuffixes {
		if err := cjsiRequireAbsent(path + suffix); err != nil {
			return err
		}
	}
	return nil
}

func cjsiRequireCleanIndexPath(path string) error {
	if err := cjsiRequireAbsent(path); err != nil {
		return err
	}
	return cjsiRequireAbsentSidecars(path)
}

func cjsiRequireAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errCJSILayoutInvalid
}

func cjsiWriteFence(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	marker := cjsiFenceBytes()
	if _, err := file.Write(marker[:]); err != nil {
		return errors.Join(err, file.Close())
	}
	return errors.Join(file.Sync(), file.Close())
}

func cjsiSQLiteURI(path, mode string) string {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode + "&_pragma=busy_timeout(20)"}
	return u.String()
}

func cjsiProbeFence(ctx context.Context, path string) error {
	for _, probe := range []struct{ mode, statement string }{
		{"ro", "SELECT count(*) FROM sqlite_master"},
		{"rw", "PRAGMA journal_mode=WAL"},
	} {
		db, err := sql.Open("sqlite", cjsiSQLiteURI(path, probe.mode))
		if err != nil {
			return err
		}
		var value string
		probeErr := db.QueryRowContext(ctx, probe.statement).Scan(&value)
		if err := db.Close(); err != nil {
			return err
		}
		var driverErr *sqlite.Error
		if !errors.As(probeErr, &driverErr) || driverErr.Code()&0xff != sqlite3.SQLITE_NOTADB {
			return errors.Join(errCJSIFenceInvalid, probeErr)
		}
	}
	return nil
}

// This uses the same online backup API as backupDatabase. Reserve just the
// main file and pass an escaped no-create URI so staging does not depend on
// plain SQLite filenames or pre-created sidecars.
func cjsiBackupLayoutIndex(ctx context.Context, source, destination string) error {
	dest, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := dest.Close(); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", cjsiSQLiteURI(source, "ro"))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(driverConn any) error {
		backup, err := driverConn.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		}).NewBackup(cjsiSQLiteURI(destination, "rw"))
		if err != nil {
			return err
		}
		more, err := backup.Step(-1)
		if err != nil || more {
			return errors.Join(errCJSILayoutInvalid, err, backup.Finish())
		}
		dest, err := backup.Commit()
		if err != nil {
			return err
		}
		return dest.Close()
	})
}

func cjsiCheckpointLegacy(ctx context.Context, path string) (err error) {
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw"))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var busy, logFrames, checkpointed int
	if err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return ErrStateBusy
	}
	if logFrames != checkpointed || logFrames > 0 {
		return errCJSILayoutInvalid
	}
	return nil
}

func cjsiCheckSnapshot(ctx context.Context, path string) error {
	return cjsiCheckStandaloneIndex(ctx, path, func(db *sql.DB) error {
		var tables int
		err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master
			WHERE type='table' AND name IN ('session_sources','session_metadata')`).Scan(&tables)
		if err != nil {
			return err
		}
		if tables != 2 {
			return errCJSILayoutInvalid
		}
		return nil
	})
}

func cjsiSealPrivateFile(ctx context.Context, path string, epoch int64, seal cjsiIndexSeal, readSeal cjsiReadIndexSeal) error {
	return cjsiCheckStandaloneIndex(ctx, path, func(db *sql.DB) error {
		return verifyCJSIIndexDB(ctx, db, cjsiLayoutSelection{epoch: epoch, seal: seal}, readSeal)
	})
}

func cjsiCheckStandaloneIndex(ctx context.Context, path string, verify func(*sql.DB) error) error {
	for _, suffix := range [...]string{"", "-wal", "-shm", "-journal"} {
		stat, err := privateServingPath(path+suffix, false)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || stat.Nlink != 1 {
			return errors.Join(errCJSILayoutInvalid, err)
		}
	}
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw"))
	if err != nil {
		return err
	}
	// Close first so SQLite has finished with any staging sidecars.
	if err := errors.Join(cjsiVerifyStandaloneDB(ctx, db, verify), db.Close()); err != nil {
		return err
	}
	return cjsiRemoveLayoutSidecars(path, true)
}

func cjsiVerifyStandaloneDB(ctx context.Context, db *sql.DB, verify func(*sql.DB) error) error {
	var mode, result string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&mode); err != nil {
		return err
	}
	if mode != "delete" {
		return errCJSILayoutInvalid
	}
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errCJSILayoutInvalid
	}
	return verify(db)
}
