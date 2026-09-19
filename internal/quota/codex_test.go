package quota

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return data
}

func TestParseCodexResultFullMapping(t *testing.T) {
	raw := readFixture(t, "codex_rate_limits.json")
	observedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	result, err := parseCodexResult(raw, observedAt)
	if err != nil {
		t.Fatalf("parseCodexResult: %v", err)
	}

	if result.AccountID != "acct_demo_0001" {
		t.Fatalf("AccountID = %q, want acct_demo_0001", result.AccountID)
	}
	// CA-R1-F1: plan comes from the required rateLimits snapshot, not a
	// bare top-level field.
	if result.Plan != "prolite" || result.PlanReason != "" {
		t.Fatalf("Plan = (%q, %q), want (prolite, \"\")", result.Plan, result.PlanReason)
	}
	// CA-R1-F1: billing (credits.balance, a decimal string) comes from the
	// same snapshot.
	if !result.Billing.HasBalance || result.Billing.Balance != 12.5 {
		t.Fatalf("Billing = %+v, want HasBalance=true Balance=12.5", result.Billing)
	}

	if len(result.Windows) != 3 {
		t.Fatalf("Windows = %d, want 3 (codex primary, bengalfox primary, bengalfox secondary): %+v", len(result.Windows), result.Windows)
	}

	// Vendor order: codex (the account's main limit) first, then
	// codex_bengalfox's primary then secondary — C11.
	codex := result.Windows[0]
	if codex.WindowKey != "codex" || codex.VendorOrder != 0 {
		t.Fatalf("Windows[0] = %+v, want WindowKey=codex VendorOrder=0", codex)
	}
	if codex.UsedPercent != 79 || codex.WindowMinutes != 10080 || codex.WindowMinutesReason != "" {
		t.Fatalf("codex window = %+v, want used=79 minutes=10080", codex)
	}
	if !codex.ResetsAt.Equal(time.Unix(1758121200, 0).UTC()) {
		t.Fatalf("codex.ResetsAt = %v, want %v", codex.ResetsAt, time.Unix(1758121200, 0).UTC())
	}
	if codex.Label != "" {
		// CA-R1-F1: limitName lives on the RateLimitSnapshot the window
		// belongs to (rateLimitsByLimitId["codex"].limitName), not inside
		// the window itself — this fixture's codex snapshot has none.
		t.Fatalf("codex.Label = %q, want empty (this snapshot's limitName is null)", codex.Label)
	}
	if codex.Source != SourceCodex || !codex.ObservedAt.Equal(observedAt) {
		t.Fatalf("codex window Source/ObservedAt = %v/%v, want %v/%v", codex.Source, codex.ObservedAt, SourceCodex, observedAt)
	}
	if codex.AccountID != "acct_demo_0001" {
		t.Fatalf("codex.AccountID = %q, want propagated from the response", codex.AccountID)
	}

	bengalfoxPrimary := result.Windows[1]
	if bengalfoxPrimary.WindowKey != "codex_bengalfox" || bengalfoxPrimary.VendorOrder != 1 {
		t.Fatalf("Windows[1] = %+v, want WindowKey=codex_bengalfox VendorOrder=1", bengalfoxPrimary)
	}
	if bengalfoxPrimary.UsedPercent != 12.5 || bengalfoxPrimary.WindowMinutes != 300 || bengalfoxPrimary.Label != "bengalfox" {
		// CA-R1-F1: label comes from rateLimitsByLimitId["codex_bengalfox"].limitName,
		// applied to BOTH its primary and secondary windows.
		t.Fatalf("bengalfox primary window = %+v, want label=bengalfox", bengalfoxPrimary)
	}

	bengalfoxSecondary := result.Windows[2]
	if bengalfoxSecondary.WindowKey != "codex_bengalfox_secondary" || bengalfoxSecondary.VendorOrder != 2 {
		t.Fatalf("Windows[2] = %+v, want WindowKey=codex_bengalfox_secondary VendorOrder=2", bengalfoxSecondary)
	}
	if bengalfoxSecondary.UsedPercent != 3 || bengalfoxSecondary.WindowMinutes != 10080 || bengalfoxSecondary.Label != "bengalfox" {
		t.Fatalf("bengalfox secondary window = %+v, want label=bengalfox", bengalfoxSecondary)
	}

	// C2 / open question 1: total is always not_reported.
	if result.ResetAllowance.TotalReason != ReasonNotReported {
		t.Fatalf("ResetAllowance.TotalReason = %q, want not_reported", result.ResetAllowance.TotalReason)
	}
	if result.ResetAllowance.Total != 0 {
		t.Fatalf("ResetAllowance.Total = %d, want 0 (never derived from availableCount)", result.ResetAllowance.Total)
	}
	if !result.ResetAllowance.HasRemaining || result.ResetAllowance.Remaining != 3 {
		t.Fatalf("ResetAllowance.Remaining = (%d, %v), want (3, true)", result.ResetAllowance.Remaining, result.ResetAllowance.HasRemaining)
	}
	if len(result.ResetAllowance.Credits) != 1 {
		t.Fatalf("ResetAllowance.Credits = %+v, want one credit", result.ResetAllowance.Credits)
	}
	credit := result.ResetAllowance.Credits[0]
	if credit.Title != "Monthly reset credit" || credit.Status != "available" {
		t.Fatalf("credit = %+v, want title/status mapped (id and description refused, C2)", credit)
	}
	if !credit.GrantedAt.Equal(time.Unix(1755000000, 0).UTC()) || !credit.ExpiresAt.Equal(time.Unix(1762776000, 0).UTC()) {
		t.Fatalf("credit timestamps = %+v", credit)
	}
}

