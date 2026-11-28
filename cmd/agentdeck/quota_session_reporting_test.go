package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const quotaFixtureSession = "quota-fixture-session"

type quotaCLIFixture struct {
	t             *testing.T
	ctx           context.Context
	core          *store.Store
	home, state   string
	client        quota.Client
	model         string
	endpoint      string
	baseline, now time.Time
	probes        int
	extraPool     bool
	resetShort    bool
}

func newQuotaCLIFixture(t *testing.T, client quota.Client) *quotaCLIFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	useUTCDisplayClock(t)
	f := &quotaCLIFixture{
		t: t, ctx: context.Background(), state: filepath.Join(home, "state"), home: home, client: client,
		baseline: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
		model:    "gpt-quota-fixture", endpoint: "v1/responses",
	}
	if client == quota.ClientClaude {
		f.model, f.endpoint = "claude-quota-fixture", "v1/messages"
	}
	f.now = f.baseline.Add(2 * time.Minute)
	index, err := store.OpenSessions(f.ctx, f.state)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceDocuments(f.ctx, index.DB, string(client), quotaFixtureSession,
		[]session.Document{{Client: string(client), SessionID: quotaFixtureSession, Kind: "user_prompt", Text: "quota fixture"}}); err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	f.core, err = store.Open(f.ctx, f.state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.core.Close() })
	at := f.baseline.Format(time.RFC3339Nano)
	f.exec(`INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES(?,?,?,?)`, client, quotaFixtureSession, at, at)
	if err := f.core.RecordSelection(f.ctx, store.Selection{Client: string(client), ProviderName: "official",
		MultiplierSnapshot: "1", SelectedAt: f.baseline.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	settings := quota.DefaultSettings()
	settings.ProbeEnabled = true
	if err := quota.SaveSettings(f.ctx, f.core, settings); err != nil {
		t.Fatal(err)
	}
	f.installPrices()

	oldHome, oldProbe := userHomeDir, quotaRefreshService
	userHomeDir = func() (string, error) { return home, nil }
	quotaRefreshService = f.probeService
	t.Cleanup(func() { userHomeDir, quotaRefreshService = oldHome, oldProbe })
	return f
}

func (f *quotaCLIFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.core.DB.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *quotaCLIFixture) installPrices() {
	f.t.Helper()
	provider, sourceURL := "openai", "https://developers.openai.com/api/docs/pricing"
	if f.client == quota.ClientClaude {
		provider, sourceURL = "anthropic", "https://platform.claude.com/docs/en/about-claude/pricing"
	}
	pricing := usage.New(f.core, "")
	pricing.Now = func() time.Time { return f.baseline.Add(-time.Hour) }
	if err := pricing.ImportBundledCatalog(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	const start = "2026-01-01T00:00:00Z"
	boundary := usage.PriceRuleBoundary{Value: f.baseline.Add(time.Minute).Format(time.RFC3339Nano), Precision: "instant"}
	current := usage.PriceRule{
		ID: "fixture.standard", Selector: usage.PriceRuleSelector{
			Provider: provider, Channel: "public_api", Models: []string{f.model},
			Endpoints: []string{f.endpoint}, Regions: []string{"global"}, ServiceTier: "standard",
		},
		Validity: usage.PriceRuleValidity{Basis: usage.PriceRuleEffective,
			From: boundary},
		Sources:  []usage.PriceRuleSource{{URL: sourceURL, Revision: "fixture-v1", VerifiedOn: "2026-01-01", Kind: "vendor_dated"}},
		Currency: "USD", Unit: usage.PriceRuleUSDPerMillion,
		Rates: map[string]string{"input": "2", "output": "6"},
	}
	historical := current
	historical.ID = "fixture.previous"
	historical.Validity.From = usage.PriceRuleBoundary{Value: start, Precision: "instant"}
	historical.Validity.To = &boundary
	historical.Rates = map[string]string{"input": "2", "output": "12"}
	wireRules, err := json.Marshal(usage.PriceRuleSet{SchemaVersion: 2, Rules: []usage.PriceRule{historical, current}})
	if err != nil {
		f.t.Fatal(err)
	}
	rules, err := usage.DecodePriceRules(wireRules)
	if err != nil {
		f.t.Fatal(err)
	}
	catalog, err := json.Marshal(map[string]any{
		"schema_version": 1, "catalog_version": "quota-pricing-fixture", "currency": "USD",
		"models": map[string]any{f.model: map[string]any{
			"provider": provider, "effective_from": start, "prices_per_million": map[string]string{"input": "2", "output": "6"},
		}},
		"pricing_rules": rules,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := pricing.ImportOfficialCatalog(f.ctx, catalog, sourceURL); err != nil {
		f.t.Fatal(err)
	}
}

func (f *quotaCLIFixture) event(key, sessionID string, at time.Time, input, output int64, evidence *usage.RequestPricingEvidence, outputKnown bool) {
	f.t.Helper()
	f.exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
		VALUES (?,?,?,?,?,?,?,?,'/private/fixture-transcript',0)`,
		key, f.client, sessionID, "private-request-marker", at.Format(time.RFC3339Nano), f.model, input, output)
	var zero int64
	facts := usage.RequestFacts{
		Source: "fixture", Association: "fixture", ObservedAt: at.Format(time.RFC3339Nano), ExecutedTier: "standard",
		InputTokens: &input, CachedReadTokens: &zero, CacheWriteTokens: &zero,
		CacheWrite5m: &zero, CacheWrite1h: &zero, CacheWrite30m: &zero, CacheWrite12h: &zero, Pricing: evidence,
	}
	if outputKnown {
		facts.OutputTokens = &output
	}
	payload, err := json.Marshal(facts)
	if err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT OR REPLACE INTO usage_request_facts(event_key,facts_json) VALUES (?,?)`, key, string(payload))
	if sessionID == quotaFixtureSession {
		f.exec(`UPDATE usage_sessions SET last_at=? WHERE client=? AND session_id=?`, at.Format(time.RFC3339Nano), f.client, sessionID)
	}
}

func (f *quotaCLIFixture) eligible() *usage.RequestPricingEvidence {
	value := &usage.RequestPricingEvidence{APIEndpoint: f.endpoint, Region: "global", Activity: "ordinary"}
	if f.client == quota.ClientCodex {
		value.Channel, value.ChannelEndpoint = "personal_subscription", "codex"
		value.Eligibility = []string{"chatgpt_sign_in_included_usage"}
	}
	return value
}

func (f *quotaCLIFixture) probeService(root, home string) desktop.Service {
	s := desktop.Service{StateRoot: root, Home: home, Now: func() time.Time { return f.now }}
	s.QuotaProbeCodex = func(_ context.Context, at time.Time, _ time.Duration) (quota.CodexResult, error) {
		if f.client != quota.ClientCodex {
			return quota.CodexResult{}, errors.New("unexpected Codex probe")
		}
		return quota.CodexResult{AccountID: "private-fixture-account", Windows: f.windows(at)}, nil
	}
	s.QuotaProbeClaudeProse = func(_ context.Context, at time.Time, _ time.Duration) (quota.ClaudeProseResult, error) {
		if f.client != quota.ClientClaude {
			return quota.ClaudeProseResult{}, errors.New("unexpected Claude probe")
		}
		return quota.ClaudeProseResult{Windows: f.windows(at)}, nil
	}
	return s
}

func (f *quotaCLIFixture) windows(_ time.Time) []quota.Observation {
	f.probes++
	f.now = f.now.Add(time.Second)
	pool, source := "codex", quota.SourceCodex
	short, long, firstSlot, secondSlot := "codex", "codex_secondary", "primary", "secondary"
	if f.client == quota.ClientClaude {
		pool, source = "claude", quota.SourceClaudeProse
		short, long, firstSlot, secondSlot = quota.ClaudeWindowFiveHour, quota.ClaudeWindowSevenDay, quota.ClaudeWindowFiveHour, quota.ClaudeWindowSevenDay
	}
	result := []quota.Observation{
		{Client: f.client, Source: source, WindowKey: short, Slot: firstSlot, Pool: pool, WindowMinutes: 300,
			UsedPercent: 40, ResetsAt: f.baseline.Add(3 * time.Hour)},
		{Client: f.client, Source: source, WindowKey: long, Slot: secondSlot, Pool: pool, WindowMinutes: 10080,
			UsedPercent: 50, ResetsAt: f.baseline.Add(24 * time.Hour)},
	}
	if f.probes == 2 {
		result[0].UsedPercent, result[1].UsedPercent = 44, 52
		if f.resetShort {
			result[0].ResetsAt = result[0].ResetsAt.Add(time.Hour)
		}
	}
	if f.extraPool {
		other := result[0]
		other.WindowKey, other.Pool = "review", "review"
		result = append(result, other)
	}
	return result
}

func (f *quotaCLIFixture) cli(format string, args ...string) []byte {
	f.t.Helper()
	var out, stderr bytes.Buffer
	argv := append([]string{"--state-dir", f.state, "--format", format}, args...)
	if exit := execute(argv, bytes.NewReader(nil), &out, &stderr); exit != 0 || stderr.Len() != 0 {
		f.t.Fatalf("%v exit=%d stderr=%q stdout=%q", args, exit, &stderr, &out)
	}
	return out.Bytes()
}

func (f *quotaCLIFixture) sample(action quota.SampleAction) quota.SampleResult {
	f.t.Helper()
	raw := f.cli("json", "quota", "sample", string(action), "--client", string(f.client), "--session", quotaFixtureSession)
	var envelope struct {
		Command string             `json:"command"`
		Data    quota.SampleResult `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Command != "quota.sample."+string(action) {
		f.t.Fatalf("sample response=%s, %v", raw, err)
	}
	quotaFixturePrivacy(f.t, raw)
	return envelope.Data
}

type quotaShowEnvelope struct {
	Data struct {
		Usage       usage.SessionSummary      `json:"usage"`
		Invocations []usage.SessionInvocation `json:"invocations"`
	} `json:"data"`
}

func (f *quotaCLIFixture) show(flags ...string) quotaShowEnvelope {
	f.t.Helper()
	args := append([]string{"session", "show", quotaFixtureSession, "--client", string(f.client), "--tokens"}, flags...)
	raw := f.cli("json", args...)
	var envelope quotaShowEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.Usage.QuotaValuation == nil {
		f.t.Fatalf("session response=%s, %v", raw, err)
	}
	quotaFixturePrivacy(f.t, raw)
	return envelope
}

func quotaFixturePrivacy(t *testing.T, raw []byte) {
	t.Helper()
	for _, marker := range []string{"account_digest", "private-fixture-account", "start_json", "finish_json", "\"generation\"", "private-request-marker", "/private/fixture-transcript"} {
		if bytes.Contains(raw, []byte(marker)) {
			t.Fatalf("output exposed %q: %s", marker, raw)
		}
	}
}

func (f *quotaCLIFixture) startAndComplete() quota.SampleResult {
	f.t.Helper()
	f.sample(quota.SampleStart)
	f.now = f.now.Add(time.Minute)
	f.event("measured", quotaFixtureSession, f.now, 0, 1_000_000, f.eligible(), true)
	f.now = f.now.Add(time.Minute)
	return f.sample(quota.SampleFinish)
}

func TestQuotaSessionReportingCLI(t *testing.T) {
	for _, client := range []quota.Client{quota.ClientCodex, quota.ClientClaude} {
		t.Run(string(client), func(t *testing.T) {
			f := newQuotaCLIFixture(t, client)
			wantCoverage, wantTotal := quota.CoverageFull, "6.000000000"
			if client == quota.ClientCodex {
				f.event("earlier", quotaFixtureSession, f.baseline, 0, 1_000_000, f.eligible(), true)
				wantCoverage, wantTotal = quota.CoveragePortion, "18.000000000"
			}
			if result := f.show().Data.Usage.QuotaValuation; result.Status != quota.MeasurementMissing || len(result.Windows) != 0 {
				t.Fatalf("missing=%+v", result)
			}
			if started := f.sample(quota.SampleStart); started.Valuation != nil || started.Status != quota.MeasurementActive || started.FinishedAt != nil {
				t.Fatalf("start=%+v", started)
			}
			if result := f.show().Data.Usage.QuotaValuation; result.Status != quota.MeasurementActive ||
				len(result.Windows) != 2 || result.Windows[0].Reason != quota.ValuationMeasurementInProgress || result.Windows[0].Before == nil {
				t.Fatalf("active=%+v", result)
			}
			// The first probe's completion is an inclusive boundary.
			f.event("measured", quotaFixtureSession, f.now, 0, 1_000_000, f.eligible(), true)
			f.now = f.now.Add(time.Minute)
			finished := f.sample(quota.SampleFinish)
			valuation := finished.Valuation
			if valuation == nil || valuation.Status != quota.MeasurementComplete || valuation.Coverage != wantCoverage || len(valuation.Windows) != 2 {
				t.Fatalf("finish=%+v", valuation)
			}
			for i, want := range []struct {
				delta float64
				value string
			}{{4, "150.000000000"}, {2, "300.000000000"}} {
				window := valuation.Windows[i]
				if window.Estimate == nil || window.Estimate.Amount != want.value || window.Estimate.BasisAmount != "6.000000000" ||
					window.Estimate.Currency != "USD" || window.ObservedDeltaPP == nil || *window.ObservedDeltaPP != want.delta || window.Reason != "" {
					t.Fatalf("window=%+v estimate=%+v", window, window.Estimate)
				}
				assumption := quota.AssumeSessionUsesSampledAccount
				if client == quota.ClientClaude {
					assumption = quota.AssumeAccountUnverified
					if !slices.Contains(window.Assumptions, quota.AssumeSubscriptionScope) {
						t.Fatalf("unverified Claude subscription omitted assumption: %v", window.Assumptions)
					}
				}
				if !slices.Contains(window.Assumptions, assumption) || !slices.Contains(window.Assumptions, quota.AssumeAllChangeFromSelectedUsage) {
					t.Fatalf("assumptions=%v", window.Assumptions)
				}
			}
			if again := f.sample(quota.SampleFinish); !reflect.DeepEqual(finished, again) || f.probes != 2 {
				t.Fatalf("finish replay=%+v probes=%d", again, f.probes)
			}
			finishText := f.cli("text", "quota", "sample", "finish", "--client", string(client), "--session", quotaFixtureSession)
			for _, want := range []string{"Measurement completed", "USD 150", "USD 300"} {
				if !bytes.Contains(finishText, []byte(want)) {
					t.Fatalf("finish text missing %q: %s", want, finishText)
				}
			}
			before := f.readOnlyState()
			for _, flags := range [][]string{nil, {"--page", "1", "--limit", "1"}, {"--page", "2", "--limit", "1"}} {
				got := f.show(flags...).Data.Usage
				if !reflect.DeepEqual(got.QuotaValuation, valuation) || got.Pricing.APIEquivalent.Amount == nil ||
					*got.Pricing.APIEquivalent.Amount != wantTotal {
					t.Fatalf("page %v quota=%+v pricing=%+v", flags, got.QuotaValuation, got.Pricing.APIEquivalent)
				}
			}
			text := string(f.cli("text", "session", "show", quotaFixtureSession, "--client", string(client), "--tokens"))
			for _, want := range []string{"QUOTA VALUATION", "4 pp observed", "2 pp observed", "USD 150", "USD 300", "if this session used all of it"} {
				if !strings.Contains(text, want) {
					t.Fatalf("text missing %q: %s", want, text)
				}
			}
			if client == quota.ClientCodex && !strings.Contains(text, "Measured portion") {
				t.Fatalf("text missing measured portion: %s", text)
			}
			f.checkViewer("Quota valuation", "4 pp observed", "2 pp observed", "USD 150", "USD 300")
			if after := f.readOnlyState(); before != after || f.probes != 2 {
				t.Fatalf("report changed local database files/read generations or probed: calls=%d", f.probes)
			}
			plain := f.cli("json", "session", "show", quotaFixtureSession, "--client", string(client))
			if bytes.Contains(plain, []byte("quota_valuation")) || bytes.Contains(plain, []byte("\"usage\"")) {
				t.Fatalf("plain show carries Tokens: %s", plain)
			}
			if cancelled := f.sample(quota.SampleCancel); cancelled.Valuation != nil || cancelled.StartedAt != nil || cancelled.FinishedAt != nil || f.probes != 2 {
				t.Fatalf("cancel=%+v probes=%d", cancelled, f.probes)
			}
		})
	}
}

func (f *quotaCLIFixture) checkViewer(wantText ...string) {
	f.t.Helper()
	index, err := store.OpenSessionsReadOnly(f.ctx, f.state)
	if err != nil {
		f.t.Fatal(err)
	}
	defer index.Close()
	core, err := store.OpenReadOnly(f.ctx, f.state)
	if err != nil {
		f.t.Fatal(err)
	}
	defer core.Close()
	load := newSessionViewerLoad(f.ctx, index, session.Metadata{Client: string(f.client), SessionID: quotaFixtureSession}, usage.New(core, f.home))
	viewer := newSessionViewerState(load)
	viewer.section = viewerTokens
	if err := viewer.refresh(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	var frame bytes.Buffer
	if err := renderSessionViewer(&frame, 120, 32, viewer); err != nil {
		f.t.Fatal(err)
	}
	for _, want := range wantText {
		if !strings.Contains(frame.String(), want) {
			f.t.Fatalf("interactive Tokens missing %q: %s", want, &frame)
		}
	}
}

// Compare the committed files and existing WAL contents. Read-only open may
// create or update SQLite shared-memory bookkeeping.
func (f *quotaCLIFixture) readOnlyState() [4][32]byte {
	f.t.Helper()
	var state [4][32]byte
	for i, name := range []string{"agentdeck.sqlite3", "agentdeck.sqlite3-wal", "sessions.sqlite3", "sessions.sqlite3-wal"} {
		data, err := os.ReadFile(filepath.Join(f.state, name))
		if err != nil && !(strings.HasSuffix(name, "-wal") && errors.Is(err, os.ErrNotExist)) {
			f.t.Fatal(err)
		}
		// A read-only pooled SQLite connection can create an empty WAL
		// sidecar. Only its bytes are relevant to persistence.
		state[i] = sha256.Sum256(data)
	}
	return state
}

func TestQuotaSessionReportLegacySchemaIsReadOnly(t *testing.T) {
	for _, schema := range []int{30, 31, 35, store.QuotaIdentitySchemaVersion, store.PricingFactsSchemaVersion} {
		t.Run(fmt.Sprint(schema), func(t *testing.T) {
			f := newQuotaCLIFixture(t, quota.ClientCodex)
			f.event("legacy", quotaFixtureSession, f.baseline, 0, 1_000_000, f.eligible(), true)
			f.exec(`DROP TRIGGER quota_samples_on_session_delete; DROP TRIGGER quota_samples_on_provider_selection;
				DROP TRIGGER quota_samples_on_observed_provider; DROP TABLE quota_session_samples;`)
			if schema < store.PricingFactsSchemaVersion {
				f.exec(`DROP TABLE usage_request_facts; DROP TABLE price_rule_sets`)
			}
			f.exec(`UPDATE schema_metadata SET version=?`, schema)
			if schema < 32 {
				// Schema 32 introduced the projection revision. The legacy
				// Tokens reader must work without this table.
				f.exec(`DROP TABLE usage_derived_metadata`)
				index, err := store.OpenSessions(f.ctx, f.state)
				if err != nil {
					t.Fatal(err)
				}
				// v0.6.5 has the document and metadata index, without the
				// newer serving storage or source identity index.
				_, err = index.DB.ExecContext(f.ctx, `
					DROP TABLE source_domain_checkpoints;
					DROP TABLE domain_coverage_heads;
					DROP TABLE inventory_runs;
					DROP TABLE serving_generation;
					DROP INDEX session_sources_identity;`)
				closeErr := index.Close()
				if err := errors.Join(err, closeErr); err != nil {
					t.Fatal(err)
				}
			}
			before := f.readOnlyState()
			report := f.show().Data
			result := report.Usage.QuotaValuation
			if result.Status != quota.MeasurementMissing || result.Coverage != quota.CoverageUnknown || len(result.Windows) != 0 {
				t.Fatalf("legacy result=%+v", result)
			}
			if report.Usage.Tokens["output_tokens"] != 1_000_000 || len(report.Invocations) != 1 || report.Invocations[0].Model != f.model {
				t.Fatalf("legacy Tokens report=%+v", report)
			}
			text := string(f.cli("text", "session", "show", quotaFixtureSession, "--client", string(f.client), "--tokens"))
			for _, want := range []string{f.model, "No saved measurement"} {
				if !strings.Contains(text, want) {
					t.Fatalf("legacy text missing %q: %s", want, text)
				}
			}
			f.checkViewer(f.model, "Quota valuation", "No saved measurement")
			if actual, err := f.core.SchemaVersion(f.ctx); err != nil || actual != schema || f.probes != 0 || before != f.readOnlyState() {
				t.Fatalf("legacy schema=%d want=%d err=%v probes=%d", actual, schema, err, f.probes)
			}
		})
	}
}

func TestQuotaSessionReportExcludesPaidAndFreeActivities(t *testing.T) {
	f := newQuotaCLIFixture(t, quota.ClientCodex)
	f.startAndComplete()
	at := f.now.Add(-time.Minute)
	api := f.eligible()
	api.Channel = "api_key"
	thirdParty := f.eligible()
	thirdParty.Channel = "third_party"
	free := f.eligible()
	free.Activity = "guardian_approval"
	free.Eligibility = []string{"chatgpt_sign_in"}
	f.event("selected-api", quotaFixtureSession, at, 0, 1_000_000, api, true)
	f.event("selected-third-party", quotaFixtureSession, at, 0, 1_000_000, thirdParty, true)
	f.event("selected-free", quotaFixtureSession, at, 0, 1_000_000, free, true)
	f.event("other-api", "other-session", at, 0, 1_000_000, api, true)
	f.event("other-third-party", "other-session", at, 0, 1_000_000, thirdParty, true)
	f.event("other-free", "other-session", at, 0, 1_000_000, free, true)

	got := f.show().Data
	if total := got.Usage.Pricing.APIEquivalent.Amount; total == nil || *total != "24.000000000" {
		t.Fatalf("full session amount=%+v", got.Usage.Pricing.APIEquivalent)
	}
	for _, window := range got.Usage.QuotaValuation.Windows {
		if window.Estimate == nil || window.Estimate.BasisAmount != "6.000000000" {
			t.Fatalf("known exclusions changed the window basis: %+v", window)
		}
	}
	competing := f.eligible()
	competing.Channel = ""
	f.event("other-unknown", "other-session", at, 0, 1_000_000, competing, true)
	result := f.show().Data.Usage.QuotaValuation
	for _, window := range result.Windows {
		if window.Estimate != nil || window.Reason != quota.ValuationOtherLocalUsage || window.ObservedDeltaPP == nil {
			t.Fatalf("competing local usage=%+v", window)
		}
	}
	text := f.cli("text", "session", "show", quotaFixtureSession, "--tokens")
	if !bytes.Contains(text, []byte("Other local usage detected")) {
		t.Fatalf("local usage text=%s", text)
	}
}

func TestQuotaSessionReportPreservesObservedWindowsWithLimits(t *testing.T) {
	for _, tc := range []struct {
		name        string
		adjustProbe func(*quotaCLIFixture)
		addEvidence func(*quotaCLIFixture)
		reasons     []quota.ValuationReason
	}{
		{
			name:        "distinct pools and unverified membership",
			adjustProbe: func(f *quotaCLIFixture) { f.extraPool = true },
			reasons:     []quota.ValuationReason{quota.ValuationPoolUnresolved, quota.ValuationPoolUnresolved, quota.ValuationPoolUnresolved},
		},
		{
			name:        "short reset with usable weekly window",
			adjustProbe: func(f *quotaCLIFixture) { f.resetShort = true },
			reasons:     []quota.ValuationReason{quota.ValuationResetDetected, ""},
		},
		{
			name: "missing finish window",
			addEvidence: func(f *quotaCLIFixture) {
				f.exec(`UPDATE quota_session_samples SET finish_json=json_remove(finish_json,'$.windows[0]') WHERE slot='latest'`)
			},
			reasons: []quota.ValuationReason{quota.ValuationWindowMissing, ""},
		},
		{
			name: "unavailable Codex account",
			addEvidence: func(f *quotaCLIFixture) {
				f.exec(`UPDATE quota_session_samples SET start_json=json_remove(start_json,'$.account_digest') WHERE slot='latest'`)
			},
			reasons: []quota.ValuationReason{quota.ValuationWindowUnverified, quota.ValuationWindowUnverified},
		},
		{
			name: "pending usage within the captured interval",
			addEvidence: func(f *quotaCLIFixture) {
				f.exec(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,source_path)
					VALUES (?, ?, 1, ?, 'pending', 'default_work', 'fixture')`, f.client, quotaFixtureSession, f.now.Add(-time.Second).Format(time.RFC3339Nano))
			},
			reasons: []quota.ValuationReason{quota.ValuationUnsettled, quota.ValuationUnsettled},
		},
		{
			name: "pending work after capture",
			addEvidence: func(f *quotaCLIFixture) {
				f.exec(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,source_path)
					VALUES (?, ?, 1, ?, 'pending', 'default_work', 'fixture')`, f.client, quotaFixtureSession, f.now.Add(time.Second).Format(time.RFC3339Nano))
			},
			reasons: []quota.ValuationReason{"", ""},
		},
		{
			name: "exact finish boundary is outside the amount bracket",
			addEvidence: func(f *quotaCLIFixture) {
				f.event("at-finish", quotaFixtureSession, f.now.Add(-time.Second), 0, 1_000_000, f.eligible(), true)
			},
			reasons: []quota.ValuationReason{quota.ValuationUnsettled, quota.ValuationUnsettled},
		},
		{
			name: "partial eligible API price",
			addEvidence: func(f *quotaCLIFixture) {
				f.event("partial", quotaFixtureSession, f.now.Add(-time.Minute), 1_000_000, 1_000_000, f.eligible(), false)
			},
			reasons: []quota.ValuationReason{quota.ValuationPricingIncomplete, quota.ValuationPricingIncomplete},
		},
		{
			name: "unresolved enterprise billing",
			addEvidence: func(f *quotaCLIFixture) {
				mixed := f.eligible()
				mixed.Channel, mixed.Eligibility = "enterprise_usd", nil
				f.event("mixed", quotaFixtureSession, f.now.Add(-time.Minute), 0, 1_000_000, mixed, true)
			},
			reasons: []quota.ValuationReason{quota.ValuationMixedBilling, quota.ValuationMixedBilling},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newQuotaCLIFixture(t, quota.ClientCodex)
			if tc.adjustProbe != nil {
				tc.adjustProbe(f)
			}
			f.startAndComplete()
			if tc.addEvidence != nil {
				tc.addEvidence(f)
			}
			report := f.show().Data.Usage.QuotaValuation
			if len(report.Windows) != len(tc.reasons) {
				t.Fatalf("windows=%+v", report.Windows)
			}
			for i, reason := range tc.reasons {
				win := report.Windows[i]
				if win.Reason != reason || (win.Estimate != nil) != (reason == "") || win.Before == nil ||
					(reason != quota.ValuationWindowMissing && win.After == nil) {
					t.Fatalf("window %d=%+v want reason %q", i, win, reason)
				}
			}
			if tc.reasons[0] == quota.ValuationPricingIncomplete {
				text := f.cli("text", "session", "show", quotaFixtureSession, "--tokens")
				if !bytes.Contains(text, []byte("USD 8 known")) {
					t.Fatalf("partial quota basis missing from text: %s", text)
				}
			}
			if f.probes != 2 {
				t.Fatalf("report probed: %d calls", f.probes)
			}
		})
	}
}

func TestCompletedQuotaReportRequiresReturnedGeneration(t *testing.T) {
	f := newQuotaCLIFixture(t, quota.ClientCodex)
	completed := f.startAndComplete()
	actual, err := f.probeService(f.state, "").SampleQuota(f.ctx, f.core, quotaSampleLockTimeout, quota.SampleFinish, f.client, quotaFixtureSession)
	if err != nil || actual.Completion() == nil {
		t.Fatalf("finish replay=%+v, %v", actual, err)
	}
	f.now = f.now.Add(time.Minute)
	f.sample(quota.SampleStart)
	if got := f.show().Data.Usage.QuotaValuation; got.Status != quota.MeasurementActive {
		t.Fatalf("ordinary report did not select the matching active sample: %+v", got)
	}
	report := usage.New(f.core, "")
	result, err := report.CompletedSampleValuation(f.ctx, f.client, quotaFixtureSession, actual.Completion())
	gotWire, _ := json.Marshal(result)
	wantWire, _ := json.Marshal(completed.Valuation)
	if err != nil || !bytes.Equal(gotWire, wantWire) {
		t.Fatalf("completed report was influenced by the newer active slot: %+v (%v)", result, err)
	}

	// An intervening writer can replace latest between mutation and report.
	f.exec(`UPDATE quota_session_samples SET generation='fixture-replacement' WHERE slot='latest'`)
	result, err = report.CompletedSampleValuation(f.ctx, f.client, quotaFixtureSession, actual.Completion())
	var missing *quota.SampleError
	if result != nil || !errors.As(err, &missing) || missing.Code != quota.SampleMissing {
		t.Fatalf("superseded completion borrowed another generation: %+v (%v)", result, err)
	}
}
