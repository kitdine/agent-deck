package activity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

const cjsiSelectedThread = "00000000-0000-4000-8000-000000000001"

func cjsiSelectedActivityFixture(t *testing.T, input string) (ingest.CodexSelectedSource, ingest.CodexSelectedLimits) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout-2026-01-02T03-04-05-"+cjsiSelectedThread+".jsonl")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	p, err := ingest.CaptureCodexPrefix(t.Context(), file, path, int64(len(input)), 512, false, func(ingest.CodexRecord, *ingest.CodexVisible) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return ingest.CodexSelectedSource{Path: path, ThreadID: cjsiSelectedThread, Epoch: 3, Evidence: p.Evidence()},
		ingest.CodexSelectedLimits{MaxBytes: int64(len(input)), MaxLineBytes: 512, MaxRecords: 12}
}

func TestCJSISelectedCodexActivityPage(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiSelectedThread + `"}}` + "\n"
	lines := []string{
		`{"timestamp":"2026-01-02T10:00:01Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"later","arguments":"PRIVATE_SENTINEL"}}`,
		`{"timestamp":"2026-01-02T10:00:00Z","type":"response_item","payload":{"type":"function_call","name":"apply_patch","call_id":"first"}}`,
		`{"timestamp":"2026-01-02T10:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"first"}}`,
	}
	// The selected extent includes a partial line; an append outside its
	// digest is not allowed to complete the call in this view.
	outside := `{"timestamp":"2026-01-02T10:00:03Z","type":"response_item","payload":{"type":"function_call_output","call_id":"later"}}` + "\n"
	input := head + strings.Join(lines, "\n") + "\n" + outside[:15]
	selected, limits := cjsiSelectedActivityFixture(t, input)
	file, err := os.OpenFile(selected.Path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(outside[15:])
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append failed: %v, %v", err, closeErr)
	}
	current := func(context.Context) (ingest.CodexSelectedSource, error) { return selected, nil }
	details, err := ReadSelectedCodexDetails(t.Context(), selected, current, limits)
	if err != nil || len(details) != 2 || details[0].Tool != "apply_patch" || details[0].Status != "completed" ||
		details[1].Tool != "exec_command" || details[1].Status != "started" {
		t.Fatalf("unchecked bytes changed selected details: %+v, %v", details, err)
	}
	for i := 1; i <= 2; i++ {
		page, err := ReadSelectedCodexDetailsPage(t.Context(), selected, current, limits, i, 1, false)
		if err != nil || len(page.Details) != 1 || !reflect.DeepEqual(page.Details[0], details[i-1]) ||
			page.Total != 2 || page.Summary.Total != 2 || page.Summary.Completed != 1 || page.Summary.Incomplete != 1 ||
			page.Summary.TotalDurationMS != 2000 || page.HasMore != (i == 1) {
			t.Fatalf("page %d differs from the selected details/summary: %+v, %v", i, page, err)
		}
	}
}

func TestCJSISelectedCodexActivityRejectsStale(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiSelectedThread + `"}}` + "\n"
	input := head + `{"timestamp":"2026-01-02T10:00:00Z","type":"response_item","payload":{"type":"function_call","name":"apply_patch","call_id":"first"}}` + "\n"
	for _, tc := range []string{"rename", "replacement", "same inode alias", "edited bytes", "ambiguous thread", "stale epoch", "canceled",
		"changed during final check", "same size edit during final check", "line policy"} {
		t.Run(tc, func(t *testing.T) {
			selected, limits := cjsiSelectedActivityFixture(t, input)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			actual := selected
			inputPath := selected.Path
			want := ingest.ErrSourceChanged
			checks := 0
			current := func(context.Context) (ingest.CodexSelectedSource, error) {
				checks++
				if tc == "changed during final check" && checks == 2 {
					if err := os.Rename(inputPath, inputPath+".old"); err != nil {
						t.Fatal(err)
					}
				}
				if tc == "same size edit during final check" && checks == 2 {
					file, err := os.OpenFile(inputPath, os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					defer file.Close()
					if _, err := file.WriteAt([]byte("X"), int64(strings.Index(input, "apply_patch"))); err != nil {
						t.Fatal(err)
					}
					// Make the final stat-wall change independent of timestamp resolution.
					at := time.Unix(1700000000, 0)
					if err := os.Chtimes(inputPath, at, at); err != nil {
						t.Fatal(err)
					}
				}
				return actual, nil
			}
			switch tc {
			case "rename", "replacement":
				if err := os.Rename(inputPath, inputPath+".old"); err != nil {
					t.Fatal(err)
				}
				if tc == "replacement" {
					if err := os.WriteFile(inputPath, []byte(input), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "edited bytes":
				if err := os.WriteFile(inputPath, []byte(strings.Replace(input, "apply_patch", "unsafe_tool", 1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "same inode alias":
				selected.Path = inputPath + "-alias.jsonl"
				if err := os.Link(inputPath, selected.Path); err != nil {
					t.Fatal(err)
				}
			case "ambiguous thread":
				selected.ThreadID = "00000000-0000-4000-8000-000000000002"
				actual = selected
			case "stale epoch":
				actual.Epoch++
			case "line policy":
				limits.MaxLineBytes--
			case "canceled":
				cancel()
				want = context.Canceled
			}
			page, err := ReadSelectedCodexDetailsPage(ctx, selected, current, limits, 1, 1, false)
			if !errors.Is(err, want) || page.Details != nil || page.Total != 0 || page.Summary.Total != 0 {
				t.Fatalf("unverified activity escaped: %+v, %v", page, err)
			}
		})
	}
}
