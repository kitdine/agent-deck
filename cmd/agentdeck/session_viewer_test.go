package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSessionViewerStateUsesIndependentLazyPages(t *testing.T) {
	var calls []string
	state := newSessionViewerState(func(_ context.Context, section sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		calls = append(calls, sessionViewerSections[section]+":"+string(rune('0'+page)))
		return sessionViewerPage{Lines: []string{"row-1", "row-2"}, Page: page, Total: limit * 2}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
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
	if got, want := strings.Join(calls, ","), "OVERVIEW:1,DOCUMENTS:1,DOCUMENTS:2"; got != want {
		t.Fatalf("loads = %q, want %q", got, want)
	}
	if reload, _ := state.apply("left"); !reload {
		t.Fatal("left did not request prior section")
	}
	if state.pages[viewerDocuments] != 2 {
		t.Fatalf("documents page = %d, want 2", state.pages[viewerDocuments])
	}
}

func TestReadSessionViewerKey(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"q", "q"}, {"\t", "tab"}, {"\x1b[A", "up"}, {"\x1b[B", "down"}, {"\x1b[C", "right"}, {"\x1b[D", "left"}, {"\x1b[Z", "shift-tab"}, {"\x1b[5~", "page-up"}, {"\x1b[6~", "page-down"},
	} {
		t.Run(test.want, func(t *testing.T) {
			input, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writer.WriteString(test.input); err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			defer input.Close()
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

func TestRenderSessionViewerKeepsSelectionInViewport(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Page: page, Total: 10, Lines: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.selected = 9
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 48, 9, state); err != nil {
		t.Fatal(err)
	}
	text := rendered.String()
	if !strings.Contains(text, "> 9") || strings.Contains(text, "\n  0\n") {
		t.Fatalf("viewport = %q", text)
	}
}

func TestSessionViewerStateControlsAndRender(t *testing.T) {
	state := newSessionViewerState(func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Lines: []string{"first", "second"}, Page: page, Total: 2}, nil
	})
	if err := state.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.apply("down")
	if state.selected != 1 {
		t.Fatalf("selected = %d", state.selected)
	}
	state.apply("home")
	state.apply("end")
	if state.selected != 1 {
		t.Fatalf("selected after home/end = %d", state.selected)
	}
	if _, exit := state.apply("escape"); !exit {
		t.Fatal("escape did not exit")
	}
	var rendered strings.Builder
	if err := renderSessionViewer(&rendered, 48, 24, state); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SESSION VIEWER", "SECTION: OVERVIEW", "> second", "pgup/pgdn"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("render missing %q:\n%s", want, rendered.String())
		}
	}
}
