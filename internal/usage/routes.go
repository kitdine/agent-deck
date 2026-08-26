package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// SessionRoute is an observed client lifecycle boundary. It contains no client
// payload beyond the logical session and validated event shape.
type SessionRoute struct {
	Client, SessionID, HookEvent, Source string
}

// HookDelivery is one accepted client Hook event, normalized at the client
// adapter boundary. Every accepted Codex or Claude delivery reaches
// RecordHookDelivery through this one shape; see
// docs/topics/switch-effectiveness-boundary/architecture.md Contract 0.
type HookDelivery struct {
	Client, SessionID, HookEvent, Source string
	// DeliveryID is generated once by the caller when a delivery is accepted
	// and guards the whole operation against an internal retry.
	DeliveryID string
	// ConfigMatched is nil for a non-config event; for a Claude ConfigChange
	// it reports whether the managed settings file matched the completed
	// selection at reconcile time.
	ConfigMatched *bool
	// HasSelection and Selection carry the completed provider selection this
	// delivery observed, when one exists.
	HasSelection bool
	Selection    store.ProviderSnapshot
	// ConflictSources is the ordered conflict-scan result from the single
	// settings snapshot the reconcile already read for a matched ConfigChange
	// (ConfigMatched true); empty means clean. It is not scanned, and stays
	// empty, for a mismatch or a non-config event.
	ConflictSources []string
	// SettingsChangedAt is the managed-settings mtime observed at reconcile
	// time, RFC3339Nano; '' when unavailable or inapplicable. Diagnostic
	// only, never identity.
	SettingsChangedAt string
	// SettingsUnreadable is set for a Claude ConfigChange whose managed
	// settings snapshot could not be read or parsed on any reconcile attempt.
	// The delivery is still accepted and observed — it must not be dropped —
	// but neither a match nor a conflict scan could be determined, so
	// ConfigMatched and ConflictSources carry no information and are ignored
	// when this is true. HasSelection and Selection still apply: the completed
	// selection is read from the store, not from the unreadable settings
	// document, so it is recorded whenever the reconcile did observe one.
	SettingsUnreadable bool
}

// hookRouteEffect is the Contract 0 policy result: what an accepted delivery
// means for the effective-route stream.
type hookRouteEffect string

const (
	routeEffectAdvance hookRouteEffect = "advance"
	routeEffectRetain  hookRouteEffect = "retain"
	routeEffectUnknown hookRouteEffect = "unknown"
	routeEffectNone    hookRouteEffect = "none"
)

// claudePriorAuthentication is Contract 3's ordered three-state
// classification of a Claude session's prior effective authentication.
type claudePriorAuthentication string

const (
	priorStateKeyed         claudePriorAuthentication = "keyed"
	priorStateNoKey         claudePriorAuthentication = "no-key"
	priorStateIndeterminate claudePriorAuthentication = "indeterminate"
)

// hookClassification is Contract 0's classify step output: the observation
// fields to persist, plus the optional route write it implies.
type hookClassification struct {
	effect             hookRouteEffect
	configMatched      any
	observedProvider   any
	observedMultiplier any
	observedViaWrapper any
	priorState         any
	conflictScan       any
	conflictSources    string
	settingsChangedAt  string
	writeRoute         bool
	routeProvider      string
	routeMultiplier    string
	routeViaWrapper    bool
}

// classifyHookDelivery is the body of Contract 0's classify step for every
// event except a Claude ConfigChange, which classifyConfigChange owns because
// it requires the prior-state classifier's store reads. SessionStart's
// mapping is exactly today's behavior and does not change here.
func classifyHookDelivery(delivery HookDelivery) hookClassification {
	switch delivery.HookEvent {
	case "SessionStart":
		if delivery.Source == "compact" || !delivery.HasSelection {
			return hookClassification{effect: routeEffectNone}
		}
		provider := runtimeProviderName(delivery.Selection.Name)
		return hookClassification{
			effect:             routeEffectAdvance,
			observedProvider:   provider,
			observedMultiplier: delivery.Selection.Multiplier,
			observedViaWrapper: boolToInt64(delivery.Selection.ViaWrapper),
			writeRoute:         true,
			routeProvider:      provider,
			routeMultiplier:    delivery.Selection.Multiplier,
			routeViaWrapper:    delivery.Selection.ViaWrapper,
		}
	default:
		// SessionEnd, or any other accepted non-boundary event.
		return hookClassification{effect: routeEffectNone}
	}
}

