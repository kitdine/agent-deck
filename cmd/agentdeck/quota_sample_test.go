package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestQuotaSampleCommandHandlers(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		action quota.SampleAction
		status quota.MeasurementStatus
		text   string
	}{
		{quota.SampleStart, quota.MeasurementActive, "Measurement started"},
		{quota.SampleFinish, quota.MeasurementComplete, "Measurement completed"},
		{quota.SampleCancel, quota.MeasurementMissing, "Measurement cleared"},
	} {
		for _, format := range []string{"text", "json"} {
			t.Run(string(tc.action)+"/"+format, func(t *testing.T) {
				var stdout bytes.Buffer
				opts := &commandOptions{format: format, stdout: &stdout}
				result := quota.SampleResult{Action: tc.action, Status: tc.status}
				if tc.action != quota.SampleCancel {
					result.StartedAt = &now
				}
				if tc.action == quota.SampleFinish {
					result.FinishedAt = &now
				}
				command := quotaSampleFixtureCommand(opts, func(_ context.Context, action quota.SampleAction, client quota.Client, id string) (quota.SampleResult, error) {
					if action != tc.action || client != quota.ClientClaude || id != "session-fixture" {
						t.Fatalf("request = %s %s %s", action, client, id)
					}
					return result, nil
				})
				command.SetArgs([]string{"quota", "sample", string(tc.action), "--client", "claude", "--session", "session-fixture"})
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
				if format == "text" {
					if !strings.HasPrefix(stdout.String(), tc.text) {
						t.Fatalf("text = %q", stdout.String())
					}
					return
				}
				var envelope struct {
					Command string             `json:"command"`
					Data    quota.SampleResult `json:"data"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Command != "quota.sample."+string(tc.action) || envelope.Data.Action != tc.action ||
					envelope.Data.Status != tc.status || !sameQuotaSampleTime(envelope.Data.StartedAt, result.StartedAt) ||
					!sameQuotaSampleTime(envelope.Data.FinishedAt, result.FinishedAt) {
					t.Fatalf("envelope = %+v", envelope)
				}
				for _, field := range []string{"account_digest", "generation", "start_json", "session-fixture"} {
					if strings.Contains(stdout.String(), field) {
						t.Fatalf("output included %q", field)
					}
				}
			})
		}
	}
}

func TestQuotaSampleCommandRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		format  string
		args    []string
		failure quota.SampleErrorCode
		exit    int
		code    string
	}{
		{name: "missing client", format: "json", args: []string{"--session", "fixture"}, exit: 2, code: "invalid_argument"},
		{name: "invalid client", format: "text", args: []string{"--client", "other", "--session", "fixture"}, exit: 2, code: "invalid_argument"},
		{name: "missing session", format: "json", args: []string{"--client", "codex"}, exit: 2, code: "invalid_argument"},
		{name: "positional argument", format: "json", args: []string{"fixture"}, exit: 2, code: "invalid_argument"},
		{name: "unsupported format", format: "ndjson", args: []string{"--client", "codex", "--session", "fixture"}, exit: 2, code: "invalid_argument"},
		{name: "unknown session", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleSessionNotFound, exit: 1, code: "session_not_found"},
		{name: "conflict", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleConflict, exit: 1, code: "measurement_conflict"},
		{name: "no first reading", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleMissing, exit: 1, code: "measurement_missing"},
		{name: "unsettled", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleUnsettled, exit: 1, code: "unsettled"},
		{name: "probe disabled", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleProbeDisabled, exit: 1, code: "probe_disabled"},
		{name: "non-official route", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleNotOfficial, exit: 1, code: "not_official"},
		{name: "failed probe", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleProbeFailed, exit: 1, code: "probe_failed"},
		{name: "account changed", format: "json", args: []string{"--client", "codex", "--session", "fixture"}, failure: quota.SampleAccountChanged, exit: 1, code: "measurement_missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			opts := &commandOptions{format: tc.format, stdout: &stdout}
			command := quotaSampleFixtureCommand(opts, func(context.Context, quota.SampleAction, quota.Client, string) (quota.SampleResult, error) {
				if tc.failure == "" {
					t.Fatal("invalid arguments reached the sampler")
				}
				return quota.SampleResult{}, &quota.SampleError{Code: tc.failure}
			})
			command.SetArgs(append([]string{"quota", "sample", "finish"}, tc.args...))
			selected, err := command.ExecuteC()
			if err == nil || errorExitCode(err) != tc.exit || stdout.Len() != 0 {
				t.Fatalf("error = %v, stdout = %q", err, stdout.String())
			}
			envelope := output.NewError(automationCommandName(selected), errorCode(err), errorMessage(err), time.Time{})
			if envelope.Error == nil || envelope.Error.Code != tc.code {
				t.Fatalf("error envelope = %+v", envelope)
			}
			if tc.failure != "" {
				var sampleErr *quota.SampleError
				if !errors.As(err, &sampleErr) || sampleErr.Code != tc.failure {
					t.Fatalf("sample error = %v, expected %q", err, tc.failure)
				}
				if tc.failure == quota.SampleAccountChanged && envelope.Error.Message != "The sampled account changed" {
					t.Fatalf("account change message = %q", envelope.Error.Message)
				}
			}
		})
	}
}

func quotaSampleFixtureCommand(opts *commandOptions, run func(context.Context, quota.SampleAction, quota.Client, string) (quota.SampleResult, error)) *cobra.Command {
	root := &cobra.Command{Use: "agentdeck", SilenceErrors: true, SilenceUsage: true}
	group := &cobra.Command{Use: "quota"}
	group.AddCommand(newQuotaSampleCommand(opts, run))
	root.AddCommand(group)
	return root
}

func sameQuotaSampleTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func TestLocalQuotaSampleCommandUsesTransactionFacts(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "quota-state")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	execSQL := func(statement string, args ...any) {
		t.Helper()
		if _, err := core.DB.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}

	var migrated bool
	if err := core.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE type='table' AND name='quota_session_samples')`).Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Fatal("numbered migration did not create quota_session_samples")
	}

	baseline := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	now := baseline.Add(time.Minute)
	latestCall := baseline.Add(-time.Minute).Format(time.RFC3339Nano)
	execSQL(`INSERT INTO usage_sessions(client, session_id, first_at, last_at) VALUES ('claude','target',?,?)`, latestCall, latestCall)
	execSQL(`INSERT INTO usage_events(event_key, client, session_id, event_id, event_at, model, source_path, source_offset)
		VALUES ('first','claude','target','first',?,'fixture','fixture',0)`, latestCall)

	selection := func(name string, at time.Time) {
		t.Helper()
		if err := core.RecordSelection(ctx, store.Selection{Client: "claude", ProviderName: name,
			MultiplierSnapshot: "1", SelectedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	observe := func(id, route string, at time.Time) {
		t.Helper()
		execSQL(`INSERT INTO usage_session_observations(client,session_id,observed_at,hook_event,observed_provider,route_effect,delivery_id)
			VALUES ('claude','target',?,'ConfigChange',?,'no_change',?)`, at.Format(time.RFC3339Nano), route, id)
	}
	probeCalls := 0
	var duringProbe func()
	oldService := quotaRefreshService
	quotaRefreshService = func(stateRoot, home string) desktop.Service {
		return desktop.Service{StateRoot: stateRoot, Home: home, Now: func() time.Time { return now },
			QuotaProbeClaudeProse: func(_ context.Context, at time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
				probeCalls++
				if duringProbe != nil {
					duringProbe()
				}
				now = now.Add(time.Second)
				return quota.ClaudeProseResult{Windows: []quota.Observation{{
					Client: quota.ClientClaude, Source: quota.SourceClaudeProse, WindowKey: quota.ClaudeWindowFiveHour,
					UsedPercent: 41, ResetsAt: at.Add(time.Hour), WindowMinutes: 300,
				}}}, nil
			},
		}
	}
	t.Cleanup(func() { quotaRefreshService = oldService })

	call := func(action string, wantCode string, wantStatus quota.MeasurementStatus, wantProbes int) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exit := execute([]string{"--state-dir", root, "--format", "json", "quota", "sample", action, "--client", "claude", "--session", "target"},
			bytes.NewReader(nil), &stdout, &stderr)
		wantExit := 0
		if wantCode != "" {
			wantExit = 1
		}
		if exit != wantExit {
			t.Fatalf("%s: exit=%d, want %d, output=%s %s", action, exit, wantExit, &stdout, &stderr)
		}
		if probeCalls != wantProbes {
			t.Fatalf("%s: %d probes, want %d", action, probeCalls, wantProbes)
		}
		if wantCode != "" {
			var envelope struct {
				Error *struct{ Code string } `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil ||
				envelope.Error == nil || envelope.Error.Code != wantCode || stdout.Len() != 0 {
				t.Fatalf("%s: want %s, output=%s %s, decode=%v", action, wantCode, &stdout, &stderr, err)
			}
			return
		}
		var envelope struct {
			Command string
			Data    quota.SampleResult
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil ||
			envelope.Command != "quota.sample."+action || envelope.Data.Status != wantStatus || stderr.Len() != 0 {
			t.Fatalf("%s: output=%s %s, decode=%v", action, &stdout, &stderr, err)
		}
	}

	call("start", "probe_disabled", "", 0)
	settings := quota.DefaultSettings()
	settings.ProbeEnabled = true
	if err := quota.SaveSettings(ctx, core, settings); err != nil {
		t.Fatal(err)
	}
	call("start", "not_official", "", 0)
	selection("official", baseline.Add(time.Nanosecond))
	observe("stale", "third-party", baseline)
	call("start", "", quota.MeasurementActive, 1)
	call("start", "", quota.MeasurementActive, 1)

	signal := func(index int, id string, at time.Time) {
		t.Helper()
		execSQL(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,source_path)
			VALUES ('claude',?,?,?,'pending','default_work','fixture')`, id, index, at.Format(time.RFC3339Nano))
	}
	signal(1, "target", baseline.Add(-2*time.Minute))
	signal(2, "other", baseline)
	signal(3, "target", baseline)
	call("finish", "unsettled", "", 1)
	execSQL(`UPDATE usage_work_signals SET state='classified' WHERE client='claude' AND session_id='target' AND turn_index=3`)

	duringProbe = func() {
		// Last-at is unchanged. The older call arrived while the probe ran.
		execSQL(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset)
			VALUES ('late','claude','target','late',?,'fixture','fixture',1)`, baseline.Add(-2*time.Minute).Format(time.RFC3339Nano))
	}
	call("finish", "unsettled", "", 2)
	duringProbe = nil
	call("finish", "", quota.MeasurementComplete, 3)
	call("finish", "", quota.MeasurementComplete, 3)

	duringProbe = func() {
		observe("during-probe-away", "third-party", now.Add(time.Nanosecond))
		observe("during-probe-back", "official", now.Add(2*time.Nanosecond))
	}
	call("start", "not_official", "", 4)
	duringProbe = nil
	call("finish", "measurement_missing", "", 4)

	call("start", "", quota.MeasurementActive, 5)
	selection("third-party", now.Add(time.Nanosecond))
	selection("official", now.Add(2*time.Nanosecond))
	call("finish", "measurement_missing", "", 5)
	call("cancel", "", quota.MeasurementMissing, 5)
	call("cancel", "", quota.MeasurementMissing, 5)
}
