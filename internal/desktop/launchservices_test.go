package desktop

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/doctor"
)

func TestLaunchServicesProjectionExcludesLocalEvidence(t *testing.T) {
	r := doctor.Report{Status: "degraded", Warnings: 1, Problems: 1, Checks: []doctor.Check{{Name: "launchservices", Status: "warning", Code: "launchservices_conflict", Resource: "launchservices_registration", Reason: "launchservices_conflict", RegistrationDetails: &doctor.RegistrationDetails{Applicable: true, Host: doctor.RegistrationSource{Entries: []doctor.RegistrationEntry{{Path: "/private/obsolete.app", Version: "sensitive-version", Build: "sensitive-build"}}}}}}}
	data, err := json.Marshal(healthSnapshot(r))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"registration_details", "/private/obsolete", "sensitive", "recovery_command", "diagnostic_command", "action_kind"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("leaked %s: %s", forbidden, data)
		}
	}
	if healthSnapshot(r).Warnings != 1 || len(healthSnapshot(r).Checks) != 1 {
		t.Fatal("lost diagnostic counts")
	}
}
