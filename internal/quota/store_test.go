package quota

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentdeckstore "github.com/kitdine/agent-deck/internal/store"
)

// openTestStore opens a quota Store backed by internal/store's real,
// versioned migrations (QD-R1-F5): quota_windows/quota_envelopes are schema
// version 27 there, not a package-local CREATE TABLE.
func openTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	database, err := agentdeckstore.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	return NewStore(database.DB), database.DB
}

func TestStoreRecordAndWindows(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	accepted, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex",
		Source: SourceCodex, ObservedAt: t1, WindowMinutes: 300,
		UsedPercent: 40, ResetsAt: t1.Add(5 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !accepted {
		t.Fatal("a first observation for a key must be accepted")
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].WindowKey != "codex" || windows[0].UsedPercent != 40 {
		t.Fatalf("Windows = %+v, want one codex window at 40%%", windows)
	}
	if windows[0].Source != SourceCodex || !windows[0].ObservedAt.Equal(t1) {
		t.Fatalf("Windows must carry Source/ObservedAt through the read path: %+v", windows[0])
	}
	if windows[0].HasObservedResetAt {
		t.Fatal("no reset observed yet")
	}
}

func TestStoreRecordDetectsResetAndPersistsStickyValue(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	obs := Observation{Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 90}
	if _, _, _, err := store.Record(ctx, obs); err != nil {
		t.Fatalf("Record #1: %v", err)
	}

	t2 := t1.Add(time.Hour)
	obs.ObservedAt, obs.UsedPercent = t2, 5 // decrease: a reset
	accepted, reset, sticky, err := store.Record(ctx, obs)
	if err != nil {
		t.Fatalf("Record #2: %v", err)
	}
	if !accepted || !reset || !sticky.Equal(t2) {
		t.Fatalf("Record #2 = (%v, %v, %v), want (true, true, %v)", accepted, reset, sticky, t2)
	}

	t3 := t2.Add(time.Hour)
	obs.ObservedAt, obs.UsedPercent = t3, 20 // ordinary increase, no new reset
	accepted, reset, sticky, err = store.Record(ctx, obs)
	if err != nil {
		t.Fatalf("Record #3: %v", err)
	}
	if !accepted {
		t.Fatal("a newer observation must be accepted")
	}
	if reset {
		t.Fatal("an ordinary increase must not be reported as a fresh reset")
	}
	if !sticky.Equal(t2) {
		t.Fatalf("sticky observed_reset_at must persist across a non-reset call: got %v, want %v", sticky, t2)
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if !windows[0].HasObservedResetAt || !windows[0].ObservedResetAt.Equal(t2) {
		t.Fatalf("stored window ObservedResetAt = %+v, want %v", windows[0], t2)
	}
}

func TestStoreRecordRejectsOlderObservationArrivingLater(t *testing.T) {
	// QD-R1-F1 regression: an older reading (by observed_at) arriving after a
	// newer one must not overwrite the store or fabricate a reset.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t10 := time.Date(2026, 9, 10, 10, 10, 0, 0, time.UTC)
	t00 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeStatusLine,
		ObservedAt: t10, UsedPercent: 50,
	}); err != nil {
		t.Fatalf("Record newer: %v", err)
	}

	accepted, reset, _, err := store.Record(ctx, Observation{
		Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeStatusLine,
		ObservedAt: t00, UsedPercent: 45,
	})
	if err != nil {
		t.Fatalf("Record older: %v", err)
	}
	if accepted {
		t.Fatal("an older observation must be rejected, not applied")
	}
	if reset {
		t.Fatal("a rejected observation must never be reported as a reset")
	}

	windows, err := store.Windows(ctx, ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].UsedPercent != 50 || !windows[0].ObservedAt.Equal(t10) {
		t.Fatalf("the newer observation must remain on record, got %+v", windows[0])
	}
	if windows[0].HasObservedResetAt {
		t.Fatal("a rejected observation must not fabricate an observed reset")
	}
}

