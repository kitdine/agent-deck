// Package usagehook owns the reversible configuration lifecycle for runtime
// attribution hooks installed into Codex and Claude configuration files.
package usagehook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// MaxEventBytes bounds data accepted by the hidden hook event command.
	MaxEventBytes               = 64 * 1024
	privateFileMode fs.FileMode = 0o600
	privateDirMode  fs.FileMode = 0o700
)

type Client string

const (
	ClientCodex  Client = "codex"
	ClientClaude Client = "claude"
	ClientAll    Client = "all"
)

type ConfigurationState string

const (
	ConfigurationAbsent     ConfigurationState = "absent"
	ConfigurationConfigured ConfigurationState = "configured"
	ConfigurationModified   ConfigurationState = "modified"
	ConfigurationInvalid    ConfigurationState = "invalid"
)

type Outcome string

const (
	OutcomeConfigured Outcome = "configured"
	OutcomeUnchanged  Outcome = "unchanged"
	OutcomeRemoved    Outcome = "removed"
	OutcomeAbsent     Outcome = "absent"
	OutcomeSkipped    Outcome = "skipped"
	OutcomeFailed     Outcome = "failed"
)

type TrustState string

const (
	TrustNotApplicable          TrustState = "not_applicable"
	TrustMayRequireUserApproval TrustState = "may_require_user_approval"
)

type Environment struct {
	Home             string
	AgentDeckCommand string
}

type Request struct {
	Client Client
}

type Result struct {
	Client        Client             `json:"client"`
	Path          string             `json:"path"`
	Outcome       Outcome            `json:"outcome,omitempty"`
	Configuration ConfigurationState `json:"configuration_state"`
	Trust         TrustState         `json:"trust_state"`
	Guidance      string             `json:"guidance,omitempty"`
	Error         string             `json:"error,omitempty"`
}

type Summary struct {
	Results []Result `json:"clients"`
}

func (s Summary) HasFailures() bool {
	for _, result := range s.Results {
		if result.Outcome == OutcomeFailed || (result.Configuration == ConfigurationInvalid && result.Error != "") {
			return true
		}
	}
	return false
}

