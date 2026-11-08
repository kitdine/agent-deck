//go:build agentdeck_acceptance && darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/ownershiptest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
	"github.com/kitdine/agent-deck/internal/usage"
)

func init() { ownershipAcceptanceProcess = ownershipCommandProcess }

func ownershipCommandProcess() (bool, int) {
	if os.Getenv("AGENTDECK_OWNERSHIP_COMMAND") == "" && os.Getenv("AGENTDECK_OWNERSHIP_PROBE") == "" {
		return false, 0
	}
	var args []string
	if value := os.Getenv("AGENTDECK_OWNERSHIP_COMMAND"); value != "" {
		if err := json.Unmarshal([]byte(value), &args); err != nil {
			panic(err)
		}
	}
	platform.SetInstallationResolver(installation.DefaultResolver())
	if os.Getenv("AGENTDECK_OWNERSHIP_NOTIFY") == "1" {
		platform.SetInstallationResolver(&observedOwnershipResolver{Resolver: installation.DefaultResolver()})
	}
	defer platform.ReleaseStateOwnership()
	defer store.CloseReadPools()
	machineIdentity = func(context.Context) (string, error) { return "synthetic-machine", nil }
	doctorRegistrationProbe = func(context.Context, bool) doctor.RegistrationDetails {
		return doctor.RegistrationDetails{Applicable: false, ApplicabilityReason: "canonical_host_absent"}
	}
	runClientProcesses = func(string) ([]int, error) { return nil, nil }
	if probe := os.Getenv("AGENTDECK_OWNERSHIP_PROBE"); probe != "" {
		return true, ownershipPackageProbe(probe, os.Getenv("AGENTDECK_OWNERSHIP_ROOT"))
	}
	usage.PriceHTTPClient = func() *http.Client {
		return &http.Client{Transport: contractRoundTrip(func(r *http.Request) (*http.Response, error) {
			body := `{"gpt":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000002}}`
			if strings.Contains(r.URL.Path, "/commits/") {
				body = `{"sha":"abcdefabcdefabcdefabcdefabcdefabcdefabcd"}`
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
	}
	if os.Getenv("AGENTDECK_OWNERSHIP_RUNTIME") != "1" {
		return true, execute(args, os.Stdin, os.Stdout, os.Stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	root := newRootCommandWithError(os.Stdin, os.Stdout, os.Stderr)
	root.SetArgs(args)
	command, err := root.ExecuteContextC(ctx)
	if err == nil {
		return true, 0
	}
	if jsonOutputRequested(args) {
		_ = json.NewEncoder(os.Stderr).Encode(output.NewErrorWithDetails(automationCommandName(command), errorCode(err), errorMessage(err), errorDetails(err), time.Now()))
	} else {
		renderCommandErrorText(os.Stderr, err)
	}
	return true, errorExitCode(err)
}

type ownershipCommandCase struct {
	name, arguments, stdin, format, expectedError string
	runtime, terminal                             bool
}

// This is deliberately explicit, including non-stateful and runtime surfaces.
// The two-way inventory assertion below makes new or removed commands fail.
func ownershipCommandCases() []ownershipCommandCase {
	rows := []ownershipCommandCase{
		{name: "backup.create", arguments: "backup create $BASE/new.adb", stdin: "passphrase\n"},
		{name: "backup.inspect", arguments: "backup inspect $BASE/input.adb", stdin: "passphrase\n"},
		{name: "backup.list", arguments: "backup list"},
		{name: "backup.restore", arguments: "backup restore $BASE/input.adb", stdin: "passphrase\n"},
		{name: "credential.add", arguments: "credential add p --credential second --endpoint https://example.invalid --clients codex", stdin: "synthetic-secret\n"},
		{name: "credential.list", arguments: "credential list"},
		{name: "credential.remove", arguments: "credential remove p --credential second"},
		{name: "credential.show", arguments: "credential show p"},
		{name: "credential.update", arguments: "credential update p --rotate", stdin: "replacement-secret\n"},
		{name: "desktop.quota-alerts.ack", arguments: "desktop quota-alerts ack --id $ALERT"},
		{name: "desktop.quota-refresh", arguments: "desktop quota-refresh"},
		{name: "desktop.quota-settings", arguments: "desktop quota-settings --reading on"},
		{name: "desktop.quota-statusline.disable", arguments: "desktop quota-statusline disable"},
		{name: "desktop.quota-statusline.enable", arguments: "desktop quota-statusline enable"},
		{name: "desktop.refresh-indexes", arguments: "desktop refresh-indexes"},
		{name: "desktop.snapshot", arguments: "desktop snapshot"},
		{name: "doctor", arguments: "doctor"},
		{name: "extension.adopt", arguments: "extension adopt $EXT"},
		{name: "extension.disable", arguments: "extension disable $EXT", expectedError: "extension_read_only"},
		{name: "extension.doctor", arguments: "extension doctor"},
		{name: "extension.enable", arguments: "extension enable $EXT", expectedError: "extension_read_only"},
		{name: "extension.list", arguments: "extension list"},
		{name: "extension.release", arguments: "extension release $EXT"},
		{name: "extension.scan", arguments: "extension scan"},
		{name: "extension.show", arguments: "extension show $EXT"},
		{name: "price.history", arguments: "price history"},
		{name: "price.list", arguments: "price list"},
		{name: "price.override", arguments: "price override --file $BASE/prices.json"},
		{name: "price.status", arguments: "price status"},
		{name: "price.update", arguments: "price update"},
		{name: "provider.add", arguments: "provider add new --endpoint https://example.invalid --clients codex", stdin: "synthetic-secret\n"},
		{name: "provider.current", arguments: "provider current"},
		{name: "provider.list", arguments: "provider list"},
		{name: "provider.recover", arguments: "provider recover"},
		{name: "provider.remove", arguments: "provider remove p"},
		{name: "provider.set-wrapper", arguments: "provider set-wrapper p --url https://wrapper.invalid"},
		{name: "provider.show", arguments: "provider show p"},
		{name: "provider.status", arguments: "provider status"},
		{name: "provider.update", arguments: "provider update p --multiplier 1"},
		{name: "provider.use", arguments: "provider use p --client codex --no-shell-setup"},
		{name: "quota", arguments: "quota"},
		{name: "quota.capture", arguments: "quota capture", format: "text", stdin: `{"session_id":"ownership-session","model":{"id":"claude-sonnet-4"}}`},
		{name: "run", arguments: "run codex -- fixture"},
		{name: "scan", arguments: "scan"},
		{name: "session.exclude", arguments: "session exclude --kind session --value ownership-session"},
		{name: "session.list", arguments: "session list"},
		{name: "session.purge-index", arguments: "session purge-index"},
		{name: "session.rebuild", arguments: "session rebuild"},
		{name: "session.scan", arguments: "session scan"},
		{name: "session.search", arguments: "session search ownership"},
		{name: "session.show", arguments: "session show ownership-session"},
		{name: "state.migrate", arguments: "state migrate"},
		{name: "telemetry.config-preview", arguments: "telemetry config-preview"},
		{name: "telemetry.disable", arguments: "telemetry disable"},
		{name: "telemetry.enable", arguments: "telemetry enable --write-codex-config"},
		{name: "telemetry.import", arguments: "telemetry import $BASE/otel.json"},
		{name: "telemetry.status", arguments: "telemetry status"},
		{name: "usage.diagnose", arguments: "usage diagnose"},
		{name: "usage.hook.event", arguments: "usage hook event codex", format: "text", stdin: `{"hook_event_name":"SessionStart","session_id":"ownership-session"}`},
		{name: "usage.hook.remove", arguments: "usage hook remove --client all"},
		{name: "usage.hook.setup", arguments: "usage hook setup --client all"},
		{name: "usage.hook.status", arguments: "usage hook status --client all"},
		{name: "usage.rebuild", arguments: "usage rebuild"},
		{name: "usage.scan", arguments: "usage scan"},
		{name: "usage.sessions", arguments: "usage sessions"},
		{name: "usage.signals", arguments: "usage signals --period all"},
		{name: "usage.stats", arguments: "usage stats --from 2026-07-14 --to 2026-07-20"},
		{name: "usage.summary", arguments: "usage summary"},
		{name: "version", arguments: "version"},
		{name: "watch", arguments: "watch --interval 20ms", format: "ndjson", runtime: true},
		{name: "completion", arguments: "completion zsh", format: "text"},
		{name: "shell-init", arguments: "shell-init zsh", format: "text"},
		{name: "shell.env", arguments: "shell env codex", format: "text"},
		{name: "shell.setup", arguments: "shell setup zsh", format: "text"},
		{name: "shell.status", arguments: "shell status zsh", format: "text"},
		{name: "shell.remove", arguments: "shell remove zsh", format: "text"},
		{name: "session", arguments: "session --interactive", format: "text", terminal: true},
		{name: "scan-worker", arguments: "scan-worker", format: "text", runtime: true},
		{name: "desktop.bootstrap", arguments: "desktop bootstrap", format: "ndjson"},
		{name: "telemetry.serve", arguments: "telemetry serve", format: "text", runtime: true},
	}
	for i := range rows {
		if rows[i].format == "" {
			rows[i].format = "json"
		}
	}
	return rows
}

func allOwnershipCommands(root *cobra.Command) map[string]*cobra.Command {
	commands := map[string]*cobra.Command{}
	var visit func(*cobra.Command)
	visit = func(c *cobra.Command) {
		if c != root && (c.Run != nil || c.RunE != nil) {
			commands[strings.ReplaceAll(strings.TrimPrefix(c.CommandPath(), "agentdeck "), " ", ".")] = c
		}
		for _, child := range c.Commands() {
			visit(child)
		}
	}
	visit(root)
	return commands
}

func TestOwnershipLeafInventory(t *testing.T) {
	root := newRootCommand(strings.NewReader(""), io.Discard)
	all := allOwnershipCommands(root)
	var want, got []string
	for name := range all {
		want = append(want, name)
	}
	for _, row := range ownershipCommandCases() {
		got = append(got, row.name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ownership cases differ from actual runnable tree:\n got %q\nwant %q", got, want)
	}
	fixture := loadGUIContract(t)
	if !reflect.DeepEqual(fixture.LeafCommands, leafCommandPaths(root)) {
		t.Fatal("fixed GUI leaf inventory differs from live tree")
	}
	for _, row := range ownershipCommandCases() {
		command := all[row.name]
		if !contains(fixture.LeafCommands, row.name) && !emitsShellScript(command) && command.Annotations[shellLifecycleSurfaceOnlyAnnotation] != "true" && command.Annotations[humanInteractiveSurfaceOnlyAnnotation] != "true" && command.Annotations[internalRuntimeSurfaceOnlyAnnotation] != "true" {
			t.Errorf("unclassified surface: %s", row.name)
		}
	}
}

func ownershipChild(f ownershiptest.Fixture, args []string, runtime bool) *exec.Cmd {
	encoded, _ := json.Marshal(args)
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = f.Environment("AGENTDECK_OWNERSHIP_COMMAND="+string(encoded), fmt.Sprintf("AGENTDECK_OWNERSHIP_RUNTIME=%d", map[bool]int{true: 1}[runtime]), "PATH="+filepath.Join(f.Base, "bin")+":"+os.Getenv("PATH"))
	command.Dir = f.Home
	return command
}

func seedOwnershipCommand(t *testing.T, f ownershiptest.Fixture, state string, row ownershipCommandCase) []string {
	t.Helper()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return f.Home, nil }
	defer func() { userHomeDir = oldHome }()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(f.Home, ".codex", "config.toml"), "model = 'gpt-5.4'\n")
	write(filepath.Join(f.Home, ".claude", "settings.json"), "{}\n")
	write(filepath.Join(f.Base, "prices.json"), `[{"model":"gpt-5.4","provider":"openai","source_url":"https://example.invalid/pricing","effective_from":"2026-07-14T00:00:00Z","prices":{"output":"9"}}]`)
	write(filepath.Join(f.Base, "otel.json"), `{"resourceLogs":[]}`)
	write(filepath.Join(f.Base, "bin", "codex"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(f.Base, "bin", "codex"), 0700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(f.Home, ".codex", "sessions", "session.jsonl"), `{"timestamp":"2026-07-14T00:00:00Z","type":"session_meta","payload":{"id":"ownership-session","session_id":"ownership-session"}}`+"\n"+`{"type":"visible_user_prompt","session_id":"ownership-session","payload":{"text":"ownership fixture"}}`+"\n")
	invoke := func(stdin string, args ...string) []byte {
		t.Helper()
		var out bytes.Buffer
		if err := run(append([]string{"--state-dir", state, "--format", "json"}, args...), strings.NewReader(stdin), &out); err != nil {
			t.Fatalf("seed %v: %v", args, err)
		}
		return out.Bytes()
	}
	if row.name != "backup.restore" && row.name != "doctor" {
		invoke("", "state", "migrate")
	}
	if row.name == "run" || strings.HasPrefix(row.name, "credential.") || (strings.HasPrefix(row.name, "provider.") && row.name != "provider.add" && row.name != "provider.list") {
		invoke("synthetic-secret\n", "provider", "add", "p", "--endpoint", "https://example.invalid", "--clients", "codex")
		if row.name == "credential.remove" {
			invoke("second-secret\n", "credential", "add", "p", "--credential", "second", "--endpoint", "https://example.invalid", "--clients", "codex")
		}
		if row.name == "run" {
			invoke("", "provider", "use", "p", "--client", "codex", "--no-shell-setup")
		}
	}
	if row.name == "price.history" {
		invoke("", "price", "override", "--file", filepath.Join(f.Base, "prices.json"))
	}
	if strings.HasPrefix(row.name, "session") || strings.HasPrefix(row.name, "desktop.") {
		invoke("", "session", "scan")
	}
	ext := ""
	if strings.HasPrefix(row.name, "extension.") {
		write(filepath.Join(f.Home, ".codex", "skills", "ownership", "SKILL.md"), "---\nname: ownership\ndescription: isolated fixture\n---\n\n# Ownership\n")
		invoke("", "extension", "scan")
		var env struct {
			Data []struct {
				ID string `json:"id"`
			}
		}
		if err := json.Unmarshal(invoke("", "extension", "list"), &env); err != nil || len(env.Data) == 0 {
			t.Fatalf("extension fixture: %+v %v", env, err)
		}
		ext = env.Data[0].ID
		if row.name == "extension.release" || row.name == "extension.enable" || row.name == "extension.disable" {
			invoke("", "extension", "adopt", ext)
		}
	}
	if row.name == "backup.inspect" || row.name == "backup.restore" {
		original := state
		state = filepath.Join(f.Base, "archive-source")
		invoke("passphrase\n", "backup", "create", filepath.Join(f.Base, "input.adb"))
		state = original
	}
	if row.name == "desktop.quota-statusline.enable" {
		invoke("", "desktop", "quota-settings", "--reading", "on")
	}
	alertID := ""
	if row.name == "desktop.quota-alerts.ack" {
		seedQuotaState(t, state, func(s *quota.Settings) {
			s.ProbeEnabled, s.AlertsEnabled, s.AlertThresholds = true, true, []float64{75}
		}, "codex", "claude")
		withFakeQuotaRefresh(t, 80)
		var response struct {
			Data struct {
				Alerts []struct {
					ID string `json:"id"`
				} `json:"alerts"`
			} `json:"data"`
		}
		if err := json.Unmarshal(invoke("", "desktop", "quota-refresh", "--manual"), &response); err != nil || len(response.Data.Alerts) == 0 {
			t.Fatalf("quota fixture produced no alert: %v %+v", err, response)
		}
		alertID = response.Data.Alerts[0].ID
	}
	if row.name == "telemetry.serve" || row.name == "telemetry.config-preview" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		if err := telemetry.SaveConfig(state, telemetry.Config{Enabled: true, Port: port, RetentionDays: 7}); err != nil {
			t.Fatal(err)
		}
		if _, err := telemetry.EnsureToken(state); err != nil {
			t.Fatal(err)
		}
	}
	args := strings.Fields(row.arguments)
	replacer := strings.NewReplacer("$BASE", f.Base, "$HOME", f.Home, "$EXT", ext, "$ALERT", alertID)
	for i := range args {
		args[i] = replacer.Replace(args[i])
	}
	return append([]string{"--format", row.format}, args...)
}

func TestOwnershipAllCommandSurfacesBehindBarrier(t *testing.T) {
	for _, row := range ownershipCommandCases() {
		for _, branch := range []string{"default", "explicit-isolated", "explicit-real"} {
			t.Run(row.name+"/"+branch, func(t *testing.T) {
				f := ownershiptest.New(t)
				state := filepath.Join(f.Home, ".agentdeck-dev")
				if branch != "default" {
					state = filepath.Join(f.Base, "isolated")
				}
				args := seedOwnershipCommand(t, f, state, row)
				if branch == "explicit-isolated" {
					args = append([]string{"--state-dir", state}, args...)
				}
				if branch == "explicit-real" {
					args = append([]string{"--state-dir", f.Real}, args...)
				}
				f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
				command := ownershipChild(f, args, row.runtime)
				f.Sandbox(command)
				var stdout, stderr bytes.Buffer
				var err error
				if row.terminal && branch != "explicit-real" {
					command.Env = append(command.Env, "TERM=xterm-256color")
					file, slave := openSessionViewerPTY(t)
					if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 32, Col: 120}); err != nil {
						t.Fatal(err)
					}
					command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
					command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
					if err := command.Start(); err != nil {
						file.Close()
						slave.Close()
						t.Fatal(err)
					}
					slave.Close()
					readDone := make(chan struct{})
					go func() {
						defer close(readDone)
						buffer := make([]byte, 8192)
						sent := false
						for {
							n, e := file.Read(buffer)
							if n > 0 {
								stdout.Write(buffer[:n])
								if !sent && strings.Contains(stdout.String(), terminalEnterScreen) {
									_, _ = file.Write([]byte("q"))
									sent = true
								}
							}
							if e != nil {
								return
							}
						}
					}()
					timer := time.AfterFunc(10*time.Second, func() { _ = command.Process.Kill() })
					err = command.Wait()
					timer.Stop()
					file.Close()
					<-readDone
				} else {
					command.Stdin = strings.NewReader(row.stdin)
					command.Stdout = &stdout
					command.Stderr = &stderr
					err = command.Run()
				}
				code := 0
				if err != nil {
					if command.ProcessState == nil {
						t.Fatal(err)
					}
					code = command.ProcessState.ExitCode()
				}
				wantCode := 0
				wantError := row.expectedError
				if wantError != "" {
					wantCode = 1
				}
				silent := row.name == "usage.hook.event" || row.name == "quota.capture"
				if branch == "explicit-real" {
					wantCode = 1
					wantError = "state_ownership_refused"
					if silent || row.name == "doctor" {
						wantCode = 0
						wantError = ""
					}
				}
				if code != wantCode {
					t.Fatalf("exit=%d want=%d err=%v stdout=%s stderr=%s", code, wantCode, err, stdout.String(), stderr.String())
				}
				if silent {
					if stdout.Len() != 0 || stderr.Len() != 0 {
						t.Fatalf("silent surface emitted stdout=%q stderr=%q", stdout.String(), stderr.String())
					}
					return
				}
				if wantError != "" {
					if !strings.Contains(stderr.String(), wantError) {
						t.Fatalf("missing %s: %s", wantError, stderr.String())
					}
					if stdout.Len() != 0 {
						t.Fatalf("refused command emitted stdout: %s", stdout.String())
					}
					return
				}
				if row.format == "json" {
					var env struct {
						Command  string          `json:"command"`
						Data     json.RawMessage `json:"data"`
						Partial  bool            `json:"partial"`
						Warnings []string        `json:"warnings"`
					}
					if e := json.Unmarshal(stdout.Bytes(), &env); e != nil || len(env.Data) == 0 {
						t.Fatalf("invalid success envelope: %v %s", e, stdout.String())
					}
					if string(env.Data) == "null" && row.name != "credential.remove" && row.name != "extension.release" && row.name != "provider.remove" && row.name != "provider.use" {
						t.Fatalf("unexpected null data: %s", stdout.String())
					}
					if branch == "explicit-real" && row.name == "doctor" && (!env.Partial || !reflect.DeepEqual(env.Warnings, []string{"checks_skipped"})) {
						t.Fatalf("doctor refusal not partial: %s", stdout.String())
					}
				}
				t.Logf("%s %s: exit %d; output contract %s; positive and negative access controls passed", row.name, branch, code, row.format)
			})
		}
	}
}
