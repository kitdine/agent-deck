package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"syscall"

	"github.com/kitdine/agent-deck/internal/ingest"
	"golang.org/x/sys/unix"
)

// CJSICapturedWorkset lends an authenticated physical file and independent
// iterators over its sealed metadata to the usage and session owners.
type CJSICapturedWorkset struct {
	Round, Attempt int64
	File           *os.File
	Replay         *ingest.CodexCheckpointReplay
	Open           func(context.Context) (ingest.CodexWorksetNext, error)
}
type publicationCheckpoint struct {
	draft  CJSIPublicationDraft
	db     *sql.DB
	work   *cjsiCheckpointWorkset
	sealed bool
}

func (s publicationCheckpoint) Abort() {} // Keep ownership and cleanup with the root worker.
func (s publicationCheckpoint) OpenCodexWorkset(ctx context.Context) (ingest.CodexWorksetNext, error) {
	return s.work.OpenCodexWorkset(ctx)
}
func (s publicationCheckpoint) StageCodexRecord(ctx context.Context, r ingest.CodexRecord, e ingest.CodexCheckpointEvidence) error {
	if s.sealed {
		return ctx.Err()
	}
	return s.draft.root.withPublicationLocks(ctx, s.draft.election, func() error {
		if err := s.draft.checkDraft(ctx, s.db); err != nil {
			return err
		}
		return s.work.StageCodexRecord(ctx, r, e)
	})
}

// WithSourceWorkset replays one already selected capture under the draft. No
// scan/state write lock spans parsing, reduction, or the consumer's file reads.
func (d CJSIPublicationDraft) WithSourceWorkset(ctx context.Context, src ingest.Source, selected ingest.CodexSelectedSource, stepBytes int64, consume func(CJSICapturedWorkset) error) (err error) {
	if consume == nil || stepBytes <= 0 || !src.Stable || src.Client != "codex" || src.Path != selected.Path || src.Identity != selected.Evidence.Identity || selected.Evidence.Progress.ReadEnd > src.Size {
		return ErrTelemetrySourceIdentityPending
	}
	if err = ingest.Validate(src); err != nil {
		return err
	}
	encoded, err := json.Marshal(struct {
		Domain                 string
		Epoch, Ticket, Attempt int64
		Input                  [32]byte
		Selected               ingest.CodexSelectedSource
	}{"agentdeck/cjsi/workset/v1", d.Parent().CoreEpoch, d.Basis().Ticket, d.Basis().Attempt, d.Basis().InputDigest, selected})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(encoded)
	round := int64(binary.BigEndian.Uint64(sum[:])>>1) | 1
	dir, err := openCJSISlotRoot(d.root.path)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	// Opening the slot root validates permissions. Bind the actual directory
	// descriptor to the original root before creating the lock entry in it.
	var st syscall.Stat_t
	if err := syscall.Fstat(dir, &st); err != nil {
		return errors.Join(errCJSISlotAdmission, err)
	}
	if cjsiIdentity(&st) != d.root.root {
		return errCJSISlotAdmission
	}
	lease, err := acquireCJSIWorkerLock(dir, ".cjsi-checkpoint-worker.lock")
	if err != nil {
		return errors.Join(ErrTelemetrySourceIdentityPending, err)
	}
	defer lease.Close()
	db, err := d.root.runner().core(ctx, true)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	b := cjsiCheckpointBinding{Round: round, Identity: src.Identity, Version: selected.Evidence.Version, CaptureEnd: selected.Evidence.Progress.ReadEnd}
	var work *cjsiCheckpointWorkset
	var sealed bool
	err = d.root.withPublicationLocks(ctx, d.election, func() error {
		if err := d.checkDraft(ctx, db); err != nil {
			return err
		}
		var state string
		err := db.QueryRowContext(ctx, `SELECT attempt_id,state FROM cjsi_checkpoint_attempts WHERE round_id=? AND identity=? AND generation=? AND parser=? AND max_line_bytes=? AND capture_end=? ORDER BY attempt_id DESC LIMIT 1`, round, b.Identity, b.Version.Generation, b.Version.Parser, b.Version.MaxLineBytes, b.CaptureEnd).Scan(&b.Attempt, &state)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if state == "sealed" {
			sealed = true
			work = &cjsiCheckpointWorkset{db: db, binding: b}
			return nil
		}
		if state == "draft" {
			own := cjsiCheckpointDraftOwner{Round: round, Identity: b.Identity, Version: b.Version}
			stale, found, e := findCJSICheckpointDraft(ctx, db, own)
			if e != nil {
				return e
			}
			if !found || stale.binding != b {
				return ErrTelemetrySourceIdentityPending
			}
			if e = discardCJSICheckpointDraft(ctx, db, stale); e != nil {
				return e
			}
		}
		// An aborted workset's attempt ID must never be reused.
		var nonce [8]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return err
		}
		b.Attempt = int64(binary.BigEndian.Uint64(nonce[:])>>1) | 1
		work, err = beginCJSICheckpointWorkset(ctx, db, b)
		return err
	})
	if err != nil {
		return err
	}
	fd, err := unix.Open(src.Path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), src.Path)
	defer file.Close()
	sink := publicationCheckpoint{d, db, work, sealed}
	replay, err := ingest.BeginCodexCheckpointWorkset(ctx, file, src.Path, b.CaptureEnd, b.Version, nil, sink)
	if err != nil {
		return err
	}
	defer replay.Abort()
	for {
		step, err := replay.Step(ctx, stepBytes)
		if err != nil {
			return err
		}
		if !step.Done {
			continue
		}
		if step.Progress != selected.Evidence.Progress {
			return ErrTelemetrySourceIdentityPending
		}
		if !sealed {
			err = d.root.withPublicationLocks(ctx, d.election, func() error {
				if e := d.checkDraft(ctx, db); e != nil {
					return e
				}
				return work.Seal(ctx, step)
			})
			if err != nil {
				return err
			}
		}
		break
	}
	if err = consume(CJSICapturedWorkset{round, b.Attempt, file, replay, work.OpenCodexWorkset}); err != nil {
		return err
	}
	if err = ingest.Validate(src); err != nil {
		return err
	}
	return d.checkDraft(ctx, db)
}
