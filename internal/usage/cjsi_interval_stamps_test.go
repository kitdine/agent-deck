package usage

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

const intervalStampFixtureDDL = `-- Cache invalidation stamps only. Facts and coverage remain in their own tables.
-- Day bounds are signed UTC days since Unix epoch and form [from_day, to_day).
CREATE TABLE main.cjsi_usage_interval_dirty_ranges (
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0),
    scope_kind TEXT NOT NULL CHECK(scope_kind IN ('aggregate-only','model','client-summary','all-summary','coverage')),
    client TEXT NOT NULL,
    model TEXT NOT NULL,
    from_day INTEGER NOT NULL,
    to_day INTEGER NOT NULL CHECK(to_day>from_day),
    ticket INTEGER NOT NULL CHECK(ticket>0),
    CHECK((scope_kind='all-summary' AND client='') OR (scope_kind<>'all-summary' AND client<>'')),
    CHECK(scope_kind='model' OR model=''),
    PRIMARY KEY(core_epoch,scope_kind,client,model,from_day,to_day)
) STRICT, WITHOUT ROWID;
`

func newStampFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	installIntervalTables(t, db, "cjsi_index_schema.go", "cjsi_publication_head")
	intervalFixtureExec(t, db, `INSERT INTO cjsi_publication_head(singleton,core_epoch,ticket) VALUES(1,11,1)`)
	intervalFixtureExec(t, db, intervalStampFixtureDDL)
	return db
}

