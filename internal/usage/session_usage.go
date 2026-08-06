package usage

import (
	"context"
	"fmt"
)

const maxSessionInvocationPageLimit = 1000

// SessionInvocation is one normalized stored usage event for a logical session.
// It intentionally omits persistent event identity and source metadata.
type SessionInvocation struct {
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
// An indexed session with no usage events returns a zero-valued aggregate rather
// than treating absence of usage as an error.
func (s *Service) SessionUsageSummary(ctx context.Context, client, sessionID string) (SessionSummary, error) {
	events, err := s.events(ctx, client, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	result := SessionSummary{
		Client:    client,
		SessionID: sessionID,
		Tokens:    sessionUsageTokens(),
		Unpriced:  []string{},
		Warnings:  []string{},
	}
	if len(events) == 0 {
		return result, nil
	}
	summary, err := s.summarize(ctx, events)
	if err != nil {
		return SessionSummary{}, err
	}
	result.FirstAt = events[0].EventAt
	result.LastAt = events[len(events)-1].EventAt
	result.Tokens = summary.Tokens
	result.CatalogBaseCost = summary.CatalogBaseCost
	result.ProviderCost = summary.ProviderCost
	result.KnownCatalogBaseCost = summary.KnownCatalogBaseCost
	result.KnownProviderCost = summary.KnownProviderCost
	result.Unpriced = summary.Unpriced
	result.Warnings = summary.Warnings
	return result, nil
}

// SessionInvocations returns a deterministic chronological page of the session's
// normalized stored usage events. Sequence numbers are positions in the complete
// session order, not persistent identifiers.
func (s *Service) SessionInvocations(ctx context.Context, client, sessionID string, page, limit int, all bool) ([]SessionInvocation, InvocationPagination, error) {
	if page < 1 || limit < 1 || limit > maxSessionInvocationPageLimit {
		return nil, InvocationPagination{}, fmt.Errorf("page must be positive and limit must be between 1 and %d", maxSessionInvocationPageLimit)
	}
	total, err := s.sessionInvocationCount(ctx, client, sessionID)
	if err != nil {
		return nil, InvocationPagination{}, err
	}
	pagination := InvocationPagination{Page: page, Limit: limit, Total: total}
	offset, queryLimit := 0, limit
	if all {
		pagination.Page, pagination.Limit = 1, total
		queryLimit = total
	} else {
		if page-1 > total/limit {
			return []SessionInvocation{}, pagination, nil
		}
		offset = (page - 1) * limit
	}
	if total == 0 {
		return []SessionInvocation{}, pagination, nil
	}
	events, err := s.sessionInvocationEventsPage(ctx, client, sessionID, queryLimit, offset)
	if err != nil {
		return nil, InvocationPagination{}, err
	}
	pagination.Shown = len(events)
	pagination.HasMore = offset+len(events) < total
	if pagination.HasMore {
		pagination.NextPage = page + 1
	}
	resolver, err := s.loadReadPriceResolver(ctx, s.now())
	if err != nil {
		return nil, InvocationPagination{}, err
	}
	invocations := make([]SessionInvocation, 0, len(events))
	for index, event := range events {
		attribution, err := resolver.priceForEvent(event)
		if err != nil {
			return nil, InvocationPagination{}, err
		}
		priced, err := Calculate(event.Client, event.Model, event.Tokens, attribution.price, attribution.multiplier)
		if err != nil {
			return nil, InvocationPagination{}, err
		}
		warnings := append([]string{}, priced.Warnings...)
		if attribution.quality != "exact" {
			warnings = append(warnings, attribution.quality+" attribution")
		}
		invocations = append(invocations, SessionInvocation{
			Sequence:             offset + index + 1,
			EventAt:              event.EventAt,
			Model:                event.Model,
			Tokens:               copySessionUsageTokens(event.Tokens),
			CatalogBaseCost:      priced.CatalogBaseCost,
			ProviderCost:         priced.ProviderCost,
			KnownCatalogBaseCost: priced.KnownCatalogBaseCost,
			KnownProviderCost:    priced.KnownProviderCost,
			Unpriced:             append([]string{}, priced.Unpriced...),
			Warnings:             warnings,
		})
	}
	return invocations, pagination, nil
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

func (s *Service) sessionInvocationCount(ctx context.Context, client, sessionID string) (int, error) {
	var count int
	err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE client=? AND session_id=?`, client, sessionID).Scan(&count)
	return count, err
}

func (s *Service) sessionInvocationEventsPage(ctx context.Context, client, sessionID string, limit, offset int) ([]storedEvent, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.source_path,e.source_offset,COALESCE(b.run_id,e.run_id),r.exact,r.multiplier,r.provider,r.started_at,us.first_at FROM usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key LEFT JOIN usage_runs r ON r.id=COALESCE(b.run_id,e.run_id) LEFT JOIN usage_sessions us ON us.client=e.client AND us.session_id=e.session_id WHERE e.client=? AND e.session_id=? ORDER BY e.event_at,e.event_key LIMIT ? OFFSET ?`, client, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]storedEvent, 0, limit)
	for rows.Next() {
		var event storedEvent
		var input, cachedInput, output, cacheRead, cacheCreation, cacheWrite5m, cacheWrite1h int64
		if err := rows.Scan(&event.Key, &event.Client, &event.SessionID, &event.EventID, &event.EventAt, &event.Model, &input, &cachedInput, &output, &cacheRead, &cacheCreation, &cacheWrite5m, &cacheWrite1h, &event.SourcePath, &event.SourceOffset, &event.runID, &event.runExact, &event.runMultiplier, &event.runProvider, &event.runStart, &event.sessionStart); err != nil {
			return nil, err
		}
		event.Tokens = map[string]int64{
			"input_tokens":          input,
			"cached_input_tokens":   cachedInput,
			"output_tokens":         output,
			"cache_read_tokens":     cacheRead,
			"cache_creation_tokens": cacheCreation,
			"cache_write_5m_tokens": cacheWrite5m,
			"cache_write_1h_tokens": cacheWrite1h,
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
