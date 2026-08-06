package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestApprovedDocumentRejectsProhibitedContent(t *testing.T) {
	for _, kind := range []string{"system", "developer", "reasoning", "tool_result", "environment"} {
		if _, err := ApprovedDocument("codex", "s", kind, "secret"); err == nil {
			t.Fatalf("%s accepted", kind)
		}
	}
	if _, err := ApprovedDocument("claude", "s", "assistant_final", "visible"); err != nil {
		t.Fatal(err)
	}
}
func TestScanCodexFixtureIndexesOnlyVisibleFields(t *testing.T) {
	root := t.TempDir()
	s, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := filepath.Join(root, "x.jsonl")
	data := "{\"type\":\"session_meta\",\"payload\":{\"session_id\":\"s\"}}\n{\"type\":\"system\",\"payload\":{\"text\":\"secret\"}}\n{\"type\":\"visible_user_prompt\",\"payload\":{\"text\":\"find battery\"}}\n{\"type\":\"visible_assistant_final\",\"payload\":{\"text\":\"visible reply\"}}\n{\"type\":\"tool_result\",\"payload\":{\"text\":\"token\"}}\n"
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := ScanCodexFixture(context.Background(), s.DB, p)
	if err != nil || n != 2 {
		t.Fatalf("scan=%d,%v", n, err)
	}
	got, err := Search(context.Background(), s.DB, "visible")
	if err != nil || len(got) != 1 {
		t.Fatalf("search=%v,%v", got, err)
	}
	if got[0].Text != "visible reply" {
		t.Fatal(got)
	}
}

func TestSearchOrdersDocumentsByNormalizedEventAt(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "timestamps.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := "{\"type\":\"session_meta\",\"payload\":{\"session_id\":\"s\"}}\n" +
		"{\"type\":\"visible_user_prompt\",\"timestamp\":\"2026-02-01T10:00:00+08:00\",\"payload\":{\"text\":\"needle earliest\"}}\n" +
		"{\"type\":\"visible_assistant_final\",\"timestamp\":\"2026-02-01T03:00:00Z\",\"payload\":{\"text\":\"needle latest\"}}\n" +
		"{\"type\":\"visible_user_prompt\",\"timestamp\":\"not-a-time\",\"payload\":{\"text\":\"needle unknown\"}}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := Scan(context.Background(), database.DB, home); err != nil {
		t.Fatal(err)
	}
	documents, err := Search(context.Background(), database.DB, "needle")
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 3 {
		t.Fatalf("documents = %#v", documents)
	}
	if documents[0].Text != "needle latest" || documents[0].EventAt != "2026-02-01T03:00:00Z" {
		t.Fatalf("latest = %#v", documents[0])
	}
	if documents[1].Text != "needle earliest" || documents[1].EventAt != "2026-02-01T02:00:00Z" {
		t.Fatalf("earliest = %#v", documents[1])
	}
	if documents[2].Text != "needle unknown" || documents[2].EventAt != "" {
		t.Fatalf("unknown = %#v", documents[2])
	}
}

