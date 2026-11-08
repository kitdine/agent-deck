//go:build agentdeck_acceptance && darwin

// Package ownershiptest contains only tagged, isolated ownership acceptance
// fixtures. Production artifacts never compile this package.
package ownershiptest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

type Fixture struct {
	Base, Account, Home, Real string
	Protected                 []string
}

func New(t *testing.T) Fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := Fixture{Base: base, Account: filepath.Join(base, "account"), Home: filepath.Join(base, "home")}
	f.Real = filepath.Join(f.Account, ".agentdeck")
	f.Protected = []string{f.Real}
	for _, path := range []string{f.Real, f.Home} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.Real, "sentinel"), []byte("protected contents\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f Fixture) Environment(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra)+2)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "HOME=") || strings.HasPrefix(value, "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=") {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "HOME="+f.Home, "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="+f.Account)
	return append(env, extra...)
}

func (f Fixture) Sandbox(command *exec.Cmd) {
	var profile strings.Builder
	profile.WriteString("(version 1)\n(allow default)\n(deny file-read* file-write*")
	for _, path := range f.Protected {
		fmt.Fprintf(&profile, " (subpath %s)", strconv.Quote(path))
	}
	profile.WriteString(" (with send-signal SIGKILL))\n(allow file-read-metadata")
	for _, path := range f.Protected {
		fmt.Fprintf(&profile, " (literal %s)", strconv.Quote(path))
	}
	profile.WriteString(")\n")
	command.Args = append([]string{"/usr/bin/sandbox-exec", "-p", profile.String(), "--"}, command.Args...)
	command.Path = "/usr/bin/sandbox-exec"
}

// Controls must precede each accepted observation. The positive case executes
// the actual guard; the negative case deliberately stats a protected child.
// An unavailable/nested sandbox is a failure, never a skip or a no-write proxy.
func (f Fixture) Controls(t *testing.T, positive *exec.Cmd) {
	t.Helper()
	f.Sandbox(positive)
	output, err := positive.CombinedOutput()
	if positive.ProcessState == nil || positive.ProcessState.ExitCode() != 1 || !bytes.Contains(output, []byte("state_ownership_refused")) {
		t.Fatalf("access barrier positive control unavailable or incorrect: %v, %s", err, output)
	}
	negative := exec.Command("/usr/bin/stat", "-f", "%z", filepath.Join(f.Real, "sentinel"))
	f.Sandbox(negative)
	output, err = negative.CombinedOutput()
	if negative.ProcessState == nil {
		t.Fatalf("access barrier negative control could not run: %v, %s", err, output)
	}
	status, ok := negative.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("access barrier did not kill forbidden metadata access: %v, %s", err, output)
	}
}
