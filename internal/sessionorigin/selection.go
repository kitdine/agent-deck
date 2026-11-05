package sessionorigin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

// MaxSelectionRetries bounds how often a selection is re-read when the
// serving vector moves between capture and verification.
const MaxSelectionRetries = 2

var (
	// ErrSelectionStale reports a vector that kept moving through every retry.
	ErrSelectionStale = errors.New("session selection generation kept changing")
	// ErrSessionsUnavailable reports an unreadable session index.
	ErrSessionsUnavailable = errors.New("session index unavailable")
	// ErrClassificationUnavailable reports an unreadable core. Missing
	// classification metadata is not this error: it reads as unknown.
	ErrClassificationUnavailable = errors.New("session classification unavailable")
	// ErrGenerationUnavailable reports a vector that could not be verified.
	ErrGenerationUnavailable = errors.New("serving generation unavailable")
)

// Selection is one read-only session-scope request. A zero From/To selects
// every time; otherwise a session belongs to the window when its last event
// falls in [From, To). Query, when set, selects search matches instead of
// sessions.
type Selection struct {
	Scope  string    `json:"scope"`
	Client string    `json:"client"`
	Period string    `json:"period,omitempty"`
	From   time.Time `json:"-"`
	To     time.Time `json:"-"`
	Query  string    `json:"-"`
}

// Validate rejects a selection before any state is read.
func (s Selection) Validate() error {
	if s.Scope != ScopeWork && s.Scope != ScopeAuxiliary && s.Scope != ScopeAll {
		return fmt.Errorf("%w: session scope", ErrInvalidArgument)
	}
	if s.Client != "" && s.Client != "codex" && s.Client != "claude" {
		return fmt.Errorf("%w: client", ErrInvalidArgument)
	}
	if s.From.IsZero() != s.To.IsZero() || (!s.From.IsZero() && !s.From.Before(s.To)) {
		return fmt.Errorf("%w: time range", ErrInvalidArgument)
	}
	return nil
}

// key identifies what a publication answers; a result for another key is
// never a fallback for this one.
func (s Selection) key() string {
	bound := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("scope=%q client=%q period=%q from=%q to=%q query=%q", s.Scope, s.Client, s.Period, bound(s.From), bound(s.To), s.Query)
}

// Vector is the serving generation a selection was read at.
type Vector struct {
	CoreEpoch       int64 `json:"core_epoch"`
	UsageRevision   int64 `json:"usage_revision"`
	SessionEpoch    int64 `json:"session_epoch"`
	SessionRevision int64 `json:"session_revision"`
}

func vectorOf(generation store.ServingGeneration) Vector {
	return Vector{CoreEpoch: generation.CoreEpoch, UsageRevision: generation.UsageRevision, SessionEpoch: generation.SessionEpoch, SessionRevision: generation.SessionRevision}
}

func (v Vector) sameEpochs(other Vector) bool {
	return v.CoreEpoch == other.CoreEpoch && v.SessionEpoch == other.SessionEpoch
}

// OverrideState is the user override a session's classification considered.
// Applied is false for an unmatched binding: the override is kept for review
// and revocation but does not classify.
type OverrideState struct {
	Value          string `json:"value"`
	BindingStatus  string `json:"binding_status"`
	BindingLimited bool   `json:"binding_limited"`
	Applied        bool   `json:"applied"`
}

// Origin is one session's classification as every consumer shows it.
type Origin struct {
	Automatic   string         `json:"automatic"`
	Effective   string         `json:"effective"`
	ReasonCode  string         `json:"reason_code"`
	RuleVersion string         `json:"rule_version"`
	Override    *OverrideState `json:"override"`
	Revision    int64          `json:"revision"`
}

// SelectedSession is one session of a selection.
type SelectedSession struct {
	session.Metadata
	Origin Origin `json:"origin"`
}

// Counts are non-additive figures recomputed from the selected set.
type Counts struct {
	Sessions             int    `json:"sessions"`
	Projects             int    `json:"projects"`
	KnownDurationMS      int64  `json:"known_duration_ms"`
	MissingDurationCount int    `json:"missing_duration_count"`
	AverageDurationMS    *int64 `json:"average_duration_ms"`
}

// Snapshot is one same-generation selection. Sessions is the final selected
// set in List order, before pagination. For a search, Sessions are the
// selected sessions with a match and Documents their matches in Search order.
type Snapshot struct {
	Selection            Selection
	Vector               Vector
	Sessions             []SelectedSession
	Documents            []session.Document
	SelectedSessionCount int
	UnknownCount         int
	Counts               Counts
	auxiliary            []usage.SessionKey
}

