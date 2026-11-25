//go:build cjsi_adapter_fixture

package scanruntime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func reportOTelObservation(key, thread, at string, n int64) store.TelemetryObservation {
	out, cached := int64(1), int64(0)
	return store.TelemetryObservation{
		Key: key, ThreadID: thread, Source: "import",
		Producer: "codex_exec", ProducerVersion: "1", Originator: "codex_exec",
		EventName: "codex.sse_event", EventKind: "response.completed", Transport: "sse",
		RecordAt: at, ObservedAt: at, ReceivedAt: "2026-01-03T00:00:00Z", Model: "m",
		ResponseID: "resp-" + key, RequestID: "request-" + key, AttemptID: "attempt-" + key,
		SourceClass: "fixture", InputTokens: &n, OutputTokens: &out, CachedTokens: &cached,
		Metered: true, SupportEntry: "codex_exec/1/codex_exec/sse",
		State: "pending", Reason: "ownership_not_settled",
	}
}

func reportOTelAttribution(catalog string) store.CJSIOTelAttribution {
	a := store.CJSIOTelAttribution{
		Provider: "p", Quality: "exact", Multiplier: "1",
		CatalogVersion: catalog, PriceReference: catalog,
	}
	a.Evidence = store.CJSIOTelAttributionEvidence(a)
	return a
}

