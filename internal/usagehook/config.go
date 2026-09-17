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
	// OutcomeRestoreIncomplete is RestoreStatusLine's disposition when the
	// current statusLine value no longer matches what AgentDeck registered
	// (subscription-quota C3/C9): AgentDeck's command is not written back
	// over, since the file changed underneath in a way that makes silently
	// restoring the recorded prior value a potential overwrite of a
	// deliberate edit. Distinct from OutcomeFailed: this is an expected,
	// reportable disposition, not an error.
	OutcomeRestoreIncomplete Outcome = "restore_incomplete"
)

type TrustState string

const (
	TrustNotApplicable          TrustState = "not_applicable"
	TrustMayRequireUserApproval TrustState = "may_require_user_approval"
)

type Environment struct {
	Home             string
	AgentDeckCommand string
	// StateDir is where SetupStatusLine/RestoreStatusLine record the prior
	// statusLine value (CLA-R1-F1): kept in AgentDeck's own state rather than
	// in ~/.claude/settings.json, so the only key AgentDeck ever writes
	// there is "statusLine" itself — matching what registration is
	// authorized to write (ux/settings-quota.md). Required only for
	// SetupStatusLine, RestoreStatusLine, and PriorStatusLineCommand.
	StateDir string
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

// Claude Code's status-line registration (subscription-quota architecture.md
// C3). Unlike the "hooks" key above — an array AgentDeck adds one entry
// to — "statusLine" is a singleton: only one command can be configured at a
// time, so registering AgentDeck's own command means replacing whatever was
// there, and restoring means putting it back. The prior value therefore has
// to be remembered somewhere; it is kept in AgentDeck's own state (see
// statusLinePriorPath), not in ~/.claude/settings.json (CLA-R1-F1) — the
// only key AgentDeck ever writes there is statusLineKey itself.
const statusLineKey = "statusLine"

const statusLinePriorFileName = "usagehook-statusline-prior.json"

// statusLinePriorRecord is what SetupStatusLine persists under
// statusLinePriorPath: Existed distinguishes "there was no statusLine before
// AgentDeck" (Existed == false, Value unset) from "there was one, and this
// is it" — the two restore to different outcomes (removing the key entirely
// vs. writing the recorded value back).
//
// Raw carries the value's exact bytes as a JSON string. Encoding Value alone
// compacts it, so a hand-formatted statusLine would come back on one line.
// Records written before Raw existed restore from Value.
type statusLinePriorRecord struct {
	Existed bool            `json:"existed"`
	Value   json.RawMessage `json:"value,omitempty"`
	Raw     string          `json:"raw,omitempty"`
}

type statusLineCommandEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

func (m *Manager) desiredStatusLineEntry() json.RawMessage {
	command := strings.TrimSpace(m.environment.AgentDeckCommand)
	if command == "" {
		command = "agentdeck"
	}
	encoded, _ := json.Marshal(statusLineCommandEntry{Type: "command", Command: command + " quota capture"})
	return encoded
}

// managedStatusLineCommand mirrors managedHookCommand's suffix/prefix check
// for the "quota capture" marker, so a value that looks like AgentDeck's own
// command is recognized independent of the exact --state-dir prefix.
func managedStatusLineCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	const marker = " quota capture"
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

func decodeStatusLineCommandEntry(raw json.RawMessage) (command string, ok bool) {
	var entry statusLineCommandEntry
	if json.Unmarshal(raw, &entry) != nil || entry.Type != "command" {
		return "", false
	}
	return entry.Command, true
}

// SetupStatusLine registers AgentDeck's status-line capture command in
// ~/.claude/settings.json — the only key this writes there is "statusLine"
// itself (CLA-R1-F1) — after recording whatever was previously there,
// including its absence, to AgentDeck's own state (writeStatusLinePrior), so
// RestoreStatusLine and runtime chaining (PriorStatusLineCommand) can find
// it later. Idempotent: re-running once AgentDeck's own command is already
// registered reports Unchanged rather than re-capturing itself as the prior
// value.
func (m *Manager) SetupStatusLine() (Result, error) {
	path := m.path(ClientClaude)
	result := Result{Client: ClientClaude, Path: path, Trust: TrustNotApplicable}

	snap, err := m.readSnapshot(path)
	if err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}

	currentRaw, currentFound, err := topLevelValueSpanRaw(snap.original, statusLineKey)
	if err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}
	if currentFound && jsonEquivalent(currentRaw, m.desiredStatusLineEntry()) {
		// Exact match only: managedStatusLineCommand alone recognizes any
		// AgentDeck installation's route regardless of --state-dir (by
		// design, for RestoreStatusLine's safety), so using it here would
		// report Unchanged for a *different* state dir's registration and
		// leave this instance's own route never actually installed. A
		// managed entry that is not an exact match falls through below,
		// recording it as the prior value like any other rewrite.
		result.Outcome, result.Configuration = OutcomeUnchanged, ConfigurationConfigured
		return result, nil
	}

	prior := statusLinePriorRecord{Existed: currentFound}
	if currentFound {
		prior.Value = currentRaw
	}
	if err := m.writeStatusLinePrior(prior); err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}

	updated, err := setTopLevelValue(snap.original, statusLineKey, m.desiredStatusLineEntry())
	if err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}

	mode := snap.mode.Perm()
	if !snap.exists {
		mode = privateFileMode.Perm()
	}
	if err := m.writeAtomic(path, updated, mode); err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}
	result.Outcome, result.Configuration = OutcomeConfigured, ConfigurationConfigured
	return result, nil
}

