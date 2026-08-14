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
	}}, 5)

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
	if len(result.Snapshot.Sessions.Items) != 1 || result.Snapshot.Sessions.Items[0].Project != "agent-deck" {
		t.Fatalf("sessions = %#v", result.Snapshot.Sessions)
	}
	encoded, err := json.Marshal(result.Snapshot)
	if err != nil {
		t.Fatalf("Marshal snapshot: %v", err)
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
	for _, name := range []string{"snapshot-complete.json", "snapshot-partial.json"} {
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
			if envelope.Data.GeneratedAt == "" || envelope.Data.NextRefreshAt == "" || envelope.Data.Provider.Routes == nil || envelope.Data.Usage.Tokens == nil || envelope.Data.Usage.Counts == nil || envelope.Data.Usage.Warnings == nil || envelope.Data.Sessions.Items == nil || envelope.Data.Health.Checks == nil {
				t.Fatalf("fixture required fields = %#v", envelope.Data)
			}
			if name == "snapshot-complete.json" && (envelope.Partial || len(envelope.Warnings) != 0 || !envelope.Data.Provider.Available || !envelope.Data.Usage.Available || !envelope.Data.Sessions.Available || !envelope.Data.Health.Available) {
				t.Fatalf("complete fixture availability = %#v", envelope)
			}
			if name == "snapshot-partial.json" && (!envelope.Partial || len(envelope.Warnings) == 0 || envelope.Data.Provider.Available || envelope.Data.Usage.Available || envelope.Data.Sessions.Available || !envelope.Data.Health.Available) {
				t.Fatalf("partial fixture availability = %#v", envelope)
			}
			var raw any
			if err = json.Unmarshal(contents, &raw); err != nil {
				t.Fatalf("Unmarshal raw: %v", err)
			}
			assertForbiddenKeysAbsent(t, raw)
		})
	}
}

func assertForbiddenKeysAbsent(t *testing.T, value any) {
	t.Helper()
	forbidden := map[string]bool{
		"credential": true, "credential_ref": true, "endpoint": true,
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
