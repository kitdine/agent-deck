package session

import (
	"context"
	"database/sql"
	"os"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

// CJSIReader borrows the request's core transaction. Each operation opens its
// matching private snapshot and releases it before returning any result.
type CJSIReader struct {
	with func(context.Context, func(indexReadView) error) error
}

// NewCJSIReader neither selects a newer core ticket nor opens legacy state.
// limits is the caller's independent budget for on-demand Codex activity.
func NewCJSIReader(root string, core *sql.Tx, limits ingest.CodexSelectedLimits) CJSIReader {
	return CJSIReader{with: func(ctx context.Context, read func(indexReadView) error) error {
		return store.WithCJSIIndexSnapshot(ctx, root, core, func(s store.CJSIIndexSnapshot) error {
			return read(privateIndexView(s, limits))
		})
	}}
}

func (r CJSIReader) snapshot(ctx context.Context) withIndexSnapshot {
	return func(read func(indexReadView) error) error {
		if r.with == nil {
			return store.ErrCJSIIndexChanged
		}
		return r.with(ctx, read)
	}
}

func privateIndexView(s store.CJSIIndexSnapshot, limits ingest.CodexSelectedLimits) indexReadView {
	// IDs identify payloads; the selected membership supplies their logical
	// group and ordinal. A moved base payload still lives at payload_path.
	const fields = `SELECT m.source_path,m.client,m.session_id,m.doc_id,m.detail_ordinal,d.event_at,d.kind,d.text `
	members := `WITH eligible AS (` + s.MembersSQL() + `) `
	base := fields + `FROM eligible m JOIN main.session_documents d ON d.rowid=m.doc_id AND d.source_path=m.payload_path WHERE m.doc_side='base'`
	candidate := fields + `FROM eligible m JOIN main.cjsi_candidate_payloads d ON d.doc_id=m.doc_id WHERE m.doc_side='candidate'`
	candidateMatch := fields + `FROM eligible m JOIN main.cjsi_candidate_documents d ON d.rowid=m.doc_id WHERE m.doc_side='candidate' AND cjsi_candidate_documents MATCH @query`
	return indexReadView{
		stamp: s.Selection(), tx: s.Tx, copies: s.CopiesSQL(),
		documents:      members + base + ` UNION ALL ` + candidate,
		matches:        members + base + ` AND session_documents MATCH @query UNION ALL ` + candidateMatch,
		detailIdentity: s.DetailIdentity,
		activity: func(ctx context.Context, m Metadata) ([]activity.Detail, error) {
			if m.Client == "claude" {
				return readCompatibleActivity(ctx, s, m, func(file *os.File, end int64) ([]activity.Detail, error) {
					return activity.ReadDetailsPrefix(file, end, m.Client, m.SessionID)
				})
			}
			selected, current, err := activitySelection(ctx, s, m)
			if err != nil {
				return nil, err
			}
			return activity.ReadSelectedCodexDetails(ctx, selected, current, limits)
		},
		activityPage: func(ctx context.Context, m Metadata, page, limit int, all bool) (activity.Page, error) {
			if IsTelemetrySource(m.SourcePath) {
				// OTel has no captured tool records. The in-memory reader
				// paginates an empty capture without opening a source file.
				return activity.CodexDetailsPage(m.SessionID, ingest.CodexPrefixProof{}, ingest.CodexCapturedReduction{}, page, limit, all)
			}
			if m.Client == "claude" {
				return readCompatibleActivity(ctx, s, m, func(file *os.File, end int64) (activity.Page, error) {
					return activity.ReadDetailsPagePrefix(file, end, m.Client, m.SessionID, page, limit, all)
				})
			}
			selected, current, err := activitySelection(ctx, s, m)
			if err != nil {
				return activity.Page{}, err
			}
			return activity.ReadSelectedCodexDetailsPage(ctx, selected, current, limits, page, limit, all)
		},
	}
}

func activitySelection(ctx context.Context, s store.CJSIIndexSnapshot, m Metadata) (ingest.CodexSelectedSource, ingest.CodexCurrentSelection, error) {
	current := func(ctx context.Context) (ingest.CodexSelectedSource, error) {
		return selectedActivitySource(ctx, s, m)
	}
	selected, err := current(ctx)
	return selected, current, err
}

func selectedActivitySource(ctx context.Context, s store.CJSIIndexSnapshot, m Metadata) (ingest.CodexSelectedSource, error) {
	var src ingest.CodexSelectedSource
	e := &src.Evidence
	var digest []byte
	// The C1 activity reader emits through CompleteEnd. A shorter published
	// range cannot authorize that read; do not truncate and reuse its digest.
	err := s.Tx.QueryRowContext(ctx, `SELECT source_path,session_id,identity,selection_epoch,generation,replay_parser,max_line_bytes,read_end,complete_end,prefix_digest
		FROM (`+s.RangesSQL()+`) WHERE source_path=? AND client=? AND session_id=? AND published_end=complete_end`,
		m.SourcePath, m.Client, m.SessionID).Scan(&src.Path, &src.ThreadID, &e.Identity, &src.Epoch, &e.Version.Generation, &e.Version.Parser, &e.MaxLineBytes, &e.Progress.ReadEnd, &e.Progress.CompleteEnd, &digest)
	if err != nil {
		return ingest.CodexSelectedSource{}, err
	}
	if len(digest) != len(e.Digest) {
		return ingest.CodexSelectedSource{}, ingest.ErrSourceChanged
	}
	copy(e.Digest[:], digest)
	e.Version.MaxLineBytes = e.MaxLineBytes
	e.Progress.Partial = e.Progress.ReadEnd > e.Progress.CompleteEnd
	return src, nil
}

func (r CJSIReader) List(ctx context.Context) ([]Metadata, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) ([]Metadata, error) { return v.list(ctx) })
}

