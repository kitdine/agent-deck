package quota

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// AlertKind distinguishes C10's two notifications.
type AlertKind string

const (
	AlertThreshold AlertKind = "threshold"
	AlertReset     AlertKind = "reset"
)

// AlertConfig is the alert half of the settings group (ux/settings-quota.md):
// quotaAlerts, quotaThresholds, and quotaResetNotice. The zero value is the
// product default — everything off.
type AlertConfig struct {
	Enabled     bool
	Thresholds  []float64
	ResetNotice bool
}

// Notification is what C10 delivers. It names the client, the window, and the
// figure, and has no field an account identifier could travel in.
type Notification struct {
	Kind        AlertKind
	Client      Client
	Window      string
	UsedPercent float64
	Threshold   float64
}

func (n Notification) Title() string {
	switch n.Client {
	case ClientCodex:
		return "Codex quota"
	case ClientClaude:
		return "Claude quota"
	default:
		return "Quota"
	}
}

func (n Notification) Body() string {
	if n.Kind == AlertReset {
		return fmt.Sprintf("%s has reset — now at %.0f%%", n.Window, n.UsedPercent)
	}
	return fmt.Sprintf("%s at %.0f%% (alert threshold %.0f%%)", n.Window, n.UsedPercent, n.Threshold)
}

// Notifier delivers one notification. EvaluateAlerts records a notice as sent
// only after Notify returns nil, so a failed delivery is retried by the next
// evaluation rather than silently lost.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// alertInstanceTolerance decides when two resets_at values name the same
// window occurrence. The same occurrence is reported with different precision
// by different routes — Claude's prose route resolves to the minute, the
// status-line payload and Codex to the second, and a vendor's value can drift
// by seconds between reads — so exact equality would notify twice within one
// occurrence. Fifteen minutes absorbs that jitter while staying far below the
// shortest window (five hours), so two real occurrences are never merged.
const alertInstanceTolerance = 15 * time.Minute

// alertNoticeRetention bounds the ledger to a working set. The evaluator only
// considers occurrences whose resets_at is no more than alertInstanceTolerance
// in the past, and matches instances within that same tolerance, so an entry
// older than twice the tolerance can never be needed again; 31 days is far
// beyond that bound (QA-R1-F1).
const alertNoticeRetention = 31 * 24 * time.Hour

