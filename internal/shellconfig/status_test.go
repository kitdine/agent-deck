package shellconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusReportsConfigurationAndActivationWithoutExposingModifiedBytes(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	parentPID := 4242
	markers := map[Shell]string{ShellZsh: "4242"}

	configuredPath := filepath.Join(home, ".zshrc")
	if summary, err := manager.Setup(Request{Shell: ShellZsh, RC: configuredPath}); err != nil {
		t.Fatal(err)
	} else {
		assertSingleOutcome(t, summary, OutcomeConfigured)
	}
	status, err := manager.Status(Request{Shell: ShellZsh, RC: configuredPath}, markers, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationConfigured, ActivationActive)

	status, err = manager.Status(Request{Shell: ShellZsh, RC: configuredPath}, nil, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationConfigured, ActivationInactive)

	markers[ShellZsh] = "4000"
	status, err = manager.Status(Request{Shell: ShellZsh, RC: configuredPath}, markers, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationConfigured, ActivationInherited)

	modified := bytes.Replace(
		mustReadFile(t, configuredPath),
		[]byte("shell-init zsh"),
		[]byte("shell-init secret-project"),
		1,
	)
	if err := os.WriteFile(configuredPath, modified, 0o600); err != nil {
		t.Fatal(err)
	}
	status, err = manager.Status(Request{Shell: ShellZsh, RC: configuredPath}, nil, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationModified, ActivationInactive)
	if bytes.Contains([]byte(status.Results[0].Error), []byte("secret-project")) {
		t.Fatalf("status error exposed modified startup bytes: %q", status.Results[0].Error)
	}

	invalidPath := filepath.Join(home, "invalid")
	if err := os.WriteFile(invalidPath, []byte(startMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err = manager.Status(Request{Shell: ShellZsh, RC: invalidPath}, nil, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationInvalid, ActivationInactive)

	missingPath := filepath.Join(home, "missing")
	status, err = manager.Status(Request{Shell: ShellZsh, RC: missingPath}, nil, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleStatus(t, status, ConfigurationAbsent, ActivationInactive)
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("status created missing startup file: %v", err)
	}
}

func TestStatusReportsAtMostTheInvokingShellActive(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellFish}})
	parentPID := 4242
	status, err := manager.Status(Request{}, map[Shell]string{
		ShellZsh:  "4242",
		ShellFish: "4242",
	}, parentPID)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, result := range status.Results {
		if result.Activation == ActivationActive {
			active++
			if result.Shell != ShellFish {
				t.Fatalf("active shell = %s, want fish", result.Shell)
			}
		}
	}
	if active != 1 {
		t.Fatalf("active results = %d, want 1: %#v", active, status.Results)
	}
}

func TestStatusFiltersDefaultTargetsButKeepsExplicitMissingShell(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})

	status, err := manager.Status(Request{}, nil, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Results) != 1 ||
		status.Results[0].Shell != ShellZsh ||
		status.Results[0].Configuration != ConfigurationAbsent {
		t.Fatalf("invoking-only status = %#v, want absent zsh only", status.Results)
	}

	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	status, err = manager.Status(Request{}, nil, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Results) != 2 ||
		status.Results[0].Shell != ShellZsh ||
		status.Results[1].Shell != ShellFish {
		t.Fatalf("multi-shell status = %#v, want zsh and existing fish", status.Results)
	}

	missingBash := filepath.Join(home, "missing-bash")
	status, err = manager.Status(
		Request{Shell: ShellBash, RC: missingBash},
		nil,
		4242,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Results) != 1 ||
		status.Results[0].Shell != ShellBash ||
		status.Results[0].Path != missingBash ||
		status.Results[0].Configuration != ConfigurationAbsent {
		t.Fatalf("explicit missing status = %#v, want absent bash", status.Results)
	}
}

func TestStatusReportsOnlyTheInvokingBashStartupFileActive(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{".bash_profile", ".bashrc"} {
		if err := os.WriteFile(filepath.Join(home, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, login := range []bool{true, false} {
		t.Run(map[bool]string{true: "login", false: "non-login"}[login], func(t *testing.T) {
			manager := New(Environment{
				Home:       home,
				Invocation: Invocation{Shell: ShellBash, Login: login},
			})
			status, err := manager.Status(
				Request{},
				map[Shell]string{ShellBash: "4242"},
				4242,
			)
			if err != nil {
				t.Fatal(err)
			}
			wantPath := filepath.Join(home, ".bashrc")
			if login {
				wantPath = filepath.Join(home, ".bash_profile")
			}
			active := 0
			for _, result := range status.Results {
				if result.Activation == ActivationActive {
					active++
					if result.Path != wantPath {
						t.Errorf("active path = %s, want %s", result.Path, wantPath)
					}
				}
			}
			if active != 1 {
				t.Fatalf("active results = %d, want 1: %#v", active, status.Results)
			}
		})
	}
}

func assertSingleStatus(t *testing.T, summary StatusSummary, configuration ConfigurationState, activation ActivationState) {
	t.Helper()
	if len(summary.Results) != 1 {
		t.Fatalf("results = %#v, want one", summary.Results)
	}
	result := summary.Results[0]
	if result.Configuration != configuration || result.Activation != activation {
		t.Fatalf("status = %#v, want configuration=%s activation=%s", result, configuration, activation)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
