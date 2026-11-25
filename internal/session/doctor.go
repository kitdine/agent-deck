package session

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

type Health struct {
	Present           bool
	FTSAvailable      bool
	Integrity         string
	UnreadableSources int
}

// CheckHealth checks the selected index without migrating either database.
func CheckHealth(ctx context.Context, stateRoot string, full bool) (Health, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return Health{}, err
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "agentdeck.sqlite3")); err == nil {
		core, err := store.OpenReadOnly(ctx, stateRoot)
		if err == nil {
			if err := core.Close(); err != nil {
				return Health{}, err
			}
		} else if errors.Is(err, store.ErrSchemaAhead) {
			managed, err := store.ResolveCJSIRuntimeRoot(ctx, stateRoot)
			if err != nil || managed == nil {
				return unavailableIndex(ctx)
			}
			return checkPublishedIndexHealth(ctx, managed, full)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Health{}, err
	}
	database, err := store.OpenSessionDiagnosticsReadOnly(ctx, stateRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return Health{Integrity: "not_requested"}, nil
	}
	if err != nil {
		return Health{}, err
	}
	defer database.Close()
	return readIndexHealth(ctx, database.DB, full, "SELECT source_path FROM session_sources")
}

func checkPublishedIndexHealth(ctx context.Context, root *store.CJSIRuntimeRoot, full bool) (Health, error) {
	var health Health
	err := root.WithPublishedCore(ctx, func(core *sql.Tx, _ store.CJSIIndexStamp) (err error) {
		health, err = CheckSelectedIndexHealth(ctx, root, core, full)
		return err
	})
	if err != nil {
		return unavailableIndex(ctx)
	}
	return health, nil
}

// CheckSelectedIndexHealth borrows only the private component selected by the
// caller's committed core view. Unopened attempts do not affect its health.
func CheckSelectedIndexHealth(ctx context.Context, root *store.CJSIRuntimeRoot, core *sql.Tx, full bool) (Health, error) {
	var health Health
	err := root.WithIndexSnapshot(ctx, core, func(snapshot store.CJSIIndexSnapshot) (err error) {
		var compatible bool
		if err := snapshot.Tx.QueryRowContext(ctx, `SELECT count(*)=10 FROM sqlite_schema WHERE type='table' AND name IN
			('session_sources','session_metadata','session_exclusions','cjsi_base_groups','cjsi_base_order',
			 'cjsi_base_ranges','cjsi_session_candidates','cjsi_candidate_payloads','cjsi_candidate_order','cjsi_candidate_documents')`).
			Scan(&compatible); err != nil || !compatible {
			return errors.Join(store.ErrCJSIIndexChanged, err)
		}
		health, err = readIndexHealth(ctx, snapshot.Tx, full, `SELECT DISTINCT source_path FROM (`+snapshot.CopiesSQL()+`)
			WHERE source_kind IS NULL OR source_kind IN ('codex-jsonl','claude-jsonl')`)
		return err
	})
	if errors.Is(err, store.ErrCJSIIndexAbsent) {
		return Health{Integrity: "not_requested"}, nil
	}
	if err != nil {
		return Health{}, err
	}
	return health, nil
}

func unavailableIndex(ctx context.Context) (Health, error) {
	if err := ctx.Err(); err != nil {
		return Health{}, err
	}
	return Health{Present: true, Integrity: "unavailable"}, nil
}

type healthQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readIndexHealth(ctx context.Context, database healthQueryer, full bool, sourcesSQL string) (Health, error) {
	health := Health{Present: true, Integrity: "not_requested"}
	if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE type='table' AND name='session_documents')`).Scan(&health.FTSAvailable); err != nil {
		return Health{}, err
	}
	if !full {
		if health.FTSAvailable {
			health.Integrity = "ok"
		}
		return health, nil
	}
	var integrity string
	if err := database.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return Health{}, err
	}
	if integrity != "ok" {
		health.Integrity = "unavailable"
		return health, nil
	}
	health.Integrity = "ok"
	rows, err := database.QueryContext(ctx, sourcesSQL)
	if err != nil {
		return Health{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return Health{}, err
		}
		if filepath.IsAbs(path) && !readableSource(path) {
			health.UnreadableSources++
		}
	}
	return health, rows.Err()
}

func readableSource(path string) bool {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	return statErr == nil && info.Mode().IsRegular() && closeErr == nil
}
