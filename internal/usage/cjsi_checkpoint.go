package usage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Progress.ReadEnd may include an unfinished line; CompleteEnd and ReplayFrom
// are independently authenticated. Coverage may only advance with the final
// writer's decision. The legacy prefix_hash remains a suffix anchor for runs.
type CJSIUsageCheckpoint struct {
	Entry         InventoryEntry
	Workset       CJSIUsageWorkset
	Progress      ingest.CodexCheckpoint
	HeadAt, Birth time.Time
	Anchor        [32]byte
}

func (c CJSIUsageCandidate) Checkpoints() []CJSIUsageCheckpoint { return slices.Clone(c.checkpoints) }
func (c CJSIUsageCandidate) checkpointStart() (string, error) {
	var headBirth, executionBirth time.Time
	for _, p := range c.checkpoints {
		e, s := p.Entry, p.Workset.Selected
		if s.ThreadID != c.thread || s.Evidence.Progress.ReadEnd != p.Progress.ReadEnd || s.Evidence.Progress.CompleteEnd != p.Progress.CompleteEnd || p.Progress.ReadEnd != e.Size {
			return "", ledger.ErrProposalMismatch
		}
		if !p.HeadAt.IsZero() && (headBirth.IsZero() || p.HeadAt.Before(headBirth)) {
			headBirth = p.HeadAt
		}
		if !p.Birth.IsZero() && (executionBirth.IsZero() || p.Birth.Before(executionBirth)) {
			executionBirth = p.Birth
		}
	}
	if headBirth.IsZero() {
		headBirth = executionBirth
	}
	if headBirth.IsZero() {
		return "", nil
	}
	return headBirth.UTC().Format(sessionStartLayout), nil
}

func (c CJSIUsageCandidate) writeCheckpoints(ctx context.Context, tx *sql.Tx) error {
	started, err := c.checkpointStart()
	if err != nil {
		return err
	}
	for _, p := range c.checkpoints {
		e := p.Entry
		_, err := tx.ExecContext(ctx, `INSERT INTO main.usage_source_files
  (path,identity,size,cursor,prefix_hash,session_id,turn_id,model,session_started_at,parser_version,codex_cumulative_json,modified_at,changed_at,thread_id)
  VALUES(?,?,?,?,?,?,'','',?,?,'{}',?,?,?)
  ON CONFLICT(path) DO UPDATE SET identity=excluded.identity,size=excluded.size,cursor=excluded.cursor,prefix_hash=excluded.prefix_hash,
  session_id=excluded.session_id,turn_id='',model='',session_started_at=excluded.session_started_at,parser_version=excluded.parser_version,
  codex_cumulative_json='{}',modified_at=excluded.modified_at,changed_at=excluded.changed_at,thread_id=excluded.thread_id`, e.Path, e.Identity, e.Size, p.Progress.CompleteEnd, hex.EncodeToString(p.Anchor[:]), c.thread, started, ledger.Parser, e.ModifiedAt, e.ChangedAt, c.thread)
		if err != nil {
			return err
		}
	}
	return nil
}
