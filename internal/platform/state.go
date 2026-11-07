// Package platform owns portable operating-system boundaries.
package platform

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// InstallationIdentity describes an observation, never a durable access grant.
type InstallationIdentity struct {
	Installed bool
	Channel   string
	Reason    string
}

// InstallationResolver is injected by the executable; libraries fail closed
// when no resolver is installed. AccountHome never uses the HOME environment.
type InstallationResolver interface {
	Resolve() InstallationIdentity
	AccountHome() (string, error)
}

var installationResolver struct {
	sync.RWMutex
	value InstallationResolver
}

func SetInstallationResolver(resolver InstallationResolver) {
	installationResolver.Lock()
	defer installationResolver.Unlock()
	installationResolver.value = resolver
}

func CurrentInstallation() InstallationIdentity {
	installationResolver.RLock()
	resolver := installationResolver.value
	installationResolver.RUnlock()
	if resolver == nil {
		return InstallationIdentity{Reason: "receipt_missing"}
	}
	return resolver.Resolve()
}

func AccountHome() (string, error) {
	installationResolver.RLock()
	resolver := installationResolver.value
	installationResolver.RUnlock()
	if resolver != nil {
		return resolver.AccountHome()
	}
	return RealAccountHome()
}

// RealAccountHome reads the operating system account database. On macOS the
// standard library uses getpwuid_r, including builds with CGO_ENABLED=0.
func RealAccountHome() (string, error) {
	account, err := user.Current()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(account.HomeDir) {
		return "", errors.New("real_root_unresolved")
	}
	return account.HomeDir, nil
}

var ErrProtectedStatePath = errors.New("protected state path")

type pathResolver struct {
	accountHome            func() (string, error)
	lstat                  func(string) (fs.FileInfo, error)
	readlink               func(string) (string, error)
	caseSensitive          func(string) (bool, error)
	normalizationSensitive func(string) (bool, error)
	once                   sync.Once
	protected              []string
	err                    error
}

var statePaths = pathResolver{accountHome: AccountHome, lstat: os.Lstat, readlink: os.Readlink}

// ProtectedStatePaths includes the logical entrance, each link entrance and
// the physical target. Discovery only reads entrance metadata, never contents.
func ProtectedStatePaths() ([]string, error) { return statePaths.protectedPaths() }

// ResolveStatePath returns the path callers must actually use. Known entrances
// are refused before lstat; aliases may require entrance metadata, but never
// descendant metadata. Installation identity belongs to the ownership session.
func ResolveStatePath(path string) (string, error) {
	protected, err := ProtectedStatePaths()
	if err != nil {
		return "", err
	}
	return statePaths.walk(path, protected, nil)
}

func (r *pathResolver) protectedPaths() ([]string, error) {
	r.once.Do(func() {
		home, err := r.accountHome()
		if err != nil || !filepath.IsAbs(home) {
			r.err = errors.New("real_root_unresolved")
			return
		}
		home, err = r.walk(home, nil, nil)
		if err != nil {
			r.err = errors.New("real_root_unresolved")
			return
		}
		entrance := filepath.Join(home, ".agentdeck")
		r.protected = append(r.protected, entrance)
		physical, err := r.walk(entrance, nil, func(link string) {
			// Ancestor links have already been resolved with the same walk.
			if !slicesContain(r.protected, link) {
				r.protected = append(r.protected, link)
			}
		})
		if err != nil {
			r.err = errors.New("real_root_unresolved")
			return
		}
		if !slicesContain(r.protected, physical) {
			r.protected = append(r.protected, physical)
		}
	})
	return append([]string(nil), r.protected...), r.err
}

type pathIdentity struct {
	metadata               map[string]fs.FileInfo
	caseSensitive          func(string) (bool, error)
	normalizationSensitive func(string) (bool, error)
}

func (r *pathResolver) pathIdentity(protected []string) (*pathIdentity, error) {
	identity := &pathIdentity{metadata: make(map[string]fs.FileInfo), caseSensitive: r.caseSensitive, normalizationSensitive: r.normalizationSensitive}
	// The protected paths are stable, but object identities are observations
	// for this walk only. Replacing a directory must not leave a stale inode
	// that permits its case alias on the next call. Read entrance/ancestor
	// metadata only; never enumerate or inspect a protected directory's contents.
	for _, path := range protected {
		for {
			if _, known := identity.metadata[path]; known {
				break
			}
			info, err := r.lstat(path)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			identity.metadata[path] = info
			parent := filepath.Dir(path)
			if parent == path {
				break
			}
			path = parent
		}
	}
	return identity, nil
}

