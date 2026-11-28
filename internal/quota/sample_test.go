package quota

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

type sampleFixture struct {
	ctx     context.Context
	core    *store.Store
	sampler Sampler
	now     time.Time
	gate    SampleGate
	pending bool
	calls   int
	account string
	used    float64
	probing func()
	failure error
}

func newSampleFixture(t *testing.T) *sampleFixture {
	t.Helper()
	f := &sampleFixture{
		ctx: context.Background(), now: time.Date(2026, 11, 19, 12, 0, 0, 123, time.UTC),
		gate: SampleGate{ProbeEnabled: true, RecordedOfficial: true}, account: "synthetic-account-A", used: 40,
	}
	root := filepath.Join(t.TempDir(), "state")
	core, err := store.Open(f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	f.core = core
	t.Cleanup(func() { _ = core.Close() })
	var installed bool
	if err := core.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='quota_session_samples')`).Scan(&installed); err != nil {
		t.Fatal(err)
	}
	if !installed {
		schema, err := os.ReadFile("../store/quota_session_samples.sql")
		if err != nil {
			t.Fatal(err)
		}
		f.exec(t, string(schema))
	}
	for _, client := range []Client{ClientCodex, ClientClaude} {
		for _, id := range []string{"selected", "other"} {
			f.exec(t, `INSERT INTO usage_sessions(client, session_id, first_at, last_at) VALUES(?,?,?,?)`,
				client, id, f.now.Add(-time.Minute).Format(timeLayout), f.now.Add(-time.Minute).Format(timeLayout))
		}
	}
	f.sampler = Sampler{
		StateRoot: root,
		Scheduler: Scheduler{
			Store: NewStore(core.DB), Now: func() time.Time { return f.now },
			ProbeCodex: func(_ context.Context, at time.Time, _ time.Duration) (CodexResult, error) {
				f.calls++
				if f.probing != nil {
					f.probing()
				}
				f.now = f.now.Add(time.Millisecond)
				return CodexResult{AccountID: f.account, Plan: "private-plan-sentinel",
					Windows: []Observation{
						{Client: ClientCodex, Source: SourceCodex, AccountID: "unused-other-account",
							WindowKey: "x_secondary", Pool: "x_secondary", Slot: "primary", Label: "private-label-sentinel",
							UsedPercent: f.used, WindowMinutes: 300, ResetsAt: at.Truncate(time.Second).Add(time.Hour)},
						{Client: ClientCodex, Source: SourceCodex, WindowKey: "review_secondary", Pool: "review", Slot: "secondary", VendorOrder: 1,
							UsedPercent: f.used + 1, WindowMinutes: 300, ResetsAt: at.Truncate(time.Second).Add(time.Hour)},
					}}, f.failure
			},
			ProbeClaudeProse: func(_ context.Context, at time.Time, _ time.Duration) (ClaudeProseResult, error) {
				f.calls++
				return ClaudeProseResult{Windows: []Observation{{
					Client: ClientClaude, Source: SourceClaudeProse, WindowKey: ClaudeWindowFiveHour, Pool: "claude",
					Slot: ClaudeWindowFiveHour, UsedPercent: 18, WindowMinutes: 300, ResetsAt: at.Truncate(time.Minute).Add(time.Hour),
				}}}, f.failure
			},
		},
		Gate: func(context.Context, *sql.Tx, Client, time.Time) (SampleGate, error) { return f.gate, nil },
		UsagePending: func(context.Context, *sql.Tx, Client, string, time.Time) (bool, error) {
			return f.pending, nil
		},
	}
	return f
}

func (f *sampleFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.core.DB.ExecContext(f.ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *sampleFixture) run(t *testing.T, action SampleAction, client Client, session string, code SampleErrorCode) SampleResult {
	t.Helper()
	result, err := f.sampler.Run(f.ctx, action, client, session)
	if code == "" && err != nil {
		t.Fatalf("%s %s: %v", action, session, err)
	}
	if code != "" {
		var failure *SampleError
		if !errors.As(err, &failure) || failure.Code != code {
			t.Fatalf("%s %s error = %v, want %s", action, session, err, code)
		}
	}
	return result
}

func (f *sampleFixture) row(t *testing.T, client Client, slot sampleSlot) (generation, before, after string) {
	t.Helper()
	err := f.core.DB.QueryRowContext(f.ctx, `SELECT generation, start_json, COALESCE(finish_json,'')
		FROM quota_session_samples WHERE client=? AND slot=?`, client, slot).Scan(&generation, &before, &after)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	return generation, before, after
}

func TestSampleTransactionsAndReplay(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleFinish, ClientCodex, "selected", SampleMissing)
	f.run(t, SampleStart, ClientCodex, "missing", SampleSessionNotFound)
	start := f.run(t, SampleStart, ClientCodex, "selected", "")
	gen, before, empty := f.row(t, ClientCodex, sampleActive)
	if start.Status != MeasurementActive || start.StartedAt == nil || len(gen) != 32 || empty != "" {
		t.Fatalf("start = %+v, generation length %d, finish %q", start, len(gen), empty)
	}
	var endpoint sampleEndpoint
	if err := json.Unmarshal([]byte(before), &endpoint); err != nil {
		t.Fatal(err)
	}
	if endpoint.Version != 1 || endpoint.Source != SourceCodex || endpoint.AccountDigest != accountDigest(ClientCodex, f.account) ||
		!endpoint.ProbeStartedAt.Equal(*start.StartedAt) || !endpoint.ProbeCompletedAt.Equal(f.now) || len(endpoint.Windows) != 2 ||
		endpoint.Windows[0].Pool != "x_secondary" || endpoint.Windows[0].Slot != "primary" ||
		endpoint.Windows[1].Pool != "review" || endpoint.Windows[1].Slot != "secondary" {
		t.Fatalf("stored endpoint = %+v", endpoint)
	}
	for _, private := range []string{f.account, "unused-other-account", "private-plan-sentinel", "private-label-sentinel"} {
		if strings.Contains(before, private) {
			t.Fatalf("endpoint retained %q", private)
		}
	}

	replay := f.run(t, SampleStart, ClientCodex, "selected", "")
	f.run(t, SampleStart, ClientCodex, "other", SampleConflict)
	f.run(t, SampleFinish, ClientCodex, "other", SampleConflict)
	f.run(t, SampleCancel, ClientCodex, "other", "")
	if got, data, _ := f.row(t, ClientCodex, sampleActive); f.calls != 1 || got != gen || data != before || !reflect.DeepEqual(start, replay) {
		t.Fatal("start replay or a different session altered the first endpoint")
	}
	f.used = 36
	finish := f.run(t, SampleFinish, ClientCodex, "selected", "")
	completeGen, completeBefore, after := f.row(t, ClientCodex, sampleLatest)
	if err := json.Unmarshal([]byte(after), &endpoint); err != nil {
		t.Fatal(err)
	}
	if finish.Status != MeasurementComplete || finish.FinishedAt == nil || !finish.FinishedAt.Equal(f.now) ||
		completeGen != gen || completeBefore != before || endpoint.Windows[0].UsedPercent != 36 ||
		endpoint.Windows[0].ObservedResetAt == nil || !endpoint.Windows[0].ObservedResetAt.Equal(endpoint.ProbeStartedAt) {
		t.Fatalf("finish = %+v, endpoint = %+v", finish, endpoint)
	}
	if active, _, _ := f.row(t, ClientCodex, sampleActive); active != "" {
		t.Fatal("finish left active in place")
	}
	if replay := f.run(t, SampleFinish, ClientCodex, "selected", ""); f.calls != 2 || !reflect.DeepEqual(finish, replay) {
		t.Fatalf("finish replay = %+v, calls = %d", replay, f.calls)
	}
	f.run(t, SampleStart, ClientCodex, "other", "")
	newGen, newBefore, _ := f.row(t, ClientCodex, sampleActive)
	if newGen == "" || newGen == gen {
		t.Fatalf("new generation = %q", newGen)
	}
	if replay := f.run(t, SampleFinish, ClientCodex, "selected", ""); f.calls != 3 || !reflect.DeepEqual(finish, replay) {
		t.Fatalf("finish replay behind another session = %+v, calls = %d", replay, f.calls)
	}
	if got, data, _ := f.row(t, ClientCodex, sampleActive); got != newGen || data != newBefore {
		t.Fatal("finish replay changed the other session's active sample")
	}
	f.run(t, SampleCancel, ClientCodex, "other", "")
	f.run(t, SampleCancel, ClientCodex, "other", "")
	if active, _, _ := f.row(t, ClientCodex, sampleActive); f.calls != 3 || active != "" {
		t.Fatal("cancel did not clear the matching active without a probe")
	}
	if latest, _, _ := f.row(t, ClientCodex, sampleLatest); latest != gen {
		t.Fatal("cancel cleared latest")
	}
	f.run(t, SampleStart, ClientCodex, "selected", "")
	freshGen, _, _ := f.row(t, ClientCodex, sampleActive)
	freshFinish := f.run(t, SampleFinish, ClientCodex, "selected", "")
	if latest, _, _ := f.row(t, ClientCodex, sampleLatest); f.calls != 5 || latest != freshGen || latest == gen || reflect.DeepEqual(finish, freshFinish) {
		t.Fatalf("finish did not complete the new generation: %+v, calls = %d", freshFinish, f.calls)
	}
}

func TestSamplePreservesNewerQuotaCache(t *testing.T) {
	f := newSampleFixture(t)
	quotaStore := f.sampler.Scheduler.Store
	cacheAt := f.now.Add(time.Minute)
	window := Observation{
		Client: ClientCodex, AccountID: f.account, Source: SourceCodex, WindowKey: "x_secondary",
		ObservedAt: cacheAt.Add(-time.Second), ResetsAt: cacheAt.Add(time.Hour), UsedPercent: 60, WindowMinutes: 300,
	}
	if accepted, _, _, err := quotaStore.Record(f.ctx, window); err != nil || !accepted {
		t.Fatalf("baseline window accepted=%t: %v", accepted, err)
	}
	window.ObservedAt, window.UsedPercent = cacheAt, 32
	if accepted, reset, resetAt, err := quotaStore.Record(f.ctx, window); err != nil || !accepted || !reset || !resetAt.Equal(cacheAt) {
		t.Fatalf("cached reset accepted=%t reset=%t timestamp=%s: %v", accepted, reset, resetAt, err)
	}
	window.WindowKey = "only-in-cache"
	if accepted, _, _, err := quotaStore.Record(f.ctx, window); err != nil || !accepted {
		t.Fatalf("extra window accepted=%t: %v", accepted, err)
	}
	if err := quotaStore.PutEnvelope(f.ctx, EnvelopeRecord{
		Client: ClientCodex, AccountID: f.account, Source: SourceCodex, ObservedAt: cacheAt, Plan: "newer-envelope",
	}); err != nil {
		t.Fatal(err)
	}
	beforeWindows, err := quotaStore.Windows(f.ctx, ClientCodex)
	if err != nil || len(beforeWindows) != 2 {
		t.Fatalf("cached windows = %+v: %v", beforeWindows, err)
	}
	beforeEnvelope, exists, err := quotaStore.Envelope(f.ctx, ClientCodex)
	if err != nil || !exists {
		t.Fatalf("cached envelope exists=%t: %v", exists, err)
	}

	f.run(t, SampleStart, ClientCodex, "selected", "")
	afterWindows, err := quotaStore.Windows(f.ctx, ClientCodex)
	if err != nil || !reflect.DeepEqual(beforeWindows, afterWindows) {
		t.Fatalf("sampling changed cached windows: %+v: %v", afterWindows, err)
	}
	afterEnvelope, exists, err := quotaStore.Envelope(f.ctx, ClientCodex)
	if err != nil || !exists || !reflect.DeepEqual(beforeEnvelope, afterEnvelope) {
		t.Fatalf("sampling changed cached envelope: %+v: %v", afterEnvelope, err)
	}
	_, start, _ := f.row(t, ClientCodex, sampleActive)
	var endpoint sampleEndpoint
	if err := json.Unmarshal([]byte(start), &endpoint); err != nil {
		t.Fatal(err)
	}
	if len(endpoint.Windows) != 2 || endpoint.Windows[0].UsedPercent != f.used ||
		endpoint.Windows[0].ObservedResetAt == nil || !endpoint.Windows[0].ObservedResetAt.Equal(cacheAt) {
		t.Fatalf("sample did not preserve explicit readings and the cached reset: %+v", endpoint)
	}
}

func TestSampleGateFailureAndUnsettledRetainStart(t *testing.T) {
	f := newSampleFixture(t)
	f.gate.ProbeEnabled = false
	f.run(t, SampleStart, ClientCodex, "selected", SampleProbeDisabled)
	f.gate.ProbeEnabled, f.gate.RecordedOfficial = true, false
	f.run(t, SampleStart, ClientCodex, "selected", SampleNotOfficial)
	f.gate.RecordedOfficial, f.gate.ObservedKnown = true, true
	f.run(t, SampleStart, ClientCodex, "selected", SampleNotOfficial)
	if f.calls != 0 {
		t.Fatalf("denied gate ran %d probes", f.calls)
	}
	f.gate.ObservedOfficial = true
	f.run(t, SampleStart, ClientCodex, "selected", "")
	gen, before, _ := f.row(t, ClientCodex, sampleActive)
	f.failure = &CodexFailureError{Kind: CodexFailureServerError, Err: errors.New("private response sentinel")}
	f.used = 82
	f.run(t, SampleFinish, ClientCodex, "selected", SampleProbeFailed)
	if latest, _, _ := f.row(t, ClientCodex, sampleLatest); latest != "" {
		t.Fatal("failed probe published a cached or partial endpoint")
	}
	f.failure = nil
	f.pending = true
	f.run(t, SampleFinish, ClientCodex, "selected", SampleUnsettled)
	if f.calls != 2 {
		t.Fatal("known pending usage ran a probe")
	}
	f.pending = false
	f.probing = func() {
		f.exec(t, `UPDATE usage_sessions SET last_at=? WHERE client='codex' AND session_id='selected'`, f.now.Format(timeLayout))
	}
	f.run(t, SampleFinish, ClientCodex, "selected", SampleUnsettled)
	if got, data, _ := f.row(t, ClientCodex, sampleActive); got != gen || data != before {
		t.Fatal("failure or unsettled changed the active generation")
	}
	windows, err := f.sampler.Scheduler.Store.Windows(f.ctx, ClientCodex)
	if err != nil || len(windows) != 2 || windows[0].UsedPercent != 40 {
		t.Fatalf("failed or unsettled attempt changed ordinary windows: %+v, %v", windows, err)
	}
	f.probing = nil
	f.run(t, SampleFinish, ClientCodex, "selected", "")
}

func TestSampleRejectsStaleGenerationAndMissingSelection(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleStart, ClientCodex, "selected", "")
	f.probing = func() {
		f.exec(t, `UPDATE quota_session_samples SET generation='replacement' WHERE client='codex' AND slot='active'`)
	}
	f.run(t, SampleFinish, ClientCodex, "selected", SampleConflict)
	if got, _, _ := f.row(t, ClientCodex, sampleActive); got != "replacement" {
		t.Fatalf("stale finish overwrote generation %q", got)
	}
	f.probing = func() {
		f.exec(t, `DELETE FROM usage_sessions WHERE client='codex' AND session_id='selected'`)
	}
	f.run(t, SampleFinish, ClientCodex, "selected", SampleSessionNotFound)
	if latest, _, _ := f.row(t, ClientCodex, sampleLatest); latest != "" {
		t.Fatal("invalid selection published an endpoint")
	}
}

func TestSampleRechecksGateBeforePublishing(t *testing.T) {
	f := newSampleFixture(t)
	f.probing = func() { f.gate.ProbeEnabled = false }
	f.run(t, SampleStart, ClientCodex, "selected", SampleProbeDisabled)
	if active, _, _ := f.row(t, ClientCodex, sampleActive); active != "" {
		t.Fatal("a gate disabled during the probe allowed publication")
	}
	if count, err := f.sampler.Scheduler.Store.Windows(f.ctx, ClientCodex); err != nil || len(count) != 0 {
		t.Fatalf("denied probe altered ordinary quota windows: %v, %v", count, err)
	}
}

func TestSampleFinishRollbackKeepsBothSlots(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleStart, ClientCodex, "other", "")
	f.run(t, SampleFinish, ClientCodex, "other", "")
	latest, _, _ := f.row(t, ClientCodex, sampleLatest)
	f.run(t, SampleStart, ClientCodex, "selected", "")
	active, _, _ := f.row(t, ClientCodex, sampleActive)
	f.exec(t, `CREATE TRIGGER fail_sample_publish BEFORE UPDATE ON quota_session_samples
		WHEN NEW.slot='latest' BEGIN SELECT RAISE(ABORT,'synthetic write failure'); END`)
	if _, err := f.sampler.Run(f.ctx, SampleFinish, ClientCodex, "selected"); err == nil {
		t.Fatal("finish ignored a persistence failure")
	}
	if got, _, _ := f.row(t, ClientCodex, sampleActive); got != active {
		t.Fatal("failed transaction changed active")
	}
	if got, _, _ := f.row(t, ClientCodex, sampleLatest); got != latest {
		t.Fatal("failed transaction deleted latest")
	}
	f.exec(t, `DROP TRIGGER fail_sample_publish`)
	f.run(t, SampleFinish, ClientCodex, "selected", "")
}

func TestSampleKnownAccountAndRouteInvalidation(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleStart, ClientClaude, "selected", "")
	f.run(t, SampleStart, ClientCodex, "other", "")
	f.run(t, SampleFinish, ClientCodex, "other", "")
	f.run(t, SampleStart, ClientCodex, "selected", "")
	f.account = "synthetic-account-B"
	f.run(t, SampleFinish, ClientCodex, "selected", SampleAccountChanged)
	for _, slot := range []sampleSlot{sampleActive, sampleLatest} {
		if got, _, _ := f.row(t, ClientCodex, slot); got != "" {
			t.Fatalf("account change left %s", slot)
		}
	}
	if got, _, _ := f.row(t, ClientClaude, sampleActive); got == "" {
		t.Fatal("Codex account change cleared Claude")
	}
	f.run(t, SampleStart, ClientCodex, "selected", "")
	for _, account := range []string{"", "synthetic-account-C"} {
		rec := EnvelopeRecord{Client: ClientCodex, AccountID: account, Source: SourceCodex, ObservedAt: f.now}
		if err := f.sampler.Scheduler.Store.PutEnvelope(f.ctx, rec); err != nil {
			t.Fatal(err)
		}
		if got, _, _ := f.row(t, ClientCodex, sampleActive); (got == "") != (account != "") {
			t.Fatalf("unexpected invalidation for account %q", account)
		}
	}
	f.gate.RouteChanged = true
	f.run(t, SampleFinish, ClientClaude, "selected", SampleNotOfficial)
	if got, _, _ := f.row(t, ClientClaude, sampleActive); got != "" {
		t.Fatal("confirmed route change kept the Claude sample")
	}
}

func TestSampleRecordAccountChangeInvalidatesOnlyThatClient(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleStart, ClientClaude, "selected", "")
	claudeGeneration, claudeStart, _ := f.row(t, ClientClaude, sampleActive)
	f.run(t, SampleStart, ClientCodex, "other", "")
	f.run(t, SampleFinish, ClientCodex, "other", "")
	f.run(t, SampleStart, ClientCodex, "selected", "")
	for _, slot := range []sampleSlot{sampleActive, sampleLatest} {
		if generation, _, _ := f.row(t, ClientCodex, slot); generation == "" {
			t.Fatalf("fixture lacks Codex %s sample", slot)
		}
	}

	if accepted, _, _, err := f.sampler.Scheduler.Store.Record(f.ctx, Observation{
		Client: ClientCodex, AccountID: "synthetic-account-B", Source: SourceCodex, WindowKey: "x_secondary",
		ObservedAt: f.now, ResetsAt: f.now.Add(time.Hour), UsedPercent: 21, WindowMinutes: 300,
	}); err != nil || !accepted {
		t.Fatalf("new account window accepted=%t: %v", accepted, err)
	}
	for _, slot := range []sampleSlot{sampleActive, sampleLatest} {
		if generation, _, _ := f.row(t, ClientCodex, slot); generation != "" {
			t.Fatalf("recording a new account kept Codex %s sample", slot)
		}
	}
	if generation, start, _ := f.row(t, ClientClaude, sampleActive); generation != claudeGeneration || start != claudeStart {
		t.Fatal("recording a Codex account change altered the Claude sample")
	}
}

func TestSampleKnownAuthenticationFailureInvalidates(t *testing.T) {
	f := newSampleFixture(t)
	f.run(t, SampleStart, ClientClaude, "selected", "")
	f.failure = &ClaudeFailureError{Kind: ClaudeFailureExit, Err: errors.New("synthetic opaque failure")}
	f.run(t, SampleFinish, ClientClaude, "selected", SampleProbeFailed)
	if got, _, _ := f.row(t, ClientClaude, sampleActive); got == "" {
		t.Fatal("unclassified Claude probe failure removed the active sample")
	}
	f.gate.AuthenticationFailed = true
	f.run(t, SampleFinish, ClientClaude, "selected", SampleProbeFailed)
	if got, _, _ := f.row(t, ClientClaude, sampleActive); f.calls != 2 || got != "" {
		t.Fatal("confirmed authentication failure did not invalidate before probing")
	}
}

func TestSampleExplicitAndRefreshAuthenticationEvidenceInvalidatesClient(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		name := "refresh"
		if explicit {
			name = "explicit"
		}
		t.Run(name, func(t *testing.T) {
			f := newSampleFixture(t)
			f.run(t, SampleStart, ClientCodex, "selected", "")
			f.run(t, SampleStart, ClientClaude, "other", "")
			f.run(t, SampleFinish, ClientClaude, "other", "")
			f.run(t, SampleStart, ClientClaude, "selected", "")
			f.failure = &ClaudeFailureError{Kind: ClaudeFailureExit, Err: errProbeAuthentication}
			if explicit {
				f.run(t, SampleFinish, ClientClaude, "selected", SampleProbeFailed)
			} else {
				f.sampler.Scheduler.Run(f.ctx, ClientClaude, TriggerManual, true)
				rec, _, err := f.sampler.Scheduler.Store.Envelope(f.ctx, ClientClaude)
				if err != nil || rec.Failure != ReasonProbeFailed {
					t.Fatalf("refresh failure = %v: %v", rec.Failure, err)
				}
			}
			for _, slot := range []sampleSlot{sampleActive, sampleLatest} {
				if generation, _, _ := f.row(t, ClientClaude, slot); generation != "" {
					t.Fatalf("confirmed authentication failure kept %s", slot)
				}
			}
			if generation, _, _ := f.row(t, ClientCodex, sampleActive); generation == "" {
				t.Fatal("Claude authentication failure affected Codex")
			}
		})
	}
}

func TestSampleAndRefreshUseSameLock(t *testing.T) {
	f := newSampleFixture(t)
	lock, err := store.AcquireQuotaRefreshLock(f.ctx, f.sampler.StateRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := f.sampler.Run(f.ctx, SampleStart, ClientCodex, "selected"); !errors.Is(err, store.ErrStateBusy) || f.calls != 0 {
		t.Fatalf("held refresh lock allowed sampling: calls=%d, error=%v", f.calls, err)
	}
}
