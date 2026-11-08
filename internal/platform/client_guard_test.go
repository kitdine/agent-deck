package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func requireClientRefusal(t *testing.T, err error, reason string) {
	t.Helper()
	var refused *ErrStateOwnershipRefused
	if !errors.As(err, &refused) || refused.Resource != "client_config" || refused.Reason != reason {
		t.Fatalf("refusal = %#v, %v; want client_config/%s", refused, err, reason)
	}
	if refused.Error() != clientConfigRefusedMessage {
		t.Fatalf("message = %q", refused.Error())
	}
}

func TestGuardClientWriteRefusesTargetsUnderTheRealAccount(t *testing.T) {
	r, _ := ownershipFixture(t)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(r.account, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A final symlink outside the account whose target is inside it, and an
	// account-home link entrance, are both judged on the physical location.
	inside := filepath.Join(r.account, ".codex", "config.toml")
	if err := os.Symlink(inside, filepath.Join(outside, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.account, filepath.Join(outside, "home-link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		inside,
		filepath.Join(r.account, ".zshrc"),
		filepath.Join(r.account, "missing", "dir", "config.fish"),
		filepath.Join(outside, "config.toml"),
		filepath.Join(outside, "home-link", ".claude", "settings.json"),
		r.account,
	} {
		_, err := GuardClientWrite(path)
		requireClientRefusal(t, err, "receipt_other_executable")
	}
	if entries, err := os.ReadDir(filepath.Join(r.account, ".codex")); err != nil || len(entries) != 0 {
		t.Fatalf("guard created entries: %v %v", entries, err)
	}
	if _, err := os.Lstat(filepath.Join(r.account, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("guard created a directory: %v", err)
	}
}

func TestGuardClientWriteAllowsTemporaryHomeAndReturnsPhysicalParent(t *testing.T) {
	ownershipFixture(t)
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logical := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(physical, logical); err != nil {
		t.Fatal(err)
	}
	got, err := GuardClientWrite(filepath.Join(logical, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(physical, ".codex", "config.toml"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	// The final entry itself is not followed: writers replace the entry.
	link := filepath.Join(physical, ".zshrc")
	if err := os.Symlink(filepath.Join(physical, "target"), link); err != nil {
		t.Fatal(err)
	}
	if got, err := GuardClientWrite(link); err != nil || got != link {
		t.Fatalf("link path = %q, %v", got, err)
	}
}

func TestGuardClientWriteAllowsInstalledIdentity(t *testing.T) {
	r, _ := ownershipFixture(t)
	r.installed.Store(true)
	path := filepath.Join(r.account, ".claude", "settings.json")
	if got, err := GuardClientWrite(path); err != nil || got != path {
		t.Fatalf("installed path = %q, %v", got, err)
	}
}

func TestGuardClientWriteFailsClosedWithoutAccountHome(t *testing.T) {
	ownershipFixture(t)
	statePaths.accountHome = func() (string, error) { return "relative", nil }
	_, err := GuardClientWrite(filepath.Join(t.TempDir(), "config.toml"))
	requireClientRefusal(t, err, "real_root_unresolved")
}
