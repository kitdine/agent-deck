package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestUsagePartitionShadowPublicationAndTransactionalInvalidation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string) {
		t.Helper()
		if _, err := db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES
('a','codex','s','a','2026-10-19T01:00:00Z','m','source',0),
('b','claude','s','b','2026-10-19T01:00:00Z','n','source',1)`)
	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)
	parts, err := db.UsagePartitions(ctx, from, to)
	if err != nil || len(parts) != 2 {
		t.Fatalf("partitions: %+v %v", parts, err)
	}
	for i := range parts {
		p := &parts[i]
		revision, err := db.BeginUsagePartition(ctx, *p)
		if err != nil {
			t.Fatal(err)
		}
		p.Revision = revision
		p.Dirty = false
		if err := db.PublishUsagePartition(ctx, *p, revision, 0, "v1", []byte("old")); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CheckUsagePartitions(ctx, from, to, parts); err != nil {
		t.Fatal(err)
	}
	p := parts[1] // ordered by client: claude, codex
	revision, err := db.BeginUsagePartition(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	active, _ := db.UsagePartitions(ctx, from, to)
	if string(active[1].Payload) != "old" {
		t.Fatal("build removed active payload")
	}
	exec(`UPDATE usage_events SET input_tokens=17 WHERE event_key='a'`)
	if err := db.PublishUsagePartition(ctx, p, revision, 0, "v2", []byte("new")); !errors.Is(err, ErrUsagePartitionChanged) {
		t.Fatalf("stale publication accepted: %v", err)
	}
	active, _ = db.UsagePartitions(ctx, from, to)
	if string(active[1].Payload) != "old" || !active[1].Dirty || active[0].Dirty {
		t.Fatalf("invalidation crossed client boundary: %+v", active)
	}
	// Rollback must roll back invalidation as well as the fact mutation.
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE usage_events SET event_at='2026-10-20T02:00:00Z',model='new' WHERE event_key='b'`); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	rolled, _ := db.UsagePartitions(ctx, from, to)
	if len(rolled) != 2 || rolled[0].Dirty {
		t.Fatal("rolled-back input dirtied cache")
	}
	exec(`UPDATE usage_events SET event_at='2026-10-20T02:00:00Z',model='new' WHERE event_key='b'`)
	changed, _ := db.UsagePartitions(ctx, from, to)
	if len(changed) != 3 || !changed[0].Dirty || !changed[2].Dirty {
		t.Fatalf("rewrite missed old/new partition: %+v", changed)
	}
	if err := db.CheckUsagePartitions(ctx, from, to, parts); !errors.Is(err, ErrUsagePartitionChanged) {
		t.Fatal("optimistic read missed new key")
	}
}

func TestUsagePartitionSessionSpansAndPricingInvalidation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES ('a','codex','s','a','2026-10-19T23:00:00Z','m','f',0),('b','codex','s','b','2026-10-20T01:00:00Z','n','f',1),('c','claude','other','c','2026-10-20T01:00:00Z','x','g',0)`,
		`INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s','2026-10-19T23:00:00Z','2026-10-20T01:00:00Z')`,
	} {
		if _, err := db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)
	clean := func() {
		if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
			t.Fatal(err)
		}
		metadata, err := db.UsageProjectionMetadataRevision(ctx)
		if err != nil {
			t.Fatal(err)
		}
		parts, err := db.UsagePartitions(ctx, from, to)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range parts {
			revision, err := db.BeginUsagePartition(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.PublishUsagePartition(ctx, p, revision, metadata, "v", []byte("active")); err != nil {
				t.Fatal(err)
			}
		}
	}
	clean()
	if _, err := db.DB.ExecContext(ctx, `UPDATE usage_sessions SET started_at='2026-10-19T22:00:00Z' WHERE client='codex' AND session_id='s'`); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
		t.Fatal(err)
	}
	parts, _ := db.UsagePartitions(ctx, from, to)
	for _, p := range parts {
		if p.Dirty != (p.Client == "codex") {
			t.Fatalf("wrong session dependency: %+v", p)
		}
	}
	clean()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,exact) VALUES(1,'codex','test','1','2026-10-19T22:00:00Z',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_run_bindings(event_key,run_id) VALUES('a',1)`); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
		t.Fatal(err)
	}
	parts, _ = db.UsagePartitions(ctx, from, to)
	for _, p := range parts {
		if p.Dirty != (p.Client == "codex" && p.Day == "2026-10-19") {
			t.Fatalf("binding invalidated unrelated partition: %+v", p)
		}
	}
	clean()
	if _, err := db.DB.ExecContext(ctx, `UPDATE usage_runs SET multiplier='2' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
		t.Fatal(err)
	}
	parts, _ = db.UsagePartitions(ctx, from, to)
	for _, p := range parts {
		if p.Dirty != (p.Client == "codex" && p.Day == "2026-10-19") {
			t.Fatal("run update missed dependency or crossed partition boundary")
		}
	}
}

