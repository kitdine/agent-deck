package usage

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"
)

var signalToolKindOrder = []string{"edit", "read", "bash", "mcp", "other"}

// SignalOptions selects the one period/client position rendered by the CLI.
// Activity, when set, narrows every family to turns of that category or
// subcategory; it is not merely a presentation filter on the Activity rows.
type SignalOptions struct {
	Period     string
	From       time.Time
	To         time.Time
	Client     string
	Kinds      []string
	Activity   string
	IncludeSub bool
}

type SignalReport struct {
	Period   string          `json:"period"`
	Client   string          `json:"client"`
	Activity *SignalActivity `json:"activity,omitempty"`
	Workflow *SignalWorkflow `json:"workflow,omitempty"`
	Tooling  *SignalTooling  `json:"tooling,omitempty"`
}

type SignalActivity struct {
	Available bool                 `json:"available"`
	CostBasis string               `json:"cost_basis,omitempty"`
	Kinds     []SignalActivityKind `json:"kinds,omitempty"`
}

type SignalActivityKind struct {
	Kind   string              `json:"kind"`
	Share  float64             `json:"share"`
	Cost   float64             `json:"cost"`
	Events int64               `json:"events"`
	Sub    []SignalActivitySub `json:"sub,omitempty"`
}

type SignalActivitySub struct {
	Kind   string  `json:"kind"`
	Share  float64 `json:"share"`
	Cost   float64 `json:"cost"`
	Events int64   `json:"events"`
}

type SignalWorkflow struct {
	Available        bool     `json:"available"`
	FirstEditSeconds *int     `json:"first_edit_seconds"`
	FilesTouched     *int     `json:"files_touched"`
	Retries          *int     `json:"retries"`
	EditsPerSession  *float64 `json:"edits_per_session"`
	TopFile          *string  `json:"top_file"`
	TopFileEdits     *int     `json:"top_file_edits"`
}

type SignalToolRow struct {
	Kind  string  `json:"kind"`
	Calls int64   `json:"calls"`
	Share float64 `json:"share"`
}

type SignalTooling struct {
	Available    bool            `json:"available"`
	Calls        int64           `json:"calls"`
	Groups       int             `json:"groups"`
	Rows         []SignalToolRow `json:"rows,omitempty"`
	TopMCPServer string          `json:"top_mcp_server,omitempty"`
	TopMCPCalls  int64           `json:"top_mcp_calls,omitempty"`
}

// SessionSignalSummary is the bounded derivation used by session show. It
// carries no path digest and exposes only the base-name-free counts that the
// one-line session contract needs.
type SessionSignalSummary struct {
	CostBasis        string
	Kind             string
	ToolCalls        int64
	FilesTouched     *int
	FirstEditSeconds *int
}

func (s *Service) Signals(ctx context.Context, options SignalOptions) (SignalReport, error) {
	if !options.From.Before(options.To) {
		return SignalReport{}, fmt.Errorf("usage signals range must have from before to")
	}
	if options.Client != "" && options.Client != "codex" && options.Client != "claude" {
		return SignalReport{}, fmt.Errorf("usage signals client must be codex or claude")
	}
	if options.Activity != "" && !ValidActivityFilter(options.Activity) {
		return SignalReport{}, fmt.Errorf("usage signals activity must be a documented category or subcategory")
	}
	selected, err := selectedSignalFamilies(options.Kinds)
	if err != nil {
		return SignalReport{}, err
	}
	client := options.Client
	if client == "" {
		client = "all"
	}
	report := SignalReport{Period: options.Period, Client: client}
	scope := SignalScope{From: options.From, To: options.To, Client: options.Client, Activity: options.Activity}
	if selected["activity"] {
		cost, costErr := s.ActivityCostForScope(ctx, scope)
		if costErr != nil {
			return SignalReport{}, costErr
		}
		activity, convertErr := signalActivity(cost, options.IncludeSub, options.Activity)
		if convertErr != nil {
			return SignalReport{}, convertErr
		}
		report.Activity = &activity
	}
	if selected["workflow"] {
		metrics, metricsErr := s.WorkflowMetrics(ctx, scope)
		if metricsErr != nil {
			return SignalReport{}, metricsErr
		}
		report.Workflow = &SignalWorkflow{
			Available:        metrics.EditsPerSession != nil,
			FirstEditSeconds: metrics.FirstEditSeconds,
			FilesTouched:     metrics.FilesTouched,
			Retries:          metrics.Retries,
			EditsPerSession:  metrics.EditsPerSession,
			TopFile:          metrics.TopFile,
			TopFileEdits:     metrics.TopFileEdits,
		}
	}
	if selected["tooling"] {
		tooling, toolingErr := s.ToolingSummary(ctx, scope)
		if toolingErr != nil {
			return SignalReport{}, toolingErr
		}
		report.Tooling = &tooling
	}
	return report, nil
}

