package quota

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
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

// DueAlert is one notice C10 has decided is due. The helper does not deliver
// it: the app posts it under its own bundle identity and then acknowledges it
// with AcknowledgeAlert. It names the client, the window, and the figure, and
// has no field an account identifier could travel in; ID is an opaque ledger
// key the app passes back unchanged.
type DueAlert struct {
	ID            string
	Kind          AlertKind
	Client        Client
	Label         string
	WindowMinutes int
	UsedPercent   float64
	Threshold     float64
}

// ErrInvalidAlertID reports an acknowledgement that does not name a notice
// this evaluator could have produced. Nothing is recorded for it.
var ErrInvalidAlertID = errors.New("invalid quota alert id")

const alertIDPrefix = "qa1."

type alertKey struct {
	Client    Client    `json:"c"`
	WindowKey string    `json:"w"`
	Kind      AlertKind `json:"k"`
	Threshold float64   `json:"t"`
	Instance  int64     `json:"i"`
}

func (k alertKey) id() string {
	encoded, _ := json.Marshal(k)
	return alertIDPrefix + base64.RawURLEncoding.EncodeToString(encoded)
}

func parseAlertID(id string) (alertKey, error) {
	raw, ok := strings.CutPrefix(id, alertIDPrefix)
	if !ok {
		return alertKey{}, ErrInvalidAlertID
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return alertKey{}, ErrInvalidAlertID
	}
	var key alertKey
	if err := json.Unmarshal(decoded, &key); err != nil {
		return alertKey{}, ErrInvalidAlertID
	}
	validClient := key.Client == ClientCodex || key.Client == ClientClaude
	validKind := (key.Kind == AlertThreshold && key.Threshold > 0 && key.Threshold <= 100) || (key.Kind == AlertReset && key.Threshold == 0)
	if !validClient || !validKind || key.WindowKey == "" || key.Instance <= 0 || key.id() != id {
		return alertKey{}, ErrInvalidAlertID
	}
	return key, nil
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

// DueAlerts is C10's evaluator. With reading off or alerts off it returns
// immediately without touching the store — nothing is evaluated, not merely
// nothing delivered (C9, C10). It records nothing: a notice enters the ledger
// only when the app acknowledges a delivery it made, so a notice the app could
// not post stays due and is offered again by the next evaluation.
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
// Errors from individual windows are collected and returned together with
// the notices found elsewhere; one failing window does not stop the rest from
// being evaluated.
// officialClients reports, per client, whether C1's provider gate currently
// resolves to the official provider. A client absent from the map, or
// mapped to false, is skipped entirely: when reading switched away from
// official for that client (Reason=not_official), its retained windows are
// no longer this account's current quota and must not still generate
// threshold or reset alerts, even though they have not yet been probed away.
func DueAlerts(ctx context.Context, store *Store, probeEnabled bool, officialClients map[Client]bool, cfg AlertConfig, now time.Time) ([]DueAlert, error) {
	if !probeEnabled || !cfg.Enabled {
		return nil, nil
	}
	if err := store.pruneAlertNotices(ctx, now.Add(-alertNoticeRetention)); err != nil {
		return nil, err
	}
	var due []DueAlert
	thresholds := normalizeThresholds(cfg.Thresholds)
	var errs []error
	for _, client := range []Client{ClientCodex, ClientClaude} {
		if !officialClients[client] {
			continue
		}
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
				alert, isDue, err := store.dueAlert(ctx, client, w, AlertThreshold, threshold)
				if err != nil {
					errs = append(errs, err)
				} else if isDue {
					due = append(due, alert)
				}
			}
			if cfg.ResetNotice && w.HasObservedResetAt && resetInCurrentOccurrence(w) {
				alert, isDue, err := store.dueAlert(ctx, client, w, AlertReset, 0)
				if err != nil {
					errs = append(errs, err)
				} else if isDue {
					due = append(due, alert)
				}
			}
		}
	}
	return due, errors.Join(errs...)
}

// AcknowledgeAlert records that the app delivered the notice id names. It is
// idempotent, so an acknowledgement repeated after a crash records nothing
// new. An id this evaluator could not have produced is rejected.
func AcknowledgeAlert(ctx context.Context, store *Store, id string, now time.Time) error {
	key, err := parseAlertID(id)
	if err != nil {
		return err
	}
	return store.recordAlertNotice(ctx, key.Client, key.WindowKey, key.Kind, key.Threshold, time.Unix(key.Instance, 0), now)
}

// ValidateAlertIDs checks every id before a caller records any of them.
func ValidateAlertIDs(ids []string) error {
	for _, id := range ids {
		if _, err := parseAlertID(id); err != nil {
			return err
		}
	}
	return nil
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

// dueAlert reports a notice for w unless the ledger already holds one for the
// same occurrence. The window's vendor label and length travel so the app can
// name the window; window_key stays opaque and is carried only inside the id.
func (s *Store) dueAlert(ctx context.Context, client Client, w Window, kind AlertKind, threshold float64) (DueAlert, bool, error) {
	noticed, err := s.alertNoticed(ctx, client, w.WindowKey, kind, threshold, w.ResetsAt, alertInstanceTolerance)
	if err != nil || noticed {
		return DueAlert{}, false, err
	}
	minutes := 0
	if w.WindowMinutesReason == "" && w.WindowMinutes > 0 {
		minutes = w.WindowMinutes
	}
	key := alertKey{Client: client, WindowKey: w.WindowKey, Kind: kind, Threshold: threshold, Instance: w.ResetsAt.Unix()}
	return DueAlert{ID: key.id(), Kind: kind, Client: client, Label: w.Label, WindowMinutes: minutes, UsedPercent: w.UsedPercent, Threshold: threshold}, true, nil
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
