package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

// Every state root below is an isolated fixture under t.TempDir(); nothing here
// reads or writes the real AgentDeck state, home or any plugin library.

const (
	birthOK    = "2026-10-29T10:00:00Z"
	birthMoved = "2026-10-29T11:00:00Z"
	// otherDeviceBirth is the start time another device's session of the same
	// canonical key would have.
	otherDeviceBirth = "2026-10-31T00:00:00Z"
	// Markers that exist only in the non-portable tables.
	evidenceMarker = "marker-evidence-token-7f3a"
	runMarker      = "marker-run-reason-91bc"
	configMarker   = "/private/synthetic-claude-mem-root"
)

var fixtureNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type overrideRow struct {
	client, sessionID string
	value             sql.NullString
	digest            string
	limited           int
	state             string
	revision          int64
	updatedAt         string
}

func readOverrides(t *testing.T, db *sql.DB) []overrideRow {
	t.Helper()
	rows, err := db.Query(`SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides ORDER BY client,session_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []overrideRow
	for rows.Next() {
		var row overrideRow
		if err := rows.Scan(&row.client, &row.sessionID, &row.value, &row.digest, &row.limited, &row.state, &row.revision, &row.updatedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func scalar[T any](t *testing.T, db *sql.DB, query string, args ...any) T {
	t.Helper()
	var value T
	if err := db.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func rowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	return scalar[int](t, db, "SELECT count(*) FROM "+table)
}

// originSourceState captures everything an export must leave untouched in the
// live core, so a sanitising bug that wrote the live database is visible.
type originSourceState struct {
	overrides                    []overrideRow
	preference                   string
	origins, evidence, runs, rec string
	revision, epoch              int64
}

func captureSource(t *testing.T, db *sql.DB) originSourceState {
	t.Helper()
	dump := func(query string) string {
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
	return originSourceState{
		overrides:  readOverrides(t, db),
		preference: dump(`SELECT * FROM origin_preferences`),
		origins:    dump(`SELECT * FROM session_origins ORDER BY client,session_id`),
		evidence:   dump(`SELECT * FROM origin_evidence ORDER BY client,session_id`),
		runs:       dump(`SELECT * FROM origin_adapter_runs ORDER BY adapter`),
		rec:        dump(`SELECT * FROM origin_mutations ORDER BY request_id`),
		revision:   scalar[int64](t, db, `SELECT revision FROM serving_generation WHERE singleton=1`),
		epoch:      scalar[int64](t, db, `SELECT epoch FROM derived_snapshot_generation WHERE singleton=1`),
	}
}

func originBinding(client, id, startedAt string) store.OriginBinding {
	return sessionorigin.BindingFor(client, id, sessionorigin.Birth{StartedAt: startedAt})
}

func applyOrigin(t *testing.T, core *store.Store, mutation store.OriginMutation) store.OriginReceipt {
	t.Helper()
	mutation.PayloadHash = "hash-" + mutation.RequestID
	mutation.Now = fixtureNow
	receipt, err := core.ApplyOriginMutation(context.Background(), mutation)
	if err != nil {
		t.Fatalf("%s: %v", mutation.RequestID, err)
	}
	return receipt
}

// seedOriginState writes the user's choices through the real mutation path and
// adds the non-portable rows an export must drop. Overrides:
//
//	codex/s-ok       auxiliary, complete birth, session exists on the target
//	codex/s-moved    work, complete birth, target's session has another birth
//	codex/s-limited  auxiliary, incomplete birth (binding_limited)
//	codex/s-absent   auxiliary, complete birth, session never indexed on target
//	claude/s-cleared tombstone: set, then cleared
func seedOriginState(t *testing.T, core *store.Store) {
	t.Helper()
	applyOrigin(t, core, store.OriginMutation{RequestID: "req-scope", Kind: store.OriginMutationScopeSet, Value: "auxiliary"})
	for _, item := range []struct{ client, id, value, birth string }{
		{"codex", "s-ok", "auxiliary", birthOK},
		{"codex", "s-moved", "work", birthMoved},
		{"codex", "s-limited", "auxiliary", ""},
		{"codex", "s-absent", "auxiliary", birthOK},
		{"claude", "s-cleared", "auxiliary", birthOK},
	} {
		applyOrigin(t, core, store.OriginMutation{RequestID: "req-" + item.id, Kind: store.OriginMutationOverrideSet, Client: item.client, SessionID: item.id,
			Value: item.value, Binding: originBinding(item.client, item.id, item.birth)})
	}
	applyOrigin(t, core, store.OriginMutation{RequestID: "req-clear", Kind: store.OriginMutationOverrideClear, Client: "claude", SessionID: "s-cleared", ExpectedRevision: 1})
	for _, statement := range []string{
		`INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES('codex','s-ok','auxiliary','maintenance_origin','rule','token',1)`,
		`INSERT INTO origin_evidence(adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at) VALUES('codex-otel-background','rule','codex','s-ok','` + evidenceMarker + `','auxiliary','v','2026-10-29T12:00:00Z')`,
		`INSERT INTO origin_adapter_runs(adapter,outcome,reason,validated_version,finished_at) VALUES('claude-mem','partial','` + runMarker + `','v','2026-10-29T12:00:00Z')`,
	} {
		if _, err := core.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

type originSource struct {
	root    string
	core    *store.Store
	service Service
}

func newOriginSource(t *testing.T, withSessions bool) originSource {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "source")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	if withSessions {
		sessions, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err = sessions.Close(); err != nil {
			t.Fatal(err)
		}
	}
	seedOriginState(t, core)
	if err = sessionorigin.SaveSourceConfig(root, sessionorigin.SourceConfig{ClaudeMem: sessionorigin.AdapterConfig{DataDir: configMarker}}); err != nil {
		t.Fatal(err)
	}
	return originSource{root: root, core: core, service: Service{Core: core, StateRoot: root, Vault: testBackupVault(t, "source-machine"), Version: "test"}}
}

func (s originSource) archive(t *testing.T, includeSessions bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "origin.adb")
	if _, err := s.service.Create(context.Background(), path, "passphrase", includeSessions); err != nil {
		t.Fatal(err)
	}
	return path
}

func restoreInto(t *testing.T, archive string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "target")
	if _, err := Restore(context.Background(), archive, target, "passphrase", syntheticMachineIdentity("target-machine")); err != nil {
		t.Fatal(err)
	}
	return target
}

// openCoreBytes opens an archive's core entry as a read-only database.
func openCoreBytes(t *testing.T, data []byte) *store.Store {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, coreName), data, platform.FileMode); err != nil {
		t.Fatal(err)
	}
	database, err := store.OpenReadOnly(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestBackupCarriesOnlyTheUsersOriginChoices(t *testing.T) {
	for _, includeSessions := range []bool{false, true} {
		t.Run(fmt.Sprintf("include_sessions=%t", includeSessions), func(t *testing.T) {
			source := newOriginSource(t, includeSessions)
			before := captureSource(t, source.core.DB)
			archive := source.archive(t, includeSessions)
			// The export's cleaning ran on a private snapshot: the live core's rows,
			// its serving revision and its epoch are exactly what they were.
			if after := captureSource(t, source.core.DB); !reflect.DeepEqual(before, after) {
				t.Fatalf("live core changed by export:\nbefore %+v\nafter  %+v", before, after)
			}
			manifest, entries, err := readEncrypted(archive, "passphrase")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{coreName, credentialsName}
			if includeSessions {
				want = append(want, sessionsName)
			}
			if !sameSet(manifest.Included, want) {
				t.Fatalf("included = %v, want %v", manifest.Included, want)
			}
			for name, data := range entries {
				if bytes.Contains(data, []byte(configMarker)) {
					t.Fatalf("archive entry %s carries the machine-local origin-source path", name)
				}
			}
			// The digest was computed over the cleaned snapshot: readEncrypted already
			// verified every entry's size and SHA-256 against the manifest.
			for _, marker := range []string{evidenceMarker, runMarker} {
				if bytes.Contains(entries[coreName], []byte(marker)) {
					t.Fatalf("core entry still holds non-portable row %q in its pages", marker)
				}
			}
			snapshot := openCoreBytes(t, entries[coreName])
			for _, table := range originNonPortableTables {
				if count := rowCount(t, snapshot.DB, table); count != 0 {
					t.Errorf("snapshot %s has %d rows, want none", table, count)
				}
			}
			if got := readOverrides(t, snapshot.DB); !reflect.DeepEqual(got, before.overrides) {
				t.Errorf("snapshot overrides = %+v\nwant %+v", got, before.overrides)
			}
			var scope string
			var revision int64
			if err = snapshot.DB.QueryRow(`SELECT scope,revision FROM origin_preferences`).Scan(&scope, &revision); err != nil || scope != "auxiliary" || revision != 1 {
				t.Errorf("snapshot preference = %q/%d, %v", scope, revision, err)
			}
		})
	}
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, name := range got {
		seen[name] = true
	}
	for _, name := range want {
		if !seen[name] {
			return false
		}
	}
	return true
}

func TestRestoreLeavesEveryOverrideUnmatchedAndDropsDerivedState(t *testing.T) {
	for _, includeSessions := range []bool{false, true} {
		t.Run(fmt.Sprintf("include_sessions=%t", includeSessions), func(t *testing.T) {
			ctx := context.Background()
			source := newOriginSource(t, includeSessions)
			before := captureSource(t, source.core.DB)
			target := restoreInto(t, source.archive(t, includeSessions))
			restored, err := store.OpenReadOnly(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			want := append([]overrideRow(nil), before.overrides...)
			for index := range want {
				want[index].state = "unmatched"
			}
			if got := readOverrides(t, restored.DB); !reflect.DeepEqual(got, want) {
				t.Fatalf("restored overrides = %+v\nwant %+v", got, want)
			}
			if got := captureSource(t, restored.DB).preference; got != before.preference {
				t.Errorf("restored preference = %q, want %q", got, before.preference)
			}
			for _, table := range originNonPortableTables {
				if count := rowCount(t, restored.DB, table); count != 0 {
					t.Errorf("restored %s has %d rows, want none", table, count)
				}
			}
			if _, found, err := restored.OriginReceipt(ctx, "req-scope"); err != nil || found {
				t.Errorf("source receipt survived restore: found=%t err=%v", found, err)
			}
			if epoch := scalar[int64](t, restored.DB, `SELECT epoch FROM derived_snapshot_generation WHERE singleton=1`); epoch == before.epoch {
				t.Errorf("restore kept the source epoch %d", epoch)
			}
			// The machine-local source configuration never travels, and a restore does
			// not recreate it.
			if _, err = os.Stat(sessionorigin.SourceConfigDir(target)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("restored root has an origin-source configuration: %v", err)
			}
			if after := captureSource(t, source.core.DB); !reflect.DeepEqual(before, after) {
				t.Errorf("restore changed the source state")
			}
		})
	}
}

// indexOn adds one indexed copy of a canonical session to a target's session
// index, the shape a scan produces from that root's own source logs.
func indexOn(t *testing.T, sessions *store.Store, client, id, firstAt string) {
	t.Helper()
	path := "/synthetic/" + client + "/" + id + ".jsonl"
	if _, err := sessions.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,'prefix',1,1,'2026-10-29T12:00:00Z')`, path, "identity-"+id); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,'proj','m',1,?,?)`, path, client, id, firstAt, firstAt); err != nil {
		t.Fatal(err)
	}
}

func stateOf(rows []overrideRow, client, id string) overrideRow {
	for _, row := range rows {
		if row.client == client && row.sessionID == id {
			return row
		}
	}
	return overrideRow{}
}

func TestRestoredOverridesApplyOnlyWhenTheTargetConfirmsTheSameBirth(t *testing.T) {
	ctx := context.Background()
	source := newOriginSource(t, false)
	before := captureSource(t, source.core.DB)
	target := restoreInto(t, source.archive(t, false))
	core, err := store.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	// The target's own source logs: s-ok has the stable birth the override was
	// made against; s-moved has the same ID but another device's birth;
	// s-limited has no complete birth; s-absent is not indexed here at all.
	indexOn(t, sessions, "codex", "s-ok", birthOK)
	indexOn(t, sessions, "codex", "s-moved", otherDeviceBirth)
	indexOn(t, sessions, "codex", "s-limited", "")
	reconcile := func() sessionorigin.ReconcileSummary {
		summary, err := sessionorigin.Reconcile(ctx, core, sessions, sessionorigin.ReconcileOptions{StateRoot: target, Now: func() time.Time { return fixtureNow }})
		if err != nil {
			t.Fatal(err)
		}
		return summary
	}
	if summary := reconcile(); summary.Reactivated != 1 {
		t.Fatalf("reactivated = %d, want only the confirmed birth", summary.Reactivated)
	}
	restored := readOverrides(t, core.DB)
	for _, item := range []struct{ client, id, state string }{
		{"codex", "s-ok", "matched"},
		{"codex", "s-moved", "unmatched"},
		{"codex", "s-limited", "unmatched"},
		{"codex", "s-absent", "unmatched"},
		{"claude", "s-cleared", "unmatched"},
	} {
		if got := stateOf(restored, item.client, item.id); got.state != item.state {
			t.Errorf("%s/%s state = %q, want %q", item.client, item.id, got.state, item.state)
		}
	}
	// Reactivation is not a user write: value, binding and revision are the
	// source's, so a settings view holding that revision can still save.
	for _, row := range restored {
		original := stateOf(before.overrides, row.client, row.sessionID)
		original.state = row.state
		if row != original {
			t.Errorf("%s/%s changed beyond its binding state: %+v vs %+v", row.client, row.sessionID, row, original)
		}
	}
	// Classification follows the stored state: s-ok applies, the others are kept
	// for review but do not change what any scope shows.
	selection, err := core.ReadOriginSelectionState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[string]store.OriginOverride{}
	for _, override := range selection.Overrides {
		overrides[override.SessionID] = override
	}
	classify := func(id, firstAt string) sessionorigin.Origin {
		override := overrides[id]
		return sessionorigin.Classify(session.IndexedSession{Metadata: session.Metadata{Client: "codex", SessionID: id}, Copies: []session.SourceCopy{{FirstAt: firstAt}}}, nil, &override)
	}
	if got := classify("s-ok", birthOK); got.Effective != "auxiliary" || got.Override == nil || !got.Override.Applied {
		t.Errorf("s-ok = %+v, want the override applied", got)
	}
	for id, firstAt := range map[string]string{"s-moved": otherDeviceBirth, "s-limited": ""} {
		if got := classify(id, firstAt); got.Effective != "unknown" || got.Override == nil || got.Override.Applied {
			t.Errorf("%s = %+v, want a kept but unapplied override", id, got)
		}
	}
	// A second pass with nothing new writes nothing and publishes nothing.
	revision := scalar[int64](t, core.DB, `SELECT revision FROM serving_generation WHERE singleton=1`)
	if summary := reconcile(); summary.Reactivated != 0 {
		t.Errorf("second pass reactivated %d", summary.Reactivated)
	}
	if after := scalar[int64](t, core.DB, `SELECT revision FROM serving_generation WHERE singleton=1`); after != revision {
		t.Errorf("idle pass advanced the serving revision %d → %d", revision, after)
	}
}

func TestExplicitSetRebindsARestoredOverrideToTheCurrentSession(t *testing.T) {
	ctx := context.Background()
	source := newOriginSource(t, false)
	target := restoreInto(t, source.archive(t, false))
	core, err := store.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	indexOn(t, sessions, "codex", "s-moved", otherDeviceBirth)
	record, found, err := core.OriginOverrideRecord(ctx, "codex", "s-moved")
	if err != nil || !found || record.BindingState != "unmatched" || record.Value == nil || *record.Value != "work" {
		t.Fatalf("restored override = %+v found=%t %v", record, found, err)
	}
	binding, err := sessionorigin.ResolveBinding(ctx, sessions, "codex", "s-moved")
	if err != nil || binding == nil || binding.Digest == record.BindingDigest {
		t.Fatalf("current binding = %+v, %v", binding, err)
	}
	// Setting the same value again is a real change: only the binding differs.
	receipt, err := core.ApplyOriginMutation(ctx, store.OriginMutation{RequestID: "req-rebind", PayloadHash: "hash-rebind", Kind: store.OriginMutationOverrideSet,
		Client: "codex", SessionID: "s-moved", Value: "work", Binding: *binding, ExpectedRevision: record.Revision, Now: fixtureNow})
	if err != nil || receipt.Outcome != store.OriginOutcomeSaved || receipt.Revision != record.Revision+1 {
		t.Fatalf("rebind receipt = %+v, %v", receipt, err)
	}
	rebound, _, err := core.OriginOverrideRecord(ctx, "codex", "s-moved")
	if err != nil || rebound.BindingState != "matched" || rebound.BindingDigest != binding.Digest {
		t.Fatalf("rebound override = %+v, %v", rebound, err)
	}
}

func TestRescanAfterLosingTheSessionIndexAppliesChoicesWithoutAWrite(t *testing.T) {
	// Losing the session index (cmd/agentdeck TestPurgeIndexKeepsEveryCoreOriginChoice
	// runs the real purge-index) leaves the core choices alone. After a rescan the
	// same stable birth applies them again from the stored binding, so nothing is
	// rewritten and nothing is published for them.
	ctx := context.Background()
	source := newOriginSource(t, true)
	before := captureSource(t, source.core.DB)
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err := os.Remove(filepath.Join(source.root, sessionsName) + suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if after := captureSource(t, source.core.DB); !reflect.DeepEqual(before, after) {
		t.Fatalf("losing the session index changed core choices")
	}
	sessions, err := store.OpenSessions(ctx, source.root)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	indexOn(t, sessions, "codex", "s-ok", birthOK)
	summary, err := sessionorigin.Reconcile(ctx, source.core, sessions, sessionorigin.ReconcileOptions{StateRoot: source.root, Now: func() time.Time { return fixtureNow }})
	if err != nil || summary.Reactivated != 0 {
		t.Fatalf("rescan = %+v, %v", summary, err)
	}
	if got := stateOf(readOverrides(t, source.core.DB), "codex", "s-ok"); got.state != "matched" || got.revision != stateOf(before.overrides, "codex", "s-ok").revision {
		t.Fatalf("s-ok after rescan = %+v", got)
	}
}

// tamperCore rewrites an archive's core database with mutate and re-seals the
// archive with a manifest that matches the new bytes, so only the content under
// test is invalid.
func tamperCore(t *testing.T, archive string, mutate func(*sql.DB)) string {
	t.Helper()
	manifest, entries, err := readEncrypted(archive, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, coreName)
	if err = os.WriteFile(path, entries[coreName], platform.FileMode); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", "file:"+path+"?mode=rw")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	mutate(database)
	if _, err = database.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	if entries[coreName], err = os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	for index, entry := range manifest.Entries {
		if entry.Name == coreName {
			digest := sha256Hex(entries[coreName])
			manifest.Entries[index].Size = int64(len(entries[coreName]))
			manifest.Entries[index].SHA256 = digest
		}
	}
	if entries[manifestName], err = EncodeManifest(manifest); err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.adb")
	if err = writeEncrypted(tampered, "passphrase", entries, fixtureNow); err != nil {
		t.Fatal(err)
	}
	return tampered
}

func execAll(t *testing.T, db *sql.DB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func TestRestoreRejectsInvalidOriginState(t *testing.T) {
	source := newOriginSource(t, false)
	archive := source.archive(t, false)
	loose := `PRAGMA ignore_check_constraints=ON`
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *sql.DB)
	}{
		{"default scope outside the enumeration", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_preferences SET scope='weekly'`)
		}},
		{"default revision zero", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_preferences SET revision=0`)
		}},
		{"default revision not an integer", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_preferences SET revision='one'`)
		}},
		{"override value outside the enumeration", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET value='maybe' WHERE session_id='s-ok'`)
		}},
		{"override binding state outside the enumeration", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET binding_state='pending' WHERE session_id='s-ok'`)
		}},
		{"override limited flag outside 0 and 1", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET binding_limited=2 WHERE session_id='s-ok'`)
		}},
		{"override revision not positive", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET revision=-3 WHERE session_id='s-ok'`)
		}},
		{"override revision stored as text", func(t *testing.T, db *sql.DB) {
			// Numeric text such as '2' would be coerced to an integer by the column's
			// affinity; only non-numeric text can stay text.
			execAll(t, db, loose, `UPDATE origin_overrides SET revision='two' WHERE session_id='s-ok'`)
		}},
		{"override for an unknown client", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET client='gemini' WHERE session_id='s-ok'`)
		}},
		{"override with an empty binding", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET binding_digest='' WHERE session_id='s-ok'`)
		}},
		{"override session id with a path separator", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose, `UPDATE origin_overrides SET session_id='a/b' WHERE session_id='s-ok'`)
		}},
		{"valid overrides without their primary key", func(t *testing.T, db *sql.DB) {
			// Every row and column is legal and unique; only the key is gone, so the
			// restored state could not take an explicit set (ON CONFLICT target).
			execAll(t, db, loose,
				`CREATE TABLE origin_overrides_loose AS SELECT * FROM origin_overrides`,
				`DROP TABLE origin_overrides`,
				`ALTER TABLE origin_overrides_loose RENAME TO origin_overrides`)
		}},
		{"valid default without its singleton key", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose,
				`CREATE TABLE origin_preferences_loose AS SELECT * FROM origin_preferences`,
				`DROP TABLE origin_preferences`,
				`ALTER TABLE origin_preferences_loose RENAME TO origin_preferences`)
		}},
		{"partial unique index instead of the key", func(t *testing.T, db *sql.DB) {
			execAll(t, db, loose,
				`CREATE TABLE origin_overrides_loose AS SELECT * FROM origin_overrides`,
				`DROP TABLE origin_overrides`,
				`ALTER TABLE origin_overrides_loose RENAME TO origin_overrides`,
				`CREATE UNIQUE INDEX origin_overrides_partial ON origin_overrides(client,session_id) WHERE value IS NOT NULL`)
		}},
		{"two overrides for one session", func(t *testing.T, db *sql.DB) {
			// Rebuild the table without its primary key so the duplicate can exist.
			execAll(t, db, loose,
				`CREATE TABLE origin_overrides_loose AS SELECT * FROM origin_overrides`,
				`DROP TABLE origin_overrides`,
				`ALTER TABLE origin_overrides_loose RENAME TO origin_overrides`,
				`INSERT INTO origin_overrides SELECT * FROM origin_overrides WHERE session_id='s-ok'`)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			tampered := tamperCore(t, archive, func(db *sql.DB) { test.mutate(t, db) })
			target := filepath.Join(t.TempDir(), "target")
			if _, err := Restore(context.Background(), tampered, target, "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("Restore error = %v, want invalid archive", err)
			}
			if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("rejected archive touched the target: %v", err)
			}
		})
	}
}

