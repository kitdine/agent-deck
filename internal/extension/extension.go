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
	"unicode"
	"unicode/utf8"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/pelletier/go-toml/v2"
)

const (
	ReadOnlyCapability = "read_only"
	unknown            = "unknown"
)

var ErrReadOnly = errors.New("extension_read_only")

type ErrExtensionSyncIncomplete struct{}

func (e *ErrExtensionSyncIncomplete) Error() string {
	return "extension_sync_incomplete: extension inventory changed but extension scan fingerprint update failed"
}

type ErrExtensionInventoryUnreadable struct {
	Err error
}

func (e *ErrExtensionInventoryUnreadable) Error() string {
	return "extension_inventory_unreadable: AgentDeck extension inventory database is unreadable or malformed"
}

func (e *ErrExtensionInventoryUnreadable) Unwrap() error {
	return e.Err
}

type ExtensionDiscoverer interface {
	Discover(home, workdir string) ([]store.Extension, []string, error)
}

type defaultDiscoverer struct{}

func (d defaultDiscoverer) Discover(home, workdir string) ([]store.Extension, []string, error) {
	return discover(home, workdir)
}

var DefaultDiscoverer ExtensionDiscoverer = defaultDiscoverer{}

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
	DiscoveryStatus           string   `json:"discovery_status"`
	Diagnostics               []string `json:"diagnostics"`
	StaleInventory            []string `json:"stale_inventory"`
	MissingPaths              []string `json:"missing_paths"`
	DuplicateIDs              []string `json:"duplicate_ids"`
	DriftedIDs                []string `json:"drifted_ids"`
	ManagementAnomalies       []string `json:"management_anomalies"`
	NativeUnavailable         []string `json:"native_unavailable"`
	FingerprintSyncIncomplete bool     `json:"fingerprint_sync_incomplete"`
	Reason                    string   `json:"reason,omitempty"`
	ActionKind                string   `json:"action_kind,omitempty"`
	RecoveryCommand           *string  `json:"recovery_command"`
	ManualPrerequisite        *string  `json:"manual_prerequisite"`
}

func (r DoctorReport) CountForReason() int {
	switch r.Reason {
	case "extension_duplicate_id":
		return len(r.DuplicateIDs)
	case "extension_management_anomaly":
		return len(r.ManagementAnomalies)
	case "extension_managed_drift":
		return len(r.DriftedIDs)
	case "extension_stale_inventory":
		return len(r.StaleInventory)
	case "extension_native_unavailable":
		return len(r.NativeUnavailable)
	case "extension_fingerprint_update_failed", "extension_discovery_failed", "extension_state_missing", "extension_inventory_unreadable":
		return 1
	default:
		return 0
	}
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
	return ScanWithDiscoverer(ctx, db, DefaultDiscoverer, home, workdir)
}

func ScanWithDiscoverer(ctx context.Context, db *store.Store, discoverer ExtensionDiscoverer, home, workdir string) (Result, error) {
	if discoverer == nil {
		discoverer = DefaultDiscoverer
	}
	values, diagnostics, err := discoverer.Discover(home, workdir)
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
	sanitizedDiags := make([]string, 0, len(diagnostics))
	for _, d := range diagnostics {
		sanitizedDiags = append(sanitizedDiags, SanitizeDiagnostic(d))
	}
	result := Result{Found: len(values), Diagnostics: nonNil(sanitizedDiags), Summary: map[string]int{}}
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
	return DoctorWithDiscoverer(ctx, db, DefaultDiscoverer, home, workdir)
}

