package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

func seedQuotaState(t *testing.T, state string, mutate func(*quota.Settings), officialClients ...string) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer database.Close()
	settings := quota.DefaultSettings()
	mutate(&settings)
	if err := quota.SaveSettings(ctx, database, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	for _, client := range officialClients {
		if err := database.RecordSelection(ctx, store.Selection{
			Client: client, ProviderName: "official", MultiplierSnapshot: "1",
			SelectedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RecordSelection: %v", err)
		}
	}
}

func runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := run(args, bytes.NewReader(nil), &out); err != nil {
		t.Fatalf("run %v: %v (output %s)", args, err, out.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s: %v", out.String(), err)
	}
	data, _ := envelope["data"].(map[string]any)
	return data
}

func quotaClientFromJSON(t *testing.T, data map[string]any, name string) map[string]any {
	t.Helper()
	clients, _ := data["clients"].([]any)
	for _, raw := range clients {
		client, _ := raw.(map[string]any)
		if client["client"] == name {
			return client
		}
	}
	t.Fatalf("no %s client in %v", name, data)
	return nil
}

func TestQuotaCommandOnAFreshInstallReportsReadingOff(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())

	data := runJSON(t, "--state-dir", state, "--format", "json", "quota")
	for _, name := range []string{"codex", "claude"} {
		client := quotaClientFromJSON(t, data, name)
		if client["applicable"] != true || client["failure"] != "probe_disabled" {
			t.Fatalf("%s = %v, want reading off reported as probe_disabled", name, client)
		}
	}
	if _, err := os.Stat(filepath.Join(state, "agentdeck.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("agentdeck quota created state (stat err %v); it must write nothing", err)
	}
}

func TestQuotaCommandReportsTheGateAsAPayloadStateAtExitZero(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true }, "claude")

	data := runJSON(t, "--state-dir", state, "--format", "json", "quota")
	codex := quotaClientFromJSON(t, data, "codex")
	if codex["applicable"] != false || codex["applicable_reason"] != "not_official" {
		t.Fatalf("codex = %v, want applicable false with not_official", codex)
	}
	claude := quotaClientFromJSON(t, data, "claude")
	if claude["failure"] != "never_probed" || claude["attribution_confirmed"] != false {
		t.Fatalf("claude = %v, want never_probed without confirmed attribution", claude)
	}

	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "quota"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatalf("text quota: %v", err)
	}
	for _, want := range []string{"Codex: not applicable, provider is not official", "Claude: no figures, not yet observed"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("text output %q does not contain %q", text.String(), want)
		}
	}
}

func TestQuotaCommandRendersFiguresAsText(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true }, "claude")
	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	now := time.Now().UTC()
	if _, _, _, err := quota.NewStore(database.DB).Record(context.Background(), quota.Observation{
		Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeStatusLine,
		ObservedAt: now, WindowMinutes: 300, UsedPercent: 64, ResetsAt: now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	database.Close()

	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "quota"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatalf("text quota: %v", err)
	}
	for _, want := range []string{"Claude, plan not reported, via claude_statusline", "account attribution cannot be confirmed", "5-hour window", "64%", "tightest", "(observed "} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("text output %q does not contain %q", text.String(), want)
		}
	}
}

func TestQuotaCommandRendersUnavailableResetAllowanceTotal(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true }, "codex")
	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	now := time.Now().UTC()
	quotaStore := quota.NewStore(database.DB)
	if _, _, _, err := quotaStore.Record(context.Background(), quota.Observation{
		Client: quota.ClientCodex, AccountID: "acct", WindowKey: "codex", Source: quota.SourceCodex,
		ObservedAt: now, WindowMinutes: 300, UsedPercent: 64, ResetsAt: now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := quotaStore.PutEnvelope(context.Background(), quota.EnvelopeRecord{
		Client: quota.ClientCodex, AccountID: "acct", Applicable: true, Source: quota.SourceCodex, ObservedAt: now,
		ResetAllowance: quota.ResetAllowance{Remaining: 3, HasRemaining: true, TotalReason: quota.ReasonNotReported},
	}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}
	database.Close()

	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "quota"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatalf("text quota: %v", err)
	}
	if want := "reset allowance: 3 remaining, total not reported"; !strings.Contains(text.String(), want) {
		t.Fatalf("text output %q does not contain %q", text.String(), want)
	}
}

// TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible is WC-R1-F1's
// regression: a manual failure leaves the envelope's FailureAt untouched
// (GS-R3-F1), so a probe that fails manually right after a success must not
// be judged by comparing that stale FailureAt against the last observation.
// It also covers WC-R1-F2 (the CLI text must not read "last probe probe
// failed") and WC-R2-F1 (a manual parse failure must still carry the
// observation instant of the failed attempt, requirements.md clause 6,
// even though FailureAt itself stays untouched).
func TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true }, "codex", "claude")

	codexFail, claudeFail := false, false
	previousService := quotaRefreshService
	quotaRefreshService = func(stateRoot, home string) desktop.Service {
		return desktop.Service{
			StateRoot: stateRoot, Home: home,
			QuotaProbeCodex: func(_ context.Context, observedAt time.Time, _ time.Duration) (quota.CodexResult, error) {
				if codexFail {
					return quota.CodexResult{}, errors.New("boom")
				}
				return quota.CodexResult{AccountID: "acct", Plan: "pro", Windows: []quota.Observation{{
					Client: quota.ClientCodex, AccountID: "acct", WindowKey: "codex", Source: quota.SourceCodex,
					ObservedAt: observedAt, WindowMinutes: 300, UsedPercent: 40, ResetsAt: observedAt.Add(4 * time.Hour),
				}}}, nil
			},
			QuotaProbeClaudeProse: func(_ context.Context, observedAt time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
				if claudeFail {
					return quota.ClaudeProseResult{}, &quota.ClaudeFailureError{Kind: quota.ClaudeFailureParse, Err: errors.New("bad output")}
				}
				return quota.ClaudeProseResult{Windows: []quota.Observation{{
					Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeProse,
					ObservedAt: observedAt, WindowMinutes: 300, UsedPercent: 30, ResetsAt: observedAt.Add(3 * time.Hour),
				}}}, nil
			},
		}
	}
	t.Cleanup(func() { quotaRefreshService = previousService })

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")

	codexFail, claudeFail = true, true
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")

	data := runJSON(t, "--state-dir", state, "--format", "json", "quota")
	codex := quotaClientFromJSON(t, data, "codex")
	windows, _ := codex["windows"].([]any)
	if codex["failure"] != "probe_failed" || len(windows) != 1 {
		t.Fatalf("codex = %v, want the manual probe failure visible with the last figure kept (WC-R1-F1)", codex)
	}
	claude := quotaClientFromJSON(t, data, "claude")
	claudeWindows, _ := claude["windows"].([]any)
	if claude["failure"] != "parse_failed" || len(claudeWindows) != 0 {
		t.Fatalf("claude = %v, want the manual parse failure visible with no figure re-presented (requirements.md clause 6)", claude)
	}
	if claude["observed_at"] == nil || claude["observed_at"] == "" {
		t.Fatalf("claude = %v, want observed_at set to the manual attempt's instant (requirements.md clause 6, WC-R2-F1)", claude)
	}

	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "quota"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatalf("text quota: %v", err)
	}
	if !strings.Contains(text.String(), "last probe failed") {
		t.Fatalf("text output %q does not contain %q", text.String(), "last probe failed")
	}
	if strings.Contains(text.String(), "probe probe") {
		t.Fatalf("text output %q duplicates the word probe (WC-R1-F2)", text.String())
	}
	if !strings.Contains(text.String(), "attempted") {
		t.Fatalf("text output %q does not report the failed attempt's instant (WC-R2-F1)", text.String())
	}
}

