//go:build cjsi_adapter_fixture

package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Exercise the existing two-capture reducer on a synthetic selected file.
// Only the coordinator-owned coordinates and opaque seals come from the test.
func stageFixturePlan(t *testing.T, f physicalFixture, selectedPath string, messages bool) indexStageCandidate {
	t.Helper()
	ctx := t.Context()
	input := string(f.prefix)
	if messages {
		raw, err := os.ReadFile("../ingest/testdata/cjsi_child.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		input = strings.NewReplacer(adapterThread, "00000000-0000-4000-8000-000000000003",
			"00000000-0000-4000-8000-000000000002", adapterThread,
			"local-user-synthetic", "A", "local-final-synthetic", "stageword").Replace(string(raw))
	}
	prepared := fixturePreparedNative(t, selectedPath, adapterThread, input)
	r := prepared.replacement
	update := r.Material
	if messages && r.Metadata.Model != "child-model" {
		t.Fatalf("local model = %q", r.Metadata.Model)
	}
	r.Sequence = sha256.Sum256([]byte("accepted sequence"))
	if messages {
		if len(update.Documents) != 2 || update.Documents[0].Text != "A" || update.Documents[1].Text != "stageword" {
			t.Fatalf("accepted documents: %+v", update.Documents)
		}
		r.DocumentIDs = []int64{1, 8}
		r.DeletedDocumentIDs = []int64{2}
		// Model an earlier verified base. Legacy ordinal alone cannot preserve
		// an ID; row 1 gains an explicit logical-record key and generation.
		span := update.Documents[0].Span
		adapterExec(t, f.private, `UPDATE cjsi_base_groups SET order_verified=1,source_kind='codex-jsonl' WHERE source_path=? AND session_id=?`, f.path, adapterThread)
		adapterExec(t, f.private, `UPDATE cjsi_base_order SET record_start=?,record_end=?,emission=0 WHERE doc_id=1`, span.Start, span.End)
		adapterExec(t, f.private, `INSERT INTO cjsi_base_ranges VALUES(?,'codex',?,4,2,?,10,3,1,4096,?,?,0,?,?)`,
			f.path, adapterThread, f.identity, r.Size, r.Size, r.Size, r.Selected.Evidence.Digest[:])
	} else if len(update.Documents) != 0 {
		t.Fatalf("expected zero accepted documents, got %v", update.Documents)
	}
	var head store.CJSIIndexStamp
	f.read(t, func(reader CJSIReader) {
		d, err := reader.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		head = d.identity.Stamp
	})
	return indexStageCandidate{Attempt: indexStageAttempt{Parent: head, Ticket: 1, Attempt: 2, SessionRevision: head.SessionRevision + 1,
		Seal: sha256.Sum256([]byte("authenticated session seal"))}, Replacements: []indexReplacement{r},
		DeletedSources: []indexSourceDelete{{"replica", "second"}},
		DeletedGroups: []indexGroupDelete{{f.path, "codex", "masked", sha256.Sum256([]byte("masked tombstone"))},
			{f.path, "codex", "old-group", sha256.Sum256([]byte("old-group tombstone"))}}}
}

func stageTx(t *testing.T, f physicalFixture) *sql.Tx {
	t.Helper()
	tx, err := f.private.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func stageScalar[T comparable](t *testing.T, db *sql.DB, want T, query string, args ...any) {
	t.Helper()
	var got T
	if err := db.QueryRowContext(t.Context(), query, args...).Scan(&got); err != nil || got != want {
		t.Fatalf("%s: got %v, want %v (err %v)", query, got, want, err)
	}
}

func assertOldStageSelection(t *testing.T, f physicalFixture, h store.CJSIIndexStamp) {
	t.Helper()
	f.read(t, func(r CJSIReader) {
		d, err := r.SelectDetail(t.Context(), "codex", adapterThread)
		if err != nil || d.identity.Stamp != h {
			t.Fatalf("old stamp: %+v, %v", d.identity.Stamp, err)
		}
		docs, p, err := r.DocumentsPage(t.Context(), d, 1, 10, true)
		if err != nil || p.Total != 3 || docLabels(docs) != "ABC" {
			t.Fatalf("old detail: %+v %+v %v", docs, p, err)
		}
		for query, want := range map[string]string{"stageword": "", "maskword": "maskword"} {
			rows, err := r.Search(t.Context(), query)
			if err != nil || docLabels(rows) != want {
				t.Fatalf("old MATCH %q: %v %v", query, rows, err)
			}
		}
	})
}

func TestCJSIPrivateIndexStage(t *testing.T) {
	for _, messages := range []bool{true, false} {
		name := "zero accepted members mask old group"
		if messages {
			name = "retained and new members"
		}
		t.Run(name, func(t *testing.T) {
			f := newPhysicalFixture(t, true, true)
			c := stageFixturePlan(t, f, f.path, messages)
			tx := stageTx(t, f)
			if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); err != nil {
				t.Fatal(err)
			}
			// The borrower did not commit the draft or allocator.
			stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_session_candidates")
			assertOldStageSelection(t, f, c.Attempt.Parent)
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			a := c.Attempt
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_session_candidates WHERE ticket=? AND attempt=? AND seal_digest=?
				AND base_seal=? AND core_epoch=? AND index_epoch=? AND base_revision=? AND session_revision=?`,
				a.Ticket, a.Attempt, a.Seal[:], a.Parent.SessionSeal[:], a.Parent.CoreEpoch, a.Parent.IndexEpoch, a.Parent.SessionRevision, a.SessionRevision)
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_groups WHERE op='replace' AND order_verified=1 AND source_path=? AND session_id=?`, f.path, adapterThread)
			stageScalar(t, f.private, 2, `SELECT count(*) FROM cjsi_candidate_groups WHERE op='delete'`)
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_sources WHERE op='delete' AND source_path='replica'`)
			r := c.Replacements[0]
			e := r.Selected.Evidence
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_ranges WHERE source_path=? AND session_id=? AND identity=? AND
				selection_epoch=? AND workset_round=? AND workset_attempt=? AND generation=? AND replay_parser=? AND max_line_bytes=?
				AND read_end=? AND complete_end=? AND replay_from=? AND published_end=? AND prefix_digest=?`,
				f.path, adapterThread, f.identity, r.Selected.Epoch, r.Round, r.WorksetAttempt, e.Version.Generation, e.Version.Parser, e.MaxLineBytes,
				e.Progress.ReadEnd, e.Progress.CompleteEnd, r.Material.Checkpoint.ReplayFrom, r.PublishedEnd, e.Digest[:])
			water := 7
			if messages {
				water = 8
				stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_order WHERE doc_id=1 AND doc_side='base' AND detail_ordinal=1`)
				stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_order WHERE doc_id=8 AND doc_side='candidate' AND detail_ordinal=2`)
				stageScalar(t, f.private, 8, `SELECT rowid FROM cjsi_candidate_documents WHERE cjsi_candidate_documents MATCH 'text:stageword'`)
				stageScalar(t, f.private, 0, `SELECT count(*) FROM cjsi_candidate_payloads WHERE doc_id=1`)
			}
			stageScalar(t, f.private, len(r.Material.Documents), "SELECT count(*) FROM cjsi_candidate_order")
			stageScalar(t, f.private, len(r.DeletedDocumentIDs), "SELECT count(*) FROM cjsi_candidate_doc_masks WHERE doc_id=2 AND source_path=?", f.path)
			stageScalar(t, f.private, water-7, "SELECT count(*) FROM cjsi_candidate_payloads")
			// The frozen draft-insert trigger rejects all occupied keys before
			// allocating anything. An exact retransmission never mutates its seal.
			again := stageTx(t, f)
			err := stageIndexCandidate(t.Context(), again, a, c)
			if err == nil || !strings.Contains(err.Error(), "candidate must be a new draft") || !errors.Is(again.Commit(), sql.ErrTxDone) {
				t.Fatalf("repeat: %v", err)
			}
			stageScalar(t, f.private, water, "SELECT doc_id_high_water FROM cjsi_index_state")
			stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_session_candidates WHERE seal_digest=?", a.Seal[:])
			assertOldStageSelection(t, f, a.Parent)
		})
	}
}

func TestCJSIPrivateIndexStageRollback(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, true)
	for _, tt := range []struct {
		name   string
		change func(*indexStageCandidate)
	}{
		{"mismatched admitted attempt", func(c *indexStageCandidate) { c.Attempt.Attempt++ }},
		{"mismatched admitted ticket", func(c *indexStageCandidate) { c.Attempt.Ticket++ }},
		{"mismatched proposed revision", func(c *indexStageCandidate) { c.Attempt.SessionRevision++ }},
		{"stale compacted revision", func(c *indexStageCandidate) { c.Attempt.Parent.SessionRevision++ }},
		{"stale epoch", func(c *indexStageCandidate) { c.Attempt.Parent.IndexEpoch++ }},
		{"stale protection revision", func(c *indexStageCandidate) { c.Attempt.Parent.ProtectionRevision++ }},
		{"missing accepted ID", func(c *indexStageCandidate) { c.Replacements[0].DocumentIDs = []int64{1} }},
		{"wrong base owner", func(c *indexStageCandidate) { c.Replacements[0].DocumentIDs = []int64{5, 8} }},
		{"reuse retired ID", func(c *indexStageCandidate) { c.Replacements[0].DocumentIDs = []int64{1, 3} }},
		{"invalid ordinal", func(c *indexStageCandidate) { c.Replacements[0].Material.Documents[1].Ordinal = 1 }},
		{"overlapping span", func(c *indexStageCandidate) {
			r := &c.Replacements[0]
			r.Material.Documents[1].Span.Start = r.Material.Documents[0].Span.Start
		}},
		{"mismatched owner", func(c *indexStageCandidate) { c.Replacements[0].Material.Documents[1].SessionID = "other" }},
		{"mismatched range", func(c *indexStageCandidate) { c.Replacements[0].Material.Checkpoint.CompleteEnd-- }},
		{"unproved publication boundary", func(c *indexStageCandidate) { c.Replacements[0].PublishedEnd-- }},
		{"mismatched generation", func(c *indexStageCandidate) { c.Replacements[0].Selected.Evidence.Version.Generation++ }},
		{"unaccounted sibling", func(c *indexStageCandidate) { c.DeletedGroups = c.DeletedGroups[:1] }},
		{"alien doc tombstone", func(c *indexStageCandidate) { c.Replacements[0].DeletedDocumentIDs = []int64{5} }},
		{"late seal rejection", func(c *indexStageCandidate) { c.Replacements[0].DeletedDocumentIDs = []int64{1} }},
		{"late SQLite write rejection", func(c *indexStageCandidate) { c.Replacements = append(c.Replacements, c.Replacements[0]) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			c.Replacements = slices.Clone(base.Replacements)
			c.Replacements[0].Material.Documents = slices.Clone(base.Replacements[0].Material.Documents)
			tt.change(&c)
			admitted := base.Attempt
			if strings.HasPrefix(tt.name, "stale ") {
				admitted = c.Attempt
			}
			tx := stageTx(t, f)
			err := stageIndexCandidate(t.Context(), tx, admitted, c)
			if err == nil {
				t.Fatal("invalid candidate accepted")
			}
			if tt.name == "late SQLite write rejection" {
				// The second insertion of the same source fails only AFTER the
				// first replacement's payload, FTS, order and range were written.
				var se *sqlite.Error
				if !errors.As(err, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
					t.Fatalf("want SQLITE_CONSTRAINT_PRIMARYKEY, got %v", err)
				}
			}
			if tt.name == "late seal rejection" {
				var se *sqlite.Error
				if !errors.As(err, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_TRIGGER || !strings.Contains(err.Error(), "invalid candidate detail mapping") {
					t.Fatalf("expected frozen membership guard to reject the final seal, got %v", err)
				}
			}
			if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("failed borrower left transaction committable: %v", err)
			}
			for _, table := range []string{"session_candidates", "candidate_sources", "candidate_groups", "candidate_doc_masks", "candidate_order", "candidate_ranges", "candidate_payloads"} {
				stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_"+table)
			}
			stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_candidate_documents WHERE cjsi_candidate_documents MATCH 'stageword'")
			stageScalar(t, f.private, 7, "SELECT doc_id_high_water FROM cjsi_index_state")
		})
	}
	assertOldStageSelection(t, f, base.Attempt.Parent)
	// A current protection still vetoes the member even with matching revision.
	adapterExec(t, f.private, `INSERT INTO cjsi_session_protections VALUES(?,'codex',?,1)`, f.path, adapterThread)
	base.Attempt.Parent.ProtectionRevision++
	tx := stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, base.Attempt, base); !errors.Is(err, errIndexStage) {
		t.Fatalf("protected member: %v", err)
	}
	stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_session_candidates")
}

func TestCJSIPrivateIndexStageMove(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	newPath := filepath.Join(t.TempDir(), filepath.Base(f.path))
	if err := os.Rename(f.path, newPath); err != nil {
		t.Fatal(err)
	}
	c := stageFixturePlan(t, f, newPath, true)
	c.DeletedSources = append(c.DeletedSources, indexSourceDelete{f.path, f.identity})
	// A fresh replacement cannot silently claim a tombstoned source identity.
	tx := stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
		t.Fatalf("implicit move: %v", err)
	}
	c.Replacements[0].From = f.path
	tx = stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_order o JOIN cjsi_base_order b USING(doc_id)
		WHERE o.source_path=? AND b.source_path=? AND o.doc_side='base' AND o.doc_id=1`, newPath, f.path)
	stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_sources WHERE source_path=? AND op='delete'`, f.path)
	stageScalar(t, f.private, 0, `SELECT count(*) FROM cjsi_candidate_payloads WHERE doc_id=1`)
	assertOldStageSelection(t, f, c.Attempt.Parent)
}

func TestCJSIPrivateIndexStageBorrower(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, false)
	tx := stageTx(t, f)
	if _, _, err := store.ReserveCJSIDocumentIDs(t.Context(), tx, 1); err != nil {
		t.Fatal(err)
	}
	// Refusal also rolls back writes made by the owner before the borrow.
	c.Attempt.Attempt = 0
	if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
		t.Fatalf("invalid attempt: %v", err)
	}
	stageScalar(t, f.private, 7, "SELECT doc_id_high_water FROM cjsi_index_state")
	if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("caller could commit: %v", err)
	}

	c.Attempt.Attempt = 2
	tx = stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_session_candidates")
}

