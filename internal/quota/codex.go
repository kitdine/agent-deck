package quota

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/kitdine/agent-deck/internal/buildinfo"
)

// DefaultCodexTimeout bounds the full three-step handshake (architecture.md
// C2): initialize, initialized, account/rateLimits/read.
const DefaultCodexTimeout = 5 * time.Second

// CodexResult is account/rateLimits/read mapped onto the domain (C2). When
// rateLimitsByLimitId is absent or null, Windows falls back to the required
// rateLimits snapshot's own primary/secondary — "a backward-compatible
// single-bucket view" per the protocol schema — rather than reporting
// nothing (CA-R1-F1's follow-up; the operator-approved scope and its
// rationale are recorded in tasks.md's task 2 section, not only here).
// rateLimitsByLimitId present as an explicit empty object ({}) does NOT
// fall back: that is a bucketed view the backend supplied and found empty,
// distinct from no bucketed view existing at all, and yields zero windows.
// Neither case is ever a probe failure.
type CodexResult struct {
	Windows        []Observation
	ResetAllowance ResetAllowance
	Plan           string
	PlanReason     Reason // set when rateLimits.planType is absent
	AccountID      string
	Billing        Billing
}

// CodexFailureKind distinguishes why a Codex probe did not produce a result
// (C2's failure classification).
type CodexFailureKind int

const (
	CodexFailureSpawn CodexFailureKind = iota + 1
	CodexFailureExit
	CodexFailureDeadline
	CodexFailureMalformed
	// CodexFailureServerError is a JSON-RPC response carrying a non-null
	// "error" member for the request's id: the process answered, but with a
	// refusal (e.g. not logged in) rather than a result. This is a probe
	// failure, not a parse failure (CA-R1-F2) — the JSON itself was
	// well-formed.
	CodexFailureServerError
)

func (k CodexFailureKind) String() string {
	switch k {
	case CodexFailureSpawn:
		return "spawn"
	case CodexFailureExit:
		return "exit"
	case CodexFailureDeadline:
		return "deadline"
	case CodexFailureMalformed:
		return "malformed"
	case CodexFailureServerError:
		return "server_error"
	default:
		return "unknown"
	}
}

// Reason maps a failure kind to C6's closed reason set. Malformed JSON is
// parse_failed; every other kind — the process never started, exited before
// answering, the deadline expired, or it answered with a JSON-RPC error — is
// probe_failed.
func (k CodexFailureKind) Reason() Reason {
	if k == CodexFailureMalformed {
		return ReasonParseFailed
	}
	return ReasonProbeFailed
}

// CodexFailureError reports a classified Codex probe failure. Err is the
// underlying cause and remains matchable via errors.Unwrap.
type CodexFailureError struct {
	Kind CodexFailureKind
	Err  error
}

func (e *CodexFailureError) Error() string {
	return fmt.Sprintf("codex probe failed (%s): %v", e.Kind, e.Err)
}

func (e *CodexFailureError) Unwrap() error { return e.Err }

// codexCommandArgs names the executable and arguments ProbeCodex runs.
// Package-level so tests can substitute a fake process (CA-R1-F2) — no test
// in this package points it at the real "codex" binary.
var codexCommandArgs = []string{"codex", "app-server"}

func newCodexCommand(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, codexCommandArgs[0], codexCommandArgs[1:]...)
}

