package scanruntime

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// crashedAcceptance leaves the durable state a scan worker leaves when it is
// killed after accepting round id's inventory and before finishing it: both
// domains have a pending inventory with that id, and it is the coverage head.
func crashedAcceptance(t *testing.T, home, root, id string) {
	t.Helper()
	ctx := context.Background()
	sources, err := ingest.Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	for _, domain := range []struct {
		db      *store.Store
		session bool
	}{{core, false}, {sessions, true}} {
		if err := acceptDomainInventory(ctx, domain.db, id, sources, domain.session, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
}

type inventoryRun struct{ id, state string }

func inventoryRuns(t *testing.T, db *sql.DB) (head string, runs []inventoryRun) {
	t.Helper()
	if err := db.QueryRow(`SELECT inventory_id FROM domain_coverage_heads WHERE singleton=1`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT inventory_id,state FROM inventory_runs ORDER BY inventory_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var run inventoryRun
		if err := rows.Scan(&run.id, &run.state); err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return head, runs
}

// usageIdentity is every imported event's identity and token values, without
// the run binding, so two independent imports of one home compare equal.
func usageIdentity(t *testing.T, root string) []string {
	t.Helper()
	core, err := store.OpenReadOnly(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	rows, err := core.DB.Query(`SELECT event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens FROM usage_events ORDER BY event_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var key, client, session, event, at, model string
		var input, cached, output, read, creation int64
		if err := rows.Scan(&key, &client, &session, &event, &at, &model, &input, &cached, &output, &read, &creation); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%d|%d|%d|%d", key, client, session, event, at, model, input, cached, output, read, creation))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func recoverAcceptedRound(t *testing.T, home, root, roundID string) scanReceipt {
	t.Helper()
	journal, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.accept(scanReceipt{ID: "receipt-" + roundID, StateID: "state", Home: home, Scope: ScopeBoth, RoundID: roundID, Observation: 2}); err != nil {
		t.Fatal(err)
	}
	// A new worker reopens the journal and replans the persisted round with
	// the production runner; it holds the state root's scan lock.
	reopened, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	server := &server{ctx: context.Background(), stateRoot: root, stateID: "state", journal: reopened, rounds: map[string]*round{}}
	if err := server.recoverReceipts(); err != nil {
		t.Fatal(err)
	}
	if server.round == nil || server.round.id != roundID {
		t.Fatalf("recovered round=%#v, want persisted round %q", server.round, roundID)
	}
	select {
	case <-server.round.done:
	case <-time.After(30 * time.Second):
		t.Fatal("recovered round did not terminate")
	}
	final, found := reopened.get("receipt-" + roundID)
	if !found || final.State != receiptTerminal || final.Result == nil {
		t.Fatalf("recovered receipt=%#v found=%t", final, found)
	}
	return final
}

// A worker killed during a round leaves that round's pending inventory as the
// coverage head. Its replacement replays the accepted receipt under the same
// round id; the replay must complete instead of colliding with the abandoned
// acceptance, keep both domains' coverage consistent and import exactly what a
// fresh full scan of the same sources imports. A replay that was itself killed
// after accepting, and a worker killed after finishing its inventories but
// before recording the receipt terminal, recover the same way.
func TestRecoveredRoundCompletesAfterCrashedInventoryAcceptance(t *testing.T) {
	for _, test := range []struct {
		name  string
		crash func(t *testing.T, home, root string)
		want  []inventoryRun // precondition, head first
	}{
		{"crashed after acceptance", func(t *testing.T, home, root string) {
			crashedAcceptance(t, home, root, "crashed")
		}, []inventoryRun{{"crashed", "pending"}, {"seed", "complete"}}},
		{"replay crashed after acceptance", func(t *testing.T, home, root string) {
			crashedAcceptance(t, home, root, "crashed")
			for _, open := range []func(context.Context, string) (*store.Store, error){store.Open, store.OpenSessions} {
				db, err := open(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				exists, err := db.AbandonRoundInventories(context.Background(), "crashed")
				db.Close()
				if err != nil || !exists {
					t.Fatalf("first replay abandonment exists=%t err=%v", exists, err)
				}
			}
			crashedAcceptance(t, home, root, "crashed.first-replay")
		}, []inventoryRun{{"crashed.first-replay", "pending"}, {"crashed", "paused"}, {"seed", "complete"}}},
		{"crashed after finishing inventories", func(t *testing.T, home, root string) {
			finished := newRoundWithID(home, "crashed", 2)
			finished.executeWith(context.Background(), root, false, nil)
			if finished.result.Usage.State != "completed" || finished.result.Session.State != "completed" {
				t.Fatalf("finished usage=%+v session=%+v", finished.result.Usage, finished.result.Session)
			}
		}, []inventoryRun{{"crashed", "complete"}}},
	} {
		t.Run(test.name, func(t *testing.T) { testRecoveredRound(t, test.crash, test.want) })
	}
}

func testRecoveredRound(t *testing.T, crash func(t *testing.T, home, root string), want []inventoryRun) {
	ctx := context.Background()
	home := t.TempDir()
	root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first := writeServingSource(t, home, "2000/01/01", now.Add(-2*time.Hour), 11)
	writeServingSource(t, home, "2000/01/02", now.Add(-time.Hour), 13)

	seed := newRoundWithID(home, "seed", 1)
	seed.executeWith(ctx, root, false, nil)
	if seed.result.Usage.State != "completed" || seed.result.Session.State != "completed" {
		t.Fatalf("seed usage=%+v session=%+v", seed.result.Usage, seed.result.Session)
	}
	committed := usageIdentity(t, root)
	if len(committed) != 2 {
		t.Fatalf("seed imported %d events", len(committed))
	}

	appendServingRecord(t, first, now.Add(-time.Minute), 17)
	crash(t, home, root)
	if before := usageIdentity(t, root); len(before) != len(committed) && len(before) != len(committed)+1 {
		t.Fatalf("crash left %d events", len(before))
	}
	for _, open := range []func(context.Context, string) (*store.Store, error){store.Open, store.OpenSessions} {
		db, err := open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		head, runs := inventoryRuns(t, db.DB)
		db.Close()
		got := map[inventoryRun]bool{}
		for _, run := range runs {
			got[run] = true
		}
		if head != want[0].id || len(runs) != len(want) {
			t.Fatalf("crash precondition head=%q runs=%v, want %v", head, runs, want)
		}
		for _, run := range want {
			if !got[run] {
				t.Fatalf("crash precondition runs=%v, want %v", runs, want)
			}
		}
	}

	final := recoverAcceptedRound(t, home, root, "crashed")
	if final.Result.RoundID != "crashed" || final.Result.Usage.State != "completed" || final.Result.Session.State != "completed" {
		t.Fatalf("recovered round result usage=%+v session=%+v", final.Result.Usage, final.Result.Session)
	}
	if want[0].state == "pending" && final.Result.Usage.Changes["imported"] != 1 {
		t.Fatalf("recovered round imported %d events, want the appended one", final.Result.Usage.Changes["imported"])
	}

	heads := map[string]string{}
	for name, open := range map[string]func(context.Context, string) (*store.Store, error){"core": store.Open, "session": store.OpenSessions} {
		db, err := open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		head, runs := inventoryRuns(t, db.DB)
		coverage, coverageErr := db.ReadSourceCoverage(ctx)
		db.Close()
		if coverageErr != nil || coverage.State != "complete" || coverage.InventoryID != head || coverage.Sources != 2 || coverage.Committed != 2 {
			t.Fatalf("%s coverage=%+v err=%v", name, coverage, coverageErr)
		}
		if head == want[0].id || !strings.HasPrefix(head, "crashed.") {
			t.Fatalf("%s head=%q, want a new acceptance of round crashed", name, head)
		}
		for _, run := range runs {
			if run.state == "pending" {
				t.Fatalf("%s retained an abandoned pending inventory: %v", name, runs)
			}
		}
		heads[name] = head
	}
	if heads["core"] != heads["session"] {
		t.Fatalf("domains accepted different inventories: %v", heads)
	}

	recovered := usageIdentity(t, root)
	for i, event := range committed {
		if !contains(recovered, event) {
			t.Fatalf("committed event %d changed or lost: %s", i, event)
		}
	}
	oracleRoot, _, err := prepareStateRoot(filepath.Join(t.TempDir(), "oracle"))
	if err != nil {
		t.Fatal(err)
	}
	oracle := newRoundWithID(home, "oracle", 1)
	oracle.executeWith(ctx, oracleRoot, false, nil)
	if oracle.result.Usage.State != "completed" {
		t.Fatalf("oracle usage=%+v", oracle.result.Usage)
	}
	if want := usageIdentity(t, oracleRoot); strings.Join(recovered, "\n") != strings.Join(want, "\n") {
		t.Fatalf("recovered events differ from a fresh full scan:\n got %v\nwant %v", recovered, want)
	}

	// The next ordinary round is a no-op that keeps its own round id as the
	// inventory id and reclaims every superseded acceptance.
	next := newRoundWithID(home, "next", 3)
	next.executeWith(ctx, root, false, nil)
	if next.result.Usage.State != "completed" || next.result.Session.State != "completed" || next.result.Usage.Changes["imported"] != 0 {
		t.Fatalf("next usage=%+v session=%+v", next.result.Usage, next.result.Session)
	}
	for _, open := range []func(context.Context, string) (*store.Store, error){store.Open, store.OpenSessions} {
		db, err := open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		head, runs := inventoryRuns(t, db.DB)
		db.Close()
		if head != "next" || len(runs) != 1 || runs[0] != (inventoryRun{"next", "complete"}) {
			t.Fatalf("after next round head=%q runs=%v", head, runs)
		}
	}
	if strings.Join(usageIdentity(t, root), "\n") != strings.Join(recovered, "\n") {
		t.Fatal("no-op round changed imported events")
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// A replayed CLI round can be upgraded to serving by an App request during
// discovery; the serving execution accepts coverage through the same attempt
// naming and finishes both domains on the new acceptance.
func TestRecoveredServingRoundCompletesAfterCrashedInventoryAcceptance(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	writeServingSource(t, home, "2000/01/01", now.AddDate(0, 0, -20), 99)
	crashedAcceptance(t, home, root, "crashed")

	replay := newRoundWithID(home, "crashed", 2)
	replay.serving = true
	replay.cacheNow = func() time.Time { return now }
	replay.executeWith(ctx, root, false, nil)
	if replay.result.Usage.State != "completed" || replay.result.Session.State != "completed" {
		t.Fatalf("serving replay usage=%+v session=%+v", replay.result.Usage, replay.result.Session)
	}
	if replay.inventoryID == "crashed" || !strings.HasPrefix(replay.inventoryID, "crashed.") {
		t.Fatalf("serving replay inventory=%q", replay.inventoryID)
	}
	for name, open := range map[string]func(context.Context, string) (*store.Store, error){"core": store.Open, "session": store.OpenSessions} {
		db, err := open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		head, runs := inventoryRuns(t, db.DB)
		coverage, coverageErr := db.ReadSourceCoverage(ctx)
		db.Close()
		if head != replay.inventoryID || coverageErr != nil || coverage.State != "complete" || coverage.Sources != 2 || coverage.Committed != 2 {
			t.Fatalf("%s head=%q coverage=%+v err=%v runs=%v", name, head, coverage, coverageErr, runs)
		}
		for _, run := range runs {
			if run.state == "pending" {
				t.Fatalf("%s retained an abandoned pending inventory: %v", name, runs)
			}
		}
	}
	if got := usageIdentity(t, root); len(got) != 2 {
		t.Fatalf("serving replay imported %d events, want both sources", len(got))
	}
}
