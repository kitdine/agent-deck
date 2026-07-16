package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kitdine/agent-deck/internal/providermeta"
)

type migration struct {
	version    int
	statements []string
	apply      func(context.Context, *sql.Tx) error
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
	{version: 6, statements: []string{
		`CREATE TABLE extensions (id TEXT PRIMARY KEY, client TEXT NOT NULL, kind TEXT NOT NULL, scope TEXT NOT NULL, native_id TEXT NOT NULL, source_path TEXT NOT NULL, version TEXT NOT NULL DEFAULT 'unknown', enabled TEXT NOT NULL DEFAULT 'unknown', capabilities_json TEXT NOT NULL, diagnostics_json TEXT NOT NULL, fingerprint TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE extension_management (extension_id TEXT PRIMARY KEY REFERENCES extensions(id) ON DELETE CASCADE, fingerprint TEXT NOT NULL, adopted_at TEXT NOT NULL)`,
		`CREATE INDEX extensions_client_kind ON extensions(client, kind, scope, native_id)`,
	}},
	{version: 7, statements: []string{
		`CREATE TABLE provider_credentials (id INTEGER PRIMARY KEY, provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE, name TEXT NOT NULL, credential_ref TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(provider_id,name))`,
		`CREATE TABLE provider_credential_clients (credential_id INTEGER NOT NULL REFERENCES provider_credentials(id) ON DELETE CASCADE, client TEXT NOT NULL, PRIMARY KEY(credential_id,client))`,
		`INSERT INTO provider_credentials(provider_id,name,credential_ref,created_at,updated_at) SELECT id,'default',credential_ref,created_at,updated_at FROM providers`,
		`INSERT INTO provider_credential_clients(credential_id,client) SELECT pc.id,pcl.client FROM provider_credentials pc JOIN provider_clients pcl ON pcl.provider_id=pc.provider_id GROUP BY pc.id,pcl.client`,
		`CREATE TABLE operations_v7 (id TEXT PRIMARY KEY, kind TEXT NOT NULL, state TEXT NOT NULL, provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL, client TEXT, started_at TEXT NOT NULL, updated_at TEXT NOT NULL, redacted_backup_path TEXT, error_code TEXT, config_fingerprint TEXT, resource_name TEXT NOT NULL DEFAULT '', details_json TEXT NOT NULL DEFAULT '{}')`,
		`INSERT INTO operations_v7(id,kind,state,provider_id,client,started_at,updated_at,redacted_backup_path,error_code,config_fingerprint) SELECT id,kind,state,provider_id,client,started_at,updated_at,redacted_backup_path,error_code,config_fingerprint FROM operations`,
		`DROP TABLE operations`,
		`ALTER TABLE operations_v7 RENAME TO operations`,
		`CREATE TABLE provider_selections_v7 (id INTEGER PRIMARY KEY, provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL, client TEXT NOT NULL, provider_name_snapshot TEXT NOT NULL, endpoint_snapshot TEXT NOT NULL DEFAULT '', multiplier_snapshot TEXT NOT NULL, credential_id INTEGER REFERENCES provider_credentials(id) ON DELETE SET NULL, credential_name_snapshot TEXT NOT NULL DEFAULT '', operation_id TEXT REFERENCES operations(id) ON DELETE SET NULL, selected_at TEXT NOT NULL)`,
		`INSERT INTO provider_selections_v7(id,provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_id,credential_name_snapshot,operation_id,selected_at) SELECT ps.id,ps.provider_id,ps.client,p.name,p.endpoint,ps.multiplier_snapshot,pc.id,COALESCE(pc.name,''),(SELECT o.id FROM operations o WHERE o.kind='provider.use' AND o.state='completed' AND o.provider_id=ps.provider_id AND o.client=ps.client AND o.started_at<=ps.selected_at AND o.updated_at>=ps.selected_at ORDER BY o.updated_at,o.id LIMIT 1),ps.selected_at FROM provider_selections ps JOIN providers p ON p.id=ps.provider_id LEFT JOIN provider_credentials pc ON pc.provider_id=ps.provider_id AND pc.name='default' WHERE EXISTS (SELECT 1 FROM operations o WHERE o.kind='provider.use' AND o.state='completed' AND o.provider_id=ps.provider_id AND o.client=ps.client AND o.started_at<=ps.selected_at AND o.updated_at>=ps.selected_at) OR NOT EXISTS (SELECT 1 FROM operations o WHERE o.kind='provider.use' AND o.provider_id=ps.provider_id AND o.client=ps.client AND o.started_at<=ps.selected_at AND o.updated_at>=ps.selected_at)`,
		`DROP TABLE provider_selections`,
		`ALTER TABLE provider_selections_v7 RENAME TO provider_selections`,
		`CREATE UNIQUE INDEX provider_selections_completed_operation ON provider_selections(operation_id) WHERE operation_id IS NOT NULL`,
		`ALTER TABLE usage_source_files ADD COLUMN modified_at INTEGER NOT NULL DEFAULT 0`,
	}},
	{version: 8, statements: []string{
		`ALTER TABLE provider_credentials ADD COLUMN logical_ref TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE provider_credentials ADD COLUMN endpoint TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE provider_credentials ADD COLUMN multiplier TEXT NOT NULL DEFAULT '1'`,
	}, apply: backfillCredentialMetadata},
	{version: 9, statements: []string{
		`CREATE TABLE credential_secrets (credential_id INTEGER PRIMARY KEY REFERENCES provider_credentials(id) ON DELETE CASCADE, algorithm TEXT NOT NULL, key_version INTEGER NOT NULL, key_id TEXT NOT NULL, nonce BLOB NOT NULL, ciphertext BLOB NOT NULL, updated_at TEXT NOT NULL)`,
	}},
}

