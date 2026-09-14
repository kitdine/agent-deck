package desktop

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

func openSubscriptionStore(t *testing.T) *store.Store {
	t.Helper()
	core, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { core.Close() })
	return core
}

func recordOfficial(t *testing.T, core *store.Store, clients ...string) {
	t.Helper()
	for _, client := range clients {
		if err := core.RecordSelection(context.Background(), store.Selection{
			Client: client, ProviderName: "official", MultiplierSnapshot: "1",
			SelectedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RecordSelection: %v", err)
		}
	}
}

func saveQuotaSettings(t *testing.T, core *store.Store, mutate func(*quota.Settings)) {
	t.Helper()
	settings := quota.DefaultSettings()
	mutate(&settings)
	if err := quota.SaveSettings(context.Background(), core, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
}

func recordQuotaWindow(t *testing.T, core *store.Store, obs quota.Observation) {
	t.Helper()
	if _, _, _, err := quota.NewStore(core.DB).Record(context.Background(), obs); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func subscriptionFor(t *testing.T, subscription SubscriptionSnapshot, client string) SubscriptionClient {
	t.Helper()
	for _, entry := range subscription.Clients {
		if entry.Client == client {
			return entry
		}
	}
	t.Fatalf("no %s client in %+v", client, subscription)
	return SubscriptionClient{}
}

func reasonIs(value *string, want quota.Reason) bool {
	return value != nil && *value == string(want)
}

func TestBuildSubscriptionReadingOffByDefaultKeepsStoredObservations(t *testing.T) {
	core := openSubscriptionStore(t)
	recordOfficial(t, core, "codex", "claude")
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeStatusLine,
		ObservedAt: now.Add(-time.Minute), WindowMinutes: 300, UsedPercent: 40, ResetsAt: now.Add(3 * time.Hour),
	})

	subscription, err := Service{Home: t.TempDir()}.BuildSubscription(context.Background(), core, now)
	if err != nil {
		t.Fatalf("BuildSubscription: %v", err)
	}
	if !subscription.Available || len(subscription.Clients) != 2 {
		t.Fatalf("subscription = %+v, want both clients present, not an absent section", subscription)
	}
	for _, name := range []string{"codex", "claude"} {
		client := subscriptionFor(t, subscription, name)
		if !client.Applicable || !reasonIs(client.Failure, quota.ReasonProbeDisabled) || len(client.Windows) != 0 || client.ObservedAt != nil || !reasonIs(client.ResetAllowanceReason, quota.ReasonProbeDisabled) {
			t.Fatalf("%s = %+v, want reading off reported as probe_disabled with no figure", name, client)
		}
	}
	if !reasonIs(subscriptionFor(t, subscription, "codex").PlanReason, quota.ReasonProbeDisabled) {
		t.Fatal("codex plan_reason must be probe_disabled while reading is off")
	}
	windows, err := quota.NewStore(core.DB).Windows(context.Background(), quota.ClientClaude)
	if err != nil || len(windows) != 1 {
		t.Fatalf("stored windows = (%+v, %v), want the observation retained (requirements.md clause 2)", windows, err)
	}
}

func TestBuildSubscriptionReportsTheGateAndNeverProbedAsClientStates(t *testing.T) {
	core := openSubscriptionStore(t)
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	recordOfficial(t, core, "claude")

	subscription, err := Service{Home: t.TempDir()}.BuildSubscription(context.Background(), core, time.Now())
	if err != nil {
		t.Fatalf("BuildSubscription: %v", err)
	}
	codex := subscriptionFor(t, subscription, "codex")
	if codex.Applicable || !reasonIs(codex.ApplicableReason, quota.ReasonNotOfficial) || codex.Failure != nil || !reasonIs(codex.PlanReason, quota.ReasonNotOfficial) {
		t.Fatalf("codex = %+v, want not applicable with not_official", codex)
	}
	claude := subscriptionFor(t, subscription, "claude")
	if !claude.Applicable || !reasonIs(claude.Failure, quota.ReasonNeverProbed) || !reasonIs(claude.PlanReason, quota.ReasonNotReported) {
		t.Fatalf("claude = %+v, want applicable and never_probed", claude)
	}
}

func TestBuildSubscriptionCarriesFiguresInVendorOrderWithTheTightestWindow(t *testing.T) {
	core := openSubscriptionStore(t)
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	recordOfficial(t, core, "codex", "claude")
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	observed := now.Add(-time.Minute)
	const account = "acct-secret-4711"

	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientCodex, AccountID: account, WindowKey: "codex", Source: quota.SourceCodex,
		ObservedAt: observed, VendorOrder: 0, WindowMinutes: 10080, UsedPercent: 40, ResetsAt: now.Add(48 * time.Hour),
	})
	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientCodex, AccountID: account, WindowKey: "codex_bengalfox", Source: quota.SourceCodex, Label: "GPT-5.3-Codex-Spark",
		ObservedAt: observed, VendorOrder: 1, WindowMinutes: 300, UsedPercent: 70, ResetsAt: now.Add(2 * time.Hour),
	})
	if err := quota.NewStore(core.DB).PutEnvelope(context.Background(), quota.EnvelopeRecord{
		Client: quota.ClientCodex, AccountID: account, Applicable: true, Source: quota.SourceCodex, ObservedAt: observed, Plan: "pro",
		ResetAllowance: quota.ResetAllowance{TotalReason: quota.ReasonNotReported, Remaining: 2, HasRemaining: true,
			Credits: []quota.ResetCredit{{Title: "Full reset", Status: "available", GrantedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour)}}},
		Billing: quota.Billing{Balance: 12.5, HasBalance: true},
	}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}
	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeStatusLine,
		ObservedAt: observed, WindowMinutes: 300, UsedPercent: 22, ResetsAt: now.Add(3 * time.Hour),
	})

	subscription, err := Service{Home: t.TempDir()}.BuildSubscription(context.Background(), core, now)
	if err != nil {
		t.Fatalf("BuildSubscription: %v", err)
	}
	codex := subscriptionFor(t, subscription, "codex")
	if len(codex.Windows) != 2 || codex.Windows[0].Key != "codex" || codex.Windows[1].Key != "codex_bengalfox" {
		t.Fatalf("codex windows = %+v, want vendor order, not sorted by used share", codex.Windows)
	}
	if codex.TightestWindowKey == nil || *codex.TightestWindowKey != "codex_bengalfox" {
		t.Fatalf("tightest_window_key = %v, want codex_bengalfox", codex.TightestWindowKey)
	}
	if codex.Plan == nil || *codex.Plan != "pro" || !reasonIs(codex.Source, "codex_app_server") || !codex.AttributionConfirmed || codex.Stale {
		t.Fatalf("codex = %+v", codex)
	}
	allowance := codex.ResetAllowance
	if allowance == nil || allowance.Remaining == nil || *allowance.Remaining != 2 || allowance.Total != nil || !reasonIs(allowance.TotalReason, quota.ReasonNotReported) || len(allowance.Credits) != 1 || allowance.Credits[0].Key != "c1" {
		t.Fatalf("reset allowance = %+v", allowance)
	}
	claude := subscriptionFor(t, subscription, "claude")
	if !reasonIs(claude.Source, "claude_statusline") || claude.AttributionConfirmed || claude.ResetAllowance != nil || !reasonIs(claude.ResetAllowanceReason, quota.ReasonNotReported) {
		t.Fatalf("claude = %+v", claude)
	}

	encoded, err := json.Marshal(subscription)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, forbidden := range []string{"account_id", account, "balance", "billing"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("subscription JSON carries %q (C11 excludes account_id and billing.balance): %s", forbidden, encoded)
		}
	}
}

