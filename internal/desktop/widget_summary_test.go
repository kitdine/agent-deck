package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

// Task 7's Widget batch reads every supported client × period at one pinned
// generation and one instant. Every state root here is an isolated fixture
// under t.TempDir().

var widgetBatchClients = []string{"all", "codex", "claude"}
var widgetBatchPeriods = []string{"today", "7d", "30d"}

// widgetBatchSeed is one synthetic session: its client, class and how many
// days before fixtureNow's day its last event is.
type widgetBatchSeed struct {
	client    string
	auxiliary bool
	daysAgo   int
}

// widgetBatchSeeds gives each client deliberately different counts in every
// period and scope, so a value read for the wrong client or window cannot
// match by accident:
//
//	auxiliary: codex 1/2/5, claude 3/6/7, all 4/8/12
//	work:      codex 2/4/5, claude 1/6/7, all 3/10/12
//	all:       codex 3/6/10, claude 4/12/14, all 7/18/24
func widgetBatchSeeds() []widgetBatchSeed {
	seeds := []widgetBatchSeed{}
	add := func(client string, auxiliary bool, daysAgo, count int) {
		for range count {
			seeds = append(seeds, widgetBatchSeed{client: client, auxiliary: auxiliary, daysAgo: daysAgo})
		}
	}
	add("codex", true, 0, 1)
	add("codex", true, 3, 1)
	add("codex", true, 20, 3)
	add("codex", false, 0, 2)
	add("codex", false, 3, 2)
	add("codex", false, 20, 1)
	add("claude", true, 0, 3)
	add("claude", true, 3, 3)
	add("claude", true, 20, 1)
	add("claude", false, 0, 1)
	add("claude", false, 3, 5)
	add("claude", false, 20, 1)
	return seeds
}

// setWidgetDefault saves the default scope with a request ID of its own, so
// one fixture can save several defaults in turn.
func setWidgetDefault(t *testing.T, root, value string) {
	t.Helper()
	ctx := context.Background()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer core.Close()
	service := sessionorigin.Service{Store: core}
	current := service.DefaultScope(ctx)
	if _, err := service.SetDefaultScope(ctx, sessionorigin.ScopeRequest{RequestID: "widget-default-" + value, Value: value, ExpectedRevision: *current.Revision}); err != nil {
		t.Fatalf("SetDefaultScope(%s): %v", value, err)
	}
}

func widgetBatchState(t *testing.T) string {
	t.Helper()
	root := fixtureStateRoot(t)
	sessions := []sessionSeed{}
	origins := []originSeed{}
	for index, seed := range widgetBatchSeeds() {
		id := fmt.Sprintf("%s-%02d", seed.client, index)
		last := fixtureNow.AddDate(0, 0, -seed.daysAgo).Add(-30 * time.Minute)
		sessions = append(sessions, sessionSeed{
			client: seed.client, id: id, project: "/Users/example/private/project", model: "synthetic",
			first: last.Add(-20 * time.Minute).Format(time.RFC3339), last: last.Format(time.RFC3339),
		})
		if seed.auxiliary {
			origins = append(origins, originSeed{client: seed.client, id: id, automatic: "auxiliary", reason: sessionorigin.ReasonMaintenanceOrigin, rule: "codex-otel-background-v1"})
		}
	}
	seedSessions(t, root, sessions)
	seedOrigins(t, root, origins)
	return root
}

// widgetBatchOracle counts the seeds directly — not through the selection
// service — for one scope, client and period. Unclassified sessions are
// unknown and stay in Work, so Work's unknown count is its whole count.
func widgetBatchOracle(scope, client, period string) (count, unknown int) {
	days := map[string]int{"today": 1, "7d": 7, "30d": 30}[period]
	for _, seed := range widgetBatchSeeds() {
		if (client != "all" && seed.client != client) || seed.daysAgo >= days {
			continue
		}
		if (scope == "work" && seed.auxiliary) || (scope == "auxiliary" && !seed.auxiliary) {
			continue
		}
		count++
		if !seed.auxiliary {
			unknown++
		}
	}
	return count, unknown
}

// readWidgetBatch reads the nine selections the way the host does: the first
// read fixes the generation, every later one pins it, all at one instant.
func readWidgetBatch(t *testing.T, s Service, scope string, asOf time.Time) []SessionScopeView {
	t.Helper()
	views := []SessionScopeView{}
	generation := ""
	for _, client := range widgetBatchClients {
		for _, period := range widgetBatchPeriods {
			view, err := s.SessionScopeView(context.Background(), SessionScopeRequest{
				Scope: scope, Client: client, Period: period, RecentLimit: 1, GenerationID: generation, AsOf: asOf,
			})
			if err != nil {
				t.Fatalf("%s/%s/%s: %v", scope, client, period, err)
			}
			if !view.Available || view.GenerationID == nil {
				t.Fatalf("%s/%s/%s unavailable: %+v", scope, client, period, view)
			}
			generation = *view.GenerationID
			views = append(views, view)
		}
	}
	return views
}

