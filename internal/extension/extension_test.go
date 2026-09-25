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
	"time"

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
	if _, err = Scan(context.Background(), db, home, workdir); err != nil {
		t.Fatalf("scan failed on unreadable fingerprint source: %v", err)
	}
	value, err := Show(context.Background(), db, id)
	if err != nil || !value.Managed || value.Drift {
		t.Fatalf("preserved extension = %#v, %v", value, err)
	}
	if len(value.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics on candidate with unreadable fingerprint source, got %#v", value.Diagnostics)
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

type syntheticDiscoverer struct {
	values      []store.Extension
	diagnostics []string
	err         error
}

func (s syntheticDiscoverer) Discover(home, workdir string) ([]store.Extension, []string, error) {
	return s.values, s.diagnostics, s.err
}

func TestSanitizeDiagnostic(t *testing.T) {
	raw := "\x1b[31mError:\x1b[0m   line 1\n\n\t  line 2   \r\n"
	got := SanitizeDiagnostic(raw)
	want := "Error: line 1 line 2"
	if got != want {
		t.Fatalf("SanitizeDiagnostic(%q) = %q, want %q", raw, got, want)
	}
}

func TestSyntheticDiscoveryAndPriorityClassification(t *testing.T) {
	ctx := context.Background()
	state := t.TempDir()
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 1. Missing state root / database: tested via DoctorFromStateRoot with nonexistent dir
	missingReport, err := DoctorFromStateRoot(ctx, filepath.Join(state, "missing"), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if missingReport.Reason != "extension_state_missing" || missingReport.ActionKind != "synchronize_inventory" || missingReport.RecoveryCommand == nil || *missingReport.RecoveryCommand != "agentdeck extension scan" {
		t.Fatalf("state missing report = %#v", missingReport)
	}

	// 2. Discovery failed
	discFail := syntheticDiscoverer{err: fmt.Errorf("scanner failure")}
	rep, err := DoctorWithDiscoverer(ctx, db, discFail, "", "")
	if err != nil || rep.DiscoveryStatus != "failed" || rep.Reason != "extension_discovery_failed" || rep.ActionKind != "manual_prerequisite" || rep.ManualPrerequisite == nil || *rep.ManualPrerequisite != "prereq_extension_discovery_failed" {
		t.Fatalf("discovery failure report = %#v, %v", rep, err)
	}
	if rep.StaleInventory != nil || rep.DuplicateIDs != nil || rep.NativeUnavailable != nil {
		t.Fatalf("expected nil collections on discovery failure, got: %#v", rep)
	}

	// 3. Duplicate IDs
	dupDisc := syntheticDiscoverer{values: []store.Extension{
		{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
		{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
	}}
	rep, err = DoctorWithDiscoverer(ctx, db, dupDisc, "", "")
	if err != nil || rep.Reason != "extension_duplicate_id" || rep.ActionKind != "manual_prerequisite" || rep.ManualPrerequisite == nil || *rep.ManualPrerequisite != "prereq_extension_duplicate_id" {
		t.Fatalf("duplicate IDs report = %#v, %v", rep, err)
	}

	// 4. Stale inventory
	if err = db.ReplaceExtensions(ctx, []store.Extension{
		{ID: "codex:mcp:user:computer-use", Client: "codex", Kind: "mcp", Scope: "user", NativeID: "computer-use", Fingerprint: "fp1"},
	}); err != nil {
		t.Fatal(err)
	}
	emptyDisc := syntheticDiscoverer{values: []store.Extension{}}
	rep, err = DoctorWithDiscoverer(ctx, db, emptyDisc, "", "")
	if err != nil || rep.Reason != "extension_stale_inventory" || rep.ActionKind != "synchronize_inventory" || rep.RecoveryCommand == nil || *rep.RecoveryCommand != "agentdeck extension scan" || rep.ManualPrerequisite != nil {
		t.Fatalf("stale inventory report = %#v, %v", rep, err)
	}
	if len(rep.StaleInventory) != 1 || rep.StaleInventory[0] != "codex:mcp:user:computer-use" {
		t.Fatalf("stale inventory list = %#v", rep.StaleInventory)
	}
	if len(rep.MissingPaths) != 1 || rep.MissingPaths[0] != "codex:mcp:user:computer-use" {
		t.Fatalf("missing paths compatibility = %#v", rep.MissingPaths)
	}

	// 5. Native unavailable
	natUnavailDisc := syntheticDiscoverer{values: []store.Extension{
		{ID: "codex:mcp:user:computer-use", Client: "codex", Kind: "mcp", Scope: "user", NativeID: "computer-use", Fingerprint: "", Diagnostics: []string{"source_unavailable"}},
	}}
	rep, err = DoctorWithDiscoverer(ctx, db, natUnavailDisc, "", "")
	if err != nil || rep.Reason != "extension_native_unavailable" || rep.ActionKind != "manual_prerequisite" || rep.ManualPrerequisite == nil || *rep.ManualPrerequisite != "prereq_extension_native_unavailable" {
		t.Fatalf("native unavailable report = %#v, %v", rep, err)
	}
	if len(rep.NativeUnavailable) != 1 || rep.NativeUnavailable[0] != "codex:mcp:user:computer-use" {
		t.Fatalf("native unavailable list = %#v", rep.NativeUnavailable)
	}

	// 6. Fingerprint sync incomplete marker
	healthyDisc := syntheticDiscoverer{values: []store.Extension{
		{ID: "codex:mcp:user:computer-use", Client: "codex", Kind: "mcp", Scope: "user", NativeID: "computer-use", Fingerprint: "fp1"},
	}}
	if err = db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
		t.Fatal(err)
	}
	rep, err = DoctorWithDiscoverer(ctx, db, healthyDisc, "", "")
	if err != nil || rep.Reason != "extension_fingerprint_update_failed" || rep.ActionKind != "diagnose" || !rep.FingerprintSyncIncomplete {
		t.Fatalf("fingerprint sync incomplete report = %#v, %v", rep, err)
	}
	// Clear marker
	if err = db.SetSetting(ctx, "extension.sync_incomplete", ""); err != nil {
		t.Fatal(err)
	}
	rep, err = DoctorWithDiscoverer(ctx, db, healthyDisc, "", "")
	if err != nil || rep.Reason != "" || rep.ActionKind != "" || rep.FingerprintSyncIncomplete {
		t.Fatalf("healthy report = %#v, %v", rep, err)
	}

	// Priority ordering test: duplicate_id vs stale_inventory (duplicate_id must win)
	dupAndStaleDisc := syntheticDiscoverer{values: []store.Extension{
		{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
		{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
	}}
	rep, err = DoctorWithDiscoverer(ctx, db, dupAndStaleDisc, "", "")
	if err != nil || rep.Reason != "extension_duplicate_id" {
		t.Fatalf("priority: expected duplicate_id to win over stale_inventory, got %#v", rep)
	}
	if len(rep.StaleInventory) != 1 {
		t.Fatalf("expected stale inventory to still be enumerated, got %#v", rep.StaleInventory)
	}
}

func TestDiscoveryFailedAndDatabaseUnreadablePriority(t *testing.T) {
	ctx := context.Background()
	failDisc := syntheticDiscoverer{err: fmt.Errorf("scanner failure")}

	// Case 1: db == nil and discovery failed -> extension_state_missing (Condition 1 > Condition 3)
	repMissing, err := DoctorWithDiscoverer(ctx, nil, failDisc, "", "")
	if err != nil {
		t.Fatalf("expected nil error on db == nil, got %v", err)
	}
	if repMissing.Reason != "extension_state_missing" {
		t.Fatalf("expected extension_state_missing, got %q", repMissing.Reason)
	}
	if repMissing.DiscoveryStatus != "failed" {
		t.Fatalf("expected discovery_status == failed, got %q", repMissing.DiscoveryStatus)
	}
	if repMissing.ActionKind != "manual_prerequisite" || repMissing.ManualPrerequisite == nil || *repMissing.ManualPrerequisite != "prereq_extension_discovery_failed" {
		t.Fatalf("unexpected action mapping: %#v", repMissing)
	}
	if repMissing.StaleInventory != nil || repMissing.DuplicateIDs != nil {
		t.Fatalf("expected nil collections, got: %#v", repMissing)
	}

	// Case 2: corrupted database and discovery failed -> extension_inventory_unreadable (Condition 2 > Condition 3)
	state := t.TempDir()
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ctx, "DROP TABLE extensions"); err != nil {
		t.Fatal(err)
	}

	repUnreadable, err := DoctorWithDiscoverer(ctx, db, failDisc, "", "")
	if err == nil {
		t.Fatal("expected error on unreadable db with failing discovery, got nil")
	}
	var unreadableErr *ErrExtensionInventoryUnreadable
	if !errors.As(err, &unreadableErr) {
		t.Fatalf("expected ErrExtensionInventoryUnreadable, got %v (%T)", err, err)
	}
	if repUnreadable.Reason != "extension_inventory_unreadable" {
		t.Fatalf("expected extension_inventory_unreadable, got %q", repUnreadable.Reason)
	}
	if repUnreadable.DiscoveryStatus != "failed" {
		t.Fatalf("expected discovery_status == failed, got %q", repUnreadable.DiscoveryStatus)
	}
	if repUnreadable.ActionKind != "manual_prerequisite" || repUnreadable.ManualPrerequisite == nil || *repUnreadable.ManualPrerequisite != "prereq_extension_inventory_unreadable" {
		t.Fatalf("unexpected action mapping: %#v", repUnreadable)
	}

	// Case 3: healthy db and discovery failed -> extension_discovery_failed (Condition 3)
	stateHealthy := t.TempDir()
	dbHealthy, err := store.Open(ctx, stateHealthy)
	if err != nil {
		t.Fatal(err)
	}
	defer dbHealthy.Close()

	repHalting, err := DoctorWithDiscoverer(ctx, dbHealthy, failDisc, "", "")
	if err != nil {
		t.Fatalf("expected nil error on healthy db with failing discovery, got %v", err)
	}
	if repHalting.Reason != "extension_discovery_failed" {
		t.Fatalf("expected extension_discovery_failed, got %q", repHalting.Reason)
	}
	if repHalting.DiscoveryStatus != "failed" {
		t.Fatalf("expected discovery_status == failed, got %q", repHalting.DiscoveryStatus)
	}
	if repHalting.ActionKind != "manual_prerequisite" || repHalting.ManualPrerequisite == nil || *repHalting.ManualPrerequisite != "prereq_extension_discovery_failed" {
		t.Fatalf("unexpected action mapping: %#v", repHalting)
	}
	if repHalting.StaleInventory != nil || repHalting.DuplicateIDs != nil || repHalting.DriftedIDs != nil || repHalting.ManagementAnomalies != nil || repHalting.NativeUnavailable != nil {
		t.Fatalf("expected nil collections on discovery failure, got: %#v", repHalting)
	}
}

func TestDoctorPriorityHierarchyTableDriven(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                 string
		setupDB              func(t *testing.T, db *store.Store)
		discoverer           ExtensionDiscoverer
		expectedReason       string
		expectedActionKind   string
		expectedRecoveryCmd  *string
		expectedManualPrereq *string
		verifyCounts         func(t *testing.T, rep DoctorReport)
	}{
		{
			name: "All conditions 4 through 9 present simultaneously -> duplicate_id wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:anomaly", Client: "codex", Kind: "skill", Scope: "user", NativeID: "anomaly",
					},
					{
						ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift",
					},
					{
						ID: "codex:skill:user:stale", Client: "codex", Kind: "skill", Scope: "user", NativeID: "stale",
					},
					{
						ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(ctx, "INSERT INTO extension_management(extension_id, fingerprint, adopted_at) VALUES (?, ?, ?)", "codex:skill:user:anomaly", "", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(ctx, "INSERT INTO extension_management(extension_id, fingerprint, adopted_at) VALUES (?, ?, ?)", "codex:skill:user:drift", "orig_fp", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				// Condition 4: Duplicate IDs
				{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
				{ID: "codex:skill:user:dup", Client: "codex", Kind: "skill", Scope: "user", NativeID: "dup"},
				// Matching Condition 5
				{ID: "codex:skill:user:anomaly", Client: "codex", Kind: "skill", Scope: "user", NativeID: "anomaly"},
				// Matching Condition 6
				{ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift", Managed: true, Fingerprint: "new_fp"},
				// Matching Condition 8
				{ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail", Diagnostics: []string{"unreachable"}},
			}},
			expectedReason:       "extension_duplicate_id",
			expectedActionKind:   "manual_prerequisite",
			expectedManualPrereq: strPtr("prereq_extension_duplicate_id"),
			verifyCounts: func(t *testing.T, rep DoctorReport) {
				if len(rep.DuplicateIDs) != 1 {
					t.Errorf("DuplicateIDs count = %d, want 1", len(rep.DuplicateIDs))
				}
				if len(rep.ManagementAnomalies) != 1 {
					t.Errorf("ManagementAnomalies count = %d, want 1", len(rep.ManagementAnomalies))
				}
				if len(rep.DriftedIDs) != 1 {
					t.Errorf("DriftedIDs count = %d, want 1", len(rep.DriftedIDs))
				}
				if len(rep.StaleInventory) != 1 {
					t.Errorf("StaleInventory count = %d, want 1", len(rep.StaleInventory))
				}
				if len(rep.NativeUnavailable) != 1 {
					t.Errorf("NativeUnavailable count = %d, want 1", len(rep.NativeUnavailable))
				}
				if !rep.FingerprintSyncIncomplete {
					t.Errorf("FingerprintSyncIncomplete = false, want true")
				}
			},
		},
		{
			name: "Conditions 5 through 9 present (no duplicate) -> management_anomaly wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:anomaly", Client: "codex", Kind: "skill", Scope: "user", NativeID: "anomaly",
					},
					{
						ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift",
					},
					{
						ID: "codex:skill:user:stale", Client: "codex", Kind: "skill", Scope: "user", NativeID: "stale",
					},
					{
						ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(ctx, "INSERT INTO extension_management(extension_id, fingerprint, adopted_at) VALUES (?, ?, ?)", "codex:skill:user:anomaly", "", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(ctx, "INSERT INTO extension_management(extension_id, fingerprint, adopted_at) VALUES (?, ?, ?)", "codex:skill:user:drift", "orig_fp", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				{ID: "codex:skill:user:anomaly", Client: "codex", Kind: "skill", Scope: "user", NativeID: "anomaly"},
				{ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift", Managed: true, Fingerprint: "new_fp"},
				{ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail", Diagnostics: []string{"unreachable"}},
			}},
			expectedReason:       "extension_management_anomaly",
			expectedActionKind:   "manual_prerequisite",
			expectedManualPrereq: strPtr("prereq_extension_management_anomaly"),
		},
		{
			name: "Conditions 6 through 9 present -> managed_drift wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift",
					},
					{
						ID: "codex:skill:user:stale", Client: "codex", Kind: "skill", Scope: "user", NativeID: "stale",
					},
					{
						ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(ctx, "INSERT INTO extension_management(extension_id, fingerprint, adopted_at) VALUES (?, ?, ?)", "codex:skill:user:drift", "orig_fp", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				{ID: "codex:skill:user:drift", Client: "codex", Kind: "skill", Scope: "user", NativeID: "drift", Managed: true, Fingerprint: "new_fp"},
				{ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail", Diagnostics: []string{"unreachable"}},
			}},
			expectedReason:       "extension_managed_drift",
			expectedActionKind:   "manual_prerequisite",
			expectedManualPrereq: strPtr("prereq_extension_managed_drift"),
		},
		{
			name: "Conditions 7 through 9 present -> stale_inventory wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:stale", Client: "codex", Kind: "skill", Scope: "user", NativeID: "stale",
					},
					{
						ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				{ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail", Diagnostics: []string{"unreachable"}},
			}},
			expectedReason:      "extension_stale_inventory",
			expectedActionKind:  "synchronize_inventory",
			expectedRecoveryCmd: strPtr("agentdeck extension scan"),
		},
		{
			name: "Conditions 8 and 9 present -> native_unavailable wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				{ID: "codex:skill:user:unavail", Client: "codex", Kind: "skill", Scope: "user", NativeID: "unavail", Diagnostics: []string{"unreachable"}},
			}},
			expectedReason:       "extension_native_unavailable",
			expectedActionKind:   "manual_prerequisite",
			expectedManualPrereq: strPtr("prereq_extension_native_unavailable"),
		},
		{
			name: "Condition 9 only present -> fingerprint_update_failed wins",
			setupDB: func(t *testing.T, db *store.Store) {
				if err := db.ReplaceExtensions(ctx, []store.Extension{
					{
						ID: "codex:skill:user:ok", Client: "codex", Kind: "skill", Scope: "user", NativeID: "ok",
					},
				}); err != nil {
					t.Fatal(err)
				}
				if err := db.SetSetting(ctx, "extension.sync_incomplete", "true"); err != nil {
					t.Fatal(err)
				}
			},
			discoverer: syntheticDiscoverer{values: []store.Extension{
				{ID: "codex:skill:user:ok", Client: "codex", Kind: "skill", Scope: "user", NativeID: "ok"},
			}},
			expectedReason:     "extension_fingerprint_update_failed",
			expectedActionKind: "diagnose",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := t.TempDir()
			db, err := store.Open(ctx, s)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if tc.setupDB != nil {
				tc.setupDB(t, db)
			}
			rep, err := DoctorWithDiscoverer(ctx, db, tc.discoverer, "", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rep.Reason != tc.expectedReason {
				t.Errorf("reason = %q, want %q", rep.Reason, tc.expectedReason)
			}
			if rep.ActionKind != tc.expectedActionKind {
				t.Errorf("action_kind = %q, want %q", rep.ActionKind, tc.expectedActionKind)
			}
			if tc.expectedRecoveryCmd != nil {
				if rep.RecoveryCommand == nil || *rep.RecoveryCommand != *tc.expectedRecoveryCmd {
					t.Errorf("recovery_command = %v, want %v", rep.RecoveryCommand, *tc.expectedRecoveryCmd)
				}
			} else if rep.RecoveryCommand != nil {
				t.Errorf("recovery_command = %v, want nil", *rep.RecoveryCommand)
			}
			if tc.expectedManualPrereq != nil {
				if rep.ManualPrerequisite == nil || *rep.ManualPrerequisite != *tc.expectedManualPrereq {
					t.Errorf("manual_prerequisite = %v, want %v", rep.ManualPrerequisite, *tc.expectedManualPrereq)
				}
			} else if rep.ManualPrerequisite != nil {
				t.Errorf("manual_prerequisite = %v, want nil", *rep.ManualPrerequisite)
			}
			if tc.verifyCounts != nil {
				tc.verifyCounts(t, rep)
			}
		})
	}
}

func TestDoctorReportsNativeUnavailableBeforeFirstInventoryScan(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id := "codex:skill:user:unavailable"
	report, err := DoctorWithDiscoverer(ctx, db, syntheticDiscoverer{
		values: []store.Extension{{ID: id, Diagnostics: []string{"source_unavailable"}}},
	}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if report.Reason != "extension_native_unavailable" || len(report.NativeUnavailable) != 1 || report.NativeUnavailable[0] != id {
		t.Fatalf("unavailable first discovery = %#v", report)
	}
	if report.ActionKind != "manual_prerequisite" || report.RecoveryCommand != nil {
		t.Fatalf("unsafe first-discovery action = %#v", report)
	}
}

func TestUnresolvableNativePathNativeUnavailableResilience(t *testing.T) {
	root, home, workdir := t.TempDir(), t.TempDir(), t.TempDir()
	skillPath := filepath.Join(home, ".codex", "skills", "resilient")
	if err := os.MkdirAll(skillPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("content"), 0600); err != nil {
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

	id := "codex:skill:user:resilient"
	if _, err = Adopt(context.Background(), db, id); err != nil {
		t.Fatal(err)
	}

	// Break source by pointing a symlink inside the skill to a missing path
	if err = os.Symlink(filepath.Join(root, "nonexistent"), filepath.Join(skillPath, "broken_link")); err != nil {
		t.Fatal(err)
	}

	// Discovery and scan must not abort!
	res, scanErr := Scan(context.Background(), db, home, workdir)
	if scanErr != nil {
		t.Fatalf("Scan failed unexpectedly: %v", scanErr)
	}
	if res.Found != 1 {
		t.Fatalf("expected 1 found, got %d", res.Found)
	}

	// Doctor must classify as extension_native_unavailable, NOT extension_managed_drift
	rep, err := Doctor(context.Background(), db, home, workdir)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Reason != "extension_native_unavailable" {
		t.Fatalf("expected extension_native_unavailable, got %q", rep.Reason)
	}
	if len(rep.DriftedIDs) != 0 {
		t.Fatalf("expected 0 drifted IDs, got %#v", rep.DriftedIDs)
	}
	if len(rep.NativeUnavailable) != 1 || rep.NativeUnavailable[0] != id {
		t.Fatalf("expected %s in native unavailable, got %#v", id, rep.NativeUnavailable)
	}
}

func TestInvalidCanonicalIDInformational(t *testing.T) {
	ctx := context.Background()
	state := t.TempDir()
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	disc := syntheticDiscoverer{
		values:      []store.Extension{},
		diagnostics: []string{"invalid extension identity"},
	}
	rep, err := DoctorWithDiscoverer(ctx, db, disc, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if rep.DiscoveryStatus != "ok" {
		t.Fatalf("expected discovery_status ok, got %q", rep.DiscoveryStatus)
	}
	if rep.Reason != "" {
		t.Fatalf("expected empty reason, got %q", rep.Reason)
	}
	if len(rep.Diagnostics) != 1 || rep.Diagnostics[0] != "invalid extension identity" {
		t.Fatalf("expected sanitized diagnostic, got %#v", rep.Diagnostics)
	}
	if rep.CountForReason() != 0 {
		t.Fatalf("expected CountForReason = 0, got %d", rep.CountForReason())
	}
}

func TestSettingUnreadableReturnsErrExtensionInventoryUnreadable(t *testing.T) {
	ctx := context.Background()
	state := t.TempDir()
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Drop settings table to simulate unreadable/corrupted settings table
	if _, err := db.Exec(ctx, "DROP TABLE settings"); err != nil {
		t.Fatal(err)
	}

	rep, docErr := Doctor(ctx, db, "", "")
	if docErr == nil {
		t.Fatal("expected error on unreadable settings table, got nil")
	}
	var unreadable *ErrExtensionInventoryUnreadable
	if !errors.As(docErr, &unreadable) {
		t.Fatalf("expected *ErrExtensionInventoryUnreadable, got %T: %v", docErr, docErr)
	}
	if rep.Reason != "extension_inventory_unreadable" {
		t.Fatalf("expected extension_inventory_unreadable, got %q", rep.Reason)
	}
	if rep.ActionKind != "manual_prerequisite" {
		t.Fatalf("expected manual_prerequisite, got %q", rep.ActionKind)
	}
	if rep.ManualPrerequisite == nil || *rep.ManualPrerequisite != "prereq_extension_inventory_unreadable" {
		t.Fatalf("unexpected manual_prerequisite: %#v", rep.ManualPrerequisite)
	}
}
