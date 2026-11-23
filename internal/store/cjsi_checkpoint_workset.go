package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// Owned by the future numbered migration. The adapter never applies this DDL
// or opens a database. These tables are private staging, not publication.
const cjsiCheckpointWorksetSchema = `
CREATE TABLE cjsi_checkpoint_attempts (
    round_id INTEGER NOT NULL, attempt_id INTEGER NOT NULL,
    identity TEXT NOT NULL, generation INTEGER NOT NULL, parser INTEGER NOT NULL,
    max_line_bytes INTEGER NOT NULL, capture_end INTEGER NOT NULL,
    complete_end INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft','sealed','aborted')),
    PRIMARY KEY (round_id, attempt_id),
    CHECK (round_id > 0 AND attempt_id > 0 AND generation > 0 AND parser > 0 AND max_line_bytes > 0),
    CHECK (complete_end >= 0 AND complete_end <= capture_end)
);
CREATE UNIQUE INDEX cjsi_checkpoint_one_draft ON cjsi_checkpoint_attempts(round_id) WHERE state='draft';
CREATE TABLE cjsi_checkpoint_records (
    round_id INTEGER NOT NULL, attempt_id INTEGER NOT NULL,
    start_byte INTEGER NOT NULL, end_byte INTEGER NOT NULL,
    digest BLOB NOT NULL CHECK (length(digest)=32),
    kind TEXT NOT NULL, thread_id TEXT NOT NULL, turn_id TEXT NOT NULL, call_id TEXT NOT NULL,
    metadata TEXT NOT NULL,
    PRIMARY KEY (round_id, attempt_id, start_byte),
    FOREIGN KEY (round_id, attempt_id) REFERENCES cjsi_checkpoint_attempts(round_id, attempt_id),
    CHECK (start_byte >= 0 AND end_byte > start_byte)
) WITHOUT ROWID;
CREATE INDEX cjsi_checkpoint_turn ON cjsi_checkpoint_records(round_id, attempt_id, turn_id, start_byte);
CREATE INDEX cjsi_checkpoint_call ON cjsi_checkpoint_records(round_id, attempt_id, call_id, start_byte);
`

var errCJSICheckpointWorkset = errors.New("invalid or unavailable Codex checkpoint workset")

// Round names one source workset, not a multi-source scan. Attempt IDs must
// not be reused, including after abort. Identity is ingest's device:inode.
type cjsiCheckpointBinding struct {
	Round, Attempt int64
	Identity       string
	Version        ingest.CodexCheckpointVersion
	CaptureEnd     int64
}

const cjsiCheckpointMatch = `round_id=? AND attempt_id=? AND identity=? AND generation=? AND parser=? AND max_line_bytes=? AND capture_end=?`

func (b cjsiCheckpointBinding) args() []any {
	return []any{b.Round, b.Attempt, b.Identity, b.Version.Generation, b.Version.Parser, b.Version.MaxLineBytes, b.CaptureEnd}
}

func (b cjsiCheckpointBinding) valid() bool {
	dev, ino, found := strings.Cut(b.Identity, ":")
	signedDevice, signedDeviceErr := strconv.ParseInt(dev, 10, 64)
	device, deviceErr := strconv.ParseUint(dev, 10, 64)
	inode, inodeErr := strconv.ParseUint(ino, 10, 64)
	canonicalDevice := deviceErr == nil && strconv.FormatUint(device, 10) == dev ||
		signedDeviceErr == nil && strconv.FormatInt(signedDevice, 10) == dev
	return found && canonicalDevice && inodeErr == nil && inode != 0 && strconv.FormatUint(inode, 10) == ino &&
		b.Round > 0 && b.Attempt > 0 && b.Version.Generation > 0 && b.Version.Parser > 0 &&
		b.Version.MaxLineBytes > 0 && b.CaptureEnd >= 0
}

// One replay caller owns this sink. Writes are short transactions so source
// I/O between Stage calls holds no writer lock.
type cjsiCheckpointWorkset struct {
	db      *sql.DB
	binding cjsiCheckpointBinding
	aborted bool
}

var _ ingest.CodexCheckpointWorkset = (*cjsiCheckpointWorkset)(nil)

func beginCJSICheckpointWorkset(ctx context.Context, db *sql.DB, binding cjsiCheckpointBinding) (*cjsiCheckpointWorkset, error) {
	if db == nil || !binding.valid() {
		return nil, errCJSICheckpointWorkset
	}
	_, err := db.ExecContext(ctx, `INSERT INTO cjsi_checkpoint_attempts
		(round_id,attempt_id,identity,generation,parser,max_line_bytes,capture_end)
		VALUES(?,?,?,?,?,?,?)`, binding.args()...)
	if err != nil {
		return nil, err
	}
	return &cjsiCheckpointWorkset{db: db, binding: binding}, nil
}

