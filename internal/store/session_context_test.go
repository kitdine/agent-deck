package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionParserContextUpgradePreservesIndexAndCore(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	core, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.SetSetting(ctx, "fixture.preserved", "user/usage state"); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	corePath := filepath.Join(root, "agentdeck.sqlite3")
	before, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	index, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-v6 cache schema, retaining actual indexed user content.
	for _, statement := range []string{
		`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES('/fixture','fixture',1,1,1,'hash',0,5,'time')`,
		`INSERT INTO session_metadata(source_path,client,session_id,project,parser_version,first_at,last_at) VALUES('/fixture','codex','old','/project',5,'first','last')`,
		`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('/fixture','codex','old','time','user_prompt','preserved text')`,
		`INSERT INTO session_exclusions(kind,value) VALUES('project','/excluded')`,
		`ALTER TABLE session_sources DROP COLUMN parser_context`,
	} {
		if _, err := index.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	index, err = OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	var parserContext, project, text, exclusion string
	if err := index.DB.QueryRowContext(ctx, `SELECT parser_context FROM session_sources WHERE source_path='/fixture'`).Scan(&parserContext); err != nil || parserContext != "" {
		t.Fatalf("context=%q %v", parserContext, err)
	}
	if err := index.DB.QueryRowContext(ctx, `SELECT project FROM session_metadata WHERE session_id='old'`).Scan(&project); err != nil || project != "/project" {
		t.Fatalf("project=%q %v", project, err)
	}
	if err := index.DB.QueryRowContext(ctx, `SELECT text FROM session_documents WHERE session_id='old'`).Scan(&text); err != nil || text != "preserved text" {
		t.Fatalf("text=%q %v", text, err)
	}
	if err := index.DB.QueryRowContext(ctx, `SELECT value FROM session_exclusions`).Scan(&exclusion); err != nil || exclusion != "/excluded" {
		t.Fatalf("exclusion=%q %v", exclusion, err)
	}
	after, err := os.ReadFile(corePath)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("session migration changed core database")
	}
}
