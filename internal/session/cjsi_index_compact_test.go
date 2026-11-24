//go:build cjsi_adapter_fixture

package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type compactFixture struct {
	physicalFixture
	plan           indexStageCandidate
	selected       store.CJSIIndexStamp
	path, coverage string
}

// Install through Store, seal through the reviewed writer, and select that
// seal on the synthetic core only. The real reader supplies the admission
// stamp. A later core-only ticket carries the same session component.
func newCompactFixture(t *testing.T, messages, move bool, coverage string, seed ...func(*sql.DB)) compactFixture {
	t.Helper()
	f := newMigratedFixture(t, true, true, func(db *sql.DB) {
		for _, setup := range seed {
			setup(db)
		}
	})
	adapterExec(t, f.private, `INSERT INTO inventory_runs VALUES('active',1,'t',0,?),
		('other',2,'t',0,'partial'); INSERT INTO domain_coverage_heads VALUES(1,'active')`, coverage)
	adapterExec(t, f.private, `INSERT INTO cjsi_session_protections VALUES('replica','','',0);
		UPDATE cjsi_index_state SET base_revision=(SELECT revision FROM serving_generation)`)
	var revision int64
	if err := f.private.QueryRow("SELECT revision FROM serving_generation").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET base_revision=?,session_revision=?,protection_revision=1`, revision, revision)
	path := f.path
	if move {
		path = filepath.Join(t.TempDir(), filepath.Base(f.path))
		if err := os.Rename(f.path, path); err != nil {
			t.Fatal(err)
		}
	}
	c := stageFixturePlan(t, f, path, messages)
	r := &c.Replacements[0]
	if messages {
		// Earlier content owns the second accepted record. The new detail
		// order (8,2) is deliberately the reverse of ascending stable IDs.
		d := r.Material.Documents[1]
		adapterExec(t, f.private, `UPDATE cjsi_base_order SET record_start=NULL,record_end=NULL,emission=NULL WHERE doc_id=1`)
		adapterExec(t, f.private, `UPDATE cjsi_base_order SET record_start=?,record_end=?,emission=0 WHERE doc_id=2`, d.Span.Start, d.Span.End)
		adapterExec(t, f.private, `UPDATE session_documents SET event_at=?,kind=?,text=? WHERE rowid=2`, d.EventAt, d.Kind, d.Text)
		r.DocumentIDs = []int64{8, 2}
		r.DeletedDocumentIDs = []int64{1}
	}
	if move {
		r.From = f.path
		c.DeletedSources = append(c.DeletedSources, indexSourceDelete{f.path, f.identity})
	}
	tx := stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	a := c.Attempt
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=?,session_revision=?,session_seal=?`,
		a.Ticket+1, a.Ticket, a.Attempt, a.SessionRevision, a.Seal[:])
	var h store.CJSIIndexStamp
	f.read(t, func(r CJSIReader) {
		d, err := r.SelectDetail(t.Context(), "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		h = d.identity.Stamp
	})
	return compactFixture{f, c, h, path, coverage}
}

func (f compactFixture) current(context.Context) (store.CJSIIndexStamp, error) {
	return f.selected, nil
}

type compactObserved struct {
	detail        CJSISelectedDetail
	docs          []Document
	page          Pagination
	list          []Metadata
	copies        []EffectiveSessionCopy
	hits          map[string][]Document
	ids, ordinals []int64
	ranges        [][]any
}

