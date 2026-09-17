package hookrefusal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRecordLifecycle(t *testing.T) {
	root := t.TempDir()
	if _, ok := Read(root); ok {
		t.Fatal("missing record accepted")
	}
	if err := Write(root, 99, 23); err != nil {
		t.Fatal(err)
	}
	first, ok := Read(root)
	if !ok || first.Count != 1 || first.Stored != 99 || first.Supported != 23 {
		t.Fatalf("record = %+v, %v", first, ok)
	}
	if err := Write(root, 99, 23); err != nil {
		t.Fatal(err)
	}
	next, ok := Live(root, 23)
	if !ok || next.Count != 2 || !next.FirstAt.Equal(first.FirstAt) || next.LastAt.Before(first.LastAt) {
		t.Fatalf("repeat = %+v", next)
	}
	path := filepath.Join(root, Filename)
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 || len(before) > maxRecordBytes {
		t.Fatal("record not bounded/private")
	}
	var keys map[string]any
	if err := json.Unmarshal(before, &keys); err != nil || len(keys) != 7 {
		t.Fatalf("record keys = %v, %v", keys, err)
	}
	if _, live := Live(root, 99); live {
		t.Fatal("upgrade did not suppress record")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("reader changed record")
	}
	if err := Clear(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("clear failed")
	}
}

func TestRecordInvalidAndFailedReplacement(t *testing.T) {
	for _, data := range []string{"{", "{}", strings.Repeat("x", maxRecordBytes+1), `{"schema_version":2}`, `null`} {
		root := t.TempDir()
		path := filepath.Join(root, Filename)
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, ok := Read(root); ok {
			t.Fatalf("accepted %q", data)
		}
		if err := Write(root, 99, 23); err != nil {
			t.Fatal(err)
		}
		if r, ok := Read(root); !ok || r.Count != 1 {
			t.Fatal("did not replace corrupt record")
		}
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, Filename), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, 99, 23); err == nil {
		t.Fatal("rename failure not returned")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
	if err := Write(filepath.Join(root, "missing"), 99, 23); err == nil {
		t.Fatal("missing root accepted")
	}
}

func TestConcurrentReplacementRemainsReadable(t *testing.T) {
	root := t.TempDir()
	if err := Write(root, 99, 23); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Write(root, 99, 23); err != nil {
				t.Error(err)
			}
			if r, ok := Read(root); !ok || r.Count < 1 || r.Count > 21 {
				t.Errorf("torn record: %+v, %v", r, ok)
			}
		}()
	}
	wg.Wait()
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
}
