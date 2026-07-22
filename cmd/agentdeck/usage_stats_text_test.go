package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsageStatsBalancedTextLayout(t *testing.T) {
	report := usageStatsTextFixture()
	for _, width := range []int{48, 72, 100, 140} {
		t.Run(groupedInt(int64(width))+" columns", func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			for _, want := range []string{
				"📊 USAGE STATS · LAST 7 DAYS",
				"TOKENS", "366.9M",
				"COST", "$316.83", "KNOWN",
				"SESSIONS", "34",
				"🗓 TREND · TOKENS",
				"🤖 MODELS", "claude-opus-4-8", "codex-auto-review",
				"CLIENTS", "Claude", "Codex",
				"PROVIDERS", "Claude/relay", "Codex/official",
				"CACHE HIT RATE", "MODEL Claude/claude-opus-4-8", "SESSION Codex/codex-session", "--activity",
				"AVG COST", "PEAK", "PRICED  87.69%",
				"▦ ACTIVITY BY WEEKDAY / HOUR · TOKENS",
				"LESS  · ░ ▒ ▓ █  MORE",
				"UNPRICED MODELS", "Claude/claude-fable-5", "output",
			} {
				if !strings.Contains(text, want) {
					t.Fatalf("%d-column stats missing %q:\n%s", width, want, text)
				}
			}
			for _, noisy := range []string{"366924859", "316.832730700", "(partial)"} {
				if strings.Contains(text, noisy) {
					t.Fatalf("%d-column stats retained noisy value %q:\n%s", width, noisy, text)
				}
			}
			if strings.Contains(text, "Known priced subtotal") {
				t.Fatalf("%d-column retained generic partial-cost footnote:\n%s", width, text)
			}
			for lineNumber, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
				if got := statsVisibleWidth(line); got > width {
					t.Fatalf("%d-column line %d width = %d:\n%s", width, lineNumber+1, got, line)
				}
			}
		})
	}
}

func TestUsageStatsWideLayoutAndColorAreDeterministic(t *testing.T) {
	report := usageStatsTextFixture()
	var plain, colored bytes.Buffer
	if err := renderUsageStatsWithOptions(&plain, report, usageTextRenderOptions{width: 140}); err != nil {
		t.Fatal(err)
	}
	if err := renderUsageStatsWithOptions(&colored, report, usageTextRenderOptions{width: 140, color: true}); err != nil {
		t.Fatal(err)
	}
	plainText, coloredText := plain.String(), colored.String()
	if strings.Contains(plainText, "\x1b[") || !strings.Contains(coloredText, "\x1b[") {
		t.Fatalf("ANSI color plain=%q colored=%q", plainText, coloredText)
	}
	if stripStatsANSI(coloredText) != plainText {
		t.Fatal("colored stats changed layout or content")
	}
	var paired bool
	for _, line := range strings.Split(plainText, "\n") {
		if strings.Contains(line, "TREND · TOKENS") && strings.Contains(line, "🤖 MODELS") {
			paired = true
		}
		if statsVisibleWidth(line) > 140 {
			t.Fatalf("wide line overflowed: %q", line)
		}
	}
	if !paired {
		t.Fatalf("wide layout did not pair trend and ranking:\n%s", plainText)
	}
}

