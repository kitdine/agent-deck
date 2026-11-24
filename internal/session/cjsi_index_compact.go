package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/kitdine/agent-deck/internal/store"
)

// Task 8 keeps the actual root, selection and database identity pinned across
// this borrow and the owner's commit. current checks that admission (including
// the global ticket); it must not open a database or start another transaction.
// The registered writer attempt authenticates the full-replacement producer.
func compactIndexCandidate(ctx context.Context, tx *sql.Tx, a indexStageAttempt, selected store.CJSIIndexStamp,
	current func(context.Context) (store.CJSIIndexStamp, error)) (err error) {
	if tx == nil {
		return store.ErrCJSIIndexChanged
	}
	err = store.ErrCJSIIndexChanged
	defer func() {
		if err != nil {
			if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
				err = errors.Join(err, e)
			}
		}
	}()
	check := func() error {
		if current == nil {
			return store.ErrCJSIIndexChanged
		}
		h, e := current(ctx)
		if e != nil {
			return e
		}
		if h != selected {
			return store.ErrCJSIIndexChanged
		}
		return ctx.Err()
	}
	if e := check(); e != nil {
		return e
	}
	p, h := a.Parent, selected
	if h.Ticket < h.SessionTicket || h.SessionTicket <= p.Ticket || h.SessionTicket != a.Ticket || h.SessionAttempt != a.Attempt ||
		h.SessionRevision != a.SessionRevision || h.SessionSeal != a.Seal || a.Seal == ([32]byte{}) ||
		h.IndexEpoch != p.IndexEpoch || h.CoreEpoch != p.CoreEpoch || h.Device != p.Device || h.Inode == 0 || h.Inode != p.Inode ||
		h.BaseTicket != p.SessionTicket || h.BaseAttempt != p.SessionAttempt || h.BaseSeal != p.SessionSeal || h.BaseRevision != p.SessionRevision ||
		h.ProtectionRevision != p.ProtectionRevision || h.CoverageRevision != p.CoverageRevision {
		return store.ErrCJSIIndexChanged
	}
	// All CTE inputs are typed keys from the pinned stamp.
	side := fmt.Sprintf(" WHERE ticket=%d AND attempt=%d", h.SessionTicket, h.SessionAttempt)
	w := indexCompaction{ctx: ctx, tx: tx, with: "WITH cs AS (SELECT * FROM main.cjsi_candidate_sources" + side +
		"), cg AS (SELECT * FROM main.cjsi_candidate_groups" + side + "), co AS (SELECT * FROM main.cjsi_candidate_order" + side +
		"), cr AS (SELECT * FROM main.cjsi_candidate_ranges" + side + "), cm AS (SELECT * FROM main.cjsi_candidate_doc_masks" + side + ") "}
	if e := w.compact(a, h); e != nil {
		return e
	}
	return check()
}

type indexCompaction struct {
	ctx  context.Context
	tx   *sql.Tx
	with string
}

// In this producer every replacement occupies an entire source, including
// zero-member groups. Group-only deletes may also touch a source with other
// surviving groups. Every physical base mutation stays within this predicate.
const compactTouched = `(source_path IN (SELECT source_path FROM cs) OR
	(source_path,client,session_id) IN (SELECT source_path,client,session_id FROM cg))`
const compactGroupKey = "source_path,client,session_id"
const compactRangeFields = compactGroupKey + `,workset_round,workset_attempt,identity,selection_epoch,generation,
	replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest`
const compactOrderFields = compactGroupKey + ",doc_id,detail_ordinal,record_start,record_end,emission"

