package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	corestore "github.com/kitdine/agent-deck/internal/store"
)

func newIdentityStore(t *testing.T) (context.Context, *Store, *corestore.Lock, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	core, err := corestore.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	lock, err := corestore.AcquireQuotaRefreshLock(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lock.Release() })
	return ctx, NewWritableStore(core), lock, root
}

func startIdentityCycle(t *testing.T, ctx context.Context, s *Store, lock *corestore.Lock, at time.Time) RefreshCycle {
	t.Helper()
	cycle, err := s.BeginRefreshCycle(ctx, lock, at)
	if err != nil {
		t.Fatal(err)
	}
	return cycle
}

func requireQuotaOutcome(t *testing.T, got WriteOutcome, err error, want WriteOutcome) {
	t.Helper()
	if err != nil || got != want {
		t.Fatalf("outcome=%v expected=%v: %v", got, want, err)
	}
}

func syntheticClaudeIdentity(key, correlation byte) IdentityObservation {
	return IdentityObservation{
		State: IdentityIdentified, Display: IdentityDisplay{EmailMasked: "a***z@example.test", OrganizationKind: OrgPersonal},
		claude: &claudeComparison{keyID: [8]byte{key}, correlation: [32]byte{correlation}},
	}
}

func TestClaudeIdentityGuardRollsBackAndReconcilesEpoch(t *testing.T) {
	ctx, s, lock, _ := newIdentityStore(t)
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	first := startIdentityCycle(t, ctx, s, lock, at)
	mirror := Generation(0)
	updateMirror := func(_ context.Context, next Generation) error {
		mirror = next
		return nil
	}
	accepted, err := s.ApplyClaudeIdentity(ctx, first, syntheticClaudeIdentity(1, 1), time.Time{}, time.Time{}, updateMirror)
	if err != nil || accepted.Outcome != WriteCommitted || accepted.proof == nil || mirror != 0 {
		t.Fatalf("first comparable identity outcome=%v mirror=%d: %v", accepted.Outcome, mirror, err)
	}
	if _, err := s.db.Exec("INSERT INTO quota_alert_notices_v2(client,window_key,kind,instance_unix,notified_at) VALUES('claude','five_hour','threshold',1,'old')"); err != nil {
		t.Fatal(err)
	}
	second := startIdentityCycle(t, ctx, s, lock, at.Add(-time.Hour))
	replayed, err := s.ApplyClaudeIdentity(ctx, first, syntheticClaudeIdentity(1, 9), time.Time{}, time.Time{}, updateMirror)
	requireQuotaOutcome(t, replayed.Outcome, err, WriteDropped)
	fault := errors.New("isolated mirror synchronization failure")
	changed := syntheticClaudeIdentity(1, 2)
	failed, err := s.ApplyClaudeIdentity(ctx, second, changed, time.Time{}, time.Time{}, func(_ context.Context, next Generation) error {
		mirror = next
		return fault
	})
	if !errors.Is(err, fault) || failed.Outcome != WriteUnresolved || failed.proof != nil || mirror != 1 {
		t.Fatalf("rollback outcome=%v mirror=%d: %v", failed.Outcome, mirror, err)
	}
	committed, err := s.CommittedClaudeContext(ctx, second)
	if err != nil || committed.Generation != 0 || committed.Gate != CaptureOpenInitial {
		t.Fatalf("committed state after rollback = %+v, %v", committed, err)
	}
	mirror = committed.Generation
	var notices int
	if err := s.db.QueryRow("SELECT count(*) FROM quota_alert_notices_v2").Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("rollback lost old notices %d: %v", notices, err)
	}
	applied, err := s.ApplyClaudeIdentity(ctx, second, changed, time.Time{}, time.Time{}, updateMirror)
	if err != nil || applied.Outcome != WriteCommitted || applied.Generation != 1 || mirror != 1 || applied.Invalidation != InvalidatedContextChanged {
		t.Fatalf("retry outcome=%v generation=%d mirror=%d: %v", applied.Outcome, applied.Generation, mirror, err)
	}
	again, err := s.ApplyClaudeIdentity(ctx, second, changed, time.Time{}, time.Time{}, updateMirror)
	requireQuotaOutcome(t, again.Outcome, err, WriteDropped)
}

