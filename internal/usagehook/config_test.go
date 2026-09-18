package usagehook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
)

func TestSetupIsIdempotentAndSecuresFiles(t *testing.T) {
	manager, home := newTestManager(t)

	first, err := manager.Setup(Request{Client: ClientAll})
	if err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if len(first.Results) != 2 {
		t.Fatalf("first setup results = %d, want 2", len(first.Results))
	}
	for _, result := range first.Results {
		if result.Outcome != OutcomeConfigured {
			t.Errorf("first setup %s outcome = %q, want configured", result.Client, result.Outcome)
		}
	}

	second, err := manager.Setup(Request{Client: ClientAll})
	if err != nil {
		t.Fatalf("second setup: %v", err)
	}
	for _, result := range second.Results {
		if result.Outcome != OutcomeUnchanged {
			t.Errorf("second setup %s outcome = %q, want unchanged", result.Client, result.Outcome)
		}
		if result.Configuration != ConfigurationConfigured {
			t.Errorf("second setup %s state = %q, want configured", result.Client, result.Configuration)
		}
	}

	for _, client := range []Client{ClientCodex, ClientClaude} {
		path := configPath(home, client)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat %s: %v", client, statErr)
		}
		if got := info.Mode().Perm(); got != privateFileMode.Perm() {
			t.Errorf("%s mode = %o, want %o", client, got, privateFileMode.Perm())
		}
		dirInfo, statErr := os.Stat(filepath.Dir(path))
		if statErr != nil {
			t.Fatalf("stat %s directory: %v", client, statErr)
		}
		if got := dirInfo.Mode().Perm(); got != privateDirMode.Perm() {
			t.Errorf("%s directory mode = %o, want %o", client, got, privateDirMode.Perm())
		}
	}
}

