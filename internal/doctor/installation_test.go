package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

type installedDoctorFixture struct{ account string }

func (r installedDoctorFixture) AccountHome() (string, error) { return r.account, nil }
func (r installedDoctorFixture) Resolve() platform.InstallationIdentity {
	return platform.InstallationIdentity{Installed: true, Channel: "make"}
}

func TestMain(m *testing.M) {
	account, err := os.MkdirTemp("", "agentdeck-doctor-account-")
	if err != nil {
		panic(err)
	}
	home := filepath.Join(account, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", account); err != nil {
		panic(err)
	}
	platform.SetInstallationResolver(installedDoctorFixture{account})
	code := m.Run()
	_ = store.CloseReadPools()
	_ = platform.ReleaseStateOwnership()
	_ = os.RemoveAll(account)
	os.Exit(code)
}

func TestInstallationCheckPrecedesStateAndHasNoPath(t *testing.T) {
	check := installationCheck()
	if check.Name != "installation" || check.Status != "ok" || check.Reason != "make" || check.Code != "" || check.Recovery != "" {
		t.Fatalf("installed check = %+v", check)
	}
	report := RefusedStateReport(false, "receipt_other_executable")
	if !report.Partial || report.Status != "unhealthy" || len(report.Checks) != 2 || report.Checks[1].Code != "state_ownership_refused" {
		t.Fatalf("partial = %+v", report)
	}
}