func (f compactFixture) observe(t *testing.T) compactObserved {
	t.Helper()
	var got compactObserved
	ctx := t.Context()
	f.read(t, func(r CJSIReader) {
		var err error
		got.detail, err = r.SelectDetail(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		got.docs, got.page, err = r.DocumentsPage(ctx, got.detail, 1, 20, true)
		if err != nil {
			t.Fatal(err)
		}
		got.list, err = r.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got.copies, err = r.EffectiveCopies(ctx, "codex", adapterThread)
		if err != nil {
			t.Fatal(err)
		}
		got.hits = make(map[string][]Document)
		for _, q := range []string{"text:A OR text:stageword", "stageword", "B OR C OR maskword OR movedword OR loserword OR excludedword", "kind:assistant_final"} {
			got.hits[q], err = r.Search(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	tx, err := f.core.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	err = store.WithCJSIIndexSnapshot(ctx, f.root, tx, func(s store.CJSIIndexSnapshot) error {
		rows, err := s.Tx.QueryContext(ctx, "SELECT doc_id,detail_ordinal FROM ("+s.MembersSQL()+") WHERE source_path=? ORDER BY detail_ordinal", f.path)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, n int64
			if err = rows.Scan(&id, &n); err != nil {
				rows.Close()
				return err
			}
			got.ids = append(got.ids, id)
			got.ordinals = append(got.ordinals, n)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err = rows.Close(); err != nil {
			return err
		}
		rows, err = s.Tx.QueryContext(ctx, "SELECT * FROM ("+s.RangesSQL()+") ORDER BY source_path,client,session_id")
		if err != nil {
			return err
		}
		got.ranges, err = compactRows(rows)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCJSIIndexCompaction(t *testing.T) {
	for _, tt := range []struct {
		name       string
		docs, move bool
		coverage   string
	}{
		{"replacement restores complete", true, false, "complete"},
		{"move preserves pending", true, true, "pending"},
		{"empty moved replacement", false, true, "update_pending"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCompactFixture(t, tt.docs, tt.move, tt.coverage)
			before := f.observe(t)
			if before.detail.identity.Stamp != f.selected || before.detail.SourcePath != f.path || len(before.copies) != 1 || len(before.list) != 1 || len(before.ranges) != 1 {
				t.Fatalf("unexpected selected group: %+v", before)
			}
			if before.page.Total != len(f.plan.Replacements[0].DocumentIDs) || !reflect.DeepEqual(before.ids, f.plan.Replacements[0].DocumentIDs) ||
				len(before.hits["B OR C OR maskword OR movedword OR loserword OR excludedword"]) != 0 || len(before.hits["kind:assistant_final"]) != 0 {
				t.Fatalf("selected members: %+v", before)
			}
			if tt.docs && (docLabels(before.docs) != "Astageword" || !reflect.DeepEqual(before.ordinals, []int64{1, 2}) || len(before.hits["stageword"]) != 1 || len(before.hits["text:A OR text:stageword"]) != 2) {
				t.Fatalf("order/MATCH: %+v", before)
			}
			if !tt.docs && (len(before.docs) != 0 || len(before.hits["text:A OR text:stageword"]) != 0) {
				t.Fatal("empty replacement leaked a hit")
			}

			tx := stageTx(t, f.physicalFixture)
			// Observe the actual trigger-generated revision and coverage state
			// at normalization, and the logical revision at final transfer.
			if _, err := tx.ExecContext(t.Context(), `CREATE TEMP TABLE compact_trace(phase,rev,state);
				CREATE TEMP TRIGGER compact_trace_coverage AFTER UPDATE OF state ON main.inventory_runs
				WHEN NEW.inventory_id='active' BEGIN INSERT INTO compact_trace VALUES('coverage',0,OLD.state||'>'||NEW.state); END;
				CREATE TEMP TRIGGER compact_trace_rev BEFORE UPDATE OF revision ON main.serving_generation
				WHEN NEW.revision<=OLD.revision BEGIN INSERT INTO compact_trace SELECT 'restore',OLD.revision,state FROM main.inventory_runs WHERE inventory_id='active'; END;
				CREATE TEMP TRIGGER compact_trace_ticket BEFORE UPDATE OF compacted_ticket ON main.cjsi_index_state
				BEGIN INSERT INTO compact_trace SELECT 'transfer',revision,state FROM main.serving_generation,main.inventory_runs WHERE inventory_id='active'; END;`); err != nil {
				t.Fatal(err)
			}
			if err := compactIndexCandidate(t.Context(), tx, f.plan.Attempt, f.selected, f.current); err != nil {
				t.Fatal(err)
			}
			// Independently count the fixture's source and metadata mutations.
			writes := int64(6) // upsert source and group, delete replica and its group, delete two siblings
			if tt.move {
				writes = 8
			} // also delete old selected source and metadata
			var temporary, logical int64
			var observedCoverage, transferredCoverage string
			if err := tx.QueryRow("SELECT rev,state FROM compact_trace WHERE phase='restore'").Scan(&temporary, &observedCoverage); err != nil {
				t.Fatal(err)
			}
			if err := tx.QueryRow("SELECT rev,state FROM compact_trace WHERE phase='transfer'").Scan(&logical, &transferredCoverage); err != nil {
				t.Fatal(err)
			}
			if temporary != f.selected.BaseRevision+writes || logical != f.selected.SessionRevision || observedCoverage != tt.coverage || transferredCoverage != tt.coverage {
				t.Fatalf("trigger trace: %d/%s -> %d/%s; base %d", temporary, observedCoverage, logical, transferredCoverage, f.selected.BaseRevision)
			}
			var transitions string
			if err := tx.QueryRow("SELECT coalesce(group_concat(state,','),'') FROM (SELECT state FROM compact_trace WHERE phase='coverage' ORDER BY rowid)").Scan(&transitions); err != nil {
				t.Fatal(err)
			}
			wantTransition := ""
			if tt.coverage == "complete" {
				wantTransition = "complete>update_pending,update_pending>complete"
			}
			if transitions != wantTransition {
				t.Fatalf("coverage trace %q, want %q", transitions, wantTransition)
			}
			t.Logf("coverage transitions %q", transitions)
			t.Logf("base revision %d, after %d trigger increments %d, normalized before transfer %d; coverage %s", f.selected.BaseRevision, writes, temporary, logical, tt.coverage)
			stageScalar(t, f.private, int64(0), "SELECT compacted_ticket FROM cjsi_index_state")
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			after := f.observe(t)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("selected reader changed: before %+v after %+v", before, after)
			}
			h := f.selected
			stageScalar(t, f.private, 1, `SELECT count(*) FROM cjsi_index_state WHERE compacted_ticket=? AND compacted_attempt=? AND compacted_seal=?
				AND base_revision=? AND protection_revision=? AND doc_id_high_water=?`, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.ProtectionRevision, 7+len(before.hits["stageword"]))
			stageScalar(t, f.private, tt.coverage, "SELECT state FROM inventory_runs WHERE inventory_id='active'")
			stageScalar(t, f.private, "partial", "SELECT state FROM inventory_runs WHERE inventory_id='other'")
			stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_session_protections WHERE source_path='replica'")
			stageScalar(t, f.private, len(before.docs), "SELECT count(*) FROM session_documents WHERE source_path=?", f.path)
			stageScalar(t, f.private, 0, "SELECT count(*) FROM session_documents WHERE session_documents MATCH 'B OR C OR maskword OR movedword OR loserword'")
			stageScalar(t, f.private, 1, "SELECT count(*) FROM session_documents WHERE session_documents MATCH 'excludedword'")
			if tt.docs {
				stageScalar(t, f.private, int64(2), "SELECT rowid FROM session_documents WHERE session_documents MATCH 'text:stageword'")
			}
			again := stageTx(t, f.physicalFixture)
			if err := compactIndexCandidate(t.Context(), again, f.plan.Attempt, h, f.current); err == nil || !errors.Is(again.Commit(), sql.ErrTxDone) {
				t.Fatalf("reapplied compacted delta: %v", err)
			}
			// A former ticket cannot read the compacted physical base.
			p := f.plan.Attempt.Parent
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=?,session_revision=?,session_seal=?`,
				p.Ticket, p.SessionTicket, p.SessionAttempt, p.SessionRevision, p.SessionSeal[:])
			f.read(t, func(r CJSIReader) {
				if _, err := r.List(t.Context()); !errors.Is(err, store.ErrCJSIIndexChanged) {
					t.Fatalf("old ticket: %v", err)
				}
			})
		})
	}
}

func TestCJSIIndexCompactionRefusal(t *testing.T) {
	f := newCompactFixture(t, true, true, "complete")
	before := f.observe(t)
	for _, name := range []string{"retired global ticket", "wrong attempt", "wrong base seal", "wrong device", "late revoke", "retired selected seal", "damaged candidate postings",
		"changed local protection", "unexpected revision increment", "unexpected coverage transition", "late SQLite failure", "postimage changed", "competing sealed base reference"} {
		t.Run(name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			h := f.selected
			current := f.current
			var fixtureSQL string
			switch name {
			case "retired global ticket":
				h.Ticket--
			case "wrong attempt":
				h.SessionAttempt++
			case "wrong base seal":
				h.BaseSeal[0] ^= 1
			case "wrong device":
				h.Device++
			case "late revoke":
				calls := 0
				current = func(context.Context) (store.CJSIIndexStamp, error) {
					calls++
					s := h
					if calls == 2 {
						s.CoverageRevision++
					}
					return s, nil
				}
			case "retired selected seal":
				if _, err := tx.Exec("DELETE FROM cjsi_session_candidates WHERE ticket=? AND attempt=?", h.SessionTicket, h.SessionAttempt); err != nil {
					t.Fatal(err)
				}
			case "damaged candidate postings":
				fixtureSQL = `INSERT INTO cjsi_candidate_documents(cjsi_candidate_documents,rowid,event_at,kind,text)
					SELECT 'delete',doc_id,event_at,kind,text FROM cjsi_candidate_payloads WHERE doc_id=8`
			case "competing sealed base reference":
				// A different immutable attempt still owns a base FK to row 1.
				// Compaction cannot retire it on behalf of a lifecycle owner.
				fixtureSQL = `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,
					base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
					SELECT ticket,attempt+5,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,
					protection_revision,coverage_revision,session_revision FROM cjsi_session_candidates;
					INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,sequence_id)
					SELECT ticket,attempt+5,source_path,client,session_id,'patch',project,model,parser_version,first_at,last_at,sequence_id FROM cjsi_candidate_groups WHERE op='replace';
					INSERT INTO cjsi_candidate_order(ticket,attempt,source_path,client,session_id,doc_id,doc_side,detail_ordinal)
					SELECT ticket,attempt,source_path,client,session_id,1,'base',1 FROM cjsi_candidate_groups WHERE op='patch';
					UPDATE cjsi_session_candidates SET seal_digest=randomblob(32) WHERE attempt=7`
			case "changed local protection":
				if _, err := tx.Exec("INSERT INTO cjsi_session_protections VALUES(?,'codex',?,2)", f.physicalFixture.path, adapterThread); err != nil {
					t.Fatal(err)
				}
			case "unexpected revision increment":
				fixtureSQL = `CREATE TEMP TRIGGER extra_revision AFTER DELETE ON main.session_sources BEGIN UPDATE serving_generation SET revision=revision+1; END`
			case "unexpected coverage transition":
				fixtureSQL = `CREATE TEMP TRIGGER different_coverage AFTER DELETE ON main.session_sources BEGIN UPDATE inventory_runs SET state='partial' WHERE inventory_id='active'; END`
			case "late SQLite failure":
				fixtureSQL = `CREATE TEMP TRIGGER reject_transfer BEFORE UPDATE OF compacted_ticket ON main.cjsi_index_state
					WHEN (SELECT revision FROM serving_generation)=NEW.base_revision AND (SELECT state FROM inventory_runs WHERE inventory_id='active')='complete'
					BEGIN SELECT RAISE(ABORT,'fixture late compact transfer'); END`
			case "postimage changed":
				fixtureSQL = `CREATE TEMP TRIGGER corrupt_order AFTER UPDATE OF compacted_ticket ON main.cjsi_index_state
					BEGIN UPDATE cjsi_base_order SET detail_ordinal=17 WHERE doc_id=2; END`
			}
			if fixtureSQL != "" {
				if _, err := tx.Exec(fixtureSQL); err != nil {
					t.Fatal(err)
				}
			}
			if name == "competing sealed base reference" {
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				tx = stageTx(t, f.physicalFixture)
			}
			err := compactIndexCandidate(t.Context(), tx, f.plan.Attempt, h, current)
			if err == nil {
				t.Fatal("accepted invalid compaction")
			}
			if name == "competing sealed base reference" {
				var se *sqlite.Error
				if !errors.As(err, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
					t.Fatalf("expected foreign-key refusal, got %v", err)
				}
				stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_session_candidates WHERE attempt=7 AND seal_digest IS NOT NULL")
				stageScalar(t, f.private, 1, "SELECT count(*) FROM cjsi_candidate_order WHERE attempt=7 AND doc_id=1")
			}
			if name == "damaged candidate postings" {
				var se *sqlite.Error
				if !errors.As(err, &se) || se.Code()&0xff != sqlite3.SQLITE_CORRUPT {
					t.Fatalf("expected FTS integrity failure, got %v", err)
				}
			}
			if name == "late SQLite failure" {
				var se *sqlite.Error
				if !errors.As(err, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_TRIGGER || !strings.Contains(err.Error(), "fixture late compact transfer") {
					t.Fatalf("unexpected failure: %v", err)
				}
			}
			if !errors.Is(tx.Commit(), sql.ErrTxDone) {
				t.Fatal("partial compaction remained committable")
			}
			stageScalar(t, f.private, int64(0), "SELECT compacted_ticket FROM cjsi_index_state")
			stageScalar(t, f.private, h.BaseRevision, "SELECT revision FROM serving_generation")
			stageScalar(t, f.private, 8, "SELECT doc_id_high_water FROM cjsi_index_state")
			stageScalar(t, f.private, "complete", "SELECT state FROM inventory_runs WHERE inventory_id='active'")
			stageScalar(t, f.private, 0, "SELECT count(*) FROM session_documents WHERE source_path=?", f.path)
			if got := f.observe(t); !reflect.DeepEqual(got, before) {
				t.Fatalf("sealed selection changed: %+v", got)
			}
		})
	}
}

func TestCJSIIndexCompactionGroupTombstone(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, false)
	c.Replacements = nil
	c.DeletedSources = nil
	c.DeletedGroups = c.DeletedGroups[:1]
	a := c.Attempt
	tx := stageTx(t, f)
	if err := stageIndexCandidate(t.Context(), tx, a, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=?,session_revision=?,session_seal=?`,
		a.Ticket, a.Ticket, a.Attempt, a.SessionRevision, a.Seal[:])
	inspect := func() store.CJSIIndexStamp {
		t.Helper()
		var h store.CJSIIndexStamp
		f.read(t, func(r CJSIReader) {
			d, err := r.SelectDetail(t.Context(), "codex", adapterThread)
			if err != nil {
				t.Fatal(err)
			}
			h = d.identity.Stamp
			docs, p, err := r.DocumentsPage(t.Context(), d, 1, 10, true)
			if err != nil || p.Total != 3 || docLabels(docs) != "ABC" {
				t.Fatalf("sibling: %v %+v %v", docs, p, err)
			}
			rows, err := r.Search(t.Context(), "maskword OR movedword")
			if err != nil || docLabels(rows) != "movedword" {
				t.Fatalf("tombstone: %v %v", rows, err)
			}
		})
		return h
	}
	h := inspect()
	tx = stageTx(t, f)
	if err := compactIndexCandidate(t.Context(), tx, a, h, func(context.Context) (store.CJSIIndexStamp, error) { return h, nil }); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if inspect() != h {
		t.Fatal("compaction changed selection")
	}
	stageScalar(t, f.private, 0, "SELECT count(*) FROM main.domain_coverage_heads")
	stageScalar(t, f.private, 7, "SELECT doc_id_high_water FROM main.cjsi_index_state")
	stageScalar(t, f.private, 0, "SELECT count(*) FROM session_documents WHERE session_documents MATCH 'maskword'")
	stageScalar(t, f.private, 0, "SELECT count(*) FROM session_documents WHERE source_path=? AND client=? AND session_id=?", c.DeletedGroups[0].Path, c.DeletedGroups[0].Client, c.DeletedGroups[0].SessionID)

}

func TestCJSIIndexCompactionSparseRetainedID(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, true)
	c := stageFixturePlan(t, f, f.path, true)

	// The allocator can retain a large high-water mark after old documents
	// have been removed. The group still contains only a few detail positions.
	const retainedID = math.MaxInt64 - 1
	tx := stageTx(t, f)
	_, highWater, err := store.ReserveCJSIDocumentIDs(ctx, tx, retainedID-7)
	if err != nil || highWater != retainedID {
		t.Fatalf("document reservation: high water %d, error %v", highWater, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
		SELECT ?,source_path,client,session_id,event_at,kind,text FROM session_documents WHERE rowid=1;
		DELETE FROM session_documents WHERE rowid=1;
		UPDATE cjsi_base_order SET doc_id=? WHERE doc_id=1`, retainedID, retainedID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	c.Replacements[0].DocumentIDs = []int64{retainedID, math.MaxInt64}
	tx = stageTx(t, f)
	if err := stageIndexCandidate(ctx, tx, c.Attempt, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	a := c.Attempt
	adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=?,session_revision=?,session_seal=?`,
		a.Ticket+1, a.Ticket, a.Attempt, a.SessionRevision, a.Seal[:])

	fixture := compactFixture{physicalFixture: f, plan: c, path: f.path}
	before := fixture.observe(t)
	if !reflect.DeepEqual(before.ids, c.Replacements[0].DocumentIDs) ||
		!reflect.DeepEqual(before.ordinals, []int64{1, 2}) || before.page.Total != 2 ||
		len(before.hits["text:A OR text:stageword"]) != 2 {
		t.Fatalf("invalid selected index: %+v", before)
	}

	fixture.selected = before.detail.identity.Stamp
	tx = stageTx(t, f)
	if err := compactIndexCandidate(ctx, tx, a, fixture.selected, fixture.current); err != nil {
		t.Fatalf("compacting IDs %v at ordinals %v: %v", before.ids, before.ordinals, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if after := fixture.observe(t); !reflect.DeepEqual(after, before) {
		t.Fatalf("compaction changed the selected index: before %+v after %+v", before, after)
	}
	stageScalar(t, f.private, int64(math.MaxInt64), "SELECT doc_id_high_water FROM cjsi_index_state")
	stageScalar(t, f.private, a.Ticket, "SELECT compacted_ticket FROM cjsi_index_state")
}

func compactRows(rows *sql.Rows) ([][]any, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var values [][]any
	targets := make([]any, len(cols))
	for rows.Next() {
		v := make([]any, len(cols))
		for i := range v {
			targets[i] = &v[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func TestCJSICompactionImageUsesTypedValues(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	digest := func(query string) [32]byte {
		t.Helper()
		rows, err := db.QueryContext(t.Context(), query)
		if err != nil {
			t.Fatal(err)
		}
		sum, err := digestIndexRows(rows)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	seen := map[[32]byte]string{}
	for _, q := range []string{"SELECT NULL", "SELECT ''", "SELECT x''", "SELECT 0", "SELECT '0'", "SELECT x'30'", "SELECT 'ab','c'", "SELECT 'a','bc'", "SELECT 1 UNION ALL SELECT 2", "SELECT 2 UNION ALL SELECT 1"} {
		key := digest(q)
		if other, ok := seen[key]; ok {
			t.Fatalf("same compaction image for %s and %s", other, q)
		}
		seen[key] = q
	}
	rows, err := db.QueryContext(t.Context(), "SELECT 1.25")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := digestIndexRows(rows); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("unsupported SQLite storage class: %v", err)
	}
	rows, err = db.QueryContext(t.Context(), "SELECT 'closed'")
	if err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := digestIndexRows(rows); err == nil {
		t.Fatal("closed rows produced a compaction digest")
	}
}

func TestCJSICompactAuthenticatesWholeSelection(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	type check func(*indexStageAttempt, *store.CJSIIndexStamp)
	for _, tc := range []struct {
		name  string
		alter check
	}{
		{"global ticket", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.Ticket = h.SessionTicket - 1 }},
		{"retired parent", func(a *indexStageAttempt, h *store.CJSIIndexStamp) { a.Parent.Ticket = h.SessionTicket }},
		{"ticket", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Ticket++ }},
		{"attempt", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Attempt++ }},
		{"session revision", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.SessionRevision++ }},
		{"session seal", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Seal[0] ^= 1 }},
		{"absent seal", func(a *indexStageAttempt, h *store.CJSIIndexStamp) { a.Seal = [32]byte{}; h.SessionSeal = a.Seal }},
		{"index epoch", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Parent.IndexEpoch++ }},
		{"core epoch", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Parent.CoreEpoch++ }},
		{"device", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Parent.Device++ }},
		{"inode", func(a *indexStageAttempt, _ *store.CJSIIndexStamp) { a.Parent.Inode++ }},
		{"absent inode", func(a *indexStageAttempt, h *store.CJSIIndexStamp) { a.Parent.Inode = 0; h.Inode = 0 }},
		{"base ticket", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.BaseTicket++ }},
		{"base attempt", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.BaseAttempt++ }},
		{"base seal", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.BaseSeal[0] ^= 1 }},
		{"base revision", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.BaseRevision++ }},
		{"protection revision", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.ProtectionRevision++ }},
		{"coverage revision", func(_ *indexStageAttempt, h *store.CJSIIndexStamp) { h.CoverageRevision++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, h := f.plan.Attempt, f.selected
			tc.alter(&a, &h)
			tx := stageTx(t, f.physicalFixture)
			err := compactIndexCandidate(ctx, tx, a, h, func(context.Context) (store.CJSIIndexStamp, error) { return h, nil })
			if !errors.Is(err, store.ErrCJSIIndexChanged) {
				tx.Rollback()
				t.Fatalf("selection mismatch: %v", err)
			}
			if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("rejected transaction committed: %v", err)
			}
		})
	}
	if err := compactIndexCandidate(ctx, nil, f.plan.Attempt, f.selected, f.current); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("nil transaction: %v", err)
	}
	if err := compactIndexCandidate(ctx, stageTx(t, f.physicalFixture), f.plan.Attempt, f.selected, nil); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("unbound selection: %v", err)
	}
	lookupErr := errors.New("selection unavailable")
	if err := compactIndexCandidate(ctx, stageTx(t, f.physicalFixture), f.plan.Attempt, f.selected, func(context.Context) (store.CJSIIndexStamp, error) { return store.CJSIIndexStamp{}, lookupErr }); !errors.Is(err, lookupErr) {
		t.Fatalf("lookup failure: %v", err)
	}
}