type temporaryFile interface {
	io.Writer
	Name() string
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

type fileOperations struct {
	lstat      func(string) (fs.FileInfo, error)
	readFile   func(string) ([]byte, error)
	mkdirAll   func(string, fs.FileMode) error
	chmod      func(string, fs.FileMode) error
	createTemp func(string, string) (temporaryFile, error)
	rename     func(string, string) error
	remove     func(string) error
}

func osFileOperations() fileOperations {
	return fileOperations{
		lstat:    os.Lstat,
		readFile: os.ReadFile,
		mkdirAll: os.MkdirAll,
		chmod:    os.Chmod,
		createTemp: func(dir, pattern string) (temporaryFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		rename: os.Rename,
		remove: os.Remove,
	}
}

type Manager struct {
	environment Environment
	files       fileOperations
	ownsFile    func(fs.FileInfo) bool
}

func New(environment Environment) *Manager {
	if strings.TrimSpace(environment.AgentDeckCommand) == "" {
		environment.AgentDeckCommand = "agentdeck"
	}
	return &Manager{
		environment: environment,
		files:       osFileOperations(),
		ownsFile:    ownedByCurrentUser,
	}
}

func ParseClient(value string) (Client, error) {
	switch Client(value) {
	case ClientCodex, ClientClaude, ClientAll:
		return Client(value), nil
	default:
		return "", fmt.Errorf("usage hook client must be codex, claude, or all")
	}
}

func (m *Manager) Setup(request Request) (Summary, error) {
	return m.mutate(request, false)
}

func (m *Manager) Remove(request Request) (Summary, error) {
	return m.mutate(request, true)
}

func (m *Manager) Status(request Request) (Summary, error) {
	clients, err := m.clients(request)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Results: make([]Result, 0, len(clients))}
	for _, client := range clients {
		path := m.path(client)
		result := Result{Client: client, Path: path, Trust: trustFor(client), Guidance: guidanceFor(client)}
		snapshot, readErr := m.readSnapshot(path)
		if readErr != nil {
			result.Configuration = ConfigurationInvalid
			result.Error = readErr.Error()
			summary.Results = append(summary.Results, result)
			continue
		}
		if !snapshot.exists {
			result.Configuration = ConfigurationAbsent
			summary.Results = append(summary.Results, result)
			continue
		}
		state, stateErr := m.configurationState(snapshot.document, client)
		result.Configuration = state
		if stateErr != nil {
			result.Error = stateErr.Error()
		}
		summary.Results = append(summary.Results, result)
	}
	return summary, nil
}

type snapshot struct {
	path     string
	exists   bool
	mode     fs.FileMode
	original []byte
	document map[string]json.RawMessage
}

type change struct {
	resultIndex int
	snapshot    snapshot
	updated     []byte
	mode        fs.FileMode
}

func (m *Manager) mutate(request Request, remove bool) (Summary, error) {
	clients, err := m.clients(request)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Results: make([]Result, 0, len(clients))}
	changes := make([]change, 0, len(clients))
	for index, client := range clients {
		path := m.path(client)
		result := Result{Client: client, Path: path, Trust: trustFor(client), Guidance: guidanceFor(client)}
		snapshot, readErr := m.readSnapshot(path)
		if readErr != nil {
			result.Outcome = OutcomeFailed
			result.Configuration = ConfigurationInvalid
			result.Error = readErr.Error()
			summary.Results = append(summary.Results, result)
			m.markSkipped(&summary, index+1, clients, "not attempted after another client failed")
			return summary, nil
		}
		var updated []byte
		var outcome Outcome
		var state ConfigurationState
		if remove {
			updated, outcome, state, err = m.prepareRemove(snapshot, client)
		} else {
			updated, outcome, state, err = m.prepareSetup(snapshot, client)
		}
		result.Outcome = outcome
		result.Configuration = state
		if err != nil {
			result.Outcome = OutcomeFailed
			result.Error = err.Error()
			summary.Results = append(summary.Results, result)
			m.markSkipped(&summary, index+1, clients, "not attempted after another client failed")
			return summary, nil
		}
		summary.Results = append(summary.Results, result)
		if outcome == OutcomeConfigured || outcome == OutcomeRemoved {
			mode := snapshot.mode.Perm()
			if !snapshot.exists {
				mode = privateFileMode.Perm()
			}
			changes = append(changes, change{resultIndex: index, snapshot: snapshot, updated: updated, mode: mode})
		}
	}

	committed := make([]change, 0, len(changes))
	for _, pending := range changes {
		if err := m.writeAtomic(pending.snapshot.path, pending.updated, pending.mode); err != nil {
			rollbackErr := m.rollback(committed)
			combined := errors.Join(fmt.Errorf("apply hook configuration transaction: %w", err), rollbackErr)
			for _, committedChange := range committed {
				summary.Results[committedChange.resultIndex].Outcome = OutcomeFailed
				summary.Results[committedChange.resultIndex].Error = combined.Error()
			}
			summary.Results[pending.resultIndex].Outcome = OutcomeFailed
			summary.Results[pending.resultIndex].Error = combined.Error()
			return summary, nil
		}
		committed = append(committed, pending)
	}
	return summary, nil
}

func (m *Manager) markSkipped(summary *Summary, start int, clients []Client, reason string) {
	for _, client := range clients[start:] {
		summary.Results = append(summary.Results, Result{
			Client: client, Path: m.path(client), Outcome: OutcomeSkipped,
			Configuration: ConfigurationAbsent, Trust: trustFor(client), Guidance: guidanceFor(client), Error: reason,
		})
	}
}