func TestRestoreAcceptsAnEquivalentFullUniqueKeyAndStillTakesExplicitSets(t *testing.T) {
	ctx := context.Background()
	source := newOriginSource(t, false)
	// The key is a full unique index rather than the primary key: it supports the
	// same upsert, so a valid archive of that shape must not be rejected.
	tampered := tamperCore(t, source.archive(t, false), func(db *sql.DB) {
		execAll(t, db,
			`CREATE TABLE origin_overrides_loose AS SELECT * FROM origin_overrides`,
			`DROP TABLE origin_overrides`,
			`ALTER TABLE origin_overrides_loose RENAME TO origin_overrides`,
			`CREATE UNIQUE INDEX origin_overrides_identity ON origin_overrides(client,session_id)`)
	})
	target := restoreInto(t, tampered)
	core, err := store.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	receipt, err := core.ApplyOriginMutation(ctx, store.OriginMutation{RequestID: "req-after-restore", PayloadHash: "hash-after-restore", Kind: store.OriginMutationOverrideSet,
		Client: "codex", SessionID: "s-new", Value: "auxiliary", Binding: originBinding("codex", "s-new", birthOK), Now: fixtureNow})
	if err != nil || receipt.Outcome != store.OriginOutcomeSaved {
		t.Fatalf("explicit set after restore = %+v, %v", receipt, err)
	}
}