func slicesContain(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

func protectedPath(path string, protected []string) bool {
	for _, p := range protected {
		if path == p || strings.HasPrefix(path, strings.TrimSuffix(p, string(os.PathSeparator))+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// protectedAlias compares OS identities before the walk can enter another
// component. Canonicalizing known ancestors makes subsequent lexical checks
// reliable even when the account or a mount-point name has alternate spelling.
func (r *pathIdentity) protectedAlias(path string, info fs.FileInfo, protected []string) (string, error) {
	for _, p := range protected {
		if original := r.metadata[p]; original != nil && os.SameFile(info, original) {
			return "", ErrProtectedStatePath
		}
	}
	if info.IsDir() {
		for _, p := range protected {
			for ancestor := filepath.Dir(p); ; ancestor = filepath.Dir(ancestor) {
				if original := r.metadata[ancestor]; original != nil && os.SameFile(info, original) {
					return ancestor, nil
				}
				if filepath.Dir(ancestor) == ancestor {
					break
				}
			}
		}
	}
	return path, nil
}

// A missing entrance has no inode to compare. Its nearest existing ancestor
// has already been canonicalized by the walk; query that directory's filesystem
// policy for ambiguous case or canonical Unicode variants. Neither case nor
// normalization sensitivity implies the other; unknown policy fails closed.
func (r *pathIdentity) protectedMissingAlias(path string, protected []string) (bool, error) {
	for _, p := range protected {
		if r.metadata[p] != nil {
			continue
		}
		ancestor := filepath.Dir(p)
		for r.metadata[ancestor] == nil && filepath.Dir(ancestor) != ancestor {
			ancestor = filepath.Dir(ancestor)
		}
		prefix := strings.TrimSuffix(ancestor, string(os.PathSeparator)) + string(os.PathSeparator)
		if !strings.HasPrefix(path, prefix) || !strings.HasPrefix(p, prefix) {
			continue
		}
		candidate := strings.Split(strings.TrimPrefix(path, prefix), string(os.PathSeparator))
		entrance := strings.Split(strings.TrimPrefix(p, prefix), string(os.PathSeparator))
		if len(candidate) < len(entrance) {
			continue
		}
		matches := true
		for i, component := range entrance {
			if !strings.EqualFold(component, candidate[i]) && !strings.EqualFold(canonicalNameKey(component), canonicalNameKey(candidate[i])) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		for i, component := range entrance {
			equal, err := r.equivalentMissingName(ancestor, component, candidate[i])
			if err != nil {
				return false, err
			}
			if !equal {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *pathIdentity) equivalentMissingName(directory, a, b string) (bool, error) {
	if a == b {
		return true, nil
	}
	// These are comparison keys only. Never normalize the path returned to a
	// caller, or normalize unresolved path components before symlink traversal.
	normalA, normalB := canonicalNameKey(a), canonicalNameKey(b)
	caseAlias := strings.EqualFold(a, b)
	if !caseAlias && !strings.EqualFold(normalA, normalB) {
		return false, nil
	}
	if caseAlias || normalA != normalB {
		lookup := r.caseSensitive
		if lookup == nil {
			lookup = filesystemCaseSensitive
		}
		sensitive, err := lookup(directory)
		if err != nil {
			return false, err
		}
		if caseAlias && !sensitive {
			return true, nil
		}
		if sensitive && normalA != normalB {
			return false, nil
		}
	}
	// Any remaining possible match requires canonical normalization. In
	// particular, a case-sensitive APFS volume can still equate NFC and NFD.
	lookup := r.normalizationSensitive
	if lookup == nil {
		lookup = filesystemNormalizationSensitive
	}
	sensitive, err := lookup(directory)
	if err != nil {
		return false, err
	}
	return !sensitive, nil
}

// canonicalNameKey performs canonical decomposition and ordering without the
// Stream-Safe insertions made by normalizing a whole name with norm.NFD.String.
// A single rune's decomposition cannot accumulate the 30-non-starter limit;
// ordering the resulting runs ourselves preserves every literal U+034F as a
// starter/boundary rather than confusing it with an inserted grapheme joiner.
func canonicalNameKey(name string) string {
	if norm.NFD.IsNormalString(name) {
		return name
	}
	type mark struct {
		value rune
		class uint8
	}
	var result strings.Builder
	result.Grow(len(name))
	var marks []mark
	flush := func() {
		slices.SortStableFunc(marks, func(a, b mark) int { return int(a.class) - int(b.class) })
		for _, m := range marks {
			result.WriteRune(m.value)
		}
		marks = marks[:0]
	}
	for offset := 0; offset < len(name); {
		value, size := utf8.DecodeRuneInString(name[offset:])
		if value == utf8.RuneError && size == 1 {
			// Keep malformed filename bytes intact, including their boundaries.
			flush()
			result.WriteByte(name[offset])
			offset++
			continue
		}
		for _, decomposed := range norm.NFD.String(name[offset : offset+size]) {
			class := norm.NFD.PropertiesString(string(decomposed)).CCC()
			if class == 0 {
				flush()
				result.WriteRune(decomposed)
			} else {
				marks = append(marks, mark{decomposed, class})
			}
		}
		offset += size
	}
	flush()
	return result.String()
}

// walk deliberately does not use Abs, Join or Clean on unresolved input:
// link/.. is the parent of the link's target, not the link's lexical parent.
func (r *pathResolver) walk(path string, protected []string, onLink func(string)) (string, error) {
	identity, err := r.pathIdentity(protected)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = cwd + string(os.PathSeparator) + path
	}
	parts := strings.Split(path, string(os.PathSeparator))
	cur, links := string(os.PathSeparator), 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, part)
		if protectedPath(next, protected) {
			return "", ErrProtectedStatePath
		}
		if match, err := identity.protectedMissingAlias(next, protected); err != nil {
			return "", err
		} else if match {
			return "", ErrProtectedStatePath
		}
		info, err := r.lstat(next)
		if errors.Is(err, fs.ErrNotExist) {
			for _, remaining := range parts {
				if remaining == ".." {
					return "", errors.New("state_path_unresolved: parent after missing component")
				}
			}
			result := filepath.Join(append([]string{next}, parts...)...)
			if protectedPath(result, protected) {
				return "", ErrProtectedStatePath
			}
			if match, err := identity.protectedMissingAlias(result, protected); err != nil {
				return "", err
			} else if match {
				return "", ErrProtectedStatePath
			}
			return result, nil
		}
		if err != nil {
			return "", err
		}
		next, err = identity.protectedAlias(next, info, protected)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > 40 {
				return "", errors.New("state_path_unresolved: symlink loop")
			}
			if onLink != nil {
				onLink(next)
			}
			target, err := r.readlink(next)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = cur + string(os.PathSeparator) + target
			}
			parts = append(strings.Split(target, string(os.PathSeparator)), parts...)
			cur = string(os.PathSeparator)
			continue
		}
		if len(parts) > 0 && !info.IsDir() {
			return "", errors.New("state_path_unresolved: non-directory component")
		}
		cur = next
	}
	if protectedPath(cur, protected) {
		return "", ErrProtectedStatePath
	}
	return cur, nil
}

// Clock keeps time-dependent domain behavior testable without tying it to the
// host clock.
type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// FileSystem is the portable subset needed by state and future atomic-write
// adapters. Production code uses OSFileSystem; tests can supply a fake.
type FileSystem interface {
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	OpenFile(string, int, fs.FileMode) (*os.File, error)
	Remove(string) error
	Stat(string) (fs.FileInfo, error)
}

type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFileSystem) Chmod(path string, mode fs.FileMode) error    { return os.Chmod(path, mode) }
func (OSFileSystem) OpenFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm)
}
func (OSFileSystem) Remove(path string) error              { return os.Remove(path) }
func (OSFileSystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

const (
	DirectoryMode fs.FileMode = 0o700
	FileMode      fs.FileMode = 0o600
)

// StateRoot returns the explicit test or CLI override, or the user's default
// AgentDeck state directory. It does not create or modify the path.
func StateRoot(override, home string) string {
	if override != "" {
		return override
	}
	return filepath.Join(home, ".agentdeck")
}

// EnsureStateRoot creates a private state directory without changing client
// configuration or inspecting legacy state.
func EnsureStateRoot(path string) error {
	return EnsureStateRootWithFS(OSFileSystem{}, path)
}

func EnsureStateRootWithFS(filesystem FileSystem, path string) error {
	if err := filesystem.MkdirAll(path, DirectoryMode); err != nil {
		return err
	}
	return filesystem.Chmod(path, DirectoryMode)
}
