package provider

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/store"
)

func testCredentialVault(t *testing.T) *credentialvault.Vault {
	t.Helper()
	return credentialvault.New(filepath.Join(t.TempDir(), "vault"), func(context.Context) (string, error) {
		return "synthetic-machine", nil
	})
}

func TestServiceStoresAuthenticatedCiphertextAndReportsPresence(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := testCredentialVault(t)
	service := Service{Store: database, Vault: vault}
	created, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example/v1", Clients: []Client{ClientCodex}, CredentialRef: "agentdeck:provider:example", Multiplier: "1.5"}, "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := service.List(ctx)
	if err != nil || len(definitions) != 2 || definitions[1].Definition.CreatedAt == nil || definitions[1].Definition.UpdatedAt == nil {
		t.Fatalf("provider definitions = %#v, %v", definitions, err)
	}
	statuses, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0].Definition.Name != OfficialProviderName || len(statuses[0].Credentials) != 0 || len(statuses[1].Credentials) != 1 || !statuses[1].Credentials[0].Present || statuses[1].Definition.ID != created.ID {
		t.Fatalf("statuses = %#v", statuses)
	}
	credential, err := database.ProviderCredential(ctx, "example", "default")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(secret.Ciphertext, []byte("synthetic-secret")) {
		t.Fatal("credential ciphertext contains plaintext")
	}
	if opened, openErr := vault.Open(ctx, credential.CredentialRef, sealedRecord(secret)); openErr != nil || opened != "synthetic-secret" {
		t.Fatalf("opened credential = %q, %v", opened, openErr)
	}
	if _, err = database.Exec(ctx, `DELETE FROM credential_secrets WHERE credential_id=?`, credential.ID); err != nil {
		t.Fatal(err)
	}
	statuses, err = service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses[1].Credentials) != 1 || statuses[1].Credentials[0].Present {
		t.Fatal("credential remains present")
	}
}

type rejectingCredentialVault struct{ calls int }

func (s *rejectingCredentialVault) reject() error {
	s.calls++
	return errors.New("credential vault must not be accessed")
}
func (s *rejectingCredentialVault) Seal(context.Context, string, string) (credentialvault.Sealed, error) {
	return credentialvault.Sealed{}, s.reject()
}
func (s *rejectingCredentialVault) SealExisting(context.Context, string, string) (credentialvault.Sealed, error) {
	return credentialvault.Sealed{}, s.reject()
}
func (s *rejectingCredentialVault) Open(context.Context, string, credentialvault.Sealed) (string, error) {
	return "", s.reject()
}
func (s *rejectingCredentialVault) InspectKey(context.Context) (string, error) {
	return "", s.reject()
}

