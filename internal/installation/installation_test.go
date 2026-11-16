package installation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

func fixture(t *testing.T) (*Resolver, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "account")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(root, "environment-home"))
	self := filepath.Join(root, "bin", "agentdeck")
	putExecutable(t, self)
	r := &Resolver{accountHome: func() (string, error) { return home, nil }, executable: func() (string, error) { return self, nil }}
	return r, root, self
}

func putExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture executable"), 0700); err != nil {
		t.Fatal(err)
	}
}

func register(t *testing.T, r *Resolver, path, channel string) Receipt {
	t.Helper()
	home, err := r.AccountHome()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := platform.AcquireOwnershipLock(context.Background(), home, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	receipt, err := r.WriteReceipt(lock, path, channel)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func TestResolveReasonsAndLiveReceipt(t *testing.T) {
	r, root, self := fixture(t)
	assertReason := func(want string) {
		t.Helper()
		got := r.Resolve()
		if got.Installed || got.Channel != "" || got.Reason != want {
			t.Fatalf("identity = %+v, want %s", got, want)
		}
	}
	assertReason("receipt_missing")
	home, _ := r.AccountHome()
	if err := os.MkdirAll(filepath.Dir(receiptPath(home)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath(home), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	assertReason("receipt_invalid")
	other := filepath.Join(root, "other", "agentdeck")
	putExecutable(t, other)
	register(t, r, other, "make")
	assertReason("receipt_other_executable")
	register(t, r, self, "local_app")
	if got := r.Resolve(); !got.Installed || got.Channel != "local_app" || got.Reason != "" {
		t.Fatalf("identity = %+v", got)
	}
	register(t, r, other, "homebrew_cask")
	assertReason("receipt_other_executable")
	if err := os.Remove(other); err != nil {
		t.Fatal(err)
	}
	assertReason("receipt_missing")
	r.accountHome = func() (string, error) { return "", errors.New("account failure") }
	assertReason("real_root_unresolved")
	r.executable = func() (string, error) { return "", errors.New("exec failure") }
	assertReason("executable_unresolved")
}

func TestResolveChannelsSymlinkAndEnvironment(t *testing.T) {
	r, root, self := fixture(t)
	link := filepath.Join(root, "command")
	if err := os.Symlink(self, link); err != nil {
		t.Fatal(err)
	}
	r.executable = func() (string, error) { return link, nil }
	for _, channel := range []string{"homebrew_cask", "homebrew_formula", "make", "local_app"} {
		register(t, r, self, channel)
		t.Setenv("HOME", filepath.Join(root, channel))
		t.Setenv("AGENTDECK_INSTALLED", "false")
		t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", filepath.Join(root, "fake"))
		if got := r.Resolve(); !got.Installed || got.Channel != channel {
			t.Fatalf("%s: %+v", channel, got)
		}
	}
}

func TestOnlyRecordedKegOwnsStateAcrossOptSwitch(t *testing.T) {
	r, root, _ := fixture(t)
	paths := []string{
		filepath.Join(root, "prefix-a/Cellar/agentdeck/1/bin/agentdeck"),
		filepath.Join(root, "prefix-a/Cellar/agentdeck/2/bin/agentdeck"),
		filepath.Join(root, "prefix-b/Cellar/agentdeck/1/bin/agentdeck"),
		filepath.Join(root, "prefix-a/Cellar/agentdeck-rc/1/bin/agentdeck"),
	}
	for _, path := range paths {
		putExecutable(t, path)
	}
	register(t, r, paths[0], "homebrew_formula")
	opt := filepath.Join(root, "prefix-a/opt/agentdeck")
	if err := os.MkdirAll(filepath.Dir(opt), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.Dir(paths[0])), opt); err != nil {
		t.Fatal(err)
	}
	for _, target := range paths[:2] {
		if err := os.Remove(opt); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(filepath.Dir(target)), opt); err != nil {
			t.Fatal(err)
		}
		for i, path := range paths {
			r.executable = func() (string, error) { return path, nil }
			got := r.Resolve()
			if got.Installed != (i == 0) || (i > 0 && got.Reason != "receipt_other_executable") {
				t.Fatalf("opt %s self %s: %+v", target, path, got)
			}
		}
	}
}

func TestFormulaRenewalConditions(t *testing.T) {
	for _, missing := range []string{"", "invalid-receipt", "wrong-channel", "old-exists", "cross-formula", "cross-prefix", "not-current"} {
		t.Run(missing, func(t *testing.T) {
			r, root, _ := fixture(t)
			old := filepath.Join(root, "prefix/Cellar/agentdeck/1/bin/agentdeck")
			self := filepath.Join(root, "prefix/Cellar/agentdeck/2/bin/agentdeck")
			if missing == "cross-formula" {
				self = filepath.Join(root, "prefix/Cellar/agentdeck-rc/2/bin/agentdeck")
			}
			if missing == "cross-prefix" {
				self = filepath.Join(root, "other/Cellar/agentdeck/2/bin/agentdeck")
			}
			putExecutable(t, old)
			putExecutable(t, self)
			r.executable = func() (string, error) { return self, nil }
			channel := "homebrew_formula"
			if missing == "wrong-channel" {
				channel = "make"
			}
			previous := register(t, r, old, channel)
			if missing != "old-exists" {
				if err := os.Remove(old); err != nil {
					t.Fatal(err)
				}
			}
			keg, _ := parseFormulaKeg(self)
			opt := filepath.Join(keg.prefix, "opt", keg.formula)
			if err := os.MkdirAll(filepath.Dir(opt), 0700); err != nil {
				t.Fatal(err)
			}
			target := keg.root
			if missing == "not-current" {
				target = filepath.Join(root, "other-keg")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, opt); err != nil {
				t.Fatal(err)
			}
			home, _ := r.AccountHome()
			path := receiptPath(home)
			if missing == "invalid-receipt" {
				if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := missing == ""
			if got, err := r.CanRenewFormula(); err != nil || got != want {
				t.Fatalf("eligible=%v err=%v", got, err)
			}
			if got, err := r.RenewFormula(context.Background()); err != nil || got != want {
				t.Fatalf("renewed=%v err=%v", got, err)
			}
			if want {
				got, err := r.ReadReceipt()
				if err != nil {
					t.Fatal(err)
				}
				if !r.Resolve().Installed || got.Executable != self || got.InstallationID == previous.InstallationID {
					t.Fatalf("receipt=%+v", got)
				}
				if got, err := r.RenewFormula(context.Background()); got || err != nil {
					t.Fatalf("second renewal=%v %v", got, err)
				}
			} else {
				after, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(after) {
					t.Fatal("ineligible renewal changed receipt")
				}
			}
		})
	}
}

func TestRenewalWaitRechecksReceipt(t *testing.T) {
	r, root, _ := fixture(t)
	old := filepath.Join(root, "prefix/Cellar/agentdeck/1/bin/agentdeck")
	self := filepath.Join(root, "prefix/Cellar/agentdeck/2/bin/agentdeck")
	other := filepath.Join(root, "other/agentdeck")
	for _, path := range []string{old, self, other} {
		putExecutable(t, path)
	}
	register(t, r, old, "homebrew_formula")
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	opt := filepath.Join(root, "prefix/opt/agentdeck")
	if err := os.MkdirAll(filepath.Dir(opt), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.Dir(self)), opt); err != nil {
		t.Fatal(err)
	}
	home, _ := r.AccountHome()
	lock, err := platform.AcquireOwnershipLock(context.Background(), home, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	r.executable = func() (string, error) { return self, nil }
	started := make(chan struct{})
	proceed := make(chan struct{})
	result := make(chan error, 1)
	r.acquireLock = func(ctx context.Context, account string, exclusive bool) (*platform.OwnershipLock, error) {
		close(started)
		<-proceed
		return platform.AcquireOwnershipLock(ctx, account, exclusive)
	}
	go func() {
		renewed, err := r.RenewFormula(context.Background())
		if renewed {
			err = errors.New("renewal overwrote competing owner")
		}
		result <- err
	}()
	<-started
	// Eligibility was read before this replacement. The lock-time recheck must
	// observe the new owner, without overwriting its receipt.
	if _, err := r.WriteReceipt(lock, other, "make"); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	close(proceed)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(); got.Reason != "receipt_other_executable" {
		t.Fatalf("identity=%+v", got)
	}
}

func TestReceiptFreshIDSameTimestamp(t *testing.T) {
	r, _, self := fixture(t)
	r.now = func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) }
	a := register(t, r, self, "make")
	b := register(t, r, self, "make")
	if a.InstallationID == b.InstallationID || !a.InstalledAt.Equal(b.InstalledAt) {
		t.Fatalf("first=%+v second=%+v", a, b)
	}
	home, _ := r.AccountHome()
	for path, want := range map[string]os.FileMode{receiptPath(home): 0600, filepath.Dir(receiptPath(home)): 0700} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s mode=%v err=%v", path, info, err)
		}
	}
	var wire map[string]any
	data, err := os.ReadFile(receiptPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire) != 5 {
		t.Fatalf("unexpected receipt fields: %v", wire)
	}
}