// ProbeCodex drives codex app-server over stdio through the three-step
// handshake (C2): initialize, the initialized notification, then
// account/rateLimits/read. Stdin is held open until the matching response
// arrives or timeout expires — writing both requests and closing stdin
// would return only the initialize result, so process exit before the
// second response is a failure, never treated as an empty answer.
//
// Transport framing assumption: one JSON-RPC message per line
// (newline-delimited), matching the request/response arrow diagram in
// architecture.md C2. No captured transcript in this topic's documents
// confirms the exact wire framing; this function's own unit tests exercise
// it against an injected fake process (via codexCommandArgs), never the
// real codex binary.
func ProbeCodex(ctx context.Context, observedAt time.Time, timeout time.Duration) (CodexResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := newCodexCommand(ctx)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return CodexResult{}, &CodexFailureError{Kind: CodexFailureSpawn, Err: err}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CodexResult{}, &CodexFailureError{Kind: CodexFailureSpawn, Err: err}
	}
	if err := cmd.Start(); err != nil {
		return CodexResult{}, &CodexFailureError{Kind: CodexFailureSpawn, Err: err}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	kill := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	// A write failure here means the process has already started but is no
	// longer accepting input (e.g. it already exited) — that is an exit
	// failure, not a spawn failure (CA-R1-F2): spawn is reserved for never
	// having started at all.
	write := func(v any) *CodexFailureError {
		line, err := json.Marshal(v)
		if err != nil {
			return &CodexFailureError{Kind: CodexFailureExit, Err: err}
		}
		line = append(line, '\n')
		if _, err := stdin.Write(line); err != nil {
			return &CodexFailureError{Kind: CodexFailureExit, Err: err}
		}
		return nil
	}

	if failErr := write(map[string]any{
		"method": "initialize", "id": 1,
		"params": map[string]any{"clientInfo": map[string]any{"name": "agentdeck", "version": buildinfo.Version}},
	}); failErr != nil {
		kill()
		return CodexResult{}, failErr
	}

	if _, failErr := readJSONRPCResult(ctx, scanner, 1); failErr != nil {
		kill()
		return CodexResult{}, failErr
	}

	if failErr := write(map[string]any{"method": "initialized"}); failErr != nil {
		kill()
		return CodexResult{}, failErr
	}
	if failErr := write(map[string]any{"method": "account/rateLimits/read", "id": 2}); failErr != nil {
		kill()
		return CodexResult{}, failErr
	}

	resultBytes, failErr := readJSONRPCResult(ctx, scanner, 2)
	if failErr != nil {
		kill()
		return CodexResult{}, failErr
	}

	_ = stdin.Close()
	// Codex PR #5 sixth review, P2: a matching JSON-RPC response arriving
	// before the process's own exit does not prove the process finished
	// cleanly. Discarding cmd.Wait()'s error here let a crash after printing
	// the response still parse and persist that response as a successful
	// current reading -- bypassing this adapter's own documented nonzero-exit
	// classification -- and could replace retained data with output from a
	// crashed process. Classified and returned before parsing, mirroring
	// readJSONRPCResult's own deadline/exit classification.
	if waitErr := cmd.Wait(); waitErr != nil {
		if ctx.Err() != nil {
			return CodexResult{}, &CodexFailureError{Kind: CodexFailureDeadline, Err: ctx.Err()}
		}
		return CodexResult{}, &CodexFailureError{Kind: CodexFailureExit, Err: waitErr}
	}

	result, err := parseCodexResult(resultBytes, observedAt)
	if err != nil {
		return CodexResult{}, &CodexFailureError{Kind: CodexFailureMalformed, Err: err}
	}
	return result, nil
}

type jsonrpcEnvelope struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// readJSONRPCResult scans lines until the response for id arrives.
//
//   - A line that fails to decode as a JSON-RPC envelope is classified
//     malformed immediately, rather than skipped: envelope decoding already
//     guarantees any *matching* real response is well-formed JSON, so an
//     undecodable line while waiting for one is transport corruption, not
//     routine noise (CA-R1-F2).
//   - A matching id whose "error" member is present and non-null is a
//     server-reported failure (e.g. not logged in): probe_failed, not
//     malformed.
//   - Process exit (scanner.Scan() returns false) before either arrives is
//     classified by whether the context deadline had already expired.
func readJSONRPCResult(ctx context.Context, scanner *bufio.Scanner, id int) (json.RawMessage, *CodexFailureError) {
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var env jsonrpcEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			return nil, &CodexFailureError{Kind: CodexFailureMalformed, Err: err}
		}
		if env.ID == nil || *env.ID != id {
			continue
		}
		if len(env.Error) > 0 && !bytes.Equal(env.Error, []byte("null")) {
			return nil, &CodexFailureError{Kind: CodexFailureServerError, Err: fmt.Errorf("codex app-server returned an error response: %s", env.Error)}
		}
		out := make(json.RawMessage, len(env.Result))
		copy(out, env.Result)
		return out, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, &CodexFailureError{Kind: CodexFailureExit, Err: err}
	}
	if ctx.Err() != nil {
		return nil, &CodexFailureError{Kind: CodexFailureDeadline, Err: ctx.Err()}
	}
	return nil, &CodexFailureError{Kind: CodexFailureExit, Err: errors.New("codex app-server exited before answering")}
}

