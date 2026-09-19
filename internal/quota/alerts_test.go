package quota

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// recordingNotifier stands in for the app: it "posts" each due notice and
// acknowledges only the ones it posted, as C10's delivery contract requires.
type recordingNotifier struct {
	sent []DueAlert
	err  error
}

var bothClientsOfficial = map[Client]bool{ClientCodex: true, ClientClaude: true}

func evaluate(ctx context.Context, store *Store, probeEnabled bool, cfg AlertConfig, notifier *recordingNotifier, now time.Time) error {
	due, err := DueAlerts(ctx, store, probeEnabled, bothClientsOfficial, cfg, now)
	errs := []error{err}
	for _, alert := range due {
		if notifier.err != nil {
			errs = append(errs, notifier.err)
			continue
		}
		notifier.sent = append(notifier.sent, alert)
		errs = append(errs, AcknowledgeAlert(ctx, store, alert.ID, now))
	}
	return errors.Join(errs...)
}

func recordWindow(t *testing.T, store *Store, obs Observation) {
	t.Helper()
	if _, _, _, err := store.Record(context.Background(), obs); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func claudeFiveHour(observedAt, resetsAt time.Time, used float64, source Source) Observation {
	return Observation{
		Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Source: source,
		ObservedAt: observedAt, WindowMinutes: 300, UsedPercent: used, ResetsAt: resetsAt,
	}
}

var bothThresholds = AlertConfig{Enabled: true, Thresholds: []float64{75, 90}}

func TestEvaluateAlertsDoesNotRunWhenAlertsOrReadingAreOff(t *testing.T) {
	// A nil store proves nothing is read: evaluating would dereference it.
	notifier := &recordingNotifier{}
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	for name, run := range map[string]func() error{
		"alerts off":             func() error { return evaluate(context.Background(), nil, true, AlertConfig{}, notifier, now) },
		"reading off":            func() error { return evaluate(context.Background(), nil, false, bothThresholds, notifier, now) },
		"both off (the default)": func() error { return evaluate(context.Background(), nil, false, AlertConfig{}, notifier, now) },
	} {
		if err := run(); err != nil {
			t.Fatalf("%s: EvaluateAlerts = %v, want nil", name, err)
		}
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("notifications = %+v, want none", notifier.sent)
	}
}

func TestEvaluateAlertsThresholdFiresOncePerOccurrenceAndAgainAfterReset(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	r1 := t0.Add(5 * time.Hour)

	step := func(offset time.Duration, resetsAt time.Time, used float64) {
		t.Helper()
		recordWindow(t, store, claudeFiveHour(t0.Add(offset), resetsAt, used, SourceClaudeStatusLine))
		if err := evaluate(ctx, store, true, bothThresholds, notifier, t0.Add(offset)); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
	}

	step(0, r1, 60)
	if len(notifier.sent) != 0 {
		t.Fatalf("below both thresholds: notifications = %d, want 0", len(notifier.sent))
	}
	step(10*time.Minute, r1, 80)
	step(20*time.Minute, r1, 82) // repeated probe above 75: nothing new
	if len(notifier.sent) != 1 || notifier.sent[0].Threshold != 75 {
		t.Fatalf("after crossing 75 then staying above: notifications = %+v, want one at 75", notifier.sent)
	}
	step(30*time.Minute, r1, 92)
	step(40*time.Minute, r1.Add(40*time.Second), 95) // same occurrence, jittered resets_at
	if len(notifier.sent) != 2 || notifier.sent[1].Threshold != 90 {
		t.Fatalf("after crossing 90: notifications = %+v, want a second at 90 and nothing more", notifier.sent)
	}

	r2 := r1.Add(5 * time.Hour)
	step(5*time.Hour+time.Minute, r2, 5) // the window reset
	if len(notifier.sent) != 2 {
		t.Fatalf("after a reset below thresholds: notifications = %d, want still 2", len(notifier.sent))
	}
	step(5*time.Hour+time.Hour, r2, 78)
	if len(notifier.sent) != 3 || notifier.sent[2].Threshold != 75 {
		t.Fatalf("crossing 75 in the next occurrence: notifications = %+v, want a third at 75", notifier.sent)
	}
}

func TestEvaluateAlertsInstanceTolerance(t *testing.T) {
	// The prose route reports the same occurrence to the minute; that must not
	// notify again. A resets_at beyond the tolerance is a different occurrence.
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	cfg := AlertConfig{Enabled: true, Thresholds: []float64{75}}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	resetsAt := time.Date(2026, 9, 10, 13, 49, 58, 0, time.UTC)

	recordWindow(t, store, claudeFiveHour(t0, resetsAt, 80, SourceClaudeStatusLine))
	if err := evaluate(ctx, store, true, cfg, notifier, t0); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	recordWindow(t, store, claudeFiveHour(t0.Add(time.Minute), resetsAt.Truncate(time.Minute).Add(time.Minute), 81, SourceClaudeProse))
	if err := evaluate(ctx, store, true, cfg, notifier, t0.Add(time.Minute)); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("same occurrence reported by another route: notifications = %d, want 1", len(notifier.sent))
	}

	recordWindow(t, store, claudeFiveHour(t0.Add(2*time.Minute), resetsAt.Add(alertInstanceTolerance+time.Minute), 82, SourceClaudeProse))
	if err := evaluate(ctx, store, true, cfg, notifier, t0.Add(2*time.Minute)); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("resets_at beyond the tolerance: notifications = %d, want 2", len(notifier.sent))
	}
}

