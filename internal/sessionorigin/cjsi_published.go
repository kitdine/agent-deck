package sessionorigin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// PublishedOriginReader borrows the caller's transaction, already admitted
// for this root. Neither construction nor a lookup opens a publication.
type PublishedOriginReader struct {
	session.CJSIReader
	root string
	core *sql.Tx
}

func NewPublishedOriginReader(root string, core *sql.Tx) PublishedOriginReader {
	return PublishedOriginReader{session.NewCJSIReader(root, core, ingest.CodexSelectedLimits{}), root, core}
}

func (r PublishedOriginReader) SettledBirth(ctx context.Context, target Target) (string, error) {
	var at, settled, kind string
	err := r.core.QueryRowContext(ctx, `SELECT first_at,settled_at,source_kind FROM main.telemetry_sessions WHERE client=? AND session_id=?`, target.Client, target.SessionID).Scan(&at, &settled, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if kind != store.TelemetrySessionSourceKind || instant(settled) == "" {
		return "", ErrClassificationUnavailable
	}
	return instant(at), nil
}

// SessionOriginSourcesFor implements ResolveBinding's input using only
// eligible copies and the birth tier shared with classification/selection.
func (r PublishedOriginReader) SessionOriginSourcesFor(ctx context.Context, client, id string) ([]store.SessionOriginSource, error) {
	item, err := readEffectiveTarget(ctx, r, Target{client, id})
	if err != nil || item == nil {
		return nil, err
	}
	return []store.SessionOriginSource{{Client: client, SessionID: id, FirstAt: item.birth.StartedAt}}, nil
}

func (r PublishedOriginReader) resolveBinding(ctx context.Context, client, id string) (*store.OriginBinding, error) {
	if err := validTarget(client, id); err != nil {
		return nil, err
	}
	var b *store.OriginBinding
	err := store.WithCJSIIndexSnapshot(ctx, r.root, r.core, func(s store.CJSIIndexSnapshot) error {
		return visitOriginCopies(ctx, s, client, id, func(key Target, copies []session.EffectiveSessionCopy) error {
			item, err := targetFromCopies(ctx, r, key, copies)
			if err != nil {
				return err
			}
			b = stampedBinding(key, item.birth, s.Selection())
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Select pairs core classification with Task 5's effective enumeration and
// MATCH results. The outer admission revokes the entire result (including an
// empty enumeration) if protection moves during assembly.
func (r PublishedOriginReader) Select(ctx context.Context, selection Selection) (Snapshot, error) {
	return r.selectWhere(ctx, selection, "")
}

// Lookup verifies just the requested ID. A stale OTel projection in an
// unrelated session is not evidence against this target's binding.
func (r PublishedOriginReader) Lookup(ctx context.Context, client, id string) (Snapshot, error) {
	if id == "" {
		return Snapshot{}, ErrInvalidArgument
	}
	return r.selectWhere(ctx, Selection{Scope: ScopeAll, Client: client}, id)
}

func (r PublishedOriginReader) selectWhere(ctx context.Context, selection Selection, id string) (Snapshot, error) {
	if err := selection.Validate(); err != nil {
		return Snapshot{}, err
	}
	var out Snapshot
	err := store.WithCJSIIndexSnapshot(ctx, r.root, r.core, func(s store.CJSIIndexSnapshot) error {
		state, err := store.ReadPublishedOriginTargetTx(ctx, r.core, selection.Client, id)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrClassificationUnavailable, err)
		}
		overrides := overridesByTarget(state.Overrides)
		stamp := s.Selection()
		index := session.IndexSnapshot{Epoch: stamp.IndexEpoch, Revision: stamp.SessionRevision}
		indexEmpty := true
		err = visitOriginCopies(ctx, s, selection.Client, id, func(key Target, copies []session.EffectiveSessionCopy) error {
			indexEmpty = false
			if selection.Client != "" && key.Client != selection.Client {
				return nil
			}
			item, _, err := r.classificationTarget(ctx, key, copies, overrides[key])
			if err != nil {
				return err
			}
			index.Sessions = append(index.Sessions, item.indexed)
			return nil
		})
		if err != nil {
			return err
		}
		// Lookup still reports whether the whole index is empty on a miss.
		if id != "" && indexEmpty {
			if err := s.Tx.QueryRowContext(ctx, "SELECT NOT EXISTS(SELECT 1 FROM ("+s.CopiesSQL()+"))").Scan(&indexEmpty); err != nil {
				return err
			}
		}
		// Groups arrive in key order; preserve that order for tied display times.
		sort.SliceStable(index.Sessions, func(i, j int) bool { return index.Sessions[i].LastAt > index.Sessions[j].LastAt })
		if selection.Query != "" {
			index.Documents, err = r.Search(ctx, selection.Query)
			if err != nil {
				return err
			}
		}
		snap := Snapshot{Selection: selection, Vector: Vector{CoreEpoch: state.Epoch, UsageRevision: state.Revision, SessionEpoch: stamp.IndexEpoch, SessionRevision: stamp.SessionRevision}, published: &stamp}
		out = assemble(snap, index, state)
		out.indexEmpty = indexEmpty
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrClassificationUnavailable) {
			return Snapshot{}, err
		}
		return Snapshot{}, fmt.Errorf("%w: %w", ErrSessionsUnavailable, err)
	}
	return out, nil
}

// Consume only the eligible relation supplied by the store. Its protection,
// candidate and compaction rules remain inside the admitted snapshot.
func visitOriginCopies(ctx context.Context, s store.CJSIIndexSnapshot, client, id string, visit func(Target, []session.EffectiveSessionCopy) error) error {
	filter := ""
	var args []any
	if id != "" {
		filter += " WHERE session_id=?"
		args = append(args, id)
		if client != "" {
			filter += " AND client=?"
			args = append(args, client)
		}
	}
	return visitFilteredOriginCopies(ctx, s, filter, args, visit)
}

func visitFilteredOriginCopies(ctx context.Context, s store.CJSIIndexSnapshot, filter string, args []any, visit func(Target, []session.EffectiveSessionCopy) error) error {
	query := `SELECT client,session_id,project,source_path,model,first_at,last_at,identity,prefix_hash,parser_version,head_at,birth FROM (` + s.CopiesSQL() + `)` + filter
	rows, err := s.Tx.QueryContext(ctx, query+" ORDER BY client,session_id,priority DESC,source_path", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var group []session.EffectiveSessionCopy
	var key Target
	for rows.Next() {
		var c session.EffectiveSessionCopy
		m := &c.Metadata
		if err := rows.Scan(&m.Client, &m.SessionID, &m.Project, &m.SourcePath, &m.Model, &m.FirstAt, &m.LastAt, &c.Source.Identity, &c.Source.PrefixHash, &c.Source.ParserVersion, &c.HeadAt, &c.Birth); err != nil {
			return err
		}
		target := Target{m.Client, m.SessionID}
		if target != key && len(group) > 0 {
			if err := visit(key, group); err != nil {
				return err
			}
			group = group[:0]
		}
		c.Source.FirstAt = m.FirstAt
		c.Selected = len(group) == 0
		key = target
		group = append(group, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(group) > 0 {
		return visit(key, group)
	}
	return nil
}

func (r PublishedOriginReader) classificationTarget(ctx context.Context, key Target, copies []session.EffectiveSessionCopy, o store.OriginOverride) (*effectiveTarget, string, error) {
	if err := validTarget(key.Client, key.SessionID); err != nil {
		return nil, "", err
	}
	item, err := targetFromCopies(ctx, r, key, copies)
	if err != nil {
		return nil, "", err
	}
	status, err := effectiveOverrideStatus(key, item.birth, o)
	return item, status, err
}

// ListOverrides lists visible bound targets and retained unindexed user choices.
// The core read supplies the order; protected keys never enter the page or count.
func (r PublishedOriginReader) ListOverrides(ctx context.Context, page, limit int) (OverridePage, error) {
	if limit == 0 {
		limit = DefaultOverrideLimit
	}
	if page < 1 || limit < 1 || limit > MaxOverrideLimit {
		return OverridePage{}, ErrInvalidArgument
	}
	if r.core == nil {
		return OverridePage{}, &Error{Code: CodeReadFailed, Err: ErrGenerationUnavailable}
	}
	rows, err := store.ReadActiveOriginOverridesTx(ctx, r.core)
	if err != nil {
		return OverridePage{}, &Error{Code: CodeReadFailed, Err: err}
	}
	out := OverridePage{Page: page, Limit: limit, Items: []Override{}}
	add := func(o store.OriginOverride, status string) {
		if out.Total/limit == page-1 {
			out.Items = append(out.Items, Override{Client: o.Client, SessionID: o.SessionID, Value: *o.Value, BindingStatus: status, BindingLimited: o.BindingLimited, Revision: o.Revision})
		}
		out.Total++
	}
	err = store.WithCJSIIndexSnapshot(ctx, r.root, r.core, func(s store.CJSIIndexSnapshot) error {
		const batchSize = 200
		hasUnindexed := false
		for batch := range slices.Chunk(rows, batchSize) {
			states, unindexed, err := r.listedOverrideStates(ctx, s, batch)
			if err != nil {
				return err
			}
			hasUnindexed = hasUnindexed || unindexed
			for _, o := range batch {
				if status, ok := states[Target{o.Client, o.SessionID}]; ok {
					add(o, status)
				}
			}
		}
		if hasUnindexed {
			return r.checkUnindexedOriginMask(ctx)
		}
		return nil
	})
	if errors.Is(err, store.ErrCJSIIndexAbsent) {
		if len(rows) > 0 {
			if err := r.checkUnindexedOriginMask(ctx); err != nil {
				return OverridePage{}, &Error{Code: CodeReadFailed, Err: err}
			}
		}
		for _, o := range rows {
			add(o, BindingUnmatched)
		}
		return out, nil
	}
	if err != nil {
		return OverridePage{}, &Error{Code: CodeReadFailed, Err: err}
	}
	return out, nil
}

// Return only states that can appear on the public page. Missing keys must
// be classified in the same private snapshot before any result is returned.
func (r PublishedOriginReader) listedOverrideStates(ctx context.Context, s store.CJSIIndexSnapshot, batch []store.OriginOverride) (map[Target]string, bool, error) {
	overrides := overridesByTarget(batch)
	states := make(map[Target]string, len(batch))
	args := make([]any, 0, 2*len(batch))
	for _, o := range batch {
		args = append(args, o.Client, o.SessionID)
	}
	filter := " WHERE (client,session_id) IN (VALUES " + strings.Repeat("(?,?),", len(batch)-1) + "(?,?))"
	err := visitFilteredOriginCopies(ctx, s, filter, args, func(key Target, copies []session.EffectiveSessionCopy) error {
		_, status, err := r.classificationTarget(ctx, key, copies, overrides[key])
		if err != nil {
			return err
		}
		states[key] = status
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	var missing []store.OriginOverride
	for _, o := range batch {
		if _, ok := states[Target{o.Client, o.SessionID}]; !ok {
			missing = append(missing, o)
		}
	}
	unindexed, err := s.UnindexedOriginOverrides(ctx, missing)
	if err != nil {
		return nil, false, err
	}
	for _, o := range unindexed {
		if o.Client == "codex" && o.BindingState == BindingMatched {
			return nil, false, ErrClassificationUnavailable
		}
		states[Target{o.Client, o.SessionID}] = BindingUnmatched
	}
	return states, len(unindexed) > 0, nil
}

// Without copy evidence, an origin/session mask cannot be mapped to a safe
// override key. The caller must discard the whole provisional page.
func (r PublishedOriginReader) checkUnindexedOriginMask(ctx context.Context) error {
	var masked bool
	if err := r.core.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.cjsi_scope_overlays WHERE domain IN ('session','origin') AND masked=1)`).Scan(&masked); err != nil {
		return err
	}
	if masked {
		return ErrClassificationUnavailable
	}
	return nil
}
