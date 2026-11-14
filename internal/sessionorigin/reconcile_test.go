package sessionorigin

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem"
	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem/claudememtest"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

// All databases below are isolated fixtures under t.TempDir().

var receivedAt = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type stores struct {
	root, home     string
	core, sessions *store.Store
}

func openStores(t *testing.T) stores {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	core, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	sessions, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Close() })
	return stores{root: root, home: claudememtest.Home(t, claudememtest.ReviewedInstall()), core: core, sessions: sessions}
}

func (s stores) reconcile(t *testing.T, options ReconcileOptions) ReconcileSummary {
	t.Helper()
	options.StateRoot = s.root
	if options.Home == "" {
		options.Home = s.home
	}
	if options.Now == nil {
		options.Now = func() time.Time { return receivedAt.Add(time.Hour) }
	}
	summary, err := Reconcile(context.Background(), s.core, s.sessions, options)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

// indexSession adds one indexed copy of a canonical session.
func (s stores) indexSession(t *testing.T, client, id, path, identity, prefix, firstAt string) {
	t.Helper()
	if _, err := s.sessions.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,?,1,1,'2026-10-29T12:00:00Z')`, path, identity, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := s.sessions.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,'proj','m',1,?,?)`, path, client, id, firstAt, firstAt); err != nil {
		t.Fatal(err)
	}
}

// OTel export bodies in the shape of the scan-performance isolated producer
// experiment (codex_exec and codex-app-server/codex-tui 0.165.0).
func otelAttr(key, value string) string {
	return fmt.Sprintf(`{"key":%q,"value":{"stringValue":%q}}`, key, value)
}

func otelInt(key, value string) string {
	return fmt.Sprintf(`{"key":%q,"value":{"intValue":%q}}`, key, value)
}

func otelExport(service, originator, conversation string) string {
	resource := "[" + otelAttr("service.name", service) + "," + otelAttr("service.version", "0.165.0") + "]"
	record := "[" + strings.Join([]string{
		otelAttr("event.name", "codex.sse_event"), otelAttr("event.kind", "response.completed"), otelAttr("model", "gpt-5.6-terra"),
		otelAttr("originator", originator), otelAttr("conversation.id", conversation),
		otelInt("input_token_count", "1000"), otelInt("output_token_count", "10"), otelInt("cached_token_count", "0"),
	}, ",") + "]"
	return fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":%s},"scopeLogs":[{"logRecords":[{"timeUnixNano":"1793272990000000000","body":{"stringValue":"x"},"attributes":%s}]}]}]}`, resource, record)
}

