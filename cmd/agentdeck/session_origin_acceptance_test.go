package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem"
	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem/claudememtest"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

// The integrated acceptance cohort. Every source below is an isolated fixture
// in the format its producer writes; nothing reads the user's real state.
const (
	cohortWork     = "11111111-1111-4111-8111-111111111111" // claude, the user's task
	cohortObserver = "22222222-2222-4222-8222-222222222222" // claude, claude-mem observer
	cohortUnknown  = "33333333-3333-4333-8333-333333333333" // claude, no evidence
	cohortCodex    = "44444444-4444-4444-8444-444444444444" // codex rollout, two copies
	cohortOTel     = "55555555-5555-4555-8555-555555555555" // codex, OTel background only
	cohortDay      = "2026-10-29"
	cohortAsOf     = "2026-10-29T23:00:00Z"
)

var cohortReceivedAt = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type cohort struct {
	home, state string
	claudeMem   *claudememtest.Fixture
}

func claudeTranscript(id, model, prompt string, input, output int) string {
	return strings.Join([]string{
		`{"type":"user","sessionId":"` + id + `","timestamp":"` + cohortDay + `T08:00:00Z","cwd":"/project","message":{"role":"user","content":"` + prompt + `"}}`,
		fmt.Sprintf(`{"type":"assistant","sessionId":"%s","timestamp":"%sT08:00:04Z","cwd":"/project","message":{"role":"assistant","id":"m-%s","model":"%s","usage":{"input_tokens":%d,"output_tokens":%d},"content":[{"type":"tool_use","id":"t-%s","name":"Edit","input":{"file_path":"/project/a.go"}}]}}`, id, cohortDay, id, model, input, output, id),
		`{"type":"user","sessionId":"` + id + `","timestamp":"` + cohortDay + `T08:00:05Z","cwd":"/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t-` + id + `","content":"ok"}]}}`,
	}, "\n") + "\n"
}

func codexRollout(id string) string {
	return strings.Join([]string{
		`{"timestamp":"` + cohortDay + `T09:00:00Z","type":"session_meta","payload":{"id":"` + id + `","session_id":"` + id + `","timestamp":"` + cohortDay + `T09:00:00Z","cwd":"/project"}}`,
		`{"timestamp":"` + cohortDay + `T09:00:00Z","type":"turn_context","payload":{"turn_id":"turn-1","model":"gpt-5.4"}}`,
		`{"timestamp":"` + cohortDay + `T09:00:00Z","type":"event_msg","payload":{"type":"user_message","message":"implement the cache"}}`,
		`{"timestamp":"` + cohortDay + `T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":70,"cached_input_tokens":0,"output_tokens":7}}}}`,
	}, "\n") + "\n"
}

func writeCohortFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func cohortOTelExport(conversation string) string {
	attr := func(key, value string) string {
		return fmt.Sprintf(`{"key":%q,"value":{"stringValue":%q}}`, key, value)
	}
	count := func(key, value string) string { return fmt.Sprintf(`{"key":%q,"value":{"intValue":%q}}`, key, value) }
	resource := "[" + attr("service.name", "codex_exec") + "," + attr("service.version", "0.165.0") + "]"
	record := "[" + strings.Join([]string{
		attr("event.name", "codex.sse_event"), attr("event.kind", "response.completed"), attr("model", "gpt-5.4"),
		attr("originator", "codex_exec"), attr("conversation.id", conversation),
		count("input_token_count", "1000"), count("output_token_count", "10"), count("cached_token_count", "0"),
	}, ",") + "]"
	return fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":%s},"scopeLogs":[{"logRecords":[{"timeUnixNano":"1793272990000000000","body":{"stringValue":"x"},"attributes":%s}]}]}]}`, resource, record)
}

func runOriginCLIInput(t *testing.T, state, input string, args ...string) originCLIResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := execute(append([]string{"--state-dir", state}, args...), bytes.NewBufferString(input), &stdout, &stderr)
	return originCLIResult{exit: exit, stdout: stdout.String(), stderr: stderr.String()}
}

