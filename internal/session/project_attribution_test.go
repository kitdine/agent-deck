package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestCodexCanonicalProjectAttribution(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "shared"}[shared], func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			home := filepath.Join(root, "home")
			path := filepath.Join(home, ".codex", "sessions", "2026", "09", "29", "rollout.jsonl")
			contents := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"canonical\",\"session_id\":\"legacy\",\"cwd\":\"/work/agent-deck\"}}\n" +
				"{\"type\":\"response_item\",\"payload\":{\"id\":\"message-id\",\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"needle prompt\"}]}}\n"
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			scan := func() ScanResult {
				t.Helper()
				options := ScanOptions{}
				if shared {
					sources, err := ingest.Discover(home)
					if err != nil {
						t.Fatal(err)
					}
					coordinator := ingest.NewCoordinator(sources, ingest.Options{RequirePlans: true})
					for _, source := range sources {
						coordinator.Skip(source.Path, ingest.ConsumerUsage)
					}
					if err := coordinator.Seal(ingest.ConsumerUsage); err != nil {
						t.Fatal(err)
					}
					if err := PlanForCoordinator(ctx, database.DB, home, sources, coordinator); err != nil {
						t.Fatal(err)
					}
					coordinator.Start(ctx)
					defer coordinator.Close()
					options = ScanOptions{PreparedSources: sources, Coordinator: coordinator}
				}
				result, err := ScanWithOptions(ctx, database.DB, home, options)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			scan()
			assertIdentity := func(wantDocs int) {
				t.Helper()
				items, err := List(ctx, database.DB)
				if err != nil || len(items) != 1 || items[0].SessionID != "canonical" || items[0].Project != "/work/agent-deck" {
					t.Fatalf("canonical identity lost: %#v, %v", items, err)
				}
				docs, err := Search(ctx, database.DB, "needle")
				if err != nil || len(docs) != wantDocs {
					t.Fatalf("documents=%#v, %v", docs, err)
				}
				for _, doc := range docs {
					if doc.SessionID != "canonical" {
						t.Fatalf("wrong document identity: %#v", doc)
					}
				}
			}
			assertIdentity(1)
			parsed, err := parseFile("codex", path)
			if err != nil || len(parsed) != 1 || parsed[0].SessionID != "canonical" || len(parsed[0].Documents) != 1 {
				t.Fatalf("fixture reader=%#v, %v", parsed, err)
			}
			// Real Codex appends need not repeat either session ID or cwd.
			appendix := "{\"type\":\"turn_context\",\"payload\":{\"cwd\":\"/work/subdir\"}}\n{\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"needle answer\"}]}}\n"
			if err := os.WriteFile(path, []byte(contents+appendix), 0600); err != nil {
				t.Fatal(err)
			}
			scan()
			assertIdentity(2)
			fresh, err := store.OpenSessions(ctx, filepath.Join(root, "fresh"))
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.Close()
			if _, err := Scan(ctx, fresh.DB, home); err != nil {
				t.Fatal(err)
			}
			incremental, err := List(ctx, database.DB)
			if err != nil {
				t.Fatal(err)
			}
			full, err := List(ctx, fresh.DB)
			if err != nil || !reflect.DeepEqual(incremental, full) {
				t.Fatalf("append/full metadata differ: %#v / %#v, %v", incremental, full, err)
			}
			if result := scan(); result.Sources != 0 || result.Skipped != 1 {
				t.Fatalf("unchanged scan=%+v", result)
			}
		})
	}
}

func TestCodexExcludedTailRetainsSessionContext(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "excluded.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Exclude(ctx, database.DB, "session", "excluded"); err != nil {
		t.Fatal(err)
	}
	contents := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"visible\",\"cwd\":\"/work/visible\"}}\n{\"type\":\"visible_user_prompt\",\"session_id\":\"excluded\",\"cwd\":\"/work/excluded\",\"payload\":{\"text\":\"needle hidden\"}}\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, database.DB, home); err != nil {
		t.Fatal(err)
	}
	contents += "{\"type\":\"visible_assistant_final\",\"payload\":{\"text\":\"needle still hidden\"}}\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, database.DB, home); err != nil {
		t.Fatal(err)
	}
	docs, err := Search(ctx, database.DB, "needle")
	if err != nil || len(docs) != 0 {
		t.Fatalf("excluded tail reassigned: %#v, %v", docs, err)
	}
}

func TestCodexProjectFallbackPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, record, id, project string }{
		{"canonical", `{"type":"session_meta","session_id":"outer","payload":{"id":"canonical","session_id":"legacy","cwd":"/work/canonical","project":"/wrong"}}`, "canonical", "/work/canonical"},
		{"cwd-before-project", `{"type":"session_meta","cwd":"/work/top","payload":{"session_id":"legacy","project":"/wrong"}}`, "legacy", "/work/top"},
		{"payload-cwd", `{"type":"session_meta","cwd":"/wrong","payload":{"id":"canonical","cwd":"/work/payload"}}`, "canonical", "/work/payload"},
		{"top-id", `{"type":"session_meta","session_id":"top","sessionId":"alias","project":"/work/project","payload":{}}`, "top", "/work/project"},
		{"alias", `{"type":"session_meta","sessionId":"alias","payload":{}}`, "alias", ""},
		{"message-id-is-not-session", `{"type":"response_item","payload":{"id":"message"}}`, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var record map[string]any
			if err := json.Unmarshal([]byte(tc.record), &record); err != nil {
				t.Fatal(err)
			}
			id, _, metadata := extractCodex(record)
			if id != tc.id || metadata.Project != tc.project {
				t.Fatalf("id=%q project=%q; want %q %q", id, metadata.Project, tc.id, tc.project)
			}
		})
	}
}