func TestSearchPreservesSourceOrderForEqualEventAt(t *testing.T) {
	database, err := store.OpenSessions(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	documents := []Document{
		{Client: "codex", SessionID: "s", EventAt: "2026-08-06T00:00:00Z", Kind: "user_prompt", Text: "needle first"},
		{Client: "codex", SessionID: "s", EventAt: "2026-08-06T00:00:00Z", Kind: "assistant_final", Text: "needle second"},
	}
	if err := ReplaceDocuments(context.Background(), database.DB, "codex", "s", documents); err != nil {
		t.Fatal(err)
	}
	got, err := Search(context.Background(), database.DB, "needle")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "needle first" || got[1].Text != "needle second" {
		t.Fatalf("ordered documents = %#v", got)
	}
}

func TestScanRejectsStructuredPrivacyCounterexamples(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "privacy.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := "{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"allowed\"}}\n" +
		"{\"type\":\"developer\",\"session_id\":\"s\",\"payload\":{\"text\":\"developer-only\"}}\n" +
		"{\"type\":\"response_item\",\"session_id\":\"s\",\"payload\":{\"item\":{\"type\":\"reasoning\",\"text\":\"hidden-reasoning\"}}}\n" +
		"{\"type\":\"response_item\",\"session_id\":\"s\",\"payload\":{\"item\":{\"type\":\"tool_call\",\"arguments\":\"tool-arguments\"}}}\n" +
		"{\"type\":\"response_item\",\"session_id\":\"s\",\"payload\":{\"item\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_image\",\"image_url\":\"image-bytes\"}]},\"environment\":\"environment-value\",\"credential\":\"credential-value\",\"attachment\":\"attachment-bytes\"}}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"developer", "reasoning", "arguments", "image", "environment", "credential", "attachment"} {
		if docs, err := Search(context.Background(), s.DB, query); err != nil || len(docs) != 0 {
			t.Fatalf("prohibited query %q returned docs=%v err=%v", query, docs, err)
		}
	}
}

func TestScanClaudeAllowlistAndExclusion(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	p := filepath.Join(home, ".claude", "projects", "p", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	data := "{\"type\":\"user\",\"sessionId\":\"s\",\"cwd\":\"/work/p\",\"message\":{\"content\":\"visible prompt\"}}\n" +
		"{\"type\":\"assistant\",\"sessionId\":\"s\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"visible answer\"}]}}\n" +
		"{\"type\":\"assistant\",\"sessionId\":\"s\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"input\":\"credential\"}]}}\n" +
		"{\"type\":\"system\",\"sessionId\":\"s\",\"text\":\"hidden reasoning\"}\n"
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := Scan(context.Background(), s.DB, home); err != nil || got.Documents != 2 {
		t.Fatalf("scan=%+v err=%v", got, err)
	}
	if docs, err := Search(context.Background(), s.DB, "credential OR reasoning"); err != nil || len(docs) != 0 {
		t.Fatalf("prohibited docs=%v err=%v", docs, err)
	}
	if err := Exclude(context.Background(), s.DB, "session", "s"); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "visible"); err != nil || len(docs) != 0 {
		t.Fatalf("excluded docs=%v err=%v", docs, err)
	}
}

func assertSessionSourceOwnership(t *testing.T, database *sql.DB, path string, wantSources, wantMetadata, wantDocuments int) {
	t.Helper()
	for _, check := range []struct {
		table string
		want  int
	}{
		{table: "session_sources", want: wantSources},
		{table: "session_metadata", want: wantMetadata},
		{table: "session_documents", want: wantDocuments},
	} {
		var got int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE source_path = ?", check.table)
		if err := database.QueryRow(query, filepath.Clean(path)).Scan(&got); err != nil {
			t.Fatalf("count %s ownership for %q: %v", check.table, path, err)
		}
		if got != check.want {
			t.Fatalf("%s ownership for %q = %d rows, want %d", check.table, path, got, check.want)
		}
	}
}

func TestScanRemovesDocumentsDeletedFromOrAlongWithSource(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	initial := "{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"private prompt\"}}\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	var cursor int64
	var partialLine string
	if err := s.DB.QueryRow("SELECT cursor, partial_line FROM session_sources WHERE source_path = ?", filepath.Clean(path)).Scan(&cursor, &partialLine); err != nil || cursor != int64(len(initial)) || partialLine != "" {
		t.Fatalf("source cursor=%d partial=%q err=%v", cursor, partialLine, err)
	}
	if docs, err := Search(context.Background(), s.DB, "private"); err != nil || len(docs) != 1 {
		t.Fatalf("initial docs=%v err=%v", docs, err)
	}
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "private"); err != nil || len(docs) != 0 {
		t.Fatalf("rewritten source left docs=%v err=%v", docs, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	if entries, err := List(context.Background(), s.DB); err != nil || len(entries) != 0 {
		t.Fatalf("removed source left metadata=%v err=%v", entries, err)
	}
	assertSessionSourceOwnership(t, s.DB, path, 0, 0, 0)
}

func TestScanKeepsActiveSourceAuthoritativeAndFallsBackToArchive(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	archive := filepath.Join(home, ".codex", "archived_sessions", "archive.jsonl")
	active := filepath.Join(home, ".codex", "sessions", "active.jsonl")
	for _, path := range []string{archive, active} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, text string) {
		t.Helper()
		data := "{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"" + text + "\"}}\n"
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(archive, "archive")
	write(active, "active")
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "active"); err != nil || len(docs) != 1 {
		t.Fatalf("active source was not selected: docs=%v err=%v", docs, err)
	}
	write(archive, "archive changed")
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "active"); err != nil || len(docs) != 1 {
		t.Fatalf("archive rewrite replaced active source: docs=%v err=%v", docs, err)
	}
	if err := os.Remove(active); err != nil {
		t.Fatal(err)
	}
	if result, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	} else if result.Removed != 0 {
		t.Fatalf("duplicate owner removal removed %d logical documents", result.Removed)
	}
	if docs, err := Search(context.Background(), s.DB, "archive"); err != nil || len(docs) != 1 {
		t.Fatalf("archive did not replace removed active source: docs=%v err=%v", docs, err)
	}
	assertSessionSourceOwnership(t, s.DB, active, 0, 0, 0)
	assertSessionSourceOwnership(t, s.DB, archive, 1, 1, 1)
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if result, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	} else if result.Removed != 1 {
		t.Fatalf("last source removal removed %d logical documents, want 1", result.Removed)
	}
	assertSessionSourceOwnership(t, s.DB, archive, 0, 0, 0)
}