func TestStoreRecordEqualAgeSourcePrecedence(t *testing.T) {
	// QD-R1-F1: at equal observed_at, only the status-line route may replace
	// a non-status-line one; the reverse must be rejected regardless of
	// arrival order at the store.
	at := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	t.Run("status-line then equal-age prose is rejected", func(t *testing.T) {
		store, _ := openTestStore(t)
		ctx := context.Background()
		if _, _, _, err := store.Record(ctx, Observation{
			Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeStatusLine,
			ObservedAt: at, UsedPercent: 30,
		}); err != nil {
			t.Fatalf("Record status-line: %v", err)
		}
		accepted, _, _, err := store.Record(ctx, Observation{
			Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeProse,
			ObservedAt: at, UsedPercent: 30,
		})
		if err != nil {
			t.Fatalf("Record prose: %v", err)
		}
		if accepted {
			t.Fatal("an equal-age prose observation must not overwrite a stored status-line one")
		}
		windows, err := store.Windows(ctx, ClientClaude)
		if err != nil {
			t.Fatalf("Windows: %v", err)
		}
		if windows[0].Source != SourceClaudeStatusLine {
			t.Fatalf("stored source = %v, want %v", windows[0].Source, SourceClaudeStatusLine)
		}
	})

	t.Run("prose then equal-age status-line is accepted", func(t *testing.T) {
		store, _ := openTestStore(t)
		ctx := context.Background()
		if _, _, _, err := store.Record(ctx, Observation{
			Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeProse,
			ObservedAt: at, UsedPercent: 30,
		}); err != nil {
			t.Fatalf("Record prose: %v", err)
		}
		accepted, _, _, err := store.Record(ctx, Observation{
			Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: SourceClaudeStatusLine,
			ObservedAt: at, UsedPercent: 30,
		})
		if err != nil {
			t.Fatalf("Record status-line: %v", err)
		}
		if !accepted {
			t.Fatal("an equal-age status-line observation must replace a stored prose one")
		}
		windows, err := store.Windows(ctx, ClientClaude)
		if err != nil {
			t.Fatalf("Windows: %v", err)
		}
		if windows[0].Source != SourceClaudeStatusLine {
			t.Fatalf("stored source = %v, want %v", windows[0].Source, SourceClaudeStatusLine)
		}
	})
}

