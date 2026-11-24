//go:build cjsi_adapter_fixture

package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

const adapterThread = "00000000-0000-4000-8000-000000000001"

type physicalFixture struct {
	root, path, identity string
	prefix               []byte
	core, private        *sql.DB
}

func adapterExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func adapterDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newPhysicalFixture(t *testing.T, built, seed bool) physicalFixture {
	t.Helper()
	return newMigratedFixture(t, built, seed, nil)
}
func newMigratedFixture(t *testing.T, built, seed bool, beforeMigration func(*sql.DB)) physicalFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := physicalFixture{root: root}
	if built {
		index, err := store.OpenSessions(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		if seed {
			f.path = filepath.Join(t.TempDir(), "rollout-2026-01-02T03-04-05-"+adapterThread+".jsonl")
			f.prefix = []byte(`{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}` + "\n" + `{"timestamp":"2026-01-02T10:00:00Z","type":"response_item","payload":{"type":"function_call","name":"apply_patch","call_id":"tool"}}` + "\n")
			if err := os.WriteFile(f.path, append([]byte(string(f.prefix)), []byte(`{"timestamp":"2026-01-02T10:00:01Z","type":"response_item","payload":{"type":"function_call_output","call_id":"tool"}}`+"\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(f.path)
			if err != nil {
				t.Fatal(err)
			}
			var stable bool
			f.identity, _, stable = ingest.FileGeneration(st)
			if !stable {
				t.Fatal("unstable fixture")
			}
			for _, s := range []struct {
				p, i     string
				priority int
			}{{f.path, f.identity, 3}, {"replica", "second", 2}, {"excluded", "excluded", 9}} {
				adapterExec(t, index.DB, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,1,'hash',?,6,'t')`, s.p, s.i, s.priority)
				adapterExec(t, index.DB, `INSERT INTO session_metadata VALUES(?,'codex',?,'project','model',6,'first','last')`, s.p, adapterThread)
			}
			adapterExec(t, index.DB, `INSERT INTO session_exclusions VALUES('path','excluded')`)
			adapterExec(t, index.DB, `INSERT INTO session_metadata VALUES(?,'codex','masked','project','model',6,'',''),(?,'codex','old-group','project','model',6,'','')`, f.path, f.path)
			for _, row := range []struct{ p, id, text string }{{f.path, adapterThread, "A"}, {f.path, adapterThread, "B"}, {f.path, adapterThread, "C"}, {f.path, "masked", "maskword"}, {"replica", adapterThread, "loserword"}, {"excluded", adapterThread, "excludedword"}, {f.path, "old-group", "movedword"}} {
				adapterExec(t, index.DB, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,'codex',?,'','user_prompt',?)`, row.p, row.id, row.text)
			}
		}
		if beforeMigration != nil {
			beforeMigration(index.DB)
		}
		if err := index.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.InstallTask5Fixture(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	f.core = adapterDB(t, filepath.Join(root, "agentdeck.sqlite3"))
	if built {
		f.private = adapterDB(t, filepath.Join(root, "session-index-v1.sqlite3"))
		adapterExec(t, f.private, "PRAGMA journal_mode=WAL")
	}
	t.Cleanup(func() {
		if err := store.CloseReadPools(); err != nil {
			t.Error(err)
		}
	})
	return f
}

func (f physicalFixture) read(t *testing.T, run func(CJSIReader)) {
	t.Helper()
	tx, err := f.core.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	run(NewCJSIReader(f.root, tx, ingest.CodexSelectedLimits{MaxBytes: 1 << 20, MaxLineBytes: 4096, MaxRecords: 64}))
}

// A synthetic writer prepares a single sealed delta; no product writer is
// supplied by this reader test. The migrated base is produced by real Store.
func (f physicalFixture) advance(t *testing.T, h store.CJSIIndexStamp, published int) {
	t.Helper()
	db := f.private
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, last, err := store.ReserveCJSIDocumentIDs(t.Context(), tx, 2)
	if err != nil || first != 8 || last != 9 {
		tx.Rollback()
		t.Fatalf("IDs %d..%d, %v", first, last, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, db, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
		VALUES(1,1,?,?,0,0,?,?,0,0,0,?)`, h.CoreEpoch, h.IndexEpoch, h.BaseSeal[:], h.BaseRevision, h.BaseRevision+1)
	for _, s := range []struct{ path, id string }{{f.path, adapterThread}, {f.path, "masked"}, {"moved", "new-group"}} {
		adapterExec(t, db, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,order_verified,sequence_id,head_at,birth)
			VALUES(1,1,?,'codex',?,'replace','project','model',7,'first','last',1,randomblob(32),'2026-01-01T00:00:00Z','2026-01-01T00:00:01Z')`, s.path, s.id)
	}
	adapterExec(t, db, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,sequence_id) VALUES(1,1,?,'codex','old-group','delete',randomblob(32))`, f.path)
	for _, s := range []struct{ p, i string }{{f.path, f.identity}, {"moved", "move-id"}} {
		adapterExec(t, db, `INSERT INTO cjsi_candidate_sources VALUES(1,1,?,'upsert','codex-jsonl',?,3,7,?,?,1,1,'hash','t')`, s.p, s.i, len(f.prefix), len(f.prefix))
	}
	adapterExec(t, db, `INSERT INTO cjsi_candidate_payloads VALUES(8,'','user_prompt','X'),(9,'','user_prompt','unselectedword')`)
	for i, id := range []int{1, 8, 2, 3, 7} {
		path, group, ordinal, side := f.path, adapterThread, i+1, "base"
		if id == 8 {
			side = "candidate"
		}
		if id == 7 {
			path, group, ordinal = "moved", "new-group", 1
		}
		adapterExec(t, db, `INSERT INTO cjsi_candidate_order(ticket,attempt,source_path,client,session_id,doc_id,doc_side,detail_ordinal) VALUES(1,1,?,'codex',?,?,?,?)`, path, group, id, side, ordinal)
	}
	hash := sha256.Sum256(f.prefix)
	adapterExec(t, db, `INSERT INTO cjsi_candidate_ranges VALUES(1,1,?,'codex',?,7,2,?,3,4,1,4096,?,?,0,?,?)`, f.path, adapterThread, f.identity, len(f.prefix), len(f.prefix), published, hash[:])
	seal := sha256.Sum256([]byte("fixture session seal"))
	adapterExec(t, db, `UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=1 AND attempt=1`, seal[:])
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=base_revision+1,session_seal=?`, seal[:])
}

func TestCJSIPhysicalSnapshotAdapter(t *testing.T) {
	ctx := t.Context()
	t.Run("absent versus admitted empty", func(t *testing.T) {
		for _, built := range []bool{false, true} {
			f := newPhysicalFixture(t, built, false)
			f.read(t, func(r CJSIReader) {
				rows, e := r.List(ctx)
				if built && (e != nil || rows == nil || len(rows) != 0) || !built && (!errors.Is(e, store.ErrCJSIIndexAbsent) || rows != nil) {
					t.Fatalf("built=%v list=%v err=%v", built, rows, e)
				}
			})
		}
	})
	f := newPhysicalFixture(t, true, true)
	var old, selected CJSISelectedDetail
	f.read(t, func(r CJSIReader) {
		var err error
		old, err = r.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		page, p, err := r.DocumentsPage(ctx, old, 1, 2, false)
		if err != nil || docLabels(page) != "AB" || p.Total != 3 {
			t.Fatalf("base page %+v %+v %v", page, p, err)
		}
		if got, e := r.ShowWithActivity(ctx, "codex", adapterThread, true); !errors.Is(e, sql.ErrNoRows) || !reflect.DeepEqual(got, Result{}) {
			t.Fatalf("base with no range %+v %v", got, e)
		}
	})
	f.advance(t, old.identity.Stamp, len(f.prefix))
	f.read(t, func(r CJSIReader) {
		page, p, err := r.DocumentsPage(ctx, old, 2, 2, false)
		if !errors.Is(err, sql.ErrNoRows) || page != nil || p != (Pagination{}) {
			t.Fatalf("stitched old page %+v %+v %v", page, p, err)
		}
		if a, e := r.ActivityPage(ctx, old, 1, 1, false); !errors.Is(e, sql.ErrNoRows) || !reflect.DeepEqual(a, activity.Page{}) {
			t.Fatalf("stale activity %+v %v", a, e)
		}
		selected, err = r.SelectDetail(ctx, "codex", adapterThread)
		if err != nil || selected.SourcePath != old.SourcePath || selected.identity.Stamp.Ticket != 1 || selected.identity.Sequence == old.identity.Sequence {
			t.Fatalf("selection %+v %v", selected, err)
		}
		for i, want := range []string{"AX", "BC", ""} {
			page, p, err := r.DocumentsPage(ctx, selected, i+1, 2, false)
			if err != nil || docLabels(page) != want || p.Total != 4 || p.HasMore != (i == 0) {
				t.Fatalf("page %d %+v %+v %v", i+1, page, p, err)
			}
		}
		list, err := r.List(ctx)
		if err != nil || len(list) != 3 {
			t.Fatalf("list %+v %v", list, err)
		}
		groups := map[string]string{}
		for _, m := range list {
			groups[m.SessionID] = m.SourcePath
		}
		if !reflect.DeepEqual(groups, map[string]string{adapterThread: f.path, "masked": f.path, "new-group": "moved"}) {
			t.Fatalf("effective groups %+v", groups)
		}
		got, err := r.Search(ctx, "A OR B OR C OR X")
		if err != nil || docLabels(got) != "ABCX" {
			t.Fatalf("stable MATCH %+v %v", got, err)
		}
		for _, match := range []struct {
			query string
			want  string
		}{
			{"{source_path client session_id text}:(A OR X)", "AX"},
			{"-{source_path client session_id}:(A OR X)", "AX"},
			{"{source_path client session_id}:(A OR X)", ""},
		} {
			got, err := r.Search(ctx, match.query)
			if err != nil || docLabels(got) != match.want {
				t.Errorf("MATCH %q = %q, %v; want %q", match.query, docLabels(got), err, match.want)
			} else {
				t.Logf("MATCH %q = %q", match.query, docLabels(got))
			}
		}
		got, err = r.Search(ctx, "maskword OR loserword OR excludedword OR unselectedword")
		if err != nil || len(got) != 0 {
			t.Fatalf("masked MATCH %+v %v", got, err)
		}
		got, err = r.Search(ctx, "movedword")
		if err != nil || len(got) != 1 || got[0].SessionID != "new-group" {
			t.Fatalf("moved payload %+v %v", got, err)
		}
		copies, err := r.EffectiveCopies(ctx, "codex", adapterThread)
		if err != nil || len(copies) != 2 || !copies[0].Selected || copies[1].Selected || copies[0].Source.Identity != f.identity || copies[0].Source.ParserVersion != 7 || copies[0].Source.PrefixHash != "hash" || copies[0].Birth != "2026-01-01T00:00:01Z" || copies[0].HeadAt != "2026-01-01T00:00:00Z" || copies[1].Metadata.SourcePath != "replica" {
			t.Fatalf("copies %+v %v", copies, err)
		}
		show, err := r.ShowWithActivity(ctx, "codex", adapterThread, true)
		if err != nil || docLabels(show.Documents) != "AXBC" || show.ActivitySummary == nil || show.ActivitySummary.Total != 1 || len(show.Activity) != 1 || show.Activity[0].Tool != "apply_patch" || show.Activity[0].Status != "started" {
			t.Fatalf("same-ticket prefix %+v %v", show, err)
		}
		a, e := r.ActivityPage(ctx, selected, 1, 1, false)
		if e != nil || a.Total != 1 || !reflect.DeepEqual(a.Details, show.Activity) {
			t.Fatalf("same-ticket activity page %+v %v", a, e)
		}
	})
	t.Run("core-only publication preserves session component", func(t *testing.T) {
		f.read(t, func(r CJSIReader) {
			// Pin this request's core read before another connection publishes.
			m, e := r.SelectDetail(ctx, "codex", adapterThread)
			if e != nil || m != selected {
				t.Fatalf("before core-only publication: %+v, %v", m, e)
			}
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=ticket+1`)
			if x, e := r.List(ctx); !errors.Is(e, store.ErrCJSIIndexChanged) || x != nil {
				t.Fatalf("obsolete caller core transaction returned %+v, %v", x, e)
			}
		})
		f.read(t, func(r CJSIReader) {
			m, e := r.SelectDetail(ctx, "codex", adapterThread)
			want := selected.identity
			want.Stamp.Ticket++
			if e != nil || m.identity != want {
				t.Fatalf("new core ticket changed session component: %+v, %v", m, e)
			}
			if docs, p, e := r.DocumentsPage(ctx, selected, 2, 2, false); !errors.Is(e, sql.ErrNoRows) || docs != nil || p != (Pagination{}) {
				t.Fatalf("continued prior core ticket: %+v, %+v, %v", docs, p, e)
			}
		})
	})
	for _, name := range []string{"list", "search", "selection", "page", "activity page", "show with activity", "copies"} {
		t.Run("revoke "+name, func(t *testing.T) {
			f.read(t, func(r CJSIReader) {
				// A new protection revision also requires a fresh page token.
				m, e := r.SelectDetail(ctx, "codex", adapterThread)
				if e != nil {
					t.Fatal(e)
				}
				original, ran := r.with, false
				r.with = func(ctx context.Context, read func(indexReadView) error) error {
					return original(ctx, func(v indexReadView) error {
						if err := read(v); err != nil {
							return err
						}
						adapterExec(t, f.private, `INSERT INTO cjsi_session_protections VALUES(?,'','',0)`, name)
						ran = true
						return nil
					})
				}
				var empty bool
				switch name {
				case "list":
					x, err := r.List(ctx)
					e, empty = err, x == nil
				case "search":
					x, err := r.Search(ctx, "X")
					e, empty = err, x == nil
				case "selection":
					x, err := r.SelectDetail(ctx, "codex", adapterThread)
					e, empty = err, x == (CJSISelectedDetail{})
				case "page":
					x, p, err := r.DocumentsPage(ctx, m, 1, 2, false)
					e, empty = err, x == nil && p == (Pagination{})
				case "activity page":
					x, err := r.ActivityPage(ctx, m, 1, 1, false)
					e, empty = err, reflect.DeepEqual(x, activity.Page{})
				case "show with activity":
					x, err := r.ShowWithActivity(ctx, "codex", adapterThread, true)
					e, empty = err, reflect.DeepEqual(x, Result{})
				case "copies":
					x, err := r.EffectiveCopies(ctx, "codex", adapterThread)
					e, empty = err, x == nil
				}
				if !ran || !errors.Is(e, store.ErrCJSIIndexChanged) || !empty {
					t.Fatalf("revoked %s: read=%v empty=%v err=%v", name, ran, empty, e)
				}
			})
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET protection_revision=protection_revision+1`)
		})
	}
	t.Run("same length changed file", func(t *testing.T) {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("apply_patch"), []byte("other_patch"), 1)
		if err := os.WriteFile(f.path, data, 0600); err != nil {
			t.Fatal(err)
		}
		f.read(t, func(r CJSIReader) {
			got, e := r.ShowWithActivity(t.Context(), "codex", adapterThread, true)
			if !errors.Is(e, ingest.ErrSourceChanged) || !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("unverified body %+v %v", got, e)
			}
		})
	})
	t.Run("publication shorter than captured prefix", func(t *testing.T) {
		f := newPhysicalFixture(t, true, true)
		var base CJSISelectedDetail
		f.read(t, func(r CJSIReader) {
			var err error
			base, err = r.SelectDetail(t.Context(), "codex", adapterThread)
			if err != nil {
				t.Fatal(err)
			}
		})
		f.advance(t, base.identity.Stamp, bytes.IndexByte(f.prefix, '\n')+1)
		f.read(t, func(r CJSIReader) {
			got, err := r.ShowWithActivity(t.Context(), "codex", adapterThread, true)
			if !errors.Is(err, sql.ErrNoRows) || !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("unpublished activity %+v %v", got, err)
			}
		})
	})
}

