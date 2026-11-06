package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/spf13/cobra"
)

// Session-scope sources reported in session_selection.source_default.
const (
	sessionScopeTextDefault = "text_default"
	sessionScopeJSONCompat  = "json_compat"
	sessionScopeExplicit    = "explicit"
)

// sessionScopeFlag accepts exactly one legal --session-scope value. A
// repeated flag with a different value is a conflicting input, rejected by
// flag parsing before any state is read or written.
type sessionScopeFlag struct {
	value string
	set   bool
}

func (f *sessionScopeFlag) String() string { return f.value }
func (f *sessionScopeFlag) Type() string   { return "scope" }

func (f *sessionScopeFlag) Set(value string) error {
	if value != sessionorigin.ScopeWork && value != sessionorigin.ScopeAuxiliary && value != sessionorigin.ScopeAll {
		return errors.New("session scope must be work, auxiliary, or all")
	}
	if f.set && f.value != value {
		return errors.New("conflicting --session-scope values")
	}
	f.value, f.set = value, true
	return nil
}

func addSessionScopeFlag(command *cobra.Command, flag *sessionScopeFlag) {
	command.Flags().Var(flag, "session-scope", "Session scope: work, auxiliary, or all (text default work, JSON default all)")
}

// resolve returns the effective scope, its source and the requested value.
// Text defaults to work; JSON without a scope keeps the historical all set.
func (f *sessionScopeFlag) resolve(format string) (scope, source string, requested *string) {
	if f.set {
		value := f.value
		return value, sessionScopeExplicit, &value
	}
	if format == "json" {
		return sessionorigin.ScopeAll, sessionScopeJSONCompat, nil
	}
	return sessionorigin.ScopeWork, sessionScopeTextDefault, nil
}

// sessionSelectionMeta is the additive top-level session_selection envelope
// field. It never replaces data and never changes its array/object shape.
type sessionSelectionMeta struct {
	RequestedScope       *string               `json:"requested_scope"`
	EffectiveScope       string                `json:"effective_scope"`
	SourceDefault        string                `json:"source_default"`
	Availability         string                `json:"availability"`
	Reason               *string               `json:"reason,omitempty"`
	UnknownCount         *int                  `json:"unknown_count"`
	SelectedSessionCount *int                  `json:"selected_session_count,omitempty"`
	ReadVector           *sessionorigin.Vector `json:"read_vector"`
	PublicationID        *string               `json:"publication_id"`
	// usageScope is the text-only consumption-scope label for usage stats.
	usageScope string
}

const (
	selectionAvailable   = "available"
	selectionUnavailable = "unavailable"
)

func newSessionSelectionMeta(scope, source string, requested *string) *sessionSelectionMeta {
	return &sessionSelectionMeta{RequestedScope: requested, EffectiveScope: scope, SourceDefault: source, Availability: selectionUnavailable}
}

func (m *sessionSelectionMeta) fill(snapshot sessionorigin.Snapshot) {
	m.Availability = selectionAvailable
	m.Reason = nil
	unknown := snapshot.UnknownCount
	m.UnknownCount = &unknown
	vector := snapshot.Vector
	m.ReadVector = &vector
	id := sessionorigin.NewPublication(snapshot, time.Now()).ID
	m.PublicationID = &id
	if snapshot.Selection.Query != "" {
		count := snapshot.SelectedSessionCount
		m.SelectedSessionCount = &count
	}
}

func (m *sessionSelectionMeta) unavailable(err error) {
	m.Availability = selectionUnavailable
	reason := sessionSelectionReason(err)
	m.Reason = &reason
}

func sessionSelectionReason(err error) string {
	switch {
	case errors.Is(err, sessionorigin.ErrSessionsUnavailable):
		return sessionorigin.PublicationReasonSessionsUnavailable
	case errors.Is(err, sessionorigin.ErrSelectionStale):
		return sessionorigin.PublicationReasonStale
	case errors.Is(err, sessionorigin.ErrGenerationUnavailable):
		return sessionorigin.PublicationReasonGenerationUnavailable
	}
	return sessionorigin.PublicationReasonClassificationUnavailable
}

// withSessionSelection carries session_selection beside a command's data
// through the shared result writers.
type withSessionSelection struct {
	data any
	meta *sessionSelectionMeta
}

func unwrapSessionSelection(data any) (any, *sessionSelectionMeta) {
	if value, ok := data.(withSessionSelection); ok {
		return value.data, value.meta
	}
	return data, nil
}

// errSessionSelectionUnavailable is the runtime failure of a scope whose
// membership depends on classification that cannot be read: no list or count
// is shown instead of a zero.
type errSessionSelectionUnavailable struct {
	scope string
	err   error
}

func (e *errSessionSelectionUnavailable) Error() string {
	return fmt.Sprintf("session scope %s is unavailable: %s; run agentdeck doctor", e.scope, sessionSelectionReason(e.err))
}

func (e *errSessionSelectionUnavailable) Unwrap() error { return e.err }

// sessionSelectionReader reads selections over the given session index and a
// read-only core. It never creates, migrates or scans state.
type sessionSelectionReader struct {
	stateRoot string
	sessions  *store.Store
	core      *store.Store
	coreErr   error
	// sessionsErr is why the session index could not be opened when
	// sessions is nil; it stays the cause of the unavailable selection.
	sessionsErr error
}

func openSessionSelectionReader(ctx context.Context, stateRoot string, sessions *store.Store) *sessionSelectionReader {
	core, err := store.OpenReadOnly(ctx, stateRoot)
	return &sessionSelectionReader{stateRoot: stateRoot, sessions: sessions, core: core, coreErr: err}
}

func (r *sessionSelectionReader) Close() {
	if r != nil && r.core != nil {
		_ = r.core.Close()
	}
}

func (r *sessionSelectionReader) selectSessions(ctx context.Context, selection sessionorigin.Selection, attach func(context.Context, *sessionorigin.Snapshot) error) (sessionorigin.Snapshot, error) {
	if r.sessions == nil {
		if r.sessionsErr != nil {
			return sessionorigin.Snapshot{}, fmt.Errorf("%w: %v", sessionorigin.ErrSessionsUnavailable, r.sessionsErr)
		}
		return sessionorigin.Snapshot{}, sessionorigin.ErrSessionsUnavailable
	}
	reader := sessionorigin.Reader{Core: r.core, Sessions: r.sessions, StateRoot: r.stateRoot}
	if r.coreErr != nil {
		// Only a core file that was never created is a confirmed empty
		// classification state; any other open failure is unavailable.
		if attach == nil && stateDatabaseAbsent(r.stateRoot, coreDatabaseName) {
			return reader.SelectAbsentCore(ctx, selection)
		}
		return sessionorigin.Snapshot{}, fmt.Errorf("%w: %v", sessionorigin.ErrClassificationUnavailable, r.coreErr)
	}
	return reader.Select(ctx, selection, attach)
}

const (
	coreDatabaseName    = "agentdeck.sqlite3"
	sessionDatabaseName = "sessions.sqlite3"
)

// stateDatabaseAbsent reports a database file that does not exist at all,
// which is a confirmed empty state rather than a read failure.
func stateDatabaseAbsent(stateRoot, name string) bool {
	_, err := os.Lstat(filepath.Join(stateRoot, name))
	return errors.Is(err, os.ErrNotExist)
}

// sessionListItem is a session.list element: the historical metadata fields
// plus the additive origin object.
type sessionListItem struct {
	session.Metadata
	Origin *sessionorigin.Origin `json:"origin,omitempty"`
}

type sessionDocumentItem struct {
	session.Document
	Origin *sessionorigin.Origin `json:"origin,omitempty"`
}

type sessionScopedListPage struct {
	Sessions    []sessionListItem             `json:"sessions"`
	Pagination  map[string]session.Pagination `json:"pagination,omitempty"`
	NextCommand string                        `json:"-"`
	hint        sessionListHint
}

