package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/store"
)

type registrationFailWriter struct{}

func (registrationFailWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }

func TestDoctorLaunchServicesTextAndSuccessEnvelope(t *testing.T) {
	d := &doctor.RegistrationDetails{Applicable: true, Scope: "returned_host_urls_and_targeted_widget_matches", Host: doctor.RegistrationSource{Source: "host_application_urls", Complete: true, Entries: []doctor.RegistrationEntry{{Path: "/tmp/旧版\n\x1b.app", State: "different_build", Version: "1", Build: "2"}}}, Widget: doctor.RegistrationSource{Source: "widget_pluginkit", Reason: "timeout", Entries: []doctor.RegistrationEntry{}}}
	r := doctor.Report{Mode: "quick", Status: "degraded", Problems: 1, Warnings: 1, Checks: []doctor.Check{{Name: "extensions", Status: "ok"}, {Name: "launchservices", Status: "warning", Code: "launchservices_unknown", Resource: "launchservices_registration", Reason: "launchservices_unknown", RegistrationDetails: d}}}
	var text bytes.Buffer
	if err := writeDoctorResult(&text, "text", r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "extensions: ok\nlaunchservices: warning (launchservices_unknown)") || strings.Contains(text.String(), "\x1b") || strings.Contains(text.String(), "recovery:") {
		t.Fatal(text.String())
	}
	var encoded bytes.Buffer
	if err := writeDoctorResult(&encoded, "json", r); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(encoded.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["command"] != "doctor" || env["error"] != nil || env["partial"] != false {
		t.Fatalf("envelope %s", encoded.Bytes())
	}
	if !bytes.Contains(encoded.Bytes(), []byte("registration_details")) || bytes.Contains(encoded.Bytes(), []byte("recovery_command")) {
		t.Fatal(encoded.String())
	}
	if err := writeDoctorResult(registrationFailWriter{}, "text", r); err == nil {
		t.Fatal("lost writer failure")
	}
}

// This measures the complete fixture refresh with the real native registration
// probe and a production worker, without registering or changing any bundle.
func TestLaunchServicesRefreshPerformanceFixture(t *testing.T) {
	if os.Getenv("AGENTDECK_NATIVE_REGISTRATION_ACCEPTANCE") != "1" {
		t.Skip("explicit native acceptance only")
	}
	if os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_EXECUTABLE") == "" || os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_REPORT") == "" {
		t.Fatal("provide built executable and private report path")
	}
	corpus := writeSnapshotPerformanceCorpus(t)
	t.Setenv("AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS", corpus.Home)
	t.Setenv("AGENTDECK_SNAPSHOT_PERFORMANCE_SAMPLES", "3")
	TestSnapshotPerformanceRepresentativeCorpus(t)
}

type registrationNoStdin struct{ calls int }

func (r *registrationNoStdin) Read([]byte) (int, error) {
	r.calls++
	return 0, errors.New("doctor must not read stdin")
}

func TestDoctorLaunchServicesAllStatesWithoutTTYOrStdin(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	previousProbe, previousHome := doctorRegistrationProbe, userHomeDir
	t.Cleanup(func() { doctorRegistrationProbe, userHomeDir = previousProbe, previousHome })
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Setenv("TERM", "dumb")
	t.Setenv("COLUMNS", "20")
	for _, state := range []string{"consistent", "conflict", "stale", "unknown", "not_applicable"} {
		t.Run(state, func(t *testing.T) {
			canonical := doctor.RegistrationEntry{Path: "/canonical.app", Canonical: true, State: "canonical", Version: "1", Build: "1"}
			d := doctor.RegistrationDetails{Applicable: true, Host: doctor.RegistrationSource{Complete: true, Entries: []doctor.RegistrationEntry{canonical}}, Widget: doctor.RegistrationSource{Complete: true, Entries: []doctor.RegistrationEntry{canonical}}}
			switch state {
			case "conflict":
				d.Host.Entries = append(d.Host.Entries, doctor.RegistrationEntry{Path: "/old.app", State: "different_build", Version: "1", Build: "2"})
			case "stale":
				d.Host.Entries = append(d.Host.Entries, doctor.RegistrationEntry{Path: "/gone.app", State: "missing"})
			case "unknown":
				d.Widget.Complete = false
				d.Widget.Reason = "timeout"
			case "not_applicable":
				d.Applicable = false
				d.ApplicabilityReason = "gui_host_absent"
			}
			doctorRegistrationProbe = func(context.Context, bool) doctor.RegistrationDetails { return d }
			for _, format := range []string{"text", "json"} {
				var out bytes.Buffer
				input := &registrationNoStdin{}
				if err := run([]string{"--state-dir", root, "--format", format, "doctor"}, input, &out); err != nil {
					t.Fatal(err)
				}
				if input.calls != 0 || bytes.Contains(out.Bytes(), []byte("\x1b")) || !strings.Contains(out.String(), "launchservices_"+state) {
					t.Fatalf("stdin=%d output=%s", input.calls, out.String())
				}
				if format == "text" {
					hasTargetAdvice := strings.Contains(out.String(), "manually unregister")
					if hasTargetAdvice != (state == "conflict" || state == "stale") {
						t.Fatalf("unsafe advice for %s: %s", state, out.String())
					}
				} else {
					var envelope map[string]any
					if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
						t.Fatal(err)
					}
					if envelope["command"] != "doctor" || envelope["error"] != nil || envelope["partial"] != false {
						t.Fatalf("%s", out.String())
					}
				}
			}
		})
	}
}
