package session

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// cleanupFixture describes one seeded session index: file sources, the OTel
// and replace-documents sources that file inventory never removes, and which
// file sources the current inventory still sees.
type cleanupFixture struct {
	files   int
	docs    int
	textLen int
	keep    func(index int) bool
}

func (f cleanupFixture) path(index int) string {
	return fmt.Sprintf("/fixture/.codex/sessions/2026/11/%05d.jsonl", index)
}

func (f cleanupFixture) seen() map[string]bool {
	seen := map[string]bool{}
	for index := range f.files {
		if f.keep(index) {
			seen[f.path(index)] = true
		}
	}
	return seen
}

func (f cleanupFixture) missing() int {
	count := 0
	for index := range f.files {
		if !f.keep(index) {
			count++
		}
	}
	return count
}

func openCleanupIndex(t testing.TB, f cleanupFixture) *store.Store {
	t.Helper()
	ctx := context.Background()
	database, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	filler := strings.Repeat("transcript filler ", max(1, f.textLen/18))
	insert := func(path, identity, client, sessionID string) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,10,10,1,'hash',1,1,'2026-11-27T00:00:00Z')`, path, identity); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_metadata(source_path,client,session_id,parser_version,first_at,last_at) VALUES(?,?,?,1,'2026-11-27T00:00:00Z','2026-11-27T00:00:01Z')`, path, client, sessionID); err != nil {
			t.Fatal(err)
		}
		for doc := range f.docs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,'2026-11-27T00:00:00Z','user_prompt',?)`, path, client, sessionID, fmt.Sprintf("needle%d %s %d", doc, filler, doc)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Interleave the protected sources with file sources so rowids of
	// removed and retained documents alternate.
	for index := range f.files {
		insert(f.path(index), fmt.Sprintf("ino:%d", index), "codex", fmt.Sprintf("s-%05d", index))
		if index == f.files/2 {
			insert("otel:codex/background", "otel:codex/background", "codex", "otel-session")
			insert(replaceDocumentsSourcePath, replaceDocumentsSourceIdentity, "claude", "replaced-session")
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return database
}

// legacyRemoveMissingSources is the per-path cleanup this change replaced,
// kept as the result oracle.
func legacyRemoveMissingSources(ctx context.Context, db *sql.DB, seen map[string]bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT source_path FROM session_sources WHERE identity != ? AND substr(source_path,1,5) != 'otel:'", replaceDocumentsSourceIdentity)
	if err != nil {
		return err
	}
	var missing []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	if err = rows.Close(); err != nil {
		return err
	}
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

func servingRevision(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var revision int64
	if err := db.QueryRow(`SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestRemoveMissingSourcesClearsEveryMissingSourceAndKeepsTheRest(t *testing.T) {
	ctx := context.Background()
	fixture := cleanupFixture{files: 40, docs: 3, textLen: 40, keep: func(index int) bool { return index%3 == 0 }}
	database := openCleanupIndex(t, fixture)
	db := database.DB
	beforeRevision := servingRevision(t, db)
	keptDocuments := snapshotSessionRows(t, db, `SELECT rowid,source_path,client,session_id,event_at,kind,text FROM session_documents WHERE source_path IN (SELECT source_path FROM session_sources WHERE source_path NOT LIKE '/fixture/%' OR CAST(substr(source_path,-11,5) AS INTEGER) % 3 = 0) ORDER BY rowid`)
	if err := removeMissingSources(ctx, db, fixture.seen()); err != nil {
		t.Fatal(err)
	}
	for index := range fixture.files {
		path := fixture.path(index)
		want := 0
		if fixture.keep(index) {
			want = 1
		}
		if got := countRows(t, db, `SELECT count(*) FROM session_sources WHERE source_path=?`, path); got != want {
			t.Fatalf("%s registry rows=%d, want %d", path, got, want)
		}
		if got := countRows(t, db, `SELECT count(*) FROM session_metadata WHERE source_path=?`, path); got != want {
			t.Fatalf("%s metadata rows=%d, want %d", path, got, want)
		}
		if got := countRows(t, db, `SELECT count(*) FROM session_documents WHERE source_path=?`, path); got != want*fixture.docs {
			t.Fatalf("%s documents=%d, want %d", path, got, want*fixture.docs)
		}
	}
	for _, protected := range []string{"otel:codex/background", replaceDocumentsSourcePath} {
		if countRows(t, db, `SELECT count(*) FROM session_sources WHERE source_path=?`, protected) != 1 || countRows(t, db, `SELECT count(*) FROM session_documents WHERE source_path=?`, protected) != fixture.docs {
			t.Fatalf("file inventory removed protected source %s", protected)
		}
	}
	if got := snapshotSessionRows(t, db, `SELECT rowid,source_path,client,session_id,event_at,kind,text FROM session_documents ORDER BY rowid`); !reflect.DeepEqual(got, keptDocuments) {
		t.Fatalf("retained documents changed:\n got %d rows\nwant %d rows", len(got), len(keptDocuments))
	}
	if after := servingRevision(t, db); after <= beforeRevision {
		t.Fatalf("serving revision %d did not advance from %d", after, beforeRevision)
	}
	if hits, err := Search(ctx, db, "needle1"); err != nil || len(hits) != 14+2 {
		t.Fatalf("search after cleanup hits=%d err=%v, want 14 file sessions and 2 protected sessions", len(hits), err)
	}
	// A second reconciliation with the same inventory finds nothing to do.
	settled := servingRevision(t, db)
	if err := removeMissingSources(ctx, db, fixture.seen()); err != nil {
		t.Fatal(err)
	}
	if after := servingRevision(t, db); after != settled {
		t.Fatalf("idempotent cleanup moved the serving revision %d -> %d", settled, after)
	}
}

