package quota

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// claudeHomeInstallDirs lists the fixed, home-relative directories where
// Claude Code's own installers place the `claude` executable. App-launched
// helpers get a fixed PATH without these (they are not Homebrew or system
// roots), so a bare "claude" is never found there even though it resolves in
// an interactive shell. The list is closed: no shell profile is read, no
// environment variable names a directory, and nothing is stored.
var claudeHomeInstallDirs = []string{
	filepath.Join(".local", "bin"),    // native installer
	filepath.Join(".claude", "local"), // legacy local installer
}

// resolveClaudeExecutable maps the configured command name to the path to
// execute.
//
//  1. A name containing a path separator is an explicit override and is used
//     verbatim: a missing or unexecutable explicit path is a spawn failure,
//     never silently replaced by a different binary.
//  2. A bare name is looked up on PATH first, so an executable on the
//     caller's PATH keeps precedence.
//  3. Only a genuine not-found (exec.ErrNotFound) falls through to the fixed
//     home install directories. Any other lookup error — notably exec.ErrDot,
//     where the command resolves only relative to the working directory — is
//     returned unchanged, so a different executable is never substituted for
//     one the lookup refused.
//
// A home candidate is accepted only if every component the kernel would
// traverse to reach it is trusted (see resolveTrustedClaudeCandidate), and the
// symlink-free path that verification reached is what gets executed. The
// original lookup error is returned when nothing qualifies.
func resolveClaudeExecutable(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	path, lookErr := exec.LookPath(name)
	if lookErr == nil {
		return path, nil
	}
	if !errors.Is(lookErr, exec.ErrNotFound) {
		return "", lookErr
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		for _, dir := range claudeHomeInstallDirs {
			// Concatenated, not filepath.Join'd: Join would lexically clean
			// a ".." in $HOME that the kernel resolves after following a
			// symlink, which is exactly what "$HOME/.local/bin/claude" means
			// to a shell. The walk below applies the kernel's order.
			candidate := home + string(filepath.Separator) + dir + string(filepath.Separator) + name
			if real, ok := resolveTrustedClaudeCandidate(candidate); ok {
				return real, nil
			}
		}
	}
	return "", lookErr
}

// maxClaudeSymlinkHops bounds the total number of symlinks the walk follows
// for one candidate, counting directory links (such as macOS's /var alias)
// as well as the native installer's link to its versioned binary.
const maxClaudeSymlinkHops = 8

// resolveTrustedClaudeCandidate resolves candidate component by component in
// the order the kernel does, verifying each step before taking it, and
// returns the resulting symlink-free path when every step is trusted.
//
// Nothing is cleaned lexically ahead of time. A symlink's target is spliced
// in front of the components still to be walked; an absolute target restarts
// the walk at "/"; ".." moves to the parent of the directory actually reached,
// so "alias/.." means "the parent of wherever alias really points". The
// directory being walked is therefore always a real, symlink-free path that
// was reached through trusted directories only.
//
// Each lookup of a name inside a directory requires that directory to be
// trusted for that entry (trustedDirectory); this covers every ancestor of the
// entry, of each symlink, and of the final target. The final component must be
// a regular executable owned by root or the current user and not group/other
// writable. Returning the walked path, rather than the candidate, means the
// object executed is the object verified: no later resolution reinterprets it.
//
// Mode bits are not the whole story where ACLs exist, so "/" and every object
// the walk looks up — each directory, each symlink itself, and the final
// file — must also pass claudeACLGrantsNoMutation. Every directory the walk
// stands in was one of those objects (or "/"), so no traversed object escapes
// the check. This also keeps an ACL from undoing the sticky exception, since a
// delete grant on the entry or a delete_child grant on the sticky directory
// would authorize removal regardless of the sticky bit.
func resolveTrustedClaudeCandidate(candidate string) (string, bool) {
	if !filepath.IsAbs(candidate) {
		return "", false
	}
	sep := string(filepath.Separator)
	if !claudeACLGrantsNoMutation(sep) {
		return "", false
	}
	current := sep
	pending := splitPathComponents(candidate)
	hops := 0
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		switch name {
		case ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		next := filepath.Join(current, name)
		if !trustedDirectory(current, next) {
			return "", false
		}
		info, err := os.Lstat(next)
		if err != nil || !claudeACLGrantsNoMutation(next) {
			return "", false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			hops++
			if hops > maxClaudeSymlinkHops {
				return "", false
			}
			target, err := os.Readlink(next)
			if err != nil || target == "" {
				return "", false
			}
			if filepath.IsAbs(target) {
				current = sep
			}
			pending = append(splitPathComponents(target), pending...)
			continue
		}
		if len(pending) > 0 {
			if !info.IsDir() {
				return "", false
			}
			current = next
			continue
		}
		if !trustedClaudeLeaf(info) {
			return "", false
		}
		return next, true
	}
	// The path ended on "." or ".." (a directory), never an executable.
	return "", false
}

// splitPathComponents splits path into its non-empty components without
// interpreting "." or "..", which the walk handles in kernel order.
func splitPathComponents(path string) []string {
	var components []string
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if component != "" {
			components = append(components, component)
		}
	}
	return components
}

func trustedClaudeLeaf(info os.FileInfo) bool {
	mode := info.Mode()
	return mode.IsRegular() && mode.Perm()&0o111 != 0 && mode.Perm()&0o022 == 0 && ownedByRootOrCurrentUser(info)
}

// trustedDirectory reports whether looking up child inside dir is safe from
// other users. dir must be a real directory owned by root or the current user
// and either not group/other writable, or sticky (the /tmp boundary) with
// child itself owned by root or the current user — a sticky directory stops
// other users from renaming or removing entries they do not own. A writable
// directory without that protection is never trusted, whatever it contains.
func trustedDirectory(dir, child string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || !ownedByRootOrCurrentUser(info) {
		return false
	}
	if info.Mode().Perm()&0o022 == 0 {
		return true
	}
	if info.Mode()&os.ModeSticky == 0 {
		return false
	}
	childInfo, err := os.Lstat(child)
	return err == nil && ownedByRootOrCurrentUser(childInfo)
}

var errClaudeEmptyCommand = errors.New("claude command is empty")

func claudeCommandPath(args []string) (string, error) {
	if len(args) == 0 {
		return "", errClaudeEmptyCommand
	}
	path, err := resolveClaudeExecutable(args[0])
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", args[0], err)
	}
	return path, nil
}
