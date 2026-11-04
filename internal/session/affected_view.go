package session

import (
	"context"
	"sort"
	"strings"
)

type affectedSession struct{ client, id string }

func affectedSessionKeys(ctx context.Context, e sessionExecutor, paths map[string]bool) (map[string]affectedSession, error) {
	out := map[string]affectedSession{}
	if len(paths) == 0 {
		return out, nil
	}
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	args := make([]any, 0, len(names))
	marks := make([]string, 0, len(names))
	for _, p := range names {
		args = append(args, p)
		marks = append(marks, "?")
	}
	rows, err := e.QueryContext(ctx, `SELECT client,session_id FROM session_metadata WHERE source_path IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pair affectedSession
		if err := rows.Scan(&pair.client, &pair.id); err != nil {
			return nil, err
		}
		out[pair.client+"\x00"+pair.id] = pair
	}
	return out, rows.Err()
}

// nil keys selects the full oracle. Ownership is ranked across every source of
// the selected logical sessions, so removing an owner reveals its alternative.
func visibleDocumentsFiltered(ctx context.Context, e sessionExecutor, keys map[string]affectedSession, excluded map[string]bool) (map[string][]visibleDocument, error) {
	if keys != nil && len(keys) == 0 {
		return map[string][]visibleDocument{}, nil
	}
	where := ""
	args := []any{}
	if keys != nil {
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		marks := make([]string, 0, len(names))
		for _, k := range names {
			pair := keys[k]
			marks = append(marks, "(?,?)")
			args = append(args, pair.client, pair.id)
		}
		where = " WHERE (m.client,m.session_id) IN (" + strings.Join(marks, ",") + ")"
	}
	if len(excluded) > 0 {
		if where == "" {
			where = " WHERE 1=1"
		}
		names := make([]string, 0, len(excluded))
		for p := range excluded {
			names = append(names, p)
		}
		sort.Strings(names)
		marks := make([]string, 0, len(names))
		for _, p := range names {
			marks = append(marks, "?")
			args = append(args, p)
		}
		where += " AND m.source_path NOT IN (" + strings.Join(marks, ",") + ")"
	}
	query := `WITH visible AS (SELECT m.source_path,m.client,m.session_id,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path` + where + `) SELECT d.client,d.session_id,d.event_at,d.kind,d.text FROM session_documents d JOIN visible v ON v.source_path=d.source_path AND v.client=d.client AND v.session_id=d.session_id WHERE v.n=1 ORDER BY d.client,d.session_id,d.rowid`
	rows, err := e.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]visibleDocument{}
	for rows.Next() {
		var client, id string
		var d visibleDocument
		if err := rows.Scan(&client, &id, &d.eventAt, &d.kind, &d.text); err != nil {
			return nil, err
		}
		key := client + "\x00" + id
		out[key] = append(out[key], d)
	}
	return out, rows.Err()
}

func affectedViewChanges(ctx context.Context, e sessionExecutor, changed map[string]bool, oldKeys map[string]affectedSession, before map[string][]visibleDocument) (int, int, error) {
	afterKeys, err := affectedSessionKeys(ctx, e, changed)
	if err != nil {
		return 0, 0, err
	}
	unexpected := map[string]affectedSession{}
	all := map[string]affectedSession{}
	for k, p := range oldKeys {
		all[k] = p
	}
	for k, p := range afterKeys {
		all[k] = p
		if _, known := oldKeys[k]; !known {
			unexpected[k] = p
		}
	}
	var selected map[string]affectedSession = all
	if len(all) > 128 {
		selected = nil
	}
	if len(unexpected) > 0 {
		var reconstruct map[string]affectedSession = unexpected
		if selected == nil {
			reconstruct = nil
		}
		// A newly introduced session had no pre-write row in changed sources.
		// Its old winner can therefore be reconstructed from unchanged sources.
		prior, err := visibleDocumentsFiltered(ctx, e, reconstruct, changed)
		if err != nil {
			return 0, 0, err
		}
		for k, docs := range prior {
			if _, known := oldKeys[k]; !known {
				before[k] = docs
			}
		}
	}
	after, err := visibleDocumentsFiltered(ctx, e, selected, nil)
	if err != nil {
		return 0, 0, err
	}
	changedCount, removed := visibleDocumentChanges(before, after)
	return changedCount, removed, nil
}
