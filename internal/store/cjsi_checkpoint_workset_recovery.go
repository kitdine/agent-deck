package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// A coordinator that already owns this source round can discover its one
// draft after restart. Production ownership/lease checks remain with it.
type cjsiCheckpointDraftOwner struct {
	Round    int64
	Identity string
	Version  ingest.CodexCheckpointVersion
}

type cjsiCheckpointDraft struct {
	binding     cjsiCheckpointBinding
	completeEnd int64 // Durable watermark only; not resume evidence.
}

func findCJSICheckpointDraft(ctx context.Context, db *sql.DB, owner cjsiCheckpointDraftOwner) (draft cjsiCheckpointDraft, found bool, err error) {
	b := cjsiCheckpointBinding{Round: owner.Round, Attempt: 1, Identity: owner.Identity, Version: owner.Version}
	if db == nil || !b.valid() {
		return draft, false, errCJSICheckpointWorkset
	}
	// The one-draft partial index bounds this query to at most one row.
	err = db.QueryRowContext(ctx, `SELECT attempt_id,capture_end,complete_end FROM cjsi_checkpoint_attempts
		WHERE round_id=? AND identity=? AND generation=? AND parser=? AND max_line_bytes=? AND state='draft'`,
		owner.Round, owner.Identity, owner.Version.Generation, owner.Version.Parser, owner.Version.MaxLineBytes).
		Scan(&b.Attempt, &b.CaptureEnd, &draft.completeEnd)
	if errors.Is(err, sql.ErrNoRows) {
		return cjsiCheckpointDraft{}, false, nil
	}
	if err != nil {
		return cjsiCheckpointDraft{}, false, err
	}
	if !b.valid() || draft.completeEnd < 0 || draft.completeEnd > b.CaptureEnd {
		return cjsiCheckpointDraft{}, false, errCJSICheckpointWorkset
	}
	draft.binding = b
	return draft, true, nil
}

// discardCJSICheckpointDraft abandons an observed draft using its exact binding
// and watermark. The write must lose to a seal or newer progress, and an
// aborted attempt keeps its tombstone. No draft is resumed as a writer.
func discardCJSICheckpointDraft(ctx context.Context, db *sql.DB, draft cjsiCheckpointDraft) error {
	return discardCJSICheckpointAttempt(ctx, db, draft.binding, &draft.completeEnd, "draft")
}
