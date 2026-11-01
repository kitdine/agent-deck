package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

const secretPrompt = "SECRET-PROMPT-TEXT"

type attr struct {
	key, kind, value string
}

func s(key, value string) attr { return attr{key, "stringValue", fmt.Sprintf("%q", value)} }
func i(key, value string) attr { return attr{key, "intValue", value} }

func exportBody(resource []attr, records ...[]attr) string {
	encode := func(list []attr) string {
		parts := make([]string, 0, len(list))
		for _, a := range list {
			parts = append(parts, fmt.Sprintf(`{"key":%q,"value":{%q:%s}}`, a.key, a.kind, a.value))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	logs := make([]string, 0, len(records))
	for _, record := range records {
		logs = append(logs, fmt.Sprintf(`{"timeUnixNano":"%d","eventName":"event otel/src/events/session_telemetry.rs:1132","body":{"stringValue":%q},"attributes":%s}`, 1793272990000000000+int64(len(logs)), secretPrompt, encode(record)))
	}
	return fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":%s},"scopeLogs":[{"logRecords":[%s]}]}]}`, encode(resource), strings.Join(logs, ","))
}

// codexResource and completed reproduce the codex_exec 0.165.0 shape seen in
// the isolated producer experiment: no service.instance.id, no response id on
// usage events, and an OTLP eventName that is only a tracing target.
var codexResource = []attr{s("service.name", "codex_exec"), s("service.version", "0.165.0")}

func completed(conversation string, input, output string) []attr {
	return []attr{
		s("event.name", "codex.sse_event"), s("event.kind", "response.completed"), s("model", "gpt-5.6-terra"),
		s("originator", "codex_exec"), s("conversation.id", conversation),
		i("input_token_count", input), i("output_token_count", output), i("cached_token_count", `"0"`),
		s("prompt", secretPrompt), s("user.email", "person@example.com"), s("authorization", "Bearer sk-secret"),
	}
}

func TestDecodeKeepsOnlyAllowlistedFieldsAndExactIntegers(t *testing.T) {
	records, err := Decode([]byte(exportBody(codexResource, completed("thread-1", `"9007199254740993"`, "12"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	r := records[0]
	if r.Producer != "codex_exec" || r.ProducerVersion != "0.165.0" || r.Originator != "codex_exec" || r.Transport != "sse" ||
		r.EventName != "codex.sse_event" || r.ThreadID != "thread-1" || r.Model != "gpt-5.6-terra" || r.RecordAt == "" {
		t.Fatalf("identity fields not kept: %+v", r)
	}
	if r.InputTokens == nil || *r.InputTokens != 9007199254740993 || *r.OutputTokens != 12 || *r.CachedTokens != 0 {
		t.Fatalf("token counts lost precision: %+v", r)
	}
	if r.ReasoningTokens != nil || r.CacheWriteTokens != nil || r.TotalTokens != nil {
		t.Fatal("absent counts must stay nil, not zero")
	}
	if !r.Metered() || r.Invalid != "" {
		t.Fatalf("completed response not metered: %+v", r)
	}
	line := r.Key() + fmt.Sprintf("%+v", r)
	for _, forbidden := range []string{secretPrompt, "person@example.com", "sk-secret"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("record retained %q", forbidden)
		}
	}
}

func TestDecodeClassifiesInvalidCountsWithoutMetering(t *testing.T) {
	for _, test := range []struct {
		name   string
		extra  []attr
		input  string
		output string
		want   string
	}{
		{"negative", nil, "-1", "1", "count_negative"},
		{"overflow", nil, `"9223372036854775808"`, "1", "count_overflow"},
		{"fraction", nil, "1", "1", ""},
		{"cached exceeds input", []attr{i("cached_token_count", "5")}, "4", "1", "cached_exceeds_input"},
		{"reasoning exceeds output", []attr{i("reasoning_token_count", "3")}, "4", "2", "reasoning_exceeds_output"},
		{"total disagrees", []attr{i("total_token_count", "7")}, "4", "2", "total_disagrees_with_components"},
		{"total agrees", []attr{i("total_token_count", "6")}, "4", "2", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			fields := []attr{s("event.name", "codex.sse_event"), s("event.kind", "response.completed"), i("input_token_count", test.input), i("output_token_count", test.output)}
			fields = append(fields, test.extra...)
			records, err := Decode([]byte(exportBody(codexResource, fields)))
			if err != nil {
				t.Fatal(err)
			}
			if records[0].Invalid != test.want {
				t.Fatalf("invalid = %q, want %q", records[0].Invalid, test.want)
			}
			observation := Observation(records[0], "receiver", time.Now())
			if test.want != "" && (observation.State != "invalid" || observation.Metered) {
				t.Fatalf("invalid record metered: %+v", observation)
			}
		})
	}
	records, err := Decode([]byte(`{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"input_token_count","value":{"doubleValue":1.5}}]}]}]}]}`))
	if err != nil || records[0].Invalid != "count_not_integer" {
		t.Fatalf("fractional double accepted: %+v %v", records, err)
	}
}

func TestDecodeSkipsNonEventsAndRejectsMalformedOrOversizedBodies(t *testing.T) {
	records, err := Decode([]byte(exportBody(codexResource, []attr{s("model", "m")})))
	if err != nil || len(records) != 0 {
		t.Fatalf("record without event name kept: %+v %v", records, err)
	}
	delta, _ := Decode([]byte(exportBody(codexResource, []attr{s("event.name", "codex.sse_event"), s("event.kind", "response.output_text.delta")})))
	if delta[0].Metered() {
		t.Fatal("stream delta treated as a call")
	}
	failed, _ := Decode([]byte(exportBody(codexResource, []attr{s("event.name", "codex.sse_event"), s("event.kind", "response.failed")})))
	if failed[0].Metered() {
		t.Fatal("failed response metered without usage")
	}
	for _, body := range []string{`{`, `{"resourceLogs":[]} {}`, `[]`} {
		if _, err := Decode([]byte(body)); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Decode(%q) = %v", body, err)
		}
	}
	if _, err := Decode(bytes.Repeat([]byte(" "), MaxBodyBytes+1)); !errors.Is(err, ErrMalformed) {
		t.Fatal("oversized body accepted")
	}
	long := strings.Repeat("x", maxFieldBytes+1)
	overlong, _ := Decode([]byte(exportBody(codexResource, append(completed("r", "1", "1"), s("model", long)))))
	if overlong[0].Invalid != "field_too_long" || strings.Contains(fmt.Sprint(overlong[0]), long) {
		t.Fatalf("overlong field kept: %+v", overlong[0].Invalid)
	}
}

type cursorHolder struct{ cursor store.TelemetryCursor }

func newJournal(t *testing.T, segmentBytes, cap int64, processed *cursorHolder, now time.Time) *Journal {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "journal")
	if err := os.MkdirAll(dir, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	return &Journal{Dir: dir, SegmentBytes: segmentBytes, Cap: cap, Retention: 7 * 24 * time.Hour,
		Processed: func() (store.TelemetryCursor, error) { return processed.cursor, nil },
		Now:       func() time.Time { return now }}
}

func sampleRecords(t *testing.T, n int) []Record {
	t.Helper()
	var all []Record
	for k := 0; k < n; k++ {
		records, err := Decode([]byte(exportBody(codexResource, completed(fmt.Sprintf("resp-%d", k), "10", "2"))))
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, records...)
	}
	return all
}

func TestJournalRotatesReadsCompleteLinesAndKeepsUnprocessedData(t *testing.T) {
	holder := &cursorHolder{}
	journal := newJournal(t, 600, 1<<20, holder, time.Now())
	records := sampleRecords(t, 6)
	for _, record := range records {
		if err := journal.Append([]Record{record}); err != nil {
			t.Fatal(err)
		}
	}
	segments, _ := journal.segments()
	if len(segments) < 2 {
		t.Fatalf("segments did not rotate: %d", len(segments))
	}
	for _, segment := range segments {
		info, _ := os.Stat(segment.path)
		if info.Mode().Perm() != platform.FileMode {
			t.Fatalf("segment mode %v", info.Mode().Perm())
		}
	}
	// A torn final write is not consumed until it is complete.
	last := segments[len(segments)-1].path
	file, _ := os.OpenFile(last, os.O_WRONLY|os.O_APPEND, 0)
	_, _ = file.WriteString(`{"producer":"torn`)
	file.Close()
	read, next, err := journal.ReadFrom(store.TelemetryCursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != len(records) {
		t.Fatalf("read %d records, want %d", len(read), len(records))
	}
	for k := range records {
		if read[k].ResponseID != records[k].ResponseID {
			t.Fatal("journal order changed")
		}
	}
	again, same, _ := journal.ReadFrom(next, 100)
	if len(again) != 0 || same != next {
		t.Fatal("torn line consumed")
	}
	partial, mid, _ := journal.ReadFrom(store.TelemetryCursor{}, 2)
	rest, _, _ := journal.ReadFrom(mid, 100)
	if len(partial) != 2 || len(rest) != len(records)-2 {
		t.Fatalf("limited reads lost records: %d + %d", len(partial), len(rest))
	}
}

func TestJournalAppendAfterTornTailKeepsAcknowledgedRecords(t *testing.T) {
	for _, tc := range []struct {
		name         string
		segmentBytes int64
	}{
		{name: "same segment", segmentBytes: 1 << 20},
		// The torn segment is already full, so the next batch rotates.
		{name: "rotation", segmentBytes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			holder := &cursorHolder{}
			journal := newJournal(t, tc.segmentBytes, 1<<20, holder, time.Now())
			records := sampleRecords(t, 2)
			if err := journal.Append(records[:1]); err != nil {
				t.Fatal(err)
			}
			segments, _ := journal.segments()
			file, _ := os.OpenFile(segments[0].path, os.O_WRONLY|os.O_APPEND, 0)
			_, _ = file.WriteString(`{"producer":"torn`)
			file.Close()
			first, cursor, err := journal.ReadFrom(store.TelemetryCursor{}, 100)
			if err != nil || len(first) != 1 {
				t.Fatalf("read before restart: %d records, %v", len(first), err)
			}
			// After a restart the receiver appends and acknowledges the next delivery.
			if err := journal.Append(records[1:]); err != nil {
				t.Fatal(err)
			}
			rest, _, err := journal.ReadFrom(cursor, 100)
			if err != nil {
				t.Fatal(err)
			}
			if len(rest) != 2 || rest[0].Invalid != "journal_line_unreadable" || rest[1].ResponseID != records[1].ResponseID || rest[1].Invalid != "" {
				t.Fatalf("acknowledged record not replayed after torn tail: %+v", rest)
			}
			if after, _ := journal.segments(); tc.segmentBytes == 1 && len(after) != 2 {
				t.Fatalf("batch did not rotate: %d segments", len(after))
			}
		})
	}
}

func TestJournalCapNeverDeletesUnprocessedSegments(t *testing.T) {
	holder := &cursorHolder{}
	journal := newJournal(t, 400, 1200, holder, time.Now())
	var err error
	appended := 0
	for k, record := range sampleRecords(t, 20) {
		if err = journal.Append([]Record{record}); err != nil {
			appended = k
			break
		}
	}
	if !errors.Is(err, ErrJournalFull) {
		t.Fatalf("cap not enforced: %v", err)
	}
	read, next, _ := journal.ReadFrom(store.TelemetryCursor{}, 100)
	if len(read) != appended {
		t.Fatalf("unprocessed data lost under cap: read %d of %d", len(read), appended)
	}
	// Once everything before the last segment is processed, the cap frees it.
	segments, _ := journal.segments()
	holder.cursor = store.TelemetryCursor{Segment: segments[len(segments)-1].index}
	if err := journal.Append(sampleRecords(t, 1)); err != nil {
		t.Fatalf("processed segments were not reclaimed: %v", err)
	}
	if _, err := os.Stat(segments[0].path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("processed segment kept beyond cap")
	}
	if _, err := os.Stat(segments[len(segments)-1].path); err != nil {
		t.Fatal("unprocessed segment removed")
	}
	_ = next
}

func TestJournalRetentionRemovesOnlyOldProcessedSegments(t *testing.T) {
	holder := &cursorHolder{}
	now := time.Now()
	journal := newJournal(t, 400, 1<<20, holder, now.Add(8*24*time.Hour))
	for _, record := range sampleRecords(t, 6) {
		if err := journal.Append([]Record{record}); err != nil {
			t.Fatal(err)
		}
	}
	segments, _ := journal.segments()
	holder.cursor = store.TelemetryCursor{Segment: segments[1].index}
	if err := journal.Cleanup(); err != nil {
		t.Fatal(err)
	}
	remaining, _ := journal.segments()
	if len(remaining) != len(segments)-1 || remaining[0].index != segments[1].index {
		t.Fatalf("retention removed the wrong segments: %d -> %d", len(segments), len(remaining))
	}
	journal.Now = func() time.Time { return now }
	holder.cursor = store.TelemetryCursor{Segment: 1 << 40}
	_ = journal.Cleanup()
	if kept, _ := journal.segments(); len(kept) != len(remaining) {
		t.Fatal("young processed segments removed before retention")
	}
}

func openStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database, root
}

func TestReceiverAcknowledgesOnlyAfterDurableAppend(t *testing.T) {
	holder := &cursorHolder{}
	journal := newJournal(t, DefaultSegmentBytes, DefaultJournalCap, holder, time.Now())
	token := strings.Repeat("a", 64)
	persisted := make(chan struct{}, 1)
	var refused []string
	receiver := &Receiver{Token: token, Journal: journal, Persisted: persisted, Now: time.Now,
		Refused: func(_ time.Time, reason string) { refused = append(refused, reason) }}
	body := exportBody(codexResource, completed("thread-1", "10", "2"))
	post := func(path, contentType, token, payload string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		request.Header.Set("Content-Type", contentType)
		if token != "" {
			request.Header.Set(TokenHeader, token)
		}
		response := httptest.NewRecorder()
		receiver.ServeHTTP(response, request)
		return response
	}
	for _, test := range []struct {
		name, path, contentType, token, body string
		want                                 int
	}{
		{"no token", LogsPath, "application/json", "", body, 401},
		{"wrong token", LogsPath, "application/json", strings.Repeat("b", 64), body, 401},
		{"protobuf", LogsPath, "application/x-protobuf", token, body, 415},
		{"malformed", LogsPath, "application/json", token, "{", 400},
		{"too large", LogsPath, "application/json", token, strings.Repeat(" ", MaxBodyBytes+1), 413},
		{"wrong path", "/v1/traces", "application/json", token, body, 404},
	} {
		if got := post(test.path, test.contentType, test.token, test.body); got.Code != test.want {
			t.Fatalf("%s: status %d, want %d", test.name, got.Code, test.want)
		} else if strings.Contains(got.Body.String(), secretPrompt) {
			t.Fatalf("%s: response echoed request content", test.name)
		}
	}
	if records, _, _ := journal.ReadFrom(store.TelemetryCursor{}, 10); len(records) != 0 {
		t.Fatal("rejected request reached the journal")
	}
	if got := post(LogsPath, "application/json; charset=utf-8", token, body); got.Code != 200 {
		t.Fatalf("valid request status %d", got.Code)
	}
	if records, _, _ := journal.ReadFrom(store.TelemetryCursor{}, 10); len(records) != 1 {
		t.Fatal("200 returned before the record was journaled")
	}
	select {
	case <-persisted:
	default:
		t.Fatal("reconciler not signalled")
	}
	journal.Cap = 1
	if got := post(LogsPath, "application/json", token, body); got.Code != 503 {
		t.Fatalf("unpersistable request status %d", got.Code)
	}
	if len(refused) != 1 || refused[0] != "journal_full" {
		t.Fatalf("refusal not recorded: %v", refused)
	}
}

func TestServeRecordsCoverageStopsOnDisableAndKeepsPrivacy(t *testing.T) {
	database, root := openStore(t)
	home := t.TempDir()
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ServeOptions{StateRoot: root, Home: home, Owner: "app", Store: database, Ready: func(addr string) { ready <- addr }})
	}()
	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("listener not ready")
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("listener bound to %s", addr)
	}
	token, _ := ReadToken(root)
	request, _ := http.NewRequest(http.MethodPost, "http://"+addr+LogsPath, strings.NewReader(exportBody(codexResource, completed("thread-9", "10", "2"))))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(TokenHeader, token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("status %d", response.StatusCode)
	}
	waitFor(t, func() bool {
		summary, _ := database.TelemetrySummary(context.Background())
		return summary.Pending == 1
	})
	status, err := ReadStatus(context.Background(), root, home, func() (*store.Store, error) { return store.OpenReadOnly(context.Background(), root) })
	if err != nil {
		t.Fatal(err)
	}
	if status.Receiver != "enabled" || status.Listener != "running" || status.EngineMode != "app" || status.Metering != "reconciled" || status.ProducerConfigured != "unknown" ||
		status.CodexMemories != "unknown" || status.Reception != "received" ||
		status.HistoricalCoverage != "unknown" || status.LastReceivedAt == nil || len(status.Coverage) != 1 || status.Coverage[0].EndedAt != nil {
		t.Fatalf("status = %+v", status)
	}
	// Disabling through AgentDeck's own configuration stops the listener.
	time.Sleep(20 * time.Millisecond)
	if err := SaveConfig(root, Config{Enabled: false, Port: status.Port, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("listener kept running after disable")
	}
	summary, _ := database.TelemetrySummary(context.Background())
	if summary.Pending != 1 || len(summary.Coverage) != 1 || summary.Coverage[0].EndedAt == "" || summary.Coverage[0].Reason != "disabled" {
		t.Fatalf("disable lost data or coverage: %+v", summary)
	}
	if listener, _ := ReadListenerState(root); listener.State != "stopped" {
		t.Fatalf("listener state %s after exit", listener.State)
	}
	// Nothing outside the allowlist, and never the token, is on disk.
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Base(path) == "token" {
			return err
		}
		data, _ := os.ReadFile(path)
		for _, forbidden := range []string{secretPrompt, "person@example.com", "sk-secret", token} {
			if bytes.Contains(data, []byte(forbidden)) {
				t.Fatalf("%s contains forbidden content", filepath.Base(path))
			}
		}
		return nil
	})
}

