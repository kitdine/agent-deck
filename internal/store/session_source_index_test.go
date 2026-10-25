package store

import (
	"context"
	"strings"
	"testing"
)

func TestExistingSessionIndexGainsBoundedIdentityLookupWithoutRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/first", "/alias"} {
		_, err = db.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?, 'same-inode', 7, 7, 1, 'anchor', 0, 1, 'original')`, path)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES('/first','codex','s','/project','model',1,'first','last')`,
		`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('/first','codex','s','now','user','preserved searchable document')`,
		`INSERT INTO session_exclusions(kind,value) VALUES('project','/excluded')`,
		`DROP INDEX session_sources_identity`,
	} {
		if _, err = db.DB.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	readOnly, err := OpenSessionsReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	var indexes int
	if err = readOnly.DB.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name='session_sources_identity'`).Scan(&indexes); err != nil || indexes != 0 {
		t.Fatalf("read-only index migration: %d %v", indexes, err)
	}
	readOnly.Close()
	db, err = OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sources, documents, metadata, exclusions int
	for _, check := range []struct {
		query  string
		target *int
		want   int
	}{
		{`SELECT count(*) FROM session_sources WHERE identity='same-inode' AND cursor=7 AND scanned_at='original'`, &sources, 2},
		{`SELECT count(*) FROM session_documents WHERE session_documents MATCH 'searchable'`, &documents, 1},
		{`SELECT count(*) FROM session_metadata WHERE session_id='s' AND project='/project'`, &metadata, 1},
		{`SELECT count(*) FROM session_exclusions WHERE value='/excluded'`, &exclusions, 1},
	} {
		if err = db.DB.QueryRowContext(ctx, check.query).Scan(check.target); err != nil || *check.target != check.want {
			t.Fatalf("preservation: got %d want %d: %v", *check.target, check.want, err)
		}
	}
	rows, err := db.DB.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT source_path,cursor FROM session_sources WHERE identity=?`, "missing")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	indexed := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "SCAN session_sources") {
			t.Fatalf("identity lookup scans source inventory: %s", detail)
		}
		indexed = indexed || strings.Contains(detail, "USING INDEX")
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("identity lookup did not select an index")
	}
}
