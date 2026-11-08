package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	base, err := os.MkdirTemp("", "agentdeck-platform-tests-")
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
	// Keep the nil-resolver identity contract intact while replacing only the
	// private filesystem boundary used by this package's guard tests.
	statePaths = pathResolver{accountHome: func() (string, error) { return account, nil }, lstat: os.Lstat, readlink: os.Readlink}
	code := m.Run()
	_ = os.RemoveAll(base)
	os.Exit(code)
}
