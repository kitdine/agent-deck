package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestRequestValidate(t *testing.T) {
	for _, test := range []struct {
		name    string
		request Request
		want    error
	}{
		{name: "supported", request: Request{WireVersion: 1, RecentLimit: 5}},
		{name: "wire version", request: Request{WireVersion: 2, RecentLimit: 5}, want: ErrUnsupportedWireVersion},
		{name: "zero limit", request: Request{WireVersion: 1}, want: ErrInvalidRecentLimit},
		{name: "large limit", request: Request{WireVersion: 1, RecentLimit: 21}, want: ErrInvalidRecentLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSnapshotsRedactPrivateDomainFields(t *testing.T) {
	providerData := providerSnapshot([]provider.CurrentSelection{{
		Client: "codex", Provider: "example", Credential: "private-reference",
		Endpoint: "https://private.example/v1", SelectedAt: "2026-08-13T10:00:00Z",
	}})
	sessionData := sessionsSnapshot([]session.Metadata{{
		Client: "codex", SessionID: "session-1",
		Project: "/Users/example/private/agent-deck", SourcePath: "/Users/example/.codex/session.jsonl",
		Model: "gpt-5", FirstAt: "2026-08-13T09:00:00Z", LastAt: "2026-08-13T10:00:00Z",
	}}, 5, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), time.UTC)

	encoded, err := json.Marshal(struct {
		Provider ProviderSnapshot `json:"provider"`
		Sessions SessionsSnapshot `json:"sessions"`
	}{providerData, sessionData})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"private-reference", "private.example", "source_path", "/Users/example"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("snapshot contains private value %q: %s", forbidden, text)
		}
	}
	if sessionData.Items[0].Project != "agent-deck" {
		t.Fatalf("project = %q, want privacy-bounded label", sessionData.Items[0].Project)
	}
}

func TestUsageSnapshotPublishesPricingCompleteness(t *testing.T) {
	from := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	complete := usageSnapshot(usage.Summary{}, from, from.AddDate(0, 0, 1))
	if !complete.PricingComplete || complete.Warnings == nil || complete.Tokens == nil || complete.Counts == nil {
		t.Fatalf("complete usage snapshot = %#v", complete)
	}
	incomplete := usageSnapshot(usage.Summary{Unpriced: []string{"model:input_tokens"}}, from, from.AddDate(0, 0, 1))
	if incomplete.PricingComplete || incomplete.UnpricedComponents != 1 {
		t.Fatalf("incomplete usage snapshot = %#v", incomplete)
	}
}

func TestProviderCandidateOptionsResolveExactTuplesAndReasons(t *testing.T) {
	candidate := ProviderCandidate{
		Provider: "relay", Clients: []string{"codex", "claude"}, HasWrapper: true,
		Credentials: []ProviderCandidateCredential{
			{Name: "work", Clients: []string{"codex"}, Present: true},
			{Name: "missing", Clients: []string{"claude"}, Present: false},
		},
	}
	options := providerCandidateOptions(candidate, map[string]provider.CurrentSelection{
		"codex": {Client: "codex", Provider: "relay", Credential: "work", ViaWrapper: false},
	})
	if len(options) != 8 {
		t.Fatalf("options = %#v, want 8 exact client/credential/route tuples", options)
	}
	wantReasons := map[string]string{
		"codex/work/direct":     "already_selected",
		"codex/work/via":        "",
		"claude/work/direct":    "credential_client_mismatch",
		"codex/missing/direct":  "credential_client_mismatch",
		"claude/missing/direct": "credential_missing",
	}
	for _, option := range options {
		credential := ""
		if option.Credential != nil {
			credential = *option.Credential
		}
		route := "direct"
		if option.ViaWrapper {
			route = "via"
		}
		key := option.Client + "/" + credential + "/" + route
		want, checked := wantReasons[key]
		if !checked {
			continue
		}
		got := ""
		if option.ReasonCode != nil {
			got = *option.ReasonCode
		}
		if got != want || option.Ready != (want == "") {
			t.Fatalf("option %s = %#v, want reason %q", key, option, want)
		}
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"endpoint", "credential_ref", "multiplier", "wrapper_url", "private-secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("candidate contains forbidden %q: %s", forbidden, encoded)
		}
	}
}

