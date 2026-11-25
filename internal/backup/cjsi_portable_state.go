package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
)

func rejectPrivateSessionSchema(ctx context.Context, db *sql.DB) error {
	var private bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE lower(name) GLOB 'cjsi_*')`).Scan(&private); err != nil || private {
		return fmt.Errorf("%w: unsupported portable session layout", ErrInvalidArchive)
	}
	return nil
}

func sanitizeRestoredSession(ctx context.Context, contents []byte) ([]byte, error) {
	staging, err := os.MkdirTemp("", "agentdeck-restore-session-sanitize-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	snapshot := filepath.Join(staging, sessionsName)
	if err := os.WriteFile(snapshot, contents, platform.FileMode); err != nil {
		return nil, err
	}
	clean := filepath.Join(staging, "sessions.compact.sqlite3")
	if err := preparePortableSession(ctx, snapshot, clean); err != nil {
		return nil, err
	}
	return os.ReadFile(clean)
}

// preparePortableSession works on a disposable legacy snapshot. The new FTS
// is built from allowed rows before VACUUM removes the old table's pages.
func preparePortableSession(ctx context.Context, source, destination string) error {
	uri := url.URL{Scheme: "file", Path: source, RawQuery: "mode=rw&_pragma=secure_delete(1)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := rejectPrivateSessionSchema(ctx, db); err != nil {
		return err
	}
	if err := scrubLegacySession(ctx, db); err != nil {
		return fmt.Errorf("%w: session snapshot could not be cleaned", ErrInvalidArchive)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("%w: session snapshot could not be compacted", ErrInvalidArchive)
	}
	return nil
}

func scrubLegacySession(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exclusions, tail, parserContext, eventAt, serving bool
	if err := tx.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM sqlite_schema WHERE name='session_exclusions' AND type='table'),
		EXISTS(SELECT 1 FROM pragma_table_info('session_sources') WHERE name='partial_line'),
		EXISTS(SELECT 1 FROM pragma_table_info('session_sources') WHERE name='parser_context'),
		EXISTS(SELECT 1 FROM pragma_table_info('session_documents') WHERE name='event_at'),
		EXISTS(SELECT 1 FROM sqlite_schema WHERE name='serving_generation' AND type='table')`).
		Scan(&exclusions, &tail, &parserContext, &eventAt, &serving); err != nil {
		return err
	}
	if serving {
		if err := session.MarkSearchTextChanged(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE VIRTUAL TABLE portable_documents USING fts5(
		source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, event_at UNINDEXED, kind UNINDEXED, text)`); err != nil {
		return err
	}
	eventTime := "''"
	if eventAt {
		eventTime = "d.event_at"
	}
	copyDocuments := `INSERT INTO portable_documents(rowid,source_path,client,session_id,event_at,kind,text)
		SELECT d.rowid,d.source_path,d.client,d.session_id,` + eventTime + `,d.kind,d.text FROM session_documents d`
	if exclusions {
		copyDocuments += ` WHERE NOT EXISTS (
			SELECT 1 FROM session_exclusions e WHERE (e.kind='path' AND e.value=d.source_path)
			OR (e.kind='client' AND e.value=d.client) OR (e.kind='session' AND e.value=d.session_id)
			OR (e.kind='project' AND EXISTS(SELECT 1 FROM session_metadata m WHERE m.project=e.value
				AND (m.source_path,m.client,m.session_id)=(d.source_path,d.client,d.session_id))))`
	}
	if _, err := tx.ExecContext(ctx, copyDocuments); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE session_documents; ALTER TABLE portable_documents RENAME TO session_documents`); err != nil {
		return err
	}
	if exclusions {
		if _, err := tx.ExecContext(ctx, `DELETE FROM session_metadata AS m WHERE EXISTS (
			SELECT 1 FROM session_exclusions e WHERE (e.kind='path' AND e.value=m.source_path)
			OR (e.kind='client' AND e.value=m.client) OR (e.kind='project' AND e.value=m.project)
			OR (e.kind='session' AND e.value=m.session_id))`); err != nil {
			return err
		}
	}
	if tail {
		if _, err := tx.ExecContext(ctx, `UPDATE session_sources SET cursor=max(0,cursor-length(partial_line)),
			partial_line=X'',parser_version=0 WHERE length(partial_line)>0`); err != nil {
			return err
		}
	}
	if parserContext {
		if _, err := tx.ExecContext(ctx, `UPDATE session_sources SET parser_context='',parser_version=0
			WHERE parser_context<>''`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
