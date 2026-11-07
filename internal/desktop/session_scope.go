package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

// SessionScopeReasonDefaultUnconfirmed marks a snapshot's view when the saved
// default scope could not be read: no selection is implied, and a consumer
// must not fall back to work. The other unavailable reasons are
// sessionorigin's publication reasons.
const SessionScopeReasonDefaultUnconfirmed = "default_unconfirmed"

// ErrInvalidSessionScope rejects a session-scope request before any read.
var ErrInvalidSessionScope = errors.New("invalid_session_scope_request")

// SessionScopeRequest is one explicit selection: scope × client × period. It
// never depends on presentation state such as the Popover's current choice.
type SessionScopeRequest struct {
	Scope       string
	Client      string
	Period      string
	RecentLimit int
}

func (r SessionScopeRequest) Validate() error {
	if r.Scope != sessionorigin.ScopeWork && r.Scope != sessionorigin.ScopeAuxiliary && r.Scope != sessionorigin.ScopeAll {
		return fmt.Errorf("%w: scope must be work, auxiliary or all", ErrInvalidSessionScope)
	}
	if r.Client != "all" && r.Client != "codex" && r.Client != "claude" {
		return fmt.Errorf("%w: client must be all, codex or claude", ErrInvalidSessionScope)
	}
	if r.Period != "today" && r.Period != "7d" && r.Period != "30d" {
		return fmt.Errorf("%w: period must be today, 7d or 30d", ErrInvalidSessionScope)
	}
	if r.RecentLimit < 1 || r.RecentLimit > MaxRecentLimit {
		return fmt.Errorf("%w: recent limit must be 1-%d", ErrInvalidSessionScope, MaxRecentLimit)
	}
	return nil
}

// SessionScopeSelection names what a view answers. Every field is required
// whenever it is present; there is no implied default.
type SessionScopeSelection struct {
	Scope  string `json:"scope"`
	Client string `json:"client"`
	Period string `json:"period"`
}

// SessionOriginCoverage reports how complete the classification behind a view
// is. Reasons lists, sorted, the coverage-gap reasons present in the
// selection's client and period (before scope): a non-empty list means some
// session might change class once its source is readable again.
type SessionOriginCoverage struct {
	Complete     bool     `json:"complete"`
	UnknownCount *int     `json:"unknown_count"`
	Reasons      []string `json:"reasons"`
}

// SessionScopeCounts are recomputed from the selected set. KnownDurationMS is
// null when every selected session lacks a measurable duration; an empty
// selection is a trusted 0.
type SessionScopeCounts struct {
	Sessions             int    `json:"sessions"`
	Projects             int    `json:"projects"`
	KnownDurationMS      *int64 `json:"known_duration_ms"`
	MissingDurationCount int    `json:"missing_duration_count"`
}

// SessionScopeRecent is a recent row's historical safe fields plus its
// classification. The project is a basename label, never a full path.
type SessionScopeRecent struct {
	RecentSession
	Origin sessionorigin.Origin `json:"origin"`
}

// SessionScopeView is architecture §7's additive session_scope_view. It never
// reinterprets the legacy sessions fields: those keep describing every
// indexed session. When Available is false only Default, Selection (when one
// was requested) and Reason are meaningful, and every count is absent rather
// than zero.
type SessionScopeView struct {
	Available      bool                       `json:"available"`
	Reason         *string                    `json:"reason"`
	Default        sessionorigin.DefaultScope `json:"default"`
	Selection      *SessionScopeSelection     `json:"selection"`
	PublicationID  *string                    `json:"publication_id"`
	GeneratedAt    *string                    `json:"generated_at"`
	OriginCoverage *SessionOriginCoverage     `json:"origin_coverage"`
	Counts         *SessionScopeCounts        `json:"counts"`
	Recent         []SessionScopeRecent       `json:"recent"`
	WorkSignals    WorkSignalsSnapshot        `json:"work_signals"`
}

func unavailableSessionScopeView(defaults sessionorigin.DefaultScope, selection *SessionScopeSelection, reason string) SessionScopeView {
	return SessionScopeView{
		Reason: &reason, Default: defaults, Selection: selection,
		Recent: []SessionScopeRecent{}, WorkSignals: emptyWorkSignalsSnapshot(),
	}
}

