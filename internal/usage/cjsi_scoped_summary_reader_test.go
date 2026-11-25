package usage

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

func fixtureScopedRequests(ctx context.Context, tx *sql.Tx, w scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
	var unitPrice string
	if err := tx.QueryRowContext(ctx, `SELECT input_price FROM fixture_prices`).Scan(&unitPrice); err != nil {
		return nil, nil, err
	}
	resolver := &readPriceResolver{current: w.to, byModel: map[string][]readPriceRow{}}
	for client, provider := range map[string]string{"codex": "openai", "claude": "anthropic"} {
		resolver.byModel[provider+"\x00"+readPriceModelKey(client, "M")] = []readPriceRow{{price: modelPrice{Prices: map[string]string{"input": unitPrice, "cache_write_5m": unitPrice}}}}
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.client,r.session,r.model,r.route,r.at_ns,r.tokens,r.cache_create,r.exact
		FROM fixture_requests r JOIN main.cjsi_publication_head h ON h.singleton=1 AND r.view_ticket=h.ticket
		WHERE r.at_ns>=? AND r.at_ns<? AND (?='' OR r.client=?) AND (?='' OR r.session=?)
		ORDER BY r.at_ns,r.id`, w.from.UnixNano(), w.to.UnixNano(), w.client, w.client, w.session, w.session)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	events := []storedEvent{}
	for rows.Next() {
		var e storedEvent
		var at, tokens, cacheCreate int64
		var exact bool
		if err := rows.Scan(&e.Key, &e.Client, &e.SessionID, &e.Model, &e.runProvider, &at, &tokens, &cacheCreate, &exact); err != nil {
			return nil, nil, err
		}
		e.EventAt = time.Unix(0, at).UTC().Format(time.RFC3339Nano)
		e.Tokens = sessionUsageTokens()
		e.Tokens["input_tokens"] = tokens
		e.Tokens["cache_creation_tokens"] = cacheCreate
		if exact {
			e.runID = sql.NullInt64{Int64: 1, Valid: true}
			e.runExact = e.runID
			e.runMultiplier = sql.NullString{String: "2", Valid: true}
		}
		events = append(events, e)
	}
	return events, resolver, rows.Err()
}

func TestCJSIScopedSummaryReader(t *testing.T) {
	ctx := t.Context()
	db, selected := newIntervalFixture(t)
	from := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	intervalFixtureExec(t, db, `CREATE TEMP TABLE fixture_requests(view_ticket INTEGER,id TEXT,client TEXT,session TEXT,model TEXT,route TEXT,at_ns INTEGER,tokens INTEGER,cache_create INTEGER,exact INTEGER);
		CREATE TEMP TABLE fixture_prices(input_price TEXT); INSERT INTO fixture_prices VALUES('0')`)
	add := func(ticket int, id, client, session, model, route string, at time.Time, tokens, cacheCreate int64, exact int) {
		t.Helper()
		intervalFixtureExec(t, db, `INSERT INTO fixture_requests VALUES(?,?,?,?,?,?,?,?,?,?)`, ticket, id, client, session, model, route, at.UnixNano(), tokens, cacheCreate, exact)
	}
	add(9, "m", "codex", "B", "M", "Q", from, 10, 0, 1)
	add(9, "n", "codex", "B", "N", "P", to.Add(-time.Nanosecond), 1, 0, 1)
	add(9, "outside", "codex", "B", "M", "Q", to, 100, 0, 1)
	add(9, "before", "codex", "B", "M", "Q", from.Add(-time.Nanosecond), 100, 0, 1)
	add(9, "other-session", "codex", "A", "M", "Q", from, 50, 0, 1)
	add(9, "claude", "claude", "B", "M", "Q", from, 4, 0, 1)
	// R8-F3: equal-timestamp Claude events inserted in REVERSE key order ("z-unattr"
	// before "a-ttl"), one priced with default cache TTL warning, one unpriced +
	// unattributed. Legacy order (at_ns, id) must emit cache-TTL warning BEFORE
	// unattributed attribution warning, with known base=5, known provider=10,
	// both complete costs nil.
	add(9, "z-unattr", "claude", "order-sess", "U", "", from, 3, 0, 0)
	add(9, "a-ttl", "claude", "order-sess", "M", "Q", from, 2, 3, 1)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE cjsi_publication_head SET ticket=9; UPDATE fixture_prices SET input_price='1000000'`); err != nil {
		t.Fatal(err)
	}
	selected.ticket = 9
	scope := StatsOptions{From: from, To: to, Client: "codex"}
	read := func(q StatsOptions, session string, sel cjsiIntervalSelection) jointUsageSummary {
		t.Helper()
		calls := 0
		got, err := readScopedUsageSummary(ctx, tx, sel, q, session, func(c context.Context, borrowed *sql.Tx, w scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
			calls++
			if borrowed != tx || w != (scopedRequestWindow{from: q.From, to: q.To, client: q.Client, session: session}) {
				t.Fatal("wrong transaction or request window", w)
			}
			return fixtureScopedRequests(c, borrowed, w)
		})
		if err != nil || calls != 1 {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
		return got
	}
	got := read(scope, "B", selected)
	// B has 10 (m) + 1 (n) = 11 request tokens + (4+2 input + 2+1 output) = 9 residual tokens = 20 total!
	if got.totals.Tokens != 20 || got.shareBase != 20 || got.totals.Events != 2 || got.totals.Sessions != 1 ||
		got.summary.Counts["priced"] != 1 || got.summary.Counts["unpriced"] != 1 || got.summary.AttributionReasons[attributionReasonExactRun] != 2 ||
		got.totals.KnownCatalogBaseCost != "10.000000000" || got.totals.KnownProviderCost != "20.000000000" || got.totals.CatalogBaseCost != nil || got.totals.ProviderCost != nil {
		t.Fatalf("got=%+v", got)
	}
	for _, dims := range [][2]string{{"M", ""}, {"", "Q"}, {"M", "Q"}} {
		q := scope
		q.Model, q.Provider = dims[0], dims[1]
		got := read(q, "B", selected)
		if got.totals.Tokens != 10 || got.totals.Events != 1 || got.shareBase != 20 || got.knownTokenShare() != "50.00" || !got.partial ||
			got.totals.KnownProviderCost != "20.000000000" || got.totals.PricingComplete {
			t.Fatalf("%v: %+v", dims, got)
		}
	}

	// R8-F3 verification: equal-timestamp reverse-key-order warnings & priced/unpriced.
	orderScope := scope
	orderScope.Client = "claude"
	gotOrder := read(orderScope, "order-sess", cjsiIntervalSelection{})
	orderEvents, orderResolver, err := fixtureScopedRequests(ctx, tx, scopedRequestWindow{from, to, "claude", "order-sess"})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder, err := summarizeEvents(orderEvents, orderResolver.priceForEvent)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOrder.summary, wantOrder) ||
		gotOrder.totals.KnownCatalogBaseCost != "5.000000000" ||
		gotOrder.totals.KnownProviderCost != "10.000000000" ||
		gotOrder.totals.CatalogBaseCost != nil || gotOrder.totals.ProviderCost != nil ||
		len(gotOrder.summary.Warnings) < 2 ||
		gotOrder.summary.Warnings[0] != "defaulted 5m cache creation TTL" ||
		gotOrder.summary.Warnings[1] != "unattributed attribution" {
		t.Fatalf("R8-F3 warning order or costs mismatch: got=%+v want=%+v", gotOrder.summary, wantOrder)
	}
	// Reversing the input order MUST produce a different warning order (proving the
	// assertion catches order-reversing bugs).
	reversed := slices.Clone(orderEvents)
	slices.Reverse(reversed)
	revGot, err := summarizeJointContributions(reversed, scopedIntervalSum{}, orderScope, "order-sess", orderResolver.priceForEvent)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(revGot.summary.Warnings, gotOrder.summary.Warnings) {
		t.Fatalf("reversed request order did not change warning order: %v", revGot.summary.Warnings)
	}

	// Reject malformed / unavailable inputs cleanly.
	reject := func(sel cjsiIntervalSelection, q StatsOptions, reader readScopedRequests, want error) {
		t.Helper()
		got, err := readScopedUsageSummary(ctx, tx, sel, q, "B", reader)
		if err == nil || want != nil && !errors.Is(err, want) || !reflect.DeepEqual(got, jointUsageSummary{}) {
			t.Fatalf("returned %+v, %v", got, err)
		}
	}
	stale := selected
	stale.ticket = 7
	reject(stale, scope, fixtureScopedRequests, nil)
	unavailable := errors.New("fixture request selection failed")
	reject(selected, scope, func(context.Context, *sql.Tx, scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
		return orderEvents, orderResolver, unavailable
	}, unavailable)
	reject(selected, scope, func(context.Context, *sql.Tx, scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
		return nil, orderResolver, nil
	}, nil)
	if err := tx.Rollback(); err != nil {
		t.Fatal("lost caller transaction", err)
	}
	reject(cjsiIntervalSelection{}, orderScope, fixtureScopedRequests, sql.ErrTxDone)
}

