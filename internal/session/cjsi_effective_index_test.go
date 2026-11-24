package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIEffectiveIndex(t *testing.T) {
	t.Run("single ticket reader", testEffectiveReader)
	t.Run("reopened pages", testReopenedPages)
	t.Run("native material", testNativeMaterial)
	t.Run("explicit thread and missing birth", testExplicitThreadAndMissingBirth)
}

type ordinalFixtureRow struct {
	label       string
	id, ordinal int
}

func ordinalFixtureStages() (old, inserted, appended []ordinalFixtureRow) {
	old = []ordinalFixtureRow{{"C", 3, 3}, {"A", 1, 1}, {"B", 2, 2}}
	inserted = []ordinalFixtureRow{{"C", 3, 4}, {"X", 4, 2}, {"A", 1, 1}, {"B", 2, 3}}
	appended = append(slices.Clone(inserted), ordinalFixtureRow{"D", 5, 5})
	return
}

type ordinalFixturePage struct {
	labels       string
	total, shown int
	hasMore      bool
}

func observedOrdinalFixturePage(labels string, p Pagination) ordinalFixturePage {
	return ordinalFixturePage{labels, p.Total, p.Shown, p.HasMore}
}

var insertedOrdinalFixturePages = [...]ordinalFixturePage{
	{"AX", 4, 2, true},
	{"BC", 4, 2, false},
	{"", 4, 0, false},
}

// These relations describe the synthetic adapter only. Task 2 owns physical
// names and schema. Its selected memberships must distinguish the FTS sides:
// a base row can have the same rowid as an ineligible candidate.
type fixtureStamp struct {
	ticket, protection int64
	seal               string
}

func effectiveReaderFixture(t *testing.T) (func() indexReadView, func(string, ...any), func(func()) withIndexSnapshot) {
	t.Helper()
	ctx := t.Context()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("PRAGMA journal_mode=WAL")
	exec("CREATE TABLE ticket(n INTEGER, sequence_id INTEGER, protection INTEGER, seal TEXT)")
	exec("INSERT INTO ticket VALUES(1,1,1,'original')")
	exec("CREATE TABLE members(ticket,side,doc_id,ordinal)")
	exec("CREATE TABLE copies_fixture(ticket,source_path,priority,hidden,identity)")
	exec("CREATE VIRTUAL TABLE base USING fts5(source_path UNINDEXED,text,kind)")
	exec("CREATE VIRTUAL TABLE delta USING fts5(source_path UNINDEXED,text,kind)")
	for i, s := range []string{"A", "B", "C", "superseded"} {
		exec("INSERT INTO base(rowid,source_path,text,kind) VALUES(?,'selected',?,'needle')", i+1, s)
	}
	exec("INSERT INTO delta(rowid,source_path,text,kind) VALUES(1,'selected','collision','needle'),(4,'selected','X','needle'),(5,'selected','D','needle'),(6,'hidden','excluded','needle'),(7,'stale','stale','needle'),(8,'replica','lower','needle')")
	old, inserted, appended := ordinalFixtureStages()
	for i, rows := range [][]ordinalFixtureRow{old, inserted, appended} {
		n := i + 1
		for _, r := range rows {
			side := "base"
			if r.label == "D" || r.label == "X" {
				side = "delta"
			}
			exec("INSERT INTO members VALUES(?,?,?,?)", n, side, r.id, r.ordinal)
		}
		exec("INSERT INTO copies_fixture VALUES(?,'selected',20,0,'selected'),(?,'hidden',50,1,'hidden'),(?,'replica',10,0,'replica'),(99,'stale',99,0,'stale')", n, n, n)
		exec("INSERT INTO members VALUES(?,'delta',6,1),(99,'delta',7,1),(?,'delta',8,1)", n, n)
	}
	stamp := func(q interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}) fixtureStamp {
		t.Helper()
		var s fixtureStamp
		if err := q.QueryRowContext(ctx, "SELECT n,protection,seal FROM ticket").Scan(&s.ticket, &s.protection, &s.seal); err != nil {
			t.Fatal(err)
		}
		return s
	}
	read := func() indexReadView {
		t.Helper()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { tx.Rollback() })
		// An approved adapter will supply the ticket through its own protected
		// selection. Here SQLite pins the first SELECT and all later reads.
		part := func(side string, search bool) string {
			q := "SELECT f.source_path,'codex' client,'s' session_id,'2026-01-01T00:00:00Z' event_at,f.kind,f.text,m.doc_id,m.ordinal detail_ordinal FROM " + side + " f JOIN members m ON m.side='" + side + "' AND m.doc_id=f.rowid JOIN ticket ON ticket.sequence_id=m.ticket"
			if search {
				q += " WHERE " + side + " MATCH @query"
			}
			return q
		}
		view := indexReadView{tx: tx, copies: `SELECT source_path,'codex' client,'s' session_id,'' project,'' model,'' first_at,'' last_at,priority,sequence_id,identity,'p-'||source_path prefix_hash,7 parser_version,CASE source_path WHEN 'selected' THEN '' ELSE '2026-01-01T00:00:00Z' END head_at,CASE source_path WHEN 'selected' THEN '2026-01-02T00:00:00Z' ELSE '2026-01-01T00:00:00Z' END birth FROM copies_fixture JOIN ticket ON n=ticket WHERE hidden=0`,
			documents: part("base", false) + " UNION ALL " + part("delta", false),
			matches:   part("base", true) + " UNION ALL " + part("delta", true)}
		selected := stamp(tx)
		view.detailIdentity = func(ctx context.Context, client, id string) (store.CJSIDetailIdentity, error) {
			var path, sequence string
			err := tx.QueryRowContext(ctx, "SELECT source_path,sequence_id FROM ("+view.copies+") WHERE client=? AND session_id=? ORDER BY priority DESC,source_path LIMIT 1", client, id).Scan(&path, &sequence)
			if err != nil {
				return store.CJSIDetailIdentity{}, err
			}
			return store.CJSIDetailIdentity{Stamp: store.CJSIIndexStamp{Ticket: selected.ticket, ProtectionRevision: selected.protection, SessionSeal: sha256.Sum256([]byte(selected.seal))}, SourcePath: path, Client: client, SessionID: id, Sequence: sha256.Sum256([]byte(sequence))}, nil
		}
		view.activity = func(ctx context.Context, m Metadata) ([]activity.Detail, error) {
			var ticket int
			if err := tx.QueryRowContext(ctx, "SELECT n FROM ticket").Scan(&ticket); err != nil {
				return nil, err
			}
			if m.SourcePath != "selected" || ticket != 2 {
				return nil, errors.New("unexpected activity selection")
			}
			return []activity.Detail{{Tool: "same-ticket"}}, nil
		}
		return view
	}
	with := func(afterRead func()) withIndexSnapshot {
		return func(callback func(indexReadView) error) error {
			v := read()
			defer v.tx.Rollback()
			old := stamp(v.tx)
			if err := callback(v); err != nil {
				return err
			}
			if afterRead != nil {
				afterRead()
			}
			if err := v.tx.Commit(); err != nil {
				return err
			}
			if stamp(db) != old {
				return sql.ErrNoRows
			}
			return nil
		}
	}
	return read, exec, with
}

