// Package desktop owns the versioned, privacy-bounded desktop snapshot contract.
package desktop

import (
	"context"
	"errors"
	"fmt"
	pathpkg "path"
	"sort"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	WireVersion        = 1
	DefaultRecentLimit = 5
	MaxRecentLimit     = 20
	RefreshInterval    = 5 * time.Minute
)

var (
	ErrUnsupportedWireVersion = errors.New("unsupported_wire_version")
	ErrInvalidRecentLimit     = errors.New("invalid_recent_limit")
)

type Request struct {
	WireVersion int
	RecentLimit int
}

func (r Request) Validate() error {
	if r.WireVersion != WireVersion {
		return fmt.Errorf("%w: got %d, supported %d", ErrUnsupportedWireVersion, r.WireVersion, WireVersion)
	}
	if r.RecentLimit < 1 || r.RecentLimit > MaxRecentLimit {
		return fmt.Errorf("%w: got %d, want 1-%d", ErrInvalidRecentLimit, r.RecentLimit, MaxRecentLimit)
	}
	return nil
}

type Snapshot struct {
	WireVersion   int              `json:"wire_version"`
	GeneratedAt   string           `json:"generated_at"`
	NextRefreshAt string           `json:"next_refresh_at"`
	Provider      ProviderSnapshot `json:"provider"`
	Usage         UsageSnapshot    `json:"usage"`
	Sessions      SessionsSnapshot `json:"sessions"`
	Health        HealthSnapshot   `json:"health"`
}

type ProviderSnapshot struct {
	Available  bool                `json:"available"`
	Routes     []ProviderRoute     `json:"routes"`
	Candidates []ProviderCandidate `json:"candidates"`
}

type ProviderRoute struct {
	Client     string `json:"client"`
	Provider   string `json:"provider"`
	SelectedAt string `json:"selected_at"`
	ViaWrapper bool   `json:"via_wrapper"`
}

type ProviderCandidate struct {
	Provider    string                        `json:"provider"`
	BuiltIn     bool                          `json:"built_in"`
	Clients     []string                      `json:"clients"`
	Credentials []ProviderCandidateCredential `json:"credentials"`
	HasWrapper  bool                          `json:"has_wrapper"`
	Ready       bool                          `json:"ready"`
	Options     []ProviderSwitchOption        `json:"options"`
}

type ProviderCandidateCredential struct {
	Name    string   `json:"name"`
	Clients []string `json:"clients"`
	Present bool     `json:"present"`
}

type ProviderSwitchOption struct {
	Client     string  `json:"client"`
	Provider   string  `json:"provider"`
	Credential *string `json:"credential"`
	ViaWrapper bool    `json:"via_wrapper"`
	Ready      bool    `json:"ready"`
	ReasonCode *string `json:"reason_code"`
}

type UsageSnapshot struct {
	Available            bool                     `json:"available"`
	From                 string                   `json:"from"`
	To                   string                   `json:"to"`
	Tokens               map[string]int64         `json:"tokens"`
	Counts               map[string]int64         `json:"counts"`
	CatalogBaseCost      *string                  `json:"catalog_base_cost"`
	ProviderCost         *string                  `json:"provider_cost"`
	KnownCatalogBaseCost *string                  `json:"known_catalog_base_cost"`
	KnownProviderCost    *string                  `json:"known_provider_cost"`
	PricingComplete      bool                     `json:"pricing_complete"`
	UnpricedComponents   int                      `json:"unpriced_components"`
	Warnings             []string                 `json:"warnings"`
	Presentation         usage.PresentationReport `json:"presentation"`
}

type SessionsSnapshot struct {
	Available bool            `json:"available"`
	Total     int             `json:"total"`
	Periods   SessionsPeriods `json:"periods"`
	Items     []RecentSession `json:"items"`
}

// SessionsPeriods carries the per-period statistics the Sessions panel reads
// under a period filter. The recent list below it stays a recent list: the host
// never derives these figures from it, because a bounded list of the newest
// sessions cannot answer a question about a period.
type SessionsPeriods struct {
	Available bool                 `json:"available"`
	Items     []SessionsPeriodItem `json:"items"`
}

