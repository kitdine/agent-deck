package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// TelemetrySourcePrefix marks usage facts and session-index sources that the
// OTel reconciler owns. They are never files: inventory and orphan cleanup
// skip them, and they never count as JSONL evidence.
const TelemetrySourcePrefix = "otel:"

// TelemetrySessionSourceKind is the verifiable source marker of a session
// whose usage exists only in OTel (a Codex background request).
const TelemetrySessionSourceKind = "codex-otel-background"

// Telemetry storage keeps sanitized OTel observations beside the canonical
// usage facts. Source ownership is decided per conversation: a conversation
// with JSONL evidence is jsonl_owned and OTel never counts it; a supported
// conversation without JSONL evidence becomes otel_owned once settled, and
// each of its metered observations contributes one usage_events row.
func initializeTelemetryStorage(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE telemetry_observations (
 observation_key TEXT PRIMARY KEY,
 source TEXT NOT NULL CHECK(source IN ('receiver','import')),
 producer TEXT NOT NULL, producer_version TEXT NOT NULL, originator TEXT NOT NULL,
 event_name TEXT NOT NULL, event_kind TEXT NOT NULL, transport TEXT NOT NULL,
 record_at TEXT NOT NULL, observed_at TEXT NOT NULL, received_at TEXT NOT NULL,
 model TEXT NOT NULL, response_id TEXT NOT NULL, request_id TEXT NOT NULL,
 attempt_id TEXT NOT NULL, thread_id TEXT NOT NULL, source_class TEXT NOT NULL,
 input_tokens INTEGER, output_tokens INTEGER, cached_tokens INTEGER,
 cache_write_tokens INTEGER, reasoning_tokens INTEGER, total_tokens INTEGER,
 metered INTEGER NOT NULL CHECK(metered IN (0,1)),
 support_entry TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('capture_only','pending','jsonl_owned','otel_owned','conflict','invalid','unmetered')),
 reason TEXT NOT NULL
);
CREATE INDEX telemetry_observations_thread ON telemetry_observations(thread_id, state);
CREATE INDEX telemetry_observations_state ON telemetry_observations(state);
CREATE TABLE telemetry_contributions (
 observation_key TEXT PRIMARY KEY REFERENCES telemetry_observations(observation_key),
 event_key TEXT NOT NULL UNIQUE
);
CREATE TABLE telemetry_sessions (
 client TEXT NOT NULL, session_id TEXT NOT NULL,
 source_kind TEXT NOT NULL, model TEXT NOT NULL,
 first_at TEXT NOT NULL, last_at TEXT NOT NULL,
 support_entry TEXT NOT NULL, settled_at TEXT NOT NULL,
 PRIMARY KEY(client, session_id)
);
CREATE TABLE telemetry_journal_cursor (
 singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
 segment INTEGER NOT NULL, offset INTEGER NOT NULL
);
INSERT INTO telemetry_journal_cursor(singleton, segment, offset) VALUES (1, 0, 0);
CREATE TABLE telemetry_coverage (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 kind TEXT NOT NULL CHECK(kind IN ('listening','gap')),
 started_at TEXT NOT NULL, ended_at TEXT NOT NULL, reason TEXT NOT NULL
);
-- A late JSONL fact for an otel_owned conversation retracts every OTel
-- contribution, its session record and marker in the transaction that writes
-- the JSONL fact, so no committed state holds two contributions.
CREATE TRIGGER telemetry_jsonl_retracts_otel AFTER INSERT ON usage_events
WHEN NEW.client = 'codex' AND substr(NEW.source_path, 1, 5) <> 'otel:'
 AND EXISTS (SELECT 1 FROM telemetry_sessions WHERE client = 'codex' AND session_id = NEW.session_id)
BEGIN
 DELETE FROM telemetry_contributions WHERE observation_key IN
  (SELECT observation_key FROM telemetry_observations WHERE thread_id = NEW.session_id);
 UPDATE telemetry_observations SET state = 'jsonl_owned', reason = 'jsonl_evidence'
  WHERE thread_id = NEW.session_id AND state IN ('pending','otel_owned');
 DELETE FROM usage_events WHERE client = 'codex' AND session_id = NEW.session_id AND substr(source_path, 1, 5) = 'otel:';
 DELETE FROM telemetry_sessions WHERE client = 'codex' AND session_id = NEW.session_id;
 UPDATE usage_sessions SET
  first_at = (SELECT min(event_at) FROM usage_events WHERE client = 'codex' AND session_id = NEW.session_id),
  last_at = (SELECT max(event_at) FROM usage_events WHERE client = 'codex' AND session_id = NEW.session_id)
  WHERE client = 'codex' AND session_id = NEW.session_id;
