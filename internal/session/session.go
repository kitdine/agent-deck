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

	"github.com/kitdine/agent-deck/internal/activity"
)

const (
	ParserVersion                  = 5
	CodeSessionNotFound            = "session_not_found"
	replaceDocumentsSourcePath     = "agentdeck://replace-documents"
	replaceDocumentsSourceIdentity = "synthetic:replace-documents"
)

const MaxPageLimit = 1000

type Document struct {
	Client    string `json:"client"`
	SessionID string `json:"session_id"`
	EventAt   string `json:"event_at"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
}
type Metadata struct {
	Client     string `json:"client"`
	SessionID  string `json:"session_id"`
	Project    string `json:"project"`
	SourcePath string `json:"source_path"`
	Model      string `json:"model"`
	FirstAt    string `json:"first_at"`
	LastAt     string `json:"last_at"`
}
type Result struct {
	Metadata
	Documents       []Document        `json:"documents"`
	Activity        []activity.Detail `json:"activity,omitempty"`
	ActivitySummary *ActivitySummary  `json:"activity_summary,omitempty"`
	Signals         *WorkSignals      `json:"signals,omitempty"`
}

type WorkSignals struct {
	CostBasis        string `json:"cost_basis"`
	Kind             string `json:"kind,omitempty"`
	ToolCalls        int64  `json:"tool_calls"`
	FilesTouched     *int   `json:"files_touched"`
	FirstEditSeconds *int   `json:"first_edit_seconds"`
}
type Pagination struct {
	Page     int  `json:"page"`
	Limit    int  `json:"limit"`
	Total    int  `json:"total"`
	Shown    int  `json:"shown"`
	HasMore  bool `json:"has_more"`
	NextPage int  `json:"next_page,omitempty"`
}
type ActivitySummary struct {
	Total             int         `json:"total"`
	Completed         int         `json:"completed"`
	Failed            int         `json:"failed"`
	Incomplete        int         `json:"incomplete"`
	TotalDurationMS   int64       `json:"total_duration_ms"`
	AverageDurationMS *int64      `json:"average_duration_ms,omitempty"`
	ByTool            []ToolCount `json:"by_tool"`
}
type ToolCount struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

func Paginate[T any](values []T, page, limit int, all bool) ([]T, Pagination, error) {
	if page < 1 || limit < 1 || limit > MaxPageLimit {
		return nil, Pagination{}, fmt.Errorf("page must be positive and limit must be between 1 and %d", MaxPageLimit)
	}
	p := Pagination{Page: page, Limit: limit, Total: len(values)}
	if all {
		p.Page, p.Limit, p.Shown = 1, len(values), len(values)
		return values, p, nil
	}
	// Check the quotient first: page may be a user-supplied max-int value.
	if page-1 > len(values)/limit {
		return []T{}, p, nil
	}
	start := (page - 1) * limit
	end := start + limit
	if end > len(values) {
		end = len(values)
	}
	p.Shown, p.HasMore = end-start, end < len(values)
	if p.HasMore {
		p.NextPage = page + 1
	}
	return values[start:end], p, nil
}

func SummarizeActivity(values []activity.Detail) *ActivitySummary {
	s := &ActivitySummary{Total: len(values), ByTool: []ToolCount{}}
	tools := map[string]int{}
	known := int64(0)
	knownCount := int64(0)
	for _, v := range values {
		tools[v.Tool]++
		switch v.Status {
		case "completed":
			s.Completed++
		case "failed":
			s.Failed++
		default:
			s.Incomplete++
		}
		if v.DurationMS != nil {
			known += *v.DurationMS
			knownCount++
		}
	}
	s.TotalDurationMS = known
	if knownCount > 0 {
		average := known / knownCount
		s.AverageDurationMS = &average
	}
	for tool, count := range tools {
		s.ByTool = append(s.ByTool, ToolCount{tool, count})
	}
	sort.Slice(s.ByTool, func(i, j int) bool { return s.ByTool[i].Tool < s.ByTool[j].Tool })
	return s
}

// ActivitySummaryFromSourceSummary converts the bounded source-reader summary
// into the stable session-show JSON contract.
func ActivitySummaryFromSourceSummary(value activity.Summary) *ActivitySummary {
	summary := &ActivitySummary{
		Total:             value.Total,
		Completed:         value.Completed,
		Failed:            value.Failed,
		Incomplete:        value.Incomplete,
		TotalDurationMS:   value.TotalDurationMS,
		AverageDurationMS: value.AverageDurationMS,
		ByTool:            make([]ToolCount, 0, len(value.ByTool)),
	}
	for tool, count := range value.ByTool {
		summary.ByTool = append(summary.ByTool, ToolCount{Tool: tool, Count: count})
	}
	sort.Slice(summary.ByTool, func(i, j int) bool { return summary.ByTool[i].Tool < summary.ByTool[j].Tool })
	return summary
}

type ScanResult struct {
	Sources   int `json:"sources"`
	Documents int `json:"documents"`
	Skipped   int `json:"skipped"`
	Removed   int `json:"removed"`
}

// ScanProgress reports aggregate scan progress without source identifiers or content.
type ScanProgress struct {
	Processed int
	Total     int
	Documents int
	Skipped   int
}

// ScanProgressReporter receives synchronous scan lifecycle and aggregate progress updates.
type ScanProgressReporter interface {
	Start()
	Update(ScanProgress)
	Stop()
}

// ScanOptions configures optional observers for a session scan.
type ScanOptions struct {
	Progress ScanProgressReporter
}

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
	return ScanWithOptions(ctx, db, home, ScanOptions{})
}

// ScanWithOptions reads known session sources with optional aggregate progress reporting.
func ScanWithOptions(ctx context.Context, db *sql.DB, home string, options ScanOptions) (ScanResult, error) {
	if options.Progress != nil {
		options.Progress.Start()
		defer options.Progress.Stop()
	}
	return scan(ctx, db, home, options.Progress,
		func(src source) (bool, int, error) {
			return scanSource(ctx, db, src)
		},
		func(seen map[string]bool) error {
			return removeMissingSources(ctx, db, seen)
		},
	)
}

func scan(
	ctx context.Context,
	executor sessionExecutor,
	home string,
	progress ScanProgressReporter,
	scanOne func(source) (bool, int, error),
	removeMissing func(map[string]bool) error,
) (ScanResult, error) {
	before, err := visibleDocuments(ctx, executor)
	if err != nil {
		return ScanResult{}, err
	}
	var paths []source
	for _, root := range []struct {
		client, path string
		priority     int
	}{
		{"codex", filepath.Join(home, ".codex", "archived_sessions"), 0},
		{"claude", filepath.Join(home, ".claude", "projects"), 0},
		{"codex", filepath.Join(home, ".codex", "sessions"), 1},
	} {
		err = filepath.WalkDir(root.path, func(path string, d fs.DirEntry, err error) error {
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
	progressDocuments := 0
	if progress != nil {
		progress.Update(ScanProgress{Total: len(paths)})
	}
	seen := make(map[string]bool, len(paths))
	for index, p := range paths {
		seen[filepath.Clean(p.path)] = true
		changed, documents, err := scanOne(p)
		if err != nil {
			return result, err
		}
		if !changed {
			result.Skipped++
		} else {
			result.Sources++
			result.Documents += documents
			progressDocuments += documents
		}
		if progress != nil {
			progress.Update(ScanProgress{Processed: index + 1, Total: len(paths), Documents: progressDocuments, Skipped: result.Skipped})
		}
	}
	if err = removeMissing(seen); err != nil {
		return result, err
	}
	after, err := visibleDocuments(ctx, executor)
	if err != nil {
		return result, err
	}
	result.Documents, result.Removed = visibleDocumentChanges(before, after)
	if progress != nil {
		progress.Update(ScanProgress{Processed: len(paths), Total: len(paths), Documents: progressDocuments, Skipped: result.Skipped})
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

type sessionExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sourceUpdate struct {
	movedFrom  string
	path       string
	appendOnly bool
	writeState bool
	state      sourceState
	results    []Result
}

func scanSource(ctx context.Context, db *sql.DB, src source) (bool, int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	update, changed, err := prepareSourceUpdate(ctx, tx, src)
	if err != nil {
		return false, 0, err
	}
	if !changed && update.movedFrom == "" {
		return false, 0, nil
	}
	docs, err := applySourceUpdate(ctx, tx, update)
	if err != nil {
		return false, 0, err
	}
	if err = tx.Commit(); err != nil {
		return false, 0, err
	}
	return changed, docs, nil
}

func scanSourceExec(ctx context.Context, executor sessionExecutor, src source) (bool, int, error) {
	update, changed, err := prepareSourceUpdate(ctx, executor, src)
	if err != nil {
		return false, 0, err
	}
	if !changed && update.movedFrom == "" {
		return false, 0, nil
	}
	docs, err := applySourceUpdate(ctx, executor, update)
	if err != nil {
		return false, 0, err
	}
	return changed, docs, nil
}

func prepareSourceUpdate(ctx context.Context, executor sessionExecutor, src source) (sourceUpdate, bool, error) {
	path := filepath.Clean(src.path)
	info, err := os.Stat(path)
	if err != nil {
		return sourceUpdate{}, false, err
	}
	identity, err := fileIdentity(info)
	if err != nil {
		return sourceUpdate{}, false, err
	}
	prefix, err := prefixHash(path, info.Size())
	if err != nil {
		return sourceUpdate{}, false, err
	}
	state, found, err := loadSource(ctx, executor, path)
	if err != nil {
		return sourceUpdate{}, false, err
	}
	update := sourceUpdate{path: path}
	if !found {
		// A rename preserves source ownership and avoids a full index rebuild.
		state, found, err = loadSourceByIdentity(ctx, executor, identity)
		if err != nil {
			return sourceUpdate{}, false, err
		}
		if found {
			update.movedFrom = state.path
			state.path = path
		}
	}
	oldPrefix := ""
	if found {
		oldPrefix, err = prefixHash(path, state.cursor)
		if err != nil {
			return sourceUpdate{}, false, err
		}
	}
	unchanged := found && state.identity == identity && state.parserVersion == ParserVersion && info.Size() == state.size && state.modifiedAt == info.ModTime().UnixNano() && prefix == state.prefixHash && state.priority == int64(src.priority)
	if unchanged {
		return update, false, nil
	}
	update.appendOnly = found && state.identity == identity && state.parserVersion == ParserVersion && info.Size() > state.cursor && oldPrefix == state.prefixHash
	var partial []byte
	if update.appendOnly {
		update.results, partial, err = parseRange(src.client, path, state.cursor, state.partial)
	} else {
		update.results, partial, err = parseRange(src.client, path, 0, nil)
	}
	if err != nil {
		return sourceUpdate{}, false, err
	}
	if partial == nil {
		partial = []byte{}
	}
	update.writeState = true
	update.state = sourceState{path: path, identity: identity, cursor: info.Size(), size: info.Size(), modifiedAt: info.ModTime().UnixNano(), prefixHash: prefix, priority: int64(src.priority), parserVersion: ParserVersion, partial: partial}
	return update, true, nil
}

func applySourceUpdate(ctx context.Context, executor sessionExecutor, update sourceUpdate) (int, error) {
	if update.movedFrom != "" {
		if err := moveSource(ctx, executor, update.movedFrom, update.path); err != nil {
			return 0, err
		}
	}
	if !update.writeState {
		return 0, nil
	}
	if !update.appendOnly {
		if err := deleteSource(ctx, executor, update.path); err != nil {
			return 0, err
		}
	}
	if err := saveSource(ctx, executor, update.state); err != nil {
		return 0, err
	}
	docs := 0
	for _, r := range update.results {
		r.SourcePath = update.path
		excluded, err := excluded(ctx, executor, r.Metadata)
		if err != nil {
			return 0, err
		}
		if excluded {
			continue
		}
		if err = insertResult(ctx, executor, r); err != nil {
			return 0, err
		}
		docs += len(r.Documents)
	}
	return docs, nil
}

func loadSource(ctx context.Context, executor sessionExecutor, path string) (sourceState, bool, error) {
	var s sourceState
	s.path = path
	err := executor.QueryRowContext(ctx, "SELECT identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources WHERE source_path=?", path).Scan(&s.identity, &s.cursor, &s.partial, &s.size, &s.modifiedAt, &s.prefixHash, &s.priority, &s.parserVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}
func loadSourceByIdentity(ctx context.Context, executor sessionExecutor, identity string) (sourceState, bool, error) {
	var s sourceState
	err := executor.QueryRowContext(ctx, "SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources WHERE identity=?", identity).Scan(&s.path, &s.identity, &s.cursor, &s.partial, &s.size, &s.modifiedAt, &s.prefixHash, &s.priority, &s.parserVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}
func moveSource(ctx context.Context, executor sessionExecutor, old, new string) error {
	if _, err := executor.ExecContext(ctx, "INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) SELECT ?,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at FROM session_sources WHERE source_path=?", new, old); err != nil {
		return err
	}
	if _, err := executor.ExecContext(ctx, "UPDATE session_documents SET source_path=? WHERE source_path=?; UPDATE session_metadata SET source_path=? WHERE source_path=?; DELETE FROM session_sources WHERE source_path=?", new, old, new, old, old); err != nil {
		return err
	}
	return nil
}
func deleteSource(ctx context.Context, executor sessionExecutor, path string) error {
	if _, err := executor.ExecContext(ctx, "DELETE FROM session_documents WHERE source_path=?", path); err != nil {
		return err
	}
	if _, err := executor.ExecContext(ctx, "DELETE FROM session_metadata WHERE source_path=?", path); err != nil {
		return err
	}
	return nil
}
func saveSource(ctx context.Context, executor sessionExecutor, s sourceState) error {
	_, err := executor.ExecContext(ctx, "INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(source_path) DO UPDATE SET identity=excluded.identity,cursor=excluded.cursor,partial_line=excluded.partial_line,size=excluded.size,modified_at=excluded.modified_at,prefix_hash=excluded.prefix_hash,priority=excluded.priority,parser_version=excluded.parser_version,scanned_at=excluded.scanned_at", s.path, s.identity, s.cursor, s.partial, s.size, s.modifiedAt, s.prefixHash, s.priority, s.parserVersion, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func removeMissingSources(ctx context.Context, db *sql.DB, seen map[string]bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = removeMissingSourcesExec(ctx, tx, seen); err != nil {
		return err
	}
	return tx.Commit()
}

func removeMissingSourcesExec(ctx context.Context, executor sessionExecutor, seen map[string]bool) error {
	rows, err := executor.QueryContext(ctx, "SELECT source_path FROM session_sources WHERE identity != ?", replaceDocumentsSourceIdentity)
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
	if err = rows.Close(); err != nil {
		return err
	}
	for _, p := range missing {
		if err = deleteSource(ctx, executor, p); err != nil {
			return err
		}
		if _, err = executor.ExecContext(ctx, "DELETE FROM session_sources WHERE source_path=?", p); err != nil {
			return err
		}
	}
	return nil
}

type visibleDocument struct {
	eventAt string
	kind    string
	text    string
}

func visibleDocuments(ctx context.Context, executor sessionExecutor) (map[string][]visibleDocument, error) {
	rows, err := executor.QueryContext(ctx, `WITH visible AS (SELECT m.source_path,m.client,m.session_id,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT d.client,d.session_id,d.event_at,d.kind,d.text FROM session_documents d JOIN visible v ON v.source_path=d.source_path AND v.client=d.client AND v.session_id=d.session_id WHERE v.n=1 ORDER BY d.client,d.session_id,d.rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]visibleDocument{}
	for rows.Next() {
		var client, sessionID string
		var document visibleDocument
		if err = rows.Scan(&client, &sessionID, &document.eventAt, &document.kind, &document.text); err != nil {
			return nil, err
		}
		key := client + "\x00" + sessionID
		out[key] = append(out[key], document)
	}
	return out, rows.Err()
}