func TestRemovePreservesUnrelatedHooksAndTopLevelFields(t *testing.T) {
	manager, home := newTestManager(t)
	if _, err := manager.Setup(Request{Client: ClientCodex}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	path := configPath(home, ClientCodex)
	document := readDocument(t, path)
	document["keep"] = json.RawMessage(`{"nested":[1,true,"value"]}`)
	appendEntry(t, document, "SessionStart", map[string]string{
		"type":    "command",
		"command": "other-tool --session-start",
	})
	writeDocument(t, path, document, privateFileMode)

	summary, err := manager.Remove(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	result := resultFor(summary, ClientCodex)
	if result.Outcome != OutcomeRemoved {
		t.Fatalf("remove outcome = %q, want removed (result: %+v)", result.Outcome, result)
	}

	after := readDocument(t, path)
	if !jsonEquivalent(after["keep"], document["keep"]) {
		t.Errorf("unrelated top-level field changed: before %s, after %s", document["keep"], after["keep"])
	}
	entries := eventEntries(t, after, "SessionStart")
	if len(entries) != 1 {
		t.Fatalf("remaining SessionStart entries = %d, want 1", len(entries))
	}
	var unrelated map[string]string
	if err := json.Unmarshal(entries[0], &unrelated); err != nil {
		t.Fatalf("decode unrelated entry: %v", err)
	}
	if unrelated["command"] != "other-tool --session-start" {
		t.Errorf("remaining command = %q, want unrelated command", unrelated["command"])
	}
}

func TestSetupAndRemovePreserveClaudeEnvironment(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	document := map[string]json.RawMessage{
		"env":   json.RawMessage(`{"KEEP":"yes","OTHER":"value"}`),
		"hooks": json.RawMessage(`{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"other-tool"}]}],"UnrelatedEvent":[{"hooks":[{"type":"command","command":"unrelated-event"}]}]}`),
	}
	writeDocument(t, path, document, privateFileMode)

	if summary, err := manager.Setup(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("setup: %v", err)
	} else if resultFor(summary, ClientClaude).Outcome != OutcomeConfigured {
		t.Fatalf("setup outcome = %q, want configured", resultFor(summary, ClientClaude).Outcome)
	}
	afterSetup := readDocument(t, path)
	assertRawEqual(t, afterSetup["env"], document["env"])
	if len(eventEntries(t, afterSetup, "UnrelatedEvent")) != 1 {
		t.Fatal("setup removed unrelated event")
	}
	if got := string(eventGroup(t, afterSetup, "SessionStart")["matcher"]); got != `"*"` {
		t.Errorf("setup changed SessionStart matcher to %s", got)
	}

	if summary, err := manager.Remove(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("remove: %v", err)
	} else if resultFor(summary, ClientClaude).Outcome != OutcomeRemoved {
		t.Fatalf("remove outcome = %q, want removed", resultFor(summary, ClientClaude).Outcome)
	}
	afterRemove := readDocument(t, path)
	assertRawEqual(t, afterRemove["env"], document["env"])
	if len(eventEntries(t, afterRemove, "UnrelatedEvent")) != 1 {
		t.Fatal("remove removed unrelated event")
	}
	if len(eventEntries(t, afterRemove, "SessionStart")) != 1 {
		t.Fatal("remove removed unrelated SessionStart hook")
	}
	if got := string(eventGroup(t, afterRemove, "SessionStart")["matcher"]); got != `"*"` {
		t.Errorf("remove changed SessionStart matcher to %s", got)
	}
}

func TestClaudeHookAndProviderWritersPreserveEachOther(t *testing.T) {
	t.Run("hook then provider", func(t *testing.T) {
		manager, home := newTestManager(t)
		path := configPath(home, ClientClaude)

		if _, err := manager.Setup(Request{Client: ClientClaude}); err != nil {
			t.Fatalf("hook setup: %v", err)
		}
		if err := provider.WriteClaudeConfig(path, provider.ClientConfig{Endpoint: "https://provider.example/v1"}); err != nil {
			t.Fatalf("provider write: %v", err)
		}
		status, err := manager.Status(Request{Client: ClientClaude})
		if err != nil {
			t.Fatalf("hook status: %v", err)
		}
		if got := resultFor(status, ClientClaude).Configuration; got != ConfigurationConfigured {
			t.Fatalf("status = %q, want configured", got)
		}
		if _, err := manager.Remove(Request{Client: ClientClaude}); err != nil {
			t.Fatalf("hook remove: %v", err)
		}
		after := readDocument(t, path)
		if !jsonEquivalent(after["env"], json.RawMessage(`{"ANTHROPIC_BASE_URL":"https://provider.example/v1"}`)) {
			t.Fatalf("provider environment changed after hook remove: %s", after["env"])
		}
	})

	t.Run("provider then hook", func(t *testing.T) {
		manager, home := newTestManager(t)
		path := configPath(home, ClientClaude)
		writeFile(t, path, []byte("{}\n"), 0o644)
		if err := provider.WriteClaudeConfig(path, provider.ClientConfig{Endpoint: "https://provider.example/v1"}); err != nil {
			t.Fatalf("provider write: %v", err)
		}
		if _, err := manager.Setup(Request{Client: ClientClaude}); err != nil {
			t.Fatalf("hook setup: %v", err)
		}
		if _, err := manager.Remove(Request{Client: ClientClaude}); err != nil {
			t.Fatalf("hook remove: %v", err)
		}
		after := readDocument(t, path)
		if !jsonEquivalent(after["env"], json.RawMessage(`{"ANTHROPIC_BASE_URL":"https://provider.example/v1"}`)) {
			t.Fatalf("provider environment changed after hook lifecycle: %s", after["env"])
		}
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("stat Claude settings: %v", err)
		} else if got := info.Mode().Perm(); got != 0o644 {
			t.Fatalf("Claude settings mode = %o, want 644", got)
		}
	})
}

func TestClaudeLifecyclePreservesOrderingAndRemovesOnlyManagedEvents(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	original := []byte("{\n  \"env\": {\"KEEP\": \"yes\"},\n  \"model\": \"sonnet\",\n  \"hooks\": {\n    \"PreToolUse\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"other-tool\"}]}]\n  },\n  \"theme\": \"dark\"\n}\n")
	writeFile(t, path, original, 0o644)

	if _, err := manager.Setup(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	afterSetup := string(readFile(t, path))
	assertOrderedSubstrings(t, afterSetup, `"env"`, `"model"`, `"hooks"`, `"theme"`)
	assertOrderedSubstrings(t, afterSetup, `"PreToolUse"`, `"SessionStart"`, `"ConfigChange"`, `"SessionEnd"`)

	if _, err := manager.Remove(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	afterRemove := string(readFile(t, path))
	assertOrderedSubstrings(t, afterRemove, `"env"`, `"model"`, `"hooks"`, `"theme"`)
	if strings.Contains(afterRemove, `"SessionStart"`) || strings.Contains(afterRemove, `"ConfigChange"`) || strings.Contains(afterRemove, `"SessionEnd"`) {
		t.Fatalf("remove left managed-only hook events: %s", afterRemove)
	}
	if len(eventEntries(t, readDocument(t, path), "PreToolUse")) != 1 {
		t.Fatal("remove changed unrelated PreToolUse hook")
	}
}

func TestStateDirVariantsRemainManaged(t *testing.T) {
	home := t.TempDir()
	first := New(Environment{Home: home, AgentDeckCommand: "agentdeck --state-dir '/tmp/one'"})
	second := New(Environment{Home: home, AgentDeckCommand: "agentdeck --state-dir '/tmp/two'"})

	if _, err := first.Setup(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	status, err := second.Status(Request{Client: ClientClaude})
	if err != nil {
		t.Fatalf("second status: %v", err)
	}
	if got := resultFor(status, ClientClaude).Configuration; got != ConfigurationConfigured {
		t.Fatalf("second status = %q, want configured", got)
	}
	setup, err := second.Setup(Request{Client: ClientClaude})
	if err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if got := resultFor(setup, ClientClaude).Outcome; got != OutcomeUnchanged {
		t.Fatalf("second setup = %q, want unchanged", got)
	}
	removed, err := second.Remove(Request{Client: ClientClaude})
	if err != nil {
		t.Fatalf("second remove: %v", err)
	}
	if got := resultFor(removed, ClientClaude).Outcome; got != OutcomeRemoved {
		t.Fatalf("second remove = %q, want removed", got)
	}
}

func TestSetupDeduplicatesOwnedEntries(t *testing.T) {
	manager, home := newTestManager(t)
	if _, err := manager.Setup(Request{Client: ClientCodex}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	path := configPath(home, ClientCodex)
	document := readDocument(t, path)
	entries := eventEntries(t, document, "SessionStart")
	appendRawEntry(t, document, "SessionStart", entries[0])
	writeDocument(t, path, document, privateFileMode)

	summary, err := manager.Setup(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("deduplicating setup: %v", err)
	}
	if resultFor(summary, ClientCodex).Outcome != OutcomeConfigured {
		t.Fatalf("deduplicating setup outcome = %q, want configured", resultFor(summary, ClientCodex).Outcome)
	}
	if got := countOwnedEntries(t, manager, readDocument(t, path), ClientCodex); got != 1 {
		t.Fatalf("owned entry count after deduplication = %d, want 1", got)
	}
}

func TestClaudePartialHookConfigurationIsModified(t *testing.T) {
	manager, home := newTestManager(t)
	if _, err := manager.Setup(Request{Client: ClientClaude}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	path := configPath(home, ClientClaude)
	document := readDocument(t, path)
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(document["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	delete(hooks, "SessionStart")
	document["hooks"] = mustJSON(t, hooks)
	writeDocument(t, path, document, privateFileMode)

	status, err := manager.Status(Request{Client: ClientClaude})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got := resultFor(status, ClientClaude).Configuration; got != ConfigurationModified {
		t.Fatalf("partial Claude status = %q, want modified", got)
	}
}

func TestModifiedOwnedEntryIsReportedAndNotOverwritten(t *testing.T) {
	manager, home := newTestManager(t)
	if _, err := manager.Setup(Request{Client: ClientCodex}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	path := configPath(home, ClientCodex)
	document := readDocument(t, path)
	entries := eventEntries(t, document, "SessionStart")
	var modified map[string]string
	if err := json.Unmarshal(entries[0], &modified); err != nil {
		t.Fatalf("decode owned entry: %v", err)
	}
	modified["timeout"] = "1"
	replaceEntry(t, document, "SessionStart", 0, mustJSON(t, modified))
	writeDocument(t, path, document, privateFileMode)

	status, err := manager.Status(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if resultFor(status, ClientCodex).Configuration != ConfigurationModified {
		t.Fatalf("status state = %q, want modified", resultFor(status, ClientCodex).Configuration)
	}

	before := readFile(t, path)
	setup, err := manager.Setup(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("setup of modified entry: %v", err)
	}
	result := resultFor(setup, ClientCodex)
	if result.Outcome != OutcomeFailed || result.Configuration != ConfigurationModified {
		t.Fatalf("modified setup result = %+v, want failed/modified", result)
	}
	if after := readFile(t, path); !bytes.Equal(before, after) {
		t.Fatal("setup overwrote modified AgentDeck entry")
	}
}

func TestMalformedJSONAndSymlinkAreRefused(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		manager, home := newTestManager(t)
		path := configPath(home, ClientCodex)
		writeFile(t, path, []byte(`{"hooks":`), privateFileMode)

		status, err := manager.Status(Request{Client: ClientCodex})
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if resultFor(status, ClientCodex).Configuration != ConfigurationInvalid {
			t.Fatalf("malformed status = %q, want invalid", resultFor(status, ClientCodex).Configuration)
		}
		before := readFile(t, path)
		setup, err := manager.Setup(Request{Client: ClientCodex})
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		if resultFor(setup, ClientCodex).Outcome != OutcomeFailed {
			t.Fatalf("malformed setup outcome = %q, want failed", resultFor(setup, ClientCodex).Outcome)
		}
		if after := readFile(t, path); !bytes.Equal(before, after) {
			t.Fatal("malformed configuration was overwritten")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		manager, home := newTestManager(t)
		path := configPath(home, ClientCodex)
		target := filepath.Join(t.TempDir(), "hooks.json")
		writeFile(t, target, []byte(`{"hooks":{}}`), privateFileMode)
		if err := os.MkdirAll(filepath.Dir(path), privateDirMode); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		status, err := manager.Status(Request{Client: ClientCodex})
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if resultFor(status, ClientCodex).Configuration != ConfigurationInvalid {
			t.Fatalf("symlink status = %q, want invalid", resultFor(status, ClientCodex).Configuration)
		}
		setup, err := manager.Setup(Request{Client: ClientCodex})
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		if resultFor(setup, ClientCodex).Outcome != OutcomeFailed {
			t.Fatalf("symlink setup outcome = %q, want failed", resultFor(setup, ClientCodex).Outcome)
		}
	})
}

func TestOwnershipAndPermissionsAreHandled(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientCodex)
	writeFile(t, path, []byte(`{"hooks":{}}`), 0o644)

	status, err := manager.Status(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if resultFor(status, ClientCodex).Configuration != ConfigurationAbsent {
		t.Fatalf("permissive status = %q, want absent", resultFor(status, ClientCodex).Configuration)
	}
	setup, err := manager.Setup(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("setup with permissive mode: %v", err)
	}
	if resultFor(setup, ClientCodex).Outcome != OutcomeConfigured {
		t.Fatalf("setup with permissive mode outcome = %q, want configured", resultFor(setup, ClientCodex).Outcome)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("stat secured file: %v", err)
	} else if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("existing file mode = %o, want 644", got)
	}
	removed, err := manager.Remove(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("remove with permissive mode: %v", err)
	}
	if resultFor(removed, ClientCodex).Outcome != OutcomeRemoved {
		t.Fatalf("remove permissive mode outcome = %q, want removed", resultFor(removed, ClientCodex).Outcome)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("stat removed file: %v", err)
	} else if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("removed file mode = %o, want 644", got)
	}

	manager.ownsFile = func(fs.FileInfo) bool { return false }
	refused, err := manager.Status(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("status with wrong owner: %v", err)
	}
	if resultFor(refused, ClientCodex).Configuration != ConfigurationInvalid {
		t.Fatalf("wrong-owner status = %q, want invalid", resultFor(refused, ClientCodex).Configuration)
	}
}

func TestAllClientSetupRollsBackWhenSecondCommitFails(t *testing.T) {
	manager, home := newTestManager(t)
	realRename := manager.files.rename
	manager.files.rename = func(oldPath, newPath string) error {
		if strings.HasSuffix(newPath, filepath.Join(".claude", "settings.json")) {
			return errors.New("injected second-client commit failure")
		}
		return realRename(oldPath, newPath)
	}

	summary, err := manager.Setup(Request{Client: ClientAll})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !summary.HasFailures() {
		t.Fatal("rollback setup did not report failure")
	}
	for _, client := range []Client{ClientCodex, ClientClaude} {
		if _, statErr := os.Lstat(configPath(home, client)); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("%s configuration after rollback stat error = %v, want not exist", client, statErr)
		}
	}
}

func TestCodexTrustGuidance(t *testing.T) {
	manager, _ := newTestManager(t)
	summary, err := manager.Status(Request{Client: ClientCodex})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	result := resultFor(summary, ClientCodex)
	if result.Trust != TrustMayRequireUserApproval {
		t.Fatalf("trust = %q, want %q", result.Trust, TrustMayRequireUserApproval)
	}
	if !strings.Contains(result.Guidance, "/hooks") || strings.Contains(result.Guidance, "dangerously-bypass") {
		t.Fatalf("trust guidance = %q, want /hooks guidance without bypass", result.Guidance)
	}
}

func TestSetupStatusLineRegistersAndIsIdempotent(t *testing.T) {
	manager, home := newTestManager(t)

	first, err := manager.SetupStatusLine()
	if err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	if first.Outcome != OutcomeConfigured || first.Configuration != ConfigurationConfigured {
		t.Fatalf("first SetupStatusLine = %+v", first)
	}

	document := readDocument(t, configPath(home, ClientClaude))
	var entry statusLineCommandEntry
	if err := json.Unmarshal(document[statusLineKey], &entry); err != nil {
		t.Fatalf("decode statusLine: %v", err)
	}
	if entry.Type != "command" || entry.Command != "agentdeck quota capture" {
		t.Fatalf("statusLine entry = %+v", entry)
	}
	// CLA-R1-F1: the only key SetupStatusLine ever writes to
	// ~/.claude/settings.json is "statusLine" itself — no bookkeeping key of
	// AgentDeck's own belongs in another tool's config file.
	for key := range document {
		if key != "statusLine" {
			t.Fatalf("unexpected key %q written to settings.json; only statusLine may be written", key)
		}
	}
	if command, ok := manager.PriorStatusLineCommand(); ok {
		t.Fatalf("PriorStatusLineCommand = (%q, true), want ok=false (nothing was registered before)", command)
	}

	second, err := manager.SetupStatusLine()
	if err != nil {
		t.Fatalf("second SetupStatusLine: %v", err)
	}
	if second.Outcome != OutcomeUnchanged || second.Configuration != ConfigurationConfigured {
		t.Fatalf("second SetupStatusLine = %+v, want unchanged/configured", second)
	}
}

func TestSetupStatusLineRecordsExistingPriorCommand(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"ccstatusline"}`),
	}, privateFileMode)

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}

	command, ok := manager.PriorStatusLineCommand()
	if !ok || command != "ccstatusline" {
		t.Fatalf("PriorStatusLineCommand = (%q, %v), want (ccstatusline, true)", command, ok)
	}
}

// Codex PR #5 P2: settings.json already registered under a *different*
// --state-dir (a stale or another profile's AgentDeck route).
// managedStatusLineCommand alone recognizes it as "AgentDeck's own" and
// would report Unchanged without ever installing this instance's own route.
func TestSetupStatusLineReconfiguresARouteRegisteredForADifferentStateDir(t *testing.T) {
	manager, home := newTestManager(t)
	otherStateDir := t.TempDir()
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"agentdeck --state-dir ` + otherStateDir + ` quota capture"}`),
	}, privateFileMode)

	result, err := manager.SetupStatusLine()
	if err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	if result.Outcome != OutcomeConfigured {
		t.Fatalf("Outcome = %v, want configured -- a different state dir's route is not this instance's own", result.Outcome)
	}

	document := readDocument(t, path)
	var entry statusLineCommandEntry
	if err := json.Unmarshal(document[statusLineKey], &entry); err != nil {
		t.Fatalf("decode statusLine: %v", err)
	}
	if entry.Command != "agentdeck quota capture" {
		t.Fatalf("statusLine command = %q, want this instance's own desired entry", entry.Command)
	}
	// Codex PR #5 third review, P1: the other state dir's own AgentDeck route
	// is still recorded internally (SetupStatusLine's "record whatever was
	// there before" contract, unchanged), but PriorStatusLineCommand must
	// refuse to hand back a managed AgentDeck command as something to chain
	// to at runtime -- doing so is what let two installations registering
	// over each other chain A -> B -> A recursively.
	prior, found, err := manager.readStatusLinePrior()
	if err != nil || !found || !prior.Existed {
		t.Fatalf("readStatusLinePrior = (%+v, %v, %v), want the other state dir's entry recorded", prior, found, err)
	}
	if command, ok := decodeStatusLineCommandEntry(prior.Value); !ok || command != "agentdeck --state-dir "+otherStateDir+" quota capture" {
		t.Fatalf("recorded prior command = (%q, %v), want the other state dir's entry", command, ok)
	}
	if command, ok := manager.PriorStatusLineCommand(); ok {
		t.Fatalf("PriorStatusLineCommand = (%q, %v), want ok=false -- must never chain to another AgentDeck installation's own route", command, ok)
	}
}

func TestAbsoluteEmbeddedHelperStatusLineIsManagedAndNeverChained(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	absolute := "'/Applications/AgentDeck.app/Contents/Helpers/agentdeck' --state-dir '/tmp/direct state' quota capture"
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":` + strconv.Quote(absolute) + `}`),
	}, privateFileMode)

	if !managedStatusLineCommand(absolute) {
		t.Fatalf("direct-download route %q was not recognized as managed", absolute)
	}
	if managedStatusLineCommand("/tmp/agentdeck quota capture") {
		t.Fatal("an arbitrary unquoted executable named agentdeck must not be treated as AgentDeck's managed route")
	}
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	if command, ok := manager.PriorStatusLineCommand(); ok {
		t.Fatalf("PriorStatusLineCommand = (%q, %v), want an absolute AgentDeck helper route rejected as a chain target", command, ok)
	}
}

