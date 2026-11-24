package usage

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Replay canonicalizes sources by their selected name digest. Assign the
// fixture bodies in that order so reversing their evidence exercises both
// possible merge orders regardless of the temporary directory name.
func orderedLedgerCopies(t *testing.T, bodies ...string) ledger.Eligibility {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, len(bodies))
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("%d.jsonl", i))
	}
	slices.SortFunc(paths, func(a, b string) int {
		x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
		return bytes.Compare(x[:], y[:])
	})
	in := ledgerInput(t)
	for i, path := range paths {
		src, stamp, err := ledgerStage(t, path, bodies[i], nil, 32)
		if err != nil {
			t.Fatal(err)
		}
		in.Sources = append(in.Sources, src)
		in.Expected = append(in.Expected, stamp)
	}
	_, v := ledgerView(t, in)
	return v
}
func meterWithCached(input, cached int) string {
	if cached < 0 {
		return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":0}`, input)
	}
	return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":0,"cached_input_tokens":%d}`, input, cached)
}
func testResidualDetails(t *testing.T) {
	req := ledgerNative("R", meterWithCached(10, -1), `,"thread_token_usage":`+meterWithCached(10, -1))
	head := ledgerHead + ledgerTurn
	mixed := func(cache int) string {
		baseline := 0
		if cache < 0 {
			baseline = -1
		}
		old := strings.ReplaceAll(oldNotice("09:00:00", -1, 0), counter(0), meterWithCached(0, baseline))
		last := strings.ReplaceAll(oldNotice("10:10:00", 20, 20), counter(20), meterWithCached(20, cache))
		return head + old + last + req
	}
	anonymous := func(cache int) string {
		return head + req + ledgerNative("", meterWithCached(5, cache), `,"thread_token_usage":`+meterWithCached(15, cache))
	}
	for _, tc := range []struct {
		name                  string
		body                  func(int) string
		cached, total, lo, hi int64
		residualKnown         bool
	}{
		{"legacy remainder", mixed, 4, 20, 0, 20, false}, {"anonymous native", anonymous, 2, 15, 10, 15, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var key ledger.Key
			for _, bodies := range [][]string{{tc.body(-1), tc.body(int(tc.cached))}, {tc.body(int(tc.cached)), tc.body(-1)}} {
				v := orderedLedgerCopies(t, bodies...)
				if len(v.Members) != 2 {
					t.Fatalf("wanted one call and one interval, got %v", v.Members)
				}
				call, m := v.Members[0], v.Members[1]
				r := m.Interval
				if call.Key.Kind != ledger.Response || m.Key.Kind != ledger.TokenInterval || call.Tokens.Input+m.Tokens.Input != tc.total || r.Lower.Input != tc.lo || r.Upper.Input != tc.hi {
					t.Fatal(call, m)
				}
				if r.Lower.Known != ledger.CachedKnown || r.Lower.Cached != 0 || r.Upper.Known != ledger.CachedKnown || r.Upper.Cached != tc.cached {
					t.Fatalf("lost or invented endpoint evidence %+v", r)
				}
				if (m.Tokens.Known&ledger.CachedKnown != 0) != tc.residualKnown || tc.residualKnown && m.Tokens.Cached != tc.cached {
					t.Fatal("unproven or missing cached remainder", m.Tokens)
				}
				if key.ID != "" && key != m.Key {
					t.Fatal("order-dependent interval key", key, m.Key)
				}
				key = m.Key
			}
		})
	}
	// A sparse copy is compatible with either of two mutually contradictory
	// endpoints. It must never resurrect that range after its evidence conflicts.
	for _, copies := range [][]string{{mixed(-1), mixed(2), mixed(4)}, {mixed(2), mixed(4), mixed(-1)}, {mixed(4), mixed(-1), mixed(2)}} {
		v := orderedLedgerCopies(t, copies...)
		if len(v.Members) != 1 || v.Members[0].Key.Kind != ledger.Response || v.Members[0].Tokens.Input != 10 || v.Gaps&ledger.PossibleOverlap == 0 {
			t.Fatalf("conflicting interval survived: %+v", v)
		}
	}
}
func testContainedModels(t *testing.T) {
	turn := func(id, m string) string {
		return fmt.Sprintf(`{"type":"turn_context","payload":{"turn_id":"%s","model":"%s"}}`+"\n", id, m)
	}
	req := ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10))
	old := ledgerHead + ledgerTurn + oldNotice("09:00:00", -1, 0) + oldNotice("10:10:00", 20, 20)
	later := strings.ReplaceAll(req, `"turn_id":"t"`, `"turn_id":"later"`)
	for _, tc := range []struct {
		name, model string
		bodies      []string
	}{
		{"contained later model", "", []string{old + turn("later", "q") + later}},
		{"conflicting copy last", "", []string{old + req, ledgerHead + turn("t", "q") + req}},
		{"conflicting copy first", "", []string{ledgerHead + turn("t", "q") + req, old + req}},
		{"unknown copy", "", []string{ledgerHead + req, old + req}},
		{"all agree", "m", []string{old + req, ledgerHead + ledgerTurn + req}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := orderedLedgerCopies(t, tc.bodies...)
			if len(v.Members) != 2 || v.Members[1].Key.Kind != ledger.TokenInterval || v.Members[1].Tokens.Input != 10 || v.Members[1].Model != tc.model {
				t.Fatalf("want one call and 10-token interval model %q: %+v", tc.model, v)
			}
		})
	}
}
