package sessionorigin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// All databases below are isolated fixtures under t.TempDir().

// indexCopy adds one indexed copy with explicit project, times and priority.
func (s stores) indexCopy(t *testing.T, client, id, path, identity, prefix, project, firstAt, lastAt string, priority int) {
	t.Helper()
	if _, err := s.sessions.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,?,?,1,'2026-10-29T12:00:00Z')`, path, identity, prefix, priority); err != nil {
		t.Fatal(err)
	}
	if _, err := s.sessions.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,'m',1,?,?)`, path, client, id, project, firstAt, lastAt); err != nil {
		t.Fatal(err)
	}
}

func (s stores) reader() Reader {
	return Reader{Core: s.core, Sessions: s.sessions, StateRoot: s.root}
}

func (s stores) selectScope(t *testing.T, selection Selection) Snapshot {
	t.Helper()
	snapshot, err := s.reader().Select(context.Background(), selection, nil)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func keysOf(values []SelectedSession) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Client+"/"+value.SessionID)
	}
	return out
}

// mixedFixture holds every class: OTel maintenance, overrides both ways,
// unknown sessions, a duplicate copy and the same ID on another client.
func mixedFixture(t *testing.T) stores {
	t.Helper()
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "bg-1")
	s.settleBackground(t, "codex-app-server", "codex-tui", "bg-2")
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p1", "/w/alpha", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1)
	s.indexCopy(t, "claude", "c-1", "/archive/c-1.jsonl", "c-1-archive", "p1a", "/w/alpha-archive", "2026-10-29T08:00:00Z", "2026-10-29T08:30:00Z", 0)
	s.indexCopy(t, "claude", "c-2", "/c/c-2.jsonl", "c-2", "p2", "/w/beta", "2026-10-29T07:00:00Z", "2026-10-29T07:20:00Z", 1)
	s.indexCopy(t, "codex", "x-1", "/x/x-1.jsonl", "x-1", "p3", "/w/alpha", "2026-10-29T06:00:00Z", "2026-10-29T06:45:00Z", 1)
	s.indexCopy(t, "claude", "bg-1", "/c/bg-1.jsonl", "claude-bg-1", "p4", "/w/gamma", "2026-10-29T05:00:00Z", "", 1)
	s.reconcile(t, ReconcileOptions{})
	s.override(t, "claude", "c-2", ClassAuxiliary)
	s.override(t, "codex", "bg-2", ClassWork)
	return s
}

// The oracle is the unscoped index listing with hand-stated effective classes.
var mixedEffective = map[string]string{
	"codex/bg-1": ClassAuxiliary, "codex/bg-2": ClassWork, "claude/c-1": ClassUnknown,
	"claude/c-2": ClassAuxiliary, "codex/x-1": ClassUnknown, "claude/bg-1": ClassUnknown,
}