func (m *Manager) rollback(committed []change) error {
	var rollbackErr error
	for index := len(committed) - 1; index >= 0; index-- {
		pending := committed[index]
		var err error
		if pending.snapshot.exists {
			err = m.writeAtomic(pending.snapshot.path, pending.snapshot.original, pending.snapshot.mode.Perm())
		} else {
			err = m.removePath(pending.snapshot.path)
		}
		rollbackErr = errors.Join(rollbackErr, err)
	}
	return rollbackErr
}

func (m *Manager) prepareSetup(snapshot snapshot, client Client) ([]byte, Outcome, ConfigurationState, error) {
	hooks, hooksOrder, err := decodeHooks(snapshot.document)
	if err != nil {
		return nil, OutcomeFailed, ConfigurationInvalid, err
	}
	needsChange := !snapshot.exists
	for _, event := range hookEvents(client) {
		analysis, analysisErr := analyzeEvent(hooks[event], m.desiredEntry(client), client)
		if analysisErr != nil {
			return nil, OutcomeFailed, ConfigurationInvalid, analysisErr
		}
		if analysis.modified {
			return nil, OutcomeFailed, ConfigurationModified, fmt.Errorf("AgentDeck hook for %s/%s was modified; refusing to overwrite it", client, event)
		}
		if analysis.exact != 1 {
			needsChange = true
		}
	}
	if !needsChange {
		return nil, OutcomeUnchanged, ConfigurationConfigured, nil
	}
	for _, event := range hookEvents(client) {
		groups, existed := hooks[event]
		updated, updateErr := updateEvent(groups, m.desiredEntry(client), client, false)
		if updateErr != nil {
			return nil, OutcomeFailed, ConfigurationInvalid, updateErr
		}
		hooks[event] = updated
		if !existed {
			hooksOrder = append(hooksOrder, event)
		}
	}
	updated, err := encodeDocument(snapshot.original, snapshot.document, hooks, hooksOrder)
	if err != nil {
		return nil, OutcomeFailed, ConfigurationInvalid, err
	}
	return updated, OutcomeConfigured, ConfigurationConfigured, nil
}

func (m *Manager) prepareRemove(snapshot snapshot, client Client) ([]byte, Outcome, ConfigurationState, error) {
	if !snapshot.exists {
		return nil, OutcomeAbsent, ConfigurationAbsent, nil
	}
	hooks, hooksOrder, err := decodeHooks(snapshot.document)
	if err != nil {
		return nil, OutcomeFailed, ConfigurationInvalid, err
	}
	for _, event := range hookEvents(client) {
		analysis, analysisErr := analyzeEvent(hooks[event], m.desiredEntry(client), client)
		if analysisErr != nil {
			return nil, OutcomeFailed, ConfigurationInvalid, analysisErr
		}
		if analysis.modified {
			return nil, OutcomeFailed, ConfigurationModified, fmt.Errorf("AgentDeck hook for %s/%s was modified; refusing to remove it", client, event)
		}
	}
	removed := false
	for _, event := range hookEvents(client) {
		groups, existed := hooks[event]
		if !existed {
			continue
		}
		updated, updateErr := updateEvent(groups, m.desiredEntry(client), client, true)
		if updateErr != nil {
			return nil, OutcomeFailed, ConfigurationInvalid, updateErr
		}
		analysis, analysisErr := analyzeEvent(hooks[event], m.desiredEntry(client), client)
		if analysisErr != nil {
			return nil, OutcomeFailed, ConfigurationInvalid, analysisErr
		}
		if analysis.exact > 0 {
			removed = true
		}
		if len(updated) == 0 {
			delete(hooks, event)
			hooksOrder = removeOrderedKey(hooksOrder, event)
		} else {
			hooks[event] = updated
		}
	}
	if !removed {
		return nil, OutcomeAbsent, ConfigurationAbsent, nil
	}
	updated, err := encodeDocument(snapshot.original, snapshot.document, hooks, hooksOrder)
	if err != nil {
		return nil, OutcomeFailed, ConfigurationInvalid, err
	}
	return updated, OutcomeRemoved, ConfigurationAbsent, nil
}