// EarliestSignalAt lets the shared `all` range include the full first turn.
// Usage events begin at the assistant response, while workflow and tooling are
// scoped by the earlier user-message turn boundary.
func (s *Service) EarliestSignalAt(ctx context.Context) (*time.Time, error) {
	var raw sql.NullString
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT MIN(started_at) FROM usage_work_signals WHERE state=?`, signalStateClassified).Scan(&raw); err != nil {
		return nil, err
	}
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

func selectedSignalFamilies(values []string) (map[string]bool, error) {
	selected := map[string]bool{"activity": false, "workflow": false, "tooling": false}
	if len(values) == 0 {
		for kind := range selected {
			selected[kind] = true
		}
		return selected, nil
	}
	for _, value := range values {
		if _, ok := selected[value]; !ok {
			return nil, fmt.Errorf("usage signals kind must be activity, workflow, or tooling")
		}
		selected[value] = true
	}
	return selected, nil
}

func ValidActivityFilter(value string) bool {
	for _, kind := range activityKindPrecedence {
		if value == kind {
			return true
		}
		for _, sub := range activitySubOrder[kind] {
			if value == sub {
				return true
			}
		}
	}
	return false
}

func signalActivity(cost ActivityCost, includeSub bool, filter string) (SignalActivity, error) {
	turns := int64(0)
	for _, row := range cost.Kinds {
		turns += row.Turns
	}
	if turns == 0 {
		return SignalActivity{Available: false}, nil
	}
	out := SignalActivity{Available: true, CostBasis: cost.CostBasis, Kinds: []SignalActivityKind{}}
	for _, row := range cost.Kinds {
		parentSelected := filter == "" || filter == row.Kind
		subSelected := ""
		if !parentSelected {
			for _, sub := range row.Sub {
				if filter == sub.Kind {
					subSelected = sub.Kind
					break
				}
			}
		}
		if !parentSelected && subSelected == "" {
			continue
		}
		parsedCost, err := strconv.ParseFloat(row.Cost, 64)
		if err != nil {
			return SignalActivity{}, err
		}
		item := SignalActivityKind{Kind: row.Kind, Share: row.Share, Cost: parsedCost, Events: row.Events}
		if includeSub {
			for _, sub := range row.Sub {
				if subSelected != "" && sub.Kind != subSelected {
					continue
				}
				parsedSubCost, parseErr := strconv.ParseFloat(sub.Cost, 64)
				if parseErr != nil {
					return SignalActivity{}, parseErr
				}
				item.Sub = append(item.Sub, SignalActivitySub{Kind: sub.Kind, Share: sub.Share, Cost: parsedSubCost, Events: sub.Events})
			}
		}
		out.Kinds = append(out.Kinds, item)
	}
	return out, nil
}

func (s *Service) ToolingSummary(ctx context.Context, scope SignalScope) (SignalTooling, error) {
	query := `SELECT c.tool_kind,c.mcp_server,COUNT(*)
FROM usage_tool_calls c
JOIN usage_work_signals w ON w.client=c.client AND w.session_id=c.session_id AND w.turn_index=c.turn_index
WHERE w.state=? AND w.started_at>=? AND w.started_at<?`
	args := []any{signalStateClassified, scope.From.UTC().Format(time.RFC3339Nano), scope.To.UTC().Format(time.RFC3339Nano)}
	if scope.Client != "" {
		query += ` AND w.client=?`
		args = append(args, scope.Client)
	}
	if scope.Session != "" {
		query += ` AND w.session_id=?`
		args = append(args, scope.Session)
	}
	if scope.Activity != "" {
		query += ` AND (w.activity_kind=? OR w.activity_sub=?)`
		args = append(args, scope.Activity, scope.Activity)
	}
	query += ` GROUP BY c.tool_kind,c.mcp_server`
	rows, err := s.Store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return SignalTooling{}, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	mcpCounts := map[string]int64{}
	var total int64
	for rows.Next() {
		var kind string
		var server sql.NullString
		var count int64
		if err = rows.Scan(&kind, &server, &count); err != nil {
			return SignalTooling{}, err
		}
		if !knownSignalToolKind(kind) {
			kind = "other"
		}
		counts[kind] += count
		total += count
		if kind == "mcp" && server.Valid && server.String != "" {
			mcpCounts[server.String] += count
		}
	}
	if err = rows.Err(); err != nil {
		return SignalTooling{}, err
	}
	out := SignalTooling{Available: total > 0, Calls: total, Rows: []SignalToolRow{}}
	for _, kind := range signalToolKindOrder {
		count := counts[kind]
		if count == 0 {
			continue
		}
		share := math.Round(float64(count)*1000/float64(total)) / 10
		out.Rows = append(out.Rows, SignalToolRow{Kind: kind, Calls: count, Share: share})
	}
	out.Groups = len(out.Rows)
	for server, count := range mcpCounts {
		if count > out.TopMCPCalls || count == out.TopMCPCalls && (out.TopMCPServer == "" || server < out.TopMCPServer) {
			out.TopMCPServer, out.TopMCPCalls = server, count
		}
	}
	return out, nil
}

func knownSignalToolKind(value string) bool {
	for _, kind := range signalToolKindOrder {
		if value == kind {
			return true
		}
	}
	return false
}

func (s *Service) SessionSignals(ctx context.Context, client, sessionID string) (SessionSignalSummary, bool, error) {
	var turns int
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_work_signals WHERE state=? AND client=? AND session_id=?`, signalStateClassified, client, sessionID).Scan(&turns); err != nil {
		return SessionSignalSummary{}, false, err
	}
	if turns == 0 {
		return SessionSignalSummary{}, false, nil
	}
	category, err := s.SessionActivityCategory(ctx, client, sessionID)
	if err != nil {
		return SessionSignalSummary{}, false, err
	}
	scope := SignalScope{
		From:    time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
		Client:  client,
		Session: sessionID,
	}
	metrics, err := s.WorkflowMetrics(ctx, scope)
	if err != nil {
		return SessionSignalSummary{}, false, err
	}
	tooling, err := s.ToolingSummary(ctx, scope)
	if err != nil {
		return SessionSignalSummary{}, false, err
	}
	return SessionSignalSummary{
		CostBasis:        category.CostBasis,
		Kind:             category.Kind,
		ToolCalls:        tooling.Calls,
		FilesTouched:     metrics.FilesTouched,
		FirstEditSeconds: metrics.FirstEditSeconds,
	}, true, nil
}