func testReopenedPages(t *testing.T) {
	for _, tc := range []struct{ name, change, fresh string }{
		{"same path new ticket and sequence", "UPDATE ticket SET n=2,sequence_id=2", "AX"},
		{"ticket alone", "UPDATE ticket SET n=2", "AB"},
		{"sequence alone", "UPDATE ticket SET sequence_id=2", "AX"},
		{"protection alone", "UPDATE ticket SET protection=2", "AB"},
		{"seal alone", "UPDATE ticket SET seal='replacement'", "AB"},
		{"source loses protection", "UPDATE copies_fixture SET hidden=1 WHERE source_path='selected'", "lower"},
		{"revoke after page read", "", "AB"},
		{"unchanged", "", "AB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			_, exec, open := effectiveReaderFixture(t)
			selectMeta := func(v indexReadView) (CJSISelectedDetail, error) { return v.metadata(ctx, "codex", "s") }
			expected, err := snapshotResult(open(nil), selectMeta)
			if err != nil {
				t.Fatal(err)
			}
			a, _, err := snapshotPage(ctx, open(nil), expected, 1, 2, false)
			if err != nil || docLabels(a) != "AB" {
				t.Fatalf("first page = %q, %v", docLabels(a), err)
			}
			if tc.change != "" {
				exec(tc.change)
			}
			var after func()
			revoked := false
			if tc.name == "revoke after page read" {
				after = func() { exec("UPDATE ticket SET protection=protection+1"); revoked = true }
			}
			b, p, err := snapshotPage(ctx, open(after), expected, 2, 2, false)
			if after != nil && !revoked {
				t.Fatal("callback did not complete before protection changed")
			}
			if tc.name == "unchanged" {
				if err != nil || docLabels(b) != "C" || p.Total != 3 {
					t.Fatalf("unchanged continuation = %q, %+v, %v", docLabels(b), p, err)
				}
			} else if !errors.Is(err, sql.ErrNoRows) || len(b) != 0 || p != (Pagination{}) {
				t.Errorf("stale continuation = %q + %q, %+v, %v", docLabels(a), docLabels(b), p, err)
			}
			fresh, err := snapshotResult(open(nil), selectMeta)
			if err != nil {
				t.Fatal(err)
			}
			docs, _, err := snapshotPage(ctx, open(nil), fresh, 1, 2, false)
			if err != nil || docLabels(docs) != tc.fresh {
				t.Fatalf("fresh selection page 1 = %q, %v; want %q", docLabels(docs), err, tc.fresh)
			}
		})
	}
}

func testEffectiveReader(t *testing.T) {
	ctx := t.Context()
	read, exec, _ := effectiveReaderFixture(t)
	check := func(v indexReadView, detail, matched string, ids []int64) {
		t.Helper()
		show, err := v.ShowWithActivity(ctx, "codex", "s", false)
		if err != nil || docLabels(show.Documents) != detail {
			t.Fatalf("detail = %q, %v; want %s", docLabels(show.Documents), err, detail)
		}
		hits, err := v.Search(ctx, "needle")
		if err != nil || docLabels(hits) != matched {
			t.Fatalf("MATCH = %s, %v; want %s", docLabels(hits), err, matched)
		}
		var gotIDs []int64
		err = v.visitHits(ctx, "needle", func(_ Document, id int64) { gotIDs = append(gotIDs, id) })
		if err != nil || !slices.Equal(gotIDs, ids) {
			t.Fatalf("MATCH IDs = %v, %v; want %v", gotIDs, err, ids)
		}
	}
	first := read()
	check(first, "ABC", "ABC", []int64{1, 2, 3})
	exec("UPDATE ticket SET n=2,sequence_id=2")
	v := read()
	check(v, "AXBC", "ABCX", []int64{1, 2, 3, 4})
	check(first, "ABC", "ABC", []int64{1, 2, 3})
	// Advance the live ticket while the reader is between page operations.
	exec("UPDATE ticket SET n=3,sequence_id=3")
	m, err := v.metadata(ctx, "codex", "s")
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range insertedOrdinalFixturePages {
		docs, page, err := v.DocumentsPage(ctx, m, i+1, 2, false)
		got := observedOrdinalFixturePage(docLabels(docs), page)
		if err != nil || got != want || page.Page != i+1 || page.Limit != 2 || page.NextPage != []int{2, 0, 0}[i] {
			t.Fatalf("page %d = %+v %+v, %v; want %+v", i+1, got, page, err, want)
		}
	}
	docs, page, err := v.DocumentsPage(ctx, m, 2, 2, true)
	if err != nil || docLabels(docs) != "AXBC" || page.Total != 4 || page.Shown != 4 || page.HasMore {
		t.Fatalf("all = %v %+v %v", docs, page, err)
	}
	result, err := v.ShowWithActivity(ctx, "codex", "s", true)
	if err != nil || !reflect.DeepEqual(result.Documents, docs) || len(result.Activity) != 1 || result.Activity[0].Tool != "same-ticket" {
		t.Fatalf("activity detail = %+v %v", result, err)
	}
	for _, q := range []string{"collision", "superseded", "excluded", "stale", "lower", "D"} {
		rows, err := v.Search(ctx, q)
		if err != nil || len(rows) != 0 {
			t.Fatalf("masked MATCH %q = %v, %v", q, rows, err)
		}
	}
	m.SourcePath = "hidden"
	if got, _, err := v.DocumentsPage(ctx, m, 1, 2, false); !errors.Is(err, sql.ErrNoRows) || len(got) != 0 {
		t.Fatalf("unselected metadata exposed %v %v", got, err)
	}
	copies, err := v.EffectiveCopies(ctx, "codex", "s")
	if err != nil || len(copies) != 2 || !copies[0].Selected || copies[1].Selected || copies[0].Source.Identity != "selected" || copies[1].Source.Identity != "replica" || copies[0].HeadAt != "" || copies[0].Birth != "2026-01-02T00:00:00Z" || copies[1].HeadAt != "2026-01-01T00:00:00Z" || copies[1].Source.PrefixHash != "p-replica" || copies[0].Source.ParserVersion != 7 {
		t.Fatalf("Task 6 copy inputs = %+v %v", copies, err)
	}
	check(read(), "AXBCD", "ABCXD", []int64{1, 2, 3, 4, 5})
}

