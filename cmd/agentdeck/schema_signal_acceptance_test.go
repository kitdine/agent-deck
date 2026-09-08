package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestSchemaSignalBuiltBinaryAcceptance(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "agentdeck")
	build := exec.Command("go", "build", "-mod=vendor", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	contents, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("binary SHA256=%x; go build -mod=vendor -o <isolated-binary> .", sha256.Sum256(contents))
	home := t.TempDir()
	root := filepath.Join(home, ".agentdeck")
	ctx := context.Background()
	db, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'official','https://example.invalid','','1','2026-09-08T00:00:00Z','2026-09-08T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"codex", "claude"} {
		if _, err := db.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,?,'official','https://example.invalid','1','2026-09-08T00:00:00Z')`, client); err != nil {
			t.Fatal(err)
		}
	}
	runBinary := func(input string, wantExit int, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		format := "json"
		if len(args) >= 3 && args[0] == "usage" && args[1] == "hook" {
			format = "text"
		}
		cmd := exec.CommandContext(ctx, binary, append([]string{"--state-dir", root, "--format", format}, args...)...)
		cmd.Env = append(os.Environ(), "HOME="+home)
		cmd.Stdin = strings.NewReader(input)
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != wantExit {
			t.Fatalf("%v exit=%d want=%d: %s %s", args, code, wantExit, out.String(), stderr.String())
		}
		if len(args) >= 3 && args[0] == "usage" && args[1] == "hook" {
			if out.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("Hook emitted output: %q %q", out.String(), stderr.String())
			}
			return nil
		}
		if out.Len() == 0 {
			return stderr.Bytes()
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v unexpected stderr: %s", args, stderr.String())
		}
		return out.Bytes()
	}
	decode := func(raw []byte) map[string]any {
		t.Helper()
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("decode %q: %v", raw, err)
		}
		return v
	}
	deliver := func(client, id string) {
		t.Helper()
		dir := filepath.Join(home, ".codex", "sessions")
		if client == "claude" {
			dir = filepath.Join(home, ".claude", "projects", "-fixture")
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		transcript := filepath.Join(dir, id+".jsonl")
		if err := os.WriteFile(transcript, nil, 0600); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(map[string]string{"session_id": id, "transcript_path": transcript, "hook_event_name": "SessionStart", "source": "resume"})
		runBinary(string(payload), 0, "usage", "hook", "event", client)
	}
	routeCount := func(want int) {
		t.Helper()
		var got int
		if err := db.DB.QueryRow("SELECT count(*) FROM usage_session_routes").Scan(&got); err != nil || got != want {
			t.Fatalf("routes=%d want=%d: %v", got, want, err)
		}
	}
	for _, client := range []string{"codex", "claude"} {
		deliver(client, "before-"+client)
	}
	routeCount(2)
	if _, err := db.Exec(ctx, "UPDATE schema_metadata SET version=99"); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"codex", "claude"} {
		deliver(client, "refused-"+client)
	}
	routeCount(2)
	record, ok := hookrefusal.Read(root)
	if !ok || record.Count != 2 || record.Stored != 99 || record.Supported != store.CurrentSchemaVersion {
		t.Fatalf("refusals=%+v %v", record, ok)
	}
	assertChecks := func(health map[string]any) {
		t.Helper()
		schema, hook := false, false
		for _, entry := range health["checks"].([]any) {
			check := entry.(map[string]any)
			switch check["code"] {
			case "schema_ahead":
				schema = true
				if check["count"] != float64(99) || check["supported_count"] != float64(store.CurrentSchemaVersion) || check["recovery_command"] != nil {
					t.Fatalf("schema=%v", check)
				}
			case "hook_deliveries_dropped":
				hook = true
				if check["count"] != float64(2) {
					t.Fatalf("hook=%v", check)
				}
			}
		}
		if !schema || !hook {
			t.Fatalf("checks missing: %v", health)
		}
	}
	path := filepath.Join(root, hookrefusal.Filename)
	before, _ := os.ReadFile(path)
	beforeInfo, _ := os.Stat(path)
	for _, args := range [][]string{{"doctor"}, {"doctor", "--full"}} {
		envelope := decode(runBinary("", 0, args...))
		if envelope["partial"] != true || !bytes.Contains(mustJSONBytes(envelope["warnings"]), []byte("checks_skipped")) {
			t.Fatalf("doctor envelope=%v", envelope)
		}
		assertChecks(envelope["data"].(map[string]any))
	}
	doctorText := string(runBinary("", 0, "doctor", "--format", "text"))
	if !strings.Contains(strings.ToLower(doctorText), "upgrade agentdeck") || !strings.Contains(doctorText, "99") || !strings.Contains(doctorText, "23") {
		t.Fatalf("doctor text missing upgrade/version pair: %s", doctorText)
	}
	snapshot := decode(runBinary("", 0, "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "5"))
	data := snapshot["data"].(map[string]any)
	if snapshot["partial"] != true || data["wire_version"] != float64(1) {
		t.Fatalf("snapshot=%v", snapshot)
	}
	assertChecks(data["health"].(map[string]any))
	for _, name := range []string{"provider", "usage", "sessions"} {
		if data[name].(map[string]any)["available"] != false {
			t.Fatalf("%s unexpectedly available", name)
		}
	}
	after, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) || beforeInfo.Mode() != afterInfo.Mode() {
		t.Fatal("read-only path changed refusal record")
	}
	lock, err := store.AcquireLock(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	refusal := decode(runBinary("", 1, "provider", "list"))
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	errorData := refusal["error"].(map[string]any)
	if errorData["code"] != "schema_ahead" || !strings.Contains(fmt.Sprint(errorData["message"]), "upgrade AgentDeck") {
		t.Fatalf("held-lock refusal=%v", refusal)
	}
	if _, ok := hookrefusal.Read(root); !ok {
		t.Fatal("failed open cleared record")
	}
	if _, err := db.Exec(ctx, "UPDATE schema_metadata SET version=?", store.CurrentSchemaVersion); err != nil {
		t.Fatal(err)
	}
	runBinary("", 0, "provider", "list")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("successful non-Hook open did not clear: %v", err)
	}
	for _, client := range []string{"codex", "claude"} {
		deliver(client, "after-"+client)
	}
	routeCount(4)
	t.Log("PASS: real binary doctor quick/full, snapshot, held-lock write refusal, both Hooks, persisted record/routes, read-only retention and successful-open clearing")
}

func mustJSONBytes(value any) []byte { data, _ := json.Marshal(value); return data }
