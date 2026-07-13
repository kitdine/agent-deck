package platform

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoot(t *testing.T) {
	if got := StateRoot("/tmp/isolated", "/Users/example"); got != "/tmp/isolated" {
		t.Fatalf("override root = %q", got)
	}
	if got, want := StateRoot("", "/Users/example"), filepath.Join("/Users/example", ".agentdeck"); got != want {
		t.Fatalf("default root = %q, want %q", got, want)
	}
}

func TestEnsureStateRootIsPrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if err := EnsureStateRoot(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != DirectoryMode {
		t.Fatalf("root mode = %#o, want %#o", got, DirectoryMode)
	}
}

func TestSystemClockProducesTime(t *testing.T) {
	before := time.Now().Add(-time.Second)
	got := SystemClock{}.Now()
	if got.Before(before) || got.After(time.Now().Add(time.Second)) {
		t.Fatalf("clock returned unexpected time %s", got)
	}
}
