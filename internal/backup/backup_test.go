package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/platform"
	providerpkg "github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/store"
)

func syntheticMachineIdentity(value string) credentialvault.MachineIdentity {
	return func(context.Context) (string, error) { return value, nil }
}

func testBackupVault(t *testing.T, machine string) *credentialvault.Vault {
	t.Helper()
	return credentialvault.New(filepath.Join(t.TempDir(), "vault"), syntheticMachineIdentity(machine))
}

func assertRestoredCredential(t *testing.T, ctx context.Context, target, machine, providerName, name, want string) {
	t.Helper()
	database, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	credential, err := database.ProviderCredential(ctx, providerName, name)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := database.CredentialSecret(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	vault := credentialvault.New(target, syntheticMachineIdentity(machine))
	value, err := vault.Open(ctx, credential.CredentialRef, credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext})
	if err != nil || value != want {
		t.Fatalf("restored %s/%s = %q, %v", providerName, name, value, err)
	}
}

func TestPortableBackupRestoreIncludesAllNamedCredentials(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: vault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "multi", Endpoint: "https://example.invalid", CredentialRef: "multi-ref", Multiplier: "1", Clients: []providerpkg.Client{providerpkg.ClientCodex, providerpkg.ClientClaude}}, "default-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = providers.AddCredential(ctx, "multi", "work", "https://example.invalid", "1", []providerpkg.Client{providerpkg.ClientCodex}, "work-secret"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "multi.adb")
	if _, err = (Service{Core: database, StateRoot: state, Vault: vault, Version: "test"}).Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored")
	if _, err = Restore(ctx, archive, target, "passphrase", syntheticMachineIdentity("target-machine")); err != nil {
		t.Fatal(err)
	}
	assertRestoredCredential(t, ctx, target, "target-machine", "multi", "default", "default-secret")
	assertRestoredCredential(t, ctx, target, "target-machine", "multi", "work", "work-secret")
	restored, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := restored.ProviderCredential(ctx, "multi", "default")
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	secret, err := restored.CredentialSecret(ctx, credential.ID)
	if closeErr := restored.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	_, err = credentialvault.New(target, syntheticMachineIdentity("source-machine")).Open(ctx, credential.CredentialRef, credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext})
	if !errors.Is(err, credentialvault.ErrKeyMachineMismatch) {
		t.Fatalf("source machine opened restored credential: %v", err)
	}
}

func TestPortableBackupUsesOneCoreSnapshotAcrossCredentialMutations(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(context.Context, providerpkg.Service) error
	}{
		{
			name: "add",
			mutate: func(ctx context.Context, service providerpkg.Service) error {
				_, err := service.AddCredential(ctx, "snapshot", "work", "https://example.invalid", "1", []providerpkg.Client{providerpkg.ClientCodex}, "work-secret")
				return err
			},
		},
		{
			name: "delete",
			mutate: func(ctx context.Context, service providerpkg.Service) error {
				return service.RemoveNamedCredential(ctx, "snapshot", "default")
			},
		},
		{
			name: "rotate",
			mutate: func(ctx context.Context, service providerpkg.Service) error {
				value := "rotated-secret"
				_, err := service.UpdateNamedCredential(ctx, "snapshot", "default", nil, nil, nil, &value)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			state := filepath.Join(t.TempDir(), "state")
			database, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			vault := credentialvault.New(state, syntheticMachineIdentity("source-machine"))
			providers := providerpkg.Service{Store: database, Vault: vault}
			if _, err = providers.Add(ctx, providerpkg.Definition{Name: "snapshot", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientCodex}}, "snapshot-secret"); err != nil {
				t.Fatal(err)
			}

			var mutationErr error
			archive := filepath.Join(t.TempDir(), test.name+".adb")
			service := Service{
				Core:      database,
				StateRoot: state,
				Vault:     vault,
				Version:   "test",
				AfterCoreSnapshot: func() {
					mutationErr = test.mutate(ctx, providers)
				},
			}
			if _, err = service.Create(ctx, archive, "passphrase", false); err != nil {
				t.Fatal(err)
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}

			target := filepath.Join(t.TempDir(), "restored")
			if _, err = Restore(ctx, archive, target, "passphrase", syntheticMachineIdentity("target-machine")); err != nil {
				t.Fatal(err)
			}
			assertRestoredCredential(t, ctx, target, "target-machine", "snapshot", "default", "snapshot-secret")
			restored, err := store.OpenReadOnly(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			_, workErr := restored.ProviderCredential(ctx, "snapshot", "work")
			if closeErr := restored.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !errors.Is(workErr, sql.ErrNoRows) {
				t.Fatalf("snapshot unexpectedly contains post-snapshot credential: %v", workErr)
			}
		})
	}
}