func (w *indexCompaction) compact(a indexStageAttempt, h store.CJSIIndexStamp) error {
	var water int64
	err := w.tx.QueryRowContext(w.ctx, `SELECT s.doc_id_high_water FROM main.cjsi_index_state s
		JOIN main.serving_generation v USING(singleton) JOIN main.session_index_generation g USING(singleton)
		JOIN main.cjsi_layout_initial_seal i USING(singleton) JOIN main.cjsi_session_candidates c
		ON (c.base_ticket,c.base_attempt,c.base_seal,c.base_revision)=(s.compacted_ticket,s.compacted_attempt,s.compacted_seal,s.base_revision)
		WHERE s.singleton=1 AND (SELECT foreign_keys FROM pragma_foreign_keys)=1 AND (SELECT defer_foreign_keys FROM pragma_defer_foreign_keys)=0
		  AND v.revision=s.base_revision AND g.epoch=i.index_epoch AND g.epoch=c.index_epoch
		  AND (s.compacted_ticket<>0 OR i.seal_digest=s.compacted_seal) AND s.protection_revision=c.protection_revision
		  AND (c.ticket,c.attempt,c.seal_digest,c.parent_ticket,c.core_epoch,c.index_epoch,c.base_ticket,c.base_attempt,c.base_seal,
		       c.base_revision,c.session_revision,c.protection_revision,c.coverage_revision)=(?,?,?,?,?,?,?,?,?,?,?,?,?)
		  AND NOT EXISTS(SELECT 1 FROM main.cjsi_candidate_coverage cc WHERE (cc.ticket,cc.attempt)=(c.ticket,c.attempt))`,
		h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], a.Parent.Ticket, h.CoreEpoch, h.IndexEpoch, h.BaseTicket, h.BaseAttempt,
		h.BaseSeal[:], h.BaseRevision, h.SessionRevision, h.ProtectionRevision, h.CoverageRevision).Scan(&water)
	if err != nil {
		return err
	}
	if err = w.checkProducer(water); err != nil {
		return err
	}

	coverage, err := w.coverage()
	if err != nil {
		return err
	}
	before, err := w.image(false)
	if err != nil {
		return err
	}
	// Both physical FTS5 tables index the same text column with the same
	// tokenizer. Verify their actual postings against their content, including
	// external-content postings which a plain SELECT cannot inspect.
	if _, err = w.tx.ExecContext(w.ctx, `INSERT INTO main.cjsi_candidate_documents(cjsi_candidate_documents,rank) VALUES('integrity-check',1)`); err != nil {
		return err
	}
	if err = w.checkBaseFTS(); err != nil {
		return err
	}
	writes, err := w.merge()
	if err != nil {
		return err
	}

	observed, err := w.coverage()
	if err != nil {
		return err
	}
	expected := coverage
	if writes > 0 && coverage.state.String == "complete" {
		expected.state.String = "update_pending"
	}
	if observed != expected {
		return store.ErrCJSIIndexChanged
	}
	if observed != coverage {
		if err = stageOne(w.ctx, w.tx, `UPDATE main.inventory_runs SET state='complete'
			WHERE inventory_id=? AND state='update_pending'`, coverage.id.String); err != nil {
			return err
		}
	}
	// Count only direct source/metadata writes. Metadata for deleted sources
	// was removed explicitly, so no uncounted cascade should advance revision.
	if err = stageOne(w.ctx, w.tx, `UPDATE main.serving_generation SET revision=? WHERE singleton=1 AND revision=?+?`,
		h.SessionRevision, h.BaseRevision, writes); err != nil {
		return err
	}
	err = stageOne(w.ctx, w.tx, `UPDATE main.cjsi_index_state SET compacted_ticket=?,compacted_attempt=?,compacted_seal=?,base_revision=?
		WHERE singleton=1 AND (compacted_ticket,compacted_attempt,compacted_seal,base_revision,protection_revision,doc_id_high_water)
		=(?,?,?,?,?,?) AND (SELECT revision FROM main.serving_generation WHERE singleton=1)=?`,
		h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.BaseTicket, h.BaseAttempt, h.BaseSeal[:],
		h.BaseRevision, h.ProtectionRevision, water, h.SessionRevision)
	if err != nil {
		return err
	}
	final, err := w.coverage()
	if err != nil {
		return err
	}
	if final != coverage {
		return store.ErrCJSIIndexChanged
	}
	var ok int
	if err = w.tx.QueryRowContext(w.ctx, `SELECT 1 FROM main.cjsi_index_state s JOIN main.serving_generation v USING(singleton)
		JOIN main.session_index_generation g USING(singleton)
		WHERE s.singleton=1 AND (s.compacted_ticket,s.compacted_attempt,s.compacted_seal,s.base_revision,s.protection_revision,
			s.doc_id_high_water,v.revision,g.epoch)=(?,?,?,?,?,?,?,?)`, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision,
		h.ProtectionRevision, water, h.SessionRevision, h.IndexEpoch).Scan(&ok); err != nil {
		return err
	}
	if err = w.checkBaseFTS(); err != nil {
		return err
	}
	after, err := w.image(true)
	if err != nil {
		return err
	}
	if before != after {
		return store.ErrCJSIIndexChanged
	}

	return nil
}

