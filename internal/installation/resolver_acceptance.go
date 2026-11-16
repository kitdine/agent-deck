//go:build agentdeck_acceptance

package installation

import (
	"errors"
	"os"
	"path/filepath"
)

func init() { processResolver.accountHome = acceptanceAccountHome }

//go:noinline
func acceptanceAccountHome() (string, error) {
	home := os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME")
	if home == "" {
		// Homebrew forwards only HOMEBREW_* variables to a cask's code.
		home = os.Getenv("HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME")
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("acceptance account home must be absolute")
	}
	return filepath.EvalSymlinks(home)
}
