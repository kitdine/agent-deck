package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const ServingStorageVersion = 1

// ServingGeneration is a committed vector, not a source cursor or a derived
// cache dirty bit. Revisions advance even while the computation cache is dirty.
type ServingGeneration struct {
	StateIdentity   string `json:"state_identity"`
	CoreEpoch       int64  `json:"core_epoch"`
	SessionEpoch    int64  `json:"session_epoch"`
	CoreSchema      int    `json:"core_schema"`
	SessionSchema   int    `json:"session_schema"`
	UsageRevision   int64  `json:"usage_revision"`
	SessionRevision int64  `json:"session_revision"`
}

func initializeServingStorage(ctx context.Context, tx *sql.Tx) error {
	if err := createServingStorage(ctx, tx); err != nil {
		return err
	}
	tables := append(append([]string{}, derivedSnapshotGenerationTables...), "settings", "quota_envelopes", "domain_coverage_heads", "source_domain_checkpoints")
	return installServingRevisionTriggers(ctx, tx, tables)
}

func ensureSessionServingStorage(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = createServingStorage(ctx, tx); err != nil {
		return err
	}
	if err = installServingRevisionTriggers(ctx, tx, []string{"session_sources", "session_metadata", "session_exclusions", "domain_coverage_heads", "source_domain_checkpoints"}); err != nil {
		return err
	}
	return tx.Commit()
}

func createServingStorage(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS serving_generation (singleton INTEGER PRIMARY KEY CHECK(singleton=1), schema_version INTEGER NOT NULL, revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0))`,
		`INSERT OR IGNORE INTO serving_generation(singleton,schema_version,revision) VALUES (1,1,0)`,
		`CREATE TABLE IF NOT EXISTS inventory_runs (inventory_id TEXT PRIMARY KEY, epoch INTEGER NOT NULL, captured_at TEXT NOT NULL, source_count INTEGER NOT NULL CHECK(source_count>=0), state TEXT NOT NULL CHECK(state IN ('pending','complete','partial','paused','failed','update_pending')))`,
		`CREATE TABLE IF NOT EXISTS source_domain_checkpoints (inventory_id TEXT NOT NULL REFERENCES inventory_runs(inventory_id), source_path TEXT NOT NULL, identity TEXT NOT NULL, captured_end INTEGER NOT NULL CHECK(captured_end>=0), committed_end INTEGER NOT NULL DEFAULT 0 CHECK(committed_end>=0), state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','complete','failed')), PRIMARY KEY(inventory_id,source_path))`,
		`CREATE TABLE IF NOT EXISTS domain_coverage_heads (singleton INTEGER PRIMARY KEY CHECK(singleton=1), inventory_id TEXT NOT NULL REFERENCES inventory_runs(inventory_id))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT schema_version FROM serving_generation WHERE singleton=1`).Scan(&version); err != nil {
		return err
	}
	if version != ServingStorageVersion {
		return errors.New("unsupported serving storage version")
	}
	return nil
}

