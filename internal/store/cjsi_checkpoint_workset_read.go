package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"

	"github.com/kitdine/agent-deck/internal/ingest"
)

type cjsiCheckpointRecordReader struct {
	db            *sql.DB
	binding       cjsiCheckpointBinding
	completeEnd   int64
	cursor        int64
	previousStart int64
}

// openCJSICheckpointRecords opens the private lexical observations of one
// sealed attempt. These observations are not attribution or publication
// authority. The caller still owns source replay and the publication CAS.
func openCJSICheckpointRecords(ctx context.Context, db *sql.DB, b cjsiCheckpointBinding) (*cjsiCheckpointRecordReader, error) {
	if db == nil || !b.valid() {
		return nil, errCJSICheckpointWorkset
	}
	var completeEnd int64
	if err := db.QueryRowContext(ctx, `SELECT complete_end FROM cjsi_checkpoint_attempts
		WHERE `+cjsiCheckpointMatch+` AND state='sealed'`, b.args()...).Scan(&completeEnd); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errCJSICheckpointWorkset
		}
		return nil, err
	}
	if completeEnd < 0 || completeEnd > b.CaptureEnd {
		return nil, errCJSICheckpointWorkset
	}
	return &cjsiCheckpointRecordReader{db: db, binding: b, completeEnd: completeEnd, previousStart: -1}, nil
}

func (r *cjsiCheckpointRecordReader) Prefix() ingest.CodexCheckpointPrefix {
	return ingest.CodexCheckpointPrefix{Version: r.binding.Version, Identity: r.binding.Identity, CompleteEnd: r.completeEnd}
}

// Next seeks by the last start, not the last end, so overlapping rows cannot
// be skipped. A checked seal and absence of another row prove EOF together.
// Size guards run inside SQLite before a damaged field is transferred to Go.
func (r *cjsiCheckpointRecordReader) Next(ctx context.Context) (ingest.CodexRecord, ingest.CodexCheckpointEvidence, error) {
	var zeroRecord ingest.CodexRecord
	var evidence ingest.CodexCheckpointEvidence
	b := r.binding
	var start, end sql.NullInt64
	var digest []byte
	var kind, thread, turn, call, metadata sql.NullString
	args := append([]any{b.Round, b.Attempt, r.previousStart}, b.args()...)
	args = append(args, r.completeEnd)
	err := r.db.QueryRowContext(ctx, `
		SELECT r.start_byte,r.end_byte,
			CASE WHEN typeof(r.digest)='blob' AND length(r.digest)=32 THEN r.digest END,
			CASE WHEN typeof(r.kind)='text' AND length(CAST(r.kind AS BLOB))<=32 THEN r.kind END,
			CASE WHEN typeof(r.thread_id)='text' AND length(CAST(r.thread_id AS BLOB))<=128 THEN r.thread_id END,
			CASE WHEN typeof(r.turn_id)='text' AND length(CAST(r.turn_id AS BLOB))<=128 THEN r.turn_id END,
			CASE WHEN typeof(r.call_id)='text' AND length(CAST(r.call_id AS BLOB))<=128 THEN r.call_id END,
			CASE WHEN typeof(r.metadata)='text' AND length(CAST(r.metadata AS BLOB))<=8192 THEN r.metadata END
		FROM cjsi_checkpoint_attempts a LEFT JOIN cjsi_checkpoint_records r
		ON r.round_id=a.round_id AND r.attempt_id=a.attempt_id AND r.start_byte=(
			SELECT start_byte FROM cjsi_checkpoint_records WHERE round_id=? AND attempt_id=? AND start_byte>?
			ORDER BY start_byte LIMIT 1)
		WHERE a.round_id=? AND a.attempt_id=? AND a.identity=? AND a.generation=? AND a.parser=?
		AND a.max_line_bytes=? AND a.capture_end=? AND a.state='sealed' AND a.complete_end=?`, args...).Scan(
		&start, &end, &digest, &kind, &thread, &turn, &call, &metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return zeroRecord, evidence, errCJSICheckpointWorkset
	}
	if err != nil {
		return zeroRecord, evidence, err
	}
	if !start.Valid && r.cursor == r.completeEnd {
		return zeroRecord, evidence, io.EOF
	}
	if !start.Valid || !end.Valid || start.Int64 != r.cursor || end.Int64 <= r.cursor ||
		end.Int64 > r.completeEnd || len(digest) != len(evidence.Digest) ||
		!kind.Valid || !thread.Valid || !turn.Valid || !call.Valid || !metadata.Valid {
		return zeroRecord, evidence, errCJSICheckpointWorkset
	}
	span := ingest.CodexSpan{Start: start.Int64, End: end.Int64}
	record, err := decodeCJSICheckpointRecord(span, kind.String, thread.String, turn.String, call.String, metadata.String)
	if err != nil || (span.End-span.Start > int64(b.Version.MaxLineBytes)) != (record.Kind == ingest.CodexOversize) {
		return zeroRecord, evidence, errCJSICheckpointWorkset
	}
	evidence.Span = span
	copy(evidence.Digest[:], digest)
	if evidence.Digest == ([sha256.Size]byte{}) {
		return zeroRecord, ingest.CodexCheckpointEvidence{}, errCJSICheckpointWorkset
	}
	r.previousStart, r.cursor = span.Start, span.End
	return record, evidence, nil
}

func openCJSICheckpointResume(ctx context.Context, db *sql.DB, b cjsiCheckpointBinding) (*ingest.CodexCheckpointResume, error) {
	reader, err := openCJSICheckpointRecords(ctx, db, b)
	if err != nil {
		return nil, err
	}
	return &ingest.CodexCheckpointResume{
		Prefix: reader.Prefix(),
		Next: func(ctx context.Context) (ingest.CodexCheckpointEvidence, error) {
			_, evidence, err := reader.Next(ctx)
			return evidence, err
		},
	}, nil
}
