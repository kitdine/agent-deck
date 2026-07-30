package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/shellconfig"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestShellInitEmitsDynamicClientWrappersForSupportedShells(t *testing.T) {
	for _, shell := range []string{"bash", "fish", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "state")
			var stdout bytes.Buffer
			err := run([]string{"--state-dir", stateDir, "shell-init", shell}, strings.NewReader(""), &stdout)
			if err != nil {
				t.Fatalf("shell-init %s: %v", shell, err)
			}

			script := stdout.String()
			for _, client := range []string{"codex", "claude"} {
				if !strings.Contains(script, client) {
					t.Errorf("shell-init %s does not wrap %s:\n%s", shell, client, script)
				}
			}
			if !strings.Contains(script, "agentdeck shell-init") {
				t.Errorf("shell-init %s does not derive attribution dynamically:\n%s", shell, script)
			}
			if marker := shellconfig.ActivationMarkerName(shellconfig.Shell(shell)); !strings.Contains(script, marker) {
				t.Errorf("shell-init %s does not set activation marker %s:\n%s", shell, marker, script)
			}
			gatePath := provider.ProjectAttributionGatePath(stateDir)
			if !strings.Contains(script, shellQuote(gatePath)) {
				t.Errorf("shell-init %s does not guard on %s:\n%s", shell, gatePath, script)
			}
			for _, forbidden := range []string{
				"my+project",
				"https://wrapper.example",
				"synthetic-secret",
			} {
				if strings.Contains(script, forbidden) {
					t.Errorf("shell-init %s embeds %q:\n%s", shell, forbidden, script)
				}
			}
			if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
				t.Errorf("shell-init %s state directory = %v, want not created", shell, err)
			}
		})
	}
}

func TestShellInitQuotesGatePathAndDefinesBothWrappers(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state'o")
	gatePath := provider.ProjectAttributionGatePath(stateDir)

	for _, shell := range []string{"bash", "fish", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			shellPath, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not installed", shell)
			}
			var stdout bytes.Buffer
			if err := run(
				[]string{"--state-dir", stateDir, "shell-init", shell},
				strings.NewReader(""),
				&stdout,
			); err != nil {
				t.Fatalf("generate %s shell-init: %v", shell, err)
			}
			scriptPath := filepath.Join(root, "agentdeck-"+shell)
			if err := os.WriteFile(scriptPath, stdout.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			syntax := exec.Command(shellPath, "-n", scriptPath)
			if output, err := syntax.CombinedOutput(); err != nil {
				t.Fatalf("%s -n rejected generated script: %v\n%s", shell, err, output)
			}

			script := stdout.String()
			var gateExpressions []string
			for _, line := range strings.Split(script, "\n") {
				line = strings.TrimSpace(line)
				if shell == "fish" && strings.HasPrefix(line, "if test -f ") {
					gateExpressions = append(
						gateExpressions,
						strings.TrimPrefix(line, "if test -f "),
					)
					continue
				}
				if shell != "fish" && strings.HasPrefix(line, "if [ -f ") {
					expression := strings.TrimPrefix(line, "if [ -f ")
					end := strings.Index(expression, " ] &&")
					if end < 0 {
						t.Fatalf("bash gate line has no closing test: %q", line)
					}
					gateExpressions = append(gateExpressions, expression[:end])
				}
			}
			if len(gateExpressions) != 2 {
				t.Fatalf("%s gate expressions = %#v, want codex and claude", shell, gateExpressions)
			}
			for _, expression := range gateExpressions {
				var evaluate *exec.Cmd
				if shell == "fish" {
					evaluate = exec.Command(
						shellPath,
						"--no-config",
						"-c",
						`printf '%s' `+expression,
					)
				} else if shell == "zsh" {
					evaluate = exec.Command(
						shellPath,
						"-f",
						"-c",
						`printf '%s' `+expression,
					)
				} else {
					evaluate = exec.Command(
						shellPath,
						"--noprofile",
						"--norc",
						"-c",
						`printf '%s' `+expression,
					)
				}
				output, err := evaluate.CombinedOutput()
				if err != nil {
					t.Fatalf("evaluate %s gate expression %q: %v\n%s", shell, expression, err, output)
				}
				if string(output) != gatePath {
					t.Fatalf("%s embedded gate path = %q, want %q", shell, output, gatePath)
				}
			}

			var inspect *exec.Cmd
			if shell == "fish" {
				inspect = exec.Command(
					shellPath,
					"--no-config",
					"-c",
					`source $argv[1]
functions -q codex; or exit 20
functions -q claude; or exit 21`,
					scriptPath,
				)
			} else if shell == "zsh" {
				inspect = exec.Command(
					shellPath,
					"-f",
					"-c",
					`source "$1"
whence -w codex >/dev/null || exit 20
whence -w claude >/dev/null || exit 21`,
					"_",
					scriptPath,
				)
			} else {
				inspect = exec.Command(
					shellPath,
					"--noprofile",
					"--norc",
					"-c",
					`source "$1"
declare -F codex >/dev/null || exit 20
declare -F claude >/dev/null || exit 21`,
					"_",
					scriptPath,
				)
			}
			output, err := inspect.CombinedOutput()
			if err != nil {
				t.Fatalf("source and inspect %s script: %v\n%s", shell, err, output)
			}
		})
	}
}