type eventAnalysis struct {
	exact    int
	modified bool
}

func analyzeEvent(groups []json.RawMessage, desired json.RawMessage, client Client) (eventAnalysis, error) {
	var analysis eventAnalysis
	for _, groupRaw := range groups {
		entries, _, err := decodeGroup(groupRaw)
		if err != nil {
			return eventAnalysis{}, err
		}
		for _, entry := range entries {
			candidate, exact, classifyErr := classifyEntry(entry, desired, client)
			if classifyErr != nil {
				return eventAnalysis{}, classifyErr
			}
			if exact {
				analysis.exact++
			} else if candidate {
				analysis.modified = true
			}
		}
	}
	return analysis, nil
}

func updateEvent(groups []json.RawMessage, desired json.RawMessage, client Client, remove bool) ([]json.RawMessage, error) {
	updatedGroups := make([]json.RawMessage, 0, len(groups)+1)
	exactCount := 0
	for _, groupRaw := range groups {
		entries, group, err := decodeGroup(groupRaw)
		if err != nil {
			return nil, err
		}
		changed := false
		kept := make([]json.RawMessage, 0, len(entries))
		for _, entry := range entries {
			candidate, exact, classifyErr := classifyEntry(entry, desired, client)
			if classifyErr != nil {
				return nil, classifyErr
			}
			if candidate && !exact {
				return nil, fmt.Errorf("AgentDeck hook for %s was modified; refusing to overwrite it", client)
			}
			if exact {
				exactCount++
				if remove || exactCount > 1 {
					changed = true
					continue
				}
			}
			kept = append(kept, entry)
		}
		if !changed {
			updatedGroups = append(updatedGroups, groupRaw)
			continue
		}
		if len(kept) == 0 && len(group) == 1 {
			continue
		}
		group["hooks"], err = json.Marshal(kept)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(group)
		if err != nil {
			return nil, err
		}
		updatedGroups = append(updatedGroups, encoded)
	}
	if !remove && exactCount == 0 {
		updatedGroups = append(updatedGroups, json.RawMessage(fmt.Sprintf(`{"hooks":[%s]}`, desired)))
	}
	return updatedGroups, nil
}