type sessionScopedSearchPage struct {
	Documents   []sessionDocumentItem         `json:"documents"`
	Pagination  map[string]session.Pagination `json:"pagination"`
	NextCommand string                        `json:"-"`
	hint        sessionListHint
}

// sessionListHint is text-only context: the selection and why it is empty.
type sessionListHint struct {
	meta         *sessionSelectionMeta
	indexEmpty   bool
	sessionCount int
}

func listItemsFromSnapshot(snapshot sessionorigin.Snapshot) []sessionListItem {
	items := make([]sessionListItem, 0, len(snapshot.Sessions))
	for _, value := range snapshot.Sessions {
		origin := value.Origin
		items = append(items, sessionListItem{Metadata: value.Metadata, Origin: &origin})
	}
	return items
}

func documentItemsFromSnapshot(snapshot sessionorigin.Snapshot) []sessionDocumentItem {
	origins := map[[2]string]sessionorigin.Origin{}
	for _, value := range snapshot.Sessions {
		origins[[2]string{value.Client, value.SessionID}] = value.Origin
	}
	items := make([]sessionDocumentItem, 0, len(snapshot.Documents))
	for _, document := range snapshot.Documents {
		item := sessionDocumentItem{Document: document}
		if origin, ok := origins[[2]string{document.Client, document.SessionID}]; ok {
			item.Origin = &origin
		}
		items = append(items, item)
	}
	return items
}

// scopedNextCommand keeps an explicit scope in the copyable next-page command.
func scopedNextCommand(next string, scope string, explicit bool) string {
	if next == "" || !explicit {
		return next
	}
	return strings.Replace(next, " --page ", " --session-scope "+scope+" --page ", 1)
}

func sessionClassLabel(class string) string {
	switch class {
	case sessionorigin.ClassWork:
		return "WORK"
	case sessionorigin.ClassAuxiliary:
		return "AUXILIARY"
	case sessionorigin.ClassUnknown:
		return "UNKNOWN"
	}
	return "UNAVAILABLE"
}

func renderSessionScopeLine(w io.Writer, meta *sessionSelectionMeta, noun string, count int) error {
	if meta == nil {
		return nil
	}
	line := "Scope: " + meta.EffectiveScope
	if noun != "" {
		line += fmt.Sprintf(" · %d %s", count, noun)
	}
	if meta.Availability != selectionAvailable {
		line += " · classification unavailable"
	} else if meta.UnknownCount != nil && meta.EffectiveScope != sessionorigin.ScopeAuxiliary {
		line += fmt.Sprintf(" · %d unknown", *meta.UnknownCount)
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func renderSessionListEmpty(w io.Writer, hint sessionListHint) error {
	if hint.indexEmpty {
		_, err := fmt.Fprintln(w, "No indexed sessions.\nRun: agentdeck session scan")
		return err
	}
	if hint.meta != nil {
		_, err := fmt.Fprintf(w, "No %s sessions in this selection.\n", hint.meta.EffectiveScope)
		if err == nil && hint.meta.EffectiveScope != sessionorigin.ScopeAll {
			_, err = fmt.Fprintln(w, "Try: --session-scope all")
		}
		return err
	}
	_, err := fmt.Fprintln(w, "No sessions.")
	return err
}

func renderUnknownHint(w io.Writer, meta *sessionSelectionMeta) error {
	if meta == nil || meta.UnknownCount == nil || *meta.UnknownCount == 0 || meta.EffectiveScope == sessionorigin.ScopeAuxiliary {
		return nil
	}
	_, err := fmt.Fprintf(w, "UNKNOWN sessions stay in Work. Check: agentdeck session classify <session-id> --client <client>\n")
	return err
}

func renderScopedSessionList(w io.Writer, page sessionScopedListPage) error {
	if err := renderSessionScopeLine(w, page.hint.meta, "sessions", page.Pagination["sessions"].Total); err != nil {
		return err
	}
	if len(page.Sessions) == 0 {
		if err := renderSessionListEmpty(w, page.hint); err != nil {
			return err
		}
	} else {
		rows := make([][]string, 0, len(page.Sessions))
		for _, value := range page.Sessions {
			class := "UNAVAILABLE"
			if value.Origin != nil {
				class = sessionClassLabel(value.Origin.Effective)
			}
			rows = append(rows, []string{value.Client, value.SessionID, class, value.Project, value.Model, renderDisplayTime(value.FirstAt), renderDisplayTime(value.LastAt)})
		}
		zone := displayZoneName()
		if err := output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "CLASS", "PROJECT", "MODEL", fmt.Sprintf("FIRST (%s)", zone), fmt.Sprintf("LAST (%s)", zone)}, rows); err != nil {
			return err
		}
		if err := renderUnknownHint(w, page.hint.meta); err != nil {
			return err
		}
	}
	return renderPagination(w, page.Pagination["sessions"], page.NextCommand)
}

func renderScopedSessionSearch(w io.Writer, page sessionScopedSearchPage) error {
	if meta := page.hint.meta; meta != nil {
		line := "Scope: " + meta.EffectiveScope + fmt.Sprintf(" · %d matching documents", page.Pagination["search"].Total)
		if meta.SelectedSessionCount != nil {
			line += fmt.Sprintf(" in %d sessions", *meta.SelectedSessionCount)
		}
		if meta.Availability != selectionAvailable {
			line += " · classification unavailable"
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if len(page.Documents) == 0 {
		if _, err := fmt.Fprintln(w, "No session documents."); err != nil {
			return err
		}
	} else {
		rows := make([][]string, 0, len(page.Documents))
		for _, value := range page.Documents {
			class := "UNAVAILABLE"
			if value.Origin != nil {
				class = sessionClassLabel(value.Origin.Effective)
			}
			rows = append(rows, []string{value.Client, value.SessionID, class, renderSessionDocumentTime(value.EventAt), value.Kind, oneLine(value.Text)})
		}
		zone := displayZoneName()
		if err := output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "CLASS", fmt.Sprintf("EVENT AT (%s)", zone), "KIND", "TEXT"}, rows); err != nil {
			return err
		}
	}
	return renderPagination(w, page.Pagination["search"], page.NextCommand)
}

// originCommandError is a typed failure of the classify/scope/origin-source
// contract with its stable code, exit status and safe details.
type originCommandError struct {
	code    string
	message string
	next    string
	details any
	exit    int
	err     error
}

func (e *originCommandError) Error() string {
	text := e.code + ": " + e.message
	if e.next != "" {
		text += "\n  next: " + e.next
	}
	return text
}

func (e *originCommandError) Unwrap() error { return e.err }

func originInputError(message string) error {
	return &inputError{err: errors.New(message)}
}

// originServiceError maps the service's typed errors onto the CLI contract.
func originServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sessionorigin.ErrInvalidArgument) {
		return &inputError{err: err}
	}
	var typed *sessionorigin.Error
	if !errors.As(err, &typed) {
		return err
	}
	details := map[string]any{}
	if typed.RequestID != "" {
		details["request_id"] = typed.RequestID
	}
	if typed.CurrentRevision != nil {
		details["current_revision"] = *typed.CurrentRevision
	}
	result := &originCommandError{code: typed.Code, details: details, exit: 1, err: err}
	switch typed.Code {
	case sessionorigin.CodeConflict:
		result.message = "The confirmed revision changed; read the latest revision and choose again."
	case sessionorigin.CodeWriteFailed:
		result.message = "Classification was not saved."
	case sessionorigin.CodeWriteUnknown:
		result.message = "Save status unknown; read the request receipt before retrying."
		result.next = "agentdeck session classify --request " + typed.RequestID
	case sessionorigin.CodeIdentityUnavailable:
		result.message = "The session identity is not confirmed; no override was written."
	case sessionorigin.CodeInvalidRequest:
		result.message = "This request ID was already used with a different payload."
		result.exit = 2
	case sessionorigin.CodeReadFailed:
		result.code = "runtime_error"
		result.message = "Session origin state could not be read."
	default:
		result.message = "Session origin operation failed."
	}
	if len(details) == 0 {
		result.details = nil
	}
	return result
}

