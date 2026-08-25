package provider

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/errdefs"
	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
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

func TestUpdateNamedCredentialReplacesLegacyKeyIDWithCurrentVersion(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, t.TempDir())
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
	keyIDs, err := vault.InspectKeyIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET key_version=?, key_id=? WHERE credential_id=?`, credentialvault.KeyVersionLegacy, keyIDs[credentialvault.KeyVersionLegacy], credential.ID); err != nil {
		t.Fatal(err)
	}
	value := "new-secret"
	if _, err = service.UpdateNamedCredential(ctx, "example", "default", nil, nil, nil, &value); err != nil {
		t.Fatalf("UpdateNamedCredential() mixed version store: %v", err)
	}
	secret, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret.KeyVersion != credentialvault.KeyVersion || secret.KeyID != keyIDs[credentialvault.KeyVersion] {
		t.Fatalf("rewritten secret key = (%d, %q), want current version and ID", secret.KeyVersion, secret.KeyID)
	}
}

func TestUpdateNamedCredentialRejectsKeyIDFromDifferentVersion(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, t.TempDir())
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
	keyIDs, err := vault.InspectKeyIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET key_version=?, key_id=? WHERE credential_id=?`, credentialvault.KeyVersionLegacy, keyIDs[credentialvault.KeyVersion], credential.ID); err != nil {
		t.Fatal(err)
	}
	before, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	value := "new-secret"
	if _, err = service.UpdateNamedCredential(ctx, "example", "default", nil, nil, nil, &value); !errors.Is(err, credentialvault.ErrKeyMachineMismatch) {
		t.Fatalf("UpdateNamedCredential() cross-version key ID error = %v, want %v", err, credentialvault.ErrKeyMachineMismatch)
	}
	after, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("cross-version secret overwritten: before=%#v after=%#v", before, after)
	}
}

