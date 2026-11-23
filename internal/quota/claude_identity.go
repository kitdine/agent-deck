package quota

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"time"
	"unicode/utf8"
)

const (
	DefaultClaudeAuthTimeout = 4 * time.Second
	claudeProbeWaitDelay     = 500 * time.Millisecond
	maxClaudeAuthOutput      = 64 << 10
	claudeSubscriptionKind   = "claude.ai"
)

// ClaudeProbe keeps both invocations on the same executable, environment and
// working directory. The caller decides which are due and commits identity
// before starting usage.
type ClaudeProbe struct {
	path      string
	env       []string
	dir       string
	usageArgs []string
}

func NewClaudeProbe() (*ClaudeProbe, error) {
	path, err := claudeCommandPath(claudeProseCommandArgs)
	if err != nil {
		return nil, claudeSpawnError(err)
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, &ClaudeFailureError{Kind: ClaudeFailureSpawn}
	}
	return &ClaudeProbe{path: path, dir: dir, env: os.Environ(), usageArgs: slices.Clone(claudeProseCommandArgs[1:])}, nil
}

// ProbeIdentity only loads the local key for a comparable subscription
// response. The coordinator calls this after the quota gate permits a probe.
func (p *ClaudeProbe) ProbeIdentity(ctx context.Context, stateRoot string) IdentityObservation {
	return p.probeIdentity(ctx, stateRoot, DefaultClaudeAuthTimeout)
}

func (p *ClaudeProbe) probeIdentity(ctx context.Context, stateRoot string, timeout time.Duration) IdentityObservation {
	var stdout claudeAuthOutput
	err := p.run(ctx, timeout, &stdout, "auth", "status", "--json")
	if err != nil {
		// The captured 2.1.325 sample only proves successful signed-in exit.
		// A nonzero status is not evidence of a signed-out account.
		return IdentityObservation{Failure: IdentityProbeFailed}
	}
	if stdout.overflow {
		return IdentityObservation{Failure: IdentityParseFailed}
	}
	return classifyClaudeIdentity(stdout.data.Bytes(), func(email, org string) (*claudeComparison, error) {
		key, err := loadClaudeIdentityKey(stateRoot)
		if err != nil {
			return nil, err
		}
		return deriveClaudeComparison(key, claudeSubscriptionKind, email, org), nil
	})
}

func (p *ClaudeProbe) ProbeUsage(ctx context.Context, observedAt time.Time) (ClaudeProseResult, error) {
	return p.probeUsage(ctx, observedAt, DefaultClaudeProseTimeout)
}

func (p *ClaudeProbe) probeUsage(ctx context.Context, observedAt time.Time, timeout time.Duration) (ClaudeProseResult, error) {
	var stdout bytes.Buffer
	if err := p.run(ctx, timeout, &stdout, p.usageArgs...); err != nil {
		return ClaudeProseResult{}, err
	}
	result, err := ParseClaudeProse(stdout.String(), observedAt)
	if err != nil {
		return ClaudeProseResult{}, &ClaudeFailureError{Kind: ClaudeFailureParse, Err: err}
	}
	return result, nil
}

func (p *ClaudeProbe) run(ctx context.Context, timeout time.Duration, stdout io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return &ClaudeFailureError{Kind: ClaudeFailureDeadline, Err: err}
	}
	cmd := exec.CommandContext(ctx, p.path, args...)
	cmd.Env, cmd.Dir = p.env, p.dir
	cmd.Stdout, cmd.Stderr = stdout, io.Discard
	cmd.WaitDelay = claudeProbeWaitDelay
	if err := cmd.Start(); err != nil {
		if cause := ctx.Err(); cause != nil {
			return &ClaudeFailureError{Kind: ClaudeFailureDeadline, Err: cause}
		}
		return claudeSpawnError(err)
	}
	err := cmd.Wait()
	if cause := ctx.Err(); cause != nil {
		return &ClaudeFailureError{Kind: ClaudeFailureDeadline, Err: cause}
	}
	if err != nil {
		return &ClaudeFailureError{Kind: ClaudeFailureExit}
	}
	return nil
}

