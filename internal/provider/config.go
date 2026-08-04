package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/pelletier/go-toml/v2"
)

func ConfigFingerprint(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(contents)
	return fmt.Sprintf("%x", digest), nil
}

// ConfigMatchesEndpoint checks only the AgentDeck-owned endpoint selection and
// never returns native configuration contents.
func ConfigMatchesEndpoint(client Client, path, endpoint string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	expected := strings.TrimRight(endpoint, "/")
	if client == ClientCodex {
		var document map[string]any
		if err = toml.Unmarshal(contents, &document); err != nil {
			return false, err
		}
		if document["model_provider"] != "custom" {
			return false, nil
		}
		providers, _ := document["model_providers"].(map[string]any)
		custom, _ := providers["custom"].(map[string]any)
		baseURL, _ := custom["base_url"].(string)
		return strings.TrimRight(baseURL, "/") == expected+"/v1", nil
	}
	if client == ClientClaude {
		var document map[string]any
		if err = json.Unmarshal(contents, &document); err != nil {
			return false, err
		}
		environment, _ := document["env"].(map[string]any)
		baseURL, _ := environment["ANTHROPIC_BASE_URL"].(string)
		return strings.TrimRight(baseURL, "/") == expected, nil
	}
	return false, fmt.Errorf("unsupported client %q", client)
}

func ConfigMatchesOfficialCodex(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return false, err
	}
	if document["model_provider"] != "custom" {
		return false, nil
	}
	providers, _ := document["model_providers"].(map[string]any)
	custom, _ := providers["custom"].(map[string]any)
	name, _ := custom["name"].(string)
	_, hasBaseURL := custom["base_url"]
	_, hasBearerToken := custom["experimental_bearer_token"]
	return name == OfficialProviderName && !hasBaseURL && !hasBearerToken, nil
}

// ConfigMatchesOfficialClaude is the Claude counterpart of
// ConfigMatchesOfficialCodex: it reports whether the settings carry neither
// owned transport field, which is exactly what a direct built-in-provider
// selection writes for Claude. An absent env object and an env object holding
// only unowned keys both match, because AgentDeck owns no endpoint and no
// credential under that selection.
func ConfigMatchesOfficialClaude(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return false, err
	}
	environment, _ := document["env"].(map[string]any)
	_, hasBaseURL := environment["ANTHROPIC_BASE_URL"]
	_, hasToken := environment["ANTHROPIC_AUTH_TOKEN"]
	return !hasBaseURL && !hasToken, nil
}

// ConfigMatchesOfficialWrapper reports whether the client configuration carries
// the built-in provider's wrapper endpoint and no credential, which is what a
// --via built-in-provider selection writes. It is separate from the two direct
// official matchers rather than folded into them through a possibly-empty
// endpoint argument, because "no endpoint at all" and "exactly this wrapper
// endpoint" are two different states to prove, not one comparison.
func ConfigMatchesOfficialWrapper(client Client, path, endpoint string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	expected := strings.TrimRight(endpoint, "/")
	if client == ClientCodex {
		var document map[string]any
		if err := toml.Unmarshal(contents, &document); err != nil {
			return false, err
		}
		if document["model_provider"] != "custom" {
			return false, nil
		}
		providers, _ := document["model_providers"].(map[string]any)
		custom, _ := providers["custom"].(map[string]any)
		name, _ := custom["name"].(string)
		baseURL, _ := custom["base_url"].(string)
		_, hasBearerToken := custom["experimental_bearer_token"]
		return name == OfficialProviderName && strings.TrimRight(baseURL, "/") == expected+"/v1" && !hasBearerToken, nil
	}
	if client == ClientClaude {
		var document map[string]any
		if err := json.Unmarshal(contents, &document); err != nil {
			return false, err
		}
		environment, _ := document["env"].(map[string]any)
		baseURL, _ := environment["ANTHROPIC_BASE_URL"].(string)
		_, hasToken := environment["ANTHROPIC_AUTH_TOKEN"]
		return strings.TrimRight(baseURL, "/") == expected && !hasToken, nil
	}
	return false, fmt.Errorf("unsupported client %q", client)
}

