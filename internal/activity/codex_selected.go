package activity

import (
	"context"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// ReadSelectedCodexDetails is the on-demand entry for callers with an
// authoritative selected source. The legacy path reader remains separate.
func ReadSelectedCodexDetails(ctx context.Context, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection, limits ingest.CodexSelectedLimits) ([]Detail, error) {
	var details []Detail
	err := ingest.VisitSelectedCodexReduction(ctx, selected, current, limits, func(r ingest.CodexCapturedReduction) error {
		details = CodexDetails(selected.ThreadID, r.Capture(), r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return details, nil
}

func ReadSelectedCodexDetailsPage(ctx context.Context, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection, limits ingest.CodexSelectedLimits, page, limit int, all bool) (Page, error) {
	keep, err := activityPageWindow(page, limit, all)
	if err != nil {
		return Page{}, err
	}
	var result Page
	err = ingest.VisitSelectedCodexReduction(ctx, selected, current, limits, func(r ingest.CodexCapturedReduction) error {
		details, summary, durations := collectCodexDetails(selected.ThreadID, r.Capture(), r, keep, all)
		result = finishActivityPage(details, summary, durations, page, limit, all)
		return nil
	})
	if err != nil {
		return Page{}, err
	}
	return result, nil
}