// usageStatsJSONReport exercises the real `--format json` output path
// (writeUsageEnvelope -> json.NewEncoder) on the same report value used for
// the paired text assertion, so JSON completeness is proven against actual
// serialized output rather than the caller-side Go slice length.
func usageStatsJSONReport(t *testing.T, report usage.StatsReport) usage.StatsReport {
	t.Helper()
	var buf bytes.Buffer
	if err := writeUsageEnvelope(&buf, "json", "usage.stats", report, false, nil, false, usageTextRenderOptions{}); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data usage.StatsReport `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode usage stats JSON: %v\n%s", err, buf.String())
	}
	return envelope.Data
}

func TestUsageStatsCacheSessionsAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.CacheSessions = make([]usage.StatsCacheSession, 0, 11)
	rate := "50.00"
	for index := 0; index < 11; index++ {
		report.CacheSessions = append(report.CacheSessions, usage.StatsCacheSession{Client: "codex", SessionID: fmt.Sprintf("session-%02d", index), Models: []string{"gpt-5.6-sol"}, CachedReadTokens: int64(100 - index), LogicalInputTokens: 200, CacheHitRate: &rate, DetailCommand: fmt.Sprintf("agentdeck session show session-%02d --client codex --activity", index)})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 48}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "SESSION Codex/") != 10 || !strings.Contains(text, "+1 more cache sessions in JSON") || len(report.CacheSessions) != 11 {
		t.Fatalf("cache session text cap =\n%s", text)
	}
	assertUsageStatsWidth(t, text, 48)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.CacheSessions) != 11 || jsonReport.CacheSessions[10].SessionID != "session-10" {
		t.Fatalf("cache sessions JSON incomplete: %d rows, want 11 including text-omitted tail\n%#v", len(jsonReport.CacheSessions), jsonReport.CacheSessions)
	}
}

func TestUsageStatsModelsAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.Models = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Models = append(report.Models, usage.StatsDimension{Name: fmt.Sprintf("model-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "model-") != 8 || !strings.Contains(text, "+1 more models in JSON") || len(report.Models) != 9 {
		t.Fatalf("models text cap =\n%s", text)
	}
	for index := 0; index < 8; index++ {
		if name := fmt.Sprintf("model-%02d", index); !strings.Contains(text, name) {
			t.Fatalf("models text missing retained-order row %q:\n%s", name, text)
		}
	}
	if strings.Contains(text, "model-08") {
		t.Fatalf("models text retained the row past the cap boundary:\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Models) != 9 || jsonReport.Models[8].Name != "model-08" {
		t.Fatalf("models JSON incomplete: %d rows, want 9 including text-omitted tail\n%#v", len(jsonReport.Models), jsonReport.Models)
	}
}

func TestUsageStatsProvidersAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.Providers = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Providers = append(report.Providers, usage.StatsDimension{Name: fmt.Sprintf("prov-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "prov-") != 8 || !strings.Contains(text, "+1 more providers in JSON") || len(report.Providers) != 9 {
		t.Fatalf("providers text cap =\n%s", text)
	}
	for index := 0; index < 8; index++ {
		if name := fmt.Sprintf("prov-%02d", index); !strings.Contains(text, name) {
			t.Fatalf("providers text missing retained-order row %q:\n%s", name, text)
		}
	}
	if strings.Contains(text, "prov-08") {
		t.Fatalf("providers text retained the row past the cap boundary:\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Providers) != 9 || jsonReport.Providers[8].Name != "prov-08" {
		t.Fatalf("providers JSON incomplete: %d rows, want 9 including text-omitted tail\n%#v", len(jsonReport.Providers), jsonReport.Providers)
	}
}

func TestUsageStatsUnpricedModelsAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.UnpricedModels = make([]usage.StatsUnpricedModel, 0, 13)
	for index := 0; index < 13; index++ {
		report.UnpricedModels = append(report.UnpricedModels, usage.StatsUnpricedModel{Client: "codex", Model: fmt.Sprintf("unpriced-%02d", index), Components: []string{"output"}})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "unpriced-") != 12 || !strings.Contains(text, "+1 more unpriced models in JSON") || len(report.UnpricedModels) != 13 {
		t.Fatalf("unpriced models text cap =\n%s", text)
	}
	for index := 0; index < 12; index++ {
		if name := fmt.Sprintf("unpriced-%02d", index); !strings.Contains(text, name) {
			t.Fatalf("unpriced models text missing retained-order row %q:\n%s", name, text)
		}
	}
	if strings.Contains(text, "unpriced-12") {
		t.Fatalf("unpriced models text retained the row past the cap boundary:\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.UnpricedModels) != 13 || jsonReport.UnpricedModels[12].Model != "unpriced-12" {
		t.Fatalf("unpriced models JSON incomplete: %d rows, want 13 including text-omitted tail\n%#v", len(jsonReport.UnpricedModels), jsonReport.UnpricedModels)
	}
}

func TestUsageStatsModelCacheRowsAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	rate := "50.00"
	report.Models = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Models = append(report.Models, usage.StatsDimension{Name: fmt.Sprintf("cache-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share, CachedReadTokens: int64(100 - index), LogicalInputTokens: 200, CacheHitRate: &rate})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "MODEL Codex/cache-") != 8 || !strings.Contains(text, "+1 more cache models in JSON") || len(report.Models) != 9 {
		t.Fatalf("model cache text cap =\n%s", text)
	}
	for index := 0; index < 8; index++ {
		if name := fmt.Sprintf("MODEL Codex/cache-%02d", index); !strings.Contains(text, name) {
			t.Fatalf("model cache text missing retained-order row %q:\n%s", name, text)
		}
	}
	if strings.Contains(text, "MODEL Codex/cache-08") {
		t.Fatalf("model cache text retained the row past the cap boundary:\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Models) != 9 || jsonReport.Models[8].Name != "cache-08" {
		t.Fatalf("model cache JSON incomplete: %d rows, want 9 including text-omitted tail\n%#v", len(jsonReport.Models), jsonReport.Models)
	}
}

// TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText covers the
// `--group-by hour --period 30d` shape that produced the 709-bucket, 822-line
// wall in the baseline: an hour-grouped trend spanning multiple days.
func TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.GroupBy = "hour"
	const total = 60
	base := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	report.Buckets = make([]usage.StatsBucket, 0, total)
	for index := 0; index < total; index++ {
		start := base.Add(time.Duration(index) * time.Hour)
		report.Buckets = append(report.Buckets, usage.StatsBucket{Start: start.Format(time.RFC3339Nano), Tokens: int64(1000 + index), Sessions: 1, KnownMetricValue: strconv.FormatInt(int64(1000+index), 10)})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "+12 earlier buckets in JSON") {
		t.Fatalf("trend text missing overflow note:\n%s", text)
	}
	if got := strings.Count(text, "Jun "); got != statsTrendCap {
		t.Fatalf("trend bar-row count = %d, want bounded at %d:\n%s", got, statsTrendCap, text)
	}
	// omitted = total - statsTrendCap = 12: buckets 0..11 must be gone, the
	// most recent contiguous window 12..59 must all still be present in order.
	for index := 0; index < total-statsTrendCap; index++ {
		if label := base.Add(time.Duration(index) * time.Hour).Format("Jan 02 15:04"); strings.Contains(text, label) {
			t.Fatalf("trend text retained an earlier bucket %q that should have been windowed out:\n%s", label, text)
		}
	}
	previousPosition := -1
	for index := total - statsTrendCap; index < total; index++ {
		label := base.Add(time.Duration(index) * time.Hour).Format("Jan 02 15:04")
		position := strings.Index(text, label)
		if position < 0 {
			t.Fatalf("trend text missing retained recent bucket %q:\n%s", label, text)
		}
		if position <= previousPosition {
			t.Fatalf("trend text reordered retained bucket %q at position %d after position %d:\n%s", label, position, previousPosition, text)
		}
		previousPosition = position
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Buckets) != total || jsonReport.Buckets[0].Start != report.Buckets[0].Start {
		t.Fatalf("trend JSON incomplete: %d buckets, want %d including the text-omitted earliest bucket\n%#v", len(jsonReport.Buckets), total, jsonReport.Buckets)
	}
}

// TestUsageStatsTopFlagDoesNotAffectTrendOrClients closes a gap the prior
// top-flag tests left open: they never combined a positive --top with more
// than statsTrendCap buckets or more than a handful of clients, so an
// implementation that mistakenly ran capFor over TREND or CLIENTS could still
// pass. This mirrors TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText's
// bucket construction and assertions verbatim (same window, same overflow
// note, same retained-order check), but renders with --top 3 and a
// five-client fixture, so both negative contracts are proven together.
func TestUsageStatsTopFlagDoesNotAffectTrendOrClients(t *testing.T) {
	report := usageStatsTextFixture()
	report.GroupBy = "hour"
	const totalBuckets = 60
	base := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	report.Buckets = make([]usage.StatsBucket, 0, totalBuckets)
	for index := 0; index < totalBuckets; index++ {
		start := base.Add(time.Duration(index) * time.Hour)
		report.Buckets = append(report.Buckets, usage.StatsBucket{Start: start.Format(time.RFC3339Nano), Tokens: int64(1000 + index), Sessions: 1, KnownMetricValue: strconv.FormatInt(int64(1000+index), 10)})
	}
	const totalClients = 5
	report.Clients = make([]usage.StatsDimension, 0, totalClients)
	for index := 0; index < totalClients; index++ {
		report.Clients = append(report.Clients, usage.StatsDimension{Name: fmt.Sprintf("client-%02d", index), KnownShare: "20.00"})
	}
	top := 3
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100, top: &top}); err != nil {
		t.Fatal(err)
	}
	text := output.String()

	// CLIENTS: --top 3 must not truncate a 5-client list.
	for index := 0; index < totalClients; index++ {
		if name := fmt.Sprintf("Client-%02d", index); !strings.Contains(text, name) {
			t.Fatalf("--top 3 dropped client %q that CLIENTS must never cap:\n%s", name, text)
		}
	}

	// TREND: unaffected by --top, same as with no --top at all.
	if !strings.Contains(text, "+12 earlier buckets in JSON") {
		t.Fatalf("--top 3 changed trend's own overflow note:\n%s", text)
	}
	if got := strings.Count(text, "Jun "); got != statsTrendCap {
		t.Fatalf("--top 3 changed trend bar-row count = %d, want bounded at %d:\n%s", got, statsTrendCap, text)
	}
	for index := 0; index < totalBuckets-statsTrendCap; index++ {
		if label := base.Add(time.Duration(index) * time.Hour).Format("Jan 02 15:04"); strings.Contains(text, label) {
			t.Fatalf("--top 3 retained an earlier bucket %q that trend-cap should still windowed out:\n%s", label, text)
		}
	}
	previousPosition := -1
	for index := totalBuckets - statsTrendCap; index < totalBuckets; index++ {
		label := base.Add(time.Duration(index) * time.Hour).Format("Jan 02 15:04")
		position := strings.Index(text, label)
		if position < 0 {
			t.Fatalf("--top 3 dropped retained recent bucket %q:\n%s", label, text)
		}
		if position <= previousPosition {
			t.Fatalf("--top 3 reordered retained bucket %q at position %d after position %d:\n%s", label, position, previousPosition, text)
		}
		previousPosition = position
	}
	assertUsageStatsWidth(t, text, 100)
}

// topFlagFixture builds a report whose MODELS, PROVIDERS, UNPRICED MODELS,
// and cache-session lists all exceed their shared-topn defaults (8, 8, 12,
// 10), so --top's effect on each is independently observable via disjoint
// name prefixes. Models deliberately carry no cache data so the MODELS
// section's bare "model-NN" names cannot also be counted inside the
// per-model CACHE section (that cap gets its own fixture and test below,
// mirroring how shared-topn kept those two fixtures separate). TREND and
// CLIENTS are left at the small fixture defaults since --top must never
// affect them.
func topFlagFixture() usage.StatsReport {
	report := usageStatsTextFixture()
	rate := "50.00"
	report.Models = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Models = append(report.Models, usage.StatsDimension{Name: fmt.Sprintf("model-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share})
	}
	report.Providers = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Providers = append(report.Providers, usage.StatsDimension{Name: fmt.Sprintf("prov-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share})
	}
	report.UnpricedModels = make([]usage.StatsUnpricedModel, 0, 13)
	for index := 0; index < 13; index++ {
		report.UnpricedModels = append(report.UnpricedModels, usage.StatsUnpricedModel{Client: "codex", Model: fmt.Sprintf("unpriced-%02d", index), Components: []string{"output"}})
	}
	report.CacheSessions = make([]usage.StatsCacheSession, 0, 11)
	for index := 0; index < 11; index++ {
		report.CacheSessions = append(report.CacheSessions, usage.StatsCacheSession{Client: "codex", SessionID: fmt.Sprintf("session-%02d", index), Models: []string{"gpt-5.6-sol"}, CachedReadTokens: int64(100 - index), LogicalInputTokens: 200, CacheHitRate: &rate, DetailCommand: fmt.Sprintf("agentdeck session show session-%02d --client codex --activity", index)})
	}
	return report
}

// topFlagModelCacheFixture isolates the per-model CACHE cap from the MODELS
// cap: unlike topFlagFixture's models, these carry cache data, and "cache-NN"
// is a name prefix distinct from "model-NN"/"prov-NN"/"unpriced-NN" above.
func topFlagModelCacheFixture() usage.StatsReport {
	report := usageStatsTextFixture()
	rate := "50.00"
	report.Models = make([]usage.StatsDimension, 0, 9)
	for index := 0; index < 9; index++ {
		share := strconv.FormatFloat(10-float64(index)*0.5, 'f', 2, 64)
		report.Models = append(report.Models, usage.StatsDimension{Name: fmt.Sprintf("cache-%02d", index), Client: "codex", Tokens: int64(1000 - index), Sessions: int64(index + 1), KnownShare: share, CachedReadTokens: int64(100 - index), LogicalInputTokens: 200, CacheHitRate: &rate})
	}
	return report
}

func TestUsageStatsTopFlagOmittedKeepsSharedTopNDefaults(t *testing.T) {
	report := topFlagFixture()
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "model-") != 8 || strings.Count(text, "prov-") != 8 || strings.Count(text, "unpriced-") != 12 || strings.Count(text, "SESSION Codex/") != 10 {
		t.Fatalf("unset --top did not keep shared-topn defaults:\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)

	cacheReport := topFlagModelCacheFixture()
	var cacheOutput bytes.Buffer
	if err := renderUsageStatsWithOptions(&cacheOutput, cacheReport, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	cacheText := cacheOutput.String()
	if strings.Count(cacheText, "MODEL Codex/cache-") != 8 || !strings.Contains(cacheText, "+1 more cache models in JSON") {
		t.Fatalf("unset --top did not keep per-model cache default:\n%s", cacheText)
	}
}

func TestUsageStatsTopFlagPositiveOverridesAllListedCaps(t *testing.T) {
	top := 3
	report := topFlagFixture()
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100, top: &top}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "model-") != 3 || !strings.Contains(text, "+6 more models in JSON") {
		t.Fatalf("--top 3 models cap =\n%s", text)
	}
	if strings.Count(text, "prov-") != 3 || !strings.Contains(text, "+6 more providers in JSON") {
		t.Fatalf("--top 3 providers cap =\n%s", text)
	}
	if strings.Count(text, "unpriced-") != 3 || !strings.Contains(text, "+10 more unpriced models in JSON") {
		t.Fatalf("--top 3 unpriced cap =\n%s", text)
	}
	if strings.Count(text, "SESSION Codex/") != 3 || !strings.Contains(text, "+8 more cache sessions in JSON") {
		t.Fatalf("--top 3 cache sessions cap =\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Models) != 9 || len(jsonReport.Providers) != 9 || len(jsonReport.UnpricedModels) != 13 || len(jsonReport.CacheSessions) != 11 {
		t.Fatalf("--top 3 JSON incomplete: %#v", jsonReport)
	}

	cacheReport := topFlagModelCacheFixture()
	var cacheOutput bytes.Buffer
	if err := renderUsageStatsWithOptions(&cacheOutput, cacheReport, usageTextRenderOptions{width: 100, top: &top}); err != nil {
		t.Fatal(err)
	}
	cacheText := cacheOutput.String()
	if strings.Count(cacheText, "MODEL Codex/cache-") != 3 || !strings.Contains(cacheText, "+6 more cache models in JSON") {
		t.Fatalf("--top 3 per-model cache cap =\n%s", cacheText)
	}
	cacheJSONReport := usageStatsJSONReport(t, cacheReport)
	if len(cacheJSONReport.Models) != 9 {
		t.Fatalf("--top 3 per-model cache JSON incomplete: %#v", cacheJSONReport.Models)
	}
}

func TestUsageStatsTopFlagExplicitZeroRestoresFullText(t *testing.T) {
	top := 0
	report := topFlagFixture()
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100, top: &top}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "model-") != 9 || strings.Contains(text, "more models in JSON") {
		t.Fatalf("--top 0 models cap =\n%s", text)
	}
	if strings.Count(text, "prov-") != 9 || strings.Contains(text, "more providers in JSON") {
		t.Fatalf("--top 0 providers cap =\n%s", text)
	}
	if strings.Count(text, "unpriced-") != 13 || strings.Contains(text, "more unpriced models in JSON") {
		t.Fatalf("--top 0 unpriced cap =\n%s", text)
	}
	if strings.Count(text, "SESSION Codex/") != 11 || strings.Contains(text, "more cache sessions in JSON") {
		t.Fatalf("--top 0 cache sessions cap =\n%s", text)
	}
	assertUsageStatsWidth(t, text, 100)

	cacheReport := topFlagModelCacheFixture()
	var cacheOutput bytes.Buffer
	if err := renderUsageStatsWithOptions(&cacheOutput, cacheReport, usageTextRenderOptions{width: 100, top: &top}); err != nil {
		t.Fatal(err)
	}
	cacheText := cacheOutput.String()
	if strings.Count(cacheText, "MODEL Codex/cache-") != 9 || strings.Contains(cacheText, "more cache models in JSON") {
		t.Fatalf("--top 0 per-model cache cap =\n%s", cacheText)
	}
}

// modelDetailBlock returns every consecutive non-blank line following the
// bar row that starts with barPrefix, up to (not including) the next blank
// line. statsWrap silently wraps an over-width line onto a visually similar
// second line with no indentation or other marker, so a test must check the
// full block length — not just inspect the first line's content — to tell a
// genuinely single-line compacted detail from a two-line wrap whose first
// fragment happens to look the same.
func modelDetailBlock(t *testing.T, text, barPrefix string) []string {
	t.Helper()
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, barPrefix) {
			continue
		}
		var block []string
		for _, follow := range lines[index+1:] {
			if follow == "" {
				return block
			}
			block = append(block, follow)
		}
		t.Fatalf("bar row %q detail block ran off the end of output:\n%s", barPrefix, text)
	}
	t.Fatalf("no bar row starting with %q:\n%s", barPrefix, text)
	return nil
}

// modelDetailLine is modelDetailBlock for the common case where the caller
// only wants to assert the detail is exactly one line and inspect it.
func modelDetailLine(t *testing.T, text, barPrefix string) string {
	t.Helper()
	block := modelDetailBlock(t, text, barPrefix)
	if len(block) != 1 {
		t.Fatalf("bar row %q detail was %d lines, want exactly 1 (wrapped instead of compacted): %#v", barPrefix, len(block), block)
	}
	return block[0]
}

func TestUsageStatsModelDetailCompactsSecondaryFieldsByWidth(t *testing.T) {
	report := usageStatsTextFixture()
	rate := "88.14"
	report.Metric = "tokens"
	report.Models = []usage.StatsDimension{{
		Name: "wide-model", Client: "codex", Tokens: 84_900_000, Sessions: 69,
		KnownShare: "1.6", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
		Activity: &usage.StatsModelActivity{ToolCalls: 57},
	}}
	// Full detail at these values is 82 chars: high-value (58) + " · 57 tools"
	// (11) + " · 88.14% hit" (13). This is the exact example from the plan's
	// root-cause note (`84.9M tokens · 1.6% · unavailable · UNPRICED · 69
	// sessions · 57 tools · 88.14% hit`), just with a priced-but-cost-nil
	// model swapped for an explicitly unavailable one is not needed here:
	// tokens metric never marks share "unavailable", so cost is what's
	// unavailable (no ProviderCost/KnownProviderCost set -> UNPRICED).
	for _, tc := range []struct {
		name         string
		width        int
		wantTools    bool
		wantCacheHit bool
	}{
		{name: "width 80 drops only cache-hit", width: 80, wantTools: true, wantCacheHit: false},
		{name: "width 100 keeps everything", width: 100, wantTools: true, wantCacheHit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: tc.width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			// modelDetailLine itself requires exactly one line, so a
			// regression that stops compacting (falls back to statsWrap's
			// two-line wrap) fails here regardless of the two field checks
			// below.
			detail := modelDetailLine(t, text, "wide-model")
			if !strings.Contains(detail, "84.9M tokens") || !strings.Contains(detail, "1.6%") || !strings.Contains(detail, "UNPRICED") || !strings.Contains(detail, "69 sessions") {
				t.Fatalf("width %d dropped a high-value field:\n%s", tc.width, detail)
			}
			if strings.Contains(detail, "tools") != tc.wantTools {
				t.Fatalf("width %d tools presence = %v, want %v:\n%s", tc.width, strings.Contains(detail, "tools"), tc.wantTools, detail)
			}
			if strings.Contains(detail, "hit") != tc.wantCacheHit {
				t.Fatalf("width %d cache-hit presence = %v, want %v:\n%s", tc.width, strings.Contains(detail, "hit"), tc.wantCacheHit, detail)
			}
			if tc.wantCacheHit && !strings.Contains(detail, "88.14% hit") {
				t.Fatalf("cache-hit percentage was rounded or reformatted, want unrounded 88.14%%:\n%s", detail)
			}
			assertUsageStatsWidth(t, text, tc.width)
			jsonReport := usageStatsJSONReport(t, report)
			if len(jsonReport.Models) != 1 || jsonReport.Models[0].Activity == nil || jsonReport.Models[0].Activity.ToolCalls != 57 || jsonReport.Models[0].CacheHitRate == nil || *jsonReport.Models[0].CacheHitRate != "88.14" {
				t.Fatalf("width %d JSON lost a field text compaction trimmed: %#v", tc.width, jsonReport.Models)
			}
		})
	}
}

