package usage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func requireRejectedReport(t *testing.T, tx *sql.Tx, s ReportSelection, p ReportProposal, want error, reason string) {
	t.Helper()
	called := false
	impact, err := WithProposedReports(t.Context(), tx, s, p, func(*PublishedReports) error {
		called = true
		return nil
	})
	if !errors.Is(err, want) || called || impact.plan != nil {
		t.Fatalf("%s: proposed error=%v, want %v; callback=%v, plan=%v", reason, err, want, called, impact.plan)
	}
}

func newReportGuardFixture(t *testing.T) (replayDatabase, ReportSelection) {
	t.Helper()
	r := newReplayDatabase(t)
	intervalFixtureExec(t, r.db, "DROP TABLE cjsi_usage_intervals")
	installIntervalTables(t, r.db, "cjsi_core_facts_schema.go", "cjsi_usage_intervals", "cjsi_usage_interval_dirty_ranges")
	return r, ReportSelection{CoreEpoch: r.epoch, Groups: []ReportGroup{}, Clock: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)}
}

func (r replayDatabase) installReportPublicationColumns(t *testing.T) {
	t.Helper()
	intervalFixtureExec(t, r.db, `ALTER TABLE cjsi_publication_attempts ADD COLUMN session_seal;
		ALTER TABLE cjsi_publication_head ADD COLUMN slot_ticket;
		ALTER TABLE cjsi_publication_head ADD COLUMN slot_attempt`)
}

func (r replayDatabase) frozenReportProposal(t *testing.T, body string) ReportProposal {
	t.Helper()
	c, d := r.candidate(t, 1, body)
	r.installReportPublicationColumns(t)
	return r.freezeReportGroup(t, c, d, [32]byte{77})
}

func (r replayDatabase) freezeReportGroup(t *testing.T, c CJSIUsageCandidate, d CJSIUsageDraft, diff [32]byte) ReportProposal {
	t.Helper()
	if err := c.Stage(t.Context(), r.db, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }); err != nil {
		t.Fatal(err)
	}
	b := d.Stamp.Basis
	intervalFixtureExec(t, r.db, "UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=zeroblob(32) WHERE ticket=? AND attempt=?", diff[:], b.Ticket, b.Attempt)
	return ReportProposal{b.Ticket, b.Attempt, diff, []ProposedReportGroup{{Usage: c, Draft: d, Receipts: emptyReportReceipts{stamp: d.Stamp}}}}
}

func (r replayDatabase) emptyReportProposal(t *testing.T) ReportProposal {
	t.Helper()
	r.installReportPublicationColumns(t)
	diff := [32]byte{55}
	input := [32]byte{1}
	intervalFixtureExec(t, r.db, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,input_digest,diff_digest,state,accept_until,index_epoch,session_revision,coverage_revision,protection_revision,session_seal)
		VALUES(1,1,0,?,?,?,'draft','2099-01-01T00:00:00Z',0,0,0,0,zeroblob(32))`, r.epoch, input[:], diff[:])
	return ReportProposal{1, 1, diff, []ProposedReportGroup{}}
}

func (r replayDatabase) withSelectedReport(t *testing.T, p ReportProposal, fn func(*sql.Tx)) {
	t.Helper()
	tx := reportGuardWriteTx(t, r.db)
	defer tx.Rollback()
	if err := selectReportFixture(t.Context(), tx, p); err != nil {
		t.Fatal(err)
	}
	fn(tx)
}

func selectReportFixture(ctx context.Context, tx *sql.Tx, p ReportProposal) error {
	if _, err := tx.ExecContext(ctx, "UPDATE cjsi_publication_head SET ticket=?,slot_ticket=?,slot_attempt=?", p.Ticket, p.Ticket, p.Attempt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='sealed' WHERE ticket=? AND attempt=?", p.Ticket, p.Attempt); err != nil {
		return err
	}
	for _, g := range p.Groups {
		if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO cjsi_usage_selection VALUES(?,?,?)", g.Draft.Thread, p.Ticket, p.Attempt); err != nil {
			return err
		}
	}
	return nil
}

func reportGuardWriteTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestReportRejectsUnregisteredExistingEventKey(t *testing.T) {
	ctx := t.Context()
	r, parent := newReportGuardFixture(t)
	p := r.frozenReportProposal(t, ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10)))
	g := p.Groups[0]
	key := (ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: "R"}).EventKey()
	intervalFixtureExec(t, r.db, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
		VALUES(?,'codex',?,'R','2026-01-02T10:00:00Z','m',999,0,'unregistered',0)`, key, ledgerThread)

	tx := borrowIntervalFixture(t, r.db)
	requireRejectedReport(t, tx, parent, p, ledger.ErrProposalMismatch, "unregistered existing event key")
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	r.withSelectedReport(t, p, func(tx *sql.Tx) {
		if err := g.Usage.ApplySelectedRequests(ctx, tx, g.Draft.Stamp, g.Draft.ExpectedOldRows); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("unregistered existing event key: writer error=%v, want %v", err, ledger.ErrProposalMismatch)
		}
	})
}