// Run the coordinator-facing plan against the production compacted baseline,
// then check the effective and compacted pictures through public readers.
func TestCJSIPlannedIndex(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	fixture := stageFixturePlan(t, f, f.path, true)
	source := fixture.Replacements[0]
	// The planner independently resolves old IDs and identities.
	source.DocumentIDs = nil
	source.DeletedDocumentIDs = nil
	source.Sequence = [32]byte{}
	h := fixture.Attempt.Parent
	projections := []TelemetrySession{{Client: "codex", SessionID: "otel-only", Model: "otel-model", FirstAt: "2026-01-01T02:00:00Z", LastAt: "2026-01-01T03:00:00Z"}}
	with := func(run func(store.CJSIIndexSnapshot) error) {
		t.Helper()
		tx, err := f.core.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := store.WithCJSIIndexSnapshot(ctx, f.root, tx, run); err != nil {
			t.Fatal(err)
		}
	}
	var candidate *CJSIIndexCandidate
	with(func(s store.CJSIIndexSnapshot) error {
		var err error
		candidate, err = PlanCJSIIndex(ctx, s, 1, 2, h.SessionRevision+1, []CJSIPreparedSource{{source}}, nil, &projections)
		return err
	})
	if candidate == nil {
		t.Fatal("empty plan")
	}
	n, removed := candidate.DocumentChanges()
	if n != 1 || removed != 3 {
		t.Fatalf("effective diff changed=%d removed=%d", n, removed)
	}
	if !slices.Equal(candidate.plan.Replacements[0].DocumentIDs, []int64{1, 8}) {
		t.Fatal("stable document ID mismatch")
	}
	// A separate authorizing seal is required even when every other stamp agrees.
	tx := stageTx(t, f)
	if err := candidate.Stage(ctx, tx, h, 1, 2, h.SessionRevision+1, [32]byte{}); !errors.Is(err, errIndexStage) {
		t.Fatalf("wrong seal %v", err)
	}
	tx = stageTx(t, f)
	if err := candidate.Stage(ctx, tx, h, 1, 2, h.SessionRevision+1, candidate.Seal()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertOldStageSelection(t, f, h)
	if _, err := Scan(ctx, f.private, t.TempDir()); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("bare scan admitted: %v", err)
	}
	if _, err := List(ctx, f.private); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("bare list admitted: %v", err)
	}
	if err := ProjectTelemetrySessions(ctx, f.private, nil); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("bare projection admitted: %v", err)
	}
	seal := candidate.Seal()
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=2,session_revision=?,session_seal=?`, h.SessionRevision+1, seal[:])
	var selected store.CJSIIndexStamp
	observe := func() {
		with(func(s store.CJSIIndexSnapshot) error {
			order, err := ReadCJSIOrderSnapshot(ctx, s)
			if err != nil {
				return err
			}
			selected = order.Stamp
			observation := CJSISourceObservation{Source: ingest.Source{Path: f.path, Client: "codex", Priority: 3, Stable: true, Identity: f.identity, Size: source.Size, ModifiedAt: source.ModifiedAt, ChangedAt: source.ChangedAt}, Kind: "codex-jsonl"}
			unchanged, err := CJSISourceUnchanged(ctx, s, observation)
			if err != nil || !unchanged {
				t.Fatalf("selected native checkpoint not reused: %v, %v", unchanged, err)
			}
			observation.Kind = "synthetic"
			unchanged, err = CJSISourceUnchanged(ctx, s, observation)
			if err != nil || unchanged {
				t.Fatalf("mismatched kind reused: %v, %v", unchanged, err)
			}
			if order.HighWater != 8 {
				t.Fatalf("high water %d", order.HighWater)
			}
			var ids []int64
			for _, e := range order.Entries {
				if e.SourcePath == f.path {
					if e.Ordinal != int64(len(ids)+1) || !e.Verified {
						t.Fatal(e)
					}
					ids = append(ids, e.DocumentID)
				}
			}
			if !slices.Equal(ids, []int64{1, 8}) {
				t.Fatalf("ordinal map %v", ids)
			}
			return nil
		})
		f.read(t, func(reader CJSIReader) {
			snap, err := reader.ReadIndexSnapshot(ctx, "A OR stageword")
			if err != nil {
				t.Fatal(err)
			}
			if snap.Epoch != selected.IndexEpoch || snap.Revision != selected.SessionRevision || len(snap.Sessions) != 2 || docLabels(snap.Documents) != "Astageword" {
				t.Fatalf("snapshot %+v", snap)
			}
			detail, err := reader.ShowWithActivity(ctx, "codex", "otel-only", true)
			if err != nil || detail.Model != "otel-model" || len(detail.Documents) != 0 || len(detail.Activity) != 0 {
				t.Fatalf("projection %+v %v", detail, err)
			}
		})
	}
	observe()
	tx = stageTx(t, f)
	if err := CompactCJSIIndex(ctx, tx, h, selected, func(context.Context) (store.CJSIIndexStamp, error) { return selected, nil }); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	observe()
	with(func(s store.CJSIIndexSnapshot) error {
		next, err := PlanCJSIIndex(ctx, s, 2, 1, selected.SessionRevision, nil, nil, &projections)
		if next != nil {
			t.Fatal("unchanged telemetry planned another publication")
		}
		return err
	})
}

func (f physicalFixture) snapshot(t *testing.T, read func(store.CJSIIndexSnapshot) error) {
	t.Helper()
	ctx := t.Context()
	tx, err := f.core.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := store.WithCJSIIndexSnapshot(ctx, f.root, tx, read); err != nil {
		t.Fatal(err)
	}
}

func TestCJSIMigratedProjectionAndHeadlessSource(t *testing.T) {
	ctx := t.Context()
	p := TelemetrySession{Client: "codex", SessionID: "legacy-otel", Model: "model", FirstAt: "2026-01-01T02:00:00Z", LastAt: "2026-01-01T03:00:00Z"}
	orphan := TelemetrySession{Client: "claude", SessionID: "wanted-orphan", FirstAt: "2026-02-01T00:00:00Z"}
	projections := []TelemetrySession{p, orphan}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if err := ProjectTelemetrySessions(ctx, db, []TelemetrySession{p}); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{TelemetrySourcePath(orphan.Client, orphan.SessionID), "otel:codex/stale-orphan"} {
			adapterExec(t, db, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,'',-1,6,'old')`, path, path)
		}
	})
	const thread = "old-session"
	input := `{"timestamp":"2026-01-02T09:00:00Z","type":"turn_context","sessionId":"old-session","payload":{"turn_id":"old-turn","cwd":"/untrusted","model":"old-model"}}
{"timestamp":"2026-01-02T09:00:01Z","type":"response_item","session_id":"old-session","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"headless-word"}]}}
`
	src := fixturePreparedNative(t, filepath.Join(t.TempDir(), "old.jsonl"), thread, input)
	if src.replacement.Material.Source.State != ingest.CodexSourceVerified || src.replacement.Material.Source.Head != (ingest.CodexSpan{}) {
		t.Fatal(src.replacement.Material.Source)
	}
	var parent, selected store.CJSIIndexStamp
	var c *CJSIIndexCandidate
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		parent = s.Selection()
		var err error
		c, err = PlanCJSIIndex(ctx, s, 1, 1, parent.SessionRevision+1, []CJSIPreparedSource{src}, nil, &projections)
		return err
	})
	if c == nil || len(c.plan.Telemetry) != 2 || len(c.plan.DeletedSources) != 1 || c.plan.DeletedSources[0].Path != "otel:codex/stale-orphan" {
		t.Fatalf("plan %+v", c)
	}
	tx := stageTx(t, f)
	if err := c.Stage(ctx, tx, parent, 1, 1, parent.SessionRevision+1, c.Seal()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	seal := c.Seal()
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?`, parent.SessionRevision+1, seal[:])
	observe := func() {
		t.Helper()
		f.read(t, func(reader CJSIReader) {
			d, err := reader.SelectDetail(ctx, "codex", thread)
			if err != nil {
				t.Fatal(err)
			}
			selected = d.identity.Stamp
			docs, page, err := reader.DocumentsPage(ctx, d, 1, 10, false)
			if err != nil || page.Total != 1 || docLabels(docs) != "headless-word" || d.Project != "" || d.Model != "old-model" || d.FirstAt != "2026-01-02T09:00:00Z" {
				t.Fatalf("headless detail %+v %+v %+v %v", d, docs, page, err)
			}
			for _, want := range projections {
				copies, err := reader.EffectiveCopies(ctx, want.Client, want.SessionID)
				if err != nil || len(copies) != 1 || copies[0].Birth != want.FirstAt || copies[0].Metadata.FirstAt != want.FirstAt {
					t.Fatalf("projection %+v: %+v %v", want, copies, err)
				}
			}
		})
	}
	observe()
	tx = stageTx(t, f)
	if err := CompactCJSIIndex(ctx, tx, parent, selected, func(context.Context) (store.CJSIIndexStamp, error) { return selected, nil }); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	observe()
	stageScalar(t, f.private, 0, "SELECT count(*) FROM session_sources WHERE source_path='otel:codex/stale-orphan'")
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		next, err := PlanCJSIIndex(ctx, s, 2, 1, selected.SessionRevision, nil, nil, &projections)
		if next != nil {
			t.Fatal("canonical projections planned again")
		}
		return err
	})
}

func TestCJSIMigratedFileCheckpointReuse(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	files := map[string]string{
		".claude/projects/p/clean.jsonl":   "{\"type\":\"user\",\"sessionId\":\"clean\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n",
		".claude/projects/p/partial.jsonl": "{\"type\":\"user\",\"sessionId\":\"partial\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n{\"tail\":",
		".codex/sessions/old.jsonl":        "{\"type\":\"session_meta\",\"payload\":{\"id\":\"old\"}}\n",
		".claude/projects/p/mixed.jsonl":   "{\"type\":\"user\",\"sessionId\":\"mixed\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n",
	}
	for path, body := range files {
		path = filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if r, err := Scan(ctx, db, home); err != nil || r.Sources != 4 {
			t.Fatalf("legacy scan %+v %v", r, err)
		}
		adapterExec(t, db, `INSERT INTO session_metadata VALUES(?,'codex','other-client','','',6,'','')`, filepath.Join(home, ".claude/projects/p/mixed.jsonl"))
	})
	found, err := ingest.Discover(home)
	if err != nil || len(found) != 4 {
		t.Fatalf("discover %+v %v", found, err)
	}
	var partial CJSISourceObservation
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, src := range found {
			name := filepath.Base(src.Path)
			want := name == "clean.jsonl" || name == "partial.jsonl"
			obs := CJSISourceObservation{Source: src, Kind: src.Client + "-jsonl"}
			got, err := CJSISourceUnchanged(ctx, s, obs)
			if err != nil || got != want {
				t.Fatalf("%s, priority %d: got %v want %v err=%v", name, src.Priority, got, want, err)
			}
			switch name {
			case "old.jsonl":
				for _, kind := range []string{"synthetic", "otel"} {
					got, err := CJSISourceUnchanged(ctx, s, CJSISourceObservation{Source: src, Kind: kind})
					if err != nil || got {
						t.Fatalf("old Codex file labeled %s: unchanged=%v err=%v", kind, got, err)
					}
				}
			case "partial.jsonl":
				partial = obs
			}
			if want {
				obs.Priority++
				got, err = CJSISourceUnchanged(ctx, s, obs)
				if err != nil || got {
					t.Fatalf("changed priority on %s: %v %v", name, got, err)
				}
			}
		}
		return nil
	})
	// Reopen the selected snapshot after removing the exact tail evidence.
	deleted, err := f.private.ExecContext(ctx, "DELETE FROM cjsi_legacy_cursor_hints WHERE source_path=?", partial.Path)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := deleted.RowsAffected(); err != nil || count != 1 {
		t.Fatalf("removed %d tail hints, want one: %v", count, err)
	}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		got, err := CJSISourceUnchanged(ctx, s, partial)
		if err != nil || got {
			t.Fatalf("partial Claude file without cursor hint: unchanged=%v err=%v", got, err)
		}
		return nil
	})
}

func TestCJSISelectedNativeCheckpointReuse(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "rollout-2026-01-02T03-04-05-"+adapterThread+".jsonl")
	input := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`
	prepared := fixturePreparedNative(t, path, adapterThread, input)
	r := prepared.replacement
	observation := CJSISourceObservation{Kind: "codex-jsonl", Source: ingest.Source{
		Client: "codex", Path: path, Priority: r.Metadata.projectPriority, Identity: r.Selected.Evidence.Identity,
		Size: r.Size, ModifiedAt: r.ModifiedAt, ChangedAt: r.ChangedAt, Stable: true,
	}}
	for _, tc := range []struct {
		name       string
		replayFrom int64
		want       bool
	}{
		{"replay_pending", 0, false},
		{"replay_complete", r.Size, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Both candidates describe the same captured bytes. Change the replay
			// boundary before planning so the selection authenticates that value.
			src := prepared
			src.replacement.Material.Checkpoint.ReplayFrom = tc.replayFrom
			f := newPhysicalFixture(t, true, false)
			var parent store.CJSIIndexStamp
			var candidate *CJSIIndexCandidate
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				parent = s.Selection()
				var err error
				candidate, err = PlanCJSIIndex(ctx, s, 1, 1, parent.SessionRevision+1, []CJSIPreparedSource{src}, nil, nil)
				return err
			})
			if candidate == nil {
				t.Fatal("expected a native checkpoint candidate")
			}
			seal := candidate.Seal()
			tx := stageTx(t, f)
			if err := candidate.Stage(ctx, tx, parent, 1, 1, parent.SessionRevision+1, seal); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?`, parent.SessionRevision+1, seal[:])
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				var read, complete, published, replay int64
				err := s.Tx.QueryRowContext(ctx, `SELECT read_end,complete_end,published_end,replay_from FROM (`+s.RangesSQL()+`) WHERE source_path=? AND client='codex' AND session_id=?`, path, adapterThread).Scan(&read, &complete, &published, &replay)
				if err != nil {
					return err
				}
				if read != r.Size || complete != r.Size || published != r.Size || replay != tc.replayFrom {
					t.Fatalf("selected range read=%d complete=%d published=%d replay=%d; want size=%d replay=%d", read, complete, published, replay, r.Size, tc.replayFrom)
				}
				got, err := CJSISourceUnchanged(ctx, s, observation)
				if err != nil || got != tc.want {
					t.Fatalf("replay_from=%d complete_end=%d: unchanged=%v err=%v, want %v", replay, complete, got, err, tc.want)
				}
				batch, err := CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{observation})
				if err != nil || batch != tc.want {
					t.Fatalf("full inventory replay_from=%d: unchanged=%v err=%v, want %v", replay, batch, err, tc.want)
				}
				missing, err := CJSISourcesUnchanged(ctx, s, nil)
				if err != nil || missing {
					t.Fatalf("missing inventory member: unchanged=%v err=%v", missing, err)
				}
				return nil
			})
			if tc.want {
				t.Run("file_grew_after_observation", func(t *testing.T) {
					file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
					if err != nil {
						t.Fatal(err)
					}
					_, err = file.WriteString("{}\n")
					if err = errors.Join(err, file.Close()); err != nil {
						t.Fatal(err)
					}
					f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
						got, err := CJSISourceUnchanged(ctx, s, observation)
						if got || !errors.Is(err, ingest.ErrSourceChanged) {
							t.Errorf("stale single observation: unchanged=%v err=%v, want false and ErrSourceChanged", got, err)
						}
						got, err = CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{observation})
						if got || !errors.Is(err, ingest.ErrSourceChanged) {
							t.Errorf("stale inventory: unchanged=%v err=%v, want false and ErrSourceChanged", got, err)
						}
						return nil
					})
				})
			}
		})
	}
}

func TestCJSIFileInventoryIgnoresVirtualSources(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	body := "{\"type\":\"user\",\"sessionId\":\"file-one\",\"message\":{\"role\":\"user\",\"content\":\"confirmed\"}}\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := ingest.Discover(home)
	if err != nil || len(files) != 1 {
		t.Fatalf("discovered %v, %v", files, err)
	}
	obs := CJSISourceObservation{Source: files[0], Kind: "claude-jsonl"}
	for _, projection := range []string{"file_only", "otel", "replace_documents"} {
		t.Run(projection, func(t *testing.T) {
			f := newMigratedFixture(t, true, false, func(db *sql.DB) {
				if scan, err := Scan(ctx, db, home); err != nil || scan.Sources != 1 {
					t.Fatalf("scan %+v err=%v", scan, err)
				}
				switch projection {
				case "otel":
					if err := ProjectTelemetrySessions(ctx, db, []TelemetrySession{{Client: "codex", SessionID: "virtual"}}); err != nil {
						t.Fatal(err)
					}
				case "replace_documents":
					doc, err := ApprovedDocument("codex", "virtual", "user_prompt", "virtual")
					if err != nil {
						t.Fatal(err)
					}
					if err := ReplaceDocuments(ctx, db, "codex", "virtual", []Document{doc}); err != nil {
						t.Fatal(err)
					}
				}
			})
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				one, err := CJSISourceUnchanged(ctx, s, obs)
				if err != nil || !one {
					t.Fatalf("file checkpoint failed single-source proof: %v, %v", one, err)
				}
				batch, err := CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{obs})

				if !batch || err != nil {
					t.Errorf("unchanged complete file inventory: got %v, %v; want true, nil", batch, err)
				}
				missing, err := CJSISourcesUnchanged(ctx, s, nil)
				if err != nil || missing {
					t.Fatalf("omitted real file: unchanged=%v err=%v", missing, err)
				}
				return nil
			})
		})
	}
}

// A malformed replacement must fail the producer's validation, before its
// metadata, approved text or document ordering is written to private storage.
func TestCJSIReplacementValidation(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, true).Replacements[0]
	if !base.valid() {
		t.Fatal("fixture is not a valid replacement")
	}
	cases := []struct {
		name   string
		change func(*indexReplacement)
	}{
		{"empty source path", func(r *indexReplacement) { r.Metadata.SourcePath = ""; r.Selected.Path = ""; r.From = "prior" }},
		{"other source path", func(r *indexReplacement) { r.Metadata.SourcePath += ".other" }},
		{"other client", func(r *indexReplacement) { r.Metadata.Client = "claude" }},
		{"empty thread", func(r *indexReplacement) {
			r.Metadata.SessionID = ""
			r.Selected.ThreadID = ""
			r.Material.Source.ThreadID = ""
		}},
		{"other metadata thread", func(r *indexReplacement) { r.Metadata.SessionID = "other" }},
		{"unverified source", func(r *indexReplacement) { r.Material.Source.State = ingest.CodexSourceUnknown }},
		{"other proof thread", func(r *indexReplacement) { r.Material.Source.ThreadID = "other" }},
		{"legacy native parser", func(r *indexReplacement) { r.Material.ParserVersion = ParserVersion }},
		{"empty selection epoch", func(r *indexReplacement) { r.Selected.Epoch = 0 }},
		{"selection disagrees with capture", func(r *indexReplacement) { r.Material.Evidence.Identity = "other" }},
		{"empty identity", func(r *indexReplacement) { r.Selected.Evidence.Identity = "" }},
		{"empty digest", func(r *indexReplacement) { r.Selected.Evidence.Digest = [32]byte{} }},
		{"zero generation", func(r *indexReplacement) { r.Selected.Evidence.Version.Generation = 0 }},
		{"zero replay parser", func(r *indexReplacement) { r.Selected.Evidence.Version.Parser = 0 }},
		{"empty line budget", func(r *indexReplacement) {
			r.Selected.Evidence.MaxLineBytes = 0
			r.Selected.Evidence.Version.MaxLineBytes = 0
		}},
		{"different line budget", func(r *indexReplacement) { r.Selected.Evidence.Version.MaxLineBytes++ }},
		{"invalid round", func(r *indexReplacement) { r.Round = 0 }},
		{"invalid workset", func(r *indexReplacement) { r.WorksetAttempt = 0 }},
		{"read beyond file", func(r *indexReplacement) { r.Size-- }},
		{"no complete input", func(r *indexReplacement) {
			r.Selected.Evidence.Progress.CompleteEnd = 0
			r.Selected.Evidence.Progress.Partial = true
			r.Material.Checkpoint.CompleteEnd = 0
			r.Material.Checkpoint.ReplayFrom = 0
			r.Material.Checkpoint.Partial = true
			r.PublishedEnd = 0
			r.Material.Documents = nil
			r.DocumentIDs = nil
		}},
		{"complete beyond read", func(r *indexReplacement) {
			r.Selected.Evidence.Progress.CompleteEnd++
			r.Material.Checkpoint.CompleteEnd++
			r.PublishedEnd++
		}},
		{"missing partial flag on captured tail", func(r *indexReplacement) {
			r.Selected.Evidence.Progress.ReadEnd++
			r.Size++
			r.Material.Checkpoint.ReadEnd++
		}},
		{"wrong partial flag", func(r *indexReplacement) {
			r.Selected.Evidence.Progress.Partial = true
			r.Material.Checkpoint.Partial = true
		}},
		{"checkpoint read boundary", func(r *indexReplacement) { r.Material.Checkpoint.ReadEnd-- }},
		{"checkpoint complete boundary", func(r *indexReplacement) {
			r.Material.Checkpoint.CompleteEnd--
			r.Material.Checkpoint.ReplayFrom = 0
			r.PublishedEnd--
		}},
		{"checkpoint partial flag", func(r *indexReplacement) { r.Material.Checkpoint.Partial = true }},
		{"negative replay boundary", func(r *indexReplacement) { r.Material.Checkpoint.ReplayFrom = -1 }},
		{"replay past complete boundary", func(r *indexReplacement) { r.Material.Checkpoint.ReplayFrom = r.Material.Checkpoint.CompleteEnd + 1 }},
		{"published boundary", func(r *indexReplacement) { r.PublishedEnd-- }},
		{"move to same path", func(r *indexReplacement) { r.From = r.Selected.Path }},
		{"empty sequence", func(r *indexReplacement) { r.Sequence = [32]byte{} }},
		{"empty scan stamp", func(r *indexReplacement) { r.ScannedAt = "" }},
		{"extra id", func(r *indexReplacement) { r.DocumentIDs = append(r.DocumentIDs, 27) }},
		{"ordinal gap", func(r *indexReplacement) { r.Material.Documents[1].Ordinal++ }},
		{"foreign document client", func(r *indexReplacement) { r.Material.Documents[1].Client = "claude" }},
		{"foreign document thread", func(r *indexReplacement) { r.Material.Documents[1].SessionID = "other" }},
		{"disallowed document kind", func(r *indexReplacement) { r.Material.Documents[1].Kind = "function_call_output" }},
		{"overlapping documents", func(r *indexReplacement) { r.Material.Documents[1].Span.Start = r.Material.Documents[0].Span.End - 1 }},
		{"empty span", func(r *indexReplacement) { r.Material.Documents[1].Span.End = r.Material.Documents[1].Span.Start }},
		{"document beyond publication", func(r *indexReplacement) { r.Material.Documents[1].Span.End = r.PublishedEnd + 1 }},
		{"zero document ID", func(r *indexReplacement) { r.DocumentIDs[1] = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			r.Material.Documents = slices.Clone(r.Material.Documents)
			r.DocumentIDs = slices.Clone(r.DocumentIDs)
			// Empty visible groups are valid; their identity must still be checked.
			switch tc.name {
			case "other client", "empty thread", "other metadata thread":
				r.Material.Documents = nil
				r.DocumentIDs = nil
			}

			tc.change(&r)
			if r.Selected.Evidence != base.Selected.Evidence {
				r.Material.Evidence = r.Selected.Evidence
			}
			if r.valid() {
				t.Fatal("invalid replacement accepted")
			}
		})
	}
}

func TestCJSIRemovalRevealsSurvivingReplica(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		c, err := PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, nil, []CJSISourceRemoval{{Path: f.path, Identity: f.identity}}, nil)
		if err != nil {
			return err
		}
		if c == nil {
			t.Fatal("missing removal plan")
		}
		// The surviving replica replaces A/B/C with one different document.
		// Two other single-document groups have no surviving replica.
		changed, removed := c.DocumentChanges()
		if changed != 1 || removed != 4 {
			t.Fatalf("winner change: changed=%d removed=%d, want 1 and 4", changed, removed)
		}
		return nil
	})
}

// Exercise SQLite failures at the statement, iteration and row-count
// boundaries of a borrowed transaction. Discover the operations from a
// successful rolled-back run so database refactors do not renumber tests.
type indexFaultSite struct {
	operation, query string
	occurrence       int
}
type indexFaultTrace struct {
	rowsClosed      func(string)
	rollbackFailure error
	corruptRow      func(string, []driver.Value)
	target          indexFaultSite
	counts          map[[2]string]int
	sites           []indexFaultSite
	fired           bool
}

var errIndexFixtureIO = errors.New("fixture index I/O failed")

func (f *indexFaultTrace) hit(op, q string) error {
	key := [2]string{op, q}
	f.counts[key]++
	site := indexFaultSite{op, q, f.counts[key]}
	if f.target.operation == "" {
		f.sites = append(f.sites, site)
	} else if f.target == site {
		f.fired = true
		return errIndexFixtureIO
	}
	return nil
}

type indexFaultConnector struct {
	dsn   string
	trace *indexFaultTrace
}

func (c indexFaultConnector) Driver() driver.Driver { return &sqlite.Driver{} }
func (c indexFaultConnector) Connect(context.Context) (driver.Conn, error) {
	inner, err := c.Driver().Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &indexFaultConn{inner, c.trace}, nil
}

type indexFaultConn struct {
	driver.Conn
	trace *indexFaultTrace
}

func (c *indexFaultConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if err != nil || c.trace.rollbackFailure == nil {
		return tx, err
	}
	return indexFaultTx{tx, c.trace.rollbackFailure}, nil
}
func (c *indexFaultConn) ExecContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Result, error) {
	if err := c.trace.hit("exec", q); err != nil {
		return nil, err
	}
	res, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, q, a)
	if err != nil {
		return nil, err
	}
	return indexFaultResult{res, c.trace, q}, nil
}
func (c *indexFaultConn) QueryContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Rows, error) {
	if err := c.trace.hit("query", q); err != nil {
		return nil, err
	}
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, a)
	if err != nil {
		return nil, err
	}
	return &indexFaultRows{Rows: rows, trace: c.trace, query: q}, nil
}

type indexFaultTx struct {
	driver.Tx
	failure error
}

func (tx indexFaultTx) Rollback() error { return errors.Join(tx.Tx.Rollback(), tx.failure) }

type indexFaultResult struct {
	driver.Result
	trace *indexFaultTrace
	query string
}

func (r indexFaultResult) RowsAffected() (int64, error) {
	if e := r.trace.hit("rows affected", r.query); e != nil {
		return 0, e
	}
	return r.Result.RowsAffected()
}

type indexFaultRows struct {
	driver.Rows
	trace *indexFaultTrace
	query string
	read  int
}

func (r *indexFaultRows) Next(dest []driver.Value) error {
	r.read++
	switch r.read {
	case 1:
		if err := r.trace.hit("first row", r.query); err != nil {
			return err
		}
	case 2:
		if err := r.trace.hit("next row", r.query); err != nil {
			return err
		}
	}
	err := r.Rows.Next(dest)
	if err == nil && r.trace.corruptRow != nil {
		r.trace.corruptRow(r.query, dest)
	}
	return err
}

func (r *indexFaultRows) Close() error {
	err := errors.Join(r.Rows.Close(), r.trace.hit("close rows", r.query))
	if r.trace.rowsClosed != nil {
		r.trace.rowsClosed(r.query)
	}
	return err
}

func exerciseIndexFailures(t *testing.T, f physicalFixture, writer bool, operation func(*sql.Tx) error) {
	t.Helper()
	ctx := t.Context()
	u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	trace := indexFaultTrace{}
	db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
	defer db.Close()
	run := func(site indexFaultSite) (*sql.Tx, error) {
		trace = indexFaultTrace{target: site, counts: make(map[[2]string]int)}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		return tx, operation(tx)
	}
	tx, err := run(indexFaultSite{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	sites := trace.sites
	if len(sites) == 0 {
		t.Fatal("successful write produced no trace")
	}
	for i, site := range sites {
		t.Run(fmt.Sprintf("%03d_%s", i, site.operation), func(t *testing.T) {
			tx, err := run(site)
			if !trace.fired || !errors.Is(err, errIndexFixtureIO) {
				tx.Rollback()
				t.Fatalf("fault %s #%d on %s: fired=%v err=%v", site.operation, site.occurrence, site.query, trace.fired, err)
			}
			if !writer {
				tx.Rollback()
				return
			}
			if e := tx.Commit(); !errors.Is(e, sql.ErrTxDone) {
				t.Fatalf("failed transaction remained committable: %v", e)
			}
		})
	}
	if !writer {
		exerciseIndexScanFailures(t, db, &trace, operation)
	}
}

// A driver can deliver a row and still fail to decode it into the required
// non-null projection. Verify that no partial output escapes that boundary.
func exerciseIndexScanFailures(t *testing.T, db *sql.DB, trace *indexFaultTrace, operation func(*sql.Tx) error) {
	t.Helper()
	queries := map[string]bool{}
	run := func(alter func(string, []driver.Value)) error {
		*trace = indexFaultTrace{counts: map[[2]string]int{}, corruptRow: alter}
		tx, err := db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		return operation(tx)
	}
	if err := run(func(query string, _ []driver.Value) { queries[query] = true }); err != nil {
		t.Fatal(err)
	}
	for query := range queries {
		key := sha256.Sum256([]byte(query))
		t.Run(fmt.Sprintf("invalid_row_%x", key[:6]), func(t *testing.T) {
			fired := false
			err := run(func(q string, values []driver.Value) {
				if q == query && !fired {
					clear(values)
					fired = true
				}
			})
			if !fired || err == nil || !strings.Contains(err.Error(), "sql: Scan error on column index") {
				t.Fatalf("invalid result row from %s: injected=%v err=%v", query, fired, err)
			}
		})
	}
}

func TestCJSIPrivateWriteFailureBoundaries(t *testing.T) {
	ctx := t.Context()
	t.Run("stage", func(t *testing.T) {
		f := newPhysicalFixture(t, true, true)
		c := stageFixturePlan(t, f, f.path, true)
		exerciseIndexFailures(t, f, true, func(tx *sql.Tx) error { return stageIndexCandidate(ctx, tx, c.Attempt, c) })
		stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_session_candidates")
		stageScalar(t, f.private, 7, "SELECT doc_id_high_water FROM cjsi_index_state")
	})
	t.Run("compact", func(t *testing.T) {
		f := newCompactFixture(t, true, true, "complete")
		exerciseIndexFailures(t, f.physicalFixture, true, func(tx *sql.Tx) error { return compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current) })
		stageScalar(t, f.private, 0, "SELECT compacted_ticket FROM cjsi_index_state")
	})
}

func TestCJSIPlanReadFailureBoundaries(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	source := stageFixturePlan(t, f, f.path, true).Replacements[0]
	source.DocumentIDs = nil
	source.DeletedDocumentIDs = nil
	source.Sequence = [32]byte{}
	projections := []TelemetrySession{{Client: "codex", SessionID: "planned", FirstAt: "2026-01-01T00:00:00Z"}}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		exerciseIndexFailures(t, f, false, func(tx *sql.Tx) error {
			pinned := s
			pinned.Tx = tx
			c, err := PlanCJSIIndex(ctx, pinned, h.Ticket+1, 1, h.SessionRevision+1, []CJSIPreparedSource{{source}}, nil, &projections)
			if err != nil && c != nil {
				t.Fatalf("failed plan returned candidate %x: %v", c.Seal(), err)
			}
			return err
		})
		return nil
	})
}

func TestCJSITelemetryPlanRequiresCanonicalProjection(t *testing.T) {
	ctx := t.Context()
	items := []TelemetrySession{{Client: "codex", SessionID: "one", Model: "test", FirstAt: "first", LastAt: "last"}}
	path := TelemetrySourcePath("codex", "one")
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if err := ProjectTelemetrySessions(ctx, db, items); err != nil {
			t.Fatal(err)
		}
	})
	seq, err := indexDigest([]int64(nil))
	if err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.private, "UPDATE session_sources SET scanned_at='' WHERE source_path=?", path)
	adapterExec(t, f.private, "UPDATE cjsi_base_groups SET source_kind='otel', order_verified=1, birth='first',sequence_id=? WHERE source_path=?", seq[:], path)
	for _, tc := range []struct {
		name, update string
		replan       bool
	}{
		{"canonical", "", false},
		{"identity", "UPDATE session_sources SET identity='different' WHERE source_path=?", true},
		{"priority", "UPDATE session_sources SET priority=1 WHERE source_path=?", true},
		{"source parser", "UPDATE session_sources SET parser_version=3 WHERE source_path=?", true},
		{"cursor", "UPDATE session_sources SET cursor=1 WHERE source_path=?", true},
		{"size", "UPDATE session_sources SET size=1 WHERE source_path=?", true},
		{"mtime", "UPDATE session_sources SET modified_at=1 WHERE source_path=?", true},
		{"ctime", "UPDATE session_sources SET changed_at=1 WHERE source_path=?", true},
		{"prefix", "UPDATE session_sources SET prefix_hash='old' WHERE source_path=?", true},
		{"partial", "UPDATE session_sources SET partial_line=x'61' WHERE source_path=?", true},
		{"context", "UPDATE session_sources SET parser_context='old' WHERE source_path=?", true},
		{"scanned", "UPDATE session_sources SET scanned_at='old' WHERE source_path=?", true},
		{"project", "UPDATE session_metadata SET project='/old' WHERE source_path=?", true},
		{"metadata parser", "UPDATE session_metadata SET parser_version=3 WHERE source_path=?", true},
		{"kind", "UPDATE cjsi_base_groups SET source_kind='synthetic' WHERE source_path=?", true},
		{"unverified order", "UPDATE cjsi_base_groups SET order_verified=0 WHERE source_path=?", true},
		{"missing birth", "UPDATE cjsi_base_groups SET birth='' WHERE source_path=?", true},
		{"head time", "UPDATE cjsi_base_groups SET head_at='old' WHERE source_path=?", true},
		{"sequence", "UPDATE cjsi_base_groups SET sequence_id=randomblob(32) WHERE source_path=?", true},
		{"obsolete model", "UPDATE session_metadata SET model='old' WHERE source_path=?", true},
		{"obsolete first time", "UPDATE session_metadata SET first_at='other' WHERE source_path=?", true},
		{"obsolete last time", "UPDATE session_metadata SET last_at='other' WHERE source_path=?", true},
		{"obsolete member", "INSERT INTO cjsi_base_order(source_path,client,session_id,doc_id,detail_ordinal) VALUES(?,'codex','one',1,1)", true},
		{"obsolete range", `INSERT INTO cjsi_base_ranges(source_path,client,session_id,workset_round,workset_attempt,identity,selection_epoch,generation,replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest) VALUES(?,'codex','one',1,1,'obsolete',1,1,1,100,1,1,1,1,randomblob(32))`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			if tc.update != "" {
				if _, err := tx.ExecContext(ctx, tc.update, path); err != nil {
					t.Fatal(err)
				}
			}
			var c indexStageCandidate
			if err := c.planTelemetry(ctx, tx, items); err != nil {
				t.Fatal(err)
			}
			if len(c.DeletedSources) != 0 || tc.replan && !slices.Equal(c.Telemetry, items) || !tc.replan && len(c.Telemetry) != 0 {
				t.Fatalf("telemetry=%+v removed=%+v want replacement=%v", c.Telemetry, c.DeletedSources, tc.replan)
			}
		})
	}
}

func TestCJSIInventoryObservationMustMatchStoredFile(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"user\",\"sessionId\":\"one\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if result, err := Scan(ctx, db, home); err != nil || result.Sources != 1 {
			t.Fatalf("scan %+v err=%v", result, err)
		}
	})
	sources, err := ingest.Discover(home)
	if err != nil || len(sources) != 1 {
		t.Fatalf("discover %v %v", sources, err)
	}
	base := CJSISourceObservation{Source: sources[0], Kind: "claude-jsonl"}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, tc := range []struct {
			name  string
			alter func(*CJSISourceObservation)
		}{
			{"unstable", func(o *CJSISourceObservation) { o.Stable = false }},
			{"blank path", func(o *CJSISourceObservation) { o.Path = "" }},
			{"other path", func(o *CJSISourceObservation) { o.Path += ".other" }},
			{"absent identity", func(o *CJSISourceObservation) { o.Identity = "" }},
			{"different identity", func(o *CJSISourceObservation) { o.Identity += "-other" }},
			{"absent ctime", func(o *CJSISourceObservation) { o.ChangedAt = 0 }},
			{"different ctime", func(o *CJSISourceObservation) { o.ChangedAt++ }},
			{"different size", func(o *CJSISourceObservation) { o.Size++ }},
			{"different mtime", func(o *CJSISourceObservation) { o.ModifiedAt++ }},
			{"different priority", func(o *CJSISourceObservation) { o.Priority++ }},
			{"foreign client", func(o *CJSISourceObservation) { o.Client = "codex" }},
			{"unknown kind", func(o *CJSISourceObservation) { o.Kind = "unknown" }},
			{"non-file kind", func(o *CJSISourceObservation) { o.Kind = "otel" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				o := base
				tc.alter(&o)
				got, err := CJSISourceUnchanged(ctx, s, o)
				if err != nil || got {
					t.Fatalf("single=%v,%v", got, err)
				}
				got, err = CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{o})
				if err != nil || got {
					t.Fatalf("batch=%v,%v", got, err)
				}
			})
		}
		got, err := CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{base, base})
		if err != nil || got {
			t.Fatalf("duplicate discovery=%v,%v", got, err)
		}
		got, err = CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{base})
		if err != nil || !got {
			t.Fatalf("exact observation=%v,%v", got, err)
		}
		return nil
	})
}

func TestCJSIRetainedDocumentNeedsMatchingBase(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, true).Replacements[0]
	for _, tc := range []struct {
		name, sql string
		change    func(*nativeSessionDocument)
		origin    string
		wantID    int64
		wantErr   error
	}{
		{name: "verified mapping", wantID: 1},
		{name: "other origin", origin: "another origin"},
		{name: "other session", change: func(d *nativeSessionDocument) { d.SessionID = "other" }},
		{name: "unverified order", sql: "UPDATE cjsi_base_groups SET order_verified=0 WHERE source_path=?"},
		{name: "unclassified source", sql: "UPDATE cjsi_base_groups SET source_kind=NULL WHERE source_path=?"},
		{name: "other source identity", sql: "UPDATE session_sources SET identity='other' WHERE source_path=?"},
		{name: "other range identity", sql: "UPDATE cjsi_base_ranges SET identity='other' WHERE source_path=?"},
		{name: "other generation", sql: "UPDATE cjsi_base_ranges SET generation=generation+1 WHERE source_path=?"},
		{name: "other parser", sql: "UPDATE cjsi_base_ranges SET replay_parser=replay_parser+1 WHERE source_path=?"},
		{name: "other line budget", sql: "UPDATE cjsi_base_ranges SET max_line_bytes=max_line_bytes+1 WHERE source_path=?"},
		{name: "unpublished record", sql: "UPDATE cjsi_base_ranges SET published_end=0 WHERE source_path=?"},
		{name: "other record start", change: func(d *nativeSessionDocument) { d.Span.Start++ }},
		{name: "other emission", sql: "UPDATE cjsi_base_order SET emission=1 WHERE doc_id=1 AND source_path=?"},
		{name: "other kind", change: func(d *nativeSessionDocument) { d.Kind = "assistant_final" }},
		{name: "different end at same key", change: func(d *nativeSessionDocument) { d.Span.End++ }, wantErr: errIndexStage},
		{name: "different time at same key", change: func(d *nativeSessionDocument) { d.EventAt = "2000-01-01T00:00:00Z" }, wantErr: errIndexStage},
		{name: "different text at same key", change: func(d *nativeSessionDocument) { d.Text += "changed" }, wantErr: errIndexStage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			origin := f.path
			if tc.origin != "" {
				origin = tc.origin
			}
			if tc.sql != "" {
				if _, err := tx.ExecContext(ctx, tc.sql, f.path); err != nil {
					t.Fatal(err)
				}
			}
			doc := base.Material.Documents[0]
			if tc.change != nil {
				tc.change(&doc)
			}
			got, err := base.baseDocument(ctx, tx, origin, doc)
			if got != tc.wantID || !errors.Is(err, tc.wantErr) {
				t.Fatalf("retained id=%d err=%v, want %d %v", got, err, tc.wantID, tc.wantErr)
			}
		})
	}
}

func TestCJSIPlanAdmissionAndSourcePaths(t *testing.T) {
	ctx := t.Context()
	t.Run("parent and allocator", func(t *testing.T) {
		f := newPhysicalFixture(t, true, false)
		input := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"new"}}
`
		src := fixturePreparedNative(t, filepath.Join(t.TempDir(), "new.jsonl"), adapterThread, input)
		if candidate, err := PlanCJSIIndex(ctx, store.CJSIIndexSnapshot{}, 1, 1, 1, nil, nil, nil); candidate != nil || !errors.Is(err, errIndexStage) {
			t.Fatalf("unbound plan=%v %v", candidate, err)
		}
		adapterExec(t, f.private, "UPDATE cjsi_index_state SET doc_id_high_water=9223372036854775807")
		f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
			h := s.Selection()
			for _, tc := range []struct {
				name                 string
				ticket, attempt, rev int64
				sources              []CJSIPreparedSource
			}{
				{"old ticket", h.Ticket, 1, h.SessionRevision, nil},
				{"zero attempt", h.Ticket + 1, 0, h.SessionRevision, nil},
				{"old revision", h.Ticket + 1, 1, h.SessionRevision - 1, nil},
				{"exhausted ids", h.Ticket + 1, 1, h.SessionRevision + 1, []CJSIPreparedSource{src}},
			} {
				c, err := PlanCJSIIndex(ctx, s, tc.ticket, tc.attempt, tc.rev, tc.sources, nil, nil)
				if c != nil || !errors.Is(err, errIndexStage) {
					t.Fatalf("%s returned candidate=%v err=%v", tc.name, c, err)
				}
			}
			return nil
		})
	})
	t.Run("move and duplicate tombstones", func(t *testing.T) {
		f := newPhysicalFixture(t, true, true)
		dest := filepath.Join(t.TempDir(), filepath.Base(f.path))
		if err := os.Rename(f.path, dest); err != nil {
			t.Fatal(err)
		}
		physical := stageFixturePlan(t, f, dest, true)
		r := physical.Replacements[0]
		r.From = f.path
		r.Sequence = [32]byte{}
		r.DocumentIDs = nil
		r.DeletedDocumentIDs = nil
		f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
			h := s.Selection()
			removed := []CJSISourceRemoval{{"replica", "second"}, {f.path, f.identity}}
			c, err := PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{{r}}, removed, nil)
			if err != nil {
				return err
			}
			if c == nil || !slices.Equal(c.plan.DeletedSources, []indexSourceDelete{{f.path, f.identity}, {"replica", "second"}}) || !slices.Equal(c.plan.Replacements[0].DocumentIDs, []int64{1, 8}) {
				t.Fatalf("move plan %+v", c)
			}
			return nil
		})
	})
	t.Run("excluded existing and new source", func(t *testing.T) {
		f := newMigratedFixture(t, true, true, func(db *sql.DB) {
			adapterExec(t, db, "INSERT INTO session_exclusions VALUES('session',?)", adapterThread)
		})
		body := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"must stay private"}}
