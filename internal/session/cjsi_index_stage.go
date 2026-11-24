package session

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

var errIndexStage = errors.New("private session candidate does not match admitted material")

// The root writer supplies both the still-admitted attempt and material bound to
// that exact attempt. Core-only coordinates cannot be established by a private
// SQLite connection. The caller authenticates the exact ordered manifest,
// including planned IDs, workset/selection coordinates and metadata, before
// borrowing the transaction. Seal and sequence identities are opaque here.
type indexStageAttempt struct {
	Parent                           store.CJSIIndexStamp
	Ticket, Attempt, SessionRevision int64
	Seal                             [32]byte
}

type indexStageCandidate struct {
	Attempt        indexStageAttempt
	Replacements   []indexReplacement
	Telemetry      []TelemetrySession
	DeletedSources []indexSourceDelete
	DeletedGroups  []indexGroupDelete
}

type indexSourceDelete struct{ Path, Identity string }
type indexGroupDelete struct {
	Path, Client, SessionID string
	Sequence                [32]byte
}

// One selected native reduction is the entire replacement, including an empty
// reduction. IDs are in reducer order and belong to the authenticated manifest.
// From names the original source only for a verified move with a source delete.
type indexReplacement struct {
	Selected                            ingest.CodexSelectedSource
	Material                            nativeSessionUpdate
	Metadata                            Metadata
	Sequence                            [32]byte
	DocumentIDs, DeletedDocumentIDs     []int64
	From, PrefixHash, ScannedAt         string
	Size, ModifiedAt, ChangedAt         int64
	Round, WorksetAttempt, PublishedEnd int64
}

// stageIndexCandidate exclusively borrows an already admitted transaction.
// Success leaves commit or rollback to its owner. Any error rolls back the
// WHOLE transaction (including prior caller writes), so committing after a
// rejected statement cannot expose earlier payloads, FTS rows or reservations.
// Replaying an occupied ticket/attempt is a deterministic refusal, even if exact.
func stageIndexCandidate(ctx context.Context, tx *sql.Tx, admitted indexStageAttempt, c indexStageCandidate) (err error) {
	if tx == nil {
		return errIndexStage
	}
	err = errIndexStage // Also roll back if staging unwinds before it returns.
	defer func() {
		if err != nil {
			if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
				err = errors.Join(err, e)
			}
		}
	}()
	return c.write(ctx, tx, admitted)
}

