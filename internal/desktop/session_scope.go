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
//
// GenerationID, when set, pins the committed generation the answer must come
// from: a read that verifies at any other generation is unavailable with
// generation_unavailable rather than silently answering from the latest one.
// AsOf, when set, is the instant the period windows and generated_at use, so
// several selections read as one batch share one window and one time.
type SessionScopeRequest struct {
	Scope        string
	Client       string
	Period       string
	RecentLimit  int
	GenerationID string
	AsOf         time.Time
}

func (r SessionScopeRequest) Validate() error {
	if r.GenerationID != "" && !isGenerationID(r.GenerationID) {
		return fmt.Errorf("%w: generation id must be 64 lowercase hexadecimal characters", ErrInvalidSessionScope)
	}
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
	Available     bool                       `json:"available"`
	Reason        *string                    `json:"reason"`
	Default       sessionorigin.DefaultScope `json:"default"`
	Selection     *SessionScopeSelection     `json:"selection"`
	PublicationID *string                    `json:"publication_id"`
	// GenerationID is sessionorigin.GenerationID of the verified read: shared
	// by every selection read at the same committed generation.
	GenerationID   *string                `json:"generation_id"`
	GeneratedAt    *string                `json:"generated_at"`
	OriginCoverage *SessionOriginCoverage `json:"origin_coverage"`
	Counts         *SessionScopeCounts    `json:"counts"`
	Recent         []SessionScopeRecent   `json:"recent"`
	WorkSignals    WorkSignalsSnapshot    `json:"work_signals"`
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
	if !request.AsOf.IsZero() {
		now = request.AsOf.UTC()
	}
	core, coreErr := store.OpenReadOnly(ctx, s.StateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	return s.sessionScopeView(ctx, core, coreErr, readSessionScopeDefault(ctx, core, coreErr), request, now), nil
}

// SessionScopeBatchClients and SessionScopeBatchPeriods are every client and
// period a batch answers, in answer order: all × today comes first and fixes
// the batch's generation.
var (
	SessionScopeBatchClients = []string{"all", "codex", "claude"}
	SessionScopeBatchPeriods = []string{"today", "7d", "30d"}
)

// sessionScopeBatchObserver is a test seam called before each view of a batch
// is read, with that view's index. It is nil in production.
var sessionScopeBatchObserver func(index int)

// SessionScopeBatch is one scope's view for every client and period.
type SessionScopeBatch struct {
	Views []SessionScopeView `json:"views"`
}

// SessionScopeBatch reads one scope for every client and period in one
// process, the read a consumer that needs all of them (the Widget publisher)
// makes once per publication instead of starting a helper per selection. The
// request's Client and Period are ignored. Every view is answered at one
// instant, and each view after the first available one is pinned to that
// view's generation, so a generation that moves mid-batch leaves the later
// views unavailable with generation_unavailable rather than splicing two
// generations together. Like SessionScopeView it never scans or writes.
func (s Service) SessionScopeBatch(ctx context.Context, request SessionScopeRequest) (SessionScopeBatch, error) {
	request.Client, request.Period = SessionScopeBatchClients[0], SessionScopeBatchPeriods[0]
	if err := request.Validate(); err != nil {
		return SessionScopeBatch{}, err
	}
	now := s.now().UTC()
	if !request.AsOf.IsZero() {
		now = request.AsOf.UTC()
	}
	core, coreErr := store.OpenReadOnly(ctx, s.StateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	batch := SessionScopeBatch{Views: make([]SessionScopeView, 0, len(SessionScopeBatchClients)*len(SessionScopeBatchPeriods))}
	for _, client := range SessionScopeBatchClients {
		for _, period := range SessionScopeBatchPeriods {
			if sessionScopeBatchObserver != nil {
				sessionScopeBatchObserver(len(batch.Views))
			}
			request.Client, request.Period = client, period
			view := s.sessionScopeView(ctx, core, coreErr, readSessionScopeDefault(ctx, core, coreErr), request, now)
			if request.GenerationID == "" && view.Available && view.GenerationID != nil {
				request.GenerationID = *view.GenerationID
			}
			batch.Views = append(batch.Views, view)
		}
	}
	return batch, nil
}

// SessionScopeDefaultBatch is the batch for the saved default scope, with the
// instant it was answered at. Views is empty when the default is unconfirmed:
// an unconfirmed default starts no batch.
type SessionScopeDefaultBatch struct {
	AsOf    string                     `json:"as_of"`
	Default sessionorigin.DefaultScope `json:"default"`
	Views   []SessionScopeView         `json:"views"`
}

// SessionScopeDefaultBatch answers SessionScopeBatch for the saved default
// scope at one whole-second instant. A process that has just published a
// serving snapshot (the App's bootstrap) hands it over with that publication,
// so the Widget publisher starts no helper of its own to read it.
func (s Service) SessionScopeDefaultBatch(ctx context.Context, recentLimit int) SessionScopeDefaultBatch {
	now := s.now().UTC().Truncate(time.Second)
	core, coreErr := store.OpenReadOnly(ctx, s.StateRoot)
	defaults := readSessionScopeDefault(ctx, core, coreErr)
	if coreErr == nil {
		core.Close()
	}
	result := SessionScopeDefaultBatch{AsOf: now.Format(time.RFC3339), Default: defaults, Views: []SessionScopeView{}}
	for attempt := 0; attempt <= sessionorigin.MaxSelectionRetries; attempt++ {
		if !defaults.Confirmed || defaults.Value == nil {
			result.Default, result.Views = defaults, []SessionScopeView{}
			return result
		}
		batch, err := s.SessionScopeBatch(ctx, SessionScopeRequest{Scope: *defaults.Value, RecentLimit: recentLimit, AsOf: now})
		if err != nil {
			return result
		}
		if len(batch.Views) > 0 && !sameDefaultScope(batch.Views[0].Default, defaults) {
			defaults = batch.Views[0].Default
			result.Default = defaults
			continue
		}
		result.Views = batch.Views
		return result
	}
	return result
}

func sameDefaultScope(a, b sessionorigin.DefaultScope) bool {
	if a.Confirmed != b.Confirmed || (a.Value == nil) != (b.Value == nil) || (a.Revision == nil) != (b.Revision == nil) || (a.Reason == nil) != (b.Reason == nil) {
		return false
	}
	if a.Value != nil && *a.Value != *b.Value {
		return false
	}
	if a.Revision != nil && *a.Revision != *b.Revision {
		return false
	}
	if a.Reason != nil && *a.Reason != *b.Reason {
		return false
	}
	return true
}

func isGenerationID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
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
	var view SessionScopeView
	for attempt := 0; attempt <= sessionorigin.MaxSelectionRetries; attempt++ {
		request, ok := defaultSessionScopeRequest(defaults, limit)
		if !ok {
			view = unavailableSessionScopeView(defaults, nil, SessionScopeReasonDefaultUnconfirmed)
			break
		}
		view = s.sessionScopeView(ctx, core, coreErr, defaults, request, now)
		if !view.Available || sameDefaultScope(view.Default, defaults) {
			break
		}
		defaults = view.Default
		if attempt == sessionorigin.MaxSelectionRetries {
			view = unavailableSessionScopeView(defaults, nil, sessionorigin.PublicationReasonGenerationUnavailable)
		}
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
			// Default and signals are read before the vector is verified, so
			// they share the selection's generation. A signal failure degrades
			// only the signal families, never the selection itself.
			defaults = readSessionScopeDefault(ctx, core, coreErr)
			if !defaults.Confirmed {
				return fmt.Errorf("%w: default scope unavailable", sessionorigin.ErrClassificationUnavailable)
			}
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
		if coreErr == nil {
			defaults = readSessionScopeDefault(ctx, core, coreErr)
		}
		view, _ := sessionorigin.Publish(originSelection, nil, nil, err, nil)
		return unavailableSessionScopeView(defaults, selection, view.Reason)
	}
	generation := sessionorigin.GenerationID(snapshot.Vector)
	if request.GenerationID != "" && request.GenerationID != generation {
		// The pinned generation is no longer the committed one. Answering from
		// the latest generation would splice two generations into one batch.
		return unavailableSessionScopeView(defaults, selection, sessionorigin.PublicationReasonGenerationUnavailable)
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
		PublicationID: &id, GenerationID: &generation, GeneratedAt: &generatedAt,
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
