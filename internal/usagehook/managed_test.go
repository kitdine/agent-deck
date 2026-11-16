package usagehook

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestManagedCommandsEnumeratesHooksAndStatusLineReadOnly(t *testing.T) {
	home := t.TempDir()
	writeDocument(t, configPath(home, ClientCodex), map[string]json.RawMessage{
		"hooks": json.RawMessage(`{"SessionStart":[{"hooks":[{"type":"command","command":"agentdeck usage hook event codex"},{"type":"command","command":"printf other"}]}]}`),
	}, privateFileMode)
	writeDocument(t, configPath(home, ClientClaude), map[string]json.RawMessage{
		"hooks":      json.RawMessage(`{"SessionStart":[{"hooks":[{"type":"command","command":"'/Applications/Old App.app/Contents/Helpers/agentdeck' --state-dir '/tmp/s' usage hook event claude"}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"/opt/x/agentdeck usage hook event codex"}]}]}`),
		"statusLine": json.RawMessage(`{"type":"command","command":"'/usr/local/bin/agentdeck' quota capture"}`),
	}, privateFileMode)
	before, _ := os.ReadFile(configPath(home, ClientClaude))
	got := New(Environment{Home: home}).ManagedCommands()
	want := []ManagedCommand{
		{Client: ClientCodex, Kind: ManagedHook, Executable: ""},
		{Client: ClientClaude, Kind: ManagedHook, Executable: "/Applications/Old App.app/Contents/Helpers/agentdeck"},
		{Client: ClientClaude, Kind: ManagedStatusLine, Executable: "/usr/local/bin/agentdeck"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %+v\nwant %+v", got, want)
	}
	if after, _ := os.ReadFile(configPath(home, ClientClaude)); string(after) != string(before) {
		t.Fatal("enumeration changed the configuration")
	}
}

func TestManagedCommandsIgnoresAbsentAndInvalidConfiguration(t *testing.T) {
	home := t.TempDir()
	if got := New(Environment{Home: home}).ManagedCommands(); len(got) != 0 {
		t.Fatalf("absent = %+v", got)
	}
	path := configPath(home, ClientClaude)
	if err := os.MkdirAll(home+"/.claude", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := New(Environment{Home: home}).ManagedCommands(); len(got) != 0 {
		t.Fatalf("invalid = %+v", got)
	}
	if _, err := os.Stat(home + "/.codex"); !os.IsNotExist(err) {
		t.Fatalf("enumeration created a directory: %v", err)
	}
}