func TestShellInitScriptsMarkTheEvaluatingShellProcess(t *testing.T) {
	for _, shell := range []string{"bash", "fish", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			shellPath, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not installed", shell)
			}
			scriptPath := filepath.Join(t.TempDir(), "agentdeck-"+shell)
			if err := os.WriteFile(
				scriptPath,
				[]byte(shellInitScript(shell, "", filepath.Join(t.TempDir(), "project-attribution.enabled"))),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			marker := shellconfig.ActivationMarkerName(shellconfig.Shell(shell))
			var command *exec.Cmd
			if shell == "fish" {
				command = exec.Command(
					shellPath,
					"--no-config",
					"-c",
					fmt.Sprintf(`source $argv[1]; printf '%%s\n%%s\n' "$%s" "$fish_pid"`, marker),
					scriptPath,
				)
			} else {
				command = exec.Command(
					shellPath,
					"--noprofile",
					"--norc",
					"-c",
					fmt.Sprintf(`source "$1"; printf '%%s\n%%s\n' "$%s" "$$"`, marker),
					"_",
					scriptPath,
				)
				if shell == "zsh" {
					command = exec.Command(
						shellPath,
						"-f",
						"-c",
						fmt.Sprintf(`source "$1"; printf '%%s\n%%s\n' "$%s" "$$"`, marker),
						"_",
						scriptPath,
					)
				}
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("source %s script: %v\n%s", shell, err, output)
			}
			lines := strings.Fields(string(output))
			if len(lines) != 2 || lines[0] != lines[1] {
				t.Fatalf("%s marker and shell PID = %q, want equal values", shell, output)
			}
		})
	}
}

