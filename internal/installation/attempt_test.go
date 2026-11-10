package installation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testToken   = "agentdeck-app"
	testVersion = "1.2.3"
)

type caskFixture struct {
	r          *Resolver
	root       string
	home       string
	staged     string
	executable string
	// helperCalls records which program ran each unregister.
	helperCalls []string
}

// newCaskFixture replaces the helper runner with the in-process protocol so
// the dispatcher's decisions are observed without executing a program.
func newCaskFixture(t *testing.T) *caskFixture {
	t.Helper()
	r, root, _ := fixture(t)
	f := &caskFixture{r: r, root: root, home: filepath.Join(root, "account"), staged: filepath.Join(root, "Caskroom", testToken, testVersion)}
	f.executable = filepath.Join(root, "Applications", "AgentDeck.app", bundleHelper)
	for _, dir := range []string{f.staged, filepath.Join(root, "Applications")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	oldRun, oldWidgets := runHelper, deregisterWidgets
	deregisterWidgets = func(string) error { return nil }
	runHelper = func(ctx context.Context, program string, args ...string) ([]byte, []byte, error) {
		f.helperCalls = append(f.helperCalls, program)
		if !usableHelper(program) {
			return nil, nil, errors.New("exec: no such program")
		}
		request := UnregisterRequest{}
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--channel":
				i++
				request.Channel = args[i]
			case "--executable":
				i++
				request.Executable = args[i]
			case "--expected-installation":
				i++
				request.ExpectedInstallation = args[i]
			case "--cleanup-manifest":
				i++
				request.CleanupManifest = args[i]
			case "--remove":
				request.Remove = true
			case "--authorized-repair":
				request.AuthorizedRepair = true
			case "--require-identity":
				request.RequireIdentity = true
			default:
				t.Fatalf("unexpected helper argument %q", args[i])
			}
		}
		result, err := r.Unregister(ctx, request)
		if err != nil {
			var refusal *InstallError
			if errors.As(err, &refusal) {
				return nil, []byte("install_claim_refused: refused\n  reason: " + refusal.Reason + "\n"), errors.New("exit status 1")
			}
			return nil, []byte(err.Error()), errors.New("exit status 1")
		}
		return []byte(strings.ReplaceAll(result.Outcome, "_", " ") + " " + request.Executable + "\n"), nil, nil
	}
	t.Cleanup(func() { runHelper, deregisterWidgets = oldRun, oldWidgets })
	return f
}

func (f *caskFixture) request(id string) AttemptRequest {
	return AttemptRequest{AttemptID: id, Token: testToken, Version: testVersion, Executable: f.executable, Locator: filepath.Join(f.staged, AttemptLocatorName)}
}

func attemptID(n int) string { return fmt.Sprintf("%032x", n) }

