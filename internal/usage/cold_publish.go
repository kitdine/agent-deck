package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
)

// publishColdSource is valid only for an unregistered source whose event and
// activity keys are absent inside the publication transaction. Existing keys
// fall back before any write so duplicate-source ownership stays with upsertTx.
// Reduction removes intermediate SQL statements, not the source transaction:
// rows, classifications, file links and the checkpoint still commit together.
func publishColdSource(ctx context.Context, tx *sql.Tx, path string, events []Event, records []activity.Record) (handled bool, imported, updated int, err error) {
	eventKeys, toolKeys := make([]string, 0, len(events)), make([]string, 0, len(records))
	for _, e := range events {
		if e.SourcePath != path {
			return false, 0, 0, nil
		}
		eventKeys = append(eventKeys, e.Key)
	}
	for _, r := range records {
		if r.SourcePath != path {
			return false, 0, 0, nil
		}
		toolKeys = append(toolKeys, r.Key)
	}
	for _, keys := range []struct {
		table, column string
		values        []string
	}{{"usage_events", "event_key", eventKeys}, {"usage_tool_calls", "activity_key", toolKeys}} {
		absent, checkErr := absentUsageKeys(ctx, tx, keys.table, keys.column, keys.values)
		if checkErr != nil || !absent {
			return false, 0, 0, checkErr
		}
	}

	// Keep first-insertion order and the reference's logical-change counts even
	// when a provider reports the same event more than once in one source.
	positions := make(map[string]int, len(events))
	finalEvents := make([]Event, 0, len(events))
	for _, e := range events {
		at, parseErr := time.Parse(time.RFC3339Nano, e.EventAt)
		if parseErr != nil {
			return true, 0, 0, fmt.Errorf("invalid usage event timestamp %q: %w", e.EventAt, parseErr)
		}
		e.EventAt = at.UTC().Format(time.RFC3339Nano)
		if i, exists := positions[e.Key]; exists {
			previous := finalEvents[i]
			// These columns are deliberately not updated by the reference UPSERT.
			if previous.Client != e.Client || previous.SessionID != e.SessionID || previous.EventID != e.EventID {
				return false, 0, 0, nil
			}
			if !sameColdEvent(previous, e) {
				updated++
			}
			finalEvents[i] = e
		} else {
			positions[e.Key] = len(finalEvents)
			finalEvents = append(finalEvents, e)
			imported++
		}
	}
	type toolRow struct {
		start     activity.Record
		completed any
		status    string
		duration  any
	}
	tools := make([]toolRow, 0)
	positions = make(map[string]int)
	for _, r := range records {
		i, exists := positions[r.Key]
		if r.StartedAt != "" {
			row := toolRow{start: r, status: "started"}
			if exists {
				tools[i] = row // A new start clears completion and previous files.
			} else {
				positions[r.Key] = len(tools)
				tools = append(tools, row)
			}
		} else if exists && r.CompletedAt != "" {
			row := &tools[i]
			row.completed, row.status, row.duration = r.CompletedAt, r.Status, nil
			started, startErr := time.Parse(time.RFC3339Nano, row.start.StartedAt)
			completed, completeErr := time.Parse(time.RFC3339Nano, r.CompletedAt)
			if startErr == nil && completeErr == nil && !completed.Before(started) {
				row.duration = completed.Sub(started).Milliseconds()
			}
		}
	}
	eventPrefix, _, _ := strings.Cut(upsertUsageEventSQL, "VALUES")
	batch := coldSQLBatch{tx: tx, prefix: eventPrefix + "VALUES", columns: 17}
	for _, e := range finalEvents {
		if err = batch.add(ctx, usageEventValues(e)...); err != nil {
			return true, 0, 0, err
		}
	}
	if err = batch.flush(ctx); err != nil {
		return true, 0, 0, err
	}
	batch = coldSQLBatch{tx: tx, columns: 16, prefix: `INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,completed_at,status,duration_ms,source_path,source_offset,turn_index,tool_kind,mcp_server,command_read,command_hint) VALUES`}
	for _, row := range tools {
		r := row.start
		if err = batch.add(ctx, r.Key, r.Client, r.SessionID, r.Model, r.Tool, r.StartedAt, row.completed, row.status, row.duration, r.SourcePath, r.SourceOffset, nullableInt(r.TurnIndex), r.ToolKind, nullableString(r.MCPServer), r.CommandRead, r.CommandHint); err != nil {
			return true, 0, 0, err
		}
	}
	if err = batch.flush(ctx); err != nil {
		return true, 0, 0, err
	}
	filePrefix, fileSuffix, _ := strings.Cut(upsertToolFileSQL, "VALUES(?,?,?,?)")
	batch = coldSQLBatch{tx: tx, columns: 4, prefix: filePrefix + "VALUES", suffix: fileSuffix}
	for _, row := range tools {
		for _, file := range row.start.Files {
			if err = batch.add(ctx, row.start.Key, file.PathDigest, file.BaseName, file.Wrote); err != nil {
				return true, 0, 0, err
			}
		}
	}
	return true, imported, updated, batch.flush(ctx)
}