func TestParseCodexResultMissingRateLimitsByLimitIDIsNotAFailure(t *testing.T) {
	raw := readFixture(t, "codex_rate_limits_missing.json")
	result, err := parseCodexResult(raw, time.Now())
	if err != nil {
		t.Fatalf("a well-formed response with rateLimitsByLimitId=null must not be an error: %v", err)
	}
	if len(result.Windows) != 0 {
		t.Fatalf("Windows = %+v, want none", result.Windows)
	}
	if result.Plan != "free" || result.AccountID != "acct_demo_0002" {
		t.Fatalf("account-level fields must still be mapped: plan=%q account=%q", result.Plan, result.AccountID)
	}
}

func TestParseCodexResultFallsBackToTopLevelSnapshotWindows(t *testing.T) {
	// CA-R1-F1 follow-up: when rateLimitsByLimitId is null (no bucketed
	// view — an older backend, or an account without one), the required
	// rateLimits snapshot's own primary/secondary must still surface as
	// windows rather than reporting nothing.
	raw := readFixture(t, "codex_rate_limits_fallback.json")
	observedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	result, err := parseCodexResult(raw, observedAt)
	if err != nil {
		t.Fatalf("parseCodexResult: %v", err)
	}
	if result.Plan != "go" || result.AccountID != "acct_demo_0004" {
		t.Fatalf("Plan/AccountID = %q/%q, want go/acct_demo_0004", result.Plan, result.AccountID)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %d, want 2 (fallback primary+secondary): %+v", len(result.Windows), result.Windows)
	}
	primary := result.Windows[0]
	if primary.WindowKey != "codex" || primary.UsedPercent != 45 || primary.WindowMinutes != 300 {
		// No limitId in the fixture: falls back to the "codex" placeholder
		// (resolveCodexLimitID), matching the bucketed key a newer backend
		// would use for the account's main limit.
		t.Fatalf("primary fallback window = %+v, want WindowKey=codex used=45 minutes=300", primary)
	}
	if primary.AccountID != "acct_demo_0004" {
		t.Fatalf("primary.AccountID = %q, want propagated", primary.AccountID)
	}
	secondary := result.Windows[1]
	if secondary.WindowKey != "codex_secondary" || secondary.UsedPercent != 60 || secondary.WindowMinutes != 10080 {
		t.Fatalf("secondary fallback window = %+v, want WindowKey=codex_secondary used=60 minutes=10080", secondary)
	}
}

func TestParseCodexResultFallbackUsesSnapshotLimitIDAndLabel(t *testing.T) {
	raw := readFixture(t, "codex_rate_limits_fallback_with_limit_id.json")
	result, err := parseCodexResult(raw, time.Now())
	if err != nil {
		t.Fatalf("parseCodexResult: %v", err)
	}
	if len(result.Windows) != 1 {
		t.Fatalf("Windows = %+v, want exactly one (only primary present)", result.Windows)
	}
	w := result.Windows[0]
	if w.WindowKey != "codex_bengalfox" || w.Label != "bengalfox" {
		t.Fatalf("fallback window = %+v, want WindowKey=codex_bengalfox label=bengalfox (from rateLimits.limitId/limitName)", w)
	}
}

