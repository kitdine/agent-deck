package ingest

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	cjsiP = "00000000-0000-4000-8000-000000000001"
	cjsiC = "00000000-0000-4000-8000-000000000002"
	cjsiS = "00000000-0000-4000-8000-000000000003"
	cjsiG = "00000000-0000-4000-8000-000000000004"
)

func cjsiCanonical(thread string) string {
	return "rollout-2026-01-02T03-04-05-" + thread + ".jsonl"
}

func cjsiRead(t *testing.T, input string, start int64, visible bool) ([]CodexRecord, []CodexVisible, CodexReadProgress) {
	t.Helper()
	var records []CodexRecord
	var documents []CodexVisible
	bounds := CodexCaptureBounds{Start: start, End: start + int64(len(input)), MaxLineBytes: 4096}
	progress, err := ReadCodexJSONL(t.Context(), strings.NewReader(input), bounds, visible, func(r CodexRecord, doc *CodexVisible) error {
		records = append(records, r)
		if doc != nil {
			documents = append(documents, *doc)
		}
		return nil
	})
	if err != nil {
		t.Fatal("unexpected lexer error:", err)
	}
	return records, documents, progress
}

func cjsiReduce(t *testing.T, path string, records []CodexRecord, p CodexReadProgress) CodexReduction {
	t.Helper()
	result, err := ReduceCodexProof(path, records, p)
	if err != nil {
		t.Fatal("unexpected reducer error:", err)
	}
	return result
}

func cjsiSource(t *testing.T, path, input string) CodexReduction {
	t.Helper()
	records, _, progress := cjsiRead(t, input, 0, false)
	return cjsiReduce(t, path, records, progress)
}

func cjsiLocalDocuments(result CodexReduction, docs []CodexVisible) []string {
	local := make(map[CodexSpan]bool)
	for _, record := range result.Records {
		local[record.Record.Span] = record.Local()
	}
	var texts []string
	for _, doc := range docs {
		if local[doc.Span] {
			texts = append(texts, doc.Text)
		}
	}
	return texts
}

func cjsiNamedTurnBoundaryProof(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `","session_id":"` + cjsiP + `"}}` + "\n"
	foreign := `{"type":"turn_context","payload":{"thread_id":"` + cjsiP + `","turn_id":"reused","model":"parent-model"}}` + "\n"
	local := `{"type":"turn_context","payload":{"thread_id":"` + cjsiC + `","turn_id":"reused","model":"child-model"}}` + "\n"
	request := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"reused","response_id":"id","usage":{"input_tokens":1,"output_tokens":0}}}` + "\n"
	reply := `{"type":"response_item","payload":{"type":"message","turn_id":"reused","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"synthetic-final"}]}}` + "\n"
	path := cjsiCanonical(cjsiC)
	complete := `{"type":"event_msg","payload":{"type":"task_complete","thread_id":"` + cjsiC + `","turn_id":"reused"}}` + "\n"
	for _, tc := range []struct{ name, history string }{{"unique turn", ""}, {"reused turn", foreign}} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := head + tc.history + local + request + reply
			before := cjsiSource(t, path, prefix)
			replyIndex := len(before.Records) - 1
			call := before.Records[replyIndex-1]
			if !call.Local() || call.Model != "child-model" {
				t.Fatal("the explicit request and matching local context must remain attributable")
			}
			if before.Records[replyIndex].Local() {
				t.Fatal("a bare turn ID attributed a threadless reply beyond the request anchor")
			}
			after := cjsiSource(t, path, prefix+complete)
			if after.Records[replyIndex].Execution != CodexRightAnchored || after.Records[replyIndex].Model != "child-model" {
				t.Fatal("an explicit local completion should establish the buffered reply's right anchor")
			}
		})
	}
}

