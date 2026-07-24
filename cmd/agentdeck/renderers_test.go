package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/output"
)

func TestExtensionAndBackupTextContracts(t *testing.T) {
	extensionValue := rendererExtensionFixture()
	manifest := rendererBackupManifestFixture()

	tests := []struct {
		name    string
		command string
		data    any
		want    []string
	}{
		{
			name:    "extension list",
			command: "extension.list",
			data:    []extension.DTO{extensionValue},
			want:    []string{"Inventory from the most recent extension scan (not a live scan)."},
		},
		{
			name:    "extension show",
			command: "extension.show",
			data:    extensionValue,
			want: []string{
				"id: " + extensionValue.ID,
				"client: " + extensionValue.Client,
				"kind: " + extensionValue.Kind,
				"scope: " + extensionValue.Scope,
				"native id: " + extensionValue.NativeID,
				"source path: " + extensionValue.SourcePath,
				"version: " + extensionValue.Version,
				"enabled: " + extensionValue.Enabled,
				"capabilities: read_only,commands",
				"diagnostics: version_unverified,metadata_stale",
				"managed: true",
				"drift: false",
			},
		},
		{
			name:    "backup inspect",
			command: "backup.inspect",
			data:    manifest,
			want: []string{
				"schema version: 3",
				"agentdeck version: v1.2.3",
				"created: " + manifest.CreatedAt.Format(time.RFC3339Nano),
				"source platform: darwin/arm64",
				"included: core,sessions",
				"entries: 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rendered bytes.Buffer
			if err := writeResult(&rendered, "text", tt.command, tt.data); err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(rendered.String(), want) {
					t.Fatalf("%s text missing %q:\n%s", tt.command, want, rendered.String())
				}
			}
			if tt.command == "extension.list" {
				rows := rendererTableRows(rendered.String())
				if len(rows) != 2 {
					t.Fatalf("extension.list table rows = %#v, want header and one data row", rows)
				}
				assertRendererCells(t, rows[0], []string{"ID", "CLIENT", "KIND", "SCOPE", "VERSION", "ENABLED", "MANAGED", "DRIFT"})
				assertRendererCells(t, rows[1], []string{
					extensionValue.ID, extensionValue.Client, extensionValue.Kind, extensionValue.Scope,
					extensionValue.Version, extensionValue.Enabled, "true", "false",
				})
			}
		})
	}
}

func TestExtensionAndBackupJSONContracts(t *testing.T) {
	extensionValue := rendererExtensionFixture()
	manifest := rendererBackupManifestFixture()

	t.Run("extension list", func(t *testing.T) {
		envelope, raw := rendererJSONEnvelope(t, "extension.list", []extension.DTO{extensionValue})

		items, ok := envelope["data"].([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("extension JSON data = %#v, want one-element array", envelope["data"])
		}
		item, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("extension JSON item = %#v, want object", items[0])
		}
		assertRendererKeys(t, item,
			"id", "client", "kind", "scope", "native_id", "source_path", "version", "enabled",
			"capabilities", "diagnostics", "fingerprint", "managed", "drift",
		)
		for key, want := range map[string]string{
			"id":          extensionValue.ID,
			"client":      extensionValue.Client,
			"kind":        extensionValue.Kind,
			"scope":       extensionValue.Scope,
			"native_id":   extensionValue.NativeID,
			"source_path": extensionValue.SourcePath,
			"version":     extensionValue.Version,
			"enabled":     extensionValue.Enabled,
			"fingerprint": extensionValue.Fingerprint,
		} {
			if got, ok := item[key].(string); !ok || got != want {
				t.Fatalf("extension JSON %s = %#v, want string %q", key, item[key], want)
			}
		}
		if values, ok := item["capabilities"].([]any); !ok || len(values) != 2 ||
			values[0] != "read_only" || values[1] != "commands" {
			t.Fatalf("extension JSON capabilities = %#v", item["capabilities"])
		}
		if values, ok := item["diagnostics"].([]any); !ok || len(values) != 2 ||
			values[0] != "version_unverified" || values[1] != "metadata_stale" {
			t.Fatalf("extension JSON diagnostics = %#v", item["diagnostics"])
		}
		if item["managed"] != true || item["drift"] != false {
			t.Fatalf("extension JSON booleans = managed:%#v drift:%#v", item["managed"], item["drift"])
		}
		if strings.Contains(raw, "Inventory from the most recent extension scan") ||
			strings.Contains(raw, "native id:") {
			t.Fatalf("extension JSON contains human text rendering: %s", raw)
		}
	})

	t.Run("backup inspect", func(t *testing.T) {
		envelope, raw := rendererJSONEnvelope(t, "backup.inspect", manifest)

		data, ok := envelope["data"].(map[string]any)
		if !ok {
			t.Fatalf("backup JSON data = %#v, want object", envelope["data"])
		}
		assertRendererKeys(t, data,
			"schema_version", "agentdeck_version", "created_at", "source_platform",
			"database_schemas", "included", "entries",
		)
		if data["schema_version"] != float64(manifest.SchemaVersion) ||
			data["agentdeck_version"] != manifest.AgentDeckVersion ||
			data["created_at"] != manifest.CreatedAt.Format(time.RFC3339Nano) ||
			data["source_platform"] != manifest.SourcePlatform {
			t.Fatalf("backup JSON scalar fields = %#v", data)
		}
		included, ok := data["included"].([]any)
		if !ok || len(included) != 2 || included[0] != "core" || included[1] != "sessions" {
			t.Fatalf("backup JSON included = %#v", data["included"])
		}
		databaseSchemas, ok := data["database_schemas"].(map[string]any)
		if !ok {
			t.Fatalf("backup JSON database_schemas = %#v, want object", data["database_schemas"])
		}
		assertRendererKeys(t, databaseSchemas, "core", "sessions")
		if databaseSchemas["core"] != float64(13) || databaseSchemas["sessions"] != float64(1) {
			t.Fatalf("backup JSON database_schemas = %#v", databaseSchemas)
		}
		entries, ok := data["entries"].([]any)
		if !ok || len(entries) != 2 {
			t.Fatalf("backup JSON entries = %#v", data["entries"])
		}
		wantEntries := []backup.Entry{
			{Name: "agentdeck.sqlite3", Size: 101, SHA256: "synthetic-core-hash"},
			{Name: "sessions.sqlite3", Size: 202, SHA256: "synthetic-sessions-hash"},
		}
		for i, want := range wantEntries {
			entry, ok := entries[i].(map[string]any)
			if !ok {
				t.Fatalf("backup JSON entry %d = %#v, want object", i, entries[i])
			}
			assertRendererKeys(t, entry, "name", "size", "sha256")
			if entry["name"] != want.Name || entry["size"] != float64(want.Size) || entry["sha256"] != want.SHA256 {
				t.Fatalf("backup JSON entry %d = %#v, want %#v", i, entry, want)
			}
		}
		if strings.Contains(raw, "schema version:") || strings.Contains(raw, "included:") {
			t.Fatalf("backup JSON contains human text rendering: %s", raw)
		}
	})
}

