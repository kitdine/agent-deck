package usage

import (
	"context"
	"database/sql"
)

// Each branch reads only its source_path index; the UNION also includes
// tool-only orphans. Discovery and all associated deletes share one transaction.
// otel: facts are owned by the telemetry reconciler, not by any source file.
const orphanUsagePathsSQL = `SELECT e.source_path FROM (SELECT DISTINCT source_path FROM usage_events) e
WHERE substr(e.source_path,1,5)<>'otel:' AND NOT EXISTS (SELECT 1 FROM usage_source_files f WHERE f.path=e.source_path)
UNION SELECT t.source_path FROM (SELECT DISTINCT source_path FROM usage_tool_calls) t
WHERE NOT EXISTS (SELECT 1 FROM usage_source_files f WHERE f.path=t.source_path)`

func orphanUsagePaths(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, orphanUsagePathsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
