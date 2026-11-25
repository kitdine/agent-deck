package usage

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

type CJSIUsageDraftReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type CJSIUsageDraft struct {
	Thread   string
	Stamp    ledger.ProposalStamp
	Worksets []CJSIUsageWorkset
	// Independently registered selection, including rows that will be retained.
	// A nil slice means the selection is unavailable.
	ExpectedOldRows []ledger.OldRow
}
type CJSIUsageSelectDraft func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error)

func requireUsageRow(row *sql.Row) error {
	var ok int
	if err := row.Scan(&ok); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ledger.ErrProposalMismatch
		}
		return err
	}
	if ok != 1 {
		return ledger.ErrProposalMismatch
	}
	return nil
}

// checkDraft runs on the writing connection both for initial staging and for
// reuse. In particular the published head alone cannot vouch for the live epoch.
func (c CJSIUsageCandidate) checkDraft(ctx context.Context, tx *sql.Tx, d CJSIUsageDraft) error {
	return c.checkStagingInputs(ctx, tx, d, "a.state='draft' AND a.diff_digest IS NULL")
}
func (c CJSIUsageCandidate) checkStagingInputs(ctx context.Context, tx *sql.Tx, d CJSIUsageDraft, stateSQL string) error {
	st := c.proposal.Stamp()
	b := st.Basis
	if c.thread == "" || d.Stamp != st || d.Thread != c.thread || d.ExpectedOldRows == nil || len(c.old) != len(d.ExpectedOldRows) {
		return ledger.ErrProposalMismatch
	}
	err := requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_attempts a
 JOIN main.cjsi_publication_head h ON h.singleton=1 AND a.base_ticket=h.ticket AND a.core_epoch=h.core_epoch
 JOIN main.derived_snapshot_generation g ON g.singleton=1 AND g.epoch=h.core_epoch
 WHERE a.ticket=? AND a.attempt=? AND a.base_ticket=? AND a.core_epoch=? AND a.input_digest=? AND (`+stateSQL+`) AND julianday(a.accept_until)>julianday('now') AND EXISTS(SELECT 1 FROM main.cjsi_layout_journal WHERE phase=5)
 AND h.index_epoch=a.index_epoch AND h.session_revision=a.session_revision AND h.coverage_revision=a.coverage_revision AND h.protection_revision=a.protection_revision`, b.Ticket, b.Attempt, b.BaseTicket, b.CoreEpoch, b.InputDigest[:]))
	if err != nil {
		return err
	}
	if _, err = c.readOldRows(ctx, tx, d.ExpectedOldRows); err != nil {
		return err
	}
	for _, prior := range c.committed {
		current, err := ReadCJSICommittedCall(ctx, tx, prior.Selection, prior.Key)
		if errors.Is(err, sql.ErrNoRows) {
			return ledger.ErrProposalMismatch
		}
		if err != nil {
			return err
		}
		if current != prior {
			return ledger.ErrProposalMismatch
		}
	}
	ws := slices.Clone(d.Worksets)
	slices.SortFunc(ws, func(a, b CJSIUsageWorkset) int { return strings.Compare(a.Selected.Path, b.Selected.Path) })
	if !slices.Equal(ws, c.worksets) {
		return ledger.ErrProposalMismatch
	}
	for _, w := range ws {
		e := w.Selected.Evidence
		if err := requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_checkpoint_attempts WHERE round_id=? AND attempt_id=? AND state='sealed' AND identity=? AND generation=? AND parser=? AND max_line_bytes=? AND capture_end=? AND complete_end=?`, w.Round, w.Attempt, e.Identity, e.Version.Generation, e.Version.Parser, e.MaxLineBytes, e.Progress.ReadEnd, e.Progress.CompleteEnd)); err != nil {
			return err
		}
	}
	return nil
}

type usageStageTable struct {
	name, columns string
	rows          [][]any
}