// Compare the established empty activity pagination contract against the
// selected zero-activity OTel projection. Both inputs have zero details.
func TestCJSISelectedOTelActivityPage(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	items := []TelemetrySession{{Client: "codex", SessionID: "settled", Model: "model", FirstAt: "2026-01-01T00:00:00Z", LastAt: "2026-01-01T00:00:00Z"}}
	var h store.CJSIIndexStamp
	var c *CJSIIndexCandidate
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h = s.Selection()
		var err error
		c, err = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, nil, nil, &items)
		return err
	})
	if c == nil {
		t.Fatal("OTel projection was not planned")
	}
	tx := stageTx(t, f)
	seal := c.Seal()
	if err := c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, seal); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?`, h.SessionRevision+1, seal[:])
	emptyFile := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(emptyFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.read(t, func(reader CJSIReader) {
		selected, err := reader.SelectDetail(ctx, "codex", "settled")
		if err != nil || !IsTelemetrySource(selected.SourcePath) {
			t.Fatalf("selection=%+v err=%v", selected, err)
		}
		for _, tc := range []struct {
			name        string
			page, limit int
			all         bool
		}{{"normal", 1, 10, false}, {"all", 1, 10, true}, {"zero_page", 0, 10, false}, {"oversized_limit", 1, 101, false}} {
			t.Run(tc.name, func(t *testing.T) {
				want, e := activity.ReadDetailsPage(emptyFile, "codex", "settled", tc.page, tc.limit, tc.all)
				got, err := reader.ActivityPage(ctx, selected, tc.page, tc.limit, tc.all)
				t.Logf("reference page/limit/total=%d/%d/%d err=%v; selected OTel=%d/%d/%d err=%v", want.Page, want.Limit, want.Total, e, got.Page, got.Limit, got.Total, err)
				if (e != nil) != (err != nil) || (e != nil && e.Error() != err.Error()) || !reflect.DeepEqual(want, got) {
					t.Error("selected OTel disagrees with empty activity pagination")
				}
			})
		}
	})
}

func TestCJSIPrivateReaderSeparatesSharedSourceGroups(t *testing.T) {
	ctx := t.Context()
	groups := []struct{ client, id, text string }{{"claude", "same", "alpha"}, {"codex", "same", "bravo"}, {"codex", "other", "charlie"}}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		for _, g := range groups {
			doc, err := ApprovedDocument(g.client, g.id, "user_prompt", g.text)
			if err != nil {
				t.Fatal(err)
			}
			if err := ReplaceDocuments(ctx, db, g.client, g.id, []Document{doc}); err != nil {
				t.Fatal(err)
			}
		}
		// Put the identical public IDs next to one another across clients.
		adapterExec(t, db, "UPDATE session_metadata SET last_at='2026-01-01T00:00:00Z' WHERE session_id='same'")

	})
	f.read(t, func(r CJSIReader) {
		for _, g := range groups {
			d, err := r.SelectDetail(ctx, g.client, g.id)
			if err != nil {
				t.Fatal(err)
			}
			docs, p, err := r.DocumentsPage(ctx, d, 1, 5, false)
			if err != nil || d.Client != g.client || d.SessionID != g.id || d.SourcePath != replaceDocumentsSourcePath || p.Total != 1 || docLabels(docs) != g.text {
				t.Fatalf("group %v: selected=%+v docs=%+v page=%+v err=%v", g, d, docs, p, err)
			}
			copies, err := r.EffectiveCopies(ctx, g.client, g.id)
			if err != nil || len(copies) != 1 || copies[0].Metadata.Client != g.client || copies[0].Metadata.SessionID != g.id || !copies[0].Selected {
				t.Fatalf("group %v: copies=%+v err=%v", g, copies, err)
			}
			show, err := r.ShowWithActivity(ctx, g.client, g.id, false)
			if err != nil || show.Client != g.client || show.SessionID != g.id || docLabels(show.Documents) != g.text {
				t.Fatalf("group %v: show=%+v err=%v", g, show, err)
			}
		}
		snap, err := r.ReadIndexSnapshot(ctx, "alpha OR bravo OR charlie")
		if err != nil || len(snap.Sessions) != 3 || len(snap.Documents) != 3 {
			t.Fatalf("shared-source snapshot %+v err=%v", snap, err)
		}
		for _, s := range snap.Sessions {
			if len(s.Copies) != 1 {
				t.Fatalf("wrong group copies: %+v", s)
			}
		}
	})
}

// Once the private snapshot is admitted, SQL and row-stream errors must
// discard every provisional result, even after an earlier row was delivered.
func TestCJSIReadFailureBoundaries(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	var selected CJSISelectedDetail
	f.read(t, func(reader CJSIReader) {
		var err error
		selected, err = reader.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
	})
	f.advance(t, selected.identity.Stamp, len(f.prefix))
	f.read(t, func(reader CJSIReader) {
		var err error
		selected, err = reader.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
	})
	type readCase struct {
		name string
		run  func(CJSIReader, store.CJSIIndexSnapshot) (bool, error)
	}
	for _, tc := range []readCase{
		{"list", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) { v, e := r.List(ctx); return v == nil, e }},
		{"search", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.Search(ctx, "A OR X")
			return v == nil, e
		}},
		{"select", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.SelectDetail(ctx, "codex", adapterThread)
			return v == (CJSISelectedDetail{}), e
		}},
		{"page", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, p, e := r.DocumentsPage(ctx, selected, 1, 2, false)
			return v == nil && p == (Pagination{}), e
		}},
		{"copies", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.EffectiveCopies(ctx, "codex", adapterThread)
			return v == nil, e
		}},
		{"snapshot", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.ReadIndexSnapshot(ctx, "A OR X")
			return reflect.DeepEqual(v, IndexSnapshot{}), e
		}},
		{"order", func(_ CJSIReader, s store.CJSIIndexSnapshot) (bool, error) {
			v, e := ReadCJSIOrderSnapshot(ctx, s)
			return reflect.DeepEqual(v, CJSIOrderSnapshot{}), e
		}},
		{"show", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.ShowWithActivity(ctx, "codex", adapterThread, true)
			return reflect.DeepEqual(v, Result{}), e
		}},
		{"activity page", func(r CJSIReader, _ store.CJSIIndexSnapshot) (bool, error) {
			v, e := r.ActivityPage(ctx, selected, 1, 2, false)
			return reflect.DeepEqual(v, activity.Page{}), e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				exerciseIndexFailures(t, f, false, func(tx *sql.Tx) error {
					// The wrapped connection reaches the same physical file as the
					// admitted stamp; only the injected driver operation changes.
					injected := s
					injected.Tx = tx
					r := CJSIReader{with: func(ctx context.Context, visit func(indexReadView) error) error {
						return visit(privateIndexView(injected, ingest.CodexSelectedLimits{MaxBytes: 1 << 20, MaxLineBytes: 4096, MaxRecords: 64}))
					}}
					empty, err := tc.run(r, injected)
					if err != nil && !empty {
						t.Errorf("failed %s exposed provisional data: %v", tc.name, err)
					}
					return err
				})
				return nil
			})
		})
	}
}

// The same local, visible messages must keep their normalized search times
// and order through the old parser and the staged native reader.
func TestCJSIDocumentEventTimeParity(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "rollout-2026-01-02T03-04-05-"+adapterThread+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	input := `{"type":"session_meta","payload":{"id":"` + adapterThread + `","timestamp":"2026-01-01T00:00:00Z"}}
{"type":"response_item","timestamp":"2026-01-01T01:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"needle earlier"}]}}
{"type":"response_item","payload":{"timestamp":"2026-01-01T03:00:00.5+02:00","type":"message","role":"user","content":[{"type":"input_text","text":"needle later"}]}}
`
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if _, err := Scan(ctx, legacy.DB, home); err != nil {
		t.Fatal(err)
	}
	want, err := Search(ctx, legacy.DB, "needle")
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 2 || want[0].Text != "needle later" || want[0].EventAt != "2026-01-01T01:00:00.5Z" {
		t.Errorf("invalid legacy order %+v", want)
	}
	legacyMeta, err := ShowMetadata(ctx, legacy.DB, "codex", adapterThread)
	if err != nil {
		t.Fatal(err)
	}
	legacyShow, page, err := DocumentsPage(ctx, legacy.DB, legacyMeta, 1, 10, true)
	if err != nil || page.Total != 2 || len(legacyShow) != 2 {
		t.Fatalf("legacy show documents %+v %+v %v", legacyShow, page, err)
	}
	src := fixturePreparedNative(t, path, adapterThread, input)
	f := newPhysicalFixture(t, true, false)
	var h store.CJSIIndexStamp
	var c *CJSIIndexCandidate
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h = s.Selection()
		var e error
		c, e = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{src}, nil, nil)
		return e
	})
	if c == nil {
		t.Fatal("empty candidate")
	}
	tx := stageTx(t, f)
	seal := c.Seal()
	if err := c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, seal); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?`, h.SessionRevision+1, seal[:])
	f.read(t, func(r CJSIReader) {
		got, err := r.Search(ctx, "needle")
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != 2 || got[0].Text != "needle later" || got[0].EventAt != "2026-01-01T01:00:00.5Z" {
			t.Errorf("invalid CJSI order %+v", got)
		}
		if !reflect.DeepEqual(got, want) {
			t.Error("search event_at or order changed for identical proved local documents")
		}
		show, err := r.ShowWithActivity(ctx, "codex", adapterThread, false)
		if err != nil || !reflect.DeepEqual(show.Documents, legacyShow) {
			t.Fatalf("show: docs=%+v err=%v", show.Documents, err)
		}

	})
}

