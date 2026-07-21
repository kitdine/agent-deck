// Package extension discovers native client extensions without copying or
// rewriting their configuration. Management state belongs to AgentDeck only.
package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/pelletier/go-toml/v2"
)

const (
	ReadOnlyCapability = "read_only"
	unknown            = "unknown"
)

var ErrReadOnly = errors.New("extension_read_only")

type Result struct {
	Found       int            `json:"found"`
	Added       int            `json:"added"`
	Updated     int            `json:"updated"`
	Removed     int            `json:"removed"`
	Unchanged   int            `json:"unchanged"`
	Summary     map[string]int `json:"summary"`
	Roots       []string       `json:"roots,omitempty"`
	Workdir     string         `json:"workdir,omitempty"`
	Diagnostics []string       `json:"diagnostics"`
}

type DTO struct {
	ID           string   `json:"id"`
	Client       string   `json:"client"`
	Kind         string   `json:"kind"`
	Scope        string   `json:"scope"`
	NativeID     string   `json:"native_id"`
	SourcePath   string   `json:"source_path"`
	Version      string   `json:"version"`
	Enabled      string   `json:"enabled"`
	Capabilities []string `json:"capabilities"`
	Diagnostics  []string `json:"diagnostics"`
	Fingerprint  string   `json:"fingerprint"`
	Managed      bool     `json:"managed"`
	Drift        bool     `json:"drift"`
}

type DoctorReport struct {
	Diagnostics         []string `json:"diagnostics"`
	MissingPaths        []string `json:"missing_paths"`
	DuplicateIDs        []string `json:"duplicate_ids"`
	DriftedIDs          []string `json:"drifted_ids"`
	ManagementAnomalies []string `json:"management_anomalies"`
}

type nativeExtension struct {
	client, kind, scope, nativeID string
	sourcePath, version, enabled  string
}

func CanonicalID(client, kind, scope, nativeID string) (string, error) {
	for _, value := range []string{client, kind, scope, nativeID} {
		if strings.TrimSpace(value) == "" || strings.Contains(value, ":") {
			return "", fmt.Errorf("invalid extension identity")
		}
	}
	if client != "codex" && client != "claude" {
		return "", fmt.Errorf("invalid extension client")
	}
	if kind != "plugin" && kind != "mcp" && kind != "skill" {
		return "", fmt.Errorf("invalid extension kind")
	}
	if scope != "user" && scope != "project" {
		return "", fmt.Errorf("invalid extension scope")
	}
	return strings.Join([]string{client, kind, scope, nativeID}, ":"), nil
}

func Scan(ctx context.Context, db *store.Store, home, workdir string) (Result, error) {
	values, diagnostics, err := discover(home, workdir)
	if err != nil {
		return Result{}, err
	}
	previous, err := db.ListExtensions(ctx)
	if err != nil {
		return Result{}, err
	}
	old := make(map[string]store.Extension, len(previous))
	for _, item := range previous {
		old[item.ID] = item
	}
	result := Result{Found: len(values), Diagnostics: nonNil(diagnostics), Summary: map[string]int{}}
	for _, item := range values {
		result.Summary[item.Client+":"+item.Kind+":"+item.Scope]++
		if before, found := old[item.ID]; !found {
			result.Added++
		} else if before.Fingerprint != item.Fingerprint || before.SourcePath != item.SourcePath || before.Version != item.Version || before.Enabled != item.Enabled {
			result.Updated++
		} else {
			result.Unchanged++
		}
		delete(old, item.ID)
	}
	result.Removed = len(old)
	if err = db.ReplaceExtensions(ctx, values); err != nil {
		return Result{}, err
	}
	return result, nil
}

func List(ctx context.Context, db *store.Store) ([]DTO, error) {
	values, err := db.ListExtensions(ctx)
	return toDTO(values), err
}

func Show(ctx context.Context, db *store.Store, id string) (DTO, error) {
	value, err := db.ExtensionByID(ctx, id)
	return dto(value), err
}

