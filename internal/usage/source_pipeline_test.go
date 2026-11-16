package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// pipelineCorpus writes sources whose publication depends on other sources
// in the same round: Codex files sharing one session (the turn index of a
// later file reads the earlier file's classified signals), an archived copy
// owning the same events as its live source, Claude sessions with tools, a
// malformed record and an incomplete tail.
func pipelineCorpus(t *testing.T, root, home string, round int) {
	t.Helper()
	for i := range 24 {
		session := fmt.Sprintf("s%02d", i%6)
		path := filepath.Join(home, ".codex", "sessions", "2026", "10", fmt.Sprintf("%02d", 1+i%3), fmt.Sprintf("rollout-%02d.jsonl", i))
		lines := []string{
			`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"` + session + `"}}`,
			`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t` + fmt.Sprint(i) + `","model":"gpt-5"}}`,
		}
		for turn := range round + i%3 {
			at := fmt.Sprintf("2026-10-08T12:%02d:%02dZ", turn, i)
			lines = append(lines, codexSignalUser(t, at, []string{"fix the crash", "add the feature", "look at the parser"}[(turn+i)%3]))
			lines = append(lines, codexSignalJSON(t, at, "response_item", map[string]any{
				"type": "custom_tool_call", "name": "apply_patch", "call_id": fmt.Sprintf("edit-%d-%d", i, turn),
				"input": "*** Begin Patch\n*** Add File: " + filepath.Join(root, fmt.Sprintf("f%d.go", turn)) + "\n+package fixture\n*** End Patch",
			}))
			total := 1000 * (turn + 1)
			lines = append(lines, fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":%d,"output_tokens":100},"total_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":%d}}}}`, at, i, total, 100*(turn+1)))
		}
		if i == 5 {
			lines = append(lines, `{"timestamp":"2026-10-08T12:00:00Z","type":`)
		}
		writeSource(t, path, lines...)
		if i%8 == 0 {
			writeSource(t, filepath.Join(home, ".codex", "archived_sessions", fmt.Sprintf("rollout-%02d.jsonl", i)), lines...)
		}
	}
	for i := range 8 {
		session := fmt.Sprintf("c%d", i%3)
		var lines []string
		for turn := range round + 1 {
			at := fmt.Sprintf("2026-10-08T13:%02d:%02dZ", turn, i)
			lines = append(lines,
				claudeUser(session, at, "add the feature"),
				claudeAssistant(session, fmt.Sprintf("m%d-%d", i, turn), at, claudeEdit(fmt.Sprintf("e%d-%d", i, turn), filepath.Join(root, "c.go"))))
		}
		writeSource(t, filepath.Join(home, ".claude", "projects", "p", fmt.Sprintf("%d.jsonl", i)), lines...)
	}
	// A rewrite: round 2 truncates this source to a different first record.
	if round > 1 {
		writeSource(t, filepath.Join(home, ".codex", "sessions", "2026", "10", "02", "rollout-01.jsonl"),
			`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"rewritten"}}`)
	}
}

// usageRows dumps every usage table in a deterministic order.
func usageRows(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'usage\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	out := map[string][]string{}
	for _, table := range tables {
		data, err := db.QueryContext(ctx, "SELECT * FROM "+table)
		if err != nil {
			t.Fatal(err)
		}
		columns, _ := data.Columns()
		var lines []string
		for data.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err = data.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			fields := make([]string, len(values))
			for i, value := range values {
				if bytes, ok := value.([]byte); ok {
					value = string(bytes)
				}
				fields[i] = fmt.Sprintf("%s=%v", columns[i], value)
			}
			lines = append(lines, strings.Join(fields, "|"))
		}
		data.Close()
		sort.Strings(lines)
		out[table] = lines
	}
	return out
}

