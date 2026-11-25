package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// CJSISourceInput describes a discovered native file and the sealed workset
// for its selected prefix. Project metadata is read from the proved own head.
type CJSISourceInput struct {
	Selected                                           ingest.CodexSelectedSource
	MovedFrom, PrefixHash, ScannedAt                   string
	Priority                                           int
	Size, ModifiedAt, ChangedAt, Round, WorksetAttempt int64
}

// CJSIPreparedSource owns only approved material. The caller cannot substitute
// documents between verification and planning.
type CJSIPreparedSource struct{ replacement indexReplacement }

// PrepareCJSISource joins an independent capture to the sealed proof outside
// the index write transaction. A budget refusal publishes no partial material.
func PrepareCJSISource(ctx context.Context, file *os.File, replay *ingest.CodexCheckpointReplay, limits ingest.CodexWorksetLimits, maxBytes int64, input CJSISourceInput, current ingest.CodexCurrentSelection) (CJSIPreparedSource, error) {
	selected := input.Selected
	end := selected.Evidence.Progress.ReadEnd
	if file == nil || replay == nil || current == nil || end <= 0 || end > input.Size {
		return CJSIPreparedSource{}, ingest.ErrSourceChanged
	}
	if end > maxBytes {
		return CJSIPreparedSource{}, ingest.ErrCodexWorksetBudget
	}
	info, err := file.Stat()
	if err != nil {
		return CJSIPreparedSource{}, err
	}
	identity, changed, stable := ingest.FileGeneration(info)
	if !stable || identity != selected.Evidence.Identity || info.Size() < end {
		return CJSIPreparedSource{}, ingest.ErrSourceChanged
	}
	observed := ingest.Source{Path: selected.Path, Identity: identity, Size: info.Size(), ChangedAt: changed, ModifiedAt: info.ModTime().UnixNano(), Stable: true}
	material, err := captureSessionMaterial(ctx, file, selected.Path, end, selected.Evidence.MaxLineBytes)
	if err != nil {
		return CJSIPreparedSource{}, err
	}
	update, err := acceptSessionMaterial(ctx, replay, limits, material, selected, current)
	if err != nil {
		return CJSIPreparedSource{}, err
	}
	first := update.Source.Birth
	last := update.LastAt
	if last.Before(first) {
		last = first
	}
	project, err := ownHeadProject(file, selected, update.Source.Head)
	if err != nil {
		return CJSIPreparedSource{}, err
	}
	if err := ingest.Validate(observed); err != nil {
		return CJSIPreparedSource{}, err
	}
	m := Metadata{Client: "codex", SessionID: selected.ThreadID, SourcePath: selected.Path, Project: project,
		projectPriority: input.Priority, Model: update.Model, FirstAt: stageTimestamp(first), LastAt: stageTimestamp(last)}
	return CJSIPreparedSource{indexReplacement{Selected: selected, Material: update, Metadata: m, From: input.MovedFrom,
		Size: input.Size, ModifiedAt: input.ModifiedAt, ChangedAt: input.ChangedAt, PrefixHash: input.PrefixHash, ScannedAt: input.ScannedAt,
		Round: input.Round, WorksetAttempt: input.WorksetAttempt, PublishedEnd: update.Checkpoint.CompleteEnd}}, nil
}

// A verified single-identity legacy file may have no metadata record.
func ownHeadProject(file *os.File, selected ingest.CodexSelectedSource, head ingest.CodexSpan) (string, error) {
	if head == (ingest.CodexSpan{}) {
		return "", nil
	}
	if head.Start < 0 || head.End <= head.Start || head.End > selected.Evidence.Progress.ReadEnd || head.End-head.Start > int64(selected.Evidence.MaxLineBytes)+1 {
		return "", ingest.ErrSourceChanged
	}
	raw := make([]byte, head.End-head.Start)
	if _, err := file.ReadAt(raw, head.Start); err != nil {
		return "", err
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		return "", err
	}
	payload, _ := record["payload"].(map[string]any)
	return meta("codex", selected.ThreadID, payload, record).Project, nil
}

// CJSISourceRemoval is an inventory-proved removal, move or quarantined source.
type CJSISourceRemoval struct{ Path, Identity string }

// CJSIIndexCandidate is an immutable plan bound to the selected parent and
// its reserved ticket/attempt. Only the owning coordinator commits the write.
type CJSIIndexCandidate struct {
	plan             indexStageCandidate
	changed, removed int
}

func (c CJSIIndexCandidate) Seal() [32]byte { return c.plan.Attempt.Seal }

