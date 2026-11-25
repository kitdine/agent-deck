package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func openSchema37PartitionFixture(t *testing.T) *Store {
	t.Helper()
	ctx := t.Context()
	root := t.TempDir()
	db, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Install the schema 37 head and interval dirty-range tables directly on
	// an isolated temporary database so partition invalidation is exercised
	// alongside the real usage_derived_partitions triggers and metadata.
	if _, err := db.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main.cjsi_publication_head (
		singleton INTEGER PRIMARY KEY CHECK(singleton=1),
		core_epoch INTEGER NOT NULL CHECK(core_epoch>0),
		ticket INTEGER NOT NULL CHECK(ticket>=0),
		index_epoch INTEGER NOT NULL DEFAULT 0,
		session_revision INTEGER NOT NULL DEFAULT 0,
		coverage_revision INTEGER NOT NULL DEFAULT 0,
		protection_revision INTEGER NOT NULL DEFAULT 0
	) STRICT, WITHOUT ROWID;
	INSERT OR REPLACE INTO main.cjsi_publication_head(singleton,core_epoch,ticket) VALUES(1,11,1);
	CREATE TABLE IF NOT EXISTS main.cjsi_usage_interval_dirty_ranges (
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
	) STRICT, WITHOUT ROWID;`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCJSIPartitionInvalidation(t *testing.T) {
	ctx := t.Context()

	t.Run("old and new dirty ranges, delete last interval, and client isolation", func(t *testing.T) {
		db := openSchema37PartitionFixture(t)
		// Seed both Codex and Claude events in usage_events so real triggers
		// populate usage_derived_partitions.
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES
			('cx-1','codex','s1','e1','2026-01-02T10:00:00Z','M','f',0),
			('cx-empty','codex','s1','e2','2026-01-02T10:05:00Z','','f',1),
			('cl-1','claude','s2','e3','2026-01-02T10:00:00Z','C','g',0)`); err != nil {
			t.Fatal(err)
		}
		day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		fromDay, ok := UsageIntervalStampDay(day)
		if !ok {
			t.Fatal("expected UTC midnight")
		}
		toDay := fromDay + 1

		// Publish clean priced-event partitions for both Codex and Claude.
		parts, err := db.UsagePartitions(ctx, day, day.AddDate(0, 0, 1))
		if err != nil || len(parts) != 3 {
			t.Fatalf("partitions=%+v err=%v", parts, err)
		}
		metaRev, err := db.UsageProjectionMetadataRevision(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for i := range parts {
			rev, err := db.BeginUsagePartition(ctx, parts[i])
			if err != nil {
				t.Fatal(err)
			}
			if err := db.PublishUsagePartition(ctx, parts[i], rev, metaRev, "v1", []byte("clean")); err != nil {
				t.Fatal(err)
			}
			parts[i].Revision = rev
			parts[i].Dirty = false
		}

		writeTicket := func(ticket int64, ranges []IntervalDirtyRange) {
			t.Helper()
			tx, err := db.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET ticket=? WHERE singleton=1 AND core_epoch=11`, ticket); err != nil {
				t.Fatal(err)
			}
			if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, ticket, ranges); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}

		// Ticket 2: add an unproved-model Codex interval (aggregate-only).
		aggRanges := []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAggregateOnly, Client: "codex", FromDay: fromDay, ToDay: toDay},
			{ScopeKind: IntervalDirtyClientSummary, Client: "codex", FromDay: fromDay, ToDay: toDay},
			{ScopeKind: IntervalDirtyAllSummary, FromDay: fromDay, ToDay: toDay},
			{ScopeKind: IntervalDirtyCoverage, Client: "codex", FromDay: fromDay, ToDay: toDay},
		}
		writeTicket(2, aggRanges)

		// Priced-event partitions for Claude (and Codex event rows themselves)
		// remain clean: interval stamps live in their own typed relation.
		if err := db.CheckUsagePartitions(ctx, day, day.AddDate(0, 0, 1), parts); err != nil {
			t.Fatalf("interval stamp dirtied priced-event partitions: %v", err)
		}

		// Ticket 3: delete the last interval — OLD range must STILL be stamped
		// so cached aggregates cannot resurrect stale interval tokens.
		writeTicket(3, aggRanges)

		// Ticket 4: empty plan (no interval change) must preserve ticket 3.
		writeTicket(4, []IntervalDirtyRange{})

		tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		// Typed namespace isolation: aggregate-only stamp at ticket 3 does NOT
		// collide with model='' (empty model) or Claude scopes.
		check := func(kind IntervalDirtyScopeKind, client, model string, d0, d1, want int64) {
			t.Helper()
			got, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, 4, kind, client, model, d0, d1)
			if err != nil || got != want {
				t.Fatalf("%s/%s/%q [%d,%d) = %d, %v; want %d", kind, client, model, d0, d1, got, err, want)
			}
		}
		check(IntervalDirtyAggregateOnly, "codex", "", fromDay, toDay, 3)
		check(IntervalDirtyClientSummary, "codex", "", fromDay, toDay, 3)
		check(IntervalDirtyAllSummary, "", "", fromDay, toDay, 3)
		check(IntervalDirtyCoverage, "codex", "", fromDay, toDay, 3)
		check(IntervalDirtyModel, "codex", "", fromDay, toDay, 0)
		check(IntervalDirtyModel, "codex", "M", fromDay, toDay, 0)
		check(IntervalDirtyClientSummary, "claude", "", fromDay, toDay, 0)
		check(IntervalDirtyCoverage, "claude", "", fromDay, toDay, 0)
		// Disjoint adjacent days return 0.
		check(IntervalDirtyAllSummary, "", "", fromDay-1, fromDay, 0)
		check(IntervalDirtyAllSummary, "", "", toDay, toDay+1, 0)
	})

	t.Run("year-one UTC midnight boundary (CJSI-AGG-R7-F1)", func(t *testing.T) {
		db := openSchema37PartitionFixture(t)
		midnight := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
		prev := midnight.AddDate(0, 0, -1)
		next := midnight.AddDate(0, 0, 1)

		dPrev, mPrev := UsageIntervalStampDay(prev)
		dMid, mMid := UsageIntervalStampDay(midnight)
		dNext, mNext := UsageIntervalStampDay(next)
		if !mPrev || !mMid || !mNext || dPrev+1 != dMid || dMid+1 != dNext {
			t.Fatalf("day numbering broken around year 1: prev=%d/%v mid=%d/%v next=%d/%v", dPrev, mPrev, dMid, mMid, dNext, mNext)
		}
		// Non-midnight inside negative Unix day floors downward.
		dMidPlus, isMid := UsageIntervalStampDay(midnight.Add(time.Hour))
		if isMid || dMidPlus != dMid {
			t.Fatalf("expected floor to %d, got %d (midnight=%v)", dMid, dMidPlus, isMid)
		}

		for i, span := range [][2]int64{{dMid, dNext}, {dPrev, dMid}} {
			ticket := int64(2 + i)
			tx, err := db.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET ticket=? WHERE singleton=1 AND core_epoch=11`, ticket); err != nil {
				tx.Rollback()
				t.Fatal(err)
			}
			err = WriteUsageIntervalDirtyRanges(ctx, tx, 11, ticket, []IntervalDirtyRange{
				{ScopeKind: IntervalDirtyAllSummary, FromDay: span[0], ToDay: span[1]},
			})
			if err != nil {
				tx.Rollback()
				t.Fatalf("span %v rejected: %v", span, err)
			}
			got, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, ticket, IntervalDirtyAllSummary, "", "", span[0], span[1])
			if err != nil || got != ticket {
				tx.Rollback()
				t.Fatalf("read span %v = %d, %v; want %d", span, got, err, ticket)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("fail-closed on future stamp, mismatched head, invalid scope, and rollback", func(t *testing.T) {
		db := openSchema37PartitionFixture(t)
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		// Invalid scopes / bounds / nil ranges / wrong head all fail closed.
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, nil); err == nil {
			t.Fatal("accepted nil ranges")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 2, []IntervalDirtyRange{}); err == nil {
			t.Fatal("accepted mismatched ticket head")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 12, 1, []IntervalDirtyRange{}); err == nil {
			t.Fatal("accepted mismatched epoch head")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAllSummary, FromDay: 5, ToDay: 5},
		}); err == nil {
			t.Fatal("accepted empty day range")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAllSummary, Client: "codex", FromDay: 5, ToDay: 6},
		}); err == nil {
			t.Fatal("accepted all-summary with client")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAggregateOnly, Client: "codex", Model: "M", FromDay: 5, ToDay: 6},
		}); err == nil {
			t.Fatal("accepted aggregate-only with model")
		}
		if _, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, 1, IntervalDirtyAllSummary, "", "", 5, 5); err == nil {
			t.Fatal("read accepted empty day range")
		}
		// Inject a corrupt future-ticket overlap and confirm both read and write reject it.
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_usage_interval_dirty_ranges VALUES(11,'coverage','codex','',10,12,9)`); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, 1, IntervalDirtyCoverage, "codex", "", 11, 13); err == nil {
			t.Fatal("read accepted future-ticket stamp")
		}
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAllSummary, FromDay: 11, ToDay: 12},
			{ScopeKind: IntervalDirtyCoverage, Client: "codex", FromDay: 11, ToDay: 12},
		}); err == nil {
			t.Fatal("write accepted future-ticket overlap")
		}
		var allCount int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM main.cjsi_usage_interval_dirty_ranges WHERE scope_kind='all-summary'`).Scan(&allCount); err != nil || allCount != 0 {
			t.Fatalf("write partially inserted before preflight failed: count=%d err=%v", allCount, err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, 1, IntervalDirtyAllSummary, "", "", 10, 12); !errors.Is(err, sql.ErrTxDone) {
			t.Fatalf("closed tx error=%v", err)
		}
	})

	t.Run("long multi-year range stays compact without per-day expansion", func(t *testing.T) {
		db := openSchema37PartitionFixture(t)
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		d0, _ := UsageIntervalStampDay(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC))
		d1, _ := UsageIntervalStampDay(time.Date(2500, 1, 2, 0, 0, 0, 0, time.UTC))
		if err := WriteUsageIntervalDirtyRanges(ctx, tx, 11, 1, []IntervalDirtyRange{
			{ScopeKind: IntervalDirtyAggregateOnly, Client: "codex", FromDay: d0, ToDay: d1},
			{ScopeKind: IntervalDirtyClientSummary, Client: "codex", FromDay: d0, ToDay: d1},
			{ScopeKind: IntervalDirtyAllSummary, FromDay: d0, ToDay: d1},
			{ScopeKind: IntervalDirtyCoverage, Client: "codex", FromDay: d0, ToDay: d1},
		}); err != nil {
			t.Fatal(err)
		}
		var rows int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM main.cjsi_usage_interval_dirty_ranges`).Scan(&rows); err != nil || rows != 4 {
			t.Fatalf("expected 4 compact rows, got %d (err=%v)", rows, err)
		}
		mid, _ := UsageIntervalStampDay(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
		got, err := ReadUsageIntervalDirtyStamp(ctx, tx, 11, 1, IntervalDirtyAllSummary, "", "", mid, mid+1)
		if err != nil || got != 1 {
			t.Fatalf("midpoint query = %d, %v; want 1", got, err)
		}
	})
}
