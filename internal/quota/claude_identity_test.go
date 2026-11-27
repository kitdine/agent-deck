package quota

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestClaudeIdentityClassifiesApprovedAndUnknownShapes(t *testing.T) {
	fixture, err := os.ReadFile("testdata/claude-auth-status-2.1.325.sample.json")
	if err != nil {
		t.Fatal(err)
	}
	compare := func(email, org string) (*claudeComparison, error) {
		if email != "alice@example.test" || org != "org:synthetic-org-42" {
			t.Fatal("comparison received wrong identity")
		}
		return &claudeComparison{correlation: [32]byte{5}}, nil
	}
	got := classifyClaudeIdentity(fixture, compare)
	if got.State != IdentityIdentified || got.Failure != "" || got.claude == nil ||
		got.Display.EmailMasked != "a***e@example.test" || got.Display.OrganizationName != "Example Organization" ||
		got.Display.OrganizationKind != OrgOrganization || got.accountKind != "claude.ai" {
		t.Fatalf("fixture yielded state %q failure %q display %+v", got.State, got.Failure, got.Display)
	}
	plan := ProjectPlan(PlanSourceClaudeAuthStatus, got.Display.Plan)
	if plan.Name != "Claude Pro" || plan.ShortName != "Pro" || plan.Basis != PlanBasisMapping || len(plan.Raw) != 2 {
		t.Fatalf("fixture plan = %+v", plan)
	}
	tests := []struct {
		name, input string
		state       IdentityState
		failure     IdentityFailure
	}{
		{"signed out", `{"loggedIn":false,"secret":{"ignored":"sentinel"}}`, IdentitySignedOut, ""},
		{"API key", `{"loggedIn":true,"authMethod":"api_key"}`, IdentityNonSubscription, ""},
		{"external provider", `{"loggedIn":true,"apiProvider":"vertex"}`, IdentityNonSubscription, ""},
		{"future method", `{"loggedIn":true,"authMethod":"future","apiProvider":"firstParty"}`, "", IdentityIncomparable},
		{"future provider", strings.Replace(string(fixture), "firstParty", "future", 1), "", IdentityIncomparable},
		{"missing org", `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","email":"alice@example.test"}`, "", IdentityIncomparable},
		{"null org", strings.Replace(string(fixture), `"synthetic-org-42"`, "null", 1), "", IdentityIncomparable},
		{"bad email", strings.Replace(string(fixture), "alice@example.test", "alice", 1), "", IdentityIncomparable},
		{"missing loggedIn", `{"LoggedIn":false}`, "", IdentityParseFailed},
		{"null loggedIn", `{"loggedIn":null}`, "", IdentityParseFailed},
		{"bad field type", `{"loggedIn":false,"orgId":27}`, "", IdentityParseFailed},
		{"duplicate loggedIn", `{"loggedIn":true,"loggedIn":false}`, "", IdentityParseFailed},
		{"trailing value", `{"loggedIn":false} {}`, "", IdentityParseFailed},
		{"non-object", `[{"loggedIn":false}]`, "", IdentityParseFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := classifyClaudeIdentity([]byte(test.input), func(string, string) (*claudeComparison, error) {
				t.Fatal("uncomparable response tried to read the local key")
				return nil, nil
			})
			if result.State != test.state || result.Failure != test.failure || result.Display != (IdentityDisplay{}) {
				t.Fatalf("unexpected state=%q failure=%q", result.State, result.Failure)
			}
		})
	}
	noKey := classifyClaudeIdentity(fixture, func(string, string) (*claudeComparison, error) {
		return nil, errors.New("private path sentinel")
	})
	if noKey.Failure != IdentityIncomparable || noKey.Display != (IdentityDisplay{}) {
		t.Fatal("unavailable key produced a comparable identity")
	}
}

