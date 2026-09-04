package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/pelletier/go-toml/v2"
)

func TestWriteOfficialCodexConfigPreservesUnmanagedTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"Keep Name\" # keep field comment\nbase_url = \"https://provider.example/v1\"\nexperimental_bearer_token = \"synthetic-secret\"\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	want := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"official\" # keep field comment\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != want {
		t.Fatalf("official config:\n%s\nwant:\n%s", contents, want)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != want {
		t.Fatalf("idempotent official config = %q, %v", again, err)
	}
}

func TestWriteOfficialCodexConfigSetsCustomSelector(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"other\" # preserved comment\n[model_providers.custom]\nbase_url = \"https://provider.example/v1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "model_provider = \"custom\" # preserved comment\n[model_providers.custom]\nname = \"official\"\n" {
		t.Fatalf("official selector config = %q, %v", contents, err)
	}
}

func TestWriteOfficialCodexConfigCreatesCustomTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model = 'keep'\n[features]\nmemories = true\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, custom := readCodexCustomProvider(t, path)
	if custom["name"] != OfficialProviderName {
		t.Fatalf("official custom provider = %#v", custom)
	}
	for _, expected := range []string{before, "[model_providers.custom]\nname = \"official\"\n"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("official config missing %q: %s", expected, contents)
		}
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != string(contents) {
		t.Fatalf("idempotent inserted custom provider = %q, %v", again, err)
	}
}

func TestWriteOfficialCodexConfigPreservesInsertionBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		before string
		want   string
	}{
		{
			name:   "crlf missing name before next table",
			before: "model_provider = \"custom\"\r\n[model_providers.custom]\r\nbase_url = \"https://provider.example/v1\"\r\n[features]\r\nmemories = true\r\n",
			want:   "model_provider = \"custom\"\r\n[model_providers.custom]\r\nname = \"official\"\r\n[features]\r\nmemories = true\r\n",
		},
		{
			name:   "no final newline and missing custom table",
			before: "model = \"keep\"",
			want:   "model_provider = \"custom\"\nmodel = \"keep\"\n[model_providers.custom]\nname = \"official\"\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.before), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := WriteOfficialCodexConfig(path); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != test.want {
				t.Fatalf("official config = %q, %v, want %q", contents, err, test.want)
			}
			if err = WriteOfficialCodexConfig(path); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil || string(again) != test.want {
				t.Fatalf("idempotent official config = %q, %v, want %q", again, err, test.want)
			}
		})
	}
}

func TestWriteOfficialCodexConfigFailureLeavesOriginalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := []byte("model_provider = \"custom\"\n[model_providers.custom]\nbase_url = \"https://provider.example/v1\"\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	oldReplace := replaceFile
	replaceFile = func(string, string) error { return errors.New("synthetic replace failure") }
	t.Cleanup(func() { replaceFile = oldReplace })
	if err := WriteOfficialCodexConfig(path); err == nil {
		t.Fatal("WriteOfficialCodexConfig succeeded during replace failure")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("config after failed replace = %q, %v", after, err)
	}
}

func TestCodexBearerOfficialBearerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := ClientConfig{Name: "first", Endpoint: "https://first.example/", Credential: "first-secret"}
	second := ClientConfig{Name: "second", Endpoint: "https://second.example", Credential: "second-secret"}
	if err := WriteCodexConfig(path, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	official, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(official), "base_url") || strings.Contains(string(official), "experimental_bearer_token") || !strings.Contains(string(official), "model_provider = 'custom'") {
		t.Fatalf("official config = %s", official)
	}
	_, custom := readCodexCustomProvider(t, path)
	if custom["name"] != OfficialProviderName {
		t.Fatalf("official custom provider = %#v", custom)
	}
	if err := WriteCodexConfig(path, second); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"https://second.example/v1", "second-secret"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("bearer config missing %q: %s", expected, contents)
		}
	}
	_, custom = readCodexCustomProvider(t, path)
	if custom["name"] != second.Name {
		t.Fatalf("second custom provider = %#v", custom)
	}
}