func docLabels(docs []Document) string {
	var s strings.Builder
	for _, d := range docs {
		s.WriteString(d.Text)
	}
	return s.String()
}

type fixtureWorkset struct {
	records []ingest.CodexRecord
	facts   []ingest.CodexCheckpointEvidence
	sealed  bool
}

func (w *fixtureWorkset) StageCodexRecord(_ context.Context, r ingest.CodexRecord, e ingest.CodexCheckpointEvidence) error {
	w.records = append(w.records, r)
	w.facts = append(w.facts, e)
	return nil
}
func (w *fixtureWorkset) Abort() { w.records = nil; w.facts = nil; w.sealed = false }
func (w *fixtureWorkset) OpenCodexWorkset(ctx context.Context) (ingest.CodexWorksetNext, error) {
	if !w.sealed {
		return nil, errors.New("unsealed fixture")
	}
	i := 0
	return func(ctx context.Context) (ingest.CodexRecord, ingest.CodexCheckpointEvidence, error) {
		if i == len(w.records) {
			return ingest.CodexRecord{}, ingest.CodexCheckpointEvidence{}, io.EOF
		}
		r, e := w.records[i], w.facts[i]
		i++
		return r, e, ctx.Err()
	}, nil
}

type observedCapture struct {
	capturedMessages
	lookups int
}

func (v *observedCapture) lookup(span ingest.CodexSpan) (ingest.CodexVisible, bool) {
	v.lookups++
	return v.capturedMessages.lookup(span)
}