// ClaudeConfigMatchesSnapshot checks only the AgentDeck-owned Claude route
// fields for a completed provider selection. It never returns config content.
func ClaudeConfigMatchesSnapshot(path string, snapshot store.ProviderSnapshot) (bool, error) {
	if snapshot.Name == OfficialProviderName {
		if snapshot.ViaWrapper {
			return ConfigMatchesOfficialWrapper(ClientClaude, path, snapshot.Endpoint)
		}
		return ConfigMatchesOfficialClaude(path)
	}
	return ConfigMatchesEndpoint(ClientClaude, path, snapshot.Endpoint)
}

// ClaudeConflictAPIKey and ClaudeConflictAPIKeyHelper name the two credential
// sources Claude honors that AgentDeck never writes, clears, or reorders. Both
// override a built-in-provider selection, so a switch to official reports them
// instead of removing a field it does not own.
const (
	ClaudeConflictAPIKey       = "env.ANTHROPIC_API_KEY"
	ClaudeConflictAPIKeyHelper = "apiKeyHelper"
)

// ClaudeCredentialConflicts names the unowned credential sources present in a
// Claude settings file, in a stable order. It returns key names only and never
// a value, because one of the two keys holds a credential. A key that
// configures no usable credential is not reported, so the advisory never fires
// on something that overrides nothing.
//
// Detection is scoped to this file. A credential exported into the shell
// environment is also honored by Claude and is not visible here; see the
// scope decisions recorded with the switch-advisories task.
func ClaudeCredentialConflicts(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	conflicts := make([]string, 0, 2)
	environment, _ := document["env"].(map[string]any)
	if configuresCredential(environment["ANTHROPIC_API_KEY"]) {
		conflicts = append(conflicts, ClaudeConflictAPIKey)
	}
	if configuresCredential(document["apiKeyHelper"]) {
		conflicts = append(conflicts, ClaudeConflictAPIKeyHelper)
	}
	return conflicts, nil
}

// configuresCredential reports whether a settings value can actually supply a
// credential to Claude. Both keys are string-valued to Claude — an env value
// and a helper command line — so exactly one shape configures anything: a
// non-empty string. Every other shape (null, an empty string, a bool, a
// number, an object, an array) either configures nothing or is malformed for
// the key it sits on, and Claude cannot derive a credential from it, so
// reporting it would train users to ignore the advisory.
//
// A blank-but-not-empty string such as " " is reported. It is a non-empty
// value to Claude, which will use it and fail to authenticate — precisely the
// confusing state the advisory exists to explain — so it is deliberately not
// trimmed away.
func configuresCredential(value any) bool {
	text, ok := value.(string)
	return ok && text != ""
}

type ClientConfig struct {
	Name, Endpoint, Credential string
	ProjectAttribution         bool
}

var (
	tomlTablePattern         = regexp.MustCompile(`^\s*\[\[?\s*([^]]+?)\s*]]?\s*(?:#.*)?$`)
	tomlCustomFieldPattern   = regexp.MustCompile(`^\s*(base_url|experimental_bearer_token)\s*=`)
	tomlCustomBearerPattern  = regexp.MustCompile(`^\s*experimental_bearer_token\s*=`)
	tomlCustomNamePattern    = regexp.MustCompile(`^(\s*name\s*=\s*)(?:"(?:\\.|[^"\\])*"|'[^']*')(\s*(?:#.*)?)$`)
	tomlCustomBaseURLPattern = regexp.MustCompile(`^(\s*base_url\s*=\s*)(?:"(?:\\.|[^"\\])*"|'[^']*')(\s*(?:#.*)?)$`)
	tomlModelProviderPattern = regexp.MustCompile(`^(\s*model_provider\s*=\s*)([^#\r\n]*?)(\s*(?:#.*)?)$`)
	replaceFile              = os.Rename
)

