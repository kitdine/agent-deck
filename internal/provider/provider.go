// Package provider owns provider definitions and credential references.
package provider

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
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
	parsed, err := url.ParseRequestURI(definition.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Definition{}, fmt.Errorf("%w: endpoint", ErrInvalidProvider)
	}
	seen := map[Client]bool{}
	for _, client := range definition.Clients {
		if client != ClientCodex && client != ClientClaude || seen[client] {
			return Definition{}, fmt.Errorf("%w: client", ErrInvalidProvider)
		}
		seen[client] = true
	}
	multiplier, err := NormalizeMultiplier(definition.Multiplier)
	if err != nil {
		return Definition{}, err
	}
	definition.Multiplier = multiplier
	return definition, nil
}

func NormalizeMultiplier(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "1", nil
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok || rational.Sign() < 0 {
		return "", ErrInvalidMultiplier
	}
	return rational.FloatString(12), nil
}
