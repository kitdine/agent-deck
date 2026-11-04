package session

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// TelemetrySourcePrefix marks session-index sources projected from OTel-owned
// background conversations. Such a source is not a file: it has no documents,
// no activity details, and file inventory never removes it.
const TelemetrySourcePrefix = "otel:"

// telemetrySourcePriority ranks a projected source below every file source
// (archived 0, live 1), so a late JSONL file always wins the visible row.
const telemetrySourcePriority = -1

// TelemetrySession is one OTel-owned conversation to project.
type TelemetrySession struct {
	Client    string
	SessionID string
	Model     string
	FirstAt   string
	LastAt    string
}

// IsTelemetrySource reports whether a source path is a projected OTel source.
func IsTelemetrySource(path string) bool { return strings.HasPrefix(path, TelemetrySourcePrefix) }

// TelemetrySourcePath is the synthetic source key shared by session_sources
// and session_metadata for one conversation.
func TelemetrySourcePath(client, sessionID string) string {
	return TelemetrySourcePrefix + client + "/" + sessionID
}

// ProjectTelemetrySessions makes the index's otel: sources exactly the given
// sessions, minus excluded ones. It is idempotent, so it also regenerates the
// projection after a purge or rebuild.
func ProjectTelemetrySessions(ctx context.Context, db *sql.DB, sessions []TelemetrySession) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	restoreCoverage, err := store.PreserveSourceCoverage(ctx, tx)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	scannedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range sessions {
		path := TelemetrySourcePath(item.Client, item.SessionID)
		metadata := Metadata{Client: item.Client, SessionID: item.SessionID, SourcePath: path, Model: item.Model, FirstAt: item.FirstAt, LastAt: item.LastAt}
		skip, err := excluded(ctx, tx, metadata)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		want[path] = true
		if _, err = tx.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,X'',0,0,'',?,?,?)
ON CONFLICT(source_path) DO UPDATE SET priority=excluded.priority,parser_version=excluded.parser_version
WHERE (session_sources.priority,session_sources.parser_version) IS NOT (excluded.priority,excluded.parser_version)`, path, path, telemetrySourcePriority, ParserVersion, scannedAt); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,'',?,?,?,?)
ON CONFLICT(source_path,client,session_id) DO UPDATE SET model=excluded.model,first_at=excluded.first_at,last_at=excluded.last_at,parser_version=excluded.parser_version
WHERE (session_metadata.model,session_metadata.first_at,session_metadata.last_at,session_metadata.parser_version) IS NOT (excluded.model,excluded.first_at,excluded.last_at,excluded.parser_version)`, path, item.Client, item.SessionID, item.Model, ParserVersion, item.FirstAt, item.LastAt); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT source_path FROM session_sources WHERE substr(source_path,1,5)='otel:'`)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var path string
		if err = rows.Scan(&path); err != nil {
			rows.Close()
			return err
		}
		if !want[path] {
			stale = append(stale, path)
		}
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, path := range stale {
		if _, err = tx.ExecContext(ctx, `DELETE FROM session_metadata WHERE source_path=?; DELETE FROM session_sources WHERE source_path=?`, path, path); err != nil {
			return err
		}
	}
	if err := restoreCoverage(); err != nil {
		return err
	}
	return tx.Commit()
}
