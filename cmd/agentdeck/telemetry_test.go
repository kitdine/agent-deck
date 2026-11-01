package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

func telemetryState(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(t.TempDir(), "state"), home
}

func runTelemetry(t *testing.T, state string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := execute(append([]string{"--state-dir", state}, args...), bytes.NewReader(nil), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func decodeEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return envelope
}

func writeCodexConfigFile(t *testing.T, home, content string) string {
	t.Helper()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTelemetryStatusUsesSpecimenFieldsAndKeepsUnknowns(t *testing.T) {
	state, _ := telemetryState(t)
	stdout, stderr, code := runTelemetry(t, state, "--format", "json", "telemetry", "status")
	if code != 0 || stderr != "" {
		t.Fatalf("status exit %d stderr %q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	data := envelope["data"].(map[string]any)
	if envelope["command"] != "telemetry.status" || data["receiver"] != "disabled" || data["listener"] != "stopped" || data["engine_mode"] != "none" ||
		data["metering"] != "disabled" || data["producer_configured"] != "unknown" || data["last_received_at"] != nil ||
		data["reception"] != "unknown" || data["historical_coverage"] != "unknown" || data["codex_memories"] != "unknown" || data["codex_exporter"] != "unknown" {
		t.Fatalf("default status = %v", data)
	}
	if _, err := os.Stat(state); err == nil {
		t.Fatal("status created state")
	}
	runTelemetry(t, state, "telemetry", "enable")
	text, _, _ := runTelemetry(t, state, "telemetry", "status")
	for _, want := range []string{"Receiver: enabled", "Producer configuration: unknown", "Codex Memories: unknown", "Codex exporter: unknown",
		"Reception: waiting for first verified event", "Connection: unknown (independent HTTP requests)", "Metering: capture_only",
		"Reason: no verified producer event received yet", "Historical coverage: unknown"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status text missing %q:\n%s", want, text)
		}
	}
	token, _ := telemetry.ReadToken(state)
	if strings.Contains(text, token) {
		t.Fatal("status leaked the local token")
	}
}

func TestTelemetryStatusFollowsTheMemoriesLinkage(t *testing.T) {
	state, home := telemetryState(t)
	writeCodexConfigFile(t, home, "[features]\nmemories = true\n")
	stdout, _, code := runTelemetry(t, state, "--format", "json", "telemetry", "status")
	envelope := decodeEnvelope(t, stdout)
	warnings := envelope["warnings"].([]any)
	if code != 0 || len(warnings) != 1 || warnings[0] != telemetry.WarningMemoriesNotCounted || envelope["data"].(map[string]any)["codex_memories"] != "on" {
		t.Fatalf("memories on, collection off = %v", envelope)
	}
	text, _, _ := runTelemetry(t, state, "telemetry", "status")
	if !strings.Contains(text, "Warning: Codex Memories background usage is not counted.\nRun agentdeck telemetry enable --write-codex-config to count it.") {
		t.Fatalf("missing warning:\n%s", text)
	}
	writeCodexConfigFile(t, home, "[features]\nmemories = false\n")
	runTelemetry(t, state, "telemetry", "enable")
	text, _, _ = runTelemetry(t, state, "telemetry", "status")
	if !strings.Contains(text, "Listener: stopped (Codex Memories is off; collection is not needed)") || !strings.Contains(text, "Codex Memories: off") {
		t.Fatalf("memories off status:\n%s", text)
	}
}

func TestTelemetryEnableWritesCodexConfigOnlyWithConsent(t *testing.T) {
	state, home := telemetryState(t)
	original := "model = \"gpt-6-astra\"\n\n[features]\nmemories = true\n"
	path := writeCodexConfigFile(t, home, original)
	out, _, code := runTelemetry(t, state, "telemetry", "enable")
	if code != 0 || !strings.Contains(out, "Codex configuration was not changed") {
		t.Fatalf("plain enable = %d %q", code, out)
	}
	if now, _ := os.ReadFile(path); string(now) != original {
		t.Fatal("enable without consent changed Codex configuration")
	}
	out, _, code = runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	if code != 0 || !strings.Contains(out, "Appended AgentDeck's [otel] block to ~/.codex/config.toml.\nRestart Codex for it to take effect.") {
		t.Fatalf("consented enable = %d %q", code, out)
	}
	written, _ := os.ReadFile(path)
	if !bytes.HasPrefix(written, []byte(original)) || !bytes.Contains(written, []byte("# BEGIN AgentDeck OTel (managed)")) {
		t.Fatalf("written config:\n%s", written)
	}
	out, _, code = runTelemetry(t, state, "--format", "json", "telemetry", "disable")
	data := decodeEnvelope(t, out)["data"].(map[string]any)
	if code != 0 || data["receiver"] != "disabled" || data["codex_config"] != "removed" {
		t.Fatalf("disable = %d %v", code, data)
	}
	if now, _ := os.ReadFile(path); string(now) != original {
		t.Fatalf("disable did not restore the original bytes:\n%s", now)
	}
}

func TestTelemetryCodexConfigFailuresChangeNothing(t *testing.T) {
	state, home := telemetryState(t)
	other := "[otel]\nexporter = { otlp-http = { endpoint = \"https://collector.example/v1/logs\" } }\n"
	path := writeCodexConfigFile(t, home, other)
	stdout, stderr, code := runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "codex_otel_conflict: ~/.codex/config.toml already has an [otel] section.\nNothing was changed.") {
		t.Fatalf("conflict = %d %q %q", code, stdout, stderr)
	}
	stdout, stderr, _ = runTelemetry(t, state, "--format", "json", "telemetry", "enable", "--write-codex-config")
	envelope := decodeEnvelope(t, stderr)
	if stdout != "" || envelope["data"] != nil || envelope["error"].(map[string]any)["code"] != "codex_otel_conflict" {
		t.Fatalf("conflict JSON = %q %v", stdout, envelope)
	}
	if config, _ := telemetry.LoadConfig(state); config.Enabled {
		t.Fatal("a refused Codex write still enabled the receiver")
	}
	if now, _ := os.ReadFile(path); string(now) != other {
		t.Fatal("conflict changed Codex configuration")
	}
	// Restore incomplete: the managed block was edited after it was written.
	writeCodexConfigFile(t, home, "[features]\nmemories = true\n")
	runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	edited, _ := os.ReadFile(path)
	edited = bytes.Replace(edited, []byte("log_user_prompt = false"), []byte("log_user_prompt = true"), 1)
	_ = os.WriteFile(path, edited, 0o644)
	stdout, stderr, code = runTelemetry(t, state, "telemetry", "disable")
	if code != 0 || !strings.Contains(stdout, "Receiver disabled.") || stderr != "codex_config_restore_incomplete: the [otel] block was changed elsewhere and was left untouched.\n" {
		t.Fatalf("restore incomplete = %d %q %q", code, stdout, stderr)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, edited) {
		t.Fatal("incomplete restore changed the file")
	}
}

func TestTelemetryEnableFailureAfterCodexWriteLeavesNoUnmanagedBlock(t *testing.T) {
	state, home := telemetryState(t)
	original := "[features]\nmemories = true\n"
	path := writeCodexConfigFile(t, home, original)
	// The managed record cannot be written: nothing may change.
	recordPath := filepath.Join(telemetry.Dir(state), "codex-config.json")
	if err := os.MkdirAll(recordPath, 0o700); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "codex_config_write_failed: ~/.codex/config.toml could not be updated.\nNothing was changed.") {
		t.Fatalf("record failure = %d %q %q", code, stdout, stderr)
	}
	if now, _ := os.ReadFile(path); string(now) != original {
		t.Fatal("record failure changed Codex configuration")
	}
	if config, _ := telemetry.LoadConfig(state); config.Enabled {
		t.Fatal("record failure enabled the receiver")
	}
	if err := os.Remove(recordPath); err != nil {
		t.Fatal(err)
	}
	// The receiver preference cannot be saved: the written block is taken back.
	saveErr := errors.New("receiver preference not saved")
	saveTelemetryConfig = func(string, telemetry.Config) error { return saveErr }
	t.Cleanup(func() { saveTelemetryConfig = telemetry.SaveConfig })
	stdout, stderr, code = runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	if code != 1 || stdout != "" || strings.Contains(stderr, "codex_config_write_failed") {
		t.Fatalf("save failure = %d %q %q", code, stdout, stderr)
	}
	if now, _ := os.ReadFile(path); string(now) != original {
		t.Fatalf("save failure left the Codex block:\n%s", now)
	}
	// Retrying after the failure writes and later removes the block.
	saveTelemetryConfig = telemetry.SaveConfig
	if _, _, code := runTelemetry(t, state, "telemetry", "enable", "--write-codex-config"); code != 0 {
		t.Fatalf("retry = %d", code)
	}
	out, _, _ := runTelemetry(t, state, "--format", "json", "telemetry", "disable")
	if data := decodeEnvelope(t, out)["data"].(map[string]any); data["codex_config"] != "removed" {
		t.Fatalf("disable after retry = %v", data)
	}
	if now, _ := os.ReadFile(path); string(now) != original {
		t.Fatal("disable after retry did not restore the original bytes")
	}
}

func TestTelemetryEnableReportsABlockThatCouldNotBeTakenBack(t *testing.T) {
	state, home := telemetryState(t)
	path := writeCodexConfigFile(t, home, "[features]\nmemories = true\n")
	t.Cleanup(func() { saveTelemetryConfig = telemetry.SaveConfig })
	saveTelemetryConfig = func(string, telemetry.Config) error {
		// The block is edited before the rollback, so it must stay.
		edited, _ := os.ReadFile(path)
		_ = os.WriteFile(path, bytes.Replace(edited, []byte("log_user_prompt = false"), []byte("log_user_prompt = true"), 1), 0o644)
		return errors.New("receiver preference not saved")
	}
	stdout, stderr, code := runTelemetry(t, state, "telemetry", "enable", "--write-codex-config")
	if code != 1 || stdout != "" || strings.Contains(stderr, "Nothing was changed") ||
		!strings.HasPrefix(stderr, "codex_config_write_failed: ~/.codex/config.toml was updated, but the receiver was not enabled") {
		t.Fatalf("block remains = %d %q %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(telemetry.Dir(state), "codex-config.json")); err != nil {
		t.Fatalf("managed record was not kept: %v", err)
	}
}

func TestTelemetryConfigPreviewRedactsTokenUnlessRevealed(t *testing.T) {
	state, _ := telemetryState(t)
	runTelemetry(t, state, "telemetry", "enable")
	token, _ := telemetry.ReadToken(state)
	redacted, _, code := runTelemetry(t, state, "telemetry", "config-preview")
	if code != 0 || strings.Contains(redacted, token) || !strings.Contains(redacted, "Codex config was not changed") || !strings.Contains(redacted, "[otel]") ||
		!strings.Contains(redacted, `protocol = "json"`) || !strings.Contains(redacted, telemetry.TokenHeader) {
		t.Fatalf("redacted preview = %q", redacted)
	}
	revealed, _, _ := runTelemetry(t, state, "--format", "json", "telemetry", "config-preview", "--reveal-local-token")
	data := decodeEnvelope(t, revealed)["data"].(map[string]any)
	if data["applied"] != false || data["token_shown"] != true || !strings.Contains(data["snippet"].(string), token) {
		t.Fatalf("revealed preview = %v", data)
	}
}

const importFixture = `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex_exec"}},{"key":"service.version","value":{"stringValue":"0.165.0"}}]},"scopeLogs":[{"logRecords":[{"timeUnixNano":"1793272990000000000","eventName":"event otel/src/events/session_telemetry.rs:1132","body":{"stringValue":"PRIVATE BODY"},"attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"conversation.id","value":{"stringValue":"bg-cli-1"}},{"key":"originator","value":{"stringValue":"codex_exec"}},{"key":"model","value":{"stringValue":"gpt-5.6-terra"}},{"key":"input_token_count","value":{"intValue":"1021"}},{"key":"output_token_count","value":{"intValue":"13"}}]}]}]}]}`

func TestTelemetryImportIsPartialWhilePendingAndRepeatsAsNoOp(t *testing.T) {
	state, _ := telemetryState(t)
	file := filepath.Join(t.TempDir(), "otel.json")
	if err := os.WriteFile(file, []byte(importFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runTelemetry(t, state, "--format", "json", "telemetry", "import", file)
	envelope := decodeEnvelope(t, stdout)
	data := envelope["data"].(map[string]any)
	if code != 0 || envelope["partial"] != true || data["inserted"] != float64(1) || data["pending"] != float64(1) || len(envelope["warnings"].([]any)) != 1 {
		t.Fatalf("first import = %v", envelope)
	}
	repeat, _, code := runTelemetry(t, state, "telemetry", "import", file)
	if code != 0 || !strings.Contains(repeat, "No new observations (1 already imported)") {
		t.Fatalf("repeat import = %q", repeat)
	}
	bad := filepath.Join(t.TempDir(), "cpa.csv")
	_ = os.WriteFile(bad, []byte("time,model,input\n"), 0o600)
	if _, stderr, code := runTelemetry(t, state, "telemetry", "import", bad); code != 1 || stderr == "" {
		t.Fatalf("unsupported import exit %d stderr %q", code, stderr)
	}
	database, _ := os.ReadFile(filepath.Join(state, "agentdeck.sqlite3"))
	if bytes.Contains(database, []byte("PRIVATE BODY")) {
		t.Fatal("import stored a log body")
	}
}

func TestBackgroundSessionIsVisibleThroughSessionCommands(t *testing.T) {
	state, _ := telemetryState(t)
	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	records, err := telemetry.Decode([]byte(importFixture))
	if err != nil {
		t.Fatal(err)
	}
	received := time.Now().Add(-time.Hour)
	if _, err := telemetry.Import(ctx, core, records, received); err != nil {
		t.Fatal(err)
	}
	if _, err := core.DB.Exec(`INSERT INTO inventory_runs(inventory_id, epoch, captured_at, source_count, state) VALUES('inv', 1, ?, 0, 'complete')`, time.Now().Add(-30*time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := telemetry.SettleAndProject(ctx, core, index, time.Now()); err != nil || !changed {
		t.Fatalf("settle = %v %v", changed, err)
	}
	index.Close()
	core.Close()
	listed, _, code := runTelemetry(t, state, "--format", "json", "session", "list")
	if code != 0 || !strings.Contains(listed, `"session_id":"bg-cli-1"`) || !strings.Contains(listed, `"source_path":"otel:codex/bg-cli-1"`) {
		t.Fatalf("session list = %d %s", code, listed)
	}
	shown, stderr, code := runTelemetry(t, state, "session", "show", "bg-cli-1")
	if code != 0 || strings.Contains(shown+stderr, "session scan") || strings.Contains(shown, "otel:codex") {
		t.Fatalf("session show = %d %q %q", code, shown, stderr)
	}
	shownJSON, _, code := runTelemetry(t, state, "--format", "json", "session", "show", "bg-cli-1", "--activity")
	if code != 0 || !strings.Contains(shownJSON, `"source_path":"otel:codex/bg-cli-1"`) {
		t.Fatalf("session show JSON = %d %s", code, shownJSON)
	}
	// The session index can be rebuilt: the projection is regenerated.
	if _, _, code := runTelemetry(t, state, "session", "rebuild"); code != 0 {
		t.Fatal("session rebuild failed")
	}
	if relisted, _, _ := runTelemetry(t, state, "--format", "json", "session", "list"); !strings.Contains(relisted, `"session_id":"bg-cli-1"`) {
		t.Fatalf("rebuild lost the projection: %s", relisted)
	}
}

func TestTelemetryImportRetractionReachesTheSessionIndex(t *testing.T) {
	state, _ := telemetryState(t)
	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	records, err := telemetry.Decode([]byte(importFixture))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := telemetry.Import(ctx, core, records, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := core.DB.Exec(`INSERT INTO inventory_runs(inventory_id, epoch, captured_at, source_count, state) VALUES('inv', 1, ?, 0, 'complete')`, time.Now().Add(-30*time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := telemetry.SettleAndProject(ctx, core, index, time.Now()); err != nil || !changed {
		t.Fatalf("settle = %v %v", changed, err)
	}
	index.Close()
	core.Close()
	if listed, _, _ := runTelemetry(t, state, "--format", "json", "session", "list"); !strings.Contains(listed, `"session_id":"bg-cli-1"`) {
		t.Fatalf("background session not projected: %s", listed)
	}
	// The same record with other counts conflicts: only the import retracts
	// the contribution, and settlement afterwards finds nothing left to change.
	file := filepath.Join(t.TempDir(), "conflict.json")
	if err := os.WriteFile(file, []byte(strings.Replace(importFixture, `"intValue":"13"`, `"intValue":"14"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if stdout, _, code := runTelemetry(t, state, "--format", "json", "telemetry", "import", file); code != 0 || decodeEnvelope(t, stdout)["data"].(map[string]any)["conflict"] == float64(0) {
		t.Fatalf("conflicting import = %d %s", code, stdout)
	}
	if listed, _, code := runTelemetry(t, state, "--format", "json", "session", "list"); code != 0 || strings.Contains(listed, `"session_id":"bg-cli-1"`) {
		t.Fatalf("retracted session still listed: %d %s", code, listed)
	}
}

func TestTelemetryListenerWaitsOutAHeldStateLock(t *testing.T) {
	state, _ := telemetryState(t)
	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	core.Close()
	lock, err := store.AcquireLock(ctx, state, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	released := time.Now().Add(6 * time.Second)
	go func() {
		time.Sleep(time.Until(released))
		_ = lock.Release()
	}()
	database, err := openTelemetryStore(ctx, state)
	if err != nil {
		t.Fatalf("listener gave up while the state lock was held: %v", err)
	}
	database.Close()
	if time.Now().Before(released) {
		t.Fatal("store opened while the lock was still held")
	}
}
