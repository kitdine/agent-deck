package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	terminaloutput "github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"golang.org/x/term"
)

// sessionBrowserPage is one committed scope selection shown by the browser.
type sessionBrowserPage struct {
	scope   string
	items   []session.Metadata
	classes []string
	unknown int
	// coverageGaps are the selection's classification coverage gap reasons;
	// non-empty marks the page partial, as in the text and JSON outputs.
	coverageGaps []string
	// indexEmpty distinguishes an empty index from an empty selection.
	indexEmpty bool
}

func newSessionBrowserPage(scope string, snapshot sessionorigin.Snapshot) sessionBrowserPage {
	page := sessionBrowserPage{scope: scope, unknown: snapshot.UnknownCount, coverageGaps: selectionCoverageGaps(snapshot)}
	for _, value := range snapshot.Sessions {
		page.items = append(page.items, value.Metadata)
		page.classes = append(page.classes, value.Origin.Effective)
	}
	return page
}

var sessionBrowserScopes = []string{sessionorigin.ScopeWork, sessionorigin.ScopeAuxiliary, sessionorigin.ScopeAll}

type sessionBrowserState struct {
	items    []session.Metadata
	classes  []string
	selected int
	// scope is empty for an unscoped browser, which offers no scope menu.
	scope        string
	unknown      int
	coverageGaps []string
	indexEmpty   bool
	// menuOpen shows the scope menu; menuIndex is its highlighted entry.
	menuOpen  bool
	menuIndex int
	// pendingScope is the scope the menu applied, awaiting a load.
	pendingScope string
	// failedScope is a requested scope whose load failed; the previous list
	// and scope stay shown until Escape or another choice.
	failedScope string
	// untrusted is set while no scope has ever loaded: there is no previous
	// list to keep, so a failed load shows the error page for
	// unavailableScope instead of an invented empty list.
	untrusted        bool
	unavailableScope string
}

func newSessionBrowserState(items []session.Metadata) *sessionBrowserState {
	return &sessionBrowserState{items: items}
}

func newScopedSessionBrowserState(page sessionBrowserPage) *sessionBrowserState {
	state := &sessionBrowserState{}
	state.replace(page)
	return state
}

// replace shows a newly loaded scope, keeping the selected canonical session
// when it is still visible and selecting the first session otherwise.
func (s *sessionBrowserState) replace(page sessionBrowserPage) {
	var previous *session.Metadata
	if s.selected < len(s.items) {
		item := s.items[s.selected]
		previous = &item
	}
	s.items, s.classes, s.scope, s.unknown, s.indexEmpty = page.items, page.classes, page.scope, page.unknown, page.indexEmpty
	s.coverageGaps = page.coverageGaps
	s.selected, s.failedScope = 0, ""
	s.untrusted, s.unavailableScope = false, ""
	if previous == nil {
		return
	}
	for index, item := range s.items {
		if item.Client == previous.Client && item.SessionID == previous.SessionID {
			s.selected = index
			return
		}
	}
}

func (s *sessionBrowserState) class(index int) string {
	if index < len(s.classes) {
		return s.classes[index]
	}
	return ""
}

func (s *sessionBrowserState) apply(key string, viewport int) (open, exit bool) {
	if key == "q" {
		return false, true
	}
	if s.menuOpen {
		switch key {
		case "escape":
			s.menuOpen = false
		case "up":
			s.menuIndex = max(0, s.menuIndex-1)
		case "down":
			s.menuIndex = min(len(sessionBrowserScopes)-1, s.menuIndex+1)
		case "enter":
			s.menuOpen = false
			s.pendingScope = sessionBrowserScopes[s.menuIndex]
		}
		return false, false
	}
	last := max(0, len(s.items)-1)
	page := max(1, viewport)
	switch key {
	case "escape":
		if s.failedScope != "" {
			s.failedScope = ""
			return false, false
		}
		return false, true
	case "s":
		if s.scope != "" {
			s.menuOpen = true
			s.menuIndex = max(0, slices.Index(sessionBrowserScopes, s.scope))
		}
	case "enter":
		return len(s.items) > 0, false
	case "up":
		s.selected = max(0, s.selected-1)
	case "down":
		s.selected = min(last, s.selected+1)
	case "home":
		s.selected = 0
	case "end":
		s.selected = last
	case "page-up":
		s.selected = max(0, s.selected-page)
	case "page-down":
		s.selected = min(last, s.selected+page)
	}
	return false, false
}