func (c *indexStageCandidate) write(ctx context.Context, tx *sql.Tx, admitted indexStageAttempt) error {
	h := admitted.Parent
	if admitted != c.Attempt || admitted.Seal == ([32]byte{}) || h.SessionSeal == ([32]byte{}) ||
		h.CoreEpoch <= 0 || h.IndexEpoch <= 0 || h.Ticket < h.SessionTicket ||
		admitted.Ticket <= h.Ticket || admitted.Attempt <= 0 || admitted.SessionRevision < h.SessionRevision ||
		len(c.Replacements)+len(c.Telemetry)+len(c.DeletedGroups)+len(c.DeletedSources) == 0 {
		return errIndexStage
	}
	// Only a fully compacted selected component can be the physical base of
	// this delta. A core-only parent ticket may carry that component forward.
	err := stageOne(ctx, tx, `INSERT INTO main.cjsi_session_candidates
		(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,
		 parent_ticket,protection_revision,coverage_revision,session_revision)
		SELECT ?,?,?,g.epoch,s.compacted_ticket,s.compacted_attempt,s.compacted_seal,s.base_revision,?,s.protection_revision,?,?
		FROM main.cjsi_index_state s JOIN main.serving_generation v USING(singleton)
		JOIN main.session_index_generation g USING(singleton) JOIN main.cjsi_layout_initial_seal i USING(singleton)
		WHERE s.singleton=1 AND g.epoch=? AND i.index_epoch=g.epoch AND s.compacted_ticket=? AND s.compacted_attempt=?
		  AND s.compacted_seal=? AND (s.compacted_ticket<>0 OR i.seal_digest=s.compacted_seal)
		  AND s.base_revision=? AND v.revision=s.base_revision AND s.protection_revision=?
		  AND (SELECT foreign_keys FROM pragma_foreign_keys)=1`,
		admitted.Ticket, admitted.Attempt, h.CoreEpoch, h.Ticket, h.CoverageRevision, admitted.SessionRevision,
		h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.ProtectionRevision)
	if err != nil {
		return err
	}
	retired, err := c.stageSourceRemovals(ctx, tx, admitted)
	if err != nil {
		return err
	}
	for _, g := range c.DeletedGroups {
		if g.Path == "" || g.Client == "" || g.SessionID == "" || g.Sequence == ([32]byte{}) {
			return errIndexStage
		}
		if err := stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,sequence_id)
			SELECT ?,?,source_path,client,session_id,'delete',? FROM main.cjsi_base_groups
			WHERE source_path=? AND client=? AND session_id=?`, admitted.Ticket, admitted.Attempt, g.Sequence[:],
			g.Path, g.Client, g.SessionID); err != nil {
			return err
		}
	}
	for _, projection := range c.Telemetry {
		if err := stageCJSITelemetry(ctx, tx, admitted, projection); err != nil {
			return err
		}
	}
	for _, r := range c.Replacements {
		if err := r.stage(ctx, tx, admitted, retired); err != nil {
			return err
		}
	}
	return stageOne(ctx, tx, `UPDATE main.cjsi_session_candidates SET seal_digest=?
		WHERE ticket=? AND attempt=? AND seal_digest IS NULL`, admitted.Seal[:], admitted.Ticket, admitted.Attempt)
}

// The seal includes every old (path, identity). If a different inode takes
// the same path, its upsert masks the old source in the selected snapshot.
// The index has one physical source operation per path and attempt.
func (c *indexStageCandidate) stageSourceRemovals(ctx context.Context, tx *sql.Tx, a indexStageAttempt) (map[string]string, error) {
	replacements := make(map[string]string, len(c.Replacements))
	for _, r := range c.Replacements {
		replacements[r.Selected.Path] = r.Selected.Evidence.Identity
	}
	retired := make(map[string]string, len(c.DeletedSources))
	for _, s := range c.DeletedSources {
		if s.Path == "" || s.Identity == "" {
			return nil, errIndexStage
		}
		if replacement, reused := replacements[s.Path]; reused {
			var matches bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM main.session_sources WHERE source_path=? AND identity=?)", s.Path, s.Identity).Scan(&matches); err != nil {
				return nil, err
			}
			if !matches || replacement == s.Identity {
				return nil, errIndexStage
			}
		} else if err := stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_sources(ticket,attempt,source_path,op)
			SELECT ?,?,source_path,'delete' FROM main.session_sources WHERE source_path=? AND identity=?`, a.Ticket, a.Attempt, s.Path, s.Identity); err != nil {
			return nil, err
		}
		// Two removed aliases of a single inode do not prove one move origin.
		if prior, ok := retired[s.Identity]; ok && prior != s.Path {
			retired[s.Identity] = ""
		} else {
			retired[s.Identity] = s.Path
		}
	}
	return retired, nil
}

// A false storage precondition or an ignored write cannot count as success.
func stageOne(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errIndexStage
	}
	return nil
}