func originErrorFields(err error) (code, message string, details any, exit int, ok bool) {
	var typed *originCommandError
	if !errors.As(err, &typed) {
		return "", "", nil, 0, false
	}
	return typed.code, typed.message, typed.details, typed.exit, true
}

func writeOriginEnvelope(opts *commandOptions, command string, data any, partial bool, warnings []string, text func(io.Writer) error) error {
	if opts.format == "json" {
		if warnings == nil {
			warnings = []string{}
		}
		envelope := output.New(command, data, time.Now())
		envelope.Partial, envelope.Warnings = partial, warnings
		return json.NewEncoder(opts.stdout).Encode(envelope)
	}
	if opts.format != "text" {
		return &inputError{err: fmt.Errorf("%s format is not supported by %s", opts.format, command)}
	}
	if opts.quiet {
		return nil
	}
	return text(opts.stdout)
}

func optionalRevision(command *cobra.Command, value int64) *int64 {
	if !command.Flags().Changed("expected-revision") {
		return nil
	}
	return &value
}

// classification query, set and clear --------------------------------------

type classificationQuery struct {
	Target                  sessionorigin.Target  `json:"target"`
	ClassificationAvailable bool                  `json:"classification_available"`
	Origin                  *sessionorigin.Origin `json:"origin"`
}

type classificationSetResult struct {
	sessionorigin.Receipt
	PublicationPending bool `json:"publication_pending"`
}

type classificationClearResult struct {
	sessionorigin.ClearResult
	PublicationPending bool `json:"publication_pending"`
}

type classificationReadBack struct {
	RequestID string `json:"request_id"`
	Outcome   string `json:"outcome"`
}

const sessionClassifyRequestHelp = "agentdeck session classify --request <request-id>"

// locateClassifiedSession finds one visible canonical session. Without a
// client the ID must be unique across clients; an ambiguous ID writes nothing.
func locateClassifiedSession(ctx context.Context, opts *commandOptions, reader *sessionSelectionReader, client, sessionID string) (sessionorigin.SelectedSession, error) {
	snapshot, err := reader.selectSessions(ctx, sessionorigin.Selection{Scope: sessionorigin.ScopeAll, Client: client}, nil)
	if err != nil {
		return sessionorigin.SelectedSession{}, &errSessionSelectionUnavailable{scope: "classification", err: err}
	}
	var found []sessionorigin.SelectedSession
	for _, value := range snapshot.Sessions {
		if value.SessionID == sessionID {
			found = append(found, value)
		}
	}
	switch len(found) {
	case 0:
		return sessionorigin.SelectedSession{}, sessionShowNotFound(ctx, opts, client, sessionID)
	case 1:
		return found[0], nil
	}
	return sessionorigin.SelectedSession{}, originInputError("session id is ambiguous; use --client")
}

func newSessionClassifyCommand(opts *commandOptions) *cobra.Command {
	var client, id, set, requestID, receiptID string
	var clear bool
	var expected int64
	cmd := &cobra.Command{
		Use:   "classify [session-id]",
		Short: "Show or correct one session's work/auxiliary classification",
		Long: `Show or correct one session's work/auxiliary classification.

Arguments:
  session-id  Exact session id. Use --id for an id equal to a subcommand name.

A query is read only. --set saves one reversible override; --clear withdraws
it by client and session id without reading the session index. --request
reads an earlier operation's receipt and never writes.`,
		Example: `  agentdeck session classify 019a-example --client codex
  agentdeck session classify 019a-example --client codex --set work
  agentdeck session classify --id list --client claude --clear
  agentdeck session classify --request req-0123`,
		Args: rangeArgs(0, 1),
	}
	cmd.RunE = func(command *cobra.Command, args []string) error {
		if len(args) == 1 && id != "" {
			return originInputError("use either a positional session id or --id, not both")
		}
		if len(args) == 1 {
			id = args[0]
		}
		if receiptID != "" {
			if id != "" || set != "" || clear || command.Flags().Changed("client") || command.Flags().Changed("expected-revision") || requestID != "" {
				return originInputError("--request cannot be combined with a session, --client, --set, --clear, --expected-revision or --request-id")
			}
			return runClassifyReadBack(command.Context(), opts, receiptID)
		}
		if id == "" {
			return originInputError("a session id is required; use session classify --id <id> --client <client>, session classify list, or --request")
		}
		if set != "" && clear {
			return originInputError("--set and --clear are mutually exclusive")
		}
		if (command.Flags().Changed("expected-revision") || requestID != "") && set == "" && !clear {
			return originInputError("--expected-revision and --request-id apply only to --set or --clear")
		}
		if client != "" && client != "codex" && client != "claude" {
			return originInputError("client must be codex or claude")
		}
		if set != "" && set != sessionorigin.ClassWork && set != sessionorigin.ClassAuxiliary {
			return originInputError("--set must be work or auxiliary")
		}
		if requestID != "" && !sessionorigin.ValidRequestID(requestID) {
			return originInputError("--request-id must be 1-128 letters, digits, '.', '_', ':' or '-'")
		}
		if command.Flags().Changed("expected-revision") && expected < 0 {
			return originInputError("--expected-revision must be zero or positive")
		}
		if clear {
			if client == "" {
				return originInputError("--clear requires --client")
			}
			return runClassifyClear(command.Context(), opts, client, id, requestID, optionalRevision(command, expected))
		}
		return runClassifyQueryOrSet(command.Context(), opts, client, id, set, requestID, optionalRevision(command, expected))
	}
	cmd.Flags().StringVar(&client, "client", "", "Session client: codex or claude")
	cmd.Flags().StringVar(&id, "id", "", "Session id, for ids that equal a reserved subcommand such as list")
	cmd.Flags().StringVar(&set, "set", "", "Set an override: work or auxiliary")
	cmd.Flags().BoolVar(&clear, "clear", false, "Withdraw this session's override (requires --client)")
	cmd.Flags().Int64Var(&expected, "expected-revision", 0, "Compare-and-set against this confirmed revision")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency key; reuse it only to retry the same operation")
	cmd.Flags().StringVar(&receiptID, "request", "", "Read back the receipt of an earlier request without writing")
	var page, limit int
	list := &cobra.Command{Use: "list", Short: "List effective overrides, unmatched ones included", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		return runClassifyList(command.Context(), opts, page, limit)
	}}
	list.Flags().IntVar(&page, "page", 1, "Page number")
	list.Flags().IntVar(&limit, "limit", sessionorigin.DefaultOverrideLimit, "Rows per page (maximum 200)")
	cmd.AddCommand(list)
	return cmd
}