func (s *sessionBrowserState) viewport(width, height int) (start, end, size int) {
	compact := sessionBrowserLayoutFor(sessionBrowserCanvasWidth(width)).compact
	fixedRows, recordRows := 6, 1
	if compact {
		// The compact header and every compact record use two complete lines.
		fixedRows, recordRows = 7, 2
	}
	if len(s.coverageGaps) > 0 {
		// The coverage warning line sits above the list.
		fixedRows++
	}
	size = max(1, (height-fixedRows)/recordRows)
	start = max(0, min(s.selected-size/2, len(s.items)-size))
	end = min(len(s.items), start+size)
	return start, end, size
}

type sessionBrowserDetailLoad func(session.Metadata) sessionViewerLoad

// sessionBrowserScopeLoad reads one scope's committed selection. It never
// scans and must stop when ctx is cancelled; a nil loader disables the scope
// menu.
type sessionBrowserScopeLoad func(ctx context.Context, scope string) (sessionBrowserPage, error)

// sessionBrowserLoadDrain bounds how long exit waits for a cancelled scope
// read to return before the terminal is restored. A loader that honors its
// context returns well within it; one that does not cannot hold the
// terminal hostage.
const sessionBrowserLoadDrain = 500 * time.Millisecond

// sessionBrowserLoad is one in-flight scope read. Its result is accepted only
// while it is still the browser's current load: a cancelled or superseded
// load's late result is discarded with it.
type sessionBrowserLoad struct {
	scope  string
	cancel context.CancelFunc
	result chan sessionBrowserLoadResult
	ready  chan struct{}
}

type sessionBrowserLoadResult struct {
	page sessionBrowserPage
	err  error
}

func startSessionBrowserLoad(ctx context.Context, load sessionBrowserScopeLoad, scope string) *sessionBrowserLoad {
	loadCtx, cancel := context.WithCancel(ctx)
	current := &sessionBrowserLoad{scope: scope, cancel: cancel, result: make(chan sessionBrowserLoadResult, 1), ready: make(chan struct{})}
	go func() {
		page, err := load(loadCtx, scope)
		current.result <- sessionBrowserLoadResult{page: page, err: err}
		close(current.ready)
	}()
	return current
}

// stop cancels the read and waits a bounded time for it to return.
func (l *sessionBrowserLoad) stop() {
	l.cancel()
	select {
	case <-l.ready:
	case <-time.After(sessionBrowserLoadDrain):
	}
}

func (l *sessionBrowserLoad) finished() bool {
	select {
	case <-l.ready:
		return true
	default:
		return false
	}
}

func runSessionBrowser(
	ctx context.Context,
	input, output *os.File,
	initial sessionBrowserPage,
	scopeLoad sessionBrowserScopeLoad,
	detailLoad sessionBrowserDetailLoad,
	noColor ...bool,
) error {
	state := newScopedSessionBrowserState(initial)
	if scopeLoad == nil {
		state.scope = ""
	}
	return runSessionBrowserState(ctx, input, output, state, "", scopeLoad, detailLoad, noColor...)
}

// runSessionBrowserLoadingFirst opens the browser before any scope has been
// read and loads scope inside the terminal owner, so the first read is
// cancellable like any other and its failure is an exitable error page
// rather than a command error.
func runSessionBrowserLoadingFirst(
	ctx context.Context,
	input, output *os.File,
	scope string,
	scopeLoad sessionBrowserScopeLoad,
	detailLoad sessionBrowserDetailLoad,
	noColor ...bool,
) error {
	state := &sessionBrowserState{scope: scope, untrusted: true}
	return runSessionBrowserState(ctx, input, output, state, scope, scopeLoad, detailLoad, noColor...)
}