func (r *indexReplacement) valid() bool {
	m, s, md := &r.Material, &r.Selected, &r.Metadata
	e, p, k := s.Evidence, s.Evidence.Progress, m.Checkpoint
	if md.SourcePath == "" || md.SourcePath != s.Path || md.Client != "codex" || md.SessionID == "" ||
		md.SessionID != s.ThreadID || m.Source.State != ingest.CodexSourceVerified || m.Source.ThreadID != s.ThreadID ||
		m.ParserVersion != nativeSessionParserVersion || s.Epoch <= 0 || e != m.Evidence ||
		e.Identity == "" || e.Digest == ([32]byte{}) || e.Version.Generation <= 0 || e.Version.Parser <= 0 ||
		e.MaxLineBytes <= 0 || e.Version.MaxLineBytes != e.MaxLineBytes || r.Round <= 0 || r.WorksetAttempt <= 0 ||
		p.ReadEnd > r.Size || p.CompleteEnd <= 0 || p.CompleteEnd > p.ReadEnd || p.Partial != (p.CompleteEnd < p.ReadEnd) ||
		k.ReadEnd != p.ReadEnd || k.CompleteEnd != p.CompleteEnd || k.Partial != p.Partial ||
		k.ReplayFrom < 0 || k.ReplayFrom > k.CompleteEnd || r.PublishedEnd != k.CompleteEnd ||
		r.From == s.Path || r.Sequence == ([32]byte{}) || r.ScannedAt == "" || len(m.Documents) != len(r.DocumentIDs) {
		return false
	}
	var end int64
	for i, doc := range m.Documents {
		if doc.Ordinal != i+1 || doc.Client != md.Client || doc.SessionID != md.SessionID ||
			(doc.Kind != "user_prompt" && doc.Kind != "assistant_final") || doc.Span.Start < end ||
			doc.Span.End <= doc.Span.Start || doc.Span.End > r.PublishedEnd || r.DocumentIDs[i] <= 0 {
			return false
		}
		end = doc.Span.End
	}
	return true
}

