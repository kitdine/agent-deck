package shellconfig

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
		platform.SetInstallationResolver(nil) // this package installs no resolver
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

func TestShellSetupAndRemoveRefuseRealAccountStartupFiles(t *testing.T) {
	account := injectAccount(t, false)
	for _, name := range []string{".zshrc", ".bashrc"} {
		if err := os.WriteFile(filepath.Join(account, name), []byte("# keep\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manager := New(Environment{Home: account, Invocation: Invocation{Shell: ShellZsh}})
	before := snapshotTree(t, account)
	_, err := manager.Setup(Request{})
	requireClientConfigRefusal(t, err)
	_, err = manager.SetupIfUnconfigured(Request{})
	requireClientConfigRefusal(t, err)
	_, err = manager.Remove(Request{})
	requireClientConfigRefusal(t, err)
	// An explicit rc outside the account still refuses when it links inside.
	outside := physicalTempDir(t)
	link := filepath.Join(outside, ".zshrc")
	if err := os.Symlink(filepath.Join(account, ".zshrc"), link); err != nil {
		t.Fatal(err)
	}
	_, err = New(Environment{Home: outside, Invocation: Invocation{Shell: ShellZsh}}).Setup(Request{Shell: ShellZsh, RC: link})
	requireClientConfigRefusal(t, err)
	requireSameTree(t, account, before)
}

func TestShellSetupAndRemoveWriteTemporaryHomeAndInstalledAccount(t *testing.T) {
	for _, installed := range []bool{false, true} {
		account := injectAccount(t, installed)
		home := physicalTempDir(t)
		if installed {
			home = account
		}
		manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
		summary, err := manager.Setup(Request{Shell: ShellZsh})
		if err != nil || summary.HasFailures() {
			t.Fatalf("installed=%v setup = %#v, %v", installed, summary, err)
		}
		summary, err = manager.Remove(Request{Shell: ShellZsh})
		if err != nil || summary.HasFailures() {
			t.Fatalf("installed=%v remove = %#v, %v", installed, summary, err)
		}
	}
}