func TestCredentialMutationTransactionsRollbackMetadataAndCiphertext(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := testCredentialVault(t)
	service := Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://example.invalid", Clients: []Client{ClientCodex}}, "old-secret"); err != nil {
		t.Fatal(err)
	}
	credential, err := database.ProviderCredential(ctx, "example", "default")
	if err != nil {
		t.Fatal(err)
	}
	before, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_credential_update BEFORE UPDATE ON provider_credentials BEGIN SELECT RAISE(FAIL,'injected update failure'); END`); err != nil {
		t.Fatal(err)
	}
	rotated := "new-secret"
	if _, err = service.UpdateNamedCredential(ctx, "example", "default", nil, nil, nil, &rotated); err == nil {
		t.Fatal("credential rotation succeeded")
	}
	if _, err = database.Exec(ctx, `DROP TRIGGER fail_credential_update`); err != nil {
		t.Fatal(err)
	}
	after, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil || !bytes.Equal(after.Ciphertext, before.Ciphertext) {
		t.Fatalf("ciphertext changed after rollback: %v", err)
	}
	if opened, openErr := vault.Open(ctx, credential.CredentialRef, sealedRecord(after)); openErr != nil || opened != "old-secret" {
		t.Fatalf("rolled back credential = %q, %v", opened, openErr)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_credential_delete BEFORE DELETE ON provider_credentials BEGIN SELECT RAISE(FAIL,'injected delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveNamedCredential(ctx, "example", "default"); err == nil {
		t.Fatal("credential removal succeeded")
	}
	if _, err = database.CredentialSecret(ctx, credential.ID); err != nil {
		t.Fatalf("ciphertext removed after rollback: %v", err)
	}
}

func TestCredentialWritesDoNotRegenerateMissingOrMismatchedKey(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := credentialvault.New(state, func(context.Context) (string, error) { return "machine-a", nil })
	service := Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://example.invalid", Clients: []Client{ClientCodex}}, "old-secret"); err != nil {
		t.Fatal(err)
	}
	credential, err := database.ProviderCredential(ctx, "example", "default")
	if err != nil {
		t.Fatal(err)
	}
	before, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}

	mismatched := Service{Store: database, Vault: credentialvault.New(state, func(context.Context) (string, error) { return "machine-b", nil })}
	if _, err = mismatched.AddCredential(ctx, "example", "work", "https://example.invalid", "1", []Client{ClientCodex}, "work-secret"); !errors.Is(err, credentialvault.ErrKeyMachineMismatch) {
		t.Fatalf("machine mismatch error = %v", err)
	}
	if _, err = database.ProviderCredential(ctx, "example", "work"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("mismatched credential metadata = %v", err)
	}

	if err = os.Remove(vault.KeyPath()); err != nil {
		t.Fatal(err)
	}
	replacement := "replacement"
	if _, err = service.UpdateNamedCredential(ctx, "example", "default", nil, nil, nil, &replacement); !errors.Is(err, credentialvault.ErrKeyMissing) {
		t.Fatalf("missing key rotation error = %v", err)
	}
	if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
		t.Fatalf("missing key was regenerated: %v", statErr)
	}
	after, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil || !bytes.Equal(after.Ciphertext, before.Ciphertext) {
		t.Fatalf("ciphertext changed after missing key failure: %v", err)
	}
}

func TestOfficialProviderIsBuiltInAndDefinitionReadsDoNotAccessSecrets(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := &rejectingCredentialVault{}
	service := Service{Store: database, Vault: secrets}

	providers, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].Definition.Name != OfficialProviderName || !providers[0].Definition.BuiltIn || providers[0].Definition.Authentication != "codex_existing_login" || providers[0].Definition.CredentialCount != 0 || len(providers[0].Definition.Clients) != 1 || providers[0].Definition.Clients[0].Client != string(ClientCodex) {
		t.Fatalf("providers = %#v", providers)
	}
	shown, err := service.Show(ctx, OfficialProviderName)
	if err != nil || shown.Definition.Name != OfficialProviderName || !shown.Definition.BuiltIn {
		t.Fatalf("official show = %#v, %v", shown, err)
	}
	if secrets.calls != 0 {
		t.Fatalf("definition reads accessed credential vault %d times", secrets.calls)
	}
	var stored int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM providers").Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("stored official provider count = %d, %v", stored, err)
	}
}

func TestBearerOfficialBearerSwitchKeepsActiveStateAndDriftConsistent(t *testing.T) {
	ctx := context.Background()
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets, Home: home, StateRoot: state}
	if _, err := service.Add(ctx, Definition{Name: "bearer", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "bearer-ref", Multiplier: "2"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := service.Use(ctx, "bearer", ClientCodex, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.Use(ctx, OfficialProviderName, ClientCodex, "", ""); err != nil {
		t.Fatal(err)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || !snapshot.Official || snapshot.Name != OfficialProviderName || snapshot.Multiplier != "1" {
		t.Fatalf("official active snapshot = %#v, %v", snapshot, err)
	}
	if drift, err := service.ConfigDrift(ctx, home); err != nil || drift != 0 {
		t.Fatalf("official drift = %d, %v", drift, err)
	}

	if err := service.Use(ctx, "bearer", ClientCodex, "", ""); err != nil {
		t.Fatal(err)
	}
	snapshot, err = database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.Official || snapshot.Name != "bearer" || snapshot.Multiplier != "2.000000000000" {
		t.Fatalf("bearer active snapshot = %#v, %v", snapshot, err)
	}
	if drift, err := service.ConfigDrift(ctx, home); err != nil || drift != 0 {
		t.Fatalf("bearer drift = %d, %v", drift, err)
	}
}

func TestUseOfficialSetsNamePreservesAuthAndRemovesManagedTransportFields(t *testing.T) {
	ctx := context.Background()
	root, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := &rejectingCredentialVault{}
	service := Service{Store: database, Vault: secrets, StateRoot: state}
	config := filepath.Join(root, "config.toml")
	auth := filepath.Join(root, "auth.json")
	beforeConfig := "# keep\nmodel_provider = 'custom'\n[model_providers.custom]\nname = 'keep'\nbase_url = 'https://provider.example/v1'\nexperimental_bearer_token = 'synthetic-secret'\nwire_api = 'responses' # keep\n"
	wantConfig := "# keep\nmodel_provider = 'custom'\n[model_providers.custom]\nname = \"official\"\nwire_api = 'responses' # keep\n"
	authBytes := []byte("{\"tokens\":\"untouched bytes\"}\n")
	if err := os.WriteFile(config, []byte(beforeConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auth, authBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := service.Use(ctx, OfficialProviderName, ClientCodex, config, ""); err != nil {
			t.Fatal(err)
		}
	}
	contents, err := os.ReadFile(config)
	if err != nil || string(contents) != wantConfig {
		t.Fatalf("official config = %q, %v", contents, err)
	}
	afterAuth, err := os.ReadFile(auth)
	if err != nil || string(afterAuth) != string(authBytes) {
		t.Fatalf("auth.json changed = %q, %v", afterAuth, err)
	}
	if secrets.calls != 0 {
		t.Fatalf("official switch accessed credential vault %d times", secrets.calls)
	}
	var providers, selections, completed int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM providers").Scan(&providers); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_selections").Scan(&selections); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM operations WHERE state = 'completed' AND provider_id IS NULL").Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if providers != 0 || selections != 2 || completed != 2 {
		t.Fatalf("official persistence providers=%d selections=%d completed=%d", providers, selections, completed)
	}
}

func TestUseWritesOnlyExplicitTemporaryConfigPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "agentdeck:provider:example"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.Use(ctx, "example", ClientCodex, config, filepath.Join(root, "backup.toml")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "provider.example/v1") {
		t.Fatalf("config = %s", contents)
	}
}

func TestUseResolvesDefaultConfigAndUniqueManagedBackups(t *testing.T) {
	ctx := context.Background()
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets, Home: home, StateRoot: state}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "agentdeck:provider:example"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := service.Use(ctx, "example", ClientCodex, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(state, "client-backups", "codex", "*.redacted.toml"))
	if err != nil || len(backups) != 2 {
		t.Fatalf("managed backups = %v, %v", backups, err)
	}
	for _, backup := range backups {
		info, statErr := os.Stat(backup)
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("backup %q mode = %v, %v", backup, info.Mode().Perm(), statErr)
		}
		contents, readErr := os.ReadFile(backup)
		if readErr != nil || strings.Contains(string(contents), "synthetic-secret") {
			t.Fatalf("backup %q is not redacted: %v", backup, readErr)
		}
	}
	var recorded int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM operations WHERE redacted_backup_path LIKE ?", filepath.Join(state, "client-backups", "codex", "%")).Scan(&recorded); err != nil || recorded != 2 {
		t.Fatalf("recorded managed backups = %d, %v", recorded, err)
	}
}

func TestUseRejectsUnsupportedClient(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err := service.Use(ctx, "example", ClientClaude, filepath.Join(t.TempDir(), "settings.json"), filepath.Join(t.TempDir(), "backup.json")); err == nil {
		t.Fatal("Use succeeded for unsupported client")
	}
}

func TestNamedCredentialsCanShareClientsAndRequireExplicitSelectionWhenAmbiguous(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}
	definition := Definition{Name: "shared", Endpoint: "https://provider.example", Clients: []Client{ClientCodex, ClientClaude}, CredentialRef: "shared-ref", Multiplier: "1"}
	if _, err = service.AddProvider(ctx, definition, "default", "first-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddCredential(ctx, "shared", "work", "https://provider.example", "1", []Client{ClientCodex, ClientClaude}, "work-secret"); err != nil {
		t.Fatal(err)
	}
	credentials, err := service.ListCredentials(ctx, "shared", "")
	if err != nil || len(credentials) != 2 {
		t.Fatalf("credentials=%#v err=%v", credentials, err)
	}
	if credentials[1].Reference != "shared-work-ref" {
		t.Fatalf("generated reference=%q", credentials[1].Reference)
	}
	config := filepath.Join(root, "config.toml")
	if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "shared", ClientCodex, "", config, filepath.Join(root, "ambiguous.toml")); err == nil {
		t.Fatal("ambiguous credential selection succeeded")
	}
	if err = service.UseCredential(ctx, "shared", ClientCodex, "work", config, filepath.Join(root, "work.toml")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "work-secret") {
		t.Fatalf("selected config=%s", contents)
	}
}

func TestProviderRemovalRollsBackMetadataAndCiphertextOnDatabaseFailure(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://example.invalid", Clients: []Client{ClientCodex}}, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_provider_delete BEFORE DELETE ON providers BEGIN SELECT RAISE(FAIL,"injected provider delete failure"); END`); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveProvider(ctx, "example"); err == nil {
		t.Fatal("provider removal succeeded")
	}
	credential, lookupErr := database.ProviderCredential(ctx, "example", "default")
	if lookupErr != nil || !credential.SecretPresent {
		t.Fatalf("credential after rollback = %#v, %v", credential, lookupErr)
	}
}

