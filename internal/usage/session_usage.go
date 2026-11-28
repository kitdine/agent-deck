package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
)

const maxSessionInvocationPageLimit = 1000

// SessionInvocation is one normalized stored usage event for a logical session.
// It intentionally omits persistent event identity and source metadata.
type SessionInvocation struct {
	Pricing              *PricingReport   `json:"pricing,omitempty"`
	Sequence             int              `json:"sequence"`
	EventAt              string           `json:"event_at"`
	Model                string           `json:"model"`
	Tokens               map[string]int64 `json:"tokens"`
	CatalogBaseCost      *string          `json:"catalog_base_cost"`
	ProviderCost         *string          `json:"provider_cost"`
	KnownCatalogBaseCost string           `json:"known_catalog_base_cost"`
	KnownProviderCost    string           `json:"known_provider_cost"`
	Unpriced             []string         `json:"unpriced_components"`
	Warnings             []string         `json:"warnings"`
}

// InvocationPagination describes one deterministic page of session invocations.
type InvocationPagination struct {
	Page     int  `json:"page"`
	Limit    int  `json:"limit"`
	Total    int  `json:"total"`
	Shown    int  `json:"shown"`
	HasMore  bool `json:"has_more"`
	NextPage int  `json:"next_page,omitempty"`
}

// SessionUsageSummary returns complete aggregate usage for one logical session.
func (s *Service) SessionUsageSummary(ctx context.Context, client, sessionID string) (SessionSummary, error) {
	summary, _, _, err := s.readSessionUsage(ctx, client, sessionID, 1, 0, false, true, false)
	return summary, err
}

// SessionUsagePage keeps the summary and visible invocation page on one
// facts, attribution and catalog snapshot.
func (s *Service) SessionUsagePage(ctx context.Context, client, sessionID string, page, limit int, all bool) (SessionSummary, []SessionInvocation, InvocationPagination, error) {
	if err := validateInvocationPage(page, limit); err != nil {
		return SessionSummary{}, nil, InvocationPagination{}, err
	}
	return s.readSessionUsage(ctx, client, sessionID, page, limit, all, true, true)
}

func (s *Service) SessionInvocations(ctx context.Context, client, sessionID string, page, limit int, all bool) ([]SessionInvocation, InvocationPagination, error) {
	if err := validateInvocationPage(page, limit); err != nil {
		return nil, InvocationPagination{}, err
	}
	_, rows, pagination, err := s.readSessionUsage(ctx, client, sessionID, page, limit, all, false, false)
	return rows, pagination, err
}

func validateInvocationPage(page, limit int) error {
	if page < 1 || limit < 1 || limit > maxSessionInvocationPageLimit {
		return fmt.Errorf("page must be positive and limit must be between 1 and %d", maxSessionInvocationPageLimit)
	}
	return nil
}

