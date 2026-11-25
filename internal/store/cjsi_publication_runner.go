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
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

var errCJSIOpenUnconfirmed = errors.New("CJSI final commit requires receipt reconciliation")

// On success transfer a read-only fd for the exact immutable reserved entry
// relative to the supplied directory. The adapter binds name to the reserved
// identity, and supplies the actual digest/size from durable I/O. For a new
// file use WriteCJSIServingSlot, which owns the root's in-flight
// descriptor until the kernel operation returns, even after waiter expiry.
// On recovery authenticate the original reservation and inode, verify bytes
// and establish file + directory durability before adopting an orphan. A
// registered seal permits only its existing qualified file, never recreation.
type cjsiRunnerSlot struct {
	file     *os.File
	name     string
	identity [16]byte
	digest   [32]byte
	size     int64
}

type cjsiRunnerIO func(context.Context, *os.File, os.FileInfo, [16]byte, []byte, bool) (cjsiRunnerSlot, error)

// Requalify independently on EACH writer admission, before opening the
// SQLite transaction. Pin the same-session candidate, source proofs and slot
// lifecycle under writer fences. current performs only bounded core checks
// from those proofs inside that transaction. release drops all phase-local
// resources after commit/rollback (and any exact final confirmation).
type cjsiRunnerProof struct {
	current cjsiSelectSlotAdmission
	target  cjsiOpenTarget
	delta   cjsiOpenDelta
	release func() error
}

type cjsiRunnerQualify func(context.Context, *os.File, *os.File) (cjsiRunnerProof, error)

type cjsiFrozenRunner struct {
	admission cjsiSlotAdmission
	coreID    cjsiFileIdentity // pinned/qualified physical core at preparation
	slotIO    cjsiRunnerIO
	qualify   cjsiRunnerQualify
}

// Invoke only AFTER releasing scan/maintenance, state and serving locks,
// core/index read snapshots and legacy directory leases. In particular a
// caller inside scanruntime.Serve cannot borrow its lifetime scan.lock here.
// Caller owns the immutable bytes until return. No store worker is spawned.
func (r cjsiFrozenRunner) run(ctx context.Context, contents []byte) error {
	return r.runWithCommit(ctx, contents, (*sql.Tx).Commit)
}

func (r cjsiFrozenRunner) runWithCommit(ctx context.Context, contents []byte, commitOpen func(*sql.Tx) error) (err error) {
	a := r.admission
	p := a.proposal.utc()
	r.admission.proposal = p
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.slotIO == nil || r.qualify == nil || commitOpen == nil || r.coreID.inode == 0 || !p.valid() || int64(len(contents)) != p.expected.size ||
		sha256.Sum256(contents) != p.expected.digest {
		return errCJSISlotAdmission
	}
	deadline := p.acceptUntil
	if p.expected.validUntil.Before(deadline) {
		deadline = p.expected.validUntil
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if err := r.waitErr(ctx); err != nil {
		return err
	}
	rootPath, e := platform.GuardStateRoot(a.rootPath)
	if e != nil {
		return e
	}
	if !filepath.IsAbs(a.rootPath) || rootPath != a.rootPath {
		return errCJSISlotAdmission
	}
	root, e := cjsiOpenPrivateLayoutPath(rootPath, true)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, e := root.Stat()
	if e != nil {
		return e
	}
	if cjsiIdentity(info.Sys().(*syscall.Stat_t)) != a.root {
		return errCJSISlotAdmission
	}
	sealed, e := r.committedInput(ctx)
	if e != nil {
		return e
	}
	if e := r.waitErr(ctx); e != nil {
		return e
	}
	slot, e := r.slotIO(ctx, root, info, p.expected.identity, contents, sealed)
	if slot.file != nil {
		defer func() { err = errors.Join(err, slot.file.Close()) }()
	}
	if e != nil {
		return e
	}
	if e := r.waitErr(ctx); e != nil {
		return e
	}
	if slot.file == nil || slot.identity != p.expected.identity || slot.digest != p.expected.digest || slot.size != p.expected.size {
		return errCJSISlotAdmission
	}
	before, e := slot.statAt(root)
	if e != nil {
		return e
	}
	h := sha256.New()
	n, e := io.Copy(h, io.NewSectionReader(slot.file, 0, slot.size+1))
	if e != nil {
		return e
	}
	if n != slot.size || !bytes.Equal(h.Sum(nil), slot.digest[:]) {
		return errCJSISlotAdmission
	}
	// Neither phase inherits a root lock, transaction, or owner selection
	// from the other. A refused open leaves the committed private seal.
	if e := r.writePhase(ctx, root, slot, before, nil); e != nil {
		return e
	}
	return r.writePhase(ctx, root, slot, before, commitOpen)
}

func (r cjsiFrozenRunner) waitErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	if t, ok := ctx.Deadline(); ok && !now.Before(t) {
		return context.DeadlineExceeded
	}
	if !r.admission.proposal.live(now) {
		return context.DeadlineExceeded
	}
	return nil
}