// The types below mirror GetAccountRateLimitsResponse as generated by
// `codex app-server generate-json-schema` against codex-cli 0.154.0 (Round 1
// review evidence for CA-R1-F1) — not the shape architecture.md C2's table
// alone would suggest. In particular: planType, credits (billing), and
// limitName live on a RateLimitSnapshot, not at the JSON-RPC result's top
// level and not on a window; accountId and rateLimitResetCredits are top
// level; credits.balance is a decimal string, never a JSON number.

// codexRateLimitsResult is account/rateLimits/read's result shape.
// rateLimitsByLimitId is decoded separately (decodeOrderedLimits) to
// preserve JSON object key order — C11 requires windows[] to keep vendor
// order. rateLimitUpsell and ordinaryUsageAllowed are intentionally absent:
// the former is refused per C2, the latter is not part of this topic's
// scope.
type codexRateLimitsResult struct {
	AccountID             *string                 `json:"accountId"`
	RateLimitResetCredits *codexResetCredits      `json:"rateLimitResetCredits"`
	RateLimits            *codexRateLimitSnapshot `json:"rateLimits"`
	RateLimitsByLimitID   json.RawMessage         `json:"rateLimitsByLimitId"`
}

// codexRateLimitSnapshot is RateLimitSnapshot. individualLimit,
// rateLimitReachedType, and spendControlReached are refused per C2 and
// intentionally absent; normalModelSlug is not part of this topic's scope.
type codexRateLimitSnapshot struct {
	Credits   *codexCredits `json:"credits"`
	LimitID   *string       `json:"limitId"`
	LimitName *string       `json:"limitName"`
	PlanType  *string       `json:"planType"`
	Primary   *codexWindow  `json:"primary"`
	Secondary *codexWindow  `json:"secondary"`
}

// codexCredits is CreditsSnapshot. balance is a decimal string ("12.50") or
// null, never a JSON number.
type codexCredits struct {
	Balance *string `json:"balance"`
}

// codexWindow is RateLimitWindow. It carries no limitName — that field
// lives one level up, on the RateLimitSnapshot the window belongs to.
//
// UsedPercent is a pointer so a window that omits it, or sends it null, is
// distinguishable from a genuine 0%: encoding/json otherwise leaves a
// non-pointer float64 at its zero value on either input, and mapCodexWindow
// would then persist a false 0% that can replace a previously valid
// percentage and make ApplyObservation record a false reset (Codex PR #5
// fifth review, P1).
type codexWindow struct {
	UsedPercent        *float64 `json:"usedPercent"`
	WindowDurationMins *int     `json:"windowDurationMins"`
	ResetsAt           *int64   `json:"resetsAt"`
}

type codexResetCredits struct {
	AvailableCount *int               `json:"availableCount"`
	Credits        []codexResetCredit `json:"credits"`
}

// codexResetCredit intentionally omits id and description — refused per C2
// (an account-scoped identifier, and vendor marketing copy).
type codexResetCredit struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	GrantedAt *int64 `json:"grantedAt"`
	ExpiresAt *int64 `json:"expiresAt"`
}

type orderedLimitEntry struct {
	LimitID  string
	Snapshot codexRateLimitSnapshot
}