func TestWriteCodexConfigPreservesUnmanagedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = 'keep'\n[features]\nmemories = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexConfig(path, ClientConfig{Name: "example", Endpoint: "https://provider.example/", Credential: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{"model = 'keep'", "memories = true", "base_url = 'https://provider.example/v1'"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}

func TestWriteCodexWrapperConfigPreservesUnmanagedTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"Keep Name\" # keep field comment\nbase_url = \"https://old.example/v1\"\nexperimental_bearer_token = \"synthetic-secret\"\nrequires_openai_auth = true\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	want := "# leading comment\nmodel_provider = 'custom' # keep selector formatting\nmodel = \"gpt-5\"\n\n[model_providers.custom]\nname = \"official\" # keep field comment\nbase_url = \"https://wrapper.example/v1\"\nrequires_openai_auth = true\nwire_api = \"responses\"\ncustom_flag = true\n\n[features] # keep table comment\nmemories = true\n\n[[tools]]\nbase_url = \"keep-outside-custom\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example/", false); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != want {
		t.Fatalf("wrapper config:\n%s\nwant:\n%s", contents, want)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example/", false); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != want {
		t.Fatalf("idempotent wrapper config = %q, %v", again, err)
	}
}

func TestWriteCodexWrapperConfigRemovesExactlyTheBearerToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model_provider = \"custom\"\n[model_providers.custom]\nname = \"official\"\nbase_url = \"https://old.example/v1\"\nexperimental_bearer_token = \"synthetic-secret\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "experimental_bearer_token") {
		t.Fatalf("bearer token not removed: %s", contents)
	}
	if !strings.Contains(string(contents), `base_url = "https://wrapper.example/v1"`) {
		t.Fatalf("wrapper base_url missing: %s", contents)
	}
}

func TestWriteCodexWrapperConfigWithoutBearerTokenIsByteIdenticalApartFromNamedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model_provider = \"custom\"\n[model_providers.custom]\nname = \"official\"\nbase_url = \"https://old.example/v1\"\nrequires_openai_auth = true\nwire_api = \"responses\"\n"
	want := "model_provider = \"custom\"\n[model_providers.custom]\nname = \"official\"\nbase_url = \"https://wrapper.example/v1\"\nrequires_openai_auth = true\nwire_api = \"responses\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != want {
		t.Fatalf("wrapper config = %q, %v, want %q", contents, err, want)
	}
}

func TestWriteCodexWrapperConfigCreatesCustomTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model = 'keep'\n[features]\nmemories = true\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	contents, custom := readCodexCustomProvider(t, path)
	if custom["name"] != OfficialProviderName || custom["base_url"] != "https://wrapper.example/v1" {
		t.Fatalf("wrapper custom provider = %#v", custom)
	}
	for _, expected := range []string{before, "[model_providers.custom]\nname = \"official\"\nbase_url = \"https://wrapper.example/v1\"\n"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("wrapper config missing %q: %s", expected, contents)
		}
	}
}

func TestWriteCodexWrapperConfigFailureLeavesOriginalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := []byte("model_provider = \"custom\"\n[model_providers.custom]\nbase_url = \"https://old.example/v1\"\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	oldReplace := replaceFile
	replaceFile = func(string, string) error { return errors.New("synthetic replace failure") }
	t.Cleanup(func() { replaceFile = oldReplace })
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err == nil {
		t.Fatal("WriteCodexWrapperConfig succeeded during replace failure")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("config after failed replace = %q, %v", after, err)
	}
}

