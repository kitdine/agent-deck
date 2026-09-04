package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestUsageHookEventRejectsInvalidTranscriptAndNonBoundaryEvents(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
		VALUES(1,'codex','official','x','1','2026-08-04T00:00:00Z');
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(home, ".codex", "sessions", "valid-session.jsonl")
	if err = os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mismatchedTranscript := filepath.Join(filepath.Dir(transcript), "other-session.jsonl")
	if err = os.WriteFile(mismatchedTranscript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	outsideTranscript := filepath.Join(home, "outside-valid-session.jsonl")
	if err = os.WriteFile(outsideTranscript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkTranscript := filepath.Join(filepath.Dir(transcript), "valid-session-link.jsonl")
	if err = os.Symlink(outsideTranscript, symlinkTranscript); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	deliver := func(eventName, source, path string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      "valid-session",
			"transcript_path": path,
			"hook_event_name": eventName,
			"source":          source,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var stdout bytes.Buffer
		if err := run([]string{"--state-dir", state, "usage", "hook", "event", "codex"}, bytes.NewReader(payload), &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook event stdout = %q", stdout.String())
		}
	}
	countRoutes := func() int {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		var count int
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&count); queryErr != nil {
			t.Fatal(queryErr)
		}
		return count
	}
	countObservations := func() int {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		var count int
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&count); queryErr != nil {
			t.Fatal(queryErr)
		}
		return count
	}

	deliver("SessionStart", "resume", filepath.Join(home, "outside", "valid-session.jsonl"))
	deliver("SessionStart", "resume", mismatchedTranscript)
	deliver("SessionStart", "resume", symlinkTranscript)
	deliver("ConfigChange", "", transcript)
	if got := countRoutes(); got != 0 {
		t.Fatalf("invalid events wrote %d routes", got)
	}
	if got := countObservations(); got != 0 {
		t.Fatalf("invalid events wrote %d observations", got)
	}
	deliver("SessionStart", "resume", transcript)
	if got := countRoutes(); got != 1 {
		t.Fatalf("valid event wrote %d routes, want 1", got)
	}
	if got := countObservations(); got != 1 {
		t.Fatalf("valid event wrote %d observations, want 1", got)
	}
}

func TestUsageHookEventAcceptsClaudeStartupBeforeTranscriptExists(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
		VALUES(1,'claude','official','x','1','2026-09-01T00:00:00Z');
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(home, ".claude", "projects", "-fixture")
	if err = os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	deliver := func(sessionID, source, transcriptPath string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      sessionID,
			"transcript_path": transcriptPath,
			"hook_event_name": "SessionStart",
			"source":          source,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var stdout bytes.Buffer
		if runErr := run([]string{"--state-dir", state, "usage", "hook", "event", "claude"}, bytes.NewReader(payload), &stdout); runErr != nil {
			t.Fatal(runErr)
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook event stdout = %q", stdout.String())
		}
	}
	countRoutes := func() int {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		var count int
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&count); queryErr != nil {
			t.Fatal(queryErr)
		}
		return count
	}

	deliver("startup-session", "startup", filepath.Join(projectDir, "startup-session.jsonl"))
	if got := countRoutes(); got != 1 {
		t.Fatalf("startup before transcript wrote %d routes, want 1", got)
	}
	deliver("resume-session", "resume", filepath.Join(projectDir, "resume-session.jsonl"))
	if got := countRoutes(); got != 1 {
		t.Fatalf("missing resume transcript changed routes to %d, want 1", got)
	}
	outsideDir := filepath.Join(home, "outside")
	if err = os.MkdirAll(outsideDir, 0o700); err != nil {
		t.Fatal(err)
	}
	deliver("outside-session", "startup", filepath.Join(outsideDir, "outside-session.jsonl"))
	if got := countRoutes(); got != 1 {
		t.Fatalf("outside startup transcript changed routes to %d, want 1", got)
	}
	symlinkDir := filepath.Join(home, ".claude", "projects", "-linked")
	if err = os.Symlink(outsideDir, symlinkDir); err != nil {
		t.Fatal(err)
	}
	deliver("linked-session", "startup", filepath.Join(symlinkDir, "linked-session.jsonl"))
	if got := countRoutes(); got != 1 {
		t.Fatalf("symlinked startup transcript changed routes to %d, want 1", got)
	}
}