// decodeOrderedLimits preserves rateLimitsByLimitId's JSON key order (C11 —
// windows[] keeps vendor order, never sorted): unmarshaling into a Go map
// would discard it, since map iteration order is unspecified.
func decodeOrderedLimits(raw json.RawMessage) ([]orderedLimitEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("rateLimitsByLimitId: expected an object, got %v", tok)
	}
	var entries []orderedLimitEntry
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("rateLimitsByLimitId: non-string key %v", keyTok)
		}
		var snapshot codexRateLimitSnapshot
		if err := dec.Decode(&snapshot); err != nil {
			return nil, err
		}
		entries = append(entries, orderedLimitEntry{LimitID: key, Snapshot: snapshot})
	}
	if _, err := dec.Token(); err != nil { // consume closing '}'
		return nil, err
	}
	return entries, nil
}

// parseCodexResult maps one account/rateLimits/read result (raw JSON bytes)
// onto the domain, per C2's field-mapping table and the real protocol shape
// (see the type docs above). It returns a non-nil error only for malformed
// JSON. rateLimitsByLimitId absent or null falls back to rateLimits'
// primary/secondary (CodexResult's doc; CA-R1-F1 follow-up); present as an
// explicit empty object it does not, yielding no Windows. Neither is a
// probe failure.
func parseCodexResult(raw []byte, observedAt time.Time) (CodexResult, error) {
	var resp codexRateLimitsResult
	if err := json.Unmarshal(raw, &resp); err != nil {
		return CodexResult{}, err
	}

	// Codex PR #5 sixth review, P1: an absent or null accountId must not
	// become a successful observation scoped to the empty-account sentinel.
	// Store.Record/PutEnvelope compare that sentinel against whatever
	// account digest is already on record, so an unscoped write here would
	// read as an account change, discarding the correctly attributed
	// account's windows, notices, and envelope and replacing them with data
	// that has no confirmed account -- while the wire still reports Codex
	// attribution as confirmed.
	if resp.AccountID == nil || *resp.AccountID == "" {
		return CodexResult{}, errors.New("codex account/rateLimits/read: missing accountId")
	}

	result := CodexResult{AccountID: *resp.AccountID}

	if resp.RateLimits != nil {
		if resp.RateLimits.PlanType != nil {
			result.Plan = *resp.RateLimits.PlanType // opaque: never branched on
		} else {
			result.PlanReason = ReasonNotReported
		}
		result.Billing = mapCodexBilling(resp.RateLimits.Credits)
	} else {
		result.PlanReason = ReasonNotReported
	}

	// C2 / open question 1: total is always not_reported. No vendor field
	// supplies it, and availableCount is a remaining count, not a total.
	result.ResetAllowance.TotalReason = ReasonNotReported
	if resp.RateLimitResetCredits != nil {
		if resp.RateLimitResetCredits.AvailableCount != nil {
			result.ResetAllowance.Remaining = *resp.RateLimitResetCredits.AvailableCount
			result.ResetAllowance.HasRemaining = true
		}
		for _, c := range resp.RateLimitResetCredits.Credits {
			credit := ResetCredit{Title: c.Title, Status: c.Status}
			if c.GrantedAt != nil {
				credit.GrantedAt = time.Unix(*c.GrantedAt, 0).UTC()
			}
			if c.ExpiresAt != nil {
				credit.ExpiresAt = time.Unix(*c.ExpiresAt, 0).UTC()
			}
			result.ResetAllowance.Credits = append(result.ResetAllowance.Credits, credit)
		}
	}

	if len(bytes.TrimSpace(resp.RateLimitsByLimitID)) == 0 || bytes.Equal(bytes.TrimSpace(resp.RateLimitsByLimitID), []byte("null")) {
		// No per-limit bucketed view (an older backend, or an account with
		// none). rateLimits — required — is itself a RateLimitSnapshot with
		// its own primary/secondary, "a backward-compatible single-bucket
		// view" per the schema: fall back to it rather than reporting zero
		// windows, per the operator's decision on this open question.
		if resp.RateLimits != nil {
			windows, err := appendCodexSnapshotWindows(nil, resolveCodexLimitID(resp.RateLimits), *resp.RateLimits, observedAt, 0)
			if err != nil {
				return CodexResult{}, err
			}
			result.Windows = windows
		}
	} else {
		entries, err := decodeOrderedLimits(resp.RateLimitsByLimitID)
		if err != nil {
			return CodexResult{}, err
		}
		order := 0
		for _, entry := range entries {
			windows, err := appendCodexSnapshotWindows(result.Windows, entry.LimitID, entry.Snapshot, observedAt, order)
			if err != nil {
				return CodexResult{}, err
			}
			result.Windows = windows
			order = len(result.Windows)
		}
	}

	for i := range result.Windows {
		result.Windows[i].AccountID = result.AccountID
	}

	return result, nil
}

