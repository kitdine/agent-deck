package activity

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func TestCJSINativeSourceProof(t *testing.T) {
	const parent = "00000000-0000-4000-8000-000000000099"
	input := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"` + cjsiSelectedThread + `","parent_thread_id":"` + parent + `"}}`,
		`{"type":"turn_context","payload":{"thread_id":"` + parent + `","turn_id":"parent","model":"wrong"}}`,
		`{"timestamp":"2026-01-01T00:00:01Z","type":"response_item","payload":{"type":"function_call","call_id":"shared","name":"apply_patch"}}`,
		`{"type":"turn_context","payload":{"turn_id":"child","model":"child-model"}}`,
		`{"timestamp":"2026-01-01T00:00:02Z","type":"response_item","payload":{"type":"function_call","call_id":"shared","name":"exec_command"}}`,
		`{"type":"token_usage_record","payload":{"thread_id":"` + cjsiSelectedThread + `","turn_id":"child","response_id":"local","usage":{"input_tokens":1,"output_tokens":0}}}`,
		`{"timestamp":"2026-01-01T00:00:03Z","type":"response_item","payload":{"type":"function_call_output","call_id":"shared"}}`,
	}, "\n") + "\n"
	selected, limits := cjsiSelectedActivityFixture(t, input)
	current := func(context.Context) (ingest.CodexSelectedSource, error) { return selected, nil }
	details, err := ReadSelectedCodexDetails(t.Context(), selected, current, limits)
	if err != nil || len(details) != 1 {
		t.Fatalf("details=%+v err=%v", details, err)
	}
	call := details[0]
	if call.SessionID != cjsiSelectedThread || call.Model != "child-model" || call.Tool != "exec_command" ||
		call.Status != "completed" || call.DurationMS == nil || *call.DurationMS != 1000 {
		t.Fatalf("wrong local pair: %+v", call)
	}
	page, err := ReadSelectedCodexDetailsPage(t.Context(), selected, current, limits, 1, 1, false)
	if err != nil || !reflect.DeepEqual(page.Details, details) || page.Total != 1 || page.Summary.Completed != 1 || page.Summary.TotalDurationMS != 1000 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
