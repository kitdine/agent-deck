package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRootCommandRegistersGlobalFlags(t *testing.T) {
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	for _, name := range []string{"state-dir", "format", "no-color", "quiet"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing global flag %q", name)
		}
	}
	commands := map[string]bool{}
	for _, command := range root.Commands() {
		commands[command.Name()] = true
	}
	for _, name := range []string{"extension", "watch", "backup", "doctor"} {
		if !commands[name] {
			t.Fatalf("command %q missing", name)
		}
	}
}

func TestRunJSONPropagatesChildFailureAndClosesRun(t *testing.T) {
	for _, client := range []string{"codex", "claude"} {
		t.Run(client, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			state, home, bin := filepath.Join(root, "state"), filepath.Join(root, "home"), filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, client), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
				t.Fatal(err)
			}
			database, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			created, err := database.CreateProvider(ctx, store.Provider{Name: "synthetic", Endpoint: "https://example.invalid", CredentialRef: "agentdeck:test", Multiplier: "1", Clients: []store.ClientMapping{{Client: client}}})
			if err != nil {
				database.Close()
				t.Fatal(err)
			}
			if err = database.RecordSelection(ctx, store.Selection{ProviderID: created.ID, Client: client, MultiplierSnapshot: "1", SelectedAt: time.Now()}); err != nil {
				database.Close()
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}

			oldHome := userHomeDir
			userHomeDir = func() (string, error) { return home, nil }
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Cleanup(func() { userHomeDir = oldHome })
			var stdout, stderr bytes.Buffer
			if exit := execute([]string{"--state-dir", state, "--format", "json", "run", client, "--", "synthetic"}, bytes.NewReader(nil), &stdout, &stderr); exit != 1 {
				t.Fatalf("run exit = %d, stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed run wrote success output: %s", stdout.String())
			}
			var envelope map[string]any
			if err = json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("decode run error: %q: %v", stderr.String(), err)
			}
			if envelope["command"] != "run" || envelope["error"].(map[string]any)["code"] != "runtime_error" {
				t.Fatalf("run error envelope = %#v", envelope)
			}

			database, err = store.OpenReadOnly(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var openRuns int
			if err = database.DB.QueryRowContext(ctx, "SELECT count(*) FROM usage_runs WHERE ended_at IS NULL").Scan(&openRuns); err != nil || openRuns != 0 {
				t.Fatalf("open runs = %d, err=%v", openRuns, err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}

			if err = os.WriteFile(filepath.Join(bin, client), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
				t.Fatal(err)
			}
			userHomeDir = func() (string, error) { return "", errors.New("synthetic home failure") }
			stdout.Reset()
			stderr.Reset()
			if exit := execute([]string{"--state-dir", state, "--format", "json", "run", client, "--", "synthetic"}, bytes.NewReader(nil), &stdout, &stderr); exit != 1 {
				t.Fatalf("cleanup failure exit = %d, stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			database, err = store.OpenReadOnly(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var endedAt, reason string
			var exact int
			if err = database.DB.QueryRowContext(ctx, "SELECT ended_at,exact,ambiguity_reason FROM usage_runs ORDER BY id DESC LIMIT 1").Scan(&endedAt, &exact, &reason); err != nil {
				t.Fatal(err)
			}
			if endedAt == "" || exact != 0 || reason != "wrapper_cleanup_failed" {
				t.Fatalf("failed cleanup run = ended:%q exact:%d reason:%q", endedAt, exact, reason)
			}
		})
	}
}

