package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
)

const projectAttributionAdvisoryPrefix = "advisory: agentdeck run attributes launches by project through this wrapper"

var httpURLPattern = regexp.MustCompile(`https?://[^\s]+`)

func TestProjectAttributionGuidanceAppearsOnlyForHeadroomWrapperActions(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)

	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "set-wrapper", "example",
		"--url", "https://plain-wrapper.example",
		"--kind", "plain",
	); exit != 0 {
		t.Fatalf("plain set-wrapper exit = %d: %s", exit, stderr)
	} else if strings.Contains(stderr, provider.ProjectAttributionGuideURL) {
		t.Fatalf("plain set-wrapper printed project guidance: %q", stderr)
	}

	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "set-wrapper", "example",
		"--url", "https://headroom-wrapper.example",
		"--kind", "headroom",
	); exit != 0 {
		t.Fatalf("headroom set-wrapper exit = %d: %s", exit, stderr)
	} else {
		assertProjectAttributionGuidance(t, stderr)
	}

	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
	); exit != 0 {
		t.Fatalf("direct provider use exit = %d: %s", exit, stderr)
	} else if strings.Contains(stderr, provider.ProjectAttributionGuideURL) {
		t.Fatalf("direct provider use printed project guidance: %q", stderr)
	}

	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
	); exit != 0 {
		t.Fatalf("Headroom provider use exit = %d: %s", exit, stderr)
	} else {
		assertProjectAttributionGuidance(t, stderr)
	}
}

func TestProjectAttributionGuidanceStaysOutOfJSONAndObeysQuiet(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)

	setArgs := []string{
		"--state-dir", state,
		"--format", "json",
		"provider", "set-wrapper", "example",
		"--url", "https://headroom-wrapper.example",
		"--kind", "headroom",
	}
	setOutput, setStderr, exit := runRouteCommand(t, setArgs...)
	if exit != 0 {
		t.Fatalf("JSON set-wrapper exit = %d: %s", exit, setStderr)
	}
	assertProjectAttributionGuidance(t, setStderr)
	if strings.Contains(setOutput, provider.ProjectAttributionGuideURL) {
		t.Fatalf("project guidance entered set-wrapper JSON: %q", setOutput)
	}

	quietSetArgs := append([]string{}, setArgs[:2]...)
	quietSetArgs = append(quietSetArgs, "--quiet")
	quietSetArgs = append(quietSetArgs, setArgs[2:]...)
	quietSetOutput, quietSetStderr, exit := runRouteCommand(t, quietSetArgs...)
	if exit != 0 {
		t.Fatalf("quiet JSON set-wrapper exit = %d: %s", exit, quietSetStderr)
	}
	if quietSetStderr != "" {
		t.Fatalf("quiet JSON set-wrapper stderr = %q, want empty", quietSetStderr)
	}
	if strings.Contains(quietSetOutput, provider.ProjectAttributionGuideURL) {
		t.Fatalf("project guidance entered quiet set-wrapper JSON: %q", quietSetOutput)
	}

	useArgs := []string{
		"--state-dir", state,
		"--format", "json",
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
	}
	useOutput, useStderr, exit := runRouteCommand(t, useArgs...)
	if exit != 0 {
		t.Fatalf("JSON provider use exit = %d: %s", exit, useStderr)
	}
	assertProjectAttributionGuidance(t, useStderr)
	if strings.Contains(useOutput, provider.ProjectAttributionGuideURL) {
		t.Fatalf("project guidance entered provider use JSON: %q", useOutput)
	}

	quietUseArgs := append([]string{}, useArgs[:2]...)
	quietUseArgs = append(quietUseArgs, "--quiet")
	quietUseArgs = append(quietUseArgs, useArgs[2:]...)
	quietUseOutput, quietUseStderr, exit := runRouteCommand(t, quietUseArgs...)
	if exit != 0 {
		t.Fatalf("quiet JSON provider use exit = %d: %s", exit, quietUseStderr)
	}
	if quietUseStderr != "" {
		t.Fatalf("quiet JSON provider use stderr = %q, want empty", quietUseStderr)
	}
	if strings.Contains(quietUseOutput, provider.ProjectAttributionGuideURL) {
		t.Fatalf("project guidance entered quiet provider use JSON: %q", quietUseOutput)
	}
	assertJSONApartFromGeneratedAt(t, useOutput, quietUseOutput)
}

func assertProjectAttributionGuidance(t *testing.T, stderr string) {
	t.Helper()

	var advisoryLines []string
	for _, line := range strings.Split(strings.TrimSuffix(stderr, "\n"), "\n") {
		if strings.HasPrefix(line, projectAttributionAdvisoryPrefix) {
			advisoryLines = append(advisoryLines, line)
		}
	}
	if len(advisoryLines) != 1 {
		t.Fatalf("project attribution advisory lines = %d in %q, want exactly one", len(advisoryLines), stderr)
	}

	advisoryLine := advisoryLines[0]
	wantLine := "advisory: " + provider.ProjectAttributionAdvisory
	if advisoryLine != wantLine {
		t.Fatalf("project attribution advisory = %q, want %q", advisoryLine, wantLine)
	}

	urls := httpURLPattern.FindAllString(advisoryLine, -1)
	if len(urls) != 1 {
		t.Fatalf("project attribution advisory URLs = %d in %q, want exactly one", len(urls), advisoryLine)
	}
	if urls[0] != provider.ProjectAttributionGuideURL {
		t.Fatalf("project attribution advisory URL = %q, want %q", urls[0], provider.ProjectAttributionGuideURL)
	}

	for _, forbidden := range []string{"headroomlabs-ai", "issues/", "releases/"} {
		if strings.Contains(strings.ToLower(advisoryLine), forbidden) {
			t.Fatalf("project attribution advisory exposed third-party content %q: %q", forbidden, advisoryLine)
		}
	}
}

func assertJSONApartFromGeneratedAt(t *testing.T, left, right string) {
	t.Helper()
	decode := func(value string) map[string]any {
		t.Helper()
		var envelope map[string]any
		if err := json.Unmarshal([]byte(value), &envelope); err != nil {
			t.Fatalf("JSON envelope = %q: %v", value, err)
		}
		delete(envelope, "generated_at")
		return envelope
	}
	if !jsonEqual(t, decode(left), decode(right)) {
		t.Fatalf("JSON envelopes differ beyond generated_at:\nleft: %s\nright: %s", left, right)
	}
}