func visibleDocumentChanges(before, after map[string][]visibleDocument) (changed, removed int) {
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	for key := range keys {
		keyChanged, keyRemoved := logicalDocumentChanges(before[key], after[key])
		changed += keyChanged
		removed += keyRemoved
	}
	return changed, removed
}

type documentChangeCount struct {
	changed int
	removed int
}

func logicalDocumentChanges(before, after []visibleDocument) (changed, removed int) {
	before, after = trimCommonDocumentWindow(before, after)
	if len(before) == 0 {
		return len(after), 0
	}
	if len(after) == 0 {
		return 0, len(before)
	}
	previous := make([]documentChangeCount, len(after)+1)
	current := make([]documentChangeCount, len(after)+1)
	for index := 1; index <= len(after); index++ {
		previous[index].changed = index
	}
	for beforeIndex, oldDocument := range before {
		current[0] = documentChangeCount{removed: beforeIndex + 1}
		for afterIndex, newDocument := range after {
			if oldDocument == newDocument {
				current[afterIndex+1] = previous[afterIndex]
				continue
			}
			replaced := previous[afterIndex]
			replaced.changed++
			inserted := current[afterIndex]
			inserted.changed++
			deleted := previous[afterIndex+1]
			deleted.removed++
			current[afterIndex+1] = bestDocumentChange(replaced, inserted, deleted)
		}
		previous, current = current, previous
	}
	result := previous[len(after)]
	return result.changed, result.removed
}

