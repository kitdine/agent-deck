//go:build cjsi_adapter_fixture

package scanruntime

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

type reportOutputs struct {
	summary            usage.ReportSummary
	stats              usage.StatsReport
	partial, hasSignal bool
	session            usage.SessionSummary
	signal             usage.SessionSignalSummary
	signals            []usage.SignalReport
	at                 *time.Time
	cache              desktop.CJSIReportCache
	stamp              [32]byte
}

func TestCJSIReportPublication(t *testing.T) {
	ctx := t.Context()
	root, home := t.TempDir(), t.TempDir()
	idx, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatal(err)
	}
	m, err := store.ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	clock := time.Now().UTC()
	at := clock.Format(time.RFC3339Nano)
	const thread = publicationFixtureThread
	body := fmt.Sprintf(`{"timestamp":%q,"type":"session_meta","payload":{"id":%q}}
{"type":"turn_context","payload":{"turn_id":"t","model":"m"}}
{"timestamp":%q,"type":"token_usage_record","payload":{"thread_id":%q,"turn_id":"t","response_id":"r","usage":{"input_tokens":7,"output_tokens":3},"thread_token_usage":{"input_tokens":7,"output_tokens":3}}}
{"timestamp":%q,"type":"token_usage_record","payload":{"thread_id":%q,"turn_id":"t","response_id":"","usage":{"input_tokens":103,"output_tokens":0},"thread_token_usage":{"input_tokens":110,"output_tokens":3}}}
`, at, thread, at, thread, clock.Add(time.Nanosecond).Format(time.RFC3339Nano), thread)
	if err := os.WriteFile(filepath.Join(dir, "fixture.jsonl"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	planner, inventory := fixturePublicationPlanner(t, home, ScopeBoth)
	plan, _, err := planner.Combined(ctx, CJSIRoundRequest{Root: *m, Home: home, Sources: inventory})
	if err != nil {
		t.Fatal(err)
	}
	var pre, post reportOutputs
	var selection usage.ReportSelection
	var in desktop.CJSIReportCacheInputs
	var impact usage.ReportIntervalImpact
	var slot []byte
	g := plan.Groups[0]
	b := g.Draft.Stamp.Basis
	selection = usage.ReportSelection{CoreEpoch: b.CoreEpoch, Ticket: b.BaseTicket, Clock: plan.BuiltAt, Groups: []usage.ReportGroup{}}
	q := usage.StatsOptions{From: clock.Add(-time.Hour), To: clock.Add(time.Hour), Location: time.UTC, Timezone: "UTC", GroupBy: "day", Metric: "tokens"}
	opts := []usage.SignalOptions{{From: q.From, To: q.To}}
	read := func(r *usage.PublishedReports, o *reportOutputs) error {
		var err error
		if o.summary, err = r.Summary(ctx, q, ""); err != nil {
			return err
		}
		if o.stats, o.partial, err = r.Stats(ctx, q); err != nil {
			return err
		}
		if o.session, err = r.SessionUsage(ctx, "codex", thread); err != nil {
			return err
		}
		if o.signal, o.hasSignal, err = r.SessionSignals(ctx, "codex", thread); err != nil {
			return err
		}
		if o.signals, err = r.SignalsBatch(ctx, opts); err != nil {
			return err
		}
		if o.at, err = r.Earliest(ctx); err != nil {
			return err
		}
		if o.stamp, err = r.AggregateCacheStamp(ctx, q, in.Coverage); err != nil {
			return err
		}
		o.cache, err = desktop.PrepareCJSIReportCache(ctx, r, in, nil)
		return err
	}
	apply := plan.Changes.ApplySelected
	plan.Changes.ApplySelected = func(ctx context.Context, tx *sql.Tx) error {
		if err := apply(ctx, tx); err != nil {
			return err
		}
		ranges, err := impact.SelectedRanges(ctx, tx)
		if err != nil {
			return err
		}
		return store.WriteUsageIntervalDirtyRanges(ctx, tx, b.CoreEpoch, b.Ticket, ranges)
	}
	plan.Build = func(ctx context.Context, v CJSIProposedView) (func() ([]byte, error), error) {
		in = desktop.CJSIReportCacheInputs{CoreEpoch: b.CoreEpoch, Ticket: b.Ticket, Database: v.Generation.StateIdentity, Publisher: "managed-test", Schema: v.Generation.CoreSchema, UsageParser: 1, SessionParser: 1,
			Facts: v.Digest, Sessions: v.Index.Selection().SessionSeal, Protection: [32]byte{1}, Coverage: [32]byte{2}, Prices: [32]byte{3}, Routes: [32]byte{4}, Location: time.UTC, ValidUntil: v.ValidUntil}
		p := usage.ReportProposal{Ticket: b.Ticket, Attempt: b.Attempt, FrozenDiff: v.FrozenDiff, Groups: []usage.ProposedReportGroup{usage.ProposedReportGroup(v.Groups[0])}}
		for _, change := range []func(*usage.ReportSelection, *usage.ReportProposal){
			func(s *usage.ReportSelection, _ *usage.ReportProposal) { s.Ticket++ },
			func(_ *usage.ReportSelection, p *usage.ReportProposal) { p.Ticket++ },
			func(_ *usage.ReportSelection, p *usage.ReportProposal) { p.Attempt++ },
			func(_ *usage.ReportSelection, p *usage.ReportProposal) { p.FrozenDiff[0] ^= 1 },
			func(_ *usage.ReportSelection, p *usage.ReportProposal) { p.Groups = []usage.ProposedReportGroup{} },
		} {
			s, p := selection, p
			change(&s, &p)
			if _, err := usage.WithProposedReports(ctx, v.Core, s, p, func(*usage.PublishedReports) error { return errors.New("admitted wrong proposal") }); !errors.Is(err, ledger.ErrProposalMismatch) {
				return nil, fmt.Errorf("identity rejection: %v", err)
			}
		}
		var escaped *usage.PublishedReports
		var err error
		impact, err = usage.WithProposedReports(ctx, v.Core, selection, p, func(r *usage.PublishedReports) error {
			escaped = r
			return read(r, &pre)
		})
		if err != nil {
			return nil, err
		}
		if _, err := escaped.Earliest(ctx); !errors.Is(err, context.Canceled) {
			return nil, errors.New("escaped report borrow")
		}
		if err := usage.WithPublishedReports(ctx, v.Core, selection, func(r *usage.PublishedReports) error {
			old, err := r.Summary(ctx, q, "")
			if err == nil && old.Summary.Tokens["input_tokens"] != 0 {
				return errors.New("parent observed private tokens")
			}
			return err
		}); err != nil {
			return nil, err
		}
		if n := pre.summary.Summary.Tokens["input_tokens"]; n != 110 {
			return nil, fmt.Errorf("proposed input=%d, want 110", n)
		}
		return func() ([]byte, error) {
			if err := v.Core.QueryRow("SELECT 1").Scan(new(int)); !errors.Is(err, sql.ErrTxDone) {
				return nil, errors.New("encoder ran during borrow")
			}
			return pre.cache.Encode()
		}, nil
	}
	if err := plan.Publish(ctx); err != nil {
		t.Fatal(err)
	}
	err = m.WithPublishedCore(ctx, func(tx *sql.Tx, h store.CJSIIndexStamp) error {
		selection.Ticket = h.Ticket
		selection.Groups = []usage.ReportGroup{{Thread: thread, Stamp: g.Draft.Stamp}}
		var err error
		slot, err = m.ReadServingSlot(ctx, tx, plan.BuiltAt)
		if err != nil {
			return err
		}
		return usage.WithPublishedReports(ctx, tx, selection, func(r *usage.PublishedReports) error {
			if err := read(r, &post); err != nil {
				return err
			}
			reused, err := desktop.PrepareCJSIReportCache(ctx, r, in, slot)
			if err != nil {
				return err
			}
			a, b := reused.Snapshots()
			c, d := post.cache.Snapshots()
			if !reflect.DeepEqual(a, c) || !reflect.DeepEqual(b, d) {
				return errors.New("reused snapshots differ")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := post.cache.Encode()
	if err != nil || !bytes.Equal(encoded, slot) {
		t.Fatalf("opened cache bytes differ: %v", err)
	}
	if !reflect.DeepEqual(pre, post) {
		t.Fatal("opened report fields or aggregate stamp differ from proposed")
	}
}