func TestProviderCandidateWithoutWrapperKeepsDirectReadyAndExplainsVia(t *testing.T) {
	candidate := ProviderCandidate{Provider: "official", BuiltIn: true, Clients: []string{"codex"}, Credentials: []ProviderCandidateCredential{}}
	options := providerCandidateOptions(candidate, nil)
	if len(options) != 2 || !options[0].Ready || options[1].ReasonCode == nil || *options[1].ReasonCode != "wrapper_not_configured" {
		t.Fatalf("official options = %#v", options)
	}
}

func TestBuildMissingStateIsPartialWithoutCreatingDatabases(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	result, err := (Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now: func() time.Time { return now }, Location: time.UTC,
	}).Build(context.Background(), Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !result.Partial {
		t.Fatalf("Build partial = false: %#v", result)
	}
	for _, name := range []string{"agentdeck.sqlite3", "sessions.sqlite3"} {
		if _, statErr := os.Stat(filepath.Join(root, name)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("%s after Build: %v", name, statErr)
		}
	}
}

func TestBuildReadsCompleteIsolatedSnapshot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	if err = core.Close(); err != nil {
		t.Fatalf("close core: %v", err)
	}
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatalf("OpenSessions: %v", err)
	}
	if _, err = index.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		"/private/source.jsonl", "synthetic", 0, []byte{}, 0, 0, "", 1, session.ParserVersion, "2026-08-13T10:00:00Z",
	); err != nil {
		index.Close()
		t.Fatalf("seed session source: %v", err)
	}
	if _, err = index.DB.ExecContext(ctx, `INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,?,?,?,?)`,
		"/private/source.jsonl", "codex", "session-1", "/Users/example/private/agent-deck", "gpt-5", session.ParserVersion, "2026-08-13T09:00:00Z", "2026-08-13T10:00:00Z",
	); err != nil {
		index.Close()
		t.Fatalf("seed session metadata: %v", err)
	}
	if err = index.Close(); err != nil {
		t.Fatalf("close sessions: %v", err)
	}
	databaseDigests := make(map[string][sha256.Size]byte, 2)
	for _, name := range []string{"agentdeck.sqlite3", "sessions.sqlite3"} {
		path := filepath.Join(root, name)
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("sidecar %s before snapshot: %v, want not exist", path+suffix, err)
			}
		}
		databaseDigests[name] = snapshotFileDigest(t, path)
	}

	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	result, err := (Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now: func() time.Time { return now }, Location: time.UTC,
	}).Build(ctx, Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.Partial || len(result.Warnings) != 0 {
		t.Fatalf("complete snapshot = %#v", result)
	}
	if !result.Snapshot.Provider.Available || !result.Snapshot.Usage.Available || !result.Snapshot.Sessions.Available || !result.Snapshot.Health.Available {
		t.Fatalf("section availability = %#v", result.Snapshot)
	}
	if len(result.Snapshot.Provider.Candidates) != 1 || result.Snapshot.Provider.Candidates[0].Provider != "official" {
		t.Fatalf("provider candidates = %#v", result.Snapshot.Provider.Candidates)
	}
	if presentation := result.Snapshot.Usage.Presentation; !presentation.Available || len(presentation.Scopes) != 3 || len(presentation.Scopes[0].Daily.Items) != 90 || len(presentation.Scopes[0].Rhythm.Intensities) != 168 || len(presentation.ClientSubtotals.Items) != 6 {
		t.Fatalf("usage presentation bounds = %#v", presentation)
	}
	if len(result.Snapshot.Sessions.Items) != 1 || result.Snapshot.Sessions.Items[0].Project != "agent-deck" {
		t.Fatalf("sessions = %#v", result.Snapshot.Sessions)
	}
	encoded, err := json.Marshal(result.Snapshot)
	if err != nil {
		t.Fatalf("Marshal snapshot: %v", err)
	}
	if len(encoded) > 256*1024 {
		t.Fatalf("bounded desktop snapshot = %d bytes, exceeds helper capture limit", len(encoded))
	}
	for _, forbidden := range []string{"/private/source.jsonl", "/Users/example", "source_path"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("snapshot contains %q: %s", forbidden, encoded)
		}
	}
	for name, want := range databaseDigests {
		path := filepath.Join(root, name)
		if got := snapshotFileDigest(t, path); got != want {
			t.Fatalf("database %s changed during snapshot", path)
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			info, err := os.Stat(path + suffix)
			if err != nil {
				t.Fatalf("sidecar %s after snapshot: %v", path+suffix, err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("sidecar %s mode = %04o, want 0600", path+suffix, got)
			}
		}
	}
}

func snapshotFileDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return sha256.Sum256(contents)
}

func TestCanonicalFixturesDecodeAndExcludeForbiddenKeys(t *testing.T) {
	type fixtureEnvelope struct {
		SchemaVersion int       `json:"schema_version"`
		Command       string    `json:"command"`
		GeneratedAt   time.Time `json:"generated_at"`
		Data          Snapshot  `json:"data"`
		Warnings      []string  `json:"warnings"`
		Partial       bool      `json:"partial"`
		Error         *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	for _, name := range []string{"snapshot-complete.json", "snapshot-partial.json", "snapshot-empty-client.json"} {
		t.Run(name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("..", "..", "desktop", "fixtures", "v1", name))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			var envelope fixtureEnvelope
			decoder := json.NewDecoder(strings.NewReader(string(contents)))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&envelope); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if envelope.SchemaVersion != 1 || envelope.Command != "desktop.snapshot" || envelope.GeneratedAt.IsZero() || envelope.Data.WireVersion != 1 || envelope.Error != nil {
				t.Fatalf("fixture identity = %#v", envelope)
			}
			if envelope.Data.GeneratedAt == "" || envelope.Data.NextRefreshAt == "" || envelope.Data.Provider.Routes == nil || envelope.Data.Provider.Candidates == nil || envelope.Data.Usage.Tokens == nil || envelope.Data.Usage.Counts == nil || envelope.Data.Usage.Warnings == nil || envelope.Data.Usage.Presentation.Scopes == nil || envelope.Data.Usage.Presentation.ClientSubtotals.Items == nil || envelope.Data.Sessions.Items == nil || envelope.Data.Health.Checks == nil {
				t.Fatalf("fixture required fields = %#v", envelope.Data)
			}
			if name == "snapshot-complete.json" && (envelope.Partial || len(envelope.Warnings) != 0 || !envelope.Data.Provider.Available || !envelope.Data.Usage.Available || !envelope.Data.Sessions.Available || !envelope.Data.Health.Available) {
				t.Fatalf("complete fixture availability = %#v", envelope)
			}
			if name == "snapshot-partial.json" && (!envelope.Partial || len(envelope.Warnings) == 0 || envelope.Data.Provider.Available || envelope.Data.Usage.Available || envelope.Data.Sessions.Available || !envelope.Data.Health.Available) {
				t.Fatalf("partial fixture availability = %#v", envelope)
			}
			// The partial fixture used to declare an available session-period
			// family beside an unavailable session index, which is a state the
			// producer cannot reach and which hid the null-collection defect.
			if name == "snapshot-partial.json" && (envelope.Data.Sessions.Periods.Available || envelope.Data.Sessions.Periods.Items == nil || len(envelope.Data.Sessions.Periods.Items) != 0) {
				t.Fatalf("partial fixture session periods = %#v", envelope.Data.Sessions.Periods)
			}
			if name == "snapshot-empty-client.json" {
				assertEmptyClientScope(t, envelope.Data)
			}
			if name != "snapshot-partial.json" {
				assertPresentationBounds(t, envelope.Data, name == "snapshot-complete.json")
				assertSessionPeriodKeys(t, envelope.Data)
			}
			var raw any
			if err = json.Unmarshal(contents, &raw); err != nil {
				t.Fatalf("Unmarshal raw: %v", err)
			}
			assertForbiddenKeysAbsent(t, raw)
		})
	}
}

