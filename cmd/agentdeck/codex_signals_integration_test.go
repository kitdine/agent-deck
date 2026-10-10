package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestCodexCurrentMessageSignalsReachCLIAndDesktop(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	stateRoot, home := filepath.Join(root, "state"), filepath.Join(root, "home")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	source := filepath.Join(home, ".codex", "sessions", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	tool, err := json.Marshal(map[string]any{
		"timestamp": "2026-10-08T12:00:02Z", "type": "response_item",
		"payload": map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "edit",
			"input": "*** Begin Patch\n*** Add File: " + filepath.Join(root, "parser.go") + "\n+package fixture\n*** End Patch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the crash"}]}}`,
		string(tool),
		`{"timestamp":"2026-10-08T12:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"output_tokens":100}}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	service := usage.New(database, home)
	service.MachineIdentity = func(context.Context) (string, error) { return "fixture-machine", nil }
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	report, err := service.Signals(ctx, usage.SignalOptions{Period: "today", Client: "codex", From: from, To: from.AddDate(0, 0, 1), IncludeSub: true})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := renderUsageSignalsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Debugging", "Repair", "2s (median)", "Edit", "1 call"} {
		if !strings.Contains(output.String(), value) {
			t.Fatalf("CLI is missing %q:\n%s", value, output.String())
		}
	}
	builder := desktop.Service{StateRoot: stateRoot, Home: home, Workdir: root, Location: time.UTC,
		Now:               func() time.Time { return from.Add(13 * time.Hour) },
		RegistrationProbe: func(context.Context, bool) doctor.RegistrationDetails { return doctor.RegistrationDetails{} },
	}
	result, err := builder.Build(ctx, desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	signals := result.Snapshot.Sessions.WorkSignals
	var activityFound, workflowFound, toolingFound bool
	for _, item := range signals.Activity.Items {
		if item.Period == "today" && item.Client == "codex" {
			activityFound = item.CostBasis == report.Activity.CostBasis && reflect.DeepEqual(item.Kinds, report.Activity.Kinds)
		}
	}
	for _, item := range signals.Workflow.Items {
		if item.Period == "today" && item.Client == "codex" {
			workflowFound = reflect.DeepEqual(item.FirstEditSeconds, report.Workflow.FirstEditSeconds) &&
				reflect.DeepEqual(item.FilesTouched, report.Workflow.FilesTouched) && reflect.DeepEqual(item.Retries, report.Workflow.Retries)
		}
	}
	for _, item := range signals.Tooling.Items {
		if item.Period == "today" && item.Client == "codex" {
			toolingFound = item.Calls == report.Tooling.Calls && reflect.DeepEqual(item.Rows, report.Tooling.Rows)
		}
	}
	if !activityFound || !workflowFound || !toolingFound {
		t.Fatalf("desktop differs from CLI: activity=%v workflow=%v tooling=%v; warnings=%v", activityFound, workflowFound, toolingFound, result.Warnings)
	}
}
