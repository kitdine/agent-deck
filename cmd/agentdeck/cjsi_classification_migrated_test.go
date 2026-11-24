//go:build cjsi_adapter_fixture

package main

import (
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
)

// Use Task 5's installation-only test overlay. There is no fake index
// reader, alternate SQL view, or test-only admission for the public reads.
const migratedID = "00000000-0000-4000-8000-000000000006"
const migratedPeer = "00000000-0000-4000-8000-000000000007"
const birth = "2026-01-02T09:05:00Z"
const settled = "2026-01-02T10:00:00Z"

func migratedOriginFixture(t *testing.T, ids ...string) (string, *store.Store) {
	t.Helper()
	return migratedOriginFixtureWithClaude(t, "", ids...)
}
func migratedOriginFixtureWithClaude(t *testing.T, claudeID string, ids ...string) (string, *store.Store) {
	t.Helper()
	ctx := t.Context()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	var sessions []session.TelemetrySession
	for _, id := range ids {
		_, err = core.DB.Exec(`INSERT INTO telemetry_sessions(client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at) VALUES('codex',?,?,'m',?,?,'codex-app-server/0.165.0/codex-tui/sse',?)`, id, store.TelemetrySessionSourceKind, birth, birth, settled)
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, session.TelemetrySession{Client: "codex", SessionID: id, FirstAt: birth, LastAt: birth})
	}
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ProjectTelemetrySessions(ctx, index.DB, sessions); err != nil {
		t.Fatal(err)
	}
	if len(ids) > 1 {
		// A second physical copy stays eligible when its OTel projection is masked.
		if err := session.ReplaceDocuments(ctx, index.DB, "codex", migratedPeer, []session.Document{{Client: "codex", SessionID: migratedPeer, EventAt: birth, Kind: "user_prompt", Text: "replica"}}); err != nil {
			t.Fatal(err)
		}
	}
	if claudeID != "" {
		if err := session.ReplaceDocuments(ctx, index.DB, "claude", claudeID, []session.Document{{Client: "claude", SessionID: claudeID, EventAt: birth, Kind: "user_prompt", Text: "claude"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sessionorigin.Reconcile(ctx, core, index, sessionorigin.ReconcileOptions{StateRoot: root, Home: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		request := "user-set"
		if id != migratedID {
			request = id
		}
		before := mustOriginOK(t, runOriginCLI(t, root, "--format", "json", "session", "classify", id, "--client", "codex", "--set", "work", "--request-id", request))
		if before["data"].(map[string]any)["revision"] != float64(1) {
			t.Fatal(before)
		}
	}
	if claudeID != "" {
		mustOriginOK(t, runOriginCLI(t, root, "--format", "json", "session", "classify", claudeID, "--client", "claude", "--set", "auxiliary", "--request-id", "claude-set"))
	}
	return installOriginFixture(t, root)
}

func installOriginFixture(t *testing.T, root string) (string, *store.Store) {
	t.Helper()
	ctx := t.Context()
	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatal(err)
	}
	uri := url.URL{Scheme: "file", Path: filepath.Join(root, "agentdeck.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return root, &store.Store{DB: db}
}

func TestCJSIClassificationMigrated(t *testing.T) {
	ctx := t.Context()
	const id = migratedID
	root, c := migratedOriginFixture(t, id)
	db := c.DB
	run := func(args ...string) map[string]any {
		result, err := managedClassifyCLI(t, root, c, args...)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	page := run("list")
	if page["items"].([]any)[0].(map[string]any)["binding_status"] != "matched" {
		t.Fatal(page)
	}
	item := run(id, "--client", "codex")["origin"].(map[string]any)
	if item["effective"] != "work" || item["automatic"] != "auxiliary" || !item["override"].(map[string]any)["applied"].(bool) {
		t.Fatal(item)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	reader := sessionorigin.NewPublishedOriginReader(root, tx)
	copies, err := reader.EffectiveCopies(ctx, "codex", id)
	if err != nil || len(copies) != 1 || copies[0].Birth != "" || copies[0].HeadAt != "" || copies[0].Source.FirstAt != birth {
		t.Fatal("fixture did not exercise migrated marker", err)
	}
	binding, err := sessionorigin.ResolveBinding(ctx, reader, "codex", id)
	if err != nil || binding == nil || binding.Limited || binding.Selection == nil || binding.Digest != sessionorigin.BindingFor("codex", id, sessionorigin.Birth{StartedAt: birth}).Digest {
		t.Fatal("uncertified birth", err)
	}
	target := sessionorigin.Target{Client: "codex", SessionID: id}
	state, err := store.ReadPublishedOriginSelectionTx(ctx, tx)
	if err != nil || len(state.Overrides) != 1 || len(state.Origins) != 1 {
		t.Fatal("missing pre-migration state", err)
	}
	beforeRow := state.Overrides[0]
	ev, err := c.OriginEvidenceFor(ctx, "codex", []string{id})
	if err != nil {
		t.Fatal(err)
	}
	input := sessionorigin.OriginChangeInputs{Target: target, Previous: reader, Override: &beforeRow, Origin: &state.Origins[0], Evidence: ev[id], Marker: &store.TelemetrySessionOriginMarker{SourceKind: store.TelemetrySessionSourceKind, SupportEntry: "codex-app-server/0.165.0/codex-tui/sse", SettledAt: settled}}
	plan, err := sessionorigin.PrepareEffectiveOrigin(ctx, id, time.Now(), func(f func(sessionorigin.EffectiveCopyReader, sessionorigin.OriginChangeInputs) error) error {
		return f(reader, input)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	write, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer write.Rollback()
	if err := plan.ApplyTx(ctx, write); err != nil {
		t.Fatal(err)
	}
	if err := write.Commit(); err != nil {
		t.Fatal(err)
	}
	row, found, err := c.OriginOverrideRecord(ctx, "codex", id)
	if err != nil || !found || row.BindingState != "matched" || row.BindingDigest != beforeRow.BindingDigest || row.UpdatedAt != beforeRow.UpdatedAt || row.Revision != beforeRow.Revision {
		t.Fatal("reconciliation lost settled birth", err)
	}
	if got := run(id, "--client", "codex", "--set", "auxiliary")["revision"]; got != float64(2) {
		t.Fatal(got)
	}
	// Change the selected ticket without changing the user's binding
	// revision: the old selection may not authorize a fresh user set.
	if _, err := db.Exec(`UPDATE cjsi_publication_head SET ticket=ticket+1`); err != nil {
		t.Fatal(err)
	}
	_, err = (sessionorigin.Service{Store: c}).SetOverride(ctx, sessionorigin.OverrideSetRequest{Client: "codex", SessionID: id, Value: "work", ExpectedRevision: 2, Binding: binding})
	if !errors.Is(err, store.ErrOriginProjectionStale) {
		t.Fatal("stale ticket allowed set", err)
	}
	// No admitted settled observation at the projected birth. A detail
	// must fail rather than reporting unmatched against a matched core.
	if _, err := db.Exec(`UPDATE telemetry_sessions SET first_at=?`, settled); err != nil {
		t.Fatal(err)
	}
	if _, err := managedClassifyCLI(t, root, c, id, "--client", "codex"); !errors.Is(err, sessionorigin.ErrClassificationUnavailable) {
		t.Fatal("stale projected birth accepted", err)
	}
	if got := run(id, "--client", "codex", "--clear")["revision"]; got != float64(3) {
		t.Fatal(got)
	}
	if got := run("--request", "user-set")["revision"]; got != float64(1) {
		t.Fatal("lost historical receipt", got)
	}
	if got := run("list")["total"]; got != float64(0) {
		t.Fatal(got)
	}
}

// The adapter opens a query-only private SQLite connection for each admission.
// Count only this test's private path. The hook is installed before tests start.
var privateAdmissions sync.Map

func init() {
	sqlite.RegisterConnectionHook(func(_ sqlite.ExecQuerierContext, dsn string) error {
		u, err := url.Parse(dsn)
		if err == nil && slices.Contains(u.Query()["_pragma"], "query_only(1)") {
			if counter, ok := privateAdmissions.Load(u.Path); ok {
				counter.(*atomic.Int64).Add(1)
			}
		}
		return nil
	})
}

func TestCJSIClassifyProtection(t *testing.T) {
	ctx := t.Context()
	root, c := migratedOriginFixture(t, migratedID, migratedPeer)
	// The legacy native copy has no verified birth. Leave its override
	// unmatched after migration, as the publisher does for unproven continuity.
	if _, err := c.DB.Exec("UPDATE origin_overrides SET binding_state='unmatched' WHERE session_id=?", migratedPeer); err != nil {
		t.Fatal(err)
	}
	tx, err := c.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	reader := sessionorigin.NewPublishedOriginReader(root, tx)
	binding, err := sessionorigin.ResolveBinding(ctx, reader, "codex", migratedID)
	if err != nil || binding == nil {
		t.Fatal("initial binding", err)
	}
	privatePath := filepath.Join(root, "session-index-v1.sqlite3")
	var opens atomic.Int64
	privateAdmissions.Store(privatePath, &opens)
	t.Cleanup(func() { privateAdmissions.Delete(privatePath) })
	view, err := reader.Select(ctx, sessionorigin.Selection{Scope: sessionorigin.ScopeAll})
	if err != nil || len(view.Sessions) != 2 {
		t.Fatal("initial selection", err)
	}
	if n := opens.Load(); n != 1 {
		t.Errorf("selection opened %d private snapshots for two targets; want one", n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: privatePath, RawQuery: "mode=rw"}
	private, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer private.Close()
	exec := func(db *sql.DB, stmt string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate one malformed unrelated value in an otherwise admitted
	// physical fixture. Keep the published revision fixed so the only failure
	// a full scan can encounter here is decoding the peer's parser version.
	setPeerVersion := func(version any) {
		t.Helper()
		editOriginFixtureGroup(t, root, "UPDATE session_metadata SET parser_version=? WHERE session_id=?", version, migratedPeer)
	}
	setPeerVersion("invalid")
	if _, err := managedClassifyCLI(t, root, c, migratedID, "--client", "codex"); err != nil {
		t.Errorf("single-ID lookup consumed unrelated metadata: %v", err)
	}
	setPeerVersion(session.ParserVersion)
	for _, id := range []string{migratedID, migratedPeer} {
		exec(private, "INSERT INTO cjsi_session_protections VALUES(?,'codex',?,0)", session.TelemetrySourcePath("codex", id), id)
	}
	if _, err := managedClassifyCLI(t, root, c, "list"); !errors.Is(err, store.ErrCJSIIndexChanged) {
		t.Errorf("expected changed index at list admission, got %v", err)
	}
	_, err = (sessionorigin.Service{Store: c}).SetOverride(ctx, sessionorigin.OverrideSetRequest{RequestID: "after-mask", Client: "codex", SessionID: migratedID, Value: "auxiliary", ExpectedRevision: 1, Binding: binding})
	if !errors.Is(err, store.ErrOriginProjectionStale) {
		t.Errorf("set after private protection: %v; want stale", err)
	}
	if _, found, err := c.OriginReceipt(ctx, "after-mask"); err != nil || found {
		t.Errorf("rejected set receipt found=%v err=%v", found, err)
	}
	row, found, err := c.OriginOverrideRecord(ctx, "codex", migratedID)
	if err != nil || !found || row.Revision != 1 || row.Value == nil || *row.Value != "work" {
		t.Errorf("protected override after set: %+v, %v", row, err)
	}
	exec(c.DB, "UPDATE cjsi_publication_head SET protection_revision=2")
	run := func(args ...string) map[string]any {
		t.Helper()
		v, err := managedClassifyCLI(t, root, c, args...)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	page := run("list", "--limit", "1")
	items := page["items"].([]any)
	if page["total"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["session_id"] != migratedPeer || items[0].(map[string]any)["binding_status"] != "unmatched" {
		t.Errorf("eligible override on first page: %v", page)
	}
	if page := run("list", "--limit", "1", "--page", "2"); page["total"] != float64(1) || len(page["items"].([]any)) != 0 {
		t.Errorf("hidden override affected paging: %v", page)
	}
	if got := run("--request", "user-set")["revision"]; got != float64(1) {
		t.Errorf("historic receipt revision %v", got)
	}
	if got := run(migratedID, "--client", "codex", "--clear")["revision"]; got != float64(2) {
		t.Errorf("explicit clear revision %v", got)
	}
}

func editOriginFixtureGroup(t *testing.T, root, stmt string, args ...any) {
	t.Helper()
	u := url.URL{Scheme: "file", Path: filepath.Join(root, "session-index-v1.sqlite3"), RawQuery: "mode=rw"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), stmt, args...); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(t.Context(), "UPDATE serving_generation SET revision=(SELECT base_revision FROM cjsi_index_state WHERE singleton=1) WHERE singleton=1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestCJSIManagedClaudeRebind(t *testing.T) {
	const id = "claude-reused"
	root, c := migratedOriginFixtureWithClaude(t, id, migratedID)
	editOriginFixtureGroup(t, root, "UPDATE session_metadata SET first_at=? WHERE client='claude' AND session_id=?", settled, id)
	run := func(args ...string) map[string]any {
		t.Helper()
		v, err := managedClassifyCLI(t, root, c, args...)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	page := run("list")
	rows := page["items"].([]any)
	if page["total"] != float64(2) || len(rows) != 2 || rows[0].(map[string]any)["binding_status"] != "unmatched" || rows[1].(map[string]any)["binding_status"] != "matched" {
		t.Fatal("list bindings", page)
	}
	claude := run(id, "--client", "claude")["origin"].(map[string]any)
	if o := claude["override"].(map[string]any); o["binding_status"] != "unmatched" || o["applied"] != false || claude["effective"] != claude["automatic"] {
		t.Fatal("stale Claude binding", claude)
	}
	codex := run(migratedID, "--client", "codex")["origin"].(map[string]any)
	if codex["effective"] != "work" || codex["override"].(map[string]any)["applied"] != true {
		t.Fatal("Codex changed", codex)
	}
	tx, err := c.DB.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	view, err := sessionorigin.NewPublishedOriginReader(root, tx).Select(t.Context(), sessionorigin.Selection{Scope: sessionorigin.ScopeWork})
	tx.Rollback()
	if err != nil || len(view.Sessions) != 2 {
		t.Fatal("work selection", len(view.Sessions), err)
	}
	if v := run(id, "--client", "claude", "--set", "auxiliary"); v["revision"] != float64(2) {
		t.Fatal("rebind", v)
	}
	rebound := run(id, "--client", "claude")["origin"].(map[string]any)
	if rebound["effective"] != "auxiliary" || rebound["override"].(map[string]any)["applied"] != true {
		t.Fatal("Claude explicit override", rebound)
	}
}

func TestCJSIGenericBindingWrite(t *testing.T) {
	ctx := t.Context()
	root, c := migratedOriginFixture(t, migratedID)
	tx, err := c.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	reader := sessionorigin.NewPublishedOriginReader(root, tx)
	var selection store.CJSIIndexStamp
	with := func(f func(sessionorigin.EffectiveCopyReader, store.CJSIIndexStamp) error) error {
		return store.WithCJSIIndexSnapshot(ctx, root, tx, func(s store.CJSIIndexSnapshot) error {
			selection = s.Selection()
			return f(reader, selection)
		})
	}
	b, err := sessionorigin.ResolveEffectiveBinding(ctx, sessionorigin.Target{Client: "codex", SessionID: migratedID}, with)
	tx.Rollback()
	if err != nil || b == nil {
		t.Fatal("resolve managed binding", err)
	}
	if b.Selection == nil || *b.Selection != selection {
		t.Error("generic binding lost its admitted stamp")
	}
	receipt, err := (sessionorigin.Service{Store: c}).SetOverride(ctx, sessionorigin.OverrideSetRequest{RequestID: "generic-set", Client: "codex", SessionID: migratedID, Value: "auxiliary", ExpectedRevision: 1, Binding: b})
	if err != nil || receipt.Revision != 2 {
		t.Fatal("managed set", receipt, err)
	}
}

func TestCJSIOverrideKeyPages(t *testing.T) {
	root, c := migratedOriginFixture(t, migratedID, migratedPeer)
	// Put two eligible rows on opposite sides of a batch boundary. The keys
	// between them have user overrides and explicit session exclusions.
	if _, err := c.DB.Exec("UPDATE origin_overrides SET binding_state='unmatched' WHERE session_id=?", migratedPeer); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DB.Exec(`WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<201)
 INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at)
 SELECT 'codex',?||'-'||i,'absent',0,'unmatched','work',1,? FROM n`, migratedID, settled); err != nil {
		t.Fatal(err)
	}
	editOriginFixtureGroup(t, root, `WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<201)
 INSERT INTO session_exclusions(kind,value) SELECT 'session',?||'-'||i FROM n`, migratedID)
	for i, id := range []string{migratedID, migratedPeer} {
		page, err := managedClassifyCLI(t, root, c, "list", "--page", strconv.Itoa(i+1), "--limit", "1")
		if err != nil || page["total"] != float64(2) {
			t.Fatal("batch total", page, err)
		}
		if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["session_id"] != id {
			t.Fatal("batch order", i, items)
		}
	}
	if _, err := c.DB.Exec("DELETE FROM origin_overrides WHERE session_id=?", migratedPeer); err != nil {
		t.Fatal(err)
	}
	editOriginFixtureGroup(t, root, "UPDATE session_metadata SET parser_version='invalid' WHERE session_id=?", migratedPeer)
	tx, err := c.DB.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	reader := sessionorigin.NewPublishedOriginReader(root, tx)
	for _, limit := range []int{1, 0} {
		page, err := reader.ListOverrides(t.Context(), 1, limit)
		want := limit
		if want == 0 {
			want = sessionorigin.DefaultOverrideLimit
		}
		if err != nil || page.Limit != want || page.Total != 1 || len(page.Items) != 1 || page.Items[0].SessionID != migratedID {
			t.Errorf("limit %d, unrelated group blocked page: %+v err=%v", limit, page, err)
		}
	}
}

func TestCJSIAbsentOverrideList(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	_, c := installOriginFixture(t, root)
	read := func() (map[string]any, error) { return managedClassifyCLI(t, root, c, "list") }
	page, err := read()
	if err != nil || page["total"] != float64(0) {
		t.Errorf("certified absence with no overrides: %v %v", page, err)
	}
	_, err = c.DB.Exec(`INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES('codex','archived-1','old-digest',0,'unmatched','auxiliary',4,?)`, settled)
	if err != nil {
		t.Fatal(err)
	}
	page, err = read()
	if err != nil || page["total"] != float64(1) {
		t.Fatalf("certified absence with restored override: %v %v", page, err)
	}
	row := page["items"].([]any)[0].(map[string]any)
	if row["session_id"] != "archived-1" || row["revision"] != float64(4) || row["binding_status"] != "unmatched" || row["value"] != "auxiliary" {
		t.Fatal(row)
	}
	// A retained group mask cannot be mapped back to this user choice
	// without the index. An unavailable answer exposes neither its ID nor count.
	if _, err := c.DB.Exec(`INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision)
 VALUES('origin','group',zeroblob(32),zeroblob(32),0,0,1,0,1)`); err != nil {
		t.Fatal(err)
	}
	if protected, err := read(); !errors.Is(err, sessionorigin.ErrClassificationUnavailable) || protected != nil {
		t.Fatal("unresolved mask after restore", protected, err)
	}
	if _, err := c.DB.Exec("DELETE FROM cjsi_scope_overlays WHERE domain='origin'"); err != nil {
		t.Fatal(err)
	}
	cleared, err := managedClassifyCLI(t, root, c, "archived-1", "--client", "codex", "--clear")
	if err != nil || cleared["revision"] != float64(5) {
		t.Fatal("clear", cleared, err)
	}
	if page, err = read(); err != nil || page["total"] != float64(0) {
		t.Fatal("tombstone", page, err)
	}
	// Purge retains user state. A formerly matched choice has no live
	// birth in the absent index; the read must not rewrite its durable row.
	if _, err := c.DB.Exec(`INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES('codex','before-purge','old',1,'matched','work',2,?)`, settled); err != nil {
		t.Fatal(err)
	}
	page, err = read()
	if err != nil || page["total"] != float64(1) {
		t.Fatal("retained purge choice", page, err)
	}
	if item := page["items"].([]any)[0].(map[string]any); item["session_id"] != "before-purge" || item["binding_status"] != "unmatched" || item["binding_limited"] != true || item["revision"] != float64(2) {
		t.Fatal(item)
	}
	saved, found, err := c.OriginOverrideRecord(t.Context(), "codex", "before-purge")
	if err != nil || !found || saved.BindingState != "matched" || saved.Revision != 2 {
		t.Fatal("read changed retained choice", saved, err)
	}
	// A missing file accompanied by a nonzero index epoch is not verified absence.
	digest := [32]byte{1}
	if _, err = c.DB.Exec("UPDATE cjsi_publication_head SET index_epoch=1,base_seal=?,session_seal=?", digest[:], digest[:]); err != nil {
		t.Fatal(err)
	}
	if page, err = read(); err == nil || !errors.Is(err, store.ErrCJSIIndexChanged) || page != nil {
		t.Fatal("uncertified absence disclosed page", page, err)
	}
}

func seedUnindexedOverride(t *testing.T, c *store.Store, client, id string) {
	t.Helper()
	_, err := c.DB.Exec(`INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at)
 VALUES(?,?,'historical-birth',0,'unmatched','auxiliary',4,?)`, client, id, settled)
	if err != nil {
		t.Fatal(err)
	}
}

// All expected rows fit one page; this checks both the public order and total.
func checkOverrideList(t *testing.T, root string, c *store.Store, want ...string) []any {
	t.Helper()
	page, err := managedClassifyCLI(t, root, c, "list")
	if err != nil {
		t.Fatal(err)
	}
	rows := page["items"].([]any)
	var got []string
	for _, v := range rows {
		row := v.(map[string]any)
		got = append(got, row["client"].(string)+"/"+row["session_id"].(string)+"/"+row["binding_status"].(string))
	}
	if page["total"] != float64(len(want)) || !slices.Equal(got, want) {
		t.Fatalf("total=%v rows=%v, want %v", page["total"], got, want)
	}
	return rows
}

func TestCJSIUnindexedOverrides(t *testing.T) {
	t.Run("empty-existing-index", func(t *testing.T) {
		root, c := migratedOriginFixture(t)
		seedUnindexedOverride(t, c, "codex", "archived-1")
		row := checkOverrideList(t, root, c, "codex/archived-1/unmatched")[0].(map[string]any)
		if row["revision"] != float64(4) || row["value"] != "auxiliary" {
			t.Fatal(row)
		}
		if _, err := managedClassifyCLI(t, root, c, "archived-1", "--client", "codex"); err == nil {
			t.Fatal("listed user choice became a live session")
		}
		if _, err := managedClassifyCLI(t, root, c, "archived-1", "--client", "codex", "--set", "work"); err == nil {
			t.Fatal("unindexed choice accepted set")
		}
		clear, err := managedClassifyCLI(t, root, c, "archived-1", "--client", "codex", "--clear", "--expected-revision", "4")
		if err != nil || clear["revision"] != float64(5) {
			t.Fatal(clear, err)
		}
		checkOverrideList(t, root, c)
	})
	t.Run("deleted-claude-and-distinct-live-codex", func(t *testing.T) {
		const id = "deleted-claude"
		root, c := migratedOriginFixtureWithClaude(t, id, migratedID)
		editOriginFixtureGroup(t, root, "DELETE FROM session_metadata WHERE client='claude' AND session_id=?", id)
		seedUnindexedOverride(t, c, "codex", "archived-1")
		checkOverrideList(t, root, c, "claude/"+id+"/unmatched", "codex/"+migratedID+"/matched", "codex/archived-1/unmatched")
		// Listing retained user state cannot add a session to the index.
		read, err := c.DB.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		reader := sessionorigin.NewPublishedOriginReader(root, read)
		lookup, lookupErr := reader.Lookup(t.Context(), "claude", id)
		all, selectErr := reader.Select(t.Context(), sessionorigin.Selection{Scope: sessionorigin.ScopeAll})
		read.Rollback()
		if lookupErr != nil || len(lookup.Sessions) != 0 || lookup.IndexEmpty() {
			t.Fatal("deleted Claude lookup", lookup, lookupErr)
		}
		if selectErr != nil || len(all.Sessions) != 1 || all.Sessions[0].SessionID != migratedID {
			t.Fatal("retained overrides entered selection", all.Sessions, selectErr)
		}
		o, found, err := c.OriginOverrideRecord(t.Context(), "claude", id)
		if err != nil || !found || o.BindingState != "matched" || o.Revision != 1 {
			t.Fatal("list rewrote durable Claude choice", o, err)
		}
		if _, err := c.DB.Exec("UPDATE origin_overrides SET binding_state='matched' WHERE session_id='archived-1'"); err != nil {
			t.Fatal(err)
		}
		page, err := managedClassifyCLI(t, root, c, "list", "--limit", "1")
		if !errors.Is(err, sessionorigin.ErrClassificationUnavailable) || page != nil {
			t.Fatal("missing matched Codex exposed a page", page, err)
		}
	})
	t.Run("hidden-keys-and-core-mask", func(t *testing.T) {
		root, c := migratedOriginFixture(t, migratedID)
		for _, id := range []string{"archived-1", "session-hidden", "group-hidden"} {
			seedUnindexedOverride(t, c, "codex", id)
		}
		seedUnindexedOverride(t, c, "claude", "client-hidden")
		editOriginFixtureGroup(t, root, "INSERT INTO session_exclusions VALUES('client','claude'),('session','session-hidden'),('path',?)", session.TelemetrySourcePath("codex", migratedID))
		editOriginFixtureGroup(t, root, "INSERT INTO cjsi_session_protections VALUES('historical-path','codex','group-hidden',0)")
		if _, err := c.DB.Exec("UPDATE cjsi_publication_head SET protection_revision=1"); err != nil {
			t.Fatal(err)
		}
		checkOverrideList(t, root, c, "codex/archived-1/unmatched")
		if _, err := c.DB.Exec(`INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision)
 VALUES('session','group',zeroblob(32),zeroblob(32),0,0,1,0,1)`); err != nil {
			t.Fatal(err)
		}
		page, err := managedClassifyCLI(t, root, c, "list")
		if !errors.Is(err, sessionorigin.ErrClassificationUnavailable) || page != nil {
			t.Fatal("unresolved core mask exposed unindexed choices", page, err)
		}
	})
}

func TestCJSISelectedMissingGroups(t *testing.T) {
	ctx := t.Context()
	root, c := migratedOriginFixture(t, migratedID)
	// Both attempts replace the same compacted OTel base. Only attempt 2
	// is published. A later path protection masks its selected new group.
	candidates := []string{"draft-only", "selected-protected"}
	for _, id := range candidates {
		seedUnindexedOverride(t, c, "codex", id)
	}
	if _, err := c.DB.Exec("UPDATE origin_overrides SET binding_state='unmatched' WHERE session_id=?", migratedID); err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(root, "session-index-v1.sqlite3"), RawQuery: "mode=rw&_pragma=foreign_keys(1)"}
	private, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer private.Close()
	for i, id := range candidates {
		attempt := int64(i + 1)
		read, err := c.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		var parent store.CJSIIndexStamp
		var plan *session.CJSIIndexCandidate
		err = store.WithCJSIIndexSnapshot(ctx, root, read, func(s store.CJSIIndexSnapshot) error {
			parent = s.Selection()
			sessions := []session.TelemetrySession{{Client: "codex", SessionID: id, FirstAt: birth, LastAt: birth}}
			var e error
			plan, e = session.PlanCJSIIndex(ctx, s, 1, attempt, parent.SessionRevision+1, nil, nil, &sessions)
			return e
		})
		read.Rollback()
		if err != nil || plan == nil {
			t.Fatal("candidate planning", err)
		}
		write, err := private.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		seal := plan.Seal()
		if err := plan.Stage(ctx, write, parent, 1, attempt, parent.SessionRevision+1, seal); err != nil {
			write.Rollback()
			t.Fatal(err)
		}
		if err := write.Commit(); err != nil {
			t.Fatal(err)
		}
		if attempt == 2 {
			if _, err := c.DB.Exec("UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=2,session_revision=?,session_seal=?", parent.SessionRevision+1, seal[:]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := private.Exec("INSERT INTO cjsi_session_protections VALUES(?,'','',0)", session.TelemetrySourcePath("codex", "selected-protected")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DB.Exec("UPDATE cjsi_publication_head SET protection_revision=1"); err != nil {
		t.Fatal(err)
	}
	checkOverrideList(t, root, c, "codex/"+migratedID+"/unmatched", "codex/draft-only/unmatched")
}