`
		for _, tc := range []struct {
			path  string
			known bool
		}{{f.path, true}, {filepath.Join(t.TempDir(), "unseen.jsonl"), false}} {
			sourced := fixturePreparedNative(t, tc.path, adapterThread, body)
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				c, err := PlanCJSIIndex(ctx, s, 1, 1, s.Selection().SessionRevision+1, []CJSIPreparedSource{sourced}, nil, nil)
				if err != nil {
					return err
				}
				if !tc.known {
					if c != nil {
						t.Fatal("new excluded source produced a plan")
					}
					return nil
				}
				if c == nil || len(c.plan.Replacements) != 0 || !slices.Equal(c.plan.DeletedSources, []indexSourceDelete{{f.path, f.identity}}) {
					t.Fatalf("excluded source retained: %+v", c)
				}
				return nil
			})
		}
	})
}

func TestCJSIStageTicketAndTombstonePreconditions(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, false)
	if err := stageIndexCandidate(ctx, nil, base.Attempt, base); !errors.Is(err, errIndexStage) {
		t.Fatalf("nil transaction: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*indexStageCandidate)
	}{
		{"empty payload", func(c *indexStageCandidate) { c.Replacements = nil; c.DeletedSources = nil; c.DeletedGroups = nil }},
		{"empty seal", func(c *indexStageCandidate) { c.Attempt.Seal = [32]byte{} }},
		{"empty parent seal", func(c *indexStageCandidate) { c.Attempt.Parent.SessionSeal = [32]byte{} }},
		{"invalid core epoch", func(c *indexStageCandidate) { c.Attempt.Parent.CoreEpoch = 0 }},
		{"invalid index epoch", func(c *indexStageCandidate) { c.Attempt.Parent.IndexEpoch = 0 }},
		{"parent ahead of global ticket", func(c *indexStageCandidate) { c.Attempt.Parent.SessionTicket = 1 }},
		{"ticket already current", func(c *indexStageCandidate) { c.Attempt.Ticket = c.Attempt.Parent.Ticket }},
		{"invalid attempt", func(c *indexStageCandidate) { c.Attempt.Attempt = 0 }},
		{"regressed revision", func(c *indexStageCandidate) { c.Attempt.SessionRevision = c.Attempt.Parent.SessionRevision - 1 }},
		{"wrong compacted seal", func(c *indexStageCandidate) { c.Attempt.Parent.SessionSeal[0] ^= 1 }},
		{"wrong compacted attempt", func(c *indexStageCandidate) { c.Attempt.Parent.SessionAttempt++ }},
		{"wrong compacted ticket", func(c *indexStageCandidate) {
			c.Attempt.Parent.SessionTicket++
			c.Attempt.Parent.Ticket++
			c.Attempt.Ticket++
		}},
		{"unnamed source removal", func(c *indexStageCandidate) { c.DeletedSources[0].Path = "" }},
		{"unproved source removal", func(c *indexStageCandidate) { c.DeletedSources[0].Identity = "" }},
		{"wrong removal identity", func(c *indexStageCandidate) { c.DeletedSources[0].Identity = "not-this-source" }},
		{"wrong removal path", func(c *indexStageCandidate) { c.DeletedSources[0].Path = "missing" }},
		{"unnamed group source", func(c *indexStageCandidate) { c.DeletedGroups[0].Path = "" }},
		{"unnamed group client", func(c *indexStageCandidate) { c.DeletedGroups[0].Client = "" }},
		{"unnamed group session", func(c *indexStageCandidate) { c.DeletedGroups[0].SessionID = "" }},
		{"unproved group sequence", func(c *indexStageCandidate) { c.DeletedGroups[0].Sequence = [32]byte{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.DeletedSources = slices.Clone(base.DeletedSources)
			c.DeletedGroups = slices.Clone(base.DeletedGroups)
			tc.change(&c)
			tx := stageTx(t, f)
			if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
				tx.Rollback()
				t.Fatalf("invalid candidate: %v", err)
			}
			if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("failed stage committed: %v", err)
			}
		})
	}
}

func TestCJSIProjectionScopeAndGroupIdentity(t *testing.T) {
	ctx := t.Context()
	items := []TelemetrySession{{Client: "codex", SessionID: "one", FirstAt: "first", LastAt: "last"}, {Client: "claude", SessionID: "two", FirstAt: "first", LastAt: "last"}}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if e := ProjectTelemetrySessions(ctx, db, items[:1]); e != nil {
			t.Fatal(e)
		}
		for _, m := range []Metadata{{Client: "claude", SessionID: "one"}, {Client: "codex", SessionID: "sibling"}} {
			if err := ReplaceDocuments(ctx, db, m.Client, m.SessionID, nil); err != nil {
				t.Fatal(err)
			}
		}

	})
	path := TelemetrySourcePath("codex", "one")
	for _, tc := range []struct {
		name, setup           string
		projected             []TelemetrySession
		wantRemove, wantExtra int
		wantErr               bool
	}{
		{"multiple clients", "", items, 0, 0, false},
		{"duplicate", "", []TelemetrySession{items[0], items[0]}, 0, 0, true},
		{"blank client", "", []TelemetrySession{{SessionID: "one"}}, 0, 0, true},
		{"blank thread", "", []TelemetrySession{{Client: "codex"}}, 0, 0, true},
		{"path excluded", "INSERT INTO session_exclusions VALUES('path','otel:codex/one')", items, 1, 0, false},
		{"wrong path protection", "INSERT INTO cjsi_session_protections VALUES('other','', '',0)", items, 0, 0, false},
		{"path protection", "INSERT INTO cjsi_session_protections VALUES('otel:codex/one','', '',0)", items, 1, 0, false},
		{"other client protection", "INSERT INTO cjsi_session_protections VALUES('otel:codex/one','claude','one',0)", items, 0, 0, false},
		{"other session protection", "INSERT INTO cjsi_session_protections VALUES('otel:codex/one','codex','two',0)", items, 0, 0, false},
		{"same group protected", "INSERT INTO cjsi_session_protections VALUES('otel:codex/one','codex','one',0)", items, 1, 0, false},
		{"sibling session", `INSERT INTO session_metadata SELECT source_path,client,'sibling',project,model,parser_version,first_at,last_at FROM session_metadata WHERE source_path='otel:codex/one';
		INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id) VALUES('otel:codex/one','codex','sibling',randomblob(32))`, items, 0, 1, false},
		{"sibling client", `INSERT INTO session_metadata SELECT source_path,'claude',session_id,project,model,parser_version,first_at,last_at FROM session_metadata WHERE source_path='otel:codex/one';
		INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id) VALUES('otel:codex/one','claude','one',randomblob(32))`, items, 0, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			if tc.setup != "" {
				if _, e := tx.ExecContext(ctx, tc.setup); e != nil {
					t.Fatal(e)
				}
			}
			var c indexStageCandidate
			e := c.planTelemetry(ctx, tx, tc.projected)
			if tc.wantErr {
				if !errors.Is(e, errIndexStage) {
					t.Fatalf("invalid projection: %v", e)
				}
				return
			}
			if e != nil || len(c.DeletedSources) != tc.wantRemove || len(c.DeletedGroups) != tc.wantExtra {
				t.Fatalf("removed sources=%+v groups=%+v err=%v", c.DeletedSources, c.DeletedGroups, e)
			}
			expected := []TelemetrySession{items[1], items[0]}
			if tc.wantRemove == 1 {
				expected = expected[:1]
				if c.DeletedSources[0] != (indexSourceDelete{path, path}) {
					t.Fatal(c.DeletedSources)
				}
			}
			if !slices.Equal(c.Telemetry, expected) {
				t.Fatalf("ordered projection %+v want %+v", c.Telemetry, expected)
			}
		})
	}
}

func TestCJSIDocumentChangeWinnerAndReadScope(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		for _, entry := range []struct {
			p, text string
			prio    int
		}{{"middle", "old", 1}, {"losing", "unused", 0}} {
			adapterExec(t, db, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,1,'',?,6,'t')`, entry.p, entry.p, entry.prio)
			adapterExec(t, db, `INSERT INTO session_metadata VALUES(?,'codex','s','','',6,'','')`, entry.p)
			adapterExec(t, db, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,'codex','s','','user_prompt',?)`, entry.p, entry.text)
		}
	})
	replace := func(path, id string, prio int) indexStageCandidate {
		return indexStageCandidate{Replacements: []indexReplacement{{Selected: ingest.CodexSelectedSource{Path: path}, Metadata: Metadata{SourcePath: path, Client: "codex", SessionID: id, projectPriority: prio}, Material: nativeSessionUpdate{Documents: []nativeSessionDocument{{Document: Document{Client: "codex", SessionID: id, Kind: "user_prompt", Text: "new"}}}}}}}
	}
	for _, tc := range []struct {
		name                        string
		plan                        indexStageCandidate
		changes, removed, bodyReads int
	}{
		{"unchanged winner", indexStageCandidate{DeletedSources: []indexSourceDelete{{"losing", "losing"}}}, 0, 0, 0},
		{"same path replaces visible content", replace("middle", "s", 1), 1, 0, 1},
		{"equal priority earlier path", replace("a", "s", 1), 1, 0, 1},
		{"equal priority later path", replace("z", "s", 1), 0, 0, 0},
		{"new archive group", replace("archive", "brand-new", 0), 1, 0, 0},
		{"remove last copy", indexStageCandidate{DeletedSources: []indexSourceDelete{{"losing", "losing"}, {"middle", "middle"}}}, 0, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=ro"}
				trace := indexFaultTrace{counts: map[[2]string]int{}}
				db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
				defer db.Close()
				tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
				if err != nil {
					return err
				}
				defer tx.Rollback()
				s.Tx = tx
				changed, removed, err := tc.plan.affectedChanges(ctx, s)
				if err != nil {
					return err
				}
				reads := 0
				for _, site := range trace.sites {
					if site.operation == "query" && strings.HasPrefix(site.query, "SELECT event_at,kind,text FROM (") {
						reads++
					}
				}
				if changed != tc.changes || removed != tc.removed || reads != tc.bodyReads {
					t.Fatalf("changed/removed/body reads=%d/%d/%d want %d/%d/%d", changed, removed, reads, tc.changes, tc.removed, tc.bodyReads)
				}
				return nil
			})
		})
	}
}

func TestCJSIRollbackFailureDoesNotHideAdmissionError(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	fail := errors.New("rollback unavailable")
	trace := indexFaultTrace{rollbackFailure: fail, counts: map[[2]string]int{}}
	u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
	defer db.Close()
	for _, tc := range []struct {
		name string
		want error
		run  func(*sql.Tx) error
	}{
		{"stage", errIndexStage, func(tx *sql.Tx) error {
			return stageIndexCandidate(ctx, tx, indexStageAttempt{}, indexStageCandidate{})
		}},
		{"compact", store.ErrCJSIIndexChanged, func(tx *sql.Tx) error {
			return compactIndexCandidate(ctx, tx, indexStageAttempt{}, store.CJSIIndexStamp{}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = tc.run(tx)
			if !errors.Is(err, tc.want) || !errors.Is(err, fail) {
				t.Fatalf("lost admission or rollback error: %v", err)
			}
			err = tc.run(tx)
			if !errors.Is(err, tc.want) || errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("second refusal changed cause: %v", err)
			}
		})
	}
}

func TestCJSIExcludedPlanningAndProjectionStageFailures(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, true, func(db *sql.DB) {
		adapterExec(t, db, "INSERT INTO session_exclusions VALUES('session',?)", adapterThread)
	})
	src := fixturePreparedNative(t, f.path, adapterThread, `{"type":"session_meta","payload":{"id":"`+adapterThread+`"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"excluded"}}
