package usage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
)

type queryCountingDriver struct {
	inner   driver.Driver
	queries *atomic.Int64
}

func (d *queryCountingDriver) Open(name string) (driver.Conn, error) {
	connection, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &queryCountingConn{Conn: connection, queries: d.queries}, nil
}

type queryCountingConn struct {
	driver.Conn
	queries *atomic.Int64
}

func (c *queryCountingConn) Prepare(query string) (driver.Stmt, error) {
	statement, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &queryCountingStmt{Stmt: statement, queries: c.queries}, nil
}

func (c *queryCountingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	statement, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &queryCountingStmt{Stmt: statement, queries: c.queries}, nil
}

func (c *queryCountingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	querier, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	c.queries.Add(1)
	return querier.QueryContext(ctx, query, args)
}

type queryCountingStmt struct {
	driver.Stmt
	queries *atomic.Int64
}

func (s *queryCountingStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.queries.Add(1)
	return s.Stmt.Query(args)
}

func (s *queryCountingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	querier, ok := s.Stmt.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	s.queries.Add(1)
	return querier.QueryContext(ctx, args)
}

func openQueryCountingStore(t *testing.T, path string) (*store.Store, *atomic.Int64) {
	t.Helper()
	queries := &atomic.Int64{}
	driverName := fmt.Sprintf("agentdeck-stats-query-count-%p", queries)
	sql.Register(driverName, &queryCountingDriver{inner: &sqlite.Driver{}, queries: queries})
	database, err := sql.Open(driverName, path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	return &store.Store{DB: database}, queries
}

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

func TestCodexScanKeepsEveryTokenCountInOneTurn(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(home, ".codex", "sessions", "2026", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":1},"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":1}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"cached_input_tokens":8,"output_tokens":2},"total_token_usage":{"input_tokens":30,"cached_input_tokens":12,"output_tokens":3}}}}` + "\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	result, err := New(database, home).Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result["imported"] != 2 {
		t.Fatalf("imported = %d, want 2", result["imported"])
	}
	var events, turns int
	var input, cached, output int64
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(DISTINCT event_id),SUM(input_tokens),SUM(cached_input_tokens),SUM(output_tokens) FROM usage_events WHERE client='codex' AND session_id='session'`).Scan(&events, &turns, &input, &cached, &output); err != nil {
		t.Fatal(err)
	}
	if events != 2 || turns != 1 || input != 30 || cached != 12 || output != 3 {
		t.Fatalf("events=%d turns=%d input=%d cached=%d output=%d", events, turns, input, cached, output)
	}
}

func TestCodexCumulativeUsageSurvivesAppendRestartResetAndArchiveCopy(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	stateRoot := filepath.Join(root, "state")
	source := filepath.Join(home, ".codex", "sessions", "2026", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	line := func(timestamp string, last, total any) string {
		info := map[string]any{}
		if last != nil {
			info["last_token_usage"] = last
		}
		if total != nil {
			info["total_token_usage"] = total
		}
		encoded, err := json.Marshal(map[string]any{"type": "event_msg", "timestamp": timestamp, "payload": map[string]any{"type": "token_count", "info": info}})
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded) + "\n"
	}
	tokens := func(input int) map[string]any {
		return map[string]any{"input_tokens": input, "cached_input_tokens": 0, "output_tokens": 0}
	}
	prefix := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n"
	initial := prefix +
		line("2026-07-20T00:00:01Z", tokens(10), tokens(10)) +
		line("2026-07-20T00:00:02Z", tokens(20), tokens(30)) +
		line("2026-07-20T00:00:03Z", tokens(999), tokens(30))
	if err := os.WriteFile(source, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result, scanErr := New(database, home).Scan(ctx); scanErr != nil || result["imported"] != 2 {
		database.Close()
		t.Fatalf("initial scan result=%v err=%v", result, scanErr)
	}
	var cumulativeJSON string
	if err = database.DB.QueryRowContext(ctx, `SELECT codex_cumulative_json FROM usage_source_files WHERE path=?`, source).Scan(&cumulativeJSON); err != nil || !strings.Contains(cumulativeJSON, `"input_tokens":30`) {
		database.Close()
		t.Fatalf("initial persisted cumulative cursor=%q err=%v", cumulativeJSON, err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	appendix := line("2026-07-20T00:00:04Z", tokens(5), tokens(5)) +
		line("2026-07-20T00:00:05Z", tokens(7), nil) +
		line("2026-07-20T00:00:06Z", tokens(8), tokens(20)) +
		line("2026-07-20T00:00:07Z", nil, tokens(30)) +
		line("2026-07-20T00:00:08Z", nil, nil)
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
	database, err = store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if result, scanErr := New(database, home).Scan(ctx); scanErr != nil || result["imported"] != 4 {
		t.Fatalf("restart append scan result=%v err=%v", result, scanErr)
	}
	var events int
	var input int64
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(input_tokens) FROM usage_events`).Scan(&events, &input); err != nil {
		t.Fatal(err)
	}
	if events != 6 || input != 60 {
		t.Fatalf("after restart events=%d input=%d, want 6 and 60", events, input)
	}
	archive := filepath.Join(home, ".codex", "archived_sessions", "copy.jsonl")
	if err = os.MkdirAll(filepath.Dir(archive), 0o700); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(archive, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = New(database, home).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(input_tokens) FROM usage_events`).Scan(&events, &input); err != nil {
		t.Fatal(err)
	}
	if events != 6 || input != 60 {
		t.Fatalf("archive copy events=%d input=%d, want stable deduplicated usage", events, input)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT codex_cumulative_json FROM usage_source_files WHERE path=?`, source).Scan(&cumulativeJSON); err != nil || cumulativeJSON != "{}" {
		t.Fatalf("persisted cumulative cursor=%q err=%v", cumulativeJSON, err)
	}
}

func TestDuplicateSourceOwnershipSurvivesEitherCopyRemoval(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(home, ".codex", "sessions", "2026", "fixture.jsonl")
	archive := filepath.Join(home, ".codex", "archived_sessions", "copy.jsonl")
	unrelated := filepath.Join(home, ".codex", "sessions", "2026", "unrelated.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10},"total_token_usage":{"input_tokens":10}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20},"total_token_usage":{"input_tokens":30}}}}` + "\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelatedContents := `{"type":"session_meta","payload":{"session_id":"unrelated"}}` + "\n" +
		strings.Repeat(`{"type":"response_item","payload":{"type":"message"}}`+"\n", 4096)
	if err := os.WriteFile(unrelated, []byte(unrelatedContents), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var boundEvent string
	if err = database.DB.QueryRowContext(ctx, `SELECT event_key FROM usage_events ORDER BY event_at LIMIT 1`).Scan(&boundEvent); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(300,'codex','official','1','2026-07-20T00:00:00Z','2026-07-20T00:01:00Z',1,''); INSERT INTO usage_run_bindings(event_key,run_id) VALUES(?,300)`, boundEvent); err != nil {
		t.Fatal(err)
	}
	assertUsage := func(stage, owner string, wantEvents, wantBindings int) {
		t.Helper()
		var events, bindings int
		var input sql.NullInt64
		if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(input_tokens) FROM usage_events`).Scan(&events, &input); err != nil {
			t.Fatal(err)
		}
		if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_run_bindings WHERE run_id=300`).Scan(&bindings); err != nil {
			t.Fatal(err)
		}
		wantInput := int64(0)
		if wantEvents > 0 {
			wantInput = 30
		}
		if events != wantEvents || input.Int64 != wantInput || bindings != wantBindings {
			t.Fatalf("%s events=%d input=%d bindings=%d", stage, events, input.Int64, bindings)
		}
		if owner != "" {
			var owners int
			if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE source_path=?`, owner).Scan(&owners); err != nil {
				t.Fatal(err)
			}
			if owners != wantEvents {
				t.Fatalf("%s owner %q has %d events, want %d", stage, owner, owners, wantEvents)
			}
			var cumulative string
			if err := database.DB.QueryRowContext(ctx, `SELECT codex_cumulative_json FROM usage_source_files WHERE path=?`, owner).Scan(&cumulative); err != nil || !strings.Contains(cumulative, `"input_tokens":30`) {
				t.Fatalf("%s cumulative cursor=%q err=%v", stage, cumulative, err)
			}
		}
	}
	copyToArchive := func() {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(archive), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(archive, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Scan(ctx); err != nil {
			t.Fatal(err)
		}
	}

	copyToArchive()
	assertUsage("after archive copy", source, 2, 1)
	if err = os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	assertUsage("after archive removal", source, 2, 1)

	copyToArchive()
	var unrelatedOpens int
	var unrelatedReadBytes int64
	service.Open = func(path string) (SourceFile, error) {
		file, openErr := os.Open(path)
		if openErr != nil {
			return nil, openErr
		}
		if path == unrelated {
			unrelatedOpens++
			return countingSourceFile{File: file, bytes: &unrelatedReadBytes}, nil
		}
		return file, nil
	}
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	originalOpen := service.Open
	service.Open = func(path string) (SourceFile, error) {
		if path == archive {
			return nil, errors.New("synthetic archive read failure")
		}
		return originalOpen(path)
	}
	if _, err = service.Scan(ctx); err == nil {
		t.Fatal("scan unexpectedly recovered ownership without reading the remaining source")
	}
	var orphanedEvents, preservedBindings int
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE source_path=?`, source).Scan(&orphanedEvents); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_run_bindings WHERE run_id=300`).Scan(&preservedBindings); err != nil {
		t.Fatal(err)
	}
	if orphanedEvents != 2 || preservedBindings != 1 {
		t.Fatalf("failed recovery orphaned events=%d bindings=%d", orphanedEvents, preservedBindings)
	}
	service.Open = originalOpen
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	assertUsage("after original removal", archive, 2, 1)
	if unrelatedOpens != 0 || unrelatedReadBytes != 0 {
		t.Fatalf("original removal opened unrelated source %d times and read %d bytes", unrelatedOpens, unrelatedReadBytes)
	}

	unrelatedOpens, unrelatedReadBytes = 0, 0
	if err = os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	assertUsage("after final source removal", "", 0, 0)
	if unrelatedOpens != 0 || unrelatedReadBytes != 0 {
		t.Fatalf("final removal opened unrelated source %d times and read %d bytes", unrelatedOpens, unrelatedReadBytes)
	}
}