func (w *indexCompaction) checkBaseFTS() error {
	_, err := w.tx.ExecContext(w.ctx, `INSERT INTO main.session_documents(session_documents) VALUES('integrity-check')`)
	return err
}

type compactCoverage struct {
	id, state, capturedAt sql.NullString
	epoch, sourceCount    sql.NullInt64
}

func (w *indexCompaction) coverage() (compactCoverage, error) {
	var c compactCoverage
	err := w.tx.QueryRowContext(w.ctx, `SELECT h.inventory_id,r.state,r.captured_at,r.epoch,r.source_count FROM main.cjsi_index_state s
		LEFT JOIN main.domain_coverage_heads h USING(singleton) LEFT JOIN main.inventory_runs r USING(inventory_id)
		WHERE s.singleton=1`).Scan(&c.id, &c.state, &c.capturedAt, &c.epoch, &c.sourceCount)
	if err == nil && c.id.Valid != c.state.Valid {
		err = store.ErrCJSIIndexChanged
	}
	return c, err
}

// Make the restricted candidate projection explicit before changing any rows.
// In particular, every eligible non-deleted group on an upsert path must be
// represented, and every retained origin must remain safe across a move.
func (w *indexCompaction) checkProducer(water int64) error {
	var unsafe bool
	err := w.tx.QueryRowContext(w.ctx, w.with+`SELECT
		(NOT EXISTS(SELECT 1 FROM cs) AND NOT EXISTS(SELECT 1 FROM cg))
		OR EXISTS(SELECT 1 FROM cs WHERE op='upsert' AND (source_kind NOT IN ('codex-jsonl','otel') OR
			(SELECT count(*) FROM cg WHERE cg.source_path=cs.source_path AND cg.op='replace')<>1))
		OR EXISTS(SELECT 1 FROM cg WHERE op='patch' OR (op='replace' AND NOT EXISTS(
            SELECT 1 FROM cs WHERE cs.source_path=cg.source_path AND cs.op='upsert' AND cs.source_kind='otel'
              AND cs.identity=cs.source_path AND cs.source_path='otel:'||cg.client||'/'||cg.session_id
              AND cg.order_verified=1 AND cs.size=0 AND cs.complete_end=0 AND cs.parser_version=cg.parser_version
              AND NOT EXISTS(SELECT 1 FROM cr WHERE cr.source_path=cg.source_path)
              AND NOT EXISTS(SELECT 1 FROM co WHERE co.source_path=cg.source_path))
            AND (client<>'codex' OR order_verified<>1 OR parser_version<>? OR
			NOT EXISTS(SELECT 1 FROM cs JOIN cr USING(source_path,identity) WHERE cs.source_path=cg.source_path AND cs.op='upsert'
				AND cs.source_kind='codex-jsonl' AND cr.client=cg.client AND cr.session_id=cg.session_id AND cr.published_end=cr.complete_end AND cr.complete_end>0
				AND cs.complete_end=cr.complete_end AND cr.read_end<=cs.size AND cs.parser_version=cg.parser_version))))
		OR EXISTS(SELECT 1 FROM (SELECT o.*,row_number() OVER w AS n,lag(record_end) OVER w AS previous_end
			FROM co o WINDOW w AS (PARTITION BY source_path,client,session_id ORDER BY detail_ordinal)) o
			LEFT JOIN cr r USING(source_path,client,session_id) WHERE r.source_path IS NULL OR o.detail_ordinal<>o.n
			OR o.record_start IS NULL OR o.emission<>0 OR o.record_start<o.previous_end OR o.record_end>r.published_end)
		OR EXISTS(SELECT 1 FROM co WHERE doc_side='candidate' AND NOT EXISTS(SELECT 1 FROM main.cjsi_candidate_payloads p
			WHERE p.doc_id=co.doc_id AND p.kind IN ('user_prompt','assistant_final')))
		OR EXISTS(SELECT 1 FROM cr JOIN cg USING(source_path,client,session_id) WHERE cg.op<>'replace')
		OR EXISTS(SELECT 1 FROM main.session_metadata b JOIN cs USING(source_path) WHERE cs.op='upsert'
			AND NOT EXISTS(SELECT 1 FROM cg WHERE (source_path,client,session_id)=(b.source_path,b.client,b.session_id)))
		OR EXISTS(SELECT 1 FROM co LEFT JOIN main.cjsi_base_order b USING(doc_id) WHERE co.doc_id>? OR
			(co.doc_side='candidate' AND (b.doc_id IS NOT NULL OR EXISTS(SELECT 1 FROM main.session_documents WHERE rowid=co.doc_id))) OR
			(co.doc_side='base' AND (b.doc_id IS NULL OR (b.client,b.session_id)<>(co.client,co.session_id) OR
				(b.source_path<>co.source_path AND NOT EXISTS(SELECT 1 FROM cs JOIN main.session_sources old USING(source_path) WHERE cs.source_path=b.source_path AND (cs.op='delete' OR (cs.op='upsert' AND cs.identity<>old.identity))
					AND old.identity=(SELECT dest.identity FROM cs dest WHERE dest.source_path=co.source_path))) OR
				NOT EXISTS(SELECT 1 FROM main.session_documents d WHERE d.rowid=b.doc_id AND
					(d.source_path,d.client,d.session_id)=(b.source_path,b.client,b.session_id)))))
		OR EXISTS(SELECT 1 FROM co JOIN cm USING(source_path,client,session_id,doc_id))
		OR EXISTS(SELECT 1 FROM cg WHERE op='replace' AND (
			EXISTS(SELECT 1 FROM main.cjsi_session_protections p WHERE p.source_path=cg.source_path
				AND (p.client='' OR (p.client,p.session_id)=(cg.client,cg.session_id))) OR
			EXISTS(SELECT 1 FROM main.session_exclusions x WHERE (x.kind='path' AND x.value=cg.source_path) OR
				(x.kind='client' AND x.value=cg.client) OR (x.kind='session' AND x.value=cg.session_id) OR (x.kind='project' AND x.value=cg.project))))
		OR EXISTS(SELECT 1 FROM co JOIN main.cjsi_base_order b USING(doc_id) JOIN main.session_metadata m
			ON (m.source_path,m.client,m.session_id)=(b.source_path,b.client,b.session_id) WHERE co.doc_side='base' AND (
			EXISTS(SELECT 1 FROM main.cjsi_session_protections p WHERE p.source_path=b.source_path AND
				(p.client='' OR ((p.client,p.session_id)=(b.client,b.session_id) AND p.doc_id IN (0,b.doc_id)))) OR
			EXISTS(SELECT 1 FROM main.session_exclusions x WHERE (x.kind='path' AND x.value=b.source_path) OR
				(x.kind='project' AND x.value=m.project))))`, nativeSessionParserVersion, water).Scan(&unsafe)
	if err != nil {
		return err
	}
	if unsafe {
		return store.ErrCJSIIndexChanged
	}
	return nil
}