// Claude fires SessionStart before it writes the transcript, and `startup` is
// not the only source that begins a session with a new id and a new file:
// `clear` and `fork` do too. A brand-new project widens the same gap, because
// its ~/.claude/projects/<project>/ directory is created together with the
// first transcript rather than before it, so resolving only the immediate
// parent still fails on that session. Both edges drop a route silently, which
// is the shape ad-bug-claude-startup-route fixed for one source only.
//
// The containment check must survive the widening: a path outside the projects
// root stays rejected even when its own directory is missing, so the admission
// is by resolved location and never by source alone.
func TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name        string
		source      string
		project     string
		createDir   bool
		outsideHome bool
		danglingTo  string
		wantRoutes  int
	}{
		{name: "startup with an existing project directory", source: "startup", project: "-fixture", createDir: true, wantRoutes: 1},
		{name: "clear with an existing project directory", source: "clear", project: "-fixture", createDir: true, wantRoutes: 1},
		{name: "fork with an existing project directory", source: "fork", project: "-fixture", createDir: true, wantRoutes: 1},
		{name: "startup in a brand-new project", source: "startup", project: "-brand-new", wantRoutes: 1},
		{name: "clear in a brand-new project", source: "clear", project: "-brand-new", wantRoutes: 1},
		{name: "fork in a brand-new project", source: "fork", project: "-brand-new", wantRoutes: 1},
		// resume names a session whose transcript Claude has already written, so
		// a missing file is a mismatch rather than a timing gap.
		{name: "resume without a transcript stays rejected", source: "resume", project: "-fixture", createDir: true, wantRoutes: 0},
		{name: "startup outside the projects root stays rejected", source: "startup", project: "-fixture", outsideHome: true, wantRoutes: 0},
		{name: "clear outside the projects root stays rejected", source: "clear", project: "-fixture", outsideHome: true, wantRoutes: 0},
		// A dangling symlink already inside the projects root reports the same
		// ErrNotExist as a directory Claude has not created yet, so admitting by
		// "not created yet" alone would step over a component that does exist
		// and lands outside the root the moment its target appears. The
		// component exists; only its target does not, so it must resolve or be
		// rejected -- never be treated as a name to rejoin lexically.
		{name: "startup through a dangling symlink to an uncreated target outside the root stays rejected", source: "startup", project: "-dangling", danglingTo: "outside/not-created-yet", wantRoutes: 0},
		{name: "clear through a dangling symlink to an uncreated target outside the root stays rejected", source: "clear", project: "-dangling", danglingTo: "outside/not-created-yet", wantRoutes: 0},
		{name: "fork through a dangling symlink to an uncreated target outside the root stays rejected", source: "fork", project: "-dangling", danglingTo: "outside/not-created-yet", wantRoutes: 0},
		// Same shape with a target that would land back inside the root: the
		// admission still refuses, because a component that exists has to
		// resolve. Claude never creates these, so refusing is the safe reading.
		{name: "startup through a dangling symlink to an uncreated target inside the root stays rejected", source: "startup", project: "-dangling", danglingTo: ".claude/projects/not-created-yet", wantRoutes: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := filepath.Join(t.TempDir(), "state")
			home := t.TempDir()
			database, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = database.Exec(ctx, `
				INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
				VALUES(1,'official','x','','1','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
				INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
				VALUES(1,'claude','official','x','1','2026-09-01T00:00:00Z');
			`); err != nil {
				database.Close()
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o700); err != nil {
				t.Fatal(err)
			}
			projectDir := filepath.Join(home, ".claude", "projects", test.project)
			if test.outsideHome {
				projectDir = filepath.Join(home, "outside", test.project)
			}
			if test.createDir {
				if err = os.MkdirAll(projectDir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if test.danglingTo != "" {
				// The link exists; its target does not. Nothing along the target
				// path is created, so EvalSymlinks on the link reports
				// ErrNotExist even though Lstat on it succeeds.
				if err = os.Symlink(filepath.Join(home, test.danglingTo), projectDir); err != nil {
					t.Fatal(err)
				}
				if info, statErr := os.Lstat(projectDir); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("dangling fixture is not a symlink: %#v, %v", info, statErr)
				}
				if _, statErr := os.Stat(projectDir); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("dangling fixture resolves to something: %v", statErr)
				}
			}
			oldHome := userHomeDir
			userHomeDir = func() (string, error) { return home, nil }
			t.Cleanup(func() { userHomeDir = oldHome })

			payload, marshalErr := json.Marshal(map[string]string{
				"session_id":      "edge-session",
				"transcript_path": filepath.Join(projectDir, "edge-session.jsonl"),
				"hook_event_name": "SessionStart",
				"source":          test.source,
			})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var stdout bytes.Buffer
			if runErr := run([]string{"--state-dir", state, "usage", "hook", "event", "claude"}, bytes.NewReader(payload), &stdout); runErr != nil {
				t.Fatal(runErr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("hook event stdout = %q", stdout.String())
			}
			database, err = store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var routes int
			if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); err != nil {
				t.Fatal(err)
			}
			if routes != test.wantRoutes {
				t.Fatalf("routes = %d, want %d", routes, test.wantRoutes)
			}
		})
	}
}

func TestUsageHookEventRejectsUnmanagedClaudeConfigChangeSources(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err = os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	deliver := func(source, path string) {
		t.Helper()
		payload, marshalErr := json.Marshal(map[string]string{
			"session_id":      "valid-session",
			"hook_event_name": "ConfigChange",
			"source":          source,
			"file_path":       path,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var stdout bytes.Buffer
		if err := run([]string{"--state-dir", state, "usage", "hook", "event", "claude"}, bytes.NewReader(payload), &stdout); err != nil {
			t.Fatal(err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook event stdout = %q", stdout.String())
		}
	}
	counts := func() (routes, observations int) {
		t.Helper()
		database, openErr := store.Open(ctx, state)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer database.Close()
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); queryErr != nil {
			t.Fatal(queryErr)
		}
		if queryErr := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&observations); queryErr != nil {
			t.Fatal(queryErr)
		}
		return routes, observations
	}

	for _, source := range []string{"project_settings", "local_settings", "policy_settings", "skills"} {
		deliver(source, configPath)
	}
	unmanagedPath := filepath.Join(home, ".claude", "settings.local.json")
	if err = os.WriteFile(unmanagedPath, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deliver("user_settings", unmanagedPath)
	if routes, observations := counts(); routes != 0 || observations != 0 {
		t.Fatalf("unmanaged ConfigChange sources wrote routes=%d observations=%d, want 0/0", routes, observations)
	}
}