// classifyConfigChange is Contract 3's classify-step body for a Claude
// ConfigChange: the prior-state classifier that turns a matched rotation,
// removal, or indeterminate change into `retain` instead of overwriting the
// session's route with a switch the running session cannot have adopted. A
// settings mismatch keeps today's explicit `unknown` behavior unchanged and
// never runs the classifier, so prior_state and conflict_scan stay NULL for
// it. It runs on the same *sql.Conn transaction as the observation insert
// that follows, so the prior-route read (Contract 3 steps 1-2) can never race
// this same operation's own optional route write.
func (s *Service) classifyConfigChange(ctx context.Context, conn *sql.Conn, delivery HookDelivery) (hookClassification, error) {
	if delivery.SettingsUnreadable {
		// A read or parse failure of the single settings snapshot is
		// indeterminate, not a dropped delivery: the accepted Hook is still
		// observed, but neither a match nor a conflict scan could be
		// determined, so config_matched stays NULL and no route is written.
		// The completed selection comes from the store rather than the
		// unreadable document, so Stream 1's observed-selection columns still
		// apply whenever one was seen.
		result := hookClassification{
			effect:          routeEffectRetain,
			priorState:      string(priorStateIndeterminate),
			conflictScan:    "unreadable",
			conflictSources: "",
		}
		if delivery.HasSelection {
			result.observedProvider = runtimeProviderName(delivery.Selection.Name)
			result.observedMultiplier = delivery.Selection.Multiplier
			result.observedViaWrapper = boolToInt64(delivery.Selection.ViaWrapper)
		}
		return result, nil
	}

	matched := delivery.ConfigMatched != nil && *delivery.ConfigMatched
	result := hookClassification{}
	if delivery.ConfigMatched != nil {
		result.configMatched = boolToInt64(matched)
	}
	if !matched {
		result.effect = routeEffectUnknown
		result.writeRoute = true
		result.routeProvider = "unknown"
		result.routeMultiplier = "1"
		return result, nil
	}

	provider := runtimeProviderName(delivery.Selection.Name)
	result.observedProvider = provider
	result.observedMultiplier = delivery.Selection.Multiplier
	result.observedViaWrapper = boolToInt64(delivery.Selection.ViaWrapper)
	result.settingsChangedAt = delivery.SettingsChangedAt

	candidate, err := s.classifyPriorAuthentication(ctx, conn, delivery.SessionID)
	if err != nil {
		return hookClassification{}, err
	}
	conflicted := len(delivery.ConflictSources) > 0
	final := candidate
	if candidate == priorStateNoKey && conflicted {
		// Step 3: a candidate no-key is confirmed only when the settings
		// snapshot names no unowned credential source.
		final = priorStateIndeterminate
	}
	result.priorState = string(final)
	result.conflictSources = strings.Join(delivery.ConflictSources, ",")
	if conflicted {
		result.conflictScan = "conflicted"
	} else {
		result.conflictScan = "clean"
	}

	if final == priorStateNoKey && delivery.Selection.Credential != "" {
		// Step 4: only a confirmed no-key prior state plus a consistent new
		// selection carrying a credential is the effective first-key
		// transition. Every other matched change (rotation, removal, or an
		// indeterminate/keyed prior state) retains the existing route.
		result.effect = routeEffectAdvance
		result.writeRoute = true
		result.routeProvider = provider
		result.routeMultiplier = delivery.Selection.Multiplier
		result.routeViaWrapper = delivery.Selection.ViaWrapper
	} else {
		result.effect = routeEffectRetain
	}
	return result, nil
}

