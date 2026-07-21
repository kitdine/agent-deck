package session

import (
	"context"
	"fmt"
	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
	"os"
	"path/filepath"
	"testing"
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
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if result, err := Scan(context.Background(), s.DB, home); err != nil {
		t.Fatal(err)
	} else if result.Removed != 1 {
		t.Fatalf("last source removal removed %d logical documents, want 1", result.Removed)
	}
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
	if err = os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if result, scanErr := Scan(context.Background(), database.DB, home); scanErr != nil || result.Documents != 0 || result.Removed != 2 {
		t.Fatalf("final duplicate removal = %#v, %v", result, scanErr)
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
	if err != nil || shown.SourcePath != archive {
		t.Fatalf("move show=%+v err=%v", shown, err)
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