func TestUsageStatsModelDetailDropsToolsWhenHighValueAloneFillsTheLine(t *testing.T) {
	report := usageStatsTextFixture()
	report.Metric = "cost"
	report.Models = []usage.StatsDimension{{
		Name: "huge-model", Client: "codex", Tokens: 999_900_000_000_000, Sessions: 999_999_999,
		KnownShare: "0", Coverage: "0", Activity: &usage.StatsModelActivity{ToolCalls: 999},
	}}
	// metric=cost with no known cost/coverage makes both share and cost render
	// "unavailable" (11 chars each). High-value alone is 75 chars; adding
	// " · 999 tools" (12) would push it to 87, over width 80, so even tools
	// must be dropped, leaving only the always-present high-value fields.
	width := 80
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	detail := modelDetailLine(t, text, "huge-model")
	if !strings.Contains(detail, "999,999,999 sessions") || !strings.Contains(detail, "UNPRICED") {
		t.Fatalf("width %d dropped a high-value field:\n%s", width, detail)
	}
	if strings.Contains(detail, "tools") {
		t.Fatalf("width %d kept tools even though high-value fields alone fill the line:\n%s", width, detail)
	}
	if statsVisibleWidth(detail) > width {
		t.Fatalf("width %d detail line still overflowed at %d:\n%s", width, statsVisibleWidth(detail), detail)
	}
	jsonReport := usageStatsJSONReport(t, report)
	if len(jsonReport.Models) != 1 || jsonReport.Models[0].Activity == nil || jsonReport.Models[0].Activity.ToolCalls != 999 {
		t.Fatalf("JSON lost the tool-call count text compaction trimmed: %#v", jsonReport.Models)
	}
}

