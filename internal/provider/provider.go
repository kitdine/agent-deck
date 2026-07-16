// Package provider owns provider definitions and credential references.
package provider

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kitdine/agent-deck/internal/providermeta"
)

type Client string

const (
	ClientCodex  Client = "codex"
	ClientClaude Client = "claude"
)

var (
	ErrInvalidProvider   = errors.New("invalid_provider")
	ErrInvalidMultiplier = errors.New("invalid_multiplier")
)

type Definition struct {
	Name          string
	Endpoint      string
	Clients       []Client
	CredentialRef string
	Multiplier    string
	ModelMappings map[string]string
}

type SelectionSnapshot struct {
	ProviderName string
	Client       Client
	Multiplier   string
}

func Validate(definition Definition) (Definition, error) {
	definition.Name = strings.TrimSpace(definition.Name)
	if definition.Name == "" || definition.CredentialRef == "" || len(definition.Clients) == 0 {
		return Definition{}, fmt.Errorf("%w: name, credential reference, and clients are required", ErrInvalidProvider)
	}
	seen := map[Client]bool{}
	for _, client := range definition.Clients {
		if client != ClientCodex && client != ClientClaude || seen[client] {
			return Definition{}, fmt.Errorf("%w: client", ErrInvalidProvider)
		}
		seen[client] = true
	}
	var err error
	definition.Endpoint, err = NormalizeCredentialEndpoint(definition.Endpoint, seen[ClientCodex])
	if err != nil {
		return Definition{}, err
	}
	multiplier, err := NormalizeMultiplier(definition.Multiplier)
	if err != nil {
		return Definition{}, err
	}
	definition.Multiplier = multiplier
	return definition, nil
}

func NormalizeCredentialEndpoint(value string, codex bool) (string, error) {
	normalized, err := providermeta.NormalizeEndpoint(value, codex)
	if err != nil {
		return "", fmt.Errorf("%w: endpoint", ErrInvalidProvider)
	}
	return normalized, nil
}

func NormalizeMultiplier(value string) (string, error) {
	normalized, err := providermeta.NormalizeMultiplier(value)
	if err != nil {
		return "", ErrInvalidMultiplier
	}
	return normalized, nil
}
