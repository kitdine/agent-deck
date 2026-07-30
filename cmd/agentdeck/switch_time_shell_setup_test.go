package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/shellconfig"
)

func TestProviderUseViaAutomaticallyConfiguresEveryShellInUseOnce(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)
	fishRC := filepath.Join(home, ".config", "fish", "config.fish")
	bashRC := filepath.Join(home, ".bashrc")
	writeStartupFile(t, fishRC, "set -gx EXISTING fish\n")
	writeStartupFile(t, bashRC, "export EXISTING=bash\n")

	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)
	_, stderr, exit := runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("eligible switch exit = %d: %s", exit, stderr)
	}

	for shell, path := range map[string]string{
		"zsh":  filepath.Join(home, ".zshrc"),
		"fish": fishRC,
		"bash": bashRC,
	} {
		contents := readStartupFile(t, path)
		if !strings.Contains(contents, "# >>> agentdeck shell integration >>>") {
			t.Fatalf("%s startup file was not configured: %q", shell, contents)
		}
		if !strings.Contains(stderr, shell+": configured "+path) {
			t.Fatalf("%s setup result missing from stderr %q", shell, stderr)
		}
	}
	if !strings.Contains(
		stderr,
		`new shell sessions are covered; activate current zsh session: eval "$(agentdeck --state-dir `+
			shellQuote(state)+` shell-init zsh)"`,
	) {
		t.Fatalf("activation guidance missing from stderr %q", stderr)
	}

	before := map[string]string{
		filepath.Join(home, ".zshrc"): readStartupFile(t, filepath.Join(home, ".zshrc")),
		fishRC:                        readStartupFile(t, fishRC),
		bashRC:                        readStartupFile(t, bashRC),
	}
	_, secondStderr, exit := runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("second eligible switch exit = %d: %s", exit, secondStderr)
	}
	if strings.Contains(secondStderr, ": configured ") ||
		strings.Contains(secondStderr, "new shell sessions are covered") {
		t.Fatalf("second switch reported shell configuration: %q", secondStderr)
	}
	for path, want := range before {
		if got := readStartupFile(t, path); got != want {
			t.Fatalf("second switch changed %s:\nwant %q\ngot  %q", path, want, got)
		}
	}
}

