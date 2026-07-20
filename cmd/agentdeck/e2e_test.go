package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestIsolatedEndToEndFlow(t *testing.T) {
	previousProcesses := runClientProcesses
	runClientProcesses = func(string) ([]int, error) { return nil, nil }
	t.Cleanup(func() { runClientProcesses = previousProcesses })
	fixture := loadGUIContract(t)
	observed := make(map[string]guiCommandContract, len(fixture.Contracts))
	root := t.TempDir()
	state, restoredState := filepath.Join(root, "state"), filepath.Join(root, "restored")
	home, bin := filepath.Join(root, "home"), filepath.Join(root, "bin")
	if err := os.MkdirAll(filepath.Join(home, ".codex", "sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(config, []byte("model = 'synthetic'\n[mcp_servers.synthetic]\ncommand = 'synthetic'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(home, ".codex", "sessions", "run.jsonl")
	script := "#!/bin/sh\nmkdir -p \"$(dirname \"$AGENTDECK_PHASE7_LOG\")\"\nprintf '%s\\n' " +
		"'{\"timestamp\":\"2026-07-14T00:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"session_id\":\"phase7-run\"}}' " +
		"'{\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"phase7-turn\",\"model\":\"gpt-5.4\"}}' " +
		"'{\"timestamp\":\"2026-07-14T00:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"last_token_usage\":{\"input_tokens\":10,\"cached_input_tokens\":0,\"output_tokens\":2}}}}' " +
		"'{\"type\":\"visible_user_prompt\",\"session_id\":\"phase7-run\",\"payload\":{\"text\":\"phase7 visible prompt\"}}' > \"$AGENTDECK_PHASE7_LOG\"\n"
	for _, client := range []string{"codex", "claude"} {
		if err := os.WriteFile(filepath.Join(bin, client), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	oldHome := userHomeDir
	oldPriceHTTPClient := usage.PriceHTTPClient
	userHomeDir = func() (string, error) { return home, nil }
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGENTDECK_PHASE7_LOG", log)
	t.Cleanup(func() {
		userHomeDir = oldHome
		usage.PriceHTTPClient = oldPriceHTTPClient
	})
	runJSON := func(command string, stdin string, args ...string) []byte {
		t.Helper()
		var stdout bytes.Buffer
		if err := run(append([]string{"--state-dir", state, "--format", "json"}, args...), bytes.NewBufferString(stdin), &stdout); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		assertJSONEnvelope(t, stdout.Bytes(), command)
		contract := observed[command]
		contract.Success = jsonSchema(t, stdout.Bytes())
		observed[command] = contract
		return stdout.Bytes()
	}
	runJSON("provider.add", "phase7-e2e-secret\n", "provider", "add", "phase7", "--endpoint", "https://example.invalid", "--clients", "codex")
	runJSON("provider.add", "disposable-secret\n", "provider", "add", "disposable", "--endpoint", "https://example.invalid", "--clients", "codex")
	runJSON("provider.remove", "", "provider", "remove", "disposable")
	providerList := runJSON("provider.list", "", "provider", "list")
	assertProviderDefinitionsExcludeCredentialMetadata(t, providerList)
	providerStatus := runJSON("provider.status", "", "provider", "status")
	assertProviderDefinitionsExcludeCredentialMetadata(t, providerStatus)
	providerShow := runJSON("provider.show", "", "provider", "show", "phase7")
	assertProviderDefinitionsExcludeCredentialMetadata(t, providerShow)
	runJSON("credential.list", "", "credential", "list")
	runJSON("credential.show", "", "credential", "show", "phase7")
	runJSON("credential.update", "phase7-e2e-secret\n", "credential", "update", "phase7", "--rotate")
	runJSON("credential.add", "temporary-secret\n", "credential", "add", "phase7", "--credential", "temporary", "--endpoint", "https://example.invalid", "--clients", "codex")
	runJSON("credential.remove", "", "credential", "remove", "phase7", "--credential", "temporary")
	runJSON("provider.update", "", "provider", "update", "phase7", "--multiplier", "1")
	pendingStore, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if err = pendingStore.CreateOperation(context.Background(), store.Operation{ID: "phase7-contract-pending", Kind: "provider.use", State: "prepared", Client: "codex"}); err != nil {
		pendingStore.Close()
		t.Fatal(err)
	}
	if err = pendingStore.Close(); err != nil {
		t.Fatal(err)
	}
	runJSON("provider.recover", "", "provider", "recover")
	runJSON("provider.use", "", "provider", "use", "phase7")
	claudeConfig := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudeConfig), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeConfig, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	runJSON("provider.add", "phase7-claude-secret\n", "provider", "add", "phase7-claude", "--endpoint", "https://example.invalid", "--clients", "claude")
	runJSON("provider.use", "", "provider", "use", "phase7-claude")
	runJSON("provider.current", "", "provider", "current")
	runResult := runJSON("run.codex", "", "run", "codex", "--", "phase7")
	var runEnvelope struct {
		Data struct {
			Exact       bool   `json:"exact"`
			Attribution string `json:"attribution"`
		} `json:"data"`
	}
	if err := json.Unmarshal(runResult, &runEnvelope); err != nil || !runEnvelope.Data.Exact || runEnvelope.Data.Attribution != "exact" {
		t.Fatalf("exact run = %#v, %v", runEnvelope.Data, err)
	}
	runJSON("run.claude", "", "run", "claude", "--", "phase7")
	runJSON("usage.scan", "", "usage", "scan")
	runJSON("usage.summary", "", "usage", "summary")
	runJSON("usage.stats", "", "usage", "stats", "--from", "2026-07-14", "--to", "2026-07-20")
	runJSON("usage.sessions", "", "usage", "sessions")
	runJSON("usage.diagnose", "", "usage", "diagnose")
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	latestURL := "https://api.github.com/repos/BerriAI/litellm/commits/main"
	priceURL := "https://raw.githubusercontent.com/BerriAI/litellm/" + commit + "/model_prices_and_context_window.json"
	usage.PriceHTTPClient = func() *http.Client {
		return &http.Client{Transport: contractRoundTrip(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case latestURL:
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"sha":"` + commit + `"}`)), Header: make(http.Header)}, nil
			case priceURL:
				body := `{"gpt":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000002}}`
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			default:
				return nil, fmt.Errorf("unexpected URL %s", request.URL)
			}
		})}
	}
	runJSON("price.update", "", "price", "update")
	overridePath := filepath.Join(root, "official-overrides.json")
	if err := os.WriteFile(overridePath, []byte(`[{"model":"gpt-5.4","provider":"openai","source_url":"https://example.invalid/pricing","effective_from":"2026-07-14T00:00:00Z","prices":{"output":"9"}}]`), 0600); err != nil {
		t.Fatal(err)
	}
	runJSON("price.override", "", "price", "override", "--file", overridePath)
	runJSON("price.history", "", "price", "history")
	runJSON("price.status", "", "price", "status")
	runJSON("price.list", "", "price", "list")
	runJSON("session.scan", "", "session", "scan")
	runJSON("session.list", "", "session", "list")
	runJSON("session.show", "", "session", "show", "phase7-run")
	search := runJSON("session.search", "", "session", "search", "phase7")
	if !bytes.Contains(search, []byte("phase7 visible prompt")) {
		t.Fatalf("session search did not return approved synthetic content: %s", search)
	}
	runJSON("extension.scan", "", "extension", "scan")
	extensions := runJSON("extension.list", "", "extension", "list")
	var extensionEnvelope struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(extensions, &extensionEnvelope); err != nil || len(extensionEnvelope.Data) == 0 {
		t.Fatalf("extensions = %#v, %v", extensionEnvelope, err)
	}
	extensionID := extensionEnvelope.Data[0].ID
	runJSON("extension.show", "", "extension", "show", extensionID)
	runJSON("extension.doctor", "", "extension", "doctor")
	runJSON("extension.adopt", "", "extension", "adopt", extensionID)
	runJSON("extension.release", "", "extension", "release", extensionID)
	archive := filepath.Join(state, "backups", "portable", "phase7.adb")
	runJSON("backup.create", "passphrase\n", "backup", "create", archive)
	runJSON("backup.list", "", "backup", "list")
	runJSON("backup.inspect", "passphrase\n", "backup", "inspect", archive)
	runJSON("doctor", "", "doctor", "--full")
	runJSON("version", "", "version")
	var restoredOutput bytes.Buffer
	if err := run([]string{"--state-dir", restoredState, "--format", "json", "backup", "restore", archive}, bytes.NewBufferString("passphrase\n"), &restoredOutput); err != nil {
		t.Fatal(err)
	}
	assertNonNullJSONEnvelope(t, restoredOutput.Bytes(), "backup.restore")
	restoreContract := observed["backup.restore"]
	restoreContract.Success = jsonSchema(t, restoredOutput.Bytes())
	observed["backup.restore"] = restoreContract
	restoredCredentials, err := store.OpenReadOnly(context.Background(), restoredState)
	if err != nil {
		t.Fatal(err)
	}
	restoredCredential, err := restoredCredentials.ProviderCredential(context.Background(), "phase7", "default")
	if err != nil {
		restoredCredentials.Close()
		t.Fatal(err)
	}
	restoredSecret, err := restoredCredentials.CredentialSecret(context.Background(), restoredCredential.ID)
	if err != nil {
		restoredCredentials.Close()
		t.Fatal(err)
	}
	value, err := credentialvault.New(restoredState, machineIdentity).Open(context.Background(), restoredCredential.CredentialRef, credentialvault.Sealed{Algorithm: restoredSecret.Algorithm, KeyVersion: restoredSecret.KeyVersion, KeyID: restoredSecret.KeyID, Nonce: restoredSecret.Nonce, Ciphertext: restoredSecret.Ciphertext})
	if closeErr := restoredCredentials.Close(); err == nil {
		err = closeErr
	}
	if err != nil || value != "phase7-e2e-secret" {
		t.Fatalf("restored synthetic credential = %q, %v", value, err)
	}
	source, err := store.OpenReadOnly(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	var bindings int
	if err = source.DB.QueryRowContext(context.Background(), "SELECT count(*) FROM usage_run_bindings").Scan(&bindings); err != nil || bindings == 0 {
		t.Fatalf("run bindings = %d, %v", bindings, err)
	}
	runJSON("usage.rebuild", "", "usage", "rebuild")
	restored, err := store.OpenReadOnly(context.Background(), restoredState)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	providers, err := restored.ListProviders(context.Background())
	if err != nil || len(providers) != 2 || providers[0].Name != "phase7" || providers[1].Name != "phase7-claude" {
		t.Fatalf("restored providers = %#v, %v", providers, err)
	}
	var restoredEvents int
	if err = restored.DB.QueryRowContext(context.Background(), "SELECT count(*) FROM usage_events").Scan(&restoredEvents); err != nil || restoredEvents == 0 {
		t.Fatalf("restored usage events = %d, %v", restoredEvents, err)
	}
	if _, err = os.Stat(filepath.Join(restoredState, "sessions.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("default restore sessions database = %v", err)
	}
	runJSON("session.exclude", "", "session", "exclude", "--kind", "session", "--value", "phase7-run")
	runJSON("session.rebuild", "", "session", "rebuild")
	runJSON("session.purge-index", "", "session", "purge-index")
	watchSchema := runWatchCommandSchema(t, state)
	watchContract := observed["watch"]
	watchContract.Success = watchSchema
	observed["watch"] = watchContract
	captureCommandErrorContracts(t, state, observed)
	if os.Getenv("UPDATE_AGENTDECK_GOLDEN") == "1" {
		fixture.LeafCommands = leafCommandPaths(newRootCommand(bytes.NewReader(nil), &bytes.Buffer{}))
		fixture.Contracts = observed
		encoded, err := json.MarshalIndent(fixture, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join("testdata", "phase7", "gui-json-contract.json"), append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertCommandContracts(t, fixture.Contracts, observed)
	for _, contents := range [][]byte{search, restoredOutput.Bytes()} {
		if bytes.Contains(contents, []byte("phase7-e2e-secret")) {
			t.Fatal("end-to-end output exposed credential")
		}
	}
}

func assertProviderDefinitionsExcludeCredentialMetadata(t *testing.T, encoded []byte) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	var items []map[string]json.RawMessage
	if len(envelope.Data) > 0 && envelope.Data[0] == '[' {
		if err := json.Unmarshal(envelope.Data, &items); err != nil {
			t.Fatal(err)
		}
	} else {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Data, &item); err != nil {
			t.Fatal(err)
		}
		items = []map[string]json.RawMessage{item}
	}
	for _, item := range items {
		if _, exists := item["credential"]; exists {
			t.Fatalf("provider status contains deprecated singular credential field: %s", encoded)
		}
		var definition map[string]json.RawMessage
		if err := json.Unmarshal(item["definition"], &definition); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"credential_ref", "endpoint", "multiplier", "credentials"} {
			if _, exists := definition[field]; exists {
				t.Fatalf("provider definition contains credential-owned field %q: %s", field, encoded)
			}
		}
	}
}

type cancelAfterLineWriter struct {
	bytes.Buffer
	cancel func()
}

func (w *cancelAfterLineWriter) Write(data []byte) (int, error) {
	written, err := w.Buffer.Write(data)
	if bytes.Contains(data, []byte("\n")) {
		w.cancel()
	}
	return written, err
}

func runWatchCommandSchema(t *testing.T, state string) any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writer := &cancelAfterLineWriter{cancel: cancel}
	root := newRootCommand(bytes.NewReader(nil), writer)
	root.SetContext(ctx)
	root.SetArgs([]string{"--state-dir", state, "--format", "ndjson", "watch", "--interval", "10ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("watch command: %v", err)
	}
	line := bytes.Split(bytes.TrimSpace(writer.Bytes()), []byte("\n"))[0]
	if len(line) == 0 {
		t.Fatal("watch command emitted no NDJSON")
	}
	return jsonSchema(t, line)
}

func captureCommandErrorContracts(t *testing.T, state string, observed map[string]guiCommandContract) {
	t.Helper()
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	for _, leaf := range leafCommands(root) {
		args := append([]string{"--state-dir", state, "--format", "json"}, invalidArgsForLeaf(leaf)...)
		var stdout, stderr bytes.Buffer
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
			t.Fatalf("%s error exit = %d, want 2", commandOutputName(leaf), exit)
		}
		var envelope struct {
			Command string `json:"command"`
			Error   struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %s error: %v", commandOutputName(leaf), err)
		}
		commands := []string{commandOutputName(leaf)}
		if commands[0] == "run" {
			commands = []string{"run.codex", "run.claude"}
		}
		for _, command := range commands {
			contract := observed[command]
			contract.Error = jsonSchema(t, stderr.Bytes())
			contract.ErrorCode = envelope.Error.Code
			contract.ErrorCommand = envelope.Command
			observed[command] = contract
		}
	}
	for _, command := range []string{"extension.enable", "extension.disable"} {
		args := []string{"--state-dir", state, "--format", "json", "extension", strings.TrimPrefix(command, "extension."), "synthetic"}
		var stdout, stderr bytes.Buffer
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 1 {
			t.Fatalf("%s runtime error exit = %d, want 1", command, exit)
		}
		var envelope struct {
			Command string `json:"command"`
			Error   struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		contract := observed[command]
		contract.Error = jsonSchema(t, stderr.Bytes())
		contract.ErrorCode = envelope.Error.Code
		contract.ErrorCommand = envelope.Command
		observed[command] = contract
	}
}

func jsonSchema(t *testing.T, encoded []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatalf("decode schema JSON %q: %v", encoded, err)
	}
	return jsonValueSchema(t, value)
}

func jsonValueSchema(t *testing.T, value any) any {
	t.Helper()
	switch typed := value.(type) {
	case nil:
		return nil
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = jsonValueSchema(t, child)
		}
		return result
	case []any:
		if len(typed) == 0 {
			return []any{}
		}
		unique := make(map[string]any)
		for _, child := range typed {
			schema := jsonValueSchema(t, child)
			encoded, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			unique[string(encoded)] = schema
		}
		keys := make([]string, 0, len(unique))
		for key := range unique {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]any, 0, len(keys))
		for _, key := range keys {
			result = append(result, unique[key])
		}
		return result
	default:
		t.Fatalf("unsupported JSON value %T", value)
		return nil
	}
}

func assertCommandContracts(t *testing.T, expected, actual map[string]guiCommandContract) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		return
	}
	encoded, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Fatalf("command contracts differ; actual contracts:\n%s", encoded)
}

func TestProviderRemoveGolden(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := run([]string{"--state-dir", state, "provider", "add", "disposable", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("disposable-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "provider", "remove", "disposable"}, bytes.NewReader(nil), &stdout); err != nil {
		t.Fatal(err)
	}
	assertJSONEnvelope(t, stdout.Bytes(), "provider.remove")
	database, err := store.OpenReadOnly(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	var secrets int
	if err = database.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM credential_secrets`).Scan(&secrets); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if closeErr := database.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if secrets != 0 {
		t.Fatalf("removed credential ciphertext count = %d", secrets)
	}
}

func assertJSONEnvelope(t *testing.T, encoded []byte, command string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode %s: %q: %v", command, encoded, err)
	}
	if envelope["schema_version"] != float64(output.SchemaVersion) || envelope["command"] != command {
		t.Fatalf("%s envelope = %#v", command, envelope)
	}
}

func assertNonNullJSONEnvelope(t *testing.T, encoded []byte, command string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode %s: %q: %v", command, encoded, err)
	}
	if envelope["schema_version"] != float64(output.SchemaVersion) || envelope["command"] != command || envelope["data"] == nil {
		t.Fatalf("%s envelope = %#v", command, envelope)
	}
}
