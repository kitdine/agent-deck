package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCanonicalIDRejectsAmbiguousParts(t *testing.T) {
	if _, err := CanonicalID("codex", "mcp", "user", "github:bad"); err == nil {
		t.Fatal("accepted ambiguous native ID")
	}
	if got, err := CanonicalID("claude", "plugin", "project", "sample"); err != nil || got != "claude:plugin:project:sample" {
		t.Fatalf("CanonicalID = %q, %v", got, err)
	}
}

func TestScanPersistsNativeInventoryWithoutCopyingContent(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.github]\ncommand = 'ignored'\n")
	write(filepath.Join(home, ".claude.json"), `{"mcpServers":{"filesystem":{"command":"ignored"}}}`)
	write(filepath.Join(home, ".codex", "plugins", "cache", "market", "example", "1.2.3", ".codex-plugin", "plugin.json"), `{}`)
	write(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), fmt.Sprintf(`{"plugins":{"sample@market":[{"scope":"user","version":"2.0.0"}],"project@market":[{"scope":"project","projectPath":%q,"version":"3.0.0"}]}}`, workdir))
	write(filepath.Join(workdir, ".codex", "skills", "local", "SKILL.md"), "private instructions")
	write(filepath.Join(home, ".claude", "skills", "user-skill", "SKILL.md"), "private instructions")
	write(filepath.Join(workdir, ".claude", "skills", "project-skill", "SKILL.md"), "private instructions")
	write(filepath.Join(home, "skills", "wrong-user-skill", "SKILL.md"), "must not be scanned")
	write(filepath.Join(workdir, "skills", "wrong-project-skill", "SKILL.md"), "must not be scanned")
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result, err := Scan(context.Background(), db, home, workdir)
	if err != nil || result.Found != 8 {
		t.Fatalf("Scan = %#v, %v", result, err)
	}
	values, err := List(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]DTO, len(values))
	for _, value := range values {
		byID[value.ID] = value
		if len(value.Capabilities) != 1 || value.Capabilities[0] != ReadOnlyCapability {
			t.Fatalf("capabilities = %#v", value.Capabilities)
		}
	}
	for _, id := range []string{"claude:skill:user:user-skill", "claude:skill:project:project-skill", "claude:plugin:project:project@market"} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("missing native extension %q: %#v", id, values)
		}
	}
	for _, id := range []string{"claude:skill:user:wrong-user-skill", "claude:skill:project:wrong-project-skill"} {
		if _, ok := byID[id]; ok {
			t.Fatalf("scanned non-native path as %q", id)
		}
	}
	if got := byID["codex:plugin:user:example@market"].Version; got != "1.2.3" {
		t.Fatalf("Codex plugin version = %q", got)
	}
	if err := SetEnabled(context.Background(), db, "codex:mcp:user:github", false); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("SetEnabled = %v", err)
	}
}

func TestCodexPluginMultipleCachedVersionsAreUnknown(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "plugin")
	for _, version := range []string{"9", "10"} {
		if err := os.MkdirAll(filepath.Join(pluginPath, version), 0700); err != nil {
			t.Fatal(err)
		}
	}
	version, sourcePath, err := codexPluginVersion(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if version != unknown || sourcePath != pluginPath {
		t.Fatalf("codexPluginVersion = %q, %q", version, sourcePath)
	}
}

func TestScanIsAtomicAndDoctorReportsParseFailureAndDrift(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	path := filepath.Join(home, ".codex", "skills", "one", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatal(err)
	}
	id := "codex:skill:user:one"
	if _, err = Adopt(context.Background(), db, id); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Doctor(context.Background(), db, home, workdir)
	if err != nil || len(report.DriftedIDs) != 1 || report.DriftedIDs[0] != id {
		t.Fatalf("Doctor = %#v, %v", report, err)
	}
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("[mcp_servers"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Scan(context.Background(), db, home, workdir); err == nil {
		t.Fatal("scan accepted invalid required source")
	}
	values, err := List(context.Background(), db)
	if err != nil || len(values) != 1 || values[0].ID != id {
		t.Fatalf("atomic inventory = %#v, %v", values, err)
	}
	report, err = Doctor(context.Background(), db, home, workdir)
	if err != nil || len(report.Diagnostics) == 0 {
		t.Fatalf("doctor parse report = %#v, %v", report, err)
	}
}

func TestFingerprintFailurePreservesInventoryAndManagement(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	skillPath := filepath.Join(home, ".codex", "skills", "one")
	if err := os.MkdirAll(skillPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatal(err)
	}
	id := "codex:skill:user:one"
	if _, err = Adopt(context.Background(), db, id); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(root, "missing"), filepath.Join(skillPath, "broken")); err != nil {
		t.Fatal(err)
	}
	if _, err = Scan(context.Background(), db, home, workdir); err == nil {
		t.Fatal("scan accepted an unreadable fingerprint source")
	}
	value, err := Show(context.Background(), db, id)
	if err != nil || !value.Managed || value.Drift {
		t.Fatalf("preserved extension = %#v, %v", value, err)
	}
}

func TestInventoryNeverStoresOrReturnsSensitiveSourceContent(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	secret := "credential-secret ENV_VALUE private-config"
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"safe":{"env":{"TOKEN":"`+secret+`"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatal(err)
	}
	values, err := List(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(values)
	if bytes.Contains(encoded, []byte(secret)) {
		t.Fatalf("JSON exposed secret: %s", encoded)
	}
	var count int
	if err = db.DB.QueryRow("SELECT count(*) FROM extensions WHERE diagnostics_json LIKE ? OR fingerprint LIKE ?", "%"+secret+"%", "%"+secret+"%").Scan(&count); err != nil || count != 0 {
		t.Fatalf("store contains secret count=%d err=%v", count, err)
	}
}

func TestAdoptReleaseAndRescanRespectFingerprintState(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	path := filepath.Join(home, ".codex", "skills", "one", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatal(err)
	}
	id := "codex:skill:user:one"
	if value, err := Adopt(context.Background(), db, id); err != nil || !value.Managed || value.Drift {
		t.Fatalf("Adopt = %#v, %v", value, err)
	}
	if err := Release(context.Background(), db, id); err != nil {
		t.Fatal(err)
	}
	if value, err := Show(context.Background(), db, id); err != nil || value.Managed {
		t.Fatalf("Show = %#v, %v", value, err)
	}
}
