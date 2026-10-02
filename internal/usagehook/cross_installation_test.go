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

func TestSetupStatusLineRefusesQuotedOptionLikeStatePath(t *testing.T) {
	home := t.TempDir()
	stateA, stateB := t.TempDir()+"/state -- blue", t.TempDir()
	a := New(Environment{Home: home, StateDir: stateA, AgentDeckCommand: "agentdeck --state-dir '" + stateA + "'"})
	b := New(Environment{Home: home, StateDir: stateB, AgentDeckCommand: "agentdeck --state-dir '" + stateB + "'"})
	if r, e := a.SetupStatusLine(); e != nil || r.Outcome != OutcomeConfigured {
		t.Fatalf("A setup: %+v %v", r, e)
	}
	path := configPath(home, ClientClaude)
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	r, e := b.SetupStatusLine()
	if e != nil || r.Outcome != OutcomeFailed {
		t.Errorf("B setup: %+v %v; want refusal", r, e)
	}
	after, e := os.ReadFile(path)
	if e != nil || string(before) != string(after) {
		t.Errorf("route changed: %v", e)
	}
	if _, found, e := b.readStatusLinePrior(); e != nil || found {
		t.Errorf("B prior found=%v err=%v", found, e)
	}
}

func TestSetupStatusLineMigratesExecutableForSameState(t *testing.T) {
	home := t.TempDir()
	state := home + "/.agentdeck"
	path := configPath(home, ClientClaude)
	writeDocument(t, path, map[string]json.RawMessage{"statusLine": json.RawMessage(`{"type":"command","command":"printf prior"}`)}, privateFileMode)
	old := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "agentdeck"})
	replacement := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "'/Applications/AgentDeck.app/Contents/Helpers/agentdeck'"})
	if r, e := old.SetupStatusLine(); e != nil || r.Outcome != OutcomeConfigured {
		t.Fatalf("old setup: %+v %v", r, e)
	}
	priorPath, e := old.statusLinePriorPath()
	if e != nil {
		t.Fatal(e)
	}
	priorBefore, e := os.ReadFile(priorPath)
	if e != nil {
		t.Fatal(e)
	}
	r, e := replacement.SetupStatusLine()
	if e != nil || r.Outcome != OutcomeConfigured {
		t.Fatalf("migration: %+v %v; want configured", r, e)
	}
	if !jsonEquivalent(readDocument(t, path)[statusLineKey], replacement.desiredStatusLineEntry()) {
		t.Fatal("new executable not registered")
	}
	priorAfter, e := os.ReadFile(priorPath)
	if e != nil || string(priorBefore) != string(priorAfter) {
		t.Fatalf("prior changed: %v", e)
	}
	if r, e := replacement.RestoreStatusLine(); e != nil || r.Outcome != OutcomeRemoved {
		t.Fatalf("restore: %+v %v", r, e)
	}
	if cmd, ok := decodeStatusLineCommandEntry(readDocument(t, path)[statusLineKey]); !ok || cmd != "printf prior" {
		t.Fatalf("lost original prior: %q %v", cmd, ok)
	}
}

func TestManagedStatusLineCommandQuotedArguments(t *testing.T) {
	for _, tc := range []struct {
		command, state string
		managed        bool
	}{
		{"agentdeck quota capture", "", true},
		{"agentdeck --state-dir '/tmp/state -- blue' quota capture", "/tmp/state -- blue", true},
		{"'/Applications/Agent --state-dir Deck.app/agentdeck' --state-dir '/tmp/it'\"'\"'s -- blue' quota capture", "/tmp/it's -- blue", true},
		{"agentdeck --state-dir '/tmp/a' --other quota capture", "", false},
		{"agentdeck --state-dir '/tmp/a' ; quota capture", "", false},
		{"agentdeck --state-dir '/tmp/a'junk quota capture", "", false},
		{"agentdeck --state-dir '/tmp/a quota capture", "", false},
		{"agentdeck --state-dir /tmp/a\nquota capture", "", false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			state, ok := managedStatusLineStateDir(tc.command)
			if ok != tc.managed || ok && state != tc.state {
				t.Fatalf("got %q,%v want %q,%v", state, ok, tc.state, tc.managed)
			}
		})
	}
}