func TestRemovingLastNamedCredentialMakesProviderUnavailable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "empty", Endpoint: "https://example.invalid", CredentialRef: "empty-ref", Clients: []Client{ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveNamedCredential(ctx, "empty", "default"); err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(ctx)
	if err != nil || len(status) != 2 || status[1].Definition.Name != "empty" || status[1].Ready || len(status[1].Credentials) != 0 {
		t.Fatalf("status = %#v, %v", status, err)
	}
	var secretCount int
	if err = database.DB.QueryRowContext(ctx, `SELECT count(*) FROM credential_secrets`).Scan(&secretCount); err != nil || secretCount != 0 {
		t.Fatalf("credential secrets = %d, %v", secretCount, err)
	}
	config := filepath.Join(root, "config.toml")
	if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.Use(ctx, "empty", ClientCodex, config, filepath.Join(root, "backup.toml")); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("provider use error = %v", err)
	}
}

func TestUsedProviderRemovalDeletesLiveMetadataAndPreservesAttributionSnapshot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}
	if _, err = service.Add(ctx, Definition{Name: "used", Endpoint: "https://used.invalid", CredentialRef: "used-ref", Clients: []Client{ClientCodex}, Multiplier: "3"}, "used-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddCredential(ctx, "used", "work", "https://used.invalid", "3", []Client{ClientCodex}, "work-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "used", ClientCodex, "work", config, filepath.Join(root, "backup.toml")); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveProvider(ctx, "used"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ProviderByName(ctx, "used"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted provider lookup = %v", err)
	}
	var credentials int
	if err = database.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_credentials").Scan(&credentials); err != nil || credentials != 0 {
		t.Fatalf("credential metadata count = %d, %v", credentials, err)
	}
	var secretCount int
	if err = database.DB.QueryRowContext(ctx, "SELECT count(*) FROM credential_secrets").Scan(&secretCount); err != nil || secretCount != 0 {
		t.Fatalf("credential secret count = %d, %v", secretCount, err)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.Name != "used" || snapshot.Endpoint != "https://used.invalid" || snapshot.Multiplier != "3.000000000000" || snapshot.Credential != "work" {
		t.Fatalf("historical attribution = %#v, %v", snapshot, err)
	}
}