func sameColdEvent(a, b Event) bool {
	if a.EventAt != b.EventAt || a.Model != b.Model || a.TurnIndex != b.TurnIndex {
		return false
	}
	for _, key := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens", "cache_write_tokens"} {
		if a.Tokens[key] != b.Tokens[key] {
			return false
		}
	}
	return true
}

func usageEventValues(e Event) []any {
	return []any{e.Key, e.Client, e.SessionID, e.EventID, e.EventAt, e.Model, nullableInt(e.TurnIndex), e.Tokens["input_tokens"], e.Tokens["cached_input_tokens"], e.Tokens["output_tokens"], e.Tokens["cache_read_tokens"], e.Tokens["cache_creation_tokens"], e.Tokens["cache_write_5m_tokens"], e.Tokens["cache_write_1h_tokens"], e.Tokens["cache_write_tokens"], e.SourcePath, e.SourceOffset}
}

func absentUsageKeys(ctx context.Context, tx *sql.Tx, table, column string, keys []string) (bool, error) {
	for start := 0; start < len(keys); start += 128 {
		end := min(start+128, len(keys))
		args := make([]any, 0, end-start)
		for _, key := range keys[start:end] {
			args = append(args, key)
		}
		var exists bool
		query := "SELECT EXISTS(SELECT 1 FROM " + table + " WHERE " + column + " IN (" + strings.TrimSuffix(strings.Repeat("?,", len(args)), ",") + "))"
		if err := tx.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil || exists {
			return false, err
		}
	}
	return true, nil
}

// Bound each statement below SQLite's conservative 999-variable limit and
// 1 MiB of string arguments. One already-supported oversized value is allowed.
type coldSQLBatch struct {
	tx                   *sql.Tx
	prefix, suffix       string
	columns, rows, bytes int
	args                 []any
}

func (b *coldSQLBatch) add(ctx context.Context, args ...any) error {
	size := 0
	for _, arg := range args {
		if text, ok := arg.(string); ok {
			size += len(text)
		}
	}
	if b.rows > 0 && (b.rows == 48 || b.bytes+size > 1<<20) {
		if err := b.flush(ctx); err != nil {
			return err
		}
	}
	b.args = append(b.args, args...)
	b.rows++
	b.bytes += size
	return nil
}

func (b *coldSQLBatch) flush(ctx context.Context) error {
	if b.rows == 0 {
		return nil
	}
	row := "(" + strings.TrimSuffix(strings.Repeat("?,", b.columns), ",") + "),"
	_, err := b.tx.ExecContext(ctx, b.prefix+strings.TrimSuffix(strings.Repeat(row, b.rows), ",")+b.suffix, b.args...)
	clear(b.args)
	b.args, b.rows, b.bytes = b.args[:0], 0, 0
	return err
}
