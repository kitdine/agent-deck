package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type cjsiRunnerFixtureDelta struct {
	p       cjsiSlotProposal
	next    cjsiOpenTarget
	applied int
	late    error
}

func (d *cjsiRunnerFixtureDelta) binding() (cjsiSlotProposal, cjsiOpenTarget) { return d.p, d.next }
func (d *cjsiRunnerFixtureDelta) apply(ctx context.Context, tx *sql.Tx) error {
	d.applied++
	return cjsiOpenOne(tx.ExecContext(ctx, `INSERT INTO main.usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
		VALUES('runner-event','codex','fixture-a','ev','2030-01-01','unit',17,'fixture',0)`))
}
func (d *cjsiRunnerFixtureDelta) stampSelected(context.Context, *sql.Tx) error { return d.late }
func (d *cjsiRunnerFixtureDelta) verify(ctx context.Context, q cjsiRowQueryer) error {
	var one int
	return q.QueryRowContext(ctx, `SELECT 1 FROM main.usage_events WHERE event_key='runner-event' AND client='codex' AND session_id='fixture-a' AND input_tokens=17`).Scan(&one)
}

type cjsiRunnerFixture struct {
	t                                 *testing.T
	db                                *sql.DB
	r                                 cjsiFrozenRunner
	d                                 *cjsiRunnerFixtureDelta
	data                              []byte
	ioCalls, qualifications, releases int
	afterPhase                        func(int)
	atSync                            func()
	releaseGate                       chan struct{}
	gateOnce                          sync.Once
	adopt                             bool
}

func (f *cjsiRunnerFixture) releaseIO() { f.gateOnce.Do(func() { close(f.releaseGate) }) }

func newCJSIRunnerFixture(t *testing.T) *cjsiRunnerFixture {
	t.Helper()
	base := newCJSIPreparationFixture(t)
	p := base.frozen.admission.proposal
	next := cjsiOpenTarget{p.base, p.usageRevision + 1}
	next.session.Ticket, next.session.SessionTicket, next.session.SessionAttempt = p.ticket, p.ticket, p.attempt
	next.session.SessionSeal = p.sessionSeal
	next.session.SessionRevision++
	p.diff = sha256.Sum256([]byte(fmt.Sprintf("fixture: absent->runner-event=17; no interval stamps; %x/%x/%+v", p.usageMaterial, p.sessionSeal, next)))
	base.frozen.admission.proposal = p
	commitCJSIPrepare(t, base.db, base.reserve, 1)
	base.stage(t)
	commitCJSIPrepare(t, base.db, base.freeze, 1)
	// Finish all preparer's read leases before entering slot I/O. Independent
	// test observers use separate SQLite connections.
	for _, name := range []string{"agentdeck.sqlite3", cjsiPrivateIndexName} {
		if err := closeCJSIInstallReadPool(filepath.Join(base.draft.admission.rootPath, name)); err != nil {
			t.Fatal(err)
		}
	}
	f := &cjsiRunnerFixture{t: t, db: base.db, data: []byte("immutable prepared serving bytes"), releaseGate: make(chan struct{})}
	p.expected.digest = sha256.Sum256(f.data)
	p.expected.size = int64(len(f.data))
	p.expected.format = 1
	p.expected.builtAt = time.Now().UTC().Add(-time.Minute)
	p.expected.validUntil = p.acceptUntil.Add(time.Hour)
	a := base.frozen.admission
	a.proposal = p
	st, err := privateServingPath(filepath.Join(a.rootPath, "agentdeck.sqlite3"), false)
	if err != nil {
		t.Fatal(err)
	}
	f.d = &cjsiRunnerFixtureDelta{p: p, next: next}
	f.r = cjsiFrozenRunner{admission: a, coreID: cjsiIdentity(st), slotIO: f.write}
	f.r.qualify = func(ctx context.Context, root, slot *os.File) (cjsiRunnerProof, error) {
		f.qualifications++
		// The session-owner stand-in pins the actual private index inode and
		// reads its sealed candidate before the core write transaction.
		indexPath := filepath.Join(a.rootPath, cjsiPrivateIndexName)
		fd, err := cjsiOpenPrivateLayoutPath(indexPath, false)
		if err != nil {
			return cjsiRunnerProof{}, err
		}
		db, err := sql.Open("sqlite", cjsiSQLiteURI(indexPath, "ro"))
		if err != nil {
			fd.Close()
			return cjsiRunnerProof{}, err
		}
		var one int
		err = db.QueryRowContext(ctx, `SELECT 1 FROM cjsi_session_candidates WHERE ticket=? AND attempt=? AND parent_ticket=? AND index_epoch=? AND base_seal=? AND session_revision=? AND seal_digest=?`,
			p.ticket, p.attempt, p.base.Ticket, p.base.IndexEpoch, p.base.BaseSeal[:], next.session.SessionRevision, p.sessionSeal[:]).Scan(&one)
		if err = errors.Join(err, db.Close()); err != nil {
			fd.Close()
			return cjsiRunnerProof{}, err
		}
		return cjsiRunnerProof{target: next, delta: f.d, current: func(ctx context.Context, q cjsiRowQueryer) (cjsiSlotAdmission, error) {
			return a, checkCJSIStagedGroups(ctx, q.(*sql.Tx), p, base.frozen.groups)
		}, release: func() error {
			f.releases++
			if f.afterPhase != nil {
				f.afterPhase(f.releases)
			}
			return fd.Close()
		}}, nil
	}
	return f
}

