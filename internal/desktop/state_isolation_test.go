package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

// Test-only dependency injection keeps even guard metadata discovery away from
// the operator's account. Production account resolution has no environment hook.
type isolatedStateTestResolver struct{ account string }

func (r isolatedStateTestResolver) AccountHome() (string, error) { return r.account, nil }
func (r isolatedStateTestResolver) Resolve() platform.InstallationIdentity {
	return platform.InstallationIdentity{Reason: "receipt_missing"}
}

func TestMain(m *testing.M) {
	base, err := os.MkdirTemp("", "agentdeck-desktop-tests-")
	if err != nil {
		panic(err)
	}
	account, home := filepath.Join(base, "account"), filepath.Join(base, "home")
	for _, path := range []string{account, home} {
		if err := os.Mkdir(path, 0700); err != nil {
			panic(err)
		}
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", account); err != nil {
		panic(err)
	}
	platform.SetInstallationResolver(isolatedStateTestResolver{account})
	code := m.Run()
	_ = os.RemoveAll(base)
	os.Exit(code)
}
