package quota

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowedReadingOffSuppressesRegardlessOfProvider(t *testing.T) {
	allowed, reason := Allowed(false, true, true, true)
	if allowed || reason != ReasonProbeDisabled {
		t.Fatalf("Allowed = (%v, %v), want (false, probe_disabled)", allowed, reason)
	}
}

func TestAllowedReadingOffTakesPrecedenceOverNotOfficial(t *testing.T) {
	// The two conditions are ordered (C1): quotaProbe off always reports
	// probe_disabled, never not_official, even when the provider would also
	// fail the gate.
	allowed, reason := Allowed(false, false, false, false)
	if allowed || reason != ReasonProbeDisabled {
		t.Fatalf("Allowed = (%v, %v), want (false, probe_disabled)", allowed, reason)
	}
}

func TestAllowedNotOfficialRecordedSelection(t *testing.T) {
	allowed, reason := Allowed(true, false, false, false)
	if allowed || reason != ReasonNotOfficial {
		t.Fatalf("Allowed = (%v, %v), want (false, not_official)", allowed, reason)
	}
}

func TestAllowedNoRecordedSelectionIsNotOfficial(t *testing.T) {
	// A client that has never completed a provider selection is treated the
	// same as an explicit non-official selection: recordedOfficial is false
	// in both cases, and the caller must not distinguish them here.
	allowed, reason := Allowed(true, false, true, true)
	if allowed || reason != ReasonNotOfficial {
		t.Fatalf("Allowed = (%v, %v), want (false, not_official)", allowed, reason)
	}
}

func TestAllowedOfficialWithNoObservedProviderKnown(t *testing.T) {
	// The ordinary case without Hook integration: observedKnown is false and
	// must never itself suppress a probe.
	allowed, reason := Allowed(true, true, false, false)
	if !allowed || reason != "" {
		t.Fatalf("Allowed = (%v, %q), want (true, \"\")", allowed, reason)
	}
}

func TestAllowedOfficialWithObservedProviderAgreeing(t *testing.T) {
	allowed, reason := Allowed(true, true, true, true)
	if !allowed || reason != "" {
		t.Fatalf("Allowed = (%v, %q), want (true, \"\")", allowed, reason)
	}
}

func TestAllowedDisagreeingObservedProviderSuppressesDespiteRecordedOfficial(t *testing.T) {
	// C1's mitigation: an available observed provider that disagrees with the
	// recorded selection suppresses the probe even though the recorded
	// selection alone would have allowed it.
	allowed, reason := Allowed(true, true, true, false)
	if allowed || reason != ReasonNotOfficial {
		t.Fatalf("Allowed = (%v, %v), want (false, not_official)", allowed, reason)
	}
}

// TestQuotaPackagesReadNoCredentialAndOpenNoNetworkConnection is C0's test:
// "asserted, not merely intended" (requirements.md clause 15), scoped to this
// topic's own production Go files. It checks two things per architecture.md
// C0: no import of internal/credentialvault or a keychain API, and no
// construction of an HTTP client. The second property already has a
// repository-wide check (cmd/agentdeck's TestNetworkImportsAreLimitedToPriceUpdate);
// this test is topic-scoped and independent of that one surviving unchanged,
// per C0's "enforced in three places rather than trusted once."
func TestQuotaPackagesReadNoCredentialAndOpenNoNetworkConnection(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := []string{
		filepath.Join(root, "internal", "quota"),
	}
	forbiddenImports := []string{
		"github.com/kitdine/agent-deck/internal/credentialvault",
		"net/http",
		"net",
	}
	forbiddenSubstrings := []string{".credentials.json", "auth.json", "keychain", "Keychain"}

	quotaFile := func(name string) bool {
		return name == "quota_capture.go" || strings.HasPrefix(name, "quota")
	}

	visit := func(path string, entry fs.DirEntry) error {
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, substr := range forbiddenSubstrings {
			if strings.Contains(string(source), substr) {
				t.Errorf("%s contains forbidden substring %q (C0: no credential file or keychain access)", path, substr)
			}
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			name := strings.Trim(imported.Path.Value, "\"")
			for _, forbidden := range forbiddenImports {
				if name == forbidden {
					t.Errorf("%s imports %q, forbidden by C0 over this topic's own packages", path, name)
				}
			}
		}
		return nil
	}

	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			return visit(path, entry)
		}); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	cmdDir := filepath.Join(root, "cmd", "agentdeck")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", cmdDir, err)
	}
	for _, entry := range entries {
		if !quotaFile(entry.Name()) {
			continue
		}
		if err := visit(filepath.Join(cmdDir, entry.Name()), entry); err != nil {
			t.Fatalf("walking %s: %v", entry.Name(), err)
		}
	}
}
