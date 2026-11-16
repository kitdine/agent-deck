package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

// Every state root below is an isolated fixture under t.TempDir().

type originSeed struct {
	client, id, automatic, reason, rule string
}

// seedOrigins commits classification rows whose input token matches the copy
// seedSessions indexes, so they apply exactly as the reconciler's would.
func seedOrigins(t *testing.T, root string, seeds []originSeed) {
	t.Helper()
	ctx := context.Background()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer core.Close()
	generation := []sessionorigin.SourceGeneration{{Identity: "synthetic", PrefixHash: "", ParserVersion: session.ParserVersion}}
	for _, seed := range seeds {
		token := sessionorigin.InputToken(seed.client, seed.id, generation, nil)
		if _, err := core.DB.ExecContext(ctx, `INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES(?,?,?,?,?,?,1)`,
			seed.client, seed.id, seed.automatic, seed.reason, seed.rule, token); err != nil {
			t.Fatalf("seed origin: %v", err)
		}
	}
}

func setDefaultScope(t *testing.T, root, value string) {
	t.Helper()
	ctx := context.Background()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer core.Close()
	current := (sessionorigin.Service{Store: core}).DefaultScope(ctx)
	if _, err := (sessionorigin.Service{Store: core}).SetDefaultScope(ctx, sessionorigin.ScopeRequest{RequestID: "test-default", Value: value, ExpectedRevision: *current.Revision}); err != nil {
		t.Fatalf("SetDefaultScope: %v", err)
	}
}

func scopeService(t *testing.T, root string) Service {
	t.Helper()
	return Service{
		RegistrationProbe: func(context.Context, bool) doctor.RegistrationDetails { return doctor.RegistrationDetails{} },
		StateRoot:         root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now: func() time.Time { return fixtureNow }, Location: time.UTC,
	}
}

// scopeFixture is the complete fixture's state plus committed classes:
// codex-today is a maintenance session (it has usage, so scoped signals are
// observable), claude-today's source was unreadable, and the rest have no
// classification evidence and stay unknown in Work.
func scopeFixture(t *testing.T) string {
	t.Helper()
	root := fixtureStateRoot(t)
	seedUsage(t, root, []usageSeed{
		{client: "codex", session: "codex-today", at: "2026-08-13T08:00:00Z", model: "gpt-5", input: 1200, output: 240, activityKind: "coding", activitySub: "feature", toolKind: "edit", baseName: "main.go", wrote: true},
		{client: "claude", session: "claude-today", at: "2026-08-13T09:00:00Z", model: "claude-sonnet-5", input: 300, output: 60, activityKind: "coding", activitySub: "testing", toolKind: "mcp", mcpServer: "codegraph"},
	})
	seedSessions(t, root, []sessionSeed{
		{client: "codex", id: "codex-today", project: "/Users/example/private/maintenance", model: "gpt-5", first: "2026-08-13T08:00:00Z", last: "2026-08-13T09:00:00Z"},
		{client: "codex", id: "codex-other", project: "/Users/example/private/other/agent-deck", model: "gpt-5", first: "2026-08-13T09:10:00Z", last: "2026-08-13T09:40:00Z"},
		{client: "codex", id: "codex-week", project: "/Users/example/private/agent-deck", model: "gpt-5", first: "2026-08-10T08:00:00Z", last: "2026-08-10T08:45:00Z"},
		{client: "claude", id: "claude-today", project: "/Users/example/private/notes", model: "claude-sonnet-5", first: "2026-08-13T09:00:00Z", last: "2026-08-13T09:20:00Z"},
		{client: "claude", id: "claude-child", project: "/Users/example/private/notes", model: "claude-sonnet-5", first: "2026-08-13T09:21:00Z", last: "2026-08-13T09:30:00Z"},
	})
	seedOrigins(t, root, []originSeed{
		{client: "codex", id: "codex-today", automatic: "auxiliary", reason: sessionorigin.ReasonMaintenanceOrigin, rule: "codex-otel-background-v1"},
		{client: "claude", id: "claude-today", automatic: "unknown", reason: sessionorigin.ReasonOriginSourceUnavailable, rule: "claude-mem-sdk-map-v1"},
		{client: "claude", id: "claude-child", automatic: "unknown", reason: sessionorigin.ReasonTaskChildUnconfirmed, rule: sessionorigin.ReducerVersion},
	})
	return root
}

func readScope(t *testing.T, s Service, scope, client, period string, limit int) SessionScopeView {
	t.Helper()
	view, err := s.SessionScopeView(context.Background(), SessionScopeRequest{Scope: scope, Client: client, Period: period, RecentLimit: limit})
	if err != nil {
		t.Fatalf("SessionScopeView(%s/%s/%s): %v", scope, client, period, err)
	}
	return view
}

