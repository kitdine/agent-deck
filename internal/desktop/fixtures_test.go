package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// fixtureNow is the instant every canonical fixture is generated at, so the
// files stay byte-reproducible.
var fixtureNow = time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

// The canonical fixtures are producer output rather than hand-written examples.
// A hand-written fixture can satisfy a decoder while being a payload the
// producer cannot emit — which is how a complete fixture came to carry one
// scope where the contract requires three, and how a partial fixture came to
// declare an available session-period family beside an unavailable session
// index, hiding the null-collection defect that blocked real decoding.
func TestCanonicalFixturesAreReproducibleProducerOutput(t *testing.T) {
	update := os.Getenv("AGENTDECK_UPDATE_FIXTURES") == "1"
	for _, fixture := range []struct {
		name  string
		build func(*testing.T) string
	}{
		{name: "snapshot-complete.json", build: buildCompleteFixture},
		{name: "snapshot-partial.json", build: buildPartialFixture},
		{name: "snapshot-empty-client.json", build: buildEmptyClientFixture},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			generated := fixture.build(t)
			path := filepath.Join("..", "..", "desktop", "fixtures", "v1", fixture.name)
			if update {
				if err := os.WriteFile(path, []byte(generated), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				t.Logf("updated %s", path)
				return
			}
			existing, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(existing) != generated {
				t.Fatalf(
					"%s does not match producer output at %s; regenerate with AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop",
					fixture.name, firstDifference(string(existing), generated),
				)
			}
		})
	}
}

// firstDifference names the first differing line so a mismatch is diagnosable
// from the failure alone rather than by regenerating and diffing by hand.
func firstDifference(existing, generated string) string {
	existingLines := strings.Split(existing, "\n")
	generatedLines := strings.Split(generated, "\n")
	for index := 0; index < len(existingLines) && index < len(generatedLines); index++ {
		if existingLines[index] != generatedLines[index] {
			return fmt.Sprintf("line %d: have %q, want %q", index+1, existingLines[index], generatedLines[index])
		}
	}
	return fmt.Sprintf("line %d: length %d vs %d", min(len(existingLines), len(generatedLines))+1, len(existingLines), len(generatedLines))
}

func encodeFixture(t *testing.T, result Result) string {
	t.Helper()
	envelope := output.New("desktop.snapshot", result.Snapshot, fixtureNow)
	envelope.Partial, envelope.Warnings = result.Partial, result.Warnings
	if envelope.Warnings == nil {
		envelope.Warnings = []string{}
	}
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(envelope); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return builder.String()
}

// fixtureStateRoot creates the state root explicitly at 0700. Doctor reports on
// the directory's mode, so leaving the root to be created implicitly made the
// health section depend on ambient filesystem state and the fixture stopped
// being reproducible. A real state root is 0700; the fixture uses the same.
func fixtureStateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatalf("Mkdir state root: %v", err)
	}
	return root
}

func buildFixtureResult(t *testing.T, root string) Result {
	t.Helper()
	result, err := (Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now: func() time.Time { return fixtureNow }, Location: time.UTC,
	}).Build(context.Background(), Request{WireVersion: 1, RecentLimit: 5})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return result
}

// buildCompleteFixture seeds both clients with values that differ per client and
// per period, so a producer that copied today's figures into 7d and 30d, or one
// client's into the other, cannot leave this fixture unchanged.
func buildCompleteFixture(t *testing.T) string {
	t.Helper()
	root := fixtureStateRoot(t)
	seedSelections(t, root)
	seedUsage(t, root, []usageSeed{
		{client: "codex", session: "codex-today", at: "2026-08-13T08:00:00Z", model: "gpt-5", input: 1200, output: 240},
		{client: "codex", session: "codex-week", at: "2026-08-10T08:00:00Z", model: "gpt-5", input: 800, output: 160},
		{client: "codex", session: "codex-month", at: "2026-07-28T08:00:00Z", model: "gpt-5-mini", input: 400, output: 80},
		{client: "claude", session: "claude-today", at: "2026-08-13T09:00:00Z", model: "claude-sonnet-5", input: 300, output: 60},
		{client: "claude", session: "claude-week", at: "2026-08-09T09:00:00Z", model: "claude-opus-5", input: 150, output: 30},
		{client: "claude", session: "claude-month", at: "2026-07-25T09:00:00Z", model: "claude-opus-5", input: 90, output: 18},
	})
	seedSessions(t, root, []sessionSeed{
		{client: "codex", id: "codex-today", project: "/Users/example/private/agent-deck", model: "gpt-5", first: "2026-08-13T08:00:00Z", last: "2026-08-13T09:00:00Z"},
		{client: "codex", id: "codex-other", project: "/Users/example/private/other/agent-deck", model: "gpt-5", first: "2026-08-13T09:10:00Z", last: "2026-08-13T09:40:00Z"},
		{client: "codex", id: "codex-week", project: "/Users/example/private/agent-deck", model: "gpt-5", first: "2026-08-10T08:00:00Z", last: "2026-08-10T08:45:00Z"},
		{client: "claude", id: "claude-today", project: "/Users/example/private/notes", model: "claude-sonnet-5", first: "2026-08-13T09:00:00Z", last: "2026-08-13T09:20:00Z"},
		{client: "claude", id: "claude-month", project: "/Users/example/private/notes", model: "claude-opus-5", first: "2026-07-25T09:00:00Z", last: "2026-07-25T10:00:00Z"},
	})
	return encodeFixture(t, buildFixtureResult(t, root))
}

