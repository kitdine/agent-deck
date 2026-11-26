//go:build cjsi_adapter_fixture

package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/ingest"
	providerpkg "github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/scanruntime"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func openRawSQLite(t *testing.T, path, mode string) *sql.DB {
	t.Helper()
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode + "&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func execRaw(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func seedManagedSourceRoot(t *testing.T, withSessions bool) (root string, vault *credentialvault.Vault, sourceFile string) {
	t.Helper()
	ctx := context.Background()
	root = t.TempDir()
	vault = testBackupVault(t, "managed-source-machine")

	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	// Seed a provider + encrypted credential.
	providers := providerpkg.Service{Store: core, Vault: vault}
	if _, err := providers.Add(ctx, providerpkg.Definition{
		Name:          "managed-prov",
		Endpoint:      "https://example.invalid",
		CredentialRef: "managed-prov/default",
		Multiplier:    "1",
		Clients:       []providerpkg.Client{providerpkg.ClientCodex},
	}, "fake-secret-value-for-managed-roundtrip"); err != nil {
		core.Close()
		t.Fatal(err)
	}

	// Seed origin preferences, overrides, non-portable origin tables, and usage events.
	execRaw(t, core.DB, `
		INSERT INTO origin_preferences(singleton, scope, revision, updated_at) VALUES(1, 'work', 3, '2026-01-01T00:00:00Z');
		INSERT INTO origin_overrides(client, session_id, binding_digest, binding_limited, binding_state, value, revision, updated_at)
			VALUES('codex', 'thread-1', 'digest-thread-1', 0, 'matched', 'auxiliary', 7, '2026-01-01T00:00:00Z');
		INSERT INTO session_origins(client, session_id, automatic, reason_code, rule_version, input_token, evidence_revision)
			VALUES('codex', 'thread-1', 'work', 'default_work', 'v1', 'tok-1', 1);
		INSERT INTO usage_sessions(client, session_id, first_at, last_at)
			VALUES('codex', 'thread-1', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z');
		INSERT INTO usage_events(event_key, client, session_id, event_id, event_at, model, input_tokens, output_tokens, source_path, source_offset)
			VALUES('ev-1', 'codex', 'thread-1', 'resp-1', '2026-01-01T00:00:30Z', 'gpt-5', 100, 20, '/synthetic/src.jsonl', 0);
		UPDATE usage_derived_partitions SET payload = CAST('DerivedCacheSentinel' AS BLOB), active_revision = 1, dirty = 0, version = 'v1';
	`)
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}

	if withSessions {
		sourceFile = filepath.Join(t.TempDir(), "codex-session.jsonl")
		if err := os.WriteFile(sourceFile, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		index, err := store.OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		execRaw(t, index.DB, `
			INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,priority,parser_version,scanned_at,parser_context)
			VALUES(?, 'id-src-1', 50, CAST('LegacyTailSentinel' AS BLOB), 50, 10, 10, 'pfx1', 10, 8, '2026-01-01T00:00:00Z', 'LegacyContextSentinel');
			INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
			VALUES(?, 'codex', 'thread-1', 'proj-a', 'gpt-5', 8, '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z'),
			      (?, 'codex', 'thread-excluded', 'proj-drop', 'gpt-5', 8, '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z'),
			      (?, 'claude', 'thread-empty', 'proj-empty', 'claude-4', 6, '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z');
			INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text)
			VALUES(1, ?, 'codex', 'thread-1', '2026-01-01T00:00:01Z', 'user_prompt', 'AlphaDoc'),
			      (2, ?, 'codex', 'thread-1', '2026-01-01T00:00:02Z', 'assistant_message', 'BetaDoc'),
			      (3, ?, 'codex', 'thread-1', '2026-01-01T00:00:03Z', 'user_prompt', 'GammaDoc'),
			      (5, ?, 'codex', 'thread-excluded', '2026-01-01T00:00:04Z', 'user_prompt', 'ExcludedDocSentinel');
			INSERT INTO session_exclusions(kind, value) VALUES('project', 'proj-drop');`,
			sourceFile, sourceFile, sourceFile, sourceFile, sourceFile, sourceFile, sourceFile, sourceFile)
		if err := index.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatalf("InstallTask5Fixture: %v", err)
	}
	return root, vault, sourceFile
}

