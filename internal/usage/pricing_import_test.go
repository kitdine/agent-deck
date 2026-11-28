package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPricingImportKeepsLegacyModelsAndCommitsRulesAtomically(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, t.TempDir())
	s := New(db, "")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	const source = "https://openai.example/catalog"
	const legacy = `{"schema_version":1,"catalog_version":"legacy","currency":"USD","models":{"gpt-example":{"provider":"openai","effective_from":"2026-01-01T00:00:00Z","prices_per_million":{"input":"1"}}}}`
	if err := s.ImportOfficialCatalog(ctx, []byte(legacy), source); err != nil {
		t.Fatal(err)
	}
	if rules, err := s.CatalogPriceRules(ctx, "legacy"); err != nil || rules != nil {
		t.Fatalf("legacy import gained rules: %v, %v", rules, err)
	}

	future := packagedPricingFixture(t, "historical", "2")
	now = now.Add(time.Minute)
	if err := s.ImportOfficialCatalog(ctx, future, source); err != nil {
		t.Fatal(err)
	}
	rules, err := s.CatalogPriceRules(ctx, "historical")
	if err != nil || rules == nil || len(rules.Rules) != 1 || rules.Rules[0].Rates["input"] != "2" {
		t.Fatalf("imported rules: %v, %v", rules, err)
	}

	before, err := s.PriceStatus(ctx)
	if err != nil || before["version"] != "historical" {
		t.Fatalf("initial active catalog: %v, %v", before, err)
	}
	prices, err := s.PriceList(ctx, "openai", "gpt-example")
	if err != nil || len(prices) != 1 || prices[0].Prices["input"] != "1" {
		t.Fatalf("legacy model price = %v, error %v", prices, err)
	}
	revision, err := db.UsageProjectionMetadataRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Minute)
	if err := s.ImportOfficialCatalog(ctx, future, source); err != nil {
		t.Fatalf("identical retry: %v", err)
	}
	if err := s.ImportOfficialCatalog(ctx, packagedPricingFixture(t, "historical", "3"), source); err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("version collision = %v", err)
	}
	badDigest := strings.Replace(string(packagedPricingFixture(t, "bad-digest", "3")), `"input":"3"`, `"input":"4"`, 1)
	if err := s.ImportOfficialCatalog(ctx, []byte(badDigest), source); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("invalid rule accepted: %v", err)
	}
	execPricingSQL(t, db, `CREATE TRIGGER reject_price_rules BEFORE INSERT ON price_rule_sets
WHEN NEW.catalog_version='write-failure' BEGIN SELECT RAISE(ABORT,'injected rule failure'); END`)
	failed := packagedPricingFixture(t, "write-failure", "9")
	if err := s.ImportOfficialCatalog(ctx, failed, source); err == nil || !strings.Contains(err.Error(), "injected rule failure") {
		t.Fatalf("injected write failure = %v", err)
	}
	execPricingSQL(t, db, `DROP TRIGGER reject_price_rules`)
	after, err := s.PriceStatus(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("failed import changed catalog status: %v", err)
	}
	if got, err := db.UsageProjectionMetadataRevision(ctx); err != nil || got != revision {
		t.Fatalf("retry, rejected or rolled-back import dirtied metadata: %d, %v", got, err)
	}
	assertPricingCount(t, db, "price_catalogs", 2)
	assertPricingCount(t, db, "model_prices", 2)
	assertPricingCount(t, db, "price_rule_sets", 1)
	if err := s.ImportOfficialCatalog(ctx, failed, source); err != nil {
		t.Fatal(err)
	}
	assertPricingCount(t, db, "price_catalogs", 3)
	assertPricingCount(t, db, "model_prices", 3)
	assertPricingCount(t, db, "price_rule_sets", 2)
}