func TestParseCodexResultEmptyRateLimitsByLimitIDDoesNotFallBack(t *testing.T) {
	// CA-R2-F1 regression: the fallback fixture deliberately has an explicit
	// empty rateLimitsByLimitId ({}) AND a populated rateLimits.primary, so
	// this proves the no-fallback rule rather than merely having nothing to
	// fall back to. An empty object is a bucketed view the backend supplied
	// and found empty, distinct from rateLimitsByLimitId being absent or
	// null — only those two fall back (CodexResult's doc).
	raw := readFixture(t, "codex_rate_limits_empty.json")
	result, err := parseCodexResult(raw, time.Now())
	if err != nil {
		t.Fatalf("an explicitly empty rateLimitsByLimitId must not be an error: %v", err)
	}
	if len(result.Windows) != 0 {
		t.Fatalf("Windows = %+v, want none: an empty rateLimitsByLimitId must not fall back to rateLimits.primary/.secondary even though this fixture's rateLimits has data", result.Windows)
	}
	if result.Plan != "free" {
		t.Fatalf("Plan = %q, want free (account-level fields still map)", result.Plan)
	}
}

func TestParseCodexResultPlanAbsentIsNotReported(t *testing.T) {
	result, err := parseCodexResult([]byte(`{"accountId":"acct_x","rateLimits":{}}`), time.Now())
	if err != nil {
		t.Fatalf("parseCodexResult: %v", err)
	}
	if result.Plan != "" || result.PlanReason != ReasonNotReported {
		t.Fatalf("Plan = (%q, %q), want (\"\", not_reported)", result.Plan, result.PlanReason)
	}
}

func TestParseCodexResultBillingAbsentOrUnparseable(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"no rateLimits", `{"accountId":"acct_x"}`},
		{"no credits", `{"accountId":"acct_x","rateLimits":{}}`},
		{"null balance", `{"accountId":"acct_x","rateLimits":{"credits":{"hasCredits":false,"unlimited":false,"balance":null}}}`},
		{"unparseable balance", `{"accountId":"acct_x","rateLimits":{"credits":{"hasCredits":true,"unlimited":false,"balance":"not-a-number"}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := parseCodexResult([]byte(c.raw), time.Now())
			if err != nil {
				t.Fatalf("parseCodexResult: %v", err)
			}
			if result.Billing.HasBalance {
				t.Fatalf("Billing = %+v, want HasBalance=false", result.Billing)
			}
		})
	}
}

func TestParseCodexResultMalformedJSONIsAnError(t *testing.T) {
	raw := readFixture(t, "codex_rate_limits_malformed.json")
	if _, err := parseCodexResult(raw, time.Now()); err == nil {
		t.Fatal("malformed JSON must produce an error, distinct from a missing-rateLimitsByLimitId non-failure")
	}
}

// Codex PR #5 fifth review, P1: usedPercent absent or null must not be
// silently persisted as a false 0% (codexWindow's doc). Both cases are
// exercised because encoding/json treats an absent key and an explicit null
// identically for a pointer field, but a naive hand-rolled check of "is the
// key present" would only catch the first.
func TestParseCodexResultMissingUsedPercentIsMalformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"absent", `{"accountId":"acct_x","rateLimits":{"primary":{"windowDurationMins":300}}}`},
		{"null", `{"accountId":"acct_x","rateLimits":{"primary":{"usedPercent":null,"windowDurationMins":300}}}`},
		{"absent in bucketed view", `{"accountId":"acct_x","rateLimitsByLimitId":{"codex":{"primary":{"windowDurationMins":300}}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseCodexResult([]byte(c.raw), time.Now()); err == nil {
				t.Fatal("a window missing usedPercent must be classified malformed, not accepted as a 0% observation")
			}
		})
	}
}

