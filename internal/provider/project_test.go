package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestProjectIdentityMatchesStoredSessionProject(t *testing.T) {
	ctx := context.Background()
	stateRoot := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	cwd := filepath.Join(t.TempDir(), "parent", "..", "project")
	sourcePath := filepath.Join(home, ".claude", "projects", "fixture", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0700); err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf(
		"{\"type\":\"user\",\"sessionId\":\"project-identity\",\"cwd\":%q,\"message\":{\"content\":\"visible prompt\"}}\n",
		cwd,
	)
	if err := os.WriteFile(sourcePath, []byte(record), 0600); err != nil {
		t.Fatal(err)
	}

	s, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := session.Scan(ctx, s.DB, home); err != nil {
		t.Fatal(err)
	}
	result, err := session.Show(ctx, s.DB, "claude", "project-identity")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := ProjectIdentity(cwd), result.Metadata.Project; got != want {
		t.Fatalf("ProjectIdentity(%q) = %q, stored session project = %q", cwd, got, want)
	}
	if got, want := result.Metadata.Project, filepath.Clean(cwd); got != want {
		t.Fatalf("stored session project = %q, want cleaned cwd %q", got, want)
	}
}

func TestProjectWireValueUsesOnlyThePercentEncodedBaseName(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "space", base: "project name", want: "project%20name"},
		{name: "newline", base: "project\nname", want: "project%0Aname"},
		{name: "quote", base: "project\"name", want: "project%22name"},
		{name: "non ASCII", base: "项目", want: "%E9%A1%B9%E7%9B%AE"},
		{name: "plus", base: "my+project", want: "my%2Bproject"},
		{name: "multiple pluses", base: "c++", want: "c%2B%2B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := filepath.Join(t.TempDir(), tt.base)
			got := ProjectWireValue(cwd)
			if got != tt.want {
				t.Fatalf("ProjectWireValue(%q) = %q, want %q", cwd, got, tt.want)
			}
			if strings.Contains(got, "+") {
				t.Fatalf("ProjectWireValue(%q) contains a bare plus: %q", cwd, got)
			}
			decoded, err := url.PathUnescape(got)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != tt.base {
				t.Fatalf("decoded wire value = %q, want %q", decoded, tt.base)
			}
		})
	}
}

func TestProjectWireValueAttributesNamelessDirectoriesToNothing(t *testing.T) {
	for _, cwd := range []string{"", ".", "..", filepath.Clean(string(filepath.Separator))} {
		t.Run(fmt.Sprintf("%q", cwd), func(t *testing.T) {
			if got := ProjectWireValue(cwd); got != "" {
				t.Fatalf("ProjectWireValue(%q) = %q, want empty", cwd, got)
			}
		})
	}
}

