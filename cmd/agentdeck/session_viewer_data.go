package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func newSessionViewerLoad(ctx context.Context, database *store.Store, metadata session.Metadata, service *usage.Service) sessionViewerLoad {
	return func(_ context.Context, section sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		switch section {
		case viewerOverview:
			return sessionViewerPage{Page: 1, Total: 1, Lines: []string{
				"client: " + metadata.Client, "session: " + metadata.SessionID, "project: " + metadata.Project,
				"model: " + metadata.Model, "first: " + renderDisplayTimeWithZone(metadata.FirstAt), "last: " + renderDisplayTimeWithZone(metadata.LastAt),
			}}, nil
		case viewerDocuments:
			documents, pagination, err := session.DocumentsPage(ctx, database.DB, metadata, page, limit, false)
			if err != nil {
				return sessionViewerPage{}, err
			}
			lines := make([]string, 0, len(documents))
			for _, document := range documents {
				lines = append(lines, renderSessionDocumentTime(document.EventAt)+" "+document.Kind+": "+oneLine(document.Text))
			}
			_, sourceErr := os.Stat(metadata.SourcePath)
			return sessionViewerPage{Lines: lines, Page: pagination.Page, Total: pagination.Total, Stale: os.IsNotExist(sourceErr)}, nil
		case viewerActivity:
			result, err := activity.ReadDetailsPage(metadata.SourcePath, metadata.Client, metadata.SessionID, page, limit, false)
			if err != nil {
				return sessionViewerPage{Page: page, Warning: "Activity source is unavailable.", Stale: os.IsNotExist(err)}, nil
			}
			lines := []string{fmt.Sprintf("%d calls · %d completed · %d failed · %d incomplete", result.Summary.Total, result.Summary.Completed, result.Summary.Failed, result.Summary.Incomplete)}
			for _, detail := range result.Details {
				duration := "—"
				if detail.DurationMS != nil {
					duration = strconv.FormatInt(*detail.DurationMS, 10) + "ms"
				}
				lines = append(lines, renderDisplayTime(detail.StartedAt)+" "+detail.Tool+" "+detail.Model+" "+detail.Status+" "+duration)
			}
			return sessionViewerPage{Lines: lines, Page: result.Page, Total: result.Total}, nil
		case viewerTokens:
			if service == nil {
				return sessionViewerPage{Page: page, Warning: "Token data is unavailable."}, nil
			}
			summary, err := service.SessionUsageSummary(ctx, metadata.Client, metadata.SessionID)
			if err != nil {
				return sessionViewerPage{Page: page, Warning: "Token data is unavailable.", Partial: true}, nil
			}
			invocations, pagination, err := service.SessionInvocations(ctx, metadata.Client, metadata.SessionID, page, limit, false)
			if err != nil {
				return sessionViewerPage{Page: page, Warning: "Token data is unavailable.", Partial: true}, nil
			}
			lines := []string{fmt.Sprintf("input: %d · output: %d", summary.Tokens["input_tokens"], summary.Tokens["output_tokens"])}
			for _, invocation := range invocations {
				lines = append(lines, fmt.Sprintf("%d %s %s", invocation.Sequence, renderDisplayTime(invocation.EventAt), invocation.Model))
			}
			return sessionViewerPage{Lines: lines, Page: pagination.Page, Total: pagination.Total, Warning: textList(summary.Warnings)}, nil
		default:
			return sessionViewerPage{}, fmt.Errorf("unknown viewer section")
		}
	}
}
