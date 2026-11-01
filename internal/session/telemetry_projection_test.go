package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func openProjectionIndex(t *testing.T) *store.Store {
	t.Helper()
	index, err := store.OpenSessions(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { index.Close() })
	return index
}

var background = TelemetrySession{Client: "codex", SessionID: "bg-1", Model: "gpt-5.6-terra", FirstAt: "2026-10-29T12:00:00Z", LastAt: "2026-10-29T12:00:05Z"}

func TestTelemetryProjectionPreservesCompletedFileCoverageAndNoopRevision(t *testing.T) {
	ctx := context.Background()
	index := openProjectionIndex(t)
	epoch, err := index.SessionIndexEpoch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.AcceptInventory(ctx, "captured-files", epoch, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := index.FinishInventory(ctx, "captured-files", "complete"); err != nil {
		t.Fatal(err)
	}
	for _, projected := range [][]TelemetrySession{{background}, {background}, nil} {
		if err := ProjectTelemetrySessions(ctx, index.DB, projected); err != nil {
			t.Fatal(err)
		}
		coverage, err := index.ReadSourceCoverage(ctx)
		if err != nil || coverage.State != "complete" || coverage.InventoryID != "captured-files" {
			t.Fatalf("synthetic projection invalidated file coverage: %+v %v", coverage, err)
		}
		var before, after int64
		if err := index.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err := ProjectTelemetrySessions(ctx, index.DB, projected); err != nil {
			t.Fatal(err)
		}
		if err := index.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("unchanged projection advanced revision: %d -> %d", before, after)
		}
	}
	if _, err := index.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES('real.jsonl','real',0,0,0,'',1,?,'t')`, ParserVersion); err != nil {
		t.Fatal(err)
	}
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background}); err != nil {
		t.Fatal(err)
	}
	coverage, err := index.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "update_pending" {
		t.Fatalf("projection hid a real file change: %+v %v", coverage, err)
	}
}

func TestProjectedBackgroundSessionIsReadableThroughExistingReaders(t *testing.T) {
	ctx := context.Background()
	index := openProjectionIndex(t)
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background}); err != nil {
		t.Fatal(err)
	}
	listed, err := List(ctx, index.DB)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].SessionID != "bg-1" || listed[0].SourcePath != "otel:codex/bg-1" || listed[0].Project != "" || listed[0].Model != "gpt-5.6-terra" {
		t.Fatalf("list = %+v", listed)
	}
	metadata, err := ShowMetadata(ctx, index.DB, "codex", "bg-1")
	if err != nil || metadata.SourcePath != "otel:codex/bg-1" || metadata.FirstAt != background.FirstAt {
		t.Fatalf("show metadata = %+v %v", metadata, err)
	}
	documents, pagination, err := DocumentsPage(ctx, index.DB, metadata, 1, 20, false)
	if err != nil || len(documents) != 0 || pagination.Total != 0 {
		t.Fatalf("documents = %v %+v %v", documents, pagination, err)
	}
	shown, err := ShowWithActivity(ctx, index.DB, "codex", "bg-1", true)
	if err != nil || len(shown.Documents) != 0 || len(shown.Activity) != 0 {
		t.Fatalf("show with activity read a file for a projected source: %+v %v", shown, err)
	}
	if hits, err := Search(ctx, index.DB, "terra"); err != nil || len(hits) != 0 {
		t.Fatalf("search matched a session without text: %v %v", hits, err)
	}
	// Idempotent: projecting again changes nothing.
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background}); err != nil {
		t.Fatal(err)
	}
	if again, _ := List(ctx, index.DB); len(again) != 1 {
		t.Fatalf("second projection duplicated: %+v", again)
	}
}

func TestFileInventoryNeverRemovesProjectedSources(t *testing.T) {
	ctx := context.Background()
	index := openProjectionIndex(t)
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background}); err != nil {
		t.Fatal(err)
	}
	// A full-set reconciliation whose inventory has no files at all.
	if err := removeMissingSources(ctx, index.DB, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	// A full discovery scan of a HOME without any session files.
	if _, err := ScanWithOptions(ctx, index.DB, t.TempDir(), ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	if listed, _ := List(ctx, index.DB); len(listed) != 1 || listed[0].SourcePath != "otel:codex/bg-1" {
		t.Fatalf("inventory removed the projected source: %+v", listed)
	}
}

func TestProjectionHonorsExclusionsAndRemovesRetractedSessions(t *testing.T) {
	ctx := context.Background()
	index := openProjectionIndex(t)
	other := TelemetrySession{Client: "codex", SessionID: "bg-2", Model: "gpt-5.6-luna", FirstAt: "2026-10-29T13:00:00Z", LastAt: "2026-10-29T13:00:00Z"}
	if err := Exclude(ctx, index.DB, "session", "bg-2"); err != nil {
		t.Fatal(err)
	}
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background, other}); err != nil {
		t.Fatal(err)
	}
	if listed, _ := List(ctx, index.DB); len(listed) != 1 || listed[0].SessionID != "bg-1" {
		t.Fatalf("excluded session projected: %+v", listed)
	}
	// Retraction in core: the projection removes the source and its metadata.
	if err := ProjectTelemetrySessions(ctx, index.DB, nil); err != nil {
		t.Fatal(err)
	}
	var sources, metadata int
	_ = index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE source_path LIKE 'otel:%'`).Scan(&sources)
	_ = index.DB.QueryRow(`SELECT count(*) FROM session_metadata WHERE source_path LIKE 'otel:%'`).Scan(&metadata)
	if sources != 0 || metadata != 0 {
		t.Fatalf("retracted session still projected: sources=%d metadata=%d", sources, metadata)
	}
}

func TestLateFileSourceWinsTheVisibleRow(t *testing.T) {
	ctx := context.Background()
	index := openProjectionIndex(t)
	if err := ProjectTelemetrySessions(ctx, index.DB, []TelemetrySession{background}); err != nil {
		t.Fatal(err)
	}
	file := "/home/.codex/sessions/rollout-bg-1.jsonl"
	if _, err := index.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,'',1,?,'t')`, file, file, ParserVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := index.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,'codex','bg-1','/work/app','gpt-6-astra',?,'a','b')`, file, ParserVersion); err != nil {
		t.Fatal(err)
	}
	listed, _ := List(ctx, index.DB)
	if len(listed) != 1 || listed[0].SourcePath != file || listed[0].Project != "/work/app" {
		t.Fatalf("late JSONL did not win: %+v", listed)
	}
}
