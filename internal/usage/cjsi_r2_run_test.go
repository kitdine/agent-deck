package usage

import (
	"database/sql"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func testAliasedRunConflict(t *testing.T) {
	ctx := t.Context()
	prefix := ledgerHead + ledgerTurn
	body := prefix + ledgerNative("R", counter(10), "") + "extra\n"
	span := ingest.CodexSpan{Start: int64(len(prefix)), End: int64(len(body) - len("extra\n"))}
	var sources []CJSIUsagePreparedSource
	in := ledgerInput(t)
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		path := filepath.Join(t.TempDir(), name)
		src, st, err := ledgerStage(t, path, body, nil, 16)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := usageSourceWall(path)
		if err != nil {
			t.Fatal(err)
		}
		w := CJSIUsageWorkset{Round: 1, Attempt: int64(len(sources) + 1), Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: st.Epoch, Evidence: st.Prefix}}
		sources = append(sources, CJSIUsagePreparedSource{entry: entry, binding: w, source: src})
		in.Expected = append(in.Expected, st)
	}
	full := ledger.RunRange{RunID: 1, Completed: true, Exact: true, OriginalPath: sources[0].entry.Path, Source: in.Expected[0], Bytes: span, StartHash: "start1", EndHash: "end1"}
	partial := ledger.RunRange{RunID: 2, Completed: true, Exact: true, OriginalPath: sources[1].entry.Path, Source: in.Expected[1], Bytes: ingest.CodexSpan{Start: span.Start + 5, End: span.End}, StartHash: "start2", EndHash: "end2"}
	overrun := full
	overrun.Bytes.End = overrun.Source.Prefix.Progress.CompleteEnd + 1
	boundary := full
	boundary.Bytes.End = boundary.Source.Prefix.Progress.CompleteEnd
	before, after, empty := partial, partial, partial
	before.Bytes = ingest.CodexSpan{Start: 0, End: span.Start}
	after.Bytes = ingest.CodexSpan{Start: span.End, End: int64(len(body))}
	empty.Bytes.End = empty.Bytes.Start
	sameRun, pending := partial, partial
	sameRun.RunID = 1
	pending.Completed = false
	left := partial
	left.Bytes = ingest.CodexSpan{Start: 0, End: span.Start + 1}
	for _, tc := range []struct {
		name string
		runs []ledger.RunRange
		want int64
	}{
		{"exactly complete prefix", []ledger.RunRange{boundary}, 1},
		{"sole witness past complete prefix", []ledger.RunRange{overrun}, 0},
		{"cover", []ledger.RunRange{full}, 1}, {"partial only", []ledger.RunRange{partial}, 0},
		{"partial alias", []ledger.RunRange{full, partial}, 0}, {"left overlap", []ledger.RunRange{left, full}, 0},
		{"same run", []ledger.RunRange{sameRun, full}, 1}, {"pending", []ledger.RunRange{full, pending}, 1},
		{"half-open adjacency", []ledger.RunRange{before, full, after, empty}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				in.History.Runs = slices.Clone(tc.runs)
				if reverse {
					slices.Reverse(in.History.Runs)
				}
				c, err := PrepareCJSIUsageCandidate(ctx, in, sources)
				if err != nil {
					t.Fatal(err)
				}
				v, err := c.Proposal().Eligibility(c.Proposal().Stamp(), ledgerThread)
				if err != nil || len(v.Members) != 1 || v.Members[0].ExactRun != tc.want || len(v.Members[0].Proof) != 2 {
					t.Fatal(v, err)
				}
			}
		})
	}
	db := newReplayDatabase(t)
	in.Basis.CoreEpoch = db.epoch
	key := (ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: "R"}).EventKey()
	for _, tc := range []struct {
		name                      string
		known, late, moved, stale bool
		wantExact                 int64
		wantBinding               bool
	}{
		{name: "independent covering run", wantExact: 1, wantBinding: true},
		{name: "selected sibling conflict", known: true},
		{name: "late sibling conflict", late: true, wantExact: 1},
		{name: "late pre-move path conflict", late: true, moved: true, wantExact: 1},
		{name: "vanished covering witness", stale: true, wantExact: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.History.Runs = []ledger.RunRange{full}
			siblingPath := partial.OriginalPath
			oversized := full
			oversized.Source = in.Expected[1]
			oversized.OriginalPath = siblingPath
			oversized.Bytes.End = int64(len(body)) + 1
			if tc.stale {
				in.History.Runs = append(in.History.Runs, oversized)
			}
			if tc.known {
				in.History.Runs = append(in.History.Runs, partial)
			}
			if tc.moved {
				retained := full
				retained.Source = in.Expected[1]
				retained.OriginalPath = filepath.Join(t.TempDir(), "pre-move.jsonl")
				siblingPath = retained.OriginalPath
				in.History.Runs = append(in.History.Runs, retained)
			}
			c, err := PrepareCJSIUsageCandidate(ctx, in, sources)
			if err != nil {
				t.Fatal(err)
			}
			st := c.Proposal().Stamp()
			v, err := c.Proposal().Eligibility(st, ledgerThread)
			if err != nil || len(v.Members) != 1 || v.Members[0].ExactRun != tc.wantExact {
				t.Fatal(v, err)
			}
			tx, err := db.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			exec := func(q string, args ...any) {
				t.Helper()
				if _, err := tx.ExecContext(ctx, q, args...); err != nil {
					t.Fatal(err)
				}
			}
			exec("UPDATE cjsi_publication_head SET ticket=?", st.Basis.Ticket)
			exec("INSERT INTO cjsi_usage_selection(thread,ticket,attempt) VALUES(?,?,?)", ledgerThread, st.Basis.Ticket, st.Basis.Attempt)
			exec("INSERT INTO cjsi_usage_groups(thread,ticket,attempt,input_digest,diff_digest) VALUES(?,?,?,?,?)", ledgerThread, st.Basis.Ticket, st.Basis.Attempt, st.Basis.InputDigest[:], st.Diff[:])
			exec(`INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason)
   VALUES(1,'codex','p1','1','2026-01-02T09:00:00Z','2026-01-02T11:00:00Z',1,''),
   (2,'codex','p2','2','2026-01-02T09:00:00Z','2026-01-02T11:00:00Z',1,'')`)
			const insertRange = "INSERT INTO usage_run_sources(run_id,path,start_offset,end_offset,start_hash,end_hash) VALUES(?,?,?,?,?,?)"
			if tc.stale {
				exec(insertRange, 1, siblingPath, oversized.Bytes.Start, oversized.Bytes.End, oversized.StartHash, oversized.EndHash)
			} else {
				exec(insertRange, 1, full.OriginalPath, span.Start, span.End, full.StartHash, full.EndHash)
			}
			if tc.moved {
				exec(insertRange, 1, siblingPath, span.Start, span.End, full.StartHash, full.EndHash)
			}
			if tc.known || tc.late {
				exec(insertRange, 2, siblingPath, partial.Bytes.Start, partial.Bytes.End, partial.StartHash, partial.EndHash)
			}
			if err := c.ApplySelectedRequests(ctx, tx, st, []ledger.OldRow{}); err != nil {
				t.Fatal(err)
			}
			var bound sql.NullInt64
			err = tx.QueryRowContext(ctx, "SELECT b.run_id FROM usage_events e LEFT JOIN usage_run_bindings b USING(event_key) WHERE e.event_key=? AND e.input_tokens=10", key).Scan(&bound)
			if err != nil || bound.Valid != tc.wantBinding || bound.Valid && bound.Int64 != 1 {
				t.Fatalf("live binding %+v: %v", bound, err)
			}
		})
	}
}
