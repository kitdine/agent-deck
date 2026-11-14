package usage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// scanSessions indexes the named claude sessions ("a", "b") with distinct
// tool shapes, so every family differs between them. Edited files live under
// a fixed directory so reports of separate stores are comparable.
func scanSessions(t *testing.T, names ...string) (*Service, time.Time, time.Time) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	sources := map[string][]string{
		"a": {claudeUser("a", "2026-08-27T00:00:00Z", "fix the crash in the parser"),
			claudeAssistant("a", "a1", "2026-08-27T00:00:05Z", claudeEdit("ta1", "/work/a.go")+","+claudeBash("ta2", "go test ./...")+","+claudeEdit("ta3", "/work/a.go"))},
		"b": {claudeUser("b", "2026-08-27T01:00:00Z", "add a feature"),
			claudeAssistant("b", "b1", "2026-08-27T01:00:09Z", claudeEdit("tb1", "/work/b.go")),
			claudeUser("b", "2026-08-27T01:05:00Z", "now write the docs"),
			claudeAssistant("b", "b2", "2026-08-27T01:05:07Z", claudeBash("tb2", "ls"))},
	}
	for _, name := range names {
		writeSource(t, filepath.Join(home, ".claude", "projects", "p", name+".jsonl"), sources[name]...)
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	return service, time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
}

// The oracle is a separate store that indexed only the selected session: a
// filtered report over both sessions must equal its unfiltered report.
func TestSessionFilterEqualsAStoreOfOnlyTheSelectedSessions(t *testing.T) {
	ctx := context.Background()
	service, from, to := scanSessions(t, "a", "b")
	keyA, keyB := SessionKey{"claude", "a"}, SessionKey{"claude", "b"}
	oracles := map[string]SignalReport{}
	for _, name := range []string{"a", "b"} {
		only, _, _ := scanSessions(t, name)
		report, err := only.Signals(ctx, SignalOptions{From: from, To: to, IncludeSub: true})
		if err != nil {
			t.Fatal(err)
		}
		oracles[name] = report
	}
	for _, c := range []struct {
		name   string
		filter *SessionFilter
		only   string
	}{
		{"exclude b", ExcludeSessions([]SessionKey{keyB}), "a"},
		{"include b", IncludeSessions([]SessionKey{keyB}), "b"},
		{"exclude a", ExcludeSessions([]SessionKey{keyA}), "b"},
		{"include a, other client b", IncludeSessions([]SessionKey{keyA, {"codex", "b"}}), "a"},
	} {
		got, err := service.Signals(ctx, SignalOptions{From: from, To: to, Sessions: c.filter, IncludeSub: true})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, oracles[c.only]) {
			t.Fatalf("%s: %+v\nwant %+v", c.name, got, oracles[c.only])
		}
	}
	if reflect.DeepEqual(oracles["a"], oracles["b"]) {
		t.Fatal("fixture sessions are indistinguishable")
	}
	// Excluding nobody is the unfiltered report; including nobody is empty.
	all, err := service.Signals(ctx, SignalOptions{From: from, To: to, IncludeSub: true})
	if err != nil {
		t.Fatal(err)
	}
	none, err := service.Signals(ctx, SignalOptions{From: from, To: to, IncludeSub: true, Sessions: ExcludeSessions(nil)})
	if err != nil || !reflect.DeepEqual(all, none) {
		t.Fatalf("empty exclusion changed the report: %v", err)
	}
	empty, err := service.Signals(ctx, SignalOptions{From: from, To: to, IncludeSub: true, Sessions: IncludeSessions(nil)})
	if err != nil || empty.Tooling.Calls != 0 || empty.Workflow.EditsPerSession != nil {
		t.Fatalf("empty inclusion %+v %v", empty, err)
	}
}

// The shared batch read and the partition-projected read apply the same
// per-option filter as an individual report.
func TestFilteredBatchesMatchIndividualReports(t *testing.T) {
	ctx := context.Background()
	service, from, to := scanSessions(t, "a", "b")
	options := []SignalOptions{
		{From: from, To: to, IncludeSub: true},
		{From: from, To: to, IncludeSub: true, Sessions: ExcludeSessions([]SessionKey{{"claude", "b"}})},
		{From: from, To: to, IncludeSub: true, Sessions: IncludeSessions([]SessionKey{{"claude", "b"}})},
	}
	for name, batch := range map[string]func(context.Context, []SignalOptions) ([]SignalReport, error){
		"shared": service.SignalsBatch, "projected": service.ProjectedSignalsBatch,
	} {
		reports, err := batch(ctx, options)
		if err != nil {
			t.Fatal(err)
		}
		for i, option := range options {
			want, err := service.Signals(ctx, option)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reports[i], want) {
				t.Fatalf("%s option %d: %+v, want %+v", name, i, reports[i], want)
			}
		}
		if reflect.DeepEqual(reports[1].Activity, reports[2].Activity) {
			t.Fatalf("%s: filters did not select different sessions", name)
		}
	}
}
