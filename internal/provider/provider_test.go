package provider

import (
	"errors"
	"testing"
)

func TestValidateProvider(t *testing.T) {
	valid := Definition{Name: "example", Endpoint: "https://provider.example/v1", Clients: []Client{ClientCodex}, CredentialRef: "example-default-ref", Multiplier: "1.25"}
	got, err := Validate(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Multiplier != "1.250000000000" {
		t.Fatalf("multiplier = %q", got.Multiplier)
	}
	if got.Endpoint != "https://provider.example" {
		t.Fatalf("endpoint = %q", got.Endpoint)
	}
}

func TestValidateNormalizesCredentialEndpointByClient(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		clients  []Client
		want     string
	}{
		{name: "codex base", endpoint: "https://provider.example/api", clients: []Client{ClientCodex}, want: "https://provider.example/api"},
		{name: "codex v1", endpoint: "https://provider.example/api/v1/", clients: []Client{ClientCodex}, want: "https://provider.example/api"},
		{name: "shared v1", endpoint: "https://provider.example/v1", clients: []Client{ClientClaude, ClientCodex}, want: "https://provider.example"},
		{name: "claude v1", endpoint: "https://provider.example/api/v1/", clients: []Client{ClientClaude}, want: "https://provider.example/api/v1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Validate(Definition{Name: "example", Endpoint: test.endpoint, Clients: test.clients, CredentialRef: "example-default-ref"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Endpoint != test.want {
				t.Fatalf("endpoint = %q, want %q", got.Endpoint, test.want)
			}
		})
	}
}

func TestCredentialReferenceAlwaysIncludesProviderAndShortName(t *testing.T) {
	for _, test := range []struct {
		provider   string
		credential string
		want       string
	}{
		{provider: "Work", credential: "", want: "work-default-ref"},
		{provider: "sssaicode", credential: "codex", want: "sssaicode-codex-ref"},
	} {
		got, err := CredentialReference(test.provider, test.credential)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("CredentialReference(%q, %q) = %q, want %q", test.provider, test.credential, got, test.want)
		}
	}
}

func TestValidateRejectsUnsafeDefinitions(t *testing.T) {
	cases := []Definition{
		{Name: "", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "file:///tmp/key", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "https://token@provider.example/v1", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "https://provider.example/v1?token=secret", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "https://provider.example/v1#fragment", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "https://provider.example", Clients: []Client{Client("other")}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex, ClientCodex}, CredentialRef: "ref"},
	}
	for _, definition := range cases {
		if _, err := Validate(definition); !errors.Is(err, ErrInvalidProvider) {
			t.Fatalf("Validate(%+v) error = %v", definition, err)
		}
	}
}

func TestNormalizeMultiplier(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "true", "not-a-number"} {
		if _, err := NormalizeMultiplier(value); !errors.Is(err, ErrInvalidMultiplier) {
			t.Fatalf("NormalizeMultiplier(%q) error = %v", value, err)
		}
	}
}