`)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=ro"}
		trace := indexFaultTrace{target: indexFaultSite{"query", "SELECT identity FROM session_sources WHERE source_path=?", 1}, counts: map[[2]string]int{}}
		db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
		defer db.Close()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return err
		}
		defer tx.Rollback()
		pinned := s
		pinned.Tx = tx
		c, err := PlanCJSIIndex(ctx, pinned, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{src}, nil, nil)
		if !trace.fired || c != nil || !errors.Is(err, errIndexFixtureIO) {
			t.Fatalf("failed excluded-source lookup returned candidate=%v err=%v", c, err)
		}

		return nil
	})
	p := TelemetrySession{Client: "claude", SessionID: "telemetry", FirstAt: "first", LastAt: "last"}
	var c *CJSIIndexCandidate
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h = s.Selection()
		var err error
		rows := []TelemetrySession{p}
		c, err = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, nil, nil, &rows)
		return err
	})
	if c == nil {
		t.Fatal("missing projection plan")
	}
	exerciseIndexFailures(t, f, true, func(tx *sql.Tx) error { return c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, c.Seal()) })
	tx := stageTx(t, f)
	if _, err := tx.ExecContext(ctx, "INSERT INTO session_exclusions VALUES('session',?)", p.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, c.Seal()); !errors.Is(err, errIndexStage) {
		t.Fatalf("late exclusion: %v", err)
	}
	if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("rejected candidate committed: %v", err)
	}
}

func TestCJSIInventoryFailureBoundaries(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"user\",\"sessionId\":\"one\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if scan, err := Scan(ctx, db, home); err != nil || scan.Sources != 1 {
			t.Fatalf("legacy scan %+v err=%v", scan, err)
		}
	})
	sources, err := ingest.Discover(home)
	if err != nil || len(sources) != 1 {
		t.Fatalf("discover %+v err=%v", sources, err)
	}
	o := CJSISourceObservation{Source: sources[0], Kind: "claude-jsonl"}

	if ok, e := CJSISourcesUnchanged(ctx, store.CJSIIndexSnapshot{}, []CJSISourceObservation{o}); ok || !errors.Is(e, store.ErrCJSIIndexChanged) {
		t.Fatalf("unadmitted inventory: %v %v", ok, e)
	}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		exerciseIndexFailures(t, f, false, func(tx *sql.Tx) error {
			pinned := s
			pinned.Tx = tx
			ok, e := CJSISourceUnchanged(ctx, pinned, o)
			if e != nil && ok {
				t.Fatal("failed checkpoint read returned reusable")
			}
			return e
		})
		return nil
	})
}

func TestCJSIStageProtectionAndExclusionScope(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, true)
	base.Replacements[0].Metadata.Project = "/current"
	for _, tc := range []struct {
		name, source, client, id, kind, value string
		deny                                  bool
	}{
		{name: "whole source hold", source: f.path, deny: true},
		{name: "group hold", source: f.path, client: "codex", id: adapterThread, deny: true},
		{name: "other source hold", source: "elsewhere", client: "codex", id: adapterThread},
		{name: "other client hold", source: f.path, client: "claude", id: adapterThread},
		{name: "other group hold", source: f.path, client: "codex", id: "elsewhere"},
		{name: "project named like source path", kind: "project", value: f.path},
		{name: "session named like client", kind: "session", value: "codex"},
		{name: "project named like session", kind: "project", value: adapterThread},
		{name: "path named like proposed project", kind: "path", value: "/current"},
		{name: "path named like old project", kind: "path", value: "project"},
		{name: "excluded destination path", kind: "path", value: f.path, deny: true},
		{name: "other excluded path", kind: "path", value: "elsewhere"},
		{name: "excluded client", kind: "client", value: "codex", deny: true},
		{name: "other excluded client", kind: "client", value: "claude"},
		{name: "excluded logical session", kind: "session", value: adapterThread, deny: true},
		{name: "other excluded logical session", kind: "session", value: "other"},
		{name: "excluded proposed project", kind: "project", value: "/current", deny: true},
		{name: "excluded original project", kind: "project", value: "project", deny: true},
		{name: "other excluded project", kind: "project", value: "/other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			c := base
			if tc.source != "" {
				if _, err := tx.ExecContext(ctx, "INSERT INTO cjsi_session_protections VALUES(?,?,?,0)", tc.source, tc.client, tc.id); err != nil {
					t.Fatal(err)
				}
				// The candidate was admitted with the new protection revision;
				// this exercises membership itself, independently of stale stamps.
				if err := tx.QueryRowContext(ctx, "SELECT protection_revision FROM cjsi_index_state").Scan(&c.Attempt.Parent.ProtectionRevision); err != nil {
					t.Fatal(err)
				}
			} else if _, err := tx.ExecContext(ctx, "INSERT INTO session_exclusions VALUES(?,?)", tc.kind, tc.value); err != nil {
				t.Fatal(err)
			}
			if err := tx.QueryRowContext(ctx, "SELECT revision FROM serving_generation WHERE singleton=1").Scan(&c.Attempt.Parent.SessionRevision); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, "UPDATE cjsi_index_state SET base_revision=? WHERE singleton=1", c.Attempt.Parent.SessionRevision); err != nil {
				t.Fatal(err)
			}
			if err := tx.QueryRowContext(ctx, "SELECT protection_revision FROM cjsi_index_state WHERE singleton=1").Scan(&c.Attempt.Parent.ProtectionRevision); err != nil {
				t.Fatal(err)
			}
			c.Attempt.SessionRevision = c.Attempt.Parent.SessionRevision + 1
			err := stageIndexCandidate(ctx, tx, c.Attempt, c)
			if tc.deny {
				if !errors.Is(err, errIndexStage) {
					t.Fatalf("protected members were admitted: %v", err)
				}
				if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
					t.Fatalf("protected write remained committable: %v", err)
				}
			} else if err != nil {
				t.Fatalf("unrelated protection rejected valid members: %v", err)
			}
		})
	}
}

func TestCJSIPlanRejectsInvalidSourceAndOrdersRemovals(t *testing.T) {
	f := newPhysicalFixture(t, true, false)
	ctx := t.Context()
	native := fixturePreparedNative(t, filepath.Join(t.TempDir(), "one.jsonl"), adapterThread, `{"type":"session_meta","payload":{"id":"`+adapterThread+`"}}