func runClassifyQueryOrSet(ctx context.Context, opts *commandOptions, client, sessionID, set, requestID string, expected *int64) error {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return err
	}
	sessions, err := store.OpenSessionsReadOnly(ctx, stateRoot)
	if err != nil {
		return &errSessionSelectionUnavailable{scope: "classification", err: fmt.Errorf("%w: %v", sessionorigin.ErrSessionsUnavailable, err)}
	}
	defer sessions.Close()
	reader := openSessionSelectionReader(ctx, stateRoot, sessions)
	defer reader.Close()
	target, err := locateClassifiedSession(ctx, opts, reader, client, sessionID)
	if err != nil {
		return err
	}
	if set == "" {
		origin := target.Origin
		data := classificationQuery{Target: sessionorigin.Target{Client: target.Client, SessionID: target.SessionID}, ClassificationAvailable: true, Origin: &origin}
		return writeOriginEnvelope(opts, "session.classify", data, false, nil, func(w io.Writer) error { return renderClassificationQuery(w, data) })
	}
	binding, err := sessionorigin.ResolveBinding(ctx, sessions, target.Client, target.SessionID)
	if err != nil {
		return &originCommandError{code: sessionorigin.CodeIdentityUnavailable, message: "The session identity is not confirmed; no override was written.", exit: 1, err: err}
	}
	reader.Close()
	core, _, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	service := sessionorigin.Service{Store: core}
	request := sessionorigin.OverrideSetRequest{RequestID: requestID, Client: target.Client, SessionID: target.SessionID, Value: set, Binding: binding}
	if expected != nil {
		request.ExpectedRevision = *expected
	} else {
		revision, revisionErr := service.OverrideRevision(ctx, target.Client, target.SessionID)
		if revisionErr != nil {
			return originServiceError(revisionErr)
		}
		request.ExpectedRevision, request.ImplicitExpected = revision, true
	}
	receipt, err := service.SetOverride(ctx, request)
	if err != nil {
		return originServiceError(err)
	}
	data := classificationSetResult{Receipt: receipt, PublicationPending: receipt.Outcome == sessionorigin.OutcomeSaved}
	return writeOriginEnvelope(opts, "session.classify", data, false, receipt.Warnings, func(w io.Writer) error {
		return renderClassificationReceipt(w, data.Receipt, data.PublicationPending, false, data.Receipt.Replayed)
	})
}

func runClassifyClear(ctx context.Context, opts *commandOptions, client, sessionID, requestID string, expected *int64) error {
	core, _, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	result, err := (sessionorigin.Service{Store: core}).ClearOverride(ctx, sessionorigin.OverrideClearRequest{RequestID: requestID, Client: client, SessionID: sessionID, ExpectedRevision: expected})
	if err != nil {
		return originServiceError(err)
	}
	data := classificationClearResult{ClearResult: result, PublicationPending: result.Outcome == sessionorigin.OutcomeSaved}
	return writeOriginEnvelope(opts, "session.classify", data, false, result.Warnings, func(w io.Writer) error {
		return renderClassificationReceipt(w, data.Receipt, data.PublicationPending, true, data.Receipt.Replayed)
	})
}

func openOriginCoreReadOnly(ctx context.Context, opts *commandOptions) (*store.Store, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return nil, err
	}
	core, err := store.OpenReadOnly(ctx, stateRoot)
	if err != nil {
		return nil, &originCommandError{code: "runtime_error", message: "Session origin state could not be read.", exit: 1, err: err}
	}
	return core, nil
}

func runClassifyReadBack(ctx context.Context, opts *commandOptions, requestID string) error {
	if !sessionorigin.ValidRequestID(requestID) {
		return originInputError("--request must be 1-128 letters, digits, '.', '_', ':' or '-'")
	}
	core, err := openOriginCoreReadOnly(ctx, opts)
	if err != nil {
		return err
	}
	defer core.Close()
	read, err := (sessionorigin.Service{Store: core}).ReadReceipt(ctx, requestID)
	if err != nil {
		return originServiceError(err)
	}
	if !read.Known {
		data := classificationReadBack{RequestID: requestID, Outcome: sessionorigin.OutcomeUnknown}
		return writeOriginEnvelope(opts, "session.classify", data, false, nil, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "SAVE STATUS UNKNOWN\nRequest ID  %s\nOutcome     unknown\nNo committed result is recorded for this request. Do not assume it was saved.\n", safeCell(requestID))
			return err
		})
	}
	receipt := read.Receipt
	switch receipt.Kind {
	case store.OriginMutationOverrideClear:
		data := classificationClearResult{ClearResult: sessionorigin.ClearReadBack(receipt)}
		return writeOriginEnvelope(opts, "session.classify", data, false, nil, func(w io.Writer) error { return renderClassificationReceipt(w, data.Receipt, false, true, true) })
	case store.OriginMutationScopeSet:
		return writeOriginEnvelope(opts, "session.classify", receipt, false, nil, func(w io.Writer) error { return renderScopeReceipt(w, receipt, false, true) })
	}
	data := classificationSetResult{Receipt: receipt}
	return writeOriginEnvelope(opts, "session.classify", data, false, nil, func(w io.Writer) error { return renderClassificationReceipt(w, data.Receipt, false, false, true) })
}

func runClassifyList(ctx context.Context, opts *commandOptions, page, limit int) error {
	if limit < 1 || limit > sessionorigin.MaxOverrideLimit || page < 1 {
		return originInputError("page must be positive and limit must be between 1 and 200")
	}
	core, err := openOriginCoreReadOnly(ctx, opts)
	if err != nil {
		return err
	}
	defer core.Close()
	result, err := (sessionorigin.Service{Store: core}).ListOverrides(ctx, page, limit)
	if err != nil {
		return originServiceError(err)
	}
	return writeOriginEnvelope(opts, "session.classify.list", result, false, nil, func(w io.Writer) error { return renderOverrideList(w, result) })
}

func safeCell(value string) string { return output.SanitizeTerminalCell(value) }

func unavailableText(value string) string {
	if value == "" {
		return "unavailable"
	}
	return value
}

func renderClassificationQuery(w io.Writer, data classificationQuery) error {
	origin := data.Origin
	override := "none"
	if origin.Override != nil {
		override = origin.Override.Value + " (" + origin.Override.BindingStatus
		if origin.Override.BindingLimited {
			override += ", binding limited"
		}
		if !origin.Override.Applied {
			override += ", not applied"
		}
		override += ")"
	}
	lines := [][2]string{
		{"Client", safeCell(data.Target.Client)},
		{"Session", safeCell(data.Target.SessionID)},
		{"Effective", sessionClassLabel(origin.Effective)},
		{"Automatic", sessionClassLabel(origin.Automatic)},
		{"Reason", unavailableText(origin.ReasonCode) + " — " + classificationReasonText(origin.ReasonCode)},
		{"Rule", unavailableText(origin.RuleVersion)},
		{"Override", override},
		{"Revision", strconv.FormatInt(origin.Revision, 10)},
	}
	if _, err := fmt.Fprintln(w, "SESSION CLASSIFICATION · READ ONLY"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%-10s %s\n", line[0], line[1]); err != nil {
			return err
		}
	}
	id, idOK := terminalSafeShellArg(data.Target.SessionID)
	client, clientOK := terminalSafeShellArg(data.Target.Client)
	if !idOK || !clientOK {
		_, err := fmt.Fprintln(w, uncopyableCommandText)
		return err
	}
	_, err := fmt.Fprintf(w, "Change: agentdeck session classify --id %s --client %s --set work|auxiliary\n", id, client)
	return err
}

// uncopyableCommandText replaces a copyable command whose identity cannot be
// written as one shell argument; printing a sanitized ID instead would name
// a different session.
const uncopyableCommandText = "Command unavailable: this session ID cannot be written as a shell argument. Read the exact target with --format json."

// terminalSafeShellArg quotes value as one shell argument whose printed form
// contains no terminal control characters. Plain values use single quotes.
// A value with C0/C1 controls, DEL or invalid UTF-8 uses ANSI-C quoting
// ($'...', bash and zsh) with \xHH escapes, which restores the exact bytes.
// A NUL byte cannot be passed in argv at all, so ok is false.
func terminalSafeShellArg(value string) (quoted string, ok bool) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", false
	}
	if !needsEscapedShellArg(value) {
		return shellQuote(value), true
	}
	var b strings.Builder
	b.WriteString("$'")
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		switch {
		case r == utf8.RuneError && size == 1, isTerminalControl(r):
			for _, octet := range []byte(value[index : index+size]) {
				fmt.Fprintf(&b, "\\x%02x", octet)
			}
		case r == '\\' || r == '\'':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteString(value[index : index+size])
		}
		index += size
	}
	b.WriteByte('\'')
	return b.String(), true
}