// attachPublishedManagedState adds:
//   - Ticket 1 (opened): confirmed usage selection/intervals, OTel takeover + receipts,
//     scope overlay, AXBC candidate order (1,4,2,3) with high_water=12 & order_verified=1,
//     and a valid physical serving slot.
//   - Ticket 2 (draft/unopened): candidate payload with UnopenedCandidateSentinel,
//     unopened usage group, transient checkpoint attempt/record, and dirty range.
func attachPublishedManagedState(t *testing.T, root, sourceFile string) (slotFile string, origCoreEpoch, origIndexEpoch int64) {
	t.Helper()
	ctx := context.Background()
	coreDB := openRawSQLite(t, filepath.Join(root, "agentdeck.sqlite3"), "rw")
	indexDB := openRawSQLite(t, filepath.Join(root, "session-index-v1.sqlite3"), "rw")

	var baseSeal []byte
	var baseRev int64
	if err := coreDB.QueryRowContext(ctx, `SELECT core_epoch, index_epoch, base_seal, base_revision FROM cjsi_publication_head WHERE singleton=1`).
		Scan(&origCoreEpoch, &origIndexEpoch, &baseSeal, &baseRev); err != nil {
		t.Fatal(err)
	}

	sessSeal := sha256.Sum256([]byte("managed-ticket1-session-seal"))
	slotSeal := sha256.Sum256([]byte("managed-ticket1-slot-seal"))
	diffDigest := sha256.Sum256([]byte("managed-ticket1-diff"))
	inputDigest := sha256.Sum256([]byte("managed-ticket1-input"))
	proofDigest := sha256.Sum256([]byte("managed-ticket1-proof"))
	obsDigest := sha256.Sum256([]byte("managed-ticket1-obs"))
	epochDigest := sha256.Sum256([]byte("managed-ticket1-counter-epoch"))
	scopeKey := sha256.Sum256([]byte("managed-scope-key"))
	windowKey := sha256.Sum256([]byte("managed-window-key"))
	seqID := sha256.Sum256([]byte("managed-seq-id"))

	// Write physical serving slot for Ticket 1.
	slotIdentity := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	slotPayload := []byte(`{"slot":"ticket-1-serving-payload"}`)
	slotDigest := sha256.Sum256(slotPayload)
	slotFile = filepath.Join(root, fmt.Sprintf(".cjsi-serving-slot-%x", slotIdentity))
	if err := os.WriteFile(slotFile, slotPayload, 0o600); err != nil {
		t.Fatal(err)
	}

	// Stage Ticket 1 (opened) and Ticket 2 (draft/unopened) in private session index.
	execRaw(t, indexDB, `UPDATE cjsi_index_state SET doc_id_high_water = 12 WHERE singleton = 1`)
	execRaw(t, indexDB, `INSERT INTO cjsi_candidate_payloads(doc_id, event_at, kind, text) VALUES
		(4, '2026-01-01T00:00:01.500Z', 'assistant_message', 'InsertedXDoc'),
		(9, '2026-01-01T00:00:09Z', 'user_prompt', 'UnopenedCandidateSentinel')`)
	execRaw(t, indexDB, `INSERT INTO cjsi_session_candidates(ticket, attempt, core_epoch, index_epoch, base_ticket, base_attempt, base_seal, base_revision, parent_ticket, protection_revision, coverage_revision, session_revision)
		VALUES(1, 1, ?, ?, 0, 0, ?, ?, 0, 0, 0, ?)`,
		origCoreEpoch, origIndexEpoch, baseSeal, baseRev, baseRev+1)
	execRaw(t, indexDB, `INSERT INTO cjsi_candidate_groups(ticket, attempt, source_path, client, session_id, op, project, model, parser_version, first_at, last_at, order_verified, sequence_id, head_at, birth)
		VALUES(1, 1, ?, 'codex', 'thread-1', 'replace', 'proj-a', 'gpt-5', 8, '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', 1, ?, '', '')`,
		sourceFile, seqID[:])
	execRaw(t, indexDB, `INSERT INTO cjsi_candidate_order(ticket, attempt, source_path, client, session_id, doc_id, doc_side, detail_ordinal) VALUES
		(1, 1, ?, 'codex', 'thread-1', 1, 'base', 1),
		(1, 1, ?, 'codex', 'thread-1', 4, 'candidate', 2),
		(1, 1, ?, 'codex', 'thread-1', 2, 'base', 3),
		(1, 1, ?, 'codex', 'thread-1', 3, 'base', 4)`,
		sourceFile, sourceFile, sourceFile, sourceFile)
	execRaw(t, indexDB, `UPDATE cjsi_session_candidates SET seal_digest = ? WHERE ticket = 1 AND attempt = 1`, sessSeal[:])

	// Unopened draft attempt (ticket 2) that must NEVER be exported.
	execRaw(t, indexDB, `INSERT INTO cjsi_session_candidates(ticket, attempt, core_epoch, index_epoch, base_ticket, base_attempt, base_seal, base_revision, parent_ticket, protection_revision, coverage_revision, session_revision)
		VALUES(2, 1, ?, ?, 0, 0, ?, ?, 1, 0, 0, ?)`,
		origCoreEpoch, origIndexEpoch, baseSeal, baseRev, baseRev+2)
	execRaw(t, indexDB, `INSERT INTO cjsi_candidate_groups(ticket, attempt, source_path, client, session_id, op, project, model, parser_version, first_at, last_at, order_verified, sequence_id, head_at, birth)
		VALUES(2, 1, ?, 'codex', 'thread-1', 'replace', 'proj-a', 'gpt-5', 8, '2026-01-01T00:00:00Z', '2026-01-01T00:02:00Z', 1, ?, '', '')`,
		sourceFile, seqID[:])
	execRaw(t, indexDB, `INSERT INTO cjsi_candidate_order(ticket, attempt, source_path, client, session_id, doc_id, doc_side, detail_ordinal)
		VALUES(2, 1, ?, 'codex', 'thread-1', 9, 'candidate', 1)`, sourceFile)

	// Stage Ticket 1 (opened) + receipts + intervals + overlays, plus Ticket 2 (draft) + transient tables in core.
	execRaw(t, coreDB, `INSERT INTO cjsi_publication_attempts(
			ticket, attempt, base_ticket, core_epoch, index_epoch,
			session_revision, usage_revision, user_revision, route_revision, price_revision,
			protection_revision, coverage_revision, input_digest, diff_digest,
			session_seal, slot_seal, slot_digest, slot_size, slot_format, slot_identity,
			built_at, valid_until, accept_until, state
		) VALUES (
			1, 1, 0, ?, ?,
			?, 1, 1, 1, 1,
			0, 0, ?, ?,
			?, ?, ?, ?, 1, ?,
			'2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z', 'opened'
		)`,
		origCoreEpoch, origIndexEpoch, baseRev+1, inputDigest[:], diffDigest[:],
		sessSeal[:], slotSeal[:], slotDigest[:], len(slotPayload), slotIdentity[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_groups(ticket, attempt, thread, input_digest, diff_digest, state, gaps, pending, partial, unresolved_overlap)
		VALUES(1, 1, 'thread-1', ?, ?, 1, 0, 0, 0, 0)`, inputDigest[:], diffDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_selection(thread, ticket, attempt) VALUES('thread-1', 1, 1)`)
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_members(ticket, attempt, thread, kind, member_id, turn_id, model, event_at, input_tokens, output_tokens)
		VALUES(1, 1, 'thread-1', 3, 'iv-1', '', 'gpt-5', '2026-01-01T00:00:30Z', 10, 5)`)
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_intervals(ticket, attempt, thread, kind, member_id, counter_epoch, lower_at, upper_at, lower_input, upper_input, lower_output, upper_output)
		VALUES(1, 1, 'thread-1', 3, 'iv-1', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', 0, 10, 0, 5)`, epochDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_takeovers(client, thread, period, opened_ticket, closed_ticket, attempt, receipt_version, safe_old_side, proof_digest)
		VALUES('codex', 'thread-1', 1, 1, NULL, 1, 1, 1, ?)`, proofDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_receipt_observations(
			client, thread, period, observation_key, state, reason, metered,
			support_entry, source, producer, producer_version, originator, source_class, transport,
			model, event_kind, event_name, request_id, response_id, attempt_id,
			record_at, observed_at, received_at, input_tokens, output_tokens, observation_digest, disposition
		) VALUES (
			'codex', 'thread-1', 1, 'obs-1', 'settled', 'ok', 1,
			'codex', 'otel', 'codex', '1.0', 'cli', 'otel', 'grpc',
			'gpt-5', 'response', 'codex.response', 'req-1', 'resp-1', 'att-1',
			'2026-01-01T00:00:10Z', '2026-01-01T00:00:11Z', '2026-01-01T00:00:12Z', 7, 3, ?, 'applied'
		)`, obsDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_receipt_events(
			client, thread, period, observation_key, event_key, session_id, event_id, event_at,
			model, source_path, source_offset,
			input_tokens, cached_input_tokens, output_tokens,
			cache_read_tokens, cache_creation_tokens, cache_write_tokens, cache_write_5m_tokens, cache_write_1h_tokens
		) VALUES (
			'codex', 'thread-1', 1, 'obs-1', 'otel-ev-1', 'thread-1', 'resp-1', '2026-01-01T00:00:10Z',
			'gpt-5', 'otel://codex', 0, 7, 0, 3, 0, 0, 0, 0, 0
		)`)
	execRaw(t, coreDB, `INSERT INTO cjsi_receipt_sessions(client, thread, period, first_at, last_at, started_at)
		VALUES('codex', 'thread-1', 1, '2026-01-01T00:00:10Z', '2026-01-01T00:00:10Z', '2026-01-01T00:00:10Z')`)
	execRaw(t, coreDB, `INSERT INTO cjsi_receipt_markers(client, thread, period, source_kind, model, first_at, last_at, support_entry, settled_at)
		VALUES('codex', 'thread-1', 1, 'otel', 'gpt-5', '2026-01-01T00:00:10Z', '2026-01-01T00:00:10Z', 'codex', '2026-01-01T00:00:12Z')`)
	execRaw(t, coreDB, `INSERT INTO cjsi_scope_overlays(domain, scope_kind, scope_key, window_key, pending, partial, masked, coverage_revision, protection_revision)
		VALUES('usage', 'thread', ?, ?, 0, 1, 0, 0, 0)`, scopeKey[:], windowKey[:])
	execRaw(t, coreDB, `UPDATE cjsi_publication_head
		SET ticket = 1, session_ticket = 1, session_attempt = 1, session_revision = ?, session_seal = ?,
		    slot_ticket = 1, slot_attempt = 1, slot_seal = ?
		WHERE singleton = 1`, baseRev+1, sessSeal[:], slotSeal[:])

	// Unopened draft attempt (ticket 2) + dirty ranges (all must be scrubbed).
	execRaw(t, coreDB, `INSERT INTO cjsi_publication_attempts(
			ticket, attempt, base_ticket, core_epoch, index_epoch,
			session_revision, usage_revision, user_revision, route_revision, price_revision,
			protection_revision, coverage_revision, input_digest, accept_until, state
		) VALUES (
			2, 1, 1, ?, ?,
			?, 2, 1, 1, 1,
			0, 0, ?, '2026-01-02T00:00:00Z', 'draft'
		)`, origCoreEpoch, origIndexEpoch, baseRev+2, inputDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_groups(ticket, attempt, thread, input_digest, diff_digest, state, gaps, pending, partial, unresolved_overlap)
		VALUES(2, 1, 'thread-draft', ?, ?, 1, 0, 1, 0, 0)`, inputDigest[:], diffDigest[:])
	execRaw(t, coreDB, `INSERT INTO cjsi_usage_interval_dirty_ranges(core_epoch, scope_kind, client, model, from_day, to_day, ticket)
		VALUES(?, 'client-summary', 'codex', '', 20454, 20455, 1)`, origCoreEpoch)
	return slotFile, origCoreEpoch, origIndexEpoch
}

func assertRestoredRootEntries(t *testing.T, root string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("unexpected restored entry %q: %v", entry.Name(), err)
		}
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, want) {
		t.Fatalf("restored root entries = %v, want %v", names, want)
	}
}