func testNativeMaterial(t *testing.T) {
	raw, err := os.ReadFile("../ingest/testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	const thread = "00000000-0000-4000-8000-000000000002"
	input := string(raw) + `{"type":"turn_context","payload":{"turn_id":"unresolved-turn"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"unresolved-synthetic"}]}}
{"type":"partial`
	const cap = 4096
	for _, scenario := range []string{"accept local only", "transient equal length", "partial tail digest", "different capture end", "different file identity", "different line policy", "stale selection", "different replay version", "wrong span", "wrong kind"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := t.Context()
			path := filepath.Join(t.TempDir(), "rollout-2026-01-02T03-04-05-"+thread+".jsonl")
			file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.WriteString(input); err != nil {
				t.Fatal(err)
			}
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			identity, _, stable := ingest.FileGeneration(info)
			if !stable {
				t.Fatal("synthetic file lacks a stable identity")
			}
			version := ingest.CodexCheckpointVersion{Generation: 3, Parser: 1, MaxLineBytes: cap}
			// Authoritative fixture state exists before either capture. It knows
			// the selected original bytes, full extent, incomplete tail and policy.
			authority := ingest.CodexSelectedSource{Path: path, ThreadID: thread, Epoch: 17, Evidence: ingest.CodexPrefixEvidence{
				Identity: identity, Progress: ingest.CodexReadProgress{ReadEnd: int64(len(input)), CompleteEnd: int64(strings.LastIndexByte(input, '\n') + 1), Partial: true},
				Digest: sha256.Sum256([]byte(input)), MaxLineBytes: cap, Version: version}}
			if scenario == "different replay version" {
				authority.Evidence.Version.Parser++
			}
			selected := authority
			transient := input
			switch scenario {
			case "transient equal length":
				transient = strings.Replace(input, "local-user-synthetic", "other-user-synthetic", 1)
			case "partial tail digest":
				transient = strings.TrimSuffix(input, "partial") + "PARTIAL"
			}
			if len(transient) != len(input) {
				t.Fatal("fixture length changed")
			}
			if _, err := file.WriteAt([]byte(transient), 0); err != nil {
				t.Fatal(err)
			}
			end, lineCap := int64(len(input)), cap
			if scenario == "different capture end" {
				end--
			}
			if scenario == "different line policy" {
				lineCap++
			}
			captureFile := file
			if scenario == "different file identity" {
				other := filepath.Join(t.TempDir(), filepath.Base(path))
				if err := os.WriteFile(other, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				captureFile, err = os.Open(other)
				if err != nil {
					t.Fatal(err)
				}
				defer captureFile.Close()
			}
			material, err := captureSessionMaterial(ctx, captureFile, captureFile.Name(), end, lineCap)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			capture := &observedCapture{capturedMessages: material.visible.(capturedMessages)}
			material.visible = capture
			for key, v := range capture.capturedMessages {
				seen[v.Text] = true
				if v.Text == "local-user-synthetic" {
					switch scenario {
					case "wrong span":
						v.Span.Start++
						capture.capturedMessages[key] = v
					case "wrong kind":
						v.Kind = ingest.CodexAssistantFinal
						capture.capturedMessages[key] = v
					}
				}
			}
			if !seen["inherited-synthetic"] || !seen["unresolved-synthetic"] || !seen["local-final-synthetic"] {
				t.Fatal("fixture did not capture both local and unowned visible material")
			}
			if transient != input {
				if _, err := file.WriteAt([]byte(input), 0); err != nil {
					t.Fatal(err)
				}
			}
			work := new(fixtureWorkset)
			replay, err := ingest.BeginCodexCheckpointWorkset(ctx, file, path, int64(len(input)), version, nil, work)
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Abort()
			step, err := replay.Step(ctx, int64(len(input)))
			if err != nil || !step.Done || !step.Progress.Partial {
				t.Fatalf("seal: %+v %v", step, err)
			}
			work.sealed = true
			if scenario == "stale selection" {
				authority.Epoch++
			}
			calls := 0
			got, err := acceptSessionMaterial(ctx, replay, ingest.CodexWorksetLimits{SegmentRecords: 64, Turns: 16}, material, selected, func(context.Context) (ingest.CodexSelectedSource, error) {
				calls++
				if capture.lookups != 0 {
					t.Errorf("selection reached after %d body lookups", capture.lookups)
				}
				return authority, nil
			})
			switch scenario {
			case "accept local only":
				if capture.lookups != 2 {
					t.Errorf("accepted capture: %d body lookups, want 2", capture.lookups)
				}
			case "wrong span", "wrong kind":
				if calls != 1 || capture.lookups == 0 {
					t.Errorf("material mismatch: selection calls=%d body lookups=%d", calls, capture.lookups)
				}
			default:
				wantCalls := 0
				if scenario == "stale selection" {
					wantCalls = 1
				}
				if calls != wantCalls || capture.lookups != 0 {
					t.Errorf("rejected proof: selection calls=%d (want %d), body lookups=%d (want 0)", calls, wantCalls, capture.lookups)
				}
			}
			if len(capture.capturedMessages) != 0 {
				t.Fatal("transient material retained")
			}
			if scenario != "accept local only" {
				if !errors.Is(err, ingest.ErrSourceChanged) || !reflect.DeepEqual(got, nativeSessionUpdate{}) {
					t.Fatalf("rejected input produced %+v, %v", got, err)
				}
				return
			}
			if calls != 1 || err != nil || got.Source.ThreadID != thread || got.Evidence != selected.Evidence || !got.Checkpoint.Partial || got.ParserVersion != 7 || ParserVersion != 6 || len(got.Documents) != 2 {
				t.Fatalf("handoff %+v, %v", got, err)
			}
			for i, want := range []string{"local-user-synthetic", "local-final-synthetic"} {
				d := got.Documents[i]
				if d.Text != want || d.Kind != []string{"user_prompt", "assistant_final"}[i] || d.SessionID != thread || d.Ordinal != i+1 || !strings.Contains(input[d.Span.Start:d.Span.End], want) {
					t.Fatalf("document %d = %+v", i, d)
				}
			}
			encoded, err := json.Marshal([]any{work.records, work.facts, got.Evidence, got.Source, got.Checkpoint})
			if err != nil {
				t.Fatal(err)
			}
			for body := range seen {
				if strings.Contains(string(encoded), body) {
					t.Fatal("body entered workset proof")
				}
			}
		})
	}
}