func TestEvaluateAlertsResetNoticeOncePerOccurrence(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	cfg := AlertConfig{Enabled: true, ResetNotice: true}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	r1 := t0.Add(5 * time.Hour)
	r2 := r1.Add(5 * time.Hour)

	step := func(at, resetsAt time.Time, used float64) {
		t.Helper()
		recordWindow(t, store, claudeFiveHour(at, resetsAt, used, SourceClaudeStatusLine))
		if err := evaluate(ctx, store, true, cfg, notifier, at); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
	}

	step(t0, r1, 50)
	if len(notifier.sent) != 0 {
		t.Fatalf("no decrease yet: notifications = %d, want 0", len(notifier.sent))
	}
	step(r1.Add(time.Minute), r2, 3) // observed decrease: a reset
	if len(notifier.sent) != 1 || notifier.sent[0].Kind != AlertReset {
		t.Fatalf("after an observed reset: notifications = %+v, want one reset notice", notifier.sent)
	}
	step(r1.Add(10*time.Minute), r2, 12)
	// QA-R1-F2: a small same-source drop inside the same occurrence moves
	// observed_reset_at forward, but it is still the same occurrence.
	step(r1.Add(40*time.Minute), r2, 11.5)
	if len(notifier.sent) != 1 {
		t.Fatalf("a drop inside the same occurrence: notifications = %+v, want still 1", notifier.sent)
	}

	r3 := r2.Add(5 * time.Hour)
	step(r2.Add(-time.Hour), r2, 60)
	step(r2.Add(time.Minute), r3, 2) // the next occurrence's reset
	if len(notifier.sent) != 2 {
		t.Fatalf("a reset in the next occurrence: notifications = %d, want 2", len(notifier.sent))
	}
}

