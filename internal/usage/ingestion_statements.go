package usage

import (
	"context"
	"database/sql"
)

type ingestionExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const lookupUsageEventSQL = `SELECT e.source_path,CASE WHEN f.path IS NULL THEN 0 ELSE 1 END,e.event_at,e.model,COALESCE(e.turn_index,0),e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.cache_write_tokens FROM usage_events e LEFT JOIN usage_source_files f ON f.path=e.source_path WHERE e.event_key=?`
const upsertUsageEventSQL = `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,turn_index,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens,source_path,source_offset)VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(event_key) DO UPDATE SET event_at=excluded.event_at,model=excluded.model,turn_index=excluded.turn_index,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,cache_read_tokens=excluded.cache_read_tokens,cache_creation_tokens=excluded.cache_creation_tokens,cache_write_5m_tokens=excluded.cache_write_5m_tokens,cache_write_1h_tokens=excluded.cache_write_1h_tokens,cache_write_tokens=excluded.cache_write_tokens,source_path=excluded.source_path,source_offset=excluded.source_offset WHERE (usage_events.event_at,usage_events.model,usage_events.turn_index,usage_events.input_tokens,usage_events.cached_input_tokens,usage_events.output_tokens,usage_events.cache_read_tokens,usage_events.cache_creation_tokens,usage_events.cache_write_5m_tokens,usage_events.cache_write_1h_tokens,usage_events.cache_write_tokens,usage_events.source_path,usage_events.source_offset) IS NOT (excluded.event_at,excluded.model,excluded.turn_index,excluded.input_tokens,excluded.cached_input_tokens,excluded.output_tokens,excluded.cache_read_tokens,excluded.cache_creation_tokens,excluded.cache_write_5m_tokens,excluded.cache_write_1h_tokens,excluded.cache_write_tokens,excluded.source_path,excluded.source_offset)`
const lookupToolActivitySQL = `SELECT a.source_path,a.started_at,CASE WHEN f.path IS NULL THEN 0 ELSE 1 END FROM usage_tool_calls a LEFT JOIN usage_source_files f ON f.path=a.source_path WHERE a.activity_key=?`
const startToolActivitySQL = `INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,completed_at,status,duration_ms,source_path,source_offset,turn_index,tool_kind,mcp_server,command_read,command_hint) VALUES(?,?,?,?,?,?,NULL,'started',NULL,?,?,?,?,?,?,?) ON CONFLICT(activity_key) DO UPDATE SET client=excluded.client,session_id=excluded.session_id,model=excluded.model,tool_name=excluded.tool_name,started_at=excluded.started_at,completed_at=NULL,status='started',duration_ms=NULL,source_path=excluded.source_path,source_offset=excluded.source_offset,turn_index=excluded.turn_index,tool_kind=excluded.tool_kind,mcp_server=excluded.mcp_server,command_read=excluded.command_read,command_hint=excluded.command_hint`
const deleteToolFilesSQL = `DELETE FROM usage_tool_files WHERE activity_key=?`
const upsertToolFileSQL = `INSERT INTO usage_tool_files(activity_key,path_digest,base_name,wrote) VALUES(?,?,?,?) ON CONFLICT(activity_key,path_digest) DO UPDATE SET base_name=excluded.base_name,wrote=MAX(usage_tool_files.wrote,excluded.wrote)`
const completeToolActivitySQL = `UPDATE usage_tool_calls SET completed_at=?,status=?,duration_ms=?,source_path=? WHERE activity_key=?`

type ingestionStatements struct{ statements map[string]*sql.Stmt }

func prepareIngestion(ctx context.Context, tx *sql.Tx) (*ingestionStatements, error) {
	p := &ingestionStatements{statements: make(map[string]*sql.Stmt)}
	for _, q := range []string{lookupUsageEventSQL, upsertUsageEventSQL, lookupToolActivitySQL, startToolActivitySQL, deleteToolFilesSQL, upsertToolFileSQL, completeToolActivitySQL} {
		s, e := tx.PrepareContext(ctx, q)
		if e != nil {
			p.Close()
			return nil, e
		}
		p.statements[q] = s
	}
	return p, nil
}
func (p *ingestionStatements) Close() {
	for _, s := range p.statements {
		s.Close()
	}
}
func (p *ingestionStatements) ExecContext(ctx context.Context, q string, a ...any) (sql.Result, error) {
	return p.statements[q].ExecContext(ctx, a...)
}
func (p *ingestionStatements) QueryRowContext(ctx context.Context, q string, a ...any) *sql.Row {
	return p.statements[q].QueryRowContext(ctx, a...)
}