// Capture exact SQLite values for affected physical copies (before replica
// ranking), not a new seal. Unaffected copies and their protections stay in
// place. Equality here implies identical winners, counts and page ordering.
// Compare raw FTS owners independently so a leftover masked payload fails too.
func (w *indexCompaction) image(base bool) ([5][sha256.Size]byte, error) {
	const srcFields = "source_path,identity,priority,parser_version,size,modified_at,changed_at,prefix_hash,scanned_at"
	const groupFields = "source_path,client,session_id,project,model,parser_version,first_at,last_at,order_verified,sequence_id,head_at,birth"
	queries := []string{
		"SELECT " + srcFields + ",complete_end,x'','' FROM cs WHERE op='upsert'",
		"SELECT " + groupFields + ",(SELECT source_kind FROM cs WHERE cs.source_path=cg.source_path) FROM cg WHERE op='replace'",
		"SELECT " + compactOrderFields + " FROM co",
		"SELECT " + compactRangeFields + " FROM cr",
		`SELECT o.source_path,o.client,o.session_id,o.doc_id,d.event_at,d.kind,d.text FROM co o JOIN main.session_documents d ON d.rowid=o.doc_id WHERE o.doc_side='base'
		 UNION ALL SELECT o.source_path,o.client,o.session_id,o.doc_id,p.event_at,p.kind,p.text FROM co o JOIN main.cjsi_candidate_payloads p USING(doc_id) WHERE o.doc_side='candidate'`,
	}
	if base {
		queries = []string{
			"SELECT " + srcFields + ",cursor,partial_line,parser_context FROM main.session_sources WHERE source_path IN (SELECT source_path FROM cs)",
			"SELECT " + groupFields + ",source_kind FROM main.session_metadata JOIN main.cjsi_base_groups USING(" + compactGroupKey + ") WHERE " + compactTouched,
			"SELECT " + compactOrderFields + " FROM main.cjsi_base_order WHERE " + compactTouched,
			"SELECT " + compactRangeFields + " FROM main.cjsi_base_ranges WHERE " + compactTouched,
			"SELECT source_path,client,session_id,rowid,event_at,kind,text FROM main.session_documents WHERE " + compactTouched,
		}
	}
	// Both source and group rows have a unique key within the first three
	// fields. Order and document rows also require the fourth (document ID).
	var image [5][sha256.Size]byte
	for i, q := range queries {
		rows, err := w.tx.QueryContext(w.ctx, w.with+q+" ORDER BY 1,2,3,4")
		if err != nil {
			return image, err
		}
		image[i], err = digestIndexRows(rows)
		if err != nil {
			return image, err
		}
	}
	return image, nil
}

