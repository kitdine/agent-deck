//go:build agentdeck_acceptance && darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/ownershiptest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/watch"
)

type observedOwnershipResolver struct {
	*installation.Resolver
	once sync.Once
}

func (r *observedOwnershipResolver) Resolve() platform.InstallationIdentity {
	identity := r.Resolver.Resolve()
	r.once.Do(func() {
		f := os.NewFile(3, "ownership-observed")
		if f != nil {
			_, _ = f.WriteString("resolved\n")
			_ = f.Close()
		}
	})
	return identity
}

func ownershipSessionProbe(mode, root string) int {
	ctx := context.Background()
	var core *store.Store
	var err error
	if mode == "hold-ro" {
		core, err = store.OpenReadOnly(ctx, root)
	} else {
		core, err = store.Open(ctx, root)
	}
	if err != nil {
		renderCommandErrorText(os.Stderr, err)
		return 1
	}
	defer core.Close()
	closeSession := func() { _ = core.Close(); _ = store.CloseReadPools(); _ = platform.ReleaseStateOwnership() }
	count := 0
	w := watch.Service{StateRoot: root, Sources: watch.SourceSet{{Domain: "fixture", Snapshot: func(context.Context) (string, error) { return fmt.Sprint(count + 1), nil }, Scan: func(context.Context) (int, error) { count++; return 1, nil }}}, Lock: func(ctx context.Context) (func() error, error) {
		lock, err := store.AcquireScanLock(ctx, root, time.Second)
		if err != nil {
			return nil, err
		}
		return lock.Release, nil
	}}
	if strings.HasPrefix(mode, "rounds-") {
		if _, err = w.Poll(ctx); err != nil {
			renderCommandErrorText(os.Stderr, err)
			return 1
		}
	}
	if mode == "rounds-release" {
		closeSession()
	}
	fmt.Println("ready")
	if _, err = io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
		return 2
	}
	if mode == "hold-renewal" {
		closeSession()
		fmt.Println("released")
		if _, err = io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
			return 2
		}
		err = platform.RevalidateOwnership(root)
	} else if strings.HasPrefix(mode, "rounds-") {
		_, err = w.Poll(ctx)
		fmt.Printf("rounds=%d\n", count)
	} else {
		_, err = core.SchemaVersion(ctx)
		if err == nil {
			fmt.Println("complete")
		}
	}
	if err != nil {
		renderCommandErrorText(os.Stderr, err)
		return 1
	}
	return 0
}