func TestClaudeUsageCommitAndCapturePublishAreSeparate(t *testing.T) {
	ctx, s, lock, _ := newIdentityStore(t)
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	guard := func(context.Context, Generation) error { return nil }
	first := startIdentityCycle(t, ctx, s, lock, at)
	_, err := s.ApplyClaudeIdentity(ctx, first, syntheticClaudeIdentity(1, 1), time.Time{}, time.Time{}, guard)
	if err != nil {
		t.Fatal(err)
	}
	second := startIdentityCycle(t, ctx, s, lock, at.Add(time.Minute))
	identity, err := s.ApplyClaudeIdentity(ctx, second, syntheticClaudeIdentity(2, 1), time.Time{}, time.Time{}, guard)
	if err != nil || identity.Invalidation != InvalidatedIdentityKeyReset {
		t.Fatalf("reset classification=%q: %v", identity.Invalidation, err)
	}
	windows := []Observation{{WindowKey: "five_hour", UsedPercent: 25, ResetsAt: at.Add(time.Hour)}}
	stamp := BootStamp{Boot: "boot-a", Mono: 20}
	out, err := s.PublishClaudeCaptureGate(ctx, second, 1, stamp)
	requireQuotaOutcome(t, out, err, WriteDropped)
	out, err = s.CommitClaudeUsage(ctx, second, 0, ClaudeProseResult{Windows: windows}, identity)
	requireQuotaOutcome(t, out, err, WriteDropped)
	out, err = s.CommitClaudeUsage(ctx, second, 1, ClaudeProseResult{Windows: windows}, identity)
	requireQuotaOutcome(t, out, err, WriteCommitted)
	state, err := s.CommittedClaudeContext(ctx, second)
	if err != nil || state.Gate != CaptureBlocked || state.ConfirmedGeneration != 1 || !state.InvalidatedAt.IsZero() {
		t.Fatalf("after usage=%+v: %v", state, err)
	}
	out, err = s.RecordStatusLineCapture(ctx, at.Add(2*time.Minute), nil, windows)
	requireQuotaOutcome(t, out, err, WriteDropped)
	third := startIdentityCycle(t, ctx, s, lock, at.Add(2*time.Minute))
	out, err = s.PublishClaudeCaptureGate(ctx, third, 1, stamp)
	requireQuotaOutcome(t, out, err, WriteCommitted)
	out, err = s.RecordStatusLineCapture(ctx, at.Add(2*time.Minute), &stamp, windows)
	requireQuotaOutcome(t, out, err, WriteDropped)
	newer := BootStamp{Boot: "boot-a", Mono: 21}
	out, err = s.RecordStatusLineCapture(ctx, at.Add(2*time.Minute), &newer, windows)
	requireQuotaOutcome(t, out, err, WriteCommitted)
}

func TestQuotaIdentityManualFailurePreservesBackgroundPair(t *testing.T) {
	ctx, s, lock, _ := newIdentityStore(t)
	at := time.Now().UTC().Truncate(time.Second)
	bg := startIdentityCycle(t, ctx, s, lock, at)
	bgFailure := UsageFailure{Reason: ReasonProbeFailed, FailureObservedAt: at, FailureAt: at, BackoffUntil: at.Add(time.Hour)}
	out, err := s.CommitClaudeUsageFailure(ctx, bg, 0, bgFailure)
	requireQuotaOutcome(t, out, err, WriteCommitted)
	manualAt := at.Add(time.Minute)
	manual := startIdentityCycle(t, ctx, s, lock, manualAt)
	out, err = s.CommitClaudeUsageFailure(ctx, manual, 0, UsageFailure{Trigger: TriggerManual, Reason: ReasonParseFailed,
		Detail: DetailMissingLine, FailureObservedAt: manualAt, FailureAt: manualAt, BackoffUntil: manualAt})
	requireQuotaOutcome(t, out, err, WriteCommitted)
	rec, err := readQuota(s.db, ctx, func(r *SnapshotReader) (UsageRecord, error) {
		row, _, err := r.Usage(ctx, ClientClaude)
		return row, err
	})
	if err != nil || rec.Detail != DetailMissingLine || !rec.Envelope.FailureAt.Equal(at) ||
		!rec.Envelope.BackoffUntil.Equal(bgFailure.BackoffUntil) || !rec.Envelope.FailureObservedAt.Equal(manualAt) {
		t.Fatalf("manual failure record %+v: %v", rec, err)
	}
}

