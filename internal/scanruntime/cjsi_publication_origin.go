package scanruntime

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

// CJSIOriginChange supplies the proposed marker and continuity evidence for
// one affected key. Stored overrides, classifications, and evidence are read
// from the same checked core snapshot as the old and proposed index views.
type CJSIOriginChange struct {
	Input                 sessionorigin.OriginChangeInputs
	ProposedMarkerFirstAt string
}
type publicationOriginView struct {
	index store.CJSIIndexSnapshot

	birth func(context.Context, sessionorigin.Target) (string, error)
}

func (v publicationOriginView) EffectiveCopies(ctx context.Context, client, id string) ([]session.EffectiveSessionCopy, error) {
	rows, err := v.index.Tx.QueryContext(ctx, "SELECT client,session_id,project,source_path,model,first_at,last_at,identity,prefix_hash,parser_version,head_at,birth FROM ("+v.index.CopiesSQL()+") WHERE client=? AND session_id=? ORDER BY priority DESC,source_path", client, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var copies []session.EffectiveSessionCopy
	for rows.Next() {
		var c session.EffectiveSessionCopy
		m := &c.Metadata
		if err := rows.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt, &c.Source.Identity, &c.Source.PrefixHash, &c.Source.ParserVersion, &c.HeadAt, &c.Birth); err != nil {
			return nil, err
		}
		c.Source.FirstAt = m.FirstAt
		c.Selected = len(copies) == 0
		copies = append(copies, c)
	}
	return copies, rows.Err()
}
func (v publicationOriginView) SettledBirth(ctx context.Context, target sessionorigin.Target) (string, error) {
	return v.birth(ctx, target)
}

func PreparePublicationOrigins(ctx context.Context, core *sql.Tx, old, next store.CJSIIndexSnapshot, at time.Time, changes []CJSIOriginChange) ([]sessionorigin.PreparedOrigin, error) {
	if core == nil || old.Tx == nil || old.Tx != next.Tx || changes == nil {
		return nil, sessionorigin.ErrClassificationUnavailable
	}
	previous := publicationOriginView{old, sessionorigin.NewPublishedOriginReader("", core).SettledBirth}
	result := make([]sessionorigin.PreparedOrigin, 0, len(changes))
	seen := make(map[sessionorigin.Target]bool)
	for _, change := range changes {
		input := change.Input
		key := input.Target
		if key.Client != "codex" || key.SessionID == "" || seen[key] {
			return nil, sessionorigin.ErrClassificationUnavailable
		}
		seen[key] = true
		state, err := store.ReadPublishedOriginTargetTx(ctx, core, key.Client, key.SessionID)
		if err != nil {
			return nil, err
		}
		if len(state.Origins) > 1 || len(state.Overrides) > 1 {
			return nil, sessionorigin.ErrClassificationUnavailable
		}
		input.Origin, input.Override = nil, nil
		if len(state.Origins) == 1 {
			input.Origin = &state.Origins[0]
		}
		if len(state.Overrides) == 1 {
			input.Override = &state.Overrides[0]
		}
		rows, err := core.QueryContext(ctx, `SELECT adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at FROM origin_evidence WHERE client=? AND session_id=? AND adapter=? ORDER BY source_token`, key.Client, key.SessionID, sessionorigin.SourceCodexOTelBackground)
		if err != nil {
			return nil, err
		}
		input.Evidence = nil
		for rows.Next() {
			var e store.OriginEvidence
			if err = rows.Scan(&e.Adapter, &e.RuleVersion, &e.Client, &e.SessionID, &e.SourceToken, &e.Purpose, &e.ValidatedVersion, &e.ObservedAt); err != nil {
				rows.Close()
				return nil, err
			}
			input.Evidence = append(input.Evidence, e)
		}
		if err = errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
		input.Previous = previous
		proposed := publicationOriginView{next, func(_ context.Context, target sessionorigin.Target) (string, error) {
			if target != key {
				return "", sessionorigin.ErrClassificationUnavailable
			}
			return change.ProposedMarkerFirstAt, nil
		}}
		op, err := sessionorigin.PrepareEffectiveOrigin(ctx, key.SessionID, at, func(read func(sessionorigin.EffectiveCopyReader, sessionorigin.OriginChangeInputs) error) error {
			return read(proposed, input)
		})
		if err != nil {
			return nil, err
		}
		result = append(result, op)
	}
	return result, nil
}
