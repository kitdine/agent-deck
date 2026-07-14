package buildinfo

import "testing"

func TestDevelopmentDefaults(t *testing.T) {
	if Version != "dev" || Commit != "unknown" || BuildTime != "unknown" {
		t.Fatalf("development identity = version:%q commit:%q build_time:%q", Version, Commit, BuildTime)
	}
	identity := Current()
	if identity.Version != Version || identity.Commit != Commit || identity.BuildTime != BuildTime || identity.GoVersion == "" {
		t.Fatalf("Current() = %#v", identity)
	}
}
