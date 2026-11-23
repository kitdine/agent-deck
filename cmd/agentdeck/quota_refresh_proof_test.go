package main

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
)

// Run the actual command in a private subprocess so the fault-injection
// SQLite function cannot be registered on another test's connections.
func TestDesktopQuotaRefreshLossDuringCollection(t *testing.T) {
	const marker = "AGENTDECK_TEST_QUOTA_LATE_PROOF"
	if os.Getenv(marker) != "child" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDesktopQuotaRefreshLossDuringCollection$", "-test.v")
		cmd.Env = append(os.Environ(), marker+"=child")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated helper: %v\n%s", err, out)
		}
		return
	}
	state := filepath.Join(t.TempDir(), "state")
	withTestHome(t, t.TempDir())
	if err := sqlite.RegisterScalarFunction("lose_fixture_refresh_lock", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		return int64(1), os.Remove(filepath.Join(state, "quota-refresh.lock"))
	}); err != nil {
		t.Fatal(err)
	}
	seedQuotaState(t, state, func(s *quota.Settings) {
		s.ProbeEnabled, s.AlertsEnabled, s.AlertThresholds = true, true, []float64{75}
	}, "codex", "claude")
	withFakeQuotaRefresh(t, 80)
	service := quotaRefreshService
	reset := time.Now().Add(time.Hour)
	quotaRefreshService = func(root, home string) desktop.Service {
		s := service(root, home)
		s.QuotaProbeClaudeIdentity = func(context.Context, string) quota.IdentityObservation {
			return quota.IdentityObservation{State: quota.IdentitySignedOut}
		}
		s.QuotaProbeClaudeProse = func(context.Context, time.Time, time.Duration) (quota.ClaudeProseResult, error) {
			return quota.ClaudeProseResult{Windows: []quota.Observation{{WindowKey: "five_hour", UsedPercent: 80, WindowMinutes: 300, ResetsAt: reset}}}, nil
		}
		return s
	}
	args := []string{"--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual"}
	healthy := quotaRefreshAlerts(t, runJSON(t, args...))
	if len(healthy) != 2 || healthy[0]["client"] != "codex" || healthy[1]["client"] != "claude" {
		t.Fatalf("healthy envelope: %v", healthy)
	}
	core, err := store.Open(t.Context(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	for _, tc := range []struct{ name, fault string }{
		{"sequence", "UPDATE quota_cycle SET seq=seq+1 WHERE singleton=1"},
		{"lock", "SELECT lose_fixture_refresh_lock()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := core.DB.Exec(`INSERT INTO quota_alert_notices_v2(client,window_key,kind,instance_unix,notified_at) VALUES ('codex','expired','reset',1,'fixture');
				CREATE TRIGGER lose_proof AFTER DELETE ON quota_alert_notices_v2 WHEN OLD.window_key='expired' BEGIN ` + tc.fault + `; END;`)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := core.DB.Exec("DROP TRIGGER lose_proof"); err != nil {
					t.Error(err)
				}
			}()
			var out bytes.Buffer
			if err := run(args, bytes.NewReader(nil), &out); err != nil {
				t.Fatal(err)
			}
			var env struct {
				Partial  bool
				Warnings []string
				Data     desktopQuotaRefreshResult
			}
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if !env.Partial || len(env.Warnings) != 1 || env.Warnings[0] != "quota_alerts_failed" || env.Data.StateRootID != "" || len(env.Data.Alerts) != 1 || env.Data.Alerts[0].ID != healthy[0]["id"] {
				t.Fatalf("lost proof: %s", out.String())
			}
		})
	}
}