func WriteCodexConfig(path string, config ClientConfig) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid codex toml: %w", err)
	}
	providers, _ := document["model_providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
		document["model_providers"] = providers
	}
	headers, err := codexEnvironmentHeaders(document)
	if err != nil {
		return fmt.Errorf("invalid codex env_http_headers: %w", err)
	}
	document["model_provider"] = "custom"
	custom := map[string]any{"name": config.Name, "base_url": strings.TrimRight(config.Endpoint, "/") + "/v1", "requires_openai_auth": true, "experimental_bearer_token": config.Credential, "wire_api": "responses"}
	if len(headers) > 0 {
		custom["env_http_headers"] = headers
	}
	providers["custom"] = custom
	encoded, err := toml.Marshal(document)
	if err != nil {
		return err
	}
	encoded, err = rewriteCodexProjectHeaders(encoded, config.ProjectAttribution)
	if err != nil {
		return fmt.Errorf("rewrite codex project headers: %w", err)
	}
	var updated map[string]any
	if err := toml.Unmarshal(encoded, &updated); err != nil {
		return fmt.Errorf("invalid codex toml after config update: %w", err)
	}
	return atomicPrivateReplace(path, encoded)
}

// rewriteCodexCustomTable performs the line-by-line rewrite shared by every
// Codex writer that must select [model_providers.custom] while preserving
// every unrelated TOML field, comment, and ordering. onLine may rewrite or
// drop a line inside the custom table; returning handled=false leaves the
// line untouched. onEnter runs once per occurrence of the custom table
// (including an array-of-tables source with more than one occurrence),
// before any of its lines are processed, so the caller can reset the
// per-occurrence owed-field state flush and onLine close over. flush
// appends any custom-table fields the caller still owes once that
// occurrence ends, on leaving it for another table or at end of file; it
// does not run for a table that was never present, which is what
// ensureTable is for — it supplies the lines that create the table when
// the source never had one.
func rewriteCodexCustomTable(
	contents []byte,
	onEnter func(),
	onLine func(body []byte) (rewritten []byte, handled bool),
	flush func(result []byte, lineEnding []byte) []byte,
	ensureTable func(result []byte, lineEnding []byte) []byte,
) []byte {
	lines := bytes.SplitAfter(contents, []byte("\n"))
	result := make([]byte, 0, len(contents)+32)
	lineEnding := []byte("\n")
	if firstNewline := bytes.IndexByte(contents, '\n'); firstNewline > 0 && contents[firstNewline-1] == '\r' {
		lineEnding = []byte("\r\n")
	}
	inTable := false
	inCustomTable := false
	modelProviderSeen := false
	customTableSeen := false
	for _, line := range lines {
		body := bytes.TrimSuffix(line, []byte("\n"))
		ending := line[len(body):]
		body = bytes.TrimSuffix(body, []byte("\r"))
		if len(body) != len(line)-len(ending) {
			ending = append([]byte("\r"), ending...)
		}
		trimmed := strings.TrimSpace(string(body))
		if matches := tomlTablePattern.FindStringSubmatch(string(body)); matches != nil {
			if inCustomTable {
				result = flush(result, lineEnding)
			}
			inTable = true
			inCustomTable = tomlTableMatches(body, "model_providers.custom")
			if inCustomTable {
				customTableSeen = true
				onEnter()
			}
			result = append(result, line...)
			continue
		}
		if inCustomTable {
			if rewritten, handled := onLine(body); handled {
				if rewritten != nil {
					result = append(result, rewritten...)
					result = append(result, ending...)
				}
				continue
			}
		}
		if !inTable && strings.HasPrefix(trimmed, "model_provider") {
			matches := tomlModelProviderPattern.FindSubmatch(body)
			if matches != nil {
				modelProviderSeen = true
				var current map[string]any
				if err := toml.Unmarshal(body, &current); err == nil && current["model_provider"] == "custom" {
					result = append(result, line...)
				} else {
					result = append(result, matches[1]...)
					result = append(result, `"custom"`...)
					result = append(result, matches[3]...)
					result = append(result, ending...)
				}
				continue
			}
		}
		result = append(result, line...)
	}
	if inCustomTable {
		result = flush(result, lineEnding)
	}
	if !customTableSeen {
		result = ensureTable(result, lineEnding)
	}
	if !modelProviderSeen {
		prefix := append([]byte(`model_provider = "custom"`), lineEnding...)
		result = append(prefix, result...)
	}
	return result
}

