package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

const childThread = "019a1234-abcd-7000-8000-000000000001"
const otherThread = "019a1234-abcd-7000-8000-000000000002"

func identityService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := New(db, filepath.Join(root, "home"))
	s.MachineIdentity = func(context.Context) (string, error) { return "test-machine", nil }
	return s
}
func identityExec(t *testing.T, s *Service, q string, args ...any) {
	t.Helper()
	if _, err := s.Store.DB.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
func identityCount(t *testing.T, s *Service, q string, args ...any) int {
	t.Helper()
	var n int
	if err := s.Store.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func oldIdentitySource(t *testing.T, s *Service, path, id string) {
	t.Helper()
	writeSource(t, path, `{"type":"session_meta","payload":{"id":"`+id+`","session_id":"parent"}}`)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	identityExec(t, s, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES(?,?,?,0,'')`, path, ingest.FileIdentity(info), info.Size())
	identityExec(t, s, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?,'codex','parent',?,'2026-10-01T00:00:00Z','gpt-5',10,1,?,0)`, path, path, path)
}
func identityObservation(t *testing.T, s *Service, key, thread string) {
	t.Helper()
	identityExec(t, s, `INSERT INTO telemetry_observations(observation_key,source,producer,producer_version,originator,event_name,event_kind,transport,record_at,observed_at,received_at,model,response_id,request_id,attempt_id,thread_id,source_class,input_tokens,output_tokens,metered,support_entry,state,reason) VALUES(?,'import','codex_exec','0.169.0','codex_exec','codex.sse_event','response.completed','sse','2026-10-01T00:00:00Z','','2026-10-01T00:00:00Z','gpt-5','','','',?,'',100,10,1,'codex_exec/0.169.0/codex_exec/sse','pending','ownership_not_settled')`, key, thread)
}
func identityInventory(t *testing.T, s *Service) {
	identityExec(t, s, `INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('identity',1,'2026-10-02T00:00:00Z',0,'complete')`)
}
func identitySettle(s *Service) (store.TelemetrySettleResult, error) {
	return s.Store.SettleTelemetry(context.Background(), time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), 10*time.Minute)
}

func TestIdentitySweep65OwnedSourcesAndFailedSourceRecovery(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			s := identityService(t)
			identityInventory(t, s)
			for i := 0; i < 65; i++ {
				identityObservation(t, s, fmt.Sprint(i), fmt.Sprintf("019a1234-abcd-7000-8000-%012d", i))
			}
			if r, err := identitySettle(s); err != nil || r.Owned != 65 {
				t.Fatalf("initial settle %+v %v", r, err)
			}
			var broken string
			for i := 0; i < 65; i++ {
				path := filepath.Join(s.Home, ".codex", "sessions", fmt.Sprintf("copy-%02d.jsonl", i))
				oldIdentitySource(t, s, path, fmt.Sprintf("019a1234-abcd-7000-8000-%012d", i))
				if i == 0 {
					broken = path
				}
			}
			if failure {
				s.Open = func(path string) (SourceFile, error) {
					if path == broken {
						return nil, os.ErrPermission
					}
					return os.Open(path)
				}
			}
			changed, err := s.PrepareSourceIdentities(context.Background())
			if !changed || (failure && !errors.Is(err, store.ErrTelemetrySourceIdentityPending)) || (!failure && err != nil) {
				t.Fatalf("prepare %v %v", changed, err)
			}
			want := 65
			if failure {
				want = 64
			}
			if n := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE state='jsonl_owned'`); n != want {
				t.Fatalf("retracted %d want %d", n, want)
			}
			if failure {
				// An already-owned contribution removed externally must not be restored
				// while another source is still unassessed.
				identityExec(t, s, `DELETE FROM usage_events WHERE source_path LIKE 'otel:%'`)
				if _, err := identitySettle(s); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
					t.Fatalf("settle %v", err)
				}
				if n := identityCount(t, s, `SELECT count(*) FROM usage_events WHERE source_path LIKE 'otel:%'`); n != 0 {
					t.Fatal("restored while incomplete")
				}
				s.Open = func(path string) (SourceFile, error) { return os.Open(path) }
				if _, err = s.PrepareSourceIdentities(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := identitySettle(s); err != nil {
				t.Fatal(err)
			}
			if n := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE state='jsonl_owned'`); n != 65 {
				t.Fatalf("owned %d", n)
			}
			if n := identityCount(t, s, `SELECT count(*) FROM telemetry_contributions`); n != 0 {
				t.Fatalf("contributions %d", n)
			}
		})
	}
}

