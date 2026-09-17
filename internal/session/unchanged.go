package session

import (
	"context"
	"database/sql"
	"os"
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
	anchors := make(map[string]string, len(paths))
	for _, src := range paths {
		if !src.stable || src.changedAt == 0 {
			return false, nil
		}
		expected[filepath.Clean(src.path)] = src
	}
	rows, err := db.QueryContext(ctx, `SELECT source_path,identity,size,cursor,modified_at,changed_at,prefix_hash,priority,parser_version FROM session_sources`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	matched := 0
	for rows.Next() {
		var path, identity, anchor string
		var size, cursor, modified, changed, priority, parser int64
		if err := rows.Scan(&path, &identity, &size, &cursor, &modified, &changed, &anchor, &priority, &parser); err != nil {
			return false, err
		}
		src, found := expected[path]
		if !found || identity != src.identity || size != src.size || cursor != size ||
			modified != src.modifiedAt || changed != src.changedAt || priority != int64(src.priority) || parser != ParserVersion {
			return false, nil
		}
		anchors[path] = anchor
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
		if err := validateUnchangedObservation(src, anchors[filepath.Clean(src.path)]); err != nil {
			return false, err
		}
	}
	return true, nil
}

func validateUnchangedObservation(src source, storedAnchor string) error {
	info, err := os.Stat(src.path)
	if err != nil {
		return err
	}
	observed := ingest.Source{Path: src.path, Identity: src.identity, Size: src.size,
		ModifiedAt: src.modifiedAt, ChangedAt: src.changedAt, Stable: src.stable}
	if info.Size() == src.size {
		return ingest.Validate(observed)
	}
	if err = ingest.ValidateCapturedRange(observed, src.size); err != nil {
		return err
	}
	anchor, err := prefixHash(src.path, src.size)
	if err != nil {
		return err
	}
	if anchor != storedAnchor {
		return ingest.ErrSourceChanged
	}
	return nil
}