func TestQuotaCommandRejectsAnUnsupportedFormat(t *testing.T) {
	withTestHome(t, t.TempDir())
	if err := run([]string{"--state-dir", filepath.Join(t.TempDir(), "state"), "--format", "ndjson", "quota"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("agentdeck quota accepted --format ndjson")
	}
}

type fakeQuotaRefresh struct{ codexCalls, claudeCalls int }

func withFakeQuotaRefresh(t *testing.T, used float64) *fakeQuotaRefresh {
	t.Helper()
	fake := &fakeQuotaRefresh{}
	previousService := quotaRefreshService
	quotaRefreshService = func(stateRoot, home string) desktop.Service {
		return desktop.Service{
			StateRoot: stateRoot, Home: home,
			QuotaProbeCodex: func(_ context.Context, observedAt time.Time, _ time.Duration) (quota.CodexResult, error) {
				fake.codexCalls++
				return quota.CodexResult{AccountID: "acct", Plan: "pro", Windows: []quota.Observation{{
					Client: quota.ClientCodex, AccountID: "acct", WindowKey: "codex", Source: quota.SourceCodex,
					ObservedAt: observedAt, WindowMinutes: 300, UsedPercent: used, ResetsAt: observedAt.Add(3 * time.Hour),
				}}}, nil
			},
			QuotaProbeClaudeProse: func(_ context.Context, _ time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
				fake.claudeCalls++
				return quota.ClaudeProseResult{}, nil
			},
		}
	}
	t.Cleanup(func() { quotaRefreshService = previousService })
	return fake
}

func quotaRefreshAlerts(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()
	raw, ok := data["alerts"].([]any)
	if !ok {
		t.Fatalf("alerts = %#v, want an array", data["alerts"])
	}
	alerts := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		alerts = append(alerts, item.(map[string]any))
	}
	return alerts
}

func TestDesktopQuotaRefreshReturnsDueAlertsForTheAppToDeliver(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) {
		s.ProbeEnabled, s.AlertsEnabled, s.AlertThresholds = true, true, []float64{75}
	}, "codex", "claude")
	fake := withFakeQuotaRefresh(t, 80)

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if fake.codexCalls != 1 || fake.claudeCalls != 1 {
		t.Fatalf("probes = codex %d, claude %d, want one each", fake.codexCalls, fake.claudeCalls)
	}
	alerts := quotaRefreshAlerts(t, data)
	if len(alerts) != 1 || alerts[0]["client"] != "codex" || alerts[0]["kind"] != "threshold" || alerts[0]["threshold"] != 75.0 ||
		alerts[0]["used_percent"] != 80.0 || alerts[0]["window_minutes"] != 300.0 || alerts[0]["id"] == "" {
		t.Fatalf("alerts = %+v, want one Codex threshold notice at 75 naming the window length and figure", alerts)
	}
	if encoded, _ := json.Marshal(alerts); strings.Contains(string(encoded), "acct") {
		t.Fatalf("alerts %s carry an account identifier", encoded)
	}
	gates, _ := data["gate_reasons"].(map[string]any)
	if gates["codex"] != nil || gates["claude"] != nil {
		t.Fatalf("gate_reasons = %v, want both allowed", gates)
	}

	// Not acknowledged: the app could not post it, so it is offered again.
	again := quotaRefreshAlerts(t, runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual"))
	if len(again) != 1 || again[0]["id"] != alerts[0]["id"] {
		t.Fatalf("unacknowledged alert on the next refresh = %+v, want the same id again", again)
	}

	ack := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-alerts", "ack", "--id", alerts[0]["id"].(string))
	if ack["acknowledged"] != 1.0 {
		t.Fatalf("ack = %v, want acknowledged 1", ack)
	}
	if after := quotaRefreshAlerts(t, runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")); len(after) != 0 {
		t.Fatalf("alerts after acknowledgement = %+v, want none", after)
	}
}

func TestDesktopQuotaRefreshWithReadingOffProbesAndEvaluatesNothing(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.AlertsEnabled = true }, "codex", "claude")
	fake := withFakeQuotaRefresh(t, 95)

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if alerts := quotaRefreshAlerts(t, data); fake.codexCalls != 0 || fake.claudeCalls != 0 || len(alerts) != 0 {
		t.Fatalf("with reading off: probes codex %d claude %d, alerts %d, want none", fake.codexCalls, fake.claudeCalls, len(alerts))
	}
	gates, _ := data["gate_reasons"].(map[string]any)
	if gates["codex"] != "probe_disabled" || gates["claude"] != "probe_disabled" {
		t.Fatalf("gate_reasons = %v, want probe_disabled for both", gates)
	}
}