func TestUpdateNamedCredentialRejectsUnsupportedStoredKeyVersion(t *testing.T) {
	for _, version := range []int{0, 3} {
		t.Run(fmt.Sprintf("version-%d", version), func(t *testing.T) {
			ctx := context.Background()
			database, err := store.Open(ctx, t.TempDir())
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
			keyIDs, err := vault.InspectKeyIDs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = database.Exec(ctx, `UPDATE credential_secrets SET key_version=?, key_id=? WHERE credential_id=?`, version, keyIDs[credentialvault.KeyVersion], credential.ID); err != nil {
				t.Fatal(err)
			}
			before, err := database.CredentialSecret(ctx, credential.ID)
			if err != nil {
				t.Fatal(err)
			}
			value := "new-secret"
			if _, err = service.UpdateNamedCredential(ctx, "example", "default", nil, nil, nil, &value); !errors.Is(err, credentialvault.ErrKeyVersionUnsupported) {
				t.Fatalf("UpdateNamedCredential() unsupported version error = %v, want %v", err, credentialvault.ErrKeyVersionUnsupported)
			}
			after, err := database.CredentialSecret(ctx, credential.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("unsupported-version secret overwritten: before=%#v after=%#v", before, after)
			}
		})
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

func (s *rejectingCredentialVault) InspectKeyIDs(context.Context) (map[int]string, error) {
	return nil, s.reject()
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
	officialClients := map[string]bool{}
	for _, mapping := range providers[0].Definition.Clients {
		officialClients[mapping.Client] = true
	}
	if len(providers) != 1 || providers[0].Definition.Name != OfficialProviderName || !providers[0].Definition.BuiltIn || providers[0].Definition.Authentication != "client_existing_login" || providers[0].Definition.CredentialCount != 0 || len(officialClients) != 2 || !officialClients[string(ClientCodex)] || !officialClients[string(ClientClaude)] {
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

func TestUseCredentialPropagatesProviderNotFound(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}

	err = service.UseCredential(ctx, "missing-provider", ClientCodex, "", filepath.Join(t.TempDir(), "config.toml"), "", false)
	var notFound *errdefs.NotFound
	if !errors.As(err, &notFound) || notFound.Code != store.CodeProviderNotFound {
		t.Fatalf("UseCredential error = %#v, %v", notFound, err)
	}
	if !errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "sql:") {
		t.Fatalf("UseCredential error = %v", err)
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
	if err = service.UseCredential(ctx, "shared", ClientCodex, "", config, filepath.Join(root, "ambiguous.toml"), false); err == nil {
		t.Fatal("ambiguous credential selection succeeded")
	}
	if err = service.UseCredential(ctx, "shared", ClientCodex, "work", config, filepath.Join(root, "work.toml"), false); err != nil {
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

func TestUpdateDefinitionResolvesCredentialAndRejectsAmbiguityOrBuiltInProvider(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}

	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, Multiplier: "1"}, "default-secret"); err != nil {
		t.Fatal(err)
	}

	endpoint := "https://resolved.example"
	multiplier := "3"
	if _, err = service.UpdateDefinition(ctx, "example", "", &endpoint, nil, &multiplier); err != nil {
		t.Fatal(err)
	}
	defaultCredential, err := database.ProviderCredential(ctx, "example", "default")
	if err != nil {
		t.Fatal(err)
	}
	if defaultCredential.Endpoint != endpoint || defaultCredential.Multiplier != "3.000000000000" || !strings.EqualFold(defaultCredential.CredentialRef, "example-default-ref") {
		t.Fatalf("resolved update credential = %#v", defaultCredential)
	}

	if _, err = service.AddCredential(ctx, "example", "work", "https://provider.example", "1", []Client{ClientCodex}, "work-secret"); err != nil {
		t.Fatal(err)
	}
	explicitEndpoint := "https://work.example"
	if _, err = service.UpdateDefinition(ctx, "example", "work", &explicitEndpoint, nil, nil); err != nil {
		t.Fatal(err)
	}
	workCredential, err := database.ProviderCredential(ctx, "example", "work")
	if err != nil {
		t.Fatal(err)
	}
	if workCredential.Endpoint != explicitEndpoint {
		t.Fatalf("explicit update endpoint = %q", workCredential.Endpoint)
	}
	if _, err = service.AddCredential(ctx, "example", "extra", "https://extra.example", "1", []Client{ClientCodex}, "extra-secret"); err != nil {
		t.Fatal(err)
	}
	// UpdateDefinition resolves the credential *before* mutating anything, and
	// that ordering is the whole protection here: an implementation that wrote
	// to the default or work credential and only then discovered the ambiguity
	// would leave `extra` untouched, so checking `extra` alone cannot tell the
	// two apart. Snapshot every credential and require exact equality after the
	// call fails.
	before, err := credentialSnapshot(ctx, database, "example")
	if err != nil {
		t.Fatal(err)
	}
	ambiguousEndpoint := "https://ambiguous.example"
	if _, err = service.UpdateDefinition(ctx, "example", "", &ambiguousEndpoint, nil, nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("ambiguity not rejected: %v", err)
	}
	after, err := credentialSnapshot(ctx, database, "example")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("a rejected ambiguous update mutated stored credentials:\nbefore %#v\nafter  %#v", before, after)
	}

	officialEndpoint := "https://official.example"
	if _, err = service.UpdateDefinition(ctx, OfficialProviderName, "", &officialEndpoint, nil, nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatal("official provider update succeeded")
	}
	official, err := service.Show(ctx, OfficialProviderName)
	if err != nil || !official.Definition.BuiltIn || official.Definition.Name != OfficialProviderName {
		t.Fatalf("official provider visibility = %#v, %v", official, err)
	}
}

func TestResolveCredentialNameAndShowCredentialProtectsSecretsAndAmbiguity(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}

	if _, err = service.Add(ctx, Definition{Name: "single", Endpoint: "https://single.invalid", Clients: []Client{ClientCodex}, Multiplier: "1"}, "single-secret"); err != nil {
		t.Fatal(err)
	}
	credential, err := service.ResolveCredentialName(ctx, "single", "")
	if err != nil || credential != "default" {
		t.Fatalf("unique resolve = %q, %v", credential, err)
	}
	if _, err = service.AddCredential(ctx, "single", "other", "https://other.invalid", "1", []Client{ClientCodex}, "other-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveCredentialName(ctx, "single", ""); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("multiple credentials not rejected: %v", err)
	}

	exposed, err := service.ResolveCredentialName(ctx, "single", "missing")
	if exposed != "" || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing credential lookup = %q, %v", exposed, err)
	}
	show, err := service.ShowCredential(ctx, "single", "default")
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(show)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "single-secret") {
		t.Fatalf("showed credential secret = %q", serialized)
	}
	if show.Provider != "single" || show.Name != "default" || show.Reference == "" || !show.Present || len(show.Clients) != 1 {
		t.Fatalf("show credential metadata = %#v", show)
	}

	if _, err = service.ShowCredential(ctx, "single", "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("show missing credential = %v", err)
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
	if err = service.UseCredential(ctx, "used", ClientCodex, "work", config, filepath.Join(root, "backup.toml"), false); err != nil {
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

func TestServiceFailedProviderSelectionIsIsolatedAcrossClients(t *testing.T) {
	tests := []struct {
		name             string
		failJournalWrite bool
		wantState        string
		wantCode         string
	}{
		{name: "selection persistence failure", wantState: "failed", wantCode: "selection_commit_failed"},
		{name: "selection and failure journal errors", failJournalWrite: true, wantState: "external_written"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProviderSelectionIsolationFixture(t)
			if _, err := fixture.database.Exec(context.Background(), `CREATE TRIGGER fail_later_selection BEFORE INSERT ON provider_selections BEGIN SELECT RAISE(FAIL,'synthetic later selection failure'); END`); err != nil {
				t.Fatal(err)
			}
			if test.failJournalWrite {
				if _, err := fixture.database.Exec(context.Background(), `CREATE TRIGGER fail_failure_journal BEFORE UPDATE OF state ON operations WHEN NEW.state='failed' BEGIN SELECT RAISE(FAIL,'synthetic failed-transition failure'); END`); err != nil {
					t.Fatal(err)
				}
			}

			useErr := fixture.service.UseCredential(context.Background(), "next", ClientCodex, "shared", fixture.codexConfig, filepath.Join(fixture.root, "next-codex.backup.toml"), false)
			if useErr == nil || !strings.Contains(useErr.Error(), "synthetic later selection failure") {
				t.Fatalf("later Codex selection error = %v", useErr)
			}
			claudeBytes, err := os.ReadFile(fixture.claudeConfig)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(claudeBytes, fixture.claudeBytes) {
				t.Fatalf("completed Claude config changed:\nbefore %q\nafter  %q", fixture.claudeBytes, claudeBytes)
			}
			claudeSelection, err := fixture.database.CurrentProviderSnapshot(context.Background(), string(ClientClaude))
			if err != nil || !reflect.DeepEqual(claudeSelection, fixture.claudeSelection) {
				t.Fatalf("completed Claude selection changed:\nbefore %#v\nafter  %#v\nerror  %v", fixture.claudeSelection, claudeSelection, err)
			}
			codexSelection, err := fixture.database.CurrentProviderSnapshot(context.Background(), string(ClientCodex))
			if err != nil || !reflect.DeepEqual(codexSelection, fixture.codexSelection) {
				t.Fatalf("failed Codex selection replaced prior state:\nbefore %#v\nafter  %#v\nerror  %v", fixture.codexSelection, codexSelection, err)
			}
			codexBytes, err := os.ReadFile(fixture.codexConfig)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(codexBytes, fixture.codexBytes) {
				t.Fatal("Codex config did not retain the externally written provider")
			}
			matches, err := ConfigMatchesEndpoint(ClientCodex, fixture.codexConfig, "https://next.invalid")
			if err != nil || !matches {
				t.Fatalf("Codex external state = %q, matches=%t, error=%v", codexBytes, matches, err)
			}

			pending, err := fixture.database.PendingOperations(context.Background())
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending operations = %#v, %v", pending, err)
			}
			operation := pending[0]
			if operation.ID == "" || operation.Kind != "provider.use" || operation.ProviderID == nil || *operation.ProviderID != fixture.nextProviderID || operation.Client != string(ClientCodex) || operation.ResourceName != "next" || operation.State != test.wantState || operation.ErrorCode != test.wantCode {
				t.Fatalf("failed Codex journal = %#v", operation)
			}
			var details providerUseDetails
			if err := json.Unmarshal([]byte(operation.DetailsJSON), &details); err != nil || details.ConfigPath != fixture.codexConfig {
				t.Fatalf("failed Codex journal = %#v", pending[0])
			}
		})
	}
}

