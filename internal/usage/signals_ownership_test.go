package usage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// ownershipSourceLines is one Codex session that writes a row into all three
// usage tables for the same turn: a work signal (from the user message), a tool
// call (from the patch), and a usage event (from the token count). A duplicate
// of this exact content under a second path is the conflict the ownership rule
// resolves, and it is the same scenario
// TestUsageToolActivityFollowsDuplicateSourceOwnership already covers.
func ownershipSourceLines(editPath string) []string {
	return []string{
		`{"type":"session_meta","payload":{"session_id":"ownership-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"turn-1","model":"gpt-5.4"}}`,
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:00Z","payload":{"type":"user_message","message":"fix the crash in the parser"}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"call-1","name":"apply_patch","arguments":{"patch":"*** Update File: ` + editPath + `"}}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:03Z","payload":{"item":{"type":"function_call_output","call_id":"call-1","output":"done"}}}`,
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":1}}}}`,
	}
}

// ownedSources is what each of the three tables says about the conflicted
// session. The source is read as a DISTINCT concatenation rather than a single
// value on purpose: a table that has split the session across both paths shows
// up as two paths in one field instead of silently passing whichever row the
// query happened to return first.
type ownedSources struct {
	signal, event, toolCall         string
	signalRows, eventRows, toolRows int
}

func readOwnedSources(t *testing.T, database *store.Store, session string) ownedSources {
	t.Helper()
	ctx := context.Background()
	read := func(table string) (string, int) {
		t.Helper()
		var paths string
		var count int
		query := `SELECT COALESCE(group_concat(DISTINCT source_path),''),COUNT(*) FROM ` + table + ` WHERE session_id=?`
		if err := database.DB.QueryRowContext(ctx, query, session).Scan(&paths, &count); err != nil {
			t.Fatalf("reading %s: %v", table, err)
		}
		return paths, count
	}
	var out ownedSources
	out.signal, out.signalRows = read("usage_work_signals")
	out.event, out.eventRows = read("usage_events")
	out.toolCall, out.toolRows = read("usage_tool_calls")
	return out
}

// assertTablesAgree is where the whole point of this file lives. `usage_events`
// and `usage_tool_calls` carried the duplicate-source rule before work signals
// existed, so they are the definition of the winner; the expectation is read off
// them at run time rather than restated here as a direction. A reversed rule in
// upsertWorkSignalTx fails this whichever way it is reversed, and a prose
// paragraph describing the direction does not — which is how the finding this
// test exists for survived its first repair.
func assertTablesAgree(t *testing.T, got ownedSources, candidates map[string]string) string {
	t.Helper()
	if got.eventRows == 0 || got.toolRows == 0 || got.signalRows == 0 {
		t.Fatalf("a table has no rows for the conflicted session: %+v — the fixture is not exercising the conflict", got)
	}
	if got.event != got.toolCall {
		t.Fatalf("usage_events owner %q and usage_tool_calls owner %q disagree — the baseline this test derives its expectation from is itself broken", got.event, got.toolCall)
	}
	if got.signal != got.event {
		t.Fatalf("usage_work_signals owner %q does not match the owner usage_events and usage_tool_calls name (%q); signals must follow the existing rule, not carry their own", got.signal, got.event)
	}
	role := ""
	for name, path := range candidates {
		if path == got.event {
			role = name
		}
	}
	if role == "" {
		t.Fatalf("owner %q is neither of the conflicting sources %v", got.event, candidates)
	}
	return role
}

