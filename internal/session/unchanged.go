package session

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// unchangedSources proves the entire discovered source set against committed
// checkpoints in one query. Without any index writes there can be no document
// delta, so the caller need not materialize two copies of the full FTS view.
// Unknown change times and incomplete legacy checkpoints always fall back.
func unchangedSources(ctx context.Context, db *sql.DB, paths []source) (bool, error) {
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name IN ('session_sources','session_metadata','session_documents')`).Scan(&tables); err != nil {
		return false, err
	}
	if tables != 3 {
		return false, nil
	}
	expected := make(map[string]source, len(paths))
	for _, src := range paths {
		if !src.stable || src.changedAt == 0 {
			return false, nil
		}
		expected[filepath.Clean(src.path)] = src
	}
	rows, err := db.QueryContext(ctx, `SELECT source_path,identity,size,cursor,modified_at,changed_at,priority,parser_version FROM session_sources`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	matched := 0
	for rows.Next() {
		var path, identity string
		var size, cursor, modified, changed, priority, parser int64
		if err := rows.Scan(&path, &identity, &size, &cursor, &modified, &changed, &priority, &parser); err != nil {
			return false, err
		}
		src, found := expected[path]
		if !found || identity != src.identity || size != src.size || cursor != size ||
			modified != src.modifiedAt || changed != src.changedAt || priority != int64(src.priority) || parser != ParserVersion {
			return false, nil
		}
		matched++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if matched != len(expected) {
		return false, nil
	}
	// Validate the same observation again after the registry read. Never bless
	// a checkpoint using source metadata from before an intervening mutation.
	for _, src := range paths {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := ingest.ValidateCapturedRange(ingest.Source{Path: src.path, Identity: src.identity, Size: src.size,
			ModifiedAt: src.modifiedAt, ChangedAt: src.changedAt, Stable: src.stable}, src.size); err != nil {
			return false, err
		}
	}
	return true, nil
}
