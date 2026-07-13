// Package session indexes only explicitly approved visible conversation text.
package session

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	ParserVersion                  = 3
	replaceDocumentsSourcePath     = "agentdeck://replace-documents"
	replaceDocumentsSourceIdentity = "synthetic:replace-documents"
)

type Document struct{ Client, SessionID, Kind, Text string }
type Metadata struct{ Client, SessionID, Project, SourcePath, Model, FirstAt, LastAt string }
type Result struct {
	Metadata
	Documents []Document
}
type ScanResult struct{ Sources, Documents, Skipped int }

// ApprovedDocument is the privacy boundary: only text already classified by a
// client-specific allowlist as a visible user prompt or final assistant reply
// can enter sessions.sqlite3.
func ApprovedDocument(client, sessionID, kind, text string) (Document, error) {
	if client != "codex" && client != "claude" {
		return Document{}, errors.New("unsupported client")
	}
	if sessionID == "" || strings.TrimSpace(text) == "" {
		return Document{}, errors.New("session id and text are required")
	}
	if kind != "user_prompt" && kind != "assistant_final" {
		return Document{}, fmt.Errorf("prohibited session content kind %q", kind)
	}
	return Document{Client: client, SessionID: sessionID, Kind: kind, Text: strings.TrimSpace(text)}, nil
}

// Scan reads only known JSONL shapes. Unknown records and content types are
// deliberately ignored, so a client format change fails closed.
func Scan(ctx context.Context, db *sql.DB, home string) (ScanResult, error) {
	var paths []source
	for _, root := range []struct {
		client, path string
		priority     int
	}{
		{"codex", filepath.Join(home, ".codex", "archived_sessions"), 0},
		{"claude", filepath.Join(home, ".claude", "projects"), 0},
		{"codex", filepath.Join(home, ".codex", "sessions"), 1},
	} {
		err := filepath.WalkDir(root.path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".jsonl") {
				paths = append(paths, source{root.client, path, root.priority})
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return ScanResult{}, err
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].priority != paths[j].priority {
			return paths[i].priority < paths[j].priority
		}
		return paths[i].path < paths[j].path
	})
	result := ScanResult{}
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		seen[filepath.Clean(p.path)] = true
		changed, docs, err := scanSource(ctx, db, p)
		if err != nil {
			return result, err
		}
		if !changed {
			result.Skipped++
			continue
		}
		result.Sources++
		result.Documents += docs
	}
	if err := removeMissingSources(ctx, db, seen); err != nil {
		return result, err
	}
	return result, nil
}

type source struct {
	client, path string
	priority     int
}

type sourceState struct {
	path, identity, prefixHash                        string
	cursor, size, modifiedAt, priority, parserVersion int64
	partial                                           []byte
}