func TestCJSICompatibleActivityRevalidatesFile(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "activity.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	input := "{\"type\":\"user\",\"sessionId\":\"activity\",\"message\":{\"role\":\"user\",\"content\":\"approved\"}}\n"
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if result, err := Scan(ctx, db, home); err != nil || result.Sources != 1 {
			t.Fatalf("scan %+v err=%v", result, err)
		}
	})
	var selected CJSISelectedDetail
	f.read(t, func(r CJSIReader) {
		var err error
		selected, err = r.SelectDetail(ctx, "claude", "activity")
		if err != nil {
			t.Fatal(err)
		}
		show, err := r.ShowWithActivity(ctx, "claude", "activity", true)
		if err != nil || docLabels(show.Documents) != "approved" || show.ActivitySummary == nil || show.ActivitySummary.Total != 0 {
			t.Fatalf("compatible show %+v err=%v", show, err)
		}
		p, err := r.ActivityPage(ctx, selected, 1, 5, false)
		if err != nil || p.Page != 1 || p.Limit != 5 || p.Total != 0 {
			t.Fatalf("compatible page %+v err=%v", p, err)
		}
	})
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		readErr := errors.New("activity read failed")
		value, err := readCompatibleActivity(ctx, s, selected.Metadata, func(*os.File, int64) (string, error) { return "provisional", readErr })
		if value != "" || !errors.Is(err, readErr) {
			t.Fatalf("failed reader returned %q %v", value, err)
		}
		value, err = readCompatibleActivity(ctx, s, selected.Metadata, func(*os.File, int64) (string, error) {
			if err := os.WriteFile(path, []byte(input+"{}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			return "provisional", nil
		})
		if value != "" || !errors.Is(err, ingest.ErrSourceChanged) {
			t.Fatalf("changed file returned %q %v", value, err)
		}
		called := false
		value, err = readCompatibleActivity(ctx, s, selected.Metadata, func(*os.File, int64) (string, error) { called = true; return "new file", nil })
		if called || value != "" || !errors.Is(err, ingest.ErrSourceChanged) {
			t.Fatalf("stale selection entered reader=%v value=%q err=%v", called, value, err)
		}
		return nil
	})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		got, err := readCompatibleActivity(ctx, s, selected.Metadata, func(*os.File, int64) (string, error) { t.Fatal("missing source opened"); return "", nil })
		if got != "" || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing source: %q %v", got, err)
		}
		return nil
	})
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("fifo_replacement", func(t *testing.T) {
		check := func(t *testing.T, read func() error) {
			t.Helper()
			done := make(chan error, 1)
			go func() { done <- read() }()
			select {
			case err := <-done:
				if !errors.Is(err, ingest.ErrSourceChanged) {
					t.Fatalf("FIFO replacement returned %v, want ErrSourceChanged", err)
				}
			case <-time.After(5 * time.Second):
				// Release a blocking open even if the reader has not reached it yet.
				peer, err := os.OpenFile(path, os.O_RDWR|unix.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				select {
				case <-done:
					t.Fatal("activity read waited for a FIFO writer")
				case <-time.After(5 * time.Second):
					t.Fatal("activity read failed to exit after FIFO was opened")
				}
			}
		}
		t.Run("callback", func(t *testing.T) {
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
				check(t, func() error {
					_, err := readCompatibleActivity(ctx, s, selected.Metadata, func(*os.File, int64) (bool, error) {
						t.Error("reader callback entered for FIFO")
						return false, nil
					})
					return err
				})
				return nil
			})
		})
		t.Run("activity_page", func(t *testing.T) {
			f.read(t, func(r CJSIReader) {
				check(t, func() error {
					_, err := r.ActivityPage(ctx, selected, 1, 5, false)
					return err
				})
			})
		})
	})
}

