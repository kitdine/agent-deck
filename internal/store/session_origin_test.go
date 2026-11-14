package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func originTestStore(t *testing.T) (string, *Store) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	core, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	return root, core
}

func setOriginFault(t *testing.T, fault func(string) error) {
	t.Helper()
	originMutationFault = fault
	t.Cleanup(func() { originMutationFault = nil })
}

func servingRevision(t *testing.T, s *Store) int64 {
	t.Helper()
	var revision int64
	if err := s.DB.QueryRow(`SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func clearMutation(id string, expected int64) OriginMutation {
	return OriginMutation{RequestID: id, PayloadHash: "hash-" + id, Kind: OriginMutationOverrideClear, Client: "codex", SessionID: "archived-1", ExpectedRevision: expected, Now: time.Unix(1, 0)}
}

func setMutation(id, value, digest string, expected int64) OriginMutation {
	return OriginMutation{RequestID: id, PayloadHash: "hash-" + id, Kind: OriginMutationOverrideSet, Client: "codex", SessionID: "archived-1", Value: value, Binding: OriginBinding{Digest: digest}, ExpectedRevision: expected, Now: time.Unix(1, 0)}
}

func overrideRow(t *testing.T, s *Store) (OriginOverride, bool) {
	t.Helper()
	row, found, err := s.OriginOverrideRecord(context.Background(), "codex", "archived-1")
	if err != nil {
		t.Fatal(err)
	}
	return row, found
}

func TestOriginReadsDistinguishMissingConfigurationFromReadFailure(t *testing.T) {
	ctx := context.Background()
	root, core := originTestStore(t)
	preference, err := core.OriginPreference(ctx)
	if err != nil || preference.Present {
		t.Fatalf("fresh preference = %+v, %v; want successful absent read", preference, err)
	}
	failing := closedOriginStore(t, root)
	if _, err = failing.OriginPreference(ctx); err == nil {
		t.Fatal("failed core read reported an absent preference instead of an error")
	}
	if _, _, err = failing.OriginOverrideRecord(ctx, "codex", "x"); err == nil {
		t.Fatal("failed core override read did not fail")
	}
	if _, _, err = failing.OriginOverrides(ctx, 0, 20); err == nil {
		t.Fatal("failed core override list did not fail")
	}
}

// closedOriginStore is a core handle whose every query fails.
func closedOriginStore(t *testing.T, root string) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(root, "agentdeck.sqlite3")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	return &Store{DB: db, readOnly: true}
}

func TestOriginReadOnlyPathsNeverCreateOrMigrate(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	if _, err := OpenReadOnly(ctx, root); err == nil {
		t.Fatal("read-only open created a missing core")
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only open created the state root: %v", err)
	}
	// A core that predates migration 34 is read as having no origin facts.
	core, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	downgradeToSchema33(t, core)
	if err = core.Close(); err != nil {
		t.Fatal(err)
	}
	readOnly, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	preference, err := readOnly.OriginPreference(ctx)
	if err != nil || preference.Present {
		t.Fatalf("schema-33 preference = %+v, %v", preference, err)
	}
	items, total, err := readOnly.OriginOverrides(ctx, 0, 20)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("schema-33 overrides = %v %d %v", items, total, err)
	}
	if version, err := readOnly.SchemaVersion(ctx); err != nil || version != 33 {
		t.Fatalf("read-only path migrated: version=%d err=%v", version, err)
	}
	if _, err = readOnly.ApplyOriginMutation(ctx, clearMutation("ro", 0)); !errors.Is(err, ErrDerivedSnapshotGenerationReadOnly) {
		t.Fatalf("read-only mutation err = %v", err)
	}
}

func downgradeToSchema33(t *testing.T, core *Store) {
	t.Helper()
	for _, op := range []string{"insert", "update", "delete"} {
		if _, err := core.DB.Exec("DROP TRIGGER derived_snapshot_generation_telemetry_sessions_" + op); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.DB.Exec(`DROP TABLE origin_preferences; DROP TABLE origin_overrides; DROP TABLE origin_mutations; DROP TABLE session_origins; DROP TABLE origin_evidence; DROP TABLE origin_adapter_runs; UPDATE schema_metadata SET version=33`); err != nil {
		t.Fatal(err)
	}
}

// The seven-scenario clear chain of architecture §9, against core only.
func TestOriginClearChain(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	if _, err := core.DB.Exec(`INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES('codex','archived-1','birth',0,'unmatched','auxiliary',4,'t')`); err != nil {
		t.Fatal(err)
	}
	items, total, err := core.OriginOverrides(ctx, 0, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].BindingState != "unmatched" {
		t.Fatalf("unmatched override not listed: %+v %d %v", items, total, err)
	}

	// expected4/requestA writes tombstone5.
	before := servingRevision(t, core)
	receipt, err := core.ApplyOriginMutation(ctx, clearMutation("requestA", 4))
	if err != nil || receipt.Outcome != OriginOutcomeSaved || receipt.Revision != 5 || receipt.Value != nil {
		t.Fatalf("clear = %+v, %v", receipt, err)
	}
	if servingRevision(t, core) <= before {
		t.Fatal("saved clear did not advance the serving revision")
	}
	row, found := overrideRow(t, core)
	if !found || row.Value != nil || row.Revision != 5 || row.BindingDigest != "birth" {
		t.Fatalf("tombstone = %+v found=%t", row, found)
	}
	if _, total, _ = core.OriginOverrides(ctx, 0, 20); total != 0 {
		t.Fatal("tombstone listed as an effective override")
	}

	// tombstone5, new requestB/expected5: no_change, no new version or publication.
	before = servingRevision(t, core)
	receipt, err = core.ApplyOriginMutation(ctx, clearMutation("requestB", 5))
	if err != nil || receipt.Outcome != OriginOutcomeNoChange || receipt.Revision != 5 {
		t.Fatalf("tombstone clear = %+v, %v", receipt, err)
	}
	if servingRevision(t, core) != before {
		t.Fatal("no_change advanced the serving revision")
	}

	// expected4 against tombstone5 still conflicts: CAS precedes no-change.
	var conflict *OriginConflictError
	if _, err = core.ApplyOriginMutation(ctx, clearMutation("requestStale", 4)); !errors.As(err, &conflict) || conflict.Current != 5 {
		t.Fatalf("stale clear err = %v", err)
	}

	// never-present key, expected0: no_change/0 without creating any row.
	never := clearMutation("requestNever", 0)
	never.SessionID = "never-present"
	receipt, err = core.ApplyOriginMutation(ctx, never)
	if err != nil || receipt.Outcome != OriginOutcomeNoChange || receipt.Revision != 0 {
		t.Fatalf("never-present clear = %+v, %v", receipt, err)
	}
	if _, found, _ = core.OriginOverrideRecord(ctx, "codex", "never-present"); found {
		t.Fatal("never-present clear created an override row")
	}

	// Someone sets revision6; replaying requestA returns its own receipt only.
	if _, err = core.ApplyOriginMutation(ctx, setMutation("requestSet", "work", "birth2", 5)); err != nil {
		t.Fatal(err)
	}
	receipt, err = core.ApplyOriginMutation(ctx, clearMutation("requestA", 4))
	if err != nil || !receipt.Replayed || receipt.Revision != 5 || receipt.Outcome != OriginOutcomeSaved {
		t.Fatalf("replay = %+v, %v", receipt, err)
	}
	if row, _ = overrideRow(t, core); row.Value == nil || *row.Value != "work" || row.Revision != 6 {
		t.Fatalf("replay changed the newer override: %+v", row)
	}
	if receipt, err = core.ApplyOriginMutation(ctx, clearMutation("requestC", 6)); err != nil || receipt.Revision != 7 {
		t.Fatalf("new request against revision6 = %+v, %v", receipt, err)
	}

	// The same request ID with a different payload is refused, not redone.
	mismatch := clearMutation("requestA", 4)
	mismatch.PayloadHash = "other"
	if _, err = core.ApplyOriginMutation(ctx, mismatch); !errors.Is(err, ErrOriginRequestMismatch) {
		t.Fatalf("payload mismatch err = %v", err)
	}
}

func TestOriginCommitBoundaryFailures(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	if _, err := core.ApplyOriginMutation(ctx, setMutation("seed", "auxiliary", "birth", 0)); err != nil {
		t.Fatal(err)
	}

	// Failure before commit: nothing persists, no receipt, no visible revision.
	before := servingRevision(t, core)
	setOriginFault(t, func(stage string) error {
		if stage == "before_commit" {
			return errors.New("injected")
		}
		return nil
	})
	if _, err := core.ApplyOriginMutation(ctx, clearMutation("lost", 1)); err == nil {
		t.Fatal("pre-commit failure reported success")
	} else if errors.As(err, new(*OriginCommitUnknownError)) {
		t.Fatalf("pre-commit failure reported as unknown: %v", err)
	}
	if _, found, _ := core.OriginReceipt(ctx, "lost"); found {
		t.Fatal("rolled-back mutation left a receipt")
	}
	if row, _ := overrideRow(t, core); row.Value == nil || row.Revision != 1 {
		t.Fatalf("rolled-back clear changed the override: %+v", row)
	}
	if servingRevision(t, core) != before {
		t.Fatal("rolled-back mutation advanced the serving revision")
	}

	// Acknowledgement lost after COMMIT: unknown, and the receipt tells the truth.
	setOriginFault(t, func(stage string) error {
		if stage == "commit" {
			return errors.New("connection lost")
		}
		return nil
	})
	_, err := core.ApplyOriginMutation(ctx, clearMutation("ack-lost", 1))
	var unknown *OriginCommitUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("commit-boundary failure err = %v", err)
	}
	originMutationFault = nil
	receipt, found, err := core.OriginReceipt(ctx, "ack-lost")
	if err != nil || !found || receipt.Outcome != OriginOutcomeSaved || receipt.Revision != 2 {
		t.Fatalf("read-back = %+v found=%t err=%v", receipt, found, err)
	}
	// A retry with the same ID and payload replays, it does not clear twice.
	again, err := core.ApplyOriginMutation(ctx, clearMutation("ack-lost", 1))
	if err != nil || !again.Replayed || again.Revision != 2 {
		t.Fatalf("retry = %+v, %v", again, err)
	}
}

func TestOriginSecureFilesFailureAfterCommitIsSaved(t *testing.T) {
	ctx := context.Background()
	root, core := originTestStore(t)
	// Make the post-commit chmod fail by pointing secureFiles at a missing
	// directory entry that cannot be chmodded: a path under a regular file.
	saved := core.path
	core.path = filepath.Join(root, "agentdeck.sqlite3", "not-a-dir")
	t.Cleanup(func() { core.path = saved })
	receipt, err := core.ApplyOriginMutation(ctx, setMutation("warn", "work", "birth", 0))
	if !errors.Is(err, ErrOriginSecureFilesFailed) || receipt.Outcome != OriginOutcomeSaved || receipt.Revision != 1 {
		t.Fatalf("receipt=%+v err=%v; want saved with permission warning", receipt, err)
	}
	core.path = saved
	if row, found := overrideRow(t, core); !found || row.Revision != 1 {
		t.Fatalf("committed override missing after permission warning: %+v", row)
	}
}

func TestOriginSetCASBindingAndNoChange(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	if r, err := core.ApplyOriginMutation(ctx, setMutation("s1", "auxiliary", "birth", 0)); err != nil || r.Revision != 1 || r.Outcome != OriginOutcomeSaved {
		t.Fatalf("first set = %+v, %v", r, err)
	}
	// A new request with a stale revision conflicts even when the value matches.
	var conflict *OriginConflictError
	if _, err := core.ApplyOriginMutation(ctx, setMutation("s2", "auxiliary", "birth", 0)); !errors.As(err, &conflict) {
		t.Fatalf("stale same-value set err = %v", err)
	}
	before := servingRevision(t, core)
	if r, err := core.ApplyOriginMutation(ctx, setMutation("s3", "auxiliary", "birth", 1)); err != nil || r.Outcome != OriginOutcomeNoChange || r.Revision != 1 {
		t.Fatalf("same value and binding = %+v, %v", r, err)
	}
	if servingRevision(t, core) != before {
		t.Fatal("no_change set advanced the serving revision")
	}
	// The same value with another binding is a change.
	if r, err := core.ApplyOriginMutation(ctx, setMutation("s4", "auxiliary", "birth-other", 1)); err != nil || r.Outcome != OriginOutcomeSaved || r.Revision != 2 {
		t.Fatalf("rebinding set = %+v, %v", r, err)
	}
	// An unmatched row is rebound by an explicit set of the same value.
	if _, err := core.DB.Exec(`UPDATE origin_overrides SET binding_state='unmatched'`); err != nil {
		t.Fatal(err)
	}
	if r, err := core.ApplyOriginMutation(ctx, setMutation("s5", "auxiliary", "birth-other", 2)); err != nil || r.Outcome != OriginOutcomeSaved || r.Revision != 3 {
		t.Fatalf("unmatched rebind = %+v, %v", r, err)
	}
	if row, _ := overrideRow(t, core); row.BindingState != "matched" {
		t.Fatalf("rebind left state %q", row.BindingState)
	}
	invalid := setMutation("s6", "unknown", "birth", 3)
	if _, err := core.ApplyOriginMutation(ctx, invalid); err == nil {
		t.Fatal("set accepted a non-override value")
	}
}

func TestOriginScopePreferenceCAS(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	scope := func(id, value string, expected int64) (OriginReceipt, error) {
		return core.ApplyOriginMutation(ctx, OriginMutation{RequestID: id, PayloadHash: "h-" + id, Kind: OriginMutationScopeSet, Value: value, ExpectedRevision: expected, Now: time.Unix(2, 0)})
	}
	unchanged := func(label string, wantServing int64) {
		t.Helper()
		var rows, dirty int
		if err := core.DB.QueryRow(`SELECT count(*) FROM origin_preferences`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if err := core.DB.QueryRow(`SELECT dirty FROM derived_snapshot_generation WHERE singleton=1`).Scan(&dirty); err != nil {
			t.Fatal(err)
		}
		if rows != 0 || dirty != 0 || servingRevision(t, core) != wantServing {
			t.Fatalf("%s: preference rows=%d dirty=%d serving=%d, want 0/0/%d", label, rows, dirty, servingRevision(t, core), wantServing)
		}
	}
	if _, err := core.DB.Exec(`UPDATE derived_snapshot_generation SET dirty=0`); err != nil {
		t.Fatal(err)
	}
	serving := servingRevision(t, core)

	// A missing preference row is the confirmed default Work at revision 0:
	// saving Work is a no_change that only records its receipt.
	if r, err := scope("p1", "work", 0); err != nil || r.Outcome != OriginOutcomeNoChange || r.Revision != 0 || r.Replayed {
		t.Fatalf("first work over missing config = %+v, %v", r, err)
	}
	unchanged("first work", serving)
	var receipts int
	if err := core.DB.QueryRow(`SELECT count(*) FROM origin_mutations WHERE request_id='p1' AND outcome='no_change' AND revision=0`).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("first work receipt count = %d, %v", receipts, err)
	}
	// Same-ID replay returns the stored receipt.
	if r, err := scope("p1", "work", 0); err != nil || !r.Replayed || r.Outcome != OriginOutcomeNoChange || r.Revision != 0 {
		t.Fatalf("replayed first work = %+v, %v", r, err)
	}
	// A different ID with the same value is also a no_change.
	if r, err := scope("p2", "work", 0); err != nil || r.Replayed || r.Outcome != OriginOutcomeNoChange || r.Revision != 0 {
		t.Fatalf("second same-value work = %+v, %v", r, err)
	}
	unchanged("second work", serving)
	// CAS precedes the same-value check: a stale revision conflicts even for Work.
	var conflict *OriginConflictError
	if _, err := scope("p3", "work", 1); !errors.As(err, &conflict) || conflict.Current != 0 {
		t.Fatalf("stale same-value work err = %v", err)
	}
	unchanged("stale work", serving)

	if r, err := scope("p4", "auxiliary", 0); err != nil || r.Outcome != OriginOutcomeSaved || r.Revision != 1 {
		t.Fatalf("scope change = %+v, %v", r, err)
	}
	if servingRevision(t, core) != serving+1 {
		t.Fatal("scope change did not advance the serving revision exactly once")
	}
	if r, err := scope("p5", "auxiliary", 1); err != nil || r.Outcome != OriginOutcomeNoChange || r.Revision != 1 {
		t.Fatalf("same scope = %+v, %v", r, err)
	}
	if _, err := scope("p6", "all", 0); !errors.As(err, &conflict) {
		t.Fatalf("stale scope err = %v", err)
	}
	if r, err := scope("p7", "work", 1); err != nil || r.Outcome != OriginOutcomeSaved || r.Revision != 2 {
		t.Fatalf("explicit return to work = %+v, %v", r, err)
	}
	preference, err := core.OriginPreference(ctx)
	if err != nil || !preference.Present || preference.Scope != "work" || preference.Revision != 2 {
		t.Fatalf("preference = %+v, %v", preference, err)
	}
}

func TestOriginTablesAdvanceServingRevisionAndInvalidateSessions(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	writes := []struct {
		name, statement string
		keyed           bool
	}{
		{"preferences", `INSERT INTO origin_preferences(singleton,scope,revision,updated_at) VALUES(1,'all',1,'t')`, false},
		{"overrides", `INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES('codex','k-over','d',0,'matched','work',1,'t')`, true},
		{"origins", `INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES('codex','k-origin','unknown','origin_metadata_missing','','tok',0)`, true},
		{"evidence", `INSERT INTO origin_evidence(adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at) VALUES('a','r','claude','k-evidence','t','auxiliary','v','t')`, true},
		{"telemetry", `INSERT INTO telemetry_sessions(client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at) VALUES('codex','k-otel','codex-otel-background','m','a','b','e','s')`, true},
		{"telemetry delete", `DELETE FROM telemetry_sessions WHERE session_id='k-otel'`, true},
		{"runs", `INSERT INTO origin_adapter_runs(adapter,outcome,finished_at) VALUES('claude-mem','ok','t')`, false},
	}
	for _, write := range writes {
		if _, err := core.DB.Exec(`DELETE FROM usage_derived_invalidations; UPDATE derived_snapshot_generation SET dirty=0`); err != nil {
			t.Fatal(err)
		}
		before := servingRevision(t, core)
		if _, err := core.DB.Exec(write.statement); err != nil {
			t.Fatalf("%s: %v", write.name, err)
		}
		if servingRevision(t, core) != before+1 {
			t.Fatalf("%s did not advance serving revision exactly once", write.name)
		}
		generation, err := core.DerivedSnapshotGeneration(ctx)
		if err != nil || !generation.Dirty {
			t.Fatalf("%s did not dirty the derived generation: %+v %v", write.name, generation, err)
		}
		var invalidations int
		if err := core.DB.QueryRow(`SELECT count(*) FROM usage_derived_invalidations WHERE kind='session'`).Scan(&invalidations); err != nil {
			t.Fatal(err)
		}
		if write.keyed != (invalidations == 1) {
			t.Fatalf("%s session invalidations = %d, keyed=%t", write.name, invalidations, write.keyed)
		}
	}
	// Receipts alone never publish.
	before := servingRevision(t, core)
	if _, err := core.DB.Exec(`INSERT INTO origin_mutations(request_id,payload_hash,kind,target_client,target_session_id,outcome,revision,recorded_at) VALUES('r','h','scope_set','','','no_change',0,'t')`); err != nil {
		t.Fatal(err)
	}
	if servingRevision(t, core) != before {
		t.Fatal("receipt insert advanced the serving revision")
	}
}

func TestOriginConcurrentCASAdmitsOneWriter(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	const writers = 8
	var wg sync.WaitGroup
	results := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = core.ApplyOriginMutation(ctx, setMutation(fmt.Sprintf("race-%d", i), "auxiliary", fmt.Sprintf("birth-%d", i), 0))
		}(i)
	}
	wg.Wait()
	saved := 0
	for _, err := range results {
		switch {
		case err == nil:
			saved++
		case errors.As(err, new(*OriginConflictError)):
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if row, _ := overrideRow(t, core); saved != 1 || row.Revision != 1 {
		t.Fatalf("saved=%d revision=%d; want exactly one CAS winner", saved, row.Revision)
	}
}

func TestOriginWritesKeepPrivatePermissions(t *testing.T) {
	ctx := context.Background()
	root, core := originTestStore(t)
	if _, err := core.ApplyOriginMutation(ctx, setMutation("perm", "work", "birth", 0)); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(root); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("state root mode = %v, %v", info.Mode().Perm(), err)
	}
	for _, name := range []string{"agentdeck.sqlite3", "agentdeck.sqlite3-wal", "agentdeck.sqlite3-shm"} {
		info, err := os.Stat(filepath.Join(root, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, %v", name, info.Mode().Perm(), err)
		}
	}
}

func TestOriginMigrationFailureRollsBackAndSchemaAheadRefuses(t *testing.T) {
	ctx := context.Background()
	root, core := originTestStore(t)
	downgradeToSchema33(t, core)
	if _, err := core.DB.Exec(`CREATE TABLE origin_preferences (unexpected TEXT)`); err != nil {
		t.Fatal(err)
	}
	before := servingRevision(t, core)
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, root); err == nil {
		t.Fatal("migration 34 succeeded over a conflicting table")
	}
	readOnly, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := readOnly.SchemaVersion(ctx)
	var overrides int
	_ = readOnly.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='origin_overrides'`).Scan(&overrides)
	if version != 33 || overrides != 0 || servingRevision(t, readOnly) != before {
		t.Fatalf("failed migration left version=%d overrides=%d", version, overrides)
	}
	readOnly.Close()

	// Remove the obstacle: the same migration now applies, then schema-ahead refuses.
	raw, err := sql.Open("sqlite", "file:"+filepath.Join(root, "agentdeck.sqlite3")+"?mode=rw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec(`DROP TABLE origin_preferences`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	migrated, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if version, _ = migrated.SchemaVersion(ctx); version != CurrentSchemaVersion {
		t.Fatalf("migrated version = %d", version)
	}
	if _, err = migrated.DB.Exec(`UPDATE schema_metadata SET version=?`, CurrentSchemaVersion+1); err != nil {
		t.Fatal(err)
	}
	migrated.Close()
	if _, err = Open(ctx, root); !errors.Is(err, ErrSchemaAhead) {
		t.Fatalf("schema-ahead open err = %v", err)
	}
	if _, err = OpenReadOnly(ctx, root); !errors.Is(err, ErrSchemaAhead) {
		t.Fatalf("schema-ahead read-only err = %v", err)
	}
}