func cjsiPendingReplayProof(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `","session_id":"` + cjsiP + `"}}` + "\n"
	closed := head + "{bad\n" + `{"type":"compacted","payload":{}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"inherit","model":"parent-model"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"inherit"}}` + "\n"
	path := cjsiCanonical(cjsiC)
	if got := cjsiSource(t, path, closed).Checkpoint; got.ReplayFrom != got.CompleteEnd {
		t.Fatalf("closed segment or malformed line held replay at %d, complete end %d", got.ReplayFrom, got.CompleteEnd)
	}
	tail := `{"type":"turn_context","payload":{"thread_id":"` + cjsiC + `","turn_id":"local"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","turn_id":"local","call_id":"pending"}}` + "\n" +
		`{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"local","response_id":"local-response"}}` + "\n"
	if got := cjsiSource(t, path, closed+tail).Checkpoint; got.ReplayFrom != int64(len(closed)) {
		t.Fatalf("open tool start should keep only its segment pending: %+v", got)
	}
	output := `{"type":"response_item","payload":{"type":"function_call_output","turn_id":"local","call_id":"pending"}}` + "\n"
	if got := cjsiSource(t, path, closed+tail+output).Checkpoint; got.ReplayFrom != got.CompleteEnd {
		t.Fatal("paired tool continued to hold replay")
	}
}

func cjsiEventIDProof(t *testing.T) {
	raw := `{"type":"turn_context","payload":{"turn_id":"synthetic/private/target.txt","model":"provider/test-model"}}
{"type":"token_usage_record","payload":{"response_id":"synthetic/private/target.txt","root_turn_id":"synthetic/private/target.txt"}}
{"type":"response_item","payload":{"type":"function_call","call_id":"synthetic/private/target.txt"}}
`
	records, _, _ := cjsiRead(t, raw, 0, false)
	if records[0].Model != "provider/test-model" || !records[0].BadIdentity || records[1].ResponseID != "" ||
		records[1].RootTurnID != "" || records[2].CallID != "" {
		t.Fatal("event ID checking retained a path or removed a valid provider-qualified model")
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "synthetic/private/target.txt") {
		t.Fatal("a path-shaped event ID entered the proof facts")
	}
}

func cjsiGoldenProof(t *testing.T, golden string) {
	t.Helper()
	path := cjsiCanonical(cjsiC)
	closeStart := strings.LastIndex(golden, "\n{") + 1
	cut := len(golden) - 5
	records, documents, progress := cjsiRead(t, golden[:cut], 0, true)
	if progress != (CodexReadProgress{ReadEnd: int64(cut), CompleteEnd: int64(closeStart), Partial: true}) {
		t.Fatalf("unexpected half-line progress: %+v", progress)
	}
	before := cjsiReduce(t, path, records, progress)
	if before.Source.ThreadID != cjsiC || before.Source.ParentCandidate != cjsiP ||
		before.Source.RootTurnID != "root-turn" || !before.Source.Mixed ||
		!before.Source.Birth.Equal(time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected source proof: %+v", before.Source)
	}
	if got := cjsiLocalDocuments(before, documents); !slices.Equal(got, []string{"local-user-synthetic"}) {
		t.Fatal("wrong documents before complete right anchor")
	}
	pendingReply := before.Records[9]
	toolEnd := before.Records[10]
	if pendingReply.Local() || !toolEnd.Local() || toolEnd.Execution != CodexPairedTool ||
		toolEnd.ToolStart == nil || *toolEnd.ToolStart != records[7].Span {
		t.Fatal("reply or tool completion used the wrong right boundary")
	}
	if before.Records[8].Model != "child-model" || before.Records[8].Execution != CodexExplicit ||
		before.Records[8].TurnIndex != 1 || before.Records[4].Local() {
		t.Fatal("new request did not use the bound child model independently of inherited usage")
	}
	rest, laterDocs, end := cjsiRead(t, golden[progress.CompleteEnd:], progress.CompleteEnd, true)
	all := append(records, rest...)
	after := cjsiReduce(t, path, all, end)
	if after.Checkpoint.Partial || after.Checkpoint.CompleteEnd != int64(len(golden)) ||
		before.Checkpoint.ReplayFrom >= pendingReply.Record.Span.End {
		t.Fatal("checkpoint lost the pending range or advanced over a suffix")
	}
	if got := cjsiLocalDocuments(after, append(documents, laterDocs...)); !slices.Equal(got, []string{"local-user-synthetic", "local-final-synthetic"}) {
		t.Fatal("replay did not publish precisely the right-anchored documents in source order")
	}
	cold, noText, coldEnd := cjsiRead(t, golden, 0, false)
	if len(noText) != 0 || !reflect.DeepEqual(after, cjsiReduce(t, path, cold, coldEnd)) {
		t.Fatal("reopened full capture differs from append replay, or proof mode emitted text")
	}
	if _, err := ReduceCodexProof(path, rest, end); err == nil {
		t.Fatal("a suffix without the verified prefix established a source")
	}
	cancel := errors.New("stop")
	stopped, err := ReadCodexJSONL(t.Context(), strings.NewReader(golden), CodexCaptureBounds{End: int64(len(golden)), MaxLineBytes: 4096}, false, func(CodexRecord, *CodexVisible) error {
		return cancel
	})
	if !errors.Is(err, cancel) || stopped.CompleteEnd != 0 {
		t.Fatal("a rejected callback advanced the complete checkpoint")
	}
	for _, rec := range after.Records {
		if rec.Record.Kind == CodexRequestUsage {
			total, valid := rec.Record.Usage.Total()
			if !valid || total != 35 || rec.Record.Usage.CacheWrite.State != CodexValid || rec.Record.ThreadTotal.Input.Value != 90 {
				t.Fatal("expected primary counts plus separately preserved components and snapshots")
			}
		}
	}
	proof, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"inherited-synthetic", "local-user-synthetic", "local-final-synthetic", "private-argument-synthetic", "path-synthetic", "private-tool-output-synthetic"} {
		if strings.Contains(string(proof), sentinel) {
			t.Fatal("transient document or prohibited content entered proof")
		}
	}
}