func TestReportEmptyGroupsRequireCurrentAttempt(t *testing.T) {
	r, parent := newReportGuardFixture(t)
	p := r.emptyReportProposal(t)
	for _, tc := range []struct{ name, update string }{
		{"expired", "UPDATE cjsi_publication_attempts SET accept_until='2000-01-01T00:00:00Z'"},
		{"index_epoch", "UPDATE cjsi_publication_head SET index_epoch=9"},
		{"session_revision", "UPDATE cjsi_publication_head SET session_revision=9"},
		{"coverage_revision", "UPDATE cjsi_publication_head SET coverage_revision=9"},
		{"protection_revision", "UPDATE cjsi_publication_head SET protection_revision=9"},
		{"layout", "UPDATE cjsi_layout_journal SET phase=4"},
		{"schema", "UPDATE schema_metadata SET version=36"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := reportGuardWriteTx(t, r.db)
			defer tx.Rollback()
			if _, err := tx.ExecContext(t.Context(), tc.update); err != nil {
				t.Fatal(err)
			}
			requireRejectedReport(t, tx, parent, p, ledger.ErrProposalMismatch, "invalid empty attempt")
		})
	}

	tx := borrowIntervalFixture(t, r.db)
	defer tx.Rollback()
	called := false
	if _, err := WithProposedReports(t.Context(), tx, parent, p, func(*PublishedReports) error {
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("live empty attempt: error=%v, callback=%v", err, called)
	}
}

func TestReportPublishedSelectionRequiresHead(t *testing.T) {
	r, parent := newReportGuardFixture(t)
	for _, tc := range []struct {
		name string
		edit func(*ReportSelection)
	}{
		{"wrong_core_epoch", func(s *ReportSelection) { s.CoreEpoch++ }},
		{"wrong_ticket", func(s *ReportSelection) { s.Ticket++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parent
			tc.edit(&s)
			tx := borrowIntervalFixture(t, r.db)
			defer tx.Rollback()
			called := false
			err := WithPublishedReports(t.Context(), tx, s, func(*PublishedReports) error {
				called = true
				return nil
			})
			if !errors.Is(err, ledger.ErrProposalMismatch) || called {
				t.Fatalf("published head guard: error=%v, callback=%v", err, called)
			}
		})
	}
}