func newCohort(t *testing.T) cohort {
	t.Helper()
	useUTCDisplayClock(t)
	home := claudememtest.Home(t, claudememtest.ReviewedInstall())
	previous := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = previous })
	state := filepath.Join(t.TempDir(), "state")
	waitForDetachedScanCleanup(t, state)

	projects := filepath.Join(home, ".claude", "projects", "-project")
	writeCohortFile(t, filepath.Join(projects, cohortWork+".jsonl"), claudeTranscript(cohortWork, "claude-sonnet-4-5", "change the cache", 100, 10))
	writeCohortFile(t, filepath.Join(projects, cohortObserver+".jsonl"), claudeTranscript(cohortObserver, "claude-haiku-4-5", "observe the session", 300, 30))
	writeCohortFile(t, filepath.Join(projects, cohortUnknown+".jsonl"), claudeTranscript(cohortUnknown, "model-without-a-price", "something else", 50, 5))
	writeCohortFile(t, filepath.Join(home, ".codex", "sessions", "2026", "10", "29", "rollout-2026-10-29T09-00-00-"+cohortCodex+".jsonl"), codexRollout(cohortCodex))
	writeCohortFile(t, filepath.Join(home, ".codex", "archived_sessions", "rollout-2026-10-29T09-00-00-"+cohortCodex+".jsonl"), codexRollout(cohortCodex))

	fixture := claudememtest.New(t, claudemem.SupportedSchemaVersion, []claudememtest.Mapping{{RowID: 7, ContentID: cohortWork, MemoryID: cohortObserver}})

	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	records, err := telemetry.Decode([]byte(cohortOTelExport(cohortOTel)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := telemetry.Import(ctx, core, records, cohortReceivedAt); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	return cohort{home: home, state: state, claudeMem: fixture}
}

// ready configures the reviewed claude-mem source and runs one real scan.
func (c cohort) ready(t *testing.T) {
	t.Helper()
	c.run(t, "--format", "json", "session", "origin-source", "set", "claude-mem", "--data-dir", c.claudeMem.Dir)
	c.run(t, "--format", "json", "scan")
}

func (c cohort) run(t *testing.T, args ...string) map[string]any {
	t.Helper()
	return mustOriginOK(t, runOriginCLI(t, c.state, args...))
}

// cohortOracle is the expected effective class of every canonical session,
// written by hand from the cohort definition, never read from the product.
type cohortOracle map[string]string

func baseOracle() cohortOracle {
	return cohortOracle{
		"claude/" + cohortWork:     "unknown",
		"claude/" + cohortObserver: "auxiliary",
		"claude/" + cohortUnknown:  "unknown",
		"codex/" + cohortCodex:     "unknown",
		"codex/" + cohortOTel:      "auxiliary",
	}
}

func (o cohortOracle) with(key, class string) cohortOracle {
	next := cohortOracle{}
	for k, v := range o {
		next[k] = v
	}
	next[key] = class
	return next
}

// selected applies the scope rule: work shows work and unknown, auxiliary
// shows auxiliary only, all shows everything.
func (o cohortOracle) selected(scope, client string) (keys []string, unknown int) {
	keys = []string{}
	for key, class := range o {
		if client != "all" && !strings.HasPrefix(key, client+"/") {
			continue
		}
		if scope == "work" && class == "auxiliary" || scope == "auxiliary" && class != "auxiliary" {
			continue
		}
		keys = append(keys, key)
		if class == "unknown" {
			unknown++
		}
	}
	sort.Strings(keys)
	return keys, unknown
}

// assertSurfaces compares the CLI list, the menu-bar scope view and the
// widget batch (the same view pinned to one generation) with the oracle for
// every scope and client.
func (c cohort) assertSurfaces(t *testing.T, step string, oracle cohortOracle) {
	t.Helper()
	generation := ""
	for _, scope := range []string{"work", "auxiliary", "all"} {
		for _, client := range []string{"all", "claude", "codex"} {
			want, unknown := oracle.selected(scope, client)
			where := fmt.Sprintf("%s: scope %s client %s", step, scope, client)

			args := []string{"--format", "json", "session", "list", "--session-scope", scope, "--all"}
			if client != "all" {
				args = append(args, "--client", client)
			}
			list := c.run(t, args...)
			page := list["data"].(map[string]any)
			if got := sessionKeys(t, page["sessions"]); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: CLI list %v, oracle %v", where, got, want)
			}
			if total := page["pagination"].(map[string]any)["sessions"].(map[string]any)["total"]; total != float64(len(want)) {
				t.Fatalf("%s: CLI total %v, oracle %d", where, total, len(want))
			}
			if got := list["session_selection"].(map[string]any)["unknown_count"]; got != float64(unknown) {
				t.Fatalf("%s: CLI unknown_count %v, oracle %d", where, got, unknown)
			}
			for _, item := range page["sessions"].([]any) {
				value := item.(map[string]any)
				key := value["client"].(string) + "/" + value["session_id"].(string)
				if got := value["origin"].(map[string]any)["effective"]; got != oracle[key] {
					t.Fatalf("%s: CLI %s effective %v, oracle %s", where, key, got, oracle[key])
				}
			}

			for _, period := range []string{"today", "7d", "30d"} {
				view := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", scope, "--client", client,
					"--period", period, "--recent-limit", "20", "--as-of", cohortAsOf)["data"].(map[string]any)
				if view["available"] != true {
					t.Fatalf("%s %s: desktop view unavailable: %#v", where, period, view["reason"])
				}
				if got := view["counts"].(map[string]any)["sessions"]; got != float64(len(want)) {
					t.Fatalf("%s %s: desktop sessions %v, oracle %d", where, period, got, len(want))
				}
				if got := view["origin_coverage"].(map[string]any)["unknown_count"]; got != float64(unknown) {
					t.Fatalf("%s %s: desktop unknown_count %v, oracle %d", where, period, got, unknown)
				}
				if got := sessionKeys(t, view["recent"]); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s %s: desktop recent %v, oracle %v", where, period, got, want)
				}
				id := view["generation_id"].(string)
				if generation == "" {
					generation = id
				}
				if id != generation {
					t.Fatalf("%s %s: generation %s differs from %s within one committed state", where, period, id, generation)
				}
			}
		}
	}
	// A widget batch pins every selection to the generation it started from.
	pinned := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", "work", "--client", "all", "--period", "today",
		"--as-of", cohortAsOf, "--generation-id", generation)["data"].(map[string]any)
	if pinned["available"] != true || pinned["generation_id"] != generation {
		t.Fatalf("%s: pinned widget read = %#v", step, pinned)
	}
}