func TestClaudeIdentityDoesNotPublishForbiddenStrings(t *testing.T) {
	input := `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty",
		"email":"AbC@example.test","orgId":"aBcD","orgName":"my abcd Organization",
		"planTier":"P-abc-plus","subscriptionType":"pro","unknown":"secret-sentinel"}`
	result := classifyClaudeIdentity([]byte(input), func(string, string) (*claudeComparison, error) {
		return &claudeComparison{}, nil
	})
	plan := ProjectPlan(PlanSourceClaudeAuthStatus, result.Display.Plan)
	if result.State != IdentityIdentified || result.Display.OrganizationName != "" ||
		result.Display.EmailMasked != "A***@example.test" || plan.Reason != PlanUnsafeValue || len(plan.Raw) != 0 {
		t.Fatal("identity did not enforce the forbidden-value screen")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := encodePlanSources(result.Display.Plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"AbC@", "abcd", "P-abc-plus", "secret-sentinel"} {
		if strings.Contains(strings.ToLower(string(encoded)+stored), strings.ToLower(marker)) {
			t.Fatal("forbidden identity text escaped the adapter")
		}
	}
}

func isolateClaudeAdapterTestEnv(t *testing.T, root string) {
	t.Helper()
	t.Setenv("HOME", root)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "claude-config"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex-config"))
}

func TestClaudeIdentityProbeSharesResolvedContext(t *testing.T) {
	root := t.TempDir()
	isolateClaudeAdapterTestEnv(t, root)
	fixture, err := filepath.Abs("testdata/claude-auth-status-2.1.325.sample.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROBE_FIXTURE", fixture)
	log := filepath.Join(root, "calls")
	t.Setenv("PROBE_LOG", log)
	t.Setenv("PROBE_MARKER", "frozen")
	t.Setenv("PATH", root+":/bin")
	args := claudeProseCommandArgs
	claudeProseCommandArgs = []string{"claude", "-p", "/usage"}
	t.Cleanup(func() { claudeProseCommandArgs = args })
	script := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$*\" \"$PROBE_MARKER\" \"$PWD\" >> \"$PROBE_LOG\"\n" + `
case "$*" in
  'auth status --json') /bin/cat "$PROBE_FIXTURE";;
  '-p /usage') printf '%s\n' '` + strings.ReplaceAll(strings.TrimSpace(claudeProseSample), "'", "'\\''") + `';;
  *) exit 77;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	probe, err := NewClaudeProbe()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROBE_MARKER", "changed")
	t.Setenv("PATH", "/no-such-fixture-bin")
	t.Chdir(t.TempDir())
	state := filepath.Join(root, "state")
	identity := probe.ProbeIdentity(context.Background(), state)
	if identity.State != IdentityIdentified || identity.claude == nil {
		t.Fatalf("identity state=%q failure=%q", identity.State, identity.Failure)
	}
	usage, err := probe.ProbeUsage(context.Background(), time.Now())
	if err != nil || len(usage.Windows) != 2 {
		t.Fatalf("usage windows=%d: %v", len(usage.Windows), err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"auth status --json|frozen|" + probe.dir, "-p /usage|frozen|" + probe.dir}
	if got := strings.Split(strings.TrimSpace(string(calls)), "\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("subprocess context/order = %q, want %q", got, want)
	}
}

func TestClaudeIdentityProbeRejectsInvalidProcessResults(t *testing.T) {
	fixture, err := os.ReadFile("testdata/claude-auth-status-2.1.325.sample.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROBE_AUTH_JSON", string(fixture))

	tests := []struct {
		name, body string
		want       IdentityFailure
		probeLimit time.Duration
	}{
		{"nonzero signed out", "printf '%s' '{\"loggedIn\":false}'; exit 1", IdentityProbeFailed, 0},
		{"unsupported", "printf '%s' sentinel >&2; exit 2", IdentityProbeFailed, 0},
		{"signal", "printf '%s' '{\"loggedIn\":false}'; kill -TERM $$", IdentityProbeFailed, 0},
		// Keep the signed-in payload parseable even inside the truncated buffer.
		{"oversize", `printf '%s%65537s' "$PROBE_AUTH_JSON" ''`, IdentityParseFailed, 0},
		{"timeout", "while :; do :; done", IdentityProbeFailed, 100 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			isolateClaudeAdapterTestEnv(t, dir)
			script := filepath.Join(dir, "claude")
			if err := os.WriteFile(script, []byte("#!/bin/sh\n"+test.body), 0700); err != nil {
				t.Fatal(err)
			}
			probe := ClaudeProbe{path: script, dir: dir}
			const fixtureBudget = 2 * DefaultClaudeAuthTimeout
			ctx, cancel := context.WithTimeout(context.Background(), fixtureBudget)
			defer cancel()
			probeLimit := test.probeLimit
			if probeLimit == 0 {
				// The fixture context bounds startup and output for non-timeout cases.
				// A longer runner limit lets the assertion distinguish that deadline.
				probeLimit = 2 * fixtureBudget
			}
			result := probe.probeIdentity(ctx, dir, probeLimit)
			if err := ctx.Err(); err != nil {
				t.Fatalf("fixture context expired before result classification: %v", err)
			}
			if result.Failure != test.want || result.State != "" {
				t.Errorf("state=%q failure=%q, want %q", result.State, result.Failure, test.want)
			}
			if _, err := os.Stat(filepath.Join(dir, claudeIdentityKeyName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed auth probe touched the local key")
			}
		})
	}
}

func TestClaudeProseFailureDetailIsRedacted(t *testing.T) {
	tests := []struct {
		input  string
		detail FailureDetail
	}{
		{"Current session: private-sentinel", DetailMissingLine},
		{strings.Replace(claudeProseSample, "22% used", "private-sentinel% used", 1), DetailShapeInvalid},
		{strings.Replace(claudeProseSample, "America/Los_Angeles", "Private/Sentinel", 1), DetailTimezoneUnresolved},
	}
	for _, test := range tests {
		_, err := ParseClaudeProse(test.input, time.Now())
		var detail *ProseParseError
		if !errors.As(err, &detail) || detail.Detail != test.detail || err.Error() != string(test.detail) {
			t.Fatalf("failure=%v, want %q", err, test.detail)
		}
	}
}

func TestClaudeProbeWaitDelayBoundsInheritedStdout(t *testing.T) {
	root := t.TempDir()
	isolateClaudeAdapterTestEnv(t, root)
	childFile := filepath.Join(root, "child.pid")
	var child *os.Process
	t.Cleanup(func() {
		if child != nil {
			_ = child.Kill()
		}
	})

	const marker = "fixture parent completed"
	script := filepath.Join(root, "claude")
	body := "#!/bin/sh\nset -eu\n/bin/sleep 20 &\nprintf '%s\\n' \"$!\" > \"$CHILD_PID_FILE\"\nprintf '%s' '" + marker + "'\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHILD_PID_FILE", childFile)
	probe := ClaudeProbe{path: script, dir: root, env: os.Environ()}
	const fixtureBudget = 2 * DefaultClaudeAuthTimeout
	ctx, cancel := context.WithTimeout(context.Background(), fixtureBudget)
	defer cancel()
	var stdout bytes.Buffer
	err := probe.run(ctx, 2*fixtureBudget, &stdout)

	data, readErr := os.ReadFile(childFile)
	if readErr != nil {
		t.Fatalf("fixture did not start the pipe holder: %v", readErr)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if parseErr != nil || pid <= 0 {
		t.Fatalf("fixture did not record a valid pipe holder PID: %q", data)
	}
	child, readErr = os.FindProcess(pid)
	if readErr != nil {
		t.Fatalf("resolve fixture pipe holder: %v", readErr)
	}
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) || failure.Kind != ClaudeFailureExit || ctx.Err() != nil {
		t.Fatalf("want exit-class failure before the fixture context expired; got %v (context: %v)", err, ctx.Err())
	}
	if stdout.String() != marker {
		t.Fatalf("fixture did not complete its successful parent path: stdout=%q", stdout.String())
	}
	if err := child.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("pipe holder was not still running when the runner returned: %v", err)
	}
}

func TestClaudeIdentityTimeoutDoesNotSpendUsageDeadline(t *testing.T) {
	root := t.TempDir()
	isolateClaudeAdapterTestEnv(t, root)
	path := filepath.Join(root, "claude")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then while :; do :; done; fi\n" +
		"printf '%s\\n' '" + strings.TrimSpace(claudeProseSample) + "'\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	probe := ClaudeProbe{path: path, dir: root, env: os.Environ(), usageArgs: []string{"-p", "/usage"}}
	ctx := context.Background()
	if identity := probe.probeIdentity(ctx, root, 100*time.Millisecond); identity.Failure != IdentityProbeFailed {
		t.Fatalf("timed-out identity failed to return probe_failed: %q", identity.Failure)
	}
	usage, err := probe.ProbeUsage(ctx, time.Now())
	if err != nil || len(usage.Windows) != 2 {
		t.Fatalf("auth deadline contaminated a subsequent usage probe: windows=%d err=%v", len(usage.Windows), err)
	}
}