func decodeGroup(raw json.RawMessage) ([]json.RawMessage, map[string]json.RawMessage, error) {
	var group map[string]json.RawMessage
	if err := json.Unmarshal(raw, &group); err != nil || group == nil {
		if err == nil {
			err = errors.New("hook group must be an object")
		}
		return nil, nil, fmt.Errorf("invalid hook group: %w", err)
	}
	hooksRaw, ok := group["hooks"]
	if !ok {
		return nil, group, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(hooksRaw, &entries); err != nil || entries == nil {
		if err == nil {
			err = errors.New("hooks must be an array")
		}
		return nil, nil, fmt.Errorf("invalid hook group entries: %w", err)
	}
	return entries, group, nil
}

func classifyEntry(raw, desired json.RawMessage, client Client) (candidate, exact bool, err error) {
	var entry map[string]json.RawMessage
	if err = json.Unmarshal(raw, &entry); err != nil || entry == nil {
		if err == nil {
			err = errors.New("hook entry must be an object")
		}
		return false, false, fmt.Errorf("invalid hook entry: %w", err)
	}
	var kind, command string
	if rawKind, ok := entry["type"]; ok {
		_ = json.Unmarshal(rawKind, &kind)
	}
	if rawCommand, ok := entry["command"]; ok {
		_ = json.Unmarshal(rawCommand, &command)
	}
	trimmed := strings.TrimSpace(command)
	candidate = kind == "command" && managedHookCommand(trimmed, client)
	if !candidate {
		return false, false, nil
	}
	exact = managedEntryEquivalent(raw, desired, client)
	return candidate, exact, nil
}

func jsonEquivalent(left, right []byte) bool {
	var leftCompact, rightCompact bytes.Buffer
	if json.Compact(&leftCompact, left) != nil || json.Compact(&rightCompact, right) != nil {
		return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
	}
	return bytes.Equal(leftCompact.Bytes(), rightCompact.Bytes())
}

func managedEntryEquivalent(left, right []byte, client Client) bool {
	if jsonEquivalent(left, right) {
		return true
	}
	var leftEntry, rightEntry map[string]json.RawMessage
	if json.Unmarshal(left, &leftEntry) != nil || json.Unmarshal(right, &rightEntry) != nil {
		return false
	}
	var leftCommand, rightCommand string
	if json.Unmarshal(leftEntry["command"], &leftCommand) != nil || json.Unmarshal(rightEntry["command"], &rightCommand) != nil {
		return false
	}
	if !managedHookCommand(leftCommand, client) || !managedHookCommand(rightCommand, client) {
		return false
	}
	delete(leftEntry, "command")
	delete(rightEntry, "command")
	leftRemainder, err := json.Marshal(leftEntry)
	if err != nil {
		return false
	}
	rightRemainder, err := json.Marshal(rightEntry)
	if err != nil {
		return false
	}
	return jsonEquivalent(leftRemainder, rightRemainder)
}

func managedHookCommand(command string, client Client) bool {
	trimmed := strings.TrimSpace(command)
	marker := " usage hook event " + string(client)
	if !strings.HasSuffix(trimmed, marker) {
		return false
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(trimmed, marker))
	if prefix == "agentdeck" {
		return true
	}
	const statePrefix = "agentdeck --state-dir "
	if !strings.HasPrefix(prefix, statePrefix) {
		return false
	}
	stateDir := strings.TrimSpace(strings.TrimPrefix(prefix, statePrefix))
	if stateDir == "" {
		return false
	}
	if strings.HasPrefix(stateDir, "'") && strings.HasSuffix(stateDir, "'") {
		return true
	}
	return !strings.ContainsAny(stateDir, " \t\r\n")
}

func decodeHooks(document map[string]json.RawMessage) (map[string][]json.RawMessage, []string, error) {
	hooks := make(map[string][]json.RawMessage)
	raw, ok := document["hooks"]
	if !ok {
		return hooks, nil, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		if err == nil {
			err = errors.New("hooks must be an object")
		}
		return nil, nil, fmt.Errorf("invalid hook configuration: %w", err)
	}
	order, err := orderedObjectKeys(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid hook configuration: %w", err)
	}
	for event, groupsRaw := range object {
		var groups []json.RawMessage
		if err := json.Unmarshal(groupsRaw, &groups); err != nil || groups == nil {
			if err == nil {
				err = errors.New("hook event entries must be an array")
			}
			return nil, nil, fmt.Errorf("invalid %s hook entries: %w", event, err)
		}
		hooks[event] = groups
	}
	return hooks, order, nil
}

func encodeDocument(original []byte, document map[string]json.RawMessage, hooks map[string][]json.RawMessage, hooksOrder []string) ([]byte, error) {
	encodedHooks, err := marshalOrderedHooks(hooks, hooksOrder)
	if err != nil {
		return nil, err
	}
	if len(original) == 0 {
		encoded := append([]byte(`{"hooks":`), encodedHooks...)
		encoded = append(encoded, '}', '\n')
		return encoded, nil
	}
	start, end, found, err := topLevelValueSpan(original, "hooks")
	if err != nil {
		return nil, err
	}
	if found {
		encoded := make([]byte, 0, len(original)-end+start+len(encodedHooks))
		encoded = append(encoded, original[:start]...)
		encoded = append(encoded, encodedHooks...)
		encoded = append(encoded, original[end:]...)
		return encoded, nil
	}
	return insertTopLevelValue(original, "hooks", encodedHooks, len(document) > 0)
}

func orderedObjectKeys(raw []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, errors.New("JSON value must be an object")
	}
	order := make([]string, 0)
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("JSON object key must be a string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			order = append(order, key)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("JSON object has trailing data")
		}
		return nil, err
	}
	return order, nil
}