func recentKeys(view SessionScopeView) []string {
	keys := []string{}
	for _, item := range view.Recent {
		keys = append(keys, item.Client+"/"+item.SessionID)
	}
	sort.Strings(keys)
	return keys
}

func TestSessionScopeViewPartitionsTheSelectionWithoutTouchingLegacySessions(t *testing.T) {
	root := scopeFixture(t)
	s := scopeService(t, root)
	result, err := s.Build(context.Background(), Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	// The legacy section keeps describing every indexed session.
	legacy := result.Snapshot.Sessions
	if !legacy.Available || legacy.Total != 5 {
		t.Fatalf("legacy sessions changed meaning: available=%v total=%d", legacy.Available, legacy.Total)
	}
	for _, item := range legacy.Periods.Items {
		if item.Period == "today" && item.Client == "all" && item.Sessions != 4 {
			t.Fatalf("legacy today/all = %d, want every indexed session (4)", item.Sessions)
		}
	}
	// The published view is the confirmed default (work, never saved) for
	// every client today.
	embedded := result.Snapshot.SessionScopeView
	if embedded == nil || !embedded.Available || !embedded.Default.Confirmed || *embedded.Default.Value != "work" ||
		!reflect.DeepEqual(*embedded.Selection, SessionScopeSelection{Scope: "work", Client: "all", Period: "today"}) {
		t.Fatalf("embedded view = %+v", embedded)
	}

	work := readScope(t, s, "work", "all", "today", 20)
	aux := readScope(t, s, "auxiliary", "all", "today", 20)
	all := readScope(t, s, "all", "all", "today", 20)
	if !reflect.DeepEqual(*embedded.Counts, *work.Counts) || *embedded.PublicationID != *work.PublicationID {
		t.Fatalf("embedded %+v differs from the explicit read of the same selection %+v", embedded.Counts, work.Counts)
	}
	// Unknown sessions — the task child included — stay in Work.
	if got := recentKeys(work); !reflect.DeepEqual(got, []string{"claude/claude-child", "claude/claude-today", "codex/codex-other"}) {
		t.Fatalf("work = %v", got)
	}
	if got := recentKeys(aux); !reflect.DeepEqual(got, []string{"codex/codex-today"}) {
		t.Fatalf("auxiliary = %v", got)
	}
	if work.Counts.Sessions+aux.Counts.Sessions != all.Counts.Sessions || all.Counts.Sessions != 4 {
		t.Fatalf("work %d + aux %d != all %d", work.Counts.Sessions, aux.Counts.Sessions, all.Counts.Sessions)
	}
	// Projects are a set per selection, never a sum of the two scopes.
	if work.Counts.Projects != 2 || aux.Counts.Projects != 1 || all.Counts.Projects != 3 {
		t.Fatalf("projects work=%d aux=%d all=%d", work.Counts.Projects, aux.Counts.Projects, all.Counts.Projects)
	}
	if *work.Counts.KnownDurationMS != (59*time.Minute).Milliseconds() || *aux.Counts.KnownDurationMS != time.Hour.Milliseconds() {
		t.Fatalf("known durations work=%d aux=%d", *work.Counts.KnownDurationMS, *aux.Counts.KnownDurationMS)
	}
	if *work.OriginCoverage.UnknownCount != 3 || *aux.OriginCoverage.UnknownCount != 0 || *all.OriginCoverage.UnknownCount != 3 {
		t.Fatalf("unknown work=%d aux=%d all=%d", *work.OriginCoverage.UnknownCount, *aux.OriginCoverage.UnknownCount, *all.OriginCoverage.UnknownCount)
	}
	// claude-today's unreadable source makes every scope's coverage partial —
	// Auxiliary too, although the session itself is not in it.
	for name, view := range map[string]SessionScopeView{"work": work, "auxiliary": aux, "all": all} {
		if view.OriginCoverage.Complete || !reflect.DeepEqual(view.OriginCoverage.Reasons, []string{sessionorigin.ReasonOriginSourceUnavailable}) {
			t.Fatalf("%s coverage %+v", name, view.OriginCoverage)
		}
	}
	// Each recent row carries its classification and only safe fields.
	maintenance := aux.Recent[0]
	if maintenance.Origin.Effective != "auxiliary" || maintenance.Origin.ReasonCode != sessionorigin.ReasonMaintenanceOrigin ||
		maintenance.Origin.RuleVersion != "codex-otel-background-v1" || maintenance.Project != "maintenance" {
		t.Fatalf("maintenance row %+v", maintenance)
	}
	encoded, err := json.Marshal(work)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/Users/example") || strings.Contains(string(encoded), "/private/source.jsonl") {
		t.Fatalf("view leaked a path: %s", encoded)
	}
	// Signals follow the same membership: the maintenance session's edit call
	// is Auxiliary's only tool call and absent from Work; All has both.
	calls := func(view SessionScopeView) int64 {
		var total int64
		for _, item := range view.WorkSignals.Tooling.Items {
			if item.Period != "today" || item.Client != "all" {
				t.Fatalf("signal item for another selection: %+v", item)
			}
			total += item.Calls
		}
		return total
	}
	if !work.WorkSignals.Tooling.Available || calls(work) != 1 || calls(aux) != 1 || calls(all) != 2 {
		t.Fatalf("tooling calls work=%d aux=%d all=%d", calls(work), calls(aux), calls(all))
	}
	// Client and period come before scope.
	codexWeek := readScope(t, s, "work", "codex", "7d", 20)
	if got := recentKeys(codexWeek); !reflect.DeepEqual(got, []string{"codex/codex-other", "codex/codex-week"}) {
		t.Fatalf("codex/7d work = %v", got)
	}
	// One generation answers all of these reads.
	if *work.PublicationID == *aux.PublicationID || readScope(t, s, "work", "all", "today", 20).PublicationID == nil {
		t.Fatal("publication identity must name the selection it answers")
	}
}

