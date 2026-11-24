package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// SessionOriginSource is one indexed copy of a canonical session as the
// origin reducer sees it: the source's stable identity and generation, never
// its path or content.
type SessionOriginSource struct {
	Client        string
	SessionID     string
	Identity      string
	PrefixHash    string
	ParserVersion int64
	FirstAt       string
}

// SessionOriginSources lists every indexed copy of every canonical session,
// ordered by client, session ID and source identity. It reads the session
// index only; it opens no source and reads no project or document column.
func (s *Store) SessionOriginSources(ctx context.Context) ([]SessionOriginSource, error) {
	return s.legacyOriginSources(ctx, "", "")
}

// SessionOriginSourcesFor lists the indexed copies of one canonical session.
func (s *Store) SessionOriginSourcesFor(ctx context.Context, client, sessionID string) ([]SessionOriginSource, error) {
	if client == "" || sessionID == "" {
		return nil, nil
	}
	return s.legacyOriginSources(ctx, client, sessionID)
}

// Only an old index without private candidates can answer without a core
// ticket. Keep this check inside the same transaction as the legacy query.
func (s *Store) legacyOriginSources(ctx context.Context, client, id string) ([]SessionOriginSource, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var private bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.sqlite_schema WHERE name='cjsi_index_state')`).Scan(&private); err != nil {
		return nil, err
	}
	if private {
		return nil, ErrCJSIIndexChanged
	}
	query := `SELECT m.client,m.session_id,s.identity,s.prefix_hash,m.parser_version,m.first_at
FROM session_metadata m JOIN session_sources s USING(source_path)`
	var args []any
	if client != "" {
		query += ` WHERE m.client=? AND m.session_id=?`
		args = []any{client, id}
	}
	rows, err := tx.QueryContext(ctx, query+` ORDER BY m.client,m.session_id,s.identity,s.prefix_hash,m.parser_version,m.first_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionOriginSource
	for rows.Next() {
		var item SessionOriginSource
		if err := rows.Scan(&item.Client, &item.SessionID, &item.Identity, &item.PrefixHash, &item.ParserVersion, &item.FirstAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// OriginDerivedState is the stored derived classification state.
type OriginDerivedState struct {
	Origins  []SessionOrigin
	Evidence []OriginEvidence
}

// OriginDerivedState reads every stored classification and evidence row in
// one read transaction, so the reducer diffs against one consistent state.
func (s *Store) OriginDerivedState(ctx context.Context) (OriginDerivedState, error) {
	var state OriginDerivedState
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return state, err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return state, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision FROM session_origins ORDER BY client, session_id`)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var item SessionOrigin
		if err := rows.Scan(&item.Client, &item.SessionID, &item.Automatic, &item.ReasonCode, &item.RuleVersion, &item.InputToken, &item.EvidenceRevision); err != nil {
			rows.Close()
			return state, err
		}
		state.Origins = append(state.Origins, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return state, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at FROM origin_evidence ORDER BY adapter, client, session_id, source_token`)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OriginEvidence
		if err := rows.Scan(&item.Adapter, &item.RuleVersion, &item.Client, &item.SessionID, &item.SourceToken, &item.Purpose, &item.ValidatedVersion, &item.ObservedAt); err != nil {
			return state, err
		}
		state.Evidence = append(state.Evidence, item)
	}
	return state, rows.Err()
}

// OriginSelectionState is the core input of a session-scope selection, read
// in one read transaction with the core half of the serving vector.
//
// Its read dependencies are derived_snapshot_generation (epoch),
// serving_generation, schema_metadata, session_origins and origin_overrides.
// session_origins and origin_overrides advance serving_generation.revision on
// every INSERT/UPDATE/DELETE (migration 34); an epoch change is part of the
// vector itself.
type OriginSelectionState struct {
	SchemaVersion int
	Epoch         int64
	Revision      int64
	Origins       []SessionOrigin
	Overrides     []OriginOverride
}

// ReadOriginSelectionState reads the selection input. A core that predates
// origin storage is a successful read with no origin facts.
func (s *Store) ReadOriginSelectionState(ctx context.Context) (OriginSelectionState, error) {
	var state OriginSelectionState
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return state, err
	}
	defer tx.Rollback()
	state, err = readOriginSelectionTx(ctx, tx, CurrentSchemaVersion, "", "")
	if err != nil {
		return OriginSelectionState{}, err
	}
	return state, tx.Commit()
}

// ReadPublishedOriginSelectionTx borrows the snapshot admitted by the CJSI
// index reader. It must not reopen core at the current/latest generation.
func ReadPublishedOriginSelectionTx(ctx context.Context, tx *sql.Tx) (OriginSelectionState, error) {
	return ReadPublishedOriginTargetTx(ctx, tx, "", "")
}

// ReadPublishedOriginTargetTx restricts both origin tables before decoding
// their contents. An omitted client keeps both sides of an ambiguous ID.
func ReadPublishedOriginTargetTx(ctx context.Context, tx *sql.Tx, client, id string) (OriginSelectionState, error) {
	return readOriginSelectionTx(ctx, tx, cjsiLayoutCoreSchemaVersion, client, id)
}