func TestFailedFinalSelectionOperationDoesNotReplaceCompletedCredentialAttribution(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://example.invalid", CredentialRef: "default-ref", Clients: []Client{ClientCodex}, Multiplier: "2"}, "default-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddCredential(ctx, "example", "work", "https://example.invalid", "2", []Client{ClientCodex}, "work-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "example", ClientCodex, "default", config, filepath.Join(root, "default.backup")); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_completed_selection BEFORE INSERT ON provider_selections BEGIN SELECT RAISE(FAIL,'injected final selection failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "example", ClientCodex, "work", config, filepath.Join(root, "work.backup")); err == nil {
		t.Fatal("selection unexpectedly completed")
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.Name != "example" || snapshot.Credential != "default" || snapshot.Multiplier != "2.000000000000" {
		t.Fatalf("active snapshot = %#v, %v", snapshot, err)
	}
	pending, err := database.PendingOperations(ctx)
	if err != nil || len(pending) != 1 || pending[0].ErrorCode != "selection_commit_failed" {
		t.Fatalf("failed operation = %#v, %v", pending, err)
	}
}

func TestProviderAddPlansAndAddsCredentialToExistingProvider(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}

	if _, err = service.AddProvider(ctx, Definition{Name: "sssaicode", Endpoint: "https://claude.example/v1", Clients: []Client{ClientClaude}, Multiplier: "1"}, "claude", "claude-secret"); err != nil {
		t.Fatal(err)
	}
	definition := Definition{Name: "sssaicode", Endpoint: "https://codex.example/api/v1", Clients: []Client{ClientCodex}, Multiplier: "0.4"}
	plan, err := service.PlanProviderCredential(ctx, definition, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.ProviderExists || plan.Noop || plan.Reference != "sssaicode-codex-ref" || plan.Definition.Endpoint != "https://codex.example/api" {
		t.Fatalf("plan = %#v", plan)
	}
	if _, err = service.AddProviderWithCredential(ctx, definition, "codex", "codex-secret"); err != nil {
		t.Fatal(err)
	}

	credentials, err := database.ListProviderCredentials(ctx, "sssaicode")
	if err != nil || len(credentials) != 2 {
		t.Fatalf("credentials = %#v, %v", credentials, err)
	}
	codex := credentials[1]
	if codex.Name != "codex" || codex.CredentialRef != "sssaicode-codex-ref" || codex.SecretRef != codex.CredentialRef || codex.Endpoint != "https://codex.example/api" || codex.Multiplier != "0.400000000000" || strings.Join(codex.Clients, ",") != "codex" {
		t.Fatalf("codex credential = %#v", codex)
	}
	provider, err := database.ProviderByName(ctx, "sssaicode")
	if err != nil || len(provider.Clients) != 2 || provider.Clients[0].Client != "claude" || provider.Clients[1].Client != "codex" {
		t.Fatalf("provider = %#v, %v", provider, err)
	}
	plan, err = service.PlanProviderCredential(ctx, definition, "codex")
	if err != nil || !plan.Noop {
		t.Fatalf("idempotent plan = %#v, %v", plan, err)
	}
	config := filepath.Join(t.TempDir(), "config.toml")
	if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "sssaicode", ClientCodex, "codex", config, filepath.Join(t.TempDir(), "backup.toml")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(contents), `base_url = 'https://codex.example/api/v1'`) && !strings.Contains(string(contents), `base_url = "https://codex.example/api/v1"`) {
		t.Fatalf("codex config = %q, %v", contents, err)
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.Endpoint != "https://codex.example/api" || snapshot.Multiplier != "0.400000000000" || snapshot.Credential != "codex" {
		t.Fatalf("codex snapshot = %#v, %v", snapshot, err)
	}
}

