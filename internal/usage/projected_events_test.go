package usage

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestProjectedPresentationMatchesFullOracleThroughCorrections(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, "")
	if err := s.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	exec := func(q string) {
		t.Helper()
		if _, err := db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES ('codex','shared','2026-10-31T23:30:00Z','2026-11-01T06:30:00Z'),('claude','shared','2026-11-01T07:30:00Z','2026-11-01T07:30:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES
('a','codex','shared','a','2026-10-31T23:30:00Z','gpt-5',11,1,'f',0),
('b','codex','shared','b','2026-11-01T04:30:00Z','gpt-5',17,2,'f',1),
('c','codex','shared','c','2026-11-01T06:30:00Z','gpt-5',23,3,'f',2),
('d','claude','shared','d','2026-11-01T07:30:00Z','unknown',31,4,'g',0)`)
	check := func(name string, wantReads int64) PresentationReport {
		t.Helper()
		s.Profile = nil
		full, err := s.Presentation(ctx, now, location)
		if err != nil {
			t.Fatal(err)
		}
		s.Profile = &ScanProfile{}
		projected, err := s.ProjectedPresentation(ctx, now, location)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(projected, full) {
			t.Fatalf("%s differs from full oracle", name)
		}
		if got := s.Profile.Report()["events_range"].Calls; got != wantReads {
			t.Fatalf("%s read %d partitions, want %d", name, got, wantReads)
		}
		return projected
	}
	check("initial", 3)
	report := check("warm", 0)
	if report.Scopes[0].Periods.Items[1].Totals.Sessions != 2 {
		t.Fatal("All double counted a cross-day session or merged clients")
	}
	exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES ('late','codex','shared','late','2026-10-31T22:30:00Z','gpt-5',7,'f',3)`)
	check("late", 1)
	exec(`UPDATE usage_events SET event_at='2026-11-01T05:30:00Z',model='unknown-new',input_tokens=101 WHERE event_key='a'`)
	check("cross-day model rewrite", 2)
	exec(`DELETE FROM usage_events WHERE source_path='g'`)
	check("source removal", 1)
	exec(`UPDATE usage_sessions SET started_at='2026-10-31T21:00:00Z' WHERE client='codex' AND session_id='shared'`)
	check("session span", 3)
	exec(`UPDATE model_prices SET prices_json=prices_json WHERE model='gpt-5'`)
	check("pricing revision", 4)
	exec(`UPDATE usage_derived_partitions SET payload=x'00' WHERE client='codex' AND model='unknown-new'`)
	check("corrupt cache", 1)
	exec(`UPDATE usage_derived_partitions SET version='old-algorithm' WHERE client='codex' AND model='unknown-new'`)
	check("algorithm revision", 1)
	if err := db.MintDerivedSnapshotEpoch(ctx); err != nil {
		t.Fatal(err)
	}
	check("restore epoch", 4)
	ro, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	readOnly := New(ro, "")
	full, err := s.Presentation(ctx, now, location)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := readOnly.ProjectedPresentation(ctx, now, location)
	if err != nil || !reflect.DeepEqual(actual, full) {
		t.Fatal("read-only fallback changed the oracle")
	}
}
