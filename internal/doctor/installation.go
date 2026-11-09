package doctor

import (
	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/platform"
)

func installationCheck() Check {
	identity := platform.CurrentInstallation()
	if identity.Installed {
		return Check{Name: "installation", Status: "ok", Reason: identity.Channel}
	}
	check := Check{Name: "installation", Status: "warning", Code: "not_installed", Reason: identity.Reason}
	if installation.DefaultResolver().IsFormulaKeg() {
		check.Recovery = "agentdeck install claim"
	}
	return check
}

func newReport(full bool) Report {
	report := Report{Mode: map[bool]string{false: "quick", true: "full"}[full], Status: "healthy", Healthy: true, Checks: []Check{}}
	report.add(installationCheck())
	return report
}

// RefusedStateReport never inspects the requested state, including its metadata.
func RefusedStateReport(full bool, reason string) Report {
	report := newReport(full)
	report.Partial = true
	report.add(Check{Name: "state", Status: "error", Code: "state_ownership_refused", Reason: reason})
	return report
}

// Add appends a check with the report's status accounting; renderers' tests
// use it to build reports from contract specimens.
func (r *Report) Add(check Check) { r.add(check) }