func (w *cjsiCheckpointWorkset) StageCodexRecord(ctx context.Context, record ingest.CodexRecord, evidence ingest.CodexCheckpointEvidence) (err error) {
	if w.aborted {
		return errCJSICheckpointWorkset
	}
	defer func() {
		if err != nil {
			w.Abort()
		}
	}()
	span := evidence.Span
	if span != record.Span || span.Start < 0 || span.End <= span.Start || span.End > w.binding.CaptureEnd ||
		evidence.Digest == ([sha256.Size]byte{}) ||
		(span.End-span.Start > int64(w.binding.Version.MaxLineBytes)) != (record.Kind == ingest.CodexOversize) {
		return errCJSICheckpointWorkset
	}
	kind, metadata, err := encodeCJSICheckpointRecord(record)
	if err != nil {
		return err
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	args := append([]any{span.End}, w.binding.args()...)
	args = append(args, span.Start)
	result, err := tx.ExecContext(ctx, `UPDATE cjsi_checkpoint_attempts SET complete_end=?
		WHERE `+cjsiCheckpointMatch+` AND state='draft' AND complete_end=?`, args...)
	if err := cjsiCheckpointRequireRow(result, err); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_checkpoint_records
		(round_id,attempt_id,start_byte,end_byte,digest,kind,thread_id,turn_id,call_id,metadata)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, w.binding.Round, w.binding.Attempt, span.Start, span.End,
		evidence.Digest[:], kind, record.ThreadID, record.TurnID, record.CallID, metadata); err != nil {
		return err
	}
	return tx.Commit()
}

// Seal takes the terminal Step from the owning ingest replay. The private
// seal permits ordered resume; the caller still owns source revalidation and
// any later publication CAS. Repeating a successful seal is idempotent.
func (w *cjsiCheckpointWorkset) Seal(ctx context.Context, step ingest.CodexCheckpointStep) error {
	b := w.binding
	p := step.Progress
	if w.aborted || !step.Done || step.Prefix != (ingest.CodexCheckpointPrefix{
		Version: b.Version, Identity: b.Identity, CompleteEnd: p.CompleteEnd,
	}) || p.ReadEnd != b.CaptureEnd || p.CompleteEnd < 0 || p.CompleteEnd > p.ReadEnd ||
		p.Partial != (p.CompleteEnd < p.ReadEnd) {
		return errCJSICheckpointWorkset
	}
	result, err := w.db.ExecContext(ctx, `UPDATE cjsi_checkpoint_attempts SET state='sealed'
		WHERE `+cjsiCheckpointMatch+` AND state IN ('draft','sealed') AND complete_end=?`, append(b.args(), p.CompleteEnd)...)
	return cjsiCheckpointRequireRow(result, err)
}

// OpenCodexWorkset opens an independent pass on this attempt's sealed rows.
func (w *cjsiCheckpointWorkset) OpenCodexWorkset(ctx context.Context) (ingest.CodexWorksetNext, error) {
	reader, err := openCJSICheckpointRecords(ctx, w.db, w.binding)
	if err != nil {
		return nil, err
	}
	// Each Next finishes its SQL query before returning. A pass can stop
	// before EOF without keeping rows, a connection, or a transaction open.
	return reader.Next, nil
}

// Retire discards this sealed attempt when its owner no longer needs it
// for reduction or resume. Draft cleanup must still lose to a seal.
func (w *cjsiCheckpointWorkset) Retire(ctx context.Context) error {
	return discardCJSICheckpointAttempt(ctx, w.db, w.binding, nil, "sealed")
}

// Abort has no error channel in ingest.CodexCheckpointSink. A failed cleanup
// leaves a nonselectable draft; the coordinator can retry with its exact key.
func (w *cjsiCheckpointWorkset) Abort() {
	if w.aborted {
		return
	}
	w.aborted = true
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = discardCJSICheckpointWorkset(ctx, w.db, w.binding)
}

func discardCJSICheckpointWorkset(ctx context.Context, db *sql.DB, b cjsiCheckpointBinding) error {
	return discardCJSICheckpointAttempt(ctx, db, b, nil, "draft")
}

func discardCJSICheckpointAttempt(ctx context.Context, db *sql.DB, b cjsiCheckpointBinding, completeEnd *int64, state string) error {
	if db == nil || !b.valid() {
		return errCJSICheckpointWorkset
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := `UPDATE cjsi_checkpoint_attempts SET state='aborted'
		WHERE ` + cjsiCheckpointMatch + ` AND state IN (?,'aborted')`
	args := append(b.args(), state)
	if completeEnd != nil {
		query += ` AND complete_end=?`
		args = append(args, *completeEnd)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err := cjsiCheckpointRequireRow(result, err); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cjsi_checkpoint_records WHERE round_id=? AND attempt_id=?`, b.Round, b.Attempt); err != nil {
		return err
	}
	// Keep the tombstone so a stale attempt cannot be reused.
	return tx.Commit()
}

func cjsiCheckpointRequireRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errCJSICheckpointWorkset
	}
	return nil
}
