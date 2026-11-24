package usage

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func testBudgetedReplay(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "selected.jsonl")
	body := ledgerHead + ledgerTurn + ledgerNative("result", counter(3), "") + "partial"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := usageSourceWall(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	version := ingest.CodexCheckpointVersion{Generation: 3, Parser: ledger.Parser, MaxLineBytes: 4096}
	meta := &ledgerWorkset{}
	capture, err := ingest.BeginCodexCheckpointWorkset(ctx, f, path, entry.Size, version, nil, meta)
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Abort()
	step, err := capture.Step(ctx, entry.Size)
	if err != nil || !step.Done {
		t.Fatal(step, err)
	}
	b := CJSIUsageWorkset{Round: 4, Attempt: 2, Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: 2, Evidence: ingest.CodexPrefixEvidence{Identity: entry.Identity, Progress: step.Progress, Version: version, MaxLineBytes: version.MaxLineBytes, Digest: sha256.Sum256([]byte(body))}}}
	current := func(context.Context) (CJSIUsageWorkset, error) { return b, nil }
	open := func(ctx context.Context) (CJSIUsageWorkset, ingest.CodexWorksetNext, error) {
		next, err := meta.OpenCodexWorkset(ctx)
		return b, next, err
	}
	begin := func() *CJSIUsageSourceReplay {
		r, err := BeginCJSIUsageSourceReplay(ctx, entry, current, open, ingest.CodexWorksetLimits{Turns: 4, SegmentRecords: 16}, 8)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		return r
	}
	r := begin()
	run := func(r *CJSIUsageSourceReplay) CJSIUsagePreparedSource {
		var consumed int64
		for consumed < entry.Size {
			source, n, done, err := r.Step(ctx, 17)
			if err != nil || n <= 0 || n > 17 {
				t.Fatalf("step: %d %t %v", n, done, err)
			}
			consumed += n
			if done {
				if consumed != entry.Size || source.binding != b {
					t.Fatal("premature or wrong candidate")
				}
				return source
			}
			if source.binding.Round != 0 {
				t.Fatal("unfinished source escaped")
			}
		}
		t.Fatal("no terminal step")
		return CJSIUsagePreparedSource{}
	}
	a := run(r)
	again := run(begin())
	in := ledger.ReplayInput{Thread: ledgerThread, Basis: ledger.Basis{Ticket: 3, Attempt: 1, BaseTicket: 2, CoreEpoch: 1, InputDigest: sha256.Sum256([]byte("selected"))}, Expected: []ledger.SourceStamp{ledger.SelectionStamp(b.Selected)}, History: ledger.History{Assessed: true}}
	ca, err := PrepareCJSIUsageCandidate(ctx, in, []CJSIUsagePreparedSource{a})
	if err != nil {
		t.Fatal(err)
	}
	cb, err := PrepareCJSIUsageCandidate(ctx, in, []CJSIUsagePreparedSource{again})
	if err != nil || ca.Proposal().Stamp() != cb.Proposal().Stamp() {
		t.Fatal("restart changed facts", err)
	}
	p := ca.Checkpoints()[0].Progress
	if !p.Partial || p.CompleteEnd != entry.Size-7 {
		t.Fatal("unsafe checkpoint", p)
	}
	if err := os.WriteFile(path, []byte(body+"rewrite"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.Step(ctx, 1); !errors.Is(err, errUsageSourceChanged) {
		t.Fatal("reused changed source", err)
	}
}