// Withdrawals returns the logical removals authenticated by Seal, including
// moves and exclusions that the planner discovered while reading the base.
func (c CJSIIndexCandidate) Withdrawals() []CJSISourceRemoval {
	result := make([]CJSISourceRemoval, len(c.plan.DeletedSources))
	for i, old := range c.plan.DeletedSources {
		result[i] = CJSISourceRemoval(old)
	}
	return result
}

// DocumentChanges reports the public changed and removed document counts.
func (c CJSIIndexCandidate) DocumentChanges() (int, int) { return c.changed, c.removed }

// Encoding uses ordered structs and slices; path, source priority, ordered
// membership, parser evidence and content all participate in the plan seal.
func indexDigest(value any) ([32]byte, error) {
	h := sha256.New()
	if err := json.NewEncoder(h).Encode(value); err != nil {
		return [32]byte{}, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// PlanCJSIIndex reads only the admitted private snapshot. telemetry, when
// present, is the complete settled OTel projection for the proposed ticket.
// A nil candidate means no changes were supplied or the projection is current.
func PlanCJSIIndex(ctx context.Context, snapshot store.CJSIIndexSnapshot, ticket, attempt, revision int64, prepared []CJSIPreparedSource, removed []CJSISourceRemoval, telemetry *[]TelemetrySession) (*CJSIIndexCandidate, error) {
	return planCJSIIndex(ctx, snapshot, ticket, attempt, revision, prepared, removed, telemetry, false)
}

// RecoverCJSIIndex reconstructs a previously staged candidate using independent
// source captures and the original compacted parent. Only the sealed attempt's
// verified membership can supply allocated IDs. A missing attempt permits new
// planning; an existing, inconsistent attempt must never take that fallback.
func RecoverCJSIIndex(ctx context.Context, snapshot store.CJSIIndexSnapshot, ticket, attempt, revision int64, prepared []CJSIPreparedSource, removed []CJSISourceRemoval, telemetry *[]TelemetrySession) (*CJSIIndexCandidate, bool, error) {
	h := snapshot.Selection()
	if snapshot.Tx == nil || ticket <= h.Ticket || attempt <= 0 || revision < h.SessionRevision {
		return nil, false, errIndexStage
	}
	var digest []byte
	var matches bool
	err := snapshot.Tx.QueryRowContext(ctx, `SELECT c.seal_digest,EXISTS(
    SELECT 1 FROM cjsi_index_state s JOIN serving_generation g USING(singleton) JOIN session_index_generation i USING(singleton)
    WHERE s.singleton=1 AND c.parent_ticket=? AND c.core_epoch=? AND c.index_epoch=? AND i.epoch=c.index_epoch
    AND c.base_ticket=? AND c.base_attempt=? AND c.base_seal=? AND c.base_revision=?
    AND (s.compacted_ticket,s.compacted_attempt,s.compacted_seal,s.base_revision)=(c.base_ticket,c.base_attempt,c.base_seal,c.base_revision)
    AND g.revision=s.base_revision AND c.protection_revision=s.protection_revision AND c.protection_revision=?
    AND c.coverage_revision=? AND c.session_revision=?)
    FROM cjsi_session_candidates c WHERE c.ticket=? AND c.attempt=?`, h.Ticket, h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.ProtectionRevision, h.CoverageRevision, revision, ticket, attempt).Scan(&digest, &matches)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !matches || len(digest) != 32 || [32]byte(digest) == ([32]byte{}) {
		return nil, true, errIndexStage
	}
	c, err := planCJSIIndex(ctx, snapshot, ticket, attempt, revision, prepared, removed, telemetry, true)
	if err != nil {
		return nil, true, err
	}
	if c == nil || c.Seal() != [32]byte(digest) {
		return nil, true, errIndexStage
	}
	documents := 0
	for _, r := range c.plan.Replacements {
		documents += len(r.DocumentIDs)
	}
	var extra bool
	if err = snapshot.Tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM cjsi_candidate_order WHERE ticket=? AND attempt=? LIMIT 1 OFFSET ?)", ticket, attempt, documents).Scan(&extra); err != nil {
		return nil, true, err
	}
	if extra {
		return nil, true, errIndexStage
	}
	return c, true, nil
}

