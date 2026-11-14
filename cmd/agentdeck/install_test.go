package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/installation"
)

type fakeInstallOperator struct {
	claim    installation.ClaimResult
	err      error
	takeover *bool
}

func (f fakeInstallOperator) Claim(_ context.Context, takeover bool) (installation.ClaimResult, error) {
	if f.takeover != nil {
		*f.takeover = takeover
	}
	return f.claim, f.err
}

func (f fakeInstallOperator) Register(context.Context, installation.RegisterRequest) (installation.RegisterResult, error) {
	return installation.RegisterResult{}, f.err
}

func (f fakeInstallOperator) Unregister(context.Context, installation.UnregisterRequest) (installation.UnregisterResult, error) {
	return installation.UnregisterResult{}, f.err
}

func withInstallOperator(t *testing.T, operator installOperator) {
	t.Helper()
	old := currentInstallOperator
	currentInstallOperator = func() (installOperator, error) { return operator, nil }
	t.Cleanup(func() { currentInstallOperator = old })
}

// The claim specimens prove copy, stream placement, JSON fields and exit codes
// through the complete CLI path. That the protocol produces each outcome is
// proved in internal/installation.
func TestInstallClaimSpecimensRender(t *testing.T) {
	data, err := os.ReadFile("testdata/ownership-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var goldens map[string]struct {
		Stdout, Stderr, JSON string
		Exit                 int
	}
	if err := json.Unmarshal(data, &goldens); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		operator fakeInstallOperator
		args     []string
	}{
		"claim_ok":              {fakeInstallOperator{claim: installation.ClaimResult{Channel: "homebrew_formula"}}, []string{"install", "claim"}},
		"claim_owned_by_other":  {fakeInstallOperator{err: &installation.InstallError{Reason: installation.ReasonOwnedByOther}}, []string{"install", "claim"}},
		"claim_owner_running":   {fakeInstallOperator{err: &installation.InstallError{Reason: installation.ReasonOwnerRunning}}, []string{"install", "claim", "--takeover"}},
		"claim_not_current_keg": {fakeInstallOperator{err: &installation.InstallError{Reason: installation.ReasonNotCurrentFormulaKeg}}, []string{"install", "claim"}},
	} {
		t.Run(name, func(t *testing.T) {
			golden := goldens[name]
			withInstallOperator(t, tc.operator)
			var stdout, stderr bytes.Buffer
			if code := execute(tc.args, strings.NewReader(""), &stdout, &stderr); code != golden.Exit {
				t.Fatalf("exit = %d, want %d", code, golden.Exit)
			}
			if stdout.String() != golden.Stdout || stderr.String() != golden.Stderr {
				t.Fatalf("stdout=%q stderr=%q\nwant stdout=%q stderr=%q", stdout.String(), stderr.String(), golden.Stdout, golden.Stderr)
			}
			stdout.Reset()
			stderr.Reset()
			code := execute(append([]string{"--format", "json"}, tc.args...), strings.NewReader(""), &stdout, &stderr)
			if code != golden.Exit {
				t.Fatalf("json exit = %d, want %d", code, golden.Exit)
			}
			encoded := stdout.Bytes()
			if golden.Exit != 0 {
				encoded = stderr.Bytes()
				if stdout.Len() != 0 {
					t.Fatalf("refusal wrote stdout: %s", stdout.String())
				}
			}
			var got, want map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("decode %q: %v", encoded, err)
			}
			if err := json.Unmarshal([]byte(golden.JSON), &want); err != nil {
				t.Fatal(err)
			}
			delete(got, "generated_at")
			delete(want, "generated_at")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("json = %s\nwant %s", encoded, golden.JSON)
			}
		})
	}
}

