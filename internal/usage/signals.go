package usage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kitdine/agent-deck/internal/activity"
)

// Decision 11's two row states. `pending` is a turn that does not exist yet:
// the message was seen, no assistant call has followed it, and every aggregate
// in Decisions 4, 5 and 6 must ignore it. Counting it would inflate turn counts,
// and classifying it would mean guessing a tool shape that has not happened.
const (
	signalStatePending    = "pending"
	signalStateClassified = "classified"
)

// upsertWorkSignalTx writes a pending row, resolving a duplicate-source conflict
// the same way events and tool calls already do: the `source_path` that sorts
// last wins, and an existing owner yields only while it is still an indexed
// source. Signals do not get their own ownership policy — three tables
// disagreeing about which source owns a session is invisible from inside any one
// of them.
func upsertWorkSignalTx(ctx context.Context, tx *sql.Tx, signal turnSignal, path string) error {
	if signal.startedAt == "" || signal.session == "" {
		// Decision 11: a row is never written with a synthesized time, because
		// started_at is what first_edit_seconds measures from. The turn is
		// recorded directly as classified when its first assistant call arrives.
		return nil
	}
	var existingPath, existingState string
	var existingIndexed int
	lookupErr := tx.QueryRowContext(ctx, `SELECT w.source_path,w.state,CASE WHEN f.path IS NULL THEN 0 ELSE 1 END FROM usage_work_signals w LEFT JOIN usage_source_files f ON f.path=w.source_path WHERE w.client=? AND w.session_id=? AND w.turn_index=?`,
		signal.client, signal.session, signal.turnIndex).Scan(&existingPath, &existingState, &existingIndexed)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return lookupErr
	}
	if lookupErr == nil && existingIndexed == 1 && existingPath > path {
		return nil
	}
	// An already classified turn keeps its verdict: a later scan of the same
	// content re-derives it, and a message replayed after promotion must not
	// demote the turn back to pending.
	state := signalStatePending
	if lookupErr == nil && existingState == signalStateClassified && existingPath == path {
		state = signalStateClassified
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path) VALUES(?,?,?,?,?,?,?,'','',?) ON CONFLICT(client,session_id,turn_index) DO UPDATE SET started_at=excluded.started_at,state=CASE WHEN usage_work_signals.state=? AND usage_work_signals.source_path=excluded.source_path THEN usage_work_signals.state ELSE excluded.state END,message_class=excluded.message_class,intent_sub=excluded.intent_sub,source_path=excluded.source_path`,
		signal.client, signal.session, signal.turnIndex, signal.startedAt, state, signal.messageClass, signal.intentSub, path, signalStateClassified)
	return err
}

// classifySourceTurns recomputes every turn this source owns. A turn becomes
// `classified` once an assistant call has arrived, which is what a usage event
// carrying its turn_index records. The computation is a pure function of the
// stored message reduction and the turn's rows in usage_tool_calls, so running
// it again over unchanged content produces identical rows.
func classifySourceTurns(ctx context.Context, tx *sql.Tx, path string) error {
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,turn_index,message_class,intent_sub FROM usage_work_signals WHERE source_path=?`, path)
	if err != nil {
		return err
	}
	type pending struct {
		client, session         string
		turnIndex               int
		messageClass, intentSub string
	}
	var candidates []pending
	for rows.Next() {
		var candidate pending
		if err = rows.Scan(&candidate.client, &candidate.session, &candidate.turnIndex, &candidate.messageClass, &candidate.intentSub); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, candidate := range candidates {
		var assistantCalls int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE client=? AND session_id=? AND turn_index=?`,
			candidate.client, candidate.session, candidate.turnIndex).Scan(&assistantCalls); err != nil {
			return err
		}
		if assistantCalls == 0 {
			continue
		}
		shape, shapeErr := turnShape(ctx, tx, candidate.client, candidate.session, candidate.turnIndex)
		if shapeErr != nil {
			return shapeErr
		}
		// brainstorming is false here by construction: it is answered from the
		// message in hand, and Decision 2's persisted set carries message_class
		// and intent_sub and nothing else. A tool-less turn whose message and
		// reply fall in different scans takes the visible `exploration`
		// fallback. Widening the persisted set is a design change.
		kind, sub := activity.Classify(candidate.messageClass, candidate.intentSub, shape, false)
		if _, err = tx.ExecContext(ctx, `UPDATE usage_work_signals SET state=?,activity_kind=?,activity_sub=? WHERE client=? AND session_id=? AND turn_index=?`,
			signalStateClassified, kind, sub, candidate.client, candidate.session, candidate.turnIndex); err != nil {
			return err
		}
	}
	return nil
}

// turnShape reads a turn's tool shape back from usage_tool_calls rather than
// accumulating it across scans. Task 1 already persists every call with its
// turn_index, so only the message ever needed a home of its own. command_hint
// is read back the same way: it is the bounded fact commandHint reduced a
// command to, persisted on the tool row precisely so a turn split across a
// scan boundary still has it. codingSub applies Decision 3's precedence, so a
// message-derived intent set here would be redundant, not wrong.
func turnShape(ctx context.Context, tx *sql.Tx, client, session string, turnIndex int) (activity.TurnShape, error) {
	var shape activity.TurnShape
	rows, err := tx.QueryContext(ctx, `SELECT tool_name,tool_kind,command_hint FROM usage_tool_calls WHERE client=? AND session_id=? AND turn_index=?`, client, session, turnIndex)
	if err != nil {
		return shape, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, kind, commandHint string
		if err = rows.Scan(&name, &kind, &commandHint); err != nil {
			return shape, err
		}
		shape.AnyCall = true
		switch kind {
		case "edit":
			shape.Edited = true
		case "read":
			shape.Read = true
		}
		switch name {
		case "spawn_agent", "Task", "Agent":
			shape.Delegated = true
		case "Skill", "Workflow":
			shape.Workflow = true
		case "update_plan", "TodoWrite":
			shape.Planned = true
		}
		switch commandHint {
		case activity.HintTesting:
			shape.TestingCmd = true
		case activity.HintChore:
			shape.ChoreCmd = true
		}
	}
	return shape, rows.Err()
}
