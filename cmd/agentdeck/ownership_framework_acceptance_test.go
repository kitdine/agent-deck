//go:build agentdeck_acceptance && darwin

package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ownershiptest"
)

// Cobra adds help at execution time and adds __complete only for a completion
// request. Keep these framework surfaces separate from the fixed GUI inventory,
// but exercise them under exactly the same access barrier.
func TestOwnershipFrameworkSurfacesBehindBarrier(t *testing.T) {
	root := newRootCommand(strings.NewReader(""), io.Discard)
	before := allOwnershipCommands(root)
	root.InitDefaultHelpCmd()
	after := allOwnershipCommands(root)
	if len(after) != len(before)+1 || after["help"] == nil {
		t.Fatalf("unclassified framework command set: before=%d after=%d", len(before), len(after))
	}
	for _, tc := range []struct {
		name        string
		args        []string
		realRefused bool
		output      string
	}{
		{"help-command", []string{"help", "provider"}, true, "Usage:"},
		{"help-flag", []string{"--help"}, false, "Usage:"},
		{"version-flag", []string{"--version"}, true, "Release Version:"},
		{"completion-request", []string{"__complete", "provider", ""}, false, ":"},
		{"completion-without-description", []string{"__completeNoDesc", "provider", ""}, false, ":"},
	} {
		for _, branch := range []string{"default", "explicit-isolated", "explicit-real"} {
			t.Run(tc.name+"/"+branch, func(t *testing.T) {
				f := ownershiptest.New(t)
				args := append([]string(nil), tc.args...)
				if branch == "explicit-isolated" {
					args = append([]string{"--state-dir", filepath.Join(f.Base, "isolated")}, args...)
				}
				if branch == "explicit-real" {
					args = append([]string{"--state-dir", f.Real}, args...)
				}
				f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
				command := ownershipChild(f, args, false)
				f.Sandbox(command)
				var stdout, stderr bytes.Buffer
				command.Stdout = &stdout
				command.Stderr = &stderr
				err := command.Run()
				want := 0
				if branch == "explicit-real" && tc.realRefused {
					want = 1
				}
				if command.ProcessState == nil || command.ProcessState.ExitCode() != want {
					t.Fatalf("framework exit: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
				}
				if want == 1 {
					if !strings.Contains(stderr.String(), "state_ownership_refused") {
						t.Fatal(stderr.String())
					}
				} else if !strings.Contains(stdout.String(), tc.output) {
					t.Fatalf("missing framework output %q: %s", tc.output, stdout.String())
				}
			})
		}
	}
}