// Decision 11's "Ownership and reset": signals, events, and tool calls must name
// the same `source_path` for the same conflict. Three tables disagreeing about
// which source owns a session is invisible from inside any one of them, so the
// assertion is a cross-table comparison and never a hardcoded path.
func TestWorkSignalOwnershipAgreesWithEventsAndToolCalls(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	live := filepath.Join(home, ".codex", "sessions", "2026", "07", "20", "session.jsonl")
	archive := filepath.Join(home, ".codex", "archived_sessions", "session.jsonl")
	editPath := filepath.Join(root, "parser.go")

	writeSource(t, live, ownershipSourceLines(editPath)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	writeSource(t, archive, ownershipSourceLines(editPath)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	got := readOwnedSources(t, database, "ownership-session")
	if got.signalRows != 1 || got.eventRows != 1 || got.toolRows != 1 {
		t.Fatalf("row counts signals=%d events=%d tool_calls=%d, want 1 each — a duplicate source must not split the turn", got.signalRows, got.eventRows, got.toolRows)
	}
	assertTablesAgree(t, got, map[string]string{"live": live, "archive": archive})

	if state, kind, sub, class, source := signalRow(t, database, "codex", "ownership-session", 1); state != signalStateClassified || kind != "debugging" || sub != "repair" || class != "fault" || source != got.event {
		t.Fatalf("signal row = %q %q/%q class=%q source=%q, want a classified debugging/repair owned by %q", state, kind, sub, class, source, got.event)
	}
}

// The winner must be a function of the paths themselves, not of scan order. A
// last-writer rule looks equivalent and is not: it hands the row to whichever
// copy was read second, after which removing that copy deletes a turn whose other
// source was never re-scanned. Both orders are scanned here and the winner is
// compared between them by role, so the test does not have to know the direction.
func TestWorkSignalOwnershipHoldsInBothScanOrders(t *testing.T) {
	winners := map[string]string{}
	for _, order := range [][2]string{{"live", "archive"}, {"archive", "live"}} {
		name := order[0] + "-then-" + order[1]
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			service, database, home := newSignalService(t, root)
			candidates := map[string]string{
				"live":    filepath.Join(home, ".codex", "sessions", "2026", "07", "20", "session.jsonl"),
				"archive": filepath.Join(home, ".codex", "archived_sessions", "session.jsonl"),
			}
			editPath := filepath.Join(root, "parser.go")
			for _, role := range order {
				writeSource(t, candidates[role], ownershipSourceLines(editPath)...)
				if _, err := service.Scan(ctx); err != nil {
					t.Fatal(err)
				}
			}

			got := readOwnedSources(t, database, "ownership-session")
			if got.signalRows != 1 || got.eventRows != 1 || got.toolRows != 1 {
				t.Fatalf("row counts signals=%d events=%d tool_calls=%d, want 1 each", got.signalRows, got.eventRows, got.toolRows)
			}
			winner := assertTablesAgree(t, got, candidates)
			winners[name] = winner

			loser := "live"
			if winner == "live" {
				loser = "archive"
			}
			if err := os.Remove(candidates[loser]); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Scan(ctx); err != nil {
				t.Fatal(err)
			}
			after := readOwnedSources(t, database, "ownership-session")
			if after.signalRows != 1 || after.eventRows != 1 || after.toolRows != 1 {
				t.Fatalf("after removing the losing %s source, row counts signals=%d events=%d tool_calls=%d, want 1 each — the winning row must survive", loser, after.signalRows, after.eventRows, after.toolRows)
			}
			if stillWinner := assertTablesAgree(t, after, candidates); stillWinner != winner {
				t.Fatalf("removing the losing %s source moved ownership from %s to %s", loser, winner, stillWinner)
			}
			if state, kind, sub, _, source := signalRow(t, database, "codex", "ownership-session", 1); state != signalStateClassified || kind != "debugging" || sub != "repair" || source != candidates[winner] {
				t.Fatalf("after removing the losing %s source, signal = %q %q/%q source=%q, want the classified debugging/repair row still owned by %s", loser, state, kind, sub, source, winner)
			}
		})
	}
	if winners["live-then-archive"] != winners["archive-then-live"] {
		t.Fatalf("scan order decided the owner: live-then-archive gave %q, archive-then-live gave %q", winners["live-then-archive"], winners["archive-then-live"])
	}
}

// codexComparableLines and claudeComparableLines are the same three turns of
// work written in each client's log shape: a fault turn that edits, a build turn
// that edits, and a turn that calls no tool at all. Decision 1 puts the turn
// boundary in different places on the two clients — Codex opens a turn on a new
// `turn_context` turn_id, Claude on a user entry that draws an assistant reply —
// so identical work reaching identical classifications is the property that makes
// any per-turn ratio comparable across clients.
func codexComparableLines(editPath string) []string {
	return []string{
		`{"type":"session_meta","payload":{"session_id":"codex-work"}}`,
		`{"type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.4"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:00Z","payload":{"type":"user_message","message":"fix the crash in the parser"}}`,
		`{"type":"response_item","timestamp":"2026-08-27T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"c1","name":"apply_patch","arguments":{"patch":"*** Update File: ` + editPath + `"}}}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`,
		`{"type":"turn_context","payload":{"turn_id":"t2","model":"gpt-5.4"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:01:00Z","payload":{"type":"user_message","message":"add a new endpoint"}}`,
		`{"type":"response_item","timestamp":"2026-08-27T00:01:01Z","payload":{"item":{"type":"function_call","call_id":"c2","name":"apply_patch","arguments":{"patch":"*** Update File: ` + editPath + `"}}}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":11,"output_tokens":2}}}}`,
		`{"type":"turn_context","payload":{"turn_id":"t3","model":"gpt-5.4"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:02:00Z","payload":{"type":"user_message","message":"summarize what we just went through"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:02:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":4,"output_tokens":3}}}}`,
	}
}

