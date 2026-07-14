package usage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCalculateSeparatesCachedAndClaudeTTLComponents(t *testing.T) {
	openAI := modelPrice{Provider: "openai", Prices: map[string]string{"input": "2", "cached_input": "0.5", "output": "8"}}
	got, err := Calculate("codex", "gpt", map[string]int64{"input_tokens": 1000000, "cached_input_tokens": 400000, "output_tokens": 100000}, openAI, "2")
	if err != nil || *got.CatalogBaseCost != "2.200000000" || *got.ProviderCost != "4.400000000" {
		t.Fatalf("cost = %#v, %v", got, err)
	}
	claude := modelPrice{Provider: "anthropic", Prices: map[string]string{"input": "3", "output": "15", "cache_write_5m": "3.75", "cache_write_1h": "6", "cache_read": "0.3"}}
	got, err = Calculate("claude", "claude", map[string]int64{"input_tokens": 100000, "output_tokens": 10000, "cache_write_5m_tokens": 200000, "cache_write_1h_tokens": 300000, "cache_read_tokens": 400000}, claude, "0.5")
	if err != nil || *got.CatalogBaseCost != "3.120000000" || *got.ProviderCost != "1.560000000" {
		t.Fatalf("ttl cost = %#v, %v", got, err)
	}
}

func TestParserRejectsMalformedTokenCounts(t *testing.T) {
	state := parseState{session: "s", turn: "t", model: "gpt-5.4"}
	_, ok := parse("codex", map[string]any{"type": "event_msg", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": -1}}}}, &state, "fixture", 0)
	if ok {
		t.Fatal("negative token count must be rejected")
	}
	_, ok = parse("claude", map[string]any{"type": "assistant", "sessionId": "s", "message": map[string]any{"id": "m", "model": "claude-sonnet-4-6", "usage": map[string]any{"input_tokens": "not-a-number"}}}, &parseState{}, "fixture", 0)
	if ok {
		t.Fatal("non-numeric Claude token count must be rejected")
	}
}

func TestSummarySessionsAndEventTimeCatalogSelection(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, filepath.Join(root, "home"))
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC) }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset) VALUES('e','codex','s','e','2026-07-13T01:00:00Z','gpt-5.4',1000000,0,0,'fixture',0)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, `INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s','2026-07-13T01:00:00Z','2026-07-13T01:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "2.500000000" || summary.ProviderCost == nil || *summary.ProviderCost != "2.500000000" || summary.Counts["historical"] != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	sessions, err := service.Sessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].SessionID != "s" {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
}

func TestUpdateLiteLLMFiltersAndPinsDirectProviders(t *testing.T) {
	body := `{"gpt":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000002},"bedrock":{"litellm_provider":"bedrock","input_cost_per_token":1,"output_cost_per_token":1},"claude":{"litellm_provider":"anthropic","input_cost_per_token":0.000003,"output_cost_per_token":0.000015,"cache_read_input_token_cost":0.0000003,"cache_creation_input_token_cost":0.00000375,"cache_creation_input_token_cost_above_1hr":0.000006}}`
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	url := "https://raw.githubusercontent.com/BerriAI/litellm/abcdef123/model_prices_and_context_window.json"
	got, err := service.UpdateLiteLLM(context.Background(), url, "abcdef123", client)
	if err != nil || got["models"].(int) != 2 {
		t.Fatalf("update=%v err=%v", got, err)
	}
	if _, err := service.UpdateLiteLLM(context.Background(), url, "bad", client); err == nil {
		t.Fatal("expected short commit rejection")
	}
	history, err := service.PriceHistory(context.Background())
	if err != nil || len(history) != 1 || history[0]["version"] != "litellm-abcdef123" {
		t.Fatalf("history=%v err=%v", history, err)
	}
}

func TestExactRunBindsOnlyItsTimeRangeAndStaleRecovery(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	now := time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	_, err = s.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'p','https://fixture.invalid','ref','2','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,multiplier_snapshot,selected_at) VALUES(1,'codex','2','2026-07-13T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	run, start, err := service.StartRun(ctx, "codex", 123)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('f','fixture',10,0,''); INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('inside','codex','s','i','2026-07-13T02:00:00Z','gpt-5.4','f',1),('outside','codex','s','o','2026-07-13T01:59:59Z','gpt-5.4','f',2); UPDATE usage_source_files SET cursor=2,prefix_hash='end' WHERE path='f'`)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err = service.EndRun(ctx, run, "codex", start); err != nil {
		t.Fatal(err)
	}
	var bound int
	if err = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_run_bindings WHERE run_id=?`, run).Scan(&bound); err != nil || bound != 1 {
		t.Fatalf("bindings=%d err=%v", bound, err)
	}
	if _, _, err = service.StartRun(ctx, "codex", 124); err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, "UPDATE usage_runs SET process_pid=999999 WHERE ended_at IS NULL")
	if err != nil {
		t.Fatal(err)
	}
	if recovered, err := service.RecoverStaleRuns(ctx); err != nil || recovered != 1 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
}

func TestExactRunUsesByteRangeAndExternalOverlapDowngrades(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	now := time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	service.ClientProcesses = func(string) ([]int, error) { return []int{777}, nil }
	_, err = s.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'p','x','r','2','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,multiplier_snapshot,selected_at) VALUES(1,'codex','2','2026-07-13T00:00:00Z'); INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('f','i',9,5,'start')`)
	if err != nil {
		t.Fatal(err)
	}
	run, start, err := service.StartRun(ctx, "codex", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('old','codex','s','old','2026-07-13T02:00:00Z','gpt','f',4),('new','codex','s','new','2026-01-01T00:00:00Z','gpt','f',5); UPDATE usage_source_files SET cursor=9,prefix_hash='end' WHERE path='f'`)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.EndRun(ctx, run, "codex", start); err != nil {
		t.Fatal(err)
	}
	var bindings, exact int
	if err = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_run_bindings WHERE run_id=?`, run).Scan(&bindings); err != nil || bindings != 1 {
		t.Fatalf("bindings=%d %v", bindings, err)
	}
	if err = s.DB.QueryRowContext(ctx, `SELECT exact FROM usage_runs WHERE id=?`, run).Scan(&exact); err != nil || exact != 0 {
		t.Fatalf("exact=%d %v", exact, err)
	}
}

