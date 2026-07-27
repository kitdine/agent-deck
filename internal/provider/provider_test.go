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

// wrapper-schema: a wrapper URL must normalize identically to a Codex-bound
// credential endpoint, since one wrapper instance serves both client
// protocols from the same stored base and reuses that normalization rather
// than a second implementation.
func TestNormalizeWrapperURLReusesCodexAwareCredentialEndpointNormalization(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "base", url: "https://proxy.example"},
		{name: "trailing slash", url: "https://proxy.example/"},
		{name: "path with v1", url: "https://proxy.example/api/v1/"},
		{name: "bare v1", url: "https://proxy.example/v1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper, err := NormalizeWrapperURL(test.url)
			if err != nil {
				t.Fatal(err)
			}
			credentialEndpoint, err := NormalizeCredentialEndpoint(test.url, true)
			if err != nil {
				t.Fatal(err)
			}
			if wrapper != credentialEndpoint {
				t.Fatalf("NormalizeWrapperURL(%q) = %q, want match with Codex-bound credential endpoint %q", test.url, wrapper, credentialEndpoint)
			}
		})
	}
}

// review fix: wrapper normalization is always the Codex-bound form, never a
// generic "same as credential endpoints" rule — on a /v1-suffixed input it
// must differ from the codex=false (Claude-only) credential endpoint form,
// which preserves that trailing /v1 instead of stripping it.
func TestNormalizeWrapperURLDiffersFromClaudeOnlyCredentialEndpointOnV1Input(t *testing.T) {
	url := "https://proxy.example/api/v1/"
	wrapper, err := NormalizeWrapperURL(url)
	if err != nil {
		t.Fatal(err)
	}
	claudeOnlyEndpoint, err := NormalizeCredentialEndpoint(url, false)
	if err != nil {
		t.Fatal(err)
	}
	if wrapper == claudeOnlyEndpoint {
		t.Fatalf("NormalizeWrapperURL(%q) = %q, want it to differ from the Claude-only (codex=false) credential endpoint form %q", url, wrapper, claudeOnlyEndpoint)
	}
	if wrapper != "https://proxy.example/api" || claudeOnlyEndpoint != "https://proxy.example/api/v1" {
		t.Fatalf("wrapper = %q, claude-only endpoint = %q", wrapper, claudeOnlyEndpoint)
	}
}

func TestNormalizeWrapperURLRejectsInvalidEndpoints(t *testing.T) {
	for _, url := range []string{"", "not-a-url", "https://token@proxy.example", "https://proxy.example?x=1", "https://proxy.example#frag"} {
		if _, err := NormalizeWrapperURL(url); !errors.Is(err, ErrInvalidProvider) {
			t.Fatalf("NormalizeWrapperURL(%q) error = %v, want ErrInvalidProvider", url, err)
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