// Compare proposed takeover and partial restoration with the actual write path
// on the managed core schema, including both invalid release orders.
func TestCJSIReportPublicationOTelTakeoverAndRelease(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	idx, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	const thread = "00000000-0000-4000-8000-0000000000ab"
	clock := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)

	attrs := map[string]store.CJSIOTelAttribution{}
	for i, id := range []string{"first", "middle", "last"} {
		at := time.Date(2026, 1, 2, 10+i, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
		o := reportOTelObservation(id, thread, at, int64(i+2)) // 2, 3, 4 input (+1 output each)
		if _, err := st.ApplyTelemetryObservations(ctx, []store.TelemetryObservation{o}, nil, nil); err != nil {
			t.Fatal(err)
		}
		attrs["otel:"+id] = reportOTelAttribution(id)
	}
	if _, err := st.DB.ExecContext(ctx, "INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('synthetic',1,'2026-01-04T00:00:00Z',0,'complete')"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SettleTelemetry(ctx, clock, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.ExecContext(ctx, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(41,'codex','p','1','2026-01-01T00:00:00Z');
		INSERT INTO usage_run_bindings VALUES('otel:middle',41)`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(root, "agentdeck.sqlite3")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var epoch int64
	if err := db.QueryRowContext(ctx, "SELECT core_epoch FROM cjsi_publication_head WHERE singleton=1").Scan(&epoch); err != nil {
		t.Fatal(err)
	}

	m, err := store.ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	// Stage the same authenticated usage candidate for both consumers.
	buildCand := func(ticket int64, body string, oldRows []ledger.OldRow, committed []ledger.CommittedCall) (usage.CJSIUsageCandidate, usage.CJSIUsageDraft, store.CJSIPublicationDraft) {
		t.Helper()
		homeDir := t.TempDir()
		sdir := filepath.Join(homeDir, ".codex", "sessions")
		if err := os.MkdirAll(sdir, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(sdir, fmt.Sprintf("src-%d.jsonl", ticket))
		content := fmt.Sprintf(`{"timestamp":"2026-01-02T09:00:00Z","type":"session_meta","payload":{"id":%q}}`+"\n"+
			`{"type":"turn_context","payload":{"turn_id":"t","model":"m"}}`+"\n"+body, thread)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		inv, err := ingest.Discover(homeDir)
		if err != nil || len(inv) != 1 {
			t.Fatal(inv, err)
		}
		src := inv[0]
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		red, err := ingest.CaptureCodexReduction(ctx, f, path, src.Size, 1<<16)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		e := red.Capture().Evidence()
		e.Version = ingest.CodexCheckpointVersion{Generation: 1, Parser: ledger.Parser, MaxLineBytes: 1 << 16}
		sel := ingest.CodexSelectedSource{Path: path, ThreadID: thread, Epoch: 1, Evidence: e}
		now := time.Now().UTC()
		draft, err := m.BeginPublication(ctx, store.CJSIPublicationInputs{
			BuiltAt:  now,
			Digest:   sha256.Sum256([]byte(fmt.Sprintf("in-%d-%s", ticket, body))),
			Validate: func(context.Context, *sql.Tx) error { return nil },
		}, now.Add(time.Hour), nil)
		if err != nil {
			t.Fatal(err)
		}
		var prep usage.CJSIUsagePreparedSource
		var bound usage.CJSIUsageWorkset
		limits := ingest.CodexWorksetLimits{SegmentRecords: 1000, Turns: 1000}
		if err := draft.WithSourceWorkset(ctx, src, sel, src.Size+1, func(work store.CJSICapturedWorkset) error {
			bound = usage.CJSIUsageWorkset{Round: work.Round, Attempt: work.Attempt, Selected: sel}
			cur := func(context.Context) (usage.CJSIUsageWorkset, error) { return bound, nil }
			open := func(context.Context) (usage.CJSIUsageWorkset, ingest.CodexWorksetNext, error) {
				n, e := work.Open(ctx)
				return bound, n, e
			}
			entry := usage.InventoryEntry{Client: src.Client, Path: src.Path, Identity: src.Identity, Size: src.Size, ModifiedAt: src.ModifiedAt, ChangedAt: src.ChangedAt, Stable: src.Stable}
			var e error
			prep, e = usage.PrepareCJSIUsageCandidateSource(ctx, entry, work.File, cur, open, limits, 1000, src.Size+1)
			return e
		}); err != nil {
			t.Fatal(err)
		}
		b := draft.Basis()
		if oldRows == nil {
			oldRows = []ledger.OldRow{}
		}
		c, err := usage.PrepareCJSIUsageCandidate(ctx, ledger.ReplayInput{
			Basis: b, Thread: thread,
			Expected: []ledger.SourceStamp{ledger.SelectionStamp(sel)},
			History:  ledger.History{Assessed: true, Rows: oldRows, Committed: committed},
		}, []usage.CJSIUsagePreparedSource{prep})
		if err != nil {
			t.Fatal(err)
		}
		d := usage.CJSIUsageDraft{Thread: thread, Stamp: c.Proposal().Stamp(), Worksets: []usage.CJSIUsageWorkset{bound}, ExpectedOldRows: oldRows}
		if err := c.Stage(ctx, db, func(context.Context, usage.CJSIUsageDraftReader, string) (usage.CJSIUsageDraft, error) { return d, nil }); err != nil {
			t.Fatal(err)
		}
		return c, d, draft
	}

	// Ticket 1: Takeover OTel with a JSONL request (17 input, 3 output)
	reqBody := fmt.Sprintf(`{"timestamp":"2026-01-02T11:00:00Z","type":"token_usage_record","payload":{"thread_id":%q,"turn_id":"t","response_id":"r1","usage":{"input_tokens":17,"output_tokens":3},"thread_token_usage":{"input_tokens":17,"output_tokens":3}}}`+"\n", thread)
	c1, d1, dr1 := buildCand(1, reqBody, nil, nil)
	b1 := dr1.Basis()
	diff1 := sha256.Sum256([]byte("diff1"))
	if _, err := db.ExecContext(ctx, `UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=CAST(zeroblob(31)||x'01' AS BLOB) WHERE ticket=? AND attempt=?`, diff1[:], b1.Ticket, b1.Attempt); err != nil {
		t.Fatal(err)
	}
	rs1 := store.CJSIOTelSQLiteStore{
		Guard: func(context.Context, *sql.Tx, string) (store.CJSIOTelGuard, error) {
			return store.CJSIOTelGuard{Thread: thread, Stamp: d1.Stamp, Revision: 1}, nil
		},
		Attributions: attrs,
	}
	rtx1, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	otel1, err := store.PrepareCJSIOTel(ctx, rtx1, thread, c1.Proposal(), rs1)
	if err != nil || otel1 == nil {
		t.Fatalf("prepare takeover: otel=%v err=%v", otel1, err)
	}
	// A takeover projects the JSONL request as the sole owner.
	q := usage.StatsOptions{From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), Location: time.UTC, Timezone: "UTC", GroupBy: "day", Metric: "tokens"}
	var pre1, post1 usage.ReportSummary
	parent0 := usage.ReportSelection{CoreEpoch: epoch, Ticket: 0, Clock: clock, Groups: []usage.ReportGroup{}}
	prop1 := usage.ReportProposal{Ticket: b1.Ticket, Attempt: b1.Attempt, FrozenDiff: diff1, Groups: []usage.ProposedReportGroup{{Usage: c1, Draft: d1, OTel: otel1, Receipts: rs1, RestoreOTel: false}}}
	if _, err := usage.WithProposedReports(ctx, rtx1, parent0, prop1, func(r *usage.PublishedReports) error {
		pre1, err = r.Summary(ctx, q, "")
		return err
	}); err != nil {
		t.Fatalf("takeover proposed: %v", err)
	}
	if err := rtx1.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Apply the takeover through the final writer.
	wtx1 := selectedOTelReportTx(t, db, b1)
	if err := (CJSIPublicationGroup{Usage: c1, Draft: d1, OTel: otel1, Receipts: rs1, RestoreOTel: false}).apply(ctx, wtx1); err != nil {
		t.Fatalf("apply ticket 1: %v", err)
	}
	if _, err := wtx1.ExecContext(ctx, `UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=? AND attempt=?`, b1.Ticket, b1.Attempt); err != nil {
		t.Fatal(err)
	}
	if err := wtx1.Commit(); err != nil {
		t.Fatal(err)
	}

	rtx1b, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	parent1 := usage.ReportSelection{CoreEpoch: epoch, Ticket: b1.Ticket, Clock: clock, Groups: []usage.ReportGroup{{Thread: thread, Stamp: d1.Stamp}}}
	if err := usage.WithPublishedReports(ctx, rtx1b, parent1, func(r *usage.PublishedReports) error {
		post1, err = r.Summary(ctx, q, "")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	rtx1b.Rollback()
	if !reflect.DeepEqual(pre1, post1) || pre1.Summary.Tokens["input_tokens"] != 17 {
		t.Fatalf("takeover parity mismatch: pre=%+v post=%+v", pre1, post1)
	}

	// Only the middle observation remains valid for restoration (3 input,
	// 1 output, and run 41).
	if _, err := db.ExecContext(ctx, `UPDATE telemetry_observations SET state='invalid',reason='revalidated' WHERE observation_key IN ('first','last')`); err != nil {
		t.Fatal(err)
	}
	// The conflicting response ID makes ticket 2 retract the proved old call.
	callKey := ledger.Key{Kind: ledger.Response, Thread: thread, ID: "r1"}
	committed, err := usage.ReadCJSICommittedCall(ctx, db, d1.Stamp, callKey)
	if err != nil {
		t.Fatalf("read committed: %v", err)
	}
	conflictBody := fmt.Sprintf(`{"timestamp":"2026-01-02T11:00:00Z","type":"token_usage_record","payload":{"thread_id":%q,"turn_id":"t","response_id":"r1","usage":{"input_tokens":17,"output_tokens":3}}}
{"timestamp":"2026-01-02T11:00:01Z","type":"token_usage_record","payload":{"thread_id":%q,"turn_id":"t","response_id":"r1","usage":{"input_tokens":99,"output_tokens":3}}}
`, thread, thread)
	c2, d2, dr2 := buildCand(2, conflictBody, []ledger.OldRow{committed.Prior}, []ledger.CommittedCall{committed})
	b2 := dr2.Basis()
	diff2 := sha256.Sum256([]byte("diff2"))
	if _, err := db.ExecContext(ctx, `UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=CAST(zeroblob(31)||x'01' AS BLOB) WHERE ticket=? AND attempt=?`, diff2[:], b2.Ticket, b2.Attempt); err != nil {
		t.Fatal(err)
	}
	rs2 := store.CJSIOTelSQLiteStore{
		Guard: func(context.Context, *sql.Tx, string) (store.CJSIOTelGuard, error) {
			return store.CJSIOTelGuard{Thread: thread, Stamp: d2.Stamp, Revision: 2}, nil
		},
		Attributions: attrs,
		Release: &store.CJSIOTelRelease{
			Thread: thread, Number: 1, Support: d1.Stamp, Current: d2.Stamp,
			Removed: []ledger.Key{callKey}, InventoryAt: clock,
		},
	}

	prepare := func(t *testing.T, tx *sql.Tx) *store.CJSIOTelChange {
		t.Helper()
		change, err := store.PrepareCJSIOTel(t.Context(), tx, thread, c2.Proposal(), rs2)
		if err != nil || change == nil {
			t.Fatalf("prepare release: change=%v, error=%v", change, err)
		}
		return change
	}
	proposal := func(g CJSIPublicationGroup) usage.ReportProposal {
		return usage.ReportProposal{Ticket: b2.Ticket, Attempt: b2.Attempt, FrozenDiff: diff2, Groups: []usage.ProposedReportGroup{usage.ProposedReportGroup(g)}}
	}
	for _, tc := range []struct {
		name       string
		restore    bool
		nilChange  bool
		extraJSONL bool
		want       error
	}{
		{"missing_change", true, true, false, store.ErrTelemetrySourceIdentityPending},
		{"restoring_change_in_takeover_order", false, false, false, store.ErrCJSIOTelPrecondition},
		{"canonical_thread_not_vacated", true, false, true, store.ErrCJSIOTelPrecondition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.extraJSONL {
				if _, err := db.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
					VALUES('extra:jsonl','codex',?,'extra','2026-01-02T11:30:00Z','m',500,0,'extra.jsonl',0)`, strings.ToUpper(thread)); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := db.Exec("DELETE FROM usage_events WHERE event_key='extra:jsonl'"); err != nil {
						t.Error(err)
					}
				})
			}
			rtx := readOTelReportTx(t, db)
			g := CJSIPublicationGroup{Usage: c2, Draft: d2, Receipts: rs2, RestoreOTel: tc.restore}
			if !tc.nilChange {
				g.OTel = prepare(t, rtx)
			}
			called := false
			_, propErr := usage.WithProposedReports(ctx, rtx, parent1, proposal(g), func(*usage.PublishedReports) error {
				called = true
				return nil
			})
			if err := rtx.Rollback(); err != nil {
				t.Fatal(err)
			}
			wtx := selectedOTelReportTx(t, db, b2)
			applyErr := g.apply(ctx, wtx)
			if !errors.Is(applyErr, tc.want) {
				t.Fatalf("release writer error=%v, want %v", applyErr, tc.want)
			}
			if !errors.Is(propErr, tc.want) || called {
				t.Fatalf("release proposal guard: error=%v, want %v; callback=%v", propErr, tc.want, called)
			}
		})
	}

	t.Run("partial_restore_parity", func(t *testing.T) {
		ctx := t.Context()
		rtx := readOTelReportTx(t, db)
		g := CJSIPublicationGroup{Usage: c2, Draft: d2, OTel: prepare(t, rtx), Receipts: rs2, RestoreOTel: true}
		type report struct {
			all     usage.ReportSummary
			session usage.SessionSummary
		}
		read := func(v *usage.PublishedReports, result *report) error {
			var err error
			if result.all, err = v.Summary(ctx, q, ""); err != nil {
				return err
			}
			result.session, err = v.SessionUsage(ctx, "codex", thread)
			return err
		}
		var proposed, opened report
		if _, err := usage.WithProposedReports(ctx, rtx, parent1, proposal(g), func(v *usage.PublishedReports) error {
			return read(v, &proposed)
		}); err != nil {
			t.Fatalf("partial restore proposal: %v", err)
		}
		if err := rtx.Rollback(); err != nil {
			t.Fatal(err)
		}
		wtx := selectedOTelReportTx(t, db, b2)
		if err := g.apply(ctx, wtx); err != nil {
			t.Fatalf("apply partial restore: %v", err)
		}
		if _, err := wtx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=? AND attempt=?", b2.Ticket, b2.Attempt); err != nil {
			t.Fatal(err)
		}
		if err := wtx.Commit(); err != nil {
			t.Fatal(err)
		}
		parent2 := usage.ReportSelection{CoreEpoch: epoch, Ticket: b2.Ticket, Clock: clock, Groups: []usage.ReportGroup{{Thread: thread, Stamp: d2.Stamp}}}
		if err := usage.WithPublishedReports(ctx, readOTelReportTx(t, db), parent2, func(v *usage.PublishedReports) error {
			return read(v, &opened)
		}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(proposed, opened) || proposed.all.Summary.Tokens["input_tokens"] != 3 || proposed.session.Tokens["output_tokens"] != 1 {
			t.Fatalf("partial restore differs: proposed=%+v, opened=%+v", proposed, opened)
		}
	})
}

func readOTelReportTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func selectedOTelReportTx(t *testing.T, db *sql.DB, b ledger.Basis) *sql.Tx {
	t.Helper()
	ctx := t.Context()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(ctx, `UPDATE cjsi_publication_head SET ticket=?,slot_ticket=?,slot_attempt=?,slot_seal=CAST(zeroblob(31)||x'01' AS BLOB);
		UPDATE cjsi_publication_attempts SET state='sealed' WHERE ticket=? AND attempt=?`, b.Ticket, b.Ticket, b.Attempt, b.Ticket, b.Attempt); err != nil {
		t.Fatal(err)
	}
	return tx
}
