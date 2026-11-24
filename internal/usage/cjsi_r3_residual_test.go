package usage

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func residualVector(input, output int, details string) string {
	return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d%s}`, input, output, details)
}
func residualNotice(at string, total, last string) string {
	fields := `"total_token_usage":` + total
	if last != "" {
		fields += `,"last_token_usage":` + last
	}
	return fmt.Sprintf(`{"timestamp":"2026-01-02T%sZ","type":"event_msg","payload":{"type":"token_count","thread_id":"%s","turn_id":"t","info":{%s}}}`+"\n", at, ledgerThread, fields)
}

func testGlobalResidualVeto(t *testing.T) {
	const c0 = `,"cached_input_tokens":0`
	const c2 = `,"cached_input_tokens":2`
	const c3 = `,"cached_input_tokens":3`
	const c4 = `,"cached_input_tokens":4`
	const c5 = `,"cached_input_tokens":5`
	const w0 = `,"cache_write_input_tokens":0`
	const w2 = `,"cache_write_input_tokens":2`
	const w3 = `,"cache_write_input_tokens":3`
	call := func(id string, in, out int, detail string, hiIn, hiOut int, hiDetail string) string {
		return ledgerNative(id, residualVector(in, out, detail), `,"thread_token_usage":`+residualVector(hiIn, hiOut, hiDetail))
	}
	copyFor := func(upperIn, upperOut int, lo, hi, requests string) string {
		end := residualVector(upperIn, upperOut, hi)
		return ledgerHead + ledgerTurn + residualNotice("09:00:00", residualVector(0, 0, lo), "") + residualNotice("10:10:00", end, end) + requests
	}
	type scenario struct {
		nativeProofOnly  string    // exact native line for a vetoed legacy mirror
		lowerAt, upperAt time.Time // expected window supplied by complete witnesses
		untimedOnly      bool
		intervalRefs     int
		name             string
		bodies           [3]string
		requests         int
		left             ledger.Counts
		lower, upper     int64 // input endpoints of the only eligible remainder
		overlap          bool
	}
	var cases []scenario
	add := func(name string, in, out int, requests string, n int, details [3][2]string, left ledger.Counts, overlap bool) {
		tc := scenario{intervalRefs: 6, name: name, requests: n, left: left, upper: int64(in), overlap: overlap}
		for i, d := range details {
			tc.bodies[i] = copyFor(in, out, d[0], d[1], requests)
		}
		cases = append(cases, tc)
	}
	r := call("R1", 10, 0, c5, 10, 0, c5)
	add("sparse copy after local containment rejection", 20, 0, r, 1, [3][2]string{{c0, c4}, {"", ""}, {c0, ""}}, ledger.Counts{}, true)
	r = call("R1", 10, 0, c2, 10, 0, c2) + call("R2", 10, 0, c3, 20, 0, "")
	add("split endpoints below sum of two claims", 30, 0, r, 2, [3][2]string{{c0, ""}, {"", c4}, {"", ""}}, ledger.Counts{}, true)
	add("split endpoints sufficient for both claims", 30, 0, r, 2, [3][2]string{{c0, ""}, {"", `,"cached_input_tokens":6`}, {"", ""}}, ledger.Counts{Input: 10, Cached: 1, Known: ledger.CachedKnown}, false)
	r = call("R1", 10, 0, c2+w3, 10, 0, c2+w3)
	add("cache and write contradiction", 20, 0, r, 1, [3][2]string{{c0, c4}, {w0, w2}, {"", ""}}, ledger.Counts{}, true)
	add("compatible cache and write", 20, 0, r, 1, [3][2]string{{c0, c4}, {w0, `,"cache_write_input_tokens":4`}, {"", ""}}, ledger.Counts{Input: 10, Cached: 2, Write: 1, Known: ledger.CachedKnown | ledger.WriteKnown}, false)
	r = call("R1", 10, 0, "", 10, 0, "") + call("R2", 10, 0, c5, 20, 0, "")
	add("unknown detail cannot erase known lower bound", 30, 0, r, 2, [3][2]string{{c0, ""}, {"", c4}, {"", ""}}, ledger.Counts{}, true)
	add("unknown detail keeps a feasible remainder unallocated", 30, 0, r, 2, [3][2]string{{c0, ""}, {"", `,"cached_input_tokens":6`}, {"", ""}}, ledger.Counts{Input: 10}, false)
	r = call("R1", 10, 0, "", 10, 0, "") + call("R2", 10, 0, c0, 20, 0, "")
	add("known uncached input cannot exceed whole uncached input", 30, 0, r, 2, [3][2]string{{c0, ""}, {"", `,"cached_input_tokens":25`}, {"", ""}}, ledger.Counts{}, true)
	r = call("R1", 2, 4, `,"reasoning_output_tokens":3`, 2, 4, "") + call("R2", 2, 4, `,"reasoning_output_tokens":2`, 4, 8, "")
	add("split reasoning bounds", 8, 12, r, 2, [3][2]string{{`,"reasoning_output_tokens":0`, ""}, {"", `,"reasoning_output_tokens":4`}, {"", ""}}, ledger.Counts{}, true)
	r = call("R1", 5, 0, c4, 10, 0, `,"cached_input_tokens":8`) + call("R2", 5, 0, "", 20, 0, "") + call("R3", 5, 0, c2, 30, 0, c2)
	add("unknown intervening position cannot conceal a counter rewind", 40, 0, r, 3, [3][2]string{{"", ""}, {c0, ""}, {"", ""}}, ledger.Counts{}, true)
	r = call("R1", 10, 0, c5, 10, 0, c5) + call("R2", 10, 0, "", 40, 0, "")
	independent := scenario{name: "veto stays local to its primary range", requests: 2, left: ledger.Counts{Input: 10}, lower: 30, upper: 50, overlap: true, intervalRefs: 2}
	independent.bodies = [3]string{
		copyFor(20, 0, c0, c4, r), copyFor(20, 0, "", "", r),
		ledgerHead + ledgerTurn + residualNotice("10:15:00", residualVector(30, 0, c5), "") + residualNotice("11:05:00", residualVector(50, 0, ""), residualVector(20, 0, "")) + r,
	}
	cases = append(cases, independent)
	// One copy fails the original time window. The numeric final check alone
	// would accept the sparse copies; the once-rejected key must stay absent.
	good := copyFor(20, 0, c0, c4, call("R1", 10, 0, c2, 10, 0, c2))
	cases = append(cases, scenario{name: "one bad window permanently vetoes a feasible range", requests: 1, overlap: true, bodies: [3]string{good, strings.Replace(good, "10:10:00Z", "08:00:00Z", 1), good}})
	// The old 0->10 mirror is an exact match on the first copy. A second
	// copy disproves the range. The third contains only the native response.
	exact := call("R1", 10, 0, c4, 10, 0, c4)
	cases = append(cases, scenario{name: "vetoed exact mirror leaves only native proof", requests: 1, overlap: true, nativeProofOnly: exact,
		bodies: [3]string{copyFor(10, 0, c0, c4, exact), copyFor(10, 0, "", c3, exact), ledgerHead + ledgerTurn + exact}})
	instant := func(hour, minute int) time.Time { return time.Date(2026, 1, 2, hour, minute, 0, 0, time.UTC) }
	early := strings.Replace(good, "09:00:00Z", "08:55:00Z", 1)
	late := strings.Replace(good, "10:10:00Z", "10:20:00Z", 1)
	left := ledger.Counts{Input: 10, Cached: 2, Known: ledger.CachedKnown}
	cases = append(cases, scenario{name: "union of complete original windows", requests: 1, left: left, upper: 20, intervalRefs: 6,
		lowerAt: instant(8, 55), upperAt: instant(10, 20), bodies: [3]string{good, early, late}})
	omit := func(body, at string) string { return strings.Replace(body, `"timestamp":"2026-01-02T`+at+`Z",`, "", 1) }
	// The surviving bounds come from the complete copy alone. Neither
	// extending the window nor pairing two incomplete copies is justified.
	noUpper := omit(early, "10:10:00")
	noLower := omit(late, "09:00:00")
	cases = append(cases, scenario{name: "untimed copies defer to complete window", requests: 1, left: left, upper: 20, intervalRefs: 2,
		lowerAt: instant(9, 0), upperAt: instant(10, 10), bodies: [3]string{noUpper, good, noLower}})
	cases = append(cases, scenario{name: "incomplete copies cannot assemble a time window", requests: 1, untimedOnly: true,
		bodies: [3]string{noUpper, noLower, omit(noLower, "10:20:00")}})
	// Only absence of time is neutral. Even an untimed copy still supplies
	// numeric evidence against a group whose claimed cached tokens exceed it.
	r = call("R1", 10, 0, c5, 10, 0, c5)
	cases = append(cases, scenario{name: "untimed counter contradiction still vetoes", requests: 1, overlap: true,
		bodies: [3]string{copyFor(20, 0, "", "", r), omit(copyFor(20, 0, c0, c4, r), "10:10:00"), copyFor(20, 0, c0, "", r)}})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Reassign bodies to actual digest-sorted source names for all 3!
			// permutations. Only physical references and their stamps change.
			var first []ledger.Member
			var firstGaps ledger.Gap
			for _, order := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
				v := orderedLedgerCopies(t, tc.bodies[order[0]], tc.bodies[order[1]], tc.bodies[order[2]])
				if (v.Gaps&ledger.PossibleOverlap != 0) != tc.overlap || v.UnresolvedOverlap != tc.overlap {
					t.Fatalf("%v: gaps=%x unresolved=%t", order, v.Gaps, v.UnresolvedOverlap)
				}
				if tc.untimedOnly && (!v.Partial || v.Gaps&ledger.UnallocatedTokens == 0) {
					t.Fatalf("%v: missing window reported complete: gaps=%x", order, v.Gaps)
				}
				calls, intervals := 0, 0
				members := slices.Clone(v.Members)
				for j, m := range members {
					switch m.Key.Kind {
					case ledger.Response:
						calls++
						if tc.nativeProofOnly != "" {
							if m.Key.ID != "R1" || m.Tokens.Input != 10 {
								t.Fatalf("%v: lost confirmed native response", order)
							}
							assertNativeProofs(t, tc.bodies, tc.nativeProofOnly, m.Proof)
						}
						if len(m.Proof) != 3 {
							t.Fatalf("%v: lost confirmed aliases for %s", order, m.Key.ID)
						}
					case ledger.TokenInterval:
						intervals++
						if !tc.lowerAt.IsZero() && (!m.Interval.LowerAt.Equal(tc.lowerAt) || !m.Interval.UpperAt.Equal(tc.upperAt)) {
							t.Fatalf("%v: got %v..%v, want %v..%v", order, m.Interval.LowerAt, m.Interval.UpperAt, tc.lowerAt, tc.upperAt)
						}
						if m.Tokens != tc.left || m.Interval.Lower.Input != tc.lower || m.Interval.Upper.Input != tc.upper || len(m.Proof) != tc.intervalRefs {
							t.Fatalf("%v: unexpected remainder %v over %d..%d", order, m.Tokens, m.Interval.Lower.Input, m.Interval.Upper.Input)
						}
					default:
						t.Fatalf("%v: unexpected member kind %d", order, m.Key.Kind)
					}
					members[j].Proof = nil
				}
				wantIntervals := 0
				if tc.left.Input+tc.left.Output != 0 {
					wantIntervals = 1
				}
				if calls != tc.requests || intervals != wantIntervals {
					t.Fatalf("%v: calls=%d intervals=%d, want %d/%d", order, calls, intervals, tc.requests, wantIntervals)
				}
				if first == nil {
					first, firstGaps = members, v.Gaps
				} else if !reflect.DeepEqual(first, members) || v.Gaps != firstGaps {
					t.Fatalf("%v: different selected members or gaps", order)
				}
			}
		})
	}
}

// Match each reference to the native line in its authenticated file bytes;
// a legacy span or a duplicate source cannot stand in for a native alias.
func assertNativeProofs(t *testing.T, bodies [3]string, line string, refs []ledger.Reference) {
	t.Helper()
	want := make(map[[32]byte]int64, len(bodies))
	for _, body := range bodies {
		offset := strings.Index(body, line)
		if offset < 0 {
			t.Fatal("fixture lacks native line")
		}
		want[sha256.Sum256([]byte(body))] = int64(offset)
	}
	if len(want) != len(bodies) {
		t.Fatal("expected three distinct captures")
	}
	for _, ref := range refs {
		digest := ref.Source.Prefix.Digest
		offset, ok := want[digest]
		if !ok || ref.Span.Start != offset || ref.Span.End != offset+int64(len(line)) {
			t.Fatal("unexpected proof for native response", ref.Span)
		}
		delete(want, digest)
	}
	if len(want) != 0 {
		t.Fatalf("missing %d native proofs", len(want))
	}
}
