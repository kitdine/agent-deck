package session

import (
	"context"
	"database/sql"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// The public scan counters count changes to the winning document sequence.
// A tombstone can reveal a previously losing replica with a different order.
type proposedCopy struct {
	path        string
	priority    int
	replacement bool
	docs        []visibleDocument
}

func (c *indexStageCandidate) affectedChanges(ctx context.Context, snapshot store.CJSIIndexSnapshot) (changed, removed int, err error) {
	v := privateIndexView(snapshot, ingest.CodexSelectedLimits{})
	keys := map[affectedSession]bool{}
	touched := map[string]bool{}
	proposed := map[affectedSession][]proposedCopy{}
	for _, d := range c.DeletedSources {
		touched[d.Path] = true
	}
	for _, r := range c.Replacements {
		touched[r.Metadata.SourcePath] = true
		k := affectedSession{r.Metadata.Client, r.Metadata.SessionID}
		keys[k] = true
		p := proposedCopy{path: r.Selected.Path, priority: r.Metadata.projectPriority, replacement: true}
		for _, d := range r.Material.Documents {
			p.docs = append(p.docs, visibleDocument{d.EventAt, d.Kind, d.Text})
		}
		proposed[k] = append(proposed[k], p)
	}
	for _, p := range c.Telemetry {
		path := TelemetrySourcePath(p.Client, p.SessionID)
		k := affectedSession{p.Client, p.SessionID}
		touched[path] = true
		keys[k] = true
		proposed[k] = append(proposed[k], proposedCopy{path: path, priority: telemetrySourcePriority, replacement: true})
	}
	for path := range touched {
		rows, e := v.tx.QueryContext(ctx, "SELECT client,session_id FROM ("+v.copies+") WHERE source_path=?", path)
		if e != nil {
			return 0, 0, e
		}
		for rows.Next() {
			var k affectedSession
			if e = rows.Scan(&k.client, &k.id); e != nil {
				rows.Close()
				return 0, 0, e
			}
			keys[k] = true
		}
		if e = closeIndexRows(rows); e != nil {
			return 0, 0, e
		}
	}
	for k := range keys {
		rows, e := v.tx.QueryContext(ctx, "SELECT source_path,priority FROM ("+v.copies+") WHERE client=? AND session_id=? ORDER BY priority DESC,source_path", k.client, k.id)
		if e != nil {
			return 0, 0, e
		}
		var before, after proposedCopy
		for rows.Next() {
			var p proposedCopy
			if e = rows.Scan(&p.path, &p.priority); e != nil {
				rows.Close()
				return 0, 0, e
			}
			if before.path == "" {
				before = p
			}
			if !touched[p.path] && after.path == "" {
				after = p
			}
		}
		if e = closeIndexRows(rows); e != nil {
			return 0, 0, e
		}
		for _, p := range proposed[k] {
			if after.path == "" || p.priority > after.priority || p.priority == after.priority && p.path < after.path {
				after = p
			}
		}
		if before.path == after.path && !after.replacement {
			continue
		}
		load := func(p proposedCopy) ([]visibleDocument, error) {
			if p.path == "" || p.replacement {
				return p.docs, nil
			}
			rows, err := v.tx.QueryContext(ctx, "SELECT event_at,kind,text FROM ("+v.documents+") WHERE source_path=? AND client=? AND session_id=? ORDER BY detail_ordinal", p.path, k.client, k.id)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var docs []visibleDocument
			for rows.Next() {
				var d visibleDocument
				if err := rows.Scan(&d.eventAt, &d.kind, &d.text); err != nil {
					return nil, err
				}
				docs = append(docs, d)
			}
			return docs, rows.Err()
		}
		b, e := load(before)
		if e != nil {
			return 0, 0, e
		}
		a, e := load(after)
		if e != nil {
			return 0, 0, e
		}
		n, m := logicalDocumentChanges(b, a)
		changed += n
		removed += m
	}
	return changed, removed, nil
}

func closeIndexRows(rows *sql.Rows) error {
	err := rows.Err()
	if closeErr := rows.Close(); err == nil {
		return closeErr
	}
	return err
}