func TestWidgetBatchMatchesTheSelectionOracleForEveryClientAndPeriod(t *testing.T) {
	root := widgetBatchState(t)
	s := scopeService(t, root)
	for _, scope := range []string{"work", "auxiliary", "all"} {
		views := readWidgetBatch(t, s, scope, fixtureNow)
		generation := *views[0].GenerationID
		seen := map[int]string{}
		for index, view := range views {
			client, period := widgetBatchClients[index/3], widgetBatchPeriods[index%3]
			want, wantUnknown := widgetBatchOracle(scope, client, period)
			if view.Selection.Scope != scope || view.Selection.Client != client || view.Selection.Period != period {
				t.Fatalf("view %d answers %+v", index, view.Selection)
			}
			if view.Counts.Sessions != want || *view.OriginCoverage.UnknownCount != wantUnknown || !view.OriginCoverage.Complete {
				t.Fatalf("%s/%s/%s = %d (unknown %d), oracle %d (unknown %d)", scope, client, period,
					view.Counts.Sessions, *view.OriginCoverage.UnknownCount, want, wantUnknown)
			}
			if *view.GenerationID != generation || *view.GeneratedAt != fixtureNow.Format(time.RFC3339Nano) {
				t.Fatalf("%s/%s/%s left the batch: generation %s at %s", scope, client, period, *view.GenerationID, *view.GeneratedAt)
			}
			if other, ok := seen[want]; ok {
				t.Fatalf("%s: %s/%s and %s share count %d; the fixture must keep them distinct", scope, client, period, other, want)
			}
			seen[want] = client + "/" + period
		}
	}
	// Work + Auxiliary = All for every combination of one generation.
	work, aux, all := readWidgetBatch(t, s, "work", fixtureNow), readWidgetBatch(t, s, "auxiliary", fixtureNow), readWidgetBatch(t, s, "all", fixtureNow)
	for index := range all {
		if work[index].Counts.Sessions+aux[index].Counts.Sessions != all[index].Counts.Sessions {
			t.Fatalf("combination %d: work %d + aux %d != all %d", index, work[index].Counts.Sessions, aux[index].Counts.Sessions, all[index].Counts.Sessions)
		}
	}
}

