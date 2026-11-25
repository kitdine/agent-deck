package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func TestCJSIScopedAggregation(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 1, 2, h, m, 0, 0, time.UTC) }
	window := StatsOptions{From: at(9, 0), To: at(11, 0)}

	// Replay a real Task 3 ledger with a 20-token legacy span containing a
	// 10-token native response R. The first fixture keeps model "m" constant
	// across the whole interval (A4b proved whole-interval model); the second
	// switches models mid-interval so TokenInterval.Model is "".
	provedBody := ledgerHead + ledgerTurn +
		residualNotice("09:55:00", residualVector(0, 0, ""), "") +
		residualNotice("10:05:00", residualVector(20, 0, ""), residualVector(20, 0, "")) +
		ledgerNative("r", residualVector(10, 0, ""), `,"thread_token_usage":`+residualVector(10, 0, ""))
	_, vProved := ledgerView(t, ledgerInput(t, provedBody))

	unprovedBody := ledgerHead + ledgerTurn +
		residualNotice("09:55:00", residualVector(0, 0, ""), "") +
		`{"type":"turn_context","payload":{"turn_id":"t2","model":"other"}}` + "\n" +
		residualNotice("10:05:00", residualVector(20, 0, ""), residualVector(20, 0, "")) +
		ledgerNative("r", residualVector(10, 0, ""), `,"thread_token_usage":`+residualVector(10, 0, ""))
	_, vUnproved := ledgerView(t, ledgerInput(t, unprovedBody))

	extract := func(v ledger.Eligibility) (requestTokens int64, interval ledger.Member) {
		t.Helper()
		intervals := 0
		for _, m := range v.Members {
			if m.Key.Kind == ledger.TokenInterval {
				interval = m
				intervals++
			} else {
				requestTokens += m.Tokens.Input + m.Tokens.Output
			}
		}
		if intervals != 1 || requestTokens != 10 || interval.Tokens.Input != 10 {
			t.Fatalf("unexpected replayed members: req=%d intervals=%d interval=%+v", requestTokens, intervals, interval)
		}
		return requestTokens, interval
	}
	reqProved, intProved := extract(vProved)
	reqUnproved, intUnproved := extract(vUnproved)
	if intProved.Model != "m" || intUnproved.Model != "" {
		t.Fatalf("expected proved model 'm' and unproved '', got %q and %q", intProved.Model, intUnproved.Model)
	}

	reduce := func(rows []ledger.Member, scope StatsOptions, session string) scopedIntervalSum {
		t.Helper()
		got, err := reduceScopedIntervals(rows, scope, session)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("R1-F1 disjoint client short-circuits before nil members while rejecting bad window", func(t *testing.T) {
		claudeScope := StatsOptions{From: at(9, 0), To: at(11, 0), Client: "claude"}
		got, err := reduceScopedIntervals(nil, claudeScope, "")
		if err != nil || !reflect.DeepEqual(got, scopedIntervalSum{}) {
			t.Fatalf("claude with nil members: got=%+v err=%v", got, err)
		}
		badWindow := StatsOptions{From: at(11, 0), To: at(11, 0), Client: "claude"}
		if _, err := reduceScopedIntervals(nil, badWindow, ""); err == nil {
			t.Fatal("accepted empty window for claude")
		}
		for _, client := range []string{"", "codex"} {
			q := window
			q.Client = client
			if _, err := reduceScopedIntervals(nil, q, ""); err == nil {
				t.Fatalf("accepted nil members for client %q", client)
			}
		}
	})

	t.Run("A4b unproved model vs whole-interval proved model and provider filter", func(t *testing.T) {
		// Unproved model: unfiltered All & Codex allocate 10; M, Q, M+Q, unknown
		// all refuse allocation, set partial=true, and keep shareBase=10 (50%).
		for _, client := range []string{"", "codex"} {
			scope := window
			scope.Client = client
			got := reduce(append(append([]ledger.Member{}, vUnproved.Members...), intUnproved), scope, ledgerThread)
			if got.counts != (ledger.Counts{Input: 10}) || got.partial || got.shareBase != 10 ||
				got.allocated != 1 || len(got.sessions) != 1 || reqUnproved+got.shareBase != 20 {
				t.Fatalf("unproved unfiltered %q: %+v", client, got)
			}
		}
		for _, dims := range [][2]string{{"m", ""}, {"", "Q"}, {"m", "Q"}, {"", "unknown"}} {
			scope := window
			scope.Model, scope.Provider = dims[0], dims[1]
			got := reduce(vUnproved.Members, scope, "")
			if !got.partial || got.allocated != 0 || len(got.sessions) != 0 || got.counts != (ledger.Counts{}) ||
				got.shareBase != 10 || reqUnproved*100/(reqUnproved+got.shareBase) != 50 {
				t.Fatalf("unproved dims %v: %+v", dims, got)
			}
		}

		// Whole-interval proved local model "m" (Task 3 TokenInterval.Model):
		// --model m allocates the 10-token residual (M=20 total with R=10);
		// --model other proves disjoint (0 tokens, partial=false);
		// --provider Q or --model m --provider Q still refuse allocation and mark partial=true;
		// --model other --provider Q is proved disjoint via model!
		mScope := window
		mScope.Model = "m"
		gotM := reduce(vProved.Members, mScope, "")
		if gotM.partial || gotM.allocated != 1 || gotM.counts != (ledger.Counts{Input: 10}) ||
			gotM.shareBase != 10 || reqProved+gotM.counts.Input != 20 || len(gotM.sessions) != 1 {
			t.Fatalf("proved model m: %+v", gotM)
		}
		otherModel := window
		otherModel.Model = "other"
		gotOther := reduce(vProved.Members, otherModel, "")
		if gotOther.partial || gotOther.allocated != 0 || gotOther.counts != (ledger.Counts{}) || gotOther.shareBase != 10 {
			t.Fatalf("disjoint proved model other: %+v", gotOther)
		}
		for _, dims := range [][2]string{{"", "Q"}, {"m", "Q"}, {"", "unknown"}} {
			q := window
			q.Model, q.Provider = dims[0], dims[1]
			got := reduce(vProved.Members, q, "")
			if !got.partial || got.allocated != 0 || got.counts != (ledger.Counts{}) || got.shareBase != 10 {
				t.Fatalf("provider filter %v on proved model: %+v", dims, got)
			}
		}
		otherModelQ := window
		otherModelQ.Model, otherModelQ.Provider = "other", "Q"
		if got := reduce(vProved.Members, otherModelQ, ""); got.partial || got.allocated != 0 {
			t.Fatalf("disjoint model+provider should not be partial: %+v", got)
		}

		// Conflicting duplicate interval models must fail closed.
		badDup := intProved
		badDup.Model = "other"
		if _, err := reduceScopedIntervals([]ledger.Member{intProved, badDup}, window, ""); err == nil {
			t.Fatal("accepted conflicting duplicate interval models")
		}
	})

	t.Run("inclusive bounds against half-open buckets", func(t *testing.T) {
		for _, w := range []struct {
			from, to time.Time
			tokens   int64
			partial  bool
		}{
			{at(9, 55), at(10, 5).Add(time.Nanosecond), 10, false},
			{at(9, 55), at(10, 5), 0, true},
			{at(9, 0), at(10, 0), 0, true},
			{at(10, 0), at(11, 0), 0, true},
			{at(10, 5), at(11, 0), 0, true},
			{at(8, 0), at(9, 55), 0, false},
			{at(10, 5).Add(time.Nanosecond), at(11, 0), 0, false},
		} {
			got := reduce(vUnproved.Members, StatsOptions{From: w.from, To: w.to}, "")
			if got.counts.Input != w.tokens || got.partial != w.partial || got.shareBase != w.tokens ||
				(len(got.sessions) != 0) != (w.tokens != 0) {
				t.Fatalf("%v: %+v", w, got)
			}
		}
	})

	row := func(id, thread string, n ledger.Counts) ledger.Member {
		return ledger.Member{
			Key:      ledger.Key{Kind: ledger.TokenInterval, Thread: thread, ID: id},
			Interval: ledger.CounterRange{LowerAt: at(9, 0), UpperAt: at(9, 0)},
			Tokens:   n,
		}
	}

	t.Run("exact int64, source-order independence, known vs missing details, explicit zero vs empty", func(t *testing.T) {
		a := row("a", "A", ledger.Counts{Input: 9007199254740993, Output: 3, Cached: 5, Reasoning: 2, Known: intervalDetails})
		b := row("b", "A", ledger.Counts{Input: 2, Output: 1})
		z := row("z", "B", ledger.Counts{Known: intervalDetails})

		ref := reduce([]ledger.Member{a, b, z}, window, "")
		if ref.counts != (ledger.Counts{Input: 9007199254740995, Output: 4, Cached: 5, Reasoning: 2, Known: intervalDetails}) ||
			ref.missing != intervalDetails || ref.shareBase != 9007199254740999 ||
			!reflect.DeepEqual(ref.sessions, map[SessionKey]struct{}{{"codex", "A"}: {}, {"codex", "B"}: {}}) {
			t.Fatalf("unexpected exact sum: %+v", ref)
		}
		for _, perm := range [][]ledger.Member{{a, z, b}, {b, a, z}, {b, z, a}, {z, a, b}, {z, b, a}} {
			if got := reduce(perm, window, ""); !reflect.DeepEqual(got, ref) {
				t.Fatalf("order-dependent reduction: got=%+v want=%+v", got, ref)
			}
		}
		zero := reduce([]ledger.Member{z}, window, "B")
		empty := reduce([]ledger.Member{}, window, "")
		if zero.counts != z.Tokens || zero.missing != 0 || zero.allocated != 1 || len(zero.sessions) != 1 ||
			!reflect.DeepEqual(empty, scopedIntervalSum{}) {
			t.Fatalf("explicit zero=%+v empty read=%+v", zero, empty)
		}
	})

	t.Run("overflow and malformed residuals fail closed", func(t *testing.T) {
		large := row("max", "A", ledger.Counts{Input: math.MaxInt64, Write: math.MaxInt64, Known: ledger.WriteKnown})
		if got := reduce([]ledger.Member{large}, window, ""); got.shareBase != math.MaxInt64 || got.counts != large.Tokens {
			t.Fatal(got)
		}
		reject := func(label string, rows []ledger.Member, scope StatsOptions) {
			t.Helper()
			if got, err := reduceScopedIntervals(rows, scope, ""); err == nil || !reflect.DeepEqual(got, scopedIntervalSum{}) {
				t.Fatalf("%s: %+v err=%v", label, got, err)
			}
		}
		for _, n := range []ledger.Counts{
			{Input: -1}, {Output: -1}, {Input: math.MaxInt64, Output: 1},
			{Input: 1, Cached: 2, Known: ledger.CachedKnown},
			{Input: 1, Write: 2, Known: ledger.WriteKnown},
			{Output: 1, Reasoning: 2, Known: ledger.ReasoningKnown},
			{Input: 2, Cached: 1}, {Write: 1}, {Output: 2, Reasoning: 1}, {Known: 128},
			{Write: -1, Known: ledger.WriteKnown},
		} {
			reject("malformed residual", []ledger.Member{row("bad", "B", n)}, window)
		}
		reject("sum overflow", []ledger.Member{large, row("one", "A", ledger.Counts{Output: 1})}, window)
		reject("write overflow", []ledger.Member{large, row("one", "A", ledger.Counts{Write: 1, Known: ledger.WriteKnown})}, window)
	})

	t.Run("A4b full Stats, Presentation, SessionUsage, and Signals isolation", func(t *testing.T) {
		ctx := t.Context()
		db, err := store.Open(ctx, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// Seed price catalog: M input=1, multiplier=2 (so 10 tokens => base=10, provider=20).
		// Also seed a Claude event C=5 (base=5, provider=5) to prove disjoint client stays complete.
		for _, q := range []string{
			`INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('cat-1','bundled','bundled://fixture','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1)`,
			`INSERT INTO model_prices(catalog_version,provider,model,effective_from,prices_json,aliases_json) VALUES
				('cat-1','openai','m','2026-01-01T00:00:00Z','{"input":"1000000"}','[]'),
				('cat-1','anthropic','claude-x','2026-01-01T00:00:00Z','{"input":"1000000"}','[]')`,
			`INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact) VALUES
				(1,'codex','Q','2','2026-01-02T09:00:00Z','2026-01-02T11:00:00Z',1),
				(2,'claude','Y','1','2026-01-02T09:00:00Z','2026-01-02T11:00:00Z',1)`,
			`INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
				('codex','` + ledgerThread + `','2026-01-02T09:55:00Z','2026-01-02T10:05:00Z'),
				('claude','cl-1','2026-01-02T09:30:00Z','2026-01-02T09:30:00Z')`,
			`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset,run_id) VALUES
				('codex:` + ledgerThread + `:r','codex','` + ledgerThread + `','r','2026-01-02T10:00:00Z','m',10,0,'f',0,1),
				('claude:cl-1:c1','claude','cl-1','c1','2026-01-02T09:30:00Z','claude-x',5,0,'g',0,2)`,
		} {
			if _, err := db.DB.ExecContext(ctx, q); err != nil {
				t.Fatal(err)
			}
		}

		svc := New(db, t.TempDir())
		now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
		svc.Now = func() time.Time { return now }

		// Install CJSI head/selection/interval rows for the replayed unproved member
		// inside the same SQLite store so every scoped reader operates on one tx.
		installIntervalTables(t, db.DB, "cjsi_index_schema.go", "cjsi_publication_head")
		installIntervalTables(t, db.DB, "cjsi_core_facts_schema.go", "cjsi_publication_attempts", "cjsi_usage_groups", "cjsi_usage_selection", "cjsi_usage_members", "cjsi_usage_intervals")
		seedIntervalSelection := func(coreEpoch, ticket int64, m ledger.Member) cjsiIntervalSelection {
			t.Helper()
			b := ledger.Basis{CoreEpoch: coreEpoch, Ticket: ticket, Attempt: 1, BaseTicket: ticket - 1, InputDigest: [32]byte{byte(ticket), 1}}
			diff := [32]byte{byte(ticket), 2}
			attemptDiff := []byte("aggregate-attempt-digest-32bytes")
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_publication_head(singleton,core_epoch,ticket) VALUES(1,?,?)`, coreEpoch, ticket); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,
				session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,
				input_digest,state,accept_until,diff_digest) VALUES(?,1,?,?,0,0,0,0,0,0,0,0,?,'opened','2027-01-01T00:00:00Z',?)`,
				ticket, b.BaseTicket, coreEpoch, b.InputDigest[:], attemptDiff); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_usage_groups VALUES(?,1,?,?,?,1,64,0,1,0)`,
				ticket, m.Key.Thread, b.InputDigest[:], diff[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_usage_selection VALUES(?,?,1)`, m.Key.Thread, ticket); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_usage_members VALUES(?,1,?,3,?,'',?,NULL,NULL,?,?,NULL,NULL,NULL)`,
				ticket, m.Key.Thread, m.Key.ID, m.Model, m.Tokens.Input, m.Tokens.Output); err != nil {
				t.Fatal(err)
			}
			epoch := m.Interval.Epoch
			if epoch == ([32]byte{}) {
				epoch = [32]byte{9}
			}
			if _, err := db.DB.ExecContext(ctx, `INSERT OR REPLACE INTO cjsi_usage_intervals(ticket,attempt,thread,kind,member_id,counter_epoch,lower_at,upper_at,lower_input,upper_input,lower_output,upper_output,lower_cached,upper_cached,lower_write,upper_write,lower_reasoning,upper_reasoning) VALUES(?,1,?,3,?,?,?,?,?,?,?,?,NULL,NULL,NULL,NULL,NULL,NULL)`,
				ticket, m.Key.Thread, m.Key.ID, epoch[:], m.Interval.LowerAt.UTC().Format(time.RFC3339Nano), m.Interval.UpperAt.UTC().Format(time.RFC3339Nano),
				m.Interval.Lower.Input, m.Interval.Upper.Input, m.Interval.Lower.Output, m.Interval.Upper.Output); err != nil {
				t.Fatal(err)
			}
			return cjsiIntervalSelection{
				coreEpoch: coreEpoch,
				ticket:    ticket,
				groups:    []cjsiIntervalGroup{{thread: m.Key.Thread, stamp: ledger.ProposalStamp{Basis: b, Diff: diff}}},
			}
		}

		readRequestsFromTx := func(c context.Context, tx *sql.Tx, w scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
			query := `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,COALESCE(e.turn_index,0),
				e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,
				e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.cache_write_tokens,e.source_path,e.source_offset,
				COALESCE(b.run_id,e.run_id),r.exact,r.multiplier,r.provider,r.started_at,us.first_at,us.started_at,us.last_at
				FROM usage_events e
				LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key
				LEFT JOIN usage_runs r ON r.id=COALESCE(b.run_id,e.run_id)
				LEFT JOIN usage_sessions us ON us.client=e.client AND us.session_id=e.session_id
				WHERE e.event_at>=? AND e.event_at<?`
			args := []any{w.from.UTC().Format(time.RFC3339Nano), w.to.UTC().Format(time.RFC3339Nano)}
			if w.client != "" {
				query += ` AND e.client=?`
				args = append(args, w.client)
			}
			if w.session != "" {
				query += ` AND e.session_id=?`
				args = append(args, w.session)
			}
			query += ` ORDER BY e.event_at,e.event_key`
			rows, err := tx.QueryContext(c, query, args...)
			if err != nil {
				return nil, nil, err
			}
			defer rows.Close()
			out := []storedEvent{}
			for rows.Next() {
				var e storedEvent
				var in, cached, outTok, readTok, createTok, w5, w1, wTok int64
				if err := rows.Scan(&e.Key, &e.Client, &e.SessionID, &e.EventID, &e.EventAt, &e.Model, &e.TurnIndex,
					&in, &cached, &outTok, &readTok, &createTok, &w5, &w1, &wTok, &e.SourcePath, &e.SourceOffset,
					&e.runID, &e.runExact, &e.runMultiplier, &e.runProvider, &e.runStart, &e.sessionStart, &e.processStart, &e.sessionEnd); err != nil {
					return nil, nil, err
				}
				e.Tokens = map[string]int64{
					"input_tokens": in, "cached_input_tokens": cached, "output_tokens": outTok,
					"cache_read_tokens": readTok, "cache_creation_tokens": createTok,
					"cache_write_5m_tokens": w5, "cache_write_1h_tokens": w1, "cache_write_tokens": wTok,
				}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return nil, nil, err
			}
			resolver := &readPriceResolver{current: now, byModel: map[string][]readPriceRow{
				"openai\x00" + readPriceModelKey("codex", "m"):            {{price: modelPrice{Provider: "openai", Prices: map[string]string{"input": "1000000"}}}},
				"anthropic\x00" + readPriceModelKey("claude", "claude-x"): {{price: modelPrice{Provider: "anthropic", Prices: map[string]string{"input": "1000000"}}}},
			}}
			return out, resolver, nil
		}

		selectedUnproved := seedIntervalSelection(11, 1, intUnproved)
		tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		// 1. SessionUsageSummary includes residual (20), while SessionInvocations stays 1 event (10).
		sessSummary, err := readScopedSessionUsage(ctx, tx, selectedUnproved, "codex", ledgerThread, readRequestsFromTx)
		if err != nil {
			t.Fatal(err)
		}
		if sessSummary.Tokens["input_tokens"] != 20 || sessSummary.CatalogBaseCost != nil || sessSummary.ProviderCost != nil ||
			sessSummary.KnownCatalogBaseCost == nil || *sessSummary.KnownCatalogBaseCost != "10.000000000" ||
			sessSummary.KnownProviderCost == nil || *sessSummary.KnownProviderCost != "20.000000000" ||
			sessSummary.FirstAt != "2026-01-02T10:00:00Z" || sessSummary.LastAt != "2026-01-02T10:00:00Z" {
			t.Fatalf("unexpected session summary: %+v", sessSummary)
		}
		invocations, page, err := svc.SessionInvocations(ctx, "codex", ledgerThread, 1, 10, false)
		if err != nil || page.Total != 1 || len(invocations) != 1 || invocations[0].Tokens["input_tokens"] != 10 ||
			invocations[0].CatalogBaseCost == nil || *invocations[0].CatalogBaseCost != "10.000000000" ||
			invocations[0].ProviderCost == nil || *invocations[0].ProviderCost != "20.000000000" {
			t.Fatalf("invocations=%+v page=%+v err=%v", invocations, page, err)
		}

		// 2. Scoped Stats (Codex only, day vs hour buckets, unproved vs proved model).
		dayOpts := StatsOptions{
			From: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
			Client: "codex", GroupBy: "day", Metric: "tokens", Location: time.UTC, Timezone: "UTC",
		}
		events, err := svc.eventsRange(ctx, dayOpts.From, dayOpts.To, "codex", "")
		if err != nil {
			t.Fatal(err)
		}
		resolver, err := svc.loadReadPriceResolver(ctx, now)
		if err != nil {
			t.Fatal(err)
		}

		// Unproved model on day bucket: Totals=20, Events=1, Sessions=1, M=10 (50%), Q=10 (50%), no unknown provider row!
		statsUnproved, partial, err := reduceScopedStatsWithToolCalls(events, []scopedToolCall{}, vUnproved.Members, dayOpts, resolver.priceForEvent)
		if err != nil || !partial {
			t.Fatalf("stats unproved err=%v partial=%v", err, partial)
		}
		if statsUnproved.Totals.Tokens != 20 || statsUnproved.Totals.Events != 1 || statsUnproved.Totals.Sessions != 1 ||
			statsUnproved.Totals.CatalogBaseCost != nil || statsUnproved.Totals.ProviderCost != nil ||
			statsUnproved.Totals.KnownCatalogBaseCost != "10.000000000" || statsUnproved.Totals.KnownProviderCost != "20.000000000" ||
			statsUnproved.Coverage.PricedEvents != 1 || statsUnproved.Coverage.TotalEvents != 1 || statsUnproved.Coverage.Percent != "100.00" {
			t.Fatalf("unexpected stats totals/coverage: %+v / %+v", statsUnproved.Totals, statsUnproved.Coverage)
		}
		if len(statsUnproved.Models) != 1 || statsUnproved.Models[0].Name != "m" || statsUnproved.Models[0].Tokens != 10 ||
			statsUnproved.Models[0].KnownShare != "50.00" || statsUnproved.Models[0].ProviderCost != nil {
			t.Fatalf("unexpected unproved models: %+v", statsUnproved.Models)
		}
		if len(statsUnproved.Providers) != 1 || statsUnproved.Providers[0].Name != "Q" || statsUnproved.Providers[0].Tokens != 10 ||
			statsUnproved.Providers[0].KnownShare != "50.00" || statsUnproved.Providers[0].ProviderCost != nil {
			t.Fatalf("unexpected unproved providers: %+v", statsUnproved.Providers)
		}
		if len(statsUnproved.Buckets) != 1 || statsUnproved.Buckets[0].Tokens != 20 || statsUnproved.Buckets[0].ProviderCost != nil {
			t.Fatalf("day bucket should contain full 20: %+v", statsUnproved.Buckets)
		}

		// Hourly buckets: interval spans 09:55..10:05 so neither hour 9 nor hour 10 gets the 10-token residual!
		hourOpts := dayOpts
		hourOpts.GroupBy = "hour"
		statsHourly, _, err := reduceScopedStatsWithToolCalls(events, []scopedToolCall{}, vUnproved.Members, hourOpts, resolver.priceForEvent)
		if err != nil {
			t.Fatal(err)
		}
		if statsHourly.Buckets[9].Tokens != 0 || statsHourly.Buckets[9].ProviderCost != nil ||
			statsHourly.Buckets[10].Tokens != 10 || statsHourly.Buckets[10].ProviderCost != nil ||
			statsHourly.Buckets[11].Tokens != 0 || statsHourly.Buckets[11].ProviderCost == nil {
			t.Fatalf("hourly buckets violated non-splitting or disjoint completeness: h9=%+v h10=%+v h11=%+v",
				statsHourly.Buckets[9], statsHourly.Buckets[10], statsHourly.Buckets[11])
		}

		// Proved whole-interval model "m": M=20 (100% share), Q still 10 (50%), complete cost still nil!
		statsProved, partialProved, err := reduceScopedStatsWithToolCalls(events, []scopedToolCall{}, vProved.Members, dayOpts, resolver.priceForEvent)
		if err != nil || !partialProved {
			t.Fatalf("stats proved err=%v partial=%v", err, partialProved)
		}
		if len(statsProved.Models) != 1 || statsProved.Models[0].Name != "m" || statsProved.Models[0].Tokens != 20 ||
			statsProved.Models[0].Events != 1 || statsProved.Models[0].KnownShare != "100.00" || statsProved.Models[0].ProviderCost != nil {
			t.Fatalf("unexpected proved model row: %+v", statsProved.Models)
		}
		if len(statsProved.Providers) != 1 || statsProved.Providers[0].Name != "Q" || statsProved.Providers[0].Tokens != 10 ||
			statsProved.Providers[0].KnownShare != "50.00" || statsProved.Providers[0].ProviderCost != nil {
			t.Fatalf("unexpected proved provider row: %+v", statsProved.Providers)
		}

		// 3. Presentation with intervals via same-view pinned tx: All=25 (20 Codex + 5 Claude), Codex=20 (PricingComplete=false),
		// Claude=5 (PricingComplete=true, CostIncomplete=false), hourly does not split 09:55..10:05.
		pres, err := readScopedPresentation(ctx, tx, selectedUnproved, now, time.UTC, false, readRequestsFromTx)
		if err != nil {
			t.Fatal(err)
		}
		if pres.Summary.Tokens["input_tokens"] != 25 || pres.Summary.Counts["events"] != 2 ||
			pres.Summary.CatalogBaseCost != nil || pres.Summary.ProviderCost != nil ||
			pres.Summary.KnownCatalogBaseCost == nil || *pres.Summary.KnownCatalogBaseCost != "15.000000000" ||
			pres.Summary.KnownProviderCost == nil || *pres.Summary.KnownProviderCost != "25.000000000" {
			t.Fatalf("unexpected presentation summary: %+v", pres.Summary)
		}
		var codexScope, claudeScope PresentationScope
		for _, sc := range pres.Scopes {
			if sc.Client == "codex" {
				codexScope = sc
			} else if sc.Client == "claude" {
				claudeScope = sc
			}
		}
		if codexScope.Periods.Items[0].Totals.Tokens != 20 || codexScope.Periods.Items[0].Totals.Events != 1 ||
			codexScope.Periods.Items[0].Totals.PricingComplete || codexScope.Periods.Items[0].Totals.KnownProviderCost != "20.000000000" {
			t.Fatalf("codex today totals: %+v", codexScope.Periods.Items[0].Totals)
		}
		if codexScope.Hourly.Items[9].Value.Tokens != 0 || !codexScope.Hourly.Items[9].Value.CostIncomplete ||
			codexScope.Hourly.Items[10].Value.Tokens != 10 || !codexScope.Hourly.Items[10].Value.CostIncomplete ||
			codexScope.Hourly.Items[11].Value.CostIncomplete {
			t.Fatalf("codex hourly items: h9=%+v h10=%+v h11=%+v", codexScope.Hourly.Items[9], codexScope.Hourly.Items[10], codexScope.Hourly.Items[11])
		}
		if claudeScope.Periods.Items[0].Totals.Tokens != 5 || !claudeScope.Periods.Items[0].Totals.PricingComplete ||
			claudeScope.Periods.Items[0].Totals.ProviderCost == nil || *claudeScope.Periods.Items[0].Totals.ProviderCost != "5.000000000" {
			t.Fatalf("claude scope was polluted by codex interval: %+v", claudeScope.Periods.Items[0].Totals)
		}

		// 4. Signals remain strictly request/work-fact based (zero fabricated signals).
		sigReports, err := svc.SignalsBatch(ctx, []SignalOptions{{From: dayOpts.From, To: dayOpts.To, IncludeSub: true}})
		if err != nil || len(sigReports) != 1 || sigReports[0].Activity.Available || sigReports[0].Tooling.Calls != 0 {
			t.Fatalf("signals fabricated activity from intervals: %+v err=%v", sigReports, err)
		}
	})

	t.Run("R9-F1 stats buckets clip to query window", func(t *testing.T) {
		day := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		mkInterval := func(id string, lo, hi time.Time) ledger.Member {
			return ledger.Member{
				Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "thread", ID: id},
				Tokens: ledger.Counts{Input: 10}, Model: "M",
				Interval: ledger.CounterRange{
					Epoch: [32]byte{1}, Lower: ledger.Counts{Input: 10}, Upper: ledger.Counts{Input: 20},
					LowerAt: lo, UpperAt: hi,
				},
			}
		}
		for _, c := range []struct {
			name    string
			lo, hi  time.Time
			tokens  int64
			sess    int64
			partial bool
		}{
			{"outside-before", day.Add(-3 * time.Hour), day.Add(-time.Hour), 0, 0, false},
			{"outside-after", day.Add(25 * time.Hour), day.Add(26 * time.Hour), 0, 0, false},
			{"crosses-from", day.Add(-time.Hour), day.Add(time.Hour), 0, 0, true},
			{"touches-exclusive-to", day.Add(23 * time.Hour), day.Add(24 * time.Hour), 0, 0, true},
			{"inside-query", day.Add(time.Hour), day.Add(2 * time.Hour), 10, 1, true},
		} {
			t.Run(c.name, func(t *testing.T) {
				report, partial, err := reduceScopedStatsWithToolCalls([]storedEvent{}, []scopedToolCall{}, []ledger.Member{mkInterval("one", c.lo, c.hi)}, StatsOptions{
					From: day, To: day.Add(24 * time.Hour), GroupBy: "week", Metric: "tokens", Location: time.UTC,
				}, nil)
				if err != nil || len(report.Buckets) != 1 {
					t.Fatalf("expected one week bucket, got %d err=%v", len(report.Buckets), err)
				}
				bucket := report.Buckets[0]
				if report.Totals.Tokens != c.tokens || bucket.Tokens != c.tokens || bucket.Sessions != c.sess ||
					partial != c.partial || (bucket.ProviderCost == nil) != c.partial {
					t.Fatalf("totals=%d bucket=%d sess=%d partial=%v costIncomplete=%v; want %d/%d/%d/%v/%v",
						report.Totals.Tokens, bucket.Tokens, bucket.Sessions, partial, bucket.ProviderCost == nil,
						c.tokens, c.tokens, c.sess, c.partial, c.partial)
				}
			})
		}
	})

	t.Run("R9-F2 R10-F1 pinned transaction isolates presentation, session usage and WorkSignals", func(t *testing.T) {
		ctx := t.Context()
		dir := t.TempDir()
		core, err := store.Open(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		// Open a separate writer connection to the same WAL SQLite file so we can
		// commit concurrent writes while a read transaction holds its snapshot.
		writerDB, err := sql.Open("sqlite", filepath.Join(dir, "agentdeck.sqlite3")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			t.Fatal(err)
		}
		defer writerDB.Close()

		day := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		now := day.Add(12 * time.Hour)
		installIntervalTables(t, core.DB, "cjsi_index_schema.go", "cjsi_publication_head")
		installIntervalTables(t, core.DB, "cjsi_core_facts_schema.go", "cjsi_publication_attempts", "cjsi_usage_groups", "cjsi_usage_selection", "cjsi_usage_members", "cjsi_usage_intervals")
		b := ledger.Basis{CoreEpoch: 11, Ticket: 1, Attempt: 1, BaseTicket: 0, InputDigest: [32]byte{1, 1}}
		diff := [32]byte{1, 2}
		attemptDiff := []byte("aggregate-attempt-digest-32bytes")
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset,turn_index) VALUES('old','codex','th','old','2026-01-07T01:15:00Z','M',10,0,'fixture',0,1)`, nil},
			{`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,activity_kind,activity_sub,source_path) VALUES('codex','th',1,'2026-01-07T01:00:00Z','classified','build','coding','feature','fixture')`, nil},
			{`INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset,turn_index,tool_kind) VALUES('old-edit','codex','th','M','apply_patch','2026-01-07T01:05:00Z','completed','fixture',1,1,'edit')`, nil},
			{`INSERT INTO usage_tool_files VALUES('old-edit','old-digest','old.go',1)`, nil},
			{`CREATE TABLE fixture_signal_price (unit_price TEXT NOT NULL); INSERT INTO fixture_signal_price VALUES('1000000')`, nil},
			{`INSERT INTO cjsi_publication_head(singleton,core_epoch,ticket) VALUES(1,11,1)`, nil},
			{`INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,state,accept_until,diff_digest) VALUES(1,1,0,11,0,0,0,0,0,0,0,0,?,'opened','2027-01-01T00:00:00Z',?)`, []any{b.InputDigest[:], attemptDiff}},
			{`INSERT INTO cjsi_usage_groups VALUES(1,1,'th',?,?,1,64,0,1,0)`, []any{b.InputDigest[:], diff[:]}},
			{`INSERT INTO cjsi_usage_selection VALUES('th',1,1)`, nil},
			{`INSERT INTO cjsi_usage_members VALUES(1,1,'th',3,'iv','','',NULL,NULL,10,0,NULL,NULL,NULL)`, nil},
			{`INSERT INTO cjsi_usage_intervals(ticket,attempt,thread,kind,member_id,counter_epoch,lower_at,upper_at,lower_input,upper_input,lower_output,upper_output,lower_cached,upper_cached,lower_write,upper_write,lower_reasoning,upper_reasoning) VALUES(1,1,'th',3,'iv',?,?,?,10,20,0,0,NULL,NULL,NULL,NULL,NULL,NULL)`, []any{[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, day.Add(time.Hour).Format(time.RFC3339Nano), day.Add(2 * time.Hour).Format(time.RFC3339Nano)}},
		} {
			if _, err := core.DB.ExecContext(ctx, q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
		selected := cjsiIntervalSelection{
			coreEpoch: 11, ticket: 1,
			groups: []cjsiIntervalGroup{{thread: "th", stamp: ledger.ProposalStamp{Basis: b, Diff: diff}}},
		}
		readTxRequests := func(c context.Context, tx *sql.Tx, w scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
			query := `SELECT event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,turn_index FROM usage_events WHERE event_at>=? AND event_at<?`
			args := []any{w.from.UTC().Format(time.RFC3339Nano), w.to.UTC().Format(time.RFC3339Nano)}
			if w.client != "" {
				query += ` AND client=?`
				args = append(args, w.client)
			}
			if w.session != "" {
				query += ` AND session_id=?`
				args = append(args, w.session)
			}
			query += ` ORDER BY event_at,event_key`
			rows, err := tx.QueryContext(c, query, args...)
			if err != nil {
				return nil, nil, err
			}
			defer rows.Close()
			out := []storedEvent{}
			for rows.Next() {
				var e storedEvent
				var inTok, outTok int64
				if err := rows.Scan(&e.Key, &e.Client, &e.SessionID, &e.EventID, &e.EventAt, &e.Model, &inTok, &outTok, &e.TurnIndex); err != nil {
					return nil, nil, err
				}
				e.Tokens = map[string]int64{"input_tokens": inTok, "output_tokens": outTok}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return nil, nil, err
			}
			if err := rows.Close(); err != nil {
				return nil, nil, err
			}
			var unitPrice string
			if err := tx.QueryRowContext(c, `SELECT unit_price FROM fixture_signal_price`).Scan(&unitPrice); err != nil {
				return nil, nil, err
			}
			resolver := &readPriceResolver{current: now, byModel: map[string][]readPriceRow{
				"openai\x00" + readPriceModelKey("codex", "M"): {{price: modelPrice{Provider: "openai", Prices: map[string]string{"input": unitPrice}}}},
			}}
			return out, resolver, nil
		}

		// Pin the read snapshot before the concurrent writer commits.
		pinnedTx, err := core.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer pinnedTx.Rollback()
		var pinnedTicket int64
		if err := pinnedTx.QueryRowContext(ctx, `SELECT ticket FROM cjsi_publication_head WHERE singleton=1`).Scan(&pinnedTicket); err != nil || pinnedTicket != 1 {
			t.Fatalf("failed to pin read snapshot: ticket=%d err=%v", pinnedTicket, err)
		}

		// Commit the newer request, classification, tool, written-file and
		// pricing inputs together through the second connection.
		writer, err := writerDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback()
		for _, query := range []string{
			`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset,turn_index) VALUES('new','codex','th','new','2026-01-07T01:20:00Z','M',100,0,'fixture',0,2)`,
			`UPDATE usage_work_signals SET started_at='2026-01-07T00:30:00Z', activity_kind='debugging', activity_sub='repair' WHERE turn_index=1`,
			`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,activity_kind,activity_sub,source_path) VALUES('codex','th',2,'2026-01-07T01:16:00Z','classified','build','debugging','repair','fixture')`,
			`INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset,turn_index,tool_kind) VALUES('new-edit','codex','th','M','apply_patch','2026-01-07T01:22:00Z','completed','fixture',2,2,'edit')`,
			`INSERT INTO usage_tool_files VALUES('new-edit','new-digest','new.go',1)`,
			`UPDATE fixture_signal_price SET unit_price='2000000'`,
			`UPDATE cjsi_publication_head SET ticket=2 WHERE singleton=1`,
		} {
			if _, err := writer.ExecContext(ctx, query); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatal(err)
		}

		// Pinned snapshot sees strictly ticket 1: 10 (old request) + 10 (interval) = 20, NEVER 120.
		pres, err := readScopedPresentation(ctx, pinnedTx, selected, now, time.UTC, true, readTxRequests)
		if err != nil {
			t.Fatalf("pinned readScopedPresentation failed: %v", err)
		}
		if pres.Summary.Tokens["input_tokens"] != 20 || pres.Summary.Counts["events"] != 1 {
			t.Fatalf("presentation mixed admitted intervals with newer request rows: input=%d events=%d", pres.Summary.Tokens["input_tokens"], pres.Summary.Counts["events"])
		}
		ss, err := readScopedSessionUsage(ctx, pinnedTx, selected, "codex", "th", readTxRequests)
		if err != nil {
			t.Fatalf("pinned readScopedSessionUsage failed: %v", err)
		}
		if ss.Tokens["input_tokens"] != 20 {
			t.Fatalf("session summary mixed admitted intervals with newer request rows: input=%d", ss.Tokens["input_tokens"])
		}

		all := SignalOptions{Period: "today", From: day, To: day.AddDate(0, 0, 1)}
		codex := all
		codex.Client = "codex"
		checkSignals := func(tx *sql.Tx, options []SignalOptions, kind string, calls int64, firstEdit int, amount float64) {
			t.Helper()
			reports, err := readScopedSignalsBatch(ctx, tx, options, readTxRequests)
			if err != nil || len(reports) != len(options) {
				t.Fatalf("work signals: %v (%d reports)", err, len(reports))
			}
			for _, got := range reports {
				if got.Activity == nil || got.Activity.CostBasis != CostBasisTurn || got.Workflow == nil ||
					got.Workflow.FilesTouched == nil || *got.Workflow.FilesTouched != int(calls) ||
					got.Workflow.FirstEditSeconds == nil || *got.Workflow.FirstEditSeconds != firstEdit ||
					got.Tooling == nil || got.Tooling.Calls != calls {
					t.Fatalf("mixed signal families: activity=%+v workflow=%+v tooling=%+v", got.Activity, got.Workflow, got.Tooling)
				}
				found := false
				for _, activity := range got.Activity.Kinds {
					found = found || activity.Kind == kind && activity.Events == calls && activity.Cost == amount
				}
				if !found {
					t.Fatalf("wrong activity category, calls or cost: %+v", got.Activity.Kinds)
				}
			}
		}
		checkSignals(pinnedTx, []SignalOptions{all, codex}, "coding", 1, 300, 10)
		checkSignals(pinnedTx, []SignalOptions{codex}, "coding", 1, 300, 10)
		sessionSignals, ok, err := readScopedSessionSignals(ctx, pinnedTx, "codex", "th", readTxRequests)
		if err != nil || !ok || sessionSignals.Kind != "coding" || sessionSignals.ToolCalls != 1 {
			t.Fatalf("wrong pinned session signals: %+v, %v, %v", sessionSignals, ok, err)
		}
		earliest, err := readEarliestSignalAt(ctx, pinnedTx)
		if err != nil || earliest == nil || !earliest.Equal(day.Add(time.Hour)) {
			t.Fatalf("wrong pinned earliest signal: %v, %v", earliest, err)
		}

		// A fresh transaction after the head moved MUST fail closed when given the
		// stale ticket-1 selection, never mixing ticket-1 intervals with ticket-2 requests.
		afterTx, err := core.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer afterTx.Rollback()
		checkSignals(afterTx, []SignalOptions{all, codex}, "debugging", 2, 2100, 220)
		if _, err := readScopedPresentation(ctx, afterTx, selected, now, time.UTC, true, readTxRequests); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("stale selection in readScopedPresentation did not fail closed: %v", err)
		}
		if _, err := readScopedSessionUsage(ctx, afterTx, selected, "codex", "th", readTxRequests); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("stale selection in readScopedSessionUsage did not fail closed: %v", err)
		}
	})

	t.Run("R9-F4 and R9-F7 stats retains model activity, tool calls, detail command, and deterministic ordering", func(t *testing.T) {
		day := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		events := make([]storedEvent, 0, 12)
		for i := 1; i <= 8; i++ {
			input := int64(10)
			if i == 5 {
				input = 20
			}
			events = append(events, storedEvent{Event: Event{
				Key: fmt.Sprintf("priced-%d", i), Client: "codex", SessionID: fmt.Sprintf("session-%d", i),
				Model: fmt.Sprintf("M%d", i), EventAt: day.Add(time.Hour).Format(time.RFC3339Nano),
				Tokens: map[string]int64{"input_tokens": input, "cached_input_tokens": int64((i % 3) + 1), "output_tokens": 0},
			}})
		}
		events = append(events, storedEvent{Event: Event{
			Key: "claude-cache-tie", Client: "claude", SessionID: "session-8", Model: "M1",
			EventAt: day.Add(time.Hour).Format(time.RFC3339Nano),
			Tokens:  map[string]int64{"input_tokens": 7, "cache_read_tokens": 3, "output_tokens": 0},
		}})
		for _, pair := range [][2]string{{"codex", "U3"}, {"claude", "U2"}, {"codex", "U1"}, {"claude", "U1"}} {
			events = append(events, storedEvent{Event: Event{
				Key: fmt.Sprintf("unpriced-%s-%s", pair[0], pair[1]), Client: pair[0], SessionID: "unpriced-sess",
				Model: pair[1], EventAt: day.Add(2 * time.Hour).Format(time.RFC3339Nano),
				Tokens: map[string]int64{"input_tokens": 5, "output_tokens": 0},
			}})
		}
		dur := int64(250)
		toolCalls := []scopedToolCall{{
			Client: "codex", SessionID: "session-1", Model: "M1", ToolName: "shell",
			StartedAt: day.Add(90 * time.Minute).Format(time.RFC3339Nano), Status: "completed", DurationMS: &dur, Provider: "Q",
		}}
		// Interval on an interval-only model must NOT fabricate model activity.
		intervalOnly := ledger.Member{
			Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "iv-thread", ID: "iv-only"},
			Tokens: ledger.Counts{Input: 7, Cached: 0, Known: ledger.CachedKnown}, Model: "M-interval-only",
			Interval: ledger.CounterRange{
				Epoch: [32]byte{2}, Lower: ledger.Counts{Input: 0}, Upper: ledger.Counts{Input: 7},
				LowerAt: day.Add(3 * time.Hour), UpperAt: day.Add(4 * time.Hour),
			},
		}
		price := func(e storedEvent) (eventAttribution, error) {
			if e.Model[0] == 'U' {
				return eventAttribution{provider: "Q", spendEligible: true, quality: "exact", reason: attributionReasonExactRun}, nil
			}
			var provider string
			if e.Client == "claude" {
				provider = "anthropic"
			} else {
				provider = "openai"
			}
			return eventAttribution{
				price:      modelPrice{Provider: provider, Prices: map[string]string{"input": "1", "cached_input": "1", "cache_read": "1"}},
				multiplier: "1", provider: "Q", spendEligible: true, quality: "exact", reason: attributionReasonExactRun,
			}, nil
		}
		options := StatsOptions{From: day, To: day.Add(24 * time.Hour), GroupBy: "day", Metric: "tokens", Location: time.UTC, Activity: true}
		var firstJSON []byte
		for i := 0; i < 64; i++ {
			report, _, err := reduceScopedStatsWithToolCalls(events, toolCalls, []ledger.Member{intervalOnly}, options, price)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				firstJSON = raw
				var sawM1, sawIntervalModel bool
				for _, m := range report.Models {
					if m.Client == "codex" && m.Name == "M1" {
						sawM1 = true
						if m.Activity == nil || m.Activity.ActiveSessions != 1 || m.Activity.ToolCalls != 1 ||
							m.Activity.CompletedCalls != 1 || m.Activity.TotalDurationMS != 250 ||
							m.Activity.AverageDuration == nil || *m.Activity.AverageDuration != 250 {
							t.Fatalf("unexpected M1 model activity: %+v", m.Activity)
						}
					}
					if m.Name == "M-interval-only" {
						sawIntervalModel = true
						if m.Activity != nil {
							t.Fatalf("interval-only model fabricated activity: %+v", m.Activity)
						}
					}
				}
				if !sawM1 || !sawIntervalModel {
					t.Fatalf("missing expected models: sawM1=%v sawIntervalModel=%v", sawM1, sawIntervalModel)
				}
				wantCacheOrder := []struct {
					client, session string
					cached, logical int64
				}{
					{"codex", "session-5", 3, 20},
					{"claude", "session-8", 3, 10},
					{"codex", "session-2", 3, 10},
					{"codex", "session-8", 3, 10},
					{"codex", "session-1", 2, 10},
					{"codex", "session-4", 2, 10},
					{"codex", "session-7", 2, 10},
					{"codex", "session-3", 1, 10},
					{"codex", "session-6", 1, 10},
				}
				if len(report.CacheSessions) != len(wantCacheOrder) {
					t.Fatalf("unexpected cache session count: %d", len(report.CacheSessions))
				}
				for i, cs := range report.CacheSessions {
					want := wantCacheOrder[i]
					if cs.Client != want.client || cs.SessionID != want.session || cs.CachedReadTokens != want.cached || cs.LogicalInputTokens != want.logical {
						t.Fatalf("cache sessions[%d]=%+v want=%+v", i, cs, want)
					}
					wantCmd := fmt.Sprintf("agentdeck session show %s --client %s --activity", cs.SessionID, cs.Client)
					if cs.DetailCommand != wantCmd {
						t.Fatalf("cache session detail command=%q want=%q", cs.DetailCommand, wantCmd)
					}
				}
				// Verify exact UnpricedModels ordering: (client ASC, model ASC).
				wantUnpriced := [][2]string{{"claude", "U1"}, {"claude", "U2"}, {"codex", "U1"}, {"codex", "U3"}}
				if len(report.UnpricedModels) != len(wantUnpriced) {
					t.Fatalf("unexpected unpriced models: %+v", report.UnpricedModels)
				}
				for idx, w := range wantUnpriced {
					if report.UnpricedModels[idx].Client != w[0] || report.UnpricedModels[idx].Model != w[1] {
						t.Fatalf("unpriced[%d]=%+v want=%v", idx, report.UnpricedModels[idx], w)
					}
				}
			} else if string(raw) != string(firstJSON) {
				t.Fatalf("iteration %d produced non-deterministic stats JSON ordering", i)
			}
		}
	})

	t.Run("R9-F5 unmeasured cache detail reports nil cache_hit_rate while measured zero reports 0.00", func(t *testing.T) {
		day := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		opts := StatsOptions{From: day, To: day.Add(24 * time.Hour), GroupBy: "day", Metric: "tokens", Location: time.UTC}

		unmeasured := ledger.Member{
			Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "th", ID: "iv-unmeasured"},
			Tokens: ledger.Counts{Input: 10}, Model: "M",
			Interval: ledger.CounterRange{
				Epoch: [32]byte{1}, Lower: ledger.Counts{Input: 1}, Upper: ledger.Counts{Input: 11},
				LowerAt: day.Add(time.Hour), UpperAt: day.Add(2 * time.Hour),
			},
		}
		repUnmeasured, _, err := reduceScopedStatsWithToolCalls([]storedEvent{}, []scopedToolCall{}, []ledger.Member{unmeasured}, opts, nil)
		if err != nil || len(repUnmeasured.Clients) != 1 || len(repUnmeasured.Models) != 1 {
			t.Fatalf("expected one Codex client and model row: err=%v clients=%+v models=%+v", err, repUnmeasured.Clients, repUnmeasured.Models)
		}
		if repUnmeasured.Clients[0].CacheHitRate != nil {
			t.Fatalf("10 input tokens with unmeasured cache reported client cache_hit_rate=%s; want nil", *repUnmeasured.Clients[0].CacheHitRate)
		}
		if repUnmeasured.Models[0].CacheHitRate != nil {
			t.Fatalf("10 input tokens with unmeasured cache reported model cache_hit_rate=%s; want nil", *repUnmeasured.Models[0].CacheHitRate)
		}

		measuredZero := ledger.Member{
			Key:    ledger.Key{Kind: ledger.TokenInterval, Thread: "th", ID: "iv-zero"},
			Tokens: ledger.Counts{Input: 10, Cached: 0, Known: ledger.CachedKnown}, Model: "M",
			Interval: ledger.CounterRange{
				Epoch: [32]byte{1}, Lower: ledger.Counts{Input: 1, Cached: 0, Known: ledger.CachedKnown}, Upper: ledger.Counts{Input: 11, Cached: 0, Known: ledger.CachedKnown},
				LowerAt: day.Add(time.Hour), UpperAt: day.Add(2 * time.Hour),
			},
		}
		repMeasuredZero, _, err := reduceScopedStatsWithToolCalls([]storedEvent{}, []scopedToolCall{}, []ledger.Member{measuredZero}, opts, nil)
		if err != nil || len(repMeasuredZero.Clients) != 1 || len(repMeasuredZero.Models) != 1 {
			t.Fatalf("expected measured-zero rows: err=%v", err)
		}
		if repMeasuredZero.Clients[0].CacheHitRate == nil || *repMeasuredZero.Clients[0].CacheHitRate != "0.00" {
			t.Fatalf("measured zero client cache_hit_rate=%v; want 0.00", repMeasuredZero.Clients[0].CacheHitRate)
		}
		if repMeasuredZero.Models[0].CacheHitRate == nil || *repMeasuredZero.Models[0].CacheHitRate != "0.00" {
			t.Fatalf("measured zero model cache_hit_rate=%v; want 0.00", repMeasuredZero.Models[0].CacheHitRate)
		}
	})

	t.Run("R9-F6 scoped presentation and stats overflow fail closed", func(t *testing.T) {
		day := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		events := []storedEvent{
			{Event: Event{Key: "A", Client: "codex", SessionID: "th", Model: "M", EventAt: day.Add(time.Hour).Format(time.RFC3339Nano), Tokens: map[string]int64{"input_tokens": math.MaxInt64, "output_tokens": 0}}},
			{Event: Event{Key: "B", Client: "codex", SessionID: "th", Model: "M", EventAt: day.Add(2 * time.Hour).Format(time.RFC3339Nano), Tokens: map[string]int64{"input_tokens": 1, "output_tokens": 0}}},
		}
		presPrice := func(e storedEvent) (eventAttribution, Result, error) {
			return eventAttribution{provider: "Q", quality: "exact", reason: attributionReasonExactRun, spendEligible: true},
				Result{KnownCatalogBaseCost: "0.000000000", KnownProviderCost: "0.000000000"}, nil
		}
		for _, todayOnly := range []bool{true, false} {
			report, err := reducePresentation(events, []ledger.Member{}, presPrice, day.Add(12*time.Hour), time.UTC, todayOnly)
			if err == nil {
				t.Fatalf("reducePresentation(todayOnly=%v) overflow returned success: summary input=%d", todayOnly, report.Summary.Tokens["input_tokens"])
			}
		}
		statsPrice := func(e storedEvent) (eventAttribution, error) {
			return eventAttribution{provider: "Q", quality: "exact", reason: attributionReasonExactRun, spendEligible: true}, nil
		}
		if _, _, err := reduceScopedStatsWithToolCalls(events, []scopedToolCall{}, []ledger.Member{}, StatsOptions{From: day, To: day.Add(24 * time.Hour), GroupBy: "day", Metric: "tokens", Location: time.UTC}, statsPrice); err == nil {
			t.Fatal("reduceScopedStats overflow returned success")
		}
		if _, err := summarizeJointContributions(events, scopedIntervalSum{}, StatsOptions{From: day, To: day.Add(24 * time.Hour)}, "", statsPrice); err == nil {
			t.Fatal("summarizeJointContributions overflow returned success")
		}
	})
}

