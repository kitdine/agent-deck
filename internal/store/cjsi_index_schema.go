package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// This singleton is the *selected* vector, not a preparation queue. Ticket
// zero is the installed base. A core-only publication may carry forward a
// session or slot component from an earlier ticket. No domain payload lives here.
const cjsiPublicationHeadSchema = `
CREATE TABLE main.cjsi_publication_head (
    singleton INTEGER PRIMARY KEY CHECK(singleton=1),
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0),
    ticket INTEGER NOT NULL DEFAULT 0 CHECK(ticket>=0),
    session_ticket INTEGER NOT NULL DEFAULT 0 CHECK(session_ticket BETWEEN 0 AND ticket),
    session_attempt INTEGER NOT NULL DEFAULT 0 CHECK(session_attempt>=0),
    index_epoch INTEGER NOT NULL DEFAULT 0 CHECK(index_epoch>=0),
    base_ticket INTEGER NOT NULL DEFAULT 0 CHECK(base_ticket>=0),
    base_attempt INTEGER NOT NULL DEFAULT 0 CHECK(base_attempt>=0),
    base_seal BLOB CHECK(length(base_seal)=32 AND base_seal<>zeroblob(32)),
    base_revision INTEGER NOT NULL DEFAULT 0 CHECK(base_revision>=0),
    session_revision INTEGER NOT NULL DEFAULT 0 CHECK(session_revision>=base_revision),
    session_seal BLOB CHECK(length(session_seal)=32 AND session_seal<>zeroblob(32)),
    slot_ticket INTEGER NOT NULL DEFAULT 0 CHECK(slot_ticket BETWEEN 0 AND ticket),
    slot_attempt INTEGER NOT NULL DEFAULT 0 CHECK(slot_attempt>=0),
    slot_seal BLOB CHECK(length(slot_seal)=32 AND slot_seal<>zeroblob(32)),
    protection_revision INTEGER NOT NULL DEFAULT 0 CHECK(protection_revision>=0),
    coverage_revision INTEGER NOT NULL DEFAULT 0 CHECK(coverage_revision>=0),
    CHECK((index_epoch=0 AND session_ticket=0 AND session_seal IS NULL AND session_revision=0 AND base_ticket=0)
       OR (index_epoch>0 AND session_seal IS NOT NULL)),
    CHECK((base_ticket=0)=(base_attempt=0) AND (index_epoch=0)=(base_seal IS NULL)),
    CHECK((session_ticket=0 AND session_attempt=0 AND base_ticket=0 AND base_revision=session_revision AND base_seal IS session_seal)
       OR (session_ticket>0 AND session_attempt>0 AND base_ticket<session_ticket)),
    CHECK((slot_ticket=0 AND slot_attempt=0 AND slot_seal IS NULL)
       OR (slot_ticket>0 AND slot_attempt>0 AND slot_seal IS NOT NULL))
) STRICT;
INSERT INTO main.cjsi_publication_head(singleton,core_epoch)
VALUES(1,(SELECT epoch FROM main.derived_snapshot_generation WHERE singleton=1));
`

// These are private-index tables. A group key includes the source replica;
// (client,session_id) alone is not a physical group. An ordinal may refer to
// a retained base row without copying its text. FTS rowid is always doc_id.
const cjsiGroupKeyColumns = `
    source_path TEXT NOT NULL, client TEXT NOT NULL, session_id TEXT NOT NULL,`

const cjsiGroupEvidenceColumns = `
    order_verified INTEGER NOT NULL DEFAULT 0 CHECK(order_verified IN (0,1)),
    sequence_id BLOB NOT NULL CHECK(length(sequence_id)=32 AND sequence_id<>zeroblob(32)),
    head_at TEXT NOT NULL DEFAULT '', birth TEXT NOT NULL DEFAULT '',`

const cjsiOrderColumns = `
    detail_ordinal INTEGER NOT NULL CHECK(detail_ordinal>0),
    record_start INTEGER, record_end INTEGER, emission INTEGER,
    CHECK((record_start IS NULL AND record_end IS NULL AND emission IS NULL)
       OR (record_start IS NOT NULL AND record_start>=0 AND record_end IS NOT NULL AND record_end>record_start
           AND emission IS NOT NULL AND emission>=0)),`

