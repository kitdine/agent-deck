package usagehook

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSetupStatusLineRefusesAnotherInstallationWithoutMutation(t *testing.T) {
	home := t.TempDir()
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{"statusLine": json.RawMessage(`{"type":"command","command":"printf prior"}`)}, privateFileMode)
	stateA, stateB := t.TempDir(), t.TempDir()
	a := New(Environment{Home: home, AgentDeckCommand: "agentdeck --state-dir " + stateA, StateDir: stateA})
	b := New(Environment{Home: home, AgentDeckCommand: "agentdeck --state-dir " + stateB, StateDir: stateB})
	if r, err := a.SetupStatusLine(); err != nil || r.Outcome != OutcomeConfigured {
		t.Fatalf("setup A: %+v, %v", r, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.SetupStatusLine()
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeFailed || result.Configuration != ConfigurationModified || result.Error == "" {
		t.Errorf("setup B = %+v; want actionable failed/modified conflict", result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("B changed A's active route")
	}
	if _, found, err := b.readStatusLinePrior(); err != nil || found {
		t.Errorf("B created a prior: found=%v err=%v", found, err)
	}
	if command, ok := a.PriorStatusLineCommand(); !ok || command != "printf prior" {
		t.Errorf("A prior = %q, %v", command, ok)
	}
}
