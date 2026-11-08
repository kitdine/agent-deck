package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestWorkerOwnershipRefusalCLIContract(t *testing.T) {
	data, err := os.ReadFile("testdata/ownership-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var goldens map[string]struct{ Stderr string }
	if err := json.Unmarshal(data, &goldens); err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"executable_unresolved", "receipt_other_executable", "ownership_busy"} {
		t.Run(reason, func(t *testing.T) {
			err := fmt.Errorf("scan worker: %w", &platform.ErrStateOwnershipRefused{Resource: "state_root", Reason: reason, Revoked: true})
			if got := errorCode(err); got != "state_ownership_refused" {
				t.Fatalf("error code = %q", got)
			}
			if got := errorExitCode(err); got != 1 {
				t.Fatalf("exit code = %d", got)
			}
			encoded, encodeErr := json.Marshal(errorDetails(err))
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			var got map[string]string
			if decodeErr := json.Unmarshal(encoded, &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			want := map[string]string{"resource": "state_root", "reason": reason}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("error details = %#v, want %#v", got, want)
			}
			var stderr bytes.Buffer
			renderCommandErrorText(&stderr, err)
			wantText := strings.Replace(goldens["revoked"].Stderr, "reason: receipt_other_executable", "reason: "+reason, 1)
			if stderr.String() != wantText {
				t.Fatalf("revoked recovery text = %q, want %q", stderr.String(), wantText)
			}
		})
	}
}