func TestRestoreRejectsACoreFromANewerSchema(t *testing.T) {
	source := newOriginSource(t, false)
	archive := source.archive(t, false)
	ahead := store.CurrentSchemaVersion + 1
	tampered := tamperCore(t, archive, func(db *sql.DB) {
		execAll(t, db, fmt.Sprintf(`UPDATE schema_metadata SET version=%d`, ahead))
	})
	manifest, entries, err := readEncrypted(tampered, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	manifest.DatabaseSchemas[coreName] = ahead
	if entries[manifestName], err = EncodeManifest(manifest); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(t.TempDir(), "newer.adb")
	if err = writeEncrypted(newer, "passphrase", entries, fixtureNow); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if _, err = Restore(context.Background(), newer, target, "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Restore error = %v, want invalid archive", err)
	}
	if _, err = os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("schema-ahead archive touched the target: %v", err)
	}
}

func TestRestoreRollsBackWhenOriginStateCannotBeReset(t *testing.T) {
	source := newOriginSource(t, false)
	archive := source.archive(t, false)
	// An archive that carries a derived row and a trigger that refuses its
	// removal: the reset fails after the target files exist, so everything the
	// restore created must go.
	tampered := tamperCore(t, archive, func(db *sql.DB) {
		execAll(t, db,
			`INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES('codex','s-ok','unknown','origin_metadata_missing','rule','token',0)`,
			`CREATE TRIGGER refuse_origin_reset BEFORE DELETE ON session_origins BEGIN SELECT RAISE(ABORT, 'synthetic origin reset failure'); END`)
	})
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "target")
			if existing {
				if err := os.Mkdir(target, platform.DirectoryMode); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Restore(context.Background(), tampered, target, "passphrase", syntheticMachineIdentity("target-machine")); err == nil || !strings.Contains(err.Error(), "synthetic origin reset failure") {
				t.Fatalf("Restore error = %v", err)
			}
			if existing {
				if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
					t.Fatalf("target rollback: %v, %v", entries, err)
				}
			} else if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("new target remains: %v", err)
			}
		})
	}
}

