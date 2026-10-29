package telemetry

import (
	"context"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// ProjectSessionIndex writes the OTel-owned conversations held in core into
// the session index, so the existing session list, show and popover session
// panel read them. Core stays the source of truth; the index projection is
// regenerated from it after any purge or rebuild.
func ProjectSessionIndex(ctx context.Context, core, index *store.Store) error {
	owned, err := core.TelemetrySessions(ctx)
	if err != nil {
		return err
	}
	return session.ProjectTelemetrySessions(ctx, index.DB, projected(owned))
}

// SettleAndProject runs one settle pass and refreshes the session-index
// projection whenever the core holds OTel-owned sessions or the index still
// holds projected sources, so a retraction made by a JSONL write, or a
// purged and rebuilt index, converges without its own trigger. The caller
// passes its open index connection. It reports whether counted usage changed.
func SettleAndProject(ctx context.Context, core, index *store.Store, now time.Time) (bool, error) {
	result, err := core.SettleTelemetry(ctx, now, SettleAfter)
	if err != nil {
		return false, err
	}
	owned, err := core.TelemetrySessions(ctx)
	if err != nil {
		return result.Changed, err
	}
	var present int
	if err := index.DB.QueryRowContext(ctx, `SELECT count(*) FROM session_sources WHERE substr(source_path,1,5)='otel:'`).Scan(&present); err != nil {
		return result.Changed, err
	}
	if len(owned) == 0 && present == 0 {
		return result.Changed, nil
	}
	return result.Changed, session.ProjectTelemetrySessions(ctx, index.DB, projected(owned))
}

func projected(owned []store.TelemetrySession) []session.TelemetrySession {
	out := make([]session.TelemetrySession, 0, len(owned))
	for _, item := range owned {
		out = append(out, session.TelemetrySession{Client: item.Client, SessionID: item.SessionID, Model: item.Model, FirstAt: item.FirstAt, LastAt: item.LastAt})
	}
	return out
}
