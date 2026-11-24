package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

// The open-ticket coordinator supplies the two admitted sides. Exercise
// the real prepared participant and actual classify/list/search commands
// across an incompatible birth; no user write is needed to persist it.
type originIndexSide []session.EffectiveSessionCopy

func (s originIndexSide) EffectiveCopies(context.Context, string, string) ([]session.EffectiveSessionCopy, error) {
	return s, nil
}
func (s originIndexSide) SettledBirth(context.Context, sessionorigin.Target) (string, error) {
	return "", nil
}

func TestCJSIClassification(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	state := seedOriginSessions(t)
	ctx := t.Context()
	const id = "shared-id"
	const oldAt = "2026-01-02T09:04:00Z"
	const newAt = "2026-01-02T09:00:00Z"
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	if _, err := index.DB.Exec(`UPDATE session_metadata SET first_at=? WHERE session_id=? AND client='codex'`, oldAt, id); err != nil {
		t.Fatal(err)
	}
	command := func(args ...string) map[string]any {
		return mustOriginOK(t, runOriginCLI(t, state, append([]string{"--format", "json", "session"}, args...)...))
	}
	set := command("classify", id, "--client", "codex", "--set", "auxiliary", "--request-id", "birth-set")
	if set["data"].(map[string]any)["revision"] != float64(1) {
		t.Fatal(set)
	}
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := core.OriginOverrideRecord(ctx, "codex", id)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := index.SessionOriginSourcesFor(ctx, "codex", id)
	if err != nil || len(sources) != 1 {
		t.Fatal("fixture copy", err)
	}
	s := sources[0]
	copyAt := func(at string) originIndexSide {
		return originIndexSide{{Selected: true, Metadata: session.Metadata{Client: "codex", SessionID: id, SourcePath: "local", FirstAt: at}, Source: session.SourceCopy{Identity: s.Identity, PrefixHash: s.PrefixHash, ParserVersion: s.ParserVersion}, HeadAt: at, Birth: at}}
	}
	p, err := sessionorigin.PrepareEffectiveOrigin(ctx, id, time.Now(), func(read func(sessionorigin.EffectiveCopyReader, sessionorigin.OriginChangeInputs) error) error {
		return read(copyAt(newAt), sessionorigin.OriginChangeInputs{Target: sessionorigin.Target{Client: "codex", SessionID: id}, Previous: copyAt(oldAt), Override: &o, TaskChild: true})
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := p.ApplyTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := index.DB.Exec(`UPDATE session_metadata SET first_at=? WHERE session_id=? AND client='codex'`, newAt, id); err != nil {
		t.Fatal(err)
	}
	// Reopen the core-only list before any detail command.
	list := command("classify", "list")["data"].(map[string]any)
	item := list["items"].([]any)[0].(map[string]any)
	if list["total"] != float64(1) || item["binding_status"] != "unmatched" || item["revision"] != float64(1) || item["value"] != "auxiliary" {
		t.Fatal(list)
	}
	detail := command("classify", id, "--client", "codex")["data"].(map[string]any)["origin"].(map[string]any)
	if detail["effective"] != "work" || detail["reason_code"] != sessionorigin.ReasonTaskChildUnconfirmed || detail["override"].(map[string]any)["applied"] != false {
		t.Fatal(detail)
	}
	for _, scope := range []string{"all", "work", "auxiliary"} {
		want := 1
		if scope == "auxiliary" {
			want = 0
		}
		if got := command("search", "needle", "--client", "codex", "--session-scope", scope)["data"].([]any); len(got) != want {
			t.Fatalf("%s search: %d, want %d", scope, len(got), want)
		}
		if got := command("list", "--client", "codex", "--session-scope", scope)["data"].([]any); len(got) != want {
			t.Fatalf("%s list: %d, want %d", scope, len(got), want)
		}
	}
	if got := command("classify", "--request", "birth-set")["data"].(map[string]any); got["revision"] != float64(1) || got["value"] != "auxiliary" {
		t.Fatal(got)
	}
	if got := command("classify", id, "--client", "codex", "--clear")["data"].(map[string]any)["revision"]; got != float64(2) {
		t.Fatal(got)
	}
}

// Use exactly the production command handlers with an already admitted
// managed root, while Task 8 owns dispatcher admission.
func managedClassifyCLI(t *testing.T, root string, core *store.Store, args ...string) (map[string]any, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCommand(bytes.NewReader(nil), &out)
	cmd.SetArgs(append([]string{"--state-dir", root, "--format", "json", "session", "classify"}, args...))
	err := cmd.ExecuteContext(withPublishedOriginCommand(t.Context(), root, core))
	if err != nil {
		return nil, err
	}
	return decodeOriginJSON(t, out.String())["data"].(map[string]any), nil
}
