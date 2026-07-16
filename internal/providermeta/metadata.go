// Package providermeta owns canonical provider credential metadata rules that
// must remain identical in runtime validation and database migrations.
package providermeta

import (
	"errors"
	"math/big"
	"net/url"
	"strings"
)

var (
	ErrInvalidEndpoint       = errors.New("invalid endpoint")
	ErrInvalidCredentialName = errors.New("invalid credential name")
	ErrInvalidProviderName   = errors.New("invalid provider name")
	ErrInvalidMultiplier     = errors.New("invalid multiplier")
)

func NormalizeCredentialName(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = "default"
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", ErrInvalidCredentialName
		}
	}
	return value, nil
}

func CredentialReference(providerName, credentialName string) (string, error) {
	providerName = strings.TrimSpace(strings.ToLower(providerName))
	if providerName == "" {
		return "", ErrInvalidProviderName
	}
	name, err := NormalizeCredentialName(credentialName)
	if err != nil {
		return "", err
	}
	return providerName + "-" + name + "-ref", nil
}

func NormalizeEndpoint(value string, codex bool) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidEndpoint
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	if codex && (parsed.Path == "/v1" || strings.HasSuffix(parsed.Path, "/v1")) {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/v1")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
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
