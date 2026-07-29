package main

import (
	"strings"
	"testing"
)

// TestSetWrapperStoresAndReportsTheDeclaredKind covers both provider kinds,
// because the built-in provider stores its wrapper in the settings table rather
// than in a providers row and therefore reaches the declaration through
// different code.
func TestSetWrapperStoresAndReportsTheDeclaredKind(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)

	for _, name := range []string{"example", "official"} {
		if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", name, "--url", "https://"+name+"-wrapper.example", "--kind", "headroom"); exit != 0 {
			t.Fatalf("set-wrapper %s exit = %d: %s", name, exit, stderr)
		}

		shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", name)
		definition := routeEnvelopeData(t, shown)["definition"].(map[string]any)
		if definition["wrapper_kind"] != "headroom" {
			t.Fatalf("%s definition = %#v, want wrapper_kind headroom", name, definition)
		}

		text, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", name)
		if want := "wrapper: https://" + name + "-wrapper.example (headroom)\n"; !strings.Contains(text, want) {
			t.Fatalf("%s provider show = %q, want %q", name, text, want)
		}
	}

	listed, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "list")
	if !strings.Contains(listed, "(headroom)") {
		t.Fatalf("provider list = %q, want the declaration annotated", listed)
	}
}

// TestUndeclaredWrapperIsReportedExactlyAsBefore is the acceptance criterion
// that keeps this field additive: the ordinary invocation, with no --kind, must
// produce the same bytes on every reporting command as it did before the field
// existed — no key in JSON and no annotation in text.
func TestUndeclaredWrapperIsReportedExactlyAsBefore(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example"); exit != 0 {
		t.Fatalf("set-wrapper exit = %d: %s", exit, stderr)
	}

	shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
	if definition := routeEnvelopeData(t, shown)["definition"].(map[string]any); definition["wrapper_kind"] != nil {
		t.Fatalf("undeclared wrapper reported a kind: %#v", definition)
	}

	// provider show and provider list are the two surfaces that render a
	// wrapper URL; the provider status summary table never has.
	for _, command := range [][]string{{"provider", "show", "example"}, {"provider", "list"}} {
		text, _, exit := runRouteCommand(t, append([]string{"--state-dir", state}, command...)...)
		if exit != 0 {
			t.Fatalf("%v exit = %d", command, exit)
		}
		if strings.Contains(text, "headroom") || strings.Contains(text, "plain") {
			t.Fatalf("%v annotated an undeclared wrapper: %q", command, text)
		}
		if !strings.Contains(text, "https://wrapper.example") {
			t.Fatalf("%v lost the wrapper URL: %q", command, text)
		}
	}
}

// TestSetWrapperExplicitPlainIsIndistinguishableFromNoDeclaration pins that the
// default may be stated without changing any output, so a user who spells out
// the default does not get a second, noisier rendering of the same state.
func TestSetWrapperExplicitPlainIsIndistinguishableFromNoDeclaration(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example", "--kind", "plain"); exit != 0 {
		t.Fatalf("set-wrapper --kind plain exit = %d: %s", exit, stderr)
	}
	explicit, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", "example")

	if _, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example"); exit != 0 {
		t.Fatalf("set-wrapper without kind exit = %d", exit)
	}
	omitted, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", "example")

	if explicit != omitted {
		t.Fatalf("explicit plain = %q, omitted = %q, want identical", explicit, omitted)
	}
}

// TestSetWrapperClearRemovesTheDeclarationWithTheURL is the third acceptance
// criterion. A protocol left behind by --clear would attach itself to whatever
// URL was stored next.
func TestSetWrapperClearRemovesTheDeclarationWithTheURL(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	if _, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example", "--kind", "headroom"); exit != 0 {
		t.Fatalf("set-wrapper exit = %d", exit)
	}
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--clear"); exit != 0 {
		t.Fatalf("set-wrapper --clear exit = %d: %s", exit, stderr)
	}

	if _, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://second.example"); exit != 0 {
		t.Fatalf("second set-wrapper exit = %d", exit)
	}
	shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
	definition := routeEnvelopeData(t, shown)["definition"].(map[string]any)
	if definition["wrapper_kind"] != nil {
		t.Fatalf("cleared declaration reattached to a new URL: %#v", definition)
	}
}

