package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
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

type ClientConfig struct{ Name, Endpoint, Credential string }

var (
	tomlTablePattern         = regexp.MustCompile(`^\s*\[\[?\s*([^]]+?)\s*]]?\s*(?:#.*)?$`)
	tomlCustomFieldPattern   = regexp.MustCompile(`^\s*(base_url|experimental_bearer_token)\s*=`)
	tomlCustomNamePattern    = regexp.MustCompile(`^(\s*name\s*=\s*)(?:"(?:\\.|[^"\\])*"|'[^']*')(\s*(?:#.*)?)$`)
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
	document["model_provider"] = "custom"
	providers["custom"] = map[string]any{"name": config.Name, "base_url": strings.TrimRight(config.Endpoint, "/") + "/v1", "requires_openai_auth": true, "experimental_bearer_token": config.Credential, "wire_api": "responses"}
	encoded, err := toml.Marshal(document)
	if err != nil {
		return err
	}
	return atomicPrivateReplace(path, encoded)
}

// WriteOfficialCodexConfig restores Codex's built-in OpenAI transport while
// leaving authentication entirely under Codex's ownership. It selects the
// custom provider, sets its managed name to official, and removes the two
// AgentDeck-managed custom transport fields.
func WriteOfficialCodexConfig(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid codex toml: %w", err)
	}

	lines := bytes.SplitAfter(contents, []byte("\n"))
	result := make([]byte, 0, len(contents)+32)
	lineEnding := []byte("\n")
	if firstNewline := bytes.IndexByte(contents, '\n'); firstNewline > 0 && contents[firstNewline-1] == '\r' {
		lineEnding = []byte("\r\n")
	}
	table := ""
	modelProviderSeen := false
	customTableSeen := false
	customNameSeen := false
	appendMissingCustomName := func() {
		if table == "model_providers.custom" && !customNameSeen {
			result = appendTOMLLine(result, `name = "official"`, lineEnding)
			customNameSeen = true
		}
	}
	for _, line := range lines {
		body := bytes.TrimSuffix(line, []byte("\n"))
		ending := line[len(body):]
		body = bytes.TrimSuffix(body, []byte("\r"))
		if len(body) != len(line)-len(ending) {
			ending = append([]byte("\r"), ending...)
		}
		trimmed := strings.TrimSpace(string(body))
		if matches := tomlTablePattern.FindStringSubmatch(string(body)); matches != nil {
			appendMissingCustomName()
			table = strings.TrimSpace(matches[1])
			if table == "model_providers.custom" {
				customTableSeen = true
				customNameSeen = false
			}
			result = append(result, line...)
			continue
		}
		if table == "model_providers.custom" && tomlCustomFieldPattern.Match(body) {
			continue
		}
		if table == "model_providers.custom" {
			if matches := tomlCustomNamePattern.FindSubmatch(body); matches != nil {
				customNameSeen = true
				result = append(result, matches[1]...)
				result = append(result, `"official"`...)
				result = append(result, matches[2]...)
				result = append(result, ending...)
				continue
			}
		}
		if table == "" && strings.HasPrefix(trimmed, "model_provider") {
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
	appendMissingCustomName()
	if !customTableSeen {
		result = appendTOMLLine(result, "[model_providers.custom]", lineEnding)
		result = appendTOMLLine(result, `name = "official"`, lineEnding)
	}
	if !modelProviderSeen {
		prefix := append([]byte(`model_provider = "custom"`), lineEnding...)
		result = append(prefix, result...)
	}
	var updated map[string]any
	if err := toml.Unmarshal(result, &updated); err != nil {
		return fmt.Errorf("invalid codex toml after official provider update: %w", err)
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

func WriteClaudeConfig(path string, config ClientConfig) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("invalid claude json: %w", err)
	}
	env, _ := document["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
		document["env"] = env
	}
	env["ANTHROPIC_BASE_URL"] = strings.TrimRight(config.Endpoint, "/")
	env["ANTHROPIC_AUTH_TOKEN"] = config.Credential
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
