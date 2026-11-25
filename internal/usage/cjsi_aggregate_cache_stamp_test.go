package usage

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIAggregateCacheStamp(t *testing.T) {
	db := newStampFixture(t)
	ctx := t.Context()
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		h := intervalStampHead{11, 1}
		q := StatsOptions{From: day, To: day.AddDate(0, 0, 1), Timezone: "UTC", Location: time.UTC, Client: "codex", Model: "M", Provider: "Q"}
		proof := [32]byte{1}
		read := func(q StatsOptions) [32]byte {
			t.Helper()
			stamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, h.coreEpoch, h.ticket, q, proof)
			if err != nil || stamp == ([32]byte{}) {
				t.Fatal("stamp", stamp, err)
			}
			return stamp
		}
		publish := func(at time.Time, remove bool) {
			t.Helper()
			h = stampFixtureHead(t, tx, h.ticket, h.ticket+1)
			if err := writeIntervalStamps(ctx, tx, h, stampFixturePlan(t, at, at, "N", remove)); err != nil {
				t.Fatal(err)
			}
		}
		claude, all := q, q
		claude.Client, all.Client = "claude", ""
		base, disjoint, global := read(q), read(claude), read(all)
		publish(q.To, false)
		if read(q) != base || read(all) != global {
			t.Fatal("disjoint day invalidated the query")
		}
		publish(day, false)
		first := read(q)
		if first == base || read(all) == global || read(claude) != disjoint {
			t.Fatal("wrong aggregate scope")
		}
		publish(day, true)
		last, lastAll := read(q), read(all)
		if last == first || read(claude) != disjoint {
			t.Fatal("lost withdrawal or dirtied unrelated client")
		}
		h = stampFixtureHead(t, tx, h.ticket, h.ticket+1)
		if err := writeIntervalStamps(ctx, tx, h, intervalImpactPlan{}); err != nil {
			return err
		}
		if read(q) != last || read(all) != lastAll {
			t.Fatal("empty plan resurrected a cache entry")
		}
		d, _ := store.UsageIntervalStampDay(day)
		set := func(kind string, ticket int) {
			t.Helper()
			res, err := tx.ExecContext(ctx, `UPDATE cjsi_usage_interval_dirty_ranges SET ticket=? WHERE scope_kind=? AND from_day=?`, ticket, kind, d)
			if err != nil {
				t.Fatal(err)
			}
			if n, err := res.RowsAffected(); n != 1 || err != nil {
				t.Fatal("fixture marker", n, err)
			}
		}
		set("client-summary", 3)
		if read(q) == last || read(all) != lastAll {
			t.Fatal("missing filtered-client denominator")
		}
		set("all-summary", 3)
		if read(all) == lastAll {
			t.Fatal("missing filtered-All denominator")
		}
		covered := read(q)
		set("coverage", 3)
		if read(q) == covered {
			t.Fatal("missing interval coverage")
		}
		beforeModel := read(q)
		otherModel := q
		otherModel.Model = "X"
		unrelated := read(otherModel)
		if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_usage_interval_dirty_ranges VALUES(11,'model','codex','M',?,?,?)`, d, d+1, h.ticket); err != nil {
			return err
		}
		if read(q) == beforeModel || read(otherModel) != unrelated {
			t.Fatal("model stamp crossed scopes")
		}
		proof[0]++
		if read(claude) == disjoint {
			t.Fatal("missing independent factual coverage")
		}
		for _, bad := range []StatsOptions{{}, {From: q.From, To: q.To, Client: "codex"}, {From: q.To, To: q.From, Timezone: "UTC"}, {From: q.From, To: q.To, Timezone: "other", Location: time.UTC}} {
			if st, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, h.ticket, bad, proof); err == nil || st != ([32]byte{}) {
				t.Fatal("invalid window", st, err)
			}
		}
		if _, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, h.ticket-1, q, proof); err == nil {
			t.Fatal("accepted stale head")
		}
		if _, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, h.ticket, q, [32]byte{}); err == nil {
			t.Fatal("missing factual coverage became verified empty")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCJSIAggregateCacheStampSystemTimezone(t *testing.T) {
	db := newStampFixture(t)
	ctx := t.Context()
	from := time.Date(2026, 1, 7, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 1, 8, 0, 0, 0, 0, time.Local)
	proof := [32]byte{1}

	// Find an IANA zone matching time.Local across [from, to] (e.g. America/Los_Angeles
	// under TZ=:/etc/localtime on macOS, or UTC on UTC runners), plus a genuinely
	// conflicting zone with a different UTC offset.
	_, localOffset := from.In(time.Local).Zone()
	var matchingZone string
	var conflictingZone string
	for _, candidate := range []string{"America/Los_Angeles", "America/New_York", "Europe/Berlin", "Asia/Tokyo", "UTC"} {
		loc, err := time.LoadLocation(candidate)
		if err != nil {
			continue
		}
		if matchingZone == "" && from.Equal(time.Date(2026, 1, 7, 0, 0, 0, 0, loc)) && to.Equal(time.Date(2026, 1, 8, 0, 0, 0, 0, loc)) {
			n1, o1 := from.In(time.Local).Zone()
			n2, o2 := from.In(loc).Zone()
			if n1 == n2 && o1 == o2 {
				matchingZone = candidate
			}
		}
		_, off := from.In(loc).Zone()
		if conflictingZone == "" && off != localOffset {
			conflictingZone = candidate
		}
	}
	if conflictingZone == "" {
		t.Fatal("failed to pick conflicting IANA zone")
	}

	err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		// 1. Named zone and time.Local with matching CLI display name both succeed
		// and yield the identical cache stamp (CJSI-AGG-R9-F8).
		if matchingZone != "" {
			named, err := time.LoadLocation(matchingZone)
			if err != nil {
				return err
			}
			namedOpts := StatsOptions{From: from, To: to, Timezone: matchingZone, Location: named}
			namedStamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, namedOpts, proof)
			if err != nil {
				t.Fatalf("named-zone control %q: %v", matchingZone, err)
			}
			localOpts := namedOpts
			localOpts.Location = time.Local
			localStamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, localOpts, proof)
			if err != nil {
				t.Fatalf("legal CLI options (display zone=%q, location=%q): %v", localOpts.Timezone, localOpts.Location.String(), err)
			}
			if localStamp != namedStamp {
				t.Fatalf("matching local and named timezone produced different stamps: %x vs %x", localStamp, namedStamp)
			}
		}

		// The fixed-offset display fallback is used for an unnamed local
		// location. Give it a deterministic location regardless of host TZ.
		local := time.FixedZone("Local", -8*3600)
		q := StatsOptions{From: from, To: to, Timezone: "UTC-08:00", Location: local}
		if st, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, q, proof); err != nil || st == ([32]byte{}) {
			t.Fatalf("fixed-offset local display label rejected: %v", err)
		}

		conflict := q
		conflict.Timezone = "UTC+14:00"
		if stamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, conflict, proof); err == nil || stamp != ([32]byte{}) {
			t.Fatalf("accepted conflicting fixed offset %q for the UTC-08:00 fixture: %x, %v", conflict.Timezone, stamp, err)
		}

		host := StatsOptions{From: from, To: to, Timezone: time.Local.String(), Location: time.Local}
		if st, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, host, proof); err != nil || st == ([32]byte{}) {
			t.Fatalf("time.Local desktop scope rejected: %v", err)
		}

		for _, badZone := range []string{"other", conflictingZone} {
			host.Timezone = badZone
			if _, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, host, proof); err == nil {
				t.Fatalf("conflicting timezone %q with time.Local was accepted", badZone)
			}
		}
		for _, label := range []string{"Local", "UTC", "America/Los_Angeles", "bogus"} {
			q.Location, q.Timezone = nil, label
			if stamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, q, proof); err == nil || stamp != ([32]byte{}) {
				t.Fatalf("accepted %q without the reducer's location: %x, %v", label, stamp, err)
			}
		}

		// The synthetic Local zone switched from -08:00 to +09:00 after
		// this historical report. Its present offset cannot identify this window.
		const tzif = "TZif\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" +
			"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" +
			"\x00\x00\x00\x01\x00\x00\x00\x02\x00\x00\x00\x08" +
			"\x00\x01\x51\x80\x01" + // transition at Unix 86400 to zone 1
			"\xff\xff\x8f\x80\x00\x00\x00\x00\x7e\x90\x00\x04PST\x00JST\x00"
		transitional, err := time.LoadLocationFromTZData("Local", []byte(tzif))
		if err != nil {
			return err
		}
		historical := StatsOptions{From: time.Unix(0, 0), To: time.Unix(3600, 0), Timezone: "UTC+09:00", Location: transitional}
		if stamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, historical, proof); err == nil || stamp != ([32]byte{}) {
			t.Fatalf("accepted offset outside historical report window: %x, %v", stamp, err)
		}
		historical.Timezone = "UTC-08:00"
		if stamp, err := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, historical, proof); err != nil || stamp == ([32]byte{}) {
			t.Fatalf("rejected offset used by historical report window: %x, %v", stamp, err)
		}

		west := StatsOptions{From: from, To: to, Timezone: "Local", Location: local}
		east := west
		east.Location = time.FixedZone("Local", 9*3600)
		westStamp, westErr := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, west, proof)
		eastStamp, eastErr := ReadCJSIAggregateCacheStamp(ctx, tx, 11, 1, east, proof)
		if westErr != nil || eastErr != nil || westStamp == eastStamp {
			t.Fatalf("different Local zone spans must produce distinct valid stamps: west=%x/%v east=%x/%v", westStamp, westErr, eastStamp, eastErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
