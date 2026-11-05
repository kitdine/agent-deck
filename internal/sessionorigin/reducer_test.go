package sessionorigin

import (
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func auxiliaryEvidence(rule string) Evidence {
	return Evidence{Adapter: "a", RuleVersion: rule, Purpose: PurposeAuxiliary, SourceToken: "t"}
}

func TestReduceAppliesTheClosedReasonEnumInOrder(t *testing.T) {
	work := Evidence{Adapter: "p", RuleVersion: "parser", Purpose: PurposeWork, SourceToken: "w"}
	cases := []struct {
		name string
		in   Assessment
		want Result
	}{
		{"no evidence", Assessment{}, Result{ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion}},
		{"codex maintenance", Assessment{Evidence: []Evidence{auxiliaryEvidence(RuleCodexOTelBackgroundV1)}}, Result{ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1}},
		{"claude-mem maintenance", Assessment{Evidence: []Evidence{auxiliaryEvidence(RuleClaudeMemSDKMapV1)}}, Result{ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1}},
		{"maintenance and work", Assessment{Evidence: []Evidence{auxiliaryEvidence(RuleClaudeMemSDKMapV1), work}}, Result{ClassUnknown, ReasonConflictingOrigin, ReducerVersion}},
		{"mixed use or cycle", Assessment{Evidence: []Evidence{auxiliaryEvidence(RuleClaudeMemSDKMapV1)}, Conflicting: true}, Result{ClassUnknown, ReasonConflictingOrigin, ReducerVersion}},
		{"work alone is not a proven work class", Assessment{Evidence: []Evidence{work}}, Result{ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion}},
		{"token mismatch beats maintenance", Assessment{Evidence: []Evidence{auxiliaryEvidence(RuleCodexOTelBackgroundV1)}, InputMismatch: true}, Result{ClassUnknown, ReasonOriginInputMismatch, ReducerVersion}},
		{"source unavailable", Assessment{Unavailable: RuleClaudeMemSDKMapV1}, Result{ClassUnknown, ReasonOriginSourceUnavailable, RuleClaudeMemSDKMapV1}},
		{"rule unsupported", Assessment{Unsupported: RuleCodexOTelBackgroundV1}, Result{ClassUnknown, ReasonOriginRuleUnsupported, RuleCodexOTelBackgroundV1}},
		{"task child", Assessment{TaskChild: true}, Result{ClassUnknown, ReasonTaskChildUnconfirmed, ReducerVersion}},
		{"task child with maintenance evidence", Assessment{TaskChild: true, Evidence: []Evidence{auxiliaryEvidence(RuleClaudeMemSDKMapV1)}}, Result{ClassAuxiliary, ReasonMaintenanceOrigin, RuleClaudeMemSDKMapV1}},
	}
	for _, c := range cases {
		got := Reduce(c.in)
		if got != c.want {
			t.Fatalf("%s: %+v, want %+v", c.name, got, c.want)
		}
		if class, ok := AutomaticForReason(got.ReasonCode); !ok || class != got.Automatic {
			t.Fatalf("%s: reason %s outside the closed enum or class %s", c.name, got.ReasonCode, got.Automatic)
		}
	}
}

func TestEffectiveAppliesOnlyMatchedOverrides(t *testing.T) {
	work, auxiliary := "work", "auxiliary"
	if got, overridden := Effective(ClassUnknown, nil); got != ClassUnknown || overridden {
		t.Fatal("no override")
	}
	if got, overridden := Effective(ClassAuxiliary, &store.OriginOverride{Value: &work, BindingState: BindingMatched}); got != ClassWork || !overridden {
		t.Fatal("matched override must win")
	}
	if got, _ := Effective(ClassUnknown, &store.OriginOverride{Value: &auxiliary, BindingState: BindingUnmatched}); got != ClassUnknown {
		t.Fatal("unmatched override must not classify")
	}
	if got, _ := Effective(ClassAuxiliary, &store.OriginOverride{BindingState: BindingMatched}); got != ClassAuxiliary {
		t.Fatal("tombstone must not classify")
	}
}

func TestInputTokenEncodingIsCanonical(t *testing.T) {
	a := SourceGeneration{Identity: "dev:1/ino:2", PrefixHash: "p1", ParserVersion: 9}
	b := SourceGeneration{Identity: "dev:1/ino:3", PrefixHash: "p2", ParserVersion: 9}
	base := InputToken("claude", "s1", []SourceGeneration{a, b}, nil)
	if len(base) != 64 {
		t.Fatalf("token %q", base)
	}
	if InputToken("claude", "s1", []SourceGeneration{b, a, a}, nil) != base {
		t.Fatal("copy order or duplicates changed the token")
	}
	// Append and move keep identity and prefix: the token stays.
	if InputToken("claude", "s1", []SourceGeneration{a, b}, nil) != base {
		t.Fatal("same inputs, different token")
	}
	changed := map[string]string{
		"rewrite":          InputToken("claude", "s1", []SourceGeneration{{Identity: a.Identity, PrefixHash: "p1-rewritten", ParserVersion: 9}, b}, nil),
		"new copy":         InputToken("claude", "s1", []SourceGeneration{a, b, {Identity: "otel:claude/s1"}}, nil),
		"parser version":   InputToken("claude", "s1", []SourceGeneration{{Identity: a.Identity, PrefixHash: "p1", ParserVersion: 10}, b}, nil),
		"other client":     InputToken("codex", "s1", []SourceGeneration{a, b}, nil),
		"other session":    InputToken("claude", "s2", []SourceGeneration{a, b}, nil),
		"parser evidence":  InputToken("claude", "s1", []SourceGeneration{a, b}, []Evidence{{RuleVersion: "parser-v1", Purpose: PurposeWork, SourceToken: "x"}}),
		"field separation": InputToken("claude", "s1", []SourceGeneration{{Identity: a.Identity + "|p1", PrefixHash: "", ParserVersion: 9}, b}, nil),
	}
	seen := map[string]string{base: "base"}
	for name, token := range changed {
		if other, dup := seen[token]; dup {
			t.Fatalf("%s collides with %s", name, other)
		}
		seen[token] = name
	}
}

func TestBindingUsesStableBirthOnly(t *testing.T) {
	copies := []store.SessionOriginSource{
		{Client: "claude", SessionID: "s1", Identity: "a", FirstAt: "2026-10-01T10:00:00Z"},
		{Client: "claude", SessionID: "s1", Identity: "b", FirstAt: "2026-10-01T09:00:00+00:00"},
	}
	birth := BirthOf(copies)
	if birth.StartedAt != "2026-10-01T09:00:00Z" {
		t.Fatalf("birth %+v", birth)
	}
	binding := BindingFor("claude", "s1", birth)
	if binding.Limited || len(binding.Digest) != 64 {
		t.Fatalf("binding %+v", binding)
	}
	// A moved or extra copy that starts later keeps the binding.
	moved := append([]store.SessionOriginSource{{Client: "claude", SessionID: "s1", Identity: "moved", FirstAt: "2026-10-02T00:00:00Z"}}, copies...)
	if BindingFor("claude", "s1", BirthOf(moved)) != binding {
		t.Fatal("binding changed with a later copy")
	}
	// Same ID on another client, or another birth, is a different binding.
	if BindingFor("codex", "s1", birth).Digest == binding.Digest || BindingFor("claude", "s1", Birth{StartedAt: "2026-10-01T09:00:01Z"}).Digest == binding.Digest {
		t.Fatal("binding ignores client or birth")
	}
	limited := BindingFor("claude", "s1", BirthOf([]store.SessionOriginSource{{FirstAt: ""}}))
	if !limited.Limited || limited.Digest == "" {
		t.Fatalf("incomplete birth must bind locally as limited: %+v", limited)
	}
}

func TestAssessCodexMarkerUsesOnlyTheRuleEntries(t *testing.T) {
	for _, entry := range CodexOTelBackgroundEntries() {
		a := AssessCodexMarker(store.TelemetrySessionOriginMarker{SourceKind: store.TelemetrySessionSourceKind, SupportEntry: entry, SettledAt: "2026-10-29T12:10:00Z"})
		if got := Reduce(a); got != (Result{ClassAuxiliary, ReasonMaintenanceOrigin, RuleCodexOTelBackgroundV1}) {
			t.Fatalf("%s: %+v", entry, got)
		}
		if a.Evidence[0].SourceToken != `"`+entry+`"|"2026-10-29T12:10:00Z"` {
			t.Fatalf("source token %q", a.Evidence[0].SourceToken)
		}
	}
	for _, entry := range []string{"codex_exec/0.166.0/codex_exec/sse", "codex-app-server/0.165.0/codex-desktop/sse", "codex_exec/0.165.0/codex_exec/websocket", ""} {
		got := Reduce(AssessCodexMarker(store.TelemetrySessionOriginMarker{SourceKind: store.TelemetrySessionSourceKind, SupportEntry: entry}))
		if got != (Result{ClassUnknown, ReasonOriginRuleUnsupported, RuleCodexOTelBackgroundV1}) {
			t.Fatalf("%q: %+v", entry, got)
		}
	}
	if got := Reduce(AssessCodexMarker(store.TelemetrySessionOriginMarker{SourceKind: "other", SupportEntry: CodexOTelBackgroundEntries()[0]})); got.Automatic != ClassUnknown || got.ReasonCode != ReasonOriginMetadataMissing {
		t.Fatalf("other source kind: %+v", got)
	}
}