func TestBuildSubscriptionTightestWindowIsNullWithoutWindows(t *testing.T) {
	core := openSubscriptionStore(t)
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	recordOfficial(t, core, "codex")
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if err := quota.NewStore(core.DB).PutEnvelope(context.Background(), quota.EnvelopeRecord{
		Client: quota.ClientCodex, Applicable: true, Source: quota.SourceCodex, ObservedAt: now, PlanReason: quota.ReasonNotReported,
	}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}
	subscription, err := Service{Home: t.TempDir()}.BuildSubscription(context.Background(), core, now)
	if err != nil {
		t.Fatalf("BuildSubscription: %v", err)
	}
	codex := subscriptionFor(t, subscription, "codex")
	if codex.TightestWindowKey != nil || codex.Windows == nil || len(codex.Windows) != 0 {
		t.Fatalf("codex = %+v, want an empty windows array and a null tightest_window_key", codex)
	}
}

func TestBuildSubscriptionFailureStates(t *testing.T) {
	core := openSubscriptionStore(t)
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	recordOfficial(t, core, "codex", "claude")
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	qs := quota.NewStore(core.DB)

	// Codex: a probe failure after a success keeps the last figure, with its
	// real age (C9).
	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientCodex, AccountID: "acct", WindowKey: "codex", Source: quota.SourceCodex,
		ObservedAt: t1, WindowMinutes: 300, UsedPercent: 55, ResetsAt: t1.Add(4 * time.Hour),
	})
	if err := qs.PutEnvelope(context.Background(), quota.EnvelopeRecord{Client: quota.ClientCodex, AccountID: "acct", Applicable: true, Source: quota.SourceCodex, ObservedAt: t1, Plan: "pro"}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}
	if err := qs.PutEnvelopeFailure(context.Background(), quota.ClientCodex, quota.ReasonProbeFailed, t2, t2.Add(5*time.Minute), t2); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}
	// Claude: a parse failure after a success presents no figure
	// (requirements.md clause 6).
	recordQuotaWindow(t, core, quota.Observation{
		Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeProse,
		ObservedAt: t1, WindowMinutes: 300, UsedPercent: 30, ResetsAt: t1.Add(4 * time.Hour),
	})
	if err := qs.PutEnvelopeFailure(context.Background(), quota.ClientClaude, quota.ReasonParseFailed, t2, time.Time{}, t2); err != nil {
		t.Fatalf("PutEnvelopeFailure: %v", err)
	}

	subscription, err := Service{Home: t.TempDir()}.BuildSubscription(context.Background(), core, t2.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSubscription: %v", err)
	}
	codex := subscriptionFor(t, subscription, "codex")
	if len(codex.Windows) != 1 || !reasonIs(codex.Failure, quota.ReasonProbeFailed) || codex.ObservedAt == nil || *codex.ObservedAt != t1.Format(time.RFC3339Nano) || !codex.Stale {
		t.Fatalf("codex = %+v, want the last figure kept, observed at %v, stale, failure probe_failed", codex, t1)
	}
	claude := subscriptionFor(t, subscription, "claude")
	if len(claude.Windows) != 0 || !reasonIs(claude.Failure, quota.ReasonParseFailed) || claude.ObservedAt == nil || *claude.ObservedAt != t2.Format(time.RFC3339Nano) {
		t.Fatalf("claude = %+v, want no figure, failure parse_failed, observed at the failed attempt %v", claude, t2)
	}
}

