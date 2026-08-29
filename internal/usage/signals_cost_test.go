package usage

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// Two models, one in the fixture catalog and one absent from it. The absent one
// is how a scope reaches Decision 4's `none`: every figure it produces is
// 0.000000000, and only the discriminator separates that from a measured zero.
const (
	pricedFixtureModel   = "claude-priced"
	unpricedFixtureModel = "claude-unpriced"
)

// signalCostWindow spans every timestamp used by the fixtures below.
func signalCostWindow() (time.Time, time.Time) {
	return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
}

// seedSignalCostPrices prices input at $3 per million so a round token count
// produces a round cost and the assertions can name it exactly.
func seedSignalCostPrices(t *testing.T, database *store.Store) {
	t.Helper()
	if _, err := database.Exec(context.Background(), `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES ('fixture','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES ('fixture','`+pricedFixtureModel+`','anthropic','2026-01-01T00:00:00Z','{"input":"3.000000000","output":"15.000000000"}','[]')`); err != nil {
		t.Fatal(err)
	}
}

func userLine(session, at, text string) string {
	return `{"type":"user","sessionId":"` + session + `","timestamp":"` + at + `","message":{"role":"user","content":"` + text + `"}}`
}

// assistantLine emits one API call, which is what makes the preceding message a
// turn. An empty tool name produces a chat-only reply.
func assistantLine(session, at, id, model string, inputTokens int64, tool, path string) string {
	content := `[{"type":"text","text":"ok"}]`
	if tool != "" {
		content = `[{"type":"tool_use","id":"` + id + `-tool","name":"` + tool + `","input":{"file_path":"` + path + `"}}]`
	}
	return `{"type":"assistant","timestamp":"` + at + `","sessionId":"` + session + `","message":{"role":"assistant","id":"` + id +
		`","model":"` + model + `","usage":{"input_tokens":` + strconv.FormatInt(inputTokens, 10) + `,"output_tokens":0},"content":` + content + `}}`
}

func costKind(t *testing.T, cost ActivityCost, kind string) ActivityCostKind {
	t.Helper()
	for _, row := range cost.Kinds {
		if row.Kind == kind {
			return row
		}
	}
	t.Fatalf("no %q row among %d kinds", kind, len(cost.Kinds))
	return ActivityCostKind{}
}

func costSubKinds(row ActivityCostKind) []string {
	names := make([]string, 0, len(row.Sub))
	for _, sub := range row.Sub {
		names = append(names, sub.Kind)
	}
	return names
}

// The normal case: every event in scope reached a classified turn, so the cost
// figures cover the whole scope and the basis says so. The four categories are
// always present, because the surfaces render four rows whatever the data holds.
func TestActivityCostAttributesEventsThroughTheirTurns(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "attributed.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 200000, "Edit", editPath),
		userLine("s", "2026-08-27T00:00:02Z", "what do you think of this"),
		assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 100000, "", ""),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	cost, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if cost.CostBasis != CostBasisTurn {
		t.Fatalf("cost_basis = %q, want %q", cost.CostBasis, CostBasisTurn)
	}
	if len(cost.Kinds) != 4 {
		t.Fatalf("kinds = %d, want the four categories always", len(cost.Kinds))
	}
	coding := costKind(t, cost, "coding")
	if coding.Cost != "0.600000000" || coding.Events != 1 || coding.Turns != 1 || coding.Share != 66.7 {
		t.Fatalf("coding = %+v, want $0.60 over one event in one turn at 66.7%%", coding)
	}
	conversation := costKind(t, cost, "conversation")
	if conversation.Cost != "0.300000000" || conversation.Events != 1 || conversation.Turns != 1 || conversation.Share != 33.3 {
		t.Fatalf("conversation = %+v, want $0.30 over one event in one turn at 33.3%%", conversation)
	}
	for _, empty := range []string{"delegation", "debugging"} {
		row := costKind(t, cost, empty)
		if row.Cost != "0.000000000" || row.Events != 0 || row.Turns != 0 || row.Share != 0 {
			t.Fatalf("%s = %+v, want a measured-zero row rather than an omission", empty, row)
		}
	}
	// The subcategory carries the same figure as its parent when it is the only
	// one populated, and the share is taken against the scope rather than
	// against the parent — 24 of 52, not 46 of 100, in the contract's example.
	if len(coding.Sub) != 4 || coding.Sub[0].Kind != "feature" || coding.Sub[0].Cost != "0.600000000" || coding.Sub[0].Share != 66.7 {
		t.Fatalf("coding subcategories = %+v, want feature carrying the parent's figure", coding.Sub)
	}
}

