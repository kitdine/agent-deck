package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestDesktopQuotaRefreshCodexSingleProcess(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(home, "state")
	withTestHome(t, home)
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex-home"))
	t.Setenv("PATH", home)
	script := `#!/bin/sh
echo process >> "$HOME/requests"
while IFS= read -r line; do
  case "$line" in
    *account/rateLimits/read*) echo usage >> "$HOME/requests"; echo '{"id":2,"result":{"accountId":"synthetic-account","rateLimits":{"planType":"prolite","primary":{"usedPercent":37}}}}';;
    *account/read*'"refreshToken":false'*) echo identity >> "$HOME/requests"; echo '{"id":3,"result":{"account":{"type":"chatgpt","email":"fixture@example.test","planType":"prolite"},"requiresOpenaiAuth":true,"userId":"synthetic-account"}}';;
    *initialized*) echo ready >> "$HOME/requests";;
    *initialize*) echo handshake >> "$HOME/requests"; echo '{"id":1,"result":{}}';;
    *) exit 8;;
  esac
done
`
	if err := os.WriteFile(filepath.Join(home, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	seedQuotaState(t, state, func(s *quota.Settings) { s.ProbeClients = []quota.Client{quota.ClientCodex} }, "codex")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	if _, err := os.Stat(filepath.Join(home, "requests")); !os.IsNotExist(err) {
		t.Fatal("reading disabled: started Codex", err)
	}
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-settings", "--reading", "on")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh", "--manual")
	runJSON(t, "--state-dir", state, "--format", "json", "desktop", "quota-refresh")
	b, err := os.ReadFile(filepath.Join(home, "requests"))
	if err != nil || string(b) != "process\nhandshake\nready\nidentity\nusage\n" {
		t.Fatal(string(b), err)
	}
	core, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	var email, association, account, plan string
	var seq, usageSeq, generation int
	if err := core.DB.QueryRow(`SELECT i.email_masked,i.association,i.success_cycle_seq,e.cycle_seq,e.account_id,e.plan,e.context_generation FROM quota_identities i JOIN quota_envelopes_v2 e USING(client) WHERE client='codex'`).Scan(&email, &association, &seq, &usageSeq, &account, &plan, &generation); err != nil {
		t.Fatal(err)
	}
	if email != "f***e@example.test" || association != "" || seq <= 0 || seq != usageSeq || account == "" || strings.Contains(account, "synthetic-account") || generation != 0 || plan != "prolite" {
		t.Fatal("incorrect persisted joint projection")
	}
}