func TestUsageStatsProviderDetailCompactsCacheHitByWidth(t *testing.T) {
	report := usageStatsTextFixture()
	rate := "88.14"
	report.Metric = "cost"
	report.Providers = []usage.StatsDimension{{
		Name: "wide-provider", Client: "codex", Tokens: 999_900_000_000_000, Sessions: 999_999_999,
		KnownShare: "0", Coverage: "0", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
	}}
	// Same 75-char "unavailable"/"unavailable" high-value line as the model
	// case above; adding " · 88.14% hit" (13) reaches 88, over width 80 but
	// under 100.
	for _, tc := range []struct {
		name         string
		width        int
		wantCacheHit bool
	}{
		{name: "width 80 drops cache-hit", width: 80, wantCacheHit: false},
		{name: "width 100 keeps cache-hit", width: 100, wantCacheHit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: tc.width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			detail := modelDetailLine(t, text, "Codex/wide-provider")
			if !strings.Contains(detail, "999,999,999 sessions") || !strings.Contains(detail, "UNPRICED") {
				t.Fatalf("width %d dropped a high-value field:\n%s", tc.width, detail)
			}
			if strings.Contains(detail, "hit") != tc.wantCacheHit {
				t.Fatalf("width %d cache-hit presence = %v, want %v:\n%s", tc.width, strings.Contains(detail, "hit"), tc.wantCacheHit, detail)
			}
			if tc.wantCacheHit && !strings.Contains(detail, "88.14% hit") {
				t.Fatalf("cache-hit percentage was rounded or reformatted, want unrounded 88.14%%:\n%s", detail)
			}
			if statsVisibleWidth(detail) > tc.width {
				t.Fatalf("width %d detail line overflowed at %d:\n%s", tc.width, statsVisibleWidth(detail), detail)
			}
			assertUsageStatsWidth(t, text, tc.width)
			jsonReport := usageStatsJSONReport(t, report)
			if len(jsonReport.Providers) != 1 || jsonReport.Providers[0].CacheHitRate == nil || *jsonReport.Providers[0].CacheHitRate != "88.14" {
				t.Fatalf("width %d JSON lost cache-hit data text compaction trimmed: %#v", tc.width, jsonReport.Providers)
			}
		})
	}
}

