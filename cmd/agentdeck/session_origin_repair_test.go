package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// ASC-CLI-R3-F1: a usage report's session selection metadata answers the
// report's own time window, through both usage entries.
func TestUsageSelectionMetadataFollowsTheReportWindow(t *testing.T) {
	useUTCDisplayClock(t)
	home := withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	waitForDetachedScanCleanup(t, state)
	// The only session is active on 2026-08-29 and stays unknown.
	writeScopedSignalSession(t, home, "window-work")
	if result := runOriginCLI(t, state, "session", "scan"); result.exit != 0 {
		t.Fatalf("scan = %#v", result)
	}
	// ASC-CLI-R4-F2: the --no-scan reads below need committed usage state.
	// usage signals runs its own scan round synchronously, so the state does
	// not depend on a detached scan having finished; confirm it is readable
	// before the window assertions.
	initialized := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "signals", "--period", "all", "--client", "claude"))
	if selection := initialized["session_selection"].(map[string]any); selection["availability"] != "available" || initialized["partial"] != false {
		t.Fatalf("usage state is not established before --no-scan: partial=%v selection=%#v", initialized["partial"], selection)
	}

	stats := func(day string) (map[string]any, map[string]any) {
		t.Helper()
		envelope := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "stats", "--no-scan", "--from", day, "--to", day, "--client", "claude", "--session-scope", "work"))
		return envelope["data"].(map[string]any), envelope["session_selection"].(map[string]any)
	}
	activeData, active := stats("2026-08-29")
	emptyData, empty := stats("2026-08-30")
	// Precondition: the report itself is windowed around the session.
	sessions := func(data map[string]any) any { return data["totals"].(map[string]any)["sessions"] }
	if sessions(activeData) != float64(1) || sessions(emptyData) != float64(0) {
		t.Fatalf("windowed sessions = %v/%v, want 1/0", sessions(activeData), sessions(emptyData))
	}
	if active["unknown_count"] != float64(1) || empty["unknown_count"] != float64(0) {
		t.Fatalf("stats unknown_count = %v/%v, want 1/0", active["unknown_count"], empty["unknown_count"])
	}
	if active["publication_id"] == empty["publication_id"] {
		t.Fatalf("two windows share publication_id %v", active["publication_id"])
	}

	signals := func(period string) map[string]any {
		t.Helper()
		envelope := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "signals", "--period", period, "--client", "claude", "--session-scope", "work"))
		return envelope["session_selection"].(map[string]any)
	}
	all, today := signals("all"), signals("today")
	if all["unknown_count"] != float64(1) || today["unknown_count"] != float64(0) {
		t.Fatalf("signals unknown_count all/today = %v/%v, want 1/0", all["unknown_count"], today["unknown_count"])
	}
	if all["publication_id"] == today["publication_id"] {
		t.Fatalf("two signal windows share publication_id %v", all["publication_id"])
	}
}

// ASC-CLI-R3-F2: the copyable form restores the exact argument in a real
// shell and never prints a terminal control character.
func TestTerminalSafeShellArgRestoresExactBytesWithoutControls(t *testing.T) {
	values := []string{"plain-id", "it's", `back\slash`, "control-\x1b[31m-id", "bell\a\ttab\nline", "c1-\u009b-csi", "del\x7f", "bad-\xff-utf8", "会话-id", "$HOME `x` $(y)"}
	shells := []string{}
	for _, shell := range []string{"/bin/bash", "/bin/zsh"} {
		if _, err := os.Stat(shell); err == nil {
			shells = append(shells, shell)
		}
	}
	if len(shells) == 0 {
		t.Skip("no bash or zsh to verify the shell encoding")
	}
	for _, value := range values {
		quoted, ok := terminalSafeShellArg(value)
		if !ok {
			t.Fatalf("%q is not encodable", value)
		}
		for _, r := range quoted {
			if isTerminalControl(r) {
				t.Fatalf("%q encodes with control %U: %q", value, r, quoted)
			}
		}
		if strings.ContainsRune(quoted, '\uFFFD') && !strings.ContainsRune(value, '\uFFFD') {
			t.Fatalf("%q encodes with a replacement character: %q", value, quoted)
		}
		for _, shell := range shells {
			out, err := exec.Command(shell, "-c", "printf %s "+quoted).Output()
			if err != nil {
				t.Fatalf("%s %q: %v", shell, quoted, err)
			}
			if !bytes.Equal(out, []byte(value)) {
				t.Fatalf("%s restored %q from %q, want %q", shell, out, quoted, value)
			}
		}
	}
	if _, ok := terminalSafeShellArg("nul\x00id"); ok {
		t.Fatal("a NUL byte cannot be one argv element")
	}
}