func TestRunProjectEnvironmentRequiresAHeadroomViaRouteAndHonorsUserValues(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err := service.Add(ctx, Definition{
		Name:          "example",
		Endpoint:      "https://provider.example",
		Clients:       []Client{ClientCodex, ClientClaude},
		CredentialRef: "ref",
	}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}

	codexConfig := filepath.Join(root, "config.toml")
	if err := os.WriteFile(codexConfig, []byte("model_provider = \"custom\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	useCodex := func(name string, via bool) {
		t.Helper()
		if err := service.UseCredential(ctx, "example", ClientCodex, "", codexConfig, filepath.Join(root, name+".backup.toml"), via); err != nil {
			t.Fatal(err)
		}
	}
	assertCodexMapping := func(want bool) {
		t.Helper()
		contents, err := os.ReadFile(codexConfig)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(contents), codexProjectHeadersMappingTOML); got != want {
			t.Fatalf("Codex project mapping present = %v, want %v\n%s", got, want, contents)
		}
	}
	assertNoChange := func(client Client, cwd string, environ []string) {
		t.Helper()
		if got, changed := service.RunProjectEnvironment(ctx, client, cwd, environ); changed || got != nil {
			t.Fatalf("RunProjectEnvironment(%s) = %#v, %v, want nil, false", client, got, changed)
		}
	}

	useCodex("direct", false)
	assertCodexMapping(false)
	assertNoChange(ClientCodex, filepath.Join(root, "project"), []string{"KEEP=value"})

	if _, _, err := service.SetWrapper(ctx, "example", "https://wrapper.example", WrapperKindPlain, false); err != nil {
		t.Fatal(err)
	}
	useCodex("plain", true)
	assertCodexMapping(false)
	assertNoChange(ClientCodex, filepath.Join(root, "project"), []string{"KEEP=value"})

	if _, _, err := service.SetWrapper(ctx, "example", "https://wrapper.example", WrapperKindHeadroom, false); err != nil {
		t.Fatal(err)
	}
	useCodex("headroom", true)
	assertCodexMapping(true)
	cwd := filepath.Join(root, "my+project")
	environment, changed := service.RunProjectEnvironment(ctx, ClientCodex, cwd, []string{"KEEP=value"})
	if !changed {
		t.Fatal("Codex Headroom route did not inject project environment")
	}
	if got, found := environmentValue(environment, HeadroomProjectEnvironment); !found || got != "my%2Bproject" {
		t.Fatalf("%s = %q, %v, want encoded project", HeadroomProjectEnvironment, got, found)
	}
	if got, found := environmentValue(environment, "KEEP"); !found || got != "value" {
		t.Fatalf("unrelated environment changed: KEEP = %q, %v", got, found)
	}
	assertNoChange(ClientCodex, cwd, []string{HeadroomProjectEnvironment + "=user-value"})
	assertNoChange(ClientCodex, ".", []string{"KEEP=value"})
	useCodex("headroom-to-direct", false)
	assertCodexMapping(false)
	assertNoChange(ClientCodex, cwd, []string{"KEEP=value"})

	if _, _, err := service.SetWrapper(ctx, OfficialProviderName, "https://official-wrapper.example", WrapperKindHeadroom, false); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(ctx, OfficialProviderName, ClientCodex, "", codexConfig, filepath.Join(root, "official-headroom.backup.toml"), true); err != nil {
		t.Fatal(err)
	}
	assertCodexMapping(true)
	if _, changed := service.RunProjectEnvironment(ctx, ClientCodex, filepath.Join(root, "official-project"), []string{"KEEP=value"}); !changed {
		t.Fatal("official Headroom route did not inject project environment")
	}

	claudeConfig := filepath.Join(root, "settings.json")
	if err := os.WriteFile(claudeConfig, []byte("{\"env\":{\"KEEP\":\"value\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(ctx, "example", ClientClaude, "", claudeConfig, filepath.Join(root, "claude.backup.json"), true); err != nil {
		t.Fatal(err)
	}
	environment, changed = service.RunProjectEnvironment(ctx, ClientClaude, filepath.Join(root, "project"), []string{
		"KEEP=value",
		ClaudeCustomHeadersEnvironment + "=Other-Header: keep",
	})
	if !changed {
		t.Fatal("Claude Headroom route did not inject project header")
	}
	if got, found := environmentValue(environment, ClaudeCustomHeadersEnvironment); !found || got != "Other-Header: keep\n"+HeadroomProjectHeader+": project" {
		t.Fatalf("%s = %q, %v", ClaudeCustomHeadersEnvironment, got, found)
	}
	assertNoChange(ClientClaude, filepath.Join(root, "project"), []string{
		ClaudeCustomHeadersEnvironment + "=Other-Header: keep\nx-headroom-project: user-value",
	})
}

func TestProjectRouteEligibilityExplainsEveryRouteState(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Store: database}

	assertEligibility := func(want ProjectRouteEligibility) {
		t.Helper()
		got, eligibilityErr := service.ProjectRouteEligibility(ctx, ClientCodex)
		if eligibilityErr != nil {
			t.Fatal(eligibilityErr)
		}
		if got != want {
			t.Fatalf("eligibility = %q, want %q", got, want)
		}
	}

	assertEligibility(ProjectRouteNoWrapper)
	if err := database.SetOfficialWrapperURL(ctx, "https://plain.example", string(WrapperKindPlain)); err != nil {
		t.Fatal(err)
	}
	if err := database.RecordSelection(ctx, store.Selection{
		Client:             string(ClientCodex),
		ProviderName:       OfficialProviderName,
		EndpointSnapshot:   "https://plain.example",
		MultiplierSnapshot: "1",
		ViaWrapper:         true,
	}); err != nil {
		t.Fatal(err)
	}
	assertEligibility(ProjectRouteWrapperNotHeadroom)

	if err := database.SetOfficialWrapperURL(ctx, "https://plain.example", string(WrapperKindHeadroom)); err != nil {
		t.Fatal(err)
	}
	assertEligibility(ProjectRouteEligible)

	if err := database.SetOfficialWrapperURL(ctx, "https://new.example", string(WrapperKindHeadroom)); err != nil {
		t.Fatal(err)
	}
	assertEligibility(ProjectRouteEndpointDrifted)

	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	got, eligibilityErr := service.ProjectRouteEligibility(ctx, ClientCodex)
	if got != ProjectRouteUndetermined || eligibilityErr == nil {
		t.Fatalf("closed-store eligibility = %q, %v, want undetermined error", got, eligibilityErr)
	}
}

func TestRunProjectEnvironmentRejectsStaleWrapperEndpoints(t *testing.T) {
	for _, providerName := range []string{"example", OfficialProviderName} {
		for _, client := range []Client{ClientCodex, ClientClaude} {
			t.Run(providerName+"/"+string(client), func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				database, err := store.Open(ctx, filepath.Join(root, "state"))
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()

				service := Service{Store: database, Vault: testCredentialVault(t)}
				if providerName != OfficialProviderName {
					if _, err := service.Add(ctx, Definition{
						Name:          providerName,
						Endpoint:      "https://provider.example",
						Clients:       []Client{ClientCodex, ClientClaude},
						CredentialRef: "ref",
					}, "synthetic-secret"); err != nil {
						t.Fatal(err)
					}
				}
				if _, _, err := service.SetWrapper(ctx, providerName, "https://wrapper-a.example", WrapperKindHeadroom, false); err != nil {
					t.Fatal(err)
				}

				configPath := filepath.Join(root, "settings.json")
				initialConfig := []byte("{}\n")
				if client == ClientCodex {
					configPath = filepath.Join(root, "config.toml")
					initialConfig = []byte("model_provider = \"custom\"\n")
				}
				if err := os.WriteFile(configPath, initialConfig, 0600); err != nil {
					t.Fatal(err)
				}
				if err := service.UseCredential(
					ctx,
					providerName,
					client,
					"",
					configPath,
					filepath.Join(root, "backup"),
					true,
				); err != nil {
					t.Fatal(err)
				}
				if _, _, err := service.SetWrapper(ctx, providerName, "https://wrapper-b.example", WrapperKindHeadroom, false); err != nil {
					t.Fatal(err)
				}

				if environment, changed := service.RunProjectEnvironment(
					ctx,
					client,
					filepath.Join(root, "project"),
					[]string{"KEEP=value"},
				); changed || environment != nil {
					t.Fatalf("stale wrapper route received project attribution: %#v, %v", environment, changed)
				}
			})
		}
	}
}