func TestScopesPartitionTheCanonicalSet(t *testing.T) {
	s := mixedFixture(t)
	listed, err := session.List(context.Background(), s.sessions.DB)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(mixedEffective) {
		t.Fatalf("fixture lists %d sessions", len(listed))
	}
	selections := map[string]Snapshot{}
	for _, scope := range []string{ScopeWork, ScopeAuxiliary, ScopeAll} {
		snapshot := s.selectScope(t, Selection{Scope: scope})
		selections[scope] = snapshot
		var want []string
		for _, item := range listed {
			key := item.Client + "/" + item.SessionID
			if InScope(scope, mixedEffective[key]) {
				want = append(want, key)
			}
		}
		if got := keysOf(snapshot.Sessions); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v (List order)", scope, got, want)
		}
		for _, item := range snapshot.Sessions {
			if item.Origin.Effective != mixedEffective[item.Client+"/"+item.SessionID] {
				t.Fatalf("%s/%s effective %+v", item.Client, item.SessionID, item.Origin)
			}
		}
	}
	work, aux, all := selections[ScopeWork], selections[ScopeAuxiliary], selections[ScopeAll]
	// Session counts add up; the scopes are disjoint and cover All.
	union := append(keysOf(work.Sessions), keysOf(aux.Sessions)...)
	sort.Strings(union)
	allKeys := keysOf(all.Sessions)
	sort.Strings(allKeys)
	if !reflect.DeepEqual(union, allKeys) || work.Counts.Sessions+aux.Counts.Sessions != all.Counts.Sessions {
		t.Fatalf("work %v + aux %v != all %v", keysOf(work.Sessions), keysOf(aux.Sessions), keysOf(all.Sessions))
	}
	// Unknown stays in Work and is counted there; Auxiliary has none.
	if work.UnknownCount != 3 || aux.UnknownCount != 0 || all.UnknownCount != 3 {
		t.Fatalf("unknown work=%d aux=%d all=%d", work.UnknownCount, aux.UnknownCount, all.UnknownCount)
	}
	// Projects are a set union, not a sum: /w/alpha is shared by c-1 and x-1;
	// the canonical copy of c-1 is the higher-priority /w/alpha one.
	if work.Counts.Projects != 2 || aux.Counts.Projects != 1 || all.Counts.Projects != 3 {
		t.Fatalf("projects work=%d aux=%d all=%d", work.Counts.Projects, aux.Counts.Projects, all.Counts.Projects)
	}
	// Durations recomputed per set: work = c-1 60m + x-1 45m + bg-2; claude/bg-1
	// has no last time and is missing, never zero.
	bg2 := all.Sessions[indexOf(t, all.Sessions, "codex/bg-2")]
	bg2Duration := durationMS(t, bg2.FirstAt, bg2.LastAt)
	if work.Counts.KnownDurationMS != (105*time.Minute).Milliseconds()+bg2Duration || work.Counts.MissingDurationCount != 1 {
		t.Fatalf("work durations %+v", work.Counts)
	}
	if aux.Counts.KnownDurationMS != (20*time.Minute).Milliseconds()+durationOf(t, all, "codex/bg-1") || aux.Counts.MissingDurationCount != 0 {
		t.Fatalf("aux durations %+v", aux.Counts)
	}
	if all.Counts.KnownDurationMS != work.Counts.KnownDurationMS+aux.Counts.KnownDurationMS ||
		*all.Counts.AverageDurationMS != all.Counts.KnownDurationMS/int64(all.Counts.Sessions-all.Counts.MissingDurationCount) {
		t.Fatalf("all durations %+v", all.Counts)
	}
	// Pagination happens after scope: pages concatenate to the selected set.
	var paged []SelectedSession
	for page := 1; ; page++ {
		values, pagination, err := session.Paginate(work.Sessions, page, 2, false)
		if err != nil {
			t.Fatal(err)
		}
		if pagination.Total != len(work.Sessions) {
			t.Fatalf("page total %d", pagination.Total)
		}
		paged = append(paged, values...)
		if !pagination.HasMore {
			break
		}
	}
	if !reflect.DeepEqual(keysOf(paged), keysOf(work.Sessions)) {
		t.Fatal("pages differ from the selected set")
	}
	// Work signals use the same membership; unclassified keys stay in Work.
	filter := work.SessionFilter()
	for key, class := range mixedEffective {
		client, id, _ := strings.Cut(key, "/")
		if filter.Allows(client, id) != (class != ClassAuxiliary) {
			t.Fatalf("work signal filter on %s", key)
		}
		if aux.SessionFilter().Allows(client, id) != (class == ClassAuxiliary) {
			t.Fatalf("aux signal filter on %s", key)
		}
	}
	if !filter.Allows("codex", "usage-only") || aux.SessionFilter().Allows("codex", "usage-only") || all.SessionFilter() != nil {
		t.Fatal("usage-only sessions must stay in Work")
	}
	// All three selections were read at one generation.
	if work.Vector != aux.Vector || aux.Vector != all.Vector {
		t.Fatalf("vectors %+v %+v %+v", work.Vector, aux.Vector, all.Vector)
	}
}

func indexOf(t *testing.T, values []SelectedSession, key string) int {
	t.Helper()
	for i, value := range values {
		if value.Client+"/"+value.SessionID == key {
			return i
		}
	}
	t.Fatalf("%s not selected", key)
	return -1
}

