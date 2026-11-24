package usage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// In-memory transport for synthetic rows only. C1 verifies both ordered
// passes against the actual source and the selection captured before replay.
type ledgerWorkset struct {
	rows []struct {
		r ingest.CodexRecord
		e ingest.CodexCheckpointEvidence
	}
}

func (w *ledgerWorkset) Abort() { w.rows = nil }
func (w *ledgerWorkset) StageCodexRecord(_ context.Context, r ingest.CodexRecord, e ingest.CodexCheckpointEvidence) error {
	w.rows = append(w.rows, struct {
		r ingest.CodexRecord
		e ingest.CodexCheckpointEvidence
	}{r, e})
	return nil
}
func (w *ledgerWorkset) OpenCodexWorkset(context.Context) (ingest.CodexWorksetNext, error) {
	i := 0
	return func(context.Context) (ingest.CodexRecord, ingest.CodexCheckpointEvidence, error) {
		if i == len(w.rows) {
			return ingest.CodexRecord{}, ingest.CodexCheckpointEvidence{}, io.EOF
		}
		row := w.rows[i]
		i++
		return row.r, row.e, nil
	}, nil
}

const ledgerThread = "00000000-0000-4000-8000-000000000002"
const ledgerHead = `{"type":"session_meta","payload":{"id":"` + ledgerThread + `"}}` + "\n"
const ledgerTurn = `{"type":"turn_context","payload":{"turn_id":"t","model":"m"}}` + "\n"

func ledgerNative(id, usage, extra string) string {
	return fmt.Sprintf(`{"timestamp":"2026-01-02T10:00:00Z","type":"token_usage_record","payload":{"thread_id":"%s","turn_id":"t","response_id":"%s","usage":%s%s}}`+"\n", ledgerThread, id, usage, extra)
}

func ledgerStage(t *testing.T, path, body string, current func(ingest.CodexSelectedSource) ingest.CodexSelectedSource, maxMeters int) (ledger.Source, ledger.SourceStamp, error) {
	t.Helper()
	ctx := t.Context()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	version := ingest.CodexCheckpointVersion{Generation: 1, Parser: ledger.Parser, MaxLineBytes: 8192}
	n, complete := int64(len(body)), int64(strings.LastIndexByte(body, '\n')+1)
	selected := ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: 7, Evidence: ingest.CodexPrefixEvidence{
		Identity: ingest.FileIdentity(info), Version: version, MaxLineBytes: version.MaxLineBytes, Digest: sha256.Sum256([]byte(body)),
		Progress: ingest.CodexReadProgress{ReadEnd: n, CompleteEnd: complete, Partial: n != complete}}}
	var w ledgerWorkset
	replay, err := ingest.BeginCodexCheckpointWorkset(ctx, f, path, n, version, nil, &w)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	for {
		step, err := replay.Step(ctx, 43)
		if err != nil {
			t.Fatal(err)
		}
		if step.Done {
			break
		}
	}
	source, err := ledger.StageWithLegacy(ctx, selected, func(context.Context) (ingest.CodexSelectedSource, error) {
		if current != nil {
			return current(selected), nil
		}
		return selected, nil
	}, replay, ingest.CodexWorksetLimits{Turns: 32, SegmentRecords: 128}, maxMeters, f, legacyUsageDecoder(path, maxMeters))
	return source, ledger.SelectionStamp(selected), err
}

func ledgerInput(t *testing.T, bodies ...string) ledger.ReplayInput {
	t.Helper()
	in := ledger.ReplayInput{Basis: ledger.Basis{Ticket: 4, Attempt: 2, BaseTicket: 3, CoreEpoch: 1, InputDigest: sha256.Sum256([]byte("synthetic-base"))},
		Thread: ledgerThread, History: ledger.History{Assessed: true}}
	for i, body := range bodies {
		s, stamp, err := ledgerStage(t, filepath.Join(t.TempDir(), fmt.Sprintf("copy-%d.jsonl", i)), body, nil, 256)
		if err != nil {
			t.Fatal(err)
		}
		in.Sources = append(in.Sources, s)
		in.Expected = append(in.Expected, stamp)
	}
	return in
}

