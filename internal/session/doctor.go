package session

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Health struct {
	Present           bool
	FTSAvailable      bool
	Integrity         string
	UnreadableSources int
}

// CheckHealth inspects the rebuildable session index without creating,
// migrating, or changing it.
func CheckHealth(ctx context.Context, stateRoot string, full bool) (Health, error) {
	path := filepath.Join(stateRoot, "sessions.sqlite3")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return Health{Integrity: "not_requested"}, nil
	} else if err != nil {
		return Health{}, err
	}
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return Health{}, err
	}
	defer database.Close()
	health := Health{Present: true, Integrity: "not_requested"}
	var count int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='session_documents'").Scan(&count); err != nil {
		return Health{}, err
	}
	health.FTSAvailable = count == 1
	if full {
		if err = database.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&health.Integrity); err != nil {
			return Health{}, err
		}
		rows, queryErr := database.QueryContext(ctx, "SELECT source_path FROM session_sources ORDER BY source_path")
		if queryErr != nil {
			return Health{}, queryErr
		}
		for rows.Next() {
			var sourcePath string
			if err = rows.Scan(&sourcePath); err != nil {
				rows.Close()
				return Health{}, err
			}
			file, openErr := os.Open(sourcePath)
			if openErr != nil {
				health.UnreadableSources++
				continue
			}
			if closeErr := file.Close(); closeErr != nil {
				health.UnreadableSources++
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return Health{}, err
		}
		rows.Close()
	} else if health.FTSAvailable {
		health.Integrity = "ok"
	}
	return health, nil
}
