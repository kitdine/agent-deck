package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestUsageHookEventRejectsInvalidTranscriptAndNonBoundaryEvents(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
		VALUES(1,'codex','official','x','1','2026-08-04T00:00:00Z');
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(home, ".codex", "sessions", "valid-session.jsonl")
	if err = os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mismatchedTranscript := filepath.Join(filepath.Dir(transcript), "other-session.jsonl")
	if err = os.WriteFile(mismatchedTranscript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	outsideTranscript := filepath.Join(home, "outside-valid-session.jsonl")
	if err = os.WriteFile(outsideTranscript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkTranscript := filepath.Join(filepath.Dir(transcript), "valid-session-link.jsonl")
	if err = os.Symlink(outsideTranscript, symlinkTranscript); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	deliver := func(eventName, source, path string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      "valid-session",
			"transcript_path": path,
			"hook_event_name": eventName,
			"source":          source,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var stdout bytes.Buffer
		if err := run([]string{"--state-dir", state, "usage", "hook", "event", "codex"}, bytes.NewReader(payload), &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook event stdout = %q", stdout.String())
		}
	}
	countRoutes := func() int {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		var count int
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&count); queryErr != nil {
			t.Fatal(queryErr)
		}
		return count
	}
	countObservations := func() int {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		var count int
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&count); queryErr != nil {
			t.Fatal(queryErr)
		}
		return count
	}

	deliver("SessionStart", "resume", filepath.Join(home, "outside", "valid-session.jsonl"))
	deliver("SessionStart", "resume", mismatchedTranscript)
	deliver("SessionStart", "resume", symlinkTranscript)
	deliver("ConfigChange", "", transcript)
	if got := countRoutes(); got != 0 {
		t.Fatalf("invalid events wrote %d routes", got)
	}
	if got := countObservations(); got != 0 {
		t.Fatalf("invalid events wrote %d observations", got)
	}
	deliver("SessionStart", "resume", transcript)
	if got := countRoutes(); got != 1 {
		t.Fatalf("valid event wrote %d routes, want 1", got)
	}
	if got := countObservations(); got != 1 {
		t.Fatalf("valid event wrote %d observations, want 1", got)
	}
}

func TestUsageHookEventRejectsUnmanagedClaudeConfigChangeSources(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err = os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	deliver := func(source, path string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      "valid-session",
			"hook_event_name": "ConfigChange",
			"source":          source,
			"file_path":       path,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var stdout bytes.Buffer
		if err := run([]string{"--state-dir", state, "usage", "hook", "event", "claude"}, bytes.NewReader(payload), &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook event stdout = %q", stdout.String())
		}
	}
	counts := func() (routes, observations int) {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); queryErr != nil {
			t.Fatal(queryErr)
		}
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&observations); queryErr != nil {
			t.Fatal(queryErr)
		}
		return routes, observations
	}

	for _, source := range []string{"project_settings", "local_settings", "policy_settings", "skills"} {
		deliver(source, configPath)
	}
	unmanagedPath := filepath.Join(home, ".claude", "settings.local.json")
	if err = os.WriteFile(unmanagedPath, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deliver("user_settings", unmanagedPath)
	if routes, observations := counts(); routes != 0 || observations != 0 {
		t.Fatalf("unmanaged ConfigChange sources wrote routes=%d observations=%d, want 0/0", routes, observations)
	}
}
