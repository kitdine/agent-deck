package platform

import (
	"errors"
	"path/filepath"
)

const clientConfigRefusedMessage = "This AgentDeck is not the installed program and cannot change client configuration in your home directory."

func clientRefused(reason string) error {
	return &ErrStateOwnershipRefused{Resource: "client_config", Reason: reason}
}

// protectedAccountPaths returns phys(account): the logical account home, each
// link entrance on the way and the physical target. It is recomputed per call
// because a moved home must not leave a stale grant; only entrance metadata is
// read, never directory contents.
func protectedAccountPaths() ([]string, error) {
	home, err := statePaths.accountHome()
	if err != nil || !filepath.IsAbs(home) {
		return nil, errors.New("real_root_unresolved")
	}
	// Resolve ancestors first so onLink sees only the home entrance's own
	// chain, never a system link such as /var that merely precedes it.
	parent, err := statePaths.walk(filepath.Dir(home), nil, nil)
	if err != nil {
		return nil, errors.New("real_root_unresolved")
	}
	entrance := filepath.Join(parent, filepath.Base(home))
	protected := []string{filepath.Clean(home)}
	if !slicesContain(protected, entrance) {
		protected = append(protected, entrance)
	}
	physical, err := statePaths.walk(entrance, nil, func(link string) {
		if !slicesContain(protected, link) {
			protected = append(protected, link)
		}
	})
	if err != nil {
		return nil, errors.New("real_root_unresolved")
	}
	if !slicesContain(protected, physical) {
		protected = append(protected, physical)
	}
	return protected, nil
}

// GuardClientWrite returns the path a client or shell configuration writer
// must use. An uninstalled process may not write anywhere under the real
// account home, judged on the fully resolved target (a final symlink that
// points into the account is refused too); a temporary HOME elsewhere is
// allowed. Callers check before creating directories, temporary files,
// backups or removals.
//
// The returned path is the physical parent directory joined with the final
// name. Every writer replaces or removes the directory entry itself (and the
// shell and Hook writers refuse a symlinked entry), so resolving the parent is
// what pins the write to the location that was checked without turning a
// replace-the-link write into a write through the link.
func GuardClientWrite(path string) (string, error) {
	protected, err := protectedAccountPaths()
	if err != nil {
		return "", clientRefused("real_root_unresolved")
	}
	_, err = statePaths.walk(path, protected, nil)
	if err != nil && !errors.Is(err, ErrProtectedStatePath) {
		return "", err
	}
	if err != nil {
		identity, resolveErr := ResolveInstallationForState()
		if resolveErr != nil {
			var refused *ErrStateOwnershipRefused
			if errors.As(resolveErr, &refused) {
				return "", clientRefused(refused.Reason)
			}
			return "", clientRefused("receipt_invalid")
		}
		if !identity.Installed {
			return "", clientRefused(identity.Reason)
		}
	}
	parent, err := statePaths.walk(filepath.Dir(path), nil, nil)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}
