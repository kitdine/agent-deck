package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type countingSourceFile struct {
	*os.File
	bytes *int64
}

func (f countingSourceFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	*f.bytes += int64(n)
	return n, err
}
func (f countingSourceFile) ReadAt(p []byte, off int64) (int, error) {
	n, err := f.File.ReadAt(p, off)
	*f.bytes += int64(n)
	return n, err
}

func TestIncrementalScanReadsNoUnchangedContentAndOnlyAppendSuffix(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(home, ".codex", "sessions", "2026", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := `{"type":"session_meta","payload":{"session_id":"s"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-15T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":1,"output_tokens":2}}}}` + "\n"
	if err := os.WriteFile(source, []byte(prefix), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, home)
	var readBytes int64
	var opens int
	service.Open = func(path string) (SourceFile, error) {
		opens++
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return countingSourceFile{File: file, bytes: &readBytes}, nil
	}
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	readBytes = 0
	opens = 0
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if opens != 0 || readBytes != 0 {
		t.Fatalf("unchanged scan opens=%d bytes=%d", opens, readBytes)
	}
	appendix := `{"type":"turn_context","payload":{"turn_id":"t2","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-15T00:01:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"cached_input_tokens":2,"output_tokens":3}}}}` + "\n"
	file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(appendix); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	readBytes = 0
	opens = 0
	result, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result["imported"] != 1 || opens != 1 || readBytes > int64(len(appendix)+8192) {
		t.Fatalf("append result=%v opens=%d bytes=%d suffix=%d", result, opens, readBytes, len(appendix))
	}
	fingerprint, err := service.InventoryFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := database.Setting(ctx, "watch.fingerprint.usage")
	if err != nil || !found || stored != fingerprint {
		t.Fatalf("checkpoint=%q found=%t err=%v want=%q", stored, found, err, fingerprint)
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

func TestBundledCatalogUsesStableProvenanceAndAcceptsLegacy(t *testing.T) {
	const stableSource = "bundled://agentdeck/model-prices.json"
	const legacySource = "bundled://config/model-prices.json"

	var metadata struct {
		Sources []struct {
			URL string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(bundledCatalog, &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Sources) != 1 || metadata.Sources[0].URL != stableSource {
		t.Fatalf("bundled catalog source = %#v, want %q", metadata.Sources, stableSource)
	}

	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = New(s, "").ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	var importedSource string
	if err = s.DB.QueryRowContext(ctx, "SELECT source_url FROM price_catalogs").Scan(&importedSource); err != nil {
		t.Fatal(err)
	}
	if importedSource != stableSource {
		t.Fatalf("imported bundled source = %q, want %q", importedSource, stableSource)
	}

	hash := strings.Repeat("a", 64)
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "stable", url: stableSource, want: true},
		{name: "legacy", url: legacySource, want: true},
		{name: "unknown", url: "bundled://other/model-prices.json", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validPriceProvenance("bundled", test.url, "", hash, 1); got != test.want {
				t.Fatalf("validPriceProvenance(%q) = %t, want %t", test.url, got, test.want)
			}
		})
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
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	url := "https://raw.githubusercontent.com/BerriAI/litellm/" + commit + "/model_prices_and_context_window.json"
	var requests []string
	client := &http.Client{Transport: roundTrip(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.String())
		switch request.URL.String() {
		case liteLLMLatestCommitURL:
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"sha":"` + commit + `"}`)), Header: make(http.Header)}, nil
		case url:
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", request.URL)
		}
	})}
	got, err := service.UpdateLiteLLM(context.Background(), "", client)
	wantHash := hash([]byte(body))
	if err != nil || got["models"].(int) != 2 || got["commit_sha"] != commit || got["content_sha256"] != wantHash {
		t.Fatalf("update=%v err=%v", got, err)
	}
	if len(requests) != 2 || requests[0] != liteLLMLatestCommitURL || requests[1] != url {
		t.Fatalf("automatic update requests = %v", requests)
	}
	if _, err = service.UpdateLiteLLM(context.Background(), commit, client); err != nil {
		t.Fatalf("pinned update: %v", err)
	}
	if len(requests) != 3 || requests[2] != url {
		t.Fatalf("pinned update requests = %v", requests)
	}
	if _, err := service.UpdateLiteLLM(context.Background(), "bad", client); err == nil {
		t.Fatal("expected short commit rejection")
	}
	history, err := service.PriceHistory(context.Background())
	if err != nil || len(history) != 1 || history[0]["version"] != "litellm-"+commit || history[0]["content_sha256"] != wantHash {
		t.Fatalf("history=%v err=%v", history, err)
	}
	status, err := service.PriceStatus(context.Background())
	if err != nil || status["content_sha256"] != wantHash {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestPriceHTTPClientUsesBoundedTimeout(t *testing.T) {
	client := PriceHTTPClient()
	if client == nil || client.Timeout != 60*time.Second {
		t.Fatalf("price HTTP client = %#v, want timeout %s", client, 60*time.Second)
	}
}

func TestUpdateLiteLLMReportsLatestCommitFailures(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTrip
		want      string
	}{
		{name: "transport", transport: func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }, want: "offline"},
		{name: "status", transport: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Body: io.NopCloser(strings.NewReader("denied")), Header: make(http.Header)}, nil
		}, want: "resolve latest LiteLLM commit: 403 Forbidden"},
		{name: "malformed", transport: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{")), Header: make(http.Header)}, nil
		}, want: "resolve latest LiteLLM commit:"},
		{name: "invalid SHA", transport: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"sha":"main"}`)), Header: make(http.Header)}, nil
		}, want: "invalid SHA"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			_, err = New(s, "").UpdateLiteLLM(context.Background(), "", &http.Client{Transport: test.transport})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("latest commit error = %v, want %q", err, test.want)
			}
			history, historyErr := New(s, "").PriceHistory(context.Background())
			if historyErr != nil || len(history) != 0 {
				t.Fatalf("history after failed update = %v, %v", history, historyErr)
			}
		})
	}
}