func TestEvaluateAlertsIgnoresAnOccurrenceThatHasEnded(t *testing.T) {
	// QA-R1-F1: when probes stop succeeding, the last good window stays stored
	// with a resets_at that has long passed. It must not notify — not after
	// the ledger's retention, and not when alerts are first switched on.
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	cfg := AlertConfig{Enabled: true, Thresholds: []float64{75, 90}, ResetNotice: true}

	t.Run("no repeat once the ledger is pruned", func(t *testing.T) {
		store, _ := openTestStore(t)
		notifier := &recordingNotifier{}
		recordWindow(t, store, claudeFiveHour(t0, t0.Add(3*time.Hour), 80, SourceClaudeStatusLine))
		if err := evaluate(ctx, store, true, cfg, notifier, t0); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
		if len(notifier.sent) != 1 {
			t.Fatalf("fresh evaluation: notifications = %d, want 1", len(notifier.sent))
		}
		for _, at := range []time.Time{
			t0.Add(4 * time.Hour),
			t0.Add(alertNoticeRetention - time.Hour),
			t0.Add(alertNoticeRetention + 10*24*time.Hour),
			t0.Add(alertNoticeRetention + 10*24*time.Hour + 5*time.Minute),
			t0.Add(alertNoticeRetention + 10*24*time.Hour + 10*time.Minute),
		} {
			if err := evaluate(ctx, store, true, cfg, notifier, at); err != nil {
				t.Fatalf("EvaluateAlerts at %v: %v", at, err)
			}
		}
		if len(notifier.sent) != 1 {
			t.Fatalf("stale window evaluated repeatedly: notifications = %+v, want still 1", notifier.sent)
		}
	})

	t.Run("alerts switched on after the occurrence ended", func(t *testing.T) {
		store, _ := openTestStore(t)
		notifier := &recordingNotifier{}
		recordWindow(t, store, claudeFiveHour(t0, t0.Add(time.Hour), 50, SourceClaudeStatusLine))
		recordWindow(t, store, claudeFiveHour(t0.Add(time.Minute), t0.Add(time.Hour), 95, SourceClaudeStatusLine))
		recordWindow(t, store, claudeFiveHour(t0.Add(2*time.Minute), t0.Add(time.Hour), 94, SourceClaudeStatusLine)) // observed decrease too
		if err := evaluate(ctx, store, true, cfg, notifier, t0.Add(48*time.Hour)); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
		if len(notifier.sent) != 0 {
			t.Fatalf("occurrence ended 47h ago: notifications = %+v, want none", notifier.sent)
		}
	})
}

func TestEvaluateAlertsNoResetNoticeWithoutResetsAt(t *testing.T) {
	// A reset notice is deduplicated per occurrence, identified by resets_at;
	// without one there is no occurrence to notify once for.
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	recordWindow(t, store, claudeFiveHour(t0, time.Time{}, 50, SourceClaudeStatusLine))
	recordWindow(t, store, claudeFiveHour(t0.Add(time.Minute), time.Time{}, 3, SourceClaudeStatusLine))

	if err := evaluate(ctx, store, true, AlertConfig{Enabled: true, ResetNotice: true}, notifier, t0.Add(time.Minute)); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("notifications = %+v, want none without resets_at", notifier.sent)
	}
}

func TestEvaluateAlertsIgnoresResetFromAnEarlierOccurrence(t *testing.T) {
	// A reset observed while alerts were off must not notify once the window
	// has moved on to a later occurrence.
	store, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	r1 := t0.Add(5 * time.Hour)
	r2 := r1.Add(5 * time.Hour)
	r3 := r2.Add(5 * time.Hour)

	recordWindow(t, store, claudeFiveHour(t0, r1, 50, SourceClaudeStatusLine))
	recordWindow(t, store, claudeFiveHour(r1.Add(time.Minute), r2, 3, SourceClaudeStatusLine))  // reset, alerts off
	recordWindow(t, store, claudeFiveHour(r2.Add(time.Minute), r3, 30, SourceClaudeStatusLine)) // next occurrence, no decrease

	notifier := &recordingNotifier{}
	if err := evaluate(ctx, store, true, AlertConfig{Enabled: true, ResetNotice: true}, notifier, r2.Add(2*time.Minute)); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("stale reset from an earlier occurrence: notifications = %+v, want none", notifier.sent)
	}
}