func TestStoreWindowsPreservesVendorOrder(t *testing.T) {
	// QD-R1-F3: windows[] must keep vendor order, not alphabetical key order.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	order := []string{"codex", "codex_secondary", "codex_bengalfox", "codex_bengalfox_secondary"}
	for i, key := range order {
		if _, _, _, err := store.Record(ctx, Observation{
			Client: ClientCodex, AccountID: "acct-1", WindowKey: key, Source: SourceCodex,
			ObservedAt: t1, VendorOrder: i, UsedPercent: 10,
		}); err != nil {
			t.Fatalf("Record %s: %v", key, err)
		}
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != len(order) {
		t.Fatalf("Windows returned %d rows, want %d", len(windows), len(order))
	}
	for i, w := range windows {
		if w.WindowKey != order[i] {
			t.Fatalf("Windows[%d] = %q, want %q (vendor order must be preserved): got order %v", i, w.WindowKey, order[i], windowKeys(windows))
		}
	}
}

func windowKeys(windows []Window) []string {
	keys := make([]string, len(windows))
	for i, w := range windows {
		keys[i] = w.WindowKey
	}
	return keys
}

func TestStoreAccountChangeDiscardsRatherThanMerges(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 80,
	}); err != nil {
		t.Fatalf("Record acct-A: %v", err)
	}
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex_secondary", Source: SourceCodex, ObservedAt: t1, UsedPercent: 10,
	}); err != nil {
		t.Fatalf("Record acct-A secondary: %v", err)
	}
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: "acct-A", Plan: "pro", ObservedAt: t1}); err != nil {
		t.Fatalf("PutEnvelope acct-A: %v", err)
	}
	if err := store.recordAlertNotice(ctx, ClientCodex, "codex", AlertThreshold, 75, t1.Add(time.Hour), t1); err != nil {
		t.Fatalf("recordAlertNotice acct-A: %v", err)
	}

	t2 := t1.Add(time.Hour)
	accepted, reset, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-B", WindowKey: "codex", Source: SourceCodex, ObservedAt: t2, UsedPercent: 5,
	})
	if err != nil {
		t.Fatalf("Record acct-B: %v", err)
	}
	if !accepted {
		t.Fatal("a fresh account's first observation must be accepted")
	}
	if reset {
		t.Fatal("a fresh account must not be reported as observing a reset against discarded data")
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].WindowKey != "codex" {
		t.Fatalf("account change must discard every prior key for the client, got %+v", windows)
	}
	if windows[0].HasObservedResetAt {
		t.Fatal("a fresh account must start with no observed reset")
	}

	if _, ok, err := store.Envelope(ctx, ClientCodex); err != nil {
		t.Fatalf("Envelope: %v", err)
	} else if ok {
		t.Fatal("account change must also discard the client's previously stored envelope")
	}
	var notices int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quota_alert_notices WHERE client = ?`, string(ClientCodex)).Scan(&notices); err != nil {
		t.Fatalf("count alert notices: %v", err)
	}
	if notices != 0 {
		t.Fatalf("account change left %d alert notice(s) from the previous account", notices)
	}
}

func TestStoreClaudeAccountIDIsStructurallyCleared(t *testing.T) {
	// QD-R1-F6: Record must force AccountID="" for Claude regardless of what
	// the caller passed, so two calls that differ only in AccountID collapse
	// onto the same key rather than isolating like Codex would.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientClaude, AccountID: "someone", WindowKey: ClaudeWindowFiveHour, ObservedAt: t1, UsedPercent: 10,
	}); err != nil {
		t.Fatalf("Record with account_id: %v", err)
	}
	t2 := t1.Add(time.Minute)
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientClaude, AccountID: "", WindowKey: ClaudeWindowFiveHour, ObservedAt: t2, UsedPercent: 12,
	}); err != nil {
		t.Fatalf("Record without account_id: %v", err)
	}

	windows, err := store.Windows(ctx, ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("Claude must have no account isolation: got %d windows, want 1: %+v", len(windows), windows)
	}
	if windows[0].UsedPercent != 12 {
		t.Fatalf("the second (structurally same-key) observation must have applied: got %+v", windows[0])
	}
}

func TestStoreEnvelopePutAndGet(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	rec := EnvelopeRecord{
		Client: ClientCodex, Applicable: true, Source: SourceCodex, ObservedAt: t1,
		Plan: "pro",
		ResetAllowance: ResetAllowance{
			TotalReason: ReasonNotReported, Remaining: 3, HasRemaining: true,
			Credits: []ResetCredit{{Title: "grant", Status: "available", GrantedAt: t1, ExpiresAt: t1.Add(24 * time.Hour)}},
		},
		Billing: Billing{Balance: 12.5, HasBalance: true},
	}
	if err := store.PutEnvelope(ctx, rec); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok {
		t.Fatal("Envelope must report ok=true for a recorded client")
	}
	if got.Plan != "pro" || !got.Applicable || got.ResetAllowance.Remaining != 3 || !got.ResetAllowance.HasRemaining {
		t.Fatalf("Envelope round-trip = %+v, want plan=pro applicable=true remaining=3", got)
	}
	if len(got.ResetAllowance.Credits) != 1 || got.ResetAllowance.Credits[0].Title != "grant" {
		t.Fatalf("Envelope credits round-trip = %+v", got.ResetAllowance.Credits)
	}
	if got.Billing.Balance != 12.5 || !got.Billing.HasBalance {
		t.Fatalf("Envelope billing round-trip = %+v", got.Billing)
	}

	if _, ok, err := store.Envelope(ctx, ClientClaude); err != nil {
		t.Fatalf("Envelope(claude): %v", err)
	} else if ok {
		t.Fatal("an unrecorded client's envelope must report ok=false")
	}
}

func TestStoreFailedProbeRetainsWindows(t *testing.T) {
	// A failed probe's envelope write (PutEnvelopeFailure) must not touch or
	// erase the windows already on record for the client.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 40,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	t2 := t1.Add(time.Minute)
	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, t2, time.Time{}, t2); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].UsedPercent != 40 {
		t.Fatalf("a failed probe must not alter previously recorded windows, got %+v", windows)
	}

	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || env.Failure != ReasonProbeFailed || !env.FailureAt.Equal(t2) {
		t.Fatalf("Envelope = (%+v, %v), want Failure=probe_failed FailureAt=%v", env, ok, t2)
	}
}

func TestStoreEnvelopeFailurePreservesLastKnownGoodFields(t *testing.T) {
	// QD-R2-F2 regression: recording a failure must never erase the fields a
	// prior successful PutEnvelope wrote — plan, reset allowance, applicable,
	// source, and observed_at all must survive a subsequent failure. A full
	// PutEnvelope call carrying only Failure (the old, buggy pattern) would
	// zero every other column via its unconditional overwrite; PutEnvelopeFailure
	// exists precisely to avoid that.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	success := EnvelopeRecord{
		Client: ClientCodex, AccountID: "acct-1", Applicable: true, Source: SourceCodex, ObservedAt: t1,
		Plan: "pro",
		ResetAllowance: ResetAllowance{
			TotalReason: ReasonNotReported, Remaining: 3, HasRemaining: true,
			Credits: []ResetCredit{{Title: "grant", Status: "available", GrantedAt: t1, ExpiresAt: t1.Add(24 * time.Hour)}},
		},
	}
	if err := store.PutEnvelope(ctx, success); err != nil {
		t.Fatalf("PutEnvelope success: %v", err)
	}

	t2 := t1.Add(5 * time.Minute)
	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, t2, time.Time{}, t2); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok {
		t.Fatal("Envelope must still report ok=true")
	}
	if got.Failure != ReasonProbeFailed || !got.FailureAt.Equal(t2) {
		t.Fatalf("Envelope failure fields = (%v, %v), want (probe_failed, %v)", got.Failure, got.FailureAt, t2)
	}
	if !got.Applicable || got.Plan != "pro" || !got.ObservedAt.Equal(t1) || got.Source != SourceCodex {
		t.Fatalf("a failure write must preserve last-known-good fields, got %+v", got)
	}
	if got.ResetAllowance.Remaining != 3 || !got.ResetAllowance.HasRemaining || len(got.ResetAllowance.Credits) != 1 {
		t.Fatalf("a failure write must preserve the last-known reset allowance, got %+v", got.ResetAllowance)
	}
}

func TestStoreEnvelopeSuccessClearsPriorFailure(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, t1, time.Time{}, t1); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	t2 := t1.Add(time.Minute)
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, Applicable: true, ObservedAt: t2, Plan: "pro"}); err != nil {
		t.Fatalf("PutEnvelope success: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || got.Failure != "" || !got.FailureAt.IsZero() || !got.FailureObservedAt.IsZero() {
		t.Fatalf("a subsequent success must clear the prior failure, got %+v", got)
	}
}

func TestStoreAccountChangeDiscardsWhenEnvelopeWrittenBeforeWindows(t *testing.T) {
	// QD-R2-F1 scenario 1: writing the new account's envelope before its
	// window must not have the window write's own discard delete it again.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 80,
	}); err != nil {
		t.Fatalf("Record acct-A: %v", err)
	}
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: "acct-A", Plan: "plan-A", ObservedAt: t1}); err != nil {
		t.Fatalf("PutEnvelope acct-A: %v", err)
	}

	t2 := t1.Add(time.Hour)
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: "acct-B", Plan: "plan-B", ObservedAt: t2}); err != nil {
		t.Fatalf("PutEnvelope acct-B: %v", err)
	}
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-B", WindowKey: "codex", Source: SourceCodex, ObservedAt: t2, UsedPercent: 5,
	}); err != nil {
		t.Fatalf("Record acct-B: %v", err)
	}

	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || env.Plan != "plan-B" {
		t.Fatalf("account B's envelope must survive the subsequent window write, got (%+v, %v)", env, ok)
	}
	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].UsedPercent != 5 {
		t.Fatalf("account B's window must be the only one on record, got %+v", windows)
	}
}

func TestStoreAccountChangeDiscardsWithNoNewWindow(t *testing.T) {
	// QD-R2-F1 scenario 2: a new account whose probe never produces a window
	// (C2 — a well-formed response missing rateLimits is not_reported, not a
	// failure) must still discard the previous account's windows, triggered
	// by the envelope write alone.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 80,
	}); err != nil {
		t.Fatalf("Record acct-A: %v", err)
	}

	t2 := t1.Add(time.Hour)
	if err := store.PutEnvelope(ctx, EnvelopeRecord{
		Client: ClientCodex, AccountID: "acct-B", Applicable: true, ObservedAt: t2, Plan: "plan-B",
	}); err != nil {
		t.Fatalf("PutEnvelope acct-B: %v", err)
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 0 {
		t.Fatalf("account B's window-less envelope write must discard account A's windows, got %+v", windows)
	}
	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || env.Plan != "plan-B" {
		t.Fatalf("Envelope = (%+v, %v), want account B's data", env, ok)
	}
}

func TestStoreAccountIDNeverStoredRaw(t *testing.T) {
	// QD-R3-F2 regression: only a digest of the account ID may reach SQL —
	// the raw value must never appear in either quota table, since the core
	// database is exported verbatim by internal/backup with no table-level
	// filtering (C8: account_id must never reach an exported file).
	store, db := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	const rawAccount = "acct-super-secret-12345"

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: rawAccount, WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 40,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: rawAccount, Applicable: true, ObservedAt: t1}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}

	var windowAccountID string
	if err := db.QueryRowContext(ctx, `SELECT account_id FROM quota_windows WHERE client = ?`, string(ClientCodex)).Scan(&windowAccountID); err != nil {
		t.Fatalf("query quota_windows.account_id: %v", err)
	}
	var envelopeAccountID string
	if err := db.QueryRowContext(ctx, `SELECT account_id FROM quota_envelopes WHERE client = ?`, string(ClientCodex)).Scan(&envelopeAccountID); err != nil {
		t.Fatalf("query quota_envelopes.account_id: %v", err)
	}

	for name, got := range map[string]string{"quota_windows": windowAccountID, "quota_envelopes": envelopeAccountID} {
		if got == rawAccount {
			t.Fatalf("%s.account_id stored the raw account ID verbatim", name)
		}
		if strings.Contains(got, rawAccount) {
			t.Fatalf("%s.account_id = %q contains the raw account ID", name, got)
		}
		if len(got) != 64 {
			t.Fatalf("%s.account_id = %q, want a 64-hex-char SHA-256 digest", name, got)
		}
	}
	if windowAccountID != envelopeAccountID {
		t.Fatalf("both tables must digest the same account to the same value: windows=%q envelopes=%q", windowAccountID, envelopeAccountID)
	}

	// The digest is still a valid isolation key: a different account
	// produces a different digest and correctly triggers discard.
	t2 := t1.Add(time.Hour)
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-other", WindowKey: "codex", Source: SourceCodex, ObservedAt: t2, UsedPercent: 10,
	}); err != nil {
		t.Fatalf("Record other account: %v", err)
	}
	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 1 || windows[0].UsedPercent != 10 {
		t.Fatalf("digesting must not break account isolation, got %+v", windows)
	}
}

func TestStoreFailureRowWithoutEnvelopeDoesNotDiscardSameAccountWindows(t *testing.T) {
	// QD-R3-F1 regression: a PutEnvelopeFailure call that creates the first
	// envelope row for a client (account_id defaults to "", observed_at
	// stays "") must not be read by currentAccount as "the account is now
	// empty." Otherwise the next successful observation for the same real
	// account looks like an account change and discards its own windows.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 90,
	}); err != nil {
		t.Fatalf("Record codex: %v", err)
	}
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex_secondary", Source: SourceCodex, ObservedAt: t1, UsedPercent: 20,
	}); err != nil {
		t.Fatalf("Record codex_secondary: %v", err)
	}

	t2 := t1.Add(time.Minute)
	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, t2, time.Time{}, t2); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	t3 := t2.Add(time.Minute)
	accepted, reset, sticky, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-A", WindowKey: "codex", Source: SourceCodex, ObservedAt: t3, UsedPercent: 5,
	})
	if err != nil {
		t.Fatalf("Record codex again: %v", err)
	}
	if !accepted {
		t.Fatal("the same account's next observation must be accepted")
	}
	if !reset || !sticky.Equal(t3) {
		t.Fatalf("90->5 for the same account must be recognized as a reset, got reset=%v sticky=%v", reset, sticky)
	}

	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("a failure row with no account evidence must not discard the account's other window, got %+v", windows)
	}

	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || env.Failure != ReasonProbeFailed {
		t.Fatalf("the failure record must survive until the next successful PutEnvelope, got (%+v, %v)", env, ok)
	}
}

func TestStoreCorruptStickyResetPropagatesError(t *testing.T) {
	// QD-R1-F8: an unparseable observed_reset_at must surface as an error,
	// not be silently treated as absent.
	store, db := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 40,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE quota_windows SET observed_reset_at = 'not-a-time'`); err != nil {
		t.Fatalf("corrupt observed_reset_at: %v", err)
	}

	t2 := t1.Add(time.Hour)
	_, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex, ObservedAt: t2, UsedPercent: 50,
	})
	if err == nil {
		t.Fatal("a corrupt stored observed_reset_at must surface as an error, not be silently erased")
	}
}

