package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/watch"
)

// usePinnedDisplayZone fixes the zone human-readable output uses, including the
// zone usage reports resolve their local dates in, for one test.
//
// `usage stats --from/--to` take local dates by design (cli-manual.md: 本机
// 日期), so the window they resolve to depends on where the machine is. Tests
// whose fixtures carry UTC timestamps near the start of a day are otherwise
// silently sensitive to that: on a host west of UTC the local-day window opens
// hours later and drops those events, and the test fails for a reason that has
// nothing to do with what it asserts.
//
// It swaps the displayLocation seam rather than time.Local, matching how this
// package already pins userHomeDir and runClientProcesses. The seam exists
// because there is no other in-process way to pin the zone: TZ is read once,
// the first time the process resolves time.Local, so setting the environment
// variable from inside a test is too late.
func usePinnedDisplayZone(t *testing.T, location *time.Location) {
	t.Helper()
	previous := displayLocation
	displayLocation = func() *time.Location { return location }
	t.Cleanup(func() { displayLocation = previous })
}

func useUTCDisplayClock(t *testing.T) {
	t.Helper()
	usePinnedDisplayZone(t, time.UTC)
}

// TestUsageStatsResolvesRangeDatesInTheDisplayZone pins the behavior that made
// the two tests above zone-sensitive, so it stays a deliberate contract rather
// than a trap: --from/--to name local dates, not UTC dates.
//
// The sample instant is what makes this a real guard. 2026-07-20T16:00:00Z is
// 2026-07-21 00:00 in UTC+8 but still 2026-07-20 in UTC, so the two readings
// disagree about which day owns it: only local-date semantics put it in the
// 21st's window and leave the 20th's empty. An instant that landed on the same
// date in both frames — 15:00Z, say — would pass under either reading and
// guard nothing.
func TestUsageStatsResolvesRangeDatesInTheDisplayZone(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	state := filepath.Join(t.TempDir(), "state")
	if err := run([]string{"--state-dir", state, "state", "migrate"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	seedUsageEventForRangeTest(t, state, "2026-07-20T16:00:00Z")

	localDay := usageStatsEventCount(t, state, "2026-07-21", "2026-07-21")
	if localDay != 1 {
		t.Fatalf("events in the local day (2026-07-21 in UTC+8) = %d, want 1", localDay)
	}
	utcDay := usageStatsEventCount(t, state, "2026-07-20", "2026-07-20")
	if utcDay != 0 {
		t.Fatalf("events counted in the UTC day (2026-07-20) = %d, want 0", utcDay)
	}
}

// TestDisplayLocationDefaultsToTheMachineZone covers the half of the contract
// the test above cannot: that one pins the seam, so it proves dates are read
// in whatever zone is configured, not that the configured zone is the
// machine's. Human-readable output is local by design, so the default is part
// of the contract and not an implementation detail.
func TestDisplayLocationDefaultsToTheMachineZone(t *testing.T) {
	if displayLocation() != time.Local {
		t.Fatalf("displayLocation() = %v, want the machine zone", displayLocation())
	}
}

func TestRenderDisplayTime(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	t.Run("UTC input", func(t *testing.T) {
		for _, input := range []struct {
			name string
			got  string
		}{
			{name: "stored string", got: renderDisplayTime("2026-07-20T16:00:00Z")},
			{name: "time.Time", got: renderDisplayTime(time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC))},
		} {
			if input.got != "2026-07-21 00:00:00" {
				t.Errorf("%s = %q", input.name, input.got)
			}
		}
	})
	t.Run("fractional-second input", func(t *testing.T) {
		input := "2026-07-20T16:00:00.123456789Z"
		if got := renderDisplayTime(input); got != "2026-07-21 00:00:00" {
			t.Fatalf("renderDisplayTime(%q) = %q", input, got)
		}
	})
	t.Run("empty string", func(t *testing.T) {
		if got := renderDisplayTime(""); got != "" {
			t.Fatalf("renderDisplayTime(empty) = %q", got)
		}
	})
	t.Run("non-timestamp string", func(t *testing.T) {
		input := "not-a-timestamp"
		if got := renderDisplayTime(input); got != input {
			t.Fatalf("renderDisplayTime(%q) = %q", input, got)
		}
	})
}

func TestProviderAndSessionTextSurfacesUseDisplayZone(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	usePinnedDisplayZone(t, location)

	const (
		stored    = "2026-07-20T16:00:00Z"
		localized = "2026-07-21 00:00:00"
	)
	cases := []struct {
		name    string
		command string
		data    any
		want    []string
	}{
		{
			name:    "provider current",
			command: "provider.current",
			data: []provider.CurrentSelection{{
				Client:     "codex",
				Provider:   "example",
				SelectedAt: stored,
			}},
			want: []string{"SELECTED AT (UTC+8)", localized},
		},
		{
			name:    "provider status detail",
			command: "provider.status",
			data: provider.Status{
				Definition: provider.Provider{Name: "example"},
				Active: []provider.ActiveSelection{{
					Client:     "codex",
					SelectedAt: stored,
				}},
			},
			want: []string{"SELECTED AT (UTC+8)", localized},
		},
		{
			name:    "session list",
			command: "session.list",
			data: []session.Metadata{{
				Client:    "codex",
				SessionID: "session-1",
				FirstAt:   stored,
				LastAt:    stored,
			}},
			want: []string{"FIRST (UTC+8)", "LAST (UTC+8)", localized},
		},
		{
			name:    "session show and activity",
			command: "session.show",
			data: session.Result{
				Metadata: session.Metadata{
					Client:    "codex",
					SessionID: "session-1",
					FirstAt:   stored,
					LastAt:    stored,
				},
				Activity: []activity.Detail{{
					StartedAt: stored,
					Tool:      "exec_command",
					Status:    "completed",
				}},
			},
			want: []string{
				"first: " + localized + " UTC+8",
				"last: " + localized + " UTC+8",
				"STARTED (UTC+8)",
				localized,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var text bytes.Buffer
			if err := writeResult(&text, "text", test.command, test.data); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(text.String(), want) {
					t.Errorf("text output missing %q:\n%s", want, text.String())
				}
			}
			if strings.Contains(text.String(), stored) {
				t.Errorf("text output kept stored UTC instant:\n%s", text.String())
			}

			var encoded bytes.Buffer
			if err := writeResult(&encoded, "json", test.command, test.data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(encoded.String(), stored) {
				t.Errorf("JSON output changed stored instant:\n%s", encoded.String())
			}
			if strings.Contains(encoded.String(), localized) {
				t.Errorf("JSON output contains localized instant:\n%s", encoded.String())
			}
		})
	}
}

func TestSessionSearchTextShowsZoneHeaderAndDashWithoutInstant(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	var text bytes.Buffer
	if err := writeResult(&text, "text", "session.search", []session.Document{{
		Client:    "codex",
		SessionID: "session-1",
		Kind:      "user_prompt",
		Text:      "visible search result",
	}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CLIENT", "SESSION", "EVENT AT (UTC+8)", "KIND", "TEXT", "—", "visible search result"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("search text missing %q:\n%s", want, text.String())
		}
	}
}

func TestSessionShowLeavesInvalidDisplayTimesUnchanged(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	var text bytes.Buffer
	if err := writeResult(&text, "text", "session.show", session.Result{
		Metadata: session.Metadata{
			Client:    "codex",
			SessionID: "session-1",
			FirstAt:   "not-a-timestamp",
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"first: not-a-timestamp\n", "last: \n"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("session show missing unchanged value %q:\n%s", want, text.String())
		}
	}
	for _, unwanted := range []string{"not-a-timestamp UTC+8", "last:  UTC+8"} {
		if strings.Contains(text.String(), unwanted) {
			t.Errorf("session show added a zone to invalid value %q:\n%s", unwanted, text.String())
		}
	}
}

func TestBackupAndPriceTextSurfacesUseDisplayZone(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	instant := time.Date(2026, 7, 20, 16, 0, 0, 123456789, time.UTC)
	stored := instant.Format(time.RFC3339Nano)
	const localized = "2026-07-21 00:00:00"
	manifest := backup.Manifest{
		SchemaVersion:    3,
		AgentDeckVersion: "v1.2.3",
		CreatedAt:        instant,
		SourcePlatform:   "darwin/arm64",
	}
	catalogs := []usage.PriceCatalog{{
		Version:       "catalog-1",
		SourceKind:    "official",
		EffectiveFrom: stored,
		Models:        1,
		Components:    2,
	}}
	cases := []struct {
		name    string
		command string
		data    any
		want    []string
	}{
		{
			name:    "backup create",
			command: "backup.create",
			data:    map[string]any{"path": "/tmp/backup.adb", "manifest": manifest},
			want: []string{
				`Completed backup.create for "/tmp/backup.adb".`,
				"created: " + localized + " UTC+8",
			},
		},
		{
			name:    "backup list",
			command: "backup.list",
			data: []backup.FileInfo{{
				Path:       "/tmp/backup.adb",
				Size:       123,
				ModifiedAt: instant,
			}},
			want: []string{"MODIFIED (UTC+8)", localized},
		},
		{
			name:    "backup inspect",
			command: "backup.inspect",
			data:    manifest,
			want:    []string{"created: " + localized + " UTC+8"},
		},
		{
			name:    "price history",
			command: "price.history",
			data:    catalogs,
			want:    []string{"EFFECTIVE (UTC+8)", localized},
		},
		{
			name:    "price status catalogs",
			command: "price.status",
			data: map[string]any{
				"available":  true,
				"models":     1,
				"components": 2,
				"catalogs":   catalogs,
			},
			want: []string{"EFFECTIVE (UTC+8)", localized},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var text bytes.Buffer
			if err := writeResult(&text, "text", test.command, test.data); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(text.String(), want) {
					t.Errorf("text output missing %q:\n%s", want, text.String())
				}
			}
			if strings.Contains(text.String(), stored) {
				t.Errorf("text output kept stored UTC instant:\n%s", text.String())
			}

			var encoded bytes.Buffer
			if err := writeResult(&encoded, "json", test.command, test.data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(encoded.String(), stored) {
				t.Errorf("JSON output changed stored instant:\n%s", encoded.String())
			}
			if strings.Contains(encoded.String(), localized) {
				t.Errorf("JSON output contains localized instant:\n%s", encoded.String())
			}
		})
	}
}

func TestInstantBearingTextRendererSweepUsesDisplayZone(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))

	instant := time.Date(2026, 7, 20, 16, 0, 0, 123456789, time.UTC)
	stored := instant.Format(time.RFC3339Nano)
	const localized = "2026-07-21 00:00:00"

	t.Run("price list verbose provenance", func(t *testing.T) {
		prices := []usage.EffectivePrice{{
			Provider: "openai",
			Model:    "gpt-test",
			Unit:     "USD / 1M tokens",
			Prices:   map[string]string{"input": "1"},
			Provenance: map[string]usage.PriceProvenance{
				"input": {
					CatalogVersion: "catalog-1",
					SourceKind:     "official",
					EffectiveFrom:  stored,
				},
			},
		}}
		var text bytes.Buffer
		if err := writePriceEnvelope(&text, "text", "price.list", prices, false, nil, false, true); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"EFFECTIVE (UTC+8)", localized} {
			if !strings.Contains(text.String(), want) {
				t.Errorf("price list verbose text missing %q:\n%s", want, text.String())
			}
		}
		if strings.Contains(text.String(), stored) {
			t.Errorf("price list verbose text kept stored UTC instant:\n%s", text.String())
		}

		var encoded bytes.Buffer
		if err := writePriceEnvelope(&encoded, "json", "price.list", prices, false, nil, false, true); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(encoded.String(), stored) || strings.Contains(encoded.String(), localized) {
			t.Errorf("price list JSON changed stored instant:\n%s", encoded.String())
		}
	})

	t.Run("usage sessions", func(t *testing.T) {
		sessions := []usage.SessionSummary{{
			Client:    "codex",
			SessionID: "session-1",
			FirstAt:   stored,
			LastAt:    stored,
			Tokens:    map[string]int64{},
		}}
		var text bytes.Buffer
		if err := writeUsageEnvelope(&text, "text", "usage.sessions", sessions, false, nil, false, usageTextRenderOptions{}); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"FIRST (UTC+8)", "LAST (UTC+8)", localized} {
			if !strings.Contains(text.String(), want) {
				t.Errorf("usage sessions text missing %q:\n%s", want, text.String())
			}
		}
		if strings.Contains(text.String(), stored) {
			t.Errorf("usage sessions text kept stored UTC instant:\n%s", text.String())
		}

		var encoded bytes.Buffer
		if err := writeUsageEnvelope(&encoded, "json", "usage.sessions", sessions, false, nil, false, usageTextRenderOptions{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(encoded.String(), stored) || strings.Contains(encoded.String(), localized) {
			t.Errorf("usage sessions JSON changed stored instant:\n%s", encoded.String())
		}
	})

	t.Run("watch", func(t *testing.T) {
		event := watch.Event{
			SchemaVersion: watch.EventSchemaVersion,
			Type:          "scan_completed",
			Domain:        "session",
			GeneratedAt:   instant,
			Changes:       1,
		}
		var text bytes.Buffer
		if err := renderWatchText(&text, event); err != nil {
			t.Fatal(err)
		}
		if want := localized + " UTC+8"; !strings.Contains(text.String(), want) {
			t.Errorf("watch text missing %q:\n%s", want, text.String())
		}
		if strings.Contains(text.String(), stored) {
			t.Errorf("watch text kept stored UTC instant:\n%s", text.String())
		}

		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(event); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(encoded.String(), stored) || strings.Contains(encoded.String(), localized) {
			t.Errorf("watch NDJSON changed stored instant:\n%s", encoded.String())
		}
	})

	t.Run("usage stats model activity range", func(t *testing.T) {
		last := "2026-07-20T18:30:00.987654321Z"
		report := usage.StatsReport{Models: []usage.StatsDimension{{
			Name:     "gpt-test",
			Activity: &usage.StatsModelActivity{FirstAt: stored, LastAt: last},
		}}}
		renderer := statsTextRenderer{report: report, width: 120}
		text := strings.Join(renderer.modelActivityLines(report.Models[0]), "\n")
		if want := "range " + localized + " - 2026-07-21 02:30:00 UTC+8"; !strings.Contains(text, want) {
			t.Errorf("model activity range missing %q:\n%s", want, text)
		}
		if strings.Contains(text, stored) || strings.Contains(text, last) {
			t.Errorf("model activity range kept a stored UTC instant:\n%s", text)
		}
	})

	t.Run("usage stats model activity keeps unparseable range unchanged", func(t *testing.T) {
		report := usage.StatsReport{Models: []usage.StatsDimension{{
			Name:     "gpt-test",
			Activity: &usage.StatsModelActivity{FirstAt: "not-a-timestamp", LastAt: stored},
		}}}
		renderer := statsTextRenderer{report: report, width: 120}
		text := strings.Join(renderer.modelActivityLines(report.Models[0]), "\n")
		if want := "range not-a-timestamp - " + stored; !strings.Contains(text, want) {
			t.Errorf("model activity range changed an unparseable value:\n%s", text)
		}
		if strings.Contains(text, "UTC+8") {
			t.Errorf("model activity range named a zone for an unparseable value:\n%s", text)
		}
	})
}

func seedUsageEventForRangeTest(t *testing.T, state, at string) {
	t.Helper()
	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(context.Background(), `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES ('codex','zone-session',?,?);
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('zone','codex','zone-session','zone',?,'gpt-test',10,0,'fixture',0)`, at, at, at); err != nil {
		t.Fatal(err)
	}
}

func usageStatsEventCount(t *testing.T, state, from, to string) int64 {
	t.Helper()
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "stats", "--from", from, "--to", to, "--no-scan"}, bytes.NewReader(nil), &stdout); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Totals struct {
				Events int64 `json:"events"`
			} `json:"totals"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stats envelope = %q: %v", stdout.String(), err)
	}
	if strings.Contains(stdout.String(), "\"error\"") {
		t.Fatalf("stats reported an error: %s", stdout.String())
	}
	return envelope.Data.Totals.Events
}
