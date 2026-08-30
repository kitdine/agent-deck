package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkSignalProjectionUsesKeyedBoundedFamilies(t *testing.T) {
	var envelope struct {
		Data Snapshot `json:"data"`
	}
	if err := json.Unmarshal([]byte(buildCompleteFixture(t)), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.WireVersion != 1 {
		t.Fatalf("wire_version = %d, want 1", envelope.Data.WireVersion)
	}
	wantKeys := []string{
		"today/all", "today/codex", "today/claude",
		"7d/all", "7d/codex", "7d/claude",
		"30d/all", "30d/codex", "30d/claude",
	}
	signals := envelope.Data.Sessions.WorkSignals
	if !signals.Activity.Available || !signals.Workflow.Available || !signals.Tooling.Available {
		t.Fatalf("work signal availability = %#v", signals)
	}
	activityKeys := make([]string, 0, len(signals.Activity.Items))
	for _, item := range signals.Activity.Items {
		activityKeys = append(activityKeys, item.Period+"/"+item.Client)
		if len(item.Kinds) != 4 {
			t.Fatalf("activity %s/%s kinds = %d, want 4", item.Period, item.Client, len(item.Kinds))
		}
		for _, kind := range item.Kinds {
			if len(kind.Sub) > 4 {
				t.Fatalf("activity %s/%s/%s sub = %d, want <= 4", item.Period, item.Client, kind.Kind, len(kind.Sub))
			}
		}
	}
	workflowKeys := make([]string, 0, len(signals.Workflow.Items))
	for _, item := range signals.Workflow.Items {
		workflowKeys = append(workflowKeys, item.Period+"/"+item.Client)
	}
	toolingKeys := make([]string, 0, len(signals.Tooling.Items))
	for _, item := range signals.Tooling.Items {
		toolingKeys = append(toolingKeys, item.Period+"/"+item.Client)
		if item.Groups != len(item.Rows) || len(item.Rows) > 5 {
			t.Fatalf("tooling %s/%s groups=%d rows=%d", item.Period, item.Client, item.Groups, len(item.Rows))
		}
	}
	for name, got := range map[string][]string{"activity": activityKeys, "workflow": workflowKeys, "tooling": toolingKeys} {
		if !reflect.DeepEqual(got, wantKeys) {
			t.Fatalf("%s keys = %v, want %v", name, got, wantKeys)
		}
	}
}

func TestWorkSignalProjectionPreservesUnavailableAndLegacyStates(t *testing.T) {
	var partial struct {
		Data Snapshot `json:"data"`
	}
	if err := json.Unmarshal([]byte(buildPartialFixture(t)), &partial); err != nil {
		t.Fatal(err)
	}
	signals := partial.Data.Sessions.WorkSignals
	if signals.Activity.Available || signals.Workflow.Available || signals.Tooling.Available ||
		signals.Activity.Items == nil || signals.Workflow.Items == nil || signals.Tooling.Items == nil {
		t.Fatalf("partial work signals = %#v, want unavailable non-nil empty families", signals)
	}

	contents, err := os.ReadFile(filepath.Join("..", "..", "desktop", "fixtures", "v1", "snapshot-legacy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Data Snapshot `json:"data"`
	}
	if err = json.Unmarshal(contents, &legacy); err != nil {
		t.Fatal(err)
	}
	legacySignals := legacy.Data.Sessions.WorkSignals
	if legacySignals.Activity.Available || legacySignals.Workflow.Available || legacySignals.Tooling.Available {
		t.Fatalf("legacy work signals = %#v, want every missing family unavailable", legacySignals)
	}
}
