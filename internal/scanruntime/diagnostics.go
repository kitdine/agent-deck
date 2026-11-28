package scanruntime

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

const (
	// The worker's private failure log lives beside the receipt journal.
	// Public results keep only the classified code; this log adds the stage
	// and an enumerable description of the cause so a failed round can be
	// traced by its round id. It never holds error text: messages can carry
	// source paths and transcript values in forms no redaction rule bounds.
	diagnosticLogFilename = "scan-worker-diagnostics.log"
	diagnosticLogMaxBytes = 256 << 10
	diagnosticTypeLimit   = 16
)

// stageError names the serving step whose error ended a round. It changes
// neither errors.Is nor errors.As, so failure classification is unaffected.
type stageError struct {
	stage string
	err   error
}

func (e *stageError) Error() string { return e.stage + ": " + e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

func withStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &stageError{stage: stage, err: err}
}

type diagnosticLog struct {
	mu        sync.Mutex
	stateRoot string
	path      string
	maxBytes  int64
	now       func() time.Time
}

func newDiagnosticLog(stateRoot string) *diagnosticLog {
	return &diagnosticLog{stateRoot: stateRoot, path: filepath.Join(stateRoot, diagnosticLogFilename), maxBytes: diagnosticLogMaxBytes, now: time.Now}
}

// diagnosticEntry holds only enumerable values: identifiers this worker
// generated, stage names from the code, the public classification, Go type
// names, numeric codes and the names derived from them, and the operation
// names of system calls.
type diagnosticEntry struct {
	At                string              `json:"at"`
	RoundID           string              `json:"round_id"`
	InventoryID       string              `json:"inventory_id,omitempty"`
	Domain            string              `json:"domain,omitempty"`
	Stage             string              `json:"stage"`
	ErrorCode         string              `json:"error_code"`
	ErrorTypes        []string            `json:"error_types"`
	Sentinels         []string            `json:"sentinels,omitempty"`
	SQLiteCode        int                 `json:"sqlite_code,omitempty"`
	SQLiteCodeName    string              `json:"sqlite_code_name,omitempty"`
	SQLitePrimaryCode int                 `json:"sqlite_primary_code,omitempty"`
	SQLitePrimaryName string              `json:"sqlite_primary_name,omitempty"`
	SystemErrors      []diagnosticSyscall `json:"system_errors,omitempty"`
}

// diagnosticSyscall describes a file system or system call error by its
// operation and errno, never by the path it named.
type diagnosticSyscall struct {
	Type  string `json:"type"`
	Op    string `json:"op"`
	Errno string `json:"errno,omitempty"`
}

// record appends one failure of a round. Diagnostics are best effort: a log
// that cannot be written never changes the round's outcome.
func (l *diagnosticLog) record(roundID, inventoryID, domain string, err error) error {
	if l == nil || err == nil {
		return nil
	}
	if ownershipErr := platform.RevalidateOwnership(l.stateRoot); ownershipErr != nil {
		return ownershipErr
	}
	entry := describeFailure(err)
	entry.At = l.now().UTC().Format(time.RFC3339Nano)
	entry.RoundID = diagnosticRoundID(roundID)
	entry.InventoryID = diagnosticInventoryID(inventoryID)
	entry.Domain = domain
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if encodeErr := encoder.Encode(entry); encodeErr != nil {
		return encodeErr
	}
	return l.append(line.Bytes())
}

// invalidDiagnosticID replaces an identifier that newID could not have
// produced. A recovered round takes its id from the private receipt journal,
// which a damaged or rewritten file can fill with any text.
const invalidDiagnosticID = "invalid"