func trimCommonDocumentWindow(before, after []visibleDocument) ([]visibleDocument, []visibleDocument) {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	before, after = before[prefix:], after[prefix:]
	suffix := 0
	for suffix < len(before) && suffix < len(after) && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	return before[:len(before)-suffix], after[:len(after)-suffix]
}

func bestDocumentChange(candidates ...documentChangeCount) documentChangeCount {
	// Equal-cost rewrites prefer update semantics over a remove-plus-add split.
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		candidateTotal := candidate.changed + candidate.removed
		bestTotal := best.changed + best.removed
		if candidateTotal < bestTotal || (candidateTotal == bestTotal && candidate.removed < best.removed) {
			best = candidate
		}
	}
	return best
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
			r.Project = NormalizeProject(filepath.Dir(path))
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
			r.Metadata.Project = NormalizeProject(filepath.Dir(path))
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
		d.EventAt = normalizedEventAt(str(v["timestamp"]), str(p["timestamp"]))
		return d
	case "visible_assistant_final":
		d, _ := ApprovedDocument(client, id, "assistant_final", str(p["text"]))
		d.EventAt = normalizedEventAt(str(v["timestamp"]), str(p["timestamp"]))
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
		d.EventAt = normalizedEventAt(str(v["timestamp"]), str(p["timestamp"]))
		return id, d, m
	}
	if typ == "visible_assistant_final" {
		d, _ := ApprovedDocument("codex", id, "assistant_final", str(p["text"]))
		d.EventAt = normalizedEventAt(str(v["timestamp"]), str(p["timestamp"]))
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
	d.EventAt = normalizedEventAt(str(v["timestamp"]), str(p["timestamp"]))
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
	if typ == "assistant" && m.Model == "" {
		m.Model = str(msg["model"])
	}
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
	d.EventAt = normalizedEventAt(str(v["timestamp"]))
	return id, d, m
}

