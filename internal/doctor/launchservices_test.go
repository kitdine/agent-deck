package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestDoctorIncludesLaunchServicesDiagnostic(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := (Service{StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir()}).Check(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Checks {
		if c.Name == "launchservices" {
			return
		}
	}
	t.Fatal("normal doctor report lacks LaunchServices diagnosis")
}

func registrationFixture() RegistrationDetails {
	return RegistrationDetails{Applicable: true, Host: RegistrationSource{Source: "host_application_urls", Complete: true, Entries: []RegistrationEntry{{Path: "/canonical.app", Canonical: true, State: "canonical", Version: "1", Build: "1"}}}, Widget: RegistrationSource{Source: "widget_pluginkit", Complete: true, Entries: []RegistrationEntry{{Path: "/canonical.appex", Canonical: true, State: "canonical", Version: "1", Build: "1"}}}}
}

func TestRegistrationClassification(t *testing.T) {
	for _, tc := range []struct{ name, state, version, build, reason, want string }{
		{"same", "matching_build", "1", "1", "", "consistent"},
		{"different_build", "matching_build", "1", "2", "", "conflict"},
		{"different_version", "matching_build", "2", "1", "", "conflict"},
		{"stale", "missing", "", "", "", "stale"},
		{"malformed", "invalid_metadata", "", "", "", "unknown"},
		{"unreadable", "unreadable_metadata", "", "", "", "unknown"},
		{"unknown_wins", "matching_build", "2", "2", "timeout", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := registrationFixture()
			d.Host.Entries = append(d.Host.Entries, RegistrationEntry{Path: "/other.app", State: tc.state, Version: tc.version, Build: tc.build})
			if tc.reason != "" {
				sourceFailure(&d.Widget, tc.reason)
			}
			c := registrationCheck(d)
			if c.Code != "launchservices_"+tc.want {
				t.Fatalf("%#v", c)
			}
			if len(c.RegistrationDetails.Host.Entries) != 2 {
				t.Fatal("lost positive evidence")
			}
			if c.Recovery != "" || c.ActionKind != "" {
				t.Fatal("unsafe action")
			}
		})
	}
	d := registrationFixture()
	d.Host.Entries = append(d.Host.Entries, d.Host.Entries[0])
	if c := registrationCheck(d); len(c.RegistrationDetails.Host.Entries) != 1 || c.Code != "launchservices_consistent" {
		t.Fatalf("alias: %#v", c)
	}
	for _, which := range []string{"host", "widget"} {
		d := registrationFixture()
		if which == "host" {
			d.Host.Entries = nil
		} else {
			d.Widget.Entries = nil
		}
		if registrationCheck(d).Code != "launchservices_unknown" {
			t.Fatal("empty source healthy")
		}
	}
	d = registrationFixture()
	d.Host.Entries[0].Canonical = false
	if registrationCheck(d).Code != "launchservices_unknown" {
		t.Fatal("canonical omission healthy")
	}
	d = registrationFixture()
	for i := 1; i < 64; i++ {
		d.Host.Entries = append(d.Host.Entries, RegistrationEntry{Path: fmt.Sprintf("/%02d.app", i), State: "matching_build", Version: "1", Build: "1"})
	}
	if c := registrationCheck(d); c.Code != "launchservices_unknown" || c.RegistrationDetails.Host.Reason != "entry_limit" {
		t.Fatal("exact entry cap healthy")
	}
}

func TestRegistrationAccountingAndSkippedPaths(t *testing.T) {
	d := registrationFixture()
	sourceFailure(&d.Host, "control_failed")
	var r Report
	r.add(registrationCheck(d))
	if r.Warnings != 1 || r.Problems != 1 || r.Errors != 0 {
		t.Fatalf("%#v", r)
	}
	called := false
	probe := func(context.Context, bool) RegistrationDetails { called = true; return registrationFixture() }
	if _, err := (Service{StateRoot: t.TempDir() + "/missing", RegistrationProbe: probe}).Check(context.Background(), false); err != nil || called {
		t.Fatal("missing state acquired diagnostic")
	}
	root := t.TempDir()
	if err := os.WriteFile(root+"/state.db", []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Service{StateRoot: root, RegistrationProbe: probe}).Check(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("unreadable database acquired diagnostic")
	}
}

