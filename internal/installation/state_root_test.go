package installation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestDefaultStateRootPreservesPhysicalHomeMeaning(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "target", "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "jump")
	if err := os.Symlink(filepath.Join(base, "target", "inner"), link); err != nil {
		t.Fatal(err)
	}
	home := link + "/.."
	want := filepath.Join(base, "target", ".agentdeck-dev")
	if got, err := StateRootWithHome("", home); err != nil || got != want {
		t.Fatalf("default physical root=%q %v; want %q", got, err, want)
	}
	t.Setenv("HOME", home)
	if got, err := StateRoot(""); err != nil || got != want {
		t.Fatalf("HOME physical root=%q %v; want %q", got, err, want)
	}
	_, err = StateRootWithHome("", base+"/missing/../home")
	var refused *platform.ErrStateOwnershipRefused
	if !errors.As(err, &refused) || refused.Reason != "state_path_unresolved" {
		t.Fatalf("missing HOME parent was cleaned away: %v", err)
	}
}
