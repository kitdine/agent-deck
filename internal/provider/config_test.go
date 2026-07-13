package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