func TestSetupStatusLineMigrationRequiresValidPrior(t *testing.T) {
	for _, sidecar := range []string{"missing", "broken", "managed"} {
		t.Run(sidecar, func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			old := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "agentdeck --state-dir '" + state + "'"})
			replacement := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "'/new/agentdeck' --state-dir '" + state + "'"})
			if r, e := old.SetupStatusLine(); e != nil || r.Outcome != OutcomeConfigured {
				t.Fatalf("setup %+v %v", r, e)
			}
			p, e := old.statusLinePriorPath()
			if e != nil {
				t.Fatal(e)
			}
			switch sidecar {
			case "missing":
				e = os.Remove(p)
			case "broken":
				e = os.WriteFile(p, []byte(`{"existed":true}`), 0600)
			case "managed":
				e = old.writeStatusLinePrior(statusLinePriorRecord{Existed: true, Value: old.desiredStatusLineEntry()})
			}
			if e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(configPath(home, ClientClaude))
			if e != nil {
				t.Fatal(e)
			}
			r, e := replacement.SetupStatusLine()
			if e != nil || r.Outcome != OutcomeFailed {
				t.Fatalf("migration %+v %v", r, e)
			}
			after, e := os.ReadFile(configPath(home, ClientClaude))
			if e != nil || string(before) != string(after) {
				t.Fatalf("route changed %v", e)
			}
		})
	}
}

func TestStatusLineMigrationRollbackPreservesPriorAndLaterChanges(t *testing.T) {
	for _, scenario := range []string{"restore-old", "later-route", "deleted"} {
		t.Run(scenario, func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			path := configPath(home, ClientClaude)
			writeDocument(t, path, map[string]json.RawMessage{"statusLine": json.RawMessage(`{"type":"command","command":"printf prior"}`)}, privateFileMode)
			old := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "agentdeck --state-dir '" + state + "'"})
			replacement := New(Environment{Home: home, StateDir: state, AgentDeckCommand: "'/new/agentdeck' --state-dir '" + state + "'"})
			if r, e := old.SetupStatusLine(); e != nil || r.Outcome != OutcomeConfigured {
				t.Fatalf("old setup %+v %v", r, e)
			}
			if r, e := replacement.SetupStatusLine(); e != nil || r.Outcome != OutcomeConfigured {
				t.Fatalf("migration %+v %v", r, e)
			}
			if scenario == "later-route" {
				writeDocument(t, path, map[string]json.RawMessage{"statusLine": json.RawMessage(`{"type":"command","command":"printf later"}`)}, privateFileMode)
			}
			if scenario == "deleted" {
				if e := os.Remove(path); e != nil {
					t.Fatal(e)
				}
			}
			r, e := replacement.RollbackStatusLineSetup()
			if e != nil {
				t.Fatal(e)
			}
			if scenario == "restore-old" {
				if r.Outcome != OutcomeRemoved || !jsonEquivalent(readDocument(t, path)[statusLineKey], old.desiredStatusLineEntry()) {
					t.Fatalf("rollback %+v", r)
				}
			} else if r.Outcome != OutcomeFailed {
				t.Fatalf("rollback %+v; want failed", r)
			}
			if scenario == "later-route" {
				if cmd, ok := decodeStatusLineCommandEntry(readDocument(t, path)[statusLineKey]); !ok || cmd != "printf later" {
					t.Fatalf("later route changed: %q", cmd)
				}
			}
			if scenario == "deleted" {
				if _, e := os.Stat(path); !os.IsNotExist(e) {
					t.Fatalf("deleted file recreated: %v", e)
				}
			}
			if cmd, ok := replacement.PriorStatusLineCommand(); !ok || cmd != "printf prior" {
				t.Fatalf("prior changed %q %v", cmd, ok)
			}
		})
	}
}