func durationMS(t *testing.T, first, last string) int64 {
	t.Helper()
	a, err := time.Parse(time.RFC3339Nano, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := time.Parse(time.RFC3339Nano, last)
	if err != nil {
		t.Fatal(err)
	}
	return b.Sub(a).Milliseconds()
}

func durationOf(t *testing.T, snapshot Snapshot, key string) int64 {
	value := snapshot.Sessions[indexOf(t, snapshot.Sessions, key)]
	return durationMS(t, value.FirstAt, value.LastAt)
}

// Between the index commit of a new copy and the next classification pass
// the stored token no longer matches: the session is unknown in Work, never
// hidden by its outdated auxiliary class.
func TestTokenMismatchNeverHidesASession(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "bg-1")
	s.reconcile(t, ReconcileOptions{})
	if got := keysOf(s.selectScope(t, Selection{Scope: ScopeAuxiliary}).Sessions); !reflect.DeepEqual(got, []string{"codex/bg-1"}) {
		t.Fatalf("aux before %v", got)
	}
	s.indexCopy(t, "codex", "bg-1", "/archive/bg-1.jsonl", "archive-bg-1", "a", "", "2026-10-29T12:00:00Z", "2026-10-29T12:00:00Z", 0)
	if got := s.selectScope(t, Selection{Scope: ScopeAuxiliary}); len(got.Sessions) != 0 {
		t.Fatalf("outdated auxiliary hid a session: %v", keysOf(got.Sessions))
	}
	work := s.selectScope(t, Selection{Scope: ScopeWork})
	if len(work.Sessions) != 1 || work.Sessions[0].Origin != (Origin{Automatic: ClassUnknown, Effective: ClassUnknown, ReasonCode: ReasonOriginInputMismatch, RuleVersion: ReducerVersion}) || work.UnknownCount != 1 {
		t.Fatalf("work during the window %+v", work.Sessions)
	}
	// The next pass binds the new token and the session returns to Auxiliary.
	s.reconcile(t, ReconcileOptions{})
	if got := keysOf(s.selectScope(t, Selection{Scope: ScopeAuxiliary}).Sessions); !reflect.DeepEqual(got, []string{"codex/bg-1"}) {
		t.Fatalf("aux after %v", got)
	}
}

func TestOverridesApplyOnlyToTheirCurrentBinding(t *testing.T) {
	s := openStores(t)
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p", "/w/a", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1)
	s.reconcile(t, ReconcileOptions{})
	s.override(t, "claude", "c-1", ClassAuxiliary)
	get := func() SelectedSession {
		t.Helper()
		all := s.selectScope(t, Selection{Scope: ScopeAll})
		if len(all.Sessions) != 1 {
			t.Fatalf("sessions %v", keysOf(all.Sessions))
		}
		return all.Sessions[0]
	}
	if got := get().Origin; got.Effective != ClassAuxiliary || got.Automatic != ClassUnknown || got.Override == nil || !got.Override.Applied || got.Override.BindingStatus != BindingMatched || got.Revision != 1 {
		t.Fatalf("matched override %+v %+v", got, got.Override)
	}
	// A restored, unmatched override is listed but never classifies.
	if _, err := s.core.DB.Exec(`UPDATE origin_overrides SET binding_state='unmatched'`); err != nil {
		t.Fatal(err)
	}
	if got := get().Origin; got.Effective != ClassUnknown || got.Override == nil || got.Override.Applied || got.Override.BindingStatus != BindingUnmatched {
		t.Fatalf("unmatched override %+v %+v", got, got.Override)
	}
	// A matched state whose birth differs from the indexed one does not apply.
	if _, err := s.core.DB.Exec(`UPDATE origin_overrides SET binding_state='matched', binding_digest='other-birth'`); err != nil {
		t.Fatal(err)
	}
	if got := get().Origin; got.Effective != ClassUnknown || got.Override.Applied || got.Override.BindingStatus != BindingUnmatched {
		t.Fatalf("foreign binding %+v %+v", got, got.Override)
	}
	// A tombstone keeps its revision and no override.
	if _, err := s.core.DB.Exec(`UPDATE origin_overrides SET value=NULL, revision=2`); err != nil {
		t.Fatal(err)
	}
	if got := get().Origin; got.Effective != ClassUnknown || got.Override != nil || got.Revision != 2 {
		t.Fatalf("tombstone %+v", got)
	}
}

// A classification of a session the index does not contain never creates a
// visible session; the signal filter keeps such usage in Work.
func TestUsageOnlyOriginsDoNotInventSessions(t *testing.T) {
	s := openStores(t)
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p", "/w/a", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1)
	if _, err := s.core.DB.Exec(`INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES('codex','usage-only','auxiliary','maintenance_origin','codex-otel-background-v1','t',1)`); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{ScopeWork, ScopeAuxiliary, ScopeAll} {
		snapshot := s.selectScope(t, Selection{Scope: scope})
		for _, key := range keysOf(snapshot.Sessions) {
			if key == "codex/usage-only" {
				t.Fatalf("%s invented a session", scope)
			}
		}
	}
	work := s.selectScope(t, Selection{Scope: ScopeWork})
	if work.Counts.Sessions != 1 || !work.SessionFilter().Allows("codex", "usage-only") {
		t.Fatalf("work %+v", work.Counts)
	}
}