// The proof is for one selected plaintext capture. The complete/read boundary
// expresses Partial without storing the trailing bytes. Never synthesize a
// range proof from an old cursor or a digest of document text.
const cjsiRangeColumns = `
    workset_round INTEGER NOT NULL CHECK(workset_round>0), workset_attempt INTEGER NOT NULL CHECK(workset_attempt>0),
    identity TEXT NOT NULL CHECK(length(identity)>0),
    selection_epoch INTEGER NOT NULL CHECK(selection_epoch>0),
    generation INTEGER NOT NULL CHECK(generation>0),
    replay_parser INTEGER NOT NULL CHECK(replay_parser>0),
    max_line_bytes INTEGER NOT NULL CHECK(max_line_bytes>0),
    read_end INTEGER NOT NULL, complete_end INTEGER NOT NULL CHECK(complete_end BETWEEN 0 AND read_end),
    replay_from INTEGER NOT NULL CHECK(replay_from BETWEEN 0 AND complete_end),
    published_end INTEGER NOT NULL CHECK(published_end BETWEEN 0 AND complete_end),
    prefix_digest BLOB NOT NULL CHECK(length(prefix_digest)=32 AND prefix_digest<>zeroblob(32)),`

const cjsiCandidateKeyColumns = `
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL,`

const cjsiCandidateFK = `FOREIGN KEY(ticket,attempt) REFERENCES cjsi_session_candidates(ticket,attempt) ON DELETE CASCADE`
const cjsiCandidateGroupFK = `FOREIGN KEY(ticket,attempt,source_path,client,session_id)
    REFERENCES cjsi_candidate_groups(ticket,attempt,source_path,client,session_id) ON DELETE CASCADE`