// TestUsageStatsModelProviderDetailStaysOneLineInTwoColumnLayout closes a gap
// the single-column 80/100 detail-compaction tests left open: at terminal
// width >= 104, render() switches to a two-column layout and passes only a
// fraction of the terminal width to rankingLines (MODELS/PROVIDERS), not the
// full width — e.g. the pre-fix split gave the ranking column just 40 columns
// at terminal width 104, well under the 80-column single-line contract, even
// though 104 itself comfortably clears it. This exercises 104/140/160 (the
// two-column band up to statsMaxWidth) with fields sized so the full detail
// does not fit, proving the actual available column width, not just the raw
// terminal width, respects the single-line contract.
func TestUsageStatsModelProviderDetailStaysOneLineInTwoColumnLayout(t *testing.T) {
	rate := "88.14"
	report := usageStatsTextFixture()
	report.Metric = "cost"
	report.Models = []usage.StatsDimension{{
		Name: "wide-model", Client: "codex", Tokens: 84_900_000, Sessions: 69,
		KnownShare: "0", Coverage: "0", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
		Activity: &usage.StatsModelActivity{ToolCalls: 57},
	}}
	report.Providers = []usage.StatsDimension{{
		Name: "wide-provider", Client: "codex", Tokens: 84_900_000, Sessions: 69,
		KnownShare: "0", Coverage: "0", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
	}}
	// Under metric=cost with no known cost/coverage, both share and cost
	// render "unavailable" (11 chars each). High-value alone is 65 chars:
	// "84.9M tokens · unavailable · unavailable · UNPRICED · 69 sessions".
	// +" · 57 tools" (11) = 76, still fits an 80-column ranking width, so the
	// model keeps tools. +" · 88.14% hit" (13) = 89, over 80, so the model
	// drops cache-hit. The provider has no tools field, so its full detail is
	// 65+13 = 78, which fits 80 outright and keeps cache-hit — showing the
	// two sections compact independently, not in lockstep.
	// A two-column joined row interleaves the trend column, a gap, and the
	// ranking column, so a wrapped continuation line does not start at column
	// 0 the way it would in single-column mode, and it is not globally blank
	// either while trend still has rows on the same line — the row-scanning
	// modelDetailLine helper (built for single-column detail blocks) cannot
	// reliably isolate the ranking column's lines here. Instead, assert that
	// each field run appears as one *contiguous* substring: statsWrap only
	// ever breaks at a space between two of these fields, so if the run from
	// the first high-value field through the last kept secondary field
	// appears unbroken in the full text, no wrap occurred within it — proof
	// of a single line — without needing to isolate the column at all.
	const modelHighValue = "84.9M tokens · unavailable · unavailable · UNPRICED · 69 sessions"
	const providerHighValue = modelHighValue // identical fixture values, no tools field
	// The fixture's first bucket (2026-07-14, day-grouped) renders as label
	// "Jul 14" and, under metric=cost with a known-but-not-complete value,
	// value "$13.4M KNOWN" — both must stay visible at every width: below
	// statsTwoColumnFits' threshold trend gets the full stacked width, at or
	// above it trend gets its own floored column (statsTrendMinWidth), never
	// a column truncated below what its own label+value need.
	for _, tc := range []struct {
		width             int
		wantModelCacheHit bool
	}{
		// width 104 is below statsTwoColumnFits' threshold (needs inner >=
		// 80+28=108, i.e. width >= 112), so it now stacks: rankingLines gets
		// the full 104-column width, comfortably fitting the model's full
		// 89-column detail (cache-hit included) as well as the provider's.
		{width: 104, wantModelCacheHit: true},
		// 140/160 clear the threshold and stay two-column with the ranking
		// column floored at 80, reproducing the original Round 1 scenario
		// where the model (but not the shorter, tools-free provider) has to
		// drop cache-hit.
		{width: 140, wantModelCacheHit: false},
		{width: 160, wantModelCacheHit: false},
	} {
		t.Run(fmt.Sprintf("width %d", tc.width), func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: tc.width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()

			if !strings.Contains(text, modelHighValue+" · 57 tools") {
				t.Fatalf("width %d model detail did not keep high-value+tools contiguous on one line (wrapped, or tools dropped):\n%s", tc.width, text)
			}
			if hasModelCacheHit := strings.Contains(text, modelHighValue+" · 57 tools · 88.14% hit"); hasModelCacheHit != tc.wantModelCacheHit {
				t.Fatalf("width %d model cache-hit contiguous-on-one-line = %v, want %v:\n%s", tc.width, hasModelCacheHit, tc.wantModelCacheHit, text)
			}

			if !strings.Contains(text, providerHighValue+" · 88.14% hit") {
				t.Fatalf("width %d provider detail did not keep high-value+cache-hit contiguous on one line (wrapped, cache-hit dropped, or rounded):\n%s", tc.width, text)
			}

			if !strings.Contains(text, "Jul 14") {
				t.Fatalf("width %d trend bucket label was truncated or dropped:\n%s", tc.width, text)
			}
			if !strings.Contains(text, "$13.4M KNOWN") {
				t.Fatalf("width %d trend bucket value was truncated or dropped:\n%s", tc.width, text)
			}

			assertUsageStatsWidth(t, text, tc.width)
			jsonReport := usageStatsJSONReport(t, report)
			if len(jsonReport.Models) != 1 || jsonReport.Models[0].Activity == nil || jsonReport.Models[0].Activity.ToolCalls != 57 || jsonReport.Models[0].CacheHitRate == nil || *jsonReport.Models[0].CacheHitRate != "88.14" {
				t.Fatalf("width %d JSON lost model fields text compaction trimmed: %#v", tc.width, jsonReport.Models)
			}
			if len(jsonReport.Providers) != 1 || jsonReport.Providers[0].CacheHitRate == nil || *jsonReport.Providers[0].CacheHitRate != "88.14" {
				t.Fatalf("width %d JSON lost provider fields text compaction trimmed: %#v", tc.width, jsonReport.Providers)
			}
			if len(jsonReport.Buckets) == 0 || jsonReport.Buckets[0].Tokens != 13402755 {
				t.Fatalf("width %d JSON lost the trend bucket: %#v", tc.width, jsonReport.Buckets)
			}
		})
	}
}

// isTwoColumnStatsLayout reports whether text was rendered in the two-column
// layout, by checking whether joinStatsColumns paired the TREND and MODELS
// section titles onto the same line — the same signal
// TestUsageStatsWideLayoutAndColorAreDeterministic already uses.
func isTwoColumnStatsLayout(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "TREND") && strings.Contains(line, "MODELS") {
			return true
		}
	}
	return false
}