func TestServiceFailOperationJoinsPrimaryCauseAndSQLiteJournalError(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database}
	const operationID = "fail-operation"
	if err := database.CreateOperation(ctx, store.Operation{ID: operationID, Kind: "provider.use", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `CREATE TRIGGER fail_operation_update BEFORE UPDATE OF state ON operations WHEN NEW.state='failed' BEGIN SELECT RAISE(FAIL,'synthetic failed-transition failure'); END`); err != nil {
		t.Fatal(err)
	}

	primarySentinel := errors.New("primary provider use failure")
	result := service.failOperation(ctx, operationID, "selection_commit_failed", primarySentinel)
	if !errors.Is(result, primarySentinel) {
		t.Fatalf("joined failure does not preserve primary cause: %v", result)
	}
	if !strings.Contains(result.Error(), "record operation failure") {
		t.Fatalf("failure journal wrapper missing: %v", result)
	}
	var sqliteErr *sqlite.Error
	if !errors.As(result, &sqliteErr) || !strings.Contains(sqliteErr.Error(), "synthetic failed-transition failure") {
		t.Fatalf("joined failure does not expose SQLite journal cause: %v", result)
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
	if err = service.UseCredential(ctx, "example", ClientCodex, "default", config, filepath.Join(root, "default.backup"), false); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TRIGGER fail_completed_selection BEFORE INSERT ON provider_selections BEGIN SELECT RAISE(FAIL,'injected final selection failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "example", ClientCodex, "work", config, filepath.Join(root, "work.backup"), false); err == nil {
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
	if err = service.UseCredential(ctx, "sssaicode", ClientCodex, "codex", config, filepath.Join(t.TempDir(), "backup.toml"), false); err != nil {
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

// credentialSnapshot captures every field of every credential of a provider
// that a mutation could plausibly touch: endpoint, multiplier, client mapping,
// and the credential reference. Tests use it to assert that a call which
// returns an error changed nothing at all, rather than that one credential the
// test happened to look at is unchanged.
func credentialSnapshot(ctx context.Context, database *store.Store, provider string) (map[string][]string, error) {
	items, err := database.ListProviderCredentials(ctx, provider)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string][]string, len(items))
	for _, item := range items {
		clients := append([]string(nil), item.Clients...)
		sort.Strings(clients)
		snapshot[item.Name] = []string{
			item.Endpoint,
			item.Multiplier,
			item.CredentialRef,
			strings.Join(clients, ","),
		}
	}
	return snapshot, nil
}

type providerSelectionIsolationFixture struct {
	root            string
	database        *store.Store
	service         Service
	codexConfig     string
	claudeConfig    string
	codexBytes      []byte
	claudeBytes     []byte
	codexSelection  store.ProviderSnapshot
	claudeSelection store.ProviderSnapshot
	nextProviderID  int64
}

func newProviderSelectionIsolationFixture(t *testing.T) providerSelectionIsolationFixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close provider isolation store: %v", closeErr)
		}
	})
	service := Service{Store: database, Vault: testCredentialVault(t)}
	clients := []Client{ClientCodex, ClientClaude}
	if _, err = service.AddProvider(ctx, Definition{Name: "stable", Endpoint: "https://stable.invalid", Clients: clients, Multiplier: "1"}, "shared", "stable-synthetic-value"); err != nil {
		t.Fatal(err)
	}
	nextProvider, err := service.AddProvider(ctx, Definition{Name: "next", Endpoint: "https://next.invalid", Clients: clients, Multiplier: "2"}, "shared", "next-synthetic-value")
	if err != nil {
		t.Fatal(err)
	}

	codexConfig := filepath.Join(root, "config.toml")
	claudeConfig := filepath.Join(root, "settings.json")
	if err = os.WriteFile(codexConfig, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(claudeConfig, []byte("{\"theme\":\"keep\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "stable", ClientClaude, "shared", claudeConfig, filepath.Join(root, "stable-claude.backup.json"), false); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, "stable", ClientCodex, "shared", codexConfig, filepath.Join(root, "stable-codex.backup.toml"), false); err != nil {
		t.Fatal(err)
	}

	codexBytes, err := os.ReadFile(codexConfig)
	if err != nil {
		t.Fatal(err)
	}
	claudeBytes, err := os.ReadFile(claudeConfig)
	if err != nil {
		t.Fatal(err)
	}
	codexSelection, err := database.CurrentProviderSnapshot(ctx, string(ClientCodex))
	if err != nil {
		t.Fatal(err)
	}
	claudeSelection, err := database.CurrentProviderSnapshot(ctx, string(ClientClaude))
	if err != nil {
		t.Fatal(err)
	}
	return providerSelectionIsolationFixture{
		root:            root,
		database:        database,
		service:         service,
		codexConfig:     codexConfig,
		claudeConfig:    claudeConfig,
		codexBytes:      codexBytes,
		claudeBytes:     claudeBytes,
		codexSelection:  codexSelection,
		claudeSelection: claudeSelection,
		nextProviderID:  nextProvider.ID,
	}
}
