// Package buildinfo owns the identity embedded in an AgentDeck binary.
package buildinfo

import "runtime"

var (
	Version   = "dev"
	Commit    = "unknown"
	Branch    = "unknown"
	BuildTime = "unknown"
)

// Identity is the support-facing identity of the running binary.
type Identity struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// Current returns the injected identity and the active Go runtime version.
func Current() Identity {
	return Identity{
		Version:   Version,
		Commit:    Commit,
		Branch:    Branch,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
	}
}
