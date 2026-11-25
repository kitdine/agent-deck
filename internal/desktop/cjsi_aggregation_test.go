package desktop

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const desktopStampDDL = `-- Cache invalidation stamps only. Facts and coverage remain in their own tables.
-- Day bounds are signed UTC days since Unix epoch and form [from_day, to_day).
CREATE TABLE main.cjsi_usage_interval_dirty_ranges (
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0),
    scope_kind TEXT NOT NULL CHECK(scope_kind IN ('aggregate-only','model','client-summary','all-summary','coverage')),
    client TEXT NOT NULL,
    model TEXT NOT NULL,
    from_day INTEGER NOT NULL,
    to_day INTEGER NOT NULL CHECK(to_day>from_day),
    ticket INTEGER NOT NULL CHECK(ticket>0),
    CHECK((scope_kind='all-summary' AND client='') OR (scope_kind<>'all-summary' AND client<>'')),
    CHECK(scope_kind='model' OR model=''),
    PRIMARY KEY(core_epoch,scope_kind,client,model,from_day,to_day)
) STRICT, WITHOUT ROWID;
`

func TestCJSIDesktopTotals(t *testing.T) {
	ctx := t.Context()

	t.Run("A4b desktop top-level, period, compact, and Claude isolation", func(t *testing.T) {
		now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
		from, to := localDay(now, time.UTC)
		knownBase := "15.000000000"
		knownProvider := "25.000000000"
		claudeCost := "5.000000000"
		share50 := "50.00"
		share100 := "100.00"

		// Presentation produced by scoped aggregation with Codex request R=10 (known provider 20),
		// unproved Codex interval residual=10, and Claude request C=5 (complete cost 5).
		pres := usage.PresentationReport{
			Available: true,
			Summary: usage.Summary{
				Tokens:               map[string]int64{"input_tokens": 25},
				Counts:               map[string]int64{"events": 2, "exact": 2, "priced": 2, "unpriced": 0},
				AttributionReasons:   map[string]int64{"exact_run": 2},
				KnownCatalogBaseCost: &knownBase,
				KnownProviderCost:    &knownProvider,
				Unpriced:             []string{},
				Warnings:             []string{},
			},
			Scopes: []usage.PresentationScope{
				{
					Client: "all",
					Periods: usage.PresentationPeriods{Available: true, Items: []usage.PresentationPeriod{{
						Period: "today",
						Totals: usage.PresentationTotals{Tokens: 25, InputTokens: 25, Events: 2, Sessions: 2, KnownCatalogBaseCost: knownBase, KnownProviderCost: knownProvider, PricingComplete: false},
					}}},
				},
				{
					Client: "codex",
					Periods: usage.PresentationPeriods{Available: true, Items: []usage.PresentationPeriod{{
						Period: "today",
						Totals: usage.PresentationTotals{Tokens: 20, InputTokens: 20, Events: 1, Sessions: 1, KnownCatalogBaseCost: "10.000000000", KnownProviderCost: "20.000000000", PricingComplete: false},
						Models: []usage.PresentationModel{{Model: "M", Value: usage.PresentationDisplayValue{Tokens: 10, Events: 1, ProviderCost: "20.000000000", CostIncomplete: true}, Share: &share50}},
					}}},
				},
				{
					Client: "claude",
					Periods: usage.PresentationPeriods{Available: true, Items: []usage.PresentationPeriod{{
						Period: "today",
						Totals: usage.PresentationTotals{Tokens: 5, InputTokens: 5, Events: 1, Sessions: 1, CatalogBaseCost: &claudeCost, ProviderCost: &claudeCost, KnownCatalogBaseCost: claudeCost, KnownProviderCost: claudeCost, PricingComplete: true},
					}}},
				},
			},
			ClientSubtotals: usage.PresentationClientSubtotals{Available: true, Items: []usage.PresentationClientSubtotal{
				{Period: "today", Client: "codex", Value: usage.PresentationDisplayValue{Tokens: 20, Events: 1, ProviderCost: "20.000000000", CostIncomplete: true}},
				{Period: "today", Client: "claude", Value: usage.PresentationDisplayValue{Tokens: 5, Events: 1, ProviderCost: claudeCost, CostIncomplete: false}},
			}},
		}
		snap := usageSnapshotFromPresentation(pres, from, to)

		// Top-level UsageSnapshot: PricingComplete MUST be false even with 0 unpriced components!
		if !snap.Available || snap.PricingComplete || snap.UnpricedComponents != 0 ||
			snap.CatalogBaseCost != nil || snap.ProviderCost != nil ||
			snap.KnownCatalogBaseCost == nil || *snap.KnownCatalogBaseCost != "15.000000000" ||
			snap.KnownProviderCost == nil || *snap.KnownProviderCost != "25.000000000" ||
			snap.Tokens["input_tokens"] != 25 || snap.Counts["events"] != 2 {
			t.Fatalf("top-level usage snapshot wrong: %+v", snap)
		}
		// CJSI derived snapshot payload marks PricingComplete=false for partial/interval summaries
		// and passes validCJSIDerivedPayload.
		work := WorkSignalsSnapshot{
			Activity: WorkSignalActivityFamily{Items: []WorkSignalActivityItem{}},
			Workflow: WorkSignalWorkflowFamily{Items: []WorkSignalWorkflowItem{}},
			Tooling:  WorkSignalToolingFamily{Items: []WorkSignalToolingItem{}},
		}
		cjsiPayload := cjsiDerivedSnapshotPayload(pres, work)
		if cjsiPayload.PriceAvailability.PricingComplete || cjsiPayload.PriceAvailability.UnpricedComponents != 0 || !validCJSIDerivedPayload(cjsiPayload) {
			t.Fatalf("CJSI derived snapshot payload wrong: %+v", cjsiPayload.PriceAvailability)
		}
		// Compact client subtotals: codex has 20 tokens, 1 event, CostIncomplete=true; claude has 5, CostIncomplete=false.
		var codexSub, claudeSub usage.PresentationClientSubtotal
		for _, item := range snap.Presentation.ClientSubtotals.Items {
			if item.Period == "today" && item.Client == "codex" {
				codexSub = item
			}
			if item.Period == "today" && item.Client == "claude" {
				claudeSub = item
			}
		}
		if codexSub.Value.Tokens != 20 || codexSub.Value.Events != 1 || codexSub.Value.ProviderCost != "20.000000000" || !codexSub.Value.CostIncomplete {
			t.Fatalf("codex compact subtotal wrong: %+v", codexSub)
		}
		if claudeSub.Value.Tokens != 5 || claudeSub.Value.Events != 1 || claudeSub.Value.ProviderCost != "5.000000000" || claudeSub.Value.CostIncomplete {
			t.Fatalf("claude compact subtotal polluted: %+v", claudeSub)
		}
		// Codex period model row M has 10 tokens (50.00% share) and CostIncomplete=true.
		var codexScope usage.PresentationScope
		for _, sc := range snap.Presentation.Scopes {
			if sc.Client == "codex" {
				codexScope = sc
			}
		}
		todayPeriod := codexScope.Periods.Items[0]
		if todayPeriod.Totals.PricingComplete || todayPeriod.Totals.Tokens != 20 || todayPeriod.Totals.Events != 1 ||
			todayPeriod.Totals.Sessions != 1 || todayPeriod.Totals.CatalogBaseCost != nil || todayPeriod.Totals.ProviderCost != nil ||
			todayPeriod.Totals.KnownCatalogBaseCost != "10.000000000" || todayPeriod.Totals.KnownProviderCost != "20.000000000" {
			t.Fatalf("codex today period totals wrong: %+v", todayPeriod.Totals)
		}
		if len(todayPeriod.Models) != 1 || todayPeriod.Models[0].Model != "M" || todayPeriod.Models[0].Value.Tokens != 10 ||
			todayPeriod.Models[0].Share == nil || *todayPeriod.Models[0].Share != "50.00" || !todayPeriod.Models[0].Value.CostIncomplete {
			t.Fatalf("codex unproved model row wrong: %+v", todayPeriod.Models)
		}
		// Whole-interval proved model M=20 (100.00% share), still CostIncomplete=true.
		pres.Scopes[1].Periods.Items[0].Models[0].Value.Tokens = 20
		pres.Scopes[1].Periods.Items[0].Models[0].Share = &share100
		snapProved := usageSnapshotFromPresentation(pres, from, to)
		if snapProved.PricingComplete || snapProved.Presentation.Scopes[1].Periods.Items[0].Models[0].Value.Tokens != 20 ||
			*snapProved.Presentation.Scopes[1].Periods.Items[0].Models[0].Share != "100.00" {
			t.Fatalf("codex proved model snapshot wrong: %+v", snapProved)
		}
	})

	t.Run("R9-F3 legacy v1 derived cache bytes preserved", func(t *testing.T) {
		root := t.TempDir()
		core, err := store.Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		defer core.Close()
		for _, q := range []string{
			`INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('cat-1','bundled','bundled://review','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1)`,
			`INSERT INTO model_prices(catalog_version,provider,model,effective_from,prices_json,aliases_json) VALUES ('cat-1','anthropic','review-model','2026-01-01T00:00:00Z','{"input":"1000000"}','[]')`,
			`INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES ('claude','review-thread','2026-01-02T09:30:00Z','2026-01-02T09:30:00Z')`,
			`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES ('review-key','claude','review-thread','one','2026-01-02T09:30:00Z','review-model',5,0,'g',0)`,
		} {
			if _, err := core.DB.ExecContext(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
		now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
		from, to := localDay(now, time.UTC)
		s := Service{StateRoot: root, Home: t.TempDir(), Location: time.UTC}
		p, err := s.derivedSnapshotPayload(ctx, core, now)
		if err != nil {
			t.Fatal(err)
		}
		if p.Summary.CatalogBaseCost == nil || len(p.Summary.Unpriced) != 0 || p.Summary.ProviderCost != nil || p.Summary.KnownProviderCost == nil {
			t.Fatalf("unexpected request fixture: catalog=%v provider=%v known-provider=%v unpriced=%v",
				p.Summary.CatalogBaseCost, p.Summary.ProviderCost != nil, p.Summary.KnownProviderCost != nil, p.Summary.Unpriced)
		}
		if !p.PriceAvailability.PricingComplete {
			t.Fatal("legacy v1 derivedSnapshotPayload must preserve pricing_complete=true when len(Unpriced)==0")
		}
		if legacySnap := usageSnapshot(p.Summary, from, to); !legacySnap.PricingComplete {
			t.Fatal("legacy v1 usageSnapshot must preserve PricingComplete=true when len(Unpriced)==0")
		}
		b, err := encodeDerivedSnapshotCache(derivedSnapshotCacheHeader{Version: 1, AlgorithmVersion: 1, DatabaseIdentity: "same-fixture"}, p)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		const wantBaselineV1SHA256 = "bc5616deee8313e7f685512b98b25724f663dca3e0f8c6085ae5c1ecb136a5ff"
		if got := hex.EncodeToString(sum[:]); got != wantBaselineV1SHA256 {
			t.Fatalf("v1 derived cache bytes drifted from baseline: got sha256=%s want=%s", got, wantBaselineV1SHA256)
		}
		// Meanwhile CJSI v2 payload and usageSnapshotFromPresentation apply strict completeness.
		if cjsiPayload := cjsiDerivedSnapshotPayload(p.Presentation, p.WorkSignals); cjsiPayload.PriceAvailability.PricingComplete {
			t.Fatal("CJSI v2 payload must not mark unattributed provider cost as pricing complete")
		}
		if cjsiSnap := usageSnapshotFromPresentation(p.Presentation, from, to); cjsiSnap.PricingComplete {
			t.Fatal("CJSI usageSnapshotFromPresentation must not mark unattributed provider cost as pricing complete")
		}
	})

	t.Run("derived cache v2 lifecycle, interval deletion invalidation, and R8-F1 clock controls", func(t *testing.T) {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`CREATE TABLE main.cjsi_publication_head(singleton INTEGER PRIMARY KEY,core_epoch INTEGER,ticket INTEGER);
			INSERT INTO cjsi_publication_head VALUES(1,11,1);` + desktopStampDDL)
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 3, 10, 12, 10, 0, 0, loc)
		until := now.Add(25 * time.Minute)
		view := cjsiDerivedView{
			epoch: 11, ticket: 1, database: "fixture-db", schema: 37, wire: WireVersion, usageParser: 9, sessionParser: 7,
			proof: cjsiDerivedProof{Publisher: "fixture", Facts: [32]byte{1}, Sessions: [32]byte{2}, Protection: [32]byte{3}, Coverage: [32]byte{4}, Prices: [32]byte{5}, Routes: [32]byte{6}},
		}
		key := func(v cjsiDerivedView, at, expiry time.Time, zone *time.Location) cjsiDerivedCacheKey {
			t.Helper()
			k, err := newCJSIDerivedCacheKey(ctx, tx, v, at, expiry, zone)
			if err != nil {
				t.Fatal(err)
			}
			return k
		}
		k := key(view, now, until, loc)
		known := "20.000000000"
		p := derivedSnapshotCachePayload{
			Summary: usage.Summary{
				Tokens: map[string]int64{"input_tokens": 20}, Counts: map[string]int64{"events": 1},
				AttributionReasons: map[string]int64{"exact_run": 1}, KnownProviderCost: &known, KnownCatalogBaseCost: &known,
				Unpriced: []string{}, Warnings: []string{"partial"},
			},
			PriceAvailability: derivedSnapshotPriceAvailability{Validated: true},
			WorkSignals: WorkSignalsSnapshot{
				Activity: WorkSignalActivityFamily{Items: []WorkSignalActivityItem{}},
				Workflow: WorkSignalWorkflowFamily{Items: []WorkSignalWorkflowItem{}},
				Tooling:  WorkSignalToolingFamily{Items: []WorkSignalToolingItem{}},
			},
		}
		p.Presentation = usage.PresentationReport{
			Available: true,
			Scopes: []usage.PresentationScope{{Client: "all", Periods: usage.PresentationPeriods{Available: true,
				Items: []usage.PresentationPeriod{{Period: "30d", Totals: usage.PresentationTotals{Tokens: 20, Events: 1, Sessions: 1, KnownProviderCost: known}}}}}},
			ClientSubtotals: usage.PresentationClientSubtotals{Available: true, Items: []usage.PresentationClientSubtotal{{
				Period: "today", Client: "codex", Value: usage.PresentationDisplayValue{Tokens: 20, Events: 1, ProviderCost: known, CostIncomplete: true},
			}}},
		}
		encode := func(k cjsiDerivedCacheKey) []byte {
			t.Helper()
			data, err := k.encode(p)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		data := encode(k)

		// Control 1: Berlin repeated hour (fall-back DST 2026-10-25).
		t.Run("first-repeated-local-hour", func(t *testing.T) {
			berlin, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				t.Fatal(err)
			}
			created := time.Date(2026, 10, 24, 23, 50, 0, 0, time.UTC) // 01:50 CEST
			first02 := created.Add(10 * time.Minute)                   // 02:00 CEST (00:00Z)
			later := first02.Add(10 * time.Minute)                     // 02:10 CEST (00:10Z)
			expiry := later.Add(2 * time.Hour)
			old, err := newCJSIDerivedCacheKey(ctx, tx, view, created, expiry, berlin)
			if err != nil {
				t.Fatal(err)
			}
			payload := p
			payload.Presentation.Scopes = append([]usage.PresentationScope(nil), p.Presentation.Scopes...)
			payload.Presentation.Scopes[0].Hourly.ThroughHour = 1
			body, err := old.encode(payload)
			if err != nil {
				t.Fatal(err)
			}
			if got, hit := old.reuse(body, first02.Add(-time.Nanosecond)); !hit || got.Presentation.Scopes[0].Hourly.ThroughHour != 1 {
				t.Fatal("valid preceding-hour cache was not reusable 1ns before first 02:00")
			}
			fresh, err := newCJSIDerivedCacheKey(ctx, tx, view, later, expiry, berlin)
			if err != nil {
				t.Fatal(err)
			}
			if got, hit := fresh.reuse(body, later); hit || got.Summary.Tokens != nil {
				t.Fatalf("preceding hour reused at first 02:10: hit=%v until=%s", hit, old.header.ValidUntil)
			}
			if old.header.ValidUntil != first02.Format(time.RFC3339Nano) {
				t.Fatalf("expected first occurrence deadline %s, got %s", first02.Format(time.RFC3339Nano), old.header.ValidUntil)
			}
		})

		// Control 2: Predecessor v2 (AlgorithmVersion=1) rejected even with valid checksum.
		t.Run("predecessor-v2-expiry", func(t *testing.T) {
			berlin, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				t.Fatal(err)
			}
			created := time.Date(2026, 10, 24, 23, 50, 0, 0, time.UTC)
			first0210 := created.Add(20 * time.Minute)
			expiry := first0210.Add(2 * time.Hour)
			old, err := newCJSIDerivedCacheKey(ctx, tx, view, created, expiry, berlin)
			if err != nil {
				t.Fatal(err)
			}
			_, tomorrow := localDay(created, berlin)
			old.header.AlgorithmVersion = derivedSnapshotCacheAlgorithmVersion
			old.header.ValidUntil = derivedSnapshotClockBoundary(created, berlin, tomorrow).UTC().Format(time.RFC3339Nano)
			if old.header.ValidUntil != "2026-10-25T01:00:00Z" {
				t.Fatalf("unexpected predecessor deadline: %s", old.header.ValidUntil)
			}
			payload := p
			payload.Presentation.Scopes = append([]usage.PresentationScope(nil), p.Presentation.Scopes...)
			payload.Presentation.Scopes[0].Hourly.ThroughHour = 1
			body, err := encodeDerivedSnapshotCache(old.header, payload)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := newCJSIDerivedCacheKey(ctx, tx, view, first0210, expiry, berlin)
			if err != nil {
				t.Fatal(err)
			}
			if got, hit := fresh.reuse(body, first0210); hit || got.Summary.Tokens != nil {
				t.Fatal("accepted predecessor v2 algorithm 1 with stale hourly payload")
			}
		})

		// Control 3: Pacific/Chatham early 03:45 DST transition truncates before 04:00.
		t.Run("chatham-early-offset-transition", func(t *testing.T) {
			chatham, err := time.LoadLocation("Pacific/Chatham")
			if err != nil {
				t.Fatal(err)
			}
			// 2026-04-05 03:45 +13:45 -> 02:45 +12:45 (14:00Z).
			at := time.Date(2026, 4, 4, 13, 30, 0, 0, time.UTC) // 03:15 +13:45
			kChatham, err := newCJSIDerivedCacheKey(ctx, tx, view, at, at.Add(2*time.Hour), chatham)
			if err != nil {
				t.Fatal(err)
			}
			want := time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
			if kChatham.header.ValidUntil != want {
				t.Fatalf("chatham boundary=%s want=%s", kChatham.header.ValidUntil, want)
			}
		})

		// Control 4: Asia/Kathmandu (+05:45) midnight / next-hour boundary.
		t.Run("kathmandu-offset-boundary", func(t *testing.T) {
			ktm, err := time.LoadLocation("Asia/Kathmandu")
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 6, 10, 23, 20, 0, 0, ktm).UTC()
			kKtm, err := newCJSIDerivedCacheKey(ctx, tx, view, at, at.Add(2*time.Hour), ktm)
			if err != nil {
				t.Fatal(err)
			}
			_, nextDay := localDay(at, ktm)
			if kKtm.header.ValidUntil != nextDay.UTC().Format(time.RFC3339Nano) {
				t.Fatalf("kathmandu midnight=%s want=%s", kKtm.header.ValidUntil, nextDay.UTC().Format(time.RFC3339Nano))
			}
		})

		// Control 5: Berlin 2040-12-31 where ZoneBounds returns a past bound.
		t.Run("past-extrapolated-zone-bound", func(t *testing.T) {
			berlin, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2040, 12, 31, 12, 10, 0, 0, time.UTC)
			k2040, err := newCJSIDerivedCacheKey(ctx, tx, view, at, at.Add(2*time.Hour), berlin)
			if err != nil {
				t.Fatalf("valid workday rejected after past zone bound: %v", err)
			}
			if k2040.header.ValidUntil != "2040-12-31T13:00:00Z" {
				t.Fatalf("wrong next-hour deadline: %s", k2040.header.ValidUntil)
			}
		})

		// Control 6: Earlier day boundary overrides next-hour calculation.
		t.Run("earlier-day-boundary", func(t *testing.T) {
			at := time.Date(2026, 6, 10, 23, 30, 0, 0, time.UTC)
			earlyDay := at.Add(10 * time.Minute)
			if got := cjsiDerivedCacheClockBoundary(at, time.UTC, earlyDay); !got.Equal(earlyDay) {
				t.Fatalf("got %s, want earlier day boundary %s", got, earlyDay)
			}
		})

		// Reuse, file round-trip, tamper rejection, v1/v2 isolation, and interval deletion invalidation.
		miss := func(k cjsiDerivedCacheKey, data []byte, at time.Time) {
			t.Helper()
			got, ok := k.reuse(data, at)
			if ok || got.Summary.Tokens != nil {
				t.Fatal("stale or corrupt payload reused")
			}
		}
		got, ok := k.reuse(data, now)
		if !ok || got.Summary.ProviderCost != nil || got.Summary.KnownProviderCost == nil || *got.Summary.KnownProviderCost != known ||
			got.PriceAvailability.PricingComplete || got.Presentation.Summary.Tokens["input_tokens"] != 20 ||
			!got.Presentation.ClientSubtotals.Items[0].Value.CostIncomplete {
			t.Fatalf("lost partial/known/compact shape: ok=%v got=%+v", ok, got)
		}
		path := filepath.Join(t.TempDir(), "v2.json")
		if err := writeDerivedSnapshotCache(path, data); err != nil {
			t.Fatal(err)
		}
		fileData, err := readDerivedSnapshotCacheBytes(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := key(view, now.Add(time.Minute), until.Add(time.Minute), loc).reuse(fileData, now.Add(time.Minute)); !ok {
			t.Fatal("older conservative validity boundary was still current")
		}
		miss(k, data, until)
		miss(k, data, now.Add(-time.Nanosecond))
		miss(k, bytes.Replace(data, []byte(known), []byte("5.000000000"), 1), now)
		miss(k, make([]byte, derivedSnapshotCacheMaxBytes+1), now)

		// v1 format must stay byte-compatible (no dependencies field) and never cross-match v2.
		legacy := k.header
		legacy.Version = 1
		legacy.AlgorithmVersion = derivedSnapshotCacheAlgorithmVersion
		legacy.Dependencies = ""
		v1, err := encodeDerivedSnapshotCache(legacy, p)
		if err != nil || bytes.Contains(v1, []byte(`"dependencies"`)) {
			t.Fatal("changed legacy v1 encoding", err)
		}
		miss(k, v1, now)
		if derivedSnapshotCacheHeaderMatches(k.header, legacy, now) {
			t.Fatal("v1 accepted v2")
		}

		// Interval add and delete-last-interval inside the 90-day daily series invalidate v2 cache.
		start, end := usage.CJSIPresentationWindow(now, loc)
		if start.Hour() != 0 || end.Hour() != 0 || end.Sub(start) != 90*24*time.Hour-time.Hour {
			t.Fatal("calendar window across DST", start, end)
		}
		stamp := func(at time.Time) {
			view.ticket++
			exec(`UPDATE cjsi_publication_head SET ticket=?`, view.ticket)
			day, _ := store.UsageIntervalStampDay(at)
			exec(`INSERT INTO cjsi_usage_interval_dirty_ranges VALUES(11,'all-summary','','',?,?,?)
				ON CONFLICT DO UPDATE SET ticket=excluded.ticket`, day, day+1, view.ticket)
		}
		stamp(start.AddDate(0, 0, -2))
		if _, ok := key(view, now, until, loc).reuse(data, now); !ok {
			t.Fatal("disjoint interval publication caused a cache miss")
		}
		deep := now.AddDate(0, 0, -65)
		stamp(deep)
		added := key(view, now, until, loc)
		miss(added, data, now)
		withInterval := encode(added)
		stamp(deep) // deletion of last interval
		miss(key(view, now, until, loc), withInterval, now)

		// False complete price and unavailable summary must be rejected at encode time.
		p.PriceAvailability.PricingComplete = true
		if _, err := k.encode(p); err == nil {
			t.Fatal("false complete price accepted")
		}
		p.PriceAvailability.PricingComplete = false
		p.Summary.Tokens = nil
		if _, err := k.encode(p); err == nil {
			t.Fatal("unavailable summary became empty")
		}
		// No private fields leak into Presentation wire JSON.
		var wire map[string]json.RawMessage
		encoded, _ := json.Marshal(got.Presentation)
		if err := json.Unmarshal(encoded, &wire); err != nil || wire["dependencies"] != nil || wire["summary"] != nil {
			t.Fatal("private state leaked into presentation wire shape")
		}
	})
}