// SessionFilter restricts work signals to the snapshot's scope. Work keeps
// every session that is not effectively auxiliary, so unknown and
// usage-only sessions stay in Work; All selects everything.
func (s Snapshot) SessionFilter() *usage.SessionFilter {
	switch s.Selection.Scope {
	case ScopeWork:
		return usage.ExcludeSessions(s.auxiliary)
	case ScopeAuxiliary:
		return usage.IncludeSessions(s.auxiliary)
	}
	return nil
}

// Reader assembles selections from read-only stores. Verify reads the
// current serving vector over a fresh connection; when nil it reads
// StateRoot's serving generation.
type Reader struct {
	Core      *store.Store
	Sessions  *store.Store
	StateRoot string
	Verify    func(context.Context) (store.ServingGeneration, error)
}

// beforeSelectionVerify lets tests interleave writes between a selection's
// reads and its verification.
var beforeSelectionVerify = func(attempt int) {}

// Select reads the selection in a read transaction per database, runs attach
// for further data that must share its generation (for example work
// signals), then verifies the vector over a new connection. A moved vector
// discards the candidate and retries at most MaxSelectionRetries times.
// Select never writes, scans or opens a source.
func (r Reader) Select(ctx context.Context, selection Selection, attach func(context.Context, *Snapshot) error) (Snapshot, error) {
	if err := selection.Validate(); err != nil {
		return Snapshot{}, err
	}
	verify := r.Verify
	if verify == nil {
		verify = func(ctx context.Context) (store.ServingGeneration, error) {
			return store.ReadServingGeneration(ctx, r.StateRoot)
		}
	}
	for attempt := 0; attempt <= MaxSelectionRetries; attempt++ {
		snapshot, err := r.read(ctx, selection)
		if err != nil {
			return Snapshot{}, err
		}
		if attach != nil {
			if err := attach(ctx, &snapshot); err != nil {
				return Snapshot{}, err
			}
		}
		beforeSelectionVerify(attempt)
		current, err := verify(ctx)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%w: %v", ErrGenerationUnavailable, err)
		}
		if vectorOf(current) == snapshot.Vector {
			return snapshot, nil
		}
	}
	return Snapshot{}, ErrSelectionStale
}

func (r Reader) read(ctx context.Context, selection Selection) (Snapshot, error) {
	index, err := session.ReadIndexSnapshot(ctx, r.Sessions.DB, selection.Query)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrSessionsUnavailable, err)
	}
	state, err := r.Core.ReadOriginSelectionState(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrClassificationUnavailable, err)
	}
	snapshot := Snapshot{Selection: selection, Vector: Vector{CoreEpoch: state.Epoch, UsageRevision: state.Revision, SessionEpoch: index.Epoch, SessionRevision: index.Revision}}
	return assemble(snapshot, index, state), nil
}

type sessionKey struct{ client, sessionID string }

// assemble applies canonical identity (already resolved by the index read),
// then classification, then client, time and scope, and only then counts.
func assemble(snapshot Snapshot, index session.IndexSnapshot, state store.OriginSelectionState) Snapshot {
	selection := snapshot.Selection
	origins := make(map[sessionKey]store.SessionOrigin, len(state.Origins))
	for _, item := range state.Origins {
		origins[sessionKey{item.Client, item.SessionID}] = item
	}
	overrides := make(map[sessionKey]store.OriginOverride, len(state.Overrides))
	for _, item := range state.Overrides {
		overrides[sessionKey{item.Client, item.SessionID}] = item
	}
	selected := map[sessionKey]bool{}
	snapshot.Sessions = []SelectedSession{}
	for _, indexed := range index.Sessions {
		key := sessionKey{indexed.Client, indexed.SessionID}
		var origin *store.SessionOrigin
		if item, ok := origins[key]; ok {
			origin = &item
		}
		var override *store.OriginOverride
		if item, ok := overrides[key]; ok {
			override = &item
		}
		classified := Classify(indexed, origin, override)
		if classified.Effective == ClassAuxiliary {
			snapshot.auxiliary = append(snapshot.auxiliary, usage.SessionKey{Client: indexed.Client, SessionID: indexed.SessionID})
		}
		if !selection.includes(indexed.Metadata, classified.Effective) {
			continue
		}
		selected[key] = true
		snapshot.Sessions = append(snapshot.Sessions, SelectedSession{Metadata: indexed.Metadata, Origin: classified})
	}
	if selection.Query != "" {
		snapshot.Documents = []session.Document{}
		matched := map[sessionKey]bool{}
		for _, document := range index.Documents {
			key := sessionKey{document.Client, document.SessionID}
			if selected[key] {
				snapshot.Documents = append(snapshot.Documents, document)
				matched[key] = true
			}
		}
		withMatches := snapshot.Sessions[:0]
		for _, item := range snapshot.Sessions {
			if matched[sessionKey{item.Client, item.SessionID}] {
				withMatches = append(withMatches, item)
			}
		}
		snapshot.Sessions = withMatches
		snapshot.SelectedSessionCount = len(withMatches)
	}
	snapshot.Counts, snapshot.UnknownCount = countSessions(snapshot.Sessions)
	return snapshot
}

