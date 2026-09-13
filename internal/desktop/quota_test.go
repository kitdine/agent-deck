package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

// fakeQuotaProbes counts invocations per client and always succeeds with no
// windows, so these tests exercise RefreshQuota's own gating and wiring
// (architecture.md C1/C9) without ever risking a call to a real "codex" or
// "claude" binary.
type fakeQuotaProbes struct {
	codexCalls  int
	claudeCalls int
}

func (f *fakeQuotaProbes) codex(_ context.Context, _ time.Time, _ time.Duration) (quota.CodexResult, error) {
	f.codexCalls++
	return quota.CodexResult{}, nil
}

func (f *fakeQuotaProbes) claude(_ context.Context, _ time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
	f.claudeCalls++
	return quota.ClaudeProseResult{}, nil
}

func openWritableQuotaTestStore(t *testing.T, root string) *store.Store {
	t.Helper()
	core, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	t.Cleanup(func() { core.Close() })
	return core
}

func TestRefreshQuotaProbeDisabledDoesNothing(t *testing.T) {
	root := t.TempDir()
	seedSelections(t, root) // both clients official
	core := openWritableQuotaTestStore(t, root)
	fakes := &fakeQuotaProbes{}

	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, false, 5*time.Minute, time.Hour)

	if fakes.codexCalls != 0 || fakes.claudeCalls != 0 {
		t.Fatalf("codexCalls=%d claudeCalls=%d, want 0/0 with probing disabled", fakes.codexCalls, fakes.claudeCalls)
	}
	quotaStore := quota.NewStore(core.DB)
	if _, ok, err := quotaStore.Envelope(context.Background(), quota.ClientCodex); err != nil || ok {
		t.Fatalf("Codex envelope = (ok=%v, err=%v), want none written while probing is disabled", ok, err)
	}
}

func TestRefreshQuotaProbesBothClientsWhenOfficialAndEnabled(t *testing.T) {
	root := t.TempDir()
	seedSelections(t, root) // both clients official
	core := openWritableQuotaTestStore(t, root)
	fakes := &fakeQuotaProbes{}

	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)

	if fakes.codexCalls != 1 || fakes.claudeCalls != 1 {
		t.Fatalf("codexCalls=%d claudeCalls=%d, want 1/1 when both clients are official and probing is enabled", fakes.codexCalls, fakes.claudeCalls)
	}
	quotaStore := quota.NewStore(core.DB)
	for _, client := range []quota.Client{quota.ClientCodex, quota.ClientClaude} {
		env, ok, err := quotaStore.Envelope(context.Background(), client)
		if err != nil || !ok || !env.Applicable {
			t.Fatalf("%s envelope = (%+v, %v, %v), want an applicable envelope", client, env, ok, err)
		}
	}
}

func TestRefreshQuotaSuppressesClientWithoutOfficialSelection(t *testing.T) {
	root := t.TempDir()
	core := openWritableQuotaTestStore(t, root)
	// Only Codex has a completed, official selection; Claude has none at all.
	if err := core.RecordSelection(context.Background(), store.Selection{
		Client: "codex", ProviderName: "official", MultiplierSnapshot: "1",
		SelectedAt: time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordSelection: %v", err)
	}
	fakes := &fakeQuotaProbes{}

	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)

	if fakes.codexCalls != 1 {
		t.Fatalf("codexCalls = %d, want 1 for the client with a completed official selection", fakes.codexCalls)
	}
	if fakes.claudeCalls != 0 {
		t.Fatalf("claudeCalls = %d, want 0 for a client with no completed selection at all", fakes.claudeCalls)
	}
	quotaStore := quota.NewStore(core.DB)
	if _, ok, err := quotaStore.Envelope(context.Background(), quota.ClientClaude); err != nil || ok {
		t.Fatalf("Claude envelope = (ok=%v, err=%v), want none written", ok, err)
	}
}

func TestRefreshQuotaManualTriggerBypassesInterval(t *testing.T) {
	root := t.TempDir()
	seedSelections(t, root)
	core := openWritableQuotaTestStore(t, root)
	fakes := &fakeQuotaProbes{}
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return now },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 1 {
		t.Fatalf("codexCalls after the first background probe = %d, want 1", fakes.codexCalls)
	}

	// A background trigger one second later must not re-probe...
	service.Now = func() time.Time { return now.Add(time.Second) }
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 1 {
		t.Fatalf("codexCalls after an immediate background refresh = %d, want still 1", fakes.codexCalls)
	}

	// ...but a manual trigger at the same instant does (C9: the interval
	// does not gate a user-initiated refresh).
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerManual, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 2 {
		t.Fatalf("codexCalls after an immediate manual refresh = %d, want 2", fakes.codexCalls)
	}
}