func TestOriginDerivationIsAtomicWithRunSummary(t *testing.T) {
	ctx := context.Background()
	_, core := originTestStore(t)
	reason := "source_unavailable"
	derivation := OriginDerivation{
		Adapter: "claude-mem", Client: "claude", SessionIDs: []string{"s1", "s2"},
		Evidence: []OriginEvidence{{Adapter: "claude-mem", RuleVersion: "claude-mem-sdk-map-v1", Client: "claude", SessionID: "s1", SourceToken: "row-1", Purpose: "auxiliary", ValidatedVersion: "15.2.1", ObservedAt: "t"}},
		Origins: []SessionOrigin{
			{Client: "claude", SessionID: "s1", Automatic: "auxiliary", ReasonCode: "maintenance_origin", RuleVersion: "claude-mem-sdk-map-v1", InputToken: "tok1", EvidenceRevision: 1},
			{Client: "claude", SessionID: "s2", Automatic: "unknown", ReasonCode: "origin_metadata_missing", InputToken: "tok2"},
		},
		Run: &OriginAdapterRun{Adapter: "claude-mem", Outcome: "partial", Reason: &reason, FinishedAt: "t1"},
	}
	setOriginFault(t, func(stage string) error {
		if stage == "derivation_before_commit" {
			return errors.New("injected")
		}
		return nil
	})
	if err := core.ReplaceOriginDerivation(ctx, derivation); err == nil {
		t.Fatal("injected derivation failure succeeded")
	}
	if _, found, _ := core.OriginAdapterRunFor(ctx, "claude-mem"); found {
		t.Fatal("failed derivation left a run summary")
	}
	if origins, _ := core.SessionOrigins(ctx, "claude", []string{"s1", "s2"}); len(origins) != 0 {
		t.Fatalf("failed derivation left origins: %v", origins)
	}
	originMutationFault = nil
	if err := core.ReplaceOriginDerivation(ctx, derivation); err != nil {
		t.Fatal(err)
	}
	run, found, err := core.OriginAdapterRunFor(ctx, "claude-mem")
	if err != nil || !found || run.Outcome != "partial" || run.Reason == nil || *run.Reason != reason || run.ValidatedVersion != nil {
		t.Fatalf("run = %+v found=%t err=%v", run, found, err)
	}
	origins, err := core.SessionOrigins(ctx, "claude", []string{"s1", "s2", "absent"})
	if err != nil || len(origins) != 2 || origins["s1"].Automatic != "auxiliary" {
		t.Fatalf("origins = %v, %v", origins, err)
	}
	evidence, err := core.OriginEvidenceFor(ctx, "claude", []string{"s1"})
	if err != nil || len(evidence["s1"]) != 1 {
		t.Fatalf("evidence = %v, %v", evidence, err)
	}
	// Replacing a candidate set without evidence withdraws it.
	if err = core.ReplaceOriginDerivation(ctx, OriginDerivation{Adapter: "claude-mem", Client: "claude", SessionIDs: []string{"s1"}}); err != nil {
		t.Fatal(err)
	}
	if evidence, _ = core.OriginEvidenceFor(ctx, "claude", []string{"s1"}); len(evidence) != 0 {
		t.Fatalf("withdrawn evidence remains: %v", evidence)
	}
	if run, _, _ = core.OriginAdapterRunFor(ctx, "claude-mem"); run.Outcome != "partial" {
		t.Fatal("derivation without a run replaced the summary")
	}
	bad := OriginDerivation{Adapter: "claude-mem", Client: "claude", Origins: []SessionOrigin{{Client: "claude", SessionID: "x", Automatic: "unknown", ReasonCode: "invented"}}}
	if err = core.ReplaceOriginDerivation(ctx, bad); err == nil {
		t.Fatal("reason code outside the closed enum was stored")
	}
}

