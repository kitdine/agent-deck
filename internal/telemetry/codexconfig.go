package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/kitdine/agent-deck/internal/platform"
)

const (
	managedBegin = "# BEGIN AgentDeck OTel (managed)"
	managedEnd   = "# END AgentDeck OTel (managed)"
)

// Codex facts read from the user-level configuration. Unknown is never
// inferred into on/off or absent.
const (
	CodexUnknown      = "unknown"
	MemoriesOn        = "on"
	MemoriesOff       = "off"
	ExporterManaged   = "managed"
	ExporterAgentDeck = "agentdeck"
	ExporterOther     = "other"
	ExporterAbsent    = "absent"
)

var (
	// ErrCodexOTelConflict reports an existing [otel] section; nothing was changed.
	ErrCodexOTelConflict = errors.New("codex_otel_conflict")
	// ErrCodexConfigWriteFailed reports a write that was refused, raced or
	// failed verification; the file keeps its original bytes unless
	// ErrCodexConfigBlockRemains is also reported.
	ErrCodexConfigWriteFailed = errors.New("codex_config_write_failed")
	// ErrCodexConfigBlockRemains accompanies a failed write whose managed
	// block could not be taken back; its digest record is kept so disable
	// can still remove it.
	ErrCodexConfigBlockRemains = errors.New("codex_config_block_remains")
	// ErrCodexConfigRestoreIncomplete reports a managed block that was changed
	// or is missing; the file was left untouched.
	ErrCodexConfigRestoreIncomplete = errors.New("codex_config_restore_incomplete")
)

// CodexFacts are the two read-only facts the linkage uses.
type CodexFacts struct {
	Memories string
	Exporter string
}

// CodexConfigPath is the user-level Codex configuration file.
func CodexConfigPath(home string) string { return filepath.Join(home, ".codex", "config.toml") }

func managedRecordPath(stateRoot string) string {
	return filepath.Join(Dir(stateRoot), "codex-config.json")
}

type managedRecord struct {
	BlockSHA256 string `json:"block_sha256"`
	WrittenAt   string `json:"written_at"`
}

// ReadCodexFacts parses the user-level configuration. A missing, unreadable
// or unparsable file makes both facts unknown; profiles and command-line
// overrides are not visible and are not guessed.
func ReadCodexFacts(home string, port int) CodexFacts {
	data, err := os.ReadFile(CodexConfigPath(home))
	if err != nil {
		return CodexFacts{Memories: CodexUnknown, Exporter: CodexUnknown}
	}
	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		return CodexFacts{Memories: CodexUnknown, Exporter: CodexUnknown}
	}
	facts := CodexFacts{Memories: CodexUnknown, Exporter: ExporterAbsent}
	if features, ok := config["features"].(map[string]any); ok {
		if value, ok := features["memories"].(bool); ok {
			facts.Memories = map[bool]string{true: MemoriesOn, false: MemoriesOff}[value]
		}
	}
	otel, present := config["otel"]
	if !present {
		return facts
	}
	table, _ := otel.(map[string]any)
	switch {
	case blockCount(data) == 1 && pointsAtReceiver(table, port):
		facts.Exporter = ExporterManaged
	case blockCount(data) == 0 && pointsAtReceiver(table, port):
		facts.Exporter = ExporterAgentDeck
	default:
		facts.Exporter = ExporterOther
	}
	return facts
}

func pointsAtReceiver(table map[string]any, port int) bool {
	exporter, _ := table["exporter"].(map[string]any)
	httpExporter, _ := exporter["otlp-http"].(map[string]any)
	endpoint, _ := httpExporter["endpoint"].(string)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return (host == "127.0.0.1" || host == "localhost") && parsed.Port() == fmt.Sprint(port) && parsed.Path == LogsPath
}

func blockCount(data []byte) int {
	return min(bytes.Count(data, []byte(managedBegin)), bytes.Count(data, []byte(managedEnd)))
}

// ManagedBlock is the exact block appended to the Codex configuration.
func ManagedBlock(endpoint, token string) string {
	return managedBegin + "\n[otel]\nlog_user_prompt = false\n" +
		"exporter = { otlp-http = { endpoint = \"" + endpoint + "\", protocol = \"json\", headers = { \"" + TokenHeader + "\" = \"" + token + "\" } } }\n" +
		managedEnd + "\n"
}

