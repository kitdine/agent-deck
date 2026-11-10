//go:build agentdeck_acceptance

package installation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	installHook = acceptanceInstallHook
	// Homebrew forwards only HOMEBREW_* variables to a cask's code.
	for _, name := range []string{"AGENTDECK_ACCEPTANCE_PLUGINKIT", "HOMEBREW_AGENTDECK_ACCEPTANCE_PLUGINKIT"} {
		if tool := os.Getenv(name); filepath.IsAbs(tool) {
			pluginkitTool = tool
			break
		}
	}
	prepareHelperCommand = func(command *exec.Cmd) {
		if AcceptanceHelperCommand != nil {
			AcceptanceHelperCommand(command)
		}
	}
}

// AcceptanceHelperCommand lets an acceptance test process adjust how a cleanup
// helper is started, because its helper is the test binary itself. Only
// acceptance builds compile this; production binaries start helpers as given.
var AcceptanceHelperCommand func(*exec.Cmd)

// acceptanceInstallHook reads AGENTDECK_ACCEPTANCE_INSTALL_HOOK, or its
// HOMEBREW_-prefixed twin when that one is unset, a comma-separated list of <point>=fail or <point>=wait:<directory>. A wait
// creates <directory>/reached while the exclusive lock is held and blocks
// until the test creates <directory>/release. Only acceptance builds compile
// this; production binaries have no hook.
//
//go:noinline
func acceptanceInstallHook(point string) error {
	hooks := os.Getenv("AGENTDECK_ACCEPTANCE_INSTALL_HOOK")
	if hooks == "" {
		// Homebrew forwards only HOMEBREW_* variables to a cask's code.
		hooks = os.Getenv("HOMEBREW_AGENTDECK_ACCEPTANCE_INSTALL_HOOK")
	}
	for _, entry := range strings.Split(hooks, ",") {
		name, action, ok := strings.Cut(entry, "=")
		if !ok || name != point {
			continue
		}
		if action == "fail" {
			return fmt.Errorf("injected failure at %s", point)
		}
		directory, ok := strings.CutPrefix(action, "wait:")
		if !ok || !filepath.IsAbs(directory) {
			return fmt.Errorf("invalid acceptance install hook %q", entry)
		}
		if err := os.WriteFile(filepath.Join(directory, "reached"), []byte(point+"\n"), 0o600); err != nil {
			return err
		}
		deadline := time.Now().Add(2 * time.Minute)
		for {
			if _, err := os.Stat(filepath.Join(directory, "release")); err == nil {
				return nil
			}
			if time.Now().After(deadline) {
				return errors.New("acceptance install hook was never released")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return nil
}
