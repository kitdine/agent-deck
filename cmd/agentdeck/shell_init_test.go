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
