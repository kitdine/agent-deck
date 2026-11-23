package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func TestCJSIProvenCodexActivity(t *testing.T) {
	const parent = "00000000-0000-4000-8000-000000000001"
	const child = "00000000-0000-4000-8000-000000000002"

	head := `{"timestamp":"2026-01-02T09:00:00Z","type":"session_meta","payload":{"id":"` + child + `","session_id":"` + parent + `","parent_thread_id":"` + parent + `"}}`
	contextLine := func(thread, turn string) string {
		return `{"type":"turn_context","payload":{"thread_id":"` + thread + `","turn_id":"` + turn + `","model":"provider/gpt-synthetic"}}`
	}
	tool := func(at, thread, kind, id, extra string) string {
		scope := ""
		if thread != "" {
			scope = `"thread_id":"` + thread + `",`
		}
		return `{"timestamp":"` + at + `","type":"response_item","payload":{` + scope + `"type":"` + kind + `","call_id":"` + id + `"` + extra + `}}`
	}
	request := `{"type":"token_usage_record","payload":{"thread_id":"` + child + `","turn_id":"child-1","response_id":"synthetic","usage":{"input_tokens":1,"output_tokens":0}}}`
	input := strings.Join([]string{
		head,
		`{"timestamp":"2026-01-02T08:00:00Z","type":"session_meta","payload":{"id":"` + parent + `"}}`,
		contextLine(parent, "parent-1"),
		tool("2026-01-02T08:00:00Z", parent, "function_call", "reuse", `,"name":"apply_patch"`),
		contextLine(child, "child-1"),
		tool("2026-01-02T09:00:02Z", "", "function_call", "reuse", `,"name":"exec_command","arguments":"/privacy-path-sentinel command-sentinel"`),
		request,
		tool("2026-01-02T09:00:03Z", "", "function_call_output", "reuse", `,"output":"tool-output-sentinel"`),
		contextLine(child, "child-2"),
		tool("2026-01-02T09:00:01Z", child, "custom_tool_call", "reuse", `,"name":"apply_patch","input":"credential-sentinel"`),
		contextLine(child, "child-3"),
		tool("2026-01-02T09:00:04Z", child, "custom_tool_call_output", "reuse", `,"output":"orphan-sentinel"`),
		tool("2026-01-02T09:00:06Z", child, "function_call", "inverted", `,"name":"/privacy-tool-name-sentinel"`),
		tool("2026-01-02T09:00:05Z", child, "function_call_output", "inverted", ""),
		tool("2026-01-02T09:00:07Z", child, "function_call", "untimed-end", `,"name":"unrecognized-tool-sentinel"`),
		tool("", child, "function_call_output", "untimed-end", ""),
		tool("", child, "function_call", "untimed-start", `,"name":"exec"`),
		`{"type":"response_item","payload":{"role":"user","type":"message","content":"prompt-sentinel"}}`,
		`{"type":"synthetic","text":"partial-sentinel`,
	}, "\n")

	capture := func(t *testing.T, name, contents string) (ingest.CodexPrefixProof, ingest.CodexCapturedReduction) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "rollout-2026-01-02T09-00-00-"+name+".jsonl")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		result, err := ingest.CaptureCodexReduction(t.Context(), file, path, int64(len(contents)), 4096)
		if err != nil {
			t.Fatal(err)
		}
		proof := result.Capture()
		if err := proof.Revalidate(t.Context()); err != nil {
			t.Fatal(err)
		}
		return proof, result
	}

	t.Run("equal extent does not bind another capture", func(t *testing.T) {
		prefix := head + "\n" + contextLine(child, "capture-a") + "\n"
		contentsA := prefix + tool("2026-01-02T09:00:00Z", child, "function_call", "start", `,"name":"exec_command"`) + "\n"
		contentsB := prefix + tool("2026-01-02T09:00:01Z", child, "function_call", "start", `,"name":"exec_command"`) + "\n"
		proofA, reductionA := capture(t, child, contentsA)
		proofB, reductionB := capture(t, child, contentsB)
		if len(contentsA) != len(contentsB) || proofA.Progress() != proofB.Progress() {
			t.Fatal("captures must have equal progress")
		}
		a := CodexDetails(child, proofA, reductionA)
		b := CodexDetails(child, proofB, reductionB)
		if len(a) != 1 || len(b) != 1 || a[0].StartedAt == b[0].StartedAt {
			t.Fatal("captures must have distinct legitimate activity")
		}
		for _, input := range []ingest.CodexCapturedReduction{reductionB, {}} {
			if len(CodexDetails(child, proofA, input)) != 0 {
				t.Error("detail accepted mismatched or unbound reduction")
			}
			page, err := CodexDetailsPage(child, proofA, input, 1, 1, false)
			if err != nil || page.Total != 0 || page.Summary.Total != 0 || len(page.Details) != 0 {
				t.Error("page accepted mismatched or unbound reduction")
			}
		}
	})

	proof, result := capture(t, child, input)
	if !proof.Progress().Partial || result.Source().ParentCandidate != parent ||
		result.Source().Birth.Format("15:04") != "09:00" {
		t.Fatal("fixture lost partial boundary, direct parent, or own birth")
	}
	details := CodexDetails(child, proof, result)
	if len(details) != 4 {
		t.Fatalf("got %d details, want 4", len(details))
	}
	if got := details[0]; got.Tool != "apply_patch" || got.Status != "started" || got.DurationMS != nil || got.CompletedAt != "" {
		t.Fatal("completion crossed a segment and closed a reused call")
	}
	if got := details[1]; got.Tool != "exec_command" || got.Status != "completed" ||
		got.DurationMS == nil || *got.DurationMS != 1000 || got.CompletedAt != "2026-01-02T09:00:03Z" {
		t.Fatal("proved completion did not close the proper child span")
	}
	if got := details[2]; got.Tool != "other" || got.Status != "completed" || got.DurationMS != nil || got.CompletedAt != "2026-01-02T09:00:05Z" {
		t.Fatal("unsafe tool label or inverted timestamps reached duration")
	}
	if got := details[3]; got.Tool != "other" || got.Status != "started" || got.DurationMS != nil || got.CompletedAt != "" {
		t.Fatal("untimed completion changed known status or inferred time")
	}
	for _, detail := range details {
		if detail.SessionID != child || detail.Client != "codex" {
			t.Fatal("result escaped the selected child binding")
		}
	}
	t.Run("qualified model label", func(t *testing.T) {
		if got := details[1].Model; got != "provider/gpt-synthetic" {
			t.Fatalf("Model = %q, want provider/gpt-synthetic", got)
		}
		badInput := strings.ReplaceAll(input, "provider/gpt-synthetic", "synthetic/private/target.txt")
		badProof, badReduction := capture(t, child, badInput)
		for _, detail := range CodexDetails(child, badProof, badReduction) {
			if detail.Model != "" {
				t.Fatal("a path-shaped model reached activity")
			}
		}
	})
	t.Run("bounded page with complete summary", func(t *testing.T) {
		page, err := CodexDetailsPage(child, proof, result, 1, 1, false)
		if err != nil || len(page.Details) != 1 || page.Summary.Total != len(details) {
			t.Fatal("first page lost details or full-capture summary")
		}
		if got := cap(page.Details); got > 1 {
			t.Fatalf("first page retained a %d-row backing array for a one-row window", got)
		}
	})
	t.Run("equal display keys keep source order", func(t *testing.T) {
		contents := strings.Join([]string{
			head, contextLine(child, "ties"),
			tool("2026-01-02T09:00:00Z", child, "function_call", "first", `,"name":"exec_command"`),
			tool("2026-01-02T09:00:00Z", child, "function_call", "second", `,"name":"exec_command"`),
			tool("2026-01-02T09:00:01Z", child, "function_call_output", "second", ""),
			tool("2026-01-02T09:00:02Z", child, "function_call_output", "first", ""),
		}, "\n") + "\n"
		p, reduction := capture(t, child, contents)
		for i, expected := range []int64{2000, 1000} {
			page, err := CodexDetailsPage(child, p, reduction, i+1, 1, false)
			if err != nil || len(page.Details) != 1 || page.Details[0].DurationMS == nil || *page.Details[0].DurationMS != expected {
				t.Fatalf("page %d broke start-span order for tied display keys", i+1)
			}
		}
	})

	for pageNumber := 1; pageNumber <= 3; pageNumber++ {
		page, err := CodexDetailsPage(child, proof, result, pageNumber, 2, false)
		if err != nil {
			t.Fatal(err)
		}
		start := (pageNumber - 1) * 2
		want := details[start:min(start+2, len(details))]
		if !reflect.DeepEqual(want, page.Details) || page.Total != 4 || page.Shown != len(want) ||
			page.HasMore != (pageNumber == 1) || page.Summary.Completed != 2 || page.Summary.Incomplete != 2 ||
			page.Summary.TotalDurationMS != 1000 || page.Summary.AverageDurationMS == nil ||
			*page.Summary.AverageDurationMS != 1000 || page.Summary.ByTool["other"] != 2 {
			t.Fatalf("page %d did not preserve details and full summary", pageNumber)
		}
	}
	all, err := CodexDetailsPage(child, proof, result, 3, 2, true)
	if err != nil || all.Page != 1 || all.Limit != 4 || !reflect.DeepEqual(details, all.Details) {
		t.Fatal("all activity did not retain detail order and page shape")
	}
	if _, err := CodexDetailsPage(child, proof, result, 1001, 1, false); err == nil {
		t.Fatal("proven activity bypassed the legacy page window")
	}

	for record := range result.Records() {
		if record.ToolStart != nil {
			*record.ToolStart = ingest.CodexSpan{}
		}
	}
	if !reflect.DeepEqual(details, CodexDetails(child, proof, result)) {
		t.Fatal("exported records could alter the bound completion proof")
	}

	conflictProof, conflict := capture(t, parent, input)
	for _, rejected := range [][]Detail{
		CodexDetails(parent, proof, result),
		CodexDetails(child, conflictProof, conflict),
		CodexDetails(child, ingest.CodexPrefixProof{}, result),
	} {
		if len(rejected) != 0 {
			t.Fatal("foreign, conflicted or unproved source emitted details")
		}
	}
	unknownProof, unknown := capture(t, child, strings.Replace(input, head, `{"type":"compacted"}`, 1))
	if len(CodexDetails(child, unknownProof, unknown)) != 0 {
		t.Fatal("unknown source emitted details")
	}

	for _, output := range []any{result.Source(), slices.Collect(result.Records()), details, all} {
		encoded, err := json.Marshal(output)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "sentinel") {
			t.Fatal("proven activity retained prohibited fixture material")
		}
	}
}

func TestCJSIProvenActivityPreservesLegacyAllowlistBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.jsonl")
	raw := `{"type":"session_meta","payload":{"session_id":"legacy"}}
{"type":"response_item","timestamp":"2026-01-02T09:00:00Z","payload":{"type":"function_call","name":"legacy_custom_tool","call_id":"id","arguments":"private-sentinel"}}
{"type":"response_item","timestamp":"2026-01-02T09:00:01Z","payload":{"type":"function_call_output","call_id":"id","output":"private-sentinel"}}
`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	full, err := ReadDetails(path, "codex", "legacy")
	if err != nil || len(full) != 1 || full[0].Tool != "legacy_custom_tool" || full[0].Status != "completed" {
		t.Fatal("legacy activity shape changed")
	}
	page, err := ReadDetailsPage(path, "codex", "legacy", 1, 1, false)
	if err != nil || !reflect.DeepEqual(full, page.Details) || page.Total != 1 || page.Summary.TotalDurationMS != 1000 {
		t.Fatal("legacy page or summary behavior changed")
	}
}
