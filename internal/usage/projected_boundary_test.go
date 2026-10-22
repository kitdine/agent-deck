package usage

import (
	"context"
	"github.com/kitdine/agent-deck/internal/store"
	"reflect"
	"testing"
	"time"
)

func TestProjectedEmptyModelAndFuturePriceBoundary(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, "")
	if err := s.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES ('empty','codex','s','a','2026-11-01T01:00:00Z','',17,'f',0),('priced','codex','s','b','2026-11-01T02:00:00Z','gpt-5',23,'f',1); UPDATE model_prices SET effective_from='2026-11-02T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	for _, now := range []time.Time{time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)} {
		full, err := s.Presentation(ctx, now, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		projected, err := s.ProjectedPresentation(ctx, now, time.UTC)
		if err != nil || !reflect.DeepEqual(full, projected) {
			t.Fatalf("future boundary or rollback changed pricing: %v", err)
		}
	}
}

func TestProjectedPartitionDoesNotPriceErrorsOutsideLocalWindow(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, "")
	if err := s.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at,exact) VALUES(1,'codex','relay','invalid','2026-11-01T00:00:00Z',1);
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset,run_id) VALUES('good','codex','s','a','2026-11-01T12:00:00Z','gpt-5',17,'f',0,NULL),('outside','codex','s','b','2026-11-02T12:00:00Z','gpt-5',23,'f',1,1)`); err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		full, err := s.Presentation(ctx, now, location)
		if err != nil {
			t.Fatal(err)
		}
		projected, err := s.ProjectedPresentation(ctx, now, location)
		if err != nil || !reflect.DeepEqual(full, projected) {
			t.Fatalf("unselected UTC-partition tail changed local window: %v", err)
		}
	}
	// Advancing the window must expose the same invalid event in both paths.
	next := now.AddDate(0, 0, 1)
	_, fullErr := s.Presentation(ctx, next, location)
	_, projectedErr := s.ProjectedPresentation(ctx, next, location)
	if fullErr == nil || projectedErr == nil || fullErr.Error() != projectedErr.Error() {
		t.Fatalf("selected pricing error was hidden or changed: full=%v projected=%v", fullErr, projectedErr)
	}
}
