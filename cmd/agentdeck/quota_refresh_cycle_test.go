package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

func TestDesktopQuotaRefreshGuardedClaudeCycle(t *testing.T) {
	ctx := context.Background()
	withTestHome(t, t.TempDir())
	state := filepath.Join(t.TempDir(), "state")
	seedQuotaState(t, state, func(s *quota.Settings) {
		s.ProbeEnabled = true
		s.ProbeClients = []quota.Client{quota.ClientClaude}
		s.AlertsEnabled = true
		s.AlertThresholds = []float64{75}
	}, "claude")
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	prev := quotaRefreshService
	t.Cleanup(func() { quotaRefreshService = prev })
	identity := quota.IdentitySignedOut
	want := 1
	quotaRefreshService = func(root, home string) desktop.Service {
		return desktop.Service{
			QuotaProbeClaudeIdentity: func(ctx context.Context, path string) quota.IdentityObservation {
				if root != path {
					t.Fatal("identity root mismatch")
				}
				d, ok := ctx.Deadline()
				if !ok || time.Until(d) > 37*time.Second {
					t.Fatal("no helper deadline")
				}
				l, err := store.AcquireQuotaRefreshLock(ctx, path, 0)
				if err == nil {
					l.Release()
					t.Fatal("probe outside refresh lock")
				}
				return quota.IdentityObservation{State: identity}
			},
			QuotaProbeClaudeProse: func(ctx context.Context, at time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
				var generation int
				if err := core.DB.QueryRowContext(ctx, `SELECT context_generation FROM quota_envelopes_v2 WHERE client='claude'`).Scan(&generation); err != nil || generation != want {
					t.Fatal("usage before committed generation", generation, err)
				}
				b, err := os.ReadFile(filepath.Join(root, "quota-delivery.epoch"))
				if err != nil || len(b) != 24 || binary.BigEndian.Uint64(b[16:]) != uint64(want) {
					t.Fatal("usage before reconciliation", err)
				}
				return quota.ClaudeProseResult{Windows: []quota.Observation{{WindowKey: "five_hour", UsedPercent: 80, WindowMinutes: 300, ResetsAt: at.Add(time.Hour)}}}, nil
			}}
	}
	refresh := func() string {
		t.Helper()
		data := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
		a := quotaRefreshAlerts(t, data)
		id, _ := data["state_root_id"].(string)
		if len(a) != 1 || a[0]["generation"] != float64(want) || id == "" {
			t.Fatal(data)
		}
		return a[0]["id"].(string)
	}
	old := refresh()
	identity = quota.IdentityNonSubscription
	want = 2
	fresh := refresh()
	if old == fresh {
		t.Fatal("generation not bound to alert")
	}
	ack := runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-alerts", "ack", "--id", old)
	if ack["acknowledged"] != 0.0 {
		t.Fatal("stale acknowledgement", ack)
	}
}