func (r *indexReplacement) stage(ctx context.Context, tx *sql.Tx, a indexStageAttempt, retired map[string]string) error {
	if !r.valid() {
		return errIndexStage
	}
	s, e, m := r.Selected, r.Material.Evidence, &r.Metadata
	origin := s.Path
	if r.From != "" {
		origin = r.From
	}
	retiredPath, removed := retired[e.Identity]
	if (r.From != "" && !removed) || (removed && retiredPath != origin) {
		return errIndexStage
	}
	// Changing a source can hide sibling base groups. Each such disappearance
	// needs an explicit tombstone. Reject protected groups at either end of a
	// move, including doc-specific holds that a fresh ID could otherwise evade.
	err := stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_sources
		(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at)
		SELECT ?,?,?,'upsert','codex-jsonl',?,?,?,?,?,?,?,?,?
		WHERE NOT EXISTS(SELECT 1 FROM main.cjsi_session_protections WHERE source_path IN (?,?)
			AND (client='' OR (client='codex' AND session_id=?)))
		AND NOT EXISTS(SELECT 1 FROM main.session_exclusions x WHERE (x.kind='path' AND x.value IN (?,?))
			OR (x.kind='client' AND x.value='codex') OR (x.kind='session' AND x.value=?)
			OR (x.kind='project' AND (x.value=? OR EXISTS(SELECT 1 FROM main.session_metadata b WHERE
				b.source_path IN (?,?) AND b.client='codex' AND b.session_id=? AND b.project=x.value))))
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_base_groups b WHERE b.source_path=? AND (b.client,b.session_id)<>('codex',?)
			AND NOT EXISTS(SELECT 1 FROM main.cjsi_candidate_groups d WHERE d.ticket=? AND d.attempt=? AND d.op='delete'
				AND (d.source_path,d.client,d.session_id)=(b.source_path,b.client,b.session_id)))`,
		a.Ticket, a.Attempt, s.Path, e.Identity, m.projectPriority, r.Material.ParserVersion, e.Progress.CompleteEnd,
		r.Size, r.ModifiedAt, r.ChangedAt, r.PrefixHash, r.ScannedAt, s.Path, origin, s.ThreadID,
		s.Path, origin, s.ThreadID, m.Project, s.Path, origin, s.ThreadID, s.Path, s.ThreadID, a.Ticket, a.Attempt)
	if err != nil {
		return err
	}
	err = stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_groups
		(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,order_verified,sequence_id,head_at,birth)
		VALUES(?,?,?,'codex',?,'replace',?,?,?,?,?,1,?,?,?)`, a.Ticket, a.Attempt, s.Path, s.ThreadID,
		m.Project, m.Model, r.Material.ParserVersion, m.FirstAt, m.LastAt, r.Sequence[:],
		stageTimestamp(r.Material.Source.HeadTime), stageTimestamp(r.Material.Source.Birth))
	if err != nil {
		return err
	}
	for i, d := range r.Material.Documents {
		if err := r.stageMember(ctx, tx, a, origin, d, r.DocumentIDs[i]); err != nil {
			return err
		}
	}
	for _, id := range r.DeletedDocumentIDs {
		if err := stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_doc_masks(ticket,attempt,source_path,client,session_id,doc_id)
			SELECT ?,?,?,b.client,b.session_id,b.doc_id FROM main.cjsi_base_order b
			JOIN main.session_documents d ON d.rowid=b.doc_id AND (d.source_path,d.client,d.session_id)=(b.source_path,b.client,b.session_id)
			WHERE b.source_path=? AND b.client='codex' AND b.session_id=? AND b.doc_id=?`,
			a.Ticket, a.Attempt, s.Path, origin, s.ThreadID, id); err != nil {
			return err
		}
	}
	return stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_ranges
		(ticket,attempt,source_path,client,session_id,workset_round,workset_attempt,identity,selection_epoch,generation,
		 replay_parser,max_line_bytes,read_end,complete_end,replay_from,published_end,prefix_digest)
		VALUES(?,?,?,'codex',?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.Ticket, a.Attempt, s.Path, s.ThreadID, r.Round,
		r.WorksetAttempt, e.Identity, s.Epoch, e.Version.Generation, e.Version.Parser, e.MaxLineBytes, e.Progress.ReadEnd,
		e.Progress.CompleteEnd, r.Material.Checkpoint.ReplayFrom, r.PublishedEnd, e.Digest[:])
}

func stageTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func (r *indexReplacement) baseDocument(ctx context.Context, tx *sql.Tx, origin string, d nativeSessionDocument) (int64, error) {
	e := r.Material.Evidence
	// Only a verified record key from the same source generation can retain
	// an ID. Text equality is checked after lookup and never identifies a row.
	var baseID, end int64
	var eventAt, text string
	err := tx.QueryRowContext(ctx, `SELECT b.doc_id,b.record_end,p.event_at,p.text
		FROM main.cjsi_base_order b JOIN main.cjsi_base_groups g USING(source_path,client,session_id)
		JOIN main.cjsi_base_ranges r USING(source_path,client,session_id) JOIN main.session_sources s USING(source_path)
		JOIN main.session_documents p ON p.rowid=b.doc_id AND (p.source_path,p.client,p.session_id)=(b.source_path,b.client,b.session_id)
		WHERE b.source_path=? AND b.client='codex' AND b.session_id=? AND g.order_verified=1 AND g.source_kind='codex-jsonl'
		  AND r.identity=? AND s.identity=r.identity AND r.generation=? AND r.replay_parser=? AND r.max_line_bytes=?
		  AND b.record_start=? AND b.emission=0 AND b.record_end<=r.published_end AND p.kind=?`,
		origin, d.SessionID, e.Identity, e.Version.Generation, e.Version.Parser, e.MaxLineBytes,
		d.Span.Start, d.Kind).Scan(&baseID, &end, &eventAt, &text)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if end != d.Span.End || eventAt != d.EventAt || text != d.Text {
		return 0, errIndexStage
	}
	return baseID, nil
}

func (r *indexReplacement) stageMember(ctx context.Context, tx *sql.Tx, a indexStageAttempt, origin string, d nativeSessionDocument, id int64) error {
	baseID, err := r.baseDocument(ctx, tx, origin, d)
	if err != nil {
		return err
	}
	side := "base"
	switch {
	case baseID == 0:
		first, _, err := store.ReserveCJSIDocumentIDs(ctx, tx, 1)
		if err != nil {
			return err
		}
		if first != id {
			return errIndexStage
		}
		if err = stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_payloads(doc_id,event_at,kind,text) VALUES(?,?,?,?)`,
			id, d.EventAt, d.Kind, d.Text); err != nil {
			return err
		}
		side = "candidate"
	case baseID != id:
		return errIndexStage
	}
	return stageOne(ctx, tx, `INSERT INTO main.cjsi_candidate_order
		(ticket,attempt,source_path,client,session_id,doc_id,doc_side,detail_ordinal,record_start,record_end,emission)
		VALUES(?,?,?,'codex',?,?,?,?,?,?,0)`, a.Ticket, a.Attempt, r.Selected.Path, d.SessionID, id, side,
		d.Ordinal, d.Span.Start, d.Span.End)
}
