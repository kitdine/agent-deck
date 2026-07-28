package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func setProviderWrapperForTest(t *testing.T, database *store.Store, ctx context.Context, name, url string) string {
	t.Helper()
	normalized, err := NormalizeWrapperURL(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SetProviderWrapper(ctx, name, normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}

func setOfficialWrapperForTest(t *testing.T, database *store.Store, ctx context.Context, url string) string {
	t.Helper()
	normalized, err := NormalizeWrapperURL(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetOfficialWrapperURL(ctx, normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}

// TestUseCustomProviderCodexViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical
// covers the route-composition acceptance criterion directly: switching a
// custom provider between direct and --via must change only the endpoint
// field and leave the written credential byte-identical.
func TestUseCustomProviderCodexViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	wrapper := setProviderWrapperForTest(t, database, ctx, "example", "https://wrapper.example/")
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := service.UseCredential(ctx, "example", ClientCodex, "", config, filepath.Join(root, "direct.backup.toml"), false); err != nil {
		t.Fatal(err)
	}
	_, direct := readCodexCustomProvider(t, config)
	if direct["base_url"] != "https://provider.example/v1" || direct["experimental_bearer_token"] != "synthetic-secret" {
		t.Fatalf("direct custom provider = %#v", direct)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.ViaWrapper || snapshot.Endpoint != "https://provider.example" {
		t.Fatalf("direct snapshot = %#v, %v", snapshot, err)
	}

	if err := service.UseCredential(ctx, "example", ClientCodex, "", config, filepath.Join(root, "via.backup.toml"), true); err != nil {
		t.Fatal(err)
	}
	_, viaCustom := readCodexCustomProvider(t, config)
	if viaCustom["base_url"] != "https://wrapper.example/v1" {
		t.Fatalf("via custom provider base_url = %#v, want wrapper endpoint", viaCustom)
	}
	if viaCustom["experimental_bearer_token"] != direct["experimental_bearer_token"] {
		t.Fatalf("via credential = %#v, want byte-identical to direct %#v", viaCustom["experimental_bearer_token"], direct["experimental_bearer_token"])
	}
	snapshot, err = database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || !snapshot.ViaWrapper || snapshot.Endpoint != wrapper {
		t.Fatalf("via snapshot = %#v, %v", snapshot, err)
	}
}

// TestUseCustomProviderClaudeViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical
// mirrors the Codex case on Claude, which is where claude-writer-routes
// Round 1's deferred P3 lives: every WriteClaudeConfig call here must decide
// its endpoint/credential intent explicitly rather than passing through a
// lookup that could return "" for an unrelated reason.
func TestUseCustomProviderClaudeViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientClaude}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	wrapper := setProviderWrapperForTest(t, database, ctx, "example", "https://wrapper.example/")
	config := filepath.Join(root, "settings.json")
	if err := os.WriteFile(config, []byte(`{"unrelated":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := service.UseCredential(ctx, "example", ClientClaude, "", config, filepath.Join(root, "direct.backup.json"), false); err != nil {
		t.Fatal(err)
	}
	direct := readClaudeEnv(t, config)
	if direct["ANTHROPIC_BASE_URL"] != "https://provider.example" || direct["ANTHROPIC_AUTH_TOKEN"] != "synthetic-secret" {
		t.Fatalf("direct env = %#v", direct)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "claude")
	if err != nil || snapshot.ViaWrapper || snapshot.Endpoint != "https://provider.example" {
		t.Fatalf("direct snapshot = %#v, %v", snapshot, err)
	}

	if err := service.UseCredential(ctx, "example", ClientClaude, "", config, filepath.Join(root, "via.backup.json"), true); err != nil {
		t.Fatal(err)
	}
	via := readClaudeEnv(t, config)
	if via["ANTHROPIC_BASE_URL"] != "https://wrapper.example" {
		t.Fatalf("via env base url = %#v, want wrapper endpoint", via)
	}
	if via["ANTHROPIC_AUTH_TOKEN"] != direct["ANTHROPIC_AUTH_TOKEN"] {
		t.Fatalf("via credential = %#v, want byte-identical to direct %#v", via["ANTHROPIC_AUTH_TOKEN"], direct["ANTHROPIC_AUTH_TOKEN"])
	}
	snapshot, err = database.CurrentProviderSnapshot(ctx, "claude")
	if err != nil || !snapshot.ViaWrapper || snapshot.Endpoint != wrapper {
		t.Fatalf("via snapshot = %#v, %v", snapshot, err)
	}
}

// TestUseOfficialCodexViaWrapperWritesEndpointRemovesCredentialAndRecordsSnapshot
// covers the second acceptance criterion: provider use official --client codex
// completes with --via and records a route snapshot, without touching the
// credential vault.
func TestUseOfficialCodexViaWrapperWritesEndpointRemovesCredentialAndRecordsSnapshot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := &rejectingCredentialVault{}
	wrapper := setOfficialWrapperForTest(t, database, ctx, "https://wrapper.example/")
	service := Service{Store: database, Vault: secrets}
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model_provider = 'custom'\n[model_providers.custom]\nname = 'keep'\nbase_url = 'https://old.example/v1'\nexperimental_bearer_token = 'stale-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(ctx, OfficialProviderName, ClientCodex, "", config, filepath.Join(root, "via.backup.toml"), true); err != nil {
		t.Fatal(err)
	}
	if secrets.calls != 0 {
		t.Fatalf("official via switch accessed credential vault %d times", secrets.calls)
	}
	_, custom := readCodexCustomProvider(t, config)
	if custom["name"] != OfficialProviderName || custom["base_url"] != "https://wrapper.example/v1" {
		t.Fatalf("official via custom provider = %#v", custom)
	}
	if _, hasBearer := custom["experimental_bearer_token"]; hasBearer {
		t.Fatalf("official via custom provider kept a credential: %#v", custom)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || !snapshot.Official || !snapshot.ViaWrapper || snapshot.Endpoint != wrapper || snapshot.Multiplier != "1" {
		t.Fatalf("official via snapshot = %#v, %v", snapshot, err)
	}
}

// TestUseOfficialClaudeDirectAndViaWrapperDecideIntentExplicitly closes the
// claude-writer-routes Round 1 carried-forward P3: the official route never
// passes an accidental empty-string lookup into WriteClaudeConfig. Direct
// deliberately removes both owned keys (official has no endpoint or
// credential); --via deliberately writes the wrapper endpoint alone.
func TestUseOfficialClaudeDirectAndViaWrapperDecideIntentExplicitly(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := &rejectingCredentialVault{}
	wrapper := setOfficialWrapperForTest(t, database, ctx, "https://wrapper.example/")
	service := Service{Store: database, Vault: secrets}
	config := filepath.Join(root, "settings.json")
	if err := os.WriteFile(config, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"stale-secret","UNRELATED":true},"unrelatedTop":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := service.UseCredential(ctx, OfficialProviderName, ClientClaude, "", config, filepath.Join(root, "direct.backup.json"), false); err != nil {
		t.Fatal(err)
	}
	direct := readClaudeEnv(t, config)
	if _, ok := direct["ANTHROPIC_BASE_URL"]; ok {
		t.Fatalf("official direct kept endpoint: %#v", direct)
	}
	if _, ok := direct["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Fatalf("official direct kept credential: %#v", direct)
	}
	if direct["UNRELATED"] != true {
		t.Fatalf("official direct lost unrelated env key: %#v", direct)
	}

	if err := service.UseCredential(ctx, OfficialProviderName, ClientClaude, "", config, filepath.Join(root, "via.backup.json"), true); err != nil {
		t.Fatal(err)
	}
	if secrets.calls != 0 {
		t.Fatalf("official switch accessed credential vault %d times", secrets.calls)
	}
	via := readClaudeEnv(t, config)
	if via["ANTHROPIC_BASE_URL"] != wrapper {
		t.Fatalf("official via endpoint = %#v, want wrapper %q", via["ANTHROPIC_BASE_URL"], wrapper)
	}
	if _, ok := via["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Fatalf("official via wrote a credential: %#v", via)
	}
	if via["UNRELATED"] != true {
		t.Fatalf("official via lost unrelated env key: %#v", via)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "claude")
	if err != nil || !snapshot.Official || !snapshot.ViaWrapper || snapshot.Endpoint != wrapper {
		t.Fatalf("official via claude snapshot = %#v, %v", snapshot, err)
	}
}

// TestUseViaWrapperWithoutConfiguredWrapperFailsBeforeWritingClientFile pins
// the route-composition validation that a --via switch never touches the
// client file when no wrapper is configured for the selected route.
func TestUseViaWrapperWithoutConfiguredWrapperFailsBeforeWritingClientFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	before := []byte("model = 'keep'\n")
	if err := os.WriteFile(config, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(ctx, "example", ClientCodex, "", config, filepath.Join(root, "via.backup.toml"), true); err == nil {
		t.Fatal("UseCredential succeeded with --via and no configured wrapper")
	}
	after, err := os.ReadFile(config)
	if err != nil || string(after) != string(before) {
		t.Fatalf("config changed despite missing wrapper: %q, %v", after, err)
	}
	if _, err := database.CurrentProviderSnapshot(ctx, "codex"); err != sql.ErrNoRows {
		t.Fatalf("selection recorded despite missing wrapper: %v", err)
	}
}

// newRouteDriftFixture builds a service whose default config paths resolve
// under one temporary home, which is what ConfigDrift reads.
func newRouteDriftFixture(t *testing.T, vault CredentialVault) (context.Context, *store.Store, Service, string) {
	t.Helper()
	ctx := context.Background()
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return ctx, database, Service{Store: database, Vault: vault, Home: home, StateRoot: state}, home
}

func writeClientConfigForTest(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func driftForTest(t *testing.T, ctx context.Context, service Service, home string) int {
	t.Helper()
	drift, err := service.ConfigDrift(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	return drift
}

// TestOfficialClaudeSelectionIsNotReportedAsConfigDrift pins the read-back half
// of the official Claude route. Before route-composition made official
// selectable for Claude, ConfigDrift only ever saw a Codex config.toml for an
// official snapshot, so it matched every official selection with the
// TOML-parsing Codex matcher; handing it settings.json made a correct switch
// report permanent drift with a recovery that could never clear it.
func TestOfficialClaudeSelectionIsNotReportedAsConfigDrift(t *testing.T) {
	ctx, _, service, home := newRouteDriftFixture(t, &rejectingCredentialVault{})
	settings := filepath.Join(home, ".claude", "settings.json")
	writeClientConfigForTest(t, settings, `{"env":{"ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"stale-secret","UNRELATED":true},"unrelatedTop":1}`)

	if err := service.UseCredential(ctx, OfficialProviderName, ClientClaude, "", "", "", false); err != nil {
		t.Fatal(err)
	}
	if drift := driftForTest(t, ctx, service, home); drift != 0 {
		t.Fatalf("official claude drift after switch = %d, want 0", drift)
	}

	// A real divergence must still count, so the assertion above cannot be
	// satisfied by a matcher that accepts anything.
	writeClientConfigForTest(t, settings, `{"env":{"ANTHROPIC_BASE_URL":"https://elsewhere.example"}}`)
	if drift := driftForTest(t, ctx, service, home); drift != 1 {
		t.Fatalf("official claude drift after external edit = %d, want 1", drift)
	}
}

// TestOfficialCodexViaWrapperSelectionIsNotReportedAsConfigDrift covers the
// latent half of the same defect: the direct Codex matcher requires no
// base_url, but a --via official switch writes one by design.
func TestOfficialCodexViaWrapperSelectionIsNotReportedAsConfigDrift(t *testing.T) {
	ctx, database, service, home := newRouteDriftFixture(t, &rejectingCredentialVault{})
	setOfficialWrapperForTest(t, database, ctx, "https://wrapper.example/")
	config := filepath.Join(home, ".codex", "config.toml")
	writeClientConfigForTest(t, config, "model = 'keep'\n")

	if err := service.UseCredential(ctx, OfficialProviderName, ClientCodex, "", "", "", true); err != nil {
		t.Fatal(err)
	}
	if drift := driftForTest(t, ctx, service, home); drift != 0 {
		t.Fatalf("official codex via-wrapper drift after switch = %d, want 0", drift)
	}

	// A credential appearing under a wrapper-routed official selection is
	// drift: that route is defined as endpoint-only.
	writeClientConfigForTest(t, config, "model_provider = 'custom'\n[model_providers.custom]\nname = 'official'\nbase_url = 'https://wrapper.example/v1'\nexperimental_bearer_token = 'injected'\n")
	if drift := driftForTest(t, ctx, service, home); drift != 1 {
		t.Fatalf("official codex via-wrapper drift after credential injection = %d, want 1", drift)
	}
}

// TestOfficialClaudeViaWrapperSelectionIsNotReportedAsConfigDrift is the Claude
// arm of the wrapper-routed official route.
func TestOfficialClaudeViaWrapperSelectionIsNotReportedAsConfigDrift(t *testing.T) {
	ctx, database, service, home := newRouteDriftFixture(t, &rejectingCredentialVault{})
	setOfficialWrapperForTest(t, database, ctx, "https://wrapper.example/")
	settings := filepath.Join(home, ".claude", "settings.json")
	writeClientConfigForTest(t, settings, `{"env":{"UNRELATED":true}}`)

	if err := service.UseCredential(ctx, OfficialProviderName, ClientClaude, "", "", "", true); err != nil {
		t.Fatal(err)
	}
	if drift := driftForTest(t, ctx, service, home); drift != 0 {
		t.Fatalf("official claude via-wrapper drift after switch = %d, want 0", drift)
	}

	writeClientConfigForTest(t, settings, `{"env":{"ANTHROPIC_BASE_URL":"https://wrapper.example","ANTHROPIC_AUTH_TOKEN":"injected"}}`)
	if drift := driftForTest(t, ctx, service, home); drift != 1 {
		t.Fatalf("official claude via-wrapper drift after credential injection = %d, want 1", drift)
	}
}

// TestCustomProviderViaWrapperSelectionIsNotReportedAsConfigDrift covers the
// fourth route ConfigDrift now branches on. It already worked, because the
// selection snapshot records the endpoint actually written, and this pins that
// the endpoint recorded for a wrapped custom switch stays the comparison
// basis.
func TestCustomProviderViaWrapperSelectionIsNotReportedAsConfigDrift(t *testing.T) {
	ctx, database, service, home := newRouteDriftFixture(t, testCredentialVault(t))
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	setProviderWrapperForTest(t, database, ctx, "example", "https://wrapper.example/")
	config := filepath.Join(home, ".codex", "config.toml")
	writeClientConfigForTest(t, config, "model = 'keep'\n")

	if err := service.UseCredential(ctx, "example", ClientCodex, "", "", "", true); err != nil {
		t.Fatal(err)
	}
	if drift := driftForTest(t, ctx, service, home); drift != 0 {
		t.Fatalf("custom via-wrapper drift after switch = %d, want 0", drift)
	}

	if err := service.UseCredential(ctx, "example", ClientCodex, "", "", "", false); err != nil {
		t.Fatal(err)
	}
	if drift := driftForTest(t, ctx, service, home); drift != 0 {
		t.Fatalf("custom direct drift after switching back = %d, want 0", drift)
	}
}

// TestUseCustomProviderWithEmptyStoredEndpointFailsBeforeTouchingClientFile
// closes the carried claude-writer-routes P3 on the custom route: an empty
// ClientConfig field is the writer's "remove this owned key" sentinel, so a
// lookup that comes back empty must fail the switch rather than silently
// downgrade it to a deletion. Credential creation and update both reject an
// empty endpoint and an empty secret, so the row is forced here directly.
func TestUseCustomProviderWithEmptyStoredEndpointFailsBeforeTouchingClientFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientClaude}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE provider_credentials SET endpoint=''`); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "settings.json")
	before := []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"stale-secret"}}`)
	if err := os.WriteFile(config, before, 0o600); err != nil {
		t.Fatal(err)
	}

	err = service.UseCredential(ctx, "example", ClientClaude, "", config, filepath.Join(root, "backup.json"), false)
	if !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("UseCredential with an empty stored endpoint = %v, want ErrInvalidProvider", err)
	}
	after, readErr := os.ReadFile(config)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("client file changed despite the rejected switch: %q, %v", after, readErr)
	}
	if _, err := database.CurrentProviderSnapshot(ctx, "claude"); err != sql.ErrNoRows {
		t.Fatalf("selection recorded despite the rejected switch: %v", err)
	}
}

func readClaudeEnv(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	env, _ := document["env"].(map[string]any)
	return env
}