// TestUsageStatsTwoColumnThresholdCoversWidestSupportedTrendFormats closes a
// gap the round-2 repair's static statsTrendMinWidth=28 left open: 28 assumes
// trendLines' 7/9-column *defaults*, but compact label/value formats can be
// wider than that — a known-but-partial cost value like "$13.4M KNOWN" is 12
// columns, and so is a multi-date hour label like "Jul 14 09:00" (the format
// compactBucketLabels switches to whenever buckets span more than one
// calendar date under hour grouping). At width 112 (the old static
// threshold's exact activation point), a report using both wide formats
// still needs 36 trend columns (12 label + 2 + 8 min-bar + 2 + 12 value), not
// 28, so two-column mode would still truncate at that width. This asserts
// the layout instead stacks until the terminal is actually wide enough for
// this report's real content — not a fixed guess — and that model/provider
// detail stays single-line throughout.
func TestUsageStatsTwoColumnThresholdCoversWidestSupportedTrendFormats(t *testing.T) {
	rate := "88.14"
	report := usageStatsTextFixture()
	report.Metric = "cost"
	report.GroupBy = "hour"
	// Two different calendar dates under hour grouping forces
	// compactBucketLabels' "Jan 02 15:04" (12-column) format for every label.
	// KnownMetricValue with no MetricValue/Coverage renders as a 12-column
	// "$13.4M KNOWN" partial-cost value, matching the fixture value used
	// elsewhere in this file for the same reason.
	report.Buckets = []usage.StatsBucket{
		{Start: "2026-07-14T09:00:00Z", Tokens: 13402755, Sessions: 3, KnownMetricValue: "13402755"},
		{Start: "2026-07-15T09:00:00Z", Tokens: 13402755, Sessions: 3, KnownMetricValue: "13402755"},
	}
	report.Models = []usage.StatsDimension{{
		Name: "wide-model", Client: "codex", Tokens: 84_900_000, Sessions: 69,
		KnownShare: "0", Coverage: "0", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
		Activity: &usage.StatsModelActivity{ToolCalls: 57},
	}}
	report.Providers = []usage.StatsDimension{{
		Name: "wide-provider", Client: "codex", Tokens: 84_900_000, Sessions: 69,
		KnownShare: "0", Coverage: "0", CacheHitRate: &rate, CachedReadTokens: 1, LogicalInputTokens: 2,
	}}
	const modelHighValue = "84.9M tokens · unavailable · unavailable · UNPRICED · 69 sessions"
	const trendLabel = "Jul 14 09:00"
	const trendValue = "$13.4M KNOWN"
	// trendMinWidth = 12 (label) + 2 + 8 (min bar) + 2 + 12 (value) = 36.
	// Two-column needs inner (width-4) >= statsRankingMinWidth(80) + 36 = 116,
	// i.e. width >= 120.
	for _, tc := range []struct {
		width         int
		wantStacked   bool
		wantTwoColumn bool
	}{
		{width: 112, wantStacked: true, wantTwoColumn: false}, // the old static-28 threshold's own activation point
		{width: 119, wantStacked: true, wantTwoColumn: false}, // one column short of fitting this report's real content
		{width: 120, wantStacked: false, wantTwoColumn: true}, // first width that truly fits ranking + this trend + gap
	} {
		t.Run(fmt.Sprintf("width %d", tc.width), func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: tc.width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()

			if got := isTwoColumnStatsLayout(text); got != tc.wantTwoColumn {
				t.Fatalf("width %d two-column layout = %v, want %v:\n%s", tc.width, got, tc.wantTwoColumn, text)
			}
			if !strings.Contains(text, trendLabel) {
				t.Fatalf("width %d trend bucket label %q was truncated or dropped:\n%s", tc.width, trendLabel, text)
			}
			if !strings.Contains(text, trendValue) {
				t.Fatalf("width %d trend bucket value %q was truncated or dropped:\n%s", tc.width, trendValue, text)
			}
			if !strings.Contains(text, modelHighValue+" · 57 tools") {
				t.Fatalf("width %d model detail did not keep high-value+tools contiguous on one line (wrapped, or tools dropped):\n%s", tc.width, text)
			}

			assertUsageStatsWidth(t, text, tc.width)
			jsonReport := usageStatsJSONReport(t, report)
			if len(jsonReport.Buckets) != 2 {
				t.Fatalf("width %d JSON lost a trend bucket: %#v", tc.width, jsonReport.Buckets)
			}
			if len(jsonReport.Models) != 1 || jsonReport.Models[0].CacheHitRate == nil || *jsonReport.Models[0].CacheHitRate != "88.14" {
				t.Fatalf("width %d JSON lost model fields: %#v", tc.width, jsonReport.Models)
			}
		})
	}
}

func TestUsageStatsModelActivityDetailIsOptIn(t *testing.T) {
	report := usageStatsTextFixture()
	report.Models = report.Models[:1]
	report.ShowModelActivity = true
	average := int64(250)
	report.Models[0].Activity = &usage.StatsModelActivity{ActiveSessions: 3, ActiveDays: 2, FirstAt: "2026-07-19T00:00:00Z", LastAt: "2026-07-20T00:00:00Z", ToolCalls: 7, CompletedCalls: 5, FailedCalls: 1, TotalDurationMS: 1500, AverageDuration: &average, Tools: []usage.StatsToolCount{{Name: "exec_command", Calls: 5}, {Name: "apply_patch", Calls: 2}}}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 72}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MODEL ACTIVITY · claude-opus-4-8", "3 sessions", "2 active days", "exec_command", "apply_patch", "250 ms average"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("model activity missing %q:\n%s", want, output.String())
		}
	}
	assertUsageStatsWidth(t, output.String(), 72)
}

func TestUsageStatsBalancedGolden(t *testing.T) {
	expected, err := os.ReadFile("testdata/usage-stats-balanced.txt")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = renderUsageStatsWithOptions(&output, usageStatsTextFixture(), usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	if output.String() != string(expected) {
		t.Fatalf("balanced stats output changed\n--- want ---\n%s\n--- got ---\n%s", expected, output.String())
	}
}

func TestUsageTextRenderOptionsHonorColumnsWithoutColoringRedirects(t *testing.T) {
	t.Setenv("COLUMNS", "140")
	t.Setenv("TERM", "xterm-256color")
	options := newUsageTextRenderOptions(&bytes.Buffer{}, false)
	if options.width != 140 || options.color {
		t.Fatalf("redirected render options = %#v", options)
	}
}

func TestUsageStatsTextUsesInclusiveDisplayRange(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		to    string
		want  string
		not   string
		label string
	}{
		{name: "custom exclusive midnight", from: "2026-07-01T00:00:00+08:00", to: "2026-07-08T00:00:00+08:00", want: "Jul 01, 2026 - Jul 07, 2026", not: "Jul 08, 2026", label: "LAST 7 DAYS"},
		{name: "range in progress", from: "2026-07-14T00:00:00+08:00", to: "2026-07-20T15:43:01+08:00", want: "Jul 14, 2026 - Jul 20, 2026", not: "Jul 21, 2026", label: "LAST 7 DAYS"},
		{name: "DST boundary", from: "2026-03-07T00:00:00-05:00", to: "2026-03-10T00:00:00-04:00", want: "Mar 07, 2026 - Mar 09, 2026", not: "Mar 10, 2026", label: "LAST 3 DAYS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := usageStatsTextFixture()
			report.Range = usage.StatsRange{From: test.from, To: test.to}
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), test.want) || !strings.Contains(output.String(), test.label) || strings.Contains(output.String(), test.not) {
				t.Fatalf("display range =\n%s\nwant %q and %q without %q", output.String(), test.want, test.label, test.not)
			}
		})
	}
}

