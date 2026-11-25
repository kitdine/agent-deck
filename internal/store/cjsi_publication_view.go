package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func (m CJSIRuntimeRoot) runner() cjsiFrozenRunner {
	return cjsiFrozenRunner{admission: cjsiSlotAdmission{rootPath: m.path, root: m.root, fence: m.fence}, coreID: m.core}
}

// WithPublishedCore supplies the same committed read transaction to every
// domain reader in one response. Buffer the response until this call returns.
func (m CJSIRuntimeRoot) WithPublishedCore(ctx context.Context, read func(*sql.Tx, CJSIIndexStamp) error) (err error) {
	if read == nil {
		return ErrCJSIIndexChanged
	}
	db, err := m.runner().core(ctx, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	h, err := cjsiReadIndexHead(ctx, tx)
	if err != nil {
		return err
	}
	if h.CoreEpoch != m.epoch {
		return ErrCJSIIndexChanged
	}
	if err = read(tx, h); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	live, err := cjsiReadIndexHead(ctx, db)
	if err != nil {
		return err
	}
	if live != h {
		return ErrCJSIIndexChanged
	}
	return m.Check()
}

// ReadServingSlot selects an opened receipt inside the caller's core read.
// The desktop owner checks the encoded payload's date, format and domain gates.
func (m CJSIRuntimeRoot) ReadServingSlot(ctx context.Context, tx *sql.Tx, now time.Time) ([]byte, error) {
	if tx == nil || now.IsZero() {
		return nil, ErrCJSIIndexChanged
	}
	var identity, digest []byte
	var size int64
	var built, until string
	err := tx.QueryRowContext(ctx, `SELECT a.slot_identity,a.slot_digest,a.slot_size,a.built_at,a.valid_until
 FROM main.cjsi_publication_head h JOIN main.cjsi_publication_attempts a
 ON a.ticket=h.slot_ticket AND a.attempt=h.slot_attempt AND a.slot_seal=h.slot_seal
 JOIN main.derived_snapshot_generation g ON g.singleton=h.singleton AND g.epoch=h.core_epoch
 WHERE h.singleton=1 AND h.core_epoch=? AND a.core_epoch=h.core_epoch AND a.index_epoch=h.index_epoch
 AND a.state='opened' AND h.session_ticket=a.ticket AND h.session_attempt=a.attempt AND a.session_seal=h.session_seal
 AND NOT EXISTS(SELECT 1 FROM cjsi_scope_overlays WHERE masked=1)
 AND h.protection_revision=a.protection_revision AND a.slot_format=1 AND (SELECT file FROM pragma_database_list WHERE name='main')=?`, m.epoch, filepath.Join(m.path, "agentdeck.sqlite3")).Scan(&identity, &digest, &size, &built, &until)
	if err != nil {
		return nil, errors.Join(ErrCJSIIndexChanged, err)
	}
	start, e := time.Parse(time.RFC3339Nano, built)
	end, e2 := time.Parse(time.RFC3339Nano, until)
	if e != nil || e2 != nil || now.Before(start) || !now.Before(end) || len(identity) != 16 || len(digest) != 32 || size <= 0 || size > cjsiMaxSlotBytes {
		return nil, ErrCJSIIndexChanged
	}
	var data []byte
	// Even with a matching core head a private-first revocation may already
	// be durable. Recheck the private protection/revision before exposing bytes.
	err = WithCJSIIndexSnapshot(ctx, m.path, tx, func(CJSIIndexSnapshot) error {
		dir, e := openCJSISlotRoot(m.path)
		if e != nil {
			return e
		}
		defer unix.Close(dir)
		name := cjsiServingSlotName([16]byte(identity))
		fd, e := unix.Openat(dir, name, unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if e != nil {
			return e
		}
		file := os.NewFile(uintptr(fd), name)
		defer file.Close()
		var st unix.Stat_t
		if e = unix.Fstat(fd, &st); e != nil || !privateCJSISlotFile(&st, size) {
			return errors.Join(ErrCJSIIndexChanged, e)
		}
		data, e = io.ReadAll(io.LimitReader(file, size+1))
		if e != nil {
			return e
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != size || !bytes.Equal(sum[:], digest) {
			return ErrCJSIIndexChanged
		}
		var live unix.Stat_t
		if e = unix.Fstatat(dir, name, &live, unix.AT_SYMLINK_NOFOLLOW); e != nil || !privateCJSISlotFile(&live, size) || live.Dev != st.Dev || live.Ino != st.Ino {
			return errors.Join(ErrCJSIIndexChanged, e)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ServingGenerationFor encodes the physical root, selected ticket and privacy
// revisions in the existing state identity field. Builders use the proposed
// selection; ordinary readers use the core-authorized committed selection.
func (m CJSIRuntimeRoot) ServingGenerationFor(h CJSIIndexStamp, usage int64) ServingGeneration {
	identity := fmt.Sprintf("cjsi/v1/%d/%d/%d/%d/%d/%d/%d/%d/%d/%x/%d/%d", m.root.device, m.root.inode, m.core.device, m.core.inode, h.Device, h.Inode, h.Ticket, h.SessionTicket, h.SessionAttempt, h.SessionSeal, h.ProtectionRevision, h.CoverageRevision)
	digest := sha256.Sum256([]byte(identity))
	return ServingGeneration{StateIdentity: hex.EncodeToString(digest[:]), CoreEpoch: h.CoreEpoch, SessionEpoch: h.IndexEpoch, CoreSchema: cjsiLayoutCoreSchemaVersion, SessionSchema: ServingStorageVersion, UsageRevision: usage, SessionRevision: h.SessionRevision}
}

// WithIndexSnapshot verifies the current physical root before borrowing the
// private component of an already-open published core view.
func (m CJSIRuntimeRoot) WithIndexSnapshot(ctx context.Context, tx *sql.Tx, read func(CJSIIndexSnapshot) error) error {
	if err := m.Check(); err != nil {
		return err
	}
	return WithCJSIIndexSnapshot(ctx, m.path, tx, read)
}

// CJSIOpenReceipt names an exact opened attempt. The materializer may store
// or recover this identity, but must recheck the live head and its postimages.
type CJSIOpenReceipt struct {
	CoreEpoch, Ticket, Attempt         int64
	Input, Diff, SessionSeal, SlotSeal [32]byte
}

func (m CJSIRuntimeRoot) ConfirmOpen(ctx context.Context, r CJSIOpenReceipt, now time.Time, verify func(*sql.Tx) error) error {
	if r.CoreEpoch != m.epoch || r.Ticket <= 0 || r.Attempt <= 0 || r.Input == ([32]byte{}) || r.Diff == ([32]byte{}) || verify == nil {
		return ErrCJSIIndexChanged
	}
	return m.WithPublishedCore(ctx, func(tx *sql.Tx, h CJSIIndexStamp) error {
		if h.Ticket != r.Ticket {
			return ErrCJSIIndexChanged
		}
		var session, slot []byte
		if err := tx.QueryRowContext(ctx, `SELECT session_seal,slot_seal FROM cjsi_publication_attempts WHERE core_epoch=? AND ticket=? AND attempt=? AND input_digest=? AND diff_digest=? AND state='opened'`, r.CoreEpoch, r.Ticket, r.Attempt, r.Input[:], r.Diff[:]).Scan(&session, &slot); err != nil {
			return err
		}
		if r.SessionSeal == ([32]byte{}) && r.SlotSeal == ([32]byte{}) {
			if session != nil || slot != nil {
				return ErrCJSIIndexChanged
			}
		} else {
			if !bytes.Equal(slot, r.SlotSeal[:]) || !bytes.Equal(session, r.SessionSeal[:]) || h.SessionSeal != r.SessionSeal || h.SessionTicket != r.Ticket || h.SessionAttempt != r.Attempt {
				return ErrCJSIIndexChanged
			}
			if _, err := m.ReadServingSlot(ctx, tx, now); err != nil {
				return err
			}
		}
		return verify(tx)
	})
}
