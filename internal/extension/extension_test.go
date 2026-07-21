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

func TestScanSkillsFollowsValidLinksAndDiscoversSystemSkills(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	target := filepath.Join(root, "linked-skill")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(home, ".codex", "skills")
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(base, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, ".system", "builtin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, ".system", "builtin", "SKILL.md"), []byte("system"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, ".backups", "ignored"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, ".backups", "ignored", "SKILL.md"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result, err := Scan(context.Background(), db, home, workdir)
	if err != nil || result.Found != 2 {
		t.Fatalf("Scan = %#v, %v", result, err)
	}
	linked, err := Show(context.Background(), db, "codex:skill:user:linked")
	if err != nil {
		t.Fatal(err)
	}
	before := linked.Fingerprint
	if err = os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatal(err)
	}
	linked, err = Show(context.Background(), db, "codex:skill:user:linked")
	if err != nil || linked.Fingerprint == before {
		t.Fatalf("linked fingerprint = %#v, %v", linked, err)
	}
}

func TestSkillSymlinkLifecyclePreservesAdoptionAndInventory(t *testing.T) {
	for _, kind := range []string{"ordinary", "system_child", "system_directory"} {
		t.Run(kind, func(t *testing.T) {
			root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
			base := filepath.Join(home, ".codex", "skills")
			if err := os.MkdirAll(base, 0o700); err != nil {
				t.Fatal(err)
			}
			targetA, targetB := filepath.Join(root, "target-a"), filepath.Join(root, "target-b")
			linkPath := filepath.Join(base, "linked")
			skillPath := func(target string) string { return filepath.Join(target, "SKILL.md") }
			id := "codex:skill:user:linked"
			switch kind {
			case "system_child":
				if err := os.MkdirAll(filepath.Join(base, ".system"), 0o700); err != nil {
					t.Fatal(err)
				}
				linkPath = filepath.Join(base, ".system", "linked")
				id = "codex:skill:user:.system/linked"
			case "system_directory":
				linkPath = filepath.Join(base, ".system")
				skillPath = func(target string) string { return filepath.Join(target, "linked", "SKILL.md") }
				id = "codex:skill:user:.system/linked"
			}
			writeTarget := func(target, contents string) {
				t.Helper()
				path := skillPath(target)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writeTarget(targetA, "one")
			writeTarget(targetB, "two")
			if err := os.Symlink(targetA, linkPath); err != nil {
				t.Fatal(err)
			}
			database, err := store.Open(context.Background(), filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			result, err := Scan(context.Background(), database, home, workdir)
			if err != nil || result.Added != 1 || result.Found != 1 {
				t.Fatalf("initial scan = %#v, %v", result, err)
			}
			adopted, err := Adopt(context.Background(), database, id)
			if err != nil || !adopted.Managed || adopted.Drift {
				t.Fatalf("adopt = %#v, %v", adopted, err)
			}
			var adoptedFingerprint string
			if err = database.DB.QueryRowContext(context.Background(), "SELECT fingerprint FROM extension_management WHERE extension_id=?", id).Scan(&adoptedFingerprint); err != nil || adoptedFingerprint == "" {
				t.Fatalf("adopted fingerprint = %q, %v", adoptedFingerprint, err)
			}
			writeTarget(targetA, "one changed")
			result, err = Scan(context.Background(), database, home, workdir)
			changed, showErr := Show(context.Background(), database, id)
			if err != nil || showErr != nil || result.Updated != 1 || !changed.Managed || !changed.Drift || changed.Fingerprint == adoptedFingerprint {
				t.Fatalf("content change result=%#v extension=%#v err=%v/%v", result, changed, err, showErr)
			}
			if err = os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(targetB, linkPath); err != nil {
				t.Fatal(err)
			}
			result, err = Scan(context.Background(), database, home, workdir)
			switched, showErr := Show(context.Background(), database, id)
			if err != nil || showErr != nil || result.Updated != 1 || switched.Fingerprint == changed.Fingerprint || !switched.Managed || !switched.Drift {
				t.Fatalf("target switch result=%#v extension=%#v err=%v/%v", result, switched, err, showErr)
			}
			assertPreserved := func(stage string) {
				t.Helper()
				value, showErr := Show(context.Background(), database, id)
				if showErr != nil || value.Fingerprint != switched.Fingerprint || !value.Managed || !value.Drift {
					t.Fatalf("%s inventory = %#v, %v", stage, value, showErr)
				}
				var fingerprint string
				if queryErr := database.DB.QueryRowContext(context.Background(), "SELECT fingerprint FROM extension_management WHERE extension_id=?", id).Scan(&fingerprint); queryErr != nil || fingerprint != adoptedFingerprint {
					t.Fatalf("%s adopted fingerprint = %q, %v", stage, fingerprint, queryErr)
				}
			}
			if err = os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(filepath.Join(root, "missing"), linkPath); err != nil {
				t.Fatal(err)
			}
			if _, err = Scan(context.Background(), database, home, workdir); err == nil {
				t.Fatal("broken link scan succeeded")
			}
			assertPreserved("broken link")
			if err = os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(targetB, linkPath); err != nil {
				t.Fatal(err)
			}
			if result, err = Scan(context.Background(), database, home, workdir); err != nil || result.Unchanged != 1 {
				t.Fatalf("broken link recovery = %#v, %v", result, err)
			}
			assertPreserved("broken link recovery")
			if err = os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(linkPath, linkPath); err != nil {
				t.Fatal(err)
			}
			if _, err = Scan(context.Background(), database, home, workdir); err == nil {
				t.Fatal("symlink cycle scan succeeded")
			}
			assertPreserved("symlink cycle")
			if err = os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(targetB, linkPath); err != nil {
				t.Fatal(err)
			}
			if result, err = Scan(context.Background(), database, home, workdir); err != nil || result.Unchanged != 1 {
				t.Fatalf("cycle recovery = %#v, %v", result, err)
			}
			assertPreserved("cycle recovery")
		})
	}
}
