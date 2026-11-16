package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ownershipExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
func ownershipCount(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRolloutThreadIdentity(t *testing.T) {
	id := "019a1234-abcd-7000-8000-000000000001"
	other := "019a1234-abcd-7000-8000-000000000002"
	for _, name := range []string{"rollout-2026-10-01T12-13-14-" + id + ".jsonl", "/archive/rollout-2026-10-01T12-13-14-" + strings.ToUpper(id) + "_" + other + ".jsonl"} {
		if got, ok := CodexRolloutThreadID(name); !ok || got != id {
			t.Fatalf("%s => %q %v", name, got, ok)
		}
	}
	for _, name := range []string{id + ".jsonl", "rollout-" + id + ".jsonl", "other-2026-10-01T12-13-14-" + id + ".jsonl", "rollout-2026-10-01T12-13-14-" + id + "_bad.jsonl", "rollout-2026-10-01T12-13-14-" + id + ".jsonl.zst", "rollout-2026-10-01T12-13-1x-" + id + ".jsonl"} {
		if got, ok := CodexRolloutThreadID(name); ok {
			t.Fatalf("accepted %s => %q", name, got)
		}
	}
}

func TestTelemetryAdmissionSQLEquivalence(t *testing.T) {
	_, s := originTestStore(t)
	predicate, args := telemetryAdmissionSQL()
	versions := []string{"", "0.169.0", "2-rc.1", "/", "v/x", strings.Repeat("v", 256), strings.Repeat("v", 257), strings.Repeat("é", 128), strings.Repeat("é", 129), "\x00", "a\x00/b"}
	for _, family := range [][2]string{{"codex_exec", "codex_exec"}, {"codex-app-server", "codex-tui"}, {"codex_exec", "codex-tui"}, {"other", "other"}} {
		for _, v := range versions {
			for _, transport := range []string{"sse", "websocket"} {
				var got bool
				err := s.DB.QueryRow(`SELECT `+predicate+` FROM (SELECT ? AS producer, ? AS producer_version, ? AS originator, ? AS transport, ? AS event_name, ? AS event_kind)`, append(args, []any{family[0], v, family[1], transport, "codex.sse_event", "response.completed"}...)...).Scan(&got)
				want := TelemetryAdmitted(family[0], v, family[1], transport, "codex.sse_event", "response.completed")
				if err != nil || got != want {
					t.Fatalf("family=%v version=%q transport=%s sql=%v go=%v err=%v", family, v, transport, got, want, err)
				}
			}
		}
	}
}

func TestHistoricalReadmissionBoundedWithoutStarvation(t *testing.T) {
	_, s := originTestStore(t)
	ctx := context.Background()
	at := "2026-10-01T01:02:03Z"
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO telemetry_observations(observation_key,source,producer,producer_version,originator,event_name,event_kind,transport,record_at,observed_at,received_at,model,response_id,request_id,attempt_id,thread_id,source_class,input_tokens,output_tokens,metered,support_entry,state,reason) VALUES(?,'import','codex_exec',?,'codex_exec','codex.sse_event','response.completed','sse',?,'',?,'gpt-5','','','',?,'',100,10,1,'',?,'producer_unsupported')`
	for i := 0; i < 2001; i++ {
		if _, err = tx.Exec(insert, fmt.Sprintf("eligible-%04d", i), "0.169.0", at, at, fmt.Sprintf("t%d", i), "capture_only"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2100; i++ {
		if _, err = tx.Exec(insert, fmt.Sprintf("a-reject-%04d", i), "", at, at, "rejected", "capture_only"); err != nil {
			t.Fatal(err)
		}
	}
	for _, state := range []string{"invalid", "conflict", "unmetered"} {
		if _, err = tx.Exec(insert, state, "0.169.0", at, at, state, state); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = tx.Exec(insert, "z-missing", "0.169.0", at, at, "", "capture_only"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(insert, "z-incomplete", "0.169.0", at, at, "incomplete", "capture_only"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`UPDATE telemetry_observations SET output_tokens=NULL WHERE observation_key='z-incomplete'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ownershipExec(t, s.DB, `INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('inv',1,'2026-10-02T00:00:00Z',0,'complete')`)
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	first, err := s.SettleTelemetry(ctx, now, time.Minute)
	if err != nil || first.Owned != 2000 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := s.SettleTelemetry(ctx, now, time.Minute)
	if err != nil || second.Owned != 1 {
		t.Fatalf("second %+v %v", second, err)
	}
	third, err := s.SettleTelemetry(ctx, now, time.Minute)
	if err != nil || third.Changed {
		t.Fatalf("third %+v %v", third, err)
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM usage_events WHERE event_at=? AND input_tokens=100 AND output_tokens=10`, at); n != 2001 {
		t.Fatalf("historical counts/date %d", n)
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM telemetry_observations WHERE observation_key='z-missing' AND reason='conversation_missing' AND state='capture_only' AND record_at=? AND received_at=?`, at, at); n != 1 {
		t.Fatal("missing thread mutated incorrectly")
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM telemetry_observations WHERE observation_key LIKE 'a-reject-%' AND state='capture_only'`); n != 2100 {
		t.Fatal("invalid versions readmitted")
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM telemetry_observations WHERE observation_key IN ('invalid','conflict','unmetered') AND state=observation_key`); n != 3 {
		t.Fatal("terminal states changed")
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM telemetry_contributions`); n != 2001 {
		t.Fatalf("contributions %d", n)
	}
}

func TestSchema35PreservesFactsAndRestoresInvalidationTriggers(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	var old []migration
	for _, m := range migrations {
		if m.version <= 34 {
			old = append(old, m)
		}
	}
	if err = migrate(ctx, db, old); err != nil {
		t.Fatal(err)
	}
	ownershipExec(t, db, `INSERT INTO session_origins VALUES('codex','s','auxiliary','maintenance_origin','rule','input',1)`)
	ownershipExec(t, db, `INSERT INTO origin_overrides VALUES('codex','s','digest',0,'matched','work',1,'now')`)
	ownershipExec(t, db, `INSERT INTO origin_mutations VALUES('request','hash','override_set','codex','s','saved','work',1,'now')`)
	ownershipExec(t, db, `INSERT INTO origin_evidence VALUES('adapter','rule','codex','s','source','auxiliary','v','now')`)
	ownershipExec(t, db, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('source','identity',0,0,'')`)
	before := ownershipCount(t, db, `SELECT revision FROM serving_generation WHERE singleton=1`)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if n := ownershipCount(t, ro.DB, `SELECT version FROM schema_metadata`); n != 34 {
		t.Fatal("read-only migrated")
	}
	if n := ownershipCount(t, ro.DB, `SELECT count(*) FROM session_origins`); n != 1 {
		t.Fatal("lost old classifications")
	}
	ro.Close()
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM session_origins`); n != 0 {
		t.Fatal("kept derived origins")
	}
	for _, table := range []string{"origin_overrides", "origin_mutations", "origin_evidence", "usage_source_files"} {
		if n := ownershipCount(t, s.DB, `SELECT count(*) FROM `+table); n != 1 {
			t.Fatalf("lost %s", table)
		}
	}
	if n := ownershipCount(t, s.DB, `SELECT count(*) FROM usage_source_files WHERE thread_id IS NULL`); n != 1 {
		t.Fatal("not unassessed")
	}
	if n := ownershipCount(t, s.DB, `SELECT revision FROM serving_generation WHERE singleton=1`); n <= before {
		t.Fatal("stale serving generation")
	}
	for _, q := range []string{`INSERT INTO session_origins VALUES('codex','s','work','default_work','rule','input',1)`, `UPDATE session_origins SET input_token='new'`, `DELETE FROM session_origins`} {
		before = ownershipCount(t, s.DB, `SELECT revision FROM serving_generation WHERE singleton=1`)
		ownershipExec(t, s.DB, q)
		if after := ownershipCount(t, s.DB, `SELECT revision FROM serving_generation WHERE singleton=1`); after <= before {
			t.Fatal("missing invalidation trigger")
		}
	}
	backup := filepath.Join(t.TempDir(), "snapshot.sqlite3")
	if err = s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restoredRoot := t.TempDir()
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(restoredRoot, "agentdeck.sqlite3"), data, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(ctx, restoredRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if n := ownershipCount(t, restored.DB, `SELECT count(*) FROM origin_overrides`); n != 1 {
		t.Fatal("backup lost override")
	}
	ownershipExec(t, restored.DB, `UPDATE schema_metadata SET version=36`)
	restored.Close()
	if ahead, err := Open(ctx, restoredRoot); err == nil {
		ahead.Close()
		t.Fatal("accepted newer schema")
	}
}
