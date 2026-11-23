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

func TestDesktopQuotaRefreshCodexUsageOnly(t *testing.T) {
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
	if err != nil || string(b) != "process\nhandshake\nready\nusage\n" {
		t.Fatal(string(b), err)
	}
	core, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	var account, plan string
	var seq, generation, identities int
	if err := core.DB.QueryRow(`SELECT cycle_seq,account_id,plan,context_generation,(SELECT COUNT(*) FROM quota_identities WHERE client='codex') FROM quota_envelopes_v2 WHERE client='codex'`).Scan(&seq, &account, &plan, &generation, &identities); err != nil {
		t.Fatal(err)
	}
	if seq <= 0 || account == "" || strings.Contains(account, "synthetic-account") || generation != 0 || plan != "prolite" || identities != 0 {
		t.Fatal("incorrect usage-only commit")
	}
}
