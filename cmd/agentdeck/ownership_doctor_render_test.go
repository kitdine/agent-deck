package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/kitdine/agent-deck/internal/doctor"
)

// The two complete-report specimens abbreviate unrelated checks, so their
// rendering contract is proved by feeding the specimen's own checks through
// the doctor writer: line order, extra lines and JSON fields must match the
// prototype byte for byte. That the checks are produced is proved in
// internal/doctor.
func TestOwnershipCompleteDoctorSpecimensRender(t *testing.T) {
	data, err := os.ReadFile("testdata/ownership-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var goldens map[string]struct {
		Stdout string
		JSON   string
	}
	if err := json.Unmarshal(data, &goldens); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"doctor_installed", "doctor_command_owner"} {
		t.Run(name, func(t *testing.T) {
			golden := goldens[name]
			var envelope struct {
				Data struct {
					Mode   string         `json:"mode"`
					Checks []doctor.Check `json:"checks"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(golden.JSON), &envelope); err != nil {
				t.Fatal(err)
			}
			report := doctor.Report{Mode: envelope.Data.Mode, Status: "healthy", Healthy: true, Checks: []doctor.Check{}}
			for _, check := range envelope.Data.Checks {
				report.Add(check)
			}
			var text bytes.Buffer
			if err := writeDoctorResult(&text, "text", report); err != nil {
				t.Fatal(err)
			}
			if text.String() != golden.Stdout {
				t.Fatalf("text = %q\nwant %q", text.String(), golden.Stdout)
			}
			var encoded bytes.Buffer
			if err := writeDoctorResult(&encoded, "json", report); err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(encoded.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(golden.JSON), &want); err != nil {
				t.Fatal(err)
			}
			delete(got, "generated_at")
			delete(want, "generated_at")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("json = %s\nwant %s", encoded.String(), golden.JSON)
			}
		})
	}
}