func rewriteCodexProjectHeaders(contents []byte, enabled bool) ([]byte, error) {
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	headers, err := codexEnvironmentHeaders(document)
	if err != nil {
		return nil, err
	}
	_, projectFound := headers[HeadroomProjectHeader]
	if !enabled && !projectFound {
		return contents, nil
	}
	if enabled {
		headers[HeadroomProjectHeader] = HeadroomProjectEnvironment
	} else {
		delete(headers, HeadroomProjectHeader)
	}

	contents = dropTOMLTable(contents, "model_providers.custom.env_http_headers")
	mapping := codexEnvironmentHeadersTOML(headers)
	onEnter := func() {}
	onLine := func(body []byte) ([]byte, bool) {
		if tomlKeyMatches(body, "env_http_headers") {
			return nil, true
		}
		return nil, false
	}
	flush := func(result []byte, lineEnding []byte) []byte {
		if mapping != "" {
			result = appendTOMLLine(result, mapping, lineEnding)
		}
		return result
	}
	ensureTable := func(result []byte, lineEnding []byte) []byte {
		result = appendTOMLLine(result, "[model_providers.custom]", lineEnding)
		return flush(result, lineEnding)
	}
	return rewriteCodexCustomTable(contents, onEnter, onLine, flush, ensureTable), nil
}

func codexEnvironmentHeaders(document map[string]any) (map[string]string, error) {
	result := map[string]string{}
	providers, _ := document["model_providers"].(map[string]any)
	custom, _ := providers["custom"].(map[string]any)
	raw, found := custom["env_http_headers"]
	if !found {
		return result, nil
	}
	headers, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected table, got %T", raw)
	}
	for name, value := range headers {
		environment, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected string, got %T", name, value)
		}
		result[name] = environment
	}
	return result, nil
}

func codexEnvironmentHeadersTOML(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	mappings := make([]string, 0, len(names))
	for _, name := range names {
		mappings = append(mappings, strconv.Quote(name)+" = "+strconv.Quote(headers[name]))
	}
	return "env_http_headers = { " + strings.Join(mappings, ", ") + " }"
}

func dropTOMLTable(contents []byte, name string) []byte {
	lines := bytes.SplitAfter(contents, []byte("\n"))
	result := make([]byte, 0, len(contents))
	dropping := false
	for _, line := range lines {
		body := bytes.TrimSuffix(line, []byte("\n"))
		body = bytes.TrimSuffix(body, []byte("\r"))
		if matches := tomlTablePattern.FindStringSubmatch(string(body)); matches != nil {
			dropping = tomlTableMatches(body, name)
			if dropping {
				continue
			}
		}
		if !dropping {
			result = append(result, line...)
		}
	}
	return result
}

func tomlTableMatches(header []byte, name string) bool {
	const probe = "__agentdeck_table_probe__"
	candidate := append(append([]byte{}, header...), '\n')
	candidate = append(candidate, probe+" = true\n"...)
	var document map[string]any
	if err := toml.Unmarshal(candidate, &document); err != nil {
		return false
	}
	var current any = document
	for _, part := range strings.Split(name, ".") {
		table, ok := tomlLastTable(current)
		if !ok {
			return false
		}
		current, ok = table[part]
		if !ok {
			return false
		}
	}
	table, ok := tomlLastTable(current)
	value, found := table[probe]
	return ok && found && value == true
}

