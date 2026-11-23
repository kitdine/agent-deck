package activity

import (
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// CodexDetails projects activity from the selected thread's proved capture.
// The selected capture must match the ingest-bound reduction. The caller
// owns final revalidation. This entry reads no file.
func CodexDetails(thread string, capture ingest.CodexPrefixProof, reduction ingest.CodexCapturedReduction) []Detail {
	details, _, _ := collectCodexDetails(thread, capture, reduction, 0, true)
	return details
}

func visitCodexDetails(thread string, capture ingest.CodexPrefixProof, reduction ingest.CodexCapturedReduction, visit func(int64, Detail)) {
	source := reduction.Source()
	progress := capture.Progress()
	if !reduction.MatchesCapture(capture) || thread == "" || source.State != ingest.CodexSourceVerified ||
		source.ThreadID != thread || progress.CompleteEnd == 0 {
		return
	}

	starts := make(map[ingest.CodexSpan]Detail)
	flush := func() {
		for span, detail := range starts {
			visit(span.Start, detail)
		}
		clear(starts)
	}
	segmentIndex := 0
	segment, hasSegment := reduction.Segment(segmentIndex)
	for proved := range reduction.Records() {
		record := proved.Record
		span := record.Span
		if record.Kind != ingest.CodexToolStart && record.Kind != ingest.CodexToolEnd {
			continue
		}
		if !proved.Local() || proved.ThreadID != thread ||
			span.Start < 0 || span.End <= span.Start || span.End > progress.CompleteEnd || record.At.IsZero() {
			continue
		}
		for hasSegment && span.Start >= segment.Span.End {
			segmentIndex++
			segment, hasSegment = reduction.Segment(segmentIndex)
			flush()
		}
		if !hasSegment || segment.ThreadID != thread ||
			span.Start < segment.Span.Start || span.End > segment.Span.End {
			continue
		}
		switch record.Kind {
		case ingest.CodexToolStart:
			starts[span] = Detail{
				Client: "codex", SessionID: thread, Model: ingest.CodexModelLabel(proved.Model),
				Tool: ingest.CodexToolLabel(record.Tool), StartedAt: record.At.UTC().Format(time.RFC3339Nano),
				Status: "started",
			}
		case ingest.CodexToolEnd:
			if proved.ToolStart == nil || proved.ToolStart.End > span.Start {
				continue
			}
			detail, found := starts[*proved.ToolStart]
			if !found {
				continue
			}
			delete(starts, *proved.ToolStart)
			detail.CompletedAt, detail.Status = record.At.UTC().Format(time.RFC3339Nano), "completed"
			at, err := time.Parse(time.RFC3339Nano, detail.StartedAt)
			if err == nil && !record.At.Before(at) {
				duration := record.At.Sub(at).Milliseconds()
				detail.DurationMS = &duration
			}
			visit(proved.ToolStart.Start, detail)
		}
	}
	flush()
}

type rankedCodexDetail struct {
	start  int64
	detail Detail
}

func codexDetailBefore(left, right rankedCodexDetail) bool {
	if detailBefore(left.detail, right.detail) {
		return true
	}
	return !detailBefore(right.detail, left.detail) && left.start < right.start
}

func collectCodexDetails(thread string, capture ingest.CodexPrefixProof, reduction ingest.CodexCapturedReduction, keep int, all bool) ([]Detail, Summary, int) {
	selected := make([]rankedCodexDetail, 0, keep)
	summary, durations := Summary{ByTool: map[string]int{}}, 0
	visitCodexDetails(thread, capture, reduction, func(start int64, detail Detail) {
		if detail.DurationMS != nil {
			durations++
		}
		summarize(&summary, detail)
		item := rankedCodexDetail{start: start, detail: detail}
		if all {
			selected = append(selected, item)
			return
		}
		at := sort.Search(len(selected), func(i int) bool { return codexDetailBefore(item, selected[i]) })
		if at >= keep {
			return
		}
		if len(selected) < keep {
			selected = append(selected, rankedCodexDetail{})
		}
		copy(selected[at+1:], selected[at:len(selected)-1])
		selected[at] = item
	})
	if all {
		sort.Slice(selected, func(i, j int) bool { return codexDetailBefore(selected[i], selected[j]) })
	}
	details := make([]Detail, len(selected))
	for i, item := range selected {
		details[i] = item.detail
	}
	return details, summary, durations
}

// CodexDetailsPage uses the existing activity pagination and full-capture
// summary over the same proved details as CodexDetails.
func CodexDetailsPage(thread string, capture ingest.CodexPrefixProof, reduction ingest.CodexCapturedReduction, page, limit int, all bool) (Page, error) {
	keep, err := activityPageWindow(page, limit, all)
	if err != nil {
		return Page{}, err
	}
	details, summary, durations := collectCodexDetails(thread, capture, reduction, keep, all)
	return finishActivityPage(details, summary, durations, page, limit, all), nil
}