// Decision 4's `partial`: an event that predates the backfill carries no
// turn_index, so its cost reaches no category. The figures still render, and the
// discriminator is the only thing that says they are short.
func TestActivityCostIsPartialWhenAnEventCarriesNoTurn(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "partial.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 200000, "Edit", editPath),
		userLine("s", "2026-08-27T00:00:02Z", "what do you think of this"),
		assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 100000, "", ""),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	before, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if before.CostBasis != CostBasisTurn {
		t.Fatalf("cost_basis before = %q, want %q so the change below is the only difference", before.CostBasis, CostBasisTurn)
	}
	// Strip one event's turn_index in place, which is the shape a source indexed
	// before the migration leaves behind.
	if _, err = database.Exec(ctx, `UPDATE usage_events SET turn_index=NULL WHERE client='claude' AND session_id='s' AND turn_index=2`); err != nil {
		t.Fatal(err)
	}
	after, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if after.CostBasis != CostBasisPartial {
		t.Fatalf("cost_basis after = %q, want %q", after.CostBasis, CostBasisPartial)
	}
	if row := costKind(t, after, "conversation"); row.Cost != "0.000000000" || row.Events != 0 || row.Turns != 0 {
		t.Fatalf("conversation = %+v, want the unattributable event excluded rather than guessed into a category", row)
	}
	if row := costKind(t, after, "coding"); row.Cost != "0.600000000" || row.Share != 100 {
		t.Fatalf("coding = %+v, want the remaining attributed cost renormalized over what is covered", row)
	}
}

// Decision 4's `none`: the scope holds events, but none of them is priced, so
// every figure is 0.000000000 and none of them is a measurement. The basis is
// what carries that, which is why the surfaces render unavailable rather than
// reading the number.
func TestActivityCostIsNoneWhenNoPricedEventCoversTheScope(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "unpriced.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", unpricedFixtureModel, 200000, "Edit", editPath),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	cost, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if cost.CostBasis != CostBasisNone {
		t.Fatalf("cost_basis = %q, want %q", cost.CostBasis, CostBasisNone)
	}
	coding := costKind(t, cost, "coding")
	if coding.Cost != "0.000000000" || coding.Share != 0 {
		t.Fatalf("coding = %+v, want the zero that only the basis distinguishes from a measurement", coding)
	}
	if coding.Turns != 1 || coding.Events != 1 {
		t.Fatalf("coding = %+v, want the turn still counted — the classification exists, the price does not", coding)
	}
	var rows int
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE client='claude' AND session_id='s'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows == 0 {
		t.Fatal("no usage event in scope, which would make this test pass for the wrong reason")
	}
}

// Decision 11 makes a `pending` row a turn that does not exist yet. Its events
// must reach no category: counting them would inflate the turn count and file
// spend under a classification nobody has made.
func TestPendingTurnIsInvisibleToCostAttribution(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "pending.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 200000, "Edit", editPath),
		userLine("s", "2026-08-27T00:00:02Z", "add the eviction policy"),
		assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 100000, "Edit", editPath),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	before, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if row := costKind(t, before, "coding"); row.Turns != 2 || row.Cost != "0.900000000" {
		t.Fatalf("coding before = %+v, want both classified turns counted", row)
	}
	if _, err = database.Exec(ctx, `UPDATE usage_work_signals SET state='pending',activity_kind='',activity_sub='' WHERE client='claude' AND session_id='s' AND turn_index=2`); err != nil {
		t.Fatal(err)
	}
	after, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	coding := costKind(t, after, "coding")
	if coding.Turns != 1 || coding.Events != 1 || coding.Cost != "0.600000000" {
		t.Fatalf("coding after = %+v, want the pending turn invisible to every figure", coding)
	}
	if after.CostBasis != CostBasisPartial {
		t.Fatalf("cost_basis = %q, want %q — the pending turn's spend is not covered by the figures", after.CostBasis, CostBasisPartial)
	}
}

// Decision 5's stated reason for dividing by cost: a session of many short
// conversation turns and one long coding turn is a coding session, and counting
// turns would call it a conversation.
func TestSessionCategoryFollowsCostShareNotTurnCount(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "dominant.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "ok"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 1000, "", ""),
		userLine("s", "2026-08-27T00:00:02Z", "sure"),
		assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 1000, "", ""),
		userLine("s", "2026-08-27T00:00:04Z", "go on"),
		assistantLine("s", "2026-08-27T00:00:05Z", "m3", pricedFixtureModel, 1000, "", ""),
		userLine("s", "2026-08-27T00:00:06Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:07Z", "m4", pricedFixtureModel, 500000, "Edit", editPath),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	category, err := service.SessionActivityCategory(ctx, "claude", "s")
	if err != nil {
		t.Fatal(err)
	}
	if category.CostBasis != CostBasisTurn || category.Kind != "coding" {
		t.Fatalf("session category = %+v, want coding despite conversation holding three of the four turns", category)
	}
	from, to := signalCostWindow()
	cost, err := service.ActivityCostRange(ctx, from, to, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if row := costKind(t, cost, "conversation"); row.Turns != 3 {
		t.Fatalf("conversation turns = %d, want 3 — this test is only meaningful when turn count would pick the other answer", row.Turns)
	}
}

