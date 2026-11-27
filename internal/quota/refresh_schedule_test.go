package quota

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

func refreshFixture(t *testing.T) (context.Context, *RefreshCore, *store.Lock) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), RefreshTimeout)
	t.Cleanup(cancel)
	root := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRefreshCore(ctx, alias)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	lock, err := store.AcquireQuotaRefreshLock(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	})
	return ctx, r, lock
}

func TestRefreshOpenAnchorRejectsReplacement(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	c, err := r.store.BeginRefreshCycle(ctx, lock, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	old := r.root + ".original"
	if err := os.Rename(r.root, old); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(old, r.root)
	if err := os.Mkdir(r.root, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(r.root)
	// Preserve the token as well: a pathname-only lock/pragma check would pass.
	b, err := os.ReadFile(filepath.Join(old, "quota-refresh.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.root, "quota-refresh.lock"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if g, err := r.store.reconcileQuotaDelivery(ctx, c, r); g != nil || !errors.Is(err, ErrQuotaDeliveryUnavailable) {
		t.Fatal("replacement accepted", err)
	}
	if _, err := os.Stat(filepath.Join(r.root, quotaEpochName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("replacement received an epoch", err)
	}
}

func TestRefreshRequiresLockInOpenedCoreDirectory(t *testing.T) {
	for _, name := range []string{"unchanged", "replaced-before-lock"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), RefreshTimeout)
			defer cancel()
			root := filepath.Join(t.TempDir(), "state")
			r, err := OpenRefreshCore(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if name == "replaced-before-lock" {
				if err := os.Rename(root, root+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			lock, err := store.AcquireQuotaRefreshLockBefore(ctx, root, RefreshLockDeadline(ctx))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := lock.Release(); err != nil {
					t.Error(err)
				}
			}()

			probes := 0
			s := Scheduler{Store: r.store, ProbeCodex: func(context.Context, time.Time, time.Duration) (CodexResult, error) {
				probes++
				return CodexResult{AccountID: "synthetic"}, nil
			}}
			c, _, err := s.RunRefresh(ctx, r, lock, TriggerManual, map[Client]Reason{ClientCodex: ""})
			var seq CycleSeq
			if readErr := r.core.DB.QueryRowContext(ctx, "SELECT seq FROM quota_cycle WHERE singleton=1").Scan(&seq); readErr != nil {
				t.Fatal(readErr)
			}
			if name == "replaced-before-lock" {
				if !errors.Is(err, store.ErrLockLost) || c.seq != 0 || seq != 0 || probes != 0 {
					t.Fatalf("want lock refusal before any work; err=%v cycle=%d stored=%d probes=%d", err, c.seq, seq, probes)
				}
			} else if err != nil || c.seq != 1 || seq != 1 || probes != 1 {
				t.Fatalf("want one admitted cycle and probe; err=%v cycle=%d stored=%d probes=%d", err, c.seq, seq, probes)
			}
		})
	}
}

func TestRefreshBudgetAndRetirement(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	count := 0
	s := Scheduler{Store: r.store, Interval: time.Minute, ProbeClaudeIdentity: func(context.Context, string) IdentityObservation {
		count++
		return IdentityObservation{State: IdentitySignedOut}
	}, ProbeClaudeProse: func(context.Context, time.Time, time.Duration) (ClaudeProseResult, error) {
		count++
		return ClaudeProseResult{}, nil
	}}
	allowed := map[Client]Reason{ClientClaude: ""}
	short, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if RefreshLockWait(short) > 0 {
		t.Fatal("insufficient reserve")
	}
	if _, _, err := s.RunRefresh(short, r, lock, TriggerManual, allowed); !errors.Is(err, store.ErrStateBusy) {
		t.Fatal(err)
	}
	if _, _, err := s.RunRefresh(ctx, r, lock, TriggerManual, map[Client]Reason{ClientClaude: ReasonNotOfficial}); err != nil {
		t.Fatal(err)
	}
	var seq int
	if err := r.core.DB.QueryRow("SELECT seq FROM quota_cycle").Scan(&seq); err != nil || seq != 0 || count != 0 {
		t.Fatal("gate/budget wrote or probed", seq, count, err)
	}
	s.ProbeClaudeIdentity = func(context.Context, string) IdentityObservation {
		count++
		// Model a superseding cycle between observation and guarded write.
		if _, err := r.store.BeginRefreshCycle(ctx, lock, time.Now()); err != nil {
			t.Fatal(err)
		}
		return IdentityObservation{State: IdentitySignedOut}
	}
	_, g, err := s.RunRefresh(ctx, r, lock, TriggerManual, allowed)
	if err != nil || g != nil || count != 1 {
		t.Fatal("stale identity reached usage", count, err)
	}
	if err := r.core.DB.QueryRow("SELECT count(*) FROM quota_identities").Scan(&seq); err != nil || seq != 0 {
		t.Fatal("stale attempt written")
	}
}

func TestRefreshIndependentBackoffsAndRecovery(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	now := time.Now().UTC()
	idCalls, usageCalls := 0, 0
	fail := true
	obs := syntheticClaudeIdentity(1, 1)
	s := Scheduler{Store: r.store, Interval: time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now },
		ProbeClaudeIdentity: func(context.Context, string) IdentityObservation {
			idCalls++
			if fail {
				return IdentityObservation{Failure: IdentityIncomparable}
			}
			return obs
		},
		ProbeClaudeProse: func(context.Context, time.Time, time.Duration) (ClaudeProseResult, error) {
			usageCalls++
			if fail {
				return ClaudeProseResult{}, &ClaudeFailureError{Kind: ClaudeFailureParse, Err: &ProseParseError{Detail: DetailMissingLine}}
			}
			return ClaudeProseResult{Windows: []Observation{{WindowKey: "five_hour", WindowMinutes: 300, UsedPercent: 80, ResetsAt: now.Add(time.Hour)}}}, nil
		}}
	run := func(tr Trigger) RefreshCycle {
		t.Helper()
		c, g, err := s.RunRefresh(ctx, r, lock, tr, map[Client]Reason{ClientClaude: ""})
		if err != nil || g == nil {
			t.Fatal("no live guard", err)
		}
		return c
	}
	run(TriggerBackground)
	now = now.Add(time.Second)
	run(TriggerManual)
	if idCalls != 1 || usageCalls != 2 {
		t.Fatal("manual retry broke backoff", idCalls, usageCalls)
	}
	var base, until, detail string
	if err := r.core.DB.QueryRow("SELECT failure_at,backoff_until,failure_detail FROM quota_envelopes_v2 WHERE client='claude'").Scan(&base, &until, &detail); err != nil || base != timeValue(now.Add(-time.Second)) || until != timeValue(now.Add(59*time.Second)) || detail != "missing_line" {
		t.Fatal(base, until, detail, err)
	}
	now = now.Add(time.Minute)
	fail = false
	run(TriggerBackground)
	now = now.Add(time.Minute)
	obs = syntheticClaudeIdentity(1, 2)
	c := run(TriggerBackground)
	state, err := r.store.CommittedClaudeContext(ctx, c)
	if err != nil || state.Generation != 1 || state.ConfirmedGeneration != 1 || state.Gate != CaptureOpen || state.OpenedAt.Boot == "" {
		t.Fatal(state, err)
	}
	// Crash between tx1 and tx2: the next not-due refresh reopens the gate.
	if _, err = r.core.DB.Exec(`UPDATE quota_envelopes_v2 SET capture_gate='blocked',capture_open_boot='',capture_open_mono=0 WHERE client='claude'`); err != nil {
		t.Fatal(err)
	}
	id0, u0 := idCalls, usageCalls
	c = run(TriggerBackground)
	state, err = r.store.CommittedClaudeContext(ctx, c)
	if err != nil || state.Gate != CaptureOpen || idCalls != id0 || usageCalls != u0 {
		t.Fatal("recovery required probing", state, err)
	}
}

func TestRefreshPruningBoundedByWriter(t *testing.T) {
	ctx, r, _ := refreshFixture(t)
	conn, err := r.core.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, "ROLLBACK")
	start := time.Now()
	if err := r.store.pruneAlertNotices(ctx, time.Now()); err == nil || time.Since(start) > 2*time.Second {
		t.Fatal("prune did not use bounded immediate transaction", err, time.Since(start))
	}
}