func TestOfficialOverridesRetainCatalogComponentsAndProvenance(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) }
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if err = service.ImportOfficialOverrides(ctx, []OfficialOverride{{Model: "gpt-5.4", Provider: "openai", SourceURL: "https://openai.example/pricing", EffectiveFrom: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), Prices: map[string]string{"output": "9"}}}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, `INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z'); INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('e','codex','s','e','2026-07-13T00:00:00Z','gpt-5.4',1000000,1000000,'f',0)`)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Summary(ctx)
	if err != nil || summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "11.500000000" {
		base := ""
		if summary.CatalogBaseCost != nil {
			base = *summary.CatalogBaseCost
		}
		t.Fatalf("base=%s summary=%+v err=%v", base, summary, err)
	}
	history, err := service.PriceHistory(ctx)
	if err != nil || len(history) != 2 || history[0]["source_kind"] != "official" {
		t.Fatalf("history=%v err=%v", history, err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestScanDeduplicatesAndKeepsPartialLineForNextScan(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	state := filepath.Join(root, "state")
	s, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(home, ".codex", "sessions", "x.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{{"type": "session_meta", "timestamp": "2026-07-13T10:00:00Z", "payload": map[string]any{"session_id": "s"}}, {"type": "turn_context", "timestamp": "2026-07-13T10:00:01Z", "payload": map[string]any{"turn_id": "t", "model": "gpt-5.4"}}}
	var raw []byte
	for _, row := range rows {
		b, _ := json.Marshal(row)
		raw = append(raw, append(b, '\n')...)
	}
	event, _ := json.Marshal(map[string]any{"type": "event_msg", "timestamp": "2026-07-13T10:00:02Z", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": 10}}}})
	if err := os.WriteFile(path, append(raw, event...), 0600); err != nil {
		t.Fatal(err)
	}
	service := New(s, home)
	first, err := service.Scan(context.Background())
	if err != nil || first["imported"] != 0 {
		t.Fatalf("first = %#v, %v", first, err)
	}
	if err := os.WriteFile(path, append(append(raw, event...), '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := service.Scan(context.Background())
	if err != nil || second["imported"] != 1 {
		t.Fatalf("second = %#v, %v", second, err)
	}
	third, err := service.Scan(context.Background())
	if err != nil || third["imported"] != 0 {
		t.Fatalf("third = %#v, %v", third, err)
	}
}

// This sequence fixes the scanner's mutation contract: an append and a
// completed partial line advance normally; equal-prefix and growing rewrites,
// truncation, replacement, and an archive move rebuild only that source while
// stable event keys prevent duplicate logical usage.
func TestScanSourceMutationScenarios(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	s, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(home, ".codex", "sessions", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	line := func(v map[string]any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }
	meta := line(map[string]any{"type": "session_meta", "payload": map[string]any{"session_id": "s"}})
	turn := line(map[string]any{"type": "turn_context", "payload": map[string]any{"turn_id": "t", "model": "gpt-5.4"}})
	event := func(id string) []byte {
		return line(map[string]any{"type": "event_msg", "timestamp": "2026-07-13T00:00:00Z", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": float64(len(id))}}}})
	}
	writeScan := func(data []byte) {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(s, home).Scan(ctx); err != nil {
			t.Fatal(err)
		}
	}
	base := append(append([]byte{}, meta...), turn...)
	writeScan(append(base, event("one")...))                          // initial
	writeScan(append(append(base, event("one")...), event("two")...)) // append
	partial := event("three")
	if err := os.WriteFile(path, append(append(base, event("one")...), partial[:len(partial)-1]...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(s, home).Scan(ctx); err != nil {
		t.Fatal(err)
	} // partial
	writeScan(append(append(base, event("one")...), partial...)) // completed partial / growing rewrite
	writeScan(append(base, event("four")...))                    // equal-prefix replacement
	writeScan(append(base, event("f")...))                       // truncate
	writeScan(append(base, event("replacement")...))             // replacement
	archive := filepath.Join(home, ".codex", "archived_sessions", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, archive); err != nil {
		t.Fatal(err)
	}
	if _, err := New(s, home).Scan(ctx); err != nil {
		t.Fatal(err)
	} // archive move
	var n int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM usage_events").Scan(&n); err != nil || n != 1 {
		t.Fatalf("events=%d err=%v", n, err)
	}
}