const cjsiPrivateIndexSchema = `
CREATE TABLE main.cjsi_index_state (
    singleton INTEGER PRIMARY KEY CHECK(singleton=1),
    doc_id_high_water INTEGER NOT NULL CHECK(doc_id_high_water>=0),
    compacted_ticket INTEGER NOT NULL CHECK(compacted_ticket>=0),
    compacted_attempt INTEGER NOT NULL CHECK(compacted_attempt>=0),
    compacted_seal BLOB NOT NULL CHECK(length(compacted_seal)=32 AND compacted_seal<>zeroblob(32)),
    base_revision INTEGER NOT NULL CHECK(base_revision>=0),
    protection_revision INTEGER NOT NULL DEFAULT 0 CHECK(protection_revision>=0),
    CHECK((compacted_ticket=0)=(compacted_attempt=0))
) STRICT;
CREATE TRIGGER cjsi_id_high_water BEFORE UPDATE OF doc_id_high_water ON cjsi_index_state
WHEN NEW.doc_id_high_water<OLD.doc_id_high_water
BEGIN SELECT RAISE(ABORT,'document ID high water decreased'); END;
-- A bounded legacy tail length, never a capture proof. An independently
-- classified compatible source can compare its unchanged observation against
-- the original read end without requiring the discarded plaintext.
CREATE TABLE main.cjsi_legacy_cursor_hints (
    source_path TEXT PRIMARY KEY REFERENCES session_sources(source_path) ON DELETE CASCADE,
    identity TEXT NOT NULL, parser_version INTEGER NOT NULL, size INTEGER NOT NULL,
    modified_at INTEGER NOT NULL, changed_at INTEGER NOT NULL, prefix_hash TEXT NOT NULL,
    read_end INTEGER NOT NULL CHECK(read_end BETWEEN 0 AND size),
    complete_end INTEGER NOT NULL CHECK(complete_end BETWEEN 0 AND read_end)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_session_protections (` + cjsiGroupKeyColumns + `
    doc_id INTEGER NOT NULL CHECK(doc_id>=0),
    CHECK(length(source_path)>0 AND ((client='' AND session_id='' AND doc_id=0) OR (client<>'' AND session_id<>''))),
    PRIMARY KEY(source_path,client,session_id,doc_id)
) STRICT, WITHOUT ROWID;
CREATE TRIGGER main.cjsi_protect_insert AFTER INSERT ON cjsi_session_protections BEGIN
    UPDATE cjsi_index_state SET protection_revision=protection_revision+1 WHERE singleton=1;
END;
CREATE TRIGGER main.cjsi_protect_update AFTER UPDATE ON cjsi_session_protections BEGIN
    UPDATE cjsi_index_state SET protection_revision=protection_revision+1 WHERE singleton=1;
END;
CREATE TRIGGER main.cjsi_protect_delete AFTER DELETE ON cjsi_session_protections BEGIN
    UPDATE cjsi_index_state SET protection_revision=protection_revision+1 WHERE singleton=1;
END;
CREATE TABLE main.cjsi_base_groups (` + cjsiGroupKeyColumns + cjsiGroupEvidenceColumns + `
    source_kind TEXT CHECK(source_kind IN ('codex-jsonl','claude-jsonl','otel','synthetic')),
    PRIMARY KEY(source_path,client,session_id),
    FOREIGN KEY(source_path,client,session_id) REFERENCES session_metadata(source_path,client,session_id) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_base_order (` + cjsiGroupKeyColumns + `
    doc_id INTEGER PRIMARY KEY CHECK(doc_id>0),` + cjsiOrderColumns + `
    UNIQUE(source_path,client,session_id,detail_ordinal),
    UNIQUE(source_path,client,session_id,record_start,emission),
    FOREIGN KEY(source_path,client,session_id) REFERENCES cjsi_base_groups(source_path,client,session_id) ON DELETE CASCADE
) STRICT;
CREATE TABLE main.cjsi_base_ranges (` + cjsiGroupKeyColumns + cjsiRangeColumns + `
    PRIMARY KEY(source_path,client,session_id),
    FOREIGN KEY(source_path,client,session_id) REFERENCES cjsi_base_groups(source_path,client,session_id) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_session_candidates (
    ticket INTEGER NOT NULL CHECK(ticket>0), attempt INTEGER NOT NULL CHECK(attempt>0),
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0), index_epoch INTEGER NOT NULL CHECK(index_epoch>0),
    base_ticket INTEGER NOT NULL CHECK(base_ticket>=0 AND base_ticket<ticket),
    base_attempt INTEGER NOT NULL CHECK(base_attempt>=0 AND (base_attempt=0)=(base_ticket=0)),
    base_seal BLOB NOT NULL CHECK(length(base_seal)=32 AND base_seal<>zeroblob(32)),
    base_revision INTEGER NOT NULL CHECK(base_revision>=0),
    parent_ticket INTEGER NOT NULL CHECK(parent_ticket BETWEEN base_ticket AND ticket-1),
    protection_revision INTEGER NOT NULL CHECK(protection_revision>=0),
    coverage_revision INTEGER NOT NULL CHECK(coverage_revision>=0),
    session_revision INTEGER NOT NULL CHECK(session_revision>=base_revision),
    seal_digest BLOB CHECK(length(seal_digest)=32 AND seal_digest<>zeroblob(32)),
    PRIMARY KEY(ticket,attempt)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_sources (` + cjsiCandidateKeyColumns + `
    source_path TEXT NOT NULL, op TEXT NOT NULL CHECK(op IN ('upsert','delete')),
    source_kind TEXT CHECK(source_kind IN ('codex-jsonl','claude-jsonl','otel','synthetic')),
    identity TEXT, priority INTEGER, parser_version INTEGER, complete_end INTEGER,
    size INTEGER, modified_at INTEGER, changed_at INTEGER, prefix_hash TEXT, scanned_at TEXT,
    CHECK(op='delete' OR (source_kind IS NOT NULL AND identity IS NOT NULL AND priority IS NOT NULL
       AND parser_version IS NOT NULL AND parser_version>0 AND size IS NOT NULL
       AND complete_end IS NOT NULL AND complete_end BETWEEN 0 AND size AND modified_at IS NOT NULL
       AND changed_at IS NOT NULL AND prefix_hash IS NOT NULL AND scanned_at IS NOT NULL)),
    PRIMARY KEY(ticket,attempt,source_path), ` + cjsiCandidateFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_groups (` + cjsiCandidateKeyColumns + cjsiGroupKeyColumns + `
    op TEXT NOT NULL CHECK(op IN ('patch','replace','delete')),
    project TEXT, model TEXT, parser_version INTEGER, first_at TEXT, last_at TEXT,` + cjsiGroupEvidenceColumns + `
    CHECK(op='delete' OR (project IS NOT NULL AND model IS NOT NULL AND parser_version IS NOT NULL
       AND parser_version>0 AND first_at IS NOT NULL AND last_at IS NOT NULL)),
    PRIMARY KEY(ticket,attempt,source_path,client,session_id), ` + cjsiCandidateFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_doc_masks (` + cjsiCandidateKeyColumns + cjsiGroupKeyColumns + `
    doc_id INTEGER NOT NULL CHECK(doc_id>0),
    PRIMARY KEY(ticket,attempt,source_path,client,session_id,doc_id), ` + cjsiCandidateGroupFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_payloads (
    doc_id INTEGER PRIMARY KEY CHECK(doc_id>0), event_at TEXT NOT NULL, kind TEXT NOT NULL, text TEXT NOT NULL
) STRICT;
-- A payload acquires its group through the selected membership. Expose the
-- same MATCH columns as session_documents without assigning it a group here.
CREATE VIEW main.cjsi_candidate_document_content AS
    SELECT doc_id, NULL AS source_path, NULL AS client, NULL AS session_id, event_at, kind, text
    FROM cjsi_candidate_payloads;
CREATE VIRTUAL TABLE main.cjsi_candidate_documents USING fts5(
    source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, event_at UNINDEXED, kind UNINDEXED, text,
    content='cjsi_candidate_document_content',content_rowid='doc_id');
CREATE TRIGGER cjsi_payload_no_replace BEFORE INSERT ON cjsi_candidate_payloads
WHEN EXISTS(SELECT 1 FROM cjsi_candidate_payloads WHERE doc_id=NEW.doc_id)
BEGIN SELECT RAISE(ABORT,'candidate payload already exists'); END;
CREATE TRIGGER cjsi_payload_immutable BEFORE UPDATE ON cjsi_candidate_payloads
BEGIN SELECT RAISE(ABORT,'immutable candidate payload'); END;
CREATE TRIGGER cjsi_payload_insert AFTER INSERT ON cjsi_candidate_payloads BEGIN
    INSERT INTO cjsi_candidate_documents(rowid,source_path,client,session_id,event_at,kind,text)
    VALUES(NEW.doc_id,NULL,NULL,NULL,NEW.event_at,NEW.kind,NEW.text);
END;
CREATE TRIGGER cjsi_payload_delete AFTER DELETE ON cjsi_candidate_payloads BEGIN
    INSERT INTO cjsi_candidate_documents(cjsi_candidate_documents,rowid,source_path,client,session_id,event_at,kind,text)
    VALUES('delete',OLD.doc_id,NULL,NULL,NULL,OLD.event_at,OLD.kind,OLD.text);
END;
CREATE TABLE main.cjsi_candidate_order (` + cjsiCandidateKeyColumns + cjsiGroupKeyColumns + `
    doc_id INTEGER NOT NULL CHECK(doc_id>0), doc_side TEXT NOT NULL CHECK(doc_side IN ('base','candidate')),
    base_id INTEGER GENERATED ALWAYS AS (CASE WHEN doc_side='base' THEN doc_id END) VIRTUAL REFERENCES cjsi_base_order(doc_id),
    payload_id INTEGER GENERATED ALWAYS AS (CASE WHEN doc_side='candidate' THEN doc_id END) VIRTUAL REFERENCES cjsi_candidate_payloads(doc_id),` + cjsiOrderColumns + `
    PRIMARY KEY(ticket,attempt,doc_id),
    UNIQUE(ticket,attempt,source_path,client,session_id,detail_ordinal),
    UNIQUE(ticket,attempt,source_path,client,session_id,record_start,emission), ` + cjsiCandidateGroupFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_ranges (` + cjsiCandidateKeyColumns + cjsiGroupKeyColumns + cjsiRangeColumns + `
    PRIMARY KEY(ticket,attempt,source_path,client,session_id), ` + cjsiCandidateGroupFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_candidate_coverage (` + cjsiCandidateKeyColumns + `
    inventory_id TEXT NOT NULL, source_path TEXT NOT NULL, identity TEXT NOT NULL,
    captured_end INTEGER NOT NULL, committed_end INTEGER NOT NULL CHECK(committed_end BETWEEN 0 AND captured_end),
    state TEXT NOT NULL CHECK(state IN ('pending','complete','failed')),
    PRIMARY KEY(ticket,attempt,inventory_id,source_path), ` + cjsiCandidateFK + `
) STRICT, WITHOUT ROWID;
-- Seals are installed only on a pre-existing draft. Domain evidence and
-- sequence generation are checked by the producer; these are the storage
-- invariants that can be verified without replay or an open core transaction.
CREATE TRIGGER main.cjsi_draft_insert BEFORE INSERT ON cjsi_session_candidates
WHEN NEW.seal_digest IS NOT NULL OR EXISTS(SELECT 1 FROM cjsi_session_candidates
    WHERE (ticket,attempt)=(NEW.ticket,NEW.attempt))
BEGIN SELECT RAISE(ABORT,'candidate must be a new draft'); END;
CREATE TRIGGER main.cjsi_sealed_update BEFORE UPDATE ON cjsi_session_candidates
WHEN OLD.seal_digest IS NOT NULL OR (NEW.ticket,NEW.attempt)<>(OLD.ticket,OLD.attempt)
BEGIN SELECT RAISE(ABORT,'immutable candidate'); END;
CREATE TRIGGER main.cjsi_seal_members BEFORE UPDATE OF seal_digest ON cjsi_session_candidates
WHEN NEW.seal_digest IS NOT NULL AND (
  EXISTS(SELECT 1 FROM cjsi_candidate_order WHERE (ticket,attempt)=(NEW.ticket,NEW.attempt)
    GROUP BY source_path,client,session_id HAVING min(detail_ordinal)<>1 OR max(detail_ordinal)<>count(*))
  OR EXISTS(SELECT 1 FROM cjsi_candidate_order o JOIN cjsi_candidate_groups g USING(ticket,attempt,source_path,client,session_id)
    WHERE (o.ticket,o.attempt)=(NEW.ticket,NEW.attempt) AND (g.op='delete'
    OR o.doc_id>(SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1)
    OR EXISTS(SELECT 1 FROM cjsi_candidate_doc_masks m WHERE (m.ticket,m.attempt,m.source_path,m.client,m.session_id,m.doc_id)=
        (o.ticket,o.attempt,o.source_path,o.client,o.session_id,o.doc_id))
    OR (o.doc_side='base' AND NOT EXISTS(SELECT 1 FROM session_documents d JOIN cjsi_base_order b ON b.doc_id=d.rowid
        AND (d.source_path,d.client,d.session_id)=(b.source_path,b.client,b.session_id) WHERE d.rowid=o.doc_id))
    OR (o.doc_side='candidate' AND NOT EXISTS(SELECT 1 FROM cjsi_candidate_payloads p WHERE p.doc_id=o.doc_id)))))
BEGIN SELECT RAISE(ABORT,'invalid candidate detail mapping'); END;
`

// Guard both sides of an UPDATE: moving a row out of a sealed attempt is
// just as invalid as inserting into one. Lifecycle retirement removes the
// parent only after external pins permit it; FK cascades then have no parent.
func cjsiCandidateGuards() string {
	var sql strings.Builder
	for _, table := range []string{"sources", "groups", "doc_masks", "order", "ranges", "coverage"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			var terms []string
			for _, side := range []string{"OLD", "NEW"} {
				if op == "INSERT" && side == "OLD" || op == "DELETE" && side == "NEW" {
					continue
				}
				terms = append(terms, fmt.Sprintf("EXISTS(SELECT 1 FROM cjsi_session_candidates WHERE (ticket,attempt)=(%[1]s.ticket,%[1]s.attempt) AND seal_digest IS NOT NULL)", side))
			}
			fmt.Fprintf(&sql, "CREATE TRIGGER main.cjsi_lock_%s_%s BEFORE %s ON cjsi_candidate_%s WHEN %s BEGIN SELECT RAISE(ABORT,'sealed candidate'); END;\n", table, op, op, table, strings.Join(terms, " OR "))
		}
	}
	return sql.String()
}

func initializeCJSIPrivateIndex(ctx context.Context, tx *sql.Tx, seal cjsiIndexSeal) error {
	if _, err := tx.ExecContext(ctx, cjsiPrivateIndexSchema+cjsiCandidateGuards()); err != nil {
		return err
	}
	// Include the highest allocated legacy row, even one without reachable
	// metadata. Unreachable rows do not acquire a group or an ordinal here.
	if _, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_index_state
		(singleton,doc_id_high_water,compacted_ticket,compacted_attempt,compacted_seal,base_revision)
		VALUES(1,max(0,coalesce((SELECT max(rowid) FROM main.session_documents),0)),0,0,?,
		(SELECT revision FROM main.serving_generation WHERE singleton=1))`, seal.digest[:]); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_base_groups(source_path,client,session_id,sequence_id)
		SELECT m.source_path,m.client,m.session_id,randomblob(32) FROM main.session_metadata m JOIN main.session_sources s USING(source_path);
		INSERT INTO main.cjsi_base_order(source_path,client,session_id,doc_id,detail_ordinal)
		SELECT g.source_path,g.client,g.session_id,d.rowid,
		  row_number() OVER(PARTITION BY g.source_path,g.client,g.session_id ORDER BY d.rowid)
		FROM main.session_documents d JOIN main.cjsi_base_groups g USING(source_path,client,session_id)`); err != nil {
		return err
	}
	// No inventory is opened here, so an old metadata client cannot authorize
	// retaining a plaintext tail. A bounded cursor is the same start the old
	// range reader got from cursor-len(partial); it is not prefix proof.
	var complete int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.domain_coverage_heads h
		JOIN main.inventory_runs i USING(inventory_id) WHERE h.singleton=1 AND i.state='complete')`).Scan(&complete); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_legacy_cursor_hints
		SELECT source_path,identity,parser_version,size,modified_at,changed_at,prefix_hash,cursor,cursor-length(CAST(partial_line AS BLOB))
		FROM main.session_sources WHERE length(CAST(partial_line AS BLOB))>0 AND typeof(cursor)='integer'
		  AND typeof(size)='integer' AND cursor BETWEEN length(CAST(partial_line AS BLOB)) AND size AND instr(CAST(partial_line AS BLOB),x'0a')=0;
		UPDATE main.session_sources AS s SET
		cursor=coalesce((SELECT complete_end FROM main.cjsi_legacy_cursor_hints h WHERE h.source_path=s.source_path),0),
		parser_context=CASE WHEN EXISTS(SELECT 1 FROM main.cjsi_legacy_cursor_hints h WHERE h.source_path=s.source_path) THEN parser_context ELSE '' END,
		parser_version=CASE WHEN EXISTS(SELECT 1 FROM main.cjsi_legacy_cursor_hints h WHERE h.source_path=s.source_path) THEN parser_version ELSE 0 END,
		partial_line=x'' WHERE length(CAST(partial_line AS BLOB))>0;
		UPDATE main.serving_generation SET revision=(SELECT base_revision FROM main.cjsi_index_state WHERE singleton=1)
		WHERE singleton=1`)
	if err == nil && complete == 1 {
		_, err = tx.ExecContext(ctx, `UPDATE main.inventory_runs SET state='complete'
			WHERE inventory_id=(SELECT inventory_id FROM main.domain_coverage_heads WHERE singleton=1) AND state='update_pending'`)
	}
	return err
}

// ReserveCJSIDocumentIDs allocates a closed interval from the one private
// allocator. A writer must commit before any ID escapes this transaction.
func ReserveCJSIDocumentIDs(ctx context.Context, tx *sql.Tx, count int64) (first, last int64, err error) {
	if tx == nil || count <= 0 {
		return 0, 0, errCJSILayoutInvalid
	}
	err = tx.QueryRowContext(ctx, `UPDATE main.cjsi_index_state SET doc_id_high_water=doc_id_high_water+?
		WHERE singleton=1 AND doc_id_high_water<=9223372036854775807-? RETURNING doc_id_high_water`, count, count).Scan(&last)
	if err != nil {
		return 0, 0, err
	}
	return last - count + 1, last, nil
}