func TestTimeWindowsAreHalfOpenAndMissingTimesAreSeparate(t *testing.T) {
	s := openStores(t)
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("time zone data unavailable")
	}
	// 2026-11-01 is the DST end day in New York: 25 hours long.
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, location)
	to := from.AddDate(0, 0, 1)
	if to.Sub(from) != 25*time.Hour {
		t.Fatalf("fixture day is %s", to.Sub(from))
	}
	add := func(id, first, last string) {
		s.indexCopy(t, "claude", id, "/c/"+id+".jsonl", id, id, "/w/"+id, first, last, 1)
	}
	add("at-start", from.UTC().Format(time.RFC3339Nano), from.UTC().Format(time.RFC3339Nano))
	add("late-local", "2026-11-02T03:00:00Z", "2026-11-02T04:30:00Z") // 23:30 local, after the repeated hour
	add("at-end", "2026-11-02T04:00:00Z", to.UTC().Format(time.RFC3339Nano))
	add("no-last", "2026-11-01T10:00:00Z", "")
	add("no-first", "", "2026-11-01T15:00:00Z")
	add("reversed", "2026-11-01T16:00:00Z", "2026-11-01T15:00:00Z")
	add("day-before", "2026-10-31T10:00:00Z", "2026-10-31T23:00:00Z")
	day := s.selectScope(t, Selection{Scope: ScopeAll, Period: "today", From: from, To: to})
	got := keysOfSorted(day.Sessions)
	want := []string{"claude/at-start", "claude/late-local", "claude/no-first", "claude/reversed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("window %v, want %v", got, want)
	}
	// Missing or reversed ends are missing durations, never zero.
	if day.Counts.MissingDurationCount != 2 || day.Counts.KnownDurationMS != (90*time.Minute).Milliseconds() || *day.Counts.AverageDurationMS != (45*time.Minute).Milliseconds() {
		t.Fatalf("window counts %+v", day.Counts)
	}
	// Without a window every session is selected; an unparseable last time
	// cannot be attributed to a period but is not dropped from the set.
	if all := s.selectScope(t, Selection{Scope: ScopeAll}); all.Counts.Sessions != 7 || all.Counts.MissingDurationCount != 3 {
		t.Fatalf("unwindowed %+v", all.Counts)
	}
}

func keysOfSorted(values []SelectedSession) []string {
	keys := keysOf(values)
	sort.Strings(keys)
	return keys
}

func TestSearchSeparatesDocumentAndSessionCounts(t *testing.T) {
	s := mixedFixture(t)
	for _, document := range []struct{ path, client, id, text string }{
		{"/c/c-1.jsonl", "claude", "c-1", "needle one"},
		{"/c/c-1.jsonl", "claude", "c-1", "needle two"},
		{"/c/c-2.jsonl", "claude", "c-2", "needle aux"},
		{"/archive/c-1.jsonl", "claude", "c-1", "needle archived copy"},
	} {
		if _, err := s.sessions.DB.Exec(`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,'2026-10-29T08:00:00Z','user',?)`, document.path, document.client, document.id, document.text); err != nil {
			t.Fatal(err)
		}
	}
	work := s.selectScope(t, Selection{Scope: ScopeWork, Query: "needle"})
	if len(work.Documents) != 2 || work.SelectedSessionCount != 1 || !reflect.DeepEqual(keysOf(work.Sessions), []string{"claude/c-1"}) || work.UnknownCount != 1 {
		t.Fatalf("work search docs=%d sessions=%d %v", len(work.Documents), work.SelectedSessionCount, keysOf(work.Sessions))
	}
	aux := s.selectScope(t, Selection{Scope: ScopeAuxiliary, Query: "needle"})
	if len(aux.Documents) != 1 || aux.SelectedSessionCount != 1 || aux.Documents[0].SessionID != "c-2" {
		t.Fatalf("aux search %+v", aux.Documents)
	}
	all := s.selectScope(t, Selection{Scope: ScopeAll, Query: "needle"})
	plain, err := session.Search(context.Background(), s.sessions.DB, "needle")
	if err != nil || !reflect.DeepEqual(all.Documents, plain) || all.SelectedSessionCount != 2 {
		t.Fatalf("all search differs from Search: %v", err)
	}
}