func TestCodexOldIndexProjectRecovery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "2026", "09", "29", "rollout.jsonl")
	contents := []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"canonical\",\"cwd\":\"/work/agent-deck\"}}\n{\"type\":\"visible_user_prompt\",\"session_id\":\"canonical\",\"payload\":{\"text\":\"needle preserved\"}}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := Scan(ctx, database.DB, home); err != nil {
		t.Fatal(err)
	}
	// Seed the exact v5 cache shape without rebuilding any real database.
	if _, err := database.DB.ExecContext(ctx, `UPDATE session_metadata SET session_id='stale-id',project=?,parser_version=5`, filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.ExecContext(ctx, `UPDATE session_documents SET session_id='stale-id'`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.ExecContext(ctx, `UPDATE session_sources SET parser_version=5`); err != nil {
		t.Fatal(err)
	}
	if err := Exclude(ctx, database.DB, "project", "/work/unrelated"); err != nil {
		t.Fatal(err)
	}
	exclusions := snapshotSessionRows(t, database.DB, `SELECT * FROM session_exclusions`)
	// A publication failure must leave the entire old source projection intact.
	before := captureSessionIndex(t, database.DB)
	if _, err := database.DB.ExecContext(ctx, `CREATE TRIGGER reject_reparse BEFORE INSERT ON session_metadata BEGIN SELECT RAISE(ABORT,'fixture publication failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, database.DB, home); err == nil {
		t.Fatal("old parser cache skipped instead of reparsed")
	}
	if after := captureSessionIndex(t, database.DB); !reflect.DeepEqual(before, after) {
		t.Fatal("failed recovery changed old projection")
	}
	if _, err := database.DB.ExecContext(ctx, `DROP TRIGGER reject_reparse`); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(ctx, database.DB, home)
	if err != nil || result.Sources != 1 || result.Skipped != 0 {
		t.Fatalf("reparse=%+v, %v", result, err)
	}
	items, err := List(ctx, database.DB)
	if err != nil || len(items) != 1 || items[0].SessionID != "canonical" || items[0].Project != "/work/agent-deck" {
		t.Fatalf("recovered=%#v, %v", items, err)
	}
	docs, err := Search(ctx, database.DB, "needle")
	if err != nil || len(docs) != 1 || docs[0].Text != "needle preserved" {
		t.Fatalf("documents=%#v, %v", docs, err)
	}
	if after := snapshotSessionRows(t, database.DB, `SELECT * FROM session_exclusions`); !reflect.DeepEqual(exclusions, after) {
		t.Fatal("exclusions changed")
	}
	if after, err := os.ReadFile(path); err != nil || !reflect.DeepEqual(contents, after) {
		t.Fatal("raw log changed")
	}
	if result, err := Scan(ctx, database.DB, home); err != nil || result.Skipped != 1 {
		t.Fatalf("repeated recovery=%+v, %v", result, err)
	}
}

func TestCodexLateCwdProjectFallback(t *testing.T) {
	for _, appendCwd := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-read", true: "append"}[appendCwd], func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			home := filepath.Join(root, "home")
			path := filepath.Join(home, ".codex", "sessions", "fallback.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			contents := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"canonical\",\"project\":\"/fallback\"}}\n"
			if appendCwd {
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := Scan(ctx, database.DB, home); err != nil {
					t.Fatal(err)
				}
			}
			contents += "{\"type\":\"turn_context\",\"payload\":{\"cwd\":\"/work/real\"}}\n{\"type\":\"visible_user_prompt\",\"payload\":{\"text\":\"needle late cwd\"}}\n"
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Scan(ctx, database.DB, home); err != nil {
				t.Fatal(err)
			}
			items, err := List(ctx, database.DB)
			if err != nil || len(items) != 1 || items[0].Project != "/work/real" {
				t.Fatalf("cwd fallback=%#v, %v", items, err)
			}
		})
	}
}

func TestCodexLateCwdExclusionParity(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, initiallyExcluded := range []bool{false, true} {
			t.Run(map[bool]string{false: "standalone", true: "shared"}[shared]+"/"+map[bool]string{false: "allowed-to-excluded", true: "excluded-to-allowed"}[initiallyExcluded], func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				home := filepath.Join(root, "home")
				path := filepath.Join(home, ".codex", "sessions", "late-excluded.jsonl")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				if err := Exclude(ctx, database.DB, "project", "/excluded"); err != nil {
					t.Fatal(err)
				}
				scan := func() error {
					options := ScanOptions{}
					if shared {
						sources, err := ingest.Discover(home)
						if err != nil {
							return err
						}
						coordinator := ingest.NewCoordinator(sources, ingest.Options{RequirePlans: true})
						defer coordinator.Close()
						for _, source := range sources {
							coordinator.Skip(source.Path, ingest.ConsumerUsage)
						}
						if err := coordinator.Seal(ingest.ConsumerUsage); err != nil {
							return err
						}
						if err := PlanForCoordinator(ctx, database.DB, home, sources, coordinator); err != nil {
							return err
						}
						coordinator.Start(ctx)
						options = ScanOptions{PreparedSources: sources, Coordinator: coordinator}
					}
					_, err := ScanWithOptions(ctx, database.DB, home, options)
					return err
				}
				initialProject, targetCwd, want := "/fallback", "/excluded", 0
				trigger := `CREATE TRIGGER reject_exclusion BEFORE DELETE ON session_metadata BEGIN SELECT RAISE(ABORT,'fixture exclusion failure'); END`
				if initiallyExcluded {
					initialProject, targetCwd, want = "/excluded", "/allowed", 1
					trigger = `CREATE TRIGGER reject_exclusion BEFORE INSERT ON session_metadata BEGIN SELECT RAISE(ABORT,'fixture replay failure'); END`
				}
				contents := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"canonical\",\"project\":\"" + initialProject + "\"}}\n{\"type\":\"visible_user_prompt\",\"payload\":{\"text\":\"needle indexed before cwd\"}}\n"
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				if err := scan(); err != nil {
					t.Fatal(err)
				}
				before := captureSessionIndex(t, database.DB)
				contextBefore := snapshotSessionRows(t, database.DB, `SELECT parser_context FROM session_sources`)
				contents += "{\"type\":\"turn_context\",\"payload\":{\"cwd\":\"" + targetCwd + "\"}}\n"
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := database.DB.ExecContext(ctx, trigger); err != nil {
					t.Fatal(err)
				}
				if err := scan(); err == nil {
					t.Fatal("deletion failure not propagated")
				}
				if after := captureSessionIndex(t, database.DB); !reflect.DeepEqual(before, after) {
					t.Fatal("failed exclusion changed old projection/cursor")
				}
				if after := snapshotSessionRows(t, database.DB, `SELECT parser_context FROM session_sources`); !reflect.DeepEqual(contextBefore, after) {
					t.Fatal("failed exclusion advanced parser context")
				}
				if _, err := database.DB.ExecContext(ctx, `DROP TRIGGER reject_exclusion`); err != nil {
					t.Fatal(err)
				}
				if err := scan(); err != nil {
					t.Fatal(err)
				}
				fresh, err := store.OpenSessions(ctx, filepath.Join(root, "fresh"))
				if err != nil {
					t.Fatal(err)
				}
				defer fresh.Close()
				if err := Exclude(ctx, fresh.DB, "project", "/excluded"); err != nil {
					t.Fatal(err)
				}
				if _, err := Scan(ctx, fresh.DB, home); err != nil {
					t.Fatal(err)
				}
				incremental, err := Search(ctx, database.DB, "needle")
				if err != nil {
					t.Fatal(err)
				}
				full, err := Search(ctx, fresh.DB, "needle")
				if err != nil || len(incremental) != want || len(full) != want {
					t.Fatalf("excluded projection remains: %#v / %#v, %v", incremental, full, err)
				}
				items, err := List(ctx, database.DB)
				if err != nil || len(items) != want {
					t.Fatalf("excluded metadata remains: %#v, %v", items, err)
				}
				if err := scan(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCodexMovedExcludedSourceParity(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, directoryFallback := range []bool{false, true} {
			t.Run(map[bool]string{false: "standalone", true: "shared"}[shared]+"/"+map[bool]string{false: "path", true: "directory-project"}[directoryFallback], func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				home := filepath.Join(root, "home")
				old := filepath.Join(home, ".codex", "sessions", "old", "session.jsonl")
				path := filepath.Join(home, ".codex", "sessions", "new", "session.jsonl")
				for _, dir := range []string{filepath.Dir(old), filepath.Dir(path)} {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				kind, value, cwd := "path", old, `,"cwd":"/work/project"`
				if directoryFallback {
					kind, value, cwd = "project", filepath.Dir(old), ""
				}
				if err := Exclude(ctx, database.DB, kind, value); err != nil {
					t.Fatal(err)
				}
				contents := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"canonical\"" + cwd + "}}\n{\"type\":\"visible_user_prompt\",\"payload\":{\"text\":\"needle initial\"}}\n"
				if err := os.WriteFile(old, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				scan := func() error {
					options := ScanOptions{}
					if shared {
						sources, err := ingest.Discover(home)
						if err != nil {
							return err
						}
						coordinator := ingest.NewCoordinator(sources, ingest.Options{RequirePlans: true})
						defer coordinator.Close()
						for _, source := range sources {
							coordinator.Skip(source.Path, ingest.ConsumerUsage)
						}
						if err := coordinator.Seal(ingest.ConsumerUsage); err != nil {
							return err
						}
						if err := PlanForCoordinator(ctx, database.DB, home, sources, coordinator); err != nil {
							return err
						}
						coordinator.Start(ctx)
						options = ScanOptions{PreparedSources: sources, Coordinator: coordinator}
					}
					_, err := ScanWithOptions(ctx, database.DB, home, options)
					return err
				}
				if err := scan(); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(old, path); err != nil {
					t.Fatal(err)
				}
				contents += "{\"type\":\"visible_assistant_final\",\"payload\":{\"text\":\"needle appended\"}}\n"
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				if err := scan(); err != nil {
					t.Fatal(err)
				}
				fresh, err := store.OpenSessions(ctx, filepath.Join(root, "fresh"))
				if err != nil {
					t.Fatal(err)
				}
				defer fresh.Close()
				if err := Exclude(ctx, fresh.DB, kind, value); err != nil {
					t.Fatal(err)
				}
				if _, err := Scan(ctx, fresh.DB, home); err != nil {
					t.Fatal(err)
				}
				incremental, err := Search(ctx, database.DB, "needle")
				if err != nil {
					t.Fatal(err)
				}
				full, err := Search(ctx, fresh.DB, "needle")
				if err != nil || len(incremental) != 2 || !reflect.DeepEqual(incremental, full) {
					t.Fatalf("moved source parity: %#v / %#v, %v", incremental, full, err)
				}
				assertSessionSourceOwnership(t, database.DB, path, 1, 1, 2)
			})
		}
	}
}