// assertPresentationBounds holds the canonical fixtures to the fixed collection
// bounds the contract states. A fixture the producer cannot emit proves nothing
// about the producer, which is what one scope, one period and one rhythm cell
// were doing here.
func assertPresentationBounds(t *testing.T, snapshot Snapshot, distinguishPeriods bool) {
	t.Helper()
	presentation := snapshot.Usage.Presentation
	if !presentation.Available || len(presentation.Scopes) != 3 {
		t.Fatalf("presentation scopes = %#v, want exactly three records", presentation.Scopes)
	}
	for index, want := range []string{"all", "codex", "claude"} {
		scope := presentation.Scopes[index]
		if scope.Client != want {
			t.Fatalf("scope %d = %q, want %q", index, scope.Client, want)
		}
		if !scope.Periods.Available {
			// An unavailable scope keeps its record with empty collections.
			continue
		}
		if got := periodNames(scope); !reflect.DeepEqual(got, []string{"today", "7d", "30d"}) {
			t.Fatalf("%s periods = %v", scope.Client, got)
		}
		if len(scope.Daily.Items) != 90 || len(scope.Rhythm.Intensities) != 168 {
			t.Fatalf("%s daily/rhythm = %d/%d, want 90/168", scope.Client, len(scope.Daily.Items), len(scope.Rhythm.Intensities))
		}
		if len(scope.Pricing.Items) != 3 {
			t.Fatalf("%s pricing = %d records, want one per period", scope.Client, len(scope.Pricing.Items))
		}
		// Copying today's figures into 7d and 30d must be visible, so the
		// complete fixture carries values that differ across periods. The
		// empty-client fixture deliberately has today-only history and is not
		// held to it.
		if distinguishPeriods && (scope.Periods.Items[0].Totals.Tokens == scope.Periods.Items[1].Totals.Tokens ||
			scope.Periods.Items[1].Totals.Tokens == scope.Periods.Items[2].Totals.Tokens) {
			t.Fatalf("%s period totals do not distinguish the periods: %#v", scope.Client, periodTotals(scope))
		}
	}
	if len(presentation.ClientSubtotals.Items) != 6 {
		t.Fatalf("client subtotals = %d, want one per period per concrete client", len(presentation.ClientSubtotals.Items))
	}
}

func periodNames(scope usage.PresentationScope) []string {
	names := make([]string, 0, len(scope.Periods.Items))
	for _, item := range scope.Periods.Items {
		names = append(names, item.Period)
	}
	return names
}

func periodTotals(scope usage.PresentationScope) []int64 {
	totals := make([]int64, 0, len(scope.Periods.Items))
	for _, item := range scope.Periods.Items {
		totals = append(totals, item.Totals.Tokens)
	}
	return totals
}

// assertSessionPeriodKeys pins the exact nine (period, client) records, so a
// producer that dropped or duplicated one cannot pass.
func assertSessionPeriodKeys(t *testing.T, snapshot Snapshot) {
	t.Helper()
	want := []string{
		"today/all", "today/codex", "today/claude",
		"7d/all", "7d/codex", "7d/claude",
		"30d/all", "30d/codex", "30d/claude",
	}
	got := make([]string, 0, len(snapshot.Sessions.Periods.Items))
	for _, item := range snapshot.Sessions.Periods.Items {
		got = append(got, item.Period+"/"+item.Client)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("session period keys = %v, want %v", got, want)
	}
}