func TestShellInitNegativeGateControlsForkButNotAttributionDecision(t *testing.T) {
	unsetEnvironmentForTest(t, provider.HeadroomProjectEnvironment)
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "agentdeck.log")
	agentdeckPath := filepath.Join(bin, "agentdeck")
	codexPath := filepath.Join(bin, "codex")
	if err := os.WriteFile(agentdeckPath, []byte(`#!/bin/sh
printf 'called\n' >>"$AGENTDECK_TEST_LOG"
printf '%s' "${FAKE_PROJECT_VALUE-}"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexPath, []byte(`#!/bin/sh
printf '%s\n' "${HEADROOM_PROJECT-}"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	gatePath := filepath.Join(root, provider.ProjectAttributionGateFilename)
	scriptPath := filepath.Join(root, "agentdeck.bash")
	if err := os.WriteFile(scriptPath, []byte(shellInitScript("bash", "", gatePath)), 0o600); err != nil {
		t.Fatal(err)
	}

	runCodex := func(projectValue string) string {
		t.Helper()
		command := exec.Command("bash", "--noprofile", "--norc", "-c", `source "$1"; codex`, "_", scriptPath)
		command.Env = append(
			os.Environ(),
			"PATH="+bin+":"+os.Getenv("PATH"),
			"AGENTDECK_TEST_LOG="+logPath,
			"FAKE_PROJECT_VALUE="+projectValue,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("run gated wrapper: %v\n%s", err, output)
		}
		return string(output)
	}

	if output := runCodex("project"); output != "\n" {
		t.Fatalf("missing gate output = %q, want unchanged client", output)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("missing gate agentdeck log = %v, want no fork", err)
	}

	if err := os.WriteFile(gatePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if output := runCodex(""); output != "\n" {
		t.Fatalf("stale gate output = %q, want resolver to decide no attribution", output)
	}
	if calls, err := os.ReadFile(logPath); err != nil || string(calls) != "called\n" {
		t.Fatalf("stale gate calls = %q, %v", calls, err)
	}

	if output := runCodex("project"); output != "project\n" {
		t.Fatalf("eligible gate output = %q, want attributed client", output)
	}
	if calls, err := os.ReadFile(logPath); err != nil || string(calls) != "called\ncalled\n" {
		t.Fatalf("eligible gate calls = %q, %v", calls, err)
	}
}

func TestShellInitRejectsUnsupportedShell(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"shell-init", "powershell"}, strings.NewReader(""), &stdout)
	if err == nil || !strings.Contains(err.Error(), `unsupported shell "powershell"`) {
		t.Fatalf("shell-init powershell error = %v, want unsupported shell", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("shell-init powershell stdout = %q, want empty", stdout.String())
	}
}

func TestShellInitSyntaxErrorUsesStableJSONEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--format", "json", "shell-init", "powershell"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("shell-init JSON error exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("shell-init JSON error stdout = %q, want empty", stdout.String())
	}
	var envelope struct {
		Command string `json:"command"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode shell-init JSON error %q: %v", stderr.String(), err)
	}
	if envelope.Command != "shell-init" || envelope.Error.Code != "invalid_argument" {
		t.Fatalf("shell-init JSON error = %#v", envelope)
	}
}

func TestShellInitProjectEnvironmentRequiresHeadroomSelection(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "my+project")
	if err := os.Mkdir(fixture, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fixture)
	unsetEnvironmentForTest(t, provider.HeadroomProjectEnvironment)

	stateDir := filepath.Join(t.TempDir(), "state")
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", stateDir, "shell-init", "--project-environment", "codex"}, strings.NewReader(""), &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("resolver without a Headroom selection = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("resolver without state created %q: %v", stateDir, err)
	}
}

func TestShellInitProjectEnvironmentUsesCurrentDirectoryAndHonorsUserValues(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "c++")
	if err := os.Mkdir(fixture, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fixture)
	stateDir := filepath.Join(t.TempDir(), "state")
	configureHeadroomSelections(t, stateDir)
	resolve := func(client string, stdout *bytes.Buffer) error {
		return run([]string{"--state-dir", stateDir, "shell-init", "--project-environment", client}, strings.NewReader(""), stdout)
	}

	t.Run("codex derives wire value", func(t *testing.T) {
		unsetEnvironmentForTest(t, provider.HeadroomProjectEnvironment)
		var stdout bytes.Buffer
		if err := resolve("codex", &stdout); err != nil {
			t.Fatal(err)
		}
		if got := stdout.String(); got != "c%2B%2B" {
			t.Fatalf("Codex project environment = %q, want c%%2B%%2B", got)
		}
	})

	t.Run("codex user value wins", func(t *testing.T) {
		t.Setenv(provider.HeadroomProjectEnvironment, "user-value")
		var stdout bytes.Buffer
		if err := resolve("codex", &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("Codex resolver overwrote user value with %q", stdout.String())
		}
	})

	t.Run("claude preserves unrelated headers", func(t *testing.T) {
		t.Setenv(provider.ClaudeCustomHeadersEnvironment, "Other-Header: keep")
		var stdout bytes.Buffer
		if err := resolve("claude", &stdout); err != nil {
			t.Fatal(err)
		}
		want := "Other-Header: keep\n" + provider.HeadroomProjectHeader + ": c%2B%2B"
		if got := stdout.String(); got != want {
			t.Fatalf("Claude project environment = %q, want %q", got, want)
		}
	})

	t.Run("claude user header wins", func(t *testing.T) {
		t.Setenv(provider.ClaudeCustomHeadersEnvironment, "x-headroom-project: user-value")
		var stdout bytes.Buffer
		if err := resolve("claude", &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("Claude resolver overwrote user header with %q", stdout.String())
		}
	})
}

func TestShellCommandSurfaceAndHiddenCompatibility(t *testing.T) {
	var rootHelp bytes.Buffer
	if err := run([]string{"--help"}, strings.NewReader(""), &rootHelp); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rootHelp.String(), "shell-init") {
		t.Fatalf("root help exposes hidden shell-init:\n%s", rootHelp.String())
	}
	if !strings.Contains(rootHelp.String(), "shell ") {
		t.Fatalf("root help does not expose shell lifecycle:\n%s", rootHelp.String())
	}

	var shellHelp bytes.Buffer
	if err := run([]string{"shell", "--help"}, strings.NewReader(""), &shellHelp); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"setup",
		"status",
		"remove",
		"env",
		"every shell in use by default",
	} {
		if !strings.Contains(shellHelp.String(), want) {
			t.Errorf("shell help missing %q:\n%s", want, shellHelp.String())
		}
	}
	for _, operation := range []string{"setup", "status", "remove"} {
		var help bytes.Buffer
		if err := run([]string{"shell", operation, "--help"}, strings.NewReader(""), &help); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"--shell", "--rc", "Arguments:"} {
			if !strings.Contains(help.String(), want) {
				t.Errorf("shell %s help missing %q:\n%s", operation, want, help.String())
			}
		}
	}

	var initHelp bytes.Buffer
	if err := run([]string{"shell-init", "--help"}, strings.NewReader(""), &initHelp); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"stdout",
		"changes no shell state",
		"writes no file",
		"eligible Headroom route",
		"invoke the real client unchanged",
		"agentdeck shell setup",
		`eval "$(agentdeck shell-init bash)"`,
		"agentdeck shell-init fish | source",
		`eval "$(agentdeck shell-init zsh)"`,
	} {
		if !strings.Contains(initHelp.String(), want) {
			t.Errorf("shell-init help missing %q:\n%s", want, initHelp.String())
		}
	}
}

func TestShellInitAndEnvironmentResolverRejectNonTextOutput(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "json", "shell-init", "zsh"},
		{"--format", "json", "shell", "env", "codex"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := execute(args, strings.NewReader(""), &stdout, &stderr); exit != 2 {
			t.Fatalf("%v exit = %d, want 2", args, exit)
		}
		if stdout.Len() != 0 {
			t.Fatalf("%v stdout = %q, want empty", args, stdout.String())
		}
		var envelope struct {
			Command string `json:"command"`
			Error   struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %v JSON error %q: %v", args, stderr.String(), err)
		}
		if envelope.Error.Code != "invalid_argument" || !strings.Contains(envelope.Error.Message, "requires text format") {
			t.Fatalf("%v JSON error = %#v", args, envelope)
		}
	}
}

func TestShellEnvMatchesHiddenProjectEnvironmentResolver(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "c++")
	if err := os.Mkdir(fixture, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fixture)
	unsetEnvironmentForTest(t, provider.HeadroomProjectEnvironment)
	unsetEnvironmentForTest(t, provider.ClaudeCustomHeadersEnvironment)

	stateDir := filepath.Join(t.TempDir(), "state")
	configureHeadroomSelections(t, stateDir)
	for _, client := range []string{"codex", "claude"} {
		t.Run(client, func(t *testing.T) {
			var public, compatibility bytes.Buffer
			if err := run([]string{"--state-dir", stateDir, "shell", "env", client}, strings.NewReader(""), &public); err != nil {
				t.Fatal(err)
			}
			if err := run([]string{"--state-dir", stateDir, "shell-init", "--project-environment", client}, strings.NewReader(""), &compatibility); err != nil {
				t.Fatal(err)
			}
			if public.String() != compatibility.String() {
				t.Fatalf("shell env = %q, compatibility resolver = %q", public.String(), compatibility.String())
			}
			if public.Len() == 0 {
				t.Fatal("eligible resolver returned no value")
			}
		})
	}
}

func TestShellEnvUnsupportedClientUsesStableJSONEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--format", "json", "shell", "env", "cursor"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("shell env unsupported client exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("shell env unsupported client stdout = %q, want empty", stdout.String())
	}
	var envelope struct {
		Command string `json:"command"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode shell env JSON error %q: %v", stderr.String(), err)
	}
	if envelope.Command != "shell.env" || envelope.Error.Code != "invalid_argument" {
		t.Fatalf("shell env JSON error = %#v", envelope)
	}
}

