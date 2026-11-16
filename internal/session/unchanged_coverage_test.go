package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestUnchangedCoveragePreparedBatchRetainsSourceValidationAndAtomicity(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	directory := filepath.Join(home, ".codex", "sessions")
	if err = os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two"} {
		text := `{"type":"visible_user_prompt","session_id":"` + name + `","timestamp":"2026-10-20T01:00:00Z","payload":{"text":"hello"}}` + "\n"
		if err = os.WriteFile(filepath.Join(directory, name+".jsonl"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = Scan(ctx, db.DB, home); err != nil {
		t.Fatal(err)
	}
	sources, err := sessionSources(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("sources=%d", len(sources))
	}
	var epoch int64
	if err = db.DB.QueryRow(`SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	var inventory []store.InventorySource
	for _, src := range sources {
		inventory = append(inventory, store.InventorySource{Path: src.path, Identity: src.identity, End: src.size})
	}
	for _, id := range []string{"good", "mismatch"} {
		if err = db.AcceptInventory(ctx, id, epoch, inventory, time.Now()); err != nil {
			t.Fatal(err)
		}
		batch := append([]source(nil), sources...)
		if id == "mismatch" {
			batch[1].size++
		}
		err = commitUnchangedSessionCoverageBatch(ctx, db.DB, batch, id)
		if id == "good" && err != nil {
			t.Fatal(err)
		}
		if id == "mismatch" && !errors.Is(err, ingest.ErrSourceChanged) {
			t.Fatalf("mismatch=%v", err)
		}
		coverage, readErr := db.ReadSourceCoverage(ctx)
		want := 2
		if id == "mismatch" {
			want = 0
		}
		if readErr != nil || coverage.Committed != want {
			t.Fatalf("%s coverage=%+v %v", id, coverage, readErr)
		}
	}
}
