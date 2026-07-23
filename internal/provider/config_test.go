package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestWriteOfficialCodexConfigPreservesUnmanagedTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"Keep Name\" # keep field comment\nbase_url = \"https://provider.example/v1\"\nexperimental_bearer_token = \"synthetic-secret\"\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	want := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"official\" # keep field comment\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != want {
		t.Fatalf("official config:\n%s\nwant:\n%s", contents, want)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != want {
		t.Fatalf("idempotent official config = %q, %v", again, err)
	}
}

func TestWriteOfficialCodexConfigSetsCustomSelector(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"other\" # preserved comment\n[model_providers.custom]\nbase_url = \"https://provider.example/v1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "model_provider = \"custom\" # preserved comment\n[model_providers.custom]\nname = \"official\"\n" {
		t.Fatalf("official selector config = %q, %v", contents, err)
	}
}

func TestWriteOfficialCodexConfigCreatesCustomTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model = 'keep'\n[features]\nmemories = true\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, custom := readCodexCustomProvider(t, path)
	if custom["name"] != OfficialProviderName {
		t.Fatalf("official custom provider = %#v", custom)
	}
	for _, expected := range []string{before, "[model_providers.custom]\nname = \"official\"\n"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("official config missing %q: %s", expected, contents)
		}
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != string(contents) {
		t.Fatalf("idempotent inserted custom provider = %q, %v", again, err)
	}
}

func TestWriteOfficialCodexConfigPreservesInsertionBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		before string
		want   string
	}{
		{
			name:   "crlf missing name before next table",
			before: "model_provider = \"custom\"\r\n[model_providers.custom]\r\nbase_url = \"https://provider.example/v1\"\r\n[features]\r\nmemories = true\r\n",
			want:   "model_provider = \"custom\"\r\n[model_providers.custom]\r\nname = \"official\"\r\n[features]\r\nmemories = true\r\n",
		},
		{
			name:   "no final newline and missing custom table",
			before: "model = \"keep\"",
			want:   "model_provider = \"custom\"\nmodel = \"keep\"\n[model_providers.custom]\nname = \"official\"\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.before), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := WriteOfficialCodexConfig(path); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != test.want {
				t.Fatalf("official config = %q, %v, want %q", contents, err, test.want)
			}
			if err = WriteOfficialCodexConfig(path); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil || string(again) != test.want {
				t.Fatalf("idempotent official config = %q, %v, want %q", again, err, test.want)
			}
		})
	}
}

func TestWriteOfficialCodexConfigFailureLeavesOriginalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := []byte("model_provider = \"custom\"\n[model_providers.custom]\nbase_url = \"https://provider.example/v1\"\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	oldReplace := replaceFile
	replaceFile = func(string, string) error { return errors.New("synthetic replace failure") }
	t.Cleanup(func() { replaceFile = oldReplace })
	if err := WriteOfficialCodexConfig(path); err == nil {
		t.Fatal("WriteOfficialCodexConfig succeeded during replace failure")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("config after failed replace = %q, %v", after, err)
	}
}

func TestCodexBearerOfficialBearerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := ClientConfig{Name: "first", Endpoint: "https://first.example/", Credential: "first-secret"}
	second := ClientConfig{Name: "second", Endpoint: "https://second.example", Credential: "second-secret"}
	if err := WriteCodexConfig(path, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	official, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(official), "base_url") || strings.Contains(string(official), "experimental_bearer_token") || !strings.Contains(string(official), "model_provider = 'custom'") {
		t.Fatalf("official config = %s", official)
	}
	_, custom := readCodexCustomProvider(t, path)
	if custom["name"] != OfficialProviderName {
		t.Fatalf("official custom provider = %#v", custom)
	}
	if err := WriteCodexConfig(path, second); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"https://second.example/v1", "second-secret"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("bearer config missing %q: %s", expected, contents)
		}
	}
	_, custom = readCodexCustomProvider(t, path)
	if custom["name"] != second.Name {
		t.Fatalf("second custom provider = %#v", custom)
	}
}

