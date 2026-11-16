package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrTelemetrySourceIdentityPending reports a settle pass that could retract
// but not promote: a Codex source with JSONL facts has no assessed thread
// identity yet, or a committed source prefix has no checkpoint. Promoting then
// could count a child thread from both JSONL and OTel. The pass is not a
// successful settlement; the next normal scan or settle retries.
var ErrTelemetrySourceIdentityPending = errors.New("telemetry source identity preparation incomplete")

// TelemetryReadmissionLimit bounds the capture_only observations one settle
// pass re-evaluates against the current admission definition.
const TelemetryReadmissionLimit = 2000

// NormalizeThreadID returns the lower-case form of a UUID-shaped thread ID.
func NormalizeThreadID(value string) (string, bool) {
	if len(value) != 36 {
		return "", false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return "", false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return "", false
			}
		}
	}
	return strings.ToLower(value), true
}

// CodexRolloutThreadID parses the thread ID out of a canonical Codex rollout
// basename: rollout-YYYY-MM-DDThh-mm-ss-<thread>.jsonl, or for a reverted
// thread rollout-...-<thread>_<rollout>.jsonl. The thread is the first UUID;
// the directory, such as archived_sessions, does not matter. Any other name
// yields no evidence.
func CodexRolloutThreadID(path string) (string, bool) {
	name := path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	core, ok := strings.CutPrefix(name, "rollout-")
	if !ok {
		return "", false
	}
	if core, ok = strings.CutSuffix(core, ".jsonl"); !ok {
		return "", false
	}
	const stamp = "0000-00-00T00-00-00"
	if len(core) < len(stamp)+1+36 || core[len(stamp)] != '-' {
		return "", false
	}
	for i := 0; i < len(stamp); i++ {
		c := core[i]
		switch stamp[i] {
		case '0':
			if c < '0' || c > '9' {
				return "", false
			}
		default:
			if c != stamp[i] {
				return "", false
			}
		}
	}
	ids := core[len(stamp)+1:]
	thread, rollout, reverted := strings.Cut(ids, "_")
	if reverted {
		if _, ok := NormalizeThreadID(rollout); !ok {
			return "", false
		}
	}
	return NormalizeThreadID(thread)
}

// RetractTelemetryForJSONLThreads gives the named threads to JSONL inside the
// caller's transaction: every pending or otel_owned observation of a thread
// becomes jsonl_owned, its contribution, usage row and session marker go, and
// nothing else is touched. Callers pass the thread identity of the source
// whose facts the same transaction writes, so no committed state counts a
// thread from both JSONL and OTel. It reports how many threads changed.
func RetractTelemetryForJSONLThreads(ctx context.Context, tx *sql.Tx, threads []string, now time.Time) (int, error) {
	retracted := 0
	seen := map[string]bool{}
	for _, raw := range threads {
		normalized, ok := NormalizeThreadID(raw)
		if !ok || seen[normalized] {
			continue
		}
		seen[normalized] = true
		stored, err := queryStrings(ctx, tx, `SELECT DISTINCT thread_id FROM telemetry_observations WHERE lower(thread_id) = ? AND state IN ('pending','otel_owned')`, normalized)
		if err != nil {
			return retracted, err
		}
		for _, thread := range stored {
			if err := retractTelemetryThread(ctx, tx, thread, now); err != nil {
				return retracted, err
			}
			retracted++
		}
	}
	return retracted, nil
}

