package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usagehook"
)

func withTestHome(t *testing.T, home string) {
	t.Helper()
	old := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = old })
}

func TestRunQuotaCaptureChainsAndPersists(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	stateDir := t.TempDir()

	manager := usagehook.New(usagehook.Environment{Home: home, StateDir: stateDir})
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	// Simulate a real prior tool already having been registered before
	// AgentDeck, by overwriting the recorded prior value: SetupStatusLine
	// ran against an empty settings.json, so its own prior is "nothing was
	// there". Rerun registration against a settings.json that already had
	// a command, so the sidecar prior record captures it.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command","command":"printf prior-output"}}`), 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine (with prior): %v", err)
	}

	payload := `{"rate_limits":{"five_hour":{"used_percentage":22.4,"resets_at":1757516400},"seven_day":{"used_percentage":3,"resets_at":1758121200}}}`
	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: stateDir,
		format:   "text",
		stdin:    strings.NewReader(payload),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	// quotaCaptureStoreTimeout deliberately excludes first-run migration from
	// its 200ms budget (CLA-R1-F3): capture is fail-open, and this test wants
	// to observe a within-budget open succeeding, not race that budget
	// against applying every migration on a brand-new state dir under -race.
	// Pre-migrating once, exactly as the production capture path would on a
	// warm database, keeps the assertion below deterministic.
	if warmup, _, err := opts.openStore(context.Background()); err != nil {
		t.Fatalf("openStore (warmup): %v", err)
	} else if err := warmup.Close(); err != nil {
		t.Fatalf("close warmup store: %v", err)
	}

	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture: %v", err)
	}

	if stdout.String() != "prior-output" {
		t.Fatalf("stdout = %q, want the prior command's output passed through unchanged", stdout.String())
	}

	database, _, err := opts.openStore(context.Background())
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer database.Close()
	quotaStore := quota.NewStore(database.DB)
	windows, err := quotaStore.Windows(context.Background(), quota.ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %+v, want both five_hour and seven_day captured", windows)
	}
}

func TestRunQuotaCaptureNeverErrorsOnMalformedStdin(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)

	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: t.TempDir(),
		format:   "text",
		stdin:    strings.NewReader("not json at all"),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture must be fail-open, got: %v", err)
	}
}

func TestRunQuotaCaptureNoPriorCommandProducesNoOutput(t *testing.T) {
	home := t.TempDir()
	withTestHome(t, home)
	// AgentDeck was never registered: PriorStatusLineCommand has nothing to
	// find, so no subprocess runs and stdout stays empty.

	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: t.TempDir(),
		format:   "text",
		stdin:    strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":1,"resets_at":1}}}`),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty with no prior command registered", stdout.String())
	}
}

func TestRunQuotaCaptureDoesNotDelayPriorCommandWhenStateLockIsHeld(t *testing.T) {
	// CLA-R1-F3 regression: opening the core store (behind a lock another
	// process may be holding, plus a possible first-run migration) must
	// never sit in front of the chain. The prior command's output must
	// appear promptly regardless.
	home := t.TempDir()
	withTestHome(t, home)
	stateDir := t.TempDir()

	manager := usagehook.New(usagehook.Environment{Home: home, StateDir: stateDir})
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command","command":"printf prior-output"}}`), 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}

	lock, err := store.AcquireLock(context.Background(), stateDir, 0)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()

	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: stateDir,
		format:   "text",
		stdin:    strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":1,"resets_at":1}}}`),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	started := time.Now()
	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture: %v", err)
	}
	elapsed := time.Since(started)

	if stdout.String() != "prior-output" {
		t.Fatalf("stdout = %q, want the prior command's output despite the state lock being held", stdout.String())
	}
	if elapsed > time.Second {
		t.Fatalf("runQuotaCapture took %v with the state lock held, want well under the old 5s default lock wait", elapsed)
	}
}

func TestRunQuotaCapturePassesFullOriginalStdinToPriorRegardlessOfSize(t *testing.T) {
	// CLA-R1-F5 regression: the prior command must receive the complete,
	// untruncated original stdin even when it exceeds
	// quota.MaxStatusLinePayloadBytes — that limit bounds only what capture
	// itself parses.
	home := t.TempDir()
	withTestHome(t, home)
	stateDir := t.TempDir()

	manager := usagehook.New(usagehook.Environment{Home: home, StateDir: stateDir})
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command","command":"wc -c"}}`), 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}

	oversized := strings.Repeat("x", int(quota.MaxStatusLinePayloadBytes)+7000)
	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: stateDir,
		format:   "text",
		stdin:    strings.NewReader(oversized),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := strconv.Itoa(len(oversized))
	if got != want {
		t.Fatalf("prior command (wc -c) saw %s bytes, want %s (the full original stdin, not truncated or zeroed)", got, want)
	}
}

func TestRunQuotaCaptureSkipsPersistenceOnceStdinExceedsSizeLimitButStillChainsFullPayload(t *testing.T) {
	// CLA-R2-F1 regression: once stdin exceeds quota.MaxStatusLinePayloadBytes,
	// runQuotaCapture must skip RecordStatusLinePayload entirely — no store
	// opened, no rows written — even for a payload that would otherwise parse
	// cleanly, while ChainStatusLine still receives and forwards the exact,
	// complete, oversized stdin unchanged.
	home := t.TempDir()
	withTestHome(t, home)
	stateDir := t.TempDir()

	manager := usagehook.New(usagehook.Environment{Home: home, StateDir: stateDir})
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command","command":"wc -c"}}`), 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}

	// Leading whitespace is insignificant JSON: this payload still decodes
	// cleanly, isolating the size limit as the reason capture is skipped
	// rather than a parse failure.
	padding := strings.Repeat(" ", int(quota.MaxStatusLinePayloadBytes)+7000)
	payload := padding + `{"rate_limits":{"five_hour":{"used_percentage":22.4,"resets_at":1757516400}}}`
	var stdout bytes.Buffer
	opts := &commandOptions{
		stateDir: stateDir,
		format:   "text",
		stdin:    strings.NewReader(payload),
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
	}

	if err := runQuotaCapture(context.Background(), opts); err != nil {
		t.Fatalf("runQuotaCapture: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := strconv.Itoa(len(payload))
	if got != want {
		t.Fatalf("prior command (wc -c) saw %s bytes, want %s (the full oversized stdin, unaffected by the size limit)", got, want)
	}

	database, _, err := opts.openStore(context.Background())
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer database.Close()
	quotaStore := quota.NewStore(database.DB)
	windows, err := quotaStore.Windows(context.Background(), quota.ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 0 {
		t.Fatalf("windows = %+v, want none: a payload over quota.MaxStatusLinePayloadBytes must skip capture entirely, even though this one parses cleanly", windows)
	}
}