func TestNativeRegistrationReadOnlyAcceptance(t *testing.T) {
	if os.Getenv("AGENTDECK_NATIVE_REGISTRATION_ACCEPTANCE") != "1" {
		t.Skip("ordinary-user native acceptance is explicit")
	}
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, full := range []bool{false, true, false} {
		start := time.Now()
		report, err := (Service{StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir()}).Check(ctx, full)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range report.Checks {
			if c.Name != "launchservices" {
				continue
			}
			raw, _ := json.Marshal(c)
			t.Logf("full=%v total_doctor_ms=%.3f evidence=%s", full, float64(elapsed)/float64(time.Millisecond), raw)
			if c.Code == "launchservices_unknown" || c.Code == "launchservices_not_applicable" {
				t.Fatalf("native sources not accepted: %s", raw)
			}
			if elapsed > registrationBudget(full) {
				t.Fatalf("native completed doctor exceeded diagnostic budget: %s", elapsed)
			}
		}
	}
}

func TestWidgetParserFailsClosed(t *testing.T) {
	good := "     com.kitdine.agentdeck.widget(1.0)\n\tPath = /fixture/widget.appex\n\tUUID = fixture\n\n (1 plug-in)\n"
	paths, reason := parseWidgetPaths([]byte(good))
	if reason != "" || len(paths) != 1 || paths[0] != "/fixture/widget.appex" {
		t.Fatalf("%v %s", paths, reason)
	}
	for _, bad := range []string{"", strings.ReplaceAll(good, "1 plug-in", "2 plug-ins"), good + "unexpected\n", strings.ReplaceAll(good, "UUID", "New Field"), strings.ReplaceAll(good, "Path = /fixture/widget.appex", "Path = relative"), strings.ReplaceAll(good, "com.kitdine.agentdeck.widget", "com.example.other"), strings.ReplaceAll(good, "(1 plug-in)", "(1 插件)")} {
		if _, r := parseWidgetPaths([]byte(bad)); r == "" {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestRegistrationTextEscapesAndPreservesEvidence(t *testing.T) {
	d := registrationFixture()
	d.Host.Entries[0].Path = "/tmp/旧版\n\x1b.app"
	sourceFailure(&d.Widget, "timeout")
	c := registrationCheck(d)
	var b bytes.Buffer
	if err := WriteRegistrationText(&b, c.RegistrationDetails, c.Code); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "\x1b") || !strings.Contains(b.String(), `\n\x1b`) || !strings.Contains(b.String(), "旧版") {
		t.Fatalf("unsafe output %q", b.String())
	}
	if strings.Contains(b.String(), "unregister") {
		t.Fatal("unknown gives target advice")
	}
}

func TestRegistrationChildProcess(t *testing.T) {
	mode := ""
	pidPath := ""
	for i, a := range os.Args {
		if a == "--" && i+2 < len(os.Args) {
			mode = os.Args[i+1]
			pidPath = os.Args[i+2]
			break
		}
	}
	if mode == "" {
		return
	}
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
	switch mode {
	case "sleep":
		time.Sleep(10 * time.Second)
	case "overflow":
		for i := 0; i < 256; i++ {
			_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 4096))
		}
	case "stderr":
		fmt.Fprint(os.Stderr, "diagnostic failure")
	}
	os.Exit(0)
}

