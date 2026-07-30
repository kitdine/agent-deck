package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/shellconfig"
)

const projectAttributionAdvisoryPrefix = "advisory: project attribution requires an eligible Headroom wrapper route, its eligibility marker, and configured shell integration"

var httpURLPattern = regexp.MustCompile(`https?://[^\s]+`)

func TestProjectAttributionGuidanceAppearsOnlyForHeadroomWrapperActions(t *testing.T) {
	stubShellAdvisoryEnvironment(t, t.TempDir(), shellconfig.ShellZsh)
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
	} else if !strings.Contains(
		stderr,
		"advisory: project attribution needs one-time shell integration setup: agentdeck shell setup",
	) {
		t.Fatalf("unconfigured Headroom provider use advisory = %q", stderr)
	}
}

func TestProjectAttributionGuidanceStaysOutOfJSONAndObeysQuiet(t *testing.T) {
	stubShellAdvisoryEnvironment(t, t.TempDir(), shellconfig.ShellZsh)
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
	if !strings.Contains(
		useStderr,
		"advisory: project attribution needs one-time shell integration setup: agentdeck shell setup",
	) {
		t.Fatalf("JSON provider use advisory = %q", useStderr)
	}
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

func TestProjectAttributionRouteChangeAdvisoryMatrix(t *testing.T) {
	t.Run("enter eligible with integration configured", func(t *testing.T) {
		home := t.TempDir()
		stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
		state, config := newRouteSurfaceFixture(t)
		if _, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"shell", "setup", "zsh",
		); exit != 0 {
			t.Fatalf("shell setup exit = %d: %s", exit, stderr)
		}
		configureHeadroomWrapper(t, state)

		_, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"provider", "use", "example",
			"--client", "codex",
			"--config-path", config,
			"--via",
		)
		if exit != 0 {
			t.Fatalf("eligible switch exit = %d: %s", exit, stderr)
		}
		for _, want := range []string{
			"advisory: project attribution is in effect",
			"configured shell startup files carry it in new sessions",
			`activate this zsh session: eval "$(agentdeck --state-dir ` +
				shellQuote(state) + ` shell-init zsh)"`,
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("configured eligible advisory = %q, want %q", stderr, want)
			}
		}
		assertRouteAdvisorySafe(t, stderr)
	})

	t.Run("enter eligible without integration", func(t *testing.T) {
		home := t.TempDir()
		stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
		state, config := newRouteSurfaceFixture(t)
		configureHeadroomWrapper(t, state)

		_, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"provider", "use", "example",
			"--client", "codex",
			"--config-path", config,
			"--via",
		)
		if exit != 0 {
			t.Fatalf("eligible switch exit = %d: %s", exit, stderr)
		}
		if !strings.Contains(
			stderr,
			"advisory: project attribution needs one-time shell integration setup: agentdeck shell setup",
		) {
			t.Fatalf("unconfigured eligible advisory = %q", stderr)
		}
		if strings.Contains(stderr, "is in effect") {
			t.Fatalf("unconfigured advisory claimed attribution active: %q", stderr)
		}
		assertRouteAdvisorySafe(t, stderr)
	})

	t.Run("leave eligible with integration configured", func(t *testing.T) {
		home := t.TempDir()
		stubShellAdvisoryEnvironment(t, home, shellconfig.ShellFish)
		state, config := newRouteSurfaceFixture(t)
		if _, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"shell", "setup", "fish",
		); exit != 0 {
			t.Fatalf("shell setup exit = %d: %s", exit, stderr)
		}
		configureHeadroomWrapper(t, state)
		useHeadroomRoute(t, state, config)

		_, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"provider", "use", "example",
			"--client", "codex",
			"--config-path", config,
		)
		if exit != 0 {
			t.Fatalf("direct switch exit = %d: %s", exit, stderr)
		}
		if !strings.Contains(
			stderr,
			"advisory: managed shell wrappers remain installed but stop injecting project attribution now",
		) {
			t.Fatalf("configured leave advisory = %q", stderr)
		}
		assertRouteAdvisorySafe(t, stderr)
	})

	t.Run("leave eligible without integration", func(t *testing.T) {
		home := t.TempDir()
		stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
		state, config := newRouteSurfaceFixture(t)
		configureHeadroomWrapper(t, state)
		useHeadroomRoute(t, state, config)

		_, stderr, exit := runRouteCommand(
			t,
			"--state-dir", state,
			"provider", "use", "example",
			"--client", "codex",
			"--config-path", config,
		)
		if exit != 0 {
			t.Fatalf("direct switch exit = %d: %s", exit, stderr)
		}
		if strings.Contains(stderr, provider.ProjectAttributionGuideURL) ||
			strings.Contains(stderr, "project attribution") {
			t.Fatalf("unconfigured leave switch printed attribution advisory: %q", stderr)
		}
	})
}

func TestProjectAttributionAdvisoryUnreadableStartupDegradesToSetup(t *testing.T) {
	home := t.TempDir()
	stubShellAdvisoryEnvironment(t, home, shellconfig.ShellZsh)
	if err := os.Mkdir(filepath.Join(home, ".zshrc"), 0o700); err != nil {
		t.Fatal(err)
	}
	state, config := newRouteSurfaceFixture(t)
	configureHeadroomWrapper(t, state)

	_, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
	)
	if exit != 0 {
		t.Fatalf("eligible switch with unreadable startup file exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(
		stderr,
		"advisory: project attribution needs one-time shell integration setup: agentdeck shell setup",
	) {
		t.Fatalf("unreadable startup advisory = %q", stderr)
	}
}

func configureHeadroomWrapper(t *testing.T, state string) {
	t.Helper()
	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"provider", "set-wrapper", "example",
		"--url", "https://headroom-wrapper.example",
		"--kind", "headroom",
	); exit != 0 {
		t.Fatalf("set-wrapper exit = %d: %s", exit, stderr)
	}
}

func useHeadroomRoute(t *testing.T, state, config string) {
	t.Helper()
	if _, stderr, exit := runRouteCommand(
		t,
		"--state-dir", state,
		"--quiet",
		"provider", "use", "example",
		"--client", "codex",
		"--config-path", config,
		"--via",
	); exit != 0 {
		t.Fatalf("Headroom switch exit = %d: %s", exit, stderr)
	}
}

func stubShellAdvisoryEnvironment(t *testing.T, home string, shell shellconfig.Shell) {
	t.Helper()
	oldHome := userHomeDir
	oldDetect := detectInvokingShell
	userHomeDir = func() (string, error) { return home, nil }
	detectInvokingShell = func() (shellconfig.Invocation, error) {
		return shellconfig.Invocation{Shell: shell}, nil
	}
	t.Cleanup(func() {
		userHomeDir = oldHome
		detectInvokingShell = oldDetect
	})
}

func assertRouteAdvisorySafe(t *testing.T, stderr string) {
	t.Helper()
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.HasPrefix(line, "advisory: project attribution") &&
			!strings.HasPrefix(line, "advisory: managed shell wrappers") {
			continue
		}
		for _, forbidden := range []string{
			"HEADROOM_PROJECT",
			"https://headroom-wrapper.example",
			"https://provider.example",
			"synthetic-secret",
		} {
			if strings.Contains(line, forbidden) {
				t.Fatalf("route advisory exposed %q: %q", forbidden, line)
			}
		}
	}
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