func TestResolveShellTarget(t *testing.T) {
	for _, test := range []struct {
		name      string
		args      []string
		shellFlag string
		rc        string
		want      shellTarget
		wantError string
	}{
		{name: "all shells", want: shellTarget{}},
		{name: "positional", args: []string{"zsh"}, want: shellTarget{shell: "zsh"}},
		{name: "flag", shellFlag: "zsh", want: shellTarget{shell: "zsh"}},
		{name: "custom rc", shellFlag: "fish", rc: "/tmp/config.fish", want: shellTarget{shell: "fish", rc: "/tmp/config.fish"}},
		{name: "both forms", args: []string{"zsh"}, shellFlag: "zsh", wantError: "not both"},
		{name: "unsupported", args: []string{"powershell"}, wantError: `unsupported shell "powershell"`},
		{name: "rc without shell", rc: "/tmp/rc", wantError: "--rc requires exactly one shell"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveShellTarget(test.args, test.shellFlag, test.rc)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("resolveShellTarget() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("resolveShellTarget() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestShellLifecycleSelectionErrorsAreInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"shell", "setup", "--rc", "/tmp/rc"},
		{"shell", "status", "zsh", "--shell", "zsh"},
		{"shell", "remove", "powershell"},
	} {
		var stdout, stderr bytes.Buffer
		exit := execute(append([]string{"--format", "json"}, args...), strings.NewReader(""), &stdout, &stderr)
		if exit != 2 {
			t.Fatalf("%v exit = %d, want 2", args, exit)
		}
		if stdout.Len() != 0 {
			t.Fatalf("%v stdout = %q, want empty", args, stdout.String())
		}
		var envelope struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %v JSON error %q: %v", args, stderr.String(), err)
		}
		if envelope.Error.Code != "invalid_argument" {
			t.Fatalf("%v JSON error = %#v", args, envelope)
		}
	}
}

