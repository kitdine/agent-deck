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
	Available bool            `json:"available"`
	Routes    []ProviderRoute `json:"routes"`
}

type ProviderRoute struct {
	Client     string `json:"client"`
	Provider   string `json:"provider"`
	SelectedAt string `json:"selected_at"`
	ViaWrapper bool   `json:"via_wrapper"`
}

type UsageSnapshot struct {
	Available            bool             `json:"available"`
	From                 string           `json:"from"`
	To                   string           `json:"to"`
	Tokens               map[string]int64 `json:"tokens"`
	Counts               map[string]int64 `json:"counts"`
	CatalogBaseCost      *string          `json:"catalog_base_cost"`
	ProviderCost         *string          `json:"provider_cost"`
	KnownCatalogBaseCost *string          `json:"known_catalog_base_cost"`
	KnownProviderCost    *string          `json:"known_provider_cost"`
	PricingComplete      bool             `json:"pricing_complete"`
	UnpricedComponents   int              `json:"unpriced_components"`
	Warnings             []string         `json:"warnings"`
}

type SessionsSnapshot struct {
	Available bool            `json:"available"`
	Total     int             `json:"total"`
	Items     []RecentSession `json:"items"`
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
		Provider:      ProviderSnapshot{Routes: []ProviderRoute{}},
		Usage:         emptyUsageSnapshot(now, s.location()),
		Sessions:      SessionsSnapshot{Items: []RecentSession{}},
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
	s.loadSessions(ctx, request.RecentLimit, &result)
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
	values, err := (provider.Service{Store: core}).Current(ctx)
	if err != nil {
		result.warn("provider_unavailable")
		return
	}
	result.Snapshot.Provider = providerSnapshot(values)
}

func providerSnapshot(values []provider.CurrentSelection) ProviderSnapshot {
	routes := make([]ProviderRoute, 0, len(values))
	for _, value := range values {
		routes = append(routes, ProviderRoute{
			Client: value.Client, Provider: value.Provider,
			SelectedAt: value.SelectedAt, ViaWrapper: value.ViaWrapper,
		})
	}
	return ProviderSnapshot{Available: true, Routes: routes}
}

func emptyUsageSnapshot(now time.Time, location *time.Location) UsageSnapshot {
	from, to := localDay(now, location)
	return UsageSnapshot{
		From: from.Format(time.RFC3339), To: to.Format(time.RFC3339),
		Tokens: map[string]int64{}, Counts: map[string]int64{}, Warnings: []string{},
	}
}

func (s Service) loadUsage(ctx context.Context, core *store.Store, now time.Time, result *Result) {
	from, to := localDay(now, s.location())
	summary, err := usage.New(core, s.Home).SummaryRange(ctx, from, to)
	if err != nil {
		result.warn("usage_unavailable")
		return
	}
	result.Snapshot.Usage = usageSnapshot(summary, from, to)
}

func usageSnapshot(summary usage.Summary, from, to time.Time) UsageSnapshot {
	return UsageSnapshot{
		Available: true, From: from.Format(time.RFC3339), To: to.Format(time.RFC3339),
		Tokens: nonNilMap(summary.Tokens), Counts: nonNilMap(summary.Counts),
		CatalogBaseCost: summary.CatalogBaseCost, ProviderCost: summary.ProviderCost,
		KnownCatalogBaseCost: summary.KnownCatalogBaseCost, KnownProviderCost: summary.KnownProviderCost,
		PricingComplete: len(summary.Unpriced) == 0, UnpricedComponents: len(summary.Unpriced),
		Warnings: nonNilStrings(summary.Warnings),
	}
}

func localDay(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return from, from.AddDate(0, 0, 1)
}

func (s Service) loadSessions(ctx context.Context, limit int, result *Result) {
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
	result.Snapshot.Sessions = sessionsSnapshot(values, limit)
}

func sessionsSnapshot(values []session.Metadata, limit int) SessionsSnapshot {
	items := make([]RecentSession, 0, min(limit, len(values)))
	for _, value := range values[:min(limit, len(values))] {
		items = append(items, RecentSession{
			Client: value.Client, SessionID: value.SessionID,
			Project: projectLabel(value.Project), Model: value.Model,
			FirstAt: value.FirstAt, LastAt: value.LastAt,
		})
	}
	return SessionsSnapshot{Available: true, Total: len(values), Items: items}
}

func projectLabel(value string) string {
	value = strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if value == "" {
		return ""
	}
	label := pathpkg.Base(value)
	if label == "." || label == "/" {
		return ""
	}
	return label
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
