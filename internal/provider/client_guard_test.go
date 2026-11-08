package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

type clientGuardResolver struct {
	account   string
	installed bool
}

func (r clientGuardResolver) AccountHome() (string, error) { return r.account, nil }
func (r clientGuardResolver) Resolve() platform.InstallationIdentity {
	if r.installed {
		return platform.InstallationIdentity{Installed: true, Channel: "make"}
	}
	return platform.InstallationIdentity{Reason: "receipt_missing"}
}

// injectAccount replaces the real account home with a temporary directory for
// this test only; TestMain's isolated resolver is restored afterwards.
func injectAccount(t *testing.T, installed bool) string {
	t.Helper()
	account, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	platform.SetInstallationResolver(clientGuardResolver{account: account, installed: installed})
	t.Cleanup(func() {
		platform.SetInstallationResolver(isolatedStateTestResolver{os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME")})
	})
	return account
}

func requireClientConfigRefusal(t *testing.T, err error) {
	t.Helper()
	var refused *platform.ErrStateOwnershipRefused
	if !errors.As(err, &refused) || refused.Resource != "client_config" || refused.Reason != "receipt_missing" {
		t.Fatalf("error = %v (%#v), want client_config refusal", err, refused)
	}
}

func writeGuardFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireDirectoryUnchanged(t *testing.T, path, contents string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != contents {
		t.Fatalf("%s = %q, %v; want unchanged %q", path, got, err, contents)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory entries = %v, %v; want only the original file", entries, err)
	}
}

const guardCodexConfig = "# keep\nmodel_provider = 'custom'\n[model_providers.custom]\nname = 'keep'\nbase_url = 'https://provider.example/v1'\nwire_api = 'responses'\n"
const guardClaudeConfig = "{\"env\":{\"ANTHROPIC_BASE_URL\":\"https://provider.example\"}}\n"

func newGuardService(t *testing.T) (context.Context, Service, *store.Store) {
	t.Helper()
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return ctx, Service{Store: database, Vault: &rejectingCredentialVault{}, StateRoot: state}, database
}

func TestProviderSwitchRefusesClientConfigUnderRealAccount(t *testing.T) {
	account := injectAccount(t, false)
	ctx, service, database := newGuardService(t)
	codex := filepath.Join(account, ".codex", "config.toml")
	claude := filepath.Join(account, ".claude", "settings.json")
	writeGuardFixture(t, codex, guardCodexConfig)
	writeGuardFixture(t, claude, guardClaudeConfig)

	requireClientConfigRefusal(t, service.Use(ctx, OfficialProviderName, ClientCodex, codex, ""))
	requireClientConfigRefusal(t, service.Use(ctx, OfficialProviderName, ClientClaude, claude, ""))
	requireDirectoryUnchanged(t, codex, guardCodexConfig)
	requireDirectoryUnchanged(t, claude, guardClaudeConfig)
	var operations int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM operations").Scan(&operations); err != nil || operations != 0 {
		t.Fatalf("operations = %d, %v; refusal must precede the backup and operation record", operations, err)
	}
	requireNoManagedBackup(t, service.StateRoot)

	// atomicPrivateReplace, the four Write*Config functions' single
	// convergence point, refuses on its own as well.
	for _, write := range []func() error{
		func() error {
			return WriteCodexConfig(codex, ClientConfig{Name: "x", Endpoint: "https://x.example", Credential: "synthetic"})
		},
		func() error { return WriteOfficialCodexConfig(codex) },
		func() error { return WriteCodexWrapperConfig(codex, "x", "https://x.example", false) },
		func() error { return WriteClaudeConfig(claude, ClientConfig{}) },
	} {
		requireClientConfigRefusal(t, write())
	}
	requireDirectoryUnchanged(t, codex, guardCodexConfig)
	requireDirectoryUnchanged(t, claude, guardClaudeConfig)
}

func TestProviderSwitchWritesTemporaryHomeAndInstalledAccount(t *testing.T) {
	for _, installed := range []bool{false, true} {
		account := injectAccount(t, installed)
		ctx, service, _ := newGuardService(t)
		home := t.TempDir()
		if installed {
			home = account
		}
		codex := filepath.Join(home, ".codex", "config.toml")
		writeGuardFixture(t, codex, guardCodexConfig)
		if err := service.Use(ctx, OfficialProviderName, ClientCodex, codex, ""); err != nil {
			t.Fatalf("installed=%v: %v", installed, err)
		}
		got, err := os.ReadFile(codex)
		if err != nil || string(got) == guardCodexConfig {
			t.Fatalf("installed=%v: config not written: %q, %v", installed, got, err)
		}
	}
}

// A caller-chosen backup destination is client-side and guarded on its own;
// the main configuration sits in an allowed temporary HOME so only the
// destination can trigger the refusal (ISO-CCG-R1-F1).
func TestProviderCustomBackupDestinationRefusedUnderRealAccount(t *testing.T) {
	const protected = "# protected configuration\nmodel = 'preserve-me'\n"
	for _, shape := range []string{"account", "parent-alias", "final-link"} {
		for _, mode := range []string{"direct", "service"} {
			t.Run(shape+"/"+mode, func(t *testing.T) {
				account := injectAccount(t, false)
				source := filepath.Join(t.TempDir(), ".codex", "config.toml")
				writeGuardFixture(t, source, guardCodexConfig)
				target := filepath.Join(account, ".codex", "config.toml")
				writeGuardFixture(t, target, protected)
				outside := t.TempDir()
				destination := target
				switch shape {
				case "parent-alias":
					if err := os.Symlink(filepath.Join(account, ".codex"), filepath.Join(outside, "alias")); err != nil {
						t.Fatal(err)
					}
					destination = filepath.Join(outside, "alias", "config.toml")
				case "final-link":
					destination = filepath.Join(outside, "backup.toml")
					if err := os.Symlink(target, destination); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "direct" {
					requireClientConfigRefusal(t, WriteRedactedBackup(ClientCodex, source, destination))
				} else {
					ctx, service, database := newGuardService(t)
					requireClientConfigRefusal(t, service.Use(ctx, OfficialProviderName, ClientCodex, source, destination))
					var operations int
					if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM operations").Scan(&operations); err != nil || operations != 0 {
						t.Fatalf("operations = %d, %v; refusal must precede the operation record", operations, err)
					}
					requireNoManagedBackup(t, service.StateRoot)
					if got, err := os.ReadFile(source); err != nil || string(got) != guardCodexConfig {
						t.Fatalf("allowed main config changed before refusal: %q, %v", got, err)
					}
				}
				requireDirectoryUnchanged(t, target, protected)
			})
		}
	}
}

func TestProviderCustomBackupDestinationAllowedOutsideAccountAndInstalled(t *testing.T) {
	for _, installed := range []bool{false, true} {
		for _, mode := range []string{"direct", "service"} {
			account := injectAccount(t, installed)
			source := filepath.Join(t.TempDir(), ".codex", "config.toml")
			writeGuardFixture(t, source, guardCodexConfig)
			destination := filepath.Join(t.TempDir(), "backup", "config.toml")
			if installed {
				destination = filepath.Join(account, "backup", "config.toml")
			}
			var err error
			if mode == "direct" {
				err = WriteRedactedBackup(ClientCodex, source, destination)
			} else {
				ctx, service, _ := newGuardService(t)
				err = service.Use(ctx, OfficialProviderName, ClientCodex, source, destination)
			}
			if err != nil {
				t.Fatalf("installed=%v %s: %v", installed, mode, err)
			}
			if got, err := os.ReadFile(destination); err != nil || len(got) == 0 {
				t.Fatalf("installed=%v %s: backup not written: %q, %v", installed, mode, got, err)
			}
		}
	}
}

// requireNoManagedBackup checks the managed backup location derived from
// managedBackupPath, so a refusal moved after the backup write is caught.
func requireNoManagedBackup(t *testing.T, stateRoot string) {
	t.Helper()
	for _, client := range []Client{ClientCodex, ClientClaude} {
		path, err := managedBackupPath(stateRoot, client, "probe")
		if err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err == nil && len(entries) != 0 {
			t.Fatalf("managed backup written before refusal: %s %v", filepath.Dir(path), entries)
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}