// assertEmptyClientScope covers the contract's empty concrete client: the record
// exists, every family reports that nothing was supplied, and every collection
// is present and empty rather than null.
func assertEmptyClientScope(t *testing.T, snapshot Snapshot) {
	t.Helper()
	scopes := snapshot.Usage.Presentation.Scopes
	if len(scopes) != 3 {
		t.Fatalf("scopes = %d", len(scopes))
	}
	empty := scopes[2]
	if empty.Client != "claude" {
		t.Fatalf("third scope = %q", empty.Client)
	}
	if empty.Periods.Available || empty.Daily.Available || empty.Quality.Available ||
		empty.Pricing.Available || empty.Rhythm.Available {
		t.Fatalf("empty client families = %#v, want every family unavailable", empty)
	}
	if empty.Periods.Items == nil || empty.Daily.Items == nil || empty.Quality.Items == nil ||
		empty.Pricing.Items == nil || empty.Rhythm.Intensities == nil || empty.Rhythm.Tokens == nil ||
		empty.Rhythm.ProviderCosts == nil || empty.Rhythm.CostIncomplete == nil {
		t.Fatalf("empty client scope carries a null collection: %#v", empty)
	}
	if !scopes[1].Periods.Available {
		t.Fatalf("populated client scope = %#v, want its families available", scopes[1])
	}
}

func TestLegacyFixtureRetainsTheMissingAdditiveFields(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "desktop", "fixtures", "v1", "snapshot-legacy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data Snapshot `json:"data"`
	}
	if err = json.Unmarshal(contents, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Provider.Candidates != nil || envelope.Data.Usage.Presentation.Available {
		t.Fatalf("legacy additive fields = %#v %#v", envelope.Data.Provider.Candidates, envelope.Data.Usage.Presentation)
	}
}

func assertForbiddenKeysAbsent(t *testing.T, value any) {
	t.Helper()
	forbidden := map[string]bool{
		"credential_ref": true, "endpoint": true,
		"source_path": true, "text": true, "prompt": true, "response": true,
		"tool_arguments": true, "provider_headers": true, "config_contents": true,
	}
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				if forbidden[key] {
					t.Errorf("forbidden fixture key %q", key)
				}
				visit(nested)
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		}
	}
	visit(value)
	if t.Failed() {
		t.Logf("fixture shape: %v", reflect.TypeOf(value))
	}
}

func TestSessionsPeriodsAreProducerComputedForEveryScope(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	values := []session.Metadata{
		{Client: "codex", SessionID: "a", Project: "/tmp/one", FirstAt: "2026-08-13T09:00:00Z", LastAt: "2026-08-13T09:30:00Z"},
		{Client: "codex", SessionID: "b", Project: "/tmp/two", FirstAt: "2026-08-13T10:00:00Z", LastAt: "2026-08-13T10:10:00Z"},
		{Client: "claude", SessionID: "c", Project: "/tmp/one", FirstAt: "2026-08-10T08:00:00Z", LastAt: "2026-08-10T09:00:00Z"},
	}

	// The recent list is bounded to one entry, and the statistics must not come
	// from it: a bounded list of the newest sessions cannot answer a question
	// about a period, which is why the producer computes these.
	snapshot := sessionsSnapshot(values, 1, now, time.UTC)

	if len(snapshot.Items) != 1 || snapshot.Total != 3 {
		t.Fatalf("recent list = %d items, total %d", len(snapshot.Items), snapshot.Total)
	}
	if !snapshot.Periods.Available || len(snapshot.Periods.Items) != 9 {
		t.Fatalf("periods = %#v, want one record per period per client scope", snapshot.Periods)
	}
	byKey := map[string]SessionsPeriodItem{}
	for _, item := range snapshot.Periods.Items {
		byKey[item.Period+"/"+item.Client] = item
	}
	today := byKey["today/all"]
	if today.Sessions != 2 || today.DistinctProjects != 2 {
		t.Fatalf("today/all = %#v", today)
	}
	if today.TotalDurationSeconds != 2400 || today.MedianDurationSeconds != 1200 {
		t.Fatalf("today/all durations = %#v", today)
	}
	if !reflect.DeepEqual(today.Projects, []SessionsProjectItem{
		{Project: "one", Sessions: 1, DurationSeconds: 1800},
		{Project: "two", Sessions: 1, DurationSeconds: 600},
	}) {
		t.Fatalf("today/all projects = %#v", today.Projects)
	}
	if got := byKey["today/claude"]; got.Sessions != 0 || got.MedianDurationSeconds != 0 {
		t.Fatalf("today/claude = %#v, want an empty record rather than a missing one", got)
	}
	if got := byKey["today/claude"].Projects; got == nil || len(got) != 0 {
		t.Fatalf("today/claude projects = %#v, want a non-nil empty array", got)
	}
	if got := byKey["7d/claude"]; got.Sessions != 1 || got.TotalDurationSeconds != 3600 {
		t.Fatalf("7d/claude = %#v", got)
	}
}