func TestCJSIBareHandleRejectsPrivateReadsAndWrites(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	m := Metadata{Client: "codex", SessionID: adapterThread, SourcePath: f.path}
	for _, tc := range []struct {
		name string
		use  func() error
	}{
		{"search", func() error {
			v, e := Search(ctx, f.private, "A")
			if v != nil {
				t.Fatal("bare search data")
			}
			return e
		}},
		{"metadata", func() error {
			v, e := ShowMetadata(ctx, f.private, m.Client, m.SessionID)
			if v != (Metadata{}) {
				t.Fatal("bare metadata")
			}
			return e
		}},
		{"page", func() error {
			d, p, e := DocumentsPage(ctx, f.private, m, 1, 5, false)
			if d != nil || p != (Pagination{}) {
				t.Fatal("bare documents")
			}
			return e
		}},
		{"show", func() error {
			v, e := ShowWithActivity(ctx, f.private, m.Client, m.SessionID, false)
			if !reflect.DeepEqual(v, Result{}) {
				t.Fatal("bare show data")
			}
			return e
		}},
		{"snapshot", func() error {
			v, e := ReadIndexSnapshot(ctx, f.private, "A")
			if !reflect.DeepEqual(v, IndexSnapshot{}) {
				t.Fatal("bare index snapshot")
			}
			return e
		}},
		{"replace", func() error { return ReplaceDocuments(ctx, f.private, m.Client, m.SessionID, nil) }},
		{"exclude", func() error { return Exclude(ctx, f.private, "session", m.SessionID) }},
		{"rebuild", func() error { _, e := Rebuild(ctx, f.private, t.TempDir()); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.use(); !errors.Is(err, store.ErrCJSIIndexChanged) {
				t.Fatalf("bare private handle: %v", err)
			}
		})
	}
	f.read(t, func(r CJSIReader) {
		got, err := r.ShowWithActivity(ctx, m.Client, m.SessionID, false)
		if err != nil || docLabels(got.Documents) != "ABC" {
			t.Fatalf("private index changed: %+v %v", got, err)
		}
	})
}