// classifyPriorAuthentication resolves Contract 3's ordered three-state
// classification (steps 1-2) of a Claude session's prior effective
// authentication. Step 1 reads the latest route for this session on the
// pinned Hook-delivery connection, because that table is the same one this
// operation may append to; step 2 falls back to the store's ordinary
// provider-timeline read at session start, which this operation never
// mutates, so it does not need the same connection to stay consistent with
// this transaction's own write.
func (s *Service) classifyPriorAuthentication(ctx context.Context, conn *sql.Conn, sessionID string) (claudePriorAuthentication, error) {
	routeProvider, found, err := s.latestClaudeSessionRouteProvider(ctx, conn, sessionID)
	if err != nil {
		return "", err
	}
	if found {
		switch routeProvider {
		case "official":
			return priorStateNoKey, nil
		case "", "unknown":
			return priorStateIndeterminate, nil
		default:
			return priorStateKeyed, nil
		}
	}

	firstAt, hasFirstAt, err := s.claudeSessionFirstAt(ctx, conn, sessionID)
	if err != nil {
		return "", err
	}
	if !hasFirstAt {
		return priorStateIndeterminate, nil
	}
	snapshot, err := s.Store.ProviderSnapshotAt(ctx, "claude", firstAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return priorStateIndeterminate, nil
		}
		return "", err
	}
	if !snapshot.Official && snapshot.Credential != "" {
		return priorStateKeyed, nil
	}
	if snapshot.Official && snapshot.Credential == "" {
		return priorStateNoKey, nil
	}
	return priorStateIndeterminate, nil
}

func (s *Service) latestClaudeSessionRouteProvider(ctx context.Context, conn *sql.Conn, sessionID string) (provider string, found bool, err error) {
	err = conn.QueryRowContext(ctx, `SELECT provider FROM usage_session_routes WHERE client='claude' AND session_id=? ORDER BY observed_at DESC,id DESC LIMIT 1`, sessionID).Scan(&provider)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return provider, err == nil, err
}