// SessionScopeView reads one explicit selection from committed state. It
// never scans, creates or migrates state, and never opens a classification
// source: the classification it shows is the one already committed to core.
func (s Service) SessionScopeView(ctx context.Context, request SessionScopeRequest) (SessionScopeView, error) {
	if err := request.Validate(); err != nil {
		return SessionScopeView{}, err
	}
	now := s.now().UTC()
	core, coreErr := store.OpenReadOnly(ctx, s.StateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	return s.sessionScopeView(ctx, core, coreErr, readSessionScopeDefault(ctx, core, coreErr), request, now), nil
}

// readSessionScopeDefault reads the saved default the same way
// `session scope get` does: an unreadable core is an unconfirmed default.
func readSessionScopeDefault(ctx context.Context, core *store.Store, coreErr error) sessionorigin.DefaultScope {
	if coreErr != nil || core == nil {
		reason := sessionorigin.CodeReadFailed
		return sessionorigin.DefaultScope{Reason: &reason}
	}
	return (sessionorigin.Service{Store: core}).DefaultScope(ctx)
}

// defaultSessionScopeRequest is the selection a published snapshot carries:
// the confirmed default scope over every client for today, which is where a
// newly opened Popover starts. Without a confirmed default there is none.
func defaultSessionScopeRequest(defaults sessionorigin.DefaultScope, limit int) (SessionScopeRequest, bool) {
	if !defaults.Confirmed || defaults.Value == nil {
		return SessionScopeRequest{}, false
	}
	return SessionScopeRequest{Scope: *defaults.Value, Client: "all", Period: "today", RecentLimit: limit}, true
}

// loadSessionScopeView adds the default selection's view to a full snapshot.
// It reads the same committed generation the rest of the snapshot does; the
// serving publication's generation check covers both.
func (s Service) loadSessionScopeView(ctx context.Context, limit int, now time.Time, result *Result) {
	core, coreErr := store.OpenReadOnly(ctx, s.StateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	defaults := readSessionScopeDefault(ctx, core, coreErr)
	request, ok := defaultSessionScopeRequest(defaults, limit)
	var view SessionScopeView
	if !ok {
		view = unavailableSessionScopeView(defaults, nil, SessionScopeReasonDefaultUnconfirmed)
	} else {
		view = s.sessionScopeView(ctx, core, coreErr, defaults, request, now)
	}
	result.Snapshot.SessionScopeView = &view
}

// todaySessionScopeView is the view of a today-only bootstrap publication,
// which carries no session data: the default selection is named but
// unavailable until a full publication or an explicit read.
func todaySessionScopeView(ctx context.Context, core *store.Store) *SessionScopeView {
	defaults := readSessionScopeDefault(ctx, core, nil)
	request, ok := defaultSessionScopeRequest(defaults, DefaultRecentLimit)
	if !ok {
		view := unavailableSessionScopeView(defaults, nil, SessionScopeReasonDefaultUnconfirmed)
		return &view
	}
	view := unavailableSessionScopeView(defaults, request.selection(), sessionorigin.PublicationReasonSessionsUnavailable)
	return &view
}

func (r SessionScopeRequest) selection() *SessionScopeSelection {
	return &SessionScopeSelection{Scope: r.Scope, Client: r.Client, Period: r.Period}
}

func (s Service) sessionScopeView(ctx context.Context, core *store.Store, coreErr error, defaults sessionorigin.DefaultScope, request SessionScopeRequest, now time.Time) SessionScopeView {
	selection := request.selection()
	var window desktopPeriodRange
	for _, candidate := range desktopPeriodRanges(now, s.location()) {
		if candidate.name == request.Period {
			window = candidate
		}
	}
	client := request.Client
	if client == "all" {
		client = ""
	}
	originSelection := sessionorigin.Selection{Scope: request.Scope, Client: client, Period: window.name, From: window.start, To: window.end}
	sessions, err := store.OpenSessionsReadOnly(ctx, s.StateRoot)
	if err != nil {
		return unavailableSessionScopeView(defaults, selection, sessionorigin.PublicationReasonSessionsUnavailable)
	}
	defer sessions.Close()
	reader := sessionorigin.Reader{Core: core, Sessions: sessions, StateRoot: s.StateRoot}
	signals := emptyWorkSignalsSnapshot()
	var snapshot sessionorigin.Snapshot
	switch {
	case coreErr == nil:
		snapshot, err = reader.Select(ctx, originSelection, func(ctx context.Context, selected *sessionorigin.Snapshot) error {
			// Signals are read before the vector is verified, so they share the
			// selection's generation. A signal failure degrades only the
			// signal families, never the selection itself.
			signals = s.scopedWorkSignals(ctx, core, now, window, request.Client, selected.SessionFilter())
			return nil
		})
	case coreDatabaseAbsent(s.StateRoot):
		// A core that was never created holds no classification: every session
		// is unknown and stays in Work. It also holds no usage, so there are no
		// signals to read.
		snapshot, err = reader.SelectAbsentCore(ctx, originSelection)
	default:
		err = fmt.Errorf("%w: %v", sessionorigin.ErrClassificationUnavailable, coreErr)
	}
	if err != nil {
		view, _ := sessionorigin.Publish(originSelection, nil, nil, err, nil)
		return unavailableSessionScopeView(defaults, selection, view.Reason)
	}
	publication := sessionorigin.NewPublication(snapshot, now)
	recent := make([]SessionScopeRecent, 0, min(request.RecentLimit, len(snapshot.Sessions)))
	for _, value := range snapshot.Sessions[:min(request.RecentLimit, len(snapshot.Sessions))] {
		recent = append(recent, SessionScopeRecent{
			RecentSession: RecentSession{
				Client: value.Client, SessionID: value.SessionID,
				Project: projectLabel(value.Project), Model: value.Model,
				FirstAt: value.FirstAt, LastAt: value.LastAt,
			},
			Origin: value.Origin,
		})
	}
	counts := SessionScopeCounts{
		Sessions: snapshot.Counts.Sessions, Projects: snapshot.Counts.Projects,
		MissingDurationCount: snapshot.Counts.MissingDurationCount,
	}
	if snapshot.Counts.Sessions == 0 || snapshot.Counts.MissingDurationCount < snapshot.Counts.Sessions {
		known := snapshot.Counts.KnownDurationMS
		counts.KnownDurationMS = &known
	}
	reasons := make([]string, 0, len(snapshot.CoverageGaps))
	for reason := range snapshot.CoverageGaps {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	unknown := snapshot.UnknownCount
	id, generatedAt := publication.ID, publication.GeneratedAt
	return SessionScopeView{
		Available: true, Default: defaults, Selection: selection,
		PublicationID: &id, GeneratedAt: &generatedAt,
		OriginCoverage: &SessionOriginCoverage{Complete: len(reasons) == 0, UnknownCount: &unknown, Reasons: reasons},
		Counts:         &counts, Recent: recent, WorkSignals: signals,
	}
}

// scopedWorkSignals reads the selection's three signal families restricted to
// its sessions. Each family is available exactly when its own report is; a
// read failure leaves all three unavailable rather than reporting zeros.
func (s Service) scopedWorkSignals(ctx context.Context, core *store.Store, now time.Time, window desktopPeriodRange, client string, filter *usage.SessionFilter) WorkSignalsSnapshot {
	service := usage.New(core, s.Home)
	service.Now = func() time.Time { return now }
	clientFilter := client
	if client == "all" {
		clientFilter = ""
	}
	reports, err := service.SignalsBatch(ctx, []usage.SignalOptions{{
		Period: window.name, From: window.start, To: window.end, Client: clientFilter, IncludeSub: true, Sessions: filter,
	}})
	if err != nil || len(reports) != 1 {
		return emptyWorkSignalsSnapshot()
	}
	snapshot := emptyWorkSignalsSnapshot()
	report := reports[0]
	snapshot.Activity.Available = report.Activity != nil && report.Activity.Available
	snapshot.Workflow.Available = report.Workflow != nil && report.Workflow.Available
	snapshot.Tooling.Available = report.Tooling != nil && report.Tooling.Available
	if err := appendWorkSignalReport(&snapshot, window.name, client, report); err != nil {
		return emptyWorkSignalsSnapshot()
	}
	return snapshot
}

// coreDatabaseAbsent reports a core file that was never created, which is a
// confirmed empty classification state rather than a read failure.
func coreDatabaseAbsent(stateRoot string) bool {
	_, err := os.Lstat(filepath.Join(stateRoot, "agentdeck.sqlite3"))
	return errors.Is(err, os.ErrNotExist)
}
