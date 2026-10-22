package usage

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func orphanFixture(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "orphan.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, query := range []string{
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE usage_source_files(path TEXT PRIMARY KEY, session_id TEXT, session_started_at TEXT)`,
		`CREATE TABLE usage_events(event_key TEXT PRIMARY KEY, source_path TEXT,client TEXT,session_id TEXT,event_at TEXT)`,
		`CREATE INDEX usage_events_source ON usage_events(source_path)`,
		`CREATE TABLE usage_tool_calls(activity_key TEXT PRIMARY KEY,source_path TEXT)`,
		`CREATE INDEX usage_tool_calls_source ON usage_tool_calls(source_path)`,
		`CREATE TABLE usage_tool_files(activity_key TEXT REFERENCES usage_tool_calls(activity_key) ON DELETE CASCADE)`,
		`CREATE TABLE usage_sessions(client TEXT,session_id TEXT,first_at TEXT,last_at TEXT,started_at TEXT)`,
		`INSERT INTO usage_source_files VALUES('live','shared','2026-01-01')`,
		`INSERT INTO usage_events VALUES('live','live','codex','shared','2026-01-02')`,
		`INSERT INTO usage_sessions VALUES('codex','shared','2026-01-01','2026-01-02','2026-01-01')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	return New(&store.Store{DB: db}, t.TempDir())
}

func TestCleanupOrphanedEventsCoversToolOnlyAndSharedSession(t *testing.T) {
	s := orphanFixture(t)
	for _, query := range []string{
		`INSERT INTO usage_events VALUES('orphan','gone','codex','shared','2025-01-01')`,
		`INSERT INTO usage_tool_calls VALUES('event-tool','gone'),('tool-only','tool-gone'),('live-tool','live')`,
		`INSERT INTO usage_tool_files VALUES('tool-only')`,
	} {
		if _, err := s.Store.DB.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.cleanupOrphanedEvents(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("removed=%d err=%v", n, err)
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT count(*) FROM usage_events`, 1},
		{`SELECT count(*) FROM usage_tool_calls`, 1},
		{`SELECT count(*) FROM usage_tool_files`, 0},
		{`SELECT count(*) FROM usage_sessions WHERE first_at='2026-01-02' AND last_at='2026-01-02'`, 1},
	} {
		var got int
		if err := s.Store.DB.QueryRow(check.query).Scan(&got); err != nil || got != check.want {
			t.Fatalf("%s: %d %v", check.query, got, err)
		}
	}
	if n, err := s.cleanupOrphanedEvents(context.Background()); err != nil || n != 0 {
		t.Fatalf("no-orphan cleanup=%d %v", n, err)
	}
}

func TestCleanupOrphanedEventsKeepsAtomicRollback(t *testing.T) {
	s := orphanFixture(t)
	for _, query := range []string{
		`INSERT INTO usage_events VALUES('orphan','gone','codex','shared','2025-01-01')`,
		`INSERT INTO usage_tool_calls VALUES('tool','gone')`,
		`CREATE TRIGGER fail_cleanup BEFORE DELETE ON usage_tool_calls BEGIN SELECT RAISE(ABORT,'test failure'); END`,
	} {
		if _, err := s.Store.DB.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.cleanupOrphanedEvents(context.Background()); err == nil {
		t.Fatal("expected deletion failure")
	}
	var events int
	if err := s.Store.DB.QueryRow(`SELECT count(*) FROM usage_events`).Scan(&events); err != nil || events != 2 {
		t.Fatalf("partial cleanup committed: %d %v", events, err)
	}
}

func TestOrphanPathDiscoveryUsesBothCoveringIndexes(t *testing.T) {
	s := orphanFixture(t)
	rows, err := s.Store.DB.Query("EXPLAIN QUERY PLAN " + orphanUsagePathsSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(plan, "\n")
	for _, index := range []string{"COVERING INDEX usage_events_source", "COVERING INDEX usage_tool_calls_source"} {
		if !strings.Contains(text, index) {
			t.Fatalf("missing %s in plan:\n%s", index, text)
		}
	}
	t.Log(text)
}
