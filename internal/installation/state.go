package installation

import (
	"os"

	"github.com/kitdine/agent-deck/internal/platform"
)

// StateRoot selects the installed or isolated default and guards every result.
// HOME selects the default only; it never supplies the ownership account.
func StateRoot(override string) (string, error) {
	if override != "" {
		return platform.GuardStateRoot(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return StateRootWithHome(override, home)
}

// StateRootWithHome preserves callers' existing HOME lookup seams. The account
// and receipt still come exclusively from the injected installation resolver.
func StateRootWithHome(override, home string) (string, error) {
	if override != "" {
		return platform.GuardStateRoot(override)
	}
	identity, err := platform.ResolveInstallationForState()
	if err != nil {
		return "", err
	}
	name := ".agentdeck-dev"
	if identity.Installed {
		name = ".agentdeck"
	}
	// HOME may itself contain link/..; joining lexically before the guard
	// would change its physical meaning just like cleaning an explicit root.
	root := name
	if home != "" {
		root = home + string(os.PathSeparator) + name
	}
	return platform.GuardStateRoot(root)
}