func TestCodexProjectHeadersMappingIsCanonicalAcrossWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := `model_provider = "custom"

[model_providers.custom]
name = "initial"
base_url = "https://initial.example/v1"

[model_providers.custom.env_http_headers]
X-Headroom-Project = "HEADROOM_PROJECT"
X-Unrelated = "OTHER_ENV"
`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	assertMapping := func(want bool) {
		t.Helper()
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := toml.Unmarshal(contents, &document); err != nil {
			t.Fatalf("invalid TOML after writer: %v\n%s", err, contents)
		}
		providers, _ := document["model_providers"].(map[string]any)
		custom, _ := providers["custom"].(map[string]any)
		headers, found := custom["env_http_headers"].(map[string]any)
		if !found {
			t.Fatalf("env_http_headers missing\n%s", contents)
		}
		projectEnvironment, projectFound := headers[HeadroomProjectHeader]
		if projectFound != want {
			t.Fatalf("project header presence = %v, want %v: %#v", projectFound, want, headers)
		}
		if want && projectEnvironment != HeadroomProjectEnvironment {
			t.Fatalf("project header mapping = %#v", headers)
		}
		if headers["X-Unrelated"] != "OTHER_ENV" {
			t.Fatalf("unrelated header mapping = %#v, want it preserved", headers)
		}
		if got := strings.Count(string(contents), `"X-Headroom-Project" = "HEADROOM_PROJECT"`); got != boolCount(want) {
			t.Fatalf("canonical mapping count = %d, want %d\n%s", got, boolCount(want), contents)
		}
		if strings.Contains(string(contents), "[model_providers.custom.env_http_headers]") {
			t.Fatalf("sub-table representation survived normalization:\n%s", contents)
		}
	}

	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", true); err != nil {
		t.Fatal(err)
	}
	assertMapping(true)
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	assertMapping(false)
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", true); err != nil {
		t.Fatal(err)
	}
	assertMapping(true)
	if err := WriteCodexConfig(path, ClientConfig{Name: "example", Endpoint: "https://provider.example", Credential: "synthetic-secret", ProjectAttribution: true}); err != nil {
		t.Fatal(err)
	}
	assertMapping(true)
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	assertMapping(false)
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", true); err != nil {
		t.Fatal(err)
	}
	assertMapping(true)
	if err := WriteCodexConfig(path, ClientConfig{Name: "example", Endpoint: "https://provider.example", Credential: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	assertMapping(false)
}

func TestCodexProjectHeadersNormalizeEquivalentSubtableSyntax(t *testing.T) {
	for _, header := range []string{
		`[model_providers.custom."env_http_headers"]`,
		`[model_providers.custom.'env_http_headers']`,
		`[ model_providers . custom . env_http_headers ]`,
	} {
		t.Run(header, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			initial := `model_provider = "custom"

[model_providers.custom]
name = "initial"
base_url = "https://initial.example/v1"

` + header + `
X-Headroom-Project = "OLD_ENV"
X-Unrelated = "OTHER_ENV"
`
			if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
				t.Fatal(err)
			}
			if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", true); err != nil {
				t.Fatal(err)
			}

			_, custom := readCodexCustomProvider(t, path)
			headers, ok := custom["env_http_headers"].(map[string]any)
			if !ok {
				t.Fatalf("env_http_headers = %#v, want mapping", custom["env_http_headers"])
			}
			if headers[HeadroomProjectHeader] != HeadroomProjectEnvironment {
				t.Fatalf("project header mapping = %#v", headers)
			}
			if headers["X-Unrelated"] != "OTHER_ENV" {
				t.Fatalf("unrelated header mapping = %#v, want it preserved", headers)
			}
		})
	}
}

func TestCodexProjectHeadersNormalizeQuotedOuterTablesAndInlineKeys(t *testing.T) {
	for _, test := range []struct {
		name    string
		initial string
	}{
		{
			name: "fully quoted table path",
			initial: `model_provider = "custom"

["model_providers".'custom']
name = "initial"
base_url = "https://initial.example/v1"

["model_providers".'custom'."env_http_headers"]
X-Headroom-Project = "OLD_ENV"
X-Unrelated = "OTHER_ENV"
`,
		},
		{
			name: "basic quoted inline key",
			initial: `model_provider = "custom"

[model_providers.custom]
name = "initial"
base_url = "https://initial.example/v1"
"env_http_headers" = { X-Headroom-Project = "OLD_ENV", X-Unrelated = "OTHER_ENV" }
`,
		},
		{
			name: "literal quoted inline key",
			initial: `model_provider = "custom"

[model_providers.custom]
name = "initial"
base_url = "https://initial.example/v1"
'env_http_headers' = { X-Headroom-Project = "OLD_ENV", X-Unrelated = "OTHER_ENV" }
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.initial), 0600); err != nil {
				t.Fatal(err)
			}
			if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", true); err != nil {
				t.Fatal(err)
			}

			_, custom := readCodexCustomProvider(t, path)
			headers, ok := custom["env_http_headers"].(map[string]any)
			if !ok {
				t.Fatalf("env_http_headers = %#v, want mapping", custom["env_http_headers"])
			}
			if headers[HeadroomProjectHeader] != HeadroomProjectEnvironment {
				t.Fatalf("project header mapping = %#v", headers)
			}
			if headers["X-Unrelated"] != "OTHER_ENV" {
				t.Fatalf("unrelated header mapping = %#v, want it preserved", headers)
			}
		})
	}
}

func TestCodexProjectHeadersDisabledLeavesUnrelatedInlineFieldUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	const unrelated = `env_http_headers   = { "Z-Unrelated" = "Z_ENV", "A-Unrelated" = "A_ENV" } # keep formatting`
	initial := `model_provider = "custom"

[model_providers.custom]
name = "initial"
base_url = "https://initial.example/v1"
` + unrelated + "\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	assertUnrelatedLine := func() {
		t.Helper()
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), unrelated) {
			t.Fatalf("unrelated inline field was rewritten:\n%s", contents)
		}
	}

	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	assertUnrelatedLine()
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	assertUnrelatedLine()
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestWriteOfficialCodexConfigResetsOwedNameAcrossArrayOfTablesOccurrences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model_provider = \"custom\"\n[[model_providers.custom]]\nbase_url = \"https://a.example/v1\"\n[[model_providers.custom]]\nbase_url = \"https://b.example/v1\"\n"
	want := "model_provider = \"custom\"\n[[model_providers.custom]]\nname = \"official\"\n[[model_providers.custom]]\nname = \"official\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOfficialCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != want {
		t.Fatalf("official config = %q, %v, want %q", contents, err, want)
	}
}