func TestIdentitySweepCancellationKeepsCommittedBatch(t *testing.T) {
	s := identityService(t)
	for i := 0; i < 65; i++ {
		oldIdentitySource(t, s, filepath.Join(s.Home, ".codex", "sessions", fmt.Sprintf("copy-%02d.jsonl", i)), childThread)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads := 0
	s.Open = func(path string) (SourceFile, error) {
		reads++
		if reads == 65 {
			cancel()
		}
		return os.Open(path)
	}
	if _, err := s.PrepareSourceIdentities(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM usage_source_files WHERE thread_id IS NOT NULL`); n != 64 {
		t.Fatalf("durable batch %d", n)
	}
	reads = 0
	s.Open = func(path string) (SourceFile, error) { reads++; return os.Open(path) }
	if _, err := s.PrepareSourceIdentities(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reads != 1 {
		t.Fatalf("restart reread %d sources", reads)
	}
}

func TestIdentityHeadAssessmentAndChangedRange(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		decided          bool
	}{
		{"first", `{"type":"session_meta","payload":{"id":"` + childThread + `"}}` + "\n" + `{"type":"session_meta","payload":{"id":"` + otherThread + `"}}` + "\n", childThread, true},
		{"accounting only", `{"type":"session_meta","payload":{"session_id":"` + childThread + `"}}` + "\n", "", true},
		{"partial", `{"type":"event_msg"}` + "\n" + `{"type":"session_meta"`, "", false},
		{"complete no identity", `{"type":"event_msg"}` + "\n", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := firstSessionThread([]byte(tt.body), true)
			if id != tt.want || ok != tt.decided {
				t.Fatalf("%q %v", id, ok)
			}
		})
	}
	s := identityService(t)
	path := filepath.Join(s.Home, ".codex", "sessions", "copy.jsonl")
	oldIdentitySource(t, s, path, childThread)
	original := s.Stat
	calls := 0
	s.Stat = func(path string) (os.FileInfo, error) {
		calls++
		if calls == 2 {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, []byte(strings.ReplaceAll(string(data), childThread, otherThread)), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return original(path)
	}
	if _, err := s.PrepareSourceIdentities(context.Background()); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("changed source %v", err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM usage_source_files WHERE thread_id IS NOT NULL`); n != 0 {
		t.Fatal("saved changed head")
	}
	s.Stat = original
	writeSource(t, path, strings.Repeat("x", sourceIdentityHeadBytes+1))
	if _, err := s.PrepareSourceIdentities(context.Background()); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("oversize %v", err)
	}
}

func TestIdentityMetadataAppendRewriteAndAtomicPublications(t *testing.T) {
	for _, mode := range []string{"cold", "normal", "prefix"} {
		t.Run(mode, func(t *testing.T) {
			s := identityService(t)
			identityInventory(t, s)
			identityObservation(t, s, "first", childThread)
			if _, err := identitySettle(s); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.Home, ".codex", "sessions", "renamed.jsonl")
			lines := []string{`{"type":"session_meta","payload":{"id":"` + childThread + `","session_id":"parent"}}`, `{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5"}}`}
			if mode == "normal" {
				writeSource(t, path, lines...)
				if _, err := s.Scan(context.Background()); err != nil {
					t.Fatal(err)
				}
				identityObservation(t, s, "later", childThread)
			}
			n := 1
			if mode == "prefix" {
				n = 513
			}
			for i := 0; i < n; i++ {
				lines = append(lines, fmt.Sprintf(`{"timestamp":"2026-10-01T%02d:%02d:%02dZ","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`, i/3600, i/60%60, i%60))
			}
			writeSource(t, path, lines...)
			// A trigger observes the same transaction as every JSONL insertion.
			identityExec(t, s, `CREATE TRIGGER no_duplicate BEFORE INSERT ON usage_events WHEN NEW.source_path NOT LIKE 'otel:%' AND EXISTS(SELECT 1 FROM telemetry_observations WHERE thread_id='`+childThread+`' AND state IN ('pending','otel_owned')) BEGIN SELECT RAISE(ABORT,'OTel not retracted atomically'); END`)
			if mode == "prefix" {
				identityExec(t, s, `CREATE TRIGGER reject_identity BEFORE INSERT ON usage_source_files BEGIN SELECT RAISE(ABORT,'crash before checkpoint'); END`)
			}
			_, err := s.Scan(context.Background())
			if mode == "prefix" {
				if err == nil {
					t.Fatal("expected checkpoint failure")
				}
				if got := identityCount(t, s, `SELECT count(*) FROM usage_events WHERE source_path=?`, path); got != 512 {
					t.Fatalf("prefix %d (scan error: %v)", got, err)
				}
				identityObservation(t, s, "after-prefix", childThread)
				identityObservation(t, s, "unrelated", otherThread)
				if _, err := identitySettle(s); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
					t.Fatalf("prefix settle %v", err)
				}
				if got := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE thread_id=? AND state='jsonl_owned'`, childThread); got != 2 {
					t.Fatalf("sticky %d", got)
				}
				identityExec(t, s, `DROP TRIGGER reject_identity`)
				if _, err = s.Scan(context.Background()); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var id sql.NullString
			if err := s.Store.DB.QueryRow(`SELECT thread_id FROM usage_source_files WHERE path=?`, path).Scan(&id); err != nil || id.String != childThread {
				t.Fatalf("identity %v %v", id, err)
			}
			// A later inherited session_meta cannot replace the source's identity.
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintln(file, `{"type":"session_meta","payload":{"id":"`+otherThread+`"}}`)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Scan(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err = s.Store.DB.QueryRow(`SELECT thread_id FROM usage_source_files WHERE path=?`, path).Scan(&id); err != nil || id.String != childThread {
				t.Fatalf("append identity %v %v", id, err)
			}
			writeSource(t, path, `{"type":"session_meta","payload":{"id":"`+otherThread+`"}}`)
			if _, err = s.Scan(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err = s.Store.DB.QueryRow(`SELECT thread_id FROM usage_source_files WHERE path=?`, path).Scan(&id); err != nil || id.String != otherThread {
				t.Fatalf("rewrite identity %v %v", id, err)
			}
		})
	}
}

func TestIdentityMissingCanonicalClaudeAndOrphanCleanup(t *testing.T) {
	s := identityService(t)
	ctx := context.Background()
	identityInventory(t, s)
	canonical := filepath.Join(s.Home, ".codex", "sessions", "rollout-2026-10-01T12-00-00-"+childThread+"_"+otherThread+".jsonl")
	oldIdentitySource(t, s, canonical, childThread)
	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(s.Home, ".claude", "projects", "p", "ordinary.jsonl")
	identityExec(t, s, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES(?,'missing',0,0,'')`, claude)
	identityObservation(t, s, "child", childThread)
	if _, err := s.PrepareSourceIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM usage_source_files WHERE path=? AND thread_id=''`, claude); n != 1 {
		t.Fatal("Claude blocked readiness")
	}
	if n := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE state='jsonl_owned'`); n != 1 {
		t.Fatal("missing canonical not protected")
	}
	// An orphan prefix has no checkpoint and blocks unrelated promotion until
	// normal cleanup. Sticky ownership remains after its source is gone.
	identityExec(t, s, `DELETE FROM usage_source_files WHERE path=?`, canonical)
	identityObservation(t, s, "unrelated", otherThread)
	if _, err := identitySettle(s); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("orphan %v", err)
	}
	if _, err := s.cleanupOrphanedEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if r, err := identitySettle(s); err != nil || r.Owned != 1 {
		t.Fatalf("cleanup %+v %v", r, err)
	}
	identityObservation(t, s, "sticky", childThread)
	if _, err := identitySettle(s); err != nil {
		t.Fatal(err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE thread_id=? AND state='jsonl_owned'`, childThread); n != 2 {
		t.Fatal("lost sticky ownership")
	}
}

func TestIdentityConcurrentCheckpointWinsAndOldAppendReadsOriginalHead(t *testing.T) {
	s := identityService(t)
	ctx := context.Background()
	path := filepath.Join(s.Home, ".codex", "sessions", "copy.jsonl")
	writeSource(t, path, `{"type":"session_meta","payload":{"id":"`+childThread+`","session_id":"parent"}}`, `{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5"}}`)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	identityExec(t, s, `UPDATE usage_source_files SET thread_id=NULL WHERE path=?`, path)
	var identity string
	if err := s.Store.DB.QueryRow(`SELECT identity FROM usage_source_files WHERE path=?`, path).Scan(&identity); err != nil {
		t.Fatal(err)
	}
	identityInventory(t, s)
	identityObservation(t, s, "late", childThread)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintln(f, `{"timestamp":"2026-10-01T00:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM telemetry_observations WHERE state='jsonl_owned'`); n != 1 {
		t.Fatal("append did not retract old source identity")
	}
	if _, err := s.Store.SaveSourceThreadIdentities(ctx, []store.SourceThreadIdentity{{Path: path, Identity: identity, ThreadID: otherThread}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM usage_source_files WHERE path=? AND thread_id=?`, path, childThread); n != 1 {
		t.Fatal("stale assessment replaced committed identity")
	}
}

func TestHistoricalReadmissionUsesOriginalDayAndCost(t *testing.T) {
	s := identityService(t)
	ctx := context.Background()
	identityInventory(t, s)
	if err := s.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	identityObservation(t, s, "historical", childThread)
	identityExec(t, s, `UPDATE telemetry_observations SET state='capture_only',reason='producer_unsupported',support_entry='',model='gpt-5.4',input_tokens=1000000,output_tokens=0 WHERE observation_key='historical'`)
	if r, err := identitySettle(s); err != nil || r.Owned != 1 {
		t.Fatalf("readmission %+v %v", r, err)
	}
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	summary, err := s.SummaryRange(ctx, day, day.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "2.500000000" {
		t.Fatalf("historical cost %+v", summary)
	}
	if n := identityCount(t, s, `SELECT count(*) FROM usage_events WHERE event_at='2026-10-01T00:00:00Z' AND input_tokens=1000000 AND output_tokens=0`); n != 1 {
		t.Fatal("changed original date or counts")
	}
	if r, err := identitySettle(s); err != nil || r.Changed {
		t.Fatalf("repeated %+v %v", r, err)
	}
	again, err := s.SummaryRange(ctx, day, day.AddDate(0, 0, 1))
	if err != nil || again.CatalogBaseCost == nil || *again.CatalogBaseCost != *summary.CatalogBaseCost {
		t.Fatalf("repeated cost %+v %v", again, err)
	}
}