func TestDesktopQuotaRefreshClaudeFixtureExecutable(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	t.Setenv("HOME", home)
	t.Setenv("PATH", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "claude-config"))
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HOME/calls"
case "$*" in
'auth status --json') printf '%s\n' '{"loggedIn":false}';;
'-p /usage') printf '%s\n' 'Current session: 80% used · resets Sep 9 at 1:50am (America/Los_Angeles)' 'Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)';;
*) exit 9;;
esac
`
	if err := os.WriteFile(filepath.Join(home, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(home, "state")
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeEnabled = true; s.ProbeClients = []quota.Client{quota.ClientClaude} }, "claude")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if b, err := os.ReadFile(filepath.Join(home, "calls")); err != nil || string(b) != "auth status --json\n-p /usage\n" {
		t.Fatal("unexpected fixture invocations", string(b), err)
	}
}

func TestDesktopQuotaIndependentGuardPartialWarning(t *testing.T) {
	for _, mode := range []string{"missing", "replaced", "after-identity-probe", "busy", "missing-during-usage", "held-during-usage"} {
		t.Run(mode, func(t *testing.T) {
			duringUsage := mode == "missing-during-usage" || mode == "held-during-usage"
			state := filepath.Join(t.TempDir(), "state")
			withTestHome(t, t.TempDir())
			seedQuotaState(t, state, func(s *quota.Settings) {
				s.ProbeEnabled = true
				s.AlertsEnabled = true
				s.AlertThresholds = []float64{75}
			}, "codex", "claude")
			fake := withFakeQuotaRefresh(t, 80)
			// Seed a successful identity so the following manual cycle is eligible.
			baseService := quotaRefreshService
			quotaRefreshService = func(root, home string) desktop.Service {
				s := baseService(root, home)
				s.QuotaProbeClaudeIdentity = func(context.Context, string) quota.IdentityObservation {
					return quota.IdentityObservation{State: quota.IdentitySignedOut}
				}
				if duringUsage {
					probe := s.QuotaProbeClaudeProse
					s.QuotaProbeClaudeProse = func(ctx context.Context, at time.Time, timeout time.Duration) (quota.ClaudeProseResult, error) {
						_, err := probe(ctx, at, timeout)
						return quota.ClaudeProseResult{Windows: []quota.Observation{{
							WindowKey: "five_hour", UsedPercent: 80, WindowMinutes: 300, ResetsAt: at.Add(time.Hour),
						}}}, err
					}
				}
				return s
			}
			args := []string{"--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual"}
			warm := runJSON(t, args...)
			if duringUsage && len(quotaRefreshAlerts(t, warm)) != 2 {
				t.Fatalf("expected both clients in the healthy control: %+v", warm)
			}
			before := *fake
			path := filepath.Join(state, "quota-delivery.lock")
			old, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer unix.Close(old)
			damage := func() {
				t.Helper()
				if e := unix.Flock(old, unix.LOCK_EX|unix.LOCK_NB); e != nil {
					t.Fatal(e)
				}
				if mode != "busy" && mode != "held-during-usage" {
					if e := os.Remove(path); e != nil {
						t.Fatal(e)
					}
				}
				if mode == "replaced" || mode == "after-identity-probe" {
					if e := os.WriteFile(path, nil, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			if mode == "after-identity-probe" {
				base := quotaRefreshService
				quotaRefreshService = func(root, home string) desktop.Service {
					s := base(root, home)
					s.QuotaProbeClaudeIdentity = func(context.Context, string) quota.IdentityObservation {
						damage()
						return quota.IdentityObservation{State: quota.IdentitySignedOut}
					}
					return s
				}
			} else if duringUsage {
				base := quotaRefreshService
				quotaRefreshService = func(root, home string) desktop.Service {
					s := base(root, home)
					probe := s.QuotaProbeClaudeProse
					s.QuotaProbeClaudeProse = func(ctx context.Context, at time.Time, timeout time.Duration) (quota.ClaudeProseResult, error) {
						usage, err := probe(ctx, at, timeout)
						damage()
						return usage, err
					}
					return s
				}
			} else {
				damage()
			}
			var out bytes.Buffer
			began := time.Now()
			if e := run(args, bytes.NewReader(nil), &out); e != nil {
				t.Fatal(e)
			}
			elapsed := time.Since(began)
			var env struct {
				Partial  bool
				Warnings []string
				Data     desktopQuotaRefreshResult
			}
			if e := json.Unmarshal(out.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			if mode == "held-during-usage" {
				if env.Partial || len(env.Warnings) != 0 || len(env.Data.Alerts) != 2 || env.Data.StateRootID == "" {
					t.Fatal("a held but valid guard must still vouch for the result:", out.String())
				}
			} else if !env.Partial || len(env.Warnings) != 1 || env.Warnings[0] != "quota_alerts_failed" || len(env.Data.Alerts) != 1 || env.Data.Alerts[0].Client != "codex" || env.Data.StateRootID != "" {
				t.Fatal(out.String())
			}
			wantClaudeCalls := before.claudeCalls
			if duringUsage {
				wantClaudeCalls++
			}
			if fake.claudeCalls != wantClaudeCalls || fake.codexCalls != before.codexCalls+1 {
				t.Fatal("invalid probe counts", before, *fake)
			}
			if elapsed >= quota.RefreshTimeout {
				t.Fatal("cycle over budget", elapsed)
			}
			t.Logf("guard scenario %s: %d alert(s), partial=%t, elapsed %v", mode, len(env.Data.Alerts), env.Partial, elapsed)
		})
	}
}

// A fixture-only executable for the Swift coordinator test. The same bytes
// emitted by run are sent to EmbeddedHelperRunner and saved for inspection.
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
		seedQuotaState(t, state, func(s *quota.Settings) {
			s.ProbeEnabled = true
			s.AlertsEnabled = true
			s.AlertThresholds = []float64{75}
		}, "codex", "claude")
		fake := withFakeQuotaRefresh(t, 80)
		base := quotaRefreshService
		usage := 0
		quotaRefreshService = func(root, home string) desktop.Service {
			s := base(root, home)
			s.QuotaProbeClaudeIdentity = func(context.Context, string) quota.IdentityObservation {
				return quota.IdentityObservation{State: quota.IdentitySignedOut}
			}
			s.QuotaProbeClaudeProse = func(_ context.Context, at time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
				usage++
				return quota.ClaudeProseResult{Windows: []quota.Observation{{WindowKey: "five_hour", UsedPercent: 80, WindowMinutes: 300, ResetsAt: at.Add(time.Hour)}}}, nil
			}
			return s
		}
		warm := quotaRefreshAlerts(t, runJSON(t, flagArgs...))
		if len(warm) != 2 || warm[0]["client"] == warm[1]["client"] || usage != 1 {
			t.Fatal("need a genuinely due notice for both clients", warm)
		}
		path := filepath.Join(state, "quota-delivery.lock")
		old, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			t.Fatal(e)
		}
		defer unix.Close(old)
		if e = unix.Flock(old, unix.LOCK_EX|unix.LOCK_NB); e != nil {
			t.Fatal(e)
		}
		if e = os.Rename(path, path+".retained"); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, nil, 0600); e != nil {
			t.Fatal(e)
		}
		if e = run(flagArgs, bytes.NewReader(nil), &output); e != nil {
			t.Fatal(e)
		}
		if usage != 1 || fake.codexCalls != 2 {
			t.Fatal("unexpected probe calls", usage, *fake)
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
