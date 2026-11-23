package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type cjsiLayoutPhase uint8

const (
	cjsiLayoutInvalid cjsiLayoutPhase = iota
	cjsiLayoutNeverBuilt
	cjsiLayoutPurged
	cjsiLayoutStaging
	cjsiLayoutSelected
)

type cjsiIndexLocator uint8

const (
	cjsiIndexNone cjsiIndexLocator = iota
	cjsiIndexPrivate
)

const (
	cjsiLegacyIndexName  = "sessions.sqlite3"
	cjsiPrivateIndexName = "session-index-v1.sqlite3"
	cjsiFenceHeader      = "AgentDeck private session index fence\n"
	// Above one minimum SQLite page and not a regular SQLite page multiple.
	cjsiFenceSize = 4097
)

var (
	errCJSILayoutInvalid = errors.New("invalid Codex session index layout")
	errCJSIFenceInvalid  = errors.New("invalid legacy session index fence")
)

type cjsiFileIdentity struct{ device, inode uint64 }

func cjsiIdentity(stat *syscall.Stat_t) cjsiFileIdentity {
	return cjsiFileIdentity{uint64(stat.Dev), uint64(stat.Ino)}
}

type cjsiIndexSeal struct {
	version int
	digest  [sha256.Size]byte
}

func (s cjsiIndexSeal) valid() bool {
	return s.version > 0 && s.digest != ([sha256.Size]byte{})
}

// Only a committed selection carries a locator and expected file metadata.
// Staging and purge history belong to the coordinator, outside this snapshot.
type cjsiLayoutSelection struct {
	phase    cjsiLayoutPhase
	locator  cjsiIndexLocator
	identity cjsiFileIdentity
	epoch    int64
	seal     cjsiIndexSeal
}

type cjsiLayoutResolution struct {
	phase cjsiLayoutPhase
	path  string // Set only for a verified selection.
}

// cjsiReadIndexSeal reads the private version and seal using the resolver's
// read transaction. Its schema adapter is supplied by the numbered migration.
type cjsiReadIndexSeal func(context.Context, *sql.Tx) (cjsiIndexSeal, error)

// resolveCJSILayout verifies a coordinator snapshot under an already guarded
// physical root. Staging never looks for a candidate. This check does not pin
// a future open: routing must recheck the selection and the opened identity
// through its read or write boundary.
func resolveCJSILayout(ctx context.Context, root string, selection cjsiLayoutSelection, readSeal cjsiReadIndexSeal) (cjsiLayoutResolution, error) {
	if err := ctx.Err(); err != nil {
		return cjsiLayoutResolution{}, err
	}
	if !filepath.IsAbs(root) {
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	rootStat, err := privateServingPath(root, true)
	if err != nil {
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	switch selection.phase {
	case cjsiLayoutNeverBuilt, cjsiLayoutPurged, cjsiLayoutStaging:
		if selection != (cjsiLayoutSelection{phase: selection.phase}) {
			return cjsiLayoutResolution{}, errCJSILayoutInvalid
		}
		// The physical fence may still be awaiting installation.
		if selection.phase != cjsiLayoutStaging {
			if _, err := validateCJSIFence(root); err != nil {
				return cjsiLayoutResolution{}, err
			}
		}
		return cjsiLayoutResolution{phase: selection.phase}, nil
	case cjsiLayoutSelected:
		if selection.locator != cjsiIndexPrivate || selection.identity.inode == 0 ||
			selection.epoch <= 0 || !selection.seal.valid() || readSeal == nil {
			return cjsiLayoutResolution{}, errCJSILayoutInvalid
		}
	default:
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}

	fence, err := validateCJSIFence(root)
	if err != nil {
		return cjsiLayoutResolution{}, err
	}
	path := filepath.Join(root, cjsiPrivateIndexName)
	indexStat, err := privateServingPath(path, false)
	if err != nil || cjsiIdentity(indexStat) != selection.identity {
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	if err := verifyCJSIIndex(ctx, path, selection, readSeal); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return cjsiLayoutResolution{}, ctxErr
		}
		var driverErr *sqlite.Error
		if errors.As(err, &driverErr) {
			switch driverErr.Code() & 0xff {
			case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
				return cjsiLayoutResolution{}, ErrStateBusy
			}
		}
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	currentFence, err := validateCJSIFence(root)
	if err != nil || currentFence != fence {
		return cjsiLayoutResolution{}, errCJSIFenceInvalid
	}
	currentRoot, err := privateServingPath(root, true)
	if err != nil || cjsiIdentity(currentRoot) != cjsiIdentity(rootStat) {
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	currentIndex, err := privateServingPath(path, false)
	if err != nil || cjsiIdentity(currentIndex) != selection.identity {
		return cjsiLayoutResolution{}, errCJSILayoutInvalid
	}
	if err := ctx.Err(); err != nil {
		return cjsiLayoutResolution{}, err
	}
	return cjsiLayoutResolution{phase: cjsiLayoutSelected, path: path}, nil
}

func verifyCJSIIndex(ctx context.Context, path string, selection cjsiLayoutSelection, readSeal cjsiReadIndexSeal) error {
	db, release, err := acquirePrivateVerifySQL(ctx, path, "identity")
	if err != nil {
		return err
	}
	defer release()
	return verifyCJSIIndexDB(ctx, db, selection, readSeal)
}

func verifyCJSIIndexDB(ctx context.Context, db *sql.DB, selection cjsiLayoutSelection, readSeal cjsiReadIndexSeal) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var epoch int64
	if err := tx.QueryRowContext(ctx, `SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&epoch); err != nil {
		return err
	}
	seal, err := readSeal(ctx, tx)
	if err != nil {
		return err
	}
	if epoch != selection.epoch || seal != selection.seal {
		return errCJSILayoutInvalid
	}
	return tx.Commit()
}

func cjsiFenceBytes() [cjsiFenceSize]byte {
	var marker [cjsiFenceSize]byte
	copy(marker[:], cjsiFenceHeader)
	return marker
}

func validateCJSIFence(root string) (cjsiFileIdentity, error) {
	path := filepath.Join(root, cjsiLegacyIndexName)
	before, err := privateServingPath(path, false)
	// Refuse a normal SQLite file without opening another descriptor to it.
	if err != nil || before.Size != cjsiFenceSize {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	file := os.NewFile(uintptr(fd), cjsiLegacyIndexName)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	opened, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || opened.Mode != before.Mode ||
		opened.Uid != before.Uid || opened.Size != before.Size || cjsiIdentity(opened) != cjsiIdentity(before) {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	want := cjsiFenceBytes()
	var content [cjsiFenceSize + 1]byte
	n, readErr := io.ReadFull(file, content[:])
	if readErr != io.ErrUnexpectedEOF || n != cjsiFenceSize || !bytes.Equal(content[:n], want[:]) {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	after, err := privateServingPath(path, false)
	if err != nil || after.Size != before.Size || cjsiIdentity(after) != cjsiIdentity(before) {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	return cjsiIdentity(before), nil
}