// Frame each SQLite value by type and size. Keep at most one row in memory
// while comparing the source, metadata, order, range and content relations.
func digestIndexRows(rows *sql.Rows) ([sha256.Size]byte, error) {
	defer rows.Close()
	var digest [sha256.Size]byte
	cols, err := rows.Columns()
	if err != nil {
		return digest, err
	}
	values := make([]any, len(cols))
	targets := make([]any, len(cols))
	for i := range values {
		targets[i] = &values[i]
	}
	h := sha256.New()
	var header [9]byte
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return digest, err
		}
		for _, v := range values {
			switch x := v.(type) {
			case nil:
				h.Write([]byte{0})
			case int64:
				header[0] = 1
				binary.BigEndian.PutUint64(header[1:], uint64(x))
				h.Write(header[:])
			case string:
				header[0] = 2
				binary.BigEndian.PutUint64(header[1:], uint64(len(x)))
				h.Write(header[:])
				io.WriteString(h, x)
			case []byte:
				header[0] = 3
				binary.BigEndian.PutUint64(header[1:], uint64(len(x)))
				h.Write(header[:])
				h.Write(x)
			default:
				return digest, store.ErrCJSIIndexChanged
			}
		}
	}
	copy(digest[:], h.Sum(nil))
	return digest, rows.Err()
}