// checkProducer consumes five selected relations. Copy the sealed relations
// into temporary tables so each subtest can vary one input, without weakening
// the Store schema or editing sealed rows. All temporary changes roll back.
func TestCJSICompactionProducerRelations(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, true, "complete")
	tables := []struct{ alias, relation string }{{"cs", "sources"}, {"cg", "groups"}, {"co", "order"}, {"cr", "ranges"}, {"cm", "doc_masks"}}
	setSource := "UPDATE temp.cs SET "
	upsert := " WHERE op='upsert'"
	setGroup := "UPDATE temp.cg SET "
	replace := " WHERE op='replace'"
	for _, tc := range []struct {
		name, sql string
		wantOK    bool
	}{
		{"same value different exclusion kind", `UPDATE temp.cg SET project='/proposed' WHERE op='replace';INSERT INTO session_exclusions VALUES('session','/proposed')`, true},
		{"canonical", "", true},
		{"empty producer", "DELETE FROM temp.cs; DELETE FROM temp.cg; DELETE FROM temp.co; DELETE FROM temp.cr; DELETE FROM temp.cm", false},
		{"source deleted while native replacement remains", setSource + "op='delete'" + upsert, false},
		{"OTel source with native replay", setSource + "source_kind='otel'" + upsert, false},
		{"payload not eligible", `INSERT INTO main.cjsi_candidate_payloads(doc_id,event_at,kind,text) VALUES(9,'','unapproved','hidden');
			UPDATE temp.co SET doc_id=9 WHERE doc_side='candidate'`, false},
		{"retained id has another payload owner", `UPDATE main.session_documents SET source_path='another owner' WHERE rowid=2`, false},
		{"retained id lost its physical row", `DELETE FROM main.session_documents WHERE rowid=2`, false},
		{"reoccupied origin retains same identity", `UPDATE temp.cs SET op='upsert',source_kind='codex-jsonl',identity=(SELECT identity FROM main.session_sources WHERE source_path=temp.cs.source_path) WHERE source_path=(SELECT source_path FROM main.cjsi_base_order WHERE doc_id=2)`, false},
		{"unsupported source kind", setSource + "source_kind='synthetic'" + upsert, false},
		{"missing group", "DELETE FROM temp.cg WHERE op='replace'", false},
		{"patch group", setGroup + "op='patch'" + replace, false},
		{"foreign native group", setGroup + "client='claude'" + replace, false},
		{"unverified native group", setGroup + "order_verified=0" + replace, false},
		{"other group parser", setGroup + "parser_version=5" + replace, false},
		{"absent native range", "DELETE FROM temp.cr", false},
		{"range belongs to another source", "UPDATE temp.cr SET source_path='other'", false},
		{"range belongs to another client", "UPDATE temp.cr SET client='claude'", false},
		{"range belongs to another session", "UPDATE temp.cr SET session_id='other'", false},
		{"wrong capture identity", "UPDATE temp.cr SET identity='other'", false},
		{"shorter publication", "UPDATE temp.cr SET published_end=complete_end-1", false},
		{"empty capture", "DELETE FROM temp.co; UPDATE temp.cr SET read_end=0,complete_end=0,published_end=0; UPDATE temp.cs SET complete_end=0 WHERE op='upsert'", false},
		{"source boundary mismatch", setSource + "complete_end=complete_end-1" + upsert, false},
		{"read beyond source", "UPDATE temp.cr SET read_end=read_end+1", false},
		{"source and group parsers differ", setSource + "parser_version=5" + upsert, false},
		{"ordinal gap", "UPDATE temp.co SET detail_ordinal=9 WHERE detail_ordinal=2", false},
		{"unproved position", "UPDATE temp.co SET record_start=NULL WHERE detail_ordinal=1", false},
		{"unrecognized emission", "UPDATE temp.co SET emission=1 WHERE detail_ordinal=1", false},
		{"overlapping positions", "UPDATE temp.co SET record_start=0 WHERE detail_ordinal=2", false},
		{"member exceeds range", "UPDATE temp.co SET record_end=(SELECT published_end+1 FROM temp.cr) WHERE detail_ordinal=2", false},
		{"absent candidate payload", "UPDATE temp.co SET doc_id=7 WHERE doc_side='candidate'", false},
		{"unregistered base member", "UPDATE temp.co SET doc_id=99 WHERE doc_side='base'", false},
		{"candidate id aliases base", "UPDATE temp.co SET doc_id=1 WHERE doc_side='candidate'", false},
		{"move without old tombstone", "DELETE FROM temp.cs WHERE source_path=(SELECT source_path FROM main.cjsi_base_order WHERE doc_id=2)", false},
		{"correct identity exists at another base path", `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
			SELECT 'another-physical-owner',identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM main.session_sources WHERE source_path=(SELECT source_path FROM main.cjsi_base_order WHERE doc_id=2);
			UPDATE main.session_sources SET identity='different-original' WHERE source_path=(SELECT source_path FROM main.cjsi_base_order WHERE doc_id=2)`, false},
		{"move between identities", setSource + "identity='other'" + upsert + "; UPDATE temp.cr SET identity='other'", false},
		{"mask for an unrelated source", `INSERT INTO cg(source_path,client,session_id,op) SELECT 'replica',client,session_id,'delete' FROM cg WHERE op='replace'; UPDATE cm SET source_path='replica',doc_id=2`, true},
		{"mask for another client", `INSERT INTO cg(source_path,client,session_id,op) SELECT source_path,'claude',session_id,'delete' FROM cg WHERE op='replace'; UPDATE cm SET client='claude',doc_id=2`, true},
		{"mask for another session", `INSERT INTO cg(source_path,client,session_id,op) SELECT source_path,client,'retired','delete' FROM cg WHERE op='replace'; UPDATE cm SET session_id='retired',doc_id=2`, true},
		{"masked retained member", "UPDATE temp.cm SET doc_id=2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			defer tx.Rollback()
			for _, table := range tables {
				_, err := tx.ExecContext(ctx, "CREATE TEMP TABLE "+table.alias+" AS SELECT * FROM main.cjsi_candidate_"+table.relation+" WHERE ticket=? AND attempt=?", f.selected.SessionTicket, f.selected.SessionAttempt)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.sql != "" {
				if _, err := tx.ExecContext(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			compaction := indexCompaction{ctx: ctx, tx: tx}
			err := compaction.checkProducer(9)
			if tc.wantOK && err != nil || !tc.wantOK && !errors.Is(err, store.ErrCJSIIndexChanged) {
				t.Fatalf("producer error=%v, want accepted=%v", err, tc.wantOK)
			}
		})
	}
}

func TestCJSICompactionRequiresEmptyOTelLayout(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	projections := []TelemetrySession{{Client: "claude", SessionID: "settled", FirstAt: "first", LastAt: "last"}}
	var c *CJSIIndexCandidate
	var parent store.CJSIIndexStamp
	f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
		parent = s.Selection()
		var err error
		c, err = PlanCJSIIndex(ctx, s, 1, 1, parent.SessionRevision+1, nil, nil, &projections)
		return err
	})
	tx := stageTx(t, f)
	if c == nil {
		t.Fatal("empty OTel plan")
	}
	if err := c.Stage(ctx, tx, parent, 1, 1, parent.SessionRevision+1, c.Seal()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, change string }{
		{"canonical", ""},
		{"different source and group", "UPDATE cs SET source_path='otel:claude/another',identity='otel:claude/another'"},
		{"source deletion", "UPDATE cs SET op='delete'"},
		{"other source kind", "UPDATE cs SET source_kind='codex-jsonl'"},
		{"other identity", "UPDATE cs SET identity='other'"},
		{"path and logical group disagree", "UPDATE cg SET session_id='other'"},
		{"unknown order", "UPDATE cg SET order_verified=0"},
		{"nonempty source", "UPDATE cs SET size=9"},
		{"nonempty checkpoint", "UPDATE cs SET complete_end=1"},
		{"different parser versions", "UPDATE cg SET parser_version=parser_version+1"},
		{"native replay range", "INSERT INTO cr(source_path,client,session_id) SELECT source_path,client,session_id FROM cg"},
		{"document in an empty group", "INSERT INTO co(source_path,client,session_id,doc_id,doc_side,detail_ordinal,emission) SELECT source_path,client,session_id,1,'candidate',1,0 FROM cg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f)
			defer tx.Rollback()
			for _, rel := range []struct{ alias, table string }{{"cs", "sources"}, {"cg", "groups"}, {"co", "order"}, {"cr", "ranges"}, {"cm", "doc_masks"}} {
				if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE "+rel.alias+" AS SELECT * FROM main.cjsi_candidate_"+rel.table+" WHERE ticket=1 AND attempt=1"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.change != "" {
				if _, err := tx.ExecContext(ctx, tc.change); err != nil {
					t.Fatal(err)
				}
			}
			compaction := indexCompaction{ctx: ctx, tx: tx}
			err := compaction.checkProducer(0)
			if tc.change == "" && err != nil || tc.change != "" && !errors.Is(err, store.ErrCJSIIndexChanged) {
				t.Fatalf("OTel layout: %v", err)
			}
		})
	}
}

