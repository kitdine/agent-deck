package providermeta

import (
	"errors"
	"testing"
)

func TestCanonicalCredentialNamesAndReferences(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "trims and lowercases", input: "  Team_Key-2  ", want: "team_key-2"},
		{name: "blank selects default", input: " \t\n ", want: "default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeCredentialName(test.input)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeCredentialName(%q) = %q, %v; want %q, nil", test.input, got, err, test.want)
			}
		})
	}

	for _, test := range []struct {
		provider   string
		credential string
		want       string
	}{
		{provider: "  Work-Relay  ", credential: "  TEAM_A  ", want: "work-relay-team_a-ref"},
		{provider: "Work-Relay", credential: "", want: "work-relay-default-ref"},
	} {
		got, err := CredentialReference(test.provider, test.credential)
		if err != nil || got != test.want {
			t.Fatalf("CredentialReference(%q, %q) = %q, %v; want %q, nil", test.provider, test.credential, got, err, test.want)
		}
	}
}

func TestCanonicalMetadataRejectsInvalidNamesWithSentinels(t *testing.T) {
	if _, err := NormalizeCredentialName("contains space"); !errors.Is(err, ErrInvalidCredentialName) {
		t.Fatalf("invalid credential error = %v, want ErrInvalidCredentialName", err)
	}
	if _, err := CredentialReference(" \t", "default"); !errors.Is(err, ErrInvalidProviderName) {
		t.Fatalf("blank provider error = %v, want ErrInvalidProviderName", err)
	}
	if _, err := CredentialReference("relay", "invalid/name"); !errors.Is(err, ErrInvalidCredentialName) {
		t.Fatalf("invalid credential reference error = %v, want ErrInvalidCredentialName", err)
	}
}

func TestNormalizeEndpointPreservesSafeBaseAndOnlyStripsCodexV1(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		codex bool
		want  string
	}{
		{name: "codex strips trailing v1", input: " https://provider.example/api/v1/// ", codex: true, want: "https://provider.example/api"},
		{name: "codex strips root v1", input: "https://provider.example/v1", codex: true, want: "https://provider.example"},
		{name: "non codex retains v1", input: "https://provider.example/api/v1/", want: "https://provider.example/api/v1"},
		{name: "safe base is otherwise canonical", input: "https://provider.example/api///", codex: true, want: "https://provider.example/api"},
		{name: "codex retains v10 near miss", input: "https://provider.example/api/v10", codex: true, want: "https://provider.example/api/v10"},
		{name: "codex retains v1beta near miss", input: "https://provider.example/api/v1beta", codex: true, want: "https://provider.example/api/v1beta"},
		{name: "codex retains non-final v1", input: "https://provider.example/api/v1/models", codex: true, want: "https://provider.example/api/v1/models"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeEndpoint(test.input, test.codex)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeEndpoint(%q, %t) = %q, %v; want %q, nil", test.input, test.codex, got, err, test.want)
			}
		})
	}

	for _, input := range []string{
		"file:///tmp/provider",
		"https://token@provider.example/v1",
		"https://provider.example/v1?key=synthetic",
		"https://provider.example/v1#fragment",
		"https:///missing-host",
		"https://provider.example/%zz",
	} {
		if _, err := NormalizeEndpoint(input, true); !errors.Is(err, ErrInvalidEndpoint) {
			t.Fatalf("NormalizeEndpoint(%q) error = %v, want ErrInvalidEndpoint", input, err)
		}
	}
}

func TestNormalizeMultiplierCanonicalBoundaries(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "blank defaults", input: "", want: "1"},
		{name: "whitespace blank defaults", input: " \t", want: "1"},
		{name: "zero has 12 decimal places", input: "0", want: "0.000000000000"},
		{name: "ordinary decimal has 12 decimal places", input: "1.25", want: "1.250000000000"},
		{name: "rounds to 12 places", input: "1.2345678901239", want: "1.234567890124"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeMultiplier(test.input)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeMultiplier(%q) = %q, %v; want %q, nil", test.input, got, err, test.want)
			}
		})
	}
}

func TestNormalizeMultiplierRejectsNonDecimalSyntax(t *testing.T) {
	for _, input := range []string{
		"-0.1",
		"+0.1",
		"1/3",
		"1e3",
		"0x10",
		" 1.25",
		"1.25 ",
		".5",
		"1.",
		"not-a-number",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := NormalizeMultiplier(input); !errors.Is(err, ErrInvalidMultiplier) {
				t.Fatalf("NormalizeMultiplier(%q) error = %v, want ErrInvalidMultiplier", input, err)
			}
		})
	}
}
