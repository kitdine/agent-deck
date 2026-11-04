package telemetry

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const userConfig = "model = \"gpt-6-astra\"\n\n[features]\nmemories = true\n\n# keep this comment\n[mcp_servers.docs]\ncommand = \"docs\"\n"

func writeCodexConfig(t *testing.T, home, content string, mode os.FileMode) string {
	t.Helper()
	path := CodexConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	// The umask may narrow the mode; the tests assert an exact mode.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCodexFactsAreReadOnlyAndUnknownStaysUnknown(t *testing.T) {
	home := t.TempDir()
	if facts := ReadCodexFacts(home, 4318); facts.Memories != CodexUnknown || facts.Exporter != CodexUnknown {
		t.Fatalf("missing file facts = %+v", facts)
	}
	for _, test := range []struct {
		content  string
		memories string
		exporter string
	}{
		{userConfig, MemoriesOn, ExporterAbsent},
		{"[features]\nmemories = false\n", MemoriesOff, ExporterAbsent},
		{"model = \"x\"\n", CodexUnknown, ExporterAbsent},
		{"[otel]\nexporter = { otlp-http = { endpoint = \"https://collector.example/v1/logs\" } }\n", CodexUnknown, ExporterOther},
		{"[otel]\nexporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"json\" } }\n", CodexUnknown, ExporterAgentDeck},
		{"not = [valid toml", CodexUnknown, CodexUnknown},
	} {
		path := writeCodexConfig(t, home, test.content, 0o644)
		before, _ := os.ReadFile(path)
		facts := ReadCodexFacts(home, 4318)
		if facts.Memories != test.memories || facts.Exporter != test.exporter {
			t.Fatalf("facts(%q) = %+v", test.content, facts)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
			t.Fatal("reading facts changed the file")
		}
	}
}

func TestManagedBlockAppendsOnlyItsOwnBytesAndRemovesCleanly(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	path := writeCodexConfig(t, home, userConfig, 0o644)
	token := strings.Repeat("c", 64)
	if err := WriteManagedBlock(home, state, 4318, token, time.Now()); err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(path)
	if !bytes.HasPrefix(written, []byte(userConfig)) || !strings.HasSuffix(string(written), ManagedBlock(Endpoint(4318), token)) {
		t.Fatalf("write changed other bytes:\n%s", written)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Fatalf("mode changed to %v", info.Mode().Perm())
	}
	if facts := ReadCodexFacts(home, 4318); facts.Exporter != ExporterManaged || facts.Memories != MemoriesOn {
		t.Fatalf("facts after write = %+v", facts)
	}
	if record, _ := os.Stat(managedRecordPath(state)); record.Mode().Perm() != 0o600 {
		t.Fatal("managed record is not private")
	}
	removed, err := RemoveManagedBlock(home, state)
	if err != nil || !removed {
		t.Fatalf("remove = %v %v", removed, err)
	}
	if restored, _ := os.ReadFile(path); string(restored) != userConfig {
		t.Fatalf("removal did not restore the original bytes:\n%q", restored)
	}
	if removed, err := RemoveManagedBlock(home, state); removed || err != nil {
		t.Fatalf("second removal = %v %v", removed, err)
	}
}

func TestManagedWriteRefusesWithoutChangingTheFile(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	other := "[otel]\nexporter = { otlp-http = { endpoint = \"https://collector.example/v1/logs\" } }\n"
	path := writeCodexConfig(t, home, other, 0o644)
	if err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now()); !errors.Is(err, ErrCodexOTelConflict) {
		t.Fatalf("existing exporter = %v", err)
	}
	if now, _ := os.ReadFile(path); string(now) != other {
		t.Fatal("conflict changed the file")
	}
	writeCodexConfig(t, home, "otel.exporter = \"none\"\n", 0o644)
	if err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now()); !errors.Is(err, ErrCodexOTelConflict) {
		t.Fatalf("dotted otel key = %v", err)
	}
	writeCodexConfig(t, home, userConfig, 0o644)
	if err := os.Chmod(filepath.Dir(path), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o700) })
	if err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now()); !errors.Is(err, ErrCodexConfigWriteFailed) {
		t.Fatalf("unwritable directory = %v", err)
	}
	if now, _ := os.ReadFile(path); string(now) != userConfig {
		t.Fatal("refused write changed the file")
	}
	if _, err := os.Stat(managedRecordPath(state)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused write left a managed record: %v", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	// A change between reading and replacing aborts the replacement.
	if err := replaceIfUnchanged(path, []byte("stale expectation"), []byte("new"), 0o644); !errors.Is(err, ErrCodexConfigWriteFailed) {
		t.Fatalf("raced write = %v", err)
	}
	if now, _ := os.ReadFile(path); string(now) != userConfig {
		t.Fatal("raced write changed the file")
	}
}

func TestManagedRecordFailureChangesNothingAndRetrySucceeds(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	path := writeCodexConfig(t, home, userConfig, 0o644)
	if err := ensureDir(state); err != nil {
		t.Fatal(err)
	}
	// A directory in the record's place makes its atomic rename fail.
	if err := os.Mkdir(managedRecordPath(state), 0o700); err != nil {
		t.Fatal(err)
	}
	err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now())
	if !errors.Is(err, ErrCodexConfigWriteFailed) || errors.Is(err, ErrCodexConfigBlockRemains) {
		t.Fatalf("record failure = %v", err)
	}
	if now, _ := os.ReadFile(path); string(now) != userConfig {
		t.Fatal("record failure changed the Codex configuration")
	}
	if err := os.Remove(managedRecordPath(state)); err != nil {
		t.Fatal(err)
	}
	if err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now()); err != nil {
		t.Fatalf("retry = %v", err)
	}
	if removed, err := RemoveManagedBlock(home, state); !removed || err != nil {
		t.Fatalf("retried block removal = %v %v", removed, err)
	}
	if now, _ := os.ReadFile(path); string(now) != userConfig {
		t.Fatal("removal after retry did not restore the original bytes")
	}
}

