//go:build agentdeck_acceptance

package installation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestAcceptanceResolverIsolatesIdentityAndPlatform(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", home)
	platform.SetInstallationResolver(DefaultResolver())
	t.Cleanup(func() { platform.SetInstallationResolver(nil) })
	if got, err := platform.AccountHome(); err != nil || got != home {
		t.Fatalf("platform account=%s err=%v", got, err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		t.Fatal(err)
	}
	register(t, DefaultResolver(), self, "make")
	if got := platform.CurrentInstallation(); !got.Installed || got.Channel != "make" {
		t.Fatalf("identity=%+v", got)
	}
	if _, err := os.Stat(receiptPath(home)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", "")
	if got := platform.CurrentInstallation(); got.Installed || got.Reason != "real_root_unresolved" {
		t.Fatalf("missing acceptance home=%+v", got)
	}
}