func TestReportSelectedRangesRequireSealedHead(t *testing.T) {
	ctx := t.Context()
	r, parent := newReportGuardFixture(t)
	p := r.frozenReportProposal(t, ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10))+
		ledgerNative("", counter(40), `,"thread_token_usage":`+counter(50)))
	tx := borrowIntervalFixture(t, r.db)
	impact, err := WithProposedReports(ctx, tx, parent, p, func(*PublishedReports) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, plan ReportIntervalImpact, tx *sql.Tx) {
		t.Helper()
		if ranges, err := plan.SelectedRanges(ctx, tx); !errors.Is(err, ledger.ErrProposalMismatch) || ranges != nil {
			t.Fatalf("selected ranges guard: ranges=%v, error=%v", ranges, err)
		}
	}
	check(t, impact, tx)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	r.withSelectedReport(t, p, func(tx *sql.Tx) {
		if ranges, err := impact.SelectedRanges(ctx, tx); err != nil || len(ranges) == 0 {
			t.Fatalf("selected sealed attempt lost dirty ranges: %v, %v", ranges, err)
		}
		check(t, ReportIntervalImpact{}, tx)
		wrongDiff := impact
		wrongDiff.diff[0] ^= 1
		check(t, wrongDiff, tx)
		if _, err := tx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='opened'"); err != nil {
			t.Fatal(err)
		}
		check(t, impact, tx)
	})
}

func TestReportBootstrapStampRejectsResidualDirtyRanges(t *testing.T) {
	ctx := t.Context()
	r, parent := newReportGuardFixture(t)
	p := r.emptyReportProposal(t)
	intervalFixtureExec(t, r.db, `INSERT INTO cjsi_usage_interval_dirty_ranges(core_epoch,scope_kind,client,model,from_day,to_day,ticket)
		VALUES(?,'all-summary','','',1,2,1)`, r.epoch)
	tx := borrowIntervalFixture(t, r.db)
	defer tx.Rollback()
	q := StatsOptions{From: parent.Clock.Add(-time.Hour), To: parent.Clock, Timezone: "UTC", Location: time.UTC}
	called := false
	impact, err := WithProposedReports(ctx, tx, parent, p, func(v *PublishedReports) error {
		called = true
		_, err := v.AggregateCacheStamp(ctx, q, [32]byte{1})
		return err
	})
	if !called || !errors.Is(err, ledger.ErrProposalMismatch) || impact.plan != nil {
		t.Fatalf("bootstrap dirty range guard: error=%v, callback=%v, plan=%v", err, called, impact.plan)
	}
}

func TestReportRequestsValidateCheckpoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*CJSIUsageCheckpoint)
	}{
		{"wrong_thread", func(c *CJSIUsageCheckpoint) { c.Workset.Selected.ThreadID = "other" }},
		{"wrong_read_end", func(c *CJSIUsageCheckpoint) { c.Workset.Selected.Evidence.Progress.ReadEnd++ }},
		{"wrong_complete_end", func(c *CJSIUsageCheckpoint) { c.Workset.Selected.Evidence.Progress.CompleteEnd++ }},
		{"wrong_file_size", func(c *CJSIUsageCheckpoint) { c.Entry.Size++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			r, parent := newReportGuardFixture(t)
			// Each case starts from a newly staged candidate; only the private
			// checkpoint is changed after staging.
			p := r.frozenReportProposal(t, ledgerNative("R", counter(10), ""))
			g := &p.Groups[0]
			tc.edit(&g.Usage.checkpoints[0])

			tx := borrowIntervalFixture(t, r.db)
			requireRejectedReport(t, tx, parent, p, ledger.ErrProposalMismatch, "invalid checkpoint")
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			r.withSelectedReport(t, p, func(tx *sql.Tx) {
				if err := g.Usage.ApplySelectedRequests(ctx, tx, g.Draft.Stamp, g.Draft.ExpectedOldRows); !errors.Is(err, ledger.ErrProposalMismatch) {
					t.Fatalf("invalid checkpoint: writer error=%v, want %v", err, ledger.ErrProposalMismatch)
				}
			})
		})
	}
}