func TestCJSICompactionBindsPhysicalBaseBeforeWriting(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	for _, tc := range []struct{ name, change string }{
		{"deferred foreign keys", "PRAGMA defer_foreign_keys=ON"},
		{"serving revision diverged", "UPDATE serving_generation SET revision=revision+1"},
		{"base receipt changed", "UPDATE cjsi_index_state SET compacted_seal=randomblob(32)"},
		{"layout receipt changed", "UPDATE cjsi_layout_initial_seal SET seal_digest=randomblob(32)"},
		{"entire base receipt differs", "UPDATE cjsi_index_state SET compacted_seal=randomblob(32); UPDATE cjsi_layout_initial_seal SET seal_digest=(SELECT compacted_seal FROM cjsi_index_state)"},
		{"physical epoch advanced", "UPDATE session_index_generation SET epoch=epoch+1; UPDATE cjsi_layout_initial_seal SET index_epoch=(SELECT epoch FROM session_index_generation)"},
		{"layout epoch changed", "UPDATE cjsi_layout_initial_seal SET index_epoch=index_epoch+1"},
		{"protection changed", "UPDATE cjsi_index_state SET protection_revision=protection_revision+1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			if _, err := tx.ExecContext(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, `CREATE TEMP TRIGGER wrong_base_write BEFORE INSERT ON main.session_sources BEGIN SELECT RAISE(ABORT,'base write before admission'); END`); err != nil {
				t.Fatal(err)
			}
			if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("physical base mismatch must fail admission, got %v", err)
			}
			if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("rejected writer committed: %v", err)
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
	if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign keys disabled: %v", err)
	}
}