func TestPortableBackupIgnoresRemovedLastCredential(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: vault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "empty", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err = providers.RemoveNamedCredential(ctx, "empty", "default"); err != nil {
		t.Fatal(err)
	}

	service := Service{Core: database, StateRoot: state, Vault: vault, Version: "test"}
	encoded, err := service.credentials(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	var credentials []Credential
	if err = json.Unmarshal(encoded, &credentials); err != nil || len(credentials) != 0 {
		t.Fatalf("backup credentials = %#v, %v", credentials, err)
	}
	archive := filepath.Join(t.TempDir(), "empty.adb")
	if _, err = service.Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored")
	if _, err = Restore(ctx, archive, target, "passphrase", syntheticMachineIdentity("target-machine")); err != nil {
		t.Fatal(err)
	}
	restored, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	items, err := restored.ListProviders(ctx)
	if err != nil || len(items) != 1 || len(items[0].Credentials) != 0 {
		t.Fatalf("restored providers = %#v, %v", items, err)
	}
}

func TestEncryptedBackupInspectAndEmptyRootRestore(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := credentialvault.New(stateRoot, syntheticMachineIdentity("source-machine"))
	providers := providerpkg.Service{Store: database, Vault: vault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "synthetic", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientCodex}}, "synthetic-secret-value"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(stateRoot, "backups", "portable", "sample.adb")
	service := Service{Core: database, StateRoot: stateRoot, Vault: vault, Version: "test", Now: func() time.Time { return time.Unix(1, 0) }}
	manifest, err := service.Create(ctx, archive, "correct horse battery staple", false)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("synthetic-secret-value")) || bytes.Contains(ciphertext, []byte("agentdeck.sqlite3")) {
		t.Fatal("encrypted archive exposes plaintext")
	}
	if contains(manifest.Included, sessionsName) {
		t.Fatalf("default backup included sessions: %#v", manifest.Included)
	}
	inspected, err := service.Inspect(archive, "correct horse battery staple")
	if err != nil || inspected.SchemaVersion != 1 || inspected.AgentDeckVersion != "test" {
		t.Fatalf("Inspect = %#v, %v", inspected, err)
	}
	if _, err = service.Inspect(archive, "wrong passphrase"); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("wrong passphrase error = %v", err)
	}
	_, archiveEntries, err := readEncrypted(archive, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, included := archiveEntries["credential.key"]; included {
		t.Fatal("portable backup included credential.key")
	}
	tampered := filepath.Join(t.TempDir(), "tampered.adb")
	corrupted := append([]byte(nil), ciphertext...)
	corrupted[len(corrupted)-1] ^= 0xff
	if err = os.WriteFile(tampered, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Inspect(tampered, "correct horse battery staple"); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("tampered archive error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "restored")
	if err = os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err = Restore(ctx, archive, target, "correct horse battery staple", syntheticMachineIdentity("target-machine")); err != nil {
		t.Fatal(err)
	}
	assertRestoredCredential(t, ctx, target, "target-machine", "synthetic", "default", "synthetic-secret-value")
	restored, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	items, err := restored.ListProviders(ctx)
	if err != nil || len(items) != 1 || items[0].Name != "synthetic" {
		t.Fatalf("restored providers = %#v, %v", items, err)
	}
	if info, err := os.Stat(filepath.Join(target, coreName)); err != nil || info.Mode().Perm() != platform.FileMode {
		t.Fatalf("restored database mode = %v, %v", info, err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != platform.DirectoryMode {
		t.Fatalf("restored state root mode = %v, %v", info.Mode(), err)
	}
	if _, err := os.Stat(filepath.Join(target, "credential.key")); err != nil {
		t.Fatalf("restored credential key: %v", err)
	}
}

func TestBackupIncludesSessionsOnlyWhenRequested(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sessions, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	service := Service{Core: database, StateRoot: stateRoot, Vault: testBackupVault(t, "source-machine"), Version: "test"}
	archive := filepath.Join(t.TempDir(), "with-sessions.adb")
	manifest, err := service.Create(ctx, archive, "passphrase", true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(manifest.Included, sessionsName) {
		t.Fatalf("included = %#v", manifest.Included)
	}
	if manifest.DatabaseSchemas[sessionsName] != sessionSnapshotSchemaVersion {
		t.Fatalf("session schema = %#v", manifest.DatabaseSchemas)
	}
}

func TestBackupDoesNotChangeExistingDestinationParentMode(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	parent := filepath.Join(t.TempDir(), "exports")
	if err = os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err = (Service{Core: database, StateRoot: stateRoot, Vault: testBackupVault(t, "source-machine")}).Create(ctx, filepath.Join(parent, "portable.adb"), "passphrase", false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(parent)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("destination parent mode = %v, %v", info.Mode(), err)
	}
}

func TestBackupDoesNotOverwriteExistingDestination(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	destination := filepath.Join(t.TempDir(), "existing.adb")
	original := []byte("existing backup")
	if err = os.WriteFile(destination, original, platform.FileMode); err != nil {
		t.Fatal(err)
	}
	service := Service{Core: database, StateRoot: stateRoot, Vault: testBackupVault(t, "source-machine")}
	if _, err = service.Create(ctx, destination, "passphrase", false); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("backup existing destination error = %v", err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, original) {
		t.Fatalf("existing destination changed: %q", contents)
	}
}

func TestRestoreRejectsUnsupportedSessionSchema(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sessions, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "with-sessions.adb")
	if _, err = (Service{Core: database, StateRoot: stateRoot, Vault: testBackupVault(t, "source-machine")}).Create(ctx, archive, "passphrase", true); err != nil {
		t.Fatal(err)
	}
	manifest, entries, err := readEncrypted(archive, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	manifest.DatabaseSchemas[sessionsName]++
	entries[manifestName], err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "unsupported-session.adb")
	if err = writeEncrypted(tampered, "passphrase", entries, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = Restore(ctx, tampered, filepath.Join(t.TempDir(), "target"), "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Restore error = %v", err)
	}
}

func TestRestoreRejectsNonEmptyTargetWithoutChangingIt(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: vault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "synthetic", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientClaude}}, "archive-secret"); err != nil {
		t.Fatal(err)
	}
	service := Service{Core: database, StateRoot: stateRoot, Vault: vault, Version: "test"}
	archive := filepath.Join(t.TempDir(), "sample.adb")
	if _, err = service.Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err = os.Mkdir(target, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "keep")
	if err = os.WriteFile(marker, []byte("unchanged"), platform.FileMode); err != nil {
		t.Fatal(err)
	}
	if _, err = Restore(ctx, archive, target, "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("Restore error = %v", err)
	}
	if contents, readErr := os.ReadFile(marker); readErr != nil || string(contents) != "unchanged" {
		t.Fatalf("target changed after rejection: %q, %v", contents, readErr)
	}
}

func TestRestoreHoldsLockAndPreservesUnownedFilesDuringRollback(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sourceVault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: sourceVault}
	for index, name := range []string{"first", "second"} {
		client := []providerpkg.Client{providerpkg.ClientCodex, providerpkg.ClientClaude}[index]
		if _, err = providers.Add(ctx, providerpkg.Definition{Name: name, Endpoint: "https://example.invalid", Clients: []providerpkg.Client{client}}, []string{"first-secret", "second-secret"}[index]); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(t.TempDir(), "sample.adb")
	if _, err = (Service{Core: database, StateRoot: stateRoot, Vault: sourceVault, Version: "test"}).Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err = os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "concurrent-marker")
	lockObserved := false
	failingIdentity := func(ctx context.Context) (string, error) {
		lock, lockErr := store.AcquireLock(ctx, target, 0)
		if errors.Is(lockErr, store.ErrStateBusy) {
			lockObserved = true
		} else if lockErr == nil {
			_ = lock.Release()
		} else {
			return "", lockErr
		}
		if writeErr := os.WriteFile(marker, []byte("keep"), platform.FileMode); writeErr != nil {
			return "", writeErr
		}
		return "", errors.New("synthetic machine identity failure")
	}
	if _, err = Restore(ctx, archive, target, "passphrase", failingIdentity); err == nil {
		t.Fatal("Restore succeeded")
	}
	if !lockObserved {
		t.Fatal("restore did not hold the state lock while sealing credentials")
	}
	if info, statErr := os.Stat(target); statErr != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("failed restore target mode = %v, %v", info.Mode(), statErr)
	}
	if contents, readErr := os.ReadFile(marker); readErr != nil || string(contents) != "keep" {
		t.Fatalf("failed restore removed unowned marker: %q, %v", contents, readErr)
	}
	entries, readErr := os.ReadDir(target)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(marker) {
		t.Fatalf("failed restore retained files: %v, %v", entries, readErr)
	}
}

