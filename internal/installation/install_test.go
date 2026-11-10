package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func receiptBytes(t *testing.T, r *Resolver) []byte {
	t.Helper()
	home, err := r.AccountHome()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receiptPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func currentReceipt(t *testing.T, r *Resolver) Receipt {
	t.Helper()
	home, _ := r.AccountHome()
	receipt, err := readReceipt(home)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func wantRefused(t *testing.T, err error, reason string) {
	t.Helper()
	var refusal *InstallError
	if !errors.As(err, &refusal) || refusal.Reason != reason {
		t.Fatalf("err = %v, want refusal %s", err, reason)
	}
}

func stageFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".agentdeck.staged")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func stageBundle(t *testing.T, path, content string) {
	t.Helper()
	for _, sub := range []string{"Contents/Helpers", "Contents/PlugIns/AgentDeckWidget.appex", "Contents/MacOS"} {
		if err := os.MkdirAll(filepath.Join(path, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, bundleHelper), []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "Contents/MacOS/AgentDeck"), []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func fileRequest(root, channel, content string, t *testing.T) RegisterRequest {
	dir := filepath.Join(root, "bin")
	return RegisterRequest{
		Channel:         channel,
		Executable:      filepath.Join(dir, "agentdeck"),
		Source:          stageFile(t, dir, content),
		CleanupManifest: filepath.Join(root, "records", channel+"-"+content+".json"),
	}
}

func installFixture(t *testing.T) (*Resolver, string) {
	t.Helper()
	r, root, _ := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "records"), 0o700); err != nil {
		t.Fatal(err)
	}
	// The fixture's own executable lives in root/bin; installs use their own
	// directory so a registered target is never the resolver's self.
	if err := os.Remove(filepath.Join(root, "bin", "agentdeck")); err != nil {
		t.Fatal(err)
	}
	return r, root
}

func TestRegisterPublishesFileAndRecordsManifest(t *testing.T) {
	r, root := installFixture(t)
	request := fileRequest(root, "make", "v1", t)
	staged, err := os.Lstat(request.Source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Register(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Takeover || result.Residual != "" || !installationIDPattern.MatchString(result.InstallationID) {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Lstat(request.Source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged source remains: %v", err)
	}
	published, err := os.Lstat(request.Executable)
	if err != nil || !os.SameFile(staged, published) {
		t.Fatalf("published object is not the staged file: %v", err)
	}
	receipt := currentReceipt(t, r)
	if receipt.Executable != request.Executable || receipt.Channel != "make" || receipt.InstallationID != result.InstallationID {
		t.Fatalf("receipt = %+v", receipt)
	}
	manifest, err := readManifest(request.CleanupManifest)
	if err != nil {
		t.Fatal(err)
	}
	if manifest != (CleanupManifest{result.InstallationID, "make", request.Executable, request.Executable}) {
		t.Fatalf("manifest = %+v", manifest)
	}
	info, err := os.Stat(request.CleanupManifest)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %v %v", info.Mode(), err)
	}
}

func TestRegisterReplacesSameInstallationWithFreshID(t *testing.T) {
	r, root := installFixture(t)
	first, err := r.Register(context.Background(), fileRequest(root, "make", "v1", t))
	if err != nil {
		t.Fatal(err)
	}
	request := fileRequest(root, "make", "v2", t)
	second, err := r.Register(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.InstallationID == first.InstallationID || second.Takeover {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if got := readFile(t, request.Executable); got != "v2" {
		t.Fatalf("target = %q", got)
	}
	if _, err := os.Lstat(request.Source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaced copy was not removed: %v", err)
	}
	// Same path, channel and content still gets a new manifest; an old
	// manifest is never overwritten.
	reuse := fileRequest(root, "make", "v2", t)
	if _, err := r.Register(context.Background(), reuse); err == nil {
		t.Fatal("register overwrote an existing cleanup manifest")
	}
	if got := readFile(t, reuse.Executable); got != "v2" || currentReceipt(t, r).InstallationID != second.InstallationID {
		t.Fatal("refused manifest creation changed the installation")
	}
}

func TestRegisterOwnershipDecisions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		seed     func(t *testing.T, r *Resolver, root string)
		channel  string
		takeover bool
		reason   string
	}{
		{name: "other-live-owner", seed: func(t *testing.T, r *Resolver, root string) {
			other := filepath.Join(root, "other", "agentdeck")
			putExecutable(t, other)
			register(t, r, other, "local_app")
		}, channel: "make", reason: ReasonOwnedByOther},
		{name: "same-target-other-channel", seed: func(t *testing.T, r *Resolver, root string) {
			if _, err := r.Register(context.Background(), fileRequest(root, "local_app", "v0", t)); err != nil {
				t.Fatal(err)
			}
		}, channel: "make", reason: ReasonOwnedByOther},
		{name: "other-live-owner-takeover", seed: func(t *testing.T, r *Resolver, root string) {
			other := filepath.Join(root, "other", "agentdeck")
			putExecutable(t, other)
			register(t, r, other, "local_app")
		}, channel: "make", takeover: true},
		{name: "stale-owner", seed: func(t *testing.T, r *Resolver, root string) {
			other := filepath.Join(root, "other", "agentdeck")
			putExecutable(t, other)
			register(t, r, other, "local_app")
			if err := os.Remove(other); err != nil {
				t.Fatal(err)
			}
		}, channel: "make"},
		{name: "invalid-receipt", seed: func(t *testing.T, r *Resolver, root string) {
			home, _ := r.AccountHome()
			if err := os.MkdirAll(filepath.Dir(receiptPath(home)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(receiptPath(home), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, channel: "make"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root := installFixture(t)
			tc.seed(t, r, root)
			target := filepath.Join(root, "bin", "agentdeck")
			beforeTarget, _ := os.ReadFile(target)
			beforeReceipt := receiptBytes(t, r)
			request := fileRequest(root, tc.channel, "new", t)
			request.Takeover = tc.takeover
			result, err := r.Register(context.Background(), request)
			if tc.reason != "" {
				wantRefused(t, err, tc.reason)
				afterTarget, _ := os.ReadFile(target)
				if !bytes.Equal(beforeTarget, afterTarget) || !bytes.Equal(beforeReceipt, receiptBytes(t, r)) {
					t.Fatal("refused register changed the target or receipt")
				}
				if _, err := os.Lstat(request.CleanupManifest); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("refused register wrote a cleanup manifest")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Takeover != tc.takeover || currentReceipt(t, r).InstallationID != result.InstallationID {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestRegisterOwnerRunningLeavesEverything(t *testing.T) {
	r, root := installFixture(t)
	home, _ := r.AccountHome()
	session, err := platform.AcquireOwnershipLock(context.Background(), home, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Release()
	request := fileRequest(root, "make", "v1", t)
	_, err = r.Register(context.Background(), request)
	wantRefused(t, err, ReasonOwnerRunning)
	if _, err := os.Lstat(request.Executable); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owner_running published the target")
	}
	if receiptBytes(t, r) != nil {
		t.Fatal("owner_running wrote a receipt")
	}
	if _, err := os.Lstat(request.Source); err != nil {
		t.Fatal("owner_running consumed the staged source")
	}
}

func TestRegisterFailureBetweenPublishAndReceiptRestores(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "replace"}[existing], func(t *testing.T) {
			r, root := installFixture(t)
			if existing {
				if _, err := r.Register(context.Background(), fileRequest(root, "make", "old", t)); err != nil {
					t.Fatal(err)
				}
			}
			before := receiptBytes(t, r)
			installHook = func(point string) error {
				if point == "register_published" {
					return errors.New("injected")
				}
				return nil
			}
			defer func() { installHook = nil }()
			request := fileRequest(root, "make", "new", t)
			if _, err := r.Register(context.Background(), request); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("err = %v", err)
			}
			if !bytes.Equal(before, receiptBytes(t, r)) {
				t.Fatal("failed register changed the receipt")
			}
			if existing {
				if got := readFile(t, request.Executable); got != "old" {
					t.Fatalf("target = %q, want restored old", got)
				}
			} else if _, err := os.Lstat(request.Executable); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed fresh register left a target")
			}
			if got := readFile(t, request.Source); got != "new" {
				t.Fatalf("staged source = %q, want it restored", got)
			}
			// The left-over manifest names an ID no receipt carries, so it
			// grants no removal.
			manifest, err := readManifest(request.CleanupManifest)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Unregister(context.Background(), UnregisterRequest{Channel: "make", Executable: request.Executable, ExpectedInstallation: manifest.InstallationID, CleanupManifest: request.CleanupManifest, Remove: true, AuthorizedRepair: false})
			if existing {
				if err != nil || result.Outcome != OutcomeSkippedSuperseded || readFile(t, request.Executable) != "old" {
					t.Fatalf("orphan manifest cleanup = %+v %v", result, err)
				}
			} else if err != nil || result.Outcome != OutcomeAbsent {
				t.Fatalf("orphan manifest cleanup = %+v %v", result, err)
			}
		})
	}
}

func TestRegisterRejectsUnsafeLayouts(t *testing.T) {
	r, root := installFixture(t)
	other := filepath.Join(root, "elsewhere")
	for name, mutate := range map[string]func(*RegisterRequest){
		"source-not-sibling":     func(q *RegisterRequest) { q.Source = stageFile(t, other, "x") },
		"manifest-inside-target": func(q *RegisterRequest) { q.CleanupManifest = q.Executable },
		"missing-manifest":       func(q *RegisterRequest) { q.CleanupManifest = "" },
		"relative-executable":    func(q *RegisterRequest) { q.Executable = "bin/agentdeck" },
		"bundle-into-file-target": func(q *RegisterRequest) {
			q.Source = filepath.Join(root, "bin", "Staged.app")
			stageBundle(t, q.Source, "x")
		},
		"unknown-channel":          func(q *RegisterRequest) { q.Channel = "manual" },
		"source-is-not-executable": func(q *RegisterRequest) { q.Source = filepath.Join(root, "bin", "missing") },
	} {
		t.Run(name, func(t *testing.T) {
			request := fileRequest(root, "make", name, t)
			mutate(&request)
			if _, err := r.Register(context.Background(), request); err == nil {
				t.Fatal("unsafe register succeeded")
			}
			if _, err := os.Lstat(filepath.Join(root, "bin", "agentdeck")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("unsafe register published a target")
			}
			if receiptBytes(t, r) != nil {
				t.Fatal("unsafe register wrote a receipt")
			}
		})
	}
}

func bundleRequest(t *testing.T, root, channel, content string) RegisterRequest {
	t.Helper()
	appdir := filepath.Join(root, "Applications")
	source := filepath.Join(appdir, ".AgentDeck-"+content+".app")
	stageBundle(t, source, content)
	return RegisterRequest{
		Channel:         channel,
		Executable:      filepath.Join(appdir, "AgentDeck.app", bundleHelper),
		Source:          source,
		CleanupManifest: filepath.Join(root, "records", channel+"-"+content+".json"),
	}
}

func TestBundleRegisterSwapAndConditionalRemoval(t *testing.T) {
	r, root := installFixture(t)
	var deregistered []string
	old := deregisterWidgets
	deregisterWidgets = func(path string) error { deregistered = append(deregistered, path); return nil }
	defer func() { deregisterWidgets = old }()
	first := bundleRequest(t, root, "local_app", "v1")
	firstResult, err := r.Register(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	second := bundleRequest(t, root, "local_app", "v2")
	secondResult, err := r.Register(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, second.Executable); got != "v2" {
		t.Fatalf("helper = %q", got)
	}
	if _, err := os.Lstat(second.Source); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("swapped-out bundle was not deleted")
	}
	appRoot := filepath.Join(root, "Applications", "AgentDeck.app")
	manifest, err := readManifest(second.CleanupManifest)
	if err != nil || manifest.RemoveRoot != appRoot {
		t.Fatalf("manifest = %+v %v", manifest, err)
	}
	// The older installation's delayed cleanup sees a different ID and skips.
	result, err := r.Unregister(context.Background(), UnregisterRequest{Channel: "local_app", Executable: first.Executable, ExpectedInstallation: firstResult.InstallationID, CleanupManifest: first.CleanupManifest, Remove: true, AuthorizedRepair: false})
	if err != nil || result.Outcome != OutcomeSkippedSuperseded || len(deregistered) != 0 || readFile(t, second.Executable) != "v2" {
		t.Fatalf("stale cleanup = %+v %v deregistered=%v", result, err, deregistered)
	}
	request := UnregisterRequest{Channel: "local_app", Executable: second.Executable, ExpectedInstallation: secondResult.InstallationID, CleanupManifest: second.CleanupManifest, Remove: true, AuthorizedRepair: false}
	result, err = r.Unregister(context.Background(), request)
	if err != nil || result.Outcome != OutcomeRemoved {
		t.Fatalf("cleanup = %+v %v", result, err)
	}
	if _, err := os.Lstat(appRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("bundle not removed")
	}
	if receiptBytes(t, r) != nil || len(deregistered) != 1 || deregistered[0] != appRoot {
		t.Fatalf("receipt or widgets not cleaned: %v", deregistered)
	}
	if _, err := os.Lstat(filepath.Join(root, "Applications")); err != nil {
		t.Fatal("cleanup removed the bundle's parent directory")
	}
	result, err = r.Unregister(context.Background(), request)
	if err != nil || result.Outcome != OutcomeAbsent {
		t.Fatalf("repeated cleanup = %+v %v", result, err)
	}
}

func TestUnregisterRefusesWithoutProvenOwnership(t *testing.T) {
	setup := func(t *testing.T) (*Resolver, string, RegisterRequest, RegisterResult) {
		r, root := installFixture(t)
		request := fileRequest(root, "make", "v1", t)
		result, err := r.Register(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		return r, root, request, result
	}
	unregister := func(r *Resolver, request RegisterRequest, id string) (UnregisterResult, error) {
		return r.Unregister(context.Background(), UnregisterRequest{Channel: "make", Executable: request.Executable, ExpectedInstallation: id, CleanupManifest: request.CleanupManifest, Remove: true, AuthorizedRepair: false})
	}
	t.Run("receipt-missing-target-present", func(t *testing.T) {
		r, _, request, result := setup(t)
		home, _ := r.AccountHome()
		if err := os.Remove(receiptPath(home)); err != nil {
			t.Fatal(err)
		}
		_, err := unregister(r, request, result.InstallationID)
		wantRefused(t, err, ReasonReceiptMissing)
		if readFile(t, request.Executable) != "v1" {
			t.Fatal("target removed without a receipt")
		}
	})
	t.Run("receipt-corrupt-target-present", func(t *testing.T) {
		r, _, request, result := setup(t)
		home, _ := r.AccountHome()
		if err := os.WriteFile(receiptPath(home), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := unregister(r, request, result.InstallationID)
		wantRefused(t, err, ReasonReceiptInvalid)
		if readFile(t, request.Executable) != "v1" {
			t.Fatal("target removed with a corrupt receipt")
		}
	})
	t.Run("manifest-missing", func(t *testing.T) {
		r, _, request, result := setup(t)
		if err := os.Remove(request.CleanupManifest); err != nil {
			t.Fatal(err)
		}
		_, err := unregister(r, request, result.InstallationID)
		wantRefused(t, err, ReasonReceiptMissing)
		if readFile(t, request.Executable) != "v1" || receiptBytes(t, r) == nil {
			t.Fatal("cleanup without a manifest changed the installation")
		}
	})
	t.Run("expected-id-mismatch", func(t *testing.T) {
		r, _, request, _ := setup(t)
		_, err := unregister(r, request, strings.Repeat("a", 32))
		wantRefused(t, err, ReasonReceiptInvalid)
		if readFile(t, request.Executable) != "v1" || receiptBytes(t, r) == nil {
			t.Fatal("mismatched ID changed the installation")
		}
	})
	t.Run("target-replaced-by-symlink", func(t *testing.T) {
		r, root, request, result := setup(t)
		if err := os.Remove(request.Executable); err != nil {
			t.Fatal(err)
		}
		putExecutable(t, filepath.Join(root, "victim"))
		if err := os.Symlink(filepath.Join(root, "victim"), request.Executable); err != nil {
			t.Fatal(err)
		}
		if _, err := unregister(r, request, result.InstallationID); err == nil {
			t.Fatal("cleanup followed or removed a symlink target")
		}
		if receiptBytes(t, r) == nil || readFile(t, filepath.Join(root, "victim")) != "fixture executable" {
			t.Fatal("refused cleanup changed the receipt or the link target")
		}
	})
	t.Run("same-path-same-channel-reinstall", func(t *testing.T) {
		r, root, request, result := setup(t)
		newer, err := r.Register(context.Background(), fileRequest(root, "make", "v2", t))
		if err != nil {
			t.Fatal(err)
		}
		got, err := unregister(r, request, result.InstallationID)
		if err != nil || got.Outcome != OutcomeSkippedSuperseded {
			t.Fatalf("old cleanup = %+v %v", got, err)
		}
		if readFile(t, request.Executable) != "v2" || currentReceipt(t, r).InstallationID != newer.InstallationID {
			t.Fatal("old cleanup removed the newer same-channel installation")
		}
	})
	t.Run("receipt-only", func(t *testing.T) {
		r, _, request, result := setup(t)
		got, err := r.Unregister(context.Background(), UnregisterRequest{Channel: "make", Executable: request.Executable, ExpectedInstallation: result.InstallationID, CleanupManifest: request.CleanupManifest, Remove: false, AuthorizedRepair: false})
		if err != nil || got.Outcome != OutcomeUnregistered || receiptBytes(t, r) != nil || readFile(t, request.Executable) != "v1" {
			t.Fatalf("receipt-only = %+v %v", got, err)
		}
	})
	t.Run("owner-running", func(t *testing.T) {
		r, _, request, result := setup(t)
		r.acquireLock = func(context.Context, string, bool) (*platform.OwnershipLock, error) {
			return nil, &platform.OwnershipLockError{Reason: "owner_running"}
		}
		_, err := unregister(r, request, result.InstallationID)
		wantRefused(t, err, ReasonOwnerRunning)
		if readFile(t, request.Executable) != "v1" || receiptBytes(t, r) == nil {
			t.Fatal("owner_running cleanup changed the installation")
		}
	})
}

func formulaLayout(t *testing.T, root, prefix, formula, version string, current bool) string {
	t.Helper()
	path := filepath.Join(root, prefix, "Cellar", formula, version, "bin", "agentdeck")
	putExecutable(t, path)
	if current {
		opt := filepath.Join(root, prefix, "opt", formula)
		_ = os.Remove(opt)
		if err := os.MkdirAll(filepath.Dir(opt), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(filepath.Dir(path)), opt); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestClaimCurrentFormulaKeg(t *testing.T) {
	for _, tc := range []struct {
		name     string
		self     func(t *testing.T, root string) string
		seed     func(t *testing.T, r *Resolver, root string)
		takeover bool
		reason   string
		took     bool
	}{
		{name: "current-stable", self: func(t *testing.T, root string) string {
			return formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
		}},
		{name: "current-rc", self: func(t *testing.T, root string) string {
			return formulaLayout(t, root, "prefix-a", "agentdeck-rc", "1.1-rc1", true)
		}},
		{name: "second-prefix", self: func(t *testing.T, root string) string {
			formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
			return formulaLayout(t, root, "prefix-b", "agentdeck", "1.0", true)
		}},
		{name: "retained-old-keg", self: func(t *testing.T, root string) string {
			old := formulaLayout(t, root, "prefix-a", "agentdeck", "0.9", false)
			formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
			return old
		}, reason: ReasonNotCurrentFormulaKeg},
		{name: "opt-missing", self: func(t *testing.T, root string) string {
			return formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", false)
		}, reason: ReasonNotCurrentFormulaKeg},
		{name: "not-a-keg", self: func(t *testing.T, root string) string {
			path := filepath.Join(root, "dist", "agentdeck")
			putExecutable(t, path)
			return path
		}, reason: ReasonNotCurrentFormulaKeg},
		{name: "other-owner", self: func(t *testing.T, root string) string {
			return formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
		}, seed: func(t *testing.T, r *Resolver, root string) {
			register(t, r, formulaLayout(t, root, "prefix-b", "agentdeck", "1.0", true), "homebrew_formula")
		}, reason: ReasonOwnedByOther},
		{name: "other-owner-takeover", self: func(t *testing.T, root string) string {
			return formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
		}, seed: func(t *testing.T, r *Resolver, root string) {
			register(t, r, formulaLayout(t, root, "prefix-a", "agentdeck-rc", "1.1", true), "homebrew_formula")
		}, takeover: true, took: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root := installFixture(t)
			if tc.seed != nil {
				tc.seed(t, r, root)
			}
			self := tc.self(t, root)
			r.executable = func() (string, error) { return self, nil }
			before := receiptBytes(t, r)
			result, err := r.Claim(context.Background(), tc.takeover)
			if tc.reason != "" {
				wantRefused(t, err, tc.reason)
				if !bytes.Equal(before, receiptBytes(t, r)) {
					t.Fatal("refused claim changed the receipt")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			receipt := currentReceipt(t, r)
			if result != (ClaimResult{"homebrew_formula", tc.took}) || receipt.Executable != self || receipt.Channel != "homebrew_formula" {
				t.Fatalf("result=%+v receipt=%+v", result, receipt)
			}
			if got := r.Resolve(); !got.Installed {
				t.Fatalf("claimed keg is not installed: %+v", got)
			}
		})
	}
}

func TestClaimOwnerRunningKeepsReceipt(t *testing.T) {
	r, root := installFixture(t)
	register(t, r, formulaLayout(t, root, "prefix-b", "agentdeck", "1.0", true), "homebrew_formula")
	self := formulaLayout(t, root, "prefix-a", "agentdeck", "1.0", true)
	r.executable = func() (string, error) { return self, nil }
	home, _ := r.AccountHome()
	session, err := platform.AcquireOwnershipLock(context.Background(), home, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Release()
	before := receiptBytes(t, r)
	_, err = r.Claim(context.Background(), true)
	wantRefused(t, err, ReasonOwnerRunning)
	if !bytes.Equal(before, receiptBytes(t, r)) {
		t.Fatal("owner_running claim changed the receipt")
	}
}
