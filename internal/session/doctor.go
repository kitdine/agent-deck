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

// CheckHealth reads the rebuildable session index without migrating it or
// changing its committed contents. Opening its WAL-mode SQLite database
// read-only may create 0600 -wal and -shm sidecars inside the 0700 state root:
// immutable=1 assumes the file cannot change and can yield incorrect results or
// SQLITE_CORRUPT during concurrent watcher or scanner writes, while
// nolock=1 could return a stale snapshot. busy_timeout matters for the same
// reason: a concurrent watcher or detached scanner can still hold a brief
// write lock while this reads, and without it SQLite fails immediately with
// SQLITE_BUSY -- callers such as agentdeck doctor treat that as a hard
// error -- instead of retrying briefly.
func CheckHealth(ctx context.Context, stateRoot string, full bool) (Health, error) {
	path := filepath.Join(stateRoot, "sessions.sqlite3")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return Health{Integrity: "not_requested"}, nil
	} else if err != nil {
		return Health{}, err
	}
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
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