func (f *cjsiRunnerFixture) write(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (out cjsiRunnerSlot, err error) {
	f.ioCalls++
	name := fmt.Sprintf(".cjsi-serving-slot-%x", id)
	out = cjsiRunnerSlot{name: name, identity: id, digest: sha256.Sum256(data), size: int64(len(data))}
	if !sealed && !f.adopt {
		if f.atSync != nil {
			f.atSync()
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		disk, e := persistCJSIServingSlotAt(int(root.Fd()), id, data, unix.Fsync)
		if e != nil {
			return out, e
		}
		out.identity, out.digest, out.size = disk.identity, disk.digest, disk.size
	}
	fd, err := unix.Openat(int(root.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return out, err
	}
	out.file = os.NewFile(uintptr(fd), name)
	if f.adopt && !sealed {
		err = errors.Join(out.file.Sync(), root.Sync())
	}
	return out, err
}

func (f *cjsiRunnerFixture) checkView(t *testing.T, tx *sql.Tx, opened bool) {
	t.Helper()
	p := f.r.admission.proposal
	var ticket, session, slot, tokens int64
	var state string
	var ss, ds []byte
	err := tx.QueryRow(`SELECT h.ticket,h.session_ticket,h.slot_ticket,h.session_seal,h.slot_seal,a.state,
		coalesce((SELECT input_tokens FROM usage_events WHERE event_key='runner-event'),0)
		FROM cjsi_publication_head h JOIN cjsi_publication_attempts a ON a.ticket=? AND a.attempt=? WHERE h.singleton=1`, p.ticket, p.attempt).Scan(&ticket, &session, &slot, &ss, &ds, &state, &tokens)
	if err != nil {
		t.Fatal(err)
	}
	if opened {
		seal := p.seal()
		if ticket != p.ticket || session != ticket || slot != ticket || state != "opened" || tokens != 17 || !bytes.Equal(ss, p.sessionSeal[:]) || !bytes.Equal(ds, seal[:]) {
			t.Fatal("mixed or incomplete opened view")
		}
	} else if ticket != p.base.Ticket || session != p.base.SessionTicket || slot != p.baseSlotTicket || tokens != 0 || state == "opened" {
		t.Fatal("candidate visible to precommit reader")
	}
}

func checkCJSIRunnerLocks(t *testing.T, root string) {
	t.Helper()
	scan, err := AcquireScanLock(t.Context(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer scan.Release()
	state, err := AcquireLock(t.Context(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestCJSIFrozenRunnerVisibility(t *testing.T) {
	f := newCJSIRunnerFixture(t)
	a := f.r.admission
	old := beginCJSIFixtureTx(t, f.db)
	f.checkView(t, old, false)
	f.atSync = func() {
		checkCJSIRunnerLocks(t, a.rootPath)
		// This is a distinct connection to the real managed SQLite file.
		conn, err := f.db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.ExecContext(t.Context(), `BEGIN IMMEDIATE; INSERT INTO inventory_runs VALUES('unrelated-runner',1,'t',0,'complete'); COMMIT`)
		if err = errors.Join(err, conn.Close()); err != nil {
			t.Fatal(err)
		}
		pin, err := admitLegacy(t.Context(), a.rootPath)
		if err != nil {
			t.Fatal(err)
		}
		if err = pin.close(); err != nil {
			t.Fatal(err)
		}
		f.releaseIO()
	}
	f.afterPhase = func(n int) {
		if n == 1 {
			assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='sealed'", 1)
			tx := beginCJSIFixtureTx(t, f.db)
			f.checkView(t, tx, false)
			tx.Rollback()
		}
	}
	if err := f.r.run(t.Context(), f.data); err != nil {
		t.Fatal(err)
	}
	f.checkView(t, old, false)
	old.Rollback()
	fresh := beginCJSIFixtureTx(t, f.db)
	f.checkView(t, fresh, true)
	if err := WithCJSIIndexSnapshot(t.Context(), a.rootPath, fresh, func(s CJSIIndexSnapshot) error {
		if s.Selection() != f.d.next.session {
			return errCJSIOpenChanged
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fresh.Rollback()
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM inventory_runs WHERE inventory_id='unrelated-runner'", 1)
	if f.ioCalls != 1 || f.releases != 2 || f.qualifications != 2 || f.d.applied != 1 {
		t.Fatal("unexpected I/O, ownership or apply counts")
	}
	if err := f.r.run(t.Context(), f.data); err == nil || f.ioCalls != 1 || f.d.applied != 1 {
		t.Fatalf("already-opened attempt replay: %v", err)
	}
}

func TestCJSIFrozenRunnerCanceledWait(t *testing.T) {
	f := newCJSIRunnerFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.atSync = cancel
	if err := f.r.run(ctx, f.data); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter: %v", err)
	}
	if f.qualifications != 0 || f.d.applied != 0 {
		t.Fatal("canceled waiter entered a writer phase")
	}

	checkCJSIRunnerLocks(t, f.r.admission.rootPath)
	f.releaseIO()
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft' AND slot_seal IS NULL", 1)
}

func TestCJSIFrozenRunnerRefusals(t *testing.T) {
	t.Run("unfrozen", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		cjsiFixtureExec(t, f.db, "UPDATE cjsi_publication_attempts SET diff_digest=NULL,session_seal=NULL")
		if err := f.r.run(t.Context(), f.data); err == nil || f.ioCalls != 0 {
			t.Fatalf("unfrozen: %v", err)
		}
	})
	t.Run("io-result-and-dependency", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		write := f.r.slotIO
		f.r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
			slot, err := write(ctx, root, info, id, data, sealed)
			cjsiFixtureExec(t, f.db, "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1")
			return slot, err
		}
		if err := f.r.run(t.Context(), f.data); err == nil || f.qualifications != 1 || f.d.applied != 0 {
			t.Fatalf("stale result: %v", err)
		}
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE slot_seal IS NULL", 1)
	})
	t.Run("deadline-after-io", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		write := f.r.slotIO
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Minute))
		defer cancel()
		// A changed durable accept window invalidates a just-finished receipt.
		f.r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
			slot, err := write(ctx, root, info, id, data, sealed)
			cjsiFixtureExec(t, f.db, "UPDATE cjsi_publication_attempts SET accept_until='2000-01-01T00:00:00Z'")
			return slot, err
		}
		if err := f.r.run(ctx, f.data); err == nil {
			t.Fatal("admitted late disk result")
		}
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft' AND slot_seal IS NULL", 1)
	})
	t.Run("release-and-retry-private-seal", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		f.d.late = errors.New("reject after head write")
		if err := f.r.run(t.Context(), f.data); !errors.Is(err, f.d.late) {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='sealed'", 1)
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM usage_events WHERE event_key='runner-event'", 0)
		checkCJSIRunnerLocks(t, f.r.admission.rootPath)
		f.d.late = nil
		if err := f.r.run(t.Context(), f.data); err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM usage_events WHERE event_key='runner-event'", 1)
	})
	t.Run("changed-between-seal-and-open", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		f.afterPhase = func(n int) {
			if n == 1 {
				cjsiFixtureExec(t, f.db, "UPDATE cjsi_publication_head SET protection_revision=protection_revision+1")
			}
		}
		if err := f.r.run(t.Context(), f.data); err == nil || f.d.applied != 0 {
			t.Fatalf("opened after protection changed: %v", err)
		}
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='sealed'", 1)
	})
	t.Run("displaced-root", func(t *testing.T) {
		f := newCJSIRunnerFixture(t)
		path := f.r.admission.rootPath
		moved := filepath.Join(filepath.Dir(path), "moved")
		if err := os.Rename(path, moved); err != nil {
			t.Fatal(err)
		}
		defer os.Rename(moved, path)
		if err := f.r.run(t.Context(), f.data); err == nil || f.ioCalls != 0 {
			t.Fatalf("missing root: %v", err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("recreated root: %v", err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)
		if err := f.r.run(t.Context(), f.data); err == nil || f.ioCalls != 0 {
			t.Fatalf("successor root: %v", err)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			t.Fatalf("modified successor: %v %v", entries, err)
		}
	})
}

func TestCJSIFrozenRunnerCommitOutcome(t *testing.T) {
	for _, committed := range []bool{true, false} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			f := newCJSIRunnerFixture(t)
			lost := errors.New("commit acknowledgment lost")
			err := f.r.runWithCommit(t.Context(), f.data, func(tx *sql.Tx) error {
				if committed {
					if err := tx.Commit(); err != nil {
						return err
					}
				}
				return lost
			})
			if committed && err != nil {
				t.Fatalf("failed to confirm committed opening: %v", err)
			}
			if !committed && (!errors.Is(err, errCJSIOpenUnconfirmed) || !errors.Is(err, lost)) {
				t.Fatalf("missing unresolved outcome: %v", err)
			}
			if f.d.applied != 1 || f.releases != 2 {
				t.Fatalf("unexpected replay or release: %d/%d", f.d.applied, f.releases)
			}
			want := 0
			if committed {
				want = 1
			}
			assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM usage_events WHERE event_key='runner-event'", want)
		})
	}
}

// A caller deadline can already have passed while its Done notification is
// still pending. Change only the reported deadline at the I/O return barrier.
type cjsiRunnerDeadline struct {
	context.Context
	nanos atomic.Int64
}

func (c *cjsiRunnerDeadline) Deadline() (time.Time, bool) { return time.Unix(0, c.nanos.Load()), true }
func TestCJSIFrozenRunnerLateReceipt(t *testing.T) {
	f := newCJSIRunnerFixture(t)
	wait := &cjsiRunnerDeadline{Context: t.Context()}
	wait.nanos.Store(time.Now().Add(time.Minute).UnixNano())
	write := f.r.slotIO
	f.r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
		pin, err := write(ctx, root, info, id, data, sealed)
		wait.nanos.Store(time.Now().Add(-time.Minute).UnixNano())
		return pin, err
	}
	if err := f.r.run(wait, f.data); !errors.Is(err, context.DeadlineExceeded) || f.qualifications != 0 {
		t.Fatalf("late disk receipt entered writer admission: %v", err)
	}
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft' AND slot_seal IS NULL", 1)
	// Explicit adoption of the qualified orphan re-establishes file and
	// directory durability, without replacing or reserializing its bytes.
	slotPath := filepath.Join(f.r.admission.rootPath, fmt.Sprintf(".cjsi-serving-slot-%x", f.d.p.expected.identity))
	before, err := os.Lstat(slotPath)
	if err != nil {
		t.Fatal(err)
	}
	f.adopt = true
	f.r.slotIO = write
	if err := f.r.run(t.Context(), f.data); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(slotPath)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("adoption replaced orphan")
	}
}