func fixtureNativeInputs(t *testing.T, selectedPath, thread, input string) (*os.File, *ingest.CodexCheckpointReplay, CJSISourceInput) {
	t.Helper()
	ctx := t.Context()
	if err := os.WriteFile(selectedPath, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	identity, changedAt, _ := ingest.FileGeneration(info)
	size := int64(len(input))
	complete := int64(strings.LastIndexByte(input, '\n') + 1)
	version := ingest.CodexCheckpointVersion{Generation: 3, Parser: 1, MaxLineBytes: 4096}
	selected := ingest.CodexSelectedSource{Path: selectedPath, ThreadID: thread, Epoch: 11, Evidence: ingest.CodexPrefixEvidence{
		Identity: identity, Progress: ingest.CodexReadProgress{ReadEnd: size, CompleteEnd: complete, Partial: complete < size},
		Digest: sha256.Sum256([]byte(input)), Version: version, MaxLineBytes: version.MaxLineBytes}}
	work := new(fixtureWorkset)
	replay, err := ingest.BeginCodexCheckpointWorkset(ctx, file, selectedPath, size, version, nil, work)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(replay.Abort)
	step, err := replay.Step(ctx, size)
	if err != nil || !step.Done {
		t.Fatalf("replay: %+v %v", step, err)
	}
	work.sealed = true
	return file, replay, CJSISourceInput{
		Selected: selected, Size: size, ModifiedAt: info.ModTime().UnixNano(), ChangedAt: changedAt, Priority: 3, Round: 4, WorksetAttempt: 3, PrefixHash: "fixture", ScannedAt: "t",
	}
}

func fixturePreparedNative(t *testing.T, selectedPath, thread, input string) CJSIPreparedSource {
	t.Helper()
	file, replay, in := fixtureNativeInputs(t, selectedPath, thread, input)
	prepared, err := PrepareCJSISource(t.Context(), file, replay, ingest.CodexWorksetLimits{SegmentRecords: 64, Turns: 16}, in.Size, in, func(context.Context) (ingest.CodexSelectedSource, error) { return in.Selected, nil })
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func testExplicitThreadAndMissingBirth(t *testing.T) {
	const thread = "00000000-0000-4000-8000-000000000002"
	const parent = "00000000-0000-4000-8000-000000000001"
	// The picked message has independent execution proof but neither it nor
	// the foreign message has a local turn or a qualified birth timestamp.
	input := `{"type":"session_meta","payload":{"id":"` + thread + `","parent_thread_id":"` + parent + `"}}
{"timestamp":"2026-01-02T09:00:00Z","type":"response_item","payload":{"thread_id":"` + parent + `","type":"message","role":"user","content":[{"type":"input_text","text":"foreign"}]}}
{"timestamp":"2026-01-02T09:00:01Z","type":"response_item","payload":{"thread_id":"` + thread + `","type":"message","role":"user","content":[{"type":"input_text","text":"explicit"}]}}
`
	r := fixturePreparedNative(t, filepath.Join(t.TempDir(), "rollout-2026-01-02T03-04-05-"+thread+".jsonl"), thread, input).replacement
	if !r.Material.Source.Birth.IsZero() || r.Metadata.FirstAt != "" || r.Metadata.LastAt != "2026-01-02T09:00:01Z" {
		t.Fatalf("birth=%v metadata=%+v", r.Material.Source.Birth, r.Metadata)
	}
	if len(r.Material.Documents) != 1 || r.Material.Documents[0].Text != "explicit" || r.Material.Documents[0].Ordinal != 1 {
		t.Fatalf("local messages %+v", r.Material.Documents)
	}
}

func TestCJSIPrepareSourceAdmission(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000002"
	body := `{"type":"session_meta","payload":{"id":"` + id + `","timestamp":"2026-01-01T12:00:00Z","cwd":"/project"}}
`
	type req struct {
		file    *os.File
		replay  *ingest.CodexCheckpointReplay
		input   CJSISourceInput
		budget  int64
		current ingest.CodexCurrentSelection
	}
	currentErr := errors.New("selection callback failed")
	for _, tc := range []struct {
		name   string
		change func(*req)
		want   error
	}{
		{"valid", func(*req) {}, nil},
		{"missing file", func(r *req) { r.file = nil }, ingest.ErrSourceChanged},
		{"missing replay", func(r *req) { r.replay = nil }, ingest.ErrSourceChanged},
		{"missing selection callback", func(r *req) { r.current = nil }, ingest.ErrSourceChanged},
		{"empty capture", func(r *req) { r.input.Selected.Evidence.Progress.ReadEnd = 0 }, ingest.ErrSourceChanged},
		{"captured beyond inventory", func(r *req) { r.input.Size-- }, ingest.ErrSourceChanged},
		{"insufficient budget", func(r *req) { r.budget-- }, ingest.ErrCodexWorksetBudget},
		{"closed file", func(r *req) { r.file.Close() }, os.ErrClosed},
		{"different file identity", func(r *req) { r.input.Selected.Evidence.Identity = "another-file" }, ingest.ErrSourceChanged},
		{"file shorter than capture", func(r *req) { r.input.Selected.Evidence.Progress.ReadEnd++; r.budget++; r.input.Size++ }, ingest.ErrSourceChanged},
		{"failed selection", func(r *req) {
			r.current = func(context.Context) (ingest.CodexSelectedSource, error) {
				return ingest.CodexSelectedSource{}, currentErr
			}
		}, currentErr},
		{"file changed after selection", func(r *req) {
			r.current = func(context.Context) (ingest.CodexSelectedSource, error) {
				file, err := os.OpenFile(r.file.Name(), os.O_WRONLY|os.O_APPEND, 0)
				if err != nil {
					return ingest.CodexSelectedSource{}, err
				}
				_, err = file.WriteString("{}\n")
				return r.input.Selected, errors.Join(err, file.Close())
			}
		}, ingest.ErrSourceChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, replay, in := fixtureNativeInputs(t, filepath.Join(t.TempDir(), "input.jsonl"), id, body)
			r := req{file, replay, in, in.Size, func(context.Context) (ingest.CodexSelectedSource, error) { return in.Selected, nil }}
			tc.change(&r)
			got, err := PrepareCJSISource(t.Context(), r.file, r.replay, ingest.CodexWorksetLimits{SegmentRecords: 64, Turns: 16}, r.budget, r.input, r.current)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				if !reflect.DeepEqual(got, CJSIPreparedSource{}) {
					t.Fatal("failed preparation returned material")
				}
				return
			}
			m := got.replacement.Metadata
			if m.Project != "/project" || m.FirstAt != "2026-01-01T12:00:00Z" || m.LastAt != m.FirstAt {
				t.Fatalf("own head metadata %+v", m)
			}
		})
	}
}

func TestCJSIRejectsUnboundOrMismatchedDetail(t *testing.T) {
	ctx := t.Context()
	open, _, _ := effectiveReaderFixture(t)
	v := open()
	if v.tx == nil {
		t.Fatal("missing fixture transaction")
	}
	defer v.tx.Rollback()
	m, err := v.metadata(ctx, "codex", "s")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*store.CJSIDetailIdentity)
	}{
		{"missing stamp", func(d *store.CJSIDetailIdentity) { d.Stamp = store.CJSIIndexStamp{} }},
		{"missing sequence", func(d *store.CJSIDetailIdentity) { d.Sequence = [32]byte{} }},
		{"other path", func(d *store.CJSIDetailIdentity) { d.SourcePath += "-other" }},
		{"other client", func(d *store.CJSIDetailIdentity) { d.Client = "claude" }},
		{"other session", func(d *store.CJSIDetailIdentity) { d.SessionID += "-other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := m.identity
			tc.change(&d)
			view := v
			view.detailIdentity = func(context.Context, string, string) (store.CJSIDetailIdentity, error) { return d, nil }
			got, err := view.metadata(ctx, m.Client, m.SessionID)
			if got != (CJSISelectedDetail{}) || !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("detail=%+v err=%v", got, err)
			}
		})
	}
	v.detailIdentity = nil
	if got, err := v.metadata(ctx, m.Client, m.SessionID); got != (CJSISelectedDetail{}) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unbound detail=%+v err=%v", got, err)
	}
	if got, err := (CJSIReader{}).List(ctx); got != nil || !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("unadmitted reader=%+v err=%v", got, err)
	}
}