func recoveredDocumentID(ctx context.Context, tx *sql.Tx, a indexStageAttempt, r indexReplacement, d nativeSessionDocument, baseID, water int64) (int64, error) {
	var id int64
	// baseDocument has independently verified every reused base row. Newly
	// allocated IDs must point at an exact payload in this attempt's ordinal.
	err := tx.QueryRowContext(ctx, `SELECT o.doc_id FROM cjsi_candidate_order o LEFT JOIN cjsi_candidate_payloads p ON p.doc_id=o.payload_id
 WHERE o.ticket=? AND o.attempt=? AND o.source_path=? AND o.client='codex' AND o.session_id=? AND o.detail_ordinal=?
 AND o.record_start=? AND o.record_end=? AND o.emission=0 AND o.doc_id BETWEEN 1 AND ?
 AND ((o.doc_side='base' AND o.doc_id=?) OR (o.doc_side='candidate' AND ?=0 AND p.event_at=? AND p.kind=? AND p.text=?))`,
		a.Ticket, a.Attempt, r.Selected.Path, d.SessionID, d.Ordinal, d.Span.Start, d.Span.End, water, baseID, baseID, d.EventAt, d.Kind, d.Text).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errIndexStage
	}
	return id, err
}

func verifyRecoveredSource(ctx context.Context, tx *sql.Tx, a indexStageAttempt, r indexReplacement) error {
	e, s, m := r.Material.Evidence, r.Selected, r.Metadata
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM cjsi_candidate_sources s JOIN cjsi_candidate_groups g USING(ticket,attempt,source_path)
 JOIN cjsi_candidate_ranges r USING(ticket,attempt,source_path,client,session_id)
 WHERE s.ticket=? AND s.attempt=? AND s.source_path=? AND g.client='codex' AND g.session_id=? AND s.op='upsert' AND s.source_kind='codex-jsonl'
 AND s.identity=? AND s.priority=? AND s.parser_version=? AND s.complete_end=? AND s.size=? AND s.modified_at=? AND s.changed_at=? AND s.prefix_hash=? AND s.scanned_at=?
 AND g.op='replace' AND g.order_verified=1 AND g.sequence_id=? AND g.project=? AND g.model=? AND g.parser_version=? AND g.first_at=? AND g.last_at=? AND g.head_at=? AND g.birth=?
 AND r.workset_round=? AND r.workset_attempt=? AND r.identity=s.identity AND r.selection_epoch=? AND r.generation=? AND r.replay_parser=?
 AND r.max_line_bytes=? AND r.read_end=? AND r.complete_end=? AND r.replay_from=? AND r.published_end=? AND r.prefix_digest=?`,
		a.Ticket, a.Attempt, s.Path, s.ThreadID, e.Identity, m.projectPriority, r.Material.ParserVersion, e.Progress.CompleteEnd, r.Size, r.ModifiedAt, r.ChangedAt, r.PrefixHash, r.ScannedAt,
		r.Sequence[:], m.Project, m.Model, r.Material.ParserVersion, m.FirstAt, m.LastAt, stageTimestamp(r.Material.Source.HeadTime), stageTimestamp(r.Material.Source.Birth),
		r.Round, r.WorksetAttempt, s.Epoch, e.Version.Generation, e.Version.Parser, e.MaxLineBytes, e.Progress.ReadEnd, e.Progress.CompleteEnd, r.Material.Checkpoint.ReplayFrom, r.PublishedEnd, e.Digest[:]).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return errIndexStage
	}
	return err
}

func planCJSIIndex(ctx context.Context, snapshot store.CJSIIndexSnapshot, ticket, attempt, revision int64, prepared []CJSIPreparedSource, removed []CJSISourceRemoval, telemetry *[]TelemetrySession, recoverIDs bool) (*CJSIIndexCandidate, error) {
	h := snapshot.Selection()
	if snapshot.Tx == nil || ticket <= h.Ticket || attempt <= 0 || revision < h.SessionRevision {
		return nil, errIndexStage
	}
	c := indexStageCandidate{Attempt: indexStageAttempt{Parent: h, Ticket: ticket, Attempt: attempt, SessionRevision: revision}}
	for _, s := range removed {
		c.DeletedSources = append(c.DeletedSources, indexSourceDelete(s))
	}
	if telemetry != nil {
		if err := c.planTelemetry(ctx, snapshot.Tx, *telemetry); err != nil {
			return nil, err
		}
	}
	var water int64
	if err := snapshot.Tx.QueryRowContext(ctx, `SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1 AND compacted_ticket=? AND compacted_attempt=? AND compacted_seal=? AND base_revision=?`, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision).Scan(&water); err != nil {
		return nil, err
	}
	sources := slices.Clone(prepared)
	slices.SortFunc(sources, func(a, b CJSIPreparedSource) int {
		return strings.Compare(a.replacement.Selected.Path, b.replacement.Selected.Path)
	})
	var priorities []int
	for _, source := range sources {
		r := source.replacement
		skip, err := excluded(ctx, snapshot.Tx, r.Metadata)
		if err != nil {
			return nil, err
		}
		if skip {
			// Persist no approved text when the user excluded this source.
			for _, path := range []string{r.From, r.Selected.Path} {
				if path == "" {
					continue
				}
				var identity string
				err := snapshot.Tx.QueryRowContext(ctx, "SELECT identity FROM session_sources WHERE source_path=?", path).Scan(&identity)
				if err == nil {
					c.DeletedSources = append(c.DeletedSources, indexSourceDelete{path, identity})
				} else if !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
			}
			continue
		}
		origin := r.Selected.Path
		if r.From != "" {
			origin = r.From
			c.DeletedSources = append(c.DeletedSources, indexSourceDelete{r.From, r.Material.Evidence.Identity})
		}
		for _, d := range r.Material.Documents {
			id, err := r.baseDocument(ctx, snapshot.Tx, origin, d)
			if err != nil {
				return nil, err
			}
			if recoverIDs {
				id, err = recoveredDocumentID(ctx, snapshot.Tx, c.Attempt, r, d, id, water)
				if err != nil {
					return nil, err
				}
			} else if id == 0 {
				if water == math.MaxInt64 {
					return nil, errIndexStage
				}
				water++
				id = water
			}
			r.DocumentIDs = append(r.DocumentIDs, id)
		}
		seq, err := indexDigest(r.DocumentIDs)
		if err != nil {
			return nil, err
		}
		r.Sequence = seq
		if recoverIDs {
			if err := verifyRecoveredSource(ctx, snapshot.Tx, c.Attempt, r); err != nil {
				return nil, err
			}
		}
		if !r.valid() {
			return nil, errIndexStage
		}
		// Replacing this physical source also retires any inherited sibling
		// identities from the old parser, including zero-document groups.
		rows, err := snapshot.Tx.QueryContext(ctx, `SELECT source_path,client,session_id,sequence_id FROM cjsi_base_groups WHERE source_path=? AND (client,session_id)<>(?,?) ORDER BY client,session_id`, r.Selected.Path, r.Metadata.Client, r.Metadata.SessionID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var g indexGroupDelete
			var seq []byte
			if err = rows.Scan(&g.Path, &g.Client, &g.SessionID, &seq); err != nil {
				rows.Close()
				return nil, err
			}
			if len(seq) != len(g.Sequence) {
				rows.Close()
				return nil, errIndexStage
			}
			copy(g.Sequence[:], seq)
			c.DeletedGroups = append(c.DeletedGroups, g)
		}
		if err = errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
		c.Replacements = append(c.Replacements, r)
		priorities = append(priorities, r.Metadata.projectPriority)
	}
	if len(c.Replacements)+len(c.DeletedSources)+len(c.Telemetry) == 0 {
		return nil, nil
	}
	slices.SortFunc(c.DeletedSources, func(a, b indexSourceDelete) int {
		if n := strings.Compare(a.Path, b.Path); n != 0 {
			return n
		}
		return strings.Compare(a.Identity, b.Identity)
	})
	c.DeletedSources = slices.Compact(c.DeletedSources)
	seal, err := indexDigest(struct {
		Domain     string
		Plan       indexStageCandidate
		Priorities []int
	}{"agentdeck/session-index/v1", c, priorities})
	if err != nil {
		return nil, err
	}
	c.Attempt.Seal = seal
	changed, removedDocs, err := c.affectedChanges(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	return &CJSIIndexCandidate{c, changed, removedDocs}, nil
}

// Stage borrows a caller-admitted private transaction. A failed borrow rolls
// back the whole transaction. The coordinator authenticates the planned seal.
func (c CJSIIndexCandidate) Stage(ctx context.Context, tx *sql.Tx, parent store.CJSIIndexStamp, ticket, attempt, revision int64, seal [32]byte) error {
	return stageIndexCandidate(ctx, tx, indexStageAttempt{Parent: parent, Ticket: ticket, Attempt: attempt, SessionRevision: revision, Seal: seal}, c.plan)
}

// CompactCJSIIndex also works after restart using only the admitted parent,
// selected stamp and the sealed private tables.
func CompactCJSIIndex(ctx context.Context, tx *sql.Tx, parent, selected store.CJSIIndexStamp, current func(context.Context) (store.CJSIIndexStamp, error)) error {
	return compactIndexCandidate(ctx, tx, indexStageAttempt{Parent: parent, Ticket: selected.SessionTicket, Attempt: selected.SessionAttempt, SessionRevision: selected.SessionRevision, Seal: selected.SessionSeal}, selected, current)
}