`)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		invalid := native
		invalid.replacement.Round = 0
		c, err := PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{invalid}, nil, nil)
		if c != nil || !errors.Is(err, errIndexStage) {
			t.Fatalf("invalid producer coordinates: %+v %v", c, err)
		}
		invalid = native
		invalid.replacement.Material.Source.HeadTime = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
		c, err = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{invalid}, nil, nil)
		var encodingFailure *json.MarshalerError
		if c != nil || !errors.As(err, &encodingFailure) {
			t.Fatalf("unencodable manifest: %+v %v", c, err)
		}
		first := []CJSISourceRemoval{{"z", "aaa"}, {"a", "zzz"}, {"z", "aaa"}}
		second := []CJSISourceRemoval{{"a", "zzz"}, {"z", "aaa"}}
		c, err = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, nil, first, nil)
		if err != nil {
			return err
		}
		d, err := PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, nil, second, nil)
		if err != nil {
			return err
		}
		if c == nil || d == nil || c.Seal() != d.Seal() || !slices.Equal(c.plan.DeletedSources, []indexSourceDelete{{"a", "zzz"}, {"z", "aaa"}}) {
			t.Fatalf("removal receipt depends on caller ordering")
		}
		return nil
	})
}

func TestCJSICompactedNativeReuseNeedsMatchingRange(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	tx := stageTx(t, f.physicalFixture)
	if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	src := f.plan.Replacements[0]
	o := CJSISourceObservation{Kind: "codex-jsonl", Source: ingest.Source{Path: f.path, Client: "codex", Identity: src.Selected.Evidence.Identity, Stable: true, Size: src.Size, Priority: src.Metadata.projectPriority, ModifiedAt: src.ModifiedAt, ChangedAt: src.ChangedAt}}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, tc := range []struct{ name, change string }{
			{"canonical", ""},
			{"different source parser", "UPDATE session_sources SET parser_version=6 WHERE source_path=?"},
			{"cursor still behind", "UPDATE session_sources SET cursor=size-1 WHERE source_path=?"},
			{"one unclassified sibling", `INSERT INTO session_metadata SELECT source_path,client,'unclassified',project,model,parser_version,first_at,last_at FROM session_metadata WHERE source_path=?;
				INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id) SELECT source_path,client,session_id,randomblob(32) FROM session_metadata WHERE session_id='unclassified'`},
			{"unknown kind", "UPDATE cjsi_base_groups SET source_kind=NULL WHERE source_path=?"},
			{"wrong kind", "UPDATE cjsi_base_groups SET source_kind='synthetic' WHERE source_path=?"},
			{"not fully published", "UPDATE cjsi_base_ranges SET published_end=complete_end-1 WHERE source_path=?"},
			{"longer captured end", "UPDATE cjsi_base_ranges SET read_end=read_end+1 WHERE source_path=?"},
			{"pending replay", "UPDATE cjsi_base_ranges SET replay_from=0 WHERE source_path=?"},
			{"missing proof", "DELETE FROM cjsi_base_ranges WHERE source_path=?"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx := stageTx(t, f.physicalFixture)
				defer tx.Rollback()
				pinned := s
				pinned.Tx = tx
				if tc.change != "" {
					if _, e := tx.ExecContext(ctx, tc.change, f.path); e != nil {
						t.Fatal(e)
					}
				}
				got, err := CJSISourceUnchanged(ctx, pinned, o)
				if err != nil || got != (tc.change == "") {
					t.Fatalf("selected checkpoint: unchanged=%v err=%v", got, err)
				}
			})
		}
		return nil
	})
}

// One inventory observes a verified rollout moved into an archive directory
// and a new replica of the same thread at the original filename.
func TestCJSIMoveWithReoccupiedOrigin(t *testing.T) {
	for _, tc := range []struct {
		name             string
		reuse, precreate bool
		archive          string
	}{{"move only", false, false, "archive"}, {"new inode after original", true, false, "a-archive"}, {"new inode before original", true, true, "z-archive"}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			pending := filepath.Join(t.TempDir(), "pending")
			if tc.precreate {
				if err := os.WriteFile(pending, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			f := newCompactFixture(t, true, false, "complete")
			tx := stageTx(t, f.physicalFixture)
			if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(filepath.Dir(f.path), tc.archive)
			if err := os.MkdirAll(archive, 0700); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(archive, filepath.Base(f.path))
			if err := os.Rename(f.path, dest); err != nil {
				t.Fatal(err)
			}
			moved := fixturePreparedNative(t, dest, adapterThread, string(raw))
			moved.replacement.From = f.path
			if moved.replacement.Selected.Evidence.Identity != f.identity {
				t.Fatal("move lost inode identity")
			}
			sources := []CJSIPreparedSource{moved}
			var replacementID string
			if tc.reuse {
				if tc.precreate {
					if err := os.Rename(pending, f.path); err != nil {
						t.Fatal(err)
					}
				}
				newSource := fixturePreparedNative(t, f.path, adapterThread, `{"type":"session_meta","payload":{"id":"`+adapterThread+`"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"new copy"}}