func TestPricingImportBackfillsIdenticalPreMigrationCatalog(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, t.TempDir())
	s := New(db, "")
	document := packagedPricingFixture(t, "pre-migration", "2")
	const at = "2026-11-01T00:00:00Z"
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version)
VALUES('pre-migration','official','https://openai.example/catalog',?,?,?,'USD',1);
INSERT INTO model_prices VALUES('pre-migration','gpt-example','openai',?,'{"input":"1"}','[]')`,
		hash(document), at, at, at); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportOfficialCatalog(ctx, document, "https://openai.example/catalog"); err != nil {
		t.Fatal(err)
	}
	assertPricingCount(t, db, "price_catalogs", 1)
	assertPricingCount(t, db, "model_prices", 1)
	assertPricingCount(t, db, "price_rule_sets", 1)
	var imported string
	if err := db.DB.QueryRowContext(ctx, `SELECT imported_at FROM price_catalogs WHERE version='pre-migration'`).Scan(&imported); err != nil || imported != at {
		t.Fatalf("backfill re-dated catalog: %q, %v", imported, err)
	}
}

func TestPricingImportBundledEquivalentEncodings(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, t.TempDir())
	s := New(db, "")
	const version = "bundled-encoding-fixture"
	document := packagedPricingFixture(t, version, "2")
	importBundled := func(data []byte) error {
		return s.importCatalog(ctx, data, "bundled", bundledCatalogSourceURL, "", s.now(), hash(data))
	}
	if err := importBundled(document); err != nil {
		t.Fatal(err)
	}
	execPricingSQL(t, db, "UPDATE price_rule_sets SET rules_json = ' ' || rules_json")
	revision, err := db.UsageProjectionMetadataRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := importBundled(document); err != nil {
		t.Errorf("same rule set with a different stored encoding: %v", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, document, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := importBundled(indented.Bytes()); err != nil {
		t.Errorf("same bundled content with a different document encoding: %v", err)
	}
	if err := importBundled(packagedPricingFixture(t, version, "3")); err == nil {
		t.Error("accepted changed rules with the existing version")
	}
	differentModels := bytes.Replace(document, []byte(`"input":"1"`), []byte(`"input":"4"`), 1)
	if err := importBundled(differentModels); err == nil {
		t.Error("accepted changed v1 rates with the existing version")
	}
	if got, err := db.UsageProjectionMetadataRevision(ctx); err != nil || got != revision {
		t.Fatalf("bundled retries changed metadata revision: %d, %v", got, err)
	}
	history, err := s.PriceHistory(ctx)
	if err != nil || len(history) != 1 || history[0].ContentSHA256 != hash(document) {
		t.Fatalf("bundled retries changed catalog provenance: %#v, %v", history, err)
	}
}

func TestPricingImportConcurrentVersionCollision(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, t.TempDir())
	s := New(db, "")
	const version = "concurrent"
	first, second := packagedPricingFixture(t, version, "2"), packagedPricingFixture(t, version, "3")
	type outcome struct {
		rate string
		err  error
	}
	start, results := make(chan struct{}), make(chan outcome, 2)
	for rate, document := range map[string][]byte{"2": first, "3": second} {
		go func() {
			<-start
			results <- outcome{rate, s.ImportOfficialCatalog(ctx, document, "https://openai.example/catalog")}
		}()
	}
	close(start)
	winner := ""
	for range 2 {
		result := <-results
		switch {
		case result.err == nil && winner == "":
			winner = result.rate
		case result.err != nil && strings.Contains(result.err.Error(), "different content"):
		default:
			t.Fatalf("unexpected import result: %v", result)
		}
	}
	rules, err := s.CatalogPriceRules(ctx, version)
	if err != nil || winner == "" || rules == nil || rules.Rules[0].Rates["input"] != winner {
		t.Fatalf("stored rule does not match transaction winner %q: %v, %v", winner, rules, err)
	}
	assertPricingCount(t, db, "model_prices", 1)
}

func packagedPricingFixture(t *testing.T, version, rate string) []byte {
	t.Helper()
	rules, err := DecodePriceRules(marshalPriceRules(t, fixturePriceRule("import-fixture", rate)))
	if err != nil {
		t.Fatal(err)
	}
	document := struct {
		catalog
		Rules PriceRuleSet `json:"pricing_rules"`
	}{
		catalog: catalog{
			SchemaVersion: 1, Version: version, Currency: "USD",
			Models: map[string]modelPrice{"gpt-example": {
				Provider: "openai", EffectiveFrom: "2026-01-01T00:00:00Z", Prices: map[string]string{"input": "1"},
			}},
		},
		Rules: rules,
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