func TestClassificationCommandsEscapeTerminalControls(t *testing.T) {
	state := seedOriginSessions(t)
	const id = "control-\x1b[31m-id"
	index, err := store.OpenSessions(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	err = session.ReplaceDocuments(context.Background(), index.DB, "claude", id, []session.Document{{Client: "claude", SessionID: id, EventAt: "2026-11-05T08:00:00Z", Kind: "user_prompt", Text: "control"}})
	index.Close()
	if err != nil {
		t.Fatal(err)
	}
	const command = `--id $'control-\x1b[31m-id' --client 'claude'`
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"query", []string{"--no-color", "session", "classify", "--id", id, "--client", "claude"}, "Change: agentdeck session classify " + command + " --set work|auxiliary"},
		{"set", []string{"--no-color", "session", "classify", "--id", id, "--client", "claude", "--set", "work"}, "Undo: agentdeck session classify " + command + " --clear"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := runOriginCLI(t, state, tc.args...)
			if result.exit != 0 {
				t.Fatalf("exit %d: %q %q", result.exit, result.stdout, result.stderr)
			}
			if strings.ContainsAny(result.stdout, "\x1b\u009b") {
				t.Fatalf("text output contains a terminal control: %q", result.stdout)
			}
			if !strings.Contains(result.stdout, tc.want) {
				t.Fatalf("missing %q in:\n%s", tc.want, result.stdout)
			}
		})
	}
	query := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "--id", id, "--client", "claude"))
	if target := query["data"].(map[string]any)["target"].(map[string]any); target["session_id"] != id {
		t.Fatalf("JSON identity = %#v, want the exact ID", target)
	}
}

// ASC-CLI-R3-F3: a replayed request renders as the recorded historical
// receipt on every mutation entry, never as a current result.
func TestReplayedMutationsRenderAsRecordedReceipts(t *testing.T) {
	state := seedOriginSessions(t)
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "auxiliary"))
	clearArgs := []string{"session", "classify", "work-a", "--client", "claude", "--clear", "--request-id", "req-clear-a", "--expected-revision", "1"}
	mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json"}, clearArgs...)...))
	setArgs := []string{"session", "classify", "work-a", "--client", "claude", "--set", "work", "--request-id", "req-set-b"}
	mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json"}, setArgs...)...))

	replay := mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json"}, clearArgs...)...))
	if receipt := replay["data"].(map[string]any); receipt["replayed"] != true || receipt["revision"] != float64(2) {
		t.Fatalf("clear replay = %#v", receipt)
	}
	historical := []string{"· RECORDED RECEIPT", "This receipt describes that operation, not necessarily the current state."}
	check := func(name string, args []string, want []string, absent []string) {
		t.Helper()
		result := runOriginCLI(t, state, append([]string{"--no-color"}, args...)...)
		if result.exit != 0 {
			t.Fatalf("%s exit %d: %q", name, result.exit, result.stderr)
		}
		for _, text := range append(historical, want...) {
			if !strings.Contains(result.stdout, text) {
				t.Fatalf("%s replay text missing %q:\n%s", name, text, result.stdout)
			}
		}
		for _, text := range append(absent, "Views: pending publication") {
			if strings.Contains(result.stdout, text) {
				t.Fatalf("%s replay text claims %q:\n%s", name, text, result.stdout)
			}
		}
	}
	check("clear", clearArgs, []string{"CLASSIFICATION CLEARED · RECORDED RECEIPT", "Revision    2", "Current overrides: agentdeck session classify list"}, nil)
	check("set", setArgs, []string{"CLASSIFICATION SAVED · RECORDED RECEIPT", "Revision    3"}, []string{"Undo:"})
	if current := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude")); current["data"].(map[string]any)["origin"].(map[string]any)["revision"] != float64(3) {
		t.Fatalf("replays changed the current override: %#v", current["data"])
	}

	scopeArgs := []string{"session", "scope", "set", "auxiliary", "--request-id", "req-scope-c"}
	mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json"}, scopeArgs...)...))
	check("scope", scopeArgs, []string{"DEFAULT SCOPE SAVED · RECORDED RECEIPT", "Current default scope: agentdeck session scope get"}, nil)
}