func nullableDetail(n ledger.Counts, bit ledger.DetailMask, value int64) any {
	if n.Known&bit != 0 {
		return value
	}
	return nil
}
func ledgerAt(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func (c CJSIUsageCandidate) images(v ledger.Eligibility, d ledger.Delta) []usageStageTable {
	t := []usageStageTable{
		{"cjsi_usage_groups", "input_digest,diff_digest,state,gaps,pending,partial,unresolved_overlap", nil},
		{"cjsi_usage_sources", "name_digest,selection_epoch,workset_round,workset_attempt,identity,generation,replay_parser,max_line_bytes,read_end,complete_end,prefix_digest", nil},
		{"cjsi_usage_members", "kind,member_id,turn_id,model,event_at,exact_run,input_tokens,output_tokens,cached,cache_write,reasoning", nil},
		{"cjsi_usage_intervals", "kind,member_id,counter_epoch,lower_at,upper_at,lower_input,upper_input,lower_output,upper_output,lower_cached,upper_cached,lower_write,upper_write,lower_reasoning,upper_reasoning", nil},
		{"cjsi_usage_references", "kind,member_id,ordinal,name_digest,start_offset,end_offset", nil},
		{"cjsi_old_preconditions", "event_key,row_hash,disposition", nil},
	}
	add := func(i int, values ...any) { t[i].rows = append(t[i].rows, values) }
	st := v.Stamp
	add(0, st.Basis.InputDigest[:], st.Diff[:], v.State, v.Gaps, v.Pending, v.Partial, v.UnresolvedOverlap)
	for _, w := range c.worksets {
		s := ledger.SelectionStamp(w.Selected)
		e := s.Prefix
		add(1, s.Name[:], s.Epoch, w.Round, w.Attempt, e.Identity, e.Version.Generation, e.Version.Parser, e.MaxLineBytes, e.Progress.ReadEnd, e.Progress.CompleteEnd, e.Digest[:])
	}
	for _, m := range v.Members {
		n := m.Tokens
		k, id := m.Key.Kind, m.Key.ID
		var run any
		if m.ExactRun > 0 {
			run = m.ExactRun
		}
		add(2, k, id, m.TurnID, m.Model, ledgerAt(m.At), run, n.Input, n.Output, nullableDetail(n, ledger.CachedKnown, n.Cached), nullableDetail(n, ledger.WriteKnown, n.Write), nullableDetail(n, ledger.ReasoningKnown, n.Reasoning))
		if k == ledger.TokenInterval {
			i := m.Interval
			lo, hi := i.Lower, i.Upper
			add(3, k, id, i.Epoch[:], ledgerAt(i.LowerAt), ledgerAt(i.UpperAt), lo.Input, hi.Input, lo.Output, hi.Output, nullableDetail(lo, ledger.CachedKnown, lo.Cached), nullableDetail(hi, ledger.CachedKnown, hi.Cached), nullableDetail(lo, ledger.WriteKnown, lo.Write), nullableDetail(hi, ledger.WriteKnown, hi.Write), nullableDetail(lo, ledger.ReasoningKnown, lo.Reasoning), nullableDetail(hi, ledger.ReasoningKnown, hi.Reasoning))
		}
		for j, r := range m.Proof {
			add(4, k, id, j, r.Source.Name[:], r.Span.Start, r.Span.End)
		}
	}
	for _, r := range d.Hold {
		add(5, r.EventKey, r.RowHash[:], "hold")
	}
	for _, r := range d.Retract {
		add(5, r.EventKey, r.RowHash[:], "retract")
	}
	// An independently retained, already-opened response keeps its old
	// identity receipt. This is not a new member or an ownership proposal.
	if v.Gaps&ledger.OldSideHeld == 0 {
		removed := make(map[string]bool, len(d.Retract))
		for _, r := range d.Retract {
			removed[r.EventKey] = true
		}
		for _, prior := range c.committed {
			r := prior.Prior
			if !removed[r.EventKey] {
				add(5, r.EventKey, r.RowHash[:], "hold")
			}
		}
	}
	return t
}
func (c CJSIUsageCandidate) Stage(ctx context.Context, db *sql.DB, selectDraft CJSIUsageSelectDraft) error {
	if selectDraft == nil || db == nil {
		return ledger.ErrProposalMismatch
	}
	tx, release, err := beginSourcePublication(ctx, db)
	if err != nil {
		return err
	}
	defer release()
	if err = requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.schema_metadata WHERE version=37 AND (SELECT foreign_keys FROM pragma_foreign_keys)=1`)); err != nil {
		return err
	}
	d, err := selectDraft(ctx, tx, c.thread)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.ErrProposalMismatch
	}
	if err != nil {
		return err
	}
	if err = c.checkDraft(ctx, tx, d); err != nil {
		return err
	}
	base := []any{d.Stamp.Basis.Ticket, d.Stamp.Basis.Attempt, c.thread}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM main.cjsi_usage_groups"+usageStagedWhere+")", base...).Scan(&exists); err != nil {
		return err
	}
	if exists {
		if err = c.verifyStagedRows(ctx, tx, d); err != nil {
			return err
		}
	} else {
		tables, err := c.stagedImages(d)
		if err != nil {
			return err
		}
		for _, t := range tables {
			columns := "ticket,attempt,thread," + t.columns
			placeholders := strings.TrimSuffix(strings.Repeat("?,", strings.Count(columns, ",")+1), ",")
			query := "INSERT INTO main." + t.name + "(" + columns + ") VALUES(" + placeholders + ")"
			for _, row := range t.rows {
				if _, err = tx.ExecContext(ctx, query, append(slices.Clip(base), row...)...); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

const usageStagedWhere = " WHERE ticket=? AND attempt=? AND thread=?"

func (c CJSIUsageCandidate) stagedImages(d CJSIUsageDraft) ([]usageStageTable, error) {
	v, err := c.proposal.Eligibility(d.Stamp, c.thread)
	if err != nil {
		return nil, err
	}
	delta, err := c.proposal.Changes(d.Stamp, c.thread)
	if err != nil {
		return nil, err
	}
	return c.images(v, delta), nil
}

// VerifyStaged checks the independently prepared candidate and exact old rows
// on the caller's snapshot. It never stages, freezes, or commits a transaction.
func (c CJSIUsageCandidate) VerifyStaged(ctx context.Context, tx *sql.Tx, d CJSIUsageDraft) error {
	if tx == nil {
		return ledger.ErrProposalMismatch
	}
	if err := requireUsageRow(tx.QueryRowContext(ctx, "SELECT 1 FROM main.schema_metadata WHERE version=37 AND (SELECT foreign_keys FROM pragma_foreign_keys)=1")); err != nil {
		return err
	}
	if err := c.checkStagingInputs(ctx, tx, d, "a.state='draft' OR (a.state='sealed' AND a.diff_digest IS NOT NULL AND a.session_seal IS NOT NULL)"); err != nil {
		return err
	}
	return c.verifyStagedRows(ctx, tx, d)
}

func (c CJSIUsageCandidate) verifyStagedRows(ctx context.Context, tx *sql.Tx, d CJSIUsageDraft) error {
	tables, err := c.stagedImages(d)
	if err != nil {
		return err
	}
	base := []any{d.Stamp.Basis.Ticket, d.Stamp.Basis.Attempt, c.thread}
	for _, t := range tables {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM main."+t.name+usageStagedWhere, base...).Scan(&n); err != nil {
			return err
		}
		if n != len(t.rows) {
			return ledger.ErrProposalMismatch
		}
		names := strings.Split("ticket,attempt,thread,"+t.columns, ",")
		query := "SELECT 1 FROM main." + t.name + " WHERE " + strings.Join(names, " IS ? AND ") + " IS ?"
		for _, row := range t.rows {
			if err := requireUsageRow(tx.QueryRowContext(ctx, query, append(slices.Clip(base), row...)...)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c CJSIUsageCandidate) readOldRows(ctx context.Context, tx *sql.Tx, registered []ledger.OldRow) (map[string]ledger.OldImage, error) {
	images := make(map[string]ledger.OldImage, len(c.old))
	if registered == nil || len(registered) != len(c.old) {
		return nil, ledger.ErrProposalMismatch
	}
	selected := map[string]ledger.OldRow{}
	for _, row := range registered {
		if row.EventKey == "" || row.RowHash == ([32]byte{}) {
			return nil, ledger.ErrProposalMismatch
		}
		if _, dup := selected[row.EventKey]; dup {
			return nil, ledger.ErrProposalMismatch
		}
		selected[row.EventKey] = row
	}
	for _, row := range c.old {
		if s, ok := selected[row.EventKey]; !ok || row != s {
			return nil, ledger.ErrProposalMismatch
		}
		delete(selected, row.EventKey)
		image, err := ReadCJSIOldUsage(ctx, tx, row.EventKey)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ledger.ErrProposalMismatch
		}
		if err != nil {
			return nil, err
		}
		images[row.EventKey] = image
		if image.Precondition() != row {
			return nil, ledger.ErrProposalMismatch
		}
	}
	return images, nil
}