// usageIdentity is every usage event row and the totals of every scope. The
// totals never depend on the session scope.
func (c cohort) usageIdentity(t *testing.T, step string) (events string, totals map[string]any) {
	t.Helper()
	core, err := store.OpenReadOnly(context.Background(), c.state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	events = dumpRows(t, core.DB, `SELECT event_key, client, session_id, event_id, event_at, model, input_tokens, cached_input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cache_write_tokens, source_path, source_offset FROM usage_events ORDER BY event_key`)
	for _, scope := range []string{"work", "auxiliary", "all"} {
		got := c.run(t, "--format", "json", "usage", "stats", "--no-scan", "--from", cohortDay, "--to", cohortDay, "--session-scope", scope)["data"].(map[string]any)["totals"].(map[string]any)
		if totals == nil {
			totals = got
		} else if !reflect.DeepEqual(got, totals) {
			t.Fatalf("%s: usage totals depend on scope %s:\n%v\n%v", step, scope, got, totals)
		}
	}
	return events, totals
}

func dumpRows(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	var out strings.Builder
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&out, values...)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// sourceDigest fingerprints every source the product reads: both clients'
// logs and the claude-mem database with its sidecars.
func (c cohort) sourceDigest(t *testing.T) map[string]string {
	t.Helper()
	digests := map[string]string{}
	for _, root := range []string{filepath.Join(c.home, ".claude", "projects"), filepath.Join(c.home, ".codex"), c.claudeMem.Dir} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digests[path] = fmt.Sprintf("%x", sha256.Sum256(content))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return digests
}

// AC1, AC3, AC5, AC6: one cohort in real source formats, classified by a real
// scan, is reported identically by the CLI, the menu-bar view, the widget
// batch and the Settings reads.
func TestIntegratedAcceptanceEverySurfaceAgreesOnTheCohort(t *testing.T) {
	c := newCohort(t)
	c.ready(t)
	oracle := baseOracle()
	c.assertSurfaces(t, "after scan", oracle)

	// AC1: each maintenance source is identified by its own reviewed rule.
	all := c.run(t, "--format", "json", "session", "list", "--session-scope", "all")
	rules := map[string]string{}
	for _, item := range all["data"].([]any) {
		value := item.(map[string]any)
		origin := value["origin"].(map[string]any)
		rules[value["client"].(string)+"/"+value["session_id"].(string)] = origin["reason_code"].(string) + "/" + origin["rule_version"].(string)
	}
	wantRules := map[string]string{
		"claude/" + cohortWork:     "origin_metadata_missing/origin-reducer-v1",
		"claude/" + cohortObserver: "maintenance_origin/claude-mem-sdk-map-v1",
		"claude/" + cohortUnknown:  "origin_metadata_missing/origin-reducer-v1",
		"codex/" + cohortCodex:     "origin_metadata_missing/origin-reducer-v1",
		"codex/" + cohortOTel:      "maintenance_origin/codex-otel-background-v1",
	}
	if !reflect.DeepEqual(rules, wantRules) {
		t.Fatalf("rules = %v\nwant    %v", rules, wantRules)
	}

	// AC6: JSON without a scope keeps the complete legacy set and the bare
	// array; text defaults to the work view.
	legacy := c.run(t, "--format", "json", "session", "list")
	if keys, _ := oracle.selected("all", "all"); !reflect.DeepEqual(sessionKeys(t, legacy["data"]), keys) {
		t.Fatalf("legacy JSON = %v", sessionKeys(t, legacy["data"]))
	}
	if selection := legacy["session_selection"].(map[string]any); selection["effective_scope"] != "all" || selection["source_default"] != "json_compat" {
		t.Fatalf("legacy selection = %#v", selection)
	}
	text := runOriginCLI(t, c.state, "--no-color", "session", "list")
	if text.exit != 0 || !strings.Contains(text.stdout, "Scope: work · 3 sessions · 3 unknown") || strings.Contains(text.stdout, cohortObserver) || strings.Contains(text.stdout, cohortOTel) {
		t.Fatalf("text default = %#v", text)
	}

	// AC3, AC4, AC8: the OTel background session and the observer leave the
	// work view's session count, and their tokens stay in all usage. The
	// unpriced model keeps the total cost unknown instead of adding zero.
	_, totals := c.usageIdentity(t, "after scan")
	if totals["events"] != float64(5) || totals["sessions"] != float64(5) || totals["tokens"] != float64(1582) || totals["input_tokens"] != float64(1520) || totals["output_tokens"] != float64(62) {
		t.Fatalf("usage totals = %#v", totals)
	}
	if totals["catalog_base_cost"] != nil || totals["provider_cost"] != nil || totals["known_catalog_base_cost"] == nil {
		t.Fatalf("an unpriced event must leave the total unknown, not zero: %#v", totals)
	}
	work := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", "work", "--client", "codex", "--period", "today", "--as-of", cohortAsOf)["data"].(map[string]any)
	if work["counts"].(map[string]any)["sessions"] != float64(1) {
		t.Fatalf("codex work view still counts the OTel background session: %#v", work["counts"])
	}

	// AC5 (Settings): the default, the override list and both sources.
	if scope := c.run(t, "--format", "json", "session", "scope", "get")["data"].(map[string]any); scope["confirmed"] != true || scope["value"] != "work" || scope["revision"] != float64(0) {
		t.Fatalf("first default = %#v", scope)
	}
	if list := c.run(t, "--format", "json", "session", "classify", "list")["data"].(map[string]any); list["total"] != float64(0) {
		t.Fatalf("override list = %#v", list)
	}
	sources := map[string]map[string]any{}
	for _, item := range c.run(t, "--format", "json", "session", "origin-source", "status")["data"].(map[string]any)["sources"].([]any) {
		sources[item.(map[string]any)["source"].(string)] = item.(map[string]any)
	}
	if mem := sources["claude-mem"]; mem["available"] != true || mem["enabled"] != true || mem["supported_version"] != "15.2.1/sdk-schema-68" {
		t.Fatalf("claude-mem status = %#v", mem)
	}
	if otel := sources["codex-otel-background"]; otel["available"] != true || otel["background_sessions"] != float64(1) {
		t.Fatalf("otel status = %#v", otel)
	}

	// AC6 (widget privacy): the aggregated view carries no source path, home
	// directory or prompt text.
	view := runOriginCLI(t, c.state, "--format", "json", "desktop", "session-scope", "--scope", "all", "--client", "all", "--period", "today", "--recent-limit", "20", "--as-of", cohortAsOf)
	for _, leak := range []string{c.home, c.claudeMem.Dir, "source_path", "change the cache", "observe the session", claudememtest.Secret} {
		if strings.Contains(view.stdout, leak) {
			t.Fatalf("desktop view leaks %q", leak)
		}
	}
}

// AC2, AC4, AC5, AC7, AC8: default, override, clear, rescan, source disable,
// purge-index and backup/restore never change an event, a token or a cost,
// never rewrite a source and never trigger a re-import; every surface follows
// each step.
func TestIntegratedAcceptanceControlChainKeepsEveryEventAndCost(t *testing.T) {
	c := newCohort(t)
	c.ready(t)
	oracle := baseOracle()
	sources := c.sourceDigest(t)
	events, totals := c.usageIdentity(t, "after scan")
	observer := "claude/" + cohortObserver
	check := func(step string, expected cohortOracle) {
		t.Helper()
		c.assertSurfaces(t, step, expected)
		gotEvents, gotTotals := c.usageIdentity(t, step)
		if gotEvents != events {
			t.Fatalf("%s: usage events changed:\n%s\nwas:\n%s", step, gotEvents, events)
		}
		if !reflect.DeepEqual(gotTotals, totals) {
			t.Fatalf("%s: usage totals changed:\n%v\nwas:\n%v", step, gotTotals, totals)
		}
		if got := c.sourceDigest(t); !reflect.DeepEqual(got, sources) {
			t.Fatalf("%s: a source file changed", step)
		}
	}
	noImport := func(step string) {
		t.Helper()
		data := c.run(t, "--format", "json", "scan")["data"].(map[string]any)
		changes := data["usage"].(map[string]any)["changes"].(map[string]any)
		scan := data["session"].(map[string]any)["scan"].(map[string]any)
		for _, field := range []string{"imported", "updated", "replaced", "removed", "source_resets"} {
			if changes[field] != float64(0) {
				t.Fatalf("%s: rescan %s = %v, want no re-import: %#v", step, field, changes[field], changes)
			}
		}
		if scan["sources"] != float64(0) || scan["documents"] != float64(0) || scan["removed"] != float64(0) {
			t.Fatalf("%s: rescan re-read session sources: %#v", step, scan)
		}
	}

	// Persistent default: the desktop snapshot follows it, CLI text does not.
	if saved := c.run(t, "--format", "json", "session", "scope", "set", "auxiliary")["data"].(map[string]any); saved["outcome"] != "saved" || saved["revision"] != float64(1) {
		t.Fatalf("scope set = %#v", saved)
	}
	snapshot := c.run(t, "--format", "json", "desktop", "snapshot")["data"].(map[string]any)["session_scope_view"].(map[string]any)
	if snapshot["default"].(map[string]any)["value"] != "auxiliary" || snapshot["selection"].(map[string]any)["scope"] != "auxiliary" {
		t.Fatalf("snapshot did not follow the saved default: default=%#v selection=%#v", snapshot["default"], snapshot["selection"])
	}
	noImport("after scope set")
	check("after scope set", oracle)

	// Override an automatic auxiliary session to work, and a second session
	// to auxiliary.
	c.run(t, "--format", "json", "session", "classify", cohortObserver, "--client", "claude", "--set", "work")
	c.run(t, "--format", "json", "session", "classify", cohortCodex, "--client", "codex", "--set", "auxiliary")
	overridden := oracle.with(observer, "work").with("codex/"+cohortCodex, "auxiliary")
	check("after overrides", overridden)
	noImport("after overrides")
	check("after overrides and rescan", overridden)

	// Clear returns the session to its automatic class.
	c.run(t, "--format", "json", "session", "classify", cohortObserver, "--client", "claude", "--clear")
	cleared := oracle.with("codex/"+cohortCodex, "auxiliary")
	check("after clear", cleared)

	// Disabling the source invalidates its evidence only.
	c.run(t, "--format", "json", "session", "origin-source", "disable", "claude-mem")
	noImport("after disable")
	check("after disable", cleared.with(observer, "unknown"))
	c.run(t, "--format", "json", "session", "origin-source", "set", "claude-mem", "--data-dir", c.claudeMem.Dir)
	noImport("after re-enable")
	check("after re-enable", cleared)

	// purge-index drops only the rebuildable index; the rebuild is equal.
	c.run(t, "--format", "json", "session", "purge-index")
	c.run(t, "--format", "json", "scan")
	check("after purge-index and rebuild", cleared)

	// backup/restore into an empty root: the user's choices travel, automatic
	// evidence is rebuilt, and the override re-binds only after this machine's
	// scan confirms the same session birth.
	archive := filepath.Join(t.TempDir(), "cohort.agentdeck-backup")
	if result := runOriginCLIInput(t, c.state, "passphrase\n", "--format", "json", "backup", "create", archive); result.exit != 0 {
		t.Fatalf("backup create = %#v", result)
	}
	restored := cohort{home: c.home, state: filepath.Join(t.TempDir(), "restored"), claudeMem: c.claudeMem}
	waitForDetachedScanCleanup(t, restored.state)
	if result := runOriginCLIInput(t, restored.state, "passphrase\n", "--format", "json", "backup", "restore", archive); result.exit != 0 {
		t.Fatalf("backup restore = %#v", result)
	}
	if scope := restored.run(t, "--format", "json", "session", "scope", "get")["data"].(map[string]any); scope["value"] != "auxiliary" {
		t.Fatalf("restored default = %#v", scope)
	}
	waiting := restored.run(t, "--format", "json", "session", "classify", "list")["data"].(map[string]any)
	if waiting["total"] != float64(1) || waiting["items"].([]any)[0].(map[string]any)["binding_status"] != "unmatched" {
		t.Fatalf("restored overrides must wait unmatched: %#v", waiting)
	}
	restored.run(t, "--format", "json", "session", "origin-source", "set", "claude-mem", "--data-dir", c.claudeMem.Dir)
	restored.run(t, "--format", "json", "scan")
	restored.assertSurfaces(t, "after restore and scan", cleared)
	restoredEvents, restoredTotals := restored.usageIdentity(t, "after restore and scan")
	if restoredEvents != events || !reflect.DeepEqual(restoredTotals, totals) {
		t.Fatalf("restore changed usage:\n%s\n%v\nwas:\n%s\n%v", restoredEvents, restoredTotals, events, totals)
	}
	check("source root after restore", cleared)
}

// AC1, AC2, AC4: an appended record and a late rollout reach, incrementally,
// the classification and usage a full rebuild of the same sources produces;
// the late rollout revokes the OTel background mark without double counting.
func TestIntegratedAcceptanceIncrementalEqualsFullRebuild(t *testing.T) {
	c := newCohort(t)
	c.ready(t)
	c.assertSurfaces(t, "before the late rollout", baseOracle())

	appended := fmt.Sprintf(`{"type":"assistant","sessionId":"%s","timestamp":"%sT08:10:00Z","cwd":"/project","message":{"role":"assistant","id":"m2-%s","model":"claude-sonnet-4-5","usage":{"input_tokens":20,"output_tokens":2},"content":[{"type":"text","text":"done"}]}}`+"\n", cohortWork, cohortDay, cohortWork)
	work := filepath.Join(c.home, ".claude", "projects", "-project", cohortWork+".jsonl")
	file, err := os.OpenFile(work, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(appended); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	writeCohortFile(t, filepath.Join(c.home, ".codex", "sessions", "2026", "10", "29", "rollout-2026-10-29T11-23-00-"+cohortOTel+".jsonl"), codexRollout(cohortOTel))
	c.run(t, "--format", "json", "scan")

	revoked := baseOracle().with("codex/"+cohortOTel, "unknown")
	c.assertSurfaces(t, "incremental", revoked)
	events, totals := c.usageIdentity(t, "incremental")
	// The session's rollout is now the usage source: its single turn replaces
	// the OTel record instead of adding to it (1582 - 1010 + 77 + 22).
	if totals["events"] != float64(6) || totals["sessions"] != float64(5) || totals["tokens"] != float64(671) {
		t.Fatalf("late rollout double counted or lost usage: %#v", totals)
	}

	full := cohort{home: c.home, state: filepath.Join(t.TempDir(), "full"), claudeMem: c.claudeMem}
	waitForDetachedScanCleanup(t, full.state)
	core, err := store.Open(context.Background(), full.state)
	if err != nil {
		t.Fatal(err)
	}
	records, err := telemetry.Decode([]byte(cohortOTelExport(cohortOTel)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := telemetry.Import(context.Background(), core, records, cohortReceivedAt); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	full.ready(t)
	full.assertSurfaces(t, "full rebuild", revoked)
	fullEvents, fullTotals := full.usageIdentity(t, "full rebuild")
	if fullEvents != events || !reflect.DeepEqual(fullTotals, totals) {
		t.Fatalf("incremental and full rebuild differ:\n%s\n%v\nfull:\n%s\n%v", events, totals, fullEvents, fullTotals)
	}
}

// AC7 (joint failure): when the maintenance source cannot be read, its
// candidates become unknown with a stated reason on every surface, nothing is
// reported as complete or as auxiliary by stale evidence, the other source
// and all usage are untouched, and the next readable scan recovers.
func TestIntegratedAcceptanceUnreadableSourceDegradesAndRecovers(t *testing.T) {
	c := newCohort(t)
	c.ready(t)
	oracle := baseOracle()
	events, totals := c.usageIdentity(t, "after scan")

	if err := os.Chmod(c.claudeMem.Dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(c.claudeMem.Dir, 0o700) })
	c.run(t, "--format", "json", "scan")
	degraded := oracle.with("claude/"+cohortObserver, "unknown")
	c.assertSurfaces(t, "source unreadable", degraded)
	view := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", "work", "--client", "claude", "--period", "today", "--recent-limit", "20", "--as-of", cohortAsOf)["data"].(map[string]any)
	coverage := view["origin_coverage"].(map[string]any)
	if coverage["complete"] != false || !reflect.DeepEqual(coverage["reasons"], []any{"origin_source_unavailable"}) {
		t.Fatalf("unreadable source reported as complete coverage: %#v", coverage)
	}
	for _, item := range view["recent"].([]any) {
		if origin := item.(map[string]any)["origin"].(map[string]any); origin["reason_code"] != "origin_source_unavailable" {
			t.Fatalf("claude candidate lacks the unavailable reason: %#v", item)
		}
	}
	otel := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", "auxiliary", "--client", "codex", "--period", "today", "--as-of", cohortAsOf)["data"].(map[string]any)
	if otel["origin_coverage"].(map[string]any)["complete"] != true || otel["counts"].(map[string]any)["sessions"] != float64(1) {
		t.Fatalf("the unaffected source lost its classification: %#v", otel)
	}
	c.assertCLICoverage(t, "source unreadable", []any{"origin_source_unavailable"})
	for _, item := range c.run(t, "--format", "json", "session", "origin-source", "status")["data"].(map[string]any)["sources"].([]any) {
		if source := item.(map[string]any); source["source"] == "claude-mem" && (source["available"] != false || source["reason_code"] != "source_unavailable") {
			t.Fatalf("claude-mem status = %#v", source)
		}
	}
	if gotEvents, gotTotals := c.usageIdentity(t, "source unreadable"); gotEvents != events || !reflect.DeepEqual(gotTotals, totals) {
		t.Fatal("an unreadable origin source changed usage")
	}

	if err := os.Chmod(c.claudeMem.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	c.run(t, "--format", "json", "scan")
	c.assertSurfaces(t, "source readable again", oracle)
	c.assertCLICoverage(t, "source readable again", []any{})
	recovered := c.run(t, "--format", "json", "desktop", "session-scope", "--scope", "work", "--client", "claude", "--period", "today", "--as-of", cohortAsOf)["data"].(map[string]any)
	if recovered["origin_coverage"].(map[string]any)["complete"] != true {
		t.Fatalf("coverage did not recover: %#v", recovered["origin_coverage"])
	}
}

// assertCLICoverage checks the envelope of scoped CLI reads, which is all
// automation sees: while the claude source's classification input is missing,
// every read over claude sessions is partial and names the reason, in every
// scope — auxiliary included, whose own set holds none of the affected
// sessions — and the text form says so. Reads over codex alone are complete.
func (c cohort) assertCLICoverage(t *testing.T, step string, reasons []any) {
	t.Helper()
	gap := len(reasons) > 0
	check := func(name string, envelope map[string]any, wantGap bool) {
		t.Helper()
		want := []any{}
		if wantGap {
			want = reasons
		}
		if envelope["partial"] != wantGap || !reflect.DeepEqual(envelope["warnings"], want) {
			t.Fatalf("%s: %s partial=%v warnings=%#v, want partial=%t warnings=%#v", step, name, envelope["partial"], envelope["warnings"], wantGap, want)
		}
		if selection := envelope["session_selection"].(map[string]any); selection["availability"] != "available" {
			t.Fatalf("%s: %s selection = %#v, want it to stay available", step, name, selection)
		}
	}
	for _, scope := range []string{"all", "work", "auxiliary"} {
		check("list "+scope, c.run(t, "--format", "json", "session", "list", "--session-scope", scope), gap)
		check("list claude "+scope, c.run(t, "--format", "json", "session", "list", "--session-scope", scope, "--client", "claude", "--all"), gap)
		check("list codex "+scope, c.run(t, "--format", "json", "session", "list", "--session-scope", scope, "--client", "codex"), false)
		check("search "+scope, c.run(t, "--format", "json", "session", "search", "cache", "--session-scope", scope), gap)
	}
	check("list without a scope", c.run(t, "--format", "json", "session", "list"), gap)
	text := runOriginCLI(t, c.state, "session", "list", "--session-scope", "auxiliary")
	const banner = "PARTIAL: classification coverage is incomplete (origin_source_unavailable)"
	if text.exit != 0 || strings.Contains(text.stdout, banner) != gap {
		t.Fatalf("%s: text list exit=%d, partial banner present=%t, want %t:\n%s", step, text.exit, strings.Contains(text.stdout, banner), gap, text.stdout)
	}
	c.assertBrowserCoverage(t, step, gap)
}

// assertBrowserCoverage loads each scope the way session list --interactive
// does and checks its scope line states the same partial fact as the text and
// JSON reads above.
func (c cohort) assertBrowserCoverage(t *testing.T, step string, gap bool) {
	t.Helper()
	ctx := context.Background()
	index, err := store.OpenSessionsReadOnly(ctx, c.state)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	reader := openSessionSelectionReader(ctx, c.state, index)
	defer reader.Close()
	const partial = "PARTIAL: classification coverage incomplete (origin_source_unavailable)"
	for _, scope := range []string{"all", "work", "auxiliary"} {
		snapshot, err := reader.selectSessions(ctx, sessionorigin.Selection{Scope: scope}, nil)
		if err != nil {
			t.Fatalf("%s: browser %s load: %v", step, scope, err)
		}
		var frame bytes.Buffer
		if err := renderSessionBrowser(&frame, 120, 24, newScopedSessionBrowserState(newSessionBrowserPage(scope, snapshot))); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(frame.String(), partial) != gap {
			t.Fatalf("%s: browser %s partial present=%t, want %t:\n%s", step, scope, strings.Contains(frame.String(), partial), gap, frame.String())
		}
	}
}

// AC8 (write amplification): reading any scope writes nothing; one override
// advances the serving revision once and invalidates exactly that session's
// partition; a default change advances it once and invalidates nothing; an
// unchanged scan rewrites no classification, and the enabled maintenance
// source costs exactly one run summary per pass.
func TestIntegratedAcceptanceClassificationWritesAreBounded(t *testing.T) {
	c := newCohort(t)
	c.ready(t)
	type state struct {
		usage, session float64
		invalidations  string
		derived        string
	}
	read := func() state {
		t.Helper()
		vector := c.run(t, "--format", "json", "session", "list", "--session-scope", "all")["session_selection"].(map[string]any)["read_vector"].(map[string]any)
		core, err := store.OpenReadOnly(context.Background(), c.state)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		return state{
			usage: vector["usage_revision"].(float64), session: vector["session_revision"].(float64),
			invalidations: dumpRows(t, core.DB, `SELECT kind, client, key FROM usage_derived_invalidations ORDER BY 1, 2, 3`),
			derived: dumpRows(t, core.DB, `SELECT * FROM session_origins ORDER BY client, session_id`) +
				dumpRows(t, core.DB, `SELECT * FROM origin_evidence ORDER BY client, session_id, adapter`),
		}
	}
	scan := func() { t.Helper(); c.run(t, "--format", "json", "scan") }

	scan()
	settled := read()
	for _, scope := range []string{"work", "auxiliary", "all"} {
		c.run(t, "--format", "json", "session", "list", "--session-scope", scope)
		c.run(t, "--format", "json", "usage", "stats", "--no-scan", "--from", cohortDay, "--to", cohortDay, "--session-scope", scope)
		c.run(t, "--format", "json", "desktop", "session-scope", "--scope", scope, "--client", "all", "--period", "today", "--as-of", cohortAsOf)
	}
	if got := read(); got != settled {
		t.Fatalf("switching scopes wrote state:\n%+v\nwas\n%+v", got, settled)
	}

	c.run(t, "--format", "json", "session", "classify", cohortCodex, "--client", "codex", "--set", "auxiliary")
	overridden := read()
	if overridden.usage != settled.usage+1 || overridden.session != settled.session || overridden.derived != settled.derived {
		t.Fatalf("one override must advance the serving revision once and rewrite no derived row:\n%+v\nwas\n%+v", overridden, settled)
	}
	if want := fmt.Sprintln("session", "codex", cohortCodex); overridden.invalidations != want {
		t.Fatalf("override invalidated %q, want only its own session partition %q", overridden.invalidations, want)
	}
	c.run(t, "--format", "json", "session", "scope", "set", "all")
	if got := read(); got.usage != overridden.usage+1 || got.session != overridden.session || got.invalidations != overridden.invalidations {
		t.Fatalf("a default change must advance the revision once and invalidate nothing:\n%+v\nwas\n%+v", got, overridden)
	}

	// Unchanged passes: the classification rows stay byte-identical, and the
	// enabled source adds exactly one revision (its run summary) per pass over
	// the same pass with the source disabled.
	scan()
	first := read()
	scan()
	second := read()
	if second.derived != first.derived {
		t.Fatal("an unchanged scan rewrote classification rows")
	}
	c.run(t, "--format", "json", "session", "origin-source", "disable", "claude-mem")
	scan()
	third := read()
	scan()
	fourth := read()
	if fourth.derived != third.derived {
		t.Fatal("an unchanged scan with the source disabled rewrote classification rows")
	}
	if enabled, disabled := second.usage-first.usage, fourth.usage-third.usage; enabled-disabled != 1 {
		t.Fatalf("an unchanged pass advanced the serving revision by %v with the source enabled and %v with it disabled; the source may cost exactly one", enabled, disabled)
	}
}