// Writes interleaved between the two databases' reads and the verification
// discard the candidate; a vector that never settles fails closed.
func TestInterleavedWritesRetryThenFailClosed(t *testing.T) {
	s := mixedFixture(t)
	t.Cleanup(func() { beforeSelectionVerify = func(int) {} })
	attempts := 0
	beforeSelectionVerify = func(attempt int) {
		attempts++
		switch attempt {
		case 0: // a core write: another override
			s.override(t, "claude", "c-1", ClassAuxiliary)
		case 1: // a session-index write: a new session
			s.indexCopy(t, "codex", "x-2", "/x/x-2.jsonl", "x-2", "q", "/w/delta", "2026-10-29T04:00:00Z", "2026-10-29T04:10:00Z", 1)
		}
	}
	snapshot, err := s.reader().Select(context.Background(), Selection{Scope: ScopeAuxiliary}, nil)
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	// The accepted candidate contains both interleaved writes.
	if got := keysOfSorted(snapshot.Sessions); !reflect.DeepEqual(got, []string{"claude/c-1", "claude/c-2", "codex/bg-1"}) {
		t.Fatalf("accepted %v", got)
	}
	all := s.selectScope(t, Selection{Scope: ScopeAll})
	if indexOf(t, all.Sessions, "codex/x-2") < 0 {
		t.Fatal("index write missing")
	}

	attempts = 0
	beforeSelectionVerify = func(attempt int) {
		attempts++
		s.indexCopy(t, "codex", fmt.Sprintf("churn-%d", attempts), fmt.Sprintf("/x/churn-%d.jsonl", attempts), fmt.Sprintf("churn-%d", attempts), "c", "", "2026-10-29T04:00:00Z", "2026-10-29T04:00:00Z", 1)
	}
	signalsRead := 0
	_, err = s.reader().Select(context.Background(), Selection{Scope: ScopeWork}, func(context.Context, *Snapshot) error { signalsRead++; return nil })
	if !errors.Is(err, ErrSelectionStale) || attempts != MaxSelectionRetries+1 || signalsRead != MaxSelectionRetries+1 {
		t.Fatalf("err=%v attempts=%d attached=%d", err, attempts, signalsRead)
	}
}

func TestRolledBackWritesDoNotMoveTheVector(t *testing.T) {
	s := mixedFixture(t)
	before := s.selectScope(t, Selection{Scope: ScopeAll})
	for _, db := range []*store.Store{s.core, s.sessions} {
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatal(err)
		}
		statement := `DELETE FROM origin_overrides`
		if db == s.sessions {
			statement = `DELETE FROM session_metadata`
		}
		if _, err := tx.Exec(statement); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
	after := s.selectScope(t, Selection{Scope: ScopeAll})
	if after.Vector != before.Vector || !reflect.DeepEqual(after.Sessions, before.Sessions) {
		t.Fatalf("rollback moved the vector %+v -> %+v", before.Vector, after.Vector)
	}
}

// Every table the selection and its signals read advances the serving
// revision on INSERT, UPDATE and DELETE, including direct SQL.
func TestEverySelectionDependencyHasARevisionTrigger(t *testing.T) {
	s := openStores(t)
	audit := func(db *store.Store, tables []string) {
		t.Helper()
		for _, table := range tables {
			for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
				var found int
				if err := db.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND tbl_name=? AND upper(sql) LIKE ? AND sql LIKE '%serving_generation SET revision=revision+1%'`, table, "%AFTER "+op+" ON "+strings.ToUpper(table)+"%").Scan(&found); err != nil {
					t.Fatal(err)
				}
				if found == 0 {
					t.Errorf("%s %s does not advance serving revision", table, op)
				}
			}
		}
	}
	audit(s.core, []string{"session_origins", "origin_overrides", "usage_events", "usage_work_signals", "usage_tool_calls", "usage_tool_files",
		"usage_sessions", "usage_run_bindings", "usage_runs", "model_prices", "price_catalogs", "providers", "provider_clients", "provider_selections"})
	audit(s.sessions, []string{"session_metadata", "session_sources", "session_exclusions", "session_documents_content"})

	// Direct SQL on the classification inputs moves the vector.
	revision := func(db *store.Store) int64 { return servingRevision(t, db.DB) }
	for _, step := range []struct {
		db        *store.Store
		statement string
	}{
		{s.core, `INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES('claude','d','unknown','origin_metadata_missing','origin-reducer-v1','t',0)`},
		{s.core, `UPDATE session_origins SET input_token='u'`},
		{s.core, `DELETE FROM session_origins`},
		{s.core, `INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES('claude','d','x',0,'matched','work',1,'t')`},
		{s.core, `UPDATE origin_overrides SET value='auxiliary'`},
		{s.core, `DELETE FROM origin_overrides`},
		{s.sessions, `INSERT INTO session_exclusions(kind,value) VALUES('session','d')`},
		{s.sessions, `DELETE FROM session_exclusions`},
		{s.sessions, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('/d.jsonl','claude','d','','user','x')`},
		{s.sessions, `UPDATE session_documents SET text='y'`},
		{s.sessions, `DELETE FROM session_documents`},
	} {
		before := revision(step.db)
		if _, err := step.db.DB.Exec(step.statement); err != nil {
			t.Fatal(err)
		}
		if revision(step.db) <= before {
			t.Fatalf("%q did not advance the revision", step.statement)
		}
	}
}