`)
				if newSource.replacement.Selected.Evidence.Identity == f.identity {
					t.Fatal("replacement is not a new inode")
				}
				replacementID = newSource.replacement.Selected.Evidence.Identity
				if tc.precreate {
					sources = []CJSIPreparedSource{newSource, moved}
				} else {
					sources = append(sources, newSource)
				}
			}
			var c *CJSIIndexCandidate
			var h store.CJSIIndexStamp
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				h = s.Selection()
				var e error
				c, e = PlanCJSIIndex(ctx, s, h.Ticket+1, 1, h.SessionRevision+1, sources, nil, nil)
				return e
			})
			if c == nil {
				t.Fatal("missing move plan")
			}
			if !slices.Equal(c.plan.DeletedSources, []indexSourceDelete{{f.path, f.identity}}) {
				t.Fatalf("plan lost old tombstone: %+v", c.plan.DeletedSources)
			}
			var ids []int64
			for _, repl := range c.plan.Replacements {
				if repl.Selected.Path == dest {
					ids = repl.DocumentIDs
				}
			}
			if !slices.Equal(ids, f.plan.Replacements[0].DocumentIDs) {
				t.Fatalf("move failed to plan retained ids: %v", ids)
			}
			if tc.reuse {
				stageScalar(t, f.private, 1, "SELECT count(*) FROM session_sources WHERE identity='excluded' AND source_path<>?", f.path)
				f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
					bad, err := PlanCJSIIndex(ctx, s, h.Ticket+1, 1, h.SessionRevision+1, sources, []CJSISourceRemoval{{Path: f.path, Identity: "excluded"}}, nil)
					if err != nil {
						return err
					}
					if bad.Seal() == c.Seal() {
						t.Fatal("old identity missing from authenticated seal")
					}
					w := stageTx(t, f.physicalFixture)
					if err := bad.Stage(ctx, w, h, h.Ticket+1, 1, h.SessionRevision+1, bad.Seal()); !errors.Is(err, errIndexStage) {
						w.Rollback()
						t.Fatalf("stale old identity was accepted: %v", err)
					}
					if err := w.Commit(); !errors.Is(err, sql.ErrTxDone) {
						t.Fatalf("failed stage remained committable: %v", err)
					}
					return nil
				})
			}
			if tc.precreate {
				exerciseIndexFailures(t, f.physicalFixture, true, func(tx *sql.Tx) error { return c.Stage(ctx, tx, h, h.Ticket+1, 1, h.SessionRevision+1, c.Seal()) })
			}
			tx = stageTx(t, f.physicalFixture)
			err = c.Stage(ctx, tx, h, h.Ticket+1, 1, h.SessionRevision+1, c.Seal())
			t.Logf("retained IDs=%v removals=%d replacements=%d stage=%v", ids, len(c.plan.DeletedSources), len(c.plan.Replacements), err)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			ticket := h.Ticket + 1
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_candidate_sources WHERE ticket=? AND attempt=1 AND source_path=?`, ticket, f.path)
			op := "delete"
			if tc.reuse {
				op = "upsert"
			}
			stageScalar(t, f.private, op, `SELECT op FROM cjsi_candidate_sources WHERE ticket=? AND attempt=1 AND source_path=?`, ticket, f.path)
			seal := c.Seal()
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=1,session_revision=?,session_seal=?,base_ticket=?,base_attempt=?,base_revision=?,base_seal=?`,
				ticket, ticket, h.SessionRevision+1, seal[:], h.SessionTicket, h.SessionAttempt, h.SessionRevision, h.SessionSeal[:])
			var selected store.CJSIIndexStamp
			observe := func() {
				t.Helper()
				f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
					selected = s.Selection()
					order, err := ReadCJSIOrderSnapshot(ctx, s)
					if err != nil {
						return err
					}
					var atDest, atOrigin []int64
					for _, e := range order.Entries {
						switch e.SourcePath {
						case dest:
							atDest = append(atDest, e.DocumentID)
						case f.path:
							atOrigin = append(atOrigin, e.DocumentID)
						}
					}
					if !slices.Equal(atDest, []int64{8, 2}) {
						t.Fatalf("moved order: %v", atDest)
					}
					if tc.reuse && (len(atOrigin) != 1 || atOrigin[0] <= 8) || !tc.reuse && len(atOrigin) != 0 {
						t.Fatalf("old members selected at reused origin: %v", atOrigin)
					}
					return nil
				})
				f.read(t, func(reader CJSIReader) {
					copies, err := reader.EffectiveCopies(ctx, "codex", adapterThread)
					if err != nil {
						t.Fatal(err)
					}
					owners := map[string]string{}
					for _, copy := range copies {
						owners[copy.Metadata.SourcePath] = copy.Source.Identity
					}
					expected := map[string]string{dest: f.identity}
					if tc.reuse {
						expected[f.path] = replacementID
					}
					if !reflect.DeepEqual(owners, expected) {
						t.Fatalf("selected identities=%v want %v", owners, expected)
					}
					doc, err := reader.ShowWithActivity(ctx, "codex", adapterThread, false)
					if err != nil {
						t.Fatal(err)
					}
					want := "Astageword"
					if tc.reuse && f.path < dest {
						want = "new copy"
					}
					if docLabels(doc.Documents) != want {
						t.Fatalf("winning copy has stale content: %v want %s", doc.Documents, want)
					}
				})
			}
			observe()
			if tc.precreate {
				testReoccupiedOriginProof(t, f.physicalFixture, selected, f.path, f.identity, replacementID)
			}

			tx = stageTx(t, f.physicalFixture)
			if err := CompactCJSIIndex(ctx, tx, h, selected, func(context.Context) (store.CJSIIndexStamp, error) { return selected, nil }); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			observe()
			stageScalar(t, f.private, 0, "SELECT count(*) FROM session_sources WHERE source_path=? AND identity=?", f.path, f.identity)

		})
	}
}

func TestCJSIStageRejectsEmptyKeysBeforeWritingThem(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, false)
	u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	trace := indexFaultTrace{}
	db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
	defer db.Close()
	for _, tc := range []struct {
		name, table string
		invalid     func(*indexStageCandidate)
	}{
		{"missing parent seal", "cjsi_session_candidates", func(c *indexStageCandidate) { c.Attempt.Parent.SessionSeal = [32]byte{} }},
		{"invalid parent epoch", "cjsi_session_candidates", func(c *indexStageCandidate) { c.Attempt.Parent.IndexEpoch = 0 }},
		{"session ticket ahead of parent", "cjsi_session_candidates", func(c *indexStageCandidate) { c.Attempt.Parent.SessionTicket++ }},
		{"missing source path", "cjsi_candidate_sources", func(c *indexStageCandidate) { c.DeletedSources[0].Path = "" }},
		{"missing source identity", "cjsi_candidate_sources", func(c *indexStageCandidate) { c.DeletedSources[0].Identity = "" }},
		{"missing group path", "cjsi_candidate_groups", func(c *indexStageCandidate) { c.DeletedGroups[0].Path = "" }},
		{"missing group client", "cjsi_candidate_groups", func(c *indexStageCandidate) { c.DeletedGroups[0].Client = "" }},
		{"missing group ID", "cjsi_candidate_groups", func(c *indexStageCandidate) { c.DeletedGroups[0].SessionID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.DeletedSources = slices.Clone(base.DeletedSources)
			c.DeletedGroups = slices.Clone(base.DeletedGroups)
			tc.invalid(&c)
			trace = indexFaultTrace{counts: map[[2]string]int{}}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
				t.Fatalf("invalid input error: %v", err)
			}
			for _, op := range trace.sites {
				if op.operation == "exec" && strings.Contains(op.query, "INSERT INTO main."+tc.table) {
					t.Fatalf("attempted to stage an invalid key in %s", tc.table)
				}
			}
		})
	}
}

func TestCJSIMoveProofPrecedesZeroMemberWrites(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	archive := filepath.Join(t.TempDir(), filepath.Base(f.path))
	if err := os.Rename(f.path, archive); err != nil {
		t.Fatal(err)
	}
	base := stageFixturePlan(t, f, archive, false)
	for _, tc := range []struct {
		name, from string
		deletes    []indexSourceDelete
		want       bool
	}{
		{"verified move", f.path, []indexSourceDelete{{f.path, f.identity}}, true},
		{"undeclared origin", f.path, nil, false},
		{"implicit move", "", []indexSourceDelete{{f.path, f.identity}}, false},
		{"two possible origins", f.path, []indexSourceDelete{{f.path, f.identity}, {"alias", f.identity}}, false},
		{"last alias is also ambiguous", "alias", []indexSourceDelete{{f.path, f.identity}, {"alias", f.identity}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			// A second physical alias is possible in the inventory; a removal
			// of both aliases cannot establish which origin this move preserves.
			if _, e := tx.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
			SELECT 'alias',identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM session_sources WHERE source_path=?`, f.path); e != nil {
				t.Fatal(e)
			}
			var revision int64
			if err := tx.QueryRow("SELECT revision FROM serving_generation").Scan(&revision); err != nil {
				t.Fatal(err)
			}
			if _, e := tx.Exec("UPDATE cjsi_index_state SET base_revision=?", revision); e != nil {
				t.Fatal(e)
			}
			c := base
			c.Attempt.Parent.SessionRevision = revision
			c.Attempt.SessionRevision = revision + 1
			c.Replacements = slices.Clone(base.Replacements)
			c.Replacements[0].From = tc.from
			c.DeletedSources = tc.deletes
			err := stageIndexCandidate(ctx, tx, c.Attempt, c)
			if tc.want && err != nil || !tc.want && !errors.Is(err, errIndexStage) {
				t.Fatalf("move admission=%v want accepted=%v", err, tc.want)
			}
		})
	}
}

func TestCJSIReoccupationNeedsADifferentInode(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, false)
	c.DeletedSources = append(c.DeletedSources, indexSourceDelete{f.path, f.identity})
	tx := stageTx(t, f)
	if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
		tx.Rollback()
		t.Fatalf("same-inode deletion and upsert were accepted: %v", err)
	}
	if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("committed contradictory source operations: %v", err)
	}
}

func TestCJSIStageAuthenticatesPhysicalBase(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, false)
	for _, tc := range []struct{ name, alter string }{
		{"recorded epoch", "UPDATE cjsi_layout_initial_seal SET index_epoch=index_epoch+1"},
		{"recorded initial seal", "UPDATE cjsi_layout_initial_seal SET seal_digest=randomblob(32)"},
		{"actual serving revision", "UPDATE serving_generation SET revision=revision+1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			if _, err := tx.ExecContext(ctx, tc.alter); err != nil {
				t.Fatal(err)
			}
			if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
				tx.Rollback()
				t.Fatalf("wrong physical baseline accepted: %v", err)
			}
			if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("rejected write committable: %v", err)
			}
		})
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(0)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
		tx.Rollback()
		t.Fatalf("disabled foreign keys: %v", err)
	}
}

func TestCJSIStageSealsOnlyAdmittedDraft(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, false)
	h := c.Attempt.Parent
	for _, key := range [][2]int64{{2, c.Attempt.Attempt}, {c.Attempt.Ticket, 9}} {
		adapterExec(t, f.private, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, key[0], key[1], h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.Ticket, h.ProtectionRevision, h.CoverageRevision, c.Attempt.SessionRevision)
	}
	tx := stageTx(t, f)
	if err := stageIndexCandidate(ctx, tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	stageScalar(t, f.private, 2, "SELECT count(*) FROM cjsi_session_candidates WHERE seal_digest IS NULL")
	stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_session_candidates WHERE ticket=? AND attempt=? AND seal_digest=?", c.Attempt.Ticket, c.Attempt.Attempt, c.Attempt.Seal[:])
}

func TestCJSIGroupTombstoneIdentifiesWholeKey(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, true, func(db *sql.DB) {
		adapterExec(t, db, `INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			SELECT source_path,'claude',session_id,project,model,parser_version,first_at,last_at FROM session_metadata WHERE client='codex' AND session_id=?`, adapterThread)
	})
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error { h = s.Selection(); return nil })
	c := indexStageCandidate{Attempt: indexStageAttempt{Parent: h, Ticket: 1, Attempt: 1, SessionRevision: h.SessionRevision + 1, Seal: sha256.Sum256([]byte("one group"))},
		DeletedGroups: []indexGroupDelete{{Path: f.path, Client: "codex", SessionID: adapterThread, Sequence: sha256.Sum256([]byte("tombstone"))}}}
	tx := stageTx(t, f)
	if err := stageIndexCandidate(ctx, tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_candidate_groups")
	stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_candidate_groups WHERE source_path=? AND client='codex' AND session_id=? AND op='delete'", f.path, adapterThread)
}

func TestCJSIReplicaFallbackReadFailures(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		exerciseIndexFailures(t, f, false, func(tx *sql.Tx) error {
			pinned := s
			pinned.Tx = tx
			c, err := PlanCJSIIndex(ctx, pinned, h.Ticket+1, 1, h.SessionRevision+1, nil, []CJSISourceRemoval{{Path: f.path, Identity: f.identity}}, nil)
			if err != nil && c != nil {
				t.Fatal("failed fallback query returned a plan")
			}
			return err
		})
		return nil
	})
}

func TestCJSIRetainedKeyUsesOriginalInodeAndPayloadOwner(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	base := stageFixturePlan(t, f, f.path, true).Replacements[0]
	for _, tc := range []string{"new inode at same path", "FTS row now owned by another group", "other client only"} {
		t.Run(tc, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			r := base
			switch tc {
			case "new inode at same path":
				r.Material.Evidence.Identity = "new-inode"
			case "FTS row now owned by another group":
				if _, err := tx.Exec("UPDATE session_documents SET source_path='other' WHERE rowid=1"); err != nil {
					t.Fatal(err)
				}
			case "other client only":
				for _, q := range []string{
					`INSERT INTO session_metadata SELECT source_path,'claude',session_id,project,model,parser_version,first_at,last_at FROM session_metadata WHERE source_path=? AND session_id=?`,
					`INSERT INTO cjsi_base_groups SELECT source_path,'claude',session_id,order_verified,sequence_id,head_at,birth,source_kind FROM cjsi_base_groups WHERE source_path=? AND session_id=?`,
					`INSERT INTO cjsi_base_ranges SELECT source_path,'claude',session_id,workset_round,workset_attempt,identity,selection_epoch,generation,replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest FROM cjsi_base_ranges WHERE source_path=? AND session_id=?`,
				} {
					if _, err := tx.ExecContext(ctx, q, f.path, adapterThread); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := tx.Exec("UPDATE cjsi_base_order SET client='claude' WHERE doc_id=1; UPDATE session_documents SET client='claude' WHERE rowid=1"); err != nil {
					t.Fatal(err)
				}
			}
			id, err := r.baseDocument(ctx, tx, f.path, base.Material.Documents[0])
			if id != 0 || err != nil {
				t.Fatalf("retained another source's ID: %d err=%v", id, err)
			}
		})
	}
}

