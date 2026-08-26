package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestClaudeConfigChangeRecordsManagedMatchAndUnknownOnly(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'custom','https://provider.example','ref','2','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2',0,'2026-08-04T00:00:00Z');
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err = os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://provider.example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	oldSleep := sleepForHookReconciliation
	sleepForHookReconciliation = func(time.Duration) {}
	t.Cleanup(func() { sleepForHookReconciliation = oldSleep })

	deliver := func(source, path string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      "session",
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
			t.Fatalf("hook stdout = %q", stdout.String())
		}
	}
	routes := func() []string {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		rows, queryErr := database.DB.QueryContext(ctx, `SELECT provider FROM usage_session_routes ORDER BY id`)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		var values []string
		for rows.Next() {
			var provider string
			if scanErr := rows.Scan(&provider); scanErr != nil {
				t.Fatal(scanErr)
			}
			values = append(values, provider)
		}
		return values
	}

	deliver("user_settings", configPath)
	if got := routes(); len(got) != 1 || got[0] != "custom" {
		t.Fatalf("matched routes = %#v", got)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://other.example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deliver("user_settings", configPath)
	if got := routes(); len(got) != 2 || got[1] != "unknown" {
		t.Fatalf("unmatched routes = %#v", got)
	}
	for _, source := range []string{"project_settings", "local_settings", "policy_settings", "skills"} {
		deliver(source, configPath)
	}
	deliver("user_settings", filepath.Join(home, ".claude", "settings.local.json"))
	if got := routes(); len(got) != 2 {
		t.Fatalf("unmanaged scopes wrote routes = %#v", got)
	}
}

func TestClaudeConfigChangeRetriesTransientSettingsRead(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'custom','https://provider.example','ref','2','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2',0,'2026-08-04T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err = os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldSleep := sleepForHookReconciliation
	sleeps := 0
	sleepForHookReconciliation = func(time.Duration) {
		sleeps++
		if sleeps == 1 {
			if writeErr := os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://provider.example"}}`), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
	}
	t.Cleanup(func() { sleepForHookReconciliation = oldSleep })

	if err = reconcileClaudeConfigChange(ctx, database, home, "session", "delivery-1"); err != nil {
		t.Fatalf("reconcileClaudeConfigChange: %v", err)
	}
	if sleeps != 1 {
		t.Fatalf("reconciliation sleeps = %d, want 1", sleeps)
	}
	var providerName, multiplier, quality string
	var routes int
	if err = database.DB.QueryRowContext(ctx, `SELECT provider,multiplier,quality FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&providerName, &multiplier, &quality); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if routes != 1 || providerName != "custom" || multiplier != "2" || quality != "estimated" {
		t.Fatalf("reconciled routes = %d, (%q, %q, %q); want 1, custom, 2, estimated", routes, providerName, multiplier, quality)
	}
}

func TestClaudeConfigChangeRecordsUnknownAfterConfirmedMismatchAndTransientReadFailure(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'custom','https://provider.example','ref','2','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2',0,'2026-08-04T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err = os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://other.example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldSleep := sleepForHookReconciliation
	sleeps := 0
	sleepForHookReconciliation = func(time.Duration) {
		sleeps++
		if sleeps == 1 {
			if writeErr := os.WriteFile(configPath, []byte(`{"env":`), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
	}
	t.Cleanup(func() { sleepForHookReconciliation = oldSleep })

	if err = reconcileClaudeConfigChange(ctx, database, home, "session", "delivery-1"); err != nil {
		t.Fatalf("reconcileClaudeConfigChange: %v", err)
	}
	if sleeps != 2 {
		t.Fatalf("reconciliation sleeps = %d, want 2", sleeps)
	}
	var providerName, multiplier, quality string
	var routes int
	if err = database.DB.QueryRowContext(ctx, `SELECT provider,multiplier,quality FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&providerName, &multiplier, &quality); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if routes != 1 || providerName != "unknown" || multiplier != "1" || quality != "estimated" {
		t.Fatalf("reconciled routes = %d, (%q, %q, %q); want 1, unknown, 1, estimated", routes, providerName, multiplier, quality)
	}
}
