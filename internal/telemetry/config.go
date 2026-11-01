// Package telemetry is AgentDeck's optional local OTel receiver: an
// OTLP/HTTP JSON listener bound to loopback, a private fsync'd journal that
// is written before any request is acknowledged, and a reconciler that
// normalizes journaled records into sanitized observations.
//
// Only allowlisted model, token, identity and time fields are kept. Prompt,
// response, tool output, headers and credentials are dropped during decoding
// and never reach the journal, the database or a log.
package telemetry

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
)

const (
	// DefaultPort is the receiver port written by enable when none is given.
	// It is configuration, not a protocol requirement.
	DefaultPort = 4318
	// LogsPath is the only accepted request path.
	LogsPath = "/v1/logs"
	// TokenHeader carries the per-state-root local token.
	TokenHeader = "X-AgentDeck-Token"
	// DefaultRetentionDays keeps normalized journal segments this long.
	DefaultRetentionDays = 7
)

// Config is the receiver configuration AgentDeck owns. It never describes or
// edits a producer's own configuration.
type Config struct {
	Enabled       bool `json:"enabled"`
	Port          int  `json:"port"`
	RetentionDays int  `json:"retention_days"`
}

// Dir is the telemetry directory under a state root.
func Dir(stateRoot string) string { return filepath.Join(stateRoot, "telemetry") }

func configPath(stateRoot string) string { return filepath.Join(Dir(stateRoot), "receiver.json") }
func tokenPath(stateRoot string) string  { return filepath.Join(Dir(stateRoot), "token") }

// JournalDir is where the private journal segments live.
func JournalDir(stateRoot string) string { return filepath.Join(Dir(stateRoot), "journal") }

// LoadConfig reads the receiver configuration. A missing file is the default:
// disabled, with the default port and retention.
func LoadConfig(stateRoot string) (Config, error) {
	config := Config{Port: DefaultPort, RetentionDays: DefaultRetentionDays}
	data, err := os.ReadFile(configPath(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("telemetry receiver configuration is invalid: %w", err)
	}
	if err := config.validate(); err != nil {
		return config, err
	}
	return config, nil
}

func (c Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("telemetry receiver port %d is out of range", c.Port)
	}
	if c.RetentionDays < 1 || c.RetentionDays > 3650 {
		return fmt.Errorf("telemetry journal retention %d days is out of range", c.RetentionDays)
	}
	return nil
}

// SaveConfig writes the receiver configuration privately and atomically.
func SaveConfig(stateRoot string, config Config) error {
	if err := config.validate(); err != nil {
		return err
	}
	if err := ensureDir(stateRoot); err != nil {
		return err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return writePrivateFile(configPath(stateRoot), append(data, '\n'))
}

// EnsureToken returns the local token, creating it on first use.
func EnsureToken(stateRoot string) (string, error) {
	token, err := ReadToken(stateRoot)
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := ensureDir(stateRoot); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token = hex.EncodeToString(raw)
	file, err := os.OpenFile(tokenPath(stateRoot), os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if errors.Is(err, os.ErrExist) {
		return ReadToken(stateRoot)
	}
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(token + "\n"); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	return token, file.Close()
}

// ReadToken reads the local token without creating it.
func ReadToken(stateRoot string) (string, error) {
	info, err := os.Lstat(tokenPath(stateRoot))
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != platform.FileMode {
		return "", errors.New("telemetry token is not a private regular file")
	}
	data, err := os.ReadFile(tokenPath(stateRoot))
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if len(token) != 64 {
		return "", errors.New("telemetry token is malformed")
	}
	return token, nil
}

func tokenMatches(want, got string) bool {
	return len(got) == len(want) && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func ensureDir(stateRoot string) error {
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return err
	}
	for _, dir := range []string{Dir(stateRoot), JournalDir(stateRoot)} {
		if err := os.MkdirAll(dir, platform.DirectoryMode); err != nil {
			return err
		}
		if err := os.Chmod(dir, platform.DirectoryMode); err != nil {
			return err
		}
	}
	return nil
}

func writePrivateFile(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(platform.FileMode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
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
	return os.Rename(name, path)
}