func DoctorWithDiscoverer(ctx context.Context, db *store.Store, discoverer ExtensionDiscoverer, home, workdir string) (DoctorReport, error) {
	report := DoctorReport{
		DiscoveryStatus: "ok",
		Diagnostics:     []string{},
	}
	if discoverer == nil {
		discoverer = DefaultDiscoverer
	}
	current, diagnostics, discoveryErr := discoverer.Discover(home, workdir)
	if discoveryErr != nil {
		report.DiscoveryStatus = "failed"
		report.Diagnostics = []string{SanitizeDiagnostic(discoveryErr.Error())}
	} else {
		for _, d := range diagnostics {
			report.Diagnostics = append(report.Diagnostics, SanitizeDiagnostic(d))
		}
	}

	if db == nil {
		report.Reason = "extension_state_missing"
		report.StaleInventory = nil
		report.MissingPaths = nil
		report.DuplicateIDs = nil
		report.DriftedIDs = nil
		report.ManagementAnomalies = nil
		report.NativeUnavailable = nil
		applyReasonAction(&report)
		return report, nil
	}

	stored, err := db.ListExtensions(ctx)
	if err != nil {
		report.Reason = "extension_inventory_unreadable"
		applyReasonAction(&report)
		return report, &ErrExtensionInventoryUnreadable{Err: err}
	}

	syncIncomplete, _, err := db.Setting(ctx, "extension.sync_incomplete")
	if err != nil {
		report.Reason = "extension_inventory_unreadable"
		applyReasonAction(&report)
		return report, &ErrExtensionInventoryUnreadable{Err: err}
	}

	if discoveryErr != nil {
		report.Reason = "extension_discovery_failed"
		report.StaleInventory = nil
		report.MissingPaths = nil
		report.DuplicateIDs = nil
		report.DriftedIDs = nil
		report.ManagementAnomalies = nil
		report.NativeUnavailable = nil
		applyReasonAction(&report)
		return report, nil
	}

	report.StaleInventory = []string{}
	report.MissingPaths = []string{}
	report.DuplicateIDs = []string{}
	report.DriftedIDs = []string{}
	report.ManagementAnomalies = []string{}
	report.NativeUnavailable = []string{}
	currentByID := make(map[string]store.Extension, len(current))
	duplicateIDs := make(map[string]struct{})
	nativeUnavailable := make(map[string]struct{})
	for _, value := range current {
		if _, exists := currentByID[value.ID]; exists {
			if _, reported := duplicateIDs[value.ID]; !reported {
				report.DuplicateIDs = append(report.DuplicateIDs, value.ID)
				duplicateIDs[value.ID] = struct{}{}
			}
		}
		currentByID[value.ID] = value
		if len(value.Diagnostics) > 0 {
			nativeUnavailable[value.ID] = struct{}{}
		}
	}
	for id := range nativeUnavailable {
		report.NativeUnavailable = append(report.NativeUnavailable, id)
	}
	for _, value := range stored {
		if value.Managed && value.AdoptedFingerprint == "" {
			report.ManagementAnomalies = append(report.ManagementAnomalies, value.ID)
		}
		live, exists := currentByID[value.ID]
		if !exists {
			report.StaleInventory = append(report.StaleInventory, value.ID)
			continue
		}
		if len(live.Diagnostics) == 0 && value.Managed && live.Fingerprint != "" && value.AdoptedFingerprint != live.Fingerprint {
			report.DriftedIDs = append(report.DriftedIDs, value.ID)
		}
	}
	sort.Strings(report.StaleInventory)
	report.MissingPaths = report.StaleInventory
	sort.Strings(report.DuplicateIDs)
	sort.Strings(report.DriftedIDs)
	sort.Strings(report.ManagementAnomalies)
	sort.Strings(report.NativeUnavailable)

	if syncIncomplete == "true" {
		report.FingerprintSyncIncomplete = true
	}

	classifyReport(&report)
	applyReasonAction(&report)
	return report, nil
}

func DoctorFromStateRoot(ctx context.Context, stateRoot, home, workdir string) (DoctorReport, error) {
	return DoctorFromStateRootWithDiscoverer(ctx, stateRoot, home, workdir, DefaultDiscoverer)
}

func DoctorFromStateRootWithDiscoverer(ctx context.Context, stateRoot, home, workdir string, discoverer ExtensionDiscoverer) (DoctorReport, error) {
	if discoverer == nil {
		discoverer = DefaultDiscoverer
	}
	dbPath := filepath.Join(stateRoot, "agentdeck.sqlite3")
	if _, statErr := os.Stat(dbPath); errors.Is(statErr, fs.ErrNotExist) {
		return handleStateMissing(home, workdir, discoverer)
	} else if statErr != nil {
		if _, rootErr := os.Stat(stateRoot); errors.Is(rootErr, fs.ErrNotExist) {
			return handleStateMissing(home, workdir, discoverer)
		}
	}
	db, err := store.OpenReadOnly(ctx, stateRoot)
	if err != nil {
		var ahead *store.SchemaAhead
		if errors.As(err, &ahead) {
			return DoctorReport{}, ahead
		}
		if errors.Is(err, fs.ErrNotExist) {
			return handleStateMissing(home, workdir, discoverer)
		}
		report := DoctorReport{Reason: "extension_inventory_unreadable"}
		applyReasonAction(&report)
		return report, &ErrExtensionInventoryUnreadable{Err: err}
	}
	defer db.Close()
	return DoctorWithDiscoverer(ctx, db, discoverer, home, workdir)
}

func handleStateMissing(home, workdir string, discoverer ExtensionDiscoverer) (DoctorReport, error) {
	_, diags, discoveryErr := discoverer.Discover(home, workdir)
	report := DoctorReport{
		Reason: "extension_state_missing",
	}
	if discoveryErr != nil {
		report.DiscoveryStatus = "failed"
		report.Diagnostics = []string{SanitizeDiagnostic(discoveryErr.Error())}
	} else {
		report.DiscoveryStatus = "ok"
		report.Diagnostics = make([]string, 0, len(diags))
		for _, d := range diags {
			report.Diagnostics = append(report.Diagnostics, SanitizeDiagnostic(d))
		}
	}
	applyReasonAction(&report)
	return report, nil
}