// settleBackground imports a background conversation, lets it settle and
// projects it into the session index, exactly as a scan round does.
func (s stores) settleBackground(t *testing.T, service, originator, conversation string) {
	t.Helper()
	records, err := telemetry.Decode([]byte(otelExport(service, originator, conversation)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := telemetry.Import(context.Background(), s.core, records, receivedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := s.core.DB.Exec(`INSERT INTO inventory_runs(inventory_id, epoch, captured_at, source_count, state) VALUES(?, 1, ?, 0, 'complete')`, "inv-"+conversation, receivedAt.Add(time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	s.settle(t)
	var markers int
	if err := s.core.DB.QueryRow(`SELECT count(*) FROM telemetry_sessions WHERE client='codex' AND session_id=?`, conversation).Scan(&markers); err != nil || markers != 1 {
		t.Fatalf("%s not settled: %d %v", conversation, markers, err)
	}
}

func (s stores) settle(t *testing.T) {
	t.Helper()
	if _, err := telemetry.SettleAndProject(context.Background(), s.core, s.sessions, receivedAt.Add(telemetry.SettleAfter)); err != nil {
		t.Fatal(err)
	}
}

func (s stores) origin(t *testing.T, client, id string) (store.SessionOrigin, bool) {
	t.Helper()
	origins, err := s.core.SessionOrigins(context.Background(), client, []string{id})
	if err != nil {
		t.Fatal(err)
	}
	origin, ok := origins[id]
	return origin, ok
}

func (s stores) wantOrigin(t *testing.T, client, id, automatic, reason, rule string) store.SessionOrigin {
	t.Helper()
	origin, ok := s.origin(t, client, id)
	if !ok || origin.Automatic != automatic || origin.ReasonCode != reason || origin.RuleVersion != rule {
		t.Fatalf("%s/%s = %+v (found %v), want %s/%s/%s", client, id, origin, ok, automatic, reason, rule)
	}
	return origin
}

func dump(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	var out []string
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprint(values...))
	}
	return strings.Join(out, "\n")
}

func servingRevision(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var revision int64
	if err := db.QueryRow(`SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

const usageIdentity = `SELECT event_key, client, session_id, model, input_tokens, output_tokens, source_path FROM usage_events ORDER BY event_key`
const usageSessions = `SELECT * FROM usage_sessions ORDER BY client, session_id`

func TestCodexOTelBackgroundSessionsAreMaintenance(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "memories-exec")
	s.settleBackground(t, "codex-app-server", "codex-tui", "title-tui")
	eventsBefore, sessionsBefore := dump(t, s.core.DB, usageIdentity), dump(t, s.core.DB, usageSessions)
	if eventsBefore == "" {
		t.Fatal("no OTel contribution to compare")
	}

	s.reconcile(t, ReconcileOptions{})
	for _, id := range []string{"memories-exec", "title-tui"} {
		s.wantOrigin(t, "codex", id, ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1)
	}
	evidence, err := s.core.OriginEvidenceFor(context.Background(), "codex", []string{"memories-exec"})
	if err != nil || len(evidence["memories-exec"]) != 1 || evidence["memories-exec"][0].Adapter != SourceCodexOTelBackground ||
		evidence["memories-exec"][0].ValidatedVersion != "codex_exec/0.165.0/codex_exec/sse" {
		t.Fatalf("evidence %+v %v", evidence, err)
	}
	// Classification changes no OTel contribution, event identity, token or amount.
	if dump(t, s.core.DB, usageIdentity) != eventsBefore || dump(t, s.core.DB, usageSessions) != sessionsBefore {
		t.Fatal("reconcile changed usage")
	}
	// The same ID on another client is not hidden by a codex marker.
	s.indexSession(t, "claude", "memories-exec", "/home/.claude/projects/p/memories-exec.jsonl", "claude-copy", "p", "2026-10-29T11:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "claude", "memories-exec", ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	s.wantOrigin(t, "codex", "memories-exec", ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1)
}

func TestLateRolloutRevokesCodexMaintenance(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "late-1")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "codex", "late-1", ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1)

	// A late rollout JSONL event retracts the marker in the same transaction.
	if _, err := s.core.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('jsonl-late','codex','late-1','jsonl-late','2026-10-29T12:30:00Z','gpt-5.6-terra',100,5,'/home/.codex/sessions/rollout-late-1.jsonl',0)`); err != nil {
		t.Fatal(err)
	}
	var markers int
	if err := s.core.DB.QueryRow(`SELECT count(*) FROM telemetry_sessions WHERE session_id='late-1'`).Scan(&markers); err != nil || markers != 0 {
		t.Fatalf("marker not retracted: %d %v", markers, err)
	}
	s.settle(t)
	s.indexSession(t, "codex", "late-1", "/home/.codex/sessions/rollout-late-1.jsonl", "rollout-late-1", "r", "2026-10-29T12:30:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "codex", "late-1", ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	if evidence, _ := s.core.OriginEvidenceFor(context.Background(), "codex", []string{"late-1"}); len(evidence["late-1"]) != 0 {
		t.Fatalf("stale evidence kept: %+v", evidence)
	}
}

func TestCodexMarkersOutsideTheRuleAndUnmarkedOTelSourcesAreUnknown(t *testing.T) {
	s := openStores(t)
	// A marker from a support entry this rule version does not cover.
	if _, err := s.core.DB.Exec(`INSERT INTO telemetry_sessions(client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at) VALUES('codex','future-1','codex-otel-background','m','2026-10-29T12:00:00Z','2026-10-29T12:00:00Z','codex_exec/0.166.0/codex_exec/sse','2026-10-29T12:10:00Z')`); err != nil {
		t.Fatal(err)
	}
	s.settle(t)
	// An otel:-prefixed source with an empty project but no marker.
	s.indexSession(t, "codex", "plain-1", "otel:codex/plain-1", "otel:codex/plain-1", "", "2026-10-29T12:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "codex", "future-1", ClassUnknown, ReasonOriginRuleUnsupported, RuleCodexOTelBackgroundV1)
	s.wantOrigin(t, "codex", "plain-1", ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
}

func TestUnchangedPassWritesNothingAndRemovedSessionsLoseDerivedRows(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "bg-1")
	s.indexSession(t, "codex", "work-1", "/r/work-1.jsonl", "work-1", "w", "2026-10-29T10:00:00Z")
	first := s.reconcile(t, ReconcileOptions{})
	if first.Sessions != 2 || first.Changed != 2 {
		t.Fatalf("first pass %+v", first)
	}
	revision := servingRevision(t, s.core.DB)
	snapshot := dump(t, s.core.DB, `SELECT * FROM origin_evidence`) + dump(t, s.core.DB, `SELECT * FROM session_origins`)
	again := s.reconcile(t, ReconcileOptions{Now: func() time.Time { return receivedAt.Add(48 * time.Hour) }})
	if again.Changed != 0 || again.Removed != 0 || servingRevision(t, s.core.DB) != revision {
		t.Fatalf("an unchanged pass wrote: %+v", again)
	}
	if dump(t, s.core.DB, `SELECT * FROM origin_evidence`)+dump(t, s.core.DB, `SELECT * FROM session_origins`) != snapshot {
		t.Fatal("unchanged pass rewrote rows")
	}
	// An appended or moved copy keeps identity and prefix; a rewrite changes the token.
	before, _ := s.origin(t, "codex", "work-1")
	if _, err := s.sessions.DB.Exec(`UPDATE session_sources SET size=size+100 WHERE identity='work-1'`); err != nil {
		t.Fatal(err)
	}
	s.indexSession(t, "codex", "work-1", "/moved/work-1.jsonl", "work-1", "w", "2026-10-29T10:00:00Z")
	if _, err := s.sessions.DB.Exec(`DELETE FROM session_sources WHERE source_path='/r/work-1.jsonl'`); err != nil {
		t.Fatal(err)
	}
	if moved := s.reconcile(t, ReconcileOptions{}); moved.Changed != 0 {
		t.Fatalf("append/move reclassified: %+v", moved)
	}
	if _, err := s.sessions.DB.Exec(`UPDATE session_sources SET prefix_hash='rewritten' WHERE identity='work-1'`); err != nil {
		t.Fatal(err)
	}
	if rewritten := s.reconcile(t, ReconcileOptions{}); rewritten.Changed != 1 {
		t.Fatalf("rewrite not reclassified: %+v", rewritten)
	}
	if after, _ := s.origin(t, "codex", "work-1"); after.InputToken == before.InputToken {
		t.Fatal("rewrite kept the input token")
	}
	// A session leaving the index loses its derived rows; overrides are untouched.
	s.override(t, "codex", "work-1", "auxiliary")
	if _, err := s.sessions.DB.Exec(`DELETE FROM session_sources WHERE identity='work-1'`); err != nil {
		t.Fatal(err)
	}
	if removed := s.reconcile(t, ReconcileOptions{}); removed.Removed != 1 {
		t.Fatalf("removal %+v", removed)
	}
	if _, ok := s.origin(t, "codex", "work-1"); ok {
		t.Fatal("removed session kept its classification")
	}
	if record, found, err := s.core.OriginOverrideRecord(context.Background(), "codex", "work-1"); err != nil || !found || record.Value == nil {
		t.Fatalf("override lost: %+v %v %v", record, found, err)
	}
}

func runText(run store.OriginAdapterRun) string {
	text := run.Adapter + "|" + run.Outcome + "|" + run.FinishedAt
	for _, value := range []*string{run.Reason, run.ValidatedVersion} {
		if value != nil {
			text += "|" + *value
		} else {
			text += "|<nil>"
		}
	}
	return text
}

func (s stores) override(t *testing.T, client, id, value string) {
	t.Helper()
	sources, err := s.sessions.SessionOriginSourcesFor(context.Background(), client, id)
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources: %v", err)
	}
	binding := BindingFor(client, id, BirthOf(sources))
	if _, err := s.core.ApplyOriginMutation(context.Background(), store.OriginMutation{RequestID: "req-" + id, PayloadHash: "h-" + id, Kind: store.OriginMutationOverrideSet,
		Client: client, SessionID: id, Value: value, Binding: binding, Now: receivedAt}); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateCopiesShareOneCanonicalClassification(t *testing.T) {
	s := openStores(t)
	s.settleBackground(t, "codex_exec", "codex_exec", "dup-1")
	// An archived copy of the same canonical session (a file copy, not a rollout of the background session).
	s.indexSession(t, "codex", "dup-1", "/archive/dup-1.jsonl", "archive-dup-1", "a", "2026-10-29T12:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "codex", "dup-1", ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1)
	var rows int
	if err := s.core.DB.QueryRow(`SELECT count(*) FROM session_origins WHERE session_id='dup-1'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows=%d %v", rows, err)
	}
}

const (
	observerID = "11111111-1111-4111-8111-111111111111"
	userID     = "22222222-2222-4222-8222-222222222222"
	childID    = "44444444-4444-4444-8444-444444444444"
)

func TestClaudeMemObserverSessionsEndToEnd(t *testing.T) {
	s := openStores(t)
	fixture := claudememtest.New(t, claudemem.SupportedSchemaVersion, []claudememtest.Mapping{
		{RowID: 7, ContentID: userID, MemoryID: observerID},
		{RowID: 8, ContentID: childID, MemoryID: "gemini-" + childID + "-1700000000"},
	})
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir}}); err != nil {
		t.Fatal(err)
	}
	// The observer transcript, the user's own session (content ID only), a
	// user task child, and a codex session reusing the observer UUID.
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	s.indexSession(t, "claude", userID, "/c/user.jsonl", "user", "u", "2026-10-29T09:00:00Z")
	s.indexSession(t, "claude", childID, "/c/child.jsonl", "child", "c", "2026-10-29T09:30:00Z")
	s.indexSession(t, "codex", observerID, "/x/rollout.jsonl", "codex-copy", "x", "2026-10-29T10:00:00Z")

	summary := s.reconcile(t, ReconcileOptions{})
	if summary.ClaudeMem == nil || summary.ClaudeMem.Outcome != claudemem.OutcomeOK {
		t.Fatalf("summary %+v", summary)
	}
	s.wantOrigin(t, "claude", observerID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)
	for _, id := range []string{userID, childID} {
		s.wantOrigin(t, "claude", id, ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	}
	s.wantOrigin(t, "codex", observerID, ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	run, found, err := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem)
	if err != nil || !found || run.Outcome != "ok" || run.ValidatedVersion == nil || *run.ValidatedVersion != claudemem.ValidatedVersion {
		t.Fatalf("run %+v %v %v", run, found, err)
	}
	// Nothing derived carries the source's content or path.
	derived := dump(t, s.core.DB, `SELECT * FROM origin_evidence`) + dump(t, s.core.DB, `SELECT * FROM origin_adapter_runs`) + dump(t, s.core.DB, `SELECT * FROM session_origins`)
	for _, leak := range []string{claudememtest.Secret, fixture.Dir, "proj"} {
		if strings.Contains(derived, leak) {
			t.Fatalf("derived state leaks %q", leak)
		}
	}

	// Disabling the adapter invalidates its evidence; overrides and the last
	// run summary stay.
	s.override(t, "claude", userID, "auxiliary")
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir, Disabled: true}}); err != nil {
		t.Fatal(err)
	}
	if disabled := s.reconcile(t, ReconcileOptions{Now: func() time.Time { return receivedAt.Add(3 * time.Hour) }}); disabled.ClaudeMem != nil {
		t.Fatalf("disabled adapter ran: %+v", disabled)
	}
	s.wantOrigin(t, "claude", observerID, ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	if kept, _, _ := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem); runText(kept) != runText(run) {
		t.Fatalf("run summary changed by disable: %+v", kept)
	}
	if record, found, _ := s.core.OriginOverrideRecord(context.Background(), "claude", userID); !found || record.Value == nil {
		t.Fatal("override lost on disable")
	}
	// Re-enabling restores the classification.
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir}}); err != nil {
		t.Fatal(err)
	}
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "claude", observerID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)
}

func TestClaudeMemFailuresKeepCandidatesUnknown(t *testing.T) {
	s := openStores(t)
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	s.indexSession(t, "claude", userID, "/c/user.jsonl", "user", "u", "2026-10-29T09:00:00Z")
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: "/configured"}}); err != nil {
		t.Fatal(err)
	}
	// A previously mapped observer becomes unknown when the source turns
	// unavailable, unsupported or partial; it is never kept hidden.
	results := map[string]claudemem.Result{
		"ok":          {Outcome: claudemem.OutcomeOK, ValidatedVersion: claudemem.ValidatedVersion, Mapped: map[string]int64{observerID: 7}, Checked: map[string]bool{observerID: true, userID: true}},
		"unavailable": {Outcome: claudemem.OutcomeUnavailable, Reason: claudemem.ReasonPermissionDenied},
		"unsupported": {Outcome: claudemem.OutcomeUnsupported, Reason: claudemem.ReasonSchemaVersion},
		"partial":     {Outcome: claudemem.OutcomePartial, Reason: claudemem.ReasonTimeout, ValidatedVersion: claudemem.ValidatedVersion, Mapped: map[string]int64{}, Checked: map[string]bool{userID: true}},
	}
	read := func(name string) func(context.Context, string, string, []string) claudemem.Result {
		return func(_ context.Context, _, dir string, _ []string) claudemem.Result {
			if dir != "/configured" {
				t.Fatalf("adapter read %q", dir)
			}
			return results[name]
		}
	}
	s.reconcile(t, ReconcileOptions{ReadClaudeMem: read("ok")})
	s.wantOrigin(t, "claude", observerID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)
	for _, name := range []string{"unavailable", "unsupported"} {
		s.reconcile(t, ReconcileOptions{ReadClaudeMem: read(name)})
		for _, id := range []string{observerID, userID} {
			s.wantOrigin(t, "claude", id, ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1)
		}
		run, _, _ := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem)
		if run.Outcome != results[name].Outcome || run.Reason == nil || *run.Reason != results[name].Reason {
			t.Fatalf("%s run %+v", name, run)
		}
	}
	s.reconcile(t, ReconcileOptions{ReadClaudeMem: read("partial")})
	s.wantOrigin(t, "claude", observerID, ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1)
	s.wantOrigin(t, "claude", userID, ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)

	// An unreadable configuration cannot confirm the adapter's state: no run
	// is recorded and candidates are unknown.
	if err := os.WriteFile(filepath.Join(SourceConfigDir(s.root), "sources.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _, _ := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem)
	summary := s.reconcile(t, ReconcileOptions{ReadClaudeMem: func(context.Context, string, string, []string) claudemem.Result {
		t.Fatal("adapter ran without a readable configuration")
		return claudemem.Result{}
	}})
	if summary.ClaudeMem != nil {
		t.Fatalf("summary %+v", summary)
	}
	if after, _, _ := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem); runText(after) != runText(before) {
		t.Fatal("run summary written without configuration")
	}
	s.wantOrigin(t, "claude", userID, ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1)
}

func TestNotConfiguredClaudeMemNeverRuns(t *testing.T) {
	s := openStores(t)
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	summary := s.reconcile(t, ReconcileOptions{ReadClaudeMem: func(context.Context, string, string, []string) claudemem.Result {
		t.Fatal("adapter ran without configuration")
		return claudemem.Result{}
	}})
	if summary.ClaudeMem != nil {
		t.Fatalf("summary %+v", summary)
	}
	if _, found, _ := s.core.OriginAdapterRunFor(context.Background(), SourceClaudeMem); found {
		t.Fatal("run summary without configuration")
	}
	s.wantOrigin(t, "claude", observerID, ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion)
	if _, err := os.Stat(SourceConfigDir(s.root)); !os.IsNotExist(err) {
		t.Fatalf("reconcile created configuration: %v", err)
	}
}

func TestIncrementalPassesEqualAFullRebuild(t *testing.T) {
	s := openStores(t)
	fixture := claudememtest.New(t, claudemem.SupportedSchemaVersion, []claudememtest.Mapping{{RowID: 7, ContentID: userID, MemoryID: observerID}})
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir}}); err != nil {
		t.Fatal(err)
	}
	// A sequence of incremental changes, each followed by a pass.
	s.settleBackground(t, "codex_exec", "codex_exec", "bg-1")
	s.reconcile(t, ReconcileOptions{})
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	// Late metadata: the mapping row appears after the first classification.
	s.indexSession(t, "claude", childID, "/c/child.jsonl", "child", "c", "2026-10-29T09:30:00Z")
	s.reconcile(t, ReconcileOptions{})
	if _, err := fixture.Writer.Exec(`INSERT INTO sdk_sessions(id, content_session_id, memory_session_id, project, started_at, started_at_epoch) VALUES(9, 'x', '` + childID + `', 'p', 't', 1)`); err != nil {
		t.Fatal(err)
	}
	s.settleBackground(t, "codex-app-server", "codex-tui", "bg-2")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "claude", childID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)

	const classes = `SELECT client, session_id, automatic, reason_code, rule_version, input_token FROM session_origins ORDER BY 1, 2`
	const evidence = `SELECT adapter, rule_version, client, session_id, source_token, purpose, validated_version FROM origin_evidence ORDER BY 1, 3, 4, 5`
	incremental := dump(t, s.core.DB, classes) + "\n--\n" + dump(t, s.core.DB, evidence)
	if _, err := s.core.DB.Exec(`DELETE FROM session_origins; DELETE FROM origin_evidence`); err != nil {
		t.Fatal(err)
	}
	s.reconcile(t, ReconcileOptions{})
	if full := dump(t, s.core.DB, classes) + "\n--\n" + dump(t, s.core.DB, evidence); full != incremental {
		t.Fatalf("incremental:\n%s\nfull:\n%s", incremental, full)
	}
}

func TestResolveBindingRequiresAnIndexedCopy(t *testing.T) {
	s := openStores(t)
	if binding, err := ResolveBinding(context.Background(), s.sessions, "claude", observerID); err != nil || binding != nil {
		t.Fatalf("unindexed session bound: %+v %v", binding, err)
	}
	s.indexSession(t, "claude", observerID, "/c/a.jsonl", "a", "o", "2026-10-29T10:00:00Z")
	first, err := ResolveBinding(context.Background(), s.sessions, "claude", observerID)
	if err != nil || first == nil || first.Limited {
		t.Fatalf("binding %+v %v", first, err)
	}
	// A later duplicate copy keeps the stable birth binding.
	s.indexSession(t, "claude", observerID, "/backup/a.jsonl", "b", "o", "2026-10-30T10:00:00Z")
	if again, _ := ResolveBinding(context.Background(), s.sessions, "claude", observerID); *again != *first {
		t.Fatalf("binding moved: %+v %+v", again, first)
	}
}

func TestSourceConfigIsPrivateAndNeverGuessed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	config, err := LoadSourceConfig(root)
	if err != nil || config.ClaudeMem.Configured() {
		t.Fatalf("missing configuration: %+v %v", config, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("load created the state root")
	}
	for _, bad := range []string{"relative", "/a/../b"} {
		if err := SaveSourceConfig(root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: bad}}); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	if err := SaveSourceConfig(root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: "/data/claude-mem", Disabled: true}}); err != nil {
		t.Fatal(err)
	}
	dirInfo, _ := os.Stat(SourceConfigDir(root))
	fileInfo, _ := os.Stat(filepath.Join(SourceConfigDir(root), "sources.json"))
	if dirInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("modes %v %v", dirInfo.Mode(), fileInfo.Mode())
	}
	config, err = LoadSourceConfig(root)
	if err != nil || config.ClaudeMem.DataDir != "/data/claude-mem" || config.ClaudeMem.Enabled() {
		t.Fatalf("round trip %+v %v", config, err)
	}
}

// ASC-OAR-R1-F1: upgrading the plugin while the database schema stays at 68
// invalidates the adapter's automatic evidence and keeps user overrides.
func TestPluginUpgradeInvalidatesClaudeMemEvidence(t *testing.T) {
	s := openStores(t)
	fixture := claudememtest.New(t, claudemem.SupportedSchemaVersion, []claudememtest.Mapping{{RowID: 7, ContentID: userID, MemoryID: observerID}})
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir}}); err != nil {
		t.Fatal(err)
	}
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	s.indexSession(t, "claude", userID, "/c/user.jsonl", "user", "u", "2026-10-29T09:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "claude", observerID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)
	s.override(t, "claude", userID, "auxiliary")

	upgraded := claudememtest.ReviewedInstall()
	upgraded.Version, upgraded.Package = "15.3.0", "15.3.0"
	summary := s.reconcile(t, ReconcileOptions{Home: claudememtest.Home(t, upgraded)})
	if summary.ClaudeMem == nil || summary.ClaudeMem.Outcome != claudemem.OutcomeUnsupported || *summary.ClaudeMem.Reason != claudemem.ReasonPluginVersion || summary.ClaudeMem.ValidatedVersion != nil {
		t.Fatalf("summary %+v", summary.ClaudeMem)
	}
	for _, id := range []string{observerID, userID} {
		s.wantOrigin(t, "claude", id, ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1)
	}
	if evidence, _ := s.core.OriginEvidenceFor(context.Background(), "claude", []string{observerID}); len(evidence[observerID]) != 0 {
		t.Fatalf("evidence kept after plugin upgrade: %+v", evidence)
	}
	if record, found, _ := s.core.OriginOverrideRecord(context.Background(), "claude", userID); !found || record.Value == nil || *record.Value != "auxiliary" {
		t.Fatal("override lost after plugin upgrade")
	}
}

// ASC-OAR-R1-F1 (Round 2): a project-scope install of an unsupported version
// appearing beside the reviewed user install invalidates existing evidence
// and keeps overrides.
func TestMixedPluginInstallInvalidatesClaudeMemEvidence(t *testing.T) {
	s := openStores(t)
	fixture := claudememtest.New(t, claudemem.SupportedSchemaVersion, []claudememtest.Mapping{{RowID: 7, ContentID: userID, MemoryID: observerID}})
	if err := SaveSourceConfig(s.root, SourceConfig{ClaudeMem: AdapterConfig{DataDir: fixture.Dir}}); err != nil {
		t.Fatal(err)
	}
	s.indexSession(t, "claude", observerID, "/c/observer.jsonl", "observer", "o", "2026-10-29T10:00:00Z")
	s.indexSession(t, "claude", userID, "/c/user.jsonl", "user", "u", "2026-10-29T09:00:00Z")
	s.reconcile(t, ReconcileOptions{})
	s.wantOrigin(t, "claude", observerID, ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1)
	s.override(t, "claude", userID, "auxiliary")

	project := claudememtest.ReviewedInstall()
	project.Scope, project.ProjectPath, project.Version, project.Package = "project", t.TempDir(), "15.9.0", "15.9.0"
	summary := s.reconcile(t, ReconcileOptions{Home: claudememtest.Home(t, claudememtest.ReviewedInstall(), project)})
	if summary.ClaudeMem == nil || summary.ClaudeMem.Outcome != claudemem.OutcomeUnsupported || *summary.ClaudeMem.Reason != claudemem.ReasonPluginVersion {
		t.Fatalf("summary %+v", summary.ClaudeMem)
	}
	s.wantOrigin(t, "claude", observerID, ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1)
	if evidence, _ := s.core.OriginEvidenceFor(context.Background(), "claude", []string{observerID}); len(evidence[observerID]) != 0 {
		t.Fatalf("evidence kept beside a mixed install: %+v", evidence)
	}
	if record, found, _ := s.core.OriginOverrideRecord(context.Background(), "claude", userID); !found || record.Value == nil || *record.Value != "auxiliary" {
		t.Fatal("override lost")
	}
}