func TestManagedWriteFailureRestoresThePreviousRecord(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	path := writeCodexConfig(t, home, userConfig, 0o644)
	if err := ensureDir(state); err != nil {
		t.Fatal(err)
	}
	previous := []byte("{\"block_sha256\":\"stale\",\"written_at\":\"2026-10-01T00:00:00Z\"}\n")
	if err := os.WriteFile(managedRecordPath(state), previous, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o700) })
	err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now())
	if !errors.Is(err, ErrCodexConfigWriteFailed) || errors.Is(err, ErrCodexConfigBlockRemains) {
		t.Fatalf("unwritable configuration = %v", err)
	}
	if now, _ := os.ReadFile(managedRecordPath(state)); !bytes.Equal(now, previous) {
		t.Fatalf("failed write replaced the previous record: %s", now)
	}
}

func TestRemovalLeavesAChangedBlockUntouched(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	path := writeCodexConfig(t, home, userConfig, 0o644)
	if err := WriteManagedBlock(home, state, 4318, strings.Repeat("c", 64), time.Now()); err != nil {
		t.Fatal(err)
	}
	edited, _ := os.ReadFile(path)
	edited = bytes.Replace(edited, []byte("log_user_prompt = false"), []byte("log_user_prompt = true"), 1)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if removed, err := RemoveManagedBlock(home, state); removed || !errors.Is(err, ErrCodexConfigRestoreIncomplete) {
		t.Fatalf("edited block removal = %v %v", removed, err)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, edited) {
		t.Fatal("incomplete restore changed the file")
	}
	// Markers lost entirely (for example a rewrite by Codex).
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveManagedBlock(home, state); !errors.Is(err, ErrCodexConfigRestoreIncomplete) {
		t.Fatalf("missing markers = %v", err)
	}
	// A user-owned exporter is never written or removed by AgentDeck.
	userOwned := "[otel]\nexporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"json\" } }\n"
	freshState := filepath.Join(t.TempDir(), "state")
	writeCodexConfig(t, home, userOwned, 0o644)
	if removed, err := RemoveManagedBlock(home, freshState); removed || err != nil {
		t.Fatalf("user-owned exporter removal = %v %v", removed, err)
	}
	if now, _ := os.ReadFile(path); string(now) != userOwned {
		t.Fatal("user-owned exporter changed")
	}
}

func TestLinkageKeepsTheReceiverStoppedWhileMemoriesIsOff(t *testing.T) {
	database, root := openStore(t)
	home := t.TempDir()
	writeCodexConfig(t, home, "[features]\nmemories = false\n", 0o644)
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	if err := Serve(context.Background(), ServeOptions{StateRoot: root, Home: home, Owner: "app", Store: database}); !errors.Is(err, ErrCollectionNotNeeded) {
		t.Fatalf("memories off served: %v", err)
	}
	status, _ := ReadStatus(context.Background(), root, home, func() (*store.Store, error) { return store.OpenReadOnly(context.Background(), root) })
	if status.Receiver != "enabled" || status.Listener != "stopped" || status.ListenerReason == nil || *status.ListenerReason != ReasonMemoriesOff {
		t.Fatalf("status = %+v", status)
	}
	// Memories on: it runs; turning Memories off stops it with that reason
	// and keeps the preference.
	writeCodexConfig(t, home, "[features]\nmemories = true\n", 0o644)
	done := make(chan error, 1)
	ready := make(chan struct{})
	resume := make(chan struct{})
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(stopped)
		done <- Serve(ctx, ServeOptions{StateRoot: root, Home: home, Owner: "app", Store: database, Ready: func(string) {
			close(ready)
			select {
			case <-resume:
			case <-ctx.Done():
			}
		}})
	}()
	defer func() {
		cancel()
		<-stopped
	}()
	<-ready
	// Flip while Ready is paused, before initial reconciliation can finish.
	writeCodexConfig(t, home, "[features]\nmemories = false\n", 0o644)
	close(resume)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("receiver kept running after Memories was turned off")
	}
	summary, _ := database.TelemetrySummary(context.Background())
	last := summary.Coverage[len(summary.Coverage)-1]
	if last.Reason != ReasonMemoriesOff {
		t.Fatalf("coverage end reason %q", last.Reason)
	}
	if config, _ := LoadConfig(root); !config.Enabled {
		t.Fatal("linkage changed the user's preference")
	}
	status, _ = ReadStatus(context.Background(), root, home, func() (*store.Store, error) { return store.OpenReadOnly(context.Background(), root) })
	if len(status.Warnings) != 0 {
		t.Fatalf("warned while collection is enabled: %v", status.Warnings)
	}
	if err := SaveConfig(root, Config{Enabled: false, Port: 4318, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	writeCodexConfig(t, home, "[features]\nmemories = true\n", 0o644)
	status, _ = ReadStatus(context.Background(), root, home, func() (*store.Store, error) { return store.OpenReadOnly(context.Background(), root) })
	if len(status.Warnings) != 1 || status.Warnings[0] != WarningMemoriesNotCounted || status.ProducerConfigured != "false" {
		t.Fatalf("memories on and collection off: %+v", status)
	}
}