func TestQuotaIdentityPlanSafetySurvivesReopenAndFailure(t *testing.T) {
	ctx, s, lock, root := newIdentityStore(t)
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	record := func(wantPlan string, rejected bool) {
		t.Helper()
		reopened, err := corestore.OpenReadOnly(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer reopened.Close()
		row, err := readQuota(reopened.DB, ctx, func(r *SnapshotReader) (UsageRecord, error) {
			got, _, err := r.Usage(ctx, ClientCodex)
			return got, err
		})
		if err != nil || row.Envelope.Plan != wantPlan || row.Plan.planType.rejected != rejected ||
			(rejected && row.Envelope.PlanReason != ReasonNotReported) {
			t.Fatalf("plan=%q rejected=%v reason=%q: %v", row.Envelope.Plan, row.Plan.planType.rejected, row.Envelope.PlanReason, err)
		}
	}
	safe := func(account, plan string) {
		t.Helper()
		err := s.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: account, Plan: plan, ObservedAt: at})
		if err != nil {
			t.Fatal(err)
		}
	}
	safe("synthetic-a", "prolite")
	record("prolite", false)
	cycle := startIdentityCycle(t, ctx, s, lock, at)
	out, err := s.CommitCodexCycle(ctx, cycle, CodexCycleResult{Usage: &CodexResult{AccountID: "synthetic-a", Plan: "forbidden-token"}, planUnsafe: true}, time.Time{}, time.Time{})
	requireQuotaOutcome(t, out, err, WriteCommitted)
	record("", true)
	failed := startIdentityCycle(t, ctx, s, lock, at.Add(time.Minute))
	out, err = s.CommitCodexCycle(ctx, failed, CodexCycleResult{
		Failure: &UsageFailure{Trigger: TriggerManual, Reason: ReasonProbeFailed, FailureObservedAt: at.Add(time.Minute)},
	}, time.Time{}, time.Time{})
	requireQuotaOutcome(t, out, err, WriteCommitted)
	record("", true)
	safe("synthetic-a", "")
	record("", false)
	safe("synthetic-a", "forbidden@example.test")
	record("", true)
	obs := Observation{Client: ClientCodex, AccountID: "synthetic-b", WindowKey: "week", Source: SourceCodex, ObservedAt: at, ResetsAt: at.Add(time.Hour)}
	if _, _, _, err := s.Record(ctx, obs); err != nil {
		t.Fatal(err)
	}
	record("", false)
}

func TestQuotaIdentityBusyTimeoutRestored(t *testing.T) {
	ctx, s, _, _ := newIdentityStore(t)
	s.db.SetMaxOpenConns(1)
	var before, after int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&before); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("isolated transaction failure")
	out, err := s.quotaWrite(ctx, time.Second, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		var inside int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&inside); err != nil || inside > 1000 {
			t.Fatalf("bounded timeout = %d: %v", inside, err)
		}
		return false, failure
	})
	if !errors.Is(err, failure) || out != WriteUnresolved {
		t.Fatalf("failed write outcome=%v: %v", out, err)
	}
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&after); err != nil || before != after {
		t.Fatalf("busy timeout before=%d after=%d: %v", before, after, err)
	}
}