type SessionsPeriodItem struct {
	Period                string                `json:"period"`
	Client                string                `json:"client"`
	Sessions              int                   `json:"sessions"`
	TotalDurationSeconds  int64                 `json:"total_duration_seconds"`
	MedianDurationSeconds int64                 `json:"median_duration_seconds"`
	DistinctProjects      int                   `json:"distinct_projects"`
	Projects              []SessionsProjectItem `json:"projects"`
}

type SessionsProjectItem struct {
	Project         string `json:"project,omitempty"`
	Sessions        int    `json:"sessions"`
	DurationSeconds int64  `json:"duration_seconds"`
}

type RecentSession struct {
	Client    string `json:"client"`
	SessionID string `json:"session_id"`
	Project   string `json:"project,omitempty"`
	Model     string `json:"model,omitempty"`
	FirstAt   string `json:"first_at,omitempty"`
	LastAt    string `json:"last_at,omitempty"`
}

type HealthSnapshot struct {
	Available bool          `json:"available"`
	Status    string        `json:"status,omitempty"`
	Healthy   bool          `json:"healthy"`
	Problems  int           `json:"problems"`
	Warnings  int           `json:"warnings"`
	Errors    int           `json:"errors"`
	Checks    []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Code     string `json:"code,omitempty"`
	Count    int    `json:"count,omitempty"`
	Recovery string `json:"recovery_command,omitempty"`
}

type Service struct {
	StateRoot string
	Home      string
	Workdir   string
	Vault     provider.CredentialVault
	Now       func() time.Time
	Location  *time.Location
}

type Result struct {
	Snapshot Snapshot
	Partial  bool
	Warnings []string
}

func (s Service) Build(ctx context.Context, request Request) (Result, error) {
	if err := request.Validate(); err != nil {
		return Result{}, err
	}
	now := s.now().UTC()
	result := Result{Snapshot: Snapshot{
		WireVersion:   request.WireVersion,
		GeneratedAt:   now.Format(time.RFC3339Nano),
		NextRefreshAt: now.Add(RefreshInterval).Format(time.RFC3339Nano),
		Provider:      ProviderSnapshot{Routes: []ProviderRoute{}, Candidates: []ProviderCandidate{}},
		Usage:         emptyUsageSnapshot(now, s.location()),
		Sessions:      emptySessionsSnapshot(),
		Health:        HealthSnapshot{Checks: []HealthCheck{}},
	}}

	core, err := store.OpenReadOnly(ctx, s.StateRoot)
	if err != nil {
		result.warn("provider_unavailable")
		result.warn("usage_unavailable")
	} else {
		s.loadProvider(ctx, core, &result)
		s.loadUsage(ctx, core, now, &result)
		if closeErr := core.Close(); closeErr != nil {
			result.warn("state_close_failed")
		}
	}
	s.loadSessions(ctx, request.RecentLimit, now, &result)
	s.loadHealth(ctx, &result)
	sort.Strings(result.Warnings)
	return result, nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) location() *time.Location {
	if s.Location != nil {
		return s.Location
	}
	return time.Local
}

func (r *Result) warn(code string) {
	r.Partial = true
	for _, existing := range r.Warnings {
		if existing == code {
			return
		}
	}
	r.Warnings = append(r.Warnings, code)
}

func (s Service) loadProvider(ctx context.Context, core *store.Store, result *Result) {
	service := provider.Service{Store: core}
	values, err := service.Current(ctx)
	if err != nil {
		result.warn("provider_unavailable")
		return
	}
	snapshot := providerSnapshot(values)
	candidates, err := providerCandidates(ctx, service, values)
	if err != nil {
		result.Snapshot.Provider = snapshot
		result.warn("provider_candidates_unavailable")
		return
	}
	snapshot.Candidates = candidates
	result.Snapshot.Provider = snapshot
}

func providerSnapshot(values []provider.CurrentSelection) ProviderSnapshot {
	routes := make([]ProviderRoute, 0, len(values))
	for _, value := range values {
		routes = append(routes, ProviderRoute{
			Client: value.Client, Provider: value.Provider,
			SelectedAt: value.SelectedAt, ViaWrapper: value.ViaWrapper,
		})
	}
	return ProviderSnapshot{Available: true, Routes: routes, Candidates: []ProviderCandidate{}}
}