func tomlLastTable(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case []map[string]any:
		if len(typed) > 0 {
			return typed[len(typed)-1], true
		}
	case []any:
		if len(typed) > 0 {
			table, ok := typed[len(typed)-1].(map[string]any)
			return table, ok
		}
	}
	return nil, false
}

func tomlKeyMatches(line []byte, name string) bool {
	var document map[string]any
	if err := toml.Unmarshal(line, &document); err != nil || len(document) != 1 {
		return false
	}
	_, found := document[name]
	return found
}

// WriteOfficialCodexConfig restores Codex's built-in OpenAI transport while
// leaving authentication entirely under Codex's ownership. It selects the
// custom provider, sets its managed name to official, and removes the
// AgentDeck-managed custom transport and project-header fields.
func WriteOfficialCodexConfig(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid codex toml: %w", err)
	}

	nameSeen := false
	onEnter := func() { nameSeen = false }
	onLine := func(body []byte) (rewritten []byte, handled bool) {
		if tomlCustomFieldPattern.Match(body) {
			return nil, true
		}
		if matches := tomlCustomNamePattern.FindSubmatch(body); matches != nil {
			nameSeen = true
			return append(append(append([]byte{}, matches[1]...), `"official"`...), matches[2]...), true
		}
		return nil, false
	}
	flush := func(result []byte, lineEnding []byte) []byte {
		if !nameSeen {
			result = appendTOMLLine(result, `name = "official"`, lineEnding)
			nameSeen = true
		}
		return result
	}
	ensureTable := func(result []byte, lineEnding []byte) []byte {
		result = appendTOMLLine(result, "[model_providers.custom]", lineEnding)
		return appendTOMLLine(result, `name = "official"`, lineEnding)
	}
	result := rewriteCodexCustomTable(contents, onEnter, onLine, flush, ensureTable)
	result, err = rewriteCodexProjectHeaders(result, false)
	if err != nil {
		return fmt.Errorf("rewrite codex project headers: %w", err)
	}

	var updated map[string]any
	if err := toml.Unmarshal(result, &updated); err != nil {
		return fmt.Errorf("invalid codex toml after official provider update: %w", err)
	}
	return atomicPrivateReplace(path, result)
}

// WriteCodexWrapperConfig routes Codex through a wrapper URL without a
// credential: it writes base_url, removes experimental_bearer_token, keeps
// requires_openai_auth and wire_api untouched, manages the project-header
// mapping according to projectAttribution, and leaves every other TOML field,
// comment, and ordering unchanged.
func WriteCodexWrapperConfig(path, name, endpoint string, projectAttribution bool) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid codex toml: %w", err)
	}

	baseURL := strings.TrimRight(endpoint, "/") + "/v1"
	quotedName := fmt.Sprintf("%q", name)
	quotedBaseURL := fmt.Sprintf("%q", baseURL)
	nameSeen := false
	baseURLSeen := false
	onEnter := func() { nameSeen = false; baseURLSeen = false }
	onLine := func(body []byte) (rewritten []byte, handled bool) {
		if tomlCustomBearerPattern.Match(body) {
			return nil, true
		}
		if matches := tomlCustomNamePattern.FindSubmatch(body); matches != nil {
			nameSeen = true
			return append(append(append([]byte{}, matches[1]...), quotedName...), matches[2]...), true
		}
		if matches := tomlCustomBaseURLPattern.FindSubmatch(body); matches != nil {
			baseURLSeen = true
			return append(append(append([]byte{}, matches[1]...), quotedBaseURL...), matches[2]...), true
		}
		return nil, false
	}
	flush := func(result []byte, lineEnding []byte) []byte {
		if !nameSeen {
			result = appendTOMLLine(result, "name = "+quotedName, lineEnding)
			nameSeen = true
		}
		if !baseURLSeen {
			result = appendTOMLLine(result, "base_url = "+quotedBaseURL, lineEnding)
			baseURLSeen = true
		}
		return result
	}
	ensureTable := func(result []byte, lineEnding []byte) []byte {
		result = appendTOMLLine(result, "[model_providers.custom]", lineEnding)
		result = appendTOMLLine(result, "name = "+quotedName, lineEnding)
		return appendTOMLLine(result, "base_url = "+quotedBaseURL, lineEnding)
	}
	result := rewriteCodexCustomTable(contents, onEnter, onLine, flush, ensureTable)
	result, err = rewriteCodexProjectHeaders(result, projectAttribution)
	if err != nil {
		return fmt.Errorf("rewrite codex project headers: %w", err)
	}

	var updated map[string]any
	if err := toml.Unmarshal(result, &updated); err != nil {
		return fmt.Errorf("invalid codex toml after wrapper config update: %w", err)
	}
	return atomicPrivateReplace(path, result)
}