func claudeComparableLines(editPath string) []string {
	return []string{
		`{"type":"user","sessionId":"claude-work","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"fix the crash in the parser"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:01Z","sessionId":"claude-work","message":{"role":"assistant","id":"a1","model":"claude-opus","usage":{"input_tokens":10,"output_tokens":1},"content":[{"type":"tool_use","id":"u1","name":"Edit","input":{"file_path":"` + editPath + `"}}]}}`,
		`{"type":"user","sessionId":"claude-work","timestamp":"2026-08-27T00:01:00Z","message":{"role":"user","content":"add a new endpoint"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:01:01Z","sessionId":"claude-work","message":{"role":"assistant","id":"a2","model":"claude-opus","usage":{"input_tokens":11,"output_tokens":2},"content":[{"type":"tool_use","id":"u2","name":"Edit","input":{"file_path":"` + editPath + `"}}]}}`,
		`{"type":"user","sessionId":"claude-work","timestamp":"2026-08-27T00:02:00Z","message":{"role":"user","content":"summarize what we just went through"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:02:01Z","sessionId":"claude-work","message":{"role":"assistant","id":"a3","model":"claude-opus","usage":{"input_tokens":4,"output_tokens":3},"content":[{"type":"text","text":"Here is the summary."}]}}`,
	}
}

// Decision 1's reason for choosing the turn as the unit: it is the only unit
// that means the same thing on both clients. The same three turns of work are
// written in each client's log shape and must reach the same four-category
// verdicts, including the tool-less Claude turn that Decision 3 rule 4 sends to
// `conversation`.
func TestCodexAndClaudeClassifyTheSameWorkComparably(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	editPath := filepath.Join(root, "parser.go")

	writeSource(t, filepath.Join(home, ".codex", "sessions", "comparable.jsonl"), codexComparableLines(editPath)...)
	writeSource(t, filepath.Join(home, ".claude", "projects", "comparable.jsonl"), claudeComparableLines(editPath)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	want := []struct{ kind, sub string }{
		{"debugging", "repair"},
		{"coding", "feature"},
		{"conversation", "exploration"},
	}
	for index, expected := range want {
		turn := index + 1
		codexState, codexKind, codexSub, _, _ := signalRow(t, database, "codex", "codex-work", turn)
		claudeState, claudeKind, claudeSub, _, _ := signalRow(t, database, "claude", "claude-work", turn)
		if codexState != signalStateClassified || claudeState != signalStateClassified {
			t.Fatalf("turn %d states codex=%q claude=%q, want both classified", turn, codexState, claudeState)
		}
		if codexKind != claudeKind || codexSub != claudeSub {
			t.Fatalf("turn %d classified differently across clients: codex %s/%s, claude %s/%s — the same work must be comparable despite different boundary markers", turn, codexKind, codexSub, claudeKind, claudeSub)
		}
		if codexKind != expected.kind || codexSub != expected.sub {
			t.Fatalf("turn %d = %s/%s on both clients, want %s/%s", turn, codexKind, codexSub, expected.kind, expected.sub)
		}
	}

	// Both clients must produce exactly three turns. A Claude turn count that
	// drifts above Codex's is the defect Decision 1 cites when it refuses to
	// treat assistant messages as turn boundaries.
	for _, pair := range [][2]string{{"codex", "codex-work"}, {"claude", "claude-work"}} {
		var turns int
		if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_work_signals WHERE client=? AND session_id=? AND state=?`, pair[0], pair[1], signalStateClassified).Scan(&turns); err != nil {
			t.Fatal(err)
		}
		if turns != 3 {
			t.Fatalf("%s classified turns = %d, want 3", pair[0], turns)
		}
	}

	// The third turn is the one the requirement names: a Claude turn with no
	// tool call at all. Asserting the absence keeps the case honest — with a tool
	// row present, `conversation` could be reached for the wrong reason.
	var claudeTurn3Tools int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_tool_calls WHERE client='claude' AND session_id='claude-work' AND turn_index=3`).Scan(&claudeTurn3Tools); err != nil {
		t.Fatal(err)
	}
	if claudeTurn3Tools != 0 {
		t.Fatalf("claude turn 3 tool calls = %d, want 0 — this case is only meaningful without them", claudeTurn3Tools)
	}
}