func needsEscapedShellArg(value string) bool {
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		if (r == utf8.RuneError && size == 1) || isTerminalControl(r) {
			return true
		}
		index += size
	}
	return false
}

func isTerminalControl(r rune) bool { return r < 0x20 || r >= 0x7f && r <= 0x9f }

func classificationReasonText(code string) string {
	switch code {
	case sessionorigin.ReasonMaintenanceOrigin:
		return "a supported maintenance rule matched this session"
	case sessionorigin.ReasonOriginMetadataMissing:
		return "no origin evidence is available"
	case sessionorigin.ReasonTaskChildUnconfirmed:
		return "a task subagent without a verifiable purpose"
	case sessionorigin.ReasonConflictingOrigin:
		return "maintenance and work evidence conflict or the purpose is mixed"
	case sessionorigin.ReasonOriginSourceUnavailable:
		return "the origin source could not be read or its version is unknown"
	case sessionorigin.ReasonOriginRuleUnsupported:
		return "the source entry is outside the supported rule"
	case sessionorigin.ReasonOriginInputMismatch:
		return "the evidence belongs to an older version of this session's sources"
	}
	return "reason unavailable"
}

func receiptValueText(value *string) string {
	if value == nil {
		return "none"
	}
	return *value
}

func renderClassificationReceipt(w io.Writer, receipt sessionorigin.Receipt, publicationPending, clear bool, historical ...bool) error {
	title := "CLASSIFICATION SAVED"
	if receipt.Outcome == sessionorigin.OutcomeNoChange {
		title = "NO CHANGE"
	} else if clear {
		title = "CLASSIFICATION CLEARED"
	}
	if len(historical) > 0 && historical[0] {
		title += " · RECORDED RECEIPT"
	}
	target := ""
	client, id := "", ""
	if receipt.Target != nil {
		client, id = receipt.Target.Client, receipt.Target.SessionID
		target = safeCell(client) + "/" + safeCell(id)
	}
	lines := []string{title,
		"Request ID  " + safeCell(receipt.RequestID),
		"Target      " + target,
		"Outcome     " + receipt.Outcome,
		"Revision    " + strconv.FormatInt(receipt.Revision, 10),
	}
	if clear {
		lines = append(lines, "Override    none", "Automatic   unavailable (not queried)", "Effective   unavailable (not queried)")
	} else {
		lines = append(lines, "Override    "+receiptValueText(receipt.Value))
	}
	if len(historical) > 0 && historical[0] {
		lines = append(lines, "This receipt describes that operation, not necessarily the current state.", "Current overrides: agentdeck session classify list")
	} else if publicationPending {
		lines = append(lines, "Views: pending publication")
	}
	for _, warning := range receipt.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	// A recorded receipt's undo would clear whatever override is current now,
	// not the recorded one, so only a newly committed save offers it.
	if !clear && receipt.Outcome == sessionorigin.OutcomeSaved && client != "" && !(len(historical) > 0 && historical[0]) {
		quotedID, idOK := terminalSafeShellArg(id)
		quotedClient, clientOK := terminalSafeShellArg(client)
		if idOK && clientOK {
			lines = append(lines, "Undo: agentdeck session classify --id "+quotedID+" --client "+quotedClient+" --clear")
		} else {
			lines = append(lines, uncopyableCommandText)
		}
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func renderOverrideList(w io.Writer, page sessionorigin.OverridePage) error {
	if len(page.Items) == 0 {
		if _, err := fmt.Fprintln(w, "No active overrides."); err != nil {
			return err
		}
	} else {
		rows := make([][]string, 0, len(page.Items))
		for _, item := range page.Items {
			binding := item.BindingStatus
			if item.BindingLimited {
				binding += " (limited)"
			}
			rows = append(rows, []string{item.Client, item.SessionID, item.Value, binding, strconv.FormatInt(item.Revision, 10)})
		}
		if err := output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "VALUE", "BINDING", "REVISION"}, rows); err != nil {
			return err
		}
	}
	first, last := 0, 0
	if len(page.Items) > 0 {
		first = (page.Page-1)*page.Limit + 1
		last = first + len(page.Items) - 1
	}
	if _, err := fmt.Fprintf(w, "Showing %d-%d of %d\n", first, last, page.Total); err != nil {
		return err
	}
	if page.Page*page.Limit < page.Total {
		_, err := fmt.Fprintf(w, "Next page: agentdeck session classify list --page %d --limit %d\n", page.Page+1, page.Limit)
		return err
	}
	return nil
}

// default scope --------------------------------------------------------------

func newSessionScopeCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "scope", Short: "Read or save the desktop and widget default session scope"}
	get := &cobra.Command{Use: "get", Short: "Read the saved default session scope", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		return runScopeGet(command.Context(), opts)
	}}
	var expected int64
	var requestID string
	set := &cobra.Command{Use: "set <work|auxiliary|all>", Short: "Save the default session scope",
		Long:    "Save the desktop and widget default session scope.\n\nArguments:\n  work|auxiliary|all  The scope to save; the CLI text default stays work.",
		Example: "  agentdeck session scope set all\n  agentdeck session scope set work --expected-revision 3", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
			value := args[0]
			if value != sessionorigin.ScopeWork && value != sessionorigin.ScopeAuxiliary && value != sessionorigin.ScopeAll {
				return originInputError("session scope must be work, auxiliary, or all")
			}
			if requestID != "" && !sessionorigin.ValidRequestID(requestID) {
				return originInputError("--request-id must be 1-128 letters, digits, '.', '_', ':' or '-'")
			}
			if command.Flags().Changed("expected-revision") && expected < 0 {
				return originInputError("--expected-revision must be zero or positive")
			}
			return runScopeSet(command.Context(), opts, value, requestID, optionalRevision(command, expected))
		}}
	set.Flags().Int64Var(&expected, "expected-revision", 0, "Compare-and-set against this confirmed revision")
	set.Flags().StringVar(&requestID, "request-id", "", "Idempotency key; reuse it only to retry the same operation")
	cmd.AddCommand(get, set)
	return cmd
}

func scopeReadFailure(err error) error {
	reason := sessionorigin.CodeReadFailed
	return &originCommandError{code: "runtime_error", message: "The default session scope could not be read.", exit: 1, err: err,
		details: map[string]any{"default_scope": sessionorigin.DefaultScope{Reason: &reason}}}
}

func runScopeGet(ctx context.Context, opts *commandOptions) error {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return err
	}
	core, err := store.OpenReadOnly(ctx, stateRoot)
	if err != nil {
		return scopeReadFailure(err)
	}
	defer core.Close()
	scope := (sessionorigin.Service{Store: core}).DefaultScope(ctx)
	if !scope.Confirmed {
		return scopeReadFailure(errors.New("default scope read failed"))
	}
	return writeOriginEnvelope(opts, "session.scope.get", scope, false, nil, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Default scope  %s\nRevision       %d\nApplies to the desktop menu bar and widget; the CLI text default stays work.\n", *scope.Value, *scope.Revision)
		return err
	})
}

func runScopeSet(ctx context.Context, opts *commandOptions, value, requestID string, expected *int64) error {
	core, _, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	service := sessionorigin.Service{Store: core}
	request := sessionorigin.ScopeRequest{RequestID: requestID, Value: value}
	if expected != nil {
		request.ExpectedRevision = *expected
	} else {
		current := service.DefaultScope(ctx)
		if !current.Confirmed {
			return scopeReadFailure(errors.New("default scope read failed"))
		}
		request.ExpectedRevision, request.ImplicitExpected = *current.Revision, true
	}
	receipt, err := service.SetDefaultScope(ctx, request)
	if err != nil {
		return originServiceError(err)
	}
	data := classificationSetResult{Receipt: receipt, PublicationPending: receipt.Outcome == sessionorigin.OutcomeSaved}
	return writeOriginEnvelope(opts, "session.scope.set", data, false, receipt.Warnings, func(w io.Writer) error {
		return renderScopeReceipt(w, receipt, data.PublicationPending, receipt.Replayed)
	})
}

