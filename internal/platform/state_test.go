package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stateRootFileSystem struct {
	calls                []string
	mkdirErr, chmodErr   error
	mkdirPath, chmodPath string
	mkdirMode, chmodMode fs.FileMode
}

func (f *stateRootFileSystem) MkdirAll(path string, mode fs.FileMode) error {
	f.calls = append(f.calls, "MkdirAll")
	f.mkdirPath = path
	f.mkdirMode = mode
	return f.mkdirErr
}

func (f *stateRootFileSystem) Chmod(path string, mode fs.FileMode) error {
	f.calls = append(f.calls, "Chmod")
	f.chmodPath = path
	f.chmodMode = mode
	return f.chmodErr
}

func (*stateRootFileSystem) OpenFile(string, int, fs.FileMode) (*os.File, error) { return nil, nil }
func (*stateRootFileSystem) Remove(string) error                                 { return nil }
func (*stateRootFileSystem) Stat(string) (fs.FileInfo, error)                    { return nil, nil }

func TestStateRoot(t *testing.T) {
	if got := StateRoot("/tmp/isolated", "/Users/example"); got != "/tmp/isolated" {
		t.Fatalf("override root = %q", got)
	}
	if got, want := StateRoot("", "/Users/example"), filepath.Join("/Users/example", ".agentdeck"); got != want {
		t.Fatalf("default root = %q, want %q", got, want)
	}
}

func TestEnsureStateRootWithFSCallOrderModesAndErrors(t *testing.T) {
	mkdirErr := errors.New("mkdir failed")
	chmodErr := errors.New("chmod failed")
	tests := []struct {
		name       string
		filesystem *stateRootFileSystem
		wantErr    error
		wantCalls  []string
	}{
		{"success", &stateRootFileSystem{}, nil, []string{"MkdirAll", "Chmod"}},
		{"mkdir error stops before chmod", &stateRootFileSystem{mkdirErr: mkdirErr}, mkdirErr, []string{"MkdirAll"}},
		{"chmod error", &stateRootFileSystem{chmodErr: chmodErr}, chmodErr, []string{"MkdirAll", "Chmod"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const root = "/state"
			err := EnsureStateRootWithFS(tt.filesystem, root)
			if err != tt.wantErr {
				t.Fatalf("EnsureStateRootWithFS() error = %v, want exact sentinel %v", err, tt.wantErr)
			}
			if len(tt.filesystem.calls) != len(tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", tt.filesystem.calls, tt.wantCalls)
			}
			for i := range tt.filesystem.calls {
				if tt.filesystem.calls[i] != tt.wantCalls[i] {
					t.Fatalf("calls = %v, want %v", tt.filesystem.calls, tt.wantCalls)
				}
			}
			if got := tt.filesystem.mkdirMode; got != DirectoryMode {
				t.Fatalf("MkdirAll mode = %#o, want %#o", got, DirectoryMode)
			}
			if got := tt.filesystem.mkdirPath; got != root {
				t.Fatalf("MkdirAll path = %q, want %q", got, root)
			}
			if tt.wantErr != mkdirErr {
				if got := tt.filesystem.chmodPath; got != root {
					t.Fatalf("Chmod path = %q, want %q", got, root)
				}
				if got := tt.filesystem.chmodMode; got != DirectoryMode {
					t.Fatalf("Chmod mode = %#o, want %#o", got, DirectoryMode)
				}
			}
		})
	}
}

func TestEnsureStateRootSetsPrivateMode(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, root string)
	}{
		{"new root", func(*testing.T, string) {}},
		{"preexisting permissive root", func(t *testing.T, root string) {
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(root, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			tt.prepare(t, root)
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
		})
	}
}

func TestSystemClockProducesTime(t *testing.T) {
	before := time.Now().Add(-time.Second)
	got := SystemClock{}.Now()
	if got.Before(before) || got.After(time.Now().Add(time.Second)) {
		t.Fatalf("clock returned unexpected time %s", got)
	}
}
