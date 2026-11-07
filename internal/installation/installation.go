// Package installation resolves the current process against the account's
// single installation receipt. It never infers ownership from build metadata.
package installation

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

type Identity = platform.InstallationIdentity

// Resolver's zero value uses only production OS boundaries. Private seams are
// replaced by package tests, or by the separately tagged acceptance adapter.
type Resolver struct {
	executable  func() (string, error)
	accountHome func() (string, error)
	now         func() time.Time
	acquireLock func(context.Context, string, bool) (*platform.OwnershipLock, error)
}

var processResolver Resolver

func DefaultResolver() *Resolver { return &processResolver }
func Resolve() Identity          { return processResolver.Resolve() }

func (r *Resolver) AccountHome() (string, error) {
	lookup := r.accountHome
	if lookup == nil {
		lookup = platform.RealAccountHome
	}
	home, err := lookup()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("real_root_unresolved")
	}
	return home, nil
}

func (r *Resolver) self() (string, error) {
	executable := r.executable
	if executable == nil {
		executable = os.Executable
	}
	path, err := executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func (r *Resolver) Resolve() Identity {
	self, err := r.self()
	if err != nil {
		return Identity{Reason: "executable_unresolved"}
	}
	home, err := r.AccountHome()
	if err != nil {
		return Identity{Reason: "real_root_unresolved"}
	}
	receipt, err := readReceipt(home)
	if errors.Is(err, fs.ErrNotExist) {
		return Identity{Reason: "receipt_missing"}
	}
	if err != nil {
		return Identity{Reason: "receipt_invalid"}
	}
	// Never resolve links in the recorded path. In particular, switching opt
	// cannot transfer ownership while the previous keg still has live sessions.
	_, err = os.Lstat(receipt.Executable)
	if errors.Is(err, fs.ErrNotExist) {
		return Identity{Reason: "receipt_missing"}
	}
	if err != nil {
		return Identity{Reason: "receipt_invalid"}
	}
	if receipt.Executable != self {
		return Identity{Reason: "receipt_other_executable"}
	}
	return Identity{Installed: true, Channel: receipt.Channel}
}

type formulaKeg struct{ prefix, formula, version, root string }

func parseFormulaKeg(path string) (formulaKeg, bool) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return formulaKeg{}, false
	}
	parts := strings.Split(path, string(os.PathSeparator))
	n := len(parts)
	if n < 6 || parts[n-5] != "Cellar" || parts[n-2] != "bin" || parts[n-1] != "agentdeck" {
		return formulaKeg{}, false
	}
	if parts[n-4] != "agentdeck" && parts[n-4] != "agentdeck-rc" {
		return formulaKeg{}, false
	}
	prefix := strings.Join(parts[:n-5], string(os.PathSeparator))
	if prefix == "" {
		prefix = string(os.PathSeparator)
	}
	return formulaKeg{prefix, parts[n-4], parts[n-3], filepath.Dir(filepath.Dir(path))}, true
}

func (r *Resolver) renewalCandidate() (string, string, bool, error) {
	self, err := r.self()
	if err != nil {
		return "", "", false, err
	}
	home, err := r.AccountHome()
	if err != nil {
		return "", "", false, err
	}
	receipt, err := readReceipt(home)
	if err != nil || receipt.Channel != "homebrew_formula" {
		return home, self, false, nil
	}
	old, oldOK := parseFormulaKeg(receipt.Executable)
	current, currentOK := parseFormulaKeg(self)
	if !oldOK || !currentOK || old.prefix != current.prefix || old.formula != current.formula || old.version == current.version {
		return home, self, false, nil
	}
	if _, err := os.Lstat(receipt.Executable); !errors.Is(err, fs.ErrNotExist) {
		return home, self, false, nil
	}
	opt, err := filepath.EvalSymlinks(filepath.Join(current.prefix, "opt", current.formula))
	return home, self, err == nil && opt == current.root, nil
}

func (r *Resolver) CanRenewFormula() (bool, error) {
	_, _, eligible, err := r.renewalCandidate()
	return eligible, err
}

// RenewFormula is shared by default-root selection and explicit-root guards.
// No state directory is accessed. False means conditions no longer hold; callers
// must resolve again, including after a competing renewal won the lock.
func (r *Resolver) RenewFormula(ctx context.Context) (bool, error) {
	home, _, eligible, err := r.renewalCandidate()
	if err != nil || !eligible {
		return false, err
	}
	acquire := r.acquireLock
	if acquire == nil {
		acquire = platform.AcquireOwnershipLock
	}
	lock, err := acquire(ctx, home, true)
	if err != nil {
		var busy *platform.OwnershipLockError
		if errors.As(err, &busy) {
			return false, &platform.OwnershipLockError{Reason: "ownership_busy"}
		}
		return false, err
	}
	defer lock.Release()
	home, self, eligible, err := r.renewalCandidate()
	if err != nil || !eligible {
		return false, err
	}
	_, err = r.WriteReceipt(lock, self, "homebrew_formula")
	return err == nil, err
}