func TestRegistrationCommandBoundsAndReaps(t *testing.T) {
	// The race runtime normally waits one second at child exit; remove only that
	// artificial delay, while leaving race instrumentation enabled.
	t.Setenv("GORACE", os.Getenv("GORACE")+" atexit_sleep_ms=0")
	for _, mode := range []string{"sleep", "overflow", "stderr"} {
		t.Run(mode, func(t *testing.T) {
			pidPath := t.TempDir() + "/pid"
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, reason := registrationCommand(ctx, os.Args[0], "-test.run=^TestRegistrationChildProcess$", "--", mode, pidPath)
			want := map[string]string{"sleep": "timeout", "overflow": "output_limit", "stderr": "enumeration_failed"}[mode]
			if reason != want {
				t.Fatalf("reason %s want %s", reason, want)
			}
			if time.Since(start) > 500*time.Millisecond {
				t.Fatal("process budget exceeded")
			}
			p, err := os.ReadFile(pidPath)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(string(p))
			if err != nil {
				t.Fatal(err)
			}
			if err = syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("child still alive: %d %v", pid, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, reason := registrationCommand(ctx, "/bin/sleep", "10"); reason != "cancelled" {
		t.Fatal(reason)
	}
}

func TestRegistrationOutputAllowanceIsCumulative(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, reason := registrationCommandLimited(ctx, 0, "/bin/echo", "unreachable"); reason != "output_limit" {
		t.Fatal(reason)
	}
	first, reason := registrationCommandLimited(ctx, 8, "/bin/echo", "123")
	if reason != "" || len(first) != 4 {
		t.Fatalf("%q %s", first, reason)
	}
	if _, reason = registrationCommandLimited(ctx, 8-len(first), "/bin/echo", "123"); reason != "output_limit" {
		t.Fatal("second child exceeded shared cap", reason)
	}
}

func TestWidgetParserPreservesTrailingSpaces(t *testing.T) {
	output := " com.kitdine.agentdeck.widget(1)\n Path = /tmp/Old.appex \n (1 plug-in)\n"
	paths, reason := parseWidgetPaths([]byte(output))
	if reason != "" || len(paths) != 1 || paths[0] != "/tmp/Old.appex " {
		t.Fatalf("path identity changed: %q %s", paths, reason)
	}
}

func TestWidgetIncompleteOutputPreservesPositiveEvidence(t *testing.T) {
	output := []byte(" com.kitdine.agentdeck.widget(1)\n Path = /tmp/Old.appex \n unexpected\n")
	called := false
	src := widgetSourceFromOutput(context.Background(), output, func(_ context.Context, paths []string, allowance int) RegistrationSource {
		called = true
		if len(paths) != 1 || paths[0] != "/tmp/Old.appex " || allowance != registrationOutputLimit-len(output) {
			t.Fatalf("%q allowance=%d", paths, allowance)
		}
		return RegistrationSource{Source: "widget_pluginkit", Complete: true, Entries: []RegistrationEntry{{Path: paths[0], State: "missing"}}}
	})
	if !called || src.Complete || src.Reason != "unknown_format" || len(src.Entries) != 1 {
		t.Fatalf("lost positive evidence: %#v", src)
	}
	d := registrationFixture()
	d.Widget = src
	if check := registrationCheck(d); check.Code != "launchservices_unknown" || len(check.RegistrationDetails.Widget.Entries) != 1 {
		t.Fatalf("%#v", check)
	}
}

func TestWidgetMetadataFailurePreservesParsedIdentities(t *testing.T) {
	for _, parserSuffix := range []string{" (1 plug-in)\n", " unexpected\n"} {
		for _, failure := range []string{"timeout", "output_limit", "enumeration_failed", ""} {
			t.Run(fmt.Sprintf("%q/%s", parserSuffix, failure), func(t *testing.T) {
				output := []byte(" com.kitdine.agentdeck.widget(1)\n Path = /tmp/Old.appex \n" + parserSuffix)
				src := widgetSourceFromOutput(context.Background(), output, func(_ context.Context, paths []string, allowance int) RegistrationSource {
					if len(paths) != 1 || paths[0] != "/tmp/Old.appex " || allowance != registrationOutputLimit-len(output) {
						t.Fatalf("%q allowance=%d", paths, allowance)
					}
					return RegistrationSource{Source: "widget_pluginkit", Reason: failure}
				})
				if src.Complete || len(src.Entries) != 1 || src.Entries[0] != (RegistrationEntry{Path: "/tmp/Old.appex ", State: "unreadable_metadata"}) {
					t.Fatalf("lost or fabricated evidence: %#v", src)
				}
				wantReason := failure
				if wantReason == "" {
					wantReason = "unreadable_metadata"
				}
				if strings.Contains(parserSuffix, "unexpected") {
					wantReason = "unknown_format"
				}
				if src.Reason != wantReason {
					t.Fatalf("reason=%s want=%s", src.Reason, wantReason)
				}
				d := registrationFixture()
				d.Widget = src
				if check := registrationCheck(d); check.Code != "launchservices_unknown" || len(check.RegistrationDetails.Widget.Entries) != 1 {
					t.Fatalf("%#v", check)
				}
			})
		}
	}
}