func TestCJSIDocMaskRequiresExactOwner(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, true, func(db *sql.DB) {
		adapterExec(t, db, `INSERT INTO session_metadata SELECT m.source_path,'claude',m.session_id,m.project,m.model,m.parser_version,m.first_at,m.last_at
			FROM session_metadata m JOIN session_sources s USING(source_path) WHERE m.client='codex' AND m.session_id=? AND s.priority=3`, adapterThread)
		adapterExec(t, db, `INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			SELECT 8,source_path,client,session_id,'','user_prompt','other client' FROM session_metadata WHERE client='claude'`)
	})
	base := stageFixturePlan(t, f, f.path, false)
	base.DeletedGroups = append(base.DeletedGroups, indexGroupDelete{f.path, "claude", adapterThread, sha256.Sum256([]byte("other client group"))})
	for _, tc := range []struct {
		name  string
		id    int64
		alter bool
		want  bool
	}{{"owned", 1, false, true}, {"other session", 4, false, false}, {"other client", 8, false, false}, {"wrong FTS owner", 1, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.Replacements = slices.Clone(base.Replacements)
			c.Replacements[0].DeletedDocumentIDs = []int64{tc.id}
			tx := stageTx(t, f)
			defer tx.Rollback()
			if tc.alter {
				if _, err := tx.Exec("UPDATE session_documents SET session_id='unrelated' WHERE rowid=1"); err != nil {
					t.Fatal(err)
				}
			}
			err := stageIndexCandidate(ctx, tx, c.Attempt, c)
			if tc.want && err != nil || !tc.want && !errors.Is(err, errIndexStage) {
				t.Fatalf("doc mask accepted=%v err=%v", tc.want, err)
			}
		})
	}
}

func TestCJSIInventoryAdmissionDoesNotOpenInvalidObservations(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Refusing malformed input requires no access to private state.
	unavailable := store.CJSIIndexSnapshot{Tx: tx}
	o := CJSISourceObservation{Kind: "claude-jsonl", Source: ingest.Source{Path: "/one", Identity: "inode", Client: "claude", ChangedAt: 1, Stable: true}}
	for _, change := range []func(*CJSISourceObservation){
		func(o *CJSISourceObservation) { o.Path = "" }, func(o *CJSISourceObservation) { o.Identity = "" }, func(o *CJSISourceObservation) { o.ChangedAt = 0 }, func(o *CJSISourceObservation) { o.Client = "codex" },
	} {
		bad := o
		change(&bad)
		got, err := CJSISourcesUnchanged(ctx, unavailable, []CJSISourceObservation{bad})
		if got || err != nil {
			t.Fatalf("invalid observation attempted state read: %+v %v", bad, err)
		}
	}
	o.Path = ""
	if got, err := CJSISourceUnchanged(ctx, store.CJSIIndexSnapshot{}, o); got || err != nil {
		t.Fatalf("absent single path entered snapshot: %v %v", got, err)
	}
}

func TestCJSIInventoryRejectsMixedKindAndLateCancellation(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"user\",\"sessionId\":\"s\",\"message\":{\"role\":\"user\",\"content\":\"ok\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if _, err := Scan(ctx, db, home); err != nil {
			t.Fatal(err)
		}
	})
	discovered, err := ingest.Discover(home)
	if err != nil || len(discovered) != 1 {
		t.Fatal(discovered, err)
	}
	o := CJSISourceObservation{Source: discovered[0], Kind: "claude-jsonl"}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, tc := range []struct{ name, client, kind string }{{"different kinds", "claude", "synthetic"}, {"foreign legacy client", "another-client", "claude-jsonl"}} {
			t.Run(tc.name, func(t *testing.T) {
				tx := stageTx(t, f)
				defer tx.Rollback()
				if _, e := tx.Exec("UPDATE cjsi_base_groups SET source_kind='claude-jsonl'"); e != nil {
					t.Fatal(e)
				}
				if _, e := tx.Exec("INSERT INTO session_metadata VALUES(?,?,'other','','',6,'','')", path, tc.client); e != nil {
					t.Fatal(e)
				}
				if _, e := tx.Exec("INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id,source_kind) VALUES(?,?,'other',randomblob(32),?)", path, tc.client, tc.kind); e != nil {
					t.Fatal(e)
				}
				pinned := s
				pinned.Tx = tx
				got, e := CJSISourceUnchanged(ctx, pinned, o)
				if e != nil || got {
					t.Fatalf("incompatible groups accepted: %v %v", got, e)
				}
			})
		}
		closed := false
		trace := indexFaultTrace{counts: map[[2]string]int{}, rowsClosed: func(string) { closed = true }}
		u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=ro"}
		db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
		defer db.Close()
		tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return e
		}
		defer tx.Rollback()
		pinned := s
		pinned.Tx = tx
		cancelable, cancel := context.WithCancel(ctx)
		defer cancel()
		checkCtx := indexObservationContext{cancelable, func() {
			if closed {
				cancel()
			}
		}}
		got, e := CJSISourcesUnchanged(checkCtx, pinned, []CJSISourceObservation{o})
		if got || !errors.Is(e, context.Canceled) {
			t.Fatalf("cancellation at observation recheck: %v %v", got, e)
		}
		return nil
	})
}

func TestCJSIStageRequiresSiblingDeletesFromSameAttempt(t *testing.T) {
	ctx := t.Context()
	for _, key := range [][2]int64{{2, 2}, {1, 3}} {
		t.Run(fmt.Sprintf("ticket_%d_attempt_%d", key[0], key[1]), func(t *testing.T) {
			f := newPhysicalFixture(t, true, true)
			c := stageFixturePlan(t, f, f.path, false)
			other := indexStageCandidate{Attempt: c.Attempt, DeletedGroups: slices.Clone(c.DeletedGroups)}
			other.Attempt.Ticket, other.Attempt.Attempt = key[0], key[1]
			other.Attempt.Seal = sha256.Sum256([]byte("unselected deletion"))
			tx := stageTx(t, f)
			if err := stageIndexCandidate(ctx, tx, other.Attempt, other); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			c.DeletedGroups = c.DeletedGroups[:1]
			tx = stageTx(t, f)
			if err := stageIndexCandidate(ctx, tx, c.Attempt, c); !errors.Is(err, errIndexStage) {
				tx.Rollback()
				t.Fatalf("another attempt's sibling deletion admitted: %v", err)
			}
			stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_session_candidates WHERE ticket=? AND attempt=?", c.Attempt.Ticket, c.Attempt.Attempt)
		})
	}
}

func TestCJSIOldProjectExclusionUsesExactGroup(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, true, func(db *sql.DB) {
		adapterExec(t, db, `INSERT INTO session_metadata SELECT m.source_path,'claude',m.session_id,'/foreign-client',m.model,m.parser_version,m.first_at,m.last_at
			FROM session_metadata m JOIN session_sources s USING(source_path) WHERE m.client='codex' AND m.session_id=? AND s.priority=3`, adapterThread)
		adapterExec(t, db, "UPDATE session_metadata SET project='/other-replica' WHERE source_path='replica'")
		adapterExec(t, db, "UPDATE session_metadata SET project='/other-session' WHERE session_id='masked'")
	})
	base := stageFixturePlan(t, f, f.path, false)
	base.DeletedGroups = append(base.DeletedGroups, indexGroupDelete{f.path, "claude", adapterThread, sha256.Sum256([]byte("retired sibling"))})
	for _, project := range []string{"/other-session", "/other-replica", "/foreign-client"} {
		t.Run(filepath.Base(project), func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			c := base
			if _, e := tx.Exec("INSERT INTO session_exclusions VALUES('project',?)", project); e != nil {
				t.Fatal(e)
			}
			if e := tx.QueryRow("SELECT revision FROM serving_generation").Scan(&c.Attempt.Parent.SessionRevision); e != nil {
				t.Fatal(e)
			}
			if _, e := tx.Exec("UPDATE cjsi_index_state SET base_revision=?", c.Attempt.Parent.SessionRevision); e != nil {
				t.Fatal(e)
			}
			if e := tx.QueryRow("SELECT protection_revision FROM cjsi_index_state").Scan(&c.Attempt.Parent.ProtectionRevision); e != nil {
				t.Fatal(e)
			}
			c.Attempt.SessionRevision = c.Attempt.Parent.SessionRevision + 1
			if err := stageIndexCandidate(ctx, tx, c.Attempt, c); err != nil {
				t.Fatalf("unrelated old group blocked staging: %v", err)
			}
		})
	}
}

func TestCJSIPlanRejectsBareSnapshot(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		removed := []CJSISourceRemoval{{Path: "/removed", Identity: "prior"}}
		c, err := PlanCJSIIndex(ctx, s, 1, 1, s.Selection().SessionRevision, nil, removed, nil)
		if c == nil || err != nil {
			t.Fatalf("admitted snapshot: candidate=%v err=%v", c, err)
		}
		bare := store.CJSIIndexSnapshot{Tx: s.Tx}
		c, err = PlanCJSIIndex(ctx, bare, 1, 1, s.Selection().SessionRevision, nil, removed, nil)
		if c != nil || err == nil {
			t.Fatalf("bare snapshot: candidate=%v err=%v", c, err)
		}
		return nil
	})
}

func TestCJSIPlanningNeedsCompactedParent(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	check := func(compacted bool) {
		t.Helper()
		f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
			c, err := PlanCJSIIndex(ctx, s, s.Selection().Ticket+1, 1, s.Selection().SessionRevision, nil, nil, nil)
			if c != nil || compacted && err != nil || !compacted && !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("compacted=%v: plan=%v err=%v", compacted, c, err)
			}
			return nil
		})
	}
	check(false)
	tx := stageTx(t, f.physicalFixture)
	if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(true)
	// Tx is exported. A copy of an admitted snapshot can carry a transaction
	// that sees a different compacted parent while retaining the original stamp.
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		for _, column := range []string{"compacted_ticket", "compacted_attempt", "base_revision"} {
			t.Run(column, func(t *testing.T) {
				tx := stageTx(t, f.physicalFixture)
				defer tx.Rollback()
				if _, err := tx.ExecContext(ctx, "UPDATE cjsi_index_state SET "+column+"="+column+"+1"); err != nil {
					t.Fatal(err)
				}
				other := s
				other.Tx = tx
				c, err := PlanCJSIIndex(ctx, other, h.Ticket+1, 1, h.SessionRevision, nil, nil, nil)
				if c != nil || !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("mismatched %s: candidate=%v err=%v", column, c, err)
				}
			})
		}
		return nil
	})
}

func TestCJSIAffectedChangesUsesExactPhysicalAndLogicalKeys(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		for _, src := range []struct {
			path string
			prio int
		}{{"touched", 5}, {"alternate", 1}, {"unrelated", 10}} {
			adapterExec(t, db, "INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,'',?,6,'t')", src.path, src.path, src.prio)
		}
		for _, g := range []struct {
			path, client, id string
			count            int
		}{{"touched", "codex", "s", 3}, {"touched", "claude", "s", 2}, {"alternate", "codex", "s", 1}, {"unrelated", "codex", "other", 1}, {"unrelated", "claude", "s", 1}} {
			adapterExec(t, db, "INSERT INTO session_metadata VALUES(?,?,?,'','',6,'','')", g.path, g.client, g.id)
			for i := 0; i < g.count; i++ {
				adapterExec(t, db, "INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,'','user_prompt',?)", g.path, g.client, g.id, fmt.Sprintf("%s %s %d", g.path, g.client, i))
			}
		}
	})
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		trace := indexFaultTrace{counts: map[[2]string]int{}}
		u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=ro"}
		db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
		defer db.Close()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return err
		}
		defer tx.Rollback()
		s.Tx = tx
		c := indexStageCandidate{DeletedSources: []indexSourceDelete{{"touched", "touched"}}}
		changed, removed, err := c.affectedChanges(ctx, s)
		if err != nil {
			return err
		}
		winners := 0
		for _, site := range trace.sites {
			if site.operation == "query" && strings.HasPrefix(site.query, "SELECT source_path,priority FROM (") {
				winners++
			}
		}
		// Only codex/s switches winners. The higher-priority claude/s at
		// "unrelated" was selected throughout; codex/other was never touched.
		if changed != 1 || removed != 2 || winners != 2 {
			t.Fatalf("changed/removed/affected groups=%d/%d/%d want 1/2/2", changed, removed, winners)
		}
		return nil
	})
}

func TestCJSISelectedActivityProofMustHaveFullDigest(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error { h = s.Selection(); return nil })
	f.advance(t, h, len(f.prefix))
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, tc := range []string{"short digest", "negative document id", "document beyond allocator"} {
			t.Run(tc, func(t *testing.T) {
				fired := false
				trace := indexFaultTrace{counts: map[[2]string]int{}, corruptRow: func(q string, row []driver.Value) {
					if tc == "short digest" && strings.HasPrefix(q, "SELECT source_path,session_id,identity,selection_epoch,") {
						row[len(row)-1] = []byte{1}
						fired = true
					}
					if tc != "short digest" && strings.HasPrefix(q, "SELECT m.source_path,m.client,m.session_id,m.doc_id,m.detail_ordinal,c.order_verified") {
						if tc == "negative document id" {
							row[3] = int64(-1)
						} else {
							row[3] = int64(100)
						}
						fired = true
					}
				}}
				u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=ro"}
				db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
				defer db.Close()
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				pinned := s
				pinned.Tx = tx
				if tc == "short digest" {
					src, e := selectedActivitySource(ctx, pinned, Metadata{Client: "codex", SessionID: adapterThread, SourcePath: f.path})
					if !fired || src != (ingest.CodexSelectedSource{}) || !errors.Is(e, ingest.ErrSourceChanged) {
						t.Fatalf("invalid source proof: fired=%v src=%v err=%v", fired, src, e)
					}
				} else {
					order, e := ReadCJSIOrderSnapshot(ctx, pinned)
					if !fired || !reflect.DeepEqual(order, CJSIOrderSnapshot{}) || !errors.Is(e, store.ErrCJSIIndexChanged) {
						t.Fatalf("invalid order: fired=%v order=%v err=%v", fired, order, e)
					}
				}
			})
		}
		return nil
	})
}

func TestCJSITelemetryStageHonorsCurrentGroupProtection(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	p := TelemetrySession{Client: "claude", SessionID: "settled", FirstAt: "first", LastAt: "last"}
	path := TelemetrySourcePath(p.Client, p.SessionID)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		a := indexStageAttempt{Parent: h, Ticket: 1, Attempt: 1, SessionRevision: h.SessionRevision + 1, Seal: sha256.Sum256([]byte("projection"))}
		for _, tc := range []struct {
			name, src, client, id string
			excluded, want        bool
		}{
			{"clear", "", "", "", false, true},
			{"held source", path, "", "", false, false}, {"held logical group", path, p.Client, p.SessionID, false, false},
			{"other path", "different", p.Client, p.SessionID, false, true}, {"other client", path, "codex", p.SessionID, false, true}, {"other session", path, p.Client, "sibling", false, true},
			{"late exclusion", "", "", "", true, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx := stageTx(t, f)
				defer tx.Rollback()
				if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.Ticket, a.Attempt, h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.Ticket, h.ProtectionRevision, h.CoverageRevision, a.SessionRevision); err != nil {
					t.Fatal(err)
				}
				if tc.src != "" {
					if _, err := tx.Exec("INSERT INTO cjsi_session_protections VALUES(?,?,?,0)", tc.src, tc.client, tc.id); err != nil {
						t.Fatal(err)
					}
				}
				if tc.excluded {
					if _, err := tx.Exec("INSERT INTO session_exclusions VALUES('session',?)", p.SessionID); err != nil {
						t.Fatal(err)
					}
				}
				err := stageCJSITelemetry(ctx, tx, a, p)
				if tc.want && err != nil || !tc.want && !errors.Is(err, errIndexStage) {
					t.Fatalf("projection accepted=%v err=%v", tc.want, err)
				}
				var count int
				if e := tx.QueryRow("SELECT count(*) FROM cjsi_candidate_sources WHERE ticket=1 AND attempt=1").Scan(&count); e != nil || count != 0 && !tc.want || count != 1 && tc.want {
					t.Fatalf("draft source count=%d err=%v", count, e)
				}
			})
		}
		return nil
	})
}