func TestSessionScopeGenerationIsSharedAcrossSelectionsAndMatchesTheReader(t *testing.T) {
	root := widgetBatchState(t)
	s := scopeService(t, root)
	views := readWidgetBatch(t, s, "auxiliary", fixtureNow)
	ids := map[string]bool{}
	for _, view := range views {
		ids[*view.PublicationID] = true
	}
	// Publication IDs name each selection; the generation ID names only the
	// committed generation, so it is what a batch can pin and compare.
	if len(ids) != len(views) {
		t.Fatalf("publication IDs are not per selection: %d distinct of %d", len(ids), len(views))
	}
	generation, err := store.ReadServingGeneration(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := sessionorigin.GenerationID(sessionorigin.Vector{
		CoreEpoch: generation.CoreEpoch, UsageRevision: generation.UsageRevision,
		SessionEpoch: generation.SessionEpoch, SessionRevision: generation.SessionRevision,
	})
	if *views[0].GenerationID != want || !isGenerationID(want) {
		t.Fatalf("generation %s, want the serving generation's %s", *views[0].GenerationID, want)
	}
}

// A pinned generation that is no longer committed is unavailable: Go never
// answers a pinned read from a newer generation.
func TestPinnedGenerationIsUnavailableOnceTheGenerationMoves(t *testing.T) {
	root := widgetBatchState(t)
	s := scopeService(t, root)
	first := readScope(t, s, "auxiliary", "all", "today", 1)
	pinned := *first.GenerationID
	// Saving a default is a committed core write: the generation moves.
	setDefaultScope(t, root, "all")
	view, err := s.SessionScopeView(context.Background(), SessionScopeRequest{
		Scope: "auxiliary", Client: "codex", Period: "7d", RecentLimit: 1, GenerationID: pinned,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Available || view.Counts != nil || view.GenerationID != nil || view.Reason == nil ||
		*view.Reason != sessionorigin.PublicationReasonGenerationUnavailable {
		t.Fatalf("pinned read of a moved generation = %+v", view)
	}
	if view.Default.Value == nil || *view.Default.Value != "all" {
		t.Fatalf("the unavailable view still reports the current default: %+v", view.Default)
	}
	latest := readScope(t, s, "auxiliary", "codex", "7d", 1)
	if *latest.GenerationID == pinned {
		t.Fatal("an unpinned read after the write kept the old generation")
	}
	// The same pin read at the current generation answers.
	again, err := s.SessionScopeView(context.Background(), SessionScopeRequest{
		Scope: "auxiliary", Client: "claude", Period: "30d", RecentLimit: 1, GenerationID: *latest.GenerationID,
	})
	if err != nil || !again.Available || *again.GenerationID != *latest.GenerationID {
		t.Fatalf("current pin = %+v, %v", again, err)
	}
}

// AsOf fixes both the window and generated_at, so a batch read across
// midnight cannot mix two days.
func TestAsOfFixesTheWindowAndTheTime(t *testing.T) {
	root := widgetBatchState(t)
	s := scopeService(t, root)
	nextDay := fixtureNow.AddDate(0, 0, 1)
	view, err := s.SessionScopeView(context.Background(), SessionScopeRequest{
		Scope: "all", Client: "all", Period: "today", RecentLimit: 1, AsOf: nextDay,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Available || view.Counts.Sessions != 0 || *view.GeneratedAt != nextDay.Format(time.RFC3339Nano) {
		t.Fatalf("as-of next day = %+v counts %+v", view, view.Counts)
	}
	week, err := s.SessionScopeView(context.Background(), SessionScopeRequest{
		Scope: "all", Client: "all", Period: "7d", RecentLimit: 1, AsOf: nextDay,
	})
	if err != nil {
		t.Fatal(err)
	}
	// From the next day, 7d still holds today's and three-days-ago's sessions.
	if want, _ := widgetBatchOracle("all", "all", "7d"); week.Counts.Sessions != want {
		t.Fatalf("7d as of next day = %d, want %d", week.Counts.Sessions, want)
	}
}

func TestSessionScopeRequestRejectsAMalformedGenerationID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "never-created")
	s := scopeService(t, root)
	for _, generation := range []string{"abc", "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789", fmt.Sprintf("%064d", 0) + "0"} {
		_, err := s.SessionScopeView(context.Background(), SessionScopeRequest{Scope: "work", Client: "all", Period: "today", RecentLimit: 1, GenerationID: generation})
		if err == nil {
			t.Fatalf("generation %q accepted", generation)
		}
	}
	if _, err := os.Lstat(root); err == nil {
		t.Fatal("validation touched state")
	}
}

// widgetBatchFixture is the Go half of Task 7's joint fixture: the envelopes
// `desktop session-scope` prints for each batch, plus the oracle. The Swift
// tests feed these exact envelopes through DesktopWire, the host collector,
// the App Group store and the Widget reader.
type widgetBatchFixture struct {
	AsOf string `json:"as_of"`
	// Batches are keyed by the confirmed default the host read.
	Batches map[string][]json.RawMessage `json:"batches"`
	// Popover is a temporary Work selection, which must never feed the batch.
	Popover json.RawMessage                       `json:"popover"`
	Oracle  map[string]map[string]widgetOracleRow `json:"oracle"`
}

type widgetOracleRow struct {
	Count   int `json:"count"`
	Unknown int `json:"unknown"`
}

func buildWidgetBatchFixture(t *testing.T) string {
	t.Helper()
	root := widgetBatchState(t)
	s := scopeService(t, root)
	setWidgetDefault(t, root, "auxiliary")
	fixture := widgetBatchFixture{AsOf: fixtureNow.Format(time.RFC3339), Batches: map[string][]json.RawMessage{}, Oracle: map[string]map[string]widgetOracleRow{}}
	encode := func(view SessionScopeView, generation string) json.RawMessage {
		if view.PublicationID != nil {
			id := fixtureSessionScopePublication
			view.PublicationID = &id
		}
		if view.GenerationID != nil {
			view.GenerationID = &generation
		}
		envelope := output.New("desktop.session-scope", view, fixtureNow)
		envelope.Partial = !view.Available
		envelope.Warnings = []string{}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	record := func(scope string) {
		for index, view := range readWidgetBatch(t, s, scope, fixtureNow) {
			client, period := widgetBatchClients[index/3], widgetBatchPeriods[index%3]
			fixture.Batches[scope] = append(fixture.Batches[scope], encode(view, fixtureSessionScopeGeneration+"-"+scope))
			count, unknown := widgetBatchOracle(scope, client, period)
			if scope == "auxiliary" {
				unknown = 0
			}
			if fixture.Oracle[scope] == nil {
				fixture.Oracle[scope] = map[string]widgetOracleRow{}
			}
			fixture.Oracle[scope][client+"/"+period] = widgetOracleRow{Count: count, Unknown: unknown}
		}
	}
	record("auxiliary")
	fixture.Popover = encode(readScope(t, s, "work", "all", "today", 1), fixtureSessionScopeGeneration+"-auxiliary")
	setWidgetDefault(t, root, "all")
	record("all")
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded) + "\n"
}

func TestWidgetBatchFixtureIsReproducibleProducerOutput(t *testing.T) {
	generated := buildWidgetBatchFixture(t)
	path := filepath.Join("..", "..", "desktop", "fixtures", "v1", "session-scope-batch.json")
	if os.Getenv("AGENTDECK_UPDATE_FIXTURES") == "1" {
		if err := os.WriteFile(path, []byte(generated), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(existing) != generated {
		t.Fatalf("session-scope-batch.json does not match producer output at %s; regenerate with AGENTDECK_UPDATE_FIXTURES=1", firstDifference(string(existing), generated))
	}
	var decoded widgetBatchFixture
	if err := json.Unmarshal(existing, &decoded); err != nil || !reflect.DeepEqual(len(decoded.Batches["all"]), 9) {
		t.Fatalf("fixture shape: %v", err)
	}
}