func TestTelemetrySessionOriginMarkersBatchesAndScope(t *testing.T) {
	ctx := context.Background()
	root, core := originTestStore(t)
	insert := func(client, id, kind, entry string) {
		t.Helper()
		if _, err := core.DB.Exec(`INSERT INTO telemetry_sessions(client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at) VALUES(?,?,?,?,?,?,?,?)`,
			client, id, kind, "m", "a", "b", entry, "settled-"+id); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	for i := 0; i < 201; i++ {
		id := fmt.Sprintf("bg-%03d", i)
		ids = append(ids, id)
		insert("codex", id, TelemetrySessionSourceKind, "codex_exec/0.165.0/codex_exec/sse")
	}
	insert("claude", "shared-id", TelemetrySessionSourceKind, "e")
	insert("codex", "other-kind", "other", "e")

	readOnly, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	for _, size := range []int{0, 1, 200, 201} {
		markers, err := readOnly.TelemetrySessionOriginMarkers(ctx, "codex", ids[:size])
		if err != nil || len(markers) != size {
			t.Fatalf("size %d: %d markers, %v", size, len(markers), err)
		}
	}
	marker, err := readOnly.TelemetrySessionOriginMarkers(ctx, "codex", []string{"bg-200", "unmarked", "shared-id"})
	if err != nil || len(marker) != 1 {
		t.Fatalf("unmarked or cross-client ID returned: %v %v", marker, err)
	}
	if got := marker["bg-200"]; got.SourceKind != TelemetrySessionSourceKind || got.SupportEntry != "codex_exec/0.165.0/codex_exec/sse" || got.SettledAt != "settled-bg-200" {
		t.Fatalf("marker = %+v", got)
	}
	count, err := readOnly.TelemetryBackgroundSessionCount(ctx)
	if err != nil || count != 202 {
		t.Fatalf("background count = %d, %v; want only codex-otel-background rows", count, err)
	}
	// An empty candidate list issues no query, so it succeeds on a failing handle.
	closed := closedOriginStore(t, root)
	if _, err := closed.TelemetrySessionOriginMarkers(ctx, "codex", ids[:1]); err == nil {
		t.Fatal("failing handle answered a query")
	}
	if markers, err := closed.TelemetrySessionOriginMarkers(ctx, "codex", nil); err != nil || len(markers) != 0 {
		t.Fatalf("empty candidates queried: %v %v", markers, err)
	}
}
