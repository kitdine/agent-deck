package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

const receiptThread = "00000000-0000-4000-abcd-000000000002"
const receiptParent = "00000000-0000-4000-abcd-000000000001"
const receiptNeighbor = "00000000-0000-4000-abcd-000000000003"
const receiptJSONLNeighbor = "00000000-0000-4000-abcd-000000000004"
const receiptHeader = `{"type":"session_meta","payload":{"id":"` + receiptThread + `"}}` + "\n"
const receiptTurn = `{"type":"turn_context","payload":{"turn_id":"t","model":"m"}}` + "\n"

func receiptMeter(id string, n int) string {
	return fmt.Sprintf(`{"timestamp":"2026-01-02T10:00:00Z","type":"token_usage_record","payload":{"thread_id":"%s","turn_id":"t","response_id":"%s","usage":{"input_tokens":%d,"output_tokens":0}}}`+"\n", receiptThread, id, n)
}

// Use actual sealed C1 worksets and the pinned producer, including its hash
// of the diff. A caller-created Eligibility is never an input to the entry.
func receiptProposal(t *testing.T, ticket int64, body string, hints ledger.Gap) ledger.Proposal {
	t.Helper()
	ctx := t.Context()
	db, _, f, b := newCJSIWorksetFixture(t, body)
	b.Version.MaxLineBytes = 4096
	w, err := beginCJSICheckpointWorkset(ctx, db, b)
	if err != nil {
		t.Fatal(err)
	}
	replay, step := cjsiFinishIntegrationReplay(t, f, w, nil)
	if err = w.Seal(ctx, step); err != nil {
		t.Fatal(err)
	}
	sel := ingest.CodexSelectedSource{Path: f.Name(), ThreadID: receiptThread, Epoch: 9, Evidence: ingest.CodexPrefixEvidence{
		Identity: b.Identity, Version: b.Version, MaxLineBytes: b.Version.MaxLineBytes, Progress: step.Progress, Digest: sha256.Sum256([]byte(body))}}
	source, err := ledger.StageSource(ctx, sel, func(context.Context) (ingest.CodexSelectedSource, error) { return sel, nil }, replay, cjsiIntegrationLimits, 16)
	if err != nil {
		t.Fatal(err)
	}
	input := ledger.ReplayInput{Thread: receiptThread, Basis: ledger.Basis{Ticket: ticket, BaseTicket: ticket - 1, CoreEpoch: 1, Attempt: 1, InputDigest: sha256.Sum256([]byte(fmt.Sprintf("selected/%d", ticket)))}, History: ledger.History{Assessed: true}, Hints: hints,
		Expected: []ledger.SourceStamp{ledger.SelectionStamp(sel)}, Sources: []ledger.Source{source}}
	p, err := ledger.Replay(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// openCJSI37TestStore runs Task 2's real managed migration to schema 37 (cold
// or upgraded from schema 35) and opens the core database with foreign_keys=ON.
func openCJSI37TestStore(t *testing.T, fromSchema35 bool) (string, *Store) {
	t.Helper()
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	if fromSchema35 {
		c.seed35(t)
	}
	if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(c.root, "agentdeck.sqlite3"), "rw")+"&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return c.root, &Store{DB: db}
}

// OpenCJSI37ForTelemetryTest exposes the real migrated core to the external
// store test that exercises telemetry's combined public entry point.
func OpenCJSI37ForTelemetryTest(t *testing.T) *Store {
	t.Helper()
	_, st := openCJSI37TestStore(t, false)
	return st
}

// stageCJSIProposalRows records the proposed publication attempt, usage group,
// and positive request members in Task 2's real schema-37 tables inside tx so
// cjsi_takeovers foreign keys and SQLite receipt reload match real publication.
func stageCJSIProposalRows(ctx context.Context, tx *sql.Tx, thread string, p ledger.Proposal, selectGroup bool) error {
	st := p.Stamp()
	v, err := p.Eligibility(st, thread)
	if err != nil {
		return err
	}
	state := "draft"
	if selectGroup {
		state = "opened"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_publication_attempts(
 ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,
 user_revision,route_revision,price_revision,protection_revision,coverage_revision,
 input_digest,diff_digest,accept_until,state)
 VALUES(?,?,?,?,0,0,0,0,0,0,0,0,?,?,'2099-01-01T00:00:00Z',?)
 ON CONFLICT(ticket,attempt) DO UPDATE SET state=excluded.state, diff_digest=excluded.diff_digest`,
		st.Basis.Ticket, st.Basis.Attempt, st.Basis.BaseTicket, st.Basis.CoreEpoch, st.Basis.InputDigest[:], st.Diff[:], state); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_usage_groups(
 ticket,attempt,thread,input_digest,diff_digest,state,gaps,pending,partial,unresolved_overlap)
 VALUES(?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(ticket,attempt,thread) DO NOTHING`,
		st.Basis.Ticket, st.Basis.Attempt, thread, st.Basis.InputDigest[:], st.Diff[:],
		int(v.State), int(v.Gaps), boolInt(v.Pending), boolInt(v.Partial), boolInt(v.UnresolvedOverlap)); err != nil {
		return err
	}
	for _, m := range v.Members {
		if m.Key.Kind == ledger.TokenInterval {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_usage_members(
 ticket,attempt,thread,kind,member_id,turn_id,model,event_at,input_tokens,output_tokens)
 VALUES(?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(ticket,attempt,thread,kind,member_id) DO NOTHING`,
			st.Basis.Ticket, st.Basis.Attempt, thread, int(m.Key.Kind), m.Key.ID, m.TurnID, m.Model, m.At.UTC().Format(time.RFC3339Nano), m.Tokens.Input, m.Tokens.Output); err != nil {
			return err
		}
	}
	if selectGroup {
		if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_usage_selection(thread,ticket,attempt) VALUES(?,?,?)
 ON CONFLICT(thread) DO UPDATE SET ticket=excluded.ticket, attempt=excluded.attempt`,
			thread, st.Basis.Ticket, st.Basis.Attempt); err != nil {
			return err
		}
	}
	return nil
}

// receiptSQLiteFixture wraps the real schema-37 CJSIOTelSQLiteStore so tests
// exercise real cjsi_takeovers and cjsi_receipt_* persistence, foreign keys,
// unique active index, and monotonic period trigger.
type receiptSQLiteFixture struct {
	thread       string
	stamp        ledger.ProposalStamp
	revision     int64
	attributions map[string]CJSIOTelAttribution
	release      *CJSIOTelRelease
	beforeMutate func(*cjsiOTelSide)
}

func (f receiptSQLiteFixture) store() CJSIOTelSQLiteStore {
	return CJSIOTelSQLiteStore{
		Guard: func(_ context.Context, _ *sql.Tx, _ string) (CJSIOTelGuard, error) {
			return CJSIOTelGuard{Thread: f.thread, Stamp: f.stamp, Revision: f.revision}, nil
		},
		Attributions: f.attributions,
		Release:      f.release,
	}
}

func (f receiptSQLiteFixture) Select(ctx context.Context, tx *sql.Tx, thread string) (CJSIOTelSelection, error) {
	s, err := f.store().Select(ctx, tx, thread)
	if err != nil {
		return s, err
	}
	if f.beforeMutate != nil {
		f.beforeMutate(&s.Before)
	}
	return s, nil
}
func (f receiptSQLiteFixture) Current(ctx context.Context, tx *sql.Tx, thread string) (CJSIOTelGuard, error) {
	return f.store().Current(ctx, tx, thread)
}
func (f receiptSQLiteFixture) Save(ctx context.Context, tx *sql.Tx, old *CJSIOTelPeriod, next CJSIOTelPeriod) error {
	return f.store().Save(ctx, tx, old, next)
}

func receiptSide(t *testing.T, db *sql.DB, thread string) cjsiOTelSide {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	side, err := readCJSIOTelSide(t.Context(), tx, thread)
	if err != nil {
		t.Fatal(err)
	}
	return side
}

func receiptPeriodDB(t *testing.T, db *sql.DB, thread string) *CJSIOTelPeriod {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	p, err := readLatestCJSITakeoverPeriod(t.Context(), tx, thread)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func receiptApply(t *testing.T, db *sql.DB, f receiptSQLiteFixture, p ledger.Proposal, between ...func(*sql.Tx) error) error {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	change, err := PrepareCJSIOTel(t.Context(), tx, receiptThread, p, f)
	_ = tx.Rollback()
	if err != nil || change == nil {
		return err
	}
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = stageCJSIProposalRows(t.Context(), tx, receiptThread, p, true); err != nil {
		return err
	}
	var withdraw func(*sql.Tx) error
	if len(between) > 0 {
		withdraw = between[0]
	}
	if err = change.Apply(t.Context(), tx, f, withdraw); err != nil {
		return err
	}
	return tx.Commit()
}

// Prepare in one snapshot and let an independent writer commit while the
// fixture revision stays unchanged, then apply with the proved withdrawal.
func receiptRace(t *testing.T, db *sql.DB, f receiptSQLiteFixture, p ledger.Proposal, writer func(), withdraw func(*sql.Tx) error) error {
	t.Helper()
	read, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	change, err := PrepareCJSIOTel(t.Context(), read, receiptThread, p, f)
	_ = read.Rollback()
	if err != nil || change == nil {
		t.Fatal("prepare", err)
	}
	writer()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = stageCJSIProposalRows(t.Context(), tx, receiptThread, p, true); err != nil {
		return err
	}
	if err = change.Apply(t.Context(), tx, f, withdraw); err != nil {
		return err
	}
	return tx.Commit()
}

// Commit one JSONL fact the way the genuine usage publisher does: the fact
// and its source, then the summary statement of internal/usage's
// rebuildSessions verbatim (that package imports this one).
func receiptPublish(t *testing.T, db *sql.DB, session, key, at string, input, offset int64) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	path := "synthetic/" + session + ".jsonl"
	for _, q := range [][]any{
		{"INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id,session_started_at) VALUES(?,?,0,0,'',?,'2026-01-02T09:00:00Z') ON CONFLICT(path) DO NOTHING", path, path, session},
		{"INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES(?,'codex',?,?,?,'m',?,?,?)", key, session, strings.TrimPrefix(key, "jsonl:"), at, input, path, offset},
	} {
		if _, err = tx.Exec(q[0].(string), q[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	result, err := tx.Exec(`INSERT INTO usage_sessions(client,session_id,first_at,last_at,started_at) SELECT client,session_id,MIN(event_at),MAX(event_at),COALESCE((SELECT MIN(f.session_started_at) FROM usage_source_files f WHERE f.session_id=? AND f.session_started_at<>''),'') FROM usage_events WHERE client=? AND session_id=? GROUP BY client,session_id
ON CONFLICT(client,session_id) DO UPDATE SET first_at=excluded.first_at,last_at=excluded.last_at,started_at=excluded.started_at
WHERE (usage_sessions.first_at,usage_sessions.last_at,usage_sessions.started_at) IS NOT (excluded.first_at,excluded.last_at,excluded.started_at)`, session, "codex", session)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := result.RowsAffected(); err != nil {
		t.Fatal(err)
	} else if changed == 0 {
		if _, err = tx.Exec(`DELETE FROM usage_sessions WHERE client=? AND session_id=? AND NOT EXISTS(SELECT 1 FROM usage_events WHERE client=? AND session_id=?)`, "codex", session, "codex", session); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func receiptObservation(key, thread, at string, n int64) TelemetryObservation {
	out, cached := int64(1), int64(0)
	return TelemetryObservation{Key: key, ThreadID: thread, Source: "import", Producer: "codex_exec", ProducerVersion: "1", Originator: "codex_exec",
		EventName: "codex.sse_event", EventKind: "response.completed", Transport: "sse", RecordAt: at, ObservedAt: at, ReceivedAt: "2026-01-03T00:00:00Z", Model: "m",
		ResponseID: "resp-" + key, RequestID: "request-" + key, AttemptID: "attempt-" + key, SourceClass: "fixture", InputTokens: &n, OutputTokens: &out, CachedTokens: &cached,
		Metered: true, SupportEntry: "codex_exec/1/codex_exec/sse", State: "pending", Reason: "ownership_not_settled"}
}

func makeTestAttribution(provider, quality, multiplier, catalog string) CJSIOTelAttribution {
	a := CJSIOTelAttribution{
		Provider:       provider,
		Quality:        quality,
		Multiplier:     multiplier,
		CatalogVersion: catalog,
		PriceReference: catalog,
	}
	a.Evidence = CJSIOTelAttributionEvidence(a)
	return a
}

func TestCJSIOTelOwnership(t *testing.T) {
	body := receiptHeader + receiptTurn
	take := receiptProposal(t, 5, body+receiptMeter("verified", 17)+receiptMeter("dispute", 2)+receiptMeter("dispute", 3), 0)
	keep := receiptProposal(t, 6, body+receiptMeter("remaining", 5), 0)
	head := receiptProposal(t, 7, receiptHeader, 0)
	next := receiptProposal(t, 8, body+receiptMeter("new-period", 3), 0)
	overlap := receiptProposal(t, 7, body+receiptMeter("unresolved", 8), ledger.UnverifiedSource)
	view, err := take.Eligibility(take.Stamp(), receiptThread)
	if err != nil || !view.Partial || view.UnresolvedOverlap || len(view.Members) != 1 {
		t.Fatalf("partial request fixture: %+v %v", view, err)
	}

	t.Run("only authenticated positive request members", func(t *testing.T) {
		v := view
		m := view.Members[0]
		interval := m
		interval.Key.Kind = ledger.TokenInterval
		v.Members = []ledger.Member{interval}
		keys, err := cjsiOTelRequests(v)
		if err != nil || len(keys) != 0 {
			t.Fatal("interval counted as a request", keys, err)
		}
		v.Members = append(v.Members, m)
		keys, err = cjsiOTelRequests(v)
		if err != nil || !slices.Equal(keys, []ledger.Key{m.Key}) {
			t.Fatal(keys, err)
		}
		for _, mutate := range []func(*ledger.Member){
			func(m *ledger.Member) { m.Proof = nil }, func(m *ledger.Member) { m.Key.Thread = receiptNeighbor },
			func(m *ledger.Member) { m.Proof[0].Source.Epoch++ }, func(m *ledger.Member) { m.Proof[0].Span.End++ },
			func(m *ledger.Member) { m.Tokens.Input, m.Tokens.Output = 0, 0 },
		} {
			bad := m
			bad.Proof = slices.Clone(m.Proof)
			// Bind the range check to complete_end even when another row follows.
			bad.Proof[0].Span.End = bad.Proof[0].Source.Prefix.Progress.CompleteEnd
			mutate(&bad)
			v.Members = []ledger.Member{bad}
			if _, err := cjsiOTelRequests(v); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatal("accepted invalid request", bad)
			}
		}
		v.Members = []ledger.Member{m}
		v.Pending = true
		if keys, _ = cjsiOTelRequests(v); len(keys) != 0 {
			t.Fatal("pending granted ownership")
		}
	})

	t.Run("only surviving settled observations feed the marker", func(t *testing.T) {
		_, st := openCJSI37TestStore(t, false)
		db := st.DB
		old := receiptObservation("historical", receiptThread, "2025-11-30T00:00:00Z", 50)
		old.State, old.Reason = "jsonl_owned", "jsonl_evidence"
		if _, err := st.ApplyTelemetryObservations(t.Context(), []TelemetryObservation{old}, nil, nil); err != nil {
			t.Fatal(err)
		}
		beforeHist := receiptSide(t, db, receiptThread)
		f := receiptSQLiteFixture{thread: receiptThread, stamp: head.Stamp(), revision: 1,
			release: &CJSIOTelRelease{Thread: receiptThread, Number: 1, Current: head.Stamp()}}
		if err := receiptApply(t, db, f, head); err != nil || receiptPeriodDB(t, db, receiptThread) != nil || !reflect.DeepEqual(beforeHist, receiptSide(t, db, receiptThread)) {
			t.Fatal("historical suppression manufactured a receipt", err)
		}
		ownershipExec(t, db, "DELETE FROM telemetry_observations WHERE observation_key='historical'")
		f.attributions = map[string]CJSIOTelAttribution{}
		for i, id := range []string{"first", "middle", "last"} {
			o := receiptObservation(id, receiptThread, fmt.Sprintf("2025-12-0%dT10:00:00Z", i+1), int64(i+2))
			o.Model = id
			o.ProducerVersion = id
			o.SupportEntry = "codex_exec/" + id + "/codex_exec/sse"
			if _, err := st.ApplyTelemetryObservations(t.Context(), []TelemetryObservation{o}, nil, nil); err != nil {
				t.Fatal(err)
			}
			price := makeTestAttribution("p", "exact", "1", id)
			price.PriceReference = "" // CatalogVersion alone is a complete price reference.
			f.attributions["otel:"+id] = price
		}
		ownershipExec(t, db, "INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('synthetic',1,'2026-01-04T00:00:00Z',0,'complete')")
		inventory := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		if _, err := st.SettleTelemetry(t.Context(), inventory, time.Minute); err != nil {
			t.Fatal(err)
		}
		// Older stores kept each settled event's original UUID spelling. The
		// upper-case event supplied the binary minimum for both public rows.
		upper := strings.ToUpper(receiptThread)
		ownershipExec(t, db, "UPDATE telemetry_observations SET thread_id=? WHERE observation_key='first'", upper)
		ownershipExec(t, db, "UPDATE usage_events SET session_id=? WHERE event_key='otel:first'", upper)
		ownershipExec(t, db, "UPDATE telemetry_sessions SET session_id=? WHERE client='codex' AND session_id=?", upper, receiptThread)
		ownershipExec(t, db, "UPDATE usage_sessions SET session_id=? WHERE client='codex' AND session_id=?", upper, receiptThread)
		initial := receiptSide(t, db, receiptThread)
		if len(initial.Events) != 3 || len(initial.Markers) != 1 {
			t.Fatal("incomplete initial side")
		}
		settledAt := initial.Markers[0].Settled
		f.stamp = take.Stamp()
		if err := receiptApply(t, db, f, take); err != nil {
			t.Fatal(err)
		}
		ownershipExec(t, db, "UPDATE telemetry_observations SET state='invalid',reason='revalidated' WHERE observation_key IN ('first','last')")
		f.stamp = head.Stamp()
		f.release = &CJSIOTelRelease{Thread: receiptThread, Number: 1, Support: take.Stamp(), Current: head.Stamp(), Removed: receiptPeriodDB(t, db, receiptThread).Requests, InventoryAt: inventory}
		original := f.attributions["otel:middle"]
		changed := original
		changed.Provider = "different-provider"
		changed.Evidence = CJSIOTelAttributionEvidence(changed)
		f.attributions["otel:middle"] = changed
		if err := receiptApply(t, db, f, head); !errors.Is(err, ErrCJSIOTelPrecondition) {
			t.Fatalf("changed provider allowed release: %v", err)
		}
		f.attributions["otel:middle"] = original
		if err := receiptApply(t, db, f, head); err != nil {
			t.Fatal(err)
		}
		got := receiptSide(t, db, receiptThread)
		if len(got.Markers) != 1 || len(got.Sessions) != 1 || len(got.Events) != 1 || len(got.Contributions) != 1 {
			t.Fatal("wrong restored row set", got)
		}
		m, e, session := got.Markers[0], got.Events[0], got.Sessions[0]
		markers, err := st.TelemetrySessionOriginMarkers(t.Context(), "codex", []string{receiptThread, upper})
		if err != nil || len(markers) != 1 || markers[receiptThread].SettledAt != settledAt ||
			e.Thread != receiptThread || m.Thread != e.Thread || session.Thread != e.Thread {
			t.Fatalf("restored public spelling: event=%q marker=%q session=%q lookup=%+v err=%v", e.Thread, m.Thread, session.Thread, markers, err)
		}
		if e.ID != "middle" || e.At != "2025-12-02T10:00:00Z" || e.Input+e.Output != 4 || m.Model != "middle" || m.Settled != settledAt ||
			m.Support != "codex_exec/middle/codex_exec/sse" || m.First != e.At || m.Last != e.At || session.First != e.At || session.Last != e.At {
			t.Fatalf("restored marker or session spans invalidated rows: %+v %+v %+v", m, e, session)
		}
	})

	for _, disposition := range []string{"restore", "invalidate"} {
		t.Run(disposition, func(t *testing.T) {
			_, st := openCJSI37TestStore(t, disposition == "invalidate")
			db := st.DB
			// Schema 37 drops the legacy trigger and forbids bare retraction APIs.
			if n := ownershipCount(t, db, "SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'"); n != 0 {
				t.Fatalf("legacy trigger survived schema 37: %d", n)
			}
			// Seed P=7, C=11, S=13 (A5 synthetic series).
			seed := []TelemetryObservation{
				receiptObservation("parent", receiptParent, "2025-12-01T09:00:00Z", 6),
				receiptObservation("original", receiptThread, "2025-12-01T10:00:00Z", 10),
				receiptObservation("neighbor", receiptNeighbor, "2025-12-02T10:00:00Z", 12),
			}
			if _, err := st.ApplyTelemetryObservations(t.Context(), seed, nil, nil); err != nil {
				t.Fatal(err)
			}
			ownershipExec(t, db, "INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('synthetic',1,'2026-01-04T00:00:00Z',0,'complete')")
			if _, err := st.SettleTelemetry(t.Context(), time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), time.Minute); err != nil {
				t.Fatal(err)
			}
			// Verify SaveSourceThreadIdentities does NOT retract OTel on schema 37,
			// and RetractTelemetryForJSONLThreads fails closed.
			rolloutPath := fmt.Sprintf("/synthetic/rollout-2026-01-02T09-00-00-%s.jsonl", receiptThread)
			ownershipExec(t, db, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES(?,'id-1',0,0,'')`, rolloutPath)
			retracted, err := st.SaveSourceThreadIdentities(t.Context(), []SourceThreadIdentity{{Path: rolloutPath, Identity: "id-1", ThreadID: receiptThread}}, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
			if err != nil || retracted != 0 {
				t.Fatalf("SaveSourceThreadIdentities retracted on schema 37: n=%d err=%v", retracted, err)
			}
			txCheck, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := RetractTelemetryForJSONLThreads(t.Context(), txCheck, []string{receiptThread}, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrCJSIOTelAuthorityRequired) {
				txCheck.Rollback()
				t.Fatalf("RetractTelemetryForJSONLThreads bypassed schema 37 authority: %v", err)
			}
			txCheck.Rollback()
			ownershipExec(t, db, `DELETE FROM usage_source_files WHERE path=?`, rolloutPath)

			ownershipExec(t, db, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(41,'codex','original-provider','1.25','2025-12-01T00:00:00Z');
INSERT INTO usage_run_bindings(event_key,run_id) VALUES('otel:original',41);
UPDATE usage_events SET run_id=41,source_offset=123,turn_index=0 WHERE event_key='otel:original'`)
			ownershipExec(t, db, "UPDATE usage_sessions SET started_at='2025-11-30T00:00:00Z' WHERE session_id=?", receiptThread)
			pending := receiptObservation("pending", receiptThread, "2025-12-03T10:00:00Z", 49)
			if _, err := st.ApplyTelemetryObservations(t.Context(), []TelemetryObservation{pending}, nil, nil); err != nil {
				t.Fatal(err)
			}
			before, parentBefore, neighbor := receiptSide(t, db, receiptThread), receiptSide(t, db, receiptParent), receiptSide(t, db, receiptNeighbor)
			if len(before.Events) != 1 || before.Events[0].Input+before.Events[0].Output != 11 || len(before.Bindings) != 1 || len(before.Markers) != 1 {
				t.Fatal("invalid settled fixture")
			}
			if len(parentBefore.Events) != 1 || parentBefore.Events[0].Input+parentBefore.Events[0].Output != 7 || len(neighbor.Events) != 1 || neighbor.Events[0].Input+neighbor.Events[0].Output != 13 {
				t.Fatal("invalid P/S fixture")
			}
			// Raw INSERT on schema 37 must NOT silently retract OTel (legacy trigger dropped).
			ownershipExec(t, db, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('raw-probe','codex',?,'raw-probe','2026-01-02T10:00:00Z','m',9,'synthetic/raw.jsonl',1)`, receiptThread)
			if got := receiptSide(t, db, receiptThread); len(got.Events) != 1 || len(got.Markers) != 1 || len(got.OtherEvents) != 1 {
				t.Fatalf("raw INSERT silently retracted OTel on schema 37: %+v", got)
			}
			ownershipExec(t, db, `DELETE FROM usage_events WHERE event_key='raw-probe'`)

			price := makeTestAttribution("original-provider", "exact", "1.25", "catalog-at-original-event")
			f := receiptSQLiteFixture{revision: 11, thread: receiptThread, attributions: map[string]CJSIOTelAttribution{"otel:original": price}}
			run := func(p ledger.Proposal) {
				t.Helper()
				f.stamp = p.Stamp()
				if err := receiptApply(t, db, f, p); err != nil {
					t.Fatal(err)
				}
			}
			for _, change := range []func(*ledger.ProposalStamp){
				func(s *ledger.ProposalStamp) { s.Basis.Ticket++ }, func(s *ledger.ProposalStamp) { s.Basis.BaseTicket++ }, func(s *ledger.ProposalStamp) { s.Basis.Attempt++ },
				func(s *ledger.ProposalStamp) { s.Basis.CoreEpoch++ }, func(s *ledger.ProposalStamp) { s.Basis.InputDigest[0]++ }, func(s *ledger.ProposalStamp) { s.Diff[0]++ },
			} {
				f.stamp = take.Stamp()
				change(&f.stamp)
				if err := receiptApply(t, db, f, take); !errors.Is(err, ledger.ErrProposalMismatch) {
					t.Fatal("accepted stale stamp", err)
				}
			}
			f.thread = strings.ToUpper(receiptThread)
			f.stamp = take.Stamp()
			if err := receiptApply(t, db, f, take); !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatal("accepted other selection", err)
			}
			f.thread = receiptThread
			run(head)
			run(overlap)
			if receiptPeriodDB(t, db, receiptThread) != nil || !reflect.DeepEqual(receiptSide(t, db, receiptThread), before) {
				t.Fatal("header/overlap mutated old side")
			}
			f.stamp = take.Stamp()
			f.beforeMutate = func(s *cjsiOTelSide) {
				s.Markers = slices.Clone(s.Markers)
				s.Markers[0].Settled = "not the old marker"
			}
			if err := receiptApply(t, db, f, take); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("accepted stale old marker", err)
			}
			f.beforeMutate = nil
			// A selected plan loses the race before the write transaction,
			// even when all of its proposed rows still look the same.
			read, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			plan, err := PrepareCJSIOTel(t.Context(), read, receiptThread, take, f)
			_ = read.Rollback()
			if err != nil || plan == nil {
				t.Fatal("prepare", err)
			}
			for _, mutate := range []func(*receiptSQLiteFixture){
				func(s *receiptSQLiteFixture) { s.revision++ }, func(s *receiptSQLiteFixture) { s.stamp.Basis.BaseTicket++ }, func(s *receiptSQLiteFixture) { s.thread = receiptNeighbor },
			} {
				other := f
				mutate(&other)
				write, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				err = plan.Apply(t.Context(), write, other, nil)
				_ = write.Rollback()
				if !errors.Is(err, ErrCJSIOTelPrecondition) {
					t.Fatal("stale apply", err)
				}
			}
			ownershipExec(t, db, "CREATE TRIGGER fixture_stop BEFORE DELETE ON telemetry_sessions BEGIN SELECT RAISE(ABORT,'fixture_stop'); END")
			if err := receiptApply(t, db, f, take); err == nil || !strings.Contains(err.Error(), "fixture_stop") {
				t.Fatal("rollback was not exercised", err)
			}
			if receiptPeriodDB(t, db, receiptThread) != nil || !reflect.DeepEqual(receiptSide(t, db, receiptThread), before) {
				t.Fatal("receipt or withdrawal escaped rollback")
			}
			ownershipExec(t, db, "DROP TRIGGER fixture_stop")
			run(take)
			active := receiptPeriodDB(t, db, receiptThread)
			if active == nil || active.Number != 1 || active.Opened != take.Stamp() || active.Receipts["original"].Settled.Attribution != price || active.Receipts["pending"].Settled != nil {
				t.Fatalf("incorrect original receipts: %+v", active)
			}
			// Verify real SQLite receipt tables were populated atomically.
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_takeovers WHERE thread=? AND period=1 AND closed_ticket IS NULL", receiptThread); n != 1 {
				t.Fatalf("cjsi_takeovers count=%d", n)
			}
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_receipt_observations WHERE thread=? AND period=1 AND disposition='applied'", receiptThread); n != 2 {
				t.Fatalf("cjsi_receipt_observations count=%d", n)
			}
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_receipt_events WHERE thread=? AND period=1 AND event_key='otel:original' AND binding_run_id=41", receiptThread); n != 1 {
				t.Fatalf("cjsi_receipt_events count=%d", n)
			}
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_receipt_markers WHERE thread=? AND period=1", receiptThread); n != 1 {
				t.Fatalf("cjsi_receipt_markers count=%d", n)
			}
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_receipt_sessions WHERE thread=? AND period=1 AND started_at='2025-11-30T00:00:00Z'", receiptThread); n != 1 {
				t.Fatalf("cjsi_receipt_sessions count=%d", n)
			}

			afterTake := receiptSide(t, db, receiptThread)
			if len(afterTake.Observations) != 2 || len(afterTake.Events)+len(afterTake.Markers)+len(afterTake.Bindings)+len(afterTake.Sessions)+len(afterTake.Contributions) != 0 ||
				afterTake.Observations[0].State != "jsonl_owned" || afterTake.Observations[1].State != "jsonl_owned" {
				t.Fatal("incomplete withdrawal", afterTake)
			}
			// Idempotent retry is a no-op.
			run(take)
			// Surviving R2=5 keeps JSONL ownership under period 1.
			run(keep)
			active = receiptPeriodDB(t, db, receiptThread)
			if active.Number != 1 || active.Support != keep.Stamp() || len(active.Requests) != 1 || active.Requests[0].ID != "remaining" {
				t.Fatal("lost individual surviving support")
			}
			// Late pending observation arriving during active takeover is suppressed without settlement privilege.
			late := receiptObservation("late", receiptThread, "2026-01-05T10:00:00Z", 99)
			if _, err := st.ApplyTelemetryObservations(t.Context(), []TelemetryObservation{late}, nil, nil); err != nil {
				t.Fatal(err)
			}
			run(keep)
			active = receiptPeriodDB(t, db, receiptThread)
			if active.Receipts["late"].Original.State != "pending" || active.Receipts["late"].Settled != nil {
				t.Fatal("late observation acquired settlement")
			}
			run(head)
			if !reflect.DeepEqual(active, receiptPeriodDB(t, db, receiptThread)) {
				t.Fatal("pending inferred a release")
			}
			f.release = &CJSIOTelRelease{Thread: receiptThread, Number: 1, Support: keep.Stamp(), Current: head.Stamp(), Removed: active.Requests, InventoryAt: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)}
			f.release.Number++
			if err := receiptApply(t, db, f, head); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("cross-period release", err)
			}
			f.release.Number--
			f.release.InventoryAt = time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
			if err := receiptApply(t, db, f, head); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("non-later inventory granted release", err)
			}
			f.release.InventoryAt = time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
			delete(f.attributions, "otel:original")
			if err := receiptApply(t, db, f, head); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("missing original pricing inputs granted release", err)
			}
			f.attributions["otel:original"] = price
			if disposition == "invalidate" {
				ownershipExec(t, db, "UPDATE telemetry_observations SET state='conflict',reason='token_counts_disagree' WHERE observation_key='original'")
			}
			// Neither recheck may count or retire another thread's JSONL history.
			receiptPublish(t, db, receiptJSONLNeighbor, "jsonl:elsewhere", "2026-01-02T08:00:00Z", 6, 30)
			jsonlNeighbor := receiptSide(t, db, receiptJSONLNeighbor)
			if len(jsonlNeighbor.Sessions) != 1 || len(jsonlNeighbor.OtherEvents) != 1 {
				t.Fatal("neighboring JSONL fixture", jsonlNeighbor)
			}
			// R1-F1 fixture: genuine publisher wrote the session summary alongside the JSONL event.
			receiptPublish(t, db, receiptThread, "jsonl:remaining", "2026-01-02T10:00:00Z", 5, 10)
			beforeRelease := receiptSide(t, db, receiptThread)
			summary := cjsiOTelSession{"codex", receiptThread, "2026-01-02T10:00:00Z", "2026-01-02T10:00:00Z", "2026-01-02T09:00:00Z"}
			if !slices.Equal(beforeRelease.Sessions, []cjsiOTelSession{summary}) || !slices.Equal(beforeRelease.OtherEvents, []string{"jsonl:remaining"}) {
				t.Fatal("publisher fixture", beforeRelease)
			}
			held := receiptPeriodDB(t, db, receiptThread)
			unchanged := func(want cjsiOTelSide, why string) {
				t.Helper()
				if got := receiptSide(t, db, receiptThread); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(receiptPeriodDB(t, db, receiptThread), held) {
					t.Fatalf("%s escaped rollback:\n%+v\n%+v", why, got, want)
				}
			}
			f.stamp = head.Stamp()
			if err := receiptApply(t, db, f, head); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("restored alongside a remaining request", err)
			}
			unchanged(beforeRelease, "failed release")
			withdraw := func(tx *sql.Tx) error {
				return cjsiOTelChanged(tx.Exec("DELETE FROM usage_events WHERE event_key='jsonl:remaining' AND client='codex' AND session_id=? AND event_id='remaining' AND input_tokens=5 AND source_offset=10", receiptThread))
			}
			// R1-F1: independent summary edit after preparation is never overwritten.
			edited := beforeRelease
			edited.Sessions = []cjsiOTelSession{summary}
			edited.Sessions[0].Last = "2026-01-02T11:00:00Z"
			err = receiptRace(t, db, f, head, func() {
				ownershipExec(t, db, "UPDATE usage_sessions SET last_at=? WHERE client='codex' AND session_id=?", edited.Sessions[0].Last, receiptThread)
			}, withdraw)
			if !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("released over a newer summary edit", err)
			}
			unchanged(edited, "refused summary replacement")
			ownershipExec(t, db, "UPDATE usage_sessions SET last_at=first_at WHERE client='codex' AND session_id=?", receiptThread)
			unchanged(beforeRelease, "summary probe cleanup")
			// R1-F2: JSONL fact of the canonical thread (both lowercase & UPPERCASE UUID)
			// appears after preparation with frozen revision.
			for _, session := range []string{receiptThread, strings.ToUpper(receiptThread)} {
				var intervening cjsiOTelSide
				err = receiptRace(t, db, f, head, func() {
					if session == receiptThread {
						receiptPublish(t, db, session, "jsonl:intervening", "2026-01-02T10:00:00Z", 7, 20)
					} else {
						ownershipExec(t, db, "INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('jsonl:intervening','codex',?,'intervening','2026-01-02T10:00:00Z','m',7,'synthetic/upper.jsonl',20)", session)
					}
					intervening = receiptSide(t, db, receiptThread)
				}, withdraw)
				if !errors.Is(err, ErrCJSIOTelPrecondition) {
					t.Fatal("restored beside an unwithdrawn JSONL row", session, err)
				}
				if !slices.Equal(intervening.Sessions, beforeRelease.Sessions) || len(intervening.OtherEvents) != 2 {
					t.Fatal("intervening fixture", intervening)
				}
				unchanged(intervening, "refused intervening row")
				ownershipExec(t, db, "DELETE FROM usage_events WHERE event_key='jsonl:intervening'")
			}
			unchanged(beforeRelease, "intervening probe cleanup")
			// Round 2 composition probe: revision-bypassed late OTel event also fails closed in vacate.
			err = receiptRace(t, db, f, head, func() {
				ownershipExec(t, db, "INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('otel:rogue','codex',?,'rogue','2026-01-02T10:05:00Z','m',3,1,'otel:rogue',0)", receiptThread)
			}, withdraw)
			if !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatalf("release bypassed unexpected late OTel event: %v", err)
			}
			ownershipExec(t, db, "DELETE FROM usage_events WHERE event_key='otel:rogue'")
			unchanged(beforeRelease, "rogue otel probe cleanup")

			// Genuine release succeeds atomically!
			if err = receiptApply(t, db, f, head, withdraw); err != nil {
				t.Fatal(err)
			}
			// Idempotent retry on closed period is no-op.
			run(head)
			actual := receiptSide(t, db, receiptThread)
			closed := receiptPeriodDB(t, db, receiptThread)
			if closed == nil || closed.Number != 1 || closed.Closed != head.Stamp() {
				t.Fatalf("period not closed exactly once: %+v", closed)
			}
			if actual.Observations[0].Key != "late" || !reflect.DeepEqual(actual.Observations[0], late) || !reflect.DeepEqual(actual.Observations[2], pending) {
				t.Fatal("pending was settled on release", actual.Observations)
			}
			if len(actual.OtherEvents) != 0 {
				t.Fatal("withdrawn JSONL fact survived", actual.OtherEvents)
			}
			if disposition == "restore" {
				actual.Observations = actual.Observations[1:]
				if !reflect.DeepEqual(actual, before) {
					t.Fatalf("restored original images differ:\n%+v\n%+v", actual, before)
				}
				if e := actual.Events[0]; e.Input+e.Output != 11 || actual.Bindings[0].Run != 41 || actual.Sessions[0].Started != "2025-11-30T00:00:00Z" {
					t.Fatal("restored the wrong fact, binding or summary", actual)
				}
			} else if actual.Observations[1].State != "conflict" || len(actual.Events)+len(actual.Markers)+len(actual.Contributions)+len(actual.Bindings)+len(actual.Sessions) != 0 {
				t.Fatal("invalid observation regained a fact or marker, or kept the JSONL-era summary")
			}
			wantDisposition := cjsiOTelRestored
			if disposition == "invalidate" {
				wantDisposition = cjsiOTelInvalidated
			}
			if closed.Receipts["original"].Disposition != wantDisposition || closed.Receipts["late"].Disposition != cjsiOTelReturnedPending {
				t.Fatalf("missing per-observation disposition: %+v", closed.Receipts)
			}
			f.stamp = take.Stamp()
			if err := receiptApply(t, db, f, take); !errors.Is(err, ErrCJSIOTelPrecondition) {
				t.Fatal("stale selection reopened a closed period", err)
			}
			f.release = nil
			run(next)
			if p := receiptPeriodDB(t, db, receiptThread); p == nil || p.Number != 2 || p.Opened != next.Stamp() {
				t.Fatal("subsequent takeover reused period", p)
			}
			if n := ownershipCount(t, db, "SELECT count(*) FROM cjsi_takeovers WHERE thread=?", receiptThread); n != 2 {
				t.Fatalf("expected 2 monotonic takeover periods, got %d", n)
			}
			if !reflect.DeepEqual(receiptSide(t, db, receiptParent), parentBefore) {
				t.Fatal("touched parent P settled ownership")
			}
			if !reflect.DeepEqual(receiptSide(t, db, receiptNeighbor), neighbor) {
				t.Fatal("touched neighbor S settled ownership")
			}
			if !reflect.DeepEqual(receiptSide(t, db, receiptJSONLNeighbor), jsonlNeighbor) {
				t.Fatal("touched a neighboring JSONL history")
			}
		})
	}

	t.Run("schema37 per-thread settlement isolation and historical clearance", func(t *testing.T) {
		_, st := openCJSI37TestStore(t, true)
		db := st.DB
		ctx := t.Context()

		// 1. Header C / rollout name P conflict + header-only unassessed file:
		// unrelated thread S (aged 13, post-receipt inventory) must still settle!
		conflictPath := fmt.Sprintf("/synthetic/rollout-2026-01-02T09-00-00-%s.jsonl", receiptParent)
		ownershipExec(t, db, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,thread_id) VALUES(?,'c-id',100,100,'h',?)`, conflictPath, receiptThread)
		ownershipExec(t, db, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('/synthetic/header-only.jsonl','h-id',50,0,'')`)
		for _, obs := range []TelemetryObservation{
			receiptObservation("obs-p", receiptParent, "2026-01-02T10:00:00Z", 6),
			receiptObservation("obs-c", receiptThread, "2026-01-02T10:00:00Z", 10),
			receiptObservation("obs-s", receiptNeighbor, "2026-01-02T10:00:00Z", 12),
		} {
			if _, err := st.ApplyTelemetryObservations(ctx, []TelemetryObservation{obs}, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		// Fractional-second inventory check: 2026-01-03T00:00:00Z is NOT strictly after 2026-01-03T00:00:00.5Z!
		fracObs := receiptObservation("obs-frac", "non-uuid-thread", "2026-01-02T10:00:00Z", 4)
		fracObs.ReceivedAt = "2026-01-03T00:00:00.5Z"
		if _, err := st.ApplyTelemetryObservations(ctx, []TelemetryObservation{fracObs}, nil, nil); err != nil {
			t.Fatal(err)
		}
		ownershipExec(t, db, `INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('inv-1',1,'2026-01-03T00:00:00.1Z',0,'complete')`)
		settleAt := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		res, err := st.SettleTelemetry(ctx, settleAt, 10*time.Minute)
		if !errors.Is(err, ErrTelemetrySourceIdentityPending) {
			t.Fatalf("expected ErrTelemetrySourceIdentityPending while P/C pending blocked, got %+v %v", res, err)
		}
		if res.Owned != 1 {
			t.Fatalf("expected S alone to settle (Owned=1), got %+v", res)
		}
		if sSide := receiptSide(t, db, receiptNeighbor); len(sSide.Events) != 1 || sSide.Events[0].Input+sSide.Events[0].Output != 13 || len(sSide.Markers) != 1 {
			t.Fatalf("S did not settle independently: %+v", sSide)
		}
		if pSide := receiptSide(t, db, receiptParent); len(pSide.Events) != 0 || pSide.Observations[0].State != "pending" {
			t.Fatalf("conflicted P promoted: %+v", pSide)
		}
		if cSide := receiptSide(t, db, receiptThread); len(cSide.Events) != 0 || cSide.Observations[0].State != "pending" {
			t.Fatalf("conflicted C promoted: %+v", cSide)
		}
		if fracSide := receiptSide(t, db, "non-uuid-thread"); len(fracSide.Events) != 0 || fracSide.Observations[0].State != "pending" {
			t.Fatalf("fractional-second observation promoted prematurely: %+v", fracSide)
		}

		// 2. Already-settled S stays 13 + marker when a new clue or unbounded orphan appears,
		// while a new pending observation for S stays pending.
		lateS := receiptObservation("obs-s-late", receiptNeighbor, "2026-01-02T11:00:00Z", 20)
		if _, err := st.ApplyTelemetryObservations(ctx, []TelemetryObservation{lateS}, nil, nil); err != nil {
			t.Fatal(err)
		}
		orphanKey := sha256.Sum256([]byte("orphan-scope"))
		windowKey := sha256.Sum256([]byte("all-time"))
		ownershipExec(t, db, `INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision)
 VALUES('usage','orphan',?,?,1,1,0,1,0)`, orphanKey[:], windowKey[:])
		if _, err := st.SettleTelemetry(ctx, settleAt, 10*time.Minute); !errors.Is(err, ErrTelemetrySourceIdentityPending) {
			t.Fatalf("unbounded orphan did not report pending: %v", err)
		}
		sSide := receiptSide(t, db, receiptNeighbor)
		if len(sSide.Events) != 1 || sSide.Events[0].Input+sSide.Events[0].Output != 13 || len(sSide.Markers) != 1 {
			t.Fatalf("existing settled S was mutated under orphan overlay: %+v", sSide)
		}
		ownershipExec(t, db, `DELETE FROM cjsi_scope_overlays`)

		// 3. Historical false jsonl_owned suppression on P without receipt:
		// stays jsonl_owned until verified NoLocalCall clearance (0 sources, 0 clues, 0 overlap).
		ownershipExec(t, db, `DELETE FROM usage_source_files`)
		ownershipExec(t, db, `UPDATE telemetry_observations SET state='jsonl_owned', reason='jsonl_evidence' WHERE observation_key='obs-p'`)
		ownershipExec(t, db, `DELETE FROM telemetry_observations WHERE observation_key IN ('obs-c','obs-s-late','obs-frac')`)
		if _, err := st.SettleTelemetry(ctx, settleAt, 10*time.Minute); !errors.Is(err, ErrTelemetrySourceIdentityPending) {
			t.Fatalf("unverified historical jsonl_owned should stay pending/unresolved: %v", err)
		}
		if pSide := receiptSide(t, db, receiptParent); pSide.Observations[0].State != "jsonl_owned" || len(pSide.Events) != 0 {
			t.Fatalf("historical jsonl_owned recovered without proof: %+v", pSide)
		}
		// Install verified NoLocalCall clearance (0 sources, 0 gaps, 0 overlap) for P.
		ownershipExec(t, db, `INSERT INTO cjsi_publication_attempts(
 ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,
 user_revision,route_revision,price_revision,protection_revision,coverage_revision,
 input_digest,diff_digest,accept_until,state)
 VALUES(10,1,9,1,0,0,0,0,0,0,0,0,randomblob(32),randomblob(32),'2099-01-01T00:00:00Z','opened');
 INSERT INTO cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest,state,gaps,pending,partial,unresolved_overlap)
 VALUES(10,1,?,randomblob(32),randomblob(32),2,0,1,1,0);
 INSERT INTO cjsi_usage_selection(thread,ticket,attempt) VALUES(?,10,1)`, receiptParent, receiptParent)
		res, err = st.SettleTelemetry(ctx, settleAt, 10*time.Minute)
		if err != nil || res.Owned != 1 {
			t.Fatalf("verified clearance did not lift false historical suppression: %+v %v", res, err)
		}
		if pSide := receiptSide(t, db, receiptParent); len(pSide.Events) != 1 || pSide.Events[0].Input+pSide.Events[0].Output != 7 || len(pSide.Markers) != 1 {
			t.Fatalf("cleared P did not settle cleanly: %+v", pSide)
		}
		// Repeated settle pass is a stable no-op.
		if again, err := st.SettleTelemetry(ctx, settleAt.Add(time.Hour), 10*time.Minute); err != nil || again.Changed {
			t.Fatalf("repeated pass after clearance changed state: %+v %v", again, err)
		}

		// 4. Canonical thread matcher: UUIDs match case-insensitively, non-UUIDs match strictly.
		ownershipExec(t, db, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
 VALUES('case-uuid','codex',?,'case-uuid','2026-01-02T10:00:00Z','m',5,'synthetic/u.jsonl',1),
       ('case-nonuuid','codex','NonUUID-Upper','case-nonuuid','2026-01-02T10:00:00Z','m',5,'synthetic/n.jsonl',1)`, strings.ToUpper(receiptThread))
		txCase, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer txCase.Rollback()
		uuidRemains, err := cjsiOTelJSONLRemains(ctx, txCase, receiptThread)
		if err != nil || !uuidRemains {
			t.Fatalf("UUID matcher missed uppercase variant: remains=%v err=%v", uuidRemains, err)
		}
		nonUUIDLower, err := cjsiOTelJSONLRemains(ctx, txCase, "nonuuid-upper")
		if err != nil || nonUUIDLower {
			t.Fatalf("non-UUID matcher matched wrong case: remains=%v err=%v", nonUUIDLower, err)
		}
		nonUUIDExact, err := cjsiOTelJSONLRemains(ctx, txCase, "NonUUID-Upper")
		if err != nil || !nonUUIDExact {
			t.Fatalf("non-UUID matcher missed exact ID: remains=%v err=%v", nonUUIDExact, err)
		}
	})
}

func TestCJSIOTelThreadSettlement(t *testing.T) {
	at := time.Date(2026, 1, 4, 12, 0, 0, 0, time.UTC)
	const record = "2026-01-02T10:00:00Z"
	upper := strings.ToUpper(receiptThread)
	for _, scenario := range []string{"pending conflict", "late conflict", "stored conflict", "surviving alias", "non UUID"} {
		t.Run(scenario, func(t *testing.T) {
			_, st := openCJSI37TestStore(t, false)
			ctx := t.Context()
			first, second := upper, receiptThread
			if scenario == "non UUID" {
				first, second = "Thread-X", "thread-x"
			}
			apply := func(obs TelemetryObservation) TelemetryApplyResult {
				t.Helper()
				r, err := st.ApplyTelemetryObservations(ctx, []TelemetryObservation{obs}, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				return r
			}
			settle := func() {
				t.Helper()
				if _, err := st.SettleTelemetry(ctx, at, time.Minute); err != nil {
					t.Fatal(err)
				}
			}
			apply(receiptObservation("a", first, record, 10))
			if err := st.AcceptInventory(ctx, "complete", 1, nil, at.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			if err := st.FinishInventory(ctx, "complete", "complete"); err != nil {
				t.Fatal(err)
			}
			if scenario == "late conflict" || scenario == "surviving alias" {
				settle()
				if got := receiptSide(t, st.DB, first); len(got.Events) != 1 || len(got.Markers) != 1 {
					t.Fatal("first observation did not settle", got)
				}
			}
			obs := receiptObservation("b", second, record, 20)
			if scenario == "stored conflict" || scenario == "surviving alias" {
				obs.RecordAt = "2026-01-02T11:00:00Z"
			}
			applied := apply(obs)
			if scenario == "late conflict" && (!applied.Changed || applied.Conflict != 1) {
				t.Fatal("late conflict did not retract counted usage", applied)
			}
			if scenario == "stored conflict" {
				// Emulate the two disagreeing raw spellings left by an older receiver.
				ownershipExec(t, st.DB, "UPDATE telemetry_observations SET record_at=? WHERE observation_key='b'", record)
			}
			settle()
			got := receiptSide(t, st.DB, second)
			switch scenario {
			case "non UUID":
				for _, id := range []string{first, second} {
					side := receiptSide(t, st.DB, id)
					if len(side.Events) != 1 || len(side.Markers) != 1 || side.Events[0].Thread != id {
						t.Fatal("non-UUID spellings were merged", id, side)
					}
				}
			case "surviving alias":
				if len(got.Events) != 2 || len(got.Contributions) != 2 || len(got.Markers) != 1 || len(got.Sessions) != 1 ||
					got.Markers[0].First != record || got.Markers[0].Last != obs.RecordAt {
					t.Fatal("UUID variants split the public session", got)
				}
			default:
				if len(got.Observations) != 2 || got.Observations[0].State != "conflict" || got.Observations[1].State != "conflict" ||
					len(got.Contributions)+len(got.Events)+len(got.Sessions)+len(got.Markers) != 0 {
					t.Fatal("conflict retained counted usage", got)
				}
			}
		})
	}
}

func TestCJSIOTelStoredCaptureOnlyConflict(t *testing.T) {
	_, st := openCJSI37TestStore(t, false)
	ctx := t.Context()
	const record = "2026-01-02T10:00:00Z"
	capture := func(key, thread string, input int64) TelemetryObservation {
		o := receiptObservation(key, thread, record, input)
		o.Producer, o.SupportEntry, o.State, o.Reason = "unsupported_producer", "", "capture_only", "producer_unsupported"
		return o
	}

	// Seed the pre-upgrade rows directly to bypass today's conflict detection.
	tx, err := st.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, o := range []TelemetryObservation{
		capture("a", strings.ToUpper(receiptThread), 10),
		receiptObservation("b", receiptThread, record, 20),
		capture("c", strings.ToUpper(receiptNeighbor), 30),
		capture("d", receiptNeighbor, 40),
	} {
		if err := cjsiObservations.insert(ctx, tx, o); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	neighbor := receiptSide(t, st.DB, receiptNeighbor)
	at := time.Date(2026, 1, 4, 12, 0, 0, 0, time.UTC)
	if err := st.AcceptInventory(ctx, "complete", 1, nil, at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishInventory(ctx, "complete", "complete"); err != nil {
		t.Fatal(err)
	}
	res, err := st.SettleTelemetry(ctx, at, time.Minute)
	if err != nil || res != (TelemetrySettleResult{}) {
		t.Fatalf("stored conflict changed usage: %+v err=%v", res, err)
	}
	side := receiptSide(t, st.DB, receiptThread)
	if len(side.Observations) != 2 || side.Observations[0].State != "conflict" || side.Observations[1].State != "conflict" ||
		len(side.Events)+len(side.Contributions)+len(side.Markers)+len(side.Sessions) != 0 {
		t.Fatal("mixed-case disagreement was not neutralized", side)
	}
	if got := receiptSide(t, st.DB, receiptNeighbor); !reflect.DeepEqual(got, neighbor) {
		t.Fatal("settlement changed a capture-only thread", got)
	}
}

func TestCJSIOTelInventoryPromotion(t *testing.T) {
	base := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"update pending only", "current unchecked prefix", "Claude-only checkpoint", "Codex orphan"} {
		t.Run(scenario, func(t *testing.T) {
			_, st := openCJSI37TestStore(t, false)
			ctx := t.Context()
			observations := []TelemetryObservation{receiptObservation("c", receiptThread, "2026-01-02T10:00:00Z", 10)}
			if scenario == "current unchecked prefix" {
				observations = append(observations, receiptObservation("s", receiptNeighbor, "2026-01-02T10:30:00Z", 12))
			}
			if _, err := st.ApplyTelemetryObservations(ctx, observations, nil, nil); err != nil {
				t.Fatal(err)
			}
			if err := st.AcceptInventory(ctx, "old", 1, nil, base); err != nil {
				t.Fatal(err)
			}
			if err := st.FinishInventory(ctx, "old", "complete"); err != nil {
				t.Fatal(err)
			}
			var wantNeed, wantPendingErr bool
			switch scenario {
			case "update pending only":
				ownershipExec(t, st.DB, "UPDATE inventory_runs SET state='update_pending' WHERE inventory_id='old'")
				wantNeed = true
			case "current unchecked prefix":
				src := InventorySource{Path: "/logs/rollout-2026-01-04T10-00-00-" + receiptThread + ".jsonl", Identity: "new", End: 1024}
				if err := st.AcceptInventory(ctx, "new", 2, []InventorySource{src}, base.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				wantNeed, wantPendingErr = true, true
			case "Claude-only checkpoint", "Codex orphan":
				client := "claude"
				if scenario == "Codex orphan" {
					client, wantPendingErr = "codex", true
				}
				path := "/logs/" + client + ".jsonl"
				ownershipExec(t, st.DB, "INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id) VALUES(?,'id',100,100,'h',?)", path, receiptThread)
				ownershipExec(t, st.DB, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
 VALUES('original',?,?,'original','2026-01-02T10:00:00Z','m',5,?,1)`, client, receiptNeighbor, path)
				// File writes invalidate the coverage head. Complete a fresh finite inventory.
				if err := st.AcceptInventory(ctx, "fresh", 2, nil, base.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				if err := st.FinishInventory(ctx, "fresh", "complete"); err != nil {
					t.Fatal(err)
				}
			}
			need, err := st.TelemetryNeedsInventory(ctx)
			if err != nil || need != wantNeed {
				t.Fatalf("needs inventory=%t want %t err=%v", need, wantNeed, err)
			}
			res, err := st.SettleTelemetry(ctx, base.Add(2*time.Hour), time.Minute)
			if (wantPendingErr && !errors.Is(err, ErrTelemetrySourceIdentityPending)) || (!wantPendingErr && err != nil) {
				t.Fatal(res, err)
			}
			side := receiptSide(t, st.DB, receiptThread)
			if scenario == "Claude-only checkpoint" {
				if res.Owned != 1 || len(side.Events) != 1 || len(side.Contributions) != 1 || len(side.Markers) != 1 || side.Events[0].Input != 10 {
					t.Fatal("foreign checkpoint blocked Codex", res, side)
				}
			} else if len(side.Observations) != 1 || side.Observations[0].State != "pending" || len(side.Events)+len(side.Markers)+len(side.Contributions) != 0 {
				t.Fatal("unverified C became public", res, side)
			}
			if scenario == "current unchecked prefix" {
				n := receiptSide(t, st.DB, receiptNeighbor)
				if res.Owned != 1 || len(n.Events) != 1 || n.Events[0].Input != 12 || len(n.Markers) != 1 {
					t.Fatal("unrelated thread did not settle", res, n)
				}
			}
		})
	}
}