// Codex PR #5 sixth review, P1: an absent or null accountId must be
// malformed, not a successful observation scoped to the empty-account
// sentinel -- see the fix's comment in parseCodexResult for why that
// sentinel is dangerous even when the response also carries real windows.
func TestParseCodexResultMissingAccountIDIsMalformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"absent", `{"rateLimits":{"primary":{"usedPercent":10}}}`},
		{"null", `{"accountId":null,"rateLimits":{"primary":{"usedPercent":10}}}`},
		{"empty string", `{"accountId":"","rateLimits":{"primary":{"usedPercent":10}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseCodexResult([]byte(c.raw), time.Now()); err == nil {
				t.Fatal("a response missing accountId must be classified malformed, not accepted as an unscoped observation")
			}
		})
	}
}

func TestCodexFailureKindReason(t *testing.T) {
	cases := []struct {
		kind CodexFailureKind
		want Reason
	}{
		{CodexFailureSpawn, ReasonProbeFailed},
		{CodexFailureExit, ReasonProbeFailed},
		{CodexFailureDeadline, ReasonProbeFailed},
		{CodexFailureMalformed, ReasonParseFailed},
		{CodexFailureServerError, ReasonProbeFailed},
	}
	for _, c := range cases {
		if got := c.kind.Reason(); got != c.want {
			t.Errorf("%v.Reason() = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestCodexFailureErrorUnwraps(t *testing.T) {
	cause := os.ErrNotExist
	err := &CodexFailureError{Kind: CodexFailureSpawn, Err: cause}
	if got := err.Unwrap(); got != cause {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestDecodeOrderedLimitsPreservesVendorOrder(t *testing.T) {
	raw := []byte(`{"c":{"primary":{"usedPercent":1}},"a":{"primary":{"usedPercent":2}},"b":{"primary":{"usedPercent":3}}}`)
	entries, err := decodeOrderedLimits(raw)
	if err != nil {
		t.Fatalf("decodeOrderedLimits: %v", err)
	}
	want := []string{"c", "a", "b"}
	if len(entries) != len(want) {
		t.Fatalf("entries = %d, want %d", len(entries), len(want))
	}
	for i, id := range want {
		if entries[i].LimitID != id {
			t.Fatalf("entries[%d].LimitID = %q, want %q (JSON key order must be preserved, not sorted)", i, entries[i].LimitID, id)
		}
	}
}

// --- ProbeCodex transport tests, against an injected fake process.
// codexCommandArgs never points at the real "codex" binary in this file.

func withFakeCodex(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-codex.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	previous := codexCommandArgs
	codexCommandArgs = []string{"/bin/sh", path}
	t.Cleanup(func() { codexCommandArgs = previous })
}

func TestProbeCodexHappyPath(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"planType":"pro"}}}'
  fi
done
`)
	result, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeCodex: %v", err)
	}
	if result.AccountID != "acct_fake" || result.Plan != "pro" {
		t.Fatalf("result = %+v, want the fake process's fields", result)
	}
}

func TestProbeCodexServerErrorResponse(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"error":{"code":-32000,"message":"not logged in"}}'
  fi
done
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError", err)
	}
	if failure.Kind != CodexFailureServerError || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want server_error mapping to probe_failed", failure.Kind)
	}
}

func TestProbeCodexMalformedLineWhileWaiting(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2, this is not valid json'
  fi
done
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError", err)
	}
	if failure.Kind != CodexFailureMalformed || failure.Kind.Reason() != ReasonParseFailed {
		t.Fatalf("Kind = %v, want malformed mapping to parse_failed", failure.Kind)
	}
}

func TestProbeCodexExitsBeforeAnswering(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  fi
  if [ "$n" = "3" ]; then
    exit 0
  fi
done
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError", err)
	}
	if failure.Kind != CodexFailureExit || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want exit mapping to probe_failed", failure.Kind)
	}
}

// Codex PR #5 sixth review, P2: a matching response printed just before a
// crash must not be treated as a successful reading -- the process's own
// nonzero exit needs to be surfaced as a failure, not silently discarded.
func TestProbeCodexNonzeroExitAfterResponseIsAFailure(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"planType":"pro"}}}'
    exit 1
  fi
done
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError for a nonzero exit after a matching response", err)
	}
	if failure.Kind != CodexFailureExit {
		t.Fatalf("Kind = %v, want exit", failure.Kind)
	}
}

func TestProbeCodexDeadline(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  fi
done
sleep 30
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 200*time.Millisecond)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError", err)
	}
	if failure.Kind != CodexFailureDeadline || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want deadline mapping to probe_failed", failure.Kind)
	}
}

func TestProbeCodexSpawnFailure(t *testing.T) {
	previous := codexCommandArgs
	codexCommandArgs = []string{filepath.Join(t.TempDir(), "does-not-exist"), "app-server"}
	t.Cleanup(func() { codexCommandArgs = previous })

	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	var failure *CodexFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *CodexFailureError", err)
	}
	if failure.Kind != CodexFailureSpawn || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want spawn mapping to probe_failed", failure.Kind)
	}
}

func TestProbeCodexFailureErrorTextHasNoNilFormatArtifact(t *testing.T) {
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  fi
  if [ "$n" = "3" ]; then
    exit 0
  fi
done
`)
	_, err := ProbeCodex(context.Background(), time.Now(), 5*time.Second)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got == "" || strings.Contains(got, "%!w(<nil>)") {
		t.Fatalf("Error() = %q, must not contain a %%!w(<nil>) artifact", got)
	}
}
