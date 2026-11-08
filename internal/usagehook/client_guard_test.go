package usagehook

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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

func seedClientConfigs(t *testing.T, home string) {
	t.Helper()
	for path, contents := range map[string]string{
		filepath.Join(home, ".claude", "settings.json"): "{\"statusLine\":{\"type\":\"command\",\"command\":\"prior\"}}\n",
		filepath.Join(home, ".codex", "hooks.json"):     "{}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUsageHookAndStatusLineRefuseRealAccountConfig(t *testing.T) {
	account := injectAccount(t, false)
	seedClientConfigs(t, account)
	manager := New(Environment{Home: account, StateDir: t.TempDir()})
	before := snapshotTree(t, account)

	_, err := manager.Setup(Request{Client: ClientAll})
	requireClientConfigRefusal(t, err)
	_, err = manager.Remove(Request{Client: ClientAll})
	requireClientConfigRefusal(t, err)
	_, err = manager.SetupStatusLine()
	requireClientConfigRefusal(t, err)
	_, err = manager.RestoreStatusLine()
	requireClientConfigRefusal(t, err)
	_, err = manager.RollbackStatusLineSetup()
	requireClientConfigRefusal(t, err)
	manager.statusLineRestore = &statusLineRestoreReceipt{}
	_, err = manager.ReinstallRestoredStatusLine()
	requireClientConfigRefusal(t, err)
	// The package's write point and removal refuse on their own as well.
	requireClientConfigRefusal(t, manager.writeAtomic(filepath.Join(account, ".codex", "hooks.json"), []byte("x"), 0o600))
	requireClientConfigRefusal(t, manager.removePath(filepath.Join(account, ".codex", "hooks.json")))
	requireSameTree(t, account, before)
}

func TestUsageHookAndStatusLineWriteTemporaryHomeAndInstalledAccount(t *testing.T) {
	for _, installed := range []bool{false, true} {
		account := injectAccount(t, installed)
		home := t.TempDir()
		if installed {
			home = account
		}
		seedClientConfigs(t, home)
		manager := New(Environment{Home: home, StateDir: t.TempDir()})
		summary, err := manager.Setup(Request{Client: ClientAll})
		if err != nil || summary.HasFailures() {
			t.Fatalf("installed=%v setup = %#v, %v", installed, summary, err)
		}
		if _, err := manager.SetupStatusLine(); err != nil {
			t.Fatalf("installed=%v statusline setup: %v", installed, err)
		}
		if result, err := manager.RestoreStatusLine(); err != nil || result.Outcome == OutcomeFailed {
			t.Fatalf("installed=%v statusline restore = %#v, %v", installed, result, err)
		}
		if summary, err := manager.Remove(Request{Client: ClientAll}); err != nil || summary.HasFailures() {
			t.Fatalf("installed=%v remove = %#v, %v", installed, summary, err)
		}
	}
}
