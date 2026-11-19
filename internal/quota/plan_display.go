package quota

const (
	PlanSourceClaudeAuthStatus PlanSource = "claude_auth_status"
	PlanSourceCodexAccountRead PlanSource = "codex_account_read"
	PlanSourceCodexRateLimits  PlanSource = "codex_rate_limits"

	PlanKnown    PlanState = "known"
	PlanRawOnly  PlanState = "raw_only"
	PlanConflict PlanState = "conflict"
	PlanMissing  PlanState = "missing"

	PlanBasisMapping PlanBasis = "mapping"
	PlanBasisRaw     PlanBasis = "raw"
)

// ProjectPlan turns approved plan source facts into presentation values.
// Only safe vendor tokens appear in Raw.
func ProjectPlan(source PlanSource, sources PlanSources) PlanDisplay {
	result := PlanDisplay{Source: source, State: PlanMissing, Reason: PlanNotReported, Raw: []PlanRaw{}}
	fields := []struct {
		name string
		planField
	}{{"planType", sources.planType}}
	if source == PlanSourceClaudeAuthStatus {
		fields = []struct {
			name string
			planField
		}{{"planTier", sources.planTier}, {"subscriptionType", sources.subscriptionType}}
	}
	for _, field := range fields {
		if field.rejected || (field.value != "" && !safePlanToken(field.value)) {
			result.Reason = PlanUnsafeValue
			result.Raw = []PlanRaw{}
			return result
		}
		if field.value != "" {
			result.Raw = append(result.Raw, PlanRaw{Field: field.name, Value: field.value})
		}
	}
	if len(result.Raw) == 0 {
		return result
	}
	result.Reason = ""
	raw := result.Raw[0].Value
	if len(result.Raw) == 2 && raw != result.Raw[1].Value {
		result.State = PlanConflict
		return result
	}
	result.State, result.Basis, result.Name, result.ShortName = PlanRawOnly, PlanBasisRaw, raw, raw
	if source == PlanSourceClaudeAuthStatus && raw == "pro" {
		result.State, result.Basis, result.Name, result.ShortName = PlanKnown, PlanBasisMapping, "Claude Pro", "Pro"
	}
	return result
}