func TestRefreshQuotaRetainsObservationsWhenDisabledAfterASuccess(t *testing.T) {
	// requirements.md clause 2 / architecture.md C9: turning reading off
	// retains stored observations rather than probing or discarding them.
	root := t.TempDir()
	seedSelections(t, root)
	core := openWritableQuotaTestStore(t, root)
	fakes := &fakeQuotaProbes{}
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return now },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)

	quotaStore := quota.NewStore(core.DB)
	before, ok, err := quotaStore.Envelope(context.Background(), quota.ClientCodex)
	if err != nil || !ok {
		t.Fatalf("Envelope before disabling = (%+v, %v, %v)", before, ok, err)
	}

	service.Now = func() time.Time { return now.Add(time.Hour) }
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, false, 5*time.Minute, time.Hour)

	if fakes.codexCalls != 1 {
		t.Fatalf("codexCalls after disabling = %d, want still 1 (no probe while disabled)", fakes.codexCalls)
	}
	after, ok, err := quotaStore.Envelope(context.Background(), quota.ClientCodex)
	if err != nil || !ok || !after.ObservedAt.Equal(before.ObservedAt) {
		t.Fatalf("Envelope after disabling = (%+v, %v, %v), want it retained unchanged from %+v", after, ok, err, before)
	}
}

func TestRefreshQuotaIgnoresStaleObservedProviderPredatingCurrentSelection(t *testing.T) {
	// GS-R1-F1 regression: a Hook-observed provider recorded before the
	// user's current, official selection within AgentDeck must not
	// permanently suppress the probe. Before the fix, quotaObservedOfficial
	// compared the observed provider against the recorded selection without
	// checking which was newer, so a stale disagreement blocked every
	// background AND manual refresh indefinitely.
	root := t.TempDir()
	core := openWritableQuotaTestStore(t, root)

	staleAt := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	usageService := usage.New(core, "")
	usageService.Now = func() time.Time { return staleAt }
	if err := usageService.RecordHookDelivery(context.Background(), usage.HookDelivery{
		Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: "stale-observation",
		HasSelection: true, Selection: store.ProviderSnapshot{Name: "relay", Multiplier: "1"},
	}); err != nil {
		t.Fatalf("RecordHookDelivery: %v", err)
	}

	// The user switches to official within AgentDeck strictly after that
	// stale observation.
	selectedAt := staleAt.Add(time.Hour)
	if err := core.RecordSelection(context.Background(), store.Selection{
		Client: "codex", ProviderName: "official", MultiplierSnapshot: "1", SelectedAt: selectedAt,
	}); err != nil {
		t.Fatalf("RecordSelection: %v", err)
	}

	fakes := &fakeQuotaProbes{}
	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return selectedAt.Add(time.Minute) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 1 {
		t.Fatalf("codexCalls after a background refresh past a stale disagreeing observation = %d, want 1", fakes.codexCalls)
	}

	service.Now = func() time.Time { return selectedAt.Add(2 * time.Minute) }
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerManual, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 2 {
		t.Fatalf("codexCalls after a manual refresh past a stale disagreeing observation = %d, want 2", fakes.codexCalls)
	}
}

func TestRefreshQuotaSuppressesOnCurrentDisagreeingObservation(t *testing.T) {
	// Control case for the GS-R1-F1 fix: an observation strictly newer than
	// the current selection must still suppress the probe on disagreement —
	// the fix rejects only stale evidence, not the cross-check itself.
	root := t.TempDir()
	core := openWritableQuotaTestStore(t, root)

	selectedAt := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	if err := core.RecordSelection(context.Background(), store.Selection{
		Client: "codex", ProviderName: "official", MultiplierSnapshot: "1", SelectedAt: selectedAt,
	}); err != nil {
		t.Fatalf("RecordSelection: %v", err)
	}

	usageService := usage.New(core, "")
	usageService.Now = func() time.Time { return selectedAt.Add(time.Minute) }
	if err := usageService.RecordHookDelivery(context.Background(), usage.HookDelivery{
		Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: "fresh-observation",
		HasSelection: true, Selection: store.ProviderSnapshot{Name: "relay", Multiplier: "1"},
	}); err != nil {
		t.Fatalf("RecordHookDelivery: %v", err)
	}

	fakes := &fakeQuotaProbes{}
	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return selectedAt.Add(2 * time.Minute) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)
	if fakes.codexCalls != 0 {
		t.Fatalf("codexCalls = %d, want 0: a disagreeing observation newer than the current selection must still suppress the probe", fakes.codexCalls)
	}
}

func TestRefreshQuotaReturnsGateOutcomePerClient(t *testing.T) {
	// GS-R1-F5 regression: the Reason quota.Allowed computes must not be
	// silently discarded at RefreshQuota's call site.
	root := t.TempDir()
	core := openWritableQuotaTestStore(t, root)
	// Claude has a completed official selection; Codex has none at all.
	if err := core.RecordSelection(context.Background(), store.Selection{
		Client: "claude", ProviderName: "official", MultiplierSnapshot: "1",
		SelectedAt: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordSelection: %v", err)
	}
	fakes := &fakeQuotaProbes{}
	service := Service{
		StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(),
		Now:             func() time.Time { return time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC) },
		QuotaProbeCodex: fakes.codex, QuotaProbeClaudeProse: fakes.claude,
	}
	outcome := service.RefreshQuota(context.Background(), core, service.Home, quota.TriggerBackground, true, 5*time.Minute, time.Hour)
	if outcome[quota.ClientCodex] != quota.ReasonNotOfficial {
		t.Fatalf("outcome[codex] = %q, want %q", outcome[quota.ClientCodex], quota.ReasonNotOfficial)
	}
	if outcome[quota.ClientClaude] != "" {
		t.Fatalf("outcome[claude] = %q, want empty (allowed)", outcome[quota.ClientClaude])
	}
}