func runSessionBrowserState(
	ctx context.Context,
	input, output *os.File,
	state *sessionBrowserState,
	firstLoad string,
	scopeLoad sessionBrowserScopeLoad,
	detailLoad sessionBrowserDetailLoad,
	noColor ...bool,
) (err error) {
	// While no scope has loaded, leaving the error page reports the read
	// failure once the terminal is restored, so its diagnosis reaches stderr
	// and the exit status does not claim a result was shown.
	var unavailableErr error
	defer func() {
		if err == nil && state.untrusted && unavailableErr != nil {
			err = unavailableErr
		}
	}()
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("session --interactive requires TTY stdin and stdout")
	}
	if os.Getenv("TERM") == "dumb" {
		return errors.New("session --interactive requires a usable terminal")
	}

	disableColor := len(noColor) > 0 && noColor[0]
	p := newUsageTextPrimitives(output, disableColor)
	terminal, err := startInteractiveTerminal(input, output)
	if err != nil {
		return err
	}
	defer terminal.Close()

	frame := terminal.frameWriter()
	// A scope read runs off the input loop so q, Escape and resize stay live
	// while it loads. Deferred after terminal.Close, the drain runs first:
	// the read is cancelled and collected before the terminal is restored.
	var loading *sessionBrowserLoad
	defer func() {
		if loading != nil {
			loading.stop()
		}
	}()
	if firstLoad != "" && scopeLoad != nil {
		loading = startSessionBrowserLoad(ctx, scopeLoad, firstLoad)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height := 100, 24
		if columns, rows, sizeErr := term.GetSize(int(output.Fd())); sizeErr == nil && columns > 0 && rows > 0 {
			width, height = columns, rows
		}
		if loading != nil {
			if err := renderSessionBrowserLoading(frame, width, height, loading.scope, p); err != nil {
				return err
			}
			key, err := readSessionBrowserKeyWhileLoading(ctx, input, terminal.resized, loading)
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			switch {
			case key == "q":
				return nil
			case key == "escape" && state.untrusted:
				// Without a previous list there is nothing to return to.
				return nil
			case key == "escape":
				// Cancel only the load; the previous list and scope remain.
				loading.stop()
				loading = nil
			case loading.finished():
				result := <-loading.result
				if result.err != nil && state.untrusted {
					state.unavailableScope, unavailableErr = loading.scope, result.err
				} else if result.err != nil {
					state.failedScope = loading.scope
				} else {
					state.replace(result.page)
				}
				loading = nil
			}
			continue
		}
		if err := renderSessionBrowser(frame, width, height, state, p); err != nil {
			return err
		}
		_, _, viewport := state.viewport(width, height)
		key, resizedDuringRead, err := readSessionViewerKey(ctx, input, terminal.resized)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if sessionViewerShouldRedrawAfterRead(key, resizedDuringRead) {
			continue
		}
		open, exit := state.apply(key, viewport)
		if exit {
			return nil
		}
		if requested := state.pendingScope; requested != "" {
			state.pendingScope = ""
			loading = startSessionBrowserLoad(ctx, scopeLoad, requested)
			continue
		}
		if !open {
			continue
		}
		exitKey, err := runSessionViewerScreen(ctx, input, output, frame, terminal.resized, detailLoad(state.items[state.selected]), p)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if exitKey == "q" {
			return nil
		}
	}
}

