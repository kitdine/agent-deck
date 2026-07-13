// Package platform owns portable operating-system boundaries.
package platform

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Clock keeps time-dependent domain behavior testable without tying it to the
// host clock.
type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// FileSystem is the portable subset needed by state and future atomic-write
// adapters. Production code uses OSFileSystem; tests can supply a fake.
type FileSystem interface {
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	OpenFile(string, int, fs.FileMode) (*os.File, error)
	Remove(string) error
	Stat(string) (fs.FileInfo, error)
}

type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFileSystem) Chmod(path string, mode fs.FileMode) error    { return os.Chmod(path, mode) }
func (OSFileSystem) OpenFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm)
}
func (OSFileSystem) Remove(path string) error              { return os.Remove(path) }
func (OSFileSystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

const (
	DirectoryMode fs.FileMode = 0o700
	FileMode      fs.FileMode = 0o600
)

// StateRoot returns the explicit test or CLI override, or the user's default
// AgentDeck state directory. It does not create or modify the path.
func StateRoot(override, home string) string {
	if override != "" {
		return override
	}
	return filepath.Join(home, ".agentdeck")
}

// EnsureStateRoot creates a private state directory without changing client
// configuration or inspecting legacy state.
func EnsureStateRoot(path string) error {
	return EnsureStateRootWithFS(OSFileSystem{}, path)
}

func EnsureStateRootWithFS(filesystem FileSystem, path string) error {
	if err := filesystem.MkdirAll(path, DirectoryMode); err != nil {
		return err
	}
	return filesystem.Chmod(path, DirectoryMode)
}
