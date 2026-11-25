package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
)

// All managed protection, user-origin and publication writers enter the
// same physical lock order. Read-only consumers need neither lock.
func (m CJSIRuntimeRoot) withPublicationLocks(ctx context.Context, election CJSIRuntimeElection, run func() error) (err error) {
	if err = m.Check(); err != nil {
		return err
	}
	r := m.runner()
	scan, err := AcquireScanLock(ctx, m.path, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, scan.Release()) }()
	if err = r.checkLockRoot(scan); err != nil {
		return err
	}
	state, err := AcquireLock(ctx, m.path, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, state.Release()) }()
	if err = r.checkLockRoot(state); err != nil {
		return err
	}
	if err = m.Check(); err != nil {
		return err
	}
	if election != nil {
		if err = election(); err != nil {
			return err
		}
	}
	return run()
}

func (m CJSIRuntimeRoot) openPublicationIndex(ctx context.Context, h CJSIIndexStamp, writer bool) (*sql.DB, error) {
	if h.CoreEpoch != m.epoch || h.IndexEpoch <= 0 || h.Inode == 0 {
		return nil, ErrCJSIIndexChanged
	}
	path := filepath.Join(m.path, cjsiPrivateIndexName)
	check := func() error {
		st, err := privateServingPath(path, false)
		if err != nil {
			return err
		}
		if st.Nlink != 1 || cjsiIdentity(st) != (cjsiFileIdentity{h.Device, h.Inode}) {
			return ErrCJSIIndexChanged
		}
		return m.Check()
	}
	if err := check(); err != nil {
		return nil, err
	}
	mode := "mode=ro&_pragma=query_only(1)"
	if writer {
		mode = "mode=rw&_txlock=immediate"
	}
	db, err := openLegacySQL(path, legacyURI(path, mode+"&_pragma=foreign_keys(1)&_pragma=synchronous(FULL)&_pragma=busy_timeout(0)"), "session", legacyRootID{m.root.device, m.root.inode})
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err == nil {
		err = check()
	}
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return db, nil
}

// ApplyOriginMutation includes the last private selection check and the core
// COMMIT in the same exclusion window as all managed protection writers.
func (m CJSIRuntimeRoot) ApplyOriginMutation(ctx context.Context, mutation OriginMutation) (receipt OriginReceipt, err error) {
	err = m.withPublicationLocks(ctx, nil, func() (err error) {
		db, err := m.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		s := Store{DB: db, path: filepath.Join(m.path, "agentdeck.sqlite3")}
		receipt, err = s.ApplyOriginMutation(ctx, mutation)
		return err
	})
	return
}

// WithProtectionWrite reserves the core writer before touching the private
// index. Even a stamped set that already holds BEGIN IMMEDIATE finishes its
// check and COMMIT first. A crash between the two commits leaves the private
// protection authoritative; readers reject the stale core mirror.
func (m CJSIRuntimeRoot) WithProtectionWrite(ctx context.Context, change func(*sql.Tx, CJSIIndexStamp) error, mirror func(*sql.Tx, CJSIIndexStamp) error) error {
	if change == nil || mirror == nil {
		return ErrCJSIIndexChanged
	}
	return m.withPublicationLocks(ctx, nil, func() (err error) {
		db, err := m.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		h, err := cjsiReadIndexHead(ctx, tx)
		if err != nil {
			return err
		}
		j, err := cjsiReadLayoutJournal(ctx, tx)
		if err != nil {
			return err
		}
		if j.phase != cjsiLayoutReady || j.root != m.root || j.fence != m.fence || j.epoch != h.IndexEpoch {
			return ErrCJSIIndexChanged
		}
		location, err := locateCJSISessionIndex(ctx, m.path, tx)
		if err != nil {
			return err
		}
		if location.route != cjsiRoutePrivate || location.selection.identity != j.prepared.identity || location.selection.epoch != h.IndexEpoch {
			return ErrCJSIIndexChanged
		}
		h.Device, h.Inode = j.prepared.identity.device, j.prepared.identity.inode
		index, err := m.openPublicationIndex(ctx, h, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, index.Close()) }()
		it, err := index.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer it.Rollback()
		if err = change(it, h); err != nil {
			return err
		}
		if err = it.Commit(); err != nil {
			return err
		}
		if err = mirror(tx, h); err != nil {
			return err
		}
		return tx.Commit()
	})
}

// CJSISourceWithdrawal identifies the original row authenticated by the
// index planner. A replacement occupies the same physical candidate key.
type CJSISourceWithdrawal struct{ Path, Identity string }

func verifyCJSIWithdrawals(ctx context.Context, tx *sql.Tx, ticket, attempt int64, list []CJSISourceWithdrawal) error {
	expected := make(map[string]string, len(list))
	for _, old := range list {
		if old.Path == "" || old.Identity == "" || expected[old.Path] != "" {
			return ErrCJSIIndexChanged
		}
		expected[old.Path] = old.Identity
	}
	// Enumerate the physical attempt's logical withdrawals, including an
	// upsert shadowing a different base identity. A delete without a matching
	// base row is itself invalid. Stop at the first unexpected member.
	rows, err := tx.QueryContext(ctx, `SELECT c.source_path,coalesce(b.identity,'')
 FROM cjsi_candidate_sources c LEFT JOIN session_sources b ON b.source_path=c.source_path
 WHERE c.ticket=? AND c.attempt=? AND (c.op='delete' OR (c.op='upsert' AND c.identity<>b.identity))`, ticket, attempt)
	if err != nil {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, identity string
		if err := rows.Scan(&path, &identity); err != nil {
			return errors.Join(ErrCJSIIndexChanged, err)
		}
		if want, ok := expected[path]; !ok || want != identity {
			return ErrCJSIIndexChanged
		}
		delete(expected, path)
	}
	if err := rows.Err(); err != nil {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	if len(expected) != 0 {
		return ErrCJSIIndexChanged
	}
	return nil
}