func TestDesktopQuotaAlertsAckRejectsAnInvalidIDAndRecordsNothing(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) {
		s.ProbeEnabled, s.AlertsEnabled, s.AlertThresholds = true, true, []float64{75}
	}, "codex", "claude")
	withFakeQuotaRefresh(t, 80)
	alerts := quotaRefreshAlerts(t, runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual"))
	if len(alerts) != 1 {
		t.Fatalf("alerts = %+v, want one", alerts)
	}

	var out bytes.Buffer
	err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-alerts", "ack", "--id", alerts[0]["id"].(string), "--id", "qa1.forged"}, bytes.NewReader(nil), &out)
	var input *inputError
	if !errors.As(err, &input) {
		t.Fatalf("ack with a forged id = %v, want an input error", err)
	}
	if still := quotaRefreshAlerts(t, runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")); len(still) != 1 {
		t.Fatalf("after a rejected batch = %+v, want the valid alert still due (nothing recorded)", still)
	}
	if err := run([]string{"--state-dir", state, "desktop", "quota-alerts", "ack", "--id", alerts[0]["id"].(string)}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("ack accepted a non-json format")
	}
}

func claudeSettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func writeClaudeSettings(t *testing.T, home, contents string) {
	t.Helper()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

func claudeStatusLineCommand(t *testing.T, home string) string {
	t.Helper()
	contents, err := os.ReadFile(claudeSettingsPath(home))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var document struct {
		StatusLine struct {
			Command string `json:"command"`
		} `json:"statusLine"`
	}
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatalf("decode settings.json %s: %v", contents, err)
	}
	return document.StatusLine.Command
}

func TestDesktopQuotaSettingsTurningReadingOffRestoresTheStatusLineAndKeepsObservations(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{"statusLine":{"type":"command","command":"printf prior"}}`)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	enabled := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")
	if enabled["consent"] != true || !strings.HasSuffix(claudeStatusLineCommand(t, home), " quota capture") {
		t.Fatalf("enable = %v, statusLine = %q, want AgentDeck's capture command registered", enabled, claudeStatusLineCommand(t, home))
	}

	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	now := time.Now().UTC()
	if _, _, _, err := quota.NewStore(database.DB).Record(context.Background(), quota.Observation{
		Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeStatusLine,
		ObservedAt: now, WindowMinutes: 300, UsedPercent: 10, ResetsAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	database.Close()

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "off")
	settings, _ := data["settings"].(map[string]any)
	if settings["reading"] != false || settings["statusline"] != false {
		t.Fatalf("settings = %v, want reading and consent both off", settings)
	}
	restore, _ := data["statusline_restore"].(map[string]any)
	if restore == nil || restore["outcome"] == "restore_incomplete" || restore["outcome"] == "failed" {
		t.Fatalf("statusline_restore = %v, want a completed restore", restore)
	}
	if got := claudeStatusLineCommand(t, home); got != "printf prior" {
		t.Fatalf("statusLine after reading off = %q, want the prior command restored", got)
	}

	database, err = store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer database.Close()
	windows, err := quota.NewStore(database.DB).Windows(context.Background(), quota.ClientClaude)
	if err != nil || len(windows) != 1 {
		t.Fatalf("windows = (%+v, %v), want the stored observation untouched", windows, err)
	}
}

func TestDesktopQuotaSettingsReadingOffReportsRestoreIncompleteWhenTheFileChanged(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{}`)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")
	command := claudeStatusLineCommand(t, home)
	altered, err := json.Marshal(map[string]any{"statusLine": map[string]any{"type": "command", "command": command, "padding": 1}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	writeClaudeSettings(t, home, string(altered))

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "off")
	restore, _ := data["statusline_restore"].(map[string]any)
	if restore == nil || restore["outcome"] != "restore_incomplete" {
		t.Fatalf("statusline_restore = %v, want restore_incomplete", restore)
	}
	settings, _ := data["settings"].(map[string]any)
	if settings["statusline"] != false {
		t.Fatalf("settings = %v, want consent cleared even when restore is incomplete", settings)
	}
}

