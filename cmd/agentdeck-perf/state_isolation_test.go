package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

var runtimeCLIEnvironments sync.Map
var isolatedPerfAccount string

type isolatedPerfResolver struct{ account string }

func (r isolatedPerfResolver) AccountHome() (string, error) { return r.account, nil }
func (r isolatedPerfResolver) Resolve() platform.InstallationIdentity {
	return platform.InstallationIdentity{Reason: "receipt_missing"}
}

func TestMain(m *testing.M) {
	base, err := os.MkdirTemp("", "agentdeck-perf-tests-")
	if err != nil {
		panic(err)
	}
	isolatedPerfAccount = filepath.Join(base, "account")
	home := filepath.Join(base, "home")
	for _, path := range []string{isolatedPerfAccount, home} {
		if err := os.Mkdir(path, 0700); err != nil {
			panic(err)
		}
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME", isolatedPerfAccount); err != nil {
		panic(err)
	}
	platform.SetInstallationResolver(isolatedPerfResolver{isolatedPerfAccount})
	testCLIEnvironment = func(binary string) []string {
		physical, err := filepath.EvalSymlinks(binary)
		if err != nil {
			return nil
		}
		if value, ok := runtimeCLIEnvironments.Load(physical); ok {
			return value.([]string)
		}
		return nil
	}
	code := m.Run()
	_ = os.RemoveAll(base)
	os.Exit(code)
}
