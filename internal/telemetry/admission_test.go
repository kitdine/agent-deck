package telemetry

import (
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func admittedRecord() Record {
	input, output := int64(11), int64(3)
	return Record{
		Producer: "codex_exec", ProducerVersion: "0.165.0", Originator: "codex_exec",
		EventName: "codex.sse_event", EventKind: "response.completed", Transport: "sse",
		RecordAt: "2026-10-29T12:00:00Z", ObservedAt: "2026-10-29T12:00:00Z", Model: "gpt-5.6-terra",
		ThreadID: "thread-1", InputTokens: &input, OutputTokens: &output,
	}
}

func observe(record Record) store.TelemetryObservation {
	return Observation(record, "receiver", time.Date(2026, 10, 29, 12, 0, 1, 0, time.UTC))
}

func TestAdmissionAcceptsEveryPresentVersionOfAVerifiedFamily(t *testing.T) {
	atLimit := strings.Repeat("9", maxFieldBytes)
	// 85 three-byte runes plus one ASCII byte is exactly the 256-byte limit.
	multibyteAtLimit := strings.Repeat("版", 85) + "1"
	for _, version := range []string{"0.165.0", "0.169.0", "0.170.0-alpha.3", "1.0.0+build.7", "nightly 2026_11_15", "版本九", atLimit, multibyteAtLimit} {
		for _, family := range [][2]string{{"codex_exec", "codex_exec"}, {"codex-app-server", "codex-tui"}} {
			record := admittedRecord()
			record.Producer, record.Originator, record.ProducerVersion = family[0], family[1], version
			got := observe(record)
			want := family[0] + "/" + version + "/" + family[1] + "/sse"
			if got.State != "pending" || got.Reason != ReasonOwnershipNotSettled || got.SupportEntry != want || !got.Metered || got.ProducerVersion != version {
				t.Errorf("%s version %q: state=%s reason=%s entry=%q metered=%v", family[0], version, got.State, got.Reason, got.SupportEntry, got.Metered)
			}
		}
	}
}

func TestAdmissionRejectsUnusableVersionsFamiliesAndShapes(t *testing.T) {
	cases := map[string]func(*Record){
		"empty version":         func(r *Record) { r.ProducerVersion = "" },
		"separator in version":  func(r *Record) { r.ProducerVersion = "0.169.0/x" },
		"version over limit":    func(r *Record) { r.ProducerVersion = strings.Repeat("9", maxFieldBytes+1) },
		"multibyte over limit":  func(r *Record) { r.ProducerVersion = strings.Repeat("版", 85) + "12" },
		"unknown producer":      func(r *Record) { r.Producer = "codex-desktop" },
		"unknown originator":    func(r *Record) { r.Originator = "Codex Desktop" },
		"families not crossed":  func(r *Record) { r.Originator = "codex-tui" },
		"other transport":       func(r *Record) { r.Transport = "websocket" },
		"missing transport":     func(r *Record) { r.Transport = "" },
		"other event name":      func(r *Record) { r.EventName = "codex.websocket_event" },
		"producer case differs": func(r *Record) { r.Producer = "Codex_Exec" },
	}
	for name, change := range cases {
		record := admittedRecord()
		change(&record)
		got := observe(record)
		if got.State != "capture_only" || got.Reason != ReasonProducerUnsupported || got.SupportEntry != "" {
			t.Errorf("%s: state=%s reason=%s entry=%q", name, got.State, got.Reason, got.SupportEntry)
		}
		if !got.Metered {
			t.Errorf("%s: a complete unsupported count must stay metered evidence", name)
		}
	}
}

// The states that precede admission keep their baseline outcome on an
// admitted family at a version the old exact table never listed.
func TestAdmissionKeepsEarlierOutcomesAtANewVersion(t *testing.T) {
	cases := []struct {
		name          string
		change        func(*Record)
		state, reason string
		metered       bool
	}{
		{"missing thread", func(r *Record) { r.ThreadID = "" }, "capture_only", ReasonConversationMissing, true},
		{"invalid count", func(r *Record) { r.Invalid = "negative_count" }, "invalid", "negative_count", false},
		{"no counts", func(r *Record) { r.InputTokens, r.OutputTokens = nil, nil }, "unmetered", "no_completed_usage", false},
		{"output missing", func(r *Record) { r.OutputTokens = nil }, "capture_only", store.TelemetryIncompleteCountsReason, false},
		{"input missing", func(r *Record) { r.InputTokens = nil }, "capture_only", store.TelemetryIncompleteCountsReason, false},
		{"other event kind", func(r *Record) { r.EventKind = "response.created" }, "unmetered", "no_completed_usage", false},
		{"invalid beats unsupported", func(r *Record) { r.Invalid, r.Producer = "negative_count", "codex-desktop" }, "invalid", "negative_count", false},
		{"unsupported beats missing thread", func(r *Record) { r.Producer, r.ThreadID = "codex-desktop", "" }, "capture_only", ReasonProducerUnsupported, true},
	}
	for _, c := range cases {
		record := admittedRecord()
		record.ProducerVersion = "0.169.0"
		c.change(&record)
		got := observe(record)
		if got.State != c.state || got.Reason != c.reason || got.Metered != c.metered || got.SupportEntry != "" {
			t.Errorf("%s: state=%s reason=%s metered=%v entry=%q", c.name, got.State, got.Reason, got.Metered, got.SupportEntry)
		}
	}
}

// The delivery key is the journal and observation identity. Its bytes are
// pinned so admission changes can never re-key already stored records.
func TestRecordKeyBytesAreUnchanged(t *testing.T) {
	record := admittedRecord()
	if got := record.Key(); got != baselineKey0165 {
		t.Fatalf("0.165.0 key = %s", got)
	}
	record.ProducerVersion = "0.169.0"
	cached := int64(2)
	record.CachedTokens = &cached
	if got := record.Key(); got != baselineKey0169 {
		t.Fatalf("0.169.0 key = %s", got)
	}
	if observe(record).Key != baselineKey0169 {
		t.Fatal("observation key differs from the record key")
	}
}

func TestAdmissionVersionBoundIsTheDecodeFieldLimit(t *testing.T) {
	if store.TelemetryProducerVersionMaxSize != maxFieldBytes {
		t.Fatalf("version bound %d, decode field limit %d", store.TelemetryProducerVersionMaxSize, maxFieldBytes)
	}
}

// A newer version flows from an OTLP body to a settled, counted conversation.
func TestNewerVersionSettlesAndCountsOnce(t *testing.T) {
	database, _ := openStore(t)
	for _, c := range []struct{ thread, service, originator, version string }{
		{"nv-exec", "codex_exec", "codex_exec", "0.169.0"},
		{"nv-tui", "codex-app-server", "codex-tui", "0.170.0-alpha.3"},
	} {
		fields := completed(c.thread, "1000", "10")
		for i := range fields {
			if fields[i].key == "originator" {
				fields[i] = s("originator", c.originator)
			}
		}
		body := exportBody([]attr{s("service.name", c.service), s("service.version", c.version)}, fields)
		importRecords(t, database, body)
		importRecords(t, database, body)
		assertOwnership(t, database, c.thread, ownership{states: map[string]int{"pending": 1}})
	}
	recordInventory(t, database, "inv-nv", receivedAt.Add(time.Minute))
	settle(t, database, receivedAt.Add(time.Hour))
	settle(t, database, receivedAt.Add(2*time.Hour))
	for _, thread := range []string{"nv-exec", "nv-tui"} {
		got := ownershipOf(t, database, thread)
		if got.states["otel_owned"] != 1 || len(got.states) != 1 {
			t.Fatalf("%s states = %v", thread, got.states)
		}
	}
}

// Keys of admittedRecord at 0.165.0, and at 0.169.0 with two cached tokens.
// Record and Key are unchanged from baseline 1b2faeb, where these were taken.
const (
	baselineKey0165 = "8b7149774d115fee49abd3270206cf3fcf7adf6b735e0a105713b25bd1ee89de"
	baselineKey0169 = "1c4247555e2e81ab3926cf024ad871beef3fb69679e9a71108903ab78565d581"
)