func TestProviderStatusJSONDoesNotDuplicateCredentialMetadata(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}
	if _, err = service.AddProvider(ctx, Definition{Name: "example", Endpoint: "https://example.invalid", Clients: []Client{ClientCodex}}, "default", "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	statuses, err := service.Status(ctx)
	if err != nil || len(statuses) != 2 {
		t.Fatalf("statuses = %#v, %v", statuses, err)
	}
	encoded, err := json.Marshal(statuses[1])
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	if err = json.Unmarshal(encoded, &status); err != nil {
		t.Fatal(err)
	}
	if _, duplicate := status["credential"]; duplicate {
		t.Fatalf("singular credential remains: %s", encoded)
	}
	credentials, ok := status["credentials"].([]any)
	if !ok || len(credentials) != 1 {
		t.Fatalf("status credentials = %#v", status["credentials"])
	}
	definition, ok := status["definition"].(map[string]any)
	if !ok || definition["credential_count"] != float64(1) {
		t.Fatalf("provider definition = %#v", status["definition"])
	}
	if _, duplicate := definition["credentials"]; duplicate {
		t.Fatalf("definition credential details remain: %s", encoded)
	}
}

func TestProviderAddRejectsOfficialCaseVariants(t *testing.T) {
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	for _, name := range []string{"official", "Official", " OFFICIAL "} {
		if _, err = service.PlanProviderCredential(context.Background(), Definition{Name: name, Endpoint: "https://example.invalid", Clients: []Client{ClientCodex}}, "default"); !errors.Is(err, ErrInvalidProvider) {
			t.Fatalf("PlanProviderCredential(%q) error = %v", name, err)
		}
	}
}