func (v indexReadView) list(ctx context.Context) ([]Metadata, error) {
	rows, err := v.tx.QueryContext(ctx, v.selection()+` SELECT `+selectedMetadataColumns+` FROM ranked WHERE choice=1 ORDER BY last_at DESC,client,session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Metadata, 0)
	for rows.Next() {
		m, err := scanSelectedMetadata(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r CJSIReader) Search(ctx context.Context, query string) ([]Document, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) ([]Document, error) { return v.Search(ctx, query) })
}

func (r CJSIReader) SelectDetail(ctx context.Context, client, id string) (CJSISelectedDetail, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) (CJSISelectedDetail, error) { return v.metadata(ctx, client, id) })
}

func (r CJSIReader) DocumentsPage(ctx context.Context, m CJSISelectedDetail, page, limit int, all bool) ([]Document, Pagination, error) {
	return snapshotPage(ctx, r.snapshot(ctx), m, page, limit, all)
}

// ActivityPage revalidates the viewer's selection before using the independent
// activity pagination. No range on that copy authorizes no file read.
func (r CJSIReader) ActivityPage(ctx context.Context, m CJSISelectedDetail, page, limit int, all bool) (activity.Page, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) (activity.Page, error) {
		fresh, err := v.reopen(ctx, m)
		if err != nil {
			return activity.Page{}, err
		}
		return v.activityPage(ctx, fresh, page, limit, all)
	})
}

func (r CJSIReader) ShowWithActivity(ctx context.Context, client, id string, include bool) (Result, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) (Result, error) { return v.ShowWithActivity(ctx, client, id, include) })
}

func (r CJSIReader) EffectiveCopies(ctx context.Context, client, id string) ([]EffectiveSessionCopy, error) {
	return snapshotResult(r.snapshot(ctx), func(v indexReadView) ([]EffectiveSessionCopy, error) { return v.EffectiveCopies(ctx, client, id) })
}

// Compatible file activity still uses its original parser, but both sides of
// the file read must match the checkpoint held by this private selection.
func readCompatibleActivity[T any](ctx context.Context, s store.CJSIIndexSnapshot, m Metadata, read func(*os.File, int64) (T, error)) (T, error) {
	var zero T
	observed := ingest.Source{Path: m.SourcePath, Stable: true}
	var end int64
	err := s.Tx.QueryRowContext(ctx, `SELECT s.identity,s.size,s.modified_at,s.changed_at,s.cursor
		FROM session_sources s JOIN (`+s.CopiesSQL()+`) c USING(source_path,identity)
		WHERE c.source_path=? AND c.client=? AND c.session_id=? AND s.changed_at<>0 AND s.cursor BETWEEN 0 AND s.size
		AND (s.cursor=s.size OR (s.parser_version=? AND c.parser_version=s.parser_version AND `+compatibleLegacyHintSQL+`
		  AND NOT EXISTS(SELECT 1 FROM cjsi_base_groups g WHERE g.source_path=s.source_path AND
		    (g.client<>c.client OR (g.source_kind IS NOT NULL AND g.source_kind<>'claude-jsonl')))))`, m.SourcePath, m.Client, m.SessionID, ParserVersion).Scan(&observed.Identity, &observed.Size, &observed.ModifiedAt, &observed.ChangedAt, &end)
	if err != nil {
		return zero, err
	}
	// Validate the descriptor actually used by the parser. A replacement
	// at the named path must not block the open or substitute another file.
	file, err := os.OpenFile(m.SourcePath, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return zero, err
	}
	defer file.Close()
	if err := validateActivityDescriptor(file, observed); err != nil {
		return zero, err
	}
	result, err := read(file, end)
	if err != nil {
		return zero, err
	}
	if err := ingest.Validate(observed); err != nil {
		return zero, err
	}
	return result, nil
}

func validateActivityDescriptor(file *os.File, expected ingest.Source) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !ingest.SameGeneration(expected, info) {
		return ingest.ErrSourceChanged
	}
	return nil
}

// CJSIOrderEntry identifies one eligible member of a physical copy. This
// metadata is for private portable-state export; it never enters a CLI row.
type CJSIOrderEntry struct {
	SourcePath, Client, SessionID string
	DocumentID, Ordinal           int64
	Verified                      bool
}

type CJSIOrderSnapshot struct {
	Stamp     store.CJSIIndexStamp
	HighWater int64
	Entries   []CJSIOrderEntry
}

// ReadCJSIOrderSnapshot buffers eligible memberships and their allocation
// floor. Discard the return value if the surrounding Store guard later fails.
func ReadCJSIOrderSnapshot(ctx context.Context, s store.CJSIIndexSnapshot) (CJSIOrderSnapshot, error) {
	if s.Tx == nil {
		return CJSIOrderSnapshot{}, store.ErrCJSIIndexChanged
	}
	out := CJSIOrderSnapshot{Stamp: s.Selection()}
	if err := s.Tx.QueryRowContext(ctx, "SELECT doc_id_high_water FROM cjsi_index_state WHERE singleton=1").Scan(&out.HighWater); err != nil {
		return CJSIOrderSnapshot{}, err
	}
	rows, err := s.Tx.QueryContext(ctx, `SELECT m.source_path,m.client,m.session_id,m.doc_id,m.detail_ordinal,c.order_verified FROM (`+s.MembersSQL()+`) m JOIN (`+s.CopiesSQL()+`) c USING(source_path,client,session_id) ORDER BY m.source_path,m.client,m.session_id,m.detail_ordinal`)
	if err != nil {
		return CJSIOrderSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e CJSIOrderEntry
		if err := rows.Scan(&e.SourcePath, &e.Client, &e.SessionID, &e.DocumentID, &e.Ordinal, &e.Verified); err != nil {
			return CJSIOrderSnapshot{}, err
		}
		if e.DocumentID <= 0 || e.DocumentID > out.HighWater {
			return CJSIOrderSnapshot{}, store.ErrCJSIIndexChanged
		}
		out.Entries = append(out.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return CJSIOrderSnapshot{}, err
	}
	return out, nil
}
