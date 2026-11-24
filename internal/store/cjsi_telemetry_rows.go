// Private preimages for the CJSI OTel receipt consumer. Column lists are
// explicit so a future migration cannot silently change the durable receipt
// format.
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	// ErrCJSIOTelPrecondition reports that a selected CJSI OTel ownership
	// precondition, dependency revision, or live row image changed before commit.
	ErrCJSIOTelPrecondition = errors.New("Codex telemetry ownership precondition changed")
	errCJSIOTelPrecondition = ErrCJSIOTelPrecondition

	// ErrCJSIOTelAuthorityRequired reports that a caller attempted a legacy
	// unauthenticated OTel retraction or direct session-index projection bypass
	// against a CJSI schema-37 core store.
	ErrCJSIOTelAuthorityRequired = errors.New("Codex telemetry ownership requires CJSI receipt authority")
)

type cjsiOTelEvent struct {
	Key, Client, Thread, ID, At, Model                                            string
	Input, Cached, Output, CacheRead, CacheCreation, Write5m, Write1h, CacheWrite int64
	Path                                                                          string
	Offset                                                                        int64
	Run, Turn                                                                     sql.NullInt64
}

type cjsiOTelContribution struct{ Observation, Event string }
type cjsiOTelBinding struct {
	Event string
	Run   int64
}
type cjsiOTelMarker struct{ Client, Thread, Kind, Model, First, Last, Support, Settled string }
type cjsiOTelSession struct{ Client, Thread, First, Last, Started string }

// The same ordered field mapping serves capture, exact OLD comparison and
// reinstatement. database/sql preserves SQL NULL, integer width and text.
type cjsiOTelTable[T any] struct {
	name, columns string
	fields        func(*T) []any
}

var cjsiObservations = cjsiOTelTable[TelemetryObservation]{"telemetry_observations",
	"observation_key,source,producer,producer_version,originator,event_name,event_kind,transport,record_at,observed_at,received_at,model,response_id,request_id,attempt_id,thread_id,source_class,input_tokens,output_tokens,cached_tokens,cache_write_tokens,reasoning_tokens,total_tokens,metered,support_entry,state,reason",
	func(r *TelemetryObservation) []any {
		return []any{&r.Key, &r.Source, &r.Producer, &r.ProducerVersion, &r.Originator, &r.EventName, &r.EventKind, &r.Transport, &r.RecordAt, &r.ObservedAt, &r.ReceivedAt, &r.Model, &r.ResponseID, &r.RequestID, &r.AttemptID, &r.ThreadID, &r.SourceClass, &r.InputTokens, &r.OutputTokens, &r.CachedTokens, &r.CacheWriteTokens, &r.ReasoningTokens, &r.TotalTokens, &r.Metered, &r.SupportEntry, &r.State, &r.Reason}
	}}
var cjsiEvents = cjsiOTelTable[cjsiOTelEvent]{"usage_events",
	"event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens,source_path,source_offset,run_id,turn_index",
	func(r *cjsiOTelEvent) []any {
		return []any{&r.Key, &r.Client, &r.Thread, &r.ID, &r.At, &r.Model, &r.Input, &r.Cached, &r.Output, &r.CacheRead, &r.CacheCreation, &r.Write5m, &r.Write1h, &r.CacheWrite, &r.Path, &r.Offset, &r.Run, &r.Turn}
	}}
var cjsiContributions = cjsiOTelTable[cjsiOTelContribution]{"telemetry_contributions", "observation_key,event_key", func(r *cjsiOTelContribution) []any { return []any{&r.Observation, &r.Event} }}
var cjsiBindings = cjsiOTelTable[cjsiOTelBinding]{"usage_run_bindings", "event_key,run_id", func(r *cjsiOTelBinding) []any { return []any{&r.Event, &r.Run} }}
var cjsiMarkers = cjsiOTelTable[cjsiOTelMarker]{"telemetry_sessions", "client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at", func(r *cjsiOTelMarker) []any {
	return []any{&r.Client, &r.Thread, &r.Kind, &r.Model, &r.First, &r.Last, &r.Support, &r.Settled}
}}
var cjsiSessions = cjsiOTelTable[cjsiOTelSession]{"usage_sessions", "client,session_id,first_at,last_at,started_at", func(r *cjsiOTelSession) []any { return []any{&r.Client, &r.Thread, &r.First, &r.Last, &r.Started} }}