func TestWriteCodexConfigPreservesUnmanagedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = 'keep'\n[features]\nmemories = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexConfig(path, ClientConfig{Name: "example", Endpoint: "https://provider.example/", Credential: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{"model = 'keep'", "memories = true", "base_url = 'https://provider.example/v1'"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}

func TestWriteClaudeConfigPreservesUnmanagedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"keep":true,"env":{"OTHER":"preserved"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{Endpoint: "https://provider.example/", Credential: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{`"keep": true`, `"OTHER": "preserved"`, `"ANTHROPIC_BASE_URL": "https://provider.example"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}

func TestConfigMatchesEndpointWithoutReturningPrivateContent(t *testing.T) {
	root := t.TempDir()
	codex := filepath.Join(root, "config.toml")
	claude := filepath.Join(root, "settings.json")
	if err := os.WriteFile(codex, []byte("model_provider='custom'\n[model_providers.custom]\nbase_url='https://provider.example/v1'\nprivate='do-not-return'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://provider.example","PRIVATE":"do-not-return"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		client Client
		path   string
	}{
		{ClientCodex, codex},
		{ClientClaude, claude},
	} {
		matches, err := ConfigMatchesEndpoint(test.client, test.path, "https://provider.example")
		if err != nil || !matches {
			t.Fatalf("ConfigMatchesEndpoint(%s) = %t, %v", test.client, matches, err)
		}
		matches, err = ConfigMatchesEndpoint(test.client, test.path, "https://other.example")
		if err != nil || matches {
			t.Fatalf("drift ConfigMatchesEndpoint(%s) = %t, %v", test.client, matches, err)
		}
	}
}

func TestConfigMatchesOfficialCodexRequiresOfficialName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, test := range []struct {
		name     string
		contents string
		want     bool
	}{
		{name: "official", contents: "model_provider='custom'\n[model_providers.custom]\nname='official'\n", want: true},
		{name: "stale custom name", contents: "model_provider='custom'\n[model_providers.custom]\nname='aigocode'\n", want: false},
		{name: "missing name", contents: "model_provider='custom'\n[model_providers.custom]\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			matches, err := ConfigMatchesOfficialCodex(path)
			if err != nil || matches != test.want {
				t.Fatalf("ConfigMatchesOfficialCodex() = %t, %v, want %t", matches, err, test.want)
			}
		})
	}
}

func TestWriteRedactedBackupOmitsCredential(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "config.toml"), filepath.Join(root, "backups", "config.toml")
	if err := os.WriteFile(source, []byte("[model_providers.custom]\nexperimental_bearer_token = 'synthetic-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientCodex, source, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "synthetic-secret") {
		t.Fatalf("backup contains credential: %s", contents)
	}
}

func TestWriteRedactedBackupForCodexAndClaudeCreatesParentAndRedactsCredential(t *testing.T) {
	root := t.TempDir()
	codexSource, codexDestination := filepath.Join(root, "config.toml"), filepath.Join(root, "parent", "deep", "config.toml.redacted")
	// Carry representative non-secret configuration alongside the secret. A
	// redactor that emitted an empty document would satisfy every
	// plaintext-absence and file-mode assertion while silently discarding the
	// configuration the backup exists to preserve, so the backup has to be
	// checked for what it keeps, not only for what it drops.
	codexConfig := "model_provider = \"custom\"\nmodel = \"gpt-5\"\n\n" +
		"[model_providers.custom]\nname = \"Custom\"\nbase_url = \"https://provider.example/v1\"\n" +
		"experimental_bearer_token = 'synthetic-secret'\nwire_api = \"responses\"\n\n" +
		"[features]\nmemories = true\n"
	if err := os.WriteFile(codexSource, []byte(codexConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientCodex, codexSource, codexDestination); err != nil {
		t.Fatal(err)
	}
	codexContents, err := os.ReadFile(codexDestination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(codexContents), "synthetic-secret") {
		t.Fatalf("codex backup contains credential: %s", codexContents)
	}
	if info, statErr := os.Stat(codexDestination); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("codex backup mode = %#v, %v", info, statErr)
	}
	var codexBackup struct {
		ModelProvider  string `toml:"model_provider"`
		Model          string `toml:"model"`
		ModelProviders map[string]struct {
			Name    string `toml:"name"`
			BaseURL string `toml:"base_url"`
			WireAPI string `toml:"wire_api"`
			Token   string `toml:"experimental_bearer_token"`
		} `toml:"model_providers"`
		Features map[string]any `toml:"features"`
	}
	if err = toml.Unmarshal(codexContents, &codexBackup); err != nil {
		t.Fatalf("codex backup is not parseable TOML: %v\n%s", err, codexContents)
	}
	custom, ok := codexBackup.ModelProviders["custom"]
	if !ok {
		t.Fatalf("codex backup dropped the custom provider entirely: %s", codexContents)
	}
	if codexBackup.ModelProvider != "custom" || codexBackup.Model != "gpt-5" ||
		custom.Name != "Custom" || custom.WireAPI != "responses" || codexBackup.Features["memories"] != true {
		t.Fatalf("codex backup dropped restorable configuration: %#v\n%s", codexBackup, codexContents)
	}
	if custom.Token != "" {
		t.Fatalf("codex backup retained the bearer token: %q", custom.Token)
	}

	claudeSource, claudeDestination := filepath.Join(root, "settings.json"), filepath.Join(root, "backup", "claude", "settings.json.redacted")
	if err := os.WriteFile(claudeSource, []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"synthetic-secret","OTHER":"keep"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientClaude, claudeSource, claudeDestination); err != nil {
		t.Fatal(err)
	}
	claudeContents, err := os.ReadFile(claudeDestination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(claudeContents), "synthetic-secret") {
		t.Fatalf("claude backup contains credential: %s", claudeContents)
	}
	if info, statErr := os.Stat(claudeDestination); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("claude backup mode = %#v, %v", info, statErr)
	}
	var claudeBackup struct {
		Env map[string]string `json:"env"`
	}
	if err = json.Unmarshal(claudeContents, &claudeBackup); err != nil {
		t.Fatalf("claude backup is not parseable JSON: %v\n%s", err, claudeContents)
	}
	if claudeBackup.Env["OTHER"] != "keep" {
		t.Fatalf("claude backup dropped restorable env configuration: %#v\n%s", claudeBackup, claudeContents)
	}
	if _, present := claudeBackup.Env["ANTHROPIC_AUTH_TOKEN"]; present {
		t.Fatalf("claude backup retained the auth token key: %#v", claudeBackup)
	}
}

func TestWriteRedactedBackupRejectsUnsupportedClient(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(Client("unsupported"), source, filepath.Join(root, "unsupported.redacted.toml")); err == nil {
		t.Fatal("unsupported client backup succeeded")
	}
}

func TestWriteRedactedBackupWriteFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte("[model_providers.custom]\nexperimental_bearer_token = 'synthetic-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	blockedDir := filepath.Join(root, "blocked")
	if err := os.Mkdir(blockedDir, 0o500); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(blockedDir, "redacted.toml")
	if err := WriteRedactedBackup(ClientCodex, source, destination); err == nil {
		t.Fatal("write succeeded on read-only destination parent")
	}
}

func readCodexCustomProvider(t *testing.T, path string) ([]byte, map[string]any) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = toml.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	providers, ok := document["model_providers"].(map[string]any)
	if !ok {
		t.Fatalf("model providers = %#v", document["model_providers"])
	}
	custom, ok := providers["custom"].(map[string]any)
	if !ok {
		t.Fatalf("custom provider = %#v", providers["custom"])
	}
	return contents, custom
}
