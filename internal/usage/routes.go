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
}

// hookRouteEffect is the Contract 0 policy result: what an accepted delivery
// means for the effective-route stream.
type hookRouteEffect string

const (
	routeEffectAdvance hookRouteEffect = "advance"
	routeEffectUnknown hookRouteEffect = "unknown"
	routeEffectNone    hookRouteEffect = "none"
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

// classifyHookDelivery is the body of Contract 0's classify step, fixed to
// exactly today's routing behavior: Task 1 emits only advance, unknown, and
// none. A later task replaces this body with a prior-state classifier that
// introduces retain; it does not move the step this function fills.
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
	case "ConfigChange":
		var matched bool
		result := hookClassification{}
		if delivery.ConfigMatched != nil {
			matched = *delivery.ConfigMatched
			result.configMatched = boolToInt64(matched)
		}
		if !matched {
			result.effect = routeEffectUnknown
			result.writeRoute = true
			result.routeProvider = "unknown"
			result.routeMultiplier = "1"
			return result
		}
		provider := runtimeProviderName(delivery.Selection.Name)
		result.effect = routeEffectAdvance
		result.observedProvider = provider
		result.observedMultiplier = delivery.Selection.Multiplier
		result.observedViaWrapper = boolToInt64(delivery.Selection.ViaWrapper)
		result.writeRoute = true
		result.routeProvider = provider
		result.routeMultiplier = delivery.Selection.Multiplier
		result.routeViaWrapper = delivery.Selection.ViaWrapper
		return result
	default:
		// SessionEnd, or any other accepted non-boundary event.
		return hookClassification{effect: routeEffectNone}
	}
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

	classified := classifyHookDelivery(delivery)
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
