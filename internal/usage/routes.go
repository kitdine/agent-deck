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

// RecordSessionRoute stores one idempotent, estimated boundary using the
// completed provider selection visible when the hook ran.
func (s *Service) RecordSessionRoute(ctx context.Context, route SessionRoute) error {
	if route.Client != "codex" && route.Client != "claude" {
		return fmt.Errorf("unsupported route client %q", route.Client)
	}
	if strings.TrimSpace(route.SessionID) == "" || strings.TrimSpace(route.HookEvent) == "" {
		return errors.New("route requires session and hook event")
	}
	if route.Source == "compact" || route.HookEvent != "SessionStart" {
		return nil
	}
	snapshot, err := s.Store.CurrentProviderSnapshot(ctx, route.Client)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.recordSessionRoute(ctx, route, runtimeProviderName(snapshot.Name), snapshot.Multiplier, snapshot.ViaWrapper)
}

// RecordClaudeConfigChange persists an estimated route after the command
// boundary has reconciled the managed Claude settings file with a completed
// provider selection. An observed mismatch deliberately becomes unknown.
func (s *Service) RecordClaudeConfigChange(ctx context.Context, sessionID string, snapshot store.ProviderSnapshot, matched bool) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("route requires session and hook event")
	}
	route := SessionRoute{Client: "claude", SessionID: sessionID, HookEvent: "ConfigChange", Source: "user_settings"}
	if !matched {
		return s.recordSessionRoute(ctx, route, "unknown", "1", false)
	}
	return s.recordSessionRoute(ctx, route, runtimeProviderName(snapshot.Name), snapshot.Multiplier, snapshot.ViaWrapper)
}

func (s *Service) recordSessionRoute(ctx context.Context, route SessionRoute, provider, multiplier string, viaWrapper bool) error {
	observedAt := s.now().Format(time.RFC3339Nano)
	key := strings.Join([]string{route.Client, route.SessionID, route.HookEvent, route.Source, provider, multiplier, fmt.Sprint(viaWrapper), observedAt}, "\x00")
	if s.beforeSessionRouteWrite != nil {
		s.beforeSessionRouteWrite()
	}
	_, err := s.Store.Exec(ctx, `
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
