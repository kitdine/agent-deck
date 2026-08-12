package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
)

func TestSessionViewerStateRetainsSectionLocalPageSelectionAndViewport(t *testing.T) {
	var calls []string
	state := newSessionViewerState(func(_ context.Context, section sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		calls = append(calls, fmt.Sprintf("%s:%d", sessionViewerSections[section], page))
		rows := make([]sessionViewerRow, limit)
		for index := range rows {
			rows[index] = sessionViewerRow{Identity: fmt.Sprintf("%d", index), Label: fmt.Sprintf("row-%d", index)}
		}
		return sessionViewerPage{Rows: rows, Page: page, Total: limit * 2}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.apply("down")
	state.viewports[viewerOverview] = 1

	if reload, _ := state.apply("right"); !reload {
		t.Fatal("right did not request lazy section load")
	}
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reload, _ := state.apply("page-down"); !reload {
		t.Fatal("page-down did not request next page")
	}
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.apply("down")
	state.viewports[viewerDocuments] = 1

	if reload, _ := state.apply("left"); !reload {
		t.Fatal("left did not request prior section")
	}
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if state.pages[viewerDocuments] != 2 || state.selected[viewerDocuments] != 1 || state.viewports[viewerDocuments] != 1 {
		t.Fatalf("documents state = page %d selected %d viewport %d", state.pages[viewerDocuments], state.selected[viewerDocuments], state.viewports[viewerDocuments])
	}
	if state.selected[viewerOverview] != 1 || state.viewports[viewerOverview] != 1 {
		t.Fatalf("overview state = selected %d viewport %d", state.selected[viewerOverview], state.viewports[viewerOverview])
	}
	if got, want := strings.Join(calls, ","), "OVERVIEW:1,DOCUMENTS:1,DOCUMENTS:2,OVERVIEW:1"; got != want {
		t.Fatalf("loads = %q, want %q", got, want)
	}
}

func TestSessionViewerLegacyLinesAdapterRemainsBounded(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Lines: []string{"first", "second"}, Page: page, Total: 2}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(state.current.Rows) != 2 || state.current.Rows[1].Label != "second" {
		t.Fatalf("adapted rows = %#v", state.current.Rows)
	}
}

func TestSessionViewerReflowDerivesCapacityAndAnchorsStableIdentity(t *testing.T) {
	var limits []int
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		limits = append(limits, limit)
		start := (page - 1) * limit
		end := min(80, start+limit)
		rows := make([]sessionViewerRow, 0, end-start)
		for index := start; index < end; index++ {
			rows = append(rows, sessionViewerRow{Identity: fmt.Sprintf("stable-%02d", index), Label: fmt.Sprintf("row-%02d", index)})
		}
		return sessionViewerPage{Rows: rows, Page: page, Total: 80}, nil
	})
	ctx := context.Background()
	if err := state.reflow(ctx, sessionViewerAcquisitionLimit(18)); err != nil {
		t.Fatal(err)
	}
	state.selected[viewerOverview] = 10
	state.rememberSelection()
	if err := state.reflow(ctx, sessionViewerAcquisitionLimit(40)); err != nil {
		t.Fatal(err)
	}
	if got := state.current.Rows[state.selected[viewerOverview]].Identity; got != "stable-10" {
		t.Fatalf("wide reflow selected %q, want stable-10", got)
	}
	if err := state.reflow(ctx, sessionViewerAcquisitionLimit(24)); err != nil {
		t.Fatal(err)
	}
	if got := state.current.Rows[state.selected[viewerOverview]].Identity; got != "stable-10" {
		t.Fatalf("standard reflow selected %q, want stable-10", got)
	}
	wantLimits := []int{14, 36, 20}
	if fmt.Sprint(limits) != fmt.Sprint(wantLimits) {
		t.Fatalf("reflow limits = %v, want %v", limits, wantLimits)
	}
	before := len(limits)
	state.apply("down")
	state.viewport(6)
	if len(limits) != before {
		t.Fatalf("selection movement reloaded data: limits=%v", limits)
	}
}

