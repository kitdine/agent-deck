package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Independent selection for this JSONL-only fixture's proved-empty OTel side.
type emptyReportReceipts struct {
	store.CJSIOTelReceiptStore
	stamp ledger.ProposalStamp
}

// Compare the borrowed report's effective binding and all event/session
// columns to the ordinary SQL reader over a persisted restoration.
func TestReportOwnershipFactProjection(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	exec := func(q string, args ...any) { intervalFixtureExec(t, st.DB, q, args...) }
	exec(`INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(31,'codex','restored','1.3','2026-01-01T00:00:00Z'),(32,'codex','other','1','2026-01-01T00:00:00Z');
	INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('old','codex','s','r','2026-01-02T00:00:00Z','m','old',0);
	INSERT INTO usage_sessions(client,session_id,first_at,last_at,started_at) VALUES('codex','s','old','old','old')`)
	var effects store.CJSIOTelReportEffects
	if err := json.Unmarshal([]byte(`{"Restoring":true,"Remove":{"Events":[{"Key":"old"}],"Sessions":[{"Thread":"s"}]},"Insert":{"Events":[{"Key":"otel:r","Client":"codex","Thread":"s","ID":"r","At":"2026-01-02T00:00:00Z","Model":"m","Input":9007199254740993,"Cached":1,"Output":2,"CacheRead":3,"CacheCreation":4,"Write5m":5,"Write1h":6,"CacheWrite":7,"Path":"otel:r","Offset":19,"Run":{"Valid":true,"Int64":32},"Turn":{"Valid":true,"Int64":3}}],"Bindings":[{"Event":"otel:r","Run":31}],"Sessions":[{"Client":"codex","Thread":"s","First":"first","Last":"last","Started":"started"}]}}`), &effects); err != nil {
		t.Fatal(err)
	}
	tx := borrowIntervalFixture(t, st.DB)
	f := newProposedFacts(tx)
	f.otel(effects)
	q, err := f.reader()
	if err != nil {
		t.Fatal(err)
	}
	w := scopedRequestWindow{client: "codex", session: "s", from: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), to: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)}
	projected, err := readReportEvents(ctx, q, w, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	exec(`DELETE FROM usage_events WHERE event_key='old'; UPDATE usage_sessions SET first_at='first',last_at='last',started_at='started';
	INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens,source_path,source_offset,run_id,turn_index) VALUES('otel:r','codex','s','r','2026-01-02T00:00:00Z','m',9007199254740993,1,2,3,4,5,6,7,'otel:r',19,32,3);
	INSERT INTO usage_run_bindings VALUES('otel:r',31)`)
	actual, err := readReportEvents(ctx, st.DB, w, "", false)
	if err != nil || len(actual) != 1 || actual[0].runID.Int64 != 31 || !reflect.DeepEqual(actual, projected) {
		t.Fatalf("OTel report columns differ: %+v / %+v: %v", projected, actual, err)
	}
}

