package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUsagePartitionChanged = errors.New("usage partition changed during derivation")

// UsagePartition is an input identity, independent of the source cursor and
// serving generation. Dirty builds retain their previous active payload.
type UsagePartition struct {
	Client, Model, Day       string
	Revision, ActiveRevision int64
	Dirty                    bool
	Version                  string
	Payload                  []byte
}

func initializeUsagePartitions(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE usage_derived_partitions (
 client TEXT NOT NULL, model TEXT NOT NULL, day TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, dirty INTEGER NOT NULL DEFAULT 1,
 active_revision INTEGER NOT NULL DEFAULT 0, version TEXT NOT NULL DEFAULT '',
 payload BLOB, PRIMARY KEY(client,model,day)
);
CREATE TABLE usage_derived_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton=1), revision INTEGER NOT NULL);
INSERT INTO usage_derived_metadata(singleton,revision) VALUES(1,0);
CREATE TABLE usage_derived_invalidations(kind TEXT NOT NULL,client TEXT NOT NULL,key TEXT NOT NULL,PRIMARY KEY(kind,client,key));
CREATE INDEX usage_derived_partitions_day ON usage_derived_partitions(day);
INSERT INTO usage_derived_partitions(client,model,day)
SELECT DISTINCT client,model,substr(event_at,1,10) FROM usage_events;`); err != nil {
		return err
	}
	mark := func(alias string) string {
		return fmt.Sprintf(`INSERT INTO usage_derived_partitions(client,model,day) VALUES(%s.client,%s.model,substr(%s.event_at,1,10))
ON CONFLICT(client,model,day) DO UPDATE SET revision=revision+1,dirty=1 WHERE dirty=0;`, alias, alias, alias)
	}
	for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
		body := ""
		if op != "INSERT" {
			body += mark("OLD")
		}
		if op != "DELETE" {
			body += mark("NEW")
		}
		if err := installUsagePartitionTrigger(ctx, tx, "usage_events", op, "", body); err != nil {
			return err
		}
	}

	// Dependency changes enqueue scoped dirty markers in the fact transaction.
	// Resolve their event partitions outside trigger DDL: otherwise every narrow
	// read-only serving open pays to parse those joins and correlated subqueries.
	for _, table := range []string{"usage_sessions", "usage_session_routes", "usage_runs", "usage_run_bindings"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			body := "UPDATE usage_derived_metadata SET revision=revision+1 WHERE singleton=1;"
			for _, alias := range []string{"OLD", "NEW"} {
				if (alias == "OLD" && op == "INSERT") || (alias == "NEW" && op == "DELETE") {
					continue
				}
				kind, client, key := "session", alias+".client", alias+".session_id"
				if table == "usage_runs" {
					kind, client, key = "run", "''", "CAST("+alias+".id AS TEXT)"
				}
				if table == "usage_run_bindings" {
					kind, client, key = "event", "''", alias+".event_key"
				}
				body += fmt.Sprintf("INSERT INTO usage_derived_invalidations(kind,client,key) VALUES('%s',%s,%s) ON CONFLICT(kind,client,key) DO NOTHING;", kind, client, key)
			}
			if err := installUsagePartitionTrigger(ctx, tx, table, op, "", body); err != nil {
				return err
			}
		}
	}
	for _, table := range []string{"providers", "provider_clients", "provider_selections", "provider_credentials", "provider_credential_clients", "price_catalogs", "model_prices", "usage_session_observations", "operations"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			condition := ""
			if table == "operations" {
				checks := []string{}
				if op != "INSERT" {
					checks = append(checks, "OLD.kind='provider.use'")
				}
				if op != "DELETE" {
					checks = append(checks, "NEW.kind='provider.use'")
				}
				condition = " WHEN " + strings.Join(checks, " OR ")
			}
			body := `UPDATE usage_derived_metadata SET revision=revision+1 WHERE singleton=1;
INSERT INTO usage_derived_invalidations(kind,client,key) VALUES('global','','') ON CONFLICT(kind,client,key) DO NOTHING;`
			if table == "operations" {
				body += "UPDATE serving_generation SET revision=revision+1 WHERE singleton=1;"
			}
			if err := installUsagePartitionTrigger(ctx, tx, table, op, condition, body); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) UsagePartitions(ctx context.Context, from, to time.Time) ([]UsagePartition, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT client,model,day,revision,active_revision,dirty,version,payload FROM usage_derived_partitions WHERE day>=? AND day<=? ORDER BY day,client,model`, from.UTC().Format("2006-01-02"), to.UTC().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsagePartition
	for rows.Next() {
		var p UsagePartition
		var dirty int
		if err = rows.Scan(&p.Client, &p.Model, &p.Day, &p.Revision, &p.ActiveRevision, &dirty, &p.Version, &p.Payload); err != nil {
			return nil, err
		}
		p.Dirty = dirty != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) BeginUsagePartition(ctx context.Context, p UsagePartition) (int64, error) {
	if s.readOnly {
		return 0, ErrDerivedSnapshotGenerationReadOnly
	}
	var revision int64
	err := s.DB.QueryRowContext(ctx, `UPDATE usage_derived_partitions SET revision=revision+1,dirty=0 WHERE client=? AND model=? AND day=? RETURNING revision`, p.Client, p.Model, p.Day).Scan(&revision)
	return revision, err
}