func TestCJSIMigratedClaudePublishedActivityPrefix(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "p", "partial.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	complete := `{"type":"user","sessionId":"s","message":{"role":"user","content":"hello"}}
{"type":"assistant","timestamp":"2026-07-20T00:00:01Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","id":"call","name":"Read","input":{}}]}}
`
	tail := `{"type":"assistant"`
	if err := os.WriteFile(path, []byte(complete+tail), 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := activity.ReadDetailsPage(path, "claude", "s", 1, 10, false)
	if err != nil || legacy.Total != 1 {
		t.Fatalf("reference activity page %+v err=%v", legacy, err)
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if scan, e := Scan(ctx, db, home); e != nil || scan.Sources != 1 {
			t.Fatalf("scan %+v %v", scan, e)
		}
	})
	src, err := ingest.Discover(home)
	if err != nil || len(src) != 1 {
		t.Fatal(src, err)
	}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		var cursor, size, read, end int64
		if err := s.Tx.QueryRowContext(ctx, `SELECT b.cursor,b.size,h.read_end,h.complete_end FROM session_sources b JOIN cjsi_legacy_cursor_hints h USING(source_path,identity) WHERE b.source_path=?`, path).Scan(&cursor, &size, &read, &end); err != nil {
			return err
		}
		ok, e := CJSISourceUnchanged(ctx, s, CJSISourceObservation{Source: src[0], Kind: "claude-jsonl"})
		t.Logf("cursor=%d size=%d hinted_read_end=%d hinted_complete_end=%d unchanged=%v err=%v", cursor, size, read, end, ok, e)
		if cursor != int64(len(complete)) || end != cursor || read != size || size != int64(len(complete+tail)) || !ok || e != nil {
			t.Fatal("invalid migration control")
		}
		return nil
	})
	f.read(t, func(r CJSIReader) {
		m, e := r.SelectDetail(ctx, "claude", "s")
		if e != nil {
			t.Fatal(e)
		}
		page, e := r.ActivityPage(ctx, m, 1, 10, false)
		t.Logf("unmodified captured file: reference=%d selected=%d err=%v", legacy.Total, page.Total, e)
		if e != nil || page.Total != 1 || len(page.Details) != 1 || page.Details[0].Tool != legacy.Details[0].Tool {
			t.Error("published complete activity was unavailable after migration")
		}
		show, e := r.ShowWithActivity(ctx, "claude", "s", true)
		if e != nil || !reflect.DeepEqual(show.Activity, legacy.Details) {
			t.Fatalf("prefix show activity=%+v err=%v", show.Activity, e)
		}

	})

	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		returnValue, err := readCompatibleActivity(ctx, s, Metadata{SourcePath: path, Client: "claude", SessionID: "s"}, func(file *os.File, end int64) (int64, error) {
			if file.Name() != path || end != int64(len(complete)) {
				t.Fatalf("unexpected bounded reader input: %s end=%d", file.Name(), end)
			}
			return end, nil
		})
		if err != nil || returnValue != int64(len(complete)) {
			t.Fatalf("published boundary: %d %v", returnValue, err)
		}
		return nil
	})
	adapterExec(t, f.private, "DELETE FROM cjsi_legacy_cursor_hints WHERE source_path=?", path)
	f.read(t, func(r CJSIReader) {
		m, e := r.SelectDetail(ctx, "claude", "s")
		if e != nil {
			t.Fatal(e)
		}
		page, e := r.ActivityPage(ctx, m, 1, 10, false)
		if !errors.Is(e, sql.ErrNoRows) || !reflect.DeepEqual(page, activity.Page{}) {
			t.Fatalf("missing hint returned %+v, %v", page, e)
		}
	})
}

func TestCJSIPrivateReaderRequiresPhysicalPayloadOwner(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error { h = s.Selection(); return nil })
	f.advance(t, h, len(f.prefix))
	// A physically present FTS row has no membership authority. An orphan
	// with a candidate's number and a moved-away base payload must not leak.
	adapterExec(t, f.private, `INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text) VALUES(8,?,'codex',?,'','user_prompt','orphan sentinel')`, f.path, adapterThread)
	adapterExec(t, f.private, `UPDATE session_documents SET source_path='other-owner',text='relocated sentinel' WHERE rowid=1`)
	f.read(t, func(reader CJSIReader) {
		got, err := reader.Search(ctx, "sentinel")
		if err != nil || len(got) != 0 {
			t.Fatalf("ineligible MATCH %+v err=%v", got, err)
		}
		selected, err := reader.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		page, p, err := reader.DocumentsPage(ctx, selected, 1, 10, true)
		if err != nil || p.Total != 3 || docLabels(page) != "XBC" {
			t.Fatalf("ineligible detail %+v %+v err=%v", page, p, err)
		}
	})
}

func TestCJSIPartialActivityRequiresExactHint(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	directory := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		path := filepath.Join(directory, id+".jsonl")
		body := "{\"type\":\"user\",\"sessionId\":\"" + id + "\",\"message\":{\"role\":\"user\",\"content\":\"ok\"}}\n{\"pending\":"
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if scan, err := Scan(ctx, db, home); err != nil || scan.Sources != 2 {
			t.Fatal(scan, err)
		}
	})
	src, err := ingest.Discover(home)
	if err != nil || len(src) != 2 {
		t.Fatal(src, err)
	}
	path := filepath.Join(directory, "one.jsonl")
	var o CJSISourceObservation
	for _, source := range src {
		if source.Path == path {
			o = CJSISourceObservation{Source: source, Kind: "claude-jsonl"}
		}
	}
	if o.Path == "" {
		t.Fatal("missing discovered path")
	}
	m := Metadata{SourcePath: path, Client: "claude", SessionID: "one"}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, tc := range []struct {
			name, statement string
			want            bool
		}{
			{"exact", "", true},
			{"known compatible kind", "UPDATE cjsi_base_groups SET source_kind='claude-jsonl' WHERE source_path=?", true},
			{"other path only", "DELETE FROM cjsi_legacy_cursor_hints WHERE source_path=?", false},
			{"identity", "UPDATE cjsi_legacy_cursor_hints SET identity='other' WHERE source_path=?", false},
			{"observed size", "UPDATE cjsi_legacy_cursor_hints SET size=size+1 WHERE source_path=?", false},
			{"read end", "UPDATE cjsi_legacy_cursor_hints SET read_end=read_end-1 WHERE source_path=?", false},
			{"complete end", "UPDATE cjsi_legacy_cursor_hints SET complete_end=complete_end-1 WHERE source_path=?", false},
			{"mtime", "UPDATE cjsi_legacy_cursor_hints SET modified_at=modified_at+1 WHERE source_path=?", false},
			{"ctime", "UPDATE cjsi_legacy_cursor_hints SET changed_at=changed_at+1 WHERE source_path=?", false},
			{"prefix", "UPDATE cjsi_legacy_cursor_hints SET prefix_hash='other' WHERE source_path=?", false},
			{"parser", "UPDATE cjsi_legacy_cursor_hints SET parser_version=parser_version+1 WHERE source_path=?", false},
			{"incompatible source kind", "UPDATE cjsi_base_groups SET source_kind='synthetic' WHERE source_path=?", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx := stageTx(t, f)
				defer tx.Rollback()
				if tc.statement != "" {
					if _, err := tx.ExecContext(ctx, tc.statement, path); err != nil {
						t.Fatal(err)
					}
				}
				pinned := s
				pinned.Tx = tx
				ok, e := CJSISourceUnchanged(ctx, pinned, o)
				if e != nil || ok != tc.want {
					t.Fatalf("source reuse=%v err=%v want %v", ok, e, tc.want)
				}
				called := false
				got, e := readCompatibleActivity(ctx, pinned, m, func(file *os.File, end int64) (string, error) {
					called = true
					if end <= 0 || end >= o.Size {
						t.Fatal("not a complete-line prefix")
					}
					return "safe", nil
				})
				if tc.want {
					if e != nil || !called || got != "safe" {
						t.Fatal(got, e)
					}
				} else if !errors.Is(e, sql.ErrNoRows) || called || got != "" {
					t.Fatalf("bad hint read: called=%v value=%q err=%v", called, got, e)
				}
			})
		}
		return nil
	})
}

func TestCJSIActivityRangeBelongsToRequestedCopy(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error { h = s.Selection(); return nil })
	f.advance(t, h, len(f.prefix))
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		m := Metadata{Client: "codex", SessionID: adapterThread, SourcePath: f.path}
		ok, err := selectedActivitySource(ctx, s, m)
		if err != nil || ok.Path != f.path {
			t.Fatal(ok, err)
		}
		for _, bad := range []Metadata{{Client: "claude", SessionID: m.SessionID, SourcePath: m.SourcePath}, {Client: m.Client, SessionID: "unknown", SourcePath: m.SourcePath}, {Client: m.Client, SessionID: m.SessionID, SourcePath: "other-file"}} {
			selected, err := selectedActivitySource(ctx, s, bad)
			if !errors.Is(err, sql.ErrNoRows) || selected != (ingest.CodexSelectedSource{}) {
				t.Fatalf("wrong copy %+v yielded %+v err=%v", bad, selected, err)
			}
		}
		return nil
	})
}

func TestCJSISelectedPartialRangeCarriesReadBoundary(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	path := filepath.Join(t.TempDir(), "native.jsonl")
	complete := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"timestamp":"2026-01-01T00:00:01Z","type":"response_item","payload":{"type":"function_call","name":"apply_patch","call_id":"started"}}
`
	body := complete + `{"type":"response_item"`
	src := fixturePreparedNative(t, path, adapterThread, body)
	var c *CJSIIndexCandidate
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		h = s.Selection()
		var err error
		c, err = PlanCJSIIndex(ctx, s, 1, 1, h.SessionRevision+1, []CJSIPreparedSource{src}, nil, nil)
		return err
	})
	if c == nil {
		t.Fatal("missing partial capture")
	}
	seal := c.Seal()
	tx := stageTx(t, f)
	if err := c.Stage(ctx, tx, h, 1, 1, h.SessionRevision+1, seal); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, "UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?", h.SessionRevision+1, seal[:])
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		selected, err := selectedActivitySource(ctx, s, src.replacement.Metadata)
		if err != nil {
			return err
		}
		if selected != src.replacement.Selected || !selected.Evidence.Progress.Partial || selected.Evidence.Progress.CompleteEnd != int64(len(complete)) {
			t.Fatalf("partial proof changed %+v", selected)
		}
		return nil
	})
}