func TestUsageStatsActivityLegendShowsInclusiveRangeAtSupportedWidths(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want string
		not  string
	}{
		{
			name: "custom range",
			from: "2026-07-01T00:00:00+08:00",
			to:   "2026-07-08T00:00:00+08:00",
			want: "Jul 01, 2026 - Jul 07, 2026",
			not:  "Jul 08, 2026",
		},
		{
			name: "DST boundary",
			from: "2026-03-07T00:00:00-05:00",
			to:   "2026-03-10T00:00:00-04:00",
			want: "Mar 07, 2026 - Mar 09, 2026",
			not:  "Mar 10, 2026",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, width := range []int{48, 72, 100, 140} {
				t.Run(strconv.Itoa(width)+" columns", func(t *testing.T) {
					report := usageStatsTextFixture()
					report.Range = usage.StatsRange{From: test.from, To: test.to}
					var output bytes.Buffer
					if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
						t.Fatal(err)
					}
					text := output.String()
					activity := usageStatsActivitySection(t, text)
					if !strings.Contains(activity, test.want) || strings.Contains(activity, test.not) {
						t.Fatalf("%d-column Activity range =\n%s\nwant %q without %q", width, activity, test.want, test.not)
					}
					if strings.Contains(activity, "…") {
						t.Fatalf("%d-column Activity range was truncated:\n%s", width, activity)
					}
					if width == 48 && strings.Contains(usageStatsLineContaining(activity, "LESS"), test.want) {
						t.Fatalf("%d-column Activity legend and range were not split:\n%s", width, activity)
					}
					if strings.Contains(text, "\x1b[") {
						t.Fatalf("%d-column redirected output contains ANSI:\n%q", width, text)
					}
					assertUsageStatsWidth(t, text, width)
				})
			}
		})
	}
}