func TestProviderAddExistingCredentialRejectsMetadataDriftBeforeSecretRead(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := testCredentialVault(t)
	service := Service{Store: database, Vault: secrets}
	definition := Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, Multiplier: "1"}
	if _, err = service.AddProvider(ctx, definition, "default", "secret"); err != nil {
		t.Fatal(err)
	}
	definition.Multiplier = "2"
	if _, err = service.PlanProviderCredential(ctx, definition, "default"); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("metadata drift error = %v", err)
	}
}

func TestCurrentAndStatusReportCredentialShorthandWithoutOpeningSecrets(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	created, err := service.AddProvider(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, Multiplier: "1.25"}, "work", "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}
	selectedAt := time.Date(2026, 7, 20, 1, 2, 3, 0, time.UTC)
	if err = database.RecordSelection(ctx, store.Selection{ProviderID: created.ID, Client: "codex", ProviderName: "example", EndpointSnapshot: "https://provider.example/v1", MultiplierSnapshot: "1.25", CredentialName: "work", SelectedAt: selectedAt}); err != nil {
		t.Fatal(err)
	}
	rejecting := &rejectingCredentialVault{}
	service.Vault = rejecting
	current, err := service.Current(ctx)
	if err != nil || len(current) != 1 || current[0].Client != "codex" || current[0].Provider != "example" || current[0].Credential != "work" || current[0].SelectedAt != selectedAt.Format(time.RFC3339Nano) {
		t.Fatalf("current = %#v, %v", current, err)
	}
	statuses, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var active []ActiveSelection
	for _, status := range statuses {
		if status.Definition.Name == "example" {
			active = status.Active
		}
	}
	if len(active) != 1 || active[0].Client != "codex" || active[0].Credential != "work" || active[0].SelectedAt != selectedAt.Format(time.RFC3339Nano) {
		t.Fatalf("active status = %#v", active)
	}
	if rejecting.calls != 0 {
		t.Fatalf("read-only selection reporting opened credential secrets %d time(s)", rejecting.calls)
	}
}