func TestReadingOffSubscriptionReportsBothClients(t *testing.T) {
	subscription := ReadingOffSubscription()
	if !subscription.Available || len(subscription.Clients) != 2 {
		t.Fatalf("subscription = %+v", subscription)
	}
	for _, client := range subscription.Clients {
		if !reasonIs(client.Failure, quota.ReasonProbeDisabled) {
			t.Fatalf("%s = %+v, want probe_disabled", client.Client, client)
		}
	}
}

// seedQuota gives the complete canonical fixture a populated subscription
// section, so the decoders are exercised on every field and not only on two
// reading-off records.
func seedQuota(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open core: %v", err)
	}
	defer func() {
		if closeErr := core.Close(); closeErr != nil {
			t.Fatalf("close core: %v", closeErr)
		}
	}()
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	observed := fixtureNow.Add(-2 * time.Minute)
	qs := quota.NewStore(core.DB)
	for _, obs := range []quota.Observation{
		{Client: quota.ClientCodex, AccountID: "fixture-account", WindowKey: "codex", Source: quota.SourceCodex,
			ObservedAt: observed, VendorOrder: 0, WindowMinutes: 10080, UsedPercent: 79, ResetsAt: fixtureNow.Add(5*24*time.Hour + 3*time.Hour)},
		{Client: quota.ClientCodex, AccountID: "fixture-account", WindowKey: "codex_bengalfox", Source: quota.SourceCodex, Label: "GPT-5.3-Codex-Spark",
			ObservedAt: observed, VendorOrder: 1, WindowMinutes: 300, UsedPercent: 12, ResetsAt: fixtureNow.Add(2*time.Hour + 40*time.Minute)},
		{Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowFiveHour, Source: quota.SourceClaudeStatusLine,
			ObservedAt: observed, WindowMinutes: 300, UsedPercent: 22, ResetsAt: fixtureNow.Add(3*time.Hour + 20*time.Minute)},
		{Client: quota.ClientClaude, WindowKey: quota.ClaudeWindowSevenDay, Source: quota.SourceClaudeStatusLine,
			ObservedAt: observed, VendorOrder: 1, WindowMinutes: 10080, UsedPercent: 3, ResetsAt: fixtureNow.Add(6*24*time.Hour + 5*time.Hour)},
	} {
		if _, _, _, err := qs.Record(ctx, obs); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	if err := qs.PutEnvelope(ctx, quota.EnvelopeRecord{
		Client: quota.ClientCodex, AccountID: "fixture-account", Applicable: true, Source: quota.SourceCodex, ObservedAt: observed, Plan: "prolite",
		ResetAllowance: quota.ResetAllowance{TotalReason: quota.ReasonNotReported, Remaining: 3, HasRemaining: true,
			Credits: []quota.ResetCredit{{Title: "Full reset", Status: "available", GrantedAt: fixtureNow.Add(-5 * 24 * time.Hour), ExpiresAt: fixtureNow.Add(25 * 24 * time.Hour)}}},
	}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}
}
