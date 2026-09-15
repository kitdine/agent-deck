package usage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSignalsBatchMatchesIndividualReports(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s, db, home := newSignalService(t, root)
	seedSignalCostPrices(t, db)
	writeSource(t, filepath.Join(home, ".claude", "projects", "batch.jsonl"),
		userLine("s", "2026-08-26T01:00:00Z", "implement feature"),
		assistantLine("s", "2026-08-26T01:00:01Z", "a", pricedFixtureModel, 200000, "Edit", filepath.Join(root, "a.go")),
		userLine("s", "2026-08-27T01:00:00Z", "explain"),
		assistantLine("s", "2026-08-27T01:00:01Z", "b", unpricedFixtureModel, 100000, "", ""),
	)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	var options []SignalOptions
	for _, start := range []time.Time{from, from.Add(24 * time.Hour)} {
		for _, client := range []string{"", "codex", "claude"} {
			options = append(options, SignalOptions{Period: "test", From: start, To: to, Client: client, IncludeSub: true})
		}
	}
	options = append(options, SignalOptions{From: from, To: to, Activity: "coding", IncludeSub: true}, SignalOptions{From: from, To: to, Kinds: []string{"tooling"}})
	verify := func() {
		t.Helper()
		got, err := s.SignalsBatch(ctx, options)
		if err != nil {
			t.Fatal(err)
		}
		for i, o := range options {
			want, err := s.Signals(ctx, o)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[i], want) {
				t.Fatalf("option%+v got=%+v want=%+v", o, got[i], want)
			}
		}
	}
	verify()
	if _, err := db.Exec(ctx, "UPDATE usage_events SET turn_index=NULL WHERE event_id='a'"); err != nil {
		t.Fatal(err)
	}
	verify()
	if _, err := db.Exec(ctx, "DELETE FROM usage_events"); err != nil {
		t.Fatal(err)
	}
	verify()
}

func TestSignalsBatchEmptyRangeDoesNotPriceOtherClients(t *testing.T) {
	ctx := context.Background()
	s, db, _ := newSignalService(t, t.TempDir())
	seedSignalCostPrices(t, db)
	if _, err := db.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('c','claude','s','c','2026-08-27T00:00:01Z','claude-priced',1,'synthetic',0); UPDATE model_prices SET prices_json='invalid'`); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	o := SignalOptions{From: from, To: to, Client: "codex", Kinds: []string{"activity"}}
	want, err := s.Signals(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.SignalsBatch(ctx, []SignalOptions{o})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []SignalReport{want}) {
		t.Fatal("batch changed empty-scope/error behavior")
	}
}