// WriteManagedBlock appends the managed [otel] block after the user agreed.
// It refuses when any [otel] configuration exists, never changes another
// byte, and keeps the original bytes when the file changed during the write
// or the result does not parse as the managed exporter.
func WriteManagedBlock(home, stateRoot string, port int, token string, now time.Time) error {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return err
	}
	path := CodexConfigPath(home)
	original, err := os.ReadFile(path)
	mode := fs.FileMode(platform.FileMode)
	switch {
	case errors.Is(err, os.ErrNotExist):
		original = nil
		if err := os.MkdirAll(filepath.Dir(path), platform.DirectoryMode); err != nil {
			return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
		}
	case err != nil:
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	default:
		info, statErr := os.Stat(path)
		if statErr != nil {
			return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, statErr)
		}
		mode = info.Mode().Perm()
		var config map[string]any
		if err := toml.Unmarshal(original, &config); err != nil {
			return fmt.Errorf("%w: configuration does not parse", ErrCodexConfigWriteFailed)
		}
		if _, present := config["otel"]; present {
			return ErrCodexOTelConflict
		}
	}
	block := ManagedBlock(Endpoint(port), token)
	updated := append([]byte{}, original...)
	if len(updated) > 0 && !bytes.HasSuffix(updated, []byte("\n")) {
		updated = append(updated, '\n')
	}
	if len(updated) > 0 {
		updated = append(updated, '\n')
	}
	updated = append(updated, block...)
	// The digest record is written first, so a block AgentDeck wrote always
	// has the record that lets disable remove it safely.
	sum := sha256.Sum256([]byte(block))
	record, _ := json.Marshal(managedRecord{BlockSHA256: hex.EncodeToString(sum[:]), WrittenAt: now.UTC().Format(time.RFC3339)})
	if err := ensureDir(stateRoot); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	recordPath := managedRecordPath(stateRoot)
	previous, previousErr := os.ReadFile(recordPath)
	if err := writePrivateFile(recordPath, append(record, '\n')); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	err = replaceIfUnchanged(path, original, updated, mode)
	if err == nil && ReadCodexFacts(home, port).Exporter != ExporterManaged {
		err = fmt.Errorf("%w: written configuration did not verify", ErrCodexConfigWriteFailed)
	}
	if err == nil {
		return nil
	}
	if !restoreOriginal(path, original, updated, mode) {
		// The block stays, together with its record.
		return fmt.Errorf("%w: %w", err, ErrCodexConfigBlockRemains)
	}
	if previousErr == nil {
		_ = writePrivateFile(recordPath, previous)
	} else {
		_ = os.Remove(recordPath)
	}
	return err
}

// restoreOriginal reports whether path holds its original bytes, replacing
// the just-written content when it is still unchanged.
func restoreOriginal(path string, original, updated []byte, mode fs.FileMode) bool {
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && original == nil {
		return true
	}
	if err != nil {
		return false
	}
	if bytes.Equal(current, original) {
		return true
	}
	if !bytes.Equal(current, updated) {
		return false
	}
	if original == nil {
		return os.Remove(path) == nil && syncDir(filepath.Dir(path)) == nil
	}
	return replaceIfUnchanged(path, updated, original, mode) == nil
}

// RemoveManagedBlock removes the managed block only when exactly one marker
// pair exists and its bytes equal the recorded digest; otherwise the file is
// left untouched and ErrCodexConfigRestoreIncomplete is returned. Having no
// record means AgentDeck never wrote a block, which is not an error.
func RemoveManagedBlock(home, stateRoot string) (bool, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return false, err
	}
	raw, err := os.ReadFile(managedRecordPath(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var record managedRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return false, ErrCodexConfigRestoreIncomplete
	}
	path := CodexConfigPath(home)
	original, err := os.ReadFile(path)
	if err != nil || blockCount(original) != 1 || bytes.Count(original, []byte(managedBegin)) != 1 {
		return false, ErrCodexConfigRestoreIncomplete
	}
	start := bytes.Index(original, []byte(managedBegin))
	endMarker := bytes.Index(original, []byte(managedEnd))
	if endMarker < start {
		return false, ErrCodexConfigRestoreIncomplete
	}
	end := endMarker + len(managedEnd)
	if end < len(original) && original[end] == '\n' {
		end++
	}
	sum := sha256.Sum256(original[start:end])
	if hex.EncodeToString(sum[:]) != record.BlockSHA256 {
		return false, ErrCodexConfigRestoreIncomplete
	}
	// Remove the blank line the write added before the block.
	if start >= 2 && original[start-1] == '\n' && original[start-2] == '\n' {
		start--
	}
	updated := append(append([]byte{}, original[:start]...), original[end:]...)
	info, err := os.Stat(path)
	if err != nil {
		return false, ErrCodexConfigRestoreIncomplete
	}
	if err := replaceIfUnchanged(path, original, updated, info.Mode().Perm()); err != nil {
		return false, ErrCodexConfigRestoreIncomplete
	}
	return true, os.Remove(managedRecordPath(stateRoot))
}

// replaceIfUnchanged atomically replaces path with updated, keeping mode,
// unless its content no longer equals expected.
func replaceIfUnchanged(path string, expected, updated []byte, mode fs.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".agentdeck-*")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	if _, err := temporary.Write(updated); err != nil {
		temporary.Close()
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && expected == nil {
		current = nil
	} else if err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("%w: configuration changed during the write", ErrCodexConfigWriteFailed)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexConfigWriteFailed, err)
	}
	return syncDir(filepath.Dir(path))
}

// NeedsCollection reports whether an enabled Codex feature produces usage
// that only OTel records. Today that is Memories; guardian and compaction
// join only after the support experiments confirm it.
func (f CodexFacts) NeedsCollection() bool { return f.Memories == MemoriesOn }

// CollectionBlocked reports that the linkage stops the receiver: the only
// feature that needs collection is explicitly off.
func (f CodexFacts) CollectionBlocked() bool { return f.Memories == MemoriesOff }
