package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
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

// snapshotTree records every path and its bytes so a refusal can be shown to
// leave the account byte-for-byte unchanged, including no temporary files.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		contents := ""
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			contents = string(data)
		}
		files[path] = contents
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func requireSameTree(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotTree(t, root)
	if len(after) != len(before) {
		t.Fatalf("tree changed: before %v after %v", before, after)
	}
	for path, contents := range before {
		if after[path] != contents {
			t.Fatalf("%s changed: %q -> %q", path, contents, after[path])
		}
	}
}

func TestCodexManagedBlockRefusesRealAccountConfig(t *testing.T) {
	account := injectAccount(t, false)
	state := t.TempDir()
	writeCodexConfig(t, account, userConfig, 0o600)
	before := snapshotTree(t, account)
	requireClientConfigRefusal(t, WriteManagedBlock(account, state, 4318, "synthetic-token", time.Now()))
	if _, err := os.Stat(managedRecordPath(state)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("digest record written before refusal: %v", err)
	}
	requireSameTree(t, account, before)

	// A block written by the installed program cannot be removed by an
	// uninstalled one: the record lives in its state, the file in the account.
	platform.SetInstallationResolver(clientGuardResolver{account: account, installed: true})
	if err := WriteManagedBlock(account, state, 4318, "synthetic-token", time.Now()); err != nil {
		t.Fatal(err)
	}
	platform.SetInstallationResolver(clientGuardResolver{account: account})
	before = snapshotTree(t, account)
	removed, err := RemoveManagedBlock(account, state)
	requireClientConfigRefusal(t, err)
	if removed {
		t.Fatal("refused removal reported removed")
	}
	if _, err := os.Stat(managedRecordPath(state)); err != nil {
		t.Fatalf("digest record lost on refusal: %v", err)
	}
	requireSameTree(t, account, before)
}

func TestCodexManagedBlockWritesTemporaryHomeAndInstalledAccount(t *testing.T) {
	for _, installed := range []bool{false, true} {
		account := injectAccount(t, installed)
		home := t.TempDir()
		if installed {
			home = account
		}
		state := t.TempDir()
		writeCodexConfig(t, home, userConfig, 0o600)
		if err := WriteManagedBlock(home, state, 4318, "synthetic-token", time.Now()); err != nil {
			t.Fatalf("installed=%v write: %v", installed, err)
		}
		if removed, err := RemoveManagedBlock(home, state); err != nil || !removed {
			t.Fatalf("installed=%v remove = %v, %v", installed, removed, err)
		}
		got, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		if err != nil || string(got) != userConfig {
			t.Fatalf("installed=%v config = %q, %v", installed, got, err)
		}
	}
}
