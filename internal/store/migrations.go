package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{
	{
		version: 1,
		statements: []string{
			"CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		},
	},
	{
		version: 2,
		statements: []string{
			"CREATE TABLE providers (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, endpoint TEXT NOT NULL, credential_ref TEXT NOT NULL, multiplier TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)",
			"CREATE TABLE provider_clients (provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE, client TEXT NOT NULL, native_model TEXT NOT NULL DEFAULT '', provider_model TEXT, PRIMARY KEY(provider_id, client, native_model))",
			"CREATE TABLE provider_selections (id INTEGER PRIMARY KEY, provider_id INTEGER NOT NULL REFERENCES providers(id), client TEXT NOT NULL, multiplier_snapshot TEXT NOT NULL, selected_at TEXT NOT NULL)",
			"CREATE TABLE operations (id TEXT PRIMARY KEY, kind TEXT NOT NULL, state TEXT NOT NULL, provider_id INTEGER REFERENCES providers(id), client TEXT, started_at TEXT NOT NULL, updated_at TEXT NOT NULL, redacted_backup_path TEXT, error_code TEXT)",
		},
	},
	{version: 3, statements: []string{"ALTER TABLE operations ADD COLUMN config_fingerprint TEXT"}},
	{version: 4, statements: []string{
		`CREATE TABLE usage_source_files (path TEXT PRIMARY KEY, identity TEXT NOT NULL, size INTEGER NOT NULL, cursor INTEGER NOT NULL, prefix_hash TEXT NOT NULL, session_id TEXT, turn_id TEXT, model TEXT, imported INTEGER NOT NULL DEFAULT 0, replaced INTEGER NOT NULL DEFAULT 0, malformed INTEGER NOT NULL DEFAULT 0, unsupported INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE usage_sessions (client TEXT NOT NULL, session_id TEXT NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL, PRIMARY KEY(client, session_id))`,
		`CREATE TABLE usage_events (event_key TEXT PRIMARY KEY, client TEXT NOT NULL, session_id TEXT NOT NULL, event_id TEXT NOT NULL, event_at TEXT NOT NULL, model TEXT NOT NULL, input_tokens INTEGER NOT NULL DEFAULT 0, cached_input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0, cache_read_tokens INTEGER NOT NULL DEFAULT 0, cache_creation_tokens INTEGER NOT NULL DEFAULT 0, cache_write_5m_tokens INTEGER NOT NULL DEFAULT 0, cache_write_1h_tokens INTEGER NOT NULL DEFAULT 0, source_path TEXT NOT NULL, source_offset INTEGER NOT NULL, run_id INTEGER)`,
		`CREATE INDEX usage_events_source ON usage_events(source_path)`,
		`CREATE TABLE usage_runs (id INTEGER PRIMARY KEY, client TEXT NOT NULL, provider TEXT NOT NULL, multiplier TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT, process_pid INTEGER)`,
		`CREATE UNIQUE INDEX one_active_usage_run_per_client ON usage_runs(client) WHERE ended_at IS NULL`,
		`CREATE TABLE usage_run_bindings (event_key TEXT PRIMARY KEY REFERENCES usage_events(event_key) ON DELETE CASCADE, run_id INTEGER NOT NULL REFERENCES usage_runs(id) ON DELETE CASCADE)`,
		`CREATE TABLE price_catalogs (version TEXT PRIMARY KEY, source_kind TEXT NOT NULL, source_url TEXT NOT NULL, commit_sha TEXT, content_sha256 TEXT NOT NULL, imported_at TEXT NOT NULL, effective_from TEXT NOT NULL, currency TEXT NOT NULL, schema_version INTEGER NOT NULL)`,
		`CREATE TABLE model_prices (catalog_version TEXT NOT NULL REFERENCES price_catalogs(version), model TEXT NOT NULL, provider TEXT NOT NULL, effective_from TEXT NOT NULL, prices_json TEXT NOT NULL, aliases_json TEXT NOT NULL, PRIMARY KEY(catalog_version, model))`,
	}},
	{version: 5, statements: []string{
		`ALTER TABLE usage_runs ADD COLUMN exact INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE usage_runs ADD COLUMN ambiguity_reason TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE usage_run_sources (run_id INTEGER NOT NULL REFERENCES usage_runs(id) ON DELETE CASCADE, path TEXT NOT NULL, start_offset INTEGER NOT NULL, end_offset INTEGER, start_hash TEXT NOT NULL, end_hash TEXT, PRIMARY KEY(run_id,path))`,
	}},
}

func migrate(ctx context.Context, db *sql.DB, ordered []migration) error {
	fresh, err := schemaMetadataIsFresh(ctx, db)
	if err != nil {
		return err
	}
	if fresh {
		return bootstrapAndMigrate(ctx, db, ordered)
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version > CurrentSchemaVersion {
		return fmt.Errorf("%w: database version %d exceeds supported version %d", ErrUnknownSchema, version, CurrentSchemaVersion)
	}
	for _, migration := range ordered {
		if migration.version <= version {
			continue
		}
		if migration.version != version+1 {
			return fmt.Errorf("migration sequence skips version %d", version+1)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, statement := range migration.statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d: %w", migration.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET version = ?", migration.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: %w", migration.version, err)
		}
		version = migration.version
	}
	return nil
}

func schemaMetadataIsFresh(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_metadata'").Scan(&count); err != nil {
		return false, err
	}
	if count == 1 {
		return false, nil
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&count); err != nil {
		return false, err
	}
	if count != 0 {
		return false, fmt.Errorf("%w: missing schema metadata", ErrUnknownSchema)
	}
	return true, nil
}

func bootstrapAndMigrate(ctx context.Context, db *sql.DB, ordered []migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton = 1), version INTEGER NOT NULL CHECK(version >= 0)); INSERT INTO schema_metadata(singleton, version) VALUES (1, 0)"); err != nil {
		_ = tx.Rollback()
		return err
	}
	version := 0
	for _, migration := range ordered {
		if migration.version != version+1 {
			_ = tx.Rollback()
			return fmt.Errorf("migration sequence skips version %d", version+1)
		}
		for _, statement := range migration.statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d: %w", migration.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET version = ?", migration.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", migration.version, err)
		}
		version = migration.version
	}
	return tx.Commit()
}

func schemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_metadata").Scan(&count); err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, fmt.Errorf("%w: expected one schema version row, found %d", ErrUnknownSchema, count)
	}
	var version int
	err := db.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: missing version row", ErrUnknownSchema)
	}
	return version, err
}