func TestScanLastSourceRemovalCountsAllLogicalDocuments(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "many.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := "{\"type\":\"visible_user_prompt\",\"session_id\":\"many\",\"payload\":{\"text\":\"one\"}}\n" +
		"{\"type\":\"visible_assistant_final\",\"session_id\":\"many\",\"payload\":{\"text\":\"two\"}}\n" +
		"{\"type\":\"visible_user_prompt\",\"session_id\":\"many\",\"payload\":{\"text\":\"three\"}}\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Documents != 3 {
		t.Fatalf("initial scan = %#v, %v", result, scanErr)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Removed != 3 {
		t.Fatalf("removal scan = %#v, %v", result, scanErr)
	}
}

func TestScanDuplicateSourceRemovalCountsOnlyVisibleLogicalChanges(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	active := filepath.Join(home, ".codex", "sessions", "duplicate.jsonl")
	archive := filepath.Join(home, ".codex", "archived_sessions", "duplicate.jsonl")
	contents := "{\"type\":\"visible_user_prompt\",\"session_id\":\"duplicate\",\"payload\":{\"text\":\"same\"}}\n" +
		"{\"type\":\"visible_assistant_final\",\"session_id\":\"duplicate\",\"payload\":{\"text\":\"same reply\"}}\n"
	for _, path := range []string{active, archive} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	database, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Documents != 2 || result.Removed != 0 {
		t.Fatalf("initial duplicate scan = %#v, %v", result, scanErr)
	}
	if err = os.Remove(active); err != nil {
		t.Fatal(err)
	}
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Documents != 0 || result.Removed != 0 {
		t.Fatalf("duplicate owner removal = %#v, %v", result, scanErr)
	}
	assertSessionSourceOwnership(t, database.DB, active, 0, 0, 0)
	assertSessionSourceOwnership(t, database.DB, archive, 1, 1, 2)
	if docs, searchErr := Search(context.Background(), database.DB, "same"); searchErr != nil || len(docs) != 2 {
		t.Fatalf("duplicate fallback search = %#v, %v; want two archived documents", docs, searchErr)
	}
	if err = os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Documents != 0 || result.Removed != 2 {
		t.Fatalf("final duplicate removal = %#v, %v", result, scanErr)
	}
	assertSessionSourceOwnership(t, database.DB, archive, 0, 0, 0)
	if docs, searchErr := Search(context.Background(), database.DB, "same"); searchErr != nil || len(docs) != 0 {
		t.Fatalf("final duplicate removal left searchable documents = %#v, %v", docs, searchErr)
	}
}

func TestVisibleDocumentChangesCountsLogicalSequenceEdits(t *testing.T) {
	document := func(text string) visibleDocument {
		return visibleDocument{kind: "user_prompt", text: text}
	}
	sequence := func(values ...string) map[string][]visibleDocument {
		documents := make([]visibleDocument, 0, len(values))
		for _, value := range values {
			documents = append(documents, document(value))
		}
		return map[string][]visibleDocument{"codex\x00session": documents}
	}
	tests := []struct {
		name                     string
		before, after            map[string][]visibleDocument
		wantChanged, wantRemoved int
	}{
		{name: "insert at start", before: sequence("one", "two", "three"), after: sequence("zero", "one", "two", "three"), wantChanged: 1},
		{name: "insert in middle", before: sequence("one", "two", "three"), after: sequence("one", "inserted", "two", "three"), wantChanged: 1},
		{name: "insert at end", before: sequence("one", "two", "three"), after: sequence("one", "two", "three", "four"), wantChanged: 1},
		{name: "delete at start", before: sequence("one", "two", "three"), after: sequence("two", "three"), wantRemoved: 1},
		{name: "delete in middle", before: sequence("one", "two", "three"), after: sequence("one", "three"), wantRemoved: 1},
		{name: "delete at end", before: sequence("one", "two", "three"), after: sequence("one", "two"), wantRemoved: 1},
		{name: "replace one", before: sequence("one", "two", "three"), after: sequence("one", "replacement", "three"), wantChanged: 1},
		{name: "delete repeated text", before: sequence("same", "middle", "same"), after: sequence("same", "same"), wantRemoved: 1},
		{name: "insert repeated text", before: sequence("same", "same"), after: sequence("same", "middle", "same"), wantChanged: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed, removed := visibleDocumentChanges(test.before, test.after)
			if changed != test.wantChanged || removed != test.wantRemoved {
				t.Fatalf("visibleDocumentChanges = (%d, %d), want (%d, %d)", changed, removed, test.wantChanged, test.wantRemoved)
			}
		})
	}
}

