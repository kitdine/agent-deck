package usage

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

const (
	intervalStart = "2026-01-02T09:55:00.123456789Z"
	intervalEnd   = "2026-01-02T10:05:00Z"
	intervalLarge = 9007199254740993
)

func intervalFixtureExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), q, args...); err != nil {
		t.Fatal(err)
	}
}

// Install the actual STRICT tables and foreign keys from the frozen schema.
func installIntervalTables(t *testing.T, db *sql.DB, file string, tables ...string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "store", file))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		start := "CREATE TABLE main." + table + " ("
		_, tail, ok := strings.Cut(string(raw), start)
		stmt, _, end := strings.Cut(tail, ";")
		if !ok || !end {
			t.Fatalf("missing frozen table %s", table)
		}
		intervalFixtureExec(t, db, start+stmt+";")
	}
}

func newIntervalFixture(t *testing.T) (*sql.DB, cjsiIntervalSelection) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	intervalFixtureExec(t, db, "PRAGMA foreign_keys=ON")
	installIntervalTables(t, db, "cjsi_index_schema.go", "cjsi_publication_head")
	installIntervalTables(t, db, "cjsi_core_facts_schema.go", "cjsi_publication_attempts", "cjsi_usage_groups",
		"cjsi_usage_selection", "cjsi_usage_members", "cjsi_usage_intervals")
	intervalFixtureExec(t, db, `INSERT INTO cjsi_publication_head(singleton,core_epoch,ticket) VALUES(1,11,7)`)
	insertGroup := func(thread string, ticket, attempt int64, state string, selectIt bool) cjsiIntervalGroup {
		t.Helper()
		b := ledger.Basis{CoreEpoch: 11, Ticket: ticket, Attempt: attempt, BaseTicket: ticket - 1, InputDigest: [32]byte{byte(ticket), byte(attempt)}}
		var attemptDiff any
		if state != "draft" {
			attemptDiff = []byte("aggregate-attempt-digest-32bytes")
		}
		ref := cjsiIntervalGroup{thread, ledger.ProposalStamp{Basis: b, Diff: [32]byte{byte(ticket), byte(attempt), thread[0]}}}
		intervalFixtureExec(t, db, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,
			session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,
			input_digest,state,accept_until,diff_digest) VALUES(?,?,?,11,0,0,0,0,0,0,0,0,?,?,'2027-01-01T00:00:00Z',?)
			ON CONFLICT(ticket,attempt) DO NOTHING`, ticket, attempt, b.BaseTicket, b.InputDigest[:], state, attemptDiff)
		intervalFixtureExec(t, db, `INSERT INTO cjsi_usage_groups VALUES(?,?,?,?,?,3,128,0,1,0)`, ticket, attempt, thread, b.InputDigest[:], ref.stamp.Diff[:])
		if selectIt {
			intervalFixtureExec(t, db, `INSERT INTO cjsi_usage_selection VALUES(?,?,?)`, thread, ticket, attempt)
		}
		return ref
	}
	insertMember := func(g cjsiIntervalGroup, id, model string, n int64) {
		t.Helper()
		b := g.stamp.Basis
		// Collision across threads, tickets, attempts, and member kinds.
		intervalFixtureExec(t, db, `INSERT OR IGNORE INTO cjsi_usage_members VALUES(?,?,?,1,?,'','M',?,NULL,999,91,NULL,NULL,NULL)`,
			b.Ticket, b.Attempt, g.thread, id, intervalEnd)
		intervalFixtureExec(t, db, `INSERT INTO cjsi_usage_members VALUES(?,?,?,3,?,'',?,NULL,NULL,?,3,0,NULL,2)`,
			b.Ticket, b.Attempt, g.thread, id, model, n)
		intervalFixtureExec(t, db, `INSERT INTO cjsi_usage_intervals VALUES(?,?,?,3,?,?,?,?,50,?,7,26,NULL,5,2,NULL,1,5)`,
			b.Ticket, b.Attempt, g.thread, id, []byte("01234567890123456789012345678901"), intervalStart, intervalEnd, n+70)
	}
	old := insertGroup("A", 3, 2, "opened", true)
	current := insertGroup("B", 7, 1, "opened", true)
	insertMember(old, "same", "", intervalLarge)
	// R4-F2: two distinct member_ids in the same selected group B, PLUS a row
	// for the SAME thread B and SAME attempt 1 on a DIFFERENT ticket (ticket=6).
	// Removing member_id from the JOIN cross-joins B's two members into 4 rows;
	// removing ticket from WHERE includes ticket=6's extra row.
	insertMember(current, "same", "", 4)
	insertMember(current, "second", "", 2)
	intervalFixtureExec(t, db, `UPDATE cjsi_usage_members SET cached=NULL,cache_write=3,output_tokens=2,reasoning=1 WHERE thread='B' AND kind=3 AND member_id='same'`)
	intervalFixtureExec(t, db, `UPDATE cjsi_usage_members SET cached=NULL,cache_write=1,output_tokens=1,reasoning=1 WHERE thread='B' AND kind=3 AND member_id='second'`)
	intervalFixtureExec(t, db, `UPDATE cjsi_usage_intervals SET upper_input=74,upper_write=9 WHERE thread='B' AND member_id='same'`)
	intervalFixtureExec(t, db, `UPDATE cjsi_usage_intervals SET upper_input=72,upper_write=9 WHERE thread='B' AND member_id='second'`)
	// Same thread B, same attempt 1, DIFFERENT ticket 6:
	insertMember(insertGroup("B", 6, 1, "opened", false), "other-ticket", "", 55)
	insertMember(insertGroup("foreign", 3, 2, "opened", true), "same", "", 101)
	insertMember(insertGroup("A", 3, 1, "sealed", false), "same", "", 102)
	insertMember(insertGroup("A", 8, 1, "draft", false), "same", "", 103)
	empty := insertGroup("empty", 7, 1, "opened", true)
	return db, cjsiIntervalSelection{11, 7, []cjsiIntervalGroup{old, current, empty}}
}

func borrowIntervalFixture(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func TestCJSISelectedIntervalReader(t *testing.T) {
	from := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	scope := StatsOptions{From: from, To: to, Client: "codex"}
	db, selected := newIntervalFixture(t)
	tx := borrowIntervalFixture(t, db)

	// Group A has 1 row; Group B has exactly 2 distinct member_ids (proving
	// neither other-ticket row nor member_id cross-product entered).
	rowsA, err := readIntervalGroup(t.Context(), tx, selected.groups[0])
	if err != nil || len(rowsA) != 1 {
		t.Fatalf("group A rows=%v err=%v", rowsA, err)
	}
	rowsB, err := readIntervalGroup(t.Context(), tx, selected.groups[1])
	if err != nil || len(rowsB) != 2 {
		t.Fatalf("group B want 2 rows (R4-F2), got %d err=%v", len(rowsB), err)
	}
	start, _ := time.Parse(time.RFC3339Nano, intervalStart)
	end, _ := time.Parse(time.RFC3339Nano, intervalEnd)
	known := ledger.CachedKnown | ledger.ReasoningKnown
	want := ledger.Member{
		Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "A", ID: "same"},
		Tokens: ledger.Counts{Input: intervalLarge, Output: 3, Reasoning: 2, Known: known},
		Interval: ledger.CounterRange{
			Epoch:   [32]byte([]byte("01234567890123456789012345678901")),
			LowerAt: start, UpperAt: end,
			Lower: ledger.Counts{Input: 50, Output: 7, Write: 2, Reasoning: 1, Known: ledger.WriteKnown | ledger.ReasoningKnown},
			Upper: ledger.Counts{Input: intervalLarge + 70, Output: 26, Cached: 5, Reasoning: 5, Known: known},
		},
	}
	if !reflect.DeepEqual(rowsA[0], want) {
		t.Fatalf("decoded %+v, want %+v", rowsA[0], want)
	}

	read := func(s cjsiIntervalSelection, q StatsOptions, session string) scopedIntervalSum {
		t.Helper()
		got, err := readScopedIntervals(t.Context(), tx, s, q, session)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, client := range []string{"codex", ""} {
		q := scope
		q.Client = client
		got := read(selected, q, "")
		if got.counts != (ledger.Counts{Input: intervalLarge + 6, Output: 6, Write: 4, Reasoning: 4, Known: intervalDetails}) ||
			got.shareBase != intervalLarge+12 || got.partial || got.missing != (ledger.WriteKnown|ledger.CachedKnown) ||
			!reflect.DeepEqual(got.sessions, map[SessionKey]struct{}{{"codex", "A"}: {}, {"codex", "B"}: {}}) {
			t.Fatal(got)
		}
	}
	// Proved whole-interval model stored in cjsi_usage_members.model is decoded
	// and honored by model filtering (A4b).
	tx.Rollback()
	intervalFixtureExec(t, db, `UPDATE cjsi_usage_members SET model='M' WHERE thread='A' AND kind=3`)
	tx = borrowIntervalFixture(t, db)
	rowsAM, err := readIntervalGroup(t.Context(), tx, selected.groups[0])
	if err != nil || len(rowsAM) != 1 || rowsAM[0].Model != "M" {
		t.Fatalf("failed to decode TokenInterval.Model: %+v err=%v", rowsAM, err)
	}
	mScope := scope
	mScope.Model = "M"
	if got := read(selected, mScope, "A"); got.partial || got.allocated != 1 || got.counts.Input != intervalLarge {
		t.Fatalf("proved model filter on stored interval: %+v", got)
	}
	q := scope
	q.Client = "claude"
	got, err := readScopedIntervals(t.Context(), nil, cjsiIntervalSelection{}, q, "")
	if err != nil || !reflect.DeepEqual(got, scopedIntervalSum{}) {
		t.Fatalf("disjoint claude: %+v %v", got, err)
	}
}

func TestCJSISelectedIntervalReaderRejects(t *testing.T) {
	q := StatsOptions{From: time.Unix(1, 0), To: time.Unix(2, 0)}
	reject := func(t *testing.T, tx *sql.Tx, s cjsiIntervalSelection) {
		t.Helper()
		for _, window := range []StatsOptions{q, {From: time.Unix(1, 0), To: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}} {
			got, err := readScopedIntervals(t.Context(), tx, s, window, "")
			if err == nil || !reflect.DeepEqual(got, scopedIntervalSum{}) {
				t.Fatalf("accepted %+v, err=%v", got, err)
			}
		}
	}

	t.Run("R4-F1 exact residual feasibility across endpoints and remainder", func(t *testing.T) {
		for name, sqlUpdate := range map[string]string{
			// Unknown lower cached, upper cached=5, residual cached=6 (must reject!).
			"unknown lower bounded upper cached overflow": `UPDATE cjsi_usage_members SET input_tokens=6,cached=6 WHERE thread='A' AND kind=3;
				UPDATE cjsi_usage_intervals SET lower_input=50,upper_input=76,lower_cached=NULL,upper_cached=5 WHERE thread='A'`,
			// Both endpoints measured: span=26, cached span=26, residual input=6 cached=3 => remaining 20 inputs cannot hold 23 cached!
			"remaining input cannot absorb remaining cached": `UPDATE cjsi_usage_members SET input_tokens=6,cached=3 WHERE thread='A' AND kind=3;
				UPDATE cjsi_usage_intervals SET lower_input=50,upper_input=76,lower_cached=0,upper_cached=26 WHERE thread='A'`,
			// Same contradiction for reasoning/output.
			"remaining output cannot absorb remaining reasoning": `UPDATE cjsi_usage_members SET output_tokens=6,reasoning=3 WHERE thread='A' AND kind=3;
				UPDATE cjsi_usage_intervals SET lower_output=7,upper_output=33,lower_reasoning=0,upper_reasoning=26 WHERE thread='A'`,
			// Unknown lower bound constrained by lower primary: loP=10, hiP=20, hiD=18 => spanD>=8; residual nP=9 nD=6 => remP=1 cannot hold remD>=2!
			"unknown lower bound primary constrains remaining cached": `UPDATE cjsi_usage_members SET input_tokens=9,cached=6 WHERE thread='A' AND kind=3;
				UPDATE cjsi_usage_intervals SET lower_input=10,upper_input=20,lower_cached=NULL,upper_cached=18 WHERE thread='A'`,
			"unknown lower bound primary constrains remaining reasoning": `UPDATE cjsi_usage_members SET output_tokens=9,reasoning=6 WHERE thread='A' AND kind=3;
				UPDATE cjsi_usage_intervals SET lower_output=10,upper_output=20,lower_reasoning=NULL,upper_reasoning=18 WHERE thread='A'`,
		} {
			t.Run(name, func(t *testing.T) {
				db, s := newIntervalFixture(t)
				intervalFixtureExec(t, db, sqlUpdate)
				reject(t, borrowIntervalFixture(t, db), s)
			})
		}

		// Equality control (Round 6): loP=10 (loD unknown), hiP=20, hiD=18, nP=9, nD=7 (remP=1, remD=1) MUST PASS.
		db, s := newIntervalFixture(t)
		intervalFixtureExec(t, db, `UPDATE cjsi_usage_members SET input_tokens=9,cached=7,output_tokens=9,reasoning=7 WHERE thread='A' AND kind=3;
			UPDATE cjsi_usage_intervals SET lower_input=10,upper_input=20,lower_cached=NULL,upper_cached=18,
				lower_output=10,upper_output=20,lower_reasoning=NULL,upper_reasoning=18 WHERE thread='A'`)
		rows, err := readIntervalGroup(t.Context(), borrowIntervalFixture(t, db), s.groups[0])
		if err != nil || len(rows) != 1 || rows[0].Tokens.Cached != 7 || rows[0].Tokens.Reasoning != 7 {
			t.Fatalf("equality control failed: rows=%+v err=%v", rows, err)
		}

		// Both endpoints measured while residual detail is unknown MUST PASS and stay unknown.
		db2, s2 := newIntervalFixture(t)
		intervalFixtureExec(t, db2, `UPDATE cjsi_usage_members SET input_tokens=9,cached=NULL WHERE thread='A' AND kind=3;
			UPDATE cjsi_usage_intervals SET lower_input=10,upper_input=20,lower_cached=0,upper_cached=8 WHERE thread='A'`)
		rows2, err := readIntervalGroup(t.Context(), borrowIntervalFixture(t, db2), s2.groups[0])
		if err != nil || len(rows2) != 1 || rows2[0].Tokens.Known&ledger.CachedKnown != 0 {
			t.Fatalf("unknown residual with measured endpoints failed: rows=%+v err=%v", rows2, err)
		}
	})

	for name, mutation := range map[string]string{
		"missing selection":         `DELETE FROM cjsi_usage_selection WHERE thread='B'`,
		"private selection":         `UPDATE cjsi_usage_selection SET attempt=1 WHERE thread='A'`,
		"selected draft":            `UPDATE cjsi_publication_attempts SET state='draft' WHERE ticket=7`,
		"missing endpoints":         `DELETE FROM cjsi_usage_intervals WHERE thread='B' AND member_id='same'`,
		"missing exact member":      `PRAGMA foreign_keys=OFF; DELETE FROM cjsi_usage_members WHERE thread='B' AND kind=3 AND member_id='same'`,
		"NULL endpoint":             `UPDATE cjsi_usage_intervals SET lower_at=NULL WHERE thread='B' AND member_id='same'`,
		"bad timestamp":             `UPDATE cjsi_usage_intervals SET upper_at='yesterday' WHERE thread='B' AND member_id='same'`,
		"inverted time":             `UPDATE cjsi_usage_intervals SET upper_at='2025-01-01T00:00:00Z' WHERE thread='B' AND member_id='same'`,
		"empty epoch":               `UPDATE cjsi_usage_intervals SET counter_epoch=zeroblob(32) WHERE thread='B' AND member_id='same'`,
		"bad endpoint subset":       `UPDATE cjsi_usage_intervals SET lower_cached=51 WHERE thread='B' AND member_id='same'`,
		"regressing counter":        `UPDATE cjsi_usage_intervals SET lower_reasoning=6 WHERE thread='B' AND member_id='same'`,
		"residual over span":        `UPDATE cjsi_usage_members SET input_tokens=27 WHERE thread='B' AND kind=3 AND member_id='same'`,
		"residual detail over span": `UPDATE cjsi_usage_intervals SET lower_reasoning=5 WHERE thread='B' AND member_id='same'`,
		"invalid residual subset":   `UPDATE cjsi_usage_members SET cached=7 WHERE thread='B' AND kind=3 AND member_id='same'`,
		"missing relation":          `DROP TABLE cjsi_usage_intervals`,
	} {
		t.Run(name, func(t *testing.T) {
			db, s := newIntervalFixture(t)
			intervalFixtureExec(t, db, mutation)
			reject(t, borrowIntervalFixture(t, db), s)
		})
	}
}
