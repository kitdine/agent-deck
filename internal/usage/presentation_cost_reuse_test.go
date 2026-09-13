package usage

import (
	"reflect"
	"testing"
)

func TestPresentationParsedCostsMatchOriginalWithoutMutatingInputs(t *testing.T) {
	for _, client := range []string{"codex", "claude"} {
		event := storedEvent{Event: Event{Client: client, SessionID: "s", Tokens: map[string]int64{"input_tokens": 10, "output_tokens": 3, "cached_input_tokens": 4, "cache_read_tokens": 5, "cache_creation_tokens": 6}}}
		baseText, providerText := "0.100000001", "0.200000003"
		result := Result{KnownCatalogBaseCost: baseText, KnownProviderCost: providerText, CatalogBaseCost: &baseText, ProviderCost: &providerText}
		base, err := decimal(baseText)
		if err != nil {
			t.Fatal(err)
		}
		provider, err := decimal(providerText)
		if err != nil {
			t.Fatal(err)
		}
		beforeBase, beforeProvider := base.RatString(), provider.RatString()
		old, new := newPresentationAccumulator(), newPresentationAccumulator()
		for i := 0; i < 20; i++ {
			if i == 10 {
				result.CatalogBaseCost = nil
				result.ProviderCost = nil
				result.Unpriced = []string{"input"}
			}
			if err := old.add(event, result); err != nil {
				t.Fatal(err)
			}
			if err := new.add(event, result, base, provider); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(old, new) {
			t.Fatalf("%s parsed costs changed accumulation", client)
		}
		if base.RatString() != beforeBase || provider.RatString() != beforeProvider {
			t.Fatal("shared input costs were mutated")
		}
	}
}