func Adopt(ctx context.Context, db *store.Store, id string) (DTO, error) {
	value, err := db.AdoptExtension(ctx, id)
	return dto(value), err
}

func Release(ctx context.Context, db *store.Store, id string) error {
	return db.ReleaseExtension(ctx, id)
}

func SetEnabled(context.Context, *store.Store, string, bool) error {
	return ErrReadOnly
}

func Doctor(ctx context.Context, db *store.Store, home, workdir string) (DoctorReport, error) {
	report := DoctorReport{
		Diagnostics:         []string{},
		MissingPaths:        []string{},
		DuplicateIDs:        []string{},
		DriftedIDs:          []string{},
		ManagementAnomalies: []string{},
	}
	current, diagnostics, discoveryErr := discover(home, workdir)
	if discoveryErr != nil {
		report.Diagnostics = append(report.Diagnostics, discoveryErr.Error())
	} else {
		report.Diagnostics = append(report.Diagnostics, diagnostics...)
	}

	stored, err := db.ListExtensions(ctx)
	if err != nil {
		return report, err
	}
	currentByID := make(map[string]store.Extension, len(current))
	for _, value := range current {
		if _, exists := currentByID[value.ID]; exists {
			report.DuplicateIDs = append(report.DuplicateIDs, value.ID)
		}
		currentByID[value.ID] = value
	}
	for _, value := range stored {
		if value.Managed && value.AdoptedFingerprint == "" {
			report.ManagementAnomalies = append(report.ManagementAnomalies, value.ID)
		}
		if discoveryErr != nil {
			continue
		}
		live, exists := currentByID[value.ID]
		if !exists {
			report.MissingPaths = append(report.MissingPaths, value.ID)
			continue
		}
		if value.Managed && value.AdoptedFingerprint != live.Fingerprint {
			report.DriftedIDs = append(report.DriftedIDs, value.ID)
		}
	}
	sort.Strings(report.MissingPaths)
	sort.Strings(report.DuplicateIDs)
	sort.Strings(report.DriftedIDs)
	sort.Strings(report.ManagementAnomalies)
	return report, nil
}

func dto(value store.Extension) DTO {
	return DTO{
		ID:           value.ID,
		Client:       value.Client,
		Kind:         value.Kind,
		Scope:        value.Scope,
		NativeID:     value.NativeID,
		SourcePath:   value.SourcePath,
		Version:      value.Version,
		Enabled:      value.Enabled,
		Capabilities: nonNil(value.Capabilities),
		Diagnostics:  nonNil(value.Diagnostics),
		Fingerprint:  value.Fingerprint,
		Managed:      value.Managed,
		Drift:        value.Managed && value.AdoptedFingerprint != value.Fingerprint,
	}
}

