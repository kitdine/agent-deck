package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/store"
)

// This exercises the user-visible handoff between read-only diagnosis, desktop
// wire publication, and the separately requested inventory mutation.
func TestHealthRecoveryAcceptanceAcrossCLIAndDesktop(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	t.Setenv("HOME", home)
	clientConfig := filepath.Join(home, ".claude.json")
	configBytes := []byte(`{"mcpServers":{"live-mcp":{"command":"echo"}}}`)
	if err := os.WriteFile(clientConfig, configBytes, 0600); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	staleID := "codex:mcp:user:stale-mcp"
	if err := db.ReplaceExtensions(ctx, []store.Extension{{
		ID: staleID, Client: "codex", Kind: "mcp", Scope: "user",
		NativeID: "stale-mcp", Fingerprint: "old-fingerprint",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	stateLock, err := store.AcquireLock(ctx, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stateLock.Release() })
	scanLock, err := store.AcquireScanLock(ctx, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scanLock.Release() })
	stateToken, err := os.ReadFile(filepath.Join(state, "state.lock"))
	if err != nil {
		t.Fatal(err)
	}
	scanToken, err := os.ReadFile(filepath.Join(state, "scan.lock"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "doctor"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("doctor text exit=%d stderr=%s", exit, stderr.String())
	}
	for _, want := range []string{
		"state_lock: warning (lock_live)",
		"scan_lock: warning (lock_live)",
		"extensions: warning (extension_stale_inventory; count=1)",
		"recovery: agentdeck extension scan",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("doctor text lacks %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "Remove state.lock") || strings.Contains(stdout.String(), "Remove scan.lock") {
		t.Fatalf("live locks received removal guidance:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"--state-dir", state, "--format=json", "doctor"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("doctor JSON exit=%d stderr=%s", exit, stderr.String())
	}
	var doctorEnvelope struct {
		Data doctor.Report `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doctorEnvelope); err != nil {
		t.Fatalf("doctor JSON: %v: %s", err, stdout.String())
	}
	checks := make(map[string]doctor.Check)
	for _, check := range doctorEnvelope.Data.Checks {
		checks[check.Name] = check
	}
	for name, resource := range map[string]string{"state_lock": "state", "scan_lock": "scan"} {
		check := checks[name]
		if check.Reason != "lock_live" || check.Resource != resource || check.ActionKind != "retry" || check.Recovery != "" {
			t.Fatalf("%s diagnosis = %#v", name, check)
		}
	}
	extensionCheck := checks["extensions"]
	if extensionCheck.Reason != "extension_stale_inventory" || extensionCheck.ActionKind != "synchronize_inventory" ||
		extensionCheck.Recovery != "agentdeck extension scan" || extensionCheck.DiagnosticCommand != "" {
		t.Fatalf("extension diagnosis = %#v", extensionCheck)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{
		"--state-dir", state, "--format=json", "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "3",
	}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("desktop snapshot exit=%d stderr=%s", exit, stderr.String())
	}
	var desktopEnvelope struct {
		Data desktop.Snapshot `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &desktopEnvelope); err != nil {
		t.Fatalf("desktop JSON: %v: %s", err, stdout.String())
	}
	wireChecks := make(map[string]desktop.HealthCheck)
	for _, check := range desktopEnvelope.Data.Health.Checks {
		wireChecks[check.Name] = check
	}
	for name, resource := range map[string]string{"state_lock": "state", "scan_lock": "scan"} {
		check := wireChecks[name]
		if check.Reason == nil || *check.Reason != "lock_live" || check.Resource == nil || *check.Resource != resource ||
			check.ActionKind == nil || *check.ActionKind != "retry" || check.Recovery != "" {
			t.Fatalf("%s wire diagnosis = %#v", name, check)
		}
	}
	wireExtension := wireChecks["extensions"]
	if wireExtension.Reason == nil || *wireExtension.Reason != extensionCheck.Reason ||
		wireExtension.ActionKind == nil || *wireExtension.ActionKind != string(extensionCheck.ActionKind) ||
		wireExtension.Recovery != extensionCheck.Recovery {
		t.Fatalf("extension wire diagnosis = %#v; doctor = %#v", wireExtension, extensionCheck)
	}
	for path, before := range map[string][]byte{
		filepath.Join(state, "state.lock"): stateToken,
		filepath.Join(state, "scan.lock"):  scanToken,
		clientConfig:                       configBytes,
	} {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("read-only diagnosis changed %s: %v", filepath.Base(path), err)
		}
	}

	if err := scanLock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := stateLock.Release(); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"--state-dir", state, "extension", "scan"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("extension scan exit=%d stderr=%s", exit, stderr.String())
	}
	configAfter, err := os.ReadFile(clientConfig)
	if err != nil || !bytes.Equal(configAfter, configBytes) {
		t.Fatalf("extension scan changed client config: %v", err)
	}
	opened, err := store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	items, err := opened.ListExtensions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var foundLive bool
	for _, item := range items {
		if item.ID == staleID {
			t.Fatalf("stale identity survived explicit sync: %#v", items)
		}
		if item.ID == "claude:mcp:user:live-mcp" {
			foundLive = true
		}
	}
	if !foundLive {
		t.Fatalf("live identity was not recorded: %#v", items)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"--state-dir", state, "--format=json", "extension", "doctor"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("post-sync extension doctor exit=%d stderr=%s", exit, stderr.String())
	}
	var afterDoctor struct {
		Data struct {
			Reason         string   `json:"reason"`
			StaleInventory []string `json:"stale_inventory"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &afterDoctor); err != nil {
		t.Fatalf("post-sync doctor JSON: %v: %s", err, stdout.String())
	}
	if afterDoctor.Data.Reason != "" || len(afterDoctor.Data.StaleInventory) != 0 {
		t.Fatalf("stale warning remained after sync: %#v", afterDoctor.Data)
	}
}
