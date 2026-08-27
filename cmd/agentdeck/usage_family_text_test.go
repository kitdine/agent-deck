package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsageSessionsResponsiveColumnRanking(t *testing.T) {
	sessionID := strings.Repeat("session-", 10)
	values := []usage.SessionSummary{{
		Client:    "codex",
		SessionID: sessionID,
		FirstAt:   "2026-07-16T00:00:00Z",
		LastAt:    "2026-07-16T00:01:00Z",
		Tokens: map[string]int64{
			"input_tokens":          10,
			"cached_input_tokens":   3,
			"output_tokens":         2,
			"cache_read_tokens":     4,
			"cache_creation_tokens": 5,
			"cache_write_5m_tokens": 6,
			"cache_write_1h_tokens": 7,
		},
	}}

	var narrow bytes.Buffer
	if err := renderUsageTextWithOptions(&narrow, "usage.sessions", values, usageTextRenderOptions{width: 48}); err != nil {
		t.Fatal(err)
	}
	narrowText := narrow.String()
	for _, line := range strings.Split(strings.TrimSuffix(narrowText, "\n"), "\n") {
		if width := statsVisibleWidth(line); width > 48 {
			t.Fatalf("narrow session line width %d exceeds 48: %q", width, line)
		}
	}
	if strings.Index(narrowText, "STATUS") > strings.Index(narrowText, "↳ CACHE READ") || !strings.Contains(narrowText, "INPUT") || !strings.Contains(narrowText, "OUTPUT") {
		t.Fatalf("narrow layout does not retain ranked primary columns before continuation:\n%s", narrowText)
	}
	if !strings.Contains(strings.ReplaceAll(narrowText, "\n  ", ""), sessionID) {
		t.Fatalf("narrow layout did not preserve the complete session ID:\n%s", narrowText)
	}

	var wide bytes.Buffer
	if err := renderUsageTextWithOptions(&wide, "usage.sessions", values, usageTextRenderOptions{width: 160}); err != nil {
		t.Fatal(err)
	}
	wideText := wide.String()
	for _, line := range strings.Split(strings.TrimSuffix(wideText, "\n"), "\n") {
		if width := statsVisibleWidth(line); width > 160 {
			t.Fatalf("wide session line width %d exceeds 160: %q", width, line)
		}
	}
	if !strings.Contains(wideText, sessionID) {
		t.Fatalf("wide layout did not preserve the complete session ID:\n%s", wideText)
	}
	for _, label := range []string{"CACHE READ", "CACHE CREATE", "WRITE 5M", "WRITE 1H"} {
		if !strings.Contains(wideText, label) {
			t.Fatalf("wide layout omitted %q:\n%s", label, wideText)
		}
	}
	if strings.Contains(wideText, "↳ ") {
		t.Fatalf("wide layout unexpectedly used narrow continuation marker:\n%s", wideText)
	}
}

func TestUsageFamilyTextPathsPreserveJSONData(t *testing.T) {
	summary := usage.Summary{Tokens: map[string]int64{"input_tokens": 10}, Counts: map[string]int64{"events": 1}}
	sessions := []usage.SessionSummary{{Client: "codex", SessionID: "session-1", Tokens: map[string]int64{"input_tokens": 10}}}
	diagnose := map[string]any{"files": 1, "events": 2, "sessions": 3, "exact_runs": 4}
	for _, test := range []struct {
		command string
		data    any
	}{
		{command: "usage.summary", data: summary},
		{command: "usage.sessions", data: sessions},
		{command: "usage.diagnose", data: diagnose},
	} {
		t.Run(test.command, func(t *testing.T) {
			var rendered bytes.Buffer
			if err := writeUsageEnvelope(&rendered, "json", test.command, test.data, false, nil, false, usageTextRenderOptions{width: 48}); err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rendered.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(test.data)
			if err != nil {
				t.Fatal(err)
			}
			if string(envelope.Data) != string(want) {
				t.Fatalf("JSON data changed by text renderer:\n got: %s\nwant: %s", envelope.Data, want)
			}
		})
	}
}

func TestUsageSummaryOmitsEmptyAttributionReasonsSection(t *testing.T) {
	var rendered bytes.Buffer
	if err := renderUsageText(&rendered, "usage.summary", usage.Summary{Tokens: map[string]int64{}, Counts: map[string]int64{}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "ATTRIBUTION REASONS") {
		t.Fatalf("empty summary rendered an empty attribution-reasons section:\n%s", rendered.String())
	}
}