func TestRefreshGuardBusyAfterIdentityProbe(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	s := Scheduler{Store: r.store, Interval: time.Minute, ProbeClaudeIdentity: func(context.Context, string) IdentityObservation {
		fd, err := unix.Open(filepath.Join(r.root, deliveryName), deliveryOpenFlags, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { unix.Close(fd) })
		if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		return IdentityObservation{State: IdentitySignedOut}
	}, ProbeClaudeProse: func(context.Context, time.Time, time.Duration) (ClaudeProseResult, error) {
		t.Fatal("busy identity commit launched usage")
		return ClaudeProseResult{}, nil
	}}
	c, g, err := s.RunRefresh(ctx, r, lock, TriggerManual, map[Client]Reason{ClientClaude: ""})
	if !errors.Is(err, ErrQuotaDeliveryBusy) || c.seq == 0 || g != nil {
		t.Fatal("busy transition published proof", err)
	}
	var writes int
	if err := r.core.DB.QueryRow(`SELECT count(*) FROM quota_identities`).Scan(&writes); err != nil || writes != 0 {
		t.Fatal("busy transition recorded attempt", err)
	}
}

func TestRefreshBlockedJoinsOffsetCadences(t *testing.T) {
	for _, seed := range []string{"usage-due", "identity-due", "identity-backoff", "usage-backoff", "manual-usage-backoff", "neither-due"} {
		t.Run(seed, func(t *testing.T) {
			ctx, r, lock := refreshFixture(t)
			now := time.Now().UTC()
			idAt, usageAt := now, now
			ib, ub := "", ""
			trigger := TriggerBackground
			wantID, wantUsage := 1, 1
			switch seed {
			case "usage-due":
				usageAt = now.Add(-time.Hour)
			case "identity-due":
				idAt = now.Add(-time.Hour)
			case "identity-backoff":
				usageAt = now.Add(-time.Hour)
				ib = timeValue(now.Add(time.Hour))
				wantID = 0
			case "usage-backoff":
				idAt = now.Add(-time.Hour)
				ub = timeValue(now.Add(time.Hour))
				wantUsage = 0
			case "manual-usage-backoff":
				ub = timeValue(now.Add(time.Hour))
				trigger = TriggerManual
			case "neither-due":
				wantID, wantUsage = 0, 0
			}
			if _, err := r.core.DB.Exec(`INSERT INTO quota_identities(client,attempted_at,backoff_until) VALUES('claude',?,?)`, timeValue(idAt), ib); err != nil {
				t.Fatal(err)
			}
			if _, err := r.core.DB.Exec(`INSERT INTO quota_envelopes_v2(client,context_generation,capture_gate,confirmed_generation,observed_at,backoff_until) VALUES('claude',1,'blocked',-1,?,?)`, timeValue(usageAt), ub); err != nil {
				t.Fatal(err)
			}
			ids, uses := 0, 0
			s := Scheduler{Store: r.store, Interval: time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }, ProbeClaudeIdentity: func(context.Context, string) IdentityObservation { ids++; return syntheticClaudeIdentity(1, 1) }, ProbeClaudeProse: func(context.Context, time.Time, time.Duration) (ClaudeProseResult, error) {
				uses++
				return ClaudeProseResult{Windows: []Observation{{WindowKey: "five_hour", UsedPercent: 12}}}, nil
			}}
			_, g, err := s.RunRefresh(ctx, r, lock, trigger, map[Client]Reason{ClientClaude: ""})
			if err != nil || g == nil || ids != wantID || uses != wantUsage {
				t.Fatalf("id=%d usage=%d guard=%t err=%v", ids, uses, g != nil, err)
			}
		})
	}
}