END;`)
	return err
}

// TelemetryObservation is one sanitized, allowlisted record. Token counts are
// nil when the producer did not send them; nil is never read as zero.
type TelemetryObservation struct {
	Key              string
	Source           string
	Producer         string
	ProducerVersion  string
	Originator       string
	EventName        string
	EventKind        string
	Transport        string
	RecordAt         string
	ObservedAt       string
	ReceivedAt       string
	Model            string
	ResponseID       string
	RequestID        string
	AttemptID        string
	ThreadID         string
	SourceClass      string
	InputTokens      *int64
	OutputTokens     *int64
	CachedTokens     *int64
	CacheWriteTokens *int64
	ReasoningTokens  *int64
	TotalTokens      *int64
	Metered          bool
	// SupportEntry names the verified support-table entry, empty when the
	// producer, version, transport or event shape is not supported.
	SupportEntry string
	State        string
	Reason       string
}

// TelemetryCursor is the first journal byte not yet normalized.
type TelemetryCursor struct {
	Segment int64
	Offset  int64
}

// TelemetryApplyResult counts what one batch did to the observation table.
type TelemetryApplyResult struct {
	Inserted    int
	Duplicate   int
	Pending     int
	CaptureOnly int
	Conflict    int
	Invalid     int
	Unmetered   int
	// Changed includes contributions retracted while normalizing conflicts.
	Changed bool
}

const TelemetryIncompleteCountsReason = "token_components_missing"

// ErrTelemetryCursorMoved reports that another consumer advanced the journal
// cursor first; the caller rereads the cursor instead of applying twice.
var ErrTelemetryCursorMoved = errors.New("telemetry journal cursor moved")

// ApplyTelemetryObservations stores a batch of observations idempotently and,
// when from is set, advances the journal cursor in the same transaction. A
// replayed observation key is a no-op. A metered observation that disagrees
// with another metered observation of the same conversation and record time
// marks both as conflict; neither value is preferred or counted.
func (s *Store) ApplyTelemetryObservations(ctx context.Context, batch []TelemetryObservation, from, to *TelemetryCursor) (TelemetryApplyResult, error) {
	var result TelemetryApplyResult
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	restoreCoverage, err := PreserveSourceCoverage(ctx, tx)
	if err != nil {
		return result, err
	}
	if from != nil {
		var current TelemetryCursor
		if err := tx.QueryRowContext(ctx, `SELECT segment, offset FROM telemetry_journal_cursor WHERE singleton = 1`).Scan(&current.Segment, &current.Offset); err != nil {
			return result, err
		}
		if current != *from {
			return result, ErrTelemetryCursorMoved
		}
	}
	for _, observation := range batch {
		if (observation.Metered || observation.State == "pending" || observation.State == "otel_owned") && (observation.InputTokens == nil || observation.OutputTokens == nil) {
			observation.Metered, observation.SupportEntry = false, ""
			observation.State, observation.Reason = "capture_only", TelemetryIncompleteCountsReason
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM telemetry_observations WHERE observation_key = ?`, observation.Key).Scan(&exists); err != nil {
			return result, err
		}
		if exists != 0 {
			result.Duplicate++
			continue
		}
		if observation.Metered && observation.ThreadID != "" && observation.RecordAt != "" {
			conflict, err := telemetryConflicts(ctx, tx, observation)
			if err != nil {
				return result, err
			}
			if conflict {
				observation.State, observation.Reason = "conflict", "token_counts_disagree"
				var counted bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_contributions c JOIN telemetry_observations o USING(observation_key) WHERE o.thread_id=? AND o.record_at=?)`, observation.ThreadID, observation.RecordAt).Scan(&counted); err != nil {
					return result, err
				}
				if err := retractConflicting(ctx, tx, observation); err != nil {
					return result, err
				}
				result.Changed = result.Changed || counted
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO telemetry_observations(
 observation_key, source, producer, producer_version, originator, event_name, event_kind, transport,
 record_at, observed_at, received_at, model, response_id, request_id, attempt_id, thread_id, source_class,
 input_tokens, output_tokens, cached_tokens, cache_write_tokens, reasoning_tokens, total_tokens,
 metered, support_entry, state, reason) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			observation.Key, observation.Source, observation.Producer, observation.ProducerVersion, observation.Originator,
			observation.EventName, observation.EventKind, observation.Transport, observation.RecordAt, observation.ObservedAt,
			observation.ReceivedAt, observation.Model, observation.ResponseID, observation.RequestID, observation.AttemptID,
			observation.ThreadID, observation.SourceClass, nullableInt(observation.InputTokens), nullableInt(observation.OutputTokens),
			nullableInt(observation.CachedTokens), nullableInt(observation.CacheWriteTokens), nullableInt(observation.ReasoningTokens),
			nullableInt(observation.TotalTokens), boolInt(observation.Metered), observation.SupportEntry, observation.State, observation.Reason); err != nil {
			return result, err
		}
		result.Inserted++
		switch observation.State {
		case "pending":
			result.Pending++
		case "capture_only":
			result.CaptureOnly++
		case "conflict":
			result.Conflict++
		case "invalid":
			result.Invalid++
		default:
			result.Unmetered++
		}
	}
	if to != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE telemetry_journal_cursor SET segment = ?, offset = ? WHERE singleton = 1`, to.Segment, to.Offset); err != nil {
			return result, err
		}
	}
	if err := restoreCoverage(); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