func retractTelemetryThread(ctx context.Context, tx *sql.Tx, thread string, now time.Time) error {
	keys, err := queryStrings(ctx, tx, `SELECT observation_key FROM telemetry_observations WHERE thread_id = ? AND state IN ('pending','otel_owned')`, thread)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := deleteContribution(ctx, tx, key); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state = 'jsonl_owned', reason = 'jsonl_evidence' WHERE thread_id = ? AND state IN ('pending','otel_owned')`, thread); err != nil {
		return err
	}
	return syncTelemetrySession(ctx, tx, thread, now)
}

// orphanCodexFactPathsSQL lists Codex JSONL fact paths that have no source
// checkpoint: a first import committed in prefix blocks and not finished, or a
// detached source awaiting cleanup. The recursive walk steps from one distinct
// source_path to the next through the source_path index, so its cost follows
// the number of sources, not the number of events.
const orphanCodexFactPathsSQL = `WITH RECURSIVE source(path) AS (
 SELECT min(source_path) FROM usage_events
 UNION ALL
 SELECT (SELECT min(source_path) FROM usage_events WHERE source_path > source.path) FROM source WHERE source.path IS NOT NULL)
SELECT path FROM source WHERE path IS NOT NULL AND substr(path,1,5)<>'otel:'
 AND NOT EXISTS (SELECT 1 FROM usage_source_files f WHERE f.path=source.path)
 AND EXISTS (SELECT 1 FROM usage_events c WHERE c.source_path=source.path AND c.client='codex')`

type telemetryQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// telemetrySourceThreads is what local Codex JSONL sources say about thread
// identity in one snapshot.
type telemetrySourceThreads struct {
	// evidence holds the threads a source positively identifies: its saved
	// thread_id, its canonical rollout name, and the canonical name of a fact
	// path without a checkpoint.
	evidence map[string]bool
	// ready is false while promotion could double count: a source with Codex
	// JSONL facts was never assessed and its name is not a canonical rollout
	// name, or a Codex fact path has no checkpoint. The client comes from the
	// facts themselves, so a Claude transcript or a source without usage yet
	// cannot double count and never blocks.
	ready bool
}

func readTelemetrySourceThreads(ctx context.Context, q telemetryQueryer) (telemetrySourceThreads, error) {
	state := telemetrySourceThreads{evidence: map[string]bool{}, ready: true}
	rows, err := q.QueryContext(ctx, `SELECT f.path, f.thread_id,
 CASE WHEN f.thread_id IS NULL THEN EXISTS (SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client='codex') ELSE 0 END
 FROM usage_source_files f WHERE substr(f.path,1,5)<>'otel:'`)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var thread sql.NullString
		var unassessedCodex bool
		if err := rows.Scan(&path, &thread, &unassessedCodex); err != nil {
			return state, err
		}
		if id, ok := NormalizeThreadID(thread.String); ok {
			state.evidence[id] = true
		}
		id, canonical := CodexRolloutThreadID(path)
		if canonical {
			state.evidence[id] = true
		} else if unassessedCodex {
			state.ready = false
		}
	}
	if err := rows.Err(); err != nil {
		return state, err
	}
	if err := rows.Close(); err != nil {
		return state, err
	}
	orphans, err := q.QueryContext(ctx, orphanCodexFactPathsSQL)
	if err != nil {
		return state, err
	}
	defer orphans.Close()
	for orphans.Next() {
		var path string
		if err := orphans.Scan(&path); err != nil {
			return state, err
		}
		state.ready = false
		if id, ok := CodexRolloutThreadID(path); ok {
			state.evidence[id] = true
		}
	}
	return state, orphans.Err()
}

// TelemetrySourceIdentityReady is the read-only form of the readiness check
// SettleTelemetry applies inside its transaction.
func (s *Store) TelemetrySourceIdentityReady(ctx context.Context) (bool, error) {
	state, err := readTelemetrySourceThreads(ctx, s.DB)
	return state.ready, err
}

// telemetryAdmissionSQL renders TelemetryAdmitted as a predicate over
// telemetry_observations columns, from the same families, shape and version
// bound. The version length is counted in bytes, as the Go check does.
func telemetryAdmissionSQL() (string, []any) {
	args := []any{TelemetryAdmittedTransport, TelemetryAdmittedEventName, TelemetryAdmittedEventKind, TelemetryProducerVersionMaxSize}
	families := make([]string, 0, len(TelemetryAdmittedFamilies()))
	for _, family := range TelemetryAdmittedFamilies() {
		families = append(families, "(producer = ? AND originator = ?)")
		args = append(args, family.Producer, family.Originator)
	}
	return `transport = ? AND event_name = ? AND event_kind = ? AND producer_version <> ''
 AND length(CAST(producer_version AS BLOB)) <= ? AND instr(producer_version, '/') = 0
 AND (` + strings.Join(families, " OR ") + `)`, args
}

const telemetryReadmissionCandidate = `state = 'capture_only' AND reason = 'producer_unsupported' AND metered = 1
 AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL AND `

// telemetrySettleNeedsSources reports whether a settle pass has anything that
// depends on source thread identity: a pending or OTel-owned conversation, or
// a capture_only observation the current admission definition admits.
func telemetrySettleNeedsSources(ctx context.Context, tx *sql.Tx) (bool, error) {
	predicate, args := telemetryAdmissionSQL()
	var needed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_observations WHERE thread_id <> '' AND state IN ('pending','otel_owned'))
 OR EXISTS(SELECT 1 FROM telemetry_observations WHERE `+telemetryReadmissionCandidate+predicate+`)`, args...).Scan(&needed)
	return needed, err
}