// PPS-F1. An unavailable session index used to leave SessionsPeriods.Items nil,
// which encoding/json writes as `null`. The Swift decoder accepts the family
// being absent but rejects a present family whose items is null, so the whole
// snapshot failed to decode instead of reaching the unavailable panel state.
func TestUnavailableSessionIndexEncodesEmptyArraysRatherThanNull(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	result, err := (Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now: func() time.Time { return now }, Location: time.UTC,
	}).Build(context.Background(), Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.Snapshot.Sessions.Available {
		t.Fatalf("sessions availability = true, want the unavailable path")
	}

	encoded, err := json.Marshal(result.Snapshot)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"items":null`) {
		t.Fatalf("snapshot encodes a null collection: %s", encoded)
	}

	var decoded map[string]any
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	sessions, ok := decoded["sessions"].(map[string]any)
	if !ok {
		t.Fatalf("sessions = %#v", decoded["sessions"])
	}
	periods, ok := sessions["periods"].(map[string]any)
	if !ok {
		t.Fatalf("sessions.periods = %#v", sessions["periods"])
	}
	items, ok := periods["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("sessions.periods.items = %#v, want an empty array", periods["items"])
	}
	if recent, ok := sessions["items"].([]any); !ok || len(recent) != 0 {
		t.Fatalf("sessions.items = %#v, want an empty array", sessions["items"])
	}
}

// PPS-F4. Distinct projects are counted on the normalized full identity. Two
// checkouts sharing a basename are two projects, and a session with no project
// is not a project at all.
func TestDistinctProjectsCountIdentitiesRatherThanDisplayBasenames(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	values := []session.Metadata{
		{Client: "codex", SessionID: "a", Project: "/work/one/agent-deck", FirstAt: "2026-08-13T09:00:00Z", LastAt: "2026-08-13T09:30:00Z"},
		{Client: "codex", SessionID: "b", Project: "/work/two/agent-deck", FirstAt: "2026-08-13T10:00:00Z", LastAt: "2026-08-13T10:10:00Z"},
		{Client: "codex", SessionID: "c", Project: "", FirstAt: "2026-08-13T11:00:00Z", LastAt: "2026-08-13T11:10:00Z"},
	}

	snapshot := sessionsSnapshot(values, 5, now, time.UTC)
	byKey := map[string]SessionsPeriodItem{}
	for _, item := range snapshot.Periods.Items {
		byKey[item.Period+"/"+item.Client] = item
	}
	if got := byKey["today/all"]; got.Sessions != 3 || got.DistinctProjects != 2 {
		t.Fatalf("today/all = %#v, want 3 sessions across 2 distinct project identities", got)
	}

	// The display rows keep the basename projection, which is what makes the
	// two concerns separable rather than one call site serving both.
	if snapshot.Items[0].Project != "agent-deck" {
		t.Fatalf("recent row project = %q, want the display basename", snapshot.Items[0].Project)
	}

	unattributed := sessionsSnapshot(values[2:], 5, now, time.UTC)
	for _, item := range unattributed.Periods.Items {
		if item.DistinctProjects != 0 {
			t.Fatalf("%s/%s distinct projects = %d, want 0 for an unattributed session", item.Period, item.Client, item.DistinctProjects)
		}
	}
}

func TestChatGPTWorkConversationDirectoriesShareOneProjectAggregate(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	values := []session.Metadata{
		{Client: "codex", SessionID: "a", Project: "/work/2026-08-11/referenced-chatgpt-conversation-this-is-untrusted", FirstAt: "2026-08-13T08:00:00Z", LastAt: "2026-08-13T09:00:00Z"},
		{Client: "codex", SessionID: "b", Project: "/work/2026-08-12/referenced-chatgpt-conversation-this-is-an", FirstAt: "2026-08-13T09:00:00Z", LastAt: "2026-08-13T09:30:00Z"},
		{Client: "codex", SessionID: "c", Project: "/other/referenced-chatgpt-conversation-this-is-untrusted", FirstAt: "2026-08-13T10:00:00Z", LastAt: "2026-08-13T10:15:00Z"},
	}

	snapshot := sessionsSnapshot(values, 5, now, time.UTC)
	var today SessionsPeriodItem
	for _, item := range snapshot.Periods.Items {
		if item.Period == "today" && item.Client == "all" {
			today = item
			break
		}
	}
	if today.DistinctProjects != 1 {
		t.Fatalf("distinct projects = %d, want one ChatGPT Work project", today.DistinctProjects)
	}
	want := []SessionsProjectItem{{Project: "ChatGPT Work", Sessions: 3, DurationSeconds: 6300}}
	if !reflect.DeepEqual(today.Projects, want) {
		t.Fatalf("projects = %#v, want %#v", today.Projects, want)
	}
}

// PPS-F5. Period membership is half-open on the local calendar. A session whose
// last event is after the current local day must fall outside every period
// rather than being counted by all three.
func TestSessionPeriodsExcludeEventsAfterTheLocalDayEnd(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	values := []session.Metadata{
		{Client: "codex", SessionID: "future", Project: "/work/one", FirstAt: "2026-08-14T09:00:00Z", LastAt: "2026-08-14T09:30:00Z"},
		{Client: "codex", SessionID: "boundary", Project: "/work/two", FirstAt: "2026-08-13T23:00:00Z", LastAt: "2026-08-13T23:59:59Z"},
	}

	snapshot := sessionsSnapshot(values, 5, now, time.UTC)
	for _, item := range snapshot.Periods.Items {
		if item.Client == "claude" {
			continue
		}
		if item.Sessions != 1 {
			t.Fatalf("%s/%s sessions = %d, want only the session inside the local day", item.Period, item.Client, item.Sessions)
		}
		if item.DistinctProjects != 1 {
			t.Fatalf("%s/%s distinct projects = %d", item.Period, item.Client, item.DistinctProjects)
		}
	}
}

// PPS-F5. A period spans the intended number of calendar days across a DST
// transition, because both bounds come from calendar arithmetic on a local
// start-of-day rather than from a fixed multiple of 24 hours.
func TestSessionPeriodsUseCalendarDaysAcrossADaylightSavingTransition(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 2026-11-01 is the local end of daylight saving time in this zone.
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, location)
	values := []session.Metadata{
		// Local 2026-10-28, inside the 7-day window that spans the transition.
		{Client: "codex", SessionID: "inside", Project: "/work/one", FirstAt: "2026-10-28T17:00:00Z", LastAt: "2026-10-28T18:00:00Z"},
		// Local 2026-10-27, one day before that window opens.
		{Client: "codex", SessionID: "outside", Project: "/work/two", FirstAt: "2026-10-27T17:00:00Z", LastAt: "2026-10-27T18:00:00Z"},
	}

	snapshot := sessionsSnapshot(values, 5, now, location)
	byKey := map[string]SessionsPeriodItem{}
	for _, item := range snapshot.Periods.Items {
		byKey[item.Period+"/"+item.Client] = item
	}
	if got := byKey["7d/all"]; got.Sessions != 1 {
		t.Fatalf("7d/all = %#v, want the 7-day window to span exactly seven local days", got)
	}
	if got := byKey["30d/all"]; got.Sessions != 2 {
		t.Fatalf("30d/all = %#v, want both sessions inside the 30-day window", got)
	}
	if got := byKey["today/all"]; got.Sessions != 0 {
		t.Fatalf("today/all = %#v", got)
	}
}