func TestRemoveMissingSourcesMatchesThePerPathCleanup(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range []cleanupFixture{
		{files: 60, docs: 2, textLen: 30, keep: func(index int) bool { return index%4 == 1 }},
		{files: 25, docs: 4, textLen: 30, keep: func(int) bool { return false }},
		{files: 25, docs: 1, textLen: 30, keep: func(int) bool { return true }},
	} {
		batched := openCleanupIndex(t, fixture)
		legacy := openCleanupIndex(t, fixture)
		if !reflect.DeepEqual(captureSessionIndex(t, batched.DB), captureSessionIndex(t, legacy.DB)) {
			t.Fatal("fixtures differ before cleanup")
		}
		if err := removeMissingSources(ctx, batched.DB, fixture.seen()); err != nil {
			t.Fatal(err)
		}
		if err := legacyRemoveMissingSources(ctx, legacy.DB, fixture.seen()); err != nil {
			t.Fatal(err)
		}
		got, want := captureSessionIndex(t, batched.DB), captureSessionIndex(t, legacy.DB)
		if !reflect.DeepEqual(got.sources, want.sources) {
			t.Fatalf("missing=%d: registry differs from per-path cleanup:\n got %v\nwant %v", fixture.missing(), got.sources, want.sources)
		}
		if !reflect.DeepEqual(got.metadata, want.metadata) {
			t.Fatalf("missing=%d: metadata differs from per-path cleanup:\n got %v\nwant %v", fixture.missing(), got.metadata, want.metadata)
		}
		if !reflect.DeepEqual(got.documents, want.documents) {
			t.Fatalf("missing=%d: FTS documents differ from per-path cleanup: got %d rows, want %d", fixture.missing(), len(got.documents), len(want.documents))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("missing=%d: session index differs from per-path cleanup", fixture.missing())
		}
		if _, err := batched.DB.Exec(`INSERT INTO session_documents(session_documents) VALUES('integrity-check')`); err != nil {
			t.Fatalf("FTS integrity after batched cleanup: %v", err)
		}
	}
}