func TestSessionScopeViewRecentIsBoundedAndCountsAreNot(t *testing.T) {
	s := scopeService(t, scopeFixture(t))
	view := readScope(t, s, "all", "all", "30d", 2)
	if len(view.Recent) != 2 || view.Counts.Sessions != 5 {
		t.Fatalf("recent %d counts %d", len(view.Recent), view.Counts.Sessions)
	}
}

func TestSessionScopeViewDurationIsNullOnlyWhenEveryDurationIsMissing(t *testing.T) {
	root := fixtureStateRoot(t)
	seedSelections(t, root)
	// Both sessions end today, but neither has a measurable span: one has no
	// start, the other ends before it starts.
	seedSessions(t, root, []sessionSeed{
		{client: "codex", id: "no-start", project: "/p/a", model: "gpt-5", first: "", last: "2026-08-13T09:00:00Z"},
		{client: "claude", id: "reversed", project: "/p/b", model: "m", first: "2026-08-13T09:30:00Z", last: "2026-08-13T09:00:00Z"},
	})
	s := scopeService(t, root)
	missing := readScope(t, s, "work", "all", "today", 5)
	if missing.Counts.Sessions != 2 || missing.Counts.KnownDurationMS != nil || missing.Counts.MissingDurationCount != 2 {
		t.Fatalf("every duration missing must be null, never 0: %+v", missing.Counts)
	}
	// One measurable session makes the subtotal known; the other stays counted
	// as missing rather than as zero.
	partial := readScope(t, scopeService(t, scopeFixture(t)), "all", "all", "30d", 5)
	if partial.Counts.KnownDurationMS == nil || partial.Counts.MissingDurationCount != 0 {
		t.Fatalf("measurable selection %+v", partial.Counts)
	}
	empty := readScope(t, s, "auxiliary", "all", "today", 5)
	if !empty.Available || empty.Counts.Sessions != 0 || empty.Counts.KnownDurationMS == nil || *empty.Counts.KnownDurationMS != 0 {
		t.Fatalf("an empty selection is a trusted 0: %+v", empty.Counts)
	}
}