// The tie-breaks are ordered, and one example cannot show that. Equal cost with
// unequal turn counts must pick the larger count even when precedence points the
// other way; equal cost and equal counts must fall through to precedence.
func TestSessionCategoryTieBreaksByTurnCountThenPrecedence(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name  string
		lines func(edit string) []string
		want  string
	}{
		{
			// debugging outranks coding in precedence, and coding wins anyway on
			// two turns against one at the same total cost.
			name: "turn count outranks precedence",
			want: "coding",
			lines: func(edit string) []string {
				return []string{
					userLine("s", "2026-08-27T00:00:00Z", "fix the crash"),
					assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 200000, "Edit", edit),
					userLine("s", "2026-08-27T00:00:02Z", "add the eviction policy"),
					assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 100000, "Edit", edit),
					userLine("s", "2026-08-27T00:00:04Z", "implement the resize"),
					assistantLine("s", "2026-08-27T00:00:05Z", "m3", pricedFixtureModel, 100000, "Edit", edit),
				}
			},
		},
		{
			// Equal on both, so Decision 3's precedence order decides and
			// debugging comes before coding.
			name: "precedence decides an exact tie",
			want: "debugging",
			lines: func(edit string) []string {
				return []string{
					userLine("s", "2026-08-27T00:00:00Z", "fix the crash"),
					assistantLine("s", "2026-08-27T00:00:01Z", "m1", pricedFixtureModel, 100000, "Edit", edit),
					userLine("s", "2026-08-27T00:00:02Z", "add the eviction policy"),
					assistantLine("s", "2026-08-27T00:00:03Z", "m2", pricedFixtureModel, 100000, "Edit", edit),
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			service, database, home := newSignalService(t, root)
			seedSignalCostPrices(t, database)
			source := filepath.Join(home, ".claude", "projects", "tie.jsonl")
			writeSource(t, source, test.lines(filepath.Join(root, "cache.go"))...)
			if _, err := service.Scan(ctx); err != nil {
				t.Fatal(err)
			}
			from, to := signalCostWindow()
			cost, err := service.ActivityCostRange(ctx, from, to, "claude")
			if err != nil {
				t.Fatal(err)
			}
			debugging, coding := costKind(t, cost, "debugging"), costKind(t, cost, "coding")
			if debugging.Cost != coding.Cost {
				t.Fatalf("debugging %s and coding %s are not tied, so this case tests nothing", debugging.Cost, coding.Cost)
			}
			category, err := service.SessionActivityCategory(ctx, "claude", "s")
			if err != nil {
				t.Fatal(err)
			}
			if category.Kind != test.want {
				t.Fatalf("session category = %q, want %q", category.Kind, test.want)
			}
		})
	}
}

// When the reduction has no input it is omitted rather than guessed: the line
// prints its counted values alone.
func TestSessionCategoryIsOmittedWhenCostBasisIsNone(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".claude", "projects", "nocost.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:01Z", "m1", unpricedFixtureModel, 200000, "Edit", editPath),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	category, err := service.SessionActivityCategory(ctx, "claude", "s")
	if err != nil {
		t.Fatal(err)
	}
	if category.CostBasis != CostBasisNone || category.Kind != "" {
		t.Fatalf("session category = %+v, want the category omitted at basis none", category)
	}
}

// Decision 3: a subcategory with no signal for the selected client is omitted
// from the expanded list rather than rendered as a permanently empty row. Codex
// has no skill tool, so `delegation/workflow` is that subcategory.
func TestActivityCostOmitsTheSubcategoryTheClientCannotSignal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	source := filepath.Join(home, ".codex", "sessions", "codex.jsonl")
	editPath := filepath.Join(root, "cache.go")

	writeSource(t, source,
		`{"type":"session_meta","payload":{"session_id":"s"}}`,
		`{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5.6"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:00Z","payload":{"type":"user_message","message":"implement the cache"}}`,
		`{"type":"response_item","timestamp":"2026-08-27T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"c","name":"apply_patch","arguments":{"patch":"*** Update File: `+editPath+`"}}}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	codex, err := service.ActivityCostRange(ctx, from, to, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if names := costSubKinds(costKind(t, codex, "delegation")); len(names) != 1 || names[0] != "subagent" {
		t.Fatalf("codex delegation subcategories = %v, want workflow omitted rather than shown as a zero row", names)
	}
	every, err := service.ActivityCostRange(ctx, from, to, "")
	if err != nil {
		t.Fatal(err)
	}
	if names := costSubKinds(costKind(t, every, "delegation")); len(names) != 2 || names[1] != "workflow" {
		t.Fatalf("all-client delegation subcategories = %v, want workflow present where a client can signal it", names)
	}
}