func (s *Service) claudeSessionFirstAt(ctx context.Context, conn *sql.Conn, sessionID string) (time.Time, bool, error) {
	var raw string
	err := conn.QueryRowContext(ctx, `SELECT first_at FROM usage_sessions WHERE client='claude' AND session_id=?`, sessionID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	at, parseErr := time.Parse(time.RFC3339Nano, raw)
	if parseErr != nil {
		// An invalid start time is indeterminate, not a hard failure.
		return time.Time{}, false, nil
	}
	return at, true, nil
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

// RecordHookDelivery persists the one accepted-Hook operation shared by every
// client: an append-only observation for every accepted delivery, plus zero
// or one effective-route row, in one atomic transaction. See
// docs/topics/switch-effectiveness-boundary/architecture.md Contract 0's
// whole-operation idempotence for the exact executable order this follows.
//
// The transaction is opened as BEGIN IMMEDIATE on a connection pinned to this
// call only (via *sql.Conn, not *sql.DB.BeginTx), so this Hook boundary's
// write-lock-at-start guarantee does not change transaction semantics for any
// other caller of the shared store connection pool.
func (s *Service) RecordHookDelivery(ctx context.Context, delivery HookDelivery) error {
	if delivery.Client != "codex" && delivery.Client != "claude" {
		return fmt.Errorf("unsupported hook delivery client %q", delivery.Client)
	}
	if strings.TrimSpace(delivery.SessionID) == "" || strings.TrimSpace(delivery.HookEvent) == "" {
		return errors.New("hook delivery requires session and hook event")
	}
	if strings.TrimSpace(delivery.DeliveryID) == "" {
		return errors.New("hook delivery requires a delivery id")
	}

	conn, err := s.Store.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			// Roll back on a background context: ctx may already be
			// canceled by the same error that requires this rollback.
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	var exists int
	err = conn.QueryRowContext(ctx, `SELECT 1 FROM usage_session_observations WHERE delivery_id=?`, delivery.DeliveryID).Scan(&exists)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		// Duplicate delivery: the whole operation is a no-op.
		if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		committed = true
		return nil
	}

	var classified hookClassification
	if delivery.HookEvent == "ConfigChange" {
		classified, err = s.classifyConfigChange(ctx, conn, delivery)
		if err != nil {
			return err
		}
	} else {
		classified = classifyHookDelivery(delivery)
	}
	observedAt := s.now().Format(time.RFC3339Nano)

	result, err := conn.ExecContext(ctx, `
		INSERT INTO usage_session_observations(
			client,session_id,observed_at,hook_event,source,
			config_matched,observed_provider,observed_multiplier,observed_via_wrapper,
			prior_state,conflict_scan,conflict_sources,route_effect,settings_changed_at,delivery_id
		)
		SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,?,?
		WHERE NOT EXISTS (SELECT 1 FROM usage_session_observations WHERE delivery_id=?)`,
		delivery.Client, delivery.SessionID, observedAt, delivery.HookEvent, delivery.Source,
		classified.configMatched, classified.observedProvider, classified.observedMultiplier, classified.observedViaWrapper,
		classified.priorState, classified.conflictScan, classified.conflictSources, string(classified.effect), classified.settingsChangedAt,
		delivery.DeliveryID,
		delivery.DeliveryID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Committed by a racing internal retry between the guard and this
		// insert: the same no-op outcome as the guard hit above.
		if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		committed = true
		return nil
	}

	if classified.writeRoute {
		route := SessionRoute{Client: delivery.Client, SessionID: delivery.SessionID, HookEvent: delivery.HookEvent, Source: delivery.Source}
		if err = s.recordSessionRouteConn(ctx, conn, route, observedAt, classified.routeProvider, classified.routeMultiplier, classified.routeViaWrapper); err != nil {
			return err
		}
	}

	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Service) recordSessionRouteConn(ctx context.Context, conn *sql.Conn, route SessionRoute, observedAt, provider, multiplier string, viaWrapper bool) error {
	key := strings.Join([]string{route.Client, route.SessionID, route.HookEvent, route.Source, provider, multiplier, fmt.Sprint(viaWrapper), observedAt}, "\x00")
	if s.beforeSessionRouteWrite != nil {
		s.beforeSessionRouteWrite()
	}
	_, err := conn.ExecContext(ctx, `
		INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
		SELECT ?,?,?,?,?,?,?,?,?,?
		WHERE NOT EXISTS (
			SELECT 1
			FROM (
				SELECT provider,multiplier,via_wrapper,hook_event,source
				FROM usage_session_routes
				WHERE client=? AND session_id=?
				ORDER BY observed_at DESC,id DESC
				LIMIT 1
			) AS previous
			WHERE previous.provider=? AND previous.multiplier=? AND previous.via_wrapper=?
				AND previous.hook_event=? AND previous.source=?
		)`,
		route.Client, route.SessionID, observedAt, provider, multiplier, viaWrapper, route.HookEvent, route.Source, "estimated", key,
		route.Client, route.SessionID, provider, multiplier, viaWrapper, route.HookEvent, route.Source)
	return err
}

func (s *Service) sessionRouteAt(ctx context.Context, client, sessionID, eventAt string) (provider, multiplier, quality string, found bool, err error) {
	err = s.Store.DB.QueryRowContext(ctx, `SELECT provider,multiplier,quality FROM usage_session_routes WHERE client=? AND session_id=? AND observed_at<=? ORDER BY observed_at DESC,id DESC LIMIT 1`, client, sessionID, eventAt).Scan(&provider, &multiplier, &quality)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", false, nil
	}
	return provider, multiplier, quality, err == nil, err
}
