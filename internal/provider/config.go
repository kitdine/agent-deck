package provider

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

type ClientConfig struct{ Name, Endpoint, Credential string }

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
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	return os.Chmod(path, info.Mode().Perm())
}