// Selecting is a read: it writes nothing to either database, so switching
// scopes publishes nothing and starts no collection.
func TestSelectingWritesNothing(t *testing.T) {
	s := mixedFixture(t)
	const coreState = `SELECT (SELECT revision FROM serving_generation),(SELECT revision||':'||dirty FROM derived_snapshot_generation),(SELECT count(*) FROM usage_derived_invalidations),(SELECT group_concat(revision||dirty) FROM usage_derived_partitions)`
	coreBefore, sessionsBefore := dump(t, s.core.DB, coreState), servingRevision(t, s.sessions.DB)
	for _, scope := range []string{ScopeWork, ScopeAuxiliary, ScopeAll, ScopeWork} {
		s.selectScope(t, Selection{Scope: scope, Client: "codex"})
	}
	if dump(t, s.core.DB, coreState) != coreBefore || servingRevision(t, s.sessions.DB) != sessionsBefore {
		t.Fatal("selection wrote state")
	}
}

func TestIncrementalSelectionEqualsAFullRebuild(t *testing.T) {
	s := mixedFixture(t)
	s.indexCopy(t, "codex", "bg-1", "/archive/bg-1.jsonl", "archive-bg-1", "a", "", "2026-10-29T12:00:00Z", "2026-10-29T12:00:00Z", 0)
	s.reconcile(t, ReconcileOptions{})
	incremental := s.selectScope(t, Selection{Scope: ScopeAll})
	if _, err := s.core.DB.Exec(`DELETE FROM session_origins; DELETE FROM origin_evidence`); err != nil {
		t.Fatal(err)
	}
	s.reconcile(t, ReconcileOptions{})
	full := s.selectScope(t, Selection{Scope: ScopeAll})
	if !reflect.DeepEqual(full.Sessions, incremental.Sessions) || !reflect.DeepEqual(full.Counts, incremental.Counts) || full.UnknownCount != incremental.UnknownCount {
		t.Fatal("incremental and full classification select differently")
	}
}

// Missing classification metadata reads as unknown; an unreadable store is
// an unavailable section, never an empty or all-work one.
func TestUnreadableStoresAreUnavailableNotUnknown(t *testing.T) {
	s := mixedFixture(t)
	// A core that predates origin storage holds no origin fact: all unknown.
	if _, err := s.core.DB.Exec(`UPDATE schema_metadata SET version=?`, store.OriginSchemaVersion-1); err != nil {
		t.Fatal(err)
	}
	old := s.selectScope(t, Selection{Scope: ScopeWork})
	if old.Counts.Sessions != len(mixedEffective) || old.UnknownCount != len(mixedEffective) {
		t.Fatalf("pre-origin core %+v unknown=%d", old.Counts, old.UnknownCount)
	}
	if _, err := s.core.DB.Exec(`UPDATE schema_metadata SET version=?`, store.CurrentSchemaVersion); err != nil {
		t.Fatal(err)
	}
	closedCore, err := store.Open(context.Background(), s.root)
	if err != nil {
		t.Fatal(err)
	}
	closedCore.Close()
	if _, err := (Reader{Core: closedCore, Sessions: s.sessions, StateRoot: s.root}).Select(context.Background(), Selection{Scope: ScopeWork}, nil); !errors.Is(err, ErrClassificationUnavailable) {
		t.Fatalf("unreadable core: %v", err)
	}
	closedSessions, err := store.OpenSessions(context.Background(), s.root)
	if err != nil {
		t.Fatal(err)
	}
	closedSessions.Close()
	if _, err := (Reader{Core: s.core, Sessions: closedSessions, StateRoot: s.root}).Select(context.Background(), Selection{Scope: ScopeWork}, nil); !errors.Is(err, ErrSessionsUnavailable) {
		t.Fatalf("unreadable index: %v", err)
	}
	failing := Reader{Core: s.core, Sessions: s.sessions, Verify: func(context.Context) (store.ServingGeneration, error) {
		return store.ServingGeneration{}, errors.New("gone")
	}}
	if _, err := failing.Select(context.Background(), Selection{Scope: ScopeWork}, nil); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatalf("unverifiable vector: %v", err)
	}
	for _, bad := range []Selection{{Scope: "everything"}, {Scope: ScopeWork, Client: "gemini"}, {Scope: ScopeWork, From: time.Now()}, {Scope: ScopeWork, From: time.Now(), To: time.Now().Add(-time.Hour)}} {
		if _, err := s.reader().Select(context.Background(), bad, nil); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
}