func TestShellStatusTextAndJSONReportConfigurationActivationAndEligibilityOnce(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, "state")
	rc := filepath.Join(home, ".zshrc")
	manager := shellconfig.New(shellconfig.Environment{Home: home, StateRoot: stateDir})
	if summary, err := manager.Setup(shellconfig.Request{Shell: shellconfig.ShellZsh, RC: rc}); err != nil {
		t.Fatal(err)
	} else if summary.HasFailures() {
		t.Fatalf("setup failed: %#v", summary.Results)
	}
	configureHeadroomSelections(t, stateDir)

	oldHome := userHomeDir
	oldDetect := detectInvokingShell
	oldParentPID := parentProcessID
	userHomeDir = func() (string, error) { return home, nil }
	detectInvokingShell = func() (shellconfig.Invocation, error) {
		return shellconfig.Invocation{Shell: shellconfig.ShellZsh}, nil
	}
	parentProcessID = func() int { return 4242 }
	t.Cleanup(func() {
		userHomeDir = oldHome
		detectInvokingShell = oldDetect
		parentProcessID = oldParentPID
	})
	t.Setenv(shellconfig.ActivationMarkerName(shellconfig.ShellZsh), "4242")
	t.Setenv(provider.HeadroomProjectEnvironment, "secret-project-value")

	var jsonOut, jsonErr bytes.Buffer
	exit := execute(
		[]string{"--state-dir", stateDir, "--format", "json", "shell", "status"},
		strings.NewReader(""),
		&jsonOut,
		&jsonErr,
	)
	if exit != 0 {
		t.Fatalf("JSON exit = %d; stdout=%s stderr=%s", exit, jsonOut.String(), jsonErr.String())
	}
	var envelope struct {
		Command string            `json:"command"`
		Data    shellStatusOutput `json:"data"`
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON output %q: %v", jsonOut.String(), err)
	}
	if envelope.Command != "shell.status" {
		t.Fatalf("JSON envelope = %#v", envelope)
	}
	activeShells := 0
	for _, status := range envelope.Data.Shells {
		if status.Activation == shellconfig.ActivationActive {
			activeShells++
			if status.Shell != shellconfig.ShellZsh ||
				status.Configuration != shellconfig.ConfigurationConfigured {
				t.Errorf("active status = %#v", status)
			}
		}
	}
	if len(envelope.Data.Shells) != 1 ||
		envelope.Data.Shells[0].Shell != shellconfig.ShellZsh ||
		activeShells != 1 {
		t.Fatalf("JSON shells = %#v, want only active invoking zsh", envelope.Data.Shells)
	}
	if len(envelope.Data.Eligibility) != 2 {
		t.Fatalf("JSON eligibility = %#v, want two clients", envelope.Data.Eligibility)
	}
	for _, eligibility := range envelope.Data.Eligibility {
		if eligibility.Reason != provider.ProjectRouteEligible {
			t.Errorf("%s eligibility = %#v", eligibility.Client, eligibility)
		}
	}
	if !envelope.Data.Gate.Required ||
		!envelope.Data.Gate.Present ||
		!envelope.Data.Gate.Consistent ||
		envelope.Data.Gate.Error != "" {
		t.Fatalf("JSON attribution gate = %#v, want ready", envelope.Data.Gate)
	}

	fishRC := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishRC), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishRC, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	jsonOut.Reset()
	jsonErr.Reset()
	exit = execute(
		[]string{"--state-dir", stateDir, "--format", "json", "shell", "status"},
		strings.NewReader(""),
		&jsonOut,
		&jsonErr,
	)
	if exit != 0 {
		t.Fatalf("multi-shell JSON exit = %d; stdout=%s stderr=%s", exit, jsonOut.String(), jsonErr.String())
	}
	envelope = struct {
		Command string            `json:"command"`
		Data    shellStatusOutput `json:"data"`
	}{}
	if err := json.Unmarshal(jsonOut.Bytes(), &envelope); err != nil {
		t.Fatalf("decode multi-shell JSON output %q: %v", jsonOut.String(), err)
	}
	if len(envelope.Data.Shells) != 2 ||
		envelope.Data.Shells[0].Shell != shellconfig.ShellZsh ||
		envelope.Data.Shells[1].Shell != shellconfig.ShellFish {
		t.Fatalf("multi-shell JSON shells = %#v, want zsh and existing fish", envelope.Data.Shells)
	}

	missingBash := filepath.Join(home, "missing-bash")
	jsonOut.Reset()
	jsonErr.Reset()
	exit = execute(
		[]string{
			"--state-dir", stateDir,
			"--format", "json",
			"shell", "status", "bash", "--rc", missingBash,
		},
		strings.NewReader(""),
		&jsonOut,
		&jsonErr,
	)
	if exit != 0 {
		t.Fatalf("explicit missing JSON exit = %d; stdout=%s stderr=%s", exit, jsonOut.String(), jsonErr.String())
	}
	envelope = struct {
		Command string            `json:"command"`
		Data    shellStatusOutput `json:"data"`
	}{}
	if err := json.Unmarshal(jsonOut.Bytes(), &envelope); err != nil {
		t.Fatalf("decode explicit missing JSON output %q: %v", jsonOut.String(), err)
	}
	if len(envelope.Data.Shells) != 1 ||
		envelope.Data.Shells[0].Shell != shellconfig.ShellBash ||
		envelope.Data.Shells[0].Path != missingBash ||
		envelope.Data.Shells[0].Configuration != shellconfig.ConfigurationAbsent {
		t.Fatalf("explicit missing JSON shells = %#v, want absent bash", envelope.Data.Shells)
	}

	var textOut, textErr bytes.Buffer
	exit = execute(
		[]string{"--state-dir", stateDir, "shell", "status"},
		strings.NewReader(""),
		&textOut,
		&textErr,
	)
	if exit != 0 {
		t.Fatalf("text exit = %d; stdout=%s stderr=%s", exit, textOut.String(), textErr.String())
	}
	if strings.Count(textOut.String(), "Attribution:") != 1 ||
		!strings.Contains(textOut.String(), "codex eligible") ||
		!strings.Contains(textOut.String(), "claude eligible") ||
		!strings.Contains(textOut.String(), "Attribution gate: ready") ||
		!strings.Contains(textOut.String(), "session: active") {
		t.Fatalf("text status = %q", textOut.String())
	}
	if strings.Contains(textOut.String(), "secret-project-value") {
		t.Fatalf("text status exposed project value: %q", textOut.String())
	}
}