func TestTrimCommonDocumentWindowBoundsLargeSingleEdits(t *testing.T) {
	const documentCount = 10_000
	documents := make([]visibleDocument, documentCount)
	for index := range documents {
		documents[index] = visibleDocument{kind: "user_prompt", text: fmt.Sprintf("document-%05d", index)}
	}
	insertAt := func(index int) []visibleDocument {
		values := append([]visibleDocument{}, documents[:index]...)
		values = append(values, visibleDocument{kind: "assistant_final", text: "inserted"})
		return append(values, documents[index:]...)
	}
	deleteAt := func(index int) []visibleDocument {
		values := append([]visibleDocument{}, documents[:index]...)
		return append(values, documents[index+1:]...)
	}
	replaceAt := func(index int) []visibleDocument {
		values := append([]visibleDocument{}, documents...)
		values[index] = visibleDocument{kind: "assistant_final", text: "replacement"}
		return values
	}
	for _, test := range []struct {
		name                     string
		before, after            []visibleDocument
		wantBefore, wantAfter    int
		wantChanged, wantRemoved int
	}{
		{name: "unchanged", before: documents, after: documents},
		{name: "insert at start", before: documents, after: insertAt(0), wantAfter: 1, wantChanged: 1},
		{name: "insert in middle", before: documents, after: insertAt(documentCount / 2), wantAfter: 1, wantChanged: 1},
		{name: "insert at end", before: documents, after: insertAt(documentCount), wantAfter: 1, wantChanged: 1},
		{name: "delete at start", before: documents, after: deleteAt(0), wantBefore: 1, wantRemoved: 1},
		{name: "delete in middle", before: documents, after: deleteAt(documentCount / 2), wantBefore: 1, wantRemoved: 1},
		{name: "delete at end", before: documents, after: deleteAt(documentCount - 1), wantBefore: 1, wantRemoved: 1},
		{name: "replace at start", before: documents, after: replaceAt(0), wantBefore: 1, wantAfter: 1, wantChanged: 1},
		{name: "replace in middle", before: documents, after: replaceAt(documentCount / 2), wantBefore: 1, wantAfter: 1, wantChanged: 1},
		{name: "replace at end", before: documents, after: replaceAt(documentCount - 1), wantBefore: 1, wantAfter: 1, wantChanged: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, after := trimCommonDocumentWindow(test.before, test.after)
			if len(before) != test.wantBefore || len(after) != test.wantAfter {
				t.Fatalf("trimmed window = (%d, %d), want (%d, %d)", len(before), len(after), test.wantBefore, test.wantAfter)
			}
			changed, removed := logicalDocumentChanges(test.before, test.after)
			if changed != test.wantChanged || removed != test.wantRemoved {
				t.Fatalf("logical changes = (%d, %d), want (%d, %d)", changed, removed, test.wantChanged, test.wantRemoved)
			}
		})
	}
}

func BenchmarkVisibleDocumentChangesLargeSequences(b *testing.B) {
	const documentCount = 10_000
	documents := make([]visibleDocument, documentCount)
	for index := range documents {
		documents[index] = visibleDocument{kind: "user_prompt", text: fmt.Sprintf("document-%05d", index)}
	}
	middle := documentCount / 2
	inserted := append([]visibleDocument{}, documents[:middle]...)
	inserted = append(inserted, visibleDocument{kind: "assistant_final", text: "inserted"})
	inserted = append(inserted, documents[middle:]...)
	deleted := append([]visibleDocument{}, documents[:middle]...)
	deleted = append(deleted, documents[middle+1:]...)
	replaced := append([]visibleDocument{}, documents...)
	replaced[middle] = visibleDocument{kind: "assistant_final", text: "replacement"}

	for _, benchmark := range []struct {
		name          string
		before, after []visibleDocument
	}{
		{name: "unchanged", before: documents, after: documents},
		{name: "middle_insert", before: documents, after: inserted},
		{name: "middle_delete", before: documents, after: deleted},
		{name: "middle_replace", before: documents, after: replaced},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			before := map[string][]visibleDocument{"codex\x00session": benchmark.before}
			after := map[string][]visibleDocument{"codex\x00session": benchmark.after}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				visibleDocumentChanges(before, after)
			}
		})
	}
}

