package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

type notInstalledCLIFixture struct{ account string }

func (r notInstalledCLIFixture) AccountHome() (string, error) { return r.account, nil }
func (r notInstalledCLIFixture) Resolve() platform.InstallationIdentity {
	return platform.InstallationIdentity{Reason: "receipt_other_executable"}
}

func stateLocation(t *testing.T, args ...string) (map[string]any, string) {
	t.Helper()
	var stdout bytes.Buffer
	if err := run(append([]string{"--format", "json"}, append(args, "desktop", "state-location")...), bytes.NewReader(nil), &stdout); err != nil {
		t.Fatalf("state-location: %v", err)
	}
	var envelope struct {
		Command string         `json:"command"`
		Data    map[string]any `json:"data"`
		Partial bool           `json:"partial"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.Command != "desktop.state-location" || envelope.Partial {
		t.Fatalf("envelope = %s, %v", stdout.String(), err)
	}
	return envelope.Data, stdout.String()
}

func TestDesktopStateLocationInstalledAndNotInstalled(t *testing.T) {
	t.Cleanup(func() { _ = platform.ReleaseStateOwnership() })
	home, err := filepath.EvalSymlinks(os.Getenv("HOME"))
	if err != nil {
		t.Fatal(err)
	}
	account := os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME")
	explicit, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	data, _ := stateLocation(t, "--state-dir", explicit)
	if data["installed"] != true || data["channel"] != "make" || data["state_root"] != explicit || data["reason"] != nil {
		t.Fatalf("installed explicit = %v", data)
	}
	data, _ = stateLocation(t)
	if data["installed"] != true || data["state_root"] != filepath.Join(home, ".agentdeck") {
		t.Fatalf("installed default = %v", data)
	}
	if err := platform.ReleaseStateOwnership(); err != nil {
		t.Fatal(err)
	}

	platform.SetInstallationResolver(notInstalledCLIFixture{account})
	t.Cleanup(func() { platform.SetInstallationResolver(installedCLIFixture{account}) })
	isolated := filepath.Join(home, ".agentdeck-dev")
	data, _ = stateLocation(t)
	if data["installed"] != false || data["reason"] != "receipt_other_executable" || data["state_root"] != isolated || data["channel"] != nil {
		t.Fatalf("not-installed default = %v", data)
	}
	// Pointing at the real state never discloses it: the isolated root is
	// reported instead of a refusal or the real path.
	real := filepath.Join(account, ".agentdeck")
	data, raw := stateLocation(t, "--state-dir", real)
	if data["installed"] != false || data["state_root"] != isolated || strings.Contains(raw, account) {
		t.Fatalf("not-installed real = %s", raw)
	}
	if _, err := os.Stat(real); !os.IsNotExist(err) {
		t.Fatalf("real state touched: %v", err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"desktop", "state-location"}, bytes.NewReader(nil), &stdout); err == nil {
		t.Fatal("text format accepted")
	}
}