func TestCJSIMaterialJoinHonorsLocalSegment(t *testing.T) {
	const thread = "own"
	ctx := t.Context()
	span := ingest.CodexSpan{Start: 10, End: 20}
	rec := ingest.CodexResolvedRecord{Record: ingest.CodexRecord{Kind: ingest.CodexMessage, Span: span, VisibleKind: ingest.CodexUserPrompt}, Execution: ingest.CodexBoundTurn, ThreadID: thread, TurnID: "turn"}
	seg := ingest.CodexSegmentProof{Span: span, ThreadID: thread, TurnID: "turn"}
	for _, tc := range []struct {
		name  string
		alter func(*ingest.CodexResolvedRecord, *ingest.CodexSegmentProof)
		want  bool
	}{
		{"bound", func(*ingest.CodexResolvedRecord, *ingest.CodexSegmentProof) {}, true},
		{"unresolved", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) { r.Execution = ingest.CodexUnresolved }, false},
		{"foreign record", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) { r.ThreadID = "foreign" }, false},
		{"foreign segment", func(_ *ingest.CodexResolvedRecord, s *ingest.CodexSegmentProof) { s.ThreadID = "foreign" }, false},
		{"wrong turn", func(_ *ingest.CodexResolvedRecord, s *ingest.CodexSegmentProof) { s.TurnID = "other" }, false},
		{"left boundary", func(_ *ingest.CodexResolvedRecord, s *ingest.CodexSegmentProof) { s.Span.Start++ }, false},
		{"right boundary", func(_ *ingest.CodexResolvedRecord, s *ingest.CodexSegmentProof) { s.Span.End-- }, false},
		{"single without turn", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) {
			r.Execution = ingest.CodexSingleSession
			r.TurnID = ""
		}, true},
		{"unbound turn", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) { r.TurnID = "" }, false},
		{"different record type", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) {
			r.Record.Kind = ingest.CodexToolStart
		}, false},
		{"no visible material", func(r *ingest.CodexResolvedRecord, _ *ingest.CodexSegmentProof) {
			r.Record.VisibleKind = ingest.CodexNoDocument
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, s := rec, seg
			tc.alter(&r, &s)
			sink := sessionMaterialSink{thread: thread}
			if err := sink.StageCodexProof(ctx, r); err != nil {
				t.Fatal(err)
			}
			if err := sink.StageCodexSegment(ctx, s); err != nil {
				t.Fatal(err)
			}
			capture := capturedMessages{span: {Span: span, Kind: ingest.CodexUserPrompt, Text: "approved"}}
			docs, err := sink.documents(ctx, capture)
			if err != nil || len(docs) != 0 && !tc.want || tc.want && (len(docs) != 1 || docs[0].Text != "approved") {
				t.Fatalf("docs=%+v err=%v", docs, err)
			}
			if tc.want {
				stopped, cancel := context.WithCancel(ctx)
				cancel()
				if got, e := sink.documents(stopped, capture); got != nil || !errors.Is(e, context.Canceled) {
					t.Fatalf("cancelled docs=%v err=%v", got, e)
				}
				sink.Abort()
				if got, e := sink.documents(ctx, capture); len(got) != 0 || e != nil {
					t.Fatalf("aborted docs=%v err=%v", got, e)
				}
			}
		})
	}
}

func TestCJSIOwnHeadProjectBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "head.jsonl")
	body := `{"type":"session_meta","payload":{"cwd":"/owner"}}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	n := int64(len(body))
	sel := ingest.CodexSelectedSource{Evidence: ingest.CodexPrefixEvidence{Progress: ingest.CodexReadProgress{ReadEnd: n}, MaxLineBytes: int(n)}}
	for _, tc := range []struct {
		name string
		span ingest.CodexSpan
		max  int
	}{
		{"negative start", ingest.CodexSpan{Start: -1, End: n}, int(n) + 1},
		{"empty nonzero span", ingest.CodexSpan{Start: 1, End: 1}, int(n)},
		{"past selected end", ingest.CodexSpan{End: n + 1}, int(n) + 1},
		{"past line policy", ingest.CodexSpan{End: n}, int(n) - 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sel
			s.Evidence.MaxLineBytes = tc.max
			v, err := ownHeadProject(file, s, tc.span)
			if v != "" || !errors.Is(err, ingest.ErrSourceChanged) {
				t.Fatalf("project=%q err=%v", v, err)
			}
		})
	}
	if v, err := ownHeadProject(file, sel, ingest.CodexSpan{End: n}); err != nil || v != "/owner" {
		t.Fatalf("own project=%q err=%v", v, err)
	}
	if v, err := ownHeadProject(file, sel, ingest.CodexSpan{End: n - 1}); v != "" || err == nil {
		t.Fatalf("truncated header project=%q err=%v", v, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if v, err := ownHeadProject(file, sel, ingest.CodexSpan{End: n}); v != "" || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed descriptor project=%q err=%v", v, err)
	}
	if v, err := ownHeadProject(file, sel, ingest.CodexSpan{}); v != "" || err != nil {
		t.Fatalf("headless source attempted I/O: %q %v", v, err)
	}
}

// Move the file contents at each cancellation-check boundary after selection.
// This exercises the handoff between the file verifier, document join and
// metadata reader without sleeping or racing a goroutine against the disk.
type indexObservationContext struct {
	context.Context
	visit func()
}

func (c indexObservationContext) Err() error { c.visit(); return c.Context.Err() }

func TestCJSIPreparedSourceRechecksObservationAfterSelection(t *testing.T) {
	const thread = "00000000-0000-4000-8000-000000000002"
	input := `{"type":"session_meta","payload":{"id":"` + thread + `","cwd":"/owner"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"approved"}}