func (s cjsiRunnerSlot) statAt(root *os.File) (unix.Stat_t, error) {
	var fdStat, entry unix.Stat_t
	if !filepath.IsLocal(s.name) || filepath.Base(s.name) != s.name || s.name == "." {
		return fdStat, errCJSISlotAdmission
	}
	flags, err := unix.FcntlInt(s.file.Fd(), unix.F_GETFL, 0)
	if err != nil {
		return fdStat, err
	}
	if flags&unix.O_ACCMODE != unix.O_RDONLY {
		return fdStat, errCJSISlotAdmission
	}
	if err := unix.Fstat(int(s.file.Fd()), &fdStat); err != nil {
		return fdStat, err
	}
	if err := unix.Fstatat(int(root.Fd()), s.name, &entry, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fdStat, err
	}
	if fdStat.Dev != entry.Dev || fdStat.Ino != entry.Ino || fdStat.Mode&unix.S_IFMT != unix.S_IFREG ||
		fdStat.Mode&07777 != 0600 || fdStat.Uid != uint32(os.Getuid()) || fdStat.Nlink != 1 || fdStat.Size != s.size {
		return fdStat, errCJSISlotAdmission
	}
	return fdStat, nil
}

func (r cjsiFrozenRunner) location() error {
	a := r.admission
	if err := cjsiCheckInstallRoot(a.rootPath, a.root); err != nil {
		return err
	}
	st, err := privateServingPath(filepath.Join(a.rootPath, "agentdeck.sqlite3"), false)
	if err != nil {
		return err
	}
	if st.Nlink != 1 || cjsiIdentity(st) != r.coreID {
		return errCJSISlotAdmission
	}
	return cjsiCheckRecordedFence(a.rootPath, a.fence)
}

// Existing private core only. No Store bootstrap, migration, mkdir, or
// mode=rwc. Each underlying SQLite handle takes normal legacy admission.
// Close the entire preflight pool before entering the descriptor worker.
func (r cjsiFrozenRunner) core(ctx context.Context, writer bool) (*sql.DB, error) {
	if err := r.location(); err != nil {
		return nil, err
	}
	a := r.admission
	path := filepath.Join(a.rootPath, "agentdeck.sqlite3")
	query := "mode=ro&_pragma=query_only(1)"
	if writer {
		query = "mode=rw&_txlock=immediate"
	}
	db, err := openLegacySQL(path, legacyURI(path, query+"&_pragma=foreign_keys(1)&_pragma=synchronous(FULL)&_pragma=busy_timeout(0)"), "core", legacyRootID{a.root.device, a.root.inode})
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err == nil {
		err = r.location()
	}
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return db, nil
}

