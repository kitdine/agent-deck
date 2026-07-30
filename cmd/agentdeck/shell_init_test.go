package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
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

func TestShellLifecycleSurfaceDoesNotReportSuccessBeforeHandlersExist(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		args    []string
	}{
		{name: "setup", command: "shell.setup", args: []string{"shell", "setup", "--shell", "zsh"}},
		{name: "status", command: "shell.status", args: []string{"shell", "status", "zsh"}},
		{name: "remove", command: "shell.remove", args: []string{"shell", "remove"}},
	} {
		for _, format := range []string{"text", "json"} {
			t.Run(test.name+"/"+format, func(t *testing.T) {
				args := append([]string{"--format", format}, test.args...)
				var stdout, stderr bytes.Buffer
				exit := execute(args, strings.NewReader(""), &stdout, &stderr)
				if exit != 1 {
					t.Fatalf("%v exit = %d, want 1", args, exit)
				}
				if stdout.Len() != 0 {
					t.Fatalf("%v stdout = %q, want empty", args, stdout.String())
				}
				if format == "text" {
					if !strings.Contains(stderr.String(), "not available in this build") {
						t.Fatalf("%v stderr = %q, want unavailable error", args, stderr.String())
					}
					return
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
				if envelope.Command != test.command ||
					envelope.Error.Code != "runtime_error" ||
					!strings.Contains(envelope.Error.Message, "not available in this build") {
					t.Fatalf("%v JSON error = %#v", args, envelope)
				}
			})
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
	service := provider.Service{Store: database}
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