// TelemetryNeedsInventory reports pending observations that no completed finite
// inventory has checked yet. It does not grant ownership or bypass settlement.
func (s *Store) TelemetryNeedsInventory(ctx context.Context) (bool, error) {
	var needed bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_observations o WHERE o.state='pending' AND NOT EXISTS(SELECT 1 FROM inventory_runs i WHERE i.state='complete' AND i.captured_at > o.received_at))`).Scan(&needed)
	return needed, err
}

func telemetryConflicts(ctx context.Context, tx *sql.Tx, observation TelemetryObservation) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT input_tokens, output_tokens, cached_tokens, cache_write_tokens, reasoning_tokens, total_tokens
 FROM telemetry_observations WHERE thread_id = ? AND record_at = ? AND metered = 1 AND state NOT IN ('invalid','unmetered')`, observation.ThreadID, observation.RecordAt)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	mine := []*int64{observation.InputTokens, observation.OutputTokens, observation.CachedTokens, observation.CacheWriteTokens, observation.ReasoningTokens, observation.TotalTokens}
	for rows.Next() {
		values := make([]sql.NullInt64, 6)
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5]); err != nil {
			return false, err
		}
		for i, value := range values {
			if value.Valid != (mine[i] != nil) || (value.Valid && value.Int64 != *mine[i]) {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

// retractConflicting marks the disagreeing observations conflict and removes
// any contribution one of them already made.
func retractConflicting(ctx context.Context, tx *sql.Tx, observation TelemetryObservation) error {
	keys, err := queryStrings(ctx, tx, `SELECT observation_key FROM telemetry_observations WHERE thread_id = ? AND record_at = ? AND metered = 1 AND state NOT IN ('invalid','unmetered')`, observation.ThreadID, observation.RecordAt)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := deleteContribution(ctx, tx, key); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state = 'conflict', reason = 'token_counts_disagree' WHERE observation_key = ?`, key); err != nil {
			return err
		}
	}
	if len(keys) > 0 {
		return syncTelemetrySession(ctx, tx, observation.ThreadID, time.Now())
	}
	return nil
}

func deleteContribution(ctx context.Context, tx *sql.Tx, observationKey string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_events WHERE event_key = (SELECT event_key FROM telemetry_contributions WHERE observation_key = ?)`, observationKey); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM telemetry_contributions WHERE observation_key = ?`, observationKey)
	return err
}

// TelemetrySettleResult reports what one settle pass changed.
type TelemetrySettleResult struct {
	Owned     int
	Retracted int
	Restored  int
	Changed   bool
}

// SettleTelemetry re-evaluates conversation ownership. It only accepts JSONL
// evidence from local logs themselves: a non-otel usage_events row of the
// conversation, or a committed Codex source file with that session id. Rows
// this reconciler wrote are never evidence, so the result is the same after
// any number of passes, restarts or rebuilds. A pending observation settles
// once it is at least settleAfter old and a complete inventory was captured
// after it was received. Missing contributions of otel_owned observations,
// such as after a rebuild, are restored.
func (s *Store) SettleTelemetry(ctx context.Context, now time.Time, settleAfter time.Duration) (TelemetrySettleResult, error) {
	var result TelemetrySettleResult
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	restoreCoverage, err := PreserveSourceCoverage(ctx, tx)
	if err != nil {
		return result, err
	}
	result.Retracted, err = retractIncompleteTelemetry(ctx, tx, now)
	if err != nil {
		return result, err
	}
	result.Changed = result.Retracted > 0
	var inventoryAt sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT max(captured_at) FROM inventory_runs WHERE state IN ('complete','update_pending')`).Scan(&inventoryAt); err != nil {
		return result, err
	}
	threads, err := queryStrings(ctx, tx, `SELECT DISTINCT thread_id FROM telemetry_observations WHERE thread_id <> '' AND state IN ('pending','otel_owned')`)
	if err != nil {
		return result, err
	}
	cutoff := formatTelemetryTime(now.Add(-settleAfter))
	for _, thread := range threads {
		var evidence int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_events WHERE client = 'codex' AND session_id = ? AND substr(source_path, 1, 5) <> 'otel:')
 OR EXISTS(SELECT 1 FROM usage_source_files WHERE session_id = ? AND substr(path, 1, 5) <> 'otel:')`, thread, thread).Scan(&evidence); err != nil {
			return result, err
		}
		if evidence != 0 {
			keys, err := queryStrings(ctx, tx, `SELECT observation_key FROM telemetry_observations WHERE thread_id = ? AND state IN ('pending','otel_owned')`, thread)
			if err != nil {
				return result, err
			}
			for _, key := range keys {
				if err := deleteContribution(ctx, tx, key); err != nil {
					return result, err
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state = 'jsonl_owned', reason = 'jsonl_evidence' WHERE thread_id = ? AND state IN ('pending','otel_owned')`, thread); err != nil {
				return result, err
			}
			if err := syncTelemetrySession(ctx, tx, thread, now); err != nil {
				return result, err
			}
			result.Retracted++
			result.Changed = true
			continue
		}
		if inventoryAt.Valid {
			settled, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state = 'otel_owned', reason = 'no_jsonl_evidence'
 WHERE thread_id = ? AND state = 'pending' AND received_at <= ? AND received_at < ?`, thread, cutoff, inventoryAt.String)
			if err != nil {
				return result, err
			}
			if n, _ := settled.RowsAffected(); n > 0 {
				result.Owned += int(n)
				result.Changed = true
			}
		}
		restored, err := ensureContributions(ctx, tx, thread)
		if err != nil {
			return result, err
		}
		if restored > 0 {
			result.Restored += restored
			result.Changed = true
		}
		if err := syncTelemetrySession(ctx, tx, thread, now); err != nil {
			return result, err
		}
	}
	if err := restoreCoverage(); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

// Older receivers could store or count incomplete observations. Reclassify
// those persisted records before ownership/restoration so no replay restores a
// fabricated zero. Only actual removed contributions report a counted change.
func retractIncompleteTelemetry(ctx context.Context, tx *sql.Tx, now time.Time) (int, error) {
	const incomplete = `state IN ('pending','otel_owned') AND (input_tokens IS NULL OR output_tokens IS NULL)`
	threads, err := queryStrings(ctx, tx, `SELECT DISTINCT thread_id FROM telemetry_observations WHERE `+incomplete)
	if err != nil {
		return 0, err
	}
	retracted := 0
	for _, thread := range threads {
		keys, err := queryStrings(ctx, tx, `SELECT observation_key FROM telemetry_observations WHERE thread_id=? AND `+incomplete, thread)
		if err != nil {
			return 0, err
		}
		var counted bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_observations o JOIN telemetry_contributions c USING(observation_key) JOIN usage_events e ON e.event_key=c.event_key WHERE o.thread_id=? AND o.state IN ('pending','otel_owned') AND (o.input_tokens IS NULL OR o.output_tokens IS NULL))`, thread).Scan(&counted); err != nil {
			return 0, err
		}
		for _, key := range keys {
			if err := deleteContribution(ctx, tx, key); err != nil {
				return 0, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state='capture_only',reason=?,metered=0,support_entry='' WHERE thread_id=? AND `+incomplete, TelemetryIncompleteCountsReason, thread); err != nil {
			return 0, err
		}
		if err := syncTelemetrySession(ctx, tx, thread, now); err != nil {
			return 0, err
		}
		if counted {
			retracted++
		}
	}
	return retracted, nil
}

// ensureContributions writes the usage_events row of every otel_owned
// observation of a conversation that does not have one.
func ensureContributions(ctx context.Context, tx *sql.Tx, thread string) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT o.observation_key, o.record_at, o.received_at, o.model, o.input_tokens, o.cached_tokens, o.output_tokens, o.cache_write_tokens
 FROM telemetry_observations o WHERE o.thread_id = ? AND o.state = 'otel_owned' AND o.input_tokens IS NOT NULL AND o.output_tokens IS NOT NULL
 AND NOT EXISTS (SELECT 1 FROM telemetry_contributions c JOIN usage_events e ON e.event_key = c.event_key WHERE c.observation_key = o.observation_key)`, thread)
	if err != nil {
		return 0, err
	}
	type missingContribution struct {
		key, recordAt, receivedAt, model  string
		input, cached, output, cacheWrite sql.NullInt64
	}
	var missing []missingContribution
	for rows.Next() {
		var item missingContribution
		if err := rows.Scan(&item.key, &item.recordAt, &item.receivedAt, &item.model, &item.input, &item.cached, &item.output, &item.cacheWrite); err != nil {
			rows.Close()
			return 0, err
		}
		missing = append(missing, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, item := range missing {
		eventKey := TelemetrySourcePrefix + item.key
		at := item.recordAt
		if at == "" {
			at = item.receivedAt
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_write_tokens,source_path,source_offset)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,0) ON CONFLICT(event_key) DO NOTHING`,
			eventKey, "codex", thread, item.key, at, item.model, item.input.Int64, item.cached.Int64, item.output.Int64, item.cacheWrite.Int64, eventKey); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO telemetry_contributions(observation_key, event_key) VALUES(?, ?) ON CONFLICT(observation_key) DO UPDATE SET event_key = excluded.event_key`, item.key, eventKey); err != nil {
			return 0, err
		}
	}
	return len(missing), nil
}