func renderSessionBrowser(w io.Writer, width, height int, state *sessionBrowserState, primitives ...usageTextPrimitives) error {
	p := usageTextPrimitives{}
	if len(primitives) > 0 {
		p = primitives[0]
	}
	lines := []string{"\x1b[H\x1b[2J"}
	if width < 48 || height < 10 {
		lines = append(lines,
			p.style("AGENTDECK · SESSIONS", usageColorBrand),
			"Terminal too small for session browser.",
			"Resize to at least 48x10.",
			"q/esc quit",
		)
		return sessionViewerWriteFrame(w, lines[:min(len(lines), max(1, height+1))], width)
	}

	lines = append(lines,
		p.style("AGENTDECK · SESSIONS", usageColorBrand)+" · "+p.style("INTERACTIVE", usageColorSession)+" · "+p.style("READ ONLY", usageColorSuccess),
		sessionBrowserSummary(state, p, sessionBrowserCanvasWidth(width)),
	)
	if warning := sessionBrowserCoverageWarning(state, p, sessionBrowserCanvasWidth(width)); warning != "" {
		lines = append(lines, warning)
	}
	if state.menuOpen {
		lines = append(lines, p.style("SESSION SCOPE", usageColorBrand))
		for index, scope := range sessionBrowserScopes {
			prefix := "  "
			if index == state.menuIndex {
				prefix = p.style("> ", usageColorBrand)
			}
			lines = append(lines, prefix+sessionBrowserScopeLabel(scope))
		}
		lines = append(lines, "↑/↓ select · enter apply · esc cancel · q quit")
		return sessionViewerWriteFrame(w, lines[:min(len(lines), height+1)], width)
	}
	if state.untrusted {
		// No trusted result exists: never show a count, list or empty state.
		lines = []string{"\x1b[H\x1b[2J",
			p.style("AGENTDECK · SESSIONS", usageColorBrand) + " · " + p.style("READ ONLY", usageColorSuccess),
			p.style("Sessions for scope "+state.unavailableScope+" could not be read.", usageColorWarning),
			"No trusted result is available. Diagnose: agentdeck doctor",
			"s scope · q/esc quit",
		}
		return sessionViewerWriteFrame(w, lines[:min(len(lines), height+1)], width)
	}
	if len(state.items) == 0 {
		empty, recovery := "No indexed sessions.", "Run: agentdeck session scan"
		if !state.indexEmpty && state.scope != "" && state.scope != sessionorigin.ScopeAll {
			empty, recovery = fmt.Sprintf("No %s sessions in this selection.", state.scope), "Press s to choose All."
		}
		help := "q/esc quit"
		if state.scope != "" {
			help = "s scope · q/esc quit"
		}
		if state.failedScope != "" {
			lines = append(lines, p.style(fmt.Sprintf("Requested scope %s failed; showing %s.", state.failedScope, state.scope), usageColorWarning))
		}
		lines = append(lines,
			p.style(empty, usageColorWarning),
			recovery,
			help,
		)
		return sessionViewerWriteFrame(w, lines[:min(len(lines), height+1)], width)
	}

	canvasWidth := sessionBrowserCanvasWidth(width)
	lines = append(lines, sessionBrowserHeader(canvasWidth, p, state.scope != "")...)
	start, end, _ := state.viewport(width, height)
	for index := start; index < end; index++ {
		lines = append(lines, sessionBrowserStyledRows(state.items[index], state.class(index), index == state.selected, canvasWidth, p)...)
	}
	selected := state.items[state.selected]
	status := sessionBrowserPreview(selected, canvasWidth, p)
	if state.failedScope != "" {
		status = statsFit(p.style(fmt.Sprintf("Requested scope %s failed; showing %s. esc dismiss · s retry", state.failedScope, state.scope), usageColorWarning), canvasWidth)
	}
	help := "↑/↓ select · pgup/pgdn page · home/end · enter open · q/esc quit"
	if state.scope != "" {
		help = "↑/↓ select · pgup/pgdn · enter open · s scope · q/esc quit"
	}
	lines = append(lines,
		status,
		statsFit(p.style(fmt.Sprintf("row %d/%d", state.selected+1, len(state.items)), usageColorInfo), canvasWidth),
		statsFit(help, canvasWidth),
	)
	if len(lines) > height+1 {
		lines = lines[:height+1]
	}
	return sessionViewerWriteFrame(w, lines, width)
}

func sessionBrowserScopeLabel(scope string) string {
	switch scope {
	case sessionorigin.ScopeWork:
		return "Work (work and unknown)"
	case sessionorigin.ScopeAuxiliary:
		return "Auxiliary (background maintenance)"
	}
	return "All sessions"
}

func sessionBrowserSummary(state *sessionBrowserState, p usageTextPrimitives, width int) string {
	zone := " · times in " + displayZoneName()
	if state.scope == "" {
		return statsFit(p.style(fmt.Sprintf("%d INDEXED SESSIONS", len(state.items)), usageColorSession)+zone, width)
	}
	line := p.style(fmt.Sprintf("%d SESSIONS", len(state.items)), usageColorSession) + " · Scope: " + state.scope
	if state.scope != sessionorigin.ScopeAuxiliary {
		line += fmt.Sprintf(" · %d unknown", state.unknown)
	}
	return statsFit(line+zone, width)
}

// sessionBrowserCoverageCheck is where the browser points for the cause of an
// incomplete classification coverage, as the text output does.
const sessionBrowserCoverageCheck = "Check: agentdeck session origin-source status"