func appendTOMLLine(contents []byte, line string, ending []byte) []byte {
	if len(contents) > 0 && !bytes.HasSuffix(contents, []byte("\n")) {
		contents = append(contents, ending...)
	}
	contents = append(contents, line...)
	return append(contents, ending...)
}

// WriteClaudeConfig expresses all three intents AgentDeck may write for
// Claude's two owned env keys through one empty-string sentinel: a non-empty
// config.Endpoint/config.Credential sets the corresponding key, an empty one
// removes it. This lets one call site express "endpoint and credential",
// "endpoint without credential" (a wrapper-routed official selection), and
// "neither field" (a direct official selection) without three functions.
// It only creates the env object when an owned key is actually being
// written; a source with no env key and the neither-field intent is left
// byte-for-byte unowned apart from re-serialization, and a source whose env
// key holds something other than a JSON object is left untouched unless an
// owned key must be written into it, matching the pre-existing overwrite
// behavior for that write case.
func WriteClaudeConfig(path string, config ClientConfig) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid claude json: %w", err)
	}
	env, isMap := document["env"].(map[string]any)
	writesOwnedKey := config.Endpoint != "" || config.Credential != ""
	if isMap || writesOwnedKey {
		if !isMap {
			env = map[string]any{}
			document["env"] = env
		}
		if config.Endpoint == "" {
			delete(env, "ANTHROPIC_BASE_URL")
		} else {
			env["ANTHROPIC_BASE_URL"] = strings.TrimRight(config.Endpoint, "/")
		}
		if config.Credential == "" {
			delete(env, "ANTHROPIC_AUTH_TOKEN")
		} else {
			env["ANTHROPIC_AUTH_TOKEN"] = config.Credential
		}
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return atomicPrivateReplace(path, encoded)
}

func WriteRedactedBackup(client Client, source, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if client == ClientCodex {
		var document map[string]any
		if err := toml.Unmarshal(contents, &document); err != nil {
			return err
		}
		if providers, ok := document["model_providers"].(map[string]any); ok {
			if custom, ok := providers["custom"].(map[string]any); ok {
				delete(custom, "experimental_bearer_token")
			}
		}
		contents, err = toml.Marshal(document)
		if err != nil {
			return err
		}
	} else if client == ClientClaude {
		var document map[string]any
		if err := json.Unmarshal(contents, &document); err != nil {
			return err
		}
		if env, ok := document["env"].(map[string]any); ok {
			delete(env, "ANTHROPIC_AUTH_TOKEN")
		}
		contents, err = json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		contents = append(contents, '\n')
	} else {
		return fmt.Errorf("unsupported client %q", client)
	}
	if err := os.MkdirAll(filepath.Dir(destination), platform.DirectoryMode); err != nil {
		return err
	}
	if err := os.WriteFile(destination, contents, platform.FileMode); err != nil {
		return err
	}
	return os.Chmod(destination, platform.FileMode)
}

func atomicPrivateReplace(path string, contents []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".agentdeck-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(platform.FileMode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
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
	if err := replaceFile(temporaryName, path); err != nil {
		return err
	}
	return os.Chmod(path, info.Mode().Perm())
}