func TestCJSICompactionScopesCoverageToSelectedAttempt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected bool
	}{{"separate attempt", false}, {"selected attempt", true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			f := newPhysicalFixture(t, true, true)
			c := stageFixturePlan(t, f, f.path, false)
			parent := c.Attempt.Parent
			// Stage both SQLite drafts against the same physical base. An
			// independent coverage producer adds its row before the seal.
			for _, attempt := range []int64{2, 3} {
				c.Attempt.Attempt = attempt
				c.Attempt.Seal = sha256.Sum256([]byte(fmt.Sprintf("coverage attempt %d", attempt)))
				tx := stageTx(t, f)
				if (attempt == 2) == tc.selected {
					_, err := tx.ExecContext(ctx, `CREATE TEMP TRIGGER coverage_before_seal BEFORE UPDATE OF seal_digest ON main.cjsi_session_candidates BEGIN
						INSERT INTO cjsi_candidate_coverage(ticket,attempt,inventory_id,source_path,identity,captured_end,committed_end,state)
						SELECT NEW.ticket,NEW.attempt,'inventory',source_path,identity,size,complete_end,'complete' FROM main.cjsi_candidate_sources
						WHERE ticket=NEW.ticket AND attempt=NEW.attempt AND op='upsert'; END`)
					if err != nil {
						t.Fatal(err)
					}
				}
				if err := stageIndexCandidate(ctx, tx, c.Attempt, c); err != nil {
					t.Fatal(err)
				}
				if _, err := tx.ExecContext(ctx, "DROP TRIGGER IF EXISTS temp.coverage_before_seal"); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			}
			seal := sha256.Sum256([]byte("coverage attempt 2"))
			adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=2,session_revision=?,session_seal=?`, parent.SessionRevision+1, seal[:])
			var selected store.CJSIIndexStamp
			f.snapshot(t, func(s store.CJSIIndexSnapshot) error { selected = s.Selection(); return nil })
			tx := stageTx(t, f)
			err := CompactCJSIIndex(ctx, tx, parent, selected, func(context.Context) (store.CJSIIndexStamp, error) { return selected, nil })
			if tc.selected {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("compaction discarded a selected coverage delta: %v", err)
				}
				if e := tx.Commit(); !errors.Is(e, sql.ErrTxDone) {
					t.Fatalf("failed compaction could commit: %v", e)
				}
			} else {
				if err != nil {
					t.Fatalf("unselected coverage blocked compaction: %v", err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCJSICompactRechecksMoveProtectionScope(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, true, "complete")
	origin, dest := f.physicalFixture.path, f.path
	// A previously unselected replica remains excluded. Its project cannot
	// veto a move of the safe winning source.
	adapterExec(t, f.private, "UPDATE session_metadata SET project='/unrelated' WHERE source_path='excluded'")
	tests := []struct {
		name, path, client, id, kind, value string
		doc                                 int64
		deny                                bool
	}{
		{name: "origin whole source", path: origin, deny: true},
		{name: "origin retained id", path: origin, client: "codex", id: adapterThread, doc: 2, deny: true},
		{name: "origin other document", path: origin, client: "codex", id: adapterThread, doc: 1},
		{name: "origin other client", path: origin, client: "claude", id: adapterThread, doc: 2},
		{name: "origin other session", path: origin, client: "codex", id: "other", doc: 2},
		{name: "destination whole source", path: dest, deny: true},
		{name: "destination logical group", path: dest, client: "codex", id: adapterThread, deny: true},
		{name: "destination other group", path: dest, client: "codex", id: "other"},
		{name: "destination other client", path: dest, client: "claude", id: adapterThread},
		{name: "unrelated source", path: "elsewhere", client: "codex", id: adapterThread, doc: 2},
		{name: "retired sibling group", path: origin, client: "codex", id: "masked"},
		{name: "session named like destination", kind: "session", value: dest},
		{name: "project named like origin", kind: "project", value: origin},
		{name: "session named like client", kind: "session", value: "codex"},
		{name: "project named like session", kind: "project", value: adapterThread},
		{name: "path named like original project", kind: "path", value: "project"},
		{name: "destination path exclusion", kind: "path", value: dest, deny: true},
		{name: "origin path exclusion", kind: "path", value: origin, deny: true},
		{name: "other path exclusion", kind: "path", value: "another"},
		{name: "logical client exclusion", kind: "client", value: "codex", deny: true},
		{name: "other client exclusion", kind: "client", value: "claude"},
		{name: "logical session exclusion", kind: "session", value: adapterThread, deny: true},
		{name: "other session exclusion", kind: "session", value: "other"},
		{name: "origin project exclusion", kind: "project", value: "project", deny: true},
		{name: "candidate project exclusion", kind: "project", value: f.plan.Replacements[0].Metadata.Project, deny: true},
		{name: "unrelated project exclusion", kind: "project", value: "/unrelated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			defer tx.Rollback()
			if tc.path != "" {
				if _, err := tx.ExecContext(ctx, "INSERT INTO cjsi_session_protections VALUES(?,?,?,?)", tc.path, tc.client, tc.id, tc.doc); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := tx.ExecContext(ctx, "INSERT INTO session_exclusions VALUES(?,?)", tc.kind, tc.value); err != nil {
					t.Fatal(err)
				}
			}
			side := fmt.Sprintf(" WHERE ticket=%d AND attempt=%d", f.selected.SessionTicket, f.selected.SessionAttempt)
			compaction := indexCompaction{ctx: ctx, tx: tx, with: "WITH cs AS (SELECT * FROM main.cjsi_candidate_sources" + side + "),cg AS (SELECT * FROM main.cjsi_candidate_groups" + side + "),co AS (SELECT * FROM main.cjsi_candidate_order" + side + "),cr AS (SELECT * FROM main.cjsi_candidate_ranges" + side + "),cm AS (SELECT * FROM main.cjsi_candidate_doc_masks" + side + ") "}
			err := compaction.checkProducer(8)
			if tc.deny && !errors.Is(err, store.ErrCJSIIndexChanged) || !tc.deny && err != nil {
				t.Fatalf("protection deny=%v err=%v", tc.deny, err)
			}
		})
	}
}

func TestCJSICompactPreservesUnrelatedPhysicalRows(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, true, "complete", func(db *sql.DB) {
		adapterExec(t, db, "UPDATE session_sources SET size=2,cursor=2,partial_line=x'61' WHERE source_path='excluded'")
	})
	adapterExec(t, f.private, "UPDATE inventory_runs SET state='update_pending' WHERE inventory_id='other'")
	capture := func() [][][]any {
		t.Helper()
		var image [][][]any
		for _, table := range []string{"session_sources", "session_metadata", "cjsi_base_groups", "cjsi_base_order", "cjsi_base_ranges", "session_documents", "cjsi_legacy_cursor_hints"} {
			rows, err := f.private.QueryContext(ctx, "SELECT * FROM "+table+" WHERE source_path='excluded' ORDER BY 1,2")
			if err != nil {
				t.Fatal(err)
			}
			data, err := compactRows(rows)
			if err != nil {
				t.Fatal(err)
			}
			image = append(image, data)
		}
		return image
	}
	before := capture()
	if len(before[3]) != 1 || len(before[5]) != 1 || len(before[6]) != 1 {
		t.Fatal("missing hidden order or FTS fixture")
	}
	tx := stageTx(t, f.physicalFixture)
	if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(capture(), before) {
		t.Fatal("compaction changed hidden members of an unrelated source")
	}
	stageScalar(t, f.private, "update_pending", "SELECT state FROM inventory_runs WHERE inventory_id='other'")
}

func TestCJSICompactRejectsLateCoverageOrWaterChange(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	for _, tc := range []struct {
		name, trigger string
		want          error
	}{
		{"unexpected coverage during merge", `CREATE TEMP TRIGGER unexpected_state AFTER DELETE ON main.session_sources BEGIN UPDATE inventory_runs SET state='partial' WHERE inventory_id='active'; END`, store.ErrCJSIIndexChanged},
		{"coverage changed during transfer", `CREATE TEMP TRIGGER coverage_after_transfer AFTER UPDATE OF compacted_ticket ON main.cjsi_index_state BEGIN UPDATE inventory_runs SET state='failed' WHERE inventory_id='active'; END`, store.ErrCJSIIndexChanged},
		{"serving revision changed at normalization", `CREATE TEMP TRIGGER changed_normalization AFTER UPDATE OF revision ON main.serving_generation WHEN NEW.revision<OLD.revision BEGIN UPDATE serving_generation SET revision=revision+1; END`, errIndexStage},
		{"allocator changed before transfer", `CREATE TEMP TRIGGER allocator_during_merge AFTER UPDATE ON main.session_metadata BEGIN UPDATE cjsi_index_state SET doc_id_high_water=doc_id_high_water+1; END`, errIndexStage},
		{"serving revision changed after transfer", `CREATE TEMP TRIGGER late_revision AFTER UPDATE OF compacted_ticket ON main.cjsi_index_state BEGIN UPDATE serving_generation SET revision=revision+1; END`, sql.ErrNoRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			if _, err := tx.ExecContext(ctx, tc.trigger); err != nil {
				t.Fatal(err)
			}
			err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current)
			if !errors.Is(err, tc.want) {
				tx.Rollback()
				t.Fatalf("late mutation: got %v want %v", err, tc.want)
			}
			if e := tx.Commit(); !errors.Is(e, sql.ErrTxDone) {
				t.Fatalf("refused transfer committed: %v", e)
			}
			stageScalar(t, f.private, int64(0), "SELECT compacted_ticket FROM cjsi_index_state")
			stageScalar(t, f.private, "complete", "SELECT state FROM inventory_runs WHERE inventory_id='active'")
		})
	}
}

func TestCJSICompactionCannotBorrowAnotherSourcesProof(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, false, false, "complete")
	for _, tc := range []struct {
		name, change string
		valid        bool
	}{
		{"two complete native copies", "", true},
		{"first copy lacks its own range", "DELETE FROM cr WHERE source_path<>'second' ", false},
		{"other alias has the only range", `UPDATE cs SET identity='shared' WHERE op='upsert'; UPDATE cr SET identity='shared'; DELETE FROM cr WHERE source_path<>'second'`, false},
		{"deleting same logical group at other source", `INSERT INTO cg(source_path,client,session_id,op) SELECT 'old-copy',client,session_id,'delete' FROM cg WHERE op='replace' AND source_path='second'`, true},
		{"deleting another client at same source", `INSERT INTO cg(source_path,client,session_id,op) SELECT source_path,'claude',session_id,'delete' FROM cg WHERE op='replace' AND source_path='second'`, true},
		{"missing sibling tombstone", "DELETE FROM cg WHERE session_id='old-group' AND op='delete'", false},
		{"OTel eligibility at another path", `DELETE FROM cr;
			UPDATE cg SET parser_version=6 WHERE op='replace';
			UPDATE cs SET parser_version=6 WHERE op='upsert';
			UPDATE cg SET source_path='otel:codex/'||session_id WHERE source_path='second';
			UPDATE cs SET source_path=(SELECT source_path FROM cg WHERE source_path LIKE 'otel:%'), identity=(SELECT source_path FROM cg WHERE source_path LIKE 'otel:%'),source_kind='otel',size=0,complete_end=0 WHERE source_path='second'`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := stageTx(t, f.physicalFixture)
			defer tx.Rollback()
			for _, table := range []struct{ alias, relation string }{{"cs", "sources"}, {"cg", "groups"}, {"co", "order"}, {"cr", "ranges"}, {"cm", "doc_masks"}} {
				if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE "+table.alias+" AS SELECT * FROM main.cjsi_candidate_"+table.relation+" WHERE ticket=? AND attempt=?", f.selected.SessionTicket, f.selected.SessionAttempt); err != nil {
					t.Fatal(err)
				}
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO cs(source_path,op,source_kind,identity,parser_version,size,complete_end) SELECT 'second',op,source_kind,'second-identity',parser_version,size,complete_end FROM cs WHERE op='upsert';
			INSERT INTO cg(source_path,op,client,session_id,order_verified,parser_version,project) SELECT 'second',op,client,session_id,order_verified,parser_version,project FROM cg WHERE op='replace';
			INSERT INTO cr(source_path,client,session_id,identity,read_end,complete_end,published_end) SELECT 'second',client,session_id,'second-identity',read_end,complete_end,published_end FROM cr`)
			if err != nil {
				t.Fatal(err)
			}
			if tc.change != "" {
				if _, err := tx.ExecContext(ctx, tc.change); err != nil {
					t.Fatal(err)
				}
			}
			compaction := indexCompaction{ctx: ctx, tx: tx}
			err = compaction.checkProducer(7)
			if tc.valid && err != nil || !tc.valid && !errors.Is(err, store.ErrCJSIIndexChanged) {
				t.Fatalf("producer valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestCJSICompactionRequiresConnectedCoverageHead(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	// Read an inconsistent prior state through a real SQLite connection.
	// Absence of a head is permitted, but a head naming a lost run is not.
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
	defer tx.Rollback()
	c := indexCompaction{ctx: ctx, tx: tx}
	empty, err := c.coverage()
	if err != nil || empty.id.Valid || empty.state.Valid {
		t.Fatalf("empty head: %+v %v", empty, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO domain_coverage_heads(singleton,inventory_id) VALUES(1,'missing')"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.coverage(); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("dangling coverage head: %v", err)
	}
}

func TestCJSICompactionRefusesFailedFirstSelection(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	calls := 0
	failure := errors.New("selection read failed")
	tx := stageTx(t, f.physicalFixture)
	err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, func(context.Context) (store.CJSIIndexStamp, error) {
		calls++
		if calls == 1 {
			return store.CJSIIndexStamp{}, failure
		}
		return f.selected, nil
	})
	if calls != 1 || !errors.Is(err, failure) {
		tx.Rollback()
		t.Fatalf("failed admission proceeded: calls=%d err=%v", calls, err)
	}
	if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("failed admission transaction remained committable: %v", err)
	}
}

func TestCJSICompactRehomesEachRetainedDocument(t *testing.T) {
	ctx := t.Context()
	f := newPhysicalFixture(t, true, false)
	ids := []string{adapterThread, "00000000-0000-4000-8000-000000000002"}
	paths := []string{filepath.Join(t.TempDir(), "one.jsonl"), filepath.Join(t.TempDir(), "two.jsonl")}
	expected := map[string]int64{}
	for generation := 1; generation <= 2; generation++ {
		var sources []CJSIPreparedSource
		for i, id := range ids {
			prior := ""
			if generation == 2 {
				prior = paths[i]
				paths[i] = filepath.Join(t.TempDir(), filepath.Base(prior))
				if err := os.Rename(prior, paths[i]); err != nil {
					t.Fatal(err)
				}
			}
			body := fmt.Sprintf("{\"type\":\"session_meta\",\"payload\":{\"id\":%q}}\n{\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":%q}}\n", id, "record "+id)
			src := fixturePreparedNative(t, paths[i], id, body)
			src.replacement.From = prior
			sources = append(sources, src)
		}
		var parent store.CJSIIndexStamp
		var c *CJSIIndexCandidate
		f.snapshot(t, func(s store.CJSIIndexSnapshot) error {
			parent = s.Selection()
			var err error
			c, err = PlanCJSIIndex(ctx, s, parent.Ticket+1, 1, parent.SessionRevision+1, sources, nil, nil)
			return err
		})
		if c == nil {
			t.Fatal("missing replacement")
		}
		for _, repl := range c.plan.Replacements {
			id := repl.Selected.ThreadID
			doc := repl.DocumentIDs
			if len(doc) != 1 {
				t.Fatal(doc)
			}
			if generation == 1 {
				expected[id] = doc[0]
			} else if doc[0] != expected[id] {
				t.Fatalf("moved %s acquired different ID %v", id, doc)
			}
		}
		ticket := parent.Ticket + 1
		revision := parent.SessionRevision + 1
		seal := c.Seal()
		tx := stageTx(t, f)
		if err := c.Stage(ctx, tx, parent, ticket, 1, revision, seal); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		adapterExec(t, f.core, `UPDATE cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=1,session_revision=?,session_seal=?,base_ticket=?,base_attempt=?,base_revision=?,base_seal=?`, ticket, ticket, revision, seal[:], parent.SessionTicket, parent.SessionAttempt, parent.SessionRevision, parent.SessionSeal[:])
		var selected store.CJSIIndexStamp
		f.snapshot(t, func(s store.CJSIIndexSnapshot) error { selected = s.Selection(); return nil })
		tx = stageTx(t, f)
		if err := CompactCJSIIndex(ctx, tx, parent, selected, func(context.Context) (store.CJSIIndexStamp, error) { return selected, nil }); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		for i, id := range ids {
			stageScalar(t, f.private, paths[i], "SELECT source_path FROM session_documents WHERE rowid=? AND client='codex' AND session_id=?", expected[id], id)
		}
	}
}