func backfillCredentialMetadata(ctx context.Context, tx *sql.Tx) error {
	type credentialMetadata struct {
		id             int64
		providerName   string
		credentialName string
		endpoint       string
		multiplier     string
		codex          bool
	}
	rows, err := tx.QueryContext(ctx, `SELECT pc.id,p.name,pc.name,p.endpoint,p.multiplier,EXISTS(SELECT 1 FROM provider_credential_clients pcc WHERE pcc.credential_id=pc.id AND pcc.client='codex') FROM provider_credentials pc JOIN providers p ON p.id=pc.provider_id ORDER BY pc.id`)
	if err != nil {
		return err
	}
	var credentials []credentialMetadata
	for rows.Next() {
		var item credentialMetadata
		if err = rows.Scan(&item.id, &item.providerName, &item.credentialName, &item.endpoint, &item.multiplier, &item.codex); err != nil {
			rows.Close()
			return err
		}
		credentials = append(credentials, item)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, item := range credentials {
		reference, normalizeErr := providermeta.CredentialReference(item.providerName, item.credentialName)
		if normalizeErr != nil {
			return normalizeErr
		}
		endpoint, normalizeErr := providermeta.NormalizeEndpoint(item.endpoint, item.codex)
		if normalizeErr != nil {
			return normalizeErr
		}
		multiplier, normalizeErr := providermeta.NormalizeMultiplier(item.multiplier)
		if normalizeErr != nil {
			return normalizeErr
		}
		if _, err = tx.ExecContext(ctx, `UPDATE provider_credentials SET logical_ref=?,endpoint=?,multiplier=? WHERE id=?`, reference, endpoint, multiplier, item.id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `CREATE UNIQUE INDEX provider_credentials_logical_ref ON provider_credentials(logical_ref)`)
	return err
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
		if migration.apply != nil {
			if err := migration.apply(ctx, tx); err != nil {
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
		if migration.apply != nil {
			if err := migration.apply(ctx, tx); err != nil {
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