func TestUpdateLiteLLMReportsPinnedCatalogFailures(t *testing.T) {
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	tests := []struct {
		name      string
		transport roundTrip
		want      string
	}{
		{name: "transport", transport: func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }, want: "offline"},
		{name: "status", transport: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Bad Gateway", Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
		}, want: "price update: 502 Bad Gateway"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			_, err = New(s, "").UpdateLiteLLM(context.Background(), commit, &http.Client{Transport: test.transport})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("pinned catalog error = %v, want %q", err, test.want)
			}
			history, historyErr := New(s, "").PriceHistory(context.Background())
			if historyErr != nil || len(history) != 0 {
				t.Fatalf("history after failed update = %v, %v", history, historyErr)
			}
		})
	}
}

func TestPriceDiagnosticsValidatesLiteLLMProvenanceAndCountsDistinctModels(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	hash := strings.Repeat("a", 64)
	validURL := "https://raw.githubusercontent.com/BerriAI/litellm/" + commit + "/model_prices_and_context_window.json"
	if _, err = s.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('good','litellm',?,?,?,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1),('bad-commit','litellm',?,?,?,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1),('bad-url','litellm',?,?,?,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1),('bad-hash','official','https://example.invalid','','short','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1)`, validURL, commit, hash, validURL, "short", hash, "https://example.invalid/not-pinned", commit, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset) VALUES('one','codex','s','one','2026-01-01T00:00:00Z','missing-model',1,0,1,'fixture',0),('two','codex','s','two','2026-01-01T00:00:00Z','missing-model',2,0,2,'fixture',1)`); err != nil {
		t.Fatal(err)
	}
	invalid, unpriced, err := New(s, "").PriceDiagnostics(ctx)
	if err != nil || invalid != 3 || unpriced != 1 {
		t.Fatalf("PriceDiagnostics = invalid:%d unpriced:%d err:%v", invalid, unpriced, err)
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
	_, err = s.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'p','https://fixture.invalid','ref','2','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','p','https://fixture.invalid','2','2026-07-13T00:00:00Z')`)
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

func TestStartRunUsesCompletedOfficialSwitch(t *testing.T) {
	for _, withPriorBearer := range []bool{false, true} {
		name := "clean"
		if withPriorBearer {
			name = "after-bearer"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			base := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
			if withPriorBearer {
				created, createErr := s.CreateProvider(ctx, store.Provider{Name: "bearer", Endpoint: "https://provider.example", CredentialRef: "ref", Multiplier: "7", Clients: []store.ClientMapping{{Client: "codex"}}})
				if createErr != nil {
					t.Fatal(createErr)
				}
				providerID := created.ID
				if err := s.CreateOperation(ctx, store.Operation{ID: "bearer", Kind: "provider.use", State: "completed", ProviderID: &providerID, Client: "codex", StartedAt: base, UpdatedAt: base.Add(time.Second)}); err != nil {
					t.Fatal(err)
				}
				if err := s.RecordSelection(ctx, store.Selection{ProviderID: created.ID, Client: "codex", MultiplierSnapshot: "7", SelectedAt: base.Add(500 * time.Millisecond)}); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.CreateOperation(ctx, store.Operation{ID: "official", Kind: "provider.use", State: "completed", Client: "codex", StartedAt: base.Add(2 * time.Second), UpdatedAt: base.Add(3 * time.Second)}); err != nil {
				t.Fatal(err)
			}
			service := New(s, "")
			service.Now = func() time.Time { return base.Add(4 * time.Second) }
			runID, _, err := service.StartRun(ctx, "codex", 123)
			if err != nil {
				t.Fatal(err)
			}
			var providerName, multiplier string
			if err := s.DB.QueryRowContext(ctx, "SELECT provider,multiplier FROM usage_runs WHERE id=?", runID).Scan(&providerName, &multiplier); err != nil {
				t.Fatal(err)
			}
			if providerName != "official" || multiplier != "1" {
				t.Fatalf("official run provider=%q multiplier=%q", providerName, multiplier)
			}
		})
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
	_, err = s.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'p','x','r','2','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','p','x','2','2026-07-13T00:00:00Z'); INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('f','i',9,5,'start')`)
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