func scanSource(ctx context.Context, db *sql.DB, src source) (bool, int, error) {
	path := filepath.Clean(src.path)
	info, err := os.Stat(path)
	if err != nil {
		return false, 0, err
	}
	identity, err := fileIdentity(info)
	if err != nil {
		return false, 0, err
	}
	prefix, err := prefixHash(path, info.Size())
	if err != nil {
		return false, 0, err
	}
	state, found, err := loadSource(ctx, db, path)
	if err != nil {
		return false, 0, err
	}
	if !found {
		// A rename preserves source ownership and avoids a full index rebuild.
		state, found, err = loadSourceByIdentity(ctx, db, identity)
		if err != nil {
			return false, 0, err
		}
		if found {
			if err = moveSource(ctx, db, state.path, path); err != nil {
				return false, 0, err
			}
			state.path = path
		}
	}
	oldPrefix := ""
	if found {
		oldPrefix, err = prefixHash(path, state.cursor)
		if err != nil {
			return false, 0, err
		}
	}
	unchanged := found && state.identity == identity && state.parserVersion == ParserVersion && info.Size() == state.size && state.modifiedAt == info.ModTime().UnixNano() && prefix == state.prefixHash && state.priority == int64(src.priority)
	if unchanged {
		return false, 0, nil
	}
	appendOnly := found && state.identity == identity && state.parserVersion == ParserVersion && info.Size() > state.cursor && oldPrefix == state.prefixHash
	var results []Result
	var partial []byte
	if appendOnly {
		results, partial, err = parseRange(src.client, path, state.cursor, state.partial)
	} else {
		results, partial, err = parseRange(src.client, path, 0, nil)
	}
	if err != nil {
		return false, 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	if !appendOnly {
		if err = deleteSource(ctx, tx, path); err != nil {
			return false, 0, err
		}
	}
	if partial == nil {
		partial = []byte{}
	}
	newState := sourceState{path: path, identity: identity, cursor: info.Size(), size: info.Size(), modifiedAt: info.ModTime().UnixNano(), prefixHash: prefix, priority: int64(src.priority), parserVersion: ParserVersion, partial: partial}
	if err = saveSourceTx(ctx, tx, newState); err != nil {
		return false, 0, err
	}
	docs := 0
	for _, r := range results {
		r.SourcePath = path
		if excluded(ctx, db, r.Metadata) {
			continue
		}
		if err = insertResult(ctx, tx, r); err != nil {
			return false, 0, err
		}
		docs += len(r.Documents)
	}
	if err = tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, docs, nil
}

func loadSource(ctx context.Context, db *sql.DB, path string) (sourceState, bool, error) {
	var s sourceState
	s.path = path
	err := db.QueryRowContext(ctx, "SELECT identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources WHERE source_path=?", path).Scan(&s.identity, &s.cursor, &s.partial, &s.size, &s.modifiedAt, &s.prefixHash, &s.priority, &s.parserVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}
func loadSourceByIdentity(ctx context.Context, db *sql.DB, identity string) (sourceState, bool, error) {
	var s sourceState
	err := db.QueryRowContext(ctx, "SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources WHERE identity=?", identity).Scan(&s.path, &s.identity, &s.cursor, &s.partial, &s.size, &s.modifiedAt, &s.prefixHash, &s.priority, &s.parserVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}
func moveSource(ctx context.Context, db *sql.DB, old, new string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) SELECT ?,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM session_sources WHERE source_path=?", new, old); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE session_documents SET source_path=? WHERE source_path=?; UPDATE session_metadata SET source_path=? WHERE source_path=?; DELETE FROM session_sources WHERE source_path=?", new, old, new, old, old); err != nil {
		return err
	}
	return tx.Commit()
}
func deleteSource(ctx context.Context, tx *sql.Tx, path string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM session_documents WHERE source_path=?", path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM session_metadata WHERE source_path=?", path); err != nil {
		return err
	}
	return nil
}
func saveSourceTx(ctx context.Context, tx *sql.Tx, s sourceState) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(source_path) DO UPDATE SET identity=excluded.identity,cursor=excluded.cursor,partial_line=excluded.partial_line,size=excluded.size,modified_at=excluded.modified_at,prefix_hash=excluded.prefix_hash,priority=excluded.priority,parser_version=excluded.parser_version,scanned_at=excluded.scanned_at", s.path, s.identity, s.cursor, s.partial, s.size, s.modifiedAt, s.prefixHash, s.priority, s.parserVersion, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func removeMissingSources(ctx context.Context, db *sql.DB, seen map[string]bool) error {
	rows, err := db.QueryContext(ctx, "SELECT source_path FROM session_sources WHERE identity != ?", replaceDocumentsSourceIdentity)
	if err != nil {
		return err
	}
	defer rows.Close()
	var missing []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return err
		}
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range missing {
		if err = deleteSource(ctx, tx, p); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM session_sources WHERE source_path=?", p); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func prefixHash(path string, limit int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if limit > 4096 {
		limit = 4096
	}
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	sum := sha256.Sum256(buf[:n])
	return fmt.Sprintf("%x", sum[:]), nil
}
func fileIdentity(info fs.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("unsupported file identity")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

// parseRange consumes only complete JSONL records.  The unterminated suffix is
// returned byte-for-byte so a later append resumes it without indexing a
// partial prompt or reply.
func parseRange(client, path string, offset int64, previous []byte) ([]Result, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return nil, nil, err
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	data := append(append([]byte(nil), previous...), contents...)
	last := bytes.LastIndexByte(data, '\n')
	if last < 0 {
		return nil, data, nil
	}
	complete, partial := data[:last+1], append([]byte(nil), data[last+1:]...)
	byID := map[string]*Result{}
	currentID := ""
	for _, line := range bytes.Split(complete, []byte{'\n'}) {
		var v map[string]any
		if len(line) == 0 || json.Unmarshal(line, &v) != nil {
			continue
		}
		id, doc, meta := extract(client, v)
		if id != "" {
			currentID = id
		}
		if id == "" {
			id = currentID
		}
		if id == "" {
			continue
		}
		if doc.Kind == "" {
			doc = fixtureDocument(client, id, v)
		}
		if doc.SessionID == "" && doc.Kind != "" {
			doc.SessionID = id
		}
		meta.SessionID = id
		r := byID[id]
		if r == nil {
			r = &Result{Metadata: Metadata{Client: client, SessionID: id, SourcePath: filepath.Clean(path)}}
			byID[id] = r
		}
		mergeMeta(&r.Metadata, meta)
		if doc.Kind != "" {
			r.Documents = append(r.Documents, doc)
		}
	}
	out := make([]Result, 0, len(byID))
	for _, r := range byID {
		if r.Project == "" {
			r.Project = normalizeProject(filepath.Dir(path))
		}
		out = append(out, *r)
	}
	return out, partial, nil
}

func parseFile(client, path string) ([]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	byID := map[string]*Result{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	currentID := ""
	for scanner.Scan() {
		var v map[string]any
		if json.Unmarshal(scanner.Bytes(), &v) != nil {
			continue
		}
		id, doc, meta := extract(client, v)
		if id != "" {
			currentID = id
		}
		if id == "" {
			id = currentID
		}
		if id == "" {
			continue
		}
		if doc.Kind == "" {
			doc = fixtureDocument(client, id, v)
		}
		if doc.SessionID == "" && doc.Kind != "" {
			doc.SessionID = id
		}
		meta.SessionID = id
		r := byID[id]
		if r == nil {
			r = &Result{Metadata: Metadata{Client: client, SessionID: id, SourcePath: filepath.Clean(path)}}
			byID[id] = r
		}
		mergeMeta(&r.Metadata, meta)
		if doc.Kind != "" {
			r.Documents = append(r.Documents, doc)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(byID))
	for _, r := range byID {
		if r.Metadata.Project == "" {
			r.Metadata.Project = normalizeProject(filepath.Dir(path))
		}
		out = append(out, *r)
	}
	return out, nil
}
func fixtureDocument(client, id string, v map[string]any) Document {
	if client != "codex" {
		return Document{}
	}
	p, _ := v["payload"].(map[string]any)
	switch str(v["type"]) {
	case "visible_user_prompt":
		d, _ := ApprovedDocument(client, id, "user_prompt", str(p["text"]))
		return d
	case "visible_assistant_final":
		d, _ := ApprovedDocument(client, id, "assistant_final", str(p["text"]))
		return d
	}
	return Document{}
}
func extract(client string, v map[string]any) (string, Document, Metadata) {
	if client == "codex" {
		return extractCodex(v)
	}
	return extractClaude(v)
}
func extractCodex(v map[string]any) (string, Document, Metadata) {
	p, _ := v["payload"].(map[string]any)
	typ, _ := v["type"].(string)
	id := str(p["session_id"])
	if id == "" {
		id = str(v["session_id"])
	}
	if id == "" {
		id = str(v["sessionId"])
	}
	m := meta("codex", id, p, v)
	// Explicit fixture protocol is intentionally accepted as an adapter contract.
	if typ == "visible_user_prompt" {
		d, _ := ApprovedDocument("codex", id, "user_prompt", str(p["text"]))
		return id, d, m
	}
	if typ == "visible_assistant_final" {
		d, _ := ApprovedDocument("codex", id, "assistant_final", str(p["text"]))
		return id, d, m
	}
	// Real record allowlist: response_item/message with exactly text content.
	if typ != "response_item" {
		return id, Document{}, m
	}
	item, _ := p["item"].(map[string]any)
	if item == nil {
		item = p
	}
	if str(item["type"]) != "message" {
		return id, Document{}, m
	}
	role := str(item["role"])
	if role != "user" && role != "assistant" {
		return id, Document{}, m
	}
	text, ok := textContent(item["content"], map[string]string{"user": "input_text", "assistant": "output_text"}[role])
	if !ok {
		return id, Document{}, m
	}
	kind := "user_prompt"
	if role == "assistant" {
		kind = "assistant_final"
	}
	d, _ := ApprovedDocument("codex", id, kind, text)
	return id, d, m
}
func extractClaude(v map[string]any) (string, Document, Metadata) {
	typ := str(v["type"])
	id := str(v["sessionId"])
	if id == "" {
		id = str(v["session_id"])
	}
	m := meta("claude", id, v, v)
	if typ != "user" && typ != "assistant" {
		return id, Document{}, m
	}
	msg, _ := v["message"].(map[string]any)
	if msg == nil {
		return id, Document{}, m
	}
	var text string
	var ok bool
	if typ == "user" {
		text, ok = msg["content"].(string)
	} else {
		text, ok = textContent(msg["content"], "text")
	}
	if !ok {
		return id, Document{}, m
	}
	kind := "user_prompt"
	if typ == "assistant" {
		kind = "assistant_final"
	}
	d, _ := ApprovedDocument("claude", id, kind, text)
	return id, d, m
}
func textContent(raw any, want string) (string, bool) {
	if s, ok := raw.(string); ok && want == "input_text" {
		return s, true
	}
	a, ok := raw.([]any)
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, x := range a {
		m, ok := x.(map[string]any)
		if !ok || str(m["type"]) != want {
			return "", false
		}
		t := str(m["text"])
		if t == "" {
			return "", false
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t)
	}
	return b.String(), b.Len() > 0
}
func meta(client, id string, a, b map[string]any) Metadata {
	return Metadata{Client: client, SessionID: id, Project: normalizeProject(first(str(a["cwd"]), str(a["project"]), str(b["cwd"]), str(b["project"]))), Model: first(str(a["model"]), str(b["model"])), FirstAt: first(str(a["timestamp"]), str(b["timestamp"])), LastAt: first(str(a["timestamp"]), str(b["timestamp"]))}
}
func first(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
func str(v any) string { s, _ := v.(string); return s }
func normalizeProject(v string) string {
	if v == "" {
		return ""
	}
	return filepath.Clean(v)
}
func mergeMeta(dst *Metadata, src Metadata) {
	if dst.Project == "" {
		dst.Project = src.Project
	}
	if dst.Model == "" {
		dst.Model = src.Model
	}
	if dst.FirstAt == "" || src.FirstAt < dst.FirstAt {
		dst.FirstAt = src.FirstAt
	}
	if src.LastAt > dst.LastAt {
		dst.LastAt = src.LastAt
	}
}

func replace(ctx context.Context, db *sql.DB, r Result) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM session_documents WHERE source_path=? AND client=? AND session_id=?", r.SourcePath, r.Client, r.SessionID); err != nil {
		return err
	}
	if err = insertResult(ctx, tx, r); err != nil {
		return err
	}
	return tx.Commit()
}
func insertResult(ctx context.Context, tx *sql.Tx, r Result) error {
	for _, d := range r.Documents {
		if _, err := tx.ExecContext(ctx, "INSERT INTO session_documents(source_path,client,session_id,kind,text) VALUES(?,?,?,?,?)", r.SourcePath, d.Client, d.SessionID, d.Kind, d.Text); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(source_path,client,session_id) DO UPDATE SET project=CASE WHEN excluded.project='' THEN session_metadata.project ELSE excluded.project END,model=CASE WHEN excluded.model='' THEN session_metadata.model ELSE excluded.model END,parser_version=excluded.parser_version,first_at=CASE WHEN session_metadata.first_at='' OR excluded.first_at<session_metadata.first_at THEN excluded.first_at ELSE session_metadata.first_at END,last_at=CASE WHEN excluded.last_at>session_metadata.last_at THEN excluded.last_at ELSE session_metadata.last_at END", r.SourcePath, r.Client, r.SessionID, r.Project, r.Model, ParserVersion, r.FirstAt, r.LastAt)
	return err
}
func ReplaceDocuments(ctx context.Context, db *sql.DB, client, sessionID string, docs []Document) error {
	if _, err := db.ExecContext(ctx, "INSERT OR IGNORE INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)", replaceDocumentsSourcePath, replaceDocumentsSourceIdentity, 0, []byte{}, 0, 0, "", 1, ParserVersion, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return replace(ctx, db, Result{Metadata: Metadata{Client: client, SessionID: sessionID, SourcePath: replaceDocumentsSourcePath}, Documents: docs})
}
func Search(ctx context.Context, db *sql.DB, query string) ([]Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("search query is required")
	}
	rows, err := db.QueryContext(ctx, `WITH visible AS (SELECT m.source_path,m.client,m.session_id,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT d.client,d.session_id,d.kind,d.text FROM session_documents d JOIN visible v ON v.source_path=d.source_path AND v.client=d.client AND v.session_id=d.session_id WHERE v.n=1 AND session_documents MATCH ? ORDER BY rank`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.Client, &d.SessionID, &d.Kind, &d.Text); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func List(ctx context.Context, db *sql.DB) ([]Metadata, error) {
	rows, err := db.QueryContext(ctx, `WITH visible AS (SELECT m.*,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT client,session_id,project,source_path,model,first_at,last_at FROM visible WHERE n=1 ORDER BY last_at DESC,client,session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Metadata
	for rows.Next() {
		var m Metadata
		if err = rows.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func Show(ctx context.Context, db *sql.DB, client, id string) (Result, error) {
	var r Result
	if err := db.QueryRowContext(ctx, `SELECT m.client,m.session_id,m.project,m.source_path,m.model,m.first_at,m.last_at FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path WHERE m.client=? AND m.session_id=? ORDER BY s.priority DESC,m.source_path LIMIT 1`, client, id).Scan(&r.Client, &r.SessionID, &r.Project, &r.SourcePath, &r.Model, &r.FirstAt, &r.LastAt); err != nil {
		return r, err
	}
	docs, err := db.QueryContext(ctx, "SELECT client,session_id,kind,text FROM session_documents WHERE source_path=? AND client=? AND session_id=? ORDER BY rowid", r.SourcePath, client, id)
	if err != nil {
		return r, err
	}
	defer docs.Close()
	for docs.Next() {
		var d Document
		if err = docs.Scan(&d.Client, &d.SessionID, &d.Kind, &d.Text); err != nil {
			return r, err
		}
		r.Documents = append(r.Documents, d)
	}
	return r, docs.Err()
}
func Exclude(ctx context.Context, db *sql.DB, kind, value string) error {
	if kind != "project" && kind != "path" && kind != "session" && kind != "client" {
		return errors.New("exclusion kind must be project, path, session, or client")
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("exclusion value is required")
	}
	_, err := db.ExecContext(ctx, "INSERT OR IGNORE INTO session_exclusions(kind,value) VALUES(?,?)", kind, filepath.Clean(value))
	if err != nil {
		return err
	}
	switch kind {
	case "project":
		_, err = db.ExecContext(ctx, "DELETE FROM session_documents WHERE (client,session_id) IN (SELECT client,session_id FROM session_metadata WHERE project=?)", filepath.Clean(value))
		if err == nil {
			_, err = db.ExecContext(ctx, "DELETE FROM session_metadata WHERE project=?", filepath.Clean(value))
		}
	case "path":
		_, err = db.ExecContext(ctx, "DELETE FROM session_documents WHERE (client,session_id) IN (SELECT client,session_id FROM session_metadata WHERE source_path=?)", filepath.Clean(value))
		if err == nil {
			_, err = db.ExecContext(ctx, "DELETE FROM session_metadata WHERE source_path=?", filepath.Clean(value))
		}
	case "session":
		_, err = db.ExecContext(ctx, "DELETE FROM session_documents WHERE session_id=?", value)
		if err == nil {
			_, err = db.ExecContext(ctx, "DELETE FROM session_metadata WHERE session_id=?", value)
		}
	case "client":
		_, err = db.ExecContext(ctx, "DELETE FROM session_documents WHERE client=?", value)
		if err == nil {
			_, err = db.ExecContext(ctx, "DELETE FROM session_metadata WHERE client=?", value)
		}
	}
	return err
}
func excluded(ctx context.Context, db *sql.DB, m Metadata) bool {
	rows, err := db.QueryContext(ctx, "SELECT kind,value FROM session_exclusions")
	if err != nil {
		return true
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		_ = rows.Scan(&k, &v)
		if (k == "project" && m.Project == v) || (k == "path" && m.SourcePath == v) || (k == "session" && m.SessionID == v) || (k == "client" && m.Client == v) {
			return true
		}
	}
	return false
}
func Rebuild(ctx context.Context, db *sql.DB, home string) (ScanResult, error) {
	if _, err := db.ExecContext(ctx, "DELETE FROM session_documents; DELETE FROM session_metadata; DELETE FROM session_sources"); err != nil {
		return ScanResult{}, err
	}
	return Scan(ctx, db, home)
}

// ScanCodexFixture remains a small test adapter for the explicit fixture
// protocol. Production callers use Scan, which discovers client source trees.
func ScanCodexFixture(ctx context.Context, db *sql.DB, path string) (int, error) {
	results, err := parseFile("codex", path)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, errors.New("missing session metadata")
	}
	n := 0
	clean := filepath.Clean(path)
	if _, err := db.ExecContext(ctx, "INSERT OR IGNORE INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)", clean, "fixture:"+clean, 0, []byte{}, 0, 0, "", 1, ParserVersion, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return 0, err
	}
	for _, r := range results {
		r.SourcePath = clean
		if err := replace(ctx, db, r); err != nil {
			return n, err
		}
		n += len(r.Documents)
	}
	return n, nil
}