func TestRestoreDoesNotOwnCredentialKeyCreatedByConcurrentProcess(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sourceVault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: sourceVault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "concurrent", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "sample.adb")
	if _, err = (Service{Core: database, StateRoot: stateRoot, Vault: sourceVault, Version: "test"}).Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "target")
	if err = os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	originalInitializer := initializeRestoreCredentialKey
	t.Cleanup(func() { initializeRestoreCredentialKey = originalInitializer })
	var externalKey []byte
	initializeRestoreCredentialKey = func(ctx context.Context, vault *credentialvault.Vault) (bool, error) {
		contender := credentialvault.New(target, syntheticMachineIdentity("target-machine"))
		created, createErr := contender.InitializeNew(ctx)
		if createErr != nil || !created {
			return false, errors.Join(createErr, errors.New("concurrent credential key was not created"))
		}
		externalKey, createErr = os.ReadFile(contender.KeyPath())
		if createErr != nil {
			return false, createErr
		}
		return originalInitializer(ctx, vault)
	}

	if _, err = Restore(ctx, archive, target, "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("Restore error = %v, want target conflict", err)
	}
	keyPath := filepath.Join(target, "credential.key")
	contents, readErr := os.ReadFile(keyPath)
	if readErr != nil || !bytes.Equal(contents, externalKey) {
		t.Fatalf("restore removed or replaced concurrent key: %x, %v", contents, readErr)
	}
	entries, readErr := os.ReadDir(target)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(keyPath) {
		t.Fatalf("failed restore retained unexpected files: %v, %v", entries, readErr)
	}
}

func TestWriteNewPrivateFileNeverOverwritesExistingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), coreName)
	if err := os.WriteFile(path, []byte("existing"), platform.FileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := writeNewPrivateFile(path, []byte("replacement")); err == nil {
		t.Fatal("writeNewPrivateFile overwrote existing file")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "existing" {
		t.Fatalf("existing file = %q, %v", contents, err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