func ownershipExecutable(t *testing.T, f ownershiptest.Fixture, name string) string {
	t.Helper()
	path := filepath.Join(f.Base, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(os.Args[0], path); err != nil {
		source, err := os.Open(os.Args[0])
		if err != nil {
			t.Fatal(err)
		}
		defer source.Close()
		target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(target, source)
		closeErr := target.Close()
		if copyErr != nil || closeErr != nil {
			t.Fatal(copyErr, closeErr)
		}
	}
	return path
}

func registerOwnershipFixture(t *testing.T, f ownershiptest.Fixture, executable, channel string) installation.Receipt {
	t.Helper()
	t.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", f.Account)
	lock, err := platform.AcquireOwnershipLock(context.Background(), f.Account, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	receipt, err := installation.DefaultResolver().WriteReceipt(lock, executable, channel)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func seedOwnedState(t *testing.T, f ownershiptest.Fixture) {
	t.Helper()
	var out bytes.Buffer
	if err := run([]string{"--state-dir", f.Real, "provider", "add", "existing-owner-data", "--endpoint", "https://example.invalid", "--clients", "codex"}, strings.NewReader("synthetic-secret\n"), &out); err != nil {
		t.Fatal(err)
	}
}

type heldOwnershipProcess struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Reader
	errors  bytes.Buffer
}

func startHeldOwnership(t *testing.T, f ownershiptest.Fixture, executable, mode string) *heldOwnershipProcess {
	t.Helper()
	command := ownershipProbeChild(f, mode, f.Real)
	command.Path = executable
	command.Args[0] = executable
	p := &heldOwnershipProcess{command: command}
	var err error
	p.input, err = command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.output = bufio.NewReader(output)
	command.Stderr = &p.errors
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = p.input.Close()
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	line, err := p.output.ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("session failed before ready: %q %v %s", line, err, p.errors.String())
	}
	return p
}

func (p *heldOwnershipProcess) finish(t *testing.T, want int) string {
	t.Helper()
	if _, err := p.input.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	p.input.Close()
	output, err := io.ReadAll(p.output)
	if err != nil {
		t.Fatal(err)
	}
	err = p.command.Wait()
	code := 0
	if err != nil {
		code = p.command.ProcessState.ExitCode()
	}
	if code != want {
		t.Fatalf("held process exit=%d want=%d: %v, %s", code, want, err, p.errors.String())
	}
	return string(output) + p.errors.String()
}

func assertOwnershipBusy(t *testing.T, f ownershiptest.Fixture, before installation.Receipt) {
	t.Helper()
	lock, err := platform.AcquireOwnershipLock(context.Background(), f.Account, true)
	if lock != nil {
		lock.Release()
	}
	var busy *platform.OwnershipLockError
	if !errors.As(err, &busy) || busy.Reason != "owner_running" {
		t.Fatalf("takeover did not time out with owner_running: %v", err)
	}
	after, err := installation.DefaultResolver().ReadReceipt()
	if err != nil || after != before {
		t.Fatalf("contended takeover changed receipt: %+v %v", after, err)
	}
}

func runOwnedCommand(t *testing.T, f ownershiptest.Fixture, executable string, args ...string) (int, string) {
	t.Helper()
	command := ownershipChild(f, args, false)
	command.Path = executable
	command.Args[0] = executable
	out, err := command.CombinedOutput()
	if command.ProcessState == nil {
		t.Fatal(err)
	}
	return command.ProcessState.ExitCode(), string(out)
}

func TestOwnershipControlledInterleavings(t *testing.T) {
	for _, mode := range []string{"hold-rw", "hold-ro"} {
		t.Run(map[string]string{"hold-rw": "I1-write-session", "hold-ro": "I3-read-only-session"}[mode], func(t *testing.T) {
			f := ownershiptest.New(t)
			a := ownershipExecutable(t, f, "a/agentdeck")
			seedOwnedState(t, f)
			before := registerOwnershipFixture(t, f, a, "make")
			p := startHeldOwnership(t, f, a, mode)
			assertOwnershipBusy(t, f, before)
			if out := p.finish(t, 0); !strings.Contains(out, "complete") {
				t.Fatalf("owner did not finish read: %s", out)
			}
			lock, err := platform.AcquireOwnershipLock(context.Background(), f.Account, true)
			if err != nil {
				t.Fatal(err)
			}
			lock.Release()
		})
	}
	t.Run("I2-exclusive-first-hook-rechecks", func(t *testing.T) {
		f := ownershiptest.New(t)
		a := ownershipExecutable(t, f, "a/agentdeck")
		b := ownershipExecutable(t, f, "b/agentdeck")
		registerOwnershipFixture(t, f, a, "make")
		f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
		lock, err := platform.AcquireOwnershipLock(context.Background(), f.Account, true)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Release()
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer read.Close()
		command := ownershipChild(f, []string{"--state-dir", f.Real, "usage", "hook", "event", "codex"}, false)
		command.Path = a
		command.Args[0] = a
		command.Env = append(command.Env, "AGENTDECK_OWNERSHIP_NOTIFY=1")
		command.ExtraFiles = []*os.File{write}
		command.Stdin = strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"fixture"}`)
		var out bytes.Buffer
		command.Stdout = &out
		command.Stderr = &out
		f.Sandbox(command)
		if err = command.Start(); err != nil {
			t.Fatal(err)
		}
		write.Close()
		if line, err := bufio.NewReader(read).ReadString('\n'); err != nil || line != "resolved\n" {
			t.Fatalf("no initial identity observation: %q %v", line, err)
		}
		if _, err = installation.DefaultResolver().WriteReceipt(lock, b, "make"); err != nil {
			t.Fatal(err)
		}
		lock.Release()
		if err = command.Wait(); err != nil || out.Len() != 0 {
			t.Fatalf("hook did not fail open after locked recheck: %v %q", err, out.String())
		}
	})
	t.Run("I4-cross-round-session-blocks-until-exit", func(t *testing.T) {
		f := ownershiptest.New(t)
		a := ownershipExecutable(t, f, "a/agentdeck")
		b := ownershipExecutable(t, f, "b/agentdeck")
		seedOwnedState(t, f)
		before := registerOwnershipFixture(t, f, a, "make")
		p := startHeldOwnership(t, f, a, "rounds-hold")
		assertOwnershipBusy(t, f, before)
		if out := p.finish(t, 0); !strings.Contains(out, "rounds=2") {
			t.Fatalf("held owner failed across rounds: %s", out)
		}
		registerOwnershipFixture(t, f, b, "make")
		positive := ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false)
		positive.Path = a
		positive.Args[0] = a
		f.Controls(t, positive)
		command := ownershipChild(f, []string{"--state-dir", f.Real, "provider", "list"}, false)
		command.Path = a
		command.Args[0] = a
		f.Sandbox(command)
		out, err := command.CombinedOutput()
		if command.ProcessState.ExitCode() != 1 || !bytes.Contains(out, []byte("receipt_other_executable")) {
			t.Fatalf("old owner accessed state: %v %s", err, out)
		}
	})
	t.Run("I5-released-round-stops-before-next-callback", func(t *testing.T) {
		f := ownershiptest.New(t)
		a := ownershipExecutable(t, f, "a/agentdeck")
		b := ownershipExecutable(t, f, "b/agentdeck")
		seedOwnedState(t, f)
		registerOwnershipFixture(t, f, a, "make")
		p := startHeldOwnership(t, f, a, "rounds-release")
		registerOwnershipFixture(t, f, b, "make")
		out := p.finish(t, 1)
		if !strings.Contains(out, "rounds=1") || !strings.Contains(out, "state_ownership_refused") {
			t.Fatalf("revoked process began another round: %s", out)
		}
	})
	t.Run("I6-opt-switch-retains-live-keg-owner", func(t *testing.T) {
		f := ownershiptest.New(t)
		a, b, opt := ownershipFormulaFixture(t, f)
		seedOwnedState(t, f)
		registerOwnershipFixture(t, f, a, "homebrew_formula")
		p := startHeldOwnership(t, f, a, "hold-rw")
		switchOwnershipOpt(t, opt, b)
		f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
		command := ownershipChild(f, []string{"--state-dir", f.Real, "provider", "list"}, false)
		command.Path = b
		command.Args[0] = b
		f.Sandbox(command)
		out, err := command.CombinedOutput()
		if command.ProcessState.ExitCode() != 1 || !bytes.Contains(out, []byte("receipt_other_executable")) {
			t.Fatalf("opt switched ownership: %v %s", err, out)
		}
		if out := p.finish(t, 0); !strings.Contains(out, "complete") {
			t.Fatalf("old session was interrupted: %s", out)
		}
	})
	t.Run("I7-renewal-waits-and-deleted-owner-revalidates", func(t *testing.T) {
		f := ownershiptest.New(t)
		a, b, opt := ownershipFormulaFixture(t, f)
		seedOwnedState(t, f)
		before := registerOwnershipFixture(t, f, a, "homebrew_formula")
		p := startHeldOwnership(t, f, a, "hold-renewal")
		if err := os.Remove(a); err != nil {
			t.Fatal(err)
		}
		switchOwnershipOpt(t, opt, b)
		f.Home = f.Account
		f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
		command := ownershipChild(f, []string{"provider", "list"}, false)
		command.Path = b
		command.Args[0] = b
		f.Sandbox(command)
		out, err := command.CombinedOutput()
		if command.ProcessState.ExitCode() != 1 || !bytes.Contains(out, []byte("ownership_busy")) {
			t.Fatalf("renewal ignored live session: %v %s", err, out)
		}
		after, err := installation.DefaultResolver().ReadReceipt()
		if err != nil || after != before {
			t.Fatal("busy renewal changed receipt", err)
		}
		if _, err := os.Stat(filepath.Join(f.Account, ".agentdeck-dev")); !os.IsNotExist(err) {
			t.Fatalf("failed renewal fell back to isolated root: %v", err)
		}
		_, _ = p.input.Write([]byte("x"))
		if line, err := p.output.ReadString('\n'); err != nil || line != "released\n" {
			t.Fatalf("old session did not release: %q %v", line, err)
		}
		if code, out := runOwnedCommand(t, f, b, "provider", "list"); code != 0 || !strings.Contains(out, "existing-owner-data") {
			t.Fatalf("renewed default lost existing data: %d %s", code, out)
		}
		if out := p.finish(t, 1); !strings.Contains(out, "executable_unresolved") {
			t.Fatalf("deleted owner resumed: %s", out)
		}
	})
	t.Run("I8-default-renews-explicit-isolated-does-not", func(t *testing.T) {
		f := ownershiptest.New(t)
		a, b, opt := ownershipFormulaFixture(t, f)
		seedOwnedState(t, f)
		before := registerOwnershipFixture(t, f, a, "homebrew_formula")
		if err := os.Remove(a); err != nil {
			t.Fatal(err)
		}
		switchOwnershipOpt(t, opt, b)
		f.Home = f.Account
		f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
		command := ownershipChild(f, []string{"--state-dir", filepath.Join(f.Base, "isolated"), "provider", "list"}, false)
		command.Path = b
		command.Args[0] = b
		f.Sandbox(command)
		out, err := command.CombinedOutput()
		if err != nil || string(out) != "No providers.\n" {
			t.Fatalf("explicit isolation failed: %v %s", err, out)
		}
		after, err := installation.DefaultResolver().ReadReceipt()
		if err != nil || after != before {
			t.Fatal("explicit isolation renewed receipt", err)
		}
		if code, out := runOwnedCommand(t, f, b, "provider", "list"); code != 0 || !strings.Contains(out, "existing-owner-data") {
			t.Fatalf("default did not renew into original data: %d %s", code, out)
		}
		after, err = installation.DefaultResolver().ReadReceipt()
		if err != nil || after.Executable != b {
			t.Fatalf("new keg not registered: %+v %v", after, err)
		}
	})
}

func ownershipFormulaFixture(t *testing.T, f ownershiptest.Fixture) (string, string, string) {
	t.Helper()
	a := ownershipExecutable(t, f, "prefix/Cellar/agentdeck/1/bin/agentdeck")
	b := ownershipExecutable(t, f, "prefix/Cellar/agentdeck/2/bin/agentdeck")
	opt := filepath.Join(f.Base, "prefix", "opt", "agentdeck")
	if err := os.MkdirAll(filepath.Dir(opt), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.Dir(a)), opt); err != nil {
		t.Fatal(err)
	}
	return a, b, opt
}
func switchOwnershipOpt(t *testing.T, opt, executable string) {
	t.Helper()
	if err := os.Remove(opt); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.Dir(executable)), opt); err != nil {
		t.Fatal(err)
	}
}