func TestShellStatusReportsUnreadableStateAsUndeterminedWithoutCreatingIt(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "missing-state")
	rc := filepath.Join(root, "missing-zshrc")

	var stdout, stderr bytes.Buffer
	exit := execute(
		[]string{"--state-dir", stateDir, "shell", "status", "zsh", "--rc", rc},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if exit != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	if strings.Count(stdout.String(), "undetermined (") != 3 ||
		!strings.Contains(stdout.String(), "Attribution gate: undetermined (") {
		t.Fatalf("status = %q, want per-client and gate undetermined diagnostics", stdout.String())
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("status created missing state root: %v", err)
	}
}

func TestShellStatusReportsProjectAttributionGateMismatch(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, string)
		want    string
	}{
		{
			name: "missing while eligible",
			prepare: func(t *testing.T, stateDir string) {
				t.Helper()
				configureHeadroomSelections(t, stateDir)
				if err := os.Remove(provider.ProjectAttributionGatePath(stateDir)); err != nil {
					t.Fatal(err)
				}
			},
			want: "Attribution gate: missing (eligible route exists;",
		},
		{
			name: "stale while ineligible",
			prepare: func(t *testing.T, stateDir string) {
				t.Helper()
				database, err := store.Open(context.Background(), stateDir)
				if err != nil {
					t.Fatal(err)
				}
				if err := database.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(
					provider.ProjectAttributionGatePath(stateDir),
					nil,
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			},
			want: "Attribution gate: stale (no eligible route remains;",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			if err := os.Mkdir(home, 0o700); err != nil {
				t.Fatal(err)
			}
			stateDir := filepath.Join(root, "state")
			test.prepare(t, stateDir)

			oldHome := userHomeDir
			oldDetect := detectInvokingShell
			userHomeDir = func() (string, error) { return home, nil }
			detectInvokingShell = func() (shellconfig.Invocation, error) {
				return shellconfig.Invocation{Shell: shellconfig.ShellZsh}, nil
			}
			t.Cleanup(func() {
				userHomeDir = oldHome
				detectInvokingShell = oldDetect
			})

			var stdout, stderr bytes.Buffer
			exit := execute(
				[]string{
					"--state-dir", stateDir,
					"shell", "status", "zsh",
					"--rc", filepath.Join(home, ".zshrc"),
				},
				strings.NewReader(""),
				&stdout,
				&stderr,
			)
			if exit != 0 {
				t.Fatalf("exit = %d; stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("status = %q, want %q", stdout.String(), test.want)
			}
		})
	}
}