// ASC-CLI-R6-F1: when the pre-report scan fails, usage signals and stats
// text says PARTIAL / scan_incomplete and that it shows the last committed
// data, as the JSON envelope already does; the report itself is unchanged.
func TestUsageTextDisclosesAnIncompleteScan(t *testing.T) {
	useUTCDisplayClock(t)
	home := withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	waitForDetachedScanCleanup(t, state)
	writeScopedSignalSession(t, home, "scan-fail-work")
	if result := runOriginCLI(t, state, "session", "scan"); result.exit != 0 {
		t.Fatalf("session scan = %#v", result)
	}
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "signals", "--period", "all", "--client", "claude"))

	entries := []struct {
		name string
		args []string
	}{
		{"signals", []string{"usage", "signals", "--period", "all", "--client", "claude"}},
		{"stats", []string{"usage", "stats", "--period", "all", "--client", "claude"}},
	}
	const partialLines = "PARTIAL: scan_incomplete\nShowing last committed data.\n"
	baselines := map[string]string{}
	for _, entry := range entries {
		result := runOriginCLI(t, state, append([]string{"--no-color"}, entry.args...)...)
		if result.exit != 0 || strings.Contains(result.stdout, "PARTIAL") || strings.Contains(result.stderr, "scan_incomplete") {
			t.Fatalf("%s baseline after a complete scan = %#v", entry.name, result)
		}
		baselines[entry.name] = result.stdout
	}

	// A worker that cannot start makes the real scan request fail; the
	// committed state stays readable.
	oldExecutable, oldForceLocal := scanRuntimeExecutable, scanRuntimeForceLocal
	t.Cleanup(func() { scanRuntimeExecutable, scanRuntimeForceLocal = oldExecutable, oldForceLocal })
	missing := filepath.Join(t.TempDir(), "missing-agentdeck-worker")
	scanRuntimeExecutable = func() string { return missing }
	scanRuntimeForceLocal = func() bool { return false }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := runUsageScanRound(ctx, &commandOptions{stateDir: state, format: "text"}, nil); err == nil || !strings.Contains(err.Error(), "missing-agentdeck-worker") {
		t.Fatalf("precondition: the scan must fail at the missing worker, got %v", err)
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			for _, quiet := range []bool{false, true} {
				args := append([]string{"--no-color"}, entry.args...)
				if quiet {
					args = append([]string{"--quiet"}, args...)
				}
				result := runOriginCLI(t, state, args...)
				if result.exit != 0 {
					t.Fatalf("quiet=%v exit %d: %q %q", quiet, result.exit, result.stdout, result.stderr)
				}
				// The marker follows the scope block, before any report data.
				at := strings.Index(result.stdout, partialLines)
				if at <= 0 {
					t.Fatalf("quiet=%v text has no PARTIAL notice:\n%s", quiet, result.stdout)
				}
				scopeBlock := result.stdout[:at]
				for _, line := range strings.Split(strings.TrimSuffix(scopeBlock, "\n"), "\n") {
					if !strings.HasPrefix(line, "Scope: work") && line != "All usage and costs remain unchanged." {
						t.Fatalf("quiet=%v report data precedes the PARTIAL notice:\n%s", quiet, result.stdout)
					}
				}
				// The data shown is the last committed report, unchanged.
				if shown := scopeBlock + result.stdout[at+len(partialLines):]; shown != baselines[entry.name] {
					t.Fatalf("quiet=%v report changed with the failed scan:\n%s\nwant:\n%s", quiet, shown, baselines[entry.name])
				}
				if !strings.Contains(result.stderr, "Warning: scan_incomplete") {
					t.Fatalf("quiet=%v stderr = %q, want the scan_incomplete warning", quiet, result.stderr)
				}
			}
			envelope := mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json"}, entry.args...)...))
			if warnings, _ := envelope["warnings"].([]any); envelope["partial"] != true || len(warnings) == 0 || warnings[0] != "scan_incomplete" {
				t.Fatalf("JSON envelope = partial %v warnings %v", envelope["partial"], envelope["warnings"])
			}
		})
	}

	// --no-scan never scans, so it reports no scan failure.
	stored := runOriginCLI(t, state, "--no-color", "usage", "stats", "--no-scan", "--period", "all", "--client", "claude")
	if stored.exit != 0 || strings.Contains(stored.stdout, "PARTIAL") || strings.Contains(stored.stderr, "scan_incomplete") {
		t.Fatalf("stored-only stats = %#v", stored)
	}
}