func TestBackupFailsWithoutAnArchiveWhenTheSnapshotCannotBeCleaned(t *testing.T) {
	source := newOriginSource(t, false)
	execAll(t, source.core.DB,
		`CREATE TRIGGER refuse_receipt_removal BEFORE DELETE ON origin_mutations BEGIN SELECT RAISE(ABORT, 'synthetic snapshot cleaning failure'); END`)
	before := captureSource(t, source.core.DB)
	destination := filepath.Join(t.TempDir(), "never.adb")
	_, err := source.service.Create(context.Background(), destination, "passphrase", false)
	if err == nil || !strings.Contains(err.Error(), "synthetic snapshot cleaning failure") {
		t.Fatalf("Create error = %v", err)
	}
	if _, statErr := os.Stat(destination); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("failed export left an archive: %v", statErr)
	}
	if after := captureSource(t, source.core.DB); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed export changed the live core")
	}
}

func TestSessionDatabaseValidationStillRejectsCorruptSnapshots(t *testing.T) {
	// A corrupt include-sessions entry keeps the contract: rejected before the
	// target is touched, with origin choices unable to rescue it.
	source := newOriginSource(t, true)
	archive := source.archive(t, true)
	manifest, entries, err := readEncrypted(archive, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	entries[sessionsName] = append([]byte("not a database"), entries[sessionsName]...)
	for index, entry := range manifest.Entries {
		if entry.Name == sessionsName {
			manifest.Entries[index].Size = int64(len(entries[sessionsName]))
			manifest.Entries[index].SHA256 = sha256Hex(entries[sessionsName])
		}
	}
	if entries[manifestName], err = EncodeManifest(manifest); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(t.TempDir(), "corrupt.adb")
	if err = writeEncrypted(corrupt, "passphrase", entries, fixtureNow); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if _, err = Restore(context.Background(), corrupt, target, "passphrase", syntheticMachineIdentity("target-machine")); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Restore error = %v, want invalid archive", err)
	}
	if _, err = os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("corrupt archive touched the target: %v", err)
	}
}
