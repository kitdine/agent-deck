// Package sessionorigin owns session-origin classification vocabulary, the
// user default scope and override write service. The store package holds only
// SQL and transactions; adapters and the reducer produce evidence elsewhere.
package sessionorigin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// Session scopes select which sessions a view shows. Work includes unknown.
const (
	ScopeWork      = "work"
	ScopeAuxiliary = "auxiliary"
	ScopeAll       = "all"
)

// Classes are automatic and effective classification values.
const (
	ClassWork      = "work"
	ClassAuxiliary = "auxiliary"
	ClassUnknown   = "unknown"
)

// Rule versions identify which maintenance rule produced evidence.
const (
	RuleClaudeMemSDKMapV1     = "claude-mem-sdk-map-v1"
	RuleCodexOTelBackgroundV1 = "codex-otel-background-v1"
)

// Classification reason codes are a closed, externally stable enum
// (architecture §3.3). Adding one is a contract change for all four surfaces.
const (
	ReasonMaintenanceOrigin       = "maintenance_origin"
	ReasonOriginMetadataMissing   = "origin_metadata_missing"
	ReasonTaskChildUnconfirmed    = "task_child_unconfirmed"
	ReasonConflictingOrigin       = "conflicting_origin"
	ReasonOriginSourceUnavailable = "origin_source_unavailable"
	ReasonOriginRuleUnsupported   = "origin_rule_unsupported"
	ReasonOriginInputMismatch     = "origin_input_mismatch"
)

var reasonAutomatic = map[string]string{
	ReasonMaintenanceOrigin:       ClassAuxiliary,
	ReasonOriginMetadataMissing:   ClassUnknown,
	ReasonTaskChildUnconfirmed:    ClassUnknown,
	ReasonConflictingOrigin:       ClassUnknown,
	ReasonOriginSourceUnavailable: ClassUnknown,
	ReasonOriginRuleUnsupported:   ClassUnknown,
	ReasonOriginInputMismatch:     ClassUnknown,
}

// ReasonCodes lists the closed enum in its documented order.
func ReasonCodes() []string {
	return []string{ReasonMaintenanceOrigin, ReasonOriginMetadataMissing, ReasonTaskChildUnconfirmed,
		ReasonConflictingOrigin, ReasonOriginSourceUnavailable, ReasonOriginRuleUnsupported, ReasonOriginInputMismatch}
}

// AutomaticForReason returns the automatic class a known reason implies.
func AutomaticForReason(reason string) (string, bool) {
	class, ok := reasonAutomatic[reason]
	return class, ok
}

// Error codes of the write and read-back contract (architecture §5).
const (
	CodeConflict            = "origin_conflict"
	CodeWriteFailed         = "origin_write_failed"
	CodeWriteUnknown        = "origin_write_unknown"
	CodeIdentityUnavailable = "origin_identity_unavailable"
	CodeInvalidRequest      = "invalid_request"
	CodeReadFailed          = "read_failed"
)

// Receipt and outcome vocabulary.
const (
	OutcomeSaved         = store.OriginOutcomeSaved
	OutcomeNoChange      = store.OriginOutcomeNoChange
	OutcomeUnknown       = "unknown"
	WarningPermissions   = "origin_permissions_not_secured"
	ReasonNotQueried     = "not_queried"
	DefaultOverrideLimit = 20
	MaxOverrideLimit     = 200
)

// ErrInvalidArgument is an input error (exit 2), never a write failure.
var ErrInvalidArgument = errors.New("invalid session origin argument")

