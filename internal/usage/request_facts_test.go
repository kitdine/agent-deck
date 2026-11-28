package usage

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestRequestFactsFollowSameCallAndSurviveScanRetry(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "2026", "calls.jsonl")
	writeFactsFixture(t, path, `{"type":"session_meta","payload":{"session_id":"s","cli_version":"0.169.0"}}
{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-6.2-astra","service_tier":"priority"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","service_tier":"default","activity":"guardian_approval","content":"private-fixture"}}
{"type":"event_msg","timestamp":"2026-11-18T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":272000,"cached_input_tokens":0,"cache_write_input_tokens":5,"output_tokens":1,"input_tokens_details":{"cache_write_tokens_details":{"ttl_30m_tokens":5,"ttl_12h_tokens":0}}},"total_token_usage":{"input_tokens":272000,"cache_write_input_tokens":5,"output_tokens":1}}}}
{"type":"event_msg","timestamp":"2026-11-18T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cache_write_input_tokens":2,"output_tokens":1,"input_tokens_details":{"cache_write_tokens_details":{"ttl_12h_tokens":2}}},"total_token_usage":{"input_tokens":272020,"cache_write_input_tokens":9,"output_tokens":3}}}}
{"type":"event_msg","timestamp":"2026-11-18T00:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":30,"cache_write_input_tokens":9,"output_tokens":2},"total_token_usage":{"input_tokens":272021,"cache_write_input_tokens":10,"output_tokens":4}}}}
`)
	db := openPricingTestStore(t, filepath.Join(root, "state"))
	service := New(db, home)

	// Failing the sidecar write must roll back the usage rows and checkpoint in
	// the batch publication, leaving the original key available for the retry.
	execPricingSQL(t, db, `CREATE TRIGGER reject_facts BEFORE INSERT ON usage_request_facts BEGIN SELECT RAISE(ABORT,'injected fact failure'); END`)
	_, scanErr := service.Scan(ctx)
	if scanErr == nil || !strings.Contains(scanErr.Error(), "injected fact failure") {
		t.Fatalf("scan error = %v", scanErr)
	}
	assertPricingCount(t, db, "usage_events", 0)
	assertPricingCount(t, db, "usage_request_facts", 0)
	assertPricingCount(t, db, "usage_source_files", 0)
	execPricingSQL(t, db, `DROP TRIGGER reject_facts`)
	if result, err := service.Scan(ctx); err != nil || result["imported"] != 3 {
		t.Fatalf("retry = %v, %v", result, err)
	}

	before, err := service.events(ctx, "codex", "s")
	if err != nil || len(before) != 3 {
		t.Fatalf("events = %v, %v", before, err)
	}
	first := before[0].Facts
	if first == nil || first.Source != "codex_jsonl" || first.ProducerVersion != "0.169.0" || first.ObservedAt != before[0].EventAt ||
		first.Association != "last_usage_matches_admitted_counts" || first.ExecutedTier != "" {
		t.Fatalf("first call facts = %+v", first)
	}
	assertPricingFact(t, first.InputTokens, 272000)
	assertPricingFact(t, first.CachedReadTokens, 0)
	assertPricingFact(t, first.CacheWrite30m, 5)
	assertPricingFact(t, first.CacheWrite12h, 0)
	for i, want := range []int64{272000, 20, 1} {
		if before[i].EventID != "turn" || before[i].Tokens["input_tokens"] != want || i > 0 && before[i].Facts != nil {
			t.Fatalf("call %d = %+v", i, before[i].Event)
		}
	}
	if before[0].Key == before[1].Key || before[1].Key == before[2].Key {
		t.Fatal("calls in one turn shared an event key")
	}

	appendFile, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = appendFile.WriteString(`{"type":"event_msg","timestamp":"2026-11-18T00:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":272001,"output_tokens":1},"total_token_usage":{"input_tokens":544022,"cache_write_input_tokens":10,"output_tokens":5}}}}
`)
	closeErr := appendFile.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append = %v, %v", err, closeErr)
	}
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	all, err := service.eventsRange(ctx, day, day.AddDate(0, 0, 1), "codex", "gpt-6.2-astra")
	if err != nil || len(all) != 4 {
		t.Fatalf("appended calls: count %d, err %v", len(all), err)
	}
	for i := range before {
		if !reflect.DeepEqual(all[i].Event, before[i].Event) {
			t.Fatalf("append changed event %q", before[i].Key)
		}
	}
	appended := all[3].Facts
	if appended == nil || appended.ProducerVersion != "0.169.0" || appended.CachedReadTokens != nil || appended.CacheWriteTokens != nil || appended.CacheWrite30m != nil {
		t.Fatalf("appended facts = %+v", appended)
	}
	assertPricingFact(t, appended.InputTokens, 272001)

	// A duplicate copy, a no-change scan and a rebuild all retain one event
	// and evidence row for each original request.
	copyPath := filepath.Join(filepath.Dir(path), "zz-copy.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFactsFixture(t, copyPath, string(data))
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Scan(ctx); err != nil || result["imported"] != 0 || result["updated"] != 0 {
		t.Fatalf("repeat = %v, %v", result, err)
	}
	if _, _, err = service.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	assertPricingCount(t, db, "usage_events", 4)
	assertPricingCount(t, db, "usage_request_facts", 2)
}