func readOriginSelectionTx(ctx context.Context, tx *sql.Tx, ceiling int, client, id string) (OriginSelectionState, error) {
	var state OriginSelectionState
	// The first statement fixes the WAL snapshot every later read sees.
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT d.epoch,g.revision,m.version FROM derived_snapshot_generation d JOIN serving_generation g ON g.singleton=d.singleton JOIN schema_metadata m ON m.singleton=d.singleton WHERE d.singleton=1`).Scan(&state.Epoch, &state.Revision, &version); err != nil {
		return state, err
	}
	state.SchemaVersion = version
	if version > ceiling {
		return state, &SchemaAhead{Stored: version, Supported: ceiling}
	}
	if version < OriginSchemaVersion {
		return state, nil
	}
	where, args := originTargetFilter(client, id)
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision FROM session_origins`+where+` ORDER BY client,session_id`, args...)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var item SessionOrigin
		if err := rows.Scan(&item.Client, &item.SessionID, &item.Automatic, &item.ReasonCode, &item.RuleVersion, &item.InputToken, &item.EvidenceRevision); err != nil {
			rows.Close()
			return state, err
		}
		state.Origins = append(state.Origins, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return state, err
	}
	state.Overrides, err = readOriginOverridesTx(ctx, tx, where, args...)
	return state, err
}

// ReadActiveOriginOverridesTx includes unmatched rows and omits tombstones.
// The managed list reader applies visibility before counting or paginating.
func ReadActiveOriginOverridesTx(ctx context.Context, tx *sql.Tx) ([]OriginOverride, error) {
	return readOriginOverridesTx(ctx, tx, " WHERE value IS NOT NULL")
}

func readOriginOverridesTx(ctx context.Context, tx *sql.Tx, where string, args ...any) ([]OriginOverride, error) {
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides`+where+` ORDER BY client,session_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []OriginOverride
	for rows.Next() {
		o, err := scanOriginOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func originTargetFilter(client, id string) (string, []any) {
	if id == "" {
		return "", nil
	}
	if client == "" {
		return " WHERE session_id=?", []any{id}
	}
	return " WHERE session_id=? AND client=?", []any{id, client}
}

// UnindexedOriginOverrides checks missing copy keys in a batch from the
// classify list. A selected but filtered group, a direct exclusion, or a
// retained protection is never evidence of an ordinary unindexed choice.
// Inspect only membership on this capsule; no project, document or birth
// is read. As with CopiesSQL, a selected source replacement retires base
// groups of the old identity or parser even without explicit group edits.
func (s CJSIIndexSnapshot) UnindexedOriginOverrides(ctx context.Context, missing []OriginOverride) ([]OriginOverride, error) {
	if len(missing) == 0 {
		return nil, nil
	}
	args := make([]any, 0, 3*len(missing))
	for i, o := range missing {
		args = append(args, i, o.Client, o.SessionID)
	}
	values := strings.Repeat("(?,?,?),", len(missing)-1) + "(?,?,?)"
	side := s.side()
	query := `WITH wanted(n,client,session_id) AS (VALUES ` + values + `),
 cs AS (SELECT source_path,op,identity,parser_version FROM main.cjsi_candidate_sources WHERE ` + side + `),
 cg AS (SELECT source_path,client,session_id,op FROM main.cjsi_candidate_groups WHERE ` + side + `
   AND (client,session_id) IN (SELECT client,session_id FROM wanted)),
 hidden(n) AS (
   SELECT w.n FROM wanted w
   JOIN main.cjsi_base_groups b USING(client,session_id)
   JOIN main.session_metadata m USING(source_path,client,session_id)
   JOIN main.session_sources old USING(source_path)
   LEFT JOIN cs ON cs.source_path=b.source_path
   WHERE (cs.source_path IS NULL OR
     (cs.op='upsert' AND cs.identity IS old.identity AND cs.parser_version IS old.parser_version))
     AND NOT EXISTS(SELECT 1 FROM cg WHERE (cg.source_path,cg.client,cg.session_id)=(b.source_path,b.client,b.session_id))
   UNION ALL
   SELECT w.n FROM wanted w JOIN cg USING(client,session_id)
   LEFT JOIN cs ON cs.source_path=cg.source_path
   WHERE cg.op<>'delete' AND (cs.op='upsert' OR (cs.source_path IS NULL
     AND EXISTS(SELECT 1 FROM main.session_sources old WHERE old.source_path=cg.source_path)))
   UNION ALL
   SELECT w.n FROM wanted w JOIN main.cjsi_session_protections p USING(client,session_id)
   UNION ALL
   SELECT w.n FROM wanted w WHERE EXISTS(SELECT 1 FROM main.session_exclusions e
     WHERE (e.kind='session' AND e.value=w.session_id) OR (e.kind='client' AND e.value=w.client))
 )
 SELECT n FROM wanted EXCEPT SELECT n FROM hidden`
	rows, err := s.Tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var unindexed []OriginOverride
	for rows.Next() {
		var i int
		if err := rows.Scan(&i); err != nil {
			return nil, err
		}
		unindexed = append(unindexed, missing[i])
	}
	return unindexed, rows.Err()
}
