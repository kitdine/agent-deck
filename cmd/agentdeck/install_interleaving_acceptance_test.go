//go:build agentdeck_acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/ownershiptest"
)

// These interleavings run the real install register/unregister commands in
// separate processes, synchronized by the acceptance-only install hook, against
// real Task 2 ownership sessions. Every account path is the fixture's.

type installChild struct {
	command        *exec.Cmd
	stdout, stderr bytes.Buffer
	done           chan struct{}
	started        time.Time
}

func startInstallChild(t *testing.T, f ownershiptest.Fixture, executable, hook string, args ...string) *installChild {
	t.Helper()
	command := ownershipChild(f, append([]string{"--format", "json"}, args...), false)
	command.Path = executable
	command.Args[0] = executable
	if hook != "" {
		command.Env = append(command.Env, "AGENTDECK_ACCEPTANCE_INSTALL_HOOK="+hook)
	}
	c := &installChild{command: command, done: make(chan struct{}), started: time.Now()}
	command.Stdout, command.Stderr = &c.stdout, &c.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = command.Wait(); close(c.done) }()
	t.Cleanup(func() {
		select {
		case <-c.done:
		default:
			_ = command.Process.Signal(syscall.SIGCONT)
			_ = command.Process.Kill()
			<-c.done
		}
	})
	return c
}

func (c *installChild) finish(t *testing.T) int {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(90 * time.Second):
		t.Fatalf("install child did not finish: %s", c.stderr.String())
	}
	return c.command.ProcessState.ExitCode()
}

func (c *installChild) running() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