func TestInstallClaimPassesTakeoverAndSkipsStateNotice(t *testing.T) {
	var takeover bool
	withInstallOperator(t, fakeInstallOperator{claim: installation.ClaimResult{Channel: "homebrew_formula", Takeover: true}, takeover: &takeover})
	var stdout, stderr bytes.Buffer
	// --state-dir is ignored: claim never resolves or guards a state root.
	if code := execute([]string{"--state-dir", "/nonexistent/../real", "--format", "json", "install", "claim", "--takeover"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !takeover || !strings.Contains(stdout.String(), `"takeover":true`) || stderr.Len() != 0 {
		t.Fatalf("takeover=%v stdout=%s stderr=%s", takeover, stdout.String(), stderr.String())
	}
}

func TestInstallCommandsFailClosedWithoutProductionResolver(t *testing.T) {
	// TestMain injects a fixture resolver; the real receipt can never be reached.
	for _, args := range [][]string{
		{"install", "claim"},
		{"install", "register", "--channel", "make", "--executable", "/tmp/agentdeck"},
		{"install", "unregister", "--channel", "make", "--executable", "/tmp/agentdeck", "--expected-installation", strings.Repeat("a", 32), "--cleanup-manifest", "/tmp/m.json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := execute(args, strings.NewReader(""), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "installation operations are unavailable") {
			t.Fatalf("%v: exit=%d stderr=%s", args, code, stderr.String())
		}
	}
}

func TestInstallRefusalReasonsForInstallers(t *testing.T) {
	for _, reason := range []string{installation.ReasonReceiptMissing, installation.ReasonReceiptInvalid} {
		withInstallOperator(t, fakeInstallOperator{err: &installation.InstallError{Reason: reason}})
		var stdout, stderr bytes.Buffer
		code := execute([]string{"--format", "json", "install", "unregister", "--channel", "make", "--executable", "/x/agentdeck", "--expected-installation", strings.Repeat("a", 32), "--cleanup-manifest", "/x/m.json", "--remove"}, strings.NewReader(""), &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), `"code":"install_claim_refused"`) || !strings.Contains(stderr.String(), `"reason":"`+reason+`"`) || strings.Contains(stderr.String(), "/x/") {
			t.Fatalf("%s: exit=%d stderr=%s", reason, code, stderr.String())
		}
	}
}

// The cask's postflight wrapper keys its migration hint on one whole stderr
// line of the text rendering, and relies on the exit code and an empty stdout.
func TestInstallRegisterRefusalTextForCaskWrapper(t *testing.T) {
	args := []string{"--format", "text", "install", "register", "--channel", "homebrew_cask", "--executable", "/x/agentdeck"}
	for _, reason := range []string{installation.ReasonOwnedByOther, installation.ReasonOwnerRunning} {
		withInstallOperator(t, fakeInstallOperator{err: &installation.InstallError{Reason: reason}})
		var stdout, stderr bytes.Buffer
		code := execute(args, strings.NewReader(""), &stdout, &stderr)
		lines := strings.Split(stderr.String(), "\n")
		found := false
		for _, line := range lines {
			found = found || line == "  reason: "+reason
		}
		if code != 1 || stdout.Len() != 0 || !found || !strings.HasPrefix(stderr.String(), "install_claim_refused: ") || strings.Contains(stderr.String(), "/x/") {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q", reason, code, stdout.String(), stderr.String())
		}
	}
}

// The cask-only repair protocol is gone: its commands and flags must not be
// accepted, so a stale cask definition fails instead of half-running.
func TestInstallRemovedCaskRepairSurface(t *testing.T) {
	withInstallOperator(t, fakeInstallOperator{})
	id := strings.Repeat("a", 32)
	for _, args := range [][]string{
		{"install", "repair-identity", "--channel", "homebrew_cask", "--executable", "/x/agentdeck"},
		{"install", "attempt", "prepare", "--attempt", id},
		{"install", "attempt", "cleanup", "--attempt", id},
		{"install", "register", "--channel", "homebrew_cask", "--executable", "/x/agentdeck", "--require-unowned"},
		{"install", "register", "--channel", "homebrew_cask", "--executable", "/x/agentdeck", "--authorized-repair"},
		{"install", "register", "--channel", "homebrew_cask", "--executable", "/x/agentdeck", "--expected-installation", id},
		{"install", "unregister", "--channel", "make", "--executable", "/x/agentdeck", "--expected-installation", id, "--cleanup-manifest", "/x/m.json", "--authorized-repair"},
		{"install", "unregister", "--channel", "make", "--executable", "/x/agentdeck", "--expected-installation", id, "--cleanup-manifest", "/x/m.json", "--require-identity"},
	} {
		var stdout, stderr bytes.Buffer
		if code := execute(args, strings.NewReader(""), &stdout, &stderr); code == 0 || stdout.Len() != 0 {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}