func installServingRevisionTriggers(ctx context.Context, tx *sql.Tx, tables []string) error {
	for _, table := range tables {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			statement := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS serving_revision_%s_%s AFTER %s ON %s BEGIN UPDATE serving_generation SET revision=revision+1 WHERE singleton=1; END`, table, operation, operation, table)
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
			if table == "usage_source_files" || table == "usage_events" || table == "usage_work_signals" || table == "session_sources" || table == "session_metadata" {
				statement = fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS source_coverage_pending_%s_%s AFTER %s ON %s BEGIN UPDATE inventory_runs SET state='update_pending' WHERE inventory_id=(SELECT inventory_id FROM domain_coverage_heads WHERE singleton=1) AND state='complete'; END`, table, operation, operation, table)
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ServingGenerationReader retains read-only connections for the two identity
// observations of one snapshot read. It never caches the generation values.
// It is request-local, not shared concurrently, and owns one 200ms deadline.
type ServingGenerationReader struct {
	ctx            context.Context
	cancel         context.CancelFunc
	root, identity string
	paths          map[string]*syscall.Stat_t
	core, sessions *sql.DB
}

func OpenServingGenerationReader(ctx context.Context, stateRoot string) (*ServingGenerationReader, error) {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()
	root, err := filepath.Abs(stateRoot)
	if err != nil {
		return nil, err
	}
	rootInfo, err := privateServingPath(root, true)
	if err != nil {
		return nil, err
	}
	corePath, sessionPath := filepath.Join(root, "agentdeck.sqlite3"), filepath.Join(root, "sessions.sqlite3")
	coreInfo, err := privateServingPath(corePath, false)
	if err != nil {
		return nil, err
	}
	sessionInfo, err := privateServingPath(sessionPath, false)
	if err != nil {
		return nil, err
	}
	identity := fmt.Sprintf("%s:%d:%d:%d:%d:%d:%d", root, rootInfo.Dev, rootInfo.Ino, coreInfo.Dev, coreInfo.Ino, sessionInfo.Dev, sessionInfo.Ino)
	digest := sha256.Sum256([]byte(identity))
	identityValue := hex.EncodeToString(digest[:])
	core, err := openServingIdentityDB(ctx, corePath)
	if err != nil {
		return nil, err
	}

	success = true
	return &ServingGenerationReader{ctx: ctx, cancel: cancel, root: root, identity: identityValue,
		paths: map[string]*syscall.Stat_t{root: rootInfo, corePath: coreInfo, sessionPath: sessionInfo}, core: core}, nil
}

func (r *ServingGenerationReader) Close() {
	r.cancel()
	if r.sessions != nil {
		r.sessions.Close()
	}
	r.core.Close()
}

func (r *ServingGenerationReader) Read() (ServingGeneration, error) {
	ctx := r.ctx
	if err := ctx.Err(); err != nil {
		return ServingGeneration{}, err
	}
	result := ServingGeneration{StateIdentity: r.identity}
	var err error
	err = r.core.QueryRowContext(ctx, `SELECT d.epoch,s.version,g.revision FROM derived_snapshot_generation d JOIN schema_metadata s ON s.singleton=d.singleton JOIN serving_generation g ON g.singleton=d.singleton WHERE d.singleton=1`).Scan(&result.CoreEpoch, &result.CoreSchema, &result.UsageRevision)
	if err != nil {
		return ServingGeneration{}, err
	}
	if result.CoreSchema > CurrentSchemaVersion {
		return ServingGeneration{}, &SchemaAhead{Stored: result.CoreSchema, Supported: CurrentSchemaVersion}
	}
	if r.sessions == nil {
		r.sessions, err = openServingIdentityDB(ctx, filepath.Join(r.root, "sessions.sqlite3"))
		if err != nil {
			return ServingGeneration{}, err
		}
	}
	err = r.sessions.QueryRowContext(ctx, `SELECT e.epoch,g.schema_version,g.revision FROM session_index_generation e JOIN serving_generation g ON g.singleton=e.singleton WHERE e.singleton=1`).Scan(&result.SessionEpoch, &result.SessionSchema, &result.SessionRevision)
	if err != nil {
		return ServingGeneration{}, err
	}
	if result.CoreEpoch <= 0 || result.SessionEpoch <= 0 || result.SessionSchema != ServingStorageVersion {
		return ServingGeneration{}, errors.New("invalid serving generation")
	}
	// A replacement during the read must not validate a file from the old DB.
	for path, original := range r.paths {
		current, checkErr := privateServingPath(path, path == r.root)
		if checkErr != nil || current.Dev != original.Dev || current.Ino != original.Ino {
			return ServingGeneration{}, errors.New("serving state changed during identity read")
		}
	}

	return result, nil
}

// ReadServingGeneration never creates, migrates, scans, or takes a writer lock.
func ReadServingGeneration(ctx context.Context, stateRoot string) (ServingGeneration, error) {
	reader, err := OpenServingGenerationReader(ctx, stateRoot)
	if err != nil {
		return ServingGeneration{}, err
	}
	defer reader.Close()
	return reader.Read()
}

func openServingIdentityDB(ctx context.Context, path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro&_pragma=busy_timeout(20)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func privateServingPath(path string, directory bool) (*syscall.Stat_t, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	mode := os.FileMode(0600)
	if directory {
		mode = 0700
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || info.Mode().Perm() != mode || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return nil, errors.New("invalid private serving path")
	}
	return stat, nil
}