func TestCJSIManagedBackupRoundtrip(t *testing.T) {
	t.Run("with_sessions_preserves_published_state_and_rebinds_epochs", func(t *testing.T) {
		ctx := context.Background()
		sourceRoot, sourceVault, sourceFile := seedManagedSourceRoot(t, true)
		_, origCoreEpoch, origIndexEpoch := attachPublishedManagedState(t, sourceRoot, sourceFile)

		// Capture public read-back from the source root BEFORE backup.
		srcManaged, err := store.ResolveCJSIRuntimeRoot(ctx, sourceRoot)
		if err != nil || srcManaged == nil {
			t.Fatalf("source ResolveCJSIRuntimeRoot: %v %v", srcManaged, err)
		}
		var wantMeta []session.Metadata
		var wantDocs []session.Document
		var wantOrder session.CJSIOrderSnapshot
		if err := srcManaged.WithPublishedCore(ctx, func(tx *sql.Tx, _ store.CJSIIndexStamp) error {
			reader := session.NewCJSIReader(sourceRoot, tx, ingest.CodexSelectedLimits{})
			var err error
			wantMeta, err = reader.List(ctx)
			if err != nil {
				return err
			}
			detail, err := reader.SelectDetail(ctx, "codex", "thread-1")
			if err != nil {
				return err
			}
			wantDocs, _, err = reader.DocumentsPage(ctx, detail, 1, 10, false)
			if err != nil {
				return err
			}
			return srcManaged.WithIndexSnapshot(ctx, tx, func(snap store.CJSIIndexSnapshot) error {
				wantOrder, err = session.ReadCJSIOrderSnapshot(ctx, snap)
				return err
			})
		}); err != nil {
			t.Fatalf("source public read-back: %v", err)
		}
		if len(wantDocs) != 4 || wantDocs[0].Text != "AlphaDoc" || wantDocs[1].Text != "InsertedXDoc" || wantDocs[2].Text != "BetaDoc" || wantDocs[3].Text != "GammaDoc" {
			t.Fatalf("unexpected source docs: %+v", wantDocs)
		}

		// Create managed archive (Core is nil because schema 37 cannot be opened via legacy store.Open).
		archivePath := filepath.Join(t.TempDir(), "managed.adb")
		manifest, err := (Service{
			StateRoot: sourceRoot,
			Vault:     sourceVault,
			Version:   "0.7.5-test",
		}).Create(ctx, archivePath, "managed-passphrase", true)
		if err != nil {
			t.Fatalf("managed Create: %v", err)
		}
		if manifest.DatabaseSchemas[coreName] != cjsiCoreSchemaVersion || manifest.DatabaseSchemas[sessionsName] != sessionSnapshotSchemaVersion {
			t.Fatalf("manifest schemas = %+v", manifest.DatabaseSchemas)
		}

		// Verify scrubbed archive bytes never contain candidates, revoked/excluded text, tails, or caches.
		_, entries, err := readEncrypted(archivePath, "managed-passphrase")
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range []string{
			"UnopenedCandidateSentinel",
			"ExcludedDocSentinel",
			"LegacyTailSentinel",
			"LegacyContextSentinel",
			"DerivedCacheSentinel",
			"thread-draft",
		} {
			if bytes.Contains(entries[coreName], []byte(sentinel)) || bytes.Contains(entries[sessionsName], []byte(sentinel)) {
				t.Fatalf("archive leaked forbidden content category")
			}
		}

		// Restore into an empty target root with a different machine identity.
		targetRoot := filepath.Join(t.TempDir(), "restored-managed")
		targetMachine := syntheticMachineIdentity("managed-target-machine")
		if _, err := Restore(ctx, archivePath, targetRoot, "managed-passphrase", targetMachine); err != nil {
			t.Fatalf("managed Restore: %v", err)
		}
		assertRestoredRootEntries(t, targetRoot, coreName, "credential.key", "session-index-v1.sqlite3", sessionsName)

		// 1. Target must resolve as a valid managed CJSI runtime root.
		dstManaged, err := store.ResolveCJSIRuntimeRoot(ctx, targetRoot)
		if err != nil || dstManaged == nil {
			t.Fatalf("restored ResolveCJSIRuntimeRoot: %v %v", dstManaged, err)
		}

		// 2. Legacy sessions.sqlite3 must be the 4097-byte fence and reject legacy SQLite opens.
		if _, err := store.OpenSessionsReadOnly(ctx, targetRoot); err == nil {
			t.Fatal("legacy OpenSessionsReadOnly succeeded on restored fenced root")
		}

		// 3. 9a doctor check (both quick and full) must pass on the restored root.
		docReport, err := (doctor.Service{StateRoot: targetRoot}).Check(ctx, true)
		if err != nil {
			t.Fatalf("doctor Check: %v", err)
		}
		for _, want := range []string{"schema", "database_integrity", "sessions", "session_sources"} {
			found := false
			for _, c := range docReport.Checks {
				if c.Name == want && c.Status == "ok" {
					found = true
				}
			}
			if !found {
				raw, _ := json.Marshal(docReport)
				t.Fatalf("doctor missing ok check %q: %s", want, raw)
			}
		}

		// 4. Verify epochs rebound, old tickets/slots not revived, and public read-back matches field-for-field.
		var gotMeta []session.Metadata
		var gotDocs []session.Document
		var gotOrder session.CJSIOrderSnapshot
		var gotStamp store.CJSIIndexStamp
		if err := dstManaged.WithPublishedCore(ctx, func(tx *sql.Tx, stamp store.CJSIIndexStamp) error {
			gotStamp = stamp
			// Verify no old slot or old candidate session ticket survived.
			var slotTicket int64
			if err := tx.QueryRowContext(ctx, `SELECT slot_ticket FROM cjsi_publication_head WHERE singleton=1`).Scan(&slotTicket); err != nil {
				return err
			}
			if slotTicket != 0 || stamp.SessionTicket != 0 || stamp.BaseTicket != 0 {
				return fmt.Errorf("old slot/session ticket revived: slot=%d stamp=%+v", slotTicket, stamp)
			}
			// Verify confirmed usage selection, intervals, OTel takeover/receipts, overlays, and overrides survived.
			for _, check := range []struct {
				query string
				want  int
			}{
				{`SELECT count(*) FROM cjsi_usage_selection WHERE thread='thread-1'`, 1},
				{`SELECT count(*) FROM cjsi_usage_intervals WHERE thread='thread-1' AND upper_input=10 AND upper_output=5`, 1},
				{`SELECT count(*) FROM cjsi_takeovers WHERE thread='thread-1' AND period=1`, 1},
				{`SELECT count(*) FROM cjsi_receipt_observations WHERE observation_key='obs-1'`, 1},
				{`SELECT count(*) FROM cjsi_receipt_events WHERE event_key='otel-ev-1'`, 1},
				{`SELECT count(*) FROM cjsi_receipt_sessions WHERE thread='thread-1'`, 1},
				{`SELECT count(*) FROM cjsi_receipt_markers WHERE thread='thread-1'`, 1},
				{`SELECT count(*) FROM cjsi_scope_overlays WHERE domain='usage' AND partial=1`, 1},
				{`SELECT count(*) FROM origin_preferences WHERE scope='work' AND revision=3`, 1},
				{`SELECT count(*) FROM origin_overrides WHERE session_id='thread-1' AND value='auxiliary' AND binding_state='unmatched' AND revision=7`, 1},
				{`SELECT count(*) FROM session_origins`, 0},
				{`SELECT count(*) FROM cjsi_publication_attempts WHERE state<>'opened'`, 0},
				{`SELECT count(*) FROM cjsi_usage_interval_dirty_ranges`, 0},
				{`SELECT count(*) FROM usage_derived_partitions WHERE payload IS NOT NULL`, 0},
			} {
				var count int
				if err := tx.QueryRowContext(ctx, check.query).Scan(&count); err != nil || count != check.want {
					return fmt.Errorf("%s: got %d want %d (err=%v)", check.query, count, check.want, err)
				}
			}

			// Verify re-sealed credential decrypts with target machine identity.
			targetVault := credentialvault.New(targetRoot, targetMachine)
			var credRef, algo, keyID string
			var keyVersion int
			var nonce, ciphertext []byte
			if err := tx.QueryRowContext(ctx, `SELECT pc.credential_ref, cs.algorithm, cs.key_version, cs.key_id, cs.nonce, cs.ciphertext
				FROM provider_credentials pc JOIN credential_secrets cs ON cs.credential_id=pc.id
				LIMIT 1`).Scan(&credRef, &algo, &keyVersion, &keyID, &nonce, &ciphertext); err != nil {
				return fmt.Errorf("read restored credential secret: %w", err)
			}
			plain, err := targetVault.Open(ctx, credRef, credentialvault.Sealed{
				Algorithm: algo, KeyVersion: keyVersion, KeyID: keyID, Nonce: nonce, Ciphertext: ciphertext,
			})
			if err != nil || plain != "fake-secret-value-for-managed-roundtrip" {
				return fmt.Errorf("restored credential open failed: err=%v", err)
			}

			reader := session.NewCJSIReader(targetRoot, tx, ingest.CodexSelectedLimits{})
			gotMeta, err = reader.List(ctx)
			if err != nil {
				return err
			}
			detail, err := reader.SelectDetail(ctx, "codex", "thread-1")
			if err != nil {
				return err
			}
			gotDocs, _, err = reader.DocumentsPage(ctx, detail, 1, 10, false)
			if err != nil {
				return err
			}
			// Check pagination limit=2 (AX then BC).
			page1, pg1, err := reader.DocumentsPage(ctx, detail, 1, 2, false)
			if err != nil || len(page1) != 2 || page1[0].Text != "AlphaDoc" || page1[1].Text != "InsertedXDoc" || !pg1.HasMore || pg1.Total != 4 {
				return fmt.Errorf("page 1 mismatch: len=%d pg=%+v err=%v", len(page1), pg1, err)
			}
			page2, pg2, err := reader.DocumentsPage(ctx, detail, 2, 2, false)
			if err != nil || len(page2) != 2 || page2[0].Text != "BetaDoc" || page2[1].Text != "GammaDoc" || pg2.HasMore || pg2.Total != 4 {
				return fmt.Errorf("page 2 mismatch: len=%d pg=%+v err=%v", len(page2), pg2, err)
			}
			// Check FTS search does not return excluded or unopened docs.
			hits, err := reader.Search(ctx, "UnopenedCandidateSentinel")
			if err != nil || len(hits) != 0 {
				return fmt.Errorf("unopened candidate leaked in search: len=%d err=%v", len(hits), err)
			}
			return dstManaged.WithIndexSnapshot(ctx, tx, func(snap store.CJSIIndexSnapshot) error {
				var portableLeft int
				if err := snap.Tx.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE lower(name) GLOB 'portable_session_*'`).Scan(&portableLeft); err != nil || portableLeft != 0 {
					return fmt.Errorf("portable_session_* left in private index: %d err=%v", portableLeft, err)
				}
				gotOrder, err = session.ReadCJSIOrderSnapshot(ctx, snap)
				return err
			})
		}); err != nil {
			t.Fatalf("restored public read-back: %v", err)
		}

		if gotStamp.CoreEpoch == origCoreEpoch || gotStamp.IndexEpoch == origIndexEpoch {
			t.Fatalf("epochs not rebound: core %d->%d, index %d->%d", origCoreEpoch, gotStamp.CoreEpoch, origIndexEpoch, gotStamp.IndexEpoch)
		}
		if len(gotMeta) != len(wantMeta) {
			t.Fatalf("meta count = %d, want %d", len(gotMeta), len(wantMeta))
		}
		for i := range wantMeta {
			if gotMeta[i] != wantMeta[i] {
				t.Fatalf("meta[%d] = %+v, want %+v", i, gotMeta[i], wantMeta[i])
			}
		}
		if len(gotDocs) != len(wantDocs) {
			t.Fatalf("docs count = %d, want %d", len(gotDocs), len(wantDocs))
		}
		for i := range wantDocs {
			if gotDocs[i] != wantDocs[i] {
				t.Fatalf("doc[%d] = %+v, want %+v", i, gotDocs[i], wantDocs[i])
			}
		}
		if gotOrder.HighWater != wantOrder.HighWater || len(gotOrder.Entries) != len(wantOrder.Entries) {
			t.Fatalf("order snapshot mismatch: got hw=%d n=%d, want hw=%d n=%d", gotOrder.HighWater, len(gotOrder.Entries), wantOrder.HighWater, len(wantOrder.Entries))
		}
		for i := range wantOrder.Entries {
			if gotOrder.Entries[i] != wantOrder.Entries[i] {
				t.Fatalf("order[%d] = %+v, want %+v", i, gotOrder.Entries[i], wantOrder.Entries[i])
			}
		}
	})

	t.Run("without_sessions_installs_fence_only", func(t *testing.T) {
		ctx := context.Background()
		sourceRoot, sourceVault, _ := seedManagedSourceRoot(t, false)
		archivePath := filepath.Join(t.TempDir(), "managed-core-only.adb")
		manifest, err := (Service{
			StateRoot: sourceRoot,
			Vault:     sourceVault,
			Version:   "0.7.5-test",
		}).Create(ctx, archivePath, "pass", true)
		if err != nil {
			t.Fatalf("managed core-only Create: %v", err)
		}
		if _, ok := manifest.DatabaseSchemas[sessionsName]; ok {
			t.Fatalf("unexpected sessions entry in never-built root manifest: %+v", manifest.DatabaseSchemas)
		}
		targetRoot := filepath.Join(t.TempDir(), "restored-core-only")
		if _, err := Restore(ctx, archivePath, targetRoot, "pass", syntheticMachineIdentity("target-2")); err != nil {
			t.Fatalf("managed core-only Restore: %v", err)
		}
		assertRestoredRootEntries(t, targetRoot, coreName, "credential.key", sessionsName)
		if managed, err := store.ResolveCJSIRuntimeRoot(ctx, targetRoot); err != nil || managed == nil {
			t.Fatalf("ResolveCJSIRuntimeRoot = %v, %v", managed, err)
		}
		if _, err := os.Lstat(filepath.Join(targetRoot, "session-index-v1.sqlite3")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("private index created for core-only restore: %v", err)
		}
		if _, err := store.OpenSessionsReadOnly(ctx, targetRoot); err == nil {
			t.Fatal("fence did not block legacy OpenSessionsReadOnly")
		}
	})

	t.Run("preset_session_epoch_in_archive_is_still_reminted", func(t *testing.T) {
		ctx := context.Background()
		sourceRoot, sourceVault, sourceFile := seedManagedSourceRoot(t, true)
		attachPublishedManagedState(t, sourceRoot, sourceFile)
		archivePath := filepath.Join(t.TempDir(), "base.adb")
		manifest, err := (Service{StateRoot: sourceRoot, Vault: sourceVault}).Create(ctx, archivePath, "pass", true)
		if err != nil {
			t.Fatal(err)
		}
		_, entries, err := readEncrypted(archivePath, "pass")
		if err != nil {
			t.Fatal(err)
		}
		const presetEpoch int64 = 424242
		sessFile := filepath.Join(t.TempDir(), sessionsName)
		if err := os.WriteFile(sessFile, entries[sessionsName], 0o600); err != nil {
			t.Fatal(err)
		}
		sdb := openRawSQLite(t, sessFile, "rw")
		execRaw(t, sdb, `CREATE TABLE IF NOT EXISTS session_index_generation(singleton INTEGER PRIMARY KEY CHECK(singleton=1), epoch INTEGER NOT NULL CHECK(epoch>0))`)
		execRaw(t, sdb, `INSERT OR REPLACE INTO session_index_generation VALUES(1, ?)`, presetEpoch)
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
		entries[sessionsName], err = os.ReadFile(sessFile)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Entries = nil
		for _, entryName := range manifest.Included {
			sum := sha256.Sum256(entries[entryName])
			manifest.Entries = append(manifest.Entries, Entry{
				Name:   entryName,
				Size:   int64(len(entries[entryName])),
				SHA256: fmt.Sprintf("%x", sum),
			})
		}
		entries[manifestName], err = json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		repacked := filepath.Join(t.TempDir(), "preset-epoch.adb")
		if err := writeEncrypted(repacked, "pass", entries, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		targetRoot := filepath.Join(t.TempDir(), "restored-preset")
		if _, err := Restore(ctx, repacked, targetRoot, "pass", syntheticMachineIdentity("target-preset")); err != nil {
			t.Fatalf("Restore with preset epoch: %v", err)
		}
		assertRestoredRootEntries(t, targetRoot, coreName, "credential.key", "session-index-v1.sqlite3", sessionsName)
		dstManaged, err := store.ResolveCJSIRuntimeRoot(ctx, targetRoot)
		if err != nil || dstManaged == nil {
			t.Fatalf("ResolveCJSIRuntimeRoot: %v %v", dstManaged, err)
		}
		if err := dstManaged.WithPublishedCore(ctx, func(_ *sql.Tx, stamp store.CJSIIndexStamp) error {
			if stamp.IndexEpoch == presetEpoch || stamp.IndexEpoch <= 0 {
				return fmt.Errorf("preset index epoch was not reminted: got %d", stamp.IndexEpoch)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCJSIManagedBackupRejections(t *testing.T) {
	t.Run("intermediate_layout_state_refused", func(t *testing.T) {
		ctx := context.Background()
		root, vault, _ := seedManagedSourceRoot(t, false)
		coreDB := openRawSQLite(t, filepath.Join(root, "agentdeck.sqlite3"), "rw")
		execRaw(t, coreDB, `UPDATE cjsi_layout_journal SET phase=1, fence_identity=NULL`)
		dst := filepath.Join(t.TempDir(), "fail.adb")
		if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, dst, "pass", true); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("intermediate layout Create err = %v, want ErrInvalidArchive", err)
		}
		if _, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("archive published on intermediate layout: %v", err)
		}
	})

	t.Run("unsealed_or_missing_active_slot_refused", func(t *testing.T) {
		ctx := context.Background()
		root, vault, sourceFile := seedManagedSourceRoot(t, true)
		slotFile, _, _ := attachPublishedManagedState(t, root, sourceFile)

		// Corrupt active slot file.
		if err := os.WriteFile(slotFile, []byte("corrupted-slot-bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(t.TempDir(), "corrupt-slot.adb")
		if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, dst, "pass", true); err == nil {
			t.Fatal("Create succeeded with corrupted active slot")
		}
		if _, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("archive published with corrupted active slot: %v", err)
		}

		// Remove active slot file.
		if err := os.Remove(slotFile); err != nil {
			t.Fatal(err)
		}
		if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, dst, "pass", true); err == nil {
			t.Fatal("Create succeeded with missing active slot")
		}
	})

	t.Run("non_empty_target_rejected", func(t *testing.T) {
		ctx := context.Background()
		root, vault, _ := seedManagedSourceRoot(t, false)
		archivePath := filepath.Join(t.TempDir(), "valid.adb")
		if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archivePath, "pass", true); err != nil {
			t.Fatal(err)
		}
		target := t.TempDir()
		existing := filepath.Join(target, "keep.txt")
		if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Restore(ctx, archivePath, target, "pass", syntheticMachineIdentity("t")); !errors.Is(err, ErrTargetNotEmpty) {
			t.Fatalf("non-empty Restore err = %v, want ErrTargetNotEmpty", err)
		}
		if data, err := os.ReadFile(existing); err != nil || string(data) != "keep" {
			t.Fatalf("existing target file modified: %q %v", data, err)
		}
	})

	t.Run("tampered_managed_archive_rejected", func(t *testing.T) {
		ctx := context.Background()
		root, vault, sourceFile := seedManagedSourceRoot(t, true)
		attachPublishedManagedState(t, root, sourceFile)
		archivePath := filepath.Join(t.TempDir(), "valid.adb")
		manifest, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archivePath, "pass", true)
		if err != nil {
			t.Fatal(err)
		}
		_, origEntries, err := readEncrypted(archivePath, "pass")
		if err != nil {
			t.Fatal(err)
		}

		repack := func(name string, mutate func(entries map[string][]byte)) string {
			t.Helper()
			entries := make(map[string][]byte, len(origEntries))
			for k, v := range origEntries {
				cp := make([]byte, len(v))
				copy(cp, v)
				entries[k] = cp
			}
			mutate(entries)
			m := manifest
			m.Entries = nil
			for _, entryName := range m.Included {
				sum := sha256.Sum256(entries[entryName])
				m.Entries = append(m.Entries, Entry{
					Name:   entryName,
					Size:   int64(len(entries[entryName])),
					SHA256: fmt.Sprintf("%x", sum),
				})
			}
			mb, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			entries[manifestName] = mb
			out := filepath.Join(t.TempDir(), name+".adb")
			if err := writeEncrypted(out, "pass", entries, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			return out
		}

		// Case 1: inject legacy trigger into schema-37 core.
		tamperedTrigger := repack("trigger", func(entries map[string][]byte) {
			p := filepath.Join(t.TempDir(), coreName)
			if err := os.WriteFile(p, entries[coreName], 0o600); err != nil {
				t.Fatal(err)
			}
			db := openRawSQLite(t, p, "rw")
			execRaw(t, db, `CREATE TRIGGER telemetry_jsonl_retracts_otel AFTER INSERT ON usage_events BEGIN SELECT 1; END`)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			entries[coreName] = b
		})
		target1 := filepath.Join(t.TempDir(), "t1")
		if _, err := Restore(ctx, tamperedTrigger, target1, "pass", syntheticMachineIdentity("t1")); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("tampered trigger Restore err = %v, want ErrInvalidArchive", err)
		}
		if _, err := os.Stat(target1); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("target1 created on tampered archive: %v", err)
		}

		// Case 2: break portable_session_order ordinal continuity.
		tamperedOrder := repack("order", func(entries map[string][]byte) {
			p := filepath.Join(t.TempDir(), sessionsName)
			if err := os.WriteFile(p, entries[sessionsName], 0o600); err != nil {
				t.Fatal(err)
			}
			db := openRawSQLite(t, p, "rw")
			execRaw(t, db, `UPDATE portable_session_order SET detail_ordinal = 9 WHERE doc_id = 4`)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			entries[sessionsName] = b
		})
		target2 := filepath.Join(t.TempDir(), "t2")
		if _, err := Restore(ctx, tamperedOrder, target2, "pass", syntheticMachineIdentity("t2")); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("tampered order Restore err = %v, want ErrInvalidArchive", err)
		}
		if _, err := os.Stat(target2); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("target2 created on tampered archive: %v", err)
		}
	})

	t.Run("preserves_concurrent_target_file_when_locked_or_unlocked", func(t *testing.T) {
		ctx := context.Background()
		root, vault, _ := seedManagedSourceRoot(t, false)
		archive := filepath.Join(t.TempDir(), "archive.adb")
		if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false); err != nil {
			t.Fatal(err)
		}

		// Variant 1 (locked): concurrent file appears after scan.lock is acquired,
		// before empty-target validation; Restore must return ErrTargetNotEmpty
		// and preserve the concurrent owner's file.
		t.Run("post_scan_lock", func(t *testing.T) {
			target := t.TempDir()
			other := filepath.Join(target, "concurrent-owner.txt")
			physicalTarget, err := filepath.EvalSymlinks(target)
			if err != nil {
				t.Fatal(err)
			}
			injected := false
			afterManagedRestoreScanLock = func(dir string) {
				if dir != physicalTarget {
					return
				}
				if err := os.WriteFile(other, []byte("other owner"), 0o600); err != nil {
					t.Fatal(err)
				}
				injected = true
			}
			defer func() { afterManagedRestoreScanLock = nil }()

			_, err = Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target-locked"))
			if !injected || !errors.Is(err, ErrTargetNotEmpty) {
				t.Fatalf("expected post-lock ErrTargetNotEmpty, injected=%v err=%v", injected, err)
			}
			if got, err := os.ReadFile(other); err != nil || string(got) != "other owner" {
				t.Fatalf("restore removed or changed concurrent owner's file: %q %v", got, err)
			}
		})

		t.Run("locked_empty_check_runs_before_core_write", func(t *testing.T) {
			target := t.TempDir()
			other := filepath.Join(target, coreName)
			afterManagedRestoreScanLock = func(dir string) {
				if err := os.WriteFile(filepath.Join(dir, coreName), []byte("foreign core"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			defer func() { afterManagedRestoreScanLock = nil }()

			_, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target-locked"))
			if !errors.Is(err, ErrTargetNotEmpty) || errors.Is(err, fs.ErrExist) {
				t.Fatalf("expected locked emptiness rejection before opening core for write: %v", err)
			}
			if got, err := os.ReadFile(other); err != nil || string(got) != "foreign core" {
				t.Fatalf("restore changed concurrent owner's core: %q %v", got, err)
			}
		})

		t.Run("during_scan_lock_wait", func(t *testing.T) {
			target := t.TempDir()
			waiting := make(chan struct{})
			originalAcquire := acquireManagedRestoreScanLock
			acquireManagedRestoreScanLock = func(ctx context.Context, dir string, timeout time.Duration) (*store.Lock, error) {
				held, err := store.AcquireScanLock(ctx, dir, time.Second)
				if err != nil {
					return nil, err
				}
				t.Cleanup(func() {
					if err := held.Release(); err != nil {
						t.Error(err)
					}
				})
				return originalAcquire(&restoreWaitContext{Context: ctx, waiting: waiting}, dir, timeout)
			}
			defer func() { acquireManagedRestoreScanLock = originalAcquire }()
			restoreCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := Restore(restoreCtx, archive, target, "pass", syntheticMachineIdentity("target-wait"))
				result <- err
			}()

			select {
			case <-waiting:
			case err := <-result:
				t.Fatalf("restore exited before waiting for scan.lock: %v", err)
			case <-time.After(30 * time.Second):
				cancel()
				<-result
				t.Fatal("restore did not reach the scan.lock wait")
			}
			other := filepath.Join(target, "concurrent-owner.txt")
			injectErr := os.WriteFile(other, []byte("other owner"), 0o600)
			cancel()
			if err := <-result; !errors.Is(err, context.Canceled) {
				t.Fatalf("restore did not return scan lock cancellation: %v", err)
			}
			if injectErr != nil {
				t.Fatal(injectErr)
			}
			if got, err := os.ReadFile(other); err != nil || string(got) != "other owner" {
				t.Fatalf("restore changed concurrent owner's file: %q %v", got, err)
			}
			if !store.LockOwnedByCurrentProcess(filepath.Join(target, "scan.lock")) {
				t.Fatal("restore removed competing scan.lock")
			}
		})

		// Variant 3 (mid-install failure): after Restore writes agentdeck.sqlite3,
		// a concurrent owner's file appears and credential-key initialization fails;
		// Restore rolls back only its own files and preserves concurrent-owner.txt.
		t.Run("mid_install_rollback_only_owned", func(t *testing.T) {
			target := t.TempDir()
			other := filepath.Join(target, "concurrent-owner.txt")
			origInit := initializeRestoreCredentialKey
			initializeRestoreCredentialKey = func(_ context.Context, _ *credentialvault.Vault) (bool, error) {
				if err := os.WriteFile(other, []byte("other owner"), 0o600); err != nil {
					t.Fatal(err)
				}
				return false, errors.New("injected credential key failure")
			}
			defer func() { initializeRestoreCredentialKey = origInit }()

			if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target-mid")); err == nil {
				t.Fatal("restore unexpectedly succeeded after injected key failure")
			}
			if got, err := os.ReadFile(other); err != nil || string(got) != "other owner" {
				t.Fatalf("rollback removed or changed concurrent owner's file: %q %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(target, coreName)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("rollback left owned core database behind: %v", err)
			}
		})
	})
}

// AcquireScanLock consults Done in its contention select. The test passes
// this context only after it has acquired the competing lock.
type restoreWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *restoreWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestCJSIManagedBackupRollbackOwnership(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, false)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected credential key failure")
	for _, tc := range []struct {
		name     string
		filename string
		inject   func(*testing.T, string, string) error
		assert   func(*testing.T, string, string)
	}{
		{
			name: "same_name_replacement_of_owned_core_preserved", filename: coreName,
			inject: func(t *testing.T, path, _ string) error {
				foreign := filepath.Join(t.TempDir(), "foreign-core")
				if err := os.WriteFile(foreign, []byte("foreign core"), 0o600); err != nil {
					return err
				}
				return os.Rename(foreign, path)
			},
			assert: func(t *testing.T, path, _ string) {
				if got, err := os.ReadFile(path); err != nil || string(got) != "foreign core" {
					t.Fatalf("rollback changed foreign core: %q %v", got, err)
				}
			},
		},
		{
			name: "symlink_replacement_of_owned_core_preserved", filename: coreName,
			inject: func(_ *testing.T, path, outside string) error {
				if err := os.Remove(path); err != nil {
					return err
				}
				return os.Symlink(outside, path)
			},
			assert: func(t *testing.T, path, outside string) {
				if got, err := os.Readlink(path); err != nil || got != outside {
					t.Fatalf("rollback changed foreign symlink: %q %v", got, err)
				}
			},
		},
		{
			name: "foreign_file_with_reserved_layout_name_preserved", filename: "session-index-v1.sqlite3",
			inject: func(_ *testing.T, path, _ string) error {
				return os.WriteFile(path, []byte("other owner"), 0o600)
			},
			assert: func(t *testing.T, path, _ string) {
				if got, err := os.ReadFile(path); err != nil || string(got) != "other owner" {
					t.Fatalf("rollback changed foreign reserved file: %q %v", got, err)
				}
			},
		},
		{
			name: "foreign_empty_dir_with_reserved_layout_name_preserved", filename: ".cjsi-layout-private.sqlite3-wal",
			inject: func(_ *testing.T, path, _ string) error {
				return os.Mkdir(path, 0o700)
			},
			assert: func(t *testing.T, path, _ string) {
				if info, err := os.Lstat(path); err != nil || !info.IsDir() {
					t.Fatalf("rollback changed foreign reserved directory: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := t.TempDir()
			outside := filepath.Join(t.TempDir(), "victim.txt")
			if err := os.WriteFile(outside, []byte("victim"), 0o600); err != nil {
				t.Fatal(err)
			}
			originalInit := initializeRestoreCredentialKey
			initializeRestoreCredentialKey = func(_ context.Context, v *credentialvault.Vault) (bool, error) {
				if err := tc.inject(t, filepath.Join(filepath.Dir(v.KeyPath()), tc.filename), outside); err != nil {
					t.Fatal(err)
				}
				return false, injectedErr
			}
			defer func() { initializeRestoreCredentialKey = originalInit }()
			if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target")); !errors.Is(err, injectedErr) {
				t.Fatalf("restore did not reach injected failure: %v", err)
			}
			tc.assert(t, filepath.Join(target, tc.filename), outside)
			if got, err := os.ReadFile(outside); err != nil || string(got) != "victim" {
				t.Fatalf("rollback touched symlink target: %q %v", got, err)
			}
		})
	}

	t.Run("replaced_created_root_is_preserved", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "new-root")
		moved := filepath.Join(parent, "moved-root")
		var replacement os.FileInfo
		originalInit := initializeRestoreCredentialKey
		initializeRestoreCredentialKey = func(context.Context, *credentialvault.Vault) (bool, error) {
			if err := os.Rename(target, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			replacement, err = os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			return false, injectedErr
		}
		defer func() { initializeRestoreCredentialKey = originalInit }()
		if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target")); !errors.Is(err, injectedErr) {
			t.Fatalf("restore did not reach the root replacement: %v", err)
		}
		info, err := os.Lstat(target)
		if err != nil || replacement == nil || !info.IsDir() || !os.SameFile(replacement, info) {
			t.Fatalf("rollback removed the replacement directory: %v", err)
		}
		assertRestoredRootEntries(t, target)
	})

	for _, name := range []string{coreName, "credential.key"} {
		t.Run("replaced_before_ownership_record_"+name, func(t *testing.T) {
			target := t.TempDir()
			foreign := filepath.Join(t.TempDir(), "replacement")
			var foreignInfo os.FileInfo
			originalHook := beforeManagedRestoreFileRecord
			beforeManagedRestoreFileRecord = func(path string) {
				if filepath.Base(path) != name {
					return
				}
				// Give even a replacement key the same bytes. Inode identity,
				// rather than a content difference, must reject this entry.
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(foreign, contents, 0o600); err != nil {
					t.Fatal(err)
				}
				foreignInfo, err = os.Lstat(foreign)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(foreign, path); err != nil {
					t.Fatal(err)
				}
			}
			defer func() { beforeManagedRestoreFileRecord = originalHook }()
			_, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target"))
			if err == nil || !strings.Contains(err.Error(), "ownership of restored file") {
				t.Fatalf("restore did not report unproved creation identity: %v", err)
			}
			info, err := os.Lstat(filepath.Join(target, name))
			if err != nil || foreignInfo == nil || !os.SameFile(foreignInfo, info) {
				t.Fatalf("restore removed the replacement inode: %v", err)
			}
			assertRestoredRootEntries(t, target, managedRestoreMarkerName, name)
		})
	}

	t.Run("modified_key_with_same_inode_preserved", func(t *testing.T) {
		target := t.TempDir()
		keyPath := credentialvault.New(target, nil).KeyPath()
		const replacement = "updated by a different owner"
		var before os.FileInfo
		calls := 0
		machine := func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "test-machine", nil
			}
			var err error
			before, err = os.Lstat(keyPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(keyPath, []byte(replacement), 0o600); err != nil {
				t.Fatal(err)
			}
			return "", injectedErr
		}
		if _, err := Restore(ctx, archive, target, "pass", machine); !errors.Is(err, credentialvault.ErrMachineIdentityMissing) || calls != 2 {
			t.Fatalf("restore did not reach key mutation after recording its digest: calls=%d err=%v", calls, err)
		}
		after, err := os.Lstat(keyPath)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("rollback removed or replaced the modified key: %v", err)
		}
		if got, err := os.ReadFile(keyPath); err != nil || string(got) != replacement {
			t.Fatalf("rollback changed the modified key: %q %v", got, err)
		}
	})

	t.Run("unprovable_key_ownership_is_reported", func(t *testing.T) {
		target := t.TempDir()
		outside := t.TempDir()
		originalInit := initializeRestoreCredentialKey
		initializeRestoreCredentialKey = func(_ context.Context, v *credentialvault.Vault) (bool, error) {
			if err := os.Symlink(outside, v.KeyPath()); err != nil {
				t.Fatal(err)
			}
			return true, injectedErr
		}
		defer func() { initializeRestoreCredentialKey = originalInit }()
		_, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target"))
		if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "ownership of restored file") {
			t.Fatalf("expected original failure and unproved ownership: %v", err)
		}
		if got, err := os.Readlink(credentialvault.New(target, nil).KeyPath()); err != nil || got != outside {
			t.Fatalf("rollback changed entry without ownership proof: %q %v", got, err)
		}
	})
}

func TestCJSIManagedBackupRollbackLayoutPublication(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, true)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", true); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "new-root")
	foreign := filepath.Join(target, coreName+"-journal")
	var foreignScratch string
	originalInit := initializeRestoreCredentialKey
	initializeRestoreCredentialKey = func(ctx context.Context, v *credentialvault.Vault) (bool, error) {
		if err := os.WriteFile(foreign, []byte("foreign journal"), 0o600); err != nil {
			t.Fatal(err)
		}
		scratch, err := filepath.Glob(filepath.Join(target, ".cjsi-restore-*"))
		scratch = slices.DeleteFunc(scratch, func(path string) bool {
			return filepath.Base(path) == managedRestoreMarkerName
		})
		if err != nil || len(scratch) != 1 {
			t.Fatalf("restore scratch directory: %v %v", scratch, err)
		}
		foreignScratch = filepath.Join(scratch[0], "foreign.txt")
		if err := os.WriteFile(foreignScratch, []byte("foreign scratch entry"), 0o600); err != nil {
			t.Fatal(err)
		}
		return originalInit(ctx, v)
	}
	defer func() { initializeRestoreCredentialKey = originalInit }()

	if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target")); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("expected exclusive core sidecar reservation to refuse collision: %v", err)
	}
	remaining, err := os.ReadDir(target)
	wantRoot := []string{filepath.Base(filepath.Dir(foreignScratch)), managedRestoreMarkerName, filepath.Base(foreign)}
	slices.Sort(wantRoot)
	gotRoot := make([]string, 0, len(remaining))
	for _, entry := range remaining {
		gotRoot = append(gotRoot, entry.Name())
	}
	if err != nil || !slices.Equal(gotRoot, wantRoot) {
		t.Fatalf("rollback left restore-owned artifacts or removed the foreign sidecar: %v %v", remaining, err)
	}
	scratchEntries, err := os.ReadDir(filepath.Dir(foreignScratch))
	if err != nil || len(scratchEntries) != 1 || scratchEntries[0].Name() != "foreign.txt" {
		t.Fatalf("rollback must remove only its scratch files: %v %v", scratchEntries, err)
	}
	if got, err := os.ReadFile(foreignScratch); err != nil || string(got) != "foreign scratch entry" {
		t.Fatalf("rollback changed foreign scratch entry: %q %v", got, err)
	}
	if got, err := os.ReadFile(foreign); err != nil || string(got) != "foreign journal" {
		t.Fatalf("rollback changed foreign sidecar: %q %v", got, err)
	}
}

func TestCJSIManagedBackupPreservesReplacedReservedSidecar(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, true)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", true); err != nil {
		t.Fatal(err)
	}

	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		t.Run(suffix[1:], func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "target")
			sidecarName := coreName + suffix
			replacement := filepath.Join(t.TempDir(), "foreign-sidecar")
			const data = "foreign sidecar"
			if err := os.WriteFile(replacement, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			foreign, err := os.Lstat(replacement)
			if err != nil {
				t.Fatal(err)
			}

			injected := false
			originalProbeHook := afterManagedRestoreLayoutProbe
			afterManagedRestoreLayoutProbe = func(root string) {
				path := filepath.Join(root, sidecarName)
				reserved, err := os.Lstat(path)
				if err != nil || !reserved.Mode().IsRegular() || os.SameFile(reserved, foreign) {
					t.Fatalf("missing restore reservation for %s: %v", sidecarName, err)
				}
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
				injected = true
			}
			defer func() { afterManagedRestoreLayoutProbe = originalProbeHook }()

			_, err = Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target"))
			if !injected || err == nil {
				t.Errorf("expected rollback after replacing a probed sidecar, injected=%v err=%v", injected, err)
			}
			sidecar := filepath.Join(target, sidecarName)
			info, err := os.Lstat(sidecar)
			if err != nil || !os.SameFile(info, foreign) {
				t.Fatalf("cleanup removed or replaced the foreign sidecar inode: %v", err)
			}
			if got, err := os.ReadFile(sidecar); err != nil || string(got) != data {
				t.Fatalf("cleanup changed the foreign sidecar: %q, %v", got, err)
			}
			assertRestoredRootEntries(t, target, managedRestoreMarkerName, sidecarName)
		})
	}
}

func TestCJSIManagedBackupRestoreMarker(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, false)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false); err != nil {
		t.Fatal(err)
	}

	injected := errors.New("injected restore failure")
	for _, residue := range []bool{false, true} {
		name := "complete_rollback_clears_marker"
		if residue {
			name = "residue_keeps_admission_closed"
		}
		t.Run(name, func(t *testing.T) {
			target := t.TempDir()
			const foreignName = "foreign"
			sawMarker := false
			origRecordHook := beforeManagedRestoreFileRecord
			beforeManagedRestoreFileRecord = func(path string) {
				if filepath.Base(path) != coreName {
					return
				}
				root := filepath.Dir(path)
				if err := store.CheckStateRestoreInProgress(root); !errors.Is(err, store.ErrStateRestoreInProgress) {
					t.Fatalf("first core write was not fenced by the restore marker: %v", err)
				}
				for _, lock := range []string{"scan.lock", "state.lock"} {
					if !store.LockOwnedByCurrentProcess(filepath.Join(root, lock)) {
						t.Fatalf("first core write did not hold %s", lock)
					}
				}
				sawMarker = true
			}
			defer func() { beforeManagedRestoreFileRecord = origRecordHook }()
			origInit := initializeRestoreCredentialKey
			initializeRestoreCredentialKey = func(context.Context, *credentialvault.Vault) (bool, error) {
				if residue {
					if err := os.WriteFile(filepath.Join(target, foreignName), []byte("other owner"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return false, injected
			}
			defer func() { initializeRestoreCredentialKey = origInit }()
			if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target")); !sawMarker || !errors.Is(err, injected) {
				t.Fatalf("restore did not reach the injected failure while marked: observed=%v err=%v", sawMarker, err)
			}
			if !residue {
				if err := store.CheckStateRestoreInProgress(target); err != nil {
					t.Fatalf("complete rollback left the admission marker: %v", err)
				}
				assertRestoredRootEntries(t, target)
				return
			}

			assertRestoredRootEntries(t, target, managedRestoreMarkerName, foreignName)
			for _, open := range []struct {
				name string
				call func(context.Context, string) (*store.Store, error)
			}{
				{"Open", store.Open},
				{"OpenReadOnly", store.OpenReadOnly},
				{"OpenSessions", store.OpenSessions},
				{"OpenSessionsReadOnly", store.OpenSessionsReadOnly},
			} {
				db, err := open.call(ctx, target)
				if db != nil {
					_ = db.Close()
				}
				if db != nil || !errors.Is(err, store.ErrStateRestoreInProgress) {
					t.Errorf("%s admitted the marked root: %v", open.name, err)
				}
			}
			if err := store.EnsureStateMigrated(ctx, target); !errors.Is(err, store.ErrStateRestoreInProgress) {
				t.Errorf("EnsureStateMigrated admitted the marked root: %v", err)
			}
			if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("retry")); !errors.Is(err, store.ErrStateRestoreInProgress) {
				t.Errorf("second Restore admitted the marked root: %v", err)
			}
			assertRestoredRootEntries(t, target, managedRestoreMarkerName, foreignName)
		})
	}

	t.Run("ordinary_entrypoints_reject_until_clear", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "target")
		probed := false
		origProbeHook := afterManagedRestoreLayoutProbe
		afterManagedRestoreLayoutProbe = func(root string) {
			if managed, err := store.ResolveCJSIRuntimeRoot(ctx, root); managed != nil || !errors.Is(err, store.ErrStateRestoreInProgress) {
				t.Errorf("ordinary resolver admitted the published root before Clear: %v", err)
			}
			database, err := store.OpenWithLockHeld(ctx, root)
			if database != nil {
				_ = database.Close()
			}
			if database != nil || !errors.Is(err, store.ErrStateRestoreInProgress) {
				t.Errorf("ordinary locked opener admitted the published root before Clear: %v", err)
			}
			probed = true
		}
		defer func() { afterManagedRestoreLayoutProbe = origProbeHook }()
		if _, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target")); err != nil || !probed {
			t.Fatalf("restore did not publish under the marker: probed=%v err=%v", probed, err)
		}
		assertRestoredRootEntries(t, target, coreName, "credential.key", sessionsName)
		if managed, err := store.ResolveCJSIRuntimeRoot(ctx, target); err != nil || managed == nil {
			t.Fatalf("ordinary resolver after Clear: %v", err)
		}
	})

	for _, residue := range []bool{false, true} {
		name := "probe_failure_clears_after_complete_rollback"
		if residue {
			name = "probe_failure_keeps_marker_when_residue_remains"
		}
		t.Run(name, func(t *testing.T) {
			target := t.TempDir()
			const foreignName = "foreign"
			restoreCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			coreRecords := 0
			origRecordHook := beforeManagedRestoreFileRecord
			beforeManagedRestoreFileRecord = func(path string) {
				if filepath.Base(path) != coreName {
					return
				}
				coreRecords++
				if coreRecords != 2 {
					return
				}
				// The second record follows the final core link. The owner
				// probe is the next operation that consumes the context.
				if residue {
					if err := os.WriteFile(filepath.Join(target, foreignName), []byte("other owner"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				cancel()
			}
			defer func() { beforeManagedRestoreFileRecord = origRecordHook }()

			probed := false
			origProbeHook := afterManagedRestoreLayoutProbe
			afterManagedRestoreLayoutProbe = func(string) { probed = true }
			defer func() { afterManagedRestoreLayoutProbe = origProbeHook }()
			_, err := Restore(restoreCtx, archive, target, "pass", syntheticMachineIdentity("target"))
			if coreRecords != 2 || probed || !errors.Is(err, context.Canceled) {
				t.Fatalf("expected owner probe cancellation after publication: records=%d probed=%v err=%v", coreRecords, probed, err)
			}
			if residue {
				assertRestoredRootEntries(t, target, managedRestoreMarkerName, foreignName)
				if err := store.EnsureStateMigrated(ctx, target); !errors.Is(err, store.ErrStateRestoreInProgress) {
					t.Fatalf("migration admitted a target after incomplete probe rollback: %v", err)
				}
			} else {
				assertRestoredRootEntries(t, target)
				if err := store.CheckStateRestoreInProgress(target); err != nil {
					t.Fatalf("complete probe rollback left the marker: %v", err)
				}
			}
		})
	}
}

func TestCJSIManagedBackupFailedMarkOwnership(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, false)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false); err != nil {
		t.Fatal(err)
	}
	machine := syntheticMachineIdentity("target")

	t.Run("complete_binding_clears_and_allows_retry", func(t *testing.T) {
		target := t.TempDir()
		injected := errors.New("injected failure after marker directory sync")
		originalMark := markManagedRestoreInProgress
		calls := 0
		markManagedRestoreInProgress = func(ctx context.Context, root string) error {
			if err := originalMark(ctx, root); err != nil {
				return err
			}
			calls++
			if calls == 1 {
				return injected
			}
			return nil
		}
		defer func() { markManagedRestoreInProgress = originalMark }()
		if _, err := Restore(ctx, archive, target, "pass", machine); !errors.Is(err, injected) {
			t.Fatalf("first restore did not return the failed Mark: %v", err)
		}
		assertRestoredRootEntries(t, target)
		if _, err := Restore(ctx, archive, target, "pass", machine); err != nil || calls != 2 {
			t.Fatalf("retry could not use the rolled back root: calls=%d err=%v", calls, err)
		}
		assertRestoredRootEntries(t, target, coreName, "credential.key", sessionsName)
	})

	t.Run("partial_binding_remains_blocked", func(t *testing.T) {
		target := t.TempDir()
		injected := errors.New("injected failure after partial marker write")
		originalMark := markManagedRestoreInProgress
		var partial []byte
		markManagedRestoreInProgress = func(ctx context.Context, root string) error {
			if err := originalMark(ctx, root); err != nil {
				return err
			}
			path := filepath.Join(root, managedRestoreMarkerName)
			full, err := os.ReadFile(path)
			if err != nil || len(full) < 2 {
				t.Fatalf("full marker unavailable for partial-write fixture: %v", err)
			}
			partial = full[:len(full)/2]
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(file.Truncate(int64(len(partial))), file.Sync(), file.Close()); err != nil {
				t.Fatal(err)
			}
			return injected
		}
		defer func() { markManagedRestoreInProgress = originalMark }()

		if _, err := Restore(ctx, archive, target, "pass", machine); !errors.Is(err, injected) {
			t.Fatalf("restore did not return the partial Mark failure: %v", err)
		}
		assertRestoredRootEntries(t, target, managedRestoreMarkerName)
		got, err := os.ReadFile(filepath.Join(target, managedRestoreMarkerName))
		if err != nil || !bytes.Equal(got, partial) {
			t.Fatalf("rollback changed the partial marker: %v", err)
		}
		if _, err := Restore(ctx, archive, target, "pass", machine); !errors.Is(err, store.ErrStateRestoreInProgress) {
			t.Fatalf("retry admitted a root with a partial marker: %v", err)
		}
	})
}

func TestCJSIManagedBackupClearMarkerOwnership(t *testing.T) {
	ctx := context.Background()
	root, vault, _ := seedManagedSourceRoot(t, false)
	archive := filepath.Join(t.TempDir(), "archive.adb")
	if _, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false); err != nil {
		t.Fatal(err)
	}

	for _, changed := range []string{managedRestoreMarkerName, "scan.lock", "state.lock"} {
		t.Run(changed, func(t *testing.T) {
			target := t.TempDir()
			displaced := filepath.Join(t.TempDir(), "displaced")
			var wantMarker os.FileInfo
			var contents []byte
			originalHook := beforeManagedRestoreMarkerClear
			beforeManagedRestoreMarkerClear = func(root string) {
				markerPath := filepath.Join(root, managedRestoreMarkerName)
				var err error
				wantMarker, err = os.Lstat(markerPath)
				if err != nil {
					t.Fatal(err)
				}
				contents, err = os.ReadFile(markerPath)
				if err != nil {
					t.Fatal(err)
				}
				if changed != managedRestoreMarkerName {
					if err := os.Rename(filepath.Join(root, changed), displaced); err != nil {
						t.Fatal(err)
					}
					return
				}

				if err := os.WriteFile(displaced, contents, 0o600); err != nil {
					t.Fatal(err)
				}
				foreign, err := os.Lstat(displaced)
				if err != nil || os.SameFile(wantMarker, foreign) {
					t.Fatalf("replacement marker is not a different inode: %v", err)
				}
				if err := os.Rename(displaced, markerPath); err != nil {
					t.Fatal(err)
				}
				wantMarker = foreign
			}
			defer func() { beforeManagedRestoreMarkerClear = originalHook }()

			_, err := Restore(ctx, archive, target, "pass", syntheticMachineIdentity("target"))
			if changed == managedRestoreMarkerName {
				if err == nil || !strings.Contains(err.Error(), "restore marker identity changed") {
					t.Errorf("expected marker identity refusal: %v", err)
				}
			} else if !errors.Is(err, store.ErrStateRestoreInProgress) {
				t.Errorf("expected marker clear to refuse lost %s: %v", changed, err)
			}
			markerPath := filepath.Join(target, managedRestoreMarkerName)
			got, err := os.Lstat(markerPath)
			if err != nil || wantMarker == nil || !os.SameFile(wantMarker, got) {
				t.Fatalf("Clear removed or replaced the marker: %v", err)
			}
			data, err := os.ReadFile(markerPath)
			if err != nil || !bytes.Equal(data, contents) {
				t.Fatalf("Clear changed the retained marker: %v", err)
			}
			assertRestoredRootEntries(t, target, managedRestoreMarkerName, coreName, "credential.key", sessionsName)
		})
	}
}

func TestCJSIManagedBackupAcceptsPublishedSessionRevision(t *testing.T) {
	ctx := t.Context()
	root, home := t.TempDir(), t.TempDir()
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.InstallTask5Fixture(ctx, root); err != nil {
		t.Fatal(err)
	}
	managed, err := store.ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil || managed == nil {
		t.Fatalf("resolve fixture root: %v", err)
	}
	const thread = "019c0000-0000-7000-8000-000000005560"
	dir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.jsonl")
	body := []byte(`{"timestamp":"2026-07-14T00:00:00Z","type":"session_meta","payload":{"id":"` + thread + `","cwd":"/synthetic/work"}}
{"timestamp":"2026-07-14T00:00:01Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}
{"timestamp":"2026-07-14T00:00:02Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"backup publication fixture"}]}}
{"timestamp":"2026-07-14T00:00:03Z","type":"token_usage_record","payload":{"thread_id":"` + thread + `","turn_id":"t1","response_id":"r1","usage":{"input_tokens":13,"output_tokens":5}}}
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	inventory, err := ingest.Discover(home)
	if err != nil || len(inventory) != 1 {
		t.Fatalf("fixture inventory: %v, %v", inventory, err)
	}
	source := inventory[0]
	file, err := os.Open(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	reduction, err := ingest.CaptureCodexReduction(ctx, file, source.Path, source.Size, 1<<16)
	if err := errors.Join(err, file.Close()); err != nil {
		t.Fatal(err)
	}
	evidence := reduction.Capture().Evidence()
	evidence.Version = ingest.CodexCheckpointVersion{Generation: 1, Parser: cjsiledger.Parser, MaxLineBytes: 1 << 16}
	selected := ingest.CodexSelectedSource{Path: source.Path, ThreadID: thread, Epoch: 1, Evidence: evidence}
	now := time.Now().UTC()
	digest := sha256.Sum256(body)
	verifyEmpty := func(ctx context.Context, tx *sql.Tx) error {
		var value int
		return tx.QueryRowContext(ctx, "SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM usage_events)").Scan(&value)
	}
	verifyOpened := func(ctx context.Context, tx *sql.Tx) error {
		var value int
		return tx.QueryRowContext(ctx, "SELECT 1 FROM usage_events WHERE session_id=? AND input_tokens=13 AND output_tokens=5", thread).Scan(&value)
	}
	var baseRevision, coverageRevision, usageRevision int64
	if err := managed.WithPublishedCore(ctx, func(tx *sql.Tx, stamp store.CJSIIndexStamp) error {
		baseRevision, coverageRevision = stamp.SessionRevision, stamp.CoverageRevision
		return tx.QueryRowContext(ctx, "SELECT revision FROM serving_generation WHERE singleton=1").Scan(&usageRevision)
	}); err != nil {
		t.Fatal(err)
	}
	planner := scanruntime.CJSIPlanner{Select: func(context.Context, scanruntime.CJSIRoundRequest, scanruntime.Scope) (scanruntime.CJSIRoundSelection, error) {
		return scanruntime.CJSIRoundSelection{
			Inputs: store.CJSIPublicationInputs{Digest: digest, Validate: verifyEmpty},
			Changes: store.CJSICombinedChanges{
				Digest: digest, Validate: verifyEmpty, VerifyOpened: verifyOpened,
				RecheckSources:  func(context.Context) error { return ingest.Validate(source) },
				ApplySelected:   func(context.Context, *sql.Tx) error { return nil },
				SessionRevision: baseRevision + 1, UsageRevision: usageRevision + 4, CoverageRevision: coverageRevision,
			},
			OriginChanges: []scanruntime.CJSIOriginChange{{Input: sessionorigin.OriginChangeInputs{Target: sessionorigin.Target{Client: "codex", SessionID: thread}}}},
			BuiltAt:       now, AcceptUntil: now.Add(time.Minute),
			ValidUntil: func(built time.Time) time.Time { return built.Add(time.Hour) },
			Groups: []scanruntime.CJSIGroupInputs{{
				Thread: thread, History: cjsiledger.History{Assessed: true, Rows: []cjsiledger.OldRow{}},
				Receipts: func(stamp cjsiledger.ProposalStamp) store.CJSIOTelReceiptStore {
					return store.CJSIOTelSQLiteStore{Guard: func(_ context.Context, _ *sql.Tx, thread string) (store.CJSIOTelGuard, error) {
						return store.CJSIOTelGuard{Thread: thread, Stamp: stamp, Revision: 1}, nil
					}}
				},
				Sources: []scanruntime.CJSISourcePreparation{{
					Source: source, Current: func(context.Context) (ingest.CodexSelectedSource, error) { return selected, ingest.Validate(source) },
					Index: session.CJSISourceInput{
						Selected: selected, PrefixHash: fmt.Sprintf("%x", digest), ScannedAt: now.Format(time.RFC3339Nano),
						Priority: source.Priority, Size: source.Size, ModifiedAt: source.ModifiedAt, ChangedAt: source.ChangedAt,
					},
					Limits:    ingest.CodexWorksetLimits{Turns: 32, SegmentRecords: 64},
					MaxMeters: 64, MaxBytes: 1 << 20, StepBytes: 256,
				}},
			}},
			Build: func(context.Context, scanruntime.CJSIProposedView) (func() ([]byte, error), error) {
				return func() ([]byte, error) { return []byte("published backup fixture"), nil }, nil
			},
		}, nil
	}}
	plan, _, err := planner.Combined(ctx, scanruntime.CJSIRoundRequest{Root: *managed, Home: home, Sources: inventory})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Publish(ctx); err != nil {
		t.Fatalf("publish the fixture transcript: %v", err)
	}
	if err := managed.WithPublishedCore(ctx, func(tx *sql.Tx, stamp store.CJSIIndexStamp) error {
		var attemptRevision int64
		if err := tx.QueryRowContext(ctx, `SELECT a.session_revision
			FROM cjsi_publication_head h JOIN cjsi_publication_attempts a
			ON a.ticket=h.slot_ticket AND a.attempt=h.slot_attempt
			WHERE h.singleton=1 AND a.state='opened'`).Scan(&attemptRevision); err != nil {
			return err
		}
		t.Logf("opened attempt revision=%d, published session revision=%d", attemptRevision, stamp.SessionRevision)
		if attemptRevision != baseRevision || stamp.SessionRevision != baseRevision+1 {
			return fmt.Errorf("reservation revision=%d, published revision=%d, want %d and %d",
				attemptRevision, stamp.SessionRevision, baseRevision, baseRevision+1)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	service := Service{StateRoot: root, Vault: testBackupVault(t, "published-backup")}
	for _, includeSessions := range []bool{false, true} {
		name := "core_only"
		if includeSessions {
			name = "include_sessions"
		}
		t.Run(name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "backup.adb")
			manifest, err := service.Create(ctx, archive, "pass", includeSessions)
			if err != nil {
				t.Fatalf("backup of the opened publication: %v", err)
			}
			if _, got := manifest.DatabaseSchemas[sessionsName]; got != includeSessions {
				t.Fatalf("session archive included=%t, want %t", got, includeSessions)
			}
		})
	}
}

func TestCJSIManagedBackupValidatesUnreadableSlot(t *testing.T) {
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		change string
		hidden string
	}{
		{name: "masked_slot_core_backup", hidden: "UPDATE cjsi_scope_overlays SET masked=1"},
		{name: "advanced_protection_core_backup"},
		{name: "slot_seal_mismatch", change: "UPDATE cjsi_publication_attempts SET slot_seal=zeroblob(32) WHERE ticket=1"},
		{name: "session_seal_mismatch", change: "UPDATE cjsi_publication_attempts SET session_seal=zeroblob(32) WHERE ticket=1"},
		{name: "core_epoch_mismatch", change: "UPDATE cjsi_publication_attempts SET core_epoch=core_epoch+1 WHERE ticket=1"},
		{name: "index_epoch_mismatch", change: "UPDATE cjsi_publication_attempts SET index_epoch=index_epoch+1 WHERE ticket=1"},
		{name: "selected_attempt_retired", change: "UPDATE cjsi_publication_attempts SET state='retired' WHERE ticket=1"},
		{name: "empty_validity_window", change: "UPDATE cjsi_publication_attempts SET valid_until=built_at WHERE ticket=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, vault, source := seedManagedSourceRoot(t, true)
			attachPublishedManagedState(t, root, source)
			core := openRawSQLite(t, filepath.Join(root, coreName), "rw")

			// A later core-only publication keeps the earlier session/slot
			// selection. No active usage or takeover references the slot
			// attempt, so the general core preflight cannot mask corruption
			// of the selected attempt.
			execRaw(t, core, `
				DELETE FROM cjsi_receipt_markers;
				DELETE FROM cjsi_receipt_sessions;
				DELETE FROM cjsi_receipt_events;
				DELETE FROM cjsi_receipt_observations;
				DELETE FROM cjsi_takeovers;
				DELETE FROM cjsi_usage_selection;
				UPDATE cjsi_publication_attempts SET state='opened', diff_digest=input_digest WHERE ticket=2;
				UPDATE cjsi_publication_head SET ticket=2 WHERE singleton=1;`)
			if tc.hidden != "" {
				execRaw(t, core, tc.hidden)
			} else {
				execRaw(t, core, "UPDATE cjsi_publication_head SET protection_revision=protection_revision+1")
			}
			if tc.change != "" {
				execRaw(t, core, tc.change)
			}
			if err := core.Close(); err != nil {
				t.Fatal(err)
			}

			archive := filepath.Join(t.TempDir(), "core.adb")
			_, err := (Service{StateRoot: root, Vault: vault}).Create(ctx, archive, "pass", false)
			if tc.change != "" {
				if !errors.Is(err, store.ErrCJSIIndexChanged) {
					t.Fatalf("invalid selected slot: want ErrCJSIIndexChanged, got %v", err)
				}
				if _, err := os.Stat(archive); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("invalid slot published an archive: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid hidden slot must allow core-only backup: %v", err)
			}
			manifest, entries, err := readEncrypted(archive, "pass")
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{coreName, credentialsName}; !slices.Equal(manifest.Included, want) ||
				len(entries[coreName]) == 0 || len(entries) != len(want)+1 {
				t.Fatalf("unexpected core-only archive contents: %v", manifest.Included)
			}
		})
	}
}