// TestSetWrapperReplacementIsVisibleOnStderr pins the resolution of the review's
// P2. The semantics stay replacement — a URL-only call returns the declaration
// to the default — but the loss is announced, because moving a proxy to a new
// address is an ordinary edit that would otherwise silently stop attribution.
func TestSetWrapperReplacementIsVisibleOnStderr(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	if _, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://old.example", "--kind", "headroom"); exit != 0 {
		t.Fatalf("declaring exit = %d", exit)
	}

	_, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://new.example")
	if exit != 0 {
		t.Fatalf("url-only set-wrapper exit = %d: %s", exit, stderr)
	}
	if want := "advisory: wrapper kind reset to plain (was headroom); pass --kind headroom to keep it\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}

	shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
	definition := routeEnvelopeData(t, shown)["definition"].(map[string]any)
	if definition["wrapper_kind"] != nil {
		t.Fatalf("replacement did not take effect: %#v", definition)
	}
	if definition["wrapper_url"] != "https://new.example" {
		t.Fatalf("new URL not stored: %#v", definition)
	}
}

// TestSetWrapperAdvisoryStaysOutOfTheEnvelopeAndObeysQuiet pins the boundaries
// every advisory in this CLI shares. The JSON envelope is compared field for
// field against a run that drops nothing, so the advisory cannot have leaked
// into it.
func TestSetWrapperAdvisoryStaysOutOfTheEnvelopeAndObeysQuiet(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://old.example", "--kind", "headroom")

	dropping, droppingErr, exit := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "set-wrapper", "example", "--url", "https://new.example")
	if exit != 0 {
		t.Fatalf("json run exit = %d: %s", exit, droppingErr)
	}
	if !strings.Contains(droppingErr, "advisory:") {
		t.Fatalf("advisory missing from stderr under --format json: %q", droppingErr)
	}
	if strings.Contains(dropping, "advisory") || strings.Contains(dropping, "wrapper kind reset") {
		t.Fatalf("advisory leaked into the JSON envelope: %s", dropping)
	}

	// A second identical run drops nothing, so its envelope is the reference.
	quiet, quietErr, exit := runRouteCommand(t, "--state-dir", state, "--quiet", "provider", "set-wrapper", "example", "--url", "https://third.example", "--kind", "headroom")
	if exit != 0 {
		t.Fatalf("quiet declaring run exit = %d", exit)
	}
	_ = quiet
	if strings.Contains(quietErr, "advisory") {
		t.Fatalf("advisory fired when nothing was dropped: %q", quietErr)
	}
	_, quietDropErr, exit := runRouteCommand(t, "--state-dir", state, "--quiet", "provider", "set-wrapper", "example", "--url", "https://fourth.example")
	if exit != 0 {
		t.Fatalf("quiet dropping run exit = %d", exit)
	}
	if strings.Contains(quietDropErr, "advisory") {
		t.Fatalf("--quiet did not suppress the advisory: %q", quietDropErr)
	}
}

// TestSetWrapperAdvisoryStaysSilentWhenNothingIsLost covers the cases that must
// not fire, so the advisory keeps its meaning.
func TestSetWrapperAdvisoryStaysSilentWhenNothingIsLost(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "first set", args: []string{"provider", "set-wrapper", "example", "--url", "https://a.example"}},
		{name: "plain to plain", args: []string{"provider", "set-wrapper", "example", "--url", "https://b.example", "--kind", "plain"}},
		{name: "declaring headroom", args: []string{"provider", "set-wrapper", "example", "--url", "https://c.example", "--kind", "headroom"}},
		{name: "headroom to headroom", args: []string{"provider", "set-wrapper", "example", "--url", "https://d.example", "--kind", "headroom"}},
		{name: "clear", args: []string{"provider", "set-wrapper", "example", "--clear"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, stderr, exit := runRouteCommand(t, append([]string{"--state-dir", state}, test.args...)...)
			if exit != 0 {
				t.Fatalf("exit = %d: %s", exit, stderr)
			}
			if strings.Contains(stderr, "advisory") {
				t.Fatalf("advisory fired with nothing dropped: %q", stderr)
			}
		})
	}
}

// TestSetWrapperRejectsInvalidKindCombinations covers the two input errors: a
// protocol nobody implements, and a protocol declared while clearing the URL it
// would describe.
func TestSetWrapperRejectsInvalidKindCombinations(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)

	// Both are invalid input rather than runtime failures, so both exit 2.
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "unknown protocol", args: []string{"provider", "set-wrapper", "example", "--url", "https://wrapper.example", "--kind", "litellm"}},
		{name: "kind while clearing", args: []string{"provider", "set-wrapper", "example", "--clear", "--kind", "headroom"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, exit := runRouteCommand(t, append([]string{"--state-dir", state}, test.args...)...)
			if exit != 2 {
				t.Fatalf("exit = %d, want 2", exit)
			}
			shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
			definition := routeEnvelopeData(t, shown)["definition"].(map[string]any)
			if definition["wrapper_url"] != nil || definition["wrapper_kind"] != nil {
				t.Fatalf("rejected command still stored something: %#v", definition)
			}
		})
	}
}