func testReoccupiedOriginProof(t *testing.T, f physicalFixture, h store.CJSIIndexStamp, origin, oldID, newID string) {
	t.Helper()
	ctx := t.Context()
	tx := stageTx(t, f)
	defer tx.Rollback()
	for _, rel := range []struct{ key, table string }{{"cs", "sources"}, {"cg", "groups"}, {"co", "order"}, {"cr", "ranges"}, {"cm", "doc_masks"}} {
		if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE "+rel.key+" AS SELECT * FROM main.cjsi_candidate_"+rel.table+" WHERE ticket=? AND attempt=?", h.SessionTicket, h.SessionAttempt); err != nil {
			t.Fatal(err)
		}
	}
	check := indexCompaction{ctx: ctx, tx: tx}
	var water int64
	if err := tx.QueryRow("SELECT doc_id_high_water FROM cjsi_index_state").Scan(&water); err != nil {
		t.Fatal(err)
	}
	if err := check.checkProducer(water); err != nil {
		t.Fatalf("valid replacement rejected: %v", err)
	}
	if oldID == newID {
		t.Fatal("test requires a different replacement inode")
	}
	// Make the proposed source and its range internally consistent, while
	// taking away the proof that this specific origin retired its old inode.
	for _, table := range []string{"cs", "cr"} {
		if _, err := tx.ExecContext(ctx, "UPDATE temp."+table+" SET identity=? WHERE source_path=?", oldID, origin); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
		SELECT 'unrelated-alias',identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM session_sources WHERE source_path=?`, origin); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO temp.cs(source_path,op) VALUES('unrelated-alias','delete')"); err != nil {
		t.Fatal(err)
	}
	if err := check.checkProducer(water); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Fatalf("unrelated alias authorized a retained ID transfer: %v", err)
	}
}

func TestCJSICompactDoesNotRewriteRetiredGroupEvidence(t *testing.T) {
	ctx := t.Context()
	f := newCompactFixture(t, true, false, "complete")
	tx := stageTx(t, f.physicalFixture)
	// Both sibling groups are tombstoned. Their old evidence is removed
	// by the metadata delete; it must not first acquire the replacement seal.
	_, err := tx.ExecContext(ctx, `CREATE TEMP TRIGGER retired_group_write BEFORE UPDATE ON main.cjsi_base_groups
		WHEN OLD.session_id IN ('masked','old-group')
		BEGIN SELECT RAISE(ABORT,'rewrote a retired group'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err := compactIndexCandidate(ctx, tx, f.plan.Attempt, f.selected, f.current); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	stageScalar(t, f.private, 0, "SELECT count(*) FROM cjsi_base_groups WHERE session_id IN ('masked','old-group')")
}

func TestCJSICoverageHeadSelectsExactRun(t *testing.T) {
	f := newPhysicalFixture(t, true, false)
	ctx := t.Context()
	tx := stageTx(t, f)
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO inventory_runs VALUES('00-unrelated',1,'old',10,'failed'),('latest',2,'new',1,'complete'); INSERT INTO domain_coverage_heads VALUES(1,'latest')`); err != nil {
		t.Fatal(err)
	}
	compaction := indexCompaction{ctx: ctx, tx: tx}
	c, err := compaction.coverage()
	if err != nil || c.id.String != "latest" || c.state.String != "complete" || c.epoch.Int64 != 2 || c.sourceCount.Int64 != 1 {
		t.Fatalf("wrong coverage run %+v err=%v", c, err)
	}
}
