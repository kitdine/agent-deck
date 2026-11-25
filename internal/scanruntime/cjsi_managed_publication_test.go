//go:build cjsi_adapter_fixture

package scanruntime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func TestCJSIManagedRoundPublication(t *testing.T) {
	ctx := t.Context()
	for _, tc := range []struct {
		name  string
		scope Scope
		crash string
	}{{"usage", ScopeUsage, ""}, {"both", ScopeBoth, ""}, {"expired_draft", ScopeBoth, ""}, {"restart_after_session_seal", ScopeBoth, "session"}, {"restart_after_freeze", ScopeBoth, "frozen"}, {"restart_with_orphan", ScopeBoth, "orphan"}, {"restart_after_slot_seal", ScopeBoth, "sealed"}} {
		scope := tc.scope
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			if scope != ScopeUsage {
				index, err := store.OpenSessions(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				if err = index.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.InstallTask5Fixture(ctx, root); err != nil {
				t.Fatal(err)
			}
			m, err := store.ResolveCJSIRuntimeRoot(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			const thread = publicationFixtureThread
			dir := filepath.Join(home, ".codex", "sessions")
			if err = os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "fixture.jsonl")
			body := `{"timestamp":"2026-01-02T09:00:00Z","type":"session_meta","payload":{"id":"` + thread + `"}}
{"type":"turn_context","payload":{"turn_id":"t","model":"m"}}
{"timestamp":"2026-01-02T10:00:00Z","type":"token_usage_record","payload":{"thread_id":"` + thread + `","turn_id":"t","response_id":"r","usage":{"input_tokens":7,"output_tokens":3}}}
{"timestamp":"2026-01-02T10:02:00Z","type":"response_item","payload":{"type":"message","role":"user","content":"visible fixture"}}
`
			if err = os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			planner, inventory := fixturePublicationPlanner(t, home, scope)
			var expiredDraft int64
			if tc.name == "expired_draft" {
				// Persist a reservation with an old business clock, as if the process
				// exited before registering its slot. There is no built_at/valid_until
				// in this row; the selector still knows the registered pricing window.
				selection, e := planner.Select(ctx, CJSIRoundRequest{Root: *m, Home: home, Sources: inventory}, scope)
				if e != nil {
					t.Fatal(e)
				}
				in := selection.Inputs
				in.BuiltAt = selection.BuiltAt.Add(-2 * time.Hour)
				if !selection.ValidUntil(in.BuiltAt).Before(selection.BuiltAt) {
					t.Fatal("fixture has no expired slot window")
				}
				draft, e := m.BeginPublication(ctx, in, selection.AcceptUntil, nil)
				if e != nil {
					t.Fatal(e)
				}
				expiredDraft = draft.Basis().Attempt
				// Neither writer fence may be held while invoking the external pricing/day
				// policy, including when recovery needs the earlier allocation's clock.
				selectInputs := planner.Select
				planner.Select = func(c context.Context, request CJSIRoundRequest, scope Scope) (CJSIRoundSelection, error) {
					selected, e := selectInputs(c, request, scope)
					if e != nil {
						return selected, e
					}
					policy := selected.ValidUntil
					selected.ValidUntil = func(built time.Time) time.Time {
						for _, take := range []func(context.Context, string, time.Duration) (*store.Lock, error){store.AcquireScanLock, store.AcquireLock} {
							lock, err := take(c, root, 0)
							if err != nil {
								t.Errorf("window policy under writer fence: %v", err)
								return time.Time{}
							}
							if err = lock.Release(); err != nil {
								t.Errorf("release window witness: %v", err)
								return time.Time{}
							}
						}
						return policy(built)
					}
					return selected, nil
				}

			}
			var originalSeal [32]byte
			if tc.crash != "" {
				// The helper exits at the precise post-freeze boundary; the
				// detached worker has no access to its plan or SQLite connections.
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCJSIManagedFreezeProcess$")
				cmd.Env = append(os.Environ(), "AGENTDECK_TEST_CJSI_FREEZE_ROOT="+root, "AGENTDECK_TEST_CJSI_FREEZE_HOME="+home, "AGENTDECK_TEST_CJSI_FREEZE_PHASE="+tc.crash)
				output, e := cmd.CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(e, &exit) || exit.ExitCode() != 73 {
					t.Fatalf("freeze process: %v %s", e, output)
				}
				line := strings.TrimSpace(string(output))
				_, sealText, ok := strings.Cut(line, "CJSI_FREEZE_SEAL ")
				b, e := hex.DecodeString(sealText)
				if !ok || e != nil || len(b) != 32 {
					t.Fatalf("missing durable freeze witness: %v", e)
				}
				copy(originalSeal[:], b)

			}
			client := Client{StateRoot: root, Home: home, ForceLocal: true, ManagedPlanner: planner}
			if scope == ScopeBoth {
				workerCtx, cancel := context.WithCancel(ctx)
				var workers sync.WaitGroup
				defer func() { cancel(); workers.Wait() }()
				client.ForceLocal = false
				client.Launch = func(context.Context, string) error {
					workers.Add(1)
					go func() {
						defer workers.Done()
						if e := ServeWithManagedPlanner(workerCtx, root, planner); e != nil {
							t.Errorf("managed worker: %v", e)
						}
					}()
					return nil
				}
			}
			result, err := client.Request(ctx, scope)
			if err != nil || result.Usage.State != "completed" {
				t.Fatalf("managed result %s/%s: %v", result.Usage.State, result.Usage.ErrorCode, err)
			}
			if scope == ScopeUsage && result.Session.State == "completed" {
				t.Fatal("usage-only asserted a complete session scan")
			}
			err = m.WithPublishedCore(ctx, func(tx *sql.Tx, h store.CJSIIndexStamp) error {
				var input int
				if err := tx.QueryRow("SELECT input_tokens FROM usage_events WHERE session_id=?", thread).Scan(&input); err != nil || input != 7 {
					return fmt.Errorf("usage postimage %d: %w", input, err)
				}
				if h.Ticket != 1 {
					return errors.New("head did not advance")
				}
				if scope == ScopeUsage {
					if h.IndexEpoch != 0 {
						return errors.New("usage created an index")
					}
					return nil
				}
				if expiredDraft != 0 {
					var state string
					if e := tx.QueryRow("SELECT state FROM cjsi_publication_attempts WHERE ticket=1 AND attempt=?", expiredDraft).Scan(&state); e != nil || state != "retired" || h.SessionAttempt <= expiredDraft {
						return fmt.Errorf("expired draft remained selected: %s %v", state, e)
					}
				}
				if tc.crash != "" && originalSeal != h.SessionSeal {
					return errors.New("restart selected different document IDs")
				}
				var automatic string
				if err := tx.QueryRow("SELECT automatic FROM session_origins WHERE client='codex' AND session_id=?", thread).Scan(&automatic); err != nil || automatic != "work" {
					return fmt.Errorf("missing published origin %q: %w", automatic, err)
				}
				data, err := m.ReadServingSlot(ctx, tx, time.Now())
				if err != nil || string(data) != "fixture same-ticket slot" {
					return fmt.Errorf("slot readback: %w", err)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

const publicationFixtureThread = "00000000-0000-4000-8000-000000000001"

func fixturePublicationPlanner(t *testing.T, home string, scope Scope) (CJSIPlanner, []ingest.Source) {
	t.Helper()
	ctx := t.Context()
	const thread = publicationFixtureThread
	inventory, err := ingest.Discover(home)
	if err != nil || len(inventory) != 1 {
		t.Fatal(inventory, err)
	}
	src := inventory[0]
	path := src.Path
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	reduction, err := ingest.CaptureCodexReduction(ctx, file, path, src.Size, 1<<16)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	e := reduction.Capture().Evidence()
	e.Version = ingest.CodexCheckpointVersion{Generation: 1, Parser: cjsiledger.Parser, MaxLineBytes: 1 << 16}
	selected := ingest.CodexSelectedSource{Path: path, ThreadID: thread, Epoch: 1, Evidence: e}
	current := func(ctx context.Context) (ingest.CodexSelectedSource, error) { return selected, ingest.Validate(src) }
	now := time.Now().UTC()
	checksum := sha256.Sum256([]byte(body))
	verifyEmpty := func(ctx context.Context, tx *sql.Tx) error {
		var one int
		return tx.QueryRowContext(ctx, `SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM usage_events) AND NOT EXISTS(SELECT 1 FROM operations WHERE kind='provider.use') AND NOT EXISTS(SELECT 1 FROM usage_derived_invalidations)`).Scan(&one)
	}
	planner := CJSIPlanner{Select: func(ctx context.Context, req CJSIRoundRequest, requested Scope) (s CJSIRoundSelection, err error) {
		if scope != requested {
			return s, errors.New("wrong admission scope")
		}
		s.Inputs = store.CJSIPublicationInputs{Digest: checksum, Validate: verifyEmpty, UsageIndependent: func(ctx context.Context, tx *sql.Tx) error {
			var one int
			return tx.QueryRowContext(ctx, "SELECT 1 FROM cjsi_publication_head WHERE index_epoch=0 AND slot_ticket=0").Scan(&one)
		}}
		s.OriginChanges = []CJSIOriginChange{{Input: sessionorigin.OriginChangeInputs{Target: sessionorigin.Target{Client: "codex", SessionID: thread}}}}
		s.BuiltAt, s.AcceptUntil = now, now.Add(time.Minute)
		s.ValidUntil = func(built time.Time) time.Time { return built.Add(time.Hour) }
		s.Groups = []CJSIGroupInputs{{Thread: thread, History: cjsiledger.History{Assessed: true, Rows: []cjsiledger.OldRow{}}, Sources: []CJSISourcePreparation{{
			Source: src, Index: session.CJSISourceInput{Selected: selected, PrefixHash: fmt.Sprintf("%x", checksum), ScannedAt: now.Format(time.RFC3339Nano), Priority: src.Priority, Size: src.Size, ModifiedAt: src.ModifiedAt, ChangedAt: src.ChangedAt},
			Current: current, Limits: ingest.CodexWorksetLimits{Turns: 32, SegmentRecords: 64}, MaxMeters: 64, MaxBytes: 1 << 20, StepBytes: 256,
		}}, Receipts: func(st cjsiledger.ProposalStamp) store.CJSIOTelReceiptStore {
			return store.CJSIOTelSQLiteStore{Guard: func(ctx context.Context, tx *sql.Tx, id string) (store.CJSIOTelGuard, error) {
				return store.CJSIOTelGuard{Thread: id, Stamp: st, Revision: 1}, nil
			}}
		}}}
		s.Changes = store.CJSICombinedChanges{Digest: checksum, Validate: verifyEmpty, RecheckSources: func(context.Context) error { return ingest.Validate(src) }, ApplySelected: func(context.Context, *sql.Tx) error { return nil },
			VerifyOpened: func(ctx context.Context, tx *sql.Tx) error {
				var one int
				return tx.QueryRowContext(ctx, "SELECT 1 FROM usage_events WHERE session_id=? AND input_tokens=7 AND output_tokens=3", thread).Scan(&one)
			}}
		err = req.Root.WithPublishedCore(ctx, func(tx *sql.Tx, h store.CJSIIndexStamp) error {
			s.Changes.SessionRevision = h.SessionRevision + 1
			s.Changes.CoverageRevision = h.CoverageRevision
			if e := tx.QueryRow("SELECT revision+3 FROM serving_generation").Scan(&s.Changes.UsageRevision); e != nil {
				return e
			}
			if scope == ScopeBoth {
				s.Changes.UsageRevision++
			}
			return nil
		})
		s.Build = func(ctx context.Context, proposed CJSIProposedView) (func() ([]byte, error), error) {
			tx, view := proposed.Core, proposed.Index
			if len(proposed.Groups) != 1 || proposed.Generation.SessionRevision != view.Selection().SessionRevision {
				return nil, errors.New("unpaired proposed view")
			}
			group := proposed.Groups[0]
			v, e := group.Usage.Proposal().Eligibility(group.Draft.Stamp, thread)
			if e != nil || len(v.Members) != 1 || v.Members[0].Tokens.Input != 7 || v.Members[0].Tokens.Output != 3 {
				return nil, errors.New("incorrect proposed usage")
			}

			var count int
			if e := tx.QueryRow("SELECT count(*) FROM usage_events").Scan(&count); e != nil || count != 0 {
				return nil, errors.New("unopened facts leaked")
			}
			if e := view.Tx.QueryRow("SELECT count(*) FROM ("+view.CopiesSQL()+") WHERE session_id=?", thread).Scan(&count); e != nil || count != 1 {
				return nil, errors.New("proposed session not selected")
			}
			return func() ([]byte, error) { return []byte("fixture same-ticket slot"), nil }, nil
		}
		return
	}}

	return planner, inventory
}

func TestCJSIManagedFreezeProcess(t *testing.T) {
	root := os.Getenv("AGENTDECK_TEST_CJSI_FREEZE_ROOT")
	if root == "" {
		t.Skip("subprocess entry")
	}
	ctx := t.Context()
	home := os.Getenv("AGENTDECK_TEST_CJSI_FREEZE_HOME")
	m, err := store.ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil || m == nil {
		t.Fatal(err)
	}
	planner, inventory := fixturePublicationPlanner(t, home, ScopeBoth)
	request := CJSIRoundRequest{Root: *m, Home: home, Sources: inventory}
	plan, _, err := planner.Combined(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	phase := os.Getenv("AGENTDECK_TEST_CJSI_FREEZE_PHASE")
	stop := func() { fmt.Printf("CJSI_FREEZE_SEAL %x\n", plan.Index.Seal()); os.Exit(73) }
	switch phase {
	case "session":
		plan.PrepareOrigins = func(context.Context, *sql.Tx, store.CJSIIndexSnapshot, store.CJSIIndexSnapshot) ([]sessionorigin.PreparedOrigin, error) {
			stop()
			return nil, errors.New("unreachable")
		}
	case "frozen":
		plan.Build = func(context.Context, CJSIProposedView) (func() ([]byte, error), error) {
			stop()
			return nil, errors.New("unreachable")
		}
	case "orphan", "sealed":
		check := plan.Changes.Validate
		plan.Changes.Validate = func(ctx context.Context, tx *sql.Tx) error {
			var state string
			if e := tx.QueryRowContext(ctx, "SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", plan.Draft.Basis().Ticket, plan.Draft.Basis().Attempt).Scan(&state); e != nil {
				return e
			}
			if phase == "sealed" && state == "sealed" {
				stop()
			}
			if phase == "orphan" && state == "draft" {
				if _, e := os.Lstat(filepath.Join(root, fmt.Sprintf(".cjsi-serving-slot-%x", plan.Draft.SlotIdentity()))); e == nil {
					stop()
				} else if !errors.Is(e, os.ErrNotExist) {
					return e
				}
			}
			return check(ctx, tx)
		}
	default:
		t.Fatal("unknown crash phase", phase)
	}
	t.Fatal("writer did not reach its requested crash point", plan.Publish(ctx))
}