func TestReportRequestsRejectResidualOTelWithIntervals(t *testing.T) {
	ctx := t.Context()
	r, parent := newReportGuardFixture(t)
	p := r.frozenReportProposal(t, ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10))+
		ledgerNative("", counter(40), `,"thread_token_usage":`+counter(50)))
	g := p.Groups[0]
	v, err := g.Usage.Proposal().Eligibility(g.Draft.Stamp, ledgerThread)
	if err != nil || v.Gaps&ledger.OldSideHeld != 0 || len(v.Members) != 2 || v.Members[1].Key.Kind != ledger.TokenInterval {
		t.Fatalf("test requires an eligible request and interval: %+v, %v", v, err)
	}
	delta, err := g.Usage.Proposal().Changes(g.Draft.Stamp, ledgerThread)
	if err != nil {
		t.Fatal(err)
	}

	// The receipt fixture selects a no-op. A live row the usage writer
	// still sees must prevent it from publishing any JSONL members.
	insertResidual := func(t *testing.T, tx *sql.Tx) {
		t.Helper()
		if _, err := tx.ExecContext(t.Context(), `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset)
			VALUES('otel:left','codex',?,'left','2026-01-02T10:00:00Z','m','otel:left',0)`, ledgerThread); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("staged_request_and_interval", func(t *testing.T) {
		tx := reportGuardWriteTx(t, r.db)
		defer tx.Rollback()
		insertResidual(t, tx)
		requireRejectedReport(t, tx, parent, p, ledger.ErrProposalMismatch, "request and interval with residual OTel")
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		r.withSelectedReport(t, p, func(tx *sql.Tx) {
			insertResidual(t, tx)
			if err := g.Usage.ApplySelectedRequests(ctx, tx, g.Draft.Stamp, g.Draft.ExpectedOldRows); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatalf("request and interval writer with residual OTel: got %v, want %v", err, ledger.ErrProposalMismatch)
			}
		})
	})

	t.Run("only_interval_members", func(t *testing.T) {
		// Replay emits intervals alongside a request. Exercise the defensive
		// check after verification with only the original interval member.
		v.Members = v.Members[1:]
		r.withSelectedReport(t, p, func(tx *sql.Tx) {
			projected := newProposedFacts(tx)
			if err := projected.requests(ctx, g, v); err != nil {
				t.Fatalf("clean interval-only projection: %v", err)
			}
			old, err := g.Usage.readOldRows(ctx, tx, g.Draft.ExpectedOldRows)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.Usage.applyVerifiedRequests(ctx, tx, v, delta, old); err != nil {
				t.Fatalf("clean interval-only writer: %v", err)
			}
			insertResidual(t, tx)
			projected = newProposedFacts(tx)
			if err := projected.requests(ctx, g, v); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatalf("interval-only proposed guard: got %v, want %v", err, ledger.ErrProposalMismatch)
			}
			if err := g.Usage.applyVerifiedRequests(ctx, tx, v, delta, old); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatalf("interval-only writer guard: got %v, want %v", err, ledger.ErrProposalMismatch)
			}
		})
	})
}

func TestReportRequestReplacementRequiresMatchingMetadata(t *testing.T) {
	id := cjsiRequestIdentity{key: "registered", id: "R"}
	prior := ledger.OldImage{EventKey: id.key, Client: "codex", SessionID: ledgerThread, EventID: id.id}
	if err := id.checkReplacement(ledgerThread, map[string]ledger.OldImage{id.key: prior}, nil); err != nil {
		t.Fatalf("registered replacement: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*ledger.OldImage)
	}{
		{"client", func(i *ledger.OldImage) { i.Client = "claude" }},
		{"session_id", func(i *ledger.OldImage) { i.SessionID = "other-thread" }},
		{"event_id", func(i *ledger.OldImage) { i.EventID = "other-event" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := prior
			tc.edit(&bad)
			if err := id.checkReplacement(ledgerThread, map[string]ledger.OldImage{id.key: bad}, nil); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatalf("replacement metadata guard: got %v, want %v", err, ledger.ErrProposalMismatch)
			}
		})
	}
}