func cjsiIdentityProof(t *testing.T, golden string) {
	t.Helper()
	grandchild := `{"type":"session_meta","payload":{"id":"` + cjsiG + `","session_id":"` + cjsiP + `","source":{"subagent":{"thread_spawn":{"parent_thread_id":"` + cjsiC + `"}}}}}` + "\n"
	child := cjsiSource(t, "renamed.jsonl", golden)
	g := cjsiSource(t, cjsiCanonical(cjsiG), grandchild)
	parents := ResolveCodexParents([]CodexSourceProof{g.Source, child.Source})
	if parents[cjsiG] != cjsiC || parents[cjsiC] != cjsiP || g.Source.RootSessionID != cjsiP {
		t.Fatal("direct parent chain flattened or overwritten by root")
	}
	forged := cjsiSource(t, cjsiCanonical(cjsiP), golden)
	if forged.Source.State != CodexSourceConflict || slices.ContainsFunc(forged.Records, CodexResolvedRecord.Local) {
		t.Fatal("conflicting filename gave local facts")
	}
	reverted := strings.TrimSuffix(cjsiCanonical(cjsiC), ".jsonl") + "_" + cjsiG + ".jsonl"
	if got := cjsiSource(t, "archived_sessions/"+reverted, golden).Source; got.ThreadID != cjsiC || got.FilenameID != cjsiC {
		t.Fatal("reverted filename selected rollout UUID instead of thread UUID")
	}
	for _, prefix := range []string{"{broken\n", "{\"type\":\"session_meta\",\"payload\":{\"id\":\"invalid\"}}\n", "{\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"inherited\"}}\n"} {
		result := cjsiSource(t, cjsiCanonical(cjsiC), prefix+golden)
		if result.Source.State != CodexSourceUnknown || slices.ContainsFunc(result.Records, CodexResolvedRecord.Local) {
			t.Fatal("bad opening evidence allowed a later meta to impersonate the head")
		}
	}
	halfHead := cjsiSource(t, cjsiCanonical(cjsiC), golden[:strings.IndexByte(golden, '\n')])
	if halfHead.Source.State != CodexSourceUnknown || !halfHead.Checkpoint.Partial ||
		halfHead.Checkpoint.CompleteEnd != 0 || len(halfHead.Records) != 0 {
		t.Fatal("an unterminated opening line created identity or advanced the checkpoint")
	}
	if got := cjsiSource(t, cjsiCanonical(cjsiC)+".zst", golden).Source.State; got != CodexSourceUnknown {
		t.Fatal("compressed path admitted a source")
	}
	other := `{"type":"session_meta","payload":{"id":"` + cjsiS + `","forked_from_id":"` + cjsiP + `","source":{"subagent":{"other":"private-guardian-synthetic"}}}}` + "\n"
	if got := cjsiSource(t, cjsiCanonical(cjsiS), other).Source; !got.Mixed || got.ForkedFromID != cjsiP || got.ParentCandidate != "" {
		t.Fatal("guardian/derived source invented a direct parent")
	}
	if got := cjsiSource(t, cjsiCanonical(cjsiG), strings.Replace(grandchild, `"session_id":`, `"parent_thread_id":"`+cjsiP+`","session_id":`, 1)).Source; got.ParentCandidate != "" {
		t.Fatal("conflicting parent fields supplied an edge")
	}
	if got := cjsiSource(t, cjsiCanonical(cjsiG), strings.Replace(grandchild, `"parent_thread_id":"`+cjsiC, `"parent_thread_id":"`+cjsiG, 1)).Source; got.ParentCandidate != "" {
		t.Fatal("self-parent supplied an edge")
	}
	parentBack := cjsiSource(t, cjsiCanonical(cjsiP), `{"type":"session_meta","payload":{"id":"`+cjsiP+`","parent_thread_id":"`+cjsiC+`"}}`+"\n")
	if edges := ResolveCodexParents([]CodexSourceProof{g.Source, child.Source, parentBack.Source}); len(edges) != 1 || edges[cjsiG] != cjsiC {
		t.Fatal("cycle removed unrelated edges or left a cycle edge")
	}
	legacy := `{"type":"turn_context","sessionId":"old-session","payload":{"turn_id":"old-turn","model":"old-model"}}
{"type":"event_msg","session_id":"old-session","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2,"output_tokens":1}}}}
`
	if got := cjsiSource(t, "old.jsonl", legacy); got.Source.ThreadID != "old-session" || !got.Records[1].Local() || got.Records[1].Model != "old-model" {
		t.Fatal("ordinary single legacy identity did not retain turn-based attribution")
	}
	filenameOnly := `{"type":"session_meta","payload":{}}
{"type":"turn_context","payload":{"turn_id":"t"}}
`
	if cjsiSource(t, cjsiCanonical(cjsiC), filenameOnly).Source.State != CodexSourceUnknown {
		t.Fatal("filename alone supplied a source proof")
	}
	withCorroboration := filenameOnly + `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"t"}}` + "\n"
	withCorroboration += `{"type":"response_item","payload":{"type":"message","role":"user","content":"still-unanchored"}}` + "\n"
	corroborated := cjsiSource(t, cjsiCanonical(cjsiC), withCorroboration)
	if corroborated.Source.ThreadID != cjsiC || !corroborated.Source.Mixed || corroborated.Records[3].Local() {
		t.Fatal("corroboration failed or bypassed the mixed-source boundary")
	}
	plain := cjsiSource(t, cjsiCanonical(cjsiC), `{"type":"session_meta","payload":{"id":"`+cjsiC+`","forked_from_id":null}}`+"\n")
	if plain.Source.Mixed {
		t.Fatal("nullable fork metadata invented a derived source")
	}
}