func TestSessionViewerReflowPreservesPageNavigationTarget(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		start := (page - 1) * limit
		end := min(60, start+limit)
		rows := make([]sessionViewerRow, 0, end-start)
		for index := start; index < end; index++ {
			rows = append(rows, sessionViewerRow{Identity: fmt.Sprintf("stable-%02d", index), Label: fmt.Sprintf("row-%02d", index)})
		}
		return sessionViewerPage{Rows: rows, Page: page, Total: 60}, nil
	})
	ctx := context.Background()
	if err := state.reflow(ctx, 14); err != nil {
		t.Fatal(err)
	}
	state.selected[viewerOverview] = 5
	if reload, _ := state.apply("page-down"); !reload {
		t.Fatal("page-down did not request reload")
	}
	if err := state.reflow(ctx, 14); err != nil {
		t.Fatal(err)
	}
	if state.current.Page != 2 || state.current.Rows[state.selected[viewerOverview]].Identity != "stable-14" {
		t.Fatalf("page-down reflow = page %d selected %#v", state.current.Page, state.current.Rows[state.selected[viewerOverview]])
	}
	if reload, _ := state.apply("page-up"); !reload {
		t.Fatal("page-up did not request reload")
	}
	if err := state.reflow(ctx, 14); err != nil {
		t.Fatal(err)
	}
	if state.current.Page != 1 || state.current.Rows[state.selected[viewerOverview]].Identity != "stable-00" {
		t.Fatalf("page-up reflow = page %d selected %#v", state.current.Page, state.current.Rows[state.selected[viewerOverview]])
	}
}

func TestRenderSessionViewerUsesCompleteBodyBudgetWithoutDetail(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		rows := make([]sessionViewerRow, limit)
		for index := range rows {
			rows[index] = sessionViewerRow{Identity: fmt.Sprintf("stable-%02d", index), Label: fmt.Sprintf("record-%02d", index)}
		}
		return sessionViewerPage{Rows: rows, Page: page, Total: len(rows)}, nil
	})
	if err := state.reflow(context.Background(), sessionViewerAcquisitionLimit(18)); err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 80, 18, state); err != nil {
		t.Fatal(err)
	}
	plain := stripSessionViewerANSI(rendered.String())
	if got := strings.Count(plain, "record-"); got != 14 {
		t.Fatalf("18-row viewer rendered %d body records, want 14:\n%s", got, plain)
	}
}