// sessionBrowserCoverageWarning is the warning line under the scope line, or
// "" when the selection's classification coverage is complete. It is its own
// line so the scope line's counts never squeeze it. It names the partial
// state, its reason codes and the check command; as the width shrinks the
// check command and then the wording give way before the reason codes, and
// PARTIAL leads every form, so truncation never removes it.
func sessionBrowserCoverageWarning(state *sessionBrowserState, p usageTextPrimitives, width int) string {
	if len(state.coverageGaps) == 0 {
		return ""
	}
	reasons := strings.Join(state.coverageGaps, ", ")
	for _, candidate := range []string{
		"PARTIAL: classification coverage incomplete (" + reasons + ") · " + sessionBrowserCoverageCheck,
		"PARTIAL: classification coverage incomplete (" + reasons + ")",
		"PARTIAL coverage: " + reasons,
	} {
		if statsVisibleWidth(candidate) <= width {
			return p.style(candidate, usageColorWarning)
		}
	}
	return p.style(statsFit("PARTIAL coverage: "+reasons, width), usageColorWarning)
}

// readSessionBrowserKeyWhileLoading reads one key while a scope loads. It
// returns early with an empty key when the load finishes or the terminal is
// resized, so the loop can apply the result or redraw.
func readSessionBrowserKeyWhileLoading(ctx context.Context, input *os.File, resized <-chan os.Signal, loading *sessionBrowserLoad) (string, error) {
	keyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-loading.ready:
			cancel()
		case <-keyCtx.Done():
		}
	}()
	key, _, err := readSessionViewerKey(keyCtx, input, resized)
	if err != nil && ctx.Err() == nil && errors.Is(err, context.Canceled) {
		return key, nil
	}
	return key, err
}

// renderSessionBrowserLoading keeps the requested scope and exit help on
// screen while a scope loads; it never shows invented zero values.
func renderSessionBrowserLoading(w io.Writer, width, height int, scope string, p usageTextPrimitives) error {
	lines := []string{"\x1b[H\x1b[2J",
		p.style("AGENTDECK · SESSIONS", usageColorBrand) + " · " + p.style("READ ONLY", usageColorSuccess),
		"Requested scope: " + scope,
		"Loading committed sessions…",
		"q quit · esc cancel",
	}
	if width < 48 || height < 10 {
		lines = []string{"\x1b[H\x1b[2J", "Loading " + scope + "…", "q/esc"}
	}
	return sessionViewerWriteFrame(w, lines[:min(len(lines), max(1, height+1))], width)
}

func sessionBrowserCanvasWidth(width int) int {
	return max(1, min(width, 120))
}

type sessionBrowserLayout struct {
	client  int
	session int
	model   int
	project int
	last    int
	compact bool
}

func sessionBrowserLayoutFor(width int) sessionBrowserLayout {
	available := max(1, width-2)
	if width >= 118 {
		layout := sessionBrowserLayout{client: 7, session: 34, model: 24, last: 18}
		layout.project = max(14, available-layout.client-layout.session-layout.model-layout.last-4)
		return layout
	}
	if width >= 80 {
		layout := sessionBrowserLayout{client: 7, session: 18, model: 13, last: 16}
		layout.project = max(10, available-layout.client-layout.session-layout.model-layout.last-4)
		return layout
	}
	return sessionBrowserLayout{compact: true}
}

func sessionBrowserHeader(width int, p usageTextPrimitives, scoped ...bool) []string {
	layout := sessionBrowserLayoutFor(width)
	clientLabel := "CLIENT"
	if len(scoped) > 0 && scoped[0] {
		clientLabel = "CLASS CLIENT"
		if !layout.compact {
			layout.client += 10
			layout.project = max(4, layout.project-10)
		}
	}
	if layout.compact {
		return []string{
			"  " + p.style(clientLabel+" / SESSION", usageColorBrand),
			"  " + p.style("MODEL · PROJECT · LAST ACTIVITY", usageColorBrand),
		}
	}
	return []string{"  " + sessionBrowserColumns(
		[]string{clientLabel, "SESSION", "MODEL", "PROJECT", "LAST ACTIVITY"},
		layout,
		[]string{usageColorBrand, usageColorBrand, usageColorBrand, usageColorBrand, usageColorBrand},
		p,
	)}
}