// An override dirties only the usage partitions of its own session's events.
func TestReclassificationDirtiesOnlyAffectedPartitions(t *testing.T) {
	s := openStores(t)
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p", "/w/a", "2026-10-27T08:00:00Z", "2026-10-27T09:00:00Z", 1)
	s.indexCopy(t, "claude", "c-2", "/c/c-2.jsonl", "c-2", "q", "/w/b", "2026-10-28T08:00:00Z", "2026-10-28T09:00:00Z", 1)
	for _, event := range []struct{ key, session, at, model string }{
		{"e1", "c-1", "2026-10-27T08:30:00Z", "claude-opus"},
		{"e2", "c-2", "2026-10-28T08:30:00Z", "claude-opus"},
		{"e3", "c-2", "2026-10-28T08:40:00Z", "claude-sonnet"},
	} {
		if _, err := s.core.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?,'claude',?,?,?,?,10,1,'/c/x.jsonl',0)`, event.key, event.session, event.key, event.at, event.model); err != nil {
			t.Fatal(err)
		}
	}
	s.reconcile(t, ReconcileOptions{})
	// Simulate published partitions with no pending input markers.
	if _, err := s.core.DB.Exec(`UPDATE usage_derived_partitions SET dirty=0, active_revision=revision; DELETE FROM usage_derived_invalidations`); err != nil {
		t.Fatal(err)
	}
	const partitions = `SELECT client||'/'||model||'/'||day||'='||dirty FROM usage_derived_partitions ORDER BY 1`
	if got := dump(t, s.core.DB, partitions); got != "claude/claude-opus/2026-10-27=0\nclaude/claude-opus/2026-10-28=0\nclaude/claude-sonnet/2026-10-28=0" {
		t.Fatalf("fixture partitions %q", got)
	}
	s.override(t, "claude", "c-2", ClassAuxiliary)
	if err := s.core.FlushUsagePartitionInvalidations(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, s.core.DB, partitions); got != "claude/claude-opus/2026-10-27=0\nclaude/claude-opus/2026-10-28=1\nclaude/claude-sonnet/2026-10-28=1" {
		t.Fatalf("after override %q", got)
	}
}

// Trigger write amplification of the classification writes, recorded for the
// integrated budget (task 9): each committed origin row advances the core
// serving revision once and enqueues one session invalidation marker per key.
func TestOriginWriteAmplification(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "bg-1")
	s.settleBackground(t, "codex-app-server", "codex-tui", "bg-2")
	if _, err := s.core.DB.Exec(`DELETE FROM usage_derived_invalidations`); err != nil {
		t.Fatal(err)
	}
	markers := func() int {
		var n int
		if err := s.core.DB.QueryRow(`SELECT count(*) FROM usage_derived_invalidations WHERE kind='session'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := servingRevision(t, s.core.DB)
	s.reconcile(t, ReconcileOptions{})
	// Two sessions: one session_origins row and one origin_evidence row each.
	if delta, n := servingRevision(t, s.core.DB)-before, markers(); delta != 4 || n != 2 {
		t.Fatalf("classification pass: revision +%d, %d markers", delta, n)
	}
	before = servingRevision(t, s.core.DB)
	s.reconcile(t, ReconcileOptions{})
	if delta := servingRevision(t, s.core.DB) - before; delta != 0 {
		t.Fatalf("unchanged pass advanced the revision by %d", delta)
	}
	if _, err := s.core.DB.Exec(`DELETE FROM usage_derived_invalidations`); err != nil {
		t.Fatal(err)
	}
	before = servingRevision(t, s.core.DB)
	s.override(t, "codex", "bg-1", ClassWork)
	if delta, n := servingRevision(t, s.core.DB)-before, markers(); delta != 1 || n != 1 {
		t.Fatalf("override: revision +%d, %d markers", delta, n)
	}
}

