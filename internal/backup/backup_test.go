package backup

import (
	"archive/tar"
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

	"filippo.io/age"

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
	if err = os.WriteFile(providerpkg.ProjectAttributionGatePath(stateRoot), nil, 0o600); err != nil {
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
	if _, included := archiveEntries[providerpkg.ProjectAttributionGateFilename]; included {
		t.Fatal("portable backup included project-attribution eligibility marker")
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

func TestRestoreRejectsInvalidArchivesBeforeTouchingTarget(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const sourceCredential = "synthetic-restore-boundary-value"
	const archivePassphrase = "synthetic-archive-passphrase"
	vault := testBackupVault(t, "source-machine")
	providers := providerpkg.Service{Store: database, Vault: vault}
	if _, err = providers.Add(ctx, providerpkg.Definition{Name: "restore-boundary", Endpoint: "https://example.invalid", Clients: []providerpkg.Client{providerpkg.ClientCodex}}, sourceCredential); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "valid.adb")
	if _, err = (Service{Core: database, StateRoot: stateRoot, Vault: vault, Version: "test"}).Create(ctx, archive, archivePassphrase, false); err != nil {
		t.Fatal(err)
	}

	writeMalformedArchive := func(t *testing.T, name string, mutate func(*Manifest, map[string][]byte)) string {
		t.Helper()
		manifest, entries, readErr := readEncrypted(archive, archivePassphrase)
		if readErr != nil {
			t.Fatal(readErr)
		}
		mutate(&manifest, entries)
		manifestBytes, marshalErr := json.Marshal(manifest)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		entries[manifestName] = manifestBytes
		path := filepath.Join(t.TempDir(), name+".adb")
		if writeErr := writeEncrypted(path, archivePassphrase, entries, time.Unix(1, 0)); writeErr != nil {
			t.Fatal(writeErr)
		}
		return path
	}

	cases := []struct {
		name           string
		existingTarget bool
		unusableTarget bool
		prepare        func(*testing.T) (string, string)
	}{
		{
			name: "wrong passphrase leaves nonexistent target absent",
			prepare: func(t *testing.T) (string, string) {
				return archive, "wrong-passphrase"
			},
		},
		{
			name:           "truncated archive preserves empty target mode",
			existingTarget: true,
			prepare: func(t *testing.T) (string, string) {
				ciphertext, readErr := os.ReadFile(archive)
				if readErr != nil {
					t.Fatal(readErr)
				}
				path := filepath.Join(t.TempDir(), "truncated.adb")
				if writeErr := os.WriteFile(path, ciphertext[:len(ciphertext)/2], platform.FileMode); writeErr != nil {
					t.Fatal(writeErr)
				}
				return path, archivePassphrase
			},
		},
		{
			name: "missing required entry leaves target absent",
			prepare: func(t *testing.T) (string, string) {
				return writeMalformedArchive(t, "missing-core", func(_ *Manifest, entries map[string][]byte) {
					delete(entries, coreName)
				}), archivePassphrase
			},
		},
		{
			name: "hash mismatch leaves target absent without exposing sensitive input",
			prepare: func(t *testing.T) (string, string) {
				return writeMalformedArchive(t, "hash-mismatch", func(manifest *Manifest, _ map[string][]byte) {
					manifest.Entries[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
				}), archivePassphrase
			},
		},
		{
			name: "unexpected path leaves target absent",
			prepare: func(t *testing.T) (string, string) {
				return writeMalformedArchive(t, "unexpected-entry", func(_ *Manifest, entries map[string][]byte) {
					entries["unexpected"] = []byte("synthetic")
				}), archivePassphrase
			},
		},
		{
			name: "duplicate entry leaves target absent",
			prepare: func(t *testing.T) (string, string) {
				manifest, entries, readErr := readEncrypted(archive, archivePassphrase)
				if readErr != nil {
					t.Fatal(readErr)
				}
				path := filepath.Join(t.TempDir(), "duplicate.adb")
				writeEncryptedArchiveWithDuplicate(t, path, archivePassphrase, entries, manifest.CreatedAt, coreName)
				return path, archivePassphrase
			},
		},
		{
			name:           "invalid archive is rejected before unusable target",
			unusableTarget: true,
			prepare: func(t *testing.T) (string, string) {
				return writeMalformedArchive(t, "ordering-probe", func(_ *Manifest, entries map[string][]byte) {
					delete(entries, coreName)
				}), archivePassphrase
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			archivePath, passphrase := test.prepare(t)
			target := filepath.Join(t.TempDir(), "target")
			if test.unusableTarget {
				parent := filepath.Join(t.TempDir(), "regular-file")
				if writeErr := os.WriteFile(parent, []byte("unchanged"), platform.FileMode); writeErr != nil {
					t.Fatal(writeErr)
				}
				target = filepath.Join(parent, "target")
			} else if test.existingTarget {
				if mkdirErr := os.Mkdir(target, 0o755); mkdirErr != nil {
					t.Fatal(mkdirErr)
				}
				if chmodErr := os.Chmod(target, 0o755); chmodErr != nil {
					t.Fatal(chmodErr)
				}
			}

			_, restoreErr := Restore(ctx, archivePath, target, passphrase, syntheticMachineIdentity("target-machine"))
			if restoreErr == nil {
				t.Fatal("Restore succeeded, want invalid archive")
			}
			if strings.Contains(restoreErr.Error(), sourceCredential) || strings.Contains(restoreErr.Error(), archivePassphrase) || strings.Contains(restoreErr.Error(), passphrase) {
				t.Fatal("restore error exposed sensitive test input")
			}
			if !errors.Is(restoreErr, ErrInvalidArchive) {
				t.Fatal("Restore returned a non-invalid-archive error")
			}
			if test.unusableTarget {
				contents, readErr := os.ReadFile(filepath.Dir(target))
				if readErr != nil || string(contents) != "unchanged" {
					t.Fatalf("unusable target parent changed: %v", readErr)
				}
				return
			}
			assertRestoreTargetUnchanged(t, target, test.existingTarget)
		})
	}
}

func writeEncryptedArchiveWithDuplicate(t *testing.T, destination, passphrase string, entries map[string][]byte, createdAt time.Time, duplicate string) {
	t.Helper()
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := age.Encrypt(file, recipient)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(encrypted)
	for _, name := range sortedKeys(entries) {
		data := entries[name]
		header := &tar.Header{Name: name, Mode: int64(platform.FileMode), Size: int64(len(data)), ModTime: createdAt}
		if err = archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err = archive.Write(data); err != nil {
			t.Fatal(err)
		}
		if name == duplicate {
			if err = archive.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if _, err = archive.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err = encrypted.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertRestoreTargetUnchanged(t *testing.T, target string, existed bool) {
	t.Helper()
	info, err := os.Stat(target)
	if !existed {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("target exists after invalid archive: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("target mode = %v, want 0755", info.Mode())
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("target entries = %v, want none", entries)
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