func TestRemoveMissingSourcesRollsBackEverythingWhenALaterDeleteFails(t *testing.T) {
	ctx := context.Background()
	fixture := cleanupFixture{files: 30, docs: 2, textLen: 30, keep: func(index int) bool { return index%2 == 0 }}
	database := openCleanupIndex(t, fixture)
	db := database.DB
	// The registry delete is the last statement of the cleanup: documents and
	// metadata of every missing source have already been deleted in the
	// transaction when it fails.
	lastMissing := fixture.path(fixture.files - 1)
	if _, err := db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_cleanup BEFORE DELETE ON session_sources WHEN old.source_path='%s' BEGIN SELECT RAISE(ABORT,'synthetic registry delete failure'); END`, lastMissing)); err != nil {
		t.Fatal(err)
	}
	before := captureSessionIndex(t, db)
	beforeRevision := servingRevision(t, db)
	err := removeMissingSources(ctx, db, fixture.seen())
	if err == nil || !strings.Contains(err.Error(), "synthetic registry delete failure") {
		t.Fatalf("cleanup error=%v, want the synthetic registry failure", err)
	}
	after := captureSessionIndex(t, db)
	if !reflect.DeepEqual(after.documents, before.documents) {
		t.Fatalf("FTS documents not rolled back: %d rows, want %d", len(after.documents), len(before.documents))
	}
	if !reflect.DeepEqual(after.metadata, before.metadata) {
		t.Fatalf("metadata not rolled back: %d rows, want %d", len(after.metadata), len(before.metadata))
	}
	if !reflect.DeepEqual(after.sources, before.sources) {
		t.Fatalf("registry not rolled back: %d rows, want %d", len(after.sources), len(before.sources))
	}
	if revision := servingRevision(t, db); revision != beforeRevision {
		t.Fatalf("serving revision %d after rollback, want %d", revision, beforeRevision)
	}
}

// planRecordingExecutor explains every statement that reads or writes
// session_documents before running it in the same transaction.
type planRecordingExecutor struct {
	t     *testing.T
	tx    *sql.Tx
	plans [][]string
}

func (e *planRecordingExecutor) explain(ctx context.Context, query string, args ...any) {
	e.t.Helper()
	if !strings.Contains(query, "session_documents") {
		return
	}
	statement := strings.TrimPrefix(query, searchTextChanged)
	rows, err := e.tx.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
	if err != nil {
		e.t.Fatalf("explain %q: %v", statement, err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			e.t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	e.plans = append(e.plans, details)
}

func (e *planRecordingExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	e.explain(ctx, query, args...)
	return e.tx.ExecContext(ctx, query, args...)
}

func (e *planRecordingExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	e.explain(ctx, query, args...)
	return e.tx.QueryContext(ctx, query, args...)
}

func (e *planRecordingExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	e.explain(ctx, query, args...)
	return e.tx.QueryRowContext(ctx, query, args...)
}

var ftsFullScan = regexp.MustCompile(`^SCAN session_documents VIRTUAL TABLE INDEX \d+:$`)

func TestRemoveMissingSourcesScansTheFTSIndexOnceForAllMissingSources(t *testing.T) {
	ctx := context.Background()
	fixture := cleanupFixture{files: 1500, docs: 3, textLen: 30, keep: func(index int) bool { return index%10 == 0 }}
	database := openCleanupIndex(t, fixture)
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	recorder := &planRecordingExecutor{t: t, tx: tx}
	if err := removeMissingSourcesExec(ctx, recorder, fixture.seen()); err != nil {
		t.Fatal(err)
	}
	fullScans, rowidDeletes := 0, 0
	for _, plan := range recorder.plans {
		for _, detail := range plan {
			if ftsFullScan.MatchString(detail) {
				fullScans++
			}
			if strings.HasPrefix(detail, "SCAN session_documents VIRTUAL TABLE INDEX ") && strings.HasSuffix(detail, ":=") {
				rowidDeletes++
			}
		}
	}
	if fullScans != 1 {
		t.Fatalf("cleanup of %d missing sources scanned the FTS table %d times, want once; plans=%v", fixture.missing(), fullScans, recorder.plans[:min(len(recorder.plans), 4)])
	}
	if rowidDeletes != 1 {
		t.Fatalf("FTS rowid deletes=%d, want one rowid-keyed statement; plans=%v", rowidDeletes, recorder.plans)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, database.DB, `SELECT count(*) FROM session_documents`); got != (fixture.files-fixture.missing()+2)*fixture.docs {
		t.Fatalf("documents after cleanup=%d, want %d", got, (fixture.files-fixture.missing()+2)*fixture.docs)
	}
}

// BenchmarkRemoveMissingSources compares the batched cleanup with the
// per-path cleanup it replaced on an index shaped like the diagnosed cold
// state: about 15,000 FTS documents and 2,900 missing sources.
func BenchmarkRemoveMissingSources(b *testing.B) {
	fixture := cleanupFixture{files: 3000, docs: 5, textLen: 1500, keep: func(index int) bool { return index%30 == 0 }}
	for _, variant := range []struct {
		name    string
		cleanup func(context.Context, *sql.DB, map[string]bool) error
	}{
		{"batched", removeMissingSources},
		{"per-path", legacyRemoveMissingSources},
	} {
		b.Run(variant.name, func(b *testing.B) {
			for range b.N {
				b.StopTimer()
				database := openCleanupIndex(b, fixture)
				seen := fixture.seen()
				b.StartTimer()
				if err := variant.cleanup(context.Background(), database.DB, seen); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// openByteIndex seeds one source per path, with metadata and documents, the
// way the per-path writers store them: each path bound as its own parameter.
func openByteIndex(t *testing.T, paths []string) *store.Store {
	t.Helper()
	ctx := context.Background()
	database, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	for index, path := range paths {
		if _, err := database.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,10,10,1,'hash',1,1,'2026-11-27T00:00:00Z')`, path, fmt.Sprintf("ino:%d", index)); err != nil {
			t.Fatal(err)
		}
		if _, err := database.DB.ExecContext(ctx, `INSERT INTO session_metadata(source_path,client,session_id,parser_version,first_at,last_at) VALUES(?,'codex',?,1,'2026-11-27T00:00:00Z','2026-11-27T00:00:01Z')`, path, fmt.Sprintf("s-%d", index)); err != nil {
			t.Fatal(err)
		}
		for doc := range 2 {
			if _, err := database.DB.ExecContext(ctx, `INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,'codex',?,'2026-11-27T00:00:00Z','user_prompt',?)`, path, fmt.Sprintf("s-%d", index), fmt.Sprintf("needle%d text", doc)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return database
}

func rowsForPathBytes(t *testing.T, db *sql.DB, table, path string) int {
	t.Helper()
	return countRows(t, db, `SELECT count(*) FROM `+table+` WHERE CAST(source_path AS BLOB)=?`, []byte(path))
}

// A stored path keeps the bytes of its source path, which need not be valid
// UTF-8. Cleanup must match them byte for byte: a missing path with an
// invalid byte must go, and a seen path that differs only in how that byte
// would be repaired (U+FFFD) must stay.
func TestRemoveMissingSourcesMatchesInvalidUTF8PathsByteForByte(t *testing.T) {
	ctx := context.Background()
	invalid := "/fixture/.codex/sessions/rollout-\xff.jsonl"
	repaired := "/fixture/.codex/sessions/rollout-�.jsonl"
	plain := "/fixture/.codex/sessions/rollout-plain.jsonl"
	paths := []string{invalid, repaired, plain}
	seen := map[string]bool{repaired: true, plain: true}
	batched := openByteIndex(t, paths)
	legacy := openByteIndex(t, paths)
	if err := removeMissingSources(ctx, batched.DB, seen); err != nil {
		t.Fatal(err)
	}
	if err := legacyRemoveMissingSources(ctx, legacy.DB, seen); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"session_sources", "session_metadata", "session_documents"} {
		perSource := map[string]int{"session_sources": 1, "session_metadata": 1, "session_documents": 2}[table]
		if got := rowsForPathBytes(t, batched.DB, table, invalid); got != 0 {
			t.Errorf("%s keeps %d rows of the missing invalid-UTF-8 path", table, got)
		}
		for _, kept := range []string{repaired, plain} {
			if got := rowsForPathBytes(t, batched.DB, table, kept); got != perSource {
				t.Errorf("%s has %d rows of the seen path %q, want %d", table, got, kept, perSource)
			}
		}
	}
	got, want := captureSessionIndex(t, batched.DB), captureSessionIndex(t, legacy.DB)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("byte-exact cleanup differs from the per-path cleanup:\n got %q\nwant %q", got.sources, want.sources)
	}
	if _, err := batched.DB.Exec(`INSERT INTO session_documents(session_documents) VALUES('integrity-check')`); err != nil {
		t.Fatalf("FTS integrity after cleanup: %v", err)
	}
}