// Codex PR #5 sixth review, P2: managedStatusLineCommand recognizes any
// AgentDeck installation's route regardless of --state-dir (by design, for
// RestoreStatusLine's own dead-lock safety), so a state directory with no
// consent of its own must not attempt -- and thereby delete -- a route that
// belongs to a *different*, actively consented state directory.
func TestDesktopQuotaSettingsReadingOffPreservesAnotherStateDirsStatusLineRoute(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)

	stateB := filepath.Join(t.TempDir(), "state-b")
	runJSON(t, "--state-dir", stateB, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", stateB, "--format", "json", "desktop", "quota-statusline", "enable")
	commandB := claudeStatusLineCommand(t, home)
	if commandB == "" {
		t.Fatal("state B did not register its own statusLine command")
	}

	stateA := filepath.Join(t.TempDir(), "state-a")
	runJSON(t, "--state-dir", stateA, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	data := runJSON(t, "--state-dir", stateA, "--format", "json", "desktop", "quota-settings", "--reading", "off")
	if restore := data["statusline_restore"]; restore != nil {
		t.Fatalf("statusline_restore = %v, want no restore attempt: state A has no consent of its own", restore)
	}
	if got := claudeStatusLineCommand(t, home); got != commandB {
		t.Fatalf("statusLine command = %q, want state B's route %q left untouched", got, commandB)
	}
}

func TestDesktopQuotaSettingsLeavesAUsersOwnStatusLineAlone(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	original := `{"statusLine":{"type":"command","command":"python3 ~/.claude/statusline.py"}}`
	writeClaudeSettings(t, home, original)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "off")
	if data["statusline_restore"] != nil {
		t.Fatalf("statusline_restore = %v, want nothing restored when AgentDeck never installed the route", data["statusline_restore"])
	}
	contents, err := os.ReadFile(claudeSettingsPath(home))
	if err != nil || string(contents) != original {
		t.Fatalf("settings.json = (%q, %v), want it untouched", contents, err)
	}
}

func TestDesktopQuotaStatusLineEnableRequiresReadingOn(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	seedQuotaState(t, state, func(*quota.Settings) {})

	if err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("status-line consent was reachable while reading is off")
	}
	if _, err := os.Stat(claudeSettingsPath(home)); !os.IsNotExist(err) {
		t.Fatalf("settings.json was written (stat err %v)", err)
	}
}