func marshalOrderedHooks(hooks map[string][]json.RawMessage, order []string) ([]byte, error) {
	keys := make([]string, 0, len(hooks))
	seen := make(map[string]struct{}, len(hooks))
	for _, key := range order {
		if _, exists := hooks[key]; exists {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	missing := make([]string, 0, len(hooks)-len(keys))
	for key := range hooks {
		if _, exists := seen[key]; !exists {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	keys = append(keys, missing...)

	var encoded bytes.Buffer
	encoded.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			encoded.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		groupsJSON, err := json.Marshal(hooks[key])
		if err != nil {
			return nil, err
		}
		encoded.Write(keyJSON)
		encoded.WriteByte(':')
		encoded.Write(groupsJSON)
	}
	encoded.WriteByte('}')
	return encoded.Bytes(), nil
}

func topLevelValueSpan(contents []byte, target string) (start, end int, found bool, err error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return 0, 0, false, errors.New("hook configuration must be a JSON object")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, 0, false, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, 0, false, errors.New("JSON object key must be a string")
		}
		offset := int(decoder.InputOffset())
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return 0, 0, false, err
		}
		value := bytes.TrimSpace(raw)
		relative := bytes.Index(contents[offset:], value)
		if relative < 0 {
			return 0, 0, false, errors.New("cannot locate JSON object value")
		}
		if key == target {
			start = offset + relative
			end = start + len(value)
			found = true
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, 0, false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return 0, 0, false, errors.New("hook configuration has trailing data")
		}
		return 0, 0, false, err
	}
	return start, end, found, nil
}

func insertTopLevelValue(contents []byte, key string, value []byte, hasFields bool) ([]byte, error) {
	trimmed := bytes.TrimRight(contents, " \t\r\n")
	if len(trimmed) == 0 || trimmed[len(trimmed)-1] != '}' {
		return nil, errors.New("hook configuration must be a JSON object")
	}
	closeIndex := len(trimmed) - 1
	encoded := make([]byte, 0, len(contents)+len(value)+len(key)+4)
	encoded = append(encoded, trimmed[:closeIndex]...)
	if hasFields {
		encoded = append(encoded, ',')
	}
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, keyJSON...)
	encoded = append(encoded, ':')
	encoded = append(encoded, value...)
	encoded = append(encoded, trimmed[closeIndex:]...)
	encoded = append(encoded, contents[len(trimmed):]...)
	return encoded, nil
}