`
	run := func(t *testing.T, failAt int) (int, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "native.jsonl")
		file, replay, in := fixtureNativeInputs(t, path, thread, input)
		armed := false
		checks := 0
		ctx := indexObservationContext{t.Context(), func() {
			if !armed {
				return
			}
			checks++
			if checks == failAt {
				changed := strings.Replace(input, "approved", "replaced", 1)
				if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}}
		got, err := PrepareCJSISource(ctx, file, replay, ingest.CodexWorksetLimits{SegmentRecords: 64, Turns: 16}, in.Size, in, func(context.Context) (ingest.CodexSelectedSource, error) { armed = true; return in.Selected, nil })
		if err != nil && !reflect.DeepEqual(got, CJSIPreparedSource{}) {
			t.Fatal("stale capture exposed approved text")
		}
		return checks, err
	}
	count, err := run(t, 0)
	if err != nil || count == 0 {
		t.Fatalf("baseline checks=%d err=%v", count, err)
	}
	for i := 1; i <= count; i++ {
		got, err := run(t, i)
		if got < i || !errors.Is(err, ingest.ErrSourceChanged) {
			t.Fatalf("file change at check %d (visited %d): %v", i, got, err)
		}
	}
}

func TestCJSILegacyContinuationKeepsWinningCopy(t *testing.T) {
	ctx := t.Context()
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ReplaceDocuments(ctx, db.DB, "codex", "old", nil); err != nil {
		t.Fatal(err)
	}
	m, err := ShowMetadata(ctx, db.DB, "codex", "old")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := ShowWithActivity(ctx, db.DB, m.Client, m.SessionID, false)
	if err != nil || empty.Documents != nil {
		t.Fatalf("legacy empty result %+v err=%v", empty, err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
		VALUES('new','new',0,0,0,'',9,6,'t');INSERT INTO session_metadata VALUES('new','codex','old','','',6,'','')`)
	if err != nil {
		t.Fatal(err)
	}
	docs, page, err := DocumentsPage(ctx, db.DB, m, 1, 5, false)
	if docs != nil || page != (Pagination{}) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale legacy page=%+v docs=%+v err=%v", page, docs, err)
	}
}

func TestCJSIReadWindowAndParserContracts(t *testing.T) {
	ctx := t.Context()
	open, _, _ := effectiveReaderFixture(t)
	v := open()
	defer v.tx.Rollback()
	selected, err := v.metadata(ctx, "codex", "s")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ page, limit int }{{0, 1}, {1, 0}, {1, MaxPageLimit + 1}} {
		d, p, e := v.DocumentsPage(ctx, selected, tc.page, tc.limit, false)
		if e == nil || d != nil || p != (Pagination{}) {
			t.Fatalf("invalid window %+v returned %+v %+v %v", tc, d, p, e)
		}
	}
	if got, e := v.Search(ctx, " \t"); e == nil || e.Error() != "search query is required" || got != nil {
		t.Fatalf("empty search: %+v %v", got, e)
	}
	if docs, page, err := v.DocumentsPage(ctx, selected, int(^uint(0)>>1), MaxPageLimit, false); err != nil || len(docs) != 0 || page.Total < 1 || page.HasMore {
		t.Fatalf("overflowing offset: docs=%v page=%+v err=%v", docs, page, err)
	}
	v.activity = nil
	if _, err := v.ShowWithActivity(ctx, "codex", "s", true); err == nil {
		t.Fatal("required activity source missing")
	}
	for _, tc := range []struct {
		client, kind string
		want         int
	}{{"codex", "codex-jsonl", 7}, {"claude", "claude-jsonl", 6}, {"codex", "synthetic", 6}, {"claude", "synthetic", 6}, {"codex", "otel", 6}, {"claude", "otel", 6}, {"claude", "codex-jsonl", 0}, {"codex", "claude-jsonl", 0}, {"other", "otel", 0}, {"codex", "compressed", 0}} {
		if got := CJSISessionParserTarget(tc.client, tc.kind); got != tc.want {
			t.Fatalf("%s/%s parser=%d want %d", tc.client, tc.kind, got, tc.want)
		}
	}
}

func TestCJSIPreparationRejectsBeforeAccess(t *testing.T) {
	ctx := t.Context()
	const id = "00000000-0000-4000-8000-000000000002"
	input := `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/approved"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"approved"}}
`
	for _, name := range []string{"no replay", "no authority", "empty input", "wrong identity", "short file", "cancelled capture", "cannot reread head"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.jsonl")
			_, replay, in := fixtureNativeInputs(t, path, id, input)
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			current := ingest.CodexCurrentSelection(func(context.Context) (ingest.CodexSelectedSource, error) { return in.Selected, nil })
			requestCtx := ctx
			budget := in.Size
			want := ingest.ErrSourceChanged
			switch name {
			case "no replay":
				replay = nil
				file.Close()
			case "no authority":
				current = nil
				file.Close()
			case "empty input":
				in.Selected.Evidence.Progress.ReadEnd = 0
				file.Close()
			case "wrong identity", "short file", "cancelled capture":
				stopped, cancel := context.WithCancel(ctx)
				cancel()
				requestCtx = stopped
				if name == "wrong identity" {
					in.Selected.Evidence.Identity = "other"
				} else if name == "short file" {
					in.Size++
					budget++
					in.Selected.Evidence.Progress.ReadEnd++
				} else {
					want = context.Canceled
				}
			case "cannot reread head":
				want = os.ErrClosed
				current = func(context.Context) (ingest.CodexSelectedSource, error) { file.Close(); return in.Selected, nil }
			}
			p, err := PrepareCJSISource(requestCtx, file, replay, ingest.CodexWorksetLimits{SegmentRecords: 64, Turns: 16}, budget, in, current)
			if !reflect.DeepEqual(p, CJSIPreparedSource{}) || !errors.Is(err, want) {
				t.Fatalf("invalid source returned %+v %v, want %v", p, err, want)
			}
		})
	}
}
func TestCJSIIndexMaterialRequiresAdmission(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000002"
	file, replay, in := fixtureNativeInputs(t, filepath.Join(t.TempDir(), "source.jsonl"), id, `{"type":"session_meta","payload":{"id":"`+id+`"}}
`)
	ctx := t.Context()
	current := ingest.CodexCurrentSelection(func(context.Context) (ingest.CodexSelectedSource, error) { return in.Selected, nil })
	for _, name := range []string{"absent capture", "absent callback", "absent epoch", "absent replay", "invalid workset budget"} {
		t.Run(name, func(t *testing.T) {
			m, err := captureSessionMaterial(ctx, file, in.Selected.Path, in.Size, in.Selected.Evidence.MaxLineBytes)
			if err != nil {
				t.Fatal(err)
			}
			sel := in.Selected
			cb := current
			rep := replay
			switch name {
			case "absent capture":
				m = sessionMaterial{}
			case "absent callback":
				cb = nil
			case "absent epoch":
				sel.Epoch = 0
			case "absent replay":
				rep = nil
			}
			update, err := acceptSessionMaterial(ctx, rep, ingest.CodexWorksetLimits{}, m, sel, cb)
			if !reflect.DeepEqual(update, nativeSessionUpdate{}) || err == nil {
				t.Fatalf("material=%+v err=%v", update, err)
			}
			if name == "invalid workset budget" {
				if errors.Is(err, ingest.ErrSourceChanged) {
					t.Fatalf("lost reducer failure: %v", err)
				}
			} else if !errors.Is(err, ingest.ErrSourceChanged) {
				t.Fatalf("unadmitted reduction: %v", err)
			}
		})
	}
}

