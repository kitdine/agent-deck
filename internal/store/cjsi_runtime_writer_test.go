package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCJSIRuntimeResolveAndRefuse(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	m, err := ResolveCJSIRuntimeRoot(t.Context(), f.draft.admission.rootPath)
	if err != nil || m == nil {
		t.Fatalf("managed resolution: %v", err)
	}

	before := cjsiSlotCoreImage(t, f.db)
	// Losing the physical fence cannot select the legacy producer.
	fence := filepath.Join(m.path, cjsiLegacyIndexName)
	if err := os.Rename(fence, fence+".off"); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveCJSIRuntimeRoot(t.Context(), m.path); err == nil || got != nil {
		t.Fatalf("displaced fence: %+v %v", got, err)
	}
	if err := os.Rename(fence+".off", fence); err != nil {
		t.Fatal(err)
	}
	if len(before["cjsi_publication_attempts"]) != 0 {
		t.Fatal("unexpected attempt")
	}
	legacy := t.TempDir()
	got, err := ResolveCJSIRuntimeRoot(t.Context(), legacy)
	if err != nil || got != nil {
		t.Fatalf("new legacy root: %+v %v", got, err)
	}
	if entries, err := os.ReadDir(legacy); err != nil || len(entries) != 0 {
		t.Fatal("route discovery bootstrapped the root")
	}
	path := filepath.Join(legacy, "agentdeck.sqlite3")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveCJSIRuntimeRoot(t.Context(), legacy); err != nil || got != nil {
		t.Fatalf("empty legacy core: %+v %v", got, err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() != 0 {
		t.Fatal("probe initialized the empty core")
	}

}