// ASC-DM-R2-F1: the session list and the work signals are different data
// domains. When every indexed session is Auxiliary, Work has no session to
// list, while usage with no text index still belongs to Work (unknown origin)
// and carries real signals. The view states both facts; it neither invents a
// session nor drops the signals.
func TestSessionScopeViewKeepsUsageOnlySignalsWhenNoIndexedSessionIsInScope(t *testing.T) {
	root := fixtureStateRoot(t)
	seedUsage(t, root, []usageSeed{
		{client: "codex", session: "usage-only-work", at: "2026-08-13T08:00:00Z", model: "gpt-5", input: 1200, output: 240, activityKind: "coding", activitySub: "feature", toolKind: "edit", baseName: "main.go", wrote: true},
	})
	seedSessions(t, root, []sessionSeed{
		{client: "codex", id: "indexed-maintenance", project: "/Users/example/private/maintenance", model: "gpt-5", first: "2026-08-13T08:30:00Z", last: "2026-08-13T09:00:00Z"},
	})
	seedOrigins(t, root, []originSeed{
		{client: "codex", id: "indexed-maintenance", automatic: "auxiliary", reason: sessionorigin.ReasonMaintenanceOrigin, rule: "codex-otel-background-v1"},
	})
	s := scopeService(t, root)
	result, err := s.Build(context.Background(), Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if legacy := result.Snapshot.Sessions; !legacy.Available || legacy.Total != 1 {
		t.Fatalf("legacy sessions available=%v total=%d, want the one indexed session", legacy.Available, legacy.Total)
	}
	if all := readScope(t, s, "all", "all", "today", 5); all.Counts.Sessions != 1 {
		t.Fatalf("all sessions = %d, want 1", all.Counts.Sessions)
	}

	work := readScope(t, s, "work", "all", "today", 5)
	if !work.Available || work.Counts.Sessions != 0 || work.Counts.Projects != 0 || len(work.Recent) != 0 {
		t.Fatalf("work must list no session: available=%v counts=%+v recent=%d", work.Available, work.Counts, len(work.Recent))
	}
	signals := work.WorkSignals
	if !signals.Activity.Available || len(signals.Activity.Items) != 1 {
		t.Fatalf("work activity = %+v", signals.Activity)
	}
	var coding, feature int64
	for _, kind := range signals.Activity.Items[0].Kinds {
		if kind.Kind != "coding" {
			continue
		}
		coding = kind.Events
		for _, sub := range kind.Sub {
			if sub.Kind == "feature" {
				feature = sub.Events
			}
		}
	}
	if coding != 1 || feature != 1 {
		t.Fatalf("work coding/feature events = %d/%d, want 1/1", coding, feature)
	}
	if !signals.Workflow.Available || len(signals.Workflow.Items) != 1 ||
		signals.Workflow.Items[0].FilesTouched == nil || *signals.Workflow.Items[0].FilesTouched != 1 {
		t.Fatalf("work workflow = %+v", signals.Workflow)
	}
	if !signals.Tooling.Available || len(signals.Tooling.Items) != 1 || signals.Tooling.Items[0].Calls != 1 ||
		len(signals.Tooling.Items[0].Rows) != 1 || signals.Tooling.Items[0].Rows[0].Kind != "edit" || signals.Tooling.Items[0].Rows[0].Share != 100 {
		t.Fatalf("work tooling = %+v", signals.Tooling)
	}
	for _, item := range signals.Tooling.Items {
		if item.Period != "today" || item.Client != "all" {
			t.Fatalf("signal item for another selection: %+v", item)
		}
	}
	// The usage-only session is not Auxiliary: its signals never move there.
	if aux := readScope(t, s, "auxiliary", "all", "today", 5); aux.Counts.Sessions != 1 || aux.WorkSignals.Tooling.Available {
		t.Fatalf("auxiliary sessions=%d tooling=%+v", aux.Counts.Sessions, aux.WorkSignals.Tooling)
	}
}

func TestSessionScopeViewDefaultsAndUnavailableStates(t *testing.T) {
	ctx := context.Background()
	t.Run("saved default", func(t *testing.T) {
		root := scopeFixture(t)
		setDefaultScope(t, root, "auxiliary")
		result, err := scopeService(t, root).Build(ctx, Request{WireVersion: 1, RecentLimit: 5})
		if err != nil {
			t.Fatal(err)
		}
		view := result.Snapshot.SessionScopeView
		if view.Selection.Scope != "auxiliary" || *view.Default.Revision != 1 || view.Counts.Sessions != 1 {
			t.Fatalf("saved default view %+v", view)
		}
	})
	t.Run("unreadable core", func(t *testing.T) {
		root := scopeFixture(t)
		if err := os.WriteFile(filepath.Join(root, "agentdeck.sqlite3"), []byte("not a database"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := scopeService(t, root)
		result, err := s.Build(ctx, Request{WireVersion: 1, RecentLimit: 5})
		if err != nil {
			t.Fatal(err)
		}
		view := result.Snapshot.SessionScopeView
		// An unconfirmed default never becomes Work: there is no selection.
		if view.Available || view.Default.Confirmed || view.Default.Value != nil || view.Selection != nil || *view.Reason != SessionScopeReasonDefaultUnconfirmed || view.Counts != nil {
			t.Fatalf("unconfirmed default view %+v", view)
		}
		explicit := readScope(t, s, "work", "all", "today", 5)
		if explicit.Available || *explicit.Reason != sessionorigin.PublicationReasonClassificationUnavailable || explicit.Counts != nil || explicit.OriginCoverage != nil {
			t.Fatalf("unreadable classification must be unavailable, not unknown: %+v", explicit)
		}
	})
	t.Run("absent core", func(t *testing.T) {
		root := fixtureStateRoot(t)
		seedSessions(t, root, []sessionSeed{
			{client: "codex", id: "only", project: "/p/a", model: "gpt-5", first: "2026-08-13T08:00:00Z", last: "2026-08-13T09:00:00Z"},
		})
		view := readScope(t, scopeService(t, root), "work", "all", "today", 5)
		if !view.Available || view.Counts.Sessions != 1 || *view.OriginCoverage.UnknownCount != 1 || view.Recent[0].Origin.ReasonCode != sessionorigin.ReasonOriginMetadataMissing {
			t.Fatalf("a never-created core is a confirmed empty classification: %+v", view)
		}
		if _, err := os.Lstat(filepath.Join(root, "agentdeck.sqlite3")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("reading created the core: %v", err)
		}
	})
	t.Run("absent session index", func(t *testing.T) {
		root := fixtureStateRoot(t)
		seedSelections(t, root)
		view := readScope(t, scopeService(t, root), "work", "all", "today", 5)
		if view.Available || *view.Reason != sessionorigin.PublicationReasonSessionsUnavailable || !view.Default.Confirmed {
			t.Fatalf("missing index %+v", view)
		}
	})
}

func TestSessionScopeRequestValidationRejectsBeforeReading(t *testing.T) {
	root := filepath.Join(t.TempDir(), "never-created")
	s := scopeService(t, root)
	for _, request := range []SessionScopeRequest{
		{Scope: "", Client: "all", Period: "today", RecentLimit: 5},
		{Scope: "Work", Client: "all", Period: "today", RecentLimit: 5},
		{Scope: "work", Client: "", Period: "today", RecentLimit: 5},
		{Scope: "work", Client: "all", Period: "90d", RecentLimit: 5},
		{Scope: "work", Client: "all", Period: "today", RecentLimit: 0},
		{Scope: "work", Client: "all", Period: "today", RecentLimit: MaxRecentLimit + 1},
	} {
		if _, err := s.SessionScopeView(context.Background(), request); !errors.Is(err, ErrInvalidSessionScope) {
			t.Fatalf("%+v: %v", request, err)
		}
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("validation touched state: %v", err)
	}
}

// treeDigest fingerprints every regular file under root, so a read that
// writes, migrates or adds anything is detected.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digests := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(contents)
		digests[path] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digests
}

func TestSessionScopeViewReadsWithoutWriting(t *testing.T) {
	root := scopeFixture(t)
	s := scopeService(t, root)
	before := treeDigest(t, root)
	generation, err := store.ReadServingGeneration(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"work", "auxiliary", "all"} {
		for _, client := range []string{"all", "codex", "claude"} {
			for _, period := range []string{"today", "7d", "30d"} {
				readScope(t, s, scope, client, period, 5)
			}
		}
	}
	after, err := store.ReadServingGeneration(context.Background(), root)
	if err != nil || after != generation {
		t.Fatalf("generation moved: %+v -> %+v (%v)", generation, after, err)
	}
	if !reflect.DeepEqual(before, treeDigest(t, root)) {
		t.Fatal("session-scope reads changed state bytes")
	}
}

func TestServingPublicationCarriesTheViewAndTrimsItsRecentRows(t *testing.T) {
	root := scopeFixture(t)
	s := scopeService(t, root)
	ctx := context.Background()
	published, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: 5}, false)
	if err != nil {
		t.Fatal(err)
	}
	view := published.Payload.Data.SessionScopeView
	if view == nil || !view.Available || len(view.Recent) != 3 {
		t.Fatalf("published view %+v", view)
	}
	read, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	trimmed := read.Payload.Data.SessionScopeView
	if len(trimmed.Recent) != 1 || trimmed.Counts.Sessions != 3 || *trimmed.PublicationID != *view.PublicationID {
		t.Fatalf("trimmed view %+v", trimmed)
	}
	// A today-only bootstrap names the default selection but carries no
	// session data, so it is unavailable rather than legacy or empty.
	today, err := s.BuildToday(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := today.Snapshot.SessionScopeView
	if bootstrap == nil || bootstrap.Available || *bootstrap.Reason != sessionorigin.PublicationReasonSessionsUnavailable ||
		bootstrap.Selection == nil || bootstrap.Selection.Scope != "work" || bootstrap.Counts != nil {
		t.Fatalf("today-only view %+v", bootstrap)
	}
}
