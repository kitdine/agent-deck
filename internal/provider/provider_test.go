package provider

import (
	"errors"
	"testing"
)

func TestValidateProvider(t *testing.T) {
	valid := Definition{Name: "example", Endpoint: "https://provider.example/v1", Clients: []Client{ClientCodex}, CredentialRef: "keychain:example", Multiplier: "1.25"}
	got, err := Validate(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Multiplier != "1.250000000000" {
		t.Fatalf("multiplier = %q", got.Multiplier)
	}
}

func TestValidateRejectsUnsafeDefinitions(t *testing.T) {
	cases := []Definition{
		{Name: "", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
		{Name: "example", Endpoint: "file:///tmp/key", Clients: []Client{ClientCodex}, CredentialRef: "ref"},
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
