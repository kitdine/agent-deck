package session

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// unchangedSources proves the entire discovered source set against committed
// checkpoints in one query. Without any index writes there can be no document
// delta, so the caller need not materialize two copies of the full FTS view.
// Unknown change times and incomplete legacy checkpoints always fall back.
func unchangedSources(ctx context.Context, db *sql.DB, paths []source) (bool, error) {
	return unchangedSourceSet(ctx, db, paths, true)
}

func unchangedSourceSet(ctx context.Context, db *sql.DB, paths []source, fullSet bool) (bool, error) {
	var tables int
	var managed bool
	if err := db.QueryRowContext(ctx, `SELECT count(*),EXISTS(SELECT 1 FROM main.sqlite_schema WHERE name='cjsi_index_state') FROM sqlite_master WHERE name IN ('session_sources','session_metadata','session_documents')`).Scan(&tables, &managed); err != nil {
		return false, err
	}
	if managed {
		return false, store.ErrCJSIIndexChanged
	}
	if tables != 3 {
		return false, nil
	}
	expected := make(map[string]source, len(paths))
	anchors := make(map[string]string, len(paths))
	for _, src := range paths {
		if !src.stable || src.changedAt == 0 {
			return false, nil
		}
		expected[filepath.Clean(src.path)] = src
	}
	rows, err := db.QueryContext(ctx, `SELECT source_path,identity,size,cursor,modified_at,changed_at,prefix_hash,priority,parser_version FROM session_sources`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	matched := 0
	for rows.Next() {
		var path, identity, anchor string
		var size, cursor, modified, changed, priority, parser int64
		if err := rows.Scan(&path, &identity, &size, &cursor, &modified, &changed, &anchor, &priority, &parser); err != nil {
			return false, err
		}
		src, found := expected[path]
		// A projected telemetry session has no file to discover, so it is never
		// part of the inventory this proof compares against.
		if !found && (!fullSet || IsTelemetrySource(path)) {
			continue
		}
		if !found || identity != src.identity || size != src.size || cursor != size ||
			modified != src.modifiedAt || changed != src.changedAt || priority != int64(src.priority) || parser != ParserVersion {
			return false, nil
		}
		anchors[path] = anchor
		matched++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if matched != len(expected) {
		return false, nil
	}
	// Validate the same observation again after the registry read. Never bless
	// a checkpoint using source metadata from before an intervening mutation.
	for _, src := range paths {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := validateUnchangedObservation(src, anchors[filepath.Clean(src.path)]); err != nil {
			return false, err
		}
	}
	return true, nil
}

// PreparedSourcesUnchanged proves a full captured inventory without touching
// FTS projections or reading unchanged source bodies.
func PreparedSourcesUnchanged(ctx context.Context, db *sql.DB, home string, sources []ingest.Source) (bool, error) {
	paths, err := sessionSources(home, sources)
	if err != nil {
		return false, err
	}
	return unchangedSources(ctx, db, paths)
}

func validateUnchangedObservation(src source, storedAnchor string) error {
	info, err := os.Stat(src.path)
	if err != nil {
		return err
	}
	observed := ingest.Source{Path: src.path, Identity: src.identity, Size: src.size,
		ModifiedAt: src.modifiedAt, ChangedAt: src.changedAt, Stable: src.stable}
	if info.Size() == src.size {
		return ingest.Validate(observed)
	}
	if err = ingest.ValidateCapturedRange(observed, src.size); err != nil {
		return err
	}
	anchor, err := prefixHash(src.path, src.size)
	if err != nil {
		return err
	}
	if anchor != storedAnchor {
		return ingest.ErrSourceChanged
	}
	return nil
}

// The migration records the prior read end and the byte boundary of its last
// complete line. Only the exact checkpoint tuple authorizes using that hint.
const compatibleLegacyHintSQL = `EXISTS(SELECT 1 FROM cjsi_legacy_cursor_hints l WHERE (l.source_path,l.identity,l.size,l.read_end,l.complete_end,l.modified_at,l.changed_at,l.prefix_hash,l.parser_version)=
               (s.source_path,s.identity,s.size,s.size,s.cursor,s.modified_at,s.changed_at,s.prefix_hash,s.parser_version))`

// CJSISourceObservation carries the coordinator's source classification.
// A migrated, unclassified checkpoint cannot opt itself into the fast path.
type CJSISourceObservation struct {
	ingest.Source
	Kind string
}

// CJSISessionParserTarget selects the domain version from trusted inventory
// classification. Zero means this source cannot enter the reusable path.
func CJSISessionParserTarget(client, kind string) int {
	switch kind {
	case "codex-jsonl":
		if client == "codex" {
			return nativeSessionParserVersion
		}
	case "claude-jsonl":
		if client == "claude" {
			return ParserVersion
		}
	case "otel", "synthetic":
		if client == "claude" || client == "codex" {
			return ParserVersion
		}
	}
	return 0
}

// CJSISourceUnchanged allows a mixed scan to retain unaffected checkpoints.
func CJSISourceUnchanged(ctx context.Context, snapshot store.CJSIIndexSnapshot, o CJSISourceObservation) (bool, error) {
	if o.Path == "" {
		return false, nil
	}
	return cjsiUnchanged(ctx, snapshot, []CJSISourceObservation{o}, o.Path)
}

// CJSISourcesUnchanged compares the full inventory to selected source rows.
func CJSISourcesUnchanged(ctx context.Context, snapshot store.CJSIIndexSnapshot, observations []CJSISourceObservation) (bool, error) {
	return cjsiUnchanged(ctx, snapshot, observations, "")
}

func cjsiUnchanged(ctx context.Context, snapshot store.CJSIIndexSnapshot, observations []CJSISourceObservation, only string) (bool, error) {
	h := snapshot.Selection()
	if snapshot.Tx == nil {
		return false, store.ErrCJSIIndexChanged
	}
	expected := make(map[string]CJSISourceObservation, len(observations))
	for _, o := range observations {
		// Only file parsers have reusable inventory checkpoints. OTel and
		// synthetic projections also have parser versions, but no file cursor.
		if o.Kind != "codex-jsonl" && o.Kind != "claude-jsonl" {
			return false, nil
		}
		if !o.Stable || o.Path == "" || o.ChangedAt == 0 || o.Identity == "" || CJSISessionParserTarget(o.Client, o.Kind) == 0 {
			return false, nil
		}
		expected[o.Path] = o
	}
	if len(expected) != len(observations) {
		return false, nil
	}
	rows, err := snapshot.Tx.QueryContext(ctx, `WITH cs AS (
        SELECT c.* FROM cjsi_candidate_sources c,cjsi_index_state s WHERE c.ticket=? AND c.attempt=? AND s.compacted_ticket<>c.ticket),
    grp AS (SELECT source_path,min(source_kind) AS min_kind,max(source_kind) AS max_kind,count(source_kind)=count(*) AS kinds_known,
        min(client) AS min_client,max(client) AS max_client FROM cjsi_base_groups WHERE (?='' OR source_path=?) GROUP BY source_path),
    sources AS (
        SELECT s.source_path,s.identity,s.size,s.cursor,s.modified_at,s.changed_at,s.priority,s.parser_version,
           g.min_kind,g.max_kind,coalesce(g.kinds_known,0),g.min_client,g.max_client,
           `+compatibleLegacyHintSQL+` AS legacy
           FROM session_sources s LEFT JOIN grp g USING(source_path) WHERE NOT EXISTS(SELECT 1 FROM cs WHERE cs.source_path=s.source_path)
        UNION ALL SELECT source_path,identity,size,complete_end,modified_at,changed_at,priority,parser_version,source_kind,source_kind,1,NULL,NULL,0 FROM cs WHERE op='upsert')
    SELECT s.*,EXISTS(SELECT 1 FROM (`+snapshot.RangesSQL()+`) r WHERE r.source_path=s.source_path AND r.identity=s.identity AND r.read_end=s.size AND r.complete_end=s.size AND r.published_end=s.size AND r.replay_from=s.size)
        FROM sources s WHERE s.identity<>? AND substr(s.source_path,1,5)<>'otel:' AND (?='' OR s.source_path=?)`, h.SessionTicket, h.SessionAttempt, only, only, replaceDocumentsSourceIdentity, only, only)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var path, identity string
		var size, cursor, modified, changed int64
		var priority, parser int
		var kind, maxKind, minClient, maxClient sql.NullString
		var kindsKnown, legacy, complete bool
		if err := rows.Scan(&path, &identity, &size, &cursor, &modified, &changed, &priority, &parser, &kind, &maxKind, &kindsKnown, &minClient, &maxClient, &legacy, &complete); err != nil {
			return false, err
		}
		o, found := expected[path]
		if !found || identity != o.Identity || size != o.Size || modified != o.ModifiedAt || changed != o.ChangedAt || priority != o.Priority || parser != CJSISessionParserTarget(o.Client, o.Kind) {
			return false, nil
		}
		if (kind.String != "" && kind.String != o.Kind) || (maxKind.String != "" && maxKind.String != o.Kind) {
			return false, nil
		}
		if o.Kind == "codex-jsonl" {
			if !kindsKnown || kind.String != o.Kind || cursor != size || !complete {
				return false, nil
			}
		} else if (!legacy && cursor != size) || (minClient.Valid && (minClient.String != o.Client || maxClient.String != o.Client)) {
			return false, nil
		}
		count++
	}
	if err := rows.Err(); err != nil || count != len(expected) {
		return false, err
	}
	for _, o := range observations {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := ingest.Validate(o.Source); err != nil {
			return false, err
		}
	}
	return true, nil
}