// readmitTelemetry re-evaluates capture_only observations that an earlier
// admission definition rejected as producer_unsupported and the current one
// admits. An observation with a conversation becomes pending under its actual
// support entry and then follows the ordinary ownership rules; one without a
// conversation only gets the accurate reason. Keys, counts and times stay.
// The predicate selects admitted rows only, so rejected rows never use the
// limit and a processed row leaves the candidate set.
func readmitTelemetry(ctx context.Context, tx *sql.Tx, limit int) (int, error) {
	predicate, args := telemetryAdmissionSQL()
	args = append(args, limit)
	result, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET
 state = CASE WHEN thread_id = '' THEN 'capture_only' ELSE 'pending' END,
 reason = CASE WHEN thread_id = '' THEN 'conversation_missing' ELSE 'ownership_not_settled' END,
 support_entry = CASE WHEN thread_id = '' THEN '' ELSE producer || '/' || producer_version || '/' || originator || '/' || transport END
 WHERE observation_key IN (SELECT observation_key FROM telemetry_observations
  WHERE `+telemetryReadmissionCandidate+predicate+` ORDER BY observation_key LIMIT ?)`, args...)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	return int(n), err
}

// UnassessedSource is one usage source whose thread identity was never
// assessed. CodexFacts reports stored Codex JSONL facts of the source.
type UnassessedSource struct {
	Path       string
	Identity   string
	CodexFacts bool
}

// UnassessedSources lists up to limit unassessed sources with after < path <=
// through, in path order; through is the sweep's captured upper bound.
func (s *Store) UnassessedSources(ctx context.Context, after, through string, limit int) ([]UnassessedSource, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT f.path, f.identity, EXISTS (SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client='codex')
 FROM usage_source_files f WHERE f.thread_id IS NULL AND f.path > ? AND f.path <= ? ORDER BY f.path LIMIT ?`, after, through, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnassessedSource
	for rows.Next() {
		var item UnassessedSource
		if err := rows.Scan(&item.Path, &item.Identity, &item.CodexFacts); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// LastUnassessedSourcePath is the upper bound of one finite identity sweep.
func (s *Store) LastUnassessedSourcePath(ctx context.Context) (string, error) {
	var path sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT max(path) FROM usage_source_files WHERE thread_id IS NULL`).Scan(&path)
	return path.String, err
}

// SourceThreadIdentity is the assessed thread identity of one source: a
// thread ID, or empty when the source verifiably carries none.
type SourceThreadIdentity struct {
	Path     string
	Identity string
	ThreadID string
}

// SaveSourceThreadIdentities stores one batch of assessed identities and, in
// the same transaction, gives each identified thread to JSONL. A source whose
// identity changed since it was read, or that was assessed meanwhile, is
// skipped. It reports the threads whose OTel ownership was retracted.
func (s *Store) SaveSourceThreadIdentities(ctx context.Context, items []SourceThreadIdentity, now time.Time) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// Take the write lock before the first read, so a concurrent commit cannot
	// invalidate this transaction's snapshot.
	if _, err := tx.ExecContext(ctx, `UPDATE usage_source_files SET cursor=cursor WHERE 0`); err != nil {
		return 0, err
	}
	restoreCoverage, err := PreserveSourceCoverage(ctx, tx)
	if err != nil {
		return 0, err
	}
	var threads []string
	for _, item := range items {
		result, err := tx.ExecContext(ctx, `UPDATE usage_source_files SET thread_id = ? WHERE path = ? AND identity = ? AND thread_id IS NULL`, item.ThreadID, item.Path, item.Identity)
		if err != nil {
			return 0, err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			continue
		}
		threads = append(threads, item.ThreadID)
		if id, ok := CodexRolloutThreadID(item.Path); ok {
			threads = append(threads, id)
		}
	}
	retracted, err := RetractTelemetryForJSONLThreads(ctx, tx, threads, now)
	if err != nil {
		return 0, err
	}
	if err := restoreCoverage(); err != nil {
		return 0, err
	}
	return retracted, tx.Commit()
}