func TestEvaluateAlertsNotificationCarriesNoAccountIdentifier(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	const account = "acct-secret-4711"

	recordWindow(t, store, Observation{
		Client: ClientCodex, AccountID: account, WindowKey: "codex", Source: SourceCodex,
		ObservedAt: t0, WindowMinutes: 300, UsedPercent: 91, ResetsAt: t0.Add(3 * time.Hour),
	})
	if err := evaluate(ctx, store, true, bothThresholds, notifier, t0); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("notifications = %+v, want one per crossed threshold", notifier.sent)
	}
	for _, n := range notifier.sent {
		encoded, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		key, err := parseAlertID(n.ID)
		if err != nil {
			t.Fatalf("parseAlertID(%q): %v", n.ID, err)
		}
		keyJSON, _ := json.Marshal(key)
		for _, text := range []string{string(encoded), string(keyJSON)} {
			if strings.Contains(text, account) || strings.Contains(text, accountDigest(ClientCodex, account)) {
				t.Fatalf("notice %s carries an account identifier", text)
			}
		}
		if n.Client != ClientCodex || n.WindowMinutes != 300 || n.UsedPercent != 91 {
			t.Fatalf("notice %+v, want client, window length, and figure named", n)
		}
	}
}

func TestEvaluateAlertsRetriesAFailedDelivery(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	cfg := AlertConfig{Enabled: true, Thresholds: []float64{75}}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	recordWindow(t, store, claudeFiveHour(t0, t0.Add(4*time.Hour), 80, SourceClaudeStatusLine))

	notifier := &recordingNotifier{err: errors.New("notification centre unavailable")}
	if err := evaluate(ctx, store, true, cfg, notifier, t0); err == nil {
		t.Fatal("EvaluateAlerts returned nil after a failed delivery")
	}
	notifier.err = nil
	for i := 0; i < 2; i++ {
		if err := evaluate(ctx, store, true, cfg, notifier, t0.Add(time.Minute)); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("notifications = %d, want exactly 1: the failed delivery retried once, then deduplicated", len(notifier.sent))
	}
}

func TestEvaluateAlertsSkipsThresholdsWithoutResetsAt(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	recordWindow(t, store, claudeFiveHour(t0, time.Time{}, 95, SourceClaudeStatusLine))

	if err := evaluate(ctx, store, true, bothThresholds, notifier, t0); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("notifications = %+v, want none for a window with no identifiable occurrence", notifier.sent)
	}
}