func (s *Service) readSessionUsage(ctx context.Context, client, sessionID string, page, limit int, all, includeSummary, includeQuota bool) (SessionSummary, []SessionInvocation, InvocationPagination, error) {
	read, err := s.openPricingRead(ctx, s.now())
	if err != nil {
		return SessionSummary{}, nil, InvocationPagination{}, err
	}
	defer read.tx.Rollback()
	var measurement quota.ValuationInput
	if includeQuota {
		measurement, err = quota.ReadSessionMeasurement(ctx, read.tx, quota.Client(client), sessionID, nil)
		if err != nil {
			return SessionSummary{}, nil, InvocationPagination{}, err
		}
	}
	summary := SessionSummary{Client: client, SessionID: sessionID, Tokens: sessionUsageTokens(), Unpriced: []string{}, Warnings: []string{}}
	invocations := []SessionInvocation{}
	pagination := InvocationPagination{Page: page, Limit: limit}
	err = read.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE client=? AND session_id=?`, client, sessionID).Scan(&pagination.Total)
	if err != nil {
		return SessionSummary{}, nil, InvocationPagination{}, err
	}
	offset, pageLimit := pagination.Total, 0
	if all {
		pagination.Page, pagination.Limit = 1, pagination.Total
		offset, pageLimit = 0, pagination.Total
	} else if limit > 0 && page-1 <= pagination.Total/limit {
		offset, pageLimit = (page-1)*limit, limit
	}
	builder := newUsageSummaryBuilder()
	var prices pricingReportFold
	sequence := 0
	if !includeSummary {
		sequence = offset
	}
	visit := func(event storedEvent, at time.Time) error {
		sequence++
		attribution, priced, err := read.price(event, at)
		if err != nil {
			return err
		}
		if err := legacyPricingError(priced); err != nil {
			return err
		}
		if includeSummary {
			if summary.FirstAt == "" {
				summary.FirstAt = event.EventAt
			}
			summary.LastAt = event.EventAt
			builder.add(event, attribution, priced.Legacy)
			prices.add(event.Client, pricingContribution(priced))
			if measurement.Before != nil && measurement.After != nil {
				measurement.Calls = append(measurement.Calls, quotaCall(priced, at, attribution))
			}
		}
		if sequence > offset && sequence-offset <= pageLimit {
			invocations = append(invocations, sessionInvocation(sequence, event, attribution, priced))
		}
		return nil
	}
	// Keep invocation-only reads bounded to the requested page.
	if pagination.Total > 0 && (includeSummary || pageLimit > 0) {
		filter := InvocationPricingFilter{Client: client, SessionID: sessionID}
		if includeSummary {
			err = read.visit(ctx, filter, visit)
		} else {
			err = read.visitPage(ctx, filter, pageLimit, offset, false, visit)
		}
		if err != nil {
			return SessionSummary{}, nil, InvocationPagination{}, err
		}
	}
	if includeSummary {
		if prices.events > 0 {
			summary.applyUsage(builder.finish())
		}
		summary.Pricing = prices.result(read.rules.digest, false)
	}
	if includeQuota {
		summary.QuotaValuation, err = read.sessionQuotaValuation(ctx, measurement)
		if err != nil {
			return SessionSummary{}, nil, InvocationPagination{}, err
		}
	}
	pagination.Shown = len(invocations)
	pagination.HasMore = offset+pagination.Shown < pagination.Total && pageLimit > 0
	if pagination.HasMore {
		pagination.NextPage = page + 1
	}
	return summary, invocations, pagination, read.tx.Commit()
}

func (s *SessionSummary) applyUsage(summary Summary) {
	s.Tokens, s.CatalogBaseCost, s.ProviderCost = summary.Tokens, summary.CatalogBaseCost, summary.ProviderCost
	s.KnownCatalogBaseCost, s.KnownProviderCost = summary.KnownCatalogBaseCost, summary.KnownProviderCost
	s.Unpriced, s.Warnings = summary.Unpriced, summary.Warnings
}

func sessionInvocation(sequence int, event storedEvent, attribution eventAttribution, priced InvocationPricing) SessionInvocation {
	legacy := priced.Legacy
	warnings := append([]string{}, legacy.Warnings...)
	if attribution.quality != "exact" {
		warnings = append(warnings, attribution.quality+" attribution")
	}
	providerCost, knownProviderCost := legacy.ProviderCost, legacy.KnownProviderCost
	if !attribution.spendEligible {
		providerCost, knownProviderCost = nil, ""
	}
	return SessionInvocation{
		Pricing: invocationReport(priced, attribution), Sequence: sequence, EventAt: event.EventAt, Model: event.Model,
		Tokens: copySessionUsageTokens(event.Tokens), CatalogBaseCost: legacy.CatalogBaseCost, ProviderCost: providerCost,
		KnownCatalogBaseCost: legacy.KnownCatalogBaseCost, KnownProviderCost: knownProviderCost,
		Unpriced: append([]string{}, legacy.Unpriced...), Warnings: warnings,
	}
}

func sessionUsageTokens() map[string]int64 {
	values := make(map[string]int64, len(tokenNames))
	for _, name := range tokenNames {
		values[name] = 0
	}
	return values
}

func copySessionUsageTokens(tokens map[string]int64) map[string]int64 {
	values := sessionUsageTokens()
	for name, value := range tokens {
		values[name] = value
	}
	return values
}