func (s Selection) includes(metadata session.Metadata, effective string) bool {
	if s.Client != "" && metadata.Client != s.Client {
		return false
	}
	if !s.From.IsZero() {
		last, err := time.Parse(time.RFC3339Nano, metadata.LastAt)
		if err != nil || last.Before(s.From) || !last.Before(s.To) {
			return false
		}
	}
	return InScope(s.Scope, effective)
}

// InScope reports whether an effective class belongs to a scope: Work holds
// work and unknown, Auxiliary only auxiliary, All everything.
func InScope(scope, effective string) bool {
	switch scope {
	case ScopeWork:
		return effective != ClassAuxiliary
	case ScopeAuxiliary:
		return effective == ClassAuxiliary
	}
	return true
}

// Classify derives one indexed session's classification. The stored
// automatic class applies only when its input token equals the token of the
// copies indexed now: otherwise it is unknown/origin_input_mismatch, so an
// outdated auxiliary class never hides a session. A missing row is
// unknown/origin_metadata_missing. An override classifies only while its
// binding is matched and equals the binding of the session's current birth.
func Classify(indexed session.IndexedSession, origin *store.SessionOrigin, override *store.OriginOverride) Origin {
	generations := make([]SourceGeneration, 0, len(indexed.Copies))
	sources := make([]store.SessionOriginSource, 0, len(indexed.Copies))
	for _, copy := range indexed.Copies {
		generations = append(generations, SourceGeneration{Identity: copy.Identity, PrefixHash: copy.PrefixHash, ParserVersion: copy.ParserVersion})
		sources = append(sources, store.SessionOriginSource{Client: indexed.Client, SessionID: indexed.SessionID, Identity: copy.Identity, PrefixHash: copy.PrefixHash, ParserVersion: copy.ParserVersion, FirstAt: copy.FirstAt})
	}
	result := Origin{Automatic: ClassUnknown, ReasonCode: ReasonOriginMetadataMissing, RuleVersion: ReducerVersion}
	if origin != nil {
		if origin.InputToken == InputToken(indexed.Client, indexed.SessionID, generations, nil) {
			result.Automatic, result.ReasonCode, result.RuleVersion = origin.Automatic, origin.ReasonCode, origin.RuleVersion
		} else {
			result.ReasonCode = ReasonOriginInputMismatch
		}
	}
	result.Effective = result.Automatic
	if override == nil {
		return result
	}
	result.Revision = override.Revision
	if override.Value == nil {
		return result
	}
	state := OverrideState{Value: *override.Value, BindingStatus: BindingUnmatched, BindingLimited: override.BindingLimited}
	if override.BindingState == BindingMatched && override.BindingDigest == BindingFor(indexed.Client, indexed.SessionID, BirthOf(sources)).Digest {
		state.BindingStatus, state.Applied = BindingMatched, true
		result.Effective = *override.Value
	}
	result.Override = &state
	return result
}

// countSessions recomputes the selection's figures from its own sessions.
// A duration is known when both ends parse and the last is not before the
// first; any other session counts as missing, never as zero.
func countSessions(values []SelectedSession) (Counts, int) {
	counts := Counts{Sessions: len(values)}
	projects := map[string]struct{}{}
	unknown, known := 0, 0
	for _, value := range values {
		if value.Origin.Effective == ClassUnknown {
			unknown++
		}
		if identity, _ := session.ProjectGroup(value.Project); identity != "" {
			projects[identity] = struct{}{}
		}
		first, firstErr := time.Parse(time.RFC3339Nano, value.FirstAt)
		last, lastErr := time.Parse(time.RFC3339Nano, value.LastAt)
		if firstErr != nil || lastErr != nil || last.Before(first) {
			counts.MissingDurationCount++
			continue
		}
		counts.KnownDurationMS += last.Sub(first).Milliseconds()
		known++
	}
	counts.Projects = len(projects)
	if known > 0 {
		average := counts.KnownDurationMS / int64(known)
		counts.AverageDurationMS = &average
	}
	return counts, unknown
}
