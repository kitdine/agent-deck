package usage

import (
	"fmt"
	"strings"
	"testing"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func testCounterDiscontinuities(t *testing.T) {
	head := ledgerHead + ledgerTurn
	old := oldNotice("09:00:00", -1, 0) + oldNotice("09:05:00", 10, 10)
	request := func(id string, n, total int) string {
		return ledgerNative(id, counter(n), `,"thread_token_usage":`+counter(total))
	}
	for _, gap := range []string{`{}`, `{"total_token_usage":{"input_tokens":-1,"output_tokens":0}}`} {
		notice := fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","thread_id":"%s","turn_id":"t","info":%s}}`+"\n", ledgerThread, gap)
		body := head + old + request("R1", 10, 10) + notice + oldNotice("09:15:00", -1, 0) + oldNotice("09:20:00", 10, 10)
		_, v := ledgerView(t, ledgerInput(t, body))
		if !v.Partial || v.Gaps&ledger.PossibleOverlap == 0 || len(v.Members) != 1 || v.Members[0].Key.ID != "R1" || len(v.Members[0].Proof) != 2 {
			t.Fatal("unproved epoch crossed", v)
		}
	}
	// Only native samples can prove a native rewind. The old 0->10
	// cannot alias a new response that ends at 10 after the 20->10 reset.
	_, v := ledgerView(t, ledgerInput(t, head+old+request("before", 10, 20)+request("after", 10, 10)))
	if !v.Partial || v.Gaps&ledger.PossibleOverlap == 0 || len(v.Members) != 2 {
		t.Fatal("rewound native counter", v)
	}
	for _, m := range v.Members {
		if m.Key.Kind != ledger.Response || len(m.Proof) != 1 {
			t.Fatal("old alias survived rewind", m)
		}
	}
}

func testLegacyRangeOverlap(t *testing.T) {
	base := ledgerHead + ledgerTurn + oldNotice("09:00:00", -1, 0)
	for _, pair := range [][2]string{
		{oldNotice("09:05:00", 10, 10), oldNotice("09:05:01", 10, 10)},
		{oldNotice("09:10:00", 20, 20), oldNotice("09:12:00", 10, 10) + oldNotice("09:15:00", 10, 20)},
	} {
		_, v := ledgerView(t, ledgerInput(t, base+pair[0], base+pair[1]))
		if !v.Partial || v.Gaps&ledger.PossibleOverlap == 0 || len(v.Members) != 0 {
			t.Fatalf("unresolved overlapping legacy keys: %+v", v)
		}
	}
	_, v := ledgerView(t, ledgerInput(t, base+oldNotice("09:05:00", 10, 10), base+oldNotice("09:05:00", 10, 10)))
	if len(v.Members) != 1 || v.Members[0].Tokens.Input != 10 || len(v.Members[0].Proof) != 2 || v.Gaps != 0 {
		t.Fatal("identical old key was not reused", v)
	}
}

func testIntervalModels(t *testing.T) {
	start := ledgerHead + ledgerTurn + oldNotice("09:00:00", -1, 0)
	end := oldNotice("10:10:00", 20, 20)
	req := ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10))
	// Distinct turns give C1 unambiguous contexts at both endpoints. A
	// different model or an unknown record in between invalidates continuity.
	turn := func(id, model string) string {
		return fmt.Sprintf(`{"type":"turn_context","payload":{"turn_id":"%s","model":"%s"}}`+"\n", id, model)
	}
	later := strings.ReplaceAll(end+req, `"turn_id":"t"`, `"turn_id":"later"`)
	for _, tc := range []struct {
		name, want string
		bodies     []string
	}{
		{"whole local context", "m", []string{start + end + req}},
		{"local M N M", "", []string{start + turn("middle", "n") + turn("later", "m") + later}},
		{"unknown record between meters", "", []string{start + `{"type":"unrecognized"}` + "\n" + end + req}},
		{"copies disagree", "", []string{start + end + req, strings.Replace(start, `"model":"m"`, `"model":"q"`, 1) + end + req}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, v := ledgerView(t, ledgerInput(t, tc.bodies...))
			n := 0
			for _, m := range v.Members {
				if m.Key.Kind == ledger.TokenInterval {
					n++
					if m.Model != tc.want || m.Tokens.Input != 10 {
						t.Fatalf("model=%q tokens=%d want %q/10", m.Model, m.Tokens.Input, tc.want)
					}
				}
			}
			if n != 1 {
				t.Fatalf("intervals=%d: %+v", n, v)
			}
		})
	}
}