func TestUsageStatsHourLabelsDistinguishDatesAndDSTFold(t *testing.T) {
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-10-31T23:00:00-04:00", To: "2026-11-01T03:00:00-05:00"},
		Timezone: "America/New_York", GroupBy: "hour", Metric: "tokens",
		Totals: usage.StatsTotals{Tokens: 100, Sessions: 1, Events: 4},
		Buckets: []usage.StatsBucket{
			{Start: "2026-10-31T23:00:00-04:00", Tokens: 10, KnownMetricValue: "10"},
			{Start: "2026-11-01T00:00:00-04:00", Tokens: 20, KnownMetricValue: "20"},
			{Start: "2026-11-01T01:00:00-04:00", Tokens: 30, KnownMetricValue: "30"},
			{Start: "2026-11-01T01:00:00-05:00", Tokens: 40, KnownMetricValue: "40"},
		},
		Models: []usage.StatsDimension{}, Clients: []usage.StatsDimension{}, Activity: []usage.StatsActivity{},
		Peak: usage.StatsPeak{KnownValue: "40"}, Coverage: usage.StatsCoverage{Percent: "0.00"},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if !strings.Contains(text, "USAGE STATS · LAST 2 DAYS") {
			t.Fatalf("%d-column DST range label =\n%s", width, text)
		}
		for _, want := range []string{"Oct 31 23:00", "Nov 01 00:00", "Nov 01 01:00 -04:00", "Nov 01 01:00 -05:00"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%d-column hour trend missing %q:\n%s", width, want, text)
			}
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func TestUsageStatsCostTextDistinguishesCompletePartialAndUnavailable(t *testing.T) {
	complete := "1.000000000"
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-01T00:00:00Z", To: "2026-07-04T00:00:00Z"},
		Timezone: "UTC", GroupBy: "day", Metric: "cost",
		Totals: usage.StatsTotals{Sessions: 3, Events: 4, KnownProviderCost: "3.000000000", KnownAverageCost: "1.000000000"},
		Buckets: []usage.StatsBucket{
			{Start: "2026-07-01T00:00:00Z", Events: 1, ProviderCost: &complete, MetricValue: &complete, KnownMetricValue: complete, Coverage: "100.00"},
			{Start: "2026-07-02T00:00:00Z", Events: 2, KnownProviderCost: "2.000000000", KnownMetricValue: "2.000000000", Coverage: "50.00"},
			{Start: "2026-07-03T00:00:00Z", Events: 1, KnownProviderCost: "0.000000000", KnownMetricValue: "0.000000000", Coverage: "0.00"},
		},
		Models: []usage.StatsDimension{
			{Name: "complete-model", Events: 1, MetricValue: &complete, KnownMetricValue: complete, KnownShare: "33.33", Coverage: "100.00"},
			{Name: "partial-model", Events: 2, KnownMetricValue: "2.000000000", KnownShare: "66.67", Coverage: "50.00"},
			{Name: "unpriced-model", Events: 1, KnownMetricValue: "0.000000000", KnownShare: "0.00", Coverage: "0.00"},
		},
		Clients:  []usage.StatsDimension{{Name: "codex", Events: 4, KnownMetricValue: "3.000000000", KnownShare: "100.00", Coverage: "50.00"}},
		Activity: []usage.StatsActivity{{Weekday: 0, Hour: 9, KnownMetricValue: "2.000000000"}},
		Peak:     usage.StatsPeak{KnownValue: "2.000000000", Coverage: "50.00"},
		Coverage: usage.StatsCoverage{PricedEvents: 2, UnpricedEvents: 2, TotalEvents: 4, Percent: "50.00"},
		UnpricedModels: []usage.StatsUnpricedModel{
			{Client: "codex", Model: "partial-model", Components: []string{"output"}},
			{Client: "codex", Model: "unpriced-model", Components: []string{"unknown_model"}},
		},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		for _, want := range []string{"$1.00", "$2.00 KNOWN", "unavailable", "33.3%", "66.7%", "100.0%", "ACTIVITY BY WEEKDAY / HOUR · KNOWN COST", "UNPRICED MODELS", "output", "unknown_model"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%d-column cost report missing %q:\n%s", width, want, text)
			}
		}
		for marker, want := range map[string]string{
			"complete-model": "33.3%",
			"partial-model":  "66.7%",
			"unpriced-model": "unavailable",
			"Codex":          "100.0%",
		} {
			line := usageStatsLineContaining(text, marker)
			if !strings.Contains(line, want) {
				t.Fatalf("%d-column line containing %q = %q, want %q", width, marker, line, want)
			}
		}
		for marker, want := range map[string]string{"Jul 01": "$1.00", "Jul 02": "$2.00 KNOWN", "Jul 03": "unavailable"} {
			line := usageStatsTrendLine(text, marker)
			if !strings.Contains(line, want) {
				t.Fatalf("%d-column line starting with %q = %q, want %q", width, marker, line, want)
			}
		}
		if strings.Contains(usageStatsTrendLine(text, "Jul 01"), "KNOWN") {
			t.Fatalf("%d-column complete bucket marked partial:\n%s", width, text)
		}
		if strings.Contains(text, "Known priced subtotal") || strings.Contains(text, "\x1b[") {
			t.Fatalf("%d-column partial annotation/ANSI =\n%s", width, text)
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func TestUsageStatsCostTextUsesUnavailableWhenNothingIsPriced(t *testing.T) {
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-01T00:00:00Z", To: "2026-07-02T00:00:00Z"},
		Timezone: "UTC", GroupBy: "day", Metric: "cost",
		Totals:   usage.StatsTotals{Sessions: 1, Events: 1, KnownProviderCost: "0.000000000", KnownAverageCost: "0.000000000"},
		Buckets:  []usage.StatsBucket{{Start: "2026-07-01T00:00:00Z", Events: 1, KnownMetricValue: "0.000000000", Coverage: "0.00"}},
		Models:   []usage.StatsDimension{{Name: "unpriced-model", Events: 1, KnownShare: "0.00", Coverage: "0.00"}},
		Clients:  []usage.StatsDimension{{Name: "codex", Events: 1, KnownShare: "0.00", Coverage: "0.00"}},
		Activity: []usage.StatsActivity{{Weekday: 0, Hour: 9, KnownMetricValue: "0.000000000"}},
		Peak:     usage.StatsPeak{KnownValue: "0.000000000", Coverage: "0.00"},
		Coverage: usage.StatsCoverage{UnpricedEvents: 1, TotalEvents: 1, Percent: "0.00"},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if strings.Count(text, "unavailable") < 5 || strings.Contains(text, "$0.00") || strings.Contains(text, "Known priced subtotal") || strings.Contains(text, "\x1b[") {
			t.Fatalf("%d-column unpriced cost report =\n%s", width, text)
		}
		if line := usageStatsTrendLine(text, "Jul 01"); !strings.Contains(line, "unavailable") {
			t.Fatalf("%d-column line starting with Jul 01 = %q, want unavailable", width, line)
		}
		for _, marker := range []string{"unpriced-model", "Codex", "PEAK", "ACTIVITY BY WEEKDAY / HOUR · COST"} {
			if line := usageStatsLineContaining(text, marker); !strings.Contains(line, "unavailable") && marker != "ACTIVITY BY WEEKDAY / HOUR · COST" {
				t.Fatalf("%d-column line containing %q = %q, want unavailable", width, marker, line)
			}
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func usageStatsLineContaining(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	return ""
}

func usageStatsTrendLine(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, marker) && (strings.Contains(line, "█") || strings.Contains(line, "░")) {
			return line
		}
	}
	return ""
}

func usageStatsActivitySection(t *testing.T, text string) string {
	t.Helper()
	start := strings.Index(text, "▦ ACTIVITY BY WEEKDAY / HOUR")
	if start < 0 {
		t.Fatalf("stats output has no Activity section:\n%s", text)
	}
	section := text[start:]
	if end := strings.Index(section, "\n\n"); end >= 0 {
		section = section[:end]
	}
	return section
}

func assertUsageStatsWidth(t *testing.T, text string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if got := statsVisibleWidth(line); got > width {
			t.Fatalf("%d-column line %d width = %d:\n%s", width, lineNumber+1, got, line)
		}
	}
}

func usageStatsTextFixture() usage.StatsReport {
	knownCost := "316.832730700"
	knownAverage := "9.318609726"
	opusCost := "240.000000000"
	gptCost := "50.000000000"
	read, codexRead := "60.00", "40.00"
	buckets := []usage.StatsBucket{
		{Start: "2026-07-14T00:00:00+08:00", Tokens: 13402755, Sessions: 11, KnownMetricValue: "13402755"},
		{Start: "2026-07-15T00:00:00+08:00", Tokens: 86444030, Sessions: 8, KnownMetricValue: "86444030"},
		{Start: "2026-07-16T00:00:00+08:00", Tokens: 201278072, Sessions: 7, KnownMetricValue: "201278072"},
		{Start: "2026-07-17T00:00:00+08:00", Tokens: 2532096, Sessions: 4, KnownMetricValue: "2532096"},
		{Start: "2026-07-18T00:00:00+08:00", Tokens: 0, Sessions: 0, KnownMetricValue: "0"},
		{Start: "2026-07-19T00:00:00+08:00", Tokens: 379243, Sessions: 2, KnownMetricValue: "379243"},
		{Start: "2026-07-20T00:00:00+08:00", Tokens: 62888663, Sessions: 8, KnownMetricValue: "62888663"},
	}
	models := []usage.StatsDimension{
		{Name: "claude-opus-4-8", Client: "claude", Tokens: 266116000, CachedReadTokens: 600000, CacheWriteTokens: 200000, LogicalInputTokens: 1000000, CacheHitRate: &read, Sessions: 20, KnownShare: "72.53", ProviderCost: &opusCost, KnownProviderCost: opusCost, Coverage: "100.00", Activity: &usage.StatsModelActivity{ToolCalls: 80}},
		{Name: "gpt-5.6-sol", Client: "codex", Tokens: 43480000, CachedReadTokens: 200000, LogicalInputTokens: 500000, CacheHitRate: &codexRead, Sessions: 8, KnownShare: "11.85", ProviderCost: &gptCost, KnownProviderCost: gptCost, Coverage: "100.00", Activity: &usage.StatsModelActivity{ToolCalls: 30}},
		{Name: "claude-fable-5", Client: "claude", Tokens: 32544000, Sessions: 4, KnownShare: "8.87", KnownProviderCost: "26.832730700", Coverage: "50.00", Activity: &usage.StatsModelActivity{ToolCalls: 12}},
		{Name: "codex-auto-review", Client: "codex", Sessions: 2, KnownShare: "2.62", Activity: &usage.StatsModelActivity{ToolCalls: 3}},
	}
	clients := []usage.StatsDimension{
		{Name: "claude", KnownShare: "82.89", LogicalInputTokens: 1000000, CacheHitRate: &read},
		{Name: "codex", KnownShare: "17.11", CacheHitRate: &codexRead},
	}
	providers := []usage.StatsDimension{
		{Name: "relay", Client: "claude", Tokens: 304000000, Sessions: 24, KnownShare: "82.89", KnownProviderCost: "266.832730700", Coverage: "87.00", LogicalInputTokens: 1000000, CacheHitRate: &read},
		{Name: "official", Client: "codex", Tokens: 62924859, Sessions: 10, KnownShare: "17.11", ProviderCost: &gptCost, KnownProviderCost: gptCost, Coverage: "100.00", CacheHitRate: &codexRead},
	}
	cacheSessions := []usage.StatsCacheSession{
		{Client: "claude", SessionID: "claude-session", Models: []string{"claude-opus-4-8"}, CachedReadTokens: 600000, CacheWriteTokens: 200000, LogicalInputTokens: 1000000, CacheHitRate: &read, DetailCommand: "agentdeck session show claude-session --client claude --activity"},
		{Client: "codex", SessionID: "codex-session", Models: []string{"gpt-5.6-sol"}, CachedReadTokens: 200000, LogicalInputTokens: 500000, CacheHitRate: &codexRead, DetailCommand: "agentdeck session show codex-session --client codex --activity"},
	}
	activity := make([]usage.StatsActivity, 0, 168)
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			value := int64(0)
			if (weekday+hour)%5 == 0 {
				value = int64((weekday + 1) * (hour + 1) * 1000)
			}
			activity = append(activity, usage.StatsActivity{Weekday: weekday, Hour: hour, KnownMetricValue: strconv.FormatInt(value, 10)})
		}
	}
	return usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-14T00:00:00+08:00", To: "2026-07-20T15:43:01+08:00"},
		Timezone: "Asia/Shanghai", GroupBy: "day", Metric: "tokens",
		Totals:         usage.StatsTotals{Tokens: 366924859, Sessions: 34, Events: 2007, KnownProviderCost: knownCost, KnownAverageCost: knownAverage},
		Buckets:        buckets,
		Models:         models,
		Clients:        clients,
		Providers:      providers,
		CacheSessions:  cacheSessions,
		Activity:       activity,
		Peak:           usage.StatsPeak{KnownValue: "201278072"},
		Coverage:       usage.StatsCoverage{Percent: "87.69"},
		UnpricedModels: []usage.StatsUnpricedModel{{Client: "claude", Model: "claude-fable-5", Components: []string{"output"}}},
	}
}