func TestCJSICompatibleActivityDoesNotOpenAnotherSelection(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		raw := "{\"type\":\"user\",\"sessionId\":\"" + id + "\",\"message\":{\"role\":\"user\",\"content\":\"ok\"}}\n{\"partial\":"
		if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := newMigratedFixture(t, true, false, func(db *sql.DB) {
		if _, err := Scan(ctx, db, home); err != nil {
			t.Fatal(err)
		}
	})
	base := Metadata{SourcePath: filepath.Join(dir, "one.jsonl"), Client: "claude", SessionID: "one"}
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		for _, name := range []string{"missing selected path", "other client", "other logical session", "zero change stamp", "invalid cursor", "metadata parser differs", "unsupported but matching parser", "mixed source group", "unrelated mixed group"} {
			t.Run(name, func(t *testing.T) {
				tx := stageTx(t, f)
				defer tx.Rollback()
				pinned := s
				pinned.Tx = tx
				m := base
				exec := func(q string, args ...any) {
					t.Helper()
					if _, err := tx.ExecContext(ctx, q, args...); err != nil {
						t.Fatal(err)
					}
				}
				switch name {
				case "missing selected path":
					m.SourcePath = filepath.Join(dir, "missing.jsonl")
				case "other client":
					m.Client = "codex"
				case "other logical session":
					m.SessionID = "other"
				case "zero change stamp":
					exec("UPDATE session_sources SET changed_at=0,cursor=size WHERE source_path=?", base.SourcePath)
				case "invalid cursor":
					exec("UPDATE session_sources SET size=-1,cursor=-1 WHERE source_path=?", base.SourcePath)
				case "metadata parser differs":
					exec("UPDATE session_metadata SET parser_version=5 WHERE source_path=?", base.SourcePath)
				case "unsupported but matching parser":
					for _, table := range []string{"session_metadata", "session_sources", "cjsi_legacy_cursor_hints"} {
						exec("UPDATE "+table+" SET parser_version=5 WHERE source_path=?", base.SourcePath)
					}
				case "mixed source group", "unrelated mixed group":
					path := base.SourcePath
					if name == "unrelated mixed group" {
						path = filepath.Join(dir, "two.jsonl")
					}
					exec("INSERT INTO session_metadata VALUES(?,'codex','other','','',6,'','')", path)
					exec("INSERT INTO cjsi_base_groups(source_path,client,session_id,sequence_id) VALUES(?,'codex','other',randomblob(32))", path)
				}
				want := name == "unrelated mixed group"
				called := false
				got, err := readCompatibleActivity(ctx, pinned, m, func(*os.File, int64) (bool, error) { called = true; return true, nil })
				if got != want || called != want || want && err != nil || !want && !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("admitted=%v called=%v err=%v want=%v", got, called, err, want)
				}
			})
		}
		return nil
	})
}