func TestSetupStatusLinePreservesUnrelatedFields(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"theme": json.RawMessage(`"dark"`),
		"hooks": json.RawMessage(`{"SessionStart":[{"hooks":[{"type":"command","command":"other"}]}]}`),
	}, privateFileMode)

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}

	document := readDocument(t, path)
	if string(document["theme"]) != `"dark"` {
		t.Fatalf(`theme = %s, want "dark" preserved`, document["theme"])
	}
	if _, ok := document["hooks"]; !ok {
		t.Fatal("hooks key must be preserved")
	}
}

func TestRestoreStatusLineRestoresPriorValue(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"ccstatusline"}`),
	}, privateFileMode)

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	restore, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if restore.Outcome != OutcomeRemoved || restore.Configuration != ConfigurationAbsent {
		t.Fatalf("RestoreStatusLine = %+v, want removed/absent", restore)
	}

	document := readDocument(t, path)
	if !jsonEquivalent(document[statusLineKey], json.RawMessage(`{"type":"command","command":"ccstatusline"}`)) {
		t.Fatalf("statusLine = %s, want the original prior command restored", document[statusLineKey])
	}
}

func TestRestoreStatusLineRestoresFormattedPriorByteForByte(t *testing.T) {
	// A hand-edited settings.json keeps statusLine indented across lines.
	// Disabling must put the file back exactly, not a compacted equivalent.
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	original := []byte("{\n  \"model\": \"opus\",\n  \"statusLine\": {\n    \"command\": \"python3 ~/.claude/statusline.py\",\n    \"padding\": 1,\n    \"type\": \"command\"\n  },\n  \"theme\": \"auto\"\n}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, privateFileMode); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	restore, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if restore.Outcome != OutcomeRemoved {
		t.Fatalf("RestoreStatusLine = %+v, want removed", restore)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restored settings differ from the original\nrestored:\n%s\noriginal:\n%s", restored, original)
	}
}

func TestPriorStatusLineRecordWithoutRawBytesStillRestores(t *testing.T) {
	// A sidecar written before raw bytes were recorded carries only the
	// decoded value; it must keep restoring and chaining.
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"ccstatusline"}`),
	}, privateFileMode)
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	priorPath, err := manager.statusLinePriorPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"existed":true,"value":{"type":"command","command":"ccstatusline"}}`)
	if err := os.WriteFile(priorPath, legacy, privateFileMode); err != nil {
		t.Fatal(err)
	}
	if command, ok := manager.PriorStatusLineCommand(); !ok || command != "ccstatusline" {
		t.Fatalf("PriorStatusLineCommand = %q, %t, want ccstatusline", command, ok)
	}
	if _, err := manager.RestoreStatusLine(); err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	document := readDocument(t, path)
	if !jsonEquivalent(document[statusLineKey], json.RawMessage(`{"type":"command","command":"ccstatusline"}`)) {
		t.Fatalf("statusLine = %s, want the legacy prior value restored", document[statusLineKey])
	}
}

// Codex PR #5 tenth review, P2: a syntactically valid but incomplete sidecar
// such as {"existed":true} used to decode successfully with Existed=true and
// an empty Value, which RestoreStatusLine then spliced verbatim into
// statusLine's slot -- replacing valid JSON with "statusLine":<nothing>. The
// restore must fail loudly instead, leaving the external file untouched.
func TestRestoreStatusLineRejectsAnIncompletePriorRecordInsteadOfCorruptingTheFile(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"ccstatusline"}`),
	}, privateFileMode)
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	before := readDocument(t, path)

	priorPath, err := manager.statusLinePriorPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(priorPath, []byte(`{"existed":true}`), privateFileMode); err != nil {
		t.Fatal(err)
	}

	result, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine returned a Go error %v, want a Result carrying Outcome=Failed instead", err)
	}
	if result.Outcome != OutcomeFailed {
		t.Fatalf("Outcome = %v, want Failed for a prior record claiming existed=true with no value", result.Outcome)
	}
	after := readDocument(t, path)
	if !jsonEquivalent(after[statusLineKey], before[statusLineKey]) {
		t.Fatalf("statusLine changed from %s to %s; a rejected prior record must leave the external file untouched", before[statusLineKey], after[statusLineKey])
	}
}