// codexFallbackLimitID is the window_key limitId used for the
// rateLimits.primary/.secondary fallback when the vendor supplies no
// per-limit bucketed view and rateLimits.limitId is itself also absent.
// "codex" matches the observed per-limit key for the account's main limit
// when a bucketed view is present, keeping the fallback's window_key
// indistinguishable from the bucketed shape a newer backend would report
// for the same underlying limit.
const codexFallbackLimitID = "codex"

func resolveCodexLimitID(snapshot *codexRateLimitSnapshot) string {
	if snapshot.LimitID != nil && *snapshot.LimitID != "" {
		return *snapshot.LimitID
	}
	return codexFallbackLimitID
}

// appendCodexSnapshotWindows appends the (0, 1, or 2) windows a
// RateLimitSnapshot carries — its primary then its secondary — using the
// snapshot's own limitName as every appended window's label. vendorOrder
// continues from orderStart.
func appendCodexSnapshotWindows(windows []Observation, limitID string, snapshot codexRateLimitSnapshot, observedAt time.Time, orderStart int) ([]Observation, error) {
	var label string
	if snapshot.LimitName != nil {
		label = *snapshot.LimitName
	}
	order := orderStart
	if snapshot.Primary != nil {
		obs, err := mapCodexWindow(limitID, false, *snapshot.Primary, label, observedAt, order)
		if err != nil {
			return windows, err
		}
		windows = append(windows, obs)
		order++
	}
	if snapshot.Secondary != nil {
		obs, err := mapCodexWindow(limitID, true, *snapshot.Secondary, label, observedAt, order)
		if err != nil {
			return windows, err
		}
		windows = append(windows, obs)
	}
	return windows, nil
}

// mapCodexBilling maps CreditsSnapshot.balance, a decimal string, onto the
// domain Billing (C2: credits -> billing). An absent or unparseable balance
// leaves HasBalance false rather than erroring the whole probe — billing is
// a minor disclosure, not something a malformed sub-field should fail on.
func mapCodexBilling(credits *codexCredits) Billing {
	if credits == nil || credits.Balance == nil {
		return Billing{}
	}
	value, err := strconv.ParseFloat(*credits.Balance, 64)
	if err != nil {
		return Billing{}
	}
	return Billing{Balance: value, HasBalance: true}
}

// mapCodexWindow errors when w.UsedPercent is absent: an incomplete window
// is malformed, never a silent 0% (see codexWindow's doc).
func mapCodexWindow(limitID string, secondary bool, w codexWindow, label string, observedAt time.Time, vendorOrder int) (Observation, error) {
	windowKey := CodexWindowKey(limitID, secondary)
	if w.UsedPercent == nil {
		return Observation{}, fmt.Errorf("codex window %q: missing usedPercent", windowKey)
	}
	obs := Observation{
		Client: ClientCodex, WindowKey: windowKey,
		Source: SourceCodex, ObservedAt: observedAt, VendorOrder: vendorOrder,
		UsedPercent: *w.UsedPercent, Label: label,
	}
	if w.WindowDurationMins != nil {
		obs.WindowMinutes = *w.WindowDurationMins
	} else {
		obs.WindowMinutesReason = ReasonNotReported
	}
	if w.ResetsAt != nil {
		obs.ResetsAt = time.Unix(*w.ResetsAt, 0).UTC()
	}
	return obs, nil
}
