package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestStoredOnlyCoverageWarningsAndNoMigration(t *testing.T) {
	for _, command := range []string{"stats", "summary"} {
		for _, state := range []string{"unknown", "partial", "update_pending", "complete"} {
			t.Run(command+"/"+state, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				root := filepath.Join(home, ".agentdeck")
				ctx := context.Background()
				core, err := store.Open(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				defer core.Close()
				if state != "unknown" {
					generation, err := core.DerivedSnapshotGeneration(ctx)
					if err != nil {
						t.Fatal(err)
					}
					sources := []store.InventorySource{}
					if state != "complete" {
						sources = append(sources, store.InventorySource{Path: "not-read", Identity: "fixture", End: 12})
					}
					if err = core.AcceptInventory(ctx, "fixture", generation.Epoch, sources, time.Now()); err != nil {
						t.Fatal(err)
					}
					if state == "complete" {
						err = core.FinishInventory(ctx, "fixture", "complete")
					}
					if state == "partial" {
						err = core.FinishInventory(ctx, "fixture", "paused")
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				// A stored-only query must not execute even the pending migration.
				if _, err = core.DB.ExecContext(ctx, `UPDATE schema_metadata SET version=30`); err != nil {
					t.Fatal(err)
				}
				before, err := core.ReadSourceCoverage(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var stdout, stderr bytes.Buffer
				status := execute([]string{"--state-dir", root, "--format", "json", "usage", command, "--no-scan"}, bytes.NewReader(nil), &stdout, &stderr)
				if status != 0 || stderr.Len() != 0 {
					t.Fatalf("stored query failed: %d %s", status, stderr.String())
				}
				var envelope struct {
					Schema   int      `json:"schema_version"`
					Warnings []string `json:"warnings"`
					Partial  bool     `json:"partial"`
				}
				if err = json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				code := sourceCoverageWarning(state)
				if envelope.Schema != 1 || envelope.Partial != (state != "complete") || (code != "" && !slices.Contains(envelope.Warnings, code)) {
					t.Fatalf("coverage envelope mismatch: %+v", envelope)
				}
				version, err := core.SchemaVersion(ctx)
				if err != nil || version != 30 {
					t.Fatalf("stored-only query migrated schema: %d %v", version, err)
				}
				after, err := core.ReadSourceCoverage(ctx)
				if err != nil || after != before {
					t.Fatalf("stored query changed coverage: %+v %v", after, err)
				}
				if _, err = os.Stat(filepath.Join(root, "scan.lock")); !os.IsNotExist(err) {
					t.Fatal("stored query launched scanning")
				}
			})
		}
	}
}

func TestStoredOnlyCoverageReadFailureIsNotSuccessfulPartialData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".agentdeck")
	core, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	if _, err = core.DB.Exec(`DROP TABLE inventory_runs`); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := execute([]string{"--state-dir", root, "--format", "json", "usage", "stats", "--no-scan"}, bytes.NewReader(nil), &stdout, &stderr)
	if status != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("coverage SQL failure fabricated success: status=%d stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}
}

func TestDesktopBootstrapUsesSeparateVersionedStream(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".agentdeck")
	var stdout, stderr bytes.Buffer
	status := execute([]string{"--state-dir", root, "--format", "ndjson", "desktop", "bootstrap"}, bytes.NewReader(nil), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("bootstrap failed: %d %s", status, stderr.String())
	}
	decoder := json.NewDecoder(&stdout)
	publication, terminal := false, false
	for decoder.More() {
		var event struct {
			Schema  int    `json:"schema_version"`
			Command string `json:"command"`
			Type    string `json:"type"`
		}
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.Schema != 2 || event.Command != "desktop.bootstrap" {
			t.Fatalf("bootstrap mixed with v1 scan stream: %+v", event)
		}
		publication = publication || event.Type == "publication"
		terminal = terminal || event.Type == "terminal"
	}
	if !publication || !terminal {
		t.Fatal("bootstrap returned without publication and terminal")
	}
}