// Parallel reduce with one ordered writer must publish exactly what the serial
// writer publishes, for a cold import and for an incremental round whose
// sources depend on each other's committed state.
func TestParallelReduceMatchesSerialPublication(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	services := map[int]*Service{}
	for _, workers := range []int{1, 4, 8} {
		database, err := store.Open(ctx, filepath.Join(root, fmt.Sprintf("state-%d", workers)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		service := New(database, home)
		service.MachineIdentity = func(context.Context) (string, error) { return "machine-a", nil }
		service.ReduceWorkers = workers
		services[workers] = service
	}
	for round := 1; round <= 2; round++ {
		pipelineCorpus(t, root, home, round)
		results := map[int]map[string]int{}
		for workers, service := range services {
			changes, err := service.Scan(ctx)
			if err != nil {
				t.Fatalf("round %d workers %d: %v", round, workers, err)
			}
			results[workers] = changes
		}
		serial := usageRows(t, services[1].Store.DB)
		if len(serial["usage_events"]) == 0 || len(serial["usage_work_signals"]) == 0 || len(serial["usage_tool_calls"]) == 0 {
			t.Fatalf("round %d corpus produced no rows: %v", round, serial)
		}
		for _, workers := range []int{4, 8} {
			if !reflect.DeepEqual(results[workers], results[1]) {
				t.Fatalf("round %d workers %d changes = %v, serial %v", round, workers, results[workers], results[1])
			}
			parallel := usageRows(t, services[workers].Store.DB)
			for table, want := range serial {
				if !reflect.DeepEqual(parallel[table], want) {
					t.Fatalf("round %d workers %d table %s differs:\nparallel %v\nserial   %v", round, workers, table, parallel[table], want)
				}
			}
		}
	}
}

// A first import larger than one write transaction commits its facts in
// blocks; the published rows equal the serial single-transaction import,
// including duplicate events and tool calls whose start and completion fall
// in different blocks.
func TestLargeFirstImportBlocksMatchSingleTransaction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "large.jsonl")
	lines := []string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"large"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t","model":"gpt-5"}}`,
	}
	for i := range 900 {
		at := fmt.Sprintf("2026-10-08T%02d:%02d:%02dZ", 12+i/3600, i/60%60, i%60)
		if i%50 == 0 {
			lines = append(lines, codexSignalUser(t, at, []string{"fix the crash", "add the feature"}[i/50%2]))
		}
		call := fmt.Sprintf("call-%d", i)
		lines = append(lines, codexSignalJSON(t, at, "response_item", map[string]any{
			"type": "custom_tool_call", "name": "apply_patch", "call_id": call,
			"input": "*** Begin Patch\n*** Add File: " + filepath.Join(root, fmt.Sprintf("f%d.go", i%7)) + "\n+package fixture\n*** End Patch",
		}))
		if i >= 3 {
			// Completes the call started three records earlier.
			lines = append(lines, codexSignalJSON(t, at, "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": fmt.Sprintf("call-%d", i-3), "output": "ok"}))
		}
		usage := fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":1},"total_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":%d}}}}`, at, 10+i%5, 10*(i+1), i+1)
		lines = append(lines, usage)
		if i%97 == 0 {
			lines = append(lines, usage)
		}
	}
	writeSource(t, path, lines...)
	open := func(name string) *Service {
		database, err := store.Open(ctx, filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		service := New(database, home)
		service.MachineIdentity = func(context.Context) (string, error) { return "machine-a", nil }
		return service
	}
	blocked, single := open("blocked"), open("single")
	// A failure in the last block leaves the committed prefix without a
	// checkpoint; the next scan imports the whole source again over it.
	if _, err := blocked.Store.DB.Exec(`CREATE TRIGGER reject_checkpoint BEFORE INSERT ON usage_source_files BEGIN SELECT RAISE(ABORT,'synthetic crash'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.Scan(ctx); err == nil {
		t.Fatal("scan succeeded without its checkpoint")
	}
	var prefix, checkpoints int
	if err := blocked.Store.DB.QueryRow(`SELECT (SELECT count(*) FROM usage_events),(SELECT count(*) FROM usage_source_files)`).Scan(&prefix, &checkpoints); err != nil || prefix < writeTxRows || checkpoints != 0 {
		t.Fatalf("prefix events=%d checkpoints=%d err=%v", prefix, checkpoints, err)
	}
	if _, err := blocked.Store.DB.Exec(`DROP TRIGGER reject_checkpoint`); err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := open("fresh")
	blockedChanges, err := fresh.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = single.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	inventory, err := single.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	singleChanges, err := single.scanFileMode(ctx, inventory.Entries[0], false)
	if err != nil {
		t.Fatal(err)
	}
	want := usageRows(t, single.Store.DB)
	if len(want["usage_events"])+len(want["usage_tool_calls"]) < 3*writeTxRows {
		t.Fatalf("fixture fits one block: events=%d tools=%d", len(want["usage_events"]), len(want["usage_tool_calls"]))
	}
	for name, service := range map[string]*Service{"blocked": fresh, "recovered": blocked} {
		got := usageRows(t, service.Store.DB)
		for table, rows := range want {
			if table == "usage_source_files" && name == "recovered" {
				// Its counters also count the replay over the prefix.
				continue
			}
			if !reflect.DeepEqual(got[table], rows) {
				t.Fatalf("%s table %s differs from the single-transaction import:\ngot  %v\nwant %v", name, table, got[table], rows)
			}
		}
	}
	for _, key := range []string{"imported", "updated", "replaced"} {
		if blockedChanges[key] != singleChanges[key] {
			t.Fatalf("%s = %d, single transaction %d", key, blockedChanges[key], singleChanges[key])
		}
	}
}

// Coverage of unchanged sources commits in blocks; every source of an
// inventory larger than one block is still committed.
func TestUnchangedCoverageCommitsEverySourceAcrossBlocks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	count := 2*writeTxRows + 7
	for i := range count {
		writeSource(t, filepath.Join(home, ".claude", "projects", "p", fmt.Sprintf("%04d.jsonl", i)), claudeUser("s", "2026-10-08T12:00:00Z", "hello"))
	}
	service, database, _ := newSignalService(t, root)
	generation, err := database.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for round, id := range []string{"first", "unchanged"} {
		inventory, err := service.Inventory(ctx)
		if err != nil {
			t.Fatal(err)
		}
		sources := make([]store.InventorySource, 0, len(inventory.Entries))
		for _, entry := range inventory.Entries {
			sources = append(sources, store.InventorySource{Path: entry.Path, Identity: entry.Identity, End: entry.Size})
		}
		if err = database.AcceptInventory(ctx, id, generation.Epoch, sources, time.Now()); err != nil {
			t.Fatal(err)
		}
		service.CoverageInventoryID = id
		if _, err = service.ScanInventory(ctx, inventory); err != nil {
			t.Fatal(err)
		}
		coverage, err := database.ReadSourceCoverage(ctx)
		if err != nil || coverage.InventoryID != id || coverage.Sources != count || coverage.Committed != count {
			t.Fatalf("round %d coverage = %+v %v", round, coverage, err)
		}
	}
}

// closeCountingFile counts the Close calls of a source the scan opened.
type closeCountingFile struct {
	*os.File
	closed *atomic.Int64
}

func (f closeCountingFile) Close() error {
	f.closed.Add(1)
	return f.File.Close()
}

// A source that fails while later sources are being reduced ends the scan
// with its error. Its stream is never drained, so with a shared reader the
// later reductions wait on that reader; nothing outside the scan may be
// needed to end them. Every source the scan opened is closed exactly once,
// also those reduced but discarded after the failure.
func TestFailedSourceEndsScanAndClosesDiscardedSources(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%v", shared), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			home := filepath.Join(root, "home")
			for i := range 4 {
				writeSource(t, filepath.Join(home, ".claude", "projects", "p", fmt.Sprintf("%02d.jsonl", i)), claudeUser("s", "2026-10-28T00:00:00Z", "hello"))
			}
			database, err := store.Open(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { database.Close() })
			service := New(database, home)
			service.MachineIdentity = func(context.Context) (string, error) { return "machine-a", nil }
			service.ReduceWorkers = 4
			failure := errors.New("synthetic source open failure")
			var opened, closed atomic.Int64
			othersOpened := make(chan struct{})
			service.Open = func(path string) (SourceFile, error) {
				if filepath.Base(path) == "00.jsonl" {
					<-othersOpened
					time.Sleep(100 * time.Millisecond)
					return nil, failure
				}
				file, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				if opened.Add(1) == 3 {
					close(othersOpened)
				}
				return closeCountingFile{File: file, closed: &closed}, nil
			}
			if shared {
				sources, err := ingest.Discover(home)
				if err != nil {
					t.Fatal(err)
				}
				coordinator := ingest.NewCoordinator(sources, ingest.Options{Workers: 1})
				t.Cleanup(coordinator.Close)
				coordinator.ConsumerDone(ingest.ConsumerSession)
				service.Coordinator = coordinator
			}
			done := make(chan error, 1)
			go func() {
				_, err := service.Scan(ctx)
				done <- err
			}()
			select {
			case err = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("scan did not end after a source failed")
			}
			if !errors.Is(err, failure) {
				t.Fatalf("scan error = %v, want %v", err, failure)
			}
			if opened.Load() != 3 || closed.Load() != 3 {
				t.Fatalf("opened=%d closed=%d", opened.Load(), closed.Load())
			}
		})
	}
}