func toDTO(values []store.Extension) []DTO {
	out := make([]DTO, 0, len(values))
	for _, value := range values {
		out = append(out, dto(value))
	}
	return out
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func discover(home, workdir string) ([]store.Extension, []string, error) {
	var candidates []nativeExtension
	var diagnostics []string
	scanners := []func() ([]nativeExtension, error){
		func() ([]nativeExtension, error) { return scanCodexPlugins(home, workdir) },
		func() ([]nativeExtension, error) { return scanClaudePlugins(home, workdir) },
		func() ([]nativeExtension, error) {
			return scanSkills("codex", "user", filepath.Join(home, ".codex", "skills"))
		},
		func() ([]nativeExtension, error) {
			return scanSkills("codex", "project", filepath.Join(workdir, ".codex", "skills"))
		},
		func() ([]nativeExtension, error) {
			return scanSkills("claude", "user", filepath.Join(home, ".claude", "skills"))
		},
		func() ([]nativeExtension, error) {
			return scanSkills("claude", "project", filepath.Join(workdir, ".claude", "skills"))
		},
		func() ([]nativeExtension, error) {
			return scanCodexMCP("user", filepath.Join(home, ".codex", "config.toml"))
		},
		func() ([]nativeExtension, error) {
			return scanCodexMCP("project", filepath.Join(workdir, ".codex", "config.toml"))
		},
		func() ([]nativeExtension, error) { return scanClaudeMCP("user", filepath.Join(home, ".claude.json")) },
		func() ([]nativeExtension, error) {
			return scanClaudeMCP("project", filepath.Join(workdir, ".mcp.json"))
		},
	}
	for _, scan := range scanners {
		found, err := scan()
		if err != nil {
			return nil, diagnostics, err
		}
		candidates = append(candidates, found...)
	}

	values := make([]store.Extension, 0, len(candidates))
	for _, candidate := range candidates {
		id, err := CanonicalID(candidate.client, candidate.kind, candidate.scope, candidate.nativeID)
		if err != nil {
			diagnostics = append(diagnostics, err.Error())
			continue
		}
		fingerprint, err := fingerprint(candidate.sourcePath)
		if err != nil {
			return nil, diagnostics, fmt.Errorf("fingerprint %s: %w", id, err)
		}
		values = append(values, store.Extension{
			ID:           id,
			Client:       candidate.client,
			Kind:         candidate.kind,
			Scope:        candidate.scope,
			NativeID:     candidate.nativeID,
			SourcePath:   candidate.sourcePath,
			Version:      candidate.version,
			Enabled:      candidate.enabled,
			Capabilities: []string{ReadOnlyCapability},
			Diagnostics:  []string{},
			Fingerprint:  fingerprint,
		})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, diagnostics, nil
}

func scanSkills(client, scope, path string) ([]nativeExtension, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s skills unreadable", client)
	}
	values := make([]nativeExtension, 0, len(entries))
	for _, entry := range entries {
		name, candidate := entry.Name(), filepath.Join(path, entry.Name())
		if name == ".system" {
			system, systemErr := scanSystemSkills(client, scope, candidate)
			if systemErr != nil {
				return nil, systemErr
			}
			values = append(values, system...)
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("%s skill link unavailable", client)
			}
			return nil, fmt.Errorf("%s skills unreadable", client)
		}
		if !info.IsDir() {
			continue
		}
		if _, skillErr := os.Stat(filepath.Join(candidate, "SKILL.md")); skillErr != nil {
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("%s skill link unavailable", client)
			}
			continue
		}
		values = append(values, nativeExtension{client, "skill", scope, name, candidate, unknown, unknown})
	}
	return values, nil
}

func scanSystemSkills(client, scope, path string) ([]nativeExtension, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		if info, linkErr := os.Lstat(path); linkErr == nil && info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s skill link unavailable", client)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s skills unreadable", client)
	}
	values := make([]nativeExtension, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		candidate := filepath.Join(path, entry.Name())
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("%s skill link unavailable", client)
			}
			continue
		}
		if _, skillErr := os.Stat(filepath.Join(candidate, "SKILL.md")); skillErr != nil {
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("%s skill link unavailable", client)
			}
			continue
		}
		values = append(values, nativeExtension{client, "skill", scope, ".system/" + entry.Name(), candidate, unknown, unknown})
	}
	return values, nil
}

func scanCodexPlugins(home, workdir string) ([]nativeExtension, error) {
	values, err := scanCodexPluginCache(filepath.Join(home, ".codex", "plugins", "cache"))
	if err != nil {
		return nil, err
	}
	project, err := scanPluginDirectories("codex", "project", filepath.Join(workdir, ".codex", "plugins"))
	return append(values, project...), err
}

func scanCodexPluginCache(base string) ([]nativeExtension, error) {
	marketplaces, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("codex plugins unreadable")
	}
	var values []nativeExtension
	for _, marketplace := range marketplaces {
		if !marketplace.IsDir() {
			continue
		}
		marketplacePath := filepath.Join(base, marketplace.Name())
		plugins, err := os.ReadDir(marketplacePath)
		if err != nil {
			return nil, fmt.Errorf("codex plugins unreadable")
		}
		for _, plugin := range plugins {
			if !plugin.IsDir() {
				continue
			}
			pluginPath := filepath.Join(marketplacePath, plugin.Name())
			version, sourcePath, err := codexPluginVersion(pluginPath)
			if err != nil {
				return nil, err
			}
			values = append(values, nativeExtension{"codex", "plugin", "user", plugin.Name() + "@" + marketplace.Name(), sourcePath, version, unknown})
		}
	}
	return values, nil
}

