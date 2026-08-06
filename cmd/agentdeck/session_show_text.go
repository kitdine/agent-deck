package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// renderSessionShowText keeps the text view readable when one or more bounded
// collection pages are empty: session metadata always remains the first block.
func renderSessionShowText(w io.Writer, value session.Result, pagination map[string]session.Pagination, nextCommand string, usageSummary *usage.SessionSummary, invocations []usage.SessionInvocation, activityRequested bool, activityWarning string, sourceStale bool) error {
	width := sessionShowTextWidth(w)
	compact := width <= 80
	if _, err := fmt.Fprintln(w, "SESSION"); err != nil {
		return err
	}
	duration := "—"
	if first, firstErr := time.Parse(time.RFC3339Nano, value.FirstAt); firstErr == nil {
		if last, lastErr := time.Parse(time.RFC3339Nano, value.LastAt); lastErr == nil && !last.Before(first) {
			duration = last.Sub(first).Round(time.Millisecond).String()
		}
	}
	for _, field := range []struct{ label, value string }{
		{"client", value.Client}, {"session", value.SessionID}, {"project", value.Project}, {"model", value.Model},
		{"first", renderDisplayTimeWithZone(value.FirstAt)}, {"last", renderDisplayTimeWithZone(value.LastAt)}, {"duration", duration},
	} {
		if _, err := fmt.Fprintf(w, "%s: %s\n", field.label, sessionShowFit(field.value, max(1, width-runewidth.StringWidth(field.label)-2))); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\nDOCUMENTS"); err != nil {
		return err
	}
	if sourceStale {
		if _, err := fmt.Fprintln(w, "Indexed documents may be stale because the selected source is unavailable."); err != nil {
			return err
		}
	}
	if err := renderSessionShowDocuments(w, value.Documents, width, compact); err != nil {
		return err
	}
	if page, found := pagination["documents"]; found {
		if err := renderPagination(w, page, nextCommand); err != nil {
			return err
		}
	}
	if activityRequested || value.ActivitySummary != nil || len(value.Activity) > 0 {
		if _, err := fmt.Fprintln(w, "\nACTIVITY"); err != nil {
			return err
		}
		if activityWarning != "" {
			if _, err := fmt.Fprintln(w, activityWarning); err != nil {
				return err
			}
		} else {
			if err := renderSessionActivitySummary(w, value.ActivitySummary); err != nil {
				return err
			}
			if err := renderSessionShowActivity(w, value.Activity, width, compact); err != nil {
				return err
			}
			if page, found := pagination["activity"]; found {
				if err := renderPagination(w, page, nextCommand); err != nil {
					return err
				}
			}
		}
	}
	if usageSummary != nil {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if err := renderSessionUsage(w, *usageSummary); err != nil {
			return err
		}
		if err := renderSessionInvocations(w, invocations); err != nil {
			return err
		}
		if page, found := pagination["invocations"]; found {
			return renderPagination(w, page, nextCommand)
		}
	}
	return nil
}

func sessionShowTextWidth(w io.Writer) int {
	width := 100
	if file, ok := w.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if columns, _, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
			width = columns
		}
	}
	if columns, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && columns > 0 {
		width = columns
	}
	return max(1, width)
}

func renderSessionShowDocuments(w io.Writer, values []session.Document, width int, compact bool) error {
	if !compact {
		return renderSessionDocuments(w, values)
	}
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No session documents.")
		return err
	}
	for _, value := range values {
		timestamp := sessionShowFit(renderSessionDocumentTime(value.EventAt), max(1, min(24, width/3)))
		kindBudget := max(1, min(18, width-runewidth.StringWidth(timestamp)-4))
		kind := sessionShowFit(value.Kind, kindBudget)
		prefix := timestamp + " " + kind + ": "
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, sessionShowFit(value.Text, max(1, width-runewidth.StringWidth(prefix)))); err != nil {
			return err
		}
	}
	return nil
}

func renderSessionShowActivity(w io.Writer, values []activity.Detail, width int, compact bool) error {
	if !compact {
		return renderSessionActivity(w, values)
	}
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No safe activity.")
		return err
	}
	for _, value := range values {
		duration := "—"
		if value.DurationMS != nil {
			duration = strconv.FormatInt(*value.DurationMS, 10) + "ms"
		}
		timestamp := sessionShowFit(renderDisplayTime(value.StartedAt), max(1, min(24, width/3)))
		status := sessionShowFit(value.Status, max(1, min(12, width/4)))
		toolBudget := max(1, width-runewidth.StringWidth(timestamp)-runewidth.StringWidth(status)-2)
		if _, err := fmt.Fprintf(w, "%s %s %s\n", timestamp, sessionShowFit(value.Tool, toolBudget), status); err != nil {
			return err
		}
		detailPrefix := "  model: "
		model := sessionShowFit(value.Model, max(1, width-runewidth.StringWidth(detailPrefix)))
		if _, err := fmt.Fprintf(w, "%s%s\n", detailPrefix, model); err != nil {
			return err
		}
		durationPrefix := "  duration: "
		if _, err := fmt.Fprintf(w, "%s%s\n", durationPrefix, sessionShowFit(duration, max(1, width-runewidth.StringWidth(durationPrefix)))); err != nil {
			return err
		}
	}
	return nil
}

func sessionShowFit(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	var result strings.Builder
	used := 0
	for _, r := range value {
		size := runewidth.RuneWidth(r)
		if used+size > width-1 {
			break
		}
		result.WriteRune(r)
		used += size
	}
	return result.String() + "…"
}
