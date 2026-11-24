package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
)

// FTS stores timestamps as text. Compare the full instant, including nanos,
// and leave equal or unparseable values to the existing source-order keys.
func init() {
	sqlite.MustRegisterCollationUtf8("agentdeck_event_instant", func(left, right string) int {
		a, errA := time.Parse(time.RFC3339Nano, left)
		b, errB := time.Parse(time.RFC3339Nano, right)
		switch {
		case errA == nil && errB == nil:
			return a.Compare(b)
		case errA == nil:
			return 1
		case errB == nil:
			return -1
		default:
			return 0
		}
	})
}

// indexReadView borrows one transaction for the entire read. The three SELECTs
// are trusted adapter queries, never caller input. copies contains protected,
// effective source copies before priority selection. detailIdentity obtains
// the whole selected detail identity from this same borrowed snapshot.
// documents and matches refer to the same ticket's unmasked members; matches
// runs MATCH on each FTS side before joining its effective members. These
// projection names do not prescribe DDL.
type indexReadView struct {
	stamp                      store.CJSIIndexStamp
	tx                         *sql.Tx
	copies, documents, matches string
	activity                   func(context.Context, Metadata) ([]activity.Detail, error)
	activityPage               func(context.Context, Metadata, int, int, bool) (activity.Page, error)
	detailIdentity             func(context.Context, string, string) (store.CJSIDetailIdentity, error)
}

func (v indexReadView) selection() string {
	return "WITH copies AS (" + v.copies + `), ranked AS (
		SELECT c.*, row_number() OVER (PARTITION BY client,session_id ORDER BY priority DESC,source_path) AS choice
		FROM copies c)`
}

const selectedMetadataColumns = `client,session_id,project,source_path,model,first_at,last_at`

func scanSelectedMetadata(row interface{ Scan(...any) error }) (Metadata, error) {
	var m Metadata
	if err := row.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt); err != nil {
		return Metadata{}, err
	}
	return m, nil
}

func (v indexReadView) selectedMetadata(ctx context.Context, client, id string) (Metadata, error) {
	return scanSelectedMetadata(v.tx.QueryRowContext(ctx, "SELECT "+selectedMetadataColumns+" FROM ("+v.copies+") WHERE client=? AND session_id=? ORDER BY priority DESC,source_path LIMIT 1", client, id))
}

// CJSISelectedDetail belongs to an admitted snapshot, not to public session
// metadata. A legacy List or ShowMetadata result cannot authorize a CJSI page.
type CJSISelectedDetail struct {
	Metadata
	identity store.CJSIDetailIdentity
}

func (v indexReadView) metadata(ctx context.Context, client, id string) (CJSISelectedDetail, error) {
	m, err := v.selectedMetadata(ctx, client, id)
	if err != nil {
		return CJSISelectedDetail{}, err
	}
	if v.detailIdentity == nil {
		return CJSISelectedDetail{}, sql.ErrNoRows
	}
	d, err := v.detailIdentity(ctx, client, id)
	if err != nil {
		return CJSISelectedDetail{}, err
	}
	if d.Stamp == (store.CJSIIndexStamp{}) || d.Sequence == ([32]byte{}) || d.SourcePath != m.SourcePath || d.Client != m.Client || d.SessionID != m.SessionID {
		return CJSISelectedDetail{}, sql.ErrNoRows
	}
	return CJSISelectedDetail{m, d}, nil
}

func (v indexReadView) reopen(ctx context.Context, m CJSISelectedDetail) (Metadata, error) {
	selected, err := v.metadata(ctx, m.Client, m.SessionID)
	if err != nil {
		return Metadata{}, err
	}
	if selected.SourcePath != m.SourcePath || selected.identity != m.identity {
		return Metadata{}, sql.ErrNoRows
	}
	return selected.Metadata, nil
}

func (v indexReadView) DocumentsPage(ctx context.Context, m CJSISelectedDetail, page, limit int, all bool) ([]Document, Pagination, error) {
	selected, err := v.reopen(ctx, m)
	if err != nil {
		return nil, Pagination{}, err
	}
	return v.readPage(ctx, selected, page, limit, all)
}

func (v indexReadView) readPage(ctx context.Context, m Metadata, page, limit int, all bool) ([]Document, Pagination, error) {
	if page < 1 || limit < 1 || limit > MaxPageLimit {
		return nil, Pagination{}, fmt.Errorf("page must be positive and limit must be between 1 and %d", MaxPageLimit)
	}
	from := " FROM (" + v.documents + ") WHERE source_path=? AND client=? AND session_id=?"
	var total int
	if err := v.tx.QueryRowContext(ctx, "SELECT count(*)"+from, m.SourcePath, m.Client, m.SessionID).Scan(&total); err != nil {
		return nil, Pagination{}, err
	}
	p := Pagination{Page: page, Limit: limit, Total: total}
	offset := 0
	if all {
		p.Page, p.Limit = 1, total
	} else if page-1 > total/limit {
		return []Document{}, p, nil
	} else {
		offset = (page - 1) * limit
	}
	rows, err := v.tx.QueryContext(ctx, "SELECT client,session_id,event_at,kind,text"+from+" ORDER BY detail_ordinal LIMIT ? OFFSET ?", m.SourcePath, m.Client, m.SessionID, p.Limit, offset)
	if err != nil {
		return nil, Pagination{}, err
	}
	defer rows.Close()
	docs := make([]Document, 0, min(p.Limit, total))
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.Client, &d.SessionID, &d.EventAt, &d.Kind, &d.Text); err != nil {
			return nil, Pagination{}, err
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, Pagination{}, err
	}
	p.Shown = len(docs)
	p.HasMore = offset+p.Shown < total
	if p.HasMore {
		p.NextPage = page + 1
	}
	return docs, p, nil
}

