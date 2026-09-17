package desktop

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

// Explicit opt-in stage measurement against an isolated populated state.
func TestSnapshotStageProfile(t *testing.T) {
	root := os.Getenv("AGENTDECK_SNAPSHOT_STAGE_STATE")
	out := os.Getenv("AGENTDECK_SNAPSHOT_STAGE_REPORT")
	if root == "" || out == "" {
		t.Skip("isolated state and report not configured")
	}
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	s := Service{StateRoot: root, Home: os.Getenv("AGENTDECK_SNAPSHOT_STAGE_HOME"), Workdir: root, Now: func() time.Time { return now }, Location: time.UTC, Vault: credentialvault.New(root, platform.MachineIdentity)}
	ctx := context.Background()
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	samples := []map[string]float64{}
	for i := 0; i < 2; i++ {
		result := Result{}
		spans := map[string]float64{}
		measure := func(name string, f func()) {
			start := time.Now()
			f()
			spans[name] = float64(time.Since(start)) / float64(time.Millisecond)
		}
		measure("provider", func() { s.loadProvider(ctx, core, &result) })
		measure("usage", func() { s.loadUsage(ctx, core, now, &result) })
		measure("work_signals", func() { s.loadWorkSignals(ctx, core, now, &result) })
		measure("sessions", func() { s.loadSessions(ctx, 20, now, &result) })
		measure("health", func() { s.loadHealth(ctx, &result) })
		if result.Partial {
			t.Fatalf("partial stage result: %v", result.Warnings)
		}
		samples = append(samples, spans)
	}
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotSignalsPairedProfile(t *testing.T) {
	root := os.Getenv("AGENTDECK_SNAPSHOT_STAGE_STATE")
	out := os.Getenv("AGENTDECK_SNAPSHOT_STAGE_REPORT")
	if root == "" || out == "" {
		t.Skip("isolated state/report not configured")
	}
	ctx := context.Background()
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	s := usage.New(core, os.Getenv("AGENTDECK_SNAPSHOT_STAGE_HOME"))
	s.Now = func() time.Time { return now }
	var options []usage.SignalOptions
	for _, period := range desktopPeriodRanges(now, time.UTC) {
		for _, client := range []string{"", "codex", "claude"} {
			options = append(options, usage.SignalOptions{Period: period.name, From: period.start, To: period.end, Client: client, IncludeSub: true})
		}
	}
	var reference []usage.SignalReport
	type sample struct {
		Mode string  `json:"mode"`
		MS   float64 `json:"ms"`
	}
	var samples []sample
	for _, mode := range []string{"individual", "batch", "batch", "individual"} {
		started := time.Now()
		var reports []usage.SignalReport
		if mode == "batch" {
			reports, err = s.SignalsBatch(ctx, options)
		} else {
			for _, o := range options {
				r, e := s.Signals(ctx, o)
				if e != nil {
					err = e
					break
				}
				reports = append(reports, r)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, sample{Mode: mode, MS: float64(time.Since(started)) / float64(time.Millisecond)})
		if reference == nil {
			reference = reports
		} else if !reflect.DeepEqual(reference, reports) {
			t.Fatal("batch differed from individual reference")
		}
	}
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