func TestDefaultAndCustomShellStateRootFormsArePortableAndIdempotent(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	customState := filepath.Join(home, "state'custom")
	tests := []struct {
		name           string
		stateArgs      []string
		stateRoot      string
		wantBody       string
		wantActivation string
	}{
		{
			name:      "default root remains portable",
			stateArgs: nil,
			wantBody: "command -v agentdeck >/dev/null 2>&1 && " +
				"eval \"$(command agentdeck shell-init zsh)\"\n",
			wantActivation: `eval "$(agentdeck shell-init zsh)"`,
		},
		{
			name:      "custom root remains pinned",
			stateArgs: []string{"--state-dir", customState},
			stateRoot: customState,
			wantBody: "command -v agentdeck >/dev/null 2>&1 && " +
				"eval \"$(command agentdeck --state-dir " + shellQuote(customState) +
				" shell-init zsh)\"\n",
			wantActivation: `eval "$(agentdeck --state-dir ` + shellQuote(customState) +
				` shell-init zsh)"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc := filepath.Join(home, strings.ReplaceAll(test.name, " ", "-")+".zshrc")
			setupArgs := append(append([]string{}, test.stateArgs...), "shell", "setup", "zsh", "--rc", rc)
			if _, stderr, exit := runRouteCommand(t, setupArgs...); exit != 0 {
				t.Fatalf("first setup exit = %d: %s", exit, stderr)
			}
			first := readStartupFile(t, rc)
			const separatorRecord = "# managed-separator-added: false\n"
			bodyStart := strings.Index(first, separatorRecord)
			if bodyStart < 0 {
				t.Fatalf("managed separator record missing:\n%s", first)
			}
			bodyStart += len(separatorRecord)
			bodyEnd := strings.Index(first[bodyStart:], "# <<< agentdeck shell integration <<<")
			if bodyEnd < 0 {
				t.Fatalf("managed end marker missing:\n%s", first)
			}
			if body := first[bodyStart : bodyStart+bodyEnd]; body != test.wantBody {
				t.Fatalf("managed body = %q, want %q", body, test.wantBody)
			}

			if _, stderr, exit := runRouteCommand(t, setupArgs...); exit != 0 {
				t.Fatalf("second setup exit = %d: %s", exit, stderr)
			}
			if second := readStartupFile(t, rc); second != first {
				t.Fatalf("idempotent setup changed startup file:\nfirst %q\nsecond %q", first, second)
			}

			var generated strings.Builder
			initArgs := append(append([]string{}, test.stateArgs...), "shell-init", "zsh")
			if err := run(initArgs, strings.NewReader(""), &generated); err != nil {
				t.Fatalf("shell-init: %v", err)
			}
			if test.stateRoot == "" {
				if strings.Contains(generated.String(), "--state-dir") {
					t.Fatalf("default shell-init pinned state root:\n%s", generated.String())
				}
			} else if !strings.Contains(
				generated.String(),
				"command agentdeck --state-dir "+shellQuote(test.stateRoot),
			) {
				t.Fatalf("custom shell-init omitted state root:\n%s", generated.String())
			}
			if activation := shellActivationCommand(shellconfig.ShellZsh, test.stateRoot); activation != test.wantActivation {
				t.Fatalf("activation command = %q, want %q", activation, test.wantActivation)
			}
		})
	}
}

func TestProviderUseViaBindsCustomStateRootAcrossManagedActivationAndWrappers(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	unsetEnvironmentForTest(t, provider.HeadroomProjectEnvironment)
	unsetEnvironmentForTest(t, provider.ClaudeCustomHeadersEnvironment)
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)

	state, config := newRouteSurfaceFixture(t)
	customState := filepath.Join(filepath.Dir(state), "state'custom")
	if err := os.Rename(state, customState); err != nil {
		t.Fatal(err)
	}
	configureHeadroomWrapper(t, customState)
	_, stderr, exit := runEligibleProviderSwitch(t, customState, config)
	if exit != 0 {
		t.Fatalf("eligible switch exit = %d: %s", exit, stderr)
	}

	rc := filepath.Join(home, ".zshrc")
	managed := readStartupFile(t, rc)
	quotedState := shellQuote(customState)
	if want := "command agentdeck --state-dir " + quotedState + " shell-init zsh"; !strings.Contains(managed, want) {
		t.Fatalf("managed block does not bind custom state root %q:\n%s", customState, managed)
	}
	if want := `eval "$(agentdeck --state-dir ` + quotedState + ` shell-init zsh)"`; !strings.Contains(stderr, want) {
		t.Fatalf("activation guidance does not bind custom state root %q: %s", customState, stderr)
	}

	defaultState := filepath.Join(home, ".agentdeck")
	if err := os.MkdirAll(defaultState, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"codex", "claude"} {
		if err := os.WriteFile(
			filepath.Join(customState, client+".project"),
			[]byte("custom-"+client),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(defaultState, client+".project"),
			[]byte("default-"+client),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	customGate := provider.ProjectAttributionGatePath(customState)
	if _, err := os.Stat(customGate); err != nil {
		t.Fatalf("custom state gate is not present after eligible switch: %v", err)
	}

	var generated strings.Builder
	if err := run(
		[]string{"--state-dir", customState, "shell-init", "zsh"},
		strings.NewReader(""),
		&generated,
	); err != nil {
		t.Fatalf("generate custom-state shell wrapper: %v", err)
	}
	initPath := filepath.Join(root, "generated.zsh")
	if err := os.WriteFile(initPath, []byte(generated.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "resolver.log")
	writeExecutable := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("agentdeck", `#!/bin/sh
state="$HOME/.agentdeck"
if [ "$1" = "--state-dir" ]; then
    state="$2"
    shift 2
fi
if [ "$1" = "shell-init" ] && [ "$2" = "zsh" ]; then
    cat "$AGENTDECK_TEST_INIT"
elif [ "$1" = "shell-init" ] && [ "$2" = "--project-environment" ]; then
    printf '%s:%s\n' "$3" "$state" >>"$AGENTDECK_TEST_LOG"
    cat "$state/$3.project"
else
    exit 64
fi
`)
	writeExecutable("codex", `#!/bin/sh
printf 'codex=%s\n' "${HEADROOM_PROJECT-}"
`)
	writeExecutable("claude", `#!/bin/sh
printf 'claude=%s\n' "${ANTHROPIC_CUSTOM_HEADERS-}"
`)

	sourceAndRun := func() string {
		t.Helper()
		command := exec.Command(
			zshPath,
			"-f",
			"-c",
			`source "$1"; codex; claude`,
			"_",
			rc,
		)
		command.Env = append(
			os.Environ(),
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"AGENTDECK_TEST_INIT="+initPath,
			"AGENTDECK_TEST_LOG="+logPath,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("source managed block and invoke wrappers: %v\n%s", err, output)
		}
		return string(output)
	}
	if output := sourceAndRun(); output != "codex=custom-codex\nclaude=custom-claude\n" {
		t.Fatalf("custom-state wrapper output = %q", output)
	}
	resolverLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantLog := "codex:" + customState + "\nclaude:" + customState + "\n"
	if string(resolverLog) != wantLog {
		t.Fatalf("resolver state roots = %q, want %q", resolverLog, wantLog)
	}

	if err := os.Remove(customGate); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider.ProjectAttributionGatePath(defaultState), []byte("default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if output := sourceAndRun(); output != "codex=\nclaude=\n" {
		t.Fatalf("wrappers used default-state gate after custom gate removal: %q", output)
	}
	if resolverLog, err = os.ReadFile(logPath); err != nil {
		t.Fatal(err)
	}
	if len(resolverLog) != 0 {
		t.Fatalf("resolver ran through default gate: %q", resolverLog)
	}
}

func TestProviderUseViaNonInteractiveModesDoNotConfigureShell(t *testing.T) {
	tests := []struct {
		name        string
		prefix      []string
		interactive bool
		wantExit    int
	}{
		{name: "non-TTY stderr", interactive: false, wantExit: 0},
		{name: "quiet", prefix: []string{"--quiet"}, interactive: true, wantExit: 0},
		{name: "json", prefix: []string{"--format", "json"}, interactive: true, wantExit: 0},
		{name: "ndjson", prefix: []string{"--format", "ndjson"}, interactive: true, wantExit: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
			stubShellSetupTerminal(t, test.interactive)
			state, config := newRouteSurfaceFixture(t)
			configureHeadroomWrapper(t, state)
			args := append([]string{"--state-dir", state}, test.prefix...)
			args = append(args,
				"provider", "use", "example",
				"--client", "codex",
				"--config-path", config,
				"--via",
			)
			stdout, stderr, exit := runRouteCommand(t, args...)
			if exit != test.wantExit {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q", exit, test.wantExit, stdout, stderr)
			}
			if _, err := os.Lstat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
				t.Fatalf("non-interactive switch created startup file: %v", err)
			}
			if strings.Contains(stdout, "# >>> agentdeck shell integration >>>") ||
				strings.Contains(stdout, "agentdeck shell-init") {
				t.Fatalf("provider use output exposed shell wrapper text: %q", stdout)
			}
			if test.wantExit == 0 && test.name != "quiet" &&
				!strings.Contains(stderr, "agentdeck shell setup") {
				t.Fatalf("non-interactive switch did not fall back advisory: %q", stderr)
			}
		})
	}
}

func TestProviderUseNoShellSetupSuppressesOnlyCurrentInvocation(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)
	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)

	_, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
		"--no-shell-setup",
	)
	if exit != 0 {
		t.Fatalf("suppressed switch exit = %d: %s", exit, stderr)
	}
	if _, err := os.Lstat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("suppressed switch created startup file: %v", err)
	}
	if !strings.Contains(stderr, "agentdeck shell setup") {
		t.Fatalf("suppressed switch did not fall back advisory: %q", stderr)
	}

	_, stderr, exit = runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("following switch exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(
		readStartupFile(t, filepath.Join(home, ".zshrc")),
		"agentdeck --state-dir "+shellQuote(state)+" shell-init zsh",
	) {
		t.Fatal("invocation-only suppression persisted into following switch")
	}
}

func TestShellRemoveDeclinesAutomaticSetupUntilExplicitSetup(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)
	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)
	zshRC := filepath.Join(home, ".zshrc")

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "shell", "setup", "zsh"); exit != 0 {
		t.Fatalf("explicit setup exit = %d: %s", exit, stderr)
	}
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "shell", "remove", "zsh"); exit != 0 {
		t.Fatalf("explicit remove exit = %d: %s", exit, stderr)
	}
	_, stderr, exit := runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("declined switch exit = %d: %s", exit, stderr)
	}
	if strings.Contains(
		readStartupFile(t, zshRC),
		"agentdeck --state-dir "+shellQuote(state)+" shell-init zsh",
	) {
		t.Fatal("automatic setup ignored persisted decline")
	}

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "shell", "setup", "zsh"); exit != 0 {
		t.Fatalf("second explicit setup exit = %d: %s", exit, stderr)
	}
	if err := os.WriteFile(zshRC, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, exit = runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("switch after explicit setup exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(
		readStartupFile(t, zshRC),
		"agentdeck --state-dir "+shellQuote(state)+" shell-init zsh",
	) {
		t.Fatal("explicit setup did not clear persisted decline")
	}
}

func TestAutomaticShellSetupFailureRollsBackAndKeepsSwitchSuccessful(t *testing.T) {
	home := t.TempDir()
	zshRoot := t.TempDir()
	t.Setenv("ZDOTDIR", zshRoot)
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)
	bashRC := filepath.Join(home, ".bashrc")
	const bashOriginal = "export PRESERVE=1\n"
	writeStartupFile(t, bashRC, bashOriginal)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)
	_, stderr, exit := runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("switch with unwritable target exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(stderr, "agentdeck shell setup") {
		t.Fatalf("write failure did not fall back advisory: %q", stderr)
	}
	if _, err := os.Lstat(filepath.Join(zshRoot, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("successful earlier target was not rolled back: %v", err)
	}
	if got := readStartupFile(t, bashRC); got != bashOriginal {
		t.Fatalf("unwritable target changed:\nwant %q\ngot  %q", bashOriginal, got)
	}
}

func TestAutomaticShellSetupRejectsTamperedBlockWithoutWriting(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	stubInteractiveShellSetup(t)
	zshRC := filepath.Join(home, ".zshrc")
	const tampered = "# >>> agentdeck shell integration >>>\n# tampered\n# <<< agentdeck shell integration <<<\n"
	writeStartupFile(t, zshRC, tampered)
	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)

	_, stderr, exit := runEligibleProviderSwitch(t, state, config)
	if exit != 0 {
		t.Fatalf("tampered switch exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(stderr, "agentdeck shell setup") {
		t.Fatalf("tampered block did not fall back advisory: %q", stderr)
	}
	if got := readStartupFile(t, zshRC); got != tampered {
		t.Fatalf("tampered block was changed:\nwant %q\ngot  %q", tampered, got)
	}
}

func stubInteractiveShellSetup(t *testing.T) {
	t.Helper()
	stubShellSetupTerminal(t, true)
}

func stubShellSetupTerminal(t *testing.T, terminal bool) {
	t.Helper()
	old := shellSetupIsTerminal
	shellSetupIsTerminal = func(io.Writer) bool { return terminal }
	t.Cleanup(func() { shellSetupIsTerminal = old })
}

func runEligibleProviderSwitch(t *testing.T, state, config string) (stdout, stderr string, exit int) {
	t.Helper()
	return runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
	)
}

func writeStartupFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readStartupFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