func providerCandidates(ctx context.Context, service provider.Service, current []provider.CurrentSelection) ([]ProviderCandidate, error) {
	definitions, err := service.List(ctx)
	if err != nil {
		return nil, err
	}
	currentByClient := map[string]provider.CurrentSelection{}
	for _, selection := range current {
		currentByClient[selection.Client] = selection
	}
	candidates := make([]ProviderCandidate, 0, len(definitions))
	for _, item := range definitions {
		definition := item.Definition
		clients := providerCandidateClients(definition.Clients)
		credentials, err := service.ListCredentials(ctx, definition.Name, "")
		if err != nil {
			return nil, err
		}
		candidate := ProviderCandidate{
			Provider: definition.Name, BuiltIn: definition.BuiltIn, Clients: clients,
			Credentials: []ProviderCandidateCredential{}, HasWrapper: definition.WrapperURL != "",
			Options: []ProviderSwitchOption{},
		}
		for _, credential := range credentials {
			bindings := append([]string(nil), credential.Clients...)
			sort.Slice(bindings, func(i, j int) bool { return providerClientLess(bindings[i], bindings[j]) })
			candidate.Credentials = append(candidate.Credentials, ProviderCandidateCredential{Name: credential.Name, Clients: bindings, Present: credential.Present})
		}
		candidate.Options = providerCandidateOptions(candidate, currentByClient)
		for _, option := range candidate.Options {
			candidate.Ready = candidate.Ready || option.Ready
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func providerCandidateClients(mappings []store.ClientMapping) []string {
	seen := map[string]bool{}
	clients := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		if !seen[mapping.Client] {
			seen[mapping.Client] = true
			clients = append(clients, mapping.Client)
		}
	}
	sort.Slice(clients, func(i, j int) bool { return providerClientLess(clients[i], clients[j]) })
	return clients
}

func providerClientLess(left, right string) bool {
	order := map[string]int{"codex": 0, "claude": 1}
	leftOrder, leftKnown := order[left]
	rightOrder, rightKnown := order[right]
	if leftKnown && rightKnown {
		return leftOrder < rightOrder
	}
	if leftKnown != rightKnown {
		return leftKnown
	}
	return left < right
}

func providerCandidateOptions(candidate ProviderCandidate, current map[string]provider.CurrentSelection) []ProviderSwitchOption {
	credentials := candidate.Credentials
	if candidate.BuiltIn || len(credentials) == 0 {
		credentials = []ProviderCandidateCredential{{}}
	}
	options := make([]ProviderSwitchOption, 0, len(candidate.Clients)*len(credentials)*2)
	for _, client := range candidate.Clients {
		for _, credential := range credentials {
			var credentialName *string
			if credential.Name != "" {
				copy := credential.Name
				credentialName = &copy
			}
			for _, viaWrapper := range []bool{false, true} {
				option := ProviderSwitchOption{Client: client, Provider: candidate.Provider, Credential: credentialName, ViaWrapper: viaWrapper}
				reason := providerOptionReason(candidate, credential, client, viaWrapper, current[client])
				if reason == "" {
					option.Ready = true
				} else {
					option.ReasonCode = &reason
				}
				options = append(options, option)
			}
		}
	}
	return options
}

func providerOptionReason(candidate ProviderCandidate, credential ProviderCandidateCredential, client string, viaWrapper bool, current provider.CurrentSelection) string {
	if !candidate.BuiltIn {
		if credential.Name == "" {
			return "credential_missing"
		}
		bound := false
		for _, value := range credential.Clients {
			bound = bound || value == client
		}
		if !bound {
			return "credential_client_mismatch"
		}
		if !credential.Present {
			return "credential_missing"
		}
	}
	if viaWrapper && !candidate.HasWrapper {
		return "wrapper_not_configured"
	}
	credentialName := ""
	if !candidate.BuiltIn {
		credentialName = credential.Name
	}
	if current.Client == client && current.Provider == candidate.Provider && current.Credential == credentialName && current.ViaWrapper == viaWrapper {
		return "already_selected"
	}
	return ""
}

func emptyUsageSnapshot(now time.Time, location *time.Location) UsageSnapshot {
	from, to := localDay(now, location)
	return UsageSnapshot{
		From: from.Format(time.RFC3339), To: to.Format(time.RFC3339),
		Tokens: map[string]int64{}, Counts: map[string]int64{}, Warnings: []string{}, Presentation: usage.EmptyPresentationReport(),
	}
}

func (s Service) loadUsage(ctx context.Context, core *store.Store, now time.Time, result *Result) {
	from, to := localDay(now, s.location())
	service := usage.New(core, s.Home)
	service.Now = func() time.Time { return now }
	presentation, err := service.Presentation(ctx, now, s.location())
	if err != nil {
		result.warn("usage_unavailable")
		return
	}
	result.Snapshot.Usage = usageSnapshot(presentation.Summary, from, to)
	result.Snapshot.Usage.Presentation = presentation
}

func usageSnapshot(summary usage.Summary, from, to time.Time) UsageSnapshot {
	return UsageSnapshot{
		Available: true, From: from.Format(time.RFC3339), To: to.Format(time.RFC3339),
		Tokens: nonNilMap(summary.Tokens), Counts: nonNilMap(summary.Counts),
		CatalogBaseCost: summary.CatalogBaseCost, ProviderCost: summary.ProviderCost,
		KnownCatalogBaseCost: summary.KnownCatalogBaseCost, KnownProviderCost: summary.KnownProviderCost,
		PricingComplete: len(summary.Unpriced) == 0, UnpricedComponents: len(summary.Unpriced),
		Warnings: nonNilStrings(summary.Warnings), Presentation: usage.EmptyPresentationReport(),
	}
}

func localDay(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return from, from.AddDate(0, 0, 1)
}

func (s Service) loadSessions(ctx context.Context, limit int, now time.Time, result *Result) {
	index, err := store.OpenSessionsReadOnly(ctx, s.StateRoot)
	if err != nil {
		result.warn("sessions_unavailable")
		return
	}
	defer func() {
		if closeErr := index.Close(); closeErr != nil {
			result.warn("sessions_close_failed")
		}
	}()
	values, err := session.List(ctx, index.DB)
	if err != nil {
		result.warn("sessions_unavailable")
		return
	}
	result.Snapshot.Sessions = sessionsSnapshot(values, limit, now, s.location())
}

func sessionsSnapshot(values []session.Metadata, limit int, now time.Time, location *time.Location) SessionsSnapshot {
	items := make([]RecentSession, 0, min(limit, len(values)))
	for _, value := range values[:min(limit, len(values))] {
		items = append(items, RecentSession{
			Client: value.Client, SessionID: value.SessionID,
			Project: projectLabel(value.Project), Model: value.Model,
			FirstAt: value.FirstAt, LastAt: value.LastAt,
		})
	}
	return SessionsSnapshot{
		Available: true, Total: len(values),
		Periods: sessionsPeriods(values, now, location), Items: items,
	}
}

// emptySessionsSnapshot is the shape an unavailable session index reports. Every
// collection is a non-null empty array because `encoding/json` encodes a nil
// slice as `null`, and a decoder that accepts a missing additive family still
// rejects a present one whose `items` is null.
func emptySessionsSnapshot() SessionsSnapshot {
	return SessionsSnapshot{
		Periods: SessionsPeriods{Items: []SessionsPeriodItem{}},
		Items:   []RecentSession{},
	}
}

// sessionsPeriods emits one record for every supported period and client scope,
// in a fixed order, so a payload always carries the record a filter selects.
// A session belongs to a period when its last event falls inside it.
func sessionsPeriods(values []session.Metadata, now time.Time, location *time.Location) SessionsPeriods {
	if location == nil {
		location = time.Local
	}
	today, tomorrow := localDay(now, location)
	periods := []struct {
		name  string
		start time.Time
	}{
		{name: "today", start: today},
		{name: "7d", start: today.AddDate(0, 0, -6)},
		{name: "30d", start: today.AddDate(0, 0, -29)},
	}
	items := make([]SessionsPeriodItem, 0, len(periods)*3)
	for _, period := range periods {
		for _, client := range []string{"all", "codex", "claude"} {
			items = append(items, sessionsPeriodItem(values, period.name, period.start, tomorrow, client, location))
		}
	}
	return SessionsPeriods{Available: true, Items: items}
}

// The interval is half-open on the local calendar, `start <= last < end`, where
// end is the next local midnight. Without the upper bound a future-dated session
// counts in today, 7d and 30d alike, inflating every figure the panel states.
// Both bounds come from calendar arithmetic on a local start-of-day, so a period
// still spans the intended number of days across a DST transition.
func sessionsPeriodItem(values []session.Metadata, period string, start, end time.Time, client string, location *time.Location) SessionsPeriodItem {
	durations := make([]int64, 0, len(values))
	projects := map[string]struct{}{}
	type projectAccumulator struct {
		label    string
		sessions int
		duration int64
	}
	projectTotals := map[string]*projectAccumulator{}
	var total int64
	for _, value := range values {
		if client != "all" && value.Client != client {
			continue
		}
		last, err := time.Parse(time.RFC3339Nano, value.LastAt)
		if err != nil {
			continue
		}
		local := last.In(location)
		if local.Before(start) || !local.Before(end) {
			continue
		}
		first, firstErr := time.Parse(time.RFC3339Nano, value.FirstAt)
		duration := int64(0)
		if firstErr == nil && last.After(first) {
			duration = int64(last.Sub(first) / time.Second)
		}
		durations = append(durations, duration)
		total += duration
		// Ordinary checkouts keep their normalized full identity. ChatGPT Work
		// conversation directories are ephemeral one-conversation workspaces,
		// so their stable product identity is the shared ChatGPT Work bucket.
		identity, label := sessionProjectGroup(value.Project)
		if identity != "" {
			projects[identity] = struct{}{}
		}
		if identity == "" {
			identity = "\x00"
		}
		project := projectTotals[identity]
		if project == nil {
			project = &projectAccumulator{label: label}
			projectTotals[identity] = project
		}
		project.sessions++
		project.duration += duration
	}
	projectItems := make([]SessionsProjectItem, 0, len(projectTotals))
	for _, project := range projectTotals {
		projectItems = append(projectItems, SessionsProjectItem{
			Project: project.label, Sessions: project.sessions, DurationSeconds: project.duration,
		})
	}
	sort.Slice(projectItems, func(i, j int) bool {
		if projectItems[i].DurationSeconds != projectItems[j].DurationSeconds {
			return projectItems[i].DurationSeconds > projectItems[j].DurationSeconds
		}
		return projectItems[i].Project < projectItems[j].Project
	})
	return SessionsPeriodItem{
		Period: period, Client: client, Sessions: len(durations),
		TotalDurationSeconds: total, MedianDurationSeconds: medianDuration(durations),
		DistinctProjects: len(projects), Projects: projectItems,
	}
}

func medianDuration(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// projectIdentity normalizes a project path without discarding its parents, so
// it can identify a project. projectLabel below projects it to a basename for
// display, which is exactly why the two must not share one call site.
func projectIdentity(value string) string {
	return strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
}

func projectLabel(value string) string {
	value = projectIdentity(value)
	if value == "" {
		return ""
	}
	label := pathpkg.Base(value)
	if label == "." || label == "/" {
		return ""
	}
	return label
}

func sessionProjectGroup(value string) (identity, label string) {
	identity = projectIdentity(value)
	label = projectLabel(value)
	if strings.HasPrefix(strings.ToLower(label), "referenced-chatgpt-conversation-") {
		return "chatgpt-work://referenced-conversation", "ChatGPT Work"
	}
	return identity, label
}

func (s Service) loadHealth(ctx context.Context, result *Result) {
	report, err := (doctor.Service{
		StateRoot: s.StateRoot, Home: s.Home, Workdir: s.Workdir, Vault: s.Vault,
	}).Check(ctx, false)
	if err != nil {
		result.warn("health_unavailable")
		return
	}
	result.Snapshot.Health = healthSnapshot(report)
}

func healthSnapshot(report doctor.Report) HealthSnapshot {
	checks := make([]HealthCheck, 0, len(report.Checks))
	for _, check := range report.Checks {
		checks = append(checks, HealthCheck{
			Name: check.Name, Status: check.Status, Code: check.Code,
			Count: check.Count, Recovery: check.Recovery,
		})
	}
	return HealthSnapshot{
		Available: true, Status: report.Status, Healthy: report.Healthy,
		Problems: report.Problems, Warnings: report.Warnings, Errors: report.Errors,
		Checks: checks,
	}
}

func nonNilMap(values map[string]int64) map[string]int64 {
	if values == nil {
		return map[string]int64{}
	}
	return values
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
