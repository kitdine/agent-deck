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
// sourceTurnShapes aggregates each source's logical turns in one query.
// Tool ownership may differ from signal ownership, so the join deliberately
// uses client/session/turn rather than restricting tool.source_path.
const sourceTurnShapesSQL = `
WITH shapes AS MATERIALIZED (
 SELECT a.client,a.session_id,a.turn_index,
        1 AS any_call,
        MAX(a.tool_kind='edit') AS edited,
        MAX(a.tool_kind='read') AS read_call,
        MAX(a.tool_name IN ('spawn_agent','Task','Agent')) AS delegated,
        MAX(a.tool_name IN ('Skill','Workflow')) AS workflow,
        MAX(a.tool_name IN ('update_plan','TodoWrite')) AS planned,
        MAX(a.command_hint='testing') AS testing,
        MAX(a.command_hint='chore') AS chore
 FROM usage_tool_calls a CROSS JOIN usage_work_signals owned
 ON owned.client=a.client AND owned.session_id=a.session_id AND owned.turn_index=a.turn_index
 WHERE owned.source_path=?1
 GROUP BY a.client,a.session_id,a.turn_index
)
SELECT w.client,w.session_id,w.turn_index,w.message_class,w.intent_sub,
       EXISTS(SELECT 1 FROM usage_events e
              WHERE e.client=w.client AND e.session_id=w.session_id AND e.turn_index=w.turn_index),
       COALESCE(a.any_call,0),COALESCE(a.edited,0),COALESCE(a.read_call,0),
       COALESCE(a.delegated,0),COALESCE(a.workflow,0),COALESCE(a.planned,0),
       COALESCE(a.testing,0),COALESCE(a.chore,0)
FROM usage_work_signals w
LEFT JOIN shapes a
 ON a.client=w.client AND a.session_id=w.session_id AND a.turn_index=w.turn_index
WHERE w.source_path=?1`

func classifySourceTurns(ctx context.Context, tx *sql.Tx, path string) error {
	rows, err := tx.QueryContext(ctx, sourceTurnShapesSQL, path)
	if err != nil {
		return err
	}
	type classified struct {
		client, session string
		turn            int
		kind, sub       string
	}
	var results []classified
	for rows.Next() {
		var candidate classified
		var message, intent string
		var assistant bool
		var shape activity.TurnShape
		if err = rows.Scan(&candidate.client, &candidate.session, &candidate.turn, &message, &intent, &assistant,
			&shape.AnyCall, &shape.Edited, &shape.Read, &shape.Delegated, &shape.Workflow, &shape.Planned, &shape.TestingCmd, &shape.ChoreCmd); err != nil {
			rows.Close()
			return err
		}
		if !assistant {
			continue
		}
		candidate.kind, candidate.sub = activity.Classify(message, intent, shape, false)
		results = append(results, candidate)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}
	update, err := tx.PrepareContext(ctx, `UPDATE usage_work_signals SET state=?,activity_kind=?,activity_sub=?
 WHERE client=? AND session_id=? AND turn_index=?
 AND (state,activity_kind,activity_sub) IS NOT (?,?,?)`)
	if err != nil {
		return err
	}
	defer update.Close()
	for _, c := range results {
		if _, err = update.ExecContext(ctx, signalStateClassified, c.kind, c.sub, c.client, c.session, c.turn, signalStateClassified, c.kind, c.sub); err != nil {
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