// syncTelemetrySession makes usage_sessions and telemetry_sessions describe
// exactly the conversation's current OTel contributions: present with their
// first/last times while any exist, absent otherwise. A usage_sessions row
// that JSONL events also feed is left to the JSONL path.
func syncTelemetrySession(ctx context.Context, tx *sql.Tx, thread string, now time.Time) error {
	var count int
	var first, last, model sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT count(*), min(event_at), max(event_at),
 (SELECT model FROM usage_events WHERE client = 'codex' AND session_id = ? AND substr(source_path, 1, 5) = 'otel:' ORDER BY event_at DESC, event_key DESC LIMIT 1)
 FROM usage_events WHERE client = 'codex' AND session_id = ? AND substr(source_path, 1, 5) = 'otel:'`, thread, thread).Scan(&count, &first, &last, &model); err != nil {
		return err
	}
	var jsonl int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_events WHERE client = 'codex' AND session_id = ? AND substr(source_path, 1, 5) <> 'otel:')`, thread).Scan(&jsonl); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM telemetry_sessions WHERE client = 'codex' AND session_id = ?`, thread); err != nil {
			return err
		}
		if jsonl == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM usage_sessions WHERE client = 'codex' AND session_id = ?`, thread)
			return err
		}
		return nil
	}
	var support string
	if err := tx.QueryRowContext(ctx, `SELECT support_entry FROM telemetry_observations WHERE thread_id = ? AND state = 'otel_owned' ORDER BY record_at DESC LIMIT 1`, thread).Scan(&support); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telemetry_sessions(client, session_id, source_kind, model, first_at, last_at, support_entry, settled_at)
 VALUES('codex', ?, ?, ?, ?, ?, ?, ?)
 ON CONFLICT(client, session_id) DO UPDATE SET model = excluded.model, first_at = excluded.first_at, last_at = excluded.last_at, support_entry = excluded.support_entry
 WHERE (telemetry_sessions.model, telemetry_sessions.first_at, telemetry_sessions.last_at, telemetry_sessions.support_entry) IS NOT (excluded.model, excluded.first_at, excluded.last_at, excluded.support_entry)`,
		thread, TelemetrySessionSourceKind, model.String, first.String, last.String, support, formatTelemetryTime(now)); err != nil {
		return err
	}
	if jsonl != 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_sessions(client, session_id, first_at, last_at) VALUES('codex', ?, ?, ?)
 ON CONFLICT(client, session_id) DO UPDATE SET first_at = excluded.first_at, last_at = excluded.last_at
 WHERE (usage_sessions.first_at, usage_sessions.last_at) IS NOT (excluded.first_at, excluded.last_at)`, thread, first.String, last.String)
	return err
}

