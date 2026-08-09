package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

func renderRouteStatsText(t *testing.T, providers []usage.StatsDimension) string {
	t.Helper()
	report := usageStatsTextFixture()
	report.Providers = providers
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

// TestUsageStatsTextReportsWrapperRouteOnlyWhenOneCarriedEvents pins both
// halves of the route's text surface: it annotates the provider row it belongs
// to, and a report with no wrapped events renders byte-for-byte as before.
func TestUsageStatsTextReportsWrapperRouteOnlyWhenOneCarriedEvents(t *testing.T) {
	share := "100.00"
	direct := []usage.StatsDimension{{
		Name: "relay", Client: "codex", Tokens: 30, Events: 2, Sessions: 2,
		KnownProviderCost: "0", KnownMetricValue: "30", Share: &share, KnownShare: share, Coverage: "priced",
	}}
	wrapped := []usage.StatsDimension{direct[0]}
	wrapped[0].WrapperEvents = 1

	withoutRoute := renderRouteStatsText(t, direct)
	if strings.Contains(withoutRoute, "via wrapper") {
		t.Fatalf("route annotation appeared with no wrapped events:\n%s", withoutRoute)
	}
	withRoute := renderRouteStatsText(t, wrapped)
	if !strings.Contains(withRoute, "1 via wrapper") {
		t.Fatalf("route annotation missing from the provider row:\n%s", withRoute)
	}

	// The annotation is additive: removing it recovers the original rendering,
	// so no existing line moved or changed to make room for it.
	if strings.ReplaceAll(withRoute, "\n↳ 1 via wrapper", "") != withoutRoute {
		t.Fatalf("route annotation changed the surrounding rendering:\nwith:\n%s\nwithout:\n%s", withRoute, withoutRoute)
	}
}