func (c *installChild) registeredID(t *testing.T) string {
	t.Helper()
	var envelope struct {
		Data struct {
			InstallationID string `json:"installation_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(c.stdout.Bytes(), &envelope); err != nil || envelope.Data.InstallationID == "" {
		t.Fatalf("no registration result: %v %s %s", err, c.stdout.String(), c.stderr.String())
	}
	return envelope.Data.InstallationID
}

func (c *installChild) expect(t *testing.T, code int, fragment string) {
	t.Helper()
	if got := c.finish(t); got != code || !strings.Contains(c.stdout.String()+c.stderr.String(), fragment) {
		t.Fatalf("exit=%d want=%d (%q): stdout=%s stderr=%s", got, code, fragment, c.stdout.String(), c.stderr.String())
	}
}

type installPause struct{ dir string }

func newInstallPause(t *testing.T, point string) (installPause, string) {
	t.Helper()
	dir := t.TempDir()
	return installPause{dir}, point + "=wait:" + dir
}

func (p installPause) reached(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(p.dir, "reached")); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("install child never reached its pause point")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (p installPause) release(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.dir, "release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// copyProgram makes a distinct file (not a hard link) of the test executable:
// rename between two links of one inode is a no-op and would hide swaps.
func copyProgram(t *testing.T, f ownershiptest.Fixture, rel string) string {
	t.Helper()
	path := filepath.Join(f.Base, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func stagedProgram(t *testing.T, f ownershiptest.Fixture, dir, name string) string {
	t.Helper()
	return copyProgram(t, f, filepath.Join(dir, name))
}

func registerArgs(channel, executable, source, manifest string, extra ...string) []string {
	return append([]string{"install", "register", "--channel", channel, "--executable", executable, "--source", source, "--cleanup-manifest", manifest}, extra...)
}

func receiptSnapshot(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME"), "Library", "Application Support", "AgentDeck", "installation.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return data
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	left, err := os.Lstat(a)
	if err != nil {
		return false
	}
	right, err := os.Lstat(b)
	return err == nil && os.SameFile(left, right)
}

func TestInstallInterleavingI1PublishAndRegister(t *testing.T) {
	f := ownershiptest.New(t)
	seedOwnedState(t, f)
	target := copyProgram(t, f, "install/agentdeck")
	before := registerOwnershipFixture(t, f, target, "make")
	held := startHeldOwnership(t, f, target, "hold-rw")
	source := stagedProgram(t, f, "install", ".agentdeck.staged")
	installer := source
	receipt := receiptSnapshot(t)

	refusedRun := startInstallChild(t, f, installer, "", registerArgs("make", target, source, filepath.Join(f.Base, "records-1.json"))...)
	refusedRun.expect(t, 1, "owner_running")
	if !bytes.Equal(receipt, receiptSnapshot(t)) || sameFile(t, target, source) || !fileExists(source) || fileExists(filepath.Join(f.Base, "records-1.json")) {
		t.Fatal("refused registration changed the target, receipt, source or manifest")
	}
	if out := held.finish(t, 0); !strings.Contains(out, "complete") {
		t.Fatalf("owner session did not complete: %s", out)
	}

	pause, hook := newInstallPause(t, "register_published")
	retry := startInstallChild(t, f, installer, hook, registerArgs("make", target, source, filepath.Join(f.Base, "records-2.json"))...)
	pause.reached(t)
	// A shared session started during publication waits for the exclusive
	// lock; it never observes the published-but-unrecorded state.
	waiting := ownershipChild(f, []string{"--state-dir", f.Real, "--format", "json", "provider", "list"}, false)
	waiting.Path, waiting.Args[0] = target, target
	var waitingOut bytes.Buffer
	waiting.Stdout, waiting.Stderr = &waitingOut, &waitingOut
	if err := waiting.Start(); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- waiting.Wait() }()
	time.Sleep(time.Second)
	select {
	case err := <-waitDone:
		t.Fatalf("shared session ran during publication: %v %s", err, waitingOut.String())
	default:
	}
	pause.release(t)
	retry.expect(t, 0, `"installation_id"`)
	if err := <-waitDone; err != nil || !strings.Contains(waitingOut.String(), "existing-owner-data") {
		t.Fatalf("session after publication = %v %s", err, waitingOut.String())
	}
	after, err := installation.DefaultResolver().ReadReceipt()
	if err != nil || after.InstallationID != retry.registeredID(t) || after.InstallationID == before.InstallationID || after.Executable != target {
		t.Fatalf("receipt after publication = %+v %v", after, err)
	}

	// Failure injection between publication and the receipt restores both.
	next := stagedProgram(t, f, "install", ".agentdeck.next")
	published, _ := os.Lstat(target)
	receipt = receiptSnapshot(t)
	failed := startInstallChild(t, f, next, "register_published=fail", registerArgs("make", target, next, filepath.Join(f.Base, "records-3.json"))...)
	failed.expect(t, 1, "injected failure")
	current, _ := os.Lstat(target)
	if !os.SameFile(published, current) || !bytes.Equal(receipt, receiptSnapshot(t)) || !fileExists(next) {
		t.Fatal("failed publication did not restore target, receipt and source")
	}
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestInstallInterleavingI2SamePathCompetition(t *testing.T) {
	setup := func(t *testing.T) (ownershiptest.Fixture, string, string, string) {
		f := ownershiptest.New(t)
		t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
		target := filepath.Join(f.Base, "install", "agentdeck")
		a := stagedProgram(t, f, "install", ".agentdeck.a")
		b := stagedProgram(t, f, "install", ".agentdeck.b")
		return f, target, a, b
	}
	t.Run("same-channel-waits-then-replaces", func(t *testing.T) {
		f, target, a, b := setup(t)
		pause, hook := newInstallPause(t, "register_published")
		first := startInstallChild(t, f, a, hook, registerArgs("make", target, a, filepath.Join(f.Base, "a.json"))...)
		pause.reached(t)
		second := startInstallChild(t, f, b, "", registerArgs("make", target, b, filepath.Join(f.Base, "b.json"))...)
		time.Sleep(time.Second)
		if !second.running() || !fileExists(b) || fileExists(filepath.Join(f.Base, "b.json")) {
			t.Fatalf("second installer touched installation objects while the first held the lock: %s", second.stderr.String())
		}
		pause.release(t)
		first.expect(t, 0, `"installation_id"`)
		second.expect(t, 0, `"installation_id"`)
		receipt, err := installation.DefaultResolver().ReadReceipt()
		if err != nil || receipt.InstallationID != second.registeredID(t) || receipt.InstallationID == first.registeredID(t) {
			t.Fatalf("final receipt = %+v %v", receipt, err)
		}
		bInfo := mustLstat(t, target)
		if fileExists(b) || fileExists(a) || bInfo.Mode()&os.ModeSymlink != 0 {
			t.Fatal("staging residue remained")
		}
	})
	t.Run("shared-manifest-refused", func(t *testing.T) {
		f, target, a, b := setup(t)
		manifest := filepath.Join(f.Base, "shared.json")
		first := startInstallChild(t, f, a, "", registerArgs("make", target, a, manifest)...)
		first.expect(t, 0, `"installation_id"`)
		manifestBefore, _ := os.ReadFile(manifest)
		receipt := receiptSnapshot(t)
		published := mustLstat(t, target)
		second := startInstallChild(t, f, b, "", registerArgs("make", target, b, manifest)...)
		second.expect(t, 1, "cleanup manifest")
		manifestAfter, _ := os.ReadFile(manifest)
		if !bytes.Equal(manifestBefore, manifestAfter) || !bytes.Equal(receipt, receiptSnapshot(t)) || !os.SameFile(published, mustLstat(t, target)) || !fileExists(b) {
			t.Fatal("shared manifest changed the manifest, receipt or target")
		}
	})
	t.Run("different-channel-owned-by-other", func(t *testing.T) {
		f, target, a, b := setup(t)
		first := startInstallChild(t, f, a, "", registerArgs("make", target, a, filepath.Join(f.Base, "a.json"))...)
		first.expect(t, 0, `"installation_id"`)
		receipt := receiptSnapshot(t)
		targetBytes, _ := os.ReadFile(target)
		second := startInstallChild(t, f, b, "", registerArgs("local_app", target, b, filepath.Join(f.Base, "b.json"))...)
		second.expect(t, 1, "owned_by_other")
		after, _ := os.ReadFile(target)
		if !bytes.Equal(receipt, receiptSnapshot(t)) || !bytes.Equal(targetBytes, after) || fileExists(filepath.Join(f.Base, "b.json")) {
			t.Fatal("different-channel registration changed the target or receipt")
		}
	})
}

func mustLstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestInstallInterleavingI3PauseAndCrash(t *testing.T) {
	t.Run("stopped-inside-lock", func(t *testing.T) {
		f := ownershiptest.New(t)
		seedOwnedState(t, f)
		owner := ownershipExecutable(t, f, "owner/agentdeck")
		registerOwnershipFixture(t, f, owner, "make")
		target := filepath.Join(f.Base, "install", "agentdeck")
		a := stagedProgram(t, f, "install", ".agentdeck.a")
		b := stagedProgram(t, f, "install", ".agentdeck.b")
		pause, hook := newInstallPause(t, "register_manifest")
		first := startInstallChild(t, f, a, hook, registerArgs("make", target, a, filepath.Join(f.Base, "a.json"), "--takeover")...)
		pause.reached(t)
		if err := first.command.Process.Signal(syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		pause.release(t)
		// No lease expires: however long A stays stopped, B neither registers
		// nor reads state, and nobody publishes or reads a half-published P.
		second := startInstallChild(t, f, b, "", registerArgs("make", target, b, filepath.Join(f.Base, "b.json"), "--takeover")...)
		second.expect(t, 1, "owner_running")
		code, out := runOwnedCommand(t, f, owner, "--state-dir", f.Real, "provider", "list")
		if code != 1 || !strings.Contains(out, "ownership_busy") {
			t.Fatalf("state access while installer stopped = %d %s", code, out)
		}
		if fileExists(target) || !fileExists(a) || !fileExists(b) {
			t.Fatal("a stopped installer's target was published or read")
		}
		if err := first.command.Process.Signal(syscall.SIGCONT); err != nil {
			t.Fatal(err)
		}
		first.expect(t, 0, `"installation_id"`)
		receipt, err := installation.DefaultResolver().ReadReceipt()
		if err != nil || receipt.InstallationID != first.registeredID(t) || !fileExists(target) {
			t.Fatalf("resumed installer did not finish in one lock = %+v %v", receipt, err)
		}
	})
	t.Run("stopped-before-lock", func(t *testing.T) {
		for _, bSessionEnds := range []bool{false, true} {
			f := ownershiptest.New(t)
			seedOwnedState(t, f)
			target := filepath.Join(f.Base, "install", "agentdeck")
			a := stagedProgram(t, f, "install", ".agentdeck.a")
			b := stagedProgram(t, f, "install", ".agentdeck.b")
			t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
			pause, hook := newInstallPause(t, "register_unlocked")
			first := startInstallChild(t, f, a, hook, registerArgs("make", target, a, filepath.Join(f.Base, "a.json"))...)
			pause.reached(t)
			second := startInstallChild(t, f, b, "", registerArgs("make", target, b, filepath.Join(f.Base, "b.json"))...)
			second.expect(t, 0, `"installation_id"`)
			session := startHeldOwnership(t, f, target, "hold-rw")
			receipt := receiptSnapshot(t)
			published := mustLstat(t, target)
			pause.release(t)
			if bSessionEnds {
				time.Sleep(time.Second)
				if !first.running() {
					t.Fatal("resumed installer did not wait for the session")
				}
				session.finish(t, 0)
				first.expect(t, 0, `"installation_id"`)
				continue
			}
			first.expect(t, 1, "owner_running")
			if !bytes.Equal(receipt, receiptSnapshot(t)) || !os.SameFile(published, mustLstat(t, target)) {
				t.Fatal("waiting installer changed P or the receipt")
			}
			if out := session.finish(t, 0); !strings.Contains(out, "complete") {
				t.Fatalf("B's session did not complete: %s", out)
			}
		}
	})
	t.Run("crash-inside-lock", func(t *testing.T) {
		f := ownershiptest.New(t)
		t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
		target := filepath.Join(f.Base, "install", "agentdeck")
		old := stagedProgram(t, f, "install", ".agentdeck.old")
		startInstallChild(t, f, old, "", registerArgs("make", target, old, filepath.Join(f.Base, "old.json"))...).expect(t, 0, `"installation_id"`)
		oldInfo := mustLstat(t, target)
		receipt := receiptSnapshot(t)
		a := stagedProgram(t, f, "install", ".agentdeck.a")
		aInfo := mustLstat(t, a)
		pause, hook := newInstallPause(t, "register_published")
		first := startInstallChild(t, f, a, hook, registerArgs("make", target, a, filepath.Join(f.Base, "a.json"))...)
		pause.reached(t)
		if err := first.command.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		first.finish(t)
		current := mustLstat(t, target)
		if !os.SameFile(current, oldInfo) && !os.SameFile(current, aInfo) {
			t.Fatal("crash left a target that is neither complete version")
		}
		if !bytes.Equal(receipt, receiptSnapshot(t)) {
			t.Fatal("crash before the receipt changed it")
		}
		b := stagedProgram(t, f, "install", ".agentdeck.b")
		startInstallChild(t, f, b, "", registerArgs("make", target, b, filepath.Join(f.Base, "b.json"))...).expect(t, 0, `"installation_id"`)
	})
}

func stageAppBundle(t *testing.T, f ownershiptest.Fixture, name string) string {
	t.Helper()
	bundle := filepath.Join(f.Base, "Applications", name)
	copyProgram(t, f, filepath.Join("Applications", name, "Contents", "Helpers", "agentdeck"))
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestInstallInterleavingI4DelayedCompleteCleanup(t *testing.T) {
	for _, channel := range []string{"local_app", "homebrew_cask"} {
		t.Run(channel, func(t *testing.T) {
			f := ownershiptest.New(t)
			seedOwnedState(t, f)
			t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
			helper := filepath.Join(f.Base, "Applications", "AgentDeck.app", "Contents", "Helpers", "agentdeck")
			aSource := stageAppBundle(t, f, ".AgentDeck-a.app")
			aManifest := filepath.Join(f.Base, "records", "a.json")
			if err := os.MkdirAll(filepath.Dir(aManifest), 0o700); err != nil {
				t.Fatal(err)
			}
			a := startInstallChild(t, f, filepath.Join(aSource, "Contents", "Helpers", "agentdeck"), "", registerArgs(channel, helper, aSource, aManifest)...)
			a.expect(t, 0, `"installation_id"`)
			aID := a.registeredID(t)
			// A's preserved cleanup helper lives outside the final App.
			cleanup := ownershipExecutable(t, f, "records/cleanup-helper")
			unregister := []string{"install", "unregister", "--channel", channel, "--executable", helper, "--expected-installation", aID, "--cleanup-manifest", aManifest, "--remove"}
			startInstallChild(t, f, cleanup, "", unregister...).expect(t, 0, `"removed"`)
			if fileExists(filepath.Dir(filepath.Dir(filepath.Dir(helper)))) {
				t.Fatal("A's cleanup left its App")
			}

			// Same path, same channel, same version: only the ID differs.
			bSource := stageAppBundle(t, f, ".AgentDeck-b.app")
			b := startInstallChild(t, f, filepath.Join(bSource, "Contents", "Helpers", "agentdeck"), "", registerArgs(channel, helper, bSource, filepath.Join(f.Base, "records", "b.json"))...)
			b.expect(t, 0, `"installation_id"`)
			session := startHeldOwnership(t, f, helper, "hold-rw")
			receipt := receiptSnapshot(t)
			app := mustLstat(t, helper)

			// A's remaining uninstall, rollback and retry steps all carry A's ID.
			for attempt := 0; attempt < 2; attempt++ {
				startInstallChild(t, f, cleanup, "", unregister...).expect(t, 1, "owner_running")
			}
			if !bytes.Equal(receipt, receiptSnapshot(t)) || !os.SameFile(app, mustLstat(t, helper)) {
				t.Fatal("A's delayed cleanup changed B while B's session was open")
			}
			if out := session.finish(t, 0); !strings.Contains(out, "complete") {
				t.Fatalf("B's session was disturbed: %s", out)
			}
			startInstallChild(t, f, cleanup, "", unregister...).expect(t, 0, `"skipped_superseded"`)
			if !bytes.Equal(receipt, receiptSnapshot(t)) || !os.SameFile(app, mustLstat(t, helper)) {
				t.Fatal("A's delayed cleanup removed B")
			}
			code, out := runOwnedCommand(t, f, helper, "--state-dir", f.Real, "--format", "json", "provider", "list")
			if code != 0 || !strings.Contains(out, "existing-owner-data") {
				t.Fatalf("B lost ownership: %d %s", code, out)
			}
		})
	}
}

// caskAttempt drives the cask's own command sequence for one attempt.
type caskAttempt struct {
	id, source, helper, manifest, directory string
}

const (
	i5Token   = "agentdeck-app"
	i5Version = "1.2.3"
)

func caskPaths(f ownershiptest.Fixture) (helper, locator string) {
	return filepath.Join(f.Base, "Applications", "AgentDeck.app", "Contents", "Helpers", "agentdeck"), filepath.Join(f.Base, "Caskroom", i5Token, i5Version, installation.AttemptLocatorName)
}

// prepareCaskAttempt stages a bundle and creates its attempt materials with
// the staged helper, as the cask's preflight does.
func prepareCaskAttempt(t *testing.T, f ownershiptest.Fixture, n int) caskAttempt {
	t.Helper()
	helper, locator := caskPaths(f)
	if err := os.MkdirAll(filepath.Dir(locator), 0o700); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("0", 31) + string(rune('0'+n))
	source := stageAppBundle(t, f, ".AgentDeck.app."+id)
	child := startInstallChild(t, f, filepath.Join(source, "Contents", "Helpers", "agentdeck"), "", "install", "attempt", "prepare", "--attempt", id, "--token", i5Token, "--version", i5Version, "--executable", helper, "--locator", locator)
	child.expect(t, 0, `"manifest"`)
	var envelope struct {
		Data struct{ Directory, Helper, Manifest string } `json:"data"`
	}
	if err := json.Unmarshal(child.stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return caskAttempt{id, source, envelope.Data.Helper, envelope.Data.Manifest, envelope.Data.Directory}
}

func (a caskAttempt) registerArgs(f ownershiptest.Fixture, extra ...string) []string {
	helper, _ := caskPaths(f)
	return registerArgs("homebrew_cask", helper, a.source, a.manifest, append([]string{"--require-unowned"}, extra...)...)
}

func (a caskAttempt) stagedHelper() string {
	return filepath.Join(a.source, "Contents", "Helpers", "agentdeck")
}

func caskCleanupArgs(f ownershiptest.Fixture, extra ...string) []string {
	helper, locator := caskPaths(f)
	return append([]string{"install", "attempt", "cleanup", "--token", i5Token, "--version", i5Version, "--executable", helper, "--locator", locator}, extra...)
}

func directoryBytes(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out.WriteString(rel + " " + info.Mode().String())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			out.WriteString(" " + string(data[:min(len(data), 64)]) + " " + info.ModTime().String())
		}
		out.WriteByte('\n')
		return nil
	})
	return out.String()
}

// TestInstallInterleavingI5BoundRepairAndSuccessor runs the identity-bound
// repair with the real helper commands: a valid receipt and App helper, one
// cleanup material lost, and a same-channel, same-path, same-version successor
// inserted after the identity capture or after the tolerated uninstall.
func TestInstallInterleavingI5BoundRepairAndSuccessor(t *testing.T) {
	damages := map[string]func(t *testing.T, f ownershiptest.Fixture, a caskAttempt){
		"manifest": func(t *testing.T, _ ownershiptest.Fixture, a caskAttempt) { mustRemoveFile(t, a.manifest) },
		"helper":   func(t *testing.T, _ ownershiptest.Fixture, a caskAttempt) { mustRemoveFile(t, a.helper) },
		"locator": func(t *testing.T, f ownershiptest.Fixture, _ caskAttempt) {
			_, locator := caskPaths(f)
			mustRemoveFile(t, locator)
		},
	}
	type installed struct {
		f      ownershiptest.Fixture
		a      caskAttempt
		id     string
		helper string
		cli    string
	}
	setup := func(t *testing.T, damage string) installed {
		f := ownershiptest.New(t)
		seedOwnedState(t, f)
		t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
		helper, locator := caskPaths(f)
		a := prepareCaskAttempt(t, f, 1)
		child := startInstallChild(t, f, a.stagedHelper(), "", a.registerArgs(f)...)
		child.expect(t, 0, `"installation_id"`)
		original := child.registeredID(t)
		locatorBytes, err := os.ReadFile(locator)
		if err != nil {
			t.Fatal(err)
		}
		damages[damage](t, f, a)
		cli := ownershipExecutable(t, f, "Caskroom/staged-helper")
		app, receipt := directoryBytes(t, filepath.Dir(filepath.Dir(filepath.Dir(helper)))), receiptSnapshot(t)
		// Unauthorized and unbound attempts never replace the valid owner.
		startInstallChild(t, f, cli, "", caskCleanupArgs(f)...).expect(t, 1, "cleanup_materials_unavailable")
		refused := prepareCaskAttempt(t, f, 8)
		startInstallChild(t, f, refused.stagedHelper(), "", refused.registerArgs(f)...).expect(t, 1, "owned_by_other")
		startInstallChild(t, f, refused.stagedHelper(), "", refused.registerArgs(f, "--authorized-repair")...).expect(t, 1, "owned_by_other")
		// Homebrew restores the staged directory of the version it could not replace.
		if damage == "locator" {
			mustRemoveFile(t, locator)
		} else if err := os.WriteFile(locator, locatorBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		// The identity is captured once, by the installed App's own helper.
		capture := startInstallChild(t, f, helper, "", "install", "repair-identity", "--channel", "homebrew_cask", "--executable", helper)
		capture.expect(t, 0, original)
		if app != directoryBytes(t, filepath.Dir(filepath.Dir(filepath.Dir(helper)))) || !bytes.Equal(receipt, receiptSnapshot(t)) {
			t.Fatal("refused repairs or the identity capture changed the App or receipt")
		}
		return installed{f, a, original, helper, cli}
	}
	bound := func(s installed) []string { return []string{"--authorized-repair", "--expected-installation", s.id} }
	appRoot := func(s installed) string { return filepath.Dir(filepath.Dir(filepath.Dir(s.helper))) }
	installSuccessor := func(t *testing.T, s installed) {
		t.Helper()
		source := stageAppBundle(t, s.f, ".AgentDeck.app.successor")
		manifest := filepath.Join(s.f.Base, "successor.json")
		startInstallChild(t, s.f, filepath.Join(source, "Contents", "Helpers", "agentdeck"), "", registerArgs("homebrew_cask", s.helper, source, manifest)...).expect(t, 0, `"installation_id"`)
	}
	for damage := range damages {
		t.Run(damage+"/no-competitor", func(t *testing.T) {
			s := setup(t, damage)
			remaining := directoryBytes(t, s.a.directory)
			app, receipt := directoryBytes(t, appRoot(s)), receiptSnapshot(t)
			startInstallChild(t, s.f, s.cli, "", caskCleanupArgs(s.f, bound(s)...)...).expect(t, 0, `"skipped_unverified"`)
			if app != directoryBytes(t, appRoot(s)) || !bytes.Equal(receipt, receiptSnapshot(t)) {
				t.Fatal("the tolerated uninstall changed the App or receipt")
			}
			// A failure after the swap and before the receipt restores A.
			failed := prepareCaskAttempt(t, s.f, 2)
			startInstallChild(t, s.f, failed.stagedHelper(), "register_published=fail", failed.registerArgs(s.f, bound(s)...)...).expect(t, 1, "injected failure")
			if app != directoryBytes(t, appRoot(s)) || !bytes.Equal(receipt, receiptSnapshot(t)) {
				t.Fatal("a failed bound replacement did not restore the original App and receipt")
			}
			repaired := prepareCaskAttempt(t, s.f, 3)
			child := startInstallChild(t, s.f, repaired.stagedHelper(), "", repaired.registerArgs(s.f, bound(s)...)...)
			child.expect(t, 0, `"installation_id"`)
			if newID := child.registeredID(t); newID == s.id || repaired.directory == s.a.directory {
				t.Fatal("the repair reused the original attempt or installation ID")
			}
			if remaining != directoryBytes(t, s.a.directory) {
				t.Fatal("the original attempt's remaining materials changed")
			}
			repairedApp, repairedReceipt := directoryBytes(t, appRoot(s)), receiptSnapshot(t)
			// A's delayed cleanup, where its manifest survives, skips the repair.
			if damage != "manifest" {
				startInstallChild(t, s.f, s.cli, "", "install", "unregister", "--channel", "homebrew_cask", "--executable", s.helper, "--expected-installation", s.id, "--cleanup-manifest", s.a.manifest, "--remove").expect(t, 0, `"skipped_superseded"`)
			}
			if repairedApp != directoryBytes(t, appRoot(s)) || !bytes.Equal(repairedReceipt, receiptSnapshot(t)) {
				t.Fatal("the original installation's delayed cleanup changed the repaired one")
			}
			code, out := runOwnedCommand(t, s.f, s.helper, "--state-dir", s.f.Real, "--format", "json", "provider", "list")
			if code != 0 || !strings.Contains(out, "existing-owner-data") {
				t.Fatalf("the repaired installation does not own state: %d %s", code, out)
			}
			// A plain, unauthorized uninstall now works again.
			startInstallChild(t, s.f, s.cli, "", caskCleanupArgs(s.f)...).expect(t, 0, `"removed"`)
			if fileExists(appRoot(s)) || len(receiptSnapshot(t)) != 0 {
				t.Fatal("the plain uninstall after repair left the App or receipt")
			}
		})
		for _, receiptDamage := range []string{"intact", "missing", "invalid"} {
			t.Run(damage+"/successor-after-capture/"+receiptDamage, func(t *testing.T) {
				s := setup(t, damage)
				installSuccessor(t, s)
				want := damageSuccessorReceipt(t, receiptDamage)
				app, receipt := directoryBytes(t, appRoot(s)), receiptSnapshot(t)
				startInstallChild(t, s.f, s.cli, "", caskCleanupArgs(s.f, bound(s)...)...).expect(t, 1, want)
				attempt := prepareCaskAttempt(t, s.f, 2)
				startInstallChild(t, s.f, attempt.stagedHelper(), "", attempt.registerArgs(s.f, bound(s)...)...).expect(t, 1, want)
				if app != directoryBytes(t, appRoot(s)) || !bytes.Equal(receipt, receiptSnapshot(t)) {
					t.Fatal("the successor's App or receipt changed")
				}
			})
			t.Run(damage+"/successor-before-register/"+receiptDamage, func(t *testing.T) {
				s := setup(t, damage)
				startInstallChild(t, s.f, s.cli, "", caskCleanupArgs(s.f, bound(s)...)...).expect(t, 0, `"skipped_unverified"`)
				// A's bound register is paused before it takes the lock; the
				// successor is installed through the real protocol meanwhile.
				attempt := prepareCaskAttempt(t, s.f, 2)
				pause, hook := newInstallPause(t, "register_unlocked")
				a := startInstallChild(t, s.f, attempt.stagedHelper(), hook, attempt.registerArgs(s.f, bound(s)...)...)
				pause.reached(t)
				installSuccessor(t, s)
				want := damageSuccessorReceipt(t, receiptDamage)
				app, receipt := directoryBytes(t, appRoot(s)), receiptSnapshot(t)
				pause.release(t)
				a.expect(t, 1, want)
				if app != directoryBytes(t, appRoot(s)) || !bytes.Equal(receipt, receiptSnapshot(t)) {
					t.Fatal("the successor's App or receipt changed")
				}
				// The expected ID is never refreshed from the successor: an
				// unbound retry is refused too, for the same reason.
				retry := prepareCaskAttempt(t, s.f, 4)
				if receiptDamage == "intact" {
					startInstallChild(t, s.f, retry.stagedHelper(), "", retry.registerArgs(s.f, "--authorized-repair")...).expect(t, 1, "owned_by_other")
				}
				startInstallChild(t, s.f, retry.stagedHelper(), "", retry.registerArgs(s.f, bound(s)...)...).expect(t, 1, want)
				if app != directoryBytes(t, appRoot(s)) {
					t.Fatal("a retry changed the successor's App")
				}
			})
		}
	}
}

func mustRemoveFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// damageSuccessorReceipt optionally breaks the successor's receipt and returns
// the refusal a bound repair must then report.
func damageSuccessorReceipt(t *testing.T, kind string) string {
	t.Helper()
	path := filepath.Join(os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME"), "Library", "Application Support", "AgentDeck", "installation.json")
	switch kind {
	case "missing":
		mustRemoveFile(t, path)
		return "receipt_missing"
	case "invalid":
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		return "receipt_invalid"
	}
	return "owned_by_other"
}

// A bound repair must reject unverifiable ownership even when the original
// attempt's locator, manifest and retained helper are all still available.
func TestInstallBoundCleanupCompleteMaterials(t *testing.T) {
	for _, successor := range []bool{false, true} {
		for _, damage := range []string{"missing", "invalid"} {
			name := "original/" + damage
			if successor {
				name = "successor/" + damage
			}
			t.Run(name, func(t *testing.T) {
				f := ownershiptest.New(t)
				t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
				helper, _ := caskPaths(f)
				a := prepareCaskAttempt(t, f, 1)
				registration := startInstallChild(t, f, a.stagedHelper(), "", a.registerArgs(f)...)
				registration.expect(t, 0, `"installation_id"`)
				id := registration.registeredID(t)
				startInstallChild(t, f, helper, "", "install", "repair-identity", "--channel", "homebrew_cask", "--executable", helper).expect(t, 0, id)
				cli := ownershipExecutable(t, f, "Caskroom/staged-helper")
				bound := caskCleanupArgs(f, "--authorized-repair", "--expected-installation", id)
				if successor {
					source := stageAppBundle(t, f, ".AgentDeck.app.successor")
					startInstallChild(t, f, filepath.Join(source, "Contents", "Helpers", "agentdeck"), "", registerArgs("homebrew_cask", helper, source, filepath.Join(f.Base, "successor.json"))...).expect(t, 0, `"installation_id"`)
					startInstallChild(t, f, cli, "", bound...).expect(t, 1, "owned_by_other")
					// Ordinary delayed cleanup still skips an intact successor.
					startInstallChild(t, f, cli, "", caskCleanupArgs(f)...).expect(t, 0, `"skipped_superseded"`)
				}
				validReceipt := receiptSnapshot(t)
				want := damageSuccessorReceipt(t, damage)
				appRoot := filepath.Dir(filepath.Dir(filepath.Dir(helper)))
				app, receipt, materials := directoryBytes(t, appRoot), receiptSnapshot(t), directoryBytes(t, a.directory)
				helperBefore, err := os.ReadFile(helper)
				if err != nil {
					t.Fatal(err)
				}
				startInstallChild(t, f, cli, "", bound...).expect(t, 1, want)
				// Unbound authorization retains its explicit tolerance.
				startInstallChild(t, f, cli, "", caskCleanupArgs(f, "--authorized-repair")...).expect(t, 0, `"skipped_unverified"`)
				if app != directoryBytes(t, appRoot) || !bytes.Equal(receipt, receiptSnapshot(t)) || materials != directoryBytes(t, a.directory) {
					t.Fatal("cleanup changed the App, receipt or complete materials")
				}
				helperAfter, err := os.ReadFile(helper)
				if err != nil || !bytes.Equal(helperBefore, helperAfter) {
					t.Fatal("cleanup changed the complete App helper contents")
				}
				if !successor {
					path := filepath.Join(f.Account, "Library", "Application Support", "AgentDeck", "installation.json")
					if err := os.WriteFile(path, validReceipt, 0o600); err != nil {
						t.Fatal(err)
					}
					// A verified original with complete materials can be removed.
					startInstallChild(t, f, cli, "", bound...).expect(t, 0, `"removed"`)
					if fileExists(appRoot) || len(receiptSnapshot(t)) != 0 {
						t.Fatal("verified bound cleanup left the installation behind")
					}
				}
			})
		}
	}
}