// TelemetrySession is one OTel-owned background conversation.
type TelemetrySession struct {
	Client     string
	SessionID  string
	SourceKind string
	Model      string
	FirstAt    string
	LastAt     string
}

// TelemetrySessions lists every OTel-owned conversation, the input of the
// session-index projection.
func (s *Store) TelemetrySessions(ctx context.Context) ([]TelemetrySession, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT client, session_id, source_kind, model, first_at, last_at FROM telemetry_sessions ORDER BY client, session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelemetrySession
	for rows.Next() {
		var item TelemetrySession
		if err := rows.Scan(&item.Client, &item.SessionID, &item.SourceKind, &item.Model, &item.FirstAt, &item.LastAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// TelemetrySessionSourceKinds is the classification interface for
// auxiliary-session-classification: the source marker of each requested
// session that has one. It is not a session read interface.
func (s *Store) TelemetrySessionSourceKinds(ctx context.Context, client string, sessionIDs []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(sessionIDs); start += 200 {
		batch := sessionIDs[start:min(start+200, len(sessionIDs))]
		args := []any{client}
		for _, id := range batch {
			args = append(args, id)
		}
		rows, err := s.DB.QueryContext(ctx, `SELECT session_id, source_kind FROM telemetry_sessions WHERE client = ? AND session_id IN (?`+strings.Repeat(",?", len(batch)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, kind string
			if err := rows.Scan(&id, &kind); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = kind
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// TelemetrySessionOriginMarker is the part of a session's OTel marker that
// the codex-otel-background origin rule reads.
type TelemetrySessionOriginMarker struct {
	SourceKind   string
	SupportEntry string
	SettledAt    string
}

// TelemetrySessionOriginMarkers returns the marker of each requested session
// of client that has one, 200 IDs per parameterized query. It only reads: an
// empty candidate list issues no query, and no table or row is created.
func (s *Store) TelemetrySessionOriginMarkers(ctx context.Context, client string, sessionIDs []string) (map[string]TelemetrySessionOriginMarker, error) {
	out := map[string]TelemetrySessionOriginMarker{}
	for start := 0; start < len(sessionIDs); start += 200 {
		batch := sessionIDs[start:min(start+200, len(sessionIDs))]
		args := []any{client}
		for _, id := range batch {
			args = append(args, id)
		}
		rows, err := s.DB.QueryContext(ctx, `SELECT session_id, source_kind, support_entry, settled_at FROM telemetry_sessions WHERE client = ? AND session_id IN (?`+strings.Repeat(",?", len(batch)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var marker TelemetrySessionOriginMarker
			if err := rows.Scan(&id, &marker.SourceKind, &marker.SupportEntry, &marker.SettledAt); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = marker
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// TelemetryBackgroundSessionCount counts the codex-otel-background markers.
func (s *Store) TelemetryBackgroundSessionCount(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM telemetry_sessions WHERE source_kind = ?`, TelemetrySessionSourceKind).Scan(&count)
	return count, err
}

// TelemetryCursorPosition reads the journal cursor.
func (s *Store) TelemetryCursorPosition(ctx context.Context) (TelemetryCursor, error) {
	var cursor TelemetryCursor
	err := s.DB.QueryRowContext(ctx, `SELECT segment, offset FROM telemetry_journal_cursor WHERE singleton = 1`).Scan(&cursor.Segment, &cursor.Offset)
	return cursor, err
}

// TelemetryCoverageInterval is one recorded listening interval or known gap.
// An open listening interval has an empty EndedAt.
type TelemetryCoverageInterval struct {
	ID        int64
	Kind      string
	StartedAt string
	EndedAt   string
	Reason    string
}

// BeginTelemetryListening opens a listening interval and closes any interval
// a crashed listener left open, recording that span's end as unknown.
func (s *Store) BeginTelemetryListening(ctx context.Context, now time.Time) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE telemetry_coverage SET ended_at = 'unknown', reason = 'listener_exit_unrecorded' WHERE kind = 'listening' AND ended_at = ''`); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO telemetry_coverage(kind, started_at, ended_at, reason) VALUES ('listening', ?, '', '')`, formatTelemetryTime(now))
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// EndTelemetryListening closes a listening interval.
func (s *Store) EndTelemetryListening(ctx context.Context, id int64, now time.Time, reason string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE telemetry_coverage SET ended_at = ?, reason = ? WHERE id = ? AND kind = 'listening' AND ended_at = ''`, formatTelemetryTime(now), reason, id)
	return err
}

// RecordTelemetryGap records a known interval in which records may have been
// lost, such as a request refused because it could not be persisted.
func (s *Store) RecordTelemetryGap(ctx context.Context, started, ended time.Time, reason string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO telemetry_coverage(kind, started_at, ended_at, reason) VALUES ('gap', ?, ?, ?)`, formatTelemetryTime(started), formatTelemetryTime(ended), reason)
	return err
}

// TelemetrySummary is the stored part of telemetry status.
type TelemetrySummary struct {
	CaptureOnly      int
	Pending          int
	JSONLOwned       int
	OTelOwned        int
	Conflict         int
	Invalid          int
	OwnedSessions    int
	LastReceivedAt   string
	LastSupportEntry string
	Coverage         []TelemetryCoverageInterval
}

// TelemetrySummary reads observation counts and the recorded coverage.
func (s *Store) TelemetrySummary(ctx context.Context) (TelemetrySummary, error) {
	var summary TelemetrySummary
	rows, err := s.DB.QueryContext(ctx, `SELECT state, count(*) FROM telemetry_observations WHERE metered = 1 OR state IN ('invalid','conflict') GROUP BY state`)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			rows.Close()
			return summary, err
		}
		switch state {
		case "capture_only":
			summary.CaptureOnly = count
		case "pending":
			summary.Pending = count
		case "jsonl_owned":
			summary.JSONLOwned = count
		case "otel_owned":
			summary.OTelOwned = count
		case "conflict":
			summary.Conflict = count
		case "invalid":
			summary.Invalid = count
		}
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM telemetry_sessions`).Scan(&summary.OwnedSessions); err != nil {
		return summary, err
	}
	// The last valid event proves reception; the metering verdict comes from
	// the last metered event, not from startup or stream events that carry no
	// usage and therefore no support entry.
	var last, support sql.NullString
	if err := s.DB.QueryRowContext(ctx, `SELECT max(received_at) FROM telemetry_observations WHERE source = 'receiver' AND state <> 'invalid'`).Scan(&last); err != nil {
		return summary, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT support_entry FROM telemetry_observations WHERE source = 'receiver' AND metered = 1 ORDER BY received_at DESC LIMIT 1`).Scan(&support); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return summary, err
	}
	summary.LastReceivedAt, summary.LastSupportEntry = last.String, support.String
	intervals, err := s.DB.QueryContext(ctx, `SELECT id, kind, started_at, ended_at, reason FROM telemetry_coverage ORDER BY id`)
	if err != nil {
		return summary, err
	}
	defer intervals.Close()
	for intervals.Next() {
		var interval TelemetryCoverageInterval
		if err := intervals.Scan(&interval.ID, &interval.Kind, &interval.StartedAt, &interval.EndedAt, &interval.Reason); err != nil {
			return summary, err
		}
		summary.Coverage = append(summary.Coverage, interval)
	}
	return summary, intervals.Err()
}

func queryStrings(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func formatTelemetryTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func nullableInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