func TestProposedReportRunWitness(t *testing.T) {
	r := newReplayDatabase(t)
	c, d := r.candidate(t, 1, ledgerNative("R", counter(7), ""))
	v, err := c.proposal.Eligibility(d.Stamp, c.thread)
	if err != nil {
		t.Fatal(err)
	}
	m := v.Members[0]
	ref := m.Proof[0]
	path := c.worksets[0].Selected.Path
	m.ExactRun = 1
	c.runs = []ledger.RunRange{{Source: ref.Source, Bytes: ref.Span, Completed: true, Exact: true, RunID: 1, StartHash: "one", EndHash: "two"}}
	exec := func(q string, args ...any) { intervalFixtureExec(t, r.db, q, args...) }
	exec(`INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact) VALUES(1,'codex','p','1','a','b',1),(2,'codex','q','1','a','b',1)`)
	exec(`INSERT INTO usage_run_sources(run_id,path,start_offset,end_offset,start_hash,end_hash) VALUES(1,?,?,?,?,?)`, path, ref.Span.Start, ref.Span.End, "one", "two")
	check := func(want any) {
		t.Helper()
		tx := borrowIntervalFixture(t, r.db)
		got, err := proposedRun(t.Context(), tx, c, m)
		if err != nil || got != want {
			t.Fatalf("run projection=%v want=%v err=%v", got, want, err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
	check(int64(1))
	c.runs = append(c.runs, ledger.RunRange{Source: ref.Source, Bytes: ingest.CodexSpan{Start: ref.Span.Start, End: ref.Span.End}, OriginalPath: "pre-move", Completed: true, Exact: true, RunID: 1})
	exec(`INSERT INTO usage_run_sources(run_id,path,start_offset,end_offset,start_hash,end_hash) VALUES(2,'pre-move',?,?,?,?)`, ref.Span.Start, ref.Span.End, "three", "four")
	check(nil)
}

func TestReportStatsMissingToolReader(t *testing.T) {
	db, parent := newIntervalFixture(t)
	tx := borrowIntervalFixture(t, db)
	r := PublishedReports{lease: t.Context(), tx: tx, selected: parent, read: func(context.Context, *sql.Tx, scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
		return []storedEvent{}, nil, nil
	}}
	q := StatsOptions{From: time.Now().UTC().Truncate(time.Hour)}
	q.To = q.From.Add(time.Hour)
	q.GroupBy = "day"
	q.Metric = "tokens"
	if _, _, err := r.Stats(t.Context(), q); err == nil {
		t.Fatal("missing tool reader reported as known empty")
	}
}

func TestPublishedReportPinnedAcrossMethods(t *testing.T) {
	ctx := t.Context()
	db := newReplayDatabase(t)
	writer, err := sql.Open("sqlite", filepath.Join(db.root, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	intervalFixtureExec(t, writer, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('one','codex','S','one','2026-01-02T10:00:00Z','M',9,'fixture',0)`)
	clock := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	s := ReportSelection{db.epoch, 0, []ReportGroup{}, clock}
	scope := StatsOptions{From: clock.Add(-5 * time.Hour), To: clock.Add(time.Hour), Location: time.UTC, Timezone: "UTC", GroupBy: "day", Metric: "tokens"}
	tx := borrowIntervalFixture(t, db.db)
	err = WithPublishedReports(ctx, tx, s, func(v *PublishedReports) error {
		one, err := v.Summary(ctx, scope, "")
		if err != nil {
			return err
		}
		intervalFixtureExec(t, writer, `UPDATE usage_events SET input_tokens=27 WHERE event_key='one'`)
		session, err := v.SessionUsage(ctx, "codex", "S")
		if err != nil {
			return err
		}
		p, err := v.Presentation(ctx, time.UTC, true)
		if err != nil {
			return err
		}
		if one.Summary.Tokens["input_tokens"] != 9 || session.Tokens["input_tokens"] != 9 || p.Summary.Tokens["input_tokens"] != 9 {
			t.Fatal("one borrow mixed committed snapshots")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := WithPublishedReports(ctx, borrowIntervalFixture(t, db.db), s, func(v *PublishedReports) error {
		session, err := v.SessionUsage(ctx, "codex", "S")
		if err == nil && session.Tokens["input_tokens"] != 27 {
			t.Fatal("new borrow missed committed update")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func (f emptyReportReceipts) Select(_ context.Context, _ *sql.Tx, thread string) (store.CJSIOTelSelection, error) {
	return store.CJSIOTelSelection{Stamp: f.stamp, Thread: thread, Revision: 1}, nil
}

func TestUnit2aProposedReaderAndStampBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int64
	}{
		{"replace", ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10)) + ledgerNative("", counter(103), `,"thread_token_usage":`+counter(113)), 113},
		{"remove", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			r := newReplayDatabase(t)
			exec := func(q string, args ...any) { t.Helper(); intervalFixtureExec(t, r.db, q, args...) }
			exec(`DROP TABLE cjsi_usage_intervals; UPDATE cjsi_publication_head SET ticket=7`)
			installIntervalTables(t, r.db, "cjsi_core_facts_schema.go", "cjsi_usage_intervals", "cjsi_usage_interval_dirty_ranges")
			c, d := r.candidate(t, 8, tc.body)
			exec(`ALTER TABLE cjsi_publication_attempts ADD COLUMN session_seal;
			ALTER TABLE cjsi_publication_head ADD COLUMN slot_ticket; ALTER TABLE cjsi_publication_head ADD COLUMN slot_attempt`)
			base := ledger.ProposalStamp{Basis: ledger.Basis{Ticket: 7, Attempt: 3, CoreEpoch: r.epoch, BaseTicket: 6, InputDigest: [32]byte{1}}, Diff: [32]byte{2}}
			exec(`INSERT INTO cjsi_publication_attempts(ticket,attempt,core_epoch,base_ticket,input_digest,state) VALUES(7,3,?,6,?,'opened')`, r.epoch, base.Basis.InputDigest[:])
			clock := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
			parent := ReportSelection{CoreEpoch: r.epoch, Ticket: 7, Clock: clock}
			for k, thread := range []string{ledgerThread, "retained"} {
				n := int64(intervalLarge)
				if k != 0 {
					n = 17
				}
				exec(`INSERT INTO cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest) VALUES(7,3,?,?,?)`, thread, base.Basis.InputDigest[:], base.Diff[:])
				exec(`INSERT INTO cjsi_usage_selection VALUES(?,7,3)`, thread)
				exec(`INSERT INTO cjsi_usage_members(ticket,attempt,thread,kind,member_id,model,input_tokens,output_tokens) VALUES(7,3,?,3,'old','other',?,0)`, thread, n)
				exec(`INSERT INTO cjsi_usage_intervals(ticket,attempt,thread,kind,member_id,counter_epoch,lower_at,upper_at,lower_input,upper_input,lower_output,upper_output)
					VALUES(7,3,?,3,'old',CAST(zeroblob(31)||x'01' AS BLOB),?,?,0,?,0,0)`, thread, intervalStart, intervalEnd, n)
				parent.Groups = append(parent.Groups, ReportGroup{thread, base})
			}
			if err := c.Stage(ctx, r.db, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }); err != nil {
				t.Fatal(err)
			}
			diff := [32]byte{99}
			exec(`UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=zeroblob(32) WHERE ticket=8`, diff[:])
			tx := borrowIntervalFixture(t, r.db)
			q := StatsOptions{From: clock.Add(-6 * time.Hour), To: clock.Add(time.Hour), Timezone: "UTC", Location: time.UTC}
			far := q
			far.From = far.From.AddDate(0, 0, -3)
			far.To = far.To.AddDate(0, 0, -3)
			cov := [32]byte{1}
			type result struct {
				sum, filtered, provider ReportSummary
				stamp, far              [32]byte
				presentation            PresentationReport
			}
			read := func(v *PublishedReports, o *result) error {
				var err error
				if o.sum, err = v.Summary(ctx, q, ""); err != nil {
					return err
				}
				m := q
				m.Model = "other"
				if o.filtered, err = v.Summary(ctx, m, ""); err != nil {
					return err
				}
				m.Model, m.Provider = "", "absent-provider"
				if o.provider, err = v.Summary(ctx, m, ""); err != nil {
					return err
				}
				if o.stamp, err = v.AggregateCacheStamp(ctx, q, cov); err != nil {
					return err
				}
				if o.far, err = v.AggregateCacheStamp(ctx, far, cov); err != nil {
					return err
				}
				o.presentation, err = v.Presentation(ctx, time.UTC, true)
				return err
			}
			var old, proposed, opened result
			if err := WithPublishedReports(ctx, tx, parent, func(v *PublishedReports) error { return read(v, &old) }); err != nil {
				t.Fatal(err)
			}
			if old.sum.Summary.Tokens["input_tokens"] != intervalLarge+17 {
				t.Fatal("parent lost original interval")
			}
			if _, err := ReadCJSIAggregateCacheStamp(ctx, tx, r.epoch, 8, q, cov); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("public stamp accepted unopened target: %v", err)
			}
			p := ReportProposal{8, 1, diff, []ProposedReportGroup{{Usage: c, Draft: d, Receipts: emptyReportReceipts{stamp: d.Stamp}}}}
			omitted := parent
			omitted.Groups = parent.Groups[1:]
			requireRejectedReport(t, tx, omitted, p, ledger.ErrProposalMismatch, "omitted changed parent group")
			impact, err := WithProposedReports(ctx, tx, parent, p, func(v *PublishedReports) error { return read(v, &proposed) })
			if err != nil {
				t.Fatal(err)
			}
			if proposed.sum.Summary.Tokens["input_tokens"] != tc.want+17 || proposed.presentation.Summary.Tokens["input_tokens"] != tc.want+17 {
				t.Fatalf("replacement did not preserve retained group: %+v", proposed.sum)
			}
			if proposed.filtered.ShareBase != tc.want+17 || proposed.filtered.Summary.Tokens["input_tokens"] != 17 {
				t.Fatalf("filtered share base: %+v", proposed.filtered)
			}
			if proposed.provider.ShareBase != tc.want+17 || proposed.provider.Summary.Tokens["input_tokens"] != 0 {
				t.Fatalf("provider share base: %+v", proposed.provider)
			}
			if proposed.stamp == old.stamp || proposed.far != old.far {
				t.Fatal("target stamp did not track only intersecting windows")
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			err = stampFixtureTransaction(t, r.db, func(w *sql.Tx) error {
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
				if err = store.WriteUsageIntervalDirtyRanges(ctx, w, r.epoch, 8, ranges); err != nil {
					return err
				}
				_, err = w.Exec("UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=8")
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			parent.Ticket = 8
			parent.Groups[0].Stamp = d.Stamp
			if err := WithPublishedReports(ctx, borrowIntervalFixture(t, r.db), parent, func(v *PublishedReports) error { return read(v, &opened) }); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(proposed, opened) {
				t.Fatal("target reports or stamps differ after selecting the prepared delta")
			}
		})
	}
}

func TestReportQueryExtraction(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	exec := func(q string) { t.Helper(); intervalFixtureExec(t, st.DB, q) }
	exec(`INSERT INTO provider_selections(client,provider_name_snapshot,multiplier_snapshot,selected_at) VALUES('codex','P','1.3','2026-01-01T00:00:00Z');
	INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('fixture','bundled','fixture','abc','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
	INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES('fixture','M','openai','2026-01-01T00:00:00Z','{"input":"1"}','[]');
	INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(9,'codex','P','1.3','2026-01-02T00:00:00Z');
	INSERT INTO usage_sessions(client,session_id,first_at,last_at,started_at) VALUES('codex','S','2026-01-02T00:00:00Z','2026-01-02T10:00:00Z','2026-01-01T23:00:00Z');
	INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset,turn_index) VALUES('K','codex','S','R','2026-01-02T10:00:00Z','M',43,2,'fixture',19,3);
	INSERT INTO usage_run_bindings VALUES('K',9)`)
	s := New(st, t.TempDir())
	clock := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	p0, err := s.loadReadPriceResolver(ctx, clock)
	if err != nil {
		t.Fatal(err)
	}
	l0, err := st.LoadProviderTimeline(ctx)
	if err != nil {
		t.Fatal(err)
	}
	w := scopedRequestWindow{from: clock.Add(-4 * time.Hour), to: clock, client: "codex"}
	ev, err := s.eventsRangeFiltered(ctx, w.from, w.to, w.client, "M", true)
	if err != nil {
		t.Fatal(err)
	}
	tx := borrowIntervalFixture(t, st.DB)
	p1, err := loadReportPriceResolver(ctx, tx, clock)
	if err != nil {
		t.Fatal(err)
	}
	l1, err := store.ReadProviderTimeline(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	w.session = "S"
	row, err := readReportEvents(ctx, tx, w, "M", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != 1 || row[0].runID.Int64 != 9 || row[0].SourceOffset != 19 || len(p1.byModel) == 0 || !reflect.DeepEqual(ev, row) || !reflect.DeepEqual(l0, l1) || !reflect.DeepEqual(p0, p1) {
		t.Fatal("transaction extraction changed event, resolver or timeline")
	}
	w.session = "not-selected"
	empty, err := readReportEvents(ctx, tx, w, "M", true)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("session-scoped verified empty: %+v %v", empty, err)
	}
}