func TestStorePutEnvelopeFailureRoundTripsBackoffUntil(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	failureAt := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	backoffUntil := failureAt.Add(20 * time.Minute)

	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, failureAt, backoffUntil, failureAt); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || !got.BackoffUntil.Equal(backoffUntil) {
		t.Fatalf("Envelope BackoffUntil = %v (ok=%v), want %v", got.BackoffUntil, ok, backoffUntil)
	}
}

// TestStorePutEnvelopeFailureRoundTripsFailureObservedAt is WC-R2-F1's
// regression: FailureObservedAt must round-trip independently of FailureAt,
// since a manual failure writes the former but deliberately leaves the
// latter untouched (GS-R3-F1).
func TestStorePutEnvelopeFailureRoundTripsFailureObservedAt(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	priorFailureAt := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	attemptedAt := priorFailureAt.Add(20 * time.Minute)

	// A manual failure: failureAt/backoffUntil stay at their prior (here,
	// zero) value, but failureObservedAt always carries the real attempt
	// instant, matching how Scheduler.recordFailure calls this for
	// TriggerManual.
	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, time.Time{}, time.Time{}, attemptedAt); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || !got.FailureObservedAt.Equal(attemptedAt) || !got.FailureAt.IsZero() {
		t.Fatalf("Envelope = %+v (ok=%v), want FailureObservedAt=%v with FailureAt left zero", got, ok, attemptedAt)
	}
}