func TestReadSessionViewerKey(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"q", "q"}, {"\r", "enter"}, {"\t", "tab"},
		{"\x1b[A", "up"}, {"\x1b[B", "down"}, {"\x1b[C", "right"}, {"\x1b[D", "left"},
		{"\x1b[H", "home"}, {"\x1b[F", "end"}, {"\x1b[Z", "shift-tab"},
		{"\x1b[5~", "page-up"}, {"\x1b[6~", "page-down"},
	} {
		t.Run(test.want, func(t *testing.T) {
			input, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if _, err := writer.WriteString(test.input); err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			got, _, err := readSessionViewerKey(context.Background(), input, make(chan os.Signal))
			if err != nil || got != test.want {
				t.Fatalf("key = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestReadSessionViewerKeyTreatsStandaloneEscapeAsExit(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	if _, err := writer.WriteString("\x1b"); err != nil {
		t.Fatal(err)
	}
	got, _, err := readSessionViewerKey(context.Background(), input, make(chan os.Signal))
	if err != nil || got != "escape" {
		t.Fatalf("key = %q, %v", got, err)
	}
}

func TestReadSessionViewerKeyHonorsCancellationWithoutReadingInput(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = readSessionViewerKey(ctx, input, make(chan os.Signal))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func TestSessionViewerEscapeWinsOverResize(t *testing.T) {
	if sessionViewerShouldRedrawAfterRead("escape", true) {
		t.Fatal("recognized Escape was discarded for resize")
	}
	if !sessionViewerShouldRedrawAfterRead("", true) {
		t.Fatal("resize without a recognized key did not request redraw")
	}
}

func TestRenderSessionViewerKeepsSelectionVisibleWithoutChangingIt(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		rows := make([]sessionViewerRow, 10)
		for index := range rows {
			rows[index] = sessionViewerRow{Identity: fmt.Sprintf("row-%d", index), Label: fmt.Sprintf("row-%d", index), Detail: terminalDetailModel{notes: []terminalDetailNote{{text: "detail"}}}}
		}
		return sessionViewerPage{Page: page, Total: 10, Rows: rows}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.selected[viewerOverview] = 9
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 48, 10, state); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "> row-9") {
		t.Fatalf("selection not visible:\n%s", rendered.String())
	}
	if state.selected[viewerOverview] != 9 || state.viewports[viewerOverview] == 0 {
		t.Fatalf("selected/viewport = %d/%d", state.selected[viewerOverview], state.viewports[viewerOverview])
	}
}

func TestSessionViewerOverviewOmitsRedundantDetail(t *testing.T) {
	page := sessionViewerOverviewPage(session.Metadata{
		Client:    "codex",
		SessionID: "session-1",
		Model:     "gpt-5.6-sol",
		Project:   "/private/work/agent-deck",
		FirstAt:   "2026-08-01T00:00:00Z",
		LastAt:    "2026-08-01T00:01:00Z",
	})
	for _, row := range page.Rows {
		if len(row.Detail.fields) != 0 || len(row.Detail.notes) != 0 {
			t.Fatalf("overview row %q retained redundant detail: %v", row.Label, row.Detail)
		}
	}
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, _ int, _ int) (sessionViewerPage, error) {
		return page, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 80, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stripSessionViewerANSI(rendered.String()), "DETAIL ·") {
		t.Fatalf("overview rendered an empty detail region:\n%s", rendered.String())
	}
}

func TestRenderSessionViewerUsesBrightSemanticPaletteAndNoColorFallback(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{
			Rows: []sessionViewerRow{{
				Label: "MODEL", Value: "claude-opus", Detail: terminalDetailModel{notes: []terminalDetailNote{{status: "partial", role: terminalDetailRoleWarning}}},
				LabelColor: usageColorSession, ValueColor: usageColorWarning,
			}},
			Summary: []string{"CLAUDE · times in UTC"}, Page: page, Total: 1, Partial: true,
		}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var colorful strings.Builder
	if err := renderSessionViewer(&colorful, 80, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AGENTDECK · SESSION", "[OVERVIEW]", "DETAIL · MODEL", "partial", "\x1b[1;96m", "\x1b[1;95m", "\x1b[1;93m"} {
		if !strings.Contains(colorful.String(), want) {
			t.Fatalf("color render missing %q:\n%s", want, colorful.String())
		}
	}
	if !strings.Contains(colorful.String(), "\x1b[1;93mPARTIAL\x1b[0m") {
		t.Fatalf("detail value did not use warning color:\n%s", colorful.String())
	}

	var plain strings.Builder
	if err := renderSessionViewer(&plain, 80, 24, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]+m`).MatchString(plain.String()) {
		t.Fatalf("no-color render contains SGR: %q", plain.String())
	}
	for _, want := range []string{"[OVERVIEW]", "> MODEL", "partial"} {
		if !strings.Contains(plain.String(), want) {
			t.Fatalf("no-color render missing %q:\n%s", want, plain.String())
		}
	}
}

func TestRenderSessionViewerEmptyStatusDoesNotClaimComplete(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Empty: "No normalized token invocations are indexed for this session.", Page: page}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 80, 24, state); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "· empty") || strings.Contains(rendered.String(), "· complete") {
		t.Fatalf("empty status is ambiguous:\n%s", rendered.String())
	}
}

func TestRenderSessionViewerFitsResponsiveGeometries(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		rows := make([]sessionViewerRow, 20)
		for index := range rows {
			rows[index] = sessionViewerRow{
				Label:  fmt.Sprintf("文档-%02d · claude", index),
				Value:  "a deliberately long selected value",
				Detail: terminalDetailModel{notes: []terminalDetailNote{{text: strings.Repeat("detail with CJK 内容 and emoji 🧭 ", 8)}}},
			}
		}
		return sessionViewerPage{Rows: rows, Summary: []string{"20 approved rows"}, Page: page, Total: 40}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{48, 10}, {60, 18}, {80, 24}, {100, 24}, {120, 32}, {140, 32}, {180, 40}} {
		var rendered strings.Builder
		if err := renderSessionViewer(&rendered, size[0], size[1], state, usageTextPrimitives{color: true}); err != nil {
			t.Fatalf("%dx%d render: %v", size[0], size[1], err)
		}
		plain := stripSessionViewerANSI(rendered.String())
		lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
		if len(lines) > size[1] {
			t.Fatalf("%dx%d emitted %d visual lines", size[0], size[1], len(lines))
		}
		for index, line := range lines {
			visible := statsVisibleWidth(line)
			if visible > size[0] {
				t.Fatalf("%dx%d line %d width %d: %q", size[0], size[1], index, visible, line)
			}
			if size[0] == 180 && visible > 120 {
				t.Fatalf("180x40 Session canvas line %d width %d, want <= 120: %q", index, visible, line)
			}
		}
	}
}

func TestRenderSessionViewerReportsWriteFailure(t *testing.T) {
	want := errors.New("terminal write failed")
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Rows: []sessionViewerRow{{Label: "row"}}, Page: page, Total: 1}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := renderSessionViewer(sessionViewerErrorWriter{err: want}, 80, 24, state); !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
}

type sessionViewerErrorWriter struct{ err error }

func (w sessionViewerErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func stripSessionViewerANSI(value string) string {
	value = strings.ReplaceAll(value, "\x1b[H\x1b[2J", "")
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(value, "")
}

var _ io.Writer = sessionViewerErrorWriter{}
