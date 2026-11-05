package sessionorigin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/platform"
)

// SourceConfig is the local, non-exported origin-source configuration: an
// explicitly configured data root and a disabled flag per external adapter.
// It locates sources only; it never holds credentials, and it lives under the
// state root outside every database, so a portable backup never carries it.
type SourceConfig struct {
	ClaudeMem AdapterConfig `json:"claude_mem"`
}

// AdapterConfig configures one external adapter. An empty DataDir means the
// adapter is not configured; no default root is ever guessed.
type AdapterConfig struct {
	DataDir  string `json:"data_dir,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Configured reports whether an explicit data root is set.
func (c AdapterConfig) Configured() bool { return c.DataDir != "" }

// Enabled reports whether the adapter may run.
func (c AdapterConfig) Enabled() bool { return c.Configured() && !c.Disabled }

// SourceConfigDir is the private directory of the origin-source configuration.
func SourceConfigDir(stateRoot string) string { return filepath.Join(stateRoot, "session-origin") }

func sourceConfigPath(stateRoot string) string {
	return filepath.Join(SourceConfigDir(stateRoot), "sources.json")
}

// LoadSourceConfig reads the configuration without creating anything. A
// missing file is a successfully read empty configuration; any other failure
// is an error, which callers report as unreadable rather than unconfigured.
func LoadSourceConfig(stateRoot string) (SourceConfig, error) {
	var config SourceConfig
	data, err := os.ReadFile(sourceConfigPath(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return SourceConfig{}, fmt.Errorf("origin source configuration is invalid: %w", err)
	}
	if err := config.validate(); err != nil {
		return SourceConfig{}, err
	}
	return config, nil
}

func (c SourceConfig) validate() error {
	if dir := c.ClaudeMem.DataDir; dir != "" && (!filepath.IsAbs(dir) || filepath.Clean(dir) != dir) {
		return errors.New("origin source data directory must be a clean absolute path")
	}
	return nil
}

// SaveSourceConfig writes the configuration privately (0700 directory, 0600
// file) and atomically.
func SaveSourceConfig(stateRoot string, config SourceConfig) error {
	if err := config.validate(); err != nil {
		return err
	}
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return err
	}
	dir := SourceConfigDir(stateRoot)
	if err := os.MkdirAll(dir, platform.DirectoryMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, platform.DirectoryMode); err != nil {
		return err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".sources.json.*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(platform.FileMode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, sourceConfigPath(stateRoot))
}
