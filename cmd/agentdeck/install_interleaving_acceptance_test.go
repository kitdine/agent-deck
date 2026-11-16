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

// I4 covers the local installers only. A cask's files are published and
// removed by Homebrew outside the ownership lock, so it has no cleanup
// manifest and makes no delayed-cleanup promise.
func TestInstallInterleavingI4DelayedCompleteCleanup(t *testing.T) {
	for _, channel := range []string{"local_app"} {
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