// rawEnvelopeInstants reads failure_at, backoff_until, and
// failure_observed_at as stored text. Reading them through Envelope() cannot
// tell "" from 0001-01-01T00:00:00Z: both parse back to a zero time.Time
// (GS-R5-F1).
func rawEnvelopeInstants(t *testing.T, db *sql.DB, client Client) (failureAt, backoffUntil, failureObservedAt string) {
	t.Helper()
	if err := db.QueryRowContext(context.Background(),
		`SELECT failure_at, backoff_until, failure_observed_at FROM quota_envelopes WHERE client = ?`, string(client),
	).Scan(&failureAt, &backoffUntil, &failureObservedAt); err != nil {
		t.Fatalf("read raw envelope instants: %v", err)
	}
	return failureAt, backoffUntil, failureObservedAt
}

func TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent(t *testing.T) {
	// GS-R4-F1: a zero failureAt must be stored as empty text, not as a
	// persisted 0001-01-01 instant. Extended for failureObservedAt (WC-R2-F1):
	// the same must hold for it.
	store, db := openTestStore(t)
	ctx := context.Background()

	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, time.Time{}, time.Time{}, time.Time{}); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	if failureAt, backoffUntil, failureObservedAt := rawEnvelopeInstants(t, db, ClientCodex); failureAt != "" || backoffUntil != "" || failureObservedAt != "" {
		t.Fatalf("stored failure_at=%q backoff_until=%q failure_observed_at=%q, want all empty", failureAt, backoffUntil, failureObservedAt)
	}
	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil || !ok || got.Failure != ReasonProbeFailed {
		t.Fatalf("Envelope = (%+v, %v, %v), want Failure=probe_failed", got, ok, err)
	}
}

func TestStorePutEnvelopeSuccessClearsBackoffUntil(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	if err := store.PutEnvelopeFailure(ctx, ClientCodex, ReasonProbeFailed, t1, t1.Add(20*time.Minute), t1); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	t2 := t1.Add(21 * time.Minute)
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, Applicable: true, ObservedAt: t2, Plan: "pro"}); err != nil {
		t.Fatalf("PutEnvelope success: %v", err)
	}

	got, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !ok || !got.BackoffUntil.IsZero() {
		t.Fatalf("a success must clear BackoffUntil alongside Failure/FailureAt, got %+v (ok=%v)", got, ok)
	}
}