func (r cjsiFrozenRunner) committedInput(ctx context.Context) (sealed bool, err error) {
	db, err := r.core(ctx, false)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	a := r.admission
	p := a.proposal
	if err := checkCJSIPrepareBase(ctx, tx, a); err != nil {
		return false, err
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM main.cjsi_publication_attempts WHERE ticket=? AND attempt=?`, p.ticket, p.attempt).Scan(&state); err != nil {
		return false, err
	}
	switch state {
	case "draft":
		diff, session, err := readCJSIPrepareDraft(ctx, tx, p)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(diff, p.diff[:]) || !bytes.Equal(session, p.sessionSeal[:]) {
			return false, errCJSISlotAdmission
		}
		return false, nil
	case "sealed":
		o := cjsiCombinedOpen{admission: a, slotSeal: p.seal()}
		return true, o.checkAttempt(ctx, tx, "sealed")
	default:
		return false, errCJSISlotAdmission
	}
}

func (r cjsiFrozenRunner) writePhase(ctx context.Context, root *os.File, slot cjsiRunnerSlot, before unix.Stat_t, commitOpen func(*sql.Tx) error) (err error) {
	if e := r.waitErr(ctx); e != nil {
		return e
	}
	if e := r.location(); e != nil {
		return e
	}
	a := r.admission
	p := a.proposal
	// AcquireMaintenance's underlying project lock, without prepareStateRoot
	// (which can create a missing root). The established order is scan, state.
	scan, e := AcquireScanLock(ctx, a.rootPath, 0)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, scan.Release()) }()
	if e := r.checkLockRoot(scan); e != nil {
		return e
	}
	state, e := AcquireLock(ctx, a.rootPath, 0)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, state.Release()) }()
	if e := r.checkLockRoot(state); e != nil {
		return e
	}
	if e := r.location(); e != nil {
		return e
	}
	proof, e := r.qualify(ctx, root, slot.file)
	if proof.release != nil {
		defer func() { err = errors.Join(err, proof.release()) }()
	}
	if e != nil {
		return e
	}
	if proof.release == nil || proof.current == nil {
		return errCJSISlotAdmission
	}
	now, e := slot.statAt(root)
	if e != nil {
		return e
	}
	if now.Dev != before.Dev || now.Ino != before.Ino || now.Mtim != before.Mtim || now.Ctim != before.Ctim {
		return errCJSISlotAdmission
	}
	receipt := p.expected
	receipt.identity, receipt.digest, receipt.size = slot.identity, slot.digest, slot.size
	var open cjsiCombinedOpen
	if commitOpen != nil {
		open, e = bindCJSICombinedOpen(a, receipt, proof.target, proof.delta)
		if e != nil {
			return e
		}
	}
	if e := r.waitErr(ctx); e != nil {
		return e
	}
	db, e := r.core(ctx, true)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	// Also bind the selector to the original physical admission, not merely
	// a proposal with identical logical fields from some other caller.
	selectExact := func(ctx context.Context, q cjsiRowQueryer) (cjsiSlotAdmission, error) {
		current, err := proof.current(ctx, q)
		if err != nil {
			return cjsiSlotAdmission{}, err
		}
		current.proposal = current.proposal.utc()
		if current != a {
			return cjsiSlotAdmission{}, errCJSISlotAdmission
		}
		return current, nil
	}
	if commitOpen == nil {
		if _, e = registerCJSISlotReceipt(ctx, tx, p, receipt, selectExact); e != nil {
			return e
		}
		if e := r.waitErr(ctx); e != nil {
			return e
		}
		return tx.Commit()
	}
	if _, e = selectExact(ctx, tx); e != nil {
		return e
	}
	if e = open.open(ctx, tx); e != nil {
		return e
	}
	if e := r.waitErr(ctx); e != nil {
		return e
	}
	if e = commitOpen(tx); e == nil {
		return nil
	}
	// Resolve an uncertain writer and confirm through a separately opened
	// managed pool. Confirmation may outlive the waiter, but cannot write.
	commitErr := e
	if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
		return errors.Join(errCJSIOpenUnconfirmed, commitErr, rb)
	}
	check, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	reader, e := r.core(check, false)
	if e != nil {
		return errors.Join(errCJSIOpenUnconfirmed, commitErr, e)
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	if e = open.confirmCommitted(check, reader); e != nil {
		return errors.Join(errCJSIOpenUnconfirmed, commitErr, e)
	}
	return nil
}

func (r cjsiFrozenRunner) checkLockRoot(lock *Lock) error {
	info, err := lock.dir.Stat()
	if err != nil {
		return err
	}
	if cjsiIdentity(info.Sys().(*syscall.Stat_t)) != r.admission.root {
		return errCJSISlotAdmission
	}
	return nil
}