// Error is a typed write/read failure. RequestID is set whenever the caller
// must read the receipt back instead of retrying with a new request.
type Error struct {
	Code            string
	RequestID       string
	CurrentRevision *int64
	Err             error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// Store is the persistence the service needs; *store.Store implements it.
type Store interface {
	OriginPreference(context.Context) (store.OriginPreference, error)
	OriginOverrideRecord(context.Context, string, string) (store.OriginOverride, bool, error)
	OriginOverrides(context.Context, int, int) ([]store.OriginOverride, int, error)
	OriginReceipt(context.Context, string) (store.OriginReceipt, bool, error)
	ApplyOriginMutation(context.Context, store.OriginMutation) (store.OriginReceipt, error)
}

// Service validates requests, derives idempotency payloads and maps storage
// results onto the stable contract. It never reads session sources.
type Service struct {
	Store        Store
	Now          func() time.Time
	NewRequestID func() (string, error)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) requestID(given string) (string, error) {
	if given != "" {
		if !ValidRequestID(given) {
			return "", fmt.Errorf("%w: request id", ErrInvalidArgument)
		}
		return given, nil
	}
	if s.NewRequestID != nil {
		return s.NewRequestID()
	}
	return NewRequestID()
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// ValidRequestID accepts printable, path-free identifiers of at most 128 bytes.
func ValidRequestID(id string) bool { return requestIDPattern.MatchString(id) }

// NewRequestID returns a random request identifier.
func NewRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "req-" + hex.EncodeToString(value[:]), nil
}

// DefaultScope is the default DTO. Confirmed=false always has a null value:
// consumers must never fall back to work on an unconfirmed read.
type DefaultScope struct {
	Confirmed bool    `json:"confirmed"`
	Value     *string `json:"value"`
	Revision  *int64  `json:"revision"`
	Reason    *string `json:"reason"`
}

// DefaultScope reads the persisted default. Only a successful read that finds
// no configuration yields work at revision 0.
func (s Service) DefaultScope(ctx context.Context) DefaultScope {
	preference, err := s.Store.OriginPreference(ctx)
	if err != nil {
		reason := CodeReadFailed
		return DefaultScope{Reason: &reason}
	}
	value, revision := ScopeWork, int64(0)
	if preference.Present {
		value, revision = preference.Scope, preference.Revision
	}
	return DefaultScope{Confirmed: true, Value: &value, Revision: &revision}
}

// Override is one effective user override as enumerated for clients.
type Override struct {
	Client         string `json:"client"`
	SessionID      string `json:"session_id"`
	Value          string `json:"value"`
	BindingStatus  string `json:"binding_status"`
	BindingLimited bool   `json:"binding_limited"`
	Revision       int64  `json:"revision"`
}

// OverridePage is one page of effective overrides, ordered by client/session_id.
type OverridePage struct {
	Items []Override `json:"items"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
	Total int        `json:"total"`
}

// ListOverrides enumerates effective overrides, unmatched ones included.
// limit 0 selects the default of 20; the maximum is 200.
func (s Service) ListOverrides(ctx context.Context, page, limit int) (OverridePage, error) {
	if limit == 0 {
		limit = DefaultOverrideLimit
	}
	if page < 1 || limit < 1 || limit > MaxOverrideLimit {
		return OverridePage{}, fmt.Errorf("%w: page or limit", ErrInvalidArgument)
	}
	rows, total, err := s.Store.OriginOverrides(ctx, (page-1)*limit, limit)
	if err != nil {
		return OverridePage{}, &Error{Code: CodeReadFailed, Err: err}
	}
	out := OverridePage{Items: []Override{}, Page: page, Limit: limit, Total: total}
	for _, row := range rows {
		if row.Value == nil {
			continue
		}
		out.Items = append(out.Items, Override{Client: row.Client, SessionID: row.SessionID, Value: *row.Value,
			BindingStatus: row.BindingState, BindingLimited: row.BindingLimited, Revision: row.Revision})
	}
	return out, nil
}

// OverrideRevision is the core revision of a key, tombstones included; a key
// that never had an override is 0. A read error is never reported as 0.
func (s Service) OverrideRevision(ctx context.Context, client, sessionID string) (int64, error) {
	if err := validTarget(client, sessionID); err != nil {
		return 0, err
	}
	row, found, err := s.Store.OriginOverrideRecord(ctx, client, sessionID)
	if err != nil {
		return 0, &Error{Code: CodeReadFailed, Err: err}
	}
	if !found {
		return 0, nil
	}
	return row.Revision, nil
}

// Target names one override key.
type Target struct {
	Client    string `json:"client"`
	SessionID string `json:"session_id"`
}

// Receipt is the result of one recorded operation. It describes that
// operation, not necessarily the current state.
type Receipt struct {
	RequestID string   `json:"request_id"`
	Kind      string   `json:"kind"`
	Target    *Target  `json:"target,omitempty"`
	Outcome   string   `json:"outcome"`
	Value     *string  `json:"value"`
	Revision  int64    `json:"revision"`
	Replayed  bool     `json:"replayed"`
	Warnings  []string `json:"warnings,omitempty"`
}

// ClearResult is the clear contract: the operation never queries the session
// index, so automatic/effective are fixed null and classification is not_queried.
type ClearResult struct {
	Receipt
	Override                *string `json:"override"`
	Automatic               *string `json:"automatic"`
	Effective               *string `json:"effective"`
	ClassificationAvailable bool    `json:"classification_available"`
	ClassificationReason    string  `json:"classification_reason"`
}

// ScopeRequest saves the default scope with compare-and-set.
type ScopeRequest struct {
	RequestID        string
	Value            string
	ExpectedRevision int64
}

// SetDefaultScope persists the default scope under CAS and idempotency.
func (s Service) SetDefaultScope(ctx context.Context, request ScopeRequest) (Receipt, error) {
	if request.Value != ScopeWork && request.Value != ScopeAuxiliary && request.Value != ScopeAll {
		return Receipt{}, fmt.Errorf("%w: scope", ErrInvalidArgument)
	}
	if request.ExpectedRevision < 0 {
		return Receipt{}, fmt.Errorf("%w: expected revision", ErrInvalidArgument)
	}
	expected := request.ExpectedRevision
	return s.apply(ctx, store.OriginMutation{RequestID: request.RequestID, Kind: store.OriginMutationScopeSet, Value: request.Value, ExpectedRevision: expected}, &expected)
}

// OverrideSetRequest sets an override for a target whose binding the caller
// has already resolved and verified. A nil Binding means identity is unconfirmed.
type OverrideSetRequest struct {
	RequestID        string
	Client           string
	SessionID        string
	Value            string
	Binding          *store.OriginBinding
	ExpectedRevision int64
}

// SetOverride persists a work/auxiliary override bound to a verified identity.
func (s Service) SetOverride(ctx context.Context, request OverrideSetRequest) (Receipt, error) {
	if err := validTarget(request.Client, request.SessionID); err != nil {
		return Receipt{}, err
	}
	if request.Value != ClassWork && request.Value != ClassAuxiliary {
		return Receipt{}, fmt.Errorf("%w: override value", ErrInvalidArgument)
	}
	if request.ExpectedRevision < 0 {
		return Receipt{}, fmt.Errorf("%w: expected revision", ErrInvalidArgument)
	}
	if request.Binding == nil || request.Binding.Digest == "" {
		return Receipt{}, &Error{Code: CodeIdentityUnavailable}
	}
	expected := request.ExpectedRevision
	return s.apply(ctx, store.OriginMutation{RequestID: request.RequestID, Kind: store.OriginMutationOverrideSet,
		Client: request.Client, SessionID: request.SessionID, Value: request.Value, Binding: *request.Binding, ExpectedRevision: expected}, &expected)
}

// OverrideClearRequest withdraws an override by explicit core key. A nil
// ExpectedRevision performs a read-CAS against the core override record only.
type OverrideClearRequest struct {
	RequestID        string
	Client           string
	SessionID        string
	ExpectedRevision *int64
}

// ClearOverride resolves only the core override key: it never requires the
// session to exist or be visible, verifies no binding and opens no source.
func (s Service) ClearOverride(ctx context.Context, request OverrideClearRequest) (ClearResult, error) {
	if err := validTarget(request.Client, request.SessionID); err != nil {
		return ClearResult{}, err
	}
	mutation := store.OriginMutation{RequestID: request.RequestID, Kind: store.OriginMutationOverrideClear, Client: request.Client, SessionID: request.SessionID}
	explicit := request.ExpectedRevision
	if explicit != nil {
		if *explicit < 0 {
			return ClearResult{}, fmt.Errorf("%w: expected revision", ErrInvalidArgument)
		}
		mutation.ExpectedRevision = *explicit
	} else {
		revision, err := s.OverrideRevision(ctx, request.Client, request.SessionID)
		if err != nil {
			return ClearResult{}, err
		}
		mutation.ExpectedRevision = revision
	}
	receipt, err := s.apply(ctx, mutation, explicit)
	if err != nil {
		return ClearResult{}, err
	}
	return clearResult(receipt), nil
}

func clearResult(receipt Receipt) ClearResult {
	receipt.Value = nil
	return ClearResult{Receipt: receipt, ClassificationReason: ReasonNotQueried}
}

// ReadBack is a receipt lookup: Known=false means no committed result exists
// for the request and its outcome stays unknown.
type ReadBack struct {
	Known   bool
	Receipt Receipt
}

// ReadReceipt reads a request's committed outcome from the local core only.
func (s Service) ReadReceipt(ctx context.Context, requestID string) (ReadBack, error) {
	if !ValidRequestID(requestID) {
		return ReadBack{}, fmt.Errorf("%w: request id", ErrInvalidArgument)
	}
	stored, found, err := s.Store.OriginReceipt(ctx, requestID)
	if err != nil {
		return ReadBack{}, &Error{Code: CodeReadFailed, RequestID: requestID, Err: err}
	}
	if !found {
		return ReadBack{Receipt: Receipt{RequestID: requestID, Outcome: OutcomeUnknown}}, nil
	}
	receipt := receiptFrom(stored)
	receipt.Replayed = false
	return ReadBack{Known: true, Receipt: receipt}, nil
}

// ClearReadBack shapes a known clear receipt with the fixed clear fields.
func ClearReadBack(receipt Receipt) ClearResult { return clearResult(receipt) }

func (s Service) apply(ctx context.Context, mutation store.OriginMutation, explicitExpected *int64) (Receipt, error) {
	id, err := s.requestID(mutation.RequestID)
	if err != nil {
		return Receipt{}, err
	}
	mutation.RequestID = id
	mutation.PayloadHash = PayloadHash(mutation, explicitExpected)
	mutation.Now = s.now()
	stored, err := s.Store.ApplyOriginMutation(ctx, mutation)
	var conflict *store.OriginConflictError
	var unknown *store.OriginCommitUnknownError
	switch {
	case err == nil:
	case errors.Is(err, store.ErrOriginSecureFilesFailed):
		receipt := receiptFrom(stored)
		receipt.Warnings = []string{WarningPermissions}
		return receipt, nil
	case errors.As(err, &conflict):
		current := conflict.Current
		return Receipt{}, &Error{Code: CodeConflict, RequestID: id, CurrentRevision: &current, Err: err}
	case errors.Is(err, store.ErrOriginRequestMismatch):
		return Receipt{}, &Error{Code: CodeInvalidRequest, RequestID: id, Err: err}
	case errors.As(err, &unknown):
		return Receipt{}, &Error{Code: CodeWriteUnknown, RequestID: id, Err: err}
	default:
		return Receipt{}, &Error{Code: CodeWriteFailed, RequestID: id, Err: err}
	}
	return receiptFrom(stored), nil
}

func receiptFrom(stored store.OriginReceipt) Receipt {
	receipt := Receipt{RequestID: stored.RequestID, Kind: stored.Kind, Outcome: stored.Outcome, Value: stored.Value, Revision: stored.Revision, Replayed: stored.Replayed}
	if stored.Kind != store.OriginMutationScopeSet {
		receipt.Target = &Target{Client: stored.Client, SessionID: stored.SessionID}
	}
	return receipt
}

// PayloadHash is the canonical request payload digest. The expected revision
// is part of the payload only when the caller supplied it; an implicit
// read-CAS retry under the same request ID must still replay its receipt.
func PayloadHash(m store.OriginMutation, explicitExpected *int64) string {
	expected := "implicit"
	if explicitExpected != nil {
		expected = strconv.FormatInt(*explicitExpected, 10)
	}
	fields := []string{"origin-mutation-v1", m.Kind, m.Client, m.SessionID, m.Value, m.Binding.Digest, strconv.FormatBool(m.Binding.Limited), expected}
	for i, field := range fields {
		fields[i] = strconv.Itoa(len(field)) + ":" + field
	}
	digest := sha256.Sum256([]byte(strings.Join(fields, "")))
	return hex.EncodeToString(digest[:])
}

// Clients whose sessions can be classified.
var knownClients = map[string]bool{"codex": true, "claude": true}

func validTarget(client, sessionID string) error {
	if !knownClients[client] {
		return fmt.Errorf("%w: client", ErrInvalidArgument)
	}
	if sessionID == "" || len(sessionID) > 256 || strings.ContainsAny(sessionID, "\x00\n\r/") {
		return fmt.Errorf("%w: session id", ErrInvalidArgument)
	}
	return nil
}
