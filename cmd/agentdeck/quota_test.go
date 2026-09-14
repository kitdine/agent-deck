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
	for _, want := range []string{"Claude, via claude_statusline", "account attribution cannot be confirmed", "5-hour window", "64%", "tightest"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("text output %q does not contain %q", text.String(), want)
		}
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

type recordingQuotaNotifier struct{ sent []quota.Notification }

func (r *recordingQuotaNotifier) Notify(_ context.Context, n quota.Notification) error {
	r.sent = append(r.sent, n)
	return nil
}

type fakeQuotaRefresh struct{ codexCalls, claudeCalls int }

func withFakeQuotaRefresh(t *testing.T, used float64) (*fakeQuotaRefresh, *recordingQuotaNotifier) {
	t.Helper()
	fake := &fakeQuotaRefresh{}
	notifier := &recordingQuotaNotifier{}
	previousService, previousNotifier := quotaRefreshService, quotaAlertNotifier
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
	quotaAlertNotifier = func() quota.Notifier { return notifier }
	t.Cleanup(func() { quotaRefreshService, quotaAlertNotifier = previousService, previousNotifier })
	return fake, notifier
}

func TestDesktopQuotaRefreshProbesAndEvaluatesAlertsUnderStoredSettings(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) {
		s.ProbeEnabled, s.AlertsEnabled, s.AlertThresholds = true, true, []float64{75}
	}, "codex", "claude")
	fake, notifier := withFakeQuotaRefresh(t, 80)

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if fake.codexCalls != 1 || fake.claudeCalls != 1 {
		t.Fatalf("probes = codex %d, claude %d, want one each", fake.codexCalls, fake.claudeCalls)
	}
	if len(notifier.sent) != 1 || notifier.sent[0].Client != quota.ClientCodex || notifier.sent[0].Threshold != 75 {
		t.Fatalf("notifications = %+v, want one Codex notice at 75", notifier.sent)
	}
	gates, _ := data["gate_reasons"].(map[string]any)
	if gates["codex"] != nil || gates["claude"] != nil {
		t.Fatalf("gate_reasons = %v, want both allowed", gates)
	}
}

func TestDesktopQuotaRefreshWithReadingOffProbesAndEvaluatesNothing(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	seedQuotaState(t, state, func(s *quota.Settings) { s.AlertsEnabled = true }, "codex", "claude")
	fake, notifier := withFakeQuotaRefresh(t, 95)

	data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if fake.codexCalls != 0 || fake.claudeCalls != 0 || len(notifier.sent) != 0 {
		t.Fatalf("with reading off: probes codex %d claude %d, notifications %d, want none", fake.codexCalls, fake.claudeCalls, len(notifier.sent))
	}
	gates, _ := data["gate_reasons"].(map[string]any)
	if gates["codex"] != "probe_disabled" || gates["claude"] != "probe_disabled" {
		t.Fatalf("gate_reasons = %v, want probe_disabled for both", gates)
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