// EvaluateAlerts is C10's evaluator. With reading off or alerts off it returns
// immediately without touching the store or the notifier — nothing is
// evaluated, not merely nothing delivered (C9, C10).
//
// A threshold notice is due when a window's used share is at or above the
// threshold and no notice has been recorded for that (client, window_key,
// threshold) within alertInstanceTolerance of the window's resets_at. The
// evaluator does not require having seen the previous figure below the
// threshold: that would miss a crossing written by a route that never passes
// through the evaluator (the status-line capture), and alerts switched on
// while a window already sits above a threshold notify once for that
// occurrence. A window without a resets_at has no identifiable occurrence and
// gets no notice of either kind.
//
// Only an ongoing occurrence notifies (QA-R1-F1). When probes stop succeeding
// the last good window stays stored (C9) with a resets_at that has passed;
// notifying from it would report a past figure, and, once its ledger entry
// aged out, report it again on every evaluation. A window whose resets_at is
// more than alertInstanceTolerance in the past is therefore skipped.
//
// A reset notice is due when a window carries an observed_reset_at — the same
// local evidence of a decrease that sets it (C6) — inside its current
// occurrence. It is deduplicated by the occurrence's resets_at, not by
// observed_reset_at (QA-R1-F2): any same-source drop moves observed_reset_at
// forward, and a small drop later in the same occurrence is not a second
// reset.
//
// Errors from individual windows are collected and returned together; one
// failing window does not stop the rest from being evaluated.
func EvaluateAlerts(ctx context.Context, store *Store, probeEnabled bool, cfg AlertConfig, notifier Notifier, now time.Time) error {
	if !probeEnabled || !cfg.Enabled || notifier == nil {
		return nil
	}
	if err := store.pruneAlertNotices(ctx, now.Add(-alertNoticeRetention)); err != nil {
		return err
	}
	thresholds := normalizeThresholds(cfg.Thresholds)
	var errs []error
	for _, client := range []Client{ClientCodex, ClientClaude} {
		windows, err := store.Windows(ctx, client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, w := range windows {
			if !occurrenceOngoing(w, now) {
				continue
			}
			for _, threshold := range thresholds {
				if w.UsedPercent < threshold {
					continue
				}
				note := Notification{Kind: AlertThreshold, Client: client, Window: alertWindowLabel(w), UsedPercent: w.UsedPercent, Threshold: threshold}
				if err := store.notifyOnce(ctx, notifier, note, w.WindowKey, w.ResetsAt, alertInstanceTolerance, now); err != nil {
					errs = append(errs, err)
				}
			}
			if cfg.ResetNotice && w.HasObservedResetAt && resetInCurrentOccurrence(w) {
				note := Notification{Kind: AlertReset, Client: client, Window: alertWindowLabel(w), UsedPercent: w.UsedPercent}
				if err := store.notifyOnce(ctx, notifier, note, w.WindowKey, w.ResetsAt, alertInstanceTolerance, now); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

// occurrenceOngoing reports whether a stored window still describes a live
// occurrence at now: it has a resets_at, and that instant is no more than
// alertInstanceTolerance in the past.
func occurrenceOngoing(w Window, now time.Time) bool {
	return !w.ResetsAt.IsZero() && !w.ResetsAt.Add(alertInstanceTolerance).Before(now)
}

// normalizeThresholds drops duplicates and values that cannot be a used-share
// threshold, keeping the caller's order.
func normalizeThresholds(values []float64) []float64 {
	seen := make(map[float64]bool, len(values))
	out := make([]float64, 0, len(values))
	for _, v := range values {
		if math.IsNaN(v) || v <= 0 || v > 100 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// resetInCurrentOccurrence keeps a reset observed in an earlier occurrence —
// for example one recorded before alerts were switched on — from notifying
// now. The current occurrence starts one window length before resets_at; the
// same tolerance as instance matching absorbs resets_at jitter. Callers only
// pass windows with a resets_at (occurrenceOngoing).
//
// Without a window length the start cannot be bounded, so no reset is
// treated as current (QA-R2-F1). observed_reset_at is sticky, and reset
// notices are deduplicated per occurrence by resets_at, so accepting it here
// would let one old decrease notify "has reset" again in every later
// occurrence, none of which observed a decrease. Such a window gets no reset
// notice at all: a missed notice rather than a false one.
func resetInCurrentOccurrence(w Window) bool {
	if w.WindowMinutesReason != "" || w.WindowMinutes <= 0 {
		return false
	}
	start := w.ResetsAt.Add(-time.Duration(w.WindowMinutes)*time.Minute - alertInstanceTolerance)
	return !w.ObservedResetAt.Before(start)
}

// alertWindowLabel names a window for a person. window_key is opaque and
// never shown (C6); the label comes from the vendor label and the window
// length.
func alertWindowLabel(w Window) string {
	length := ""
	if w.WindowMinutesReason == "" && w.WindowMinutes > 0 {
		length = windowLengthLabel(w.WindowMinutes)
	}
	switch {
	case w.Label != "" && length != "":
		return fmt.Sprintf("%s (%s)", w.Label, length)
	case w.Label != "":
		return w.Label
	case length != "":
		return length + " window"
	default:
		return "Quota window"
	}
}

func windowLengthLabel(minutes int) string {
	switch {
	case minutes%1440 == 0:
		return fmt.Sprintf("%d-day", minutes/1440)
	case minutes%60 == 0:
		return fmt.Sprintf("%d-hour", minutes/60)
	default:
		return fmt.Sprintf("%d-minute", minutes)
	}
}

// notifyOnce delivers note unless the ledger already holds a matching notice,
// and records the notice only after a successful delivery.
func (s *Store) notifyOnce(ctx context.Context, notifier Notifier, note Notification, windowKey string, instance time.Time, tolerance time.Duration, now time.Time) error {
	noticed, err := s.alertNoticed(ctx, note.Client, windowKey, note.Kind, note.Threshold, instance, tolerance)
	if err != nil || noticed {
		return err
	}
	if err := notifier.Notify(ctx, note); err != nil {
		return err
	}
	return s.recordAlertNotice(ctx, note.Client, windowKey, note.Kind, note.Threshold, instance, now)
}

func (s *Store) alertNoticed(ctx context.Context, client Client, windowKey string, kind AlertKind, threshold float64, instance time.Time, tolerance time.Duration) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quota_alert_notices
		WHERE client = ? AND window_key = ? AND kind = ? AND threshold = ? AND ABS(instance_unix - ?) <= ?`,
		string(client), windowKey, string(kind), threshold, instance.Unix(), int64(tolerance/time.Second),
	).Scan(&found)
	return found > 0, err
}

func (s *Store) recordAlertNotice(ctx context.Context, client Client, windowKey string, kind AlertKind, threshold float64, instance, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO quota_alert_notices(client, window_key, kind, threshold, instance_unix, notified_at)
		VALUES (?,?,?,?,?,?)`,
		string(client), windowKey, string(kind), threshold, instance.Unix(), now.Format(timeLayout),
	)
	return err
}

func (s *Store) pruneAlertNotices(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM quota_alert_notices WHERE instance_unix < ?`, before.Unix())
	return err
}