// generatedID reports whether id has a shape newID produces: 24 lowercase hex
// digits, or the decimal clock fallback.
func generatedID(id string) bool {
	if len(id) == 24 {
		for _, c := range id {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
		return true
	}
	// The fallback is fmt.Sprintf("%d", time.Now().UnixNano()): a canonical
	// non-negative int64, so at most 19 digits and no leading zero.
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n >= 0 && strconv.FormatInt(n, 10) == id
}

func diagnosticRoundID(id string) string {
	if generatedID(id) {
		return id
	}
	return invalidDiagnosticID
}

// diagnosticInventoryID accepts what roundInventoryID produces: the round id,
// or the round id and a new attempt id joined by a dot.
func diagnosticInventoryID(id string) string {
	if id == "" {
		return ""
	}
	round, attempt, replayed := strings.Cut(id, ".")
	if generatedID(round) && (!replayed || generatedID(attempt)) {
		return id
	}
	return invalidDiagnosticID
}

func (l *diagnosticLog) append(line []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	info, err := os.Lstat(l.path)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return errors.New("scan worker diagnostic log is not a regular private file")
	case err == nil && info.Size()+int64(len(line)) > l.maxBytes:
		if err = restrictMode(l.path); err != nil {
			return err
		}
		if err = os.Rename(l.path, l.path+".1"); err != nil {
			return err
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	file, err := os.OpenFile(l.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|unix.O_NOFOLLOW, platform.FileMode)
	if err != nil {
		return err
	}
	if err = file.Chmod(platform.FileMode); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(line); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// restrictMode tightens a log generation before it is rotated away, so a file
// left with a wider mode never survives as the previous generation.
func restrictMode(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	if err = file.Chmod(platform.FileMode); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func describeFailure(err error) diagnosticEntry {
	entry := diagnosticEntry{ErrorCode: failureCode(err)}
	var stages []string
	walkErrors(err, func(node error) {
		if len(entry.ErrorTypes) < diagnosticTypeLimit {
			entry.ErrorTypes = append(entry.ErrorTypes, fmt.Sprintf("%T", node))
		}
		switch typed := node.(type) {
		case *stageError:
			stages = append(stages, typed.stage)
		case *fs.PathError:
			entry.SystemErrors = append(entry.SystemErrors, describeSyscall("*fs.PathError", typed.Op, typed.Err))
		case *os.LinkError:
			entry.SystemErrors = append(entry.SystemErrors, describeSyscall("*os.LinkError", typed.Op, typed.Err))
		case *os.SyscallError:
			entry.SystemErrors = append(entry.SystemErrors, describeSyscall("*os.SyscallError", typed.Syscall, typed.Err))
		}
	})
	entry.Stage = strings.Join(stages, "; ")
	for _, sentinel := range diagnosticSentinels {
		if errors.Is(err, sentinel.err) {
			entry.Sentinels = append(entry.Sentinels, sentinel.name)
		}
	}
	var native *sqlite.Error
	if errors.As(err, &native) {
		code := native.Code()
		entry.SQLiteCode = code
		entry.SQLiteCodeName = sqliteCodeNames[code]
		entry.SQLitePrimaryCode = code & 0xff
		entry.SQLitePrimaryName = sqliteCodeNames[code&0xff]
	}
	return entry
}

func walkErrors(err error, visit func(error)) {
	if err == nil {
		return
	}
	visit(err)
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range wrapped.Unwrap() {
			walkErrors(child, visit)
		}
	case interface{ Unwrap() error }:
		walkErrors(wrapped.Unwrap(), visit)
	}
}

// describeSyscall keeps an operation name only when it is one of the fixed
// names the standard library uses; anything else is reported as "other".
func describeSyscall(kind, op string, cause error) diagnosticSyscall {
	described := diagnosticSyscall{Type: kind, Op: "other"}
	if diagnosticOperations[op] {
		described.Op = op
	}
	var errno syscall.Errno
	if errors.As(cause, &errno) {
		if name := unix.ErrnoName(errno); name != "" {
			described.Errno = name
		} else {
			described.Errno = fmt.Sprintf("errno_%d", int(errno))
		}
	}
	return described
}

var diagnosticOperations = map[string]bool{
	"accept": true, "bind": true, "chdir": true, "chmod": true, "chown": true, "chtimes": true,
	"close": true, "connect": true, "dup": true, "fchmod": true, "fchown": true, "fcntl": true,
	"flock": true, "fstat": true, "fsync": true, "ftruncate": true, "getwd": true, "kill": true,
	"lchown": true, "link": true, "listen": true, "lstat": true, "mkdir": true, "mkdirat": true,
	"mkdirtemp": true, "mmap": true, "open": true, "openat": true, "pipe": true, "read": true,
	"readat": true, "readdir": true, "readdirent": true, "readfrom": true, "readlink": true,
	"remove": true, "removeall": true, "rename": true, "seek": true, "setsockopt": true,
	"socket": true, "stat": true, "statat": true, "symlink": true, "sync": true, "truncate": true,
	"unlinkat": true, "wait": true, "write": true, "writeat": true, "writeto": true, "createtemp": true,
}

// diagnosticSentinels are the named errors whose identity locates a failure
// without its text: cancellation, deadlines, missing or forbidden files, and
// the store, ingestion and serving conditions a round can end on.
var diagnosticSentinels = []struct {
	name string
	err  error
}{
	{"context.Canceled", context.Canceled},
	{"context.DeadlineExceeded", context.DeadlineExceeded},
	{"fs.ErrNotExist", fs.ErrNotExist},
	{"fs.ErrPermission", fs.ErrPermission},
	{"fs.ErrExist", fs.ErrExist},
	{"os.ErrDeadlineExceeded", os.ErrDeadlineExceeded},
	{"io.EOF", io.EOF},
	{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF},
	{"sql.ErrNoRows", sql.ErrNoRows},
	{"sql.ErrTxDone", sql.ErrTxDone},
	{"sql.ErrConnDone", sql.ErrConnDone},
	{"ingest.ErrSourceChanged", ingest.ErrSourceChanged},
	{"ingest.ErrPlanIncomplete", ingest.ErrPlanIncomplete},
	{"ingest.ErrPlanMismatch", ingest.ErrPlanMismatch},
	{"store.ErrTelemetrySourceIdentityPending", store.ErrTelemetrySourceIdentityPending},
	{"store.ErrCoverageIncomplete", store.ErrCoverageIncomplete},
	{"store.ErrUsagePartitionChanged", store.ErrUsagePartitionChanged},
	{"desktop.ErrServingChanged", desktop.ErrServingChanged},
	{"desktop.ErrDerivedSnapshotCacheChanged", desktop.ErrDerivedSnapshotCacheChanged},
}

// sqliteCodeNames names SQLite's primary and extended result codes, so the
// log can say SQLITE_BUSY_SNAPSHOT without the driver's message.
var sqliteCodeNames = map[int]string{
	sqlite3.SQLITE_ERROR:                   "SQLITE_ERROR",
	sqlite3.SQLITE_INTERNAL:                "SQLITE_INTERNAL",
	sqlite3.SQLITE_PERM:                    "SQLITE_PERM",
	sqlite3.SQLITE_ABORT:                   "SQLITE_ABORT",
	sqlite3.SQLITE_BUSY:                    "SQLITE_BUSY",
	sqlite3.SQLITE_LOCKED:                  "SQLITE_LOCKED",
	sqlite3.SQLITE_NOMEM:                   "SQLITE_NOMEM",
	sqlite3.SQLITE_READONLY:                "SQLITE_READONLY",
	sqlite3.SQLITE_INTERRUPT:               "SQLITE_INTERRUPT",
	sqlite3.SQLITE_IOERR:                   "SQLITE_IOERR",
	sqlite3.SQLITE_CORRUPT:                 "SQLITE_CORRUPT",
	sqlite3.SQLITE_NOTFOUND:                "SQLITE_NOTFOUND",
	sqlite3.SQLITE_FULL:                    "SQLITE_FULL",
	sqlite3.SQLITE_CANTOPEN:                "SQLITE_CANTOPEN",
	sqlite3.SQLITE_PROTOCOL:                "SQLITE_PROTOCOL",
	sqlite3.SQLITE_EMPTY:                   "SQLITE_EMPTY",
	sqlite3.SQLITE_SCHEMA:                  "SQLITE_SCHEMA",
	sqlite3.SQLITE_TOOBIG:                  "SQLITE_TOOBIG",
	sqlite3.SQLITE_CONSTRAINT:              "SQLITE_CONSTRAINT",
	sqlite3.SQLITE_MISMATCH:                "SQLITE_MISMATCH",
	sqlite3.SQLITE_MISUSE:                  "SQLITE_MISUSE",
	sqlite3.SQLITE_NOLFS:                   "SQLITE_NOLFS",
	sqlite3.SQLITE_AUTH:                    "SQLITE_AUTH",
	sqlite3.SQLITE_FORMAT:                  "SQLITE_FORMAT",
	sqlite3.SQLITE_RANGE:                   "SQLITE_RANGE",
	sqlite3.SQLITE_NOTADB:                  "SQLITE_NOTADB",
	sqlite3.SQLITE_NOTICE:                  "SQLITE_NOTICE",
	sqlite3.SQLITE_WARNING:                 "SQLITE_WARNING",
	sqlite3.SQLITE_ROW:                     "SQLITE_ROW",
	sqlite3.SQLITE_DONE:                    "SQLITE_DONE",
	sqlite3.SQLITE_ERROR_MISSING_COLLSEQ:   "SQLITE_ERROR_MISSING_COLLSEQ",
	sqlite3.SQLITE_BUSY_RECOVERY:           "SQLITE_BUSY_RECOVERY",
	sqlite3.SQLITE_LOCKED_SHAREDCACHE:      "SQLITE_LOCKED_SHAREDCACHE",
	sqlite3.SQLITE_READONLY_RECOVERY:       "SQLITE_READONLY_RECOVERY",
	sqlite3.SQLITE_IOERR_READ:              "SQLITE_IOERR_READ",
	sqlite3.SQLITE_CORRUPT_VTAB:            "SQLITE_CORRUPT_VTAB",
	sqlite3.SQLITE_CANTOPEN_NOTEMPDIR:      "SQLITE_CANTOPEN_NOTEMPDIR",
	sqlite3.SQLITE_CONSTRAINT_CHECK:        "SQLITE_CONSTRAINT_CHECK",
	sqlite3.SQLITE_AUTH_USER:               "SQLITE_AUTH_USER",
	sqlite3.SQLITE_NOTICE_RECOVER_WAL:      "SQLITE_NOTICE_RECOVER_WAL",
	sqlite3.SQLITE_WARNING_AUTOINDEX:       "SQLITE_WARNING_AUTOINDEX",
	sqlite3.SQLITE_ERROR_RETRY:             "SQLITE_ERROR_RETRY",
	sqlite3.SQLITE_ABORT_ROLLBACK:          "SQLITE_ABORT_ROLLBACK",
	sqlite3.SQLITE_BUSY_SNAPSHOT:           "SQLITE_BUSY_SNAPSHOT",
	sqlite3.SQLITE_LOCKED_VTAB:             "SQLITE_LOCKED_VTAB",
	sqlite3.SQLITE_READONLY_CANTLOCK:       "SQLITE_READONLY_CANTLOCK",
	sqlite3.SQLITE_IOERR_SHORT_READ:        "SQLITE_IOERR_SHORT_READ",
	sqlite3.SQLITE_CORRUPT_SEQUENCE:        "SQLITE_CORRUPT_SEQUENCE",
	sqlite3.SQLITE_CANTOPEN_ISDIR:          "SQLITE_CANTOPEN_ISDIR",
	sqlite3.SQLITE_CONSTRAINT_COMMITHOOK:   "SQLITE_CONSTRAINT_COMMITHOOK",
	sqlite3.SQLITE_NOTICE_RECOVER_ROLLBACK: "SQLITE_NOTICE_RECOVER_ROLLBACK",
	sqlite3.SQLITE_ERROR_SNAPSHOT:          "SQLITE_ERROR_SNAPSHOT",
	sqlite3.SQLITE_BUSY_TIMEOUT:            "SQLITE_BUSY_TIMEOUT",
	sqlite3.SQLITE_READONLY_ROLLBACK:       "SQLITE_READONLY_ROLLBACK",
	sqlite3.SQLITE_IOERR_WRITE:             "SQLITE_IOERR_WRITE",
	sqlite3.SQLITE_CORRUPT_INDEX:           "SQLITE_CORRUPT_INDEX",
	sqlite3.SQLITE_CANTOPEN_FULLPATH:       "SQLITE_CANTOPEN_FULLPATH",
	sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:   "SQLITE_CONSTRAINT_FOREIGNKEY",
	sqlite3.SQLITE_NOTICE_RBU:              "SQLITE_NOTICE_RBU",
	sqlite3.SQLITE_ERROR_RESERVESIZE:       "SQLITE_ERROR_RESERVESIZE",
	sqlite3.SQLITE_READONLY_DBMOVED:        "SQLITE_READONLY_DBMOVED",
	sqlite3.SQLITE_IOERR_FSYNC:             "SQLITE_IOERR_FSYNC",
	sqlite3.SQLITE_CANTOPEN_CONVPATH:       "SQLITE_CANTOPEN_CONVPATH",
	sqlite3.SQLITE_CONSTRAINT_FUNCTION:     "SQLITE_CONSTRAINT_FUNCTION",
	sqlite3.SQLITE_ERROR_KEY:               "SQLITE_ERROR_KEY",
	sqlite3.SQLITE_READONLY_CANTINIT:       "SQLITE_READONLY_CANTINIT",
	sqlite3.SQLITE_IOERR_DIR_FSYNC:         "SQLITE_IOERR_DIR_FSYNC",
	sqlite3.SQLITE_CANTOPEN_DIRTYWAL:       "SQLITE_CANTOPEN_DIRTYWAL",
	sqlite3.SQLITE_CONSTRAINT_NOTNULL:      "SQLITE_CONSTRAINT_NOTNULL",
	sqlite3.SQLITE_ERROR_UNABLE:            "SQLITE_ERROR_UNABLE",
	sqlite3.SQLITE_READONLY_DIRECTORY:      "SQLITE_READONLY_DIRECTORY",
	sqlite3.SQLITE_IOERR_TRUNCATE:          "SQLITE_IOERR_TRUNCATE",
	sqlite3.SQLITE_CANTOPEN_SYMLINK:        "SQLITE_CANTOPEN_SYMLINK",
	sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:   "SQLITE_CONSTRAINT_PRIMARYKEY",
	sqlite3.SQLITE_IOERR_FSTAT:             "SQLITE_IOERR_FSTAT",
	sqlite3.SQLITE_CONSTRAINT_TRIGGER:      "SQLITE_CONSTRAINT_TRIGGER",
	sqlite3.SQLITE_IOERR_UNLOCK:            "SQLITE_IOERR_UNLOCK",
	sqlite3.SQLITE_CONSTRAINT_UNIQUE:       "SQLITE_CONSTRAINT_UNIQUE",
	sqlite3.SQLITE_IOERR_RDLOCK:            "SQLITE_IOERR_RDLOCK",
	sqlite3.SQLITE_CONSTRAINT_VTAB:         "SQLITE_CONSTRAINT_VTAB",
	sqlite3.SQLITE_IOERR_DELETE:            "SQLITE_IOERR_DELETE",
	sqlite3.SQLITE_CONSTRAINT_ROWID:        "SQLITE_CONSTRAINT_ROWID",
	sqlite3.SQLITE_IOERR_BLOCKED:           "SQLITE_IOERR_BLOCKED",
	sqlite3.SQLITE_CONSTRAINT_PINNED:       "SQLITE_CONSTRAINT_PINNED",
	sqlite3.SQLITE_IOERR_NOMEM:             "SQLITE_IOERR_NOMEM",
	sqlite3.SQLITE_CONSTRAINT_DATATYPE:     "SQLITE_CONSTRAINT_DATATYPE",
	sqlite3.SQLITE_IOERR_ACCESS:            "SQLITE_IOERR_ACCESS",
	sqlite3.SQLITE_IOERR_CHECKRESERVEDLOCK: "SQLITE_IOERR_CHECKRESERVEDLOCK",
	sqlite3.SQLITE_IOERR_LOCK:              "SQLITE_IOERR_LOCK",
	sqlite3.SQLITE_IOERR_CLOSE:             "SQLITE_IOERR_CLOSE",
	sqlite3.SQLITE_IOERR_DIR_CLOSE:         "SQLITE_IOERR_DIR_CLOSE",
	sqlite3.SQLITE_IOERR_SHMOPEN:           "SQLITE_IOERR_SHMOPEN",
	sqlite3.SQLITE_IOERR_SHMSIZE:           "SQLITE_IOERR_SHMSIZE",
	sqlite3.SQLITE_IOERR_SHMLOCK:           "SQLITE_IOERR_SHMLOCK",
	sqlite3.SQLITE_IOERR_SHMMAP:            "SQLITE_IOERR_SHMMAP",
	sqlite3.SQLITE_IOERR_SEEK:              "SQLITE_IOERR_SEEK",
	sqlite3.SQLITE_IOERR_DELETE_NOENT:      "SQLITE_IOERR_DELETE_NOENT",
	sqlite3.SQLITE_IOERR_MMAP:              "SQLITE_IOERR_MMAP",
	sqlite3.SQLITE_IOERR_GETTEMPPATH:       "SQLITE_IOERR_GETTEMPPATH",
	sqlite3.SQLITE_IOERR_CONVPATH:          "SQLITE_IOERR_CONVPATH",
	sqlite3.SQLITE_IOERR_VNODE:             "SQLITE_IOERR_VNODE",
	sqlite3.SQLITE_IOERR_AUTH:              "SQLITE_IOERR_AUTH",
	sqlite3.SQLITE_IOERR_BEGIN_ATOMIC:      "SQLITE_IOERR_BEGIN_ATOMIC",
	sqlite3.SQLITE_IOERR_COMMIT_ATOMIC:     "SQLITE_IOERR_COMMIT_ATOMIC",
	sqlite3.SQLITE_IOERR_ROLLBACK_ATOMIC:   "SQLITE_IOERR_ROLLBACK_ATOMIC",
	sqlite3.SQLITE_IOERR_DATA:              "SQLITE_IOERR_DATA",
	sqlite3.SQLITE_IOERR_CORRUPTFS:         "SQLITE_IOERR_CORRUPTFS",
	sqlite3.SQLITE_IOERR_IN_PAGE:           "SQLITE_IOERR_IN_PAGE",
	sqlite3.SQLITE_IOERR_BADKEY:            "SQLITE_IOERR_BADKEY",
	sqlite3.SQLITE_IOERR_CODEC:             "SQLITE_IOERR_CODEC",
}