func TestServeRefusesWhenDisabledAndReportsBindFailure(t *testing.T) {
	database, root := openStore(t)
	if err := Serve(context.Background(), ServeOptions{StateRoot: root, Owner: "foreground", Store: database}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled receiver served: %v", err)
	}
	occupied := httptest.NewServer(http.NotFoundHandler())
	defer occupied.Close()
	port := occupied.Listener.Addr().(*net.TCPAddr).Port
	if err := SaveConfig(root, Config{Enabled: true, Port: port, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	if err := Serve(context.Background(), ServeOptions{StateRoot: root, Owner: "app", Store: database}); err == nil {
		t.Fatal("bind conflict not reported")
	}
	listener, _ := ReadListenerState(root)
	if listener.State != "failed" || listener.Error != "bind_failed" {
		t.Fatalf("listener state %+v", listener)
	}
}

func TestTokenAndConfigurationArePrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	config, err := LoadConfig(root)
	if err != nil || config.Enabled || config.Port != DefaultPort || config.RetentionDays != DefaultRetentionDays {
		t.Fatalf("default config %+v %v", config, err)
	}
	token, err := EnsureToken(root)
	if err != nil || len(token) != 64 {
		t.Fatalf("token %q %v", token, err)
	}
	again, _ := EnsureToken(root)
	if again != token {
		t.Fatal("token rotated on reuse")
	}
	if err := SaveConfig(root, Config{Enabled: true, Port: 4319, RetentionDays: 3}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{tokenPath(root), configPath(root)} {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != platform.FileMode {
			t.Fatalf("%s mode %v", filepath.Base(path), info.Mode().Perm())
		}
	}
	if err := SaveConfig(root, Config{Port: 0, RetentionDays: 7}); err == nil {
		t.Fatal("invalid port saved")
	}
	_ = os.Chmod(tokenPath(root), 0o644)
	if _, err := ReadToken(root); err == nil {
		t.Fatal("world-readable token accepted")
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
