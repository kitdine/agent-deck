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

const otherID = "0123456789abcdef0123456789abcdef"

// caskRequest stages a fresh bundle for the cask target with its own manifest.
func caskRequest(t *testing.T, root, content string) RegisterRequest {
	t.Helper()
	request := bundleRequest(t, root, caskChannel, content)
	request.RequireUnowned = true
	return request
}

type repairMode struct {
	name       string
	authorized bool
	expected   func(original string) (string, bool)
}

var repairModes = []repairMode{
	{"require-unowned", false, nil},
	{"authorized", true, nil},
	{"authorized-bound", true, func(original string) (string, bool) {
		if original == "" {
			return otherID, true // nothing was ever installed to capture
		}
		return original, true
	}},
	{"authorized-other-id", true, func(string) (string, bool) { return otherID, true }},
}

func (m repairMode) apply(request *RegisterRequest, original string) {
	request.AuthorizedRepair = m.authorized
	if m.expected != nil {
		if id, ok := m.expected(original); ok {
			request.BindExpectedInstallation(id)
		}
	}
}

func treeBytes(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(&out, "%s %v", rel, info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(&out, " %q", data)
		}
		out.WriteByte('\n')
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func writeRawReceipt(t *testing.T, r *Resolver, data string) {
	t.Helper()
	home, _ := r.AccountHome()
	if err := os.MkdirAll(filepath.Dir(receiptPath(home)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath(home), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func removeReceipt(t *testing.T, r *Resolver) {
	t.Helper()
	home, _ := r.AccountHome()
	if err := os.Remove(receiptPath(home)); err != nil {
		t.Fatal(err)
	}
}

// TestRegisterRequireUnownedDecisionTable walks every row of the cask
// decision table under every authorization mode and checks target and receipt
// bytes for each refusal.
func TestRegisterRequireUnownedDecisionTable(t *testing.T) {
	type state struct {
		name string
		// prepare returns the original installation ID when one exists.
		prepare func(t *testing.T, r *Resolver, root string) string
		// want maps a mode name to the refusal reason; "" publishes and
		// "error" is a non-refusal failure.
		want map[string]string
	}
	installCask := func(t *testing.T, r *Resolver, root string) string {
		request := caskRequest(t, root, "original")
		result, err := r.Register(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		return result.InstallationID
	}
	all := func(reason string) map[string]string {
		return map[string]string{"require-unowned": reason, "authorized": reason, "authorized-bound": reason, "authorized-other-id": reason}
	}
	unownedOccupied := func(reason string) map[string]string {
		return map[string]string{"require-unowned": reason, "authorized": "", "authorized-bound": reason, "authorized-other-id": reason}
	}
	states := []state{
		{"valid-original-cask-owner", installCask, map[string]string{"require-unowned": ReasonOwnedByOther, "authorized": ReasonOwnedByOther, "authorized-bound": "", "authorized-other-id": ReasonOwnedByOther}},
		{"valid-owner-other-channel-same-path", func(t *testing.T, r *Resolver, root string) string {
			request := bundleRequest(t, root, "local_app", "original")
			result, err := r.Register(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			return result.InstallationID
		}, all(ReasonOwnedByOther)},
		{"valid-owner-other-path", func(t *testing.T, r *Resolver, root string) string {
			result, err := r.Register(context.Background(), fileRequest(root, "make", "make", t))
			if err != nil {
				t.Fatal(err)
			}
			return result.InstallationID
		}, all(ReasonOwnedByOther)},
		{"no-receipt-no-target", func(*testing.T, *Resolver, string) string { return "" }, all("")},
		{"stale-receipt-no-target", func(t *testing.T, r *Resolver, root string) string {
			request := fileRequest(root, "homebrew_formula", "keg", t)
			result, err := r.Register(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(request.Executable); err != nil {
				t.Fatal(err)
			}
			return result.InstallationID
		}, all("")},
		{"missing-receipt-existing-target", func(t *testing.T, r *Resolver, root string) string {
			id := installCask(t, r, root)
			removeReceipt(t, r)
			return id
		}, unownedOccupied(ReasonReceiptMissing)},
		{"stale-receipt-existing-target", func(t *testing.T, r *Resolver, root string) string {
			id := installCask(t, r, root)
			// The receipt names a program that no longer exists, while the
			// bundle root is still occupied.
			if err := os.Remove(filepath.Join(root, "Applications", "AgentDeck.app", bundleHelper)); err != nil {
				t.Fatal(err)
			}
			return id
		}, unownedOccupied(ReasonReceiptMissing)},
		{"invalid-receipt-existing-target", func(t *testing.T, r *Resolver, root string) string {
			id := installCask(t, r, root)
			writeRawReceipt(t, r, "{not a receipt")
			return id
		}, unownedOccupied(ReasonReceiptInvalid)},
		{"unowned-symlink-target", func(t *testing.T, r *Resolver, root string) string {
			elsewhere := filepath.Join(root, "Elsewhere.app")
			stageBundle(t, elsewhere, "elsewhere")
			if err := os.MkdirAll(filepath.Join(root, "Applications"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, filepath.Join(root, "Applications", "AgentDeck.app")); err != nil {
				t.Fatal(err)
			}
			return ""
		}, all("error")},
		{"unowned-file-target", func(t *testing.T, r *Resolver, root string) string {
			if err := os.MkdirAll(filepath.Join(root, "Applications"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "Applications", "AgentDeck.app"), []byte("file"), 0o600); err != nil {
				t.Fatal(err)
			}
			return ""
		}, all("error")},
	}
	for _, state := range states {
		for _, mode := range repairModes {
			t.Run(state.name+"/"+mode.name, func(t *testing.T) {
				r, root := installFixture(t)
				original := state.prepare(t, r, root)
				target := filepath.Join(root, "Applications", "AgentDeck.app")
				elsewhere := treeBytes(t, filepath.Join(root, "Elsewhere.app"))
				beforeTarget, beforeReceipt := treeBytes(t, target), receiptBytes(t, r)
				request := caskRequest(t, root, "repair")
				mode.apply(&request, original)
				result, err := r.Register(context.Background(), request)
				want := state.want[mode.name]
				if want == "" {
					if err != nil {
						t.Fatalf("register: %v", err)
					}
					receipt := currentReceipt(t, r)
					if receipt.InstallationID != result.InstallationID || receipt.InstallationID == original || receipt.Channel != caskChannel || readFile(t, request.Executable) != "repair" {
						t.Fatalf("receipt = %+v, original %s", receipt, original)
					}
					manifest, err := readManifest(request.CleanupManifest)
					if err != nil || manifest.InstallationID != result.InstallationID {
						t.Fatalf("manifest = %+v %v", manifest, err)
					}
					if _, err := os.Lstat(request.Source); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("replaced object was not removed after commit")
					}
					return
				}
				if want == "error" {
					var refusal *InstallError
					if err == nil || errors.As(err, &refusal) {
						t.Fatalf("err = %v, want a type-check failure", err)
					}
				} else {
					wantRefused(t, err, want)
				}
				if got := treeBytes(t, target); got != beforeTarget {
					t.Fatalf("target changed:\n%s\nwant:\n%s", got, beforeTarget)
				}
				if got := receiptBytes(t, r); !bytes.Equal(got, beforeReceipt) {
					t.Fatalf("receipt changed: %s", got)
				}
				if got := treeBytes(t, filepath.Join(root, "Elsewhere.app")); got != elsewhere {
					t.Fatal("symlink destination changed")
				}
				if _, err := os.Lstat(request.CleanupManifest); want != "error" && !errors.Is(err, os.ErrNotExist) {
					t.Fatal("a refused register left a cleanup manifest")
				}
			})
		}
	}
}

func TestRegisterRepairFlagValidation(t *testing.T) {
	r, root := installFixture(t)
	for name, mutate := range map[string]func(*RegisterRequest){
		"authorized-without-require-unowned": func(q *RegisterRequest) { q.RequireUnowned, q.AuthorizedRepair = false, true },
		"expected-without-authorization":     func(q *RegisterRequest) { q.BindExpectedInstallation(otherID) },
		"empty-expected":                     func(q *RegisterRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation("") },
		"malformed-expected":                 func(q *RegisterRequest) { q.AuthorizedRepair = true; q.BindExpectedInstallation("ABC") },
		"takeover":                           func(q *RegisterRequest) { q.Takeover = true },
		"no-source":                          func(q *RegisterRequest) { q.Source, q.CleanupManifest = "", "" },
	} {
		t.Run(name, func(t *testing.T) {
			request := caskRequest(t, root, "flags-"+name)
			mutate(&request)
			var refusal *InstallError
			if _, err := r.Register(context.Background(), request); err == nil || errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a usage failure", err)
			}
			if receiptBytes(t, r) != nil {
				t.Fatal("receipt written")
			}
			if _, err := os.Lstat(filepath.Join(root, "Applications", "AgentDeck.app")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("target published")
			}
		})
	}
}

// TestAuthorizedReplaceFailureRestores injects a failure after the swap and
// before the receipt commit for both authorized replacement rows.
func TestAuthorizedReplaceFailureRestores(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(fmt.Sprintf("bound=%v", bound), func(t *testing.T) {
			r, root := installFixture(t)
			original, err := r.Register(context.Background(), caskRequest(t, root, "original"))
			if err != nil {
				t.Fatal(err)
			}
			if !bound {
				removeReceipt(t, r)
			}
			target := filepath.Join(root, "Applications", "AgentDeck.app")
			beforeTarget, beforeReceipt := treeBytes(t, target), receiptBytes(t, r)
			request := caskRequest(t, root, "repair")
			request.AuthorizedRepair = true
			if bound {
				request.BindExpectedInstallation(original.InstallationID)
			}
			installHook = func(point string) error {
				if point == "register_published" {
					if readFile(t, request.Executable) != "repair" {
						t.Error("hook ran before the swap")
					}
					return errors.New("injected")
				}
				return nil
			}
			defer func() { installHook = nil }()
			if _, err := r.Register(context.Background(), request); err == nil {
				t.Fatal("register succeeded")
			}
			if treeBytes(t, target) != beforeTarget || !bytes.Equal(receiptBytes(t, r), beforeReceipt) {
				t.Fatal("original App or receipt not restored")
			}
		})
	}
}

func TestRepairIdentity(t *testing.T) {
	r, root := installFixture(t)
	request := caskRequest(t, root, "original")
	original, err := r.Register(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	helper := request.Executable
	asHelper := *r
	asHelper.executable = func() (string, error) { return helper, nil }
	identity := RepairIdentityRequest{Channel: caskChannel, Executable: helper}
	before := treeBytes(t, root)
	id, err := asHelper.RepairIdentity(context.Background(), identity)
	if err != nil || id != original.InstallationID {
		t.Fatalf("id = %q %v", id, err)
	}
	// Run by any other program, the fixed path is not the caller's identity.
	putExecutable(t, filepath.Join(root, "bin", "agentdeck"))
	before = treeBytes(t, root)
	if _, err := r.RepairIdentity(context.Background(), identity); err == nil {
		t.Fatal("another program captured the identity")
	} else {
		wantRefused(t, err, ReasonOwnedByOther)
	}
	if _, err := asHelper.RepairIdentity(context.Background(), RepairIdentityRequest{Channel: "local_app", Executable: helper}); err == nil {
		t.Fatal("non-cask channel accepted")
	}
	if got := treeBytes(t, root); got != before {
		t.Fatal("repair-identity wrote something")
	}
	held, err := r.exclusiveLock(context.Background(), filepath.Join(root, "account"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = asHelper.RepairIdentity(ctx, identity)
	held.Release()
	if err == nil {
		t.Fatal("identity captured while the lock was held")
	}
	writeRawReceipt(t, r, "{")
	_, err = asHelper.RepairIdentity(context.Background(), identity)
	wantRefused(t, err, ReasonReceiptInvalid)
	removeReceipt(t, r)
	_, err = asHelper.RepairIdentity(context.Background(), identity)
	wantRefused(t, err, ReasonReceiptMissing)
	// A receipt for another channel at the same path is not this installation.
	local := bundleRequest(t, root, "local_app", "local")
	local.Takeover = true
	if err := os.RemoveAll(filepath.Join(root, "Applications", "AgentDeck.app")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	_, err = asHelper.RepairIdentity(context.Background(), identity)
	wantRefused(t, err, ReasonOwnedByOther)
}

func TestUnregisterAuthorizedRepairTolerance(t *testing.T) {
	type fixtureState struct {
		r        *Resolver
		root     string
		request  RegisterRequest
		original RegisterResult
	}
	setup := func(t *testing.T) fixtureState {
		r, root := installFixture(t)
		old := deregisterWidgets
		deregisterWidgets = func(string) error { return nil }
		t.Cleanup(func() { deregisterWidgets = old })
		request := caskRequest(t, root, "original")
		original, err := r.Register(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		return fixtureState{r, root, request, original}
	}
	unregister := func(f fixtureState, id, manifest string, authorized bool) (UnregisterResult, error) {
		return f.r.Unregister(context.Background(), UnregisterRequest{Channel: caskChannel, Executable: f.request.Executable, ExpectedInstallation: id, CleanupManifest: manifest, Remove: true, AuthorizedRepair: authorized})
	}
	target := func(f fixtureState) string { return filepath.Join(f.root, "Applications", "AgentDeck.app") }
	unchanged := func(t *testing.T, f fixtureState, beforeTarget string, beforeReceipt []byte) {
		t.Helper()
		if treeBytes(t, target(f)) != beforeTarget || !bytes.Equal(receiptBytes(t, f.r), beforeReceipt) {
			t.Fatal("App or receipt changed")
		}
	}
	t.Run("missing-or-invalid-receipt-is-tolerated-without-removal", func(t *testing.T) {
		for _, damage := range []func(*testing.T, fixtureState){
			func(t *testing.T, f fixtureState) { removeReceipt(t, f.r) },
			func(t *testing.T, f fixtureState) { writeRawReceipt(t, f.r, "{") },
		} {
			f := setup(t)
			damage(t, f)
			beforeTarget, beforeReceipt := treeBytes(t, target(f)), receiptBytes(t, f.r)
			if _, err := unregister(f, f.original.InstallationID, f.request.CleanupManifest, false); err == nil {
				t.Fatal("unauthorized cleanup of an unproven installation succeeded")
			}
			result, err := unregister(f, f.original.InstallationID, f.request.CleanupManifest, true)
			if err != nil || result.Outcome != OutcomeSkippedUnverified {
				t.Fatalf("authorized = %+v %v", result, err)
			}
			unchanged(t, f, beforeTarget, beforeReceipt)
		}
	})
	t.Run("missing-or-invalid-manifest-is-tolerated-without-removal", func(t *testing.T) {
		for _, damage := range []func(*testing.T, string){
			func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		} {
			f := setup(t)
			damage(t, f.request.CleanupManifest)
			beforeTarget, beforeReceipt := treeBytes(t, target(f)), receiptBytes(t, f.r)
			if _, err := unregister(f, f.original.InstallationID, f.request.CleanupManifest, false); err == nil {
				t.Fatal("unauthorized cleanup without a manifest succeeded")
			}
			result, err := unregister(f, f.original.InstallationID, f.request.CleanupManifest, true)
			if err != nil || result.Outcome != OutcomeSkippedUnverified {
				t.Fatalf("authorized = %+v %v", result, err)
			}
			unchanged(t, f, beforeTarget, beforeReceipt)
		}
	})
	t.Run("existing-semantics-are-kept", func(t *testing.T) {
		f := setup(t)
		// A manifest that disagrees with the command stays a refusal.
		_, err := f.r.Unregister(context.Background(), UnregisterRequest{Channel: caskChannel, Executable: f.request.Executable, ExpectedInstallation: otherID, CleanupManifest: f.request.CleanupManifest, Remove: true, AuthorizedRepair: true})
		wantRefused(t, err, ReasonReceiptInvalid)
		// A successor keeps skipped_superseded for the old manifest.
		successor := caskRequest(t, f.root, "successor")
		successor.AuthorizedRepair = true
		successor.BindExpectedInstallation(f.original.InstallationID)
		if _, err := f.r.Register(context.Background(), successor); err != nil {
			t.Fatal(err)
		}
		result, err := unregister(f, f.original.InstallationID, f.request.CleanupManifest, true)
		if err != nil || result.Outcome != OutcomeSkippedSuperseded || readFile(t, f.request.Executable) != "successor" {
			t.Fatalf("delayed cleanup = %+v %v", result, err)
		}
		// With the lock held it is still owner_running.
		held, err := f.r.exclusiveLock(context.Background(), filepath.Join(f.root, "account"))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = f.r.Unregister(ctx, UnregisterRequest{Channel: caskChannel, Executable: f.request.Executable, ExpectedInstallation: f.original.InstallationID, CleanupManifest: f.request.CleanupManifest, Remove: true, AuthorizedRepair: true})
		held.Release()
		if err == nil {
			t.Fatal("cleanup ran while the lock was held")
		}
		// A matching receipt and manifest is the normal removal.
		manifest, err := readManifest(successor.CleanupManifest)
		if err != nil {
			t.Fatal(err)
		}
		result, err = unregister(f, manifest.InstallationID, successor.CleanupManifest, true)
		if err != nil || result.Outcome != OutcomeRemoved {
			t.Fatalf("matching cleanup = %+v %v", result, err)
		}
	})
	t.Run("bound-check-without-materials", func(t *testing.T) {
		f := setup(t)
		beforeTarget, beforeReceipt := treeBytes(t, target(f)), receiptBytes(t, f.r)
		// An empty manifest is only the bound check, never an unauthorized call.
		if _, err := unregister(f, f.original.InstallationID, "", false); err == nil {
			t.Fatal("manifest-less cleanup accepted without authorization")
		}
		result, err := unregister(f, f.original.InstallationID, "", true)
		if err != nil || result.Outcome != OutcomeSkippedUnverified {
			t.Fatalf("bound check = %+v %v", result, err)
		}
		unchanged(t, f, beforeTarget, beforeReceipt)
		_, err = unregister(f, otherID, "", true)
		wantRefused(t, err, ReasonOwnedByOther)
		unchanged(t, f, beforeTarget, beforeReceipt)
		// A same-channel, same-path successor is still another identity.
		successor := caskRequest(t, f.root, "successor")
		successor.AuthorizedRepair = true
		successor.BindExpectedInstallation(f.original.InstallationID)
		if _, err := f.r.Register(context.Background(), successor); err != nil {
			t.Fatal(err)
		}
		beforeTarget, beforeReceipt = treeBytes(t, target(f)), receiptBytes(t, f.r)
		_, err = unregister(f, f.original.InstallationID, "", true)
		wantRefused(t, err, ReasonOwnedByOther)
		unchanged(t, f, beforeTarget, beforeReceipt)
		// The successor's receipt going missing or invalid never downgrades.
		writeRawReceipt(t, f.r, "{")
		_, err = unregister(f, f.original.InstallationID, "", true)
		wantRefused(t, err, ReasonReceiptInvalid)
		removeReceipt(t, f.r)
		_, err = unregister(f, f.original.InstallationID, "", true)
		wantRefused(t, err, ReasonReceiptMissing)
		if treeBytes(t, target(f)) != beforeTarget {
			t.Fatal("successor App changed")
		}
		if err := os.RemoveAll(target(f)); err != nil {
			t.Fatal(err)
		}
		result, err = unregister(f, f.original.InstallationID, "", true)
		if err != nil || result.Outcome != OutcomeAbsent {
			t.Fatalf("already-uninstalled bound check = %+v %v", result, err)
		}
	})
}

func TestUnregisterRequiresIdentityUnderLock(t *testing.T) {
	r, root := installFixture(t)
	original := caskRequest(t, root, "original")
	installed, err := r.Register(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	request := UnregisterRequest{
		Channel: caskChannel, Executable: original.Executable,
		ExpectedInstallation: installed.InstallationID, CleanupManifest: original.CleanupManifest,
		Remove: true, AuthorizedRepair: true, RequireIdentity: true,
	}
	before := treeBytes(t, root)
	invalid := request
	invalid.AuthorizedRepair = false
	if _, err := r.Unregister(context.Background(), invalid); err == nil {
		t.Fatal("strict identity accepted without repair authorization")
	}
	invalid = request
	invalid.CleanupManifest = "relative.json"
	if _, err := r.Unregister(context.Background(), invalid); err == nil {
		t.Fatal("strict identity accepted a relative manifest")
	}
	if treeBytes(t, root) != before {
		t.Fatal("invalid arguments changed the installation")
	}
	// Change the receipt at the existing locked hook. Validation performed
	// before lock acquisition would miss this change and tolerate the damage.
	priorHook := installHook
	t.Cleanup(func() { installHook = priorHook })
	installHook = func(point string) error {
		if point == "unregister_locked" {
			writeRawReceipt(t, r, "{")
		}
		return nil
	}
	appBefore := treeBytes(t, filepath.Join(root, "Applications"))
	_, err = r.Unregister(context.Background(), request)
	wantRefused(t, err, ReasonReceiptInvalid)
	if treeBytes(t, filepath.Join(root, "Applications")) != appBefore {
		t.Fatal("strict identity cleanup changed the App")
	}
}