// TestCJSIScopedStatsBucketSweep compares the full wire reports against
// fingerprints captured from the original exhaustive bucket reducer.
func TestCJSIScopedStatsBucketSweep(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 1, 0, 30, 0, 0, location)
	end := from.Add(5*time.Hour + 45*time.Minute)
	members := []ledger.Member{
		bucketInterval(1, end.Add(-time.Hour), end.Add(time.Nanosecond), "M"),
		bucketInterval(2, from.Add(-time.Hour), from, ""),
		bucketInterval(3, from.Add(30*time.Minute), from.Add(90*time.Minute), "N"),
		bucketInterval(4, from.Add(20*time.Minute), from.Add(30*time.Minute), "M"),
		bucketInterval(5, from.Add(31*time.Minute), from.Add(89*time.Minute), "M"),
		bucketInterval(6, end, end.Add(time.Hour), "M"),
		bucketInterval(7, from.Add(2*time.Hour), from.Add(2*time.Hour), ""),
		bucketInterval(8, from.Add(-time.Hour), end.Add(time.Hour), "M"),
		bucketInterval(9, from.Add(-2*time.Hour), from.Add(-time.Nanosecond), ""),
		bucketInterval(10, from.Add(20*time.Minute), from.Add(21*time.Minute), "M"),
	}
	events := []storedEvent{{Event: Event{
		Key: "event", Client: "codex", SessionID: "s", Model: "M", EventAt: from.Add(time.Hour).Format(time.RFC3339Nano),
		Tokens: map[string]int64{"input_tokens": 10},
	}}}
	price := func(storedEvent) (eventAttribution, error) {
		return eventAttribution{
			price:      modelPrice{Provider: "openai", Prices: map[string]string{"input": "1000000"}},
			multiplier: "2", provider: "Q", spendEligible: true, quality: "exact",
		}, nil
	}
	encoded := sha256.New()
	encoder := json.NewEncoder(encoded)
	for _, group := range []string{"hour", "day", "week", "month"} {
		for _, metric := range []string{"tokens", "cost", "sessions"} {
			for _, filter := range [][2]string{{"", ""}, {"M", ""}, {"", "Q"}, {"M", "Q"}} {
				options := StatsOptions{From: from, To: end, GroupBy: group, Metric: metric, Location: location, Model: filter[0], Provider: filter[1]}
				report, partial, err := reduceScopedStatsWithToolCalls(events, []scopedToolCall{}, members, options, price)
				if err != nil {
					t.Fatal(err)
				}
				if err := encoder.Encode(struct {
					Report  StatsReport
					Partial bool
				}{report, partial}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	const want = "b08d8592441705184ee2d24539105b1343b3f8c2417e532ed51272944f757cc2"
	if got := fmt.Sprintf("%x", encoded.Sum(nil)); got != want {
		t.Fatalf("48 Stats wire reports differ from exhaustive reduction: got %s, want %s", got, want)
	}
}

func TestCJSIScopedStatsBucketSweepCarriesUpperEndpoint(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	options := StatsOptions{From: from, To: from.Add(3 * time.Hour), GroupBy: "hour", Metric: "tokens", Location: time.UTC}
	// The upper endpoint intersects the second bucket at its inclusive start.
	members := []ledger.Member{bucketInterval(1, from.Add(30*time.Minute), from.Add(time.Hour), "M")}
	report, _, err := reduceScopedStatsWithToolCalls([]storedEvent{}, []scopedToolCall{}, members, options, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Buckets) != 3 || report.Totals.Tokens != 1 {
		t.Fatalf("got %d buckets and %d total tokens, want 3 buckets and 1 total token", len(report.Buckets), report.Totals.Tokens)
	}
	for i, wantComplete := range []bool{false, false, true} {
		bucket := report.Buckets[i]
		if complete := bucket.ProviderCost != nil; complete != wantComplete || bucket.Tokens != 0 {
			t.Errorf("bucket %d: complete provider cost=%v, tokens=%d; want complete provider cost=%v, tokens=0", i, complete, bucket.Tokens, wantComplete)
		}
	}
}

func bucketInterval(i int, from, to time.Time, model string) ledger.Member {
	return ledger.Member{
		Key: ledger.Key{Kind: ledger.TokenInterval, ID: fmt.Sprint(i), Thread: "s"}, Model: model,
		Tokens: ledger.Counts{Input: 1}, Interval: ledger.CounterRange{LowerAt: from, UpperAt: to},
	}
}

func BenchmarkCJSIScopedStatsBucketSweep(b *testing.B) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	options := StatsOptions{From: from, To: from.AddDate(0, 0, 400), Location: time.UTC, GroupBy: "hour", Metric: "tokens"}
	members := make([]ledger.Member, 20_000)
	for i := range members {
		start := from.Add(time.Duration(len(members)-i-1) * 28 * time.Minute)
		members[i] = bucketInterval(i, start, start.Add(5*time.Minute), "")
	}
	b.ResetTimer()
	for range b.N {
		report, _, err := reduceScopedStatsWithToolCalls([]storedEvent{}, []scopedToolCall{}, members, options, nil)
		if err != nil || len(report.Buckets) != 9600 || report.Totals.Tokens != 20_000 {
			b.Fatalf("400-day hourly report: %d buckets, %d tokens, %v", len(report.Buckets), report.Totals.Tokens, err)
		}
	}
}
