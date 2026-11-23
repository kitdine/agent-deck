package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/platform"
)

var (
	ErrCJSIIndexAbsent  = errors.New("selected session index was never built")
	ErrCJSIIndexChanged = errors.New("selected session index is unavailable or changed")
)

// CJSIIndexStamp is a comparable logical selection, including the global
// ticket when a core-only publication carries an earlier session component.
type CJSIIndexStamp struct {
	Ticket, CoreEpoch, IndexEpoch                  int64
	SessionTicket, SessionAttempt, SessionRevision int64
	BaseTicket, BaseAttempt, BaseRevision          int64
	BaseSeal, SessionSeal                          [32]byte
	ProtectionRevision, CoverageRevision           int64
	Device, Inode                                  uint64
}

// CJSIIndexSnapshot borrows a read-only private transaction for the callback.
// Only the effective-index adapter should use the physical relations. Callers
// buffer results until WithCJSIIndexSnapshot succeeds; never cache the Tx.
type CJSIIndexSnapshot struct {
	Tx    *sql.Tx
	stamp CJSIIndexStamp
	delta bool
}

func (s CJSIIndexSnapshot) Selection() CJSIIndexStamp { return s.stamp }

// WithCJSIIndexSnapshot pairs the caller's core snapshot with the checked
// private index. It cannot create either database or publish a selection.
func WithCJSIIndexSnapshot(ctx context.Context, root string, core *sql.Tx, read func(CJSIIndexSnapshot) error) (err error) {
	guarded, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	if core == nil || read == nil || !filepath.IsAbs(root) {
		return errCJSILayoutInvalid
	}
	root = guarded
	corePath := filepath.Join(root, "agentdeck.sqlite3")
	var actual string
	if err = core.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&actual); err != nil || actual != corePath {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	pin, err := admitLegacy(ctx, root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pin.close()) }()
	loc, err := locateCJSISessionIndex(ctx, root, core)
	if err != nil {
		return err
	}
	if loc.route != cjsiRoutePrivate && loc.route != cjsiRouteAbsent {
		return ErrCJSIIndexChanged
	}
	stamp, err := cjsiReadIndexHead(ctx, core)
	if err != nil {
		return err
	}
	j, err := cjsiReadLayoutJournal(ctx, core)
	if err != nil {
		return err
	}
	if (cjsiFileIdentity{pin.root.dev, pin.root.ino}) != j.root {
		return ErrCJSIIndexChanged
	}
	// A *sql.Tx carries no inspectable read-only flag. An uncommitted
	// caller-written head cannot confer permission even inside the callback.
	// This independent committed check can reject, never choose a newer view.
	live, done, err := acquireReadOnlySQL(ctx, corePath, "cjsi-snapshot")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, done()) }()
	committed := stamp
	checkCommitted := func() error {
		t, e := live.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return e
		}
		defer t.Rollback()
		h, e := cjsiReadIndexHead(ctx, t)
		if e != nil {
			return e
		}
		current, e := cjsiReadLayoutJournal(ctx, t)
		if e != nil {
			return e
		}
		if h != committed || current != j {
			return ErrCJSIIndexChanged
		}
		return t.Commit()
	}
	if err = checkCommitted(); err != nil {
		return err
	}
	if loc.route == cjsiRouteAbsent {
		if stamp.IndexEpoch != 0 {
			return ErrCJSIIndexChanged
		}
		return ErrCJSIIndexAbsent
	}
	stamp.Device, stamp.Inode = loc.selection.identity.device, loc.selection.identity.inode
	if stamp.IndexEpoch != loc.selection.epoch || (stamp.BaseTicket == 0 && stamp.BaseSeal != loc.selection.seal.digest) {
		return ErrCJSIIndexChanged
	}
	checkPaths := func() error {
		if e := cjsiCheckInstallRoot(root, j.root); e != nil {
			return e
		}
		if e := cjsiCheckRecordedFence(root, j.fence); e != nil {
			return e
		}
		st, e := privateServingPath(loc.path, false)
		if e != nil || st.Nlink != 1 || cjsiIdentity(st) != loc.selection.identity {
			return ErrCJSIIndexChanged
		}
		return nil
	}
	if err = checkPaths(); err != nil {
		return err
	}
	// The request's private handle ends before its root lease. Do not leave
	// an idle private connection in a pool outside that admission lifetime.
	db, err := sql.Open("sqlite", cjsiSQLiteURI(loc.path, "ro")+"&_pragma=foreign_keys(1)&_pragma=query_only(1)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seal, err := readCJSIProductionSeal(ctx, tx)
	if err != nil || seal != loc.selection.seal {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	var compactTicket, compactAttempt, revision, protection, serving int64
	var compactSeal []byte
	err = tx.QueryRowContext(ctx, `SELECT compacted_ticket,compacted_attempt,compacted_seal,base_revision,protection_revision,g.revision
		FROM main.cjsi_index_state s JOIN main.serving_generation g USING(singleton) WHERE s.singleton=1`).
		Scan(&compactTicket, &compactAttempt, &compactSeal, &revision, &protection, &serving)
	if err != nil || len(compactSeal) != 32 || protection != stamp.ProtectionRevision || revision != serving {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	compacted := compactTicket == stamp.SessionTicket && compactAttempt == stamp.SessionAttempt &&
		[32]byte(compactSeal) == stamp.SessionSeal && revision == stamp.SessionRevision
	if !compacted {
		if stamp.SessionTicket == 0 || compactTicket != stamp.BaseTicket || compactAttempt != stamp.BaseAttempt ||
			[32]byte(compactSeal) != stamp.BaseSeal || revision != stamp.BaseRevision {
			return ErrCJSIIndexChanged
		}
		var ok int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_session_candidates WHERE ticket=? AND attempt=?
			AND core_epoch=? AND index_epoch=? AND base_ticket=? AND base_attempt=? AND base_seal=?
			AND base_revision=? AND session_revision=? AND seal_digest=? AND protection_revision<=? AND coverage_revision<=?`,
			stamp.SessionTicket, stamp.SessionAttempt, stamp.CoreEpoch, stamp.IndexEpoch, stamp.BaseTicket, stamp.BaseAttempt, stamp.BaseSeal[:],
			stamp.BaseRevision, stamp.SessionRevision, stamp.SessionSeal[:], stamp.ProtectionRevision, stamp.CoverageRevision).Scan(&ok)
		if err != nil {
			return errors.Join(ErrCJSIIndexChanged, err)
		}
	}
	if err = checkPaths(); err != nil {
		return err
	}
	if err = read(CJSIIndexSnapshot{Tx: tx, stamp: stamp, delta: !compacted}); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// A protection can invalidate even an already-open ticket. Do not expose
	// buffered results after a revocation while the callback was running.
	var current, currentServing int64
	if err = db.QueryRowContext(ctx, `SELECT protection_revision,g.revision FROM main.cjsi_index_state s
		JOIN main.serving_generation g USING(singleton) WHERE s.singleton=1`).Scan(&current, &currentServing); err != nil || current != protection || currentServing != serving {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	if err = checkCommitted(); err != nil {
		return err
	}
	return checkPaths()
}

func cjsiReadIndexHead(ctx context.Context, q cjsiRowQueryer) (CJSIIndexStamp, error) {
	var s CJSIIndexStamp
	var base, session []byte
	err := q.QueryRowContext(ctx, `SELECT ticket,core_epoch,index_epoch,session_ticket,session_attempt,session_revision,
		base_ticket,base_attempt,base_revision,base_seal,session_seal,protection_revision,coverage_revision
		FROM main.cjsi_publication_head h JOIN main.derived_snapshot_generation g ON g.singleton=h.singleton AND g.epoch=h.core_epoch
		WHERE h.singleton=1`).Scan(&s.Ticket, &s.CoreEpoch, &s.IndexEpoch, &s.SessionTicket, &s.SessionAttempt, &s.SessionRevision,
		&s.BaseTicket, &s.BaseAttempt, &s.BaseRevision, &base, &session, &s.ProtectionRevision, &s.CoverageRevision)
	if err != nil || (s.IndexEpoch != 0 && (len(base) != 32 || len(session) != 32)) {
		return CJSIIndexStamp{}, errors.Join(ErrCJSIIndexChanged, err)
	}
	copy(s.BaseSeal[:], base)
	copy(s.SessionSeal[:], session)
	return s, nil
}

// The returned SELECTs are for this transaction only. Capsule-local integer
// literals are checked keys, never external text; the adapter can still bind
// its own named MATCH/pagination parameters. Physical compaction uses only
// base rows even when the selected logical session ticket is nonzero.
func (s CJSIIndexSnapshot) side() string {
	if !s.delta {
		return "0"
	}
	return fmt.Sprintf("ticket=%d AND attempt=%d", s.stamp.SessionTicket, s.stamp.SessionAttempt)
}

// CopiesSQL yields all eligible physical copies, before Task 5 ranks them.
func (s CJSIIndexSnapshot) CopiesSQL() string {
	side := s.side()
	return `WITH cs AS (SELECT * FROM main.cjsi_candidate_sources WHERE ` + side + `),
	cg AS (SELECT * FROM main.cjsi_candidate_groups WHERE ` + side + `),
	sources AS (SELECT source_path,identity,priority,prefix_hash,parser_version,NULL AS source_kind FROM main.session_sources b
	  WHERE NOT EXISTS(SELECT 1 FROM cs WHERE source_path=b.source_path)
	  UNION ALL SELECT source_path,identity,priority,prefix_hash,parser_version,source_kind FROM cs WHERE op='upsert'),
	groups_ AS (SELECT b.source_path,b.client,b.session_id,m.project,m.model,m.first_at,m.last_at,b.head_at,b.birth,b.sequence_id,b.order_verified,b.source_kind AS base_kind,m.parser_version
	  FROM main.cjsi_base_groups b JOIN main.session_metadata m USING(source_path,client,session_id)
	  WHERE NOT EXISTS(SELECT 1 FROM cg WHERE (source_path,client,session_id)=(b.source_path,b.client,b.session_id))
	  AND NOT EXISTS(SELECT 1 FROM cs JOIN main.session_sources old USING(source_path)
	    WHERE cs.source_path=b.source_path AND (cs.identity IS NOT old.identity OR cs.parser_version IS NOT old.parser_version))
	  UNION ALL SELECT g.source_path,g.client,g.session_id,g.project,g.model,g.first_at,g.last_at,g.head_at,g.birth,g.sequence_id,g.order_verified,b.source_kind,g.parser_version
	    FROM cg g LEFT JOIN main.cjsi_base_groups b USING(source_path,client,session_id) WHERE g.op<>'delete')
	SELECT g.source_path,g.client,g.session_id,g.project,g.model,g.first_at,g.last_at,g.head_at,g.birth,g.sequence_id,g.order_verified,
	  s.identity,s.priority,s.prefix_hash,g.parser_version,coalesce(s.source_kind,g.base_kind) AS source_kind FROM groups_ g JOIN sources s USING(source_path)
	WHERE NOT EXISTS(SELECT 1 FROM main.session_exclusions e WHERE (e.kind='path' AND e.value=g.source_path)
	  OR (e.kind='client' AND e.value=g.client) OR (e.kind='project' AND e.value=g.project) OR (e.kind='session' AND e.value=g.session_id))
	AND NOT EXISTS(SELECT 1 FROM main.cjsi_session_protections p WHERE p.source_path=g.source_path AND p.doc_id=0
	  AND (p.client='' OR (p.client,p.session_id)=(g.client,g.session_id)))`
}

// MembersSQL exposes eligible, side-qualified membership, never unselected
// attempt rows. For every touched group the order relation is complete,
// including retained base IDs (at their new effective location after a move).
// A replace/delete with zero MATCH rows still masks the whole old group.
func (s CJSIIndexSnapshot) MembersSQL() string {
	side := s.side()
	return `WITH copies AS (` + s.CopiesSQL() + `), cg AS (SELECT * FROM main.cjsi_candidate_groups WHERE ` + side + `),
	ord AS (SELECT source_path,client,session_id,doc_id,detail_ordinal,doc_side FROM main.cjsi_candidate_order WHERE ` + side + `),
	members AS (SELECT o.source_path,o.client,o.session_id,o.doc_id,o.detail_ordinal,'base' AS doc_side
	  FROM main.cjsi_base_order o WHERE NOT EXISTS (SELECT 1 FROM cg WHERE (source_path,client,session_id)=(o.source_path,o.client,o.session_id))
	  AND NOT EXISTS (SELECT 1 FROM ord WHERE doc_id=o.doc_id) UNION ALL SELECT * FROM ord)
	SELECT m.source_path,m.client,m.session_id,m.doc_id,
	  row_number() OVER (PARTITION BY m.source_path,m.client,m.session_id ORDER BY m.detail_ordinal) AS detail_ordinal,
	  m.doc_side,CASE WHEN m.doc_side='base' THEN b.source_path ELSE m.source_path END AS payload_path
	FROM members m JOIN copies USING(source_path,client,session_id) LEFT JOIN main.cjsi_base_order b ON b.doc_id=m.doc_id
	WHERE NOT EXISTS(SELECT 1 FROM main.cjsi_candidate_doc_masks d WHERE ` + side + `
	  AND (d.source_path,d.client,d.session_id,d.doc_id)=(m.source_path,m.client,m.session_id,m.doc_id))
	AND NOT EXISTS(SELECT 1 FROM main.cjsi_session_protections p WHERE (p.source_path,p.client,p.session_id,p.doc_id)
	  =(m.source_path,m.client,m.session_id,m.doc_id))
	AND NOT EXISTS(SELECT 1 FROM main.cjsi_session_protections p WHERE m.doc_side='base' AND p.source_path=b.source_path
	  AND (p.client='' OR ((p.client,p.session_id)=(b.client,b.session_id) AND p.doc_id IN (0,b.doc_id))))`
}

// RangesSQL returns only selected, captured native proof. A touched group
// missing its range has no activity authorization, even if its base had one.
// No legacy cursor or digest of indexed text can substitute for this tuple.
func (s CJSIIndexSnapshot) RangesSQL() string {
	const fields = `source_path,client,session_id,workset_round,workset_attempt,identity,selection_epoch,generation,
	  replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest`
	side := s.side()
	return `WITH copies AS (` + s.CopiesSQL() + `), ranges AS (SELECT ` + fields + ` FROM main.cjsi_candidate_ranges WHERE ` + side + `
	 UNION ALL SELECT ` + fields + ` FROM main.cjsi_base_ranges b WHERE NOT EXISTS(SELECT 1 FROM main.cjsi_candidate_groups g
	   WHERE ` + side + ` AND (g.source_path,g.client,g.session_id)=(b.source_path,b.client,b.session_id)))
	 SELECT r.* FROM ranges r JOIN copies c USING(source_path,client,session_id,identity)
	   WHERE c.source_kind='codex-jsonl' AND c.client='codex'`
}

type CJSIDetailIdentity struct {
	Stamp                         CJSIIndexStamp
	SourcePath, Client, SessionID string
	Sequence                      [32]byte
}

// DetailIdentity must be freshly obtained and compared on every reopen.
func (s CJSIIndexSnapshot) DetailIdentity(ctx context.Context, client, id string) (CJSIDetailIdentity, error) {
	v := CJSIDetailIdentity{Stamp: s.stamp, Client: client, SessionID: id}
	var raw []byte
	err := s.Tx.QueryRowContext(ctx, `SELECT source_path,sequence_id FROM (`+s.CopiesSQL()+`)
		WHERE client=? AND session_id=? ORDER BY priority DESC,source_path LIMIT 1`, client, id).Scan(&v.SourcePath, &raw)
	if err != nil {
		return CJSIDetailIdentity{}, err
	}
	if len(raw) != 32 || [32]byte(raw) == ([32]byte{}) {
		return CJSIDetailIdentity{}, ErrCJSIIndexChanged
	}
	v.Sequence = [32]byte(raw)
	return v, nil
}