func rendererTableRows(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		values := strings.Split(strings.Trim(line, "|"), "|")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		rows = append(rows, values)
	}
	return rows
}

func assertRendererCells(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("table cells = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("table cells = %#v, want %#v", got, want)
		}
	}
}

func assertRendererKeys(t *testing.T, got map[string]any, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("JSON keys = %#v, want exactly %v", got, want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("JSON keys = %#v, missing %q from exact set %v", got, key, want)
		}
	}
}

func rendererJSONEnvelope(t *testing.T, command string, data any) (map[string]any, string) {
	t.Helper()

	var rendered bytes.Buffer
	if err := writeResult(&rendered, "json", command, data); err != nil {
		t.Fatal(err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(rendered.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s JSON: %v\n%s", command, err, rendered.String())
	}
	warnings, warningsOK := envelope["warnings"].([]any)
	if envelope["schema_version"] != float64(output.SchemaVersion) ||
		envelope["command"] != command ||
		envelope["partial"] != false ||
		!warningsOK ||
		len(warnings) != 0 {
		t.Fatalf("%s envelope = %#v", command, envelope)
	}
	if generatedAt, ok := envelope["generated_at"].(string); !ok || generatedAt == "" {
		t.Fatalf("%s generated_at = %#v, want timestamp string", command, envelope["generated_at"])
	}
	if _, exists := envelope["error"]; exists {
		t.Fatalf("%s success envelope contains error: %#v", command, envelope)
	}
	return envelope, rendered.String()
}

func rendererExtensionFixture() extension.DTO {
	return extension.DTO{
		ID:           "codex:mcp:user:synthetic",
		Client:       "codex",
		Kind:         "mcp",
		Scope:        "user",
		NativeID:     "synthetic-server",
		SourcePath:   "/synthetic/extensions.json",
		Version:      "v1.2.3",
		Enabled:      "enabled",
		Capabilities: []string{"read_only", "commands"},
		Diagnostics:  []string{"version_unverified", "metadata_stale"},
		Fingerprint:  "synthetic-fingerprint",
		Managed:      true,
		Drift:        false,
	}
}

func rendererBackupManifestFixture() backup.Manifest {
	return backup.Manifest{
		SchemaVersion:    3,
		AgentDeckVersion: "v1.2.3",
		CreatedAt:        time.Date(2026, 7, 24, 1, 2, 3, 456789123, time.UTC),
		SourcePlatform:   "darwin/arm64",
		DatabaseSchemas:  map[string]int{"core": 13, "sessions": 1},
		Included:         []string{"core", "sessions"},
		Entries: []backup.Entry{
			{Name: "agentdeck.sqlite3", Size: 101, SHA256: "synthetic-core-hash"},
			{Name: "sessions.sqlite3", Size: 202, SHA256: "synthetic-sessions-hash"},
		},
	}
}