func (v indexReadView) ShowWithActivity(ctx context.Context, client, id string, include bool) (Result, error) {
	m, err := v.selectedMetadata(ctx, client, id)
	if err != nil {
		return Result{}, err
	}
	docs, _, err := v.readPage(ctx, m, 1, MaxPageLimit, true)
	if err != nil {
		return Result{}, err
	}
	r := Result{Metadata: m, Documents: docs}
	if include && !IsTelemetrySource(m.SourcePath) {
		if v.activity == nil {
			return r, errors.New("activity source is unavailable")
		}
		r.Activity, err = v.activity(ctx, m)
		if err == nil {
			r.ActivitySummary = SummarizeActivity(r.Activity)
		}
	}
	return r, err
}

// visitHits keeps stable IDs internal. Ordering happens after visibility and
// per-side MATCH, so an empty replacement cannot resurrect a hidden base hit.
func (v indexReadView) visitHits(ctx context.Context, query string, visit func(Document, int64)) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("search query is required")
	}
	rows, err := v.tx.QueryContext(ctx, v.selection()+", hits AS ("+v.matches+`) SELECT d.client,d.session_id,d.event_at,d.kind,d.text,d.doc_id
		FROM hits d JOIN ranked r USING (source_path,client,session_id) WHERE r.choice=1
		ORDER BY d.event_at COLLATE agentdeck_event_instant DESC,d.client,d.session_id,d.doc_id`, sql.Named("query", query))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d Document
		var id int64
		if err := rows.Scan(&d.Client, &d.SessionID, &d.EventAt, &d.Kind, &d.Text, &id); err != nil {
			return err
		}
		visit(d, id)
	}
	return rows.Err()
}

func (v indexReadView) Search(ctx context.Context, query string) ([]Document, error) {
	docs := make([]Document, 0)
	err := v.visitHits(ctx, query, func(d Document, _ int64) { docs = append(docs, d) })
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func legacyIndexView(tx *sql.Tx) indexReadView {
	return indexReadView{tx: tx,
		copies:    "SELECT m.*,s.priority FROM session_metadata m JOIN session_sources s USING(source_path)",
		documents: "SELECT source_path,client,session_id,event_at,kind,text,rowid AS detail_ordinal FROM session_documents",
		matches:   "SELECT source_path,client,session_id,event_at,kind,text,rowid AS doc_id FROM session_documents WHERE session_documents MATCH @query",
		activity: func(_ context.Context, m Metadata) ([]activity.Detail, error) {
			return activity.ReadDetails(m.SourcePath, m.Client, m.SessionID)
		},
	}
}

// The opener owns the transaction and the final protection check. Even a
// successful callback has produced only provisional output until it returns.
type withIndexSnapshot func(func(indexReadView) error) error

func snapshotResult[T any](with withIndexSnapshot, read func(indexReadView) (T, error)) (T, error) {
	var pending T
	err := with(func(v indexReadView) error {
		var err error
		pending, err = read(v)
		return err
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return pending, nil
}

func snapshotPage(ctx context.Context, with withIndexSnapshot, m CJSISelectedDetail, page, limit int, all bool) ([]Document, Pagination, error) {
	type result struct {
		docs []Document
		p    Pagination
	}
	r, err := snapshotResult(with, func(v indexReadView) (result, error) {
		docs, p, err := v.DocumentsPage(ctx, m, page, limit, all)
		return result{docs, p}, err
	})
	return r.docs, r.p, err
}

// EffectiveSessionCopy is a Task 6 input, internal to AgentDeck. HeadAt and
// Birth are verified UTC instants from this copy; an empty value is unknown.
// Priority selects displayed metadata, independently of cross-copy birth.
type EffectiveSessionCopy struct {
	Metadata      Metadata
	Source        SourceCopy
	HeadAt, Birth string
	Selected      bool
}

func (v indexReadView) EffectiveCopies(ctx context.Context, client, id string) ([]EffectiveSessionCopy, error) {
	rows, err := v.tx.QueryContext(ctx, v.selection()+" SELECT client,session_id,project,source_path,model,first_at,last_at,identity,prefix_hash,parser_version,head_at,birth,choice FROM ranked WHERE client=? AND session_id=? ORDER BY choice", client, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var copies []EffectiveSessionCopy
	for rows.Next() {
		var c EffectiveSessionCopy
		m := &c.Metadata
		var rank int
		if err := rows.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt, &c.Source.Identity, &c.Source.PrefixHash, &c.Source.ParserVersion, &c.HeadAt, &c.Birth, &rank); err != nil {
			return nil, err
		}
		c.Selected = rank == 1
		c.Source.FirstAt = m.FirstAt
		copies = append(copies, c)
	}
	return copies, rows.Err()
}

// A bare session handle cannot identify a selected private generation.
func requireLegacyIndex(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) error {
	var managed bool
	if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM main.sqlite_schema WHERE type='table' AND name='cjsi_index_state')").Scan(&managed); err != nil {
		return err
	}
	if managed {
		return store.ErrCJSIIndexChanged
	}
	return nil
}

func legacyReadTransaction(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	if err = requireLegacyIndex(ctx, tx); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