func sessionBrowserStyledRows(item session.Metadata, class string, selected bool, width int, p usageTextPrimitives) []string {
	prefix := "  "
	if selected {
		prefix = p.style("> ", usageColorBrand)
	}
	layout := sessionBrowserLayoutFor(width)
	client := sessionViewerKnown(item.Client)
	id := sessionViewerKnown(item.SessionID)
	model := sessionViewerKnown(item.Model)
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if class != "" {
		// The category is a word, never only a color, and is never truncated.
		label := sessionClassLabel(class)
		client = label + " " + client
	}
	if layout.compact {
		available := max(1, width-2)
		identity := p.style(statsFit(client+"/"+id, available), sessionClientColor(item.Client))
		second := sessionBrowserCompactMetadata(item.Client, model, project, last, available, p)
		return []string{prefix + statsFit(identity, available), "  " + second}
	}
	if class != "" && !layout.compact {
		layout.client += 10
		layout.project = max(4, layout.project-10)
	}
	return []string{prefix + sessionBrowserColumns(
		[]string{client, id, model, project, last},
		layout,
		[]string{sessionClientColor(item.Client), usageColorBrand, sessionClientColor(item.Client), usageColorSuccess, usageColorInfo},
		p,
	)}
}

func sessionBrowserCompactMetadata(client, model, project, last string, width int, p usageTextPrimitives) string {
	separator := " · "
	fixed := statsVisibleWidth("MODEL ") + statsVisibleWidth("PROJECT ") + statsVisibleWidth("LAST ") + 2*statsVisibleWidth(separator)
	values := max(3, width-fixed)
	modelWidth := max(1, values/4)
	projectWidth := max(1, values/3)
	lastWidth := max(1, values-modelWidth-projectWidth)
	line := p.style("MODEL", usageColorBrand) + " " + p.style(statsFit(model, modelWidth), sessionClientColor(client)) + separator +
		p.style("PROJECT", usageColorBrand) + " " + p.style(statsFit(project, projectWidth), usageColorSuccess) + separator +
		p.style("LAST", usageColorBrand) + " " + p.style(statsFit(last, lastWidth), usageColorInfo)
	return statsFit(line, width)
}

func sessionBrowserColumns(values []string, layout sessionBrowserLayout, colors []string, p usageTextPrimitives) string {
	widths := []int{layout.client, layout.session, layout.model, layout.project, layout.last}
	columns := make([]string, 0, len(values))
	for index, value := range values {
		columns = append(columns, statsPad(p.style(statsFit(value, widths[index]), colors[index]), widths[index]))
	}
	return strings.TrimRight(strings.Join(columns, " "), " ")
}

func sessionBrowserPreview(item session.Metadata, width int, p usageTextPrimitives) string {
	client := sessionViewerKnown(item.Client)
	model := sessionViewerKnown(item.Model)
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if sessionBrowserLayoutFor(width).compact {
		separator := " · "
		fixed := statsVisibleWidth("MODEL ") + statsVisibleWidth(separator) + statsVisibleWidth("PROJECT ")
		valueBudget := max(2, width-fixed)
		projectBudget := max(1, valueBudget*3/5)
		modelBudget := max(1, valueBudget-projectBudget)
		line := p.style("MODEL", usageColorBrand) + " " + p.style(statsFit(model, modelBudget), sessionClientColor(client)) + separator +
			p.style("PROJECT", usageColorBrand) + " " + p.style(statsFit(project, projectBudget), usageColorSuccess)
		return statsFit(line, width)
	}
	line := p.style("SELECTED", usageColorBrand) + " · " +
		p.style(client, sessionClientColor(client)) + " · MODEL " + p.style(model, sessionClientColor(client)) +
		" · PROJECT " + p.style(project, usageColorSuccess) + " · LAST " + p.style(last, usageColorInfo)
	return statsFit(line, width)
}

// sessionBrowserRow remains the plain, deterministic row adapter used by unit
// tests and non-color geometry checks.
func sessionBrowserRow(item session.Metadata, width int) string {
	layout := sessionBrowserLayoutFor(width + 2)
	client := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.Client))
	id := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.SessionID))
	model := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.Model))
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if layout.compact {
		identity := strings.TrimSpace(client + "/" + id)
		lastWidth := min(18, max(12, width/3))
		return statsFit(statsPad(statsFit(identity, max(1, width-lastWidth-1)), max(1, width-lastWidth-1))+" "+statsPadLeft(statsFit(last, lastWidth), lastWidth), width)
	}
	return sessionBrowserColumns([]string{client, id, model, project, last}, layout, []string{"", "", "", "", ""}, usageTextPrimitives{})
}