func stampFixtureTransaction(t *testing.T, db *sql.DB, run func(*sql.Tx) error) error {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := run(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func stampFixtureHead(t *testing.T, tx *sql.Tx, prior, next int64) intervalStampHead {
	t.Helper()
	result, err := tx.ExecContext(t.Context(), `UPDATE cjsi_publication_head SET ticket=? WHERE singleton=1 AND core_epoch=11 AND ticket=?`, next, prior)
	if err != nil {
		t.Fatal(err)
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		t.Fatalf("fixture CAS changed %d: %v", n, err)
	}
	return intervalStampHead{coreEpoch: 11, ticket: next}
}

func stampFixturePlan(t *testing.T, lo, hi time.Time, model string, remove bool) intervalImpactPlan {
	t.Helper()
	m := ledger.Member{
		Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "A", ID: "i"},
		Model:  model,
		Tokens: ledger.Counts{Input: 7},
		Interval: ledger.CounterRange{
			Epoch:   [32]byte{1},
			Lower:   ledger.Counts{Input: 20},
			Upper:   ledger.Counts{Input: 40},
			LowerAt: lo,
			UpperAt: hi,
		},
	}
	i := intervalImpactImage{owner: SessionKey{"codex", "A"}, member: m}
	if model != "" {
		i.model = &wholeIntervalModel{key: m.Key, bounds: m.Interval, model: model}
	}
	before, after := []intervalImpactImage{}, []intervalImpactImage{i}
	if remove {
		before, after = after, before
	}
	p, err := planIntervalImpact(before, after)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func stampFixtureNumber(t *testing.T, tx *sql.Tx, query string) int64 {
	t.Helper()
	var n int64
	if err := tx.QueryRowContext(t.Context(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCJSIIntervalImpactPlannerR5F1(t *testing.T) {
	day := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	// R5-F1: span=10, measured detail=8, residual=9 with detail=6 => 1 remaining
	// primary cannot hold 2 remaining detail tokens. Must fail on insert,
	// identical invalid sides, and withdrawal for both cached and reasoning.
	badCached := intervalImpactImage{
		owner: SessionKey{"codex", "A"},
		member: ledger.Member{
			Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "A", ID: "bad-c"},
			Tokens: ledger.Counts{Input: 9, Cached: 6, Known: ledger.CachedKnown},
			Interval: ledger.CounterRange{
				Epoch:   [32]byte{1},
				Lower:   ledger.Counts{Input: 10, Cached: 0, Known: ledger.CachedKnown},
				Upper:   ledger.Counts{Input: 20, Cached: 8, Known: ledger.CachedKnown},
				LowerAt: day, UpperAt: day,
			},
		},
	}
	badReasoning := intervalImpactImage{
		owner: SessionKey{"codex", "A"},
		member: ledger.Member{
			Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "A", ID: "bad-r"},
			Tokens: ledger.Counts{Output: 9, Reasoning: 6, Known: ledger.ReasoningKnown},
			Interval: ledger.CounterRange{
				Epoch:   [32]byte{1},
				Lower:   ledger.Counts{Output: 10, Reasoning: 0, Known: ledger.ReasoningKnown},
				Upper:   ledger.Counts{Output: 20, Reasoning: 8, Known: ledger.ReasoningKnown},
				LowerAt: day, UpperAt: day,
			},
		},
	}
	for _, bad := range []intervalImpactImage{badCached, badReasoning} {
		for _, pair := range [][2][]intervalImpactImage{
			{{}, {bad}},    // insert
			{{bad}, {bad}}, // identical invalid sides (must NOT return empty no-op plan!)
			{{bad}, {}},    // withdraw
		} {
			if plan, err := planIntervalImpact(pair[0], pair[1]); err == nil || plan != nil {
				t.Fatalf("planner accepted infeasible residual %s: plan=%+v err=%v", bad.member.Key.ID, plan, err)
			}
		}
	}
	// Feasible boundary (detail=7 leaves 1<=1) produces 4 scopes.
	good := badCached
	good.member.Tokens.Cached = 7
	good.member.Model = "M"
	plan, err := planIntervalImpact([]intervalImpactImage{}, []intervalImpactImage{good})
	if err != nil || len(plan) != 4 {
		t.Fatalf("feasible boundary failed: plan=%+v err=%v", plan, err)
	}
	// Task 3's TokenInterval.Model automatically produces impactModel even when
	// image.model is nil.
	hasModelScope := false
	for _, r := range plan {
		if r.scope == (intervalImpactScope{kind: impactModel, client: "codex", model: "M"}) {
			hasModelScope = true
		}
	}
	if !hasModelScope {
		t.Fatalf("expected impactModel from member.Model: %+v", plan)
	}
}

func TestCJSIIntervalStamps(t *testing.T) {
	ctx := t.Context()
	db := newStampFixture(t)
	day := func(n int) time.Time { return time.Date(1970, 1, 1+n, 0, 0, 0, 0, time.UTC) }
	west := time.FixedZone("fixture-west", -7*3600)
	lo, hi := day(-2).Add(time.Second).In(west), day(-1).In(west)
	publish := func(prior, next int64, plan intervalImpactPlan) {
		t.Helper()
		if err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
			return writeIntervalStamps(ctx, tx, stampFixtureHead(t, tx, prior, next), plan)
		}); err != nil {
			t.Fatal(err)
		}
	}
	publish(1, 2, stampFixturePlan(t, lo, hi, "", false))
	publish(2, 3, stampFixturePlan(t, lo, hi, "", true))
	publish(3, 4, intervalImpactPlan{})
	narrow := stampFixturePlan(t, day(5), day(5), "M", false)
	publish(4, 5, narrow)
	publish(5, 6, stampFixturePlan(t, day(5), day(6), "M", false))

	intervalFixtureExec(t, db, `INSERT INTO cjsi_usage_interval_dirty_ranges VALUES
		(11,'model','codex','',-2,0,1),(10,'aggregate-only','codex','',-2,0,99)`)
	h := intervalStampHead{11, 6}
	agg := intervalImpactScope{kind: impactAggregateOnly, client: "codex"}
	model := intervalImpactScope{kind: impactModel, client: "codex", model: "M"}
	all := intervalImpactScope{kind: impactAllSummary}
	coverage := intervalImpactScope{kind: impactCoverage, client: "codex"}
	client := intervalImpactScope{kind: impactClientSummary, client: "codex"}
	err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		check := func(scope intervalImpactScope, from, to time.Time, want int64) {
			t.Helper()
			got, err := readIntervalStamp(ctx, tx, h, scope, from.In(west), to.In(west))
			if err != nil || got != want {
				t.Fatalf("%+v [%s,%s) = %d, %v; want %d", scope, from, to, got, err, want)
			}
		}
		for _, k := range []intervalImpactScope{agg, all, client, coverage} {
			check(k, day(-3), day(-2), 0)
			check(k, day(-3), day(-2).Add(time.Nanosecond), 3)
			check(k, day(0).Add(-time.Nanosecond), day(0), 3)
			check(k, day(0), day(5), 0)
		}
		for _, k := range []intervalImpactScope{model, all, client, coverage} {
			check(k, day(4), day(5), 0)
			check(k, day(4), day(5).Add(time.Nanosecond), 6)
			check(k, day(6), day(7), 6)
			check(k, day(7), day(8), 0)
		}
		check(agg, day(5), day(6), 0)
		check(model, day(-2), day(0), 0)
		check(intervalImpactScope{kind: impactModel, client: "codex"}, day(-2), day(0), 1)
		check(intervalImpactScope{kind: impactModel, client: "codex", model: "N"}, day(5), day(6), 0)
		check(intervalImpactScope{kind: impactCoverage, client: "claude"}, day(-2), day(7), 0)
		if stampFixtureNumber(t, tx, `SELECT ticket FROM cjsi_usage_interval_dirty_ranges WHERE core_epoch=11 AND scope_kind='model' AND model='M' AND from_day=5 AND to_day=6`) != 5 {
			t.Fatal("overlap rewrote a different key")
		}
		h = stampFixtureHead(t, tx, 6, 7)
		if err := writeIntervalStamps(ctx, tx, h, narrow); err != nil {
			return err
		}
		changes := stampFixtureNumber(t, tx, `SELECT total_changes()`)
		if err := writeIntervalStamps(ctx, tx, h, narrow); err != nil {
			return err
		}
		if err := writeIntervalStamps(ctx, tx, h, intervalImpactPlan{}); err != nil {
			return err
		}
		if stampFixtureNumber(t, tx, `SELECT total_changes()`) != changes {
			t.Fatal("replay or empty plan wrote rows")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCJSIIntervalStampsFailClosed(t *testing.T) {
	ctx := t.Context()
	db := newStampFixture(t)
	day := time.Unix(0, 0).UTC()
	p := stampFixturePlan(t, day, day, "", false)
	agg := intervalImpactScope{kind: impactAggregateOnly, client: "codex"}
	h := intervalStampHead{11, 1}
	rollbackProbes := errors.New("rollback validation probes")
	err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		for _, bad := range []intervalStampHead{{12, 1}, {11, 2}, {0, 1}, {11, 0}, {11, -1}} {
			if err := writeIntervalStamps(ctx, tx, bad, intervalImpactPlan{}); err == nil {
				t.Fatal("unprotected empty write", bad)
			}
			if _, err := readIntervalStamp(ctx, tx, bad, agg, day, day.Add(time.Second)); err == nil {
				t.Fatal("unprotected read", bad)
			}
		}
		if err := writeIntervalStamps(ctx, tx, h, nil); err == nil {
			t.Fatal("unavailable plan accepted")
		}
		for _, mutate := range []func(*intervalDirtyRange){
			func(r *intervalDirtyRange) { r.from = r.from.Add(time.Nanosecond) }, func(r *intervalDirtyRange) { r.to = r.from },
			func(r *intervalDirtyRange) { r.scope.kind = 99 }, func(r *intervalDirtyRange) { r.scope = intervalImpactScope{kind: impactAllSummary, client: "codex"} },
			func(r *intervalDirtyRange) { r.scope.model = "unsourced" }, func(r *intervalDirtyRange) { r.scope.client = "" },
		} {
			bad := slices.Clone(p)
			mutate(&bad[len(bad)-1])
			if err := writeIntervalStamps(ctx, tx, h, bad); err == nil {
				t.Fatal("malformed final range accepted")
			}
			if stampFixtureNumber(t, tx, `SELECT count(*) FROM cjsi_usage_interval_dirty_ranges`) != 0 {
				t.Fatal("wrote before validating the full plan")
			}
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := readIntervalStamp(cancelled, tx, h, agg, day, day.Add(time.Second)); !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled snapshot read", err)
		}
		return rollbackProbes
	})
	if !errors.Is(err, rollbackProbes) {
		t.Fatal(err)
	}
	intervalFixtureExec(t, db, `CREATE TRIGGER abort_coverage BEFORE INSERT ON cjsi_usage_interval_dirty_ranges
		WHEN NEW.scope_kind='coverage' BEGIN SELECT RAISE(ABORT,'fixture failure'); END`)
	err = stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		return writeIntervalStamps(ctx, tx, stampFixtureHead(t, tx, 1, 2), p)
	})
	if err == nil {
		t.Fatal("trigger failure escaped")
	}
	intervalFixtureExec(t, db, `DROP TRIGGER abort_coverage`)
	var committedHead int64
	if err := db.QueryRowContext(ctx, `SELECT ticket FROM cjsi_publication_head`).Scan(&committedHead); err != nil || committedHead != 1 {
		t.Fatal("failed stamp scan committed its head", committedHead, err)
	}
}