// buildPartialFixture is the real degraded state: nothing in the state root, so
// every domain reports unavailable and every collection is an empty array.
func buildPartialFixture(t *testing.T) string {
	t.Helper()
	return encodeFixture(t, buildFixtureResult(t, fixtureStateRoot(t)))
}

// buildEmptyClientFixture covers the contract's empty concrete client: the
// record exists, its families report that nothing was supplied, and no
// synthetic zero is presented as a measurement.
func buildEmptyClientFixture(t *testing.T) string {
	t.Helper()
	root := fixtureStateRoot(t)
	seedUsage(t, root, []usageSeed{
		{client: "codex", session: "codex-today", at: "2026-08-13T08:00:00Z", model: "gpt-5", input: 1200, output: 240},
	})
	seedSessions(t, root, []sessionSeed{
		{client: "codex", id: "codex-today", project: "/Users/example/private/agent-deck", model: "gpt-5", first: "2026-08-13T08:00:00Z", last: "2026-08-13T09:00:00Z"},
	})
	return encodeFixture(t, buildFixtureResult(t, root))
}

type usageSeed struct {
	client  string
	session string
	at      string
	model   string
	input   int64
	output  int64
}

type sessionSeed struct {
	client  string
	id      string
	project string
	model   string
	first   string
	last    string
}

// seedSelections records a current route per client so the footer's provider
// state and the already_selected option reason are both exercised.
func seedSelections(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("close core: %v", closeErr)
		}
	}()
	for _, client := range []string{"codex", "claude"} {
		if err = database.RecordSelection(ctx, store.Selection{
			Client: client, ProviderName: "official", MultiplierSnapshot: "1",
			SelectedAt: time.Date(2026, 8, 13, 9, 55, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("record selection: %v", err)
		}
	}
}

func seedUsage(t *testing.T, root string, seeds []usageSeed) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("close core: %v", closeErr)
		}
	}()
	for index, seed := range seeds {
		if _, err = database.Exec(ctx,
			`INSERT OR IGNORE INTO usage_sessions(client,session_id,first_at,last_at) VALUES(?,?,?,?)`,
			seed.client, seed.session, seed.at, seed.at,
		); err != nil {
			t.Fatalf("seed usage session: %v", err)
		}
		if _, err = database.Exec(ctx,
			`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			seed.session+"-event", seed.client, seed.session, seed.session, seed.at, seed.model,
			seed.input, seed.output, "fixture", index,
		); err != nil {
			t.Fatalf("seed usage event: %v", err)
		}
	}
}

func seedSessions(t *testing.T, root string, seeds []sessionSeed) {
	t.Helper()
	ctx := context.Background()
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatalf("OpenSessions: %v", err)
	}
	if _, err = index.DB.ExecContext(ctx,
		`INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		"/private/source.jsonl", "synthetic", 0, []byte{}, 0, 0, "", 1, session.ParserVersion, "2026-08-13T10:00:00Z",
	); err != nil {
		index.Close()
		t.Fatalf("seed session source: %v", err)
	}
	for _, seed := range seeds {
		if _, err = index.DB.ExecContext(ctx,
			`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,?,?,?,?)`,
			"/private/source.jsonl", seed.client, seed.id, seed.project, seed.model, session.ParserVersion, seed.first, seed.last,
		); err != nil {
			index.Close()
			t.Fatalf("seed session metadata: %v", err)
		}
	}
	if err = index.Close(); err != nil {
		t.Fatalf("close sessions: %v", err)
	}
}