func cjsiAmbiguityProof(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `","session_id":"` + cjsiP + `"}}` + "\n"
	context := `{"type":"turn_context","payload":{"turn_id":"same","model":"model-a"}}` + "\n"
	start := `{"type":"response_item","payload":{"type":"function_call","call_id":"reuse"}}` + "\n"
	output := `{"type":"response_item","payload":{"type":"function_call_output","call_id":"reuse"}}` + "\n"
	req := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"same","response_id":"id","usage":{"input_tokens":1,"output_tokens":0}}}` + "\n"
	prompt := `{"type":"response_item","payload":{"type":"message","role":"user","content":"synthetic-visible"}}` + "\n"
	path := cjsiCanonical(cjsiC)
	prefix := head + context + start + prompt + req + output
	before := cjsiSource(t, path, prefix)
	if !before.Records[3].Local() || before.Records[5].ToolStart == nil || before.Records[4].Model != "model-a" {
		t.Fatal("proof did not establish an unambiguous local segment")
	}
	after := cjsiSource(t, path, prefix+context+start+output)
	if after.Records[3].Local() || after.Records[5].ToolStart != nil || after.Records[4].Model != "" || !after.Records[4].Local() {
		t.Fatal("late competing bare turn failed to withdraw dependent facts only")
	}
	independent := cjsiSource(t, path, head+context+start+req+`{"type":"event_msg","payload":{"type":"task_started","turn_id":"different"}}`+"\n"+output)
	if independent.Records[len(independent.Records)-1].ToolStart != nil {
		t.Fatal("tool completed across a turn switch")
	}
	dupStart := cjsiSource(t, path, head+context+start+start+req+output)
	if dupStart.Records[len(dupStart.Records)-1].ToolStart != nil {
		t.Fatal("tool completion selected one of two starts")
	}
	competing := cjsiSource(t, path, prefix+`{"type":"event_msg","payload":{"type":"task_started","turn_id":"same"}}`+"\n")
	if competing.Records[4].Model != "" || !competing.Records[4].Local() {
		t.Fatal("an anonymous competing turn opening failed to withdraw the model only")
	}
	explicit := strings.Replace(context, `"turn_id":"same"`, `"thread_id":"`+cjsiC+`","turn_id":"same"`, 1)
	bothThreads := cjsiSource(t, path, head+context+explicit+start+req+output)
	if record := bothThreads.Records[4]; !record.Local() || record.Model != "model-a" || record.Context == nil ||
		*record.Context != bothThreads.Records[2].Record.Span {
		t.Fatal("explicit thread and turn on both ends did not disambiguate context")
	}
	plainHead := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	plain := cjsiSource(t, path, plainHead+start+context+output)
	if plain.Records[3].ToolStart != nil {
		t.Fatal("first explicit turn context completed a tool from before the turn")
	}
	for _, fence := range []string{
		"{invalid\n",
		`{"type":"compacted","payload":{"replacement_history":[{"usage":{"input_tokens":999}}]}}` + "\n",
		`{"type":"event_msg","payload":{"type":"turn_aborted","turn_id":"same"}}` + "\n",
		`{"type":"response_item","payload":{"thread_id":"` + cjsiP + `","type":"reasoning"}}` + "\n",
	} {
		gap := cjsiSource(t, path, head+context+prompt+fence+req)
		if gap.Records[2].Local() || !gap.Records[4].Local() || gap.Records[4].Model != "" {
			t.Fatal("right anchor crossed a parse, turn or foreign-source boundary")
		}
	}
}

func cjsiCountsProof(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	tests := []struct {
		name, vector string
		state        CodexFieldState
		counted      bool
		total        int64
	}{
		{"absent", "", CodexAbsent, false, 0},
		{"null", "null", CodexInvalid, false, 0},
		{"empty", "{}", CodexValid, false, 0},
		{"missing output", `{"input_tokens":0}`, CodexValid, false, 0},
		{"explicit zero", `{"input_tokens":0,"output_tokens":0,"cached_input_tokens":0}`, CodexValid, true, 0},
		{"integral decimal", `{"input_tokens":9007199254740993.0,"output_tokens":1e1,"total_tokens":9007199254741003}`, CodexValid, true, 9007199254741003},
		{"max exact", `{"input_tokens":9223372036854775807,"output_tokens":0}`, CodexValid, true, math.MaxInt64},
		{"sum overflow", `{"input_tokens":9223372036854775807,"output_tokens":1}`, CodexInvalid, false, 0},
		{"fraction", `{"input_tokens":9007199254740993.1,"output_tokens":0}`, CodexInvalid, false, 0},
		{"overflow", `{"input_tokens":9223372036854775808,"output_tokens":0}`, CodexInvalid, false, 0},
		{"exponent overflow", `{"input_tokens":1e2147483647,"output_tokens":0}`, CodexInvalid, false, 0},
		{"negative", `{"input_tokens":-1,"output_tokens":0}`, CodexInvalid, false, 0},
		{"string", `{"input_tokens":"1","output_tokens":0}`, CodexInvalid, false, 0},
		{"inconsistent cached", `{"input_tokens":1,"cached_input_tokens":2,"output_tokens":0}`, CodexInvalid, false, 0},
		{"inconsistent reasoning", `{"input_tokens":2,"output_tokens":1,"reasoning_output_tokens":2}`, CodexInvalid, false, 0},
		{"bad optional", `{"input_tokens":2,"output_tokens":1,"cache_write_input_tokens":null}`, CodexInvalid, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newFields, oldFields := "", ""
			if tc.vector != "" {
				newFields = `,"usage":` + tc.vector
				oldFields = `"last_token_usage":` + tc.vector + `,"total_token_usage":` + tc.vector
			}
			lines := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","response_id":"response"` + newFields + `}}` + "\n" +
				`{"type":"event_msg","payload":{"type":"token_count","info":{` + oldFields + `}}}` + "\n"
			result := cjsiSource(t, cjsiCanonical(cjsiC), head+lines)
			if result.Records[1].Record.Kind != CodexRequestUsage || result.Records[2].Record.Kind != CodexLegacyUsage {
				t.Fatal("native usage envelopes were not kept distinct")
			}
			for _, vec := range []CodexCounts{result.Records[1].Record.Usage, result.Records[2].Record.LegacyLast, result.Records[2].Record.LegacyTotal} {
				total, ok := vec.Total()
				if vec.State != tc.state || ok != tc.counted || total != tc.total {
					t.Fatalf("unexpected count result: state=%v total=%d counted=%v", vec.State, total, ok)
				}
				if tc.name == "explicit zero" && (vec.Cached.State != CodexValid || vec.CacheWrite.State != CodexAbsent || vec.Output.State != CodexValid) {
					t.Fatal("absent and explicit zero were conflated")
				}
			}
		})
	}
	nonnative := `{"type":"event_msg","payload":{"type":"token_usage_record","usage":{"input_tokens":9,"output_tokens":2}}}
{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{}}}}
{"type":"compacted","payload":{"usage":{"input_tokens":9,"output_tokens":2}}}
`
	result := cjsiSource(t, cjsiCanonical(cjsiC), head+nonnative)
	if result.Records[1].Record.Kind != CodexIgnored || result.Records[2].Record.LegacyLast.State != CodexAbsent ||
		result.Records[3].Record.Kind != CodexBoundary || result.Records[3].Record.Usage.State != CodexAbsent {
		t.Fatal("non-native nested accounting or empty/rate-only notification was given counts")
	}
}