func TestCodexParserAcceptsCurrentSessionMetaID(t *testing.T) {
	state := parseState{}
	_, ok := parse("codex", map[string]any{"type": "session_meta", "payload": map[string]any{"id": "session"}}, &state, "fixture", 0)
	if ok || state.session != "session" {
		t.Fatalf("session metadata parsed=%t session=%q", ok, state.session)
	}
}

func TestUsageParserVersionRebuildsUnchangedSource(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(home, ".codex", "sessions", "2026", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20}}}}` + "\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM usage_events WHERE client='codex' AND session_id='session'`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('codex:session:turn','codex','session','turn','2026-07-20T00:00:02Z','gpt-5.4',999,?,0)`, source); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE usage_source_files SET parser_version=0 WHERE path=?`, source); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(200,'codex','official','1','2026-07-20T00:00:00Z','2026-07-20T00:01:00Z',1,'')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_run_sources(run_id,path,start_offset,end_offset,start_hash,end_hash) VALUES(200,?,0,?,'','')`, source, len(contents)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_run_bindings(event_key,run_id) VALUES('codex:session:turn',200)`); err != nil {
		t.Fatal(err)
	}
	result, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var events, parserVersion, bindings int
	var input int64
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(input_tokens) FROM usage_events WHERE client='codex' AND session_id='session'`).Scan(&events, &input); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT parser_version FROM usage_source_files WHERE path=?`, source).Scan(&parserVersion); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_run_bindings b JOIN usage_events e ON e.event_key=b.event_key WHERE b.run_id=200 AND e.client='codex' AND e.session_id='session'`).Scan(&bindings); err != nil {
		t.Fatal(err)
	}
	if result["replaced"] == 0 || events != 2 || input != 30 || parserVersion != usageParserVersion || bindings != 2 {
		t.Fatalf("result=%v events=%d input=%d parser_version=%d bindings=%d", result, events, input, parserVersion, bindings)
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

func TestSummaryKeepsCompleteTotalsUnavailableAndReportsKnownSubtotal(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('priced','codex','s','priced','2026-07-13T01:00:00Z','gpt-5.4',1000000,'fixture',0),('unknown','codex','s','unknown','2026-07-13T01:01:00Z','codex-auto-review',1000000,'fixture',1); INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s','2026-07-13T01:00:00Z','2026-07-13T01:01:00Z')`); err != nil {
		t.Fatal(err)
	}
	summary, err := service.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CatalogBaseCost != nil || summary.ProviderCost != nil {
		t.Fatalf("partial totals must remain unavailable: %#v", summary)
	}
	if summary.KnownCatalogBaseCost == nil || *summary.KnownCatalogBaseCost != "2.500000000" || summary.KnownProviderCost == nil || *summary.KnownProviderCost != "2.500000000" {
		t.Fatalf("known subtotal = %#v", summary)
	}
	if summary.Counts["priced"] != 1 || summary.Counts["unpriced"] != 1 || len(summary.Models) != 2 {
		t.Fatalf("coverage = counts:%#v models:%#v", summary.Counts, summary.Models)
	}
	if summary.Models[0].Model != "codex-auto-review" || summary.Models[0].UnpricedEvents != 1 || summary.Models[1].Model != "gpt-5.4" || summary.Models[1].PricedEvents != 1 {
		t.Fatalf("model coverage = %#v", summary.Models)
	}
	sessions, err := service.Sessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].KnownCatalogBaseCost == nil || *sessions[0].KnownCatalogBaseCost != "2.500000000" {
		t.Fatalf("session known subtotal = %#v, %v", sessions, err)
	}
}