func TestCJSIIntervalStampsYearOneBoundary(t *testing.T) {
	midnight := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	previous := midnight.AddDate(0, 0, -1)
	next := midnight.AddDate(0, 0, 1)
	for _, tc := range []struct {
		name                     string
		from, to                 time.Time
		disjointFrom, disjointTo time.Time
	}{
		{"midnight range start", midnight, next, previous, midnight},
		{"midnight exclusive end", previous, midnight, midnight, next},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			db := newStampFixture(t)
			plan := stampFixturePlan(t, tc.from.Add(time.Hour), tc.from.Add(2*time.Hour), "", false)
			if len(plan) != 4 {
				t.Fatalf("expected four planner footprints, got %d", len(plan))
			}
			for _, bounds := range plan {
				if !bounds.from.Equal(tc.from) || !bounds.to.Equal(tc.to) {
					t.Fatalf("planner bounds = [%s,%s), want [%s,%s)", bounds.from, bounds.to, tc.from, tc.to)
				}
				hit, err := plan.intersects(bounds.scope, tc.from, tc.to)
				if err != nil || !hit {
					t.Fatalf("plan.intersects year-one failed: hit=%v err=%v", hit, err)
				}
				miss, err := plan.intersects(bounds.scope, tc.disjointFrom, tc.disjointTo)
				if err != nil || miss {
					t.Fatalf("plan.intersects adjacent failed: miss=%v err=%v", miss, err)
				}
			}
			if err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
				return writeIntervalStamps(ctx, tx, stampFixtureHead(t, tx, 1, 2), plan)
			}); err != nil {
				t.Fatalf("write valid projected year-one day bounds: %v", err)
			}
			dFrom, _ := store.UsageIntervalStampDay(tc.from)
			dTo, _ := store.UsageIntervalStampDay(tc.to)
			if err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
				head := intervalStampHead{coreEpoch: 11, ticket: 2}
				var rows int
				err := tx.QueryRowContext(ctx, "SELECT count(*) FROM cjsi_usage_interval_dirty_ranges WHERE from_day=? AND to_day=? AND ticket=2",
					dFrom, dTo).Scan(&rows)
				if err != nil || rows != len(plan) {
					t.Fatalf("persisted signed day bounds: %d rows, error %v", rows, err)
				}
				for _, span := range plan {
					stamp, err := readIntervalStamp(ctx, tx, head, span.scope, tc.from, tc.to)
					if err != nil || stamp != 2 {
						t.Fatalf("read year-one stamp: got %d, %v; want 2", stamp, err)
					}
					stamp, err = readIntervalStamp(ctx, tx, head, span.scope, tc.disjointFrom, tc.disjointTo)
					if err != nil || stamp != 0 {
						t.Fatalf("read adjacent disjoint day: got %d, %v; want 0", stamp, err)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