func TestUsageCommandTextAndJSONContracts(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "usage", "diagnose"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(text.Bytes(), []byte(`"events":0`)) {
		t.Fatalf("text output = %s", text.String())
	}
	var encoded bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "summary"}, bytes.NewReader(nil), &encoded); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(encoded.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["command"] != "usage.summary" || envelope["schema_version"] != float64(1) {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestPhase6BackupAndDoctorCLIContracts(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	database, err := store.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.CreateProvider(ctx, store.Provider{Name: "synthetic", Endpoint: "https://example.invalid", CredentialRef: "agentdeck:test", Multiplier: "1", Clients: []store.ClientMapping{{Client: "codex"}}}); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	sourceSecrets := platform.NewMemorySecretStore()
	if err = sourceSecrets.Put(ctx, "agentdeck:test", "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	activeSecrets := platform.SecretStore(sourceSecrets)
	oldFactory := newSecretStore
	newSecretStore = func() platform.SecretStore { return activeSecrets }
	t.Cleanup(func() { newSecretStore = oldFactory })
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	archive := filepath.Join(source, "backups", "portable", "phase6.adb")
	var output bytes.Buffer
	if err = run([]string{"--state-dir", source, "--format", "json", "backup", "create", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.create")
	var failedOutput, failedError bytes.Buffer
	if exit := execute([]string{"--state-dir", source, "--format", "json", "backup", "create", archive}, bytes.NewBufferString("passphrase\n"), &failedOutput, &failedError); exit != 1 {
		t.Fatalf("existing backup exit = %d, stdout=%s stderr=%s", exit, failedOutput.String(), failedError.String())
	}
	if failedOutput.Len() != 0 {
		t.Fatalf("existing backup wrote success output: %s", failedOutput.String())
	}
	var errorEnvelope map[string]any
	if err = json.Unmarshal(failedError.Bytes(), &errorEnvelope); err != nil || errorEnvelope["error"].(map[string]any)["code"] != "backup_exists" {
		t.Fatalf("existing backup error = %#v, %v", errorEnvelope, err)
	}
	output.Reset()
	if err = run([]string{"--state-dir", source, "--format", "json", "backup", "list"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.list")
	output.Reset()
	if err = run([]string{"--format", "json", "backup", "inspect", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.inspect")

	target := filepath.Join(t.TempDir(), "target")
	restoredSecrets := platform.NewMemorySecretStore()
	activeSecrets = restoredSecrets
	output.Reset()
	if err = run([]string{"--state-dir", target, "--format", "json", "backup", "restore", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.restore")
	if value, err := restoredSecrets.Get(ctx, "agentdeck:test"); err != nil || value != "synthetic-secret" {
		t.Fatalf("restored secret = %q, %v", value, err)
	}

	activeSecrets = sourceSecrets
	output.Reset()
	if err = run([]string{"--state-dir", source, "--format", "json", "doctor", "--full"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "doctor")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "backup", "inspect"}, 2, "backup.inspect", "invalid_argument")
}

func TestReadPassphraseFromOneLine(t *testing.T) {
	value, err := readPassphrase(bytes.NewBufferString("correct horse battery staple\nignored\n"))
	if err != nil || value != "correct horse battery staple" {
		t.Fatalf("readPassphrase = %q, %v", value, err)
	}
	if _, err = readPassphrase(bytes.NewReader(nil)); !isInputError(err) {
		t.Fatalf("empty passphrase error = %v", err)
	}
}

func TestPhase6RejectsNDJSONBeforeAnyBackupOrDoctorSideEffect(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	archive := filepath.Join(t.TempDir(), "portable.adb")
	for _, test := range []struct {
		args    []string
		command string
	}{
		{[]string{"--state-dir", state, "--format", "ndjson", "backup", "create", archive}, "backup.create"},
		{[]string{"--state-dir", state, "--format", "ndjson", "doctor"}, "doctor"},
	} {
		assertExtensionCLIErrorArgs(t, test.args, 2, test.command, "invalid_argument")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("ndjson rejection created state: %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("ndjson rejection created archive: %v", err)
	}
}

func TestCobraSyntaxErrorsUseJSONInvalidArgumentExitCode(t *testing.T) {
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "--bogus", "doctor"}, 2, "agentdeck", "invalid_argument")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "unknown-command"}, 2, "agentdeck", "invalid_argument")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "run", "codex", "phase7"}, 2, "run", "invalid_argument")
}

func TestLoadWatchFingerprintsDoesNotWriteExistingState(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.SetSetting(ctx, "watch.fingerprint.extension", "stable"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "agentdeck.sqlite3")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fingerprints, err := loadWatchFingerprints(ctx, state)
	if err != nil || fingerprints["extension"] != "stable" {
		t.Fatalf("loadWatchFingerprints = %#v, %v", fingerprints, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("loadWatchFingerprints wrote database: %v", err)
	}
}

func assertCommandEnvelope(t *testing.T, contents []byte, command string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(contents, &envelope); err != nil {
		t.Fatalf("decode %s: %q: %v", command, contents, err)
	}
	if envelope["command"] != command || envelope["schema_version"] != float64(1) {
		t.Fatalf("%s envelope = %#v", command, envelope)
	}
}

func TestExtensionScanCommandUsesSyntheticHomeAndStableJSON(t *testing.T) {
	state, home := filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "home")
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("[mcp_servers.github]\ncommand = 'synthetic'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var output bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "extension", "scan"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["command"] != "extension.scan" || envelope["schema_version"] != float64(1) {
		t.Fatalf("envelope = %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["diagnostics"] == nil {
		t.Fatalf("scan data = %#v", envelope["data"])
	}
	for _, args := range [][]string{{"extension", "list"}, {"extension", "show", "codex:mcp:user:github"}, {"extension", "doctor"}} {
		output.Reset()
		if err := run(append([]string{"--state-dir", state, "--format", "json"}, args...), bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if bytes.Contains(output.Bytes(), []byte(`"ID"`)) || !bytes.Contains(output.Bytes(), []byte(`"id"`)) && args[1] != "doctor" {
			t.Fatalf("unstable DTO %v: %s", args, output.String())
		}
	}
	output.Reset()
	if err := run([]string{"--state-dir", state, "--format", "json", "extension", "adopt", "codex:mcp:user:github"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	adopted := envelope["data"].(map[string]any)
	if adopted["managed"] != true || adopted["drift"] != false {
		t.Fatalf("adopt data = %#v", adopted)
	}

	assertExtensionCLIError(t, state, []string{"extension", "show", "missing"}, "extension.show", "extension_not_found")
	for _, args := range [][]string{
		{"--state-dir", state, "--format", "json", "extension", "show", "missing"},
		{"--state-dir", state, "extension", "--format", "json", "show", "missing"},
		{"--state-dir", state, "extension", "show", "missing", "--format=json"},
	} {
		assertExtensionCLIErrorArgs(t, args, 1, "extension.show", "extension_not_found")
	}
	for _, test := range []struct {
		args    []string
		command string
	}{
		{[]string{"extension", "show"}, "extension.show"},
		{[]string{"extension", "show", "one", "two"}, "extension.show"},
		{[]string{"extension", "adopt"}, "extension.adopt"},
		{[]string{"extension", "enable", "one", "two"}, "extension.enable"},
		{[]string{"extension", "disable"}, "extension.disable"},
	} {
		args := append([]string{"--state-dir", state, "--format", "json"}, test.args...)
		assertExtensionCLIErrorArgs(t, args, 2, test.command, "invalid_argument")
	}
	before, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	assertExtensionCLIError(t, state, []string{"extension", "disable", "codex:mcp:user:github"}, "extension.disable", extension.ErrReadOnly.Error())
	after, _ := os.ReadFile(config)
	if !bytes.Equal(before, after) {
		t.Fatal("disable changed native config")
	}
}

func assertExtensionCLIError(t *testing.T, state string, args []string, command, code string) {
	t.Helper()
	fullArgs := append([]string{"--state-dir", state, "--format", "json"}, args...)
	assertExtensionCLIErrorArgs(t, fullArgs, 1, command, code)
}

func assertExtensionCLIErrorArgs(t *testing.T, args []string, wantExit int, command, code string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if exitCode := execute(args, bytes.NewReader(nil), &stdout, &stderr); exitCode != wantExit {
		t.Fatalf("execute(%v) exit code = %d, want %d", args, exitCode, wantExit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("execute(%v) stdout = %s", args, stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("execute(%v) error JSON = %q: %v", args, stderr.String(), err)
	}
	errorData, ok := envelope["error"].(map[string]any)
	if !ok || envelope["command"] != command || errorData["code"] != code {
		t.Fatalf("execute(%v) envelope = %#v", args, envelope)
	}
}

func TestReadCredentialFromPipe(t *testing.T) {
	value, err := readCredential(bytes.NewBufferString("synthetic-secret\n"))
	if err != nil || value != "synthetic-secret" {
		t.Fatalf("readCredential = %q, %v", value, err)
	}
}

func TestSessionCommandUsesOnlyTheSeparateSessionDatabase(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	if err := run([]string{"--state-dir", state, "session", "list"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); err != nil {
		t.Fatalf("sessions database missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "agentdeck.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("session command created core database: %v", err)
	}
}

func TestSessionPurgeRespectsStateLock(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := store.AcquireLock(t.Context(), state, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	err = run([]string{"--state-dir", state, "session", "purge-index"}, bytes.NewReader(nil), &bytes.Buffer{})
	if !errors.Is(err, store.ErrStateBusy) {
		t.Fatalf("purge while state is locked = %v, want state_busy", err)
	}
}

func TestSessionCommandsPreserveSourcesAndDoNotExposeProhibitedContent(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	source := filepath.Join(home, ".codex", "sessions", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	contents := []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"approved prompt\"}}\n" +
		"{\"type\":\"developer\",\"session_id\":\"s\",\"payload\":{\"text\":\"forbidden-secret\"}}\n" +
		"{\"type\":\"visible_assistant_final\",\"session_id\":\"s\",\"payload\":{\"text\":\"approved reply\"}}\n")
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	runSession := func(args ...string) string {
		t.Helper()
		var output bytes.Buffer
		if err := run(append([]string{"--state-dir", state, "--format", "json", "session"}, args...), bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("session %v: %v", args, err)
		}
		if bytes.Contains(output.Bytes(), []byte("forbidden-secret")) {
			t.Fatalf("session %v exposed prohibited content: %s", args, output.String())
		}
		return output.String()
	}
	runSession("scan")
	if output := runSession("list"); !bytes.Contains([]byte(output), []byte(`"session_id":"s"`)) {
		t.Fatalf("session list omitted metadata: %s", output)
	}
	for _, args := range [][]string{{"search", "approved"}, {"show", "codex", "s"}} {
		if output := runSession(args...); !bytes.Contains([]byte(output), []byte("approved")) {
			t.Fatalf("session %v omitted approved content: %s", args, output)
		}
	}
	runSession("search", `"forbidden-secret"`)
	runSession("exclude", "session", "s")
	if output := runSession("search", "approved"); bytes.Contains([]byte(output), []byte("approved")) {
		t.Fatalf("excluded session remained visible: %s", output)
	}
	runSession("rebuild")
	if output := runSession("search", "approved"); bytes.Contains([]byte(output), []byte("approved")) {
		t.Fatalf("rebuild restored excluded session: %s", output)
	}
	if after, err := os.ReadFile(source); err != nil || !bytes.Equal(after, contents) {
		t.Fatalf("source changed: %q err=%v", after, err)
	}
	runSession("purge-index")
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("purge-index left database: %v", err)
	}
}