func codexPluginVersion(pluginPath string) (string, string, error) {
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return "", "", fmt.Errorf("codex plugin unreadable")
	}
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	if len(versions) != 1 {
		return unknown, pluginPath, nil
	}
	version := versions[0]
	return version, filepath.Join(pluginPath, version), nil
}

func scanPluginDirectories(client, scope, path string) ([]nativeExtension, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s plugins unreadable", client)
	}
	values := make([]nativeExtension, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			values = append(values, nativeExtension{client, "plugin", scope, entry.Name(), filepath.Join(path, entry.Name()), unknown, unknown})
		}
	}
	return values, nil
}

func scanClaudePlugins(home, workdir string) ([]nativeExtension, error) {
	path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claude plugins unreadable")
	}
	var catalog struct {
		Plugins map[string][]struct {
			Version     string `json:"version"`
			Scope       string `json:"scope"`
			ProjectPath string `json:"projectPath"`
			Enabled     *bool  `json:"enabled"`
		} `json:"plugins"`
	}
	if err = json.Unmarshal(contents, &catalog); err != nil {
		return nil, fmt.Errorf("claude plugins invalid")
	}
	var values []nativeExtension
	for id, installs := range catalog.Plugins {
		for _, install := range installs {
			scope := install.Scope
			if scope == "" {
				scope = "user"
			}
			if scope != "user" && scope != "project" {
				continue
			}
			if scope == "project" && !samePath(install.ProjectPath, workdir) {
				continue
			}
			version := install.Version
			if version == "" {
				version = unknown
			}
			enabled := unknown
			if install.Enabled != nil {
				if *install.Enabled {
					enabled = "enabled"
				} else {
					enabled = "disabled"
				}
			}
			values = append(values, nativeExtension{"claude", "plugin", scope, id, path, version, enabled})
		}
	}
	return values, nil
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func scanCodexMCP(scope, path string) ([]nativeExtension, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("codex mcp configuration unreadable")
	}
	var config map[string]any
	if err = toml.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("codex mcp configuration invalid")
	}
	servers, _ := config["mcp_servers"].(map[string]any)
	values := make([]nativeExtension, 0, len(servers))
	for name := range servers {
		values = append(values, nativeExtension{"codex", "mcp", scope, name, path, unknown, unknown})
	}
	return values, nil
}

func scanClaudeMCP(scope, path string) ([]nativeExtension, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claude mcp configuration unreadable")
	}
	var config struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err = json.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("claude mcp configuration invalid")
	}
	values := make([]nativeExtension, 0, len(config.MCPServers))
	for name := range config.MCPServers {
		values = append(values, nativeExtension{"claude", "mcp", scope, name, path, unknown, unknown})
	}
	return values, nil
}

func fingerprint(path string) (string, error) {
	hash := sha256.New()
	resolved, resolveErr := filepath.EvalSymlinks(path)
	if resolveErr != nil {
		return "", fmt.Errorf("source unavailable")
	}
	path = resolved
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("source unavailable")
	}
	if !info.IsDir() {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("source unreadable")
		}
		_, _ = hash.Write(contents)
		return hex.EncodeToString(hash.Sum(nil)), nil
	}
	var files []string
	if err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, current)
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("source unreadable")
	}
	sort.Strings(files)
	for _, current := range files {
		contents, err := os.ReadFile(current)
		if err != nil {
			return "", fmt.Errorf("source unreadable")
		}
		relative, err := filepath.Rel(path, current)
		if err != nil {
			return "", fmt.Errorf("source unreadable")
		}
		_, _ = hash.Write([]byte(relative))
		_, _ = hash.Write(contents)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
