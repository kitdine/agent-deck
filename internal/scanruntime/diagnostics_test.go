package scanruntime

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"

	"github.com/kitdine/agent-deck/internal/store"
)

var diagnosticTestClock = time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC)

func readDiagnosticEntries(t *testing.T, path string) []diagnosticEntry {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var entries []diagnosticEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		var entry diagnosticEntry
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("diagnostic line %q: %v", scanner.Text(), err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return entries
}

// encodeDiagnosticEntry is the exact line the log must hold for want.
func encodeDiagnosticEntry(t *testing.T, want diagnosticEntry) string {
	t.Helper()
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(want); err != nil {
		t.Fatal(err)
	}
	return line.String()
}

func preparedDiagnosticRoot(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
	if err != nil {
		t.Fatal(err)
	}
	return home, root
}

// raiseSQLiteError returns a real driver error whose text is message, the way
// a trigger or SQLite itself can carry a path or input into an error.
func raiseSQLiteError(t *testing.T, message string) error {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	trigger := fmt.Sprintf(`CREATE TABLE probe(x); CREATE TRIGGER probe_fail BEFORE INSERT ON probe BEGIN SELECT RAISE(ABORT,'%s'); END`, strings.ReplaceAll(message, "'", "''"))
	if _, err = db.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO probe VALUES(1)`)
	var native *sqlite.Error
	if !errors.As(err, &native) {
		t.Fatalf("insert error %T %v, want *sqlite.Error", err, err)
	}
	return err
}

// assertNoSourceText fails when any byte of the raw log could belong to a
// path or to input: no entry field carries a slash, and none of the probe
// words may appear.
func assertNoSourceText(t *testing.T, raw []byte, words ...string) {
	t.Helper()
	if bytes.ContainsRune(raw, '/') {
		t.Fatalf("diagnostic log contains a path separator:\n%s", raw)
	}
	for _, word := range words {
		if word != "" && bytes.Contains(raw, []byte(word)) {
			t.Fatalf("diagnostic log contains %q:\n%s", word, raw)
		}
	}
}

func sqliteConstraintTrigger(stage string, types ...string) diagnosticEntry {
	return diagnosticEntry{
		Stage: stage, ErrorCode: "scan_failed", ErrorTypes: types,
		SQLiteCode: 1811, SQLiteCodeName: "SQLITE_CONSTRAINT_TRIGGER", SQLitePrimaryCode: 19, SQLitePrimaryName: "SQLITE_CONSTRAINT",
	}
}

// Every probe the R1 and R2 reviews used to leak a path fragment, a file name
// or transcript text now yields only the structured description.
func TestDiagnosticEntryIsStructuredForEveryReviewProbe(t *testing.T) {
	home, root := preparedDiagnosticRoot(t)
	projects := filepath.Join(home, ".claude", "projects")
	_, spaced := os.Open(filepath.Join(projects, "Acme Corp Secret", "my client chat.jsonl"))
	_, bracketed := os.Open(filepath.Join(projects, "project(TopSecret)", "a,Hidden,b.jsonl"))
	_, separated := os.Open("/Volumes/Notes: Acme Corp Secret/chat log.jsonl")
	linked := os.Rename("/nonexistent dir/Old Name (1).jsonl", "/nonexistent dir/New, Name.jsonl")
	var syntax *json.SyntaxError
	checkpointErr := json.Unmarshal([]byte("{"), &map[string]any{})
	if !errors.As(checkpointErr, &syntax) {
		t.Fatalf("checkpoint decode error %T, want *json.SyntaxError", checkpointErr)
	}
	_, timestampErr := time.Parse(time.RFC3339Nano, `/Probe"Fictional Transcript Snippet`)
	notFound := diagnosticEntry{Stage: "discovery", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fs.PathError", "syscall.Errno"}, Sentinels: []string{"fs.ErrNotExist"}, SystemErrors: []diagnosticSyscall{{Type: "*fs.PathError", Op: "open", Errno: "ENOENT"}}}
	cases := []struct {
		name string
		err  error
		want diagnosticEntry
	}{
		{"path with several spaces", withStage("discovery", spaced), notFound},
		{"path with brackets and commas", withStage("discovery", bracketed), notFound},
		{"path holding a separator", withStage("discovery", separated), notFound},
		{"link error", withStage("finalize_usage", linked), diagnosticEntry{Stage: "finalize_usage", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*os.LinkError", "syscall.Errno"}, Sentinels: []string{"fs.ErrNotExist"}, SystemErrors: []diagnosticSyscall{{Type: "*os.LinkError", Op: "rename", Errno: "ENOENT"}}}},
		{"sqlite path with one space", withStage("scan_session", raiseSQLiteError(t, "failed under "+filepath.Join(projects, "client x", "secret.jsonl"))), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"sqlite bare name and quoted input", withStage("scan_session", raiseSQLiteError(t, "duplicate source client-secret.jsonl for 'confidential prompt'")), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"sqlite colon in path", withStage("scan_session", raiseSQLiteError(t, "failed under "+filepath.Join(home, "Case: Fictional Client Notes.jsonl"))), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"sqlite apostrophe in path", withStage("scan_session", raiseSQLiteError(t, "failed under "+filepath.Join(home, "Team's Fictional Client", "Notes.jsonl"))), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"sqlite newline in path", withStage("scan_session", raiseSQLiteError(t, "failed under "+filepath.Join(home, "Line\nFictional Client", "Notes.jsonl"))), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"sqlite multi-word file name", withStage("scan_session", raiseSQLiteError(t, "failed under Fictional Client Notes.jsonl")), sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")},
		{"quoted path with escaped quote", withStage("scan_usage", fmt.Errorf("prepare source %q: %w", filepath.Join(home, `Team"Fictional Private Notes`, "s.jsonl"), checkpointErr)), diagnosticEntry{Stage: "scan_usage", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fmt.wrapError", "*json.SyntaxError"}}},
		{"raw transcript timestamp", withStage("scan_usage", fmt.Errorf("parse usage timestamp: %w", timestampErr)), diagnosticEntry{Stage: "scan_usage", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fmt.wrapError", "*time.ParseError"}}},
		{"unquoted transcript text", withStage("full_session_scan", errors.New("unexpected record please refactor the billing module SECRET-TOKEN-42")), diagnosticEntry{Stage: "full_session_scan", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*errors.errorString"}}},
	}
	log := newDiagnosticLog(root)
	log.now = func() time.Time { return diagnosticTestClock }
	probeInventory := fmt.Sprintf("%024x.%024x", 0xa00, 0xaff)
	var want strings.Builder
	for index, tc := range cases {
		if err := log.record(fmt.Sprintf("%024x", 0xa00+index), probeInventory, "session", tc.err); err != nil {
			t.Fatal(err)
		}
		expected := tc.want
		expected.At = diagnosticTestClock.Format(time.RFC3339Nano)
		expected.RoundID = fmt.Sprintf("%024x", 0xa00+index)
		expected.InventoryID = probeInventory
		expected.Domain = "session"
		want.WriteString(encodeDiagnosticEntry(t, expected))
	}
	raw, err := os.ReadFile(log.path)
	if err != nil {
		t.Fatal(err)
	}
	gotLines := strings.SplitAfter(string(raw), "\n")
	wantLines := strings.SplitAfter(want.String(), "\n")
	for index := range cases {
		if index >= len(gotLines) || gotLines[index] != wantLines[index] {
			got := ""
			if index < len(gotLines) {
				got = gotLines[index]
			}
			t.Errorf("%s:\n got %s\nwant %s", cases[index].name, got, wantLines[index])
		}
	}
	if string(raw) != want.String() {
		t.Fatalf("diagnostic log differs from the structured entries")
	}
	assertNoSourceText(t, raw, home, root, "Acme", "Corp", "Secret", "Hidden", "client", "Client", "chat", "Notes", "Fictional", "Probe", "Snippet", "confidential", "billing", "SECRET-TOKEN-42", "jsonl", "Volumes", "nonexistent", "Name")
}

// The failures most likely in the field stay locatable by stage and class
// without any error text: lock contention, a vanished or unreadable source,
// cancellation and a deadline.
func TestDiagnosticEntryLocatesLikelyFieldFailures(t *testing.T) {
	home, root := preparedDiagnosticRoot(t)
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "contention.sqlite3")
	open := func() *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite", databasePath+"?_pragma=busy_timeout(0)&_pragma=journal_mode(wal)")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { db.Close() })
		return db
	}
	holder, contender := open(), open()
	if _, err := holder.Exec(`CREATE TABLE t(x)`); err != nil {
		t.Fatal(err)
	}
	// SQLITE_BUSY: another connection holds the write lock.
	lock, err := holder.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lock.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	_, busy := contender.Exec(`INSERT INTO t VALUES(1)`)
	if _, err = lock.ExecContext(ctx, `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	lock.Close()
	// SQLITE_BUSY_SNAPSHOT: a read transaction tries to write after another
	// connection committed past its snapshot.
	reader, err := contender.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reader.ExecContext(ctx, `BEGIN`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = reader.QueryRowContext(ctx, `SELECT count(*) FROM t`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if _, err = holder.Exec(`INSERT INTO t VALUES(2)`); err != nil {
		t.Fatal(err)
	}
	_, snapshot := reader.ExecContext(ctx, `INSERT INTO t VALUES(3)`)
	_, _ = reader.ExecContext(ctx, `ROLLBACK`)
	reader.Close()
	_, missing := os.Stat(filepath.Join(home, ".codex", "sessions", "vanished rollout.jsonl"))
	unreadablePath := filepath.Join(home, "unreadable rollout.jsonl")
	if err = os.WriteFile(unreadablePath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(unreadablePath, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadablePath, 0o600) })
	_, denied := os.Open(unreadablePath)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	expired, stop := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stop()
	cases := []struct {
		name string
		err  error
		want diagnosticEntry
	}{
		{"busy", withStage("finish_usage_inventory", fmt.Errorf("finish inventory: %w", busy)), diagnosticEntry{Stage: "finish_usage_inventory", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fmt.wrapError", "*sqlite.Error"}, SQLiteCode: 5, SQLiteCodeName: "SQLITE_BUSY", SQLitePrimaryCode: 5, SQLitePrimaryName: "SQLITE_BUSY"}},
		{"busy snapshot", withStage("scan_usage", snapshot), diagnosticEntry{Stage: "scan_usage", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*sqlite.Error"}, SQLiteCode: 517, SQLiteCodeName: "SQLITE_BUSY_SNAPSHOT", SQLitePrimaryCode: 5, SQLitePrimaryName: "SQLITE_BUSY"}},
		{"vanished source", withStage("batch 3 of 9 scan_session", missing), diagnosticEntry{Stage: "batch 3 of 9 scan_session", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fs.PathError", "syscall.Errno"}, Sentinels: []string{"fs.ErrNotExist"}, SystemErrors: []diagnosticSyscall{{Type: "*fs.PathError", Op: "stat", Errno: "ENOENT"}}}},
		{"unreadable source", withStage("discovery", denied), diagnosticEntry{Stage: "discovery", ErrorCode: "scan_failed", ErrorTypes: []string{"*scanruntime.stageError", "*fs.PathError", "syscall.Errno"}, Sentinels: []string{"fs.ErrPermission"}, SystemErrors: []diagnosticSyscall{{Type: "*fs.PathError", Op: "open", Errno: "EACCES"}}}},
		{"cancelled", withStage("batch 2 of 5 start", cancelled.Err()), diagnosticEntry{Stage: "batch 2 of 5 start", ErrorCode: "cancelled", ErrorTypes: []string{"*scanruntime.stageError", "*errors.errorString"}, Sentinels: []string{"context.Canceled"}}},
		{"deadline", withStage("settle_and_classify", expired.Err()), diagnosticEntry{Stage: "settle_and_classify", ErrorCode: "deadline_exceeded", ErrorTypes: []string{"*scanruntime.stageError", "context.deadlineExceededError"}, Sentinels: []string{"context.DeadlineExceeded"}}},
	}
	log := newDiagnosticLog(root)
	log.now = func() time.Time { return diagnosticTestClock }
	for index, tc := range cases {
		if err := log.record(fmt.Sprintf("%024x", 0xb00+index), "", "usage", tc.err); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(log.path)
	if err != nil {
		t.Fatal(err)
	}
	entries := readDiagnosticEntries(t, log.path)
	if len(entries) != len(cases) {
		t.Fatalf("entries=%d, want %d", len(entries), len(cases))
	}
	for index, tc := range cases {
		want := tc.want
		want.At = diagnosticTestClock.Format(time.RFC3339Nano)
		want.RoundID = fmt.Sprintf("%024x", 0xb00+index)
		want.Domain = "usage"
		if !reflect.DeepEqual(entries[index], want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, entries[index], want)
		}
	}
	assertNoSourceText(t, raw, home, root, "vanished", "unreadable", "rollout", "contention")
}

func TestDiagnosticLogRotatesAtItsSizeLimitAndStaysPrivate(t *testing.T) {
	_, root := preparedDiagnosticRoot(t)
	log := newDiagnosticLog(root)
	log.maxBytes = 2048
	// A log left with a wider mode is tightened by the next write.
	if err := os.WriteFile(log.path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(log.path, 0o644); err != nil {
		t.Fatal(err)
	}
	cause := withStage("finalize_usage", errors.New("database is locked"))
	for index := range 40 {
		if err := log.record(fmt.Sprintf("%024x", index), "", "usage", cause); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			info, err := os.Stat(log.path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("first write left the log at mode %v, want 0600", info.Mode().Perm())
			}
		}
	}
	current, err := os.Stat(log.path)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := os.Stat(log.path + ".1")
	if err != nil {
		t.Fatalf("no rotated log after 40 entries: %v", err)
	}
	for _, info := range []os.FileInfo{current, rotated} {
		if info.Size() > log.maxBytes {
			t.Fatalf("%s size=%d exceeds the %d byte limit", info.Name(), info.Size(), log.maxBytes)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode=%v, want 0600", info.Name(), info.Mode().Perm())
		}
	}
	latest := readDiagnosticEntries(t, log.path)
	older := readDiagnosticEntries(t, log.path+".1")
	if len(latest) == 0 || latest[len(latest)-1].RoundID != fmt.Sprintf("%024x", 39) {
		t.Fatalf("current log does not end with the newest entry: %+v", latest)
	}
	if len(older) == 0 || older[len(older)-1].RoundID >= latest[0].RoundID {
		t.Fatalf("rotated log is not the generation before the current one: older=%+v latest[0]=%+v", older, latest[0])
	}
	matches, err := filepath.Glob(log.path + "*")
	if err != nil || len(matches) != 2 {
		t.Fatalf("log generations=%v err=%v, want the current file and one rotation", matches, err)
	}
}

func TestDiagnosticLogTightensAWideOversizedLogBeforeRotatingIt(t *testing.T) {
	_, root := preparedDiagnosticRoot(t)
	log := newDiagnosticLog(root)
	log.maxBytes = 256
	preset := []byte(strings.Repeat("x", 241) + "\n")
	if err := os.WriteFile(log.path, preset, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(log.path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := log.record(newID(), "", "usage", withStage("finalize_usage", errors.New("database is locked"))); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.Stat(log.path + ".1")
	if err != nil {
		t.Fatalf("oversized log was not rotated: %v", err)
	}
	if rotated.Mode().Perm() != 0o600 {
		t.Fatalf("rotated log mode=%v, want 0600", rotated.Mode().Perm())
	}
	if contents, err := os.ReadFile(log.path + ".1"); err != nil || string(contents) != string(preset) {
		t.Fatalf("rotated generation=%q err=%v, want the preset log", contents, err)
	}
	current, err := os.Stat(log.path)
	if err != nil || current.Mode().Perm() != 0o600 {
		t.Fatalf("current log=%v err=%v, want mode 0600", current, err)
	}
	if entries := readDiagnosticEntries(t, log.path); len(entries) != 1 || !generatedID(entries[0].RoundID) {
		t.Fatalf("current log entries=%+v, want only the new entry", entries)
	}
}

func TestDiagnosticLogRefusesASymlink(t *testing.T) {
	_, root := preparedDiagnosticRoot(t)
	target := filepath.Join(t.TempDir(), "elsewhere.log")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	log := newDiagnosticLog(root)
	if err := os.Symlink(target, log.path); err != nil {
		t.Fatal(err)
	}
	if err := log.record(newID(), "", "usage", errors.New("boom")); err == nil {
		t.Fatal("diagnostic log wrote through a symlink")
	}
	if contents, _ := os.ReadFile(target); len(contents) != 0 {
		t.Fatalf("symlink target received %q", contents)
	}
}

func TestStageWrapperKeepsFailureClassification(t *testing.T) {
	for _, cause := range []error{
		store.ErrTelemetrySourceIdentityPending,
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("plain"),
	} {
		wrapped := errors.Join(withStage("batch 2 of 3 scan_usage", cause), withStage("settle_and_classify", nil))
		if got, want := failureCode(wrapped), failureCode(cause); got != want {
			t.Fatalf("failureCode(%v)=%q, want %q", wrapped, got, want)
		}
	}
	if withStage("unused", nil) != nil {
		t.Fatal("withStage wrapped a nil error")
	}
}

// diagnosticRefresh runs one App serving refresh through the worker's real
// acceptance path and returns its terminal receipt.
func diagnosticRefresh(t *testing.T, worker *server, journal *receiptJournal, home, id string, now time.Time) scanReceipt {
	t.Helper()
	request := wireRequest{Protocol: ProtocolVersion, Request: id, StateID: "state", Home: home, Scope: ScopeBoth, AppOwned: true, Serving: true, CacheNow: now.Format(time.RFC3339Nano)}
	round, terminal, err := worker.accept(request)
	if err != nil || terminal != nil {
		t.Fatalf("accept %s round=%v terminal=%v err=%v", id, round, terminal, err)
	}
	select {
	case <-round.done:
	case <-time.After(30 * time.Second):
		t.Fatalf("refresh %s did not terminate", id)
	}
	receipt, found := journal.get(id)
	if !found || receipt.State != receiptTerminal || receipt.Result == nil {
		t.Fatalf("receipt %s=%#v found=%t, want terminal", id, receipt, found)
	}
	return receipt
}

func newDiagnosticWorker(t *testing.T, root string) (*server, *receiptJournal) {
	t.Helper()
	journal, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	return &server{
		ctx:         context.Background(),
		stateRoot:   root,
		stateID:     "state",
		journal:     journal,
		diagnostics: newDiagnosticLog(root),
		rounds:      map[string]*round{},
	}, journal
}

// failSessionRegistry makes registering a new session source fail with a
// SQLite message that names the source path.
func failSessionRegistry(t *testing.T, root, path string) {
	t.Helper()
	sessions, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	trigger := fmt.Sprintf(`CREATE TRIGGER fail_session_registry BEFORE INSERT ON session_sources BEGIN SELECT RAISE(ABORT,'synthetic registry failure for %s'); END`, strings.ReplaceAll(path, "'", "''"))
	if _, err = sessions.DB.Exec(trigger); err != nil {
		t.Fatal(err)
	}
}

// receiptEntry finds the single log entry of the receipt's round and checks
// it is exactly want, apart from the clock and the round's inventory id.
func receiptEntry(t *testing.T, path string, receipt scanReceipt, want diagnosticEntry) {
	t.Helper()
	var matched []diagnosticEntry
	for _, entry := range readDiagnosticEntries(t, path) {
		if entry.RoundID == receipt.RoundID {
			matched = append(matched, entry)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("entries for receipt round %s=%+v, want one", receipt.RoundID, matched)
	}
	entry := matched[0]
	if entry.InventoryID == "" || entry.At == "" {
		t.Fatalf("entry=%+v, want the round inventory and a timestamp", entry)
	}
	want.At, want.RoundID, want.InventoryID = entry.At, receipt.RoundID, entry.InventoryID
	if !reflect.DeepEqual(entry, want) {
		t.Fatalf("entry:\n got %+v\nwant %+v", entry, want)
	}
}

func TestServingFailureKeepsStageAndCauseUnderTheReceiptRound(t *testing.T) {
	home, root := preparedDiagnosticRoot(t)
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	failSessionRegistry(t, root, filepath.Join(home, ".codex", "sessions", "Acme Corp Secret"))
	worker, journal := newDiagnosticWorker(t, root)
	receipt := diagnosticRefresh(t, worker, journal, home, "app-refresh", now)
	if receipt.Result.Session.State != "failed" || receipt.Result.Session.ErrorCode != "scan_failed" {
		t.Fatalf("receipt session=%#v, want the unchanged scan_failed classification", receipt.Result.Session)
	}
	info, err := os.Stat(worker.diagnostics.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("diagnostic log mode=%v, want 0600", info.Mode().Perm())
	}
	want := sqliteConstraintTrigger("batch 1 of 1 scan_session", "*scanruntime.stageError", "*sqlite.Error")
	want.Domain = "usage,session"
	receiptEntry(t, worker.diagnostics.path, receipt, want)
	raw, err := os.ReadFile(worker.diagnostics.path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSourceText(t, raw, home, root, "Acme", "Corp", "Secret", "synthetic", "jsonl")
}

// The App's steady-state refresh: once today's full publication is readable,
// a serving request runs the ordinary production round, and its failures must
// reach the same log under the receipt's round id.
func TestSettledServingRefreshFailureKeepsStageAndCauseUnderTheReceiptRound(t *testing.T) {
	home, root := preparedDiagnosticRoot(t)
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	worker, journal := newDiagnosticWorker(t, root)
	first := diagnosticRefresh(t, worker, journal, home, "first-refresh", now)
	if first.Result.Usage.State != "completed" || first.Result.Session.State != "completed" {
		t.Fatalf("first refresh=%#v, want completed", first.Result)
	}
	if !servingSettled(context.Background(), root, home, func() time.Time { return now }) {
		t.Fatal("first refresh did not settle today's publication")
	}
	// A new transcript arrives; registering it fails with a message that
	// names its path.
	added := writeServingSource(t, home, filepath.Join(now.UTC().Format("2006/01/02"), "Acme Corp Secret"), now, 5)
	failSessionRegistry(t, root, added)
	second := diagnosticRefresh(t, worker, journal, home, "second-refresh", now)
	if second.Result.Session.State != "failed" || second.Result.Session.ErrorCode != "scan_failed" {
		t.Fatalf("second refresh session=%#v, want the unchanged scan_failed classification", second.Result.Session)
	}
	if _, err := os.Stat(worker.diagnostics.path); err != nil {
		t.Fatalf("settled refresh failure left no diagnostic log: %v", err)
	}
	for _, entry := range readDiagnosticEntries(t, worker.diagnostics.path) {
		if entry.RoundID == first.RoundID {
			t.Fatalf("successful first refresh was logged: %+v", entry)
		}
	}
	// "scan_session" without a batch prefix is the production round's stage;
	// the first-publication path would report "batch 1 of 1 scan_session".
	want := sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")
	want.Domain = "session"
	receiptEntry(t, worker.diagnostics.path, second, want)
	raw, err := os.ReadFile(worker.diagnostics.path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSourceText(t, raw, home, root, "Acme", "Corp", "Secret", "synthetic", "jsonl")
}

func TestDiagnosticIDsAcceptOnlyGeneratedShapes(t *testing.T) {
	round, attempt := newID(), newID()
	for _, tc := range []struct {
		round, inventory, wantRound, wantInventory string
	}{
		{round, round, round, round},
		{round, round + "." + attempt, round, round + "." + attempt},
		{"1764235800123456789", "1764235800123456789.42", "1764235800123456789", "1764235800123456789.42"},
		{round, "", round, ""},
		{"/R3JournalPrivate/fixture.jsonl", "/R3JournalPrivate/fixture.jsonl." + attempt, "invalid", "invalid"},
		{"ABCDEF0123456789ABCDEF01", round + ".", "invalid", "invalid"},
		{"123456789012345678901234", "123456789012345678901234." + attempt, "123456789012345678901234", "123456789012345678901234." + attempt},
		{"70000000000000001127", "70000000000000001127", "invalid", "invalid"},
		{"0123", round + ".0123", "invalid", "invalid"},
		{round[:23], round + "." + attempt + "." + attempt, "invalid", "invalid"},
		{"persisted-round", "persisted-round", "invalid", "invalid"},
		{"123456789012345678901", "." + attempt, "invalid", "invalid"},
	} {
		if got := diagnosticRoundID(tc.round); got != tc.wantRound {
			t.Errorf("diagnosticRoundID(%q)=%q, want %q", tc.round, got, tc.wantRound)
		}
		if got := diagnosticInventoryID(tc.inventory); got != tc.wantInventory {
			t.Errorf("diagnosticInventoryID(%q)=%q, want %q", tc.inventory, got, tc.wantInventory)
		}
	}
}

// A recovered CLI round takes its id from the private receipt journal. A
// damaged or rewritten journal must not put its text into the log, while the
// recovered round still runs and its failure is still recorded.
func TestRecoveredReceiptWithForeignRoundIDLogsPlaceholder(t *testing.T) {
	home, root := preparedDiagnosticRoot(t)
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	failSessionRegistry(t, root, filepath.Join(home, ".codex", "sessions"))
	foreign := "/R3JournalPrivate/fixture.jsonl"
	journal, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.accept(scanReceipt{ID: "cli-request", StateID: "state", Home: home, Scope: ScopeBoth, RoundID: foreign, Observation: 1}); err != nil {
		t.Fatal(err)
	}
	reopened, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	worker := &server{
		ctx:         context.Background(),
		stateRoot:   root,
		stateID:     "state",
		journal:     reopened,
		diagnostics: newDiagnosticLog(root),
		rounds:      map[string]*round{},
	}
	if err = worker.recoverReceipts(); err != nil {
		t.Fatal(err)
	}
	worker.mu.Lock()
	recovered := worker.round
	worker.mu.Unlock()
	if recovered == nil || recovered.id != foreign {
		t.Fatalf("recovered round=%v, want the journal's round", recovered)
	}
	select {
	case <-recovered.done:
	case <-time.After(30 * time.Second):
		t.Fatal("recovered round did not terminate")
	}
	receipt, found := reopened.get("cli-request")
	if !found || receipt.State != receiptTerminal || receipt.Result == nil || receipt.RoundID != foreign {
		t.Fatalf("receipt=%#v found=%t, want the recovered terminal receipt", receipt, found)
	}
	if receipt.Result.Session.State != "failed" || receipt.Result.Session.ErrorCode != "scan_failed" {
		t.Fatalf("receipt session=%#v, want scan_failed", receipt.Result.Session)
	}
	raw, err := os.ReadFile(worker.diagnostics.path)
	if err != nil {
		t.Fatalf("recovered round failure left no diagnostic log: %v", err)
	}
	entries := readDiagnosticEntries(t, worker.diagnostics.path)
	if len(entries) != 1 {
		t.Fatalf("entries=%+v, want the recovered session failure", entries)
	}
	want := sqliteConstraintTrigger("scan_session", "*scanruntime.stageError", "*sqlite.Error")
	want.At, want.RoundID, want.InventoryID, want.Domain = entries[0].At, "invalid", "invalid", "session"
	if !reflect.DeepEqual(entries[0], want) {
		t.Fatalf("entry:\n got %+v\nwant %+v", entries[0], want)
	}
	assertNoSourceText(t, raw, home, root, "R3JournalPrivate", "fixture", "jsonl")
}