func TestWriteCodexWrapperConfigResetsOwedFieldsAcrossArrayOfTablesOccurrences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "model_provider = \"custom\"\n[[model_providers.custom]]\nbase_url = \"https://a.example/v1\"\n[[model_providers.custom]]\nbase_url = \"https://b.example/v1\"\n"
	want := "model_provider = \"custom\"\n[[model_providers.custom]]\nbase_url = \"https://wrapper.example/v1\"\nname = \"official\"\n[[model_providers.custom]]\nbase_url = \"https://wrapper.example/v1\"\nname = \"official\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodexWrapperConfig(path, OfficialProviderName, "https://wrapper.example", false); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != want {
		t.Fatalf("wrapper config = %q, %v, want %q", contents, err, want)
	}
}

func TestWriteClaudeConfigPreservesUnmanagedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"keep":true,"env":{"OTHER":"preserved"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{Endpoint: "https://provider.example/", Credential: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{`"keep": true`, `"OTHER": "preserved"`, `"ANTHROPIC_BASE_URL": "https://provider.example"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}

func TestWriteClaudeConfigEndpointWithoutCredentialRemovesTokenKeepsUnrelated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := `{"keep":true,"env":{"OTHER":"preserved","ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"old-secret"}}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{Endpoint: "https://wrapper.example/"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	if document["keep"] != true {
		t.Fatalf("unrelated top-level key not preserved: %s", contents)
	}
	env, _ := document["env"].(map[string]any)
	if env == nil {
		t.Fatalf("env object missing: %s", contents)
	}
	if env["OTHER"] != "preserved" {
		t.Fatalf("unrelated env key not preserved: %s", contents)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://wrapper.example" {
		t.Fatalf("endpoint not written: %s", contents)
	}
	if _, hasToken := env["ANTHROPIC_AUTH_TOKEN"]; hasToken {
		t.Fatalf("credential field not removed: %s", contents)
	}
}

func TestWriteClaudeConfigNeitherFieldRemovesBothKeepsEnvObjectAndUnrelated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := `{"keep":true,"env":{"OTHER":"preserved","ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"old-secret"}}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	if document["keep"] != true {
		t.Fatalf("unrelated top-level key not preserved: %s", contents)
	}
	env, _ := document["env"].(map[string]any)
	if env == nil {
		t.Fatalf("env object missing: %s", contents)
	}
	if env["OTHER"] != "preserved" {
		t.Fatalf("unrelated env key not preserved: %s", contents)
	}
	if _, hasEndpoint := env["ANTHROPIC_BASE_URL"]; hasEndpoint {
		t.Fatalf("endpoint field not removed: %s", contents)
	}
	if _, hasToken := env["ANTHROPIC_AUTH_TOKEN"]; hasToken {
		t.Fatalf("credential field not removed: %s", contents)
	}
}

func TestWriteClaudeConfigNeitherFieldKeepsEnvObjectWhenEnvHeldOnlyOwnedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := `{"keep":true,"env":{"ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"old-secret"}}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	env, ok := document["env"].(map[string]any)
	if !ok {
		t.Fatalf("env object removed entirely: %s", contents)
	}
	if len(env) != 0 {
		t.Fatalf("env object not empty: %s", contents)
	}
}

func TestWriteClaudeConfigNeitherFieldWithoutExistingEnvLeavesDocumentUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := `{"keep":true,"model":"opus"}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	if _, hasEnv := document["env"]; hasEnv {
		t.Fatalf("env object gratuitously created: %s", contents)
	}
	if document["keep"] != true || document["model"] != "opus" {
		t.Fatalf("unrelated top-level keys not preserved: %s", contents)
	}
}

func TestWriteClaudeConfigNeitherFieldLeavesNonObjectEnvUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := `{"keep":true,"env":"user-string-value"}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeConfig(path, ClientConfig{}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	if document["env"] != "user-string-value" {
		t.Fatalf("non-object env value destroyed by a no-owned-key write: %s", contents)
	}
}

func TestConfigMatchesEndpointWithoutReturningPrivateContent(t *testing.T) {
	root := t.TempDir()
	codex := filepath.Join(root, "config.toml")
	claude := filepath.Join(root, "settings.json")
	if err := os.WriteFile(codex, []byte("model_provider='custom'\n[model_providers.custom]\nbase_url='https://provider.example/v1'\nprivate='do-not-return'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://provider.example","PRIVATE":"do-not-return"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		client Client
		path   string
	}{
		{ClientCodex, codex},
		{ClientClaude, claude},
	} {
		matches, err := ConfigMatchesEndpoint(test.client, test.path, "https://provider.example")
		if err != nil || !matches {
			t.Fatalf("ConfigMatchesEndpoint(%s) = %t, %v", test.client, matches, err)
		}
		matches, err = ConfigMatchesEndpoint(test.client, test.path, "https://other.example")
		if err != nil || matches {
			t.Fatalf("drift ConfigMatchesEndpoint(%s) = %t, %v", test.client, matches, err)
		}
	}
}

func TestClaudeConfigMatchesSnapshotRoutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, test := range []struct {
		name     string
		contents string
		snapshot store.ProviderSnapshot
		want     bool
	}{
		{name: "custom endpoint", contents: `{"env":{"ANTHROPIC_BASE_URL":"https://provider.example"}}`, snapshot: store.ProviderSnapshot{Name: "custom", Endpoint: "https://provider.example"}, want: true},
		{name: "official direct", contents: `{}`, snapshot: store.ProviderSnapshot{Name: OfficialProviderName}, want: true},
		{name: "official wrapper", contents: `{"env":{"ANTHROPIC_BASE_URL":"https://wrapper.example"}}`, snapshot: store.ProviderSnapshot{Name: OfficialProviderName, Endpoint: "https://wrapper.example", ViaWrapper: true}, want: true},
		{name: "mismatch", contents: `{"env":{"ANTHROPIC_BASE_URL":"https://other.example"}}`, snapshot: store.ProviderSnapshot{Name: "custom", Endpoint: "https://provider.example"}, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ClaudeConfigMatchesSnapshot(path, test.snapshot)
			if err != nil || got != test.want {
				t.Fatalf("ClaudeConfigMatchesSnapshot() = %t, %v; want %t", got, err, test.want)
			}
		})
	}
}

// TestClaudeSettingsSnapshotMatchesConflictsSameDocument pins the reconcile
// classifier's snapshot rule: Matches and Conflicts both answer from the one
// document ReadClaudeSettingsSnapshot already parsed, so a file that carries
// both a matching endpoint and a conflicting credential source at once
// reports both consistently from a single read, and the snapshot never
// re-reads the path to answer either question — verified by deleting the
// file before calling either method.
func TestClaudeSettingsSnapshotMatchesConflictsSameDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	contents := `{"env":{"ANTHROPIC_BASE_URL":"https://provider.example","ANTHROPIC_API_KEY":"synthetic-secret"}}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadClaudeSettingsSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Mtime().IsZero() {
		t.Fatal("snapshot mtime is zero for an existing file")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !snapshot.Matches(store.ProviderSnapshot{Name: "custom", Endpoint: "https://provider.example"}) {
		t.Fatal("snapshot no longer matches after the file was removed, want the already-parsed document to still answer")
	}
	conflicts := snapshot.Conflicts()
	if len(conflicts) != 1 || conflicts[0] != ClaudeConflictAPIKey {
		t.Fatalf("snapshot conflicts = %#v, want [%s]", conflicts, ClaudeConflictAPIKey)
	}
}

// TestClaudeSettingsSnapshotUnreadableFileFailsClosed covers the case the
// reconcile loop must treat as indeterminate: a snapshot that cannot be read
// or parsed carries no usable document.
func TestClaudeSettingsSnapshotUnreadableFileFailsClosed(t *testing.T) {
	if _, err := ReadClaudeSettingsSnapshot(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("expected an error reading a missing settings file")
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"env":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClaudeSettingsSnapshot(path); err == nil {
		t.Fatal("expected an error parsing an invalid settings file")
	}
}

func TestConfigMatchesOfficialCodexRequiresOfficialName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, test := range []struct {
		name     string
		contents string
		want     bool
	}{
		{name: "official", contents: "model_provider='custom'\n[model_providers.custom]\nname='official'\n", want: true},
		{name: "stale custom name", contents: "model_provider='custom'\n[model_providers.custom]\nname='aigocode'\n", want: false},
		{name: "missing name", contents: "model_provider='custom'\n[model_providers.custom]\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			matches, err := ConfigMatchesOfficialCodex(path)
			if err != nil || matches != test.want {
				t.Fatalf("ConfigMatchesOfficialCodex() = %t, %v, want %t", matches, err, test.want)
			}
		})
	}
}

func TestWriteRedactedBackupOmitsCredential(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "config.toml"), filepath.Join(root, "backups", "config.toml")
	if err := os.WriteFile(source, []byte("[model_providers.custom]\nexperimental_bearer_token = 'synthetic-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientCodex, source, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "synthetic-secret") {
		t.Fatalf("backup contains credential: %s", contents)
	}
}

func TestWriteRedactedBackupForCodexAndClaudeCreatesParentAndRedactsCredential(t *testing.T) {
	root := t.TempDir()
	codexSource, codexDestination := filepath.Join(root, "config.toml"), filepath.Join(root, "parent", "deep", "config.toml.redacted")
	// Carry representative non-secret configuration alongside the secret. A
	// redactor that emitted an empty document would satisfy every
	// plaintext-absence and file-mode assertion while silently discarding the
	// configuration the backup exists to preserve, so the backup has to be
	// checked for what it keeps, not only for what it drops.
	codexConfig := "model_provider = \"custom\"\nmodel = \"gpt-5\"\n\n" +
		"[model_providers.custom]\nname = \"Custom\"\nbase_url = \"https://provider.example/v1\"\n" +
		"experimental_bearer_token = 'synthetic-secret'\nwire_api = \"responses\"\n\n" +
		"[features]\nmemories = true\n"
	if err := os.WriteFile(codexSource, []byte(codexConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientCodex, codexSource, codexDestination); err != nil {
		t.Fatal(err)
	}
	codexContents, err := os.ReadFile(codexDestination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(codexContents), "synthetic-secret") {
		t.Fatalf("codex backup contains credential: %s", codexContents)
	}
	if info, statErr := os.Stat(codexDestination); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("codex backup mode = %#v, %v", info, statErr)
	}
	var codexBackup struct {
		ModelProvider  string `toml:"model_provider"`
		Model          string `toml:"model"`
		ModelProviders map[string]struct {
			Name    string `toml:"name"`
			BaseURL string `toml:"base_url"`
			WireAPI string `toml:"wire_api"`
			Token   string `toml:"experimental_bearer_token"`
		} `toml:"model_providers"`
		Features map[string]any `toml:"features"`
	}
	if err = toml.Unmarshal(codexContents, &codexBackup); err != nil {
		t.Fatalf("codex backup is not parseable TOML: %v\n%s", err, codexContents)
	}
	custom, ok := codexBackup.ModelProviders["custom"]
	if !ok {
		t.Fatalf("codex backup dropped the custom provider entirely: %s", codexContents)
	}
	if codexBackup.ModelProvider != "custom" || codexBackup.Model != "gpt-5" ||
		custom.Name != "Custom" || custom.WireAPI != "responses" || codexBackup.Features["memories"] != true {
		t.Fatalf("codex backup dropped restorable configuration: %#v\n%s", codexBackup, codexContents)
	}
	if custom.Token != "" {
		t.Fatalf("codex backup retained the bearer token: %q", custom.Token)
	}

	claudeSource, claudeDestination := filepath.Join(root, "settings.json"), filepath.Join(root, "backup", "claude", "settings.json.redacted")
	if err := os.WriteFile(claudeSource, []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"synthetic-secret","OTHER":"keep"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientClaude, claudeSource, claudeDestination); err != nil {
		t.Fatal(err)
	}
	claudeContents, err := os.ReadFile(claudeDestination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(claudeContents), "synthetic-secret") {
		t.Fatalf("claude backup contains credential: %s", claudeContents)
	}
	if info, statErr := os.Stat(claudeDestination); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("claude backup mode = %#v, %v", info, statErr)
	}
	var claudeBackup struct {
		Env map[string]string `json:"env"`
	}
	if err = json.Unmarshal(claudeContents, &claudeBackup); err != nil {
		t.Fatalf("claude backup is not parseable JSON: %v\n%s", err, claudeContents)
	}
	if claudeBackup.Env["OTHER"] != "keep" {
		t.Fatalf("claude backup dropped restorable env configuration: %#v\n%s", claudeBackup, claudeContents)
	}
	if _, present := claudeBackup.Env["ANTHROPIC_AUTH_TOKEN"]; present {
		t.Fatalf("claude backup retained the auth token key: %#v", claudeBackup)
	}
}

// The backup exists to keep an interrupted operation diagnosable, and Recover
// documents that it "intentionally excludes credential values" -- values, not
// the single key AgentDeck happens to write. ANTHROPIC_API_KEY holds a value
// Claude honors, so leaving it in place copies a credential AgentDeck does not
// own into a second file. apiKeyHelper is asserted to survive for a scoped
// reason: it is a command setting rather than a direct API-key value, and this
// redactor drops the latter, so removing it would cost restorable
// configuration. A helper command line can still embed a secret; whether such a
// string counts as a credential here is a separate decision left to its own
// triage, so this assertion pins today's boundary rather than declaring the
// helper credential-free.
func TestWriteRedactedBackupDropsTheUnmanagedClaudeAPIKey(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "settings.json"), filepath.Join(root, "backup", "settings.json.redacted")
	contents := `{"env":{"ANTHROPIC_API_KEY":"unmanaged-secret","ANTHROPIC_BASE_URL":"https://provider.example","OTHER":"keep"},"apiKeyHelper":"/bin/echo helper","model":"opus"}`
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(ClientClaude, source, destination); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(backup), "unmanaged-secret") {
		t.Fatalf("claude backup contains the unmanaged api key: %s", backup)
	}
	var document struct {
		Env          map[string]string `json:"env"`
		APIKeyHelper string            `json:"apiKeyHelper"`
		Model        string            `json:"model"`
	}
	if err = json.Unmarshal(backup, &document); err != nil {
		t.Fatalf("claude backup is not parseable JSON: %v\n%s", err, backup)
	}
	if _, present := document.Env["ANTHROPIC_API_KEY"]; present {
		t.Fatalf("claude backup retained the api key: %#v", document)
	}
	if document.Env["ANTHROPIC_BASE_URL"] != "https://provider.example" || document.Env["OTHER"] != "keep" || document.Model != "opus" {
		t.Fatalf("claude backup dropped restorable configuration: %#v\n%s", document, backup)
	}
	if document.APIKeyHelper != "/bin/echo helper" {
		t.Fatalf("claude backup dropped the helper command, which this redactor scopes out as a command setting rather than a direct API-key value: %#v", document)
	}
}

func TestWriteRedactedBackupRejectsUnsupportedClient(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRedactedBackup(Client("unsupported"), source, filepath.Join(root, "unsupported.redacted.toml")); err == nil {
		t.Fatal("unsupported client backup succeeded")
	}
}

func TestWriteRedactedBackupWriteFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte("[model_providers.custom]\nexperimental_bearer_token = 'synthetic-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	blockedDir := filepath.Join(root, "blocked")
	if err := os.Mkdir(blockedDir, 0o500); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(blockedDir, "redacted.toml")
	if err := WriteRedactedBackup(ClientCodex, source, destination); err == nil {
		t.Fatal("write succeeded on read-only destination parent")
	}
}

func readCodexCustomProvider(t *testing.T, path string) ([]byte, map[string]any) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = toml.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	providers, ok := document["model_providers"].(map[string]any)
	if !ok {
		t.Fatalf("model providers = %#v", document["model_providers"])
	}
	custom, ok := providers["custom"].(map[string]any)
	if !ok {
		t.Fatalf("custom provider = %#v", providers["custom"])
	}
	return contents, custom
}
