package usage

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// The local day crosses midnight UTC and combines a routed Codex request,
// an interval, a tool call, and a Claude request.
func TestReportTimezonePricingRoutesToolsParity(t *testing.T) {
	ctx := t.Context()
	r, parent := newReportGuardFixture(t)
	exec := func(q string, args ...any) { t.Helper(); intervalFixtureExec(t, r.db, q, args...) }
	exec(`UPDATE cjsi_publication_head SET ticket=7`)

	// Set up provider timeline, prices, routes, Claude event, and tool call for ledgerThread!
	// Tool call provider depends on usage_sessions.first_at!
	exec(`INSERT INTO provider_selections(client,provider_name_snapshot,multiplier_snapshot,selected_at) VALUES
		('codex','prov-early','1.0','2026-01-01T00:00:00Z'),
		('codex','prov-late','2.0','2026-01-02T09:00:00Z'),
		('claude','anthropic-direct','1.0','2026-01-01T00:00:00Z');
	INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
		('v1','bundled','fixture','h1','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
	INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
		('v1','m','openai','2026-01-01T00:00:00Z','{"input":"10","output":"20"}','[]'),
		('v1','claude-sonnet','anthropic','2026-01-01T00:00:00Z','{"input":"3","output":"15"}','[]');
	INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES
		('claude:c1:e1','claude','c1','e1','2026-01-03T06:30:00Z','claude-sonnet',50,10,'claude.jsonl',0);
	INSERT INTO usage_sessions(client,session_id,first_at,last_at,started_at) VALUES
		('claude','c1','2026-01-03T06:30:00Z','2026-01-03T06:30:00Z','2026-01-03T06:30:00Z');
	INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,duration_ms,source_path,source_offset,turn_index,tool_kind) VALUES
		('tc1','codex',?,'m','exec_command','2026-01-02T10:05:00Z','completed',42,'fixture',1,1,'bash')`, ledgerThread)

	// Add route for ledgerThread
	exec(`INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key) VALUES
		('codex',?,'2026-01-02T09:30:00Z','prov-late','2.0',1,'start','hook','exact','k1')`, ledgerThread)

	// Note: ledgerNative uses 2026-01-02T10:00:00Z (which is 2026-01-02T02:00:00-08:00 in America/Los_Angeles)
	// And Claude event is at 2026-01-03T06:30:00Z (which is 2026-01-02T22:30:00-08:00 — SAME local day, DIFFERENT UTC day!)
	body := ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10)) +
		ledgerNative("", counter(40), `,"thread_token_usage":`+counter(50))
	c, d := r.candidate(t, 8, body)
	r.installReportPublicationColumns(t)
	base := ledger.ProposalStamp{Basis: ledger.Basis{Ticket: 7, Attempt: 3, CoreEpoch: r.epoch, BaseTicket: 6, InputDigest: [32]byte{1}}, Diff: [32]byte{2}}
	exec(`INSERT INTO cjsi_publication_attempts(ticket,attempt,core_epoch,base_ticket,input_digest,state) VALUES(7,3,?,6,?,'opened')`, r.epoch, base.Basis.InputDigest[:])
	exec(`INSERT INTO cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest) VALUES(7,3,?,?,?)`, ledgerThread, base.Basis.InputDigest[:], base.Diff[:])
	exec(`INSERT INTO cjsi_usage_selection VALUES(?,7,3)`, ledgerThread)
	exec(`INSERT INTO cjsi_usage_members(ticket,attempt,thread,kind,member_id,model,input_tokens,output_tokens) VALUES(7,3,?,3,'old','m',200,0)`, ledgerThread)
	exec(`INSERT INTO cjsi_usage_intervals(ticket,attempt,thread,kind,member_id,counter_epoch,lower_at,upper_at,lower_input,upper_input,lower_output,upper_output)
		VALUES(7,3,?,3,'old',CAST(zeroblob(31)||x'01' AS BLOB),?,?,0,200,0,0)`, ledgerThread, intervalStart, intervalEnd)
	if err := c.Stage(ctx, r.db, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }); err != nil {
		t.Fatal(err)
	}
	diff := [32]byte{88}
	exec(`UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=zeroblob(32) WHERE ticket=8`, diff[:])

	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-01-02 23:00:00 PST = 2026-01-03 07:00:00 UTC
	clock := time.Date(2026, 1, 2, 23, 0, 0, 0, la)
	dayStart := time.Date(2026, 1, 2, 0, 0, 0, 0, la)
	dayEnd := dayStart.AddDate(0, 0, 1)
	qAll := StatsOptions{From: dayStart, To: dayEnd, Location: la, Timezone: la.String(), GroupBy: "hour", Metric: "tokens"}
	qProv := qAll
	qProv.Provider = "prov-late"
	qClaude := qAll
	qClaude.Client = "claude"
	cov := [32]byte{7}

	type snap struct {
		sumAll, sumProv, sumClaude ReportSummary
		statsAll, statsProv        StatsReport
		pAll, pProv                bool
		pres                       PresentationReport
		sess                       SessionSummary
		stampAll, stampClaude      [32]byte
	}
	capture := func(v *PublishedReports, o *snap) error {
		var err error
		if o.sumAll, err = v.Summary(ctx, qAll, ""); err != nil {
			return err
		}
		if o.sumProv, err = v.Summary(ctx, qProv, ""); err != nil {
			return err
		}
		if o.sumClaude, err = v.Summary(ctx, qClaude, ""); err != nil {
			return err
		}
		if o.statsAll, o.pAll, err = v.Stats(ctx, qAll); err != nil {
			return err
		}
		if o.statsProv, o.pProv, err = v.Stats(ctx, qProv); err != nil {
			return err
		}
		if o.pres, err = v.Presentation(ctx, la, false); err != nil {
			return err
		}
		if o.sess, err = v.SessionUsage(ctx, "codex", ledgerThread); err != nil {
			return err
		}
		if o.stampAll, err = v.AggregateCacheStamp(ctx, qAll, cov); err != nil {
			return err
		}
		if o.stampClaude, err = v.AggregateCacheStamp(ctx, qClaude, cov); err != nil {
			return err
		}
		return nil
	}

	parent.Ticket, parent.Clock = 7, clock
	parent.Groups = []ReportGroup{{ledgerThread, base}}
	p := ReportProposal{8, 1, diff, []ProposedReportGroup{{Usage: c, Draft: d, Receipts: emptyReportReceipts{stamp: d.Stamp}}}}
	tx := borrowIntervalFixture(t, r.db)
	var pre, post snap
	impact, err := WithProposedReports(ctx, tx, parent, p, func(v *PublishedReports) error { return capture(v, &pre) })
	if err != nil {
		t.Fatal(err)
	}
	tx.Rollback()

	if err := stampFixtureTransaction(t, r.db, func(w *sql.Tx) error {
		if _, err := w.Exec(`UPDATE cjsi_publication_head SET ticket=8,slot_ticket=8,slot_attempt=1;
		UPDATE cjsi_publication_attempts SET state='sealed' WHERE ticket=8; INSERT OR REPLACE INTO cjsi_usage_selection VALUES(?,8,1)`, ledgerThread); err != nil {
			return err
		}
		if err := c.ApplySelectedRequests(ctx, w, d.Stamp, d.ExpectedOldRows); err != nil {
			return err
		}
		ranges, err := impact.SelectedRanges(ctx, w)
		if err != nil {
			return err
		}
		if err := store.WriteUsageIntervalDirtyRanges(ctx, w, r.epoch, 8, ranges); err != nil {
			return err
		}
		_, err = w.Exec("UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=8")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	parent.Ticket = 8
	parent.Groups[0].Stamp = d.Stamp
	tx2 := borrowIntervalFixture(t, r.db)
	if err := WithPublishedReports(ctx, tx2, parent, func(v *PublishedReports) error { return capture(v, &post) }); err != nil {
		t.Fatal(err)
	}
	tx2.Rollback()

	if !reflect.DeepEqual(pre, post) {
		t.Fatalf("Los Angeles proposed and opened reports differ:\npre=%+v\npost=%+v", pre, post)
	}
	if pre.sumAll.Summary.Tokens["input_tokens"] != 100 { // 50 claude + 10 req + 40 interval
		t.Fatalf("unexpected total input=%d want 100", pre.sumAll.Summary.Tokens["input_tokens"])
	}
	if pre.sumClaude.Summary.Tokens["input_tokens"] != 50 {
		t.Fatalf("unexpected claude input=%d want 50", pre.sumClaude.Summary.Tokens["input_tokens"])
	}
	if pre.sumProv.Summary.Tokens["input_tokens"] != 10 {
		t.Fatalf("routed request input = %d, want 10", pre.sumProv.Summary.Tokens["input_tokens"])
	}
	if pre.sumProv.Totals.KnownProviderCost != "0.000200000" || pre.sumProv.Summary.ProviderCost != nil {
		t.Fatalf("routed request pricing: known=%q, complete=%v", pre.sumProv.Totals.KnownProviderCost, pre.sumProv.Summary.ProviderCost)
	}
	if len(pre.statsProv.Models) == 0 || pre.statsProv.Models[0].Activity.ToolCalls != 1 {
		t.Fatalf("expected tool call under prov-late via projected usage_sessions.first_at, got %+v", pre.statsProv.Models)
	}
}

type reportPublicationSnapshot struct {
	summary      ReportSummary
	stats        StatsReport
	statsPartial bool
	presentation PresentationReport
	session      SessionSummary
	stamp        [32]byte
}

func (s *reportPublicationSnapshot) read(ctx context.Context, reports *PublishedReports, options StatsOptions) error {
	var err error
	if s.summary, err = reports.Summary(ctx, options, ""); err != nil {
		return err
	}
	if s.stats, s.statsPartial, err = reports.Stats(ctx, options); err != nil {
		return err
	}
	if s.presentation, err = reports.Presentation(ctx, options.Location, false); err != nil {
		return err
	}
	if s.session, err = reports.SessionUsage(ctx, "codex", ledgerThread); err != nil {
		return err
	}
	s.stamp, err = reports.AggregateCacheStamp(ctx, options, [32]byte{9})
	return err
}

func readOpenedReportFixture(t *testing.T, r replayDatabase, s ReportSelection, q StatsOptions) reportPublicationSnapshot {
	t.Helper()
	tx := borrowIntervalFixture(t, r.db)
	defer tx.Rollback()
	var result reportPublicationSnapshot
	if err := WithPublishedReports(t.Context(), tx, s, func(v *PublishedReports) error {
		return result.read(t.Context(), v, q)
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func checkReportPublicationParity(t *testing.T, r replayDatabase, parent ReportSelection, p ReportProposal, q StatsOptions, input int64, dirty bool) reportPublicationSnapshot {
	t.Helper()
	ctx := t.Context()
	tx := borrowIntervalFixture(t, r.db)
	var proposed reportPublicationSnapshot
	impact, err := WithProposedReports(ctx, tx, parent, p, func(v *PublishedReports) error {
		return proposed.read(ctx, v, q)
	})
	if err != nil {
		t.Fatalf("valid proposal rejected: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	if err := stampFixtureTransaction(t, r.db, func(w *sql.Tx) error {
		if err := selectReportFixture(ctx, w, p); err != nil {
			return err
		}
		for _, g := range p.Groups {
			if err := g.Usage.ApplySelectedRequests(ctx, w, g.Draft.Stamp, g.Draft.ExpectedOldRows); err != nil {
				return err
			}
		}
		ranges, err := impact.SelectedRanges(ctx, w)
		if err != nil {
			return err
		}
		if ranges == nil || (len(ranges) > 0) != dirty {
			t.Fatalf("dirty=%t, got ranges=%+v", dirty, ranges)
		}
		if err := store.WriteUsageIntervalDirtyRanges(ctx, w, r.epoch, p.Ticket, ranges); err != nil {
			return err
		}
		_, err = w.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=? AND attempt=?", p.Ticket, p.Attempt)
		return err
	}); err != nil {
		t.Fatalf("open report fixture: %v", err)
	}

	opened := parent
	opened.Ticket = p.Ticket
	if len(p.Groups) != 0 {
		d := p.Groups[0].Draft
		opened.Groups = []ReportGroup{{d.Thread, d.Stamp}}
	}
	actual := readOpenedReportFixture(t, r, opened, q)
	if !reflect.DeepEqual(proposed, actual) {
		t.Fatalf("proposed and opened reports differ:\nproposed=%+v\nopened=%+v", proposed, actual)
	}
	if got := actual.summary.Summary.Tokens["input_tokens"]; got != input {
		t.Fatalf("input tokens=%d, want %d", got, input)
	}
	return actual
}

func TestReportValidPublicationParity(t *testing.T) {
	options := func(s ReportSelection) StatsOptions {
		return StatsOptions{From: s.Clock.Add(-6 * time.Hour), To: s.Clock.Add(time.Hour), Location: time.UTC, Timezone: "UTC", GroupBy: "day", Metric: "tokens"}
	}
	bodyWithInterval := func(request, total int) string {
		return ledgerNative("R1", counter(request), `,"thread_token_usage":`+counter(request)) +
			ledgerNative("", counter(total-request), `,"thread_token_usage":`+counter(total))
	}
	parentGroup := func(parent ReportSelection, stamp ledger.ProposalStamp) ReportSelection {
		parent.Ticket = stamp.Basis.Ticket
		parent.Groups = []ReportGroup{{ledgerThread, stamp}}
		return parent
	}
	committedRequest := func(t *testing.T, r replayDatabase, s ledger.ProposalStamp) ledger.CommittedCall {
		t.Helper()
		call, err := ReadCJSICommittedCall(t.Context(), r.db, s, ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: "R1"})
		if err != nil {
			t.Fatal(err)
		}
		return call
	}
	seedDirtyDay := func(t *testing.T, r replayDatabase, parent ReportSelection) {
		t.Helper()
		day, _ := store.UsageIntervalStampDay(parent.Clock)
		intervalFixtureExec(t, r.db, "INSERT INTO cjsi_usage_interval_dirty_ranges VALUES(?,'all-summary','','',?,?,?)", r.epoch, day, day+1, parent.Ticket)
	}

	t.Run("bootstrap_ticket_0_parity", func(t *testing.T) {
		r, parent := newReportGuardFixture(t)
		p := r.frozenReportProposal(t, bodyWithInterval(15, 40))
		checkReportPublicationParity(t, r, parent, p, options(parent), 40, true)
	})

	t.Run("withdraw_only_and_delete_interval_parity", func(t *testing.T) {
		r, parent := newReportGuardFixture(t)
		stamp := r.publish(t, 1, bodyWithInterval(12, 42))
		parent = parentGroup(parent, stamp)
		seedDirtyDay(t, r, parent)
		q := options(parent)
		before := readOpenedReportFixture(t, r, parent, q)
		if before.summary.Summary.Tokens["input_tokens"] != 42 {
			t.Fatal("parent fixture lacks the request or interval")
		}
		c, d := r.candidate(t, 2, ledgerNative("R1", counter(12), "")+ledgerNative("R1", counter(99), ""), committedRequest(t, r, stamp))
		r.installReportPublicationColumns(t)
		p := r.freezeReportGroup(t, c, d, [32]byte{66})
		after := checkReportPublicationParity(t, r, parent, p, q, 0, true)
		if after.session.Tokens["input_tokens"] != 0 || after.stamp == before.stamp {
			t.Fatalf("withdrawal left request tokens or the old stamp: %+v", after.session)
		}
	})

	t.Run("valid_registered_replacement_not_rejected_by_F3", func(t *testing.T) {
		r, parent := newReportGuardFixture(t)
		body := ledgerNative("R1", counter(10), "")
		stamp := r.publish(t, 1, body)
		call := committedRequest(t, r, stamp)
		c, d := r.candidate(t, 2, body+ledgerNative("R2", counter(20), ""), call)
		r.installReportPublicationColumns(t)
		p := r.freezeReportGroup(t, c, d, [32]byte{67})
		checkReportPublicationParity(t, r, parentGroup(parent, stamp), p, options(parent), 30, false)
		written, err := ReadCJSIOldUsage(t.Context(), r.db, call.Prior.EventKey)
		if err != nil || written.Path != d.Worksets[0].Selected.Path {
			t.Fatalf("registered key did not pick up the new proof path: %+v, %v", written, err)
		}
	})

	t.Run("cross_day_empty_groups_preserves_retained_and_stamp", func(t *testing.T) {
		r, parent := newReportGuardFixture(t)
		parent = parentGroup(parent, r.publish(t, 1, bodyWithInterval(10, 60)))
		seedDirtyDay(t, r, parent)
		r.installReportPublicationColumns(t)
		diff, input := [32]byte{68}, [32]byte{2}
		intervalFixtureExec(t, r.db, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,input_digest,diff_digest,state,accept_until,index_epoch,session_revision,coverage_revision,protection_revision,session_seal)
			VALUES(2,1,1,?,?,?,'draft','2099-01-01T00:00:00Z',0,0,0,0,zeroblob(32))`, r.epoch, input[:], diff[:])
		la, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			t.Fatal(err)
		}
		parent.Clock = time.Date(2026, 1, 2, 23, 30, 0, 0, la)
		start := time.Date(2026, 1, 2, 0, 0, 0, 0, la)
		q := StatsOptions{From: start, To: start.AddDate(0, 0, 1), Location: la, Timezone: la.String(), GroupBy: "hour", Metric: "tokens"}
		before := readOpenedReportFixture(t, r, parent, q)
		p := ReportProposal{2, 1, diff, []ProposedReportGroup{}}
		after := checkReportPublicationParity(t, r, parent, p, q, 60, false)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("empty publication changed retained reports or stamp:\nbefore=%+v\nafter=%+v", before, after)
		}
	})
}