func TestQuotaIdentitySnapshotKeepsOneReadTransaction(t *testing.T) {
	ctx, s, lock, _ := newIdentityStore(t)
	at := time.Now().UTC()
	cycle := startIdentityCycle(t, ctx, s, lock, at)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	reader, err := NewSnapshotReader(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyClaudeIdentity(ctx, cycle, IdentityObservation{State: IdentitySignedOut}, time.Time{}, time.Time{},
		func(context.Context, Generation) error { return nil })
	requireQuotaOutcome(t, result.Outcome, err, WriteCommitted)
	if _, present, err := reader.Identity(ctx, ClientClaude); err != nil || present {
		t.Fatalf("old snapshot mixed with committed identity: %t, %v", present, err)
	}
	if _, present, err := reader.Usage(ctx, ClientClaude); err != nil || present {
		t.Fatalf("old snapshot mixed with committed generation: %t, %v", present, err)
	}
	newVersion, err := readQuota(s.db, ctx, func(r *SnapshotReader) (ReadVersion, error) {
		return r.ReadVersion(), nil
	})
	if err != nil || newVersion.CoreEpoch != reader.ReadVersion().CoreEpoch || newVersion.Revision <= reader.ReadVersion().Revision {
		t.Fatalf("read revisions: old=%+v new=%+v: %v", reader.ReadVersion(), newVersion, err)
	}
}

func TestQuotaIdentityReadOnlySchemaMatrix(t *testing.T) {
	ctx := context.Background()
	for _, version := range []int{27, 28, 29, 30, 35} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			_, s, _, root := newIdentityStore(t)
			// Roll back just the table names and relevant column shape in
			// this read-only fixture. Upgrade tests use the original migrations.
			if _, err := s.db.Exec(`ALTER TABLE quota_windows_v2 RENAME TO quota_windows;
				ALTER TABLE quota_envelopes_v2 RENAME TO quota_envelopes;
				ALTER TABLE quota_alert_notices_v2 RENAME TO quota_alert_notices`); err != nil {
				t.Fatal(err)
			}
			for _, part := range []struct {
				added int
				sql   string
			}{
				{28, "ALTER TABLE quota_envelopes DROP COLUMN backoff_until"},
				{29, "DROP TABLE quota_alert_notices"},
				{30, "ALTER TABLE quota_envelopes DROP COLUMN failure_observed_at"},
			} {
				if version < part.added {
					if _, err := s.db.Exec(part.sql); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := s.db.Exec(`UPDATE schema_metadata SET version=?`, version); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(`INSERT INTO quota_envelopes(client,plan,failure,failure_at) VALUES
				('claude','','parse_failed','2026-11-18T10:00:00Z'),('codex','prolite','','');
				INSERT INTO quota_windows(client,window_key,source,observed_at,used_percent,resets_at) VALUES
				('claude','five_hour','claude_status_line','2026-11-18T11:00:00Z',98,'2026-11-18T15:00:00Z'),
				('codex','seven_day','codex','2026-11-18T11:00:00Z',32,'2026-11-19T15:00:00Z')`); err != nil {
				t.Fatal(err)
			}
			ro, err := corestore.OpenReadOnly(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			defer ro.Close()
			q := NewStore(ro.DB)
			claude, found, err := q.Envelope(ctx, ClientClaude)
			if err != nil || !found || claude.Failure != ReasonParseFailed || !claude.FailureObservedAt.IsZero() || claude.FailureAt.IsZero() {
				t.Fatalf("legacy Claude envelope %+v, present=%v: %v", claude, found, err)
			}
			codex, found, err := q.Envelope(ctx, ClientCodex)
			if err != nil || !found || codex.Plan != "prolite" {
				t.Fatalf("legacy Codex envelope %+v, present=%v: %v", codex, found, err)
			}
			windows, err := q.Windows(ctx, ClientClaude)
			if err != nil || len(windows) != 1 {
				t.Fatalf("legacy Claude windows %d: %v", len(windows), err)
			}
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			alerts, err := DueAlerts(ctx, q, true, map[Client]bool{ClientClaude: true}, AlertConfig{Enabled: true, Thresholds: []float64{75}}, now)
			if err != nil || len(alerts) != 0 {
				t.Fatalf("unknown failure order generated a notice: %d: %v", len(alerts), err)
			}
			if version >= 29 {
				official := map[Client]bool{ClientCodex: true}
				config := AlertConfig{Enabled: true, Thresholds: []float64{25}}
				due, err := DueAlerts(ctx, q, true, official, config, now)
				if err != nil || len(due) != 1 || due[0].Client != ClientCodex || due[0].Generation != 0 {
					t.Fatalf("legacy Codex notices = %+v: %v", due, err)
				}
				key, err := parseAlertID(due[0].ID)
				if err != nil || key.Generation != 0 {
					t.Fatalf("legacy Codex notice ID = %+v: %v", key, err)
				}
				if _, err := s.db.ExecContext(ctx,
					"INSERT INTO quota_alert_notices (client, window_key, kind, threshold, instance_unix, notified_at) VALUES (?, ?, ?, ?, ?, ?)",
					key.Client, key.WindowKey, key.Kind, key.Threshold, key.Instance, now.Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				due, err = DueAlerts(ctx, q, true, official, config, now)
				if err != nil || len(due) != 0 {
					t.Fatalf("legacy Codex notice was not deduplicated: %+v: %v", due, err)
				}
			}
			if got, err := ro.SchemaVersion(ctx); got != version || err != nil {
				t.Fatalf("read migrated schema %d: %v", got, err)
			}
		})
	}
}

func TestRefreshWriteDisablesSQLiteWaitWithShorterParent(t *testing.T) {
	ctx, s, _, _ := newIdentityStore(t)
	s.db.SetMaxOpenConns(1)
	var before, after int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&before); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(150 * time.Millisecond)
	parent, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	out, err := s.refreshWrite(parent, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		d, ok := ctx.Deadline()
		var busy int
		err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy)
		if err != nil || busy != 0 || !ok || !d.Equal(deadline) {
			t.Fatalf("busy=%d deadline=%v, want %v: %v", busy, d, deadline, err)
		}
		return false, nil
	})
	if err != nil || out != WriteDropped {
		t.Fatal(out, err)
	}
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&after); err != nil || after != before {
		t.Fatal(before, after, err)
	}
}

func TestRefreshWriteReturnsBusyAtDeadline(t *testing.T) {
	ctx, s, _, _ := newIdentityStore(t)
	writer, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err = writer.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer writer.ExecContext(ctx, "ROLLBACK")
	for _, budget := range []time.Duration{time.Second, 150 * time.Millisecond} {
		start := time.Now()
		child, cancel := context.WithDeadline(ctx, start.Add(budget))
		out, err := s.refreshWrite(child, func(context.Context, *sql.Conn) (bool, error) {
			t.Fatal("writer entered while the other connection held BEGIN IMMEDIATE")
			return false, nil
		})
		elapsed := time.Since(start)
		cancel()
		var code interface{ Code() int }
		t.Logf("budget=%v return_after_cleanup=%v", budget, elapsed)
		if elapsed < budget || out != WriteUnresolved || !errors.As(err, &code) || code.Code()&255 != 5 {
			t.Errorf("busy write took %v (budget %v), outcome %v, error %v", elapsed, budget, out, err)
		}
	}
}