func TestEvaluateAlertsPrunesExpiredNotices(t *testing.T) {
	store, db := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if err := store.recordAlertNotice(ctx, ClientClaude, ClaudeWindowFiveHour, AlertThreshold, 75, t0, t0); err != nil {
		t.Fatalf("recordAlertNotice: %v", err)
	}

	later := t0.Add(alertNoticeRetention + time.Hour)
	if err := evaluate(ctx, store, true, bothThresholds, &recordingNotifier{}, later); err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quota_alert_notices`).Scan(&rows); err != nil {
		t.Fatalf("count notices: %v", err)
	}
	if rows != 0 {
		t.Fatalf("notices after retention = %d, want 0", rows)
	}
}

func TestEvaluateAlertsNoResetNoticeWhenWindowLengthUnknown(t *testing.T) {
	// QA-R2-F1: without a window length the current occurrence cannot be
	// bounded. A sticky observed_reset_at from one real reset must not notify
	// again in a later occurrence where usage only rose; such a window gets
	// no reset notice at all.
	store, _ := openTestStore(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	cfg := AlertConfig{Enabled: true, ResetNotice: true}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	r1 := t0.Add(5 * time.Hour)
	r2 := r1.Add(5 * time.Hour)
	r3 := r2.Add(5 * time.Hour)

	step := func(at, resetsAt time.Time, used float64) {
		t.Helper()
		recordWindow(t, store, Observation{
			Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex_bengalfox", Source: SourceCodex,
			ObservedAt: at, WindowMinutesReason: ReasonNotReported, UsedPercent: used, ResetsAt: resetsAt,
		})
		if err := evaluate(ctx, store, true, cfg, notifier, at); err != nil {
			t.Fatalf("EvaluateAlerts: %v", err)
		}
	}

	step(t0, r1, 50)
	step(r1.Add(time.Minute), r2, 3)  // a real reset
	step(r2.Add(time.Minute), r3, 40) // next occurrence, no decrease
	if len(notifier.sent) != 0 {
		t.Fatalf("window of unknown length: notifications = %+v, want none", notifier.sent)
	}
}

func TestDueAlertsRecordNothingUntilAcknowledged(t *testing.T) {
	// A notice the app could not post is never acknowledged, so it must stay
	// due with the same id — the retry C10 promises — until an ack arrives.
	store, db := openTestStore(t)
	ctx := context.Background()
	cfg := AlertConfig{Enabled: true, Thresholds: []float64{75}}
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	recordWindow(t, store, claudeFiveHour(t0, t0.Add(4*time.Hour), 80, SourceClaudeStatusLine))

	first, err := DueAlerts(ctx, store, true, bothClientsOfficial, cfg, t0)
	if err != nil || len(first) != 1 {
		t.Fatalf("DueAlerts = %+v, %v; want one notice", first, err)
	}
	second, err := DueAlerts(ctx, store, true, bothClientsOfficial, cfg, t0.Add(time.Minute))
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("unacknowledged notice: second evaluation = %+v, %v; want the same id again", second, err)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quota_alert_notices`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("ledger rows before acknowledgement = %d, %v; want 0", rows, err)
	}
	for i := 0; i < 2; i++ {
		if err := AcknowledgeAlert(ctx, store, first[0].ID, t0.Add(2*time.Minute)); err != nil {
			t.Fatalf("AcknowledgeAlert: %v", err)
		}
	}
	if after, err := DueAlerts(ctx, store, true, bothClientsOfficial, cfg, t0.Add(3*time.Minute)); err != nil || len(after) != 0 {
		t.Fatalf("after acknowledgement = %+v, %v; want none", after, err)
	}
}

