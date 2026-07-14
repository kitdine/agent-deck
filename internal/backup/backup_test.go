package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestEncryptedBackupInspectAndEmptyRootRestore(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.CreateProvider(ctx, store.Provider{Name: "synthetic", Endpoint: "https://example.invalid", CredentialRef: "agentdeck:test", Multiplier: "1", Clients: []store.ClientMapping{{Client: "codex"}}}); err != nil {
		t.Fatal(err)
	}
	secrets := platform.NewMemorySecretStore()
	if err = secrets.Put(ctx, "agentdeck:test", "synthetic-secret-value"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(stateRoot, "backups", "portable", "sample.adb")
	service := Service{Core: database, StateRoot: stateRoot, Secrets: secrets, Version: "test", Now: func() time.Time { return time.Unix(1, 0) }}
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
	restoredSecrets := platform.NewMemorySecretStore()
	if _, err = Restore(ctx, archive, target, "correct horse battery staple", restoredSecrets); err != nil {
		t.Fatal(err)
	}
	value, err := restoredSecrets.Get(ctx, "agentdeck:test")
	if err != nil || value != "synthetic-secret-value" {
		t.Fatalf("restored credential = %q, %v", value, err)
	}
	restored, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	providers, err := restored.ListProviders(ctx)
	if err != nil || len(providers) != 1 || providers[0].Name != "synthetic" {
		t.Fatalf("restored providers = %#v, %v", providers, err)
	}
	if info, err := os.Stat(filepath.Join(target, coreName)); err != nil || info.Mode().Perm() != platform.FileMode {
		t.Fatalf("restored database mode = %v, %v", info, err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != platform.DirectoryMode {
		t.Fatalf("restored state root mode = %v, %v", info.Mode(), err)
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
	service := Service{Core: database, StateRoot: stateRoot, Secrets: platform.NewMemorySecretStore(), Version: "test"}
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
	if _, err = (Service{Core: database, StateRoot: stateRoot, Secrets: platform.NewMemorySecretStore()}).Create(ctx, filepath.Join(parent, "portable.adb"), "passphrase", false); err != nil {
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
	service := Service{Core: database, StateRoot: stateRoot, Secrets: platform.NewMemorySecretStore()}
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
	if _, err = (Service{Core: database, StateRoot: stateRoot, Secrets: platform.NewMemorySecretStore()}).Create(ctx, archive, "passphrase", true); err != nil {
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
	if _, err = Restore(ctx, tampered, filepath.Join(t.TempDir(), "target"), "passphrase", platform.NewMemorySecretStore()); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Restore error = %v", err)
	}
}

func TestRestoreRejectsConflictsWithoutWritingTarget(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.CreateProvider(ctx, store.Provider{Name: "synthetic", Endpoint: "https://example.invalid", CredentialRef: "agentdeck:test", Multiplier: "1", Clients: []store.ClientMapping{{Client: "claude"}}}); err != nil {
		t.Fatal(err)
	}
	secrets := platform.NewMemorySecretStore()
	_ = secrets.Put(ctx, "agentdeck:test", "archive-secret")
	service := Service{Core: database, StateRoot: stateRoot, Secrets: secrets, Version: "test"}
	archive := filepath.Join(t.TempDir(), "sample.adb")
	if _, err = service.Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	restoredSecrets := platform.NewMemorySecretStore()
	_ = restoredSecrets.Put(ctx, "agentdeck:test", "existing-secret")
	if _, err = Restore(ctx, archive, target, "passphrase", restoredSecrets); !errors.Is(err, ErrSecretConflict) {
		t.Fatalf("Restore error = %v", err)
	}
	if _, err = os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target created after conflict: %v", err)
	}
}

func TestRestoreRollsBackFilesAndCredentialsCreatedByFailedAttempt(t *testing.T) {
	ctx := context.Background()
	stateRoot := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for index, reference := range []string{"agentdeck:first", "agentdeck:second"} {
		if _, err = database.CreateProvider(ctx, store.Provider{Name: reference, Endpoint: "https://example.invalid", CredentialRef: reference, Multiplier: "1", Clients: []store.ClientMapping{{Client: []string{"codex", "claude"}[index]}}}); err != nil {
			t.Fatal(err)
		}
	}
	sourceSecrets := platform.NewMemorySecretStore()
	_ = sourceSecrets.Put(ctx, "agentdeck:first", "first-secret")
	_ = sourceSecrets.Put(ctx, "agentdeck:second", "second-secret")
	archive := filepath.Join(t.TempDir(), "sample.adb")
	if _, err = (Service{Core: database, StateRoot: stateRoot, Secrets: sourceSecrets, Version: "test"}).Create(ctx, archive, "passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err = os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(target, 0o755); err != nil {
		t.Fatal(err)
	}
	failing := &failingSecretStore{MemorySecretStore: platform.NewMemorySecretStore(), failReference: "agentdeck:second"}
	if _, err = Restore(ctx, archive, target, "passphrase", failing); err == nil {
		t.Fatal("Restore succeeded")
	}
	if info, statErr := os.Stat(target); statErr != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("failed restore target mode = %v, %v", info.Mode(), statErr)
	}
	if present, err := failing.Exists(ctx, "agentdeck:first"); err != nil || present {
		t.Fatalf("failed restore retained credential: present=%t err=%v", present, err)
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

type failingSecretStore struct {
	*platform.MemorySecretStore
	failReference string
}

func (s *failingSecretStore) Create(ctx context.Context, reference, value string) error {
	if reference == s.failReference {
		return errors.New("synthetic secret-store failure")
	}
	return s.MemorySecretStore.Create(ctx, reference, value)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