func renderScopeReceipt(w io.Writer, receipt sessionorigin.Receipt, publicationPending, historical bool) error {
	title := "DEFAULT SCOPE SAVED"
	if receipt.Outcome == sessionorigin.OutcomeNoChange {
		title = "NO CHANGE"
	}
	if historical {
		title += " · RECORDED RECEIPT"
	}
	lines := []string{title, "Request ID  " + safeCell(receipt.RequestID), "Outcome     " + receipt.Outcome,
		"Scope       " + receiptValueText(receipt.Value), "Revision    " + strconv.FormatInt(receipt.Revision, 10)}
	if historical {
		lines = append(lines, "This receipt describes that operation, not necessarily the current state.", "Current default scope: agentdeck session scope get")
	} else if publicationPending {
		lines = append(lines, "Views: pending publication")
	}
	for _, warning := range receipt.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// origin sources -------------------------------------------------------------

type originSourceResult struct {
	Source              string `json:"source"`
	Configured          bool   `json:"configured"`
	Enabled             bool   `json:"enabled"`
	MappingCoverage     string `json:"mapping_coverage"`
	EvidenceInvalidated *bool  `json:"evidence_invalidated,omitempty"`
}

func newSessionOriginSourceCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "origin-source", Short: "Configure or inspect maintenance origin sources"}
	var dataDir string
	set := &cobra.Command{Use: "set <source>", Short: "Configure an external origin source's metadata root",
		Long:    "Configure an external origin source's metadata root.\n\nArguments:\n  source  claude-mem (the built-in codex-otel-background source has no settings).",
		Example: "  agentdeck session origin-source set claude-mem --data-dir /absolute/metadata/root", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
			if err := originSourceArgument(args[0]); err != nil {
				return err
			}
			if dataDir == "" {
				return originInputError("--data-dir is required")
			}
			if !filepath.IsAbs(dataDir) || filepath.Clean(dataDir) != dataDir {
				return originInputError("--data-dir must be a clean absolute path")
			}
			return runOriginSourceSet(command.Context(), opts, dataDir)
		}}
	set.Flags().StringVar(&dataDir, "data-dir", "", "Absolute metadata root of the source")
	disable := &cobra.Command{Use: "disable <source>", Short: "Disable an external origin source and invalidate its evidence",
		Long:    "Disable an external origin source and invalidate its automatic evidence.\n\nArguments:\n  source  claude-mem. Overrides, source logs and usage are kept.",
		Example: "  agentdeck session origin-source disable claude-mem", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
			if err := originSourceArgument(args[0]); err != nil {
				return err
			}
			return runOriginSourceDisable(command.Context(), opts)
		}}
	status := &cobra.Command{Use: "status", Short: "Show origin source status without scanning", Args: exactArgs(0), RunE: func(command *cobra.Command, _ []string) error {
		return runOriginSourceStatus(command.Context(), opts)
	}}
	cmd.AddCommand(set, disable, status)
	return cmd
}

// originSourceArgument accepts only configurable adapters. The built-in
// Codex background source reads AgentDeck's own core and has no switch.
func originSourceArgument(source string) error {
	switch source {
	case sessionorigin.SourceClaudeMem:
		return nil
	case sessionorigin.SourceCodexOTelBackground:
		return originInputError("codex-otel-background is built in and cannot be configured; read it with agentdeck session origin-source status")
	}
	return originInputError("origin source must be claude-mem")
}

func withOriginSourceLock(ctx context.Context, opts *commandOptions, run func(stateRoot string) error) error {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return err
	}
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return err
	}
	lock, err := store.AcquireLock(ctx, stateRoot, 5*time.Second)
	if err != nil {
		return err
	}
	defer lock.Release()
	return run(stateRoot)
}

func runOriginSourceSet(ctx context.Context, opts *commandOptions, dataDir string) error {
	err := withOriginSourceLock(ctx, opts, func(stateRoot string) error {
		config, err := sessionorigin.LoadSourceConfig(stateRoot)
		if err != nil {
			return &originCommandError{code: "runtime_error", message: "The origin source configuration could not be read.", exit: 1, err: err}
		}
		config.ClaudeMem = sessionorigin.AdapterConfig{DataDir: dataDir}
		if err := sessionorigin.SaveSourceConfig(stateRoot, config); err != nil {
			return &originCommandError{code: "runtime_error", message: "The origin source configuration was not saved.", exit: 1, err: err}
		}
		return nil
	})
	if err != nil {
		return err
	}
	data := originSourceResult{Source: sessionorigin.SourceClaudeMem, Configured: true, Enabled: true, MappingCoverage: "not_verified"}
	return writeOriginEnvelope(opts, "session.origin_source.set", data, false, nil, func(w io.Writer) error {
		_, err := fmt.Fprintln(w, "ORIGIN SOURCE CONFIGURED\nSource    claude-mem\nEnabled   yes\nMapping   not verified; the next session scan reads it")
		return err
	})
}

// runOriginSourceDisable disables the adapter and re-derives classification
// from committed core facts only: with the adapter disabled no external
// source is opened and no log is rescanned. Overrides, sources and usage stay.
func runOriginSourceDisable(ctx context.Context, opts *commandOptions) error {
	var warnings []string
	invalidated, configured := false, false
	err := withOriginSourceLock(ctx, opts, func(stateRoot string) error {
		config, err := sessionorigin.LoadSourceConfig(stateRoot)
		if err != nil {
			return &originCommandError{code: "runtime_error", message: "The origin source configuration could not be read.", exit: 1, err: err}
		}
		config.ClaudeMem.Disabled = true
		configured = config.ClaudeMem.Configured()
		if err := sessionorigin.SaveSourceConfig(stateRoot, config); err != nil {
			return &originCommandError{code: "runtime_error", message: "The origin source configuration was not saved.", exit: 1, err: err}
		}
		if invalidateErr := reconcileDisabledOriginSource(ctx, stateRoot); invalidateErr != nil {
			warnings = append(warnings, "evidence_invalidation_pending")
			return nil
		}
		invalidated = true
		return nil
	})
	if err != nil {
		return err
	}
	data := originSourceResult{Source: sessionorigin.SourceClaudeMem, Configured: configured, Enabled: false, MappingCoverage: "unavailable", EvidenceInvalidated: &invalidated}
	return writeOriginEnvelope(opts, "session.origin_source.disable", data, len(warnings) > 0, warnings, func(w io.Writer) error {
		text := "ORIGIN SOURCE DISABLED\nSource    claude-mem\nEnabled   no\nEvidence  invalidated; affected sessions are unknown and stay in Work"
		if !invalidated {
			text = "ORIGIN SOURCE DISABLED\nSource    claude-mem\nEnabled   no\nEvidence  invalidation pending; the next session scan applies it"
		}
		_, err := fmt.Fprintln(w, text+"\nOverrides, source logs and usage are unchanged.")
		return err
	})
}

func reconcileDisabledOriginSource(ctx context.Context, stateRoot string) error {
	sessions, err := store.OpenSessionsReadOnly(ctx, stateRoot)
	if err != nil {
		return err
	}
	defer sessions.Close()
	// The caller holds the state lock for the configuration change.
	core, err := store.OpenWithLockHeld(ctx, stateRoot)
	if err != nil {
		return err
	}
	defer core.Close()
	_, err = sessionorigin.Reconcile(ctx, core, sessions, sessionorigin.ReconcileOptions{StateRoot: stateRoot})
	return err
}