// A selected replacement at the same path does not grant access to the old
// physical checkpoint, even if the old file is still readable and unchanged.
func TestCJSICompatibleActivityRejectsReplacedCheckpoint(t *testing.T) {
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
	var h store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error { h = s.Selection(); return nil })
	adapterExec(t, f.private, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision) VALUES(1,1,?,?,?,?,?,?,?,?,?,?)`, h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.Ticket, h.ProtectionRevision, h.CoverageRevision, h.SessionRevision+1)
	adapterExec(t, f.private, `INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at) SELECT 1,1,source_path,'upsert','claude-jsonl','different-inode',priority,parser_version,cursor,size,modified_at,changed_at,prefix_hash,scanned_at FROM session_sources WHERE source_path=?`, path)
	adapterExec(t, f.private, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,order_verified,sequence_id) SELECT 1,1,source_path,client,session_id,'replace',project,model,parser_version,first_at,last_at,1,randomblob(32) FROM session_metadata WHERE source_path=?`, path)
	seal := sha256.Sum256([]byte("replacement selection"))
	adapterExec(t, f.private, "UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=1 AND attempt=1", seal[:])
	adapterExec(t, f.core, "UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,session_revision=?,session_seal=?", h.SessionRevision+1, seal[:])
	f.read(t, func(reader CJSIReader) {
		m, err := reader.SelectDetail(ctx, "claude", "s")
		if err != nil {
			t.Fatal(err)
		}
		if got, err := reader.ActivityPage(ctx, m, 1, 1, false); !errors.Is(err, sql.ErrNoRows) || !reflect.DeepEqual(got, activity.Page{}) {
			t.Fatalf("read former inode: page=%+v err=%v", got, err)
		}
	})
}