// RestoreStatusLine implements C3's restore, with three distinct cases
// (CLA-R1-F1, CLA-R1-F2):
//
//  1. The current "statusLine" value is still exactly what SetupStatusLine
//     wrote: safe to fully restore. The key is removed entirely if there was
//     no prior value, or replaced with the recorded prior value verbatim if
//     there was — never left as a literal null standing in for "absent".
//     Outcome=Removed.
//  2. The current value has changed but is still recognizably AgentDeck's
//     (managedStatusLineCommand) — someone added a field to it, for
//     example. AgentDeck's command is removed outright (the key is deleted)
//     rather than either leaving it installed or guessing whether the old
//     recorded prior should win over whatever the edit intended.
//     Outcome=RestoreIncomplete, and the file is not left with AgentDeck's
//     command still active.
//  3. The current value is not AgentDeck's at all anymore (already replaced
//     by something else): left completely untouched. Outcome=RestoreIncomplete.
//
// Case 1 is the only one where the recorded prior value is ever written
// back; cases 2 and 3 never overwrite a value with something the user might
// not recognize (C3: "removes AgentDeck's command and reports that a manual
// check is needed rather than overwriting a value the user may have edited
// deliberately").
func (m *Manager) RestoreStatusLine() (Result, error) {
	path := m.path(ClientClaude)
	result := Result{Client: ClientClaude, Path: path, Trust: TrustNotApplicable}

	snap, err := m.readSnapshot(path)
	if err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}
	if !snap.exists {
		result.Outcome, result.Configuration = OutcomeAbsent, ConfigurationAbsent
		return result, nil
	}

	currentRaw, currentFound, err := topLevelValueSpanRaw(snap.original, statusLineKey)
	if err != nil {
		result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
		return result, nil
	}
	if !currentFound {
		result.Outcome, result.Configuration = OutcomeAbsent, ConfigurationAbsent
		return result, nil
	}

	if jsonEquivalent(currentRaw, m.desiredStatusLineEntry()) {
		prior, priorFound, priorErr := m.readStatusLinePrior()
		if priorErr != nil {
			result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, priorErr.Error()
			return result, nil
		}
		var updated []byte
		if priorFound && prior.Existed {
			updated, err = setTopLevelValue(snap.original, statusLineKey, prior.Value)
		} else {
			updated, err = removeTopLevelValue(snap.original, statusLineKey)
		}
		if err != nil {
			result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
			return result, nil
		}
		if err := m.writeAtomic(path, updated, snap.mode.Perm()); err != nil {
			result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
			return result, nil
		}
		result.Outcome, result.Configuration = OutcomeRemoved, ConfigurationAbsent
		return result, nil
	}

	command, isCommand := decodeStatusLineCommandEntry(currentRaw)
	if isCommand && managedStatusLineCommand(command) {
		updated, err := removeTopLevelValue(snap.original, statusLineKey)
		if err != nil {
			result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
			return result, nil
		}
		if err := m.writeAtomic(path, updated, snap.mode.Perm()); err != nil {
			result.Outcome, result.Configuration, result.Error = OutcomeFailed, ConfigurationInvalid, err.Error()
			return result, nil
		}
		result.Outcome = OutcomeRestoreIncomplete
		result.Configuration = ConfigurationAbsent
		result.Error = "statusLine no longer matched AgentDeck's registered command; removed it without restoring the recorded prior value, check " + path + " manually"
		return result, nil
	}

	result.Outcome = OutcomeRestoreIncomplete
	result.Configuration = ConfigurationModified
	result.Error = "statusLine is no longer AgentDeck's command; left untouched, check " + path + " manually"
	return result, nil
}