func TestRemovedSourceCleansStateEventsRunsAndSessionAggregation(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "removed.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"session_meta","payload":{"session_id":"removed"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-15T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10}}}}` + "\n"
	if err = os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(99,'codex','official','1','2026-07-15T00:00:00Z','2026-07-15T00:01:00Z',1,''); INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(99,?,0,'')`, path); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	inventory, err := service.Inventory(ctx)
	if err != nil || len(inventory.Removed) != 1 || inventory.Removed[0] != path {
		t.Fatalf("removed inventory = %#v, %v", inventory, err)
	}
	if _, err = service.ScanInventory(ctx, inventory); err != nil {
		t.Fatal(err)
	}
	for table, query := range map[string]string{
		"source":     "SELECT count(*) FROM usage_source_files",
		"event":      "SELECT count(*) FROM usage_events",
		"session":    "SELECT count(*) FROM usage_sessions",
		"run source": "SELECT count(*) FROM usage_run_sources",
	} {
		var count int
		if err = database.DB.QueryRowContext(ctx, query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s count = %d, %v", table, count, err)
		}
	}
}

func TestStableInventoryCheckpointDoesNotSwallowConcurrentAppend(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "race.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := `{"type":"session_meta","payload":{"session_id":"race"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"one","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-15T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}` + "\n"
	if err = os.WriteFile(path, []byte(prefix), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	inventory, err := service.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	appendix := `{"type":"turn_context","payload":{"turn_id":"two","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-15T00:01:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2}}}}` + "\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(appendix); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ScanInventory(ctx, inventory); err != nil {
		t.Fatal(err)
	}
	current, err := service.InventoryFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, _, err := database.Setting(ctx, "watch.fingerprint.usage")
	if err != nil || checkpoint == current {
		t.Fatalf("checkpoint swallowed append: checkpoint=%q current=%q err=%v", checkpoint, current, err)
	}
	result, err := service.Scan(ctx)
	if err != nil || result["imported"] != 1 {
		t.Fatalf("follow-up scan = %#v, %v", result, err)
	}
}

func TestSourceResetFailureRollsBackEntireSourceRebuild(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "reset.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	makeContents := func(tokens int) string {
		return `{"type":"session_meta","payload":{"session_id":"reset"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-07-15T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d}}}}`, tokens) + "\n"
	}
	if err = os.WriteFile(path, []byte(makeContents(1)), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var oldCursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", path).Scan(&oldCursor); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(makeContents(999)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_usage_rebuild BEFORE INSERT ON usage_events BEGIN SELECT RAISE(FAIL,'injected usage rebuild failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Scan(ctx); err == nil {
		t.Fatal("source rebuild succeeded")
	}
	var tokens, cursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT input_tokens FROM usage_events WHERE event_key='codex:reset:turn'").Scan(&tokens); err != nil || tokens != 1 {
		t.Fatalf("event after rollback = %d, %v", tokens, err)
	}
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", path).Scan(&cursor); err != nil || cursor != oldCursor {
		t.Fatalf("source cursor after rollback = %d, %v want %d", cursor, err, oldCursor)
	}
}