func TestShellSetupAndRemoveManageOnlyOwnedBlock(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, "state")
	rc := filepath.Join(home, ".zshrc")
	original := []byte("export USER_SETTING=kept")
	if err := os.WriteFile(rc, original, 0o640); err != nil {
		t.Fatal(err)
	}

	var setupOutput bytes.Buffer
	if err := run([]string{"--state-dir", stateDir, "shell", "setup", "zsh", "--rc", rc}, strings.NewReader(""), &setupOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(setupOutput.String(), "zsh: configured "+rc) {
		t.Fatalf("setup output = %q", setupOutput.String())
	}
	configured, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(configured, []byte("# >>> agentdeck shell integration >>>")) ||
		!bytes.HasPrefix(configured, append(append([]byte(nil), original...), '\n')) {
		t.Fatalf("configured startup file = %q", configured)
	}

	var jsonOutput bytes.Buffer
	if err := run([]string{"--state-dir", stateDir, "--format", "json", "shell", "setup", "--shell", "zsh", "--rc", rc}, strings.NewReader(""), &jsonOutput); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Command string              `json:"command"`
		Data    shellconfig.Summary `json:"data"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &envelope); err != nil {
		t.Fatalf("decode setup JSON %q: %v", jsonOutput.String(), err)
	}
	if envelope.Command != "shell.setup" ||
		len(envelope.Data.Results) != 1 ||
		envelope.Data.Results[0].Outcome != shellconfig.OutcomeUnchanged {
		t.Fatalf("setup JSON = %#v", envelope)
	}

	var removeOutput bytes.Buffer
	if err := run([]string{"--state-dir", stateDir, "shell", "remove", "zsh", "--rc", rc}, strings.NewReader(""), &removeOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(removeOutput.String(), "zsh: removed "+rc) {
		t.Fatalf("remove output = %q", removeOutput.String())
	}
	restored, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("remove restored %q, want %q", restored, original)
	}
}

func TestShellLifecycleRejectsNDJSONBeforeMutation(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	var stdout, stderr bytes.Buffer
	exit := execute(
		[]string{"--format", "ndjson", "shell", "setup", "zsh", "--rc", rc},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(rc); !os.IsNotExist(err) {
		t.Fatalf("startup file exists after rejected format: %v", err)
	}
}

func TestShellSetupReportsEveryShellAndReturnsFailureWhenOneIsRefused(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("zsh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fishRC := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishRC), 0o700); err != nil {
		t.Fatal(err)
	}
	fishTarget := filepath.Join(home, "fish-target")
	if err := os.WriteFile(fishTarget, []byte("fish\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fishTarget, fishRC); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	oldDetect := detectInvokingShell
	userHomeDir = func() (string, error) { return home, nil }
	detectInvokingShell = func() (shellconfig.Invocation, error) {
		return shellconfig.Invocation{Shell: shellconfig.ShellBash}, nil
	}
	t.Cleanup(func() {
		userHomeDir = oldHome
		detectInvokingShell = oldDetect
	})
	t.Setenv("ZDOTDIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--format", "json", "shell", "setup"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	var successEnvelope struct {
		Command string              `json:"command"`
		Data    shellconfig.Summary `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &successEnvelope); err != nil {
		t.Fatalf("decode result %q: %v", stdout.String(), err)
	}
	if successEnvelope.Command != "shell.setup" ||
		len(successEnvelope.Data.Results) != 3 ||
		!successEnvelope.Data.HasFailures() {
		t.Fatalf("result envelope = %#v", successEnvelope)
	}
	var errorEnvelope struct {
		Command string `json:"command"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &errorEnvelope); err != nil {
		t.Fatalf("decode error %q: %v", stderr.String(), err)
	}
	if errorEnvelope.Command != "shell.setup" || errorEnvelope.Error.Code != "runtime_error" {
		t.Fatalf("error envelope = %#v", errorEnvelope)
	}
	for _, path := range []string{filepath.Join(home, ".zshrc"), filepath.Join(home, ".bashrc")} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(contents, []byte("# >>> agentdeck shell integration >>>")) {
			t.Fatalf("%s was not configured:\n%s", path, contents)
		}
	}
}

func TestShellRemoveWithoutArgumentsDoesNotRequireInvokingShellDetection(t *testing.T) {
	home := t.TempDir()
	paths := []struct {
		shell shellconfig.Shell
		path  string
	}{
		{shell: shellconfig.ShellZsh, path: filepath.Join(home, ".zshrc")},
		{shell: shellconfig.ShellFish, path: filepath.Join(home, ".config", "fish", "config.fish")},
		{shell: shellconfig.ShellBash, path: filepath.Join(home, ".bash_profile")},
		{shell: shellconfig.ShellBash, path: filepath.Join(home, ".bashrc")},
	}
	manager := shellconfig.New(shellconfig.Environment{Home: home})
	for _, target := range paths {
		summary, err := manager.Setup(shellconfig.Request{Shell: target.shell, RC: target.path})
		if err != nil {
			t.Fatal(err)
		}
		if summary.HasFailures() {
			t.Fatalf("setup %s failed: %#v", target.path, summary.Results)
		}
	}

	oldHome := userHomeDir
	oldDetect := detectInvokingShell
	userHomeDir = func() (string, error) { return home, nil }
	detectCalls := 0
	detectInvokingShell = func() (shellconfig.Invocation, error) {
		detectCalls++
		return shellconfig.Invocation{}, errors.New("synthetic invoking shell failure")
	}
	t.Cleanup(func() {
		userHomeDir = oldHome
		detectInvokingShell = oldDetect
	})
	t.Setenv("ZDOTDIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	var stdout, stderr bytes.Buffer
	exit := execute(
		[]string{"shell", "remove"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if exit != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	if detectCalls != 0 {
		t.Fatalf("detectInvokingShell called %d times, want 0", detectCalls)
	}
	for _, target := range paths {
		if !strings.Contains(stdout.String(), target.path) {
			t.Errorf("stdout = %q, want report for %s", stdout.String(), target.path)
		}
		contents, err := os.ReadFile(target.path)
		if err != nil {
			t.Fatal(err)
		}
		if len(contents) != 0 {
			t.Errorf("%s contents = %q, want empty", target.path, contents)
		}
	}
}

func configureHeadroomSelections(t *testing.T, stateDir string) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	service := provider.Service{Store: database, StateRoot: stateDir}
	if _, _, err = service.SetWrapper(ctx, provider.OfficialProviderName, "https://wrapper.example", provider.WrapperKindHeadroom, false); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(stateDir)
	codexConfig := filepath.Join(root, "config.toml")
	if err = os.WriteFile(codexConfig, []byte("model = \"synthetic\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, provider.OfficialProviderName, provider.ClientCodex, "", codexConfig, filepath.Join(root, "codex.backup.toml"), true); err != nil {
		t.Fatal(err)
	}
	claudeConfig := filepath.Join(root, "settings.json")
	if err = os.WriteFile(claudeConfig, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = service.UseCredential(ctx, provider.OfficialProviderName, provider.ClientClaude, "", claudeConfig, filepath.Join(root, "claude.backup.json"), true); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
}

func unsetEnvironmentForTest(t *testing.T, name string) {
	t.Helper()
	value, found := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if found {
			_ = os.Setenv(name, value)
			return
		}
		_ = os.Unsetenv(name)
	})
}