// StatusLineStatus reports whether AgentDeck's status-line command is
// currently registered, without changing anything.
func (m *Manager) StatusLineStatus() (Result, error) {
	path := m.path(ClientClaude)
	result := Result{Client: ClientClaude, Path: path, Trust: TrustNotApplicable}

	snap, err := m.readSnapshot(path)
	if err != nil {
		result.Configuration, result.Error = ConfigurationInvalid, err.Error()
		return result, nil
	}
	if !snap.exists {
		result.Configuration = ConfigurationAbsent
		return result, nil
	}
	currentRaw, currentFound, err := topLevelValueSpanRaw(snap.original, statusLineKey)
	if err != nil {
		result.Configuration, result.Error = ConfigurationInvalid, err.Error()
		return result, nil
	}
	if !currentFound {
		result.Configuration = ConfigurationAbsent
		return result, nil
	}
	switch {
	case jsonEquivalent(currentRaw, m.desiredStatusLineEntry()):
		result.Configuration = ConfigurationConfigured
	default:
		if command, ok := decodeStatusLineCommandEntry(currentRaw); ok && managedStatusLineCommand(command) {
			result.Configuration = ConfigurationModified
		} else {
			result.Configuration = ConfigurationAbsent
		}
	}
	return result, nil
}

// PriorStatusLineCommand reads the command AgentDeck should chain to at
// runtime, as recorded by SetupStatusLine. ok is false when there is
// nothing to chain to — no prior command was recorded, the recorded value
// was not a command entry, or the record could not be read — in which case
// the caller runs no subprocess.
func (m *Manager) PriorStatusLineCommand() (command string, ok bool) {
	prior, found, err := m.readStatusLinePrior()
	if err != nil || !found || !prior.Existed {
		return "", false
	}
	return decodeStatusLineCommandEntry(prior.Value)
}

func (m *Manager) statusLinePriorPath() (string, error) {
	if strings.TrimSpace(m.environment.StateDir) == "" {
		return "", errors.New("status-line registration requires a configured state directory")
	}
	return filepath.Join(m.environment.StateDir, statusLinePriorFileName), nil
}

func (m *Manager) readStatusLinePrior() (statusLinePriorRecord, bool, error) {
	path, err := m.statusLinePriorPath()
	if err != nil {
		return statusLinePriorRecord{}, false, err
	}
	contents, err := m.files.readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return statusLinePriorRecord{}, false, nil
	}
	if err != nil {
		return statusLinePriorRecord{}, false, err
	}
	var record statusLinePriorRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		return statusLinePriorRecord{}, false, err
	}
	if record.Raw != "" {
		if !json.Valid([]byte(record.Raw)) {
			return statusLinePriorRecord{}, false, errors.New("recorded prior statusLine is not valid JSON")
		}
		record.Value = json.RawMessage(record.Raw)
	}
	return record, true, nil
}

func (m *Manager) writeStatusLinePrior(record statusLinePriorRecord) error {
	path, err := m.statusLinePriorPath()
	if err != nil {
		return err
	}
	if len(record.Value) > 0 {
		record.Raw = string(record.Value)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return m.writeAtomic(path, encoded, privateFileMode)
}

// topLevelValueSpanRaw wraps topLevelValueSpan to return the located value's
// bytes directly, copied so the result outlives contents.
func topLevelValueSpanRaw(contents []byte, target string) (json.RawMessage, bool, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		return nil, false, nil
	}
	start, end, found, err := topLevelValueSpan(contents, target)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	raw := make(json.RawMessage, end-start)
	copy(raw, contents[start:end])
	return raw, true, nil
}