func normalizedEventAt(values ...string) string {
	for _, value := range values {
		at, err := time.Parse(time.RFC3339Nano, value)
		if err == nil {
			return at.UTC().Format(time.RFC3339Nano)
		}
	}
	return ""
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
	return Metadata{Client: client, SessionID: id, Project: NormalizeProject(first(str(a["cwd"]), str(a["project"]), str(b["cwd"]), str(b["project"]))), Model: first(str(a["model"]), str(b["model"])), FirstAt: first(str(a["timestamp"]), str(b["timestamp"])), LastAt: first(str(a["timestamp"]), str(b["timestamp"]))}
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

// NormalizeProject returns the project identity stored for a client session.
func NormalizeProject(v string) string {
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
	if err = replaceExec(ctx, tx, r); err != nil {
		return err
	}
	return tx.Commit()
}
func replaceExec(ctx context.Context, executor sessionExecutor, r Result) error {
	if _, err := executor.ExecContext(ctx, "DELETE FROM session_documents WHERE source_path=? AND client=? AND session_id=?", r.SourcePath, r.Client, r.SessionID); err != nil {
		return err
	}
	return insertResult(ctx, executor, r)
}
func insertResult(ctx context.Context, executor sessionExecutor, r Result) error {
	for _, d := range r.Documents {
		if _, err := executor.ExecContext(ctx, "INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,?,?,?)", r.SourcePath, d.Client, d.SessionID, d.EventAt, d.Kind, d.Text); err != nil {
			return err
		}
	}
	_, err := executor.ExecContext(ctx, "INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(source_path,client,session_id) DO UPDATE SET project=CASE WHEN excluded.project='' THEN session_metadata.project ELSE excluded.project END,model=CASE WHEN excluded.model='' THEN session_metadata.model ELSE excluded.model END,parser_version=excluded.parser_version,first_at=CASE WHEN session_metadata.first_at='' OR excluded.first_at<session_metadata.first_at THEN excluded.first_at ELSE session_metadata.first_at END,last_at=CASE WHEN excluded.last_at>session_metadata.last_at THEN excluded.last_at ELSE session_metadata.last_at END", r.SourcePath, r.Client, r.SessionID, r.Project, r.Model, ParserVersion, r.FirstAt, r.LastAt)
	return err
}
func ReplaceDocuments(ctx context.Context, db *sql.DB, client, sessionID string, docs []Document) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)", replaceDocumentsSourcePath, replaceDocumentsSourceIdentity, 0, []byte{}, 0, 0, "", 1, ParserVersion, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if err = replaceExec(ctx, tx, Result{Metadata: Metadata{Client: client, SessionID: sessionID, SourcePath: replaceDocumentsSourcePath}, Documents: docs}); err != nil {
		return err
	}
	return tx.Commit()
}
func Search(ctx context.Context, db *sql.DB, query string) ([]Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("search query is required")
	}
	rows, err := db.QueryContext(ctx, `WITH visible AS (SELECT m.source_path,m.client,m.session_id,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT d.client,d.session_id,d.event_at,d.kind,d.text FROM session_documents d JOIN visible v ON v.source_path=d.source_path AND v.client=d.client AND v.session_id=d.session_id WHERE v.n=1 AND session_documents MATCH ? ORDER BY CASE WHEN d.event_at='' THEN 1 ELSE 0 END,d.event_at DESC,d.client,d.session_id,d.rowid`, query)
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
func List(ctx context.Context, db *sql.DB) ([]Metadata, error) {
	rows, err := db.QueryContext(ctx, `WITH visible AS (SELECT m.*,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT client,session_id,project,source_path,model,first_at,last_at FROM visible WHERE n=1 ORDER BY last_at DESC,client,session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Metadata, 0)
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
	return ShowWithActivity(ctx, db, client, id, false)
}

// ShowMetadata returns the selected authoritative source metadata for one session.
func ShowMetadata(ctx context.Context, db *sql.DB, client, id string) (Metadata, error) {
	var metadata Metadata
	err := db.QueryRowContext(ctx, `SELECT m.client,m.session_id,m.project,m.source_path,m.model,m.first_at,m.last_at FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path WHERE m.client=? AND m.session_id=? ORDER BY s.priority DESC,m.source_path LIMIT 1`, client, id).Scan(&metadata.Client, &metadata.SessionID, &metadata.Project, &metadata.SourcePath, &metadata.Model, &metadata.FirstAt, &metadata.LastAt)
	return metadata, err
}

// DocumentsPage returns a deterministic selected-source page without retaining
// off-page approved document text.
func DocumentsPage(ctx context.Context, db *sql.DB, metadata Metadata, page, limit int, all bool) ([]Document, Pagination, error) {
	if page < 1 || limit < 1 || limit > MaxPageLimit {
		return nil, Pagination{}, fmt.Errorf("page must be positive and limit must be between 1 and %d", MaxPageLimit)
	}
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_documents WHERE source_path=? AND client=? AND session_id=?`, metadata.SourcePath, metadata.Client, metadata.SessionID).Scan(&total); err != nil {
		return nil, Pagination{}, err
	}
	pagination := Pagination{Page: page, Limit: limit, Total: total}
	offset, queryLimit := 0, limit
	if all {
		pagination.Page, pagination.Limit = 1, total
		queryLimit = total
	} else {
		if page-1 > total/limit {
			return []Document{}, pagination, nil
		}
		offset = (page - 1) * limit
	}
	if total == 0 {
		return []Document{}, pagination, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT client,session_id,event_at,kind,text FROM session_documents WHERE source_path=? AND client=? AND session_id=? ORDER BY rowid LIMIT ? OFFSET ?`, metadata.SourcePath, metadata.Client, metadata.SessionID, queryLimit, offset)
	if err != nil {
		return nil, Pagination{}, err
	}
	defer rows.Close()
	documents := make([]Document, 0, queryLimit)
	for rows.Next() {
		var document Document
		if err := rows.Scan(&document.Client, &document.SessionID, &document.EventAt, &document.Kind, &document.Text); err != nil {
			return nil, Pagination{}, err
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, Pagination{}, err
	}
	pagination.Shown = len(documents)
	pagination.HasMore = offset+len(documents) < total
	if pagination.HasMore {
		pagination.NextPage = page + 1
	}
	return documents, pagination, nil
}

func ShowWithActivity(ctx context.Context, db *sql.DB, client, id string, includeActivity bool) (Result, error) {
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
	if err = docs.Err(); err != nil {
		return r, err
	}
	if includeActivity {
		r.Activity, err = activity.ReadDetails(r.SourcePath, client, id)
		if err == nil {
			r.ActivitySummary = SummarizeActivity(r.Activity)
		}
	}
	return r, err
}
func Exclude(ctx context.Context, db *sql.DB, kind, value string) error {
	if kind != "project" && kind != "path" && kind != "session" && kind != "client" {
		return errors.New("exclusion kind must be project, path, session, or client")
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("exclusion value is required")
	}
	if kind == "project" || kind == "path" {
		value = filepath.Clean(value)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO session_exclusions(kind,value) VALUES(?,?)", kind, value); err != nil {
		return err
	}
	switch kind {
	case "project":
		_, err = tx.ExecContext(ctx, "DELETE FROM session_documents WHERE (source_path,client,session_id) IN (SELECT source_path,client,session_id FROM session_metadata WHERE project=?)", value)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM session_metadata WHERE project=?", value)
		}
	case "path":
		_, err = tx.ExecContext(ctx, "DELETE FROM session_documents WHERE source_path=?", value)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM session_metadata WHERE source_path=?", value)
		}
	case "session":
		_, err = tx.ExecContext(ctx, "DELETE FROM session_documents WHERE session_id=?", value)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM session_metadata WHERE session_id=?", value)
		}
	case "client":
		_, err = tx.ExecContext(ctx, "DELETE FROM session_documents WHERE client=?", value)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM session_metadata WHERE client=?", value)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func excluded(ctx context.Context, executor sessionExecutor, m Metadata) (bool, error) {
	rows, err := executor.QueryContext(ctx, "SELECT kind,value FROM session_exclusions")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err = rows.Scan(&k, &v); err != nil {
			return false, err
		}
		if (k == "project" && m.Project == v) || (k == "path" && m.SourcePath == v) || (k == "session" && m.SessionID == v) || (k == "client" && m.Client == v) {
			return true, nil
		}
	}
	return false, rows.Err()
}
func Rebuild(ctx context.Context, db *sql.DB, home string) (ScanResult, error) {
	return RebuildWithOptions(ctx, db, home, ScanOptions{})
}

// RebuildWithOptions replaces the rebuildable session index with optional aggregate progress reporting.
func RebuildWithOptions(ctx context.Context, db *sql.DB, home string, options ScanOptions) (ScanResult, error) {
	if options.Progress != nil {
		options.Progress.Start()
		defer options.Progress.Stop()
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ScanResult{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM session_documents; DELETE FROM session_metadata; DELETE FROM session_sources"); err != nil {
		return ScanResult{}, err
	}
	result, err := scan(ctx, tx, home, options.Progress,
		func(src source) (bool, int, error) {
			return scanSourceExec(ctx, tx, src)
		},
		func(seen map[string]bool) error {
			return removeMissingSourcesExec(ctx, tx, seen)
		},
	)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return ScanResult{}, err
	}
	return result, nil
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
