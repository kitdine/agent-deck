package main

import (
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/mattn/go-runewidth"
)

func TestRenderSessionShowTextKeepsMetadataForEmptyPagedSections(t *testing.T) {
	var output strings.Builder
	result := session.Result{Metadata: session.Metadata{
		Client: "codex", SessionID: "session-1", Project: "project", Model: "gpt-safe",
		FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:03Z",
	}}
	err := renderSessionShowText(&output, result, map[string]session.Pagination{
		"documents": {Page: 3, Limit: 1, Total: 2},
		"activity":  {Page: 3, Limit: 1, Total: 2},
	}, "agentdeck session show --client codex session-1 --activity --page 4", nil, nil, true, "Activity source is unavailable.", false)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"SESSION", "session: session-1", "duration: 3s", "DOCUMENTS", "No session documents.", "ACTIVITY", "Activity source is unavailable."} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered text missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSessionShowTextUsesCompactDocumentRowsAtNarrowWidth(t *testing.T) {
	t.Setenv("COLUMNS", "60")
	var output strings.Builder
	err := renderSessionShowText(&output, session.Result{
		Metadata:  session.Metadata{Client: "codex", SessionID: "session-1", FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:01Z"},
		Documents: []session.Document{{EventAt: "2026-08-01T00:00:00Z", Kind: "user_prompt", Text: strings.Repeat("visible ", 12)}},
	}, nil, "", nil, nil, false, "", false)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Contains(text, "| CLIENT") || !strings.Contains(text, "user_prompt:") || !strings.Contains(text, "…") {
		t.Fatalf("narrow render = %q", text)
	}
}

func TestRenderSessionShowTextRespectsNarrowVisibleWidths(t *testing.T) {
	for _, width := range []string{"48", "60", "80"} {
		t.Run(width, func(t *testing.T) {
			t.Setenv("COLUMNS", width)
			var output strings.Builder
			duration := int64(12)
			err := renderSessionShowText(&output, session.Result{
				Metadata:  session.Metadata{Client: "codex", SessionID: "session-1", Project: "项目😀项目😀项目😀", Model: "模型😀模型😀", FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:01Z"},
				Documents: []session.Document{{EventAt: "2026-08-01T00:00:00Z", Kind: "user_prompt", Text: "可见😀文本可见😀文本可见😀文本可见😀文本可见😀文本"}},
				Activity:  []activity.Detail{{StartedAt: "2026-08-01T00:00:00Z", Tool: "工具😀工具😀工具😀", Model: "模型😀模型😀模型😀", Status: "completed", DurationMS: &duration}},
			}, nil, "", nil, nil, true, "", false)
			if err != nil {
				t.Fatal(err)
			}
			limit := 0
			for _, r := range width {
				limit = limit*10 + int(r-'0')
			}
			for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
				if got := runewidth.StringWidth(line); got > limit {
					t.Fatalf("line width %d exceeds %d: %q", got, limit, line)
				}
			}
		})
	}
}

func TestRenderSessionShowTextUsesWideTables(t *testing.T) {
	t.Setenv("COLUMNS", "140")
	var output strings.Builder
	err := renderSessionShowText(&output, session.Result{Metadata: session.Metadata{Client: "codex", SessionID: "session-1", FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:01Z"}, Documents: []session.Document{{Kind: "user_prompt", Text: "visible"}}}, nil, "", nil, nil, false, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "| CLIENT") {
		t.Fatalf("wide render did not use document table:\n%s", output.String())
	}
}
