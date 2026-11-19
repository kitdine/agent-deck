package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
)

// A fixture-only executable for the Swift coordinator test. The same bytes
// emitted by the Task 3 envelope collector are sent to EmbeddedHelperRunner and saved for inspection.
// The ACK is executed by the real command against the same synthetic store.
func TestDesktopQuotaMixedEnvelopePeer(t *testing.T) {
	i := 0
	for ; i < len(os.Args) && os.Args[i] != "sai-mixed-guard-peer"; i++ {
	}
	if i == len(os.Args) {
		t.Skip("Swift coordinator fixture")
	}
	args := os.Args[i+1:]
	home := os.Getenv("AGENTDECK_TEST_QUOTA_ENVELOPE_ROOT")
	if !filepath.IsAbs(home) {
		t.Fatal("missing synthetic envelope root")
	}
	if b, e := os.ReadFile(filepath.Join(home, "test-fixture")); e != nil || string(b) != "synthetic mixed guard\n" {
		t.Fatal("missing test marker")
	}
	t.Setenv("HOME", home)
	state := filepath.Join(home, "state")
	withTestHome(t, home)
	// State resolution supplies an isolated location without consulting any installation.
	if len(args) == 4 && args[0] == "desktop" && args[1] == "state-location" {
		if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"schema_version": 1, "command": "desktop.state-location", "partial": false, "error": nil, "data": map[string]any{"installed": false, "state_root": state}}); e != nil {
			t.Fatal(e)
		}
		os.Exit(0)
	}
	if len(args) < 3 || args[0] != "--format" || args[1] != "json" {
		t.Fatal("unexpected arguments", args)
	}
	flagArgs := append([]string{"--state-dir", state}, args...)
	var output bytes.Buffer
	switch {
	case len(args) == 5 && args[2] == "desktop" && args[3] == "quota-refresh" && args[4] == "--manual":
		// Task 3 has no production pinned refresh yet. Exercise the collector
		// with real due rows and replace the bound inode during its prune.
		// This scalar exists only in the dedicated Swift peer subprocess.
		path := filepath.Join(state, "quota-delivery.lock")
		e := sqlite.RegisterScalarFunction("replace_fixture_guard", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
			if err := os.Rename(path, path+".retained"); err != nil {
				return nil, err
			}
			return int64(1), os.WriteFile(path, nil, 0600)
		})
		if e != nil {
			t.Fatal(e)
		}
		ctx := t.Context()
		core, e := store.Open(ctx, state)
		if e != nil {
			t.Fatal(e)
		}
		defer core.Close()
		core.DB.SetMaxOpenConns(1)
		lock, e := store.AcquireQuotaRefreshLock(ctx, state, 0)
		if e != nil {
			t.Fatal(e)
		}
		defer lock.Release()
		ledger := quota.NewWritableStore(core)
		now := time.Now().UTC()
		cycle, e := ledger.BeginRefreshCycle(ctx, lock, now)
		if e != nil {
			t.Fatal(e)
		}
		guard, e := ledger.ReconcileQuotaDelivery(ctx, cycle)
		if e != nil {
			t.Fatal(e)
		}
		identity, e := guard.ApplyClaudeIdentity(ctx, quota.IdentityObservation{State: quota.IdentitySignedOut}, time.Time{}, time.Time{})
		if e != nil || identity.Generation != 1 {
			t.Fatal(identity, e)
		}
		usage := quota.ClaudeProseResult{Windows: []quota.Observation{{WindowKey: "five_hour", UsedPercent: 80, WindowMinutes: 300, ResetsAt: now.Add(time.Hour)}}}
		written, e := ledger.CommitClaudeUsage(ctx, cycle, identity.Generation, usage, identity)
		if e != nil || written != quota.WriteCommitted {
			t.Fatal(written, e)
		}
		accepted, _, _, e := ledger.Record(ctx, quota.Observation{Client: quota.ClientCodex, WindowKey: "codex", Source: quota.SourceCodex, ObservedAt: now, UsedPercent: 80, WindowMinutes: 300, ResetsAt: now.Add(time.Hour)})
		if e != nil || !accepted {
			t.Fatal(accepted, e)
		}
		settings := quota.Settings{ProbeEnabled: true, AlertsEnabled: true, AlertThresholds: []float64{75}}
		outcomes := map[quota.Client]quota.Reason{quota.ClientClaude: "", quota.ClientCodex: ""}
		fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			t.Fatal(e)
		}
		defer unix.Close(fd)
		if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e != nil {
			t.Fatal(e)
		}
		collect := func() (desktopQuotaRefreshResult, error) {
			return desktopQuotaAlertEnvelope(ctx, ledger, cycle, guard, settings, outcomes, now)
		}
		healthy, e := collect()
		if e != nil || healthy.StateRootID != guard.StateRootID() || len(healthy.Alerts) != 2 || healthy.Alerts[0].Client != "codex" || healthy.Alerts[1].Client != "claude" {
			t.Fatal(healthy, e)
		}

		if _, e = core.DB.ExecContext(ctx, `INSERT INTO quota_alert_notices_v2(client,window_key,kind,instance_unix,notified_at)
            VALUES ('codex','expired','reset',1,'fixture');
            CREATE TEMP TRIGGER damage_guard AFTER DELETE ON quota_alert_notices_v2
            WHEN OLD.window_key='expired' BEGIN SELECT replace_fixture_guard(); END;`); e != nil {
			t.Fatal(e)
		}
		result, e := collect()
		if !errors.Is(e, quota.ErrQuotaDeliveryUnavailable) || result.StateRootID != "" || len(result.Alerts) != 1 || result.Alerts[0].ID != healthy.Alerts[0].ID {
			t.Fatal(result, e)
		}
		if e = writeDesktopQuotaRefreshResult(&output, "json", result, e != nil); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(home, "mixed-envelope.json"), output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	case len(args) == 7 && args[2] == "desktop" && args[3] == "quota-alerts" && args[4] == "ack" && args[5] == "--id":
		if e := run(flagArgs, bytes.NewReader(nil), &output); e != nil {
			t.Fatal(e)
		}
		core, e := store.OpenReadOnly(t.Context(), state)
		if e != nil {
			t.Fatal(e)
		}
		var all, codex int
		if e = core.DB.QueryRow("SELECT count(*), count(CASE WHEN client='codex' THEN 1 END) FROM quota_alert_notices_v2").Scan(&all, &codex); e != nil || all != 1 || codex != 1 {
			t.Fatal("ACK ledger", all, codex, e)
		}
		if e = core.Close(); e != nil {
			t.Fatal(e)
		}
		f, e := os.OpenFile(filepath.Join(home, "ack-ids"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.WriteString(args[6] + "\n"); e != nil {
			t.Fatal(e)
		}
		if e = f.Close(); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(home, "ack-envelope.json"), output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	case len(args) == 3 && args[2] == "quota":
		if e := run(flagArgs, bytes.NewReader(nil), &output); e != nil {
			t.Fatal(e)
		}
	default:
		t.Fatal("unexpected invocation", args)
	}
	if _, e := os.Stdout.Write(output.Bytes()); e != nil {
		t.Fatal(e)
	}
	os.Exit(0)
}