func classifyReport(report *DoctorReport) {
	if report.Reason != "" {
		return
	}
	switch {
	case report.DiscoveryStatus == "failed":
		report.Reason = "extension_discovery_failed"
	case len(report.DuplicateIDs) > 0:
		report.Reason = "extension_duplicate_id"
	case len(report.ManagementAnomalies) > 0:
		report.Reason = "extension_management_anomaly"
	case len(report.DriftedIDs) > 0:
		report.Reason = "extension_managed_drift"
	case len(report.StaleInventory) > 0:
		report.Reason = "extension_stale_inventory"
	case len(report.NativeUnavailable) > 0:
		report.Reason = "extension_native_unavailable"
	case report.FingerprintSyncIncomplete:
		report.Reason = "extension_fingerprint_update_failed"
	}
}

func applyReasonAction(report *DoctorReport) {
	switch report.Reason {
	case "extension_state_missing":
		if report.DiscoveryStatus == "ok" {
			report.ActionKind = "synchronize_inventory"
			report.RecoveryCommand = strPtr("agentdeck extension scan")
			report.ManualPrerequisite = nil
		} else {
			report.ActionKind = "manual_prerequisite"
			report.RecoveryCommand = nil
			report.ManualPrerequisite = strPtr("prereq_extension_discovery_failed")
		}
	case "extension_inventory_unreadable":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_inventory_unreadable")
	case "extension_discovery_failed":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_discovery_failed")
	case "extension_duplicate_id":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_duplicate_id")
	case "extension_management_anomaly":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_management_anomaly")
	case "extension_managed_drift":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_managed_drift")
	case "extension_stale_inventory":
		report.ActionKind = "synchronize_inventory"
		report.RecoveryCommand = strPtr("agentdeck extension scan")
		report.ManualPrerequisite = nil
	case "extension_native_unavailable":
		report.ActionKind = "manual_prerequisite"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = strPtr("prereq_extension_native_unavailable")
	case "extension_fingerprint_update_failed":
		report.ActionKind = "diagnose"
		report.RecoveryCommand = nil
		report.ManualPrerequisite = nil
	default:
		report.ActionKind = ""
		report.RecoveryCommand = nil
		report.ManualPrerequisite = nil
	}
}

func strPtr(s string) *string {
	return &s
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
		Drift:        value.Managed && value.Fingerprint != "" && value.AdoptedFingerprint != value.Fingerprint,
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
		itemDiags := []string{}
		fp, fpErr := fingerprint(candidate.sourcePath)
		if fpErr != nil {
			if strings.Contains(fpErr.Error(), "unreadable") {
				itemDiags = append(itemDiags, "source_unreadable")
			} else {
				itemDiags = append(itemDiags, "source_unavailable")
			}
			fp = ""
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
			Diagnostics:  itemDiags,
			Fingerprint:  fp,
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

func SanitizeDiagnostic(raw string) string {
	var clean strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] >= 0x80 && raw[i] <= 0x9f {
			clean.WriteByte(' ')
			switch raw[i] {
			case 0x9b:
				i = skipDiagnosticCSI(raw, i+1)
			case 0x9d:
				i = skipDiagnosticString(raw, i+1, true)
			case 0x90, 0x98, 0x9e, 0x9f:
				i = skipDiagnosticString(raw, i+1, false)
			default:
				i++
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(raw[i:])
		if r == utf8.RuneError && size == 1 {
			clean.WriteRune('\uFFFD')
			i++
			continue
		}
		switch r {
		case '\x1b':
			clean.WriteByte(' ')
			i = skipDiagnosticEscape(raw, i+size)
		case '\u009b':
			clean.WriteByte(' ')
			i = skipDiagnosticCSI(raw, i+size)
		case '\u009d':
			clean.WriteByte(' ')
			i = skipDiagnosticString(raw, i+size, true)
		case '\u0090', '\u0098', '\u009e', '\u009f':
			clean.WriteByte(' ')
			i = skipDiagnosticString(raw, i+size, false)
		default:
			if unicode.IsControl(r) || unicode.IsSpace(r) {
				clean.WriteByte(' ')
			} else {
				clean.WriteRune(r)
			}
			i += size
		}
	}
	fields := strings.Fields(clean.String())
	return strings.Join(fields, " ")
}

func skipDiagnosticEscape(s string, i int) int {
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '[':
		return skipDiagnosticCSI(s, i+1)
	case ']':
		return skipDiagnosticString(s, i+1, true)
	case 'P', 'X', '^', '_':
		return skipDiagnosticString(s, i+1, false)
	default:
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
			i++
		}
		if i < len(s) {
			i++
		}
		return i
	}
}

func skipDiagnosticCSI(s string, i int) int {
	for i < len(s) {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
		i++
	}
	return i
}

func skipDiagnosticString(s string, i int, osc bool) int {
	for i < len(s) {
		if s[i] == 0x9c {
			return i + 1
		}
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\u009c' || (osc && r == '\a') {
			return i + size
		}
		i += size
	}
	return i
}