func TestUsagePartitionMigrationPreservesFactsAndRollsBackAtomically(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := migrations[:len(migrations)-1]
	if err := migrate(ctx, db, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('stable-id','codex','s','external-id','2026-10-19T23:00:00Z','m','source',17)`); err != nil {
		t.Fatal(err)
	}
	failed := append([]migration{}, old...)
	failed = append(failed, migration{version: 32, apply: func(ctx context.Context, tx *sql.Tx) error {
		if err := initializeUsagePartitions(ctx, tx); err != nil {
			return err
		}
		return errors.New("injected migration interruption")
	}})
	if err := migrate(ctx, db, failed); err == nil {
		t.Fatal("injected migration did not fail")
	}
	version, err := schemaVersion(ctx, db)
	if err != nil || version != 31 {
		t.Fatal("failed migration advanced schema")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name='usage_derived_partitions' OR name LIKE 'usage_partition_%'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed migration left partial cache objects")
	}
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	var key, eventID, path string
	var offset int64
	if err := db.QueryRowContext(ctx, `SELECT event_key,event_id,source_path,source_offset FROM usage_events`).Scan(&key, &eventID, &path, &offset); err != nil || key != "stable-id" || eventID != "external-id" || path != "source" || offset != 17 {
		t.Fatal("migration changed canonical identity or source ownership")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM usage_derived_partitions WHERE client='codex' AND model='m' AND day='2026-10-19' AND dirty=1 AND active_revision=0`).Scan(&count); err != nil || count != 1 {
		t.Fatal("migration did not seed a dirty partition")
	}
}

func TestUsagePartitionRejectsMetadataChangeBeforeBuildStarts(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('a','codex','s','a','2026-10-19T01:00:00Z','m','f',0)`); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	parts, err := db.UsagePartitions(ctx, from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := db.UsageProjectionMetadataRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The resolver has read the old route; the write occurs before Begin clears
	// the partition's dirty flag. A partition revision alone cannot detect this.
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,quality,semantic_key) VALUES('codex','s','2026-10-19T00:00:00Z','test','1',0,'SessionStart','exact','fixture-route')`); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
		t.Fatal(err)
	}
	revision, err := db.BeginUsagePartition(ctx, parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PublishUsagePartition(ctx, parts[0], revision, metadata, "stale", []byte("old-route")); !errors.Is(err, ErrUsagePartitionChanged) {
		t.Fatalf("stale resolver was published: %v", err)
	}
}

func TestUsagePartitionPendingMarkersBlockPublicationAndProviderOperationAdvancesServing(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('a','codex','s','a','2026-10-19T01:00:00Z','m','f',0)`); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	parts, err := db.UsagePartitions(ctx, from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := db.BeginUsagePartition(ctx, parts[0])
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := db.UsageProjectionMetadataRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s','2026-10-19T01:00:00Z','2026-10-19T01:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.PublishUsagePartition(ctx, parts[0], revision, metadata, "stale", []byte("old")); !errors.Is(err, ErrUsagePartitionChanged) {
		t.Fatal("pending input marker allowed publication")
	}
	if err := db.FlushUsagePartitionInvalidations(ctx); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := db.DB.QueryRowContext(ctx, `SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO operations(id,kind,state,started_at,updated_at) VALUES('provider-op','provider.use','started','2026-10-19T01:00:00Z','2026-10-19T01:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&after); err != nil || after <= before {
		t.Fatal("provider timeline mutation left serving identity current")
	}
	if err := db.CheckUsageProjectionMetadata(ctx, metadata); !errors.Is(err, ErrUsagePartitionChanged) {
		t.Fatal("changed provider timeline was reused")
	}
}