func TestCJSITelemetryPlannerRefusesUnprovedSibling(t *testing.T) {
	ctx := t.Context()
	projection := []TelemetrySession{{Client: "codex", SessionID: "known", FirstAt: "first", LastAt: "last"}}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if err := ProjectTelemetrySessions(ctx, db, projection); err != nil {
			t.Fatal(err)
		}
	})
	tx := stageTx(t, f)
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_metadata SELECT source_path,client,'unproved-sibling',project,model,parser_version,first_at,last_at FROM session_metadata`); err != nil {
		t.Fatal(err)
	}
	var c indexStageCandidate
	if err := c.planTelemetry(ctx, tx, projection); !errors.Is(err, errIndexStage) {
		t.Fatalf("retired an unproved group: %v", err)
	}
}

func TestCJSISelectedSnapshotListsEachReplicaOnce(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	f.read(t, func(reader CJSIReader) {
		s, err := reader.ReadIndexSnapshot(ctx, "A OR loserword")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, g := range s.Sessions {
			if g.SessionID == adapterThread {
				found = true
				seen := map[string]int{}
				for _, copy := range g.Copies {
					seen[copy.Identity]++
				}
				if len(seen) != 2 || seen[f.identity] != 1 || seen["second"] != 1 || g.SourcePath != f.path {
					t.Fatalf("replica selection: %+v", g)
				}
			}
		}
		if !found || docLabels(s.Documents) != "A" {
			t.Fatalf("selected snapshot missing or mixed documents: %+v", s)
		}
	})
}

func TestCJSIPlanStopsBeforeUnneededSourceIO(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	path := filepath.Join(t.TempDir(), "budget.jsonl")
	body := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"first"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"second"}}
`
	src := fixturePreparedNative(t, path, adapterThread, body)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, exclusion := range []bool{false, true} {
			func() {
				trace := indexFaultTrace{counts: map[[2]string]int{}}
				u := url.URL{Scheme: "file", Path: filepath.Join(f.root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
				db := sql.OpenDB(indexFaultConnector{u.String(), &trace})
				defer db.Close()
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if exclusion {
					if _, err := tx.Exec("INSERT INTO session_exclusions VALUES('session',?)", adapterThread); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := tx.Exec("UPDATE cjsi_index_state SET doc_id_high_water=9223372036854775807"); err != nil {
						t.Fatal(err)
					}
				}
				pinned := s
				pinned.Tx = tx
				c, err := PlanCJSIIndex(ctx, pinned, 1, 1, s.Selection().SessionRevision+1, []CJSIPreparedSource{src}, nil, nil)
				if c != nil || exclusion && err != nil || !exclusion && !errors.Is(err, errIndexStage) {
					t.Fatal(c, err)
				}
				lookups, members := 0, 0
				for _, site := range trace.sites {
					if site.operation == "query" {
						switch {
						case strings.HasPrefix(site.query, "SELECT identity FROM session_sources WHERE"):
							lookups++
						case strings.HasPrefix(site.query, "SELECT b.doc_id,b.record_end"):
							members++
						}
					}
				}
				if exclusion && (lookups != 1 || members != 0) || !exclusion && (lookups != 0 || members != 1) {
					t.Fatalf("exclusion=%v: path lookups=%d member lookups=%d", exclusion, lookups, members)
				}
			}()
		}
		return nil
	})
}

func TestCJSIExistingTelemetryPlanReadFailures(t *testing.T) {
	ctx := t.Context()
	projections := []TelemetrySession{{Client: "codex", SessionID: "old", Model: "m", FirstAt: "first", LastAt: "last"}}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if err := ProjectTelemetrySessions(ctx, db, projections); err != nil {
			t.Fatal(err)
		}
	})
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h := s.Selection()
		exerciseIndexFailures(t, f, false, func(tx *sql.Tx) error {
			pinned := s
			pinned.Tx = tx
			c, err := PlanCJSIIndex(ctx, pinned, 1, 1, h.SessionRevision+1, nil, nil, &projections)
			if err != nil && c != nil {
				t.Fatal("failed telemetry projection returned a plan")
			}
			return err
		})
		return nil
	})
}

func TestCJSIEmptyLegacyRowDoesNotStandInForDiscoveredFile(t *testing.T) {
	ctx := t.Context()
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		adapterExec(t, db, "INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at) VALUES('old-row','',0,0,0,0,'',0,0,'')")
	})
	path := filepath.Join(t.TempDir(), "absent-from-index.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	id, changed, stable := ingest.FileGeneration(info)
	o := CJSISourceObservation{Kind: "claude-jsonl", Source: ingest.Source{Path: path, Client: "claude", Stable: stable, Identity: id, ChangedAt: changed, ModifiedAt: info.ModTime().UnixNano(), Size: info.Size()}}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		ok, err := CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{o})
		if ok || err != nil {
			t.Fatalf("unrelated row in inventory: %v %v", ok, err)
		}
		return nil
	})
}

func TestCJSIFileInventoryScopesDraftsAndUntouchedFiles(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	claude := filepath.Join(home, ".claude", "projects", "p", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(claude), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte("{\"type\":\"user\",\"sessionId\":\"one\",\"message\":{\"role\":\"user\",\"content\":\"hello\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if _, err := Scan(ctx, db, home); err != nil {
			t.Fatal(err)
		}
		adapterExec(t, db, "INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES('gone','retired',0,0,1,'',0,6,'t')")
	})
	seen, err := ingest.Discover(home)
	if err != nil || len(seen) != 1 {
		t.Fatal(seen, err)
	}
	legacy := CJSISourceObservation{Source: seen[0], Kind: "claude-jsonl"}
	path := filepath.Join(t.TempDir(), "native.jsonl")
	src := fixturePreparedNative(t, path, adapterThread, `{"type":"session_meta","payload":{"id":"`+adapterThread+`"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"native"}}
`)
	r := src.replacement
	native := CJSISourceObservation{Source: ingest.Source{Path: path, Client: "codex", Priority: r.Metadata.projectPriority, Identity: r.Selected.Evidence.Identity, Size: r.Size, ModifiedAt: r.ModifiedAt, ChangedAt: r.ChangedAt, Stable: true}, Kind: "codex-jsonl"}
	var h store.CJSIIndexStamp
	var c *CJSIIndexCandidate
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h = s.Selection()
		var e error
		c, e = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{src}, []CJSISourceRemoval{{"gone", "retired"}}, nil)
		return e
	})
	if c == nil {
		t.Fatal("missing plan")
	}
	// Leave another draft for each component of the selection key.
	for _, key := range [][2]int64{{1, 2}, {2, 1}} {
		adapterExec(t, f.private, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, key[0], key[1], h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.Ticket, h.ProtectionRevision, h.CoverageRevision, h.SessionRevision+1)
		adapterExec(t, f.private, `INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at) VALUES(?,?,?,'upsert','codex-jsonl','wrong',3,7,0,0,0,0,'','t')`, key[0], key[1], path)
	}
	tx := stageTx(t, f)
	// A delete's optional columns are not membership: an old writer may
	// retain checkpoint metadata on the tombstone. Only op decides visibility.
	if _, e := tx.Exec(`CREATE TEMP TRIGGER preserve_old_columns BEFORE UPDATE OF seal_digest ON main.cjsi_session_candidates BEGIN
		UPDATE cjsi_candidate_sources SET identity='retired',priority=0,parser_version=6,complete_end=0,size=0,modified_at=1,changed_at=1,prefix_hash='old',scanned_at='t'
		WHERE ticket=NEW.ticket AND attempt=NEW.attempt AND op='delete'; END`); e != nil {
		t.Fatal(e)
	}
	seal := c.Seal()
	if err := c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, seal); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?`, h.SessionRevision+1, seal[:])
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, o := range []CJSISourceObservation{native, legacy} {
			ok, e := CJSISourceUnchanged(ctx, s, o)
			if !ok || e != nil {
				t.Fatalf("selected %s: unchanged=%v err=%v", o.Kind, ok, e)
			}
		}
		ok, e := CJSISourcesUnchanged(ctx, s, []CJSISourceObservation{native, legacy})
		if !ok || e != nil {
			t.Fatalf("selected inventory: %v %v", ok, e)
		}
		return nil
	})
}

func TestCJSINativeRangeCannotBeBorrowedFromHardLink(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	one := filepath.Join(dir, "one.jsonl")
	alias := filepath.Join(dir, "alias.jsonl")
	body := []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"same\"}}\n")
	if err := os.WriteFile(one, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(one, alias); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(one)
	if err != nil {
		t.Fatal(err)
	}
	id, ctime, stable := ingest.FileGeneration(st)
	size := int64(len(body))
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		for _, path := range []string{one, alias} {
			adapterExec(t, db, "INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,'h',1,7,'t')", path, id, size, size, st.ModTime().UnixNano(), ctime)
			adapterExec(t, db, "INSERT INTO session_metadata VALUES(?,'codex','same','','',7,'','')", path)
		}
	})
	adapterExec(t, f.private, "UPDATE cjsi_base_groups SET source_kind='codex-jsonl',order_verified=1")
	hash := sha256.Sum256(body)
	adapterExec(t, f.private, `INSERT INTO cjsi_base_ranges VALUES(?,'codex','same',1,1,?,1,1,1,4096,?,?,?,?,?)`, one, id, size, size, size, size, hash[:])
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, path := range []string{one, alias} {
			o := CJSISourceObservation{Source: ingest.Source{Path: path, Client: "codex", Identity: id, Priority: 1, Size: size, ModifiedAt: st.ModTime().UnixNano(), ChangedAt: ctime, Stable: stable}, Kind: "codex-jsonl"}
			got, e := CJSISourceUnchanged(ctx, s, o)
			if got != (path == one) || e != nil {
				t.Fatalf("%s reused=%v err=%v", filepath.Base(path), got, e)
			}
		}
		return nil
	})
}

func TestCJSICursorHintCannotBeBorrowedFromAlias(t *testing.T) {
	ctx := t.Context()
	one := filepath.Join(t.TempDir(), "one.jsonl")
	alias := filepath.Join(t.TempDir(), "alias.jsonl")
	body := []byte("{}\n{")
	if err := os.WriteFile(one, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(one, alias); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(one)
	if err != nil {
		t.Fatal(err)
	}
	id, ctime, stable := ingest.FileGeneration(st)
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		for _, path := range []string{one, alias} {
			adapterExec(t, db, `INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at)
				VALUES(?,?,4,x'7b',4,?,?,'same',1,6,'t')`, path, id, st.ModTime().UnixNano(), ctime)
			adapterExec(t, db, "INSERT INTO session_metadata VALUES(?,'claude','s','','',6,'','')", path)
		}
	})
	adapterExec(t, f.private, "DELETE FROM cjsi_legacy_cursor_hints WHERE source_path=?", alias)
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, path := range []string{one, alias} {
			o := CJSISourceObservation{Source: ingest.Source{Client: "claude", Path: path, Priority: 1, Identity: id, Size: 4, ModifiedAt: st.ModTime().UnixNano(), ChangedAt: ctime, Stable: stable}, Kind: "claude-jsonl"}
			got, e := CJSISourceUnchanged(ctx, s, o)
			if got != (path == one) || e != nil {
				t.Fatalf("%s borrowed tail hint: %v %v", path, got, e)
			}
			called := false
			_, e = readCompatibleActivity(ctx, s, Metadata{SourcePath: path, Client: "claude", SessionID: "s"}, func(*os.File, int64) (bool, error) { called = true; return true, nil })
			if called != (path == one) || path == one && e != nil || path != one && !errors.Is(e, sql.ErrNoRows) {
				t.Fatalf("%s borrowed activity prefix: %v %v", path, called, e)
			}

		}
		return nil
	})
}

func TestCJSIFileCheckSkipsUnrelatedGroupMetadata(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "unrelated"} {
		if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte("{\"type\":\"user\",\"sessionId\":\""+id+"\",\"message\":{\"role\":\"user\",\"content\":\"ok\"}}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if _, err := Scan(ctx, db, home); err != nil {
			t.Fatal(err)
		}
	})
	src, err := ingest.Discover(home)
	if err != nil || len(src) != 2 {
		t.Fatal(src, err)
	}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		tx := stageTx(t, f)
		defer tx.Rollback()
		s.Tx = tx
		// Fail deterministically if the per-file query performs even a
		// metadata aggregation on the other source.
		if _, e := tx.Exec(`CREATE TEMP VIEW cjsi_base_groups AS SELECT source_path,client,CASE WHEN session_id='unrelated' THEN abs(-9223372036854775808) ELSE source_kind END AS source_kind FROM main.cjsi_base_groups`); e != nil {
			return e
		}
		for _, p := range src {
			if filepath.Base(p.Path) == "one.jsonl" {
				ok, e := CJSISourceUnchanged(ctx, s, CJSISourceObservation{Source: p, Kind: "claude-jsonl"})
				if !ok || e != nil {
					t.Fatalf("single source touched unrelated metadata: %v %v", ok, e)
				}
			}
		}
		return nil
	})
}

func TestCJSIRetainedRecordCannotBorrowAnotherGroupsEvidence(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	r := stageFixturePlan(t, f, f.path, true).Replacements[0]
	for _, relation := range []string{"group", "range", "source"} {
		for _, component := range []string{"path", "client", "session"} {
			if relation == "source" && component != "path" {
				continue
			}
			t.Run(relation+"_"+component, func(t *testing.T) {
				tx := stageTx(t, f)
				defer tx.Rollback()
				run := func(q string, args ...any) {
					t.Helper()
					if _, e := tx.ExecContext(ctx, q, args...); e != nil {
						t.Fatal(e)
					}
				}
				path, client, id := f.path, "codex", adapterThread
				switch component {
				case "path":
					path = "replica"
					run("UPDATE session_sources SET identity=? WHERE source_path='replica'", f.identity)
				case "session":
					id = "old-group"
				case "client":
					client = "claude"
					run("INSERT INTO session_metadata SELECT source_path,'claude',session_id,project,model,parser_version,first_at,last_at FROM session_metadata WHERE source_path=? AND client='codex' AND session_id=?", f.path, adapterThread)
					run("INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id) VALUES(?,'claude',?,randomblob(32))", f.path, adapterThread)
				}
				switch relation {
				case "group":
					run("UPDATE cjsi_base_groups SET order_verified=0 WHERE source_path=? AND client='codex' AND session_id=?", f.path, adapterThread)
					run("UPDATE cjsi_base_groups SET order_verified=1,source_kind='codex-jsonl' WHERE source_path=? AND client=? AND session_id=?", path, client, id)
				case "range":
					run(`INSERT INTO cjsi_base_ranges SELECT ?,?,?,workset_round,workset_attempt,identity,selection_epoch,generation,replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest FROM cjsi_base_ranges WHERE source_path=? AND client='codex' AND session_id=?`, path, client, id, f.path, adapterThread)
					run("UPDATE cjsi_base_ranges SET generation=generation+1 WHERE source_path=? AND client='codex' AND session_id=?", f.path, adapterThread)
				case "source":
					run("UPDATE session_sources SET identity='new-inode' WHERE source_path=?", f.path)
				}
				got, err := r.baseDocument(ctx, tx, f.path, r.Material.Documents[0])
				if err != nil || got != 0 {
					t.Fatalf("other %s supplied an old id: %d %v", component, got, err)
				}
			})
		}
	}
}

func TestCJSIProjectionCountsStaleVisibleDocuments(t *testing.T) {
	ctx := t.Context()
	projections := []TelemetrySession{{Client: "codex", SessionID: "settled", FirstAt: "first", LastAt: "last"}}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if e := ProjectTelemetrySessions(ctx, db, projections); e != nil {
			t.Fatal(e)
		}
		adapterExec(t, db, "INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('otel:codex/settled','codex','settled','','user_prompt','obsolete')")
	})
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		c, err := PlanCJSIIndex(ctx, s, 1, 1, s.Selection().SessionRevision+1, nil, nil, &projections)
		if err != nil {
			return err
		}
		if c == nil {
			t.Fatal("stale projection not replaced")
		}
		changed, removed := c.DocumentChanges()
		if changed != 0 || removed != 1 {
			t.Fatalf("empty projection changed=%d removed=%d want 0/1", changed, removed)
		}
		return nil
	})
}