// Keep caller-facing sentinel classification without carrying an executable
// name, path, or other text from the process error.
func claudeSpawnError(err error) *ClaudeFailureError {
	result := &ClaudeFailureError{Kind: ClaudeFailureSpawn}
	for _, cause := range []error{exec.ErrDot, exec.ErrNotFound, os.ErrNotExist, os.ErrPermission, errClaudeEmptyCommand} {
		if errors.Is(err, cause) {
			result.Err = cause
			break
		}
	}
	return result
}

type claudeAuthOutput struct {
	data     bytes.Buffer
	overflow bool
}

func (w *claudeAuthOutput) Write(p []byte) (int, error) {
	n := min(len(p), maxClaudeAuthOutput-w.data.Len())
	_, _ = w.data.Write(p[:n])
	w.overflow = w.overflow || n < len(p)
	return len(p), nil
}

type claudeAuthStatus struct {
	loggedIn                               *bool
	authMethod, apiProvider, email         *string
	orgID, orgName, planTier, subscription *string
}

func parseClaudeAuthStatus(data []byte) (claudeAuthStatus, bool) {
	var status claudeAuthStatus
	if !utf8.Valid(data) {
		return status, false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return status, false
	}
	fields := map[string]any{
		"loggedIn": &status.loggedIn, "authMethod": &status.authMethod, "apiProvider": &status.apiProvider,
		"email": &status.email, "orgId": &status.orgID, "orgName": &status.orgName,
		"planTier": &status.planTier, "subscriptionType": &status.subscription,
	}
	for d.More() {
		name, err := d.Token()
		if err != nil {
			return status, false
		}
		key := name.(string) // JSON object keys are strings.
		field, known := fields[key]
		if known {
			if field == nil || d.Decode(field) != nil {
				return status, false
			}
			fields[key] = nil // A repeated known field is ambiguous.
		} else {
			var ignored json.RawMessage
			if d.Decode(&ignored) != nil {
				return status, false
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return status, false
	}
	_, err = d.Token()
	return status, err == io.EOF && status.loggedIn != nil
}

func classifyClaudeIdentity(data []byte, compare func(email, org string) (*claudeComparison, error)) IdentityObservation {
	status, ok := parseClaudeAuthStatus(data)
	if !ok {
		return IdentityObservation{Failure: IdentityParseFailed}
	}
	if !*status.loggedIn {
		return IdentityObservation{State: IdentitySignedOut}
	}
	method, provider := statusString(status.authMethod), statusString(status.apiProvider)
	switch provider {
	case "bedrock", "vertex", "foundry":
		return IdentityObservation{State: IdentityNonSubscription}
	}
	if method == "api_key" {
		return IdentityObservation{State: IdentityNonSubscription}
	}
	if method != claudeSubscriptionKind || provider != "firstParty" {
		return IdentityObservation{Failure: IdentityIncomparable}
	}
	email, local, masked, ok := maskedIdentityEmail(statusString(status.email))
	org := statusString(status.orgID)
	// No approved fixture yet establishes a positive personal-account shape.
	if !ok || org == "" {
		return IdentityObservation{Failure: IdentityIncomparable}
	}
	comparison, err := compare(email, "org:"+org)
	if err != nil {
		return IdentityObservation{Failure: IdentityIncomparable}
	}
	forbidden := newIdentityForbiddenSet(local, statusString(status.email), org)
	return IdentityObservation{
		State: IdentityIdentified, accountKind: claudeSubscriptionKind, claude: comparison,
		Display: IdentityDisplay{
			EmailMasked: masked, OrganizationKind: OrgOrganization,
			OrganizationName: sanitizeIdentityName(statusString(status.orgName), forbidden),
			Plan: PlanSources{
				planTier:         sanitizeIdentityPlan(statusString(status.planTier), forbidden),
				subscriptionType: sanitizeIdentityPlan(statusString(status.subscription), forbidden),
			},
		},
	}
}

func statusString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
