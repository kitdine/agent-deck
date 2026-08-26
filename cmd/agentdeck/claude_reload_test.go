package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// TestClaudeConfigChangeRecordsConfirmedFirstKeyMatchAndUnknownMismatch
// covers Contract 3's effective-route policy end to end through the real
// Hook command path: a matched change only advances the route when the
// session's prior state confirms a no-key -> first-key transition; a matched
// change from an already-keyed prior state (rotation) retains the existing
// route; and an explicit settings mismatch still records unknown/multiplier 1
// unconditionally, exactly as before this task.
func TestClaudeConfigChangeRecordsConfirmedFirstKeyMatchAndUnknownMismatch(t *testing.T) {
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
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_name_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2','default',0,'2026-08-04T00:00:00Z');
		INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
		VALUES('claude','session','2026-08-04T00:00:00Z','official','1',0,'SessionStart','startup','estimated','session-start');
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

	// Confirmed no-key -> first-key: the prior route is official (no-key),
	// and the matched selection carries a credential.
	deliver("user_settings", configPath)
	if got := routes(); len(got) != 2 || got[0] != "official" || got[1] != "custom" {
		t.Fatalf("confirmed first-key routes = %#v", got)
	}

	// Key rotation: the prior route is now the keyed "custom" selection
	// above, so a second matched, differently-credentialed selection must
	// retain the existing route rather than overwrite it.
	database, err = store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(2,'custom-b','https://other.example','ref-b','3','2026-08-04T00:00:01Z','2026-08-04T00:00:01Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_name_snapshot,via_wrapper,selected_at)
		VALUES(2,'claude','custom-b','https://other.example','3','other',0,'2026-08-04T00:00:01Z');
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://other.example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deliver("user_settings", configPath)
	if got := routes(); len(got) != 2 {
		t.Fatalf("key rotation wrote a route = %#v, want unchanged", got)
	}

	// An explicit settings mismatch still records unknown/multiplier 1,
	// unconditionally, exactly as before this task.
	if err = os.WriteFile(configPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://mismatch.example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deliver("user_settings", configPath)
	if got := routes(); len(got) != 3 || got[2] != "unknown" {
		t.Fatalf("mismatch routes = %#v", got)
	}

	for _, source := range []string{"project_settings", "local_settings", "policy_settings", "skills"} {
		deliver(source, configPath)
	}
	deliver("user_settings", filepath.Join(home, ".claude", "settings.local.json"))
	if got := routes(); len(got) != 3 {
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
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_name_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2','default',0,'2026-08-04T00:00:00Z');
		INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
		VALUES('claude','session','2026-08-04T00:00:00Z','official','1',0,'SessionStart','startup','estimated','session-start');
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
	if err = database.DB.QueryRowContext(ctx, `SELECT provider,multiplier,quality FROM usage_session_routes WHERE client='claude' AND session_id='session' ORDER BY id DESC LIMIT 1`).Scan(&providerName, &multiplier, &quality); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if routes != 2 || providerName != "custom" || multiplier != "2" || quality != "estimated" {
		t.Fatalf("reconciled routes = %d, latest (%q, %q, %q); want 2, custom, 2, estimated", routes, providerName, multiplier, quality)
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

// assertUnreadableSettingsRecordsIndeterminateRetain is the shared body for
// E1-F1's regression: reconcileClaudeConfigChange must stay fail-open (return
// nil) and still leave exactly one observation classifying
// indeterminate/retain with an unreadable conflict scan and no route, rather
// than silently dropping the accepted delivery, when the settings snapshot
// cannot be read or parsed on every attempt.
func assertUnreadableSettingsRecordsIndeterminateRetain(t *testing.T, configPath string) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'custom','https://provider.example','ref','2','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_name_snapshot,via_wrapper,selected_at)
		VALUES(1,'claude','custom','https://provider.example','2','default',0,'2026-08-04T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	home := filepath.Dir(filepath.Dir(configPath))
	oldSleep := sleepForHookReconciliation
	sleepForHookReconciliation = func(time.Duration) {}
	t.Cleanup(func() { sleepForHookReconciliation = oldSleep })

	if err = reconcileClaudeConfigChange(ctx, database, home, "session", "delivery-1"); err != nil {
		t.Fatalf("reconcileClaudeConfigChange must stay fail-open, got: %v", err)
	}

	var observations, routes int
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations WHERE client='claude' AND session_id='session'`).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes WHERE client='claude' AND session_id='session'`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if observations != 1 || routes != 0 {
		t.Fatalf("observations=%d routes=%d, want 1/0 (accepted, observed, no route)", observations, routes)
	}
	var routeEffect string
	var configMatched sql.NullInt64
	var priorState, conflictScan sql.NullString
	var conflictSources string
	if err = database.DB.QueryRowContext(ctx, `SELECT route_effect,config_matched,prior_state,conflict_scan,conflict_sources FROM usage_session_observations WHERE client='claude' AND session_id='session'`).Scan(&routeEffect, &configMatched, &priorState, &conflictScan, &conflictSources); err != nil {
		t.Fatal(err)
	}
	if routeEffect != "retain" || configMatched.Valid || !priorState.Valid || priorState.String != "indeterminate" || !conflictScan.Valid || conflictScan.String != "unreadable" || conflictSources != "" {
		t.Fatalf("classifier = (effect=%q, matched=%v, prior=%v, conflict=%v, sources=%q), want retain/NULL/indeterminate/unreadable/\"\"", routeEffect, configMatched, priorState, conflictScan, conflictSources)
	}
	// E3-F1: the provider snapshot read succeeded on every attempt; only the
	// settings document was unreadable. The completed selection this reconcile
	// observed must therefore survive into the observation instead of being
	// discarded along with the settings.
	var observedProvider, observedMultiplier sql.NullString
	var observedViaWrapper sql.NullInt64
	if err = database.DB.QueryRowContext(ctx, `SELECT observed_provider,observed_multiplier,observed_via_wrapper FROM usage_session_observations WHERE client='claude' AND session_id='session'`).Scan(&observedProvider, &observedMultiplier, &observedViaWrapper); err != nil {
		t.Fatal(err)
	}
	if !observedProvider.Valid || observedProvider.String != "custom" || !observedMultiplier.Valid || observedMultiplier.String != "2" || !observedViaWrapper.Valid || observedViaWrapper.Int64 != 0 {
		t.Fatalf("observed selection = (provider=%v, multiplier=%v, viaWrapper=%v), want custom/2/0", observedProvider, observedMultiplier, observedViaWrapper)
	}
}

func TestClaudeConfigChangeMissingSettingsFileRecordsIndeterminateRetain(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "settings.json")
	assertUnreadableSettingsRecordsIndeterminateRetain(t, configPath)
}

func TestClaudeConfigChangeMalformedSettingsFileRecordsIndeterminateRetain(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"env":`), 0o600); err != nil {
		t.Fatal(err)
	}
	assertUnreadableSettingsRecordsIndeterminateRetain(t, configPath)
}