func TestRestoreStatusLineWithNoPriorRemovesTheKeyEntirely(t *testing.T) {
	// CLA-R1-F1: "nothing before AgentDeck" must restore to the key being
	// entirely absent, never a literal statusLine:null standing in for it.
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`)}, privateFileMode)

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	restore, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if restore.Outcome != OutcomeRemoved || restore.Configuration != ConfigurationAbsent {
		t.Fatalf("RestoreStatusLine = %+v, want removed/absent", restore)
	}

	document := readDocument(t, path)
	if _, ok := document[statusLineKey]; ok {
		t.Fatalf("statusLine = %s, want the key entirely absent", document[statusLineKey])
	}
	if string(document["theme"]) != `"dark"` {
		t.Fatalf(`theme = %s, want "dark" preserved`, document["theme"])
	}
}

func TestRestoreStatusLineWhenModifiedUnderneathRemovesAgentDeckCommand(t *testing.T) {
	// CLA-R1-F2: when AgentDeck's own entry has been altered (still
	// recognizably AgentDeck's by the "quota capture" marker, but with an
	// unexpected extra field), restore must not leave that command
	// installed — it removes the key outright rather than restoring the old
	// prior value over an edit it cannot interpret.
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)

	writeDocument(t, path, map[string]json.RawMessage{
		"statusLine": json.RawMessage(`{"type":"command","command":"ccstatusline"}`),
	}, privateFileMode)
	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	document := readDocument(t, path)
	document[statusLineKey] = json.RawMessage(`{"type":"command","command":"agentdeck quota capture","padding":5}`)
	writeDocument(t, path, document, privateFileMode)

	restore, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if restore.Outcome != OutcomeRestoreIncomplete || restore.Configuration != ConfigurationAbsent {
		t.Fatalf("RestoreStatusLine = %+v, want restore_incomplete/absent", restore)
	}
	if restore.Error == "" {
		t.Fatal("RestoreStatusLine must explain why a manual check is needed")
	}

	after := readDocument(t, path)
	if _, ok := after[statusLineKey]; ok {
		t.Fatalf("statusLine = %s, want AgentDeck's altered command removed rather than left installed", after[statusLineKey])
	}
}

func TestRestoreStatusLineWhenReplacedBySomethingElseLeavesItUntouched(t *testing.T) {
	// When the current value is not AgentDeck's command at all anymore
	// (already replaced by something unrelated), restore must not touch it.
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	replaced := map[string]json.RawMessage{"statusLine": json.RawMessage(`{"type":"command","command":"some-other-tool"}`)}
	writeDocument(t, path, replaced, privateFileMode)

	restore, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if restore.Outcome != OutcomeRestoreIncomplete || restore.Configuration != ConfigurationModified {
		t.Fatalf("RestoreStatusLine = %+v, want restore_incomplete/modified", restore)
	}

	after := readDocument(t, path)
	if !jsonEquivalent(after[statusLineKey], replaced[statusLineKey]) {
		t.Fatalf("statusLine = %s, want left exactly as the unrelated tool set it", after[statusLineKey])
	}
}

func TestRestoreStatusLineAbsentWhenNeverRegistered(t *testing.T) {
	manager, _ := newTestManager(t)
	result, err := manager.RestoreStatusLine()
	if err != nil {
		t.Fatalf("RestoreStatusLine: %v", err)
	}
	if result.Outcome != OutcomeAbsent || result.Configuration != ConfigurationAbsent {
		t.Fatalf("RestoreStatusLine = %+v, want absent/absent", result)
	}
}

func TestStatusLineStatusReportsConfiguredAndModified(t *testing.T) {
	manager, home := newTestManager(t)
	path := configPath(home, ClientClaude)

	absent, err := manager.StatusLineStatus()
	if err != nil {
		t.Fatalf("StatusLineStatus (absent): %v", err)
	}
	if absent.Configuration != ConfigurationAbsent {
		t.Fatalf("absent status = %+v", absent)
	}

	if _, err := manager.SetupStatusLine(); err != nil {
		t.Fatalf("SetupStatusLine: %v", err)
	}
	configured, err := manager.StatusLineStatus()
	if err != nil {
		t.Fatalf("StatusLineStatus (configured): %v", err)
	}
	if configured.Configuration != ConfigurationConfigured {
		t.Fatalf("configured status = %+v", configured)
	}

	document := readDocument(t, path)
	document[statusLineKey] = json.RawMessage(`{"type":"command","command":"agentdeck quota capture","padding":5}`)
	writeDocument(t, path, document, privateFileMode)
	modified, err := manager.StatusLineStatus()
	if err != nil {
		t.Fatalf("StatusLineStatus (modified): %v", err)
	}
	if modified.Configuration != ConfigurationModified {
		t.Fatalf("modified status = %+v", modified)
	}
}

func TestPriorStatusLineCommandNoneWhenNeverRegistered(t *testing.T) {
	manager, _ := newTestManager(t)
	if command, ok := manager.PriorStatusLineCommand(); ok {
		t.Fatalf("PriorStatusLineCommand = (%q, true), want ok=false", command)
	}
}

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	home := t.TempDir()
	return New(Environment{Home: home, AgentDeckCommand: "agentdeck", StateDir: t.TempDir()}), home
}

func configPath(home string, client Client) string {
	if client == ClientCodex {
		return filepath.Join(home, ".codex", "hooks.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func resultFor(summary Summary, client Client) Result {
	for _, result := range summary.Results {
		if result.Client == client {
			return result
		}
	}
	panic(fmt.Sprintf("missing %s result", client))
}

func assertOrderedSubstrings(t *testing.T, text string, values ...string) {
	t.Helper()
	previous := -1
	for _, value := range values {
		index := strings.Index(text, value)
		if index == -1 {
			t.Fatalf("missing %q in %s", value, text)
		}
		if index < previous {
			t.Fatalf("%q appeared before the prior key in %s", value, text)
		}
		previous = index
	}
}

func readDocument(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	document, err := decodeDocument(readFile(t, path))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return document
}

func writeDocument(t *testing.T, path string, document map[string]json.RawMessage, mode fs.FileMode) {
	t.Helper()
	contents, err := json.MarshalIndent(document, "", " ")
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	writeFile(t, path, append(contents, '\n'), mode)
}

func writeFile(t *testing.T, path string, contents []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), privateDirMode); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return contents
}

func eventEntries(t *testing.T, document map[string]json.RawMessage, event string) []json.RawMessage {
	t.Helper()
	var groups []json.RawMessage
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(document["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	if len(hooks[event]) == 0 {
		return nil
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(hooks[event][0], &group); err != nil {
		t.Fatalf("decode %s group: %v", event, err)
	}
	if raw, ok := group["hooks"]; ok {
		if err := json.Unmarshal(raw, &groups); err != nil {
			t.Fatalf("decode %s entries: %v", event, err)
		}
	}
	return groups
}

func eventGroup(t *testing.T, document map[string]json.RawMessage, event string) map[string]json.RawMessage {
	t.Helper()
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(document["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	if len(hooks[event]) == 0 {
		t.Fatalf("missing %s hook group", event)
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(hooks[event][0], &group); err != nil {
		t.Fatalf("decode %s group: %v", event, err)
	}
	return group
}

func appendEntry(t *testing.T, document map[string]json.RawMessage, event string, entry map[string]string) {
	t.Helper()
	appendRawEntry(t, document, event, mustJSON(t, entry))
}

func appendRawEntry(t *testing.T, document map[string]json.RawMessage, event string, entry json.RawMessage) {
	t.Helper()
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(document["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(hooks[event][0], &group); err != nil {
		t.Fatalf("decode %s group: %v", event, err)
	}
	entries := eventEntries(t, document, event)
	entries = append(entries, entry)
	group["hooks"] = mustJSON(t, entries)
	hooks[event][0] = mustJSON(t, group)
	document["hooks"] = mustJSON(t, hooks)
}

func replaceEntry(t *testing.T, document map[string]json.RawMessage, event string, index int, entry json.RawMessage) {
	t.Helper()
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(document["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(hooks[event][0], &group); err != nil {
		t.Fatalf("decode %s group: %v", event, err)
	}
	entries := eventEntries(t, document, event)
	entries[index] = entry
	group["hooks"] = mustJSON(t, entries)
	hooks[event][0] = mustJSON(t, group)
	document["hooks"] = mustJSON(t, hooks)
}

func countOwnedEntries(t *testing.T, manager *Manager, document map[string]json.RawMessage, client Client) int {
	t.Helper()
	hooks, _, err := decodeHooks(document)
	if err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	count := 0
	for _, event := range hookEvents(client) {
		analysis, err := analyzeEvent(hooks[event], manager.desiredEntry(client), client)
		if err != nil {
			t.Fatalf("analyze %s: %v", event, err)
		}
		count += analysis.exact
	}
	return count
}

func assertRawEqual(t *testing.T, got, want json.RawMessage) {
	t.Helper()
	if !jsonEquivalent(got, want) {
		t.Errorf("raw JSON changed: got %s, want %s", got, want)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	return encoded
}
