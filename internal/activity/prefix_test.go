package activity

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClaudeActivityPrefix(t *testing.T) {
	first := `{"type":"assistant","timestamp":"2026-01-01T01:00:00Z","sessionId":"s","message":{"content":[{"type":"tool_use","id":"first","name":"Read","input":{}}]}}` + "\n"
	second := `{"type":"assistant","timestamp":"2026-01-01T02:00:00Z","sessionId":"s","message":{"content":[{"type":"tool_use","id":"second","name":"Edit","input":{}}]}}` + "\n"
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte(first+second), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Seek(5, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	full, err := ReadDetails(path, "claude", "s")
	if err != nil || len(full) != 2 {
		t.Fatalf("legacy details=%v err=%v", full, err)
	}
	for _, tc := range []struct {
		name       string
		end, count int
	}{{"empty", 0, 0}, {"mid-first", 20, 0}, {"first-line", len(first), 1}, {"mid-second", len(first) + 20, 1}, {"valid-json-without-newline", len(first+second) - 1, 1}, {"all", len(first + second), 2}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadDetailsPrefix(file, int64(tc.end), "claude", "s")
			if err != nil || len(got) != tc.count || tc.count > 0 && !reflect.DeepEqual(got, full[:tc.count]) {
				t.Fatalf("details %v err=%v", got, err)
			}
			page, err := ReadDetailsPagePrefix(file, int64(tc.end), "claude", "s", 1, 1, false)
			if err != nil || page.Total != tc.count || page.Shown != min(1, tc.count) || page.Summary.Total != tc.count || page.HasMore != (tc.count > 1) {
				t.Fatalf("page %+v err=%v", page, err)
			}
			if page.Shown > 0 && !reflect.DeepEqual(page.Details, full[:1]) {
				t.Fatalf("incorrect page order: %+v", page.Details)
			}
		})
	}
	for _, end := range []int64{-1, int64(len(first+second) + 1)} {
		d, err := ReadDetailsPrefix(file, end, "claude", "s")
		if err == nil || d != nil {
			t.Fatalf("invalid end %d: %v %v", end, d, err)
		}
		page, e := ReadDetailsPagePrefix(file, end, "claude", "s", 1, 2, false)
		if e == nil || !reflect.DeepEqual(page, Page{}) {
			t.Fatalf("invalid end %d: %+v %v", end, page, e)
		}
		if end > 0 && (!errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(e, io.ErrUnexpectedEOF)) {
			t.Fatalf("out of bounds: %v %v", err, e)
		}
	}
	if pos, err := file.Seek(0, io.SeekCurrent); err != nil || pos != 5 {
		t.Fatalf("changed caller offset to %d: %v", pos, err)
	}
}

func TestActivityPrefixTruncationAfterOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte("{}\n{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if scanner, e := activityPrefixScanner(file, 7); scanner != nil || !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatalf("out-of-file prefix admitted for scanning: %v %v", scanner, e)
	}
	scanner, err := activityPrefixScanner(file, 6)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, 3); err != nil {
		t.Fatal(err)
	}
	for scanner.Scan() {
	}
	if !errors.Is(scanner.Err(), io.ErrUnexpectedEOF) {
		t.Fatalf("shortened file looked complete: %v", scanner.Err())
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if d, e := ReadDetailsPrefix(file, 0, "claude", "s"); d != nil || !errors.Is(e, os.ErrClosed) {
		t.Fatalf("closed descriptor details=%v err=%v", d, e)
	}
	if p, e := ReadDetailsPagePrefix(file, 0, "claude", "s", 1, 1, false); !reflect.DeepEqual(p, Page{}) || !errors.Is(e, os.ErrClosed) {
		t.Fatalf("closed descriptor page=%v err=%v", p, e)
	}
	if p, e := ReadDetailsPagePrefix(file, 0, "claude", "s", 0, 1, false); !reflect.DeepEqual(p, Page{}) || e == nil || errors.Is(e, os.ErrClosed) {
		t.Fatalf("invalid window attempted descriptor access: %v %v", p, e)
	}
}