func TestCJSIFrozenRunnerSlotIdentity(t *testing.T) {
	for _, fault := range []string{"receipt-digest", "unlinked-handle", "changed-bytes", "writable-handle"} {
		t.Run(fault, func(t *testing.T) {
			f := newCJSIRunnerFixture(t)
			write := f.r.slotIO
			f.r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
				pin, err := write(ctx, root, info, id, data, sealed)
				if err != nil {
					return pin, err
				}
				path := filepath.Join(f.r.admission.rootPath, pin.name)
				switch fault {
				case "receipt-digest":
					pin.digest[0]++
				case "unlinked-handle":
					if err = os.Rename(path, path+".old"); err == nil {
						err = os.WriteFile(path, data, 0600)
					}
				case "changed-bytes":
					err = os.WriteFile(path, bytes.Repeat([]byte{'x'}, len(data)), 0600)
				case "writable-handle":
					if err = pin.file.Close(); err == nil {
						pin.file, err = os.OpenFile(path, os.O_RDWR, 0)
					}
				}
				return pin, err
			}
			if err := f.r.run(t.Context(), f.data); err == nil || f.qualifications != 0 {
				t.Fatalf("invalid pin accepted: %v", err)
			}
			assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft' AND slot_seal IS NULL", 1)
		})
	}
}
