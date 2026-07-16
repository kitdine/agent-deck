package buildinfo

import "testing"

func TestDevelopmentDefaults(t *testing.T) {
	if Version != "dev" || Commit != "unknown" || Branch != "unknown" || BuildTime != "unknown" {
		t.Fatalf("development identity = version:%q commit:%q branch:%q build_time:%q", Version, Commit, Branch, BuildTime)
	}
	identity := Current()
	if identity.Version != Version || identity.Commit != Commit || identity.Branch != Branch || identity.BuildTime != BuildTime || identity.GoVersion == "" {
		t.Fatalf("Current() = %#v", identity)
	}
}
