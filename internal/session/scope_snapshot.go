package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// SourceCopy is one indexed copy of a canonical session as the origin
// classifier sees it: the source's stable identity and generation, never its
// path or content.
type SourceCopy struct {
	Identity      string
	PrefixHash    string
	ParserVersion int64
	FirstAt       string
}

// IndexedSession is one visible canonical session with every indexed copy.
// Metadata is the copy List selects (highest priority, then source path).
type IndexedSession struct {
	Metadata
	Copies []SourceCopy
}

// IndexSnapshot is the visible session index read in one read transaction,
// together with the session side of the serving generation vector captured in
// that same transaction.
//
// Its read dependencies are session_index_generation, serving_generation,
// session_metadata, session_sources and, for a search, session_documents.
// session_metadata and session_sources advance serving_generation.revision on
// every INSERT/UPDATE/DELETE through their triggers; exclusions are applied
// by deleting rows, so session_exclusions changes reach the same triggers.
// session_documents is an FTS5 table and cannot carry a trigger itself; its
// content shadow table session_documents_content does, so every document
// INSERT, UPDATE and DELETE (direct SQL included) advances the same revision.
type IndexSnapshot struct {
	Epoch    int64
	Revision int64
	Sessions []IndexedSession
	// Documents are the search matches of every visible session, in Search
	// order. It is nil when no query was given.
	Documents []Document
}

// ErrIndexGenerationMissing reports an index without serving generation state.
var ErrIndexGenerationMissing = errors.New("session index generation is missing")

// ReadIndexSnapshot reads the visible index, with the same canonical
// selection and order as List, in one read-only transaction. A non-empty
// query also returns Search's matches from the same transaction.
func ReadIndexSnapshot(ctx context.Context, db *sql.DB, query string) (IndexSnapshot, error) {
	var snapshot IndexSnapshot
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return snapshot, err
	}
	defer tx.Rollback()
	// The first statement fixes the WAL snapshot every later read sees.
	err = tx.QueryRowContext(ctx, `SELECT e.epoch,g.revision FROM session_index_generation e JOIN serving_generation g ON g.singleton=e.singleton WHERE e.singleton=1`).Scan(&snapshot.Epoch, &snapshot.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return snapshot, ErrIndexGenerationMissing
	}
	if err != nil {
		return snapshot, err
	}
	rows, err := tx.QueryContext(ctx, `WITH visible AS (SELECT m.*,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT client,session_id,project,source_path,model,first_at,last_at FROM visible WHERE n=1 ORDER BY last_at DESC,client,session_id`)
	if err != nil {
		return snapshot, err
	}
	index := map[[2]string]int{}
	for rows.Next() {
		var m Metadata
		if err = rows.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt); err != nil {
			rows.Close()
			return snapshot, err
		}
		index[[2]string{m.Client, m.SessionID}] = len(snapshot.Sessions)
		snapshot.Sessions = append(snapshot.Sessions, IndexedSession{Metadata: m})
	}
	if err = rows.Close(); err != nil {
		return snapshot, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT m.client,m.session_id,s.identity,s.prefix_hash,m.parser_version,m.first_at FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path ORDER BY m.client,m.session_id,s.identity,s.prefix_hash,m.parser_version,m.first_at`)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var client, id string
		var copy SourceCopy
		if err = rows.Scan(&client, &id, &copy.Identity, &copy.PrefixHash, &copy.ParserVersion, &copy.FirstAt); err != nil {
			rows.Close()
			return snapshot, err
		}
		if position, ok := index[[2]string{client, id}]; ok {
			snapshot.Sessions[position].Copies = append(snapshot.Sessions[position].Copies, copy)
		}
	}
	if err = rows.Close(); err != nil {
		return snapshot, err
	}
	if query == "" {
		return snapshot, tx.Commit()
	}
	if strings.TrimSpace(query) == "" {
		return snapshot, errors.New("search query is required")
	}
	snapshot.Documents, err = searchDocuments(ctx, tx, query)
	if err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}

type documentQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func searchDocuments(ctx context.Context, q documentQueryer, query string) ([]Document, error) {
	rows, err := q.QueryContext(ctx, searchSQL, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Document, 0)
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.Client, &d.SessionID, &d.EventAt, &d.Kind, &d.Text); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ProjectGroup is the project identity sessions are counted under, and its
// display label. Ordinary checkouts keep their normalized full path as the
// identity; ChatGPT Work conversation directories are ephemeral
// one-conversation workspaces that share one product identity. An empty
// identity is no project.
func ProjectGroup(value string) (identity, label string) {
	identity = strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if identity != "" {
		label = identity[strings.LastIndex(identity, "/")+1:]
	}
	if label == "." {
		label = ""
	}
	if strings.HasPrefix(strings.ToLower(label), "referenced-chatgpt-conversation-") {
		return "chatgpt-work://referenced-conversation", "ChatGPT Work"
	}
	return identity, label
}
