package session

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
)

// Diff the full projection against the selected, compacted physical base.
// Excluded or protected projections cannot add metadata to a new candidate.
func (c *indexStageCandidate) planTelemetry(ctx context.Context, tx *sql.Tx, projections []TelemetrySession) error {
	wanted := make(map[string]TelemetrySession, len(projections))
	for _, p := range projections {
		path := TelemetrySourcePath(p.Client, p.SessionID)
		if p.Client == "" || p.SessionID == "" {
			return errIndexStage
		}
		if _, ok := wanted[path]; ok {
			return errIndexStage
		}
		skip, err := excluded(ctx, tx, Metadata{Client: p.Client, SessionID: p.SessionID, SourcePath: path})
		if err != nil {
			return err
		}
		var protected bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cjsi_session_protections WHERE source_path=? AND (client='' OR (client=? AND session_id=?)))`, path, p.Client, p.SessionID).Scan(&protected); err != nil {
			return err
		}
		if !skip && !protected {
			wanted[path] = p
		}
	}
	emptySequence, err := indexDigest([]int64(nil))
	if err != nil {
		return err
	}
	// Start at the source so that an orphan is removed or rebuilt. A legacy
	// projection can have identical public metadata but no verified group.
	rows, err := tx.QueryContext(ctx, `SELECT s.source_path,s.identity,
        coalesce(m.client,''),coalesce(m.session_id,''),coalesce(m.model,''),coalesce(m.first_at,''),coalesce(m.last_at,''),
        g.sequence_id, coalesce(
          s.identity=s.source_path AND s.priority=? AND s.parser_version=? AND
          s.cursor=0 AND s.size=0 AND s.modified_at=0 AND s.changed_at=0 AND s.prefix_hash='' AND
          s.partial_line=x'' AND s.parser_context='' AND s.scanned_at='' AND
          m.project='' AND m.parser_version=? AND g.source_kind='otel' AND g.order_verified=1 AND
          g.birth=m.first_at AND g.head_at='' AND g.sequence_id=? AND
          NOT EXISTS(SELECT 1 FROM cjsi_base_order o WHERE o.source_path=s.source_path) AND
          NOT EXISTS(SELECT 1 FROM cjsi_base_ranges r WHERE r.source_path=s.source_path),0)
        FROM session_sources s LEFT JOIN session_metadata m USING(source_path)
        LEFT JOIN cjsi_base_groups g USING(source_path,client,session_id)
        WHERE substr(s.source_path,1,5)='otel:' ORDER BY s.source_path,m.client,m.session_id`, telemetrySourcePriority, ParserVersion, ParserVersion, emptySequence[:])
	if err != nil {
		return err
	}
	defer rows.Close()
	current := map[string]bool{}
	extra := map[string]bool{}
	for rows.Next() {
		var path, identity string
		var old TelemetrySession
		var sequence []byte
		var canonical bool
		if err := rows.Scan(&path, &identity, &old.Client, &old.SessionID, &old.Model, &old.FirstAt, &old.LastAt, &sequence, &canonical); err != nil {
			return err
		}
		p, ok := wanted[path]
		switch {
		case !ok:
			c.DeletedSources = append(c.DeletedSources, indexSourceDelete{path, identity})
		case old.Client != "" && (old.Client != p.Client || old.SessionID != p.SessionID):
			g := indexGroupDelete{Path: path, Client: old.Client, SessionID: old.SessionID}
			if len(sequence) != len(g.Sequence) {
				return errIndexStage
			}
			copy(g.Sequence[:], sequence)
			c.DeletedGroups = append(c.DeletedGroups, g)
			extra[path] = true
		case canonical && old == p:
			current[path] = true
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for path := range current {
		if !extra[path] {
			delete(wanted, path)
		}
	}
	for _, p := range wanted {
		c.Telemetry = append(c.Telemetry, p)
	}
	slices.SortFunc(c.Telemetry, func(a, b TelemetrySession) int {
		if n := strings.Compare(a.Client, b.Client); n != 0 {
			return n
		}
		return strings.Compare(a.SessionID, b.SessionID)
	})
	return nil
}

func stageCJSITelemetry(ctx context.Context, tx *sql.Tx, a indexStageAttempt, p TelemetrySession) error {
	path := TelemetrySourcePath(p.Client, p.SessionID)
	skip, err := excluded(ctx, tx, Metadata{Client: p.Client, SessionID: p.SessionID, SourcePath: path})
	if err != nil {
		return err
	}
	if skip {
		return errIndexStage
	}
	err = stageOne(ctx, tx, `INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at)
        SELECT ?,?,?,'upsert','otel',?,?,?,0,0,0,0,'','' WHERE NOT EXISTS(SELECT 1 FROM cjsi_session_protections WHERE source_path=? AND (client='' OR (client=? AND session_id=?)))`, a.Ticket, a.Attempt, path, path, telemetrySourcePriority, ParserVersion, path, p.Client, p.SessionID)
	if err != nil {
		return err
	}
	seq, err := indexDigest([]int64(nil))
	if err != nil {
		return err
	}
	return stageOne(ctx, tx, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,order_verified,sequence_id,birth) VALUES(?,?,?,?,?,'replace','',?,?,?,?,1,?,?)`, a.Ticket, a.Attempt, path, p.Client, p.SessionID, p.Model, ParserVersion, p.FirstAt, p.LastAt, seq[:], p.FirstAt)
}
