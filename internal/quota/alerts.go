package quota

import (
	"context"
	"database/sql"
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
	Generation    Generation
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

const alertIDPrefix = "qa2."

type alertKey struct {
	Client     Client     `json:"c"`
	WindowKey  string     `json:"w"`
	Kind       AlertKind  `json:"k"`
	Threshold  float64    `json:"t"`
	Instance   int64      `json:"i"`
	Generation Generation `json:"g"`
}

func (k alertKey) id() string {
	encoded, _ := json.Marshal(k)
	return alertIDPrefix + base64.RawURLEncoding.EncodeToString(encoded)
}

func parseAlertID(id string) (alertKey, error) {
	raw, ok := strings.CutPrefix(id, alertIDPrefix)
	legacy := !ok
	if legacy {
		raw, ok = strings.CutPrefix(id, "qa1.")
	}
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
	canonical := key.id()
	if legacy {
		// qa1 is the same canonical object without the final generation field.
		image, _ := json.Marshal(struct {
			alertKey
			OmitGeneration *Generation `json:"g,omitempty"`
		}{alertKey: key})
		canonical = "qa1." + base64.RawURLEncoding.EncodeToString(image)
	}
	if !validClient || !validKind || key.WindowKey == "" || key.Instance <= 0 || key.Generation < 0 ||
		(key.Client == ClientCodex && key.Generation != 0) || canonical != id {
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
	if !probeEnabled || !cfg.Enabled || (!officialClients[ClientClaude] && !officialClients[ClientCodex]) {
		return nil, nil
	}
	version, err := readQuota(store.db, ctx, func(reader *SnapshotReader) (int, error) {
		return reader.ReadVersion().Schema, nil
	})
	if err != nil || version < 29 {
		return nil, err
	}
	if version >= identitySchemaVersion {
		if err := store.pruneAlertNotices(ctx, now.Add(-alertNoticeRetention)); err != nil {
			return nil, err
		}
	}
	return readQuota(store.db, ctx, func(reader *SnapshotReader) ([]DueAlert, error) {
		return reader.dueAlerts(ctx, officialClients, cfg, now)
	})
}

func (r *SnapshotReader) dueAlerts(ctx context.Context, officialClients map[Client]bool, cfg AlertConfig, now time.Time) ([]DueAlert, error) {
	var due []DueAlert
	thresholds := normalizeThresholds(cfg.Thresholds)
	var errs []error
	for _, client := range []Client{ClientCodex, ClientClaude} {
		if !officialClients[client] {
			continue
		}
		windows, err := r.Windows(ctx, client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// Codex PR #5 eleventh review, P2: mirror desktop.BuildSubscription's
		// own suppression of a client whose envelope shows an unsuperseded
		// parse failure -- those retained windows are not read as this
		// account's current quota there either, and must not still generate
		// threshold or reset alerts just because they have not yet been
		// probed away. Only Claude's own status-line route (a success the
		// envelope never records) can supersede a fresher prose failure.
		rec, ok, err := r.Usage(ctx, client)
		if err != nil {
			errs = append(errs, err)
			continue
		} else if ok && rec.Envelope.Failure == ReasonParseFailed {
			windowSource, windowObservedAt := newestWindowSource(windows)
			supersededByStatusLine := windowSource == SourceClaudeStatusLine && rec.FailureOrderKnown && windowObservedAt.After(rec.Envelope.FailureObservedAt)
			if !supersededByStatusLine {
				continue
			}
		}
		var generation Generation
		if client == ClientClaude {
			generation = rec.Claude.Generation
		}
		for _, w := range windows {
			if !occurrenceOngoing(w, now) {
				continue
			}
			for _, threshold := range thresholds {
				if w.UsedPercent < threshold {
					continue
				}
				alert, isDue, err := r.dueAlert(ctx, client, generation, w, AlertThreshold, threshold)
				if err != nil {
					errs = append(errs, err)
				} else if isDue {
					due = append(due, alert)
				}
			}
			if cfg.ResetNotice && w.HasObservedResetAt && resetInCurrentOccurrence(w) {
				alert, isDue, err := r.dueAlert(ctx, client, generation, w, AlertReset, 0)
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

// newestWindowSource is C5 at the client level: the newest window's source,
// preferring the status-line route at equal age. Mirrors
// desktop.newestWindowSource; kept as a separate, unexported copy here since
// desktop's is private to that package and DueAlerts needs the same
// supersession check subscription.go performs.
func newestWindowSource(windows []Window) (Source, time.Time) {
	var source Source
	var observedAt time.Time
	for _, w := range windows {
		if w.ObservedAt.After(observedAt) ||
			(w.ObservedAt.Equal(observedAt) && w.Source == SourceClaudeStatusLine && source != SourceClaudeStatusLine) {
			source, observedAt = w.Source, w.ObservedAt
		}
	}
	return source, observedAt
}

// AcknowledgeAlert records that the app delivered the notice id names. It is
// idempotent, so an acknowledgement repeated after a crash records nothing
// new. An id this evaluator could not have produced is rejected.
func AcknowledgeAlert(ctx context.Context, store *Store, id string, now time.Time) error {
	_, err := store.AcknowledgeAlerts(ctx, []string{id}, now)
	return err
}

// AcknowledgeAlerts counts distinct current notices, including idempotent
// replays. BEGIN IMMEDIATE orders this batch against identity invalidation.
func (s *Store) AcknowledgeAlerts(ctx context.Context, ids []string, now time.Time) (int, error) {
	keys := make(map[alertKey]struct{}, len(ids))
	for _, id := range ids {
		key, err := parseAlertID(id)
		if err != nil {
			return 0, err
		}
		keys[key] = struct{}{}
	}
	accepted := 0
	_, err := s.quotaWrite(ctx, time.Second, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		var generation *Generation
		for key := range keys {
			if key.Client == ClientClaude {
				if generation == nil {
					state, _, err := readClaudeContext(ctx, conn)
					if err != nil {
						return false, err
					}
					generation = &state.Generation
				}
				if key.Generation != *generation {
					continue
				}
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO quota_alert_notices_v2(client,window_key,kind,threshold,instance_unix,context_generation,notified_at)
				VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, key.Client, key.WindowKey, key.Kind, key.Threshold, key.Instance, key.Generation, now.Format(timeLayout)); err != nil {
				return false, err
			}
			accepted++
		}
		return accepted != 0, nil
	})
	if err != nil {
		return 0, err
	}
	return accepted, nil
}

// AlertStateRootID returns a proof only while this cycle owns the refresh
// lock and its bound delivery guard is still usable.
func (s *Store) AlertStateRootID(ctx context.Context, cycle RefreshCycle, guard *QuotaDeliveryGuard) (string, error) {
	if guard == nil || guard.cycle != cycle || s.checkCycle(cycle) != nil {
		return "", nil
	}
	var latest CycleSeq
	if err := s.db.QueryRowContext(ctx, "SELECT seq FROM quota_cycle WHERE singleton=1").Scan(&latest); err != nil {
		return "", err
	}
	if latest != cycle.seq {
		return "", nil
	}
	if err := guard.verifyAlertGuard(ctx); err != nil {
		return "", err
	}
	return guard.StateRootID(), nil
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
func (r *SnapshotReader) dueAlert(ctx context.Context, client Client, generation Generation, w Window, kind AlertKind, threshold float64) (DueAlert, bool, error) {
	noticed, err := r.alertNoticed(ctx, client, generation, w.WindowKey, kind, threshold, w.ResetsAt, alertInstanceTolerance)
	if err != nil || noticed {
		return DueAlert{}, false, err
	}
	minutes := 0
	if w.WindowMinutesReason == "" && w.WindowMinutes > 0 {
		minutes = w.WindowMinutes
	}
	key := alertKey{Client: client, WindowKey: w.WindowKey, Kind: kind, Threshold: threshold, Instance: w.ResetsAt.Unix(), Generation: generation}
	return DueAlert{ID: key.id(), Kind: kind, Client: client, Generation: generation, Label: w.Label, WindowMinutes: minutes, UsedPercent: w.UsedPercent, Threshold: threshold}, true, nil
}

func (r *SnapshotReader) alertNoticed(ctx context.Context, client Client, generation Generation, windowKey string, kind AlertKind, threshold float64, instance time.Time, tolerance time.Duration) (bool, error) {
	var found int
	table := "quota_alert_notices"
	predicate := ""
	args := []any{client, windowKey, kind, threshold, instance.Unix(), int64(tolerance / time.Second)}
	if r.version.Schema >= identitySchemaVersion {
		table = "quota_alert_notices_v2"
		predicate = " AND context_generation=?"
		args = append(args, generation)
	}
	err := r.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+`
		WHERE client = ? AND window_key = ? AND kind = ? AND threshold = ? AND ABS(instance_unix - ?) <= ?`+predicate,
		args...,
	).Scan(&found)
	return found > 0, err
}

func (s *Store) pruneAlertNotices(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM quota_alert_notices_v2 WHERE instance_unix < ?`, before.Unix())
	return err
}
