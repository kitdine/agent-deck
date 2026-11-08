package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type ownershipTestResolver struct {
	account   string
	installed atomic.Bool
	resolved  chan struct{}
}

func (r *ownershipTestResolver) AccountHome() (string, error) { return r.account, nil }
func (r *ownershipTestResolver) Resolve() InstallationIdentity {
	if r.resolved != nil {
		select {
		case r.resolved <- struct{}{}:
		default:
		}
	}
	if r.installed.Load() {
		return InstallationIdentity{Installed: true, Channel: "make"}
	}
	return InstallationIdentity{Reason: "receipt_other_executable"}
}

// Account injection stays private to this package's tests. These tests are
// serial because the production resolver is a process-startup dependency.
func ownershipFixture(t *testing.T) (*ownershipTestResolver, string) {
	t.Helper()
	account, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &ownershipTestResolver{account: account}
	installationResolver.RLock()
	old := installationResolver.value
	installationResolver.RUnlock()
	oldAccountHome := statePaths.accountHome
	SetInstallationResolver(r)
	statePaths = pathResolver{accountHome: AccountHome, lstat: os.Lstat, readlink: os.Readlink}
	t.Cleanup(func() {
		if err := ReleaseStateOwnership(); err != nil {
			t.Error(err)
		}
		SetInstallationResolver(old)
		statePaths = pathResolver{accountHome: oldAccountHome, lstat: os.Lstat, readlink: os.Readlink}
	})
	return r, filepath.Join(account, ".agentdeck")
}

func requireStateRefusal(t *testing.T, err error, reason string) {
	t.Helper()
	var refused *ErrStateOwnershipRefused
	if !errors.As(err, &refused) || refused.Resource != "state_root" || refused.Reason != reason {
		t.Fatalf("refusal = %#v, %v; want %s", refused, err, reason)
	}
}

func TestGuardStateRootRefusesBeforeDescendantMetadata(t *testing.T) {
	r, real := ownershipFixture(t)
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(r.account, "outside")
	inner := filepath.Join(r.account, "inner")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inner, filepath.Join(outside, "jump")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(outside, "alias")); err != nil {
		t.Fatal(err)
	}
	statePaths.lstat = func(path string) (os.FileInfo, error) {
		if filepath.Dir(path) == real {
			t.Fatalf("guard reached protected child %q", path)
		}
		return os.Lstat(path)
	}
	paths := []string{real, real + "/sentinel", outside + "/alias/sentinel", outside + "/jump/../.agentdeck/sentinel"}
	// Case folding is a volume property, not a property of every supported
	// host. The macOS barrier suite independently checks native aliases too.
	caseAlias := filepath.Join(r.account, ".AGENTDECK")
	actual, _ := os.Stat(real)
	if alias, err := os.Stat(caseAlias); err == nil && os.SameFile(actual, alias) {
		paths = append(paths, caseAlias+"/sentinel")
	} else if _, err := GuardStateRoot(caseAlias + "/sentinel"); err != nil {
		t.Fatalf("distinct case-sensitive state path refused: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path)+path[len(r.account):], func(t *testing.T) {
			_, err := GuardStateRoot(path)
			requireStateRefusal(t, err, "receipt_other_executable")
		})
	}
	if _, err := os.Stat(filepath.Join(r.account, "Library")); !os.IsNotExist(err) {
		t.Fatalf("uninstalled guard created ownership-lock directory: %v", err)
	}
}

func TestGuardStateRootUsesPhysicalPathForAllowedWrites(t *testing.T) {
	r, _ := ownershipFixture(t)
	physical := filepath.Join(r.account, "physical")
	if err := os.MkdirAll(filepath.Join(physical, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(r.account, "link")
	if err := os.Symlink(filepath.Join(physical, "inner"), link); err != nil {
		t.Fatal(err)
	}
	input := link + "/../state"
	want := filepath.Join(physical, "state")
	got, err := GuardStateRoot(input)
	if err != nil || got != want {
		t.Fatalf("physical path = %q, %v; want %q", got, err, want)
	}
	if err := EnsureStateRoot(input); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.account, "state")); !os.IsNotExist(err) {
		t.Fatalf("lexically cleaned path was used: %v", err)
	}
}

func TestGuardStateRootSessionBlocksExclusiveUntilTeardown(t *testing.T) {
	r, real := ownershipFixture(t)
	r.installed.Store(true)
	got, err := GuardStateRoot(real)
	if err != nil || got != real {
		t.Fatalf("guard = %q, %v", got, err)
	}
	if _, err := GuardStateRoot(real); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if lock, err := AcquireOwnershipLock(ctx, r.account, true); !errors.Is(err, context.DeadlineExceeded) {
		if lock != nil {
			lock.Release()
		}
		t.Fatalf("exclusive acquired during shared session: %v", err)
	}
	if err := ReleaseStateOwnership(); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireOwnershipLock(context.Background(), r.account, true)
	if err != nil {
		t.Fatal(err)
	}
	lock.Release()
}

func TestGuardStateRootResolvesIdentityAfterWaitingForSharedLock(t *testing.T) {
	r, real := ownershipFixture(t)
	r.installed.Store(true)
	r.resolved = make(chan struct{}, 2)
	exclusive, err := AcquireOwnershipLock(context.Background(), r.account, true)
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive.Release()
	done := make(chan error, 1)
	go func() { _, err := GuardStateRoot(real); done <- err }()
	select {
	case <-r.resolved:
	case <-time.After(2 * time.Second):
		t.Fatal("guard never checked initial identity")
	}
	r.installed.Store(false)
	exclusive.Release()
	select {
	case err := <-done:
		requireStateRefusal(t, err, "receipt_other_executable")
	case <-time.After(2 * time.Second):
		t.Fatal("guard did not resume")
	}
	if _, err := os.Stat(real); !os.IsNotExist(err) {
		t.Fatalf("refused root was created: %v", err)
	}
}

func TestRevalidateOwnershipMarksRevocationAndRetainsLockForTeardown(t *testing.T) {
	r, real := ownershipFixture(t)
	r.installed.Store(true)
	if _, err := GuardStateRoot(real); err != nil {
		t.Fatal(err)
	}
	r.installed.Store(false)
	err := RevalidateOwnership(real)
	requireStateRefusal(t, err, "receipt_other_executable")
	var refused *ErrStateOwnershipRefused
	if !errors.As(err, &refused) || !refused.Revoked {
		t.Fatal("missing revoked disposition")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if lock, err := AcquireOwnershipLock(ctx, r.account, true); !errors.Is(err, context.DeadlineExceeded) {
		if lock != nil {
			lock.Release()
		}
		t.Fatalf("session released before consumer teardown: %v", err)
	}
}

func TestGuardStateRootFailsClosedOnUnresolvablePathsAndMissingResolver(t *testing.T) {
	r, real := ownershipFixture(t)
	_, err := GuardStateRoot(filepath.Join(r.account, "missing") + "/../state")
	requireStateRefusal(t, err, "state_path_unresolved")
	SetInstallationResolver(nil)
	// Protected-set discovery uses the isolated test account independently of
	// the deliberately absent identity resolver.
	statePaths = pathResolver{accountHome: r.AccountHome, lstat: os.Lstat, readlink: os.Readlink}
	_, err = GuardStateRoot(real)
	requireStateRefusal(t, err, "receipt_missing")
	statePaths = pathResolver{accountHome: func() (string, error) { return "", errors.New("unavailable account") }, lstat: os.Lstat, readlink: os.Readlink}
	_, err = GuardStateRoot(filepath.Join(r.account, "isolated"))
	requireStateRefusal(t, err, "real_root_unresolved")
}