// Codex PR #5 P1: RestoreStatusLine reports a failed write as Outcome=Failed
// with no Go error; runDesktopQuotaStatusLine must not record consent as
// withdrawn when the file's actual on-disk state is unknown -- AgentDeck's
// registration may still be active there.
func TestDesktopQuotaStatusLineDisableKeepsConsentWhenTheRestoreWriteFails(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{"statusLine":{"type":"command","command":"printf prior"}}`)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")

	settingsPath := claudeSettingsPath(home)
	if err := exec.Command("chflags", "uchg", settingsPath).Run(); err != nil {
		t.Skipf("chflags unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { _ = exec.Command("chflags", "nouchg", settingsPath).Run() })

	if err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-statusline", "disable"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("disable succeeded against an immutable settings.json")
	}

	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer database.Close()
	settings, err := quota.LoadSettings(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !settings.StatusLineConsent {
		t.Fatalf("StatusLineConsent = false after a failed restore, want it preserved as true")
	}
}

func TestDesktopQuotaSettingsReadingOffKeepsConsentWhenRestoreFails(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{"statusLine":{"type":"command","command":"printf prior"}}`)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")
	settingsPath := claudeSettingsPath(home)
	if err := exec.Command("chflags", "uchg", settingsPath).Run(); err != nil {
		t.Skipf("chflags unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { _ = exec.Command("chflags", "nouchg", settingsPath).Run() })

	if err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "off"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("reading-off succeeded against an immutable settings.json")
	}
	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer database.Close()
	settings, err := quota.LoadSettings(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !settings.StatusLineConsent {
		t.Fatal("failed restore cleared durable consent while AgentDeck's route may still be installed")
	}
}

// Codex PR #5 eighth review, P2: a failed restore used to leave next
// carrying StatusLineConsent=true with ProbeEnabled=false, a combination
// Settings.Validate() rejects with its own generic message before the
// caller ever sees the more specific, structured restore-failure result.
// The app needs that structured result -- a bare validation error is not
// decodable into the statusline-restore outcome it can present.
func TestDesktopQuotaSettingsReadingOffReportsTheStructuredRestoreFailure(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{"statusLine":{"type":"command","command":"printf prior"}}`)

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")
	settingsPath := claudeSettingsPath(home)
	if err := exec.Command("chflags", "uchg", settingsPath).Run(); err != nil {
		t.Skipf("chflags unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { _ = exec.Command("chflags", "nouchg", settingsPath).Run() })

	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "off"}, bytes.NewReader(nil), &stdout); err == nil {
		t.Fatal("reading-off succeeded against an immutable settings.json")
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not decodable JSON (want the structured restore-failure result, not a generic invalid-settings error): %v\n%s", err, stdout.String())
	}
	data, _ := envelope["data"].(map[string]any)
	restore, _ := data["statusline_restore"].(map[string]any)
	if restore == nil || restore["outcome"] != "failed" {
		t.Fatalf("data.statusline_restore = %v, want the failed restore result surfaced instead of a generic invalid-settings error", data)
	}
}

func TestDesktopQuotaStatusLineEnableRollsBackRouteWhenConsentSaveFails(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	writeClaudeSettings(t, home, `{"statusLine":{"type":"command","command":"printf prior"}}`)
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")

	previousSave := saveQuotaStatusLineSettings
	saveQuotaStatusLineSettings = func(context.Context, quota.SettingStore, quota.Settings) error {
		return errors.New("injected settings write failure")
	}
	t.Cleanup(func() { saveQuotaStatusLineSettings = previousSave })

	if err := run([]string{"--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("enable unexpectedly succeeded when consent persistence failed")
	}
	if got := claudeStatusLineCommand(t, home); got != "printf prior" {
		t.Fatalf("statusLine = %q, want the newly installed route rolled back to the prior command", got)
	}
}

func TestDesktopQuotaStatusLineUsesEmbeddedHelperAbsolutePath(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")

	previousExecutable := quotaExecutable
	quotaExecutable = func() (string, error) {
		return "/Applications/AgentDeck.app/Contents/Helpers/agentdeck", nil
	}
	t.Cleanup(func() { quotaExecutable = previousExecutable })

	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-statusline", "enable")
	want := "'/Applications/AgentDeck.app/Contents/Helpers/agentdeck' --state-dir " + shellQuote(state) + " quota capture"
	if got := claudeStatusLineCommand(t, home); got != want {
		t.Fatalf("statusLine = %q, want direct-download helper command %q", got, want)
	}
}

// Codex PR #5 fifth review, P2: the persisted command is later invoked by
// Claude from its own session/project working directory, not this
// process's, so a relative --state-dir must be resolved absolute before it
// is written into settings.json.
func TestDesktopQuotaStatusLineResolvesRelativeStateDirToAbsolute(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	workdir := t.TempDir()
	t.Chdir(workdir)

	runJSON(t, "--state-dir", "relative-state", "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", "relative-state", "--format", "json", "desktop", "quota-statusline", "enable")

	want := "agentdeck --state-dir " + shellQuote(filepath.Join(workdir, "relative-state")) + " quota capture"
	if got := claudeStatusLineCommand(t, home); got != want {
		t.Fatalf("statusLine = %q, want the relative --state-dir resolved absolute: %q", got, want)
	}
}

func TestDesktopQuotaRefreshUsesCrossProcessLock(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	state := filepath.Join(t.TempDir(), "state")
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true })
	lock, err := store.AcquireQuotaRefreshLock(context.Background(), state, 0)
	if err != nil {
		t.Fatalf("AcquireQuotaRefreshLock: %v", err)
	}
	defer lock.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	opts := &commandOptions{stateDir: state, format: "json", stdin: bytes.NewReader(nil), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if err := runDesktopQuotaRefresh(ctx, opts, false); err == nil {
		t.Fatal("a second refresh crossed the held quota-refresh lock")
	}
}