// Explicit upserts preserve referenced parents. Park retained ordinals above
// both sequences before rewriting them: SQLite checks uniqueness per row,
// and sealed candidate_order.base_id prevents deleting a retained base ID.
// Use dense detail positions as offsets because stable IDs may be near MaxInt64.
func (w *indexCompaction) merge() (int64, error) {
	var ceiling int64
	err := w.tx.QueryRowContext(w.ctx, w.with+"SELECT coalesce(max(detail_ordinal),0) FROM (SELECT detail_ordinal FROM main.cjsi_base_order WHERE "+
		compactTouched+" UNION ALL SELECT detail_ordinal FROM co)").Scan(&ceiling)
	if err != nil {
		return 0, err
	}
	const retained = "(SELECT doc_id FROM co WHERE doc_side='base')"
	const deleted = "(SELECT source_path FROM cs WHERE op='delete')"
	var increments int64
	for _, op := range []struct {
		query    string
		revision bool
	}{
		{`INSERT INTO main.session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			SELECT source_path,identity,complete_end,x'',size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,'' FROM cs WHERE op='upsert'
			ON CONFLICT(source_path) DO UPDATE SET identity=excluded.identity,cursor=excluded.cursor,partial_line=excluded.partial_line,
			size=excluded.size,modified_at=excluded.modified_at,changed_at=excluded.changed_at,prefix_hash=excluded.prefix_hash,priority=excluded.priority,
			parser_version=excluded.parser_version,scanned_at=excluded.scanned_at,parser_context=excluded.parser_context`, true},
		{`INSERT INTO main.session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			SELECT source_path,client,session_id,project,model,parser_version,first_at,last_at FROM cg WHERE op='replace'
			ON CONFLICT(source_path,client,session_id) DO UPDATE SET project=excluded.project,model=excluded.model,
			parser_version=excluded.parser_version,first_at=excluded.first_at,last_at=excluded.last_at`, true},
		{`INSERT INTO main.cjsi_base_groups(source_path,client,session_id,order_verified,sequence_id,head_at,birth,source_kind)
			SELECT source_path,client,session_id,order_verified,sequence_id,head_at,birth,source_kind FROM cg JOIN cs USING(source_path) WHERE cg.op='replace'
			ON CONFLICT(source_path,client,session_id) DO UPDATE SET order_verified=excluded.order_verified,sequence_id=excluded.sequence_id,
			head_at=excluded.head_at,birth=excluded.birth,source_kind=excluded.source_kind`, false},
		{"DELETE FROM main.cjsi_legacy_cursor_hints WHERE source_path IN (SELECT source_path FROM cs)", false},
		{"DELETE FROM main.session_documents WHERE " + compactTouched + " AND rowid NOT IN " + retained, false},
		{"DELETE FROM main.cjsi_base_order WHERE " + compactTouched + " AND doc_id NOT IN " + retained, false},
		{fmt.Sprintf("UPDATE main.cjsi_base_order SET detail_ordinal=detail_ordinal+%d,record_start=NULL,record_end=NULL,emission=NULL WHERE doc_id IN ", ceiling) + retained, false},
		{"INSERT INTO main.cjsi_base_order(" + compactOrderFields + ") SELECT " + compactOrderFields + ` FROM co WHERE true
			ON CONFLICT(doc_id) DO UPDATE SET source_path=excluded.source_path,client=excluded.client,session_id=excluded.session_id,
			detail_ordinal=excluded.detail_ordinal,record_start=excluded.record_start,record_end=excluded.record_end,emission=excluded.emission`, false},
		{`UPDATE main.session_documents SET (source_path,client,session_id)=(SELECT source_path,client,session_id FROM co WHERE doc_id=session_documents.rowid)
			WHERE rowid IN ` + retained, false},
		{`INSERT INTO main.session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			SELECT doc_id,source_path,client,session_id,event_at,kind,text FROM co JOIN main.cjsi_candidate_payloads USING(doc_id) WHERE doc_side='candidate'`, false},
		{"DELETE FROM main.cjsi_base_ranges WHERE " + compactTouched, false},
		{"INSERT INTO main.cjsi_base_ranges(" + compactRangeFields + ") SELECT " + compactRangeFields + " FROM cr", false},
		{`DELETE FROM main.session_metadata WHERE source_path IN ` + deleted + ` OR (source_path,client,session_id)
			IN (SELECT source_path,client,session_id FROM cg WHERE op='delete')`, true},
		{"DELETE FROM main.session_sources WHERE source_path IN " + deleted, true},
	} {
		result, err := w.tx.ExecContext(w.ctx, w.with+op.query)
		if err != nil {
			return 0, err
		}
		if op.revision {
			n, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}
			increments += n
		}
	}
	return increments, nil
}
