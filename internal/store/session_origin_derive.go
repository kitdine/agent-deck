package store

import (
	"context"
	"database/sql"
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
	rows, err := s.DB.QueryContext(ctx, `SELECT m.client, m.session_id, s.identity, s.prefix_hash, m.parser_version, m.first_at
FROM session_metadata m JOIN session_sources s ON s.source_path = m.source_path
ORDER BY m.client, m.session_id, s.identity, s.prefix_hash, m.parser_version, m.first_at`)
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

// SessionOriginSourcesFor lists the indexed copies of one canonical session.
func (s *Store) SessionOriginSourcesFor(ctx context.Context, client, sessionID string) ([]SessionOriginSource, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT m.client, m.session_id, s.identity, s.prefix_hash, m.parser_version, m.first_at
FROM session_metadata m JOIN session_sources s ON s.source_path = m.source_path
WHERE m.client = ? AND m.session_id = ?
ORDER BY s.identity, s.prefix_hash, m.parser_version, m.first_at`, client, sessionID)
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
	if err := rows.Close(); err != nil {
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
	Epoch     int64
	Revision  int64
	Origins   []SessionOrigin
	Overrides []OriginOverride
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
	// The first statement fixes the WAL snapshot every later read sees.
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT d.epoch,g.revision,m.version FROM derived_snapshot_generation d JOIN serving_generation g ON g.singleton=d.singleton JOIN schema_metadata m ON m.singleton=d.singleton WHERE d.singleton=1`).Scan(&state.Epoch, &state.Revision, &version); err != nil {
		return state, err
	}
	if version > CurrentSchemaVersion {
		return state, &SchemaAhead{Stored: version, Supported: CurrentSchemaVersion}
	}
	if version < OriginSchemaVersion {
		return state, tx.Commit()
	}
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision FROM session_origins ORDER BY client,session_id`)
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
	if err := rows.Close(); err != nil {
		return state, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides ORDER BY client,session_id`)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		override, err := scanOriginOverride(rows)
		if err != nil {
			rows.Close()
			return state, err
		}
		state.Overrides = append(state.Overrides, override)
	}
	if err := rows.Close(); err != nil {
		return state, err
	}
	return state, tx.Commit()
}