func TestScanAppendsAndContinuesPartialLine(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "append.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	first := "{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"first\"}}\n{\"type\":\"visible_assistant_final\",\"session_id\":\"s\",\"payload\":{\"text\":\"par"
	if err := os.WriteFile(path, []byte(first), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := Scan(context.Background(), s.DB, home); err != nil || got.Documents != 1 {
		t.Fatalf("first scan=%+v err=%v", got, err)
	}
	var partial []byte
	if err := s.DB.QueryRow("SELECT partial_line FROM session_sources WHERE source_path=?", path).Scan(&partial); err != nil || string(partial) == "" {
		t.Fatalf("partial=%q err=%v", partial, err)
	}
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600); err != nil {
		t.Fatal(err)
	} else {
		if _, err = f.WriteString("tial\"}}\n"); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	if got, err := Scan(context.Background(), s.DB, home); err != nil || got.Documents != 1 {
		t.Fatalf("append scan=%+v err=%v", got, err)
	}
	if docs, err := Search(context.Background(), s.DB, "first OR partial"); err != nil || len(docs) != 2 {
		t.Fatalf("docs=%v err=%v", docs, err)
	}
}

func TestScanRebuildsEqualLengthRewriteAndTracksMove(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	active := filepath.Join(home, ".codex", "sessions", "a.jsonl")
	archive := filepath.Join(home, ".codex", "archived_sessions", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(active), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\""+text+"\"}}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(active, "before")
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	write(active, "after!") // same byte length as "before"
	if _, err = Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "after"); err != nil || len(docs) != 1 {
		t.Fatalf("rewrite docs=%v err=%v", docs, err)
	}
	if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(active, archive); err != nil {
		t.Fatal(err)
	}
	if _, err = Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	shown, err := Show(context.Background(), s.DB, "codex", "s")
	if err != nil || shown.SourcePath != archive || len(shown.Documents) != 1 || shown.Documents[0].Text != "after!" {
		t.Fatalf("move show=%+v err=%v", shown, err)
	}
	assertSessionSourceOwnership(t, s.DB, active, 0, 0, 0)
	assertSessionSourceOwnership(t, s.DB, archive, 1, 1, 1)
	if docs, searchErr := Search(context.Background(), s.DB, "after"); searchErr != nil || len(docs) != 1 || docs[0].Text != "after!" {
		t.Fatalf("move search=%+v err=%v", docs, searchErr)
	}
}

func TestScanSkipsUnchangedSourceWithoutWritingState(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "unchanged.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"stable\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenSessions(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := s.DB.QueryRow("SELECT scanned_at FROM session_sources WHERE source_path=?", path).Scan(&before); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(context.Background(), s.DB, home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 1 || got.Sources != 0 || got.Documents != 0 {
		t.Fatalf("unchanged scan=%+v, want skipped=1 sources=0 documents=0", got)
	}
	var after string
	if err := s.DB.QueryRow("SELECT scanned_at FROM session_sources WHERE source_path=?", path).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("unchanged scan rewrote source state: before=%q after=%q", before, after)
	}
}

func TestReplaceDocumentsUsesSyntheticSource(t *testing.T) {
	s, err := store.OpenSessions(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	doc, err := ApprovedDocument("codex", "s", "user_prompt", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDocuments(context.Background(), s.DB, "codex", "s", []Document{doc}); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "synthetic"); err != nil || len(docs) != 1 {
		t.Fatalf("docs=%v err=%v", docs, err)
	}
	if _, err := Scan(context.Background(), s.DB, filepath.Join(t.TempDir(), "empty-home")); err != nil {
		t.Fatal(err)
	}
	if docs, err := Search(context.Background(), s.DB, "synthetic"); err != nil || len(docs) != 1 {
		t.Fatalf("scan removed synthetic docs=%v err=%v", docs, err)
	}
}

type sessionIndexSnapshot struct {
	sources    []string
	metadata   []string
	documents  []string
	exclusions []string
}

func snapshotSessionRows(t *testing.T, database *sql.DB, query string) []string {
	t.Helper()
	rows, err := database.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err = rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(values))
		for index, value := range values {
			switch typed := value.(type) {
			case []byte:
				parts[index] = fmt.Sprintf("bytes:%x", typed)
			default:
				parts[index] = fmt.Sprintf("%T:%v", value, value)
			}
		}
		out = append(out, strings.Join(parts, "|"))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func captureSessionIndex(t *testing.T, database *sql.DB) sessionIndexSnapshot {
	t.Helper()
	return sessionIndexSnapshot{
		sources:    snapshotSessionRows(t, database, `SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM session_sources ORDER BY source_path`),
		metadata:   snapshotSessionRows(t, database, `SELECT source_path,client,session_id,project,model,parser_version,first_at,last_at FROM session_metadata ORDER BY source_path,client,session_id`),
		documents:  snapshotSessionRows(t, database, `SELECT rowid,source_path,client,session_id,event_at,kind,text FROM session_documents ORDER BY rowid`),
		exclusions: snapshotSessionRows(t, database, `SELECT kind,value FROM session_exclusions ORDER BY kind,value`),
	}
}

type sessionPublicSnapshot struct {
	metadata  []Metadata
	documents []Document
}

func captureSessionPublicState(t *testing.T, database *sql.DB, query string) sessionPublicSnapshot {
	t.Helper()
	metadata, err := List(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := Search(context.Background(), database, query)
	if err != nil {
		t.Fatal(err)
	}
	return sessionPublicSnapshot{metadata: metadata, documents: documents}
}

func TestReplaceDocumentsFirstInsertFailureIsAtomic(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.DB.Exec(`CREATE TRIGGER fail_first_replacement BEFORE INSERT ON session_documents_content WHEN new.c5='freshreplacement' BEGIN SELECT RAISE(ABORT,'synthetic first document insert failure'); END`); err != nil {
		t.Fatal(err)
	}
	beforeTables := captureSessionIndex(t, database.DB)
	beforePublic := captureSessionPublicState(t, database.DB, "freshreplacement")
	document, err := ApprovedDocument("codex", "fresh-atomic", "user_prompt", "freshreplacement")
	if err != nil {
		t.Fatalf("ApprovedDocument: %v", err)
	}

	err = ReplaceDocuments(ctx, database.DB, "codex", "fresh-atomic", []Document{document})
	if err == nil || err.Error() != "constraint failed (1811)" {
		t.Fatalf("ReplaceDocuments error = %v, want injected first insert failure", err)
	}
	afterTables := captureSessionIndex(t, database.DB)
	afterPublic := captureSessionPublicState(t, database.DB, "freshreplacement")
	if len(afterTables.sources) != 0 {
		t.Errorf("ReplaceDocuments first-insert failure left %d synthetic source rows, want 0", len(afterTables.sources))
	}
	if !reflect.DeepEqual(afterTables, beforeTables) {
		t.Fatalf("ReplaceDocuments first-insert failure changed table state: before=%#v after=%#v", beforeTables, afterTables)
	}
	if !reflect.DeepEqual(afterPublic, beforePublic) {
		t.Fatalf("ReplaceDocuments first-insert failure changed Search/List: before=%#v after=%#v", beforePublic, afterPublic)
	}
}

func TestReplaceDocumentsFailureIsAtomic(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	oldOne, err := ApprovedDocument("codex", "atomic", "user_prompt", "oldone")
	if err != nil {
		t.Fatalf("ApprovedDocument oldOne: %v", err)
	}
	oldTwo, err := ApprovedDocument("codex", "atomic", "assistant_final", "oldtwo")
	if err != nil {
		t.Fatalf("ApprovedDocument oldTwo: %v", err)
	}
	if err = ReplaceDocuments(ctx, database.DB, "codex", "atomic", []Document{oldOne, oldTwo}); err != nil {
		t.Fatalf("seed ReplaceDocuments: %v", err)
	}
	if docs, searchErr := Search(ctx, database.DB, "oldone OR oldtwo"); searchErr != nil || len(docs) != 2 {
		t.Fatalf("seed search = %#v, %v", docs, searchErr)
	}
	if _, err = database.DB.Exec(`CREATE TRIGGER fail_second_replacement BEFORE INSERT ON session_documents_content WHEN new.c5='replacementtwo' BEGIN SELECT RAISE(ABORT,'synthetic later document insert failure'); END`); err != nil {
		t.Fatal(err)
	}
	beforeTables := captureSessionIndex(t, database.DB)
	beforePublic := captureSessionPublicState(t, database.DB, "oldone OR oldtwo OR replacementone OR replacementtwo")
	replacementOne, err := ApprovedDocument("codex", "atomic", "user_prompt", "replacementone")
	if err != nil {
		t.Fatalf("ApprovedDocument replacementOne: %v", err)
	}
	replacementTwo, err := ApprovedDocument("codex", "atomic", "assistant_final", "replacementtwo")
	if err != nil {
		t.Fatalf("ApprovedDocument replacementTwo: %v", err)
	}

	err = ReplaceDocuments(ctx, database.DB, "codex", "atomic", []Document{replacementOne, replacementTwo})
	if err == nil || err.Error() != "constraint failed (1811)" {
		t.Fatalf("ReplaceDocuments error = %v, want injected later insert failure", err)
	}
	afterTables := captureSessionIndex(t, database.DB)
	afterPublic := captureSessionPublicState(t, database.DB, "oldone OR oldtwo OR replacementone OR replacementtwo")
	if !reflect.DeepEqual(afterTables, beforeTables) {
		t.Fatalf("ReplaceDocuments failure changed table state: before=%#v after=%#v", beforeTables, afterTables)
	}
	if !reflect.DeepEqual(afterPublic, beforePublic) {
		t.Fatalf("ReplaceDocuments failure changed Search/List: before=%#v after=%#v", beforePublic, afterPublic)
	}
}

type testSessionRecord struct {
	source, client, sessionID, project, text string
	priority                                 int
}

func seedSessionRecord(t *testing.T, database *sql.DB, record testSessionRecord) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,X'',0,0,'',?,?,?)`, record.source, "fixture:"+record.source, record.priority, ParserVersion, "2026-07-23T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,?,?,?,?)`, record.source, record.client, record.sessionID, record.project, "synthetic-model", ParserVersion, "2026-07-23T00:00:00Z", "2026-07-23T00:01:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,?,?,?)`, record.source, record.client, record.sessionID, "", "user_prompt", record.text); err != nil {
		t.Fatal(err)
	}
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func TestExcludeFailureIsAtomic(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project := filepath.Join(t.TempDir(), "blocked-project")
	seedSessionRecord(t, database.DB, testSessionRecord{source: filepath.Join(t.TempDir(), "blocked.jsonl"), client: "codex", sessionID: "blocked", project: project, text: "blockedvisible", priority: 1})
	seedSessionRecord(t, database.DB, testSessionRecord{source: filepath.Join(t.TempDir(), "unrelated.jsonl"), client: "claude", sessionID: "unrelated", project: filepath.Join(t.TempDir(), "other-project"), text: "unrelatedvisible", priority: 1})
	if _, err = database.DB.Exec(`CREATE TRIGGER fail_exclusion_metadata_delete BEFORE DELETE ON session_metadata WHEN old.project=` + sqlQuote(project) + ` BEGIN SELECT RAISE(ABORT,'synthetic metadata delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	beforeTables := captureSessionIndex(t, database.DB)
	beforePublic := captureSessionPublicState(t, database.DB, "blockedvisible OR unrelatedvisible")

	err = Exclude(ctx, database.DB, "project", project)
	if err == nil || err.Error() != "constraint failed: synthetic metadata delete failure (1811)" {
		t.Fatalf("Exclude error = %v, want injected metadata delete failure", err)
	}
	afterTables := captureSessionIndex(t, database.DB)
	afterPublic := captureSessionPublicState(t, database.DB, "blockedvisible OR unrelatedvisible")
	if !reflect.DeepEqual(afterTables, beforeTables) {
		t.Fatalf("Exclude failure changed table state: before=%#v after=%#v", beforeTables, afterTables)
	}
	if !reflect.DeepEqual(afterPublic, beforePublic) {
		t.Fatalf("Exclude failure changed Search/List: before=%#v after=%#v", beforePublic, afterPublic)
	}
}

func TestRebuildFailurePreservesIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	sourceRoot := filepath.Join(home, ".codex", "sessions")
	earlierSource := filepath.Join(sourceRoot, "01-earlier.jsonl")
	laterSource := filepath.Join(sourceRoot, "02-later.jsonl")
	if !(earlierSource < laterSource) {
		t.Fatalf("fixture paths are not deterministically ordered: earlier=%q later=%q", earlierSource, laterSource)
	}
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	earlierData := "{\"type\":\"visible_user_prompt\",\"session_id\":\"rebuild-earlier\",\"payload\":{\"text\":\"earlierrebuild\"}}\n"
	laterData := "{\"type\":\"visible_user_prompt\",\"session_id\":\"rebuild-later\",\"payload\":{\"text\":\"laterrebuild\"}}\n"
	if err := os.WriteFile(earlierSource, []byte(earlierData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(laterSource, []byte(laterData), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if result, scanErr := Scan(ctx, database.DB, home); scanErr != nil || result.Sources != 2 || result.Documents != 2 {
		t.Fatalf("seed Scan = %#v, %v", result, scanErr)
	}
	if err = Exclude(ctx, database.DB, "client", "unrelated-client"); err != nil {
		t.Fatalf("seed unrelated exclusion: %v", err)
	}
	trigger := `CREATE TRIGGER fail_later_rebuild_metadata_insert
		BEFORE INSERT ON session_metadata
		WHEN new.source_path=` + sqlQuote(laterSource) + `
			AND EXISTS(SELECT 1 FROM session_metadata WHERE source_path=` + sqlQuote(earlierSource) + `)
		BEGIN SELECT RAISE(ABORT,'synthetic later rebuild insert failure'); END`
	if _, err = database.DB.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	beforeTables := captureSessionIndex(t, database.DB)
	beforePublic := captureSessionPublicState(t, database.DB, "earlierrebuild OR laterrebuild")

	_, err = Rebuild(ctx, database.DB, home)
	if err == nil || err.Error() != "constraint failed: synthetic later rebuild insert failure (1811)" {
		t.Fatalf("Rebuild error = %v, want later-source failure after earlier metadata insertion", err)
	}
	afterTables := captureSessionIndex(t, database.DB)
	afterPublic := captureSessionPublicState(t, database.DB, "earlierrebuild OR laterrebuild")
	if !reflect.DeepEqual(afterTables, beforeTables) {
		t.Fatalf("Rebuild failure changed table state: before=%#v after=%#v", beforeTables, afterTables)
	}
	if !reflect.DeepEqual(afterPublic, beforePublic) {
		t.Fatalf("Rebuild failure changed Search/List: before=%#v after=%#v", beforePublic, afterPublic)
	}
}

func storedSessionTexts(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`SELECT text FROM session_documents ORDER BY text`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return values
}

func TestExcludeExactBoundaries(t *testing.T) {
	query := "targetvisible OR fallbackvisible OR codexother OR claudeshared OR claudeother"
	for _, test := range []struct {
		name         string
		kind         string
		value        func(project, targetSource string) string
		wantTexts    []string
		wantSessions []string
	}{
		{name: "project", kind: "project", value: func(project, _ string) string { return project }, wantTexts: []string{"claudeother", "claudeshared", "codexother", "fallbackvisible"}, wantSessions: []string{"claude/claude-other", "claude/shared", "codex/other", "codex/shared"}},
		{name: "path", kind: "path", value: func(_, targetSource string) string { return targetSource }, wantTexts: []string{"claudeother", "claudeshared", "codexother", "fallbackvisible"}, wantSessions: []string{"claude/claude-other", "claude/shared", "codex/other", "codex/shared"}},
		{name: "session", kind: "session", value: func(_, _ string) string { return "shared" }, wantTexts: []string{"claudeother", "codexother"}, wantSessions: []string{"claude/claude-other", "codex/other"}},
		{name: "client", kind: "client", value: func(_, _ string) string { return "codex" }, wantTexts: []string{"claudeother", "claudeshared"}, wantSessions: []string{"claude/claude-other", "claude/shared"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			project := filepath.Join(root, "project-target")
			targetSource := filepath.Join(root, "target.jsonl")
			for _, record := range []testSessionRecord{
				{source: targetSource, client: "codex", sessionID: "shared", project: project, text: "targetvisible", priority: 2},
				{source: filepath.Join(root, "fallback.jsonl"), client: "codex", sessionID: "shared", project: filepath.Join(root, "other-project"), text: "fallbackvisible", priority: 1},
				{source: filepath.Join(root, "codex-other.jsonl"), client: "codex", sessionID: "other", project: filepath.Join(root, "other-project"), text: "codexother", priority: 1},
				{source: filepath.Join(root, "claude-shared.jsonl"), client: "claude", sessionID: "shared", project: filepath.Join(root, "other-project"), text: "claudeshared", priority: 1},
				{source: filepath.Join(root, "claude-other.jsonl"), client: "claude", sessionID: "claude-other", project: filepath.Join(root, "other-project"), text: "claudeother", priority: 1},
			} {
				seedSessionRecord(t, database.DB, record)
			}
			beforeSources := captureSessionIndex(t, database.DB).sources
			value := test.value(project, targetSource)
			if err = Exclude(ctx, database.DB, test.kind, value); err != nil {
				t.Fatal(err)
			}
			after := captureSessionIndex(t, database.DB)
			if !reflect.DeepEqual(after.sources, beforeSources) {
				t.Fatalf("Exclude(%s) changed source controls: before=%#v after=%#v", test.kind, beforeSources, after.sources)
			}
			if len(after.metadata) != len(test.wantTexts) || len(after.documents) != len(test.wantTexts) {
				t.Fatalf("Exclude(%s) rows metadata=%d documents=%d, want %d each", test.kind, len(after.metadata), len(after.documents), len(test.wantTexts))
			}
			if got := storedSessionTexts(t, database.DB); !reflect.DeepEqual(got, test.wantTexts) {
				t.Fatalf("Exclude(%s) stored texts = %#v, want %#v", test.kind, got, test.wantTexts)
			}
			var exclusionKind, exclusionValue string
			if err = database.DB.QueryRow(`SELECT kind,value FROM session_exclusions`).Scan(&exclusionKind, &exclusionValue); err != nil || exclusionKind != test.kind || exclusionValue != filepath.Clean(value) {
				t.Fatalf("Exclude(%s) control = (%q,%q), %v; want (%q,%q)", test.kind, exclusionKind, exclusionValue, err, test.kind, filepath.Clean(value))
			}
			listed, err := List(ctx, database.DB)
			if err != nil {
				t.Fatal(err)
			}
			gotSessions := make([]string, 0, len(listed))
			for _, item := range listed {
				gotSessions = append(gotSessions, item.Client+"/"+item.SessionID)
			}
			sort.Strings(gotSessions)
			if !reflect.DeepEqual(gotSessions, test.wantSessions) {
				t.Fatalf("Exclude(%s) List sessions = %#v, want %#v", test.kind, gotSessions, test.wantSessions)
			}
			found, err := Search(ctx, database.DB, query)
			if err != nil {
				t.Fatal(err)
			}
			gotTexts := make([]string, 0, len(found))
			for _, document := range found {
				gotTexts = append(gotTexts, document.Text)
			}
			sort.Strings(gotTexts)
			if !reflect.DeepEqual(gotTexts, test.wantTexts) {
				t.Fatalf("Exclude(%s) Search texts = %#v, want %#v", test.kind, gotTexts, test.wantTexts)
			}
		})
	}
}

func TestPaginateAndSummarizeActivity(t *testing.T) {
	values := []int{1, 2, 3}
	page, pagination, err := Paginate(values, 2, 2, false)
	if err != nil || len(page) != 1 || page[0] != 3 || pagination.Total != 3 || pagination.HasMore {
		t.Fatalf("Paginate = %#v %#v %v", page, pagination, err)
	}
	first, second := int64(10), int64(30)
	summary := SummarizeActivity([]activity.Detail{{Tool: "read", Status: "completed", DurationMS: &first}, {Tool: "exec", Status: "failed", DurationMS: &second}, {Tool: "read", Status: "started"}})
	if summary.Total != 3 || summary.Completed != 1 || summary.Failed != 1 || summary.Incomplete != 1 || summary.TotalDurationMS != 40 || summary.AverageDurationMS == nil || *summary.AverageDurationMS != 20 || len(summary.ByTool) != 2 || summary.ByTool[0].Tool != "exec" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestPaginateRejectsUnsafeLimitsAndOverflowingPages(t *testing.T) {
	if _, _, err := Paginate([]int{1}, 1, MaxPageLimit+1, false); err == nil {
		t.Fatal("accepted excessive limit")
	}
	if page, metadata, err := Paginate([]int{1}, int(^uint(0)>>1), 1, false); err != nil || len(page) != 0 || metadata.Total != 1 {
		t.Fatalf("overflow page = %#v %#v %v", page, metadata, err)
	}
}
