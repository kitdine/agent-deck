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
			rows[index] = sessionViewerRow{Label: fmt.Sprintf("row-%d", index), Detail: []string{"detail"}}
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

func TestRenderSessionViewerUsesBrightSemanticPaletteAndNoColorFallback(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{
			Rows: []sessionViewerRow{{
				Label: "MODEL", Value: "claude-opus", Detail: []string{"PRICING STATUS partial"},
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
				Detail: []string{strings.Repeat("detail with CJK 内容 and emoji 🧭 ", 8)},
			}
		}
		return sessionViewerPage{Rows: rows, Summary: []string{"20 approved rows"}, Page: page, Total: 40}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{48, 10}, {60, 12}, {80, 24}, {120, 24}, {140, 32}} {
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
			if width := statsVisibleWidth(line); width > size[0] {
				t.Fatalf("%dx%d line %d width %d: %q", size[0], size[1], index, width, line)
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