// setTopLevelValue replaces target's value if present, or inserts it as a
// new top-level key if absent — the same surgical splice encodeDocument
// already performs for "hooks" specifically, generalized to any top-level
// key so statusLine's two keys can reuse it instead of duplicating it.
func setTopLevelValue(contents []byte, target string, value json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		encoded := append([]byte{'{'}, mustMarshalJSONKey(target)...)
		encoded = append(encoded, ':')
		encoded = append(encoded, value...)
		encoded = append(encoded, '}', '\n')
		return encoded, nil
	}
	start, end, found, err := topLevelValueSpan(contents, target)
	if err != nil {
		return nil, err
	}
	if found {
		updated := make([]byte, 0, len(contents)-end+start+len(value))
		updated = append(updated, contents[:start]...)
		updated = append(updated, value...)
		updated = append(updated, contents[end:]...)
		return updated, nil
	}
	order, err := orderedObjectKeys(contents)
	if err != nil {
		return nil, err
	}
	return insertTopLevelValue(contents, target, value, len(order) > 0)
}

func mustMarshalJSONKey(key string) []byte {
	encoded, _ := json.Marshal(key)
	return encoded
}

// removeTopLevelValue returns contents with target's "key":value entry
// deleted entirely — including exactly one adjacent comma, so the result
// stays valid JSON — or contents unchanged if target is absent. This is
// RestoreStatusLine's "there was nothing before AgentDeck" case
// (CLA-R1-F1): the key must disappear, not become a literal null standing
// in for absence.
func removeTopLevelValue(contents []byte, target string) ([]byte, error) {
	entryStart, entryEnd, found, err := topLevelEntrySpan(contents, target)
	if err != nil {
		return nil, err
	}
	if !found {
		return contents, nil
	}
	updated := make([]byte, 0, len(contents)-(entryEnd-entryStart))
	updated = append(updated, contents[:entryStart]...)
	updated = append(updated, contents[entryEnd:]...)
	return updated, nil
}

type jsonTopLevelEntry struct{ keyStart, valEnd int }

// topLevelEntrySpan locates one top-level "key":value pair's exact byte
// span, extended to consume exactly one adjacent comma (the following one
// if any entry comes after it, else the preceding one), so deleting
// contents[entryStart:entryEnd] leaves the remaining document valid JSON.
func topLevelEntrySpan(contents []byte, target string) (entryStart, entryEnd int, found bool, err error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return 0, 0, false, errors.New("hook configuration must be a JSON object")
	}
	var entries []jsonTopLevelEntry
	targetIndex := -1
	for decoder.More() {
		keyOffsetBefore := int(decoder.InputOffset())
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, 0, false, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, 0, false, errors.New("JSON object key must be a string")
		}
		relativeQuote := bytes.IndexByte(contents[keyOffsetBefore:], '"')
		if relativeQuote < 0 {
			return 0, 0, false, errors.New("cannot locate JSON object key")
		}
		keyStart := keyOffsetBefore + relativeQuote

		valOffset := int(decoder.InputOffset())
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return 0, 0, false, err
		}
		value := bytes.TrimSpace(raw)
		relative := bytes.Index(contents[valOffset:], value)
		if relative < 0 {
			return 0, 0, false, errors.New("cannot locate JSON object value")
		}
		valEnd := valOffset + relative + len(value)

		entries = append(entries, jsonTopLevelEntry{keyStart: keyStart, valEnd: valEnd})
		if key == target {
			targetIndex = len(entries) - 1
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
	if targetIndex < 0 {
		return 0, 0, false, nil
	}
	entry := entries[targetIndex]
	entryStart, entryEnd = entry.keyStart, entry.valEnd
	switch {
	case targetIndex < len(entries)-1:
		relComma := bytes.IndexByte(contents[entry.valEnd:entries[targetIndex+1].keyStart], ',')
		if relComma >= 0 {
			entryEnd = entry.valEnd + relComma + 1
		}
	case targetIndex > 0:
		prevEnd := entries[targetIndex-1].valEnd
		relComma := bytes.LastIndexByte(contents[prevEnd:entry.keyStart], ',')
		if relComma >= 0 {
			entryStart = prevEnd + relComma
		}
	}
	return entryStart, entryEnd, true, nil
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