func (t cjsiOTelTable[T]) read(ctx context.Context, tx *sql.Tx, filter string, args ...any) ([]T, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+t.columns+" FROM "+t.name+" WHERE "+filter, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		var r T
		if err := rows.Scan(t.fields(&r)...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (t cjsiOTelTable[T]) insert(ctx context.Context, tx *sql.Tx, r T) error {
	values := t.fields(&r)
	_, err := tx.ExecContext(ctx, "INSERT INTO "+t.name+"("+t.columns+") VALUES (?"+strings.Repeat(",?", len(values)-1)+")", values...)
	return err
}

func (t cjsiOTelTable[T]) exact() string {
	return strings.Join(strings.Split(t.columns, ","), " IS ? AND ") + " IS ?"
}

func cjsiOTelChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return errCJSIOTelPrecondition
	}
	return err
}

func (t cjsiOTelTable[T]) remove(ctx context.Context, tx *sql.Tx, r T) error {
	return cjsiOTelChanged(tx.ExecContext(ctx, "DELETE FROM "+t.name+" WHERE "+t.exact(), t.fields(&r)...))
}

type cjsiOTelSide struct {
	Observations  []TelemetryObservation
	Contributions []cjsiOTelContribution
	Events        []cjsiOTelEvent
	Bindings      []cjsiOTelBinding
	Markers       []cjsiOTelMarker
	Sessions      []cjsiOTelSession
	OtherEvents   []string
}

// The canonical thread: a UUID matches every spelling of its case, any other
// identifier only itself. Preparation and the write-time rechecks share it.
func cjsiOTelMatch(thread string) string {
	if _, ok := NormalizeThreadID(thread); ok {
		return " = ? COLLATE NOCASE"
	}
	return " = ?"
}

func cjsiOTelSessionMatch(thread string) string {
	return "client='codex' AND session_id" + cjsiOTelMatch(thread)
}

func readCJSIOTelSummaries(ctx context.Context, tx *sql.Tx, thread string) ([]cjsiOTelSession, error) {
	return cjsiSessions.read(ctx, tx, cjsiOTelSessionMatch(thread)+" ORDER BY session_id", thread)
}

// Any non-OTel fact of the canonical thread, whatever its key and whether or
// not the preparation snapshot held it.
func cjsiOTelJSONLRemains(ctx context.Context, tx *sql.Tx, thread string) (present bool, err error) {
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM usage_events WHERE "+cjsiOTelSessionMatch(thread)+" AND substr(source_path,1,5)<>'otel:')", thread).Scan(&present)
	return
}

// Any OTel contribution, event, or marker that appeared for the canonical thread
// after preparation (even if an erroneous writer bypassed revision bumping).
func cjsiOTelUnexpectedOTelExists(ctx context.Context, tx *sql.Tx, thread string) (present bool, err error) {
	session := cjsiOTelSessionMatch(thread)
	obs := "thread_id" + cjsiOTelMatch(thread)
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM usage_events WHERE "+session+" AND substr(source_path,1,5)='otel:')"+
		" OR EXISTS(SELECT 1 FROM telemetry_sessions WHERE "+session+")"+
		" OR EXISTS(SELECT 1 FROM telemetry_contributions WHERE observation_key IN (SELECT observation_key FROM telemetry_observations WHERE "+obs+"))",
		thread, thread, thread).Scan(&present)
	return
}

// Capture only the selected thread, in the caller's existing core snapshot.
// Include orphan OTel events independently of the contribution join. A dangling
// link, mixed side or ambiguous marker must fail closed during preparation.
func readCJSIOTelSide(ctx context.Context, tx *sql.Tx, thread string) (s cjsiOTelSide, err error) {
	observations := "thread_id" + cjsiOTelMatch(thread)
	session := cjsiOTelSessionMatch(thread)
	events := session + " AND substr(source_path,1,5)='otel:'"
	if s.Observations, err = cjsiObservations.read(ctx, tx, observations+" ORDER BY observation_key", thread); err != nil {
		return
	}
	if s.Events, err = cjsiEvents.read(ctx, tx, events+" ORDER BY event_key", thread); err != nil {
		return
	}
	if s.Contributions, err = cjsiContributions.read(ctx, tx, "observation_key IN (SELECT observation_key FROM telemetry_observations WHERE "+observations+") ORDER BY observation_key", thread); err != nil {
		return
	}
	if s.Bindings, err = cjsiBindings.read(ctx, tx, "event_key IN (SELECT event_key FROM usage_events WHERE "+events+") ORDER BY event_key", thread); err != nil {
		return
	}
	if s.Markers, err = cjsiMarkers.read(ctx, tx, session, thread); err != nil {
		return
	}
	if s.Sessions, err = readCJSIOTelSummaries(ctx, tx, thread); err != nil {
		return
	}
	s.OtherEvents, err = queryStrings(ctx, tx, "SELECT event_key FROM usage_events WHERE "+session+" AND substr(source_path,1,5)<>'otel:' ORDER BY event_key", thread)
	return
}