func TestRenewalLockTimeoutPreservesReceipt(t *testing.T) {
	r, root, _ := fixture(t)
	old := filepath.Join(root, "prefix/Cellar/agentdeck/1/bin/agentdeck")
	self := filepath.Join(root, "prefix/Cellar/agentdeck/2/bin/agentdeck")
	putExecutable(t, old)
	putExecutable(t, self)
	register(t, r, old, "homebrew_formula")
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	opt := filepath.Join(root, "prefix/opt/agentdeck")
	if err := os.MkdirAll(filepath.Dir(opt), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.Dir(self)), opt); err != nil {
		t.Fatal(err)
	}
	r.executable = func() (string, error) { return self, nil }
	home, _ := r.AccountHome()
	before, err := os.ReadFile(receiptPath(home))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := platform.AcquireOwnershipLock(context.Background(), home, false)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	renewed, err := r.RenewFormula(context.Background())
	var busy *platform.OwnershipLockError
	if renewed || !errors.As(err, &busy) || busy.Reason != "ownership_busy" {
		t.Fatalf("renewed=%v err=%v", renewed, err)
	}
	after, err := os.ReadFile(receiptPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("timed out renewal changed receipt")
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if renewed, err := r.RenewFormula(context.Background()); !renewed || err != nil {
		t.Fatalf("renewal after release=%v %v", renewed, err)
	}
	// A surviving old process loses its executable even if its resolver object
	// predates the upgrade; no cached identity may keep it installed.
	r.executable = func() (string, error) { return old, nil }
	if got := r.Resolve(); got.Reason != "executable_unresolved" {
		t.Fatalf("old process=%+v", got)
	}
}