func TestRecoveryReservesOnlyOneGateWrite(t *testing.T) {
	for _, name := range []string{"changed", "unchanged", "unchanged_near_cutoff"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				defer store.CloseReadPools()
				change := name == "changed"
				ctx, r, lock := refreshFixture(t)
				account := byte(1)
				usageCalls := 0
				usedPercent := 80.0
				s := Scheduler{Store: r.store, Interval: time.Minute,
					ProbeClaudeIdentity: func(context.Context, string) IdentityObservation { return syntheticClaudeIdentity(1, account) },
					ProbeClaudeProse: func(_ context.Context, at time.Time, _ time.Duration) (ClaudeProseResult, error) {
						usageCalls++
						return ClaudeProseResult{Windows: []Observation{{WindowKey: "five_hour", WindowMinutes: 300, UsedPercent: usedPercent, ResetsAt: at.Add(time.Hour)}}}, nil
					},
				}
				run := func(tr Trigger) ClaudeContext {
					t.Helper()
					c, _, err := s.RunRefresh(ctx, r, lock, tr, map[Client]Reason{ClientClaude: ""})
					if err != nil {
						t.Fatal(err)
					}
					state, err := r.store.CommittedClaudeContext(ctx, c)
					if err != nil {
						t.Fatal(err)
					}
					return state
				}
				run(TriggerManual)
				account++
				previous := run(TriggerManual)
				if previous.Gate != CaptureOpen {
					t.Fatal(previous)
				}
				if _, err := r.core.DB.Exec("UPDATE quota_envelopes_v2 SET capture_open_boot='prior-boot' WHERE client='claude'"); err != nil {
					t.Fatal(err)
				}
				if change {
					account++
				}
				// The new window must be newer than the seeded observation.
				time.Sleep(time.Second)
				usageCalls = 0
				usedPercent = 81
				var got ClaudeContext
				if name == "unchanged_near_cutoff" {
					cycle, err := r.store.BeginRefreshCycle(ctx, lock, s.now())
					if err != nil {
						t.Fatal(err)
					}
					guard, err := r.store.reconcileQuotaDelivery(ctx, cycle, r)
					if err != nil {
						t.Fatal(err)
					}
					// A usage probe, its write and cleanup, and reporting fit within 16s.
					// Reserving another gate write after recovery would require 17s.
					limited, cancel := context.WithTimeout(ctx, 16*time.Second)
					defer cancel()
					s.runGuardedClaude(limited, cycle, guard, TriggerManual)
					got, err = r.store.CommittedClaudeContext(ctx, cycle)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					got = run(TriggerManual)
				}
				if !change {
					if usageCalls != 1 || got.Gate != CaptureOpen || got.Generation != previous.Generation {
						t.Fatal("unchanged account did not refresh usage", usageCalls, got)
					}
					windows, err := r.store.Windows(ctx, ClientClaude)
					if err != nil || len(windows) != 1 || windows[0].UsedPercent != usedPercent {
						t.Fatalf("unchanged account did not commit fresh usage: %+v, %v", windows, err)
					}
					return
				}
				if usageCalls != 0 || got.Gate != CaptureBlocked || got.ConfirmedGeneration == got.Generation || got.Generation != previous.Generation+1 {
					t.Fatal("usage started without its gate-write budget", usageCalls, got)
				}
				next := run(TriggerBackground)
				if usageCalls != 1 || next.Gate != CaptureOpen || next.ConfirmedGeneration != got.Generation {
					t.Fatal("deferred usage was not due next cycle", usageCalls, next)
				}
			})
		})
	}
}