func TestCJSIScopedStatsRequiresToolCalls(t *testing.T) {
	ctx := t.Context()
	db, _ := newIntervalFixture(t)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	from := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	options := StatsOptions{From: from, To: from.Add(time.Hour), Client: "claude", GroupBy: "hour", Metric: "tokens", Location: time.UTC, Activity: true}
	read := func(context.Context, *sql.Tx, scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
		return []storedEvent{}, nil, nil
	}
	for _, reader := range []readScopedToolCalls{nil, func(context.Context, *sql.Tx, scopedRequestWindow) ([]scopedToolCall, error) {
		return nil, nil
	}} {
		if got, partial, err := readScopedStats(ctx, tx, cjsiIntervalSelection{}, options, read, reader); err == nil || partial || !reflect.DeepEqual(got, StatsReport{}) {
			t.Fatalf("unavailable tool reader returned a Stats report: %+v (%v)", got, err)
		}
	}
	readTools := func(_ context.Context, borrowed *sql.Tx, w scopedRequestWindow) ([]scopedToolCall, error) {
		if borrowed != tx || w != (scopedRequestWindow{from: from, to: options.To, client: "claude"}) {
			t.Fatalf("wrong scoped tool view or window: %v", w)
		}
		duration := int64(125)
		return []scopedToolCall{{Client: "claude", SessionID: "s", Model: "M", ToolName: "edit", StartedAt: from.Format(time.RFC3339Nano), Status: "completed", DurationMS: &duration}}, nil
	}
	got, _, err := readScopedStats(ctx, tx, cjsiIntervalSelection{}, options, read, readTools)
	if err != nil || len(got.Models) != 1 || got.Models[0].Activity == nil ||
		got.Models[0].Activity.ToolCalls != 1 || got.Models[0].Activity.TotalDurationMS != 125 {
		t.Fatalf("expected one 125ms tool call: %+v (%v)", got.Models, err)
	}
}