func removeOrderedKey(order []string, target string) []string {
	filtered := make([]string, 0, len(order))
	for _, key := range order {
		if key != target {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

func decodeDocument(contents []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var document map[string]json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("invalid hook configuration JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("invalid hook configuration JSON: trailing data")
		}
		return nil, fmt.Errorf("invalid hook configuration JSON: %w", err)
	}
	if document == nil {
		return nil, errors.New("hook configuration must be a JSON object")
	}
	return document, nil
}

func (m *Manager) configurationState(document map[string]json.RawMessage, client Client) (ConfigurationState, error) {
	hooks, _, err := decodeHooks(document)
	if err != nil {
		return ConfigurationInvalid, err
	}
	total := 0
	incomplete := false
	for _, event := range hookEvents(client) {
		analysis, analysisErr := analyzeEvent(hooks[event], m.desiredEntry(client), client)
		if analysisErr != nil {
			return ConfigurationInvalid, analysisErr
		}
		if analysis.modified || analysis.exact > 1 {
			return ConfigurationModified, nil
		}
		total += analysis.exact
		if analysis.exact != 1 {
			incomplete = true
		}
	}
	if total == 0 {
		return ConfigurationAbsent, nil
	}
	if !incomplete && total == len(hookEvents(client)) {
		return ConfigurationConfigured, nil
	}
	return ConfigurationModified, nil
}

func (m *Manager) desiredEntry(client Client) json.RawMessage {
	command := strings.TrimSpace(m.environment.AgentDeckCommand)
	if command == "" {
		command = "agentdeck"
	}
	encoded, _ := json.Marshal(map[string]string{
		"type":    "command",
		"command": command + " usage hook event " + string(client),
	})
	return encoded
}

func hookEvents(client Client) []string {
	switch client {
	case ClientCodex:
		return []string{"SessionStart"}
	case ClientClaude:
		return []string{"SessionStart", "ConfigChange", "SessionEnd"}
	default:
		return nil
	}
}

func (m *Manager) clients(request Request) ([]Client, error) {
	client := request.Client
	if client == "" {
		client = ClientAll
	}
	switch client {
	case ClientAll:
		return []Client{ClientCodex, ClientClaude}, nil
	case ClientCodex, ClientClaude:
		return []Client{client}, nil
	default:
		return nil, fmt.Errorf("usage hook client must be codex, claude, or all")
	}
}

func (m *Manager) path(client Client) string {
	if client == ClientCodex {
		return filepath.Join(m.environment.Home, ".codex", "hooks.json")
	}
	return filepath.Join(m.environment.Home, ".claude", "settings.json")
}

func (m *Manager) readSnapshot(path string) (snapshot, error) {
	result := snapshot{path: path, document: map[string]json.RawMessage{}}
	info, err := m.files.lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return snapshot{}, fmt.Errorf("inspect hook configuration: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return snapshot{}, errors.New("hook configuration file is symlink")
	}
	if !info.Mode().IsRegular() {
		return snapshot{}, errors.New("hook configuration file is not regular file")
	}
	if !m.ownsFile(info) {
		return snapshot{}, errors.New("hook configuration file not owned by current user")
	}
	contents, err := m.files.readFile(path)
	if err != nil {
		return snapshot{}, fmt.Errorf("read hook configuration: %w", err)
	}
	document, err := decodeDocument(contents)
	if err != nil {
		return snapshot{}, err
	}
	result.exists = true
	result.mode = info.Mode()
	result.original = contents
	result.document = document
	return result, nil
}

func (m *Manager) writeAtomic(path string, contents []byte, mode fs.FileMode) error {
	directory := filepath.Dir(path)
	if err := m.files.mkdirAll(directory, privateDirMode); err != nil {
		return fmt.Errorf("create hook configuration directory: %w", err)
	}
	if err := m.files.chmod(directory, privateDirMode); err != nil {
		return fmt.Errorf("secure hook configuration directory: %w", err)
	}
	temporary, err := m.files.createTemp(directory, ".agentdeck-usage-hook-*")
	if err != nil {
		return fmt.Errorf("create hook configuration temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer m.files.remove(temporaryPath)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure hook configuration temporary file: %w", err)
	}
	written, err := temporary.Write(contents)
	if err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write hook configuration temporary file: %w", err)
	}
	if written != len(contents) {
		_ = temporary.Close()
		return io.ErrShortWrite
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync hook configuration temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close hook configuration temporary file: %w", err)
	}
	if err := m.files.rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace hook configuration: %w", err)
	}
	return nil
}

func (m *Manager) removePath(path string) error {
	info, err := m.files.lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() || !m.ownsFile(info) {
		return errors.New("refusing to remove hook configuration changed during transaction")
	}
	return m.files.remove(path)
}

func trustFor(client Client) TrustState {
	if client == ClientCodex {
		return TrustMayRequireUserApproval
	}
	return TrustNotApplicable
}

func guidanceFor(client Client) string {
	if client == ClientCodex {
		return "Codex may require approving the new command hook through /hooks; AgentDeck does not change Codex trust state"
	}
	return ""
}
