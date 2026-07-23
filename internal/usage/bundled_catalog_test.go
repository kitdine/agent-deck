package usage

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBundledCatalogVersionCarriesItsContentDigest is the shipping guard: the
// catalog compiled into this binary must carry the content digest its own
// contents require. It fails whenever model-prices.json is edited without
// restamping catalog_version, which is exactly the case that would otherwise
// leave stale prices and a stale content_sha256 on every upgraded install
// (importCatalog uses INSERT OR IGNORE keyed on the version).
func TestBundledCatalogVersionCarriesItsContentDigest(t *testing.T) {
	if err := VerifyBundledCatalog(); err != nil {
		t.Fatalf("bundled catalog version guard: %v", err)
	}
}

// TestBundledCatalogVersionGuardRejectsChangedPriceWithUnchangedVersion is the
// regression the plan asks for: mutate a bundled price, keep the version, and
// require a loud failure rather than a silent stale-price ship.
func TestBundledCatalogVersionGuardRejectsChangedPriceWithUnchangedVersion(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(bundledCatalog, &document); err != nil {
		t.Fatal(err)
	}
	models, ok := document["models"].(map[string]any)
	if !ok || len(models) == 0 {
		t.Fatalf("bundled catalog has no models: %#v", document["models"])
	}
	var name string
	for candidate := range models {
		if name == "" || candidate < name {
			name = candidate
		}
	}
	model, ok := models[name].(map[string]any)
	if !ok {
		t.Fatalf("model %q is not an object: %#v", name, models[name])
	}
	prices, ok := model["prices_per_million"].(map[string]any)
	if !ok {
		t.Fatalf("model %q has no prices_per_million: %#v", name, model)
	}
	prices["input"] = "999999"

	mutated, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyBundledCatalogVersion(mutated)
	if err == nil {
		t.Fatal("mutating a bundled price with an unchanged version passed the guard")
	}
	if !strings.Contains(err.Error(), "stale content digest") {
		t.Fatalf("guard rejected for the wrong reason: %v", err)
	}

	// Restamping the version for the mutated contents makes it valid again,
	// so the guard demands a version bump rather than forbidding the edit.
	prefix, _, _ := strings.Cut(document["catalog_version"].(string), "-g")
	restamped, err := BundledCatalogVersionFor(mutated, prefix)
	if err != nil {
		t.Fatal(err)
	}
	document["catalog_version"] = restamped
	bumped, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyBundledCatalogVersion(bumped); err != nil {
		t.Fatalf("restamped catalog still rejected: %v", err)
	}
	if restamped == document["catalog_version"].(string) && !strings.HasSuffix(restamped, "-g") && len(restamped) <= len(prefix)+2 {
		t.Fatalf("restamped version %q carries no digest", restamped)
	}
}

// TestBundledCatalogVersionDigestIgnoresFormattingButTracksContent pins the
// normalization choice: reformatting the file must not demand a version bump,
// while any semantic change must.
func TestBundledCatalogVersionDigestIgnoresFormattingButTracksContent(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(bundledCatalog, &document); err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	indented, err := json.MarshalIndent(document, "", "      ")
	if err != nil {
		t.Fatal(err)
	}
	compactDigest, err := BundledCatalogVersionDigest(compact)
	if err != nil {
		t.Fatal(err)
	}
	indentedDigest, err := BundledCatalogVersionDigest(indented)
	if err != nil {
		t.Fatal(err)
	}
	embeddedDigest, err := BundledCatalogVersionDigest(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if compactDigest != indentedDigest || compactDigest != embeddedDigest {
		t.Fatalf("digest changed with formatting: embedded=%s compact=%s indented=%s", embeddedDigest, compactDigest, indentedDigest)
	}

	// The version string itself must not feed the digest it carries.
	document["catalog_version"] = "totally-different-version"
	renamed, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	renamedDigest, err := BundledCatalogVersionDigest(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if renamedDigest != embeddedDigest {
		t.Fatalf("digest depends on the version string it stamps: %s vs %s", renamedDigest, embeddedDigest)
	}

	// A semantic change must move the digest.
	models := document["models"].(map[string]any)
	models["a-newly-added-model"] = map[string]any{
		"provider": "openai", "effective_from": "2026-01-01T00:00:00Z",
		"prices_per_million": map[string]any{"input": "1", "output": "2"},
	}
	changed, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	changedDigest, err := BundledCatalogVersionDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == embeddedDigest {
		t.Fatal("adding a model did not change the content digest")
	}
}
