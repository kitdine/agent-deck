// Package claudememtest builds synthetic but faithful claude-mem 15.2.1
// databases for isolated tests. It never touches a real claude-mem root.
package claudememtest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// Schema reproduces the reviewed 15.2.1 SessionStore shape: sdk_sessions as
// rebuilt with composite identity (memory_session_id TEXT UNIQUE),
// schema_versions, and content tables the adapter must never read.
const Schema = `
CREATE TABLE schema_versions (id INTEGER PRIMARY KEY, version INTEGER UNIQUE NOT NULL, applied_at TEXT NOT NULL);
CREATE TABLE sdk_sessions (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 content_session_id TEXT NOT NULL,
 memory_session_id TEXT UNIQUE,
 project TEXT NOT NULL,
 platform_source TEXT NOT NULL DEFAULT 'claude',
 user_prompt TEXT,
 started_at TEXT NOT NULL,
 started_at_epoch INTEGER NOT NULL,
 completed_at TEXT,
 completed_at_epoch INTEGER,
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'completed', 'failed')),
 worker_port INTEGER,
 prompt_counter INTEGER DEFAULT 0,
 custom_title TEXT
);
CREATE INDEX idx_sdk_sessions_claude_id ON sdk_sessions(content_session_id);
CREATE INDEX idx_sdk_sessions_sdk_id ON sdk_sessions(memory_session_id);
CREATE INDEX idx_sdk_sessions_project ON sdk_sessions(project);
CREATE TABLE user_prompts (id INTEGER PRIMARY KEY, content_session_id TEXT, prompt_text TEXT);
CREATE TABLE observations (id INTEGER PRIMARY KEY, memory_session_id TEXT, text TEXT);
`

// Secret is content text the adapter must never read or emit.
const Secret = "SECRET-PROMPT-TEXT"

// Mapping is one sdk_sessions row.
type Mapping struct {
	RowID     int64
	ContentID string
	MemoryID  string
}

// Fixture is an active WAL database held open by a writer, like a running
// claude-mem worker, so its -wal and -shm sidecars exist.
type Fixture struct {
	Dir    string
	Path   string
	Writer *sql.DB
}

// New writes a fixture with schema versions 1..version, the given mappings
// and any extra statements, under a fresh temporary root.
func New(t testing.TB, version int, mappings []Mapping, extra ...string) *Fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "claude-mem")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "claude-mem.db")
	writer, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	writer.SetMaxOpenConns(1)
	t.Cleanup(func() { writer.Close() })
	statements := []string{Schema}
	for v := 1; v <= version; v++ {
		statements = append(statements, fmt.Sprintf(`INSERT INTO schema_versions(version, applied_at) VALUES(%d, '2026-10-01T00:00:00Z')`, v))
	}
	for _, m := range mappings {
		statements = append(statements, fmt.Sprintf(`INSERT INTO sdk_sessions(id, content_session_id, memory_session_id, project, user_prompt, started_at, started_at_epoch) VALUES(%d, '%s', '%s', 'proj', '%s', '2026-10-01T00:00:00Z', 1)`,
			m.RowID, m.ContentID, m.MemoryID, Secret))
		statements = append(statements, fmt.Sprintf(`INSERT INTO user_prompts(content_session_id, prompt_text) VALUES('%s', '%s')`, m.ContentID, Secret))
	}
	statements = append(statements, extra...)
	for _, statement := range statements {
		if _, err := writer.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); err != nil {
			t.Fatalf("fixture is not an active WAL database: %v", err)
		}
	}
	return &Fixture{Dir: dir, Path: path, Writer: writer}
}

// Install is one plugin catalog entry and, when Package is set, the package
// manifest written at its install path.
type Install struct {
	ID          string
	Scope       string
	Version     string
	PackageName string
	Package     string // package.json version; empty writes no package
	ProjectPath string
}

// ReviewedInstall is the reviewed claude-mem-plugin 15.2.1 install.
func ReviewedInstall() Install {
	return Install{ID: "claude-mem@thedotmack", Scope: "user", Version: "15.2.1", PackageName: "claude-mem-plugin", Package: "15.2.1"}
}

// Home writes an isolated home with Claude Code's installed plugin catalog
// holding the given installs, each with its package manifest.
func Home(t testing.TB, installs ...Install) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	plugins := filepath.Join(home, ".claude", "plugins")
	entries := map[string][]map[string]string{}
	for index, install := range installs {
		path := filepath.Join(plugins, "cache", "thedotmack", "claude-mem", fmt.Sprintf("%s-%d", install.Version, index))
		if install.Package != "" {
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			manifest := fmt.Sprintf(`{"name":%q,"version":%q}`, install.PackageName, install.Package)
			if err := os.WriteFile(filepath.Join(path, "package.json"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		entry := map[string]string{"scope": install.Scope, "version": install.Version, "installPath": path}
		if install.ProjectPath != "" {
			entry["projectPath"] = install.ProjectPath
		}
		entries[install.ID] = append(entries[install.ID], entry)
	}
	if err := os.MkdirAll(plugins, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"version": 2, "plugins": entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugins, "installed_plugins.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}
