package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

func telemetryCheck(report Report) *Check {
	for i := range report.Checks {
		if report.Checks[i].Name == "telemetry" {
			return &report.Checks[i]
		}
	}
	return nil
}

func TestTelemetryGapIsReportedOnlyWhenMemoriesNeedsCollection(t *testing.T) {
	ctx := context.Background()
	state, home := filepath.Join(t.TempDir(), "state"), t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	database.Close()
	service := Service{RegistrationProbe: deterministicRegistration, StateRoot: state, Home: home, Vault: doctorVault(state)}
	check := func() *Check {
		report, err := service.Check(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		return telemetryCheck(report)
	}
	if got := check(); got != nil {
		t.Fatalf("gap reported without a Codex configuration: %+v", got)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[features]\nmemories = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := check()
	if got == nil || got.Status != "warning" || got.Code != telemetry.WarningMemoriesNotCounted || got.Recovery != "agentdeck telemetry enable --write-codex-config" {
		t.Fatalf("memories on, collection off = %+v", got)
	}
	if err := telemetry.SaveConfig(state, telemetry.Config{Enabled: true, Port: telemetry.DefaultPort, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	if got := check(); got != nil {
		t.Fatalf("gap reported while collection is enabled: %+v", got)
	}
}