// originSourceStatus is one session.origin_source.status item. Its field
// whitelist is fixed by architecture §7: kind-specific fields are omitted on
// the other kind, and nullable fields are always present on their own kind.
type originSourceStatus struct {
	Source             string  `json:"source"`
	Kind               string  `json:"kind"`
	Configured         *bool   `json:"configured"`
	Enabled            *bool   `json:"enabled"`
	Available          bool    `json:"available"`
	SupportedVersion   *string `json:"supported_version"`
	RuleVersion        string  `json:"rule_version"`
	ReasonCode         *string `json:"reason_code"`
	LastRunAt          *string `json:"-"`
	Receiver           string  `json:"-"`
	BackgroundSessions *int    `json:"-"`
}

func (s originSourceStatus) MarshalJSON() ([]byte, error) {
	fields := map[string]any{
		"source": s.Source, "kind": s.Kind, "configured": s.Configured, "enabled": s.Enabled,
		"available": s.Available, "supported_version": s.SupportedVersion, "rule_version": s.RuleVersion, "reason_code": s.ReasonCode,
	}
	if s.Kind == "adapter" {
		fields["last_run_at"] = s.LastRunAt
	} else {
		fields["receiver"] = s.Receiver
		fields["background_sessions"] = s.BackgroundSessions
	}
	return json.Marshal(fields)
}

type originSourceStatusData struct {
	Sources []originSourceStatus `json:"sources"`
}

// Origin-source status reason codes (architecture §7), distinct from the
// classification reason codes.
const (
	sourceReasonNotConfigured      = "source_not_configured"
	sourceReasonDisabled           = "source_disabled"
	sourceReasonNotYetRead         = "source_not_yet_read"
	sourceReasonUnavailable        = "source_unavailable"
	sourceReasonVersionUnsupported = "source_version_unsupported"
	sourceReasonConfigUnreadable   = "source_config_unreadable"
	sourceReasonCoreUnavailable    = "core_unavailable"
	sourceReasonReceiverDisabled   = "otel_receiver_disabled"
	sourceReasonNoBackground       = "no_background_sessions"
	sourceReasonReceiverUnknown    = "otel_receiver_unknown"
)

// originStatusInputs are the three independent reads of the status command;
// each is a confirmed value or a read failure.
type originStatusInputs struct {
	config     sessionorigin.SourceConfig
	configErr  error
	coreErr    error
	run        store.OriginAdapterRun
	runFound   bool
	background int
	receiver   string
}

func readOriginStatusInputs(ctx context.Context, stateRoot string) originStatusInputs {
	var in originStatusInputs
	in.config, in.configErr = sessionorigin.LoadSourceConfig(stateRoot)
	in.receiver = "unknown"
	if config, err := telemetry.LoadConfig(stateRoot); err == nil {
		in.receiver = map[bool]string{true: "enabled", false: "disabled"}[config.Enabled]
	}
	core, err := store.OpenReadOnly(ctx, stateRoot)
	if err != nil {
		in.coreErr = err
		return in
	}
	defer core.Close()
	if in.run, in.runFound, err = core.OriginAdapterRunFor(ctx, sessionorigin.SourceClaudeMem); err != nil {
		in.coreErr = err
		return in
	}
	if in.background, err = core.TelemetryBackgroundSessionCount(ctx); err != nil {
		in.coreErr = err
	}
	return in
}

// originSourceStatuses applies architecture §7's decision order. partial is
// true exactly for the three read failures: core, source configuration and
// receiver.json.
func originSourceStatuses(in originStatusInputs) ([]originSourceStatus, bool) {
	reason := func(code string) *string { return &code }
	boolean := func(value bool) *bool { return &value }
	claude := originSourceStatus{Source: sessionorigin.SourceClaudeMem, Kind: "adapter", RuleVersion: sessionorigin.RuleClaudeMemSDKMapV1}
	if in.coreErr == nil && in.runFound {
		claude.SupportedVersion = in.run.ValidatedVersion
		finished := in.run.FinishedAt
		claude.LastRunAt = &finished
	}
	switch {
	case in.configErr != nil:
		claude.ReasonCode = reason(sourceReasonConfigUnreadable)
	case !in.config.ClaudeMem.Configured():
		claude.Configured, claude.Enabled = boolean(false), boolean(false)
		claude.ReasonCode = reason(sourceReasonNotConfigured)
	case !in.config.ClaudeMem.Enabled():
		claude.Configured, claude.Enabled = boolean(true), boolean(false)
		claude.ReasonCode = reason(sourceReasonDisabled)
	default:
		claude.Configured, claude.Enabled = boolean(true), boolean(true)
		switch {
		case in.coreErr != nil:
			claude.ReasonCode = reason(sourceReasonCoreUnavailable)
		case !in.runFound:
			claude.ReasonCode = reason(sourceReasonNotYetRead)
		case in.run.Outcome == "unavailable":
			claude.ReasonCode = reason(sourceReasonUnavailable)
		case in.run.Outcome == "unsupported":
			claude.ReasonCode = reason(sourceReasonVersionUnsupported)
		case in.run.Outcome == "ok" || in.run.Outcome == "partial":
			claude.Available = true
		default:
			claude.ReasonCode = reason(sourceReasonUnavailable)
		}
	}
	codex := originSourceStatus{Source: sessionorigin.SourceCodexOTelBackground, Kind: "builtin", RuleVersion: sessionorigin.RuleCodexOTelBackgroundV1, Receiver: in.receiver}
	switch {
	case in.coreErr != nil:
		codex.ReasonCode = reason(sourceReasonCoreUnavailable)
	default:
		codex.Available = true
		count := in.background
		codex.BackgroundSessions = &count
		if count == 0 {
			switch in.receiver {
			case "disabled":
				codex.ReasonCode = reason(sourceReasonReceiverDisabled)
			case "enabled":
				codex.ReasonCode = reason(sourceReasonNoBackground)
			default:
				codex.ReasonCode = reason(sourceReasonReceiverUnknown)
			}
		}
	}
	partial := in.coreErr != nil || in.configErr != nil || in.receiver == "unknown"
	return []originSourceStatus{claude, codex}, partial
}

func runOriginSourceStatus(ctx context.Context, opts *commandOptions) error {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return err
	}
	sources, partial := originSourceStatuses(readOriginStatusInputs(ctx, stateRoot))
	var warnings []string
	if partial {
		warnings = []string{"origin_source_status_incomplete"}
	}
	data := originSourceStatusData{Sources: sources}
	return writeOriginEnvelope(opts, "session.origin_source.status", data, partial, warnings, func(w io.Writer) error {
		for _, source := range sources {
			if _, err := fmt.Fprintln(w, originSourceStatusLine(source)); err != nil {
				return err
			}
		}
		return nil
	})
}

var knownSourceReasons = map[string]string{
	sourceReasonNotConfigured:      "not configured",
	sourceReasonDisabled:           "disabled",
	sourceReasonNotYetRead:         "not read yet; run agentdeck session scan",
	sourceReasonUnavailable:        "source unavailable",
	sourceReasonVersionUnsupported: "version unsupported",
	sourceReasonConfigUnreadable:   "configuration unreadable",
	sourceReasonCoreUnavailable:    "AgentDeck state unreadable",
	sourceReasonReceiverDisabled:   "OTel capture is off",
	sourceReasonNoBackground:       "no background sessions yet",
	sourceReasonReceiverUnknown:    "OTel receiver state unknown",
}

func sourceReasonText(code *string) string {
	if code == nil {
		return ""
	}
	if text, ok := knownSourceReasons[*code]; ok {
		return text + ": " + *code
	}
	return "reason unavailable"
}