func TestClaudeVersionPunctuationMatchesEquivalentCatalogModel(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('fixture','official','https://example.invalid','','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1); INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES('fixture','claude-haiku-4-5','anthropic','2026-01-01T00:00:00Z','{"input":"3"}','null'); INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('e','claude','s','e','2026-07-13T01:00:00Z','claude-haiku-4.5',1000000,'fixture',0)`); err != nil {
		t.Fatal(err)
	}
	summary, err := New(s, "").Summary(ctx)
	if err != nil || summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "3.000000000" || summary.Counts["unpriced"] != 0 {
		t.Fatalf("normalized summary = %#v, %v", summary, err)
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
	if err != nil || len(history) != 1 || history[0].Version != "litellm-"+commit || history[0].ContentSHA256 != wantHash {
		t.Fatalf("history=%v err=%v", history, err)
	}
	status, err := service.PriceStatus(context.Background())
	if err != nil || status["content_sha256"] != wantHash {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestUpdateLiteLLMRetriesTruncatedCommitAndCatalogResponses(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	url := liteLLMPriceURLPrefix + commit + liteLLMPriceURLSuffix
	body := `{"gpt":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000002}}`
	requests := map[string]int{}
	client := &http.Client{Transport: roundTrip(func(request *http.Request) (*http.Response, error) {
		requests[request.URL.String()]++
		contents := "{"
		if requests[request.URL.String()] > 1 {
			switch request.URL.String() {
			case liteLLMLatestCommitURL:
				contents = `{"sha":"` + commit + `"}`
			case url:
				contents = body
			default:
				return nil, fmt.Errorf("unexpected URL %s", request.URL)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(contents)), Header: make(http.Header)}, nil
	})}
	result, err := New(s, "").UpdateLiteLLM(ctx, "", client)
	if err != nil || result["commit_sha"] != commit || requests[liteLLMLatestCommitURL] != 2 || requests[url] != 2 {
		t.Fatalf("retry result=%#v requests=%#v err=%v", result, requests, err)
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
	if err != nil || len(history) != 2 || history[0].SourceKind != "official" {
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

func TestScanAcceptsContinuousAppendAfterInventorySnapshot(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "continuous.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	line := func(value map[string]any) []byte {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return append(encoded, '\n')
	}
	contents := append([]byte{}, line(map[string]any{"type": "session_meta", "payload": map[string]any{"session_id": "continuous"}})...)
	contents = append(contents, line(map[string]any{"type": "turn_context", "payload": map[string]any{"turn_id": "one", "model": "gpt-5.4"}})...)
	contents = append(contents, line(map[string]any{"type": "event_msg", "timestamp": "2026-07-17T00:00:00Z", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": 1}}}})...)
	appendices := [][]byte{
		append(line(map[string]any{"type": "turn_context", "payload": map[string]any{"turn_id": "two", "model": "gpt-5.4"}}), line(map[string]any{"type": "event_msg", "timestamp": "2026-07-17T00:01:00Z", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": 2}}}})...),
		append(line(map[string]any{"type": "turn_context", "payload": map[string]any{"turn_id": "three", "model": "gpt-5.4"}}), line(map[string]any{"type": "event_msg", "timestamp": "2026-07-17T00:02:00Z", "payload": map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": 3}}}})...),
	}
	if err = os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	for index, appendix := range appendices {
		service.Stat = os.Stat
		inventory, inventoryErr := service.Inventory(ctx)
		if inventoryErr != nil {
			t.Fatal(inventoryErr)
		}
		appended := false
		service.Stat = func(name string) (os.FileInfo, error) {
			if !appended && name == path {
				file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if openErr != nil {
					return nil, openErr
				}
				_, writeErr := file.Write(appendix)
				closeErr := file.Close()
				if writeErr != nil {
					return nil, writeErr
				}
				if closeErr != nil {
					return nil, closeErr
				}
				appended = true
			}
			return os.Stat(name)
		}
		result, scanErr := service.ScanInventory(ctx, inventory)
		if scanErr != nil || result["imported"] != 1 {
			t.Fatalf("scan %d = %#v, %v", index, result, scanErr)
		}
	}
	service.Stat = os.Stat
	result, err := service.Scan(ctx)
	if err != nil || result["imported"] != 1 {
		t.Fatalf("final suffix scan = %#v, %v", result, err)
	}
	var events int
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_events").Scan(&events); err != nil || events != 3 {
		t.Fatalf("events = %d, %v", events, err)
	}
}

func TestConcurrentAppendReadsOnlyBoundedPrefixAndSuffix(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "bounded.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	largePrefix := []byte(strings.Repeat("{}\n", 128*1024))
	base := append(largePrefix, []byte(`{"type":"session_meta","payload":{"session_id":"bounded"}}`+"\n"+`{"type":"turn_context","payload":{"turn_id":"one","model":"gpt-5.4"}}`+"\n"+`{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")...)
	if err = os.WriteFile(path, base, 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	visibleSuffix := []byte(`{"type":"turn_context","payload":{"turn_id":"two","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-17T00:01:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2}}}}` + "\n")
	concurrentSuffix := []byte(`{"type":"turn_context","payload":{"turn_id":"three","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-17T00:02:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":3}}}}` + "\n")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(visibleSuffix); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	inventory, err := service.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var readBytes int64
	service.Open = func(name string) (SourceFile, error) {
		opened, openErr := os.Open(name)
		if openErr != nil {
			return nil, openErr
		}
		return countingSourceFile{File: opened, bytes: &readBytes}, nil
	}
	appended := false
	service.Stat = func(name string) (os.FileInfo, error) {
		if !appended && name == path {
			writer, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if openErr != nil {
				return nil, openErr
			}
			_, writeErr := writer.Write(concurrentSuffix)
			closeErr := writer.Close()
			if writeErr != nil {
				return nil, writeErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			appended = true
		}
		return os.Stat(name)
	}
	result, err := service.ScanInventory(ctx, inventory)
	if err != nil || result["imported"] != 1 {
		t.Fatalf("bounded scan = %#v, %v", result, err)
	}
	maxRead := int64(4*4096 + 3*len(visibleSuffix))
	if readBytes > maxRead {
		t.Fatalf("concurrent append read %d bytes, want <= %d for suffix %d and history %d", readBytes, maxRead, len(visibleSuffix), len(base))
	}
	service.Stat = os.Stat
	service.Open = nil
	result, err = service.Scan(ctx)
	if err != nil || result["imported"] != 1 {
		t.Fatalf("concurrent suffix scan = %#v, %v", result, err)
	}
}

func TestScanRejectsSameLengthMutationWithPreservedMtime(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "preserved-mtime.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte(`{"type":"session_meta","payload":{"session_id":"preserved"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}` + "\n")
	mutated := []byte(strings.Replace(string(contents), `"input_tokens":1`, `"input_tokens":9`, 1))
	if len(mutated) != len(contents) {
		t.Fatal("same-length mutation fixture changed size")
	}
	if err = os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	inventory, err := service.Inventory(ctx)
	if err != nil || len(inventory.Entries) != 1 {
		t.Fatalf("inventory = %#v, %v", inventory, err)
	}
	originalTime := time.Unix(0, inventory.Entries[0].ModifiedAt)
	changed := false
	service.Stat = func(name string) (os.FileInfo, error) {
		if !changed && name == path {
			if writeErr := os.WriteFile(path, mutated, 0o600); writeErr != nil {
				return nil, writeErr
			}
			if timeErr := os.Chtimes(path, originalTime, originalTime); timeErr != nil {
				return nil, timeErr
			}
			changed = true
		}
		return os.Stat(name)
	}
	if _, err = service.ScanInventory(ctx, inventory); !errors.Is(err, errUsageSourceChanged) {
		t.Fatalf("scan error = %v, want %v", err, errUsageSourceChanged)
	}
	var events int
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_events").Scan(&events); err != nil || events != 0 {
		t.Fatalf("events after preserved-mtime mutation = %d, %v", events, err)
	}
}

func TestScanRejectsConcurrentTruncateAndReplacement(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string, []byte) error
	}{
		{name: "truncate", mutate: func(path string, contents []byte) error {
			return os.WriteFile(path, contents[:len(contents)/2], 0o600)
		}},
		{name: "growing rewrite", mutate: func(path string, contents []byte) error {
			return os.WriteFile(path, append([]byte("rewritten\n"), contents...), 0o600)
		}},
		{name: "replace", mutate: func(path string, contents []byte) error {
			replacement := path + ".replacement"
			if err := os.WriteFile(replacement, contents, 0o600); err != nil {
				return err
			}
			return os.Rename(replacement, path)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
			database, err := store.Open(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			path := filepath.Join(home, ".codex", "sessions", "mutation.jsonl")
			if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			contents := []byte(`{"type":"session_meta","payload":{"session_id":"mutation"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}` + "\n")
			if err = os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			service := New(database, home)
			inventory, err := service.Inventory(ctx)
			if err != nil {
				t.Fatal(err)
			}
			mutated := false
			service.Stat = func(name string) (os.FileInfo, error) {
				if !mutated && name == path {
					if mutationErr := test.mutate(path, contents); mutationErr != nil {
						return nil, mutationErr
					}
					mutated = true
				}
				return os.Stat(name)
			}
			if _, err = service.ScanInventory(ctx, inventory); err == nil || !strings.Contains(err.Error(), "usage source changed") {
				t.Fatalf("scan error = %v", err)
			}
			var events int
			if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_events").Scan(&events); err != nil || events != 0 {
				t.Fatalf("events after mutation = %d, %v", events, err)
			}
		})
	}
}

func TestRebuildPreservesFailedSourceAndReturnsPartialWarning(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	makeContents := func(session string, tokens int) []byte {
		return []byte(fmt.Sprintf(`{"type":"session_meta","payload":{"session_id":%q}}`, session) + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d}}}}`, tokens) + "\n")
	}
	paths := map[string]string{
		"good": filepath.Join(home, ".codex", "sessions", "good.jsonl"),
		"bad":  filepath.Join(home, ".codex", "sessions", "bad.jsonl"),
	}
	if err = os.MkdirAll(filepath.Dir(paths["good"]), 0o700); err != nil {
		t.Fatal(err)
	}
	for session, path := range paths {
		if err = os.WriteFile(path, makeContents(session, 1), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var badCursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", paths["bad"]).Scan(&badCursor); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(100,'codex','official','1','2026-07-17T00:00:00Z','2026-07-17T00:01:00Z',1,''); INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(100,?,0,'')`, paths["bad"]); err != nil {
		t.Fatal(err)
	}
	for session, path := range paths {
		if err = os.WriteFile(path, makeContents(session, 2), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_bad_usage_rebuild BEFORE INSERT ON usage_events WHEN NEW.source_path LIKE '%/bad.jsonl' BEGIN SELECT RAISE(FAIL,'injected usage rebuild failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = database.SetSetting(ctx, "watch.fingerprint.usage", "preserved-checkpoint"); err != nil {
		t.Fatal(err)
	}
	result, warnings, err := service.Rebuild(ctx)
	if err != nil || result["source_resets"] != 1 || len(warnings) != 1 || warnings[0] != "usage_source_rebuild_failed" {
		t.Fatalf("rebuild = %#v warnings=%v err=%v", result, warnings, err)
	}
	for session, want := range map[string]int64{"good": 2, "bad": 1} {
		var tokens int64
		if err = database.DB.QueryRowContext(ctx, "SELECT input_tokens FROM usage_events WHERE client='codex' AND session_id=?", session).Scan(&tokens); err != nil || tokens != want {
			t.Fatalf("%s tokens = %d, %v want %d", session, tokens, err, want)
		}
	}
	var cursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", paths["bad"]).Scan(&cursor); err != nil || cursor != badCursor {
		t.Fatalf("bad cursor = %d, %v want %d", cursor, err, badCursor)
	}
	for name, query := range map[string]string{
		"run binding": "SELECT COUNT(*) FROM usage_run_sources WHERE run_id=100",
		"session":     "SELECT COUNT(*) FROM usage_sessions WHERE client='codex' AND session_id='bad'",
	} {
		var count int
		if err = database.DB.QueryRowContext(ctx, query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count = %d, %v", name, count, err)
		}
	}
	checkpoint, found, err := database.Setting(ctx, "watch.fingerprint.usage")
	if err != nil || !found || checkpoint != "preserved-checkpoint" {
		t.Fatalf("checkpoint = %q found=%t err=%v", checkpoint, found, err)
	}
}

func TestRebuildPreservesStableRunBindings(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	path := filepath.Join(home, ".codex", "sessions", "stable.jsonl")
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"session_meta","payload":{"session_id":"stable"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}` + "\n"
	if err = os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var eventKey string
	if err = database.DB.QueryRowContext(ctx, "SELECT event_key FROM usage_events WHERE client='codex' AND session_id='stable'").Scan(&eventKey); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(101,'codex','official','1','2026-07-17T00:00:00Z','2026-07-17T00:01:00Z',1,''); INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(101,?,0,'')`, path); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_run_bindings(event_key,run_id) VALUES(?,101)`, eventKey); err != nil {
		t.Fatal(err)
	}
	result, warnings, err := service.Rebuild(ctx)
	if err != nil || len(warnings) != 0 || result["source_resets"] != 0 {
		t.Fatalf("rebuild = %#v warnings=%v err=%v", result, warnings, err)
	}
	var runSources, eventBindings int
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_run_sources WHERE run_id=101").Scan(&runSources); err != nil || runSources != 1 {
		t.Fatalf("run sources = %d, %v", runSources, err)
	}
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_run_bindings WHERE event_key=? AND run_id=101", eventKey).Scan(&eventBindings); err != nil || eventBindings != 1 {
		t.Fatalf("event bindings = %d, %v", eventBindings, err)
	}
	summary, err := service.Summary(ctx)
	if err != nil || summary.Counts["exact"] != 1 || summary.Counts["estimated"] != 0 || summary.Counts["historical"] != 0 {
		t.Fatalf("summary attribution = %#v, %v", summary.Counts, err)
	}
}

func TestRebuildDuplicateEventFailurePreservesOwningSource(t *testing.T) {
	ctx := context.Background()
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "home")
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	directory := filepath.Join(home, ".codex", "sessions")
	if err = os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{
		"copy":  filepath.Join(directory, "a-copy.jsonl"),
		"owner": filepath.Join(directory, "z-owner.jsonl"),
	}
	contents := func(tokens int) []byte {
		return []byte(`{"type":"session_meta","payload":{"session_id":"duplicate"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-07-17T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d}}}}`, tokens) + "\n")
	}
	if err = os.WriteFile(paths["copy"], contents(9), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(paths["owner"], contents(9), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(database, home)
	if _, err = service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var oldCursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", paths["owner"]).Scan(&oldCursor); err != nil {
		t.Fatal(err)
	}
	var duplicateEventKey string
	if err = database.DB.QueryRowContext(ctx, "SELECT event_key FROM usage_events WHERE source_path=?", paths["owner"]).Scan(&duplicateEventKey); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason) VALUES(102,'codex','official','1','2026-07-17T00:00:00Z','2026-07-17T00:01:00Z',1,''); INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(102,?,0,'')`, paths["owner"]); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_run_bindings(event_key,run_id) VALUES(?,102)`, duplicateEventKey); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_duplicate_owner BEFORE INSERT ON usage_events WHEN NEW.source_path LIKE '%/z-owner.jsonl' BEGIN SELECT RAISE(FAIL,'injected duplicate owner failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = database.SetSetting(ctx, "watch.fingerprint.usage", "duplicate-checkpoint"); err != nil {
		t.Fatal(err)
	}
	_, warnings, err := service.Rebuild(ctx)
	if err != nil || len(warnings) != 1 || warnings[0] != "usage_source_rebuild_failed" {
		t.Fatalf("warnings = %v, err=%v", warnings, err)
	}
	var sourcePath string
	var tokens int64
	if err = database.DB.QueryRowContext(ctx, "SELECT source_path,input_tokens FROM usage_events WHERE event_key=?", duplicateEventKey).Scan(&sourcePath, &tokens); err != nil || sourcePath != paths["owner"] || tokens != 9 {
		t.Fatalf("event owner = %q tokens=%d err=%v", sourcePath, tokens, err)
	}
	var cursor int64
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", paths["owner"]).Scan(&cursor); err != nil || cursor != oldCursor {
		t.Fatalf("owner cursor = %d, %v want %d", cursor, err, oldCursor)
	}
	var runSources, eventBindings, sessions int
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_run_sources WHERE run_id=102").Scan(&runSources); err != nil || runSources != 1 {
		t.Fatalf("run sources = %d, %v", runSources, err)
	}
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_run_bindings WHERE event_key=? AND run_id=102", duplicateEventKey).Scan(&eventBindings); err != nil || eventBindings != 1 {
		t.Fatalf("event bindings = %d, %v", eventBindings, err)
	}
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_sessions WHERE client='codex' AND session_id='duplicate'").Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("sessions = %d, %v", sessions, err)
	}
	checkpoint, found, err := database.Setting(ctx, "watch.fingerprint.usage")
	if err != nil || !found || checkpoint != "duplicate-checkpoint" {
		t.Fatalf("checkpoint = %q found=%t err=%v", checkpoint, found, err)
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
	if err = database.DB.QueryRowContext(ctx, "SELECT input_tokens FROM usage_events WHERE client='codex' AND session_id='reset'").Scan(&tokens); err != nil || tokens != 1 {
		t.Fatalf("event after rollback = %d, %v", tokens, err)
	}
	if err = database.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", path).Scan(&cursor); err != nil || cursor != oldCursor {
		t.Fatalf("source cursor after rollback = %d, %v want %d", cursor, err, oldCursor)
	}
}

func TestHistoricalPriceFallbackFillsOnlyMissingComponentsFromCurrentCatalog(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }
	_, err = database.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('old','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1),
 ('current','litellm','https://raw.githubusercontent.com/BerriAI/litellm/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/model_prices_and_context_window.json','cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc','2026-06-01T00:00:00Z','2026-06-01T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('old','gpt-hybrid','openai','2026-01-01T00:00:00Z','{"input":"2"}','[]'),
 ('current','gpt-hybrid','openai','2026-06-01T00:00:00Z','{"input":"9","cached_input":"4","output":"10"}','[]'),
 ('current','gpt-current-only','openai','2026-06-01T00:00:00Z','{"input":"7","cached_input":"3","output":"8"}','[]');
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','hybrid','2026-02-01T00:00:00Z','2026-02-01T00:00:00Z'),
 ('codex','current','2026-02-01T00:01:00Z','2026-02-01T00:01:00Z'),
 ('codex','unknown','2026-02-01T00:02:00Z','2026-02-01T00:02:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES
 ('hybrid','codex','hybrid','hybrid','2026-02-01T00:00:00Z','gpt-hybrid',1000000,1000000,'fixture',0),
 ('current','codex','current','current','2026-02-01T00:01:00Z','gpt-current-only',1000000,1000000,'fixture',1),
 ('unknown','codex','unknown','unknown','2026-02-01T00:02:00Z','codex-auto-review',1000000,0,'fixture',2)`)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CatalogBaseCost != nil || summary.KnownCatalogBaseCost == nil || *summary.KnownCatalogBaseCost != "27.000000000" {
		t.Fatalf("historical fallback total = %#v", summary)
	}
	if summary.Counts["priced"] != 2 || summary.Counts["unpriced"] != 1 {
		t.Fatalf("historical fallback coverage = %#v", summary.Counts)
	}
	prices, err := service.PriceList(ctx, "openai", "gpt-hybrid")
	if err != nil || len(prices) != 1 || prices[0].Prices["input"] != "9" || prices[0].Prices["output"] != "10" {
		t.Fatalf("current merged prices = %#v, %v", prices, err)
	}
}

func TestPriceStatusUsesOnlyCatalogsEffectiveAtCurrentTime(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name          string
		includeActive bool
		wantAvailable bool
		wantVersion   string
	}{
		{name: "future only", wantAvailable: false},
		{name: "active and future", includeActive: true, wantAvailable: true, wantVersion: "active"},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			if _, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
('future','official','https://example.invalid/future','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-07-19T18:00:00-07:00','2026-07-19T18:00:00-07:00','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
('future','gpt-future','openai','2026-07-19T18:00:00-07:00','{"input":"9","cached_input":"9","output":"9"}','[]')`); err != nil {
				t.Fatal(err)
			}
			if test.includeActive {
				if _, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
('active','bundled','bundled://agentdeck/model-prices.json','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','2026-07-20T08:00:00+08:00','2026-07-20T08:00:00+08:00','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
('active','gpt-active','openai','2026-07-20T08:00:00+08:00','{"input":"1","cached_input":"0.5","output":"2"}','[]')`); err != nil {
					t.Fatal(err)
				}
			}
			service := New(database, "")
			service.Now = func() time.Time { return time.Date(2026, 7, 20, 0, 30, 0, 0, time.UTC) }
			status, err := service.PriceStatus(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if available, _ := status["available"].(bool); available != test.wantAvailable {
				t.Fatalf("available = %t, status=%#v", available, status)
			}
			if test.wantAvailable {
				if status["version"] != test.wantVersion || status["models"] != 1 || status["components"] != 3 {
					t.Fatalf("current price status = %#v", status)
				}
				catalogs, ok := status["catalogs"].([]PriceCatalog)
				if !ok || len(catalogs) != 1 || catalogs[0].Version != test.wantVersion {
					t.Fatalf("active catalogs = %#v", status["catalogs"])
				}
			}
		})
	}
}

func TestPriceStatusSamplesAdvancingClockOnce(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
('active','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-07-20T00:00:00Z','2026-07-20T00:00:00Z','USD',1),
('boundary','official','https://example.invalid/boundary','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','2026-07-20T00:30:00Z','2026-07-20T00:30:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
('active','gpt-active','openai','2026-07-20T00:00:00Z','{"input":"1","cached_input":"0.5","output":"2"}','[]'),
('boundary','gpt-boundary','openai','2026-07-20T00:30:00Z','{"input":"9","cached_input":"9","output":"9"}','[]')`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	clock := []time.Time{
		time.Date(2026, 7, 20, 0, 29, 59, 0, time.UTC),
		time.Date(2026, 7, 20, 0, 30, 1, 0, time.UTC),
	}
	var calls int
	service.Now = func() time.Time {
		value := clock[min(calls, len(clock)-1)]
		calls++
		return value
	}
	status, err := service.PriceStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("PriceStatus clock calls = %d, want 1", calls)
	}
	catalogs, ok := status["catalogs"].([]PriceCatalog)
	if !ok || len(catalogs) != 1 || catalogs[0].Version != "active" {
		t.Fatalf("PriceStatus catalogs = %#v", status["catalogs"])
	}
	if status["version"] != "active" || status["models"] != 1 || status["components"] != 3 {
		t.Fatalf("PriceStatus mixed effective instants = %#v", status)
	}
}

func TestStatsAggregatesOneIndexedRangeIntoBalancedReport(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('fixture','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('fixture','gpt-a','openai','2026-01-01T00:00:00Z','{"input":"1","cached_input":"0.5","output":"2"}','[]'),
 ('fixture','gpt-b','openai','2026-01-01T00:00:00Z','{"input":"1","cached_input":"0.5","output":"2"}','[]');
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','a','2026-06-30T16:30:00Z','2026-07-01T01:00:00Z'),
 ('codex','b','2026-07-03T04:00:00Z','2026-07-03T04:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset) VALUES
 ('a1','codex','a','a1','2026-06-30T16:30:00Z','gpt-a',100,25,50,'fixture',0),
 ('a2','codex','a','a2','2026-07-01T01:00:00Z','gpt-a',200,50,100,'fixture',1),
 ('b1','codex','b','b1','2026-07-03T04:00:00Z','gpt-b',300,75,0,'fixture',2)`)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	countedStore, queries := openQueryCountingStore(t, filepath.Join(state, "agentdeck.sqlite3"))
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	to := time.Date(2026, 7, 8, 0, 0, 0, 0, location)
	service := New(countedStore, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }
	report, err := service.Stats(ctx, StatsOptions{From: from, To: to, GroupBy: "day", Metric: "tokens", Location: location, Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.Tokens != 750 || report.Totals.Sessions != 2 || report.Totals.Events != 3 || len(report.Buckets) != 7 || len(report.Activity) != 168 {
		t.Fatalf("stats totals = %#v buckets=%d activity=%d", report.Totals, len(report.Buckets), len(report.Activity))
	}
	if report.Totals.InputTokens != 600 || report.Totals.OutputTokens != 150 || report.Totals.CachedReadTokens != 150 || report.Totals.CacheWriteTokens != 0 || report.Clients[0].CacheReadRate == nil || *report.Clients[0].CacheReadRate != "25.00" {
		t.Fatalf("stats token components = totals:%#v clients:%#v", report.Totals, report.Clients)
	}
	if report.Peak.Start != "2026-07-01T00:00:00+08:00" || report.Peak.Value == nil || *report.Peak.Value != "450" || len(report.Models) != 2 || report.Models[0].Name != "gpt-a" || report.Coverage.Percent != "100.00" {
		t.Fatalf("stats report = %#v", report)
	}
	if got := queries.Load(); got != 4 {
		t.Fatalf("stats SQL queries for 3 events = %d, want 4", got)
	}
	queries.Store(0)
	tx, err := countedStore.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	for index := 3; index < 1003; index++ {
		at := from.Add(time.Duration(index%168) * time.Hour).UTC().Format(time.RFC3339Nano)
		key := fmt.Sprintf("bulk-%04d", index)
		if _, err = statement.ExecContext(ctx, key, "codex", "b", key, at, "gpt-b", 1, 0, "fixture", index); err != nil {
			statement.Close()
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err = statement.Close(); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	queries.Store(0)
	large, err := service.Stats(ctx, StatsOptions{From: from, To: to, GroupBy: "day", Metric: "tokens", Location: location, Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatal(err)
	}
	if large.Totals.Events != 1003 {
		t.Fatalf("large stats events = %d, want 1003", large.Totals.Events)
	}
	if got := queries.Load(); got != 4 {
		t.Fatalf("stats SQL queries for 1003 events = %d, want 4", got)
	}
	queries.Store(0)
	today, err := service.Stats(ctx, StatsOptions{From: from, To: from.Add(24 * time.Hour), GroupBy: "hour", Metric: "cost", Location: location, Timezone: "Asia/Shanghai"})
	if err != nil || len(today.Activity) != 0 || len(today.Buckets) != 24 {
		t.Fatalf("today stats = buckets:%d activity:%d err:%v", len(today.Buckets), len(today.Activity), err)
	}
	var indexed int
	rows, err := countedStore.DB.QueryContext(ctx, `PRAGMA index_list('usage_events')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var sequence, unique, partial int
		var name, origin string
		if err = rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if name == "usage_events_event_at" {
			indexed++
		}
	}
	if err = rows.Close(); err != nil || indexed != 1 {
		t.Fatalf("usage event time index = %d, %v", indexed, err)
	}
}

func TestStatsSeparatesClientCacheRatesAndUnpricedModels(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('fixture','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('fixture','gpt-priced','openai','2026-01-01T00:00:00Z','{"input":"1","cached_input":"0.5","output":"2"}','[]'),
 ('fixture','claude-priced','anthropic','2026-01-01T00:00:00Z','{"input":"1","output":"2","cache_read":"0.1","cache_write_5m":"1.25","cache_write_1h":"2"}','[]');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_write_5m_tokens,source_path,source_offset) VALUES
 ('codex','codex','c','c','2026-07-20T01:00:00Z','gpt-priced',100,40,10,0,0,'fixture',0),
 ('claude','claude','a','a','2026-07-20T02:00:00Z','claude-priced',20,0,5,60,20,'fixture',1),
 ('unknown','codex','u','u','2026-07-20T03:00:00Z','zzz-unknown',3,0,0,0,0,'fixture',2)`)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	report, err := New(database, "").Stats(ctx, StatsOptions{From: from, To: from.AddDate(0, 0, 1), GroupBy: "day", Metric: "tokens", Location: time.UTC, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.InputTokens != 123 || report.Totals.OutputTokens != 15 || report.Totals.CachedReadTokens != 100 || report.Totals.CacheWriteTokens != 20 {
		t.Fatalf("totals components = %#v", report.Totals)
	}
	if len(report.Buckets) != 1 || report.Buckets[0].CachedReadTokens != 100 || report.Buckets[0].CacheWriteTokens != 20 {
		t.Fatalf("bucket components = %#v", report.Buckets)
	}
	clients := map[string]StatsDimension{}
	for _, client := range report.Clients {
		clients[client.Name] = client
	}
	if clients["codex"].CacheReadRate == nil || *clients["codex"].CacheReadRate != "38.83" || clients["codex"].CacheWriteRate != nil {
		t.Fatalf("Codex cache analysis = %#v", clients["codex"])
	}
	if clients["claude"].LogicalInputTokens != 100 || clients["claude"].CacheReadRate == nil || *clients["claude"].CacheReadRate != "60.00" || clients["claude"].CacheWriteRate == nil || *clients["claude"].CacheWriteRate != "20.00" {
		t.Fatalf("Claude cache analysis = %#v", clients["claude"])
	}
	if len(report.UnpricedModels) != 1 || report.UnpricedModels[0].Client != "codex" || report.UnpricedModels[0].Model != "zzz-unknown" || !reflect.DeepEqual(report.UnpricedModels[0].Components, []string{"unknown_model"}) {
		t.Fatalf("unpriced models = %#v", report.UnpricedModels)
	}
	if report.Totals.ProviderCost != nil || report.Totals.KnownProviderCost == "0.000000000" {
		t.Fatalf("partial cost state = %#v", report.Totals)
	}
}

func TestRangeQueriesUseAbsoluteTimeForOffsetEvents(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{
		{Key: "positive", Client: "codex", SessionID: "offsets", EventID: "positive", EventAt: "2026-07-01T01:00:00+08:00", Model: "missing", Tokens: map[string]int64{"input_tokens": 10}, SourcePath: "fixture", SourceOffset: 1},
		{Key: "negative", Client: "codex", SessionID: "offsets", EventID: "negative", EventAt: "2026-06-30T20:00:00-05:00", Model: "missing", Tokens: map[string]int64{"input_tokens": 20}, SourcePath: "fixture", SourceOffset: 2},
	} {
		if _, _, err = upsertTx(ctx, tx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err = rebuildSessions(ctx, tx, map[string][2]string{"offsets": {"codex", "offsets"}}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	summary, err := service.SummaryRange(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Counts["events"] != 1 || summary.Tokens["input_tokens"] != 20 {
		t.Fatalf("offset range summary = %#v", summary)
	}
	report, err := service.Stats(ctx, StatsOptions{From: from, To: to, GroupBy: "hour", Metric: "tokens", Location: time.UTC, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.Events != 1 || report.Totals.Tokens != 20 {
		t.Fatalf("offset range stats = %#v", report.Totals)
	}
	earliest, err := service.EarliestEventAt(ctx)
	if err != nil || earliest == nil || !earliest.Equal(time.Date(2026, 6, 30, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("earliest offset event = %v, %v", earliest, err)
	}
	var first, last string
	if err = database.DB.QueryRowContext(ctx, `SELECT first_at,last_at FROM usage_sessions WHERE client='codex' AND session_id='offsets'`).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	if first != "2026-06-30T17:00:00Z" || last != "2026-07-01T01:00:00Z" {
		t.Fatalf("canonical session range = %q to %q", first, last)
	}
}

func TestStatsKeepsDSTFoldHoursDistinct(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES
('first','codex','fold','first','2026-11-01T05:30:00Z','missing',10,'fixture',1),
('second','codex','fold','second','2026-11-01T06:30:00Z','missing',20,'fixture',2);
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','fold','2026-11-01T05:30:00Z','2026-11-01T06:30:00Z')`); err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, location)
	to := from.AddDate(0, 0, 1)
	report, err := New(database, "").Stats(ctx, StatsOptions{From: from, To: to, GroupBy: "hour", Metric: "tokens", Location: location, Timezone: "America/New_York"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Buckets) != 25 {
		t.Fatalf("DST fold buckets = %d, want 25", len(report.Buckets))
	}
	var fold []StatsBucket
	for _, bucket := range report.Buckets {
		if strings.HasPrefix(bucket.Start, "2026-11-01T01:00:00") {
			fold = append(fold, bucket)
		}
	}
	if len(fold) != 2 || fold[0].Tokens != 10 || fold[1].Tokens != 20 || fold[0].Start == fold[1].Start {
		t.Fatalf("DST fold hours = %#v", fold)
	}
}

func TestStatsPartialCostJSONSeparatesCompleteAndKnownValues(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES
('priced','codex','priced','priced','2026-07-13T01:00:00Z','gpt-5.4',1000000,'fixture',1),
('unpriced','codex','unpriced','unpriced','2026-07-13T02:00:00Z','codex-auto-review',1000000,'fixture',2);
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
('codex','priced','2026-07-13T01:00:00Z','2026-07-13T01:00:00Z'),
('codex','unpriced','2026-07-13T02:00:00Z','2026-07-13T02:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	report, err := service.Stats(ctx, StatsOptions{From: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), GroupBy: "day", Metric: "cost", Location: time.UTC, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Totals struct {
			AverageCost      *string `json:"average_cost_per_session"`
			KnownAverageCost string  `json:"known_average_cost_per_session"`
		} `json:"totals"`
		Buckets []struct {
			MetricValue      *string `json:"metric_value"`
			KnownMetricValue string  `json:"known_metric_value"`
		} `json:"buckets"`
		Peak struct {
			Value      *string `json:"value"`
			KnownValue string  `json:"known_value"`
		} `json:"peak"`
		Models []struct {
			Name       string  `json:"name"`
			Share      *string `json:"share"`
			KnownShare string  `json:"known_share"`
		} `json:"models"`
		Clients []struct {
			Share      *string `json:"share"`
			KnownShare string  `json:"known_share"`
		} `json:"clients"`
	}
	if err = json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Totals.AverageCost != nil || payload.Totals.KnownAverageCost == "" {
		t.Fatalf("partial average cost JSON = %s", encoded)
	}
	if len(payload.Buckets) != 1 || payload.Buckets[0].MetricValue != nil || payload.Buckets[0].KnownMetricValue == "" {
		t.Fatalf("partial bucket cost JSON = %s", encoded)
	}
	if payload.Peak.Value != nil || payload.Peak.KnownValue == "" {
		t.Fatalf("partial peak cost JSON = %s", encoded)
	}
	if len(payload.Models) != 2 || payload.Models[0].Share != nil || payload.Models[0].KnownShare == "" || payload.Models[1].Share != nil || payload.Models[1].KnownShare == "" {
		t.Fatalf("partial model shares JSON = %s", encoded)
	}
	if len(payload.Clients) != 1 || payload.Clients[0].Share != nil || payload.Clients[0].KnownShare == "" {
		t.Fatalf("partial client share JSON = %s", encoded)
	}
}