func ledgerView(t *testing.T, in ledger.ReplayInput) (ledger.Proposal, ledger.Eligibility) {
	t.Helper()
	p, err := ledger.Replay(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Eligibility(p.Stamp(), in.Thread)
	if err != nil {
		t.Fatal(err)
	}
	return p, v
}

func TestCJSIUsageLedgerReplay(t *testing.T) {
	t.Run("global residual veto", testGlobalResidualVeto)
	t.Run("exact across selected aliases", testAliasedRunConflict)
	t.Run("optional interval detail", testResidualDetails)
	t.Run("contained request models", testContainedModels)
	t.Run("opened response history", testOpenedHistory)
	t.Run("WAL writer serialization", testStageWriterLock)
	t.Run("counter discontinuities", testCounterDiscontinuities)
	t.Run("overlapping old keys", testLegacyRangeOverlap)
	t.Run("whole interval model", testIntervalModels)
	t.Run("rewritten legacy turn", testLegacyTurnReplacement)
	t.Run("exact run overlap", testPartialRunOverlap)
	t.Run("final writer selection and rollback", testSelectedWriter)
	t.Run("missing source retention", testLedgerMissing)
	t.Run("identified and token-only calls", testIdlessLedger)
	t.Run("resume authenticated workset within budget", testBudgetedReplay)
	t.Run("mixed notifications", testMixedLedger)
	t.Run("stable response conflicts and copies", testLedgerCopies)
	t.Run("registered old selection and epoch", testLedgerDraftSelection)
	t.Run("publication eligibility", func(t *testing.T) {
		body := ledgerHead + ledgerTurn + ledgerNative("r:Case", `{"input_tokens":9007199254740993,"output_tokens":2,"cached_input_tokens":0}`, "")
		in := ledgerInput(t, body+"incomplete")
		p, v := ledgerView(t, in)
		if v.State != ledger.LocalCalls || v.Pending || !v.Partial || len(v.Members) != 1 {
			t.Fatalf("state=%v pending=%t partial=%t members=%d", v.State, v.Pending, v.Partial, len(v.Members))
		}
		m := v.Members[0]
		if m.Key != (ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: "r:Case"}) || m.Tokens.Input != 9007199254740993 ||
			m.Tokens.Known != ledger.CachedKnown || m.Proof[0].Source != in.Expected[0] || m.Proof[0].Span != (ingest.CodexSpan{Start: int64(len(ledgerHead + ledgerTurn)), End: int64(len(body))}) {
			t.Fatalf("incorrect request evidence: %+v", m)
		}
		wrong := p.Stamp()
		wrong.Basis.Attempt++
		if _, err := p.Eligibility(wrong, ledgerThread); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("accepted stale attempt: %v", err)
		}
		if _, err := p.Eligibility(p.Stamp(), "another-thread"); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("accepted other thread: %v", err)
		}
		v.Members[0].Proof[0] = ledger.Reference{}
		again, _ := p.Eligibility(p.Stamp(), ledgerThread)
		if again.Members[0].Proof[0].Source != in.Expected[0] {
			t.Fatal("consumer mutated sealed proof")
		}
		in.Sources = nil
		if _, err := ledger.Replay(t.Context(), in); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("omitted source: %v", err)
		}
	})
	t.Run("header or old overlap cannot authorize takeover", func(t *testing.T) {
		in := ledgerInput(t, ledgerHead)
		_, v := ledgerView(t, in)
		if v.State != ledger.NoLocalCall || !v.Pending || !v.Partial || len(v.Members) != 0 {
			t.Fatalf("head authorized %+v", v)
		}
		in = ledgerInput(t, ledgerHead+ledgerTurn+ledgerNative("r", `{"input_tokens":9,"output_tokens":1}`, ""))
		old := ledger.OldRow{EventKey: "original", RowHash: sha256.Sum256([]byte("original-preimage"))}
		in.History.Rows = []ledger.OldRow{old}
		p, v := ledgerView(t, in)
		d, err := p.Changes(p.Stamp(), in.Thread)
		if err != nil || len(d.Retract) != 0 || len(d.Hold) != 1 || d.Hold[0] != old || !v.Pending || !v.UnresolvedOverlap || len(v.Members) != 0 {
			t.Fatalf("old safety: %+v %+v %v", v, d, err)
		}
	})
	t.Run("stale source and cancellation discard staging", func(t *testing.T) {
		body := ledgerHead + ledgerTurn + ledgerNative("r", `{"input_tokens":8,"output_tokens":2}`, "")
		_, _, err := ledgerStage(t, filepath.Join(t.TempDir(), "copy.jsonl"), body, func(s ingest.CodexSelectedSource) ingest.CodexSelectedSource { s.Epoch++; return s }, 8)
		if !errors.Is(err, ingest.ErrSourceChanged) {
			t.Fatalf("accepted stale selection: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err = ledger.Replay(ctx, ledger.ReplayInput{}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}