func TestRequestFactsClaudeDedupAndPortableRestore(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home, state := filepath.Join(root, "home"), filepath.Join(root, "state")
	path := filepath.Join(home, ".claude", "projects", "fixture", "calls.jsonl")
	writeFactsFixture(t, path, `{"type":"assistant","version":"2.1.4","sessionId":"s","timestamp":"2026-11-18T00:00:00Z","message":{"id":"a","model":"claude-haiku-5-5","usage":{"input_tokens":49000,"cache_read_input_tokens":40000,"cache_creation_input_tokens":11000,"cache_creation":{"ephemeral_5m_input_tokens":10000,"ephemeral_1h_input_tokens":1000},"speed":"fast","service_tier":"standard"},"content":[{"type":"text","text":"private-fixture"}]}}
{"type":"assistant","version":"2.1.4","sessionId":"s","timestamp":"2026-11-18T00:00:00Z","message":{"id":"a","model":"claude-haiku-5-5","usage":{"input_tokens":49000,"cache_read_input_tokens":40000,"cache_creation_input_tokens":11000,"cache_creation":{"ephemeral_5m_input_tokens":10000,"ephemeral_1h_input_tokens":1000},"output_tokens":5000,"speed":"fast","service_tier":"standard"}}}
{"type":"assistant","version":"untrusted-version","sessionId":"s","timestamp":"2026-11-18T00:00:01Z","message":{"id":"b","model":"claude-haiku-5-5","usage":{"input_tokens":1,"output_tokens":0,"service_tier":"priority"}}}
`)
	db := openPricingTestStore(t, state)
	s := New(db, home)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := s.events(ctx, "claude", "s")
	if err != nil || len(before) != 2 {
		t.Fatalf("events = %v, %v", before, err)
	}
	first, second := before[0].Facts, before[1].Facts
	if first == nil || first.ProducerVersion != "2.1.4" || first.ExecutedTier != "fast" || first.Source != "claude_jsonl" ||
		second == nil || second.ExecutedTier != "" || second.ProducerVersion != "" || second.CacheWrite5m != nil {
		t.Fatalf("Claude facts = %+v, %+v", first, second)
	}
	assertPricingFact(t, first.CacheWrite5m, 10000)
	assertPricingFact(t, first.CacheWrite1h, 1000)
	assertPricingFact(t, first.OutputTokens, 5000)
	assertPricingFact(t, second.OutputTokens, 0)
	assertPricingCount(t, db, "usage_request_facts", 2)
	var raw string
	if err := db.DB.QueryRowContext(ctx, `SELECT facts_json FROM usage_request_facts WHERE event_key=?`, before[0].Key).Scan(&raw); err != nil ||
		strings.Contains(raw, "private-fixture") || strings.Contains(raw, "content") {
		t.Fatalf("non-allowlisted values in facts, query error %v", err)
	}

	catalog, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.CatalogPriceRules(ctx, catalog.Version)
	if err != nil || rules == nil {
		t.Fatalf("bundled rules missing: %v", err)
	}

	machine := func(context.Context) (string, error) { return "pricing-facts-fixture", nil }
	archive := filepath.Join(root, "pricing.adb")
	b := backup.Service{Core: db, StateRoot: state, Vault: credentialvault.New(state, machine), Version: "fixture"}
	const passphrase = "fixture passphrase"
	if _, err = b.Create(ctx, archive, passphrase, false); err != nil {
		t.Fatal(err)
	}
	restoredState := filepath.Join(root, "restored")
	if _, err = backup.Restore(ctx, archive, restoredState, passphrase, machine); err != nil {
		t.Fatal(err)
	}
	restored := openPricingTestStore(t, restoredState)
	rs := New(restored, "")
	got, err := rs.events(ctx, "claude", "s")
	if err != nil || !reflect.DeepEqual(got, before) {
		t.Fatalf("restored facts: %d, %v", len(got), err)
	}
	restoredRules, err := rs.CatalogPriceRules(ctx, catalog.Version)
	if err != nil || !reflect.DeepEqual(rules, restoredRules) {
		t.Fatalf("restored rules differ: %v", err)
	}
	execPricingSQL(t, restored, `DELETE FROM usage_events WHERE event_key='claude:s:a'`)
	assertPricingCount(t, restored, "usage_request_facts", 1)
}