// ASC-SP-R3-F1: searchable text is a selection input. A direct SQL write to
// session_documents moves the vector and the publication identity; a rolled
// back one does not; one interleaved with a selection forces a retry.
func TestSearchTextChangesMoveTheSelectionGeneration(t *testing.T) {
	s := mixedFixture(t)
	if _, err := s.sessions.DB.Exec(`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('/c/c-1.jsonl','claude','c-1','2026-10-29T08:00:00Z','user','clockneedle')`); err != nil {
		t.Fatal(err)
	}
	selection := Selection{Scope: ScopeWork, Query: "clockneedle"}
	before := s.selectScope(t, selection)
	if len(before.Documents) != 1 || before.SelectedSessionCount != 1 {
		t.Fatalf("fixture search %d/%d", len(before.Documents), before.SelectedSessionCount)
	}

	// A rolled-back text change leaves the generation and the result as they were.
	tx, err := s.sessions.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE session_documents SET text='replacement' WHERE source_path='/c/c-1.jsonl'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if again := s.selectScope(t, selection); again.Vector != before.Vector || len(again.Documents) != 1 {
		t.Fatalf("rollback moved the vector %+v -> %+v", before.Vector, again.Vector)
	}

	// A committed direct UPDATE changes the result, the vector and the identity.
	if _, err := s.sessions.DB.Exec(`UPDATE session_documents SET text='replacement' WHERE source_path='/c/c-1.jsonl'`); err != nil {
		t.Fatal(err)
	}
	after := s.selectScope(t, selection)
	if len(after.Documents) != 0 || after.SelectedSessionCount != 0 {
		t.Fatalf("search after update %d/%d", len(after.Documents), after.SelectedSessionCount)
	}
	if after.Vector == before.Vector || after.Vector.SessionRevision <= before.Vector.SessionRevision {
		t.Fatalf("direct FTS update kept the vector %+v", after.Vector)
	}
	if NewPublication(before, time.Unix(1, 0)).ID == NewPublication(after, time.Unix(2, 0)).ID {
		t.Fatal("different search results share a publication identity")
	}

	// A text write between a selection's reads and its verification is
	// detected; the accepted candidate contains it.
	t.Cleanup(func() { beforeSelectionVerify = func(int) {} })
	attempts := 0
	beforeSelectionVerify = func(attempt int) {
		attempts++
		if attempt == 0 {
			if _, err := s.sessions.DB.Exec(`UPDATE session_documents SET text='clockneedle again' WHERE source_path='/c/c-1.jsonl'`); err != nil {
				t.Fatal(err)
			}
		}
	}
	interleaved, err := s.reader().Select(context.Background(), selection, nil)
	if err != nil || attempts != 2 || len(interleaved.Documents) != 1 || interleaved.Documents[0].Text != "clockneedle again" {
		t.Fatalf("interleaved text write: attempts=%d docs=%+v err=%v", attempts, interleaved.Documents, err)
	}
}

// An index created before the content-table triggers gains them on its next
// writable open, so an upgraded install is covered too.
func TestExistingIndexGainsTheSearchTextTriggers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	index, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"insert", "update", "delete"} {
		if _, err := index.DB.Exec(`DROP TRIGGER serving_revision_session_documents_content_` + strings.ToUpper(op)); err != nil {
			t.Fatal(err)
		}
	}
	index.Close()
	index, err = store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	var triggers int
	if err := index.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND tbl_name='session_documents_content'`).Scan(&triggers); err != nil || triggers != 3 {
		t.Fatalf("triggers=%d %v", triggers, err)
	}
}

// A core that was never created holds no classification: every indexed
// session is unknown in Work, with the same index order and canonical copies
// as the full reader, and nothing is created.
func TestSelectAbsentCoreKeepsEverySessionUnknownInWork(t *testing.T) {
	s := openStores(t)
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p1", "/w/alpha", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1)
	s.indexCopy(t, "claude", "c-1", "/archive/c-1.jsonl", "c-1-archive", "p1a", "/w/alpha-archive", "2026-10-29T08:00:00Z", "2026-10-29T08:30:00Z", 0)
	s.indexCopy(t, "codex", "x-1", "/x/x-1.jsonl", "x-1", "p3", "/w/alpha", "2026-10-29T06:00:00Z", "2026-10-29T06:45:00Z", 1)
	full := s.selectScope(t, Selection{Scope: ScopeAll})
	reader := Reader{Sessions: s.sessions}
	for _, scope := range []string{ScopeWork, ScopeAuxiliary, ScopeAll} {
		snapshot, err := reader.SelectAbsentCore(context.Background(), Selection{Scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		want := keysOf(full.Sessions)
		if scope == ScopeAuxiliary {
			want = []string{}
		}
		if got := keysOf(snapshot.Sessions); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v", scope, got, want)
		}
		for _, item := range snapshot.Sessions {
			if item.Origin.Effective != ClassUnknown || item.Origin.ReasonCode != ReasonOriginMetadataMissing {
				t.Fatalf("%s origin = %+v", scope, item.Origin)
			}
		}
		if snapshot.Vector.CoreEpoch != 0 || snapshot.Vector.SessionEpoch == 0 {
			t.Fatalf("vector = %+v", snapshot.Vector)
		}
		if scope != ScopeAuxiliary && snapshot.UnknownCount != len(want) {
			t.Fatalf("%s unknown = %d", scope, snapshot.UnknownCount)
		}
	}
	if _, err := reader.SelectAbsentCore(context.Background(), Selection{Scope: "everything"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid scope = %v", err)
	}
}