// install runs one whole cask attempt: materials, publication, completion.
// A failed attempt puts the previous locator back, as Homebrew does when it
// restores the backed-up staged directory of the version it tried to replace.
func (f *caskFixture) install(t *testing.T, n int, mutate func(*RegisterRequest)) (AttemptMaterials, RegisterResult, error) {
	t.Helper()
	locator := filepath.Join(f.staged, AttemptLocatorName)
	previous, previousErr := os.ReadFile(locator)
	materials, result, err := f.attempt(t, n, mutate)
	if err != nil {
		if previousErr != nil {
			_ = os.Remove(locator)
		} else if writeErr := os.WriteFile(locator, previous, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return materials, result, err
}

func (f *caskFixture) attempt(t *testing.T, n int, mutate func(*RegisterRequest)) (AttemptMaterials, RegisterResult, error) {
	t.Helper()
	materials, err := f.r.PrepareAttempt(f.request(attemptID(n)))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(f.root, "Applications", ".AgentDeck.app."+materials.AttemptID)
	stageBundle(t, source, "attempt-"+materials.AttemptID)
	request := RegisterRequest{Channel: caskChannel, Executable: f.executable, Source: source, CleanupManifest: materials.Manifest, RequireUnowned: true}
	if mutate != nil {
		mutate(&request)
	}
	result, err := f.r.Register(context.Background(), request)
	if err == nil {
		err = f.r.CompleteAttempt(f.request(materials.AttemptID))
	}
	return materials, result, err
}

func (f *caskFixture) cleanup(mutate func(*CleanupRequest)) (string, error) {
	request := CleanupRequest{AttemptRequest: f.request("")}
	if mutate != nil {
		mutate(&request)
	}
	return f.r.CleanupAttempt(context.Background(), request)
}

func (f *caskFixture) app() string { return filepath.Join(f.root, "Applications", "AgentDeck.app") }

func TestPrepareAttemptCreatesPrivateImmutableMaterials(t *testing.T) {
	f := newCaskFixture(t)
	first, err := f.r.PrepareAttempt(f.request(attemptID(1)))
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(f.home, "Library", "Application Support", "AgentDeck", "installers", "homebrew_cask", "attempts", attemptID(1))
	if first.Directory != wantDir || first.Helper != filepath.Join(wantDir, "helper") || first.Manifest != filepath.Join(wantDir, "manifest.json") {
		t.Fatalf("materials = %+v", first)
	}
	for path, mode := range map[string]os.FileMode{first.Directory: os.ModeDir | 0o700, first.Helper: 0o700, filepath.Join(first.Directory, "attempt.json"): 0o600, filepath.Join(f.staged, AttemptLocatorName): 0o600} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode() != mode {
			t.Fatalf("%s mode = %v %v, want %v", path, info.Mode(), err, mode)
		}
	}
	if readFile(t, first.Helper) != "fixture executable" {
		t.Fatal("retained helper is not the preparing program")
	}
	if _, err := os.Lstat(first.Manifest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("manifest exists before register")
	}
	// Nothing lands under the environment HOME, only under the account.
	if entries, _ := os.ReadDir(filepath.Join(f.root, "environment-home")); len(entries) != 0 {
		t.Fatalf("HOME was written: %v", entries)
	}
	before := treeBytes(t, first.Directory)
	if _, err := f.r.PrepareAttempt(f.request(attemptID(1))); err == nil {
		t.Fatal("an attempt directory was reused")
	}
	second, err := f.r.PrepareAttempt(f.request(attemptID(2)))
	if err != nil {
		t.Fatal(err)
	}
	if treeBytes(t, first.Directory) != before || second.Directory == first.Directory {
		t.Fatal("the earlier attempt changed")
	}
	located, err := readAttemptRecord(filepath.Join(f.staged, AttemptLocatorName))
	if err != nil || located.AttemptID != attemptID(2) {
		t.Fatalf("locator = %+v %v", located, err)
	}
}

func TestPrepareAttemptRejectsUnsafeInput(t *testing.T) {
	f := newCaskFixture(t)
	for name, mutate := range map[string]func(*AttemptRequest){
		"short-id":         func(q *AttemptRequest) { q.AttemptID = "abc" },
		"path-id":          func(q *AttemptRequest) { q.AttemptID = "../" + attemptID(1)[3:] },
		"token":            func(q *AttemptRequest) { q.Token = "Bad/Token" },
		"version":          func(q *AttemptRequest) { q.Version = "1/2" },
		"relative-target":  func(q *AttemptRequest) { q.Executable = "AgentDeck.app/Contents/Helpers/agentdeck" },
		"locator-name":     func(q *AttemptRequest) { q.Locator = filepath.Join(f.staged, "other") },
		"relative-locator": func(q *AttemptRequest) { q.Locator = AttemptLocatorName },
	} {
		request := f.request(attemptID(1))
		mutate(&request)
		if _, err := f.r.PrepareAttempt(request); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if _, err := os.Lstat(filepath.Join(f.home, "Library")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected input created account files")
	}
	// A symlink planted on the materials path is never followed.
	elsewhere := filepath.Join(f.root, "elsewhere")
	support := filepath.Join(f.home, "Library", "Application Support", "AgentDeck")
	for _, dir := range []string{elsewhere, support} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(elsewhere, filepath.Join(support, "installers")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.r.PrepareAttempt(f.request(attemptID(1))); err == nil {
		t.Fatal("materials followed a symlink")
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("symlink destination was written: %v", entries)
	}
}

func TestPrepareAttemptFailsWhenAccountIsNotWritable(t *testing.T) {
	f := newCaskFixture(t)
	if err := os.Chmod(f.home, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(f.home, 0o700)
	if _, err := f.r.PrepareAttempt(f.request(attemptID(1))); err == nil {
		t.Fatal("prepare succeeded without a writable account directory")
	}
	if _, err := os.Lstat(filepath.Join(f.staged, AttemptLocatorName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a locator was written for a failed attempt")
	}
	if entries, _ := os.ReadDir(filepath.Join(f.root, "environment-home")); len(entries) != 0 {
		t.Fatalf("fell back to HOME: %v", entries)
	}
}

func TestCleanupAttemptNormalAndDelayed(t *testing.T) {
	f := newCaskFixture(t)
	first, _, err := f.install(t, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := f.cleanup(nil)
	if err != nil || outcome != OutcomeRemoved {
		t.Fatalf("cleanup = %q %v", outcome, err)
	}
	if _, err := os.Lstat(f.app()); !errors.Is(err, os.ErrNotExist) || receiptBytes(t, f.r) != nil {
		t.Fatal("installation not removed")
	}
	if f.helperCalls[0] != first.Helper {
		t.Fatalf("cleanup ran %s, want the retained helper", f.helperCalls[0])
	}
	// Cleanup marks the attempt and keeps every immutable material.
	for _, name := range []string{"cleaned", "published", "attempt.json", "helper", "manifest.json"} {
		if _, err := os.Lstat(filepath.Join(first.Directory, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	firstBytes := treeBytes(t, first.Directory)
	// A same-version reinstall gets fresh materials; the old ones are intact.
	second, secondResult, err := f.install(t, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if treeBytes(t, first.Directory) != firstBytes || second.Manifest == first.Manifest {
		t.Fatal("reinstall touched the earlier attempt")
	}
	// The earlier attempt's delayed rollback cannot remove the new install.
	outcome, err = f.cleanup(func(q *CleanupRequest) { q.AttemptID = first.AttemptID })
	if err != nil || outcome != OutcomeSkippedSuperseded {
		t.Fatalf("delayed cleanup = %q %v", outcome, err)
	}
	if currentReceipt(t, f.r).InstallationID != secondResult.InstallationID || readFile(t, f.executable) != "attempt-"+second.AttemptID {
		t.Fatal("delayed cleanup damaged the newer installation")
	}
	// A destroyed App helper does not stop the retained helper.
	if err := os.WriteFile(f.executable, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome, err = f.cleanup(nil)
	if err != nil || outcome != OutcomeRemoved {
		t.Fatalf("cleanup with a broken App helper = %q %v", outcome, err)
	}
}

func TestCleanupAttemptRollbackNeverCarriesAuthority(t *testing.T) {
	f := newCaskFixture(t)
	materials, _, err := f.install(t, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CleanupRequest){
		func(q *CleanupRequest) { q.AuthorizedRepair = true },
		func(q *CleanupRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(otherID) },
	} {
		_, err := f.cleanup(func(q *CleanupRequest) { q.AttemptID = materials.AttemptID; mutate(q) })
		if err == nil || len(f.helperCalls) != 0 {
			t.Fatalf("rollback accepted repair authority: %v", err)
		}
	}
	// A rollback whose manifest is gone keeps the target and reports it.
	if err := os.Remove(materials.Manifest); err != nil {
		t.Fatal(err)
	}
	before := treeBytes(t, f.app())
	_, err = f.cleanup(func(q *CleanupRequest) { q.AttemptID = materials.AttemptID })
	if err == nil || !strings.Contains(err.Error(), materials.AttemptID) || treeBytes(t, f.app()) != before {
		t.Fatalf("rollback without a manifest = %v", err)
	}
}

// damage removes exactly one cleanup material of an otherwise valid install.
var materialDamage = map[string]func(t *testing.T, f *caskFixture, m AttemptMaterials){
	"manifest": func(t *testing.T, f *caskFixture, m AttemptMaterials) { mustRemove(t, m.Manifest) },
	"helper":   func(t *testing.T, f *caskFixture, m AttemptMaterials) { mustRemove(t, m.Helper) },
	"locator": func(t *testing.T, f *caskFixture, m AttemptMaterials) {
		mustRemove(t, filepath.Join(f.staged, AttemptLocatorName))
	},
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// TestBoundRepairWithMissingMaterials is the helper-layer chain for a valid
// receipt whose cleanup materials are individually lost: capture the identity,
// run the authorized uninstall, then the bound register, then a plain uninstall.
func TestBoundRepairWithMissingMaterials(t *testing.T) {
	for name, damage := range materialDamage {
		t.Run(name, func(t *testing.T) {
			f := newCaskFixture(t)
			materials, original, err := f.install(t, 1, nil)
			if err != nil {
				t.Fatal(err)
			}
			damage(t, f, materials)
			remaining := treeBytes(t, materials.Directory)
			app, receipt := treeBytes(t, f.app()), receiptBytes(t, f.r)
			same := func(step string) {
				t.Helper()
				if treeBytes(t, f.app()) != app || !bytes.Equal(receiptBytes(t, f.r), receipt) {
					t.Fatalf("%s changed the App or the receipt", step)
				}
			}
			// Without authorization the uninstall is refused, and the refusal
			// points at the identity-bound repair.
			_, err = f.cleanup(nil)
			wantRefused(t, err, ReasonReceiptInvalid)
			if !strings.Contains(err.Error(), MaterialsUnavailableMarker) {
				t.Fatalf("refusal = %v, want the materials marker", err)
			}
			same("unauthorized uninstall")
			// Authorization alone tolerates the uninstall but cannot replace a
			// valid owner.
			outcome, err := f.cleanup(func(q *CleanupRequest) { q.AuthorizedRepair = true })
			if err != nil || outcome != OutcomeSkippedUnverified {
				t.Fatalf("unbound authorized uninstall = %q %v", outcome, err)
			}
			_, _, err = f.install(t, 2, func(q *RegisterRequest) { q.AuthorizedRepair = true })
			wantRefused(t, err, ReasonOwnedByOther)
			same("unbound authorized register")
			// Capture the identity once from the App helper.
			asApp := *f.r
			asApp.executable = func() (string, error) { return f.executable, nil }
			id, err := asApp.RepairIdentity(context.Background(), RepairIdentityRequest{Channel: caskChannel, Executable: f.executable})
			if err != nil || id != original.InstallationID {
				t.Fatalf("captured %q %v", id, err)
			}
			same("identity capture")
			f.helperCalls = nil
			outcome, err = f.cleanup(func(q *CleanupRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(id) })
			if err != nil || outcome != OutcomeSkippedUnverified {
				t.Fatalf("bound uninstall = %q %v", outcome, err)
			}
			same("bound uninstall")
			wantProgram := materials.Helper
			if name != "manifest" {
				wantProgram = f.executable // retained copy missing or unlocatable
			}
			if len(f.helperCalls) != 1 || f.helperCalls[0] != wantProgram {
				t.Fatalf("bound check ran %v, want %s", f.helperCalls, wantProgram)
			}
			repaired, result, err := f.install(t, 3, func(q *RegisterRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(id) })
			if err != nil {
				t.Fatal(err)
			}
			if result.InstallationID == id || repaired.Directory == materials.Directory || currentReceipt(t, f.r).InstallationID != result.InstallationID {
				t.Fatal("repair did not create a new attempt and installation ID")
			}
			if treeBytes(t, materials.Directory) != remaining {
				t.Fatal("the original attempt's remaining materials changed")
			}
			// The original ID's delayed cleanup cannot remove the repaired install.
			if name != "manifest" {
				_, err := f.r.Unregister(context.Background(), UnregisterRequest{Channel: caskChannel, Executable: f.executable, ExpectedInstallation: id, CleanupManifest: materials.Manifest, Remove: true, AuthorizedRepair: false})
				if err != nil || readFile(t, f.executable) != "attempt-"+repaired.AttemptID {
					t.Fatalf("delayed original cleanup: %v", err)
				}
			}
			outcome, err = f.cleanup(nil)
			if err != nil || outcome != OutcomeRemoved {
				t.Fatalf("plain uninstall after repair = %q %v", outcome, err)
			}
		})
	}
}

// TestBoundRepairRefusesSuccessor inserts a same-channel, same-path successor
// after the identity capture, before the uninstall and before the register.
func TestBoundRepairRefusesSuccessor(t *testing.T) {
	for name, damage := range materialDamage {
		for _, when := range []string{"before-uninstall", "before-register"} {
			for _, receiptDamage := range []string{"", "missing", "invalid"} {
				t.Run(name+"/"+when+"/"+receiptDamage, func(t *testing.T) {
					f := newCaskFixture(t)
					materials, original, err := f.install(t, 1, nil)
					if err != nil {
						t.Fatal(err)
					}
					damage(t, f, materials)
					id := original.InstallationID
					bind := func(q *CleanupRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(id) }
					if when == "before-register" {
						if outcome, err := f.cleanup(bind); err != nil || outcome != OutcomeSkippedUnverified {
							t.Fatalf("bound uninstall = %q %v", outcome, err)
						}
					}
					// B is published by a direct register, as a competing
					// installer would, and never through A's locator.
					source := filepath.Join(f.root, "Applications", ".AgentDeck.app.b")
					stageBundle(t, source, "successor")
					manifest := filepath.Join(f.root, "b-manifest.json")
					if _, err := f.r.Register(context.Background(), RegisterRequest{Channel: caskChannel, Executable: f.executable, Source: source, CleanupManifest: manifest}); err != nil {
						t.Fatal(err)
					}
					want := ReasonOwnedByOther
					switch receiptDamage {
					case "missing":
						removeReceipt(t, f.r)
						want = ReasonReceiptMissing
					case "invalid":
						writeRawReceipt(t, f.r, "{")
						want = ReasonReceiptInvalid
					}
					app, receipt := treeBytes(t, f.app()), receiptBytes(t, f.r)
					if when == "before-uninstall" {
						_, err := f.cleanup(bind)
						wantRefused(t, err, want)
					}
					_, _, err = f.install(t, 2, func(q *RegisterRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(id) })
					wantRefused(t, err, want)
					if treeBytes(t, f.app()) != app || !bytes.Equal(receiptBytes(t, f.r), receipt) {
						t.Fatal("the successor's App or receipt changed")
					}
				})
			}
		}
	}
}

// TestBoundCleanupRefusesSuccessorMaterials covers a successor installed
// through the cask itself: the locator now names complete materials of B.
func TestBoundCleanupRefusesSuccessorMaterials(t *testing.T) {
	f := newCaskFixture(t)
	_, original, err := f.install(t, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.install(t, 2, func(q *RegisterRequest) {
		q.AuthorizedRepair = true
		q.BindExpectedInstallation(original.InstallationID)
	}); err != nil {
		t.Fatal(err)
	}
	app, receipt := treeBytes(t, f.app()), receiptBytes(t, f.r)
	_, err = f.cleanup(func(q *CleanupRequest) {
		q.AuthorizedRepair = true
		q.BindExpectedInstallation(original.InstallationID)
	})
	wantRefused(t, err, ReasonOwnedByOther)
	if treeBytes(t, f.app()) != app || !bytes.Equal(receiptBytes(t, f.r), receipt) || len(f.helperCalls) != 0 {
		t.Fatal("a bound cleanup acted on the successor")
	}
}

// TestUnownedRepairThroughAttempts is the damaged-receipt repair, with and
// without the external materials.
func TestUnownedRepairThroughAttempts(t *testing.T) {
	for _, receiptDamage := range []string{"missing", "invalid"} {
		for _, dropMaterials := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/materials-removed=%v", receiptDamage, dropMaterials), func(t *testing.T) {
				f := newCaskFixture(t)
				materials, original, err := f.install(t, 1, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := ReasonReceiptMissing
				if receiptDamage == "missing" {
					removeReceipt(t, f.r)
				} else {
					writeRawReceipt(t, f.r, "{")
					want = ReasonReceiptInvalid
				}
				if dropMaterials {
					if err := os.RemoveAll(materials.Directory); err != nil {
						t.Fatal(err)
					}
				}
				remaining := treeBytes(t, materials.Directory)
				app, receipt := treeBytes(t, f.app()), receiptBytes(t, f.r)
				_, err = f.cleanup(nil)
				wantRefused(t, err, want)
				if strings.Contains(err.Error(), MaterialsUnavailableMarker) {
					t.Fatal("an unowned target was offered the identity-bound repair")
				}
				_, _, err = f.install(t, 2, nil)
				wantRefused(t, err, want)
				if treeBytes(t, f.app()) != app || !bytes.Equal(receiptBytes(t, f.r), receipt) {
					t.Fatal("unauthorized repair changed the App or receipt")
				}
				outcome, err := f.cleanup(func(q *CleanupRequest) { q.AuthorizedRepair = true })
				if err != nil || outcome != OutcomeSkippedUnverified || treeBytes(t, f.app()) != app {
					t.Fatalf("authorized uninstall = %q %v", outcome, err)
				}
				repaired, result, err := f.install(t, 3, func(q *RegisterRequest) { q.AuthorizedRepair = true })
				if err != nil {
					t.Fatal(err)
				}
				if result.InstallationID == original.InstallationID || readFile(t, f.executable) != "attempt-"+repaired.AttemptID {
					t.Fatal("authorized repair did not replace the App with a new installation")
				}
				if treeBytes(t, materials.Directory) != remaining {
					t.Fatal("the original attempt's remaining materials changed")
				}
				if outcome, err := f.cleanup(nil); err != nil || outcome != OutcomeRemoved {
					t.Fatalf("plain uninstall after repair = %q %v", outcome, err)
				}
			})
		}
	}
}

func TestCleanupAttemptRefusesOtherOwnersWithoutMaterials(t *testing.T) {
	f := newCaskFixture(t)
	// A make installation owns state and the cask has no locator at all.
	if err := os.MkdirAll(filepath.Join(f.root, "records"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := f.r.Register(context.Background(), fileRequest(f.root, "make", "make", t)); err != nil {
		t.Fatal(err)
	}
	receipt := receiptBytes(t, f.r)
	_, err := f.cleanup(nil)
	wantRefused(t, err, ReasonOwnedByOther)
	_, _, err = f.install(t, 1, func(q *RegisterRequest) { q.AuthorizedRepair = true })
	wantRefused(t, err, ReasonOwnedByOther)
	_, _, err = f.install(t, 2, func(q *RegisterRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation(otherID) })
	wantRefused(t, err, ReasonOwnedByOther)
	if !bytes.Equal(receiptBytes(t, f.r), receipt) || readFile(t, filepath.Join(f.root, "bin", "agentdeck")) != "make" {
		t.Fatal("the make installation changed")
	}
}

func TestLocatorMustMatchTheDeclaredCask(t *testing.T) {
	f := newCaskFixture(t)
	if _, _, err := f.install(t, 1, nil); err != nil {
		t.Fatal(err)
	}
	app := treeBytes(t, f.app())
	for name, mutate := range map[string]func(*CleanupRequest){
		"token":   func(q *CleanupRequest) { q.Token = "agentdeck-app-rc" },
		"version": func(q *CleanupRequest) { q.Version = "9.9.9" },
	} {
		if _, err := f.cleanup(mutate); err == nil || treeBytes(t, f.app()) != app {
			t.Fatalf("%s mismatch cleaned the installation: %v", name, err)
		}
	}
	if len(f.helperCalls) != 0 {
		t.Fatal("a mismatched locator reached a helper")
	}
}