func TestAcknowledgeAlertRejectsIDsTheEvaluatorCannotProduce(t *testing.T) {
	store, db := openTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	valid := alertKey{Client: ClientClaude, WindowKey: ClaudeWindowFiveHour, Kind: AlertThreshold, Threshold: 75, Instance: now.Unix()}
	forge := func(key alertKey) string {
		encoded, _ := json.Marshal(key)
		return alertIDPrefix + base64.RawURLEncoding.EncodeToString(encoded)
	}
	cases := map[string]string{
		"empty":             "",
		"no prefix":         strings.TrimPrefix(valid.id(), alertIDPrefix),
		"not base64":        alertIDPrefix + "!!!",
		"not json":          alertIDPrefix + base64.RawURLEncoding.EncodeToString([]byte("nope")),
		"unknown client":    forge(alertKey{Client: "gemini", WindowKey: "w", Kind: AlertThreshold, Threshold: 75, Instance: 1}),
		"unknown kind":      forge(alertKey{Client: ClientCodex, WindowKey: "w", Kind: "spam", Threshold: 75, Instance: 1}),
		"reset with figure": forge(alertKey{Client: ClientCodex, WindowKey: "w", Kind: AlertReset, Threshold: 75, Instance: 1}),
		"threshold of 0":    forge(alertKey{Client: ClientCodex, WindowKey: "w", Kind: AlertThreshold, Threshold: 0, Instance: 1}),
		"no window":         forge(alertKey{Client: ClientCodex, Kind: AlertThreshold, Threshold: 75, Instance: 1}),
		"no instance":       forge(alertKey{Client: ClientCodex, WindowKey: "w", Kind: AlertThreshold, Threshold: 75}),
		"extra field":       alertIDPrefix + base64.RawURLEncoding.EncodeToString([]byte(`{"c":"claude","w":"five_hour","k":"threshold","t":75,"i":1,"x":1}`)),
	}
	for name, id := range cases {
		if err := AcknowledgeAlert(ctx, store, id, now); !errors.Is(err, ErrInvalidAlertID) {
			t.Fatalf("%s: AcknowledgeAlert(%q) = %v, want ErrInvalidAlertID", name, id, err)
		}
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quota_alert_notices`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("ledger rows after rejected acknowledgements = %d, %v; want 0", rows, err)
	}
	if err := AcknowledgeAlert(ctx, store, valid.id(), now); err != nil {
		t.Fatalf("AcknowledgeAlert(valid) = %v", err)
	}
}

// Codex PR #5 eleventh review, P2: desktop.BuildSubscription already hides a
// client's retained windows once its envelope shows an unsuperseded parse
// failure (they are not read as the account's current quota there either),
// but DueAlerts read the store directly without that same check -- so an
// unacknowledged threshold already crossed before the failure, or alerts
// enabled while the failure is already in effect, could still fire a
// notification carrying the stale pre-failure percentage.
func TestDueAlertsSuppressesAClientWithAnUnsupersededParseFailure(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	failedAt := t0.Add(10 * time.Minute)

	recordWindow(t, store, claudeFiveHour(t0, t0.Add(3*time.Hour), 91, SourceClaudeProse))
	if err := store.PutEnvelopeFailure(ctx, ClientClaude, ReasonParseFailed, failedAt, time.Time{}, failedAt); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	due, err := DueAlerts(ctx, store, true, bothClientsOfficial, bothThresholds, failedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("DueAlerts: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due = %+v, want none: a client with an unsuperseded parse failure must not still notify from its stale retained figure", due)
	}
}

// Companion to the suppression above: a later status-line success must still
// notify normally, proving the fix does not over-broadly suppress every
// client that has ever recorded a parse failure.
func TestDueAlertsStillFiresWhenAStatusLineSuccessSupersedesAnOlderParseFailure(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	failedAt := t0.Add(10 * time.Minute)

	if err := store.PutEnvelopeFailure(ctx, ClientClaude, ReasonParseFailed, failedAt, time.Time{}, failedAt); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}
	statusLineAt := failedAt.Add(time.Minute)
	recordWindow(t, store, claudeFiveHour(statusLineAt, statusLineAt.Add(3*time.Hour), 91, SourceClaudeStatusLine))

	due, err := DueAlerts(ctx, store, true, bothClientsOfficial, bothThresholds, statusLineAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("DueAlerts: %v", err)
	}
	if len(due) == 0 {
		t.Fatalf("due = %+v, want the status-line success (newer than the failure) to still notify", due)
	}
}

// Codex PR #5 P2: when reading is on but a client has switched away from the
// official provider, that client's retained windows are stale for alerting
// purposes -- RefreshQuota's own gate skips probing it (Reason=not_official)
// for the same reason. DueAlerts must not still fire for it.
func TestDueAlertsSkipsAClientCurrentlyOffTheOfficialProvider(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	recordWindow(t, store, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex,
		ObservedAt: t0, WindowMinutes: 300, UsedPercent: 91, ResetsAt: t0.Add(3 * time.Hour),
	})
	recordWindow(t, store, claudeFiveHour(t0, t0.Add(3*time.Hour), 91, SourceClaudeStatusLine))

	codexOnly := map[Client]bool{ClientCodex: true, ClientClaude: false}
	due, err := DueAlerts(ctx, store, true, codexOnly, bothThresholds, t0)
	if err != nil {
		t.Fatalf("DueAlerts: %v", err)
	}
	for _, alert := range due {
		if alert.Client == ClientClaude {
			t.Fatalf("due = %+v, want no alert for the non-official client", due)
		}
	}
	if len(due) == 0 {
		t.Fatalf("due = %+v, want the still-official client's crossed thresholds to still fire", due)
	}
}