func cjsiWhitelistProof(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `","cwd":"/private/synthetic","environment":"private-env-synthetic"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":"allowed-user-synthetic"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"allowed-old-final-synthetic"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"private-commentary-synthetic"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"partial_answer","content":[{"type":"output_text","text":"private-partial-synthetic"}]}}
{"type":"response_item","payload":{"type":"message","role":"system","content":[{"type":"input_text","text":"private-system-synthetic"}]}}
{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"private-developer-synthetic"}]}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"private-mixed-synthetic"},{"type":"input_image","image_url":"private-image-synthetic"}]}}
{"type":"response_item","payload":{"type":"reasoning","summary":[{"text":"private-reasoning-synthetic"}]}}
{"type":"response_item","payload":{"type":"custom_tool_call","call_id":"tool","input":"private-tool-synthetic"}}
{"type":"event_msg","payload":{"type":"user_message","message":"private-event-text-synthetic"}}
`
	records, documents, progress := cjsiRead(t, input, 0, true)
	result := cjsiReduce(t, cjsiCanonical(cjsiC), records, progress)
	got := cjsiLocalDocuments(result, documents)
	if len(documents) != 2 || !slices.Equal(got, []string{"allowed-user-synthetic", "allowed-old-final-synthetic"}) {
		t.Fatal("visible output included a prohibited shape or rejected a valid legacy final")
	}
	if !records[len(records)-1].UserMessage || records[len(records)-1].VisibleKind != CodexNoDocument {
		t.Fatal("event user boundary was confused with an index document")
	}
}
