//go:build !agentdeck_acceptance

package installation

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestProductionResolverIgnoresAccountEnvironment(t *testing.T) {
	if os.Getenv("AGENTDECK_PRODUCTION_RESOLVER_PROBE") == "1" {
		got, err := DefaultResolver().AccountHome()
		if err != nil || got != os.Getenv("AGENTDECK_EXPECTED_ACCOUNT") {
			t.Fatalf("production account=%q err=%v", got, err)
		}
		return
	}
	want, err := platform.RealAccountHome()
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestProductionResolverIgnoresAccountEnvironment$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "USER=") && !strings.HasPrefix(entry, "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "HOME="+fake, "USER=fake-account", "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="+fake, "AGENTDECK_PRODUCTION_RESOLVER_PROBE=1", "AGENTDECK_EXPECTED_ACCOUNT="+want)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production resolver probe: %s: %v", out, err)
	}
}