func TestCJSIDocumentJoinRequiresCaptureMembership(t *testing.T) {
	span := ingest.CodexSpan{Start: 1, End: 2}
	sink := sessionMaterialSink{thread: "own", segments: []sessionSegmentRefs{{proof: ingest.CodexSegmentProof{Span: span, ThreadID: "own", TurnID: "t"}, refs: []sessionMessageRef{{span: span, kind: ingest.CodexUserPrompt, turn: "t"}}}}}
	capture := capturedMessages{}
	d, err := sink.documents(t.Context(), capture)
	if d != nil || !errors.Is(err, ingest.ErrSourceChanged) {
		t.Fatalf("missing capture member: docs=%v err=%v", d, err)
	}
}

func TestCJSIInvalidPlanDigestReturnsNoSeal(t *testing.T) {
	sum, err := indexDigest(func() {})
	if err == nil || sum != ([32]byte{}) {
		t.Fatalf("unsupported digest input returned %x err=%v", sum, err)
	}
}

func TestCJSIClosedLegacyIndexErrors(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := requireLegacyIndex(ctx, db); err == nil {
		t.Fatal("closed database treated as legacy")
	}
	if tx, e := legacyReadTransaction(ctx, db); tx != nil || e == nil {
		t.Fatalf("closed database returned transaction=%v err=%v", tx, e)
	}
	if ok, e := PreparedSourcesUnchanged(ctx, db, t.TempDir(), nil); ok || e == nil {
		t.Fatalf("closed legacy inventory: %v %v", ok, e)
	}
	if got, e := ReadCJSIOrderSnapshot(ctx, store.CJSIIndexSnapshot{}); !reflect.DeepEqual(got, CJSIOrderSnapshot{}) || !errors.Is(e, store.ErrCJSIIndexChanged) {
		t.Fatalf("unadmitted order snapshot: %+v %v", got, e)
	}
}

func TestCJSIActivityDescriptorMustBeLiveAndSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	id, changed, stable := ingest.FileGeneration(st)
	source := ingest.Source{Path: path, Identity: id, Size: st.Size(), ModifiedAt: st.ModTime().UnixNano(), ChangedAt: changed, Stable: stable}
	if err := validateActivityDescriptor(file, source); err != nil {
		t.Fatal(err)
	}
	other := source
	other.Identity = "other"
	if err := validateActivityDescriptor(file, other); !errors.Is(err, ingest.ErrSourceChanged) {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateActivityDescriptor(file, source); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed descriptor: %v", err)
	}
}

func TestCJSIReaderErrorsRemainClassified(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE VIEW cjsi_index_state AS SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if err := requireLegacyIndex(ctx, db); err != nil {
		t.Fatalf("a view claimed private-index ownership: %v", err)
	}
	d, p, err := DocumentsPage(ctx, db, Metadata{Client: "codex", SessionID: "s", SourcePath: "source"}, 1, 5, false)
	if d != nil || p != (Pagination{}) || err == nil || errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("storage failure reclassified as a missing selection: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	m, err := captureSessionMaterial(canceled, nil, "/not-opened.jsonl", 1, 4096)
	if m.visible != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("failed capture returned material=%t err=%v", m.visible != nil, err)
	}
}

func TestCJSILegacyWholeShowDocumentShape(t *testing.T) {
	ctx := t.Context()
	s, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := Document{Client: "codex", SessionID: "one", Kind: "user_prompt", Text: "visible", EventAt: "2026-01-01T00:00:00Z"}
	if e := ReplaceDocuments(ctx, s.DB, d.Client, d.SessionID, []Document{d}); e != nil {
		t.Fatal(e)
	}
	m, err := ShowMetadata(ctx, s.DB, d.Client, d.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := DocumentsPage(ctx, s.DB, m, 1, 5, true)
	if err != nil || !slices.Equal(page, []Document{d}) {
		t.Fatalf("legacy page: %+v %v", page, err)
	}
	show, err := ShowWithActivity(ctx, s.DB, d.Client, d.SessionID, false)
	d.EventAt = ""
	if err != nil || !slices.Equal(show.Documents, []Document{d}) {
		t.Fatalf("legacy whole-show representation changed: %+v %v", show, err)
	}
}

func TestCJSIUnrecognizedMaterialKindCannotBecomeVisible(t *testing.T) {
	const unknown ingest.CodexVisibleKind = 99
	span := ingest.CodexSpan{Start: 1, End: 2}
	sink := sessionMaterialSink{thread: "owner", segments: []sessionSegmentRefs{{proof: ingest.CodexSegmentProof{Span: span, ThreadID: "owner", TurnID: "turn"}, refs: []sessionMessageRef{{span: span, turn: "turn", kind: unknown}}}}}
	docs, err := sink.documents(t.Context(), capturedMessages{span: {Span: span, Kind: unknown, Text: "synthetic unapproved material"}})
	if err != nil || len(docs) != 0 {
		t.Fatalf("unknown kind returned docs=%+v err=%v", docs, err)
	}
}