func TestRequestFactsLegacyReadOnlyLeavesEvidenceAbsent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db := openPricingTestStore(t, root)
	execPricingSQL(t, db, `DROP TABLE usage_request_facts; DROP TABLE price_rule_sets; UPDATE schema_metadata SET version=35;
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
VALUES('legacy','codex','s','turn','2026-11-18T00:00:00Z','gpt-5',12,'fixture',0)`)
	ro, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	s := New(ro, "")
	events, err := s.events(ctx, "codex", "s")
	if err != nil || len(events) != 1 || events[0].Facts != nil || events[0].Tokens["input_tokens"] != 12 {
		t.Fatalf("legacy read-only usage: %v, %v", events, err)
	}
	if rules, err := s.CatalogPriceRules(ctx, "legacy"); err != nil || rules != nil {
		t.Fatalf("legacy read-only rules: %v, %v", rules, err)
	}
}

func TestRequestFactsRequireOwnedTelemetryContribution(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, t.TempDir())
	execPricingSQL(t, db, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
VALUES('otel:one','codex','s','one','2026-11-18T00:00:00Z','gpt-5',20,3,'otel:one',0),
('unrelated','codex','s','turn','2026-11-18T00:00:00Z','gpt-5',20,3,'fixture',0);
INSERT INTO telemetry_observations(observation_key,source,producer,producer_version,originator,event_name,event_kind,transport,record_at,observed_at,received_at,model,response_id,request_id,attempt_id,thread_id,source_class,input_tokens,output_tokens,cache_write_tokens,metered,support_entry,state,reason)
VALUES('one','import','codex_exec','0.169.0','codex_exec','codex.sse_event','response.completed','sse','2026-11-18T00:00:00Z','2026-11-18T00:00:01Z','2026-11-18T00:00:02Z','gpt-5','response','','','s','',20,3,0,1,'fixture','otel_owned','');
INSERT INTO telemetry_contributions VALUES('one','otel:one');`)
	service := New(db, "")
	rows, err := service.events(ctx, "codex", "s")
	if err != nil || len(rows) != 2 || rows[0].Facts == nil || rows[1].Facts != nil {
		t.Fatalf("OTel/unrelated ownership: %v, %v", rows, err)
	}
	facts := rows[0].Facts
	if facts.Source != "codex_otel" || facts.Association != "owned_telemetry_contribution" || facts.ProducerVersion != "0.169.0" ||
		facts.ObservedAt != "2026-11-18T00:00:01Z" || facts.CachedReadTokens != nil || facts.ExecutedTier != "" {
		t.Fatalf("OTel facts: %+v", facts)
	}
	assertPricingFact(t, facts.InputTokens, 20)
	assertPricingFact(t, facts.CacheWriteTokens, 0)
	execPricingSQL(t, db, `UPDATE telemetry_observations SET state='conflict' WHERE observation_key='one'`)
	rows, err = service.events(ctx, "codex", "s")
	if err != nil || len(rows) != 2 || rows[0].Facts != nil {
		t.Fatalf("unowned OTel evidence exposed: %v, %v", rows, err)
	}
}

func writeFactsFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func openPricingTestStore(t *testing.T, root string) *store.Store {
	t.Helper()
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func execPricingSQL(t *testing.T, db *store.Store, query string) {
	t.Helper()
	if _, err := db.DB.ExecContext(context.Background(), query); err != nil {
		t.Fatal(err)
	}
}

func assertPricingCount(t *testing.T, db *store.Store, table string, want int) {
	t.Helper()
	var count int
	if err := db.DB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != want {
		t.Fatalf("%s count = %d, want %d, error %v", table, count, want, err)
	}
}

func assertPricingFact(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("fact absent, want %d", want)
	}
	if *got != want {
		t.Fatalf("fact = %d, want %d", *got, want)
	}
}