func originSourceStatusLine(source originSourceStatus) string {
	parts := []string{fmt.Sprintf("%-23s %-8s", source.Source, source.Kind)}
	if source.Kind == "adapter" {
		configured := "configured unknown"
		if source.Configured != nil {
			configured = map[bool]string{true: "configured", false: "not configured"}[*source.Configured]
		}
		parts = append(parts, configured)
		if source.Enabled != nil && *source.Configured {
			parts = append(parts, map[bool]string{true: "enabled", false: "disabled"}[*source.Enabled])
		}
		parts = append(parts, map[bool]string{true: "available", false: "unavailable"}[source.Available])
		if source.SupportedVersion != nil {
			parts = append(parts, safeCell(*source.SupportedVersion))
		}
		if source.LastRunAt != nil {
			parts = append(parts, "last read "+safeCell(*source.LastRunAt))
		}
	} else {
		switch {
		case source.BackgroundSessions == nil:
			parts = append(parts, "records unavailable")
		case *source.BackgroundSessions == 0:
			parts = append(parts, "no records")
		default:
			parts = append(parts, fmt.Sprintf("%d background sessions", *source.BackgroundSessions))
		}
		parts = append(parts, "receiver "+source.Receiver)
	}
	if text := sourceReasonText(source.ReasonCode); text != "" {
		parts = append(parts, "("+text+")")
	}
	return strings.Join(parts, "  ")
}

// scoped reads ----------------------------------------------------------------

// scopedSessionList reads the selected sessions through the shared selection.
// When classification cannot be read only the all scope is still answerable:
// its membership never depends on classification, so it keeps the historical
// set without origin and reports session_selection as unavailable. Work and
// auxiliary fail instead of showing an untrusted list or zero.
func scopedSessionList(ctx context.Context, opts *commandOptions, sessions *store.Store, scope, client string, meta *sessionSelectionMeta) ([]sessionListItem, bool, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return nil, false, err
	}
	reader := openSessionSelectionReader(ctx, stateRoot, sessions)
	defer reader.Close()
	snapshot, err := reader.selectSessions(ctx, sessionorigin.Selection{Scope: scope, Client: client}, nil)
	if err == nil {
		meta.fill(snapshot)
		items := listItemsFromSnapshot(snapshot)
		return items, len(items) == 0 && sessionIndexEmpty(ctx, sessions), nil
	}
	if errors.Is(err, sessionorigin.ErrInvalidArgument) {
		return nil, false, &inputError{err: err}
	}
	if scope != sessionorigin.ScopeAll {
		return nil, false, &errSessionSelectionUnavailable{scope: scope, err: err}
	}
	meta.unavailable(err)
	all, listErr := session.List(ctx, sessions.DB)
	if listErr != nil {
		return nil, false, listErr
	}
	items := make([]sessionListItem, 0, len(all))
	for _, item := range all {
		if client == "" || item.Client == client {
			items = append(items, sessionListItem{Metadata: item})
		}
	}
	return items, len(all) == 0, nil
}

func sessionIndexEmpty(ctx context.Context, sessions *store.Store) bool {
	var exists bool
	if err := sessions.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM session_metadata)`).Scan(&exists); err != nil {
		return false
	}
	return !exists
}

func scopedSessionSearch(ctx context.Context, opts *commandOptions, sessions *store.Store, scope, client, query string, meta *sessionSelectionMeta) ([]sessionDocumentItem, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return nil, err
	}
	reader := openSessionSelectionReader(ctx, stateRoot, sessions)
	defer reader.Close()
	snapshot, err := reader.selectSessions(ctx, sessionorigin.Selection{Scope: scope, Client: client, Query: query}, nil)
	if err == nil {
		meta.fill(snapshot)
		return documentItemsFromSnapshot(snapshot), nil
	}
	if errors.Is(err, sessionorigin.ErrInvalidArgument) {
		return nil, &inputError{err: err}
	}
	if scope != sessionorigin.ScopeAll || !errors.Is(err, sessionorigin.ErrClassificationUnavailable) {
		if scope == sessionorigin.ScopeAll && errors.Is(err, sessionorigin.ErrSessionsUnavailable) {
			// An invalid FTS query surfaces here; keep the historical error.
			if _, searchErr := session.Search(ctx, sessions.DB, query); searchErr != nil {
				return nil, searchErr
			}
		}
		return nil, &errSessionSelectionUnavailable{scope: scope, err: err}
	}
	meta.unavailable(err)
	documents, searchErr := session.Search(ctx, sessions.DB, query)
	if searchErr != nil {
		return nil, searchErr
	}
	items := make([]sessionDocumentItem, 0, len(documents))
	for _, document := range documents {
		if client == "" || document.Client == client {
			items = append(items, sessionDocumentItem{Document: document})
		}
	}
	return items, nil
}

// scopedSignals reads work signals restricted to the selected sessions in the
// same verified generation as the selection. For the all scope a failed
// selection still yields the historical unfiltered signals, because all never
// depends on classification; work and auxiliary fail instead of guessing.
// window carries the report's resolved time range: it is part of the
// selection identity, so unknown_count and publication_id describe the
// sessions active in that window. The auxiliary filter handed to read does
// not depend on the window, because the report applies the range itself.
func scopedSignals(ctx context.Context, opts *commandOptions, core *store.Store, scope, client string, window sessionorigin.Selection, meta *sessionSelectionMeta, read func(context.Context, *usage.SessionFilter) (usage.SignalReport, error)) (usage.SignalReport, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return usage.SignalReport{}, err
	}
	var report usage.SignalReport
	if stateDatabaseAbsent(stateRoot, sessionDatabaseName) {
		// No session index was ever created: no session can be auxiliary,
		// so Work and All select everything and Auxiliary selects nothing.
		var filter *usage.SessionFilter
		if scope == sessionorigin.ScopeAuxiliary {
			filter = usage.IncludeSessions(nil)
		}
		report, err = read(ctx, filter)
		if err != nil {
			return usage.SignalReport{}, err
		}
		zero := 0
		meta.Availability, meta.UnknownCount = selectionAvailable, &zero
		return report, nil
	}
	sessions, openErr := store.OpenSessionsReadOnly(ctx, stateRoot)
	if openErr == nil {
		defer sessions.Close()
		reader := sessionorigin.Reader{Core: core, Sessions: sessions, StateRoot: stateRoot}
		snapshot, selectErr := reader.Select(ctx, sessionorigin.Selection{Scope: scope, Client: client, Period: window.Period, From: window.From, To: window.To}, func(ctx context.Context, snapshot *sessionorigin.Snapshot) error {
			var readErr error
			report, readErr = read(ctx, snapshot.SessionFilter())
			return readErr
		})
		if selectErr == nil {
			meta.fill(snapshot)
			return report, nil
		}
		err = selectErr
	} else {
		err = fmt.Errorf("%w: %v", sessionorigin.ErrSessionsUnavailable, openErr)
	}
	if !isSelectionUnavailable(err) {
		return usage.SignalReport{}, err
	}
	if scope != sessionorigin.ScopeAll {
		return usage.SignalReport{}, &errSessionSelectionUnavailable{scope: scope, err: err}
	}
	meta.unavailable(err)
	return read(ctx, nil)
}

// isSelectionUnavailable reports a selection failure that is a state
// availability problem rather than an error of the attached read.
func isSelectionUnavailable(err error) bool {
	return errors.Is(err, sessionorigin.ErrSessionsUnavailable) || errors.Is(err, sessionorigin.ErrClassificationUnavailable) ||
		errors.Is(err, sessionorigin.ErrSelectionStale) || errors.Is(err, sessionorigin.ErrGenerationUnavailable)
}

func renderUsageSelectionScope(w io.Writer, meta *sessionSelectionMeta) error {
	line := "Scope: " + meta.EffectiveScope
	if meta.usageScope != "" {
		line += " (sessions, activity, workflow, tooling) · Usage scope: " + meta.usageScope
	}
	if meta.Availability != selectionAvailable {
		line += " · classification unavailable"
	} else if meta.UnknownCount != nil && meta.EffectiveScope != sessionorigin.ScopeAuxiliary {
		line += fmt.Sprintf(" · %d unknown", *meta.UnknownCount)
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	if meta.usageScope != "" {
		_, err := fmt.Fprintln(w, "All usage and costs remain unchanged.")
		return err
	}
	return nil
}