func (s *Store) PublishUsagePartition(ctx context.Context, p UsagePartition, revision, metadataRevision int64, version string, payload []byte) error {
	if s.readOnly {
		return ErrDerivedSnapshotGenerationReadOnly
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE usage_derived_partitions SET active_revision=revision,version=?,payload=? WHERE client=? AND model=? AND day=? AND revision=? AND dirty=0 AND (SELECT revision FROM usage_derived_metadata WHERE singleton=1)=? AND NOT EXISTS(SELECT 1 FROM usage_derived_invalidations)`, version, payload, p.Client, p.Model, p.Day, revision, metadataRevision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUsagePartitionChanged
	}
	return nil
}

func (s *Store) ReadOnly() bool { return s.readOnly }

func (s *Store) UsageProjectionMetadataRevision(ctx context.Context) (int64, error) {
	var revision int64
	err := s.DB.QueryRowContext(ctx, `SELECT revision FROM usage_derived_metadata WHERE singleton=1`).Scan(&revision)
	return revision, err
}

// CheckUsagePartitions closes the optimistic read, including keys created while
// the caller was building. It deliberately reads no cached payloads.
func (s *Store) CheckUsagePartitions(ctx context.Context, from, to time.Time, expected []UsagePartition) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT client,model,day,revision,dirty FROM usage_derived_partitions WHERE day>=? AND day<=? ORDER BY day,client,model`, from.UTC().Format("2006-01-02"), to.UTC().Format("2006-01-02"))
	if err != nil {
		return err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var p UsagePartition
		var dirty int
		if err := rows.Scan(&p.Client, &p.Model, &p.Day, &p.Revision, &dirty); err != nil {
			return err
		}
		if i >= len(expected) {
			return ErrUsagePartitionChanged
		}
		e := expected[i]
		if p.Client != e.Client || p.Model != e.Model || p.Day != e.Day || p.Revision != e.Revision || dirty != 0 {
			return ErrUsagePartitionChanged
		}
		i++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if i != len(expected) {
		return ErrUsagePartitionChanged
	}
	return nil
}

// Reuse the existing generation triggers instead of adding a second trigger
// family for every fact table. SQLite parses trigger definitions when opening a
// read-only connection; duplicate families consume the serving reader's budget.
// Coalescing belongs to the UPDATE, not WHEN: partition invalidation must still
// execute while the global computation generation is already dirty.
func installUsagePartitionTrigger(ctx context.Context, tx *sql.Tx, table, operation, condition, body string) error {
	name := fmt.Sprintf("derived_snapshot_generation_%s_%s", table, strings.ToLower(operation))
	if _, err := tx.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+name); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER %s AFTER %s ON %s %s BEGIN
 UPDATE derived_snapshot_generation SET revision=revision+1,dirty=1 WHERE singleton=1 AND dirty=0;
 %s END`, name, operation, table, condition, body))
	return err
}

// FlushUsagePartitionInvalidations is a bounded, atomic handoff from committed
// input markers to dirty partitions. Markers are never cleared before the
// affected partitions are invalidated. Publication rejects pending markers.
func (s *Store) FlushUsagePartitionInvalidations(ctx context.Context) error {
	var pending int
	if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_derived_invalidations)`).Scan(&pending); err != nil {
		return err
	}
	if pending == 0 {
		return nil
	}
	if s.readOnly {
		return ErrDerivedSnapshotGenerationReadOnly
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Acquire the writer reservation before reading the marker set. A writer
	// cannot replace a coalesced marker between our observation and its removal.
	if _, err := tx.ExecContext(ctx, `UPDATE usage_derived_metadata SET revision=revision WHERE singleton=1`); err != nil {
		return err
	}
	var global, clean int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_derived_invalidations WHERE kind='global'),EXISTS(SELECT 1 FROM usage_derived_partitions WHERE dirty=0)`).Scan(&global, &clean); err != nil {
		return err
	}
	if clean != 0 {
		if global != 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE usage_derived_partitions SET revision=revision+1,dirty=1 WHERE dirty=0`); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE usage_derived_partitions SET revision=revision+1,dirty=1 WHERE dirty=0 AND (client,model,day) IN (
SELECT e.client,e.model,substr(e.event_at,1,10) FROM usage_derived_invalidations i JOIN usage_events e ON i.kind='session' AND e.client=i.client AND e.session_id=i.key
UNION
SELECT e.client,e.model,substr(e.event_at,1,10) FROM usage_derived_invalidations i JOIN usage_events e ON i.kind='event' AND e.event_key=i.key
UNION
SELECT e.client,e.model,substr(e.event_at,1,10) FROM usage_derived_invalidations i JOIN usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key WHERE i.kind='run' AND COALESCE(b.run_id,e.run_id)=CAST(i.key AS INTEGER))`); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_derived_invalidations`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CheckUsageProjectionMetadata(ctx context.Context, expected int64) error {
	var revision int64
	var pending int
	if err := s.DB.QueryRowContext(ctx, `SELECT revision,EXISTS(SELECT 1 FROM usage_derived_invalidations) FROM usage_derived_metadata WHERE singleton=1`).Scan(&revision, &pending); err != nil {
		return err
	}
	if revision != expected || pending != 0 {
		return ErrUsagePartitionChanged
	}
	return nil
}
